package model

import "sort"

type Style struct {
	Kind   string
	Key    string
	NodeID string
}

var styleFields = []struct {
	field string
	kind  string
}{
	{"styleIdForFill", "FILL"},
	{"styleIdForStrokeFill", "STROKE"},
	{"styleIdForText", "TEXT"},
	{"styleIdForEffect", "EFFECT"},
	{"styleIdForGrid", "GRID"},
}

func (d *Document) StyleUsages() []Style {
	var out []Style
	for _, n := range d.order {
		for _, sf := range styleFields {
			ref := asMap(asMap(n.raw[sf.field])["assetRef"])
			if ref == nil {
				continue
			}
			key := asString(ref["key"])
			if key == "" {
				continue
			}
			out = append(out, Style{Kind: sf.kind, Key: key, NodeID: n.ID})
		}
	}
	return out
}

func (d *Document) Styles() []Style {
	seen := make(map[string]bool)
	var out []Style
	for _, s := range d.StyleUsages() {
		dedupe := s.Kind + "\x00" + s.Key
		if seen[dedupe] {
			continue
		}
		seen[dedupe] = true
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Key < out[j].Key
	})
	return out
}
