#!/usr/bin/env node
/**
 * verify-pdf-footer.cjs — assert that a confidentiality footer reached EVERY
 * page of a PDF we just wrote. Shared by render-deck-pdf.cjs and
 * render-whitepaper-pdf.cjs; also runnable directly to audit an existing PDF:
 *
 *   node tools/verify-pdf-footer.cjs <file.pdf> "Example Studio — Public"
 *
 * ── Why this checks the OUTPUT, not the input ──
 *
 * The renderers stamp the footer with CSS (a per-page ::after, or Chromium's
 * footerTemplate in the @page margin box). Whether that CSS actually lands on
 * every physical page depends on the deck's own print rules — a deck whose
 * slides do not paginate (no page-break, no height:100vh) collapses 18 slide
 * elements into 13 physical pages with the footer on ONE of them.
 *
 * Every INPUT-side signal for this is unreliable: counting `.slide` elements
 * tells you about the DOM, not the paper; probing print media or looking for
 * an `@media print` block tells you the deck *has* rules, not that they
 * paginate. A guard that can be wrong is worse than no guard, because it
 * launders a bad artifact as a checked one.
 *
 * The one signal that cannot be wrong is the artifact itself: open the PDF we
 * just wrote and read the text off each page. poppler (`pdfinfo` + `pdftotext`)
 * does exactly that, so poppler is a HARD dependency of writing a marked PDF.
 * If it is missing we refuse to write — an unverifiable mark is not a mark.
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const { assertPdfSafe, LeakError } = require('./verify-pdf-uris.cjs');

/** A footer-verification failure. Reported as a message, never a stack trace. */
class FooterError extends Error {}

const INSTALL_HINT =
  'install poppler:  macOS: brew install poppler   Debian/Ubuntu: apt-get install poppler-utils';

/**
 * Collapse whitespace away before comparing.
 *
 * `pdftotext` line-wraps its output, so the footer can come back split across
 * lines; the PDF text layer can also carry NBSP / thin spaces where the source
 * had a plain space. Comparing flattened strings makes the match robust to both
 * without loosening WHAT is matched — every non-space character must still be
 * present, in order. (JS \s covers NBSP and the Unicode space run; NFC folds
 * decomposed accents so the two sides agree on composition.)
 */
function flatten(s) {
  return String(s).normalize('NFC').replace(/\s+/g, '');
}

