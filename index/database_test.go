package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "index.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open in missing dir: %v", err)
	}
	defer db.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created: %v", err)
	}
}
