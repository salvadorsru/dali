package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/salvadorsru/dali/assets"
	"github.com/salvadorsru/dali/mcp"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/query"
)

func open(path string) (*query.Service, error) {
	return query.Open(path)
}

func cmdSearch(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	text, err := arg(args, 1, "query")
	if err != nil {
		return err
	}
	svc, err := open(path)
	if err != nil {
		return err
	}
	defer svc.Close()

	results, err := svc.Search(text, 50)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, results)
	}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{r.ID, r.Type, truncate(r.Name, 40), truncate(r.Path, 60)})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "NAME", "PATH"}, rows)
	return nil
}

func cmdNode(args []string) error {
	_, id, svc, err := openWithID(args)
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
	path, id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()
	_ = path

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
	path, id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()
	_ = path

	nodes, err := svc.Children(id)
	if err != nil {
		return err
	}
	return printNodes(nodes)
}

func cmdParent(args []string) error {
	path, id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()
	_ = path

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
	path, id, svc, err := openWithID(args)
	if err != nil {
		return err
	}
	defer svc.Close()
	_ = path

	nodes, err := svc.Path(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, nodes)
	}
	for i, n := range nodes {
		indent := ""
		for j := 0; j < i; j++ {
			indent += "  "
		}
		prefix := "└── "
		if i == 0 {
			prefix = ""
		}
		fmt.Printf("%s%s%s: %s\n", indent, prefix, n.Type, n.Name)
	}
	return nil
}

func cmdComponents(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	svc, err := open(path)
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
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	svc, err := open(path)
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
		rows = append(rows, []string{a.Hash[:12], a.MIME, size, truncate(a.Name, 50), a.NodeID})
	}
	output.Table(os.Stdout, []string{"HASH", "MIME", "SIZE", "NAME", "NODE"}, rows)
	return nil
}

func cmdFonts(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	svc, err := open(path)
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
		rows = append(rows, []string{f.Family, f.Style, strconv.Itoa(f.Count)})
	}
	output.Table(os.Stdout, []string{"FAMILY", "STYLE", "COUNT"}, rows)
	return nil
}

func cmdQuery(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	expr, err := arg(args, 1, "expression")
	if err != nil {
		return err
	}
	svc, err := open(path)
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
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	svc, err := open(path)
	if err != nil {
		return err
	}
	defer svc.Close()

	refs, err := svc.Assets()
	if err != nil {
		return err
	}
	out, err := assets.Extract(path, refs)
	if err != nil {
		return err
	}
	if err := svc.UpdateAssetInfo(out); err != nil {
		return err
	}
	fmt.Printf("extracted %d assets to %s\n", len(out), assets.ImagesDir(path))
	return nil
}

func cmdMCP(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	svc, err := open(path)
	if err != nil {
		return err
	}
	defer svc.Close()
	return mcp.Serve(context.Background(), svc)
}

func openWithID(args []string) (string, string, *query.Service, error) {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return "", "", nil, err
	}
	id, err := arg(args, 1, "id")
	if err != nil {
		return "", "", nil, err
	}
	svc, err := open(path)
	if err != nil {
		return "", "", nil, err
	}
	return path, id, svc, nil
}

func printNodes(nodes []*model.Node) error {
	if jsonOut {
		return output.JSON(os.Stdout, nodes)
	}
	rows := make([][]string, 0, len(nodes))
	for _, n := range nodes {
		rows = append(rows, []string{n.ID, string(n.Type), truncate(n.Name, 50), truncate(n.Text, 40)})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "NAME", "TEXT"}, rows)
	return nil
}

func printNode(n *model.Node) {
	fmt.Printf("%s  %s  %q\n", n.ID, n.Type, n.Name)
	fmt.Printf("  page:   %s\n", n.PageID)
	if n.ParentID != "" {
		fmt.Printf("  parent: %s\n", n.ParentID)
	}
	fmt.Printf("  size:   %gx%g\n", n.Width, n.Height)
	fmt.Printf("  pos:    %g,%g\n", n.X, n.Y)
	fmt.Printf("  visible:%v  opacity:%g\n", n.Visible, n.Opacity)
	if n.Text != "" {
		fmt.Printf("  text:   %q\n", truncate(n.Text, 120))
	}
	if n.FontFamily != "" {
		fmt.Printf("  font:   %s %s %g\n", n.FontFamily, n.FontStyle, n.FontSize)
	}
	if n.ComponentID != "" {
		fmt.Printf("  component: %s\n", n.ComponentID)
	}
}

func printContext(ctx *query.Context) {
	fmt.Println("Path:")
	for i, n := range ctx.Path {
		fmt.Printf("  %s%s: %s\n", indent(i), n.Type, n.Name)
	}
	fmt.Printf("Node: %s  %s  %q\n", ctx.Node.ID, ctx.Node.Type, ctx.Node.Name)
	if ctx.Node.Text != "" {
		fmt.Printf("Text: %q\n", truncate(ctx.Node.Text, 200))
	}
	if ctx.Component != nil {
		fmt.Printf("Component: %s  %s\n", ctx.Component.ID, ctx.Component.Name)
	}
	if len(ctx.Children) > 0 {
		fmt.Printf("Children: %d\n", len(ctx.Children))
		for _, c := range ctx.Children {
			fmt.Printf("  %s  %s  %s\n", c.ID, c.Type, truncate(c.Name, 40))
		}
	}
	if len(ctx.Styles) > 0 {
		fmt.Printf("Styles: %d\n", len(ctx.Styles))
	}
	if len(ctx.Assets) > 0 {
		fmt.Printf("Assets: %d\n", len(ctx.Assets))
		for _, a := range ctx.Assets {
			fmt.Printf("  %s  %s  %s\n", a.Hash[:12], a.MIME, a.Name)
		}
	}
}

func indent(i int) string {
	out := ""
	for j := 0; j < i; j++ {
		out += "  "
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
