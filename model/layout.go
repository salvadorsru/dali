package model

import (
	"strconv"
	"strings"
)

// layoutFields are the node properties that describe an auto layout frame.
var layoutFields = []string{
	"itemSpacing", "cornerRadius",
	"rectangleCornerTopLeftRadius", "rectangleCornerTopRightRadius",
	"rectangleCornerBottomLeftRadius", "rectangleCornerBottomRightRadius",
}

// layoutInfo collects a node's meaningful layout geometry. It reports empty for
// nodes without auto layout, padding, rounding or stroke, so static frames do
// not produce noise. Newer .fig files name auto layout with stack* fields
// (stackMode, stackSpacing, stackPadding*) instead of layoutMode; both spell
// the same values and are normalized here.
func layoutInfo(n *Node) map[string]any {
	m := n.raw
	if n.Type == TypeText {
		return nil
	}
	info := map[string]any{}
	lm := first(m, "layoutMode")
	if lm == "NONE" {
		lm = ""
	}
	if lm == "" {
		lm = first(m, "stackMode")
	}
	if lm != "" {
		info["layoutMode"] = lm
		if v := first(m, "primaryAxisAlignItems", "stackPrimaryAlignItems"); v != "" {
			info["primaryAxisAlignItems"] = v
		}
		if v := first(m, "counterAxisAlignItems", "stackCounterAlignItems"); v != "" {
			info["counterAxisAlignItems"] = v
		}
	}
	// Sizing. Newer .fig stores per-axis sizing as stackPrimarySizing /
	// stackCounterSizing using the StackSize enum (FIXED, RESIZE_TO_FIT,
	// RESIZE_TO_FIT_WITH_IMPLICIT_SIZE); older producers expose sizingMode /
	// primaryAxisSizingMode / counterAxisSizingMode. Both are normalized to the
	// IR vocabulary; an axis the file never declared stays empty.
	if v := stackSize(first(m, "stackPrimarySizing", "primaryAxisSizingMode", "sizingMode")); v != "" {
		info["primaryAxisSizingMode"] = v
	}
	if v := stackSize(first(m, "stackCounterSizing", "counterAxisSizingMode")); v != "" {
		info["counterAxisSizingMode"] = v
	}
	// Min/max size are OptionalVector {value:{x,y}} pairs, not flat fields.
	for _, s := range []struct{ size, width, height string }{
		{"minSize", "minWidth", "minHeight"},
		{"maxSize", "maxWidth", "maxHeight"},
	} {
		v := asMap(asMap(m[s.size])["value"])
		if v == nil {
			continue
		}
		if x := asFloat(v["x"]); x > 0 {
			info[s.width] = x
		}
		if y := asFloat(v["y"]); y > 0 {
			info[s.height] = y
		}
	}
	// Padding. Newer .fig stores a base per orientation as
	// stackVerticalPadding (top) and stackHorizontalPadding (left), with
	// optional right/bottom overrides that may be zero; legacy files use the
	// flat padding* fields.
	padTop, padBottom := padAxis(m, "stackVerticalPadding", "stackPaddingTop", "stackPaddingBottom", "paddingTop", "paddingBottom")
	padLeft, padRight := padAxis(m, "stackHorizontalPadding", "stackPaddingLeft", "stackPaddingRight", "paddingLeft", "paddingRight")
	for _, p := range []struct {
		key string
		val float64
	}{
		{"paddingTop", padTop}, {"paddingRight", padRight},
		{"paddingBottom", padBottom}, {"paddingLeft", padLeft},
	} {
		if p.val > 0 {
			info[p.key] = p.val
		}
	}
	for _, field := range layoutFields {
		if v := firstFloat(m, field, stackField(field)); v > 0 {
			info[field] = v
		}
	}
	if sw := asFloat(m["strokeWeight"]); sw > 0 {
		for _, p := range paintsOf(n, "STROKE") {
			if paintWellKnown(p) && paintVisible(p) {
				info["strokeWeight"] = sw
				break
			}
		}
	}
	for _, field := range []string{"strokeCap", "strokeJoin"} {
		if v := asString(first(m, field)); v != "" {
			info[field] = v
		}
	}
	if dash := asSlice(m["strokeDashes"]); len(dash) > 0 {
		info["strokeDashes"] = dash
	}
	for _, field := range []string{"horizontalConstraint", "verticalConstraint"} {
		if v := first(m, field); v != "" {
			info[field] = v
		}
	}
	if len(info) == 0 {
		return nil
	}
	return info
}

// Layout returns the node's normalized auto-layout info (layoutMode, alignment,
// gaps, padding, radius, stroke), empty for nodes without meaningful geometry.
func (n *Node) Layout() map[string]any { return layoutInfo(n) }

