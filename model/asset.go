package model

import (
	"encoding/hex"
	"sort"
)

type ImageRef struct {
	Hash   string `json:"hash"`
	Name   string `json:"name"`
	NodeID string `json:"node_id"`
	MIME   string `json:"mime,omitempty"`
	Path   string `json:"path,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

func nodeImages(m map[string]any, nodeID string) []ImageRef {
	var out []ImageRef
	for _, key := range paintFields {
		for _, p := range asSlice(m[key]) {
			pm := asMap(p)
			if pm == nil {
				continue
			}
			img := asMap(pm["image"])
			if img == nil {
				continue
			}
			hash := hexBytes(img["hash"])
			if hash == "" {
				continue
			}
			out = append(out, ImageRef{Hash: hash, Name: asString(img["name"]), NodeID: nodeID})
		}
	}
	return out
}

var paintFields = []string{"fillPaints", "strokePaints", "backgroundPaints", "textDecorationFillPaints"}

func hexBytes(v any) string {
	b, ok := v.([]byte)
	if !ok || len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

func (d *Document) Assets() []ImageRef {
	seen := make(map[string]bool)
	var out []ImageRef
	for _, n := range d.order {
		for _, img := range n.Images {
			if seen[img.Hash] {
				continue
			}
			seen[img.Hash] = true
			out = append(out, img)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hash < out[j].Hash })
	return out
}
