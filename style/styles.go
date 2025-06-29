package style

import "github.com/charmbracelet/lipgloss"

// nolint:gochecknoglobals
// Code styles for the debugger
// These styles are used to highlight different parts of the code in the debugger viewer.
var (
	CodeHighlight = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFEA")).
			Background(lipgloss.Color("#8D2684"))
	// styleCodeNormal = lipgloss.NewStyle().
	// 		Background(lipgloss.Color("#2A3D45"))
	Cursor = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#44FF44"))
	Breakpoint = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF4444"))

	Nil               = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	Bool              = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD00"))
	Int               = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAEEFF"))
	Float             = lipgloss.NewStyle().Foreground(lipgloss.Color("#44CCEE"))
	String            = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCFFAA"))
	Function          = lipgloss.NewStyle()
	List              = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800"))
	Vector            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBB22"))
	HashMap           = lipgloss.NewStyle().Foreground(lipgloss.Color("#22BBAA"))
	Symbol            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBB55"))
	Keyword           = lipgloss.NewStyle().Foreground(lipgloss.Color("#66BBFF"))
	Error             = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	Comment           = lipgloss.NewStyle().Foreground(lipgloss.Color("#44FF44"))
	Brackets          = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	SquareBrackets    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EEEE88"))
	CurlyBrackets     = lipgloss.NewStyle().Foreground(lipgloss.Color("#EEEE88"))
	SpecialFormSymbol = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
)
