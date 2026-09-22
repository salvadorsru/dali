package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// GeneratorVersion bumps when the semantic text format changes, forcing re-embedding.
const GeneratorVersion = 1

type Doc struct {
	NodeID string
	Text   string
	Hash   string
}

var typeLabels = map[string]string{
	"SYMBOL":            "component",
	"COMPONENT":         "component",
	"COMPONENT_SET":     "component set",
	"INSTANCE":          "instance",
	"FRAME":             "frame",
	"TEXT":              "text",
	"VECTOR":            "vector",
	"ELLIPSE":           "ellipse",
	"RECTANGLE":         "rectangle",
	"ROUNDED_RECTANGLE": "rectangle",
	"STAR":              "star",
	"LINE":              "line",
	"GROUP":             "group",
	"SECTION":           "section",
	"BOOLEAN_OPERATION": "boolean operation",
	"VARIABLE":          "variable",
	"VARIABLE_SET":      "variable set",
	"CANVAS":            "page",
	"DOCUMENT":          "document",
	"REGULAR_POLYGON":   "polygon",
}

// Build renders the normalized semantic text for a node.
func Build(nodeID, nodeType, name, text, pageName, parentName string) Doc {
	var b strings.Builder
	if label := typeLabels[nodeType]; label != "" {
		b.WriteString("type: ")
		b.WriteString(label)
		b.WriteString("\n")
	}
	writeField(&b, "name", name)
	writeField(&b, "page", pageName)
	writeField(&b, "parent", parentName)
	writeField(&b, "text", truncate(text, 1000))
	body := strings.TrimSpace(b.String())
	return Doc{
		NodeID: nodeID,
		Text:   body,
		Hash:   hash(body),
	}
}

func writeField(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
}

func hash(text string) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(GeneratorVersion) + "\n" + text))
	return hex.EncodeToString(sum[:])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
