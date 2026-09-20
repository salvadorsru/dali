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
