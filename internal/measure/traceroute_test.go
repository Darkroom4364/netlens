package measure

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "measurements")
}

func TestParseRIPEAtlasTraceroute(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testdataDir(), "ripe_atlas_sample.json"))
	if err != nil {
		t.Fatalf("read test data: %v", err)
	}

	measurements, err := ParseRIPEAtlasTraceroute(data)
	if err != nil {
		t.Fatalf("ParseRIPEAtlasTraceroute: %v", err)
	}

	if len(measurements) != 2 {
		t.Fatalf("got %d measurements, want 2", len(measurements))
	}

	// First measurement: 5 hops, one anonymous, one MPLS
	m1 := measurements[0]
	if m1.Src != "192.168.1.2" {
		t.Errorf("m1.Src = %s, want 192.168.1.2", m1.Src)
	}
	if m1.Dst != "8.8.8.8" {
		t.Errorf("m1.Dst = %s, want 8.8.8.8", m1.Dst)
	}
	if len(m1.Hops) != 5 {
		t.Fatalf("m1 hops = %d, want 5", len(m1.Hops))
	}

	// Hop 1: normal
	if m1.Hops[0].IP != "192.168.1.1" || m1.Hops[0].Anonymous {
		t.Errorf("hop 1: got IP=%s anon=%v, want 192.168.1.1 anon=false", m1.Hops[0].IP, m1.Hops[0].Anonymous)
	}

	// Hop 3: anonymous (* * *)
	if !m1.Hops[2].Anonymous {
		t.Error("hop 3 should be anonymous")
	}

	// Hop 4: MPLS
	if !m1.Hops[3].MPLS {
		t.Error("hop 4 should have MPLS flag")
	}

	// End-to-end RTTs from last hop (3 samples)
	if len(m1.RTTs) != 3 {
		t.Errorf("m1 RTTs = %d, want 3", len(m1.RTTs))
	}

	// MinRTT should be the smallest sample
	minRTT := m1.MinRTT()
	if minRTT.Milliseconds() < 15 || minRTT.Milliseconds() > 16 {
		t.Errorf("m1 MinRTT = %v, want ~15ms", minRTT)
	}

	// Second measurement: 4 hops, no anonymous, no MPLS
	m2 := measurements[1]
	if len(m2.Hops) != 4 {
		t.Fatalf("m2 hops = %d, want 4", len(m2.Hops))
	}
	for i, h := range m2.Hops {
		if h.Anonymous {
			t.Errorf("m2 hop %d should not be anonymous", i+1)
		}
	}

	t.Logf("Parsed %d measurements: m1=%d hops (1 anon, 1 mpls), m2=%d hops",
		len(measurements), len(m1.Hops), len(m2.Hops))
}
