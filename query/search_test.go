package query

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/salvadorsru/dali/index"
	"github.com/salvadorsru/dali/model"
)

func TestFTSMatch(t *testing.T) {
	cases := []struct {
		query   string
		strict  string
		relaxed string
	}{
		{"checkout button", "checkout* button*", "(checkout* OR button*)"},
		{`"complete the payment"`, `"complete the payment"`, `"complete the payment"`},
		{`"exact phrase" extra`, `"exact phrase" extra*`, `("exact phrase" OR extra*)`},
		{"check-out", "check* out*", "(check* OR out*)"},
		{"   ", "", ""},
	}
	for _, c := range cases {
		strict, relaxed := ftsMatch(c.query)
		if strict != c.strict || relaxed != c.relaxed {
			t.Fatalf("ftsMatch(%q) = (%q, %q), want (%q, %q)", c.query, strict, relaxed, c.strict, c.relaxed)
		}
	}
}

func TestSearchPrefixAndFallback(t *testing.T) {
	doc := buildTexts([]textNode{
		{name: "Checkout"},
		{name: "Checkouts"},
		{name: "Payment", text: "Complete your payment"},
	})
	db, err := index.Open(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}
	svc := Bind(db)

	// A bare token matches by prefix, so "checkout" also finds "Checkouts".
	got, err := svc.Search("checkout", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("prefix search matched %d, want 2", len(got))
	}

	// A strict AND miss falls back to OR so partial wording still answers.
	got, err = svc.Search("payment nonexistentword", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("relaxed OR fallback matched nothing")
	}
}

type textNode struct {
	name string
	text string
}

func buildTexts(nodes []textNode) *model.Document {
	changes := []any{
		change(1, 0, "DOCUMENT", "Document", ""),
		change(2, 1, "CANVAS", "Page 1", ""),
	}
	for i, n := range nodes {
		changes = append(changes, change(3+i, 2, "TEXT", n.name, n.text))
	}
	return model.Build(changes, nil)
}

func change(gid, parent int, typ, name, text string) map[string]any {
	m := map[string]any{
		"phase":     "CREATED",
		"guid":      map[string]any{"sessionID": int64(1), "localID": int64(gid)},
		"type":      typ,
		"name":      name,
		"visible":   true,
		"opacity":   1.0,
		"size":      map[string]any{"x": 10.0, "y": 10.0},
		"transform": map[string]any{"m00": 1.0, "m11": 1.0},
	}
	if parent > 0 {
		m["parentIndex"] = map[string]any{
			"guid":     map[string]any{"sessionID": int64(1), "localID": int64(parent)},
			"position": fmt.Sprintf("%05d", gid),
		}
	}
	if text != "" {
		m["textData"] = map[string]any{"characters": text}
	}
	return m
}
