// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause
//
// Browser proof of the go-ndarray playground, driven by browser_test.go.
//
// It loads the BUILT page (Hugo layout + the wasm compiled from this module)
// in headless Chrome, with the colour scheme FORCED, and drives it the way a
// visitor does: real mouse clicks at widget rectangles, real key presses. It
// checks what the app reports (ndarrayDebug) AND what is on the screen
// (canvas pixels), because a flag saying "shown" is not a picture.
//
// Usage: node driver.cjs <url> <chrome> <captureDir>
// Prints one line "RESULT <json>" and exits non-zero on the first failure.
"use strict";
const puppeteer = require("puppeteer-core");
const path = require("path");

const [url, chrome, captureDir] = process.argv.slice(2);
const results = [];
function check(name, ok, detail) {
  results.push({ name, ok: !!ok, detail: detail === undefined ? "" : String(detail) });
  if (!ok) throw new Error("FAILED: " + name + (detail !== undefined ? " — " + detail : ""));
}

// Theme backgrounds the canvas must paint (app.go Theme), as on the landing.
const BG = { light: [0xff, 0xff, 0xff], dark: [0x0b, 0x0e, 0x14] };
const VIRIDIS_ENDS = [[0x44, 0x01, 0x54], [0xfd, 0xe7, 0x25]];

