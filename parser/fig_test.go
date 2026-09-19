package parser

import (
	"os"
	"testing"
)

const sample = "../woments.fig"

func TestOpenSample(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	f, err := Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if len(f.Schema.Definitions) < 100 {
		t.Fatalf("definitions = %d", len(f.Schema.Definitions))
	}
	changes := f.NodeChanges()
	if len(changes) == 0 {
		t.Fatal("no nodeChanges decoded")
	}
}

func BenchmarkOpen(b *testing.B) {
	if _, err := os.Stat(sample); err != nil {
		b.Skip("sample fig not present")
	}
	for i := 0; i < b.N; i++ {
		f, err := Open(sample)
		if err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}
