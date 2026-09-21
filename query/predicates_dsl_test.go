package query

import (
	"reflect"
	"strings"
	"testing"

	"github.com/salvadorsru/dali/index"
	"github.com/salvadorsru/dali/model"
)

// daliDoc builds a small multi-page test document:
//
//	DOCUMENT 0:0 "Doc"
//	└─ CANVAS 0:1 "Mobile"
//	   ├─ FRAME 0:2 "Screen" (390x844)
//	   │  ├─ FRAME 0:3 "Card" (200x100)
//	   │  ├─ FRAME 0:4 "Hidden" (50x50, visible=false)
//	   │  ├─ TEXT 0:5 "Price" "19,99" Inter Regular 18
//	   │  ├─ FRAME 0:6 "Wide" (640x60)
//	   │  └─ INSTANCE 0:11 "CTA" -> component 0:10
//	   └─ SYMBOL 0:10 "PrimaryButton"
func daliDoc() *model.Document {
	root := qGUID(0, 0)
	page := qGUID(0, 1)
	frame := qGUID(0, 2)
	changes := []any{
		qNode(0, "DOCUMENT", "Doc", nil, ""),
		qNode(1, "CANVAS", "Mobile", root, "a"),
		qNode(2, "FRAME", "Screen", page, "a"),
	}
	screen := changes[len(changes)-1].(map[string]any)
	screen["size"] = map[string]any{"x": uint32(390), "y": uint32(844)}
	frameNode := func(local int32, name string, pos string, mut ...func(map[string]any)) map[string]any {
		n := qNode(local, "FRAME", name, frame, pos)
		for _, m := range mut {
			m(n)
		}
		return n
	}
	card := frameNode(3, "Card", "a")
	card["size"] = map[string]any{"x": uint32(200), "y": uint32(100)}
	changes = append(changes, card)

	hidden := frameNode(4, "Hidden", "b")
	hidden["size"] = map[string]any{"x": uint32(50), "y": uint32(50)}
	hidden["visible"] = false
	changes = append(changes, hidden)

	price := qNode(5, "TEXT", "Price", frame, "c")
	price["textData"] = map[string]any{"characters": "19,99"}
	price["fontName"] = map[string]any{"family": "Inter", "style": "Regular"}
	price["fontSize"] = float64(18)
	changes = append(changes, price)

	wide := frameNode(6, "Wide", "d")
	wide["size"] = map[string]any{"x": uint32(640), "y": uint32(60)}
	changes = append(changes, wide)

	inst := qNode(11, "INSTANCE", "CTA", frame, "e")
	inst["symbolData"] = map[string]any{"symbolID": qGUID(0, 10)}
	changes = append(changes, inst)

	modal := qNode(12, "FRAME", "Modal", page, "c")
	modal["size"] = map[string]any{"x": uint32(300), "y": uint32(200)}
	changes = append(changes, modal)
	inst2 := qNode(13, "INSTANCE", "CTA2", qGUID(0, 12), "a")
	inst2["symbolData"] = map[string]any{"symbolID": qGUID(0, 10)}
	changes = append(changes, inst2)

	changes = append(changes, qNode(10, "SYMBOL", "PrimaryButton", page, "b"))
	return model.Build(changes, nil)
}

