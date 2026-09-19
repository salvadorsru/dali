package query

import (
	"strings"

	"github.com/salvadorsru/dali/model"
)

type Result struct {
	ID    string
	Type  string
	Name  string
	Text  string
	Page  string
	Path  string
	Score float64
}

func (s *Service) Search(query string, limit int) ([]Result, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.sql.Query(
		`SELECT node_id, bm25(text_search) FROM text_search
		 WHERE text_search MATCH ? ORDER BY bm25(text_search) LIMIT ?`, match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.fill(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) fill(r *Result) error {
	n, err := s.Node(r.ID)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}
	r.Type = string(n.Type)
	r.Name = n.Name
	r.Text = n.Text
	path, err := s.Path(r.ID)
	if err != nil {
		return err
	}
	r.Path = formatPath(path)
	for _, p := range path {
		if p.Type == model.TypeCanvas {
			r.Page = p.Name
			break
		}
	}
	return nil
}

func formatPath(path []*model.Node) string {
	names := make([]string, 0, len(path))
	for _, n := range path {
		names = append(names, n.Name)
	}
	return strings.Join(names, " > ")
}

// ftsQuery turns free text into a safe FTS5 match expression.
func ftsQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}
