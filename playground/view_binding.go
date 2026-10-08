// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

// The bindings mvvmtk does not provide. This is the one file allowed to set a
// widget's data fields (mvvmlint exempts *_binding.go): each function here is a
// one-way or two-way link between a ViewModel Observable and a widget, used
// once from NewState — never a per-frame copy.

package playground

import (
	"math"

	"github.com/go-ndarray/go-ndarray.github.io/playground/engine"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit"
)

// bindEntry links an Entry and a string Observable both ways. A value from the
// ViewModel goes through SetText, which parks the caret at the end — the place
// a visitor expects it after the form offers new default arguments.
func bindEntry(e *toolkit.Entry, obs *mvvm.Observable[string], invalidate func()) (unbind func()) {
	e.SetText(obs.Get())
	a := obs.Subscribe(func(s string) {
		if e.Text().Get() != s {
			e.SetText(s)
		}
		invalidate()
	})
	b := e.Text().Subscribe(func(s string) { obs.Set(s) })
	return func() { a(); b() }
}

// bindGrid shows the selected array as a heatmap table: a row-label column,
// then one column per index of the last axis, each value cell filled on the
// viridis scale between the smallest and the largest value shown.
func bindGrid(t *toolkit.Table, obs *mvvm.Observable[*engine.Grid], invalidate func()) (unbind func()) {
	show := func(g *engine.Grid) {
		t.Selected().Set(-1)
		t.ScrollRow().Set(0)
		t.ScrollX().Set(0)
		if g == nil || len(g.Values) == 0 {
			t.Columns, t.Rows, t.CellFill = nil, nil, nil
			invalidate()
			return
		}
		cols := []toolkit.TableColumn{{Title: "", Width: toolkit.Scaled(rowLabelW), Align: toolkit.AlignRight}}
		for _, c := range g.ColLabels {
			cols = append(cols, toolkit.TableColumn{Title: c, Width: toolkit.Scaled(cellW), Align: toolkit.AlignRight})
		}
		rows := make([][]string, len(g.Values))
		for i, vals := range g.Values {
			row := make([]string, 0, len(vals)+1)
			row = append(row, g.RowLabels[i])
			for _, v := range vals {
				row = append(row, engine.FormatValue(v))
			}
			rows[i] = row
		}
		t.Columns, t.Rows = cols, rows
		t.CellFill = heatFill(g)
		invalidate()
	}
	show(obs.Get())
	return obs.Subscribe(show)
}

// heatFill colours cell (row, col) of a grid table: column 0 holds the row
// labels and is not filled, nor is a value that is not finite.
func heatFill(g *engine.Grid) func(row, col int) (toolkit.RGBA, bool) {
	lo, span := g.Min, g.Max-g.Min
	return func(row, col int) (toolkit.RGBA, bool) {
		if col == 0 || row >= len(g.Values) || col-1 >= len(g.Values[row]) {
			return toolkit.RGBA{}, false
		}
		v := g.Values[row][col-1]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return toolkit.RGBA{}, false
		}
		f := 0.5
		if span > 0 {
			f = (v - lo) / span
		}
		return toolkit.Viridis(f), true
	}
}
