package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/output"
)

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
		depth := i
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
