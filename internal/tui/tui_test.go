package tui

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"citeoai/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelDefaultConfig(t *testing.T) {
	m := NewModel(nil, nil)
	if m.StatusBadge() != LabelReady {
		t.Fatalf("expected status %s, got %s", LabelReady, m.StatusBadge())
	}
	if m.TokenCount() != 0 {
		t.Fatalf("expected 0 tokens, got %d", m.TokenCount())
	}
	if m.IsRunning() {
		t.Fatalf("expected running to be false")
	}
}

func TestNewModelCustomConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ModelName = "test-model-4"
	m := NewModel(&cfg, nil)
	view := m.View()
	if !strings.Contains(view, "test-model-4") {
		t.Fatalf("expected view to contain test-model-4, got %s", view)
	}
}

func TestModelInit(t *testing.T) {
	m := NewModel(nil, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected non-nil init command")
	}
}

func TestModelWindowResize(t *testing.T) {
	m := NewModel(nil, nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	tm := newModel.(Model)
	if tm.width != 120 || tm.height != 40 {
		t.Fatalf("expected dimensions 120x40, got %dx%d", tm.width, tm.height)
	}
	if !tm.ready {
		t.Fatalf("expected ready to be true")
	}
}

func TestModelInputTypingAndNavigation(t *testing.T) {
	m := NewModel(nil, nil)

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g', 'o'}})
	tm := step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeySpace})
	tm = step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v', 'e', 't'}})
	tm = step.(Model)

	if tm.InputText() != "go vet" {
		t.Fatalf("expected input 'go vet', got '%s'", tm.InputText())
	}
	if tm.CursorPosition() != 6 {
		t.Fatalf("expected cursor 6, got %d", tm.CursorPosition())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	tm = step.(Model)
	if tm.CursorPosition() != 5 {
		t.Fatalf("expected cursor 5, got %d", tm.CursorPosition())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRight})
	tm = step.(Model)
	if tm.CursorPosition() != 6 {
		t.Fatalf("expected cursor 6, got %d", tm.CursorPosition())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	tm = step.(Model)
	if tm.InputText() != "go ve" {
		t.Fatalf("expected input 'go ve', got '%s'", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyHome})
	tm = step.(Model)
	if tm.CursorPosition() != 0 {
		t.Fatalf("expected cursor 0, got %d", tm.CursorPosition())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyDelete})
	tm = step.(Model)
	if tm.InputText() != "o ve" {
		t.Fatalf("expected input 'o ve', got '%s'", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnd})
	tm = step.(Model)
	if tm.CursorPosition() != len(tm.InputText()) {
		t.Fatalf("expected cursor at end, got %d", tm.CursorPosition())
	}
}

func TestModelPromptSubmit(t *testing.T) {
	m := NewModel(nil, nil)

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm := step.(Model)
	if tm.IsRunning() {
		t.Fatalf("empty input should not start execution")
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'e', 'l', 'l', 'o'}})
	tm = step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm = step.(Model)

	if !tm.IsRunning() {
		t.Fatalf("expected running to be true")
	}
	if tm.StatusBadge() != LabelRunning {
		t.Fatalf("expected status %s, got %s", LabelRunning, tm.StatusBadge())
	}
	if tm.InputText() != "" {
		t.Fatalf("expected input to be cleared after submit")
	}
	if len(tm.Messages()) != 1 || tm.Messages()[0].Content != "hello" {
		t.Fatalf("expected user message 'hello', got %#v", tm.Messages())
	}
}

func TestModelStreamingAndLogs(t *testing.T) {
	m := NewModel(nil, nil)

	step, _ := m.Update(tokenMsg("Stream chunk 1. "))
	tm := step.(Model)
	step, _ = tm.Update(tokenMsg("Stream chunk 2."))
	tm = step.(Model)

	if tm.StreamingText() != "Stream chunk 1. Stream chunk 2." {
		t.Fatalf("unexpected streaming content: %s", tm.StreamingText())
	}
	if tm.TokenCount() != 2 {
		t.Fatalf("expected token count 2, got %d", tm.TokenCount())
	}

	step, _ = tm.Update(logMsg("> Reading file internal/tui/tui.go"))
	tm = step.(Model)

	if len(tm.Logs()) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(tm.Logs()))
	}
	if tm.Logs()[0] != "> Reading file internal/tui/tui.go" {
		t.Fatalf("unexpected log line: %s", tm.Logs()[0])
	}
}

