// Package testdoc builds synthetic Figma node changes for scale and
// conformance testing. It feeds model.Build directly, the same seam the parser
// hands decoded nodeChanges to, so tests exercise the full pipeline
// (Build, Write, query) without committing large .fig binaries.
package testdoc

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/salvadorsru/dali/model"
)

type Spec struct {
	Pages            int
	FramesPerPage    int
	TextsPerFrame    int
	VectorsPerFrame  int
	Components       int
	InstancesPerPage int
	Seed             int64
}

func (s Spec) normalize() Spec {
	if s.Pages < 1 {
		s.Pages = 1
	}
	if s.FramesPerPage < 1 {
		s.FramesPerPage = 1
	}
	if s.TextsPerFrame < 1 {
		s.TextsPerFrame = 1
	}
	if s.Components < 1 {
		s.Components = 1
	}
	return s
}

// NodeCount estimates the document size a spec produces, without instance
// expansion clones. Useful for picking specs in tests.
func (s Spec) NodeCount() int {
	s = s.normalize()
	const varsPerDoc = 4 // one set + three variables
	return 1 + s.Pages + s.Components*3 + varsPerDoc +
		s.Pages*s.FramesPerPage*(1+s.TextsPerFrame+s.VectorsPerFrame)
}

// Build assembles a synthetic document. Deterministic for a given spec.
func Build(spec Spec) *model.Document {
	return model.Build(Changes(spec), nil)
}

// Changes returns the nodeChanges consumed by model.Build.
func Changes(spec Spec) []any {
	spec = spec.normalize()
	g := &gen{spec: spec, rnd: rand.New(rand.NewSource(spec.Seed)), next: 1, pos: map[int]int{}}

	docID := g.node(0, "DOCUMENT", "Document", nil)
	pageIDs := make([]int, spec.Pages)
	for p := 0; p < spec.Pages; p++ {
		pageIDs[p] = g.node(docID, "CANVAS", fmt.Sprintf("Page %d", p+1), nil)
	}

	compIDs := make([]int, spec.Components)
	for c := 0; c < spec.Components; c++ {
		compIDs[c] = g.component(pageIDs[0], c)
	}
	g.variables(pageIDs[0])

	for _, page := range pageIDs {
		for f := 0; f < spec.FramesPerPage; f++ {
			frame := g.node(page, "FRAME", g.words(2), setSize(240, 96))
			for t := 0; t < spec.TextsPerFrame; t++ {
				g.text(frame)
			}
			for v := 0; v < spec.VectorsPerFrame; v++ {
				g.vector(frame)
			}
		}
		for i := 0; i < spec.InstancesPerPage; i++ {
			g.instance(page, compIDs[i%len(compIDs)])
		}
	}
	return g.changes
}

type gen struct {
	spec    Spec
	rnd     *rand.Rand
	next    int
	pos     map[int]int
	changes []any
}

func (g *gen) alloc() int {
	v := g.next
	g.next++
	return v
}

func guid(v int) map[string]any {
	return map[string]any{"sessionID": int64(1), "localID": int64(v)}
}

func setSize(x, y float64) map[string]any {
	return map[string]any{"size": map[string]any{"x": x, "y": y}}
}

func (g *gen) position(parent int) string {
	p := g.pos[parent]
	g.pos[parent] = p + 1
	return fmt.Sprintf("%05d", p)
}

func (g *gen) node(parent int, typ, name string, extra map[string]any) int {
	gid := g.alloc()
	m := map[string]any{
		"phase":     "CREATED",
		"guid":      guid(gid),
		"type":      typ,
		"name":      name,
		"visible":   true,
		"opacity":   1.0,
		"size":      map[string]any{"x": 200.0, "y": 48.0},
		"transform": map[string]any{"m00": 1.0, "m01": 0.0, "m02": 0.0, "m10": 0.0, "m11": 1.0, "m12": 0.0},
	}
	if parent > 0 {
		m["parentIndex"] = map[string]any{
			"guid":     guid(parent),
			"position": g.position(parent),
		}
	}
	for k, v := range extra {
		m[k] = v
	}
	g.changes = append(g.changes, m)
	return gid
}

