//go:build tui

package panels

import (
	"fmt"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
)

// RenderStatusbar renders a single-line status bar.
func RenderStatusbar(p *tomo.Problem, s *tomo.Solution, width int) string {
	identPct := 0.0
	if p.Quality != nil {
		identPct = p.Quality.IdentifiableFrac * 100
	}

	rank := 0
	if p.Quality != nil {
		rank = p.Quality.Rank
	}

	text := fmt.Sprintf(" %s | %s | rank %d/%d | identifiable %.0f%% | q quit ↑↓ select",
		s.Method, s.Duration.Round(1e6), rank, p.NumLinks(), identPct)

	return styles.Status.Width(width).Render(text)
}
