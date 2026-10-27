package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/internal/codegen/ir"
	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/query"
)

// BuildComponentSpec constructs a ir.ComponentSpec from a component node and its
// usages (instances).
func BuildComponentSpec(svc *query.Service, componentID string) (ir.ComponentSpec, error) {
	comp, err := svc.Node(componentID)
	if err != nil {
		return ir.ComponentSpec{}, err
	}
	if comp == nil {
		return ir.ComponentSpec{}, fmt.Errorf("component %q not found", componentID)
	}

	spec := ir.ComponentSpec{
		ID:       comp.ID,
		Name:     comp.Name,
		Type:     string(comp.Type),
		Layout:   TranslateLayout(comp),
		Styles:   TranslateStyles(comp),
		Children: []ir.NodeSpec{},
	}

	// Get direct children of the component
	children, err := svc.Children(componentID)
	if err != nil {
		return ir.ComponentSpec{}, err
	}
	for _, child := range children {
		spec.Children = append(spec.Children, BuildNodeSpec(svc, child))
	}

	return spec, nil
}

// BuildNodeSpec recursively builds a ir.NodeSpec from a node.
func BuildNodeSpec(svc *query.Service, node *model.Node) ir.NodeSpec {
	spec := ir.NodeSpec{
		ID:          node.ID,
		Type:        string(node.Type),
		Name:        node.Name,
		ComponentID: node.ComponentID,
		IsInstance:  node.Type == model.TypeInstance,
		Layout:      TranslateLayout(node),
		Styles:      TranslateStyles(node),
		Children:    []ir.NodeSpec{},
	}

	if node.Type == model.TypeText {
		spec.Text = TranslateText(node)
	}
	if node.Type == model.TypeVector && node.Path != "" {
		spec.Vector = ir.VectorSpec{Path: node.Path, Width: node.Width, Height: node.Height}
	}

	// Get children recursively
	children, err := svc.Children(node.ID)
	if err == nil {
		for _, child := range children {
			spec.Children = append(spec.Children, BuildNodeSpec(svc, child))
		}
	}

	return spec
}

// SanitizeComponentName converts a Figma component name to a valid code identifier.
func SanitizeComponentName(name string) string {
	// Replace non-alphanumeric with underscore
	var b strings.Builder
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if i > 0 {
			b.WriteRune('_')
		}
	}
	// Ensure starts with letter
	result := b.String()
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "Component" + result
	}
	if result == "" {
		result = "Component"
	}
	// PascalCase
	parts := strings.Split(result, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}
