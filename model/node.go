package model

type NodeType string

const (
	TypeDocument     NodeType = "DOCUMENT"
	TypeCanvas       NodeType = "CANVAS"
	TypeFrame        NodeType = "FRAME"
	TypeGroup        NodeType = "GROUP"
	TypeText         NodeType = "TEXT"
	TypeVector       NodeType = "VECTOR"
	TypeInstance     NodeType = "INSTANCE"
	TypeSymbol       NodeType = "SYMBOL"
	TypeComponentSet NodeType = "COMPONENT_SET"
	TypeVariable     NodeType = "VARIABLE"
	TypeVariableSet  NodeType = "VARIABLE_SET"
)

func (t NodeType) IsComponent() bool {
	return t == TypeSymbol || t == "COMPONENT" || t == TypeComponentSet
}

type Node struct {
	ID            string
	Type          NodeType
	Name          string
	ParentID      string
	PageID        string
	Position      string
	Visible       bool
	Opacity       float64
	X             float64
	Y             float64
	Width         float64
	Height        float64
	Text          string
	FontFamily    string
	FontStyle     string
	FontSize      float64
	TextAlign     string
	TextAlignV    string
	LineHeight    float64
	LetterSpacing float64
	Decoration    string // UNDERLINE | STRIKETHROUGH
	Rotation      float64
	ClipContent   bool
	Path          string // SVG path geometry from vector blobs
	Runs          []TextRun
	Props         []PropOverride

	// PaintData and BoundVars are populated when a node is read back from the
	// index; they are not set during parsing (raw paint payloads live there).
	PaintData *Paints
	BoundVars map[string]string

	ComponentID string

	Children []*Node
	Images   []ImageRef
	symbol   *Node
	parent   *Node
	raw      map[string]any
}

// TextRun is a contiguous span of a text node that shares one style: base
// style for styleID 0, a styleOverrideTable entry otherwise.
type TextRun struct {
	Start         int     `json:"start"`
	Len           int     `json:"len"`
	Text          string  `json:"text"`
	FontFamily    string  `json:"font_family,omitempty"`
	FontStyle     string  `json:"font_style,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	Color         string  `json:"color,omitempty"`
	Decoration    string  `json:"decoration,omitempty"`
	LineHeight    float64 `json:"line_height,omitempty"`
	LetterSpacing float64 `json:"letter_spacing,omitempty"`
}

// PropOverride is a component prop assignment on an instance whose rendered
// value differs from the component default.
type PropOverride struct {
	ID    string `json:"id"`
	Value any    `json:"value,omitempty"`
}

func (n *Node) IsComponent() bool { return n.Type.IsComponent() }

func (n *Node) IsText() bool { return n.Type == TypeText }

// Symbol resolves the component an instance instantiates.
func (n *Node) Symbol() *Node { return n.symbol }
