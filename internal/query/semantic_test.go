package query

import (
	"testing"

	"github.com/salvadorsru/dali/internal/model"
)

func TestRank(t *testing.T) {
	text := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	semantic := []Result{{ID: "c"}, {ID: "d"}}
	out := rank(text, semantic)
	if len(out) != 4 {
		t.Fatalf("ranked results = %d, want 4", len(out))
	}
	if out[0].ID != "c" {
		t.Fatalf("top result = %q, want c (appears in both lists)", out[0].ID)
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].Score < out[i].Score {
			t.Fatalf("results not sorted: %+v", out)
		}
	}
}

func TestMatchFilter(t *testing.T) {
	n := &model.Node{Type: model.TypeSymbol, PageID: "p1"}
	if !matchFilter(n, Options{}) {
		t.Fatal("empty filter should match")
	}
	if !matchFilter(n, Options{Type: "symbol"}) {
		t.Fatal("type filter should be case-insensitive")
	}
	if matchFilter(n, Options{Type: "TEXT"}) {
		t.Fatal("type filter should reject mismatch")
	}
	if !matchFilter(n, Options{PageID: "p1"}) {
		t.Fatal("page filter should match")
	}
	if matchFilter(n, Options{PageID: "p2"}) {
		t.Fatal("page filter should reject mismatch")
	}
}
