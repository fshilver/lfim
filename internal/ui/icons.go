package ui

// Status icons for each issue status
const (
	IconStatusOpen        = "○"
	IconStatusAnalyzed    = "◐"
	IconStatusPlanned     = "●"
	IconStatusImplemented = "◉"
	IconStatusClosed      = "✓"
	IconStatusInvalid     = "✗"
	IconStatusUnknown     = "?"
)

// Type icons for each issue type
const (
	IconTypeFeature  = "💡"
	IconTypeBug      = "💥"
	IconTypeRefactor = "🔧"
	IconTypeUnknown  = "❓"
)

// UI icons for various UI elements
const (
	IconConfirm = "⚠️ "
	IconSuccess = "✓"
	IconInput   = "✎"
	IconCommit  = "📝"
)

// Checkbox icons
const (
	IconCheckboxChecked   = "◉"
	IconCheckboxUnchecked = "○"
	IconRecommendedBadge  = "★"
)

// SpinnerFrames for processing animation
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
