// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package widget wraps the go-widgets/toolkit `Widget` interface with
// two write-to-io.Writer conveniences:
//
//	widget.Snapshot(w, wg, width, height, theme, label)  // SVG envelope + base64 PNG
//	widget.PNG(w, wg, width, height, theme)              // raw PNG bytes
//
// Both allocate a fresh width×height RGBA buffer, invoke wg.Draw
// against it, then serialise. Rendering + serialisation are one
// function call for the caller — no manual `make([]byte, 4*w*h)` +
// `wg.Draw()` dance for the common "I want a snapshot" case.
//
// Splitting the toolkit-aware convenience into a subpackage keeps
// the root svg package dep-free — a consumer that only needs
// `svg.Snapshot([]byte)` for a home-grown pixel producer does not
// pull go-widgets/toolkit into its module graph.
package widget

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/svg"
	"github.com/go-widgets/toolkit"
)

// Snapshot renders wg into a fresh width×height RGBA surface via a
// painter.PixelPainter that wg.Draw writes through, then wraps the
// result in an SVG envelope via svg.Snapshot. The label lands in
// the SVG's <title> (pass "" to omit).
//
// Returns the number of bytes written + the first error, mirroring
// the io.Writer convention.
func Snapshot(w io.Writer, wg toolkit.Widget, width, height int, theme *toolkit.Theme, label string) (int, error) {
	surface, err := render(wg, width, height, theme)
	if err != nil {
		return 0, err
	}
	return svg.Snapshot(w, surface, width, height, label)
}

// PNG renders wg into a fresh width×height RGBA surface, then writes
// the surface as a raw PNG (no SVG envelope). Useful when the
// caller wants the bit-exact pixels + none of the vector wrapper —
// typical use is a docs-site preprocessor that saves one PNG per
// widget or a CI regression harness that hashes the byte stream.
func PNG(w io.Writer, wg toolkit.Widget, width, height int, theme *toolkit.Theme) (int, error) {
	surface, err := render(wg, width, height, theme)
	if err != nil {
		return 0, err
	}
	img := &image.RGBA{
		Pix:    surface,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return w.Write(buf.Bytes())
}

// render allocates a fresh RGBA surface, invokes wg.Draw against
// it, and returns the buffer. Shared by Snapshot + PNG so the
// validation + allocation lives in one spot.
func render(wg toolkit.Widget, width, height int, theme *toolkit.Theme) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("svg/widget: dimensions must be > 0 (got %dx%d)", width, height)
	}
	if wg == nil {
		return nil, fmt.Errorf("svg/widget: widget is nil")
	}
	if theme == nil {
		return nil, fmt.Errorf("svg/widget: theme is nil")
	}
	surface := make([]byte, 4*width*height)
	wg.Draw(painter.NewPixelPainter(surface, width, height), theme)
	return surface, nil
}
