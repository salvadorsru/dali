package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/query"
)

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