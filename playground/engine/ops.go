// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-ndarray/ndarray"
)

// Kind says what an operation takes.
type Kind int

const (
	// KindCreate builds a new array from its arguments alone.
	KindCreate Kind = iota
	// KindUnary acts on one input array (a view, a ufunc, a reduction).
	KindUnary
	// KindBinary combines the input array with one operand: another step, or
	// for the four arithmetic operations a number.
	KindBinary
	// KindJoin joins the input array with other steps along an axis.
	KindJoin
)

// MaxElements bounds the size of an array the playground will build. A
// browser tab has a few GiB at most, and Go cannot recover from running out
// of memory: the tab would die instead of showing an error. 2^22 float64s are
// 32 MiB — far beyond anything worth looking at in a table.
const MaxElements = 1 << 22

// errTooBig is returned for a result over MaxElements, before it is built.
var errTooBig = fmt.Errorf("playground: result would exceed %d elements; the browser tab would run out of memory", MaxElements)

// Op is one operation of the playground's catalog.
type Op struct {
	Name    string // the go-ndarray name, as listed in the operation menu
	Kind    Kind
	Hint    string // what the argument line holds
	Default string // a sensible argument line to start from
	build   func(c call) (plan, error)
}

// call is what an operation is built from: its input (nil for KindCreate),
// the input's step name, the split argument line, and a lookup for the other
// steps an argument names.
type call struct {
	in     *ndarray.Array
	inName string
	args   []string
	lookup func(name string) (*ndarray.Array, error)
}

// plan is a built operation: the computation, ready to run (and re-run, for
// timing), and the Go and NumPy spellings of it.
type plan struct {
	run    func() (*ndarray.Array, error)
	scalar bool   // the Go call returns a float64 or an int, not an *Array
	goRHS  string // the Go expression
	goErr  bool   // the Go expression also returns an error
	py     string // the NumPy expression
}

