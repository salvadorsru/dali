package model

import "testing"

func gid(session, local int32) map[string]any {
	return map[string]any{"sessionID": session, "localID": local}
}

func named(session, local int32, typ, name string, parent map[string]any) map[string]any {
	m := mkNode(local, typ, name, parent, "!")
	m["guid"] = gid(session, local)
	return m
}

func withSize(m map[string]any, x, y float32) map[string]any {
	m["size"] = map[string]any{"x": x, "y": y}
	return m
}

func withTransform(m map[string]any, x, y float32) map[string]any {
	m["transform"] = map[string]any{"m00": float32(1), "m01": float32(0), "m10": float32(0), "m11": float32(1), "m02": x, "m12": y}
	return m
}

func withText(m map[string]any, chars string) map[string]any {
	ids := make([]any, 0, len([]rune(chars)))
	for range chars {
		ids = append(ids, int32(0))
	}
	m["textData"] = map[string]any{"characters": chars, "characterStyleIDs": ids}
	return m
}

func TestResolveInstanceClone(t *testing.T) {
	comp := named(1, 100, "SYMBOL", "Chip", g(0, 0))
	withSize(comp, 100, 50)
	withTransform(comp, 0, 0)
	comp["overrideKey"] = gid(9, 10)

	text := named(1, 101, "TEXT", "Sueño", gid(1, 100))
	withSize(text, 50, 20)
	withTransform(text, 6, 6)
	withText(text, "MENTE")
	text["fontName"] = map[string]any{"family": "Syne", "style": "Medium"}
	text["fontSize"] = float32(11)
	text["overrideKey"] = gid(9, 11)

	inst := named(0, 200, "INSTANCE", "Chip Inst", g(0, 2))
	withSize(inst, 50, 25)
	withTransform(inst, 0, 0)
	inst["symbolData"] = map[string]any{
		"symbolID": gid(1, 100),
		"symbolOverrides": []any{
			map[string]any{
				"guidPath": map[string]any{"guids": []any{gid(9, 11)}},
				"textData": map[string]any{"characters": "PIEL"},
				"fontSize": float32(13),
			},
		},
	}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		mkNode(2, "FRAME", "Frame", g(0, 1), "a"),
		comp, text, inst,
	}, nil)

	instNode, _ := doc.Node("0:200")
	if instNode.symbol == nil {
		t.Fatal("instance symbol not linked")
	}
	children := doc.Children("0:200")
	if len(children) != 1 {
		t.Fatalf("resolved children = %+v, want 1", children)
	}
	clone := children[0]
	if clone.ID != "inst:0:200.1:101" {
		t.Fatalf("clone id = %q", clone.ID)
	}
	if clone.ParentID != "0:200" || doc.Parent(clone.ID).ID != "0:200" {
		t.Fatalf("clone parent = %q", clone.ParentID)
	}
	if clone.PageID != "0:1" {
		t.Fatalf("clone page = %q", clone.PageID)
	}
	if clone.Text != "PIEL" || clone.FontSize != 13 {
		t.Fatalf("clone text = %q fs %v", clone.Text, clone.FontSize)
	}
	if clone.X != 3 || clone.Y != 3 || clone.Width != 25 || clone.Height != 10 {
		t.Fatalf("clone geometry = %v,%v %vx%v", clone.X, clone.Y, clone.Width, clone.Height)
	}
	if len(clone.Runs) != 1 || clone.Runs[0].Text != "PIEL" || clone.Runs[0].FontSize != 13 {
		t.Fatalf("clone runs = %+v", clone.Runs)
	}
}

func TestResolveInstanceTextProp(t *testing.T) {
	comp := named(2, 200, "SYMBOL", "Hero", gid(0, 0))
	withSize(comp, 300, 200)
	withTransform(comp, 0, 0)

	title := named(2, 201, "TEXT", "Title", gid(2, 200))
	withText(title, "Default")
	title["fontSize"] = float32(20)
	title["componentPropRefs"] = []any{
		map[string]any{"componentPropNodeField": "TEXT_VALUE", "defID": gid(7, 1)},
	}

	inst := named(0, 210, "INSTANCE", "Hero Inst", g(0, 2))
	withSize(inst, 300, 200)
	withTransform(inst, 0, 0)
	inst["symbolData"] = map[string]any{"symbolID": gid(2, 200)}
	inst["componentPropAssignments"] = []any{
		map[string]any{
			"defID": gid(7, 1),
			"value": map[string]any{"textValue": map[string]any{"characters": "Descubre tu rutina"}},
		},
	}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		mkNode(2, "FRAME", "Frame", g(0, 1), "a"),
		comp, title, inst,
	}, nil)

	children := doc.Children("0:210")
	if len(children) != 1 {
		t.Fatalf("children = %+v, want 1", children)
	}
	if children[0].Text != "Descubre tu rutina" {
		t.Fatalf("text prop not applied: %q", children[0].Text)
	}
}

