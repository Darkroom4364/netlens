package format

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/tomo"
	"gonum.org/v1/gonum/mat"
)

// ---------- helpers ----------

// makeProblemSolution creates a minimal Problem+Solution for testing formatters.
func makeProblemSolution(labels ...string) (*tomo.Problem, *tomo.Solution) {
	nLinks := len(labels)
	if nLinks == 0 {
		nLinks = 2
		labels = []string{"nodeA", "nodeB", "nodeC"}
	}

	links := make([]tomo.Link, nLinks)
	for i := 0; i < nLinks; i++ {
		links[i] = tomo.Link{ID: i, Src: i, Dst: i + 1}
	}

	nodes := make([]tomo.Node, len(labels))
	for i, l := range labels {
		nodes[i] = tomo.Node{ID: i, Label: l}
	}

	p := &tomo.Problem{
		Topo:  &deepStubTopo{nodes: nodes},
		Links: links,
		A:     mat.NewDense(1, nLinks, nil),
		B:     mat.NewVecDense(1, nil),
		Quality: &tomo.MatrixQuality{
			Rank:            nLinks,
			NumLinks:        nLinks,
			ConditionNumber: 1.0,
			IdentifiableFrac: 1.0,
		},
	}

	xData := make([]float64, nLinks)
	confData := make([]float64, nLinks)
	ident := make([]bool, nLinks)
	for i := 0; i < nLinks; i++ {
		xData[i] = float64(i+1) * 1.5
		confData[i] = 0.1
		ident[i] = true
	}

	s := &tomo.Solution{
		X:            mat.NewVecDense(nLinks, xData),
		Confidence:   mat.NewVecDense(nLinks, confData),
		Identifiable: ident,
		Method:       "test",
		Duration:     100 * time.Millisecond,
	}

	return p, s
}

// deepStubTopo implements enough of tomo.Topology for the deep formatters tests.
type deepStubTopo struct {
	nodes []tomo.Node
}

func (s *deepStubTopo) NumNodes() int                        { return len(s.nodes) }
func (s *deepStubTopo) NumLinks() int                        { return 0 }
func (s *deepStubTopo) Links() []tomo.Link                   { return nil }
func (s *deepStubTopo) Nodes() []tomo.Node                   { return s.nodes }
func (s *deepStubTopo) Neighbors(int) []int                  { return nil }
func (s *deepStubTopo) ShortestPath(int, int) ([]int, bool)  { return nil, false }
func (s *deepStubTopo) AllPairsShortestPaths() []tomo.PathSpec { return nil }

// ---------- 1. CSV with comma in node label ----------

func TestFormatDeepCSVCommaInLabel(t *testing.T) {
	// The CSV formatter uses link Src/Dst IDs, not labels, so commas in labels
	// don't directly appear in CSV output. But we verify the output parses cleanly.
	p, s := makeProblemSolution("node,with,commas", "other")
	var buf bytes.Buffer
	f := &CSVFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	// Parse back to verify it's valid CSV.
	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV output not parseable: %v\nOutput:\n%s", err, buf.String())
	}
	// Header + 1 link row (labels has 2 entries so nLinks=1... actually nLinks=len(labels))
	// We have nLinks=2 labels, so 2 links.
	if len(records) < 2 {
		t.Errorf("expected at least header + 1 data row, got %d rows", len(records))
	}
}

// ---------- 2. CSV with newline in label ----------

func TestFormatDeepCSVNewlineInLabel(t *testing.T) {
	p, s := makeProblemSolution("line1\nline2", "other")
	var buf bytes.Buffer
	f := &CSVFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	// The CSV format doesn't emit node labels, so newlines in labels
	// don't appear. We just verify valid CSV output.
	r := csv.NewReader(strings.NewReader(buf.String()))
	_, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV with newline labels not parseable: %v", err)
	}
}

// ---------- 3. DOT with angle brackets in label ----------

