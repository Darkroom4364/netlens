//go:build tui

package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gonum.org/v1/gonum/mat"

	"github.com/Darkroom4364/netlens/tomo"
)

// stubTopo is a minimal Topology implementation for testing.
type stubTopo struct {
	nodes []tomo.Node
	links []tomo.Link
}

func (s *stubTopo) NumNodes() int                          { return len(s.nodes) }
func (s *stubTopo) NumLinks() int                          { return len(s.links) }
func (s *stubTopo) Links() []tomo.Link                     { return s.links }
func (s *stubTopo) Nodes() []tomo.Node                     { return s.nodes }
func (s *stubTopo) Neighbors(int) []int                    { return nil }
func (s *stubTopo) ShortestPath(int, int) ([]int, bool)    { return nil, false }
func (s *stubTopo) AllPairsShortestPaths() []tomo.PathSpec { return nil }

// testFixture builds a 3-node, 3-link problem+solution for testing.
// Delays: link0=3ms (green), link1=15ms (yellow), link2=25ms (red/congested).
func testFixture(t *testing.T) (*tomo.Problem, *tomo.Solution) {
	t.Helper()
	topo := &stubTopo{
		nodes: []tomo.Node{
			{ID: 0, Label: "A"},
			{ID: 1, Label: "B"},
			{ID: 2, Label: "C"},
		},
		links: []tomo.Link{
			{ID: 0, Src: 0, Dst: 1},
			{ID: 1, Src: 1, Dst: 2},
			{ID: 2, Src: 0, Dst: 2},
		},
	}
	p := &tomo.Problem{
		Topo:  topo,
		A:     mat.NewDense(3, 3, nil),
		B:     mat.NewVecDense(3, nil),
		Links: topo.links,
		Quality: &tomo.MatrixQuality{
			Rank:             3,
			NumLinks:         3,
			NumPaths:         3,
			IdentifiableFrac: 1.0,
			CoveragePerLink:  []int{5, 3, 8},
		},
	}
	s := &tomo.Solution{
		X:          mat.NewVecDense(3, []float64{3.0, 15.0, 25.0}),
		Confidence: mat.NewVecDense(3, []float64{0.5, 1.0, 2.0}),
		Method:     "test",
		Duration:   time.Millisecond,
	}
	return p, s
}

func newTestModel(t *testing.T) Model {
	t.Helper()
	p, s := testFixture(t)
	m := New(p, s, nil, 0)
	m.width = 120
	m.height = 40
	return m
}

func sendKey(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model)
}

func sendSpecialKey(m Model, k tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: k})
	return updated.(Model)
}

// --- TreeRowCount ---

