package query

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/model"
)

func (s *Service) Query(expr string) ([]*model.Node, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	for _, rel := range []string{"descendants", "ancestors", "children", "parent"} {
		if strings.HasPrefix(expr, rel+"(") {
			return s.relation(rel, inner(expr, rel))
		}
	}
	if i := strings.Index(expr, "~"); i > 0 {
		key := strings.TrimSpace(expr[:i])
		value := unquote(strings.TrimSpace(expr[i+1:]))
		if key == "name" {
			return s.queryNodes(`WHERE name LIKE ? ORDER BY name`, "%"+value+"%")
		}
		return nil, fmt.Errorf("unknown field %q", key)
	}
	if i := strings.Index(expr, "="); i > 0 {
		key := strings.TrimSpace(expr[:i])
		value := unquote(strings.TrimSpace(expr[i+1:]))
		switch key {
		case "type":
			return s.queryNodes(`WHERE type = ? ORDER BY name`, strings.ToUpper(value))
		case "name":
			return s.queryNodes(`WHERE name = ? ORDER BY name`, value)
		}
		return nil, fmt.Errorf("unknown field %q", key)
	}
	return nil, fmt.Errorf("cannot parse query %q", expr)
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
