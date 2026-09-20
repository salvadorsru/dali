package html

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/codegen/tokens"
	"github.com/salvadorsru/dali/codegen/util"
)

// HTMLGenerator implements Generator for semantic HTML + Tailwind.
type HTMLGenerator struct{}

// GenerateComponent generates semantic HTML with CSS variables.
func (g *HTMLGenerator) GenerateComponent(spec ir.ComponentSpec, tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	name := util.SanitizeComponentName(spec.Name)

	// CSS variables for this component
	b.WriteString(":root {\n")
	b.WriteString(util.LayoutToCSSVars(spec.Layout, fmt.Sprintf("comp-%s", strings.ToLower(name))))
	b.WriteString("\n")
	b.WriteString(util.StyleToCSSVars(spec.Styles, fmt.Sprintf("comp-%s", strings.ToLower(name))))
	b.WriteString("\n}\n\n")

	// HTML template - render first child as root, or component itself
	b.WriteString("<!-- Component: " + spec.Name + " -->\n")
	root := ir.NodeSpec{
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
	if len(spec.Children) > 0 {
		root = spec.Children[0]
	}
	b.WriteString(g.renderElement(root, ""))

	return b.String(), nil
}

// GenerateTokens generates CSS variables for tokens.
func (g *HTMLGenerator) GenerateTokens(set ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	tg := tokens.TokenGenerator{}
	return tg.GenerateCSSVariables(set, config)
}

// GenerateIndex generates an HTML file importing all components.
func (g *HTMLGenerator) GenerateIndex(components []ir.ComponentSpec, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString("<html lang=\"en\">\n<head>\n")
	b.WriteString("  <meta charset=\"UTF-8\">\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	b.WriteString("  <title>Design System Components</title>\n")
	b.WriteString("  <link rel=\"stylesheet\" href=\"tokens.css\">\n")
	b.WriteString("</head>\n<body>\n")

	for _, comp := range components {
		name := util.SanitizeComponentName(comp.Name)
		b.WriteString(fmt.Sprintf("  <section id=\"%s\">\n", strings.ToLower(name)))
		b.WriteString(fmt.Sprintf("    <h2>%s</h2>\n", comp.Name))
		b.WriteString(fmt.Sprintf("    <div class=\"component-preview\" data-component=\"%s\"></div>\n", name))
		b.WriteString("  </section>\n")
	}

	b.WriteString("  <script type=\"module\" src=\"index.js\"></script>\n")
	b.WriteString("</body>\n</html>\n")

	return b.String(), nil
}

func (g *HTMLGenerator) renderElement(spec ir.NodeSpec, indent string) string {
	var b strings.Builder
	tag := g.nodeTypeToTag(spec.Type)

	classes := g.buildClasses(spec)
	classAttr := ""
	if classes != "" {
		classAttr = fmt.Sprintf(" class=\"%s\"", classes)
	}

	styleAttr := g.buildInlineStyles(spec)
	style := ""
	if styleAttr != "" {
		style = fmt.Sprintf(" style=\"%s\"", styleAttr)
	}

	b.WriteString(fmt.Sprintf("%s<%s%s%s>\n", indent, tag, classAttr, style))

	// Children
	for _, child := range spec.Children {
		b.WriteString(g.renderChildElement(child, indent+"  "))
	}

	// Text content
	if spec.Text.Content != "" {
		textIndent := indent + "  "
		if len(spec.Text.Runs) > 0 {
			b.WriteString(fmt.Sprintf("%s<!-- Rich text runs -->\n", textIndent))
			for _, run := range spec.Text.Runs {
				runStyle := util.TextRunToTailwind(run)
				if runStyle != "" {
					b.WriteString(fmt.Sprintf("%s<span style=\"%s\">%s</span>\n", textIndent, runStyle, run.Text))
				} else {
					b.WriteString(fmt.Sprintf("%s%s\n", textIndent, run.Text))
				}
			}
		} else {
			textClasses := util.TextToTailwind(spec.Text)
			if textClasses != "" {
				b.WriteString(fmt.Sprintf("%s<span class=\"%s\">%s</span>\n", textIndent, textClasses, spec.Text.Content))
			} else {
				b.WriteString(fmt.Sprintf("%s%s\n", textIndent, spec.Text.Content))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *HTMLGenerator) renderChildElement(child ir.NodeSpec, indent string) string {
	var b strings.Builder
	tag := g.nodeTypeToTag(child.Type)

	classes := g.buildChildClasses(child)
	classAttr := ""
	if classes != "" {
		classAttr = fmt.Sprintf(" class=\"%s\"", classes)
	}

	styleAttr := g.buildInlineStylesChild(child)
	style := ""
	if styleAttr != "" {
		style = fmt.Sprintf(" style=\"%s\"", styleAttr)
	}

	b.WriteString(fmt.Sprintf("%s<%s%s%s>\n", indent, tag, classAttr, style))

	for _, grandchild := range child.Children {
		b.WriteString(g.renderChildElement(grandchild, indent+"  "))
	}

	if child.Text.Content != "" {
		textIndent := indent + "  "
		if len(child.Text.Runs) > 0 {
			b.WriteString(fmt.Sprintf("%s<!-- Rich text runs -->\n", textIndent))
			for _, run := range child.Text.Runs {
				runStyle := util.TextRunToTailwind(run)
				if runStyle != "" {
					b.WriteString(fmt.Sprintf("%s<span style=\"%s\">%s</span>\n", textIndent, runStyle, run.Text))
				} else {
					b.WriteString(fmt.Sprintf("%s%s\n", textIndent, run.Text))
				}
			}
		} else {
			textClasses := util.TextToTailwind(child.Text)
			if textClasses != "" {
				b.WriteString(fmt.Sprintf("%s<span class=\"%s\">%s</span>\n", textIndent, textClasses, child.Text.Content))
			} else {
				b.WriteString(fmt.Sprintf("%s%s\n", textIndent, child.Text.Content))
			}
		}
	}

	if child.Vector.Path != "" {
		b.WriteString(fmt.Sprintf("%s  <svg viewBox=\"0 0 100 100\" style=\"width:100%%;height:100%%\"><path d=\"%s\" fill=\"currentColor\" /></svg>\n", indent, child.Vector.Path))
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *HTMLGenerator) buildClasses(spec ir.NodeSpec) string {
	var parts []string

	// Semantic class based on component name
	name := util.SanitizeComponentName(spec.Name)
	parts = append(parts, fmt.Sprintf("comp-%s", strings.ToLower(name)))

	// Layout classes as utility classes
	if spec.Layout.Mode != "NONE" || spec.Layout.Gap > 0 {
		parts = append(parts, util.LayoutToTailwind(spec.Layout))
	}
	if len(spec.Styles.Fills) > 0 || len(spec.Styles.Strokes) > 0 || len(spec.Styles.Effects) > 0 {
		parts = append(parts, util.StyleToTailwind(spec.Styles))
	}

	return strings.Join(parts, " ")
}

func (g *HTMLGenerator) buildChildClasses(child ir.NodeSpec) string {
	var parts []string

	if child.Layout.Mode != "NONE" || child.Layout.Gap > 0 {
		parts = append(parts, util.LayoutToTailwind(child.Layout))
	}
	if len(child.Styles.Fills) > 0 || len(child.Styles.Strokes) > 0 || len(child.Styles.Effects) > 0 {
		parts = append(parts, util.StyleToTailwind(child.Styles))
	}

	return strings.Join(parts, " ")
}

func (g *HTMLGenerator) buildInlineStyles(spec ir.NodeSpec) string {
	var styles []string

	if spec.Styles.Opacity > 0 && spec.Styles.Opacity < 1 {
		styles = append(styles, fmt.Sprintf("opacity: %g", spec.Styles.Opacity))
	}
	if spec.Styles.Rotation != 0 {
		styles = append(styles, fmt.Sprintf("transform: rotate(%gdeg)", spec.Styles.Rotation))
	}

	return strings.Join(styles, "; ")
}

func (g *HTMLGenerator) buildInlineStylesChild(child ir.NodeSpec) string {
	var styles []string

	if child.Styles.Opacity > 0 && child.Styles.Opacity < 1 {
		styles = append(styles, fmt.Sprintf("opacity: %g", child.Styles.Opacity))
	}
	if child.Styles.Rotation != 0 {
		styles = append(styles, fmt.Sprintf("transform: rotate(%gdeg)", child.Styles.Rotation))
	}

	return strings.Join(styles, "; ")
}

func (g *HTMLGenerator) nodeTypeToTag(nodeType string) string {
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
	case "CANVAS":
		return "main"
	default:
		return "div"
	}
}