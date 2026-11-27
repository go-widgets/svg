// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

var (
	ink  = painter.RGB(240, 240, 240)
	dark = painter.RGB(30, 34, 40)
	blue = painter.RGB(30, 90, 200)
)

// recorded builds one 1000x700 application frame with svgrec: a sidebar of 40
// rows and a clipped main list of 30 rows (both static), plus a live region
// (progress bar and counter) that depends on n. mode "static" draws no live
// region, "all" changes every row too.
func recorded(tb testing.TB, n int, mode string) []byte {
	tb.Helper()
	r, err := svgrec.New(1000, 700)
	if err != nil {
		tb.Fatal(err)
	}
	r.FillRect(painter.Rect{W: 1000, H: 700}, dark)
	for i := 0; i < 40; i++ {
		r.FillRect(painter.Rect{X: 0, Y: i * 17, W: 200, H: 16}, painter.RGB(uint8(40+i), 50, 60))
		r.Text(8, i*17+4, "account "+strconv.Itoa(i), ink)
	}
	r.PushClip(painter.Rect{X: 210, Y: 10, W: 780, H: 520})
	for i := 0; i < 30; i++ {
		shade := uint8(60 + i)
		if mode == "all" {
			shade = uint8(60 + (i+n)%200)
		}
		r.FillRoundRect(painter.Rect{X: 220, Y: 12 + i*17, W: 700, H: 15}, 3, painter.RGB(shade, 70, 90))
		r.Text(230, 15+i*17, "message "+strconv.Itoa(i), ink)
	}
	r.PopClip()
	if mode != "static" {
		r.StrokeRect(painter.Rect{X: 220, Y: 600, W: 600, H: 20}, ink, 1)
		r.FillRect(painter.Rect{X: 221, Y: 601, W: 598 * (n % 61) / 60, H: 18}, blue)
		r.Text(830, 604, strconv.Itoa(n)+" %", ink)
	}
	b, err := r.SVG()
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func scenario(tb testing.TB, frames int, mode string) []Frame {
	fs := make([]Frame, frames)
	for i := range fs {
		fs[i] = Frame{At: time.Duration(i) * 50 * time.Millisecond, SVG: recorded(tb, i, mode)}
	}
	return fs
}

// vocabulary parses data with encoding/xml and fails on anything outside the
// svgrec vocabulary plus the assembler's own (style, defs, use).
func vocabulary(tb testing.TB, data []byte) (elements int) {
	tb.Helper()
	ok := map[string]bool{"svg": true, "g": true, "rect": true, "path": true, "image": true,
		"title": true, "clipPath": true, "defs": true, "use": true, "style": true}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return elements
		}
		if err != nil {
			tb.Fatalf("output does not parse: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			elements++
			if !ok[t.Name.Local] {
				tb.Fatalf("element %q outside the vocabulary", t.Name.Local)
			}
			for _, a := range t.Attr {
				if strings.HasPrefix(strings.ToLower(a.Name.Local), "on") {
					tb.Fatalf("event attribute %q", a.Name.Local)
				}
			}
		case xml.ProcInst, xml.Directive:
			tb.Fatalf("unexpected %T", tok)
		}
	}
}
