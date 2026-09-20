package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
)

type idInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	ID   string `json:"id" jsonschema:"node id, e.g. 123:456"`
}

type NodeView struct {
	Node   NodeRef       `json:"node"`
	Styles []model.Style `json:"styles"`
}

func (h *handlers) getNode(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeView, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, NodeView{}, err
	}
	n, err := svc.Node(in.ID)
	if err != nil || n == nil {
		return nil, NodeView{}, err
	}
	view := NodeView{Node: toRef(n)}
	if view.Styles, err = svc.StylesOf(in.ID); err != nil {
		return nil, NodeView{}, err
	}
	view.Node.Layout = layoutOf(view.Styles)
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

func (h *handlers) getContext(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, ContextView, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, ContextView{}, err
	}
	c, err := svc.Context(in.ID)
	if err != nil || c == nil || c.Node == nil {
		return nil, ContextView{}, err
	}
	view := ContextView{
		Node:     toRef(c.Node),
		Children: toRefs(c.Children),
		Styles:   c.Styles,
		Assets:   c.Assets,
	}
	view.Node.Layout = layoutOf(view.Styles)
	for _, p := range c.Path {
		view.Path = append(view.Path, toRef(p))
	}
	if c.Parent != nil {
		ref := toRef(c.Parent)
		view.Parent = &ref
	}
	if c.Component != nil {
		ref := toRef(c.Component)
		view.Component = &ref
	}
	return nil, view, nil
}

type childrenOutput struct {
	Children []NodeRef `json:"children"`
}

func (h *handlers) getChildren(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, childrenOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, childrenOutput{}, err
	}
	nodes, err := svc.Children(in.ID)
	if err != nil {
		return nil, childrenOutput{}, err
	}
	return nil, childrenOutput{Children: toRefs(nodes)}, nil
}

func (h *handlers) getParent(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, NodeRef, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, NodeRef{}, err
	}
	n, err := svc.Parent(in.ID)
	if err != nil || n == nil {
		return nil, NodeRef{}, err
	}
	return nil, toRef(n), nil
}

type descendantsInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	ID    string `json:"id" jsonschema:"node id, e.g. 123:456"`
	Depth int    `json:"depth,omitempty" jsonschema:"max depth from the node's children; 0 (default) walks the whole subtree"`
}

type SubtreeView struct {
	Node   NodeRef          `json:"node"`
	Depth  int              `json:"depth,omitempty"`
	Styles []model.Style    `json:"styles"`
	Images []model.ImageRef `json:"images"`
}

type SubtreeResult struct {
	Root        NodeRef       `json:"root"`
	Descendants []SubtreeView `json:"descendants"`
}

func (h *handlers) getDescendants(ctx context.Context, req *mcp.CallToolRequest, in descendantsInput) (*mcp.CallToolResult, SubtreeResult, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, SubtreeResult{}, err
	}
	out, err := SubtreeOf(svc, in.ID, in.Depth)
	return nil, out, err
}

// SubtreeOf builds the get_descendants view ('root + descendants with depth, styles'
// and images') for a user-supplied node id.
func SubtreeOf(svc *query.Service, id string, maxDepth int) (SubtreeResult, error) {
	root, err := svc.Node(id)
	if err != nil {
		return SubtreeResult{}, err
	}
	if root == nil {
		return SubtreeResult{}, nil
	}
	if maxDepth < 0 {
		maxDepth = 0
	}
	items, err := svc.Subtree(id, maxDepth)
	if err != nil {
		return SubtreeResult{}, err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.Node.ID)
	}
	sby, err := svc.StylesOfMany(ids)
	if err != nil {
		return SubtreeResult{}, err
	}
	aby, err := svc.AssetsOfMany(ids)
	if err != nil {
		return SubtreeResult{}, err
	}
	out := SubtreeResult{Root: toRef(root), Descendants: make([]SubtreeView, 0, len(items))}
	for _, it := range items {
		view := SubtreeView{Node: toRef(it.Node), Depth: it.Depth}
		if s := sby[it.Node.ID]; len(s) > 0 {
			view.Styles = s
			view.Node.Layout = layoutOf(s)
		}
		if a := aby[it.Node.ID]; len(a) > 0 {
			view.Images = a
		}
		out.Descendants = append(out.Descendants, view)
	}
	if rootStyles, err := svc.StylesOf(id); err == nil {
		out.Root.Layout = layoutOf(rootStyles)
	}
	return out, nil
}
