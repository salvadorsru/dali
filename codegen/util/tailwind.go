package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
)

// StyleToTailwind converts ir.StyleSpec to Tailwind classes.
func StyleToTailwind(spec ir.StyleSpec) string {
	var classes []string

	// Opacity
	if spec.Opacity > 0 && spec.Opacity < 1 {
		classes = append(classes, fmt.Sprintf("opacity-[%g]", spec.Opacity))
	}

	// Rotation
	if spec.Rotation != 0 {
		classes = append(classes, fmt.Sprintf("rotate-[%gdeg]", spec.Rotation))
	}

	// Fills - use first solid fill as bg
	for _, fill := range spec.Fills {
		if fill.Type == "SOLID" && fill.Visible && fill.Color != "" {
			classes = append(classes, fmt.Sprintf("bg-[%s]", fill.Color))
			break
		}
	}

	// Strokes - use first stroke as border
	for _, stroke := range spec.Strokes {
		if stroke.Width > 0 {
			classes = append(classes, fmt.Sprintf("border-[%gpx]", stroke.Width))
			if stroke.Color != "" {
				classes = append(classes, fmt.Sprintf("border-[%s]", stroke.Color))
			}
			break
		}
	}

	// Effects - shadows
	for _, effect := range spec.Effects {
		if effect.Type == "DROP_SHADOW" && effect.Radius > 0 {
			shadow := fmt.Sprintf("shadow-[%gpx_%gpx_%gpx_%gpx_%s]",
				effect.Offset.X, effect.Offset.Y, effect.Radius, effect.Spread, effect.Color)
			classes = append(classes, shadow)
		}
		if effect.Type == "INNER_SHADOW" && effect.Radius > 0 {
			shadow := fmt.Sprintf("shadow-[inset_%gpx_%gpx_%gpx_%gpx_%s]",
				effect.Offset.X, effect.Offset.Y, effect.Radius, effect.Spread, effect.Color)
			classes = append(classes, shadow)
		}
		if effect.Type == "LAYER_BLUR" && effect.Radius > 0 {
			classes = append(classes, fmt.Sprintf("backdrop-blur-[%gpx]", effect.Radius))
		}
		if effect.Type == "BACKGROUND_BLUR" && effect.Radius > 0 {
			classes = append(classes, fmt.Sprintf("backdrop-blur-[%gpx]", effect.Radius))
		}
	}

	// Clip content
	if spec.ClipContent {
		classes = append(classes, "overflow-hidden")
	}

	// Visibility
	if !spec.Visible {
		classes = append(classes, "hidden")
	}

	return strings.Join(classes, " ")
}

// StyleToCSSVars converts ir.StyleSpec to CSS custom properties.
func StyleToCSSVars(spec ir.StyleSpec, prefix string) string {
	var props []string

	if spec.Opacity > 0 && spec.Opacity < 1 {
		props = append(props, fmt.Sprintf("--%s-opacity: %g;", prefix, spec.Opacity))
	}
	if spec.Rotation != 0 {
		props = append(props, fmt.Sprintf("--%s-rotation: %gdeg;", prefix, spec.Rotation))
	}

	for i, fill := range spec.Fills {
		if fill.Type == "SOLID" && fill.Visible && fill.Color != "" {
			props = append(props, fmt.Sprintf("--%s-fill-%d: %s;", prefix, i, fill.Color))
			if fill.Opacity < 1 {
				props = append(props, fmt.Sprintf("--%s-fill-%d-opacity: %g;", prefix, i, fill.Opacity))
			}
		}
		if fill.Type != "SOLID" && fill.Gradient != nil {
			grad := gradientToCSS(fill.Gradient, fill.Type)
			props = append(props, fmt.Sprintf("--%s-fill-%d: %s;", prefix, i, grad))
		}
		if fill.Type == "IMAGE" && fill.Image != nil {
			props = append(props, fmt.Sprintf("--%s-fill-%d-image: url('%s');", prefix, i, fill.Image.Name))
		}
	}

	for i, stroke := range spec.Strokes {
		if stroke.Width > 0 {
			props = append(props, fmt.Sprintf("--%s-stroke-%d-width: %gpx;", prefix, i, stroke.Width))
			if stroke.Color != "" {
				props = append(props, fmt.Sprintf("--%s-stroke-%d-color: %s;", prefix, i, stroke.Color))
			}
		}
	}

	for i, effect := range spec.Effects {
		if effect.Type == "DROP_SHADOW" || effect.Type == "INNER_SHADOW" {
			prefix := ""
			if effect.Type == "INNER_SHADOW" {
				prefix = "inset "
			}
			shadow := fmt.Sprintf("%s%gpx %gpx %gpx %gpx %s",
				prefix, effect.Offset.X, effect.Offset.Y, effect.Radius, effect.Spread, effect.Color)
			props = append(props, fmt.Sprintf("--%s-shadow-%d: %s;", prefix, i, shadow))
		}
	}

	return strings.Join(props, " ")
}

func gradientToCSS(g *ir.GradientSpec, fillType string) string {
	var stops []string
	for _, stop := range g.Stops {
		color := stop.Color
		if stop.Opacity > 0 && stop.Opacity < 1 {
			// Approximate alpha in color
			color = fmt.Sprintf("%s / %g", color, stop.Opacity)
		}
		stops = append(stops, fmt.Sprintf("%s %g%%", color, stop.Position*100))
	}
	switch fillType {
	case "GRADIENT_LINEAR":
		angle := g.Angle
		if angle == 0 {
			angle = 180
		}
		return fmt.Sprintf("linear-gradient(%gdeg, %s)", angle, strings.Join(stops, ", "))
	case "GRADIENT_RADIAL":
		shape := "circle"
		if g.InnerRadius > 0 {
			shape = "ellipse"
		}
		return fmt.Sprintf("radial-gradient(%s at %g%% %g%%, %s)", shape, g.StartPoint.X*100, g.StartPoint.Y*100, strings.Join(stops, ", "))
	case "GRADIENT_ANGULAR":
		return fmt.Sprintf("conic-gradient(from %gdeg at %g%% %g%%, %s)", g.Angle, g.StartPoint.X*100, g.StartPoint.Y*100, strings.Join(stops, ", "))
	}
	return ""
}