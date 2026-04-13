//go:build tui

package panels

import (
	"fmt"
	"strings"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
)

// RenderTopology renders a sorted list of links with delay values and health coloring.
func RenderTopology(p *tomo.Problem, s *tomo.Solution, selected int, width, height int) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Topology — Links"))
	b.WriteByte('\n')

	n := p.NumLinks()
	for i := 0; i < n && i < height-3; i++ {
		delay := s.X.AtVec(i)
		identifiable := s.Identifiable[i]

		label := fmt.Sprintf(" %3d  %d→%d  %8.2fms", i, p.Links[i].Src, p.Links[i].Dst, delay)

		var line string
		switch {
		case !identifiable:
			line = styles.Dim.Render("? " + label)
		case delay < 5:
			line = styles.Green.Render("● " + label)
		case delay < 20:
			line = styles.Yellow.Render("● " + label)
		default:
			line = styles.Red.Render("● " + label)
		}

		if i == selected {
			line = styles.Selected.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
