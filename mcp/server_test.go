package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
)

const sample = "../woments.fig"

func textOf(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestTools(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(sample)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_figma",
		Arguments: map[string]any{"query": "menopausia", "limit": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("search_figma returned no content: %+v body=%q", res, textOf(res))
	}
	var search searchOutput
	if err := json.Unmarshal([]byte(textOf(res)), &search); err != nil {
		t.Fatalf("search_figma parse: %v", err)
	}
	if len(search.Results) == 0 || search.Results[0].ID == "" {
		t.Fatalf("search_figma results = %+v", search)
	}
	nodeID := search.Results[0].ID

	// get_descendants returns the full subtree with geometry, styles and images.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_descendants",
		Arguments: map[string]any{"id": nodeID},
	})
	if err != nil || res.IsError {
		t.Fatalf("get_descendants: %v %+v body=%q", err, res, textOf(res))
	}
	var tree SubtreeResult
	if err := json.Unmarshal([]byte(textOf(res)), &tree); err != nil {
		t.Fatalf("get_descendants parse: %v", err)
	}
	if tree.Root.ID != nodeID {
		t.Fatalf("tree root = %+v", tree.Root)
	}
	if len(tree.Descendants) > 0 {
		if n := tree.Descendants[0].Node; n.Width == 0 && n.Height == 0 {
			t.Fatalf("descendant missing geometry: %+v", n)
		}
	}

	// An explicit file argument overrides the default fig.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "find_components",
		Arguments: map[string]any{"file": sample},
	})
	if err != nil || res.IsError {
		t.Fatalf("find_components: %v %+v", err, res)
	}
}

// TestNewTools exercises the additive tools that need no embedder.
func TestNewTools(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(sample)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	call := func(name string, args map[string]any) string {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError || len(res.Content) == 0 {
			t.Fatalf("%s returned no content: %+v body=%q", name, res, textOf(res))
		}
		return textOf(res)
	}

	body := call("get_stats", nil)
	var stats statsOutput
	if err := json.Unmarshal([]byte(body), &stats); err != nil {
		t.Fatalf("get_stats parse: %v", err)
	}
	if stats.Document == "" || stats.Nodes == 0 || stats.Pages == 0 {
		t.Fatalf("get_stats = %+v", stats)
	}

	body = call("list_pages", nil)
	var pages pagesOutput
	if err := json.Unmarshal([]byte(body), &pages); err != nil {
		t.Fatalf("list_pages parse: %v", err)
	}
	if len(pages.Pages) == 0 || pages.Pages[0].ID == "" {
		t.Fatalf("list_pages = %+v", pages)
	}

	body = call("get_fonts", nil)
	var fonts fontsOutput
	if err := json.Unmarshal([]byte(body), &fonts); err != nil {
		t.Fatalf("get_fonts parse: %v", err)
	}
	if len(fonts.Fonts) == 0 {
		t.Fatalf("get_fonts empty")
	}

	body = call("find_instances", nil)
	var inst componentsOutput
	if err := json.Unmarshal([]byte(body), &inst); err != nil {
		t.Fatalf("find_instances parse: %v", err)
	}

	body = call("query_figma", map[string]any{"query": `type="FRAME"`, "limit": 5})
	var q componentsOutput
	if err := json.Unmarshal([]byte(body), &q); err != nil {
		t.Fatalf("query_figma parse: %v", err)
	}
	if len(q.Components) == 0 {
		t.Fatalf("query_figma empty")
	}

	body = call("find_assets", nil)
	var assets assetsOutput
	if err := json.Unmarshal([]byte(body), &assets); err != nil {
		t.Fatalf("find_assets parse: %v", err)
	}
	if len(assets.Assets) > 0 {
		body = call("get_image", map[string]any{"hash": assets.Assets[0].Hash})
		var img imageOutput
		if err := json.Unmarshal([]byte(body), &img); err != nil {
			t.Fatalf("get_image parse: %v", err)
		}
		if img.Data == "" || img.MIME == "" {
			t.Fatalf("get_image = %+v", img)
		}
	}
}