func TestModelDoneSuccess(t *testing.T) {
	m := NewModel(nil, nil)
	step, _ := m.Update(tokenMsg("Completed response"))
	tm := step.(Model)

	step, _ = tm.Update(doneMsg{err: nil})
	tm = step.(Model)

	if tm.IsRunning() {
		t.Fatalf("expected running to be false")
	}
	if tm.StatusBadge() != LabelDone {
		t.Fatalf("expected status %s, got %s", LabelDone, tm.StatusBadge())
	}
	if len(tm.Messages()) != 1 {
		t.Fatalf("expected committed assistant message, got %d", len(tm.Messages()))
	}
	if tm.Messages()[0].Content != "Completed response" {
		t.Fatalf("unexpected assistant message: %s", tm.Messages()[0].Content)
	}
	if tm.StreamingText() != "" {
		t.Fatalf("expected streaming buffer to be cleared")
	}
}

func TestModelDoneError(t *testing.T) {
	m := NewModel(nil, nil)
	step, _ := m.Update(doneMsg{err: errors.New("provider connection refused")})
	tm := step.(Model)

	if tm.IsRunning() {
		t.Fatalf("expected running to be false")
	}
	if tm.StatusBadge() != LabelFailed {
		t.Fatalf("expected status %s, got %s", LabelFailed, tm.StatusBadge())
	}
	if len(tm.Logs()) == 0 {
		t.Fatalf("expected error logged in execution logs")
	}
}

func TestModelCancelViaEsc(t *testing.T) {
	m := NewModel(nil, nil)
	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r', 'u', 'n'}})
	tm := step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm = step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm = step.(Model)

	if tm.IsRunning() {
		t.Fatalf("expected running to be false after cancel")
	}
	if tm.StatusBadge() != LabelCanceled {
		t.Fatalf("expected status %s, got %s", LabelCanceled, tm.StatusBadge())
	}

	step, _ = tm.Update(doneMsg{err: context.Canceled})
	tm = step.(Model)
	if tm.StatusBadge() != LabelCanceled {
		t.Fatalf("expected status to remain %s, got %s", LabelCanceled, tm.StatusBadge())
	}
}

func TestModelEscClearInput(t *testing.T) {
	m := NewModel(nil, nil)
	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c', 'l', 'e', 'a', 'r'}})
	tm := step.(Model)
	if tm.InputText() != "clear" {
		t.Fatalf("expected input 'clear', got %s", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm = step.(Model)
	if tm.InputText() != "" {
		t.Fatalf("expected empty input after Esc, got %s", tm.InputText())
	}
}

func TestModelHistoryNavigation(t *testing.T) {
	m := NewModel(nil, nil)

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f', 'i', 'r', 's', 't'}})
	tm := step.(Model)
	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm = step.(Model)
	step, _ = tm.Update(doneMsg{err: nil})
	tm = step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 'e', 'c', 'o', 'n', 'd'}})
	tm = step.(Model)
	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm = step.(Model)
	step, _ = tm.Update(doneMsg{err: nil})
	tm = step.(Model)

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyUp})
	tm = step.(Model)
	if tm.InputText() != "second" {
		t.Fatalf("expected 'second', got '%s'", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyUp})
	tm = step.(Model)
	if tm.InputText() != "first" {
		t.Fatalf("expected 'first', got '%s'", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyDown})
	tm = step.(Model)
	if tm.InputText() != "second" {
		t.Fatalf("expected 'second', got '%s'", tm.InputText())
	}

	step, _ = tm.Update(tea.KeyMsg{Type: tea.KeyDown})
	tm = step.(Model)
	if tm.InputText() != "" {
		t.Fatalf("expected empty input on bottom of history, got '%s'", tm.InputText())
	}
}