func TestTreeRowCount_NilProblem(t *testing.T) {
	if n := TreeRowCount(nil, nil, nil, "", 0); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestTreeRowCount_AllCollapsed(t *testing.T) {
	p, s := testFixture(t)
	// 3 links: src 0 has 2 links (0→1, 0→2), src 1 has 1 link (1→2) = 2 groups
	// Row count = 1 (summary) + 2 (node headers) = 3
	n := TreeRowCount(p, s, map[int]bool{}, "", SortDefault)
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestTreeRowCount_OneExpanded(t *testing.T) {
	p, s := testFixture(t)
	// Expand node 0 which has 2 links
	expanded := map[int]bool{0: true}
	n := TreeRowCount(p, s, expanded, "", SortDefault)
	// 1 (summary) + 2 (headers) + 2 (links under node 0) = 5
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
}

// --- CursorToNodeID ---

func TestCursorToNodeID_NilProblem(t *testing.T) {
	if id := CursorToNodeID(nil, nil, 0, nil, "", 0); id != -1 {
		t.Errorf("expected -1, got %d", id)
	}
}

func TestCursorToNodeID_SummaryRow(t *testing.T) {
	p, s := testFixture(t)
	if id := CursorToNodeID(p, s, 0, map[int]bool{}, "", 0); id != -1 {
		t.Errorf("expected -1 for summary row, got %d", id)
	}
}

func TestCursorToNodeID_FirstHeader(t *testing.T) {
	p, s := testFixture(t)
	id := CursorToNodeID(p, s, 1, map[int]bool{}, "", SortDefault)
	if id == -1 {
		t.Error("expected a valid node ID at cursor 1, got -1")
	}
}

func TestCursorToNodeID_OnLinkRow(t *testing.T) {
	p, s := testFixture(t)
	expanded := map[int]bool{0: true}
	// cursor 1 = node 0 header, cursor 2 = first link under node 0
	id := CursorToNodeID(p, s, 2, expanded, "", SortDefault)
	if id != -1 {
		t.Errorf("expected -1 for link row, got %d", id)
	}
}

// --- CursorToLinkIdx ---

func TestCursorToLinkIdx_SummaryRow(t *testing.T) {
	p, s := testFixture(t)
	if idx := CursorToLinkIdx(p, s, 0, map[int]bool{}, "", 0); idx != -1 {
		t.Errorf("expected -1 for summary row, got %d", idx)
	}
}

func TestCursorToLinkIdx_NodeHeader(t *testing.T) {
	p, s := testFixture(t)
	if idx := CursorToLinkIdx(p, s, 1, map[int]bool{}, "", 0); idx != -1 {
		t.Errorf("expected -1 for node header, got %d", idx)
	}
}

func TestCursorToLinkIdx_OnLink(t *testing.T) {
	p, s := testFixture(t)
	expanded := map[int]bool{0: true}
	// cursor 2 = first link under node 0
	idx := CursorToLinkIdx(p, s, 2, expanded, "", SortDefault)
	if idx < 0 {
		t.Errorf("expected valid link index at cursor 2, got %d", idx)
	}
}

// --- RenderTreeView ---

func TestRenderTreeView_NilInputs(t *testing.T) {
	out := RenderTreeView(nil, nil, 0, nil, "", 0, 80, 24)
	if out != "no data" {
		t.Errorf("expected 'no data', got %q", out)
	}
}

func TestRenderTreeView_ContainsSummary(t *testing.T) {
	p, s := testFixture(t)
	out := RenderTreeView(p, s, 0, map[int]bool{}, "", 0, 120, 40)
	if !strings.Contains(out, "3 links") {
		t.Error("expected summary to contain '3 links'")
	}
	if !strings.Contains(out, "congested") {
		t.Error("expected summary to contain 'congested'")
	}
}

func TestRenderTreeView_ExpandedShowsChildren(t *testing.T) {
	p, s := testFixture(t)
	expanded := map[int]bool{0: true}
	out := RenderTreeView(p, s, 0, expanded, "", 0, 120, 40)
	// Node 0 (A) links to B (dst 1) and C (dst 2)
	if !strings.Contains(out, "B") {
		t.Error("expected expanded node to show child label 'B'")
	}
}

func TestRenderTreeView_CollapsedHidesChildren(t *testing.T) {
	p, s := testFixture(t)
	out := RenderTreeView(p, s, 0, map[int]bool{}, "", 0, 120, 40)
	// When collapsed, should not show delay bars (█)
	if strings.Contains(out, "█") {
		t.Error("expected collapsed tree to hide delay bars")
	}
}

func TestRenderTreeView_FilterReducesOutput(t *testing.T) {
	p, s := testFixture(t)
	full := RenderTreeView(p, s, 0, map[int]bool{}, "", 0, 120, 40)
	filtered := RenderTreeView(p, s, 0, map[int]bool{}, "A", 0, 120, 40)
	if len(filtered) >= len(full) {
		t.Error("expected filtered output to be shorter")
	}
}

// --- RenderHeatmapView ---

func TestRenderHeatmapView_ContainsLegend(t *testing.T) {
	p, s := testFixture(t)
	out := RenderHeatmapView(p, s, -1, "", 120, 40)
	if !strings.Contains(out, "<2ms") {
		t.Error("expected legend to contain '<2ms'")
	}
	if !strings.Contains(out, ">10ms") {
		t.Error("expected legend to contain '>10ms'")
	}
}

func TestRenderHeatmapView_ContainsDot(t *testing.T) {
	p, s := testFixture(t)
	out := RenderHeatmapView(p, s, -1, "", 120, 40)
	if !strings.Contains(out, "·") {
		t.Error("expected heatmap to contain '·' for missing links")
	}
}

// --- RenderDetailBar ---

func TestRenderDetailBar_InvalidIndex(t *testing.T) {
	p, s := testFixture(t)
	out := RenderDetailBar(p, s, -1, 120)
	if !strings.Contains(out, "No link selected") {
		t.Errorf("expected 'No link selected', got %q", out)
	}
}

func TestRenderDetailBar_ValidLink(t *testing.T) {
	p, s := testFixture(t)
	out := RenderDetailBar(p, s, 0, 120)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Error("expected detail bar to contain link labels A and B")
	}
}

func TestRenderDetailBar_CongestedLink(t *testing.T) {
	p, s := testFixture(t)
	// Link 2 has delay 25ms > 20ms threshold
	out := RenderDetailBar(p, s, 2, 120)
	if !strings.Contains(out, "CONGESTED") {
		t.Error("expected CONGESTED alert for link with delay > 20ms")
	}
}

// --- RenderStatusBar ---

func TestRenderStatusBar_TreeMode(t *testing.T) {
	p, s := testFixture(t)
	out := RenderStatusBar(p, s, viewTree, "tikhonov", false, "", 0, "", 120)
	if !strings.Contains(out, "[h]heatmap") {
		t.Error("expected '[h]heatmap' hint in tree mode")
	}
}

func TestRenderStatusBar_HeatmapMode(t *testing.T) {
	p, s := testFixture(t)
	out := RenderStatusBar(p, s, viewHeatmap, "tikhonov", false, "", 0, "", 120)
	if !strings.Contains(out, "[t]tree") {
		t.Error("expected '[t]tree' hint in heatmap mode")
	}
}

func TestRenderStatusBar_FilterActive(t *testing.T) {
	p, s := testFixture(t)
	out := RenderStatusBar(p, s, viewTree, "tikhonov", true, "test", 0, "", 120)
	if !strings.Contains(out, "FILTER:") {
		t.Error("expected 'FILTER:' when filtering is active")
	}
	if !strings.Contains(out, "test") {
		t.Error("expected filter text in status bar")
	}
}

func TestRenderStatusBar_SortModes(t *testing.T) {
	p, s := testFixture(t)
	labels := []string{"default", "delay↓", "delay↑", "name", "coverage"}
	for i, label := range labels {
		out := RenderStatusBar(p, s, viewTree, "test", false, "", i, "", 120)
		if !strings.Contains(out, label) {
			t.Errorf("sort mode %d: expected %q in output", i, label)
		}
	}
}

// --- RenderHelpOverlay ---

func TestRenderHelpOverlay_NonEmpty(t *testing.T) {
	out := RenderHelpOverlay(80, 24)
	if len(out) == 0 {
		t.Error("expected non-empty help overlay")
	}
}

func TestRenderHelpOverlay_ContainsKeys(t *testing.T) {
	out := RenderHelpOverlay(80, 24)
	for _, key := range []string{"j", "k", "Enter", "q"} {
		if !strings.Contains(out, key) {
			t.Errorf("expected help to contain %q", key)
		}
	}
}

// --- Model.Update key handling ---

func TestModelUpdate_Quit(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
	// Execute the cmd; tea.Quit returns tea.QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestModelUpdate_CursorDown(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}
}

func TestModelUpdate_CursorUp(t *testing.T) {
	m := newTestModel(t)
	m.cursor = 2
	m = sendKey(m, "k")
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", m.cursor)
	}
}

