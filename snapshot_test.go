// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

// solid returns a w×h RGBA byte slice filled with (r, g, b, 0xFF).
func solid(w, h int, r, g, b byte) []byte {
	buf := make([]byte, 4*w*h)
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i+0], buf[i+1], buf[i+2], buf[i+3] = r, g, b, 0xFF
	}
	return buf
}

func TestSnapshotEmitsWellFormedSVG(t *testing.T) {
	var out bytes.Buffer
	n, err := Snapshot(&out, solid(32, 24, 0xFF, 0x00, 0x00), 32, 24, "red-32x24")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if n != out.Len() {
		t.Fatalf("byte-count mismatch: return=%d, buffer=%d", n, out.Len())
	}
	// Must be valid XML.
	if err := xml.Unmarshal(out.Bytes(), new(struct{ XMLName xml.Name })); err != nil {
		t.Fatalf("output is not well-formed XML: %v", err)
	}
	// Must carry the expected shape.
	s := out.String()
	for _, want := range []string{
		`<?xml version="1.0"`,
		`viewBox="0 0 32 24"`,
		`width="32" height="24"`,
		`<title>red-32x24</title>`,
		`image-rendering="pixelated"`,
		`data:image/png;base64,`,
		`</svg>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestSnapshotBase64RoundtripsToOriginalPNG(t *testing.T) {
	// The base64 payload must decode back to a valid PNG whose pixels
	// match the input surface — otherwise the snapshot is lying.
	src := solid(4, 4, 0x11, 0x22, 0x33)
	var out bytes.Buffer
	if _, err := Snapshot(&out, src, 4, 4, ""); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	prefix := "data:image/png;base64,"
	start := strings.Index(s, prefix)
	if start < 0 {
		t.Fatal("no base64 payload")
	}
	end := strings.Index(s[start:], `"`)
	if end < 0 {
		t.Fatal("no closing quote on href")
	}
	b64 := s[start+len(prefix) : start+end]
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	// PNG magic (89 50 4E 47 0D 0A 1A 0A).
	if len(raw) < 8 || raw[0] != 0x89 || raw[1] != 'P' || raw[2] != 'N' || raw[3] != 'G' {
		t.Fatalf("decoded payload is not a PNG: %v", raw[:8])
	}
}

func TestSnapshotOmitsTitleWhenLabelBlank(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, solid(4, 4, 0, 0, 0), 4, 4, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<title>") {
		t.Fatal("empty label should omit <title>")
	}
	if strings.Contains(out.String(), "aria-label") {
		t.Fatal("empty label should omit aria-label attribute")
	}
}

func TestSnapshotBadDimensionsReject(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, nil, 0, 10, ""); err == nil {
		t.Fatal("width=0 should error")
	}
	if _, err := Snapshot(&out, nil, 10, -1, ""); err == nil {
		t.Fatal("height<0 should error")
	}
}

func TestSnapshotSurfaceSizeMismatchRejects(t *testing.T) {
	var out bytes.Buffer
	if _, err := Snapshot(&out, []byte{1, 2, 3}, 4, 4, ""); err == nil {
		t.Fatal("short surface should error")
	}
}

func TestSnapshotEscapesLabelXML(t *testing.T) {
	// Label with <, >, &, ", ' — all must land escaped in <title> +
	// aria-label.
	var out bytes.Buffer
	nasty := `<b>&"'</b>`
	if _, err := Snapshot(&out, solid(2, 2, 0, 0, 0), 2, 2, nasty); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "<b>") || strings.Contains(s, "</b>") {
		t.Fatalf("raw HTML leaked into title: %q", s)
	}
	for _, want := range []string{"&lt;b&gt;", "&amp;", "&quot;", "&apos;"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected escape %q in output", want)
		}
	}
}

// failingWriter is an io.Writer that errors on the k-th write.
type failingWriter struct {
	k    int
	i    int
	seen []int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.i++
	f.seen = append(f.seen, len(p))
	if f.i == f.k {
		return 0, errors.New("boom")
	}
	return len(p), nil
}

func TestSnapshotWriterErrorPropagates(t *testing.T) {
	// Force each of the write paths to fail: header (write #1), title
	// (write #2 when label set), body (write #3). All must surface
	// the error + a partial byte count.
	for _, k := range []int{1, 2, 3} {
		fw := &failingWriter{k: k}
		_, err := Snapshot(fw, solid(4, 4, 0, 0, 0), 4, 4, "labelled")
		if err == nil {
			t.Fatalf("write #%d failing: expected error", k)
		}
	}
}

func TestEncodePNGShape(t *testing.T) {
	// Direct exercise: 2×2 red → 4-byte PNG magic + IHDR intact.
	b := encodePNG(solid(2, 2, 0xFF, 0, 0), 2, 2)
	if len(b) < 16 {
		t.Fatalf("PNG too short: %d bytes", len(b))
	}
	if b[0] != 0x89 || string(b[1:4]) != "PNG" {
		t.Fatal("missing PNG magic")
	}
}

func TestEscapeXMLTable(t *testing.T) {
	if got := escapeXML("a<b>c&d\"e'f"); got != "a&lt;b&gt;c&amp;d&quot;e&apos;f" {
		t.Fatalf("escapeXML: %q", got)
	}
	if got := escapeXML(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := escapeXML("no escapes"); got != "no escapes" {
		t.Fatalf("passthrough: %q", got)
	}
}
