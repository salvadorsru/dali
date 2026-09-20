package query

import (
	"encoding/json"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// applyRaw parses the style definition's stored raw payload into an
// unstructured value, so callers get the full authored detail (gradient
// stops, transform, text styling) without another lookup.
func applyRaw(st *model.Style, raw string) {
	if raw == "" {
		return
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) == nil {
		st.Value = v
	}
}

func (s *Service) Components() ([]*model.Node, error) {
	return s.queryNodes(`WHERE type IN ('SYMBOL','COMPONENT','COMPONENT_SET') ORDER BY name`)
}

func (s *Service) Instances() ([]*model.Node, error) {
	return s.queryNodes(`WHERE type = 'INSTANCE' ORDER BY name`)
}

func (s *Service) Assets() ([]model.ImageRef, error) {
	rows, err := s.sql.Query(`SELECT hash, name, node_id, COALESCE(mime,''), COALESCE(path,''), COALESCE(width,0), COALESCE(height,0) FROM assets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ImageRef
	for rows.Next() {
		var a model.ImageRef
		if err := rows.Scan(&a.Hash, &a.Name, &a.NodeID, &a.MIME, &a.Path, &a.Width, &a.Height); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) UpdateAssetInfo(refs []model.ImageRef) error {
	tx, err := s.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE assets SET mime = ?, path = ?, width = ?, height = ? WHERE hash = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range refs {
		if _, err := stmt.Exec(r.MIME, r.Path, r.Width, r.Height, r.Hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Variables returns every design token with its modes, ordered by set then
// name.
func (s *Service) Variables() ([]model.Variable, error) {
	rows, err := s.sql.Query(`SELECT id, key, name, vset, type, modes FROM variables ORDER BY vset, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Variable
	for rows.Next() {
		var v model.Variable
		var modes string
		if err := rows.Scan(&v.ID, &v.Key, &v.Name, &v.Set, &v.Type, &modes); err != nil {
			return nil, err
		}
		if modes != "" {
			_ = json.Unmarshal([]byte(modes), &v.Modes)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) Fonts() ([]model.Font, error) {
	rows, err := s.sql.Query(`SELECT family, style, count FROM fonts ORDER BY count DESC, family`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Font
	for rows.Next() {
		var f model.Font
		if err := rows.Scan(&f.Family, &f.Style, &f.Count); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) Styles() ([]model.Style, error) {
	return s.styles(`SELECT kind, key, MIN(node_id) FROM styles GROUP BY kind, key ORDER BY kind, key`)
}

func (s *Service) StylesOf(nodeID string) ([]model.Style, error) {
	rows, err := s.sql.Query(`SELECT st.kind, st.key, st.node_id, COALESCE(d.name, ''), COALESCE(d.summary, ''), COALESCE(d.raw, '')
		FROM styles st LEFT JOIN style_defs d ON d.key = st.key
		WHERE st.node_id = ? ORDER BY st.kind, st.key`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Style
	for rows.Next() {
		var st model.Style
		var raw string
		if err := rows.Scan(&st.Kind, &st.Key, &st.NodeID, &st.Name, &st.Summary, &raw); err != nil {
			return nil, err
		}
		applyRaw(&st, raw)
		out = append(out, st)
	}
	return out, rows.Err()
}

// StyleDefs returns every style definition with its resolved value and usage
// count, ordered by kind then name.
func (s *Service) StyleDefs() ([]model.StyleDef, error) {
	rows, err := s.sql.Query(`SELECT d.key, d.kind, d.name, d.node_id, d.summary, d.raw, (SELECT COUNT(*) FROM styles st WHERE st.key = d.key)
		FROM style_defs d ORDER BY d.kind, d.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StyleDef
	for rows.Next() {
		var d model.StyleDef
		if err := rows.Scan(&d.Key, &d.Kind, &d.Name, &d.NodeID, &d.Summary, &d.Raw, &d.Usage); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) styles(query string, args ...any) ([]model.Style, error) {
	rows, err := s.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Style
	for rows.Next() {
		var st model.Style
		if err := rows.Scan(&st.Kind, &st.Key, &st.NodeID); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Service) StylesOfMany(ids []string) (map[string][]model.Style, error) {
	if len(ids) == 0 {
		return map[string][]model.Style{}, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	rows, err := s.sql.Query(`SELECT st.kind, st.key, st.node_id, COALESCE(d.name, ''), COALESCE(d.summary, ''), COALESCE(d.raw, '')
		FROM styles st LEFT JOIN style_defs d ON d.key = st.key
		WHERE st.node_id IN (`+placeholders+`) ORDER BY st.kind, st.key`, toAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]model.Style)
	for rows.Next() {
		var st model.Style
		var raw string
		if err := rows.Scan(&st.Kind, &st.Key, &st.NodeID, &st.Name, &st.Summary, &raw); err != nil {
			return nil, err
		}
		applyRaw(&st, raw)
		out[st.NodeID] = append(out[st.NodeID], st)
	}
	return out, rows.Err()
}

func (s *Service) AssetsOfMany(ids []string) (map[string][]model.ImageRef, error) {
	if len(ids) == 0 {
		return map[string][]model.ImageRef{}, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	rows, err := s.sql.Query(`SELECT a.hash, a.name, na.node_id, COALESCE(a.mime,''), COALESCE(a.path,''), COALESCE(a.width,0), COALESCE(a.height,0)
		FROM node_assets na JOIN assets a ON a.hash = na.hash
		WHERE na.node_id IN (`+placeholders+`) ORDER BY a.name`, toAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]model.ImageRef)
	for rows.Next() {
		var a model.ImageRef
		if err := rows.Scan(&a.Hash, &a.Name, &a.NodeID, &a.MIME, &a.Path, &a.Width, &a.Height); err != nil {
			return nil, err
		}
		out[a.NodeID] = append(out[a.NodeID], a)
	}
	return out, rows.Err()
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func (s *Service) AssetsOf(nodeID string) ([]model.ImageRef, error) {
	const q = `SELECT a.hash, a.name, na.node_id, COALESCE(a.mime,''), COALESCE(a.path,''), COALESCE(a.width,0), COALESCE(a.height,0)
		FROM node_assets na JOIN assets a ON a.hash = na.hash
		WHERE na.node_id = ? ORDER BY a.name`
	rows, err := s.sql.Query(q, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ImageRef
	for rows.Next() {
		var a model.ImageRef
		if err := rows.Scan(&a.Hash, &a.Name, &a.NodeID, &a.MIME, &a.Path, &a.Width, &a.Height); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
