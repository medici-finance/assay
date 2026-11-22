#!/usr/bin/env node
// Validate local deck and film renders. See ../references/render-check.md.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync, spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

const TOOL = process.env.EDITORIAL_RENDER_DEPS || process.cwd();
const PLAYWRIGHT_ENTRY = path.join(TOOL, 'node_modules', 'playwright', 'index.mjs');

const FILM_VIEWPORT = { width: 1920, height: 1080 };   // record.mjs's stage
const DECK_VIEWPORT = { width: 1280, height: 720 };    // deck-author's fixed stage
const TILE = { w: 480, h: 270, margin: 8, padding: 8 };
const MAX_COLS = 4;
const MIN_MID_FRAMES = 6;
const OVERFLOW_TOLERANCE_PX = 2;
const TAIL_S = 0.5;                 // house-player's hold before dataset.done
const RECORD_PAD_MAX_S = 2.0;       // recorder pad + page-load skew allowance
// Playwright's webm starts at its first screencast frame (after first paint),
// not at page load, and a film whose last paint comes early gets no recorder
// pad beyond the close. Measured on fixtures/front-loaded (last paint 4.5 s,
// total+tail 6.5 s), 12 recordings: 6.44-6.72 s. The lower bound allows that
// start offset; the upper bound (the 12:14-against-7:06 class) is unchanged.
const RECORD_START_SLACK_S = 0.25;
const SHORT_SCENE_S = 0.6;          // Pass C's short per-scene durations
const SYNC_FLOOR_S = 0.1;           // ~6 frames at 60 fps: the boundary-detection floor
const SETTLE_MS = 150;              // real-time gap between a frame's two captures (the settle check)
const CLOCK_EPOCH_MS = Date.UTC(2026, 0, 1); // the held JS clock's Date at install: fixed, so Date-driven content reproduces
const RAF_PRIME_MS = 32;            // two frames of JS time at load before the jump to t
const DECK_JS_MS = 1000;            // JS time a deck slide gets before its tile (builds are stepped to their end)
const ALLOWED_WARNINGS = [/^\[house-player\] window\.SCENE_DURATIONS is empty/];

// The pipeline's own injection regex (video-author SKILL.md §2, verbatim).
const SCENE_DURATIONS_RE = /window\.SCENE_DURATIONS\s*=\s*\[[^\]]*\]\s*;?/g;
const FALLBACK_RE = /window\.FALLBACK\s*=\s*\[[^\]]*\]\s*;?/;

// Browser launch arguments.
//
// BANNED here, and anywhere a frame must show animation state:
// --virtual-time-budget. Measured in the RWA deck bake-off (verified via
// document.getAnimations()): it fast-forwards requestAnimationFrame while
// leaving every CSS animation at currentTime 0, so every "mid-animation"
// screenshot comes back as the scene-ENTRY state, which reads exactly like a
// broken build. It is not a speed optimisation — do not reintroduce it. Frames
// are frozen by freezeAt() (the ?at= seek, the held JS clock, the
// getAnimations() walk, SMIL and <video> holds) and proven still by shoot().
const LAUNCH_ARGS = [];

// ---------------------------------------------------------------- transcript

const transcript = [];
const failures = [];
const notices = [];
function out(line) { transcript.push(line); console.log(line); }
function ok(check, detail) { out(`ok: ${check}: ${detail}`); }
function fail(check, detail) { failures.push(`${check}: ${detail}`); out(`FAIL: ${check}: ${detail}`); }
function notice(check, detail) { notices.push(`${check}: ${detail}`); out(`NOTICE (not checked): ${check}: ${detail}`); }

function usage(msg) {
  if (msg) console.error('render-check: ' + msg);
  console.error('usage: node render-check.mjs [--contact-sheet] <dir>\n' +
    '       node render-check.mjs --at <sec> --dump-animation-times [--no-freeze] <dir>');
  process.exit(2);
}

function parseArgs(argv) {
  const o = { contactSheet: false, at: null, dump: false, freeze: true, target: null };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--contact-sheet') o.contactSheet = true;
    else if (a === '--dump-animation-times') o.dump = true;
    else if (a === '--no-freeze') o.freeze = false;
    else if (a === '--at') {
      o.at = Number(argv[++i]);
      if (!Number.isFinite(o.at) || o.at < 0) usage('--at needs a non-negative number of seconds');
    } else if (a === '-h' || a === '--help') usage();
    else if (a.startsWith('--')) usage('unknown flag ' + a);
    else if (o.target) usage('one target directory only');
    else o.target = a;
  }
  if (!o.target) usage('missing target directory');
  if (!o.freeze && !o.dump) usage('--no-freeze is the negative control for --dump-animation-times only; the gate never takes an unfrozen frame');
  if (o.dump && o.at == null) usage('--dump-animation-times needs --at <sec>');
  if (o.dump && o.contactSheet) usage('--dump-animation-times and --contact-sheet are separate runs');
  return o;
}

// ------------------------------------------------------------------ helpers

function sha256(buf) { return crypto.createHash('sha256').update(buf).digest('hex'); }

