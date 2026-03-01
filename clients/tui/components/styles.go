// Package components provides reusable TUI components and styles.
package components

import "github.com/charmbracelet/lipgloss"

// =============================================================================
// Color Palette - Single Source of Truth
// =============================================================================

const (
	// Primary colors
	ColorPrimary   = "#7C3AED" // Violet - user messages, headings
	ColorSecondary = "#10B981" // Green/Emerald - assistant, success
	ColorAccent    = "#60A5FA" // Blue - links, labels
	ColorWarning   = "#F59E0B" // Amber - tool calls, warnings
	ColorError     = "#EF4444" // Red - errors

	// Neutral colors
	ColorMuted      = "#6B7280" // Gray - muted text, hints
	ColorBorder     = "#374151" // Dark gray - borders
	ColorBackground = "#1F2937" // Dark slate - backgrounds
	ColorSurface    = "#1E293B" // Slightly lighter - header bg

	// Text colors
	ColorText       = "#E5E7EB" // Light gray - base text
	ColorTextBright = "#FFFFFF" // White - emphasis
	ColorTextDim    = "#9CA3AF" // Dim gray - secondary text
	ColorTextIndigo = "#A5B4FC" // Light indigo - model name
)

// Color helpers for lipgloss
var (
	Primary    = lipgloss.Color(ColorPrimary)
	Secondary  = lipgloss.Color(ColorSecondary)
	Accent     = lipgloss.Color(ColorAccent)
	Warning    = lipgloss.Color(ColorWarning)
	Error      = lipgloss.Color(ColorError)
	Muted      = lipgloss.Color(ColorMuted)
	Border     = lipgloss.Color(ColorBorder)
	Surface    = lipgloss.Color(ColorSurface)
	Text       = lipgloss.Color(ColorText)
	TextBright = lipgloss.Color(ColorTextBright)
	TextDim    = lipgloss.Color(ColorTextDim)
)

// =============================================================================
// Message Styles
// =============================================================================

var (
	// UserStyle for user messages
	UserStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// AssistantStyle for assistant messages
	AssistantStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// SystemStyle for system messages
	SystemStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)
)

// =============================================================================
// Tool Styles
// =============================================================================

var (
	// ToolCallStyle for tool call headers
	ToolCallStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	// ToolResultStyle for tool results
	ToolResultStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Tool status styles
	ToolSuccessStyle = lipgloss.NewStyle().
				Foreground(Secondary)

	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	ToolSpinnerStyle = lipgloss.NewStyle().
				Foreground(Primary)

	ToolCollapsedStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Italic(true)

	// Tool rendering - Claude Code style
	ToolBulletStyle       = lipgloss.NewStyle().Foreground(Warning)
	ToolNameStyle         = lipgloss.NewStyle().Bold(true).Foreground(Text)
	ToolArgsStyle         = lipgloss.NewStyle().Foreground(Muted)
	ToolResultPrefixStyle = lipgloss.NewStyle().Foreground(Muted)

	// Confirmation status styles
	ConfirmWaitStyle = lipgloss.NewStyle().
				Foreground(Warning)

	ConfirmApprovedStyle = lipgloss.NewStyle().
				Foreground(Secondary)

	ConfirmDeniedStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Italic(true)
)

// =============================================================================
// Input Styles
// =============================================================================

var (
	// PromptStyle for input prompts
	PromptStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Input - Claude Code style
	InputSeparatorStyle  = lipgloss.NewStyle().Foreground(Border)
	InputPromptCharStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	// LabelStyle for field labels
	LabelStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// RequiredStyle for required field indicator
	RequiredStyle = lipgloss.NewStyle().
			Foreground(Error)

	// HintStyle for keyboard hints
	HintStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// OptionStyle for unselected options
	OptionStyle = lipgloss.NewStyle().
			Foreground(Text)

	// SelectedOptionStyle for selected options
	SelectedOptionStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Bold(true)

	// DescriptionStyle for option descriptions
	DescriptionStyle = lipgloss.NewStyle().
				Foreground(Muted)

	// DisabledStyle for disabled state
	DisabledStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// AnswerStyle for completed field answers
	AnswerStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// AnswerLabelStyle for completed field labels
	AnswerLabelStyle = lipgloss.NewStyle().
				Foreground(Muted)

	// ConfirmLabelStyle for confirmation prompts (amber)
	ConfirmLabelStyle = lipgloss.NewStyle().
				Foreground(Warning).
				Bold(true)
)

// =============================================================================
// Header Styles
// =============================================================================

var (
	// HeaderStyle for header background (now used as footer)
	HeaderStyle = lipgloss.NewStyle().
			Background(Surface).
			Foreground(Text).
			Padding(0, 1)

	// HeaderModelStyle for model name
	HeaderModelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorTextIndigo))

	// HeaderTokenStyle for token count
	HeaderTokenStyle = lipgloss.NewStyle().
				Foreground(Secondary)

	// HeaderStreamStyle for streaming indicator
	HeaderStreamStyle = lipgloss.NewStyle().
				Foreground(Warning)

	// Footer styles
	FooterSeparatorStyle = lipgloss.NewStyle().Foreground(Muted)
	FooterLabelStyle     = lipgloss.NewStyle().Foreground(Muted)
)

// =============================================================================
// Layout Styles
// =============================================================================

var (
	// BorderStyle for panel borders
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border)

	// ModalStyle for modal dialogs
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Warning).
			Padding(1, 2)

	// ModalTitleStyle for modal titles
	ModalTitleStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	// StatusBarStyle for status bar
	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(ColorBackground)).
			Foreground(TextDim).
			Padding(0, 1)
)

// =============================================================================
// Indicators
// =============================================================================

var (
	// SpinnerStyle for loading spinners
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(Primary)

	// ThinkingStyle for thinking indicator
	ThinkingStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)
)

// =============================================================================
// Welcome/Help Styles
// =============================================================================

var (
	// WelcomeTitleStyle for welcome message title
	WelcomeTitleStyle = lipgloss.NewStyle().
				Foreground(Primary).
				Bold(true)

	// WelcomeSubtitleStyle for welcome message subtitle
	WelcomeSubtitleStyle = lipgloss.NewStyle().
				Foreground(Muted)

	// HelpTextStyle for help text
	HelpTextStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)
)

// =============================================================================
// Interim Message Styles
// =============================================================================

var (
	// InterimStyle for interim messages (field confirmations)
	InterimStyle = lipgloss.NewStyle().
			Foreground(Secondary)
)

// =============================================================================
// Helper Functions
// =============================================================================

// RolePrefix returns the styled prefix for a message role.
func RolePrefix(role string) string {
	switch role {
	case "user":
		return InputPromptCharStyle.Render("❯ ")
	case "assistant":
		return "" // No prefix — markdown content is enough
	case "system":
		return SystemStyle.Render("System: ")
	case "tool":
		return ToolBulletStyle.Render("⏺ ")
	default:
		return role + ": "
	}
}
