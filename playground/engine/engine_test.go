// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-ndarray/ndarray"
)

// base builds a session holding a = Arange(0,6,1), b = a.Reshape(2,3),
// c = FromData([10,20,30]) and s = b.Max() (a number), the inputs the
// operation table below reads.
func base(t *testing.T) *Session {
	t.Helper()
	s := &Session{}
	for _, c := range []Call{{"Arange", "", "0, 6, 1"}, {"Reshape", "a", "2, 3"}, {"FromData", "", "[10, 20, 30]"}, {"Max", "b", ""}} {
		if _, err := s.Apply(c.Op, c.Input, c.Args); err != nil {
			t.Fatalf("%v: %v", c, err)
		}
	}
	return s
}

// Every operation: its result shape, its Go statement and its NumPy one.
func TestOperations(t *testing.T) {
	cases := []struct {
		op, in, args string
		shape        string
		goStmt, py   string
	}{
		{"Arange", "", "0, 3, 0.5", "(6,)", "e, err := ndarray.Arange(0, 3, 0.5)", "e = np.arange(0, 3, 0.5)"},
		{"Linspace", "", "0, 1, 5", "(5,)", "e, err := ndarray.Linspace(0, 1, 5)", "e = np.linspace(0, 1, 5)"},
		{"Zeros", "", "2, 3", "(2, 3)", "e, err := ndarray.Zeros(2, 3)", "e = np.zeros((2, 3))"},
		{"Ones", "", "(4,)", "(4,)", "e, err := ndarray.Ones(4)", "e = np.ones((4,))"},
		{"Full", "", "7, 2, 2", "(2, 2)", "e, err := ndarray.Full(7, 2, 2)", "e = np.full((2, 2), 7)"},
		{"Full", "", "7", "()", "e, err := ndarray.Full(7)", "e = np.full((), 7)"},
		{"Eye", "", "3, 4, 1", "(3, 4)", "e, err := ndarray.Eye(3, 4, 1)", "e = np.eye(3, 4, 1)"},
		{"FromData", "", "[1, 2, 3, 4], 2, 2", "(2, 2)", "e, err := ndarray.FromData([]float64{1, 2, 3, 4}, 2, 2)", "e = np.array([1, 2, 3, 4]).reshape(2, 2)"},
		{"FromData", "", "[1, 2,]", "(2,)", "e, err := ndarray.FromData([]float64{1, 2}, 2)", "e = np.array([1, 2])"},
		{"Reshape", "a", "3, -1", "(3, 2)", "e, err := a.Reshape(3, -1)", "e = a.reshape(3, -1)"},
		{"Transpose", "b", "", "(3, 2)", "e := b.Transpose()", "e = b.T"},
		{"Flatten", "b", "", "(6,)", "e := b.Flatten()", "e = b.flatten()"},
		{"Copy", "b", "", "(2, 3)", "e := b.Copy()", "e = b.copy()"},
		{"Slice", "b", "1:, ::-1", "(1, 3)", "e, err := b.Slice(ndarray.From(1), ndarray.Step(-1))", "e = b[1:, ::-1]"},
		{"Slice", "b", ":, :2", "(2, 2)", "e, err := b.Slice(ndarray.All(), ndarray.To(2))", "e = b[:, :2]"},
		{"Slice", "b", "0, 0:3:2", "(2,)", "e, err := b.Slice(ndarray.A(0), ndarray.Rng(0, 3, 2))", "e = b[0, 0:3:2]"},
		{"Slice", "b", "0, 1::2", "(1,)", "e, err := b.Slice(ndarray.A(0), ndarray.Rng(1, 3, 2))", "e = b[0, 1::2]"},
		{"Slice", "b", "0, :0:-1", "(2,)", "e, err := b.Slice(ndarray.A(0), ndarray.Rng(2, 0, -1))", "e = b[0, :0:-1]"},
		{"Slice", "b", "0, 1::-1", "(2,)", "e, err := b.Slice(ndarray.A(0), ndarray.Rng(1, -4, -1))", "e = b[0, 1::-1]"},
		{"Slice", "b", "0, 1:3", "(2,)", "e, err := b.Slice(ndarray.A(0), ndarray.R(1, 3))", "e = b[0, 1:3]"},
		{"Squeeze", "b", "", "(2, 3)", "e, err := b.Squeeze()", "e = np.squeeze(b)"},
		{"ExpandDims", "b", "0", "(1, 2, 3)", "e, err := b.ExpandDims(0)", "e = np.expand_dims(b, 0)"},
		{"Exp", "b", "", "(2, 3)", "e := b.Exp()", "e = np.exp(b)"},
		{"Log", "b", "", "(2, 3)", "e := b.Log()", "e = np.log(b)"},
		{"Sqrt", "b", "", "(2, 3)", "e := b.Sqrt()", "e = np.sqrt(b)"},
		{"Sin", "b", "", "(2, 3)", "e := b.Sin()", "e = np.sin(b)"},
		{"Cos", "b", "", "(2, 3)", "e := b.Cos()", "e = np.cos(b)"},
		{"Abs", "b", "", "(2, 3)", "e := b.Abs()", "e = np.abs(b)"},
		{"Square", "b", "", "(2, 3)", "e := b.Square()", "e = np.square(b)"},
		{"Power", "b", "3", "(2, 3)", "e := b.Power(3)", "e = b ** 3"},
		{"Clip", "b", "1, 4", "(2, 3)", "e, err := b.Clip(1, 4)", "e = np.clip(b, 1, 4)"},
		{"Sum", "b", "", "()", "e := b.Sum()", "e = b.sum()"},
		{"Sum", "b", "1", "(2,)", "e, err := b.SumAxis(1, false)", "e = b.sum(axis=1)"},
		{"Mean", "b", "0, keepdims", "(1, 3)", "e, err := b.MeanAxis(0, true)", "e = b.mean(axis=0, keepdims=True)"},
		{"Mean", "b", "", "()", "e, err := b.Mean()", "e = b.mean()"},
		{"Max", "b", "1", "(2,)", "e, err := b.MaxAxis(1, false)", "e = b.max(axis=1)"},
		{"Min", "b", "", "()", "e, err := b.Min()", "e = b.min()"},
		{"Min", "b", "0, true", "(1, 3)", "e, err := b.MinAxis(0, true)", "e = b.min(axis=0, keepdims=True)"},
		{"ArgMax", "b", "", "()", "e, err := b.ArgMax()", "e = b.argmax()"},
		{"ArgMax", "b", "1", "(2,)", "e, err := b.ArgMaxAxis(1, false)", "e = b.argmax(axis=1)"},
		{"CumSum", "b", "", "(6,)", "e := b.CumSumFlat()", "e = np.cumsum(b)"},
		{"CumSum", "b", "1", "(2, 3)", "e, err := b.CumSum(1)", "e = np.cumsum(b, axis=1)"},
		{"Add", "b", "c", "(2, 3)", "e, err := b.Add(c)", "e = b + c"},
		{"Add", "b", "1.5", "(2, 3)", "e := b.AddScalar(1.5)", "e = b + 1.5"},
		{"Sub", "b", "c", "(2, 3)", "e, err := b.Sub(c)", "e = b - c"},
		{"Sub", "b", "1", "(2, 3)", "e := b.SubScalar(1)", "e = b - 1"},
		{"Mul", "b", "2", "(2, 3)", "e := b.MulScalar(2)", "e = b * 2"},
		{"Div", "b", "c", "(2, 3)", "e, err := b.Div(c)", "e = b / c"},
		{"Div", "b", "2", "(2, 3)", "e := b.DivScalar(2)", "e = b / 2"},
		{"Mul", "b", "c", "(2, 3)", "e, err := b.Mul(c)", "e = b * c"},
		{"Maximum", "b", "c", "(2, 3)", "e, err := b.Maximum(c)", "e = np.maximum(b, c)"},
		{"Minimum", "b", "c", "(2, 3)", "e, err := b.Minimum(c)", "e = np.minimum(b, c)"},
		{"Dot", "b", "c", "(2,)", "e, err := b.Dot(c)", "e = np.dot(b, c)"},
		{"Dot", "c", "c", "()", "e, err := c.Dot(c)", "e = np.dot(c, c)"},
		{"Outer", "c", "a", "(3, 6)", "e := c.Outer(a)", "e = np.outer(c, a)"},
		{"Concatenate", "a", "a, 0", "(12,)", "e, err := ndarray.Concatenate([]*ndarray.Array{a, a}, 0)", "e = np.concatenate([a, a], axis=0)"},
		{"Stack", "b", "b, b, 0", "(3, 2, 3)", "e, err := ndarray.Stack([]*ndarray.Array{b, b, b}, 0)", "e = np.stack([b, b, b], axis=0)"},
	}
	for _, c := range cases {
		s := base(t)
		st, err := s.Apply(c.op, c.in, c.args)
		if err != nil {
			t.Errorf("%s(%s) on %q: %v", c.op, c.args, c.in, err)
			continue
		}
		if got := ShapeText(st.Layout.Shape); got != c.shape {
			t.Errorf("%s(%s): shape %s, want %s", c.op, c.args, got, c.shape)
		}
		if st.Go != c.goStmt {
			t.Errorf("%s(%s): Go %q, want %q", c.op, c.args, st.Go, c.goStmt)
		}
		if st.Py != c.py {
			t.Errorf("%s(%s): NumPy %q, want %q", c.op, c.args, st.Py, c.py)
		}
	}
}

