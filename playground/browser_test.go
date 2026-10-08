// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBrowserProof builds the page exactly as the deploy does — the wasm from
// this module, wasm_exec.js from the same toolchain, then Hugo — serves it,
// and runs browsertest/driver.cjs against it in headless Chrome, light and
// dark with the scheme forced, at 1x and 2x. See the driver for what it
// checks. It skips when Chrome, node, puppeteer-core or Hugo is missing,
// unless PLAYGROUND_REQUIRE_BROWSER=1 (CI), where a skip would be a pass that
// proved nothing.
//
// Environment: CHROME (the browser), PLAYGROUND_NODE_PATH (a node_modules with
// puppeteer-core), HUGO (the hugo command, default "hugo").
func TestBrowserProof(t *testing.T) {
	need := func(what string, ok bool) {
		if ok {
			return
		}
		if os.Getenv("PLAYGROUND_REQUIRE_BROWSER") == "1" {
			t.Fatalf("browser proof required but %s is missing", what)
		}
		t.Skipf("%s is missing; set PLAYGROUND_REQUIRE_BROWSER=1 to make this a failure", what)
	}
	chrome := os.Getenv("CHROME")
	if chrome == "" && runtime.GOOS == "darwin" {
		chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	_, err := os.Stat(chrome)
	need("Chrome (CHROME)", chrome != "" && err == nil)
	node, err := exec.LookPath("node")
	need("node", err == nil)
	nodePath := os.Getenv("PLAYGROUND_NODE_PATH")
	_, err = os.Stat(filepath.Join(nodePath, "puppeteer-core"))
	need("puppeteer-core (PLAYGROUND_NODE_PATH)", nodePath != "" && err == nil)
	hugo := os.Getenv("HUGO")
	if hugo == "" {
		hugo = "hugo"
	}
	hugoArgv := strings.Fields(hugo)
	_, err = exec.LookPath(hugoArgv[0])
	need("Hugo (HUGO)", err == nil)

	site, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	static := filepath.Join(site, "static", "playground")
	if err := os.MkdirAll(static, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, env []string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out.String())
		}
	}
	run(".", []string{"GOOS=js", "GOARCH=wasm"}, "go", "build", "-trimpath", "-o",
		filepath.Join(static, "playground.wasm"), "./cmd/playground-wasm")
	execJS, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(static, "wasm_exec.js"), execJS, 0o644); err != nil {
		t.Fatal(err)
	}
	public := t.TempDir()
	run(site, nil, hugoArgv[0], append(hugoArgv[1:], "--minify", "--destination", public)...)

	srv := httptest.NewServer(http.FileServer(http.Dir(public)))
	defer srv.Close()

	cmd := exec.Command(node, "browsertest/driver.cjs", srv.URL+"/playground/", chrome, captureDir(t))
	cmd.Env = append(os.Environ(), "NODE_PATH="+nodePath)
	out, err := cmd.CombinedOutput()
	var res struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Results []struct {
			Name, Detail string
			OK           bool
		} `json:"results"`
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			_ = json.Unmarshal([]byte(rest), &res)
		}
	}
	for _, r := range res.Results {
		t.Logf("%-5v %s  %s", r.OK, r.Name, r.Detail)
	}
	if err != nil || !res.OK {
		t.Fatalf("browser proof failed: %v %s\n%s", err, res.Error, out)
	}
}
