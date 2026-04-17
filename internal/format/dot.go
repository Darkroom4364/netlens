package format

import (
	"fmt"
	"io"

	"github.com/Darkroom4364/netlens/internal/style"
	"github.com/Darkroom4364/netlens/tomo"
)

// DOTFormatter writes a Solution as a GraphViz DOT graph with color-coded links.
// Colors: green (<2ms), yellow (2-10ms), red (>10ms).
// Unidentifiable links are rendered with dashed style.
type DOTFormatter struct{}

func (f *DOTFormatter) Format(w io.Writer, p *tomo.Problem, s *tomo.Solution) error {
	if _, err := fmt.Fprintln(w, "graph netlens {"); err != nil {
		return err
	}

	// Emit nodes.
	if p.Topo != nil {
		for _, node := range p.Topo.Nodes() {
			label := node.Label
			if label == "" {
				label = fmt.Sprintf("%d", node.ID)
			}
			if _, err := fmt.Fprintf(w, "  %d [label=%q]\n", node.ID, label); err != nil {
				return err
			}
		}
	}

	// Emit edges with delay labels and color coding.
	for i, link := range p.Links {
		var delayMS float64
		var identifiable bool

		if s.X != nil && i < s.X.Len() {
			delayMS = s.X.AtVec(i)
		}
		if i < len(s.Identifiable) {
			identifiable = s.Identifiable[i]
		}

		color := delayColor(delayMS)
		style := "solid"
		if !identifiable {
			style = "dashed"
		}

		if _, err := fmt.Fprintf(w, "  %d -- %d [label=\"%.1fms\" color=%q style=%q]\n",
			link.Src, link.Dst, delayMS, color, style); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w, "}")
	return err
}

func delayColor(ms float64) string {
	switch {
	case ms < style.DelayLowMS:
		return "green"
	case ms <= style.DelayHighMS:
		return "yellow"
	default:
		return "red"
	}
}
