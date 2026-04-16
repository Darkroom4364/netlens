package style

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-isatty"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Delay thresholds (milliseconds) used for color coding and congestion alerts.
const (
	DelayLowMS        = 2  // green→yellow boundary in DOT output
	DelayMediumMS     = 5  // green→yellow boundary in CLI/TUI
	DelayHighMS       = 10 // yellow→red boundary in DOT output
	DelayCongestionMS = 20 // congestion alert threshold in CLI/TUI
)

// Enabled controls whether ANSI styling is applied.
// Automatically set based on TTY detection and NO_COLOR env var.
// Can be overridden via SetEnabled (e.g., --no-color flag).
var Enabled = isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("NO_COLOR") == ""

// IsTTY returns whether stdout is a terminal.
var IsTTY = isatty.IsTerminal(os.Stdout.Fd())

// SetEnabled overrides automatic TTY/NO_COLOR detection.
func SetEnabled(v bool) { Enabled = v }

// ANSI escape helpers — return plain strings when styling is disabled.

func Bold(s string) string {
	if !Enabled {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func Dim(s string) string {
	if !Enabled {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

func Red(s string) string {
	if !Enabled {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func Yellow(s string) string {
	if !Enabled {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func Green(s string) string {
	if !Enabled {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

// PadRight pads s to width based on visible character count,
// correctly handling ANSI escape sequences.
func PadRight(s string, width int) string {
	visible := utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// ColorDelay returns a color-coded delay string.
// Green <5ms, yellow 5-20ms, red >20ms.
func ColorDelay(ms float64) string {
	s := fmt.Sprintf("%.3f", ms)
	if ms < 0 {
		return Red(s)
	}
	if ms < DelayMediumMS {
		return Green(s)
	}
	if ms < DelayCongestionMS {
		return Yellow(s)
	}
	return Red(s)
}

// ColorIdent returns a styled identifiability string.
func ColorIdent(identifiable bool) string {
	if identifiable {
		return "yes"
	}
	return Dim("NO")
}
