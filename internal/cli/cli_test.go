package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// executeCommand runs the CLI with the given args and captures both the
// cobra output buffer and anything written to os.Stdout (which most
// subcommands use via fmt.Printf).
func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd := NewRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	// Capture os.Stdout since subcommands use fmt.Printf.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	var errExec error
	errExec = cmd.Execute()

	if err := w.Close(); err != nil && errExec == nil {
		errExec = err
	}

	captured, _ := io.ReadAll(r)
	if err := r.Close(); err != nil && errExec == nil {
		errExec = err
	}

	// Merge cobra buffer and captured stdout.
	out := buf.String() + string(captured)
	return out, errExec
}

// --- Tests ---

func TestCLI_Version(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "netlens") {
		t.Fatalf("expected output to contain 'netlens', got: %s", out)
	}
}

func TestCLI_HelpListsSubcommands(t *testing.T) {
	out, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"simulate", "scan", "benchmark", "plan"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected help to contain %q, got:\n%s", sub, out)
		}
	}
}

func TestCLI_SimulateTikhonov(t *testing.T) {
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "tikhonov")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Solver:") {
		t.Fatalf("expected output to contain 'Solver:', got:\n%s", out)
	}
}

func TestCLI_SimulateNNLS(t *testing.T) {
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "nnls", "--noise", "0")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
}

func TestCLI_SimulateADMM(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "admm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SimulateVardi(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "vardi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SimulateTomogravity(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "tomogravity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SimulateNonexistentTopology(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "nonexistent.graphml")
	if err == nil {
		t.Fatal("expected error for nonexistent topology file, got nil")
	}
}

func TestCLI_SimulateUnknownSolverReturnsError(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "unknown_solver")
	if err == nil {
		t.Fatal("expected error for unknown solver method, got nil")
	}
	if !strings.Contains(err.Error(), "unknown solver method") {
		t.Fatalf("expected 'unknown solver method' error, got: %v", err)
	}
}

func TestCLI_BenchmarkTopologies(t *testing.T) {
	out, err := executeCommand("benchmark", "-t", "../../testdata/topologies")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Topology") {
		t.Fatalf("expected output to contain 'Topology', got:\n%s", out)
	}
}

func TestCLI_BenchmarkSynthetic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping synthetic benchmark under -short (Laplacian SVD too slow with race detector)")
	}
	out, err := executeCommand("benchmark", "--synthetic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ba-") && !strings.Contains(out, "waxman-") {
		t.Fatalf("expected output to contain 'ba-' or 'waxman-', got:\n%s", out)
	}
}

func TestCLI_PlanBudget5(t *testing.T) {
	out, err := executeCommand("plan", "-t", "../../testdata/topologies/abilene.graphml", "--budget", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Step") {
		t.Fatalf("expected output to contain 'Step', got:\n%s", out)
	}
}

func TestCLI_PlanBudget0(t *testing.T) {
	out, err := executeCommand("plan", "-t", "../../testdata/topologies/abilene.graphml", "--budget", "0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "0 probes") && !strings.Contains(out, "No useful probes") {
		t.Fatalf("expected output about 0 probes, got:\n%s", out)
	}
}

func TestCLI_ScanTraceroute(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
}

func TestCLI_ScanTracerouteJSON(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
	// The output contains preamble text lines followed by JSON.
	// Find the outermost JSON structure and validate it.
	jsonFound := false
	// Try to find a JSON array or object in the output.
	for _, startChar := range []string{"[", "{"} {
		idx := strings.Index(out, startChar)
		if idx < 0 {
			continue
		}
		// Find the matching closing bracket.
		jsonPart := out[idx:]
		var js json.RawMessage
		if err := json.Unmarshal([]byte(jsonPart), &js); err == nil {
			jsonFound = true
			break
		}
		// Try trimming trailing non-JSON text.
		trimmed := strings.TrimSpace(jsonPart)
		if err := json.Unmarshal([]byte(trimmed), &js); err == nil {
			jsonFound = true
			break
		}
	}
	if !jsonFound {
		t.Fatalf("expected valid JSON output somewhere in:\n%s", out)
	}
}

