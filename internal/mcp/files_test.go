package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// designs builds the tree discovery must handle: nested projects, a .fig/.deck
// collision on one base name and pruned directories holding a design each.
func designs(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{
		"woments/woments.fig",
		"woments/woments.deck",
		"woments/marketing.fig",
		"halibut/halibut.fig",
		"node_modules/nope.fig",
		".git/history.fig",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestDiscover pins recursion, pruning, absolute paths, deduplication and the
// identity of a .fig/.deck collision.
func TestDiscover(t *testing.T) {
	root := designs(t)
	want := []string{"halibut/halibut", "woments/marketing", "woments/woments.deck", "woments/woments.fig"}

	found := discover(root)
	if len(found) != len(want) {
		t.Fatalf("discover found %d designs, want %d: %+v", len(found), len(want), names(found))
	}
	seen := make(map[string]bool, len(found))
	for i, e := range found {
		if e.name != want[i] {
			t.Fatalf("design %d = %q, want %q", i, e.name, want[i])
		}
		if seen[e.name] {
			t.Fatalf("duplicate identity %q", e.name)
		}
		seen[e.name] = true
		if !filepath.IsAbs(e.path) {
			t.Fatalf("path not absolute: %s", e.path)
		}
		if !inside(root, e.path) {
			t.Fatalf("path outside root: %s", e.path)
		}
		if strings.Contains(e.path, "node_modules") || strings.Contains(e.path, string(filepath.Separator)+".git") {
			t.Fatalf("pruned tree listed: %s", e.path)
		}
	}
	if again := discover(root); len(again) != len(found) {
		t.Fatalf("discover is not idempotent: %d then %d", len(found), len(again))
	}
}

// TestIdentity pins the naming rule: path relative to root, slash form, no
// extension, so the identity survives moving the whole tree.
func TestIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "designs")
	got := identity(root, filepath.Join(root, "woments", "woments.fig"))
	if got != "woments/woments" {
		t.Fatalf("identity = %q, want woments/woments", got)
	}
	if got := identity(root, filepath.Join(root, "woments", "woments.deck")); got != "woments/woments" {
		t.Fatalf("identity of deck = %q, want woments/woments", got)
	}
}

// TestMountedMode: figma mode serves one design, accepts it by path or by a path
// resolving to it, and explains the mode for any other design.
func TestMountedMode(t *testing.T) {
	root := designs(t)
	fig := filepath.Join(root, "woments", "woments.fig")
	_, h := newServer(fig, "", nil)
	defer h.close()

	if got, err := h.resolve(""); err != nil || got != fig {
		t.Fatalf("omitted file = %q, %v", got, err)
	}
	if got, err := h.resolve(fig); err != nil || got != fig {
		t.Fatalf("mounted path = %q, %v", got, err)
	}
	if got, err := h.resolve("woments.fig"); err != nil || got != fig {
		t.Fatalf("path relative to root = %q, %v", got, err)
	}
	for _, foreign := range []string{
		filepath.Join(root, "woments", "marketing.fig"),
		filepath.Join(root, "halibut", "halibut.fig"),
	} {
		if _, err := h.resolve(foreign); err == nil || !strings.Contains(err.Error(), "serves one design") {
			t.Fatalf("foreign %s = %v", filepath.Base(foreign), err)
		}
	}
}

// TestLookup: context mode resolves an identity, a project/file pair, a path
// relative to root and an absolute path inside root, and never guesses.
func TestLookup(t *testing.T) {
	root := designs(t)
	marketing := filepath.Join(root, "woments", "marketing.fig")
	halibut := filepath.Join(root, "halibut", "halibut.fig")
	_, h := newServer("", root, nil)
	defer h.close()

	for _, c := range []struct{ id, want string }{
		{"halibut/halibut", halibut},
		{"halibut", halibut},
		{"halibut/hal", halibut},
		{"woments/marketing", marketing},
		{"marketing", marketing},
		{"woments/marketing.fig", marketing},
		{marketing, marketing},
	} {
		got, err := h.lookup(c.id)
		if err != nil || got != c.want {
			t.Fatalf("lookup %q = %q, %v; want %q", c.id, got, err, c.want)
		}
	}

	outside := filepath.Join(t.TempDir(), "x.fig")
	if _, err := h.lookup(outside); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("absolute path outside root = %v", err)
	}

	_, err := h.lookup("")
	if err == nil || !strings.Contains(err.Error(), "no design selected") || !strings.Contains(err.Error(), "halibut/halibut") {
		t.Fatalf("no selection = %v", err)
	}

	if _, err := h.lookup("halibut/zzz"); err == nil || !strings.Contains(err.Error(), "available:") {
		t.Fatalf("miss without candidates = %v", err)
	}

	// A collided base name is ambiguous: both designs are offered, none picked,
	// and the hint repeats the candidates.
	if _, err := h.lookup("woments/woments"); err == nil ||
		!strings.Contains(err.Error(), "woments/woments.deck") ||
		!strings.Contains(err.Error(), "woments/woments.fig") ||
		!strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("ambiguous collision = %v", err)
	}

	if _, _, err := h.useFigma(context.Background(), callTool("use_figma"), useInput{Name: "woments/marketing"}); err != nil {
		t.Fatalf("use_figma = %v", err)
	}
	if got, err := h.lookup(""); err != nil || got != marketing {
		t.Fatalf("lookup after selection = %q, %v", got, err)
	}
	if got, err := h.lookup("halibut/halibut"); err != nil || got != halibut {
		t.Fatalf("explicit file over active = %q, %v", got, err)
	}
}

