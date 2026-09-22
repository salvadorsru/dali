package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Table prints an aligned table. Cells may already contain ANSI color codes;
// widths are measured on the visible text so columns stay aligned.
func Table(w io.Writer, headers []string, rows [][]string) {
	cols := len(headers)
	width := make([]int, cols)
	for i, h := range headers {
		width[i] = visibleLen(h)
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			if l := visibleLen(row[i]); l > width[i] {
				width[i] = l
			}
		}
	}

	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, Bold(h))
		fmt.Fprint(w, strings.Repeat(" ", width[i]-visibleLen(h)))
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		for i := 0; i < cols; i++ {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprint(w, cell)
			fmt.Fprint(w, strings.Repeat(" ", width[i]-visibleLen(cell)))
		}
		fmt.Fprintln(w)
	}
}

// visibleLen counts runes, skipping ANSI escape sequences.
func visibleLen(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !isFinalByte(s[i]) {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		n++
		i += size
	}
	return n
}

func isFinalByte(b byte) bool { return b >= 0x40 && b <= 0x7e }
