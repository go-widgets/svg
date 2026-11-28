// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

// glyphFrame is what svgrec.TextGlyphs writes: a symbol per distinct outline in
// <defs>, a <use> per glyph.
func glyphFrame(defs string, uses string) []byte {
	return mini(`<defs>` + defs + `</defs><g fill="#000000"><title>ab</title>` + uses + `</g>`)
}

const (
	symA = `<symbol id="gaaaa" overflow="visible"><path d="M0 0L3 0L3 3Z"/></symbol>`
	symB = `<symbol id="gbbbb" overflow="visible"><path d="M0 0L4 0Q4 4 0 4Z"/></symbol>`
)

func TestGlyphSymbolsAreSharedAcrossFrames(t *testing.T) {
	// Two frames from different recorders name the same outline differently;
	// the assembly defines it once, under its own id.
	f1 := glyphFrame(strings.Replace(symA, "gaaaa", "g1111", 1), `<use href="#g1111" x="1" y="2"/>`)
	f2 := glyphFrame(symA+symB, `<use href="#gaaaa" x="5" y="6"/><use href="#gbbbb" x="7" y="8"/>`)
	doc, err := Assemble([]Frame{{At: 0, SVG: f1}, {At: 100e6, SVG: f2}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	vocabulary(t, doc.SVG())
	s := string(doc.SVG())
	if got := strings.Count(s, "<symbol "); got != 2 || doc.Stats().Symbols != 2 {
		t.Fatalf("%d symbols, stats %+v\n%s", got, doc.Stats(), s)
	}
	for _, want := range []string{`<symbol id="y0" overflow="visible">`, `<use href="#y0" x="1" y="2"/>`,
		`<use href="#y0" x="5" y="6"/>`, `<use href="#y1" x="7" y="8"/>`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q\n%s", want, s)
		}
	}
	if strings.Contains(s, "g1111") || strings.Contains(s, "gaaaa") {
		t.Errorf("an original id leaked\n%s", s)
	}
}

func TestGlyphStillExpandsUseToTheShiftedPath(t *testing.T) {
	f := glyphFrame(symA, `<use href="#gaaaa" x="1.5" y="-2"/>`)
	doc, err := Assemble([]Frame{{At: 0, SVG: f}, {At: 1e8, SVG: mini(redR)}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	still, err := doc.Frame(0)
	if err != nil {
		t.Fatal(err)
	}
	s := string(still)
	if strings.Contains(s, "<use") || strings.Contains(s, "<symbol") || strings.Contains(s, "<defs") {
		t.Fatalf("a still must not reference anything\n%s", s)
	}
	if !strings.Contains(s, `<path d="M1.5 -2L4.5 -2L4.5 1Z"/>`) {
		t.Fatalf("expansion wrong\n%s", s)
	}
	if _, err := parseFrame(still); err != nil { // a still is a valid frame
		t.Fatal(err)
	}
}

func TestGlyphRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"missing symbol":     `<g><use href="#nope" x="0" y="0"/></g>`,
		"use outside":        `<use href="gaaaa"/>`,
		"defs in g":          `<g><defs>` + symA + `</defs></g>`,
		"symbol outside":     symA,
		"rect in defs":       `<defs><rect x="0" y="0" width="1" height="1"/></defs>`,
		"rect in symbol":     `<defs><symbol id="a"><rect x="0" y="0" width="1" height="1"/></symbol></defs>`,
		"two paths":          `<defs><symbol id="a"><path d="M0 0Z"/><path d="M0 0Z"/></symbol></defs>`,
		"path without d":     `<defs><symbol id="a"><path/></symbol></defs>`,
		"empty symbol":       `<defs><symbol id="a"></symbol></defs>`,
		"no id":              `<defs><symbol><path d="M0 0Z"/></symbol></defs>`,
		"dup id":             `<defs>` + symA + symA + `</defs>`,
		"relative path":      `<defs><symbol id="a"><path d="m0 0z"/></symbol></defs>`,
		"arc path":           `<defs><symbol id="a"><path d="M0 0A1 1 0 0 0 1 1Z"/></symbol></defs>`,
		"huge coordinate":    `<defs><symbol id="a"><path d="M0 99999999Z"/></symbol></defs>`,
		"overflow hidden":    `<defs><symbol id="a" overflow="hidden"><path d="M0 0Z"/></symbol></defs>`,
		"overflow on g":      `<g overflow="visible"/>`,
		"id on g":            `<g id="a"/>`,
		"use with kids":      symA + `<use href="#gaaaa"><title>x</title></use>`,
		"bad x":              `<defs>` + symA + `</defs><use href="#gaaaa" x="abc"/>`,
		"huge x":             `<defs>` + symA + `</defs><use href="#gaaaa" x="1e12"/>`,
		"href without #":     `<defs>` + symA + `</defs><use href="gaaaa" x="0" y="0"/>`,
		"external href":      `<defs>` + symA + `</defs><use href="http://x/#a"/>`,
		"symbol in symbol":   `<defs><symbol id="a"><symbol id="b"><path d="M0 0Z"/></symbol></symbol></defs>`,
		"inner use in defs":  `<defs><use href="#a"/></defs>`,
		"path data garbage":  `<defs><symbol id="a"><path d="M0 0 L1"/></symbol></defs>`,
		"path data trailing": `<defs><symbol id="a"><path d="M0 0Z "/></symbol></defs>`,
	} {
		if _, err := parseFrame(mini(body)); !errors.Is(err, ErrFrame) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

// The parser guarantees every <use> of a document resolves, so expand's own
// fallbacks are unreachable through Assemble; they hold on their own: a use it
// cannot resolve is left as markup, never expanded to something else.
func TestExpandLeavesWhatItCannotResolve(t *testing.T) {
	r := &renderer{syms: []string{"M0 0L1 1Z", "m0 0z"}}
	for name, c := range map[string]*cnode{
		"no href":       {name: "use", attrs: []attr{{"x", "1"}}},
		"out of range":  {name: "use", attrs: []attr{{"href", "#y2"}}},
		"not a number":  {name: "use", attrs: []attr{{"href", "#yx"}}},
		"bad position":  {name: "use", attrs: []attr{{"href", "#y0"}, {"x", "abc"}}},
		"bad path data": {name: "use", attrs: []attr{{"href", "#y1"}}},
	} {
		r.buf.Reset()
		if r.expand(c) || r.buf.Len() != 0 {
			t.Errorf("%s: expanded to %q", name, r.buf.String())
		}
	}
	r.buf.Reset()
	if !r.expand(&cnode{name: "use", attrs: []attr{{"href", "#y0"}, {"x", "2"}, {"y", "3"}}}) || r.buf.String() != `<path d="M2 3L3 4Z"/>` {
		t.Errorf("expansion %q", r.buf.String())
	}
}

func TestShiftPath(t *testing.T) {
	for _, tc := range []struct {
		d      string
		dx, dy float64
		want   string
		ok     bool
	}{
		{"M0 0L1 2Z", 0, 0, "M0 0L1 2Z", true},
		{"M0 0L1 2Z", 0.5, -1, "M0.5 -1L1.5 1Z", true},
		{"M1 1Q2 2 3 3C4 4 5 5 6 6Z", 1, 1, "M2 2Q3 3 4 4C5 5 6 6 7 7Z", true},
		{"M-0.0004 0Z", 0, 0, "M0 0Z", true}, // never "-0"
		{"", 0, 0, "", true},
		{"M0 0 ", 0, 0, "", false},
		{"M0", 0, 0, "", false},
		{"M0  0", 0, 0, "", false},
		{"L1 2 3 4", 0, 0, "", false},
		{"Z1", 0, 0, "", false},
		{"M1e3 0", 0, 0, "", false},
		{"M99999999 0", 0, 0, "", false},
		{"Mx 0", 0, 0, "", false},
	} {
		got, ok := shiftPath(tc.d, tc.dx, tc.dy)
		if ok != tc.ok || got != tc.want {
			t.Errorf("shiftPath(%q, %v, %v) = %q, %v; want %q, %v", tc.d, tc.dx, tc.dy, got, ok, tc.want, tc.ok)
		}
	}
}

// Shifting is exact to the three decimals written: shifting by (dx, dy) and back
// gives the original within one unit of the last decimal.
func TestShiftPathRoundTrips(t *testing.T) {
	d := "M0.125 -3.5L10.001 2.25Q1 1 2 2C3 3 4 4 5.5 6Z"
	for _, s := range [][2]float64{{0, 0}, {17.375, -9.5}, {1000.123, 2000.456}, {-5, 5}} {
		moved, ok := shiftPath(d, s[0], s[1])
		if !ok {
			t.Fatal("shift refused")
		}
		back, ok := shiftPath(moved, -s[0], -s[1])
		if !ok {
			t.Fatal("back refused")
		}
		a, b := numbers(d), numbers(back)
		if len(a) != len(b) {
			t.Fatalf("%q vs %q", d, back)
		}
		for i := range a {
			if math.Abs(a[i]-b[i]) > 0.0011 {
				t.Errorf("shift %v: number %d is %v, was %v", s, i, b[i], a[i])
			}
		}
	}
}

func numbers(d string) []float64 {
	var out []float64
	for _, f := range strings.FieldsFunc(d, func(r rune) bool { return r >= 'A' && r <= 'Z' || r == ' ' }) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			panic(err)
		}
		out = append(out, v)
	}
	return out
}

func TestGlyphAssemblyIsDeterministic(t *testing.T) {
	fs := []Frame{
		{At: 0, SVG: glyphFrame(symA+symB, `<use href="#gaaaa" x="1" y="2"/><use href="#gbbbb" x="3" y="4"/>`)},
		{At: 1e8, SVG: glyphFrame(symB, `<use href="#gbbbb" x="3" y="4"/>`)},
	}
	a, err := Animate(fs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Animate(fs, Options{})
	if string(a) != string(b) {
		t.Fatal("two assemblies differ")
	}
}
