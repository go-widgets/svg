# go-widgets/svg

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

```go
import (
    "os"
    "github.com/go-widgets/svg"
    "github.com/go-widgets/toolkit"
)

func main() {
    w, h := 200, 40
    surface := make([]byte, 4*w*h)

    btn := toolkit.NewButton("Click me", nil)
    btn.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})
    btn.Draw(surface, w, toolkit.DefaultLight())

    f, _ := os.Create("button.svg")
    defer f.Close()
    svg.Snapshot(f, surface, w, h, "widget: button")
}
```

The `label` argument (last positional) becomes the SVG's `<title>`
+ `aria-label` for a11y + previewer tooltips. Pass `""` to omit.

## API

```
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

## License

BSD-3-Clause.
