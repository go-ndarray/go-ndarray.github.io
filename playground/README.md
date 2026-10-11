# go-ndarray playground

The page at <https://go-ndarray.github.io/playground/>: go-ndarray itself,
compiled to WebAssembly, inside a [go-widgets](https://github.com/go-widgets)
canvas application. Nothing is computed in JavaScript — the page only loads the
wasm, forwards input and follows the landing's theme toggle. The Ruby tab runs
its program on [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), a
Ruby interpreter in Go, compiled to WebAssembly too and fetched only when the
tab is opened.

A visitor builds a pipeline of steps — create an array (`Arange`, `Linspace`,
`Zeros`/`Ones`/`Full`, `Eye`, typed-in `FromData`), then apply views
(`Reshape` with `-1`, `Transpose`, NumPy-style `Slice`), broadcasting
arithmetic, ufuncs, reductions along an axis, `Concatenate`/`Stack`,
`MatMul`/`Dot`/`Outer` — and sees for each step its shape, its strides (in
elements and in bytes), whether it is a view sharing memory with earlier steps,
the time one call took in the browser, its values as a heatmap table, and the
whole pipeline as a Go program that compiles, a Ruby program and its NumPy
equivalent (all three copyable). The Ruby program uses go-embedded-ruby's
`NDArray`, which is go-ndarray bound into the interpreter; **Run** executes it
in the page and prints every step, then the selected step's `shape` and `to_a`. Errors are go-ndarray's own (`ndarray: shapes are not broadcastable:
[2 3] vs [3 2]`).

## Layout

| package | role |
|---|---|
| `engine` | the model: the operation catalog, argument parsing, running a step (timed), its Go, Ruby and NumPy spellings, and the layout of a result read from go-ndarray's `Array` |
| `vm` | the ViewModel: every piece of state as a `go-widgets/mvvm` Observable, ObservableList or Command; no widget |
| `.` (`playground`) | the View: toolkit widgets in box layouts, bound to the ViewModel (`mvvmtk`, plus `view_binding.go`); event routing through the root box |
| `cmd/playground-wasm` | the browser shell: canvas at device pixels, input forwarding, `ndarraySetTheme`, `ndarrayDebug`/`ndarrayRects` for the proofs, and the Ruby hooks (`ndarrayRubyLoad`, `ndarrayRubyEval` around `rbgoEval`) |
| `rubywasm` | a module of its own that only pins go-embedded-ruby (`tool` directive): the deploy builds its `cmd/wasm` from here, and Renovate bumps the pin |

Widgets used: `Label`, `DropDown`, `ListBox`, `Entry`, `Button`,
`ViewSwitcher`, `TextView`, `Table`, `ProgressBar`, `VBox`/`HBox`. The heatmap is
`Table.CellFill` with `toolkit.Viridis`, added to go-widgets/toolkit for this
page.

Strides are not exported by go-ndarray v0.7.1; `engine.LayoutOf` reads the
`strides`, `offset` and `data` fields through `reflect` (read-only, no
`unsafe`), and `TestLayoutReadsTheLibrary` fails if a release renames them.

## go-ndarray's helper goroutines under WebAssembly

go-ndarray keeps up to `GOMAXPROCS-1` helper goroutines alive for large
operations (its README, "Goroutines"). Under `GOOS=js` the Go runtime has a
single P: `GOMAXPROCS` is 1, every operation takes go-ndarray's serial path, and
**no helper goroutine is ever started**. `engine/goroutines_test.go` checks it
both ways: run as WebAssembly under node, the goroutine count is unchanged after
10⁶-element `Exp`, `Sum` and a 1000² `MatMul`; run natively with
`GOMAXPROCS=4`, the same steps start three helpers (the positive control). The
browser proof repeats the WebAssembly check in Chrome. The timings the page
shows are therefore single-threaded, and without SIMD (wasm has no go-asmgen
kernel), so they are slower than native go-ndarray.

## Build and test

```sh
GOWORK=off go test -race ./...                       # 100% statement coverage in CI
GOWORK=off GOOS=js GOARCH=wasm go test \
  -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./engine
GOOS=js GOARCH=wasm go build -o ../static/playground/playground.wasm ./cmd/playground-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ../static/playground/
```

The wasm is about 13 MB (3.4 MB gzipped, as Pages serves it). Neither it nor
`wasm_exec.js` is committed: the deploy builds both from this commit.

The browser proof (`browser_test.go` + `browsertest/driver.cjs`) builds the
site with Hugo and drives it in headless Chrome:

```sh
CHROME=… PLAYGROUND_NODE_PATH=<dir with puppeteer-core> HUGO=hugo \
  GOWORK=off go test -run TestBrowserProof -v .
```

Screenshots go to `$PLAYGROUND_CAPTURE_DIR`, else the user config directory —
never into a repository (the helper refuses a directory inside a git tree).

## Ruby: go-embedded-ruby in the same page

The Ruby tab's program is written against go-embedded-ruby's `NDArray`
(`NDArray.arange(0, 6, 1)`, `a.reshape(2, -1)`, `b[true, (1..)]`,
`b.sum(0)`, `NDArray.concatenate([a, b], axis: 0)`, …); `engine` emits nothing
else, and `engine/ruby_test.go` checks every operation's spelling and that it
calls only the methods of that API.

The interpreter is a second Go program compiled to WebAssembly,
`github.com/go-embedded-ruby/ruby/cmd/wasm`, at the version `rubywasm/go.mod`
pins:

```sh
cd rubywasm
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" \
  -o ../../static/playground/rbgo.wasm github.com/go-embedded-ruby/ruby/cmd/wasm
```

It is far larger than the playground (about 99 MB, 21 MB gzipped), so the page
never loads it up front: the first time the visitor opens the Ruby tab, the
ViewModel asks the host, the page's `ndarrayRubyLoad` fetches it with a
progress line and bar counting the bytes, and starts it in its own Go runtime
(a second `new Go()` with the same `wasm_exec.js`: both modules are built by one
toolchain). **Run** then calls its `rbgoEval(src)` and shows the output.

The browser proof counts the requests for `rbgo.wasm`: none through the Go and
NumPy tabs, one when the Ruby tab opens, still one after two runs. It runs the
README example in Ruby and requires the printed `shape` and `to_a` to equal
go-ndarray's result for the same step. `TestRubyAgreesWithGo` (in `engine`) goes
further, without a browser: it runs the Ruby program of every operation case
and of every step of every preset on the interpreter under node
(`engine/testdata/rbeval.cjs`) and compares each printed shape and value with
go-ndarray's. Both read `PLAYGROUND_RUBY_WASM=<file>`, a prebuilt
interpreter (the proof builds the pinned one without it; CI builds it once
for both).
