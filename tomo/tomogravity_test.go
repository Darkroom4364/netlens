package tomo

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestTomogravityTriangle(t *testing.T) {
	// Triangle: 3 links, 3 paths. Each path uses 2 links.
	// Path 0: link 0 + link 1, Path 1: link 1 + link 2, Path 2: link 0 + link 2
	truth := []float64{1.0, 2.0, 3.0}
	A := mat.NewDense(3, 3, []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	})
	b := mat.NewVecDense(3, []float64{3.0, 5.0, 4.0}) // A * truth

	solver := &TomogravitySolver{Lambda: 1e-6}
	sol, err := solver.Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	if sol.Method != "tomogravity" {
		t.Errorf("method = %q, want tomogravity", sol.Method)
	}
	for j := 0; j < 3; j++ {
		got := sol.X.AtVec(j)
		if math.Abs(got-truth[j])/truth[j] > 0.10 {
			t.Errorf("link %d: got %.4f, want %.4f (>10%% error)", j, got, truth[j])
		}
	}
}

func TestTomogravityChain(t *testing.T) {
	// Chain: 3 links in series. Paths measure prefixes.
	// Path 0: link 0, Path 1: link 0+1, Path 2: link 0+1+2
	truth := []float64{1.0, 2.0, 3.0}
	A := mat.NewDense(3, 3, []float64{
		1, 0, 0,
		1, 1, 0,
		1, 1, 1,
	})
	b := mat.NewVecDense(3, []float64{1.0, 3.0, 6.0})

	solver := &TomogravitySolver{Lambda: 1e-6}
	sol, err := solver.Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	for j := 0; j < 3; j++ {
		got := sol.X.AtVec(j)
		if math.Abs(got-truth[j])/truth[j] > 0.10 {
			t.Errorf("link %d: got %.4f, want %.4f (>10%% error)", j, got, truth[j])
		}
	}
}

func TestTomogravityNonNegative(t *testing.T) {
	// Ensure output is non-negative even with noisy input.
	A := mat.NewDense(2, 2, []float64{
		1, 1,
		1, 0,
	})
	b := mat.NewVecDense(2, []float64{0.5, 3.0}) // would push link 1 negative

	solver := &TomogravitySolver{}
	sol, err := solver.Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	for j := 0; j < 2; j++ {
		if sol.X.AtVec(j) < 0 {
			t.Errorf("link %d: got %.4f, expected non-negative", j, sol.X.AtVec(j))
		}
	}
}
