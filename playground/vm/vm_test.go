// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package vm

import (
	"strings"
	"testing"
	"time"

	"github.com/go-ndarray/go-ndarray.github.io/playground/engine"
)

func opIndex(t *testing.T, name string) int {
	t.Helper()
	for i, n := range engine.Ops {
		if n.Name == name {
			return i
		}
	}
	t.Fatalf("no op %s", name)
	return -1
}

func TestOpensOnTheREADMEExample(t *testing.T) {
	v := New()
	if v.Steps.Len() != 7 || v.Selected.Get() != 6 {
		t.Fatalf("steps %d selected %d", v.Steps.Len(), v.Selected.Get())
	}
	if v.Steps.At(1) != "b = a.Reshape(2, -1)  (2, 3)" || v.Steps.At(2) != "c = FromData([10, 20, 30])  (3,)" {
		t.Errorf("rows: %q / %q", v.Steps.At(1), v.Steps.At(2))
	}
	if v.Title.Get() != "g — Add of b" || !strings.HasPrefix(v.Shape.Get(), "shape (2, 3) · 6 elements") {
		t.Errorf("title %q shape %q", v.Title.Get(), v.Shape.Get())
	}
	if g := v.Grid.Get(); g == nil || g.Rows != 2 || g.Cols != 3 {
		t.Errorf("grid %+v", g)
	}
	if v.OpNames[v.OpIndex.Get()] != "Reshape" || v.Args.Get() != "3, -1" || !strings.HasPrefix(v.Hint.Get(), "applies to the input") {
		t.Errorf("form: %s %q %q", v.OpNames[v.OpIndex.Get()], v.Args.Get(), v.Hint.Get())
	}
	if v.Inputs.Len() != 7 || v.input() != "g" {
		t.Errorf("inputs %v, input %q", v.Inputs.Slice(), v.input())
	}
	if !strings.HasPrefix(v.Code.Get(), "package main") || !strings.Contains(v.Status.Get(), "7 steps") {
		t.Errorf("code/status: %q", v.Status.Get())
	}
	if !v.RemoveLast.CanExecute() || !v.CopyCode.CanExecute() || !v.Reset.CanExecute() {
		t.Error("commands must be enabled with steps")
	}
}

func TestApplyAndErrors(t *testing.T) {
	v := New()
	v.Selected.Set(1) // b becomes the input
	if v.input() != "b" {
		t.Fatalf("selecting b must make it the input, got %q", v.input())
	}
	v.OpIndex.Set(opIndex(t, "Transpose"))
	if v.Args.Get() != "" {
		t.Errorf("Transpose offers no arguments, got %q", v.Args.Get())
	}
	v.Apply.Execute()
	if v.Steps.Len() != 8 || v.Selected.Get() != 7 || v.Title.Get() != "h — Transpose of b" {
		t.Fatalf("after Apply: %d steps, selected %d, %q", v.Steps.Len(), v.Selected.Get(), v.Title.Get())
	}
	if !strings.Contains(v.Memory.Get(), "not contiguous") || !strings.Contains(v.Memory.Get(), "VIEW: shares memory with a, b, d, f") {
		t.Errorf("memory %q", v.Memory.Get())
	}
	if !strings.HasPrefix(v.GoLine.Get(), "Go     h := b.Transpose()") || v.PyLine.Get() != "NumPy  h = b.T" {
		t.Errorf("lines %q %q", v.GoLine.Get(), v.PyLine.Get())
	}
	v.OpIndex.Set(opIndex(t, "Reshape"))
	v.Args.Set("4, 4")
	v.Apply.Execute()
	if !strings.HasPrefix(v.Error.Get(), "ndarray: shape mismatch") || v.Steps.Len() != 8 {
		t.Errorf("error %q, %d steps", v.Error.Get(), v.Steps.Len())
	}
	v.Args.Set("-1")
	v.Apply.Execute()
	if v.Error.Get() != "" || v.Steps.Len() != 9 {
		t.Errorf("a good apply clears the error: %q", v.Error.Get())
	}
	v.RemoveLast.Execute()
	if v.Steps.Len() != 8 || v.Selected.Get() != 7 {
		t.Errorf("RemoveLast: %d steps, selected %d", v.Steps.Len(), v.Selected.Get())
	}
}

func TestResetAndEmpty(t *testing.T) {
	v := New()
	v.Reset.Execute()
	if v.Steps.Len() != 0 || v.Selected.Get() != -1 || v.Grid.Get() != nil || v.Title.Get() != "No array yet" {
		t.Fatalf("after reset: %d steps, %d, %q", v.Steps.Len(), v.Selected.Get(), v.Title.Get())
	}
	if v.RemoveLast.CanExecute() || v.CopyCode.CanExecute() || v.Reset.CanExecute() {
		t.Error("commands must grey out with no steps")
	}
	if v.OpNames[v.OpIndex.Get()] != "Arange" || v.Args.Get() != "0, 12, 1" || !strings.HasPrefix(v.Hint.Get(), "creates a new array") {
		t.Errorf("reset form: %q %q", v.Args.Get(), v.Hint.Get())
	}
	v.OpIndex.Set(opIndex(t, "Exp"))
	v.Apply.Execute()
	if !strings.Contains(v.Error.Get(), "needs an input") {
		t.Errorf("error %q", v.Error.Get())
	}
	v.OpIndex.Set(opIndex(t, "Arange"))
	v.Apply.Execute()
	if v.Steps.Len() != 1 || v.Status.Get()[:6] != "1 step" {
		t.Errorf("status %q", v.Status.Get())
	}
	// Reset while already on Arange re-offers its arguments.
	v.Args.Set("zzz")
	v.Reset.Execute()
	if v.Args.Get() != "0, 12, 1" {
		t.Errorf("args %q", v.Args.Get())
	}
}

