package topology

import (
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

func buildGraph(ips []string, links [][2]int) *Graph {
	g := New()
	for i, ip := range ips {
		g.AddNode(tomo.Node{ID: i, Label: ip})
	}
	for _, l := range links {
		g.AddLink(l[0], l[1])
	}
	return g
}

func TestAliasSameSlash31_NoMerge(t *testing.T) {
	// 10.0.0.1 and 10.0.0.2 share /31 → different routers
	g := buildGraph([]string{"10.0.0.1", "10.0.0.2"}, [][2]int{{0, 1}})
	out := ResolveAliases(g)
	if out.NumNodes() != 2 {
		t.Fatalf("expected 2 nodes, got %d", out.NumNodes())
	}
}

func TestAliasDifferentSlash31_Merge(t *testing.T) {
	// 10.0.0.0 (/31=.0) and 10.0.0.3 (/31=.2), same /30, not connected → merge
	g := buildGraph([]string{"10.0.0.0", "10.0.0.3"}, nil)
	out := ResolveAliases(g)
	if out.NumNodes() != 1 {
		t.Fatalf("expected 1 node after merge, got %d", out.NumNodes())
	}
}

func TestAliasDifferentSlash30_NoMerge(t *testing.T) {
	g := buildGraph([]string{"10.0.0.1", "10.0.0.5"}, nil)
	out := ResolveAliases(g)
	if out.NumNodes() != 2 {
		t.Fatalf("expected 2 nodes, got %d", out.NumNodes())
	}
}

func TestAliasNonIP_NoMerge(t *testing.T) {
	g := buildGraph([]string{"Seattle", "Portland"}, [][2]int{{0, 1}})
	out := ResolveAliases(g)
	if out.NumNodes() != 2 {
		t.Fatalf("expected 2 nodes, got %d", out.NumNodes())
	}
}

func TestAliasEmptyGraph(t *testing.T) {
	g := New()
	out := ResolveAliases(g)
	if out.NumNodes() != 0 {
		t.Fatalf("expected 0 nodes, got %d", out.NumNodes())
	}
}

func TestAliasIntegration(t *testing.T) {
	// Two paths sharing a router with two IPs in same /30, different /31s.
	ms := []tomo.PathMeasurement{
		{Hops: []tomo.Hop{{IP: "10.0.0.0"}, {IP: "10.0.0.1"}, {IP: "10.0.0.5"}}},
		{Hops: []tomo.Hop{{IP: "10.0.0.3"}, {IP: "10.0.0.2"}, {IP: "10.0.0.9"}}},
	}
	g, _, _, err := InferFromMeasurements(ms, InferOpts{AliasResolution: true})
	if err != nil {
		t.Fatal(err)
	}
	// In /30=10.0.0.0: nodes .0,.1,.3,.2. .0 and .1 share /31 (no merge).
	// .3 and .2 share /31 (no merge). But .1,.3,.2 merge (diff /31, unconnected).
	// Result: node0, merged(1,3,4), node2(10.0.0.5), node5(10.0.0.9) = 4 nodes
	if g.NumNodes() != 4 {
		t.Fatalf("expected 4 nodes after alias resolution, got %d", g.NumNodes())
	}
}
