package tomo_test

import (
	"math"
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
	"gonum.org/v1/gonum/mat"
)

// buildTriangleProblem creates a simple 3-node triangle with known link delays.
func buildTriangleProblem(t *testing.T, groundTruth []float64) *tomo.Problem {
	t.Helper()
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddNode(tomo.Node{ID: 2, Label: "C"})
	g.AddLink(0, 1) // link 0
	g.AddLink(1, 2) // link 1
	g.AddLink(0, 2) // link 2

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}
	return p
}

// buildChainProblem creates a 4-node chain A--B--C--D.
func buildChainProblem(t *testing.T, groundTruth []float64) *tomo.Problem {
	t.Helper()
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
	return p
}

func TestSolversTriangle(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	p := buildTriangleProblem(t, groundTruth)

	solvers := []tomo.Solver{
		&tomo.TSVDSolver{},
		&tomo.TikhonovSolver{},
		&tomo.NNLSSolver{},
	}

	for _, solver := range solvers {
		t.Run(solver.Name(), func(t *testing.T) {
			sol, err := solver.Solve(p)
			if err != nil {
				t.Fatalf("Solve: %v", err)
			}

			// Check that solution matches ground truth within tolerance
			for i, gt := range groundTruth {
				est := sol.X.AtVec(i)
				relErr := math.Abs(est-gt) / gt
				if relErr > 0.05 { // 5% tolerance for exact system
					t.Errorf("link %d: est=%.4f, truth=%.4f, relErr=%.2f%%",
						i, est, gt, relErr*100)
				}
			}

			t.Logf("%s: residual=%.6f, duration=%v, x=[%.2f, %.2f, %.2f]",
				solver.Name(), sol.Residual, sol.Duration,
				sol.X.AtVec(0), sol.X.AtVec(1), sol.X.AtVec(2))
		})
	}
}

func TestSolversChain(t *testing.T) {
	groundTruth := []float64{2.0, 5.0, 1.0}
	p := buildChainProblem(t, groundTruth)

	solvers := []tomo.Solver{
		&tomo.TSVDSolver{},
		&tomo.TikhonovSolver{},
		&tomo.NNLSSolver{},
	}

	for _, solver := range solvers {
		t.Run(solver.Name(), func(t *testing.T) {
			sol, err := solver.Solve(p)
			if err != nil {
				t.Fatalf("Solve: %v", err)
			}

			for i, gt := range groundTruth {
				est := sol.X.AtVec(i)
				relErr := math.Abs(est-gt) / gt
				if relErr > 0.05 {
					t.Errorf("link %d: est=%.4f, truth=%.4f, relErr=%.2f%%",
						i, est, gt, relErr*100)
				}
			}

			t.Logf("%s: residual=%.6f, x=[%.2f, %.2f, %.2f]",
				solver.Name(), sol.Residual,
				sol.X.AtVec(0), sol.X.AtVec(1), sol.X.AtVec(2))
		})
	}
}

func TestSolversAbilene(t *testing.T) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}

	// Generate ground truth
	nLinks := g.NumLinks()
	groundTruth := make([]float64, nLinks)
	for i := range groundTruth {
		groundTruth[i] = float64(i+1) * 1.5 // 1.5, 3.0, 4.5, ...
	}

	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}

	solvers := []tomo.Solver{
		&tomo.TSVDSolver{},
		&tomo.TikhonovSolver{},
		&tomo.NNLSSolver{},
	}

	for _, solver := range solvers {
		t.Run(solver.Name(), func(t *testing.T) {
			sol, err := solver.Solve(p)
			if err != nil {
				t.Fatalf("Solve: %v", err)
			}

			// Compute RMSE and max relative error
			rmse := 0.0
			maxRelErr := 0.0
			for i, gt := range groundTruth {
				est := sol.X.AtVec(i)
				diff := est - gt
				rmse += diff * diff
				relErr := math.Abs(diff) / gt
				if relErr > maxRelErr {
					maxRelErr = relErr
				}
			}
			rmse = math.Sqrt(rmse / float64(nLinks))

			// For noise-free system, should recover exactly
			if rmse > 0.01 {
				t.Errorf("RMSE = %.6f, want < 0.01 for noise-free system", rmse)
			}

			t.Logf("%s: RMSE=%.6f, maxRelErr=%.4f%%, residual=%.6f, duration=%v",
				solver.Name(), rmse, maxRelErr*100, sol.Residual, sol.Duration)
		})
	}
}

func TestNNLSNonNegativity(t *testing.T) {
	// Create a problem where unconstrained solution would have negatives
	// A = [[1, 1], [1, 0]], b = [3, 4]
	// Unconstrained: x = [4, -1] → NNLS should give x = [4, 0] or similar
	A := mat.NewDense(2, 2, []float64{1, 1, 1, 0})
	b := mat.NewVecDense(2, []float64{3, 4})

	quality := tomo.AnalyzeQuality(A)
	p := &tomo.Problem{A: A, B: b, Quality: quality}

	solver := &tomo.NNLSSolver{}
	sol, err := solver.Solve(p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	for i := 0; i < 2; i++ {
		if sol.X.AtVec(i) < -1e-10 {
			t.Errorf("NNLS returned negative value at index %d: %f", i, sol.X.AtVec(i))
		}
	}

	t.Logf("NNLS non-neg: x=[%.4f, %.4f], residual=%.4f",
		sol.X.AtVec(0), sol.X.AtVec(1), sol.Residual)
}
