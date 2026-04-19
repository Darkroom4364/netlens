//go:build tui

package styles

import "github.com/charmbracelet/lipgloss"

var (
	Green    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Yellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Red      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	Dim      = lipgloss.NewStyle().Faint(true)
	Selected = lipgloss.NewStyle().Bold(true).Reverse(true)
	Title    = lipgloss.NewStyle().Bold(true).Underline(true)
	Panel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	Status   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))

	// Wizard styles.
	WizardTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	WizardSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	WizardError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// Tab bar styles.
	TabActive   = lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 2)
	TabInactive = lipgloss.NewStyle().Faint(true).Padding(0, 2)
)
