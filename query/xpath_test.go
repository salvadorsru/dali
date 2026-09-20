package query

import "testing"

func TestParseXPath(t *testing.T) {
	cases := []struct {
		in      string
		want    xpathSelector
		wantErr bool
	}{
		{
			in:   "//TEXT",
			want: xpathSelector{steps: []xpathStep{{descend: true, ntype: "TEXT"}}},
		},
		{
			in:   "/Page//FRAME",
			want: xpathSelector{steps: []xpathStep{{ntype: "PAGE"}, {descend: true, ntype: "FRAME"}}},
		},
		{
			in:   "//*",
			want: xpathSelector{steps: []xpathStep{{descend: true}}},
		},
		{
			in:   `//FRAME[@name='Hero']`,
			want: xpathSelector{steps: []xpathStep{{descend: true, ntype: "FRAME", nameOp: "=", name: "Hero"}}},
		},
		{
			in:   `//SECTION[contains(@name,"e-ro")]`,
			want: xpathSelector{steps: []xpathStep{{descend: true, ntype: "SECTION", nameOp: "contains", name: "e-ro"}}},
		},
		{
			in:      "TEXT",
			wantErr: true,
		},
		{
			in:      "//FRAME[@id='1:2']",
			wantErr: true,
		},
		{
			in:      "//",
			wantErr: true,
		},
		{
			in:      "//FRAME[",
			wantErr: true,
		},
		{
			in:      "//FRAME[text()='x']",
			wantErr: true,
		},
	}
	for _, c := range cases {
		got, err := parseXPath(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error %v", c.in, err)
		}
		if len(got.steps) != len(c.want.steps) {
			t.Fatalf("%q: steps = %+v, want %+v", c.in, got.steps, c.want.steps)
		}
		for i, ws := range c.want.steps {
			if got.steps[i] != ws {
				t.Fatalf("%q: step %d = %+v, want %+v", c.in, i, got.steps[i], ws)
			}
		}
	}
}

func TestSplitTokens(t *testing.T) {
	cases := map[string][]string{
		"PaymentScreen": {"payment", "screen"},
		"checkout  Button": {"checkout", "button"},
		"Hero":          {"hero"},
		"  ":            nil,
	}
	for in, want := range cases {
		got := splitTokens(in)
		if len(got) != len(want) {
			t.Fatalf("%q: got %v, want %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q: got %v, want %v", in, got, want)
			}
		}
	}
}