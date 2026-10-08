// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"runtime"

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
	// Goroutines and MaxProcs let a proof watch go-ndarray's helper pool
	// under GOOS=js: GOMAXPROCS is 1 there, so no helper should ever start.
	Goroutines int `json:"goroutines"`
	MaxProcs   int `json:"maxProcs"`
}

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
		Goroutines: runtime.NumGoroutine(), MaxProcs: runtime.GOMAXPROCS(0),
	}
	if i := v.InputIndex.Get(); i >= 0 && i < v.Inputs.Len() {
		d.Input = v.Inputs.At(i)
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
	}
}
