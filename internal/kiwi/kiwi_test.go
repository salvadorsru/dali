package kiwi

import (
	"reflect"
	"testing"
)

func bytes(v ...int) []byte {
	out := make([]byte, len(v))
	for i, b := range v {
		out[i] = byte(b)
	}
	return out
}

func TestPrimitives(t *testing.T) {
	bb := NewByteBuffer(bytes(0xE5, 0x8E, 0x26)) // varuint 624485
	if v, _ := bb.ReadVarUint(); v != 624485 {
		t.Fatalf("varuint = %d, want 624485", v)
	}
	bb = NewByteBuffer(bytes(0x09))
	if v, _ := bb.readVarInt(); v != -5 {
		t.Fatalf("varint = %d, want -5", v)
	}
	bb = NewByteBuffer(bytes(0x7E, 0x00, 0x00, 0x00))
	if v, _ := bb.readVarFloat(); v != 0.5 {
		t.Fatalf("varfloat = %v, want 0.5", v)
	}
	bb = NewByteBuffer(bytes(0x00))
	if v, _ := bb.readVarFloat(); v != 0 {
		t.Fatalf("varfloat zero = %v, want 0", v)
	}
	bb = NewByteBuffer(bytes('h', 'i', 0))
	if v, _ := bb.readString(); v != "hi" {
		t.Fatalf("string = %q, want hi", v)
	}
}

var testSchema = bytes(
	3,
	'C', 'o', 'l', 'o', 'r', 0, 0, 2,
	'R', 'E', 'D', 0, 0, 0, 0,
	'G', 'R', 'E', 'E', 'N', 0, 0, 0, 1,
	'P', 'o', 'i', 'n', 't', 0, 1, 2,
	'x', 0, 9, 0, 1,
	'y', 0, 9, 0, 2,
	'T', 'h', 'i', 'n', 'g', 0, 2, 4,
	'n', 'a', 'm', 'e', 0, 11, 0, 1,
	'c', 'o', 'u', 'n', 't', 0, 7, 0, 2,
	'c', 'o', 'l', 'o', 'r', 0, 0, 0, 3,
	'p', 'o', 'i', 'n', 't', 's', 0, 2, 1, 4,
)

func TestDecodeSchemaAndMessage(t *testing.T) {
	schema, err := DecodeSchema(testSchema)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Definitions) != 3 {
		t.Fatalf("definitions = %d, want 3", len(schema.Definitions))
	}
	if d, _ := schema.Definition("Point"); d.Kind != Struct || len(d.Fields) != 2 {
		t.Fatalf("Point def wrong: %+v", d)
	}

	data := bytes(
		1, 'h', 'i', 0,
		2, 0x2a,
		3, 1,
		4, 2,
		0x7e, 0x00, 0x00, 0x00, 0x7f, 0x00, 0x00, 0x80,
		0x7e, 0x00, 0x00, 0x00, 0x7f, 0x00, 0x00, 0x80,
		0,
	)
	msg, err := schema.DecodeMessage("Thing", data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name":  "hi",
		"count": uint32(42),
		"color": "GREEN",
		"points": []any{
			map[string]any{"x": float32(0.5), "y": float32(1.5)},
			map[string]any{"x": float32(0.5), "y": float32(1.5)},
		},
	}
	if !reflect.DeepEqual(msg, want) {
		t.Fatalf("message = %#v, want %#v", msg, want)
	}
}
