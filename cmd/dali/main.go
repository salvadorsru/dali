package main

import (
	"fmt"
	"os"
)

var jsonOut bool

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := stripJSON(os.Args[2:])
	var err error
	switch os.Args[1] {
	case "inspect":
		err = cmdInspect(args)
	case "search":
		err = cmdSearch(args)
	case "node":
		err = cmdNode(args)
	case "context":
		err = cmdContext(args)
	case "children":
		err = cmdChildren(args)
	case "parent":
		err = cmdParent(args)
	case "path":
		err = cmdPath(args)
	case "components":
		err = cmdComponents(args)
	case "assets":
		err = cmdAssets(args)
	case "fonts":
		err = cmdFonts(args)
	case "query":
		err = cmdQuery(args)
	case "extract-assets":
		err = cmdExtractAssets(args)
	case "mcp":
		err = cmdMCP(args)
	case "help", "-h", "--help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "dali:", err)
		os.Exit(1)
	}
}

func stripJSON(args []string) []string {
	out := args[:0:0]
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		out = append(out, a)
	}
	return out
}

func usage() {
	fmt.Fprint(os.Stderr, `dali - Figma .fig knowledge engine

Usage:
  dali inspect <file.fig>
  dali search <file.fig> <query>
  dali node <file.fig> <id>
  dali context <file.fig> <id>
  dali children <file.fig> <id>
  dali parent <file.fig> <id>
  dali path <file.fig> <id>
  dali components <file.fig>
  dali assets <file.fig>
  dali fonts <file.fig>
  dali query <file.fig> <expression>
  dali extract-assets <file.fig>
  dali mcp <file.fig>

Flags:
  --json    output JSON
`)
}
