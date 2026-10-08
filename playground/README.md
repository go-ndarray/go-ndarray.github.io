# go-ndarray playground

The page at <https://go-ndarray.github.io/playground/>: go-ndarray itself,
compiled to WebAssembly, inside a [go-widgets](https://github.com/go-widgets)
canvas application. Nothing is computed in JavaScript — the page only loads the
wasm, forwards input and follows the landing's theme toggle.

A visitor builds a pipeline of steps — create an array (`Arange`, `Linspace`,
`Zeros`/`Ones`/`Full`, `Eye`, typed-in `FromData`), then apply views
(`Reshape` with `-1`, `Transpose`, NumPy-style `Slice`), broadcasting
arithmetic, ufuncs, reductions along an axis, `Concatenate`/`Stack`,
`MatMul`/`Dot`/`Outer` — and sees for each step its shape, its strides (in
elements and in bytes), whether it is a view sharing memory with earlier steps,
the time one call took in the browser, its values as a heatmap table, and the
whole pipeline as a Go program that compiles and its NumPy equivalent (both
copyable). Errors are go-ndarray's own (`ndarray: shapes are not broadcastable:
[2 3] vs [3 2]`).

## Layout

| package | role |
|---|---|
| `engine` | the model: the operation catalog, argument parsing, running a step (timed), its Go and NumPy spellings, and the layout of a result read from go-ndarray's `Array` |
| `vm` | the ViewModel: every piece of state as a `go-widgets/mvvm` Observable, ObservableList or Command; no widget |
| `.` (`playground`) | the View: toolkit widgets in box layouts, bound to the ViewModel (`mvvmtk`, plus `view_binding.go`); event routing through the root box |
| `cmd/playground-wasm` | the browser shell: canvas at device pixels, input forwarding, `ndarraySetTheme`, `ndarrayDebug`/`ndarrayRects` for the proofs |

Widgets used: `Label`, `DropDown`, `ListBox`, `Entry`, `Button`,
`ViewSwitcher`, `TextView`, `Table`, `VBox`/`HBox`. The heatmap is
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
