// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package vm

import (
	"strings"
	"testing"
)

// fakeRuby is a host whose download the test finishes by hand.
type fakeRuby struct {
	loads    int
	progress func(got, total int64)
	done     func(err string)
	ran      []string
	out, err string
}

func (f *fakeRuby) install(v *ViewModel) {
	v.RubyLoad = func(p func(got, total int64), d func(string)) { f.loads++; f.progress, f.done = p, d }
	v.RubyEval = func(src string) (string, string) { f.ran = append(f.ran, src); return f.out, f.err }
}

// The Ruby tab sits between Go and NumPy, shows the Ruby program, and
// downloads the interpreter the first time it is opened, not before.
func TestRubyTabDownloadsOnFirstOpen(t *testing.T) {
	v := New()
	f := &fakeRuby{}
	f.install(v)
	if LangGo != 0 || LangRuby != 1 || LangNumPy != 2 {
		t.Fatal("tab order is Go | Ruby | NumPy")
	}
	if f.loads != 0 || v.RunRuby.CanExecute() || v.RubyStatus.Get() != rubyIdle {
		t.Fatalf("before the Ruby tab: %d loads, Run enabled %v, %q", f.loads, v.RunRuby.CanExecute(), v.RubyStatus.Get())
	}
	v.CodeLang.Set(LangNumPy)
	if f.loads != 0 {
		t.Fatal("the NumPy tab must not download Ruby")
	}
	v.CodeLang.Set(LangRuby)
	if f.loads != 1 || v.RunRuby.CanExecute() {
		t.Fatalf("opening the Ruby tab: %d loads, Run enabled %v", f.loads, v.RunRuby.CanExecute())
	}
	if !strings.HasPrefix(v.Code.Get(), "# go-embedded-ruby") || !strings.Contains(v.Code.Get(), "g = b + c\n") ||
		!strings.HasSuffix(v.Code.Get(), "puts g.shape.inspect\nputs g.to_a.inspect\n") {
		t.Errorf("Ruby program:\n%s", v.Code.Get())
	}
	if v.RbLine.Get() != "Ruby   g = b + c" {
		t.Errorf("Ruby line %q", v.RbLine.Get())
	}
	f.progress(5_200_000, 99_000_000)
	if v.RubyStatus.Get() != "Downloading go-embedded-ruby compiled to WebAssembly: 5.2 of 99.0 MB" || v.RubyProgress.Get() < 0.05 || v.RubyProgress.Get() > 0.06 {
		t.Errorf("progress: %q %v", v.RubyStatus.Get(), v.RubyProgress.Get())
	}
	f.progress(7_000_000, 0) // gzip: no known total
	if v.RubyStatus.Get() != "Downloading go-embedded-ruby compiled to WebAssembly: 7.0 MB" {
		t.Errorf("progress without a total: %q", v.RubyStatus.Get())
	}
	v.CodeLang.Set(LangGo)
	v.CodeLang.Set(LangRuby)
	if f.loads != 1 {
		t.Error("reopening the tab while downloading must not download again")
	}
	f.done("")
	if v.RubyStatus.Get() != rubyReadyTx || v.RubyProgress.Get() != 1 || !v.RunRuby.CanExecute() || len(f.ran) != 0 {
		t.Errorf("loaded: %q %v %v %d", v.RubyStatus.Get(), v.RubyProgress.Get(), v.RunRuby.CanExecute(), len(f.ran))
	}
	// Run runs the program shown, and shows what it printed.
	f.out = "[2, 3]\n[10.0, 21.0, 32.0, 13.0, 24.0, 35.0]\n"
	v.RunRuby.Execute()
	if len(f.ran) != 1 || f.ran[0] != v.Code.Get() || v.RubyOutput.Get() != f.out || !strings.HasPrefix(v.RubyStatus.Get(), "Ran in your browser by go-embedded-ruby in ") {
		t.Errorf("run: %d runs, output %q, status %q", len(f.ran), v.RubyOutput.Get(), v.RubyStatus.Get())
	}
	// The program prints the selected step: selecting another one changes
	// the program and drops the output of the old one.
	v.Selected.Set(1)
	if !strings.HasSuffix(v.Code.Get(), "puts b.shape.inspect\nputs b.to_a.inspect\n") || v.RubyOutput.Get() != "" {
		t.Errorf("selection: output %q, program\n%s", v.RubyOutput.Get(), v.Code.Get())
	}
	// A Ruby error is shown after the output.
	f.out, f.err = "partial", "NoMethodError: undefined method 'square'"
	v.RunRuby.Execute()
	if v.RubyOutput.Get() != "partial\nNoMethodError: undefined method 'square'\n" || !strings.HasPrefix(v.RubyStatus.Get(), "The Ruby program raised an error") {
		t.Errorf("error: %q %q", v.RubyOutput.Get(), v.RubyStatus.Get())
	}
	f.out = ""
	v.RunRuby.Execute()
	if v.RubyOutput.Get() != "NoMethodError: undefined method 'square'\n" {
		t.Errorf("error alone: %q", v.RubyOutput.Get())
	}
	// Copy copies the Ruby program.
	var copied string
	v.Clipboard = func(s string) { copied = s }
	v.CopyCode.Execute()
	if copied != v.Code.Get() || v.Status.Get() != "Ruby program copied to the clipboard" {
		t.Errorf("copy: %q", v.Status.Get())
	}
	// Run is for the Ruby tab only.
	v.CodeLang.Set(LangGo)
	if v.RunRuby.CanExecute() {
		t.Error("Run must be disabled on the Go tab")
	}
}

// Run pressed before the download finished runs once it is done; a failed
// download says so, and Run tries again.
func TestRubyRunWhileLoadingAndFailure(t *testing.T) {
	v := New()
	f := &fakeRuby{}
	f.install(v)
	v.CodeLang.Set(LangRuby)
	f.done("HTTP 404 fetching /playground/rbgo.wasm")
	if !strings.HasSuffix(v.RubyStatus.Get(), "failed to load: HTTP 404 fetching /playground/rbgo.wasm") || !v.RunRuby.CanExecute() {
		t.Fatalf("failure: %q, Run enabled %v", v.RubyStatus.Get(), v.RunRuby.CanExecute())
	}
	v.RunRuby.Execute() // retries the download, then runs
	if f.loads != 2 || len(f.ran) != 0 {
		t.Fatalf("retry: %d loads, %d runs", f.loads, len(f.ran))
	}
	f.done("")
	if len(f.ran) != 1 {
		t.Errorf("Run pressed during the download must run after it: %d runs", len(f.ran))
	}
	f.done("") // a second completion does not run again
	if len(f.ran) != 1 {
		t.Errorf("one press, one run: %d", len(f.ran))
	}
}

// Without a host (natively, in tests) there is no Ruby to run.
func TestRubyWithoutHost(t *testing.T) {
	v := New()
	v.CodeLang.Set(LangRuby)
	if v.RubyStatus.Get() != rubyNoHost || v.RunRuby.CanExecute() {
		t.Errorf("no host: %q, Run enabled %v", v.RubyStatus.Get(), v.RunRuby.CanExecute())
	}
	v.Reset.Execute()
	if v.RbLine.Get() != "" || v.Code.Get() != "# Create an array to start." {
		t.Errorf("reset: %q %q", v.RbLine.Get(), v.Code.Get())
	}
}
