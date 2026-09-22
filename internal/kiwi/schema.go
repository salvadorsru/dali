package kiwi

import "fmt"

type Kind uint8

const (
	Enum Kind = iota
	Struct
	Message
)

type Definition struct {
	Name   string
	Kind   Kind
	Fields []Field
}

type Field struct {
	Name    string
	Type    string
	IsArray bool
	Value   uint32

	rawType int32
}

type Schema struct {
	Definitions []Definition
	byName      map[string]int
}

var primitiveNames = []string{"bool", "byte", "int", "uint", "float", "string", "int64", "uint64"}

var kinds = []Kind{Enum, Struct, Message}

func DecodeSchema(data []byte) (*Schema, error) {
	bb := NewByteBuffer(data)
	count, err := bb.ReadVarUint()
	if err != nil {
		return nil, err
	}
	s := &Schema{Definitions: make([]Definition, 0, count)}
	for i := uint32(0); i < count; i++ {
		name, err := bb.readString()
		if err != nil {
			return nil, err
		}
		kindByte, err := bb.readByte()
		if err != nil {
			return nil, err
		}
		if int(kindByte) >= len(kinds) {
			return nil, fmt.Errorf("kiwi: invalid definition kind %d", kindByte)
		}
		fieldCount, err := bb.ReadVarUint()
		if err != nil {
			return nil, err
		}
		def := Definition{Name: name, Kind: kinds[kindByte], Fields: make([]Field, 0, fieldCount)}
		for j := uint32(0); j < fieldCount; j++ {
			fieldName, err := bb.readString()
			if err != nil {
				return nil, err
			}
			rawType, err := bb.readVarInt()
			if err != nil {
				return nil, err
			}
			arr, err := bb.readByte()
			if err != nil {
				return nil, err
			}
			value, err := bb.ReadVarUint()
			if err != nil {
				return nil, err
			}
			def.Fields = append(def.Fields, Field{
				Name:    fieldName,
				IsArray: arr&1 != 0,
				Value:   value,
				rawType: rawType,
			})
		}
		s.Definitions = append(s.Definitions, def)
	}

	s.byName = make(map[string]int, len(s.Definitions))
	for i, d := range s.Definitions {
		s.byName[d.Name] = i
	}

	for i := range s.Definitions {
		def := &s.Definitions[i]
		if def.Kind == Enum {
			continue
		}
		for j := range def.Fields {
			f := &def.Fields[j]
			if f.rawType < 0 {
				idx := ^f.rawType
				if int(idx) >= len(primitiveNames) {
					return nil, fmt.Errorf("kiwi: invalid primitive type %d", f.rawType)
				}
				f.Type = primitiveNames[idx]
			} else {
				if int(f.rawType) >= len(s.Definitions) {
					return nil, fmt.Errorf("kiwi: invalid type index %d", f.rawType)
				}
				f.Type = s.Definitions[f.rawType].Name
			}
		}
	}
	return s, nil
}

func (s *Schema) Definition(name string) (Definition, bool) {
	i, ok := s.byName[name]
	if !ok {
		return Definition{}, false
	}
	return s.Definitions[i], true
}
