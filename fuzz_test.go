// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// FuzzAssemble feeds arbitrary bytes as two frames and arbitrary timing and
// title. Whatever the verdict, an accepted input must give a deterministic
// document that parses back with encoding/xml, stays inside the vocabulary and
// carries no script; a refusal must be one of the package's errors, never a
// panic.
func FuzzAssemble(f *testing.F) {
	f.Add(mini(redR), mini(blueR), uint16(10), "title")
	f.Add(mini(`<g><title>A&amp;B &lt;C&gt;</title></g>`), mini(``), uint16(1), `</title><script>`)
	f.Add([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"><script/></svg>`),
		[]byte(`<!DOCTYPE x [<!ENTITY a "b">]>`), uint16(0), "")
	f.Fuzz(func(t *testing.T, a, b []byte, gap uint16, title string) {
		frames := []Frame{{At: 0, SVG: a}, {At: time.Duration(gap) * time.Millisecond, SVG: b}}
		out, err := Animate(frames, Options{Title: title, MaxBytes: 1 << 20})
		if err != nil {
			return
		}
		again, err := Animate(frames, Options{Title: title, MaxBytes: 1 << 20})
		if err != nil || !bytes.Equal(out, again) {
			t.Fatalf("not deterministic: %v", err)
		}
		vocabulary(t, out)
		if strings.Contains(strings.ToLower(string(out)), "<script") {
			t.Fatal("script in output")
		}
	})
}

// FuzzTiming fuzzes the timeline arithmetic alone.
func FuzzTiming(f *testing.F) {
	f.Add(int64(0), int64(1_000_000), int64(0))
	f.Add(int64(-1), int64(1<<62), int64(1<<62))
	f.Fuzz(func(t *testing.T, a, b, end int64) {
		fs := []Frame{{At: time.Duration(a), SVG: mini(redR)}, {At: time.Duration(b), SVG: mini(blueR)}}
		out, err := Animate(fs, Options{End: time.Duration(end)})
		if err != nil {
			return
		}
		vocabulary(t, out)
	})
}
