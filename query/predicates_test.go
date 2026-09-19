package query

import "testing"

func TestInner(t *testing.T) {
	if got := inner(`descendants("Checkout")`, "descendants"); got != "Checkout" {
		t.Fatalf("inner = %q", got)
	}
	if got := inner(`children('Payment')`, "children"); got != "Payment" {
		t.Fatalf("inner = %q", got)
	}
}

func TestUnquote(t *testing.T) {
	if got := unquote(`"hi"`); got != "hi" {
		t.Fatalf("unquote = %q", got)
	}
	if got := unquote(`hi`); got != "hi" {
		t.Fatalf("unquote = %q", got)
	}
}

func TestFTSQuery(t *testing.T) {
	if got := ftsQuery("checkout button"); got != `"checkout" "button"` {
		t.Fatalf("ftsQuery = %q", got)
	}
	if got := ftsQuery(`a"b`); got != `"a""b"` {
		t.Fatalf("ftsQuery = %q", got)
	}
	if got := ftsQuery("   "); got != "" {
		t.Fatalf("ftsQuery = %q", got)
	}
}
