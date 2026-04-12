package topology

import (
	"fmt"
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// ---------- InferFromMeasurements edge cases ----------

func TestEdge_SingleHopPath(t *testing.T) {
	ms := []tomo.PathMeasurement{{
		Hops: []tomo.Hop{{IP: "10.0.0.1", RTT: time.Millisecond}},
	}}
	g, specs, idx, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single hop cannot form a link — need at least 2 visible endpoints.
	if len(specs) != 0 {
		t.Errorf("expected 0 path specs, got %d", len(specs))
	}
	if len(idx) != 0 {
		t.Errorf("expected 0 accepted indices, got %d", len(idx))
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
}

func TestEdge_AllHopsAnonymous(t *testing.T) {
	hops := make([]tomo.Hop, 5)
	for i := range hops {
		hops[i] = tomo.Hop{Anonymous: true}
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	// With default threshold (0.3), 100% anonymous should be discarded.
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 path specs for all-anonymous path, got %d", len(specs))
	}
	if g.NumNodes() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NumNodes())
	}
}

func TestEdge_AllHopsMPLS(t *testing.T) {
	hops := make([]tomo.Hop, 5)
	for i := range hops {
		hops[i] = tomo.Hop{IP: fmt.Sprintf("10.0.0.%d", i+1), MPLS: true}
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All MPLS hops are skipped — no visible nodes, no links, no path specs.
	if len(specs) != 0 {
		t.Errorf("expected 0 path specs for all-MPLS path, got %d", len(specs))
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
}

func TestEdge_MaxAnonymousFracOne(t *testing.T) {
	// 4 out of 5 hops anonymous; with threshold 1.0 it should be accepted.
	hops := []tomo.Hop{
		{IP: "10.0.0.1"},
		{Anonymous: true},
		{Anonymous: true},
		{Anonymous: true},
		{IP: "10.0.0.2"},
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	_, specs, idx, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 path spec, got %d", len(specs))
	}
	if len(idx) != 1 || idx[0] != 0 {
		t.Errorf("expected accepted index [0], got %v", idx)
	}
}

func TestEdge_MaxAnonymousFracNearZero(t *testing.T) {
	// Even 1 anonymous hop out of 5 is 20%, which exceeds 0.0001.
	hops := []tomo.Hop{
		{IP: "10.0.0.1"},
		{Anonymous: true},
		{IP: "10.0.0.2"},
		{IP: "10.0.0.3"},
		{IP: "10.0.0.4"},
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	_, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 0.0001})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 path specs with near-zero threshold, got %d", len(specs))
	}
}

func TestEdge_SameIPAtEveryHop(t *testing.T) {
	hops := make([]tomo.Hop, 10)
	for i := range hops {
		hops[i] = tomo.Hop{IP: "1.1.1.1"}
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All hops map to the same node; self-loops are skipped, so no links.
	// A path spec is still emitted (with empty LinkIDs) because visibleNodeIDs >= 2 entries.
	if g.NumNodes() != 1 {
		t.Errorf("expected 1 node, got %d", g.NumNodes())
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec (with empty link list), got %d", len(specs))
	}
	if len(specs) == 1 && len(specs[0].LinkIDs) != 0 {
		t.Errorf("expected 0 link IDs in path spec, got %d", len(specs[0].LinkIDs))
	}
}

func TestEdge_AlternatingAnonymousHops(t *testing.T) {
	hops := []tomo.Hop{
		{IP: "10.0.0.1"},
		{Anonymous: true},
		{IP: "10.0.0.2"},
		{Anonymous: true},
		{IP: "10.0.0.3"},
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	// 2/5 = 40% anonymous; use threshold 0.5 so it's accepted.
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 path spec, got %d", len(specs))
	}
	// Anonymous hops skipped: links are 10.0.0.1→10.0.0.2 and 10.0.0.2→10.0.0.3.
	if g.NumLinks() != 2 {
		t.Errorf("expected 2 links, got %d", g.NumLinks())
	}
}

func TestEdge_1000IdenticalMeasurements(t *testing.T) {
	hops := []tomo.Hop{
		{IP: "10.0.0.1"},
		{IP: "10.0.0.2"},
		{IP: "10.0.0.3"},
	}
	ms := make([]tomo.PathMeasurement, 1000)
	for i := range ms {
		ms[i] = tomo.PathMeasurement{Hops: hops}
	}
	g, specs, idx, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Duplicate links deduplicated in graph.
	if g.NumLinks() != 2 {
		t.Errorf("expected 2 links, got %d", g.NumLinks())
	}
	if len(specs) != 1000 {
		t.Errorf("expected 1000 path specs, got %d", len(specs))
	}
	if len(idx) != 1000 {
		t.Errorf("expected 1000 accepted indices, got %d", len(idx))
	}
}

func TestEdge_TwoDisjointPaths(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Hops: []tomo.Hop{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}}},
		{Hops: []tomo.Hop{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}}},
	}
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 2 {
		t.Errorf("expected 2 links, got %d", g.NumLinks())
	}
	if len(specs) != 2 {
		t.Errorf("expected 2 path specs, got %d", len(specs))
	}
}

