// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build js && wasm

// Command playground-wasm is the browser shell of the go-ndarray playground:
// it sizes a <canvas> to CSS pixels × devicePixelRatio, blits the View's RGBA
// frame into it, and forwards mouse, wheel and keyboard input. Everything else
// — the widgets, the ViewModel, go-ndarray itself — is the tagless, natively
// tested package playground; this file is the thin, coverage-excluded shell,
// shaped after the go-tex playground's.
//
// It publishes, for the host page and for the headless proofs:
//
//	ndarrayPlaygroundReady        true once the first frame is painted
//	ndarraySetTheme(dark)         recolour (the page calls it when its theme changes)
//	ndarrayDebug()                a JSON snapshot of the scene, for tests
//	ndarrayRects()                the widgets' rectangles in CSS pixels, for tests
//
// The page owns the theme: the landing's own System/Light/Dark toggle sits in
// its nav bar, and a change of data-theme reaches the canvas through
// ndarraySetTheme.
package main

import (
	"encoding/json"
	"runtime"
	"strconv"
	"syscall/js"

	playground "github.com/go-ndarray/go-ndarray.github.io/playground"
)

const canvasID = "nd-canvas"

// buildVersion is stamped by the deploy (-ldflags -X main.buildVersion=<sha>).
var buildVersion = "dev"

func main() {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", canvasID)
	if canvas.IsUndefined() || canvas.IsNull() {
		println("playground-wasm: no #" + canvasID + " canvas in the host page")
		return
	}
	ctx := canvas.Call("getContext", "2d")

	dpr := func() float64 {
		if r := js.Global().Get("devicePixelRatio"); r.Type() == js.TypeNumber && r.Float() > 0 {
			return r.Float()
		}
		return 1
	}
	deviceSize := func() (int, int, float64) {
		w := max(canvas.Get("clientWidth").Int(), 320)
		h := max(canvas.Get("clientHeight").Int(), 480)
		d := dpr()
		return int(float64(w)*d + 0.5), int(float64(h)*d + 0.5), d
	}

	dw, dh, d := deviceSize()
	playground.SetupText(d)
	canvas.Set("width", dw)
	canvas.Set("height", dh)
	curDPR := d

	state := playground.NewState(dw, dh, pageIsDark(doc))
	state.VM.SetRuntime(runtime.GOOS + "/" + runtime.GOARCH + ", GOMAXPROCS " +
		strconv.Itoa(runtime.GOMAXPROCS(0)) + ", build " + buildVersion)
	state.VM.Clipboard = func(s string) {
		if clip := js.Global().Get("navigator").Get("clipboard"); clip.Truthy() {
			clip.Call("writeText", s)
		}
	}
	var local []byte
	var imageData, dst js.Value
	alloc := func(w, h int) {
		local = make([]byte, 4*w*h)
		imageData = ctx.Call("createImageData", w, h)
		dst = imageData.Get("data")
	}
	alloc(dw, dh)
	render := func() {
		state.Draw(local)
		js.CopyBytesToJS(dst, local)
		ctx.Call("putImageData", imageData, 0, 0)
	}

	coords := func(e js.Value) (int, int) {
		rect := canvas.Call("getBoundingClientRect")
		sx := rect.Get("width").Float() / canvas.Get("width").Float()
		sy := rect.Get("height").Float() / canvas.Get("height").Float()
		if sx == 0 || sy == 0 {
			return 0, 0
		}
		return int((e.Get("clientX").Float() - rect.Get("left").Float()) / sx),
			int((e.Get("clientY").Float() - rect.Get("top").Float()) / sy)
	}
	on := func(target js.Value, name string, fn func(e js.Value) bool, opts ...any) {
		target.Call("addEventListener", append([]any{name, js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && fn(args[0]) {
				render()
			}
			return nil
		})}, opts...)...)
	}
	on(canvas, "mousedown", func(e js.Value) bool {
		if e.Get("button").Int() != 0 {
			return false
		}
		x, y := coords(e)
		return state.HandleClick(x, y)
	})
	on(js.Global(), "mousemove", func(e js.Value) bool {
		x, y := coords(e)
		return state.HandleMove(x, y)
	})
	on(js.Global(), "mouseup", func(e js.Value) bool {
		x, y := coords(e)
		return state.HandleRelease(x, y)
	})
	on(canvas, "wheel", func(e js.Value) bool {
		e.Call("preventDefault")
		x, y := coords(e)
		return state.HandleScroll(x, y, wheelRows(e.Get("deltaX").Float()), wheelRows(e.Get("deltaY").Float()))
	}, map[string]any{"passive": false})
	on(js.Global(), "keydown", func(e js.Value) bool {
		key := e.Get("key").String()
		mod := e.Get("ctrlKey").Bool() || e.Get("metaKey").Bool()
		if e.Get("isComposing").Bool() || key == "Dead" || isModifier(key) {
			return false
		}
		var changed bool
		if len([]rune(key)) == 1 && !mod {
			changed = state.HandleChar(key)
		} else if !mod {
			if key == "Tab" && e.Get("shiftKey").Bool() {
				key = "Shift+Tab"
			}
			changed = state.HandleKeyDown(key)
		}
		if changed {
			e.Call("preventDefault")
		}
		return changed
	})

	refit := func() {
		nw, nh, nd := deviceSize()
		ow, oh := state.Size()
		if nw == ow && nh == oh && nd == curDPR {
			return
		}
		if nd != curDPR {
			playground.SetupText(nd)
			curDPR = nd
		}
		canvas.Set("width", nw)
		canvas.Set("height", nh)
		alloc(nw, nh)
		state.Resize(nw, nh)
		render()
	}
	js.Global().Call("addEventListener", "resize", js.FuncOf(func(js.Value, []js.Value) any { refit(); return nil }))

	js.Global().Set("ndarraySetTheme", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			state.SetTheme(args[0].Bool())
			render()
		}
		return nil
	}))
	js.Global().Set("ndarrayDebug", js.FuncOf(func(js.Value, []js.Value) any {
		b, _ := json.Marshal(state.Debug())
		return string(b)
	}))
	js.Global().Set("ndarrayRects", js.FuncOf(func(js.Value, []js.Value) any {
		out := map[string]any{}
		for name, r := range state.Rects() {
			out[name] = map[string]any{
				"x": float64(r.X) / curDPR, "y": float64(r.Y) / curDPR,
				"w": float64(r.W) / curDPR, "h": float64(r.H) / curDPR,
			}
		}
		b, _ := json.Marshal(out)
		return string(b)
	}))

	render()
	js.Global().Set("ndarrayPlaygroundReady", true)
	select {}
}

// pageIsDark resolves the page theme: data-theme, else the OS preference.
func pageIsDark(doc js.Value) bool {
	switch doc.Get("documentElement").Call("getAttribute", "data-theme").String() {
	case "dark":
		return true
	case "light":
		return false
	}
	mq := js.Global().Call("matchMedia", "(prefers-color-scheme: dark)")
	return mq.Truthy() && mq.Get("matches").Bool()
}

// wheelRows turns a wheel delta in CSS pixels into toolkit scroll rows.
func wheelRows(d float64) int {
	switch {
	case d > 0:
		return max(1, int(d/40))
	case d < 0:
		return min(-1, int(d/40))
	}
	return 0
}

// isModifier reports a bare modifier key, which types nothing.
func isModifier(k string) bool {
	switch k {
	case "Shift", "Control", "Alt", "Meta", "CapsLock", "AltGraph":
		return true
	}
	return false
}
