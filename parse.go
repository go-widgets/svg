// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	svgNS = "http://www.w3.org/2000/svg"
	// MaxDim bounds the canvas of a frame and any requested output size.
	MaxDim = 16384
	// maxDepth bounds element nesting in a frame.
	maxDepth = 24
	pngURI   = "data:image/png;base64,"
)

// ErrFrame wraps every refusal of a malformed or out-of-vocabulary frame.
var ErrFrame = errors.New("svg: invalid frame")

type attr struct{ k, v string }

// pnode is one parsed element. A pnode with name "" never occurs in a parsed
// tree; the empty name is reserved for the virtual grouping nodes of the DAG.
type pnode struct {
	name  string
	attrs []attr
	kids  []*pnode
	text  string
}

// frameTree is a parsed frame: its canvas, its top-level elements and the
// clipPaths it declares, by their original id.
type frameTree struct {
	w, h  int
	kids  []*pnode
	clips map[string]*pnode
}

// attrs allowed anywhere; the vocabulary is that of go-widgets/painter's
// svgrec.Recorder. Everything else (script, style, foreignObject, event
// handlers, external references) is refused, so the output carries no code.
var allowedAttr = map[string]bool{
	"x": true, "y": true, "width": true, "height": true, "rx": true, "ry": true,
	"fill": true, "fill-opacity": true, "fill-rule": true, "stroke": true,
	"stroke-width": true, "stroke-opacity": true, "stroke-linecap": true,
	"stroke-linejoin": true, "d": true, "clip-path": true,
	"preserveAspectRatio": true, "image-rendering": true, "href": true, "id": true,
}

var allowedElem = map[string]bool{
	"g": true, "rect": true, "path": true, "image": true, "title": true, "clipPath": true,
}

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrFrame, fmt.Sprintf(format, a...))
}

func safeChars(s string, extra string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte(extra, c) >= 0:
		default:
			return false
		}
	}
	return true
}

func validID(s string) bool { return s != "" && len(s) <= 32 && safeChars(s, "_-") }

// clipRef extracts id from "url(#id)".
func clipRef(v string) (string, bool) {
	id := strings.TrimSuffix(strings.TrimPrefix(v, "url(#"), ")")
	return id, strings.HasPrefix(v, "url(#") && strings.HasSuffix(v, ")") && validID(id)
}

func checkAttr(elem string, a xml.Attr) error {
	k, v := a.Name.Local, a.Value
	if a.Name.Space != "" || !allowedAttr[k] {
		return bad("attribute %q on <%s>", a.Name.Space+":"+k, elem)
	}
	switch k {
	case "id":
		if elem != "clipPath" || !validID(v) {
			return bad("id on <%s>", elem)
		}
	case "clip-path":
		if _, ok := clipRef(v); !ok || elem != "g" {
			return bad("clip-path on <%s>", elem)
		}
	case "href":
		b64, ok := strings.CutPrefix(v, pngURI)
		if elem != "image" || !ok || !safeChars(b64, "+/=") {
			return bad("href on <%s>", elem)
		}
	default:
		if !safeChars(v, "#.,%+- ") {
			return bad("value of %s on <%s>", k, elem)
		}
	}
	return nil
}

func isSpace(b []byte) bool { return len(bytes.TrimLeft(b, " \t\r\n")) == 0 }

// parseFrame reads one svgrec document. It is strict: unknown elements or
// attributes, doctypes, processing instructions, stray text and trailing
// content are refused.
func parseFrame(data []byte) (*frameTree, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	ft := &frameTree{clips: map[string]*pnode{}}
	var stack []*pnode // open elements; clipPath and title are on it too
	var root *pnode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFrame, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if root != nil && len(stack) == 0 {
				return nil, bad("content after the root element")
			}
			n, err := startElem(ft, root == nil, t, len(stack))
			if err != nil {
				return nil, err
			}
			if root == nil {
				root = n
			} else {
				top := stack[len(stack)-1]
				if top.name == "title" {
					return nil, bad("element inside <title>")
				}
				top.kids = append(top.kids, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if n.name == "clipPath" {
				if err := declareClip(ft, n); err != nil {
					return nil, err
				}
			}
		case xml.CharData:
			if len(stack) == 0 {
				if !isSpace(t) {
					return nil, bad("text outside the root element")
				}
				continue
			}
			top := stack[len(stack)-1]
			switch {
			case top.name == "title":
				top.text += string(t)
			case !isSpace(t):
				return nil, bad("text inside <%s>", top.name)
			}
		case xml.Comment:
		default:
			return nil, bad("directive or processing instruction")
		}
	}
	if root == nil { // an unclosed element is the decoder's own error
		return nil, bad("no <svg> element")
	}
	ft.kids = root.kids
	if err := stripClips(ft); err != nil {
		return nil, err
	}
	return ft, nil
}

