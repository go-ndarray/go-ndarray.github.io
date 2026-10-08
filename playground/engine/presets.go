// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Call is one step of a preset: an operation, its input and its arguments.
type Call struct{ Op, Input, Args string }

// Preset is a ready-made pipeline that shows one idea.
type Preset struct {
	Name  string
	Calls []Call
	// Next, when set, is filled into the form after loading, not applied:
	// the visitor presses Apply and sees what happens.
	Next *Call
}

// Presets are the examples offered in the menu. The first is what the page
// opens on; it is the README's example.
var Presets = []Preset{
	{Name: "The README example", Calls: []Call{
		{"Arange", "", "0, 6, 1"},
		{"Reshape", "a", "2, -1"},
		{"FromData", "", "[10, 20, 30]"},
		{"Transpose", "b", ""},
		{"Sum", "b", "0"},
		{"Slice", "b", ":, 0"},
		{"Add", "b", "c"},
	}},
	{Name: "Views share data", Calls: []Call{
		{"Arange", "", "0, 24, 1"},
		{"Reshape", "a", "4, 6"},
		{"Transpose", "b", ""},
		{"Slice", "b", "1:3, ::2"},
		{"Slice", "b", "::-1, -1"},
		{"Copy", "d", ""},
	}},
	{Name: "Broadcasting", Calls: []Call{
		{"Arange", "", "0, 4, 1"},
		{"Reshape", "a", "4, 1"},
		{"Linspace", "", "0, 1, 5"},
		{"Mul", "b", "c"},
		{"Add", "d", "100"},
	}},
	{Name: "Reductions along an axis", Calls: []Call{
		{"Arange", "", "1, 13, 1"},
		{"Reshape", "a", "3, 4"},
		{"Sum", "b", "0"},
		{"Mean", "b", "1, keepdims"},
		{"Max", "b", ""},
		{"ArgMax", "b", "1"},
		{"CumSum", "b", "1"},
	}},
	{Name: "Ufuncs on a grid", Calls: []Call{
		{"Linspace", "", "0, 6.283185307179586, 16"},
		{"Reshape", "a", "1, 16"},
		{"Reshape", "a", "16, 1"},
		{"Sin", "b", ""},
		{"Cos", "c", ""},
		{"Mul", "d", "e"},
	}},
	{Name: "Concatenate and stack", Calls: []Call{
		{"Ones", "", "2, 3"},
		{"Full", "", "5, 2, 3"},
		{"Concatenate", "a", "b, 0"},
		{"Concatenate", "a", "b, 1"},
		{"Stack", "a", "b, 0"},
	}},
	{Name: "Linear algebra", Calls: []Call{
		{"Arange", "", "0, 6, 1"},
		{"Reshape", "a", "2, 3"},
		{"Transpose", "b", ""},
		{"MatMul", "b", "c"},
		{"Eye", "", "3, 3, 0"},
		{"Dot", "c", "b"},
		{"Outer", "a", "a"},
	}},
	{Name: "A shape mismatch", Calls: []Call{
		{"Arange", "", "0, 6, 1"},
		{"Reshape", "a", "2, 3"},
		{"Ones", "", "3, 2"},
	}, Next: &Call{"Add", "b", "c"}},
}

// Load replaces the session's steps with a preset's. It stops at the first
// call that fails and returns its error, keeping the steps before it.
func (s *Session) Load(p Preset) error {
	s.Reset()
	for _, c := range p.Calls {
		if _, err := s.Apply(c.Op, c.Input, c.Args); err != nil {
			return err
		}
	}
	return nil
}
