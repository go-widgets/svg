// Command gallery-render writes an SVG + PNG snapshot of every
// go-widgets/toolkit widget kind to a directory. Used to keep
// documentation assets in sync with the toolkit's live look — run
// this on every toolkit dep bump and commit the diff to your docs
// repo, and readers see the actual widget appearance without a
// browser + wasm dance.
//
// Usage:
//
//	go run github.com/go-widgets/svg/cmd/gallery-render -out ./assets
//
// Flags:
//
//	-out DIR   destination directory (default "gallery")
//	-theme     "light" | "dark" (default "light")
//
// Each widget kind lands as two files: NAME.svg (via
// svg/widget.Snapshot) + NAME.png (via svg/widget.PNG).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	svgwidget "github.com/go-widgets/svg/widget"
	"github.com/go-widgets/toolkit"
)

// runFunc / osExit are dependency-injection seams so tests can drive
// main()'s success and error branches without spawning a subprocess
// or having log.Fatalf terminate the test binary.
var (
	runFunc = run
	osExit  = os.Exit
)

func main() {
	if err := runFunc(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "gallery-render: %v\n", err)
		osExit(1)
	}
}

// run is the testable entrypoint: parses flags, mkdirs, renders,
// prints. Split from main() so tests drive the whole pipeline
// without touching os.Args or exiting the process.
func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gallery-render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "gallery", "output directory")
	themeName := fs.String("theme", "light", "theme: light | dark")
	if err := fs.Parse(args); err != nil {
		return err
	}

	theme := toolkit.DefaultLight()
	if *themeName == "dark" {
		theme = toolkit.DefaultDark()
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", *out, err)
	}
	if err := render(*out, theme); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "gallery-render: wrote %d widget snapshots to %s\n", len(entries()), *out)
	return nil
}

// entry is one widget slot in the gallery — the widget produces the
// pixels; W/H picks the pane size.
type entry struct {
	Name string
	W, H int
	Make func() toolkit.Widget
}

// entries lists the widgets to render + their canonical pane sizes.
// Kept in a separate function so tests can drive the whole render
// loop through a temp directory.
func entries() []entry {
	label := &toolkit.Label{Text: "Label text"}
	return []entry{
		{"button", 200, 40, func() toolkit.Widget {
			b := toolkit.NewButton("Click me", nil)
			b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 40})
			return b
		}},
		{"label", 200, 24, func() toolkit.Widget {
			label.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 24})
			return label
		}},
		{"entry", 240, 32, func() toolkit.Widget {
			e := toolkit.NewEntry("editable text")
			e.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 32})
			return e
		}},
		{"checkbutton", 200, 28, func() toolkit.Widget {
			c := toolkit.NewCheckButton("Enable feature", true)
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 28})
			return c
		}},
		{"progressbar", 240, 24, func() toolkit.Widget {
			p := toolkit.NewProgressBar()
			p.Fraction = 0.66
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 24})
			return p
		}},
		{"listbox", 240, 120, func() toolkit.Widget {
			l := toolkit.NewListBox([]string{"apple", "banana", "cherry", "date"})
			l.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 120})
			return l
		}},
		{"treeview", 240, 160, func() toolkit.Widget {
			root := &toolkit.TreeNode{Label: "/", Expanded: true, Children: []*toolkit.TreeNode{
				{Label: "src", Expanded: true, Children: []*toolkit.TreeNode{{Label: "main.go"}}},
				{Label: "README.md"},
			}}
			t := toolkit.NewTreeView(root)
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 160})
			return t
		}},
		{"radiobutton", 200, 28, func() toolkit.Widget {
			r := toolkit.NewRadioButton("Enable option")
			r.Checked = true
			r.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 28})
			return r
		}},
		{"togglebutton", 200, 40, func() toolkit.Widget {
			t := toolkit.NewToggleButton("Muted", true)
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 40})
			return t
		}},
		{"spinbutton", 200, 32, func() toolkit.Widget {
			s := toolkit.NewSpinButton(0, 100, 42, 1)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 32})
			return s
		}},
	}
}

// render writes SVG + PNG for every entry into dir.
func render(dir string, theme *toolkit.Theme) error {
	for _, e := range entries() {
		if err := writeOne(dir, e, theme); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
	}
	return nil
}

// writeOne renders a single entry — SVG then PNG.
func writeOne(dir string, e entry, theme *toolkit.Theme) error {
	w := e.Make()
	svgPath := filepath.Join(dir, e.Name+".svg")
	if err := writeFile(svgPath, func(f *os.File) error {
		_, err := svgwidget.Snapshot(f, w, e.W, e.H, theme, "widget: "+e.Name)
		return err
	}); err != nil {
		return err
	}
	pngPath := filepath.Join(dir, e.Name+".png")
	return writeFile(pngPath, func(f *os.File) error {
		_, err := svgwidget.PNG(f, w, e.W, e.H, theme)
		return err
	})
}

// writeFile creates + writes to path via fn, closing the file on
// exit.
func writeFile(path string, fn func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}
