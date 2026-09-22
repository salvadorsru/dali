package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/assets"
	"github.com/salvadorsru/dali/codegen"
	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/mcp"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/query"
	"github.com/salvadorsru/dali/semantic"
)

func open(path string) (*query.Service, error) {
	return query.Open(path)
}

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

func cmdNode(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	node, err := svc.Node(id)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node %q not found", id)
	}
	if jsonOut {
		return output.JSON(os.Stdout, node)
	}
	printNode(node)
	return nil
}

func cmdContext(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	ctx, err := svc.Context(id)
	if err != nil {
		return err
	}
	if ctx == nil || ctx.Node == nil {
		return fmt.Errorf("node %q not found", id)
	}
	if jsonOut {
		return output.JSON(os.Stdout, ctx)
	}
	printContext(ctx)
	return nil
}

func cmdChildren(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	nodes, err := svc.Children(id)
	if err != nil {
		return err
	}
	return printNodes(nodes)
}

func cmdParent(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	node, err := svc.Parent(id)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node %q has no parent", id)
	}
	return printNodes([]*model.Node{node})
}

func cmdPath(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	nodes, err := svc.Path(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, nodes)
	}
	for i, n := range nodes {
		depth := 0
		for j := 0; j < i; j++ {
			depth++
		}
		prefix := "└── "
		if i == 0 {
			prefix = ""
		}
		fmt.Printf("%s%s%s %s\n", strings.Repeat("  ", depth), output.Dim(prefix), output.Type(string(n.Type)), n.Name)
	}
	return nil
}

func cmdSubtree(args []string) error {
	id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()

	depth := 0
	for i, a := range args {
		if a == "--depth" && i+1 < len(args) {
			d, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("--depth: %w", err)
			}
			depth = d
		}
	}
	res, err := svc.Subtree(id, depth)
	if err != nil {
		return err
	}
	root, err := svc.Node(id)
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("node %q not found", id)
	}
	if jsonOut {
		out := subtreeOutput{Root: root, Descendants: make([]subtreeEntry, 0, len(res))}
		for _, it := range res {
			out.Descendants = append(out.Descendants, subtreeEntry{Node: it.Node, Depth: it.Depth})
		}
		return output.JSON(os.Stdout, out)
	}
	fmt.Printf("%s %s%s\n", output.Type(string(root.Type)), root.Name, output.Dim(" "+root.ID))
	for _, d := range res {
		prefix := strings.Repeat("  ", d.Depth) + "└── "
		extra := ""
		if d.Node.Text != "" {
			extra = output.Dim(" " + strconv.Quote(truncate(d.Node.Text, 24)))
		}
		fmt.Printf("%s%s %s%s\n", prefix, output.Type(string(d.Node.Type)), d.Node.Name, extra)
	}
	return nil
}

type subtreeEntry struct {
	Node  *model.Node `json:"node"`
	Depth int         `json:"depth"`
}

type subtreeOutput struct {
	Root        *model.Node    `json:"root"`
	Descendants []subtreeEntry `json:"descendants"`
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

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	outputDir := fs.String("out", "./generated", "output directory")
	node := fs.String("id", "", "node ID to generate (default: all)")
	byName := fs.String("name", "", "node name substring to generate (default: all)")
	tokensFormat := fs.String("tokens", "css", "tokens format: css, style-dict, figma")
	if err := fs.Parse(args); err != nil {
		return err
	}

	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	config := ir.DefaultConfig()
	config.OutputDir = *outputDir

	// A single node to generate, resolved by id or by name
	if *node != "" && *byName != "" {
		return fmt.Errorf("use either --id or --name, not both")
	}
	if *byName != "" {
		nodes, err := svc.Query(fmt.Sprintf("name~%q", *byName))
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			if sugs, serr := svc.SuggestNames(*byName, 8); serr == nil && suggestHint(sugs, *byName) {
				return nil
			}
			return fmt.Errorf("no node named %q", *byName)
		}
		if len(nodes) > 1 {
			for _, n := range nodes {
				fmt.Printf("%s  %s  %s  %s\n", output.ID(n.ID), output.Type(string(n.Type)), truncate(n.Name, 40), output.Dim(n.PageID))
			}
			return fmt.Errorf("%q matches %d nodes; pick one with --id", *byName, len(nodes))
		}
		return codegen.GenerateComponents(svc, []string{nodes[0].ID}, config)
	}

	// If id specified, generate just that one
	if *node != "" {
		return codegen.GenerateComponents(svc, []string{*node}, config)
	}

	// Generate tokens if requested
	if *tokensFormat != "" {
		if err := codegen.GenerateTokens(svc, config, *tokensFormat); err != nil {
			return fmt.Errorf("generate tokens: %w", err)
		}
	}

	// Generate all components
	return codegen.GenerateAllComponents(svc, config)
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

func cmdExtractAssets(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	refs, err := svc.Assets()
	if err != nil {
		return err
	}
	out, err := assets.Extract(figFile, refs)
	if err != nil {
		return err
	}
	if err := svc.UpdateAssetInfo(out); err != nil {
		return err
	}
	fmt.Printf("extracted %d assets to %s\n", len(out), assets.ImagesDir(figFile))
	return nil
}

