// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ndarray/go-ndarray.github.io/playground/engine"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit"
)

const testW, testH = 1280, 800

func newTestState(t *testing.T, dark bool) *State {
	t.Helper()
	SetupText(1)
	return NewState(testW, testH, dark)
}

func px(buf []byte, w, x, y int) toolkit.RGBA {
	o := (y*w + x) * 4
	return toolkit.RGBA{R: buf[o], G: buf[o+1], B: buf[o+2], A: buf[o+3]}
}

// countIn counts the pixels of r equal to c.
func countIn(buf []byte, w int, r toolkit.Rect, c toolkit.RGBA) int {
	n := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if px(buf, w, x, y) == c {
				n++
			}
		}
	}
	return n
}

func center(r toolkit.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// The frame, in both themes: the theme background, the heatmap cells in the
// viridis colours, and the accent on the selected step.
func TestDrawBothThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := newTestState(t, dark)
		buf := make([]byte, testW*testH*4)
		s.Draw(buf)
		if s.Dirty() {
			t.Error("Draw must clear dirty")
		}
		if got := px(buf, testW, 1, 1); got != s.theme.Background {
			t.Errorf("dark=%v: corner %v, want the background %v", dark, got, s.theme.Background)
		}
		// g = b + c is [[10 21 32] [13 24 35]]: 10 is the viridis start, 35 its end.
		tb := s.table.Bounds()
		if n := countIn(buf, testW, tb, toolkit.Viridis(0)) + countIn(buf, testW, tb, toolkit.Viridis(1)); n < 500 {
			t.Errorf("dark=%v: %d viridis-end pixels in the table, want a heatmap", dark, n)
		}
		if n := countIn(buf, testW, s.steps.Bounds(), s.theme.Accent); n < 500 {
			t.Errorf("dark=%v: the selected step has no accent band (%d px)", dark, n)
		}
		if dark != s.Dark() {
			t.Error("Dark()")
		}
	}
}

