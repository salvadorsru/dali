package index

import (
	"testing"

	"github.com/salvadorsru/dali/model"
)

func guidNode(session, local int32) map[string]any {
	return map[string]any{"sessionID": session, "localID": local}
}

func mkNode(local int32, typ, name string, parent map[string]any, position string) map[string]any {
	m := map[string]any{
		"guid":  guidNode(0, local),
		"phase": "CREATED",
		"type":  typ,
		"name":  name,
	}
	if parent != nil {
		m["parentIndex"] = map[string]any{"guid": parent, "position": position}
	}
	return m
}

func textNode(local int32, name, text string, parent map[string]any) map[string]any {
	n := mkNode(local, "TEXT", name, parent, "a")
	n["textData"] = map[string]any{"characters": text}
	return n
}

func buildDoc(texts map[int32]string) *model.Document {
	page := guidNode(0, 1)
	changes := []any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", guidNode(0, 0), "a"),
		mkNode(2, "FRAME", "Frame", page, "a"),
	}
	for local, text := range texts {
		changes = append(changes, textNode(local, "T"+text, text, guidNode(0, 2)))
	}
	return model.Build(changes, nil)
}

func count(t *testing.T, db *DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.SQL().QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count(%s): %v", query, err)
	}
	return n
}

func textHits(db *DB, term string) int {
	var n int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM text_search WHERE text_search MATCH ?`, term).Scan(&n); err != nil {
		return -1
	}
	return n
}

func TestWriteIncrementalDiff(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(db.Write(buildDoc(map[int32]string{3: "Hello"})))
	if n := count(t, db, `SELECT COUNT(*) FROM nodes`); n != 4 {
		t.Fatalf("initial nodes = %d, want 4", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM node_fingerprints`); n != 4 {
		t.Fatalf("initial fingerprints = %d, want 4", n)
	}
	if n := textHits(db, "Hello"); n != 1 {
		t.Fatalf("initial text hits = %d, want 1", n)
	}

	must(db.Write(buildDoc(map[int32]string{3: "Hello"})))
	if n := count(t, db, `SELECT COUNT(*) FROM nodes`); n != 4 {
		t.Fatalf("rewrite nodes = %d, want 4", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM node_fingerprints`); n != 4 {
		t.Fatalf("rewrite fingerprints = %d, want 4", n)
	}

	must(db.Write(buildDoc(map[int32]string{3: "Hello World"})))
	if n := textHits(db, "World"); n != 1 {
		t.Fatalf("edited text hits = %d, want 1", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM nodes`); n != 4 {
		t.Fatalf("edited nodes = %d, want 4", n)
	}

	must(db.Write(buildDoc(map[int32]string{3: "Hello World", 4: "Bye"})))
	if n := count(t, db, `SELECT COUNT(*) FROM nodes`); n != 5 {
		t.Fatalf("added nodes = %d, want 5", n)
	}
	if n := textHits(db, "Bye"); n != 1 {
		t.Fatalf("added text hits = %d, want 1", n)
	}

	must(db.Write(buildDoc(map[int32]string{3: "Hello World"})))
	if n := count(t, db, `SELECT COUNT(*) FROM nodes`); n != 4 {
		t.Fatalf("removed nodes = %d, want 4", n)
	}
	if n := textHits(db, "Bye"); n != 0 {
		t.Fatalf("removed text still indexed: %d hits", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM node_fingerprints`); n != 4 {
		t.Fatalf("removed fingerprints = %d, want 4", n)
	}
}
