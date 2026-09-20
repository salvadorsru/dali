package query

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/model"
)

// Ranker turns one or more candidate result sets into a single ranked list.
// Implementations are free to weight, reorder or discard candidates.
type Ranker interface {
	Rank(ctx context.Context, groups ...[]Result) ([]Result, error)
}

// RRF ranks candidates with Reciprocal Rank Fusion.
type RRF struct{ K float64 }

func (r RRF) k() float64 {
	if r.K > 0 {
		return r.K
	}
	return 60
}

// Rank merges the groups by reciprocal rank; a node present in several groups
// accumulates a higher score. Ties break by score then by id.
func (r RRF) Rank(ctx context.Context, groups ...[]Result) ([]Result, error) {
	k := r.k()
	scores := make(map[string]float64)
	byID := make(map[string]Result)
	for _, group := range groups {
		for rank, res := range group {
			scores[res.ID] += 1 / (k + float64(rank+1))
			if _, ok := byID[res.ID]; !ok {
				byID[res.ID] = res
			}
		}
	}
	out := make([]Result, 0, len(scores))
	for id, score := range scores {
		res := byID[id]
		res.Score = score
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Options carried by a single unified search call. Extend with new filters or
// ranking knobs without touching the call sites below.
type UnifiedOptions struct {
	Query  string
	Type   string
	PageID string
	Limit  int
	Mode   Mode
	// Structural forces the query text to be parsed as a structural
	// expression (type=, name~, descendants(), ...). If false the text is
	// still tried structurally when it looks like one, but a failed parse is
	// skipped instead of failing the whole search.
	Structural bool
	// Context requests contextual enrichment (parent, ancestors, children) on
	// each returned result. Off by default so plain searches stay cheap.
	Context     bool
	MaxChildren int
}

func (o UnifiedOptions) limit() int {
	if o.Limit > 0 {
		return o.Limit
	}
	return 20
}

// UnifiedSearch fuses structural, full-text and semantic candidates into one
// ranked list. Semantic retrieval is skipped when emb is nil.
func (s *Service) UnifiedSearch(ctx context.Context, emb embed.Embedder, opts UnifiedOptions) ([]Result, error) {
	limit := opts.limit()
	k := limit * 4
	groups := make([][]Result, 0, 3)

	text, err := s.Search(opts.Query, k)
	if err != nil {
		return nil, err
	}
	groups = append(groups, text)

	if emb != nil && (opts.Mode == ModeSemantic || opts.Mode == ModeHybrid) {
		semantic, err := s.SemanticSearch(ctx, emb, Options{
			Query:  opts.Query,
			Type:   opts.Type,
			PageID: opts.PageID,
			Limit:  k,
		})
		if err != nil {
			return nil, err
		}
		groups = append(groups, semantic)
	}

	// Structural candidates are best-effort unless explicitly requested: a
	// quote that accidentally ends in ')' must never sink the whole search.
	if opts.Structural || opts.Type != "" || opts.PageID != "" || isStructuralExpr(opts.Query) {
		filtered, err := s.queryStructural(ctx, opts)
		if err != nil {
			if opts.Structural {
				return nil, err
			}
			filtered = nil
		}
		if len(filtered) > 0 {
			groups = append(groups, filtered)
		}
	}

	ranked, err := s.ranker.Rank(ctx, groups...)
	if err != nil {
		return nil, err
	}
	if opts.Context {
		maxCh := opts.MaxChildren
		if maxCh <= 0 {
			maxCh = 5
		}
		for i := range ranked {
			if err := s.fillContext(&ranked[i], maxCh); err != nil {
				return nil, err
			}
		}
	}
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}

// isStructuralExpr reports whether the query text looks like a structural
// expression (a closing paren or bracket), so UnifiedSearch can try it
// structurally without being explicitly told.
func isStructuralExpr(q string) bool {
	if q == "" {
		return false
	}
	c := q[len(q)-1]
	return c == ')' || c == ']'
}

func (s *Service) queryStructural(ctx context.Context, opts UnifiedOptions) ([]Result, error) {
	if opts.Type == "" && opts.PageID == "" {
		if query := opts.Query; query != "" && (opts.Structural || isStructuralExpr(query)) {
			var nodes []*model.Node
			var err error
			if strings.HasPrefix(strings.TrimSpace(query), "/") {
				nodes, err = s.XPath(query)
			} else {
				nodes, err = s.Query(query)
			}
			if err != nil {
				return nil, err
			}
			out := make([]Result, 0, len(nodes))
			for _, n := range nodes {
				r := resultFromNode(n)
				if err := s.fill(&r); err != nil {
					return nil, err
				}
				out = append(out, r)
			}
			return out, nil
		}
		return nil, nil
	}
	rows, err := s.sql.Query(fmt.Sprintf(`%s %s %s %s`,
		`SELECT id, 1.0 FROM nodes`,
		`WHERE type = ?`,
		`AND page_id = ?`,
		`ORDER BY name LIMIT ?`), opts.Type, opts.PageID, opts.limit())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.fill(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
