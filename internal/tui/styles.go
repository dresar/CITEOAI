package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorCardBg     = lipgloss.Color("#ffffff")
	ColorCardText   = lipgloss.Color("#172033")
	ColorCardMuted  = lipgloss.Color("#667085")
	ColorCardBorder = lipgloss.Color("#e4e7ec")
	ColorCardAccent = lipgloss.Color("#4f46e5")
	ColorSuccess    = lipgloss.Color("#027a48")
	ColorWarning    = lipgloss.Color("#b54708")
	ColorError      = lipgloss.Color("#b42318")

	BaseCardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorCardBorder)

	ActiveCardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorCardAccent)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCardText)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCardAccent)

	BadgeReadyStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	BadgeRunningStyle = lipgloss.NewStyle().
			Foreground(ColorCardAccent).
			Bold(true)

	BadgeDoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	BadgeCanceledStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	BadgeErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorCardMuted)

	UserLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCardAccent)

	AgentLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	ToolPrefixStyle = lipgloss.NewStyle().
			Foreground(ColorCardMuted)

	StatusLabelStyle = lipgloss.NewStyle().
			Foreground(ColorCardMuted)

	StatusValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCardText)

	PromptPrefixStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCardAccent)

	PromptInputStyle = lipgloss.NewStyle().
			Foreground(ColorCardText)

	CursorStyle = lipgloss.NewStyle().
			Foreground(ColorCardAccent).
			Bold(true)
)

const (
	LabelReady      = "Ready"
	LabelRunning    = "Running"
	LabelDone       = "Done"
	LabelCanceled   = "Canceled"
	LabelFailed     = "Failed"
	LabelSubmit     = "Submit"
	LabelCancel     = "Cancel"
	LabelQuit       = "Quit"
	LabelModel      = "Model"
	LabelRAM        = "RAM"
	LabelTokens     = "Tokens"
	LabelSession    = "Session"
	LabelTranscript = "Transcript"
	LabelLogs       = "Execution Logs"
)

func StatusBadgeStyle(status string) lipgloss.Style {
	switch status {
	case LabelRunning:
		return BadgeRunningStyle
	case LabelDone:
		return BadgeDoneStyle
	case LabelCanceled:
		return BadgeCanceledStyle
	case LabelFailed:
		return BadgeErrorStyle
	default:
		return BadgeReadyStyle
	}
}
