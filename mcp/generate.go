package mcp

import (
	"context"
	"errors"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/codegen"
	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
)

type generateInput struct {
	File      string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Component string `json:"component,omitempty" jsonschema:"node id to generate, e.g. 1:225"`
	Name      string `json:"name,omitempty" jsonschema:"node name substring to resolve; omit both to generate every component"`
	Tokens    string `json:"tokens,omitempty" jsonschema:"token format to include: css, style-dict or figma; empty omits tokens"`
}

type generatedFile struct {
	Path string `json:"path" jsonschema:"file name, e.g. Button.html"`
	Code string `json:"code" jsonschema:"file content"`
}

type generateOutput struct {
	Files        []generatedFile `json:"files"`
	Alternatives []NodeRef       `json:"alternatives,omitempty" jsonschema:"other layers matching the name, for disambiguation"`
	Matches      []string        `json:"matches,omitempty" jsonschema:"near-miss names when nothing resolved"`
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
	var alts []*model.Node
	switch {
	case in.Component != "" && in.Name != "":
		return nil, generateOutput{}, errors.New("use either component or name, not both")
	case in.Name != "":
		node, alternatives, matches, err := resolveSection(svc, sectionInput{Name: in.Name})
		if err != nil {
			return nil, generateOutput{}, err
		}
		if node == nil {
			return nil, generateOutput{Matches: matches}, nil
		}
		ids = []string{node.ID}
		alts = alternatives
	case in.Component != "":
		ids = []string{in.Component}
	default:
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

	pages := pageNames(svc)
	refs := make([]NodeRef, 0, len(alts))
	for _, a := range alts {
		ref := toRef(a)
		ref.PageName = pages[a.PageID]
		refs = append(refs, ref)
	}

	return nil, generateOutput{Files: files, Alternatives: refs}, nil
}
