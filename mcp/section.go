package mcp

import (
	"context"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
)

type sectionInput struct {
	File   string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Name   string `json:"name" jsonschema:"layer name to resolve, case-insensitive substring"`
	Page   string `json:"page,omitempty" jsonschema:"optional page name substring filter"`
	Within string `json:"within,omitempty" jsonschema:"optional ancestor name substring the section must live under"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max candidates to consider (default 25)"`
}

// SectionView is the one-shot inspection of a named block: the resolved node
// with its structured styles, its place in the tree, the contained text, and
// the full subtree so a section can be reconstructed without further calls.
type SectionView struct {
	Node         NodeRef          `json:"node"`
	Path         []NodeRef        `json:"path,omitempty"`
	Parent       *NodeRef         `json:"parent,omitempty"`
	Children     []NodeRef        `json:"children"`
	Styles       []model.Style    `json:"styles"`
	Images       []model.ImageRef `json:"images"`
	Texts        []NodeRef        `json:"texts,omitempty"`
	Descendants  []SubtreeView    `json:"descendants,omitempty" jsonschema:"whole subtree with geometry, styles and images"`
	Alternatives []NodeRef        `json:"alternatives,omitempty" jsonschema:"other layers matching the name, for disambiguation"`
	Matches      []string         `json:"matches,omitempty" jsonschema:"near-miss names when nothing resolved"`
}

func (h *handlers) sectionFigma(ctx context.Context, req *mcp.CallToolRequest, in sectionInput) (*mcp.CallToolResult, SectionView, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, SectionView{}, err
	}
	node, alternatives, matches, err := resolveSection(svc, in)
	if err != nil {
		return nil, SectionView{}, err
	}
	if node == nil {
		return nil, SectionView{Matches: matches}, nil
	}
	c, err := svc.Context(node.ID)
	if err != nil || c == nil || c.Node == nil {
		return nil, SectionView{}, err
	}
	pages := pageNames(svc)
	view := SectionView{
		Node:     pageRef(c.Node, pages),
		Children: toRefs(c.Children),
		Styles:   c.Styles,
		Images:   c.Assets,
	}
	enrich(svc, &view.Node, c.Styles)
	for _, p := range c.Path {
		view.Path = append(view.Path, toRef(p))
	}
	if c.Parent != nil {
		ref := pageRef(c.Parent, pages)
		view.Parent = &ref
	}
	for _, a := range alternatives {
		view.Alternatives = append(view.Alternatives, pageRef(a, pages))
	}
	st, err := SubtreeOf(svc, node.ID, 0)
	if err != nil {
		return nil, SectionView{}, err
	}
	view.Descendants = st.Descendants
	for _, d := range st.Descendants {
		if d.Node.Type == string(model.TypeText) && d.Node.Text != "" {
			view.Texts = append(view.Texts, d.Node)
		}
	}
	return nil, view, nil
}

// pageNames maps page ids to page names for disambiguation in section replies.
func pageNames(svc *query.Service) map[string]string {
	pages, err := svc.Pages()
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(pages))
	for _, p := range pages {
		out[p.ID] = p.Name
	}
	return out
}

// pageRef renders a node as a NodeRef annotated with the name of its page.
func pageRef(n *model.Node, pages map[string]string) NodeRef {
	ref := toRef(n)
	ref.PageName = pages[n.PageID]
	return ref
}

// resolveSection picks the best layer whose name contains in.Name, honouring the
// optional page and within filters. Exact names win, then shallower container
// types, then shallower depth. Remaining candidates are returned as
// alternatives so an ambiguous name stays disambiguable. When nothing matches
// it returns near-miss names.
func resolveSection(svc *query.Service, in sectionInput) (*model.Node, []*model.Node, []string, error) {
	expr := `name~"` + escapePredicate(in.Name) + `"`
	if in.Page != "" {
		expr += ` AND page~"` + escapePredicate(in.Page) + `"`
	}
	nodes, err := svc.Query(expr)
	if err != nil {
		return nil, nil, nil, err
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	type cand struct {
		node  *model.Node
		depth int
		exact bool
	}
	var cands []cand
	for i, n := range nodes {
		if i >= limit*4 {
			break
		}
		path, err := svc.Path(n.ID)
		if err != nil {
			continue
		}
		if in.Within != "" && !hasAncestor(path, in.Within) {
			continue
		}
		cands = append(cands, cand{node: n, depth: len(path), exact: strings.EqualFold(n.Name, in.Name)})
	}
	if len(cands) == 0 {
		return nil, nil, suggest(svc, in.Name), nil
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.exact != b.exact {
			return a.exact
		}
		if ra, rb := sectionRank(a.node.Type), sectionRank(b.node.Type); ra != rb {
			return ra < rb
		}
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		return len(a.node.Name) < len(b.node.Name)
	})
	alts := make([]*model.Node, 0, min(limit, len(cands)-1))
	for _, c := range cands[1:] {
		alts = append(alts, c.node)
		if len(alts) >= limit {
			break
		}
	}
	return cands[0].node, alts, nil, nil
}

// hasAncestor reports whether any ancestor (excluding the node itself) has a
// name containing needle.
func hasAncestor(path []*model.Node, needle string) bool {
	needle = strings.ToLower(needle)
	for _, p := range path[:max(0, len(path)-1)] {
		if strings.Contains(strings.ToLower(p.Name), needle) {
			return true
		}
	}
	return false
}

// sectionRank orders container types so a section or frame is preferred over a
// nested text or vector when several layers share a name.
func sectionRank(t model.NodeType) int {
	switch t {
	case "SECTION":
		return 0
	case model.TypeFrame:
		return 1
	case model.TypeSymbol, "COMPONENT":
		return 2
	case model.TypeInstance:
		return 3
	case model.TypeGroup:
		return 4
	default:
		return 5
	}
}

func escapePredicate(s string) string {
	return strings.NewReplacer(`"`, "", `\`, "").Replace(s)
}
