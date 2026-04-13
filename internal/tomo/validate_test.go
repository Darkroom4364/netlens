package tomo

import (
	"testing"

	"gonum.org/v1/gonum/mat"
)

type valTopo struct {
	nodes []Node
	links []Link
}

func (s *valTopo) NumNodes() int                       { return len(s.nodes) }
func (s *valTopo) NumLinks() int                       { return len(s.links) }
func (s *valTopo) Links() []Link                       { return s.links }
func (s *valTopo) Nodes() []Node                       { return s.nodes }
func (s *valTopo) Neighbors(int) []int                 { return nil }
func (s *valTopo) ShortestPath(int, int) ([]int, bool) { return nil, false }
func (s *valTopo) AllPairsShortestPaths() []PathSpec   { return nil }

func TestValidateDelays(t *testing.T) {
	// Zurich (47.37,8.54) → London (51.51,-0.13) ≈ 779 km → lb ≈ 3.9 ms
	nodes := []Node{
		{ID: 0, Latitude: 47.37, Longitude: 8.54},
		{ID: 1, Latitude: 51.51, Longitude: -0.13},
		{ID: 2}, // no coords
	}
	links := []Link{{ID: 0, Src: 0, Dst: 1}, {ID: 1, Src: 1, Dst: 2}}
	topo := &valTopo{nodes: nodes, links: links}

	t.Run("above_bound", func(t *testing.T) {
		sol := &Solution{X: mat.NewVecDense(2, []float64{10, 5})}
		if v := ValidateDelays(sol, topo); len(v) != 0 {
			t.Fatalf("expected 0 violations, got %d", len(v))
		}
	})
	t.Run("negative", func(t *testing.T) {
		sol := &Solution{X: mat.NewVecDense(2, []float64{-1, 5})}
		v := ValidateDelays(sol, topo)
		if len(v) != 1 || v[0].Estimated != -1 {
			t.Fatalf("expected negative violation, got %+v", v)
		}
	})
	t.Run("below_sol", func(t *testing.T) {
		sol := &Solution{X: mat.NewVecDense(2, []float64{0.5, 5})}
		v := ValidateDelays(sol, topo)
		if len(v) != 1 || v[0].LowerBound == 0 {
			t.Fatalf("expected speed-of-light violation, got %+v", v)
		}
	})
}
