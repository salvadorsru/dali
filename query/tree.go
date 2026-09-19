package query

import (
	"database/sql"

	"github.com/salvadorsru/dali/model"
)

const nodeColumns = `id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, component_id`

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(sc scanner) (*model.Node, error) {
	n := &model.Node{}
	var visible int
	err := sc.Scan(
		&n.ID, &n.PageID, &n.ParentID, &n.Type, &n.Name, &n.Position,
		&visible, &n.Opacity, &n.X, &n.Y, &n.Width, &n.Height,
		&n.Text, &n.FontFamily, &n.FontStyle, &n.FontSize, &n.ComponentID,
	)
	if err != nil {
		return nil, err
	}
	n.Visible = visible != 0
	return n, nil
}

func (s *Service) queryNodes(where string, args ...any) ([]*model.Node, error) {
	rows, err := s.sql.Query(`SELECT `+nodeColumns+` FROM nodes `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Service) Node(id string) (*model.Node, error) {
	n, err := scanNode(s.sql.QueryRow(`SELECT `+nodeColumns+` FROM nodes WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

func (s *Service) Children(id string) ([]*model.Node, error) {
	return s.queryNodes(`WHERE parent_id = ? ORDER BY position`, id)
}

func (s *Service) Parent(id string) (*model.Node, error) {
	n, err := s.Node(id)
	if err != nil || n == nil || n.ParentID == "" {
		return nil, err
	}
	return s.Node(n.ParentID)
}

func (s *Service) Ancestors(id string) ([]*model.Node, error) {
	var chain []*model.Node
	for cur := id; ; {
		n, err := s.Node(cur)
		if err != nil || n == nil {
			return nil, err
		}
		if n.ParentID == "" {
			break
		}
		p, err := s.Node(n.ParentID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			break
		}
		chain = append(chain, p)
		cur = p.ID
	}
	// reverse to root-first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func (s *Service) Path(id string) ([]*model.Node, error) {
	ancestors, err := s.Ancestors(id)
	if err != nil {
		return nil, err
	}
	n, err := s.Node(id)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return ancestors, nil
	}
	return append(ancestors, n), nil
}

func (s *Service) Descendants(id string) ([]*model.Node, error) {
	const q = `WITH RECURSIVE d(id) AS (
		SELECT id FROM nodes WHERE parent_id = ?
		UNION ALL
		SELECT n.id FROM nodes n JOIN d ON n.parent_id = d.id
	)
	SELECT ` + nodeColumns + ` FROM nodes WHERE id IN (SELECT id FROM d)`
	rows, err := s.sql.Query(q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
