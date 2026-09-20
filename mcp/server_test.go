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
