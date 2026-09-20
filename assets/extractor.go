package assets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/parser"
)

// Extract writes every referenced image to <base>.assets/images and returns the
// refs enriched with mime, path and dimensions. Reads and writes run in
// parallel across CPU cores.
func Extract(figPath string, refs []model.ImageRef) ([]model.ImageRef, error) {
	f, err := parser.Open(figPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dir := ImagesDir(figPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	out := make([]model.ImageRef, len(refs))
	ok := make([]bool, len(refs))

	progress := output.NewProgress("extracting", len(refs))
	defer progress.Done()

	workers := runtime.NumCPU()
	if workers > len(refs) {
		workers = len(refs)
	}
	if workers < 1 {
		return nil, nil
	}

	jobs := make(chan int, len(refs))
	for i := range refs {
		jobs <- i
	}
	close(jobs)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		skipped  int
	)
	fail := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				mu.Lock()
				stop := firstErr != nil
				mu.Unlock()
				if stop {
					return
				}
				ref := refs[i]
				data, err := f.ReadImage(ref.Hash)
				if errors.Is(err, parser.ErrEntryNotFound) {
					mu.Lock()
					skipped++
					mu.Unlock()
					progress.Add(1)
					continue
				}
				if err != nil {
					fail(fmt.Errorf("image %s: %w", ref.Hash, err))
					return
				}
				ref.MIME = http.DetectContentType(data)
				if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
					ref.Width, ref.Height = cfg.Width, cfg.Height
				}
				ref.Path = filepath.Join(dir, ref.Hash+Ext(ref.MIME))
				if err := os.WriteFile(ref.Path, data, 0o644); err != nil {
					fail(err)
					return
				}
				out[i] = ref
				ok[i] = true
				progress.Add(1)
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	result := make([]model.ImageRef, 0, len(refs))
	for i := range refs {
		if ok[i] {
			result = append(result, out[i])
		}
	}
	progress.Done()
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "dali: skipped %d image(s) not embedded in the file\n", skipped)
	}
	return result, nil
}
