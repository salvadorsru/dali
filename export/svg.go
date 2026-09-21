package export

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
)

// ExportOptions select the subtree to render. With no filters the whole
// document is exported, one page per column.
type ExportOptions struct {
	PageID string
	RootID string
}

const svgHeader = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" fill="none">`

type svgDoc struct {
	file *parser.File
	buf  strings.Builder
	def  []string
	img  int
}

// SVG renders the document (or one page/root subtree) as a single SVG.
func SVG(doc *model.Document, file *parser.File, opts ExportOptions) ([]byte, error) {
	roots := selectRoots(doc, opts)
	if len(roots) == 0 {
		return nil, fmt.Errorf("selection not found")
	}
	s := &svgDoc{file: file}
	widths := make([]float64, len(roots))
	var w, h float64
	for i, root := range roots {
		minX, minY, maxX, maxY := bbox(root, 0, 0)
		widths[i] = widthSpan(minX, maxX)
		if len(roots) == 1 {
			w = widths[i]
			h = heightSpan(minY, maxY)
		} else {
			w += widths[i]
			h = max(h, heightSpan(minY, maxY))
		}
	}
	s.buf.WriteString(fmt.Sprintf(svgHeader, num(w), num(h)))
	var offset float64
	for i, root := range roots {
		s.emitRoot(root, offset)
		offset += widths[i]
	}
	if len(s.def) > 0 {
		s.buf.WriteString(`<defs>` + strings.Join(s.def, "") + `</defs>`)
	}
	s.buf.WriteString("</svg>")
	return []byte(s.buf.String()), nil
}

func selectRoots(doc *model.Document, opts ExportOptions) []*model.Node {
	if opts.RootID != "" {
		n, ok := doc.Node(opts.RootID)
		if ok && n != nil {
			return []*model.Node{n}
		}
		return nil
	}
	if opts.PageID != "" {
		for _, p := range doc.Pages {
			if p.ID == opts.PageID {
				return []*model.Node{p}
			}
		}
		return nil
	}
	return doc.Pages
}

func widthSpan(minX, maxX float64) float64 {
	return max(0, maxX-minX)
}

func heightSpan(minY, maxY float64) float64 {
	return max(0, maxY-minY)
}

// bbox computes the axis-aligned bounds of a subtree in its local space.
func bbox(n *model.Node, ox, oy float64) (minX, minY, maxX, maxY float64) {
	x, y := ox+n.X, oy+n.Y
	minX, minY = x, y
	maxX, maxY = x+n.Width, y+n.Height
	for _, c := range n.Children {
		cx0, cy0, cx1, cy1 := bbox(c, x, y)
		minX = min(minX, cx0)
		minY = min(minY, cy0)
		maxX = max(maxX, cx1)
		maxY = max(maxY, cy1)
	}
	return
}

func (s *svgDoc) emitRoot(root *model.Node, offset float64) {
	if root.Rotation != 0 {
		s.buf.WriteString(fmt.Sprintf(`<g transform="translate(%s 0) rotate(%s %s %s)">`,
			num(offset), num(root.Rotation*180/3.141592653589793), num(root.Width/2), num(root.Height/2)))
		s.render(root, -offset, 0)
		s.buf.WriteString(`</g>`)
		return
	}
	if offset != 0 {
		s.buf.WriteString(fmt.Sprintf(`<g transform="translate(%s 0)">`, num(offset)))
	}
	s.render(root, 0, 0)
	if offset != 0 {
		s.buf.WriteString(`</g>`)
	}
}

func (s *svgDoc) render(n *model.Node, ox, oy float64) {
	if !n.Visible {
		return
	}
	switch n.Type {
	case model.TypeVariable, model.TypeVariableSet, model.TypeDocument:
		return
	}
	nx, ny := ox+n.X, oy+n.Y
	var closing []func()
	if n.Opacity > 0 && n.Opacity < 1 {
		s.buf.WriteString(fmt.Sprintf(`<g opacity="%s">`, num(n.Opacity)))
		closing = append(closing, func() { s.buf.WriteString(`</g>`) })
	}
	if n.Rotation != 0 {
		s.buf.WriteString(fmt.Sprintf(`<g transform="translate(%s %s) rotate(%s %s %s)">`,
			num(nx), num(ny), num(n.Rotation*180/3.141592653589793), num(n.Width/2), num(n.Height/2)))
		closing = append(closing, func() { s.buf.WriteString(`</g>`) })
		nx, ny = 0, 0
	}
	if n.ClipContent && n.Width > 0 && n.Height > 0 {
		s.def = append(s.def, fmt.Sprintf(
			`<clipPath id="clip-%s"><rect width="%s" height="%s" rx="%s"/></clipPath>`,
			sanitize(n.ID), num(n.Width), num(n.Height), num(s.radius(n))))
		s.buf.WriteString(fmt.Sprintf(`<g clip-path="url(#clip-%s)">`, sanitize(n.ID)))
		closing = append(closing, func() { s.buf.WriteString(`</g>`) })
	}

	s.shape(n, nx, ny)
	for _, c := range n.Children {
		s.render(c, nx, ny)
	}
	for i := len(closing) - 1; i >= 0; i-- {
		closing[i]()
	}
}

