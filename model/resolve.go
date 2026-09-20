package model

import (
	"encoding/json"
	"strings"
)

// maxExpandDepth caps how deep nested instance expansion recurses, so
// self-referential components cannot produce an unbounded clone chain.
const maxExpandDepth = 12

// expandInstances materializes the content of every instance. The .fig stores
// an INSTANCE as a reference (symbolData.symbolID) plus overrides, while the
// visible children live in the SYMBOL tree. Without this pass the instance
// has no children and referenced content is lost from index, search and text.
func (d *Document) expandInstances() {
	for i := 0; i < len(d.order); i++ {
		n := d.order[i]
		if n.Type != TypeInstance || len(n.Children) > 0 || n.symbol == nil {
			continue
		}
		d.resolveInstance(n, 0)
	}
}

// resolveInstance clones the component's subtree under inst, applying
// per-axis scaling, the instance's symbol overrides and its component-prop
// substitutions (text values and slot content).
func (d *Document) resolveInstance(inst *Node, depth int) {
	if depth >= maxExpandDepth || inst.symbol == nil {
		return
	}
	comp := inst.symbol
	if comp.Width <= 0 || comp.Height <= 0 {
		return
	}
	sx := inst.Width / comp.Width
	sy := inst.Height / comp.Height
	o := d.instanceOverrides(inst)
	props := instancePropValues(inst)
	for _, child := range comp.Children {
		d.cloneSubtree(child, inst, sx, sy, depth, o, props)
	}
	sortChildren(inst.Children)
}

// cloneSubtree clones src (a component subtree node) into inst, scaled by
// sx/sy, recursing into its children. Nested instances are cloned as leaves
// and expanded with their own symbol, scale and overrides.
func (d *Document) cloneSubtree(src, inst *Node, sx, sy float64, depth int, o map[string]map[string]any, props map[string]string) *Node {
	text, slot := d.bindProp(src, props)
	if slot != "" {
		return d.mountSlot(inst, src, slot, sx, sy, depth)
	}
	cp := deepCopyRaw(src.raw)
	if text != "" {
		td := asMap(cp["textData"])
		if td == nil {
			td = map[string]any{}
			cp["textData"] = td
		}
		td["characters"] = text
	}
	d.applyNodeOverrides(cp, src, o)
	scaleRaw(cp, sx, sy)
	clone := buildNode(cp, cloneID(inst.ID, src.ID), d.blobs)
	d.attach(inst, clone)
	if clone.Type == TypeInstance {
		if sym, ok := d.nodes[clone.ComponentID]; ok {
			clone.symbol = sym
		}
		d.resolveInstance(clone, depth+1)
		return clone
	}
	for _, c := range src.Children {
		d.cloneSubtree(c, clone, sx, sy, depth, o, props)
	}
	sortChildren(clone.Children)
	return clone
}

// bindProp resolves the component-prop binding of a subtree node against the
// instance's assigned values. It returns the replacement text for a TEXT
// prop, or the slot reference ("slot:<id>") for a SLOT_CONTENT_ID prop.
func (d *Document) bindProp(src *Node, props map[string]string) (text, slot string) {
	for _, defID := range boundPropIDs(src.raw) {
		v, ok := props[defID]
		if !ok || strings.HasPrefix(v, "var:") {
			continue
		}
		if strings.HasPrefix(v, "slot:") {
			return "", v
		}
		if text == "" {
			text = v
		}
		break
	}
	return text, ""
}

// mountSlot replaces a slot-bound node with the referenced content subtree,
// placed at the placeholder's (scaled) position within the instance. The
// mounted content keeps its own internal coordinates.
func (d *Document) mountSlot(inst, src *Node, slot string, sx, sy float64, depth int) *Node {
	content := d.nodes[strings.TrimPrefix(slot, "slot:")]
	if content == nil {
		return nil
	}
	cp := deepCopyRaw(content.raw)
	if tr := asMap(cp["transform"]); tr != nil {
		tr["m02"] = src.X * sx
		tr["m12"] = src.Y * sy
	}
	clone := buildNode(cp, cloneID(inst.ID, content.ID), d.blobs)
	d.attach(inst, clone)
	if clone.Type == TypeInstance {
		if sym, ok := d.nodes[clone.ComponentID]; ok {
			clone.symbol = sym
		}
		d.resolveInstance(clone, depth+1)
		return clone
	}
	for _, c := range content.Children {
		d.cloneSubtree(c, clone, 1, 1, depth, nil, nil)
	}
	sortChildren(clone.Children)
	return clone
}

