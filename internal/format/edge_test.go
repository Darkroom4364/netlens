package format

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"gonum.org/v1/gonum/mat"
)

// emptyProblem returns a Problem with 0 links and 0 paths (but valid matrices).
func emptyProblem() *tomo.Problem {
	A := mat.NewDense(1, 1, []float64{1})
	B := mat.NewVecDense(1, []float64{1})
	return &tomo.Problem{
		A:     A,
		B:     B,
		Links: nil, // 0 links in the slice
		Paths: nil,
	}
}

func emptySolution() *tomo.Solution {
	return &tomo.Solution{
		X:        nil,
		Method:   "test",
		Duration: time.Millisecond,
	}
}

func allFormatters() map[string]Formatter {
	return map[string]Formatter{
		"json": Get("json"),
		"csv":  Get("csv"),
		"dot":  Get("dot"),
	}
}

// TestEdgeNilSolutionX ensures nil Solution.X does not panic.
func TestEdgeNilSolutionX(t *testing.T) {
	p := emptyProblem()
	// Give it one link so formatters iterate.
	p.Links = []tomo.Link{{ID: 0, Src: 0, Dst: 1}}
	s := emptySolution() // X is nil

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeEmptyProblem ensures 0-link problem does not panic.
func TestEdgeEmptyProblem(t *testing.T) {
	p := emptyProblem() // no links
	s := emptySolution()

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeNilConfidence ensures nil Confidence does not panic.
func TestEdgeNilConfidence(t *testing.T) {
	p := emptyProblem()
	p.Links = []tomo.Link{{ID: 0, Src: 0, Dst: 1}}
	s := &tomo.Solution{
		X:          mat.NewVecDense(1, []float64{5.0}),
		Confidence: nil,
		Method:     "test",
		Duration:   time.Millisecond,
	}

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeVeryLongLabels ensures 500-character node labels do not panic.
func TestEdgeVeryLongLabels(t *testing.T) {
	longLabel := strings.Repeat("x", 500)
	p := emptyProblem()
	p.Links = []tomo.Link{{ID: 0, Src: 0, Dst: 1}}

	// DOT formatter uses Topo for node labels.
	p.Topo = &stubTopo{
		nodes: []tomo.Node{
			{ID: 0, Label: longLabel},
			{ID: 1, Label: longLabel},
		},
		links: p.Links,
	}

	s := &tomo.Solution{
		X:      mat.NewVecDense(1, []float64{1.0}),
		Method: "test",
		Duration: time.Millisecond,
	}

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeSpecialCharactersInLabels tests quotes, angle brackets, newlines in labels.
func TestEdgeSpecialCharactersInLabels(t *testing.T) {
	specialLabel := `"hello" <world> & 'foo'` + "\nnewline"
	p := emptyProblem()
	p.Links = []tomo.Link{{ID: 0, Src: 0, Dst: 1}}
	p.Topo = &stubTopo{
		nodes: []tomo.Node{
			{ID: 0, Label: specialLabel},
			{ID: 1, Label: specialLabel},
		},
		links: p.Links,
	}

	s := &tomo.Solution{
		X:      mat.NewVecDense(1, []float64{3.0}),
		Method: "test",
		Duration: time.Millisecond,
	}

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeNegativeDelayValues ensures negative delay values format correctly.
func TestEdgeNegativeDelayValues(t *testing.T) {
	p := emptyProblem()
	p.Links = []tomo.Link{{ID: 0, Src: 0, Dst: 1}}

	s := &tomo.Solution{
		X:      mat.NewVecDense(1, []float64{-42.5}),
		Method: "test",
		Duration: time.Millisecond,
	}

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeNaNInfInSolution ensures NaN and Inf values do not panic.
func TestEdgeNaNInfInSolution(t *testing.T) {
	p := emptyProblem()
	p.Links = []tomo.Link{
		{ID: 0, Src: 0, Dst: 1},
		{ID: 1, Src: 1, Dst: 2},
		{ID: 2, Src: 2, Dst: 3},
	}

	s := &tomo.Solution{
		X:      mat.NewVecDense(3, []float64{math.NaN(), math.Inf(1), math.Inf(-1)}),
		Method: "test",
		Duration: time.Millisecond,
	}

	for name, f := range allFormatters() {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := f.Format(&buf, p, s)
			// Either a returned error or non-empty output is acceptable;
			// the key requirement is no panic.
			if err != nil {
				t.Logf("%s: returned error (acceptable): %v", name, err)
				return
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", name)
			}
		})
	}
}

// TestEdgeGetNonexistent ensures Get returns nil for unknown format.
func TestEdgeGetNonexistent(t *testing.T) {
	if f := Get("nonexistent"); f != nil {
		t.Errorf("expected nil for nonexistent format, got %v", f)
	}
}

// TestEdgeGetEmpty ensures Get returns nil for empty string.
func TestEdgeGetEmpty(t *testing.T) {
	if f := Get(""); f != nil {
		t.Errorf("expected nil for empty format name, got %v", f)
	}
}

// stubTopo is a minimal Topology implementation for testing DOT output with labels.
type stubTopo struct {
	nodes []tomo.Node
	links []tomo.Link
}

func (s *stubTopo) NumNodes() int                      { return len(s.nodes) }
func (s *stubTopo) NumLinks() int                      { return len(s.links) }
func (s *stubTopo) Links() []tomo.Link                 { return s.links }
func (s *stubTopo) Nodes() []tomo.Node                 { return s.nodes }
func (s *stubTopo) Neighbors(int) []int                { return nil }
func (s *stubTopo) ShortestPath(int, int) ([]int, bool) { return nil, false }
func (s *stubTopo) AllPairsShortestPaths() []tomo.PathSpec { return nil }
