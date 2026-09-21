package mcp

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// NodeRef is the wire shape of a node: identity, geometry, typography and any
// rich payload (runs, props) carried by the model.
type NodeRef struct {
	ID       string  `json:"id" jsonschema:"node id, e.g. 123:456"`
	Type     string  `json:"type" jsonschema:"node type, e.g. FRAME, TEXT, SYMBOL"`
	Name     string  `json:"name" jsonschema:"node name"`
	ParentID string  `json:"parent_id,omitempty" jsonschema:"id of the parent node"`
	Text     string  `json:"text,omitempty" jsonschema:"text content for TEXT nodes"`
	Page     string  `json:"page,omitempty" jsonschema:"page name"`
	PageName string  `json:"page_name,omitempty" jsonschema:"page name for disambiguation"`
	PagePath string  `json:"page_path,omitempty" jsonschema:"path from page to node"`
	Score    float64 `json:"score,omitempty" jsonschema:"similarity score for semantic results"`

	X       float64 `json:"x" jsonschema:"left offset relative to the parent node"`
	Y       float64 `json:"y" jsonschema:"top offset relative to the parent node"`
	AbsX    float64 `json:"abs_x,omitempty" jsonschema:"absolute left offset from the page origin"`
	AbsY    float64 `json:"abs_y,omitempty" jsonschema:"absolute top offset from the page origin"`
	Width   float64 `json:"width" jsonschema:"width"`
	Height  float64 `json:"height" jsonschema:"height"`
	Visible bool    `json:"visible" jsonschema:"visible"`
	Opacity float64 `json:"opacity" jsonschema:"opacity"`

	FreePositioned bool `json:"free_positioned,omitempty" jsonschema:"true when the parent has no auto-layout, so x/y place the node absolutely"`

	FontFamily    string  `json:"font_family,omitempty"`
	FontStyle     string  `json:"font_style,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	TextAlign     string  `json:"text_align,omitempty" jsonschema:"horizontal alignment: LEFT, CENTER, RIGHT, JUSTIFIED"`
	TextAlignV    string  `json:"text_align_v,omitempty" jsonschema:"vertical alignment: TOP, CENTER, BOTTOM"`
	LineHeight    float64 `json:"line_height,omitempty"`
	LineHeightPx  float64 `json:"line_height_px,omitempty" jsonschema:"resolved line height in pixels"`
	LetterSpacing float64 `json:"letter_spacing,omitempty"`
	Decoration    string  `json:"decoration,omitempty" jsonschema:"UNDERLINE or STRIKETHROUGH"`
	Rotation      float64 `json:"rotation,omitempty" jsonschema:"rotation in degrees"`
	ClipContent   bool    `json:"clip_content,omitempty" jsonschema:"children clipped to bounds"`

	Path   string               `json:"path,omitempty" jsonschema:"SVG path geometry for vector shapes"`
	Runs   []model.TextRun      `json:"runs,omitempty" jsonschema:"styled text runs spanning the text"`
	Props  []model.PropOverride `json:"props,omitempty" jsonschema:"component override bindings on instances"`
	Layout string               `json:"layout,omitempty" jsonschema:"resolved layout style: direction, gap, padding, radius"`

	LayoutData     map[string]any    `json:"layout_data,omitempty" jsonschema:"structured auto-layout: layoutMode, per-axis sizing, alignment, gap, padding, radius, stroke"`
	Paints         *model.Paints     `json:"paints,omitempty" jsonschema:"ordered fill, stroke and effect stack"`
	BoundVariables map[string]string `json:"bound_variables,omitempty" jsonschema:"design-variable bindings by field name"`
	ViewBox        string            `json:"view_box,omitempty" jsonschema:"SVG viewBox for vector paths, e.g. 0 0 24 24"`
}

func toRef(n *model.Node) NodeRef {
	ref := NodeRef{
		ID: n.ID, Type: string(n.Type), Name: n.Name, Text: n.Text, Page: n.PageID,
		ParentID: n.ParentID,
		X:        n.X, Y: n.Y, Width: n.Width, Height: n.Height, Visible: n.Visible, Opacity: n.Opacity,
		FontFamily: n.FontFamily, FontStyle: n.FontStyle, FontSize: n.FontSize,
		TextAlign: n.TextAlign, TextAlignV: n.TextAlignV, LineHeight: n.LineHeight,
		LineHeightPx: n.LineHeightPx(), LetterSpacing: n.LetterSpacing, Decoration: n.Decoration,
		Rotation: n.Rotation, ClipContent: n.ClipContent,
		Path: n.Path,
	}
	if len(n.Runs) > 0 {
		ref.Runs = n.Runs
	}
	if len(n.Props) > 0 {
		ref.Props = n.Props
	}
	ref.Paints = n.PaintData
	ref.BoundVariables = n.BoundVars
	if n.Path != "" && n.Width > 0 && n.Height > 0 {
		ref.ViewBox = fmt.Sprintf("0 0 %g %g", n.Width, n.Height)
	}
	return ref
}

func toRefs(nodes []*model.Node) []NodeRef {
	out := make([]NodeRef, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toRef(n))
	}
	return out
}

// layoutOf returns the resolved LAYOUT style summary applied to a node, for
// building flex components without re-reading raw auto-layout payloads.
func layoutOf(styles []model.Style) string {
	for _, st := range styles {
		if strings.EqualFold(st.Kind, "LAYOUT") && st.Summary != "" {
			return st.Summary
		}
	}
	return ""
}

// layoutDataOf returns the structured LAYOUT style value applied to a node
// (layoutMode, per-axis sizing, alignment, gap, padding, radius, stroke).
func layoutDataOf(styles []model.Style) map[string]any {
	for _, st := range styles {
		if !strings.EqualFold(st.Kind, "LAYOUT") {
			continue
		}
		if m, ok := st.Value.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// filterInput is shared by every tool that lists a collection with an optional
// name substring filter.
type filterInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Name string `json:"name,omitempty" jsonschema:"optional name substring filter"`
}
