package tomo_test

import (
	"context"
	"math"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"gonum.org/v1/gonum/mat"
)

func TestADMMTriangle(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(0, 2)

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	solver := &tomo.ADMMSolver{}
	sol, err := solver.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	for i, gt := range groundTruth {
		if diff := math.Abs(sol.X.AtVec(i) - gt); diff > 1.0 {
			t.Errorf("link %d: got %.3f, want %.3f (diff %.3f)", i, sol.X.AtVec(i), gt, diff)
		}
	}
}

func TestADMMChain(t *testing.T) {
	groundTruth := []float64{2.0, 8.0, 1.0}
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
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	solver := &tomo.ADMMSolver{}
	sol, err := solver.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	for i, gt := range groundTruth {
		if diff := math.Abs(sol.X.AtVec(i) - gt); diff > 1.0 {
			t.Errorf("link %d: got %.3f, want %.3f (diff %.3f)", i, sol.X.AtVec(i), gt, diff)
		}
	}
}

func TestADMMSparsity(t *testing.T) {
	// 6-node topology with 5 links, only link 2 is congested.
	// A--B--C--D--E--F (chain)
	groundTruth := []float64{0.0, 0.0, 10.0, 0.0, 0.0}
	g := topology.New()
	for i := 0; i < 6; i++ {
		g.AddNode(tomo.Node{ID: i, Label: string(rune('A' + i))})
	}
	g.AddLink(0, 1) // link 0
	g.AddLink(1, 2) // link 1
	g.AddLink(2, 3) // link 2
	g.AddLink(3, 4) // link 3
	g.AddLink(4, 5) // link 4

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	solver := &tomo.ADMMSolver{}
	sol, err := solver.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	// The congested link should have the largest estimate.
	maxIdx := 0
	for i := 1; i < 5; i++ {
		if sol.X.AtVec(i) > sol.X.AtVec(maxIdx) {
			maxIdx = i
		}
	}
	if maxIdx != 2 {
		t.Errorf("expected link 2 to be largest, got link %d", maxIdx)
	}

	// The congested link should be close to 10.
	if diff := math.Abs(sol.X.AtVec(2) - 10.0); diff > 1.5 {
		t.Errorf("link 2: got %.3f, want ~10.0", sol.X.AtVec(2))
	}

	// Non-congested links should be near zero.
	for _, i := range []int{0, 1, 3, 4} {
		if math.Abs(sol.X.AtVec(i)) > 1.0 {
			t.Errorf("link %d: got %.3f, want ~0.0", i, sol.X.AtVec(i))
		}
	}
}

func TestADMMCholeskyRetry(t *testing.T) {
	// Underdetermined system (2 measurements, 5 links) so AᵀA is rank-deficient,
	// forcing the Cholesky retry loop to bump rho for stability.
	A := mat.NewDense(2, 5, []float64{
		1, 1, 0, 0, 0,
		0, 0, 1, 1, 1,
	})
	b := mat.NewVecDense(2, []float64{3.0, 6.0})

	p := &tomo.Problem{A: A, B: b}
	solver := &tomo.ADMMSolver{}
	sol, err := solver.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve should succeed with Cholesky retry: %v", err)
	}

	for i := 0; i < 5; i++ {
		v := sol.X.AtVec(i)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("link %d: got %v, want finite value", i, v)
		}
	}
}
