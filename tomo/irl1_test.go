package tomo

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestIRL1_Triangle(t *testing.T) {
	// 3 paths over 3 links (triangle): sparse ground truth (1 congested link)
	A := mat.NewDense(3, 3, []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	})
	truth := []float64{0, 5.0, 0}
	b := mat.NewVecDense(3, nil)
	b.MulVec(A, mat.NewVecDense(3, truth))

	sol, err := (&IRL1Solver{}).Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	if sol.Residual > 1.0 {
		t.Errorf("residual too large: %.4f", sol.Residual)
	}
	// Congested link should be recovered; zero links should stay small
	if v := sol.X.AtVec(1); v < 3.0 {
		t.Errorf("congested link 1: got %.3f, want ~5.0", v)
	}
	for _, i := range []int{0, 2} {
		if v := math.Abs(sol.X.AtVec(i)); v > 1.5 {
			t.Errorf("zero link %d: got %.3f, want ~0", i, v)
		}
	}
}

func TestIRL1_Sparsity(t *testing.T) {
	// 5 links, 4 paths; only link 2 is congested
	A := mat.NewDense(4, 5, []float64{
		1, 1, 0, 0, 0,
		0, 1, 1, 0, 0,
		0, 0, 1, 1, 0,
		0, 0, 0, 1, 1,
	})
	truth := []float64{0, 0, 5.0, 0, 0}
	b := mat.NewVecDense(4, nil)
	b.MulVec(A, mat.NewVecDense(5, truth))

	solIRL1, err := (&IRL1Solver{}).Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	solADMM, err := (&ADMMSolver{}).Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}

	// IRL1 should have sharper sparsity (less energy on zero links)
	irl1Leak, admmLeak := 0.0, 0.0
	for i := 0; i < 5; i++ {
		if truth[i] == 0 {
			irl1Leak += math.Abs(solIRL1.X.AtVec(i))
			admmLeak += math.Abs(solADMM.X.AtVec(i))
		}
	}
	t.Logf("leak — irl1: %.4f  admm: %.4f", irl1Leak, admmLeak)
	if irl1Leak > admmLeak+1e-3 {
		t.Errorf("irl1 leak (%.4f) should be <= admm leak (%.4f)", irl1Leak, admmLeak)
	}
}

func TestIRL1_WeightsConverge(t *testing.T) {
	// Triangle topology: 3 paths, 3 links; link 1 congested
	A := mat.NewDense(3, 3, []float64{1, 1, 0, 0, 1, 1, 1, 0, 1})
	truth := []float64{0, 5.0, 0}
	b := mat.NewVecDense(3, nil)
	b.MulVec(A, mat.NewVecDense(3, truth))

	s := &IRL1Solver{MaxOuterIter: 10, Epsilon: 0.1}
	sol, err := s.Solve(&Problem{A: A, B: b})
	if err != nil {
		t.Fatal(err)
	}
	// Solution should be close to ground truth
	for i := 0; i < 3; i++ {
		v := sol.X.AtVec(i)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("link %d: non-finite value %v", i, v)
		}
		if diff := math.Abs(v - truth[i]); diff > 1.5 {
			t.Errorf("link %d: got %.3f, want %.3f (diff %.3f)", i, v, truth[i], diff)
		}
	}
	// Sparsity: zero links in ground truth should stay small
	for _, i := range []int{0, 2} {
		if v := math.Abs(sol.X.AtVec(i)); v > 1.0 {
			t.Errorf("zero link %d leaked: got %.3f, want ~0", i, v)
		}
	}
}