// attach registers a cloned node under inst and appends it to the document.
func (d *Document) attach(inst, clone *Node) {
	clone.ParentID = inst.ID
	clone.parent = inst
	clone.PageID = inst.PageID
	d.nodes[clone.ID] = clone
	d.order = append(d.order, clone)
	inst.Children = append(inst.Children, clone)
}

func cloneID(instID, srcID string) string {
	return "inst:" + instID + "." + srcID
}

// boundPropIDs returns the component-prop def IDs a subtree node participates
// in via its componentPropRefs entries.
func boundPropIDs(m map[string]any) []string {
	var out []string
	for _, r := range asSlice(m["componentPropRefs"]) {
		rm := asMap(r)
		if rm == nil {
			continue
		}
		if id := propID(rm["defID"]); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// instancePropValues maps component-prop def IDs to their rendered string
// values (text content or "slot:<id>" references).
func instancePropValues(n *Node) map[string]string {
	var m map[string]string
	for _, p := range n.Props {
		s, ok := p.Value.(string)
		if !ok || strings.HasPrefix(s, "var:") {
			continue
		}
		if m == nil {
			m = make(map[string]string)
		}
		m[p.ID] = s
	}
	return m
}

// instanceOverrides collects symbolOverrides from an instance, indexed by
// every node id / library id appearing in its guidPath.
func (d *Document) instanceOverrides(inst *Node) map[string]map[string]any {
	sd := asMap(inst.raw["symbolData"])
	if sd == nil {
		return nil
	}
	var out map[string]map[string]any
	for _, e := range asSlice(sd["symbolOverrides"]) {
		em := asMap(e)
		if em == nil {
			continue
		}
		gp := asMap(em["guidPath"])
		if gp == nil {
			continue
		}
		for _, g := range asSlice(gp["guids"]) {
			id := guidValue(g)
			if id == "" || id == "4294967295:4294967295" {
				continue
			}
			if out == nil {
				out = make(map[string]map[string]any)
			}
			out[id] = em
		}
	}
	return out
}

// applyNodeOverrides copies the style-relevant fields of a matching symbol
// override onto the deep-copied raw of a cloned node. Geometry fields (size,
// blobs) are intentionally left alone: the clone is scaled separately.
func (d *Document) applyNodeOverrides(cp map[string]any, src *Node, o map[string]map[string]any) {
	if len(o) == 0 {
		return
	}
	var override map[string]any
	for key, e := range o {
		if key == src.ID || libNodeID(src.raw) == key {
			override = e
		}
	}
	if override == nil {
		return
	}
	for _, f := range []string{
		"fillPaints", "strokePaints", "effects",
		"fontName", "fontSize", "letterSpacing", "lineHeight",
		"styleIdForText", "textTracking", "strokeWeight",
	} {
		if v, ok := override[f]; ok {
			cp[f] = v
		}
	}
	if td := asMap(override["textData"]); td != nil {
		if chars := asString(td["characters"]); chars != "" {
			cur := asMap(cp["textData"])
			if cur == nil {
				cur = map[string]any{}
				cp["textData"] = cur
			}
			// keep the node's characterStyleIDs so runs still resolve; only
			// symbol override textData carries no style ids.
			cur["characters"] = chars
		}
	}
}

// scaleRaw multiplies a node's transform and size by the per-axis scale.
func scaleRaw(m map[string]any, sx, sy float64) {
	if sx == 1 && sy == 1 {
		return
	}
	if tr := asMap(m["transform"]); tr != nil {
		tr["m00"] = asFloat(tr["m00"]) * sx
		tr["m01"] = asFloat(tr["m01"]) * sx
		tr["m02"] = asFloat(tr["m02"]) * sx
		tr["m10"] = asFloat(tr["m10"]) * sy
		tr["m11"] = asFloat(tr["m11"]) * sy
		tr["m12"] = asFloat(tr["m12"]) * sy
	}
	if sz := asMap(m["size"]); sz != nil {
		sz["x"] = asFloat(sz["x"]) * sx
		sz["y"] = asFloat(sz["y"]) * sy
	}
}

// libNodeID resolves the library-side identifier of a node (overrideKey or
// publishID), which symbol override guidPaths refer to.
func libNodeID(m map[string]any) string {
	for _, k := range []string{"overrideKey", "publishID"} {
		if g := asMap(m[k]); g != nil && g["sessionID"] != nil {
			return formatGUID(g)
		}
	}
	return ""
}

func deepCopyRaw(m map[string]any) map[string]any {
	b, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return m
	}
	return out
}
