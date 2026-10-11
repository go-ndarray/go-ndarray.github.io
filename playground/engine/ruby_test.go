// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

// The Ruby statement of every operation, on the inputs of base. Each one is
// written against the go-embedded-ruby NDArray contract
// (ruby-ndarray-contract.md): the playground emits nothing else.
func TestRubyOperations(t *testing.T) {
	cases := []struct{ op, in, args, rb string }{
		{"Arange", "", "0, 3, 0.5", "e = NDArray.arange(0, 3, 0.5)"},
		{"Arange", "", "0, 1e6, 1", "e = NDArray.arange(0, 1e+06, 1)"},
		{"Linspace", "", "0, 1, 5", "e = NDArray.linspace(0, 1, 5)"},
		{"Zeros", "", "2, 3", "e = NDArray.zeros(2, 3)"},
		{"Zeros", "", "", "e = NDArray.zeros()"},
		{"Ones", "", "(4,)", "e = NDArray.ones(4)"},
		{"Full", "", "7, 2, 2", "e = NDArray.full(7, 2, 2)"},
		{"Full", "", "-inf", "e = NDArray.full(-Float::INFINITY)"},
		{"Eye", "", "3, 4, 1", "e = NDArray.eye(3, 4, 1)"},
		{"FromData", "", "[1, 2, 3, 4], 2, 2", "e = NDArray.from([1, 2, 3, 4], [2, 2])"},
		{"FromData", "", "[1.5, 2,]", "e = NDArray.from([1.5, 2], [2])"},
		{"Reshape", "a", "3, -1", "e = a.reshape(3, -1)"},
		{"Transpose", "b", "", "e = b.transpose"},
		{"Flatten", "b", "", "e = b.flatten"},
		{"Copy", "b", "", "e = b.dup"},
		{"Slice", "b", "1:, ::-1", "e = b[(1..), (-1..) % -1]"},
		{"Slice", "b", ":, :2", "e = b[true, (...2)]"},
		{"Slice", "b", "0, 0:3:2", "e = b[0, (0...3) % 2]"},
		{"Slice", "b", "0, 1::2", "e = b[0, (1..) % 2]"},
		{"Slice", "b", "0, :0:-1", "e = b[0, (-1...0) % -1]"},
		{"Slice", "b", "0, :3:2", "e = b[0, (0...3) % 2]"},
		{"Slice", "b", "0, 1::-1", "e = b[0, (1..) % -1]"},
		{"Slice", "b", "0, 1:3", "e = b[0, (1...3)]"},
		{"Slice", "b", "::2", "e = b[(0..) % 2]"},
		{"Slice", "b", "-1", "e = b[-1]"},
		{"Slice", "b", "1, 2", "e = NDArray.full(b[1, 2])"},
		{"Squeeze", "b", "", "e = b.squeeze"},
		{"ExpandDims", "b", "0", "e = b.expand_dims(0)"},
		{"Exp", "b", "", "e = b.exp"},
		{"Log", "b", "", "e = b.log"},
		{"Sqrt", "b", "", "e = b.sqrt"},
		{"Sin", "b", "", "e = b.sin"},
		{"Cos", "b", "", "e = b.cos"},
		{"Abs", "b", "", "e = b.abs"},
		{"Square", "b", "", "e = b.square"},
		{"Power", "b", "3", "e = b ** 3"},
		{"Power", "b", "0.5", "e = b ** 0.5"},
		{"Clip", "b", "1, 4", "e = b.clip(1, 4)"},
		{"Sum", "b", "", "e = b.sum"},
		{"Sum", "b", "1", "e = b.sum(1)"},
		{"Mean", "b", "0, keepdims", "e = b.mean(0, keepdims: true)"},
		{"Mean", "b", "", "e = b.mean"},
		{"Max", "b", "1", "e = b.max(1)"},
		{"Max", "b", "", "e = b.max"},
		{"Min", "b", "", "e = b.min"},
		{"Min", "b", "0, true", "e = b.min(0, keepdims: true)"},
		{"ArgMax", "b", "", "e = b.argmax"},
		{"ArgMax", "b", "1", "e = b.argmax(1)"},
		{"CumSum", "b", "", "e = b.cumsum"},
		{"CumSum", "b", "1", "e = b.cumsum(1)"},
		{"Add", "b", "c", "e = b + c"},
		{"Add", "b", "1.5", "e = b + 1.5"},
		{"Sub", "b", "c", "e = b - c"},
		{"Sub", "b", "-1", "e = b - -1"},
		{"Mul", "b", "2", "e = b * 2"},
		{"Mul", "b", "c", "e = b * c"},
		{"Div", "b", "c", "e = b / c"},
		{"Div", "b", "2", "e = b / 2"},
		{"Maximum", "b", "c", "e = NDArray.maximum(b, c)"},
		{"Minimum", "b", "c", "e = NDArray.minimum(b, c)"},
		{"MatMul", "b", "d", "e = b.matmul(d)"},
		{"Dot", "b", "c", "e = b.dot(c)"},
		{"Outer", "c", "a", "e = NDArray.outer(c, a)"},
		{"Concatenate", "a", "a, 0", "e = NDArray.concatenate([a, a], axis: 0)"},
		{"Stack", "b", "b, b, 0", "e = NDArray.stack([b, b, b], axis: 0)"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		s := base(t)
		if c.op == "MatMul" {
			// base's d is the number b.Max(); replace it with b's transpose.
			s.RemoveLast()
			mustApply(t, s, "Transpose", "b", "")
		}
		st, err := s.Apply(c.op, c.in, c.args)
		if err != nil {
			t.Errorf("%s(%s) on %q: %v", c.op, c.args, c.in, err)
			continue
		}
		if st.Rb != c.rb {
			t.Errorf("%s(%s): Ruby %q, want %q", c.op, c.args, st.Rb, c.rb)
		}
		if bad := offContract(st.Rb); bad != "" {
			t.Errorf("%s(%s): %q calls %s, which the NDArray contract does not have", c.op, c.args, st.Rb, bad)
		}
		seen[c.op] = true
	}
	for _, op := range Ops {
		if !seen[op.Name] {
			t.Errorf("operation %s has no Ruby case", op.Name)
		}
	}
}

// contract is every NDArray name ruby-ndarray-contract.md defines: the class
// methods, then the instance methods, then the result accessors.
var contract = map[string]bool{
	"NDArray.arange": true, "NDArray.linspace": true, "NDArray.zeros": true, "NDArray.ones": true,
	"NDArray.full": true, "NDArray.eye": true, "NDArray.from": true, "NDArray.maximum": true,
	"NDArray.minimum": true, "NDArray.outer": true, "NDArray.concatenate": true, "NDArray.stack": true,

	".reshape": true, ".transpose": true, ".T": true, ".flatten": true, ".dup": true, ".copy": true,
	".squeeze": true, ".expand_dims": true, ".exp": true, ".log": true, ".sqrt": true, ".sin": true,
	".cos": true, ".abs": true, ".square": true, ".clip": true, ".sum": true, ".mean": true,
	".max": true, ".min": true, ".argmax": true, ".cumsum": true, ".matmul": true, ".dot": true,

	".shape": true, ".ndim": true, ".size": true, ".to_a": true, ".inspect": true, ".to_s": true,
}

var rbCall = regexp.MustCompile(`(NDArray)?\.[A-Za-z_]+`)

// offContract returns the first method a Ruby expression calls that is not
// in the contract, "" when every one is. A float literal's ".5" is not a call.
func offContract(rb string) string {
	for _, m := range rbCall.FindAllString(rb, -1) {
		if !contract[m] {
			return m
		}
	}
	return ""
}

// The contract check bites: a method outside the contract is reported.
func TestOffContractBites(t *testing.T) {
	if got := offContract("e = b.cumprod(0)"); got != ".cumprod" {
		t.Errorf("an unknown method must be reported, got %q", got)
	}
	if got := offContract("e = NDArray.empty(2)"); got != "NDArray.empty" {
		t.Errorf("an unknown class method must be reported, got %q", got)
	}
	if got := offContract("e = b.sum(1, keepdims: true)"); got != "" {
		t.Errorf("a contract method was refused: %q", got)
	}
}

// The Ruby program: every statement, every step printed, then the selected
// step's shape and values; a number prints as one.
func TestRubyProgram(t *testing.T) {
	s := &Session{}
	if s.RbProgram(0) != "# Create an array to start." {
		t.Errorf("empty program: %q", s.RbProgram(0))
	}
	s = base(t)
	p := s.RbProgram(1)
	for _, want := range []string{
		"a = NDArray.arange(0, 6, 1)\nb = a.reshape(2, 3)\nc = NDArray.from([10, 20, 30], [3])\nd = b.max\n",
		"puts \"a = #{a.inspect}\"\n",
		"puts \"d = #{d.inspect}\"\n",
		"# The selected step, b: its shape, then its values in row-major order.\nputs b.shape.inspect\nputs b.to_a.inspect\n",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("Ruby program lacks %q:\n%s", want, p)
		}
	}
	if q := s.RbProgram(-1); !strings.HasSuffix(q, "# The selected step, d, is a number.\nputs d.inspect\n") {
		t.Errorf("out of range selects the last step, a number:\n%s", q)
	}
	if q := s.RbProgram(9); !strings.Contains(q, "selected step, d,") {
		t.Errorf("out of range selects the last step:\n%s", q)
	}
	for _, line := range strings.Split(p, "\n") {
		if bad := offContract(line); bad != "" && !strings.HasPrefix(line, "#") {
			t.Errorf("program line %q calls %s, outside the contract", line, bad)
		}
	}
}

func TestRbFloat(t *testing.T) {
	for v, want := range map[float64]string{0.5: "0.5", -3: "-3", 1e6: "1e+06", math.Inf(1): "Float::INFINITY", math.Inf(-1): "-Float::INFINITY"} {
		if got := rbFloat(v); got != want {
			t.Errorf("rbFloat(%v) = %q, want %q", v, got, want)
		}
	}
	if got := rbFloat(math.NaN()); got != "Float::NAN" {
		t.Errorf("rbFloat(NaN) = %q", got)
	}
}

// Flat lists a view's values in row-major order, not its storage order.
func TestFlat(t *testing.T) {
	s := base(t)
	st := mustApply(t, s, "Transpose", "b", "")
	got := Flat(st.Result)
	want := []float64{0, 3, 1, 4, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("Flat = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Flat = %v, want %v", got, want)
		}
	}
	if len(Flat(s.Steps[3].Result)) != 1 {
		t.Error("a 0-d result is one value")
	}
}