// Squeeze with axes, one and several.
func TestSqueezeAxes(t *testing.T) {
	s := &Session{}
	mustApply(t, s, "Zeros", "", "1, 3, 1")
	st := mustApply(t, s, "Squeeze", "a", "0")
	if st.Py != "b = np.squeeze(a, axis=0)" || ShapeText(st.Layout.Shape) != "(3, 1)" {
		t.Errorf("Squeeze(0): %q %v", st.Py, st.Layout.Shape)
	}
	st = mustApply(t, s, "Squeeze", "a", "0, 2")
	if st.Py != "c = np.squeeze(a, axis=(0, 2))" || ShapeText(st.Layout.Shape) != "(3,)" {
		t.Errorf("Squeeze(0, 2): %q %v", st.Py, st.Layout.Shape)
	}
}

func TestMatMul(t *testing.T) {
	s := &Session{}
	mustApply(t, s, "Arange", "", "0, 6, 1")
	mustApply(t, s, "Reshape", "a", "2, 3")
	mustApply(t, s, "Transpose", "b", "")
	st := mustApply(t, s, "MatMul", "b", "c")
	if st.Go != "d, err := b.MatMul(c)" || st.Py != "d = b @ c" || st.Result.At(1, 1) != 50 {
		t.Errorf("MatMul: %q %q %v", st.Go, st.Py, st.Result)
	}
}

