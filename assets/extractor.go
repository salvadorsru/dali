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

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
)

// Extract writes every referenced image to <base>.assets/images and returns the
// refs enriched with mime, path and dimensions.
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

	out := make([]model.ImageRef, 0, len(refs))
	var skipped int
	for _, ref := range refs {
		data, err := f.ReadImage(ref.Hash)
		if errors.Is(err, parser.ErrEntryNotFound) {
			skipped++
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", ref.Hash, err)
		}
		ref.MIME = http.DetectContentType(data)
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			ref.Width, ref.Height = cfg.Width, cfg.Height
		}
		name := ref.Hash + Ext(ref.MIME)
		ref.Path = filepath.Join(dir, name)
		if err := os.WriteFile(ref.Path, data, 0o644); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "dali: skipped %d image(s) not embedded in the file\n", skipped)
	}
	return out, nil
}