func openDSLService(t *testing.T) *Service {
	t.Helper()
	svc, err := openService(daliDoc())
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func openService(doc *model.Document) (*Service, error) {
	db, err := index.Open(":memory:")
	if err != nil {
		return nil, err
	}
	if err := db.Write(doc); err != nil {
		return nil, err
	}
	return &Service{db: db, sql: db.SQL()}, nil
}

func names(nodes []*model.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}

func nodeHasName(nodes []*model.Node, name string) bool {
	for _, n := range nodes {
		if n.Name == name {
			return true
		}
	}
	return false
}

func allBy(f func(*model.Node) bool) func([]*model.Node) bool {
	return func(nodes []*model.Node) bool {
		for _, n := range nodes {
			if !f(n) {
				return false
			}
		}
		return true
	}
}

func TestQueryPredicates(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	tests := []struct {
		name  string
		query string
		has   []string
		exact []string
		all   func(*model.Node) bool
	}{
		{name: "type", query: "type=FRAME", has: []string{"Card", "Screen"}},
		{name: "type case", query: "type=frame", has: []string{"Card"}},
		{name: "name exact", query: `name="Card"`, exact: []string{"Card"}},
		{name: "name like", query: `name~"wid"`, exact: []string{"Wide"}},
		{name: "name not", query: "name!=Card", all: func(n *model.Node) bool { return n.Name != "Card" }},
		{name: "and conjunction", query: "type=FRAME AND visible=true", has: []string{"Card", "Wide", "Screen"}, all: func(n *model.Node) bool { return n.Visible }},
		{name: "and lowercase", query: "type=text and text~\",99\"", exact: []string{"Price"}},
		{name: "text contains", query: `text~"19"`, exact: []string{"Price"}},
		{name: "font", query: `font=Inter`, exact: []string{"Price"}},
		{name: "font like", query: `font~"inter"`, exact: []string{"Price"}},
		{name: "page exact", query: "page=Mobile", all: func(n *model.Node) bool { return n.PageID == "0:1" }},
		{name: "page like", query: "page~mail", all: func(n *model.Node) bool { return n.PageID == "0:1" }},
		{name: "page miss", query: "page=Desktop", exact: nil},
		{name: "visible hidden", query: "visible=false", exact: []string{"Hidden"}},
		{name: "visible true text", query: "type=TEXT AND visible=true", exact: []string{"Price"}},
		{name: "width min", query: "width>=300", has: []string{"Wide", "Screen"}},
		{name: "width between", query: "width>=300 AND height<=100", exact: []string{"Wide"}},
		{name: "font size", query: "font_size=18", exact: []string{"Price"}},
		{name: "opacity zero", query: "opacity<=0", exact: nil},
		{name: "component name", query: "component=PrimaryButton", has: []string{"CTA"}},
		{name: "component id", query: "component=0:10", has: []string{"CTA"}},
		{name: "component not", query: "component!=PrimaryButton", exact: nil},
		{name: "component name like", query: "component~Button", has: []string{"CTA"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := svc.Query(tc.query)
			if err != nil {
				t.Fatalf("Query(%q): %v", tc.query, err)
			}
			for _, want := range tc.has {
				if !nodeHasName(nodes, want) {
					t.Fatalf("Query(%q) = %v, missing %q", tc.query, names(nodes), want)
				}
			}
			if len(tc.exact) > 0 {
				got := names(nodes)
				if !reflect.DeepEqual(got, tc.exact) {
					t.Fatalf("Query(%q) = %v, want %v", tc.query, got, tc.exact)
				}
			}
			if tc.all != nil && !allBy(tc.all)(nodes) {
				t.Fatalf("Query(%q) = %v, predicate held on all nodes", tc.query, names(nodes))
			}
		})
	}
}

func TestQueryPredicateErrors(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	for _, tc := range []struct {
		query, want string
	}{
		{query: "foo=bar", want: `unknown field "foo"`},
		{query: "width>=", want: "empty value"},
		{query: "width>=x", want: "bad numeric value"},
		{query: "visible=maybe", want: "bad boolean value"},
		{query: "visible>=1", want: "visible only supports ="},
		{query: "width~100", want: "bad operator"},
		{query: `type=TEXT AND descendants("Card")`, want: "must be the first term"},
		{query: `descendants("Nope")`, want: "no node named"},
	} {
		_, err := svc.Query(tc.query)
		if err == nil {
			t.Fatalf("Query(%q): expected error containing %q", tc.query, tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Query(%q) error = %q, want containing %q", tc.query, err, tc.want)
		}
	}
}

func TestQueryRelationPredicates(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	nodes, err := svc.Query(`descendants("Screen") AND type=INSTANCE`)
	if err != nil {
		t.Fatal(err)
	}
	got := names(nodes)
	if !reflect.DeepEqual(got, []string{"CTA"}) {
		t.Fatalf("descendants+type = %v, want [CTA]", got)
	}

	nodes, err = svc.Query(`descendants("Screen") AND visible=true AND type=TEXT`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names(nodes), []string{"Price"}) {
		t.Fatalf("visible+type = %v, want [Price]", names(nodes))
	}

	nodes, err = svc.Query(`children("Screen")`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 5 {
		t.Fatalf("children = %d, want 5", len(nodes))
	}
}

func TestParseTerm(t *testing.T) {
	tests := []struct {
		term             string
		field, op, value string
	}{
		{term: `name~"Button"`, field: "name", op: "~", value: "Button"},
		{term: "type != TEXT", field: "type", op: "!=", value: "TEXT"},
		{term: `text="hello world"`, field: "text", op: "=", value: "hello world"},
		{term: "width>=200", field: "width", op: ">=", value: "200"},
		{term: "visible=true", field: "visible", op: "=", value: "true"},
		{term: `page~'Web'`, field: "page", op: "~", value: "Web"},
	}
	for _, tc := range tests {
		p, err := parseTerm(tc.term)
		if err != nil {
			t.Fatalf("parseTerm(%q): %v", tc.term, err)
		}
		if p.field != tc.field || p.op != tc.op || p.value != tc.value {
			t.Fatalf("parseTerm(%q) = %+v, want f=%q op=%q v=%q", tc.term, p, tc.field, tc.op, tc.value)
		}
	}
}

func TestSplitAnd(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "type=TEXT", want: []string{"type=TEXT"}},
		{in: `text~"fish and chips"`, want: []string{`text~"fish and chips"`}},
		{in: "type=FRAME AND visible=true", want: []string{"type=FRAME", "visible=true"}},
		{in: "type=text and width>=2", want: []string{"type=text", "width>=2"}},
		{in: "name~android AND visible=true", want: []string{"name~android", "visible=true"}},
	}
	for _, tc := range tests {
		got, err := splitAnd(tc.in)
		if err != nil {
			t.Fatalf("splitAnd(%q): %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("splitAnd(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestPredFieldPrefix(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "type=TEXT", want: "type"},
		{in: "visible=false", want: "visible"},
		{in: "name~Card", want: "name"},
		{in: "checkout", want: ""},
		{in: "", want: ""},
	} {
		if got := predFieldPrefix(tc.in); got != tc.want {
			t.Fatalf("predFieldPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsStructuralExprPredicates(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{in: "type=TEXT AND visible=true", want: true},
		{in: "width>=200", want: true},
		{in: "checkout", want: false},
		{in: "text~price", want: true},
		{in: `descendants("Page")`, want: true},
	} {
		if got := isStructuralExpr(tc.in); got != tc.want {
			t.Fatalf("isStructuralExpr(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
