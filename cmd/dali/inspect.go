package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
)

func cmdInspect(args []string) error {
	doc, err := load(figFile)
	if err != nil {
		return err
	}
	counts := doc.Counts()
	fmt.Println(output.Title(doc.Name))
	field("pages:", fmt.Sprintf("%d", len(doc.Pages)))
	field("nodes:", fmt.Sprintf("%d", len(doc.All())))
	field("components:", fmt.Sprintf("%d", len(doc.Components())))
	field("instances:", fmt.Sprintf("%d", len(doc.Instances())))
	field("images:", fmt.Sprintf("%d", len(doc.Assets())))
	field("fonts:", fmt.Sprintf("%d", len(doc.Fonts())))
	field("styles:", fmt.Sprintf("%d", len(doc.Styles())))
	fmt.Println()
	fmt.Println(output.Title("Node types"))
	printCounts(counts)
	return nil
}

func printCounts(counts map[model.NodeType]int) {
	types := make([]model.NodeType, 0, len(counts))
	for t := range counts {
		types = append(types, t)
	}
	sortTypes(types, counts)
	rows := make([][]string, 0, len(types))
	for _, t := range types {
		rows = append(rows, []string{output.Type(string(t)), strconv.Itoa(counts[t])})
	}
	output.Table(os.Stdout, []string{"TYPE", "COUNT"}, rows)
}

func sortTypes(types []model.NodeType, counts map[model.NodeType]int) {
	sort.Slice(types, func(i, j int) bool {
		if counts[types[i]] != counts[types[j]] {
			return counts[types[i]] > counts[types[j]]
		}
		return types[i] < types[j]
	})
}
