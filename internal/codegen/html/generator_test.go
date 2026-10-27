package html

import (
	"strings"
	"testing"

	"github.com/salvadorsru/dali/internal/codegen/ir"
)

func testGen(spec ir.ComponentSpec) string {
	out, err := (&HTMLGenerator{}).GenerateComponent(spec, ir.TokenSet{}, ir.GeneratorConfig{})
	if err != nil {
		panic(err)
	}
	return out
}

func wrapPure(children ...ir.NodeSpec) ir.NodeSpec {
	return ir.NodeSpec{Type: "FRAME", Name: "wrapper", Children: children}
}

func TestGenerateVectorViewBox(t *testing.T) {
	vec := ir.NodeSpec{Type: "VECTOR", Name: "Icon", Vector: ir.VectorSpec{Path: "M0 0 L13.5 0 Z", Width: 13.5, Height: 13.5}}
	spec := ir.ComponentSpec{
		ID:       "1:1",
		Name:     "icon",
		Type:     "FRAME",
		Children: []ir.NodeSpec{{Type: "FRAME", Name: "root", Children: []ir.NodeSpec{vec}}},
	}
	out := testGen(spec)
	if !strings.Contains(out, `viewBox="0 0 13.5 13.5"`) {
		t.Fatalf("expected the node size as viewBox: %s", out)
	}
	if strings.Contains(out, "0 0 100 100") {
		t.Fatalf("fixed 100x100 viewBox leaked back: %s", out)
	}

	// A vector with no measured size has no box to declare; the coordinates
	// then stand as user units instead of being squeezed into a wrong one.
	unsized := vec
	unsized.Vector.Width, unsized.Vector.Height = 0, 0
	out = testGen(ir.ComponentSpec{
		ID: "1:2", Name: "icon", Type: "FRAME",
		Children: []ir.NodeSpec{{Type: "FRAME", Name: "root", Children: []ir.NodeSpec{unsized}}},
	})
	if strings.Contains(out, "viewBox") {
		t.Fatalf("expected no viewBox for an unsized vector: %s", out)
	}
}

func TestGenerateElidesEmptyWrapper(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:   "1:1",
		Name: "empty",
		Type: "FRAME",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root", Children: []ir.NodeSpec{
				wrapPure(),
				{Type: "TEXT", Name: "label", Text: ir.TextSpec{Content: "hello"}},
			},
		}},
	}
	out := testGen(spec)
	if n := strings.Count(out, "<div"); n != 1 {
		t.Fatalf("expected just the root div, got %d: %s", n, out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing text: %s", out)
	}
}

func TestGenerateCollapsesSingleChildWrapper(t *testing.T) {
	inner := ir.NodeSpec{Type: "FRAME", Name: "inner", Children: []ir.NodeSpec{
		{Type: "TEXT", Name: "label", Text: ir.TextSpec{Content: "hello"}},
	}}
	spec := ir.ComponentSpec{
		ID:   "1:2",
		Name: "one",
		Type: "FRAME",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root", Children: []ir.NodeSpec{
				wrapPure(inner),
			},
		}},
	}
	out := testGen(spec)
	if n := strings.Count(out, "<div"); n != 1 {
		t.Fatalf("expected wrapper chain collapsed to just the root div, got %d divs: %s", n, out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing collapsed text: %s", out)
	}
}

func TestGenerateFlattensMultiChildWrapper(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:   "1:3",
		Name: "flat",
		Type: "FRAME",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root", Children: []ir.NodeSpec{
				wrapPure(
					ir.NodeSpec{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}},
					ir.NodeSpec{Type: "TEXT", Name: "b", Text: ir.TextSpec{Content: "B"}},
				),
			},
		}},
	}
	out := testGen(spec)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("expected both children rendered: %s", out)
	}
	if n := strings.Count(out, "<div"); n != 1 {
		t.Fatalf("expected only the root div, got %d: %s", n, out)
	}
}

func TestGenerateKeepsFlexItemWrapper(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:   "1:4",
		Name: "flexy",
		Type: "FRAME",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root",
			Layout: ir.LayoutSpec{Mode: "VERTICAL"},
			Children: []ir.NodeSpec{
				wrapPure(ir.NodeSpec{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}}),
			},
		}},
	}
	out := testGen(spec)
	if n := strings.Count(out, "<div"); n != 2 {
		t.Fatalf("expected root + flex item wrapper kept, got %d divs: %s", n, out)
	}
}

func TestGenerateKeepsStyledWrapper(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:   "1:5",
		Name: "filled",
		Type: "FRAME",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root", Children: []ir.NodeSpec{
				{
					Type: "FRAME", Name: "wrapper",
					Styles: ir.StyleSpec{Fills: []ir.FillSpec{{Type: "SOLID", Color: "#F00"}}},
					Children: []ir.NodeSpec{
						{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}},
					},
				},
			},
		}},
	}
	out := testGen(spec)
	if n := strings.Count(out, "<div"); n != 2 {
		t.Fatalf("expected root + styled wrapper kept, got %d divs: %s", n, out)
	}
}

func TestGenerateInlineStyles(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:     "1:7",
		Name:   "row",
		Type:   "FRAME",
		Layout: ir.LayoutSpec{Mode: "FLEX", Direction: "row", Gap: 8, JustifyContent: "flex-start", AlignItems: "stretch"},
		Styles: ir.StyleSpec{Visible: true, Fills: []ir.FillSpec{{Type: "SOLID", Color: "#FF0000", Visible: true}}},
		Children: []ir.NodeSpec{
			{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}},
			{Type: "TEXT", Name: "b", Text: ir.TextSpec{Content: "B"}},
		},
	}
	out := testGen(spec)
	if !strings.Contains(out, `style="display:flex; flex-direction:row; gap:8px; background-color:#FF0000"`) {
		t.Fatalf("expected inline styles, got: %s", out)
	}
	if strings.Contains(out, ` class=`) {
		t.Fatalf("inline mode must not emit class attributes: %s", out)
	}
	if strings.Contains(out, ":root") {
		t.Fatalf("unexpected :root block: %s", out)
	}
}

func TestGenerateTailwindPreservesClasses(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:     "1:8",
		Name:   "row",
		Type:   "FRAME",
		Layout: ir.LayoutSpec{Mode: "FLEX", Direction: "row", Gap: 8, JustifyContent: "flex-start", AlignItems: "stretch"},
		Children: []ir.NodeSpec{
			{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}},
			{Type: "TEXT", Name: "b", Text: ir.TextSpec{Content: "B"}},
		},
	}
	out, err := (&HTMLGenerator{}).GenerateComponent(spec, ir.TokenSet{}, ir.GeneratorConfig{Style: "tailwind"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `class="comp-row flex flex-row gap-[8px]"`) {
		t.Fatalf("expected tailwind classes, got: %s", out)
	}
}

func TestGenerateKeepsSectionWrapper(t *testing.T) {
	spec := ir.ComponentSpec{
		ID:   "1:6",
		Name: "sec",
		Type: "SECTION",
		Children: []ir.NodeSpec{{
			Type: "FRAME", Name: "root", Children: []ir.NodeSpec{
				{Type: "SECTION", Name: "wrapper", Children: []ir.NodeSpec{
					{Type: "TEXT", Name: "a", Text: ir.TextSpec{Content: "A"}},
				}},
			},
		}},
	}
	out := testGen(spec)
	if n := strings.Count(out, "<section"); n != 1 {
		t.Fatalf("expected the section kept even though it is a pure wrapper, got count %d: %s", n, out)
	}
}
