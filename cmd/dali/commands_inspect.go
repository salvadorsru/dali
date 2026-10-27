package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/internal/embed"
	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/output"
	"github.com/salvadorsru/dali/internal/query"
	"github.com/salvadorsru/dali/internal/semantic"
)

func cmdSearch(args []string) error {
	q, err := arg(args, 0, "query")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 50, "max results")
	hybrid := fs.Bool("hybrid", false, "combine with semantic search (RRF)")
	yes := fs.Bool("yes", false, "confirm downloads without prompting")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *yes {
		os.Setenv("DALI_EMBED_YES", "1")
	}

	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	var emb embed.Embedder
	mode := query.ModeText
	if *hybrid {
		emb = embed.FromEnv()
		if emb == nil {
			return errors.New("embeddings disabled: DALI_EMBED_PROVIDER is 'off'")
		}
		defer embed.Close(emb)
		mode = query.ModeHybrid
		if _, err := semantic.IndexWithProgress(context.Background(), svc.DB(), emb, progress("embeddings")); err != nil {
			return err
		}
	}

	results, err := svc.UnifiedSearch(context.Background(), emb, query.UnifiedOptions{
		Query: q,
		Limit: *limit,
		Mode:  mode,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, results)
	}
	if len(results) == 0 {
		if sugs, serr := svc.SuggestNames(q, 8); serr == nil && suggestHint(sugs, q) {
			return nil
		}
	}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{output.ID(r.ID), output.Type(r.Type), truncate(r.Name, 40), output.Dim(truncate(r.Path, 60))})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "NAME", "PATH"}, rows)
	return nil
}

func cmdComponents(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	nodes, err := svc.Components()
	if err != nil {
		return err
	}
	return printNodes(nodes)
}

func cmdAssets(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	assets, err := svc.Assets()
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, assets)
	}
	rows := make([][]string, 0, len(assets))
	for _, a := range assets {
		size := ""
		if a.Width > 0 {
			size = strconv.Itoa(a.Width) + "x" + strconv.Itoa(a.Height)
		}
		rows = append(rows, []string{output.ID(a.Hash[:12]), a.MIME, output.Dim(size), truncate(a.Name, 50), output.Dim(a.NodeID)})
	}
	output.Table(os.Stdout, []string{"HASH", "MIME", "SIZE", "NAME", "NODE"}, rows)
	return nil
}

func cmdFonts(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	fonts, err := svc.Fonts()
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, fonts)
	}
	rows := make([][]string, 0, len(fonts))
	for _, f := range fonts {
		rows = append(rows, []string{f.Family, output.Dim(f.Style), strconv.Itoa(f.Count)})
	}
	output.Table(os.Stdout, []string{"FAMILY", "STYLE", "COUNT"}, rows)
	return nil
}

func cmdVariables(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	vars, err := svc.Variables()
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, vars)
	}
	rows := make([][]string, 0, len(vars))
	for _, v := range vars {
		value := ""
		for _, m := range v.Modes {
			if value != "" {
				value += " · "
			}
			value += m.Mode + "=" + m.Value
		}
		rows = append(rows, []string{v.Set, v.Name, output.Dim(v.Type), truncate(value, 60)})
	}
	output.Table(os.Stdout, []string{"SET", "NAME", "TYPE", "VALUES"}, rows)
	return nil
}

func cmdStyles(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	defs, err := svc.StyleDefs()
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, defs)
	}
	rows := make([][]string, 0, len(defs))
	for _, d := range defs {
		rows = append(rows, []string{d.Name, output.Label(d.Kind), output.Dim(d.Summary), output.Dim(strconv.Itoa(d.Usage))})
	}
	output.Table(os.Stdout, []string{"NAME", "KIND", "VALUE", "USES"}, rows)
	return nil
}

func cmdQuery(args []string) error {
	expr, err := arg(args, 0, "expression")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	limit := fs.Int("limit", 0, "max results")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	var nodes []*model.Node
	if strings.HasPrefix(strings.TrimSpace(expr), "/") {
		nodes, err = svc.XPath(expr)
	} else {
		nodes, err = svc.Query(expr)
	}
	if err != nil {
		return err
	}
	if *limit > 0 && len(nodes) > *limit {
		nodes = nodes[:*limit]
	}
	if len(nodes) == 0 {
		var sugs []query.Suggest
		var serr error
		if strings.HasPrefix(strings.TrimSpace(expr), "/") {
			sugs, serr = svc.SuggestXPath(expr, 8)
		} else {
			sugs, serr = svc.SuggestNames(expr, 8)
		}
		if serr == nil && suggestHint(sugs, expr) {
			return nil
		}
	}
	return printNodes(nodes)
}

func cmdUsages(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	n, err := svc.Node(id)
	if err != nil {
		return err
	}
	if n == nil {
		return fmt.Errorf("node %q not found", id)
	}
	places, err := svc.Usages(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, map[string]any{
			"component": n,
			"instances": len(places),
			"uses":      places,
		})
	}
	fmt.Printf("%s %s  %s\n", output.ID(n.ID), output.Type(string(n.Type)), output.Bold(n.Name))
	fmt.Printf("%s %d\n", output.Label("instances:"), len(places))
	return printPlacements(svc, places)
}

func cmdDependents(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	n, err := svc.Node(id)
	if err != nil {
		return err
	}
	if n == nil {
		return fmt.Errorf("node %q not found", id)
	}
	deps, err := svc.Dependents(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, deps)
	}
	fmt.Printf("%s %s  %s\n", output.ID(n.ID), output.Type(string(n.Type)), output.Bold(n.Name))
	if len(deps) == 0 {
		fmt.Fprintf(os.Stderr, "%s no placements for component %q\n", output.Warn("dali:"), id)
		return nil
	}
	rows := make([][]string, 0, len(deps))
	for _, d := range deps {
		rows = append(rows, []string{output.ID(d.ID), output.Type(d.Type), truncate(d.Name, 40), strconv.Itoa(d.Places)})
	}
	output.Table(os.Stdout, []string{"CONTAINER", "TYPE", "NAME", "PLACES"}, rows)
	return nil
}
