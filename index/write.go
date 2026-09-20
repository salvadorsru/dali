package index

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// Write stores the document tree. Structural indexing is incremental: each node
// keeps a fingerprint in node_fingerprints, and Write only touches rows whose
// fingerprint changed (new, modified or removed nodes). Unchanged nodes keep
// their nodes, node_assets and text_search rows untouched, so repeated indexing
// of the same file is cheap)Skip. The small derived tables (documents, pages,
// components, instances, assets, styles, style_defs, fonts) are still rebuilt
// wholesale each run—they are tiny and cheap to repopulate.
func (db *DB) Write(doc *model.Document) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Cheap tiny tables: always rebuild.
	for _, t := range []string{"documents", "pages", "components", "instances", "assets", "styles", "style_defs", "fonts"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("clear %s: %w", t, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO documents (id, name) VALUES (?, ?)`, doc.ID, doc.Name); err != nil {
		return err
	}

	// --- pages ---
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

	// --- components / instances / assets / styles / style_defs / fonts ---
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

	assetStmt, err := tx.Prepare(`INSERT INTO assets (hash, name, node_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer assetStmt.Close()
	for _, a := range doc.Assets() {
		if _, err := assetStmt.Exec(a.Hash, a.Name, a.NodeID); err != nil {
			return err
		}
	}

	styleStmt, err := tx.Prepare(`INSERT INTO styles (kind, key, node_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer styleStmt.Close()
	for _, s := range doc.StyleUsages() {
		if _, err := styleStmt.Exec(s.Kind, s.Key, s.NodeID); err != nil {
			return err
		}
	}

	styleDefStmt, err := tx.Prepare(`INSERT OR REPLACE INTO style_defs (key, kind, name, node_id, summary, raw) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer styleDefStmt.Close()
	for _, d := range doc.StyleDefs() {
		if _, err := styleDefStmt.Exec(d.Key, d.Kind, d.Name, d.NodeID, d.Summary, d.Raw); err != nil {
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

	// --- nodes: incremental, diff by fingerprint ---
	oldFP := make(map[string]string)
	rows, err := tx.Query(`SELECT node_id, fingerprint FROM node_fingerprints`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, fp string
		if err := rows.Scan(&id, &fp); err != nil {
			rows.Close()
			return err
		}
		oldFP[id] = fp
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	newFP := make(map[string]string, len(doc.All()))
	var upsert []*model.Node
	for _, n := range doc.All() {
		fp := nodeFingerprint(n)
		newFP[n.ID] = fp
		if oldFP[n.ID] == fp {
			continue // unchanged: reuse row, node_assets and text_search
		}
		upsert = append(upsert, n)
	}

	// Upsert prepared statements.
	nodeStmt, err := tx.Prepare(`INSERT OR REPLACE INTO nodes
		(id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, component_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer nodeStmt.Close()

	nodeAssetStmt, err := tx.Prepare(`INSERT OR REPLACE INTO node_assets (node_id, hash) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer nodeAssetStmt.Close()

	assetUseStmt, err := tx.Prepare(`INSERT OR IGNORE INTO node_assets (node_id, hash) SELECT a.node_id, a.hash FROM node_assets a WHERE a.node_id = ?`)
	if err != nil {
		return err
	}
	defer assetUseStmt.Close()

	ftsStmt, err := tx.Prepare(`INSERT INTO text_search (node_id, name, content) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ftsStmt.Close()

	fpStmt, err := tx.Prepare(`INSERT OR REPLACE INTO node_fingerprints (node_id, fingerprint) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer fpStmt.Close()

	for _, n := range upsert {
		for _, d := range []string{"node_assets", "text_search"} {
			if _, err := tx.Exec(`DELETE FROM `+d+` WHERE node_id = ?`, n.ID); err != nil {
				return fmt.Errorf("clear %s for %s: %w", d, n.ID, err)
			}
		}
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
		if _, err := fpStmt.Exec(n.ID, newFP[n.ID]); err != nil {
			return err
		}
	}

	// Remove rows whose node no longer exists.
	delFP, err := tx.Prepare(`DELETE FROM node_fingerprints WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer delFP.Close()
	for id := range oldFP {
		if _, ok := newFP[id]; ok {
			continue
		}
		for _, d := range []string{"node_assets", "text_search", "nodes"} {
			if _, err := tx.Exec(`DELETE FROM `+d+` WHERE node_id = ?`, id); err != nil {
				return fmt.Errorf("drop %s for %s: %w", d, id, err)
			}
		}
		if _, err := delFP.Exec(id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// nodeFingerprint derives a stable fingerprint from a node's own column values
// and its asset hashes. Two nodes with the same fingerprint are structurally
// identical and their rows can be safely reused.
func nodeFingerprint(n *model.Node) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00",
		n.ID, n.PageID, n.ParentID, n.Type, n.Name, n.Position)
	fmt.Fprintf(h, "%t\x00%.2f\x00%.2f\x00%.2f\x00%.2f\x00%.2f\x00",
		n.Visible, n.Opacity, n.X, n.Y, n.Width, n.Height)
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%.2f\x00%s\x00",
		n.Text, n.FontFamily, n.FontStyle, n.FontSize, n.ComponentID)
	hashes := make([]string, 0, len(n.Images))
	for _, img := range n.Images {
		hashes = append(hashes, img.Hash)
	}
	sort.Strings(hashes)
	fmt.Fprintf(h, "%s", strings.Join(hashes, ","))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
