package kiwi

import (
	"errors"
	"math"
)

var errOutOfBounds = errors.New("kiwi: read out of bounds")

type ByteBuffer struct {
	data []byte
	pos  int
}

func NewByteBuffer(data []byte) *ByteBuffer {
	return &ByteBuffer{data: data}
}

func (b *ByteBuffer) readByte() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, errOutOfBounds
	}
	v := b.data[b.pos]
	b.pos++
	return v, nil
}

func (b *ByteBuffer) ReadVarUint() (uint32, error) {
	var value uint32
	var shift uint
	for {
		c, err := b.readByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(c&127) << shift
		shift += 7
		if c&128 == 0 || shift >= 35 {
			return value, nil
		}
	}
}

func (b *ByteBuffer) readVarInt() (int32, error) {
	v, err := b.ReadVarUint()
	if err != nil {
		return 0, err
	}
	if v&1 != 0 {
		return ^int32(v >> 1), nil
	}
	return int32(v >> 1), nil
}

func (b *ByteBuffer) readVarUint64() (uint64, error) {
	var value uint64
	var shift uint
	for {
		c, err := b.readByte()
		if err != nil {
			return 0, err
		}
		value |= uint64(c&127) << shift
		shift += 7
		if c&128 == 0 || shift >= 63 {
			return value, nil
		}
	}
}

func (b *ByteBuffer) readVarInt64() (int64, error) {
	v, err := b.readVarUint64()
	if err != nil {
		return 0, err
	}
	if v&1 != 0 {
		return ^int64(v >> 1), nil
	}
	return int64(v >> 1), nil
}

func (b *ByteBuffer) readVarFloat() (float32, error) {
	first, err := b.readByte()
	if err != nil {
		return 0, err
	}
	if first == 0 {
		return 0, nil
	}
	b2, err := b.readByte()
	if err != nil {
		return 0, err
	}
	b3, err := b.readByte()
	if err != nil {
		return 0, err
	}
	b4, err := b.readByte()
	if err != nil {
		return 0, err
	}
	bits := uint32(first) | uint32(b2)<<8 | uint32(b3)<<16 | uint32(b4)<<24
	bits = bits<<23 | bits>>9
	return math.Float32frombits(bits), nil
}

func (b *ByteBuffer) readString() (string, error) {
	start := b.pos
	for {
		c, err := b.readByte()
		if err != nil {
			return "", err
		}
		if c == 0 {
			return string(b.data[start : b.pos-1]), nil
		}
	}
}

func (b *ByteBuffer) readByteArray() ([]byte, error) {
	n, err := b.ReadVarUint()
	if err != nil {
		return nil, err
	}
	if b.pos+int(n) > len(b.data) {
		return nil, errOutOfBounds
	}
	out := make([]byte, n)
	copy(out, b.data[b.pos:b.pos+int(n)])
	b.pos += int(n)
	return out, nil
}
