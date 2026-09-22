package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/salvadorsru/dali/internal/model"
)

// Severity of a lint finding.
type Severity string

const (
	Info  Severity = "info"
	Warn  Severity = "warn"
	Error Severity = "error"
)

// Issue is one lint finding on a node.
type Issue struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	NodeID   string   `json:"node_id,omitempty"`
	Name     string   `json:"name,omitempty"`
	Type     string   `json:"type,omitempty"`
	Path     string   `json:"path,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// Summary groups issues by rule for a compact CLI report.
func Summary(issues []Issue) map[string]int {
	out := make(map[string]int)
	for _, i := range issues {
		out[i.Rule]++
	}
	return out
}

var genericName = regexp.MustCompile(`(?i)^(frame|container|rectangle|ellipse|vector|group|line|boolean)(\s+\d+.*)?$`)

// LintOptions controls which rules run. Preset is "recommended", "strict" or
// "accessibility".
type LintOptions struct {
	Preset string
}

type lintCtx struct {
	doc    *model.Document
	preset string
	out    []Issue
}

// Lint runs the design-quality and accessibility rules of a preset.
func Lint(doc *model.Document, opts LintOptions) ([]Issue, error) {
	preset := strings.ToLower(strings.TrimSpace(opts.Preset))
	switch preset {
	case "":
		preset = "recommended"
	case "recommended", "strict", "accessibility":
	default:
		return nil, fmt.Errorf("unknown preset %q (recommended, strict, accessibility)", opts.Preset)
	}
	c := &lintCtx{doc: doc, preset: preset}
	for _, n := range doc.All() {
		switch n.Type {
		case model.TypeDocument, model.TypeCanvas, model.TypeVariable, model.TypeVariableSet:
			continue
		}
		c.check(n)
	}
	c.checkUnusedComponents()
	return c.out, nil
}

// emit appends a finding. In recommended the rule's base severity is used,
// strict promotes warn to error, accessibility keeps only accessibility rules
// and reports them as errors.
func (c *lintCtx) emit(rule, group string, base Severity, n *model.Node, detail string) {
	sev := base
	switch c.preset {
	case "strict":
		if sev == Warn {
			sev = Error
		}
	case "accessibility":
		if group != "accessibility" {
			return
		}
		sev = Error
	}
	c.out = append(c.out, Issue{
		Rule:     rule,
		Severity: sev,
		NodeID:   n.ID,
		Name:     n.Name,
		Type:     string(n.Type),
		Path:     c.path(n),
		Detail:   detail,
	})
}

func (c *lintCtx) path(n *model.Node) string {
	parts := make([]string, 0, 8)
	for _, p := range c.doc.Path(n.ID) {
		if p.Name != "" {
			parts = append(parts, p.Name)
		}
	}
	return strings.Join(parts, "/")
}

func (c *lintCtx) check(n *model.Node) {
	if !n.Visible {
		if len(c.doc.Children(n.ID)) > 0 {
			c.emit("hidden-with-children", "quality", Warn, n, "hidden node with children")
		}
	}
	if n.Width == 0 && n.Height == 0 && n.Type != model.TypeText {
		c.emit("zero-size", "quality", Warn, n, "zero-size node")
	}
	if !n.IsComponent() && genericName.MatchString(strings.TrimSpace(n.Name)) && n.Type != model.TypeText && n.Type != model.TypeVector {
		c.emit("generic-name", "quality", Warn, n, "auto-generated name")
	}
	switch {
	case n.Type == model.TypeFrame:
		if len(c.doc.Children(n.ID)) == 0 {
			c.emit("empty-frame", "quality", Warn, n, "frame without content")
		}
	case n.Type == model.TypeText:
		c.checkText(n)
	}
	switch n.Type {
	case model.TypeFrame, model.TypeGroup, model.TypeInstance, model.TypeSymbol, model.TypeComponentSet:
		c.checkTarget(n)
	}
	c.checkDuplicateName(n)
}

func (c *lintCtx) checkDuplicateName(n *model.Node) {
	if n.ParentID == "" {
		return
	}
	siblings := c.doc.Children(n.ParentID)
	count := 0
	for _, s := range siblings {
		if s.ID == n.ID {
			continue
		}
		if s.Name == n.Name && s.Name != "" {
			count++
		}
	}
	if count > 0 {
		c.emit("duplicate-name", "quality", Warn, n, fmt.Sprintf("%d sibling(s) share this name", count))
	}
}

func (c *lintCtx) checkTarget(n *model.Node) {
	minSide := n.Width
	if n.Height < minSide {
		minSide = n.Height
	}
	if minSide > 0 && minSide < 44 {
		c.emit("touch-target", "accessibility", Warn, n, fmt.Sprintf("min side %.0fpx below 44px", minSide))
	}
}

func (c *lintCtx) checkText(n *model.Node) {
	if n.FontSize > 0 {
		threshold := 11.0
		if c.preset == "strict" || c.preset == "accessibility" {
			threshold = 12
		}
		if n.FontSize < threshold {
			c.emit("font-size", "accessibility", Warn, n, fmt.Sprintf("%gpx below %gpx", n.FontSize, threshold))
		}
	}
	if opacity := c.effectiveOpacity(n); opacity > 0 && opacity < 0.5 {
		c.emit("text-opacity", "accessibility", Warn, n, fmt.Sprintf("effective opacity %.2f", opacity))
	}
	fg := ""
	for _, r := range n.Runs {
		if r.Color != "" && r.Len > 0 {
			fg = r.Color
			if fg != "" {
				break
			}
		}
	}
	if fg == "" {
		if hex, ok := n.FillHex(); ok {
			fg = hex
		}
	}
	if fg == "" {
		return
	}
	if bg, ok := c.background(n); ok {
		ratio := contrast(fg, bg)
		threshold := 4.5
		if n.FontSize >= 24 {
			threshold = 3
		}
		if ratio < threshold {
			c.emit("text-contrast", "accessibility", Warn, n,
				fmt.Sprintf("contrast %.2f:1 below %.1f:1 (%s on %s)", ratio, threshold, fg, bg))
		}
	}
}

func (c *lintCtx) effectiveOpacity(n *model.Node) float64 {
	op := 1.0
	for cur := n; cur != nil; cur = c.doc.Parent(cur.ID) {
		op *= cur.Opacity
		if !cur.Visible {
			op = 0
			break
		}
	}
	return op
}

// background walks up from a node until it finds a solid fill to contrast text
// against.
func (c *lintCtx) background(n *model.Node) (string, bool) {
	for cur := c.doc.Parent(n.ID); cur != nil; cur = c.doc.Parent(cur.ID) {
		if hex, ok := cur.FillHex(); ok {
			return hex, true
		}
	}
	return "", false
}

// checkUnusedComponents flags library components that no instance instantiates.
func (c *lintCtx) checkUnusedComponents() {
	used := make(map[string]bool)
	for _, n := range c.doc.All() {
		if n.ComponentID != "" {
			used[n.ComponentID] = true
		}
	}
	for _, comp := range c.doc.Components() {
		if comp.Type == model.TypeComponentSet || used[comp.ID] {
			continue
		}
		c.out = append(c.out, Issue{
			Rule:     "unused-component",
			Severity: c.severity(Warn),
			NodeID:   comp.ID,
			Name:     comp.Name,
			Type:     string(comp.Type),
			Path:     "",
			Detail:   "component has no instances",
		})
	}
}

func (c *lintCtx) severity(base Severity) Severity {
	if c.preset == "strict" && base == Warn {
		return Error
	}
	return base
}
