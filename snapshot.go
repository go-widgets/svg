// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package svg turns a rendered RGBA byte buffer (the output of any
// go-widgets/toolkit widget composition) into a portable SVG document
// that embeds the pixels as a base64-encoded PNG. The wrapper scales
// crisply at any zoom level and travels well in READMEs, GitHub
// issues, docs sites, tweet threads and PDF exports.
//
// Why not a "true" vector renderer? Widgets in go-widgets/toolkit are
// pixel-blitting by design — every draw call writes bytes into a
// []byte at (x, y). Converting that pipeline to <rect>/<text>/etc.
// would either double every widget's implementation or lose the exact
// alignment users see in-browser. Embedding the render as PNG is
// bit-exact (any pixel you see in the browser is the same pixel in
// the SVG) while still giving you the "scales to any DPI" benefit +
// the "one file, no external assets" portability.
//
// Typical use in a doc-generation script:
//
//	surface := make([]byte, 4*w*h)
//	btn := toolkit.NewButton("Click me", nil)
//	btn.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})
//	btn.Draw(surface, w, toolkit.DefaultLight())
//	f, _ := os.Create("button.svg")
//	svg.Snapshot(f, surface, w, h, "widget: button")
package svg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
)

// Snapshot writes an SVG document that wraps the RGBA pixels of a
// widget render. width + height are the buffer's dimensions; label
// is a short human-readable string embedded as the SVG's
// <title>/<desc> for a11y + previewer tooltips (pass "" to omit).
//
// The output is a single, self-contained SVG (no external assets)
// with a <image> child pointing at data:image/png;base64,… so it
// pastes cleanly into any Markdown renderer that already accepts
// SVG images (GitHub, GitLab, mkdocs, Hugo, ...).
//
// Returns the number of bytes written + the first error, mirroring
// the io.Writer convention.
func Snapshot(w io.Writer, surface []byte, width, height int, label string) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, fmt.Errorf("svg.Snapshot: dimensions must be > 0 (got %dx%d)", width, height)
	}
	if got, want := len(surface), 4*width*height; got != want {
		return 0, fmt.Errorf("svg.Snapshot: surface has %d bytes, want %d (4*%d*%d)", got, want, width, height)
	}
	pngBytes := encodePNG(surface, width, height)
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	return writeSVG(w, width, height, label, b64)
}

// encodePNG converts an RGBA byte slice into a lossless PNG. Never
// errors: png.Encode's only failure mode is the underlying
// io.Writer, and bytes.Buffer.Write is infallible. The error return
// is explicitly discarded so a future stdlib regression would fail
// tests (via the payload validation in
// TestSnapshotBase64RoundtripsToOriginalPNG) rather than propagate
// silently.
func encodePNG(surface []byte, width, height int) []byte {
	img := &image.RGBA{
		Pix:    surface,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// writeSVG emits the SVG envelope + the embedded <image>. viewBox is
// pinned to the pixel dimensions so the SVG scales crisply and the
// paint stays crisp at any zoom (nearest-neighbour via
// image-rendering="pixelated" — matches the toolkit's pixel-perfect
// widget draws).
func writeSVG(w io.Writer, width, height int, label, b64png string) (int, error) {
	total := 0
	write := func(s string) error {
		n, err := io.WriteString(w, s)
		total += n
		return err
	}
	header := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img"`,
		width, height, width, height,
	)
	if label != "" {
		// XML-escape the attribute value; %q would produce a
		// Go-quoted string that leaks raw '<' / '>' / '&' into the
		// SVG (breaks aria + fails XML validators on nasty labels).
		header += ` aria-label="` + escapeXML(label) + `"`
	}
	header += ">\n"
	if err := write(header); err != nil {
		return total, err
	}
	if label != "" {
		if err := write(fmt.Sprintf("  <title>%s</title>\n  <desc>Rendered by go-widgets/toolkit; snapshotted via go-widgets/svg.</desc>\n", escapeXML(label))); err != nil {
			return total, err
		}
	}
	body := fmt.Sprintf(
		`  <image width="%d" height="%d" image-rendering="pixelated" href="data:image/png;base64,%s"/>`+"\n"+`</svg>`+"\n",
		width, height, b64png,
	)
	if err := write(body); err != nil {
		return total, err
	}
	return total, nil
}

// escapeXML escapes the five predefined XML entities. Used on the
// user-supplied label since it lands inside <title> + as an
// aria-label attribute.
func escapeXML(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
