package query

import (
	"database/sql"
	"fmt"
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
	doc := model.Build(f.NodeChanges())
	if err := db.Write(doc); err != nil {
		return false, err
	}
	if err := db.SetMeta("source", want); err != nil {
		return false, err
	}
	return true, nil
}

// ponytail: source key uses size+mtime, not content hash. Add sha256 if
// mtime-preserving edits become a problem.
func sourceKey(figPath string) (string, error) {
	fi, err := os.Stat(figPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d:%d", fi.Size(), fi.ModTime().Unix(), parser.Version, index.SchemaVersion), nil
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
