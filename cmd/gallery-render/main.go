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

	"github.com/go-widgets/painter"
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
		{"dropdown-openup", 200, 90, func() toolkit.Widget {
			// The control sits at the BOTTOM of the pane with OpenUp + Open
			// set, so the host-drawn popover (an Overlay layer built from
			// DropDown.PopoverBounds()) renders ABOVE it instead of below —
			// the layout PopoverBounds computes when OpenUp is set.
			d := toolkit.NewDropDown([]string{"UTF-8", "Latin-1", "Shift-JIS"}, 0)
			d.OpenUp = true
			d.Open = true
			ov := toolkit.NewOverlay(d)
			ov.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 90})
			d.SetBounds(toolkit.Rect{X: 0, Y: 60, W: 200, H: 30})
			list := toolkit.NewListBox(d.Options)
			list.SetBounds(d.PopoverBounds())
			ov.Push(list)
			return ov
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
		{"notification-corner", 260, 120, func() toolkit.Widget {
			host := toolkit.Rect{X: 0, Y: 0, W: 260, H: 120}
			n := toolkit.NewNotification("Reconnected")
			n.Visible = true
			n.AnchorIn(host, toolkit.TopLeft)
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
		{"steps-vertical", 140, 200, func() toolkit.Widget {
			s := toolkit.NewSteps([]string{"Plan", "Build", "Test", "Ship"}, 2)
			s.Orientation = toolkit.Vertical
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 140, H: 200})
			return s
		}},
		{"toolbar-vertical", 32, 200, func() toolkit.Widget {
			tb := toolkit.NewToolbar([]toolkit.ToolbarItem{
				{Label: "New"},
				{Label: "Open"},
				{Separator: true},
				{Label: "Save"},
				{Label: "Cut"},
			})
			tb.Orientation = toolkit.Vertical
			tb.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 32, H: 200})
			return tb
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
		{"table-aligned", 320, 100, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Name", Width: 140},
				{Title: "Count", Width: 80, Align: toolkit.AlignRight},
				{Title: "Status", Align: toolkit.AlignCenter},
			}
			rows := [][]string{
				{"apples", "128", "OK"},
				{"oranges", "4,096", "WARN"},
				{"pears", "12", "OK"},
			}
			t := toolkit.NewTable(cols, rows)
			t.Selected = 0
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
		{"toast-corner", 260, 120, func() toolkit.Widget {
			host := toolkit.Rect{X: 0, Y: 0, W: 260, H: 120}
			t := toolkit.NewToast("Copied to clipboard", toolkit.ToastSuccess)
			t.Visible = true
			t.AnchorIn(host, toolkit.BottomRight, 0)
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
		{"timeline-horizontal", 480, 80, func() toolkit.Widget {
			t := toolkit.NewTimeline([]toolkit.TimelineEvent{
				{Title: "PR opened", Kind: toolkit.TimelineDefault},
				{Title: "Reviewed", Detail: "LGTM", Kind: toolkit.TimelineSuccess},
				{Title: "Build failed", Kind: toolkit.TimelineError},
				{Title: "Force-pushed", Kind: toolkit.TimelineWarning},
			})
			t.Horizontal = true
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 480, H: 80})
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
		{"notebook-tabs-left", 320, 140, func() toolkit.Widget {
			n := toolkit.NewNotebook()
			n.AddTab("One", toolkit.NewLabel("first tab body"))
			n.AddTab("Two", toolkit.NewLabel("second tab body"))
			n.TabSide = toolkit.TabLeft
			n.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 140})
			return n
		}},
		{"notebook-tabs-bottom", 320, 140, func() toolkit.Widget {
			n := toolkit.NewNotebook()
			n.AddTab("One", toolkit.NewLabel("first tab body"))
			n.AddTab("Two", toolkit.NewLabel("second tab body"))
			n.TabSide = toolkit.TabBottom
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
		// v0.42.0 catalogue-completion additions.
		{"accordion", 240, 160, func() toolkit.Widget {
			a := toolkit.NewAccordion([]toolkit.AccordionSection{
				{Title: "General", Body: toolkit.NewLabel("general settings")},
				{Title: "Advanced", Body: toolkit.NewLabel("advanced settings")},
				{Title: "About", Body: toolkit.NewLabel("version 0.42.0")},
			})
			a.Expanded = 1
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 160})
			return a
		}},
		{"carousel", 240, 140, func() toolkit.Widget {
			slide := func(text string) toolkit.Widget {
				l := toolkit.NewLabel(text)
				return l
			}
			c := toolkit.NewCarousel([]toolkit.Widget{
				slide("Slide one"),
				slide("Slide two"),
				slide("Slide three"),
			})
			c.Current = 1
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 140})
			return c
		}},
		{"command-palette", 320, 180, func() toolkit.Widget {
			cp := toolkit.NewCommandPalette([]toolkit.PaletteCommand{
				{Label: "Open File", Action: func() {}},
				{Label: "Open Folder", Action: func() {}},
				{Label: "Close Window", Action: func() {}},
				{Label: "Toggle Sidebar", Action: func() {}},
			})
			cp.Open()
			cp.Query = "open"
			cp.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 180})
			return cp
		}},
		{"wizard", 320, 200, func() toolkit.Widget {
			w := toolkit.NewWizard([]toolkit.WizardStep{
				{Title: "Account", Body: toolkit.NewLabel("account details")},
				{Title: "Profile", Body: toolkit.NewLabel("profile details")},
				{Title: "Review", Body: toolkit.NewLabel("review + submit")},
			})
			w.Current = 1
			w.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 200})
			return w
		}},
		{"date-range-picker", 190, 190, func() toolkit.Widget {
			d := toolkit.NewDateRangePicker(2026, 7)
			d.Cal.SetToday(2026, 7, 6)
			d.Start = toolkit.Date{Y: 2026, M: 7, D: 10}
			d.End = toolkit.Date{Y: 2026, M: 7, D: 18}
			d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 190, H: 190})
			return d
		}},
		{"color-picker", toolkit.ColorPickerWidth, toolkit.ColorPickerHeight, func() toolkit.Widget {
			c := toolkit.NewColorPicker(toolkit.RGB(0x0d, 0x94, 0x88))
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: toolkit.ColorPickerWidth, H: toolkit.ColorPickerHeight})
			return c
		}},
		{"segmented-bar", 240, 20, func() toolkit.Widget {
			s := toolkit.NewSegmentedBar([]toolkit.BarSegment{
				{Value: 40, Fill: toolkit.RGB(0x0d, 0x94, 0x88)},
				{Value: 25, Fill: toolkit.RGB(0xf5, 0xa6, 0x23)},
				{Value: 15, Fill: toolkit.RGB(0xc0, 0x39, 0x2b)},
			})
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 20})
			return s
		}},
		{"markdown-editor", 360, 180, func() toolkit.Widget {
			m := toolkit.NewMarkdownEditor("# Title\n\nSome **bold** sample text.\n\n- one\n- two")
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 360, H: 180})
			return m
		}},
		{"treetable", 320, 160, func() toolkit.Widget {
			root := []*toolkit.TreeTableNode{
				{
					Cells:    []string{"src", "", "dir"},
					Expanded: true,
					Children: []*toolkit.TreeTableNode{
						{Cells: []string{"main.go", "4.8 KB", "source"}},
						{Cells: []string{"scene.go", "2.1 KB", "source"}},
					},
				},
				{Cells: []string{"README.md", "1.2 KB", "text"}},
			}
			cols := []toolkit.TreeTableColumn{
				{Title: "Name", Width: 160},
				{Title: "Size", Width: 80},
				{Title: "Kind"},
			}
			t := toolkit.NewTreeTable(cols, root)
			t.Selected = root[0].Children[0]
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 160})
			return t
		}},
		{"table-multiselect", 320, 100, func() toolkit.Widget {
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
			t.MultiSelect = true
			t.SetRowSelection(0, 2)
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 100})
			return t
		}},
		{"menu-checkable", 180, 110, func() toolkit.Widget {
			m := toolkit.NewMenu([]toolkit.MenuItem{
				{Label: "Word Wrap", Checkable: true, Checked: true},
				{Separator: true},
				{Label: "Light Theme", RadioGroup: 1, Checked: true},
				{Label: "Dark Theme", RadioGroup: 1},
			})
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 180, H: 110})
			return m
		}},
		{"table-editing", 320, 100, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Task", Width: 150},
				{Title: "Owner", Width: 100, Editable: true},
				{Title: "Status"},
			}
			rows := [][]string{
				{"Ship cell editing", "alice", "done"},
				{"Frozen columns", "bob", "wip"},
				{"Group rows", "carol", "todo"},
			}
			t := toolkit.NewTable(cols, rows)
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 100})
			// Open the inline editor on row 1's Owner cell.
			t.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 200, Y: toolkit.TableHeaderHeight + toolkit.TableRowHeight + 2})
			t.OnEvent(toolkit.Event{Kind: toolkit.EventChar, Code: "y"})
			return t
		}},
		{"table-groups", 300, 140, func() toolkit.Widget {
			cols := []toolkit.TableColumn{{Title: "Status", Width: 90}, {Title: "Task", Width: 200}}
			rows := [][]string{
				{"in-progress", "Ship cell editing"},
				{"in-progress", "Frozen columns"},
				{"todo", "Group rows"},
				{"done", "Sencha layouts"},
			}
			t := toolkit.NewTable(cols, rows)
			t.GroupBy = 0
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 300, H: 140})
			// Collapse the "done" group (visual line 5).
			t.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 20, Y: toolkit.TableHeaderHeight + 5*toolkit.TableRowHeight + 2})
			return t
		}},
		{"table-frozen", 300, 110, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Name", Width: 110},
				{Title: "Q1", Width: 60, Align: toolkit.AlignRight},
				{Title: "Q2", Width: 60, Align: toolkit.AlignRight},
				{Title: "Q3", Width: 60, Align: toolkit.AlignRight},
				{Title: "Total", Width: 70, Align: toolkit.AlignRight},
			}
			rows := [][]string{
				{"Alice", "12", "18", "24", "84"},
				{"Bob", "9", "15", "21", "72"},
				{"Carol", "20", "22", "19", "86"},
			}
			t := toolkit.NewTable(cols, rows)
			t.FrozenColumns = 1
			t.Selected = 2
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 300, H: 110})
			t.ScrollXTo(120) // scroll Q1/Q2 under the frozen Name column
			return t
		}},
		{"frame-panel", 220, 90, func() toolkit.Widget {
			f := toolkit.NewFrame(toolkit.NewLabel("  Body content lives here"))
			f.Title = "Display options"
			f.Collapsible = true
			f.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 220, H: 90})
			return f
		}},
		{"propertygrid", 260, 110, func() toolkit.Widget {
			pg := toolkit.NewPropertyGrid()
			pg.Add("Width", "1024")
			pg.Add("Height", "768")
			pg.Add("Title", "Untitled")
			pg.Add("Visible", "true")
			pg.Table().Selected = 2
			pg.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 110})
			// Open the editor on the "Title" value cell.
			pg.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 190, Y: toolkit.TableHeaderHeight + 2*toolkit.TableRowHeight + 2})
			return pg
		}},
		{"loadmask", 200, 120, func() toolkit.Widget {
			m := toolkit.NewLoadMask("Loading…")
			m.Active = true
			m.Tick(0.12)
			m.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 120})
			return m
		}},
		{"pagingtoolbar", 250, toolkit.PagingBtnH, func() toolkit.Widget {
			pt := toolkit.NewPagingToolbar(6, 12)
			pt.ShowRefresh = true
			pt.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 250, H: toolkit.PagingBtnH})
			return pt
		}},
		{"listbox-dataview", 220, 128, func() toolkit.Widget {
			subs := []string{"12 unread · updated 2m ago", "3 unread · updated 1h ago",
				"all read · updated yesterday", "8 unread · updated 5m ago"}
			swatch := []toolkit.RGBA{{R: 0xE0, G: 0x50, B: 0x50, A: 255}, {R: 0x50, G: 0xA0, B: 0xE0, A: 255},
				{R: 0x50, G: 0xB0, B: 0x70, A: 255}, {R: 0xC0, G: 0x80, B: 0xE0, A: 255}}
			lb := toolkit.NewListBox([]string{"Reddit", "Hacker News", "Lobsters", "GitHub"})
			lb.RowHeight = 32
			lb.Selected = 1
			lb.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 220, H: 128})
			lb.ItemRenderer = func(p painter.Painter, theme *toolkit.Theme, rc toolkit.Rect, i int, item string, sel bool, ink toolkit.RGBA) {
				p.FillRect(painter.Rect{X: rc.X + 8, Y: rc.Y + rc.H/2 - 6, W: 12, H: 12}, swatch[i])
				title := toolkit.NewLabel(item)
				title.Ink = ink
				title.SetBounds(toolkit.Rect{X: rc.X + 28, Y: rc.Y + 4, W: rc.W - 32, H: 14})
				title.Draw(p, theme)
				sub := toolkit.NewLabel(subs[i])
				subInk := ink
				if !sel {
					subInk = toolkit.RGBA{R: 0x90, G: 0x90, B: 0x90, A: 255}
				}
				sub.Ink = subInk
				sub.SetBounds(toolkit.Rect{X: rc.X + 28, Y: rc.Y + 18, W: rc.W - 32, H: 12})
				sub.Draw(p, theme)
			}
			return lb
		}},
		// v0.82.0 dashboard-widget additions.
		{"gantt", 520, 140, func() toolkit.Widget {
			g := toolkit.NewGantt([]toolkit.GanttTask{
				{Label: "Design", Start: 0, End: 3, Progress: 1.0},
				{Label: "Build", Start: 2, End: 7, Progress: 0.6},
				{Label: "Test", Start: 6, End: 9, Progress: 0.25},
				{Label: "Docs", Start: 7, End: 10, Fill: toolkit.RGB(0xf5, 0xa6, 0x23)},
				{Label: "Ship", Start: 9, End: 11, Fill: toolkit.RGB(0xc0, 0x39, 0x2b)},
			})
			g.Selected = 1
			g.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 520, H: 140})
			return g
		}},
		{"kanban", 600, 220, func() toolkit.Widget {
			k := toolkit.NewKanban([]toolkit.KanbanColumn{
				{Title: "To Do", Cards: []toolkit.KanbanCard{
					{Title: "Group rows", Subtitle: "table feature"},
					{Title: "Dark theme", Subtitle: "polish", Accent: toolkit.RGB(0xf5, 0xa6, 0x23)},
				}},
				{Title: "In Progress", Cards: []toolkit.KanbanCard{
					{Title: "Cell editing", Subtitle: "alice"},
					{Title: "Frozen columns", Subtitle: "bob"},
				}},
				{Title: "Done", Cards: []toolkit.KanbanCard{
					{Title: "Sencha layouts", Subtitle: "shipped", Accent: toolkit.RGB(0x50, 0xb0, 0x70)},
				}},
			})
			k.SelectedCol = 1
			k.SelectedCard = 0
			k.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 600, H: 220})
			return k
		}},
		{"sparkline-line", 120, 32, func() toolkit.Widget {
			s := toolkit.NewSparkline([]float64{3, 7, 4, 8, 6, 9, 5, 8, 11, 9})
			s.Kind = toolkit.SparkLine
			s.ShowLast = true
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 120, H: 32})
			return s
		}},
		{"sparkline-bar", 120, 32, func() toolkit.Widget {
			s := toolkit.NewSparkline([]float64{3, 7, 4, 8, 6, 9, 5, 8, 11, 9})
			s.Kind = toolkit.SparkBar
			s.ShowLast = true
			s.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 120, H: 32})
			return s
		}},
		{"agenda-week", 720, 340, func() toolkit.Widget {
			a := toolkit.NewAgenda([]toolkit.AgendaEvent{
				{Title: "Standup", Day: 0, StartMin: 9 * 60, EndMin: 9*60 + 30},
				{Title: "Design review", Day: 1, StartMin: 11 * 60, EndMin: 12 * 60,
					Fill: toolkit.RGB(0xf5, 0xa6, 0x23)},
				{Title: "1:1", Day: 2, StartMin: 14 * 60, EndMin: 15 * 60},
				{Title: "Release", Day: 4, StartMin: 16 * 60, EndMin: 17*60 + 30,
					Fill: toolkit.RGB(0xc0, 0x39, 0x2b)},
			})
			a.Selected = 1
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 720, H: 340})
			return a
		}},
		{"agenda-month", 640, 360, func() toolkit.Widget {
			a := toolkit.NewAgenda([]toolkit.AgendaEvent{
				{Title: "Kickoff", Y: 2026, M: 8, D: 3},
				{Title: "Review", Y: 2026, M: 8, D: 12, Fill: toolkit.RGB(0xf5, 0xa6, 0x23)},
				{Title: "Demo", Y: 2026, M: 8, D: 12},
				{Title: "Retro", Y: 2026, M: 8, D: 20},
				{Title: "Release", Y: 2026, M: 8, D: 28, Fill: toolkit.RGB(0xc0, 0x39, 0x2b)},
			})
			a.View = toolkit.AgendaMonth
			a.Year = 2026
			a.Month = 8
			a.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 640, H: 360})
			return a
		}},
		{"areachart", 260, 180, func() toolkit.Widget {
			c := toolkit.NewAreaChart([][]float64{
				{2, 5, 3, 7, 6, 9, 8},
				{1, 2, 2, 4, 3, 5, 4},
			})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 180})
			return c
		}},
		{"scatterchart", 260, 180, func() toolkit.Widget {
			c := toolkit.NewScatterChart([][]toolkit.ScatterPoint{
				{{X: 1, Y: 2}, {X: 3, Y: 5}, {X: 4, Y: 3}, {X: 6, Y: 7}, {X: 8, Y: 6}},
				{{X: 2, Y: 8}, {X: 5, Y: 4}, {X: 7, Y: 9}, {X: 9, Y: 5}},
			})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 180})
			return c
		}},
		{"radarchart", 240, 220, func() toolkit.Widget {
			c := toolkit.NewRadarChart(
				[]string{"Speed", "Power", "Range", "Cost", "Weight"},
				[][]float64{
					{8, 6, 7, 4, 5},
					{5, 8, 4, 7, 6},
				})
			c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 240, H: 220})
			return c
		}},
		{"table-summary", 320, 140, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Task", Width: 150},
				{Title: "Hours", Width: 80, Align: toolkit.AlignRight, Aggregate: toolkit.AggregateSum},
				{Title: "Done %", Align: toolkit.AlignRight, Aggregate: toolkit.AggregateAvg},
			}
			rows := [][]string{
				{"Design", "12", "100"},
				{"Build", "28", "60"},
				{"Test", "16", "25"},
			}
			t := toolkit.NewTable(cols, rows)
			t.ShowSummary = true
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 140})
			return t
		}},
		{"table-expander", 320, 140, func() toolkit.Widget {
			cols := []toolkit.TableColumn{
				{Title: "Name", Width: 150},
				{Title: "Size", Width: 70},
				{Title: "Kind"},
			}
			rows := [][]string{
				{"main.go", "4.8 KB", "source"},
				{"scene.go", "2.1 KB", "source"},
				{"README.md", "1.2 KB", "text"},
			}
			t := toolkit.NewTable(cols, rows)
			t.RowDetail = func(r int) string {
				return "Detail for " + rows[r][0] + " — last modified today"
			}
			t.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 320, H: 140})
			// Expand row 0 by clicking its column-0 disclosure chevron.
			t.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 6, Y: toolkit.TableHeaderHeight + 2})
			return t
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