func TestModelUpdate_CursorDoesNotGoBelowZero(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}
}

func TestModelUpdate_CursorBoundsCheck(t *testing.T) {
	m := newTestModel(t)
	max := TreeRowCount(m.problem, m.solution, m.expanded, m.filterText, m.sortMode) - 1
	m.cursor = max
	m = sendKey(m, "j")
	if m.cursor != max {
		t.Errorf("expected cursor=%d at max, got %d", max, m.cursor)
	}
}

func TestModelUpdate_EnterExpandsNode(t *testing.T) {
	m := newTestModel(t)
	m.cursor = 1 // first node header
	nid := CursorToNodeID(m.problem, m.solution, m.cursor, m.expanded, m.filterText, m.sortMode)
	if nid == -1 {
		t.Fatal("cursor 1 should be a node header")
	}
	m = sendKey(m, "enter")
	if !m.expanded[nid] {
		t.Errorf("expected node %d to be expanded", nid)
	}
	// Toggle again
	m = sendKey(m, "enter")
	if m.expanded[nid] {
		t.Errorf("expected node %d to be collapsed", nid)
	}
}

func TestModelUpdate_HeatmapMode(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "h")
	if m.mode != viewHeatmap {
		t.Error("expected viewHeatmap after 'h'")
	}
	m = sendKey(m, "t")
	if m.mode != viewTree {
		t.Error("expected viewTree after 't'")
	}
}

func TestModelUpdate_FilterMode(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "/")
	if !m.filtering {
		t.Error("expected filtering=true after '/'")
	}
}

