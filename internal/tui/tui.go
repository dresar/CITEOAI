package tui

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"citeoai/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PromptRunner func(ctx context.Context, prompt string, onToken func(string), onLog func(string)) error

type ChatMessage struct {
	Role    string
	Content string
}

type tokenMsg string

type logMsg string

type doneMsg struct {
	err error
}

type ramTickMsg struct{}

type programHolder struct {
	mu sync.RWMutex
	p  *tea.Program
}

func (h *programHolder) set(p *tea.Program) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.p = p
}

func (h *programHolder) send(msg tea.Msg) {
	if h == nil {
		return
	}
	h.mu.RLock()
	p := h.p
	h.mu.RUnlock()
	if p != nil {
		p.Send(msg)
	}
}

type Model struct {
	cfg          *config.Config
	runPrompt    PromptRunner
	holder       *programHolder
	cancel       context.CancelFunc
	width        int
	height       int
	ready        bool
	messages     []ChatMessage
	streaming    strings.Builder
	logs         []string
	inputRunes   []rune
	cursor       int
	inputHistory []string
	historyIndex int
	running      bool
	statusBadge  string
	tokenCount   int
	ramUsage     string
	sessionID    string
}

func NewModel(cfg *config.Config, runPrompt PromptRunner) Model {
	holder := &programHolder{}
	return newModelWithHolder(cfg, runPrompt, holder)
}

func newModelWithHolder(cfg *config.Config, runPrompt PromptRunner, holder *programHolder) Model {
	if cfg == nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	m := Model{
		cfg:          cfg,
		runPrompt:    runPrompt,
		holder:       holder,
		statusBadge:  LabelReady,
		sessionID:    "default",
		width:        80,
		height:       24,
		ramUsage:     "0.0 MB",
		historyIndex: -1,
	}
	m.updateRAM()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return ramTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case ramTickMsg:
		m.updateRAM()
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return ramTickMsg{}
		})

	case tokenMsg:
		m.streaming.WriteString(string(msg))
		m.tokenCount++
		m.updateRAM()
		return m, nil

	case logMsg:
		m.appendLog(string(msg))
		m.updateRAM()
		return m, nil

	case doneMsg:
		m.running = false
		m.cancel = nil
		if m.streaming.Len() > 0 {
			m.appendMessage("CITEOAI", m.streaming.String())
			m.streaming.Reset()
		}
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) || m.statusBadge == LabelCanceled {
				m.statusBadge = LabelCanceled
			} else {
				m.statusBadge = LabelFailed
				m.appendLog(fmt.Sprintf("> Exit: error (%v)", msg.err))
			}
		} else {
			if m.statusBadge != LabelCanceled {
				m.statusBadge = LabelDone
			}
		}
		m.updateRAM()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.running && m.cancel != nil {
				m.cancel()
				m.cancel = nil
				m.running = false
				m.statusBadge = LabelCanceled
				m.appendLog("> Canceled")
			} else {
				m.inputRunes = nil
				m.cursor = 0
			}
			return m, nil

		case tea.KeyEnter:
			inputStr := strings.TrimSpace(string(m.inputRunes))
			if inputStr == "" || m.running {
				return m, nil
			}
			m.appendMessage("User", inputStr)
			m.inputHistory = append(m.inputHistory, inputStr)
			m.historyIndex = -1
			m.inputRunes = nil
			m.cursor = 0
			m.running = true
			m.statusBadge = LabelRunning
			m.streaming.Reset()
			m.updateRAM()

			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			runner := m.runPrompt
			holder := m.holder

			go func() {
				var runErr error
				if runner != nil {
					runErr = runner(ctx, inputStr, func(token string) {
						holder.send(tokenMsg(token))
					}, func(line string) {
						holder.send(logMsg(line))
					})
				}
				holder.send(doneMsg{err: runErr})
			}()
			return m, nil

		case tea.KeyBackspace:
			if m.cursor > 0 && len(m.inputRunes) > 0 {
				m.inputRunes = append(m.inputRunes[:m.cursor-1], m.inputRunes[m.cursor:]...)
				m.cursor--
			}
			return m, nil

		case tea.KeyDelete:
			if m.cursor < len(m.inputRunes) {
				m.inputRunes = append(m.inputRunes[:m.cursor], m.inputRunes[m.cursor+1:]...)
			}
			return m, nil

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case tea.KeyRight:
			if m.cursor < len(m.inputRunes) {
				m.cursor++
			}
			return m, nil

		case tea.KeyHome, tea.KeyCtrlA:
			m.cursor = 0
			return m, nil

		case tea.KeyEnd, tea.KeyCtrlE:
			m.cursor = len(m.inputRunes)
			return m, nil

		case tea.KeyUp:
			if len(m.inputHistory) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = len(m.inputHistory) - 1
				} else if m.historyIndex > 0 {
					m.historyIndex--
				}
				m.inputRunes = []rune(m.inputHistory[m.historyIndex])
				m.cursor = len(m.inputRunes)
			}
			return m, nil

		case tea.KeyDown:
			if len(m.inputHistory) > 0 && m.historyIndex != -1 {
				if m.historyIndex < len(m.inputHistory)-1 {
					m.historyIndex++
					m.inputRunes = []rune(m.inputHistory[m.historyIndex])
				} else {
					m.historyIndex = -1
					m.inputRunes = nil
				}
				m.cursor = len(m.inputRunes)
			}
			return m, nil

		case tea.KeyRunes, tea.KeySpace:
			runesToInsert := msg.Runes
			if msg.Type == tea.KeySpace {
				runesToInsert = []rune{' '}
			}
			if len(runesToInsert) > 0 {
				prefix := append([]rune{}, m.inputRunes[:m.cursor]...)
				suffix := m.inputRunes[m.cursor:]
				m.inputRunes = append(prefix, append(runesToInsert, suffix...)...)
				m.cursor += len(runesToInsert)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) appendMessage(role, content string) {
	m.messages = append(m.messages, ChatMessage{Role: role, Content: content})
	if len(m.messages) > 2000 {
		m.messages = m.messages[len(m.messages)-2000:]
	}
}

