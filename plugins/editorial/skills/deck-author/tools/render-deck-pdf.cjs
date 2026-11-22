#!/usr/bin/env node
// Render local HTML and verify every PDF page before publishing the output file.
const fs = require('fs');
const os = require('os');
const path = require('path');
const { FooterError, commitVerifiedPdf } = require('./verify-pdf-footer.cjs');

const USAGE = 'usage: render-deck-pdf.cjs <deck.html> [out.pdf | out-dir/] [--footer "Owner — Mark"] [--allow host1,host2]';

function fail(msg) { console.error(`error: ${msg}`); process.exit(1); }

/** Errors we own — reported as a one-line message, not a stack trace. */
class DeckError extends Error {}

// ── Naming the output from the deck's own title (#68) ──
//
// A rendered deck must be IDENTIFIABLE once it leaves the repo — attached to an
// email, dropped in a chat. Writing every deck to a constant "deck.pdf" makes a
// pile of attachments that are indistinguishable by name, so the output filename
// is DERIVED from the deck's title. The title travels with the deck (same reasoning
// as the deck-footer), resolved in this precedence:
//
//   1. <meta name="deck-title" content="..."> in the deck's <head>  (explicit, curated)
//   2. the deck's <title>                                           (every deck has one)
//   3. the deck's directory name                                    (last-resort fallback)
//
// The output path is still overridable positionally: pass an explicit "<out.pdf>"
// (a file path) to name it yourself — unchanged from before — or pass a directory
// ("<dir>/") to derive the filename INTO that directory. Omit it entirely and the
// PDF is written beside the deck's index.html with the derived name.

// Turn a human title into a filesystem- and attachment-friendly slug:
// "Assay for M&A — The End of Watermelon Reporting" → "assay-for-m-and-a-the-end-of-watermelon-reporting".
function slugifyTitle(title) {
  const slug = String(title)
    .normalize('NFKD').replace(/[\u0300-\u036f]/g, '') // strip diacritics
    .replace(/&/g, ' and ')                            // & reads as a word, not punctuation
    .replace(/[^A-Za-z0-9]+/g, '-')                    // any run of non-alphanumerics → one dash
    .replace(/^-+|-+$/g, '')                           // no leading/trailing dashes
    .toLowerCase()
    .slice(0, 80);                                     // cap length for sane filenames
  return slug.replace(/-+$/g, '');                     // slice may leave a trailing dash
}

// Derive "<slug>.pdf" from the resolved title, falling back to the directory name
// and finally the literal "deck" so this never returns just ".pdf".
function deriveOutName(title, deckDir) {
  const base = slugifyTitle(title) || slugifyTitle(path.basename(path.resolve(deckDir))) || 'deck';
  return `${base}.pdf`;
}

// ── Footer policy (mirrored in render-whitepaper-pdf.cjs) ──

function checkFooter(text, source) {
  const footer = String(text).trim();
  if (!footer) throw new DeckError(source + ' must contain the wording to print on each page');
  return footer;
}

function noFooterError(deckPath) {
  return new DeckError('Set deck-footer metadata in ' + deckPath + ' or supply --footer with the page credit or classification.');
}

// ── Args ──
const args = process.argv.slice(2);
let cliFooter = null;
const footerIdx = args.indexOf('--footer');
if (footerIdx !== -1) {
  const v = args[footerIdx + 1];
  if (!v) fail('--footer needs a value, e.g. --footer "Example Studio — Public"');
  cliFooter = v;
  args.splice(footerIdx, 2);
}
// --allow host1,host2 : extend the leak-check URI allowlist for this deck (a deck
// that legitimately links to a new public host declares it here, explicitly).
let cliAllow = null;
const allowIdx = args.indexOf('--allow');
if (allowIdx !== -1) {
  const v = args[allowIdx + 1];
  if (!v) fail('--allow needs a value, e.g. --allow "example.com,github.com/some-org"');
  cliAllow = v;
  args.splice(allowIdx, 2);
}
const [htmlPath, outArg] = args;
if (!htmlPath) fail(USAGE);
if (!fs.existsSync(htmlPath)) fail(`no such file: ${htmlPath}`);
// Resolved once the browser can read the deck's title (see the async body).
let outPath = null;

