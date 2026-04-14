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
func RenderHeatmapView(p *tomo.Problem, s *tomo.Solution, selected int, w, h int) string {
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
		b.WriteString(fmt.Sprintf("%*d", colW, id))
	}
	b.WriteByte('\n')

	// Matrix rows.
	reset := "\033[0m"
	for _, src := range nodes {
		b.WriteString(fmt.Sprintf("%*d", labelW, src))
		for _, dst := range nodes {
			d, ok := adj[[2]int{src, dst}]
			if !ok {
				b.WriteString(fmt.Sprintf("%*s", colW, "·"))
				continue
			}
			var bg string
			fg := "\033[97m" // white text
			switch {
			case d < 2:
				bg = "\033[48;2;46;160;67m"
			case d <= 10:
				bg = "\033[48;2;200;170;40m"
				fg = "\033[30m" // black on yellow
			default:
				bg = "\033[48;2;200;50;50m"
			}
			cell := fmt.Sprintf("%*.1f", colW, d)
			b.WriteString(bg + fg + cell + reset)
		}
		b.WriteByte('\n')
	}

	// Legend.
	b.WriteString("\n")
	b.WriteString("\033[48;2;46;160;67m\033[97m <2ms " + reset + " ")
	b.WriteString("\033[48;2;200;170;40m\033[30m 2-10ms " + reset + " ")
	b.WriteString("\033[48;2;200;50;50m\033[97m >10ms " + reset + " ")
	b.WriteString("· no link\n")

	return styles.Panel.MaxWidth(w).MaxHeight(h).Render(
		lipgloss.NewStyle().Render(b.String()),
	)
}
