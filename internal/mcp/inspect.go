package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/query"
)

// layoutModeOf returns a node's auto-layout mode (HORIZONTAL, VERTICAL) from
// its LAYOUT style, empty when the node has no auto-layout.
func layoutModeOf(styles []model.Style) string {
	if m := layoutDataOf(styles); m != nil {
		mode, _ := m["layoutMode"].(string)
		return strings.ToUpper(mode)
	}
	return ""
}

// absOf returns a node's absolute position by summing x/y along its ancestor
// chain. Node x/y are relative to the parent, so the page origin is the sum.
func absOf(svc *query.Service, id string) (float64, float64) {
	path, err := svc.Path(id)
	if err != nil {
		return 0, 0
	}
	var x, y float64
	for _, p := range path {
		x += p.X
		y += p.Y
	}
	return x, y
}

// isFreePositioned reports whether a node is placed absolutely, i.e. its parent
// has no auto-layout.
func isFreePositioned(svc *query.Service, id string) bool {
	parent, err := svc.Parent(id)
	if err != nil || parent == nil {
		return false
	}
	styles, err := svc.StylesOf(parent.ID)
	if err != nil {
		return false
	}
	switch layoutModeOf(styles) {
	case "", "NONE":
		return true
	}
	return false
}

func enrich(svc *query.Service, ref *NodeRef, styles []model.Style) {
	ref.Layout = layoutOf(styles)
	ref.LayoutData = layoutDataOf(styles)
	ref.AbsX, ref.AbsY = absOf(svc, ref.ID)
	ref.FreePositioned = isFreePositioned(svc, ref.ID)
}

type idInput struct {
	File string `json:"file,omitempty" jsonschema:"project from list_figs or a path under the designs directory; omit it to use the active project and always omit it when dali serves a single design"`
	ID   string `json:"id" jsonschema:"node id, e.g. 123:456"`
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
	enrich(svc, &view.Node, view.Styles)
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

type descendantsInput struct {
	File  string `json:"file,omitempty" jsonschema:"project from list_figs or a path under the designs directory; omit it to use the active project and always omit it when dali serves a single design"`
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
	rootStyles, err := svc.StylesOf(id)
	if err != nil {
		return SubtreeResult{}, err
	}
	out := SubtreeResult{Root: toRef(root), Descendants: make([]SubtreeView, 0, len(items))}
	enrich(svc, &out.Root, rootStyles)

	// Absolute positions accumulate along the tree; items arrive in pre-order
	// (parent before children), so each node's parent is already resolved.
	absX := map[string]float64{id: out.Root.AbsX}
	absY := map[string]float64{id: out.Root.AbsY}
	layoutMode := map[string]string{id: layoutModeOf(rootStyles)}
	for _, it := range items {
		view := SubtreeView{Node: toRef(it.Node), Depth: it.Depth}
		s := sby[it.Node.ID]
		view.Styles = s
		view.Node.Layout = layoutOf(s)
		view.Node.LayoutData = layoutDataOf(s)
		view.Node.AbsX = absX[it.Node.ParentID] + it.Node.X
		view.Node.AbsY = absY[it.Node.ParentID] + it.Node.Y
		absX[it.Node.ID] = view.Node.AbsX
		absY[it.Node.ID] = view.Node.AbsY
		if mode := layoutMode[it.Node.ParentID]; mode == "" || mode == "NONE" {
			view.Node.FreePositioned = true
		}
		layoutMode[it.Node.ID] = layoutModeOf(s)
		if a := aby[it.Node.ID]; len(a) > 0 {
			view.Images = a
		}
		out.Descendants = append(out.Descendants, view)
	}
	return out, nil
}
