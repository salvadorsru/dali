package model

import (
	"math"
	"strings"
	"testing"
)

func TestInlineStyles(t *testing.T) {
	fill := map[string]any{
		"type":  "SOLID",
		"color": map[string]any{"r": float32(1), "g": float32(0.5), "b": float32(0), "a": float32(1)},
	}

	fillNode := mkNode(2, "FRAME", "Card", g(0, 1), "a")
	fillNode["fillPaints"] = []any{fill}

	textNode := mkNode(3, "TEXT", "Body", g(0, 1), "b")
	textNode["textData"] = map[string]any{"characters": "Hi"}
	textNode["fontName"] = map[string]any{"family": "Onest", "style": "Medium"}
	textNode["fontSize"] = float32(40)
	textNode["fillPaints"] = []any{fill}

	dup := mkNode(4, "FRAME", "Card Copy", g(0, 1), "c")
	dup["fillPaints"] = []any{fill}

	invis := mkNode(5, "FRAME", "No Fill", g(0, 1), "d")
	invis["fillPaints"] = []any{map[string]any{
		"type": "SOLID", "visible": false,
		"color": map[string]any{"r": float32(1), "g": float32(0), "b": float32(0), "a": float32(1)},
	}}

	named := mkNode(6, "FRAME", "Named", g(0, 1), "e")
	named["styleIdForFill"] = map[string]any{"assetRef": map[string]any{"key": "k-brand"}}
	named["fillPaints"] = []any{fill}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		fillNode, textNode, dup, invis, named,
	}, nil)

	usagesFor := func(id string) []Style {
		var out []Style
		for _, s := range doc.StyleUsages() {
			if s.NodeID == id {
				out = append(out, s)
			}
		}
		return out
	}

	if got := usagesFor("0:2"); len(got) != 1 || got[0].Kind != "FILL" || got[0].Key == "" {
		t.Fatalf("inline fill usages = %+v", got)
	}
	text := usagesFor("0:3")
	if len(text) != 2 {
		t.Fatalf("inline text usages = %+v", text)
	}
	var textKind *Style
	for _, s := range text {
		if s.Kind == "TEXT" {
			textKind = &s
		} else if s.Kind != "FILL" {
			t.Fatalf("unexpected inner text usage kind: %+v", s)
		}
	}
	if textKind == nil || textKind.Key == "" {
		t.Fatalf("inline text usage missing: %+v", text)
	}
	if got := usagesFor("0:4"); len(got) != 1 || got[0].Key != usagesFor("0:2")[0].Key {
		t.Fatalf("dedupe fill usages = %+v", got)
	}
	if got := usagesFor("0:5"); len(got) != 0 {
		t.Fatalf("invisible paint emitted usage: %+v", got)
	}
	if got := usagesFor("0:6"); len(got) != 1 || got[0].Key != "k-brand" {
		t.Fatalf("named style not preferred: %+v", got)
	}

	byKey := map[string]StyleDef{}
	for _, d := range doc.StyleDefs() {
		byKey[d.Key] = d
	}
	if len(byKey) != 2 {
		t.Fatalf("defs = %d, want 2: %+v", len(byKey), byKey)
	}
	fillKey := usagesFor("0:2")[0].Key
	fd := byKey[fillKey]
	if fd.Kind != "FILL" || fd.Summary != "#FF8000" || fd.Name != "#FF8000" || fd.Usage != 3 {
		t.Fatalf("fill def = %+v", fd)
	}
	td := byKey[textKind.Key]
	if td.Kind != "TEXT" || td.Summary != "Onest Medium · 40px · #FF8000" {
		t.Fatalf("text def = %+v summary %q", td, td.Summary)
	}
}