// A visitor's sequence, through the same entry points the browser shell
// calls: pick an operation in the popover, edit the arguments, press Enter,
// read the error; click a step; switch the code language; scroll.
func TestInteractionSequence(t *testing.T) {
	s := newTestState(t, false)
	v := s.VM
	buf := make([]byte, testW*testH*4)

	// Click step b in the pipeline list (rows from the top of the list).
	lb := s.steps.Bounds()
	for y := lb.Y + 2; y < lb.Y+lb.H && v.Selected.Get() != 1; y += 3 {
		s.HandleClick(lb.X+20, y)
	}
	if v.Selected.Get() != 1 || s.Debug().Input != "b" {
		t.Fatalf("clicking the list selected %d, input %q", v.Selected.Get(), s.Debug().Input)
	}

	// Open the operation menu and click Transpose (index 8, in the first window).
	s.HandleClick(center(s.ops.Bounds()))
	if !s.ops.PopoverOpen() {
		t.Fatal("the operation menu did not open")
	}
	s.Draw(buf) // the popover paints over the scene
	pb := s.ops.PopoverBounds()
	rowH := pb.H / toolkit.PopoverMaxRows
	idx := 8
	s.HandleClick(pb.X+pb.W/2, pb.Y+idx*rowH+rowH/2)
	if v.OpNames[v.OpIndex.Get()] != "Transpose" || s.ops.PopoverOpen() {
		t.Fatalf("op %s, open %v", v.OpNames[v.OpIndex.Get()], s.ops.PopoverOpen())
	}
	s.HandleClick(center(s.apply.Bounds()))
	if v.Steps.Len() != 8 {
		t.Fatalf("Apply: %d steps", v.Steps.Len())
	}

	// Pick Reshape, type a bad shape, press Enter.
	s.HandleClick(center(s.ops.Bounds()))
	s.HandleClick(pb.X+pb.W/2, pb.Y+7*rowH+rowH/2)
	s.HandleClick(center(s.args.Bounds()))
	s.HandleKeyDown("End")
	for range 10 {
		s.HandleKeyDown("Backspace")
	}
	for _, ch := range "4, 4" {
		s.HandleChar(string(ch))
	}
	if v.Args.Get() != "4, 4" {
		t.Fatalf("typed args %q", v.Args.Get())
	}
	s.HandleKeyDown("Enter")
	if !strings.HasPrefix(v.Error.Get(), "ndarray: shape mismatch") {
		t.Fatalf("error %q", v.Error.Get())
	}
	s.Draw(buf)
	if n := countIn(buf, testW, s.errLabel.Bounds(), errorInk(false)); n < 10 {
		t.Errorf("the error is not on screen in red (%d px)", n)
	}

	// The NumPy half of the language switch.
	lr := s.lang.Bounds()
	s.HandleClick(lr.X+lr.W*3/4, lr.Y+lr.H/2)
	if !strings.HasPrefix(v.Code.Get(), "import numpy") {
		t.Errorf("code %q", v.Code.Get()[:20])
	}

	// Escape closes an open popover; a click outside closes it too.
	s.HandleClick(center(s.presets.Bounds()))
	if !s.presets.PopoverOpen() {
		t.Fatal("presets did not open")
	}
	pp := s.presets.PopoverBounds()
	if !s.HandleScroll(pp.X+5, pp.Y+5, 0, 1) {
		t.Error("scroll over a popover")
	}
	s.HandleKeyDown("Escape")
	if s.presets.PopoverOpen() {
		t.Error("Escape must close the popover")
	}

	// Scroll the code view and drag/release over the table.
	cb := s.code.Bounds()
	s.HandleScroll(cb.X+10, cb.Y+10, 0, 3)
	if s.code.ScrollLine().Get() == 0 {
		t.Error("the wheel did not scroll the code")
	}
	tb := s.table.Bounds()
	s.HandleClick(tb.X+5, tb.Y+5)
	if !s.HandleMove(tb.X+6, tb.Y+30) || !s.HandleRelease(tb.X+6, tb.Y+30) {
		t.Error("a press on the table captures the drag")
	}
	if s.HandleRelease(0, 0) {
		t.Error("a release with nothing captured changes nothing")
	}
	s.HandleClick(1, 1) // empty space: no capture
	s.HandleMove(2, 2)
}

func TestResizeStacksWhenNarrow(t *testing.T) {
	s := newTestState(t, false)
	if s.left.Bounds().Y != s.right.Bounds().Y {
		t.Error("wide: the two columns sit side by side")
	}
	s.Resize(600, 1000)
	if w, h := s.Size(); w != 600 || h != 1000 {
		t.Error("Size")
	}
	if s.right.Bounds().Y <= s.left.Bounds().Y {
		t.Error("narrow: the columns stack")
	}
	buf := make([]byte, 600*1000*4)
	s.Draw(buf)
	s.Resize(0, 0)
	if w, h := s.Size(); w != 1 || h != 1 {
		t.Error("Resize clamps to 1x1")
	}
}

func TestSetTheme(t *testing.T) {
	s := newTestState(t, false)
	s.SetTheme(true)
	if !s.Dark() || s.theme.Background != toolkit.RGB(0x0b, 0x0e, 0x14) || s.title.Ink != brandText(true) {
		t.Error("SetTheme(true)")
	}
	if Theme(false).Accent != toolkit.RGB(0x04, 0x78, 0x57) || Theme(true).Accent != toolkit.RGB(0x04, 0x78, 0x57) {
		t.Error("the accent is the brand emerald in both themes")
	}
	SetupText(0) // a non-positive scale falls back to 1
	if toolkit.MetricScale() != 1 {
		t.Error("SetupText(0)")
	}
}