func mustApply(t *testing.T, s *Session, op, in, args string) Step {
	t.Helper()
	st, err := s.Apply(op, in, args)
	if err != nil {
		t.Fatalf("%s(%s): %v", op, args, err)
	}
	return st
}

// Wrong arguments are refused with a message, and go-ndarray's own errors
// come through as go-ndarray words them.
func TestErrors(t *testing.T) {
	cases := []struct{ op, in, args, want string }{
		{"Nope", "", "", "unknown operation"},
		{"Reshape", "", "2", "needs an input"},
		{"Reshape", "zz", "2", `no step named "zz"`},
		{"Reshape", "d", "2", "is a number in Go"},
		{"Arange", "", "0, 1", "Arange takes start, stop, step"},
		{"Arange", "", "0, x, 1", `"x" is not a number`},
		{"Arange", "", "0, 1e9, 1", "would exceed"},
		{"Linspace", "", "0, 1", "Linspace takes"},
		{"Linspace", "", "x, 1, 2", "not a number"},
		{"Linspace", "", "0, x, 2", "not a number"},
		{"Linspace", "", "0, 1, 2.5", "not an integer"},
		{"Linspace", "", "0, 1, 100000000", "would exceed"},
		{"Zeros", "", "2, x", "not an integer"},
		{"Zeros", "", "100000, 100000", "would exceed"},
		{"Zeros", "", "-1", "ndarray: shape mismatch"},
		{"Full", "", "", "Full takes a value"},
		{"Full", "", "x, 2", "not a number"},
		{"Full", "", "1, x", "not an integer"},
		{"Full", "", "1, 100000, 100000", "would exceed"},
		{"Eye", "", "3, 3", "Eye takes n, m, k"},
		{"Eye", "", "3, x, 0", "not an integer"},
		{"Eye", "", "100000, 100000, 0", "would exceed"},
		{"FromData", "", "", "FromData takes"},
		{"FromData", "", "1, 2", "bracketed list"},
		{"FromData", "", "[1, x]", "not a number"},
		{"FromData", "", "[1, 2], x", "not an integer"},
		{"FromData", "", "[1, 2, 3], 2, 2", "ndarray: shape mismatch"},
		{"Reshape", "b", "", "Reshape takes the new shape"},
		{"Reshape", "b", "x", "not an integer"},
		{"Reshape", "b", "4, 4", "ndarray: shape mismatch"},
		{"Transpose", "b", "1", "Transpose takes no arguments"},
		{"Exp", "b", "1", "Exp takes no arguments"},
		{"Slice", "b", "", "Slice takes one index per axis"},
		{"Slice", "b", "x", "want an integer or a start:stop:step"},
		{"Slice", "b", "1:2:3:4", "too many colons"},
		{"Slice", "b", "1:x", "not an integer"},
		{"Slice", "b", "5", "ndarray: index out of range"},
		{"Slice", "b", "::0", "ndarray: index out of range"},
		{"Squeeze", "b", "x", "not an integer"},
		{"Squeeze", "b", "0", "ndarray"},
		{"ExpandDims", "b", "", "ExpandDims takes one axis"},
		{"ExpandDims", "b", "x", "not an integer"},
		{"Power", "b", "", "Power takes one exponent"},
		{"Power", "b", "x", "not a number"},
		{"Clip", "b", "1", "Clip takes lo, hi"},
		{"Clip", "b", "x, 1", "not a number"},
		{"Clip", "b", "1, x", "not a number"},
		{"Sum", "b", "x", "not an integer"},
		{"Sum", "b", "0, 1", "can only be keepdims"},
		{"Sum", "b", "0, keepdims, 1", "at most an axis and keepdims"},
		{"Sum", "b", "5", "ndarray: axis out of range"},
		{"Mean", "b", "x, keepdims", "not an integer"},
		{"CumSum", "b", "x", "not an integer"},
		{"CumSum", "b", "0, 1", "at most one axis"},
		{"CumSum", "b", "7", "ndarray: axis out of range"},
		{"Add", "b", "", "Add takes one operand"},
		{"Add", "b", "1x", "neither a step name nor a number"},
		{"Add", "b", "zz", `no step named "zz"`},
		{"Add", "b", "a", "ndarray: shapes are not broadcastable"},
		{"Maximum", "b", "1", "is not a step name"},
		{"MatMul", "b", "b", "ndarray: incompatible shapes for linear algebra"},
		{"Outer", "b", "", "Outer takes one operand"},
		{"Concatenate", "a", "0", "takes the other steps, then an axis"},
		{"Concatenate", "a", "b, x", "the last argument is the axis"},
		{"Concatenate", "a", "1, 0", "is not a step name"},
		{"Concatenate", "a", "zz, 0", `no step named "zz"`},
		{"Concatenate", "a", "b, 0", "ndarray"},
		{"Stack", "a", "c, 0", "ndarray"},
	}
	for _, c := range cases {
		s := base(t)
		n := len(s.Steps)
		_, err := s.Apply(c.op, c.in, c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s(%s) on %q: error %v, want it to contain %q", c.op, c.args, c.in, err, c.want)
		}
		if len(s.Steps) != n {
			t.Errorf("%s(%s): a failed step was added", c.op, c.args)
		}
	}
}

