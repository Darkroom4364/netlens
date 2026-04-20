package tomo_test

import (
	"context"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
)

func BenchmarkBootstrap_Abilene(b *testing.B) {
	g, err := topology.LoadGraphML(testdataDir() + "/abilene.graphml")
	if err != nil {
		b.Fatalf("LoadGraphML: %v", err)
	}
	nLinks := g.NumLinks()
	gt := make([]float64, nLinks)
	for i := range gt {
		gt[i] = float64(i+1) * 1.5
	}
	p, err := tomo.BuildProblemFromTopology(g, gt)
	if err != nil {
		b.Fatalf("BuildProblem: %v", err)
	}
	cfg := tomo.BootstrapConfig{NumSamples: 100, Seed: 42}
	solver := &tomo.NNLSSolver{}
	b.ResetTimer()
	for range b.N {
		_, err := tomo.Bootstrap(context.Background(), p, solver, cfg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBootstrap_Chinanet(b *testing.B) {
	g, err := topology.LoadGraphML(testdataDir() + "/chinanet.graphml")
	if err != nil {
		b.Fatalf("LoadGraphML: %v", err)
	}
	nLinks := g.NumLinks()
	gt := make([]float64, nLinks)
	for i := range gt {
		gt[i] = float64(i+1) * 0.5
	}
	p, err := tomo.BuildProblemFromTopology(g, gt)
	if err != nil {
		b.Fatalf("BuildProblem: %v", err)
	}
	cfg := tomo.BootstrapConfig{NumSamples: 100, Seed: 42}
	solver := &tomo.NNLSSolver{}
	b.ResetTimer()
	for range b.N {
		_, err := tomo.Bootstrap(context.Background(), p, solver, cfg)
		if err != nil {
			b.Fatal(err)
		}
	}
}