func TestFormatDeepDOTAngleBrackets(t *testing.T) {
	p, s := makeProblemSolution("<script>alert('xss')</script>", "normal")
	var buf bytes.Buffer
	f := &DOTFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	// The DOT formatter uses %q for labels, which escapes angle brackets.
	if !strings.Contains(out, "graph") {
		t.Error("DOT output missing 'graph' keyword")
	}
	// Angle brackets should be inside a quoted string, not bare.
	// %q will produce something like "<script>..." with quotes.
	if strings.Contains(out, `[label=<script>`) {
		t.Error("angle brackets not properly quoted in DOT label")
	}
}

// ---------- 4. DOT with backslash in label ----------

func TestFormatDeepDOTBackslash(t *testing.T) {
	p, s := makeProblemSolution(`node\path`, "other")
	var buf bytes.Buffer
	f := &DOTFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	// %q escapes backslashes to \\, which GraphViz handles correctly.
	if !strings.Contains(out, `\\`) {
		t.Error("backslash should be escaped in DOT output")
	}
}

// ---------- 5. JSON with unicode characters ----------

func TestFormatDeepJSONUnicode(t *testing.T) {
	p, s := makeProblemSolution("Zurich", "Tokyo")
	// Method with unicode.
	s.Method = "tsvd-\u00e9l\u00e8ve"

	var buf bytes.Buffer
	f := &JSONFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	// Verify it's valid UTF-8 by parsing as JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON with unicode not valid: %v", err)
	}
	if m, ok := parsed["method"].(string); !ok || m != "tsvd-\u00e9l\u00e8ve" {
		t.Errorf("method field mismatch: got %v", parsed["method"])
	}
}

// ---------- 6. JSON output is valid JSON ----------

func TestFormatDeepJSONValid(t *testing.T) {
	p, s := makeProblemSolution("A", "B", "C")
	var buf bytes.Buffer
	f := &JSONFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput:\n%s", err, buf.String())
	}

	// Check expected keys.
	for _, key := range []string{"method", "duration_ms", "residual", "links"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
}

// ---------- 7. CSV correct column count per row ----------

func TestFormatDeepCSVColumnCount(t *testing.T) {
	p, s := makeProblemSolution("A", "B", "C", "D")
	var buf bytes.Buffer
	f := &CSVFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	// Header has 6 columns.
	expectedCols := 6
	for i, row := range records {
		if len(row) != expectedCols {
			t.Errorf("row %d: expected %d columns, got %d: %v", i, expectedCols, len(row), row)
		}
	}
}

// ---------- 8. DOT starts with "graph" and ends with "}" ----------

func TestFormatDeepDOTStructure(t *testing.T) {
	p, s := makeProblemSolution("X", "Y")
	var buf bytes.Buffer
	f := &DOTFormatter{}
	if err := f.Format(&buf, p, s); err != nil {
		t.Fatal(err)
	}

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "graph") {
		t.Errorf("DOT output should start with 'graph', got: %.30s...", out)
	}
	if !strings.HasSuffix(out, "}") {
		t.Errorf("DOT output should end with '}', got: ...%s", out[max(0, len(out)-20):])
	}
}

// ---------- 9. All three formatters produce non-empty output ----------

func TestFormatDeepAllFormattersNonEmpty(t *testing.T) {
	p, s := makeProblemSolution("R1", "R2", "R3")

	formatters := []struct {
		name string
		fmt  interface{ Format(*bytes.Buffer, *tomo.Problem, *tomo.Solution) error }
	}{
		// We can't use a common interface with io.Writer, so test individually.
	}
	_ = formatters

	// Test CSV
	var csvBuf bytes.Buffer
	if err := (&CSVFormatter{}).Format(&csvBuf, p, s); err != nil {
		t.Fatalf("CSV format error: %v", err)
	}
	if csvBuf.Len() == 0 {
		t.Error("CSV output is empty")
	}

	// Test DOT
	var dotBuf bytes.Buffer
	if err := (&DOTFormatter{}).Format(&dotBuf, p, s); err != nil {
		t.Fatalf("DOT format error: %v", err)
	}
	if dotBuf.Len() == 0 {
		t.Error("DOT output is empty")
	}

	// Test JSON
	var jsonBuf bytes.Buffer
	if err := (&JSONFormatter{}).Format(&jsonBuf, p, s); err != nil {
		t.Fatalf("JSON format error: %v", err)
	}
	if jsonBuf.Len() == 0 {
		t.Error("JSON output is empty")
	}
}