func TestEdge_PathWith100Hops(t *testing.T) {
	hops := make([]tomo.Hop, 100)
	for i := range hops {
		hops[i] = tomo.Hop{IP: fmt.Sprintf("10.%d.%d.%d", i/256/256%256, i/256%256, i%256)}
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 100 {
		t.Errorf("expected 100 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 99 {
		t.Errorf("expected 99 links, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec, got %d", len(specs))
	}
}

func TestEdge_MixedIPv4AndIPv6(t *testing.T) {
	hops := []tomo.Hop{
		{IP: "10.0.0.1"},
		{IP: "2001:db8::1"},
		{IP: "10.0.0.2"},
		{IP: "2001:db8::2"},
	}
	ms := []tomo.PathMeasurement{{Hops: hops}}
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 3 {
		t.Errorf("expected 3 links, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec, got %d", len(specs))
	}
}

// ---------- Graph edge cases ----------

func TestEdge_SelfLoop(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "n0"})
	// gonum's UndirectedGraph panics on self-edges. Verify AddLink does not
	// propagate that panic (if it guards against it) or document the behavior.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AddLink(0,0) panicked: %v — self-loops should be handled gracefully", r)
		}
	}()
	g.AddLink(0, 0)
}

func TestEdge_DuplicateLink(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "n0"})
	g.AddNode(tomo.Node{ID: 1, Label: "n1"})
	idx1 := g.AddLink(0, 1)
	idx2 := g.AddLink(0, 1)
	if idx1 != idx2 {
		t.Errorf("duplicate AddLink should return same index: %d vs %d", idx1, idx2)
	}
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link after duplicate add, got %d", g.NumLinks())
	}
}

func TestEdge_IsolatedNode(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "n0"})
	g.AddNode(tomo.Node{ID: 1, Label: "n1"})
	g.AddNode(tomo.Node{ID: 2, Label: "n2"})
	g.AddLink(0, 1)

	neighbors := g.Neighbors(2)
	if len(neighbors) != 0 {
		t.Errorf("isolated node should have no neighbors, got %v", neighbors)
	}
}

func TestEdge_ShortestPathUnreachable(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "n0"})
	g.AddNode(tomo.Node{ID: 1, Label: "n1"})
	// No link between them.
	path, ok := g.ShortestPath(0, 1)
	if ok {
		t.Errorf("expected unreachable, got path %v", path)
	}
}

func TestEdge_AllPairsShortestPathsDisconnected(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0, Label: "n0"})
	g.AddNode(tomo.Node{ID: 1, Label: "n1"})
	g.AddNode(tomo.Node{ID: 2, Label: "n2"})
	g.AddLink(0, 1)
	// Node 2 is disconnected.
	paths := g.AllPairsShortestPaths()
	// Only (0,1) should be reachable.
	if len(paths) != 1 {
		t.Errorf("expected 1 path in disconnected graph, got %d", len(paths))
	}
}

// ---------- GraphML edge cases ----------

func TestEdge_EmptyGraphML(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<graphml><graph></graph></graphml>`)
	g, err := ParseGraphML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
}

func TestEdge_GraphMLNodesNoEdges(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<graphml><graph>
  <node id="0"/>
  <node id="1"/>
  <node id="2"/>
</graph></graphml>`)
	g, err := ParseGraphML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
}

func TestEdge_GraphMLEdgeNonexistentNode(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<graphml><graph>
  <node id="0"/>
  <edge source="0" target="999"/>
</graph></graphml>`)
	g, err := ParseGraphML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Edge referencing nonexistent node is silently skipped.
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links for edge with nonexistent target, got %d", g.NumLinks())
	}
}

func TestEdge_GraphMLBinaryGarbage(t *testing.T) {
	data := []byte{0x00, 0xFF, 0xFE, 0xCA, 0xDE, 0xBA, 0xBE, 0x13, 0x37}
	_, err := ParseGraphML(data)
	if err == nil {
		t.Error("expected error for binary garbage, got nil")
	}
}

// ---------- Synthetic generator edge cases ----------

func TestEdge_BarabasiAlbertSingleNode(t *testing.T) {
	g := BarabasiAlbert(1, 1, 0)
	// n=1, m=1: seed graph is m+1=2 nodes, but n=1 means we only want 1.
	// The seed graph creates m+1 nodes, so it could be 2. Just ensure no panic.
	if g.NumNodes() < 1 {
		t.Errorf("expected at least 1 node, got %d", g.NumNodes())
	}
}

func TestEdge_BarabasiAlbertTwoNodes(t *testing.T) {
	g := BarabasiAlbert(2, 1, 0)
	// Seed graph: m+1=2 nodes with one link. n=2 so no growth phase.
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link, got %d", g.NumLinks())
	}
}

func TestEdge_WaxmanSingleNode(t *testing.T) {
	g := Waxman(1, 0.5, 0.5, 0)
	if g.NumNodes() != 1 {
		t.Errorf("expected 1 node, got %d", g.NumNodes())
	}
	if g.NumLinks() != 0 {
		t.Errorf("expected 0 links, got %d", g.NumLinks())
	}
}

func TestEdge_WaxmanAlphaZero(t *testing.T) {
	// alpha=0 means division by zero in exp(-d/(alpha*L)). Should not panic.
	g := Waxman(5, 0, 0.5, 0)
	if g.NumNodes() != 5 {
		t.Errorf("expected 5 nodes, got %d", g.NumNodes())
	}
}

func TestEdge_WaxmanBetaOne(t *testing.T) {
	// beta=1.0 means maximum connection probability.
	g := Waxman(10, 0.5, 1.0, 42)
	if g.NumNodes() != 10 {
		t.Errorf("expected 10 nodes, got %d", g.NumNodes())
	}
	// Should be connected (Waxman ensures connectivity via union-find).
	paths := g.AllPairsShortestPaths()
	// 10 nodes fully connected means C(10,2)=45 pairs.
	if len(paths) != 45 {
		t.Errorf("expected 45 reachable pairs, got %d", len(paths))
	}
}
