package model

import "sort"

type Font struct {
	Family string `json:"family"`
	Style  string `json:"style"`
	Count  int    `json:"count"`
}

func (d *Document) Fonts() []Font {
	index := make(map[Font]*Font)
	for _, n := range d.order {
		if n.Type != TypeText || n.FontFamily == "" {
			continue
		}
		key := Font{Family: n.FontFamily, Style: n.FontStyle}
		if f, ok := index[key]; ok {
			f.Count++
			continue
		}
		entry := key
		entry.Count = 1
		index[key] = &entry
	}
	out := make([]Font, 0, len(index))
	for _, f := range index {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].Style < out[j].Style
	})
	return out
}
