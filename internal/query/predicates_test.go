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