// Ops is the catalog, in menu order.
var Ops = []Op{
	{Name: "Arange", Kind: KindCreate, Hint: "start, stop, step", Default: "0, 12, 1", build: buildArange},
	{Name: "Linspace", Kind: KindCreate, Hint: "start, stop, num", Default: "0, 1, 5", build: buildLinspace},
	{Name: "Zeros", Kind: KindCreate, Hint: "shape, e.g. 2, 3", Default: "2, 3", build: buildFilled("Zeros", "zeros")},
	{Name: "Ones", Kind: KindCreate, Hint: "shape, e.g. 2, 3", Default: "2, 3", build: buildFilled("Ones", "ones")},
	{Name: "Full", Kind: KindCreate, Hint: "value, shape…", Default: "7, 2, 3", build: buildFull},
	{Name: "Eye", Kind: KindCreate, Hint: "n, m, k (diagonal)", Default: "4, 4, 0", build: buildEye},
	{Name: "FromData", Kind: KindCreate, Hint: "[values], shape…", Default: "[1, 2, 3, 4, 5, 6], 2, 3", build: buildFromData},

	{Name: "Reshape", Kind: KindUnary, Hint: "new shape, -1 is inferred", Default: "3, -1", build: buildReshape},
	{Name: "Transpose", Kind: KindUnary, Hint: "(no arguments)", build: buildMethod("Transpose", ".T")},
	{Name: "Slice", Kind: KindUnary, Hint: "NumPy indices: 1:3, ::2, 0, -1, :", Default: ":, 1:", build: buildSlice},
	{Name: "Flatten", Kind: KindUnary, Hint: "(no arguments)", build: buildMethod("Flatten", ".flatten()")},
	{Name: "Copy", Kind: KindUnary, Hint: "(no arguments)", build: buildMethod("Copy", ".copy()")},
	{Name: "Squeeze", Kind: KindUnary, Hint: "axes to drop (none = every length-1 axis)", build: buildSqueeze},
	{Name: "ExpandDims", Kind: KindUnary, Hint: "axis", Default: "0", build: buildExpandDims},

	{Name: "Exp", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Exp", "exp")},
	{Name: "Log", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Log", "log")},
	{Name: "Sqrt", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Sqrt", "sqrt")},
	{Name: "Sin", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Sin", "sin")},
	{Name: "Cos", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Cos", "cos")},
	{Name: "Abs", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Abs", "abs")},
	{Name: "Square", Kind: KindUnary, Hint: "(no arguments)", build: buildUfunc("Square", "square")},
	{Name: "Power", Kind: KindUnary, Hint: "exponent", Default: "2", build: buildPower},
	{Name: "Clip", Kind: KindUnary, Hint: "lo, hi", Default: "2, 8", build: buildClip},

	{Name: "Sum", Kind: KindUnary, Hint: "axis[, keepdims] (none = whole array)", Default: "0", build: buildReduce("Sum", "sum")},
	{Name: "Mean", Kind: KindUnary, Hint: "axis[, keepdims] (none = whole array)", Default: "0", build: buildReduce("Mean", "mean")},
	{Name: "Max", Kind: KindUnary, Hint: "axis[, keepdims] (none = whole array)", Default: "1", build: buildReduce("Max", "max")},
	{Name: "Min", Kind: KindUnary, Hint: "axis[, keepdims] (none = whole array)", Default: "1", build: buildReduce("Min", "min")},
	{Name: "ArgMax", Kind: KindUnary, Hint: "axis[, keepdims] (none = flat index)", Default: "1", build: buildReduce("ArgMax", "argmax")},
	{Name: "CumSum", Kind: KindUnary, Hint: "axis (none = flattened)", Default: "0", build: buildCumSum},

	{Name: "Add", Kind: KindBinary, Hint: "a step name, or a number", Default: "10", build: buildArith("Add", "+")},
	{Name: "Sub", Kind: KindBinary, Hint: "a step name, or a number", Default: "1", build: buildArith("Sub", "-")},
	{Name: "Mul", Kind: KindBinary, Hint: "a step name, or a number", Default: "2", build: buildArith("Mul", "*")},
	{Name: "Div", Kind: KindBinary, Hint: "a step name, or a number", Default: "2", build: buildArith("Div", "/")},
	{Name: "Maximum", Kind: KindBinary, Hint: "a step name", build: buildArrayBinary("Maximum", "np.maximum(%s, %s)")},
	{Name: "Minimum", Kind: KindBinary, Hint: "a step name", build: buildArrayBinary("Minimum", "np.minimum(%s, %s)")},
	{Name: "MatMul", Kind: KindBinary, Hint: "a step name", build: buildArrayBinary("MatMul", "%s @ %s")},
	{Name: "Dot", Kind: KindBinary, Hint: "a step name", build: buildArrayBinary("Dot", "np.dot(%s, %s)")},
	{Name: "Outer", Kind: KindBinary, Hint: "a step name", build: buildOuter},

	{Name: "Concatenate", Kind: KindJoin, Hint: "other steps…, axis", build: buildJoin("Concatenate", "concatenate")},
	{Name: "Stack", Kind: KindJoin, Hint: "other steps…, axis", build: buildJoin("Stack", "stack")},
}

// OpByName finds an operation of the catalog.
func OpByName(name string) (Op, bool) {
	for _, op := range Ops {
		if op.Name == name {
			return op, true
		}
	}
	return Op{}, false
}

// --- helpers -----------------------------------------------------------------

// noArgs refuses an argument line on an operation that takes none.
func noArgs(name string, c call) error {
	if len(c.args) != 0 {
		return fmt.Errorf("%s takes no arguments", name)
	}
	return nil
}

// checkSize refuses a shape whose element count exceeds MaxElements (or is
// not representable). Negative dimensions are left to go-ndarray, so the
// visitor sees its own error for them.
func checkSize(shape []int) error {
	n := 1
	for _, d := range shape {
		if d <= 0 {
			return nil
		}
		if n > MaxElements/d {
			return errTooBig
		}
		n *= d
	}
	return nil
}

// scalarArray wraps a reduction's number as a 0-d array, so it can be shown
// like any other result.
func scalarArray(v float64) *ndarray.Array {
	a, _ := ndarray.FromData([]float64{v})
	return a
}

// --- creation ----------------------------------------------------------------

func buildArange(c call) (plan, error) {
	if len(c.args) != 3 {
		return plan{}, errors.New("Arange takes start, stop, step")
	}
	var v [3]float64
	for i, a := range c.args {
		f, err := parseFloat(a)
		if err != nil {
			return plan{}, err
		}
		v[i] = f
	}
	if v[2] != 0 {
		if n := (v[1] - v[0]) / v[2]; n > MaxElements {
			return plan{}, errTooBig
		}
	}
	args := formatFloat(v[0]) + ", " + formatFloat(v[1]) + ", " + formatFloat(v[2])
	return plan{
		run:   func() (*ndarray.Array, error) { return ndarray.Arange(v[0], v[1], v[2]) },
		goRHS: "ndarray.Arange(" + args + ")", goErr: true,
		py: "np.arange(" + args + ")",
	}, nil
}

func buildLinspace(c call) (plan, error) {
	if len(c.args) != 3 {
		return plan{}, errors.New("Linspace takes start, stop, num")
	}
	start, err := parseFloat(c.args[0])
	if err != nil {
		return plan{}, err
	}
	stop, err := parseFloat(c.args[1])
	if err != nil {
		return plan{}, err
	}
	num, err := parseInt(c.args[2])
	if err != nil {
		return plan{}, err
	}
	if err := checkSize([]int{num}); err != nil {
		return plan{}, err
	}
	args := formatFloat(start) + ", " + formatFloat(stop) + ", " + strconv.Itoa(num)
	return plan{
		run:   func() (*ndarray.Array, error) { return ndarray.Linspace(start, stop, num) },
		goRHS: "ndarray.Linspace(" + args + ")", goErr: true,
		py: "np.linspace(" + args + ")",
	}, nil
}

func buildFilled(goName, pyName string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		shape, err := parseInts(c.args)
		if err != nil {
			return plan{}, err
		}
		if err := checkSize(shape); err != nil {
			return plan{}, err
		}
		f := ndarray.Zeros
		if goName == "Ones" {
			f = ndarray.Ones
		}
		return plan{
			run:   func() (*ndarray.Array, error) { return f(shape...) },
			goRHS: "ndarray." + goName + "(" + joinInts(shape) + ")", goErr: true,
			py: "np." + pyName + "(" + pyTuple(shape) + ")",
		}, nil
	}
}

func buildFull(c call) (plan, error) {
	if len(c.args) < 1 {
		return plan{}, errors.New("Full takes a value, then the shape")
	}
	v, err := parseFloat(c.args[0])
	if err != nil {
		return plan{}, err
	}
	shape, err := parseInts(c.args[1:])
	if err != nil {
		return plan{}, err
	}
	if err := checkSize(shape); err != nil {
		return plan{}, err
	}
	goArgs := formatFloat(v)
	if len(shape) > 0 {
		goArgs += ", " + joinInts(shape)
	}
	py := "np.full(" + pyTuple(shape) + ", " + formatFloat(v) + ")"
	if len(shape) == 0 {
		py = "np.full((), " + formatFloat(v) + ")"
	}
	return plan{
		run:   func() (*ndarray.Array, error) { return ndarray.Full(v, shape...) },
		goRHS: "ndarray.Full(" + goArgs + ")", goErr: true,
		py: py,
	}, nil
}

func buildEye(c call) (plan, error) {
	v, err := parseInts(c.args)
	if err != nil {
		return plan{}, err
	}
	if len(v) != 3 {
		return plan{}, errors.New("Eye takes n, m, k")
	}
	if err := checkSize(v[:2]); err != nil {
		return plan{}, err
	}
	return plan{
		run:   func() (*ndarray.Array, error) { return ndarray.Eye(v[0], v[1], v[2]) },
		goRHS: "ndarray.Eye(" + joinInts(v) + ")", goErr: true,
		py: "np.eye(" + joinInts(v) + ")",
	}, nil
}

func buildFromData(c call) (plan, error) {
	if len(c.args) < 1 {
		return plan{}, errors.New("FromData takes [values], then the shape")
	}
	data, err := parseFloatList(c.args[0])
	if err != nil {
		return plan{}, err
	}
	shape, err := parseInts(c.args[1:])
	if err != nil {
		return plan{}, err
	}
	if len(shape) == 0 {
		shape = []int{len(data)}
	}
	vals := make([]string, len(data))
	for i, v := range data {
		vals[i] = formatFloat(v)
	}
	list := strings.Join(vals, ", ")
	py := "np.array([" + list + "])"
	if len(shape) != 1 || shape[0] != len(data) {
		py += ".reshape(" + joinInts(shape) + ")"
	}
	return plan{
		run: func() (*ndarray.Array, error) {
			// FromData keeps the slice it is given; hand each run its own so a
			// timing loop never aliases one array's storage into the next.
			return ndarray.FromData(append([]float64(nil), data...), shape...)
		},
		goRHS: "ndarray.FromData([]float64{" + list + "}, " + joinInts(shape) + ")", goErr: true,
		py: py,
	}, nil
}

// --- views -------------------------------------------------------------------

func buildReshape(c call) (plan, error) {
	shape, err := parseInts(c.args)
	if err != nil {
		return plan{}, err
	}
	if len(shape) == 0 {
		return plan{}, errors.New("Reshape takes the new shape")
	}
	in := c.in
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Reshape(shape...) },
		goRHS: c.inName + ".Reshape(" + joinInts(shape) + ")", goErr: true,
		py: c.inName + ".reshape(" + joinInts(shape) + ")",
	}, nil
}

// buildMethod is an argument-less method returning *Array and no error.
func buildMethod(goName, pySuffix string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		if err := noArgs(goName, c); err != nil {
			return plan{}, err
		}
		in := c.in
		var f func() *ndarray.Array
		switch goName {
		case "Transpose":
			f = in.Transpose
		case "Flatten":
			f = in.Flatten
		default:
			f = in.Copy
		}
		return plan{
			run:   func() (*ndarray.Array, error) { return f(), nil },
			goRHS: c.inName + "." + goName + "()",
			py:    c.inName + pySuffix,
		}, nil
	}
}

// sliceIndex turns one NumPy index token into a go-ndarray Index, its Go
// spelling and its NumPy spelling. n is the length of the axis it indexes;
// it is needed only for "a::s" and ":b:s", which go-ndarray spells with both
// bounds (Rng), so the Go code names the bound NumPy leaves implicit.
func sliceIndex(tok string, n int) (ndarray.Index, string, error) {
	if !strings.Contains(tok, ":") {
		i, err := parseInt(tok)
		if err != nil {
			return ndarray.Index{}, "", fmt.Errorf("index %q: want an integer or a start:stop:step slice", tok)
		}
		return ndarray.A(i), "ndarray.A(" + strconv.Itoa(i) + ")", nil
	}
	parts := strings.Split(tok, ":")
	if len(parts) > 3 {
		return ndarray.Index{}, "", fmt.Errorf("index %q: too many colons", tok)
	}
	var vals [3]int
	var has [3]bool
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := parseInt(p)
		if err != nil {
			return ndarray.Index{}, "", fmt.Errorf("index %q: %v", tok, err)
		}
		vals[i], has[i] = v, true
	}
	start, stop, step := vals[0], vals[1], 1
	if has[2] {
		step = vals[2]
	}
	itoa := strconv.Itoa
	switch {
	case !has[0] && !has[1] && step == 1:
		return ndarray.All(), "ndarray.All()", nil
	case !has[0] && !has[1]:
		return ndarray.Step(step), "ndarray.Step(" + itoa(step) + ")", nil
	case has[0] && has[1] && step == 1:
		return ndarray.R(start, stop), "ndarray.R(" + itoa(start) + ", " + itoa(stop) + ")", nil
	case has[0] && !has[1] && step == 1:
		return ndarray.From(start), "ndarray.From(" + itoa(start) + ")", nil
	case !has[0] && has[1] && step == 1:
		return ndarray.To(stop), "ndarray.To(" + itoa(stop) + ")", nil
	}
	// A stepped range with a bound left out: name it. NumPy's implicit
	// bound runs to the end the step walks towards.
	if !has[0] {
		start = 0
		if step < 0 {
			start = n - 1
		}
	}
	if !has[1] {
		stop = n
		if step < 0 {
			stop = -n - 1
		}
	}
	return ndarray.Rng(start, stop, step),
		"ndarray.Rng(" + itoa(start) + ", " + itoa(stop) + ", " + itoa(step) + ")", nil
}

func buildSlice(c call) (plan, error) {
	if len(c.args) == 0 {
		return plan{}, errors.New("Slice takes one index per axis, NumPy style: 1:3, ::2, 0")
	}
	shape := c.in.Shape()
	idx := make([]ndarray.Index, len(c.args))
	goIdx := make([]string, len(c.args))
	for i, tok := range c.args {
		n := 0
		if i < len(shape) {
			n = shape[i]
		}
		ix, g, err := sliceIndex(tok, n)
		if err != nil {
			return plan{}, err
		}
		idx[i], goIdx[i] = ix, g
	}
	in := c.in
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Slice(idx...) },
		goRHS: c.inName + ".Slice(" + strings.Join(goIdx, ", ") + ")", goErr: true,
		py: c.inName + "[" + strings.Join(c.args, ", ") + "]",
	}, nil
}