func TestBindings(t *testing.T) {
	obs := mvvm.NewObservable("abc")
	e := toolkit.NewEntry("")
	n := 0
	un := bindEntry(e, obs, func() { n++ })
	if e.Text().Get() != "abc" {
		t.Error("seed")
	}
	obs.Set("x")
	if e.Text().Get() != "x" || n == 0 {
		t.Error("VM to entry")
	}
	e.Text().Set("y")
	if obs.Get() != "y" {
		t.Error("entry to VM")
	}
	un()

	g := mvvm.NewObservableEq[*engine.Grid](nil, nil)
	tb := toolkit.NewTable(nil, nil)
	un = bindGrid(tb, g, func() {})
	if tb.Columns != nil || tb.CellFill != nil {
		t.Error("nil grid is an empty table")
	}
	g.Set(&engine.Grid{Values: [][]float64{{1, math.NaN(), 3}}, RowLabels: []string{"0"}, ColLabels: []string{"0", "1", "2"}, Min: 1, Max: 3})
	if len(tb.Columns) != 4 || tb.Rows[0][2] != "NaN" {
		t.Errorf("columns %d row %v", len(tb.Columns), tb.Rows)
	}
	if _, ok := tb.CellFill(0, 0); ok {
		t.Error("the row-label column is not filled")
	}
	if _, ok := tb.CellFill(0, 2); ok {
		t.Error("NaN is not filled")
	}
	if _, ok := tb.CellFill(5, 1); ok {
		t.Error("out of range is not filled")
	}
	if c, ok := tb.CellFill(0, 3); !ok || c != toolkit.Viridis(1) {
		t.Error("the max is the top of the scale")
	}
	g.Set(&engine.Grid{Values: [][]float64{{7}}, RowLabels: []string{"0"}, ColLabels: []string{"0"}, Min: 7, Max: 7})
	if c, _ := tb.CellFill(0, 1); c != toolkit.Viridis(0.5) {
		t.Error("a constant array sits mid-scale")
	}
	g.Set(&engine.Grid{})
	if tb.Rows != nil {
		t.Error("an empty grid clears the table")
	}
	un()
}

func TestDebugAndRects(t *testing.T) {
	s := newTestState(t, true)
	d := s.Debug()
	if len(d.Steps) != 7 || d.Input != "g" || d.Rows != 2 || d.Cols != 4 || !d.Dark || d.MaxProcs < 1 || len(d.Ops) == 0 {
		t.Errorf("%+v", d)
	}
	s.VM.InputIndex.Set(99)
	if s.Debug().Input != "" {
		t.Error("no input")
	}
	r := s.Rects()
	if r["apply"].W == 0 || r["table"].H == 0 || len(r) != 15 {
		t.Errorf("rects %v", r)
	}
}

// The capture directory refuses a git work tree and accepts a plain one.
func TestCaptureDir(t *testing.T) {
	tree := t.TempDir()
	if err := os.Mkdir(filepath.Join(tree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(tree, "sub", "captures")
	if got := gitTreeOf(inside); got != tree {
		t.Fatalf("gitTreeOf(%s) = %q, want %q", inside, got, tree)
	}
	outside := t.TempDir()
	if got := gitTreeOf(outside); got != "" {
		t.Skipf("the temp dir itself sits in a git tree (%s)", got)
	}
	t.Setenv("PLAYGROUND_CAPTURE_DIR", outside)
	if captureDir(t) != outside {
		t.Error("captureDir must honour the variable")
	}
	buf := make([]byte, 4*4*4)
	savePNG(t, "probe.png", buf, 4, 4)
	if _, err := os.Stat(filepath.Join(outside, "probe.png")); err != nil {
		t.Error(err)
	}
}

// With PLAYGROUND_LOOK=1, write the opening frame in both themes for a
// person to look at (outside every repository).
func TestLookCapture(t *testing.T) {
	if os.Getenv("PLAYGROUND_LOOK") == "" {
		t.Skip("set PLAYGROUND_LOOK=1 to write captures")
	}
	for _, dark := range []bool{false, true} {
		s := newTestState(t, dark)
		buf := make([]byte, testW*testH*4)
		s.Draw(buf)
		name := map[bool]string{false: "light.png", true: "dark.png"}[dark]
		savePNG(t, name, buf, testW, testH)
	}
}
