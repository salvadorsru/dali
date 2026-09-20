package index

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Vector struct {
	NodeID    string
	Embedding []float32
}

type Neighbor struct {
	NodeID   string
	Distance float64
}

type Embedding struct {
	NodeID           string
	Model            string
	Dimensions       int
	ContentHash      string
	GeneratorVersion int
}

func (db *DB) ExistingHashes() (map[string]string, error) {
	rows, err := db.sql.Query(`SELECT node_id, content_hash FROM embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		out[id] = hash
	}
	return out, rows.Err()
}

func (db *DB) SaveEmbeddings(entries []Embedding) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO embeddings
		(node_id, model, dimensions, content_hash, generator_version, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range entries {
		if _, err := stmt.Exec(e.NodeID, e.Model, e.Dimensions, e.ContentHash, e.GeneratorVersion); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// WriteEmbeddings stores vectors and their metadata in a single transaction.
func (db *DB) WriteEmbeddings(vectors []Vector, entries []Embedding) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	del, err := tx.Prepare(`DELETE FROM vec_nodes WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer del.Close()
	ins, err := tx.Prepare(`INSERT INTO vec_nodes(node_id, embedding) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	meta, err := tx.Prepare(`INSERT OR REPLACE INTO embeddings
		(node_id, model, dimensions, content_hash, generator_version, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())`)
	if err != nil {
		return err
	}
	defer meta.Close()
	for _, v := range vectors {
		if _, err := del.Exec(v.NodeID); err != nil {
			return err
		}
		if _, err := ins.Exec(v.NodeID, EncodeVector(v.Embedding)); err != nil {
			return err
		}
	}
	for _, e := range entries {
		if _, err := meta.Exec(e.NodeID, e.Model, e.Dimensions, e.ContentHash, e.GeneratorVersion); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) DeleteEmbeddings(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`DELETE FROM embeddings WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) VecVersion() (string, error) {
	var v string
	err := db.sql.QueryRow(`SELECT vec_version()`).Scan(&v)
	return v, err
}

func (db *DB) VecAvailable() bool {
	_, err := db.VecVersion()
	return err == nil
}

func (db *DB) CreateVec(dim int) error {
	_, err := db.sql.Exec(fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS vec_nodes USING vec0(node_id TEXT PRIMARY KEY, embedding float[%d] distance_metric=cosine)`,
		dim))
	return err
}

func (db *DB) DropVec() error {
	_, err := db.sql.Exec(`DROP TABLE IF EXISTS vec_nodes`)
	return err
}

func (db *DB) CountVectors() (int, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM vec_nodes`).Scan(&n)
	return n, err
}

func (db *DB) UpsertVectors(vectors []Vector) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	del, err := tx.Prepare(`DELETE FROM vec_nodes WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer del.Close()
	ins, err := tx.Prepare(`INSERT INTO vec_nodes(node_id, embedding) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, v := range vectors {
		if _, err := del.Exec(v.NodeID); err != nil {
			return err
		}
		if _, err := ins.Exec(v.NodeID, EncodeVector(v.Embedding)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) DeleteVectors(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`DELETE FROM vec_nodes WHERE node_id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// VecIndexed reports whether the vector index has been built.
func (db *DB) VecIndexed() bool {
	var ok int
	if err := db.sql.QueryRow(`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'vec_nodes'`).Scan(&ok); err != nil {
		return false
	}
	return true
}

func (db *DB) SearchVectors(query []float32, k int) ([]Neighbor, error) {
	if k <= 0 {
		k = 10
	}
	// Degrade gracefully when the vector index has not been built: callers
	// fall back to full-text and structural results.
	if !db.VecIndexed() {
		return nil, nil
	}
	rows, err := db.sql.Query(
		`SELECT node_id, distance FROM vec_nodes WHERE embedding MATCH ? AND k = ? ORDER BY distance`,
		EncodeVector(query), k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Neighbor
	for rows.Next() {
		var n Neighbor
		if err := rows.Scan(&n.NodeID, &n.Distance); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func EncodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