func buildSqueeze(c call) (plan, error) {
	axes, err := parseInts(c.args)
	if err != nil {
		return plan{}, err
	}
	in := c.in
	py := "np.squeeze(" + c.inName + ")"
	if len(axes) == 1 {
		py = "np.squeeze(" + c.inName + ", axis=" + strconv.Itoa(axes[0]) + ")"
	} else if len(axes) > 1 {
		py = "np.squeeze(" + c.inName + ", axis=" + pyTuple(axes) + ")"
	}
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Squeeze(axes...) },
		goRHS: c.inName + ".Squeeze(" + joinInts(axes) + ")", goErr: true,
		py: py,
	}, nil
}

func buildExpandDims(c call) (plan, error) {
	v, err := parseInts(c.args)
	if err != nil {
		return plan{}, err
	}
	if len(v) != 1 {
		return plan{}, errors.New("ExpandDims takes one axis")
	}
	in := c.in
	return plan{
		run:   func() (*ndarray.Array, error) { return in.ExpandDims(v[0]) },
		goRHS: c.inName + ".ExpandDims(" + strconv.Itoa(v[0]) + ")", goErr: true,
		py: "np.expand_dims(" + c.inName + ", " + strconv.Itoa(v[0]) + ")",
	}, nil
}

// --- ufuncs ------------------------------------------------------------------

