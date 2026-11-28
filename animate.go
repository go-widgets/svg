// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

// Limits and defaults for [Assemble].
const (
	DefaultMaxFrames = 4096
	DefaultMaxBytes  = 32 << 20
	// HardMaxFrames and HardMaxBytes cap what [Options] may raise the limits to.
	HardMaxFrames = 65536
	HardMaxBytes  = 256 << 20
	// MaxDuration bounds the whole animation.
	MaxDuration = 10 * time.Minute
	// MinStep is the smallest gap between two frame timestamps.
	MinStep = time.Millisecond

	chunkTrigger = 16
	chunkMin     = 2
	chunkMax     = 32
	chunkMask    = 7
)

var (
	// ErrNoFrames is returned for an empty frame list.
	ErrNoFrames = errors.New("svg: no frames")
	// ErrTooManyFrames is returned when the frame count exceeds the limit.
	ErrTooManyFrames = errors.New("svg: too many frames")
	// ErrTooLarge is returned when the frames' total size exceeds the limit.
	ErrTooLarge = errors.New("svg: input too large")
	// ErrTiming is returned for timestamps that are not strictly increasing by
	// at least [MinStep], negative, or beyond [MaxDuration].
	ErrTiming = errors.New("svg: invalid timing")
	// ErrSize is returned for a frame whose canvas differs from the first one.
	ErrSize = errors.New("svg: frames differ in canvas size")
	// ErrOptions is returned for invalid [Options].
	ErrOptions = errors.New("svg: invalid options")
)

// Frame is one recorded frame: the document returned by
// go-widgets/painter's svgrec.Recorder.SVG, and the time it appeared,
// measured from any origin (only differences matter).
type Frame struct {
	At  time.Duration
	SVG []byte
}

// Options tunes [Assemble]. The zero value is valid.
type Options struct {
	// End is when the last frame disappears (the loop length). Zero means the
	// last frame lasts as long as the one before it (100 ms for a single frame).
	End time.Duration
	// Width and Height set the displayed size; the viewBox is unchanged, so the
	// result stays lossless at any size. Zero keeps the recorded size; setting
	// only one derives the other from the aspect ratio.
	Width, Height int
	// Title is the accessible name of the whole animation (optional).
	Title string
	// MaxFrames and MaxBytes bound the input (frame count and total bytes of
	// all frames). Zero means [DefaultMaxFrames] and [DefaultMaxBytes]; they
	// cannot exceed [HardMaxFrames] and [HardMaxBytes].
	MaxFrames, MaxBytes int

	noDedupe bool // test baseline: concatenate frames, no sharing
}

// Slot is one stretch of the timeline during which one distinct picture is
// shown. Adjacent identical frames are merged into one slot.
type Slot struct {
	Start, Duration time.Duration
	Picture         int // index into the distinct pictures
}

// Stats describes an assembly.
type Stats struct {
	Frames        int // input frames
	Slots         int
	Pictures      int // distinct pictures
	Nodes         int // distinct element subtrees
	Shared        int // subtrees emitted once in <defs> and referenced
	Clips         int // distinct clipPaths
	Symbols       int // distinct glyph symbols
	InputBytes    int
	OutputBytes   int
	Width, Height int // recorded canvas
}

// Document is an assembled animation.
type Document struct {
	svg      []byte
	slots    []Slot
	stats    Stats
	pictures []int // root node id per picture
	d        *dag
	clips    []string
	syms     []string // glyph symbol path data, id "y"+index
	total    time.Duration
}

// SVG returns the animated, script-free SVG. The slice is shared: do not
// modify it.
func (doc *Document) SVG() []byte { return doc.svg }

// Slots returns the timeline.
func (doc *Document) Slots() []Slot { return append([]Slot(nil), doc.slots...) }

// Stats returns the assembly statistics.
func (doc *Document) Stats() Stats { return doc.stats }

// Duration is the loop length.
func (doc *Document) Duration() time.Duration { return doc.total }

// Animate assembles frames into one animated SVG; see [Assemble].
func Animate(frames []Frame, o Options) ([]byte, error) {
	doc, err := Assemble(frames, o)
	if err != nil {
		return nil, err
	}
	return doc.svg, nil
}

