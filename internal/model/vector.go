package model

import (
	"encoding/binary"
	"math"
	"strings"
)

// blobIndex resolves a commandsBlob / vectorNetworkBlob reference into its
// blobs array offset. Blob references are numeric indices (0 is valid), or
// guid objects for indexes served through a broker.
func blobIndex(v any) int {
	if v == nil {
		return -1
	}
	if g := asMap(v); g != nil {
		return int(asInt(g["localID"]))
	}
	return int(asInt(v))
}

// nodeVectorPath builds the node's SVG path ("d" attribute) from its geometry
// blobs: fillGeometry and strokeGeometry are pre-rendered command streams (the
// lowest-risk source); when absent, the editable vectorNetworkBlob is decoded
// as a fallback. Coordinates are already in node size space.
func nodeVectorPath(m map[string]any, blobs [][]byte) string {
	if len(blobs) == 0 {
		return ""
	}
	var ds []string
	for _, key := range []string{"fillGeometry", "strokeGeometry"} {
		for _, g := range asSlice(m[key]) {
			gm := asMap(g)
			if gm == nil {
				continue
			}
			if d := decodeCommands(gm, blobs); d != "" {
				ds = append(ds, d)
			}
		}
	}
	if len(ds) > 0 {
		return strings.Join(ds, " ")
	}
	return decodeNetwork(m, blobs)
}

// decodeCommands renders a geometry entry (winding rule + commands blob) into
// an SVG path.
func decodeCommands(gm map[string]any, blobs [][]byte) string {
	idx := blobIndex(gm["commandsBlob"])
	if idx < 0 || idx >= len(blobs) {
		return ""
	}
	return decodeCommandsBlob(blobs[idx])
}

