// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package vm is the ViewModel of the go-ndarray playground. It holds every
// piece of state the page shows or edits as a go-widgets/mvvm Observable,
// ObservableList or Command, and references no widget: the View binds to
// these, so the whole behaviour is unit-tested here without a canvas.
package vm

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-ndarray/go-ndarray.github.io/playground/engine"
	"github.com/go-widgets/mvvm"
)

// Code languages of the code panel.
const (
	LangGo = iota
	LangNumPy
)

// ViewModel is the whole playground state.
type ViewModel struct {
	session engine.Session

	// OpNames and PresetNames are the fixed menus.
	OpNames     []string
	PresetNames []string

	// --- the form that builds the next step ---------------------------------
	OpIndex     *mvvm.Observable[int]
	Inputs      *mvvm.ObservableList[string] // steps whose result is an array
	InputIndex  *mvvm.Observable[int]
	Args        *mvvm.Observable[string]
	Hint        *mvvm.Observable[string]
	PresetIndex *mvvm.Observable[int]

	// --- the pipeline --------------------------------------------------------
	Steps    *mvvm.ObservableList[string] // one line per step
	Selected *mvvm.Observable[int]

	// --- the selected step ---------------------------------------------------
	Title  *mvvm.Observable[string]
	Shape  *mvvm.Observable[string]
	Memory *mvvm.Observable[string]
	Timing *mvvm.Observable[string]
	GoLine *mvvm.Observable[string]
	PyLine *mvvm.Observable[string]
	Grid   *mvvm.Observable[*engine.Grid] // nil when there is no step
	Note   *mvvm.Observable[string]

	// --- code panel ----------------------------------------------------------
	CodeLang *mvvm.Observable[int]
	Code     *mvvm.Observable[string]

	// --- messages ------------------------------------------------------------
	Error  *mvvm.Observable[string]
	Status *mvvm.Observable[string]

	// --- commands ------------------------------------------------------------
	Apply      *mvvm.Command
	RemoveLast *mvvm.Command
	Reset      *mvvm.Command
	LoadPreset *mvvm.Command
	CopyCode   *mvvm.Command

	// Clipboard receives the text CopyCode copies; the host installs it.
	Clipboard func(string)
	// Runtime describes where the code runs, for the status line; the
	// host fills it in. It defaults to the Go runtime's own view.
	Runtime string
}

// New builds the ViewModel on the first preset.
func New() *ViewModel {
	v := &ViewModel{
		OpIndex:     mvvm.NewObservable(0),
		Inputs:      mvvm.NewObservableList[string](),
		InputIndex:  mvvm.NewObservable(0),
		Args:        mvvm.NewObservable(""),
		Hint:        mvvm.NewObservable(""),
		PresetIndex: mvvm.NewObservable(0),
		Steps:       mvvm.NewObservableList[string](),
		Selected:    mvvm.NewObservable(-1),
		Title:       mvvm.NewObservable(""),
		Shape:       mvvm.NewObservable(""),
		Memory:      mvvm.NewObservable(""),
		Timing:      mvvm.NewObservable(""),
		GoLine:      mvvm.NewObservable(""),
		PyLine:      mvvm.NewObservable(""),
		Grid:        mvvm.NewObservableEq[*engine.Grid](nil, nil),
		Note:        mvvm.NewObservable(""),
		CodeLang:    mvvm.NewObservable(LangGo),
		Code:        mvvm.NewObservable(""),
		Error:       mvvm.NewObservable(""),
		Status:      mvvm.NewObservable(""),
		Runtime:     runtime.GOOS + "/" + runtime.GOARCH + ", GOMAXPROCS " + strconv.Itoa(runtime.GOMAXPROCS(0)),
	}
	for _, op := range engine.Ops {
		v.OpNames = append(v.OpNames, op.Name)
	}
	for _, p := range engine.Presets {
		v.PresetNames = append(v.PresetNames, p.Name)
	}

	v.Apply = mvvm.NewCommand(v.apply, nil)
	v.RemoveLast = mvvm.NewCommand(v.removeLast, func() bool { return len(v.session.Steps) > 0 })
	v.Reset = mvvm.NewCommand(v.reset, func() bool { return len(v.session.Steps) > 0 })
	v.LoadPreset = mvvm.NewCommand(v.loadPreset, nil)
	v.CopyCode = mvvm.NewCommand(v.copyCode, func() bool { return len(v.session.Steps) > 0 })
	mvvm.BindCanExecute(v.RemoveLast, v.Steps)
	mvvm.BindCanExecute(v.Reset, v.Steps)
	mvvm.BindCanExecute(v.CopyCode, v.Steps)

	v.OpIndex.SubscribeChanged(v.opChanged)
	v.Selected.SubscribeChanged(v.showSelected)
	v.Selected.SubscribeChanged(v.followSelection)
	v.CodeLang.SubscribeChanged(v.showCode)
	// Picking a preset loads it: the menu is the action.
	v.PresetIndex.SubscribeChanged(v.LoadPreset.Execute)

	v.loadPreset()
	return v
}

