// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

var errBoom = errors.New("boom")

func TestEntriesShape(t *testing.T) {
	got := entries()
	if len(got) == 0 {
		t.Fatal("entries returned zero widgets")
	}
	// Every entry must have a non-empty name + positive dims + a
	// non-nil constructor.
	seen := map[string]bool{}
	for _, e := range got {
		if e.Name == "" {
			t.Errorf("entry has empty name: %+v", e)
		}
		if seen[e.Name] {
			t.Errorf("duplicate entry name: %q", e.Name)
		}
		seen[e.Name] = true
		if e.W <= 0 || e.H <= 0 {
			t.Errorf("entry %q has non-positive dims: %d×%d", e.Name, e.W, e.H)
		}
		if e.Make == nil {
			t.Errorf("entry %q has nil Make", e.Name)
		} else if e.Make() == nil {
			t.Errorf("entry %q Make returned nil widget", e.Name)
		}
	}
}

func TestRenderWritesAllPairsIntoDir(t *testing.T) {
	dir := t.TempDir()
	if err := render(dir, toolkit.DefaultLight()); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Every entry must have both a .svg and a .png.
	for _, e := range entries() {
		for _, ext := range []string{".svg", ".png"} {
			p := filepath.Join(dir, e.Name+ext)
			st, err := os.Stat(p)
			if err != nil {
				t.Errorf("missing %s: %v", p, err)
				continue
			}
			if st.Size() == 0 {
				t.Errorf("%s is zero-length", p)
			}
		}
	}
	// Spot-check the button.svg for the expected shape.
	body, err := os.ReadFile(filepath.Join(dir, "button.svg"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "<title>widget: button</title>") {
		t.Fatal("button.svg missing expected <title>")
	}
	// Spot-check the button.png magic.
	pngBytes, err := os.ReadFile(filepath.Join(dir, "button.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pngBytes) < 8 || pngBytes[0] != 0x89 || string(pngBytes[1:4]) != "PNG" {
		t.Fatal("button.png missing PNG magic")
	}
}

// TestRenderPNGShowsTextForTextBearingWidgets guards against the class
// of bug that shipped in the (since-fixed) toolkit v0.6.0..v0.6.1
// window, where Label.Draw and Button.Draw stopped rendering their
// text after the Painter refactor. The existing "file exists + PNG
// magic" checks did not catch it: an empty-body PNG is still a
// well-formed PNG.
//
// The specific bug variant Label hit was that Draw() rendered only a
// stray baseline underline — one row of drawn pixels — instead of
// glyphs across multiple rows. Text at painter.GlyphHeight = 7 covers
// several rows once composed; the tell-tale signal is therefore
// "how many rows contain at least one drawn pixel." This test asserts
// >= 3 distinct drawn-pixel rows on each widget whose entries() Make()
// supplies a non-empty text field.
//
// The "drawn pixel" filter is alpha > 0: svgwidget.render() allocates
// a zeroed RGBA buffer, so pixels the widget never touched still have
// alpha 0 (fully transparent). Anything the widget actually wrote —
// via a painter primitive — lands with alpha 255.
func TestRenderPNGShowsTextForTextBearingWidgets(t *testing.T) {
	dir := t.TempDir()
	theme := toolkit.DefaultLight()
	if err := render(dir, theme); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Widgets whose Make() supplies a non-empty text label — the ones
	// we can meaningfully assert "should contain visible glyph pixels".
	// Anything whose Draw method is expected to write text goes here so
	// a future "Draw forgot to render its text" regression trips one of
	// these entries the same way v0.6.0's Label bug trips the label
	// case.
	textBearing := map[string]bool{
		"button":       true,
		"label":        true,
		"radiobutton":  true, // "Enable option"
		"togglebutton": true, // "Muted"
		"spinbutton":   true, // "42" (numeric value)
		"statusbar":    true, // "Ready" / "Line 42" / "UTF-8"
		"textview":     true, // multi-line prose
		"notification": true, // "Saved successfully"
		"tooltip":      true, // "Undo (Ctrl+Z)"
		"badge":        true, // "42"
		"kbd":          true, // "Ctrl+K"
		"alert":        true, // "Configuration saved successfully."
		"card":         true, // title + body + footer
		"breadcrumbs":  true, // "home > projects > widgets > toolkit"
		"steps":        true, // "Plan", "Build", "Test", "Ship" captions
		"headerbar":    true, // "Files" + "~/Documents"
		"table":        true, // headers + cell text
		// switch is intentionally excluded — knob/track only, no text.
		"avatar":       true, // "DL"
		"rating":       true, // "*" glyphs
		"toast":        true, // "Copied to clipboard"
		"banner":       true, // "Software update available." + "Install"
		"popover":      true, // "Menu" header + "Popover content" child
		"actionrow":    true, // "Language" + "English (US)"
		"viewswitcher": true, // "Inbox" / "Sent" / "Archive"
		"chatbubble":   true, // "Hello, world!"
		"searchentry":  true, // "query" + prefix + clear glyph
		"diff":         true, // 4 diff lines
		"pagination":   true, // "<", "1", "2", "3", "4", "5", ">"
		// skeleton is intentionally excluded — placeholder bars only, no text.
		"splitbutton":    true, // "Deploy" + arrow glyph
		"iconbutton":     true, // "+"
		"stat":           true, // "Requests / min" + "12,845" + "+8.3%"
		"timeline":       true, // event titles + details
		"dropzone":       true, // "Drop files to upload"
		"chip":           true, // "frontend" + close glyph
		"formfield":      true, // "Username" + help caption
		"progresscircle": true, // "66%" centered text
	}
	// A single-row underline (the v0.6.0 label bug) produces exactly
	// one drawn row. Text glyphs at painter.GlyphHeight = 7 produce
	// ~5-7 non-empty rows. Three is comfortably above the underline
	// signal and well below the glyph-mass signal.
	const minDrawnRows = 3
	for _, e := range entries() {
		if !textBearing[e.Name] {
			continue
		}
		rows := countDrawnPixelRows(t, filepath.Join(dir, e.Name+".png"))
		if rows < minDrawnRows {
			t.Fatalf("%s.png has %d rows of drawn (alpha > 0) pixels, want >= %d — widget's text likely regressed to a stray baseline/underline",
				e.Name, rows, minDrawnRows)
		}
	}
}

// countDrawnPixelRows decodes a PNG and returns the number of rows
// containing at least one pixel with alpha > 0 — i.e., a pixel the
// widget's Draw call actually touched. Rows with only alpha-zero
// pixels (fully transparent, buffer never written) don't count.
func countDrawnPixelRows(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	bounds := img.Bounds()
	rows := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				rows++
				break
			}
		}
	}
	return rows
}