func TestModelUpdate_FilterTextInput(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "/")
	m = sendKey(m, "a")
	m = sendKey(m, "b")
	if m.filterText != "ab" {
		t.Errorf("expected filterText='ab', got %q", m.filterText)
	}
}

func TestModelUpdate_FilterBackspace(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "/")
	m = sendKey(m, "a")
	m = sendKey(m, "b")
	m = sendSpecialKey(m, tea.KeyBackspace)
	if m.filterText != "a" {
		t.Errorf("expected filterText='a', got %q", m.filterText)
	}
}

func TestModelUpdate_FilterEnterApplies(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "/")
	m = sendKey(m, "A")
	m = sendKey(m, "enter")
	if m.filtering {
		t.Error("expected filtering=false after enter")
	}
	if m.filterText != "A" {
		t.Errorf("expected filterText='A' preserved, got %q", m.filterText)
	}
}

func TestModelUpdate_FilterEscCancels(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "/")
	m = sendKey(m, "A")
	m = sendKey(m, "esc")
	if m.filtering {
		t.Error("expected filtering=false after esc")
	}
	if m.filterText != "" {
		t.Errorf("expected filterText cleared, got %q", m.filterText)
	}
}

func TestModelUpdate_SortCycle(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "s")
	if m.sortMode != 1 {
		t.Errorf("expected sortMode=1, got %d", m.sortMode)
	}
	for i := 0; i < 4; i++ {
		m = sendKey(m, "s")
	}
	if m.sortMode != 0 {
		t.Errorf("expected sortMode to wrap to 0, got %d", m.sortMode)
	}
}

func TestModelUpdate_HelpToggle(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, "?")
	if !m.showHelp {
		t.Error("expected showHelp=true after '?'")
	}
	m = sendKey(m, "?")
	if m.showHelp {
		t.Error("expected showHelp=false after second '?'")
	}
}

func TestModelUpdate_WindowSize(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = updated.(Model)
	if m.width != 200 || m.height != 50 {
		t.Errorf("expected 200x50, got %dx%d", m.width, m.height)
	}
}

// --- Model.View ---

func TestModelView_ZeroWidth(t *testing.T) {
	p, s := testFixture(t)
	m := New(p, s, nil, 0)
	// width is 0 by default
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("expected 'loading...' with zero width, got %q", out)
	}
}

func TestModelView_ShowHelp(t *testing.T) {
	m := newTestModel(t)
	m.showHelp = true
	out := m.View()
	if !strings.Contains(out, "q") {
		t.Error("expected help view to contain keybinding 'q'")
	}
}

func TestModelView_NormalRender(t *testing.T) {
	m := newTestModel(t)
	out := m.View()
	if len(out) == 0 {
		t.Error("expected non-empty view output")
	}
	if !strings.Contains(out, "3 links") {
		t.Error("expected view to contain tree summary")
	}
}

// --- NewWizard ---

func TestNewWizard(t *testing.T) {
	m := NewWizard()
	if m.phase != phaseSourceSelect {
		t.Errorf("expected phaseSourceSelect, got %d", m.phase)
	}
	if m.expanded == nil {
		t.Error("expected non-nil expanded map")
	}
}

// --- NewWithRefresh ---

func TestNewWithRefresh(t *testing.T) {
	p, s := testFixture(t)
	m := NewWithRefresh(p, s, nil, 0, time.Second)
	if m.refreshRate != time.Second {
		t.Errorf("expected 1s refresh rate, got %v", m.refreshRate)
	}
}

// --- Init ---

func TestModelInit_NoRefresh(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	if cmd != nil {
		t.Error("expected nil cmd without refresh rate")
	}
}

func TestModelInit_WithRefresh(t *testing.T) {
	p, s := testFixture(t)
	solvers := []tomo.Solver{&tomo.TikhonovSolver{}}
	m := NewWithRefresh(p, s, solvers, 0, 100*time.Millisecond)
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd with refresh rate and solvers")
	}
}

// --- currentSolver ---

func TestCurrentSolver_Empty(t *testing.T) {
	m := newTestModel(t)
	if m.currentSolver() != nil {
		t.Error("expected nil solver with no solvers")
	}
}

