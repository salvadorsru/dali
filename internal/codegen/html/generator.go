package html

import (
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/salvadorsru/dali/internal/codegen/ir"
	"github.com/salvadorsru/dali/internal/codegen/util"
)

// HTMLGenerator implements Generator for semantic HTML with inline CSS or
// Tailwind utility classes.
type HTMLGenerator struct {
	style string // inline (default) or tailwind
}

// GenerateComponent generates semantic HTML: inline CSS by default, or
// Tailwind utility classes when config.Style is "tailwind".
func (g *HTMLGenerator) GenerateComponent(spec ir.ComponentSpec, tokens ir.TokenSet, config ir.GeneratorConfig) (string, error) {
	var b strings.Builder

	g.style = config.Style
	if g.style == "" {
		g.style = "inline"
	}

	// HTML template - render first child as root, or component itself
	b.WriteString("<!-- Component: ")
	b.WriteString(spec.Name)
	b.WriteString(" -->\n")
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
	if len(spec.Children) == 1 {
		root = spec.Children[0]
	}
	b.WriteString(g.renderElement(root, ""))

	return b.String(), nil
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
		b.WriteString(g.renderChildElidable(child, indent+"  ", spec.Layout.Mode != "" && spec.Layout.Mode != "NONE"))
	}

	// Text content
	b.WriteString(g.textBlock(spec.Text, indent+"  "))

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

// textBlock renders a node's text content: rich runs as inline-styled spans
// (both modes), otherwise a single span styled per the active style mode.
func (g *HTMLGenerator) textBlock(text ir.TextSpec, textIndent string) string {
	var b strings.Builder
	if text.Content == "" {
		return ""
	}
	if len(text.Runs) > 0 {
		b.WriteString(fmt.Sprintf("%s<!-- Rich text runs -->\n", textIndent))
		for _, run := range text.Runs {
			runStyle := util.TextRunToTailwind(run)
			if runStyle != "" {
				b.WriteString(fmt.Sprintf("%s<span style=\"%s\">%s</span>\n", textIndent, runStyle, stdhtml.EscapeString(run.Text)))
			} else {
				b.WriteString(fmt.Sprintf("%s%s\n", textIndent, stdhtml.EscapeString(run.Text)))
			}
		}
		return b.String()
	}
	if g.style == "inline" {
		textStyle := util.TextToInline(text)
		if textStyle != "" {
			b.WriteString(fmt.Sprintf("%s<span style=\"%s\">%s</span>\n", textIndent, textStyle, stdhtml.EscapeString(text.Content)))
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", textIndent, stdhtml.EscapeString(text.Content)))
		}
	} else {
		textClasses := util.TextToTailwind(text)
		if textClasses != "" {
			b.WriteString(fmt.Sprintf("%s<span class=\"%s\">%s</span>\n", textIndent, textClasses, stdhtml.EscapeString(text.Content)))
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", textIndent, stdhtml.EscapeString(text.Content)))
		}
	}
	return b.String()
}

// renderChildElidable renders a child element, eliding transparent wrappers.
//
// A node that satisfies ir.NodeSpec.PureWrapper — a bare frame or group with no
// text, no vector path, no fills/strokes/effects, no partial opacity or
// rotation, and no layout of its own — contributes no HTML layer of its own: no
// visuals, no flex-box, no highlighted block. Emitting a <div> for it only
// nestrings the output. So when such a node is found, its children are rendered
// in its place:
//   - zero renderable children → nothing is emitted;
//   - a single renderable child  → that child is rendered in place (collapsed);
//   - several renderable children → each is rendered in place (flattened).
//
// The wrapper is kept when it is a direct flex/grid item of an auto-layout
// parent (inFlexContainer). In Figma each auto-layout child is one layout item;
// eliding the wrapper there would merge sibling items and change the layout.
// It is also kept when it would not render as a generic <div> (e.g. a semantic
// <section> or <main> landmark must never be elided).
func (g *HTMLGenerator) renderChildElidable(child ir.NodeSpec, indent string, inFlexContainer bool) string {
	if inFlexContainer || g.nodeTypeToTag(child.Type) != "div" || !child.PureWrapper() {
		return g.renderChildElement(child, indent)
	}
	var b strings.Builder
	for _, grandchild := range child.Children {
		b.WriteString(g.renderChildElidable(grandchild, indent, false))
	}
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

	styleAttr := g.buildInlineStyles(child)
	style := ""
	if styleAttr != "" {
		style = fmt.Sprintf(" style=\"%s\"", styleAttr)
	}

	b.WriteString(fmt.Sprintf("%s<%s%s%s>\n", indent, tag, classAttr, style))

	for _, grandchild := range child.Children {
		b.WriteString(g.renderChildElidable(grandchild, indent+"  ", child.Layout.Mode != "" && child.Layout.Mode != "NONE"))
	}

	b.WriteString(g.textBlock(child.Text, indent+"  "))

	if child.Vector.Path != "" {
		b.WriteString(fmt.Sprintf("%s  <svg viewBox=\"0 0 100 100\" style=\"width:100%%;height:100%%\"><path d=\"%s\" fill=\"currentColor\" /></svg>\n", indent, child.Vector.Path))
	}

	b.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	return b.String()
}

func (g *HTMLGenerator) isInline() bool {
	return g.style == "" || g.style == "inline"
}

func (g *HTMLGenerator) buildClasses(spec ir.NodeSpec) string {
	if g.isInline() {
		return ""
	}
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
	if g.isInline() {
		return ""
	}
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

	if g.isInline() {
		if layout := util.LayoutToInline(spec.Layout); layout != "" {
			styles = append(styles, layout)
		}
		if style := util.StyleToInline(spec.Styles); style != "" {
			styles = append(styles, style)
		}
	}

	if spec.Styles.Opacity > 0 && spec.Styles.Opacity < 1 {
		styles = append(styles, fmt.Sprintf("opacity: %g", spec.Styles.Opacity))
	}
	if spec.Styles.Rotation != 0 {
		styles = append(styles, fmt.Sprintf("transform: rotate(%gdeg)", spec.Styles.Rotation))
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
