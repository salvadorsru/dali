package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/embed"
	"github.com/salvadorsru/dali/internal/query"
	"github.com/salvadorsru/dali/internal/semantic"
)

type searchInput struct {
	File        string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Mode        string `json:"mode,omitempty" jsonschema:"text (default), semantic or hybrid. text = exact keyword over node names and body text; semantic = conceptual meaning (requires the vector index); hybrid = fuse name/text and vectors when the index exists, falling back to plain text"`
	Query       string `json:"query" jsonschema:"search text"`
	Type        string `json:"type,omitempty" jsonschema:"restrict results to a node type, e.g. TEXT, FRAME or COMPONENT"`
	Page        string `json:"page,omitempty" jsonschema:"restrict to a page by name (semantic and hybrid modes)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
	Context     bool   `json:"context,omitempty" jsonschema:"include parent, ancestors and children next to each result (text mode)"`
	MaxChildren int    `json:"max_children,omitempty" jsonschema:"max direct children per result when context is on, default 5"`
}

type searchOutput struct {
	Results []NodeRef `json:"results"`
	// Suggestions lists layer names close to the query when nothing matched.
	Suggestions []string `json:"suggestions,omitempty"`
	// UsedEmbeddings reports whether semantic vectors contributed to hybrid
	// results; false means the search fell back to full-text only.
	UsedEmbeddings bool `json:"used_embeddings,omitempty"`
}

func (h *handlers) search(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}

	switch in.Mode {
	case "semantic":
		out, err := h.searchSemantic(ctx, svc, in, limit)
		return nil, out, err
	default:
		out, err := h.searchUnified(ctx, svc, in, limit, in.Mode == "hybrid")
		return nil, out, err
	}
}

// searchUnified serves the text (default) and hybrid modes. Text is exact
// keyword over node names and body text; hybrid fuses full-text and semantic
// vectors when a vector index exists, otherwise falls back to plain text. It
// never builds embeddings.
func (h *handlers) searchUnified(ctx context.Context, svc *query.Service, in searchInput, limit int, wantHybrid bool) (searchOutput, error) {
	var emb embed.Embedder
	mode := query.ModeText
	if wantHybrid && h.emb != nil && svc.DB().VecIndexed() {
		emb = h.emb
		mode = query.ModeHybrid
	}
	opts := query.UnifiedOptions{
		Query:       in.Query,
		Limit:       limit,
		Mode:        mode,
		Type:        in.Type,
		Context:     in.Context,
		MaxChildren: in.MaxChildren,
	}
	if wantHybrid && in.Page != "" {
		id, err := resolvePage(svc, in.Page)
		if err != nil {
			return searchOutput{}, err
		}
		opts.PageID = id
	}
	results, err := svc.UnifiedSearch(ctx, emb, opts)
	if err != nil {
		return searchOutput{}, err
	}
	out := semanticResults(results)
	out.UsedEmbeddings = mode == query.ModeHybrid
	if len(out.Results) == 0 && in.Type == "" && !wantHybrid {
		out.Suggestions = suggest(svc, in.Query)
	}
	return out, nil
}

// searchSemantic is the pure vector path: it requires both the embedder and
// the vector index, and never builds embeddings.
func (h *handlers) searchSemantic(ctx context.Context, svc *query.Service, in searchInput, limit int) (searchOutput, error) {
	if h.emb == nil {
		return searchOutput{}, errors.New("embeddings disabled: set DALI_EMBEDDINGS=1 and restart the MCP server to use semantic search")
	}
	if !svc.DB().VecIndexed() {
		return searchOutput{}, errors.New("vector index missing: call index_embeddings first")
	}
	opts := query.Options{Query: in.Query, Type: in.Type, Limit: limit}
	if in.Page != "" {
		id, err := resolvePage(svc, in.Page)
		if err != nil {
			return searchOutput{}, err
		}
		opts.PageID = id
	}
	results, err := svc.SemanticSearch(ctx, h.emb, opts)
	if err != nil {
		return searchOutput{}, err
	}
	return semanticResults(results), nil
}

// resolvePage maps a page name to its id, failing when the page is unknown.
func resolvePage(svc *query.Service, name string) (string, error) {
	id, err := svc.PageByName(name)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("page %q not found", name)
	}
	return id, nil
}

// ensureEmbedded builds missing embeddings and reports progress, so the vector
// index is current. indexEmbeddings is its only caller; searches never index.
func ensureEmbedded(ctx context.Context, svc *query.Service, emb embed.Embedder, note *notifier) (semantic.Stats, error) {
	note.start("embedding")
	stats, err := semantic.IndexWithProgress(ctx, svc.DB(), emb, func(done, total int) {
		note.progress(fmt.Sprintf("embedding (%d/%d)", done, total), float64(done), float64(total))
	})
	if err != nil {
		return stats, err
	}
	if stats.Indexed > 0 {
		note.done(fmt.Sprintf("embedded %d/%d nodes", stats.Indexed, stats.Total))
	} else {
		note.done("embeddings up to date")
	}
	return stats, nil
}

type fileInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
}

type indexOutput struct {
	Total   int    `json:"total"`
	Indexed int    `json:"indexed"`
	Reused  int    `json:"reused"`
	Deleted int    `json:"deleted"`
	Skipped int    `json:"skipped"`
	Model   string `json:"model"`
}

// indexEmbeddings builds the vector index explicitly. It honours ctx: when the
// client disconnects, the embedding pipeline stops instead of running on.
func (h *handlers) indexEmbeddings(ctx context.Context, req *mcp.CallToolRequest, in fileInput) (*mcp.CallToolResult, indexOutput, error) {
	if h.emb == nil {
		return nil, indexOutput{}, errors.New("embeddings disabled: set DALI_EMBEDDINGS=1 and restart the MCP server")
	}
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, indexOutput{}, err
	}
	stats, err := ensureEmbedded(ctx, svc, h.emb, note)
	if err != nil {
		return nil, indexOutput{}, err
	}
	return nil, indexOutput{
		Total: stats.Total, Indexed: stats.Indexed, Reused: stats.Reused,
		Deleted: stats.Deleted, Skipped: stats.Skipped, Model: stats.Model,
	}, nil
}

func semanticResults(results []query.Result) searchOutput {
	out := searchOutput{Results: make([]NodeRef, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, NodeRef{
			ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, PagePath: r.Path, Score: r.Score,
		})
	}
	return out
}
