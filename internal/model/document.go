package model

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Document struct {
	ID        string
	Name      string
	Pages     []*Node
	Variables []Variable

	nodes map[string]*Node
	order []*Node
	blobs [][]byte
}

// Build assembles a Document from decoded node changes and the file's vector
// blobs. blobs may be nil; nodes then carry text runs and props but no vector
// geometry.
func Build(changes []any, blobs [][]byte) *Document {
	d := &Document{nodes: make(map[string]*Node, len(changes)), blobs: blobs}
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
		d.nodes[id] = buildNode(m, id, d.blobs)
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
	for _, n := range d.order {
		if n.Type != TypeInstance || n.ComponentID == "" {
			continue
		}
		if sym, ok := d.nodes[n.ComponentID]; ok {
			n.symbol = sym
		}
	}
	d.expandInstances()
	d.Variables = collectVariables(d)
	return d
}

func buildNode(m map[string]any, id string, blobs [][]byte) *Node {
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
		n.Decoration = textDecoration(td)
		n.Runs = textRuns(m, td, n.Text)
	}
	if fn := asMap(m["fontName"]); fn != nil {
		n.FontFamily = asString(fn["family"])
		n.FontStyle = asString(fn["style"])
	}
	n.FontSize = asFloat(m["fontSize"])
	n.TextAlign = asString(m["textAlignHorizontal"])
	n.TextAlignV = asString(m["textAlignVertical"])
	if lh := asMap(m["lineHeight"]); lh != nil {
		n.LineHeight = asFloat(lh["value"])
		n.LineHeightUnits = asString(lh["units"])
	}
	if ls := asMap(m["letterSpacing"]); ls != nil {
		n.LetterSpacing = asFloat(ls["value"])
	}
	n.Rotation = rotationDeg(m)
	if cc, ok := m["clipContent"].(bool); ok {
		n.ClipContent = cc
	}
	if sd := asMap(m["symbolData"]); sd != nil {
		n.ComponentID = guidValue(sd["symbolID"])
	}
	if len(blobs) > 0 {
		n.Path = nodeVectorPath(m, blobs)
	}
	n.Props = propOverrides(m["componentPropAssignments"])
	n.raw = m
	n.Images = nodeImages(m, id)
	return n
}

// textDecoration reports the decorative underline or strikethrough applied to
// any character segment of a text node, preferring strikethrough when mixed.
func textDecoration(td map[string]any) string {
	if td == nil {
		return ""
	}
	found := ""
	for _, s := range asSlice(td["styleOverrideTable"]) {
		d := asString(asMap(s)["textDecoration"])
		if d == "" {
			continue
		}
		if d == "STRIKETHROUGH" {
			return d
		}
		if found == "" {
			found = d
		}
	}
	return found
}

// textRuns splits a text node's characters into contiguous runs sharing one
// style. characterStyleIDs maps every character to a style ID (0 = base node
// style, otherwise a 1-indexed reference into styleOverrideTable); consecutive
// characters with the same ID form one run.
func textRuns(m map[string]any, td map[string]any, text string) []TextRun {
	ids := asSlice(td["characterStyleIDs"])
	if len(ids) == 0 {
		return nil
	}
	chars := []rune(text)
	overrides := asSlice(td["styleOverrideTable"])
	var out []TextRun
	for i := 0; i < len(ids); {
		styleID := asInt(ids[i])
		j := i
		for j < len(ids) && asInt(ids[j]) == styleID {
			j++
		}
		r := TextRun{Start: i, Len: j - i}
		if i < len(chars) {
			end := j
			if end > len(chars) {
				end = len(chars)
			}
			r.Text = string(chars[i:end])
		}
		applyRunStyle(&r, styleID, overrides, m)
		out = append(out, r)
		i = j
	}
	return out
}

