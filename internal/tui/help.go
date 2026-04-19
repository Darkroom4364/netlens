//go:build tui

package tui

import (
	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// RenderHelpOverlay renders a centered help box listing all keybindings.
func RenderHelpOverlay(w, h int) string {
	content := `Keybindings

j/k, ↑/↓    Navigate links
Enter        Expand/collapse node
h            Heatmap view
t            Tree view
Tab          Cycle view
/            Filter by node name
s            Cycle sort mode
m            Cycle solver
?            Toggle this help
q            Quit`

	box := styles.Panel.Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
