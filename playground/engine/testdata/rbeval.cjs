// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause
//
// Runs Ruby programs on go-embedded-ruby compiled to WebAssembly, as the
// playground page does (one Go runtime, rbgoEval per program), for
// ruby_run_test.go.
//
// Usage: node rbeval.cjs <wasm_exec.js> <rbgo.wasm> < programs.json
// Reads a JSON array of sources; prints one line "RESULT <json>", an array of
// {output, error}. The Go runtime writes its own console output (a panic
// trace) to stdout too, hence the marker.
"use strict";
const fs = require("fs");
require(process.argv[2]);
// A stopped runtime still has timers that throw; they are reported per program.
process.on("uncaughtException", (e) => {
  if (/already exited/.test(String(e))) return;
  console.error(String(e && e.stack || e));
  process.exit(2);
});
(async () => {
  const srcs = JSON.parse(fs.readFileSync(0, "utf8"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(process.argv[3]), go.importObject);
  go.run(instance);
  for (let waited = 0; !(globalThis.rbgoReady && typeof globalThis.rbgoEval === "function"); waited += 10) {
    if (waited > 60000) throw new Error("go-embedded-ruby did not start");
    await new Promise((r) => setTimeout(r, 10));
  }
  // A Go panic in the interpreter ends its runtime: every later call throws.
  const out = srcs.map((src) => {
    try {
      const r = globalThis.rbgoEval(src);
      if (!r) return { output: "", error: "go-embedded-ruby returned nothing (its Go runtime stopped)" };
      return { output: String(r.output || ""), error: String(r.error || "") };
    } catch (e) {
      return { output: "", error: "go-embedded-ruby stopped: " + String(e) };
    }
  });
  // Exit once the pipe has taken it all: exiting at once truncates it.
  process.stdout.write("\nRESULT " + JSON.stringify(out) + "\n", () => process.exit(0));
})().catch((e) => { console.error(String(e && e.stack || e)); process.exit(2); });