// Session exposes the model, read-only by convention, for the host's tests.
func (v *ViewModel) Session() *engine.Session { return &v.session }

// op is the operation the form has selected.
func (v *ViewModel) op() engine.Op {
	i := v.OpIndex.Get()
	if i < 0 || i >= len(engine.Ops) {
		i = 0
	}
	return engine.Ops[i]
}

// opChanged shows the new operation's hint and offers its default arguments.
func (v *ViewModel) opChanged() {
	op := v.op()
	hint := op.Hint
	switch op.Kind {
	case engine.KindCreate:
		hint = "creates a new array — " + hint
	default:
		hint = "applies to the input — " + hint
	}
	v.Hint.Set(hint)
	v.Args.Set(op.Default)
}

// input is the step name the form has selected as input, "" when none.
func (v *ViewModel) input() string {
	i := v.InputIndex.Get()
	if i < 0 || i >= v.Inputs.Len() {
		return ""
	}
	return v.Inputs.At(i)
}

// apply runs the form as a new step.
func (v *ViewModel) apply() {
	_, err := v.session.Apply(v.op().Name, v.input(), v.Args.Get())
	if err != nil {
		v.Error.Set(err.Error())
		return
	}
	v.Error.Set("")
	v.refresh(len(v.session.Steps) - 1)
}

func (v *ViewModel) removeLast() {
	v.session.RemoveLast()
	v.Error.Set("")
	v.refresh(len(v.session.Steps) - 1)
}

func (v *ViewModel) reset() {
	v.session.Reset()
	v.Error.Set("")
	v.refresh(-1)
	v.selectOp("Arange", "")
}

// loadPreset replaces the pipeline with the selected preset and, when the
// preset proposes a next step, puts it in the form.
func (v *ViewModel) loadPreset() {
	i := v.PresetIndex.Get()
	if i < 0 || i >= len(engine.Presets) {
		i = 0
	}
	p := engine.Presets[i]
	err := v.session.Load(p)
	v.Error.Set("")
	if err != nil {
		v.Error.Set(err.Error())
	}
	v.refresh(len(v.session.Steps) - 1)
	if p.Next != nil {
		v.selectOp(p.Next.Op, p.Next.Input)
		v.Args.Set(p.Next.Args)
	} else {
		v.selectOp("Reshape", "")
	}
}

// selectOp points the form at an operation and, when input is a step name,
// at that input.
func (v *ViewModel) selectOp(name, input string) {
	for i, op := range engine.Ops {
		if op.Name == name {
			if v.OpIndex.Get() == i {
				v.opChanged() // re-offer the default arguments
			}
			v.OpIndex.Set(i)
		}
	}
	for i := 0; i < v.Inputs.Len(); i++ {
		if v.Inputs.At(i) == input {
			v.InputIndex.Set(i)
		}
	}
}

// refresh republishes the pipeline after it changed and selects step sel.
func (v *ViewModel) refresh(sel int) {
	v.Inputs.Clear()
	v.Inputs.Append(v.session.Names()...)
	// The newest array is the natural input of the next step.
	v.InputIndex.Set(v.Inputs.Len() - 1)

	rows := make([]string, len(v.session.Steps))
	for i, st := range v.session.Steps {
		rows[i] = stepRow(st)
	}
	v.Steps.Clear()
	v.Steps.Append(rows...)

	if v.Selected.Get() == sel {
		v.showSelected()
	}
	v.Selected.Set(sel)
	v.showCode()
	v.status()
}

// stepRow is a step's line in the pipeline list.
func stepRow(st engine.Step) string {
	call := st.Op
	if st.Input != "" {
		call = st.Input + "." + st.Op
	}
	return st.Name + " = " + call + "(" + st.Args + ")  " + engine.ShapeText(st.Layout.Shape)
}

// followSelection makes the selected step the input of the next one, when
// its result is an array: inspecting a step is the usual prelude to applying
// something to it.
func (v *ViewModel) followSelection() {
	i := v.Selected.Get()
	if i < 0 || i >= len(v.session.Steps) {
		return
	}
	name := v.session.Steps[i].Name
	for j := 0; j < v.Inputs.Len(); j++ {
		if v.Inputs.At(j) == name {
			v.InputIndex.Set(j)
		}
	}
}

