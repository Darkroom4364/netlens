//go:build tui

package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/Darkroom4364/netlens/tomo"
)

// Sort modes for tree view.
const (
	SortDefault        = 0
	SortDelayDesc      = 1
	SortDelayAsc       = 2
	SortNameAlpha      = 3
	SortCoverageDesc   = 4
)

type nodeGroup struct {
	nodeID int
	links  []int
}

// buildGroups groups links by source node, returning the map and insertion order.
func buildGroups(p *tomo.Problem) (map[int]*nodeGroup, []int) {
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
	return seen, order
}

// nodeLabel returns the display label for a node ID.
func nodeLabel(p *tomo.Problem, nid int) string {
	label := fmt.Sprintf("node %d", nid)
	if p.Topo != nil {
		for _, n := range p.Topo.Nodes() {
			if n.ID == nid && n.Label != "" {
				label = n.Label
				break
			}
		}
	}
	return label
}

// RenderTreeView renders the tree panel showing links grouped by source node.
func RenderTreeView(p *tomo.Problem, s *tomo.Solution, selected int, expanded map[int]bool, filterText string, sortMode int, w, h int) string {
	if p == nil || s == nil {
		return "no data"
	}
	seen, order := buildGroups(p)

	// Filter by node label.
	if filterText != "" {
		ft := strings.ToLower(filterText)
		var filtered []int
		for _, nid := range order {
			if strings.Contains(strings.ToLower(nodeLabel(p, nid)), ft) {
				filtered = append(filtered, nid)
			}
		}
		order = filtered
	}

	// Sort nodes.
	switch sortMode {
	case SortDelayDesc:
		sort.Slice(order, func(i, j int) bool {
			return maxGroupDelay(seen[order[i]], s) > maxGroupDelay(seen[order[j]], s)
		})
	case SortDelayAsc:
		sort.Slice(order, func(i, j int) bool {
			return maxGroupDelay(seen[order[i]], s) < maxGroupDelay(seen[order[j]], s)
		})
	case SortNameAlpha:
		sort.Slice(order, func(i, j int) bool {
			return nodeLabel(p, order[i]) < nodeLabel(p, order[j])
		})
	case SortCoverageDesc:
		sort.Slice(order, func(i, j int) bool {
			return sumGroupCoverage(seen[order[i]], p) > sumGroupCoverage(seen[order[j]], p)
		})
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
		label := nodeLabel(p, nid)
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

func maxGroupDelay(g *nodeGroup, s *tomo.Solution) float64 {
	mx := 0.0
	for _, li := range g.links {
		if v := s.X.AtVec(li); v > mx {
			mx = v
		}
	}
	return mx
}

func sumGroupCoverage(g *nodeGroup, p *tomo.Problem) int {
	total := 0
	if p.Quality == nil {
		return 0
	}
	for _, li := range g.links {
		if li < len(p.Quality.CoveragePerLink) {
			total += p.Quality.CoveragePerLink[li]
		}
	}
	return total
}

// CursorToLinkIdx maps a flat cursor position (used in the tree view) to a
// link index. Returns -1 if the cursor is on the summary row or a node header.
func CursorToLinkIdx(p *tomo.Problem, cursor int, expanded map[int]bool) int {
	if p == nil {
		return -1
	}
	seen, order := buildGroups(p)
	pos := 0 // summary row
	if cursor == pos {
		return -1
	}
	pos++
	for _, nid := range order {
		if cursor == pos {
			return -1 // node header
		}
		pos++
		if expanded[nid] {
			for _, li := range seen[nid].links {
				if cursor == pos {
					return li
				}
				pos++
			}
		}
	}
	return -1
}