func TestResolveInstanceSlot(t *testing.T) {
	comp := named(3, 300, "SYMBOL", "Pagination", g(0, 0))
	withSize(comp, 200, 30)
	withTransform(comp, 0, 0)

	slot := named(3, 301, "FRAME", "Slot", gid(3, 300))
	withSize(slot, 200, 30)
	withTransform(slot, 0, 0)
	slot["componentPropRefs"] = []any{
		map[string]any{"componentPropNodeField": "SLOT_CONTENT_ID", "defID": gid(7, 1)},
	}

	content := named(0, 302, "FRAME", "Dots", g(0, 1))
	withSize(content, 100, 10)
	withTransform(content, 0, 10)
	dot := named(0, 303, "TEXT", "Dot", gid(0, 302))
	withText(dot, "•")

	inst := named(0, 310, "INSTANCE", "Pagination Inst", g(0, 2))
	withSize(inst, 200, 30)
	withTransform(inst, 0, 0)
	inst["symbolData"] = map[string]any{"symbolID": gid(3, 300)}
	inst["componentPropAssignments"] = []any{
		map[string]any{
			"defID": gid(7, 1),
			"value": map[string]any{},
			"varValue": map[string]any{
				"dataType": "SLOT_CONTENT_ID", "resolvedDataType": "SLOT_CONTENT_ID",
				"value": map[string]any{"slotContentIdValue": map[string]any{"guid": gid(0, 302)}},
			},
		},
	}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		mkNode(2, "FRAME", "Frame", g(0, 1), "a"),
		comp, slot, content, dot, inst,
	}, nil)

	children := doc.Children("0:310")
	if len(children) != 1 {
		t.Fatalf("children = %+v, want 1 mounted slot", children)
	}
	if children[0].ID != "inst:0:310.0:302" {
		t.Fatalf("mounted root = %q", children[0].ID)
	}
	if children[0].X != 0 || children[0].Y != 0 {
		t.Fatalf("mounted pos = %v,%v", children[0].X, children[0].Y)
	}
	inner := doc.Children(children[0].ID)
	if len(inner) != 1 || inner[0].Text != "•" {
		t.Fatalf("mounted content = %+v", inner)
	}
}

func TestResolveNestedInstance(t *testing.T) {
	compA := named(4, 400, "SYMBOL", "Card", g(0, 0))
	withSize(compA, 100, 100)
	withTransform(compA, 0, 0)
	nested := named(4, 401, "INSTANCE", "Cta", gid(4, 400))
	withSize(nested, 50, 20)
	withTransform(nested, 10, 10)
	nested["symbolData"] = map[string]any{"symbolID": gid(5, 500)}

	compB := named(5, 500, "SYMBOL", "Button", g(0, 0))
	withSize(compB, 50, 20)
	withTransform(compB, 0, 0)
	label := named(5, 501, "TEXT", "Label", gid(5, 500))
	withText(label, "Añadir")

	inst := named(0, 600, "INSTANCE", "Card Inst", g(0, 2))
	withSize(inst, 100, 100)
	withTransform(inst, 0, 0)
	inst["symbolData"] = map[string]any{"symbolID": gid(4, 400)}

	doc := Build([]any{
		mkNode(0, "DOCUMENT", "Doc", nil, ""),
		mkNode(1, "CANVAS", "Page", g(0, 0), "a"),
		mkNode(2, "FRAME", "Frame", g(0, 1), "a"),
		compA, nested, compB, label, inst,
	}, nil)

	cardChildren := doc.Children("0:600")
	if len(cardChildren) != 1 {
		t.Fatalf("card children = %+v, want 1 nested instance", cardChildren)
	}
	btnClone := cardChildren[0]
	if btnClone.ID != "inst:0:600.4:401" || btnClone.ParentID != "0:600" {
		t.Fatalf("nested clone = %q", btnClone.ID)
	}
	btnChildren := doc.Children(btnClone.ID)
	if len(btnChildren) != 1 {
		t.Fatalf("nested instance not expanded: %+v", btnChildren)
	}
	labelClone := btnChildren[0]
	if labelClone.Text != "Añadir" {
		t.Fatalf("nested label = %q", labelClone.Text)
	}
	if labelClone.X != 0 || labelClone.Y != 0 {
		t.Fatalf("nested label pos = %v,%v", labelClone.X, labelClone.Y)
	}
}
