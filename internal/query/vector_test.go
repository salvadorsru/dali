package query

import (
	"math"
	"strings"
	"testing"

	"github.com/salvadorsru/dali/internal/model"
)

// iconDoc is a component holding one 24x24 vector, placed once at 2x with its
// fill and stroke weight overridden:
//
//	DOCUMENT 0:0 "Doc"
//	└─ CANVAS 0:1 "Mobile"
//	   ├─ FRAME 0:2 "Screen"
//	   │  └─ INSTANCE 0:12 "Tab active" 48x48 -> 0:10
//	   └─ SYMBOL 0:10 "TabIcon" 24x24
//	      └─ VECTOR 0:11 "Icon" 24x24
func iconDoc() *model.Document {
	root := qGUID(0, 0)
	page := qGUID(0, 1)
	frame := qGUID(0, 2)
	comp := qGUID(0, 10)

	size := func(x, y float64) map[string]any {
		return map[string]any{"x": x, "y": y}
	}
	solid := func(r, g, b float64) map[string]any {
		return map[string]any{"type": "SOLID", "color": map[string]any{"r": r, "g": g, "b": b, "a": 1.0}}
	}

	icon := qNode(11, "VECTOR", "Icon", comp, "a")
	icon["size"] = size(24, 24)
	icon["fillPaints"] = []any{solid(0.56, 0.56, 0.56)}
	icon["strokePaints"] = []any{solid(0, 0, 0)}
	icon["strokeWeight"] = 1.5
	icon["fillGeometry"] = []any{map[string]any{
		"windingRule": "NONZERO", "commandsBlob": int32(0), "styleID": 0,
	}}

	symbol := qNode(10, "SYMBOL", "TabIcon", page, "b")
	symbol["size"] = size(24, 24)

	inst := qNode(12, "INSTANCE", "Tab active", frame, "a")
	inst["size"] = size(48, 48)
	inst["symbolData"] = map[string]any{
		"symbolID": qGUID(0, 10),
		"symbolOverrides": []any{map[string]any{
			"guidPath":     map[string]any{"guids": []any{qGUID(0, 11)}},
			"fillPaints":   []any{solid(0, 0.478, 1)},
			"strokeWeight": 3.0,
		}},
	}

	doc := model.Build([]any{
		qNode(0, "DOCUMENT", "Doc", nil, ""),
		qNode(1, "CANVAS", "Mobile", root, "a"),
		frameNode(2, "Screen", page, size(390, 844)),
		inst, symbol, icon,
	}, [][]byte{squareBlob()})
	return doc
}

func frameNode(local int32, name string, parent map[string]any, s map[string]any) map[string]any {
	n := qNode(local, "FRAME", name, parent, "a")
	n["size"] = s
	return n
}

func squareBlob() []byte {
	f32 := func(v float64) []byte {
		b := make([]byte, 4)
		v2 := math.Float32bits(float32(v))
		b[0], b[1], b[2], b[3] = byte(v2), byte(v2>>8), byte(v2>>16), byte(v2>>24)
		return b
	}
	var b []byte
	b = append(b, 0x01)
	b = append(b, f32(0)...)
	b = append(b, f32(0)...)
	b = append(b, 0x02)
	b = append(b, f32(24)...)
	b = append(b, f32(0)...)
	b = append(b, 0x02)
	b = append(b, f32(24)...)
	b = append(b, f32(24)...)
	b = append(b, 0x00)
	return b
}

func openIconService(t *testing.T) *Service {
	t.Helper()
	svc, err := openService(iconDoc())
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestVectorsResolvesPlacementToItsComponentBox(t *testing.T) {
	svc := openIconService(t)
	defer svc.Close()

	got, err := svc.Vectors("0:12")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Vectors = %d, want 1: %+v", len(got), got)
	}
	v := got[0]
	if v.Source != "0:11" {
		t.Fatalf("Source = %q, want 0:11", v.Source)
	}
	// The placement is 48x48 but the path lives in the component's 24x24 space,
	// so the box must come from the source node. Asserting the clone's own size
	// keeps this guard honest: if scaling ever stops, the test says so instead
	// of passing for the wrong reason.
	clone, err := svc.Node(v.ID)
	if err != nil || clone == nil {
		t.Fatalf("placement clone %s missing: %v", v.ID, err)
	}
	if clone.Width != 48 || clone.Height != 48 {
		t.Fatalf("clone size = %gx%g, want 48x48", clone.Width, clone.Height)
	}
	if v.ViewBox != "0 0 24 24" || v.Width != 24 || v.Height != 24 {
		t.Fatalf("box = %q %gx%g, want 0 0 24 24 24x24", v.ViewBox, v.Width, v.Height)
	}
	if v.Path != "M0 0 L24 0 L24 24 Z" {
		t.Fatalf("Path = %q", v.Path)
	}
}

