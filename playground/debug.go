// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"math"
	"runtime"

	"github.com/go-ndarray/go-ndarray.github.io/playground/engine"
	"github.com/go-widgets/toolkit"
)

// Debug is a snapshot of what the scene shows, for the browser proofs: they
// assert on it after driving the page with real mouse and keyboard events.
type Debug struct {
	Steps    []string `json:"steps"`
	Selected int      `json:"selected"`
	Op       string   `json:"op"`
	Input    string   `json:"input"`
	Args     string   `json:"args"`
	Error    string   `json:"error"`
	Title    string   `json:"title"`
	Shape    string   `json:"shape"`
	Memory   string   `json:"memory"`
	Timing   string   `json:"timing"`
	Rows     int      `json:"rows"`
	Cols     int      `json:"cols"`
	Dark     bool     `json:"dark"`
	Status   string   `json:"status"`
	Code     string   `json:"code"`
	Ops      []string `json:"ops"`
	Presets  []string `json:"presets"`
	// The code panel's language (0 Go, 1 Ruby, 2 NumPy) and the Ruby run: its
	// status line, download fraction and output.
	Lang         int     `json:"lang"`
	RubyStatus   string  `json:"rubyStatus"`
	RubyProgress float64 `json:"rubyProgress"`
	RubyOutput   string  `json:"rubyOutput"`
	CanRunRuby   bool    `json:"canRunRuby"`
	// The selected step's result as go-ndarray computed it: its shape and its
	// values in row-major order, for comparing with what Ruby printed.
	ResultShape  []int     `json:"resultShape"`
	ResultValues []float64 `json:"resultValues"`
	// Goroutines and MaxProcs let a proof watch go-ndarray's helper pool
	// under GOOS=js: GOMAXPROCS is 1 there, so no helper should ever start.
	Goroutines int `json:"goroutines"`
	MaxProcs   int `json:"maxProcs"`
}

// maxDebugValues bounds the values a snapshot carries: a proof compares small
// results, and a million-element step would make every snapshot megabytes.
const maxDebugValues = 4096

// Debug reports the scene's current state.
func (s *State) Debug() Debug {
	v := s.VM
	d := Debug{
		Steps: v.Steps.Slice(), Selected: v.Selected.Get(),
		Op: v.OpNames[v.OpIndex.Get()], Args: v.Args.Get(), Error: v.Error.Get(),
		Title: v.Title.Get(), Shape: v.Shape.Get(), Memory: v.Memory.Get(), Timing: v.Timing.Get(),
		Rows: len(s.table.Rows), Cols: len(s.table.Columns),
		Dark: s.dark, Status: v.Status.Get(), Code: v.Code.Get(),
		Ops: v.OpNames, Presets: v.PresetNames,
		Lang: v.CodeLang.Get(), RubyStatus: v.RubyStatus.Get(), RubyProgress: v.RubyProgress.Get(),
		RubyOutput: v.RubyOutput.Get(), CanRunRuby: v.RunRuby.CanExecute(),
		Goroutines: runtime.NumGoroutine(), MaxProcs: runtime.GOMAXPROCS(0),
	}
	if i := v.InputIndex.Get(); i >= 0 && i < v.Inputs.Len() {
		d.Input = v.Inputs.At(i)
	}
	if i, steps := v.Selected.Get(), v.Session().Steps; i >= 0 && i < len(steps) {
		r := steps[i].Result
		d.ResultShape = r.Shape()
		if r.Size() <= maxDebugValues {
			d.ResultValues = finiteOrNil(engine.Flat(r))
		}
	}
	return d
}

// Rects names the rectangles of the widgets a test clicks, in device pixels.
func (s *State) Rects() map[string]toolkit.Rect {
	return map[string]toolkit.Rect{
		"presets": s.presets.Bounds(), "reset": s.reset.Bounds(),
		"steps": s.steps.Bounds(), "ops": s.ops.Bounds(), "inputs": s.inputs.Bounds(),
		"args": s.args.Bounds(), "apply": s.apply.Bounds(), "undo": s.undo.Bounds(),
		"error": s.errLabel.Bounds(), "table": s.table.Bounds(), "lang": s.lang.Bounds(),
		"copy": s.copyB.Bounds(), "code": s.code.Bounds(), "opsPopover": s.ops.PopoverBounds(),
		"presetsPopover": s.presets.PopoverBounds(),
		"run":            s.runB.Bounds(), "rubyStatus": s.rbStatus.Bounds(), "rubyOut": s.rbOut.Bounds(),
	}
}

// finiteOrNil returns v, or nil when a value is not finite: JSON has no
// spelling for NaN or an infinity, and the snapshot must always encode.
func finiteOrNil(v []float64) []float64 {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
	}
	return v
}
