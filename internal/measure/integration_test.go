package measure

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
)

// --- Cache tests ---

func TestIntegration_CacheStoreLoad(t *testing.T) {
	c := NewCache(t.TempDir())
	data := []byte(`{"foo":"bar"}`)
	key := c.Key("test", "store")
	if err := c.Store(key, data); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	got, err := c.Load(key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("Load returned %q, want %q", got, data)
	}
}

func TestIntegration_CacheHasMissing(t *testing.T) {
	c := NewCache(t.TempDir())
	if c.Has("nonexistent") {
		t.Fatal("Has returned true for missing key")
	}
}

func TestIntegration_CacheHasAfterStore(t *testing.T) {
	c := NewCache(t.TempDir())
	key := c.Key("x")
	if err := c.Store(key, []byte(`{}`)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if !c.Has(key) {
		t.Fatal("Has returned false after Store")
	}
}

func TestIntegration_CacheLoadMissingError(t *testing.T) {
	c := NewCache(t.TempDir())
	_, err := c.Load("missing")
	if err == nil {
		t.Fatal("expected error on Load of missing key")
	}
}

func TestIntegration_CacheKeyDeterministic(t *testing.T) {
	c := NewCache(t.TempDir())
	k1 := c.Key("a", "b", "c")
	k2 := c.Key("a", "b", "c")
	if k1 != k2 {
		t.Fatalf("keys differ: %q vs %q", k1, k2)
	}
}

func TestIntegration_CacheCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)
	key := "corrupt"
	// Write garbage directly to the cache file.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	garbage := []byte{0xff, 0xfe, 0x00, 0x01, 0x02}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	// Load should succeed (it reads raw bytes), but the data is garbage.
	data, err := c.Load(key)
	if err != nil {
		t.Fatalf("Load on corrupted file should not error (raw read): %v", err)
	}
	if len(data) != len(garbage) {
		t.Fatalf("expected %d bytes, got %d", len(garbage), len(data))
	}
}

// --- Simulation tests ---

func newSmallTopo() *topology.Graph {
	return topology.BarabasiAlbert(8, 2, 99)
}

// newLineTopo builds a simple line topology: 0 — 1 — 2 — 3.
// This is fully deterministic (no randomness in construction, unique shortest paths).
func newLineTopo() *topology.Graph {
	g := topology.New()
	for i := 0; i < 4; i++ {
		g.AddNode(tomo.Node{ID: i, Label: fmt.Sprintf("n%d", i), Latitude: float64(i), Longitude: float64(i)})
	}
	g.AddLink(0, 1)
	g.AddLink(1, 2)
	g.AddLink(2, 3)
	return g
}

func TestIntegration_SimDeterministic(t *testing.T) {
	topo := newLineTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 123
	r1, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatalf("Simulate 1 failed: %v", err)
	}
	r2, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatalf("Simulate 2 failed: %v", err)
	}
	if len(r1.GroundTruth) != len(r2.GroundTruth) {
		t.Fatalf("ground truth length mismatch: %d vs %d", len(r1.GroundTruth), len(r2.GroundTruth))
	}
	for i := range r1.GroundTruth {
		if r1.GroundTruth[i] != r2.GroundTruth[i] {
			t.Fatalf("GroundTruth[%d] differs: %f vs %f", i, r1.GroundTruth[i], r2.GroundTruth[i])
		}
	}
	if len(r1.NoiseFree) != len(r2.NoiseFree) {
		t.Fatalf("NoiseFree length mismatch: %d vs %d", len(r1.NoiseFree), len(r2.NoiseFree))
	}
	for i := range r1.NoiseFree {
		if r1.NoiseFree[i] != r2.NoiseFree[i] {
			t.Fatalf("NoiseFree[%d] differs: %f vs %f", i, r1.NoiseFree[i], r2.NoiseFree[i])
		}
	}
}

func TestIntegration_SimDifferentSeeds(t *testing.T) {
	topo := newSmallTopo()
	cfg1 := DefaultSimConfig()
	cfg1.Seed = 1
	cfg2 := DefaultSimConfig()
	cfg2.Seed = 2
	r1, err := Simulate(topo, cfg1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Simulate(topo, cfg2)
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range r1.GroundTruth {
		if r1.GroundTruth[i] != r2.GroundTruth[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical ground truth")
	}
}

func TestIntegration_SimNoiseScaleZero(t *testing.T) {
	topo := newSmallTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.NoiseScale = 0
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i, nf := range r.NoiseFree {
		measured := r.Problem.B.AtVec(i)
		if math.Abs(measured-nf) > 1e-12 {
			t.Fatalf("measurement[%d]=%f != noise-free=%f with NoiseScale=0", i, measured, nf)
		}
	}
}

func TestIntegration_SimPathFractionSmall(t *testing.T) {
	topo := newSmallTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.PathFraction = 0.01
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Paths) < 1 {
		t.Fatal("PathFraction=0.01 should produce at least 1 path")
	}
}

func TestIntegration_SimSamplesPerPathOne(t *testing.T) {
	topo := newSmallTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.SamplesPerPath = 1
	// With only 1 sample, the measurement IS the single sample (no min-of-N reduction).
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NoiseFree) == 0 {
		t.Fatal("expected non-empty results")
	}
}

func TestIntegration_SimCongestionLinksExceedsNLinks(t *testing.T) {
	topo := newSmallTopo()
	nLinks := topo.NumLinks()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.CongestionLinks = nLinks + 100
	// Should not panic; clamps via min().
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.GroundTruth) != nLinks {
		t.Fatalf("expected %d ground truth entries, got %d", nLinks, len(r.GroundTruth))
	}
}

func TestIntegration_SimCongestionFactorOne(t *testing.T) {
	topo := newSmallTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.CongestionFactor = 1.0
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// CongestionFactor=1.0 means the condition (> 1.0) is false, so no congestion applied.
	// Re-run with CongestionLinks=0 to get baseline.
	cfg2 := cfg
	cfg2.CongestionLinks = 0
	r2, err := Simulate(topo, cfg2)
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.GroundTruth {
		if r.GroundTruth[i] != r2.GroundTruth[i] {
			t.Fatalf("CongestionFactor=1.0 should not alter delays, link %d: %f vs %f",
				i, r.GroundTruth[i], r2.GroundTruth[i])
		}
	}
}

func TestIntegration_SimInvalidNoiseModel(t *testing.T) {
	topo := newSmallTopo()
	cfg := DefaultSimConfig()
	cfg.Seed = 42
	cfg.NoiseModel = "invalid"
	// Should fall through to lognormal default, no error.
	r, err := Simulate(topo, cfg)
	if err != nil {
		t.Fatalf("invalid noise model should not error: %v", err)
	}
	if len(r.NoiseFree) == 0 {
		t.Fatal("expected non-empty results")
	}
}
