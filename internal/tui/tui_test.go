//go:build tui

package tui

import (
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
	out := RenderStatusBar(p, s, viewTree, "tikhonov", false, "", 0, 120)
	if !strings.Contains(out, "[h]heatmap") {
		t.Error("expected '[h]heatmap' hint in tree mode")
	}
}

func TestRenderStatusBar_HeatmapMode(t *testing.T) {
	p, s := testFixture(t)
	out := RenderStatusBar(p, s, viewHeatmap, "tikhonov", false, "", 0, 120)
	if !strings.Contains(out, "[t]tree") {
		t.Error("expected '[t]tree' hint in heatmap mode")
	}
}

func TestRenderStatusBar_FilterActive(t *testing.T) {
	p, s := testFixture(t)
	out := RenderStatusBar(p, s, viewTree, "tikhonov", true, "test", 0, 120)
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
		out := RenderStatusBar(p, s, viewTree, "test", false, "", i, 120)
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
