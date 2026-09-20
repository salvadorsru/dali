package tokens

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/query"
)

// TokenGenerator generates design token outputs in various formats.
type TokenGenerator struct{}

// GenerateCSSVariables generates CSS custom properties from tokens.
func (g *TokenGenerator) GenerateCSSVariables(tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder
	prefix := config.CSSVarPrefix
	if prefix == "" {
		prefix = "dali"
	}

	b.WriteString(":root {\n")

	// Solid colors
	for _, c := range tokens.Colors {
		name := strings.ToLower(strings.ReplaceAll(c.Hex, "#", ""))
		b.WriteString(fmt.Sprintf("  --%s-color-%s: %s;\n", prefix, name, c.Hex))
	}

	// Color variables with modes
	for _, v := range tokens.ColorVars {
		for _, mode := range v.Modes {
			b.WriteString(fmt.Sprintf("  --%s-color-%s-%s: %s;\n", prefix, v.Name, mode.Mode, mode.Value))
		}
	}

	// Font families
	seenFonts := make(map[string]bool)
	for _, f := range tokens.Fonts {
		key := strings.ToLower(strings.ReplaceAll(f.Family, " ", "-"))
		if !seenFonts[key] {
			seenFonts[key] = true
			b.WriteString(fmt.Sprintf("  --%s-font-%s: '%s', sans-serif;\n", prefix, key, f.Family))
		}
	}

	// Text classes as CSS variables
	for _, tc := range tokens.TextClasses {
		key := fmt.Sprintf("%s-%s-%g", strings.ToLower(strings.ReplaceAll(tc.Family, " ", "-")), strings.ToLower(tc.Style), tc.Size)
		b.WriteString(fmt.Sprintf("  --%s-text-%s: %gpx;\n", prefix, key, tc.Size))
	}

	// Layout tokens
	for i, l := range tokens.Layouts {
		b.WriteString(fmt.Sprintf("  --%s-layout-%d: %s;\n", prefix, i, l.Layout))
	}

	b.WriteString("}\n")

	return b.String(), nil
}

// GenerateStyleDictionary generates Style Dictionary compatible JSON.
func (g *TokenGenerator) GenerateStyleDictionary(tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	b.WriteString("{\n")

	// Colors
	if len(tokens.Colors) > 0 {
		b.WriteString("  \"color\": {\n")
		for _, c := range tokens.Colors {
			name := strings.ToLower(strings.ReplaceAll(c.Hex, "#", ""))
			b.WriteString(fmt.Sprintf("    \"%s\": {\n", name))
			b.WriteString(fmt.Sprintf("      \"value\": \"%s\",\n", c.Hex))
			b.WriteString("      \"type\": \"color\"\n")
			b.WriteString("    },\n")
		}
		b.WriteString("  },\n")
	}

	// Color variables
	if len(tokens.ColorVars) > 0 {
		b.WriteString("  \"colorVariable\": {\n")
		for _, v := range tokens.ColorVars {
			b.WriteString(fmt.Sprintf("    \"%s\": {\n", toCamelCase(v.Name)))
			for _, mode := range v.Modes {
				b.WriteString(fmt.Sprintf("      \"%s\": {\n", toCamelCase(mode.Mode)))
				b.WriteString(fmt.Sprintf("        \"value\": \"%s\",\n", mode.Value))
				b.WriteString("        \"type\": \"color\"\n")
				b.WriteString("      },\n")
			}
			b.WriteString("    },\n")
		}
		b.WriteString("  },\n")
	}

	// Fonts
	if len(tokens.Fonts) > 0 {
		b.WriteString("  \"fontFamily\": {\n")
		seen := make(map[string]bool)
		for _, f := range tokens.Fonts {
			key := toCamelCase(f.Family)
			if !seen[key] {
				seen[key] = true
				b.WriteString(fmt.Sprintf("    \"%s\": {\n", key))
				b.WriteString(fmt.Sprintf("      \"value\": \"%s\",\n", f.Family))
				b.WriteString("      \"type\": \"fontFamily\"\n")
				b.WriteString("    },\n")
			}
		}
		b.WriteString("  },\n")
	}

	// Font sizes
	if len(tokens.TextClasses) > 0 {
		b.WriteString("  \"fontSize\": {\n")
		for _, tc := range tokens.TextClasses {
			key := fmt.Sprintf("%s%s%g", toPascalCase(tc.Family), toPascalCase(tc.Style), tc.Size)
			b.WriteString(fmt.Sprintf("    \"%s\": {\n", key))
			b.WriteString(fmt.Sprintf("      \"value\": \"%gpx\",\n", tc.Size))
			b.WriteString("      \"type\": \"dimension\"\n")
			b.WriteString("    },\n")
		}
		b.WriteString("  },\n")
	}

	// Spacing
	if len(tokens.Layouts) > 0 {
		b.WriteString("  \"spacing\": {\n")
		for i, l := range tokens.Layouts {
			b.WriteString(fmt.Sprintf("    \"layout%d\": {\n", i))
			b.WriteString(fmt.Sprintf("      \"value\": \"%s\",\n", l.Layout))
			b.WriteString("      \"type\": \"spacing\"\n")
			b.WriteString("    },\n")
		}
		b.WriteString("  }\n")
	}

	b.WriteString("}\n")

	return b.String(), nil
}

