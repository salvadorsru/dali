package model

import "testing"

func g(session, local int32) map[string]any {
	return map[string]any{"sessionID": session, "localID": local}
}

func mkNode(local int32, typ, name string, parent map[string]any, position string) map[string]any {
	m := map[string]any{
		"guid":  g(0, local),
		"phase": "CREATED",
		"type":  typ,
		"name":  name,
	}
	if parent != nil {
		m["parentIndex"] = map[string]any{"guid": parent, "position": position}
	}
	return m
}

func TestBuildTree(t *testing.T) {
	docGUID := g(0, 0)
	pageGUID := g(0, 1)
	frameGUID := g(0, 2)

	frame := mkNode(2, "FRAME", "Card", pageGUID, "a")
	frame["size"] = map[string]any{"x": float32(100), "y": float32(50)}
	frame["transform"] = map[string]any{"m02": float32(10), "m12": float32(20)}

	text := mkNode(3, "TEXT", "Title", frameGUID, "a")
	text["textData"] = map[string]any{"characters": "Hi"}
	text["fontName"] = map[string]any{"family": "Inter", "style": "Regular"}
	text["fontSize"] = float32(14)

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", docGUID, "a"),
		frame,
		text,
		mkNode(4, "INSTANCE", "Copy", frameGUID, "b"),
	})

	if doc.Name != "Doc" {
		t.Fatalf("document name = %q", doc.Name)
	}
	if len(doc.Pages) != 1 || doc.Pages[0].Name != "Page" {
		t.Fatalf("pages = %+v", doc.Pages)
	}
	if got := len(doc.Children("0:2")); got != 2 {
		t.Fatalf("children = %d, want 2", got)
	}
	if p := doc.Parent("0:3"); p == nil || p.ID != "0:2" {
		t.Fatalf("parent = %+v", p)
	}
	if got := len(doc.Descendants("0:1")); got != 3 {
		t.Fatalf("descendants = %d, want 3", got)
	}
	path := doc.Path("0:3")
	if len(path) != 4 || path[0].Type != TypeDocument || path[3].Name != "Title" {
		t.Fatalf("path = %+v", path)
	}
	n, _ := doc.Node("0:3")
	if n.PageID != "0:1" || n.Text != "Hi" || n.FontFamily != "Inter" {
		t.Fatalf("node = %+v", n)
	}
	if len(doc.Fonts()) != 1 || doc.Fonts()[0].Family != "Inter" {
		t.Fatalf("fonts = %+v", doc.Fonts())
	}
	if doc.Counts()[TypeText] != 1 {
		t.Fatalf("counts = %+v", doc.Counts())
	}
}

func TestBuildSkipsRemoved(t *testing.T) {
	removed := mkNode(1, "FRAME", "Gone", nil, "a")
	removed["phase"] = "REMOVED"
	doc := Build([]any{removed})
	if len(doc.All()) != 0 {
		t.Fatalf("removed node kept: %+v", doc.All())
	}
}

func TestStyleDefs(t *testing.T) {
	fill := mkNode(1, "ROUNDED_RECTANGLE", "Brand/Primary", nil, "")
	fill["styleType"] = "FILL"
	fill["key"] = "k-fill"
	fill["fillPaints"] = []any{map[string]any{
		"type":   "SOLID",
		"color":  map[string]any{"r": float32(1), "g": float32(0.5), "b": float32(0), "a": float32(1)},
		"visible": true,
	}}

	text := mkNode(2, "TEXT", "Body/Regular", nil, "")
	text["styleType"] = "TEXT"
	text["key"] = "k-text"
	text["fontName"] = map[string]any{"family": "Inter", "style": "Regular"}
	text["fontSize"] = float32(16)
	text["fillPaints"] = []any{map[string]any{
		"type":   "SOLID",
		"color":  map[string]any{"r": float32(0), "g": float32(0), "b": float32(0), "a": float32(1)},
		"visible": true,
	}}

	dup := mkNode(3, "ROUNDED_RECTANGLE", "Old Anchor", nil, "")
	dup["styleType"] = "FILL"
	dup["key"] = "k-fill"

	applied := mkNode(4, "FRAME", "Card", nil, "")
	applied["styleIdForStrokeFill"] = map[string]any{"assetRef": map[string]any{"key": "k-fill"}}

	doc := Build([]any{fill, text, dup, applied})
	defs := doc.StyleDefs()

	if len(defs) != 2 {
		t.Fatalf("defs = %d, want 2", len(defs))
	}
	byKey := map[string]StyleDef{}
	for _, d := range defs {
		byKey[d.Key] = d
	}
	fd := byKey["k-fill"]
	if fd.Kind != "FILL" || fd.Name != "Brand/Primary" || fd.NodeID != "0:1" {
		t.Fatalf("fill def = %+v", fd)
	}
	if fd.Summary != "#FF8000" {
		t.Fatalf("fill summary = %q", fd.Summary)
	}
	if fd.Usage != 1 {
		t.Fatalf("fill usage = %d, want 1", fd.Usage)
	}
	if fd.Raw == "" {
		t.Fatal("raw not set")
	}
	td := byKey["k-text"]
	if td.Kind != "TEXT" || td.Summary != "Inter Regular · 16px · #000000" {
		t.Fatalf("text def = %+v summary %q", td, td.Summary)
	}
}
