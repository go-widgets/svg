// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

// ErrSlot is returned by [Document.Frame] for an index outside the timeline.
var ErrSlot = fmt.Errorf("svg: no such slot")

// tags returns the opening and closing markup of a node. id, when set, is
// inserted as the first attribute (a virtual group becomes <g id>). An empty
// element is self-closing and has no closing markup.
func (c *cnode) tags(id string) (open, closeT string) {
	var b bytes.Buffer
	name := c.name
	if name == "" {
		if id == "" {
			return "", ""
		}
		name = "g"
	}
	b.WriteByte('<')
	b.WriteString(name)
	if id != "" {
		writeAttr(&b, "id", id)
	}
	for _, a := range c.attrs {
		writeAttr(&b, a.k, a.v)
	}
	if len(c.kids) == 0 && c.text == "" && c.name != "" {
		b.WriteString("/>")
		return b.String(), ""
	}
	b.WriteByte('>')
	_ = xml.EscapeText(&b, []byte(c.text))
	return b.String(), "</" + name + ">"
}

type renderer struct {
	d    *dag
	def  []bool
	name []string
	buf  bytes.Buffer
}

func useMarkup(name string) string { return `<use href="#` + name + `"/>` }

func (r *renderer) decide() {
	d := r.d
	n := len(d.nodes)
	refs := make([]int, n)
	for _, c := range d.nodes {
		for _, k := range c.kids {
			refs[k]++
		}
	}
	r.def, r.name = make([]bool, n), make([]string, n)
	inl := make([]int, n)
	defs := 0
	for i := range d.nodes {
		c := &d.nodes[i]
		open, closeT := c.tags("")
		l := len(open) + len(closeT)
		for _, k := range c.kids {
			l += inl[k] // a shared child counts as its <use>, set below
		}
		inl[i] = l
		cand := "s" + strconv.FormatInt(int64(defs), 36)
		use := len(useMarkup(cand))
		if refs[i] > 1 && (refs[i]-1)*(l-use) > 14+len(cand) {
			r.def[i], r.name[i] = true, cand
			inl[i] = use
			defs++
		}
	}
}

// inline writes node i with its own markup; shared children are referenced
// unless full is set (a standalone frame has no <defs> to reference).
func (r *renderer) inline(i int, id string, full bool) {
	c := &r.d.nodes[i]
	open, closeT := c.tags(id)
	r.buf.WriteString(open)
	for _, k := range c.kids {
		if r.def[k] && !full {
			r.buf.WriteString(useMarkup(r.name[k]))
		} else {
			r.inline(k, "", full)
		}
	}
	r.buf.WriteString(closeT)
}

func (doc *Document) header(r *renderer, dw, dh int) {
	r.buf.WriteString(`<svg xmlns="` + svgNS + `" width="`)
	r.buf.WriteString(strconv.Itoa(dw))
	r.buf.WriteString(`" height="`)
	r.buf.WriteString(strconv.Itoa(dh))
	r.buf.WriteString(`" viewBox="0 0 `)
	r.buf.WriteString(strconv.Itoa(doc.stats.Width))
	r.buf.WriteByte(' ')
	r.buf.WriteString(strconv.Itoa(doc.stats.Height))
	r.buf.WriteString("\">\n")
}

func (doc *Document) clipDefs(r *renderer) {
	for i, c := range doc.clips {
		r.buf.WriteString(`<clipPath id="k` + strconv.Itoa(i) + `">` + c + "</clipPath>\n")
	}
}

