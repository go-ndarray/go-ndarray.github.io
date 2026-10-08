// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package playground is the View of the go-ndarray playground: a go-widgets
// canvas application compiled to WebAssembly. It builds toolkit widgets, lays
// them out in boxes and binds each one to the ViewModel (package vm) — it
// draws nothing by hand and holds no state of its own beyond layout and
// pointer capture. cmd/playground-wasm is the browser shell around it.
package playground

import (
	"github.com/go-ndarray/go-ndarray.github.io/playground/vm"
	"github.com/go-widgets/mvvmtk"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Logical sizes, before the device scale (toolkit.Scaled) is applied.
const (
	margin     = 12
	gap        = 8
	barH       = 34
	rowH       = 30
	labelH     = 22
	leftW      = 380
	codeH      = 220
	narrowW    = 860 // below this the two columns stack
	stackedTop = 420 // height of the form when stacked
	cellW      = 76
	rowLabelW  = 44
)

// baseFontPx is the body text size in logical pixels.
const baseFontPx = 14

// SetupText selects the toolkit's anti-aliased text at the device scale, so
// one logical pixel is one CSS pixel and text is crisp on a HiDPI screen. It
// must run before the first layout measures text.
func SetupText(scale float64) {
	if scale <= 0 {
		scale = 1
	}
	toolkit.SetMetricScale(scale)
	_ = toolkit.UseOpenTypeTextSize(int(float64(baseFontPx)*scale + 0.5))
}

// State is the View: the widgets, their layout, the ViewModel they are bound
// to, and the pointer capture.
type State struct {
	w, h  int
	dark  bool
	theme *toolkit.Theme
	VM    *vm.ViewModel

	// top bar
	title   *toolkit.Label
	presets *toolkit.DropDown
	reset   *toolkit.Button

	// left: the pipeline and the form
	pipeLbl  *toolkit.Label
	steps    *toolkit.ListBox
	opLbl    *toolkit.Label
	ops      *toolkit.DropDown
	inLbl    *toolkit.Label
	inputs   *toolkit.DropDown
	hint     *toolkit.Label
	args     *toolkit.Entry
	apply    *toolkit.Button
	undo     *toolkit.Button
	errLabel *toolkit.Label

	// right: the selected step
	stTitle *toolkit.Label
	shape   *toolkit.Label
	memory  *toolkit.Label
	timing  *toolkit.Label
	goLine  *toolkit.Label
	pyLine  *toolkit.Label
	note    *toolkit.Label
	table   *toolkit.Table
	lang    *toolkit.ViewSwitcher
	copyB   *toolkit.Button
	code    *toolkit.TextView

	status *toolkit.Label

	root, body, left, right *box
	formBtns, codeBar, top  *toolkit.HBox

	dropdowns []*toolkit.DropDown

	// Pointer capture: the widget a press landed on gets the drag and the
	// release, in its own coordinates, even when the pointer leaves it.
	dragTarget toolkit.Widget

	dirty  bool
	unbind []func()
}

// box is a VBox or an HBox: the body flips between the two with the width.
type box struct {
	toolkit.Widget
	add func(w toolkit.Widget, fixed, flex int)
}

func newVBox(spacing int) *box {
	b := toolkit.NewVBox()
	b.Spacing = spacing
	return &box{Widget: b, add: func(w toolkit.Widget, fixed, flex int) {
		if flex > 0 {
			b.AddFlex(w, flex)
		} else {
			b.AddFixed(w, fixed)
		}
	}}
}

func newHBox(spacing int) *box {
	b := toolkit.NewHBox()
	b.Spacing = spacing
	return &box{Widget: b, add: func(w toolkit.Widget, fixed, flex int) {
		if flex > 0 {
			b.AddFlex(w, flex)
		} else {
			b.AddFixed(w, fixed)
		}
	}}
}

// NewState builds the View over a fresh ViewModel, w×h device pixels.
func NewState(w, h int, dark bool) *State {
	s := &State{VM: vm.New()}
	s.dark = dark
	s.theme = Theme(dark)
	inv := func() { s.dirty = true }
	v := s.VM

	s.title = toolkit.NewLabel("go-ndarray playground")
	s.presets = toolkit.NewDropDown(v.PresetNames, 0)
	s.reset = toolkit.NewButton("Clear", nil)

	s.pipeLbl = toolkit.NewLabel("Pipeline — click a step to inspect it")
	s.steps = toolkit.NewListBox(nil)
	s.opLbl = toolkit.NewLabel("Operation")
	s.ops = toolkit.NewDropDown(v.OpNames, 0)
	s.inLbl = toolkit.NewLabel("Input")
	s.inputs = toolkit.NewDropDown(nil, 0)
	s.hint = toolkit.NewLabel("")
	s.args = toolkit.NewEntry("")
	s.apply = toolkit.NewButton("Apply", nil)
	s.undo = toolkit.NewButton("Remove last", nil)
	s.errLabel = toolkit.NewLabel("")

	s.stTitle = toolkit.NewLabel("")
	s.shape = toolkit.NewLabel("")
	s.memory = toolkit.NewLabel("")
	s.timing = toolkit.NewLabel("")
	s.goLine = toolkit.NewLabel("")
	s.pyLine = toolkit.NewLabel("")
	s.note = toolkit.NewLabel("")
	s.table = toolkit.NewTable(nil, nil)
	s.lang = toolkit.NewViewSwitcher([]string{"Go program", "NumPy equivalent"}, 0)
	s.copyB = toolkit.NewButton("Copy", nil)
	s.code = toolkit.NewTextView("")
	s.code.ShowLineNumbers = true

	s.status = toolkit.NewLabel("")

	for _, l := range []*toolkit.Label{s.hint, s.errLabel, s.memory, s.shape, s.timing, s.goLine, s.pyLine, s.note, s.status} {
		l.Ellipsis = true
	}

	// Styles first: BindCommand remembers a button's resting style to restore
	// it when the command becomes executable again.
	s.apply.Style = toolkit.ButtonProminent
	s.undo.Style = toolkit.ButtonSecondary
	s.reset.Style = toolkit.ButtonSecondary
	s.copyB.Style = toolkit.ButtonSecondary
	s.args.Placeholder = "arguments"
	s.args.OnSubmit = func(string) { v.Apply.Execute() }

	s.unbind = append(s.unbind,
		mvvmtk.BindSelectedIndex(s.presets, v.PresetIndex, inv),
		mvvmtk.BindCommand(s.reset, v.Reset, inv),
		mvvmtk.BindListItems(s.steps, v.Steps, func(r string) string { return r }, inv),
		mvvmtk.BindListSelection(s.steps, v.Selected, inv),
		mvvmtk.BindSelectedIndex(s.ops, v.OpIndex, inv),
		mvvmtk.BindDropDownOptions(s.inputs, v.Inputs, func(n string) string { return n }, inv),
		mvvmtk.BindSelectedIndex(s.inputs, v.InputIndex, inv),
		mvvmtk.BindLabel(s.hint, v.Hint, inv),
		bindEntry(s.args, v.Args, inv),
		mvvmtk.BindCommand(s.apply, v.Apply, inv),
		mvvmtk.BindCommand(s.undo, v.RemoveLast, inv),
		mvvmtk.BindLabel(s.errLabel, v.Error, inv),
		mvvmtk.BindLabel(s.stTitle, v.Title, inv),
		mvvmtk.BindLabel(s.shape, v.Shape, inv),
		mvvmtk.BindLabel(s.memory, v.Memory, inv),
		mvvmtk.BindLabel(s.timing, v.Timing, inv),
		mvvmtk.BindLabel(s.goLine, v.GoLine, inv),
		mvvmtk.BindLabel(s.pyLine, v.PyLine, inv),
		mvvmtk.BindLabel(s.note, v.Note, inv),
		bindGrid(s.table, v.Grid, inv),
		mvvmtk.BindViewSwitcher(s.lang, v.CodeLang, inv),
		mvvmtk.BindCommand(s.copyB, v.CopyCode, inv),
		mvvmtk.BindTextView(s.code, v.Code, inv),
		mvvmtk.BindLabel(s.status, v.Status, inv),
	)
	s.dropdowns = []*toolkit.DropDown{s.presets, s.ops, s.inputs}
	s.applyTheme()
	s.Resize(w, h)
	return s
}

// layout (re)builds the box tree for the current width: two columns side by
// side, or stacked on a narrow screen.
func (s *State) layout() {
	sc := toolkit.Scaled
	narrow := s.w < sc(narrowW)

	s.top = toolkit.NewHBox()
	s.top.Spacing = sc(gap)
	s.top.AddFlex(s.title, 1)
	s.top.AddFixed(s.presets, sc(240))
	s.top.AddFixed(s.reset, sc(80))

	s.formBtns = toolkit.NewHBox()
	s.formBtns.Spacing = sc(gap)
	s.formBtns.AddFlex(s.apply, 1)
	s.formBtns.AddFlex(s.undo, 1)

	s.left = newVBox(sc(gap / 2))
	s.left.add(s.pipeLbl, sc(labelH), 0)
	s.left.add(s.steps, 0, 1)
	s.left.add(s.opLbl, sc(labelH), 0)
	s.left.add(s.ops, sc(rowH), 0)
	s.left.add(s.inLbl, sc(labelH), 0)
	s.left.add(s.inputs, sc(rowH), 0)
	s.left.add(s.hint, sc(labelH), 0)
	s.left.add(s.args, sc(rowH), 0)
	s.left.add(s.formBtns, sc(rowH+2), 0)
	s.left.add(s.errLabel, sc(labelH), 0)

	s.codeBar = toolkit.NewHBox()
	s.codeBar.Spacing = sc(gap)
	s.codeBar.AddFlex(s.lang, 1)
	s.codeBar.AddFixed(s.copyB, sc(90))

	s.right = newVBox(sc(2))
	s.right.add(s.stTitle, sc(labelH+4), 0)
	s.right.add(s.shape, sc(labelH), 0)
	s.right.add(s.memory, sc(labelH), 0)
	s.right.add(s.timing, sc(labelH), 0)
	s.right.add(s.goLine, sc(labelH), 0)
	s.right.add(s.pyLine, sc(labelH), 0)
	s.right.add(s.note, sc(labelH), 0)
	s.right.add(s.table, 0, 1)
	s.right.add(s.codeBar, sc(rowH+2), 0)
	s.right.add(s.code, sc(codeH), 0)

	if narrow {
		s.body = newVBox(sc(gap * 2))
		s.body.add(s.left, sc(stackedTop), 0)
		s.body.add(s.right, 0, 1)
	} else {
		s.body = newHBox(sc(gap * 2))
		s.body.add(s.left, sc(leftW), 0)
		s.body.add(s.right, 0, 1)
	}
	s.root = newVBox(sc(gap))
	s.root.add(s.top, sc(barH), 0)
	s.root.add(s.body, 0, 1)
	s.root.add(s.status, sc(labelH), 0)
	m := sc(margin)
	s.root.SetBounds(toolkit.Rect{X: m, Y: m, W: s.w - 2*m, H: s.h - 2*m})
	s.dirty = true
}

// Resize lays the scene out for w×h device pixels.
func (s *State) Resize(w, h int) {
	s.w, s.h = max(w, 1), max(h, 1)
	s.layout()
}

// Size is the scene size in device pixels.
func (s *State) Size() (int, int) { return s.w, s.h }

// Dark reports whether the dark palette is active.
func (s *State) Dark() bool { return s.dark }

// SetTheme switches between the light and the dark palette.
func (s *State) SetTheme(dark bool) {
	s.dark = dark
	s.theme = Theme(dark)
	s.applyTheme()
	s.dirty = true
}

// applyTheme sets the ink of the labels that are not plain body text.
func (s *State) applyTheme() {
	muted := mutedInk(s.dark)
	s.title.Ink = brandText(s.dark)
	s.title.FontSize = toolkit.Scaled(19)
	s.stTitle.Ink = brandText(s.dark)
	for _, l := range []*toolkit.Label{s.hint, s.memory, s.timing, s.note, s.status, s.pipeLbl, s.opLbl, s.inLbl} {
		l.Ink = muted
	}
	s.errLabel.Ink = errorInk(s.dark)
}

// Draw paints the whole scene into buf (w×h RGBA). Open popovers are drawn
// last so they float above the widgets under them.
func (s *State) Draw(buf []byte) {
	p := painter.NewPixelPainter(buf, s.w, s.h)
	fillBackground(buf, s.theme.Background)
	s.root.Draw(p, s.theme)
	for _, d := range s.dropdowns {
		d.DrawPopover(p, s.theme)
	}
	s.dirty = false
}

// Dirty reports whether something changed since the last Draw.
func (s *State) Dirty() bool { return s.dirty }

// Theme returns the playground palette: the toolkit defaults with the
// landing's neutral backgrounds and its emerald accent (go-ndarray.github.io's
// [params.brand]: base #047857 in light, bright #34d399 in dark).
func Theme(dark bool) *toolkit.Theme {
	if dark {
		t := toolkit.DefaultDark()
		t.Background = toolkit.RGB(0x0b, 0x0e, 0x14)
		t.Surface = toolkit.RGB(0x14, 0x1a, 0x24)
		t.SurfaceAlt = toolkit.RGB(0x1c, 0x23, 0x31)
		t.Border = toolkit.RGB(0x2a, 0x33, 0x42)
		t.OnBackground = toolkit.RGB(0xe6, 0xed, 0xf3)
		t.OnSurface = toolkit.RGB(0xe6, 0xed, 0xf3)
		// Fills keep the base emerald, which carries white text in both
		// themes (5.5:1); text drawn IN the accent uses brandText instead.
		t.Accent = toolkit.RGB(0x04, 0x78, 0x57)
		white := toolkit.RGB(0xff, 0xff, 0xff)
		t.Extra = map[string]toolkit.RGBA{"OnAccent": white, "accent_fg_color": white}
		return t
	}
	t := toolkit.DefaultLight()
	t.Background = toolkit.RGB(0xff, 0xff, 0xff)
	t.Surface = toolkit.RGB(0xf7, 0xf8, 0xfb)
	t.SurfaceAlt = toolkit.RGB(0xe7, 0xe9, 0xee)
	t.Border = toolkit.RGB(0xe0, 0xe3, 0xea)
	t.OnBackground = toolkit.RGB(0x0f, 0x11, 0x15)
	t.OnSurface = toolkit.RGB(0x0f, 0x11, 0x15)
	t.Accent = toolkit.RGB(0x04, 0x78, 0x57)
	white := toolkit.RGB(0xff, 0xff, 0xff)
	t.Extra = map[string]toolkit.RGBA{"OnAccent": white, "accent_fg_color": white}
	return t
}

// brandText is the accent as a text colour: the landing's --accent, base
// emerald on white, bright emerald on the dark background (10:1).
func brandText(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0x34, 0xd3, 0x99)
	}
	return toolkit.RGB(0x04, 0x78, 0x57)
}

// mutedInk is the landing's --slate.
func mutedInk(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0x9a, 0xa4, 0xb2)
	}
	return toolkit.RGB(0x47, 0x55, 0x69)
}

// errorInk is a red that reads on the background of each theme.
func errorInk(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0xf7, 0x8b, 0x8b)
	}
	return toolkit.RGB(0xb4, 0x23, 0x18)
}

// fillBackground clears the frame to the theme background before the widgets
// paint over it (the app-template's fillBG): a buffer clear, not drawing.
func fillBackground(buf []byte, c toolkit.RGBA) {
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = c.R, c.G, c.B, c.A
	}
}
