package style

import (
	"strings"
	"testing"
)

func withEnabled(t *testing.T, v bool) {
	t.Helper()
	old := Enabled
	t.Cleanup(func() { Enabled = old })
	Enabled = v
}

func TestBold(t *testing.T) {
	withEnabled(t, true)
	if out := Bold("x"); !strings.Contains(out, "\033[1m") {
		t.Errorf("Bold enabled: expected ANSI bold, got %q", out)
	}
	withEnabled(t, false)
	if out := Bold("x"); out != "x" {
		t.Errorf("Bold disabled: expected passthrough, got %q", out)
	}
}

func TestDim(t *testing.T) {
	withEnabled(t, true)
	if out := Dim("x"); !strings.Contains(out, "\033[2m") {
		t.Errorf("Dim enabled: expected ANSI dim, got %q", out)
	}
	withEnabled(t, false)
	if out := Dim("x"); out != "x" {
		t.Errorf("Dim disabled: expected passthrough, got %q", out)
	}
}

func TestColors(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
		code string
	}{
		{"Red", Red, "\033[31m"},
		{"Yellow", Yellow, "\033[33m"},
		{"Green", Green, "\033[32m"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"_Enabled", func(t *testing.T) {
			withEnabled(t, true)
			if out := tc.fn("x"); !strings.Contains(out, tc.code) {
				t.Errorf("expected %s code in %q", tc.name, out)
			}
		})
		t.Run(tc.name+"_Disabled", func(t *testing.T) {
			withEnabled(t, false)
			if out := tc.fn("x"); out != "x" {
				t.Errorf("expected passthrough, got %q", out)
			}
		})
	}
}

func TestPadRight_Plain(t *testing.T) {
	if out := PadRight("abc", 6); out != "abc   " {
		t.Errorf("expected %q, got %q", "abc   ", out)
	}
}

func TestPadRight_WithANSI(t *testing.T) {
	withEnabled(t, true)
	s := Red("abc") // has ANSI escapes wrapping 3 visible chars
	out := PadRight(s, 6)
	// Strip ANSI to check visible width
	visible := ansiRe.ReplaceAllString(out, "")
	if len(visible) != 6 {
		t.Errorf("expected visible width 6, got %d (%q)", len(visible), visible)
	}
}

func TestPadRight_AlreadyWide(t *testing.T) {
	if out := PadRight("abcdef", 3); out != "abcdef" {
		t.Errorf("expected unchanged string, got %q", out)
	}
}

func TestPadRight_ExactWidth(t *testing.T) {
	if out := PadRight("abc", 3); out != "abc" {
		t.Errorf("expected unchanged string, got %q", out)
	}
}

func TestColorDelay(t *testing.T) {
	withEnabled(t, true)
	cases := []struct {
		name string
		ms   float64
		code string
	}{
		{"green", 3.0, "\033[32m"},
		{"yellow", 10.0, "\033[33m"},
		{"red", 25.0, "\033[31m"},
		{"negative", -1.0, "\033[31m"},
		{"boundary_5", 4.999, "\033[32m"},
		{"boundary_20", 19.999, "\033[33m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ColorDelay(tc.ms)
			if !strings.Contains(out, tc.code) {
				t.Errorf("ColorDelay(%.1f): expected %q in %q", tc.ms, tc.code, out)
			}
		})
	}
}

func TestColorDelay_Disabled(t *testing.T) {
	withEnabled(t, false)
	out := ColorDelay(10.0)
	if strings.Contains(out, "\033") {
		t.Errorf("expected no ANSI when disabled, got %q", out)
	}
}

func TestColorIdent(t *testing.T) {
	if out := ColorIdent(true); out != "yes" {
		t.Errorf("expected %q, got %q", "yes", out)
	}
	withEnabled(t, true)
	out := ColorIdent(false)
	if !strings.Contains(out, "NO") {
		t.Errorf("expected NO in output, got %q", out)
	}
	if !strings.Contains(out, "\033[2m") {
		t.Errorf("expected dim ANSI for false, got %q", out)
	}
}

func TestSetEnabled(t *testing.T) {
	old := Enabled
	t.Cleanup(func() { Enabled = old })
	SetEnabled(true)
	if !Enabled {
		t.Error("SetEnabled(true) did not set Enabled")
	}
	SetEnabled(false)
	if Enabled {
		t.Error("SetEnabled(false) did not clear Enabled")
	}
}