func buildUfunc(goName, pyName string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		if err := noArgs(goName, c); err != nil {
			return plan{}, err
		}
		in := c.in
		f := map[string]func() *ndarray.Array{
			"Exp": in.Exp, "Log": in.Log, "Sqrt": in.Sqrt, "Sin": in.Sin,
			"Cos": in.Cos, "Abs": in.Abs, "Square": in.Square,
		}[goName]
		return plan{
			run:   func() (*ndarray.Array, error) { return f(), nil },
			goRHS: c.inName + "." + goName + "()",
			py:    "np." + pyName + "(" + c.inName + ")",
		}, nil
	}
}

func buildPower(c call) (plan, error) {
	if len(c.args) != 1 {
		return plan{}, errors.New("Power takes one exponent")
	}
	p, err := parseFloat(c.args[0])
	if err != nil {
		return plan{}, err
	}
	in := c.in
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Power(p), nil },
		goRHS: c.inName + ".Power(" + formatFloat(p) + ")",
		py:    c.inName + " ** " + formatFloat(p),
	}, nil
}

func buildClip(c call) (plan, error) {
	if len(c.args) != 2 {
		return plan{}, errors.New("Clip takes lo, hi")
	}
	lo, err := parseFloat(c.args[0])
	if err != nil {
		return plan{}, err
	}
	hi, err := parseFloat(c.args[1])
	if err != nil {
		return plan{}, err
	}
	in := c.in
	args := formatFloat(lo) + ", " + formatFloat(hi)
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Clip(lo, hi) },
		goRHS: c.inName + ".Clip(" + args + ")", goErr: true,
		py: "np.clip(" + c.inName + ", " + args + ")",
	}, nil
}