func TestCurrentSolver_Valid(t *testing.T) {
	p, s := testFixture(t)
	solvers := []tomo.Solver{&tomo.TikhonovSolver{}, &tomo.NNLSSolver{}}
	m := New(p, s, solvers, 1)
	solver := m.currentSolver()
	if solver == nil {
		t.Fatal("expected non-nil solver")
	}
	if solver.Name() != "nnls" {
		t.Errorf("expected nnls solver, got %s", solver.Name())
	}
}

// --- nodeLabel ---

func TestNodeLabel_WithLabel(t *testing.T) {
	p, _ := testFixture(t)
	label := nodeLabel(p, 0)
	if label != "A" {
		t.Errorf("expected 'A', got %q", label)
	}
}

func TestNodeLabel_NoMatch(t *testing.T) {
	p, _ := testFixture(t)
	label := nodeLabel(p, 999)
	if label != "node 999" {
		t.Errorf("expected 'node 999', got %q", label)
	}
}

// --- buildGroups ---

func TestBuildGroups(t *testing.T) {
	p, _ := testFixture(t)
	groups, order := buildGroups(p)
	if len(order) != 2 {
		t.Errorf("expected 2 source nodes, got %d", len(order))
	}
	// Node 0 has links 0→1 and 0→2 = 2 links
	if g, ok := groups[0]; !ok || len(g.links) != 2 {
		t.Errorf("expected node 0 to have 2 links")
	}
	// Node 1 has link 1→2 = 1 link
	if g, ok := groups[1]; !ok || len(g.links) != 1 {
		t.Errorf("expected node 1 to have 1 link")
	}
}

// --- maxGroupDelay / sumGroupCoverage ---

func TestMaxGroupDelay(t *testing.T) {
	_, s := testFixture(t)
	g := &nodeGroup{nodeID: 0, links: []int{0, 2}}
	// link 0 = 3ms, link 2 = 25ms
	mx := maxGroupDelay(g, s)
	if mx != 25.0 {
		t.Errorf("expected 25.0, got %f", mx)
	}
}

func TestSumGroupCoverage(t *testing.T) {
	p, _ := testFixture(t)
	g := &nodeGroup{nodeID: 0, links: []int{0, 2}}
	// coverage: link 0 = 5, link 2 = 8
	sum := sumGroupCoverage(g, p)
	if sum != 13 {
		t.Errorf("expected 13, got %d", sum)
	}
}

func TestSumGroupCoverage_NilQuality(t *testing.T) {
	p, _ := testFixture(t)
	p.Quality = nil
	g := &nodeGroup{nodeID: 0, links: []int{0}}
	if sum := sumGroupCoverage(g, p); sum != 0 {
		t.Errorf("expected 0 with nil quality, got %d", sum)
	}
}

// --- computeOrder ---

func TestComputeOrder_SortNameAlpha(t *testing.T) {
	p, s := testFixture(t)
	_, order := computeOrder(p, s, "", SortNameAlpha)
	if len(order) < 2 {
		t.Fatal("expected at least 2 groups")
	}
	// Node 0 is "A", node 1 is "B" — A should come first
	if nodeLabel(p, order[0]) > nodeLabel(p, order[1]) {
		t.Error("expected alphabetical order")
	}
}

func TestComputeOrder_Filter(t *testing.T) {
	p, s := testFixture(t)
	_, order := computeOrder(p, s, "B", SortDefault)
	if len(order) != 1 {
		t.Errorf("expected 1 group matching 'B', got %d", len(order))
	}
}

func TestComputeOrder_SortDelayDesc(t *testing.T) {
	p, s := testFixture(t)
	_, order := computeOrder(p, s, "", SortDelayDesc)
	if len(order) < 2 {
		t.Fatal("expected at least 2 groups")
	}
}

func TestComputeOrder_SortCoverageDesc(t *testing.T) {
	p, s := testFixture(t)
	_, order := computeOrder(p, s, "", SortCoverageDesc)
	if len(order) < 2 {
		t.Fatal("expected at least 2 groups")
	}
}

// --- renderTabBar ---

func TestRenderTabBar(t *testing.T) {
	out := renderTabBar(viewTree, 80)
	if !strings.Contains(out, "Tree") {
		t.Error("expected tab bar to contain 'Tree'")
	}
	if !strings.Contains(out, "Heatmap") {
		t.Error("expected tab bar to contain 'Heatmap'")
	}
}

// --- allSolvers ---

func TestAllSolvers(t *testing.T) {
	solvers := allSolvers()
	if len(solvers) == 0 {
		t.Fatal("expected at least one solver from allSolvers()")
	}
	for _, s := range solvers {
		if s.Name() == "" {
			t.Error("solver has empty name")
		}
	}
}

