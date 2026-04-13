package tomo_test

import (
	"math"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

func TestBootstrapTriangleNoiseFree(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	p := buildTriangleProblem(t, groundTruth)

	sol, err := tomo.Bootstrap(p, &tomo.NNLSSolver{}, tomo.BootstrapConfig{
		NumSamples: 50,
		Seed:       42,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if sol.Confidence == nil {
		t.Fatal("Confidence is nil")
	}

	// With only 3 paths in a triangle, bootstrap resampling creates many
	// rank-deficient systems (duplicate rows), so CIs won't be near zero.
	// Just verify they are finite and non-negative.
	for j := 0; j < p.NumLinks(); j++ {
		ci := sol.Confidence.AtVec(j)
		if ci < 0 || math.IsNaN(ci) || math.IsInf(ci, 0) {
			t.Errorf("link %d: invalid CI half-width %.4f", j, ci)
		}
		t.Logf("link %d: est=%.4f, CI=±%.4f", j, sol.X.AtVec(j), ci)
	}
}

func TestBootstrapAbileneNoisy(t *testing.T) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}

	nLinks := g.NumLinks()
	groundTruth := make([]float64, nLinks)
	for i := range groundTruth {
		groundTruth[i] = float64(i+1) * 1.5
	}

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	// Add noise to measurements.
	for i := 0; i < p.NumPaths(); i++ {
		p.B.SetVec(i, p.B.AtVec(i)+float64(i%5)*0.5)
	}

	sol, err := tomo.Bootstrap(p, &tomo.NNLSSolver{}, tomo.BootstrapConfig{
		NumSamples: 50,
		Seed:       123,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	anyPositive := false
	for j := 0; j < p.NumLinks(); j++ {
		ci := sol.Confidence.AtVec(j)
		if ci > 1e-9 {
			anyPositive = true
		}
	}
	if !anyPositive {
		t.Error("expected at least some positive CI for noisy data")
	}

	// Log a few.
	maxCI := 0.0
	for j := 0; j < p.NumLinks(); j++ {
		ci := sol.Confidence.AtVec(j)
		if ci > maxCI {
			maxCI = ci
		}
	}
	t.Logf("Abilene noisy: %d links, max CI=±%.4f", p.NumLinks(), maxCI)
	_ = math.Abs(0) // keep math imported
}

func TestBootstrapSingleSample(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	p := buildTriangleProblem(t, groundTruth)

	sol, err := tomo.Bootstrap(p, &tomo.NNLSSolver{}, tomo.BootstrapConfig{
		NumSamples: 1,
		Seed:       1,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if sol.X == nil {
		t.Fatal("solution X is nil")
	}
	if sol.Confidence == nil {
		t.Fatal("Confidence is nil")
	}
	t.Logf("NumSamples=1: x=[%.2f, %.2f, %.2f]",
		sol.X.AtVec(0), sol.X.AtVec(1), sol.X.AtVec(2))
}