func cmdMCP(args []string) error {
	mcp.Version = version
	return mcp.Serve(context.Background(), figFile)
}

func openWithID(args []string) (string, *query.Service, error) {
	id, err := arg(args, 0, "id")
	if err != nil {
		return "", nil, err
	}
	svc, err := open(figFile)
	if err != nil {
		return "", nil, err
	}
	return id, svc, nil
}

func printNodes(nodes []*model.Node) error {
	if jsonOut {
		return output.JSON(os.Stdout, nodes)
	}
	rows := make([][]string, 0, len(nodes))
	for _, n := range nodes {
		rows = append(rows, []string{output.ID(n.ID), output.Type(string(n.Type)), truncate(n.Name, 50), output.Dim(truncate(n.Text, 40))})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "NAME", "TEXT"}, rows)
	return nil
}

// printPlacements lists component placements with the page they live on,
// resolving page ids through the service's page index.
func printPlacements(svc *query.Service, places []*model.Node) error {
	if jsonOut {
		return output.JSON(os.Stdout, places)
	}
	pages, err := svc.Pages()
	if err != nil {
		return err
	}
	pageName := make(map[string]string, len(pages))
	for _, p := range pages {
		pageName[p.ID] = p.Name
	}
	rows := make([][]string, 0, len(places))
	for _, p := range places {
		rows = append(rows, []string{output.ID(p.ID), output.Type(string(p.Type)), truncate(p.Name, 40), output.Dim(pageName[p.PageID])})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "NAME", "PAGE"}, rows)
	return nil
}

// suggestHint prints token-scored name suggestions to stderr when a search
// matched nothing. It reports whether anything was printed.
func suggestHint(sugs []query.Suggest, pattern string) bool {
	if jsonOut || len(sugs) == 0 {
		return false
	}
	names := make([]string, 0, len(sugs))
	for _, s := range sugs {
		names = append(names, output.Dim(s.Name))
	}
	fmt.Fprintf(os.Stderr, "%s no matches for %q, did you mean: %s\n",
		output.Warn("dali:"), pattern, strings.Join(names, ", "))
	return true
}

func field(label, value string) {
	fmt.Printf("  %s %s\n", output.Label(fmt.Sprintf("%-9s", label)), value)
}

func printNode(n *model.Node) {
	fmt.Printf("%s  %s  %s\n", output.ID(n.ID), output.Type(string(n.Type)), output.Bold(fmt.Sprintf("%q", n.Name)))
	field("page:", output.Dim(n.PageID))
	if n.ParentID != "" {
		field("parent:", output.Dim(n.ParentID))
	}
	field("size:", fmt.Sprintf("%gx%g", n.Width, n.Height))
	field("pos:", fmt.Sprintf("%g,%g", n.X, n.Y))
	field("visible:", fmt.Sprintf("%v", n.Visible))
	field("opacity:", fmt.Sprintf("%g", n.Opacity))
	if n.Text != "" {
		field("text:", fmt.Sprintf("%q", truncate(n.Text, 120)))
	}
	if n.FontFamily != "" {
		field("font:", fmt.Sprintf("%s %s %g", n.FontFamily, n.FontStyle, n.FontSize))
	}
	if n.ComponentID != "" {
		field("component:", output.Dim(n.ComponentID))
	}
}

func printContext(ctx *query.Context) {
	fmt.Println(output.Title("Path"))
	for i, n := range ctx.Path {
		prefix := "└── "
		if i == 0 {
			prefix = ""
		}
		fmt.Printf("  %s%s%s %s\n", output.Dim(fmt.Sprintf("%-*s", i*3, "")), output.Dim(prefix), output.Type(string(n.Type)), n.Name)
	}
	fmt.Printf("\n%s %s  %s  %s\n", output.Title("Node"),
		output.ID(ctx.Node.ID), output.Type(string(ctx.Node.Type)), output.Bold(fmt.Sprintf("%q", ctx.Node.Name)))
	if ctx.Node.Text != "" {
		field("text:", fmt.Sprintf("%q", truncate(ctx.Node.Text, 200)))
	}
	if ctx.Component != nil {
		field("component:", fmt.Sprintf("%s  %s", output.ID(ctx.Component.ID), ctx.Component.Name))
	}
	if len(ctx.Children) > 0 {
		fmt.Println(output.Title(fmt.Sprintf("Children (%d)", len(ctx.Children))))
		for _, c := range ctx.Children {
			fmt.Printf("  %s  %s  %s\n", output.ID(c.ID), output.Type(string(c.Type)), truncate(c.Name, 40))
		}
	}
	if len(ctx.Styles) > 0 {
		fmt.Println(output.Title(fmt.Sprintf("Styles (%d)", len(ctx.Styles))))
		for _, s := range ctx.Styles {
			name := s.Name
			if name == "" {
				name = s.Key
			}
			fmt.Printf("  %s %s %s\n", output.Label(s.Kind), output.Bold(name), output.Dim(s.Summary))
		}
	}
	if len(ctx.Assets) > 0 {
		fmt.Println(output.Title(fmt.Sprintf("Assets (%d)", len(ctx.Assets))))
		for _, a := range ctx.Assets {
			fmt.Printf("  %s  %s  %s\n", output.ID(a.Hash[:12]), a.MIME, truncate(a.Name, 40))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
