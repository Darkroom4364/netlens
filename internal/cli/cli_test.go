package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
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

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	captured, _ := io.ReadAll(r)
	r.Close()

	// Merge cobra buffer and captured stdout.
	out := buf.String() + string(captured)
	return out, err
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

func TestCLI_SimulateUnknownSolverFallsToDefault(t *testing.T) {
	out, err := executeCommand("simulate", "-t", "../../testdata/topologies/abilene.graphml", "-m", "unknown_solver")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The default solver is tikhonov
	if !strings.Contains(out, "Solver:") {
		t.Fatalf("expected output to contain 'Solver:', got:\n%s", out)
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
