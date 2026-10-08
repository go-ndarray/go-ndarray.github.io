// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"runtime"
	"testing"
)

// bigSteps runs operations over 10^6 elements, far past go-ndarray's
// parallel threshold (32768): elementwise, a ufunc, reductions, a GEMM.
func bigSteps(t *testing.T) {
	t.Helper()
	s := &Session{}
	mustApply(t, s, "Arange", "", "0, 1000000, 1")
	mustApply(t, s, "Exp", "a", "")
	mustApply(t, s, "Sum", "b", "")
	mustApply(t, s, "Reshape", "a", "1000, 1000")
	mustApply(t, s, "Sum", "d", "1")
	mustApply(t, s, "MatMul", "d", "d")
}

// TestHelperGoroutines says what go-ndarray's persistent helper pool (README,
// "Goroutines": up to GOMAXPROCS-1 helpers, kept alive) does where the
// playground runs. Under GOOS=js the Go runtime has one P — GOMAXPROCS is 1 —
// so every operation takes go-ndarray's serial path and no helper is ever
// started: the goroutine count is the same after 10^6-element operations as
// before. Natively, with GOMAXPROCS 4, the same operations DO start helpers,
// which is the positive control: this instrument can see them.
func TestHelperGoroutines(t *testing.T) {
	if runtime.GOOS == "js" {
		if p := runtime.GOMAXPROCS(0); p != 1 {
			t.Fatalf("GOMAXPROCS under js/wasm is %d, want 1", p)
		}
		before := runtime.NumGoroutine()
		bigSteps(t)
		if after := runtime.NumGoroutine(); after != before {
			t.Fatalf("goroutines %d -> %d: go-ndarray started helpers under js/wasm", before, after)
		}
		return
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	before := runtime.NumGoroutine()
	bigSteps(t)
	after := runtime.NumGoroutine()
	if after < before+1 && after < 4 {
		t.Fatalf("goroutines %d -> %d with GOMAXPROCS 4: the instrument does not see go-ndarray's helpers", before, after)
	}
	t.Logf("native, GOMAXPROCS 4: goroutines %d -> %d", before, after)
}