// What a sheet is a look at: the artifact's HTML AND every local file it
// references (src/href attributes and CSS url(...) — the engine's player.js,
// fonts, images, video). Editing any of them stales the sheet. `digest` is
// the one value the gate compares; `sources` lets a stale failure name the
// file that changed. One level deep: a file referenced only from a referenced
// stylesheet or script (an @font-face url() inside an external .css, say) is
// not hashed, so editing it does not stale the sheet.
function sourceHashes(file, source) {
  const refs = new Set();
  for (const m of source.matchAll(/\b(?:src|href)\s*=\s*["']([^"']+)["']/gi)) refs.add(m[1]);
  for (const m of source.matchAll(/url\(\s*["']?([^"')]+)["']?\s*\)/gi)) refs.add(m[1]);
  const sources = [];
  for (const ref of [...refs].sort()) {
    if (/^([a-z][a-z0-9+.-]*:|\/\/|#)/i.test(ref)) continue; // remote, data:, about:, fragment
    const rel = ref.replace(/[?#].*$/, '');
    if (!rel) continue;
    const abs = path.resolve(path.dirname(file), decodeURIComponent(rel));
    const entry = { ref: rel };
    try { entry.sha256 = sha256(fs.readFileSync(abs)); } catch { entry.sha256 = null; }
    sources.push(entry);
  }
  const html = sha256(Buffer.from(source));
  const digest = sha256(Buffer.from(JSON.stringify([html, sources.map((x) => [x.ref, x.sha256])])));
  return { sha256: html, sources, digest };
}

function pngSize(buf) {
  // PNG signature (8 bytes) + IHDR length/type (8 bytes), then width, height.
  if (buf.length < 24 || buf.readUInt32BE(12) !== 0x49484452) return null;
  return { width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) };
}

function fmtT(t) { return t.toFixed(3); }

function slug(s) { return String(s).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'x'; }

function mdCell(s) { return String(s).replace(/\|/g, '\\|').replace(/\s+/g, ' ').trim(); }

function discover(target) {
  const abs = path.resolve(target);
  if (!fs.existsSync(abs) || !fs.statSync(abs).isDirectory()) usage(`${target} is not a directory`);
  const films = [];
  const decks = [];
  for (const name of fs.readdirSync(abs).sort()) {
    if (!name.endsWith('.html')) continue;
    const file = path.join(abs, name);
    if (!fs.statSync(file).isFile()) continue;
    const src = fs.readFileSync(file, 'utf8');
    if (/\bdata-scene\s*=/.test(src)) films.push(file);
    else if (/class\s*=\s*["'][^"']*\bslide\b/.test(src)) decks.push(file);
  }
  return { dir: abs, films, decks };
}

// A rewritten copy lives in a temp dir; a <base> pointing at the ORIGINAL
// directory keeps every relative asset (the demo's ../player.js, fonts, images)
// resolving exactly as it does for the original.
function writeTempCopy(source, origFile, label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'render-check-'));
  const base = `<base href="${pathToFileURL(path.dirname(origFile) + path.sep).href}">`;
  const withBase = /<head[^>]*>/i.test(source)
    ? source.replace(/<head([^>]*)>/i, (m, attrs) => `<head${attrs}>${base}`)
    : base + source;
  const p = path.join(dir, `${label}.html`);
  fs.writeFileSync(p, withBase);
  return p;
}

function rmTemp(p) { try { fs.rmSync(path.dirname(p), { recursive: true, force: true }); } catch { /* best effort */ } }

function newSink() { return { errors: [], warnings: [], offline: [] }; }

function instrument(page, sink) {
  page.on('pageerror', (e) => sink.errors.push('pageerror: ' + e.message));
  page.on('console', (m) => {
    const type = m.type();
    if (type === 'error') sink.errors.push('console.error: ' + m.text());
    else if (type === 'warning') {
      if (!ALLOWED_WARNINGS.some((re) => re.test(m.text()))) sink.warnings.push('console.warn: ' + m.text());
    }
  });
  page.on('request', (r) => {
    const u = r.url();
    if (!/^(file|data|blob|about):/i.test(u)) sink.offline.push(u);
  });
}

async function loadPlaywright() {
  if (!fs.existsSync(PLAYWRIGHT_ENTRY)) {
    console.error(`render-check: Playwright is not installed at ${PLAYWRIGHT_ENTRY}.\n` +
      `Install it once, into this tool's own directory (never the working repo):\n` +
      `  cd ${TOOL} && npm install playwright && npx playwright install chromium\n` +
      `This harness does not degrade to a source-only check: a render gate that rendered nothing is NOT RUN.`);
    process.exit(2);
  }
  return import(pathToFileURL(PLAYWRIGHT_ENTRY).href);
}

async function launch(chromium) {
  if (LAUNCH_ARGS.some((a) => /virtual-time/i.test(a))) {
    throw new Error('a virtual-time flag is in LAUNCH_ARGS — banned (see the comment above LAUNCH_ARGS)');
  }
  const opts = { args: LAUNCH_ARGS };
  if (process.env.RENDER_CHECK_CHROMIUM) opts.executablePath = process.env.RENDER_CHECK_CHROMIUM;
  try {
    return await chromium.launch(opts);
  } catch (e) {
    console.error('render-check: could not launch Chromium: ' + (e && e.message || e).split('\n')[0] + '\n' +
      `Install Playwright's browser (cd ${TOOL} && npx playwright install chromium), ` +
      'or set RENDER_CHECK_CHROMIUM to a local Chromium/Chrome binary.');
    process.exit(2);
  }
}

// ------------------------------------------------------- in-page functions

// Runs IN THE PAGE. Computes, for every document.getAnimations() entry, the
// currentTime the film's schedule implies at absolute time t — and, when
// `drive` is set, sets it and pauses the animation. The schedule:
//   - an engine beat (target carries data-at, inside a [data-scene]): starts
//     at sceneStart + data-at, lasts data-dur (house-player's own model);
//   - any other animation inside a scene runs on that scene's clock from scene
//     entry; its own delay/duration live in its effect timing;
//   - an animation outside every scene runs on the film clock.
function pageAnimations(args) {
  const { t, drive } = args;
  const p = window.__player;
  const durs = (p && Array.isArray(p.durations)) ? p.durations.map(Number) : [];
  const scenes = Array.prototype.slice.call(document.querySelectorAll('.scene[data-scene]'));
  const cum = [];
  let acc = 0;
  for (let i = 0; i < durs.length; i++) { cum.push(acc); acc += durs[i] || 0; }
  function describe(el) {
    if (!el || !el.tagName) return '(no target)';
    let d = el.tagName.toLowerCase();
    if (el.id) d += '#' + el.id;
    const cls = (typeof el.className === 'string' ? el.className : '').trim().split(/\s+/).filter(Boolean);
    if (cls.length) d += '.' + cls.join('.');
    return d;
  }
  const rows = [];
  document.getAnimations().forEach((a, i) => {
    const el = a.effect && a.effect.target;
    const sceneEl = el && el.closest ? el.closest('.scene[data-scene]') : null;
    const sIdx = sceneEl ? scenes.indexOf(sceneEl) : -1;
    const sStart = sIdx >= 0 ? (cum[sIdx] || 0) : 0;
    const timing = a.effect ? a.effect.getComputedTiming() : { delay: 0, activeDuration: 0, endTime: 0 };
    const atAttr = el && el.getAttribute ? el.getAttribute('data-at') : null;
    let kind, start, dur, expected;
    if (atAttr != null && sIdx >= 0) {
      const at = parseFloat(atAttr) || 0;
      let d = parseFloat(el.getAttribute('data-dur'));
      if (!(d > 0)) d = 0.6;
      kind = 'beat';
      start = sStart + at;
      dur = d;
      expected = Math.min(Math.max(t - start, 0), d) * 1000;
    } else {
      kind = sIdx >= 0 ? 'scene-clock' : 'film-clock';
      start = sStart + (timing.delay || 0) / 1000;
      dur = timing.activeDuration / 1000;
      const local = Math.max(t - sStart, 0) * 1000;
      expected = Number.isFinite(timing.endTime) ? Math.min(local, timing.endTime) : local;
    }
    const before = a.currentTime == null ? null : Number(a.currentTime);
    if (drive) {
      // pause() FIRST. On a running animation pause() is asynchronous: the
      // hold time is taken when the pending pause commits, a frame later, so
      // set-then-pause lets a CSS animation drift past its offset (measured:
      // +8.5 ms on a `.scene.on`-triggered @keyframes). Pausing first makes the
      // currentTime assignment the hold time itself.
      try { a.pause(); a.currentTime = expected; } catch (e) { /* cancelled mid-walk */ }
    }
    rows.push({ i, type: a.constructor.name, target: describe(el), scene: sIdx + 1, kind, start, dur, expected, before });
  });
  void document.body.offsetHeight; // force style + layout with the driven values
  return rows;
}

// Runs IN THE PAGE. Reads every animation back (after layout and painted
// frames) and classifies it as strictly inside its active interval or not.
function pageReadBack() {
  return document.getAnimations().map((a) => {
    const ct = a.currentTime == null ? null : Number(a.currentTime);
    const tm = a.effect ? a.effect.getComputedTiming() : { delay: 0, activeDuration: 0 };
    const lo = tm.delay || 0;
    const hi = lo + tm.activeDuration;
    const mid = ct != null && Number.isFinite(hi) && ct > lo + 0.5 && ct < hi - 0.5;
    return { currentTime: ct, playState: a.playState, mid };
  });
}

// Runs IN THE PAGE. Visibility / ink / overflow walks share one notion of
// "visible": displayed, visibility visible, effective opacity (through every
// ancestor) above 0.01, non-empty box.
function pageSurfaceStats(args) {
  const { root, vw, vh, tol } = args;
  const scope = root === 'scene' ? document.querySelector('.scene.on') : document.querySelector('.slide.active');
  const res = { hasScope: !!scope, inked: 0, overflow: [], dpr: window.devicePixelRatio };
  if (!scope) return res;
  function effOpacity(el) {
    let o = 1;
    for (let n = el; n && n.nodeType === 1; n = n.parentElement) {
      const cs = getComputedStyle(n);
      if (cs.display === 'none') return 0;
      o *= parseFloat(cs.opacity);
    }
    return o;
  }
  function visible(el) {
    const cs = getComputedStyle(el);
    if (cs.visibility !== 'visible') return false;
    if (effOpacity(el) <= 0.01) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  }
  function alpha(c) {
    if (!c || c === 'transparent') return 0;
    const m = c.match(/rgba?\(([^)]+)\)/);
    if (!m) return 1;
    const parts = m[1].split(/[ ,/]+/).filter(Boolean);
    return parts.length >= 4 ? parseFloat(parts[3]) : 1;
  }
  const SHAPES = { path: 1, rect: 1, circle: 1, ellipse: 1, line: 1, polyline: 1, polygon: 1, text: 1, image: 1, use: 1 };
  function inked(el) {
    const tag = el.tagName.toLowerCase();
    if (tag === 'svg') return false; // its shapes count individually
    if (el instanceof SVGElement) return !!SHAPES[tag];
    if (tag === 'img' || tag === 'canvas' || tag === 'video') return true;
    for (const n of el.childNodes) if (n.nodeType === 3 && n.textContent.trim()) return true;
    const cs = getComputedStyle(el);
    if (alpha(cs.backgroundColor) > 0.01 || (cs.backgroundImage && cs.backgroundImage !== 'none')) return true;
    const bw = parseFloat(cs.borderTopWidth) + parseFloat(cs.borderRightWidth) + parseFloat(cs.borderBottomWidth) + parseFloat(cs.borderLeftWidth);
    return bw > 0 && alpha(cs.borderTopColor) > 0.01;
  }
  function describe(el) {
    let d = el.tagName.toLowerCase();
    if (el.id) d += '#' + el.id;
    const cls = (typeof el.className === 'string' ? el.className : '').trim().split(/\s+/).filter(Boolean);
    if (cls.length) d += '.' + cls.slice(0, 3).join('.');
    return d;
  }
  for (const el of scope.querySelectorAll('*')) {
    if (!visible(el)) continue;
    const r = el.getBoundingClientRect();
    const onStage = r.right > 0 && r.bottom > 0 && r.left < vw && r.top < vh;
    if (onStage && inked(el)) res.inked++;
    if (r.left < -tol || r.top < -tol || r.right > vw + tol || r.bottom > vh + tol) {
      res.overflow.push(`${describe(el)} rect x=${Math.round(r.left)}..${Math.round(r.right)} y=${Math.round(r.top)}..${Math.round(r.bottom)}`);
    }
  }
  return res;
}

// Runs IN THE PAGE. What a scene tile should show, for INDEX.md.
function pageSceneInfo() {
  return Array.prototype.slice.call(document.querySelectorAll('.scene[data-scene]')).map((s) => {
    const label = s.getAttribute('data-shows') || s.getAttribute('aria-label') || s.getAttribute('data-title');
    const text = (s.textContent || '').replace(/\s+/g, ' ').trim();
    const beats = Array.prototype.slice.call(s.querySelectorAll('[data-at]')).map((el) => {
      const cls = (typeof el.className === 'string' ? el.className : '').trim().split(/\s+/).filter(Boolean);
      return el.tagName.toLowerCase() + (cls.length ? '.' + cls[0] : '') + ' @' + el.getAttribute('data-at') + 's';
    });
    return { label, text: text.slice(0, 90), beats };
  });
}

// Runs IN THE PAGE. Holds the two kinds of motion that run on the browser's
// own clocks rather than on JS time or document.getAnimations():
//   - SMIL: every outermost <svg> timeline is paused and set to t seconds
//     (an SVG document timeline starts at page load, like the JS clock);
//   - <video>: every element is paused; one that was playing (or autoplays)
//     is seeked to t seconds from load (mod its length when it loops), one
//     that was already paused keeps its position.
// Returns what it held, for the transcript. Seeking is asynchronous: the
// caller waits for every video to finish seeking before the frame is taken.
function pageHoldMedia(args) {
  const { t } = args;
  const held = { svg: 0, smil: 0, video: [] };
  for (const s of document.querySelectorAll('svg')) {
    if (s.ownerSVGElement || typeof s.pauseAnimations !== 'function') continue;
    held.svg++;
    s.pauseAnimations();
    s.setCurrentTime(t);
    held.smil += s.querySelectorAll('animate, animateTransform, animateMotion, set').length;
  }
  document.querySelectorAll('video').forEach((v, i) => {
    const running = !v.paused || v.autoplay;
    v.pause();
    const d = v.duration;
    let target = v.currentTime;
    if (running && Number.isFinite(d) && d > 0) target = v.loop ? t % d : Math.min(t, d);
    else if (running && !(v.readyState >= 1)) target = t; // metadata not in yet: set once it is
    if (v.readyState >= 1) v.currentTime = target;
    else v.addEventListener('loadedmetadata', () => { v.pause(); const dd = v.duration; v.currentTime = Number.isFinite(dd) && dd > 0 ? (v.loop ? target % dd : Math.min(target, dd)) : 0; }, { once: true });
    held.video.push({ i, running, target });
  });
  void document.body.offsetHeight;
  return held;
}

class NotHousePlayer extends Error {}

// Every page a frame is taken from runs on a fake JS clock (Playwright
// page.clock): Date, performance.now(), setTimeout/setInterval and
// requestAnimationFrame are installed at a FIXED epoch and paused before the
// page loads, so JS-driven motion (a rAF canvas, a JS tween, a Date.now()
// counter) only moves when the harness advances it, by an exact amount.
async function holdClock(page) {
  await page.clock.install({ time: CLOCK_EPOCH_MS });
  await page.clock.pauseAt(CLOCK_EPOCH_MS + 1000);
}

// Advance the held JS clock to `ms` after load: a few frames first (so a loop
// that takes its start time from its first callback has taken it), then one
// jump, which fires each due timer and the pending rAF once with the new time.
// Motion computed from elapsed time (SKILL §8b: never frame-counted) lands
// exactly at `ms`; the clock stays paused afterwards.
async function advanceClock(page, ms) {
  await page.clock.runFor(RAF_PRIME_MS);
  if (ms > RAF_PRIME_MS) await page.clock.fastForward(Math.round(ms - RAF_PRIME_MS));
}

async function holdMedia(page, t) {
  const held = await page.evaluate(pageHoldMedia, { t });
  if (held.video.length) {
    try {
      await page.waitForFunction(() => Array.prototype.every.call(document.querySelectorAll('video'),
        (v) => v.paused && !v.seeking && (v.readyState >= 2 || v.networkState === 3 || v.error)), undefined, { timeout: 10000 });
    } catch {
      held.videoTimeout = true;
    }
  }
  return held;
}

function describeHeld(h) {
  const v = h.video.length ? `${h.video.length} <video> paused${h.video.some((x) => x.running) ? ' (playing ones seeked to ' + h.video.filter((x) => x.running).map((x) => x.target.toFixed(3) + 's').join(', ') + ')' : ''}${h.videoTimeout ? ' — a video did not finish seeking in 10 s' : ''}` : '0 <video>';
  return `JS clock held at load+${fmtT(h.jsT)}s; ${h.svg} <svg> timeline(s) paused at ${fmtT(h.t)}s (${h.smil} SMIL element(s)); ${v}`;
}

// THE freeze recipe, in order. Every seeked frame the gate looks at goes
// through here; nothing else in this file positions a film's timeline.
//
// What it holds: the engine clock (?at=), every document.getAnimations()
// entry (CSS animations, CSS transitions, WAAPI), JS time, SMIL and <video>.
// What it cannot hold — an animated GIF/APNG/WebP, an <iframe>'s own
// document, a worker's own timers — is caught by the settle check in shoot():
// a frame that still moves after the freeze fails, it is never passed.
async function freezeAt(browser, file, t, sink, viewport = FILM_VIEWPORT) {
  const page = await browser.newPage({ viewport, deviceScaleFactor: 1 });
  instrument(page, sink);
  // 0. hold JS time before any film script runs.
  await holdClock(page);
  // 1. navigate with ?at=<sec> — the engine seeks and pauses its master clock.
  await page.goto(pathToFileURL(file).href + '?at=' + fmtT(t));
  // 2. wait for the engine. A page that never raises __playerReady is not a
  //    house-player film (the GSAP adapter exposes neither it nor ?at=): the
  //    freeze cannot hold it, so say so instead of timing out with a stack.
  try {
    await page.waitForFunction(() => window.__playerReady === true, undefined, { timeout: 10000 });
  } catch (e) {
    await page.close();
    throw new NotHousePlayer(`${path.basename(file)} is not a house-player film: window.__playerReady never became true in 10 s, ` +
      'so there is no ?at= seek to freeze it with. render-check gates house-player films only (renderers/house-player/player.js); ' +
      'a GSAP-adapter film is previewed with renderers/gsap/qa.mjs and is not gated here');
  }
  // 3. advance the held JS clock to t — rAF/timer/Date-driven content keyed
  //    on elapsed time since load lands where real playback has it at t.
  await advanceClock(page, t * 1000);
  // 4. walk document.getAnimations(): set each entry's currentTime to the
  //    offset the schedule implies, pause() it — this also catches animations
  //    the engine does not track (an effect started by `.scene.on`, say).
  const rows = await page.evaluate(pageAnimations, { t, drive: true });
  // 5. pause every SVG timeline at t and every <video>, seek the playing ones.
  const held = await holdMedia(page, t);
  held.t = t;
  // advanceClock always primes RAF_PRIME_MS first, so for t below that the JS
  // clock sits at the prime, not at t: record where it really is.
  held.jsT = Math.max(t * 1000, RAF_PRIME_MS) / 1000;
  // 6. read every currentTime back (pause-then-set commits synchronously, so
  //    the values are the hold times; layout was forced by the walk).
  const back = await page.evaluate(pageReadBack);
  rows.forEach((r, i) => { Object.assign(r, back[i] || {}); });
  return { page, rows, held };
}

// Decode a PNG to raw rgb24 with ffmpeg (already a requirement). Only used to
// describe a settle failure, never to decide one.
function decodeRgb(buf) {
  const r = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-i', 'pipe:0', '-f', 'rawvideo', '-pix_fmt', 'rgb24', '-'], { input: buf, maxBuffer: 1 << 28 });
  return r.error || r.status !== 0 ? null : r.stdout;
}

function diffBox(a, b, w, h) {
  const ra = decodeRgb(a);
  const rb = decodeRgb(b);
  if (!ra || !rb || ra.length !== w * h * 3 || rb.length !== ra.length) return null;
  let n = 0, x0 = w, y0 = h, x1 = -1, y1 = -1;
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const o = (y * w + x) * 3;
      if (ra[o] !== rb[o] || ra[o + 1] !== rb[o + 1] || ra[o + 2] !== rb[o + 2]) {
        n++;
        if (x < x0) x0 = x; if (x > x1) x1 = x; if (y < y0) y0 = y; if (y > y1) y1 = y;
      }
    }
  }
  return n ? { n, x0, y0, x1, y1 } : null;
}

