package query

import (
	"strings"

	"github.com/salvadorsru/dali/model"
)

type Result struct {
	ID        string
	Type      string
	Name      string
	Text      string
	Page      string
	Path      string
	Score     float64
	Distance  float64
	Parent    *NodeRef  `json:"parent,omitempty"`
	Children  []NodeRef `json:"children,omitempty"`
	Ancestors []NodeRef `json:"ancestors,omitempty"`
}

// NodeRef is a lightweight reference to a node used for contextual fields so a
// result's neighbourhood is visible without loading full trees.
type NodeRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
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

// fillContext enriches a result with its immediate parent, the ancestor chain,
// and the most relevant direct children (capped so large frames stay cheap).
func (s *Service) fillContext(r *Result, maxChildren int) error {
	p, err := s.Parent(r.ID)
	if err != nil {
		return err
	}
	if p != nil {
		r.Parent = &NodeRef{ID: p.ID, Type: string(p.Type), Name: p.Name}
	}
	path, err := s.Path(r.ID)
	if err != nil {
		return err
	}
	r.Ancestors = make([]NodeRef, 0, len(path))
	for _, p := range path {
		r.Ancestors = append(r.Ancestors, NodeRef{ID: p.ID, Type: string(p.Type), Name: p.Name})
	}
	if maxChildren > 0 {
		children, err := s.childrenRefs(r.ID, maxChildren)
		if err != nil {
			return err
		}
		r.Children = children
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

// childrenRefs loads the direct children of a node as lightweight references,
// ordered by position and capped at max to keep large frames cheap.
func (s *Service) childrenRefs(id string, max int) ([]NodeRef, error) {
	children, err := s.Children(id)
	if err != nil {
		return nil, err
	}
	if max > 0 && len(children) > max {
		children = children[:max]
	}
	out := make([]NodeRef, 0, len(children))
	for _, c := range children {
		out = append(out, NodeRef{ID: c.ID, Type: string(c.Type), Name: c.Name})
	}
	return out, nil
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
