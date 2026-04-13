package topology

import "testing"

func TestBarabasiAlbert(t *testing.T) {
	g := BarabasiAlbert(50, 3, 42)
	if g.NumNodes() != 50 {
		t.Fatalf("expected 50 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() <= 100 {
		t.Fatalf("expected >100 edges, got %d", g.NumLinks())
	}
	// Check connectivity via a shortest path.
	if _, ok := g.ShortestPath(0, 49); !ok {
		t.Fatal("graph is not connected: no path from 0 to 49")
	}
}

func TestWaxman(t *testing.T) {
	g := Waxman(50, 0.4, 0.4, 42)
	if g.NumNodes() != 50 {
		t.Fatalf("expected 50 nodes, got %d", g.NumNodes())
	}
	if g.NumLinks() == 0 {
		t.Fatal("expected some edges")
	}
	// Connectivity guaranteed by construction.
	if _, ok := g.ShortestPath(0, 49); !ok {
		t.Fatal("graph is not connected: no path from 0 to 49")
	}
}
