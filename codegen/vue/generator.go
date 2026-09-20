package vue

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/codegen/tokens"
	"github.com/salvadorsru/dali/codegen/util"
)

// VueGenerator implements Generator for Vue 3 + Tailwind.
type VueGenerator struct{}

func init() {
	ir.RegisterGenerator("vue", func() ir.Generator {
		return &VueGenerator{}
	})
}

// GenerateComponent generates a Vue component with Tailwind classes.
func (g *VueGenerator) GenerateComponent(spec ir.ComponentSpec, tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	// Script setup
	b.WriteString("<script setup")
	if config.UseTypeScript {
		b.WriteString(" lang=\"ts\"")
	}
	b.WriteString(">\n")

	// Props
	if len(spec.Variants) > 0 || len(spec.Props) > 0 {
		b.WriteString("defineProps<{\n")
		if len(spec.Variants) > 0 {
			variantNames := make([]string, len(spec.Variants))
			for i, v := range spec.Variants {
				variantNames[i] = fmt.Sprintf("'%s'", v.Name)
			}
			b.WriteString(fmt.Sprintf("  variant?: %s;\n", strings.Join(variantNames, " | ")))
		}
		for _, prop := range spec.Props {
			propName := util.SanitizePropName(prop.Name)
			optional := ""
			if !prop.Required {
				optional = "?"
			}
			b.WriteString(fmt.Sprintf("  %s%s: %s;\n", propName, optional, prop.Type))
		}
		b.WriteString("  children?: any;\n")
		b.WriteString("  class?: string;\n")
		b.WriteString("}>\n")
	} else {
		b.WriteString("defineProps<{\n")
		b.WriteString("  children?: any;\n")
		b.WriteString("  class?: string;\n")
		b.WriteString("}>\n")
	}

	b.WriteString("</script>\n\n")

	// Template
	b.WriteString("<template>\n")
	b.WriteString(g.renderElement(spec, "  "))
	b.WriteString("</template>\n\n")

	// Styles (if using CSS vars)
	if config.Styling == "css-vars" {
		b.WriteString("<style>\n")
		b.WriteString(util.LayoutToCSSVars(spec.Layout, "component"))
		b.WriteString("\n")
		b.WriteString(util.StyleToCSSVars(spec.Styles, "component"))
		b.WriteString("\n</style>\n")
	}

	return b.String(), nil
}

// GenerateTokens generates CSS variables for tokens.
func (g *VueGenerator) GenerateTokens(set ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	tg := tokens.TokenGenerator{}
	return tg.GenerateCSSVariables(set, config)
}

// GenerateIndex generates an index.ts exporting all components.
func (g *VueGenerator) GenerateIndex(components []ir.ComponentSpec, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	for _, comp := range components {
		name := util.SanitizeComponentName(comp.Name)
		b.WriteString(fmt.Sprintf("export { default as %s } from './%s.vue';\n", name, name))
	}

	return b.String(), nil
}

func (g *VueGenerator) renderElement(spec ir.ComponentSpec, indent string) string {
	var b strings.Builder
	
	// Use first child as root element if available
	var root *ir.NodeSpec
	if len(spec.Children) > 0 {
		root = &spec.Children[0]
	} else {
		// Create a root node from component spec
		root = &ir.NodeSpec{
			ID:       spec.ID,
			Name:     spec.Name,
			Type:     spec.Type,
			Layout:   spec.Layout,
			Styles:   spec.Styles,
			Children: spec.Children,
			Text:     ir.TextSpec{},
		}
	}

	tag := g.nodeTypeToTag(root.Type)

	classes := g.buildClasses(*root)
	classBinding := ""
	if classes != "" {
		classBinding = fmt.Sprintf(" :class=\"[%s, $attrs.class]\"", classes)
	}

	b.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, classBinding))

	// Slots for children
	if len(root.Children) > 0 {
		b.WriteString(fmt.Sprintf("%s  <slot />\n", indent))
	} else if root.Text.Content != "" {
		textIndent := indent + "  "
		if len(root.Text.Runs) > 0 {
			b.WriteString(fmt.Sprintf("%s<template v-for=\"run in runs\" :key=\"run.start\">\n", textIndent))
			b.WriteString(fmt.Sprintf("%s  <span :style=\"runStyle(run)\">{{ run.text }}</span>\n", textIndent))
			b.WriteString(fmt.Sprintf("%s</template>\n", textIndent))
		} else {
			textClasses := util.TextToTailwind(root.Text)
			if textClasses != "" {
				b.WriteString(fmt.Sprintf("%s<span class=\"%s\">{{ text }}</span>\n", textIndent, textClasses))
			} else {
				b.WriteString(fmt.Sprintf("%s{{ text }}\n", textIndent))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *VueGenerator) buildClasses(spec ir.NodeSpec) string {
	var parts []string

	if spec.Layout.Mode != "NONE" || spec.Layout.Gap > 0 {
		parts = append(parts, fmt.Sprintf("`%s`", util.LayoutToTailwind(spec.Layout)))
	}
	if len(spec.Styles.Fills) > 0 || len(spec.Styles.Strokes) > 0 || len(spec.Styles.Effects) > 0 {
		parts = append(parts, fmt.Sprintf("`%s`", util.StyleToTailwind(spec.Styles)))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func (g *VueGenerator) nodeTypeToTag(nodeType string) string {
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