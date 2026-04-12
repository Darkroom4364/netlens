package bench

import (
	"testing"

	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

// minimalTopo builds a 2-node, 1-link graph — the smallest possible topology.
func minimalTopo() *topology.Graph {
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddLink(0, 1)
	return g
}

func defaultSolvers() []tomo.Solver {
	return []tomo.Solver{
		&tomo.NNLSSolver{MaxIter: 500},
	}
}

func TestEdgeMinimalTopology(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.CongestionLinks = 0

	results, err := RunBenchmark("minimal", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark on minimal topo: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	r := results[0]
	if r.NumNodes != 2 || r.NumLinks != 1 {
		t.Errorf("expected 2 nodes / 1 link, got %d / %d", r.NumNodes, r.NumLinks)
	}
	out := FormatResults(results)
	if len(out) == 0 {
		t.Error("FormatResults returned empty string")
	}
}

func TestEdgeNoiseZero(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.NoiseScale = 0
	cfg.CongestionLinks = 0

	results, err := RunBenchmark("noise0", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark noise=0: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// With zero noise, RMSE should be very small for the identifiable links.
	for _, r := range results {
		if r.RMSE > 1e-6 {
			t.Logf("warning: RMSE %.6f higher than expected with zero noise", r.RMSE)
		}
	}
}

func TestEdgeNoiseExtreme(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.NoiseScale = 100.0

	results, err := RunBenchmark("noise100", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark noise=100: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestEdgeZeroCongestionLinks(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.CongestionLinks = 0
	cfg.CongestionFactor = 5.0

	results, err := RunBenchmark("no-congestion", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark 0 congestion links: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestEdgeAllLinksCongested(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	// Set congestion links to more than exist — Simulate clamps via min().
	cfg.CongestionLinks = 100
	cfg.CongestionFactor = 10.0

	results, err := RunBenchmark("all-congested", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark all congested: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestEdgePathFractionTiny(t *testing.T) {
	// Need a slightly bigger graph so PathFraction has an effect.
	g := topology.New()
	for i := 0; i < 5; i++ {
		g.AddNode(tomo.Node{ID: i, Label: "n"})
	}
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(2, 3)
	g.AddLink(3, 4)
	g.AddLink(0, 4)

	cfg := measure.DefaultSimConfig()
	cfg.PathFraction = 0.01 // very few paths

	results, err := RunBenchmark("tiny-frac", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark PathFraction=0.01: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestEdgeSamplesPerPathOne(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.SamplesPerPath = 1

	results, err := RunBenchmark("samples1", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark SamplesPerPath=1: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestEdgeSamplesPerPathExcessive(t *testing.T) {
	g := minimalTopo()
	cfg := measure.DefaultSimConfig()
	cfg.SamplesPerPath = 100

	results, err := RunBenchmark("samples100", g, defaultSolvers(), cfg)
	if err != nil {
		t.Fatalf("RunBenchmark SamplesPerPath=100: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}
