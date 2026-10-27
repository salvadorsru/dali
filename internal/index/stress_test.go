package index_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/salvadorsru/dali/internal/index"
	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/query"
	"github.com/salvadorsru/dali/internal/testdoc"
)

// scaleSpec yields roughly 100k nodes, exercising the pipeline at the size a
// real design-team file can reach. Skip quick runs with `go test -short`.
var scaleSpec = testdoc.Spec{
	Pages:            40,
	FramesPerPage:    200,
	TextsPerFrame:    6,
	VectorsPerFrame:  6,
	Components:       3,
	InstancesPerPage: 1,
	Seed:             42,
}

func openTempIndex(t *testing.T, doc *model.Document) *index.DB {
	t.Helper()
	db, err := index.Open(t.TempDir() + "/index.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}
	return db
}

func fingerprints(t *testing.T, db *index.DB) map[string]string {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT node_id, fingerprint FROM node_fingerprints ORDER BY node_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var id, fp string
		if err := rows.Scan(&id, &fp); err != nil {
			t.Fatal(err)
		}
		out[id] = fp
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStressScale(t *testing.T) {
	if testing.Short() {
		t.Skip("100k-node scale test skipped with -short")
	}
	doc := testdoc.Build(scaleSpec)
	if got := len(doc.All()); got < 100000 {
		t.Fatalf("scale doc too small: %d nodes", got)
	}
	if len(doc.Pages) != scaleSpec.Pages {
		t.Fatalf("pages = %d, want %d", len(doc.Pages), scaleSpec.Pages)
	}

	db := openTempIndex(t, doc)
	svc := query.Bind(db)
	if err := exerciseReads(t, svc, doc); err != nil {
		t.Fatal(err)
	}
}

// exerciseReads runs the full read surface over the index and cross-checks it
// against the materialized model it was built from.
func exerciseReads(t *testing.T, svc *query.Service, doc *model.Document) error {
	mc := doc.Counts()
	sc, err := svc.Counts()
	if err != nil {
		return err
	}
	for typ, want := range mc {
		if got := sc[string(typ)]; got != want {
			return fmt.Errorf("count[%s] = %d, want %d", typ, got, want)
		}
	}

	results, err := svc.Search("payment", 10)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("search payment matched nothing")
	}
	for _, r := range results {
		if r.ID == "" || r.Page == "" || r.Path == "" {
			return fmt.Errorf("search result incomplete: %+v", r)
		}
	}

	nodes, err := svc.Query("type=TEXT")
	if err != nil {
		return fmt.Errorf("query type=TEXT: %w", err)
	}
	if len(nodes) != mc[model.TypeText] {
		return fmt.Errorf("query type=TEXT = %d, want %d", len(nodes), mc[model.TypeText])
	}

	if nodes, err = svc.Query("type=FRAME"); err != nil {
		return fmt.Errorf("query type=FRAME: %w", err)
	}
	if len(nodes) != mc[model.TypeFrame] {
		return fmt.Errorf("query type=FRAME = %d, want %d", len(nodes), mc[model.TypeFrame])
	}

	if nodes, err = svc.XPath("//TEXT"); err != nil {
		return fmt.Errorf("xpath //TEXT: %w", err)
	}
	if len(nodes) != mc[model.TypeText] {
		return fmt.Errorf("xpath //TEXT = %d, want %d", len(nodes), mc[model.TypeText])
	}

	frame := firstType(doc, model.TypeFrame)
	if frame == nil {
		return fmt.Errorf("no frame in doc")
	}
	items, err := svc.Subtree(frame.ID, 0)
	if err != nil {
		return fmt.Errorf("subtree: %w", err)
	}
	if len(items) != len(doc.Children(frame.ID)) {
		return fmt.Errorf("subtree = %d, want %d", len(items), len(doc.Children(frame.ID)))
	}

	ctx, err := svc.Context(frame.ID)
	if err != nil || ctx == nil || ctx.Node == nil {
		return fmt.Errorf("context: %v", err)
	}
	if len(ctx.Path) < 3 {
		return fmt.Errorf("context path too short: %d", len(ctx.Path))
	}

	if _, err = svc.SuggestNames("zznomatch paymen", 5); err != nil {
		return fmt.Errorf("suggest: %w", err)
	}

	comp := firstType(doc, model.TypeSymbol)
	if comp == nil {
		return fmt.Errorf("no component in doc")
	}
	places, err := svc.Usages(comp.ID)
	if err != nil {
		return fmt.Errorf("usages: %w", err)
	}
	if len(places) == 0 {
		return fmt.Errorf("component %s has no placements", comp.ID)
	}
	for _, p := range places {
		if p.ComponentID != comp.ID {
			return fmt.Errorf("usage %s has component %q", p.ID, p.ComponentID)
		}
	}
	if _, err = svc.Dependents(comp.ID); err != nil {
		return fmt.Errorf("dependents: %w", err)
	}

	ranked, err := svc.UnifiedSearch(context.Background(), nil, query.UnifiedOptions{
		Query: "payment", Limit: 10, Mode: query.ModeText,
	})
	if err != nil {
		return fmt.Errorf("unified search: %w", err)
	}
	if len(ranked) == 0 {
		return fmt.Errorf("unified search matched nothing")
	}

	fonts, err := svc.Fonts()
	if err != nil || len(fonts) == 0 {
		return fmt.Errorf("fonts: %v", err)
	}
	vars, err := svc.Variables()
	if err != nil || len(vars) == 0 {
		return fmt.Errorf("variables: %v", err)
	}
	return nil
}

func firstType(doc *model.Document, typ model.NodeType) *model.Node {
	for _, n := range doc.All() {
		if n.Type == typ {
			return n
		}
	}
	return nil
}

func TestStressIncremental(t *testing.T) {
	spec := testdoc.Spec{
		Pages: 2, FramesPerPage: 40, TextsPerFrame: 3, VectorsPerFrame: 3,
		Components: 2, InstancesPerPage: 2, Seed: 1,
	}
	doc := testdoc.Build(spec)
	db, err := index.Open(t.TempDir() + "/index.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}

	first := fingerprints(t, db)

	// Reindexing the same document must reuse every node row.
	before := tableHash(t, db)
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}
	after := tableHash(t, db)
	if before != after {
		t.Fatalf("unchanged reindex rewrote rows (hash %s -> %s)", before, after)
	}
	if got, want := len(fingerprints(t, db)), len(first); got != want {
		t.Fatalf("fingerprint count %d -> %d", want, got)
	}

	// A single edited node must touch exactly one fingerprint.
	for _, n := range doc.All() {
		if n.Type == model.TypeText {
			n.Name = "Renamed"
			break
		}
	}
	if err := db.Write(doc); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for id, fp := range fingerprints(t, db) {
		if first[id] != fp {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("expected exactly 1 changed fingerprint after one edit, got %d", changed)
	}
}

// tableHash snapshots the observable nodes rows so a no-op reindex can be
// proven to leave them untouched.
func tableHash(t *testing.T, db *index.DB) string {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT id, parent_id, type, name, position, text, component_id FROM nodes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	h := ""
	for rows.Next() {
		var id, pid, typ, name, pos, text, comp string
		if err := rows.Scan(&id, &pid, &typ, &name, &pos, &text, &comp); err != nil {
			t.Fatal(err)
		}
		h += id + "|" + pid + "|" + typ + "|" + name + "|" + pos + "|" + text + "|" + comp + "\n"
	}
	return h
}

func benchSpec() testdoc.Spec {
	return testdoc.Spec{
		Pages: 10, FramesPerPage: 80, TextsPerFrame: 6, VectorsPerFrame: 6,
		Components: 3, InstancesPerPage: 1, Seed: 3,
	}
}

func BenchmarkBuild(b *testing.B) {
	spec := benchSpec()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc := testdoc.Build(spec)
		if len(doc.All()) == 0 {
			b.Fatal("empty doc")
		}
	}
}

func BenchmarkReindex(b *testing.B) {
	spec := benchSpec()
	doc := testdoc.Build(spec)
	db, err := index.Open(b.TempDir() + "/index.db")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := db.Write(doc); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Write(doc); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	spec := benchSpec()
	doc := testdoc.Build(spec)
	db, err := index.Open(b.TempDir() + "/index.db")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := db.Write(doc); err != nil {
		b.Fatal(err)
	}
	svc := query.Bind(db)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Search("payment", 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXPath(b *testing.B) {
	spec := benchSpec()
	doc := testdoc.Build(spec)
	db, err := index.Open(b.TempDir() + "/index.db")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := db.Write(doc); err != nil {
		b.Fatal(err)
	}
	svc := query.Bind(db)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.XPath("//TEXT"); err != nil {
			b.Fatal(err)
		}
	}
}
