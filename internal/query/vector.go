package query

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/internal/model"
)

// Vector is one vector resource inside a node's subtree: the exact geometry
// that renders it, the paint it renders with, and what a placement changed
// locally. Width and Height are the box the path is authored in — for a
// placement, the component's box, not the scaled one the clone reports.
type Vector struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Source  string        `json:"source,omitempty"`
	ViewBox string        `json:"view_box,omitempty"`
	Width   float64       `json:"width"`
	Height  float64       `json:"height"`
	Path    string        `json:"path"`
	Paints  *model.Paints `json:"paints,omitempty"`

	// Overrides are the paint fields where this placement differs from the
	// component default it was copied from. Empty for a node that is not a
	// placement.
	Overrides []Override `json:"overrides,omitempty"`
}

// Override is one paint field where a placement renders differently from its
// component default. Values are the paint payloads themselves rather than a
// summary, so opacity, blend mode and visibility survive the round trip.
type Override struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// Vectors returns the vector resources inside a node's subtree, including the
// node itself when it carries a path. Cloned placements resolve to the
// component-side node for geometry, because instance scaling multiplies a
// clone's width and height but leaves its path in the component's coordinate
// space; the clone's own paints are kept, since they already carry whatever the
// placement overrode.
func (s *Service) Vectors(id string) ([]Vector, error) {
	root, err := s.Node(id)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, nil
	}
	candidates := make([]*model.Node, 0, 8)
	if root.Path != "" {
		candidates = append(candidates, root)
	}
	sub, err := s.Subtree(id, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range sub {
		if item.Node.Path != "" {
			candidates = append(candidates, item.Node)
		}
	}

	sources := map[string]*model.Node{}
	out := make([]Vector, 0, len(candidates))
	for _, n := range candidates {
		v := Vector{ID: n.ID, Name: n.Name, Path: n.Path, Paints: n.PaintData, Width: n.Width, Height: n.Height}
		srcID, cloned := model.SourceID(n.ID)
		if cloned {
			v.Source = srcID
			src, ok := sources[srcID]
			if !ok {
				src, err = s.Node(srcID)
				if err != nil {
					return nil, err
				}
				sources[srcID] = src
			}
			// A clone whose source is not indexed cannot be resolved; report it
			// against the clone's own box rather than dropping the resource.
			if src == nil {
				src = n
			} else {
				v.Overrides = diffPaints(src.PaintData, n.PaintData)
			}
			v.Path, v.Width, v.Height = src.Path, src.Width, src.Height
		}
		v.ViewBox = viewBox(v.Width, v.Height)
		out = append(out, v)
	}
	return out, nil
}

func viewBox(w, h float64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	return fmt.Sprintf("0 0 %g %g", w, h)
}

// diffPaints reports the paint fields a placement changed relative to its
// component default. It covers the paint surface only: a Figma override can also
// restyle text, which no vector resource renders.
func diffPaints(from, to *model.Paints) []Override {
	f, t := paintsOrEmpty(from), paintsOrEmpty(to)
	var out []Override
	if !samePaint(f.Fills, t.Fills) {
		out = append(out, Override{Field: "fills", From: f.Fills, To: t.Fills})
	}
	if !samePaint(f.Strokes, t.Strokes) {
		out = append(out, Override{Field: "strokes", From: f.Strokes, To: t.Strokes})
	}
	if !samePaint(f.StrokeWeight, t.StrokeWeight) {
		out = append(out, Override{Field: "stroke_weight", From: f.StrokeWeight, To: t.StrokeWeight})
	}
	if !samePaint(f.Effects, t.Effects) {
		out = append(out, Override{Field: "effects", From: f.Effects, To: t.Effects})
	}
	return out
}

// samePaint reports whether two paint payloads hold the same value, comparing
// floats at float32 width.
//
// A component's raw payload keeps a widened f32, while deepCopyRaw flattens a
// placement's copy through JSON, so one authored weight reaches the diff as
// 1.600000023841858 on one side and 1.6 on the other. Figma's own fields are two
// decimals, so float32 width is generous, and the values Dalí stores are left
// alone: only the comparison is tolerant.
func samePaint(a, b any) bool {
	return sameValue(trees(a), trees(b))
}

// trees decodes a paint payload into a generic tree so its numbers can be
// compared by value instead of by struct identity.
func trees(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil
	}
	return tree
}

func sameValue(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		return ok && (x == y || float32(x) == float32(y))
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !sameValue(x[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !sameValue(v, w) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func paintsOrEmpty(p *model.Paints) model.Paints {
	if p == nil {
		return model.Paints{}
	}
	return *p
}

// SVG renders the vector as a standalone SVG element ready to paste into a
// component. Fill and stroke come from the effective paints, so a placement's
// override travels with the geometry. A gradient or image fill has no SVG
// attribute form here, so the attribute is left off and the paints payload
// carries the detail.
func (v Vector) SVG() string {
	if v.Path == "" {
		return ""
	}
	paints := paintsOrEmpty(v.Paints)
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
	if v.ViewBox != "" {
		b.WriteString(` viewBox="` + v.ViewBox + `"`)
		fmt.Fprintf(&b, ` width="%g" height="%g"`, v.Width, v.Height)
	}
	if fill := solidColor(paints.Fills); fill != "" {
		b.WriteString(` fill="` + fill + `"`)
	} else {
		// SVG defaults fill to black; Figma leaves a vector with no fill paints
		// unfilled. Omitting the attribute would paint every stroke-only icon
		// black, so the absence has to be stated.
		b.WriteString(` fill="none"`)
	}
	if stroke := solidColor(paints.Strokes); stroke != "" && paints.StrokeWeight > 0 {
		b.WriteString(` stroke="` + stroke + `"`)
		fmt.Fprintf(&b, ` stroke-width="%g"`, paints.StrokeWeight)
	}
	b.WriteString(`><path d="` + v.Path + `"/></svg>`)
	return b.String()
}

// solidColor returns the first visible solid color in a paint list.
func solidColor(paints []model.Paint) string {
	for _, p := range paints {
		if p.Visible && p.Type == "SOLID" && p.Color != "" {
			return p.Color
		}
	}
	return ""
}
