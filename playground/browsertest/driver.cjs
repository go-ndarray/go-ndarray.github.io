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

// The cases: the forced scheme, the device pixel ratio and the viewport. A
// phone is set with page.setViewport (isMobile, hasTouch), because headless
// Chrome's --window-size cannot go that narrow.
const DESKTOP = { width: 1400, height: 900 };
const PHONE = { width: 390, height: 844, isMobile: true, hasTouch: true, name: "phone" };

async function run(scheme, dpr, vp = DESKTOP, opts = {}) {
  const browser = await puppeteer.launch({
    executablePath: chrome, headless: true,
    args: ["--no-sandbox", "--disable-gpu", "--blink-settings=preferredColorScheme=" + (scheme === "dark" ? 0 : 1)],
  });
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
    // Every request for the Ruby module: it must be fetched only once the Ruby
    // tab is opened, and only once.
    const rubyRequests = [];
    page.on("request", (r) => { if (/\/rbgo\.wasm(\?|$)/.test(r.url())) rubyRequests.push(r.url()); });
    await page.setViewport({ width: vp.width, height: vp.height, deviceScaleFactor: dpr, isMobile: !!vp.isMobile, hasTouch: !!vp.hasTouch });
    await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }]);
    await page.evaluateOnNewDocument(() => { try { localStorage.removeItem("theme"); } catch (e) {} });
    await page.goto(url, { waitUntil: "load" });
    await page.waitForFunction(() => globalThis.ndarrayPlaygroundReady === true &&
      document.querySelector(".stage").classList.contains("ready"), { timeout: 60000 });
    const tag = scheme + "@" + dpr + "x" + (vp.name ? " " + vp.name : "") + ": ";
    const shot = (what) => path.join(captureDir, what + "-" + scheme + "-" + dpr + "x" + (vp.name ? "-" + vp.name : "") + ".png");

    const debug = async () => { const raw = await page.evaluate(() => ndarrayDebug()); try { return JSON.parse(raw); } catch (e) { throw new Error("ndarrayDebug returned no JSON (" + e.message + "): " + String(raw).slice(0, 200)); } };
    const boxOf = async () => page.evaluate(() => { const r = document.getElementById("nd-canvas").getBoundingClientRect(); return { x: r.left, y: r.top, w: r.width, h: r.height }; });
    let canvasBox = await boxOf();
    // On a phone the canvas is taller than the window and the stage scrolls
    // (State.MinHeight): scroll a widget into view before clicking it, as a
    // visitor would, then re-read where the canvas now is.
    const reveal = async (r) => {
      await page.evaluate((y, h) => {
        const s = document.querySelector(".stage");
        if (y < s.scrollTop || y + h > s.scrollTop + s.clientHeight) s.scrollTop = Math.max(0, y + h / 2 - s.clientHeight / 2);
      }, r.y, r.h);
      canvasBox = await boxOf();
    };
    const rects = async () => JSON.parse(await page.evaluate(() => ndarrayRects()));
    const center = (r) => [canvasBox.x + r.x + r.w / 2, canvasBox.y + r.y + r.h / 2];
    const clickAt = async (x, y) => { await page.mouse.click(x, y); await new Promise((r) => setTimeout(r, 60)); };
    const click = async (name) => { const r = (await rects())[name]; await reveal(r); const [x, y] = center(r); await clickAt(x, y); };
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
      await reveal(pop);
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

    // 0. A capture of the opened page first, so a failure below still leaves
    //    the picture of what was on screen.
    await page.screenshot({ path: shot("loaded") });
    // 0b. The viewport is the one asked for, and the canvas fits it: no
    //     horizontal overflow, and something other than the background is
    //     painted across it.
    const fit = await page.evaluate(() => {
      const c = document.getElementById("nd-canvas");
      const r = c.getBoundingClientRect();
      return { iw: innerWidth, dpr: devicePixelRatio, left: r.left, right: r.right, cw: c.width,
        scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth };
    });
    // Measured against the width ASKED for: on a mobile viewport Chrome widens
    // the layout viewport to fit overflowing content, so innerWidth alone
    // would follow the overflow it is meant to catch.
    check(tag + "the canvas fits the viewport width", fit.left >= 0 && fit.right <= vp.width + 0.5, fit.left + ".." + fit.right + " of " + vp.width);
    check(tag + "the page does not scroll horizontally", fit.scroll <= vp.width && fit.client <= vp.width, "scrollWidth " + fit.scroll + ", clientWidth " + fit.client + " of " + vp.width);
    check(tag + "the viewport is " + vp.width + " CSS px at " + dpr + "x", fit.iw === vp.width && fit.dpr === dpr, fit.iw + " @ " + fit.dpr);
    check(tag + "the canvas backing store matches the device pixels", Math.abs(fit.cw - Math.round((fit.right - fit.left) * dpr)) <= dpr, fit.cw);
    const all = await pixels({ x: 0, y: 0, w: canvasBox.w, h: Math.min(canvasBox.h, 400) });
    const painted = all.length / 4 - count(all, BG[scheme], 6);
    check(tag + "the app rendered something (non-background pixels)", painted > 1000, painted + " px");

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
    await page.screenshot({ path: shot("open") });

    // 2. Click a step of the pipeline: b, the reshaped view.
    const st = (await rects()).steps;
    await reveal(st);
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
    await page.screenshot({ path: shot("error") });

    // 5. The code panel: Go program, then the NumPy equivalent.
    check(tag + "the Go program is shown", d.code.startsWith("package main") && d.code.includes("ndarray.Arange(0, 6, 1)"), d.code.slice(0, 40));
    const lang = (await rects()).lang;
    await reveal(lang);
    await clickAt(canvasBox.x + lang.x + lang.w * 5 / 6, canvasBox.y + lang.y + lang.h / 2); // Go | Ruby | NumPy
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

    // 10. Ruby, run in this browser by go-embedded-ruby (a second Go wasm
    //     runtime in the page). Everything above used the Go and NumPy tabs:
    //     the Ruby module must not have been fetched yet.
    check(tag + "the Ruby module was not downloaded before the Ruby tab was opened", rubyRequests.length === 0, rubyRequests.join(", "));
    if (opts.ruby) {
      await pick("presets", "The README example");
      d = await debug();
      check(tag + "the README example is back, g selected", d.steps.length === 7 && d.selected === 6, d.steps.length + " / " + d.selected);
      const goShape = d.resultShape, goValues = d.resultValues;
      const lg = (await rects()).lang;
      await reveal(lg);
      await clickAt(canvasBox.x + lg.x + lg.w / 2, canvasBox.y + lg.y + lg.h / 2);
      d = await debug();
      check(tag + "the middle tab is Ruby, showing the Ruby program", d.lang === 1 && d.code.startsWith("# go-embedded-ruby") && d.code.includes("g = b + c\n"), d.lang + " " + d.code.slice(0, 40));
      check(tag + "the Ruby program ends by printing the selected step", d.code.endsWith("puts g.shape.inspect\nputs g.to_a.inspect\n"), d.code.slice(-60));
      check(tag + "opening the Ruby tab starts the download", rubyRequests.length === 1, rubyRequests.length);
      await page.waitForFunction(() => JSON.parse(ndarrayDebug()).canRunRuby, { timeout: 300000, polling: 250 });
      d = await debug();
      check(tag + "the download finished, with its progress shown", d.rubyProgress === 1 && /press Run/.test(d.rubyStatus), d.rubyProgress + " " + d.rubyStatus);
      await page.screenshot({ path: shot("ruby-ready") });
      await click("run");
      await page.waitForFunction(() => JSON.parse(ndarrayDebug()).rubyOutput !== "", { timeout: 120000, polling: 250 });
      d = await debug();
      check(tag + "the Ruby program ran without an error", /^Ran in your browser/.test(d.rubyStatus), d.rubyStatus + " / " + d.rubyOutput.slice(-200));
      const lines = d.rubyOutput.trim().split("\n");
      let rbShape, rbValues;
      try { rbShape = JSON.parse(lines[lines.length - 2]); rbValues = JSON.parse(lines[lines.length - 1]); } catch (e) { rbShape = rbValues = String(e); }
      check(tag + "Ruby's g.shape equals go-ndarray's", JSON.stringify(rbShape) === JSON.stringify(goShape), JSON.stringify(rbShape) + " vs " + JSON.stringify(goShape));
      check(tag + "Ruby's g.to_a equals go-ndarray's values", Array.isArray(rbValues) && rbValues.length === goValues.length && rbValues.every((x, i) => x === goValues[i]), JSON.stringify(rbValues) + " vs " + JSON.stringify(goValues));
      const ro = (await rects()).rubyOut;
      check(tag + "the output panel fits inside the canvas (refitted for the Ruby tab)", ro.h > 40 && ro.y + ro.h <= canvasBox.h - 20, JSON.stringify(ro) + " in " + canvasBox.h);
      const op = await pixels((await rects()).rubyOut);
      check(tag + "the output is on screen (non-background pixels)", op.length / 4 - count(op, BG[scheme], 6) > 200, op.length / 4 - count(op, BG[scheme], 6));
      await page.screenshot({ path: shot("ruby-ran") });
      await click("copy");
      const rbClip = await page.evaluate(() => navigator.clipboard.readText());
      check(tag + "Copy copies the Ruby program", rbClip === d.code && rbClip.startsWith("# go-embedded-ruby"), rbClip.slice(0, 40));
      // Both Go runtimes live on: the playground still answers clicks, and a
      // second run reuses the module already there.
      const st2 = (await rects()).steps;
      await reveal(st2);
      for (let y = st2.y + 4; y < st2.y + st2.h; y += 4) {
        await clickAt(canvasBox.x + st2.x + 40, canvasBox.y + y);
        d = await debug();
        if (d.selected === 1) break;
      }
      check(tag + "after Ruby ran, clicking step b still selects it", d.selected === 1 && d.code.endsWith("puts b.shape.inspect\nputs b.to_a.inspect\n"), d.selected);
      await click("run");
      await page.waitForFunction(() => JSON.parse(ndarrayDebug()).rubyOutput !== "", { timeout: 120000, polling: 250 });
      d = await debug();
      const l2 = d.rubyOutput.trim().split("\n");
      check(tag + "a second run prints b's shape", l2[l2.length - 2] === "[2, 3]", l2.slice(-2).join(" | "));
      check(tag + "the Ruby module was fetched once", rubyRequests.length === 1, rubyRequests.length);
    }

    check(tag + "no page or console error", errors.length === 0, errors.join(" | "));
  } finally {
    await browser.close();
  }
}

(async () => {
  try {
    await run("light", 1, DESKTOP, { ruby: true });
    await run("dark", 1);
    await run("light", 2);
    await run("dark", 2);
    await run("light", 3, PHONE, { ruby: true });
    console.log("RESULT " + JSON.stringify({ ok: true, results }));
  } catch (e) {
    console.log("RESULT " + JSON.stringify({ ok: false, error: String(e.message || e), results }));
    process.exit(1);
  }
})();