// --- reductions --------------------------------------------------------------

// axisArgs reads "", "axis" or "axis, keepdims".
func axisArgs(name string, args []string) (axis int, hasAxis, keep bool, err error) {
	switch len(args) {
	case 0:
		return 0, false, false, nil
	case 1, 2:
		axis, err = parseInt(args[0])
		if err != nil {
			return 0, false, false, err
		}
		if len(args) == 2 {
			if args[1] != "keepdims" && args[1] != "true" {
				return 0, false, false, fmt.Errorf("%s: the second argument can only be keepdims", name)
			}
			keep = true
		}
		return axis, true, keep, nil
	}
	return 0, false, false, fmt.Errorf("%s takes at most an axis and keepdims", name)
}

func buildReduce(goName, pyName string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		axis, hasAxis, keep, err := axisArgs(goName, c.args)
		if err != nil {
			return plan{}, err
		}
		in, name := c.in, c.inName
		if !hasAxis {
			p := plan{scalar: true, goErr: true, py: name + "." + pyName + "()"}
			p.goRHS = name + "." + goName + "()"
			switch goName {
			case "Sum":
				p.goErr = false
				p.run = func() (*ndarray.Array, error) { return scalarArray(in.Sum()), nil }
			case "ArgMax":
				p.run = func() (*ndarray.Array, error) {
					i, err := in.ArgMax()
					return scalarArray(float64(i)), err
				}
			default:
				f := map[string]func() (float64, error){"Mean": in.Mean, "Max": in.Max, "Min": in.Min}[goName]
				p.run = func() (*ndarray.Array, error) {
					v, err := f()
					return scalarArray(v), err
				}
			}
			return p, nil
		}
		f := map[string]func(int, bool) (*ndarray.Array, error){
			"Sum": in.SumAxis, "Mean": in.MeanAxis, "Max": in.MaxAxis,
			"Min": in.MinAxis, "ArgMax": in.ArgMaxAxis,
		}[goName]
		py := name + "." + pyName + "(axis=" + strconv.Itoa(axis)
		if keep {
			py += ", keepdims=True"
		}
		return plan{
			run:   func() (*ndarray.Array, error) { return f(axis, keep) },
			goRHS: name + "." + goName + "Axis(" + strconv.Itoa(axis) + ", " + strconv.FormatBool(keep) + ")", goErr: true,
			py: py + ")",
		}, nil
	}
}

