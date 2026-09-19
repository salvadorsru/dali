package main

import (
	"fmt"
	"sort"

	"github.com/salvadorsru/dali/model"
)

func cmdInspect(args []string) error {
	path, err := arg(args, 0, "file.fig")
	if err != nil {
		return err
	}
	doc, err := load(path)
	if err != nil {
		return err
	}
	counts := doc.Counts()
	fmt.Printf("Document:   %s\n", doc.Name)
	fmt.Printf("Pages:      %d\n", len(doc.Pages))
	fmt.Printf("Nodes:      %d\n", len(doc.All()))
	fmt.Printf("Components: %d\n", len(doc.Components()))
	fmt.Printf("Instances:  %d\n", len(doc.Instances()))
	fmt.Printf("Images:     %d\n", len(doc.Assets()))
	fmt.Printf("Fonts:      %d\n", len(doc.Fonts()))
	fmt.Printf("Styles:     %d\n", len(doc.Styles()))
	fmt.Println()
	fmt.Println("Node types:")
	printCounts(counts)
	return nil
}

func printCounts(counts map[model.NodeType]int) {
	types := make([]model.NodeType, 0, len(counts))
	for t := range counts {
		types = append(types, t)
	}
	sortTypes(types, counts)
	for _, t := range types {
		fmt.Printf("  %-20s %d\n", t, counts[t])
	}
}

func sortTypes(types []model.NodeType, counts map[model.NodeType]int) {
	sort.Slice(types, func(i, j int) bool {
		if counts[types[i]] != counts[types[j]] {
			return counts[types[i]] > counts[types[j]]
		}
		return types[i] < types[j]
	})
}
