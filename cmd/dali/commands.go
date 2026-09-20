package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/assets"
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
	res, err := mcp.SubtreeOf(svc, id, depth)
	if err != nil {
		return err
	}
	if res.Root.ID == "" {
		return fmt.Errorf("node %q not found", id)
	}
	if jsonOut {
		return output.JSON(os.Stdout, res)
	}
	fmt.Printf("%s %s%s\n", output.Type(res.Root.Type), res.Root.Name, output.Dim(" "+res.Root.ID))
	for _, d := range res.Descendants {
		prefix := strings.Repeat("  ", d.Depth) + "└── "
		extra := ""
		if d.Node.Text != "" {
			extra = output.Dim(" " + strconv.Quote(truncate(d.Node.Text, 24)))
		}
		fmt.Printf("%s%s %s%s\n", prefix, output.Type(d.Node.Type), d.Node.Name, extra)
	}
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
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	nodes, err := svc.Query(expr)
	if err != nil {
		return err
	}
	return printNodes(nodes)
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
