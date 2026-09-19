package query

import "github.com/salvadorsru/dali/model"

type Context struct {
	Node      *model.Node      `json:"node"`
	Path      []*model.Node    `json:"path"`
	Parent    *model.Node      `json:"parent,omitempty"`
	Children  []*model.Node    `json:"children"`
	Component *model.Node      `json:"component,omitempty"`
	Styles    []model.Style    `json:"styles"`
	Assets    []model.ImageRef `json:"assets"`
}

func (s *Service) Context(id string) (*Context, error) {
	node, err := s.Node(id)
	if err != nil || node == nil {
		return nil, err
	}
	ctx := &Context{Node: node}
	if ctx.Path, err = s.Path(id); err != nil {
		return nil, err
	}
	if ctx.Parent, err = s.Parent(id); err != nil {
		return nil, err
	}
	if ctx.Children, err = s.Children(id); err != nil {
		return nil, err
	}
	if ctx.Styles, err = s.StylesOf(id); err != nil {
		return nil, err
	}
	if ctx.Assets, err = s.AssetsOf(id); err != nil {
		return nil, err
	}
	componentID := node.ComponentID
	if componentID == "" && node.IsComponent() {
		componentID = node.ID
	}
	if componentID != "" {
		if ctx.Component, err = s.Node(componentID); err != nil {
			return nil, err
		}
	}
	return ctx, nil
}
