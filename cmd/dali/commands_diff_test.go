package main

import (
	"testing"

	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/testdoc"
)

func TestDiffNodes(t *testing.T) {
	spec := testdoc.Spec{Pages: 1, FramesPerPage: 2, TextsPerFrame: 2, VectorsPerFrame: 3, Components: 1, InstancesPerPage: 1, Seed: 1}
	base := testdoc.Changes(spec)

	identical := diffNodes(model.Build(copyChanges(base), nil), model.Build(copyChanges(base), nil))
	if len(identical) != 0 {
		t.Fatalf("identical documents differ: %+v", identical)
	}

	oldIDs := ids(model.Build(copyChanges(base), nil))
	cur := copyChanges(base)

	var pageGUID any
	renamed := false
	var dropIdx = -1
	for i, c := range cur {
		if !renamed && isText(c) && c.(map[string]any)["name"] != "Label" {
			cur[i].(map[string]any)["name"] = "Renamed Text"
			renamed = true
		}
		if isVector(c) {
			dropIdx = i
		}
		if isCANVAS(c) {
			pageGUID = c.(map[string]any)["guid"]
		}
	}
	if !renamed {
		t.Fatal("no TEXT node found to modify")
	}
	if dropIdx == -1 {
		t.Fatal("no VECTOR node found to drop")
	}
	if pageGUID == nil {
		t.Fatal("no CANVAS page found")
	}

	// Drop one leaf vector (from a frame, so no expanded instance is affected)
	// and append a brand-new frame so the new document gains and loses exactly
	// one node each.
	cur[dropIdx] = nil
	cur = compact(cur)
	cur = append(cur, addedNode(pageGUID))

	changes := diffNodes(model.Build(base, nil), model.Build(cur, nil))

	var added, removed, modCount int
	for _, ch := range changes {
		switch ch.Status {
		case "added":
			added++
		case "removed":
			removed++
		case "modified":
			modCount++
		}
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (dropped instance; old had %d ids)", removed, oldIDs)
	}
	if modCount != 1 {
		t.Fatalf("modified = %d, want 1", modCount)
	}
	for _, ch := range changes {
		if ch.Status == "modified" {
			found := false
			for _, f := range ch.Changed {
				if f == "name" {
					found = true
				}
			}
			if !found {
				t.Fatalf("modified node missing name in Changed: %v", ch.Changed)
			}
		}
	}
}

func copyChanges(src []any) []any {
	out := make([]any, len(src))
	for i, c := range src {
		m := c.(map[string]any)
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		out[i] = cp
	}
	return out
}

func isText(c any) bool {
	m, ok := c.(map[string]any)
	return ok && m["type"] == "TEXT"
}

func isVector(c any) bool {
	m, ok := c.(map[string]any)
	return ok && m["type"] == "VECTOR"
}

func compact(src []any) []any {
	out := src[:0]
	for _, c := range src {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}

func isCANVAS(c any) bool {
	m, ok := c.(map[string]any)
	return ok && m["type"] == "CANVAS"
}

func addedNode(parentGUID any) any {
	return map[string]any{
		"phase":       "CREATED",
		"guid":        map[string]any{"sessionID": int64(1), "localID": int64(99998)},
		"type":        "FRAME",
		"name":        "New Frame",
		"visible":     true,
		"opacity":     1.0,
		"size":        map[string]any{"x": 200.0, "y": 48.0},
		"transform":   map[string]any{"m00": 1.0, "m01": 0.0, "m02": 0.0, "m10": 0.0, "m11": 1.0, "m12": 0.0},
		"parentIndex": map[string]any{"guid": parentGUID, "position": "99999"},
	}
}

func ids(doc *model.Document) int {
	return len(doc.All())
}