func buildCumSum(c call) (plan, error) {
	v, err := parseInts(c.args)
	if err != nil {
		return plan{}, err
	}
	in, name := c.in, c.inName
	switch len(v) {
	case 0:
		return plan{
			run:   func() (*ndarray.Array, error) { return in.CumSumFlat(), nil },
			goRHS: name + ".CumSumFlat()",
			py:    "np.cumsum(" + name + ")",
		}, nil
	case 1:
		return plan{
			run:   func() (*ndarray.Array, error) { return in.CumSum(v[0]) },
			goRHS: name + ".CumSum(" + strconv.Itoa(v[0]) + ")", goErr: true,
			py: "np.cumsum(" + name + ", axis=" + strconv.Itoa(v[0]) + ")",
		}, nil
	}
	return plan{}, errors.New("CumSum takes at most one axis")
}

// --- binary ------------------------------------------------------------------

// broadcastSize is the element count of two shapes broadcast together, or 0
// when they do not broadcast (go-ndarray then reports the error itself).
func broadcastSize(a, b []int) []int {
	n := max(len(a), len(b))
	out := make([]int, n)
	for i := 0; i < n; i++ {
		da, db := 1, 1
		if j := len(a) - n + i; j >= 0 {
			da = a[j]
		}
		if j := len(b) - n + i; j >= 0 {
			db = b[j]
		}
		switch {
		case da == db, db == 1:
			out[i] = da
		case da == 1:
			out[i] = db
		default:
			return nil
		}
	}
	return out
}

// operand resolves the single argument of a binary operation to a step.
func operand(name string, c call) (*ndarray.Array, string, error) {
	if len(c.args) != 1 {
		return nil, "", fmt.Errorf("%s takes one operand", name)
	}
	if !isName(c.args[0]) {
		return nil, "", fmt.Errorf("%s: %q is not a step name", name, c.args[0])
	}
	b, err := c.lookup(c.args[0])
	return b, c.args[0], err
}

