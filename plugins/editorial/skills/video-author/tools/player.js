/*!
 * house-player — the house timeline/scene engine (presentation-skills/02).
 *
 * Inline this dependency-free clock into a local preview. See references/driver-pattern.md.
 *
 * Contract (full detail in README.md beside this file):
 *   - window.SCENE_DURATIONS: declared EMPTY exactly once by the page; the
 *     pipeline's regex rewrites that one assignment with measured seconds.
 *     Non-empty -> use it (injected). Empty -> use window.FALLBACK (the
 *     authored per-scene estimates in seconds), with a console warning.
 *   - window.__player = {scene, t, total, durations, seek(t), play(), pause()}
 *       scene:  1-based index of the scene the clock is currently inside.
 *       t:      current elapsed time, in SECONDS, from film start.
 *       total:  the film's total length, in SECONDS (== sum(durations)).
 *       seek(t): jumps to an absolute time in SECONDS and PAUSES.
 *   - window.__playerReady / window.__renderReady: true once boot finishes.
 *   - window.__explainer = {durations, total, scenes, failure, syncMarks}.
 *   - `?scene=N` (1-based) starts play at that scene's first frame.
 *     `?at=<sec>` seeks to an absolute time and pauses — the frame is
 *     deterministic: every `document.getAnimations()` entry the engine
 *     tracks is driven to the matching `currentTime` and paused, never left
 *     to the browser's own live ticker.
 *   - document.body.dataset.done="1" / dataset.doneReason=complete|error|watchdog.
 *   - The take-sanity gate (video-author SKILL.md §8b): on the INJECTED path
 *     only, a scene whose measured duration exceeds
 *     max(authored*1.8, authored+8), or a total exceeding authored total*1.3,
 *     is a poisoned measurement, not a long film — fail fast (`doneReason
 *     =error`, naming the scene) rather than holding the frame for minutes.
 *   - `prefers-reduced-motion: reduce`: every beat snaps straight to its
 *     final frame; the film still runs its full authored schedule and still
 *     reaches done=1 reason=complete. The engine ships this — a film cannot
 *     omit it by forgetting.
 *
 * Global: window.HousePlayer = { boot(opts) }
 */
