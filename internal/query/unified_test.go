package query

import (
	"context"
	"testing"

	"github.com/salvadorsru/dali/internal/index"
	"github.com/salvadorsru/dali/internal/model"
)

func qGUID(session, local int32) map[string]any {
	return map[string]any{"sessionID": session, "localID": local}
}

func qNode(local int32, typ, name string, parent map[string]any, position string) map[string]any {
	m := map[string]any{
		"guid":  qGUID(0, local),
		"phase": "CREATED",
		"type":  typ,
		"name":  name,
	}
	if parent != nil {
		m["parentIndex"] = map[string]any{"guid": parent, "position": position}
	}
	return m
}

func qText(local int32, name, text string, parent map[string]any) map[string]any {
	n := qNode(local, "TEXT", name, parent, "a")
	n["textData"] = map[string]any{"characters": text}
	return n
}

func qDoc(texts map[int32]string) *model.Document {
	page := qGUID(0, 1)
	changes := []any{
		qNode(0, "DOCUMENT", "Doc", nil, ""),
		qNode(1, "CANVAS", "Page", qGUID(0, 0), "a"),
		qNode(2, "FRAME", "Frame", page, "a"),
	}
	for local, text := range texts {
		changes = append(changes, qText(local, text, text, qGUID(0, 2)))
	}
	return model.Build(changes, nil)
}

func openTestService(t *testing.T) *Service {
	t.Helper()
	db, err := index.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Write(qDoc(map[int32]string{3: "Checkout", 4: "Blue Button", 5: "Payment"})); err != nil {
		t.Fatal(err)
	}
	return &Service{db: db, sql: db.SQL()}
}

func hasText(results []Result, text string) bool {
	for _, r := range results {
		if r.Text == text {
			return true
		}
	}
	return false
}

func hasName(results []Result, name string) bool {
	for _, r := range results {
		if r.Name == name {
			return true
		}
	}
	return false
}

func TestUnifiedSearchText(t *testing.T) {
	svc := openTestService(t)
	defer svc.Close()

	res, err := svc.UnifiedSearch(context.Background(), nil, UnifiedOptions{Query: "Button", Limit: 10, Mode: ModeText})
	if err != nil {
		t.Fatal(err)
	}
	if !hasText(res, "Blue Button") {
		t.Fatalf("results = %+v, want text %q", res, "Blue Button")
	}
}

func TestUnifiedSearchStructuralAuto(t *testing.T) {
	svc := openTestService(t)
	defer svc.Close()

	res, err := svc.UnifiedSearch(context.Background(), nil, UnifiedOptions{Query: `descendants("Page")`, Limit: 10, Mode: ModeText})
	if err != nil {
		t.Fatal(err)
	}
	if !hasName(res, "Frame") {
		t.Fatalf("results = %+v, want Frame", res)
	}
}

func TestUnifiedSearchStructuralFallback(t *testing.T) {
	svc := openTestService(t)
	defer svc.Close()

	res, err := svc.UnifiedSearch(context.Background(), nil, UnifiedOptions{Query: "noise)", Limit: 10, Mode: ModeText})
	if err != nil {
		t.Fatalf("accidental structural text must not fail the search: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected no results for unparsable query, got %+v", res)
	}
}

func TestUnifiedSearchStructuralExplicitError(t *testing.T) {
	svc := openTestService(t)
	defer svc.Close()

	if _, err := svc.UnifiedSearch(context.Background(), nil, UnifiedOptions{Query: "noise", Structural: true, Limit: 10, Mode: ModeText}); err == nil {
		t.Fatal("explicit structural query must surface parse errors")
	}
}

func TestRRF(t *testing.T) {
	groups := [][]Result{
		{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		{{ID: "b"}, {ID: "d"}},
	}
	ranked := rank(groups...)
	if len(ranked) != 4 {
		t.Fatalf("ranked = %d results, want 4", len(ranked))
	}
	if ranked[0].ID != "b" {
		t.Fatalf("top result = %q, want b (present in both groups)", ranked[0].ID)
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i].Score > ranked[i-1].Score {
			t.Fatalf("ranked not descending: %+v", ranked)
		}
	}
	byID := map[string]Result{}
	for _, r := range ranked {
		byID[r.ID] = r
	}
	if byID["a"].Score <= byID["d"].Score {
		t.Fatalf("rank1 in one group (a) should outrank rank2 of another (d): %+v", ranked)
	}
}
