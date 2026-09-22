package query

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/salvadorsru/dali/internal/model"
)

const nodeColumns = `id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, text_align, text_align_v, line_height, line_height_units, letter_spacing, decoration, rotation, clip_content, component_id, runs, path, props, paints, bound_variables`

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(sc scanner) (*model.Node, error) {
	n := &model.Node{}
	var visible, clip int
	var lhu sql.NullString
	var runs, props, paints, boundVars string
	err := sc.Scan(
		&n.ID, &n.PageID, &n.ParentID, &n.Type, &n.Name, &n.Position,
		&visible, &n.Opacity, &n.X, &n.Y, &n.Width, &n.Height,
		&n.Text, &n.FontFamily, &n.FontStyle, &n.FontSize,
		&n.TextAlign, &n.TextAlignV, &n.LineHeight, &lhu, &n.LetterSpacing,
		&n.Decoration, &n.Rotation, &clip, &n.ComponentID,
		&runs, &n.Path, &props, &paints, &boundVars,
	)
	if err != nil {
		return nil, err
	}
	n.Visible = visible != 0
	n.ClipContent = clip != 0
	n.LineHeightUnits = lhu.String
	n.Runs = parseRuns(runs)
	n.Props = parseProps(props)
	n.PaintData = parsePaints(paints)
	n.BoundVars = parseBoundVars(boundVars)
	return n, nil
}

func parsePaints(raw string) *model.Paints {
	if raw == "" {
		return nil
	}
	var p model.Paints
	if json.Unmarshal([]byte(raw), &p) == nil {
		return &p
	}
	return nil
}

func parseBoundVars(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var out map[string]string
	if json.Unmarshal([]byte(raw), &out) == nil {
		return out
	}
	return nil
}

func parseRuns(raw string) []model.TextRun {
	if raw == "" {
		return nil
	}
	var out []model.TextRun
	if json.Unmarshal([]byte(raw), &out) == nil {
		return out
	}
	return nil
}

func parseProps(raw string) []model.PropOverride {
	if raw == "" {
		return nil
	}
	var out []model.PropOverride
	if json.Unmarshal([]byte(raw), &out) == nil {
		return out
	}
	return nil
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

// SubtreeItem is a node in a rooted subtree with its tree depth relative to
// the root (direct children of the root have depth 1; the root itself is
// excluded from the result).
type SubtreeItem struct {
	Node  *model.Node
	Depth int
}

// Subtree returns the descendants of a node in pre-order (parent before
// children, siblings by document order). maxDepth of 0 walks the whole
// subtree; otherwise traversal stops before depth maxDepth.
func (s *Service) Subtree(id string, maxDepth int) ([]SubtreeItem, error) {
	nodeCols := "n." + strings.ReplaceAll(nodeColumns, ", ", ", n.")
	selectNode := "SELECT " + nodeCols + ", d.depth FROM nodes n JOIN d ON n.id = d.id"
	var q string
	var args []any
	if maxDepth > 0 {
		q = `WITH RECURSIVE d(id, depth) AS (
			SELECT ?, 0
			UNION ALL
			SELECT n.id, d.depth + 1 FROM nodes n JOIN d ON n.parent_id = d.id
			WHERE d.depth < ?
		)
		` + selectNode + ` WHERE d.id <> ? ORDER BY d.depth, n.position, n.id`
		args = []any{id, maxDepth, id}
	} else {
		q = `WITH RECURSIVE d(id, depth) AS (
			SELECT ?, 0
			UNION ALL
			SELECT n.id, d.depth + 1 FROM nodes n JOIN d ON n.parent_id = d.id
		)
		` + selectNode + ` WHERE d.id <> ? ORDER BY d.depth, n.position, n.id`
		args = []any{id, id}
	}
	rows, err := s.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubtreeItem
	for rows.Next() {
		n, depth, err := scanNodeDepth(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, SubtreeItem{Node: n, Depth: depth})
	}
	return out, rows.Err()
}

// scanNodeDepth scans a node row plus a trailing depth column.
func scanNodeDepth(rows *sql.Rows) (*model.Node, int, error) {
	n := &model.Node{}
	var visible, clip, depth int
	var lhu sql.NullString
	var runs, props, paints, boundVars string
	err := rows.Scan(
		&n.ID, &n.PageID, &n.ParentID, &n.Type, &n.Name, &n.Position,
		&visible, &n.Opacity, &n.X, &n.Y, &n.Width, &n.Height,
		&n.Text, &n.FontFamily, &n.FontStyle, &n.FontSize,
		&n.TextAlign, &n.TextAlignV, &n.LineHeight, &lhu, &n.LetterSpacing,
		&n.Decoration, &n.Rotation, &clip, &n.ComponentID,
		&runs, &n.Path, &props, &paints, &boundVars, &depth,
	)
	if err != nil {
		return nil, 0, err
	}
	n.Visible = visible != 0
	n.ClipContent = clip != 0
	n.LineHeightUnits = lhu.String
	n.Runs = parseRuns(runs)
	n.Props = parseProps(props)
	n.PaintData = parsePaints(paints)
	n.BoundVars = parseBoundVars(boundVars)
	return n, depth, nil
}
