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
)

func (t NodeType) IsComponent() bool {
	return t == TypeSymbol || t == "COMPONENT" || t == TypeComponentSet
}

type Node struct {
	ID         string
	Type       NodeType
	Name       string
	ParentID   string
	PageID     string
	Position   string
	Visible    bool
	Opacity    float64
	X          float64
	Y          float64
	Width      float64
	Height     float64
	Text       string
	FontFamily string
	FontStyle  string
	FontSize   float64

	ComponentID string

	Children []*Node
	Images   []ImageRef
	parent   *Node
	raw      map[string]any
}

func (n *Node) IsComponent() bool { return n.Type.IsComponent() }

func (n *Node) IsText() bool { return n.Type == TypeText }
