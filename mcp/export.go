package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/export"
)

type exportInput struct {
	File   string `json:"file,omitempty" jsonschema:"fig file to export; defaults to the one dali was started with"`
	Format string `json:"format" jsonschema:"svg or html"`
	Page   string `json:"page,omitempty" jsonschema:"export only this page by id"`
	Root   string `json:"root,omitempty" jsonschema:"export only this node subtree by id"`
}

type exportOutput struct {
	Format string `json:"format"`
	Path   string `json:"path"`
	Size   int    `json:"size"`
	// Data carries the full export when it fits in a reply; large documents
	// are written to Path and read from disk instead.
	Data string `json:"data,omitempty"`
}

func (h *handlers) exportFigma(ctx context.Context, req *mcp.CallToolRequest, in exportInput) (*mcp.CallToolResult, exportOutput, error) {
	note := newNotifier(ctx, req)
	doc, file, err := h.load(in.File, note)
	if err != nil {
		return nil, exportOutput{}, err
	}
	defer file.Close()

	opts := export.ExportOptions{PageID: in.Page, RootID: in.Root}
	var data []byte
	switch in.Format {
	case "svg":
		data, err = export.SVG(doc, file, opts)
	case "html":
		data, err = export.HTML(doc, file, opts)
	default:
		return nil, exportOutput{}, fmt.Errorf("unknown format %q (svg, html)", in.Format)
	}
	if err != nil {
		return nil, exportOutput{}, err
	}
	abs, err := h.resolve(in.File)
	if err != nil {
		return nil, exportOutput{}, err
	}
	path, err := writeExport(abs, in.Format, opts, data)
	if err != nil {
		return nil, exportOutput{}, err
	}
	out := exportOutput{Format: in.Format, Path: path, Size: len(data)}
	if len(data) <= 1<<20 {
		out.Data = string(data)
	}
	return nil, out, nil
}

func writeExport(abs, format string, opts export.ExportOptions, data []byte) (string, error) {
	dir := filepath.Join(filepath.Dir(abs), filepath.Base(abs)+".assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	switch {
	case opts.RootID != "":
		base += "-" + sanitizeID(opts.RootID)
	case opts.PageID != "":
		base += "-" + sanitizeID(opts.PageID)
	}
	path := filepath.Join(dir, base+"."+format)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeID(id string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
}
