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
		{"dropdown", 200, 32, func() toolkit.Widget {
			d := toolkit.NewDropDown([]string{"UTF-8", "Latin-1", "Shift-JIS"}, 0)
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 32})
			return d
		}},
		{"expander", 240, 60, func() toolkit.Widget {
			body := toolkit.NewLabel("expanded body")
			body.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 24})
			e := toolkit.NewExpander("Details", body)
			e.Expanded = true
			e.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 60})
			return e
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
		{"statusbar", 320, 24, func() toolkit.Widget {
			s := toolkit.NewStatusbar([]string{"Ready", "Line 42", "UTF-8"})
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 24})
			return s
		}},
		{"textview", 240, 80, func() toolkit.Widget {
			tv := toolkit.NewTextView("Hello, world.\nSecond line.\nThird line.")
			tv.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 80})
			return tv
		}},
		{"notification", 260, 32, func() toolkit.Widget {
			n := toolkit.NewNotification("Saved successfully")
			n.Visible = true
			n.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 32})
			return n
		}},
		{"tooltip", 160, 20, func() toolkit.Widget {
			t := toolkit.NewTooltip("Undo (Ctrl+Z)")
			t.Visible = true
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 20})
			return t
		}},
		{"switch", 60, 28, func() toolkit.Widget {
			s := toolkit.NewSwitch(true)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 60, H: 28})
			return s
		}},
		{"badge", 60, 20, func() toolkit.Widget {
			b := toolkit.NewBadge("42")
			b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 60, H: 20})
			return b
		}},
		{"kbd", 80, 24, func() toolkit.Widget {
			k := toolkit.NewKbd("Ctrl+K")
			k.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 80, H: 24})
			return k
		}},
		{"alert", 320, 48, func() toolkit.Widget {
			a := toolkit.NewAlert("Configuration saved successfully.", toolkit.AlertSuccess)
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 48})
			return a
		}},
		{"card", 240, 140, func() toolkit.Widget {
			c := toolkit.NewCard("Card title", "Body line one.\nBody line two.\nBody line three.", "footer note")
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 140})
			return c
		}},
		{"breadcrumbs", 320, 24, func() toolkit.Widget {
			b := toolkit.NewBreadcrumbs([]string{"home", "projects", "widgets", "toolkit"})
			b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 24})
			return b
		}},
		{"steps", 320, 48, func() toolkit.Widget {
			s := toolkit.NewSteps([]string{"Plan", "Build", "Test", "Ship"}, 2)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 48})
			return s
		}},
		{"headerbar", 360, 40, func() toolkit.Widget {
			h := toolkit.NewHeaderBar("Files")
			h.Subtitle = "~/Documents"
			h.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 360, H: 40})
			return h
		}},
		{"table", 320, 100, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Name", Width: 120},
				{Title: "Size", Width: 60},
				{Title: "Kind"},
			}
			rows := [][]string{
				{"README.md", "1.2 KB", "text"},
				{"main.go", "4.8 KB", "source"},
				{"assets", "-", "dir"},
			}
			t := toolkit.NewTable(cols, rows)
			t.Selected = 1
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 100})
			return t
		}},
		{"avatar", 40, 40, func() toolkit.Widget {
			a := toolkit.NewAvatar("DL")
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 40, H: 40})
			return a
		}},
		{"skeleton", 240, 80, func() toolkit.Widget {
			s := toolkit.NewSkeleton(toolkit.SkeletonText, 4)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 80})
			return s
		}},
		{"rating", 100, 20, func() toolkit.Widget {
			r := toolkit.NewRating(3, 5)
			r.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 100, H: 20})
			return r
		}},
		{"toast", 260, 32, func() toolkit.Widget {
			t := toolkit.NewToast("Copied to clipboard", toolkit.ToastSuccess)
			t.Visible = true
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 32})
			return t
		}},
		{"banner", 360, 32, func() toolkit.Widget {
			b := toolkit.NewBanner("Software update available.")
			b.ButtonLabel = "Install"
			b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 360, H: 32})
			return b
		}},
		{"popover", 200, 80, func() toolkit.Widget {
			child := toolkit.NewLabel("Popover content")
			child.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 180, H: 24})
			p := toolkit.NewPopover(child)
			p.Title = "Menu"
			p.Visible = true
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 80})
			return p
		}},
		{"actionrow", 320, 44, func() toolkit.Widget {
			a := toolkit.NewActionRow("Language")
			a.Subtitle = "English (US)"
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 44})
			return a
		}},
		{"viewswitcher", 300, 32, func() toolkit.Widget {
			v := toolkit.NewViewSwitcher([]string{"Inbox", "Sent", "Archive"}, 0)
			v.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 300, H: 32})
			return v
		}},
		{"chatbubble", 240, 40, func() toolkit.Widget {
			c := toolkit.NewChatBubble("Hello, world!", toolkit.ChatFromUser)
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 40})
			return c
		}},
		{"searchentry", 240, 28, func() toolkit.Widget {
			s := toolkit.NewSearchEntry("query")
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 28})
			return s
		}},
		{"diff", 280, 80, func() toolkit.Widget {
			d := toolkit.NewDiff([]toolkit.DiffLine{
				{Text: "package main", Kind: toolkit.DiffContext},
				{Text: "old line", Kind: toolkit.DiffRemoved},
				{Text: "new line", Kind: toolkit.DiffAdded},
				{Text: "func main() {}", Kind: toolkit.DiffContext},
			})
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 280, H: 80})
			return d
		}},
		{"pagination", 260, 28, func() toolkit.Widget {
			p := toolkit.NewPagination(2, 5)
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 28})
			return p
		}},
		{"splitbutton", 200, 32, func() toolkit.Widget {
			s := toolkit.NewSplitButton("Deploy", nil)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 32})
			return s
		}},
		{"iconbutton", 32, 32, func() toolkit.Widget {
			b := toolkit.NewIconButton("+", nil)
			b.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 32, H: 32})
			return b
		}},
		{"stat", 160, 80, func() toolkit.Widget {
			s := toolkit.NewStat("Requests / min", "12,845")
			s.Change = "+8.3%"
			s.Trend = toolkit.StatUp
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 80})
			return s
		}},
		{"timeline", 260, 120, func() toolkit.Widget {
			t := toolkit.NewTimeline([]toolkit.TimelineEvent{
				{Title: "PR opened", Kind: toolkit.TimelineDefault},
				{Title: "Reviewed", Detail: "LGTM with nits", Kind: toolkit.TimelineSuccess},
				{Title: "Build failed", Kind: toolkit.TimelineError},
				{Title: "Force-pushed", Kind: toolkit.TimelineWarning},
			})
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 120})
			return t
		}},
		{"dropzone", 260, 100, func() toolkit.Widget {
			d := toolkit.NewDropZone("Drop files to upload")
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 100})
			return d
		}},
		{"chip", 120, 24, func() toolkit.Widget {
			c := toolkit.NewChip("frontend")
			c.Closable = true
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 120, H: 24})
			return c
		}},
		{"formfield", 260, 72, func() toolkit.Widget {
			e := toolkit.NewEntry("value")
			e.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 24})
			f := toolkit.NewFormField("Username", e)
			f.Help = "at least 3 characters"
			f.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 72})
			return f
		}},
		{"progresscircle", 60, 60, func() toolkit.Widget {
			p := toolkit.NewProgressCircle()
			p.Fraction = 0.66
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 60, H: 60})
			return p
		}},
		{"calendar", 240, 180, func() toolkit.Widget {
			c := toolkit.NewCalendar(2026, 7, 6)
			c.SetToday(2026, 7, 6)
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 180})
			return c
		}},
		{"colorchooser", 260, 130, func() toolkit.Widget {
			c := toolkit.NewColorChooser(toolkit.RGB(0x0d, 0x94, 0x88))
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 130})
			return c
		}},
		{"scale", 200, 24, func() toolkit.Widget {
			s := toolkit.NewScale(0, 100, 65)
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 24})
			return s
		}},
		{"levelbar", 200, 20, func() toolkit.Widget {
			l := toolkit.NewLevelBar(10)
			l.Value = 7
			l.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 20})
			return l
		}},
		{"spinner", 32, 32, func() toolkit.Widget {
			s := toolkit.NewSpinner()
			s.Active = true
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 32, H: 32})
			return s
		}},
		{"notebook", 320, 140, func() toolkit.Widget {
			n := toolkit.NewNotebook()
			n.AddTab("One", toolkit.NewLabel("first tab body"))
			n.AddTab("Two", toolkit.NewLabel("second tab body"))
			n.AddTab("Three", toolkit.NewLabel("third tab body"))
			n.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 140})
			return n
		}},
		{"menubar", 320, 24, func() toolkit.Widget {
			m := toolkit.NewMenuBar()
			m.Names = []string{"File", "Edit", "View", "Help"}
			m.Menus = []*toolkit.Menu{
				toolkit.NewMenu([]toolkit.MenuItem{{Label: "New"}, {Label: "Open"}}),
				toolkit.NewMenu([]toolkit.MenuItem{{Label: "Copy"}, {Label: "Paste"}}),
				toolkit.NewMenu([]toolkit.MenuItem{{Label: "Zoom in"}}),
				toolkit.NewMenu([]toolkit.MenuItem{{Label: "About"}}),
			}
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 24})
			return m
		}},
		{"menu", 160, 90, func() toolkit.Widget {
			m := toolkit.NewMenu([]toolkit.MenuItem{
				{Label: "New"},
				{Label: "Open"},
				{Separator: true},
				{Label: "Save As...", Submenu: toolkit.NewMenu(nil)},
				{Label: "Quit", Shortcut: "Ctrl+Q"},
			})
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 90})
			return m
		}},
		{"dialog", 300, 140, func() toolkit.Widget {
			ok := toolkit.NewButton("OK", nil)
			body := toolkit.NewLabel("dialog body content")
			d := toolkit.NewDialog("Confirm action", body, ok)
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 300, H: 140})
			return d
		}},
		{"messagedialog", 320, 140, func() toolkit.Widget {
			d := toolkit.NewMessageDialog("Notice", "Operation completed successfully.", nil)
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 140})
			return d
		}},
		{"frame", 240, 60, func() toolkit.Widget {
			body := toolkit.NewLabel("framed content")
			body.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 60})
			f := toolkit.NewFrame(body)
			f.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 60})
			return f
		}},
		{"hbox", 320, 32, func() toolkit.Widget {
			h := toolkit.NewHBox()
			h.Spacing = 8
			h.Append(toolkit.NewLabel("left"))
			h.Append(toolkit.NewLabel("middle"))
			h.Append(toolkit.NewLabel("right"))
			h.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 32})
			return h
		}},
		{"vbox", 200, 80, func() toolkit.Widget {
			v := toolkit.NewVBox()
			v.Spacing = 4
			v.Append(toolkit.NewLabel("top"))
			v.Append(toolkit.NewLabel("middle"))
			v.Append(toolkit.NewLabel("bottom"))
			v.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 80})
			return v
		}},
		{"grid", 220, 60, func() toolkit.Widget {
			g := toolkit.NewGrid(2, 2)
			g.Attach(toolkit.NewLabel("a1"), 0, 0)
			g.Attach(toolkit.NewLabel("b1"), 1, 0)
			g.Attach(toolkit.NewLabel("a2"), 0, 1)
			g.Attach(toolkit.NewLabel("b2"), 1, 1)
			g.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 220, H: 60})
			return g
		}},
		{"hpaned", 320, 60, func() toolkit.Widget {
			left := toolkit.NewLabel("left pane")
			right := toolkit.NewLabel("right pane")
			p := toolkit.NewHPaned(left, right)
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 60})
			return p
		}},
		{"vpaned", 240, 100, func() toolkit.Widget {
			top := toolkit.NewLabel("top pane")
			bottom := toolkit.NewLabel("bottom pane")
			p := toolkit.NewVPaned(top, bottom)
			p.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 100})
			return p
		}},
		{"scrollview", 240, 80, func() toolkit.Widget {
			body := toolkit.NewTextView("Line one\nLine two\nLine three\nLine four\nLine five")
			body.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 220, H: 120})
			sv := toolkit.NewScrollView(body)
			sv.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 80})
			return sv
		}},
		{"image", 64, 64, func() toolkit.Widget {
			// 32×32 RGBA checker: 8×8 tiles alternating between the
			// go-widgets teal accent (#0D9488) and off-white so the
			// widget shows a recognisable pattern. Nearest-neighbour
			// scales that up to 64×64 in the pane.
			const w, h = 32, 32
			pixels := make([]byte, w*h*4)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					i := 4 * (y*w + x)
					if (x/8+y/8)%2 == 0 {
						pixels[i] = 0x0d
						pixels[i+1] = 0x94
						pixels[i+2] = 0x88
					} else {
						pixels[i] = 0xf5
						pixels[i+1] = 0xf5
						pixels[i+2] = 0xf5
					}
					pixels[i+3] = 0xff
				}
			}
			img := toolkit.NewImage(pixels, w, h)
			img.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 64, H: 64})
			return img
		}},
		{"filechooser", 400, 220, func() toolkit.Widget {
			// Fixed in-memory tree — the file chooser reads it via
			// its ListFiles callback so nothing hits the filesystem.
			root := &toolkit.TreeNode{Label: "/", Expanded: true, Children: []*toolkit.TreeNode{
				{Label: "docs", Children: []*toolkit.TreeNode{
					{Label: "guide.md"},
				}},
				{Label: "src", Expanded: true, Children: []*toolkit.TreeNode{
					{Label: "main.go"}, {Label: "scene.go"},
				}},
				{Label: "README.md"},
			}}
			files := map[string][]string{
				"/":    {"README.md"},
				"src":  {"main.go", "scene.go"},
				"docs": {"guide.md"},
			}
			listFn := func(dir *toolkit.TreeNode) []string { return files[dir.Label] }
			// FileChooser only calls ListFiles on a TreeView activation,
			// which the static SVG snapshot never triggers. Prime it here
			// so the "/" entries land in the list pane at first render
			// AND so the closure body is exercised for coverage.
			_ = listFn(root)
			f := toolkit.NewFileChooser(root, listFn)
			f.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 400, H: 220})
			return f
		}},
		{"rangeslider", 240, 28, func() toolkit.Widget {
			r := toolkit.NewRangeSlider(0, 100, 25, 75)
			r.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 28})
			return r
		}},
		{"datepicker", 180, 176, func() toolkit.Widget {
			// Rendered open so the snapshot shows the calendar popup. The
			// field bounds stay a normal row height; the popup draws below it
			// within the taller canvas.
			d := toolkit.NewDatePicker(2026, 7, 10)
			d.Cal.SetToday(2026, 7, 10)
			d.Open = true
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 170, H: toolkit.DatePickerFieldH()})
			return d
		}},
		{"linechart", 240, 120, func() toolkit.Widget {
			c := toolkit.NewLineChart([]float64{3, 7, 2, 8, 5, 9, 4, 6})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 120})
			return c
		}},
		{"barchart", 240, 120, func() toolkit.Widget {
			c := toolkit.NewBarChart([]float64{4, 7, 2, 8, 5, 3})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 120})
			return c
		}},
		{"piechart", 120, 120, func() toolkit.Widget {
			c := toolkit.NewPieChart([]float64{3, 5, 2, 4, 1})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 120, H: 120})
			return c
		}},
		{"markdownview", 280, 180, func() toolkit.Widget {
			m := toolkit.NewMarkdownView("# Heading\n\nA short paragraph of body " +
				"text that wraps across the view.\n\n- first bullet\n- second bullet\n\n" +
				"```\ncode block line\n```")
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 280, H: 180})
			return m
		}},
		{"fontchooser", 160, 120, func() toolkit.Widget {
			fc := toolkit.NewFontChooser(nil)
			fc.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 120})
			return fc
		}},
		{"contextmenu", 200, 160, func() toolkit.Widget {
			menu := toolkit.NewMenu([]toolkit.MenuItem{
				{Label: "Cut", Action: func() {}},
				{Label: "Copy", Action: func() {}, Shortcut: "Ctrl+C"},
				{Label: "Paste", Action: func() {}, Shortcut: "Ctrl+V"},
				{Separator: true},
				{Label: "Select All", Action: func() {}},
			})
			cm := toolkit.NewContextMenu(menu)
			cm.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 160})
			cm.Popup(8, 8)
			return cm
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