func TestModelCtrlCQuit(t *testing.T) {
	m := NewModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected non-nil quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestModelRamTick(t *testing.T) {
	m := NewModel(nil, nil)
	_, cmd := m.Update(ramTickMsg{})
	if cmd == nil {
		t.Fatalf("expected non-nil next tick command")
	}
}

func TestModelViewRendering(t *testing.T) {
	m := NewModel(nil, nil)
	viewEmpty := m.View()
	if !strings.Contains(viewEmpty, "CITEOAI") {
		t.Fatalf("view should contain CITEOAI header")
	}
	if !strings.Contains(viewEmpty, LabelTranscript) {
		t.Fatalf("view should contain transcript label")
	}
	if !strings.Contains(viewEmpty, LabelLogs) {
		t.Fatalf("view should contain logs label")
	}
	if !strings.Contains(viewEmpty, LabelModel) {
		t.Fatalf("view should contain model label")
	}

	m.appendMessage("User", "What is CITEOAI?")
	m.appendMessage("CITEOAI", "Ultra-lightweight AI agent engine.")
	m.appendLog("> test log entry 1")

	viewPopulated := m.View()
	if !strings.Contains(viewPopulated, "What is CITEOAI?") {
		t.Fatalf("view should render user message")
	}
	if !strings.Contains(viewPopulated, "Ultra-lightweight AI agent engine.") {
		t.Fatalf("view should render agent message")
	}
	if !strings.Contains(viewPopulated, "test log entry 1") {
		t.Fatalf("view should render log entry")
	}
}

func TestWrapText(t *testing.T) {
	empty := wrapText("", 20)
	if len(empty) != 1 || empty[0] != "" {
		t.Fatalf("unexpected wrap of empty string: %#v", empty)
	}

	short := wrapText("hello", 20)
	if len(short) != 1 || short[0] != "hello" {
		t.Fatalf("unexpected wrap of short text: %#v", short)
	}

	long := wrapText("the quick brown fox jumps over the lazy dog", 15)
	if len(long) < 2 {
		t.Fatalf("expected multi-line wrap, got %#v", long)
	}
	for _, l := range long {
		if len([]rune(l)) > 15 {
			t.Fatalf("line exceeds max width 15: '%s'", l)
		}
	}

	noSpaces := wrapText("abcdefghijklmnopqrstuvwxyz", 10)
	if len(noSpaces) != 3 {
		t.Fatalf("expected 3 chunks for 26-char word wrapped at 10, got %d", len(noSpaces))
	}

	negativeWidth := wrapText("fallback width test", 0)
	if len(negativeWidth) == 0 {
		t.Fatalf("expected fallback width to wrap successfully")
	}
}

func TestMemoryCapping(t *testing.T) {
	m := NewModel(nil, nil)
	for i := 0; i < 2200; i++ {
		m.appendMessage("User", "ping")
		m.appendLog("ping log")
	}

	if len(m.Messages()) > 2000 {
		t.Fatalf("messages exceeded 2000 cap: %d", len(m.Messages()))
	}
	if len(m.Logs()) > 2000 {
		t.Fatalf("logs exceeded 2000 cap: %d", len(m.Logs()))
	}
}

func TestStatusBadgeStyles(t *testing.T) {
	badges := []string{LabelReady, LabelRunning, LabelDone, LabelCanceled, LabelFailed, "unknown"}
	for _, badge := range badges {
		style := StatusBadgeStyle(badge)
		rendered := style.Render(badge)
		if rendered == "" {
			t.Fatalf("expected non-empty rendered badge for %s", badge)
		}
	}
}

func TestBubbleTeaLifecycleRun(t *testing.T) {
	cfg := config.DefaultConfig()
	runner := func(ctx context.Context, prompt string, onToken func(string), onLog func(string)) error {
		onToken("tok")
		onLog("log")
		return nil
	}

	err := runTUIWithOptions(&cfg, runner, tea.WithInput(strings.NewReader("\x03")), tea.WithOutput(io.Discard))
	if err != nil {
		t.Fatalf("expected clean run exit with Ctrl+C, got err: %v", err)
	}
}
