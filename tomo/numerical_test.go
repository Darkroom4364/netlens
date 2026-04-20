package tomo

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/mat"
)

// makeMatrixFromSVs builds an m×n matrix with prescribed singular values.
// The matrix is constructed as U*diag(sv)*Vᵀ using random orthogonal factors.
func makeMatrixFromSVs(m, n int, sv []float64) *mat.Dense {
	k := len(sv)
	if k > m || k > n {
		panic("too many singular values for given dimensions")
	}

	// Build random orthogonal U (m×m) and V (n×n) via QR of random matrices.
	rng := rand.New(rand.NewSource(12345))
	randMat := func(rows, cols int) *mat.Dense {
		data := make([]float64, rows*cols)
		for i := range data {
			data[i] = rng.NormFloat64()
		}
		return mat.NewDense(rows, cols, data)
	}

	var qrU, qrV mat.QR
	qrU.Factorize(randMat(m, m))
	qrV.Factorize(randMat(n, n))

	var U, V mat.Dense
	qrU.QTo(&U)
	qrV.QTo(&V)

	// Sigma: m×n with sv on diagonal.
	sigma := mat.NewDense(m, n, nil)
	for i := 0; i < k; i++ {
		sigma.Set(i, i, sv[i])
	}

	// A = U * Sigma * Vᵀ
	var tmp, A mat.Dense
	tmp.Mul(&U, sigma)
	A.Mul(&tmp, V.T())
	return &A
}

// ---------------------------------------------------------------------------
// Numerical stress tests
// ---------------------------------------------------------------------------

func TestNumerical_NearSingularMatrix(t *testing.T) {
	// 2×2 matrix with singular values [1e7, 1e-7] → cond ≈ 1e14.
	A := makeMatrixFromSVs(3, 2, []float64{1e7, 1e-7})
	m, n := A.Dims()
	// b = A * [1, 1]
	x0 := mat.NewVecDense(n, []float64{1, 1})
	b := mat.NewVecDense(m, nil)
	b.MulVec(A, x0)
	p := &Problem{A: A, B: b, Quality: AnalyzeQuality(A)}
	runAllSolvers(t, "near-singular-1e14", p)
}

func TestNumerical_ExtremeSVSpread(t *testing.T) {
	// 4×3 matrix with singular values [1e10, 1, 1e-10].
	A := makeMatrixFromSVs(4, 3, []float64{1e10, 1, 1e-10})
	m, n := A.Dims()
	x0 := mat.NewVecDense(n, []float64{1, 1, 1})
	b := mat.NewVecDense(m, nil)
	b.MulVec(A, x0)
	p := &Problem{A: A, B: b, Quality: AnalyzeQuality(A)}
	runAllSolvers(t, "extreme-sv-spread", p)
}

func TestNumerical_ADMMLambdaZero(t *testing.T) {
	// ADMM with Lambda=0 explicitly — auto-select path, must not infinite loop.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	solver := &ADMMSolver{Lambda: 0, Rho: 1.0, MaxIter: 200}
	t.Run("lambda-zero", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "admm-lambda0", sol, err, 3)
	})
}

func TestNumerical_ADMMRhoTiny(t *testing.T) {
	// ADMM with rho=1e-10 — near-zero penalty.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	solver := &ADMMSolver{Lambda: 0.1, Rho: 1e-10, MaxIter: 200}
	t.Run("rho-tiny", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "admm-rho-tiny", sol, err, 3)
	})
}

func TestNumerical_ADMMRhoHuge(t *testing.T) {
	// ADMM with rho=1e10 — extreme penalty.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	solver := &ADMMSolver{Lambda: 0.1, Rho: 1e10, MaxIter: 200}
	t.Run("rho-huge", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "admm-rho-huge", sol, err, 3)
	})
}

func TestNumerical_VardiToleranceZero(t *testing.T) {
	// Vardi with Tolerance=0 — should not infinite loop (MaxIter guard).
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	solver := &VardiEMSolver{MaxIter: 100, Tolerance: 0}
	t.Run("tolerance-zero", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "vardi-tol0", sol, err, 3)
	})
}

