package model

import "strings"

// Paints is a node's ordered, normalized paint stack. Fills and strokes keep
// the authored stacking order (first element is the bottom layer, as stored in
// the .fig), so callers can reason about overlaps without re-reading raw paint
// payloads. Colors are #RRGGBB or #RRGGBBAA.
type Paints struct {
	Fills   []Paint  `json:"fills,omitempty"`
	Strokes []Paint  `json:"strokes,omitempty"`
	Effects []Effect `json:"effects,omitempty"`
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

// PaintStack returns the node's ordered fills, strokes and effects.
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
	return p
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
		p.Color = colorOf(pm)
	case strings.HasPrefix(p.Type, "GRADIENT"):
		for _, st := range asSlice(pm["stops"]) {
			sm := asMap(st)
			p.Stops = append(p.Stops, PaintStop{
				Position: asFloat(sm["position"]),
				Color:    colorOf(sm),
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
		Color:     colorOf(em),
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
