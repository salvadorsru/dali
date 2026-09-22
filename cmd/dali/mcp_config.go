package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/salvadorsru/dali/internal/output"
)

type configTarget struct {
	name   string
	path   string
	key    string
	global bool
	// stdio emits command/args; cline/roo add disabled + alwaysAllow; opencode
	// uses the local-dynamic command array with environment/timeout.
	stdio bool
	allow bool
	local bool
}

var configTargets = map[string]configTarget{
	"opencode": {name: "OpenCode", path: "opencode.json", key: "mcp", local: true},
	"cursor":   {name: "Cursor", path: filepath.Join(".cursor", "mcp.json"), key: "mcpServers"},
	"claude":   {name: "Claude Code", path: ".mcp.json", key: "mcpServers", stdio: true},
	"vscode":   {name: "VS Code", path: filepath.Join(".vscode", "mcp.json"), key: "servers", stdio: true},
	"cline":    {name: "Cline", path: filepath.Join(".cline", "mcp.json"), key: "mcpServers", allow: true},
	"roo":      {name: "Roo Code", path: filepath.Join(".roo", "mcp.json"), key: "mcpServers", allow: true},
	"windsurf": {name: "Windsurf", path: "mcp_config.json", key: "mcpServers", global: true},
}

// payload builds the config document for a target.
func (t configTarget) payload(exe, fig string, env map[string]string) map[string]any {
	var server map[string]any
	switch {
	case t.local:
		server = map[string]any{
			"type":        "local",
			"command":     []string{exe, fig, "mcp"},
			"environment": env,
			"enabled":     true,
			"timeout":     120000,
		}
	case t.allow:
		server = map[string]any{"command": exe, "args": []string{fig, "mcp"}, "env": env, "disabled": false, "alwaysAllow": []string{}}
	case t.stdio:
		server = map[string]any{"type": "stdio", "command": exe, "args": []string{fig, "mcp"}, "env": env}
	default:
		server = map[string]any{"command": exe, "args": []string{fig, "mcp"}, "env": env}
	}
	return map[string]any{t.key: map[string]any{"dali": server}}
}

// order lists the editor targets in a deterministic output order.
var order = []string{"opencode", "cursor", "claude", "vscode", "cline", "roo", "windsurf"}

// defaults lists the targets written when no editor is selected: every
// project-level config, but not the global Windsurf file.
var defaults = []string{"opencode", "cursor", "claude", "vscode", "cline", "roo"}

// cmdMCPConfig writes the MCP server configuration for a design file so an AI
// editor can load it. Configs are placed in the current directory, each editor
// in its own config file (Windsurf is global-only); --print sends them to
// stdout instead. Existing configs are merged, never replaced.
func cmdMCPConfig(args []string) error {
	fs := flag.NewFlagSet("mcp-config", flag.ContinueOnError)
	all := fs.Bool("all", false, "write configs for every supported editor, including Windsurf")
	selected := map[string]*bool{}
	for _, name := range order {
		selected[name] = fs.Bool(name, false, "write "+configTargets[name].name+" config")
	}
	src := fs.String("src", "", "path to the design file (.fig); defaults to the fig in the current directory")
	dir := fs.String("dir", "", "directory to write configs to; defaults to the current directory")
	print := fs.Bool("print", false, "print configs to stdout instead of writing files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fig := *src
	if fig == "" {
		fig = figFile
	}
	if fig == "" {
		found, err := detectFig(figDir())
		if err != nil {
			return errors.New("no design file: pass --src <file.fig>")
		}
		fig = found
	}
	exe := absPath(selfPath())
	fig = absPath(fig)
	base := absPath(*dir)
	if base == "" {
		base, _ = os.Getwd()
	}
	env := map[string]string{}

	var names []string
	if *all {
		names = order
	} else {
		for _, name := range order {
			if *selected[name] {
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			names = defaults
		}
	}

	for _, name := range names {
		t := configTargets[name]
		payload := t.payload(exe, fig, env)
		if *print {
			if err := output.JSON(os.Stdout, payload); err != nil {
				return err
			}
			continue
		}
		path := filepath.Join(base, t.path)
		if t.global {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path = filepath.Join(home, ".codeium", "windsurf", t.path)
		}
		if err := writeConfig(path, t.key, payload); err != nil {
			return err
		}
	}
	return nil
}

// writeConfig writes payload to path, merging it under the target's root key
// when the file already holds other servers or settings.
func writeConfig(path string, root string, payload map[string]any) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	doc := payload
	if data, err := os.ReadFile(path); err == nil {
		var existing map[string]any
		if err := json.Unmarshal(data, &existing); err == nil && existing != nil {
			doc = existing
			entries, _ := payload[root].(map[string]any)
			if entries != nil {
				base, _ := doc[root].(map[string]any)
				if base == nil {
					base = map[string]any{}
					doc[root] = base
				}
				for k, v := range entries {
					base[k] = v
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := output.JSON(f, doc); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", output.Success("wrote"), output.Bold(path))
	return nil
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
