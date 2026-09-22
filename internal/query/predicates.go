package query

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/internal/model"
)

// Query evaluates a structural DSL expression and returns matching nodes.
//
// Supported forms, combinable with AND:
//
//	field=value       exact match (type, name, text, font, page, component)
//	field~value       case-insensitive substring match
//	field!=value      negation
//	visible=true      boolean filter (true|false|1|0|yes|no|on|off)
//	width/height/font_size/opacity with =, !=, >, >=, <, <=
//	descendants|ancestors|children|parent("Name")  named-node relations
//
// Expressions starting with / are XPath; Query rejects them with a hint.
func (s *Service) Query(expr string) ([]*model.Node, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	if strings.HasPrefix(expr, "/") {
		return nil, fmt.Errorf("expressions starting with / are XPath; use Service.XPath")
	}
	terms, err := splitAnd(expr)
	if err != nil {
		return nil, err
	}
	if rel, name, ok := relationCall(terms[0]); ok {
		return s.relationQuery(rel, name, terms[1:])
	}
	var clauses []string
	var args []any
	for _, term := range terms {
		c, a, err := s.predicateSQL(term)
		if err != nil {
			return nil, err
		}
		if len(c) == 0 {
			return nil, fmt.Errorf("cannot parse query %q", expr)
		}
		clauses = append(clauses, c...)
		args = append(args, a...)
	}
	return s.queryNodes(`WHERE `+strings.Join(clauses, " AND ")+` ORDER BY name`, args...)
}

// splitAnd splits expr on the top-level AND operator, honouring quoted
// values so text like "Fish and chips" survives.
func splitAnd(expr string) ([]string, error) {
	var terms []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			cur.WriteByte(c)
			continue
		}
		if isAnd(expr, i) {
			if t := strings.TrimSpace(cur.String()); t != "" {
				terms = append(terms, t)
			}
			cur.Reset()
			i += 2
			continue
		}
		cur.WriteByte(c)
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		terms = append(terms, t)
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("cannot parse query %q", expr)
	}
	return terms, nil
}

// isAnd reports whether the standalone, space-delimited token AND starts at i.
func isAnd(expr string, i int) bool {
	if i+3 > len(expr) || !strings.EqualFold(expr[i:i+3], "and") {
		return false
	}
	if i > 0 && isIdent(expr[i-1]) {
		return false
	}
	if j := i + 3; j < len(expr) && isIdent(expr[j]) {
		return false
	}
	return true
}

