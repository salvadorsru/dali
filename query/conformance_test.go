package query

import (
	"path/filepath"
	"testing"

	"github.com/salvadorsru/dali/index"
	"github.com/salvadorsru/dali/internal/testdoc"
)

// TestConformanceModelIndex pins the two read paths to the same document: the
// in-memory model (parser.Open + model.Build, used by quality) and the SQLite
// index (query.Service). They are separate pipelines on purpose — quality
// stays fresh, query stays indexed — so this test is what keeps them from
// silently drifting apart.
func TestConformanceModelIndex(t *testing.T) {
	spec := testdoc.Spec{
		Pages: 3, FramesPerPage: 12, TextsPerFrame: 2, VectorsPerFrame: 2,
		Components: 2, InstancesPerPage: 2, Seed: 9,
	}
	doc := testdoc.Build(spec)

	db, err := index.Open(filepath.Join(t.TempDir(), "conformance.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}
	svc := Bind(db)

	mc := doc.Counts()
	sc, err := svc.Counts()
	if err != nil {
		t.Fatal(err)
	}
	for typ, want := range mc {
		if got := sc[string(typ)]; got != want {
			t.Fatalf("count[%s]: model=%d index=%d", typ, want, got)
		}
	}

	sp, err := svc.Pages()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Pages) != len(sp) {
		t.Fatalf("pages: model=%d index=%d", len(doc.Pages), len(sp))
	}
	for i := range doc.Pages {
		if doc.Pages[i].ID != sp[i].ID || doc.Pages[i].Name != sp[i].Name {
			t.Fatalf("page[%d]: model=%s/%s index=%s/%s",
				i, doc.Pages[i].ID, doc.Pages[i].Name, sp[i].ID, sp[i].Name)
		}
	}

	scomp, err := svc.Components()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Components()) != len(scomp) {
		t.Fatalf("components: model=%d index=%d", len(doc.Components()), len(scomp))
	}
	sinst, err := svc.Instances()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Instances()) != len(sinst) {
		t.Fatalf("instances: model=%d index=%d", len(doc.Instances()), len(sinst))
	}

	svars, err := svc.Variables()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Variables) != len(svars) {
		t.Fatalf("variables: model=%d index=%d", len(doc.Variables), len(svars))
	}

	sassets, err := svc.Assets()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Assets()) != len(sassets) {
		t.Fatalf("assets: model=%d index=%d", len(doc.Assets()), len(sassets))
	}

	for i, n := range doc.All() {
		if i%37 != 0 {
			continue
		}
		got, err := svc.Node(n.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("node %s missing from index", n.ID)
		}
		if got.Type != n.Type || got.Name != n.Name || got.ParentID != n.ParentID ||
			got.PageID != n.PageID || got.Text != n.Text {
			t.Fatalf("node %s drifted: model=%+v index=%+v", n.ID, n, got)
		}
	}

	rows, err := db.SQL().Query(`SELECT id FROM nodes`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, ok := doc.Node(id); !ok {
			t.Fatalf("phantom node %s in index", id)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
