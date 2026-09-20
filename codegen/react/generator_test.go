package react

import (
	"strings"
	"testing"

	"github.com/salvadorsru/dali/codegen/ir"
)

func TestGenerateComponentValidJSX(t *testing.T) {
	g := &ReactGenerator{}
	spec := ir.ComponentSpec{
		ID:   "1:2",
		Name: "Button/Primary",
		Type: "COMPONENT",
		Children: []ir.NodeSpec{{
			ID: "1:3", Type: "TEXT", Name: "Label",
			Text: ir.TextSpec{Content: "-15%"},
		}},
	}
	code, err := g.GenerateComponent(spec, ir.TokenSet{}, ir.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "export function ButtonPrimary(") {
		t.Errorf("missing export function, got:\n%s", code)
	}
	if !strings.Contains(code, `{"-15%"}`) {
		t.Errorf("text not quoted as JS string, got:\n%s", code)
	}
	if !strings.Contains(code, "import { clsx } from 'clsx';") {
		t.Errorf("missing clsx import, got:\n%s", code)
	}
}

func TestGenerateComponentVariantClasses(t *testing.T) {
	g := &ReactGenerator{}
	spec := ir.ComponentSpec{
		ID:   "1:2",
		Name: "Badge",
		Type: "COMPONENT_SET",
		Variants: []ir.ComponentVariant{{
			ID: "1:3", Name: "Default", Props: map[string]any{"state": "Default"},
		}},
	}
	code, err := g.GenerateComponent(spec, ir.TokenSet{}, ir.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "const variantClasses = {") {
		t.Errorf("missing variantClasses block, got:\n%s", code)
	}
}