// Assemble turns timestamped svgrec frames into ONE animated SVG.
//
// Every distinct element subtree is emitted once, in <defs> as a <g id>, and
// referenced with <use href> (SVG 2, 5.3.2 and 5.5); a group is used instead
// of <symbol> because a symbol is clipped to its viewport (5.4.2). The one
// exception is the glyph symbol of svgrec's TextGlyphs: it is declared with
// overflow="visible", so nothing is clipped, and it is merged by path data
// across frames into one <symbol id="yN"> in <defs>. Long
// sibling lists are cut at content-defined boundaries, so a frame that changes
// one region costs about that region. Each distinct picture sits in a <g
// class="f">, hidden by default, and a CSS @keyframes per picture flips its
// visibility. The timing function steps(1,end) holds the previous value
// across each keyframe interval, so the switch happens exactly at the
// keyframe offset (CSS Animations 1, 3 and 3.1). The output is byte for byte
// deterministic and contains no script: frames are parsed strictly and
// anything outside the svgrec vocabulary is refused with [ErrFrame].
func Assemble(frames []Frame, o Options) (*Document, error) {
	if err := checkInput(frames, &o); err != nil {
		return nil, err
	}
	in := 0
	for _, f := range frames {
		in += len(f.SVG)
	}
	asm := &assembler{d: newDAG(!o.noDedupe), clipIdx: map[string]int{}, symIdx: map[string]int{}, dedupe: !o.noDedupe}
	var slots []Slot
	var pictures []int
	byRoot := map[int]int{}
	var w, h int
	for i, f := range frames {
		ft, err := parseFrame(f.SVG)
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w", i, err)
		}
		if i == 0 {
			w, h = ft.w, ft.h
		} else if ft.w != w || ft.h != h {
			return nil, fmt.Errorf("frame %d: %w", i, ErrSize)
		}
		root := asm.root(ft)
		pic, ok := byRoot[root]
		if !ok {
			pic = len(pictures)
			byRoot[root] = pic
			pictures = append(pictures, root)
		}
		start := f.At - frames[0].At
		if n := len(slots); n > 0 && slots[n-1].Picture == pic && asm.dedupe {
			continue // identical to the previous frame: the slot just lasts longer
		}
		slots = append(slots, Slot{Start: start, Picture: pic})
	}
	total := o.End
	if total == 0 {
		prev := 100 * time.Millisecond
		if n := len(frames); n > 1 {
			prev = frames[n-1].At - frames[n-2].At
		}
		total = frames[len(frames)-1].At - frames[0].At + prev
	} else {
		total -= frames[0].At
	}
	if last := frames[len(frames)-1].At - frames[0].At; total < last+MinStep || total > MaxDuration {
		return nil, fmt.Errorf("%w: end %v", ErrTiming, o.End)
	}
	for i := range slots {
		next := total
		if i+1 < len(slots) {
			next = slots[i+1].Start
		}
		slots[i].Duration = next - slots[i].Start
	}
	doc := &Document{slots: slots, pictures: pictures, d: asm.d, clips: asm.clips, syms: asm.syms, total: total}
	doc.stats = Stats{Frames: len(frames), Slots: len(slots), Pictures: len(pictures),
		Clips: len(asm.clips), Symbols: len(asm.syms), InputBytes: in, Width: w, Height: h}
	dw, dh, err := displaySize(w, h, o)
	if err != nil {
		return nil, err
	}
	doc.render(dw, dh, o.Title)
	doc.stats.OutputBytes = len(doc.svg)
	return doc, nil
}

func checkInput(frames []Frame, o *Options) error {
	if len(frames) == 0 {
		return ErrNoFrames
	}
	maxF, maxB := o.MaxFrames, o.MaxBytes
	if maxF < 0 || maxB < 0 || maxF > HardMaxFrames || maxB > HardMaxBytes ||
		o.Width < 0 || o.Height < 0 || o.End < 0 || !validText(o.Title) {
		return ErrOptions
	}
	if maxF == 0 {
		maxF = DefaultMaxFrames
	}
	if maxB == 0 {
		maxB = DefaultMaxBytes
	}
	if len(frames) > maxF {
		return fmt.Errorf("%w: %d > %d", ErrTooManyFrames, len(frames), maxF)
	}
	sum := 0
	for i, f := range frames {
		sum += len(f.SVG)
		if sum > maxB {
			return fmt.Errorf("%w: more than %d bytes", ErrTooLarge, maxB)
		}
		if i > 0 && f.At-frames[i-1].At < MinStep {
			return fmt.Errorf("%w: frame %d at %v after %v", ErrTiming, i, f.At, frames[i-1].At)
		}
	}
	if span := frames[len(frames)-1].At - frames[0].At; span < 0 || span >= MaxDuration {
		return fmt.Errorf("%w: span %v", ErrTiming, span)
	}
	return nil
}

func validText(s string) bool {
	for _, r := range s {
		if !(r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xD7FF ||
			r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			return false
		}
	}
	return true
}

func displaySize(w, h int, o Options) (int, int, error) {
	dw, dh := o.Width, o.Height
	switch {
	case dw == 0 && dh == 0:
		dw, dh = w, h
	case dh == 0:
		dh = max(1, (dw*h+w/2)/w)
	case dw == 0:
		dw = max(1, (dh*w+h/2)/h)
	}
	if dw > MaxDim || dh > MaxDim {
		return 0, 0, fmt.Errorf("%w: size %dx%d", ErrOptions, dw, dh)
	}
	return dw, dh, nil
}

