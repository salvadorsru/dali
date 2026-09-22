package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
)

// TranslateStyles converts a node's direct styles to ir.StyleSpec.
func TranslateStyles(node *model.Node) ir.StyleSpec {
	spec := ir.StyleSpec{
		Opacity:     node.Opacity,
		Rotation:    node.Rotation,
		ClipContent: node.ClipContent,
		Visible:     node.Visible,
	}

	// Fills
	for _, p := range node.Fills() {
		if fill := translateFill(p); fill != nil {
			spec.Fills = append(spec.Fills, *fill)
		}
	}

	// Strokes
	for _, p := range node.Strokes() {
		if stroke := translateStroke(p); stroke != nil {
			spec.Strokes = append(spec.Strokes, *stroke)
		}
	}

	// Effects
	if effects := translateEffects(node); len(effects) > 0 {
		spec.Effects = effects
	}

	// Background (first solid fill as background)
	for _, p := range node.Fills() {
		pm := asMap(p)
		if pm == nil || !paintVisible(p) {
			continue
		}
		if asString(pm["type"]) == "SOLID" {
			if c := colorOf(pm); c != "" {
				spec.Background = ir.FillSpec{Type: "SOLID", Color: c, Visible: true}
				break
			}
		}
	}

	return spec
}

// translateFill converts a paint to ir.FillSpec.
func translateFill(p any) *ir.FillSpec {
	pm := asMap(p)
	if pm == nil || !paintVisible(p) {
		return nil
	}
	ptype := asString(pm["type"])
	fill := &ir.FillSpec{Visible: true}

	switch ptype {
	case "SOLID":
		fill.Type = "SOLID"
		if c := colorOf(pm); c != "" {
			fill.Color = c
		}
		if a := asFloat(pm["opacity"]); a > 0 {
			fill.Opacity = a
		} else {
			fill.Opacity = 1
		}
	case "GRADIENT_LINEAR":
		fill.Type = "GRADIENT_LINEAR"
		if g := translateGradient(pm); g != nil {
			fill.Gradient = g
		}
	case "GRADIENT_RADIAL":
		fill.Type = "GRADIENT_RADIAL"
		if g := translateGradient(pm); g != nil {
			fill.Gradient = g
		}
	case "GRADIENT_ANGULAR":
		fill.Type = "GRADIENT_ANGULAR"
		if g := translateGradient(pm); g != nil {
			fill.Gradient = g
		}
	case "IMAGE":
		fill.Type = "IMAGE"
		if img := asMap(pm["image"]); img != nil {
			if name := asString(img["name"]); name != "" {
				fill.Image = &ir.AssetSpec{Name: name}
			}
		}
	default:
		return nil
	}

	return fill
}

// translateStroke converts a paint to ir.StrokeSpec.
func translateStroke(p any) *ir.StrokeSpec {
	pm := asMap(p)
	if pm == nil || !paintVisible(p) {
		return nil
	}
	ptype := asString(pm["type"])
	stroke := &ir.StrokeSpec{}

	switch ptype {
	case "SOLID":
		stroke.Type = "SOLID"
		if c := colorOf(pm); c != "" {
			stroke.Color = c
		}
	case "GRADIENT_LINEAR", "GRADIENT_RADIAL", "GRADIENT_ANGULAR":
		stroke.Type = "GRADIENT"
		// Gradient strokes are complex; just note the type
	default:
		return nil
	}

	if w := asFloat(pm["weight"]); w > 0 {
		stroke.Width = w
	} else if w := asFloat(pm["strokeWeight"]); w > 0 {
		stroke.Width = w
	}
	if a := asString(pm["strokeAlign"]); a != "" {
		stroke.Align = a
	}

	return stroke
}

// translateGradient extracts gradient parameters.
func translateGradient(pm map[string]any) *ir.GradientSpec {
	stops := asSlice(pm["stops"])
	if len(stops) == 0 {
		return nil
	}
	g := &ir.GradientSpec{}
	for _, st := range stops {
		sm := asMap(st)
		if sm == nil {
			continue
		}
		stop := ir.GradientStop{
			Position: asFloat(sm["position"]),
			Opacity:  asFloat(sm["opacity"]),
		}
		if c := colorOf(sm); c != "" {
			stop.Color = c
		}
		g.Stops = append(g.Stops, stop)
	}
	if sp := asMap(pm["gradientStartPoint"]); sp != nil {
		g.StartPoint = ir.Point{X: asFloat(sp["x"]), Y: asFloat(sp["y"])}
	}
	if ep := asMap(pm["gradientEndPoint"]); ep != nil {
		g.EndPoint = ir.Point{X: asFloat(ep["x"]), Y: asFloat(ep["y"])}
	}
	if angle := asFloat(pm["gradientAngle"]); angle != 0 {
		g.Angle = angle
	}
	if inner := asFloat(pm["innerRadius"]); inner > 0 {
		g.InnerRadius = inner
	}
	return g
}

// translateEffects extracts effects (shadows, blurs).
func translateEffects(node *model.Node) []ir.EffectSpec {
	var effects []ir.EffectSpec
	for _, e := range node.Effects() {
		em := asMap(e)
		if em == nil {
			continue
		}
		t := asString(em["type"])
		if t == "" {
			continue
		}
		effect := ir.EffectSpec{
			Type:    strings.ToUpper(t),
			Visible: true,
		}
		switch t {
		case "DROP_SHADOW", "INNER_SHADOW":
			if c := colorOf(em); c != "" {
				effect.Color = c
			}
			effect.Offset = ir.Point{X: asFloat(em["offsetX"]), Y: asFloat(em["offsetY"])}
			effect.Radius = asFloat(em["radius"])
			effect.Spread = asFloat(em["spread"])
		case "LAYER_BLUR", "BACKGROUND_BLUR":
			effect.Radius = asFloat(em["radius"])
		}
		effects = append(effects, effect)
	}
	return effects
}

func asMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asFloat(v any) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return 0
}

func paintVisible(p any) bool {
	vis, ok := asMap(p)["visible"].(bool)
	return !ok || vis
}

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

func clamp255(v float64) int {
	v = v * 255
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v + 0.5)
}

func hexColor(r, g, b int, a float64) string {
	if a < 0.999 {
		return fmt.Sprintf("#%02X%02X%02X%02X", r, g, b, clamp255(a))
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}