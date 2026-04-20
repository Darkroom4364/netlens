package tomo_test

import (
	"context"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
)

// benchSolver is a shared helper that benchmarks a solver on the Abilene topology.
func benchSolver(b *testing.B, solver tomo.Solver) {
	b.Helper()
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		b.Fatalf("LoadGraphML: %v", err)
	}
	gt := make([]float64, g.NumLinks())
	for i := range gt {
		gt[i] = float64(i+1) * 1.5
	}
	p, err := tomo.BuildProblemFromTopology(g, gt)
	if err != nil {
		b.Fatalf("BuildProblem: %v", err)
	}
	b.ResetTimer()
	for range b.N {
		_, err := solver.Solve(context.Background(), p)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTikhonov_Abilene(b *testing.B)    { benchSolver(b, &tomo.TikhonovSolver{}) }
func BenchmarkTSVD_Abilene(b *testing.B)         { benchSolver(b, &tomo.TSVDSolver{}) }
func BenchmarkNNLS_Abilene(b *testing.B)         { benchSolver(b, &tomo.NNLSSolver{}) }
func BenchmarkADMM_Abilene(b *testing.B)         { benchSolver(b, &tomo.ADMMSolver{}) }
func BenchmarkIRL1_Abilene(b *testing.B)         { benchSolver(b, &tomo.IRL1Solver{}) }
func BenchmarkVardiEM_Abilene(b *testing.B)      { benchSolver(b, &tomo.VardiEMSolver{}) }
func BenchmarkTomogravity_Abilene(b *testing.B)  { benchSolver(b, &tomo.TomogravitySolver{}) }
