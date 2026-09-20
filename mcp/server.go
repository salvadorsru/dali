package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
	"github.com/salvadorsru/dali/semantic"
)

type handlers struct {
	def string
	emb embed.Embedder

	mu      sync.Mutex
	opened  map[string]*query.Service
	figures []string // known fig files, for discovery
}

type NodeRef struct {
	ID    string  `json:"id" jsonschema:"node id, e.g. 123:456"`
	Type  string  `json:"type" jsonschema:"node type, e.g. FRAME, TEXT, SYMBOL"`
	Name  string  `json:"name" jsonschema:"node name"`
	Text  string  `json:"text,omitempty" jsonschema:"text content for TEXT nodes"`
	Page  string  `json:"page,omitempty" jsonschema:"page name"`
	Path  string  `json:"path,omitempty" jsonschema:"path from page to node"`
	Score float64 `json:"score,omitempty" jsonschema:"similarity score for semantic results"`
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

// Serve runs the MCP server over stdio. def is the default fig file; tool calls
// may override it with their own "file" argument, so one server can serve many
// designs.
func Serve(ctx context.Context, def string) error {
	s, h := newServer(def)
	defer h.close()
	return s.Run(ctx, &mcp.StdioTransport{})
}

func newServer(def string) (*mcp.Server, *handlers) {
	h := &handlers{
		def:     def,
		emb:     embed.FromEnv(),
		opened:  make(map[string]*query.Service),
		figures: discover(),
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "dali", Version: "0.1.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "search_figma", Description: "Full-text search across node names and text content."}, h.search)
	mcp.AddTool(s, &mcp.Tool{Name: "semantic_search_figma", Description: "Semantic (vector) search across nodes. Use for conceptual queries with no exact text match."}, h.semanticSearch)
	mcp.AddTool(s, &mcp.Tool{Name: "get_node", Description: "Get a single node by id."}, h.getNode)
	mcp.AddTool(s, &mcp.Tool{Name: "get_context", Description: "Get structural context for a node: path, parent, children, component, styles, assets."}, h.getContext)
	mcp.AddTool(s, &mcp.Tool{Name: "get_children", Description: "List the direct children of a node."}, h.getChildren)
	mcp.AddTool(s, &mcp.Tool{Name: "get_parent", Description: "Get the parent of a node."}, h.getParent)
	mcp.AddTool(s, &mcp.Tool{Name: "find_components", Description: "List components, optionally filtered by name substring."}, h.findComponents)
	mcp.AddTool(s, &mcp.Tool{Name: "find_assets", Description: "List image assets, optionally filtered by name substring."}, h.findAssets)
	mcp.AddTool(s, &mcp.Tool{Name: "find_text", Description: "Search text content of TEXT nodes."}, h.findText)
	mcp.AddTool(s, &mcp.Tool{Name: "list_figs", Description: "List the .fig/.deck files available in the current directory."}, h.listFigs)

	return s, h
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

// service returns the query service for a fig file, opening and caching it.
func (h *handlers) service(file string) (*query.Service, error) {
	if file == "" {
		file = h.def
	}
	if file == "" {
		if len(h.figures) == 1 {
			file = h.figures[0]
		} else {
			return nil, errors.New("no fig file: pass \"file\" in the tool call, or start dali with a fig")
		}
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if svc, ok := h.opened[abs]; ok {
		return svc, nil
	}
	svc, err := query.Open(abs)
	if err != nil {
		return nil, err
	}
	h.opened[abs] = svc
	return svc, nil
}

func (h *handlers) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, svc := range h.opened {
		svc.Close()
	}
	h.opened = nil
	_ = embed.Close(h.emb)
}

type searchInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query string `json:"query" jsonschema:"search text"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
}

type searchOutput struct {
	Results []NodeRef `json:"results"`
}

func (h *handlers) search(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	svc, err := h.service(in.File)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := svc.Search(in.Query, limit)
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
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	ID   string `json:"id" jsonschema:"node id, e.g. 123:456"`
}

type semanticInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query string `json:"query" jsonschema:"conceptual search text"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
	Type  string `json:"type,omitempty" jsonschema:"filter by node type, e.g. COMPONENT"`
	Page  string `json:"page,omitempty" jsonschema:"filter by page name or id"`
}

func (h *handlers) semanticSearch(ctx context.Context, _ *mcp.CallToolRequest, in semanticInput) (*mcp.CallToolResult, searchOutput, error) {
	if h.emb == nil {
		return nil, searchOutput{}, errors.New("no embedder configured")
	}
	svc, err := h.service(in.File)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if _, err := semantic.Index(ctx, svc.DB(), h.emb); err != nil {
		return nil, searchOutput{}, err
	}
	opts := query.Options{Query: in.Query, Type: in.Type, Limit: limit}
	if in.Page != "" {
		id, err := svc.PageByName(in.Page)
		if err != nil {
			return nil, searchOutput{}, err
		}
		if id == "" {
			return nil, searchOutput{}, fmt.Errorf("page %q not found", in.Page)
		}
		opts.PageID = id
	}
	results, err := svc.SemanticSearch(ctx, h.emb, opts)
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{Results: make([]NodeRef, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, NodeRef{
			ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, Path: r.Path, Score: r.Score,
		})
	}
	return nil, out, nil
}

func (h *handlers) getNode(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeRef, error) {
	svc, err := h.service(in.File)
	if err != nil {
		return nil, NodeRef{}, err
	}
	n, err := svc.Node(in.ID)
	if err != nil || n == nil {
		return nil, NodeRef{}, err
	}
	return nil, toRef(n), nil
}

func (h *handlers) getContext(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, ContextView, error) {
	svc, err := h.service(in.File)
	if err != nil {
		return nil, ContextView{}, err
	}
	ctx, err := svc.Context(in.ID)
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
	svc, err := h.service(in.File)
	if err != nil {
		return nil, childrenOutput{}, err
	}
	nodes, err := svc.Children(in.ID)
	if err != nil {
		return nil, childrenOutput{}, err
	}
	return nil, childrenOutput{Children: toRefs(nodes)}, nil
}

func (h *handlers) getParent(_ context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeRef, error) {
	svc, err := h.service(in.File)
	if err != nil {
		return nil, NodeRef{}, err
	}
	n, err := svc.Parent(in.ID)
	if err != nil || n == nil {
		return nil, NodeRef{}, err
	}
	return nil, toRef(n), nil
}

type filterInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Name string `json:"name,omitempty" jsonschema:"optional name substring filter"`
}

type componentsOutput struct {
	Components []NodeRef `json:"components"`
}

func (h *handlers) findComponents(_ context.Context, _ *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, componentsOutput, error) {
	svc, err := h.service(in.File)
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

type assetsOutput struct {
	Assets []model.ImageRef `json:"assets"`
}

func (h *handlers) findAssets(_ context.Context, _ *mcp.CallToolRequest, in filterInput) (*mcp.CallToolResult, assetsOutput, error) {
	svc, err := h.service(in.File)
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

func (h *handlers) findText(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	svc, err := h.service(in.File)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := svc.Search(in.Query, limit*4)
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
