package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
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

// layoutFields are the node properties that describe an auto layout frame.
var layoutFields = []string{
	"paddingLeft", "paddingRight", "paddingTop", "paddingBottom",
	"itemSpacing", "cornerRadius",
	"rectangleCornerTopLeftRadius", "rectangleCornerTopRightRadius",
	"rectangleCornerBottomLeftRadius", "rectangleCornerBottomRightRadius",
}

// layoutInfo collects a node's meaningful layout geometry. It reports empty for
// nodes without auto layout, padding, rounding or stroke, so static frames do
// not produce noise. Newer .fig files name auto layout with stack* fields
// (stackMode, stackSpacing, stackPadding*) instead of layoutMode; both spell
// the same values and are normalized here.
func layoutInfo(n *Node) map[string]any {
	m := n.raw
	if n.Type == TypeText {
		return nil
	}
	info := map[string]any{}
	lm := first(m, "layoutMode")
	if lm == "NONE" {
		lm = ""
	}
	if lm == "" {
		lm = first(m, "stackMode")
	}
	if lm != "" {
		info["layoutMode"] = lm
		if v := first(m, "primaryAxisAlignItems", "stackPrimaryAlignItems"); v != "" {
			info["primaryAxisAlignItems"] = v
		}
		if v := first(m, "counterAxisAlignItems", "stackCounterAlignItems"); v != "" {
			info["counterAxisAlignItems"] = v
		}
	}
	if sp := asString(m["sizingMode"]); sp != "" {
		info["sizingMode"] = sp
	}
	for _, field := range layoutFields {
		if v := firstFloat(m, field, stackField(field)); v > 0 {
			info[field] = v
		}
	}
	if sw := asFloat(m["strokeWeight"]); sw > 0 {
		for _, p := range paintsOf(n, "STROKE") {
			if paintWellKnown(p) && paintVisible(p) {
				info["strokeWeight"] = sw
				break
			}
		}
	}
	for _, field := range []string{"strokeCap", "strokeJoin"} {
		if v := asString(first(m, field)); v != "" {
			info[field] = v
		}
	}
	if dash := asSlice(m["strokeDashes"]); len(dash) > 0 {
		info["strokeDashes"] = dash
	}
	for _, field := range []string{"horizontalConstraint", "verticalConstraint"} {
		if v := first(m, field); v != "" {
			info[field] = v
		}
	}
	if len(info) == 0 {
		return nil
	}
	return info
}

// stackField maps a layout field to its stack* sibling used by newer .fig
// producers (itemSpacing → stackSpacing, paddingLeft → stackPaddingLeft).
func stackField(field string) string {
	switch field {
	case "itemSpacing":
		return "stackSpacing"
	case "cornerRadius":
		return "stackCornerRadius"
	}
	if length := len(field); length > 7 && field[:7] == "padding" {
		return "stackPadding" + field[7:]
	}
	return ""
}

// first returns the first present string value among a set of fallback keys.
func first(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := asString(m[k]); s != "" {
			return s
		}
	}
	return ""
}

// firstFloat returns the first positive value among a key and its fallbacks.
func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v := asFloat(m[k]); v > 0 {
			return v
		}
	}
	return 0
}

// layoutSummary renders a compact description of a node's layout geometry.
func layoutSummary(info map[string]any) string {
	var parts []string
	if lm := asString(info["layoutMode"]); lm != "" {
		part := "layout " + strings.ToLower(lm)
		if a := asString(info["primaryAxisAlignItems"]); a != "" {
			part += "/" + strings.ToLower(a)
		}
		parts = append(parts, part)
	}
	if g := asFloat(info["itemSpacing"]); g > 0 {
		parts = append(parts, "gap "+shortFloat(g))
	}
	if p := layoutPadding(info); p != "" {
		parts = append(parts, "padding "+p)
	}
	if r := cornerRadius(info); r != "" {
		parts = append(parts, "radius "+r)
	}
	if sw := asFloat(info["strokeWeight"]); sw > 0 {
		parts = append(parts, "stroke "+shortFloat(sw))
	}
	if len(parts) == 0 {
		return humanTitle(strings.Join([]string{"layout"}, " "))
	}
	return strings.Join(parts, " · ")
}

