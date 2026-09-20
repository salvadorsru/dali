package mcp

import (
	"github.com/salvadorsru/dali/model"
)

// NodeRef is the wire shape of a node: identity, geometry, typography and any
// rich payload (runs, props) carried by the model.
type NodeRef struct {
	ID       string  `json:"id" jsonschema:"node id, e.g. 123:456"`
	Type     string  `json:"type" jsonschema:"node type, e.g. FRAME, TEXT, SYMBOL"`
	Name     string  `json:"name" jsonschema:"node name"`
	Text     string  `json:"text,omitempty" jsonschema:"text content for TEXT nodes"`
	Page     string  `json:"page,omitempty" jsonschema:"page name"`
	PagePath string  `json:"page_path,omitempty" jsonschema:"path from page to node"`
	Score    float64 `json:"score,omitempty" jsonschema:"similarity score for semantic results"`

	X       float64 `json:"x" jsonschema:"left offset"`
	Y       float64 `json:"y" jsonschema:"top offset"`
	Width   float64 `json:"width" jsonschema:"width"`
	Height  float64 `json:"height" jsonschema:"height"`
	Visible bool    `json:"visible" jsonschema:"visible"`
	Opacity float64 `json:"opacity" jsonschema:"opacity"`

	FontFamily    string  `json:"font_family,omitempty"`
	FontStyle     string  `json:"font_style,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	TextAlign     string  `json:"text_align,omitempty" jsonschema:"horizontal alignment: LEFT, CENTER, RIGHT, JUSTIFIED"`
	TextAlignV    string  `json:"text_align_v,omitempty" jsonschema:"vertical alignment: TOP, CENTER, BOTTOM"`
	LineHeight    float64 `json:"line_height,omitempty"`
	LetterSpacing float64 `json:"letter_spacing,omitempty"`
	Decoration    string  `json:"decoration,omitempty" jsonschema:"UNDERLINE or STRIKETHROUGH"`
	Rotation      float64 `json:"rotation,omitempty" jsonschema:"rotation in degrees"`
	ClipContent   bool    `json:"clip_content,omitempty" jsonschema:"children clipped to bounds"`

	Path  string               `json:"path,omitempty" jsonschema:"SVG path geometry for vector shapes"`
	Runs  []model.TextRun      `json:"runs,omitempty" jsonschema:"styled text runs spanning the text"`
	Props []model.PropOverride `json:"props,omitempty" jsonschema:"component override bindings on instances"`
}

func toRef(n *model.Node) NodeRef {
	ref := NodeRef{
		ID: n.ID, Type: string(n.Type), Name: n.Name, Text: n.Text, Page: n.PageID,
		X: n.X, Y: n.Y, Width: n.Width, Height: n.Height, Visible: n.Visible, Opacity: n.Opacity,
		FontFamily: n.FontFamily, FontStyle: n.FontStyle, FontSize: n.FontSize,
		TextAlign: n.TextAlign, TextAlignV: n.TextAlignV, LineHeight: n.LineHeight,
		LetterSpacing: n.LetterSpacing, Decoration: n.Decoration,
		Rotation: n.Rotation, ClipContent: n.ClipContent,
		Path: n.Path,
	}
	if len(n.Runs) > 0 {
		ref.Runs = n.Runs
	}
	if len(n.Props) > 0 {
		ref.Props = n.Props
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

// filterInput is shared by every tool that lists a collection with an optional
// name substring filter.
type filterInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Name string `json:"name,omitempty" jsonschema:"optional name substring filter"`
}
