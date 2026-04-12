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
)