func TestPresetsAndCode(t *testing.T) {
	v := New()
	var copied string
	v.Clipboard = func(s string) { copied = s }
	for i, p := range engine.Presets {
		v.PresetIndex.Set(i)
		if v.Steps.Len() != len(p.Calls) || v.Error.Get() != "" {
			t.Errorf("%s: %d steps, error %q", p.Name, v.Steps.Len(), v.Error.Get())
		}
	}
	// The last preset proposes a step that fails.
	if v.OpNames[v.OpIndex.Get()] != "Add" || v.input() != "b" || v.Args.Get() != "c" {
		t.Errorf("proposed: %s %s %s", v.OpNames[v.OpIndex.Get()], v.input(), v.Args.Get())
	}
	v.Apply.Execute()
	if !strings.HasPrefix(v.Error.Get(), "ndarray: shapes are not broadcastable") {
		t.Errorf("error %q", v.Error.Get())
	}
	v.CopyCode.Execute()
	if copied != v.Code.Get() || !strings.HasPrefix(copied, "package main") || v.Status.Get() != "Go program copied to the clipboard" {
		t.Errorf("copy: %q", v.Status.Get())
	}
	v.CodeLang.Set(LangNumPy)
	v.CopyCode.Execute()
	if !strings.HasPrefix(copied, "import numpy as np") || v.Status.Get() != "NumPy program copied to the clipboard" {
		t.Errorf("NumPy copy: %q", v.Status.Get())
	}
	v.Clipboard = nil
	v.CopyCode.Execute() // no host clipboard: no panic
	v.SetRuntime("js/wasm")
	if !strings.HasSuffix(v.Status.Get(), "(js/wasm)") {
		t.Error(v.Status.Get())
	}
	// Out-of-range menu indices fall back to the first entry.
	v.PresetIndex.Set(99)
	if v.Steps.Len() != len(engine.Presets[0].Calls) {
		t.Error("preset 99 must load the first")
	}
	v.OpIndex.Set(-5)
	if v.op().Name != "Arange" {
		t.Error("op -5 must be the first")
	}
	v.InputIndex.Set(99)
	if v.input() != "" {
		t.Error("input 99 is none")
	}
	v.Selected.Set(42) // nothing selected: blank details, input untouched
	if v.Title.Get() != "No array yet" {
		t.Error(v.Title.Get())
	}
}

func TestDetailLines(t *testing.T) {
	v := New()
	v.PresetIndex.Set(3) // reductions: e = b.Max() is a number, f = ArgMax along 1
	v.Selected.Set(4)
	if !strings.Contains(v.Shape.Get(), "plain float64") || !strings.Contains(v.Memory.Get(), "no strides") {
		t.Errorf("scalar: %q / %q", v.Shape.Get(), v.Memory.Get())
	}
	before := v.input()
	v.Selected.Set(4) // a number is not an input
	if v.input() != before {
		t.Error("a scalar step must not become the input")
	}
	v.session.Steps[4].Op = "ArgMax"
	v.showSelected()
	if !strings.Contains(v.Shape.Get(), "plain int") {
		t.Error(v.Shape.Get())
	}
	v.PresetIndex.Set(1) // views: d = b[1:3, ::2] has an offset
	v.Selected.Set(3)
	if !strings.Contains(v.Memory.Get(), "offset 6") {
		t.Error(v.Memory.Get())
	}
	for d, want := range map[time.Duration]string{
		500 * time.Nanosecond: "500 ns", 1500 * time.Nanosecond: "1.5 µs", 2500 * time.Microsecond: "2.50 ms",
	} {
		if got := durationText(d); got != want {
			t.Errorf("durationText(%v) = %q, want %q", d, got, want)
		}
	}
	if v.Session() == nil {
		t.Error("Session")
	}
}

func TestSelectOpUnknownInput(t *testing.T) {
	v := New()
	v.selectOp("Transpose", "nope")
	if v.OpNames[v.OpIndex.Get()] != "Transpose" {
		t.Error("selectOp")
	}
}

// A preset whose call fails keeps the steps before it and shows the error.
func TestBrokenPreset(t *testing.T) {
	saved := engine.Presets
	defer func() { engine.Presets = saved }()
	engine.Presets = append(append([]engine.Preset(nil), saved...), engine.Preset{Name: "broken",
		Calls: []engine.Call{{Op: "Arange", Args: "0, 2, 1"}, {Op: "Reshape", Input: "a", Args: "3"}}})
	v := New()
	v.PresetIndex.Set(len(engine.Presets) - 1)
	if v.Steps.Len() != 1 || !strings.HasPrefix(v.Error.Get(), "ndarray: shape mismatch") {
		t.Errorf("%d steps, error %q", v.Steps.Len(), v.Error.Get())
	}
}
