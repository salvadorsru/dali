package model

import (
	"fmt"
	"math"
	"strings"
)

// Paints is a node's ordered, normalized paint stack. Fills and strokes keep
// the authored stacking order (first element is the bottom layer, as stored in
// the .fig), so callers can reason about overlaps without re-reading raw paint
// payloads. Colors are #RRGGBB or #RRGGBBAA.
//
// StrokeWeight travels here because it is a node field that only means
// anything next to the strokes, and this struct is already the node's
// persisted paint payload — a separate column would buy nothing.
type Paints struct {
	Fills        []Paint  `json:"fills,omitempty"`
	Strokes      []Paint  `json:"strokes,omitempty"`
	Effects      []Effect `json:"effects,omitempty"`
	StrokeWeight float64  `json:"stroke_weight,omitempty"`
}

// Paint is a single fill or stroke layer with its ergonomic visual fields.
type Paint struct {
	Type      string      `json:"type"`
	Visible   bool        `json:"visible"`
	Color     string      `json:"color,omitempty"`
	Opacity   float64     `json:"opacity"`
	BlendMode string      `json:"blend_mode,omitempty"`
	Stops     []PaintStop `json:"stops,omitempty"`
	Start     *Point      `json:"start,omitempty"`
	End       *Point      `json:"end,omitempty"`
	Angle     float64     `json:"angle,omitempty"`
	Image     string      `json:"image,omitempty"`
	ScaleMode string      `json:"scale_mode,omitempty"`
}

// PaintStop is a gradient stop. Position is normalized 0..1.
type PaintStop struct {
	Position float64 `json:"position"`
	Color    string  `json:"color,omitempty"`
}

// Point is a 2D coordinate used by gradient start/end points.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Effect is a layer effect (shadow, blur) with its geometry.
type Effect struct {
	Type      string  `json:"type"`
	Visible   bool    `json:"visible"`
	Color     string  `json:"color,omitempty"`
	OffsetX   float64 `json:"offset_x,omitempty"`
	OffsetY   float64 `json:"offset_y,omitempty"`
	Radius    float64 `json:"radius,omitempty"`
	Spread    float64 `json:"spread,omitempty"`
	BlendMode string  `json:"blend_mode,omitempty"`
}

// PaintStack returns the node's ordered fills, strokes and effects, plus its
// stroke weight. It reads the raw payload, so it only carries data on nodes
// straight out of Build; nodes read back from the index expose the same values
// through PaintData.
func (n *Node) PaintStack() Paints {
	var p Paints
	for _, f := range n.Fills() {
		p.Fills = append(p.Fills, normalizePaint(f))
	}
	for _, s := range n.Strokes() {
		p.Strokes = append(p.Strokes, normalizePaint(s))
	}
	for _, e := range n.Effects() {
		p.Effects = append(p.Effects, normalizeEffect(e))
	}
	p.StrokeWeight = n.strokeWeight()
	return p
}

// strokeWeight returns the node's stroke weight, but only when it actually has
// a stroke to apply it to. A weight with no visible stroke paints is not a
// visual value, and reporting it would surface as a phantom override on
// instances.
func (n *Node) strokeWeight() float64 {
	sw := asFloat(n.raw["strokeWeight"])
	if sw <= 0 {
		return 0
	}
	for _, p := range paintsOf(n, "STROKE") {
		if paintWellKnown(p) && paintVisible(p) {
			return sw
		}
	}
	return 0
}

func normalizePaint(v any) Paint {
	pm := asMap(v)
	p := Paint{
		Type:      asString(pm["type"]),
		Visible:   paintVisible(v),
		Opacity:   1,
		BlendMode: asString(pm["blendMode"]),
	}
	if _, ok := pm["opacity"]; ok {
		p.Opacity = asFloat(pm["opacity"])
	}
	switch {
	case p.Type == "SOLID":
		p.Color = ColorOf(pm)
	case strings.HasPrefix(p.Type, "GRADIENT"):
		for _, st := range asSlice(pm["stops"]) {
			sm := asMap(st)
			p.Stops = append(p.Stops, PaintStop{
				Position: asFloat(sm["position"]),
				Color:    ColorOf(sm),
			})
		}
		if sp := asMap(pm["gradientStartPoint"]); sp != nil {
			p.Start = &Point{X: asFloat(sp["x"]), Y: asFloat(sp["y"])}
		}
		if ep := asMap(pm["gradientEndPoint"]); ep != nil {
			p.End = &Point{X: asFloat(ep["x"]), Y: asFloat(ep["y"])}
		}
		p.Angle = asFloat(pm["gradientAngle"])
	case p.Type == "IMAGE":
		if img := asMap(pm["image"]); img != nil {
			p.Image = asString(img["name"])
		}
		p.ScaleMode = asString(pm["scaleMode"])
	}
	return p
}

