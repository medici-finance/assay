#!/usr/bin/env node
"use strict";
// validate_palette.js — the dataviz CVD gate for the Console's lifecycle chart
// (design §7.6, decision-model spec §5.5). It is the executable form of the
// requirement that put the single-hue ramp into the build: the wireframes' five-
// hue lifecycle bar FAILED colour-vision-deficiency validation
// (#3366FF in-progress vs #7a5cff implemented are protan-indistinguishable at
// ΔE 0.4 on adjacent segments), so the build ships a single-hue SEQUENTIAL ramp
// instead — lifecycle is a progression, not five identities.
//
// This script reads the palette from web/static/palette.json (the same file the
// CSS ramp is generated from, so the check can never drift from what ships) and
// asserts, on the chosen surface:
//
//   1. adjacent-segment separation: every neighbouring pair in the ramp stays
//      distinguishable (ΔE76 >= MIN_ADJACENT) under normal vision AND under
//      simulated protanopia, deuteranopia, and tritanopia. A single-hue
//      LIGHTNESS ramp survives this because CVD distorts hue/chroma, not
//      lightness; a five-hue ramp collapses.
//   2. surface separation: every segment stays visible against the surface
//      (ΔE76 >= MIN_SURFACE), also under all three CVD sims.
//
// Usage:
//   node validate_palette.js                 # lifecycle_ramp on the dark surface (the shipped default)
//   node validate_palette.js --surface light
//   node validate_palette.js --set frames_5hue   # the rejected set — demonstrates the FAIL this gate exists for
//   node validate_palette.js --json          # machine-readable report
//
// Exit 0 = PASS (every pair clears every threshold under every vision model);
// non-zero = FAIL, with the offending pair(s) named.

const fs = require("fs");
const path = require("path");

const MIN_ADJACENT = 8.0; // ΔE76 floor between neighbouring lifecycle segments
const MIN_SURFACE = 8.0; // ΔE76 floor between any segment and the surface behind it

// ---- CLI -------------------------------------------------------------------

function parseArgs(argv) {
  const a = { surface: "dark", set: "lifecycle_ramp", json: false, palette: null };
  for (let i = 0; i < argv.length; i++) {
    const t = argv[i];
    if (t === "--surface") a.surface = argv[++i];
    else if (t === "--set") a.set = argv[++i];
    else if (t === "--json") a.json = true;
    else if (t === "--palette") a.palette = argv[++i];
    else if (t === "-h" || t === "--help") a.help = true;
  }
  return a;
}

// ---- colour maths ----------------------------------------------------------

function hexToRgb(hex) {
  const h = hex.replace("#", "").trim();
  const n =
    h.length === 3
      ? h
          .split("")
          .map((c) => c + c)
          .join("")
      : h;
  return [
    parseInt(n.slice(0, 2), 16),
    parseInt(n.slice(2, 4), 16),
    parseInt(n.slice(4, 6), 16),
  ];
}

const srgbToLinear = (c) => {
  const cs = c / 255;
  return cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
};
const linearToSrgb = (c) => {
  const v = c <= 0.0031308 ? c * 12.92 : 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
  return Math.max(0, Math.min(255, Math.round(v * 255)));
};

// Machado et al. (2009) severity-1.0 CVD matrices, applied in LINEAR RGB.
const CVD = {
  none: null,
  protan: [
    [0.152286, 1.052583, -0.204868],
    [0.114503, 0.786281, 0.099216],
    [-0.003882, -0.048116, 1.051998],
  ],
  deutan: [
    [0.367322, 0.860646, -0.227968],
    [0.28008, 0.672501, 0.047413],
    [-0.01182, 0.04294, 0.968881],
  ],
  tritan: [
    [1.255528, -0.076749, -0.178779],
    [-0.078411, 0.930809, 0.147602],
    [0.004733, 0.691367, 0.3039],
  ],
};

function simulate(rgb, kind) {
  const m = CVD[kind];
  if (!m) return rgb.slice();
  const [r, g, b] = rgb.map(srgbToLinear);
  const out = [
    m[0][0] * r + m[0][1] * g + m[0][2] * b,
    m[1][0] * r + m[1][1] * g + m[1][2] * b,
    m[2][0] * r + m[2][1] * g + m[2][2] * b,
  ];
  return out.map(linearToSrgb);
}

