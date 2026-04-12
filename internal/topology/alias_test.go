package topology

import (
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

func TestResolveAliases_PassThrough(t *testing.T) {
	// Build a small graph: A -- B -- C
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "10.0.0.1"})
	g.AddNode(tomo.Node{ID: 1, Label: "10.0.0.2"})
	g.AddNode(tomo.Node{ID: 2, Label: "10.0.0.5"})
	g.AddLink(0, 1)
	g.AddLink(1, 2)

	resolved := ResolveAliases(g)

	if resolved.NumNodes() != g.NumNodes() {
		t.Errorf("expected %d nodes, got %d", g.NumNodes(), resolved.NumNodes())
	}
	if resolved.NumLinks() != g.NumLinks() {
		t.Errorf("expected %d links, got %d", g.NumLinks(), resolved.NumLinks())
	}
}

func TestResolveAliases_ViaInferOpts(t *testing.T) {
	// Verify the AliasResolution flag wires through InferFromMeasurements.
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				{IP: "10.0.0.1", RTT: time.Millisecond, TTL: 1},
				{IP: "10.0.0.2", RTT: time.Millisecond, TTL: 2},
				{IP: "10.0.0.3", RTT: time.Millisecond, TTL: 3},
			},
		},
	}

	g, specs, err := InferFromMeasurements(ms, InferOpts{AliasResolution: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stub is pass-through, so counts should be unchanged.
	if g.NumNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 2 {
		t.Errorf("expected 2 links, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec, got %d", len(specs))
	}
}