// The size guards bound the results of broadcasting, products and joins
// before go-ndarray allocates them.
func TestSizeGuards(t *testing.T) {
	s := &Session{}
	mustApply(t, s, "Zeros", "", "4096, 1")    // a
	mustApply(t, s, "Zeros", "", "1, 4096")    // b
	mustApply(t, s, "Zeros", "", "4096")       // c
	mustApply(t, s, "Zeros", "", "2048, 2048") // d
	for _, c := range []struct{ op, in, args string }{
		{"Add", "a", "b"}, {"Maximum", "a", "b"}, {"MatMul", "a", "b"}, {"Dot", "a", "b"},
		{"Outer", "c", "c"}, {"Concatenate", "d", "d, 0"},
	} {
		if _, err := s.Apply(c.op, c.in, c.args); !errors.Is(err, errTooBig) {
			t.Errorf("%s %s %s: %v, want the size guard", c.in, c.op, c.args, err)
		}
	}
	if broadcastSize([]int{2, 3}, []int{4}) != nil {
		t.Error("broadcastSize of incompatible shapes must be nil")
	}
	if got := broadcastSize([]int{3}, []int{2, 1}); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("broadcastSize((3,), (2, 1)) = %v", got)
	}
	if checkSize([]int{0, 1 << 40}) != nil {
		t.Error("an empty shape is never too big")
	}
	if checkSize(nil) != nil {
		t.Error("a 0-d shape is never too big")
	}
	// A zero step leaves the count to go-ndarray.
	if _, err := s.Apply("Arange", "", "0, 1, 0"); err == nil {
		t.Error("Arange with step 0 must fail")
	}
}