// Runs IN THE PAGE. Names what is painted at a point, for a settle failure.
function pageDescribeAt(args) {
  const { x, y } = args;
  return document.elementsFromPoint(x, y).slice(0, 3).map((el) => {
    let d = el.tagName.toLowerCase();
    if (el.id) d += '#' + el.id;
    const cls = (typeof el.className === 'string' ? el.className : '').trim().split(/\s+/).filter(Boolean);
    if (cls.length) d += '.' + cls.slice(0, 2).join('.');
    return d;
  });
}

// Every frame the gate keeps is captured twice, SETTLE_MS apart in real time,
// AFTER the freeze. The two captures must be byte-identical: anything still
// moving is content the freeze did not hold, and a frame that is not a
// function of the time it claims is not evidence of that time. An unsettled
// frame FAILS; it is never counted, tiled or passed as frozen.
async function shoot(page, file, viewport) {
  const first = await page.screenshot();
  await new Promise((r) => setTimeout(r, SETTLE_MS));
  const buf = await page.screenshot();
  fs.writeFileSync(file, buf);
  const size = pngSize(buf);
  // Unscaled capture: one CSS pixel is one image pixel, or every coordinate
  // the eye reads off the frame is wrong.
  if (!size || size.width !== viewport.width || size.height !== viewport.height) {
    fail('shots', `${path.basename(file)} is ${size ? size.width + 'x' + size.height : 'not a PNG'}, expected an unscaled ${viewport.width}x${viewport.height} capture`);
  }
  if (!first.equals(buf)) {
    const box = size ? diffBox(first, buf, size.width, size.height) : null;
    let where = '';
    if (box) {
      const at = await page.evaluate(pageDescribeAt, { x: Math.round((box.x0 + box.x1) / 2), y: Math.round((box.y0 + box.y1) / 2) });
      where = ` — ${box.n} px changed in x=${box.x0}..${box.x1} y=${box.y0}..${box.y1}, painted there: ${at.join(' > ') || '(nothing)'}`;
    }
    fail('settle', `unsettled motion in ${path.basename(file)}: two captures ${SETTLE_MS} ms apart after the freeze differ${where}. ` +
      'This is content the freeze cannot hold (rAF/timer/SMIL/video/animated image/iframe) — the frame is not frozen at the time it claims');
    return { buf, settled: false };
  }
  return { buf, settled: true };
}

