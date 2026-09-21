package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
)

// TranslateLayout converts a node's Layout() map to ir.LayoutSpec.
func TranslateLayout(node *model.Node) ir.LayoutSpec {
	info := node.Layout()
	spec := ir.LayoutSpec{
		Mode:          "NONE",
		Direction:     "row",
		Gap:           0,
		Padding:       ir.PaddingSpec{},
		AlignItems:    "stretch",
		JustifyContent: "flex-start",
		AlignContent:  "stretch",
	}

	if info == nil {
		return spec
	}

	// Layout mode
	lm := asString(info["layoutMode"])
	if lm != "" && lm != "NONE" {
		spec.Mode = "FLEX"
		switch lm {
		case "HORIZONTAL":
			spec.Direction = "row"
		case "VERTICAL":
			spec.Direction = "column"
		case "GRID":
			spec.Mode = "GRID"
		}
	}

	// Stack mode (newer .fig)
	if lm == "" {
		lm = asString(info["stackMode"])
		if lm != "" && lm != "NONE" {
			spec.Mode = "FLEX"
			switch lm {
			case "HORIZONTAL":
				spec.Direction = "row"
			case "VERTICAL":
				spec.Direction = "column"
			case "GRID":
				spec.Mode = "GRID"
			}
		}
	}

	// Gap
	if g := asFloat(info["itemSpacing"]); g > 0 {
		spec.Gap = g
	} else if g := asFloat(info["stackSpacing"]); g > 0 {
		spec.Gap = g
	}

	// Padding. model.Layout() already normalizes stack* and legacy padding
	// fields to paddingTop / paddingRight / paddingBottom / paddingLeft.
	spec.Padding = ir.PaddingSpec{
		Top:    asFloat(info["paddingTop"]),
		Right:  asFloat(info["paddingRight"]),
		Bottom: asFloat(info["paddingBottom"]),
		Left:   asFloat(info["paddingLeft"]),
	}

	// Alignment
	if a := asString(info["primaryAxisAlignItems"]); a != "" {
		spec.JustifyContent = translateAlign(a)
	} else if a := asString(info["stackPrimaryAlignItems"]); a != "" {
		spec.JustifyContent = translateAlign(a)
	}
	if a := asString(info["counterAxisAlignItems"]); a != "" {
		spec.AlignItems = translateAlign(a)
	} else if a := asString(info["stackCounterAlignItems"]); a != "" {
		spec.AlignItems = translateAlign(a)
	}

	// Sizing. model.Layout() normalizes the raw .fig fields per axis: FIXED
	// needs the rendered box, HUG maps to fit-content and FILL stretches along
	// the parent's main axis. An axis the file never declared stays empty.
	spec.PrimaryAxisSizing = asString(info["primaryAxisSizingMode"])
	spec.CounterAxisSizing = asString(info["counterAxisSizingMode"])

	// Absolute rendered dimensions from the node's bounding box. FIXED sizing
	// needs these to emit w-[Npx] / h-[Npx]; HUG and FILL nodes also carry a
	// box, but the generated classes only rely on them when sizing is FIXED.
	spec.Width = node.Width
	spec.Height = node.Height

	// Corner radius
	if cr := asFloat(info["cornerRadius"]); cr > 0 {
		spec.CornerRadius = cr
	}
	corners := []string{
		"rectangleCornerTopLeftRadius",
		"rectangleCornerTopRightRadius",
		"rectangleCornerBottomRightRadius",
		"rectangleCornerBottomLeftRadius",
	}
	anySet := false
	for i, field := range corners {
		if v := asFloat(info[field]); v > 0 {
			spec.CornerRadii[i] = v
			anySet = true
		}
	}
	if !anySet && spec.CornerRadius == 0 {
		if cr := asFloat(info["stackCornerRadius"]); cr > 0 {
			spec.CornerRadius = cr
		}
	}

	// Stroke
	if sw := asFloat(info["strokeWeight"]); sw > 0 {
		spec.StrokeWeight = sw
	}
	if sc := asString(info["strokeColor"]); sc != "" {
		spec.StrokeColor = sc
	}
	if sa := asString(info["strokeAlign"]); sa != "" {
		spec.StrokeAlign = sa
	}

	// Constraints
	spec.Constraints = ir.ConstraintsSpec{
		MinWidth:  asFloat(info["minWidth"]),
		MaxWidth:  asFloat(info["maxWidth"]),
		MinHeight: asFloat(info["minHeight"]),
		MaxHeight: asFloat(info["maxHeight"]),
	}

	return spec
}

