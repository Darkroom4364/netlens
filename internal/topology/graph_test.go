package topology

import (
	"math"
	"testing"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// ---------- 1. AddNode with duplicate ID ----------

func TestGraphAddNodeDuplicateID(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 1, Label: "A"})

	// gonum panics on duplicate node ID. Verify the panic occurs.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate node ID, but none occurred")
		}
	}()
	g.AddNode(tomo.Node{ID: 1, Label: "B"})
}

// ---------- 2. Negative node ID ----------

func TestGraphNegativeNodeID(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: -1, Label: "neg"})
	g.AddNode(tomo.Node{ID: -2, Label: "neg2"})
	idx := g.AddLink(-1, -2)
	if idx < 0 {
		t.Fatal("expected valid link index for negative node IDs, got", idx)
	}
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link, got %d", g.NumLinks())
	}
	if g.NumNodes() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NumNodes())
	}
}

// ---------- 3. LinkIndex for nonexistent link ----------

func TestGraphLinkIndexNonexistent(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0})
	g.AddNode(tomo.Node{ID: 1})
	// No link added between 0 and 1.
	if idx := g.LinkIndex(0, 1); idx != -1 {
		t.Errorf("expected -1 for nonexistent link, got %d", idx)
	}
	// Completely unknown nodes.
	if idx := g.LinkIndex(99, 100); idx != -1 {
		t.Errorf("expected -1 for unknown nodes, got %d", idx)
	}
}

// ---------- 4. GeoDistance at poles ----------

func TestGraphGeoDistancePoles(t *testing.T) {
	north := tomo.Node{ID: 0, Latitude: 90, Longitude: 0}
	south := tomo.Node{ID: 1, Latitude: -90, Longitude: 0}

	d := GeoDistance(north, south)
	// Half circumference ~ 20015 km
	if d < 19900 || d > 20100 {
		t.Errorf("pole-to-pole distance should be ~20015 km, got %.1f", d)
	}
}

// ---------- 5. GeoDistance across antimeridian ----------

func TestGraphGeoDistanceAntimeridian(t *testing.T) {
	a := tomo.Node{ID: 0, Latitude: 0, Longitude: 179}
	b := tomo.Node{ID: 1, Latitude: 0, Longitude: -179}

	d := GeoDistance(a, b)
	// 2 degrees at equator ~ 222 km
	if d < 200 || d > 250 {
		t.Errorf("antimeridian crossing should be ~222 km, got %.1f", d)
	}
}

// ---------- 6. GeoDistance with zero coordinates ----------

func TestGraphGeoDistanceZeroCoords(t *testing.T) {
	// Both lat and lon are 0 => treated as "no coordinates"
	a := tomo.Node{ID: 0, Latitude: 0, Longitude: 0}
	b := tomo.Node{ID: 1, Latitude: 45, Longitude: 90}

	d := GeoDistance(a, b)
	if d != 0 {
		t.Errorf("expected 0 for node with zero coords, got %f", d)
	}

	// NaN coordinates: math operations with NaN produce NaN, but since
	// lat=0 && lon=0 check doesn't catch NaN, test that separately.
	nanNode := tomo.Node{ID: 2, Latitude: math.NaN(), Longitude: math.NaN()}
	d2 := GeoDistance(nanNode, b)
	// NaN coords won't match the zero-check, so we get NaN output.
	// Just verify no panic occurs. The result will be NaN.
	_ = d2
}

// ---------- 7. GeoDistance same point ----------

func TestGraphGeoDistanceSamePoint(t *testing.T) {
	a := tomo.Node{ID: 0, Latitude: 47.3769, Longitude: 8.5417} // Zurich
	d := GeoDistance(a, a)
	if d != 0 {
		t.Errorf("same point distance should be 0, got %f", d)
	}
}

// ---------- 8. Neighbors on node with no links ----------

func TestGraphNeighborsNoLinks(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0})
	g.AddNode(tomo.Node{ID: 1})
	// No links added.
	neighbors := g.Neighbors(0)
	if len(neighbors) != 0 {
		t.Errorf("expected empty neighbors for isolated node, got %v", neighbors)
	}
}

// ---------- 9. Neighbors on nonexistent node ----------

func TestGraphNeighborsNonexistent(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0})
	// Query node 999 which was never added.
	neighbors := g.Neighbors(999)
	if len(neighbors) != 0 {
		t.Errorf("expected empty neighbors for nonexistent node, got %v", neighbors)
	}
}

// ---------- 10. ShortestPath src == dst ----------

func TestGraphShortestPathSrcEqualsDst(t *testing.T) {
	g := New()
	g.AddNode(tomo.Node{ID: 0})
	g.AddNode(tomo.Node{ID: 1})
	g.AddLink(0, 1)

	path, ok := g.ShortestPath(0, 0)
	if ok {
		t.Errorf("expected ok=false for src==dst, got true with path %v", path)
	}
	if path != nil {
		t.Errorf("expected nil path for src==dst, got %v", path)
	}
}
