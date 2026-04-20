package tomo

import (
	"context"
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MinRTT edge cases
// ---------------------------------------------------------------------------

func TestWeight_MinRTT_Empty(t *testing.T) {
	m := PathMeasurement{RTTs: nil}
	if got := m.MinRTT(); got != 0 {
		t.Errorf("MinRTT(nil) = %v, want 0", got)
	}
	m2 := PathMeasurement{RTTs: []time.Duration{}}
	if got := m2.MinRTT(); got != 0 {
		t.Errorf("MinRTT([]) = %v, want 0", got)
	}
}

func TestWeight_MinRTT_Single(t *testing.T) {
	m := PathMeasurement{RTTs: []time.Duration{42 * time.Millisecond}}
	if got := m.MinRTT(); got != 42*time.Millisecond {
		t.Errorf("MinRTT = %v, want 42ms", got)
	}
}

func TestWeight_MinRTT_AllEqual(t *testing.T) {
	m := PathMeasurement{RTTs: []time.Duration{
		7 * time.Millisecond,
		7 * time.Millisecond,
		7 * time.Millisecond,
	}}
	if got := m.MinRTT(); got != 7*time.Millisecond {
		t.Errorf("MinRTT = %v, want 7ms", got)
	}
}

func TestWeight_MinRTT_Negative(t *testing.T) {
	// Negative RTT: MinRTT returns it (no validation).
	m := PathMeasurement{RTTs: []time.Duration{
		10 * time.Millisecond,
		-5 * time.Millisecond,
		20 * time.Millisecond,
	}}
	if got := m.MinRTT(); got != -5*time.Millisecond {
		t.Errorf("MinRTT = %v, want -5ms (negative accepted)", got)
	}
}

func TestWeight_MinRTT_Zero(t *testing.T) {
	m := PathMeasurement{RTTs: []time.Duration{
		10 * time.Millisecond,
		0,
		20 * time.Millisecond,
	}}
	if got := m.MinRTT(); got != 0 {
		t.Errorf("MinRTT = %v, want 0", got)
	}
}

func TestWeight_MinRTT_VeryLarge(t *testing.T) {
	m := PathMeasurement{RTTs: []time.Duration{
		time.Hour,
		10 * time.Millisecond,
	}}
	if got := m.MinRTT(); got != 10*time.Millisecond {
		t.Errorf("MinRTT = %v, want 10ms", got)
	}
	// Also check that time.Hour alone returns correctly.
	m2 := PathMeasurement{RTTs: []time.Duration{time.Hour}}
	if got := m2.MinRTT(); got != time.Hour {
		t.Errorf("MinRTT = %v, want 1h", got)
	}
}

// ---------------------------------------------------------------------------
// BuildProblemFromMeasurements edge cases
// ---------------------------------------------------------------------------

func TestWeight_BuildProblem_EmptyRTTs(t *testing.T) {
	// Measurements with no RTTs → MinRTT returns 0 → measurement value is 0ms.
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{
		{Src: 0, Dst: 1, LinkIDs: []int{0}},
	}
	measurements := []PathMeasurement{
		{Src: "a", Dst: "b", RTTs: nil, Weight: 1.0},
	}
	p, err := BuildProblemFromMeasurements(topo, measurements, paths)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := p.B.AtVec(0); got != 0.0 {
		t.Errorf("B[0] = %v, want 0.0 (empty RTTs)", got)
	}
}

func TestWeight_BuildProblem_AllZeroMinRTT(t *testing.T) {
	// All measurements have MinRTT=0 → all b values are 0.
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{
		{Src: 0, Dst: 1, LinkIDs: []int{0}},
		{Src: 1, Dst: 2, LinkIDs: []int{1}},
	}
	measurements := []PathMeasurement{
		{Src: "a", Dst: "b", RTTs: []time.Duration{0}, Weight: 1.0},
		{Src: "b", Dst: "c", RTTs: []time.Duration{0}, Weight: 1.0},
	}
	p, err := BuildProblemFromMeasurements(topo, measurements, paths)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for i := 0; i < p.B.Len(); i++ {
		if got := p.B.AtVec(i); got != 0.0 {
			t.Errorf("B[%d] = %v, want 0.0", i, got)
		}
	}

	// Solver should still work.
	solver := &TikhonovSolver{Lambda: 0.01}
	sol, err := solver.Solve(context.Background(), p)
	if err != nil {
		t.Logf("solver error (acceptable): %v", err)
		return
	}
	if math.IsNaN(sol.Residual) {
		t.Error("residual is NaN for all-zero b")
	}
}

func TestWeight_BuildProblem_MismatchedCounts(t *testing.T) {
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{
		{Src: 0, Dst: 1, LinkIDs: []int{0}},
		{Src: 1, Dst: 2, LinkIDs: []int{1}},
	}
	measurements := []PathMeasurement{
		{Src: "a", Dst: "b", RTTs: []time.Duration{10 * time.Millisecond}, Weight: 1.0},
	}
	_, err := BuildProblemFromMeasurements(topo, measurements, paths)
	if err == nil {
		t.Fatal("expected error for mismatched measurement/pathspec counts, got nil")
	}
}

// ---------------------------------------------------------------------------
// Timestamp edge cases (no functional impact — document acceptance)
// ---------------------------------------------------------------------------

func TestWeight_Timestamp_ZeroValue(t *testing.T) {
	m := PathMeasurement{
		Src:       "a",
		Dst:       "b",
		RTTs:      []time.Duration{10 * time.Millisecond},
		Timestamp: time.Time{},
		Weight:    1.0,
	}
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0}}}
	p, err := BuildProblemFromMeasurements(topo, []PathMeasurement{m}, paths)
	if err != nil {
		t.Fatalf("zero timestamp rejected: %v", err)
	}
	if p.B.AtVec(0) != 10.0 {
		t.Errorf("B[0] = %v, want 10.0", p.B.AtVec(0))
	}
}

func TestWeight_Timestamp_Future(t *testing.T) {
	m := PathMeasurement{
		Src:       "a",
		Dst:       "b",
		RTTs:      []time.Duration{5 * time.Millisecond},
		Timestamp: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		Weight:    1.0,
	}
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0}}}
	p, err := BuildProblemFromMeasurements(topo, []PathMeasurement{m}, paths)
	if err != nil {
		t.Fatalf("future timestamp rejected: %v", err)
	}
	_ = p
}

func TestWeight_Timestamp_NegativeUnix(t *testing.T) {
	// Pre-epoch: 1960-01-01 has a negative Unix timestamp.
	m := PathMeasurement{
		Src:       "a",
		Dst:       "b",
		RTTs:      []time.Duration{5 * time.Millisecond},
		Timestamp: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
		Weight:    1.0,
	}
	topo := makeStubTopo(3, 3)
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0}}}
	p, err := BuildProblemFromMeasurements(topo, []PathMeasurement{m}, paths)
	if err != nil {
		t.Fatalf("negative-unix timestamp rejected: %v", err)
	}
	if m.Timestamp.Unix() >= 0 {
		t.Errorf("expected negative unix timestamp, got %d", m.Timestamp.Unix())
	}
	_ = p
}
