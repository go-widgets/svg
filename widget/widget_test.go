// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package widget

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// newBtn returns a Button ready to render at (0, 0, w, h).
func newBtn(w, h int) *toolkit.Button {
	b := toolkit.NewButton("Click me", nil)
	b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})
	return b
}

func TestSnapshotEmitsSVGForToolkitWidget(t *testing.T) {
	var out bytes.Buffer
	n, err := Snapshot(&out, newBtn(120, 32), 120, 32, toolkit.DefaultLight(), "widget: button")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if n == 0 {
		t.Fatal("Snapshot returned 0 bytes")
	}
	s := out.String()
	for _, want := range []string{
		`<?xml version="1.0"`,
		`viewBox="0 0 120 32"`,
		`<title>widget: button</title>`,
		`data:image/png;base64,`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPNGEmitsRawPNGBytes(t *testing.T) {
	var out bytes.Buffer
	n, err := PNG(&out, newBtn(80, 24), 80, 24, toolkit.DefaultLight())
	if err != nil {
		t.Fatalf("PNG: %v", err)
	}
	if n != out.Len() {
		t.Fatalf("byte-count mismatch: return=%d, buffer=%d", n, out.Len())
	}
	b := out.Bytes()
	// PNG magic.
	if len(b) < 8 || b[0] != 0x89 || string(b[1:4]) != "PNG" {
		t.Fatal("PNG magic missing")
	}
	// No SVG envelope leak.
	if bytes.Contains(b, []byte("<svg")) || bytes.Contains(b, []byte("data:image/png")) {
		t.Fatal("PNG bytes should not carry an SVG envelope")
	}
}

func TestSnapshotBadDimensions(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, newBtn(1, 1), 0, 1, toolkit.DefaultLight(), ""); err == nil {
		t.Fatal("width=0 should error")
	}
	if _, err := Snapshot(&out, newBtn(1, 1), 1, -1, toolkit.DefaultLight(), ""); err == nil {
		t.Fatal("height<0 should error")
	}
}

func TestSnapshotNilWidget(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, nil, 10, 10, toolkit.DefaultLight(), ""); err == nil {
		t.Fatal("nil widget should error")
	}
}

func TestSnapshotNilTheme(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, newBtn(10, 10), 10, 10, nil, ""); err == nil {
		t.Fatal("nil theme should error")
	}
}

func TestPNGBadDimensions(t *testing.T) {
	var out bytes.Buffer
	if _, err := PNG(&out, newBtn(1, 1), 0, 1, toolkit.DefaultLight()); err == nil {
		t.Fatal("PNG width=0 should error")
	}
}

func TestPNGNilWidgetAndTheme(t *testing.T) {
	var out bytes.Buffer
	if _, err := PNG(&out, nil, 10, 10, toolkit.DefaultLight()); err == nil {
		t.Fatal("PNG nil widget should error")
	}
	if _, err := PNG(&out, newBtn(10, 10), 10, 10, nil); err == nil {
		t.Fatal("PNG nil theme should error")
	}
}

// failingWriter — writer that errors on the k-th write.
type failingWriter struct {
	k, i int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.i++
	if f.i == f.k {
		return 0, errors.New("boom")
	}
	return len(p), nil
}

func TestSnapshotWriterError(t *testing.T) {
	// svg.Snapshot writes header + body (no label = no title write). Either write
	// path failing must surface an error.
	for _, k := range []int{1, 2} {
		fw := &failingWriter{k: k}
		_, err := Snapshot(fw, newBtn(4, 4), 4, 4, toolkit.DefaultLight(), "")
		if err == nil {
			t.Errorf("Snapshot with failing write #%d: expected error", k)
		}
	}
}

func TestPNGWriterError(t *testing.T) {
	fw := &failingWriter{k: 1}
	_, err := PNG(fw, newBtn(4, 4), 4, 4, toolkit.DefaultLight())
	if err == nil {
		t.Fatal("PNG with failing writer: expected error")
	}
}
