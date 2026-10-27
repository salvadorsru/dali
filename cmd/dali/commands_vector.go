package main

import (
	"fmt"
	"os"

	"github.com/salvadorsru/dali/internal/output"
)

// cmdVector prints the exact vector resources under a node, ready to paste.
func cmdVector(args []string) error {
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
	vectors, err := svc.Vectors(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, vectors)
	}
	if len(vectors) == 0 {
		fmt.Println("no vector geometry under this node")
		return nil
	}
	for _, v := range vectors {
		fmt.Printf("%s %s  %s\n", output.ID(v.ID), v.Name, v.ViewBox)
		if v.Source != "" {
			fmt.Printf("  from %s\n", v.Source)
		}
		for _, o := range v.Overrides {
			fmt.Printf("  override %s: %v -> %v\n", o.Field, o.From, o.To)
		}
		fmt.Printf("  %s\n", v.SVG())
	}
	return nil
}
