package export

import (
	"fmt"
	"strings"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
)

// HTML wraps the SVG rendering in an interactive page that can be opened
// directly in a browser.
func HTML(doc *model.Document, file *parser.File, opts ExportOptions) ([]byte, error) {
	svg, err := SVG(doc, file, opts)
	if err != nil {
		return nil, err
	}
	title := escape(doc.Name)
	if doc.Name == "" {
		title = "Dalí export"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n", title)
	b.WriteString(`<style>
body { margin: 0; background: #17171c; display: grid; place-items: center; min-height: 100vh; }
svg { max-width: 100%; height: auto; background: #fff; outline: 1px solid #333; }
</style>
</head>
<body>
`)
	b.WriteString(string(svg))
	b.WriteString("\n</body>\n</html>\n")
	return []byte(b.String()), nil
}
