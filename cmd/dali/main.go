package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/salvadorsru/dali/internal/output"
)

var (
	jsonOut bool
	figFile string
)

var commands = map[string]func([]string) error{
	"inspect":         cmdInspect,
	"search":          cmdSearch,
	"semantic-search": cmdSemanticSearch,
	"node":            cmdNode,
	"context":         cmdContext,
	"children":        cmdChildren,
	"parent":          cmdParent,
	"path":            cmdPath,
	"subtree":         cmdSubtree,
	"components":      cmdComponents,
	"usages":          cmdUsages,
	"dependents":      cmdDependents,
	"assets":          cmdAssets,
	"fonts":           cmdFonts,
	"variables":       cmdVariables,
	"styles":          cmdStyles,
	"query":           cmdQuery,
	"index":           cmdIndex,
	"extract-assets":  cmdExtractAssets,
	"lint":            cmdLint,
	"analyze":         cmdAnalyze,
	"diff":            cmdDiff,
	"generate":        cmdGenerate,
	"mcp":             cmdMCP,
	"mcp-config":      cmdMCPConfig,
	"serve":           cmdServe,
	"stop":            cmdStop,
	"clean":           cmdClean,
	"version":         cmdVersion,
}

// figless commands do not operate on a fig file.
var figless = map[string]bool{"serve": true, "stop": true, "clean": true, "version": true, "mcp-config": true, "diff": true}

func main() {
	args := stripGlobals(os.Args[1:])
	if len(args) == 0 {
		hint()
		usage()
		return
	}
	if isHelp(args[0]) {
		usage()
		return
	}
	if args[0] == "-v" || args[0] == "--version" {
		if err := cmdVersion(nil); err != nil {
			fail(err)
		}
		return
	}

	var name string
	var rest []string
	if _, ok := commands[args[0]]; ok {
		name = args[0]
		rest = args[1:]
	} else {
		figFile = args[0]
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "dali: missing command after %q\n\n", args[0])
			usage()
			os.Exit(2)
		}
		name = args[1]
		rest = args[2:]
	}
	if isHelp(name) {
		usage()
		return
	}
	if _, ok := commands[name]; !ok {
		fail(fmt.Errorf("unknown command %q", name))
	}
	// mcp resolves its own mode from the raw argument: a folder stays a folder
	// (context mode), only a design file is collapsed to a single design.
	if !figless[name] && name != "mcp" {
		if figFile == "" {
			figFile = figDir()
		}
		info, err := os.Stat(figFile)
		if err != nil {
			fail(fmt.Errorf("cannot read %q: %w", figFile, err))
		} else if info.IsDir() {
			found, err := detectFig(figFile)
			if err != nil {
				fail(err)
			} else {
				figFile = found
			}
		}
	}
	if err := commands[name](rest); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%s %v\n", output.Error("dali:"), err)
	os.Exit(1)
}

// hint greets a new user with the fig it found in the current directory.
func hint() {
	fmt.Fprintf(os.Stderr, "%s %s\n", output.Success("Found"), output.Bold(figDir()))
	fmt.Fprintf(os.Stderr, "  %s\n", output.Dim("dali inspect"))
	fmt.Fprintf(os.Stderr, "  %s\n", output.Dim(`dali search "checkout"`))
	fmt.Fprintf(os.Stderr, "  %s\n", output.Dim(`dali semantic-search "payment screen"`))
	fmt.Fprintf(os.Stderr, "  %s\n\n", output.Dim("dali mcp-config"))
}

func isHelp(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

// figDir is where .fig/.deck files live: DALI_FIG_DIR or the working directory.
func figDir() string {
	if v := os.Getenv("DALI_FIG_DIR"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// mcpMode picks the server mode from the fig argument. A design file serves
// that one design; a directory, or no argument at all, serves every design
// found under it.
func mcpMode(file string) (fig, root string) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".fig", ".deck":
		return file, ""
	}
	if file != "" {
		return "", file
	}
	return "", figDir()
}

func detectFig(dir string) (string, error) {
	if dir == "" {
		dir = figDir()
	}
	var matches []string
	for _, pattern := range []string{"*.fig", "*.deck"} {
		found, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil {
			matches = append(matches, found...)
		}
	}
	where := dir
	if dir == "." {
		where = "the current directory"
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no .fig file in %s; pass one explicitly: dali <file.fig> <command>", where)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple fig files found in %s (%s); pass one explicitly", where, strings.Join(matches, ", "))
	}
}

func stripGlobals(args []string) []string {
	out := args[:0:0]
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--no-color":
			output.SetEnabled(false)
		case "--quiet", "-q":
			output.SetQuiet(true)
		default:
			out = append(out, a)
		}
	}
	return out
}

