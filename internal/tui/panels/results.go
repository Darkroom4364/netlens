package panels

import (
	"fmt"
	"strings"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
)

// RenderResults renders detail for the selected link.
func RenderResults(p *tomo.Problem, s *tomo.Solution, selected int, width, height int) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Link Detail"))
	b.WriteByte('\n')

	if selected < 0 || selected >= p.NumLinks() {
		b.WriteString("No link selected.\n")
		return b.String()
	}

	link := p.Links[selected]
	delay := s.X.AtVec(selected)
	ident := s.Identifiable[selected]

	b.WriteString(fmt.Sprintf("Link %d: %d → %d\n", link.ID, link.Src, link.Dst))
	b.WriteString(fmt.Sprintf("Delay:  %.2f ms\n", delay))

	if s.Confidence != nil {
		b.WriteString(fmt.Sprintf("CI ±:   %.2f ms\n", s.Confidence.AtVec(selected)))
	}

	if ident {
		b.WriteString(styles.Green.Render("Identifiable: yes") + "\n")
	} else {
		b.WriteString(styles.Red.Render("Identifiable: no") + "\n")
	}

	if p.Quality != nil {
		cov := p.Quality.CoveragePerLink[selected]
		b.WriteString(fmt.Sprintf("Coverage:     %d paths\n", cov))
	}

	// List paths through this link.
	b.WriteString("\nPaths through link:\n")
	count := 0
	for _, ps := range p.Paths {
		for _, lid := range ps.LinkIDs {
			if lid == selected {
				b.WriteString(fmt.Sprintf("  %d → %d\n", ps.Src, ps.Dst))
				count++
				break
			}
		}
		if count >= height-10 {
			b.WriteString("  ...\n")
			break
		}
	}
	if count == 0 {
		b.WriteString("  (none)\n")
	}

	return b.String()
}
