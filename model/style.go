package model

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Style struct {
	Kind    string
	Key     string
	NodeID  string
	Name    string `json:"name,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type StyleDef struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	NodeID  string `json:"node_id"`
	Summary string `json:"summary"`
	Raw     string `json:"raw,omitempty"`
	Usage   int    `json:"usage"`
}

var styleFields = []struct {
	field string
	kind  string
}{
	{"styleIdForFill", "FILL"},
	{"styleIdForStrokeFill", "STROKE"},
	{"styleIdForText", "TEXT"},
	{"styleIdForEffect", "EFFECT"},
	{"styleIdForGrid", "GRID"},
}

func (d *Document) StyleUsages() []Style {
	var out []Style
	for _, n := range d.order {
		for _, sf := range styleFields {
			ref := asMap(asMap(n.raw[sf.field])["assetRef"])
			if ref == nil {
				continue
			}
			key := asString(ref["key"])
			if key == "" {
				continue
			}
			out = append(out, Style{Kind: sf.kind, Key: key, NodeID: n.ID})
		}
	}
	return out
}

func (d *Document) Styles() []Style {
	seen := make(map[string]bool)
	var out []Style
	for _, s := range d.StyleUsages() {
		dedupe := s.Kind + "\x00" + s.Key
		if seen[dedupe] {
			continue
		}
		seen[dedupe] = true
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// StyleDefs resolves the file's style definitions. A style definition lives on
// its anchor node, which carries a styleType field plus the key, name and
// inline values (paints, font, effects).
func (d *Document) StyleDefs() []StyleDef {
	usage := make(map[string]int)
	for _, s := range d.StyleUsages() {
		usage[s.Key]++
	}
	seen := make(map[string]bool)
	var out []StyleDef
	for _, n := range d.order {
		raw := n.raw
		if raw == nil {
			continue
		}
		kind := asString(raw["styleType"])
		if kind == "" {
			continue
		}
		key := asString(raw["key"])
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		rawJSON, _ := json.Marshal(raw)
		out = append(out, StyleDef{
			Key:     key,
			Kind:    kind,
			Name:    n.Name,
			NodeID:  n.ID,
			Summary: styleSummary(raw, kind),
			Raw:     string(rawJSON),
			Usage:   usage[key],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func styleSummary(raw map[string]any, kind string) string {
	var s string
	switch kind {
	case "FILL", "STROKE":
		s = paintsSummary(raw["fillPaints"])
	case "TEXT":
		s = textSummary(raw)
	case "EFFECT":
		s = effectsSummary(raw["effects"])
	case "GRID":
		s = gridsSummary(raw["layoutGrids"])
	}
	if s == "" {
		return humanTitle(kind)
	}
	return s
}

func textSummary(raw map[string]any) string {
	var parts []string
	if fn := asMap(raw["fontName"]); fn != nil {
		family := asString(fn["family"])
		style := asString(fn["style"])
		switch {
		case family == "":
		case style == "":
			parts = append(parts, family)
		default:
			parts = append(parts, family+" "+style)
		}
	}
	if size := asFloat(raw["fontSize"]); size > 0 {
		parts = append(parts, fmt.Sprintf("%gpx", size))
	}
	if s := paintsSummary(raw["fillPaints"]); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

func paintsSummary(v any) string {
	list := asSlice(v)
	var parts []string
	for _, p := range list {
		pm := asMap(p)
		if pm == nil {
			continue
		}
		if visible, ok := pm["visible"].(bool); ok && !visible {
			continue
		}
		parts = append(parts, paintSummary(pm))
	}
	if len(parts) == 0 {
		for _, p := range list {
			if pm := asMap(p); pm != nil {
				parts = append(parts, paintSummary(pm))
			}
		}
	}
	return strings.Join(parts, "; ")
}

func paintSummary(pm map[string]any) string {
	ptype := asString(pm["type"])
	switch {
	case ptype == "SOLID" && colorOf(pm) != "":
		return colorOf(pm)
	case strings.HasPrefix(ptype, "GRADIENT"):
		var colors []string
		for _, st := range asSlice(pm["stops"]) {
			if c := colorOf(asMap(st)); c != "" {
				colors = append(colors, c)
			}
		}
		if len(colors) == 0 {
			return "gradient"
		}
		return fmt.Sprintf("gradient(%s)", strings.Join(colors, ", "))
	case ptype == "IMAGE":
		if img := asMap(pm["image"]); img != nil {
			if name := asString(img["name"]); name != "" {
				return "image " + name
			}
		}
		return "image"
	default:
		return humanTitle(ptype)
	}
}

// colorOf returns the hex color of a paint or gradient stop, preferring the
// authored sRGB color when present.
func colorOf(m map[string]any) string {
	if ac := asMap(m["authoredColor"]); ac != nil {
		r := clamp255(asFloat(ac["c0"]))
		g := clamp255(asFloat(ac["c1"]))
		b := clamp255(asFloat(ac["c2"]))
		a := asFloat(ac["c3"])
		return hexColor(r, g, b, a)
	}
	if c := asMap(m["color"]); c != nil {
		r := clamp255(asFloat(c["r"]))
		g := clamp255(asFloat(c["g"]))
		b := clamp255(asFloat(c["b"]))
		a := asFloat(c["a"])
		return hexColor(r, g, b, a)
	}
	return ""
}

func hexColor(r, g, b int, a float64) string {
	if a < 0.999 {
		return fmt.Sprintf("#%02X%02X%02X%02X", r, g, b, clamp255(a))
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func clamp255(v float64) int {
	v = math.Round(v * 255)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v)
}

func effectsSummary(v any) string {
	var parts []string
	for _, e := range asSlice(v) {
		if em := asMap(e); em != nil {
			if t := asString(em["type"]); t != "" {
				parts = append(parts, humanTitle(t))
			}
		}
	}
	return strings.Join(parts, ", ")
}

func gridsSummary(v any) string {
	n := len(asSlice(v))
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d grid%s", n, plural(n))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func humanTitle(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Split(strings.ToLower(s), "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