/** Run a poppler binary, turning "not installed" into a loud, actionable error. */
function poppler(bin, args) {
  try {
    return execFileSync(bin, args, { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  } catch (err) {
    if (err.code === 'ENOENT') {
      throw new FooterError(
        `${bin} not found — cannot verify that the footer reached every page, so refusing to\n` +
        `write the PDF. A confidentiality mark we did not check is not a mark.\n` +
        `    ${INSTALL_HINT}`);
    }
    throw new FooterError(`${bin} failed on the rendered PDF: ${(err.stderr || err.message).toString().trim()}`);
  }
}

/** Physical page count of the written PDF, straight from the file. */
function pdfPageCount(pdfPath) {
  const out = poppler('pdfinfo', [pdfPath]);
  const m = out.match(/^Pages:\s+(\d+)\s*$/m);
  if (!m) throw new FooterError(`pdfinfo reported no page count for ${pdfPath} — is it a valid PDF?`);
  return parseInt(m[1], 10);
}

/** Extracted text of a single physical page. */
function pdfPageText(pdfPath, p) {
  return poppler('pdftotext', ['-enc', 'UTF-8', '-f', String(p), '-l', String(p), pdfPath, '-']);
}

/** "2-7, 9, 11-13" — a long unmarked-page list stays readable. */
function formatPages(pages) {
  const runs = [];
  for (const p of pages) {
    const last = runs[runs.length - 1];
    if (last && p === last[1] + 1) last[1] = p;
    else runs.push([p, p]);
  }
  return runs.map(([a, b]) => (a === b ? `${a}` : `${a}-${b}`)).join(', ');
}

/**
 * Read `pdfPath` and report which physical pages carry `footerText`.
 * @returns {{pages:number, marked:number, unmarked:number[]}}
 */
function verifyPdfFooter(pdfPath, footerText) {
  const needle = flatten(footerText);
  if (!needle) throw new FooterError('cannot verify an empty footer string');
  const pages = pdfPageCount(pdfPath);
  if (pages === 0) throw new FooterError(`${pdfPath} has zero pages — nothing was marked`);

  const unmarked = [];
  for (let p = 1; p <= pages; p++) {
    if (!flatten(pdfPageText(pdfPath, p)).includes(needle)) unmarked.push(p);
  }
  return { pages, marked: pages - unmarked.length, unmarked };
}

/**
 * The gate the renderers call. Verifies `tmpPath`, and ONLY on a clean result
 * moves it to `outPath`.
 *
 * Rendering to a temp file and renaming on success means an unverified PDF is
 * never observable at the output path — not even briefly, and not if we crash
 * mid-check. A previous good PDF at `outPath` also survives a failed re-render
 * rather than being clobbered by a bad one.
 *
 * `hint` is the caller's remediation advice for a partially-marked render — the
 * cause differs by renderer (deck pagination vs. @page margin box), so the
 * diagnosis belongs to whoever knows how the footer was stamped.
 *
 * Before the footer check, the same gate runs the leak check (`assertPdfSafe`,
 * verify-pdf-uris.cjs): every `/URI` link target must be on the allowlist and no
 * identity metadata may be present. `allow` extends the URI allowlist for this
 * render; `allowAuthor` permits an Author field (a published paper legitimately
 * has one — a confidential deck does not). Both the leak check and the footer
 * check are fail-closed: a missing inspection tool refuses the write.
 *
 * @returns {{pages:number}} on success. Throws FooterError / LeakError otherwise.
 */
function commitVerifiedPdf({ tmpPath, outPath, footerText, sourcePath, hint, allow, allowAuthor = false }) {
  // ── Leak gate: no disallowed link target, no identity metadata (#51, #53) ──
  try {
    assertPdfSafe(tmpPath, { allow, allowAuthor });
  } catch (err) {
    fs.rmSync(tmpPath, { force: true });
    throw err;
  }

  let result;
  try {
    result = verifyPdfFooter(tmpPath, footerText);
  } catch (err) {
    fs.rmSync(tmpPath, { force: true });
    throw err;
  }

  if (result.unmarked.length > 0) {
    fs.rmSync(tmpPath, { force: true });
    throw new FooterError(
      `the footer did NOT reach every page — refusing to write a partially-marked PDF.\n` +
      `  source:    ${sourcePath}\n` +
      `  footer:    ${footerText}\n` +
      `  PDF pages: ${result.pages}   marked: ${result.marked}   UNMARKED: ${result.unmarked.length}\n` +
      `  unmarked page(s): ${formatPages(result.unmarked)}\n` +
      (hint ? `\n${hint}\n` : '') +
      `\nNo PDF was written.`);
  }
  fs.renameSync(tmpPath, outPath);
  return { pages: result.pages };
}

module.exports = { FooterError, LeakError, verifyPdfFooter, commitVerifiedPdf, formatPages, flatten };

// ── CLI: audit an existing PDF ──
if (require.main === module) {
  const [pdfPath, footerText] = process.argv.slice(2);
  if (!pdfPath || !footerText) {
    console.error('usage: verify-pdf-footer.cjs <file.pdf> "Owner — Mark"');
    process.exit(1);
  }
  if (!fs.existsSync(pdfPath)) {
    console.error(`error: no such file: ${pdfPath}`);
    process.exit(1);
  }
  try {
    const { pages, marked, unmarked } = verifyPdfFooter(pdfPath, footerText);
    if (unmarked.length > 0) {
      console.error(
        `FAIL ${pdfPath} — ${marked}/${pages} pages marked; unmarked: ${formatPages(unmarked)}`);
      process.exit(1);
    }
    console.log(`OK   ${pdfPath} — ${pages}/${pages} pages carry: ${footerText}`);
  } catch (err) {
    if (err instanceof FooterError) {
      console.error(`error: ${err.message}`);
      process.exit(1);
    }
    throw err;
  }
}
