package query

import (
	"context"
	"os"
	"strings"
	"testing"
)

const sample = "../woments.fig"

func TestServiceIntegration(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	svc, err := Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	results, err := svc.Search("menopausia", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("search returned no results")
	}

	n, err := svc.Node(results[0].ID)
	if err != nil || n == nil {
		t.Fatalf("node %q: %v", results[0].ID, err)
	}
	path, err := svc.Path(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(path) < 2 || path[0].Type != "DOCUMENT" {
		t.Fatalf("path = %+v", path)
	}

	pages, err := svc.Pages()
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages = %+v, err %v", pages, err)
	}

	counts, err := svc.Counts()
	if err != nil || counts["TEXT"] == 0 {
		t.Fatalf("counts = %+v, err %v", counts, err)
	}

	comps, err := svc.Components()
	if err != nil || len(comps) == 0 {
		t.Fatalf("components = %d, err %v", len(comps), err)
	}

	ctx, err := svc.Context(comps[0].ID)
	if err != nil || ctx == nil || ctx.Node == nil {
		t.Fatalf("context: %+v, err %v", ctx, err)
	}

	// UnifiedSearch with a type filter must not hit the structural branch's
	// former "no such column: node_id" bug.
	unified, err := svc.UnifiedSearch(context.Background(), nil, UnifiedOptions{
		Query: "añadir", Type: "TEXT", Limit: 5, Mode: ModeHybrid,
	})
	if err != nil {
		t.Fatalf("unified search: %v", err)
	}
	for _, r := range unified {
		if r.Type != "TEXT" {
			t.Fatalf("unified result type = %q", r.Type)
		}
	}
}

func TestQueryExpressions(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	svc, err := Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	nodes, err := svc.Query("type=TEXT")
	if err != nil || len(nodes) == 0 {
		t.Fatalf("type=TEXT: %d, err %v", len(nodes), err)
	}
	nodes, err = svc.Query(`name~"Button"`)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if !strings.Contains(strings.ToLower(n.Name), "button") {
			t.Fatalf("name~ matched %q", n.Name)
		}
	}
}
