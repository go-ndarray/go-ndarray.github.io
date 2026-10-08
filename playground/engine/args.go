// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// splitArgs splits an argument line on the commas that are not inside
// brackets or parentheses, trimming each piece. An empty (or all-blank) line
// is no arguments at all, not one empty argument.
func splitArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// parseFloat reads one number. It accepts what strconv does plus the
// spellings a NumPy user types: inf, -inf, nan (any case).
func parseFloat(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	return v, nil
}

// parseInt reads one integer.
func parseInt(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("%q is not an integer", s)
	}
	return v, nil
}

// parseInts reads every argument as an integer; a shape written as a tuple,
// "(2, 3)", is unwrapped first so both spellings work.
func parseInts(args []string) ([]int, error) {
	if len(args) == 1 && strings.HasPrefix(args[0], "(") && strings.HasSuffix(args[0], ")") {
		args = splitArgs(args[0][1 : len(args[0])-1])
	}
	out := make([]int, 0, len(args))
	for _, a := range args {
		if a == "" {
			continue // a trailing comma, as in the tuple "(3,)"
		}
		v, err := parseInt(a)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// parseFloatList reads a bracketed list of numbers, "[1, 2.5, 3]".
func parseFloatList(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("values must be a bracketed list, like [1, 2, 3]")
	}
	parts := splitArgs(s[1 : len(s)-1])
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		v, err := parseFloat(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// formatFloat writes a number the way Go source and NumPy both accept it.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// joinInts writes "2, 3".
func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = strconv.Itoa(x)
	}
	return strings.Join(s, ", ")
}

// pyTuple writes a NumPy shape tuple: "(3,)" for one axis, "(2, 3)" for two.
func pyTuple(v []int) string {
	if len(v) == 1 {
		return "(" + strconv.Itoa(v[0]) + ",)"
	}
	return "(" + joinInts(v) + ")"
}

// isName reports whether s is a step name: a lower-case letter followed by
// letters or digits.
func isName(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// StepName returns the name of the i-th step (0-based): a, b, …, z, then
// a1, b1, …, so every name is a valid identifier in both Go and Python that
// is not a keyword of either.
func StepName(i int) string {
	letter := string(rune('a' + i%26))
	if i < 26 {
		return letter
	}
	return letter + strconv.Itoa(i/26)
}
