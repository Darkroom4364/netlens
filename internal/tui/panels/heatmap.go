package panels

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
)

// RenderHeatmap renders a sorted list of links with colored delay bars (worst first).
func RenderHeatmap(p *tomo.Problem, s *tomo.Solution, width, height int) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Heatmap \u2014 Link Delays"))
	b.WriteByte('\n')

	n := p.NumLinks()
	type entry struct {
		idx   int
		delay float64
		ident bool
	}
	entries := make([]entry, n)
	maxDelay := 0.0
	for i := 0; i < n; i++ {
		d := s.X.AtVec(i)
		entries[i] = entry{i, d, s.Identifiable[i]}
		if s.Identifiable[i] && d > maxDelay {
			maxDelay = d
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].delay > entries[j].delay })

	// Label takes ~18 chars ("  12→ 7: " + " 15.2ms"), leave rest for bar.
	barMax := width - 22
	if barMax < 4 {
		barMax = 4
	}

	visible := n
	if visible > height-2 {
		visible = height - 2
	}

	for i := 0; i < visible; i++ {
		e := entries[i]
		label := fmt.Sprintf("  %2d\u2192%2d: ", p.Links[e.idx].Src, p.Links[e.idx].Dst)
		suffix := fmt.Sprintf(" %.1fms", e.delay)

		barLen := 0
		if maxDelay > 0 {
			barLen = int(math.Round(float64(barMax) * e.delay / maxDelay))
		}
		bar := strings.Repeat("\u2588", barLen)

		var colored string
		switch {
		case !e.ident:
			colored = styles.Dim.Render(bar)
		case e.delay < 5:
			colored = styles.Green.Render(bar)
		case e.delay < 20:
			colored = styles.Yellow.Render(bar)
		default:
			colored = styles.Red.Render(bar)
		}
		b.WriteString(label + colored + suffix + "\n")
	}
	return b.String()
}