function resolvePuppeteer() {
  const root = process.env.EDITORIAL_RENDER_DEPS || process.cwd();
  return require(require.resolve('puppeteer', { paths: [root] }));
}

// Decks name their page element `.slide` (most of them) or `.page` (loops/).
// The footer anchors to BOTH, so a deck using the other convention can't
// silently get the mark on a subset of its pages.
const PAGE_SELECTORS = ['.slide', '.page'];
const PAGE_SELECTOR = PAGE_SELECTORS.join(', ');

// ── The printed page box, in CSS px ──
// page.pdf() below prints landscape Letter (11in x 8.5in) at 96 CSS px/in with zero
// margins. Matching the viewport to that box makes `vh` units — which every deck's
// print CSS is built on (height:100vh) — resolve to the same height they will resolve
// to on paper, so what we measure is what gets printed.
const PAGE_W = 1056; // 11in   * 96
const PAGE_H = 816;  // 8.5in  * 96

/**
 * ── The content-fit gate: assert no slide's content is clipped by the page box ──
 *
 * The decks' print CSS discards anything that outgrows the page, on BOTH axes:
 *     .slide{height:100vh;overflow:hidden}
 *     .slide-content{max-height:calc(100vh - 48px);overflow:hidden}
 * Check both axes before printing. Fixed-height containers can silently hide content.
 *
 * Why a naive scrollHeight-vs-clientHeight comparison cannot see this:
 *
 *   A page element that is `height:100vh; overflow:hidden` CANNOT REPORT OVERFLOW.
 *   Its scrollHeight is clamped to its own fixed height, so scrollHeight - clientHeight
 *   is 0 no matter how much content is inside. In these decks the element that actually
 *   clips is the INNER `.slide-content` (max-height'd, overflow:hidden) — the child
 *   absorbs the overflow, so the parent sees none. Measuring either one in place returns
 *   a clean 0 on the very slide that is dropping cards. So instead we temporarily LIFT the
 *   height/max-height/overflow clamps on the page element and every clipping descendant,
 *   let the content take the height it actually wants, measure that against the page box,
 *   and restore every style — leaving the DOM exactly as it was.
 *
 * This is a CONTENT check, and it is deliberately not the footer check. The footer gate
 * proves the MARK reaches every page; it is green on a clipped deck, because a clipped
 * page is still a marked page. Neither check subsumes the other.
 *
 * Text extraction cannot substitute for this: poppler ignores clipping paths, so a line
 * sliced in half by overflow:hidden still extracts as clean, complete text. Only DOM
 * measurement, in-page and under print emulation, can see it.
 *
 * Runs BEFORE page.pdf(), so an overflowing deck writes no bytes at all — not even the
 * temp file. Any previously-good PDF at outPath is left untouched rather than clobbered.
 */
