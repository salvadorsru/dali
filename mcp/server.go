package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
)

type handlers struct {
	svc *query.Service
}

type NodeRef struct {
	ID   string `json:"id" jsonschema:"node id, e.g. 123:456"`
	Type string `json:"type" jsonschema:"node type, e.g. FRAME, TEXT, SYMBOL"`
	Name string `json:"name" jsonschema:"node name"`
	Text string `json:"text,omitempty" jsonschema:"text content for TEXT nodes"`
	Page string `json:"page,omitempty" jsonschema:"page name"`
	Path string `json:"path,omitempty" jsonschema:"path from page to node"`
}

func toRef(n *model.Node) NodeRef {
	return NodeRef{ID: n.ID, Type: string(n.Type), Name: n.Name, Text: n.Text, Page: n.PageID}
}

func toRefs(nodes []*model.Node) []NodeRef {
	out := make([]NodeRef, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toRef(n))
	}
	return out
}

func Serve(ctx context.Context, svc *query.Service) error {
	return newServer(svc).Run(ctx, &mcp.StdioTransport{})
}

func newServer(svc *query.Service) *mcp.Server {
	h := &handlers{svc: svc}
	s := mcp.NewServer(&mcp.Implementation{Name: "dali", Version: "0.1.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "search_figma", Description: "Full-text search across node names and text content."}, h.search)
	mcp.AddTool(s, &mcp.Tool{Name: "get_node", Description: "Get a single node by id."}, h.getNode)
	mcp.AddTool(s, &mcp.Tool{Name: "get_context", Description: "Get structural context for a node: path, parent, children, component, styles, assets."}, h.getContext)
	mcp.AddTool(s, &mcp.Tool{Name: "get_children", Description: "List the direct children of a node."}, h.getChildren)
	mcp.AddTool(s, &mcp.Tool{Name: "get_parent", Description: "Get the parent of a node."}, h.getParent)
	mcp.AddTool(s, &mcp.Tool{Name: "find_components", Description: "List components, optionally filtered by name substring."}, h.findComponents)
	mcp.AddTool(s, &mcp.Tool{Name: "find_assets", Description: "List image assets, optionally filtered by name substring."}, h.findAssets)
	mcp.AddTool(s, &mcp.Tool{Name: "find_text", Description: "Search text content of TEXT nodes."}, h.findText)

	return s
}

type searchInput struct {
	Query string `json:"query" jsonschema:"search text"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
}

type searchOutput struct {
	Results []NodeRef `json:"results"`
}

func (h *handlers) search(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := h.svc.Search(in.Query, limit)
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{Results: make([]NodeRef, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, NodeRef{ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, Path: r.Path})
	}
	return nil, out, nil
}

type idInput struct {
	ID string `json:"id" jsonschema:"node id, e.g. 123:456"`
}

func (h *handlers) getNode(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeRef, error) {
	n, err := h.svc.Node(in.ID)
	if err != nil || n == nil {
		return nil, NodeRef{}, err
	}
	return nil, toRef(n), nil
}

func (h *handlers) getContext(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, ContextView, error) {
	ctx, err := h.svc.Context(in.ID)
	if err != nil || ctx == nil || ctx.Node == nil {
		return nil, ContextView{}, err
	}
	view := ContextView{
		Node:     toRef(ctx.Node),
		Children: toRefs(ctx.Children),
		Styles:   ctx.Styles,
		Assets:   ctx.Assets,
	}
	for _, p := range ctx.Path {
		view.Path = append(view.Path, toRef(p))
	}
	if ctx.Parent != nil {
		ref := toRef(ctx.Parent)
		view.Parent = &ref
	}
	if ctx.Component != nil {
		ref := toRef(ctx.Component)
		view.Component = &ref
	}
	return nil, view, nil
}

// ContextView is a flat, acyclic view of query.Context for MCP schema generation.
type ContextView struct {
	Node      NodeRef          `json:"node"`
	Path      []NodeRef        `json:"path"`
	Parent    *NodeRef         `json:"parent,omitempty"`
	Children  []NodeRef        `json:"children"`
	Component *NodeRef         `json:"component,omitempty"`
	Styles    []model.Style    `json:"styles"`
	Assets    []model.ImageRef `json:"assets"`
}

type childrenOutput struct {
	Children []NodeRef `json:"children"`
}

func (h *handlers) getChildren(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, childrenOutput, error) {
	nodes, err := h.svc.Children(in.ID)
	if err != nil {
		return nil, childrenOutput{}, err
	}
	return nil, childrenOutput{Children: toRefs(nodes)}, nil
}

func (h *handlers) getParent(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeRef, error) {
	n, err := h.svc.Parent(in.ID)
	if err != nil || n == nil {
		return nil, NodeRef{}, err
	}
	return nil, toRef(n), nil
}

type filterInput struct {
	Name string `json:"name,omitempty" jsonschema:"optional name substring filter"`
}

type componentsOutput struct {
	Components []NodeRef `json:"components"`
}

func (h *handlers) findComponents(_ context.Context, _ *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, componentsOutput, error) {
	nodes, err := h.svc.Components()
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

func (h *handlers) findAssets(_ context.Context, _ *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, assetsOutput, error) {
	assets, err := h.svc.Assets()
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

func (h *handlers) findText(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := h.svc.Search(in.Query, limit*4)
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{Results: make([]NodeRef, 0, limit)}
	for _, r := range results {
		if r.Type != string(model.TypeText) {
			continue
		}
		out.Results = append(out.Results, NodeRef{ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, Path: r.Path})
		if len(out.Results) >= limit {
			break
		}
	}
	return nil, out, nil
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
