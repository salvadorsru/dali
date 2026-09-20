package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const sample = "../woments.fig"

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
		t.Fatalf("search_figma returned no content: %+v", res)
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
