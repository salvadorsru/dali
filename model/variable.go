package model

import (
	"encoding/json"
	"strconv"
)

// Variable is a design token: a named value belonging to a variable set, with
// one entry per mode (theme, breakpoint, …).
type Variable struct {
	ID    string         `json:"id"`
	Key   string         `json:"key"`
	Name  string         `json:"name"`
	Set   string         `json:"set"`
	Type  string         `json:"type"`
	Modes []VariableMode `json:"modes"`
}

type VariableMode struct {
	Mode  string `json:"mode"`
	Value string `json:"value"`
	Raw   any    `json:"raw,omitempty"`
}

// collectVariables assembles the document's variables from VARIABLE and
// VARIABLE_SET nodes, resolving each variable's modes against its set's mode
// definitions.
func collectVariables(d *Document) []Variable {
	type set struct {
		name  string
		modes map[string]string
	}
	sets := make(map[string]set) // set id -> set info
	for _, n := range d.order {
		if n.Type != TypeVariableSet {
			continue
		}
		modes := map[string]string{}
		for _, mode := range asSlice(n.raw["variableSetModes"]) {
			mm := asMap(mode)
			if mm == nil {
				continue
			}
			if id := guidValue(mm["id"]); id != "" {
				modes[id] = asString(mm["name"])
			}
		}
		sets[n.ID] = set{name: n.Name, modes: modes}
	}
	var out []Variable
	for _, n := range d.order {
		if n.Type != TypeVariable {
			continue
		}
		raw := n.raw
		setID := guidOf(raw["variableSetID"])
		v := Variable{
			ID:   n.ID,
			Key:  asString(raw["key"]),
			Name: n.Name,
			Type: asString(raw["variableResolvedType"]),
		}
		if name := sets[setID].name; name != "" {
			v.Set = name
		}
		for _, e := range asSlice(asMap(raw["variableDataValues"])["entries"]) {
			em := asMap(e)
			if em == nil {
				continue
			}
			modeID := guidValue(em["modeID"])
			name := modeID
			if mn := sets[setID].modes[modeID]; mn != "" {
				name = mn
			}
			vd := asMap(em["variableData"])
			if vd == nil {
				continue
			}
			vm := VariableMode{Mode: name}
			vm.Raw = vd["value"]
			vm.Value = variableValueString(vd["value"])
			v.Modes = append(v.Modes, vm)
		}
		out = append(out, v)
	}
	return out
}

// variableValueString renders a variable value as a readable scalar: text,
// number, boolean, hex color or a variable alias reference.
func variableValueString(v any) string {
	vm := asMap(v)
	if vm == nil {
		return ""
	}
	if tv := vm["textValue"]; tv != nil {
		if s := asString(tv); s != "" {
			return s
		}
		if m := asMap(tv); m != nil {
			return asString(m["characters"])
		}
	}
	if n, ok := vm["numberValue"]; ok && n != nil {
		return strconv.FormatFloat(asFloat(n), 'g', -1, 64)
	}
	if b, ok := vm["booleanValue"].(bool); ok {
		return strconv.FormatBool(b)
	}
	if cv := asMap(vm["colorValue"]); cv != nil {
		return hexColor(clamp255(asFloat(cv["r"])), clamp255(asFloat(cv["g"])), clamp255(asFloat(cv["b"])), asFloat(cv["a"]))
	}
	if va := asMap(vm["varValue"]); va != nil {
		if g := guidValue(va["guid"]); g != "" {
			return "var:" + g
		}
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return ""
}
