package mcp

import (
	"context"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
	"github.com/salvadorsru/dali/quality"
)

type lintInput struct {
	File   string `json:"file,omitempty" jsonschema:"fig file to lint; defaults to the one dali was started with"`
	Preset string `json:"preset,omitempty" jsonschema:"recommended (default), strict or accessibility"`
}

type lintOutput struct {
	Issues  []quality.Issue `json:"issues"`
	Summary map[string]int  `json:"summary"`
}

func (h *handlers) lintFigma(ctx context.Context, req *mcp.CallToolRequest, in lintInput) (*mcp.CallToolResult, lintOutput, error) {
	doc, file, err := h.load(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, lintOutput{}, err
	}
	defer file.Close()
	issues, err := quality.Lint(doc, quality.LintOptions{Preset: in.Preset})
	if err != nil {
		return nil, lintOutput{}, err
	}
	return nil, lintOutput{Issues: issues, Summary: quality.Summary(issues)}, nil
}

type analyzeInput struct {
	File     string  `json:"file,omitempty" jsonschema:"fig file to analyze; defaults to the one dali was started with"`
	Kind     string  `json:"kind" jsonschema:"spacing, clusters or overlaps"`
	MinRatio float64 `json:"min_ratio,omitempty" jsonschema:"overlaps: minimum overlap ratio of the smaller box (default 0.1)"`
	MinCount int     `json:"min_count,omitempty" jsonschema:"clusters: minimum repeats (default 3)"`
	Limit    int     `json:"limit,omitempty" jsonschema:"max rows (default 50)"`
}

type analyzeOutput struct {
	Kind     string            `json:"kind"`
	Spacing  []quality.GapStat `json:"spacing,omitempty"`
	Clusters []quality.Cluster `json:"clusters,omitempty"`
	Overlaps []quality.Overlap `json:"overlaps,omitempty"`
}

func (h *handlers) analyzeFigma(ctx context.Context, req *mcp.CallToolRequest, in analyzeInput) (*mcp.CallToolResult, analyzeOutput, error) {
	doc, file, err := h.load(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, analyzeOutput{}, err
	}
	defer file.Close()
	result, err := quality.Analyze(doc, in.Kind, quality.AnalyzeOptions{
		MinRatio: in.MinRatio,
		MinCount: in.MinCount,
		Limit:    in.Limit,
	})
	if err != nil {
		return nil, analyzeOutput{}, err
	}
	out := analyzeOutput{Kind: in.Kind}
	switch v := result.(type) {
	case []quality.GapStat:
		out.Spacing = v
	case []quality.Cluster:
		out.Clusters = v
	case []quality.Overlap:
		out.Overlaps = v
	}
	return nil, out, nil
}

// load parses a fig file once and builds the document tree. Cheap for CLI use;
// kept fresh so lint/analyze never depend on index staleness.
func (h *handlers) load(file string, note *notifier) (*model.Document, *parser.File, error) {
	abs, err := h.resolve(file)
	if err != nil {
		return nil, nil, err
	}
	note.start("parsing " + filepath.Base(abs))
	f, err := parser.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	doc := model.Build(f.NodeChanges(), f.Blobs())
	note.done("parsed " + filepath.Base(abs))
	return doc, f, nil
}