// TestExtractAssets writes images to disk and reports file paths instead of
// base64, without touching the repo's own extracted assets.
func TestExtractAssets(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	tmp := t.TempDir()
	src, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(tmp, "extract.fig")
	if err := os.WriteFile(cp, src, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(cp)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "extract_assets",
		Arguments: map[string]any{"name": "menobalance"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("extract_assets returned no content: %+v body=%q", res, textOf(res))
	}
	var out extractOutput
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("extract_assets parse: %v", err)
	}
	if len(out.Assets) == 0 {
		t.Fatalf("extract_assets assets empty: %+v", out)
	}
	for _, a := range out.Assets {
		if a.Path == "" || a.MIME == "" || a.Width == 0 || a.Height == 0 {
			t.Fatalf("extract_assets asset not enriched: %+v", a)
		}
		if _, err := os.Stat(a.Path); err != nil {
			t.Fatalf("asset file missing: %s: %v", a.Path, err)
		}
	}

	// A hash filter extracts only that single asset.
	one := out.Assets[0].Hash
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "extract_assets",
		Arguments: map[string]any{"hash": one},
	})
	if err != nil || res.IsError {
		t.Fatalf("extract_assets hash filter: %v %+v", err, res)
	}
	var single extractOutput
	if err := json.Unmarshal([]byte(textOf(res)), &single); err != nil {
		t.Fatalf("extract_assets hash parse: %v", err)
	}
	if len(single.Assets) != 1 || single.Assets[0].Hash != one {
		t.Fatalf("extract_assets hash filter = %+v", single)
	}
}

// TestTokens exercises the design-token and component-contract tools.
func TestTokens(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(sample)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	call := func(name string, args map[string]any) string {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError || len(res.Content) == 0 {
			t.Fatalf("%s returned no content: %+v body=%q", name, res, textOf(res))
		}
		return textOf(res)
	}

	body := call("get_tokens", map[string]any{"kind": "color"})
	var toks tokensOutput
	if err := json.Unmarshal([]byte(body), &toks); err != nil {
		t.Fatalf("get_tokens parse: %v", err)
	}
	if len(toks.Colors) == 0 || toks.Colors[0].Usage == 0 || len(toks.ColorVars) == 0 {
		t.Fatalf("get_tokens color = %+v", toks)
	}

	body = call("get_styles", map[string]any{"kind": "LAYOUT", "details": false})
	var styles stylesOutput
	if err := json.Unmarshal([]byte(body), &styles); err != nil {
		t.Fatalf("get_styles parse: %v", err)
	}
	if len(styles.Styles) == 0 {
		t.Fatalf("get_styles LAYOUT empty")
	}
	for _, d := range styles.Styles {
		if d.Kind != "LAYOUT" {
			t.Fatalf("kind filter leaked %q", d.Kind)
		}
		if d.Raw != "" {
			t.Fatalf("details=false kept raw for %s", d.Key)
		}
	}

	// Layout is surfaced on node views.
	body = call("find_components", map[string]any{"name": ""})
	var comps componentsOutput
	if err := json.Unmarshal([]byte(body), &comps); err != nil {
		t.Fatalf("find_components parse: %v", err)
	}
	if len(comps.Components) == 0 {
		t.Fatalf("find_components empty")
	}
	cid := comps.Components[0].ID

	body = call("get_node", map[string]any{"id": cid})
	var nv NodeView
	if err := json.Unmarshal([]byte(body), &nv); err != nil {
		t.Fatalf("get_node parse: %v", err)
	}
	if nv.Node.ID != cid {
		t.Fatalf("get_node = %+v", nv.Node)
	}

	// Placements and component contract.
	body = call("get_usages", map[string]any{"id": cid})
	var uses usagesOutput
	if err := json.Unmarshal([]byte(body), &uses); err != nil {
		t.Fatalf("get_usages parse: %v", err)
	}
	if uses.Component.ID != cid {
		t.Fatalf("get_usages component = %+v", uses.Component)
	}

	body = call("get_dependents", map[string]any{"id": cid})
	var deps dependentsOutput
	if err := json.Unmarshal([]byte(body), &deps); err != nil {
		t.Fatalf("get_dependents parse: %v", err)
	}
	if deps.Component.ID != cid {
		t.Fatalf("get_dependents component = %+v", deps.Component)
	}

	body = call("get_component", map[string]any{"id": cid})
	var comp componentOutput
	if err := json.Unmarshal([]byte(body), &comp); err != nil {
		t.Fatalf("get_component parse: %v", err)
	}
	if comp.Node.ID != cid || comp.Type == "" {
		t.Fatalf("get_component = %+v", comp)
	}
}