// stackField maps a layout field to its stack* sibling used by newer .fig
// producers (itemSpacing → stackSpacing, cornerRadius → stackCornerRadius).
func stackField(field string) string {
	switch field {
	case "itemSpacing":
		return "stackSpacing"
	case "cornerRadius":
		return "stackCornerRadius"
	}
	return ""
}

// padAxis resolves one padding axis. The base value applies to both edges; an
// explicit near-edge override wins for the first edge and an explicit far-edge
// override (which may legitimately be zero) wins for the second. Legacy flat
// padding* fields are accepted as fallbacks.
func padAxis(m map[string]any, base, start, end, legacyStart, legacyEnd string) (float64, float64) {
	first := asFloat(m[base])
	if v := firstFloat(m, start, legacyStart); v > 0 {
		first = v
	}
	last := asFloat(m[base])
	if v, ok := m[end]; ok {
		last = asFloat(v)
	} else if v, ok := m[legacyEnd]; ok {
		last = asFloat(v)
	}
	return first, last
}

// stackSize normalizes a sizing mode to FIXED, HUG or FILL. Newer .fig files
// use the StackSize enum; older ones use the legacy sizingMode spellings.
func stackSize(v string) string {
	switch strings.ToUpper(v) {
	case "FIXED", "RESIZE_TO_FIXED_SIZE":
		return "FIXED"
	case "AUTO", "HUG", "RESIZE_TO_FIT", "RESIZE_TO_FIT_WITH_IMPLICIT_SIZE":
		return "HUG"
	case "FILL", "RESIZE_TO_FILL":
		return "FILL"
	}
	return ""
}

// first returns the first present string value among a set of fallback keys.
func first(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := asString(m[k]); s != "" {
			return s
		}
	}
	return ""
}

// firstFloat returns the first positive value among a key and its fallbacks.
func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v := asFloat(m[k]); v > 0 {
			return v
		}
	}
	return 0
}

// layoutSummary renders a compact description of a node's layout geometry.
func layoutSummary(info map[string]any) string {
	var parts []string
	if lm := asString(info["layoutMode"]); lm != "" {
		part := "layout " + strings.ToLower(lm)
		if a := asString(info["primaryAxisAlignItems"]); a != "" {
			part += "/" + strings.ToLower(a)
		}
		parts = append(parts, part)
	}
	if g := asFloat(info["itemSpacing"]); g > 0 {
		parts = append(parts, "gap "+shortFloat(g))
	}
	if p := layoutPadding(info); p != "" {
		parts = append(parts, "padding "+p)
	}
	if r := cornerRadius(info); r != "" {
		parts = append(parts, "radius "+r)
	}
	if sw := asFloat(info["strokeWeight"]); sw > 0 {
		parts = append(parts, "stroke "+shortFloat(sw))
	}
	if len(parts) == 0 {
		return humanTitle(strings.Join([]string{"layout"}, " "))
	}
	return strings.Join(parts, " · ")
}

func layoutPadding(info map[string]any) string {
	l, r, t, b := asFloat(info["paddingLeft"]), asFloat(info["paddingRight"]), asFloat(info["paddingTop"]), asFloat(info["paddingBottom"])
	switch {
	case l == r && t == b && l == t:
		if l > 0 {
			return shortFloat(l)
		}
	case t == b:
		if l == r {
			return shortFloat(t) + " " + shortFloat(l)
		}
		return shortFloat(t) + " " + shortFloat(l) + " " + shortFloat(r)
	case l == r:
		return shortFloat(t) + " " + shortFloat(l) + " " + shortFloat(b)
	default:
		return shortFloat(t) + " " + shortFloat(r) + " " + shortFloat(b) + " " + shortFloat(l)
	}
	return ""
}

// cornerRadius renders the node's corner rounding as a compact description.
// A uniform rounding collapses to a single value; custom (per-corner) rounding
// lists the four corners in Figma order: top-left, top-right, bottom-right,
// bottom-left.
func cornerRadius(info map[string]any) string {
	if cr := asFloat(info["cornerRadius"]); cr > 0 {
		return shortFloat(cr)
	}
	corners := []string{
		"rectangleCornerTopLeftRadius",
		"rectangleCornerTopRightRadius",
		"rectangleCornerBottomRightRadius",
		"rectangleCornerBottomLeftRadius",
	}
	anySet := false
	nums := make([]float64, 4)
	for i, field := range corners {
		nums[i] = asFloat(info[field])
		if nums[i] > 0 {
			anySet = true
		}
	}
	if !anySet {
		return ""
	}
	if nums[0] == nums[1] && nums[1] == nums[2] && nums[2] == nums[3] {
		return shortFloat(nums[0])
	}
	parts := make([]string, 4)
	for i, v := range nums {
		parts[i] = shortFloat(v)
	}
	return strings.Join(parts, " ")
}

// shortFloat renders a dimension with at most two decimals, dropping trailing
// zeros and decimal points.
func shortFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}