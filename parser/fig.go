package parser

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/salvadorsru/dali/kiwi"
)

// ErrEntryNotFound is returned when a zip entry is missing.
var ErrEntryNotFound = errors.New("fig: entry not found")

const (
	figMagic  = "fig-kiwi"
	canvasFig = "canvas.fig"
	metaJSON  = "meta.json"
	zstdMagic = 0xFD2FB528
)

// Version identifies parser behavior for incremental indexing.
const Version = 1

type Meta struct {
	Version   int    `json:"version"`
	App       string `json:"app"`
	CreatedAt string `json:"createdAt"`
}

type File struct {
	Path    string
	Meta    Meta
	Schema  *kiwi.Schema
	Message map[string]any
	zip     *zip.ReadCloser
}

func (f *File) Close() error {
	if f.zip == nil {
		return nil
	}
	return f.zip.Close()
}

func Open(path string) (*File, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open fig archive: %w", err)
	}
	f := &File{Path: path, zip: zr}

	canvas, err := readEntry(zr, canvasFig)
	if err != nil {
		zr.Close()
		return nil, err
	}
	if len(canvas) < 16 || string(canvas[:8]) != figMagic {
		zr.Close()
		return nil, fmt.Errorf("not a fig-kiwi file: bad prelude")
	}
	if meta, err := readEntry(zr, metaJSON); err == nil {
		_ = json.Unmarshal(meta, &f.Meta)
	}

	schemaRaw, off, err := readChunk(canvas, 12)
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("read schema chunk: %w", err)
	}
	messageRaw, _, err := readChunk(canvas, off)
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("read message chunk: %w", err)
	}

	schema, err := kiwi.DecodeSchema(schemaRaw)
	if err != nil {
		zr.Close()
		return nil, err
	}
	message, err := schema.DecodeMessage("Message", messageRaw)
	if err != nil {
		zr.Close()
		return nil, err
	}
	f.Schema = schema
	f.Message = message
	return f, nil
}

func (f *File) NodeChanges() []any {
	changes, _ := f.Message["nodeChanges"].([]any)
	return changes
}

// Blobs decodes the file's binary blobs (vector commands, network geometry)
// into an ordered slice usable by model.Build. The message may carry them as
// pre-decoded entries {"bytes": [ints...]}, or as raw JSON of the form
// [{"bytes":"<base64>"}, ...] when the payload is large.
func (f *File) Blobs() [][]byte {
	v := f.Message["blobs"]
	var entries []any
	switch raw := v.(type) {
	case nil:
		return nil
	case []any:
		entries = raw
	case []byte:
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil
		}
	case string:
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return nil
		}
	default:
		return nil
	}
	out := make([][]byte, 0, len(entries))
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		b, ok := m["bytes"]
		if !ok {
			continue
		}
		switch tb := b.(type) {
		case string:
			if dec, err := base64.StdEncoding.DecodeString(tb); err == nil {
				out = append(out, dec)
			}
		case []any:
			buf := make([]byte, len(tb))
			for i, c := range tb {
				buf[i], _ = float2byte(c)
			}
			out = append(out, buf)
		case []byte:
			out = append(out, tb)
		}
	}
	return out
}

func float2byte(v any) (byte, bool) {
	switch n := v.(type) {
	case int:
		return byte(n), true
	case int32:
		return byte(n), true
	case int64:
		return byte(n), true
	case uint32:
		return byte(n), true
	case uint64:
		return byte(n), true
	case float64:
		return byte(n), true
	case float32:
		return byte(n), true
	}
	return 0, false
}

func (f *File) ReadImage(hash string) ([]byte, error) {
	return readEntry(f.zip, "images/"+hash)
}

func readEntry(zr *zip.ReadCloser, name string) ([]byte, error) {
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("%w: %s", ErrEntryNotFound, name)
}

func readChunk(data []byte, off int) ([]byte, int, error) {
	if off+4 > len(data) {
		return nil, 0, io.ErrUnexpectedEOF
	}
	n := int(binary.LittleEndian.Uint32(data[off : off+4]))
	start := off + 4
	if start+n > len(data) {
		return nil, 0, io.ErrUnexpectedEOF
	}
	chunk := data[start : start+n]
	out, err := decompress(chunk)
	if err != nil {
		return nil, 0, err
	}
	return out, start + n, nil
}

func decompress(chunk []byte) ([]byte, error) {
	if len(chunk) >= 4 && binary.LittleEndian.Uint32(chunk[:4]) == zstdMagic {
		zr, err := zstd.NewReader(bytes.NewReader(chunk))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}
	fr := flate.NewReader(bytes.NewReader(chunk))
	defer fr.Close()
	return io.ReadAll(fr)
}