(function (global) {
  'use strict';

  var TAIL = 0.5;        // hold on the final frame before the done signal (s)
  var WATCHDOG_PAD = 5;  // force doneReason=watchdog this long after the expected end (s)

  function toArray(nodeList) { return Array.prototype.slice.call(nodeList); }

  function boot(opts) {
    opts = opts || {};

    var state = {
      playing: false,
      done: false,
      t: 0,
      tBase: 0,
      tStart: 0,
      scene: 1,
      tailScheduled: false,
    };
    var scenes = [];
    var beats = [];
    var durations = [];
    var cumStart = [];
    var total = 0;
    var injected = false;
    var reducedMotion = false;
    var watchdogTimer = null;
    var lastSceneIdx = -1;

    function finish(reason, msg) {
      if (state.done) return;
      state.done = true;
      state.playing = false;
      if (watchdogTimer) { clearTimeout(watchdogTimer); watchdogTimer = null; }
      if (msg) document.body.dataset.error = msg;
      document.body.dataset.doneReason = reason;
      document.body.dataset.done = '1';
      if (global.__explainer) global.__explainer.failure = (reason === 'error') ? (msg || reason) : null;
    }

    function injectReducedMotionStyle() {
      var style = document.createElement('style');
      style.setAttribute('data-house-player', 'reduced-motion');
      // Belt-and-suspenders: an actual @media block ships on the page (so a
      // film cannot omit the degradation by forgetting it), AND the JS side
      // below independently snaps every beat to its final frame when the
      // media query matches — the CSS override is not the only enforcement.
      style.textContent =
        '@media (prefers-reduced-motion: reduce) {\n' +
        '  [data-beat] { animation-duration: 0.001ms !important; transition-duration: 0.001ms !important; }\n' +
        '}';
      document.head.appendChild(style);
    }

    function sceneIndexAt(t) {
      for (var i = cumStart.length - 1; i >= 0; i--) {
        if (t >= cumStart[i] - 1e-9) return i;
      }
      return 0;
    }

    function updatePlayerObject() {
      if (!global.__player) return;
      global.__player.scene = state.scene;
      global.__player.t = state.t;
    }

    function setVisual(t) {
      var idx = sceneIndexAt(t);
      for (var i = 0; i < scenes.length; i++) {
        scenes[i].classList.toggle('on', i === idx);
      }
      for (var i = 0; i < beats.length; i++) {
        var b = beats[i];
        var local;
        if (reducedMotion) {
          local = (t >= b.absStart) ? b.duration : 0;
        } else {
          local = Math.min(Math.max(t - b.absStart, 0), b.duration);
        }
        if (b.anim) {
          try {
            b.anim.currentTime = local * 1000;
            if (typeof b.anim.pause === 'function') b.anim.pause();
          } catch (e) { /* an animation that finished/cancelled itself — ignore */ }
        }
      }
      state.scene = idx + 1;
      state.t = t;
      updatePlayerObject();
    }

    function clockNow() {
      if (!state.playing) return state.t;
      return state.tBase + (performance.now() - state.tStart) / 1000;
    }

    function logSyncMark(idx, t) {
      var expected = cumStart[idx];
      var mark = {
        scene: idx + 1,
        timeline: Number(expected.toFixed(3)),
        wall: Number(t.toFixed(3)),
        drift: Number((t - expected).toFixed(3)),
      };
      if (global.__explainer) global.__explainer.syncMarks.push(mark);
      console.log('[sync] scene ' + mark.scene + ' timeline=' + mark.timeline +
        's wall=' + mark.wall + 's drift=' + mark.drift + 's');
    }

    function frame() {
      if (!state.playing) return;
      var t = clockNow();
      if (t >= total) {
        setVisual(total);
        if (!state.tailScheduled) {
          state.tailScheduled = true;
          setTimeout(function () { finish('complete'); }, TAIL * 1000);
        }
        return;
      }
      setVisual(t);
      var idx = sceneIndexAt(t);
      if (idx !== lastSceneIdx) {
        logSyncMark(idx, t);
        lastSceneIdx = idx;
      }
      requestAnimationFrame(frame);
    }

    function play() {
      if (state.done) return;
      state.playing = true;
      state.tBase = state.t;
      state.tStart = performance.now();
      setVisual(state.t);
      requestAnimationFrame(frame);
    }

    function pause() {
      if (!state.playing) return;
      state.t = clockNow();
      state.playing = false;
      setVisual(state.t);
    }

    function seek(t) {
      t = Math.min(Math.max(Number(t) || 0, 0), total);
      state.playing = false;
      state.t = t;
      lastSceneIdx = sceneIndexAt(t);
      setVisual(t);
    }

    function prepareBeats() {
      var list = [];
      for (var s = 0; s < scenes.length; s++) {
        var els = toArray(scenes[s].querySelectorAll('[data-at]'));
        for (var i = 0; i < els.length; i++) {
          var el = els[i];
          var at = parseFloat(el.getAttribute('data-at'));
          if (!isFinite(at)) at = 0;
          var dur = parseFloat(el.getAttribute('data-dur'));
          if (!isFinite(dur) || dur <= 0) dur = 0.6;
          var kind = el.getAttribute('data-beat') || 'waapi';
          var anim = null;
          if (kind === 'css') {
            // The animation already exists: it is declared in CSS with
            // animation-play-state:paused, so it is created idle/paused at
            // currentTime 0 the moment the element is in the document —
            // document.getAnimations() finds it without any JS trigger.
            var existing = (typeof el.getAnimations === 'function') ? el.getAnimations() : [];
            anim = existing[0] || null;
          } else {
            var kfAttr = el.getAttribute('data-waapi');
            var kf = null;
            if (kfAttr) { try { kf = JSON.parse(kfAttr); } catch (e) { kf = null; } }
            if (!kf) kf = [{ opacity: 0, transform: 'translateY(10px)' }, { opacity: 1, transform: 'translateY(0)' }];
            if (typeof el.animate === 'function') {
              anim = el.animate(kf, { duration: dur * 1000, fill: 'forwards', easing: 'linear' });
            }
          }
          if (anim) {
            anim.pause();
            anim.currentTime = 0;
            list.push({ el: el, absStart: cumStart[s] + at, duration: dur, anim: anim });
          }
        }
      }
      return list;
    }

    // On the INJECTED path only: a duration far past what natural TTS pacing
    // drift produces (~1.0-1.4x an estimate) is a poisoned measurement, not a
    // long film. Fail fast, before playback, naming the scene.
    function takeSanityFailure() {
      if (!injected) return null;
      var fb = (opts.fallback || global.FALLBACK || []).map(Number);
      if (fb.length && fb.length === durations.length) {
        for (var i = 0; i < durations.length; i++) {
          var ceiling = Math.max(fb[i] * 1.8, fb[i] + 8);
          if (durations[i] > ceiling) {
            return 'scene ' + (i + 1) + ' measured ' + durations[i] + 's exceeds the take-sanity ceiling ' +
              ceiling.toFixed(2) + 's (authored ' + fb[i] + 's) — treated as a poisoned measurement, not a long take';
          }
        }
      }
      if (fb.length) {
        var fbTotal = fb.reduce(function (a, b) { return a + (Number(b) || 0); }, 0);
        if (fbTotal > 0 && total > fbTotal * 1.3) {
          return 'total measured ' + total.toFixed(2) + 's exceeds the take-sanity ceiling ' +
            (fbTotal * 1.3).toFixed(2) + 's (authored total ' + fbTotal.toFixed(2) + 's)';
        }
      }
      return null;
    }

    try {
      scenes = toArray(document.querySelectorAll('.scene[data-scene]'));
      global.__explainer = { durations: [], total: 0, scenes: 0, failure: null, syncMarks: [] };

      if (!scenes.length) {
        global.__player = { scene: 1, t: 0, total: 0, durations: [], seek: seek, play: play, pause: pause };
        global.__playerReady = true;
        global.__renderReady = true;
        finish('error', 'no scenes');
        return;
      }

      reducedMotion = !!(global.matchMedia && global.matchMedia('(prefers-reduced-motion: reduce)').matches);
      injectReducedMotionStyle();

      var fallback = (opts.fallback || global.FALLBACK || []).map(Number);
      var raw = Array.isArray(global.SCENE_DURATIONS) ? global.SCENE_DURATIONS : [];
      injected = raw.length > 0;
      durations = (injected ? raw : fallback).map(function (v) { return Number(v) || 0; });

      if (!injected) {
        console.warn('[house-player] window.SCENE_DURATIONS is empty — using authored FALLBACK estimates: ' +
          JSON.stringify(fallback));
      }
      if (durations.length !== scenes.length) {
        console.warn('[house-player] ' + durations.length + ' durations vs ' + scenes.length +
          ' scenes — length mismatch');
      }

      cumStart = [];
      var acc = 0;
      for (var i = 0; i < scenes.length; i++) { cumStart.push(acc); acc += (durations[i] || 0); }
      total = acc;

      global.__explainer = { durations: durations.slice(), total: total, scenes: scenes.length, failure: null, syncMarks: [] };
      global.__player = {
        scene: 1,
        t: 0,
        total: total,
        durations: durations.slice(),
        seek: seek,
        play: play,
        pause: pause,
      };
      global.__playerReady = true;
      global.__renderReady = true;

      var poisonMsg = takeSanityFailure();
      if (poisonMsg) { finish('error', poisonMsg); return; }

      beats = prepareBeats();
      lastSceneIdx = -1;

      watchdogTimer = setTimeout(function () { finish('watchdog'); }, (total + TAIL + WATCHDOG_PAD) * 1000);

      var params = new URLSearchParams(global.location.search);
      if (params.has('at')) {
        // A `?at=` load is a DELIBERATE held frame for inspection (the render
        // harness, presentation-skills/05, freezes frames to screenshot them).
        // seek() pauses the clock, so the film never reaches `complete` on its
        // own — but the watchdog armed above stays live and would flip
        // doneReason to "watchdog" once total+TAIL+WATCHDOG_PAD elapsed on a
        // slow capture. A frozen frame is not a hung film: disarm the watchdog
        // on this path so a slow inspection can never spuriously read watchdog.
        if (watchdogTimer) { clearTimeout(watchdogTimer); watchdogTimer = null; }
        var at = parseFloat(params.get('at'));
        seek(isFinite(at) ? at : 0);
      } else if (params.has('scene')) {
        var n = parseInt(params.get('scene'), 10);
        if (!isFinite(n)) n = 1;
        var idx = Math.min(Math.max(n - 1, 0), scenes.length - 1);
        state.t = cumStart[idx];
        play();
      } else {
        state.t = 0;
        play();
      }
    } catch (e) {
      console.error('[house-player] boot failed: ' + (e && e.message || e));
      finish('error', String(e && e.message || e));
    }
  }

  global.HousePlayer = { boot: boot, VERSION: '1.0.0' };
})(window);
