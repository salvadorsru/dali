package query

import (
	"context"
	"strings"

	"github.com/salvadorsru/dali/internal/embed"
	"github.com/salvadorsru/dali/internal/model"
)

type Mode string

const (
	ModeText     Mode = "text"
	ModeSemantic Mode = "semantic"
	ModeHybrid   Mode = "hybrid"
)

type Options struct {
	Query  string
	Type   string
	PageID string
	Limit  int
}

func (o Options) limit() int {
	if o.Limit > 0 {
		return o.Limit
	}
	return 20
}

func (o Options) hasFilter() bool { return o.Type != "" || o.PageID != "" }

// SemanticSearch embeds the query and returns the nearest nodes, optionally
// filtered by node type and page.
func (s *Service) SemanticSearch(ctx context.Context, emb embed.Embedder, opts Options) ([]Result, error) {
	if !s.db.VecIndexed() {
		return nil, nil
	}
	vectors, err := emb.Embed(ctx, []string{opts.Query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}
	limit := opts.limit()
	k := limit
	if opts.hasFilter() {
		k = limit * 4
	}
	neighbors, err := s.db.SearchVectors(vectors[0], k)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, limit)
	for _, nb := range neighbors {
		n, err := s.Node(nb.NodeID)
		if err != nil {
			return nil, err
		}
		if n == nil || !matchFilter(n, opts) {
			continue
		}
		r := resultFromNode(n)
		r.Distance = nb.Distance
		r.Score = 1 - nb.Distance
		if err := s.fill(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func resultFromNode(n *model.Node) Result {
	return Result{
		ID:   n.ID,
		Type: string(n.Type),
		Name: n.Name,
		Text: n.Text,
		Page: n.PageID,
	}
}

func matchFilter(n *model.Node, opts Options) bool {
	if opts.Type != "" && string(n.Type) != strings.ToUpper(opts.Type) {
		return false
	}
	if opts.PageID != "" && n.PageID != opts.PageID {
		return false
	}
	return true
}

// PageByName resolves a page name to its id (or returns the input if it is an id).
func (s *Service) PageByName(name string) (string, error) {
	pages, err := s.Pages()
	if err != nil {
		return "", err
	}
	for _, p := range pages {
		if p.ID == name {
			return p.ID, nil
		}
	}
	for _, p := range pages {
		if strings.EqualFold(p.Name, name) {
			return p.ID, nil
		}
	}
	return "", nil
}
