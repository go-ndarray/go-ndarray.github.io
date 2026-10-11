// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package engine

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestRubyAgreesWithGo runs the Ruby program of every operation case and of
// every step of every preset on go-embedded-ruby compiled to WebAssembly (the
// interpreter the page downloads), and requires the shape and the values it
// prints for the selected step to be go-ndarray's. It needs node and a built
// interpreter, PLAYGROUND_RUBY_WASM; it skips without them unless
// PLAYGROUND_REQUIRE_BROWSER=1 (CI), where a skip would prove nothing.
func TestRubyAgreesWithGo(t *testing.T) {
	wasm := os.Getenv("PLAYGROUND_RUBY_WASM")
	node, nodeErr := exec.LookPath("node")
	if wasm == "" || nodeErr != nil {
		if os.Getenv("PLAYGROUND_REQUIRE_BROWSER") == "1" {
			t.Fatal("the Ruby check is required but PLAYGROUND_RUBY_WASM or node is missing")
		}
		t.Skip("set PLAYGROUND_RUBY_WASM to a go-embedded-ruby js/wasm build (and have node) to run Ruby")
	}

	type want struct {
		name   string
		src    string
		scalar bool
		shape  []int
		values []float64
	}
	var cases []want
	add := func(name string, s *Session, sel int) {
		st := s.Steps[sel]
		cases = append(cases, want{name, s.RbProgram(sel), st.Scalar, st.Layout.Shape, Flat(st.Result)})
	}
	for _, c := range rubyCases {
		s := rubyBase(t, c.op)
		if _, err := s.Apply(c.op, c.in, c.args); err != nil {
			t.Fatalf("%s(%s): %v", c.op, c.args, err)
		}
		add(c.op+"("+c.args+")", s, len(s.Steps)-1)
	}
	for _, p := range Presets {
		s := &Session{}
		if err := s.Load(p); err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		for i, st := range s.Steps {
			add(p.Name+" / "+st.Name+" = "+st.Op+"("+st.Args+")", s, i)
		}
	}

	srcs := make([]string, len(cases))
	for i, c := range cases {
		srcs[i] = c.src
	}
	in, _ := json.Marshal(srcs)
	cmd := exec.Command(node, filepath.Join("testdata", "rbeval.cjs"),
		filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"), wasm)
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var got []struct{ Output, Error string }
	var jerr error = strconv.ErrSyntax
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			jerr = json.Unmarshal([]byte(rest), &got)
		}
	}
	if err := jerr; err != nil || len(got) != len(cases) {
		t.Fatalf("node printed %d results (%v): %.300s", len(got), err, out)
	}

	for i, c := range cases {
		g := got[i]
		lines := strings.Split(strings.TrimRight(g.Output, "\n"), "\n")
		if g.Error != "" || len(lines) < 2 {
			t.Errorf("%s: Ruby raised %q\n%s\noutput:\n%s", c.name, g.Error, c.src, g.Output)
			continue
		}
		if c.scalar {
			v, err := strconv.ParseFloat(lines[len(lines)-1], 64)
			if err != nil || !close(v, c.values[0]) {
				t.Errorf("%s: Ruby printed %q, go-ndarray computed %v", c.name, lines[len(lines)-1], c.values[0])
			}
			continue
		}
		shape, err1 := rubyInts(lines[len(lines)-2])
		vals, err2 := rubyFloats(lines[len(lines)-1])
		if err1 != nil || err2 != nil || !sameInts(shape, c.shape) || !sameFloats(vals, c.values) {
			t.Errorf("%s: Ruby printed shape %s values %.200s; go-ndarray: %v %v (%v %v)",
				c.name, lines[len(lines)-2], lines[len(lines)-1], c.shape, c.values, err1, err2)
		}
	}
	t.Logf("%d Ruby programs ran on %s", len(cases), wasm)
}

// rubyFloats reads Ruby's inspect of an Array of Float: "[0.0, 1.5, Infinity]".
func rubyFloats(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, strconv.ErrSyntax
	}
	out := []float64{}
	for _, f := range strings.Split(s[1:len(s)-1], ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// rubyInts reads Ruby's inspect of an Array of Integer: "[2, 3]".
func rubyInts(s string) ([]int, error) {
	fs, err := rubyFloats(s)
	out := make([]int, len(fs))
	for i, f := range fs {
		out[i] = int(f)
	}
	return out, err
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !close(a[i], b[i]) {
			return false
		}
	}
	return true
}

// close compares two results of the same go-ndarray computation: equal, both
// NaN, or within a few ULP should the two modules link different releases.
func close(a, b float64) bool {
	switch {
	case a == b, math.IsNaN(a) && math.IsNaN(b):
		return true
	case math.IsInf(a, 0) || math.IsInf(b, 0):
		return false
	}
	return math.Abs(a-b) <= 1e-12*math.Max(math.Abs(a), math.Abs(b))
}
