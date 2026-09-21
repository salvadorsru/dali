package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/salvadorsru/dali/index"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Model() string { return "fake" }

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		h := sha256.Sum256([]byte(t))
		out[i] = []float32{float32(h[0]), float32(h[1]), float32(h[2]), float32(h[3])}
	}
	return out, nil
}

func insertNode(t *testing.T, db *index.DB, id, page, parent, typ, name, text string) {
	t.Helper()
	_, err := db.SQL().Exec(`INSERT INTO nodes
		(id, page_id, parent_id, type, name, position, visible, opacity, x, y, width, height, text, font_family, font_style, font_size, component_id)
		VALUES (?, ?, ?, ?, ?, '', 1, 1, 0, 0, 0, 0, ?, '', '', 0, '')`,
		id, page, parent, typ, name, text)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIndexIncremental(t *testing.T) {
	db, err := index.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if !db.VecAvailable() {
		t.Skip("sqlite-vec not available")
	}

	if _, err := db.SQL().Exec(`INSERT INTO pages (id, name, position, ordinal) VALUES ('p1', 'Checkout', '', 0)`); err != nil {
		t.Fatal(err)
	}
	insertNode(t, db, "n1", "p1", "", "FRAME", "Card", "")
	insertNode(t, db, "n2", "p1", "n1", "TEXT", "", "Buy now")
	insertNode(t, db, "n3", "p1", "n1", "VECTOR", "", "")

	ctx := context.Background()
	emb := fakeEmbedder{}

	stats, err := IndexWithProgress(ctx, db, emb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 || stats.Indexed != 2 || stats.Dimensions != 4 {
		t.Fatalf("first index = %+v", stats)
	}
	count, _ := db.CountVectors()
	if count != 2 {
		t.Fatalf("vectors = %d", count)
	}

	stats, err = IndexWithProgress(ctx, db, emb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Indexed != 0 || stats.Reused != 2 {
		t.Fatalf("second index = %+v", stats)
	}

	if _, err := db.SQL().Exec(`UPDATE nodes SET text = 'Buy now, fast' WHERE id = 'n2'`); err != nil {
		t.Fatal(err)
	}
	stats, err = IndexWithProgress(ctx, db, emb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Indexed != 1 || stats.Reused != 1 {
		t.Fatalf("after update = %+v", stats)
	}

	if _, err := db.SQL().Exec(`DELETE FROM nodes WHERE id = 'n2'`); err != nil {
		t.Fatal(err)
	}
	stats, err = IndexWithProgress(ctx, db, emb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Deleted != 1 || stats.Reused != 1 {
		t.Fatalf("after delete = %+v", stats)
	}
	count, _ = db.CountVectors()
	if count != 1 {
		t.Fatalf("vectors after delete = %d", count)
	}
}

// blockingEmbedder parks each embedding request until the context is cancelled,
// so a cancelled pipeline surfaces the cancellation instead of a silent success.
type blockingEmbedder struct {
	once    sync.Once
	entered chan struct{}
}

func (*blockingEmbedder) Model() string { return "block" }

func (b *blockingEmbedder) Embed(ctx context.Context, _ []string) ([][]float32, error) {
	b.once.Do(func() { close(b.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestIndexCancelled: a pre-cancelled context aborts indexing immediately
// instead of reporting an empty successful run.
func TestIndexCancelled(t *testing.T) {
	db, err := index.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if !db.VecAvailable() {
		t.Skip("sqlite-vec not available")
	}
	if _, err := db.SQL().Exec(`INSERT INTO pages (id, name, position, ordinal) VALUES ('p1', 'Checkout', '', 0)`); err != nil {
		t.Fatal(err)
	}
	insertNode(t, db, "n1", "p1", "", "FRAME", "Card", "")
	insertNode(t, db, "n2", "p1", "n1", "TEXT", "", "Buy now")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = IndexWithProgress(ctx, db, fakeEmbedder{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("index with cancelled ctx = %v, want context.Canceled", err)
	}
}

// TestIndexCancelsMidFlight: cancelling a running index stops the pipeline and
// returns promptly instead of leaving it grinding to completion.
func TestIndexCancelsMidFlight(t *testing.T) {
	db, err := index.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if !db.VecAvailable() {
		t.Skip("sqlite-vec not available")
	}
	if _, err := db.SQL().Exec(`INSERT INTO pages (id, name, position, ordinal) VALUES ('p1', 'Checkout', '', 0)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		insertNode(t, db, string(rune('a'+i)), "p1", "", "FRAME", "Card", "text")
	}

	emb := &blockingEmbedder{entered: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := IndexWithProgress(ctx, db, emb, nil)
		done <- err
	}()
	select {
	case <-emb.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("embedding never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("index after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("index hung after cancel")
	}
}
