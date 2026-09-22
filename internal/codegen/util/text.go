package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/internal/codegen/ir"
	"github.com/salvadorsru/dali/internal/model"
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
			Start:         run.Start,
			Length:        run.Len,
			Text:          run.Text,
			FontFamily:    run.FontFamily,
			FontStyle:     run.FontStyle,
			FontSize:      run.FontSize,
			Color:         run.Color,
			Decoration:    run.Decoration,
			LineHeight:    run.LineHeight,
			LetterSpacing: run.LetterSpacing,
		})
	}

	return spec
}

// fontToken prepares a font family for use in a Tailwind arbitrary value:
// single quotes never clash with the HTML attribute delimiters, and
// underscores replace spaces (Tailwind arbitrary values allow no spaces).
func fontToken(family string) string {
	family = strings.ReplaceAll(family, `"`, "")
	family = strings.ReplaceAll(family, `'`, "")
	return strings.ReplaceAll(family, " ", "_")
}

// TextToTailwind converts ir.TextSpec to Tailwind classes.
func TextToTailwind(spec ir.TextSpec) string {
	var classes []string

	// Font family - map to Tailwind font classes or use arbitrary
	if spec.FontFamily != "" {
		classes = append(classes, fmt.Sprintf("font-['%s']", fontToken(spec.FontFamily)))
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

// TextToInline converts ir.TextSpec to an inline CSS declaration list.
func TextToInline(spec ir.TextSpec) string {
	var styles []string

	if spec.FontFamily != "" {
		styles = append(styles, fmt.Sprintf("font-family:'%s'", strings.ReplaceAll(spec.FontFamily, `'`, `\'`)))
	}
	if spec.FontStyle != "" {
		if weight := textWeight(spec.FontStyle); weight != "" {
			styles = append(styles, fmt.Sprintf("font-weight:%s", weight))
		}
	}
	if spec.FontSize > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%gpx", spec.FontSize))
	}
	if spec.LineHeight > 0 {
		styles = append(styles, fmt.Sprintf("line-height:%g", spec.LineHeight))
	}
	if spec.LetterSpacing != 0 {
		styles = append(styles, fmt.Sprintf("letter-spacing:%gem", spec.LetterSpacing/1000))
	}
	switch strings.ToUpper(spec.TextAlign) {
	case "CENTER":
		styles = append(styles, "text-align:center")
	case "RIGHT", "END":
		styles = append(styles, "text-align:right")
	case "JUSTIFIED":
		styles = append(styles, "text-align:justify")
	}
	if spec.Decoration != "" {
		styles = append(styles, fmt.Sprintf("text-decoration:%s", strings.ToLower(spec.Decoration)))
	}
	if spec.Color != "" {
		styles = append(styles, fmt.Sprintf("color:%s", spec.Color))
	}

	return strings.Join(styles, "; ")
}

// textWeight maps a Figma font style to a CSS font-weight number.
func textWeight(style string) string {
	s := strings.ToLower(style)
	switch {
	case strings.Contains(s, "thin") || strings.Contains(s, "hairline"):
		return "100"
	case strings.Contains(s, "extralight") || strings.Contains(s, "ultralight"):
		return "200"
	case strings.Contains(s, "light"):
		return "300"
	case strings.Contains(s, "regular") || strings.Contains(s, "normal") || strings.Contains(s, "book"):
		return "400"
	case strings.Contains(s, "medium"):
		return "500"
	case strings.Contains(s, "semibold") || strings.Contains(s, "demibold"):
		return "600"
	case strings.Contains(s, "bold"):
		return "700"
	case strings.Contains(s, "extrabold") || strings.Contains(s, "ultrabold"):
		return "800"
	case strings.Contains(s, "black") || strings.Contains(s, "heavy"):
		return "900"
	}
	return ""
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
		styles = append(styles, fmt.Sprintf("font-family:'%s'", strings.ReplaceAll(run.FontFamily, `'`, `\'`)))
	}
	if run.FontStyle != "" {
		if w := textWeight(run.FontStyle); w != "" {
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