func (m *Model) appendLog(line string) {
	m.logs = append(m.logs, line)
	if len(m.logs) > 2000 {
		m.logs = m.logs[len(m.logs)-2000:]
	}
}

func (m *Model) updateRAM() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	allocMB := float64(mem.Alloc) / (1024 * 1024)
	m.ramUsage = fmt.Sprintf("%.1f MB", allocMB)
}

func (m Model) Messages() []ChatMessage {
	return m.messages
}

func (m Model) Logs() []string {
	return m.logs
}

func (m Model) StatusBadge() string {
	return m.statusBadge
}

func (m Model) IsRunning() bool {
	return m.running
}

func (m Model) TokenCount() int {
	return m.tokenCount
}

func (m Model) InputText() string {
	return string(m.inputRunes)
}

func (m Model) CursorPosition() int {
	return m.cursor
}

func (m Model) StreamingText() string {
	return m.streaming.String()
}

func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	header := TitleStyle.Render("CITEOAI") + " " + MutedStyle.Render("(v0.1.0)")

	paneHeight := height - 6
	if paneHeight < 5 {
		paneHeight = 5
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth
	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 20 {
		rightWidth = 20
	}

	contentLeftWidth := leftWidth - BaseCardStyle.GetHorizontalFrameSize()
	contentLeftHeight := paneHeight - BaseCardStyle.GetVerticalFrameSize()
	if contentLeftWidth < 1 {
		contentLeftWidth = 1
	}
	if contentLeftHeight < 1 {
		contentLeftHeight = 1
	}

	var transcriptLines []string
	transcriptLines = append(transcriptLines, MutedStyle.Render("["+LabelTranscript+"]"))
	for _, msg := range m.messages {
		if msg.Role == "User" {
			transcriptLines = append(transcriptLines, UserLabelStyle.Render("User: "))
		} else {
			transcriptLines = append(transcriptLines, AgentLabelStyle.Render("CITEOAI: "))
		}
		for _, line := range wrapText(msg.Content, contentLeftWidth) {
			transcriptLines = append(transcriptLines, line)
		}
		transcriptLines = append(transcriptLines, "")
	}
	if m.streaming.Len() > 0 {
		transcriptLines = append(transcriptLines, AgentLabelStyle.Render("CITEOAI: "))
		for _, line := range wrapText(m.streaming.String(), contentLeftWidth) {
			transcriptLines = append(transcriptLines, line)
		}
	}

	var visibleTranscript []string
	if len(transcriptLines) > contentLeftHeight {
		visibleTranscript = transcriptLines[len(transcriptLines)-contentLeftHeight:]
	} else {
		visibleTranscript = transcriptLines
	}
	leftBody := strings.Join(visibleTranscript, "\n")
	leftCard := BaseCardStyle
	if m.running {
		leftCard = ActiveCardStyle
	}
	renderedLeft := leftCard.Width(contentLeftWidth).Height(contentLeftHeight).Render(leftBody)

	contentRightWidth := rightWidth - BaseCardStyle.GetHorizontalFrameSize()
	contentRightHeight := paneHeight - BaseCardStyle.GetVerticalFrameSize()
	if contentRightWidth < 1 {
		contentRightWidth = 1
	}
	if contentRightHeight < 1 {
		contentRightHeight = 1
	}

	var logLines []string
	logLines = append(logLines, MutedStyle.Render("["+LabelLogs+"]"))
	for _, l := range m.logs {
		for _, line := range wrapText(l, contentRightWidth) {
			logLines = append(logLines, ToolPrefixStyle.Render(line))
		}
	}

	var visibleLogs []string
	if len(logLines) > contentRightHeight {
		visibleLogs = logLines[len(logLines)-contentRightHeight:]
	} else {
		visibleLogs = logLines
	}
	rightBody := strings.Join(visibleLogs, "\n")
	renderedRight := BaseCardStyle.Width(contentRightWidth).Height(contentRightHeight).Render(rightBody)

	panesRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedLeft, renderedRight)

	modelName := "gemini-3.6-flash"
	if m.cfg != nil && m.cfg.ModelName != "" {
		modelName = m.cfg.ModelName
	}

	statusBar := StatusLabelStyle.Render(LabelModel+": ") + StatusValueStyle.Render(modelName) +
		MutedStyle.Render(" │ ") +
		StatusLabelStyle.Render(LabelRAM+": ") + StatusValueStyle.Render(m.ramUsage) +
		MutedStyle.Render(" │ ") +
		StatusLabelStyle.Render(LabelTokens+": ") + StatusValueStyle.Render(strconv.Itoa(m.tokenCount)) +
		MutedStyle.Render(" │ ") +
		StatusLabelStyle.Render(LabelSession+": ") + StatusValueStyle.Render(m.sessionID) +
		MutedStyle.Render(" │ ") +
		StatusBadgeStyle(m.statusBadge).Render(m.statusBadge)

	contentInputWidth := width - BaseCardStyle.GetHorizontalFrameSize()
	if contentInputWidth < 1 {
		contentInputWidth = 1
	}

	var inputContent string
	if len(m.inputRunes) == 0 {
		inputContent = PromptPrefixStyle.Render("> ") + MutedStyle.Render("Enter prompt...")
	} else {
		before := string(m.inputRunes[:m.cursor])
		cursorChar := "█"
		after := ""
		if m.cursor < len(m.inputRunes) {
			cursorChar = string(m.inputRunes[m.cursor : m.cursor+1])
			after = string(m.inputRunes[m.cursor+1:])
		}
		inputContent = PromptPrefixStyle.Render("> ") + PromptInputStyle.Render(before) + CursorStyle.Render(cursorChar) + PromptInputStyle.Render(after)
	}

	inputCard := BaseCardStyle
	if !m.running {
		inputCard = ActiveCardStyle
	}
	renderedInput := inputCard.Width(contentInputWidth).Render(inputContent)

	return lipgloss.JoinVertical(lipgloss.Left, header, panesRow, statusBar, renderedInput)
}

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 40
	}
	var result []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line == "" {
			result = append(result, "")
			continue
		}
		runes := []rune(line)
		for len(runes) > maxWidth {
			breakPoint := maxWidth
			foundSpace := false
			for i := maxWidth; i > 0; i-- {
				if unicode.IsSpace(runes[i-1]) {
					breakPoint = i
					foundSpace = true
					break
				}
			}
			if !foundSpace {
				breakPoint = maxWidth
			}
			result = append(result, strings.TrimRight(string(runes[:breakPoint]), " "))
			runes = runes[breakPoint:]
			for len(runes) > 0 && unicode.IsSpace(runes[0]) {
				runes = runes[1:]
			}
		}
		if len(runes) > 0 {
			result = append(result, string(runes))
		}
	}
	return result
}

func RunTUI(cfg *config.Config, runPrompt func(ctx context.Context, prompt string, onToken func(string), onLog func(string)) error) error {
	return runTUIWithOptions(cfg, runPrompt, tea.WithAltScreen())
}

func runTUIWithOptions(cfg *config.Config, runPrompt PromptRunner, opts ...tea.ProgramOption) error {
	holder := &programHolder{}
	m := newModelWithHolder(cfg, runPrompt, holder)
	p := tea.NewProgram(m, opts...)
	holder.set(p)
	_, err := p.Run()
	return err
}
