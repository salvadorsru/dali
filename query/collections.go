package query

import "github.com/salvadorsru/dali/model"

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
	return s.styles(`SELECT kind, key, node_id FROM styles WHERE node_id = ? ORDER BY kind, key`, nodeID)
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

func (s *Service) AssetsOf(nodeID string) ([]model.ImageRef, error) {
	const q = `SELECT a.hash, a.name, a.node_id, COALESCE(a.mime,''), COALESCE(a.path,''), COALESCE(a.width,0), COALESCE(a.height,0)
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
