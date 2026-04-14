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
	name := fmt.Sprintf("%s → %s", nodes[link.Src].Label, nodes[link.Dst].Label)

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

	ident := "yes"
	if !p.Quality.IsIdentifiable(linkIdx) {
		ident = "no"
	}
	paths := p.Quality.CoveragePerLink[linkIdx]

	line := fmt.Sprintf("%s  %.2fms ±%.2f  σ=%.1f  paths=%d  ident=%s", name, delay, conf, sigma, paths, ident)
	if delay > 20 {
		line += "  " + alertStyle.Render("⚠ CONGESTED")
	}
	return detailStyle.Width(w - 2).Render(line)
}

// RenderStatusBar renders the bottom status line.
func RenderStatusBar(p *tomo.Problem, s *tomo.Solution, mode viewMode, solver string, w int) string {
	hint := "[h]heatmap"
	if mode == viewHeatmap {
		hint = "[t]tree"
	}
	identPct := 0.0
	if p.Quality != nil {
		identPct = p.Quality.IdentifiableFrac * 100
	}
	left := fmt.Sprintf(" %s [q]quit  solver=%s", hint, solver)
	right := fmt.Sprintf("rank %d/%d  ident %.0f%% ", p.Quality.Rank, p.NumLinks(), identPct)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return statusStyle.Width(w).Render(left + strings.Repeat(" ", gap) + right)
}
