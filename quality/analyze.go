package quality

import (
	"fmt"
	"sort"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// AnalyzeOptions tune the geometric analysis.
type AnalyzeOptions struct {
	MinRatio float64 // overlaps: minimum overlap ratio of the smaller box
	MinCount int     // clusters: minimum repeats to count as a cluster
	Limit    int
}

// GapStat is a layout gap value and how many frames use it.
type GapStat struct {
	Value float64 `json:"value"`
	Count int     `json:"count"`
}

// Cluster is a repeated node size in a page.
type Cluster struct {
	Type   string  `json:"type"`
	Page   string  `json:"page,omitempty"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Count  int     `json:"count"`
}

// Overlap is a pair of sibling nodes whose boxes intersect.
type Overlap struct {
	A     string  `json:"a"`
	B     string  `json:"b"`
	Page  string  `json:"page,omitempty"`
	Ratio float64 `json:"ratio"`
}

// Analyze inspects the document geometry. kind is "spacing", "clusters" or
// "overlaps". Color and typography aggregation already lives in query and is
// not duplicated here.
func Analyze(doc *model.Document, kind string, opts AnalyzeOptions) (any, error) {
	if opts.MinRatio <= 0 || opts.MinRatio > 1 {
		opts.MinRatio = 0.1
	}
	if opts.MinCount < 2 {
		opts.MinCount = 3
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "spacing":
		return gapStats(doc, opts), nil
	case "clusters":
		return clusterStats(doc, opts), nil
	case "overlaps":
		return overlapStats(doc, opts), nil
	}
	return nil, fmt.Errorf("unknown analyze kind %q (spacing, clusters, overlaps)", kind)
}

func gapStats(doc *model.Document, opts AnalyzeOptions) []GapStat {
	counts := make(map[float64]int)
	for _, n := range doc.All() {
		if n.Type == model.TypeText {
			continue
		}
		info := n.Layout()
		if len(info) == 0 || num(info, "itemSpacing") <= 0 {
			continue
		}
		counts[num(info, "itemSpacing")]++
	}
	return rankGaps(counts, opts.Limit)
}

func rankGaps(counts map[float64]int, limit int) []GapStat {
	out := make([]GapStat, 0, len(counts))
	for value, count := range counts {
		out = append(out, GapStat{Value: value, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func clusterStats(doc *model.Document, opts AnalyzeOptions) []Cluster {
	type key struct {
		t, page string
		w, h    float64
	}
	counts := make(map[key]int)
	for _, n := range doc.All() {
		switch n.Type {
		case model.TypeFrame, model.TypeGroup, model.TypeInstance, model.TypeSymbol, model.TypeText, model.TypeVector:
		default:
			continue
		}
		if n.Width <= 0 || n.Height <= 0 {
			continue
		}
		counts[key{string(n.Type), n.PageID, n.Width, n.Height}]++
	}
	out := make([]Cluster, 0, len(counts))
	for k, c := range counts {
		if c >= opts.MinCount {
			out = append(out, Cluster{Type: k.t, Page: k.page, Width: k.w, Height: k.h, Count: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Type < out[j].Type
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out
}

func overlapStats(doc *model.Document, opts AnalyzeOptions) []Overlap {
	pageName := func(id string) string {
		p, ok := doc.Node(id)
		if !ok || p == nil {
			return ""
		}
		return p.Name
	}
	var out []Overlap
	seen := make(map[string]bool)
	for _, parent := range doc.All() {
		kids := doc.Children(parent.ID)
		for i := 0; i < len(kids); i++ {
			for j := i + 1; j < len(kids); j++ {
				a, b := kids[i], kids[j]
				if !a.Visible || !b.Visible {
					continue
				}
				ratio := boxOverlap(a, b)
				if ratio < opts.MinRatio {
					continue
				}
				ids := a.ID + "\x00" + b.ID
				if seen[ids] {
					continue
				}
				seen[ids] = true
				out = append(out, Overlap{A: a.ID, B: b.ID, Page: pageName(parent.ID), Ratio: ratio})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ratio != out[j].Ratio {
			return out[i].Ratio > out[j].Ratio
		}
		return out[i].A < out[j].A
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out
}

func boxOverlap(a, b *model.Node) float64 {
	ax1, ay1 := a.X, a.Y
	ax2, ay2 := a.X+a.Width, a.Y+a.Height
	bx1, by1 := b.X, b.Y
	bx2, by2 := b.X+b.Width, b.Y+b.Height
	w := min(ax2, bx2) - max(ax1, bx1)
	h := min(ay2, by2) - max(ay1, by1)
	if w <= 0 || h <= 0 {
		return 0
	}
	overlap := w * h
	smaller := min(a.Width*a.Height, b.Width*b.Height)
	if smaller <= 0 {
		return 0
	}
	return overlap / smaller
}

func num(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case uint:
		return float64(v)
	}
	return 0
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
