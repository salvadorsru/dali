package kiwi

import "fmt"

func (s *Schema) DecodeMessage(root string, data []byte) (map[string]any, error) {
	def, ok := s.Definition(root)
	if !ok {
		return nil, fmt.Errorf("kiwi: unknown root definition %q", root)
	}
	if def.Kind != Message {
		return nil, fmt.Errorf("kiwi: root definition %q is not a message", root)
	}
	v, err := s.decodeDefinition(def, NewByteBuffer(data))
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

func (s *Schema) decodeDefinition(def Definition, bb *ByteBuffer) (any, error) {
	switch def.Kind {
	case Enum:
		v, err := bb.ReadVarUint()
		if err != nil {
			return nil, err
		}
		return s.enumName(def, v), nil
	case Struct:
		out := make(map[string]any, len(def.Fields))
		for _, f := range def.Fields {
			v, err := s.decodeValue(f, bb)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", def.Name, f.Name, err)
			}
			out[f.Name] = v
		}
		return out, nil
	default:
		out := make(map[string]any)
		for {
			tag, err := bb.ReadVarUint()
			if err != nil {
				return nil, err
			}
			if tag == 0 {
				return out, nil
			}
			f, ok := fieldByTag(def, tag)
			if !ok {
				return nil, fmt.Errorf("kiwi: invalid tag %d in %s", tag, def.Name)
			}
			v, err := s.decodeValue(f, bb)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", def.Name, f.Name, err)
			}
			out[f.Name] = v
		}
	}
}

func (s *Schema) decodeValue(f Field, bb *ByteBuffer) (any, error) {
	if f.IsArray {
		if f.Type == "byte" {
			return bb.readByteArray()
		}
		n, err := bb.ReadVarUint()
		if err != nil {
			return nil, err
		}
		out := make([]any, n)
		for i := range out {
			v, err := s.decodeSingle(f.Type, bb)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	return s.decodeSingle(f.Type, bb)
}

func (s *Schema) decodeSingle(t string, bb *ByteBuffer) (any, error) {
	switch t {
	case "bool":
		c, err := bb.readByte()
		return c != 0, err
	case "byte":
		return bb.readByte()
	case "int":
		return bb.readVarInt()
	case "uint":
		return bb.ReadVarUint()
	case "float":
		return bb.readVarFloat()
	case "string":
		return bb.readString()
	case "int64":
		return bb.readVarInt64()
	case "uint64":
		return bb.readVarUint64()
	}
	def, ok := s.Definition(t)
	if !ok {
		return nil, fmt.Errorf("kiwi: unknown type %q", t)
	}
	return s.decodeDefinition(def, bb)
}

func (s *Schema) enumName(def Definition, value uint32) string {
	for _, f := range def.Fields {
		if f.Value == value {
			return f.Name
		}
	}
	return fmt.Sprintf("%s(%d)", def.Name, value)
}

func fieldByTag(def Definition, tag uint32) (Field, bool) {
	for _, f := range def.Fields {
		if f.Value == tag {
			return f, true
		}
	}
	return Field{}, false
}