// sRGB -> CIE XYZ (D65) -> CIE Lab
function rgbToLab(rgb) {
  const [r, g, b] = rgb.map(srgbToLinear);
  let x = (r * 0.4124 + g * 0.3576 + b * 0.1805) / 0.95047;
  let y = r * 0.2126 + g * 0.7152 + b * 0.0722;
  let z = (r * 0.0193 + g * 0.1192 + b * 0.9505) / 1.08883;
  const f = (t) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116);
  x = f(x);
  y = f(y);
  z = f(z);
  return [116 * y - 16, 500 * (x - y), 200 * (y - z)];
}

function deltaE76(a, b) {
  const la = rgbToLab(a);
  const lb = rgbToLab(b);
  return Math.hypot(la[0] - lb[0], la[1] - lb[1], la[2] - lb[2]);
}

// ---- validation ------------------------------------------------------------

const VISIONS = ["none", "protan", "deutan", "tritan"];

function validate(colors, surfaceHex) {
  const failures = [];
  const surface = hexToRgb(surfaceHex);
  for (const vision of VISIONS) {
    const sim = colors.map((c) => simulate(hexToRgb(c), vision));
    const surfSim = simulate(surface, vision);
    // adjacent-segment separation
    for (let i = 0; i + 1 < sim.length; i++) {
      const de = deltaE76(sim[i], sim[i + 1]);
      if (de < MIN_ADJACENT) {
        failures.push({
          check: "adjacent",
          vision,
          a: colors[i],
          b: colors[i + 1],
          deltaE: +de.toFixed(2),
          floor: MIN_ADJACENT,
        });
      }
    }
    // surface separation
    for (let i = 0; i < sim.length; i++) {
      const de = deltaE76(sim[i], surfSim);
      if (de < MIN_SURFACE) {
        failures.push({
          check: "surface",
          vision,
          a: colors[i],
          b: surfaceHex,
          deltaE: +de.toFixed(2),
          floor: MIN_SURFACE,
        });
      }
    }
  }
  return failures;
}

// ---- main ------------------------------------------------------------------

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    console.log(
      "usage: node validate_palette.js [--surface dark|light] [--set lifecycle_ramp|frames_5hue] [--palette PATH] [--json]"
    );
    process.exit(2);
  }
  const palettePath =
    args.palette || path.join(__dirname, "..", "static", "palette.json");
  let data;
  try {
    data = JSON.parse(fs.readFileSync(palettePath, "utf8"));
  } catch (e) {
    console.error(`validate_palette: cannot read ${palettePath}: ${e.message}`);
    process.exit(2);
  }
  const set = data[args.set];
  if (!set || !Array.isArray(set.colors)) {
    console.error(
      `validate_palette: palette set ${JSON.stringify(args.set)} not found or has no .colors array`
    );
    process.exit(2);
  }
  const surfaceHex = (data.surface && data.surface[args.surface]) || "#16161a";
  const failures = validate(set.colors, surfaceHex);

  const report = {
    set: args.set,
    surface: args.surface,
    surface_hex: surfaceHex,
    colors: set.colors,
    visions: VISIONS,
    min_adjacent: MIN_ADJACENT,
    min_surface: MIN_SURFACE,
    pass: failures.length === 0,
    failures,
  };

  if (args.json) {
    console.log(JSON.stringify(report, null, 2));
  } else if (report.pass) {
    console.log(
      `PASS: ${args.set} (${set.colors.length} segments) is CVD-safe on the ${args.surface} surface ` +
        `— every adjacent pair and every segment-vs-surface pair clears ΔE ${MIN_ADJACENT} under normal, protan, deutan, and tritan vision.`
    );
  } else {
    console.log(
      `FAIL: ${args.set} has ${failures.length} indistinguishable pair(s) on the ${args.surface} surface:`
    );
    for (const f of failures) {
      console.log(
        `  [${f.vision}] ${f.check}: ${f.a} vs ${f.b} — ΔE ${f.deltaE} < ${f.floor}`
      );
    }
  }
  process.exit(report.pass ? 0 : 1);
}

main();
