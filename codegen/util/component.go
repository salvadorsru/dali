package util

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
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
		Props:    []ir.ComponentProp{},
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

	// If it's a component set, get variants
	if comp.Type == model.TypeComponentSet {
		for _, child := range children {
			if child.Type == model.TypeSymbol {
				variant := BuildVariantSpec(svc, child)
				spec.Variants = append(spec.Variants, variant)
			}
		}
	}

	// Get prop definitions from instance usages
	spec.Props = buildComponentProps(svc, componentID)
	spec.DefaultProps = buildDefaultProps(spec.Props)

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
		spec.Vector = ir.VectorSpec{Path: node.Path}
	}

	// Get children recursively
	children, err := svc.Children(node.ID)
	if err == nil {
		for _, child := range children {
			spec.Children = append(spec.Children, BuildNodeSpec(svc, child))
		}
	}

	// If instance, get its prop overrides
	if node.Type == model.TypeInstance && len(node.Props) > 0 {
		spec.Props = buildNodeProps(node.Props)
	}

	return spec
}

// BuildVariantSpec builds a ir.ComponentVariant from a variant node.
func BuildVariantSpec(svc *query.Service, variant *model.Node) ir.ComponentVariant {
	v := ir.ComponentVariant{
		ID:       variant.ID,
		Name:     variant.Name,
		Props:    map[string]any{},
		Children: []ir.NodeSpec{},
		Layout:   TranslateLayout(variant),
		Styles:   TranslateStyles(variant),
	}

	children, err := svc.Children(variant.ID)
	if err == nil {
		for _, child := range children {
			v.Children = append(v.Children, BuildNodeSpec(svc, child))
		}
	}

	return v
}

// buildComponentProps extracts prop definitions from all instances of a component.
func buildComponentProps(svc *query.Service, componentID string) []ir.ComponentProp {
	usages, err := svc.Usages(componentID)
	if err != nil {
		return nil
	}

	propMap := make(map[string]*ir.ComponentProp)
	for _, inst := range usages {
		for _, p := range inst.Props {
			if existing, ok := propMap[p.ID]; ok {
				existing.Values = appendUnique(existing.Values, p.Value)
			} else {
				propMap[p.ID] = &ir.ComponentProp{
					ID:      p.ID,
					Name:    p.ID, // Will be improved with proper name lookup
					Type:    inferPropType(p.Value),
					Default: p.Value,
					Values:  []any{p.Value},
				}
			}
		}
	}

	var props []ir.ComponentProp
	for _, p := range propMap {
		props = append(props, *p)
	}
	return props
}

// buildNodeProps converts instance prop overrides to ir.ComponentProp slice.
func buildNodeProps(props []model.PropOverride) []ir.ComponentProp {
	var out []ir.ComponentProp
	for _, p := range props {
		out = append(out, ir.ComponentProp{
			ID:      p.ID,
			Name:    p.ID,
			Type:    inferPropType(p.Value),
			Default: p.Value,
			Values:  []any{p.Value},
		})
	}
	return out
}

// buildDefaultProps creates a map of default prop values.
func buildDefaultProps(props []ir.ComponentProp) map[string]any {
	defaults := make(map[string]any)
	for _, p := range props {
		defaults[p.Name] = p.Default
	}
	return defaults
}

// inferPropType infers a TypeScript-like type from a value.
func inferPropType(value any) string {
	if value == nil {
		return "any"
	}
	switch value.(type) {
	case string:
		return "string"
	case float64, float32, int, int64:
		return "number"
	case bool:
		return "boolean"
	default:
		return "any"
	}
}

func appendUnique(slice []any, value any) []any {
	for _, v := range slice {
		if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value) {
			return slice
		}
	}
	return append(slice, value)
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

// SanitizePropName converts a prop ID to a valid code identifier.
func SanitizePropName(name string) string {
	var b strings.Builder
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if i > 0 {
			b.WriteRune('_')
		}
	}
	result := b.String()
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "prop" + result
	}
	if result == "" {
		result = "prop"
	}
	// camelCase
	parts := strings.Split(result, "_")
	for i, part := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(part)
		} else if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}
