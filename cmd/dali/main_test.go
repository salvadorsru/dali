package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMCPMode pins the two startup modes: a design file mounts that design,
// while a directory, with one design or many, always stays a context root.
func TestMCPMode(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	many := filepath.Join(root, "many")
	for dir, names := range map[string][]string{
		one:  {"only.fig"},
		many: {"a.fig", "b.fig"},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("DALI_FIG_DIR", root)

	design := filepath.Join(many, "a.fig")
	for _, c := range []struct {
		name string
		in   string
		fig  string
		root string
	}{
		{"design file", design, design, ""},
		{"directory with one design", one, "", one},
		{"directory with several designs", many, "", many},
		{"no argument", "", "", root},
	} {
		fig, dir := mcpMode(c.in)
		if fig != c.fig || dir != c.root {
			t.Fatalf("%s: mcpMode(%q) = %q, %q; want %q, %q", c.name, c.in, fig, dir, c.fig, c.root)
		}
	}
}