func usage() {
	fmt.Fprint(os.Stderr, `dali - Figma .fig knowledge engine

Usage:
  dali [<file.fig>|<dir>] <command> [args]

The file (or its directory) is optional: if omitted, the single .fig in the
current directory is used.

Commands:
  inspect                     Document counts
  search <query>              Full-text search (FTS5) [--limit N] [--hybrid]
  semantic-search <query>     Vector search [--limit N] [--type T] [--page P] [--hybrid]
  node <id>                   Show one node
  context <id>                Path, parent, children, component, styles, assets
  children <id>               Direct children
  parent <id>                 Parent node
  path <id>                   Root-to-node path
  subtree <id> [--depth N]    Subtree with depth, styles and images per node (0 = full)
  components                  List components
  usages <id>                 List every page placement of a component
  dependents <id>             Impact: the containers (screens) that place a component
  assets                      List image assets
  fonts                       List fonts in use
  variables                   List design tokens with values per mode
  styles                      List style definitions (name, kind, value, uses)
  query <expression>          Structural query DSL (type=, name~, text~, font=, page~,
                              component=, visible=, width>=, height<=, descendants()...
                              terms combine with AND); a leading / uses the XPath
                              subset (/ // * @name= @name~ contains()) [--limit N]
  lint [--preset P]          Design + accessibility checks (recommended, strict, accessibility)
  analyze <kind>             Geometry stats: spacing | clusters | overlaps
                              (--min-ratio, --min-count, --limit)
  diff <old> <new>           Compare two .fig versions: added, removed, modified
  index [--embeddings]       Build the structural index; --embeddings adds the vector index
                              (--force rebuilds even if up to date)
  extract-assets              Write images to <base>.assets/images
  generate                    Generate semantic HTML from components
                               (--out DIR, --id ID | --name NAME,
                               --tokens css|style-dict|figma, --style inline|tailwind)
  mcp                         Run the MCP server over stdio. The argument picks
                              the mode: dali <dir> mcp = context, every design
                              under the directory is a project and clients use
                              list_figs + use_figma; dali <file.fig> mcp = figma,
                              that one design is mounted, list_figs/use_figma do
                              not exist and tools never take "file"
  mcp-config [--src FILE]     Write MCP config per editor (--opencode, --cursor,
                              --claude, --vscode, --cline, --roo, --windsurf;
                              --all includes Windsurf's global file, default all
                              but --windsurf, --print to stdout) into the current
                              directory (--dir to override), merged with any
                              existing config
  serve                       Keep the embedding server warm (reused later)
  stop                        Stop the managed embedding server
  clean                       Stop the server and delete the downloaded engine + model

Flags:
  --json       output JSON
  --yes        allow downloads without prompting
  --no-color   disable colors
  --quiet, -q  hide progress and status

Environment:
  DALI_EMBED_PROVIDER  auto | ollama | llama | remote | off  (default auto)
  DALI_EMBEDDINGS      set to 1 to enable embeddings in the MCP server (opt-in)
  DALI_LLAMA_URL       use an existing llama-server
  DALI_LLAMA_MODEL     managed model repo (default ggml-org/embeddinggemma-300M-GGUF)
  DALI_LLAMA_TAG       pinned llama.cpp release tag
  DALI_LLAMA_PARALLEL  managed server slots (default: from CPU count)
  DALI_LLAMA_THREADS   managed server threads (default: from CPU count)
  DALI_EMBED_CONCURRENCY  parallel embedding requests (default: from CPU count)
  DALI_EMBED_BATCH     texts per embedding request (default 64)
  DALI_LLAMA_KEEP      set to 0 to stop the server after each command
  DALI_CACHE_DIR       download cache directory
  DALI_EMBED_URL       Ollama endpoint (default http://localhost:11434)
  DALI_EMBED_MODEL     Ollama model (default bge-m3)
  DALI_EMBED_YES       allow downloads without prompting

Examples:
  dali design.fig inspect
  dali design.fig semantic-search "payment screen" --limit 10
  dali design.fig lint --preset accessibility
  dali design.fig analyze overlaps
  dali diff old.fig new.fig
  dali design.fig generate --out ./src/components
  dali design.fig generate --id 1:123 --out ./components
  dali design.fig generate --name "VideoCaption" --out ./components
  dali design.fig generate --style tailwind
  dali inspect                 # uses ./design.fig
  dali . index                 # indexes the .fig in this directory
  dali ./designs mcp           # MCP context mode: every design under ./designs
  dali ./designs/app.fig mcp   # MCP figma mode: only that design
`)
}