func TestNumerical_VardiAllInitialZero(t *testing.T) {
	// Vardi initializes x_j = 1.0 internally, but if all A entries are zero
	// for some links, the E-step denominator for paths through those links is zero.
	// Create a matrix where some links are never on any path (zero columns).
	m, n := 3, 5
	a := make([]float64, m*n)
	// Only links 0,1,2 are used; links 3,4 have zero columns.
	a[0*n+0], a[0*n+1] = 1, 1
	a[1*n+1], a[1*n+2] = 1, 1
	a[2*n+0], a[2*n+2] = 1, 1
	b := []float64{5, 8, 6}
	A := mat.NewDense(m, n, a)
	p := &Problem{A: A, B: mat.NewVecDense(m, b), Quality: AnalyzeQuality(A)}

	solver := &VardiEMSolver{MaxIter: 500, Tolerance: 1e-6}
	t.Run("zero-columns", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "vardi-zero-cols", sol, err, n)
	})
}

func TestNumerical_SolverDeterminism(t *testing.T) {
	// Run each solver twice on the same Problem; X vectors must be identical.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	for _, solver := range allSolvers() {
		t.Run(solver.Name(), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC: %v", r)
				}
			}()
			sol1, err1 := solver.Solve(context.Background(), p)
			sol2, err2 := solver.Solve(context.Background(), p)

			if err1 != nil || err2 != nil {
				t.Logf("solver returned error (skipping determinism check): err1=%v err2=%v", err1, err2)
				return
			}
			if sol1.X.Len() != sol2.X.Len() {
				t.Fatalf("X lengths differ: %d vs %d", sol1.X.Len(), sol2.X.Len())
			}
			for i := 0; i < sol1.X.Len(); i++ {
				v1 := sol1.X.AtVec(i)
				v2 := sol2.X.AtVec(i)
				if v1 != v2 {
					t.Errorf("index %d: run1=%.15e, run2=%.15e", i, v1, v2)
				}
			}
		})
	}
}

func TestNumerical_QualityNil(t *testing.T) {
	// Problem.Quality is nil — all solvers should handle gracefully.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: nil}

	for _, solver := range allSolvers() {
		name := fmt.Sprintf("nil-quality/%s", solver.Name())
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC in %s with nil Quality: %v", solver.Name(), r)
				}
			}()
			sol, err := solver.Solve(context.Background(), p)
			if err != nil {
				t.Logf("%s returned error with nil Quality (acceptable): %v", solver.Name(), err)
				return
			}
			if sol == nil {
				t.Errorf("%s: nil solution and nil error with nil Quality", solver.Name())
				return
			}
			if sol.X == nil || sol.X.Len() != 3 {
				t.Errorf("%s: bad X vector with nil Quality", solver.Name())
			}
		})
	}
}

func TestNumerical_SequentialSolversSameProblem(t *testing.T) {
	// Run Tikhonov then NNLS on the same Problem; verify Problem is not mutated.
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(3, 3, a)
	p := &Problem{A: A, B: mat.NewVecDense(3, b), Quality: AnalyzeQuality(A)}

	// Snapshot A and B before.
	m, n := A.Dims()
	aBefore := make([]float64, m*n)
	copy(aBefore, A.RawMatrix().Data)
	bBefore := make([]float64, m)
	for i := 0; i < m; i++ {
		bBefore[i] = p.B.AtVec(i)
	}

	tik := &TikhonovSolver{Lambda: 0.01}
	_, err := tik.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Tikhonov solve failed: %v", err)
	}

	nnls := &NNLSSolver{MaxIter: 1000}
	sol2, err := nnls.Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("NNLS solve failed: %v", err)
	}
	assertSolution(t, "nnls-after-tikhonov", sol2, nil, n)

	// Verify A and B unchanged.
	aAfter := A.RawMatrix().Data
	for i, v := range aBefore {
		if aAfter[i] != v {
			t.Errorf("A was mutated at index %d: before=%f, after=%f", i, v, aAfter[i])
		}
	}
	for i, v := range bBefore {
		if p.B.AtVec(i) != v {
			t.Errorf("B was mutated at index %d: before=%f, after=%f", i, v, p.B.AtVec(i))
		}
	}
}

func TestNumerical_Bootstrap2Paths(t *testing.T) {
	// Bootstrap with only 2 paths — degenerate resampling.
	m, n := 2, 2
	a := []float64{1, 1, 1, 0}
	b := []float64{5, 3}
	A := mat.NewDense(m, n, a)
	p := &Problem{A: A, B: mat.NewVecDense(m, b), Quality: AnalyzeQuality(A)}

	solver := &NNLSSolver{MaxIter: 1000}
	cfg := BootstrapConfig{NumSamples: 20, Seed: 42}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC in bootstrap with 2 paths: %v", r)
		}
	}()

	sol, err := Bootstrap(context.Background(), p, solver, cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if sol.X == nil {
		t.Fatal("solution X is nil")
	}
	if sol.Confidence == nil {
		t.Fatal("Confidence is nil")
	}
	t.Logf("2-path bootstrap: x=[%.4f, %.4f], CI=[±%.4f, ±%.4f]",
		sol.X.AtVec(0), sol.X.AtVec(1),
		sol.Confidence.AtVec(0), sol.Confidence.AtVec(1))
}