async function assertNoOverflow(page, htmlPath) {
  await page.setViewport({ width: PAGE_W, height: PAGE_H });
  await page.emulateMediaType('print');
  await page.evaluate(() => document.fonts.ready);

  // ($$eval runs this fixed callback in the page — no user-supplied code is evaluated.)
  const slides = await page.$$eval(PAGE_SELECTOR, (els) => {
    const CLIPPING = ['hidden', 'clip', 'auto', 'scroll'];

    // Replaced/embedded elements are NOT layout containers, and must never be relaxed.
    // `svg` in particular computes to overflow:hidden in the UA stylesheet, so a naive
    // "does it clip?" test matches every inline logo — and un-clamping one lets it snap
    // to its intrinsic viewBox (256px), inflating the slide by hundreds of px and making
    // the gate scream about slides that fit perfectly well. An <svg>/<img> at a set size
    // is a deliberate size, not text being cut off.
    const REPLACED = ['svg', 'img', 'canvas', 'video', 'iframe', 'object', 'embed'];

    const clips = (el) => {
      if (REPLACED.includes(el.tagName.toLowerCase())) return false;
      const s = getComputedStyle(el);
      return CLIPPING.includes(s.overflowY) || CLIPPING.includes(s.overflow);
    };
    const describe = (el) =>
      el.tagName.toLowerCase() +
      (typeof el.className === 'string' && el.className.trim()
        ? '.' + el.className.trim().split(/\s+/).join('.')
        : '');

    // The printed content box, per page element: the page dimensions less its own padding,
    // on BOTH axes. (816×1056 landscape-Letter page - 48px/96px of padding = the 768×960 box
    // these decks get.) Read for ALL elements BEFORE any is relaxed below — once an element is
    // set to height:auto its clientHeight becomes its content height, not its page box.
    const boxes = els.map((el) => {
      const cs = getComputedStyle(el);
      const padY = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
      const padX = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight);
      return { padY, padX, boxH: el.clientHeight - padY, boxW: el.clientWidth - padX };
    });

    return els.map((el, i) => {
      const { padY, padX, boxH, boxW } = boxes[i];

      // ── Measure the content's NATURAL size by removing the clamps, on BOTH axes ──
      // `.slide-content{overflow:hidden}` clips horizontally AND vertically, so a slide loses
      // content two ways: it grows too tall (cards fall off the bottom) or too wide (a long
      // <pre>/code line / unbroken URL is cut off the right edge). Reading scroll size against
      // the clamps in place cannot see either: a height:100vh page element reports
      // scrollHeight === its own height, and overflow:hidden clamps scrollWidth to the box
      // too — the child absorbs the overflow on both axes and the parent reports a comfortable
      // zero on a slide that is dropping content.
      // So we lift the height/max-height/overflow constraints on the page element and every
      // clipping descendant (overflow:visible exposes the true scrollWidth as well as
      // scrollHeight), let the content take the size it actually wants, measure both axes, and
      // put every style back exactly as it was. Nothing is left mutated, and this works whether
      // the deck clips at the page element, at an inner wrapper, or at a nested box.
      const saved = [];
      const relax = (n) => {
        saved.push([n, n.getAttribute('style')]);
        n.style.height = 'auto';
        n.style.maxHeight = 'none';
        n.style.overflow = 'visible';
      };
      relax(el);
      for (const n of el.querySelectorAll('*')) if (clips(n)) relax(n);

      const usedH = el.scrollHeight - padY;
      const usedW = el.scrollWidth - padX;

      for (const [n, style] of saved) {
        if (style === null) n.removeAttribute('style');
        else n.setAttribute('style', style);
      }

      // Report the worse axis. Width is measured but only reported when it actually overflows,
      // so the common vertical case reads exactly as before.
      const overH = usedH - boxH;
      const overW = usedW - boxW;
      const vertical = overH >= overW;
      const h2 = el.querySelector('h2');
      return {
        n: i + 1,
        title: (h2 ? h2.textContent : '').replace(/\s+/g, ' ').trim(),
        over: Math.max(overH, overW),
        axis: vertical ? 'H' : 'W',
        used: vertical ? usedH : usedW,
        box: vertical ? boxH : boxW,
        overH, overW, usedH, usedW, boxH, boxW,
        where: describe(el),
      };
    });
  });
  const overflowing = slides.filter((s) => s.over > 0);

  if (overflowing.length) {
    const rows = overflowing
      .map((s) =>
        `    slide ${String(s.n).padStart(2)}  ${s.axis === 'W' ? 'width ' : 'height'} ` +
        `${String(s.used).padStart(5)}px of ${s.box}px — ${String(s.over).padStart(4)}px CLIPPED  <${s.where}>` +
        (s.title ? `  “${s.title}”` : ''))
      .join('\n');
    throw new DeckError(
      `${overflowing.length} slide(s) in ${htmlPath} overflow the printed page box — refusing to write a\n` +
      `PDF that silently discards content:\n\n${rows}\n\n` +
      `The print CSS clips the excess with no error and no visible tell, so the PDF would look fine\n` +
      `and be missing the bottom (height) or right edge (width) of these slides. Fix the SLIDES,\n` +
      `not this check:\n` +
      `  - height: split an over-tall slide across two (or three) slides, at a seam in the content;\n` +
      `  - width: wrap or break the long line (a <pre>/code block or unbroken URL is the usual cause);\n` +
      `  - or tighten the layout — padding/margins only.\n` +
      `NEVER delete content to make it fit: a slide trimmed to fit is a silent retraction, and it is\n` +
      `the hedges and caveats that go first, which leaves the deck LESS HONEST than its source.\n` +
      `Leave real headroom — landing a slide at 765/768 is one font-fallback away from clipping again.`);
  }

  // Report the tightest slide on the VERTICAL axis, so a deck creeping toward the bottom edge
  // is visible BEFORE it clips. (Width headroom is not informative here: wrapping content fills
  // the width box exactly, so every slide sits at ~0px width headroom by design — a horizontal
  // problem only exists as an actual overflow, which the failure path above catches.)
  // "Fits" and "fits comfortably" are different facts and only one of them keeps.
  const tightest = slides.reduce((a, b) => (b.usedH > a.usedH ? b : a), slides[0]);
  return {
    count: slides.length,
    tightest: { n: tightest.n, used: tightest.usedH, box: tightest.boxH, axis: 'H' },
  };
}