// TestGenerate renders a component as HTML+Tailwind plus tokens, returned
// inline for the caller to write into its own project.
func TestGenerate(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(sample)
	defer h.close()
	go srv.Run(ctx, st)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "find_components",
		Arguments: map[string]any{"name": ""},
	})
	if err != nil || res.IsError {
		t.Fatalf("find_components: %v %+v", err, res)
	}
	var comps componentsOutput
	if err := json.Unmarshal([]byte(textOf(res)), &comps); err != nil {
		t.Fatalf("find_components parse: %v", err)
	}
	if len(comps.Components) == 0 {
		t.Fatal("no components in sample")
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "generate_figma",
		Arguments: map[string]any{
			"component": comps.Components[0].ID,
			"tokens":    "css",
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("generate_figma: %v %+v body=%q", err, res, textOf(res))
	}
	var out generateOutput
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("generate_figma parse: %v", err)
	}
	if len(out.Files) < 2 { // component html + tokens.css
		t.Fatalf("generate_figma files = %+v", out.Files)
	}
	var componentHTML bool
	for _, f := range out.Files {
		if f.Code == "" {
			t.Fatalf("empty code for %s", f.Path)
		}
		if filepath.Ext(f.Path) == ".html" {
			componentHTML = true
		}
	}
	if !componentHTML {
		t.Fatalf("no html component generated: %+v", out.Files)
	}
}

// TestLayoutOf verifies layout extraction is case-insensitive and picks the
// LAYOUT style summary.
func TestLayoutOf(t *testing.T) {
	got := layoutOf([]model.Style{
		{Kind: "FILL", Summary: "#4A4840"},
		{Kind: "layout", Summary: "layout vertical · gap 12"},
		{Kind: "LAYOUT", Summary: "layout horizontal · gap 10"},
	})
	if got != "layout vertical · gap 12" {
		t.Fatalf("layoutOf = %q", got)
	}
	if layoutOf(nil) != "" {
		t.Fatal("layoutOf(nil) not empty")
	}
}

// TestIndexNotification verifies the server reports structural indexing via
// progress notifications when the client supplies a progress token.
func TestIndexNotification(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	tmp := t.TempDir()
	src, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(tmp, "notice.fig")
	if err := os.WriteFile(cp, src, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv, h := newServer(cp)
	defer h.close()
	go srv.Run(ctx, st)

	var mu sync.Mutex
	var notes []string
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, n *mcp.ProgressNotificationClientRequest) {
			if n.Params.ProgressToken == "tok-1" {
				mu.Lock()
				notes = append(notes, n.Params.Message)
				mu.Unlock()
			}
		},
	})
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_figma",
		Arguments: map[string]any{"query": "menopausia", "limit": 3},
		Meta:      mcp.Meta{"progressToken": "tok-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("search_figma returned no content: %+v body=%q", res, textOf(res))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(notes)
		mu.Unlock()
		if done >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notes) != 2 {
		t.Fatalf("expected 2 progress notifications, got %v", notes)
	}
	if notes[0] != "indexing notice.fig" || notes[1] != "indexed notice.fig" {
		t.Fatalf("unexpected notifications: %v", notes)
	}
}
