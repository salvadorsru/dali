package quality

import (
	"math"
	"testing"

	"github.com/salvadorsru/dali/model"
)

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

func TestContrast(t *testing.T) {
	if got := contrast("#FFFFFF", "#000000"); math.Abs(got-21) > 0.01 {
		t.Fatalf("black/white = %v", got)
	}
	if got := contrast("#FFF", "#000"); got < 20 {
		t.Fatalf("3-digit = %v", got)
	}
	if got := contrast("#FFFFFFFF", "#000000"); math.Abs(got-21) > 0.3 {
		t.Fatalf("8-digit = %v", got)
	}
	if got := contrast("nope", "#000000"); got != 1 {
		t.Fatalf("invalid = %v, want 1", got)
	}
}

func TestLintRules(t *testing.T) {
	page := g(0, 1)
	card := size(mkNode(2, "FRAME", "Card", page, "a"), 100, 50)
	card["fillPaints"] = solid(1, 1, 1)

	text := mkNode(3, "TEXT", "Body", g(0, 2), "a")
	text["textData"] = map[string]any{"characters": "Hi"}
	text["fontSize"] = float32(12)
	text["fillPaints"] = solid(1, 1, 1) // white on white -> poor contrast

	generic := size(mkNode(4, "FRAME", "Frame 12", page, "b"), 10, 10)
	buttonA := size(mkNode(6, "FRAME", "Button A", page, "c"), 20, 20)
	buttonB := size(mkNode(7, "FRAME", "Button A", page, "d"), 20, 20)
	symbol := size(mkNode(8, "SYMBOL", "IconBtn", page, "e"), 24, 24)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		card, text, generic, buttonA, buttonB, symbol,
	)

	issues, err := Lint(doc, LintOptions{Preset: "recommended"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]Issue{}
	for _, i := range issues {
		got[i.Rule] = append(got[i.Rule], i)
	}
	for rule, id := range map[string]string{
		"text-contrast":    "0:3",
		"generic-name":     "0:4",
		"empty-frame":      "0:4",
		"duplicate-name":   "0:7",
		"unused-component": "0:8",
	} {
		list := got[rule]
		if len(list) == 0 {
			t.Fatalf("rule %q not reported: %+v", rule, got)
		}
		if list[0].Rule == "duplicate-name" {
			if list[0].NodeID != "0:6" && list[0].NodeID != "0:7" {
				t.Fatalf("duplicate-name on %q", list[0].NodeID)
			}
		} else if list[0].NodeID != id {
			t.Fatalf("rule %q on %q, want %q", rule, list[0].NodeID, id)
		}
	}
	for _, i := range got["text-contrast"] {
		if i.Severity != Warn {
			t.Fatalf("recommended severity = %v, want warn", i.Severity)
		}
	}
}

func TestLintAccessibilityPreset(t *testing.T) {
	page := g(0, 1)
	fill := solid(0.1, 0.1, 0.1)
	bg := size(mkNode(2, "FRAME", "Bg", page, "a"), 100, 100)
	bg["fillPaints"] = fill
	tiny := mkNode(3, "TEXT", "Tiny", g(0, 2), "a")
	tiny["textData"] = map[string]any{"characters": "x"}
	tiny["fontSize"] = float32(10)
	tiny["fillPaints"] = solid(1, 1, 1)
	generic := size(mkNode(4, "FRAME", "Frame 2", page, "b"), 10, 10)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		bg, tiny, generic,
	)

	issues, err := Lint(doc, LintOptions{Preset: "accessibility"})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range issues {
		if i.Severity != Error {
			t.Fatalf("%s severity = %v, want error", i.Rule, i.Severity)
		}
		if i.Rule == "generic-name" {
			t.Fatalf("quality rule leaked into accessibility preset")
		}
	}
}

func TestAnalyzeSpacing(t *testing.T) {
	page := g(0, 1)
	a := size(mkNode(2, "FRAME", "A", page, "a"), 50, 50)
	a["layoutMode"] = "VERTICAL"
	a["itemSpacing"] = float32(8)
	b := size(mkNode(3, "FRAME", "B", page, "b"), 50, 50)
	b["layoutMode"] = "HORIZONTAL"
	b["itemSpacing"] = float32(8)
	plain := size(mkNode(4, "FRAME", "C", page, "c"), 50, 50)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		a, b, plain,
	)

	got, err := Analyze(doc, "spacing", AnalyzeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	gaps := got.([]GapStat)
	if len(gaps) != 1 || gaps[0].Value != 8 || gaps[0].Count != 2 {
		t.Fatalf("spacing = %+v", gaps)
	}
}

func TestAnalyzeClusters(t *testing.T) {
	page := g(0, 1)
	var items []map[string]any
	items = append(items, mkNode(0, "DOCUMENT", "Doc", nil, ""), mkNode(1, "CANVAS", "Page", g(0, 0), "a"))
	for i := 0; i < 4; i++ {
		items = append(items, size(at(mkNode(int32(2+i), "FRAME", "Cell", page, "x"), 0, 0), 20, 20))
	}
	items = append(items, size(mkNode(30, "FRAME", "Other", page, "y"), 999, 3))

	got, err := Analyze(build(items...), "clusters", AnalyzeOptions{MinCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	clusters := got.([]Cluster)
	if len(clusters) != 1 || clusters[0].Count != 4 || clusters[0].Width != 20 {
		t.Fatalf("clusters = %+v", clusters)
	}
}

func TestAnalyzeOverlaps(t *testing.T) {
	page := g(0, 1)
	a := size(at(mkNode(2, "FRAME", "A", page, "a"), 0, 0), 100, 100)
	b := size(at(mkNode(3, "FRAME", "B", page, "b"), 50, 50), 100, 100)
	c := size(at(mkNode(4, "FRAME", "C", page, "c"), 300, 300), 20, 20)

	doc := build(
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		a, b, c,
	)

	got, err := Analyze(doc, "overlaps", AnalyzeOptions{MinRatio: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	overlaps := got.([]Overlap)
	if len(overlaps) != 1 {
		t.Fatalf("overlaps = %+v", overlaps)
	}
	if math.Abs(overlaps[0].Ratio-0.25) > 0.001 {
		t.Fatalf("overlap ratio = %v, want 0.25", overlaps[0].Ratio)
	}
}
