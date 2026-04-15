//go:build tui

package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Darkroom4364/netlens/tomo"
)

var (
	detailStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	alertStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // red
	statusStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252"))
)

// RenderDetailBar renders link detail for the selected link index.
func RenderDetailBar(p *tomo.Problem, s *tomo.Solution, linkIdx int, w int) string {
	if linkIdx < 0 || linkIdx >= p.NumLinks() {
		return detailStyle.Width(w - 2).Render("No link selected")
	}
	link := p.Links[linkIdx]
	nodes := p.Topo.Nodes()
	srcLabel, dstLabel := fmt.Sprintf("%d", link.Src), fmt.Sprintf("%d", link.Dst)
	for _, n := range nodes {
		if n.ID == link.Src {
			srcLabel = n.Label
		}
		if n.ID == link.Dst {
			dstLabel = n.Label
		}
	}
	name := fmt.Sprintf("%s → %s", srcLabel, dstLabel)

	delay := s.X.AtVec(linkIdx)
	conf := 0.0
	if s.Confidence != nil {
		conf = s.Confidence.AtVec(linkIdx)
	}

	// σ deviation from mean
	n := s.X.Len()
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += s.X.AtVec(i)
	}
	mean := sum / float64(n)
	sqSum := 0.0
	for i := 0; i < n; i++ {
		d := s.X.AtVec(i) - mean
		sqSum += d * d
	}
	stddev := math.Sqrt(sqSum / float64(n))
	sigma := 0.0
	if stddev > 0 {
		sigma = (delay - mean) / stddev
	}

	ident := "unknown"
	paths := 0
	if p.Quality != nil {
		if p.Quality.IsIdentifiable(linkIdx) {
			ident = "yes"
		} else {
			ident = "no"
		}
		if linkIdx < len(p.Quality.CoveragePerLink) {
			paths = p.Quality.CoveragePerLink[linkIdx]
		}
	}

	line := fmt.Sprintf("%s  %.2fms ±%.2f  σ=%.1f  paths=%d  ident=%s", name, delay, conf, sigma, paths, ident)
	if delay > 20 {
		line += "  " + alertStyle.Render("⚠ CONGESTED")
	}
	return detailStyle.Width(w - 2).Render(line)
}

// RenderStatusBar renders the bottom status line.
func RenderStatusBar(p *tomo.Problem, s *tomo.Solution, mode viewMode, solver string, filtering bool, filterText string, sortMode int, solveErr string, w int) string {
	hint := "[h]heatmap"
	if mode == viewHeatmap {
		hint = "[t]tree"
	}
	identPct := 0.0
	if p.Quality != nil {
		identPct = p.Quality.IdentifiableFrac * 100
	}
	sortNames := []string{"default", "delay↓", "delay↑", "name", "coverage"}
	sortLabel := sortNames[0]
	if sortMode >= 0 && sortMode < len(sortNames) {
		sortLabel = sortNames[sortMode]
	}
	left := fmt.Sprintf(" %s [/]filter [s]sort:%s [m]solver [?]help [q]quit  solver=%s", hint, sortLabel, solver)
	if filtering {
		left = fmt.Sprintf(" FILTER: %s█  (Enter=apply  Esc=cancel)", filterText)
	}
	if solveErr != "" {
		left = " " + alertStyle.Render("solve error: "+solveErr)
	}
	rank := 0
	if p.Quality != nil {
		rank = p.Quality.Rank
	}
	right := fmt.Sprintf("rank %d/%d  ident %.0f%% ", rank, p.NumLinks(), identPct)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return statusStyle.Width(w).Render(left + strings.Repeat(" ", gap) + right)
}
