package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
)

type tokensInput struct {
	File     string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Kind     string `json:"kind,omitempty" jsonschema:"which token families to return: color, type, layout or all (default all)"`
	MinUsage int    `json:"min_usage,omitempty" jsonschema:"drop colours used fewer than this many times; default 0"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max entries per family; defaults: colours 256, classes 200, layouts 200"`
}

type tokensOutput struct {
	Colors      []query.SolidColor  `json:"colors,omitempty"`
	ColorVars   []model.Variable    `json:"color_variables,omitempty"`
	Fonts       []model.Font        `json:"fonts,omitempty"`
	TextClasses []query.TextClass   `json:"text_classes,omitempty"`
	Layouts     []query.TokenLayout `json:"layouts,omitempty"`
}

// getTokens aggregates design tokens that would otherwise need a full
// get_styles sweep: solid colour palette, colour variables, fonts, text
// classes and flex layouts, ranked by usage.
func (h *handlers) getTokens(ctx context.Context, req *mcp.CallToolRequest, in tokensInput) (*mcp.CallToolResult, tokensOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, tokensOutput{}, err
	}
	limit := in.Limit
	switch {
	case limit <= 0:
		limit = 200
	case limit > 500:
		limit = 500
	}
	kind := strings.ToLower(in.Kind)
	want := func(k string) bool { return kind == "" || kind == k }
	out := tokensOutput{}
	if want("color") {
		colorLimit := 256
		if in.Limit > 0 {
			colorLimit = limit
		}
		if out.Colors, err = svc.Palette(in.MinUsage, colorLimit); err != nil {
			return nil, tokensOutput{}, err
		}
		vars, err := svc.Variables()
		if err != nil {
			return nil, tokensOutput{}, err
		}
		for _, v := range vars {
			if v.Type == "COLOR" {
				out.ColorVars = append(out.ColorVars, v)
			}
		}
		if colorLimit > 0 && len(out.ColorVars) > colorLimit {
			out.ColorVars = out.ColorVars[:colorLimit]
		}
	}
	if want("type") {
		if out.Fonts, err = svc.Fonts(); err != nil {
			return nil, tokensOutput{}, err
		}
		if out.TextClasses, err = svc.TextClasses(limit); err != nil {
			return nil, tokensOutput{}, err
		}
	}
	if want("layout") {
		if out.Layouts, err = svc.TokenLayouts(limit); err != nil {
			return nil, tokensOutput{}, err
		}
	}
	return nil, out, nil
}

type usagesInput struct {
	File  string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	ID    string `json:"id" jsonschema:"component id, e.g. 654:10967"`
	Limit int    `json:"limit,omitempty" jsonschema:"max placements to list, default 50"`
}

type usagesOutput struct {
	Component NodeRef   `json:"component"`
	Instances int       `json:"instances"`
	Uses      []NodeRef `json:"uses"`
}

// getUsages lists every place a component is dropped into a page.
func (h *handlers) getUsages(ctx context.Context, req *mcp.CallToolRequest, in usagesInput) (*mcp.CallToolResult, usagesOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, usagesOutput{}, err
	}
	n, err := svc.Node(in.ID)
	if err != nil {
		return nil, usagesOutput{}, err
	}
	if n == nil {
		return nil, usagesOutput{}, fmt.Errorf("node %q not found", in.ID)
	}
	places, err := svc.Usages(in.ID)
	if err != nil {
		return nil, usagesOutput{}, err
	}
	out := usagesOutput{Component: toRef(n), Instances: len(places), Uses: toRefs(places)}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(out.Uses) > limit {
		out.Uses = out.Uses[:limit]
	}
	if out.Uses == nil {
		out.Uses = []NodeRef{}
	}
	return nil, out, nil
}

type componentProp struct {
	ID    string `json:"id"`
	Value any    `json:"value"`
	Count int    `json:"count"`
}

type componentInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	ID   string `json:"id" jsonschema:"component id, e.g. 1:225"`
}

type componentOutput struct {
	Node      NodeRef         `json:"node"`
	Type      string          `json:"type"`
	Variants  []NodeRef       `json:"variants,omitempty"`
	Children  []NodeRef       `json:"children"`
	Props     []componentProp `json:"props,omitempty"`
	Instances int             `json:"instances"`
}

// getComponent is the codegen contract of a component: how it is dropped
// (variants, children skeleton), the property keys it accepts (observed from
// placements) and how many pages use it.
func (h *handlers) getComponent(ctx context.Context, req *mcp.CallToolRequest, in componentInput) (*mcp.CallToolResult, componentOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, componentOutput{}, err
	}
	n, err := svc.Node(in.ID)
	if err != nil {
		return nil, componentOutput{}, err
	}
	if n == nil {
		return nil, componentOutput{}, fmt.Errorf("node %q not found", in.ID)
	}
	children, err := svc.Children(in.ID)
	if err != nil {
		return nil, componentOutput{}, err
	}
	out := componentOutput{Node: toRef(n), Type: string(n.Type), Children: toRefs(children)}
	if out.Children == nil {
		out.Children = []NodeRef{}
	}
	if n.Type == model.TypeComponentSet {
		out.Variants = out.Children
	}
	places, err := svc.Usages(in.ID)
	if err != nil {
		return nil, componentOutput{}, err
	}
	out.Instances = len(places)
	out.Props = componentProps(places)
	return nil, out, nil
}

// componentProps folds the override bindings of the component's placements
// into a contract: each property id with the first observed value and how many
// placements set it.
func componentProps(places []*model.Node) []componentProp {
	seen := make(map[string]*componentProp)
	var order []string
	for _, p := range places {
		for _, pr := range p.Props {
			cp, ok := seen[pr.ID]
			if !ok {
				cp = &componentProp{ID: pr.ID, Value: pr.Value}
				seen[pr.ID] = cp
				order = append(order, pr.ID)
			}
			cp.Count++
		}
	}
	out := make([]componentProp, 0, len(seen))
	for _, id := range order {
		out = append(out, *seen[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}