func TestInlineLayoutStyle(t *testing.T) {
	frame := mkNode(2, "FRAME", "List", g(0, 1), "a")
	frame["size"] = map[string]any{"x": float32(390), "y": float32(700)}
	frame["layoutMode"] = "VERTICAL"
	frame["paddingLeft"] = float32(20)
	frame["paddingRight"] = float32(20)
	frame["paddingTop"] = float32(16)
	frame["paddingBottom"] = float32(16)
	frame["itemSpacing"] = float32(8)
	frame["cornerRadius"] = float32(12)

	plain := mkNode(3, "FRAME", "Static", g(0, 1), "b")

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		frame, plain,
	}, nil)

	usagesFor := func(id string) []Style {
		var out []Style
		for _, s := range doc.StyleUsages() {
			if s.NodeID == id {
				out = append(out, s)
			}
		}
		return out
	}

	list := usagesFor("0:2")
	if len(list) != 1 || list[0].Kind != "LAYOUT" {
		t.Fatalf("layout usages = %+v", list)
	}
	if got := usagesFor("0:3"); len(got) != 0 {
		t.Fatalf("plain frame emitted usage: %+v", got)
	}

	byKey := map[string]StyleDef{}
	for _, d := range doc.StyleDefs() {
		byKey[d.Key] = d
	}
	d := byKey[list[0].Key]
	if d.Kind != "LAYOUT" || d.Summary != "layout vertical · gap 8 · padding 16 20 · radius 12" {
		t.Fatalf("layout def = %+v summary %q", d, d.Summary)
	}
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

func TestBuildTree(t *testing.T) {
	docGUID := g(0, 0)
	pageGUID := g(0, 1)
	frameGUID := g(0, 2)

	frame := mkNode(2, "FRAME", "Card", pageGUID, "a")
	frame["size"] = map[string]any{"x": float32(100), "y": float32(50)}
	frame["transform"] = map[string]any{"m00": float32(0), "m01": float32(-1), "m10": float32(1), "m11": float32(0), "m02": float32(10), "m12": float32(20)}
	frame["clipContent"] = true

	text := mkNode(3, "TEXT", "Title", frameGUID, "a")
	text["textData"] = map[string]any{"characters": "Hi"}
	text["fontName"] = map[string]any{"family": "Inter", "style": "Regular"}
	text["fontSize"] = float32(14)
	text["textAlignHorizontal"] = "CENTER"
	text["textAlignVertical"] = "TOP"
	text["lineHeight"] = map[string]any{"units": "PIXELS", "value": float32(18)}
	text["letterSpacing"] = map[string]any{"units": "PIXELS", "value": float32(1.5)}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", docGUID, "a"),
		frame,
		text,
		mkNode(4, "INSTANCE", "Copy", frameGUID, "b"),
	}, nil)

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
	if n.TextAlign != "CENTER" || n.TextAlignV != "TOP" || n.LineHeight != 18 || n.LetterSpacing != 1.5 {
		t.Fatalf("node text props = %+v", n)
	}
	if frameNode, _ := doc.Node("0:2"); frameNode.Rotation != 90 || !frameNode.ClipContent {
		t.Fatalf("node frame props = %+v", frameNode)
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
	doc := Build([]any{removed}, nil)
	if len(doc.All()) != 0 {
		t.Fatalf("removed node kept: %+v", doc.All())
	}
}