async function waitDone(page, timeoutMs) {
  const t0 = Date.now();
  try {
    await page.waitForFunction(() => document.body && document.body.dataset.done === '1', undefined, { timeout: timeoutMs });
  } catch {
    return { done: false, elapsed: Date.now() - t0 };
  }
  const info = await page.evaluate(() => ({
    reason: document.body.dataset.doneReason || null,
    error: document.body.dataset.error || null,
    durations: window.__player ? window.__player.durations : null,
    syncMarks: window.__explainer ? window.__explainer.syncMarks : null,
  }));
  return { done: true, elapsed: Date.now() - t0, ...info };
}

function sameArray(a, b) {
  return Array.isArray(a) && Array.isArray(b) && a.length === b.length && a.every((v, i) => Math.abs(Number(v) - Number(b[i])) < 1e-6);
}

function ffmpegDuration(file) {
  const r = spawnSync('ffmpeg', ['-hide_banner', '-nostats', '-i', file, '-f', 'null', '-'], { encoding: 'utf8' });
  if (r.error) return { error: r.error.message };
  // Playwright webms carry no duration header: decode, and take the last time=.
  const all = [...String(r.stderr).matchAll(/time=(\d+):(\d+):(\d+(?:\.\d+)?)/g)];
  if (!all.length) return { error: 'ffmpeg printed no time= progress' };
  const m = all[all.length - 1];
  return { seconds: Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]) };
}

// ----------------------------------------------------------------- a film