// A panic inside go-ndarray is an error, not a dead tab.
func TestTimedRecoversAPanic(t *testing.T) {
	var s Session
	_, _, _, err := s.timed(func() (*ndarray.Array, error) { panic("boom") })
	if err == nil || !strings.Contains(err.Error(), "go-ndarray panicked: boom") {
		t.Fatalf("got %v", err)
	}
}

// Timing repeats a fast call to the budget and reports the mean.
func TestTiming(t *testing.T) {
	now := time.Unix(0, 0)
	s := &Session{Clock: func() time.Time { now = now.Add(100 * time.Microsecond); return now }}
	st := mustApply(t, s, "Arange", "", "0, 3, 1")
	// Each call is bracketed by two clock reads, 100 µs apart.
	if st.Runs != 20 || st.Elapsed != 100*time.Microsecond {
		t.Fatalf("runs %d, mean %v; want 20 runs of 100µs", st.Runs, st.Elapsed)
	}
	s.Clock = func() time.Time { now = now.Add(time.Second); return now }
	st = mustApply(t, s, "Arange", "", "0, 3, 1")
	if st.Runs != 1 || st.Elapsed != time.Second {
		t.Fatalf("a slow call must run once: %d runs, %v", st.Runs, st.Elapsed)
	}
	var real Session
	if real.now().IsZero() {
		t.Fatal("the default clock is time.Now")
	}
}

