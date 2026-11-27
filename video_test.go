// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"bytes"
	"errors"
	"image"
	"io"
	"testing"
	"time"
)

type fakeRast struct {
	err  error
	size int // 0: honour the request; else force this width
	nil_ bool
	got  [][]byte
}

func (f *fakeRast) Rasterize(svg []byte, w, h int) (*image.NRGBA, error) {
	f.got = append(f.got, svg)
	if f.err != nil {
		return nil, f.err
	}
	if f.nil_ {
		return nil, nil
	}
	if f.size != 0 {
		w = f.size
	}
	return &image.NRGBA{Rect: image.Rect(0, 0, w, h)}, nil // no pixels: only the bounds are checked
}

type fakeEnc struct {
	openErr, writeErr, closeErr error
	calls                       []string
	durs                        []time.Duration
}

func (f *fakeEnc) Open(w io.Writer, width, height int) error {
	f.calls = append(f.calls, "open")
	return f.openErr
}
func (f *fakeEnc) WriteFrame(img *image.NRGBA, d time.Duration) error {
	f.calls = append(f.calls, "write")
	f.durs = append(f.durs, d)
	return f.writeErr
}
func (f *fakeEnc) Close() error { f.calls = append(f.calls, "close"); return f.closeErr }

func TestVideo(t *testing.T) {
	doc, err := Assemble([]Frame{at(0, redR), at(100, blueR), at(300, redR)}, Options{End: 400 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	r, e := &fakeRast{}, &fakeEnc{}
	if err := doc.Video(&bytes.Buffer{}, 640, 480, r, e); err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 100 * time.Millisecond}
	for i := range want {
		if e.durs[i] != want[i] {
			t.Errorf("durations %v", e.durs)
		}
	}
	if len(e.calls) != 5 || e.calls[0] != "open" || e.calls[4] != "close" || len(r.got) != 3 {
		t.Errorf("calls %v", e.calls)
	}
	vocabulary(t, r.got[0])
}

func TestVideoErrors(t *testing.T) {
	doc, _ := Assemble([]Frame{at(0, redR), at(100, blueR)}, Options{})
	boom := errors.New("boom")
	w := &bytes.Buffer{}
	for name, tc := range map[string]struct {
		w      io.Writer
		wd, ht int
		r      *fakeRast
		e      *fakeEnc
		want   error
		closed bool
	}{
		"width":       {w, 0, 10, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"height":      {w, 10, 0, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"too wide":    {w, MaxDim + 1, 1, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"too tall":    {w, 1, MaxDim + 1, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"too many px": {w, 16000, 16000, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"nil writer":  {nil, 10, 10, &fakeRast{}, &fakeEnc{}, ErrVideo, false},
		"open":        {w, 10, 10, &fakeRast{}, &fakeEnc{openErr: boom}, boom, false},
		"raster":      {w, 10, 10, &fakeRast{err: boom}, &fakeEnc{}, boom, true},
		"nil image":   {w, 10, 10, &fakeRast{nil_: true}, &fakeEnc{}, ErrVideo, true},
		"wrong size":  {w, 10, 10, &fakeRast{size: 11}, &fakeEnc{}, ErrVideo, true},
		"write":       {w, 10, 10, &fakeRast{}, &fakeEnc{writeErr: boom}, boom, true},
		"close":       {w, 10, 10, &fakeRast{}, &fakeEnc{closeErr: boom}, boom, true},
	} {
		err := doc.Video(tc.w, tc.wd, tc.ht, tc.r, tc.e)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v", name, err)
		}
		if got := len(tc.e.calls) > 0 && tc.e.calls[len(tc.e.calls)-1] == "close"; got != tc.closed {
			t.Errorf("%s: Close called = %v, calls %v", name, got, tc.e.calls)
		}
	}
	if err := doc.Video(w, 10, 10, nil, &fakeEnc{}); !errors.Is(err, ErrVideo) {
		t.Error(err)
	}
	if err := doc.Video(w, 10, 10, &fakeRast{}, nil); !errors.Is(err, ErrVideo) {
		t.Error(err)
	}
}

func TestVideoBounds(t *testing.T) {
	doc, _ := Assemble([]Frame{at(0, redR), at(100, blueR)}, Options{})
	for _, d := range [][2]int{{1, 1}, {MaxDim, 1}, {1, MaxDim}, {8192, 8192}} {
		if err := doc.Video(&bytes.Buffer{}, d[0], d[1], &fakeRast{}, &fakeEnc{}); err != nil {
			t.Errorf("%v: %v", d, err)
		}
	}
	for _, d := range [][2]int{{8193, 8192}, {MaxDim + 1, 1}, {1, MaxDim + 1}, {0, 1}, {1, 0}} {
		if err := doc.Video(&bytes.Buffer{}, d[0], d[1], &fakeRast{}, &fakeEnc{}); !errors.Is(err, ErrVideo) {
			t.Errorf("%v: %v", d, err)
		}
	}
}