// --- Wizard phase tests ---

func TestWizardSourceSelect_Navigate(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m = sendKey(m, "j")
	if m.wizCursor != 1 {
		t.Errorf("expected wizCursor=1 after j, got %d", m.wizCursor)
	}
	m = sendKey(m, "k")
	if m.wizCursor != 0 {
		t.Errorf("expected wizCursor=0 after k, got %d", m.wizCursor)
	}
}

func TestWizardSourceSelect_EnterSimulate(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	// Select "Simulate" (index 0)
	m = sendKey(m, "enter")
	if m.phase != phaseSourceConfig {
		t.Errorf("expected phaseSourceConfig, got %d", m.phase)
	}
	if m.source != sourceSimulate {
		t.Errorf("expected sourceSimulate, got %d", m.source)
	}
}

func TestWizardRenderSourceSelect(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	out := m.View()
	if !strings.Contains(out, "Simulate") {
		t.Error("expected source select to contain 'Simulate'")
	}
	if !strings.Contains(out, "RIPE Atlas") {
		t.Error("expected source select to contain 'RIPE Atlas'")
	}
}

func TestWizardRenderSolverSelect(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSolverSelect
	out := m.View()
	if !strings.Contains(out, "tikhonov") {
		t.Error("expected solver select to contain 'tikhonov'")
	}
	if !strings.Contains(out, "nnls") {
		t.Error("expected solver select to contain 'nnls'")
	}
}

func TestWizardRenderLoading(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	m.loadingMsg = "Loading test..."
	out := m.View()
	if !strings.Contains(out, "Loading test") {
		t.Error("expected loading screen to contain message")
	}
}

func TestWizardRenderLoading_Error(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	m.loadErr = "something broke"
	out := m.View()
	if !strings.Contains(out, "something broke") {
		t.Error("expected error message in loading screen")
	}
}

// --- loadResultMsg handling ---

func TestUpdateLoading_ResultSuccess(t *testing.T) {
	p, s := testFixture(t)
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading

	result := loadResultMsg{
		problem:   p,
		solution:  s,
		solvers:   []tomo.Solver{&tomo.TikhonovSolver{}},
		solverIdx: 0,
	}
	updated, _ := m.Update(result)
	m = updated.(Model)
	if m.phase != phaseDashboard {
		t.Errorf("expected phaseDashboard after successful load, got %d", m.phase)
	}
	if m.problem != p {
		t.Error("expected problem to be set")
	}
}

func TestUpdateLoading_ResultError(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading

	result := loadResultMsg{err: fmt.Errorf("test error")}
	updated, _ := m.Update(result)
	m = updated.(Model)
	if m.loadErr != "test error" {
		t.Errorf("expected loadErr='test error', got %q", m.loadErr)
	}
}

func TestUpdateLoading_ContextCanceled(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading

	result := loadResultMsg{err: context.Canceled}
	updated, _ := m.Update(result)
	m = updated.(Model)
	if m.loadErr != "" {
		t.Errorf("expected no loadErr for context.Canceled, got %q", m.loadErr)
	}
}

func TestUpdateLoading_SpinnerTick(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	m.spinnerFrame = 0

	updated, cmd := m.Update(spinnerTickMsg{})
	m = updated.(Model)
	if m.spinnerFrame != 1 {
		t.Errorf("expected spinnerFrame=1, got %d", m.spinnerFrame)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for next spinner tick")
	}
}

func TestUpdateLoading_SpinnerStopsOnError(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	m.loadErr = "some error"

	updated, cmd := m.Update(spinnerTickMsg{})
	m = updated.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when loadErr is set")
	}
}

func TestUpdateLoading_LogLine(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	ch := make(chan string, 1)
	m.progCh = ch

	updated, cmd := m.Update(logLineMsg("step 1"))
	m = updated.(Model)
	if len(m.loadLogs) != 1 || m.loadLogs[0] != "step 1" {
		t.Errorf("expected loadLogs=['step 1'], got %v", m.loadLogs)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to continue listening")
	}
}

func TestUpdateLoading_EscOnError(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	m.loadErr = "some error"

	m = sendKey(m, "esc")
	if m.phase != phaseSolverSelect {
		t.Errorf("expected phaseSolverSelect after esc on error, got %d", m.phase)
	}
	if m.loadErr != "" {
		t.Errorf("expected loadErr cleared, got %q", m.loadErr)
	}
}

