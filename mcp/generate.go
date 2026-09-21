package mcp

import (
	"context"
	"errors"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/codegen"
	"github.com/salvadorsru/dali/codegen/ir"
)

type generateInput struct {
	File      string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Component string `json:"component,omitempty" jsonschema:"component id to generate, e.g. 1:225; omit to generate every component"`
	Tokens    string `json:"tokens,omitempty" jsonschema:"token format to include: css, style-dict or figma; empty omits tokens"`
}

type generatedFile struct {
	Path string `json:"path" jsonschema:"file name, e.g. Button.html"`
	Code string `json:"code" jsonschema:"file content"`
}

type generateOutput struct {
	Files []generatedFile `json:"files"`
}

// generateFigma renders components as HTML+Tailwind and optional design
// tokens, returned inline so the caller can write them into its own project.
func (h *handlers) generateFigma(ctx context.Context, req *mcp.CallToolRequest, in generateInput) (*mcp.CallToolResult, generateOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, generateOutput{}, err
	}

	config := ir.DefaultConfig()
	config.GenerateIndex = false

	var ids []string
	if in.Component != "" {
		ids = []string{in.Component}
	} else {
		components, err := svc.Components()
		if err != nil {
			return nil, generateOutput{}, err
		}
		for _, c := range components {
			if c.Type.IsComponent() {
				ids = append(ids, c.ID)
			}
		}
	}
	if len(ids) == 0 {
		return nil, generateOutput{}, errors.New("no components in document")
	}

	contents, err := codegen.Generate(svc, ids, in.Tokens, config)
	if err != nil {
		return nil, generateOutput{}, err
	}

	files := make([]generatedFile, 0, len(contents))
	for rel, code := range contents {
		files = append(files, generatedFile{Path: filepath.Base(rel), Code: code})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return nil, generateOutput{Files: files}, nil
}
