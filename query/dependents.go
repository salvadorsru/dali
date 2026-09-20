package query

import (
	"sort"

	"github.com/salvadorsru/dali/model"
)

// Dependent is a top-level container that places a component, with the number
// of placements aggregated over it.
type Dependent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Places int    `json:"places"`
}

// Dependents returns the containers that place a component, for impact
// analysis. A container is the first node in the chain above each INSTANCE
// whose parent is a page, so a screen counts once no matter how deep the
// instance sits. Results are ordered by placement count descending.
func (s *Service) Dependents(componentID string) ([]Dependent, error) {
	places, err := s.Usages(componentID)
	if err != nil {
		return nil, err
	}
	count := make(map[string]*Dependent)
	var order []string
	for _, p := range places {
		container, err := s.containerOf(p)
		if err != nil {
			return nil, err
		}
		if container == nil {
			continue
		}
		d, ok := count[container.ID]
		if !ok {
			d = &Dependent{ID: container.ID, Name: container.Name, Type: string(container.Type)}
			count[container.ID] = d
			order = append(order, container.ID)
		}
		d.Places++
	}
	out := make([]Dependent, 0, len(order))
	for _, id := range order {
		out = append(out, *count[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Places > out[j].Places })
	return out, nil
}

// containerOf walks up from a node to the first ancestor whose parent is a
// page (the top-level container). It returns nil when the node sits directly
// under the document or no container exists.
func (s *Service) containerOf(n *model.Node) (*model.Node, error) {
	for cur := n; ; {
		p, err := s.Parent(cur.ID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, nil
		}
		if p.Type == model.TypeCanvas {
			return cur, nil
		}
		cur = p
	}
}
