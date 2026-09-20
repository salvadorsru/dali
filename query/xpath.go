package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/model"
)

// xpathSelector is a parsed XPath subset: /, //, * and a single predicate on
// @name ('=', '~' or contains()). Any other XPath feature is rejected with a
// clear message instead of being silently misparsed.
type xpathSelector struct {
	steps []xpathStep
}

type xpathStep struct {
	descend bool   // subtree (//) instead of direct children (/)
	ntype   string // node type filter, empty when the step is *
	nameOp  string // "", "=", "~" or "contains"
	name    string // predicate value
}

// parseXPath parses the supported XPath subset.
func parseXPath(s string) (*xpathSelector, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '/' {
		return nil, errors.New("XPath must start with /")
	}
	sel := &xpathSelector{}
	i := 1
	for i < len(s) {
		descend := false
		if s[i] == '/' {
			descend = true
			i++
		}
		start := i
		for i < len(s) && s[i] != '/' && s[i] != '[' {
			i++
		}
		name := strings.TrimSpace(s[start:i])
		if name == "" {
			return nil, fmt.Errorf("empty step in XPath %q", s)
		}
		step := xpathStep{descend: descend}
		if name != "*" {
			step.ntype = strings.ToUpper(name)
		}
		if i < len(s) && s[i] == '[' {
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unclosed [ in XPath %q", s)
			}
			if err := parsePredicate(&step, strings.TrimSpace(s[i+1:i+end])); err != nil {
				return nil, err
			}
			i += end + 1
		}
		sel.steps = append(sel.steps, step)
		if i < len(s) && s[i] == '/' {
			i++
		}
	}
	if len(sel.steps) == 0 {
		return nil, fmt.Errorf("empty XPath %q", s)
	}
	return sel, nil
}

func parsePredicate(step *xpathStep, pred string) error {
	switch {
	case pred == "":
		return errors.New("empty predicate []")
	case strings.HasPrefix(pred, "@name"):
		rest := strings.TrimSpace(strings.TrimPrefix(pred, "@name"))
		op, value, err := splitPredicate(rest)
		if err != nil {
			return err
		}
		step.nameOp, step.name = op, value
		return nil
	case strings.HasPrefix(pred, "contains(@name"):
		rest := strings.TrimSpace(strings.TrimPrefix(pred, "contains(@name"))
		if !strings.HasPrefix(rest, ",") {
			return fmt.Errorf("bad contains() predicate %q", pred)
		}
		value, err := stripParenthesis(rest)
		if err != nil {
			return err
		}
		step.nameOp, step.name = "contains", value
		return nil
	}
	return fmt.Errorf("unsupported XPath predicate %q (only @name='..', @name~'..' and contains(@name,'..'))", pred)
}

func splitPredicate(rest string) (op, value string, err error) {
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) < 2 {
		return "", "", fmt.Errorf("bad @name predicate %q", rest)
	}
	switch rest[0] {
	case '=', '~':
	default:
		return "", "", fmt.Errorf("bad @name predicate %q (use = or ~)", rest)
	}
	op = string(rest[0])
	value = strings.TrimSpace(rest[1:])
	if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
		value = value[1 : len(value)-1]
	}
	if value == "" {
		return "", "", fmt.Errorf("empty @name value in predicate %q", rest)
	}
	return op, value, nil
}

func stripParenthesis(rest string) (string, error) {
	value := strings.TrimSpace(strings.TrimPrefix(rest, ","))
	value = strings.TrimSpace(strings.TrimSuffix(value, ")"))
	if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
		value = value[1 : len(value)-1]
	}
	if value == "" {
		return "", fmt.Errorf("empty contains() value in %q", rest)
	}
	return value, nil
}

// XPath runs a supported XPath subset query over the whole document. It is a
// deliberate exception to the package's SQL-owned reads: the subset's tree
// axes (//, predicates with location context) are evaluated in memory over a
// materialized snapshot of the index, cached once per Service. Traversal in
// the rest of the package (Descendants, Children, ...) stays SQL-owned via
// recursive CTEs.
func (s *Service) XPath(expr string) ([]*model.Node, error) {
	sel, err := parseXPath(expr)
	if err != nil {
		return nil, err
	}
	pages, _, children, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	var cur []*model.Node
	for _, p := range pages {
		cur = append(cur, children[p.ID]...)
	}
	for _, step := range sel.steps {
		var next []*model.Node
		if step.descend {
			next = descendantsOf(cur, children)
		} else {
			next = childrenOf(cur, children)
		}
		cur = filterNodes(next, step)
	}
	return cur, nil
}

// snapshot materializes the whole tree from the index once per Service. The
// returned maps must be treated as read-only.
func (s *Service) snapshot() (pages []*model.Node, byID map[string]*model.Node, children map[string][]*model.Node, err error) {
	s.snapOnce.Do(func() {
		all, err := s.queryNodes("")
		if err != nil {
			err = fmt.Errorf("snapshot: %w", err)
			return
		}
		byID = make(map[string]*model.Node, len(all))
		children = make(map[string][]*model.Node)
		for _, n := range all {
			byID[n.ID] = n
			children[n.ParentID] = append(children[n.ParentID], n)
		}
		if pages, err = s.Pages(); err != nil {
			err = fmt.Errorf("snapshot pages: %w", err)
			return
		}
		s.snapPages, s.snapByID, s.snapChildren = pages, byID, children
		s.snapErr = err
	})
	return s.snapPages, s.snapByID, s.snapChildren, s.snapErr
}

func childrenOf(nodes []*model.Node, children map[string][]*model.Node) []*model.Node {
	var out []*model.Node
	for _, n := range nodes {
		out = append(out, children[n.ID]...)
	}
	return out
}

func descendantsOf(nodes []*model.Node, children map[string][]*model.Node) []*model.Node {
	seen := make(map[string]bool)
	var out []*model.Node
	var walk func(*model.Node)
	walk = func(n *model.Node) {
		for _, c := range children[n.ID] {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, c)
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return out
}

func filterNodes(nodes []*model.Node, step xpathStep) []*model.Node {
	var out []*model.Node
	for _, n := range nodes {
		if step.ntype != "" && n.Type != model.NodeType(step.ntype) {
			continue
		}
		switch step.nameOp {
		case "":
		case "=":
			if n.Name != step.name {
				continue
			}
		case "~", "contains":
			if !strings.Contains(strings.ToLower(n.Name), strings.ToLower(step.name)) {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}