func TestNumerical_BootstrapSeedDeterminism(t *testing.T) {
	// Same seed → identical confidence intervals.
	m, n := 3, 3
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{5, 8, 6}
	A := mat.NewDense(m, n, a)
	p := &Problem{A: A, B: mat.NewVecDense(m, b), Quality: AnalyzeQuality(A)}

	solver := &NNLSSolver{MaxIter: 1000}
	cfg := BootstrapConfig{NumSamples: 50, Seed: 999}

	sol1, err := Bootstrap(context.Background(), p, solver, cfg)
	if err != nil {
		t.Fatalf("Bootstrap run 1: %v", err)
	}
	sol2, err := Bootstrap(context.Background(), p, solver, cfg)
	if err != nil {
		t.Fatalf("Bootstrap run 2: %v", err)
	}

	if sol1.Confidence == nil || sol2.Confidence == nil {
		t.Fatal("Confidence is nil in one of the runs")
	}
	for j := 0; j < n; j++ {
		c1 := sol1.Confidence.AtVec(j)
		c2 := sol2.Confidence.AtVec(j)
		if c1 != c2 {
			t.Errorf("link %d: CI run1=%.10f, CI run2=%.10f", j, c1, c2)
		}
	}
}

func TestNumerical_TomogravityGravityPriorAllZeros(t *testing.T) {
	// Gravity prior all zeros: no path covers any link.
	// A is all zeros except we need at least one nonzero for a valid problem.
	// Actually test: A has structure but b is zero → gravity prior = 0.
	m, n := 3, 3
	a := []float64{1, 1, 0, 0, 1, 1, 1, 0, 1}
	b := []float64{0, 0, 0}
	A := mat.NewDense(m, n, a)
	p := &Problem{A: A, B: mat.NewVecDense(m, b), Quality: AnalyzeQuality(A)}

	solver := &TomogravitySolver{Lambda: 0.01}
	t.Run("zero-gravity", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p)
		assertSolution(t, "tomogravity-zero", sol, err, n)
		if err == nil && sol != nil {
			// All estimates should be zero or near-zero.
			for j := 0; j < n; j++ {
				if math.Abs(sol.X.AtVec(j)) > 1e-6 {
					t.Errorf("link %d: expected ~0, got %f", j, sol.X.AtVec(j))
				}
			}
		}
	})

	// Also test: A with zero columns (links with no coverage at all).
	a2 := make([]float64, 3*5)
	a2[0*5+0], a2[0*5+1] = 1, 1
	a2[1*5+1], a2[1*5+2] = 1, 1
	a2[2*5+0], a2[2*5+2] = 1, 1
	// links 3,4 have zero columns
	b2 := []float64{5, 8, 6}
	A2 := mat.NewDense(3, 5, a2)
	p2 := &Problem{A: A2, B: mat.NewVecDense(3, b2), Quality: AnalyzeQuality(A2)}

	t.Run("uncovered-links", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC: %v", r)
			}
		}()
		sol, err := solver.Solve(context.Background(), p2)
		assertSolution(t, "tomogravity-uncovered", sol, err, 5)
	})
}

func TestNumerical_VeryDense100x100(t *testing.T) {
	// 100×100 dense matrix — all entries nonzero, stress SVD.
	m, n := 100, 100
	rng := rand.New(rand.NewSource(42))
	aData := make([]float64, m*n)
	for i := range aData {
		aData[i] = rng.Float64() + 0.01 // ensure all nonzero
	}
	A := mat.NewDense(m, n, aData)

	// b = A * x_true where x_true is random positive.
	xTrue := make([]float64, n)
	for i := range xTrue {
		xTrue[i] = rng.Float64()*10 + 0.1
	}
	xVec := mat.NewVecDense(n, xTrue)
	b := mat.NewVecDense(m, nil)
	b.MulVec(A, xVec)

	p := &Problem{A: A, B: b, Quality: AnalyzeQuality(A)}
	runAllSolvers(t, "dense-100x100", p)
}
