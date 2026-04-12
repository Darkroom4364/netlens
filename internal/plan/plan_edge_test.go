package plan

import (
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
	"gonum.org/v1/gonum/mat"
)

// ---------- helpers ----------

func makePair() *topology.Graph {
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddLink(0, 1)
	return g
}

func makeDisconnected() *topology.Graph {
	g := topology.New()
	// Component 1: 0 -- 1
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddLink(0, 1)
	// Component 2: 2 -- 3
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddNode(tomo.Node{ID: 3, Label: "D"})
	g.AddLink(2, 3)
	return g
}

// ---------- 1. Budget = 0 ----------

func TestPlanEdgeBudgetZero(t *testing.T) {
	g := makeTriangle()
	probes := RecommendProbes(g, nil, 0)
	if len(probes) != 0 {
		t.Errorf("budget=0 should produce 0 probes, got %d", len(probes))
	}
}

// ---------- 2. Budget > total possible paths ----------

func TestPlanEdgeBudgetExceedsPaths(t *testing.T) {
	g := makePair() // 1 link, 1 possible path
	probes := RecommendProbes(g, nil, 100)
	if len(probes) != 1 {
		t.Errorf("single-link topology with huge budget: expected 1 probe, got %d", len(probes))
	}
}

// ---------- 3. Fully determined system ----------

func TestPlanEdgeFullyDetermined(t *testing.T) {
	g := makeTriangle() // 3 links, 3 paths => full rank achievable

	// First get full rank.
	allProbes := RecommendProbes(g, nil, 10)
	totalRank := 0
	for _, p := range allProbes {
		totalRank += p.RankGain
	}
	if totalRank != 3 {
		t.Fatalf("triangle should reach rank 3, got %d", totalRank)
	}

	// Build a Problem that already has full rank.
	var paths []tomo.PathSpec
	var measurements []float64
	for _, p := range allProbes {
		linkIDs, ok := g.ShortestPath(p.Src, p.Dst)
		if !ok {
			t.Fatalf("no path %d->%d", p.Src, p.Dst)
		}
		paths = append(paths, tomo.PathSpec{Src: p.Src, Dst: p.Dst, LinkIDs: linkIDs})
		measurements = append(measurements, 1.0)
	}
	existing, err := tomo.BuildProblem(g, paths, measurements)
	if err != nil {
		t.Fatal(err)
	}

	// Now recommend more probes. Should return none since rank is already maximal.
	extra := RecommendProbes(g, existing, 10)
	if len(extra) != 0 {
		t.Errorf("fully determined system should need 0 more probes, got %d", len(extra))
	}
}

// ---------- 4. Disconnected topology ----------

func TestPlanEdgeDisconnected(t *testing.T) {
	g := makeDisconnected() // Two components: {0,1} and {2,3}
	probes := RecommendProbes(g, nil, 10)

	// Should get paths only within components (0-1) and (2-3).
	for _, p := range probes {
		sameComponent := (p.Src <= 1 && p.Dst <= 1) || (p.Src >= 2 && p.Dst >= 2)
		if !sameComponent {
			t.Errorf("probe crosses disconnected components: %d -> %d", p.Src, p.Dst)
		}
	}

	// 2 links, should achieve rank 2.
	totalRank := 0
	for _, p := range probes {
		totalRank += p.RankGain
	}
	if totalRank != 2 {
		t.Errorf("disconnected 2-link topology: expected rank 2, got %d", totalRank)
	}
}

// ---------- 5. Single-link topology ----------

func TestPlanEdgeSingleLink(t *testing.T) {
	g := makePair()
	probes := RecommendProbes(g, nil, 10)
	if len(probes) != 1 {
		t.Fatalf("single link: expected 1 probe, got %d", len(probes))
	}
	if probes[0].RankGain != 1 {
		t.Errorf("single link probe should have rank gain 1, got %d", probes[0].RankGain)
	}
}

// ---------- 6. Existing problem already at full rank ----------

func TestPlanEdgeExistingFullRank(t *testing.T) {
	g := makeChain() // 3 links
	nLinks := g.NumLinks()

	// Build a problem with all pairs paths to guarantee full rank.
	allPaths := g.AllPairsShortestPaths()
	measurements := make([]float64, len(allPaths))
	for i := range measurements {
		measurements[i] = 5.0
	}
	existing, err := tomo.BuildProblem(g, allPaths, measurements)
	if err != nil {
		t.Fatal(err)
	}

	// Verify we actually have full rank.
	if existing.Quality.Rank != nLinks {
		t.Fatalf("expected full rank %d, got %d", nLinks, existing.Quality.Rank)
	}

	probes := RecommendProbes(g, existing, 10)
	if len(probes) != 0 {
		t.Errorf("existing full-rank problem should need 0 probes, got %d", len(probes))
	}
}

// Keep mat import used.
var _ = (*mat.Dense)(nil)
