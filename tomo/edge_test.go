package tomo

import (
	"context"
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

// allSolvers returns every solver with reasonable defaults.
func allSolvers() []Solver {
	return []Solver{
		&TikhonovSolver{Lambda: 0.01},
		&NNLSSolver{MaxIter: 1000},
		&TSVDSolver{},
		&ADMMSolver{MaxIter: 200, Rho: 1.0, Lambda: 0.1},
		&VardiEMSolver{MaxIter: 500, Tolerance: 1e-6},
		&TomogravitySolver{Lambda: 0.01},
	}
}

// makeProblem builds a Problem from raw A data (row-major) and b slice.
func makeProblem(m, n int, aData []float64, bData []float64) *Problem {
	A := mat.NewDense(m, n, aData)
	B := mat.NewVecDense(m, bData)
	return &Problem{
		A:       A,
		B:       B,
		Quality: AnalyzeQuality(A),
	}
}

// assertSolution checks that a solver did not panic, and that the solution (if returned)
// has valid dimensions and a finite residual.
func assertSolution(t *testing.T, solverName string, sol *Solution, err error, expectedN int) {
	t.Helper()
	if err != nil {
		// An error is acceptable — the solver chose not to handle this edge case.
		t.Logf("%s returned error (acceptable): %v", solverName, err)
		return
	}
	if sol == nil {
		t.Errorf("%s: solution is nil but error is also nil", solverName)
		return
	}
	if sol.X == nil {
		t.Errorf("%s: solution.X is nil", solverName)
		return
	}
	if sol.X.Len() != expectedN {
		t.Errorf("%s: X length = %d, want %d", solverName, sol.X.Len(), expectedN)
	}
	if math.IsInf(sol.Residual, 0) {
		t.Errorf("%s: residual is Inf", solverName)
	}
	// NaN residual is a red flag
	if math.IsNaN(sol.Residual) {
		t.Errorf("%s: residual is NaN", solverName)
	}
}

// runAllSolvers runs every solver on p and checks the output.
func runAllSolvers(t *testing.T, label string, p *Problem) {
	t.Helper()
	_, n := p.A.Dims()
	for _, solver := range allSolvers() {
		name := fmt.Sprintf("%s/%s", label, solver.Name())
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC in %s: %v", solver.Name(), r)
				}
			}()
			sol, err := solver.Solve(context.Background(), p)
			assertSolution(t, solver.Name(), sol, err, n)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

func TestEdge_SingleLinkSinglePath(t *testing.T) {
	// 1x1 system: one path through one link, b=5.0
	p := makeProblem(1, 1, []float64{1.0}, []float64{5.0})
	runAllSolvers(t, "1x1", p)
}

func TestEdge_MoreLinksThanPaths(t *testing.T) {
	// 3 paths, 10 links — heavily underdetermined
	m, n := 3, 10
	a := make([]float64, m*n)
	// Path 0 uses links 0,1,2
	a[0], a[1], a[2] = 1, 1, 1
	// Path 1 uses links 3,4,5,6
	a[1*n+3], a[1*n+4], a[1*n+5], a[1*n+6] = 1, 1, 1, 1
	// Path 2 uses links 7,8,9
	a[2*n+7], a[2*n+8], a[2*n+9] = 1, 1, 1
	b := []float64{10.0, 20.0, 15.0}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "underdetermined-3x10", p)
}

func TestEdge_ZeroMeasurements(t *testing.T) {
	// All b values are 0.0
	m, n := 4, 3
	a := make([]float64, m*n)
	// Simple routing
	a[0*n+0], a[0*n+1] = 1, 1
	a[1*n+1], a[1*n+2] = 1, 1
	a[2*n+0], a[2*n+2] = 1, 1
	a[3*n+0], a[3*n+1], a[3*n+2] = 1, 1, 1
	b := []float64{0, 0, 0, 0}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "zero-b", p)
}

func TestEdge_HugeMeasurements(t *testing.T) {
	// b values in the millions
	m, n := 3, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	b := []float64{5e6, 3e6, 8e6}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "huge-b", p)
}

func TestEdge_TinyMeasurements(t *testing.T) {
	// b values near machine epsilon
	m, n := 3, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	b := []float64{1e-15, 2e-15, 3e-15}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "tiny-b", p)
}

func TestEdge_AllPathsSameLink(t *testing.T) {
	// Rank 1 system: all 5 paths go through the single link (column).
	// A is 5x3 but only column 0 is nonzero — rank 1.
	m, n := 5, 3
	a := make([]float64, m*n)
	for i := 0; i < m; i++ {
		a[i*n+0] = 1.0 // only link 0
	}
	b := []float64{10, 20, 30, 40, 50}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "rank1", p)
}

func TestEdge_DisconnectedTopology(t *testing.T) {
	// Two separate components; links 2,3 are unreachable from any path.
	// 3 paths, 5 links. Paths only use links 0,1,4.
	m, n := 3, 5
	a := make([]float64, m*n)
	a[0*n+0], a[0*n+1] = 1, 1
	a[1*n+0], a[1*n+4] = 1, 1
	a[2*n+1], a[2*n+4] = 1, 1
	// links 2,3 have zero coverage
	b := []float64{5.0, 8.0, 6.0}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "disconnected", p)
}

