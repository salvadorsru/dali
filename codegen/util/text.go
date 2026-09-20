package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
)

// TranslateText converts a TEXT node to ir.TextSpec.
func TranslateText(node *model.Node) ir.TextSpec {
	spec := ir.TextSpec{
		Content:       node.Text,
		FontFamily:    node.FontFamily,
		FontStyle:     node.FontStyle,
		FontSize:      node.FontSize,
		LineHeight:    node.LineHeight,
		LetterSpacing: node.LetterSpacing,
		TextAlign:     node.TextAlign,
		Decoration:    node.Decoration,
	}

	// Get color from fills
	if hex, ok := node.FillHex(); ok {
		spec.Color = hex
	}

	// Text runs
	for _, run := range node.Runs {
		spec.Runs = append(spec.Runs, ir.TextRunSpec{
			Start:          run.Start,
			Length:         run.Len,
			Text:           run.Text,
			FontFamily:     run.FontFamily,
			FontStyle:      run.FontStyle,
			FontSize:       run.FontSize,
			Color:          run.Color,
			Decoration:     run.Decoration,
			LineHeight:     run.LineHeight,
			LetterSpacing:  run.LetterSpacing,
		})
	}

	return spec
}

// TextToTailwind converts ir.TextSpec to Tailwind classes.
func TextToTailwind(spec ir.TextSpec) string {
	var classes []string

	// Font family - map to Tailwind font classes or use arbitrary
	if spec.FontFamily != "" {
		classes = append(classes, fmt.Sprintf("font-[%q]", spec.FontFamily))
	}

	// Font style/weight
	if spec.FontStyle != "" {
		weight := fontStyleToWeight(spec.FontStyle)
		if weight != "" {
			classes = append(classes, fmt.Sprintf("font-%s", weight))
		}
	}

	// Font size
	if spec.FontSize > 0 {
		classes = append(classes, fmt.Sprintf("text-[%gpx]", spec.FontSize))
	}

	// Line height
	if spec.LineHeight > 0 {
		classes = append(classes, fmt.Sprintf("leading-[%g]", spec.LineHeight))
	}

	// Letter spacing
	if spec.LetterSpacing != 0 {
		classes = append(classes, fmt.Sprintf("tracking-[%gem]", spec.LetterSpacing/1000))
	}

	// Text align
	switch strings.ToUpper(spec.TextAlign) {
	case "CENTER":
		classes = append(classes, "text-center")
	case "RIGHT", "END":
		classes = append(classes, "text-right")
	case "JUSTIFIED":
		classes = append(classes, "text-justify")
	}

	// Decoration
	if spec.Decoration != "" {
		switch strings.ToUpper(spec.Decoration) {
		case "UNDERLINE":
			classes = append(classes, "underline")
		case "STRIKETHROUGH":
			classes = append(classes, "line-through")
		}
	}

	// Color
	if spec.Color != "" {
		classes = append(classes, fmt.Sprintf("text-[%s]", spec.Color))
	}

	return strings.Join(classes, " ")
}

// TextToCSSVars converts ir.TextSpec to CSS custom properties.
func TextToCSSVars(spec ir.TextSpec, prefix string) string {
	var props []string

	if spec.FontFamily != "" {
		props = append(props, fmt.Sprintf("--%s-font-family: %q;", prefix, spec.FontFamily))
	}
	if spec.FontStyle != "" {
		props = append(props, fmt.Sprintf("--%s-font-style: %s;", prefix, spec.FontStyle))
	}
	if spec.FontSize > 0 {
		props = append(props, fmt.Sprintf("--%s-font-size: %gpx;", prefix, spec.FontSize))
	}
	if spec.LineHeight > 0 {
		props = append(props, fmt.Sprintf("--%s-line-height: %g;", prefix, spec.LineHeight))
	}
	if spec.LetterSpacing != 0 {
		props = append(props, fmt.Sprintf("--%s-letter-spacing: %gem;", prefix, spec.LetterSpacing/1000))
	}
	if spec.TextAlign != "" {
		props = append(props, fmt.Sprintf("--%s-text-align: %s;", prefix, strings.ToLower(spec.TextAlign)))
	}
	if spec.Decoration != "" {
		props = append(props, fmt.Sprintf("--%s-text-decoration: %s;", prefix, strings.ToLower(spec.Decoration)))
	}
	if spec.Color != "" {
		props = append(props, fmt.Sprintf("--%s-color: %s;", prefix, spec.Color))
	}

	return strings.Join(props, " ")
}

// fontStyleToWeight maps Figma font styles to Tailwind font weights.
func fontStyleToWeight(style string) string {
	s := strings.ToLower(style)
	switch {
	case strings.Contains(s, "thin") || strings.Contains(s, "hairline"):
		return "thin"
	case strings.Contains(s, "extralight") || strings.Contains(s, "ultralight"):
		return "extralight"
	case strings.Contains(s, "light"):
		return "light"
	case strings.Contains(s, "regular") || strings.Contains(s, "normal") || strings.Contains(s, "book"):
		return "normal"
	case strings.Contains(s, "medium"):
		return "medium"
	case strings.Contains(s, "semibold") || strings.Contains(s, "demibold"):
		return "semibold"
	case strings.Contains(s, "bold"):
		return "bold"
	case strings.Contains(s, "extrabold") || strings.Contains(s, "ultrabold"):
		return "extrabold"
	case strings.Contains(s, "black") || strings.Contains(s, "heavy"):
		return "black"
	}
	return ""
}

// TextRunToTailwind converts a ir.TextRunSpec to inline style string.
func TextRunToTailwind(run ir.TextRunSpec) string {
	var styles []string

	if run.FontFamily != "" {
		styles = append(styles, fmt.Sprintf("font-family:%q", run.FontFamily))
	}
	if run.FontStyle != "" {
		if w := fontStyleToWeight(run.FontStyle); w != "" {
			styles = append(styles, fmt.Sprintf("font-weight:%s", w))
		}
	}
	if run.FontSize > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%gpx", run.FontSize))
	}
	if run.Color != "" {
		styles = append(styles, fmt.Sprintf("color:%s", run.Color))
	}
	if run.Decoration != "" {
		styles = append(styles, fmt.Sprintf("text-decoration:%s", strings.ToLower(run.Decoration)))
	}
	if run.LineHeight > 0 {
		styles = append(styles, fmt.Sprintf("line-height:%g", run.LineHeight))
	}
	if run.LetterSpacing != 0 {
		styles = append(styles, fmt.Sprintf("letter-spacing:%gem", run.LetterSpacing/1000))
	}

	return strings.Join(styles, "; ")
}