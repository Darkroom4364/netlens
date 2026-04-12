package topology

import (
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

func hop(ip string) tomo.Hop {
	return tomo.Hop{IP: ip, RTT: time.Millisecond, TTL: 1}
}

func anonHop() tomo.Hop {
	return tomo.Hop{Anonymous: true, TTL: 1}
}

func mplsHop(ip string) tomo.Hop {
	return tomo.Hop{IP: ip, RTT: time.Millisecond, TTL: 1, MPLS: true}
}

func TestInferFromMeasurements_EmptyInput(t *testing.T) {
	_, _, _, err := InferFromMeasurements(nil, InferOpts{})
	if err == nil {
		t.Fatal("expected error for nil measurements")
	}
	_, _, _, err = InferFromMeasurements([]tomo.PathMeasurement{}, InferOpts{})
	if err == nil {
		t.Fatal("expected error for empty measurements")
	}
}

func TestInferFromMeasurements_SinglePath(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
				hop("10.0.0.3"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.NumNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() != 2 {
		t.Errorf("expected 2 links, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 path spec, got %d", len(specs))
	}
	if len(specs[0].LinkIDs) != 2 {
		t.Errorf("expected 2 link IDs in path spec, got %d", len(specs[0].LinkIDs))
	}
}

func TestInferFromMeasurements_TwoPaths_SharedLink(t *testing.T) {
	// Path 1: A -> B -> C
	// Path 2: A -> B -> D
	// Shared link: A-B
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
				hop("10.0.0.3"),
			},
		},
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.4",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
				hop("10.0.0.4"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.NumNodes() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.NumNodes())
	}
	// Links: A-B, B-C, B-D = 3
	if g.NumLinks() != 3 {
		t.Errorf("expected 3 links, got %d", g.NumLinks())
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 path specs, got %d", len(specs))
	}

	// Both paths should share the A-B link (index 0).
	if specs[0].LinkIDs[0] != specs[1].LinkIDs[0] {
		t.Errorf("expected shared first link, got %d and %d", specs[0].LinkIDs[0], specs[1].LinkIDs[0])
	}
}

func TestInferFromMeasurements_AnonymousHopsSkipped(t *testing.T) {
	// Path: A -> * -> C (anonymous hop in the middle)
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				anonHop(),
				hop("10.0.0.3"),
			},
		},
	}

	// 1 out of 3 hops is anonymous (33%), so we set threshold to 0.5 to accept it.
	g, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes (anonymous hop skipped), got %d", g.NumNodes())
	}
	// A directly linked to C (bridging the gap).
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link, got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 path spec, got %d", len(specs))
	}
	if len(specs[0].LinkIDs) != 1 {
		t.Errorf("expected 1 link in path spec, got %d", len(specs[0].LinkIDs))
	}
}

func TestInferFromMeasurements_DiscardHighAnonymousFraction(t *testing.T) {
	// Path with 3 out of 4 hops anonymous (75%) → discarded at default 30%.
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.5",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				anonHop(),
				anonHop(),
				anonHop(),
			},
		},
		// Good path to ensure we still get results.
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.2",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the second (good) path should be accepted.
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec (bad path discarded), got %d", len(specs))
	}
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
}

func TestInferFromMeasurements_CustomMaxAnonymousFrac(t *testing.T) {
	// 1 out of 3 hops anonymous (33%). Default (30%) would discard, but 0.5 keeps it.
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				anonHop(),
				hop("10.0.0.3"),
			},
		},
	}

	_, specs, _, err := InferFromMeasurements(ms, InferOpts{MaxAnonymousFrac: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec with relaxed threshold, got %d", len(specs))
	}
}

func TestInferFromMeasurements_MPLSHopsSkipped(t *testing.T) {
	// Path: A -> [MPLS-B] -> C — MPLS hop should not create node/link.
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.3",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				mplsHop("10.0.0.2"),
				hop("10.0.0.3"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// MPLS hop should not become a node.
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes (MPLS hop hidden), got %d", g.NumNodes())
	}
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link (bridging MPLS tunnel), got %d", g.NumLinks())
	}
	if len(specs) != 1 || len(specs[0].LinkIDs) != 1 {
		t.Errorf("expected path spec with 1 link, got %v", specs)
	}
}

func TestInferFromMeasurements_DuplicateIPsNoPanic(t *testing.T) {
	// Same IP appearing consecutively (routing loop or load balancer).
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.2",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.1"),
				hop("10.0.0.2"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
	// Self-loop skipped, so only 1 link.
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link (self-loop skipped), got %d", g.NumLinks())
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 path spec, got %d", len(specs))
	}
}

func TestInferFromMeasurements_DiamondTopology(t *testing.T) {
	//     B
	//    / \
	//   A   D
	//    \ /
	//     C
	// Path 1: A -> B -> D
	// Path 2: A -> C -> D
	ms := []tomo.PathMeasurement{
		{
			Src: "A",
			Dst: "D",
			Hops: []tomo.Hop{
				hop("192.168.1.1"),
				hop("192.168.1.2"),
				hop("192.168.1.4"),
			},
		},
		{
			Src: "A",
			Dst: "D",
			Hops: []tomo.Hop{
				hop("192.168.1.1"),
				hop("192.168.1.3"),
				hop("192.168.1.4"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.NumNodes() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.NumNodes())
	}
	// Links: A-B, B-D, A-C, C-D = 4
	if g.NumLinks() != 4 {
		t.Errorf("expected 4 links, got %d", g.NumLinks())
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 path specs, got %d", len(specs))
	}
	// Each path should have exactly 2 links.
	for i, s := range specs {
		if len(s.LinkIDs) != 2 {
			t.Errorf("path %d: expected 2 links, got %d", i, len(s.LinkIDs))
		}
	}
}

func TestInferFromMeasurements_AllAnonymousPath(t *testing.T) {
	// Path with all anonymous hops should be discarded, and another valid path keeps things working.
	ms := []tomo.PathMeasurement{
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.5",
			Hops: []tomo.Hop{
				anonHop(),
				anonHop(),
			},
		},
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.2",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("expected 1 path spec (all-anon discarded), got %d", len(specs))
	}
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
}

func TestInferFromMeasurements_EmptyHopsSkipped(t *testing.T) {
	// Measurement with no hops should be silently skipped.
	ms := []tomo.PathMeasurement{
		{Src: "10.0.0.1", Dst: "10.0.0.2", Hops: nil},
		{
			Src: "10.0.0.1",
			Dst: "10.0.0.2",
			Hops: []tomo.Hop{
				hop("10.0.0.1"),
				hop("10.0.0.2"),
			},
		},
	}

	g, specs, _, err := InferFromMeasurements(ms, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("expected 1 path spec, got %d", len(specs))
	}
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
}
