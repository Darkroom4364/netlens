package format

import (
	"encoding/json"
	"io"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// JSONFormatter writes a Solution as machine-readable JSON.
type JSONFormatter struct{}

type jsonOutput struct {
	Method        string           `json:"method"`
	DurationMS    float64          `json:"duration_ms"`
	Residual      float64          `json:"residual"`
	MatrixQuality *jsonQuality     `json:"matrix_quality,omitempty"`
	Links         []jsonLinkResult `json:"links"`
}

type jsonQuality struct {
	Rank             int     `json:"rank"`
	NumLinks         int     `json:"num_links"`
	ConditionNumber  float64 `json:"condition_number"`
	IdentifiablePct  float64 `json:"identifiable_pct"`
}

type jsonLinkResult struct {
	ID            int     `json:"id"`
	Src           int     `json:"src"`
	Dst           int     `json:"dst"`
	DelayMS       float64 `json:"delay_ms"`
	ConfidenceMS  float64 `json:"confidence_ms"`
	Identifiable  bool    `json:"identifiable"`
}

func (f *JSONFormatter) Format(w io.Writer, p *tomo.Problem, s *tomo.Solution) error {
	out := jsonOutput{
		Method:     s.Method,
		DurationMS: float64(s.Duration.Microseconds()) / 1000.0,
		Residual:   s.Residual,
		Links:      make([]jsonLinkResult, 0, len(p.Links)),
	}

	if p.Quality != nil {
		out.MatrixQuality = &jsonQuality{
			Rank:            p.Quality.Rank,
			NumLinks:        p.Quality.NumLinks,
			ConditionNumber: p.Quality.ConditionNumber,
			IdentifiablePct: p.Quality.IdentifiableFrac * 100,
		}
	}

	for i, link := range p.Links {
		lr := jsonLinkResult{
			ID:  link.ID,
			Src: link.Src,
			Dst: link.Dst,
		}

		if s.X != nil && i < s.X.Len() {
			lr.DelayMS = s.X.AtVec(i)
		}
		if s.Confidence != nil && i < s.Confidence.Len() {
			lr.ConfidenceMS = s.Confidence.AtVec(i)
		}
		if i < len(s.Identifiable) {
			lr.Identifiable = s.Identifiable[i]
		}

		out.Links = append(out.Links, lr)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