func normalizeEffect(v any) Effect {
	em := asMap(v)
	e := Effect{
		Type:      asString(em["type"]),
		Visible:   paintVisible(v),
		Color:     ColorOf(em),
		Radius:    asFloat(em["radius"]),
		Spread:    asFloat(em["spread"]),
		BlendMode: asString(em["blendMode"]),
	}
	if off := asMap(em["offset"]); off != nil {
		e.OffsetX = asFloat(off["x"])
		e.OffsetY = asFloat(off["y"])
	}
	return e
}

// BoundVarsJSON exposes the node's raw variable-alias bindings so callers can
// fingerprint them without reaching into the unexported raw payload.
func (n *Node) BoundVarsJSON() string {
	if n.raw == nil {
		return ""
	}
	return rawJSON(n.raw["variableConsumptionMap"]) + rawJSON(n.raw["boundVariables"])
}

// BoundVariables maps a node's bound field names to the variable names bound to
// them. Newer .fig files store bindings in variableConsumptionMap (a list of
// entries with a named field and a variable alias); older files use a
// boundVariables map. Fields whose alias does not resolve to a known variable
// fall back to the raw variable id.
func (d *Document) BoundVariables(n *Node) map[string]string {
	if n.raw == nil {
		return nil
	}
	names := make(map[string]string, len(d.Variables))
	for _, v := range d.Variables {
		names[v.ID] = v.Name
	}
	out := make(map[string]string)
	add := func(field, id string) {
		if field == "" || id == "" {
			return
		}
		if name := names[id]; name != "" {
			out[field] = name
		} else {
			out[field] = id
		}
	}

	for _, e := range asSlice(asMap(n.raw["variableConsumptionMap"])["entries"]) {
		em := asMap(e)
		value := asMap(asMap(em["variableData"])["value"])
		add(asString(em["variableField"]), guidValue(asMap(value["alias"])["guid"]))
	}
	for field, alias := range asMap(n.raw["boundVariables"]) {
		add(field, guidValue(asMap(alias)["id"]))
	}

	if len(out) == 0 {
		return nil
	}
	return out
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

// Fills returns the node's direct fill paints.
func (n *Node) Fills() []any { return paintsOf(n, "FILL") }

// Strokes returns the node's direct stroke paints.
func (n *Node) Strokes() []any { return paintsOf(n, "STROKE") }

// Effects returns the node's effect paints.
func (n *Node) Effects() []any { return asSlice(n.raw["effects"]) }

// StrokeHex returns the first visible solid stroke color, if any.
func (n *Node) StrokeHex() (string, bool) {
	for _, p := range paintsOf(n, "STROKE") {
		pm := asMap(p)
		if pm == nil || !paintVisible(p) || asString(pm["type"]) != "SOLID" {
			continue
		}
		if c := ColorOf(pm); c != "" {
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
		if c := ColorOf(pm); c != "" {
			return c, true
		}
	}
	return "", false
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
	case ptype == "SOLID" && ColorOf(pm) != "":
		return ColorOf(pm)
	case strings.HasPrefix(ptype, "GRADIENT"):
		var colors []string
		for _, st := range asSlice(pm["stops"]) {
			if c := ColorOf(asMap(st)); c != "" {
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

// ColorOf returns the hex color of a paint or gradient stop, preferring the
// authored sRGB color when present.
func ColorOf(m map[string]any) string {
	if ac := asMap(m["authoredColor"]); ac != nil {
		r := Clamp255(asFloat(ac["c0"]))
		g := Clamp255(asFloat(ac["c1"]))
		b := Clamp255(asFloat(ac["c2"]))
		a := asFloat(ac["c3"])
		return HexColor(r, g, b, a)
	}
	if c := asMap(m["color"]); c != nil {
		r := Clamp255(asFloat(c["r"]))
		g := Clamp255(asFloat(c["g"]))
		b := Clamp255(asFloat(c["b"]))
		a := asFloat(c["a"])
		return HexColor(r, g, b, a)
	}
	return ""
}

func HexColor(r, g, b int, a float64) string {
	if a < 0.999 {
		return fmt.Sprintf("#%02X%02X%02X%02X", r, g, b, Clamp255(a))
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func Clamp255(v float64) int {
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
