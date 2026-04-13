package tomo

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

// stubTopo is a minimal Topology implementation for testing BuildProblem directly.
type stubTopo struct {
	nNodes int
	nLinks int
	links  []Link
	nodes  []Node
	paths  []PathSpec
}

func (s *stubTopo) NumNodes() int                            { return s.nNodes }
func (s *stubTopo) NumLinks() int                            { return s.nLinks }
func (s *stubTopo) Links() []Link                            { return s.links }
func (s *stubTopo) Nodes() []Node                            { return s.nodes }
func (s *stubTopo) Neighbors(int) []int                      { return nil }
func (s *stubTopo) ShortestPath(int, int) ([]int, bool)      { return nil, false }
func (s *stubTopo) AllPairsShortestPaths() []PathSpec         { return s.paths }

func makeStubTopo(nNodes, nLinks int) *stubTopo {
	links := make([]Link, nLinks)
	for i := range links {
		links[i] = Link{ID: i, Src: 0, Dst: i + 1}
	}
	nodes := make([]Node, nNodes)
	for i := range nodes {
		nodes[i] = Node{ID: i}
	}
	return &stubTopo{nNodes: nNodes, nLinks: nLinks, links: links, nodes: nodes}
}

// ---------- BuildProblem edge cases ----------

func TestEdge_BuildProblem_EmptyPaths(t *testing.T) {
	topo := makeStubTopo(3, 2)
	_, err := BuildProblem(topo, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty paths")
	}
}

func TestEdge_BuildProblem_LinkOutOfRange(t *testing.T) {
	topo := makeStubTopo(3, 2) // links 0,1
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0, 99}}}
	_, err := BuildProblem(topo, paths, []float64{1.0})
	if err == nil {
		t.Fatal("expected error for out-of-range link ID")
	}
}

func TestEdge_BuildProblem_MismatchedCounts(t *testing.T) {
	topo := makeStubTopo(3, 2)
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0}}}
	_, err := BuildProblem(topo, paths, []float64{1.0, 2.0})
	if err == nil {
		t.Fatal("expected error for mismatched path/measurement counts")
	}
}

func TestEdge_BuildProblem_ZeroLinks(t *testing.T) {
	topo := makeStubTopo(2, 0) // no links
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: nil}}
	_, err := BuildProblem(topo, paths, []float64{1.0})
	if err == nil {
		t.Fatal("expected error for zero links")
	}
}

func TestEdge_BuildProblem_PathWithNoLinks(t *testing.T) {
	topo := makeStubTopo(3, 2)
	paths := []PathSpec{{Src: 0, Dst: 1, LinkIDs: nil}} // empty LinkIDs
	p, err := BuildProblem(topo, paths, []float64{5.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Row should be all zeros
	for j := 0; j < p.NumLinks(); j++ {
		if p.A.At(0, j) != 0 {
			t.Errorf("expected zero at column %d, got %f", j, p.A.At(0, j))
		}
	}
}

func TestEdge_BuildProblem_AllMeasurementsIdentical(t *testing.T) {
	topo := makeStubTopo(4, 3)
	paths := []PathSpec{
		{Src: 0, Dst: 1, LinkIDs: []int{0, 1}},
		{Src: 0, Dst: 2, LinkIDs: []int{1, 2}},
		{Src: 1, Dst: 2, LinkIDs: []int{0, 2}},
	}
	meas := []float64{5.0, 5.0, 5.0}
	p, err := BuildProblem(topo, paths, meas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < p.NumPaths(); i++ {
		if p.B.AtVec(i) != 5.0 {
			t.Errorf("expected b[%d]=5.0, got %f", i, p.B.AtVec(i))
		}
	}
	assertQualitySane(t, p.Quality)
}

func TestEdge_BuildProblem_AllMeasurementsZero(t *testing.T) {
	topo := makeStubTopo(4, 3)
	paths := []PathSpec{
		{Src: 0, Dst: 1, LinkIDs: []int{0, 1}},
		{Src: 0, Dst: 2, LinkIDs: []int{1, 2}},
	}
	meas := []float64{0.0, 0.0}
	p, err := BuildProblem(topo, paths, meas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertQualitySane(t, p.Quality)
}

// ---------- BuildProblemFromTopology edge cases ----------

func TestEdge_BuildProblemFromTopology_GroundTruthLengthMismatch(t *testing.T) {
	topo := makeStubTopo(3, 2)
	topo.paths = []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0, 1}}}
	_, err := BuildProblemFromTopology(topo, []float64{1.0, 2.0, 3.0}) // 3 != 2
	if err == nil {
		t.Fatal("expected error for ground truth length mismatch")
	}
}

func TestEdge_BuildProblemFromTopology_GroundTruthAllZeros(t *testing.T) {
	topo := makeStubTopo(3, 2)
	topo.paths = []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0, 1}}}
	p, err := BuildProblemFromTopology(topo, []float64{0.0, 0.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All measurements should be zero
	for i := 0; i < p.NumPaths(); i++ {
		if p.B.AtVec(i) != 0.0 {
			t.Errorf("expected b[%d]=0.0, got %f", i, p.B.AtVec(i))
		}
	}
	assertQualitySane(t, p.Quality)
}

func TestEdge_BuildProblemFromTopology_GroundTruthWithNaN(t *testing.T) {
	topo := makeStubTopo(3, 2)
	topo.paths = []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0, 1}}}
	p, err := BuildProblemFromTopology(topo, []float64{math.NaN(), 1.0})
	// NaN propagates; BuildProblem itself doesn't validate values — just check no panic.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = p
}

