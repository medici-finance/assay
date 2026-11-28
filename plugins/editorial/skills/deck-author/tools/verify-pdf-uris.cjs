#!/usr/bin/env node
/**
 * verify-pdf-uris.cjs — assert that a PDF we just wrote leaks NO disallowed link
 * target and carries NO identity metadata. Companion to verify-pdf-footer.cjs;
 * both are called by the render gate (`commitVerifiedPdf`) before a PDF is moved
 * into place. Also runnable directly to audit an existing PDF:
 *
 *   node tools/verify-pdf-uris.cjs <file.pdf>
 *   node tools/verify-pdf-uris.cjs <file.pdf> --allow extra.example.com,github.com/some-org
 *
 * ── Why this exists (issues #51 and #53) ──
 *
 * A PDF hyperlink is an `/Annots` → `/A` → `/URI` object. Two tools were used to
 * check "does this PDF point at anything internal?", and BOTH are blind:
 *
 *   • `pdftotext` extracts the visible ANCHOR TEXT and never the link TARGET
 *     (#51). A link whose anchor reads "the spike report" and whose target is a
 *     private URL is, to pdftotext, the string "the spike report". Every
 *     "internal pointers: 0" ever asserted from pdftotext was `could-not-check`
 *     reported as `checked-clean`.
 *
 *   • `strings` CAN see an uncompressed `/URI`, but returns ZERO for a `/URI`
 *     that sits inside a compressed object stream (#53) — a false clean
 *     indistinguishable from a genuinely clean file. The leak sweeps that "found
 *     nothing" were correct by LUCK: the renderers happened to emit uncompressed
 *     streams.
 *
 * `qpdf` reads the annotation dictionary directly and cannot be fooled by
 * compression; `mutool` fully decompresses every object stream so the targets
 * are recoverable by a raw scan too. This module uses BOTH — qpdf as the
 * authoritative enumerator, mutool as an independent decompress-and-corroborate
 * pass — and checks every target against an ALLOWLIST. Allowlist, not denylist:
 * a NEW internal link then fails the render closed instead of shipping silently.
 * `exiftool` asserts no `Author` (and no unexpected identity) metadata.
 *
 * ── Fail-closed is the whole point ──
 *
 * If qpdf / mutool / exiftool is not installed, the check does NOT skip and pass
 * — it FAILS. A missing tool is `could-not-check`, and `could-not-check` must
 * refuse the write, never launder into `clean`. That single rule is what turns a
 * grep-that-happened-to-return-zero into an actual gate.
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

/** A leak / metadata verification failure. Reported as a message, never a stack. */
class LeakError extends Error {}

const INSTALL_HINT =
  'install the PDF inspection tools:\n' +
  '    macOS:         brew install qpdf mupdf-tools exiftool\n' +
  '    Debian/Ubuntu: apt-get install qpdf mupdf-tools libimage-exiftool-perl';

/**
 * Default link-target allowlist. Every `/URI` in an outward-facing PDF must
 * resolve to one of these, or the render fails closed. Extend per-render with
 * the `allow` option or the `PDF_URI_ALLOWLIST` env var (comma-separated) — a
 * deck that legitimately links to a new public host adds it here explicitly,
 * which is the point: nothing ships unreviewed.
 *
 * Supply a host or a host/path prefix for each allowed destination.
 */
const DEFAULT_URI_ALLOWLIST = Object.freeze([]);

