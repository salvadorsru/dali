package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Style struct {
	Kind    string
	Key     string
	NodeID  string
	Name    string `json:"name,omitempty"`
	Summary string `json:"summary,omitempty"`
	Value   any    `json:"value,omitempty"` // parsed style_defs.raw payload
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
		named := n.namedStyleKinds()
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
		for _, r := range n.inlineStyleRefs() {
			if named[r.kind] || n.isStyleAnchor() {
				continue
			}
			out = append(out, Style{Kind: r.kind, Key: r.key, NodeID: n.ID})
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
		return out[i].Name < out[j].Name
	})
	return out
}

// StyleDefs resolves the file's style definitions. A style definition lives on
// its anchor node, which carries a styleType field plus the key, name and
// inline values (paints, font, effects). Resolved inline values that apply to
// usage nodes without a named reference are appended as synthesized
// definitions, so a node's styles always resolve to a concrete value.
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
	for _, n := range d.order {
		named := n.namedStyleKinds()
		for _, r := range n.inlineStyleRefs() {
			if named[r.kind] || seen[r.key] || n.isStyleAnchor() {
				continue
			}
			seen[r.key] = true
			out = append(out, StyleDef{
				Key:     r.key,
				Kind:    r.kind,
				Name:    r.summary,
				NodeID:  n.ID,
				Summary: r.summary,
				Raw:     r.raw,
				Usage:   usage[r.key],
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// inlineStyleRef holds a synthesized style derived from a node's direct paints
// or text styling, and the value it resolves to.
type inlineStyleRef struct {
	kind    string
	key     string
	summary string
	raw     string
}

// namedStyleKinds reports which style kinds resolve through a named style
// reference on the node. Inline values for those kinds are not duplicated.
func (n *Node) namedStyleKinds() map[string]bool {
	named := make(map[string]bool)
	for _, sf := range styleFields {
		ref := asMap(asMap(n.raw[sf.field])["assetRef"])
		if ref == nil {
			continue
		}
		if key := asString(ref["key"]); key != "" {
			named[sf.kind] = true
		}
	}
	return named
}

// isStyleAnchor reports whether the node carries the library definition of a
// style. Anchor nodes are definitions, not usages, so they never emit inline
// styles for their own values.
func (n *Node) isStyleAnchor() bool {
	return n.raw != nil && asString(n.raw["styleType"]) != ""
}

// inlineStyleRefs captures styles the node carries directly: fills, strokes and
// text styling that are not backed by a named style reference.
func (n *Node) inlineStyleRefs() []inlineStyleRef {
	if n.raw == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []inlineStyleRef
	emit := func(r inlineStyleRef) {
		dedupe := r.kind + "\x00" + r.key
		if seen[dedupe] {
			return
		}
		seen[dedupe] = true
		out = append(out, r)
	}

	if paints := paintsOf(n, "FILL"); len(paints) > 0 {
		for _, p := range paints {
			if !paintVisible(p) {
				continue
			}
			if summary := paintsSummary([]any{p}); paintWellKnown(p) && summary != "" {
				emit(inlineStyleRef{
					kind:    "FILL",
					key:     inlineStyleKey("FILL", p),
					summary: summary,
					raw:     rawJSON(p),
				})
			}
		}
	}

	if paints := paintsOf(n, "STROKE"); len(paints) > 0 {
		for _, p := range paints {
			if !paintVisible(p) {
				continue
			}
			if summary := paintsSummary([]any{p}); paintWellKnown(p) && summary != "" {
				emit(inlineStyleRef{
					kind:    "STROKE",
					key:     inlineStyleKey("STROKE", p),
					summary: summary,
					raw:     rawJSON(p),
				})
			}
		}
	}

	if n.Type == TypeText {
		payload := map[string]any{}
		if fn := asMap(n.raw["fontName"]); fn != nil {
			payload["fontName"] = map[string]any{
				"family": asString(fn["family"]),
				"style":  asString(fn["style"]),
			}
		}
		if size := asFloat(n.raw["fontSize"]); size > 0 {
			payload["fontSize"] = size
		}
		for _, field := range []string{"textAlignHorizontal", "textAlignVertical", "textAutoResize"} {
			if v := asString(n.raw[field]); v != "" {
				payload[field] = v
			}
		}
		if lh := asMap(n.raw["lineHeight"]); lh != nil {
			payload["lineHeight"] = lh
		}
		if ls := asMap(n.raw["letterSpacing"]); ls != nil {
			payload["letterSpacing"] = ls
		}
		if d := n.Decoration; d != "" {
			payload["textDecoration"] = d
		}
		if paints := paintsOf(n, "FILL"); len(paints) > 0 {
			payload["fillPaints"] = paints
		}
		if len(payload) > 0 {
			emit(inlineStyleRef{
				kind:    "TEXT",
				key:     inlineStyleKey("TEXT", payload),
				summary: textSummary(payload),
				raw:     rawJSON(payload),
			})
		}
	}

	if info := layoutInfo(n); len(info) > 0 {
		emit(inlineStyleRef{
			kind:    "LAYOUT",
			key:     inlineStyleKey("LAYOUT", info),
			summary: layoutSummary(info),
			raw:     rawJSON(info),
		})
	}
	for _, f := range []struct {
		kind    string
		field   string
		summary func(any) string
	}{
		{"EFFECT", "effects", effectsSummary},
		{"GRID", "layoutGrids", gridsSummary},
	} {
		v := n.raw[f.field]
		if v == nil {
			continue
		}
		if s := f.summary(v); s != "" {
			emit(inlineStyleRef{
				kind:    f.kind,
				key:     inlineStyleKey(f.kind, v),
				summary: s,
				raw:     rawJSON(v),
			})
		}
	}
	return out
}

// inlineStyleKey derives a stable key for an inline style from its kind and
// serialized payload. Go's encoding/json sorts map keys, so the key is stable
// across indexing runs.
func inlineStyleKey(kind string, payload any) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(rawJSON(payload)))
	return hex.EncodeToString(h.Sum(nil))
}

func rawJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
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