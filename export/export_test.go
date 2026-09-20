package export

import (
	"strings"
	"testing"

	"github.com/salvadorsru/dali/model"
)

func page(local int32, name string) map[string]any {
	m := mkNode(local, "CANVAS", name, g(0, 0), "x")
	return m
}

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

func size(m map[string]any, w, h float32) map[string]any {
	if m["size"] == nil {
		m["size"] = map[string]any{}
	}
	s := m["size"].(map[string]any)
	s["x"], s["y"] = w, h
	return m
}

func at(m map[string]any, x, y float32) map[string]any {
	m["transform"] = map[string]any{"m00": float32(1), "m01": float32(0), "m10": float32(0), "m11": float32(1), "m02": x, "m12": y}
	return m
}

func solid(r, g, b float32) []any {
	return []any{map[string]any{
		"type":  "SOLID",
		"color": map[string]any{"r": r, "g": g, "b": b, "a": float32(1)},
	}}
}

func build(nodes ...map[string]any) *model.Document {
	items := make([]any, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, n)
	}
	return model.Build(items, nil)
}

func TestSVGPageColumns(t *testing.T) {
	card := size(mkNode(10, "FRAME", "Card", g(0, 1), "a"), 100, 40)
	card["fillPaints"] = solid(1, 0, 0)
	hero := size(mkNode(11, "FRAME", "Hero", g(0, 2), "a"), 200, 80)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page 1", g(0, 0), "a"),
		mkNode(2, "CANVAS", "Page 2", g(0, 0), "b"),
		card, hero,
	)

	svg, err := SVG(doc, nil, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := string(svg)
	if !strings.HasPrefix(out, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300.00 80.00" fill="none">`) {
		t.Fatalf("header = %s", out[:100])
	}
	if !strings.Contains(out, `fill="#FF0000"`) {
		t.Fatal("missing frame fill")
	}
	if count := strings.Count(out, "<rect"); count != 2 {
		t.Fatalf("rects = %d, want 2", count)
	}
}

func TestSVGSingleRoot(t *testing.T) {
	frame := size(mkNode(2, "FRAME", "Card", g(0, 1), "a"), 100, 40)
	frame["fillPaints"] = solid(0, 0, 1)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		at(frame, 10, 20),
	)

	svg, err := SVG(doc, nil, ExportOptions{RootID: "0:2"})
	if err != nil {
		t.Fatal(err)
	}
	out := string(svg)
	if !strings.Contains(out, `viewBox="0 0 100.00 40.00"`) {
		t.Fatalf("bounds = %s", out[:80])
	}
	// the root draws at its own offset within the subtree space
	if !strings.Contains(out, `x="10.00" y="20.00" width="100.00"`) {
		t.Fatalf("root rect: %s", out)
	}
}

func TestSVGTextAndClip(t *testing.T) {
	frame := size(mkNode(2, "FRAME", "Card", g(0, 1), "a"), 100, 40)
	frame["clipContent"] = true
	text := mkNode(3, "TEXT", "Title", g(0, 2), "a")
	text["textData"] = map[string]any{"characters": "Hello <World>"}
	text["fontSize"] = float32(14)
	text["fillPaints"] = solid(0.1, 0.1, 0.1)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		frame, text,
	)

	svg, err := SVG(doc, nil, ExportOptions{PageID: "0:1"})
	if err != nil {
		t.Fatal(err)
	}
	out := string(svg)
	if !strings.Contains(out, `clip-path="url(#clip-0-2)"`) {
		t.Fatal("missing clip path")
	}
	if !strings.Contains(out, "Hello &lt;World&gt;") {
		t.Fatal("text not escaped")
	}
	if !strings.Contains(out, `font-size="14.00"`) {
		t.Fatal("font size missing")
	}
}

func TestHTML(t *testing.T) {
	n := size(mkNode(2, "FRAME", "Card", g(0, 1), "a"), 10, 10)
	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		n,
	)
	html, err := HTML(doc, nil, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	if !strings.HasPrefix(out, "<!doctype html>") || !strings.Contains(out, "<svg") || !strings.Contains(out, "</html>") {
		t.Fatalf("html = %s", out[:120])
	}
}