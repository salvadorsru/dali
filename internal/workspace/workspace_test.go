package workspace

import "testing"

func TestPaths(t *testing.T) {
	cases := []struct{ in, dir, db string }{
		{"/a/design.fig", "/a/.dali", "/a/.dali/design.fig.db"},
		{"design.fig", ".dali", ".dali/design.fig.db"},
		{"/a/b/deck.deck", "/a/b/.dali", "/a/b/.dali/deck.deck.db"},
	}
	for _, c := range cases {
		if got := Dir(c.in); got != c.dir {
			t.Fatalf("Dir(%q) = %q, want %q", c.in, got, c.dir)
		}
		if got := DB(c.in); got != c.db {
			t.Fatalf("DB(%q) = %q, want %q", c.in, got, c.db)
		}
	}
}