// TestProject: every response can name the design that answered, and a miss
// points at the other projects only when there are any.
func TestProject(t *testing.T) {
	root := designs(t)
	_, h := newServer("", root, nil)
	defer h.close()

	if got := h.project("woments/marketing"); got != "woments/marketing" {
		t.Fatalf("project = %q", got)
	}
	if got := h.project(""); got != "" {
		t.Fatalf("project without selection = %q, want empty", got)
	}
	if _, _, err := h.useFigma(context.Background(), callTool("use_figma"), useInput{Name: "marketing"}); err != nil {
		t.Fatal(err)
	}
	if got := h.project(""); got != "woments/marketing" {
		t.Fatalf("project after selection = %q", got)
	}

	hint := h.elsewhere("")
	for _, want := range []string{"no matches in woments/marketing", "halibut/halibut", "use_figma"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("elsewhere = %q, want it to mention %q", hint, want)
		}
	}

	single := t.TempDir()
	if err := os.WriteFile(filepath.Join(single, "only.fig"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, one := newServer("", single, nil)
	defer one.close()
	if got := one.elsewhere(""); got != "" {
		t.Fatalf("elsewhere with one project = %q, want empty", got)
	}

	_, fig := newServer(filepath.Join(root, "halibut", "halibut.fig"), "", nil)
	defer fig.close()
	if got := fig.project(""); got != "halibut" {
		t.Fatalf("figma mode project = %q, want halibut", got)
	}
	if got := fig.elsewhere(""); got != "" {
		t.Fatalf("figma mode elsewhere = %q, want empty", got)
	}
}

// TestLookupSingle: a directory holding one design needs no selection.
func TestLookupSingle(t *testing.T) {
	dir := t.TempDir()
	only := filepath.Join(dir, "only.fig")
	if err := os.WriteFile(only, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, h := newServer("", dir, nil)
	defer h.close()

	if got, err := h.lookup(""); err != nil || got != only {
		t.Fatalf("single design without selection = %q, %v", got, err)
	}
	if got, err := h.lookup("only"); err != nil || got != only {
		t.Fatalf("single design by identity = %q, %v", got, err)
	}
}

// TestListFigs pins catalog listing: name filtering, limit truncation and the
// rule that only the active project exposes its absolute path.
func TestListFigs(t *testing.T) {
	_, h := newServer("", designs(t), nil)
	defer h.close()

	_, out, err := h.listFigs(context.Background(), callTool("list_figs"), figsInput{Name: "woments", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 || len(out.Projects) != 2 || !out.Truncated {
		t.Fatalf("filtered catalog = %+v, want total 3, 2 shown, truncated", out)
	}
	for _, p := range out.Projects {
		if !strings.Contains(p.Name, "woments") || p.Path != "" {
			t.Fatalf("unfiltered or leaking project = %+v", p)
		}
	}

	_, out, err = h.listFigs(context.Background(), callTool("list_figs"), figsInput{Name: "halibut"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || len(out.Projects) != 1 || out.Truncated {
		t.Fatalf("single match = %+v", out)
	}

	if _, _, err := h.useFigma(context.Background(), callTool("use_figma"), useInput{Name: "woments/marketing"}); err != nil {
		t.Fatalf("use_figma = %v", err)
	}
	_, out, err = h.listFigs(context.Background(), callTool("list_figs"), figsInput{Name: "woments", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if out.Active != "woments/marketing" {
		t.Fatalf("active = %q", out.Active)
	}
	for _, p := range out.Projects {
		want := ""
		if p.Name == out.Active {
			want = filepath.Join(h.root, "woments", "marketing.fig")
		}
		if p.Path != want {
			t.Fatalf("path for %s = %q, want %q", p.Name, p.Path, want)
		}
	}
}

// TestCatalogRefresh: the tree is scanned once per session and only rescanned
// when list_figs asks for it, so listings do not walk the tree on every call.
func TestCatalogRefresh(t *testing.T) {
	root := t.TempDir()
	only := filepath.Join(root, "one.fig")
	if err := os.WriteFile(only, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, h := newServer("", root, nil)
	defer h.close()

	_, out, err := h.listFigs(context.Background(), callTool("list_figs"), figsInput{})
	if err != nil || out.Total != 1 {
		t.Fatalf("initial catalog = %+v, %v", out, err)
	}
	added := filepath.Join(root, "two.fig")
	if err := os.WriteFile(added, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, out, err = h.listFigs(context.Background(), callTool("list_figs"), figsInput{})
	if err != nil || out.Total != 1 {
		t.Fatalf("catalog refreshed without asking = %+v, %v", out, err)
	}
	_, out, err = h.listFigs(context.Background(), callTool("list_figs"), figsInput{Refresh: true})
	if err != nil || out.Total != 2 {
		t.Fatalf("explicit refresh = %+v, %v", out, err)
	}
	_, out, err = h.listFigs(context.Background(), callTool("list_figs"), figsInput{})
	if err != nil || out.Total != 2 {
		t.Fatalf("catalog after refresh = %+v, %v", out, err)
	}
}

// TestLookupEmpty: a directory without designs explains how to get one.
func TestLookupEmpty(t *testing.T) {
	_, h := newServer("", t.TempDir(), nil)
	defer h.close()

	if _, err := h.lookup(""); err == nil || !strings.Contains(err.Error(), "no designs under") {
		t.Fatalf("empty root = %v", err)
	}
}
