package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestResponseProject: every answer names the design that produced it, so an
// agent can notice it is on the wrong project instead of trusting the result.
func TestResponseProject(t *testing.T) {
	root := t.TempDir()
	emptyFig(t, filepath.Join(root, "woments", "woments.fig"))
	emptyFig(t, filepath.Join(root, "halibut", "halibut.fig"))
	_, h := newServer("", root, nil)
	defer h.close()

	ctx := context.Background()
	if _, _, err := h.useFigma(ctx, callTool("use_figma"), useInput{Name: "woments"}); err != nil {
		t.Fatalf("use_figma: %v", err)
	}

	_, stats, err := h.getStats(ctx, callTool("get_stats"), filterInput{})
	if err != nil {
		t.Fatalf("get_stats: %v", err)
	}
	if stats.Project != "woments/woments" {
		t.Fatalf("stats project = %q, want woments/woments", stats.Project)
	}

	_, search, err := h.search(ctx, callTool("search_figma"), searchInput{Query: "zzzznothing"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if search.Project != "woments/woments" {
		t.Fatalf("search project = %q, want woments/woments", search.Project)
	}
	for _, want := range []string{"no matches in woments/woments", "halibut/halibut", "use_figma"} {
		if !strings.Contains(search.Hint, want) {
			t.Fatalf("search hint = %q, want it to mention %q", search.Hint, want)
		}
	}

	_, query, err := h.queryFigma(ctx, callTool("query_figma"), queryInput{Query: `name~"zzzznothing"`})
	if err != nil {
		t.Fatalf("query_figma: %v", err)
	}
	if query.Project != "woments/woments" || !strings.Contains(query.Hint, "halibut/halibut") {
		t.Fatalf("query_figma = %+v, want project and hint for the other project", query)
	}

	// A partial name switches projects in one call.
	if _, _, err := h.useFigma(ctx, callTool("use_figma"), useInput{Name: "halibut"}); err != nil {
		t.Fatalf("use_figma partial: %v", err)
	}
	_, stats, err = h.getStats(ctx, callTool("get_stats"), filterInput{})
	if err != nil {
		t.Fatalf("get_stats after switch: %v", err)
	}
	if stats.Project != "halibut/halibut" {
		t.Fatalf("stats project after switch = %q, want halibut/halibut", stats.Project)
	}
}

// TestFigmaModeProject: a mounted design still reports its identity, and never
// points at projects it cannot serve.
func TestFigmaModeProject(t *testing.T) {
	root := t.TempDir()
	fig := emptyFig(t, filepath.Join(root, "woments", "woments.fig"))
	_, h := newServer(fig, "", nil)
	defer h.close()

	ctx := context.Background()
	_, stats, err := h.getStats(ctx, callTool("get_stats"), filterInput{})
	if err != nil {
		t.Fatalf("get_stats: %v", err)
	}
	if stats.Project != "woments" {
		t.Fatalf("figma mode project = %q, want woments", stats.Project)
	}
	if stats.File != "woments.fig" {
		t.Fatalf("figma mode file = %q, want woments.fig", stats.File)
	}

	_, search, err := h.search(ctx, callTool("search_figma"), searchInput{Query: "zzzznothing"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if search.Hint != "" {
		t.Fatalf("figma mode hint = %q, want empty: there is nowhere else to look", search.Hint)
	}
}
