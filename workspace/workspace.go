// Package workspace defines where Dalí writes the technical files it generates
// for a Figma file. They live under a single .dali directory next to the .fig.
// User-facing output (extracted assets, JSON exports) stays next to the .fig.
package workspace

import (
	"path/filepath"
)

// Dir is the directory holding the technical files generated for a fig file.
func Dir(figPath string) string {
	return filepath.Join(filepath.Dir(figPath), ".dali")
}

// DB is the SQLite index path for a fig file.
func DB(figPath string) string {
	return filepath.Join(Dir(figPath), filepath.Base(figPath)+".db")
}
