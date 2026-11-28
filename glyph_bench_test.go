// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// An animation of a text page: 30 frames, each 20 lines of 40 glyphs out of an
// alphabet of 26 outlines of 24 curves, one line changing per frame. "Before" is
// a frame as a recorder wrote it without glyph symbols (an absolute <path> per
// glyph); "after" is the same frames with <symbol>/<use>. Both go through
// Assemble. The bytes are exact; the times are indicative.

func outline(k int) []float64 {
	var v []float64
	for i := 0; i < 24; i++ {
		a := float64(i+k) * 0.37
		v = append(v, 3+4*math.Sin(a), -7+5*math.Cos(a), 0.25*float64(i%9)+1.125*float64(k%5), -6.5+0.7*float64(i%7))
	}
	return v
}

func pathD(k int, ox, oy float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "M%s %s", appendNum(nil, ox+0.5), appendNum(nil, oy))
	v := outline(k)
	for i := 0; i < len(v); i += 4 {
		fmt.Fprintf(&b, "Q%s %s %s %s", appendNum(nil, ox+v[i]), appendNum(nil, oy+v[i+1]), appendNum(nil, ox+v[i+2]), appendNum(nil, oy+v[i+3]))
	}
	b.WriteString("Z")
	return b.String()
}

func textFrames(symbols bool) []Frame {
	var fs []Frame
	for f := 0; f < 30; f++ {
		var defs, body strings.Builder
		if symbols {
			for k := 0; k < 26; k++ {
				fmt.Fprintf(&defs, `<symbol id="g%02d" overflow="visible"><path d="%s"/></symbol>`, k, pathD(k, 0, 0))
			}
		}
		for l := 0; l < 20; l++ {
			body.WriteString(`<g fill="#14141e"><title>line</title>`)
			for i := 0; i < 40; i++ {
				k := (l*7 + i*3 + f*boolInt(l == f%20)) % 26
				x, y := 8+float64(i)*7.25, 14+float64(l)*12
				if symbols {
					fmt.Fprintf(&body, `<use href="#g%02d" x="%s" y="%s"/>`, k, appendNum(nil, x), appendNum(nil, y))
				} else {
					fmt.Fprintf(&body, `<path d="%s"/>`, pathD(k, x, y))
				}
			}
			body.WriteString(`</g>`)
		}
		pre := ""
		if symbols {
			pre = "<defs>" + defs.String() + "</defs>"
		}
		fs = append(fs, Frame{At: time.Duration(f) * 100 * time.Millisecond,
			SVG: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="260" viewBox="0 0 320 260">` + pre + body.String() + `</svg>`)})
	}
	return fs
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func benchText(b *testing.B, symbols bool) {
	fs := textFrames(symbols)
	in := 0
	for _, f := range fs {
		in += len(f.SVG)
	}
	b.ReportAllocs()
	var out []byte
	for i := 0; i < b.N; i++ {
		var err error
		if out, err = Animate(fs, Options{}); err != nil {
			b.Fatal(err)
		}
	}
	var z bytes.Buffer
	w, _ := gzip.NewWriterLevel(&z, 9)
	_, _ = w.Write(out)
	_ = w.Close()
	b.ReportMetric(float64(in), "in-bytes")
	b.ReportMetric(float64(len(out)), "out-bytes")
	b.ReportMetric(float64(z.Len()), "gzip-bytes")
}

func BenchmarkAnimateTextPerGlyphPaths(b *testing.B) { benchText(b, false) }
func BenchmarkAnimateTextSymbols(b *testing.B)       { benchText(b, true) }

// The two forms assemble to documents that differ only by the glyph markup, and
// a still of the symbol form carries the same paths as the path form, to the
// three decimals written.
func TestTextFramesStillsMatchPerGlyphPaths(t *testing.T) {
	a, err := Assemble(textFrames(false)[:3], Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Assemble(textFrames(true)[:3], Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		fa, _ := a.Frame(i)
		fs, _ := s.Frame(i)
		da, ds := pathDatas(string(fa)), pathDatas(string(fs))
		if len(da) != len(ds) || len(da) != 800 {
			t.Fatalf("frame %d: %d paths vs %d", i, len(da), len(ds))
		}
		for j := range da {
			na, ns := numbers(da[j]), numbers(ds[j])
			if len(na) != len(ns) || strings.Map(func(r rune) rune {
				if r >= 'A' && r <= 'Z' {
					return r
				}
				return -1
			}, da[j]) != strings.Map(func(r rune) rune {
				if r >= 'A' && r <= 'Z' {
					return r
				}
				return -1
			}, ds[j]) {
				t.Fatalf("frame %d path %d: shapes differ", i, j)
			}
			for n := range na {
				if math.Abs(na[n]-ns[n]) > 0.0011 {
					t.Fatalf("frame %d path %d number %d: %v vs %v", i, j, n, na[n], ns[n])
				}
			}
		}
	}
}

// pathDatas lists the d attribute of every <path> of a still.
func pathDatas(s string) []string {
	var out []string
	for _, p := range strings.Split(s, `<path d="`)[1:] {
		out = append(out, p[:strings.IndexByte(p, '"')])
	}
	return out
}