func TestStyleDefs(t *testing.T) {
	fill := mkNode(1, "ROUNDED_RECTANGLE", "Brand/Primary", nil, "")
	fill["styleType"] = "FILL"
	fill["key"] = "k-fill"
	fill["fillPaints"] = []any{map[string]any{
		"type":    "SOLID",
		"color":   map[string]any{"r": float32(1), "g": float32(0.5), "b": float32(0), "a": float32(1)},
		"visible": true,
	}}

	text := mkNode(2, "TEXT", "Body/Regular", nil, "")
	text["styleType"] = "TEXT"
	text["key"] = "k-text"
	text["fontName"] = map[string]any{"family": "Inter", "style": "Regular"}
	text["fontSize"] = float32(16)
	text["fillPaints"] = []any{map[string]any{
		"type":    "SOLID",
		"color":   map[string]any{"r": float32(0), "g": float32(0), "b": float32(0), "a": float32(1)},
		"visible": true,
	}}

	dup := mkNode(3, "ROUNDED_RECTANGLE", "Old Anchor", nil, "")
	dup["styleType"] = "FILL"
	dup["key"] = "k-fill"

	applied := mkNode(4, "FRAME", "Card", nil, "")
	applied["styleIdForStrokeFill"] = map[string]any{"assetRef": map[string]any{"key": "k-fill"}}

	doc := Build([]any{fill, text, dup, applied}, nil)
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

func TestInlineTextProps(t *testing.T) {
	price := mkNode(2, "TEXT", "Price", g(0, 1), "a")
	price["textData"] = map[string]any{
		"characters": "30€ 20€",
		"styleOverrideTable": []any{
			map[string]any{"textDecoration": "STRIKETHROUGH"},
		},
	}
	price["fontName"] = map[string]any{"family": "DM Mono", "style": "Regular"}
	price["fontSize"] = float32(16)
	price["textAlignHorizontal"] = "LEFT"
	price["letterSpacing"] = map[string]any{"units": "PIXELS", "value": float32(0)}
	price["lineHeight"] = map[string]any{"units": "PIXELS", "value": float32(28)}
	price["fillPaints"] = []any{map[string]any{
		"type": "SOLID", "visible": true,
		"color": map[string]any{"r": float32(0.5), "g": float32(0.5), "b": float32(0.5), "a": float32(1)},
	}}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		price,
	}, nil)
	n, _ := doc.Node("0:2")
	if n.Decoration != "STRIKETHROUGH" {
		t.Fatalf("decoration = %q", n.Decoration)
	}
	var textDef *StyleDef
	for _, d := range doc.StyleDefs() {
		if d.Kind == "TEXT" {
			textDef = &d
		}
	}
	if textDef == nil {
		t.Fatalf("no text def: %+v", doc.StyleDefs())
	}
	if want := "DM Mono Regular · 16px · #808080 · strikethrough"; textDef.Summary != want {
		t.Fatalf("summary = %q, want %q", textDef.Summary, want)
	}
	var textUsage *Style
	for _, s := range doc.StyleUsages() {
		if s.Kind == "TEXT" {
			textUsage = &s
		}
	}
	if textUsage == nil || textUsage.Key != textDef.Key {
		t.Fatalf("usage = %+v", doc.StyleUsages())
	}
}

func TestInlineCornerRadius(t *testing.T) {
	uniform := mkNode(2, "FRAME", "Card", g(0, 1), "a")
	uniform["rectangleCornerTopLeftRadius"] = float32(8)
	uniform["rectangleCornerTopRightRadius"] = float32(8)
	uniform["rectangleCornerBottomRightRadius"] = float32(8)
	uniform["rectangleCornerBottomLeftRadius"] = float32(8)

	mixed := mkNode(3, "FRAME", "Card Mixed", g(0, 1), "b")
	mixed["rectangleCornerTopLeftRadius"] = float32(20)
	mixed["rectangleCornerBottomRightRadius"] = float32(20)

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		uniform, mixed,
	}, nil)

	defs := map[string]StyleDef{}
	for _, d := range doc.StyleDefs() {
		defs[d.NodeID] = d
	}
	if d := defs["0:2"]; !strings.Contains(d.Summary, "radius 8") || strings.Contains(d.Summary, "8 8 8 8") {
		t.Fatalf("uniform radius def = %q", d.Summary)
	}
	if d := defs["0:3"]; !strings.Contains(d.Summary, "radius 20 0 20 0") {
		t.Fatalf("mixed radius def = %q", d.Summary)
	}
}

func blobOf(bytes ...byte) []byte {
	return bytes
}

func f32bytes(vals ...float32) []byte {
	out := make([]byte, 0, 4*len(vals))
	for _, v := range vals {
		u := math.Float32bits(v)
		out = append(out, byte(u), byte(u>>8), byte(u>>16), byte(u>>24))
	}
	return out
}

