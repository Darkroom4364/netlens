package tomo

import (
	"context"
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestVardiEM_Triangle(t *testing.T) {
	// Triangle: 3 links, 3 paths
	// Path 0: link 0, link 1
	// Path 1: link 1, link 2
	// Path 2: link 0, link 2
	A := mat.NewDense(3, 3, []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	})
	// Ground truth: x = [10, 20, 30]
	// b = A*x = [30, 50, 40]
	b := mat.NewVecDense(3, []float64{30, 50, 40})

	p := &Problem{A: A, B: b}
	sol, err := (&VardiEMSolver{}).Solve(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}

	want := []float64{10, 20, 30}
	for j, w := range want {
		if math.Abs(sol.X.AtVec(j)-w) > 0.5 {
			t.Errorf("link %d: got %.4f, want %.1f", j, sol.X.AtVec(j), w)
		}
	}
	t.Logf("iterations=%v residual=%.6f", sol.Metadata["iterations"], sol.Residual)
}

func TestVardiEM_Chain(t *testing.T) {
	// Chain: 3 links in series, 3 paths (subpaths)
	// Path 0: link 0
	// Path 1: link 0, link 1
	// Path 2: link 0, link 1, link 2
	A := mat.NewDense(3, 3, []float64{
		1, 0, 0,
		1, 1, 0,
		1, 1, 1,
	})
	// Ground truth: x = [5, 10, 15]
	// b = [5, 15, 30]
	b := mat.NewVecDense(3, []float64{5, 15, 30})

	p := &Problem{A: A, B: b}
	sol, err := (&VardiEMSolver{}).Solve(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}

	want := []float64{5, 10, 15}
	for j, w := range want {
		if math.Abs(sol.X.AtVec(j)-w) > 0.5 {
			t.Errorf("link %d: got %.4f, want %.1f", j, sol.X.AtVec(j), w)
		}
	}
	t.Logf("iterations=%v residual=%.6f", sol.Metadata["iterations"], sol.Residual)
}

func TestVardiEM_NonNegative(t *testing.T) {
	// Even with noisy input, outputs should be non-negative.
	A := mat.NewDense(4, 3, []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
		1, 1, 1,
	})
	b := mat.NewVecDense(4, []float64{2, 3, 1, 5})

	p := &Problem{A: A, B: b}
	sol, err := (&VardiEMSolver{}).Solve(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}

	for j := 0; j < 3; j++ {
		if sol.X.AtVec(j) < 0 {
			t.Errorf("link %d: got negative value %.6f", j, sol.X.AtVec(j))
		}
	}
	t.Logf("x = [%.4f, %.4f, %.4f] residual=%.6f",
		sol.X.AtVec(0), sol.X.AtVec(1), sol.X.AtVec(2), sol.Residual)
}
