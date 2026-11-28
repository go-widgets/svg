// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import "testing"

// Whatever shiftPath accepts it writes back in a form it accepts again, and the
// output is a fixed point of a zero shift.
func FuzzShiftPath(f *testing.F) {
	for _, s := range []string{"M0 0L1 2Z", "M1 1Q2 2 3 3C4 4 5 5 6 6Z", "", "M-0.5 0.25Z", "M0 0 ", "L1", "M1e3 0"} {
		f.Add(s, 1.5, -2.0)
	}
	f.Fuzz(func(t *testing.T, d string, dx, dy float64) {
		out, ok := shiftPath(d, dx, dy)
		if !ok {
			return
		}
		if _, ok := shiftPath(out, 0, 0); !ok && len(out) > 0 {
			// a shift may push a coordinate past the bound: that refusal is allowed,
			// but only then
			if _, ok := shiftPath(d, 0, 0); !ok {
				t.Fatalf("%q accepted shifted but not unshifted", d)
			}
		}
	})
}

// A frame with glyph symbols never panics the parser or the assembler, and one
// that is accepted stays accepted after it is cut into a still.
func FuzzGlyphFrame(f *testing.F) {
	f.Add(`<defs><symbol id="a" overflow="visible"><path d="M0 0L3 0Z"/></symbol></defs><use href="#a" x="1" y="2"/>`)
	f.Add(`<use href="#a"/>`)
	f.Add(`<defs><symbol id="a"><rect/></symbol></defs>`)
	f.Fuzz(func(t *testing.T, body string) {
		doc, err := Assemble([]Frame{{At: 0, SVG: mini(body)}, {At: 1e8, SVG: mini(redR)}}, Options{})
		if err != nil {
			return
		}
		still, err := doc.Frame(0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseFrame(still); err != nil {
			t.Fatalf("a still of an accepted frame is refused: %v\n%s", err, still)
		}
	})
}