func (g *gen) component(parent, n int) int {
	id := g.node(parent, "SYMBOL", fmt.Sprintf("Button %d", n+1), setSize(120, 40))
	g.node(id, "TEXT", "Label", merge(setSize(80, 20), map[string]any{
		"textData": map[string]any{"characters": "Continue"},
		"fontName": map[string]any{"family": "Inter", "style": "Medium"},
		"fontSize": 14.0,
	}))
	g.vector(id)
	return id
}

func (g *gen) text(parent int) int {
	return g.node(parent, "TEXT", g.words(2), merge(setSize(180, 20), map[string]any{
		"textData":            map[string]any{"characters": g.phrase()},
		"fontName":            map[string]any{"family": "Inter", "style": "Regular"},
		"fontSize":            14.0,
		"textAlignHorizontal": "LEFT",
	}))
}

func (g *gen) vector(parent int) int {
	return g.node(parent, "VECTOR", g.words(2), merge(setSize(24, 24), map[string]any{}))
}

func (g *gen) instance(parent, compID int) int {
	return g.node(parent, "INSTANCE", "Instance of button", merge(setSize(120, 40), map[string]any{
		"symbolData": map[string]any{"symbolID": guid(compID)},
	}))
}

func (g *gen) variables(parent int) {
	set := g.node(parent, "VARIABLE_SET", "Colors", map[string]any{
		"variableSetModes": []any{
			map[string]any{"id": guid(900001), "name": "Light"},
			map[string]any{"id": guid(900002), "name": "Dark"},
		},
	})
	for i, name := range []string{"Primary", "Ink", "Surface"} {
		g.node(set, "VARIABLE", name, map[string]any{
			"key":                  fmt.Sprintf("color-%d", i+1),
			"variableResolvedType": "COLOR",
			"variableSetID":        map[string]any{"guid": guid(set)},
			"variableDataValues": map[string]any{"entries": []any{
				map[string]any{
					"modeID":       guid(900001),
					"variableData": map[string]any{"value": map[string]any{"colorValue": map[string]any{"r": 0.12, "g": 0.43, "b": 0.92, "a": 1.0}}},
				},
				map[string]any{
					"modeID":       guid(900002),
					"variableData": map[string]any{"value": map[string]any{"colorValue": map[string]any{"r": 0.24, "g": 0.24, "b": 0.26, "a": 1.0}}},
				},
			}},
		})
	}
}

func merge(a, b map[string]any) map[string]any {
	for k, v := range b {
		a[k] = v
	}
	return a
}

func (g *gen) words(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = g.word()
	}
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ")
}

func (g *gen) word() string {
	return wordPool[g.rnd.Intn(len(wordPool))]
}

func (g *gen) phrase() string {
	return phrases[g.rnd.Intn(len(phrases))]
}

var wordPool = []string{
	"card", "button", "header", "footer", "price", "product", "health", "payment", "checkout",
	"profile", "settings", "dashboard", "report", "invoice", "notification", "search", "filter",
	"table", "chart", "banner", "hero", "menu", "navigation", "modal", "tooltip", "badge",
	"avatar", "gallery", "list", "grid", "slider", "toggle", "account", "billing", "shipping",
	"coupon", "discount", "total", "subtotal", "review", "rating", "comment", "share", "download",
	"upload", "sync", "export", "cancel", "confirm", "save", "delete", "edit", "add", "remove",
	"primary", "secondary", "success", "error", "warning", "info", "subscription", "premium",
	"upgrade", "trial", "welcome", "landing", "signup", "login", "logout", "onboarding", "empty",
	"loading", "spinner", "placeholder", "caption", "detail", "summary", "message", "email",
	"phone", "address", "order", "status", "tracking", "delivered", "pending", "refund", "wallet",
	"balance", "transfer", "verified", "secure", "protected", "privacy", "terms", "policy",
}

var phrases = []string{
	"Complete your payment securely",
	"Continue to checkout",
	"Terms of service apply to all orders",
	"Your order has been confirmed",
	"Total amount due includes shipping",
	"Sign in to manage your subscription",
	"Welcome back to your dashboard",
	"Please review the updated pricing",
	"Your balance is now available",
	"Track your delivery in real time",
}
