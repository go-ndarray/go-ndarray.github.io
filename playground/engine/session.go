// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package engine is the model of the go-ndarray playground: a pipeline of
// steps, each one go-ndarray operation applied to earlier results, with the
// Go and NumPy spellings of every step and what each result looks like in
// memory. Everything here is computed by go-ndarray itself; the package has
// no user interface and no JavaScript.
package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-ndarray/ndarray"
)

// Step is one applied operation and its result.
type Step struct {
	Name  string // the variable the result is bound to: a, b, c, …
	Op    string
	Input string // the step the operation reads; "" for creation
	Args  string

	Result *ndarray.Array // a 0-d array for a reduction to a number
	Scalar bool           // the Go call returns a number, not an *Array
	Layout Layout
	Shares []string // earlier steps whose memory the result points into

	Go    string // the Go statement
	GoErr bool   // the Go call returns an error too
	Py    string // the NumPy statement

	Elapsed time.Duration // mean wall time of one call
	Runs    int           // how many calls the mean is over
}

// Timing budget: a call is repeated until the calls together take
// timingBudget (at most timingMaxRuns calls), because one call of a small
// operation is below the browser clock's resolution.
const (
	timingBudget  = 2 * time.Millisecond
	timingMaxRuns = 1000
)

// Session is a pipeline of steps.
type Session struct {
	Steps []Step
	// Clock is the time source for timing; nil means time.Now.
	Clock func() time.Time
}

func (s *Session) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

// lookup finds a step's array by name, refusing a step whose Go value is a
// number rather than an array.
func (s *Session) lookup(name string) (*ndarray.Array, error) {
	for _, st := range s.Steps {
		if st.Name == name {
			if st.Scalar {
				return nil, fmt.Errorf("%s is a number in Go, not an *ndarray.Array; reduce along an axis to keep an array", name)
			}
			return st.Result, nil
		}
	}
	return nil, fmt.Errorf("there is no step named %q", name)
}

// Names lists the steps whose result is an array, the possible inputs.
func (s *Session) Names() []string {
	var out []string
	for _, st := range s.Steps {
		if !st.Scalar {
			out = append(out, st.Name)
		}
	}
	return out
}

// Apply runs one operation and appends it as a new step. input names the
// step the operation reads (ignored for creation). An error — a bad
// argument line, or go-ndarray's own error — appends nothing.
func (s *Session) Apply(opName, input, args string) (Step, error) {
	op, ok := OpByName(opName)
	if !ok {
		return Step{}, fmt.Errorf("unknown operation %q", opName)
	}
	c := call{args: splitArgs(args), lookup: s.lookup}
	if op.Kind != KindCreate {
		if input == "" {
			return Step{}, fmt.Errorf("%s needs an input: create an array first", op.Name)
		}
		in, err := s.lookup(input)
		if err != nil {
			return Step{}, err
		}
		c.in, c.inName = in, input
	}
	p, err := op.build(c)
	if err != nil {
		return Step{}, err
	}
	res, elapsed, runs, err := s.timed(p.run)
	if err != nil {
		return Step{}, err
	}
	st := Step{
		Name: StepName(len(s.Steps)), Op: op.Name, Args: strings.TrimSpace(args),
		Result: res, Scalar: p.scalar, Layout: LayoutOf(res),
		GoErr: p.goErr, Elapsed: elapsed, Runs: runs,
	}
	if op.Kind != KindCreate {
		st.Input = input
	}
	if p.goErr {
		st.Go = st.Name + ", err := " + p.goRHS
	} else {
		st.Go = st.Name + " := " + p.goRHS
	}
	st.Py = st.Name + " = " + p.py
	for _, prev := range s.Steps {
		if !prev.Scalar && SharesMemory(st.Layout, prev.Layout) {
			st.Shares = append(st.Shares, prev.Name)
		}
	}
	s.Steps = append(s.Steps, st)
	return st, nil
}

// timed runs f once for its result, then again until the calls add up to
// timingBudget, and returns the mean. A panic in go-ndarray (Outer panics on a
// result too big to represent) is reported as an error, not a dead tab.
func (s *Session) timed(f func() (*ndarray.Array, error)) (res *ndarray.Array, mean time.Duration, runs int, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, mean, runs, err = nil, 0, 0, fmt.Errorf("go-ndarray panicked: %v", r)
		}
	}()
	start := s.now()
	res, err = f()
	if err != nil {
		return nil, 0, 0, err
	}
	runs = 1
	total := s.now().Sub(start)
	for total < timingBudget && runs < timingMaxRuns {
		t0 := s.now()
		_, _ = f()
		total += s.now().Sub(t0)
		runs++
	}
	return res, total / time.Duration(runs), runs, nil
}

// RemoveLast drops the last step.
func (s *Session) RemoveLast() {
	if len(s.Steps) > 0 {
		s.Steps = s.Steps[:len(s.Steps)-1]
	}
}

// Reset drops every step.
func (s *Session) Reset() { s.Steps = nil }

// GoProgram is the whole pipeline as a Go program that compiles and prints
// every step, so it can be pasted into a file and run.
func (s *Session) GoProgram() string {
	if len(s.Steps) == 0 {
		return "// Create an array to start."
	}
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/go-ndarray/ndarray\"\n)\n\nfunc main() {\n")
	for _, st := range s.Steps {
		b.WriteString("\t" + st.Go + "\n")
		if st.GoErr {
			b.WriteString("\tcheck(err)\n")
		}
	}
	for _, st := range s.Steps {
		fmt.Fprintf(&b, "\tfmt.Println(%q, %s)\n", st.Name+" =", st.Name)
	}
	b.WriteString("}\n\nfunc check(err error) {\n\tif err != nil {\n\t\tpanic(err)\n\t}\n}\n")
	return b.String()
}

// PyProgram is the same pipeline in NumPy.
func (s *Session) PyProgram() string {
	if len(s.Steps) == 0 {
		return "# Create an array to start."
	}
	var b strings.Builder
	b.WriteString("import numpy as np\n\n")
	for _, st := range s.Steps {
		b.WriteString(st.Py + "\n")
	}
	b.WriteString("\n")
	for _, st := range s.Steps {
		fmt.Fprintf(&b, "print(%q, %s)\n", st.Name+" =", st.Name)
	}
	return b.String()
}