func layoutPadding(info map[string]any) string {
	l, r, t, b := asFloat(info["paddingLeft"]), asFloat(info["paddingRight"]), asFloat(info["paddingTop"]), asFloat(info["paddingBottom"])
	switch {
	case l == r && t == b && l == t:
		if l > 0 {
			return shortFloat(l)
		}
	case t == b:
		if l == r {
			return shortFloat(t) + " " + shortFloat(l)
		}
		return shortFloat(t) + " " + shortFloat(l) + " " + shortFloat(r)
	case l == r:
		return shortFloat(t) + " " + shortFloat(l) + " " + shortFloat(b)
	default:
		return shortFloat(t) + " " + shortFloat(r) + " " + shortFloat(b) + " " + shortFloat(l)
	}
	return ""
}

// cornerRadius renders the node's corner rounding as a compact description.
// A uniform rounding collapses to a single value; custom (per-corner) rounding
// lists the four corners in Figma order: top-left, top-right, bottom-right,
// bottom-left.
func cornerRadius(info map[string]any) string {
	if cr := asFloat(info["cornerRadius"]); cr > 0 {
		return shortFloat(cr)
	}
	corners := []string{
		"rectangleCornerTopLeftRadius",
		"rectangleCornerTopRightRadius",
		"rectangleCornerBottomRightRadius",
		"rectangleCornerBottomLeftRadius",
	}
	anySet := false
	nums := make([]float64, 4)
	for i, field := range corners {
		nums[i] = asFloat(info[field])
		if nums[i] > 0 {
			anySet = true
		}
	}
	if !anySet {
		return ""
	}
	if nums[0] == nums[1] && nums[1] == nums[2] && nums[2] == nums[3] {
		return shortFloat(nums[0])
	}
	parts := make([]string, 4)
	for i, v := range nums {
		parts[i] = shortFloat(v)
	}
	return strings.Join(parts, " ")
}

// shortFloat renders a dimension with at most two decimals, dropping trailing
// zeros and decimal points.
func shortFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// paintsOf returns a node's direct paint list for a kind, preferring fillPaints
// over backgroundPaints for fills.
func paintsOf(n *Node, kind string) []any {
	switch kind {
	case "FILL":
		for _, field := range []string{"fillPaints", "backgroundPaints"} {
			if list := asSlice(n.raw[field]); len(list) > 0 {
				return list
			}
		}
	case "STROKE":
		return asSlice(n.raw["strokePaints"])
	}
	return nil
}

func paintVisible(p any) bool {
	vis, ok := asMap(p)["visible"].(bool)
	return !ok || vis
}

// paintWellKnown limits inline fills and strokes to paints that resolve to a
// meaningful visual value, skipping masks and other structural paint entries.
func paintWellKnown(p any) bool {
	t := asString(asMap(p)["type"])
	if strings.HasPrefix(t, "GRADIENT") {
		return true
	}
	switch t {
	case "SOLID", "IMAGE":
		return true
	}
	return false
}

func rawJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
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

// Fills returns the node's direct fill paints.
func (n *Node) Fills() []any { return paintsOf(n, "FILL") }

// StrokeHex returns the first visible solid stroke color, if any.
func (n *Node) StrokeHex() (string, bool) {
	for _, p := range paintsOf(n, "STROKE") {
		pm := asMap(p)
		if pm == nil || !paintVisible(p) || asString(pm["type"]) != "SOLID" {
			continue
		}
		if c := colorOf(pm); c != "" {
			return c, true
		}
	}
	return "", false
}

// FillHex returns the first visible solid fill color, if any.
func (n *Node) FillHex() (string, bool) {
	for _, p := range n.Fills() {
		pm := asMap(p)
		if pm == nil || !paintVisible(p) || asString(pm["type"]) != "SOLID" {
			continue
		}
		if c := colorOf(pm); c != "" {
			return c, true
		}
	}
	return "", false
}

// Layout returns the node's normalized auto-layout info (layoutMode, alignment,
// gaps, padding, radius, stroke), empty for nodes without meaningful geometry.
func (n *Node) Layout() map[string]any { return layoutInfo(n) }

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
	if d := asString(raw["textDecoration"]); d != "" {
		parts = append(parts, strings.ToLower(d))
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
