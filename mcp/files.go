package mcp

import (
	"context"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type figsOutput struct {
	Files []string `json:"files"`
}

func (h *handlers) listFigs(_ context.Context, _ *mcp.CallToolRequest, _ filterInput) (*mcp.CallToolResult, figsOutput, error) {
	files := discover()
	if files == nil {
		files = []string{}
	}
	return nil, figsOutput{Files: files}, nil
}

func discover() []string {
	var out []string
	for _, pattern := range []string{"*.fig", "*.deck"} {
		if found, err := filepath.Glob(pattern); err == nil {
			out = append(out, found...)
		}
	}
	return out
}
