package query

import (
	"regexp"
	"sort"

	"github.com/salvadorsru/dali/internal/model"
)

// SolidColor is one unique solid paint used by the document, deduplicated by
// hex value and ranked by how many dressings reference it.
type SolidColor struct {
	Hex     string `json:"hex"`
	Usage   int    `json:"usage"`
	Fills   int    `json:"fills,omitempty"`
	Strokes int    `json:"strokes,omitempty"`
}

// TextClass groups text nodes by family, style and size.
type TextClass struct {
	Family string  `json:"family"`
	Style  string  `json:"style"`
	Size   float64 `json:"size"`
	Usage  int     `json:"usage"`
}

// TokenLayout is a resolved flex layout (direction, gap, padding, radius) and
// how many nodes use it.
type TokenLayout struct {
	Layout string `json:"layout"`
	Usage  int    `json:"usage"`
}

var solidHex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

// Palette returns solid fills and strokes deduplicated by hex, ranked by
// usage. minUsage drops colours used fewer times; limit caps the result
// (0 = all).
func (s *Service) Palette(minUsage, limit int) ([]SolidColor, error) {
	defs, err := s.StyleDefs()
	if err != nil {
		return nil, err
	}
	type acc struct{ usage, fills, strokes int }
	seen := make(map[string]*acc)
	for _, d := range defs {
		if d.Kind != "FILL" && d.Kind != "STROKE" {
			continue
		}
		if !solidHex.MatchString(d.Summary) {
			continue
		}
		a, ok := seen[d.Summary]
		if !ok {
			a = &acc{}
			seen[d.Summary] = a
		}
		a.usage += d.Usage
		if d.Kind == "FILL" {
			a.fills += d.Usage
		} else {
			a.strokes += d.Usage
		}
	}
	out := make([]SolidColor, 0, len(seen))
	for hex, a := range seen {
		if a.usage < minUsage {
			continue
		}
		out = append(out, SolidColor{Hex: hex, Usage: a.usage, Fills: a.fills, Strokes: a.strokes})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Usage > out[j].Usage })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TextClasses groups text nodes by family, style and size, ranked by usage.
func (s *Service) TextClasses(limit int) ([]TextClass, error) {
	rows, err := s.sql.Query(`SELECT font_family, font_style, font_size, COUNT(*)
		FROM nodes WHERE font_family <> '' AND font_size > 0
		GROUP BY font_family, font_style, font_size
		ORDER BY COUNT(*) DESC, font_family, font_size`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TextClass
	for rows.Next() {
		var c TextClass
		if err := rows.Scan(&c.Family, &c.Style, &c.Size, &c.Usage); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TokenLayouts aggregates layout styles by their resolved summary, ranked by
// usage.
func (s *Service) TokenLayouts(limit int) ([]TokenLayout, error) {
	defs, err := s.StyleDefs()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, d := range defs {
		if d.Kind != "LAYOUT" || d.Summary == "" {
			continue
		}
		counts[d.Summary] += d.Usage
	}
	out := make([]TokenLayout, 0, len(counts))
	for layout, usage := range counts {
		out = append(out, TokenLayout{Layout: layout, Usage: usage})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Usage > out[j].Usage })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Usages returns the root placements of a component: every INSTANCE whose
// component_id is id, excluding the materialized clones inside other
// component libraries.
func (s *Service) Usages(componentID string) ([]*model.Node, error) {
	return s.queryNodes(`WHERE component_id = ? AND type = 'INSTANCE' AND id NOT LIKE 'inst:%'
		ORDER BY page_id, name`, componentID)
}