// GenerateFigmaTokens generates Figma Tokens plugin compatible JSON.
func (g *TokenGenerator) GenerateFigmaTokens(tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	b.WriteString("{\n")
	b.WriteString("  \"$themes\": [],\n")
	b.WriteString("  \"$metadata\": {\n")
	b.WriteString("    \"tokenSetOrder\": [\"color\", \"fontFamily\", \"fontSize\", \"spacing\"]\n")
	b.WriteString("  },\n")

	// Global color tokens
	if len(tokens.Colors) > 0 {
		b.WriteString("  \"color\": {\n")
		for _, c := range tokens.Colors {
			name := strings.ToLower(strings.ReplaceAll(c.Hex, "#", ""))
			b.WriteString(fmt.Sprintf("    \"%s\": {\n", name))
			b.WriteString(fmt.Sprintf("      \"$value\": \"%s\",\n", c.Hex))
			b.WriteString("      \"$type\": \"color\",\n")
			b.WriteString(fmt.Sprintf("      \"$description\": \"Usage count: %d\"\n", c.Usage))
			b.WriteString("    },\n")
		}
		b.WriteString("  },\n")
	}

	// Color variables with modes as themes
	if len(tokens.ColorVars) > 0 {
		// Collect unique modes
		modeSet := make(map[string]bool)
		for _, v := range tokens.ColorVars {
			for _, mode := range v.Modes {
				modeSet[mode.Mode] = true
			}
		}
		modes := make([]string, 0, len(modeSet))
		for m := range modeSet {
			modes = append(modes, m)
		}

		b.WriteString("  \"$themes\": [\n")
		for _, m := range modes {
			b.WriteString(fmt.Sprintf("    \"%s\",\n", m))
		}
		b.WriteString("  ],\n")

		b.WriteString("  \"colorVariable\": {\n")
		for _, v := range tokens.ColorVars {
			b.WriteString(fmt.Sprintf("    \"%s\": {\n", toCamelCase(v.Name)))
			b.WriteString("      \"$type\": \"color\",\n")
			for _, mode := range v.Modes {
				b.WriteString(fmt.Sprintf("      \"%s\": {\n", toCamelCase(mode.Mode)))
				b.WriteString(fmt.Sprintf("        \"$value\": \"%s\",\n", mode.Value))
				b.WriteString("        \"$type\": \"color\"\n")
				b.WriteString("      },\n")
			}
			b.WriteString("    },\n")
		}
		b.WriteString("  },\n")
	}

	// Typography
	if len(tokens.Fonts) > 0 || len(tokens.TextClasses) > 0 {
		b.WriteString("  \"typography\": {\n")

		if len(tokens.Fonts) > 0 {
			b.WriteString("    \"fontFamily\": {\n")
			seen := make(map[string]bool)
			for _, f := range tokens.Fonts {
				key := toCamelCase(f.Family)
				if !seen[key] {
					seen[key] = true
					b.WriteString(fmt.Sprintf("      \"%s\": {\n", key))
					b.WriteString(fmt.Sprintf("        \"$value\": \"%s\",\n", f.Family))
					b.WriteString("        \"$type\": \"fontFamily\"\n")
					b.WriteString("      },\n")
				}
			}
			b.WriteString("    },\n")
		}

		if len(tokens.TextClasses) > 0 {
			b.WriteString("    \"fontSize\": {\n")
			for _, tc := range tokens.TextClasses {
				key := fmt.Sprintf("%s%s%g", toPascalCase(tc.Family), toPascalCase(tc.Style), tc.Size)
				b.WriteString(fmt.Sprintf("      \"%s\": {\n", key))
				b.WriteString(fmt.Sprintf("        \"$value\": \"%gpx\",\n", tc.Size))
				b.WriteString("        \"$type\": \"dimension\"\n")
				b.WriteString("      },\n")
			}
			b.WriteString("    },\n")
		}

		b.WriteString("  },\n")
	}

	// Spacing
	if len(tokens.Layouts) > 0 {
		b.WriteString("  \"spacing\": {\n")
		for i, l := range tokens.Layouts {
			b.WriteString(fmt.Sprintf("    \"layout%d\": {\n", i))
			b.WriteString(fmt.Sprintf("      \"$value\": \"%s\",\n", l.Layout))
			b.WriteString("      \"$type\": \"spacing\"\n")
			b.WriteString("    },\n")
		}
		b.WriteString("  }\n")
	}

	b.WriteString("}\n")

	return b.String(), nil
}

// CollectTokens gathers all tokens from a document via the query service.
func CollectTokens(svc *query.Service) (ir.TokenSet, error) {
	var ts ir.TokenSet

	// Colors
	colors, err := svc.Palette(0, 256)
	if err != nil {
		return ts, err
	}
	ts.Colors = colors

	// Color variables
	vars, err := svc.Variables()
	if err != nil {
		return ts, err
	}
	for _, v := range vars {
		if v.Type == "COLOR" {
			ts.ColorVars = append(ts.ColorVars, v)
		}
	}

	// Fonts
	fonts, err := svc.Fonts()
	if err != nil {
		return ts, err
	}
	ts.Fonts = fonts

	// Text classes
	textClasses, err := svc.TextClasses(200)
	if err != nil {
		return ts, err
	}
	ts.TextClasses = textClasses

	// Layouts
	layouts, err := svc.TokenLayouts(200)
	if err != nil {
		return ts, err
	}
	ts.Layouts = layouts

	return ts, nil
}

func toCamelCase(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func toPascalCase(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}