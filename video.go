// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import (
	"errors"
	"fmt"
	"image"
	"io"
	"time"
)

// MaxVideoPixels bounds one rasterised frame (width x height).
const MaxVideoPixels = 64 << 20

// ErrVideo is returned for an invalid video request.
var ErrVideo = errors.New("svg: invalid video request")

// Rasterizer renders a still SVG at a pixel size. Implementations may use any
// pure-Go rasteriser (for instance go-gfx/gfx/svg); nothing here depends on one.
type Rasterizer interface {
	Rasterize(svg []byte, width, height int) (*image.NRGBA, error)
}

// VideoEncoder writes a sequence of timed frames as a video container. A WebM
// (VP8/VP9) encoder is expected to implement it; this package ships none,
// because no pure-Go one exists and a cgo or exec one is out of policy.
// Calls come in the order Open, WriteFrame (one per slot, in time order),
// Close; Close is called even after an error so the encoder can release
// resources.
type VideoEncoder interface {
	Open(w io.Writer, width, height int) error
	WriteFrame(img *image.NRGBA, d time.Duration) error
	Close() error
}

// Video rasterises every slot of the timeline at width x height (any size:
// the source is vector) and feeds the frames to enc, writing to w.
func (doc *Document) Video(w io.Writer, width, height int, r Rasterizer, enc VideoEncoder) (err error) {
	if width < 1 || height < 1 || width > MaxDim || height > MaxDim ||
		width*height > MaxVideoPixels || r == nil || enc == nil || w == nil {
		return fmt.Errorf("%w: %dx%d", ErrVideo, width, height)
	}
	if err := enc.Open(w, width, height); err != nil {
		return err
	}
	defer func() {
		if cerr := enc.Close(); err == nil {
			err = cerr
		}
	}()
	for i, s := range doc.slots {
		img, rerr := r.Rasterize(doc.still(i), width, height)
		if rerr != nil {
			return fmt.Errorf("slot %d: %w", i, rerr)
		}
		if img == nil || img.Bounds().Dx() != width || img.Bounds().Dy() != height {
			return fmt.Errorf("slot %d: %w: rasteriser returned the wrong size", i, ErrVideo)
		}
		if werr := enc.WriteFrame(img, s.Duration); werr != nil {
			return fmt.Errorf("slot %d: %w", i, werr)
		}
	}
	return nil
}
