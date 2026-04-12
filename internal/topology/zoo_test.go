package topology

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "topologies")
}

func TestLoadAbilene(t *testing.T) {
	g, err := LoadGraphML(filepath.Join(testdataDir(), "abilene.graphml"))
	if err != nil {
		t.Fatalf("LoadGraphML: %v", err)
	}

	if g.NumNodes() != 11 {
		t.Errorf("NumNodes = %d, want 11", g.NumNodes())
	}
	if g.NumLinks() != 14 {
		t.Errorf("NumLinks = %d, want 14", g.NumLinks())
	}

	// Check that labels were extracted
	foundSeattle := false
	for _, n := range g.Nodes() {
		if n.Label == "Seattle" {
			foundSeattle = true
		}
		// All nodes should have coordinates (from yEd geometry)
		if n.Latitude == 0 && n.Longitude == 0 {
			t.Errorf("node %d (%s) has no coordinates", n.ID, n.Label)
		}
	}
	if !foundSeattle {
		t.Error("expected to find node labeled 'Seattle'")
	}

	// Check shortest path exists
	linkIDs, ok := g.ShortestPath(0, 10)
	if !ok {
		t.Error("expected path from node 0 to node 10")
	}
	if len(linkIDs) == 0 {
		t.Error("expected non-empty path")
	}
}

func TestLoadAllTopologies(t *testing.T) {
	files := []struct {
		name      string
		minNodes  int
		minLinks  int
	}{
		{"abilene.graphml", 11, 14},
		{"geant2012.graphml", 30, 40},
		{"attmpls.graphml", 20, 20},
		{"sprint.graphml", 10, 10},
		{"dfn.graphml", 30, 30},
	}

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			g, err := LoadGraphML(filepath.Join(testdataDir(), f.name))
			if err != nil {
				t.Fatalf("LoadGraphML(%s): %v", f.name, err)
			}
			if g.NumNodes() < f.minNodes {
				t.Errorf("NumNodes = %d, want >= %d", g.NumNodes(), f.minNodes)
			}
			if g.NumLinks() < f.minLinks {
				t.Errorf("NumLinks = %d, want >= %d", g.NumLinks(), f.minLinks)
			}

			// All topologies should produce paths
			paths := g.AllPairsShortestPaths()
			if len(paths) == 0 {
				t.Error("expected at least one path")
			}
			t.Logf("%s: %d nodes, %d links, %d paths", f.name, g.NumNodes(), g.NumLinks(), len(paths))
		})
	}
}