func startElem(ft *frameTree, isRoot bool, t xml.StartElement, depth int) (*pnode, error) {
	if depth >= maxDepth {
		return nil, bad("nesting deeper than %d", maxDepth)
	}
	name := t.Name.Local
	if t.Name.Space != svgNS {
		return nil, bad("element <%s> outside the SVG namespace", name)
	}
	seen := map[string]bool{}
	n := &pnode{name: name}
	if isRoot {
		if name != "svg" {
			return nil, bad("root element is <%s>", name)
		}
		return n, rootAttrs(ft, t)
	}
	if !allowedElem[name] {
		return nil, bad("element <%s>", name)
	}
	for _, a := range t.Attr {
		if err := checkAttr(name, a); err != nil {
			return nil, err
		}
		if seen[a.Name.Local] {
			return nil, bad("duplicate attribute %s", a.Name.Local)
		}
		seen[a.Name.Local] = true
		n.attrs = append(n.attrs, attr{a.Name.Local, a.Value})
	}
	if name == "clipPath" && !seen["id"] {
		return nil, bad("clipPath without id")
	}
	return n, nil
}

func rootAttrs(ft *frameTree, t xml.StartElement) error {
	var w, h, vb string
	for _, a := range t.Attr {
		switch {
		case a.Name.Space == "" && a.Name.Local == "xmlns":
		case a.Name.Space == "" && a.Name.Local == "width":
			w = a.Value
		case a.Name.Space == "" && a.Name.Local == "height":
			h = a.Value
		case a.Name.Space == "" && a.Name.Local == "viewBox":
			vb = a.Value
		default:
			return bad("attribute %q on <svg>", a.Name.Local)
		}
	}
	wi, e1 := strconv.Atoi(w)
	hi, e2 := strconv.Atoi(h)
	if e1 != nil || e2 != nil || wi < 1 || hi < 1 || wi > MaxDim || hi > MaxDim ||
		vb != "0 0 "+w+" "+h || strconv.Itoa(wi) != w || strconv.Itoa(hi) != h {
		return bad("canvas %q x %q viewBox %q", w, h, vb)
	}
	ft.w, ft.h = wi, hi
	return nil
}

func declareClip(ft *frameTree, n *pnode) error {
	var id string
	for _, a := range n.attrs {
		if a.k == "id" {
			id = a.v
		}
	}
	if _, dup := ft.clips[id]; dup {
		return bad("duplicate clipPath id %q", id)
	}
	for _, k := range n.kids {
		if k.name != "rect" && k.name != "path" {
			return bad("<%s> inside clipPath", k.name)
		}
	}
	ft.clips[id] = n
	return nil
}

// stripClips removes every clipPath from the tree (they are hoisted into the
// shared <defs>) and checks that every reference resolves.
func stripClips(ft *frameTree) error {
	var walk func(ns []*pnode) ([]*pnode, error)
	walk = func(ns []*pnode) ([]*pnode, error) {
		out := ns[:0:0]
		for _, n := range ns {
			if n.name == "clipPath" {
				continue
			}
			for _, a := range n.attrs {
				if a.k == "clip-path" {
					id, _ := clipRef(a.v)
					if ft.clips[id] == nil {
						return nil, bad("clip-path references missing %q", id)
					}
				}
			}
			var err error
			if n.kids, err = walk(n.kids); err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		return out, nil
	}
	var err error
	ft.kids, err = walk(ft.kids)
	return err
}