func TestTextRuns(t *testing.T) {
	price := mkNode(2, "TEXT", "Price", g(0, 1), "a")
	price["textData"] = map[string]any{
		"characters":        "34.90€  28.30€ ",
		"characterStyleIDs": []any{int32(0), int32(0), int32(0), int32(0), int32(0), int32(0), int32(1), int32(1), int32(1), int32(1), int32(1), int32(1), int32(1), int32(1), int32(1)},
		"styleOverrideTable": []any{
			map[string]any{
				"styleID":        1,
				"fontName":       map[string]any{"family": "Onest", "style": "Regular"},
				"fontSize":       float32(16),
				"textDecoration": "STRIKETHROUGH",
				"fillPaints": []any{map[string]any{
					"type":  "SOLID",
					"color": map[string]any{"r": float32(0.46), "g": float32(0.43), "b": float32(0.35), "a": float32(1)},
				}},
			},
		},
	}
	price["fontName"] = map[string]any{"family": "DM Mono", "style": "Regular"}
	price["fontSize"] = float32(16)
	price["fillPaints"] = []any{map[string]any{
		"type":  "SOLID",
		"color": map[string]any{"r": float32(0.7), "g": float32(0.3), "b": float32(0.2), "a": float32(1)},
	}}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		price,
	}, nil)
	n, _ := doc.Node("0:2")
	if len(n.Runs) != 2 {
		t.Fatalf("runs = %+v", n.Runs)
	}
	if n.Runs[0].Text != "34.90€" || n.Runs[0].FontFamily != "DM Mono" || n.Runs[0].Color != "#B24D33" {
		t.Fatalf("run0 = %+v", n.Runs[0])
	}
	if n.Runs[1].Text != "  28.30€ " || n.Runs[1].Decoration != "STRIKETHROUGH" || n.Runs[1].Color != "#756E59" {
		t.Fatalf("run1 = %+v", n.Runs[1])
	}
}

func TestVectorPath(t *testing.T) {
	var square []byte
	square = append(square, 0x01)
	square = append(square, f32bytes(0, 0)...)
	square = append(square, 0x02)
	square = append(square, f32bytes(10, 0)...)
	square = append(square, 0x02)
	square = append(square, f32bytes(10, 10)...)
	square = append(square, 0x00)

	vec := mkNode(2, "VECTOR", "Icon", g(0, 1), "a")
	vec["fillGeometry"] = []any{map[string]any{
		"windingRule":  "NONZERO",
		"commandsBlob": int32(0),
		"styleID":      0,
	}}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		vec,
	}, [][]byte{blobOf(square...)})
	n, _ := doc.Node("0:2")
	if n.Path != "M0 0 L10 0 L10 10 Z" {
		t.Fatalf("path = %q", n.Path)
	}
}

func TestStackLayoutAndAttributes(t *testing.T) {
	row := mkNode(2, "FRAME", "Row", g(0, 1), "a")
	row["stackMode"] = "HORIZONTAL"
	row["stackSpacing"] = float32(12)
	row["stackPaddingTop"] = float32(8)
	row["stackPrimaryAlignItems"] = "CENTER"
	row["strokeDashes"] = []any{float32(2), float32(3)}
	row["strokeCap"] = "ROUND"
	row["effects"] = []any{map[string]any{"type": "DROP_SHADOW", "visible": true}}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		row,
	}, nil)
	var layoutDef, effectDef *StyleDef
	for _, d := range doc.StyleDefs() {
		switch d.Kind {
		case "LAYOUT":
			layoutDef = &d
		case "EFFECT":
			effectDef = &d
		}
	}
	if layoutDef == nil || layoutDef.Summary != "layout horizontal/center · gap 12 · padding 8 0 0" {
		t.Fatalf("layout def = %+v", layoutDef)
	}
	if effectDef == nil || effectDef.Summary != "Drop Shadow" {
		t.Fatalf("effect def = %+v", effectDef)
	}
	for _, s := range doc.StyleUsages() {
		if s.NodeID != "0:2" {
			continue
		}
		if s.Kind == "EFFECT" && effectDef == nil {
			t.Fatalf("effect usage without def: %+v", s)
		}
		if s.Kind == "LAYOUT" && layoutDef == nil {
			t.Fatalf("layout usage without def: %+v", s)
		}
	}
}