// showSelected publishes the selected step's details, or blanks them.
func (v *ViewModel) showSelected() {
	i := v.Selected.Get()
	if i < 0 || i >= len(v.session.Steps) {
		v.Title.Set("No array yet")
		v.Shape.Set("Pick an operation on the left and press Apply.")
		v.Memory.Set("")
		v.Timing.Set("")
		v.GoLine.Set("")
		v.PyLine.Set("")
		v.Grid.Set(nil)
		v.Note.Set("")
		return
	}
	st := v.session.Steps[i]
	v.Title.Set(title(st))
	v.Shape.Set(shapeLine(st))
	v.Memory.Set(memoryLine(st))
	v.Timing.Set(timingLine(st))
	v.GoLine.Set("Go     " + st.Go)
	v.PyLine.Set("NumPy  " + st.Py)
	g := engine.GridOf(st.Result)
	v.Grid.Set(&g)
	v.Note.Set(g.Note)
}

func title(st engine.Step) string {
	if st.Input == "" {
		return st.Name + " — " + st.Op + "(" + st.Args + ")"
	}
	return st.Name + " — " + st.Op + " of " + st.Input
}

func shapeLine(st engine.Step) string {
	shape := st.Layout.Shape
	s := "shape " + engine.ShapeText(shape) + " · " + strconv.Itoa(st.Result.Size()) + " elements · float64"
	if st.Scalar {
		goType := "float64"
		if st.Op == "ArgMax" {
			goType = "int"
		}
		s = "a number: Go returns a plain " + goType + ", shown here as a 0-d array"
	}
	return s
}

func memoryLine(st engine.Step) string {
	l := st.Layout
	if st.Scalar {
		return "not an array in Go, so it has no strides"
	}
	var b strings.Builder
	b.WriteString("strides " + intsText(l.Strides) + " elements = " + intsText(l.ByteStrides()) + " bytes")
	if l.Offset != 0 {
		b.WriteString(" · offset " + strconv.Itoa(l.Offset))
	}
	if l.Contiguous() {
		b.WriteString(" · C-contiguous")
	} else {
		b.WriteString(" · not contiguous")
	}
	if len(st.Shares) > 0 {
		b.WriteString(" · VIEW: shares memory with " + strings.Join(st.Shares, ", "))
	} else {
		b.WriteString(" · owns its data")
	}
	return b.String()
}

// intsText writes strides as NumPy prints them.
func intsText(v []int) string { return engine.ShapeText(v) }

func timingLine(st engine.Step) string {
	return "took " + durationText(st.Elapsed) + " per call in this browser (mean of " + strconv.Itoa(st.Runs) + " calls)"
}

// durationText writes a duration with a unit a person reads at a glance.
func durationText(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return strconv.FormatInt(d.Nanoseconds(), 10) + " ns"
	case d < time.Millisecond:
		return strconv.FormatFloat(float64(d.Nanoseconds())/1e3, 'f', 1, 64) + " µs"
	default:
		return strconv.FormatFloat(float64(d.Nanoseconds())/1e6, 'f', 2, 64) + " ms"
	}
}

// showCode publishes the whole pipeline in the chosen language.
func (v *ViewModel) showCode() {
	if v.CodeLang.Get() == LangNumPy {
		v.Code.Set(v.session.PyProgram())
		return
	}
	v.Code.Set(v.session.GoProgram())
}

func (v *ViewModel) copyCode() {
	if v.Clipboard != nil {
		v.Clipboard(v.Code.Get())
	}
	lang := "Go"
	if v.CodeLang.Get() == LangNumPy {
		lang = "NumPy"
	}
	v.Status.Set(lang + " program copied to the clipboard")
}

func (v *ViewModel) status() {
	n := len(v.session.Steps)
	s := strconv.Itoa(n) + " steps"
	if n == 1 {
		s = "1 step"
	}
	v.Status.Set(fmt.Sprintf("%s · go-ndarray %s compiled to WebAssembly (%s)", s, ndarrayVersion(), v.Runtime))
}

// ndarrayVersion is the go-ndarray version linked into this binary, read
// from the build info so the status line cannot drift from go.mod.
func ndarrayVersion() string {
	return moduleVersion(debug.ReadBuildInfo)
}

func moduleVersion(read func() (*debug.BuildInfo, bool)) string {
	if bi, ok := read(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/go-ndarray/ndarray" {
				return d.Version
			}
		}
	}
	return "(unknown version)"
}

// SetRuntime replaces the runtime description of the status line.
func (v *ViewModel) SetRuntime(s string) {
	v.Runtime = s
	v.status()
}
