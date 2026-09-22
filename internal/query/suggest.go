package query

import (
	"sort"
	"strings"
)

// Suggest is a layer name that resembles a search pattern, for hinting when
// no exact match exists.
type Suggest struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// SuggestNames returns distinct layer names that resemble pattern, ranked by
// how many of the pattern's tokens appear in each name. Exact or substring
// matches are not needed: the caller asks only after finding none.
func (s *Service) SuggestNames(pattern string, limit int) ([]Suggest, error) {
	parts := splitTokens(pattern)
	if len(parts) == 0 {
		return nil, nil
	}
	rows, err := s.sql.Query(`SELECT DISTINCT name FROM nodes WHERE name <> '' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	scored := make([]Suggest, 0, len(names))
	for _, name := range names {
		lower := strings.ToLower(name)
		score := 0
		for _, part := range parts {
			if strings.Contains(lower, part) {
				score++
			}
		}
		if score > 0 {
			scored = append(scored, Suggest{Name: name, Score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Name < scored[j].Name
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// SuggestXPath suggests near-miss names for an XPath expression that matched
// nothing, scoring on the predicate value of the last step, falling back to
// its node type.
func (s *Service) SuggestXPath(expr string, limit int) ([]Suggest, error) {
	sel, err := parseXPath(expr)
	if err != nil {
		return nil, nil
	}
	last := sel.steps[len(sel.steps)-1]
	pattern := last.name
	if pattern == "" {
		pattern = last.ntype
	}
	if pattern == "" {
		return nil, nil
	}
	return s.SuggestNames(pattern, limit)
}

// splitTokens turns a search pattern into lowercase words, splitting on
// whitespace and camelCase boundaries like pencil's name suggester.
func splitTokens(pattern string) []string {
	var b strings.Builder
	prevLower := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c >= 'A' && c <= 'Z' && prevLower {
			b.WriteByte(' ')
		}
		prevLower = c >= 'a' && c <= 'z'
		b.WriteByte(c)
	}
	return strings.Fields(strings.ToLower(b.String()))
}
