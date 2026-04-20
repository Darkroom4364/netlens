package measure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseScamperFile(t *testing.T) {
	// Write a minimal scamper JSON to a temp file and parse it
	data := `[{"type":"trace","src":"1.0.0.1","dst":"2.0.0.1","hops":[{"addr":"1.0.0.1","probe_ttl":1,"rtt":1.5},{"addr":"2.0.0.1","probe_ttl":2,"rtt":5.0}]}]`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	measurements, err := ParseScamperFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(measurements) == 0 {
		t.Fatal("expected at least one measurement")
	}
}

func TestParseScamperFile_Nonexistent(t *testing.T) {
	_, err := ParseScamperFile("/nonexistent/path/file.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