(async () => {
  // Validate --footer before spending a browser launch on a doomed render.
  if (cliFooter !== null) cliFooter = checkFooter(cliFooter, '--footer');

  const puppeteer = resolvePuppeteer();
  const browser = await puppeteer.launch({ headless: 'new' });
  try {
    const page = await browser.newPage();
    await page.goto(`file://${path.resolve(htmlPath)}`, { waitUntil: 'networkidle0', timeout: 60000 });
    await page.evaluate(() => document.fonts.ready);

    // ── Footer precedence: --footer > <meta name="deck-footer"> > hard error ──
    let footerText = cliFooter;
    if (footerText === null) {
      const declared = await page.evaluate(() => {
        const m = document.querySelector('meta[name="deck-footer"]');
        return m ? m.getAttribute('content') : null;
      });
      if (declared === null) throw noFooterError(htmlPath);
      footerText = checkFooter(declared, `<meta name="deck-footer"> in ${htmlPath}`);
    }

    // Cheap pre-flight: with no page elements to anchor to, the footer would stamp
    // nothing at all. The post-render assertion below would catch this too, but
    // failing here costs no render and names the actual cause.
    // ($$eval runs this fixed callback in the page — no user-supplied code is evaluated.)
    const elementCount = await page.$$eval(PAGE_SELECTOR, (els) => els.length);
    if (elementCount === 0) {
      throw new DeckError(
        `no "${PAGE_SELECTOR}" elements in ${htmlPath} — the per-page footer anchors to them and would\n` +
        `stamp nothing; refusing to write an unmarked PDF. Give the deck's page elements class="slide".`);
    }

    // ── The content gate: no slide may be clipped by the page box ──
    // Before we spend a render: a PDF that drops content is worse than no PDF, because
    // it looks complete. Throws (writing nothing) if any slide overflows.
    const fit = await assertNoOverflow(page, htmlPath);

    // The decks' print CSS makes each page element a full page (position:relative,
    // height:100vh, page-break-after), so an absolutely-positioned ::after lands at
    // the bottom of every page. Mid-gray reads on both the dark house surface and the
    // light internal surface. Appended last, so it also overrides any footer baked
    // into the deck's own print CSS (same pseudo-element, later rule wins).
    await page.addStyleTag({ content:
      `@media print{` +
      `${PAGE_SELECTORS.map((s) => `${s}{position:relative}`).join('')}` +
      `${PAGE_SELECTORS.map((s) => `${s}::after`).join(',')}{content:${JSON.stringify(footerText)};` +
      `position:absolute;left:0;right:0;bottom:9px;text-align:center;` +
      `font-family:'Inter',system-ui,sans-serif;font-size:7pt;font-weight:400;` +
      `letter-spacing:.08em;color:#8A8A8A;z-index:60}}` });

    // ── Resolve where to write: explicit file path wins; otherwise derive from the title (#68) ──
    // Explicit "<out.pdf>" (a file path) → used verbatim, exactly as before. A directory
    // ("<dir>/", or an existing dir) → derive the filename INTO it. Omitted → derive beside
    // the deck's index.html. Deriving reads the title the same way the footer is read.
    {
      const deckDir = path.dirname(path.resolve(htmlPath));
      const argIsDir = !!outArg &&
        (outArg.endsWith('/') || outArg.endsWith(path.sep) ||
         (fs.existsSync(outArg) && fs.statSync(outArg).isDirectory()));
      if (outArg && !argIsDir) {
        outPath = outArg; // explicit file path — unchanged behaviour
      } else {
        const title = await page.evaluate(() => {
          const m = document.querySelector('meta[name="deck-title"]');
          const c = m && m.getAttribute('content');
          return (c && c.trim()) || (document.title || '').trim();
        });
        outPath = path.join(argIsDir ? outArg : deckDir, deriveOutName(title, deckDir));
        console.log(`derived output name from deck title: ${outPath}`);
      }
    }

    // Render beside the destination (same filesystem, so the rename is atomic) and
    // only publish it to outPath once every page is confirmed marked.
    const tmpPath = path.join(
      path.dirname(path.resolve(outPath)),
      `.${path.basename(outPath)}.tmp-${process.pid}`);
    try {
      await page.pdf({
        path: tmpPath,
        landscape: true,
        format: 'Letter',
        printBackground: true,
        margin: { top: 0, bottom: 0, left: 0, right: 0 },
      });

      // ── The gate: the footer must be on every page of the PDF we just wrote ──
      const { pages } = commitVerifiedPdf({
        tmpPath, outPath, footerText, sourcePath: htmlPath, allow: cliAllow,
        // A confidential deck carries NO Author metadata (allowAuthor defaults false).
        hint:
          `The footer is stamped once per PAGE ELEMENT (${elementCount} found), so a shortfall means this\n` +
          `deck's print CSS does not paginate: its page elements collapse into fewer physical pages than\n` +
          `elements, and only the page carrying the first element gets the mark.\n` +
          `Fix the deck's @media print rules so each page element becomes exactly one printed page — the\n` +
          `decks that paginate correctly use:\n` +
          `    @page{size:landscape;margin:0}\n` +
          `    @media print{${PAGE_SELECTOR}{height:100vh;page-break-after:always}}\n` +
          `then re-render.`,
      });
      console.log(`wrote ${outPath} — ${pages} page(s), footer verified on all ${pages}: ${footerText}`);
      console.log(
        `content fit: ${fit.count} slide(s), none clipped (both axes) — tightest is slide ${fit.tightest.n} ` +
        `at ${fit.tightest.used}px of ${fit.tightest.box}px ${fit.tightest.axis === 'W' ? 'width' : 'height'} ` +
        `(${fit.tightest.box - fit.tightest.used}px headroom)`);
    } catch (err) {
      fs.rmSync(tmpPath, { force: true });
      throw err;
    }
  } finally {
    await browser.close();
  }
})().catch((err) => {
  if (err instanceof DeckError || err instanceof FooterError) fail(err.message);
  console.error(err);
  process.exit(1);
});
