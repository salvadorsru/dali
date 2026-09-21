package util

import (
	"strconv"
	"strings"
	"testing"

	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/model"
)

func mkNode(local int32, typ, name string, parent map[string]any, position string) map[string]any {
	m := map[string]any{
		"guid":  map[string]any{"sessionID": int32(0), "localID": local},
		"phase": "CREATED",
		"type":  typ,
		"name":  name,
	}
	if parent != nil {
		m["parentIndex"] = map[string]any{"guid": parent, "position": position}
	}
	return m
}

func g(session, local int32) map[string]any {
	return map[string]any{"sessionID": session, "localID": local}
}

func build(nodes ...map[string]any) *model.Document {
	items := make([]any, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, n)
	}
	return model.Build(items, nil)
}

func node(t *testing.T, local int32, typ, name string, props map[string]any) *model.Node {
	t.Helper()
	m := mkNode(local, typ, name, g(0, local-1), "a")
	delete(m, "parentIndex")
	for k, v := range props {
		m[k] = v
	}
	doc := build(mkNode(0, "DOCUMENT", "Doc", nil, ""), m)
	n, ok := doc.Node("0:" + strconv.Itoa(int(local)))
	if !ok {
		t.Fatalf("node %d not found", local)
	}
	return n
}

func TestSanitizeComponentName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Button", "Button"},
		{"Button/Primary", "ButtonPrimary"},
		{"1-Button", "Component1Button"},
		{"Icon/Social network/Facebook", "IconSocialNetworkFacebook"},
		{"", "Component"},
	}
	for _, tc := range tests {
		if got := SanitizeComponentName(tc.in); got != tc.want {
			t.Errorf("SanitizeComponentName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTranslateLayout(t *testing.T) {
	n := node(t, 2, "FRAME", "Row", map[string]any{
		"layoutMode":   "HORIZONTAL",
		"itemSpacing":  float64(8),
		"paddingLeft":  float64(16),
		"paddingRight": float64(16),
	})
	spec := TranslateLayout(n)
	if spec.Mode != "FLEX" || spec.Direction != "row" {
		t.Fatalf("mode/direction = %s/%s", spec.Mode, spec.Direction)
	}
	if spec.Gap != 8 {
		t.Errorf("gap = %g, want 8", spec.Gap)
	}
	if spec.Padding.Left != 16 || spec.Padding.Right != 16 {
		t.Errorf("padding = %+v", spec.Padding)
	}
}

func TestTranslateLayoutStatic(t *testing.T) {
	n := node(t, 2, "FRAME", "Static", nil)
	spec := TranslateLayout(n)
	if spec.Mode != "NONE" {
		t.Errorf("mode = %s, want NONE", spec.Mode)
	}
}

func TestTranslateStylesBackground(t *testing.T) {
	n := node(t, 2, "FRAME", "Card", map[string]any{
		"fillPaints": []any{map[string]any{
			"type":  "SOLID",
			"color": map[string]any{"r": float64(1), "g": float64(0), "b": float64(0), "a": float64(1)},
		}},
	})
	spec := TranslateStyles(n)
	if spec.Background.Color != "#FF0000" {
		t.Errorf("background = %q, want #FF0000", spec.Background.Color)
	}
	if len(spec.Fills) != 1 {
		t.Errorf("fills = %d, want 1", len(spec.Fills))
	}
}

func TestStyleToTailwindRotation(t *testing.T) {
	classes := StyleToTailwind(ir.StyleSpec{Rotation: 180})
	if !strings.Contains(classes, "rotate-[180deg]") {
		t.Errorf("classes = %q, want rotate-[180deg]", classes)
	}
}

func TestLayoutToTailwindHorizontal(t *testing.T) {
	classes := LayoutToTailwind(ir.LayoutSpec{Mode: "FLEX", Direction: "row", Gap: 8})
	if !strings.Contains(classes, "flex") {
		t.Errorf("classes = %q, want flex", classes)
	}
	if !strings.Contains(classes, "flex-row") {
		t.Errorf("classes = %q, want flex-row", classes)
	}
	if !strings.Contains(classes, "gap-[8px]") {
		t.Errorf("classes = %q, want gap-[8px]", classes)
	}
}

func TestLayoutToTailwindPerAxis(t *testing.T) {
	cases := []struct {
		name string
		spec ir.LayoutSpec
		want []string
	}{
		{"fixed-horizontal", ir.LayoutSpec{Mode: "FLEX", Direction: "row", PrimaryAxisSizing: "FIXED", Width: 120, CounterAxisSizing: "HUG"}, []string{"w-[120px]", "h-fit"}},
		{"fixed-vertical", ir.LayoutSpec{Mode: "FLEX", Direction: "column", PrimaryAxisSizing: "FIXED", Height: 80, CounterAxisSizing: "FILL"}, []string{"h-[80px]", "self-stretch"}},
		{"hug-horizontal", ir.LayoutSpec{Mode: "FLEX", Direction: "row", PrimaryAxisSizing: "HUG"}, []string{"w-fit"}},
		{"fill-horizontal", ir.LayoutSpec{Mode: "FLEX", Direction: "row", PrimaryAxisSizing: "FILL"}, []string{"flex-1"}},
	}
	for _, tc := range cases {
		classes := LayoutToTailwind(tc.spec)
		for _, want := range tc.want {
			if !strings.Contains(classes, want) {
				t.Errorf("%s: classes = %q, want contains %q", tc.name, classes, want)
			}
		}
	}
}

func TestTranslateStackSizing(t *testing.T) {
	n := node(t, 2, "FRAME", "FixedRow", map[string]any{
		"stackMode":          "HORIZONTAL",
		"stackPrimarySizing": "FIXED",
		"stackCounterSizing": "RESIZE_TO_FIT_WITH_IMPLICIT_SIZE",
		"size":               map[string]any{"x": float64(120), "y": float64(40)},
	})
	spec := TranslateLayout(n)
	if spec.PrimaryAxisSizing != "FIXED" || spec.CounterAxisSizing != "HUG" {
		t.Fatalf("sizing = %q/%q, want FIXED/HUG", spec.PrimaryAxisSizing, spec.CounterAxisSizing)
	}
	classes := LayoutToTailwind(spec)
	for _, want := range []string{"flex-row", "w-[120px]", "h-fit"} {
		if !strings.Contains(classes, want) {
			t.Errorf("classes = %q, want contains %q", classes, want)
		}
	}
}

func TestTranslateStackSizingAbsent(t *testing.T) {
	n := node(t, 2, "FRAME", "PlainRow", map[string]any{
		"stackMode": "HORIZONTAL",
	})
	spec := TranslateLayout(n)
	if spec.PrimaryAxisSizing != "" || spec.CounterAxisSizing != "" {
		t.Errorf("sizing = %q/%q, want empty", spec.PrimaryAxisSizing, spec.CounterAxisSizing)
	}
	if classes := LayoutToTailwind(spec); strings.Contains(classes, "self-stretch") {
		t.Errorf("classes = %q, want no self-stretch", classes)
	}
}

func TestTranslateMinSize(t *testing.T) {
	n := node(t, 2, "FRAME", "Constrained", map[string]any{
		"stackMode": "VERTICAL",
		"minSize":   map[string]any{"value": map[string]any{"x": float64(100), "y": float64(0)}},
	})
	spec := TranslateLayout(n)
	if spec.Constraints.MinWidth != 100 || spec.Constraints.MinHeight != 0 {
		t.Errorf("constraints = %+v, want MinWidth 100, MinHeight 0", spec.Constraints)
	}
}

func TestTranslateStackPadding(t *testing.T) {
	symmetric := node(t, 2, "FRAME", "Symmetric", map[string]any{
		"stackMode":              "VERTICAL",
		"stackVerticalPadding":   float64(12),
		"stackHorizontalPadding": float64(16),
	})
	spec := TranslateLayout(symmetric)
	want := ir.PaddingSpec{Top: 12, Right: 16, Bottom: 12, Left: 16}
	if spec.Padding != want {
		t.Errorf("symmetric padding = %+v, want %+v", spec.Padding, want)
	}

	overridden := node(t, 3, "FRAME", "Overridden", map[string]any{
		"stackMode":              "VERTICAL",
		"stackVerticalPadding":   float64(14),
		"stackPaddingBottom":     float64(0),
		"stackHorizontalPadding": float64(40),
		"stackPaddingRight":      float64(40),
	})
	spec = TranslateLayout(overridden)
	want = ir.PaddingSpec{Top: 14, Right: 40, Bottom: 0, Left: 40}
	if spec.Padding != want {
		t.Errorf("overridden padding = %+v, want %+v", spec.Padding, want)
	}
}

func TestNoQuotesInCSSOutput(t *testing.T) {
	spec := ir.TextSpec{FontFamily: "Hedvig Letters Serif"}
	if classes := TextToTailwind(spec); strings.Contains(classes, `"`) {
		t.Errorf("TextToTailwind embeds double quote in class attribute: %q", classes)
	}
	run := ir.TextRunSpec{FontFamily: "Syne", FontStyle: "Regular"}
	if styles := TextRunToTailwind(run); strings.Contains(styles, `"`) {
		t.Errorf("TextRunToTailwind embeds double quote in style attribute: %q", styles)
	}
}
