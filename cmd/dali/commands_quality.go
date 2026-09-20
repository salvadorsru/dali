package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/salvadorsru/dali/export"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/parser"
	"github.com/salvadorsru/dali/quality"
)

type parsed struct {
	File *parser.File
	Doc  *model.Document
}

// openParsed parses the fig file once and builds the document tree.
func openParsed() (*parsed, error) {
	f, err := parser.Open(figFile)
	if err != nil {
		return nil, err
	}
	return &parsed{File: f, Doc: model.Build(f.NodeChanges(), f.Blobs())}, nil
}

func cmdLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	preset := fs.String("preset", "recommended", "recommended, strict or accessibility")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := openParsed()
	if err != nil {
		return err
	}
	defer p.File.Close()

	issues, err := quality.Lint(p.Doc, quality.LintOptions{Preset: *preset})
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, map[string]any{
			"issues":  issues,
			"summary": quality.Summary(issues),
		})
	}
	summary := quality.Summary(issues)
	rows := make([][]string, 0, len(issues))
	for _, i := range issues {
		rows = append(rows, []string{
			output.Label(i.Rule),
			severity(i.Severity),
			i.NodeID,
			truncate(i.Name, 30),
			output.Dim(truncate(i.Path, 40)),
			output.Dim(i.Detail),
		})
	}
	output.Table(os.Stdout, []string{"RULE", "SEVERITY", "NODE", "NAME", "PATH", "DETAIL"}, rows)
	order := []string{"text-contrast", "font-size", "touch-target", "text-opacity", "generic-name", "empty-frame", "duplicate-name", "unused-component", "hidden-with-children", "zero-size"}
	for _, r := range order {
		if count := summary[r]; count > 0 {
			fmt.Printf("%s %s\n", output.Label(r), output.Dim(strconv.Itoa(count)))
		}
	}
	return nil
}

func severity(s quality.Severity) string {
	switch s {
	case quality.Error:
		return output.Error(string(s))
	case quality.Warn:
		return output.Success(string(s))
	}
	return output.Dim(string(s))
}

func cmdAnalyze(args []string) error {
	kind, err := arg(args, 0, "kind")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	minRatio := fs.Float64("min-ratio", 0.1, "overlaps: minimum overlap ratio (0-1)")
	minCount := fs.Int("min-count", 3, "clusters: minimum repeats")
	limit := fs.Int("limit", 50, "max rows")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	p, err := openParsed()
	if err != nil {
		return err
	}
	defer p.File.Close()

	result, err := quality.Analyze(p.Doc, kind, quality.AnalyzeOptions{
		MinRatio: *minRatio,
		MinCount: *minCount,
		Limit:    *limit,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, result)
	}
	switch v := result.(type) {
	case []quality.GapStat:
		rows := make([][]string, 0, len(v))
		for _, g := range v {
			rows = append(rows, []string{fmt.Sprintf("%g", g.Value), strconv.Itoa(g.Count)})
		}
		output.Table(os.Stdout, []string{"GAP", "FRAMES"}, rows)
	case []quality.Cluster:
		rows := make([][]string, 0, len(v))
		for _, c := range v {
			rows = append(rows, []string{c.Type, c.Page, fmt.Sprintf("%gx%g", c.Width, c.Height), strconv.Itoa(c.Count)})
		}
		output.Table(os.Stdout, []string{"TYPE", "PAGE", "SIZE", "COUNT"}, rows)
	case []quality.Overlap:
		rows := make([][]string, 0, len(v))
		for _, o := range v {
			rows = append(rows, []string{o.A, o.B, o.Page, fmt.Sprintf("%.0f%%", o.Ratio*100)})
		}
		output.Table(os.Stdout, []string{"A", "B", "PAGE", "OVERLAP"}, rows)
	}
	return nil
}

