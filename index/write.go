package index

import (
	"fmt"

	"github.com/salvadorsru/dali/model"
)

func (db *DB) Write(doc *model.Document) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range []string{"documents", "pages", "nodes", "node_assets", "components", "instances", "assets", "styles", "fonts", "text_search"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("clear %s: %w", t, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO documents (id, name) VALUES (?, ?)`, doc.ID, doc.Name); err != nil {
		return err
	}

	pageStmt, err := tx.Prepare(`INSERT INTO pages (id, name, position, ordinal) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer pageStmt.Close()
	for i, p := range doc.Pages {
		if _, err := pageStmt.Exec(p.ID, p.Name, p.Position, i); err != nil {
			return err
		}
	}

	nodeStmt, err := tx.Prepare(`INSERT INTO nodes
		(id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, component_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer nodeStmt.Close()

	nodeAssetStmt, err := tx.Prepare(`INSERT OR IGNORE INTO node_assets (node_id, hash) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer nodeAssetStmt.Close()

	ftsStmt, err := tx.Prepare(`INSERT INTO text_search (node_id, name, content) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ftsStmt.Close()

	for _, n := range doc.All() {
		if _, err := nodeStmt.Exec(
			n.ID, n.PageID, n.ParentID, string(n.Type), n.Name, n.Position,
			boolInt(n.Visible), n.Opacity, n.X, n.Y, n.Width, n.Height,
			n.Text, n.FontFamily, n.FontStyle, n.FontSize, n.ComponentID,
		); err != nil {
			return err
		}
		for _, img := range n.Images {
			if _, err := nodeAssetStmt.Exec(n.ID, img.Hash); err != nil {
				return err
			}
		}
		if n.Name != "" || n.Text != "" {
			if _, err := ftsStmt.Exec(n.ID, n.Name, n.Text); err != nil {
				return err
			}
		}
	}

	componentStmt, err := tx.Prepare(`INSERT INTO components (id, name, page_id, parent_id) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer componentStmt.Close()
	for _, n := range doc.Components() {
		if _, err := componentStmt.Exec(n.ID, n.Name, n.PageID, n.ParentID); err != nil {
			return err
		}
	}

	instanceStmt, err := tx.Prepare(`INSERT INTO instances (id, name, page_id, parent_id) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer instanceStmt.Close()
	for _, n := range doc.Instances() {
		if _, err := instanceStmt.Exec(n.ID, n.Name, n.PageID, n.ParentID); err != nil {
			return err
		}
	}

	assetStmt, err := tx.Prepare(`INSERT OR REPLACE INTO assets (hash, name, node_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer assetStmt.Close()
	for _, a := range doc.Assets() {
		if _, err := assetStmt.Exec(a.Hash, a.Name, a.NodeID); err != nil {
			return err
		}
	}

	styleStmt, err := tx.Prepare(`INSERT OR REPLACE INTO styles (kind, key, node_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer styleStmt.Close()
	for _, s := range doc.StyleUsages() {
		if _, err := styleStmt.Exec(s.Kind, s.Key, s.NodeID); err != nil {
			return err
		}
	}

	fontStmt, err := tx.Prepare(`INSERT OR REPLACE INTO fonts (family, style, count) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer fontStmt.Close()
	for _, f := range doc.Fonts() {
		if _, err := fontStmt.Exec(f.Family, f.Style, f.Count); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
