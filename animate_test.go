// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata goldens")

const (
	head = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 10 10">`
	tail = `</svg>`
)

func mini(body string) []byte { return []byte(head + body + tail) }

func at(ms int, body string) Frame {
	return Frame{At: time.Duration(ms) * time.Millisecond, SVG: mini(body)}
}

const (
	redR  = `<rect x="0" y="0" width="5" height="5" fill="#ff0000"/>`
	blueR = `<rect x="5" y="5" width="5" height="5" fill="#0000ff"/>`
)

func TestAssembleTimeline(t *testing.T) {
	frames := []Frame{at(0, redR), at(100, blueR), at(300, redR)}
	doc, err := Assemble(frames, Options{End: 400 * time.Millisecond, Title: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc.SVG())
	vocabulary(t, doc.SVG())
	for _, want := range []string{
		"animation-duration:400ms", "steps(1,end)", "animation-iteration-count:infinite",
		".a0{visibility:visible;animation-name:a0}",
		"@keyframes a0{0%{visibility:visible}25%{visibility:hidden}75%{visibility:visible}}",
		"@keyframes a1{0%{visibility:hidden}25%{visibility:visible}75%{visibility:hidden}}",
		"prefers-reduced-motion", "<title>demo</title>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	st := doc.Stats()
	if st.Frames != 3 || st.Slots != 3 || st.Pictures != 2 || doc.Duration() != 400*time.Millisecond {
		t.Errorf("stats %+v", st)
	}
	sl := doc.Slots()
	if sl[1].Start != 100*time.Millisecond || sl[1].Duration != 200*time.Millisecond || sl[2].Picture != 0 {
		t.Errorf("slots %+v", sl)
	}
	sl[0].Start = 99 // a copy
	if doc.Slots()[0].Start != 0 {
		t.Error("Slots must return a copy")
	}
}

func TestIdenticalNeighboursMerge(t *testing.T) {
	doc, err := Assemble([]Frame{at(0, redR), at(40, redR), at(80, blueR)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Slots(); len(got) != 2 || got[0].Duration != 80*time.Millisecond {
		t.Errorf("slots %+v", got)
	}
	// default end: the last frame lasts as long as the one before it (40 ms)
	if doc.Duration() != 120*time.Millisecond {
		t.Errorf("duration %v", doc.Duration())
	}
}

func TestSingleFrameIsStatic(t *testing.T) {
	out, err := Animate([]Frame{at(5, redR)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<style>") || strings.Contains(s, "@keyframes") {
		t.Errorf("static document animates:\n%s", s)
	}
	vocabulary(t, out)
}

func TestFirstFrameOffsetIsIrrelevant(t *testing.T) {
	a, _ := Animate([]Frame{at(0, redR), at(50, blueR)}, Options{})
	b, _ := Animate([]Frame{at(1000, redR), at(1050, blueR)}, Options{})
	if !bytes.Equal(a, b) {
		t.Error("origin of the timestamps changed the output")
	}
}

func TestDeterministicAndGolden(t *testing.T) {
	frames := scenario(t, 6, "live")
	a, err := Animate(frames, Options{Title: "golden"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Animate(scenario(t, 6, "live"), Options{Title: "golden"})
	if !bytes.Equal(a, b) {
		t.Fatal("two assemblies of the same input differ")
	}
	vocabulary(t, a)
	const path = "testdata/live.svg"
	if *update {
		if err := os.WriteFile(path, a, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, want) {
		t.Errorf("output differs from %s (rerun with -update if intended)", path)
	}
}

// TestRoundTrip: a still frame re-assembled alone is the same document as the
// original frame assembled alone, so Frame loses nothing.
func TestRoundTrip(t *testing.T) {
	frames := scenario(t, 4, "live")
	doc, err := Assemble(frames, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Slots() {
		still, err := doc.Frame(i)
		if err != nil {
			t.Fatal(err)
		}
		vocabulary(t, still)
		want, _ := Animate([]Frame{{SVG: frames[i].SVG}}, Options{})
		got, err := Animate([]Frame{{SVG: still}}, Options{})
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("slot %d: round trip differs (err %v)", i, err)
		}
	}
	if _, err := doc.Frame(-1); !errors.Is(err, ErrSlot) {
		t.Errorf("err %v", err)
	}
	if _, err := doc.Frame(len(doc.Slots())); !errors.Is(err, ErrSlot) {
		t.Errorf("err %v", err)
	}
}

func TestOneRegionCostsOneRegion(t *testing.T) {
	doc, err := Assemble(scenario(t, 60, "live"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	naive, _ := Assemble(scenario(t, 60, "live"), Options{noDedupe: true})
	st, nv := doc.Stats(), naive.Stats()
	if st.OutputBytes*5 > nv.OutputBytes {
		t.Errorf("dedup %d B vs naive %d B: less than 5x", st.OutputBytes, nv.OutputBytes)
	}
	if st.Shared == 0 || st.Clips != 1 || nv.Clips != 60 || nv.Shared != 0 {
		t.Errorf("stats %+v naive %+v", st, nv)
	}
	vocabulary(t, naive.SVG())
}

// TestSizeTable prints the measurements quoted in the pull request.
func TestSizeTable(t *testing.T) {
	for _, tc := range []struct{ name, mode string }{
		{"one live region", "live"}, {"static (cursor-free)", "static"}, {"every row changes", "all"},
	} {
		fs := scenario(t, 60, tc.mode)
		d, err := Assemble(fs, Options{})
		if err != nil {
			t.Fatal(err)
		}
		n, _ := Assemble(fs, Options{noDedupe: true})
		st := d.Stats()
		t.Logf("| %s | %d | %d | %d | %.1fx |", tc.name, st.InputBytes, n.Stats().OutputBytes, st.OutputBytes,
			float64(n.Stats().OutputBytes)/float64(st.OutputBytes))
	}
}

func TestDisplaySize(t *testing.T) {
	for _, tc := range []struct {
		o    Options
		want string
	}{
		{Options{Width: 20}, `width="20" height="20" viewBox="0 0 10 10"`},
		{Options{Height: 40}, `width="40" height="40" viewBox="0 0 10 10"`},
		{Options{Width: 30, Height: 7}, `width="30" height="7" viewBox="0 0 10 10"`},
	} {
		out, err := Animate([]Frame{at(0, redR)}, tc.o)
		if err != nil || !strings.Contains(string(out), tc.want) {
			t.Errorf("%+v: %v\n%s", tc.o, err, out)
		}
	}
	if _, err := Animate([]Frame{at(0, redR)}, Options{Width: MaxDim + 1}); !errors.Is(err, ErrOptions) {
		t.Errorf("err %v", err)
	}
}

func TestRefusals(t *testing.T) {
	ok := []Frame{at(0, redR), at(10, blueR)}
	one := func(f Frame) []Frame { return []Frame{ok[0], f} }
	for _, tc := range []struct {
		name   string
		frames []Frame
		o      Options
		want   error
	}{
		{"empty", nil, Options{}, ErrNoFrames},
		{"too many", []Frame{ok[0], at(10, blueR), at(20, redR)}, Options{MaxFrames: 2}, ErrTooManyFrames},
		{"too large", ok, Options{MaxBytes: 100}, ErrTooLarge},
		{"neg frames", ok, Options{MaxFrames: -1}, ErrOptions},
		{"neg bytes", ok, Options{MaxBytes: -1}, ErrOptions},
		{"hard frames", ok, Options{MaxFrames: HardMaxFrames + 1}, ErrOptions},
		{"hard bytes", ok, Options{MaxBytes: HardMaxBytes + 1}, ErrOptions},
		{"neg width", ok, Options{Width: -1}, ErrOptions},
		{"neg height", ok, Options{Height: -1}, ErrOptions},
		{"neg end", ok, Options{End: -1}, ErrOptions},
		{"bad title", ok, Options{Title: "a\x00b"}, ErrOptions},
		{"same time", one(at(0, blueR)), Options{}, ErrTiming},
		{"too close", one(Frame{At: 500 * time.Microsecond, SVG: mini(blueR)}), Options{}, ErrTiming},
		{"backwards", []Frame{at(50, redR), at(10, blueR)}, Options{}, ErrTiming},
		{"end too early", ok, Options{End: 10 * time.Millisecond}, ErrTiming},
		{"end too late", ok, Options{End: MaxDuration + time.Second}, ErrTiming},
		{"middle too close", []Frame{at(0, redR), at(10, blueR), {At: 10*time.Millisecond + 500*time.Microsecond, SVG: mini(redR)}, at(100, blueR)}, Options{}, ErrTiming},
		{"middle backwards", []Frame{at(0, redR), at(50, blueR), at(20, redR), at(100, blueR)}, Options{}, ErrTiming},
		{"span wraps", []Frame{{At: -8e18, SVG: mini(redR)}, {At: 0, SVG: mini(blueR)}, {At: 8e18, SVG: mini(redR)}}, Options{}, ErrTiming},
		{"span", []Frame{at(0, redR), {At: MaxDuration, SVG: mini(blueR)}}, Options{}, ErrTiming},
		// the span is refused BEFORE any frame is parsed: garbage frames report ErrTiming
		{"span wraps unparsed", []Frame{{At: -8e18, SVG: []byte("<")}, {At: 0, SVG: []byte("<")}, {At: 8e18, SVG: []byte("<")}}, Options{}, ErrTiming},
		{"span unparsed", []Frame{{At: 0, SVG: []byte("<")}, {At: MaxDuration, SVG: []byte("<")}}, Options{}, ErrTiming},
		{"size", one(Frame{At: 10 * time.Millisecond, SVG: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="11" height="10" viewBox="0 0 11 10"/>`)}), Options{}, ErrSize},
		{"frame", one(Frame{At: 10 * time.Millisecond, SVG: []byte("<svg")}), Options{}, ErrFrame},
	} {
		if _, err := Assemble(tc.frames, tc.o); !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestInputBoundBoundary pins the exact counts: the limit itself is accepted.
func TestInputBoundBoundary(t *testing.T) {
	fs := []Frame{at(0, redR), at(10, blueR)}
	total := len(fs[0].SVG) + len(fs[1].SVG)
	if _, err := Assemble(fs, Options{MaxFrames: 2, MaxBytes: total}); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	if _, err := Assemble(fs, Options{MaxBytes: total - 1}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("one byte over: %v", err)
	}
	if _, err := Assemble(fs, Options{MaxFrames: 1}); !errors.Is(err, ErrTooManyFrames) {
		t.Errorf("one frame over: %v", err)
	}
	if _, err := Assemble([]Frame{at(0, redR), at(1, blueR)}, Options{}); err != nil {
		t.Errorf("MinStep exactly: %v", err)
	}
}

func TestHostileTextIsEscaped(t *testing.T) {
	evil := `</title><script>alert(1)</script>&amp;"'`
	frame := mini(`<g><title>` + strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(evil) + `</title></g>`)
	out, err := Animate([]Frame{{SVG: frame}}, Options{Title: evil})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "<script") {
		t.Fatalf("script survived:\n%s", out)
	}
	vocabulary(t, out)
	if strings.Count(string(out), "&lt;script&gt;") != 2 {
		t.Errorf("text not escaped twice:\n%s", out)
	}
}

func TestFrameVocabulary(t *testing.T) {
	cp := `<clipPath id="c0"><rect x="0" y="0" width="5" height="5"/></clipPath>`
	for name, body := range map[string]string{
		"script":         `<script>alert(1)</script>`,
		"style":          `<style>*{}</style>`,
		"foreign":        `<foreignObject/>`,
		"use":            `<use href="#a"/>`,
		"onload":         `<rect onload="x()"/>`,
		"xlink":          `<image xlink:href="data:image/png;base64,AA=="/>`,
		"bad clip ref":   `<g clip-path="url(#a b)"/>`,
		"trailing g":     `</svg><g/>`,
		"href js":        `<image href="javascript:alert(1)"/>`,
		"href on rect":   `<rect href="data:image/png;base64,AA=="/>`,
		"href non-b64":   `<image href="data:image/png;base64,A!A"/>`,
		"url fill":       `<rect fill="url(http://evil/x)"/>`,
		"unsafe value":   `<rect fill="red;stroke:blue"/>`,
		"id on g":        `<g id="x"/>`,
		"bad clip id":    `<clipPath id="a b"/>`,
		"clip no id":     `<clipPath/>`,
		"dup clip":       cp + cp,
		"clip in clip":   `<clipPath id="c0"><g/></clipPath>`,
		"clip in title":  `<g><title><clipPath id="c0"/></title></g>`,
		"nested missing": `<g><g clip-path="url(#nope)"/></g>`,
		"missing clip":   `<g clip-path="url(#nope)"/>`,
		"clip-path rect": cp + `<rect clip-path="url(#c0)"/>`,
		"clip-path bad":  `<g clip-path="none"/>`,
		"text in g":      `<g>hello</g>`,
		"elem in title":  `<g><title><g/></title></g>`,
		"dup attr":       `<rect x="1" x="2"/>`,
		"foreign ns":     `<rect xmlns="http://evil/"/>`,
		"doctype":        `<!DOCTYPE x>`,
		"pi":             `<?php ?>`,
		"deep":           strings.Repeat("<g>", maxDepth) + strings.Repeat("</g>", maxDepth),
	} {
		_, err := Animate([]Frame{{SVG: mini(body)}}, Options{})
		if !errors.Is(err, ErrFrame) {
			t.Errorf("%s: err %v", name, err)
		}
	}
	for name, doc := range map[string]string{
		"not svg":      `<g xmlns="http://www.w3.org/2000/svg"/>`,
		"g root sized": `<g xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"/>`,
		"trailing g":   head + tail + `<g/>`,
		"bad root ns":  `<svg width="1" height="1" viewBox="0 0 1 1"/>`,
		"root attr":    `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1" onload="x()"/>`,
		"no size":      `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"zero":         `<svg xmlns="http://www.w3.org/2000/svg" width="0" height="1" viewBox="0 0 0 1"/>`,
		"huge":         `<svg xmlns="http://www.w3.org/2000/svg" width="16385" height="1" viewBox="0 0 16385 1"/>`,
		"viewbox":      `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1" viewBox="0 0 1 1"/>`,
		"leading zero": `<svg xmlns="http://www.w3.org/2000/svg" width="02" height="1" viewBox="0 0 02 1"/>`,
		"leading text": `x<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"/>`,
		"unclosed":     `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1">`,
		"trailing el":  head + tail + `<svg/>`,
		"trailing txt": head + tail + `boo`,
		"trailing pi":  head + tail + `<?x ?>`,
		"empty":        ``,
		"only comment": `<!-- x -->`,
	} {
		if _, err := Animate([]Frame{{SVG: []byte(doc)}}, Options{}); !errors.Is(err, ErrFrame) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

func TestFrameAccepts(t *testing.T) {
	body := "\n <!-- note -->\n" + `<clipPath id="c0"><rect x="0" y="0" width="5" height="5"/></clipPath>` +
		`<g clip-path="url(#c0)"><rect x="1" y="1" width="2" height="2" rx="1" fill="#abcdef" fill-opacity="0.5"/>` +
		`<path fill-rule="evenodd" d="M1 1L2 2z"/><image x="0" y="0" width="1" height="1" preserveAspectRatio="none" image-rendering="pixelated" href="data:image/png;base64,iVBORw0KGgo="/></g>` +
		`<g><title>t</title></g><g/>`
	out, err := Animate([]Frame{{SVG: []byte("\n  " + head + body + tail + "\n<!-- end -->\n")}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	vocabulary(t, out)
}

func TestChunkingIsLocal(t *testing.T) {
	// 200 rects, one of which changes per frame: the shared groups must hold
	// most of them, so the cost per extra frame stays far below one frame.
	mk := func(changed int) Frame {
		var b strings.Builder
		for i := 0; i < 200; i++ {
			c := "#112233"
			if i == changed {
				c = "#ff0000"
			}
			b.WriteString(`<rect x="` + itoa(i%10) + `" y="` + itoa(i/10) + `" width="1" height="1" fill="` + c + `"/>`)
		}
		return Frame{At: time.Duration(changed) * 10 * time.Millisecond, SVG: mini(b.String())}
	}
	var fs []Frame
	for i := 0; i < 20; i++ {
		fs = append(fs, mk(i*7))
	}
	d, err := Assemble(fs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := d.Stats()
	perFrame := st.InputBytes / 20
	if extra := (st.OutputBytes - perFrame) / 19; extra > perFrame/3 {
		t.Errorf("extra cost per frame %d B vs frame %d B", extra, perFrame)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestPctAndDecimal(t *testing.T) {
	for _, tc := range []struct {
		ns, total time.Duration
		want      string
	}{
		{0, 400, "0%"}, {100, 400, "25%"}, {1, 3, "33.33333%"}, {2, 3, "66.66666%"},
		{1, 8, "12.5%"}, {1, 100000, "0.001%"}, {1, 10_000_000, "0.00001%"},
	} {
		if got := pct(tc.ns, tc.total); got != tc.want {
			t.Errorf("pct(%d,%d)=%s want %s", tc.ns, tc.total, got, tc.want)
		}
	}
	if got := ms(1500 * time.Microsecond); got != "1.5ms" {
		t.Error(got)
	}
	if got := ms(MaxDuration); got != "600000ms" {
		t.Error(got)
	}
}

func TestNoDedupeBaselineKeepsEveryFrame(t *testing.T) {
	d, err := Assemble([]Frame{at(0, redR), at(10, redR), at(20, redR)}, Options{noDedupe: true})
	if err != nil {
		t.Fatal(err)
	}
	if st := d.Stats(); st.Slots != 3 || st.Pictures != 3 {
		t.Errorf("%+v", st)
	}
}

func canvas(w, h int, body string) []byte {
	ws, hs := strconv.Itoa(w), strconv.Itoa(h)
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="` + ws + `" height="` + hs +
		`" viewBox="0 0 ` + ws + ` ` + hs + `">` + body + `</svg>`)
}

// TestBoundaries pins both sides of every numeric limit.
func TestBoundaries(t *testing.T) {
	okc := func(w, h int) error { _, err := Animate([]Frame{{SVG: canvas(w, h, "")}}, Options{}); return err }
	for _, d := range [][2]int{{1, 1}, {MaxDim, 1}, {1, MaxDim}, {MaxDim, MaxDim}} {
		if err := okc(d[0], d[1]); err != nil {
			t.Errorf("canvas %v: %v", d, err)
		}
	}
	for _, d := range [][2]int{{0, 1}, {1, 0}, {MaxDim + 1, 1}, {1, MaxDim + 1}} {
		if err := okc(d[0], d[1]); !errors.Is(err, ErrFrame) {
			t.Errorf("canvas %v: %v", d, err)
		}
	}
	one := []Frame{at(0, redR)}
	if _, err := Animate(one, Options{MaxFrames: HardMaxFrames, MaxBytes: HardMaxBytes}); err != nil {
		t.Errorf("hard limits themselves: %v", err)
	}
	if _, err := Animate(one, Options{Width: MaxDim, Height: MaxDim}); err != nil {
		t.Errorf("display at MaxDim: %v", err)
	}
	for _, o := range []Options{{Width: MaxDim + 1, Height: 1}, {Width: 1, Height: MaxDim + 1}} {
		if _, err := Animate(one, o); !errors.Is(err, ErrOptions) {
			t.Errorf("%+v: %v", o, err)
		}
	}
	two := []Frame{at(0, redR), at(1, blueR)}
	if _, err := Animate(two, Options{End: MaxDuration}); err != nil {
		t.Errorf("End = MaxDuration: %v", err)
	}
	if _, err := Animate(two, Options{End: MaxDuration + 1}); !errors.Is(err, ErrTiming) {
		t.Errorf("End over MaxDuration: %v", err)
	}
	if _, err := Animate([]Frame{at(0, redR), {At: MaxDuration - time.Millisecond, SVG: mini(blueR)}}, Options{End: MaxDuration}); err != nil {
		t.Errorf("span just under MaxDuration: %v", err)
	}
	if _, err := Animate(two, Options{End: 2 * time.Millisecond}); err != nil {
		t.Errorf("End = last + MinStep: %v", err)
	}
}

func TestTitleCharacterRanges(t *testing.T) {
	for _, r := range []rune{'\t', '\n', '\r', 0x20, 0xD7FF, 0xE000, 0xFFFD, 0x10000, 0x10FFFF} {
		if _, err := Animate([]Frame{at(0, redR)}, Options{Title: "a" + string(r) + "b"}); err != nil {
			t.Errorf("%U refused: %v", r, err)
		}
	}
	for _, r := range []rune{0, 0x1F, 0xFFFE, 0xFFFF} {
		if _, err := Animate([]Frame{at(0, redR)}, Options{Title: "a" + string(r) + "b"}); !errors.Is(err, ErrOptions) {
			t.Errorf("%U accepted: %v", r, err)
		}
	}
}

func TestClipIDLength(t *testing.T) {
	clip := func(id string) error {
		body := `<clipPath id="` + id + `"><rect x="0" y="0" width="1" height="1"/></clipPath><g clip-path="url(#` + id + `)"/>`
		_, err := Animate([]Frame{{SVG: mini(body)}}, Options{})
		return err
	}
	if err := clip(strings.Repeat("a", 32)); err != nil {
		t.Error(err)
	}
	if err := clip(strings.Repeat("a", 33)); !errors.Is(err, ErrFrame) {
		t.Error(err)
	}
}

func TestStaticDocumentHasNoEmptyDefs(t *testing.T) {
	out, _ := Animate([]Frame{at(0, redR)}, Options{})
	if strings.Contains(string(out), "<defs>") {
		t.Errorf("empty <defs> emitted:\n%s", out)
	}
}

func TestSyntaxErrorIsReported(t *testing.T) {
	_, err := Animate([]Frame{{SVG: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"></g>`)}}, Options{})
	if !errors.Is(err, ErrFrame) || !strings.Contains(err.Error(), "element") {
		t.Errorf("err %v", err)
	}
}
