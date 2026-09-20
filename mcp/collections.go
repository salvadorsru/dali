package mcp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
)

type componentsOutput struct {
	Components []NodeRef `json:"components"`
}

func (h *handlers) findComponents(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, componentsOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, componentsOutput{}, err
	}
	nodes, err := svc.Components()
	if err != nil {
		return nil, componentsOutput{}, err
	}
	if in.Name != "" {
		nodes = filterByName(nodes, in.Name)
	}
	return nil, componentsOutput{Components: toRefs(nodes)}, nil
}

func (h *handlers) findInstances(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, componentsOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, componentsOutput{}, err
	}
	nodes, err := svc.Instances()
	if err != nil {
		return nil, componentsOutput{}, err
	}
	if in.Name != "" {
		nodes = filterByName(nodes, in.Name)
	}
	return nil, componentsOutput{Components: toRefs(nodes)}, nil
}

type assetsOutput struct {
	Assets []model.ImageRef `json:"assets"`
}

func (h *handlers) findAssets(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, assetsOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, assetsOutput{}, err
	}
	assets, err := svc.Assets()
	if err != nil {
		return nil, assetsOutput{}, err
	}
	if in.Name != "" {
		var filtered []model.ImageRef
		for _, a := range assets {
			if containsFold(a.Name, in.Name) {
				filtered = append(filtered, a)
			}
		}
		assets = filtered
	}
	return nil, assetsOutput{Assets: assets}, nil
}

type stylesOutput struct {
	Styles []model.StyleDef `json:"styles"`
}

func (h *handlers) getStyles(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, stylesOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, stylesOutput{}, err
	}
	defs, err := svc.StyleDefs()
	if err != nil {
		return nil, stylesOutput{}, err
	}
	if in.Name != "" {
		var filtered []model.StyleDef
		for _, d := range defs {
			if containsFold(d.Name, in.Name) {
				filtered = append(filtered, d)
			}
		}
		defs = filtered
	}
	return nil, stylesOutput{Styles: defs}, nil
}

type variablesOutput struct {
	Variables []model.Variable `json:"variables"`
}

func (h *handlers) variables(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, variablesOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, variablesOutput{}, err
	}
	vars, err := svc.Variables()
	if err != nil {
		return nil, variablesOutput{}, err
	}
	if in.Name != "" {
		var filtered []model.Variable
		for _, v := range vars {
			if containsFold(v.Name, in.Name) || containsFold(v.Set, in.Name) {
				filtered = append(filtered, v)
			}
		}
		vars = filtered
	}
	if vars == nil {
		vars = []model.Variable{}
	}
	return nil, variablesOutput{Variables: vars}, nil
}

type fontsOutput struct {
	Fonts []model.Font `json:"fonts"`
}

func (h *handlers) getFonts(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, fontsOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, fontsOutput{}, err
	}
	fonts, err := svc.Fonts()
	if err != nil {
		return nil, fontsOutput{}, err
	}
	if in.Name != "" {
		var filtered []model.Font
		for _, f := range fonts {
			if containsFold(f.Family, in.Name) {
				filtered = append(filtered, f)
			}
		}
		fonts = filtered
	}
	if fonts == nil {
		fonts = []model.Font{}
	}
	return nil, fontsOutput{Fonts: fonts}, nil
}

type pagesOutput struct {
	Pages []NodeRef `json:"pages"`
}

func (h *handlers) listPages(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, pagesOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, pagesOutput{}, err
	}
	pages, err := svc.Pages()
	if err != nil {
		return nil, pagesOutput{}, err
	}
	if in.Name != "" {
		pages = filterByName(pages, in.Name)
	}
	if pages == nil {
		pages = []*model.Node{}
	}
	return nil, pagesOutput{Pages: toRefs(pages)}, nil
}

type statsOutput struct {
	File     string         `json:"file"`
	Document string         `json:"document"`
	Pages    int            `json:"pages"`
	Nodes    int            `json:"nodes"`
	Counts   map[string]int `json:"counts"`
}

func (h *handlers) getStats(ctx context.Context, req *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, statsOutput, error) {
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, statsOutput{}, err
	}
	abs, err := h.resolve(in.File)
	if err != nil {
		return nil, statsOutput{}, err
	}
	doc, err := svc.Document()
	if err != nil {
		return nil, statsOutput{}, err
	}
	counts, err := svc.Counts()
	if err != nil {
		return nil, statsOutput{}, err
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	pages, err := svc.Pages()
	if err != nil {
		return nil, statsOutput{}, err
	}
	if counts == nil {
		counts = map[string]int{}
	}
	return nil, statsOutput{
		File:     filepath.Base(abs),
		Document: doc,
		Pages:    len(pages),
		Nodes:    total,
		Counts:   counts,
	}, nil
}

type queryInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query string `json:"query" jsonschema:"structural expression, e.g. type=FRAME, name~\"card\", children(\"Header\")"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 50"`
}

func (h *handlers) queryFigma(ctx context.Context, req *mcp.CallToolRequest, in queryInput) (*mcp.CallToolResult, componentsOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, componentsOutput{}, err
	}
	nodes, err := svc.Query(in.Query)
	if err != nil {
		return nil, componentsOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	return nil, componentsOutput{Components: toRefs(nodes)}, nil
}

func filterByName(nodes []*model.Node, name string) []*model.Node {
	var out []*model.Node
	for _, n := range nodes {
		if containsFold(n.Name, name) {
			out = append(out, n)
		}
	}
	return out
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
