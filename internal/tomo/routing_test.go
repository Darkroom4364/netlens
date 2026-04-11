package tomo_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "topologies")
}

func TestBuildProblemFromTopology(t *testing.T) {
	g, err := topology.LoadGraphML(filepath.Join(testdataDir(), "abilene.graphml"))
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}

	// Create ground truth: link delays in ms
	nLinks := g.NumLinks()
	groundTruth := make([]float64, nLinks)
	for i := range groundTruth {
		groundTruth[i] = float64(i+1) * 2.0 // 2, 4, 6, ... ms
	}

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	// Abilene: 11 nodes, 14 links, 55 paths (all-pairs)
	if p.NumPaths() != 55 {
		t.Errorf("NumPaths = %d, want 55", p.NumPaths())
	}
	if p.NumLinks() != 14 {
		t.Errorf("NumLinks = %d, want 14", p.NumLinks())
	}

	// Verify b = A * x_true
	for i := 0; i < p.NumPaths(); i++ {
		var expected float64
		for _, linkID := range p.Paths[i].LinkIDs {
			expected += groundTruth[linkID]
		}
		got := p.B.AtVec(i)
		if abs(got-expected) > 1e-10 {
			t.Errorf("path %d: b=%f, expected=%f", i, got, expected)
		}
	}

	t.Logf("Abilene: %d paths × %d links, rank=%d, cond=%.1f, identifiable=%.0f%%",
		p.NumPaths(), p.NumLinks(), p.Quality.Rank, p.Quality.ConditionNumber,
		p.Quality.IdentifiableFrac*100)
	t.Logf("Unidentifiable links: %v", p.Quality.UnidentifiableLinks)
}

func TestMatrixQuality(t *testing.T) {
	// Simple 3-node triangle: fully identifiable
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddLink(0, 1) // link 0
	g.AddLink(1, 2) // link 1
	g.AddLink(0, 2) // link 2

	groundTruth := []float64{5.0, 3.0, 7.0}
	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	// 3 nodes → 3 paths, 3 links → should be fully determined
	if p.Quality.Rank != 3 {
		t.Errorf("rank = %d, want 3 (fully determined)", p.Quality.Rank)
	}
	if len(p.Quality.UnidentifiableLinks) != 0 {
		t.Errorf("unidentifiable = %v, want none", p.Quality.UnidentifiableLinks)
	}
	if p.Quality.IdentifiableFrac != 1.0 {
		t.Errorf("identifiable frac = %f, want 1.0", p.Quality.IdentifiableFrac)
	}

	t.Logf("Triangle: rank=%d, cond=%.2f, coverage=%v",
		p.Quality.Rank, p.Quality.ConditionNumber, p.Quality.CoveragePerLink)
}

func TestUnidentifiableLinks(t *testing.T) {
	// Linear chain: A--B--C--D
	// Only 3 paths (A-B, A-C, A-D) from endpoint A to all others
	// With only paths from one endpoint, some links share the same
	// path pattern and may be unidentifiable.
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddNode(tomo.Node{ID: 3, Label: "D"})
	g.AddLink(0, 1) // link 0
	g.AddLink(1, 2) // link 1
	g.AddLink(2, 3) // link 2

	// Use all-pairs: should give 6 paths for 3 links → overdetermined
	groundTruth := []float64{5.0, 3.0, 7.0}
	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	t.Logf("Chain: %d paths × %d links, rank=%d, cond=%.2f",
		p.NumPaths(), p.NumLinks(), p.Quality.Rank, p.Quality.ConditionNumber)

	// All-pairs on a chain: 6 paths, 3 links
	// A-B: [0], A-C: [0,1], A-D: [0,1,2], B-C: [1], B-D: [1,2], C-D: [2]
	// The routing matrix has rank 3 (full column rank)
	if p.Quality.Rank != 3 {
		t.Errorf("rank = %d, want 3", p.Quality.Rank)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