func translateAlign(figmaAlign string) string {
	switch strings.ToUpper(figmaAlign) {
	case "MIN", "START", "LEFT", "TOP":
		return "flex-start"
	case "CENTER", "MIDDLE":
		return "center"
	case "MAX", "END", "RIGHT", "BOTTOM":
		return "flex-end"
	case "SPACE_BETWEEN":
		return "space-between"
	case "SPACE_AROUND":
		return "space-around"
	case "SPACE_EVENLY":
		return "space-evenly"
	case "STRETCH":
		return "stretch"
	case "BASELINE":
		return "baseline"
	}
	return "flex-start"
}

// LayoutToTailwind converts ir.LayoutSpec to Tailwind CSS classes.
func LayoutToTailwind(spec ir.LayoutSpec) string {
	var classes []string

	if spec.Mode == "FLEX" {
		classes = append(classes, "flex")
		switch spec.Direction {
		case "row":
			classes = append(classes, "flex-row")
		case "column":
			classes = append(classes, "flex-col")
		case "row-reverse":
			classes = append(classes, "flex-row-reverse")
		case "column-reverse":
			classes = append(classes, "flex-col-reverse")
		}
	} else if spec.Mode == "GRID" {
		classes = append(classes, "grid")
	}

	if spec.Gap > 0 {
		classes = append(classes, fmt.Sprintf("gap-[%gpx]", spec.Gap))
	}

	// Padding
	if spec.Padding.Top > 0 || spec.Padding.Right > 0 || spec.Padding.Bottom > 0 || spec.Padding.Left > 0 {
		pad := buildPadding(spec.Padding)
		if pad != "" {
			classes = append(classes, pad)
		}
	}

	// Justify content
	if spec.JustifyContent != "flex-start" {
		classes = append(classes, fmt.Sprintf("justify-%s", spec.JustifyContent))
	}

	// Align items
	if spec.AlignItems != "stretch" {
		classes = append(classes, fmt.Sprintf("items-%s", spec.AlignItems))
	}

	// Corner radius
	if spec.CornerRadius > 0 {
		classes = append(classes, fmt.Sprintf("rounded-[%gpx]", spec.CornerRadius))
	} else if spec.CornerRadii[0] > 0 || spec.CornerRadii[1] > 0 || spec.CornerRadii[2] > 0 || spec.CornerRadii[3] > 0 {
		classes = append(classes, fmt.Sprintf("rounded-[%gpx_%gpx_%gpx_%gpx]",
			spec.CornerRadii[0], spec.CornerRadii[1], spec.CornerRadii[2], spec.CornerRadii[3]))
	}

	// Stroke
	if spec.StrokeWeight > 0 {
		classes = append(classes, fmt.Sprintf("border-[%gpx]", spec.StrokeWeight))
		if spec.StrokeColor != "" {
			classes = append(classes, fmt.Sprintf("border-[%s]", spec.StrokeColor))
		}
	}

	// Constraints
	if spec.Constraints.MinWidth > 0 {
		classes = append(classes, fmt.Sprintf("min-w-[%gpx]", spec.Constraints.MinWidth))
	}
	if spec.Constraints.MaxWidth > 0 {
		classes = append(classes, fmt.Sprintf("max-w-[%gpx]", spec.Constraints.MaxWidth))
	}
	if spec.Constraints.MinHeight > 0 {
		classes = append(classes, fmt.Sprintf("min-h-[%gpx]", spec.Constraints.MinHeight))
	}
	if spec.Constraints.MaxHeight > 0 {
		classes = append(classes, fmt.Sprintf("max-h-[%gpx]", spec.Constraints.MaxHeight))
	}

	// Sizing
	//
	// Primary axis: HUG → h-fit / w-fit, FILL → flex-1, FIXED → h-[Npx] / w-[Npx].
	switch spec.PrimaryAxisSizing {
	case "HUG":
		if spec.Direction == "column" {
			classes = append(classes, "h-fit")
		} else {
			classes = append(classes, "w-fit")
		}
	case "FILL":
		classes = append(classes, "flex-1")
	case "FIXED":
		if spec.Direction == "column" {
			classes = append(classes, fmt.Sprintf("h-[%gpx]", spec.Height))
		} else {
			classes = append(classes, fmt.Sprintf("w-[%gpx]", spec.Width))
		}
	}

	// Counter axis: HUG → w-fit / h-fit (perpendicular), FILL → self-stretch,
	// FIXED → w-[Npx] / h-[Npx] (perpendicular).
	switch spec.CounterAxisSizing {
	case "HUG":
		if spec.Direction == "column" {
			classes = append(classes, "w-fit")
		} else {
			classes = append(classes, "h-fit")
		}
	case "FILL":
		classes = append(classes, "self-stretch")
	case "FIXED":
		if spec.Direction == "column" {
			classes = append(classes, fmt.Sprintf("w-[%gpx]", spec.Width))
		} else {
			classes = append(classes, fmt.Sprintf("h-[%gpx]", spec.Height))
		}
	}

	return strings.Join(classes, " ")
}

