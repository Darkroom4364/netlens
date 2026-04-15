//go:build tui

package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/Darkroom4364/netlens/tomo"
)

// RenderHeatmapView renders a matrix heatmap of per-link delays between node pairs.
func RenderHeatmapView(p *tomo.Problem, s *tomo.Solution, selected int, filterText string, w, h int) string {
	// Build adjacency: (src,dst) -> delay.
	adj := make(map[[2]int]float64)
	nodeSet := make(map[int]bool)
	for i, l := range p.Links {
		adj[[2]int{l.Src, l.Dst}] = s.X.AtVec(i)
		nodeSet[l.Src] = true
		nodeSet[l.Dst] = true
	}
	nodes := make([]int, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Ints(nodes)

	// Filter nodes by filterText (case-insensitive match on link src/dst labels).
	if filterText != "" && p.Topo != nil {
		labelOf := make(map[int]string)
		for _, nd := range p.Topo.Nodes() {
			labelOf[nd.ID] = nd.Label
		}
		matchNodes := make(map[int]bool)
		lower := strings.ToLower(filterText)
		for _, l := range p.Links {
			if strings.Contains(strings.ToLower(labelOf[l.Src]), lower) ||
				strings.Contains(strings.ToLower(labelOf[l.Dst]), lower) {
				matchNodes[l.Src] = true
				matchNodes[l.Dst] = true
			}
		}
		filtered := nodes[:0]
		for _, n := range nodes {
			if matchNodes[n] {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	// Determine highlighted nodes from selected link.
	var highlightSrc, highlightDst int = -1, -1
	if selected >= 0 && selected < len(p.Links) {
		highlightSrc = p.Links[selected].Src
		highlightDst = p.Links[selected].Dst
	}
	boldStyle := lipgloss.NewStyle().Bold(true)

	n := len(nodes)
	colW := 6 // width per cell (%5.1f + space)
	labelW := 4
	// Auto-fit: shrink colW if needed.
	if labelW+n*colW > w-4 {
		colW = (w - 4 - labelW) / n
		if colW < 4 {
			colW = 4
		}
	}

	var b strings.Builder

	// Header row.
	b.WriteString(strings.Repeat(" ", labelW))
	for _, id := range nodes {
		hdr := fmt.Sprintf("%*d", colW, id)
		if id == highlightSrc || id == highlightDst {
			hdr = boldStyle.Render(hdr)
		}
		b.WriteString(hdr)
	}
	b.WriteByte('\n')

	// Cell styles using lipgloss (respects NO_COLOR).
	greenCell := lipgloss.NewStyle().Background(lipgloss.Color("#2EA043")).Foreground(lipgloss.Color("#FFFFFF"))
	yellowCell := lipgloss.NewStyle().Background(lipgloss.Color("#C8AA28")).Foreground(lipgloss.Color("#000000"))
	redCell := lipgloss.NewStyle().Background(lipgloss.Color("#C83232")).Foreground(lipgloss.Color("#FFFFFF"))

	// Matrix rows.
	for _, src := range nodes {
		rowLabel := fmt.Sprintf("%*d", labelW, src)
		if src == highlightSrc || src == highlightDst {
			rowLabel = boldStyle.Render(rowLabel)
		}
		b.WriteString(rowLabel)
		for _, dst := range nodes {
			d, ok := adj[[2]int{src, dst}]
			if !ok {
				b.WriteString(fmt.Sprintf("%*s", colW, "·"))
				continue
			}
			cell := fmt.Sprintf("%*.1f", colW, d)
			switch {
			case d < 2:
				b.WriteString(greenCell.Render(cell))
			case d <= 10:
				b.WriteString(yellowCell.Render(cell))
			default:
				b.WriteString(redCell.Render(cell))
			}
		}
		b.WriteByte('\n')
	}

	// Legend.
	b.WriteString("\n")
	b.WriteString(greenCell.Render(" <2ms ") + " ")
	b.WriteString(yellowCell.Render(" 2-10ms ") + " ")
	b.WriteString(redCell.Render(" >10ms ") + " ")
	b.WriteString("· no link\n")

	return styles.Panel.MaxWidth(w).MaxHeight(h).Render(
		lipgloss.NewStyle().Render(b.String()),
	)
}