func TestVectorsReportsLocalOverrides(t *testing.T) {
	svc := openIconService(t)
	defer svc.Close()

	got, err := svc.Vectors("0:12")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]Override{}
	for _, o := range got[0].Overrides {
		fields[o.Field] = o
	}
	if _, ok := fields["fills"]; !ok {
		t.Fatalf("fill override not reported: %+v", got[0].Overrides)
	}
	weight, ok := fields["stroke_weight"]
	if !ok {
		t.Fatalf("stroke weight override not reported: %+v", got[0].Overrides)
	}
	if weight.From != 1.5 || weight.To != 3.0 {
		t.Fatalf("stroke weight override = %v -> %v, want 1.5 -> 3", weight.From, weight.To)
	}
	// A stroke color override is not authored here, so it must not show up.
	if _, ok := fields["strokes"]; ok {
		t.Fatalf("unchanged strokes reported as overridden: %+v", got[0].Overrides)
	}
	if p := got[0].Paints; p == nil || solidColor(p.Fills) != "#007AFF" {
		t.Fatalf("effective fill = %+v, want #007AFF", p)
	}
}

// A component's raw payload holds the value as it was decoded, while deepCopyRaw
// flattens a placement's copy through JSON. The same authored number can therefore
// reach the diff as 1.600000023841858 on one side and 1.6 on the other, which must
// not be reported as an override the designer never made.
func TestVectorsIgnoresFloatWidthDifference(t *testing.T) {
	svc := openIconService(t)
	defer svc.Close()

	const cloneID = "inst:0:12.0:11"
	src, err := svc.Node("0:11")
	if err != nil || src == nil {
		t.Fatalf("source missing: %v", err)
	}
	clone, err := svc.Node(cloneID)
	if err != nil || clone == nil {
		t.Fatalf("clone missing: %v", err)
	}
	// Mirror what Build + deepCopyRaw produce for a value authored identically.
	src.PaintData.StrokeWeight = float64(float32(1.6))
	clone.PaintData.StrokeWeight = 1.6
	clone.PaintData.Fills = src.PaintData.Fills
	clone.PaintData.Strokes = src.PaintData.Strokes

	if got := diffPaints(src.PaintData, clone.PaintData); len(got) != 0 {
		t.Fatalf("phantom overrides reported: %+v", got)
	}
}

func TestVectorsOnComponentHasNoOverrides(t *testing.T) {
	svc := openIconService(t)
	defer svc.Close()

	got, err := svc.Vectors("0:10")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Vectors = %d, want 1: %+v", len(got), got)
	}
	if got[0].Source != "" || got[0].Overrides != nil {
		t.Fatalf("component vector should carry no source or overrides: %+v", got[0])
	}
}

func TestVectorSVG(t *testing.T) {
	svc := openIconService(t)
	defer svc.Close()

	got, err := svc.Vectors("0:12")
	if err != nil {
		t.Fatal(err)
	}
	svg := got[0].SVG()
	for _, want := range []string{`viewBox="0 0 24 24"`, `fill="#007AFF"`, `stroke="#000000"`, `stroke-width="3"`, `d="M0 0 L24 0 L24 24 Z"`} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %s: %s", want, svg)
		}
	}
}

func TestVectorSVGMarksMissingFillNone(t *testing.T) {
	v := Vector{Path: "M0 0 L1 1 Z", Width: 2, Height: 2, ViewBox: "0 0 2 2"}
	svg := v.SVG()
	// SVG would fill this black, which is not what a vector with no fill paints
	// means in Figma.
	if !strings.Contains(svg, `fill="none"`) {
		t.Fatalf("absent fill not stated: %s", svg)
	}
	if strings.Contains(svg, "stroke=") {
		t.Fatalf("invented a stroke: %s", svg)
	}
	if (Vector{}).SVG() != "" {
		t.Fatal("a vector with no path should render nothing")
	}
}
