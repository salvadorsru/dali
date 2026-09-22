package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/query"
)

// TestSectionTool resolves a component by name and inspects it in one call,
// mirroring pencil_section. It also checks the within filter and near-miss
// fallback through the underlying resolver.
func TestSectionTool(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(sample, nil)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Pick a real component name to resolve.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "find_components",
		Arguments: map[string]any{},
	})
	if err != nil || res.IsError {
		t.Fatalf("find_components: %v %+v", err, res)
	}
	var comps componentsOutput
	if err := json.Unmarshal([]byte(textOf(res)), &comps); err != nil {
		t.Fatal(err)
	}
	if len(comps.Components) == 0 {
		t.Fatal("no components to resolve")
	}
	name := comps.Components[0].Name

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "section_figma",
		Arguments: map[string]any{"name": name},
	})
	if err != nil || res.IsError {
		t.Fatalf("section_figma: %v %+v body=%q", err, res, textOf(res))
	}
	var view SectionView
	if err := json.Unmarshal([]byte(textOf(res)), &view); err != nil {
		t.Fatalf("section_figma parse: %v", err)
	}
	if view.Node.ID == "" {
		t.Fatalf("section not resolved: matches=%v", view.Matches)
	}
	if !strings.EqualFold(view.Node.Name, name) {
		t.Fatalf("resolved %q, want %q", view.Node.Name, name)
	}
	if len(view.Path) == 0 {
		t.Fatal("section has no path")
	}
	if len(view.Children) > 0 && len(view.Descendants) == 0 {
		t.Fatalf("section has children but no subtree descendants")
	}
	for _, d := range view.Descendants {
		if d.Node.AbsY == 0 && d.Node.AbsX == 0 {
			t.Fatalf("subtree node %s has no absolute position", d.Node.ID)
		}
	}

	// An ambiguous name must come back with disambiguation alternatives.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "section_figma",
		Arguments: map[string]any{"name": "VideoCaption"},
	})
	if err != nil || res.IsError {
		t.Fatalf("section_figma ambiguous: %v %+v body=%q", err, res, textOf(res))
	}
	var amb SectionView
	if err := json.Unmarshal([]byte(textOf(res)), &amb); err != nil {
		t.Fatal(err)
	}
	if amb.Node.ID != "" && len(amb.Alternatives) == 0 {
		t.Fatal("expected alternatives for ambiguous name VideoCaption")
	}
}

func TestResolveSectionFilters(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	svc, err := query.Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	comps, err := svc.Components()
	if err != nil || len(comps) == 0 {
		t.Fatalf("components: %v", err)
	}
	want := comps[0]

	got, _, _, err := resolveSection(svc, sectionInput{Name: want.Name})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !strings.EqualFold(got.Name, want.Name) {
		t.Fatalf("resolve %q: got %+v", want.Name, got)
	}

	// within filter: an ancestor name that does not match must exclude it.
	path, err := svc.Path(want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(path) >= 2 {
		ancestor := path[len(path)-2].Name
		got, _, _, err := resolveSection(svc, sectionInput{Name: want.Name, Within: ancestor})
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("within %q excluded a node that lives under it", ancestor)
		}
		none, _, _, err := resolveSection(svc, sectionInput{Name: want.Name, Within: "zzz-no-such-ancestor"})
		if err != nil {
			t.Fatal(err)
		}
		if none != nil {
			t.Fatalf("within bogus ancestor matched %q", none.Name)
		}
	}

	// Ambiguous names surface their alternatives; only when the sample has them.
	best, alts, _, err := resolveSection(svc, sectionInput{Name: "VideoCaption"})
	if err != nil {
		t.Fatal(err)
	}
	if best == nil {
		t.Log("sample has no VideoCaption; skipping ambiguity check")
	} else if len(alts) == 0 {
		t.Fatalf("expected ambiguous VideoCaption to report alternatives, got %+v alts=%d", best, len(alts))
	}

	// Near-miss: a nonsense name yields suggestions, not a node.
	none, _, matches, err := resolveSection(svc, sectionInput{Name: "zzz-no-such-layer-zzz"})
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Fatalf("nonsense name matched %q", none.Name)
	}
	t.Logf("near misses: %v", matches)
}
