// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"math"
	"strconv"
	"strings"
)

// maxPathCoord bounds a coordinate of a glyph path (svgrec's own bound, 2^24),
// so a shift of two such numbers is exact in a float64.
const maxPathCoord = 1 << 24

// pathArity is how many coordinate pairs each command of the path data svgrec
// writes takes: absolute M, L, Q, C and Z only (SVG 2,
// https://www.w3.org/TR/SVG2/paths.html 9.3).
var pathArity = map[byte]int{'M': 1, 'L': 1, 'Q': 2, 'C': 3, 'Z': 0}

// shiftPath returns the path data d moved by (dx, dy), numbers written with
// three decimals and trailing zeros trimmed, as svgrec writes them. It accepts
// exactly svgrec's grammar: a command letter, then its coordinates, numbers
// separated by one space, no other character. ok is false for anything else,
// for a number that is not finite or exceeds maxPathCoord, and for a command
// with the wrong number of coordinates. A used <symbol> is expanded with it
// when a still is cut out of an animation, because a rasteriser need not know
// <use>.
func shiftPath(d string, dx, dy float64) (out string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(d); {
		cmd := d[i]
		n, known := pathArity[cmd]
		if !known {
			return "", false
		}
		i++
		b.WriteByte(cmd)
		for k := 0; k < 2*n; k++ {
			if k > 0 {
				if i >= len(d) || d[i] != ' ' {
					return "", false
				}
				i++
				b.WriteByte(' ')
			}
			j := i
			for j < len(d) && (d[j] >= '0' && d[j] <= '9' || d[j] == '.' || d[j] == '-') {
				j++
			}
			v, err := strconv.ParseFloat(d[i:j], 64)
			if err != nil || math.IsNaN(v) || math.Abs(v) > maxPathCoord {
				return "", false
			}
			if k%2 == 0 {
				v += dx
			} else {
				v += dy
			}
			b.Write(appendNum(nil, v))
			i = j
		}
	}
	return b.String(), true
}

// useXY reads the x and y of a <use> (0 when absent). ok is false for a value
// that is not a finite number within maxPathCoord.
func useXY(attrs []attr) (x, y float64, ok bool) {
	for _, a := range attrs {
		if a.k != "x" && a.k != "y" {
			continue
		}
		v, err := strconv.ParseFloat(a.v, 64)
		if err != nil || math.IsNaN(v) || math.Abs(v) > maxPathCoord {
			return 0, 0, false
		}
		if a.k == "x" {
			x = v
		} else {
			y = v
		}
	}
	return x, y, true
}

// appendNum writes v with three decimals, trailing zeros trimmed, "-0" as "0".
func appendNum(b []byte, v float64) []byte {
	n := len(b)
	b = strconv.AppendFloat(b, v, 'f', 3, 64)
	for b[len(b)-1] == '0' {
		b = b[:len(b)-1]
	}
	if b[len(b)-1] == '.' {
		b = b[:len(b)-1]
	}
	if len(b)-n == 2 && b[n] == '-' && b[n+1] == '0' {
		b = append(b[:n], '0')
	}
	return b
}
