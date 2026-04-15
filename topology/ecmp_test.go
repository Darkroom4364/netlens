package topology

import (
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
)

func TestECMP_TwoDifferentPaths(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.1.1"}, {IP: "10.0.1.2"}}},
	}
	results := DetectECMP(ms)
	if len(results) != 1 {
		t.Fatalf("expected 1 ECMP result, got %d", len(results))
	}
	if results[0].NumPaths != 2 {
		t.Fatalf("expected NumPaths=2, got %d", results[0].NumPaths)
	}
}

func TestECMP_IdenticalPaths(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}}},
	}
	results := DetectECMP(ms)
	if len(results) != 0 {
		t.Fatalf("expected 0 ECMP results for identical paths, got %d", len(results))
	}
}

func TestECMP_EmptyMeasurements(t *testing.T) {
	results := DetectECMP(nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for nil input, got %d", len(results))
	}
	results = DetectECMP([]tomo.PathMeasurement{})
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestECMP_SingleMeasurement(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}}},
	}
	results := DetectECMP(ms)
	if len(results) != 0 {
		t.Fatalf("expected 0 ECMP results for single measurement, got %d", len(results))
	}
}

func TestECMP_ThreeDistinctPaths(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.1.1"}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.2.1"}}},
	}
	results := DetectECMP(ms)
	if len(results) != 1 {
		t.Fatalf("expected 1 ECMP result, got %d", len(results))
	}
	if results[0].NumPaths != 3 {
		t.Fatalf("expected NumPaths=3, got %d", results[0].NumPaths)
	}
}

func TestECMP_DeduplicateKeepsFewestAnonymous(t *testing.T) {
	// Both measurements have anonymous hops so dedup must actually compare counts.
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {Anonymous: true}, {Anonymous: true}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.2"}, {Anonymous: true}}},
	}
	deduped := DeduplicateECMP(ms)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 deduplicated measurement, got %d", len(deduped))
	}
	anon := countAnonymous(deduped[0])
	if anon != 1 {
		t.Fatalf("expected 1 anonymous hop in winner, got %d", anon)
	}
	if deduped[0].Hops[0].IP != "10.0.0.2" {
		t.Fatalf("expected measurement with fewer anonymous hops to win, got IP %s", deduped[0].Hops[0].IP)
	}
}

func TestECMP_DeduplicateTiePicksFirst(t *testing.T) {
	// Both measurements have exactly 1 anonymous hop each — a true tie.
	// The first measurement should win because the comparator uses < (not <=).
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {Anonymous: true}, {IP: "10.0.0.2"}}},
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}, {IP: "10.0.0.3"}, {Anonymous: true}}},
	}
	deduped := DeduplicateECMP(ms)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 deduplicated measurement, got %d", len(deduped))
	}
	// Both have 1 anonymous hop; first should win the tie.
	if deduped[0].Hops[1].Anonymous != true {
		t.Fatalf("expected first measurement (anonymous at index 1) to win tie")
	}
	if deduped[0].Hops[2].IP != "10.0.0.2" {
		t.Fatalf("expected first measurement to win tie, got hop[2] IP %s", deduped[0].Hops[2].IP)
	}
}

func TestECMP_DifferentSrcDstNoCross(t *testing.T) {
	ms := []tomo.PathMeasurement{
		{Src: "A", Dst: "B", Hops: []tomo.Hop{{IP: "10.0.0.1"}}},
		{Src: "C", Dst: "D", Hops: []tomo.Hop{{IP: "10.0.1.1"}}},
	}
	results := DetectECMP(ms)
	if len(results) != 0 {
		t.Fatalf("expected 0 ECMP results for different (src,dst) pairs, got %d", len(results))
	}
}