func TestUpdateLoading_EscAbort(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseLoading
	canceled := false
	m.cancelLoad = func() { canceled = true }

	m = sendKey(m, "esc")
	if !canceled {
		t.Error("expected cancelLoad to be called")
	}
	if m.phase != phaseSolverSelect {
		t.Errorf("expected phaseSolverSelect after abort, got %d", m.phase)
	}
}

// --- updateSourceConfig ---

func TestWizardSourceConfig_SimulateNavigate(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceSimulate

	m = sendKey(m, "j")
	if m.wizCursor != 1 {
		t.Errorf("expected wizCursor=1, got %d", m.wizCursor)
	}
	m = sendKey(m, "k")
	if m.wizCursor != 0 {
		t.Errorf("expected wizCursor=0, got %d", m.wizCursor)
	}
}

func TestWizardSourceConfig_SimulateSelectTopo(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceSimulate

	m = sendKey(m, "enter") // select first bundled topology
	if m.phase != phaseSolverSelect {
		t.Errorf("expected phaseSolverSelect, got %d", m.phase)
	}
	if m.topoChoice != 0 {
		t.Errorf("expected topoChoice=0, got %d", m.topoChoice)
	}
}

func TestWizardSourceConfig_SimulateEsc(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceSimulate

	m = sendKey(m, "esc")
	if m.phase != phaseSourceSelect {
		t.Errorf("expected phaseSourceSelect after esc, got %d", m.phase)
	}
}

func TestWizardSourceConfig_SimulateCustomPath(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceSimulate
	// Navigate to "Custom path..." (last item)
	m.wizCursor = len(bundledTopologies)

	m = sendKey(m, "enter")
	if m.source != sourceSimulateCustom {
		t.Errorf("expected sourceSimulateCustom, got %d", m.source)
	}
	if !m.inputActive {
		t.Error("expected inputActive=true for custom path")
	}
}

func TestWizardSourceConfig_RIPEAtlasInput(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceRIPEAtlas
	m.inputActive = true

	// Type MSM ID
	m = sendKey(m, "5")
	m = sendKey(m, "0")
	m = sendKey(m, "0")
	m = sendKey(m, "1")
	if m.msmIDInput != "5001" {
		t.Errorf("expected msmIDInput='5001', got %q", m.msmIDInput)
	}

	// Backspace
	m = sendSpecialKey(m, tea.KeyBackspace)
	if m.msmIDInput != "500" {
		t.Errorf("expected msmIDInput='500' after backspace, got %q", m.msmIDInput)
	}

	// Add back and enter
	m = sendKey(m, "1")
	m = sendKey(m, "enter")
	if m.phase != phaseSolverSelect {
		t.Errorf("expected phaseSolverSelect after valid MSM ID, got %d", m.phase)
	}
}

func TestWizardSourceConfig_RIPEAtlasInvalidID(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceRIPEAtlas
	m.inputActive = true

	m = sendKey(m, "a")
	m = sendKey(m, "b")
	m = sendKey(m, "enter")
	if m.loadErr == "" {
		t.Error("expected loadErr for non-numeric MSM ID")
	}
	if m.phase != phaseSourceConfig {
		t.Errorf("expected to stay on phaseSourceConfig, got %d", m.phase)
	}
}

func TestWizardSourceConfig_RIPEAtlasEsc(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceRIPEAtlas
	m.inputActive = true
	m.msmIDInput = "123"

	m = sendKey(m, "esc")
	if m.phase != phaseSourceSelect {
		t.Errorf("expected phaseSourceSelect after esc, got %d", m.phase)
	}
	if m.msmIDInput != "" {
		t.Errorf("expected msmIDInput cleared, got %q", m.msmIDInput)
	}
}

func TestWizardSourceConfig_TracerouteEmptyPath(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceTraceroute
	m.inputActive = true

	m = sendKey(m, "enter")
	if m.loadErr == "" {
		t.Error("expected loadErr for empty file path")
	}
}

func TestWizardSourceConfig_TracerouteInput(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceTraceroute
	m.inputActive = true

	m = sendKey(m, "a")
	if m.filePathInput != "a" {
		t.Errorf("expected filePathInput='a', got %q", m.filePathInput)
	}
	m = sendSpecialKey(m, tea.KeyBackspace)
	if m.filePathInput != "" {
		t.Errorf("expected filePathInput empty, got %q", m.filePathInput)
	}
}

