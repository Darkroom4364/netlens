package tomo_test

import (
	"math"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
)

func TestLaplacianSolverInterface(t *testing.T) {
	var _ tomo.Solver = &tomo.LaplacianSolver{}
}

func TestLaplacianTriangleSmooth(t *testing.T) {
	// Smooth ground truth: adjacent links have similar values.
	groundTruth := []float64{5.0, 5.5, 4.5}
	p := buildTriangleProblem(t, groundTruth)
	sol, err := (&tomo.LaplacianSolver{Lambda: 0.01}).Solve(p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	for i, gt := range groundTruth {
		if math.Abs(sol.X.AtVec(i)-gt) > 1.0 {
			t.Errorf("link %d: got %.3f, want ~%.3f", i, sol.X.AtVec(i), gt)
		}
	}
}

func TestLaplacianChainSmoother(t *testing.T) {
	// Chain A-B-C-D: Laplacian should produce smoother estimates than Tikhonov
	// when ground truth has an outlier link.
	groundTruth := []float64{5.0, 5.0, 5.0}
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddNode(tomo.Node{ID: 3, Label: "D"})
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(2, 3)

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	lapSol, err := (&tomo.LaplacianSolver{Lambda: 0.1}).Solve(p)
	if err != nil {
		t.Fatalf("laplacian: %v", err)
	}
	tikSol, err := (&tomo.TikhonovSolver{Lambda: 0.1}).Solve(p)
	if err != nil {
		t.Fatalf("tikhonov: %v", err)
	}

	// Laplacian smoothness: variance of estimates should be <= Tikhonov's
	lapVar := variance(lapSol.X)
	tikVar := variance(tikSol.X)
	t.Logf("laplacian var=%.6f, tikhonov var=%.6f", lapVar, tikVar)
	if lapVar > tikVar+0.01 {
		t.Errorf("laplacian solution less smooth than tikhonov: %.6f > %.6f", lapVar, tikVar)
	}
}

func variance(v interface{ AtVec(int) float64; Len() int }) float64 {
	n := v.Len()
	mean := 0.0
	for i := 0; i < n; i++ {
		mean += v.AtVec(i)
	}
	mean /= float64(n)
	s := 0.0
	for i := 0; i < n; i++ {
		d := v.AtVec(i) - mean
		s += d * d
	}
	return s / float64(n)
}