func TestRenderErrorOnBadDir(t *testing.T) {
	// Point at a path that can't be created (non-existent parent
	// under an existing regular file).
	tmp := t.TempDir()
	file := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(file, "sub") // "sub" of a regular file → ENOTDIR
	err := render(badDir, toolkit.DefaultLight())
	if err == nil {
		t.Fatal("render into a path under a regular file should error")
	}
}

func TestWriteFileErrorOnUnwritable(t *testing.T) {
	if err := writeFile("/no/such/dir/x", func(*os.File) error { return nil }); err == nil {
		t.Fatal("expected error on unwritable path")
	}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	if err := run([]string{"-out", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "wrote") {
		t.Fatalf("stdout missing summary: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "button.svg")); err != nil {
		t.Fatalf("expected button.svg: %v", err)
	}
}

func TestRunDarkTheme(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	if err := run([]string{"-out", dir, "-theme", "dark"}, &stdout, &stderr); err != nil {
		t.Fatalf("run dark: %v", err)
	}
}

func TestRunFlagParseError(t *testing.T) {
	var stdout, stderr strings.Builder
	if err := run([]string{"-not-a-flag"}, &stdout, &stderr); err == nil {
		t.Fatal("unknown flag should error")
	}
}

func TestRunMkdirError(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	// Point -out at a path under a regular file → mkdir errors.
	if err := run([]string{"-out", filepath.Join(file, "sub")}, &stdout, &stderr); err == nil {
		t.Fatal("mkdir under regular file should error")
	}
}

// TestRunRenderError covers the run() branch where mkdir succeeds
// but the subsequent render() call fails — a read-only output dir
// where MkdirAll(existingDir) is a no-op but writeFile can't create
// files inside.
func TestRunRenderError(t *testing.T) {
	parent := t.TempDir()
	subdir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(subdir, 0o500); err != nil {
		t.Fatal(err)
	}
	// Restore write perm so t.TempDir cleanup can remove the subdir.
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o700) })
	var stdout, stderr strings.Builder
	if err := run([]string{"-out", subdir}, &stdout, &stderr); err == nil {
		t.Fatal("run into a read-only dir should surface a render error")
	}
}

// TestMainSuccessPath drives main() through the runFunc/osExit seams so
// the fatal branch is not actually taken and main() itself gets covered.
func TestMainSuccessPath(t *testing.T) {
	origRun, origExit := runFunc, osExit
	defer func() { runFunc, osExit = origRun, origExit }()
	exited := false
	runFunc = func([]string, io.Writer, io.Writer) error { return nil }
	osExit = func(int) { exited = true }
	main()
	if exited {
		t.Fatal("main() should not have called osExit on success")
	}
}

func TestMainErrorPath(t *testing.T) {
	origRun, origExit := runFunc, osExit
	defer func() { runFunc, osExit = origRun, origExit }()
	got := -1
	runFunc = func([]string, io.Writer, io.Writer) error { return errBoom }
	osExit = func(code int) { got = code }
	main()
	if got != 1 {
		t.Fatalf("main() called osExit(%d), want 1", got)
	}
}