/** Run an inspection binary; turn "not installed" into a loud, fail-closed error. */
function tool(bin, args) {
  try {
    return execFileSync(bin, args, { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
  } catch (err) {
    if (err.code === 'ENOENT') {
      throw new LeakError(
        `${bin} not found — cannot inspect the PDF's link targets / metadata, so refusing\n` +
        `to write it. A leak check we could not run is not a leak check (could-not-check is\n` +
        `NOT clean).\n    ${INSTALL_HINT}`);
    }
    // qpdf exits non-zero on warnings but still emits JSON on stdout; only treat
    // a truly empty stdout as a failure.
    if (err.stdout && String(err.stdout).length > 0) return String(err.stdout);
    throw new LeakError(`${bin} failed on the rendered PDF: ${(err.stderr || err.message).toString().trim()}`);
  }
}

/** Decode a qpdf --json string scalar ("u:…" unicode, "b:…" hex) to a JS string. */
function decodeQpdfString(v) {
  if (typeof v !== 'string') return null;
  if (v.startsWith('u:')) return v.slice(2);
  if (v.startsWith('b:')) {
    const hex = v.slice(2).replace(/[^0-9a-fA-F]/g, '');
    try { return Buffer.from(hex, 'hex').toString('utf8'); } catch { return null; }
  }
  return v;
}

/** Recursively collect every `/URI` value out of a parsed qpdf JSON tree. */
function collectUris(node, out) {
  if (Array.isArray(node)) {
    for (const el of node) collectUris(el, out);
  } else if (node && typeof node === 'object') {
    for (const [k, v] of Object.entries(node)) {
      if (k === '/URI') {
        const s = decodeQpdfString(v);
        if (s) out.add(s.trim());
      } else {
        collectUris(v, out);
      }
    }
  }
}

/** Authoritative URI enumeration via qpdf's structured view of the annotations. */
function urisViaQpdf(pdfPath) {
  const raw = tool('qpdf', ['--json=2', pdfPath]);
  let doc;
  try { doc = JSON.parse(raw); }
  catch (err) { throw new LeakError(`qpdf produced unparseable JSON for ${pdfPath}: ${err.message}`); }
  const out = new Set();
  collectUris(doc, out);
  return out;
}

/**
 * Independent corroboration via mutool: fully decompress every object stream
 * (`clean -ggg`) so the `/URI` link-action targets are recoverable by a raw scan
 * of the expanded file — targets a compressed-stream `strings` would false-zero
 * on. This is the exact defense against the #53 root cause.
 *
 * Scoped to `/URI` link-action values ONLY (literal `(...)` and hex `<...>`
 * forms), not any bare URL anywhere in the file: a URL in the XMP metadata packet
 * or in body text is not a clickable link and must not be treated as one.
 */
function urisViaMutool(pdfPath) {
  const expanded = path.join(os.tmpdir(), `pdfuris-${process.pid}-${Date.now()}.pdf`);
  try {
    tool('mutool', ['clean', '-ggg', pdfPath, expanded]);
    const buf = fs.readFileSync(expanded, 'latin1');
    const out = new Set();
    for (const m of buf.matchAll(/\/URI\s*\(((?:\\\)|[^)])*)\)/g)) {
      out.add(m[1].replace(/\\([()\\])/g, '$1').trim());
    }
    for (const m of buf.matchAll(/\/URI\s*<([0-9a-fA-F\s]+)>/g)) {
      const hex = m[1].replace(/\s+/g, '');
      try { out.add(Buffer.from(hex, 'hex').toString('utf8').replace(/^﻿/, '').trim()); } catch { /* skip */ }
    }
    return out;
  } finally {
    fs.rmSync(expanded, { force: true });
  }
}

/** Parse the allowlist env var / option into a normalized array. */
function resolveAllowlist(extra) {
  const fromEnv = (process.env.PDF_URI_ALLOWLIST || '')
    .split(',').map((s) => s.trim()).filter(Boolean);
  const fromOpt = Array.isArray(extra) ? extra
    : (typeof extra === 'string' ? extra.split(',').map((s) => s.trim()).filter(Boolean) : []);
  return [...DEFAULT_URI_ALLOWLIST, ...fromEnv, ...fromOpt].map((s) => s.toLowerCase());
}

/** Does `host` match a bare-host allowlist entry (itself or a subdomain)? */
function hostMatches(host, entry) {
  return host === entry || host.endsWith('.' + entry);
}

/** Is a single URI target permitted by the allowlist? */
function uriAllowed(uri, allowlist) {
  let parsed;
  try { parsed = new URL(uri); } catch { return false; } // unparseable → fail closed
  const scheme = parsed.protocol.replace(/:$/, '').toLowerCase();

  if (scheme === 'http' || scheme === 'https') {
    const host = parsed.hostname.toLowerCase();
    const pathName = parsed.pathname || '/';
    for (const entry of allowlist) {
      const slash = entry.indexOf('/');
      if (slash === -1) {
        if (hostMatches(host, entry)) return true;            // bare-host entry
      } else {
        const eHost = entry.slice(0, slash);
        const ePath = entry.slice(slash);                     // begins with '/'
        if (host === eHost && (pathName === ePath || pathName.startsWith(ePath + '/'))) return true;
      }
    }
    return false;
  }

  if (scheme === 'mailto') {
    const at = uri.indexOf('@');
    if (at === -1) return false;
    const domain = uri.slice(at + 1).split(/[?]/)[0].toLowerCase();
    for (const entry of allowlist) {
      if (entry.indexOf('/') === -1 && hostMatches(domain, entry)) return true;
    }
    return false;
  }

  // tel:, ftp:, file:, javascript:, data:, … — unknown target class, fail closed.
  return false;
}