// decodeCommandsBlob decodes a command stream into an SVG path. Formats:
//
//	0x01 moveTo    x, y (f32 LE)
//	0x02 lineTo    x, y
//	0x04 cubicTo   c1x, c1y, c2x, c2y, x, y
//	0x00 closePath
func decodeCommandsBlob(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var sb strings.Builder
	i := 0
	for i < len(b) {
		op := b[i]
		i++
		switch op {
		case 0x01, 0x02:
			cmd := "L"
			if op == 0x01 {
				cmd = "M"
			}
			if i+8 > len(b) {
				return ""
			}
			sb.WriteString(cmd)
			writeFloat(&sb, b[i:])
			sb.WriteByte(' ')
			writeFloat(&sb, b[i+4:])
			sb.WriteByte(' ')
			i += 8
		case 0x04:
			if i+24 > len(b) {
				return ""
			}
			sb.WriteString("C")
			for k := 0; k < 6; k++ {
				writeFloat(&sb, b[i+k*4:])
				sb.WriteByte(' ')
			}
			i += 24
		case 0x00:
			if sb.Len() > 0 {
				sb.WriteString("Z ")
			}
		default:
			return ""
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

func writeFloat(sb *strings.Builder, at []byte) {
	v := math.Float32frombits(binary.LittleEndian.Uint32(at))
	sb.WriteString(shortFloat(float64(v)))
}

func floatBytes(v float64) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(float32(v)))
	return b
}

type networkSeg struct {
	a, e               int
	tx1, ty1, tx2, ty2 float64
}

// decodeNetwork decodes a vectorNetworkBlob into an SVG path as a fallback
// when the node has no fill/stroke geometry streams. Either the region loops
// describe closed contours directly, or unvisited segments are chained into
// runs. A segment is a straight line when all four tangents are zero, a cubic
// otherwise (controls are vertex plus tangent delta).
func decodeNetwork(m map[string]any, blobs [][]byte) string {
	vd := asMap(m["vectorData"])
	if vd == nil {
		return ""
	}
	idx := blobIndex(vd["vectorNetworkBlob"])
	if idx < 0 || idx >= len(blobs) {
		return ""
	}
	b := blobs[idx]
	if len(b) < 12 {
		return ""
	}
	vcount := int(binary.LittleEndian.Uint32(b[0:]))
	scount := int(binary.LittleEndian.Uint32(b[4:]))
	rcount := int(binary.LittleEndian.Uint32(b[8:]))
	at := 12
	pts := make([][2]float64, vcount)
	for i := range pts {
		if at+12 > len(b) {
			return ""
		}
		pts[i] = [2]float64{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+4:]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+8:]))),
		}
		at += 12
	}
	segs := make([]networkSeg, scount)
	for i := range segs {
		if at+28 > len(b) {
			return ""
		}
		s := networkSeg{
			a:   int(binary.LittleEndian.Uint32(b[at+4:])),
			e:   int(binary.LittleEndian.Uint32(b[at+16:])),
			tx1: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+8:]))),
			ty1: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+12:]))),
			tx2: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+20:]))),
			ty2: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[at+24:]))),
		}
		segs[i] = s
		at += 28
	}

	var sb strings.Builder
	move := func(v int) {
		if v >= 0 && v < len(pts) {
			sb.WriteString("M")
			writeFloat(&sb, floatBytes(pts[v][0]))
			sb.WriteByte(' ')
			writeFloat(&sb, floatBytes(pts[v][1]))
			sb.WriteByte(' ')
		}
	}
	stroke := func(s networkSeg) {
		if s.a < 0 || s.e < 0 || s.a >= len(pts) || s.e >= len(pts) {
			return
		}
		x1, y1 := pts[s.a][0], pts[s.a][1]
		x2, y2 := pts[s.e][0], pts[s.e][1]
		if s.tx1 == 0 && s.ty1 == 0 && s.tx2 == 0 && s.ty2 == 0 {
			sb.WriteString("L")
			writeFloat(&sb, floatBytes(x2))
			sb.WriteByte(' ')
			writeFloat(&sb, floatBytes(y2))
			sb.WriteByte(' ')
			return
		}
		sb.WriteString("C")
		writeFloat(&sb, floatBytes(x1+s.tx1))
		sb.WriteByte(' ')
		writeFloat(&sb, floatBytes(y1+s.ty1))
		sb.WriteByte(' ')
		writeFloat(&sb, floatBytes(x2+s.tx2))
		sb.WriteByte(' ')
		writeFloat(&sb, floatBytes(y2+s.ty2))
		sb.WriteByte(' ')
		writeFloat(&sb, floatBytes(x2))
		sb.WriteByte(' ')
		writeFloat(&sb, floatBytes(y2))
		sb.WriteByte(' ')
	}

	if rcount > 0 {
		for r := 0; r < rcount; r++ {
			if at+8 > len(b) {
				return ""
			}
			_ = binary.LittleEndian.Uint32(b[at:]) // packed: winding rule + styleID
			loops := int(binary.LittleEndian.Uint32(b[at+4:]))
			at += 8
			for l := 0; l < loops; l++ {
				if at+4 > len(b) {
					return ""
				}
				sc := int(binary.LittleEndian.Uint32(b[at:]))
				at += 4
				if sc == 0 || at+4*sc > len(b) {
					return ""
				}
				idxs := make([]int, 0, sc)
				for k := 0; k < sc; k++ {
					idxs = append(idxs, int(binary.LittleEndian.Uint32(b[at+k*4:])))
				}
				at += 4 * sc
				if idxs[0] < 0 || idxs[0] >= len(segs) {
					continue
				}
				first := segs[idxs[0]]
				if first.a < 0 || first.a >= len(pts) {
					continue
				}
				move(first.a)
				for _, s := range idxs {
					if s >= 0 && s < len(segs) {
						stroke(segs[s])
					}
				}
				sb.WriteString("Z ")
			}
		}
	} else {
		used := make([]bool, len(segs))
		for i := range segs {
			if used[i] {
				continue
			}
			cur := i
			end := -1
			first := true
			for cur != -1 && !used[cur] && cur < len(segs) {
				s := segs[cur]
				used[cur] = true
				if first {
					move(s.a)
					first = false
				} else if end != s.a {
					move(s.a)
				}
				stroke(s)
				end = s.e
				cur = -1
				for j := 0; j < len(segs); j++ {
					if !used[j] && segs[j].a == end {
						cur = j
						break
					}
				}
			}
			if !first {
				sb.WriteString("Z ")
			}
		}
	}
	d := strings.TrimRight(sb.String(), " ")
	if d == "" {
		return ""
	}
	return d
}