// Views share memory with what they view and copies do not, as NumPy's
// np.shares_memory says; strides and offsets are go-ndarray's own.
func TestLayout(t *testing.T) {
	s := &Session{}
	mustApply(t, s, "Arange", "", "0, 24, 1")
	b := mustApply(t, s, "Reshape", "a", "4, 6")
	c := mustApply(t, s, "Transpose", "b", "")
	d := mustApply(t, s, "Slice", "b", "1:3, ::2")
	e := mustApply(t, s, "Copy", "d", "")
	if strings.Join(b.Shares, ",") != "a" || strings.Join(c.Shares, ",") != "a,b" || len(e.Shares) != 0 {
		t.Errorf("shares: b %v c %v e %v", b.Shares, c.Shares, e.Shares)
	}
	if !b.Layout.Contiguous() || c.Layout.Contiguous() || d.Layout.Contiguous() || !e.Layout.Contiguous() {
		t.Error("contiguity is wrong")
	}
	if ShapeText(d.Layout.Strides) != "(6, 2)" || d.Layout.Offset != 6 || ShapeText(d.Layout.ByteStrides()) != "(48, 16)" {
		t.Errorf("d strides %v offset %d", d.Layout.Strides, d.Layout.Offset)
	}
	// Axes of length one do not break contiguity whatever their stride.
	if !(Layout{Shape: []int{4, 1}, Strides: []int{1, 1}}).Contiguous() {
		t.Error("(4,1) with strides (1,1) is contiguous")
	}
	if l := LayoutOf(nil); l.Shape != nil || SharesMemory(l, b.Layout) {
		t.Error("a nil array has no layout and shares nothing")
	}
	empty, _ := ndarray.Zeros(0)
	if SharesMemory(LayoutOf(empty), LayoutOf(empty)) {
		t.Error("an empty array has no storage to share")
	}
}

// The reflect read of go-ndarray's unexported fields must keep working: if
// the library renames strides, offset or data this fails here rather than
// leaving the page to show a guess.
func TestLayoutReadsTheLibrary(t *testing.T) {
	a, _ := ndarray.Arange(0, 12, 1)
	m, _ := a.Reshape(3, 4)
	v, _ := m.Slice(ndarray.R(1, 3), ndarray.Rng(3, 0, -2))
	l := LayoutOf(v)
	if ShapeText(l.Strides) != "(4, -2)" || l.Offset != 7 {
		t.Fatalf("strides %v offset %d, want (4, -2) and 7", l.Strides, l.Offset)
	}
	if !SharesMemory(l, LayoutOf(a)) {
		t.Fatal("a slice of a reshape of a must share a's memory")
	}
}

func TestGrid(t *testing.T) {
	s := &Session{}
	mustApply(t, s, "Arange", "", "0, 24, 1")
	g := GridOf(mustApply(t, s, "Reshape", "a", "2, 3, 4").Result)
	if g.Rows != 3 || g.Cols != 4 || g.Values[2][3] != 11 || !strings.Contains(g.Note, "index 0 of every leading axis") {
		t.Errorf("3-D grid: %+v", g)
	}
	g = GridOf(s.Steps[0].Result)
	if g.Rows != 1 || g.Cols != 24 || g.Min != 0 || g.Max != 23 || g.Note != "" {
		t.Errorf("1-D grid: rows %d cols %d min %v max %v note %q", g.Rows, g.Cols, g.Min, g.Max, g.Note)
	}
	g = GridOf(mustApply(t, s, "Sum", "a", "").Result)
	if g.Rows != 1 || g.Cols != 1 || g.Values[0][0] != 276 {
		t.Errorf("0-d grid: %+v", g)
	}
	mustApply(t, s, "Zeros", "", "300, 70")
	g = GridOf(s.Steps[len(s.Steps)-1].Result)
	if len(g.Values) != GridMaxRows || len(g.Values[0]) != GridMaxCols || !strings.HasPrefix(g.Note, "showing the first 200 × 64") {
		t.Errorf("big grid: %d x %d, note %q", len(g.Values), len(g.Values[0]), g.Note)
	}
	mustApply(t, s, "Zeros", "", "2, 300, 70")
	g = GridOf(s.Steps[len(s.Steps)-1].Result)
	if !strings.Contains(g.Note, "leading axis; first 200 × 64") {
		t.Errorf("big 3-D grid note %q", g.Note)
	}
	mustApply(t, s, "Linspace", "", "-1, 1, 3")
	g = GridOf(mustApply(t, s, "Log", "f", "").Result)
	if !math.IsNaN(g.Values[0][0]) || g.Min != 0 || g.Max != 0 {
		t.Errorf("non-finite values are left out of the range: %+v", g)
	}
	if g := GridOf(nil); g.Values != nil || g.Min <= g.Max {
		t.Error("nil grid must be empty with an empty range")
	}
	mustApply(t, s, "Zeros", "", "0, 3")
	if g := GridOf(s.Steps[len(s.Steps)-1].Result); g.Values != nil || g.Note != "the array is empty" {
		t.Errorf("empty grid %+v", g)
	}
	if FormatValue(1.0/3) != "0.333333" {
		t.Error(FormatValue(1.0 / 3))
	}
}

