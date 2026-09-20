package parser

import (
	"encoding/base64"
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
	if blobs := f.Blobs(); len(blobs) == 0 {
		t.Fatal("no blobs decoded from sample")
	}
}

func TestBlobsDecode(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	raw := []byte(`[{"bytes":"` + encoded + `"},{"bytes":"AQI="}]`)
	f := &File{Message: map[string]any{"blobs": raw}}
	blobs := f.Blobs()
	if len(blobs) != 2 {
		t.Fatalf("blobs = %d", len(blobs))
	}
	if string(blobs[0]) != string([]byte{1, 2, 3}) || string(blobs[1]) != "\x01\x02" {
		t.Fatalf("blobs = %x %x", blobs[0], blobs[1])
	}

	f = &File{Message: map[string]any{"blobs": []any{
		map[string]any{"bytes": []any{7, 8}},
	}}}
	if b := f.Blobs(); len(b) != 1 || string(b[0]) != "\x07\x08" {
		t.Fatalf("typed blobs = %x", b)
	}

	f = &File{Message: map[string]any{}}
	if b := f.Blobs(); b != nil {
		t.Fatalf("expected nil, got %v", b)
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