// LayoutToCSSVars converts ir.LayoutSpec to CSS custom properties.
func LayoutToCSSVars(spec ir.LayoutSpec, prefix string) string {
	var props []string

	if spec.Gap > 0 {
		props = append(props, fmt.Sprintf("--%s-gap: %gpx;", prefix, spec.Gap))
	}
	if spec.Padding.Top > 0 || spec.Padding.Right > 0 || spec.Padding.Bottom > 0 || spec.Padding.Left > 0 {
		props = append(props, fmt.Sprintf("--%s-padding: %gpx %gpx %gpx %gpx;",
			prefix, spec.Padding.Top, spec.Padding.Right, spec.Padding.Bottom, spec.Padding.Left))
	}
	if spec.CornerRadius > 0 {
		props = append(props, fmt.Sprintf("--%s-radius: %gpx;", prefix, spec.CornerRadius))
	}
	if spec.StrokeWeight > 0 {
		props = append(props, fmt.Sprintf("--%s-stroke-width: %gpx;", prefix, spec.StrokeWeight))
	}
	if spec.StrokeColor != "" {
		props = append(props, fmt.Sprintf("--%s-stroke-color: %s;", prefix, spec.StrokeColor))
	}

	return strings.Join(props, " ")
}

func buildPadding(p ir.PaddingSpec) string {
	// All equal
	if p.Top == p.Right && p.Right == p.Bottom && p.Bottom == p.Left {
		if p.Top > 0 {
			return fmt.Sprintf("p-[%gpx]", p.Top)
		}
	}
	// Vertical/horizontal equal
	if p.Top == p.Bottom && p.Left == p.Right {
		if p.Top > 0 && p.Left > 0 {
			return fmt.Sprintf("py-[%gpx] px-[%gpx]", p.Top, p.Left)
		}
		if p.Top > 0 {
			return fmt.Sprintf("py-[%gpx]", p.Top)
		}
		if p.Left > 0 {
			return fmt.Sprintf("px-[%gpx]", p.Left)
		}
	}
	// Individual
	var parts []string
	if p.Top > 0 {
		parts = append(parts, fmt.Sprintf("pt-[%gpx]", p.Top))
	}
	if p.Right > 0 {
		parts = append(parts, fmt.Sprintf("pr-[%gpx]", p.Right))
	}
	if p.Bottom > 0 {
		parts = append(parts, fmt.Sprintf("pb-[%gpx]", p.Bottom))
	}
	if p.Left > 0 {
		parts = append(parts, fmt.Sprintf("pl-[%gpx]", p.Left))
	}
	return strings.Join(parts, " ")
}