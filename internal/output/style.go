// Package output renders CLI results: JSON, styled tables, progress bars and
// status spinners. All visual output goes to stderr; stdout stays clean for
// data (--json) and for the MCP stdio stream.
package output

import (
	"os"

	"github.com/mattn/go-isatty"
)

var (
	enabled = detectColor()
	quiet   bool
)

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if v := os.Getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTTY(os.Stdout.Fd())
}

func isTTY(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// SetEnabled turns color on or off (--no-color).
func SetEnabled(v bool) { enabled = v }

// Enabled reports whether color is on.
func Enabled() bool { return enabled }

// SetQuiet silences progress bars and status spinners.
func SetQuiet(v bool) { quiet = v }

// Quiet reports whether progress output is suppressed.
func Quiet() bool { return quiet }

// Interactive reports whether stderr can be animated (bars, spinners).
func Interactive() bool { return isTTY(os.Stderr.Fd()) && !quiet }

func paint(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func Dim(s string) string     { return paint("2", s) }
func Bold(s string) string    { return paint("1", s) }
func Title(s string) string   { return paint("1;36", s) }
func Label(s string) string   { return paint("36", s) }
func Success(s string) string { return paint("32", s) }
func Warn(s string) string    { return paint("33", s) }
func Error(s string) string   { return paint("31", s) }
func ID(s string) string      { return paint("90", s) }

var typeColors = map[string]string{
	"FRAME": "36", "CANVAS": "1;36", "SECTION": "36",
	"TEXT": "33", "SYMBOL": "35", "COMPONENT": "35", "COMPONENT_SET": "35",
	"INSTANCE": "34", "VECTOR": "90", "GROUP": "90",
	"RECTANGLE": "32", "ROUNDED_RECTANGLE": "32", "ELLIPSE": "32",
	"STAR": "32", "LINE": "32", "REGULAR_POLYGON": "32",
}

// Type colors a node type for scanability.
func Type(t string) string {
	if !enabled || t == "" {
		return t
	}
	code := typeColors[t]
	if code == "" {
		code = "37"
	}
	return paint(code, t)
}