func (s *svgDoc) radius(n *model.Node) float64 {
	r, ok := n.Layout()["cornerRadius"].(float64)
	if !ok || r < 0 {
		return 0
	}
	return r
}

func (s *svgDoc) shape(n *model.Node, x, y float64) {
	switch n.Type {
	case model.TypeText:
		s.text(n, x, y)
		return
	case model.TypeInstance, model.TypeSymbol, model.TypeComponentSet, model.TypeGroup, model.TypeCanvas:
		return // containers are rendered as plain groups
	}
	fill := ""
	if hex, ok := n.FillHex(); ok && anyVisibleSolid(n, "FILL") {
		fill = hex
	}
	stroke := ""
	if hex, ok := n.StrokeHex(); ok {
		stroke = hex
	}
	strokeW := num(s.strokeWidth(n))
	w, h := num(n.Width), num(n.Height)
	if n.Type == model.TypeVector && n.Path != "" {
		s.buf.WriteString(fmt.Sprintf(`<path d="%s" fill="%s" stroke="%s" stroke-width="%s"/>`,
			n.Path, fill, stroke, strokeW))
		return
	}
	if n.Type == model.TypeVector {
		s.buf.WriteString(fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`,
			num(x), num(y), w, h, fill))
		return
	}
	if img := s.image(n, x, y); img != "" {
		s.buf.WriteString(fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" fill="url(#%s)"/>`,
			num(x), num(y), w, h, img))
		return
	}
	rx := ""
	if n.Type == model.TypeFrame {
		rx = fmt.Sprintf(" rx=%q", num(s.radius(n)))
	}
	s.buf.WriteString(fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s"%s fill="%s" stroke="%s" stroke-width="%s"/>`,
		num(x), num(y), w, h, rx, fill, stroke, strokeW))
}

func anyVisibleSolid(n *model.Node, _ string) bool {
	for _, p := range n.Fills() {
		pm := asMap(p)
		if pm == nil {
			continue
		}
		if v, ok := pm["visible"].(bool); ok && !v {
			continue
		}
		if t := asString(pm["type"]); t == "SOLID" || t == "" {
			return true
		}
	}
	return false
}

func (s *svgDoc) strokeWidth(n *model.Node) float64 {
	switch v := n.Layout()["strokeWeight"].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	}
	return 1
}

func (s *svgDoc) image(n *model.Node, x, y float64) string {
	if s.file == nil || len(n.Images) == 0 {
		return ""
	}
	data, err := s.file.ReadImage(n.Images[0].Hash)
	if err != nil || len(data) == 0 {
		return ""
	}
	id := fmt.Sprintf("img-%d", s.img)
	s.img++
	uri := "data:" + sniff(data) + ";base64," + base64.StdEncoding.EncodeToString(data)
	s.def = append(s.def, fmt.Sprintf(`<image id="%s" x="%s" y="%s" width="%s" height="%s" href="%s" preserveAspectRatio="xMidYMid meet"/>`,
		id, num(x), num(y), num(n.Width), num(n.Height), uri))
	return id
}

func sniff(b []byte) string {
	switch {
	case len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 12 && string(b[6:10]) == "WEBP":
		return "image/webp"
	}
	return "application/octet-stream"
}

func (s *svgDoc) text(n *model.Node, x, y float64) {
	t := n.Text
	if t == "" {
		return
	}
	fill := ""
	if hex, ok := n.FillHex(); ok && anyVisibleSolid(n, "FILL") {
		fill = hex
	}
	anchor := "start"
	if n.TextAlign == "CENTER" {
		anchor = "middle"
	} else if n.TextAlign == "RIGHT" {
		anchor = "end"
	}
	var b strings.Builder
	if n.Decoration == "UNDERLINE" || n.Decoration == "STRIKETHROUGH" {
		b.WriteString(fmt.Sprintf(`<text text-decoration="%s"`, strings.ToLower(n.Decoration)))
	} else {
		b.WriteString(`<text`)
	}
	b.WriteString(fmt.Sprintf(` x="%s" y="%s" font-family="%s" font-size="%s" font-style="%s" fill="%s" text-anchor="%s">`,
		num(x), num(y), escape(n.FontFamily), num(n.FontSize), n.FontStyle, fill, anchor))
	b.WriteString(escape(t))
	if len(n.Runs) > 0 {
		for _, r := range n.Runs {
			if r.Text == "" {
				continue
			}
			b.WriteString(fmt.Sprintf(`<tspan x="%s" fill="%s">%s</tspan>`, num(x), r.Color, escape(r.Text)))
		}
	}
	b.WriteString(`</text>`)
	s.buf.WriteString(b.String())
}

func sanitize(id string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
}

func escape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(s)
}

func num(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