// applyRunStyle resolves a run's style either from the override table entry
// matching its style ID or, for base runs, from the node's own text styling.
func applyRunStyle(r *TextRun, styleID int64, overrides []any, m map[string]any) {
	base := m
	if styleID != 0 {
		entry := overrideFor(overrides, int(styleID))
		if entry == nil {
			return
		}
		base = entry
	}
	if fn := asMap(base["fontName"]); fn != nil {
		r.FontFamily = asString(fn["family"])
		r.FontStyle = asString(fn["style"])
	}
	if size := asFloat(base["fontSize"]); size > 0 {
		r.FontSize = size
	}
	if lh := asMap(base["lineHeight"]); lh != nil {
		r.LineHeight = asFloat(lh["value"])
		if units := asString(lh["units"]); units != "" {
			r.LineHeightPx = resolveLineHeight(r.LineHeight, r.FontSize, units)
		}
	}
	if ls := asMap(base["letterSpacing"]); ls != nil {
		r.LetterSpacing = asFloat(ls["value"])
	}
	r.Decoration = asString(base["textDecoration"])
	if paints := asSlice(base["fillPaints"]); len(paints) > 0 {
		if c := colorOf(asMap(paints[0])); c != "" {
			r.Color = c
		}
	}
}

// overrideFor resolves a styleOverrideTable entry for a 1-indexed style ID.
func overrideFor(overrides []any, styleID int) map[string]any {
	for _, o := range overrides {
		if om := asMap(o); om != nil && asInt(om["styleID"]) == int64(styleID) {
			return om
		}
	}
	if styleID > 0 && styleID-1 < len(overrides) {
		return asMap(overrides[styleID-1])
	}
	return nil
}

// propOverrides extracts component property assignments from an instance:
// each defID with a non-default rendered value becomes a PropOverride. The
// file may carry them as a flat array (defID + value + varValue) or wrapped
// in a "entries" array.
func propOverrides(raw any) []PropOverride {
	entries := asSlice(raw)
	if entries == nil {
		if m := asMap(raw); m != nil {
			entries = asSlice(m["entries"])
		}
	}
	if entries == nil {
		return nil
	}
	var out []PropOverride
	for _, e := range entries {
		em := asMap(e)
		if em == nil {
			continue
		}
		v := propValue(em["value"])
		if v == nil {
			v = varValue(em["varValue"])
		}
		if v != nil {
			out = append(out, PropOverride{ID: propID(em["defID"]), Value: v})
		}
	}
	return out
}

// propID renders a defID guid, tolerating the zero-padded fixed-width string
// form used for large files.
func propID(v any) string {
	g := asMap(v)
	if g == nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", numValue(g["sessionID"]), numValue(g["localID"]))
}

func numValue(v any) int64 {
	if s, ok := v.(string); ok {
		s = strings.TrimLeft(s, "0")
		if s == "" {
			return 0
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0
		}
		return i
	}
	return asInt(v)
}

func propValue(v any) any {
	vm := asMap(v)
	if vm == nil {
		return nil
	}
	if tv := vm["textValue"]; tv != nil {
		if s := asString(tv); s != "" {
			return s
		}
		if m := asMap(tv); m != nil {
			return asString(m["characters"])
		}
	}
	if n, ok := vm["numberValue"]; ok && n != nil {
		return n
	}
	if b, ok := vm["booleanValue"].(bool); ok {
		return b
	}
	return nil
}

// varValue resolves a var-bound property override into a readable value.
func varValue(v any) any {
	va := asMap(v)
	if va == nil {
		return nil
	}
	if g := guidOf(va); g != "" {
		return "var:" + g
	}
	if vd := asMap(va["value"]); vd != nil {
		if sc := guidOf(vd["slotContentIdValue"]); sc != "" {
			return "slot:" + sc
		}
		if td := asMap(vd["textDataValue"]); td != nil {
			return asString(td["characters"])
		}
	}
	return va
}

// rotationDeg reports the node's rotation in degrees, derived from its 2D
// transform matrix, rounded to whole degrees.
func rotationDeg(m map[string]any) float64 {
	tr := asMap(m["transform"])
	if tr == nil {
		return 0
	}
	m00, m01 := asFloat(tr["m00"]), asFloat(tr["m01"])
	m10, m11 := asFloat(tr["m10"]), asFloat(tr["m11"])
	if m00 == 0 && m01 == 0 && m10 == 0 && m11 == 0 {
		return 0
	}
	deg := math.Atan2(m10, m00) * 180 / math.Pi
	if math.Abs(deg) < 0.005 {
		return 0
	}
	return math.Round(deg)
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