// --- updateSolverSelect ---

func TestWizardSolverSelect_Navigate(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSolverSelect

	m = sendKey(m, "j")
	if m.wizCursor != 1 {
		t.Errorf("expected wizCursor=1, got %d", m.wizCursor)
	}
	m = sendKey(m, "k")
	if m.wizCursor != 0 {
		t.Errorf("expected wizCursor=0, got %d", m.wizCursor)
	}
}

func TestWizardSolverSelect_Esc(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSolverSelect
	m.source = sourceSimulate

	m = sendKey(m, "esc")
	if m.phase != phaseSourceConfig {
		t.Errorf("expected phaseSourceConfig after esc, got %d", m.phase)
	}
}

func TestWizardSolverSelect_EscRIPE(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSolverSelect
	m.source = sourceRIPEAtlas

	m = sendKey(m, "esc")
	if !m.inputActive {
		t.Error("expected inputActive=true when going back to RIPE config")
	}
}

// --- renderSourceConfig ---

func TestWizardRenderSourceConfig_Simulate(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceSimulate
	out := m.View()
	if !strings.Contains(out, "topology") {
		t.Error("expected source config to mention topology")
	}
	if !strings.Contains(out, "abilene") {
		t.Error("expected bundled topology 'abilene'")
	}
}

func TestWizardRenderSourceConfig_RIPEAtlas(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceRIPEAtlas
	m.inputActive = true
	out := m.View()
	if !strings.Contains(out, "Measurement ID") {
		t.Error("expected RIPE config to contain 'Measurement ID'")
	}
}

func TestWizardRenderSourceConfig_Traceroute(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceTraceroute
	m.inputActive = true
	out := m.View()
	if !strings.Contains(out, "File path") {
		t.Error("expected traceroute config to contain 'File path'")
	}
}

func TestWizardRenderSourceConfig_WithError(t *testing.T) {
	m := NewWizard()
	m.width = 120
	m.height = 40
	m.phase = phaseSourceConfig
	m.source = sourceRIPEAtlas
	m.loadErr = "invalid input"
	out := m.View()
	if !strings.Contains(out, "invalid input") {
		t.Error("expected error to appear in source config")
	}
}

// --- Dashboard solver switch ---

func TestModelUpdate_SolverSwitch(t *testing.T) {
	p, s := testFixture(t)
	solvers := []tomo.Solver{&tomo.TikhonovSolver{}, &tomo.NNLSSolver{}}
	m := New(p, s, solvers, 0)
	m.width = 120
	m.height = 40

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(Model)
	if m.solverIdx != 1 {
		t.Errorf("expected solverIdx=1, got %d", m.solverIdx)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for async re-solve")
	}
}

func TestModelUpdate_SolveResult(t *testing.T) {
	m := newTestModel(t)
	_, newSol := testFixture(t)
	updated, _ := m.Update(solveResultMsg{sol: newSol})
	m = updated.(Model)
	if m.solution != newSol {
		t.Error("expected solution to be updated")
	}
}

func TestModelUpdate_SolveResultError(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(solveResultMsg{err: fmt.Errorf("solve failed")})
	m = updated.(Model)
	if m.solveErr != "solve failed" {
		t.Errorf("expected solveErr='solve failed', got %q", m.solveErr)
	}
}

func TestModelUpdate_TabSwitch(t *testing.T) {
	m := newTestModel(t)
	m = sendSpecialKey(m, tea.KeyTab)
	if m.mode != viewHeatmap {
		t.Error("expected viewHeatmap after tab")
	}
	m = sendSpecialKey(m, tea.KeyTab)
	if m.mode != viewTree {
		t.Error("expected viewTree after second tab")
	}
}

func TestModelUpdate_EscClosesHelp(t *testing.T) {
	m := newTestModel(t)
	m.showHelp = true
	m = sendKey(m, "esc")
	if m.showHelp {
		t.Error("expected showHelp=false after esc")
	}
}

func TestModelView_HeatmapMode(t *testing.T) {
	m := newTestModel(t)
	m.mode = viewHeatmap
	out := m.View()
	if len(out) == 0 {
		t.Error("expected non-empty heatmap view")
	}
}

func TestRenderTabBar_HeatmapActive(t *testing.T) {
	out := renderTabBar(viewHeatmap, 80)
	if !strings.Contains(out, "Heatmap") {
		t.Error("expected tab bar to contain 'Heatmap'")
	}
}
