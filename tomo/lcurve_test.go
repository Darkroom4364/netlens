package tomo_test

import (
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

func TestLCurveAbilene(t *testing.T) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}
	gt := make([]float64, g.NumLinks())
	for i := range gt {
		gt[i] = float64(i+1) * 1.5
	}
	p, err := tomo.BuildProblemFromTopology(g, gt)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}
	sol, err := (&tomo.TikhonovSolver{LambdaMethod: "lcurve"}).Solve(p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	lam := sol.Metadata["lambda"].(float64)
	t.Logf("L-curve lambda=%.6e, residual=%.6f", lam, sol.Residual)
	if lam <= 0 {
		t.Fatal("lambda must be positive")
	}
}

func TestLCurveVsGCV(t *testing.T) {
	p := buildTriangleProblem(t, []float64{5.0, 3.0, 7.0})
	gcv, _ := (&tomo.TikhonovSolver{LambdaMethod: "gcv"}).Solve(p)
	lc, _ := (&tomo.TikhonovSolver{LambdaMethod: "lcurve"}).Solve(p)
	lamGCV := gcv.Metadata["lambda"].(float64)
	lamLC := lc.Metadata["lambda"].(float64)
	t.Logf("GCV=%.6e  L-curve=%.6e", lamGCV, lamLC)
	if lamGCV <= 0 || lamLC <= 0 {
		t.Fatal("both lambdas must be positive")
	}
}
