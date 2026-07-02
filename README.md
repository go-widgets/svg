# go-widgets/svg

[![CI](https://github.com/go-widgets/svg/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/svg/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/go-widgets/svg?display_name=tag&sort=semver&color=0d9488)](https://github.com/go-widgets/svg/releases)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-svg-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/svg)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
[![gallery](https://img.shields.io/badge/live-gallery-14b8a6)](https://go-widgets.github.io/svg/)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

`Snapshot`: turn any RGBA byte buffer (the output of a
[go-widgets/toolkit](https://github.com/go-widgets/toolkit) widget
composition, or any other pixel producer) into a portable,
self-contained SVG document.

Pure Go, stdlib only, BSD-3-Clause. 100% statement coverage.

## Why

Widgets in `go-widgets/toolkit` are pixel-blitting by design — every
`Draw` call writes bytes into a `[]byte` at (x, y). That is great
for rendering, less great for embedding a screenshot in a README:
raw PNGs travel poorly across dark-mode readers, don't scale on
Retina displays, and lose their crispness at any zoom level.

`svg.Snapshot` wraps the render in an SVG envelope. The SVG
declares a `viewBox` pinned to the render's pixel dimensions and
carries the pixels as a base-64 PNG under `image-rendering:
pixelated`. The result:

- **Bit-exact**: every pixel you see in the browser is the same
  pixel the widget produced. No re-vectorisation, no font
  substitution, no antialiasing drift.
- **Scales crisply**: any zoom keeps hard edges (pixelated
  hint). Perfect for a docs site that hosts one asset but serves
  it at multiple sizes.
- **Self-contained**: one file, zero external assets. Drops into
  any Markdown renderer that already accepts SVG images (GitHub,
  GitLab, mkdocs, Hugo, ...).

## Use

The root package works against any RGBA byte producer:

```go
import (
    "os"
    "github.com/go-widgets/svg"
)

surface := make([]byte, 4*w*h)  // fill with anything
// ...your draw code here...

f, _ := os.Create("out.svg")
defer f.Close()
svg.Snapshot(f, surface, w, h, "my image")
```

For go-widgets/toolkit widgets specifically, the sibling
[`widget`](./widget) subpackage removes the manual
`make + Draw + Snapshot` dance:

```go
import (
    "os"
    "github.com/go-widgets/svg/widget"
    "github.com/go-widgets/toolkit"
)

btn := toolkit.NewButton("Click me", nil)
btn.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 40})

f, _ := os.Create("button.svg")
defer f.Close()
widget.Snapshot(f, btn, 200, 40, toolkit.DefaultLight(), "widget: button")

// Or a raw PNG (no SVG envelope):
p, _ := os.Create("button.png")
defer p.Close()
widget.PNG(p, btn, 200, 40, toolkit.DefaultLight())
```

The `label` argument on `Snapshot` becomes the SVG's `<title>` +
`aria-label` for a11y + previewer tooltips. Pass `""` to omit.

## API

Root package (`svg`):

```go
func Snapshot(w io.Writer, surface []byte, width, height int, label string) (int, error)
```

- Returns the byte count written + the first error, mirroring
  `io.Writer`.
- Errors on dimension ≤ 0 or `len(surface) != 4*width*height`.
- Errors bubble up from `w.Write` calls (header, title, body).
- All XML output escapes the five predefined XML entities in
  `label` (both inside `<title>` and in the `aria-label`
  attribute), so a label containing `<b>&"'</b>` remains valid
  XML.

Widget subpackage (`svg/widget`):

```go
func Snapshot(w io.Writer, wg toolkit.Widget, width, height int, theme *toolkit.Theme, label string) (int, error)
func PNG(w io.Writer, wg toolkit.Widget, width, height int, theme *toolkit.Theme) (int, error)
```

- Renders `wg` into a fresh `width×height` RGBA surface via
  `wg.Draw(surface, width, theme)`, then serialises. `Snapshot`
  wraps in an SVG envelope; `PNG` emits raw PNG bytes.
- Errors on non-positive dimensions, nil widget, nil theme.
- Split into a subpackage so the root `svg` stays dep-free — a
  consumer that only uses `svg.Snapshot([]byte)` for a home-grown
  pixel producer does not pull go-widgets/toolkit into its
  module graph.

## License

BSD-3-Clause.