async function checkFilm(browser, file, shotsDir, opts, sheetEntries) {
  const name = path.basename(file);
  const stem = name.replace(/\.html$/, '');
  const source = fs.readFileSync(file, 'utf8');
  const sink = newSink();
  const res = { name, scenes: 0, shots: 0, mid: 0, midFrames: [], ...sourceHashes(file, source) };
  out(`== film ${name}`);

  // ---- Pass A — LOAD (static)
  const nAssign = (source.match(SCENE_DURATIONS_RE) || []).length;
  if (nAssign === 1) ok('load', 'exactly one SCENE_DURATIONS assignment matches the pipeline regex');
  else fail('load', `${nAssign} SCENE_DURATIONS assignment(s) match the pipeline regex in ${name}, expected exactly 1`);
  const refs = [...source.matchAll(/\b(?:src|href)\s*=\s*["']([^"']+)["']/gi)].map((m) => m[1]);
  const remote = refs.filter((r) => /^(https?:)?\/\//i.test(r));
  const local = refs.filter((r) => !/^(https?:)?\/\//i.test(r) && !/^(data:|#|about:)/i.test(r));
  if (remote.length) fail('load', `external reference(s): ${remote.join(', ')}`);
  if (local.length) notice('load', `not self-contained — references ${local.join(', ')} on disk; inline before hand-off (video-author SKILL §2)`);

  // ---- Pass A — LOAD (in the browser, held at ?at=0)
  let load;
  try {
    load = await freezeAt(browser, file, 0, sink);
  } catch (e) {
    if (!(e instanceof NotHousePlayer)) throw e;
    fail('load', e.message);
    return res;
  }
  const shim = await load.page.evaluate(() => ({
    player: !!window.__player && ['seek', 'play', 'pause'].every((k) => typeof window.__player[k] === 'function'),
    ready: window.__playerReady === true,
    render: window.__renderReady === true,
    durations: window.__player ? window.__player.durations : null,
    total: window.__player ? window.__player.total : null,
    domScenes: document.querySelectorAll('.scene[data-scene]').length,
    doneReason: document.body.dataset.doneReason || null,
    error: document.body.dataset.error || null,
  }));
  const fonts = await load.page.evaluate(async () => {
    await document.fonts.ready;
    const bad = [];
    const families = new Set();
    for (const f of document.fonts) {
      if (f.status === 'unloaded') { try { await f.load(); } catch (e) { /* status says error */ } }
      if (f.status === 'error') bad.push(f.family);
      else families.add(f.family.replace(/["']/g, ''));
    }
    for (const fam of families) if (!document.fonts.check(`16px "${fam}"`)) bad.push(fam);
    return { bad, n: families.size };
  });
  const sceneInfo = await load.page.evaluate(pageSceneInfo);
  await load.page.close();
  if (shim.player && shim.ready && shim.render) ok('load', 'shim exposed: __player{seek,play,pause}, __playerReady, __renderReady');
  else fail('load', `shim incomplete: __player=${shim.player} __playerReady=${shim.ready} __renderReady=${shim.render}`);
  if (shim.doneReason === 'error') fail('load', `the film failed at boot: ${shim.error}`);
  const durations = Array.isArray(shim.durations) ? shim.durations.map(Number) : [];
  res.scenes = shim.domScenes;
  if (durations.length === shim.domScenes && shim.domScenes > 0) ok('load', `${shim.domScenes} scene(s) == ${durations.length} duration(s)`);
  else fail('load', `${shim.domScenes} .scene[data-scene] element(s) vs ${durations.length} duration(s)`);
  const sum = durations.reduce((a, b) => a + b, 0);
  if (typeof shim.total !== 'number' || Math.abs(shim.total - sum) > 0.05) fail('load', `__player.total=${shim.total} but the durations sum to ${sum}s — total must be SECONDS`);
  const scriptMd = path.join(path.dirname(file), 'SCRIPT.md');
  if (fs.existsSync(scriptMd)) {
    const rows = (fs.readFileSync(scriptMd, 'utf8').match(/^\|\s*\d+\s*\|/gm) || []).length;
    if (rows === shim.domScenes) ok('load', `scenes == SCRIPT.md table rows (${rows})`);
    else fail('load', `${shim.domScenes} scene(s) but SCRIPT.md has ${rows} numbered table row(s)`);
  } else {
    notice('load', `scenes vs SCRIPT.md rows — no SCRIPT.md beside ${name}`);
  }
  if (fonts.bad.length) fail('load', `font(s) not resolved: ${fonts.bad.join(', ')}`);
  else ok('load', `fonts resolved (${fonts.n} face famil${fonts.n === 1 ? 'y' : 'ies'})`);
  if (!durations.length) { out('   (no scenes — the remaining film passes cannot run)'); return res; }

  const cum = [];
  { let acc = 0; for (const d of durations) { cum.push(acc); acc += d; } }
  const total = sum;

  // ---- Pass A — overflow, measured per element, seeked to ~60 % of every scene
  let overflowHits = 0;
  for (let s = 0; s < durations.length; s++) {
    const t = cum[s] + durations[s] * 0.6;
    const f = await freezeAt(browser, file, t, sink);
    const st = await f.page.evaluate(pageSurfaceStats, { root: 'scene', vw: FILM_VIEWPORT.width, vh: FILM_VIEWPORT.height, tol: OVERFLOW_TOLERANCE_PX });
    await f.page.close();
    if (st.overflow.length) {
      overflowHits += st.overflow.length;
      fail('overflow', `scene ${s + 1} at t=${fmtT(t)}s: ${st.overflow.length} visible element(s) leave the ${FILM_VIEWPORT.width}x${FILM_VIEWPORT.height} stage by >${OVERFLOW_TOLERANCE_PX}px: ${st.overflow.slice(0, 5).join('; ')}`);
    }
  }
  if (!overflowHits) ok('overflow', `every scene measured at 60 % of its slot: nothing leaves the stage`);

  // ---- Pass B — the front-load probe
  let frontLoaded = 0;
  for (let s = 0; s < durations.length; s++) {
    const t15 = cum[s] + durations[s] * 0.15;
    const t85 = cum[s] + durations[s] * 0.85;
    const a = await freezeAt(browser, file, t15, sink);
    const sa = await a.page.evaluate(pageSurfaceStats, { root: 'scene', vw: FILM_VIEWPORT.width, vh: FILM_VIEWPORT.height, tol: OVERFLOW_TOLERANCE_PX });
    await a.page.close();
    const b = await freezeAt(browser, file, t85, sink);
    const sb = await b.page.evaluate(pageSurfaceStats, { root: 'scene', vw: FILM_VIEWPORT.width, vh: FILM_VIEWPORT.height, tol: OVERFLOW_TOLERANCE_PX });
    await b.page.close();
    if (sb.inked < sa.inked) {
      frontLoaded++;
      fail('front-load', `scene ${s + 1}: ${sb.inked} visible inked element(s) at 85 % of its slot (t=${fmtT(t85)}s) < ${sa.inked} at 15 % (t=${fmtT(t15)}s) — the scene empties before its slot ends`);
    } else {
      out(`   scene ${s + 1}: ${sa.inked} inked at 15 %, ${sb.inked} at 85 %`);
    }
  }
  if (!frontLoaded) ok('front-load', `no scene has less visible content at 85 % of its slot than at 15 %`);

  // ---- Pass B — SHOTS: mid-animation frames, each one MEASURED mid-animation
  // The schedule is read with each scene active in turn: an effect a film
  // starts from `.scene.on` only exists while its scene is on.
  const byKey = new Map();
  for (let s = 0; s < durations.length; s++) {
    const f = await freezeAt(browser, file, cum[s] + durations[s] / 2, sink);
    for (const r of f.rows) {
      if (!(Number.isFinite(r.dur) && r.dur > 0)) continue;
      const key = `${r.target}|${r.scene}|${r.start.toFixed(3)}|${r.dur.toFixed(3)}`;
      if (!byKey.has(key)) byKey.set(key, r);
    }
    await f.page.close();
  }
  const anims = [...byKey.values()];
  const times = new Map();
  const addT = (t, why) => { const k = Math.round(t * 1000); if (k > 0 && k < Math.round(total * 1000) && !times.has(k)) times.set(k, why); };
  for (const r of anims) addT(r.start + r.dur * 0.5, `${r.target} 50%`);
  if (times.size < MIN_MID_FRAMES) {
    for (const r of anims) { addT(r.start + r.dur * 0.25, `${r.target} 25%`); addT(r.start + r.dur * 0.75, `${r.target} 75%`); }
  }
  const keys = [...times.keys()].sort((x, y) => x - y);
  for (const k of keys) {
    const t = k / 1000;
    const f = await freezeAt(browser, file, t, sink);
    const moving = f.rows.filter((r) => r.mid);
    const sIdx = cum.findLastIndex((c) => t >= c - 1e-9) + 1;
    const fileName = `${stem}-t${fmtT(t)}s-scene-${String(sIdx).padStart(2, '0')}-${slug(times.get(k))}.png`;
    const shot = await shoot(f.page, path.join(shotsDir, fileName), FILM_VIEWPORT);
    await f.page.close();
    res.shots++;
    if (moving.length && shot.settled) {
      res.mid++;
      res.midFrames.push({ file: fileName, t, scene: sIdx, moving: moving.map((r) => `${r.target} ${Math.round(r.currentTime)}ms`) });
    }
    out(`   shot ${fileName}: ${!shot.settled ? 'UNSETTLED (not counted)' : moving.length ? 'mid-animation (' + moving.map((r) => r.target).join(', ') + ')' : 'NOT mid-animation'}`);
  }
  ok('shots', `${res.shots} frame(s) taken, ${res.mid} measured mid-animation (a finite animation strictly inside its active interval after the freeze)`);

  // ---- tiles for the contact sheet: every scene at its midpoint
  if (opts.contactSheet) {
    for (let s = 0; s < durations.length; s++) {
      const t = cum[s] + durations[s] / 2;
      const f = await freezeAt(browser, file, t, sink);
      const fileName = `${stem}-scene-${String(s + 1).padStart(2, '0')}.png`;
      await shoot(f.page, path.join(shotsDir, fileName), FILM_VIEWPORT);
      await f.page.close();
      res.shots++;
      const info = sceneInfo[s] || {};
      const beats = info.beats || [];
      const shows = info.label || (info.text ? `"${info.text}"` : (beats.length ? `no text; mid-beat: ${beats.join(', ')}` : 'no text, no beats'));
      sheetEntries.push({ artifact: name, surface: `scene ${s + 1} of ${durations.length} (midpoint)`, frozen: `t=${fmtT(t)}s (?at=${fmtT(t)})`, shows, file: fileName });
    }
  }

  // ---- Pass C — DONE, both duration paths, and a proven-negative control
  await passC(browser, file, source, durations);

  // ---- Pass D — wall clock under recordVideo, against a plain run
  await passD(browser, file, total, durations.length, cum);

  // console / page / network hygiene over every page this film opened
  if (sink.errors.length) fail('load', `${sink.errors.length} page/console error(s): ${[...new Set(sink.errors)].slice(0, 5).join(' | ')}`);
  else ok('load', 'zero page errors, zero console errors');
  if (sink.warnings.length) fail('load', `${sink.warnings.length} console warning(s) beyond the runtime's named fallback: ${[...new Set(sink.warnings)].slice(0, 5).join(' | ')}`);
  else ok('load', "zero console warnings beyond the runtime's named FALLBACK warning");
  if (sink.offline.length) fail('load', `off-file:// request(s): ${[...new Set(sink.offline)].slice(0, 5).join(', ')}`);
  else ok('load', 'zero off-file:// requests');
  return res;
}

async function passC(browser, file, source, durations) {
  const n = durations.length;
  const short = new Array(n).fill(SHORT_SCENE_S);
  const budget = (SHORT_SCENE_S * n + TAIL_S + 10) * 1000;
  const controlSink = newSink(); // deliberately-broken pages are not the film's hygiene

  // Injected path: through the pipeline's OWN regex.
  if ((source.match(SCENE_DURATIONS_RE) || []).length === 1) {
    const inj = writeTempCopy(source.replace(SCENE_DURATIONS_RE, `window.SCENE_DURATIONS = ${JSON.stringify(short)};`), file, 'injected');
    const page = await browser.newPage({ viewport: FILM_VIEWPORT });
    instrument(page, controlSink);
    await page.goto(pathToFileURL(inj).href);
    const r = await waitDone(page, budget);
    await page.close();
    rmTemp(inj);
    if (r.done && r.reason === 'complete' && sameArray(r.durations, short)) ok('duration-paths', `injected ${JSON.stringify(short)} through the pipeline regex: doneReason=complete on the injected array`);
    else fail('duration-paths', `injected path: done=${r.done} reason=${r.reason} durations=${JSON.stringify(r.durations)} (expected complete on ${JSON.stringify(short)})`);
  } else {
    fail('duration-paths', 'injected path could not run: the pipeline regex does not match exactly once');
  }

  // Fallback path: empty declaration, shrunk authored FALLBACK.
  if (FALLBACK_RE.test(source)) {
    const fb = writeTempCopy(source
      .replace(SCENE_DURATIONS_RE, 'window.SCENE_DURATIONS = [];')
      .replace(FALLBACK_RE, `window.FALLBACK = ${JSON.stringify(short)};`), file, 'fallback');
    const page = await browser.newPage({ viewport: FILM_VIEWPORT });
    instrument(page, controlSink);
    await page.goto(pathToFileURL(fb).href);
    const r = await waitDone(page, budget);
    await page.close();
    rmTemp(fb);
    if (r.done && r.reason === 'complete' && sameArray(r.durations, short)) ok('duration-paths', `fallback (empty SCENE_DURATIONS, FALLBACK shrunk to ${JSON.stringify(short)}): doneReason=complete on the fallback array`);
    else fail('duration-paths', `fallback path: done=${r.done} reason=${r.reason} durations=${JSON.stringify(r.durations)}`);
  } else {
    fail('duration-paths', 'fallback path could not run: no window.FALLBACK declaration');
  }

  // Negative control — proven negative. Renaming the attribute on the
  // <section> tags ALONE is the break that is real (render-check.md: renaming
  // it everywhere also rewrites the runtime's own selector and the page
  // rebuilds itself). A control that changed nothing is not a control.
  let renamed = 0;
  const broken = source.replace(/(<section\b[^>]*?\s)data-scene(\s*=)/gi, (m, a, b) => { renamed++; return `${a}data-broken-scene${b}`; });
  if (!renamed) {
    fail('negative-control', 'could not break the build: no <section data-scene> tags to rename — the control is could-not-check, not a pass');
  } else {
    const nc = writeTempCopy(broken.replace(SCENE_DURATIONS_RE, `window.SCENE_DURATIONS = ${JSON.stringify(short)};`), file, 'broken');
    const page = await browser.newPage({ viewport: FILM_VIEWPORT });
    instrument(page, controlSink);
    await page.goto(pathToFileURL(nc).href);
    const r = await waitDone(page, budget);
    await page.close();
    rmTemp(nc);
    if (r.done && r.reason === 'error') ok('negative-control', `renamed data-scene on ${renamed} <section> tag(s) only: doneReason=error (${r.error}) — the control is proven negative`);
    else fail('negative-control', `a genuinely broken build (${renamed} <section> tag(s) lost data-scene) did NOT go red: done=${r.done} reason=${r.reason}`);
  }

  // Take-sanity gate (SKILL §8b): a poisoned take fails fast; honest drift does not.
  const fbm = source.match(FALLBACK_RE);
  let authored = null;
  try { authored = fbm ? JSON.parse(fbm[0].replace(/^[^[]*/, '').replace(/;?\s*$/, '')).map(Number) : null; } catch { authored = null; }
  if (!authored || authored.length !== n || (source.match(SCENE_DURATIONS_RE) || []).length !== 1) {
    fail('take-sanity', 'could not run: needs one SCENE_DURATIONS assignment and a FALLBACK array matching the scene count');
    return;
  }
  const poisoned = authored.slice();
  poisoned[0] = authored[0] * 12 + 8;
  const honest = authored.map((v) => Number((v * 1.15).toFixed(3)));
  const pp = writeTempCopy(source.replace(SCENE_DURATIONS_RE, `window.SCENE_DURATIONS = ${JSON.stringify(poisoned)};`), file, 'poisoned');
  const hp = writeTempCopy(source.replace(SCENE_DURATIONS_RE, `window.SCENE_DURATIONS = ${JSON.stringify(honest)};`), file, 'honest');
  try {
    const p1 = await browser.newPage({ viewport: FILM_VIEWPORT });
    instrument(p1, controlSink);
    await p1.goto(pathToFileURL(pp).href);
    const r1 = await waitDone(p1, 3000);
    await p1.close();
    if (r1.done && r1.reason === 'error' && /scene/i.test(r1.error || '') && r1.elapsed < 1000) ok('take-sanity', `poisoned take ${JSON.stringify(poisoned)} -> doneReason=error in ${r1.elapsed}ms: ${r1.error}`);
    else fail('take-sanity', `poisoned take ${JSON.stringify(poisoned)}: done=${r1.done} reason=${r1.reason} in ${r1.elapsed}ms (expected error naming the scene in <1000ms)`);
    // Honest drift, held at ?at=0: the gate runs at boot, so no real-time wait is needed.
    const p2 = await browser.newPage({ viewport: FILM_VIEWPORT });
    instrument(p2, controlSink);
    await p2.goto(pathToFileURL(hp).href + '?at=0');
    await p2.waitForFunction(() => window.__playerReady === true, undefined, { timeout: 10000 });
    const r2 = await p2.evaluate(() => ({ reason: document.body.dataset.doneReason || null, failure: window.__explainer ? window.__explainer.failure : null }));
    await p2.close();
    if (r2.reason !== 'error' && !r2.failure) ok('take-sanity', `honest 1.15x drift ${JSON.stringify(honest)} does not fire the gate`);
    else fail('take-sanity', `honest 1.15x drift fired the gate: ${r2.failure || r2.reason}`);
  } finally {
    rmTemp(pp);
    rmTemp(hp);
  }
}

async function runReal(browser, file, total, record) {
  const dir = record ? fs.mkdtempSync(path.join(os.tmpdir(), 'render-check-rec-')) : null;
  const ctx = await browser.newContext(record
    ? { viewport: FILM_VIEWPORT, recordVideo: { dir, size: FILM_VIEWPORT } }
    : { viewport: FILM_VIEWPORT });
  const page = await ctx.newPage();
  await page.goto(pathToFileURL(file).href);
  const r = await waitDone(page, (total + TAIL_S + 15) * 1000);
  const video = record ? page.video() : null;
  await page.close();
  await ctx.close();
  if (video) {
    const vp = await video.path();
    r.webm = ffmpegDuration(vp);
  }
  if (dir) fs.rmSync(dir, { recursive: true, force: true });
  return r;
}

async function passD(browser, file, total, nScenes, cum) {
  out(`   pass D: playing the film in real time twice (${fmtT(total)}s each) — plain, then under recordVideo`);
  let plain, rec;
  try {
    plain = await runReal(browser, file, total, false);
    rec = await runReal(browser, file, total, true);
  } catch (e) {
    fail('wall-clock', `pass D could not run — reported as NOT RUN, never implied: ${String(e && e.message || e).split('\n')[0]}` +
      ` (recordVideo needs Playwright's own ffmpeg: cd ${TOOL} && npx playwright install ffmpeg)`);
    return;
  }
  for (const [label, r] of [['plain', plain], ['recorded', rec]]) {
    if (!(r.done && r.reason === 'complete')) { fail('wall-clock', `${label} run: done=${r.done} reason=${r.reason} ${r.error || ''}`); continue; }
    const marks = Array.isArray(r.syncMarks) ? r.syncMarks : [];
    if (marks.length !== nScenes) { fail('wall-clock', `${label} run logged ${marks.length} [sync] boundary mark(s) for ${nScenes} scene(s)`); continue; }
    const bad = marks.filter((m) => Math.abs(m.drift) > Math.max(0.02 * m.timeline, SYNC_FLOOR_S));
    if (bad.length) fail('wall-clock', `${label} run: boundary drift beyond max(2 % of elapsed, ${SYNC_FLOOR_S}s): ${bad.map((m) => `scene ${m.scene} drift=${m.drift}s at ${m.timeline}s`).join('; ')}`);
    else ok('wall-clock', `${label} run: every boundary within max(2 % of elapsed, ${SYNC_FLOOR_S}s) — max |drift| ${Math.max(...marks.map((m) => Math.abs(m.drift))).toFixed(3)}s`);
  }
  if (Array.isArray(plain.syncMarks) && Array.isArray(rec.syncMarks) && plain.syncMarks.length === rec.syncMarks.length) {
    const gaps = rec.syncMarks.map((m, i) => ({ scene: m.scene, gap: Math.abs(m.wall - plain.syncMarks[i].wall), timeline: m.timeline }));
    const slip = gaps.filter((g) => g.gap > Math.max(0.02 * g.timeline, SYNC_FLOOR_S));
    if (slip.length) fail('wall-clock', `load-proportional slip between plain and recorded runs: ${slip.map((g) => `scene ${g.scene} +${g.gap.toFixed(3)}s`).join('; ')}`);
    else ok('wall-clock', 'plain and recorded boundary timings agree');
  }
  if (rec.webm) {
    if (rec.webm.error) fail('webm', `could not measure the recorded webm: ${rec.webm.error} (is ffmpeg on PATH?)`);
    else {
      const lo = total + TAIL_S - RECORD_START_SLACK_S;
      const hi = (total + TAIL_S + RECORD_PAD_MAX_S) * 1.02;
      if (rec.webm.seconds >= lo && rec.webm.seconds <= hi) ok('webm', `recorded webm decodes to ${rec.webm.seconds.toFixed(2)}s, within [${lo.toFixed(2)}, ${hi.toFixed(2)}]s (sum(durations) + tail - ${RECORD_START_SLACK_S}s recorder start, + <=${RECORD_PAD_MAX_S}s pad, + 2 %)`);
      else fail('webm', `recorded webm decodes to ${rec.webm.seconds.toFixed(2)}s, outside [${lo.toFixed(2)}, ${hi.toFixed(2)}]s for a ${fmtT(total)}s film`);
    }
  }
}

// ----------------------------------------------------------------- a deck

async function checkDeck(browser, file, shotsDir, opts, sheetEntries) {
  const name = path.basename(file);
  const stem = name.replace(/\.html$/, '');
  const source = fs.readFileSync(file, 'utf8');
  const sink = newSink();
  const res = { name, slides: 0, shots: 0, ...sourceHashes(file, source) };
  out(`== deck ${name}`);
  const probe = await browser.newPage({ viewport: DECK_VIEWPORT, deviceScaleFactor: 1 });
  instrument(probe, sink);
  await probe.goto(pathToFileURL(file).href);
  res.slides = await probe.evaluate(() => document.querySelectorAll('.slide').length);
  await probe.close();
  ok('deck', `${res.slides} slide(s)`);
  for (let i = 0; i < res.slides; i++) {
    const page = await browser.newPage({ viewport: DECK_VIEWPORT, deviceScaleFactor: 1 });
    instrument(page, sink);
    // The same holds as a film frame: JS time on the fake clock (a deck gets
    // DECK_JS_MS of it, run timer by timer, then held), SMIL and <video> held
    // at that time, every build stepped to its end.
    await holdClock(page);
    await page.goto(pathToFileURL(file).href + '#' + (i + 1));
    await page.clock.runFor(DECK_JS_MS);
    const info = await page.evaluate(async (args) => {
      const { idx, heldMs } = args;
      await document.fonts.ready;
      const slides = Array.prototype.slice.call(document.querySelectorAll('.slide'));
      if (!slides[idx].classList.contains('active')) slides.forEach((s, j) => s.classList.toggle('active', j === idx));
      // Every build stepped to its end: a slide tile shows the slide as
      // presented. An endless animation has no end: it is held at the same
      // JS time the page was given, so the tile is reproducible.
      for (const a of document.getAnimations()) {
        const end = a.effect ? a.effect.getComputedTiming().endTime : 0;
        try { a.pause(); a.currentTime = Number.isFinite(end) ? end : heldMs; } catch (e) { /* ignore */ }
      }
      void document.body.offsetHeight;
      const s = slides[idx];
      const h = s.querySelector('h1, h2, h3');
      return { layout: s.getAttribute('data-layout'), title: h ? h.textContent.replace(/\s+/g, ' ').trim().slice(0, 90) : (s.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 90) };
    }, { idx: i, heldMs: DECK_JS_MS });
    await holdMedia(page, DECK_JS_MS / 1000);
    if (opts.contactSheet) {
      const fileName = `${stem}-slide-${String(i + 1).padStart(2, '0')}.png`;
      await shoot(page, path.join(shotsDir, fileName), DECK_VIEWPORT);
      res.shots++;
      sheetEntries.push({ artifact: name, surface: `slide ${i + 1} of ${res.slides}${info.layout ? ' (' + info.layout + ')' : ''}`, frozen: 'every build at its end', shows: info.title ? `"${info.title}"` : '(no text)', file: fileName });
    }
    await page.close();
  }
  if (sink.errors.length) fail('deck', `${name}: page/console error(s): ${[...new Set(sink.errors)].slice(0, 5).join(' | ')}`);
  if (sink.offline.length) fail('deck', `${name}: off-file:// request(s): ${[...new Set(sink.offline)].slice(0, 5).join(', ')}`);
  return res;
}

// --------------------------------------------------------- the contact sheet

function sheetGeometry(n) {
  const cols = Math.max(1, Math.min(MAX_COLS, n));
  const rows = Math.max(1, Math.ceil(n / cols));
  return {
    cols, rows,
    width: cols * TILE.w + (cols - 1) * TILE.padding + 2 * TILE.margin,
    height: rows * TILE.h + (rows - 1) * TILE.padding + 2 * TILE.margin,
  };
}

function buildSheet(shotsDir, entries, artifacts, counts, midFrames) {
  const n = entries.length;
  if (!n) { fail('contact-sheet', 'no surface was frozen this run, so there is nothing to tile'); return; }
  const g = sheetGeometry(n);
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'render-check-tiles-'));
  try {
    // Normalise every tile to one size FIRST, one ffmpeg call per tile: an
    // image2 sequence whose frames change size mid-stream (a 1920x1080 film
    // frame next to a 1280x720 deck slide) silently drops tiles in the montage.
    const fit = `scale=${TILE.w}:${TILE.h}:force_original_aspect_ratio=decrease,` +
      `pad=${TILE.w}:${TILE.h}:(ow-iw)/2:(oh-ih)/2:color=black`;
    entries.forEach((e, i) => {
      execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-i', path.join(shotsDir, e.file),
        '-vf', fit, '-frames:v', '1', path.join(tmp, String(i + 1).padStart(4, '0') + '.png')], { stdio: ['ignore', 'ignore', 'pipe'] });
    });
    const vf = `tile=${g.cols}x${g.rows}:margin=${TILE.margin}:padding=${TILE.padding}:color=0x333333`;
    execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-framerate', '1', '-i', path.join(tmp, '%04d.png'),
      '-vf', vf, '-frames:v', '1', '-update', '1', path.join(shotsDir, 'contact.png')], { stdio: ['ignore', 'ignore', 'pipe'] });
  } catch (e) {
    fail('contact-sheet', `ffmpeg could not build the montage: ${String(e.stderr || e.message).trim().split('\n')[0]}`);
    return;
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
  const lines = [
    `# Contact sheet — ${path.basename(path.dirname(shotsDir))}`,
    '',
    `Generated ${new Date().toISOString()} by \`tools/render-check.mjs --contact-sheet\`.`,
    `Sheet: \`shots/contact.png\` — ${n} tile(s) in a ${g.cols}x${g.rows} grid, row-major (tile 1 top-left).`,
    `Expected tiles: ${counts.scenes} scene(s) + ${counts.slides} slide(s) = ${counts.scenes + counts.slides}. Every surface, never a sample.`,
    'Frames and this sheet are build outputs (git-ignored); regenerate, never commit.',
    '',
    '| # | artifact | surface | frozen at | should show | frame |',
    '|---|---|---|---|---|---|',
    ...entries.map((e, i) => `| ${i + 1} | ${mdCell(e.artifact)} | ${mdCell(e.surface)} | ${mdCell(e.frozen)} | ${mdCell(e.shows)} | ${mdCell(e.file)} |`),
    '',
    '## Mid-animation frames (not tiles)',
    '',
    '| frame | frozen at | moving at capture |',
    '|---|---|---|',
    ...midFrames.map((m) => `| ${mdCell(m.file)} | t=${fmtT(m.t)}s | ${mdCell(m.moving.join(', '))} |`),
    '',
    '## The look-pass',
    '',
    'Open the sheet, then the mid-animation frames, and read every frame as a viewer.',
    'Record what the looking found in NOTES, naming this sheet by path. A NOTES entry',
    'claiming a look-pass without a sheet path is not a pass.',
    '',
  ];
  fs.writeFileSync(path.join(shotsDir, 'INDEX.md'), lines.join('\n'));
  const manifest = {
    version: 1,
    generated: new Date().toISOString(),
    expected: counts.scenes + counts.slides,
    scenes: counts.scenes,
    slides: counts.slides,
    tiles: n,
    grid: g,
    tile: TILE,
    sheet: 'contact.png',
    index: 'INDEX.md',
    artifacts,
    entries: entries.map((e, i) => ({ n: i + 1, ...e })),
    midFrames,
  };
  fs.writeFileSync(path.join(shotsDir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
  ok('contact-sheet', `wrote ${path.join(shotsDir, 'contact.png')} (${n} tiles), INDEX.md, manifest.json`);
}

// INDEX.md's tile table only: the rows under its `| # | artifact |` header, up
// to the first non-table line. The mid-animation table below it (whose frame
// names can start with a digit) is never counted.
function countTileRows(text) {
  const lines = text.split('\n');
  const h = lines.findIndex((l) => /^\|\s*#\s*\|\s*artifact\s*\|/.test(l));
  if (h < 0) return 0;
  let n = 0;
  for (let i = h + 2; i < lines.length && lines[i].startsWith('|'); i++) if (/^\|\s*\d+\s*\|/.test(lines[i])) n++;
  return n;
}

// The sheet as a GATE CONDITION: absent, short, stale or unindexed fails.
function checkSheet(shotsDir, counts, artifacts) {
  const sheet = path.join(shotsDir, 'contact.png');
  const index = path.join(shotsDir, 'INDEX.md');
  const mf = path.join(shotsDir, 'manifest.json');
  const expected = counts.scenes + counts.slides;
  if (!fs.existsSync(sheet)) {
    fail('contact-sheet', `missing ${sheet} — no contact sheet, no look-pass: run with --contact-sheet, then look at it. An unlooked-at run cannot report a pass.`);
    return false;
  }
  if (!fs.existsSync(index) || !fs.existsSync(mf)) {
    fail('contact-sheet', `${sheet} exists but ${fs.existsSync(index) ? '' : 'INDEX.md '}${fs.existsSync(mf) ? '' : 'manifest.json '}is missing — re-run --contact-sheet`);
    return false;
  }
  let manifest;
  try { manifest = JSON.parse(fs.readFileSync(mf, 'utf8')); } catch (e) { fail('contact-sheet', `manifest.json unreadable: ${e.message}`); return false; }
  let good = true;
  const tileRows = countTileRows(fs.readFileSync(index, 'utf8'));
  if (manifest.expected !== expected || manifest.tiles !== expected || tileRows !== expected) {
    fail('contact-sheet', `tile count does not cover every surface: this run measured ${counts.scenes} scene(s) + ${counts.slides} slide(s) = ${expected}; the sheet has ${manifest.tiles} tile(s), INDEX.md ${tileRows} row(s), manifest.expected ${manifest.expected}`);
    good = false;
  }
  const size = pngSize(fs.readFileSync(sheet));
  const g = sheetGeometry(expected);
  if (!size || size.width !== g.width || size.height !== g.height) {
    fail('contact-sheet', `contact.png is ${size ? size.width + 'x' + size.height : 'not a PNG'}, expected ${g.width}x${g.height} for ${expected} tile(s) in ${g.cols}x${g.rows}`);
    good = false;
  }
  if (size && size.width === g.width && size.height === g.height) {
    // The grid's size proves nothing about its contents: decode the sheet and
    // require every one of the N tile slots to hold pixels, not the empty-slot
    // fill (0x333333) the montage leaves where a tile was dropped.
    const raw = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-i', sheet, '-f', 'rawvideo', '-pix_fmt', 'rgb24', '-'], { maxBuffer: 1 << 28 });
    if (raw.error || raw.status !== 0 || raw.stdout.length !== g.width * g.height * 3) {
      fail('contact-sheet', `could not decode ${sheet} to check its tiles: ${raw.error ? raw.error.message : String(raw.stderr).trim().split('\n')[0]}`);
      good = false;
    } else {
      const empty = [];
      for (let i = 0; i < expected; i++) {
        const x0 = TILE.margin + (i % g.cols) * (TILE.w + TILE.padding);
        const y0 = TILE.margin + Math.floor(i / g.cols) * (TILE.h + TILE.padding);
        let filled = false;
        for (let y = y0; y < y0 + TILE.h && !filled; y += 3) {
          for (let x = x0; x < x0 + TILE.w; x += 3) {
            const o = (y * g.width + x) * 3;
            if (raw.stdout[o] !== 0x33 || raw.stdout[o + 1] !== 0x33 || raw.stdout[o + 2] !== 0x33) { filled = true; break; }
          }
        }
        if (!filled) empty.push(i + 1);
      }
      if (empty.length) { fail('contact-sheet', `tile slot(s) ${empty.join(', ')} of ${expected} are empty in contact.png — the montage dropped them`); good = false; }
    }
  }
  const missing = (manifest.entries || []).filter((e) => !fs.existsSync(path.join(shotsDir, e.file)));
  if (missing.length) { fail('contact-sheet', `tile frame(s) missing: ${missing.map((e) => e.file).join(', ')}`); good = false; }
  for (const a of artifacts) {
    const m = (manifest.artifacts || []).find((x) => x.name === a.name);
    if (!m || m.digest !== a.digest) {
      const changed = [];
      if (!m || m.sha256 !== a.sha256) changed.push(a.name);
      const before = new Map(((m && m.sources) || []).map((x) => [x.ref, x.sha256]));
      for (const x of a.sources) if (!before.has(x.ref) || before.get(x.ref) !== x.sha256) changed.push(x.ref);
      for (const ref of before.keys()) if (!a.sources.some((x) => x.ref === ref)) changed.push(ref + ' (no longer referenced)');
      fail('contact-sheet', `stale: ${a.name} or a file it references changed since the sheet was made (${changed.join(', ') || 'no source digest in manifest.json'}) — the sheet is a look at a different build; re-run --contact-sheet`);
      good = false;
    }
  }
  if (good) ok('contact-sheet', `${sheet}: ${expected} tile(s) == ${counts.scenes} scene(s) + ${counts.slides} slide(s), indexed, current with the sources`);
  return good;
}

// --------------------------------------------------- --dump-animation-times

async function dumpAnimationTimes(browser, films, t, freeze) {
  let allGood = true;
  for (const file of films) {
    const name = path.basename(file);
    let rows;
    if (freeze) {
      let f;
      try {
        f = await freezeAt(browser, file, t, newSink());
      } catch (e) {
        if (!(e instanceof NotHousePlayer)) throw e;
        fail('animation-times', e.message);
        allGood = false;
        continue;
      }
      rows = f.rows;
      await f.page.close();
      out(`animation-times ${name} @ t=${fmtT(t)}s — FROZEN (?at= seek, JS clock advanced to t and held, getAnimations() walk, SMIL/<video> held, read back)`);
      out(`  held: ${describeHeld(f.held)}`);
    } else {
      // The negative control. Hold the page's own clock at load, do NOT seek,
      // do NOT walk the timeline: the state a harness that advanced time
      // instead of driving the timeline read.
      const page = await browser.newPage({ viewport: FILM_VIEWPORT, deviceScaleFactor: 1 });
      await page.clock.install();
      await page.clock.pauseAt(new Date(Date.now() + 1000));
      await page.goto(pathToFileURL(file).href);
      await page.waitForFunction(() => window.__playerReady === true, undefined, { timeout: 10000 });
      rows = await page.evaluate(pageAnimations, { t, drive: false });
      const back = await page.evaluate(pageReadBack);
      rows.forEach((r, i) => Object.assign(r, back[i] || {}));
      await page.close();
      out(`animation-times ${name} @ t=${fmtT(t)}s — NO FREEZE (negative control: page clock held at load, no ?at= seek, no getAnimations() walk)`);
    }
    if (!rows.length) { fail('animation-times', `${name}: document.getAnimations() is empty — nothing to measure`); allGood = false; continue; }
    const off = [];
    for (const r of rows) {
      const ct = r.currentTime == null ? NaN : r.currentTime;
      const good = Math.abs(ct - r.expected) <= 1;
      if (!good) off.push(r);
      const seek = freeze ? ` after-seek=${r.before == null ? 'null' : r.before.toFixed(1)}ms` : '';
      out(`  #${r.i} ${r.type} ${r.target} scene ${r.scene} ${r.kind} start=${fmtT(r.start)}s${seek} currentTime=${Number.isFinite(ct) ? ct.toFixed(1) : 'null'}ms expected=${r.expected.toFixed(1)}ms ${r.playState}${good ? '' : '  <-- NOT AT ITS OFFSET'}`);
    }
    const zero = rows.filter((r) => !r.currentTime).length;
    if (off.length) {
      allGood = false;
      fail('animation-times', `${name}: ${off.length} of ${rows.length} animation(s) are not at their t=${fmtT(t)}s offset` +
        (zero === rows.length ? ` — every currentTime is 0: the scene-ENTRY state, which a harness that does not drive the timeline mistakes for t=${fmtT(t)}s` : ''));
    } else {
      const nonzero = rows.filter((r) => r.currentTime > 0).length;
      ok('animation-times', `${name}: ${rows.length} of ${rows.length} animation(s) at their t=${fmtT(t)}s offset (${nonzero} non-zero currentTime)`);
    }
  }
  return allGood;
}

// -------------------------------------------------------------------- main

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  const { dir, films, decks } = discover(opts.target);
  if (!films.length && !decks.length) usage(`no film (*.html with data-scene) or deck (*.html with .slide sections) at the top level of ${dir}`);
  const { chromium } = await loadPlaywright();
  const browser = await launch(chromium);
  try {
    if (opts.dump) {
      if (!films.length) usage('--dump-animation-times needs a film');
      await dumpAnimationTimes(browser, films, opts.at, opts.freeze);
      out(failures.length ? `FAIL (${failures.length} failure(s))` : 'PASS');
      return failures.length ? 1 : 0;
    }

    const shotsDir = path.join(dir, 'shots');
    fs.mkdirSync(shotsDir, { recursive: true });
    out(`render-check ${dir} (${opts.contactSheet ? 'building the contact sheet, then the gate' : 'the gate'})`);
    const sheetEntries = [];
    const filmRes = [];
    const deckRes = [];
    for (const f of films) filmRes.push(await checkFilm(browser, f, shotsDir, opts, sheetEntries));
    for (const d of decks) deckRes.push(await checkDeck(browser, d, shotsDir, opts, sheetEntries));

    const counts = {
      scenes: filmRes.reduce((a, r) => a + r.scenes, 0),
      slides: deckRes.reduce((a, r) => a + r.slides, 0),
    };
    const artifacts = [...filmRes.map((r) => ({ name: r.name, kind: 'film', surfaces: r.scenes, sha256: r.sha256, sources: r.sources, digest: r.digest })),
      ...deckRes.map((r) => ({ name: r.name, kind: 'deck', surfaces: r.slides, sha256: r.sha256, sources: r.sources, digest: r.digest }))];
    const midFrames = filmRes.flatMap((r) => r.midFrames);
    const shots = filmRes.reduce((a, r) => a + r.shots, 0) + deckRes.reduce((a, r) => a + r.shots, 0);

    out('== contact sheet');
    if (opts.contactSheet) buildSheet(shotsDir, sheetEntries, artifacts, counts, midFrames);
    checkSheet(shotsDir, counts, artifacts);
    if (films.length) {
      if (midFrames.length >= MIN_MID_FRAMES) ok('shots', `${midFrames.length} mid-animation frame(s) >= ${MIN_MID_FRAMES}`);
      else fail('shots', `only ${midFrames.length} frame(s) measured mid-animation this run, fewer than ${MIN_MID_FRAMES} — the look-pass needs frames caught moving, not entry states`);
    }

    out('');
    out(`SUMMARY: ${films.length} film(s), ${counts.scenes} scene(s); ${decks.length} deck(s), ${counts.slides} slide(s); ` +
      `${shots} shot(s) taken this run, ${midFrames.length} mid-animation; contact sheet: ${path.join(shotsDir, 'contact.png')}`);
    if (notices.length) out(`NOT CHECKED: ${notices.length} item(s) — see the NOTICE lines; they are not passes`);
    out(`LOOK: open ${path.join(shotsDir, 'contact.png')} and the mid-animation frames; record what the looking found in NOTES, naming the sheet path.`);
    out(failures.length ? `FAIL (${failures.length} failure(s))` : 'PASS');
    fs.writeFileSync(path.join(shotsDir, 'render-check.txt'), transcript.join('\n') + '\n');
    return failures.length ? 1 : 0;
  } finally {
    await browser.close();
  }
}

main().then((code) => process.exit(code), (e) => {
  console.log('FAIL: render-check raised: ' + (e && e.stack || e));
  process.exit(1);
});
