//go:build tui

package tui

import (
	"fmt"
	"strings"

	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/Darkroom4364/netlens/tomo"
)

// RenderTreeView renders the tree panel showing links grouped by source node.
func RenderTreeView(p *tomo.Problem, s *tomo.Solution, selected int, expanded map[int]bool, w, h int) string {
	if p == nil || s == nil {
		return "no data"
	}
	// Group links by source node.
	type nodeGroup struct {
		nodeID int
		links  []int
	}
	seen := map[int]*nodeGroup{}
	var order []int
	for i, l := range p.Links {
		g, ok := seen[l.Src]
		if !ok {
			g = &nodeGroup{nodeID: l.Src}
			seen[l.Src] = g
			order = append(order, l.Src)
		}
		g.links = append(g.links, i)
	}
	// Summary.
	congested := 0
	for i := 0; i < s.X.Len(); i++ {
		if s.X.AtVec(i) > 20 {
			congested++
		}
	}
	identPct := 0.0
	if p.Quality != nil {
		identPct = p.Quality.IdentifiableFrac * 100
	}
	summary := fmt.Sprintf(" %d links | %d congested | %.0f%% identifiable", p.NumLinks(), congested, identPct)
	rows := []string{styles.Title.Render(summary)}
	flatIdx := 0
	for _, nid := range order {
		g := seen[nid]
		label := fmt.Sprintf("node %d", nid)
		if p.Topo != nil {
			for _, n := range p.Topo.Nodes() {
				if n.ID == nid && n.Label != "" {
					label = n.Label
					break
				}
			}
		}
		arrow := "▶"
		if expanded[nid] {
			arrow = "▼"
		}
		header := fmt.Sprintf(" %s %s (%d links)", arrow, label, len(g.links))
		if flatIdx == selected {
			header = styles.Selected.Render(header)
		}
		rows = append(rows, header)
		flatIdx++
		if !expanded[nid] {
			continue
		}
		barW := w - 40
		if barW < 5 {
			barW = 5
		}
		for _, li := range g.links {
			l := p.Links[li]
			delay := s.X.AtVec(li)
			filled := int(delay / 50.0 * float64(barW))
			if filled > barW {
				filled = barW
			} else if filled < 0 {
				filled = 0
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
			delayStr := fmt.Sprintf("%.1fms", delay)
			switch {
			case delay > 20:
				delayStr = styles.Red.Render(delayStr) + " ⚠"
			case delay >= 5:
				delayStr = styles.Yellow.Render(delayStr)
			default:
				delayStr = styles.Green.Render(delayStr)
			}
			conf := ""
			if s.Confidence != nil {
				conf = fmt.Sprintf(" ±%.1fms", s.Confidence.AtVec(li))
			}
			cov := ""
			if p.Quality != nil && li < len(p.Quality.CoveragePerLink) {
				cov = fmt.Sprintf(" cov:%d", p.Quality.CoveragePerLink[li])
			}
			row := fmt.Sprintf("   →%d  %s %s%s%s", l.Dst, bar, delayStr, conf, cov)
			if flatIdx == selected {
				row = styles.Selected.Render(row)
			}
			rows = append(rows, row)
			flatIdx++
		}
	}
	// Scroll: keep selected visible.
	maxRows := h - 2
	if maxRows < 1 {
		maxRows = 1
	}
	if len(rows) > maxRows {
		start := 0
		if selected > maxRows-2 {
			start = selected - maxRows + 3
		}
		if start+maxRows > len(rows) {
			start = len(rows) - maxRows
		}
		if start < 0 {
			start = 0
		}
		rows = rows[start : start+maxRows]
	}
	return styles.Panel.Width(w - 2).Render(strings.Join(rows, "\n"))
}