func TestEdge_IdentityRoutingMatrix(t *testing.T) {
	// A = I (5x5), each path is one unique link
	n := 5
	a := make([]float64, n*n)
	for i := 0; i < n; i++ {
		a[i*n+i] = 1.0
	}
	b := []float64{1, 2, 3, 4, 5}
	p := makeProblem(n, n, a, b)
	runAllSolvers(t, "identity", p)
}

func TestEdge_DuplicateRows(t *testing.T) {
	// Same path measured twice — duplicate rows in A.
	m, n := 4, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 1, 0, // duplicate of row 0
		0, 1, 1, // duplicate of row 1
	}
	b := []float64{5, 8, 5, 8}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "duplicate-rows", p)
}

func TestEdge_SingleColumnOfOnes(t *testing.T) {
	// All paths share link 0 (column of ones), other links unique.
	m, n := 4, 5
	a := make([]float64, m*n)
	for i := 0; i < m; i++ {
		a[i*n+0] = 1.0     // shared link
		a[i*n+(i+1)] = 1.0 // unique link per path
	}
	b := []float64{10, 20, 30, 40}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "shared-column", p)
}

func TestEdge_VeryWideMatrix(t *testing.T) {
	// 2 paths, 100 links — massively underdetermined
	m, n := 2, 100
	a := make([]float64, m*n)
	// Path 0 uses links 0-49
	for j := 0; j < 50; j++ {
		a[0*n+j] = 1.0
	}
	// Path 1 uses links 50-99
	for j := 50; j < 100; j++ {
		a[1*n+j] = 1.0
	}
	b := []float64{100.0, 200.0}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "wide-2x100", p)
}

func TestEdge_VeryTallMatrix(t *testing.T) {
	// 1000 paths, 5 links — massively overdetermined
	m, n := 1000, 5
	a := make([]float64, m*n)
	b := make([]float64, m)
	for i := 0; i < m; i++ {
		// Each path uses 2-3 links in a round-robin pattern
		a[i*n+(i%n)] = 1.0
		a[i*n+((i+1)%n)] = 1.0
		b[i] = float64(10 + i%7)
	}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "tall-1000x5", p)
}

func TestEdge_NearZeroNoise(t *testing.T) {
	// Well-conditioned 3x3, b = A*x_true + tiny noise
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	xTrue := []float64{1.0, 2.0, 3.0}
	noise := 0.001
	b := make([]float64, 3)
	b[0] = xTrue[0] + xTrue[1] + noise
	b[1] = xTrue[1] + xTrue[2] - noise
	b[2] = xTrue[0] + xTrue[2] + noise
	p := makeProblem(3, 3, a, b)
	runAllSolvers(t, "near-zero-noise", p)
}

func TestEdge_ExtremeNoise(t *testing.T) {
	// Same structure, but noise scale 10.0
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	xTrue := []float64{1.0, 2.0, 3.0}
	noise := 10.0
	b := make([]float64, 3)
	b[0] = xTrue[0] + xTrue[1] + noise
	b[1] = xTrue[1] + xTrue[2] - noise
	b[2] = xTrue[0] + xTrue[2] + noise
	p := makeProblem(3, 3, a, b)
	runAllSolvers(t, "extreme-noise", p)
}

func TestEdge_NegativeMeasurements(t *testing.T) {
	// Negative b values — not physically meaningful, but should not panic.
	m, n := 3, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	b := []float64{-5.0, -3.0, -8.0}
	p := makeProblem(m, n, a, b)
	runAllSolvers(t, "negative-b", p)
}

func TestEdge_NaNInMeasurements(t *testing.T) {
	// NaN in b — must not panic.
	m, n := 3, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	b := []float64{math.NaN(), 5.0, 3.0}
	p := makeProblem(m, n, a, b)

	for _, solver := range allSolvers() {
		name := fmt.Sprintf("nan-b/%s", solver.Name())
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC in %s with NaN input: %v", solver.Name(), r)
				}
			}()
			sol, err := solver.Solve(context.Background(), p)
			// Either an error or a solution is fine; we just must not panic.
			if err != nil {
				t.Logf("%s returned error with NaN (acceptable): %v", solver.Name(), err)
				return
			}
			if sol == nil {
				t.Errorf("%s: nil solution and nil error with NaN input", solver.Name())
				return
			}
			if sol.X == nil || sol.X.Len() != n {
				t.Errorf("%s: bad X vector with NaN input", solver.Name())
			}
		})
	}
}

func TestEdge_InfInMeasurements(t *testing.T) {
	// +Inf in b — must not panic.
	m, n := 3, 3
	a := []float64{
		1, 1, 0,
		0, 1, 1,
		1, 0, 1,
	}
	b := []float64{math.Inf(1), 5.0, 3.0}
	p := makeProblem(m, n, a, b)

	for _, solver := range allSolvers() {
		name := fmt.Sprintf("inf-b/%s", solver.Name())
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC in %s with Inf input: %v", solver.Name(), r)
				}
			}()
			sol, err := solver.Solve(context.Background(), p)
			if err != nil {
				t.Logf("%s returned error with Inf (acceptable): %v", solver.Name(), err)
				return
			}
			if sol == nil {
				t.Errorf("%s: nil solution and nil error with Inf input", solver.Name())
				return
			}
			if sol.X == nil || sol.X.Len() != n {
				t.Errorf("%s: bad X vector with Inf input", solver.Name())
			}
		})
	}
}
