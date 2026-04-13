package plan

import (
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
	"gonum.org/v1/gonum/mat"
)

// makeTriangle builds a 3-node triangle: 0-1, 1-2, 0-2 (3 links).
func makeTriangle() *topology.Graph {
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(0, 2)
	return g
}

// makeChain builds a 4-node chain: 0-1-2-3 (3 links).
func makeChain() *topology.Graph {
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddNode(tomo.Node{ID: 3, Label: "D"})
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(2, 3)
	return g
}

func TestTriangleFullRank(t *testing.T) {
	g := makeTriangle()
	probes := RecommendProbes(g, nil, 10)

	if len(probes) != 3 {
		t.Fatalf("triangle: expected 3 probes for full rank, got %d", len(probes))
	}

	// Each probe should contribute rank gain of 1.
	for i, p := range probes {
		if p.RankGain != 1 {
			t.Errorf("probe %d: expected rank gain 1, got %d", i, p.RankGain)
		}
	}

	// Final rank should be 3 (= number of links).
	totalRank := 0
	for _, p := range probes {
		totalRank += p.RankGain
	}
	if totalRank != 3 {
		t.Errorf("triangle: expected total rank 3, got %d", totalRank)
	}
}

func TestChainCoversAllLinks(t *testing.T) {
	g := makeChain()
	probes := RecommendProbes(g, nil, 10)

	// Chain has 3 links. We need rank 3.
	totalRank := 0
	for _, p := range probes {
		totalRank += p.RankGain
	}
	if totalRank != 3 {
		t.Errorf("chain: expected total rank 3, got %d", totalRank)
	}

	// Verify all links are covered: build the routing matrix from selected probes
	// and check that rank equals number of links.
	nLinks := g.NumLinks()
	var rows [][]float64
	for _, p := range probes {
		linkIDs, ok := g.ShortestPath(p.Src, p.Dst)
		if !ok {
			t.Fatalf("no path from %d to %d", p.Src, p.Dst)
		}
		row := make([]float64, nLinks)
		for _, lid := range linkIDs {
			row[lid] = 1.0
		}
		rows = append(rows, row)
	}

	rank := computeRank(rows, nLinks)
	if rank != nLinks {
		t.Errorf("chain: probes achieve rank %d, need %d for full coverage", rank, nLinks)
	}
}

func TestWithExistingProblem(t *testing.T) {
	g := makeTriangle()

	// Create an existing problem with just one path (0->1).
	linkIDs, ok := g.ShortestPath(0, 1)
	if !ok {
		t.Fatal("no path 0->1")
	}
	paths := []tomo.PathSpec{{Src: 0, Dst: 1, LinkIDs: linkIDs}}
	measurements := []float64{10.0}

	existing, err := tomo.BuildProblem(g, paths, measurements)
	if err != nil {
		t.Fatalf("build problem: %v", err)
	}

	// With one path already measured, we should need 2 more for full rank.
	probes := RecommendProbes(g, existing, 10)

	if len(probes) != 2 {
		t.Fatalf("with existing: expected 2 more probes, got %d", len(probes))
	}

	// Verify combined rank is 3.
	nLinks := g.NumLinks()
	// Start with existing A rows.
	m, _ := existing.A.Dims()
	var rows [][]float64
	for i := 0; i < m; i++ {
		row := make([]float64, nLinks)
		for j := 0; j < nLinks; j++ {
			row[j] = existing.A.At(i, j)
		}
		rows = append(rows, row)
	}
	// Add recommended probe rows.
	for _, p := range probes {
		lid, ok := g.ShortestPath(p.Src, p.Dst)
		if !ok {
			t.Fatalf("no path %d->%d", p.Src, p.Dst)
		}
		row := make([]float64, nLinks)
		for _, l := range lid {
			row[l] = 1.0
		}
		rows = append(rows, row)
	}

	rank := computeRank(rows, nLinks)
	if rank != nLinks {
		t.Errorf("combined rank %d, expected %d", rank, nLinks)
	}
}

func TestBudgetZero(t *testing.T) {
	g := makeTriangle()
	probes := RecommendProbes(g, nil, 0)
	if len(probes) != 0 {
		t.Errorf("budget 0: expected no probes, got %d", len(probes))
	}
}

func TestNilTopology(t *testing.T) {
	// Empty graph with no links.
	g := topology.New()
	probes := RecommendProbes(g, nil, 10)
	if len(probes) != 0 {
		t.Errorf("empty graph: expected no probes, got %d", len(probes))
	}
}

// Ensure mat import is used (needed for existing.A access).
var _ = (*mat.Dense)(nil)
