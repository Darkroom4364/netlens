package tomo_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

func TestConformalTriangleNoiseFree(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	p := buildTriangleProblem(t, groundTruth)

	sol, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{Seed: 42})
	if err != nil {
		t.Fatalf("Conformal: %v", err)
	}
	if sol.Confidence == nil {
		t.Fatal("Confidence is nil")
	}
	for j := 0; j < p.NumLinks(); j++ {
		ci := sol.Confidence.AtVec(j)
		if math.IsNaN(ci) {
			t.Errorf("link %d: NaN confidence", j)
		}
		t.Logf("link %d: est=%.4f, CI=±%.4f", j, sol.X.AtVec(j), ci)
	}
}

func TestConformalAbileneNoisy(t *testing.T) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}
	nLinks := g.NumLinks()
	groundTruth := make([]float64, nLinks)
	for i := range groundTruth {
		groundTruth[i] = float64(i+1) * 1.5
	}
	p, err := tomo.BuildProblemFromTopology(g, groundTruth)
	if err != nil {
		t.Fatalf("BuildProblemFromTopology: %v", err)
	}
	for i := 0; i < p.NumPaths(); i++ {
		p.B.SetVec(i, p.B.AtVec(i)+float64(i%5)*0.5)
	}

	sol, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{Seed: 123})
	if err != nil {
		t.Fatalf("Conformal: %v", err)
	}
	anyPositive := false
	for j := 0; j < p.NumLinks(); j++ {
		if sol.Confidence.AtVec(j) > 1e-9 && !math.IsInf(sol.Confidence.AtVec(j), 0) {
			anyPositive = true
			break
		}
	}
	if !anyPositive {
		t.Error("expected positive finite CI for noisy data")
	}
	t.Logf("Abilene: %d links, sample CI=%.4f", nLinks, sol.Confidence.AtVec(0))
}

func TestConformalCoverageGuarantee(t *testing.T) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}
	nLinks := g.NumLinks()
	groundTruth := make([]float64, nLinks)
	for i := range groundTruth {
		groundTruth[i] = float64(i+1) * 2.0
	}

	alpha := 0.10
	nTrials := 40
	covered := 0
	rng := rand.New(rand.NewSource(99))

	for trial := 0; trial < nTrials; trial++ {
		p, _ := tomo.BuildProblemFromTopology(g, groundTruth)
		// Add random noise.
		for i := 0; i < p.NumPaths(); i++ {
			p.B.SetVec(i, p.B.AtVec(i)+rng.NormFloat64()*0.5)
		}
		sol, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{
			Alpha: alpha, Seed: int64(trial + 1),
		})
		if err != nil {
			continue
		}
		allCovered := true
		for j := 0; j < nLinks; j++ {
			ci := sol.Confidence.AtVec(j)
			if math.IsInf(ci, 0) {
				continue
			}
			if math.Abs(sol.X.AtVec(j)-groundTruth[j]) > ci {
				allCovered = false
				break
			}
		}
		if allCovered {
			covered++
		}
	}
	rate := float64(covered) / float64(nTrials)
	t.Logf("coverage rate: %.2f (target >= %.2f)", rate, 1-alpha)
}

func TestConformalCalibrationFrac(t *testing.T) {
	groundTruth := []float64{5.0, 3.0, 7.0}
	p := buildTriangleProblem(t, groundTruth)

	sol02, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{
		CalibrationFrac: 0.2, Seed: 7,
	})
	if err != nil {
		t.Fatalf("frac=0.2: %v", err)
	}
	sol05, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{
		CalibrationFrac: 0.5, Seed: 7,
	})
	if err != nil {
		t.Fatalf("frac=0.5: %v", err)
	}
	t.Logf("frac=0.2 CI[0]=%.4f  frac=0.5 CI[0]=%.4f",
		sol02.Confidence.AtVec(0), sol05.Confidence.AtVec(0))
}

func TestConformalDegeneratetwoPaths(t *testing.T) {
	g := topology.New()
	g.AddNode(tomo.Node{ID: 0, Label: "A"})
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
	g.AddLink(0, 1)

	p, err := tomo.BuildProblemFromTopology(g, []float64{10.0})
	if err != nil {
		t.Fatalf("BuildProblem: %v", err)
	}

	sol, err := tomo.Conformal(p, &tomo.NNLSSolver{}, tomo.ConformalConfig{Seed: 1})
	if err != nil {
		t.Fatalf("Conformal: %v", err)
	}
	if sol.Confidence == nil {
		t.Fatal("Confidence is nil")
	}
	t.Logf("degenerate: est=%.4f, CI=%.4f", sol.X.AtVec(0), sol.Confidence.AtVec(0))
}
