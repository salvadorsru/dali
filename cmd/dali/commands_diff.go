package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/salvadorsru/dali/internal/index"
	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/output"
)

type nodeChange struct {
	Status  string   `json:"status"` // added | removed | modified
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Changed []string `json:"changed,omitempty"`
}

// cmdDiff compares two .fig files by stable node ID, reporting added, removed
// and modified nodes.
func cmdDiff(args []string) error {
	a, err := arg(args, 0, "old.fig")
	if err != nil {
		return err
	}
	b, err := arg(args, 1, "new.fig")
	if err != nil {
		return err
	}
	old, err := load(a)
	if err != nil {
		return err
	}
	cur, err := load(b)
	if err != nil {
		return err
	}

	changes := diffNodes(old, cur)

	if jsonOut {
		return output.JSON(os.Stdout, changes)
	}
	if len(changes) == 0 {
		fmt.Println(output.Success("no differences"))
		return nil
	}
	rows := make([][]string, 0, len(changes))
	for _, c := range changes {
		rows = append(rows, []string{statusLabel(c.Status), output.ID(c.ID), output.Type(c.Type), truncate(c.Name, 40), output.Dim(strings.Join(c.Changed, ", "))})
	}
	output.Table(os.Stdout, []string{"STATUS", "ID", "TYPE", "NAME", "CHANGED"}, rows)
	summary := counts(changes)
	fmt.Printf("%s\n", output.Dim(fmt.Sprintf("%d added, %d removed, %d modified", summary[0], summary[1], summary[2])))
	return nil
}

// diffNodes compares two documents by stable node ID, reporting added, removed
// and fingerprint-differing (modified) nodes.
func diffNodes(old, cur *model.Document) []nodeChange {
	oldByID := make(map[string]*model.Node, len(old.All()))
	for _, n := range old.All() {
		oldByID[n.ID] = n
	}
	curByID := make(map[string]*model.Node, len(cur.All()))
	for _, n := range cur.All() {
		curByID[n.ID] = n
	}

	var changes []nodeChange
	for _, n := range cur.All() {
		o, ok := oldByID[n.ID]
		if !ok {
			changes = append(changes, nodeChange{Status: "added", ID: n.ID, Type: string(n.Type), Name: n.Name})
			continue
		}
		if index.NodeFingerprint(o) != index.NodeFingerprint(n) {
			changes = append(changes, nodeChange{Status: "modified", ID: n.ID, Type: string(n.Type), Name: n.Name, Changed: changedFields(o, n)})
		}
	}
	for id, n := range oldByID {
		if _, ok := curByID[id]; !ok {
			changes = append(changes, nodeChange{Status: "removed", ID: n.ID, Type: string(n.Type), Name: n.Name})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ID < changes[j].ID })
	return changes
}

// changedFields names the node fields that differ between two versions.
func changedFields(o, n *model.Node) []string {
	var f []string
	if o.Name != n.Name {
		f = append(f, "name")
	}
	if o.Text != n.Text {
		f = append(f, "text")
	}
	if o.X != n.X || o.Y != n.Y || o.Width != n.Width || o.Height != n.Height {
		f = append(f, "geometry")
	}
	if o.ParentID != n.ParentID || o.PageID != n.PageID {
		f = append(f, "parent")
	}
	if o.Position != n.Position {
		f = append(f, "order")
	}
	if o.Visible != n.Visible || o.Opacity != n.Opacity {
		f = append(f, "appearance")
	}
	if o.Type != n.Type {
		f = append(f, "type")
	}
	if o.ComponentID != n.ComponentID {
		f = append(f, "component")
	}
	if o.Path != n.Path {
		f = append(f, "path")
	}
	if fmt.Sprint(o.Runs) != fmt.Sprint(n.Runs) {
		f = append(f, "runs")
	}
	if fmt.Sprint(o.Props) != fmt.Sprint(n.Props) {
		f = append(f, "props")
	}
	if fmt.Sprint(o.Images) != fmt.Sprint(n.Images) {
		f = append(f, "images")
	}
	if fmt.Sprint(o.PaintStack()) != fmt.Sprint(n.PaintStack()) {
		f = append(f, "paints")
	}
	if len(f) == 0 {
		f = append(f, "other")
	}
	return f
}

func counts(changes []nodeChange) [3]int {
	var c [3]int
	for _, ch := range changes {
		switch ch.Status {
		case "added":
			c[0]++
		case "removed":
			c[1]++
		case "modified":
			c[2]++
		}
	}
	return c
}

func statusLabel(status string) string {
	switch status {
	case "added":
		return output.Success(status)
	case "removed":
		return output.Error(status)
	default:
		return output.Warn(status)
	}
}