func isIdent(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// relationCall detects a named-node relation and returns it with the
// argument.
func relationCall(term string) (rel, name string, ok bool) {
	for _, r := range []string{"descendants", "ancestors", "children", "parent"} {
		if strings.HasPrefix(term, r+"(") {
			return r, inner(term, r), true
		}
	}
	return "", "", false
}

// relationQuery resolves a named-node relation with SQL and, when extra
// predicate terms are present, filters the result in memory. Like XPath, this
// is a deliberate in-memory exception: the relation root stays SQL-owned and
// only the additional terms are evaluated over materialized nodes.
func (s *Service) relationQuery(rel, name string, predicates []string) ([]*model.Node, error) {
	nodes, err := s.relation(rel, name)
	if err != nil {
		return nil, err
	}
	if len(predicates) == 0 {
		return nodes, nil
	}
	ps := make([]*predicate, 0, len(predicates))
	for _, term := range predicates {
		if _, _, isRel := relationCall(term); isRel {
			return nil, fmt.Errorf("relation %q must be the first term", term)
		}
		p, err := parseTerm(term)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	match, err := s.newInMemoryMatch()
	if err != nil {
		return nil, err
	}
	var out []*model.Node
	for _, n := range nodes {
		ok := true
		for _, p := range ps {
			if !match.match(n, p) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

func (s *Service) relation(rel, name string) ([]*model.Node, error) {
	node, err := s.byName(name)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("no node named %q", name)
	}
	switch rel {
	case "descendants":
		return s.Descendants(node.ID)
	case "ancestors":
		return s.Ancestors(node.ID)
	case "children":
		return s.Children(node.ID)
	default:
		p, err := s.Parent(node.ID)
		if err != nil || p == nil {
			return nil, err
		}
		return []*model.Node{p}, nil
	}
}

func (s *Service) byName(name string) (*model.Node, error) {
	n, err := scanNode(s.sql.QueryRow(`SELECT `+nodeColumns+` FROM nodes WHERE name = ? LIMIT 1`, name))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

func inner(expr, fn string) string {
	open := strings.Index(expr, "(")
	end := strings.LastIndex(expr, ")")
	if open < 0 || end < open {
		return ""
	}
	return unquote(strings.TrimSpace(expr[open+1 : end]))
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

type predKind int

const (
	predString predKind = iota
	predBool
	predNumber
)

type predMeta struct {
	field  string
	column string
	kind   predKind
}

var predFields = map[string]predMeta{
	"name":      {field: "name", column: "name", kind: predString},
	"type":      {field: "type", column: "type", kind: predString},
	"text":      {field: "text", column: "text", kind: predString},
	"page":      {field: "page", column: "page_id", kind: predString},
	"font":      {field: "font", column: "font_family", kind: predString},
	"component": {field: "component", column: "component_id", kind: predString},
	"visible":   {field: "visible", column: "visible", kind: predBool},
	"width":     {field: "width", column: "width", kind: predNumber},
	"height":    {field: "height", column: "height", kind: predNumber},
	"font_size": {field: "font_size", column: "font_size", kind: predNumber},
	"opacity":   {field: "opacity", column: "opacity", kind: predNumber},
}

type predicate struct {
	field string
	op    string
	value string
}

// parseTerm splits a DSL term into field, operator and value.
func parseTerm(term string) (*predicate, error) {
	i := indexOp(term)
	if i < 0 {
		return nil, fmt.Errorf("cannot parse predicate %q", term)
	}
	field := strings.ToLower(strings.TrimSpace(term[:i]))
	if field == "" {
		return nil, fmt.Errorf("empty field in predicate %q", term)
	}
	op := opAt(term, i)
	value := unquote(strings.TrimSpace(term[i+len(op):]))
	if value == "" {
		return nil, fmt.Errorf("empty value in predicate %q", term)
	}
	return &predicate{field: field, op: op, value: value}, nil
}

// indexOp returns the index of the first operator, skipping quoted values.
func indexOp(s string) int {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
		case c == '~' || c == '=' || c == '>' || c == '<':
			return i
		case c == '!' && i+1 < len(s) && s[i+1] == '=':
			return i
		}
	}
	return -1
}

// opAt returns the two-character operator starting at i when present,
// otherwise the single character.
func opAt(s string, i int) string {
	if i+1 < len(s) && (s[i] == '!' || s[i] == '>' || s[i] == '<') && s[i+1] == '=' {
		return s[i : i+2]
	}
	return s[i : i+1]
}

// predicateSQL compiles one predicate term into SQL WHERE fragments.
func (s *Service) predicateSQL(term string) ([]string, []any, error) {
	if _, _, isRel := relationCall(term); isRel {
		return nil, nil, fmt.Errorf("relation %q must be the first term", term)
	}
	p, err := parseTerm(term)
	if err != nil {
		return nil, nil, err
	}
	meta, ok := predFields[p.field]
	if !ok {
		return nil, nil, fmt.Errorf("unknown field %q", p.field)
	}
	switch meta.kind {
	case predBool:
		if p.op != "=" {
			return nil, nil, fmt.Errorf("%s only supports =, got %q in %q", p.field, p.op, term)
		}
		b, err := parseBool(p.value)
		if err != nil {
			return nil, nil, err
		}
		if b {
			return []string{meta.column + " = 1"}, nil, nil
		}
		return []string{meta.column + " = 0"}, nil, nil
	case predNumber:
		return s.numberPredicate(meta.column, p, term)
	default:
		return s.textPredicate(meta, p, term)
	}
}

func (s *Service) numberPredicate(col string, p *predicate, term string) ([]string, []any, error) {
	switch p.op {
	case "=", "!=", ">", ">=", "<", "<=":
	default:
		return nil, nil, fmt.Errorf("bad operator %q for %s in %q", p.op, p.field, term)
	}
	num, err := strconv.ParseFloat(p.value, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("bad numeric value %q in %q", p.value, term)
	}
	op := p.op
	if op == "!=" {
		op = "<>"
	}
	return []string{col + " " + op + " ?"}, []any{num}, nil
}

func (s *Service) textPredicate(meta predMeta, p *predicate, term string) ([]string, []any, error) {
	switch meta.field {
	case "page":
		return s.pagePredicate(p, term)
	case "component":
		return s.componentPredicate(p, term)
	}
	col := meta.column
	val := p.value
	if meta.field == "type" {
		val = strings.ToUpper(val)
	}
	switch p.op {
	case "~":
		return []string{col + " LIKE ?"}, []any{"%" + val + "%"}, nil
	case "=":
		return []string{col + " = ?"}, []any{val}, nil
	case "!=":
		return []string{col + " <> ?"}, []any{val}, nil
	}
	return nil, nil, fmt.Errorf("bad operator %q for %s in %q", p.op, meta.field, term)
}

func (s *Service) pagePredicate(p *predicate, term string) ([]string, []any, error) {
	switch p.op {
	case "~":
		return []string{`page_id IN (SELECT id FROM pages WHERE name LIKE ?)`}, []any{"%" + p.value + "%"}, nil
	case "=":
		return []string{`page_id IN (SELECT id FROM pages WHERE name = ?)`}, []any{p.value}, nil
	case "!=":
		return []string{`page_id NOT IN (SELECT id FROM pages WHERE name = ?)`}, []any{p.value}, nil
	}
	return nil, nil, fmt.Errorf("bad operator %q for page in %q", p.op, term)
}

// componentPredicate matches nodes whose component_id points at a component
// the value names. A value containing ':' matches the component id
// directly; otherwise it matches the component name (exact for =, substring
// for ~).
func (s *Service) componentPredicate(p *predicate, term string) ([]string, []any, error) {
	if p.op != "=" && p.op != "!=" && p.op != "~" {
		return nil, nil, fmt.Errorf("bad operator %q for component in %q", p.op, term)
	}
	ids, err := s.componentIDs(strings.Contains(p.value, ":"), p.op == "~", p.value)
	if err != nil {
		return nil, nil, err
	}
	if p.op != "!=" {
		if len(ids) == 0 {
			return []string{"1 = 0"}, nil, nil
		}
		ph := strings.Repeat("?,", len(ids)-1) + "?"
		return []string{"component_id IN (" + ph + ")"}, toAny(ids), nil
	}
	if len(ids) == 0 {
		return []string{"1 = 1"}, nil, nil
	}
	ph := strings.Repeat("?,", len(ids)-1) + "?"
	return []string{"component_id NOT IN (" + ph + ")"}, toAny(ids), nil
}

// componentIDs resolves the ids of components whose id or name matches value.
func (s *Service) componentIDs(isID, substring bool, value string) ([]string, error) {
	if isID {
		return []string{value}, nil
	}
	comps, err := s.Components()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, c := range comps {
		name := strings.ToLower(c.Name)
		if substring {
			if strings.Contains(name, strings.ToLower(value)) {
				ids = append(ids, c.ID)
			}
		} else if name == strings.ToLower(value) {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	}
	return false, fmt.Errorf("bad boolean value %q (use true or false)", v)
}

// inMemoryMatch resolves page and component names once so relation queries can
// filter their materialized results on fields that need those lookups.
type inMemoryMatch struct {
	pages map[string]string // page id -> name
	comps map[string]string // component id -> name
}

func (s *Service) newInMemoryMatch() (*inMemoryMatch, error) {
	pages, err := s.Pages()
	if err != nil {
		return nil, err
	}
	comps, err := s.Components()
	if err != nil {
		return nil, err
	}
	m := &inMemoryMatch{pages: make(map[string]string, len(pages)), comps: make(map[string]string, len(comps))}
	for _, p := range pages {
		m.pages[p.ID] = p.Name
	}
	for _, c := range comps {
		m.comps[c.ID] = c.Name
	}
	return m, nil
}

func (m *inMemoryMatch) match(n *model.Node, p *predicate) bool {
	switch p.field {
	case "name":
		return matchString(n.Name, p.value, p.op)
	case "type":
		return matchString(string(n.Type), strings.ToUpper(p.value), p.op)
	case "text":
		return matchString(n.Text, p.value, p.op)
	case "font":
		return matchString(n.FontFamily, p.value, p.op)
	case "visible":
		b, err := parseBool(p.value)
		if err != nil {
			return false
		}
		return n.Visible == b
	case "width", "height", "font_size", "opacity":
		num, err := strconv.ParseFloat(p.value, 64)
		if err != nil {
			return false
		}
		var got float64
		switch p.field {
		case "width":
			got = n.Width
		case "height":
			got = n.Height
		case "font_size":
			got = n.FontSize
		default:
			got = n.Opacity
		}
		return matchNumber(got, num, p.op)
	case "page":
		return matchString(m.pages[n.PageID], p.value, p.op)
	case "component":
		return m.matchComponent(n.ComponentID, p)
	}
	return false
}

func matchString(got, value, op string) bool {
	switch op {
	case "~":
		return strings.Contains(strings.ToLower(got), strings.ToLower(value))
	case "=":
		return got == value
	case "!=":
		return got != value
	}
	return false
}

func matchNumber(got, want float64, op string) bool {
	switch op {
	case "=":
		return got == want
	case "!=":
		return got != want
	case ">":
		return got > want
	case ">=":
		return got >= want
	case "<":
		return got < want
	case "<=":
		return got <= want
	}
	return false
}

func (m *inMemoryMatch) matchComponent(componentID string, p *predicate) bool {
	matched := m.matchingComponentIDs(strings.Contains(p.value, ":"), p.op == "~", p.value)
	if p.op == "!=" {
		return componentID == "" || !matched[componentID]
	}
	return matched[componentID]
}

func (m *inMemoryMatch) matchingComponentIDs(isID, substring bool, value string) map[string]bool {
	out := make(map[string]bool)
	for id, name := range m.comps {
		if isID {
			if id == value {
				out[id] = true
			}
			continue
		}
		if substring {
			if strings.Contains(strings.ToLower(name), strings.ToLower(value)) {
				out[id] = true
			}
		} else if strings.ToLower(name) == strings.ToLower(value) {
			out[id] = true
		}
	}
	return out
}
