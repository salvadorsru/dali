package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

const SchemaVersion = 6

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout = 15000`); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		sqlDB.Close()
		return nil, err
	}
	for _, pragma := range []string{
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA temp_store = MEMORY`,
		`PRAGMA cache_size = -65536`,
		`PRAGMA mmap_size = 268435456`,
	} {
		_, _ = sqlDB.Exec(pragma)
	}
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) SQL() *sql.DB { return db.sql }

func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS documents (id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE IF NOT EXISTS pages (
			id TEXT PRIMARY KEY,
			name TEXT,
			position TEXT,
			ordinal INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			page_id TEXT,
			parent_id TEXT,
			type TEXT NOT NULL,
			name TEXT,
			position TEXT,
			visible INTEGER,
			opacity REAL,
			x REAL, y REAL, width REAL, height REAL,
			text TEXT,
			font_family TEXT,
			font_style TEXT,
			font_size REAL,
			text_align TEXT,
			text_align_v TEXT,
			line_height REAL,
			letter_spacing REAL,
			decoration TEXT,
			rotation REAL,
			clip_content INTEGER,
			component_id TEXT,
			runs TEXT,
			path TEXT,
			props TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS nodes_type_idx ON nodes(type)`,
		`CREATE INDEX IF NOT EXISTS nodes_name_idx ON nodes(name)`,
		`CREATE INDEX IF NOT EXISTS nodes_parent_idx ON nodes(parent_id)`,
		`CREATE INDEX IF NOT EXISTS nodes_page_idx ON nodes(page_id)`,
		`CREATE TABLE IF NOT EXISTS node_fingerprints (
			node_id TEXT PRIMARY KEY,
			fingerprint TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS node_assets (
			node_id TEXT,
			hash TEXT,
			PRIMARY KEY (node_id, hash)
		)`,
		`CREATE INDEX IF NOT EXISTS node_assets_hash_idx ON node_assets(hash)`,
		`CREATE TABLE IF NOT EXISTS components (
			id TEXT PRIMARY KEY,
			name TEXT,
			page_id TEXT,
			parent_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS instances (
			id TEXT PRIMARY KEY,
			name TEXT,
			page_id TEXT,
			parent_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS assets (
			hash TEXT PRIMARY KEY,
			name TEXT,
			node_id TEXT,
			mime TEXT,
			path TEXT,
			width INTEGER,
			height INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS styles (
			kind TEXT,
			key TEXT,
			node_id TEXT,
			PRIMARY KEY (node_id, kind, key)
		)`,
		`CREATE TABLE IF NOT EXISTS style_defs (
			key TEXT PRIMARY KEY,
			kind TEXT,
			name TEXT,
			node_id TEXT,
			summary TEXT,
			raw TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS fonts (
			family TEXT,
			style TEXT,
			count INTEGER,
			PRIMARY KEY (family, style)
		)`,
		`CREATE TABLE IF NOT EXISTS variables (
			id TEXT PRIMARY KEY,
			key TEXT,
			name TEXT,
			vset TEXT,
			type TEXT,
			modes TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS text_search USING fts5(
			node_id UNINDEXED,
			name,
			content,
			tokenize = 'unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			node_id TEXT PRIMARY KEY,
			model TEXT,
			dimensions INTEGER,
			content_hash TEXT,
			generator_version INTEGER,
			updated_at INTEGER
		)`,
	}
	for _, s := range stmts {
		if _, err := db.sql.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return db.ensureColumns()
}

// ensureColumns backfills columns added to existing databases by newer schema
// versions, so old indexes keep working until the next rebuild.
func (db *DB) ensureColumns() error {
	want := map[string][]string{
		"nodes": {"text_align", "text_align_v", "line_height", "letter_spacing", "decoration", "rotation", "clip_content", "runs", "path", "props"},
	}
	for table, cols := range want {
		rows, err := db.sql.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			return err
		}
		have := map[string]bool{}
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dflt any
			var pk int
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				return err
			}
			have[name] = true
		}
		rows.Close()
		for _, col := range cols {
			if have[col] {
				continue
			}
			typ := "TEXT"
			switch col {
			case "line_height", "letter_spacing", "rotation":
				typ = "REAL"
			case "clip_content":
				typ = "INTEGER"
			}
			if _, err := db.sql.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + typ); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) Meta(key string) (string, error) {
	var value string
	err := db.sql.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetMeta(key, value string) error {
	_, err := db.sql.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (db *DB) ClearEmbeddings() error {
	_, err := db.sql.Exec(`DELETE FROM embeddings`)
	return err
}

func (db *DB) EmbeddingCount() (int, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&n)
	return n, err
}