func buildArith(goName, pyOp string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		in, name := c.in, c.inName
		if len(c.args) == 1 && !isName(c.args[0]) {
			v, err := parseFloat(c.args[0])
			if err != nil {
				return plan{}, fmt.Errorf("%s: %q is neither a step name nor a number", goName, c.args[0])
			}
			f := map[string]func(float64) *ndarray.Array{
				"Add": in.AddScalar, "Sub": in.SubScalar, "Mul": in.MulScalar, "Div": in.DivScalar,
			}[goName]
			return plan{
				run:   func() (*ndarray.Array, error) { return f(v), nil },
				goRHS: name + "." + goName + "Scalar(" + formatFloat(v) + ")",
				py:    name + " " + pyOp + " " + formatFloat(v),
			}, nil
		}
		b, bName, err := operand(goName, c)
		if err != nil {
			return plan{}, err
		}
		if err := checkSize(broadcastSize(in.Shape(), b.Shape())); err != nil {
			return plan{}, err
		}
		f := map[string]func(*ndarray.Array) (*ndarray.Array, error){
			"Add": in.Add, "Sub": in.Sub, "Mul": in.Mul, "Div": in.Div,
		}[goName]
		return plan{
			run:   func() (*ndarray.Array, error) { return f(b) },
			goRHS: name + "." + goName + "(" + bName + ")", goErr: true,
			py: name + " " + pyOp + " " + bName,
		}, nil
	}
}

func buildArrayBinary(goName, pyFmt string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		b, bName, err := operand(goName, c)
		if err != nil {
			return plan{}, err
		}
		in, name := c.in, c.inName
		as, bs := in.Shape(), b.Shape()
		switch goName {
		case "MatMul", "Dot":
			// The result is at most the two outer extents (times any batch);
			// bound it by the product of every non-contracted extent.
			outer := append(append([]int(nil), as...), bs...)
			if len(as) > 0 && len(bs) > 0 {
				outer = append(append([]int(nil), as[:len(as)-1]...), bs[len(bs)-1])
			}
			err = checkSize(outer)
		default:
			err = checkSize(broadcastSize(as, bs))
		}
		if err != nil {
			return plan{}, err
		}
		f := map[string]func(*ndarray.Array) (*ndarray.Array, error){
			"Maximum": in.Maximum, "Minimum": in.Minimum, "MatMul": in.MatMul, "Dot": in.Dot,
		}[goName]
		return plan{
			run:   func() (*ndarray.Array, error) { return f(b) },
			goRHS: name + "." + goName + "(" + bName + ")", goErr: true,
			py: fmt.Sprintf(pyFmt, name, bName),
		}, nil
	}
}

func buildOuter(c call) (plan, error) {
	b, bName, err := operand("Outer", c)
	if err != nil {
		return plan{}, err
	}
	in, name := c.in, c.inName
	if err := checkSize([]int{in.Size(), b.Size()}); err != nil {
		return plan{}, err
	}
	return plan{
		run:   func() (*ndarray.Array, error) { return in.Outer(b), nil },
		goRHS: name + ".Outer(" + bName + ")",
		py:    "np.outer(" + name + ", " + bName + ")",
	}, nil
}

// --- joining -----------------------------------------------------------------

func buildJoin(goName, pyName string) func(c call) (plan, error) {
	return func(c call) (plan, error) {
		if len(c.args) < 2 {
			return plan{}, fmt.Errorf("%s takes the other steps, then an axis", goName)
		}
		axis, err := parseInt(c.args[len(c.args)-1])
		if err != nil {
			return plan{}, fmt.Errorf("%s: the last argument is the axis: %v", goName, err)
		}
		arrays := []*ndarray.Array{c.in}
		names := []string{c.inName}
		total := c.in.Size()
		for _, n := range c.args[:len(c.args)-1] {
			if !isName(n) {
				return plan{}, fmt.Errorf("%s: %q is not a step name", goName, n)
			}
			a, err := c.lookup(n)
			if err != nil {
				return plan{}, err
			}
			arrays = append(arrays, a)
			names = append(names, n)
			total += a.Size()
		}
		if err := checkSize([]int{total}); err != nil {
			return plan{}, err
		}
		f := ndarray.Concatenate
		if goName == "Stack" {
			f = ndarray.Stack
		}
		list := strings.Join(names, ", ")
		return plan{
			run:   func() (*ndarray.Array, error) { return f(arrays, axis) },
			goRHS: "ndarray." + goName + "([]*ndarray.Array{" + list + "}, " + strconv.Itoa(axis) + ")", goErr: true,
			py: "np." + pyName + "([" + list + "], axis=" + strconv.Itoa(axis) + ")",
		}, nil
	}
}
