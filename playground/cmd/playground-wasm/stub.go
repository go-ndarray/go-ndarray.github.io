// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

// Native build stub: the real entry point is wasm-only (see main.go). This
// keeps `go build ./...` and `go test ./...` green on every host.
//
//go:build !js || !wasm

package main

func main() {}
