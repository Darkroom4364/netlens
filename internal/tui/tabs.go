//go:build tui

package tui

import (
	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// renderTabBar renders a tab bar showing Tree and Heatmap tabs.
func renderTabBar(active viewMode, w int) string {
	tree := "Tree"
	heatmap := "Heatmap"

	if active == viewTree {
		tree = styles.TabActive.Render(tree)
		heatmap = styles.TabInactive.Render(heatmap)
	} else {
		tree = styles.TabInactive.Render(tree)
		heatmap = styles.TabActive.Render(heatmap)
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, tree, heatmap)
	return lipgloss.NewStyle().Width(w).Render(bar)
}
