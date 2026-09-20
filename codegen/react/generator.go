package react

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/codegen/util"
)

// ReactGenerator implements Generator for React + Tailwind.
type ReactGenerator struct{}

func init() {
	ir.RegisterGenerator("react", func() ir.Generator {
		return &ReactGenerator{}
	})
}

// GenerateComponent generates a React component with Tailwind classes.
func (g *ReactGenerator) GenerateComponent(spec ir.ComponentSpec, tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	// Imports
	b.WriteString(g.generateImports(spec, config))

	// TypeScript interface for props
	if config.UseTypeScript {
		b.WriteString(g.generatePropsInterface(spec, config))
	}

	// Component function
	b.WriteString(g.generateComponentFunction(spec, config))

	// Export
	b.WriteString(fmt.Sprintf("\nexport default %s;\n", util.SanitizeComponentName(spec.Name)))

	return b.String(), nil
}

// GenerateTokens generates CSS variables or Tailwind config for tokens.
func (g *ReactGenerator) GenerateTokens(tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	if config.Styling == "tailwind" {
		b.WriteString(g.generateTailwindConfig(tokens, config))
	} else if config.Styling == "css-vars" {
		b.WriteString(g.generateCSSVariables(tokens, config))
	}

	return b.String(), nil
}

// GenerateIndex generates an index.ts exporting all components.
func (g *ReactGenerator) GenerateIndex(components []ir.ComponentSpec, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	for _, comp := range components {
		name := util.SanitizeComponentName(comp.Name)
		b.WriteString(fmt.Sprintf("export { default as %s } from './%s';\n", name, name))
	}

	return b.String(), nil
}

func (g *ReactGenerator) generateImports(spec ir.ComponentSpec, config ir.GeneratorConfig) string {
	var imports []string

	if config.UseTypeScript {
		imports = append(imports, "import React from 'react';")
	} else {
		imports = append(imports, "import React from 'react';")
	}

	imports = append(imports, "import { clsx } from 'clsx';")

	return strings.Join(imports, "\n") + "\n\n"
}

func (g *ReactGenerator) generatePropsInterface(spec ir.ComponentSpec, config ir.GeneratorConfig) string {
	var b strings.Builder

	name := util.SanitizeComponentName(spec.Name)
	b.WriteString(fmt.Sprintf("export interface %sProps {\n", name))

	// Variant props
	if len(spec.Variants) > 0 {
		variantNames := make([]string, len(spec.Variants))
		for i, v := range spec.Variants {
			variantNames[i] = fmt.Sprintf("'%s'", v.Name)
		}
		b.WriteString(fmt.Sprintf("  variant?: %s;\n", strings.Join(variantNames, " | ")))
	}

	// Regular props
	for _, prop := range spec.Props {
		propName := util.SanitizePropName(prop.Name)
		optional := ""
		if !prop.Required {
			optional = "?"
		}
		b.WriteString(fmt.Sprintf("  %s%s: %s;\n", propName, optional, prop.Type))
	}

	// Children and className
	b.WriteString("  children?: React.ReactNode;\n")
	b.WriteString("  className?: string;\n")
	b.WriteString("}\n\n")

	return b.String()
}

func (g *ReactGenerator) generateComponentFunction(spec ir.ComponentSpec, config ir.GeneratorConfig) string {
	var b strings.Builder

	name := util.SanitizeComponentName(spec.Name)

	if config.UseTypeScript {
		b.WriteString(fmt.Sprintf("export function %s({", name))
		if len(spec.Variants) > 0 {
			b.WriteString("variant, ")
		}
		for _, prop := range spec.Props {
			propName := util.SanitizePropName(prop.Name)
			b.WriteString(fmt.Sprintf("%s, ", propName))
		}
		b.WriteString(fmt.Sprintf("children, className }: %sProps) {\n", name))
	} else {
		b.WriteString(fmt.Sprintf("export function %s({", name))
		if len(spec.Variants) > 0 {
			b.WriteString("variant, ")
		}
		for _, prop := range spec.Props {
			propName := util.SanitizePropName(prop.Name)
			b.WriteString(fmt.Sprintf("%s, ", propName))
		}
		b.WriteString("children, className }) {\n")
	}

	// Build className
	b.WriteString("  const baseClasses = ")
	if config.Styling == "tailwind" {
		b.WriteString(fmt.Sprintf("`%s`;\n", util.LayoutToTailwind(spec.Layout)))
	} else {
		b.WriteString(fmt.Sprintf("`%s`;\n", ""))
	}

	b.WriteString("  const styleClasses = ")
	if config.Styling == "tailwind" {
		b.WriteString(fmt.Sprintf("`%s`;\n", util.StyleToTailwind(spec.Styles)))
	} else {
		b.WriteString(fmt.Sprintf("`%s`;\n", ""))
	}

	// Variant classes
	if len(spec.Variants) > 0 {
		b.WriteString("  const variantClasses = {\n")
		for _, v := range spec.Variants {
			layoutClasses := util.LayoutToTailwind(v.Layout)
			styleClasses := util.StyleToTailwind(v.Styles)
			combined := strings.TrimSpace(layoutClasses + " " + styleClasses)
			b.WriteString(fmt.Sprintf("    %s: `%s`,\n", util.SanitizePropName(v.Name), combined))
		}
		b.WriteString("  }[variant] || '';\n")
	} else {
		b.WriteString("  const variantClasses = '';\n")
	}

	// Prop-based classes
	if len(spec.Props) > 0 {
		b.WriteString("  const propClasses = [\n")
		for _, prop := range spec.Props {
			propName := util.SanitizePropName(prop.Name)
			b.WriteString(fmt.Sprintf("    %s && `%s`,\n", propName, propName))
		}
		b.WriteString("  ].filter(Boolean).join(' ');\n")
	} else {
		b.WriteString("  const propClasses = '';\n")
	}

	b.WriteString("  const classNames = clsx(baseClasses, styleClasses, variantClasses, propClasses, className);\n\n")

	// Render
	b.WriteString("  return (\n")
	b.WriteString(g.renderNode(spec, 4))
	b.WriteString("  );\n")
	b.WriteString("}\n")

	return b.String()
}

