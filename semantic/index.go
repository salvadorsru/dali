package semantic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/index"
)

type Stats struct {
	Total      int
	Indexed    int
	Reused     int
	Deleted    int
	Skipped    int
	Dimensions int
	Model      string
}

// Progress reports embedding progress while indexing.
type Progress func(done, total int)

// Index generates or refreshes embeddings for every node with semantic content.
func Index(ctx context.Context, db *index.DB, emb embed.Embedder) (Stats, error) {
	return IndexWithProgress(ctx, db, emb, nil)
}

// IndexWithProgress is Index with a progress callback.
func IndexWithProgress(ctx context.Context, db *index.DB, emb embed.Embedder, progress Progress) (Stats, error) {
	if !db.VecAvailable() {
		return Stats{}, errors.New("sqlite-vec is not available in this build")
	}
	stats := Stats{Model: emb.Model()}

	model, _ := db.Meta("embedding_model")
	gen, _ := db.Meta("embedding_generator_version")
	dim := ""

	// An empty stored model is a legacy/unknown identity: adopt the current one
	// without dropping valid vectors.
	if (model != "" && model != emb.Model()) || gen != strconv.Itoa(GeneratorVersion) {
		if err := db.DropVec(); err != nil {
			return stats, err
		}
		if err := db.ClearEmbeddings(); err != nil {
			return stats, err
		}
	} else if known, err := db.Meta("embedding_dimensions"); err == nil {
		dim = known
	}

	docs, skipped, err := candidates(db)
	if err != nil {
		return stats, err
	}
	stats.Total = len(docs)
	stats.Skipped = skipped

	existing, err := db.ExistingHashes()
	if err != nil {
		return stats, err
	}

	present := make(map[string]bool, len(docs))
	var changed []Doc
	for _, d := range docs {
		present[d.NodeID] = true
		if existing[d.NodeID] == d.Hash {
			stats.Reused++
			continue
		}
		changed = append(changed, d)
	}
	var removed []string
	for id := range existing {
		if !present[id] {
			removed = append(removed, id)
		}
	}

	if dim != "" {
		d, err := strconv.Atoi(dim)
		if err != nil {
			return stats, err
		}
		if err := db.CreateVec(d); err != nil {
			return stats, err
		}
		stats.Dimensions = d
	}

	if len(changed) > 0 {
		if err := embedAll(ctx, db, emb, changed, &stats, &dim, progress); err != nil {
			return stats, err
		}
	}

	if len(removed) > 0 {
		if err := db.DeleteVectors(removed); err != nil {
			return stats, err
		}
		if err := db.DeleteEmbeddings(removed); err != nil {
			return stats, err
		}
		stats.Deleted = len(removed)
	}

	if err := db.SetMeta("embedding_model", emb.Model()); err != nil {
		return stats, err
	}
	if err := db.SetMeta("embedding_dimensions", strconv.Itoa(stats.Dimensions)); err != nil {
		return stats, err
	}
	if err := db.SetMeta("embedding_generator_version", strconv.Itoa(GeneratorVersion)); err != nil {
		return stats, err
	}
	return stats, nil
}

// embedAll runs embedding requests concurrently (workers) and writes results
// through a single goroutine, because SQLite has a single writer. The first
// error cancels the whole pipeline.
func embedAll(ctx context.Context, db *index.DB, emb embed.Embedder, changed []Doc, stats *Stats, dim *string, progress Progress) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	size := embed.Batch()
	if size <= 0 {
		size = 64
	}
	var batches [][]Doc
	for start := 0; start < len(changed); start += size {
		end := start + size
		if end > len(changed) {
			end = len(changed)
		}
		batches = append(batches, changed[start:end])
	}

	workers := embed.Concurrency()
	if workers > len(batches) {
		workers = len(batches)
	}

	type result struct {
		docs    []Doc
		vectors [][]float32
		err     error
	}
	jobs := make(chan []Doc)
	results := make(chan result)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for docs := range jobs {
				texts := make([]string, len(docs))
				for j, d := range docs {
					texts[j] = d.Text
				}
				vectors, err := emb.Embed(ctx, texts)
				select {
				case results <- result{docs: docs, vectors: vectors, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, b := range batches {
			select {
			case jobs <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for res := range results {
		if firstErr != nil {
			continue
		}
		if res.err != nil {
			firstErr = res.err
			cancel()
			continue
		}
		if err := writeBatch(db, res.docs, res.vectors, emb.Model(), stats, dim); err != nil {
			firstErr = err
			cancel()
			continue
		}
		if progress != nil {
			progress(stats.Indexed, len(changed))
		}
	}
	if firstErr != nil {
		return firstErr
	}
	// A cancelled caller with no embedding error must surface as a cancellation,
	// not as a silent success that reports "embeddings up to date".
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func writeBatch(db *index.DB, docs []Doc, vectors [][]float32, model string, stats *Stats, dim *string) error {
	if len(vectors) != len(docs) {
		return fmt.Errorf("embedder returned %d vectors for %d texts", len(vectors), len(docs))
	}
	if len(vectors) > 0 && stats.Dimensions == 0 {
		stats.Dimensions = len(vectors[0])
	}
	if *dim == "" && stats.Dimensions > 0 {
		if err := db.CreateVec(stats.Dimensions); err != nil {
			return err
		}
		*dim = strconv.Itoa(stats.Dimensions)
	}
	vecs := make([]index.Vector, len(docs))
	entries := make([]index.Embedding, len(docs))
	for i, d := range docs {
		vecs[i] = index.Vector{NodeID: d.NodeID, Embedding: vectors[i]}
		entries[i] = index.Embedding{
			NodeID:           d.NodeID,
			Model:            model,
			Dimensions:       stats.Dimensions,
			ContentHash:      d.Hash,
			GeneratorVersion: GeneratorVersion,
		}
	}
	if err := db.WriteEmbeddings(vecs, entries); err != nil {
		return err
	}
	stats.Indexed += len(docs)
	return nil
}

// autoName matches Figma's auto-generated layer names. Nodes with such a name
// and no text carry no semantic signal, so they are not embedded.
var autoName = regexp.MustCompile(
	`^(Vector|Group|Frame|Rectangle|Rounded Rectangle|Ellipse|Line|Star|Polygon|` +
		`Regular Polygon|Boolean Operation|Union|Subtract|Intersect|Exclude|Slice|` +
		`Mask|Arrow|Image|Shape)( \d+)?$`)

func generic(name, text string) bool {
	if strings.TrimSpace(text) != "" {
		return false
	}
	return autoName.MatchString(strings.TrimSpace(name))
}

func candidates(db *index.DB) ([]Doc, int, error) {
	const q = `SELECT n.id, n.type, COALESCE(n.name, ''), COALESCE(n.text, ''),
			COALESCE(p.name, ''), COALESCE(par.name, '')
		FROM nodes n
		LEFT JOIN nodes par ON par.id = n.parent_id
		LEFT JOIN pages p ON p.id = n.page_id
		WHERE COALESCE(n.name, '') <> '' OR COALESCE(n.text, '') <> ''`
	rows, err := db.SQL().Query(q)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Doc
	skipped := 0
	for rows.Next() {
		var id, typ, name, text, page, parent string
		if err := rows.Scan(&id, &typ, &name, &text, &page, &parent); err != nil {
			return nil, 0, err
		}
		if generic(name, text) {
			skipped++
			continue
		}
		out = append(out, Build(id, typ, name, text, page, parent))
	}
	return out, skipped, rows.Err()
}