func TestPropOverrides(t *testing.T) {
	inst := mkNode(2, "INSTANCE", "Copy", g(0, 1), "a")
	inst["componentPropAssignments"] = map[string]any{"entries": []any{
		map[string]any{
			"defID": g(0, 77),
			"value": map[string]any{"textValue": map[string]any{"characters": "Hola mundo"}},
		},
		map[string]any{
			"defID": g(0, 78),
			"value": map[string]any{"booleanValue": true},
		},
	}}
	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		inst,
	}, nil)
	n, _ := doc.Node("0:2")
	if len(n.Props) != 2 || n.Props[0].ID != "0:77" || n.Props[0].Value != "Hola mundo" || n.Props[1].Value != true {
		t.Fatalf("props = %+v", n.Props)
	}
}

func TestPropOverrideFlatShape(t *testing.T) {
	// .fig carries componentPropAssignments as a flat array with text and slot
	// var-bound values.
	inst := mkNode(2, "INSTANCE", "Copy", g(0, 1), "a")
	inst["componentPropAssignments"] = []any{
		map[string]any{
			"defID": map[string]any{"sessionID": g(0, 1)["sessionID"], "localID": g(0, 77)["localID"]},
			"value": map[string]any{"textValue": map[string]any{"characters": "Descubre tu rutina ideal"}},
			"varValue": map[string]any{
				"dataType": "TEXT_DATA", "resolvedDataType": "TEXT_DATA",
				"value": map[string]any{"textDataValue": map[string]any{"characters": "Descubre tu rutina ideal"}},
			},
		},
		map[string]any{
			"defID": map[string]any{"sessionID": uint32(6031), "localID": uint32(26)},
			"value": map[string]any{},
			"varValue": map[string]any{
				"dataType": "SLOT_CONTENT_ID", "resolvedDataType": "SLOT_CONTENT_ID",
				"value": map[string]any{"slotContentIdValue": map[string]any{"guid": g(108, 1531)}},
			},
		},
	}
	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		inst,
	}, nil)
	n, _ := doc.Node("0:2")
	if len(n.Props) != 2 {
		t.Fatalf("props = %+v", n.Props)
	}
	if n.Props[0].Value != "Descubre tu rutina ideal" {
		t.Fatalf("prop0 = %+v", n.Props[0])
	}
	if n.Props[1].ID != "6031:26" {
		t.Fatalf("prop1 id = %+v", n.Props[1].ID)
	}
	if n.Props[1].Value != "slot:108:1531" {
		t.Fatalf("prop1 value = %+v", n.Props[1].Value)
	}
}

func TestVariables(t *testing.T) {
	set := mkNode(2, "VARIABLE_SET", "Peppercorn - Typography Scale", nil, "")
	set["variableSetModes"] = []any{map[string]any{
		"id":   g(0, 6278),
		"name": "Mode 1",
	}}
	v1 := mkNode(3, "VARIABLE", "font family/Syne", nil, "")
	v1["variableSetID"] = map[string]any{"guid": g(0, 2)}
	v1["variableResolvedType"] = "STRING"
	v1["variableDataValues"] = map[string]any{"entries": []any{map[string]any{
		"modeID": g(0, 6278),
		"variableData": map[string]any{
			"dataType": "STRING", "resolvedDataType": "STRING",
			"value": map[string]any{"textValue": "Syne"},
		},
	}}}
	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		set, v1,
	}, nil)
	if len(doc.Variables) != 1 {
		t.Fatalf("variables = %+v", doc.Variables)
	}
	v := doc.Variables[0]
	if v.Name != "font family/Syne" || v.Type != "STRING" || v.Set != "Peppercorn - Typography Scale" {
		t.Fatalf("variable = %+v", v)
	}
	if len(v.Modes) != 1 || v.Modes[0].Mode != "Mode 1" || v.Modes[0].Value != "Syne" {
		t.Fatalf("modes = %+v", v.Modes)
	}
}
