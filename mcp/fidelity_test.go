package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
	"github.com/salvadorsru/dali/query"
)

// TestPayloadFidelity checks that the indexed inspection payload reconstructs
// the design faithfully: ordered paints and variable bindings survive the index
// round-trip unchanged, absolute coordinates accumulate down the tree, and
// vector view boxes match the node box. It is the acceptance test for treating
// the SQLite index as the source of truth for an agent.
func TestPayloadFidelity(t *testing.T) {
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	svc, err := query.Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	f, err := parser.Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc := model.Build(f.NodeChanges(), f.Blobs())

	// Round-trip: every sampled node's paints and variable bindings must match
	// the in-memory model exactly, order included.
	painted, bound := 0, 0
	for i, n := range doc.All() {
		if i%97 != 0 {
			continue
		}
		got, err := svc.Node(n.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("node %s missing from index", n.ID)
		}
		want := n.PaintStack()
		if len(want.Fills)+len(want.Strokes)+len(want.Effects) > 0 {
			if !sameJSON(got.PaintData, want) {
				t.Fatalf("paints drifted for %s:\n got %s\nwant %s", n.ID, mustJSON(got.PaintData), mustJSON(want))
			}
			painted++
		}
		if wantBV := doc.BoundVariables(n); !reflect.DeepEqual(got.BoundVars, wantBV) {
			t.Fatalf("bound variables drifted for %s: got %v want %v", n.ID, got.BoundVars, wantBV)
		} else if len(wantBV) > 0 {
			bound++
		}
	}
	if painted == 0 {
		t.Fatal("no painted nodes sampled; paints fidelity not exercised")
	}
	t.Logf("verified paints on %d nodes, variable bindings on %d", painted, bound)

	// Payload: absolute coordinates accumulate and vector boxes match the node.
	pages, err := svc.Pages()
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages: %v", err)
	}
	out, err := SubtreeOf(svc, pages[0].ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	absX := map[string]float64{out.Root.ID: out.Root.AbsX}
	absY := map[string]float64{out.Root.ID: out.Root.AbsY}
	layout := map[string]bool{out.Root.ID: hasAutoLayout(out.Root)}
	vectors, free, flow := 0, 0, 0
	for _, d := range out.Descendants {
		n := d.Node
		wantX := absX[n.ParentID] + n.X
		wantY := absY[n.ParentID] + n.Y
		if !near(n.AbsX, wantX) || !near(n.AbsY, wantY) {
			t.Fatalf("abs coords for %s: got (%g,%g) want (%g,%g)", n.ID, n.AbsX, n.AbsY, wantX, wantY)
		}
		absX[n.ID], absY[n.ID] = n.AbsX, n.AbsY
		layout[n.ID] = hasAutoLayout(n)

		if layout[n.ParentID] && n.FreePositioned {
			t.Fatalf("node %s reported free_positioned under an auto-layout parent", n.ID)
		}
		if !layout[n.ParentID] && !n.FreePositioned {
			t.Fatalf("node %s not free_positioned under a static parent", n.ID)
		}
		if n.FreePositioned {
			free++
		} else {
			flow++
		}
		if n.Path != "" && n.Width > 0 && n.Height > 0 {
			if want := fmt.Sprintf("0 0 %g %g", n.Width, n.Height); n.ViewBox != want {
				t.Fatalf("view_box for %s: got %q want %q", n.ID, n.ViewBox, want)
			}
			vectors++
		}
	}
	t.Logf("subtree %s: %d descendants, %d vectors, %d free / %d flow", out.Root.ID, len(out.Descendants), vectors, free, flow)
}

func hasAutoLayout(ref NodeRef) bool {
	mode, _ := ref.LayoutData["layoutMode"].(string)
	return mode != "" && mode != "NONE"
}

func sameJSON(a, b any) bool {
	return mustJSON(a) == mustJSON(b)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "!" + err.Error()
	}
	return string(b)
}

func near(a, b float64) bool {
	d := a - b
	return d < 0.0001 && d > -0.0001
}
