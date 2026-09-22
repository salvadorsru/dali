package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/internal/codegen/ir"
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

// StyleToInline converts ir.StyleSpec to an inline CSS declaration list.
// Opacity and rotation are emitted by the generator alongside layout.
func StyleToInline(spec ir.StyleSpec) string {
	var props []string

	for _, fill := range spec.Fills {
		if fill.Type == "SOLID" && fill.Visible && fill.Color != "" {
			props = append(props, fmt.Sprintf("background-color:%s", fill.Color))
			break
		}
	}

	for _, stroke := range spec.Strokes {
		if stroke.Width > 0 {
			props = append(props, fmt.Sprintf("border-width:%gpx", stroke.Width))
			if stroke.Color != "" {
				props = append(props, fmt.Sprintf("border-color:%s", stroke.Color))
			}
			break
		}
	}

	for _, effect := range spec.Effects {
		if (effect.Type == "DROP_SHADOW" || effect.Type == "INNER_SHADOW") && effect.Radius > 0 {
			prefix := ""
			if effect.Type == "INNER_SHADOW" {
				prefix = "inset "
			}
			props = append(props, fmt.Sprintf("box-shadow:%s%gpx %gpx %gpx %gpx %s",
				prefix, effect.Offset.X, effect.Offset.Y, effect.Radius, effect.Spread, effect.Color))
		}
		if (effect.Type == "LAYER_BLUR" || effect.Type == "BACKGROUND_BLUR") && effect.Radius > 0 {
			props = append(props, fmt.Sprintf("backdrop-filter:blur(%gpx)", effect.Radius))
		}
	}

	if spec.ClipContent {
		props = append(props, "overflow:hidden")
	}
	if !spec.Visible {
		props = append(props, "visibility:hidden")
	}

	return strings.Join(props, "; ")
}