/**
 * Enumerate every link target in `pdfPath` (qpdf ∪ mutool) and split into
 * allowed / disallowed against the resolved allowlist.
 * @returns {{uris:string[], disallowed:string[], allowlist:string[]}}
 */
function checkPdfUris(pdfPath, extraAllow) {
  const allowlist = resolveAllowlist(extraAllow);
  const uris = new Set([...urisViaQpdf(pdfPath), ...urisViaMutool(pdfPath)]);
  const disallowed = [...uris].filter((u) => !uriAllowed(u, allowlist)).sort();
  return { uris: [...uris].sort(), disallowed, allowlist };
}

/** exiftool metadata as a { Tag: value } map. */
function pdfMetadata(pdfPath) {
  const raw = tool('exiftool', ['-j', '-Author', '-Creator', '-Producer', '-Title', '-Subject', '-Keywords', pdfPath]);
  let arr;
  try { arr = JSON.parse(raw); } catch { return {}; }
  return (arr && arr[0]) || {};
}

/**
 * Assert an outward-facing PDF carries no identity metadata.
 * @returns {string[]} violation messages (empty = clean)
 */
function checkPdfMetadata(pdfPath, { allowAuthor = false } = {}) {
  const meta = pdfMetadata(pdfPath);
  const violations = [];
  if (!allowAuthor && meta.Author != null && String(meta.Author).trim() !== '') {
    violations.push(`Author metadata present: "${meta.Author}" (outward-facing PDFs must carry no Author)`);
  }
  return violations;
}

/**
 * The full leak gate. Throws LeakError on any disallowed link target or metadata
 * violation, or if any inspection tool is missing (fail-closed).
 * @returns {{uris:string[]}} on success.
 */
function assertPdfSafe(pdfPath, { allow, allowAuthor = false } = {}) {
  const { uris, disallowed, allowlist } = checkPdfUris(pdfPath, allow);
  const metaViolations = checkPdfMetadata(pdfPath, { allowAuthor });

  const problems = [];
  if (disallowed.length > 0) {
    problems.push(
      `DISALLOWED link target(s) — a PDF hyperlink points somewhere not on the allowlist:\n` +
      disallowed.map((u) => `    ${u}`).join('\n'));
  }
  if (metaViolations.length > 0) {
    problems.push(`metadata violation(s):\n` + metaViolations.map((m) => `    ${m}`).join('\n'));
  }

  if (problems.length > 0) {
    throw new LeakError(
      `refusing to write the PDF — leak / metadata check failed.\n\n` +
      problems.join('\n\n') + '\n\n' +
      `allowlist in effect: ${allowlist.join(', ')}\n` +
      `(extend with PDF_URI_ALLOWLIST=host1,host2 or the renderer's allow option — ` +
      `explicitly, so nothing ships unreviewed.)`);
  }
  return { uris };
}

module.exports = {
  LeakError,
  DEFAULT_URI_ALLOWLIST,
  assertPdfSafe,
  checkPdfUris,
  checkPdfMetadata,
  uriAllowed,
  resolveAllowlist,
};

// ── CLI: audit an existing PDF ──
if (require.main === module) {
  const argv = process.argv.slice(2);
  const pdfPath = argv[0];
  let allow;
  const ai = argv.indexOf('--allow');
  if (ai !== -1) allow = argv[ai + 1];
  const allowAuthor = argv.includes('--allow-author');

  if (!pdfPath) {
    console.error('usage: verify-pdf-uris.cjs <file.pdf> [--allow host1,host2] [--allow-author]');
    process.exit(2);
  }
  if (!fs.existsSync(pdfPath)) {
    console.error(`error: no such file: ${pdfPath}`);
    process.exit(2);
  }
  try {
    const { uris } = assertPdfSafe(pdfPath, { allow, allowAuthor });
    console.log(`OK   ${pdfPath} — ${uris.length} link target(s), all allowlisted:`);
    for (const u of uris) console.log(`       ${u}`);
    if (uris.length === 0) console.log('       (none)');
  } catch (err) {
    if (err instanceof LeakError) {
      console.error(`FAIL ${pdfPath}\n${err.message}`);
      process.exit(1);
    }
    throw err;
  }
}
