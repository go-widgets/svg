// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"errors"
	"testing"
)

// This file sorts first on purpose (a_ prefix). Thirteen guards in animate.go,
// parse.go, render.go and video.go protect an index or a nil dereference: with
// the guard deleted the code crashes, and mutsweep could only say "caught by
// PANIC" because no assertion named the guard. Each case below calls the guarded
// path under recover and asserts the value on BOTH sides, so a deleted guard
// fails by assertion (t.Errorf), not by luck.

// noPanic runs f, reporting a panic as a failure of the named guard.
func noPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: panicked: %v", name, r)
		}
	}()
	f()
}

func TestAPanicAssertGuards(t *testing.T) {
	cp := `<clipPath id="c0"><rect x="0" y="0" width="5" height="5"/></clipPath>`
	good := []Frame{at(0, redR), at(100, blueR), at(200, redR)}

	noPanic(t, "animate.go Animate error path", func() {
		out, err := Animate(nil, Options{})
		if !errors.Is(err, ErrNoFrames) || out != nil {
			t.Errorf("Animate(nil): %q, %v", out, err)
		}
		out, err = Animate(good, Options{})
		if err != nil || len(out) == 0 {
			t.Errorf("Animate(good): %d bytes, %v", len(out), err)
		}
	})
	noPanic(t, "animate.go Assemble checkInput / no frames", func() {
		doc, err := Assemble([]Frame{}, Options{})
		if !errors.Is(err, ErrNoFrames) || doc != nil {
			t.Errorf("Assemble(empty): %v, %v", doc, err)
		}
	})
	noPanic(t, "animate.go Assemble bad frame", func() {
		doc, err := Assemble([]Frame{at(0, redR), {At: 100e6, SVG: []byte("<svg")}}, Options{})
		if !errors.Is(err, ErrFrame) || doc != nil {
			t.Errorf("Assemble(bad frame): %v, %v", doc, err)
		}
	})
	noPanic(t, "animate.go repeated picture is shared", func() {
		doc, err := Assemble(good, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if s := doc.Stats(); s.Pictures != 2 || s.Slots != 3 || s.Frames != 3 {
			t.Errorf("stats %+v, want 2 pictures in 3 slots", s)
		}
	})
	noPanic(t, "parse.go no root element", func() {
		for _, in := range []string{"", "  \n", "<!-- only a comment -->"} {
			if ft, err := parseFrame([]byte(in)); !errors.Is(err, ErrFrame) || ft != nil {
				t.Errorf("parseFrame(%q): %v, %v", in, ft, err)
			}
		}
	})
	noPanic(t, "parse.go start element refused", func() {
		if ft, err := parseFrame([]byte(`<foo/>`)); !errors.Is(err, ErrFrame) || ft != nil {
			t.Errorf("parseFrame(<foo/>): %v, %v", ft, err)
		}
		if ft, err := parseFrame(mini(`<script/>`)); !errors.Is(err, ErrFrame) || ft != nil {
			t.Errorf("parseFrame(script): %v, %v", ft, err)
		}
	})
	noPanic(t, "parse.go root then children", func() {
		ft, err := parseFrame(mini(`<g><rect x="1" y="2" width="3" height="4"/></g>`))
		if err != nil || len(ft.kids) != 1 || len(ft.kids[0].kids) != 1 {
			t.Errorf("parseFrame(nested): %+v, %v", ft, err)
		}
	})
	noPanic(t, "parse.go text outside the root", func() {
		for _, in := range []string{"x" + head + tail, head + tail + "x"} {
			if ft, err := parseFrame([]byte(in)); !errors.Is(err, ErrFrame) || ft != nil {
				t.Errorf("parseFrame(%q): %v, %v", in, ft, err)
			}
		}
		if _, err := parseFrame([]byte(" \n" + head + tail + "\n")); err != nil {
			t.Errorf("whitespace around the root: %v", err)
		}
	})
	noPanic(t, "parse.go clip-path reference", func() {
		if _, err := parseFrame(mini(`<g clip-path="url(#nope)"/>`)); !errors.Is(err, ErrFrame) {
			t.Errorf("missing clip: %v", err)
		}
		ft, err := parseFrame(mini(cp + `<g clip-path="url(#c0)"/>`))
		if err != nil || len(ft.kids) != 1 || len(ft.clips) != 1 {
			t.Errorf("resolved clip: %+v, %v", ft, err)
		}
		if _, err := parseFrame(mini(`<g><g clip-path="url(#nope)"/></g>`)); !errors.Is(err, ErrFrame) {
			t.Errorf("nested missing clip: %v", err)
		}
	})
	noPanic(t, "render.go Frame index", func() {
		doc, err := Assemble(good, Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range []int{-1, 3, 1 << 30} {
			if b, err := doc.Frame(i); !errors.Is(err, ErrSlot) || b != nil {
				t.Errorf("Frame(%d): %q, %v", i, b, err)
			}
		}
		for _, i := range []int{0, 1, 2} {
			if b, err := doc.Frame(i); err != nil || len(b) == 0 {
				t.Errorf("Frame(%d): %d bytes, %v", i, len(b), err)
			}
		}
	})
	noPanic(t, "video.go dimensions", func() {
		doc, err := Assemble(good, Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range [][2]int{{0, 1}, {1, 0}, {-5, 10}, {MaxDim + 1, 1}, {1, MaxDim + 1}} {
			e := &fakeEnc{}
			if err := doc.Video(&bytes.Buffer{}, d[0], d[1], &fakeRast{}, e); !errors.Is(err, ErrVideo) || len(e.calls) != 0 {
				t.Errorf("Video(%v): %v, encoder calls %v", d, err, e.calls)
			}
		}
	})
}
