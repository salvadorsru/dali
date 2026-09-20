package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
	"github.com/salvadorsru/dali/semantic"
)

type searchInput struct {
	File        string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query       string `json:"query" jsonschema:"search text"`
	Limit       int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
	Context     bool   `json:"context,omitempty" jsonschema:"include parent, ancestors and children next to each result"`
	MaxChildren int    `json:"max_children,omitempty" jsonschema:"max direct children per result when context is on, default 5"`
}

type searchOutput struct {
	Results []NodeRef `json:"results"`
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
	results, err := svc.UnifiedSearch(ctx, h.emb, query.UnifiedOptions{
		Query:       in.Query,
		Limit:       limit,
		Mode:        query.ModeText,
		Context:     in.Context,
		MaxChildren: in.MaxChildren,
	})
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{Results: make([]NodeRef, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, NodeRef{ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, PagePath: r.Path})
	}
	return nil, out, nil
}

type semanticInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query string `json:"query" jsonschema:"conceptual search text"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
	Type  string `json:"type,omitempty" jsonschema:"filter by node type, e.g. COMPONENT"`
	Page  string `json:"page,omitempty" jsonschema:"filter by page name or id"`
}

func (h *handlers) semanticSearch(ctx context.Context, req *mcp.CallToolRequest, in semanticInput) (*mcp.CallToolResult, searchOutput, error) {
	if h.emb == nil {
		return nil, searchOutput{}, errors.New("no embedder configured")
	}
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if err := ensureEmbedded(ctx, svc, h.emb, note); err != nil {
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
	return nil, semanticResults(results), nil
}

type hybridInput struct {
	File        string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Query       string `json:"query" jsonschema:"search text"`
	Limit       int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
	Type        string `json:"type,omitempty" jsonschema:"filter by node type, e.g. COMPONENT"`
	Page        string `json:"page,omitempty" jsonschema:"filter by page name or id"`
	Context     bool   `json:"context,omitempty" jsonschema:"include parent, ancestors and children next to each result"`
	MaxChildren int    `json:"max_children,omitempty" jsonschema:"max direct children per result when context is on, default 5"`
}

func (h *handlers) hybridSearch(ctx context.Context, req *mcp.CallToolRequest, in hybridInput) (*mcp.CallToolResult, searchOutput, error) {
	if h.emb == nil {
		return nil, searchOutput{}, errors.New("no embedder configured")
	}
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if err := ensureEmbedded(ctx, svc, h.emb, note); err != nil {
		return nil, searchOutput{}, err
	}
	opts := query.UnifiedOptions{
		Query:       in.Query,
		Limit:       limit,
		Mode:        query.ModeHybrid,
		Type:        in.Type,
		Context:     in.Context,
		MaxChildren: in.MaxChildren,
	}
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
	results, err := svc.UnifiedSearch(ctx, h.emb, opts)
	if err != nil {
		return nil, searchOutput{}, err
	}
	return nil, semanticResults(results), nil
}

func (h *handlers) findText(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, searchOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := svc.UnifiedSearch(ctx, h.emb, query.UnifiedOptions{
		Query: in.Query,
		Type:  string(model.TypeText),
		Limit: limit * 4,
		Mode:  query.ModeHybrid,
	})
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{Results: make([]NodeRef, 0, limit)}
	for _, r := range results {
		if r.Type != string(model.TypeText) {
			continue
		}
		out.Results = append(out.Results, NodeRef{ID: r.ID, Type: r.Type, Name: r.Name, Text: r.Text, Page: r.Page, PagePath: r.Path})
		if len(out.Results) >= limit {
			break
		}
	}
	return nil, out, nil
}

// ensureEmbedded builds missing embeddings and reports progress, so semantic
// retrievers always run against a current vector index.
func ensureEmbedded(ctx context.Context, svc *query.Service, emb embed.Embedder, note *notifier) error {
	note.start("embedding")
	stats, err := semantic.IndexWithProgress(ctx, svc.DB(), emb, func(done, total int) {
		note.progress(fmt.Sprintf("embedding (%d/%d)", done, total), float64(done), float64(total))
	})
	if err != nil {
		return err
	}
	if stats.Indexed > 0 {
		note.done(fmt.Sprintf("embedded %d/%d nodes", stats.Indexed, stats.Total))
	} else {
		note.done("embeddings up to date")
	}
	return nil
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
