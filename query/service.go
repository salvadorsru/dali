package query

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/salvadorsru/dali/index"
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
)

type Service struct {
	db  *index.DB
	sql *sql.DB
}

func Open(figPath string) (*Service, error) {
	db, err := index.Open(figPath + ".db")
	if err != nil {
		return nil, err
	}
	if err := ensure(db, figPath); err != nil {
		db.Close()
		return nil, err
	}
	return &Service{db: db, sql: db.SQL()}, nil
}

func (s *Service) Close() error { return s.db.Close() }

func ensure(db *index.DB, figPath string) error {
	want, err := sourceKey(figPath)
	if err != nil {
		return err
	}
	got, err := db.Meta("source")
	if err != nil {
		return err
	}
	if got == want {
		return nil
	}
	f, err := parser.Open(figPath)
	if err != nil {
		return err
	}
	defer f.Close()
	doc := model.Build(f.NodeChanges())
	if err := db.Write(doc); err != nil {
		return err
	}
	return db.SetMeta("source", want)
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
