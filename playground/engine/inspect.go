// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"reflect"
	"strconv"

	"github.com/go-ndarray/ndarray"
)

// Layout is how an array sits in memory, read from go-ndarray's own Array
// value: its shape, its strides and offset (in elements, the unit go-ndarray
// stores them in), and the span of the storage it points into.
//
// go-ndarray v0.7.1 exports neither the strides nor the storage, so Layout
// reads the unexported fields through reflect — read-only, and without
// unsafe. This is the one place the playground depends on go-ndarray's
// internals; TestLayoutReadsTheLibrary fails if the fields are renamed, so a
// library change cannot silently turn the panel into a guess.
type Layout struct {
	Shape   []int
	Strides []int // in elements
	Offset  int   // in elements, from the start of the storage
	// base and end bound the storage the array points into (its backing
	// slice, start to capacity); two arrays share memory when these overlap.
	base, end uintptr
}

// LayoutOf reads a's layout. A nil array has the zero Layout.
func LayoutOf(a *ndarray.Array) Layout {
	if a == nil {
		return Layout{}
	}
	v := reflect.ValueOf(a).Elem()
	l := Layout{Shape: a.Shape()}
	st := v.FieldByName("strides")
	l.Strides = make([]int, st.Len())
	for i := range l.Strides {
		l.Strides[i] = int(st.Index(i).Int())
	}
	l.Offset = int(v.FieldByName("offset").Int())
	data := v.FieldByName("data")
	if data.Cap() > 0 {
		l.base = data.Pointer()
		l.end = l.base + uintptr(data.Cap())*8
	}
	return l
}

// SharesMemory reports whether two layouts point into overlapping storage,
// i.e. whether writing through one could change the other — NumPy's
// np.shares_memory, answered from the storage spans.
func SharesMemory(x, y Layout) bool {
	if x.base == 0 || y.base == 0 {
		return false
	}
	return x.base < y.end && y.base < x.end
}

// Contiguous reports whether the layout is C-contiguous: its strides are the
// row-major strides of its shape (axes of length 1 may have any stride).
func (l Layout) Contiguous() bool {
	want := 1
	for i := len(l.Shape) - 1; i >= 0; i-- {
		if l.Shape[i] != 1 && l.Strides[i] != want {
			return false
		}
		want *= l.Shape[i]
	}
	return true
}

// ByteStrides is Strides in bytes, the unit NumPy's .strides reports.
func (l Layout) ByteStrides() []int {
	out := make([]int, len(l.Strides))
	for i, s := range l.Strides {
		out[i] = s * 8
	}
	return out
}

// ShapeText writes a shape as NumPy prints it: "(2, 3)", "(3,)", "()".
func ShapeText(shape []int) string {
	if len(shape) == 0 {
		return "()"
	}
	return pyTuple(shape)
}

// Grid is the part of an array the value table shows: a 2-D window of it.
// A 0-d or 1-D array is one row; an array of more than two axes shows the
// last two axes at index 0 of every leading axis, and Note says so.
type Grid struct {
	Values    [][]float64
	RowLabels []string
	ColLabels []string
	Rows      int // rows of the whole 2-D view (Values may hold fewer)
	Cols      int
	Note      string
	Min, Max  float64 // over the finite shown values; Min > Max when none
}

// Grid limits: past these the table would only be scrolled, never read.
const (
	GridMaxRows = 200
	GridMaxCols = 64
)

// GridOf reads the window of a the table shows.
func GridOf(a *ndarray.Array) Grid {
	g := Grid{Min: math.Inf(1), Max: math.Inf(-1)}
	if a == nil {
		return g
	}
	shape := a.Shape()
	if a.Size() == 0 {
		g.Note = "the array is empty"
		return g
	}
	lead := make([]int, 0, len(shape))
	rows, cols := 1, 1
	switch len(shape) {
	case 0:
	case 1:
		cols = shape[0]
	default:
		for range shape[:len(shape)-2] {
			lead = append(lead, 0)
		}
		rows, cols = shape[len(shape)-2], shape[len(shape)-1]
		if len(shape) > 2 {
			g.Note = "showing index 0 of every leading axis"
		}
	}
	g.Rows, g.Cols = rows, cols
	shownR, shownC := min(rows, GridMaxRows), min(cols, GridMaxCols)
	if (rows > shownR || cols > shownC) && g.Note == "" {
		g.Note = "showing the first " + strconv.Itoa(shownR) + " × " + strconv.Itoa(shownC)
	} else if rows > shownR || cols > shownC {
		g.Note += "; first " + strconv.Itoa(shownR) + " × " + strconv.Itoa(shownC)
	}
	for j := 0; j < shownC; j++ {
		g.ColLabels = append(g.ColLabels, strconv.Itoa(j))
	}
	idx := make([]int, len(shape))
	copy(idx, lead)
	for i := 0; i < shownR; i++ {
		row := make([]float64, shownC)
		for j := 0; j < shownC; j++ {
			switch len(shape) {
			case 0:
			case 1:
				idx[0] = j
			default:
				idx[len(shape)-2], idx[len(shape)-1] = i, j
			}
			v := a.At(idx...)
			row[j] = v
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				g.Min = math.Min(g.Min, v)
				g.Max = math.Max(g.Max, v)
			}
		}
		g.Values = append(g.Values, row)
		g.RowLabels = append(g.RowLabels, strconv.Itoa(i))
	}
	return g
}

// FormatValue writes one value for a table cell: six significant digits.
func FormatValue(v float64) string {
	return strconv.FormatFloat(v, 'g', 6, 64)
}