func TestEdge_BuildProblemFromTopology_GroundTruthNegative(t *testing.T) {
	topo := makeStubTopo(3, 2)
	topo.paths = []PathSpec{{Src: 0, Dst: 1, LinkIDs: []int{0, 1}}}
	p, err := BuildProblemFromTopology(topo, []float64{-5.0, -10.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Measurement should be sum of ground truth per path
	if got := p.B.AtVec(0); got != -15.0 {
		t.Errorf("expected b[0]=-15.0, got %f", got)
	}
	assertQualitySane(t, p.Quality)
}

func TestEdge_BuildProblemFromTopology_SingleNode(t *testing.T) {
	topo := makeStubTopo(1, 0)
	topo.paths = nil // no paths from a single node
	_, err := BuildProblemFromTopology(topo, nil)
	if err == nil {
		t.Fatal("expected error for single-node topology (no paths)")
	}
}

// ---------- AnalyzeQuality edge cases ----------

func TestEdge_AnalyzeQuality_1x1(t *testing.T) {
	A := mat.NewDense(1, 1, []float64{1.0})
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank != 1 {
		t.Errorf("expected rank 1 for 1x1 identity, got %d", q.Rank)
	}
	if q.IdentifiableFrac != 1.0 {
		t.Errorf("expected IdentifiableFrac=1.0, got %f", q.IdentifiableFrac)
	}
}

func TestEdge_AnalyzeQuality_AllZeros(t *testing.T) {
	A := mat.NewDense(3, 3, make([]float64, 9))
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank != 0 {
		t.Errorf("expected rank 0 for all-zeros matrix, got %d", q.Rank)
	}
	if q.IdentifiableFrac != 0 {
		t.Errorf("expected IdentifiableFrac=0 for all-zeros, got %f", q.IdentifiableFrac)
	}
}

func TestEdge_AnalyzeQuality_Identity(t *testing.T) {
	n := 5
	data := make([]float64, n*n)
	for i := 0; i < n; i++ {
		data[i*n+i] = 1.0
	}
	A := mat.NewDense(n, n, data)
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank != n {
		t.Errorf("expected rank %d for identity, got %d", n, q.Rank)
	}
	if q.IdentifiableFrac != 1.0 {
		t.Errorf("expected IdentifiableFrac=1.0, got %f", q.IdentifiableFrac)
	}
	if q.ConditionNumber != 1.0 {
		t.Errorf("expected condition number 1.0 for identity, got %f", q.ConditionNumber)
	}
	if len(q.UnidentifiableLinks) != 0 {
		t.Errorf("expected no unidentifiable links, got %v", q.UnidentifiableLinks)
	}
}

func TestEdge_AnalyzeQuality_OneNonzeroColumn(t *testing.T) {
	// 3 paths, 3 links — only link 0 is used
	A := mat.NewDense(3, 3, []float64{
		1, 0, 0,
		1, 0, 0,
		1, 0, 0,
	})
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank != 1 {
		t.Errorf("expected rank 1, got %d", q.Rank)
	}
	// Links 1 and 2 should be unidentifiable
	if q.IdentifiableFrac >= 1.0 {
		t.Errorf("expected IdentifiableFrac < 1.0, got %f", q.IdentifiableFrac)
	}
}

func TestEdge_AnalyzeQuality_VeryLargeMatrix(t *testing.T) {
	m, n := 500, 200
	data := make([]float64, m*n)
	// Fill with a pattern: each path uses 3 consecutive links (wrapping)
	for i := 0; i < m; i++ {
		for k := 0; k < 3; k++ {
			col := (i*3 + k) % n
			data[i*n+col] = 1.0
		}
	}
	A := mat.NewDense(m, n, data)
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.NumPaths != m {
		t.Errorf("expected NumPaths=%d, got %d", m, q.NumPaths)
	}
	if q.NumLinks != n {
		t.Errorf("expected NumLinks=%d, got %d", n, q.NumLinks)
	}
}

func TestEdge_AnalyzeQuality_IdenticalColumns(t *testing.T) {
	// Two links always appear together — aliased, so rank < n.
	// The rank should be 1 since columns are linearly dependent.
	// Note: identifyNullSpaceLinks works via V-matrix row norms, which may
	// not flag both aliased columns as unidentifiable (each has nonzero
	// projection onto the single right singular vector). The key signal is
	// rank < NumLinks and condition number reflecting the degeneracy.
	A := mat.NewDense(3, 2, []float64{
		1, 1,
		1, 1,
		0, 0,
	})
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank > 1 {
		t.Errorf("expected rank <= 1 for identical columns, got %d", q.Rank)
	}
	if q.Rank >= q.NumLinks {
		t.Errorf("expected rank (%d) < NumLinks (%d) for aliased columns", q.Rank, q.NumLinks)
	}
}

func TestEdge_AnalyzeQuality_IdenticalRows(t *testing.T) {
	// Duplicate paths — should not increase rank beyond 1
	A := mat.NewDense(3, 2, []float64{
		1, 1,
		1, 1,
		1, 1,
	})
	q := AnalyzeQuality(A)
	assertQualitySane(t, q)
	if q.Rank > 1 {
		t.Errorf("expected rank <= 1 for identical rows, got %d", q.Rank)
	}
}

// ---------- helper ----------

func assertQualitySane(t *testing.T, q *MatrixQuality) {
	t.Helper()
	if q == nil {
		t.Fatal("MatrixQuality is nil")
	}
	if q.Rank < 0 {
		t.Errorf("Rank should be >= 0, got %d", q.Rank)
	}
	if q.IdentifiableFrac < 0 || q.IdentifiableFrac > 1.0 {
		t.Errorf("IdentifiableFrac should be in [0,1], got %f", q.IdentifiableFrac)
	}
	for i, c := range q.CoveragePerLink {
		if c < 0 {
			t.Errorf("CoveragePerLink[%d] should be >= 0, got %d", i, c)
		}
	}
}
