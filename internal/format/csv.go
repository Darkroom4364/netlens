package format

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/Darkroom4364/netlens/tomo"
)

// CSVFormatter writes a Solution as CSV suitable for spreadsheets.
type CSVFormatter struct{}

func (f *CSVFormatter) Format(w io.Writer, p *tomo.Problem, s *tomo.Solution) error {
	cw := csv.NewWriter(w)

	if err := cw.Write([]string{"link_id", "src", "dst", "delay_ms", "confidence_ms", "identifiable"}); err != nil {
		return err
	}

	for i, link := range p.Links {
		var delayMS, confMS float64
		var identifiable bool

		if s.X != nil && i < s.X.Len() {
			delayMS = s.X.AtVec(i)
		}
		if s.Confidence != nil && i < s.Confidence.Len() {
			confMS = s.Confidence.AtVec(i)
		}
		if i < len(s.Identifiable) {
			identifiable = s.Identifiable[i]
		}

		record := []string{
			fmt.Sprintf("%d", link.ID),
			fmt.Sprintf("%d", link.Src),
			fmt.Sprintf("%d", link.Dst),
			fmt.Sprintf("%.3f", delayMS),
			fmt.Sprintf("%.3f", confMS),
			fmt.Sprintf("%t", identifiable),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}