func (doc *Document) render(dw, dh int, title string) {
	r := &renderer{d: doc.d}
	r.decide()
	doc.header(r, dw, dh)
	if title != "" {
		r.buf.WriteString("<title>")
		_ = xml.EscapeText(&r.buf, []byte(title))
		r.buf.WriteString("</title>\n")
	}
	animated := len(doc.slots) > 1
	if animated {
		doc.style(r)
	}
	shared := 0
	for _, d := range r.def {
		if d {
			shared++
		}
	}
	doc.stats.Nodes, doc.stats.Shared = len(doc.d.nodes), shared
	if len(doc.clips) > 0 || shared > 0 {
		r.buf.WriteString("<defs>\n")
		doc.clipDefs(r)
		for i, d := range r.def {
			if d {
				r.inline(i, r.name[i], false)
				r.buf.WriteByte('\n')
			}
		}
		r.buf.WriteString("</defs>\n")
	}
	for p, root := range doc.pictures {
		if animated {
			r.buf.WriteString(`<g class="f a` + strconv.Itoa(p) + "\">")
		} else {
			r.buf.WriteString("<g>")
		}
		r.inline(root, "", false)
		r.buf.WriteString("</g>\n")
	}
	r.buf.WriteString("</svg>\n")
	doc.svg = r.buf.Bytes()
}

// pct formats ns/total as a percentage with five decimals, trimmed. Integer
// arithmetic keeps it exact and deterministic; total <= MaxDuration so the
// product fits in an int64.
func pct(ns, total time.Duration) string {
	v := int64(ns) * 10_000_000 / int64(total)
	return decimal(v, 100000) + "%"
}

// ms formats a duration in milliseconds, nanosecond exact.
func ms(d time.Duration) string { return decimal(int64(d), 1_000_000) + "ms" }

// decimal formats v/scale (scale a power of ten) without trailing zeros.
func decimal(v, scale int64) string {
	s := strconv.FormatInt(v/scale, 10)
	if f := v % scale; f != 0 {
		frac := strconv.FormatInt(f+scale, 10)[1:] // zero padded
		for frac[len(frac)-1] == '0' {
			frac = frac[:len(frac)-1]
		}
		s += "." + frac
	}
	return s
}

func (doc *Document) style(r *renderer) {
	b := &r.buf
	b.WriteString("<style>.f{visibility:hidden;animation-duration:" + ms(doc.total) +
		";animation-timing-function:steps(1,end);animation-iteration-count:infinite}\n")
	spans := make([][]Slot, len(doc.pictures))
	for _, s := range doc.slots {
		spans[s.Picture] = append(spans[s.Picture], s)
	}
	for p, ss := range spans {
		a := "a" + strconv.Itoa(p)
		b.WriteString("." + a + "{")
		if ss[0].Start == 0 {
			b.WriteString("visibility:visible;")
		}
		b.WriteString("animation-name:" + a + "}\n@keyframes " + a + "{")
		if ss[0].Start != 0 {
			b.WriteString("0%{visibility:hidden}")
		}
		for _, s := range ss {
			b.WriteString(pct(s.Start, doc.total) + "{visibility:visible}") // pct(0) is "0%"
			if end := s.Start + s.Duration; end < doc.total {
				b.WriteString(pct(end, doc.total) + "{visibility:hidden}")
			}
		}
		b.WriteString("}\n")
	}
	b.WriteString("@media (prefers-reduced-motion:reduce){.f{animation:none}}</style>\n")
}

// Frame returns the still SVG of slot i, with no animation and no sharing, at
// the recorded size. It is what [Document.Video] rasterises.
func (doc *Document) Frame(i int) ([]byte, error) {
	if i < 0 || i >= len(doc.slots) {
		return nil, fmt.Errorf("%w: %d of %d", ErrSlot, i, len(doc.slots))
	}
	return doc.still(i), nil
}

func (doc *Document) still(i int) []byte {
	r := &renderer{d: doc.d}
	r.def, r.name = make([]bool, len(doc.d.nodes)), make([]string, len(doc.d.nodes))
	doc.header(r, doc.stats.Width, doc.stats.Height)
	doc.clipDefs(r) // clipPath is never rendered itself: no <defs> needed, and the still stays svgrec-shaped
	r.inline(doc.pictures[doc.slots[i].Picture], "", true)
	r.buf.WriteString("\n</svg>\n")
	return r.buf.Bytes()
}