// assembler interns frames into the shared DAG.
type assembler struct {
	d       *dag
	clips   []string // clipPath markup by index
	clipIdx map[string]int
	syms    []string // glyph symbol path data by index
	symIdx  map[string]int
	dedupe  bool
}

// symID interns the glyph symbol s, across frames: two frames recorded apart
// that draw the same glyph share one <symbol>.
func (a *assembler) symID(s *pnode) string {
	d := symbolData(s)
	i, ok := a.symIdx[d]
	if !ok || !a.dedupe {
		i = len(a.syms)
		a.syms = append(a.syms, d)
		a.symIdx[d] = i
	}
	return "y" + strconv.Itoa(i)
}

func (a *assembler) clipID(c *pnode) string {
	var b bytes.Buffer
	for _, k := range c.kids {
		writePnode(&b, k)
	}
	key := b.String()
	i, ok := a.clipIdx[key]
	if !ok || !a.dedupe {
		i = len(a.clips)
		a.clips = append(a.clips, key)
		a.clipIdx[key] = i
	}
	return "k" + strconv.Itoa(i)
}

func writePnode(b *bytes.Buffer, n *pnode) {
	b.WriteByte('<')
	b.WriteString(n.name)
	for _, a := range n.attrs {
		writeAttr(b, a.k, a.v)
	}
	b.WriteString("/>")
}

func writeAttr(b *bytes.Buffer, k, v string) {
	b.WriteByte(' ')
	b.WriteString(k)
	b.WriteString(`="`)
	_ = xml.EscapeText(b, []byte(v))
	b.WriteByte('"')
}

func (a *assembler) root(ft *frameTree) int {
	ids := a.kids(ft, ft.kids)
	return a.d.intern(cnode{kids: a.d.chunk(ids)})
}

func (a *assembler) kids(ft *frameTree, ns []*pnode) []int {
	ids := make([]int, len(ns))
	for i, n := range ns {
		ids[i] = a.build(ft, n)
	}
	return ids
}

func (a *assembler) build(ft *frameTree, p *pnode) int {
	c := cnode{name: p.name, text: p.text}
	for _, at := range p.attrs {
		if at.k == "clip-path" {
			id, _ := clipRef(at.v)
			at.v = "url(#" + a.clipID(ft.clips[id]) + ")"
		}
		if at.k == "href" && p.name == "use" {
			at.v = "#" + a.symID(ft.syms[strings.TrimPrefix(at.v, "#")])
		}
		c.attrs = append(c.attrs, at)
	}
	c.kids = a.d.chunk(a.kids(ft, p.kids))
	return a.d.intern(c)
}

// cnode is a hash-consed subtree. An empty name is a virtual group: it renders
// as its children only (or as <g id> when shared).
type cnode struct {
	name  string
	attrs []attr
	kids  []int
	text  string
	h     uint64
}

type dag struct {
	nodes  []cnode
	index  map[string]int
	dedupe bool
}

func newDAG(dedupe bool) *dag { return &dag{index: map[string]int{}, dedupe: dedupe} }

func (c *cnode) key() string {
	var b strings.Builder
	b.WriteString(strconv.Quote(c.name))
	for _, a := range c.attrs {
		b.WriteByte(' ')
		b.WriteString(a.k)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(a.v))
	}
	b.WriteByte('[')
	for _, k := range c.kids {
		b.WriteString(strconv.Itoa(k))
		b.WriteByte(',')
	}
	b.WriteByte(']')
	b.WriteString(strconv.Quote(c.text))
	return b.String()
}

func (d *dag) intern(c cnode) int {
	k := c.key()
	if d.dedupe {
		if id, ok := d.index[k]; ok {
			return id
		}
	}
	hh := fnv.New64a()
	_, _ = hh.Write([]byte(k))
	c.h = hh.Sum64()
	d.nodes = append(d.nodes, c)
	d.index[k] = len(d.nodes) - 1
	return len(d.nodes) - 1
}

// chunk groups a long sibling list into virtual groups cut where the content
// hash of a child says so, so that editing one child only changes the group
// around it. Repeats until the list is short.
func (d *dag) chunk(ids []int) []int {
	for len(ids) > chunkTrigger {
		var out, cur []int
		flush := func() {
			switch len(cur) { // never empty: flush follows an append
			case 1:
				out = append(out, cur[0])
			default:
				out = append(out, d.intern(cnode{kids: append([]int(nil), cur...)}))
			}
			cur = cur[:0]
		}
		for _, id := range ids {
			cur = append(cur, id)
			if len(cur) >= chunkMax || len(cur) >= chunkMin && d.nodes[id].h&chunkMask == 0 {
				flush()
			}
		}
		if len(cur) > 0 {
			flush()
		}
		ids = out // every group but the last holds >= chunkMin children: it shrinks
	}
	return ids
}
