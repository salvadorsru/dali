package model

import "sort"

type Document struct {
	ID    string
	Name  string
	Pages []*Node

	nodes map[string]*Node
	order []*Node
}

func Build(changes []any) *Document {
	d := &Document{nodes: make(map[string]*Node, len(changes))}
	for _, c := range changes {
		m := asMap(c)
		if m == nil {
			continue
		}
		if phase := asString(m["phase"]); phase != "" && phase != "CREATED" {
			continue
		}
		id := guid(m)
		if id == "" {
			continue
		}
		d.nodes[id] = buildNode(m, id)
		d.order = append(d.order, d.nodes[id])
	}

	for _, n := range d.order {
		if p, ok := d.nodes[n.ParentID]; ok {
			n.parent = p
			p.Children = append(p.Children, n)
		}
	}
	for _, n := range d.order {
		sortChildren(n.Children)
	}
	for _, n := range d.order {
		switch n.Type {
		case TypeCanvas:
			d.Pages = append(d.Pages, n)
		case TypeDocument:
			d.ID, d.Name = n.ID, n.Name
		}
	}
	sortChildren(d.Pages)
	for _, n := range d.order {
		n.PageID = pageOf(n)
	}
	return d
}

func buildNode(m map[string]any, id string) *Node {
	n := &Node{
		ID:       id,
		Type:     NodeType(asString(m["type"])),
		Name:     asString(m["name"]),
		ParentID: parentGUID(m),
		Visible:  true,
		Opacity:  1,
	}
	pi := asMap(m["parentIndex"])
	n.Position = asString(pi["position"])
	if v, ok := m["visible"]; ok {
		n.Visible = asBool(v)
	}
	if v, ok := m["opacity"]; ok {
		n.Opacity = asFloat(v)
	}
	if size := asMap(m["size"]); size != nil {
		n.Width = asFloat(size["x"])
		n.Height = asFloat(size["y"])
	}
	if tr := asMap(m["transform"]); tr != nil {
		n.X = asFloat(tr["m02"])
		n.Y = asFloat(tr["m12"])
	}
	if td := asMap(m["textData"]); td != nil {
		n.Text = asString(td["characters"])
	}
	if fn := asMap(m["fontName"]); fn != nil {
		n.FontFamily = asString(fn["family"])
		n.FontStyle = asString(fn["style"])
	}
	n.FontSize = asFloat(m["fontSize"])
	if sd := asMap(m["symbolData"]); sd != nil {
		n.ComponentID = guidValue(sd["symbolID"])
	}
	n.raw = m
	n.Images = nodeImages(m, id)
	return n
}

func sortChildren(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].Position < nodes[j].Position
	})
}

func pageOf(n *Node) string {
	for cur := n; cur != nil; cur = cur.parent {
		if cur.Type == TypeCanvas {
			return cur.ID
		}
	}
	return ""
}

func (d *Document) Node(id string) (*Node, bool) {
	n, ok := d.nodes[id]
	return n, ok
}

func (d *Document) All() []*Node { return d.order }

func (d *Document) Parent(id string) *Node {
	n, ok := d.nodes[id]
	if !ok {
		return nil
	}
	return n.parent
}

func (d *Document) Children(id string) []*Node {
	n, ok := d.nodes[id]
	if !ok {
		return nil
	}
	return n.Children
}

func (d *Document) Ancestors(id string) []*Node {
	n, ok := d.nodes[id]
	if !ok {
		return nil
	}
	var out []*Node
	for cur := n.parent; cur != nil; cur = cur.parent {
		out = append(out, cur)
	}
	return out
}

func (d *Document) Descendants(id string) []*Node {
	n, ok := d.nodes[id]
	if !ok {
		return nil
	}
	var out []*Node
	var walk func(*Node)
	walk = func(cur *Node) {
		for _, c := range cur.Children {
			out = append(out, c)
			walk(c)
		}
	}
	walk(n)
	return out
}

func (d *Document) Path(id string) []*Node {
	n, ok := d.nodes[id]
	if !ok {
		return nil
	}
	ancestors := d.Ancestors(id)
	path := make([]*Node, 0, len(ancestors)+1)
	for i := len(ancestors) - 1; i >= 0; i-- {
		path = append(path, ancestors[i])
	}
	return append(path, n)
}

func (d *Document) Components() []*Node {
	return d.byType(func(n *Node) bool { return n.IsComponent() })
}

func (d *Document) Instances() []*Node {
	return d.byType(func(n *Node) bool { return n.Type == TypeInstance })
}

func (d *Document) TextNodes() []*Node {
	return d.byType(func(n *Node) bool { return n.Type == TypeText })
}

func (d *Document) byType(match func(*Node) bool) []*Node {
	var out []*Node
	for _, n := range d.order {
		if match(n) {
			out = append(out, n)
		}
	}
	return out
}

func (d *Document) Counts() map[NodeType]int {
	counts := make(map[NodeType]int)
	for _, n := range d.order {
		counts[n.Type]++
	}
	return counts
}
