package quality

import (
	"fmt"
	"math"
	"strings"
)

// luminance computes the relative luminance of a hex color (3, 6 or 8 digits,
// the 8th/alpha channel is ignored).
func luminance(color string) (float64, error) {
	r, g, b, err := rgb(color)
	if err != nil {
		return 0, err
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b), nil
}

func linear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func rgb(s string) (r, g, b float64, err error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	case 8:
		s = s[:6] // ignore alpha
	default:
		return 0, 0, 0, fmt.Errorf("not a hex color %q", s)
	}
	part := func(i int) (float64, error) {
		v, e := parseNibble(s, i)
		return float64(v) / 255, e
	}
	if r, err = part(0); err != nil {
		return
	}
	if g, err = part(2); err != nil {
		return
	}
	b, err = part(4)
	return
}

func parseNibble(s string, i int) (int, error) {
	hi, ok := hexVal(s[i])
	if !ok {
		return 0, fmt.Errorf("not a hex color %q", s)
	}
	lo, ok := hexVal(s[i+1])
	if !ok {
		return 0, fmt.Errorf("not a hex color %q", s)
	}
	return hi*16 + lo, nil
}

func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// contrast returns the WCAG contrast ratio between two hex colors, 1:1 when
// either color cannot be parsed.
func contrast(fg, bg string) float64 {
	fl, err := luminance(fg)
	if err != nil {
		return 1
	}
	bl, err := luminance(bg)
	if err != nil {
		return 1
	}
	if fl < bl {
		fl, bl = bl, fl
	}
	return (fl + 0.05) / (bl + 0.05)
}