package query

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/salvadorsru/dali/index"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
	"github.com/salvadorsru/dali/workspace"
)

type Service struct {
	db      *index.DB
	sql     *sql.DB
	rebuilt bool
	ranker  Ranker
}

func Open(figPath string) (*Service, error) {
	return OpenWith(figPath, false)
}

// OpenWith opens the index. When force is true the structural index is rebuilt
// even if the source file is unchanged.
func OpenWith(figPath string, force bool) (*Service, error) {
	return OpenNotify(figPath, force, nil)
}

// OpenNotify is OpenWith plus a callback invoked right before the structural
// index is rebuilt, so callers can report indexing progress.
func OpenNotify(figPath string, force bool, building func()) (*Service, error) {
	db, err := index.Open(workspace.DB(figPath))
	if err != nil {
		return nil, err
	}
	rebuilt, err := ensure(db, figPath, force, building)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Service{db: db, sql: db.SQL(), rebuilt: rebuilt, ranker: RRF{}}, nil
}

func (s *Service) Close() error { return s.db.Close() }

// Rebuilt reports whether the structural index was rebuilt by the last Open.
func (s *Service) Rebuilt() bool { return s.rebuilt }

// DB exposes the underlying index for indexing passes (embeddings).
func (s *Service) DB() *index.DB { return s.db }

func ensure(db *index.DB, figPath string, force bool, building func()) (bool, error) {
	want, err := sourceKey(figPath)
	if err != nil {
		return false, err
	}
	if !force {
		got, err := db.Meta("source")
		if err != nil {
			return false, err
		}
		if got == want {
			return false, nil
		}
	}
	if building != nil {
		building()
	}
	f, err := parser.Open(figPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	doc := model.Build(f.NodeChanges(), f.Blobs())
	if err := db.Write(doc); err != nil {
		return false, err
	}
	if err := db.SetMeta("source", want); err != nil {
		return false, err
	}
	return true, nil
}

// ponytail: source key blends size, mtime and a sampled content hash, so index
// reuse survives mtime-preserving edits (git checkout, touch) while staying
// cheap on large files (2 MB reads). Reading sampled blocks is best-effort: if
// it fails we fall back to size+mtime so indexing is never blocked.
func sourceKey(figPath string) (string, error) {
	fi, err := os.Stat(figPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%s:%d:%d", fi.Size(), fi.ModTime().Unix(), sampleHash(figPath, fi.Size()), parser.Version, index.SchemaVersion), nil
}

// sampleHash hashes the first and last 1 MB of a file. Small files are hashed
// whole (the two reads overlap, but the digest is stable).
func sampleHash(path string, size int64) string {
	const chunk = 1 << 20
	if size <= 0 {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	offsets := []int64{0}
	if tail := size - chunk; tail > 0 {
		offsets = append(offsets, tail)
	}
	for _, off := range offsets {
		n := int64(chunk)
		if off+n > size {
			n = size - off
		}
		if _, err := io.CopyN(h, io.NewSectionReader(f, off, n), n); err != nil {
			return ""
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *Service) Document() (string, error) {
	var name string
	err := s.sql.QueryRow(`SELECT name FROM documents LIMIT 1`).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}

func (s *Service) Counts() (map[string]int, error) {
	counts := make(map[string]int)
	rows, err := s.sql.Query(`SELECT type, COUNT(*) FROM nodes GROUP BY type ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		counts[t] = c
	}
	return counts, rows.Err()
}

func (s *Service) Pages() ([]*model.Node, error) {
	rows, err := s.sql.Query(`SELECT id, name FROM pages ORDER BY ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Node
	for rows.Next() {
		n := &model.Node{Type: model.TypeCanvas}
		if err := rows.Scan(&n.ID, &n.Name); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