func (g *ReactGenerator) renderNode(spec ir.ComponentSpec, indent int) string {
	var b strings.Builder
	indentStr := strings.Repeat("  ", indent)

	// Use the first child as root, or the component itself
	var root ir.NodeSpec
	if len(spec.Children) > 0 {
		root = spec.Children[0]
	} else {
		root = ir.NodeSpec{
			ID:       spec.ID,
			Name:     spec.Name,
			Type:     spec.Type,
			Layout:   spec.Layout,
			Styles:   spec.Styles,
			Children: spec.Children,
			Text:     ir.TextSpec{},
			Vector:   ir.VectorSpec{},
			Images:   spec.Assets,
		}
	}

	b.WriteString(g.renderElement(root, indentStr))

	return b.String()
}

func (g *ReactGenerator) renderElement(spec ir.NodeSpec, indent string) string {
	var b strings.Builder
	tag := g.nodeTypeToTag(spec.Type)

	var classes []string
	if spec.Layout.Mode != "NONE" || spec.Layout.Gap > 0 || len(spec.Styles.Fills) > 0 {
		classes = append(classes, fmt.Sprintf("`%s`", strings.TrimSpace(util.LayoutToTailwind(spec.Layout)+" "+util.StyleToTailwind(spec.Styles))))
	}
	classAttr := ""
	if len(classes) > 0 {
		classAttr = fmt.Sprintf(" className={clsx(%s)}", strings.Join(classes, ", "))
	}

	b.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, classAttr))

	// Children
	for _, child := range spec.Children {
		b.WriteString(g.renderChildElement(child, indent+"  "))
	}

	// Text content
	if jsString(spec.Text.Content) != "" {
		textIndent := indent + "  "
		if len(spec.Text.Runs) > 0 {
			b.WriteString(fmt.Sprintf("%s<>\n", textIndent))
			for _, run := range spec.Text.Runs {
				runStyle := util.TextRunToTailwind(run)
				if runStyle != "" {
					b.WriteString(fmt.Sprintf("%s  <span style={{%s}}>{%s}</span>\n", textIndent, runStyle, jsString(run.Text)))
				} else {
					b.WriteString(fmt.Sprintf("%s  {%s}\n", textIndent, jsString(run.Text)))
				}
			}
			b.WriteString(fmt.Sprintf("%s</>\n", textIndent))
		} else {
			textClasses := util.TextToTailwind(spec.Text)
			if textClasses != "" {
				b.WriteString(fmt.Sprintf("%s<span className={%s}>{%s}</span>\n", textIndent, fmt.Sprintf("`%s`", textClasses), jsString(spec.Text.Content)))
			} else {
				b.WriteString(fmt.Sprintf("%s{%s}\n", textIndent, jsString(spec.Text.Content)))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *ReactGenerator) renderChildElement(child ir.NodeSpec, indent string) string {
	var b strings.Builder
	tag := g.nodeTypeToTag(child.Type)

	var classes []string
	if child.Layout.Mode != "NONE" || child.Layout.Gap > 0 {
		classes = append(classes, fmt.Sprintf("`%s`", util.LayoutToTailwind(child.Layout)))
	}
	if len(child.Styles.Fills) > 0 || len(child.Styles.Strokes) > 0 || len(child.Styles.Effects) > 0 {
		classes = append(classes, fmt.Sprintf("`%s`", util.StyleToTailwind(child.Styles)))
	}
	classAttr := ""
	if len(classes) > 0 {
		classAttr = fmt.Sprintf(" className={clsx(%s)}", strings.Join(classes, ", "))
	}

	b.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, classAttr))

	for _, grandchild := range child.Children {
		b.WriteString(g.renderChildElement(grandchild, indent+"  "))
	}

	if jsString(child.Text.Content) != "" {
		textIndent := indent + "  "
		if len(child.Text.Runs) > 0 {
			b.WriteString(fmt.Sprintf("%s<>\n", textIndent))
			for _, run := range child.Text.Runs {
				runStyle := util.TextRunToTailwind(run)
				if runStyle != "" {
					b.WriteString(fmt.Sprintf("%s  <span style={{%s}}>{%s}</span>\n", textIndent, runStyle, jsString(run.Text)))
				} else {
					b.WriteString(fmt.Sprintf("%s  {%s}\n", textIndent, jsString(run.Text)))
				}
			}
			b.WriteString(fmt.Sprintf("%s</>\n", textIndent))
		} else {
			textClasses := util.TextToTailwind(child.Text)
			if textClasses != "" {
				b.WriteString(fmt.Sprintf("%s<span className={%s}>{%s}</span>\n", textIndent, fmt.Sprintf("`%s`", textClasses), jsString(child.Text.Content)))
			} else {
				b.WriteString(fmt.Sprintf("%s{%s}\n", textIndent, jsString(child.Text.Content)))
			}
		}
	}

	if child.Vector.Path != "" {
		b.WriteString(fmt.Sprintf("%s  <svg viewBox=\"0 0 100 100\" className=\"w-full h-full\"><path d=\"%s\" fill=\"currentColor\" /></svg>\n", indent, child.Vector.Path))
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *ReactGenerator) nodeTypeToTag(nodeType string) string {
	switch strings.ToUpper(nodeType) {
	case "FRAME", "COMPONENT", "COMPONENT_SET", "SYMBOL":
		return "div"
	case "GROUP":
		return "div"
	case "TEXT":
		return "span"
	case "RECTANGLE", "ELLIPSE", "POLYGON", "STAR", "LINE":
		return "div"
	case "VECTOR":
		return "div"
	case "INSTANCE":
		return "div"
	case "SECTION":
		return "section"
	default:
		return "div"
	}
}

func (g *ReactGenerator) generateTailwindConfig(tokens ir.TokenSet, config ir.GeneratorConfig) string {
	var b strings.Builder
	b.WriteString("/** @type {import('tailwindcss').Config} */\n")
	b.WriteString("module.exports = {\n")
	b.WriteString("  theme: {\n")
	b.WriteString("    extend: {\n")

	// Colors
	if len(tokens.Colors) > 0 {
		b.WriteString("      colors: {\n")
		for _, c := range tokens.Colors {
			name := strings.ToLower(strings.ReplaceAll(c.Hex, "#", ""))
			b.WriteString(fmt.Sprintf("        %s: '%s',\n", name, c.Hex))
		}
		b.WriteString("      },\n")
	}

	// Font families
	if len(tokens.Fonts) > 0 {
		b.WriteString("      fontFamily: {\n")
		seen := make(map[string]bool)
		for _, f := range tokens.Fonts {
			if !seen[f.Family] {
				seen[f.Family] = true
				b.WriteString(fmt.Sprintf("        %s: ['%s', 'sans-serif'],\n", strings.ToLower(strings.ReplaceAll(f.Family, " ", "-")), f.Family))
			}
		}
		b.WriteString("      },\n")
	}

	// Spacing from layouts
	if len(tokens.Layouts) > 0 {
		b.WriteString("      spacing: {\n")
		for _, l := range tokens.Layouts {
			// Parse gap from layout summary
			// This is simplified; real impl would parse the layout string
			b.WriteString(fmt.Sprintf("        // %s\n", l.Layout))
		}
		b.WriteString("      },\n")
	}

	b.WriteString("    },\n")
	b.WriteString("  },\n")
	b.WriteString("};\n")

	return b.String()
}

func (g *ReactGenerator) generateCSSVariables(tokens ir.TokenSet, config ir.GeneratorConfig) string {
	var b strings.Builder
	b.WriteString(":root {\n")

	prefix := config.CSSVarPrefix
	if prefix == "" {
		prefix = "dali"
	}

	// Colors
	for _, c := range tokens.Colors {
		name := strings.ToLower(strings.ReplaceAll(c.Hex, "#", ""))
		b.WriteString(fmt.Sprintf("  --%s-color-%s: %s;\n", prefix, name, c.Hex))
	}

	// Color variables
	for _, v := range tokens.ColorVars {
		for _, mode := range v.Modes {
			b.WriteString(fmt.Sprintf("  --%s-color-%s-%s: %s;\n", prefix, v.Name, mode.Mode, mode.Value))
		}
	}

	// Fonts
	for _, f := range tokens.Fonts {
		b.WriteString(fmt.Sprintf("  --%s-font-%s: '%s', sans-serif;\n", prefix, strings.ToLower(strings.ReplaceAll(f.Family, " ", "-")), f.Family))
	}

	// Text classes
	for _, tc := range tokens.TextClasses {
		b.WriteString(fmt.Sprintf("  --%s-text-%s-%s-%g: %gpx;\n", prefix, tc.Family, tc.Style, tc.Size, tc.Size))
	}

	b.WriteString("}\n")

	return b.String()
}

// jsString renders a string as a quoted JavaScript string literal.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}