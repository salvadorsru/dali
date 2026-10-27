package mcp

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// pruned trees never hold designs worth listing.
var pruned = map[string]bool{".git": true, "node_modules": true, "dist": true, ".dali": true}

// entry is one discovered design: the identity tools use to name it and the
// absolute path used to open it.
type entry struct {
	name string
	path string
}

type figsInput struct {
	Name    string `json:"name,omitempty" jsonschema:"optional project name substring filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max projects to list, default 50"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"rescan the designs directory before listing"`
}

// projectRef names a discovered design. The absolute path is only reported for
// the active project, so listing stays cheap in tokens.
type projectRef struct {
	Name string `json:"name" jsonschema:"project identity to pass to use_figma"`
	Path string `json:"path,omitempty" jsonschema:"absolute path, only for the active project"`
}

type figsOutput struct {
	Projects  []projectRef `json:"projects"`
	Active    string       `json:"active,omitempty" jsonschema:"active project identity"`
	Total     int          `json:"total" jsonschema:"projects matching the filter"`
	Truncated bool         `json:"truncated,omitempty" jsonschema:"more projects matched than limit"`
}

func (h *handlers) listFigs(_ context.Context, _ *mcp.CallToolRequest, in figsInput) (*mcp.CallToolResult, figsOutput, error) {
	found := h.designs()
	if in.Refresh {
		found = h.rescan()
	}
	h.mu.Lock()
	active := h.active
	h.mu.Unlock()

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	needle := strings.ToLower(strings.TrimSpace(in.Name))
	matched := make([]entry, 0, len(found))
	for _, e := range found {
		if needle == "" || strings.Contains(strings.ToLower(e.name), needle) {
			matched = append(matched, e)
		}
	}
	page := matched
	if len(page) > limit {
		page = page[:limit]
	}

	out := figsOutput{Projects: make([]projectRef, 0, len(page)), Total: len(matched), Truncated: len(matched) > len(page)}
	if active != "" {
		out.Active = identity(h.root, active)
	}
	for _, e := range page {
		ref := projectRef{Name: e.name}
		if e.path == active {
			ref.Path = e.path
		}
		out.Projects = append(out.Projects, ref)
	}
	return nil, out, nil
}

// useInput names the design to select for the rest of the session.
type useInput struct {
	Name string `json:"name" jsonschema:"project name from list_figs, e.g. woments/woments"`
}

type useOutput struct {
	Name  string `json:"name" jsonschema:"confirmed project identity"`
	Path  string `json:"path" jsonschema:"absolute path of the selected design"`
	Total int    `json:"total" jsonschema:"designs under the designs directory"`
}

func (h *handlers) useFigma(_ context.Context, _ *mcp.CallToolRequest, in useInput) (*mcp.CallToolResult, useOutput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, useOutput{}, errors.New("pass a project name from list_figs")
	}
	found := h.designs()
	path, ok := match(found, h.root, name)
	if !ok {
		found = h.rescan()
		path, ok = match(found, h.root, name)
	}
	if !ok {
		return nil, useOutput{}, h.miss(name, found)
	}
	h.mu.Lock()
	h.active = path
	h.mu.Unlock()
	return nil, useOutput{Name: identity(h.root, path), Path: path, Total: len(found)}, nil
}

// discover walks root recursively and returns every .fig/.deck below it with
// absolute paths, deduplicated and sorted by identity.
func discover(root string) []entry {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	var paths []string
	seen := map[string]bool{}
	_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != abs && pruned[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".fig", ".deck":
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
		return nil
	})
	count := make(map[string]int, len(paths))
	for _, p := range paths {
		count[identity(abs, p)]++
	}
	out := make([]entry, 0, len(paths))
	for _, p := range paths {
		name := identity(abs, p)
		if count[name] > 1 {
			name += strings.ToLower(filepath.Ext(p))
		}
		out = append(out, entry{name: name, path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// identity names a design by its path relative to root, in slash form and
// without the extension, so the name survives moving the whole tree. Designs
// that would collide share a base name and keep the extension in both
// identities (woments.fig and woments.deck): discover never emits a duplicate
// name, and neither design shadows the other.
func identity(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	return filepath.ToSlash(rel)
}
