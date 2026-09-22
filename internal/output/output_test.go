package output

import (
	"strings"
	"testing"
)

func TestVisibleLen(t *testing.T) {
	if got := visibleLen("\x1b[31mred\x1b[0m"); got != 3 {
		t.Fatalf("visibleLen with ANSI = %d, want 3", got)
	}
	if got := visibleLen("plain"); got != 5 {
		t.Fatalf("visibleLen = %d, want 5", got)
	}
}

func TestColorToggle(t *testing.T) {
	SetEnabled(false)
	if Dim("x") != "x" || Type("TEXT") != "TEXT" {
		t.Fatal("color should be off")
	}
	SetEnabled(true)
	if !strings.Contains(Type("TEXT"), "\x1b[") {
		t.Fatal("color should be on")
	}
	SetEnabled(false)
}

func TestTableAligns(t *testing.T) {
	SetEnabled(true)
	defer SetEnabled(false)
	var b strings.Builder
	Table(&b, []string{"A", "B"}, [][]string{
		{"\x1b[31mxx\x1b[0m", "y"},
		{"z", "ww"},
	})
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	want := visibleLen(lines[0])
	for i, l := range lines {
		if visibleLen(l) != want {
			t.Fatalf("line %d visible width = %d, want %d", i, visibleLen(l), want)
		}
	}
}

func TestCommas(t *testing.T) {
	if got := commas(13540); got != "13,540" {
		t.Fatalf("commas = %q", got)
	}
	if got := commas(999); got != "999" {
		t.Fatalf("commas = %q", got)
	}
}
