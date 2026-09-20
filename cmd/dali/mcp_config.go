package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/salvadorsru/dali/output"
)

// cmdMCPConfig prints the MCP server configuration for the current fig and
// binary, ready to paste into an AI editor.
func cmdMCPConfig(args []string) error {
	fs := flag.NewFlagSet("mcp-config", flag.ContinueOnError)
	opencode := fs.Bool("opencode", false, "emit OpenCode (opencode.json) config")
	vscode := fs.Bool("vscode", false, "emit VS Code (.vscode/mcp.json) config")
	fs.Bool("claude", false, "emit Claude/Cursor mcpServers config (default)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	exe := absPath(selfPath())
	fig := absPath(figFile)
	env := map[string]string{"DALI_EMBED_YES": "1"}

	switch {
	case *opencode:
		return output.JSON(os.Stdout, map[string]any{
			"mcp": map[string]any{
				"dali": map[string]any{
					"type":        "local",
					"command":     []string{exe, fig, "mcp"},
					"environment": env,
					"enabled":     true,
					"timeout":     120000,
				},
			},
		})
	case *vscode:
		return output.JSON(os.Stdout, map[string]any{
			"servers": map[string]any{
				"dali": map[string]any{
					"type":    "stdio",
					"command": exe,
					"args":    []string{fig, "mcp"},
					"env":     env,
				},
			},
		})
	default:
		return output.JSON(os.Stdout, map[string]any{
			"mcpServers": map[string]any{
				"dali": map[string]any{
					"command": exe,
					"args":    []string{fig, "mcp"},
					"env":     env,
				},
			},
		})
	}
}

func selfPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "dali"
	}
	return exe
}

func absPath(path string) string {
	if path == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
