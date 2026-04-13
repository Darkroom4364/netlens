package format

import (
	"io"

	"github.com/Darkroom4364/netlens/tomo"
)

// Formatter writes a Solution in a specific output format.
type Formatter interface {
	Format(w io.Writer, p *tomo.Problem, s *tomo.Solution) error
}

// Get returns a Formatter by name. Supported: "json", "csv", "dot".
// Returns nil if the name is not recognized.
func Get(name string) Formatter {
	switch name {
	case "json":
		return &JSONFormatter{}
	case "csv":
		return &CSVFormatter{}
	case "dot":
		return &DOTFormatter{}
	default:
		return nil
	}
}