async function run(scheme, dpr) {
  const browser = await puppeteer.launch({
    executablePath: chrome, headless: true,
    args: ["--no-sandbox", "--disable-gpu", "--blink-settings=preferredColorScheme=" + (scheme === "dark" ? 0 : 1)],
  });
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
    await page.setViewport({ width: 1400, height: 900, deviceScaleFactor: dpr });
    await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }]);
    await page.evaluateOnNewDocument(() => { try { localStorage.removeItem("theme"); } catch (e) {} });
    await page.goto(url, { waitUntil: "load" });
    await page.waitForFunction(() => globalThis.ndarrayPlaygroundReady === true &&
      document.querySelector(".stage").classList.contains("ready"), { timeout: 60000 });
    const tag = scheme + "@" + dpr + "x: ";

    const debug = async () => JSON.parse(await page.evaluate(() => ndarrayDebug()));
    const canvasBox = await page.evaluate(() => { const r = document.getElementById("nd-canvas").getBoundingClientRect(); return { x: r.left, y: r.top, w: r.width, h: r.height }; });
    const rects = async () => JSON.parse(await page.evaluate(() => ndarrayRects()));
    const center = (r) => [canvasBox.x + r.x + r.w / 2, canvasBox.y + r.y + r.h / 2];
    const clickAt = async (x, y) => { await page.mouse.click(x, y); await new Promise((r) => setTimeout(r, 60)); };
    const click = async (name) => { const r = (await rects())[name]; const [x, y] = center(r); await clickAt(x, y); };
    // Pixels of the canvas inside a CSS rectangle (relative to the canvas).
    const pixels = async (r) => page.evaluate((r, dpr) => {
      const c = document.getElementById("nd-canvas");
      const ctx = c.getContext("2d");
      const d = ctx.getImageData(Math.round(r.x * dpr), Math.round(r.y * dpr), Math.max(1, Math.round(r.w * dpr)), Math.max(1, Math.round(r.h * dpr))).data;
      return Array.from(d);
    }, r, dpr);
    const near = (p, i, c, tol) => Math.abs(p[i] - c[0]) <= tol && Math.abs(p[i + 1] - c[1]) <= tol && Math.abs(p[i + 2] - c[2]) <= tol;
    const count = (p, c, tol) => { let n = 0; for (let i = 0; i < p.length; i += 4) if (near(p, i, c, tol)) n++; return n; };
    // Pick an option of a dropdown: open it, then click its row in the popover.
    const pick = async (dd, option) => {
      const d = await debug();
      const list = dd === "ops" ? d.ops : d.presets;
      const idx = list.indexOf(option);
      check(tag + "option " + option + " exists in " + dd, idx >= 0, list.join(","));
      await click(dd);
      const pop = (await rects())[dd + "Popover"];
      const visible = Math.min(list.length, 12);
      const rowH = pop.h / visible;
      // The popover keeps its scroll between openings: wheel it back to the
      // top, then down until the option is in its window.
      await page.mouse.move(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + pop.h / 2);
      for (let i = 0; i < list.length; i++) await page.mouse.wheel({ deltaY: -40 });
      await new Promise((r) => setTimeout(r, 40));
      let first = 0;
      while (idx >= first + visible) {
        await page.mouse.move(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + pop.h / 2);
        await page.mouse.wheel({ deltaY: 40 });
        await new Promise((r) => setTimeout(r, 40));
        first++;
      }
      await clickAt(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + (idx - first + 0.5) * rowH);
    };
    const typeArgs = async (text) => {
      await click("args");
      await page.keyboard.press("End");
      for (let i = 0; i < 40; i++) await page.keyboard.press("Backspace");
      await page.keyboard.type(text);
    };

    // 1. The page opened on the README example, in the forced theme.
    let d = await debug();
    check(tag + "opens on the README example (7 steps)", d.steps.length === 7, d.steps.length);
    check(tag + "the last step is selected", d.selected === 6, d.selected);
    check(tag + "the canvas follows the forced theme", d.dark === (scheme === "dark"), d.dark);
    check(tag + "the heatmap table shows the (2, 3) result", d.rows === 2 && d.cols === 4, d.rows + "x" + d.cols);
    const corner = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "canvas background is the theme's", near(corner, 0, BG[scheme], 2), corner.slice(0, 3));
    const tbl = (await rects()).table;
    const tp = await pixels(tbl);
    const heat = count(tp, VIRIDIS_ENDS[0], 12) + count(tp, VIRIDIS_ENDS[1], 12);
    check(tag + "the table is painted as a heatmap (viridis end colours on screen)", heat > 200, heat + " px");
    check(tag + "the memory line names the layout", /strides \(3, 1\) elements/.test(d.memory), d.memory);
    await page.screenshot({ path: path.join(captureDir, "open-" + scheme + "-" + dpr + "x.png") });

    // 2. Click a step of the pipeline: b, the reshaped view.
    const st = (await rects()).steps;
    // Probe down the list until the click lands on row b (index 1): the test
    // does not assume the list's row height.
    for (let y = st.y + 4; y < st.y + st.h; y += 4) {
      await clickAt(canvasBox.x + st.x + 40, canvasBox.y + y);
      d = await debug();
      if (d.selected === 1) break;
    }
    check(tag + "clicking step b selects it", d.selected === 1, d.selected);
    check(tag + "step b is reported as a view of a", /VIEW: shares memory with a/.test(d.memory), d.memory);
    check(tag + "step b is the 2x3 table", d.rows === 2 && d.cols === 4, d.rows + "x" + d.cols);

    // 3. Pick Transpose, input b, Apply: a new view step.
    await pick("ops", "Transpose");
    d = await debug();
    check(tag + "the operation menu selected Transpose", d.op === "Transpose", d.op);
    await click("apply");
    d = await debug();
    check(tag + "Apply added a step", d.steps.length === 8, d.steps.join(" | "));
    check(tag + "the selected step b became the input", d.steps[7].startsWith("h = b.Transpose()"), d.steps[7]);
    check(tag + "the transpose is a non-contiguous view of b's storage (as are a, d and f)", /not contiguous/.test(d.memory) && /VIEW: shares memory with a, b, d, f$/.test(d.memory), d.memory);
    check(tag + "its table is 3x2 (+ the row-label column)", d.rows === 3 && d.cols === 3, d.rows + "x" + d.cols);

    // 4. An error, reported as go-ndarray reports it.
    await pick("ops", "Reshape");
    await typeArgs("4, 4");
    await page.keyboard.press("Enter");
    await new Promise((r) => setTimeout(r, 60));
    d = await debug();
    check(tag + "Enter in the argument field applies", d.error !== "", d.error);
    check(tag + "a bad reshape shows go-ndarray's own error", /^ndarray: shape mismatch/.test(d.error), d.error);
    check(tag + "the failed step was not added", d.steps.length === 8, d.steps.length);
    const ep = await pixels((await rects()).error);
    const red = scheme === "dark" ? [0xf7, 0x8b, 0x8b] : [0xb4, 0x23, 0x18];
    check(tag + "the error is on screen in red", count(ep, red, 40) > 20, count(ep, red, 40) + " px");
    await page.screenshot({ path: path.join(captureDir, "error-" + scheme + "-" + dpr + "x.png") });

    // 5. The code panel: Go program, then the NumPy equivalent.
    check(tag + "the Go program is shown", d.code.startsWith("package main") && d.code.includes("ndarray.Arange(0, 6, 1)"), d.code.slice(0, 40));
    const lang = (await rects()).lang;
    await clickAt(canvasBox.x + lang.x + lang.w * 0.75, canvasBox.y + lang.y + lang.h / 2);
    d = await debug();
    check(tag + "the NumPy tab shows NumPy", d.code.startsWith("import numpy as np") && d.code.includes("b.T"), d.code.slice(0, 40));

    // 6. Copy puts the shown program on the clipboard.
    await browser.defaultBrowserContext().overridePermissions(new URL(url).origin, ["clipboard-read", "clipboard-write", "clipboard-sanitized-write"]);
    await click("copy");
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    check(tag + "Copy copies the program shown", clip === d.code, clip.slice(0, 40));

    // 7. Presets: the shape-mismatch example, then Apply.
    await pick("presets", "A shape mismatch");
    d = await debug();
    check(tag + "the preset loaded 3 steps", d.steps.length === 3, d.steps.join(" | "));
    check(tag + "the preset proposes Add b + c", d.op === "Add" && d.input === "b" && d.args === "c", d.op + " " + d.input + " " + d.args);
    await click("apply");
    d = await debug();
    check(tag + "broadcasting (2,3) with (3,2) fails as go-ndarray says", /^ndarray: shapes are not broadcastable: \[2 3\] vs \[3 2\]/.test(d.error), d.error);

    // 8. go-ndarray's helper goroutines under js/wasm: GOMAXPROCS is 1, so a
    //    large operation (past the parallel threshold) must start none.
    const g0 = d.goroutines;
    await pick("ops", "Arange");
    await typeArgs("0, 1000000, 1");
    await page.keyboard.press("Enter");
    await pick("ops", "Exp");
    await click("apply");
    await pick("ops", "Sum");
    await typeArgs("");
    await click("apply");
    d = await debug();
    check(tag + "the large steps ran", d.steps.length === 6 && d.error === "", d.steps.join(" | ") + " / " + d.error);
    check(tag + "GOMAXPROCS is 1 under js/wasm", d.maxProcs === 1, d.maxProcs);
    check(tag + "no helper goroutine started for 1e6-element operations", d.goroutines === g0, g0 + " -> " + d.goroutines);
    check(tag + "the timing is reported", /per call in this browser/.test(d.timing), d.timing);

    // 9. The page's theme toggle recolours the canvas (System -> Light -> Dark).
    await page.click("#theme-toggle"); // light
    await new Promise((r) => setTimeout(r, 100));
    let px = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "toggle to Light paints the light background", near(px, 0, BG.light, 2) && (await debug()).dark === false, px.slice(0, 3));
    await page.click("#theme-toggle"); // dark
    await new Promise((r) => setTimeout(r, 100));
    px = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "toggle to Dark paints the dark background", near(px, 0, BG.dark, 2) && (await debug()).dark === true, px.slice(0, 3));

    check(tag + "no page or console error", errors.length === 0, errors.join(" | "));
  } finally {
    await browser.close();
  }
}

(async () => {
  try {
    await run("light", 1);
    await run("dark", 1);
    await run("light", 2);
    console.log("RESULT " + JSON.stringify({ ok: true, results }));
  } catch (e) {
    console.log("RESULT " + JSON.stringify({ ok: false, error: String(e.message || e), results }));
    process.exit(1);
  }
})();