func TestCLI_ScanTracerouteCSV(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Find the CSV header line (skip the text preamble)
	found := false
	for _, line := range lines {
		if strings.Contains(line, ",") && (strings.Contains(strings.ToLower(line), "link") || strings.Contains(strings.ToLower(line), "src")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected CSV with header, got:\n%s", out)
	}
}

func TestCLI_ScanTracerouteDOT(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--format", "dot")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "graph") {
		t.Fatalf("expected DOT output containing 'graph', got:\n%s", out)
	}
}

func TestCLI_ScanNonexistentSource(t *testing.T) {
	_, err := executeCommand("scan", "--source", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent source, got nil")
	}
}

func TestCLI_ScanRipeNoMsm(t *testing.T) {
	_, err := executeCommand("scan", "--source", "ripe")
	if err == nil {
		t.Fatal("expected error when --source=ripe without --msm, got nil")
	}
}

func TestCLI_CompletionBash(t *testing.T) {
	out, err := executeCommand("completion", "bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "# bash completion") {
		t.Fatalf("expected bash completion output starting with '# bash completion', got:\n%.200s", out)
	}
}

func TestCLI_CompletionZsh(t *testing.T) {
	out, err := executeCommand("completion", "zsh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected non-empty zsh completion output")
	}
}

func TestCLI_CompletionInvalidShell(t *testing.T) {
	_, err := executeCommand("completion", "invalid_shell")
	if err == nil {
		t.Fatal("expected error for invalid shell, got nil")
	}
}

// --- Missing solver coverage ---

func TestCLI_SimulateTSVD(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "tsvd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SimulateIRL1(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "irl1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SimulateLaplacian(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "laplacian")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Flag combination tests ---

func TestCLI_SimulateNoColor(t *testing.T) {
	// Baseline: default run may or may not have ANSI (pipe detection can
	// suppress it), so this test only verifies --no-color doesn't crash
	// and produces clean output.
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "--no-color")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "\033") {
		t.Error("expected no ANSI escape sequences with --no-color")
	}
}

func TestCLI_SimulateQuiet(t *testing.T) {
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "--quiet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Topology:") {
		t.Error("expected verbose preamble to be suppressed with --quiet")
	}
}

func TestCLI_SimulateNoColorQuiet(t *testing.T) {
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "--no-color", "--quiet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "\033") {
		t.Error("expected no ANSI with --no-color --quiet")
	}
	if strings.Contains(out, "Topology:") {
		t.Error("expected verbose preamble suppressed with --quiet")
	}
}

// --- TUI subcommand ---

func TestCLI_TUISubcommandDetection(t *testing.T) {
	out, _ := executeCommand("--help")
	if !strings.Contains(out, "tui") {
		t.Skip("tui subcommand not available (build without -tags tui)")
	}
	// Verify it requires -t flag
	_, err := executeCommand("tui")
	if err == nil {
		t.Error("expected error when tui is called without -t flag")
	}
}

// --- Additional coverage tests ---

func TestCLI_SetVersion(t *testing.T) {
	SetVersion("1.2.3")
	defer SetVersion("dev")

	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Fatalf("expected output to contain '1.2.3', got: %s", out)
	}
}

