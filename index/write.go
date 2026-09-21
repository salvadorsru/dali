package index

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// Write stores the document tree. Structural indexing is incremental: each node
// keeps a fingerprint in node_fingerprints, and Write only touches rows whose
// fingerprint changed (new, modified or removed nodes). Unchanged nodes keep
// their nodes, node_assets and text_search rows untouched, so repeated indexing
// of the same file is cheap). The small derived tables (documents, pages,
// components, instances, assets, styles, style_defs, fonts) are still rebuilt
// wholesale each run—they are tiny and cheap to repopulate.
func (db *DB) Write(doc *model.Document) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Cheap tiny tables: always rebuild.
	for _, t := range []string{"documents", "pages", "components", "instances", "assets", "styles", "style_defs", "fonts", "variables"} {
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

	variableStmt, err := tx.Prepare(`INSERT OR REPLACE INTO variables (id, key, name, vset, type, modes) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer variableStmt.Close()
	for _, v := range doc.Variables {
		modes, _ := json.Marshal(v.Modes)
		if _, err := variableStmt.Exec(v.ID, v.Key, v.Name, v.Set, v.Type, string(modes)); err != nil {
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
	var removed []string
	for id := range oldFP {
		if _, ok := newFP[id]; !ok {
			removed = append(removed, id)
		}
	}

	// Derived rows for every changed node are cleared in one pass. text_search
	// is FTS5 with node_id UNINDEXED, so deleting per node rescans the whole
	// table each time — quadratic on a fresh index.
	if len(upsert) > 0 || len(removed) > 0 {
		if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS dirty_nodes (node_id TEXT PRIMARY KEY)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM dirty_nodes`); err != nil {
			return err
		}
		dirtyStmt, err := tx.Prepare(`INSERT OR IGNORE INTO dirty_nodes (node_id) VALUES (?)`)
		if err != nil {
			return err
		}
		defer dirtyStmt.Close()
		for _, n := range upsert {
			if _, err := dirtyStmt.Exec(n.ID); err != nil {
				return err
			}
		}
		for _, id := range removed {
			if _, err := dirtyStmt.Exec(id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`DELETE FROM text_search WHERE node_id IN (SELECT node_id FROM dirty_nodes)`); err != nil {
			return fmt.Errorf("clear text_search: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM node_assets WHERE node_id IN (SELECT node_id FROM dirty_nodes)`); err != nil {
			return fmt.Errorf("clear node_assets: %w", err)
		}
	}

	// Upsert prepared statements.
	nodeStmt, err := tx.Prepare(`INSERT OR REPLACE INTO nodes
		(id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, text_align, text_align_v, line_height, line_height_units, letter_spacing, decoration, rotation, clip_content, component_id, runs, path, props, paints, bound_variables)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer nodeStmt.Close()

	nodeAssetStmt, err := tx.Prepare(`INSERT OR REPLACE INTO node_assets (node_id, hash) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer nodeAssetStmt.Close()

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
		paints := ""
		if p := n.PaintStack(); len(p.Fills)+len(p.Strokes)+len(p.Effects) > 0 {
			paints = jsonString(p)
		}
		bound := ""
		if bv := doc.BoundVariables(n); len(bv) > 0 {
			bound = jsonString(bv)
		}
		if _, err := nodeStmt.Exec(
			n.ID, n.PageID, n.ParentID, string(n.Type), n.Name, n.Position,
			boolInt(n.Visible), n.Opacity, n.X, n.Y, n.Width, n.Height,
			n.Text, n.FontFamily, n.FontStyle, n.FontSize,
			n.TextAlign, n.TextAlignV, n.LineHeight, n.LineHeightUnits, n.LetterSpacing, n.Decoration,
			n.Rotation, boolInt(n.ClipContent), n.ComponentID,
			jsonString(n.Runs), n.Path, jsonString(n.Props), paints, bound,
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
	delNodeStmt, err := tx.Prepare(`DELETE FROM nodes WHERE id = ?`)
	if err != nil {
		return err
	}
	defer delNodeStmt.Close()
	delFPStmt, err := tx.Prepare(`DELETE FROM node_fingerprints WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer delFPStmt.Close()
	for _, id := range removed {
		if _, err := delNodeStmt.Exec(id); err != nil {
			return fmt.Errorf("drop nodes for %s: %w", id, err)
		}
		if _, err := delFPStmt.Exec(id); err != nil {
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
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%.2f\x00%s\x00%.2f\x00%.2f\x00%s\x00%s\x00%.2f\x00%t\x00%s\x00%s\x00%x\x00%s\x00",
		n.Text, n.FontFamily, n.FontStyle, n.FontSize, n.ComponentID,
		n.LetterSpacing, n.LineHeight, n.LineHeightUnits, n.TextAlign, n.Rotation, n.ClipContent, n.Decoration,
		n.Path, n.Runs, jsonString(n.Props))
	fmt.Fprintf(h, "%s\x00%s\x00", jsonString(n.PaintStack()), n.BoundVarsJSON())
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

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