func TestProgramsAndSession(t *testing.T) {
	var s Session
	if !strings.HasPrefix(s.GoProgram(), "//") || !strings.HasPrefix(s.PyProgram(), "#") {
		t.Error("an empty pipeline has placeholder programs")
	}
	if err := s.Load(Presets[0]); err != nil {
		t.Fatal(err)
	}
	g := s.GoProgram()
	for _, want := range []string{"package main", "\ta, err := ndarray.Arange(0, 6, 1)\n\tcheck(err)\n", "\td := b.Transpose()\n\te, err", `fmt.Println("g =", g)`, "func check(err error)"} {
		if !strings.Contains(g, want) {
			t.Errorf("Go program lacks %q:\n%s", want, g)
		}
	}
	p := s.PyProgram()
	for _, want := range []string{"import numpy as np", "d = b.T\n", `print("g =", g)`} {
		if !strings.Contains(p, want) {
			t.Errorf("NumPy program lacks %q:\n%s", want, p)
		}
	}
	if strings.Join(s.Names(), "") != "abcdefg" {
		t.Error(s.Names())
	}
	s.RemoveLast()
	if len(s.Steps) != 6 {
		t.Error("RemoveLast")
	}
	s.Reset()
	s.RemoveLast()
	if len(s.Steps) != 0 {
		t.Error("Reset")
	}
	if err := s.Load(Preset{Calls: []Call{{"Arange", "", "0, 1, 1"}, {"Reshape", "a", "5"}}}); err == nil || len(s.Steps) != 1 {
		t.Errorf("Load stops at the first failing call: %v, %d steps", err, len(s.Steps))
	}
	// Every preset loads, and its proposed next step does what it says.
	for _, p := range Presets {
		if err := s.Load(p); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
		if p.Next != nil {
			if _, err := s.Apply(p.Next.Op, p.Next.Input, p.Next.Args); err == nil {
				t.Errorf("%s: the proposed step should show an error", p.Name)
			}
		}
	}
	if _, ok := OpByName("Nope"); ok {
		t.Error("OpByName")
	}
}

func TestHelpers(t *testing.T) {
	if splitArgs("  ") != nil || len(splitArgs("a, [1, 2], (3, 4)")) != 3 {
		t.Error("splitArgs")
	}
	if v, err := parseFloat(" -inf "); err != nil || !math.IsInf(v, -1) {
		t.Error("parseFloat inf")
	}
	if _, err := parseInts([]string{"(2, x)"}); err == nil {
		t.Error("parseInts must refuse x")
	}
	if pyTuple([]int{3}) != "(3,)" || pyTuple([]int{2, 3}) != "(2, 3)" || ShapeText(nil) != "()" {
		t.Error("tuples")
	}
	for _, n := range []string{"", "A", "a-b"} {
		if isName(n) {
			t.Errorf("%q is not a name", n)
		}
	}
	if !isName("a1") || StepName(0) != "a" || StepName(25) != "z" || StepName(26) != "a1" || StepName(53) != "b2" {
		t.Error("names")
	}
}