func TestCLI_LoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envContent := `# comment line
TEST_NETLENS_FOO=bar
TEST_NETLENS_QUOTED="hello world"
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
		os.Unsetenv("TEST_NETLENS_FOO")
		os.Unsetenv("TEST_NETLENS_QUOTED")
	}()

	_, _ = executeCommand("version")

	if got := os.Getenv("TEST_NETLENS_FOO"); got != "bar" {
		t.Errorf("expected TEST_NETLENS_FOO=bar, got %q", got)
	}
	if got := os.Getenv("TEST_NETLENS_QUOTED"); got != "hello world" {
		t.Errorf("expected TEST_NETLENS_QUOTED='hello world', got %q", got)
	}
}

func TestCLI_ScanTableFormat(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--format", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty output for table format")
	}
}

func TestCLI_GetSolverAllValid(t *testing.T) {
	names := []string{"tsvd", "tikhonov", "nnls", "admm", "irl1", "vardi", "tomogravity", "laplacian"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			s, err := getSolver(name)
			if err != nil {
				t.Fatalf("getSolver(%q) returned error: %v", name, err)
			}
			if s == nil {
				t.Fatalf("getSolver(%q) returned nil solver", name)
			}
		})
	}
}

func TestCLI_GetSolverEmpty(t *testing.T) {
	_, err := getSolver("")
	if err == nil {
		t.Fatal("expected error for empty solver name")
	}
	if !strings.Contains(err.Error(), "unknown solver method") {
		t.Fatalf("expected 'unknown solver method' in error, got: %v", err)
	}
}

func TestCLI_WarnNegativeDelays(t *testing.T) {
	sol := &tomo.Solution{
		X: mat.NewVecDense(3, []float64{1.0, -0.5, -0.2}),
	}

	// With tikhonov, should warn about negative delays.
	cmd := &cobra.Command{}
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)
	warnNegativeDelays(cmd, sol, "tikhonov")
	if !strings.Contains(errBuf.String(), "negative delay") {
		t.Errorf("expected warning about negative delay for tikhonov, got: %q", errBuf.String())
	}

	// With nnls, should NOT warn (nnls guarantees non-negativity).
	cmd2 := &cobra.Command{}
	errBuf2 := new(bytes.Buffer)
	cmd2.SetErr(errBuf2)
	warnNegativeDelays(cmd2, sol, "nnls")
	if errBuf2.Len() > 0 {
		t.Errorf("expected no warning for nnls, got: %q", errBuf2.String())
	}
}

func TestCLI_PrintScanTable(t *testing.T) {
	// Build a minimal Problem + Solution to exercise printScanTable.
	p := &tomo.Problem{
		Links: []tomo.Link{
			{ID: 0, Src: 1, Dst: 2},
			{ID: 1, Src: 2, Dst: 3},
			{ID: 2, Src: 3, Dst: 4},
		},
		Quality: &tomo.MatrixQuality{
			Rank:                3,
			NumLinks:            3,
			NumPaths:            3,
			ConditionNumber:     1.5,
			IdentifiableFrac:    1.0,
			UnidentifiableLinks: nil,
			CoveragePerLink:     []int{2, 3, 1},
		},
	}
	sol := &tomo.Solution{
		X: mat.NewVecDense(3, []float64{0.5, 15.0, 2.0}),
	}

	// Capture stdout (printScanTable uses fmt.Printf).
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printScanTable(p, sol, 0, false)

	w.Close()
	captured, _ := io.ReadAll(r)
	r.Close()
	os.Stdout = oldStdout

	out := string(captured)
	if !strings.Contains(out, "links") {
		t.Errorf("expected 'links' in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "Identifiable links") {
		t.Errorf("expected 'Identifiable links' in table output, got:\n%s", out)
	}
}

func TestCLI_PrintScanTableWithTop(t *testing.T) {
	p := &tomo.Problem{
		Links: []tomo.Link{
			{ID: 0, Src: 1, Dst: 2},
			{ID: 1, Src: 2, Dst: 3},
			{ID: 2, Src: 3, Dst: 4},
		},
		Quality: &tomo.MatrixQuality{
			Rank:                3,
			NumLinks:            3,
			NumPaths:            3,
			ConditionNumber:     1.5,
			IdentifiableFrac:    1.0,
			UnidentifiableLinks: nil,
			CoveragePerLink:     []int{2, 3, 1},
		},
	}
	sol := &tomo.Solution{
		X: mat.NewVecDense(3, []float64{0.5, 15.0, 2.0}),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printScanTable(p, sol, 1, true) // top=1, quiet=true

	w.Close()
	captured, _ := io.ReadAll(r)
	r.Close()
	os.Stdout = oldStdout

	out := string(captured)
	if out == "" {
		t.Error("expected non-empty output from printScanTable with top=1")
	}
}

func TestCLI_ScanMaxAnonymousInvalid(t *testing.T) {
	_, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--max-anonymous", "1.5")
	if err == nil {
		t.Fatal("expected error for --max-anonymous > 1")
	}
	if !strings.Contains(err.Error(), "max-anonymous") {
		t.Fatalf("expected error about max-anonymous, got: %v", err)
	}
}

func TestCLI_ScanTracerouteNoFile(t *testing.T) {
	_, err := executeCommand("scan", "--source", "traceroute")
	if err == nil {
		t.Fatal("expected error when --source=traceroute without --file")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Fatalf("expected error about --file, got: %v", err)
	}
}

func TestCLI_ScanUnknownFormat(t *testing.T) {
	_, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--format", "xml")
	if err == nil {
		t.Fatal("expected error for unknown format 'xml'")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("expected 'unknown format' error, got: %v", err)
	}
}

func TestCLI_CompletionFish(t *testing.T) {
	out, err := executeCommand("completion", "fish")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected non-empty fish completion output")
	}
}

func TestCLI_BenchmarkNonexistentDir(t *testing.T) {
	_, err := executeCommand("benchmark", "-t", "/nonexistent/path/topos")
	if err == nil {
		t.Fatal("expected error for nonexistent topology directory")
	}
}

func TestCLI_SimulateCustomNoise(t *testing.T) {
	_, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "nnls", "--noise", "0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_ScanQuiet(t *testing.T) {
	out, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "--quiet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Topology:") {
		t.Error("expected preamble suppressed with --quiet")
	}
}

func TestCLI_ScanWithTikhonov(t *testing.T) {
	// Exercises the scan path with a non-nnls solver (triggers warnNegativeDelays in context).
	_, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "-m", "tikhonov")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_PlanHighBudget(t *testing.T) {
	// With a high budget, should reach full rank and print "all links identifiable".
	out, err := executeCommand("plan", "-t", "../../testdata/topologies/abilene.graphml", "--budget", "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should contain either "all links identifiable" or "Rank deficit"
	if !strings.Contains(out, "identifiable") && !strings.Contains(out, "Rank deficit") {
		t.Errorf("expected rank summary in output, got:\n%s", out)
	}
}

func TestCLI_BenchmarkCustomFlags(t *testing.T) {
	out, err := executeCommand("benchmark", "-t", "../../testdata/topologies", "--noise", "0.2", "--congestion-links", "3", "--seed", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Topology") {
		t.Fatalf("expected output to contain 'Topology', got:\n%s", out)
	}
}

func TestCLI_ScanWithMethod(t *testing.T) {
	// Exercise scan with different solver methods.
	for _, method := range []string{"tsvd", "admm"} {
		t.Run(method, func(t *testing.T) {
			_, err := executeCommand("scan", "--source", "traceroute", "--file", "../../testdata/measurements/ripe_atlas_sample.json", "-m", method)
			if err != nil {
				t.Fatalf("unexpected error for method %s: %v", method, err)
			}
		})
	}
}

func TestCLI_LoadDotEnvSingleQuoted(t *testing.T) {
	dir := t.TempDir()
	envContent := "TEST_NETLENS_SQ='single quoted'\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
		os.Unsetenv("TEST_NETLENS_SQ")
	}()

	_, _ = executeCommand("version")

	if got := os.Getenv("TEST_NETLENS_SQ"); got != "single quoted" {
		t.Errorf("expected TEST_NETLENS_SQ='single quoted', got %q", got)
	}
}

func TestCLI_BenchmarkGaussianNoise(t *testing.T) {
	out, err := executeCommand("benchmark", "-t", "../../testdata/topologies", "--noise-model", "gaussian")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Topology") {
		t.Fatalf("expected output to contain 'Topology', got:\n%s", out)
	}
}

func TestCLI_RootNoArgs(t *testing.T) {
	// Running with no args should show help (exercises the RunE path).
	out, err := executeCommand()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "netlens") {
		t.Fatalf("expected help output, got:\n%s", out)
	}
}
