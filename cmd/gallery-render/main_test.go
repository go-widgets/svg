// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

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
