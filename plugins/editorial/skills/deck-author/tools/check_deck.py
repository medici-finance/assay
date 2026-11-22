#!/usr/bin/env python3
r"""Structural + brand checks for a deck built against a brand contract (brand.toml).

Adapted from create-html-deck's `scripts/check_deck.py` (MIT, (c) tt-a1i). The HARNESS is
the lift: stdlib-only `HTMLParser`, a flat finding accumulator, exit 1 on any error, one
`PASS:` summary line. The RULE TABLE is re-pinned wholesale to the house output shape,
because the upstream table encodes a different house's contract and emits six false errors
against a perfectly good house deck:

  * upstream required `width:1920px` / `height:1080px` / a centred `scale()` transform.
    The house geometry is viewport-fluid (100vw/100vh, `.slide{position:absolute;inset:0}`,
    `@page{size:landscape}`). Those three regexes are removed, and the house geometry is
    required in their place.
  * upstream required `localStorage` slide persistence. A confidential deck deliberately
    does not persist reading position into the viewer's browser profile. Removed.
  * upstream counted only `<article class="slide">`. House skeletons use `<div>`, this
    contender uses `<section>`. The selector is now "any element carrying the `slide`
    class", so the slide count is never silently zero.

Added house invariants: the mandatory `<meta name="deck-footer">` and its two-value class
enum (caught here, BEFORE the PDF renderer hard-errors on it), the per-slide print footer,
the pinned IBM Plex font set, offline-only assets, the closed layout pool, the
key-question and action-title smells, process-language and placeholder leaks, an accent
budget, and `deck.md` <-> `index.html` parity.

Scoped ignores use the house `.impeccable` shape verbatim — a `detector` object carrying
`ignoreRules` / `ignoreFiles` / `ignoreValues[]`, every value entry carrying a `reason`.
No second ignore syntax is invented.

Second re-pin (2026-08-18, packaging into skills/deck-author): the geometry rule now
requires the WINNING house contract from the deck bake-off — a fixed 1280x720 letterboxed
16:9 stage scaled to fit the viewport (a fixed-stage HTML deck), NOT the
viewport-fluid geometry the composite contender pinned. Rationale: the fixed stage makes
the presenter's view, a screenshot, and the PDF page the same composition, which is what
the render-and-look gate screenshots depend on. The folio check also accepts the
skeleton's `.slide-no` bare-number markers alongside `.folio` "N / N" pairs.

Third re-pin (2026-08-18, brand parameterization): the brand-specific pins — the footer
enum, the font allowlist, the accent budget, the remote-asset severity, the pure-tone
expectations — now come from a brand.toml CONTRACT instead of constants, so one linter
serves any brand. Resolution order (same as the skill's): --brand <path> → brand.toml
walking up from the deck to the consuming repo's root (nearest .git) → the skill's
none. With no contract (or
`--brand none`) every brand gate SKIPs with a printed notice and only the structural
checks rule — and a brand-neutral deck carries no confidentiality footer unless one is
given. TOML parses via stdlib tomllib (python >= 3.11): on an older python an EXPLICIT
--brand is a hard error (exit 2), while a merely-discovered contract downgrades to the
no-contract SKIP path with a notice. The linter stays stdlib-only either way.

Fourth pass (presentation-skills/04, from the RWA deck bake-off verdicts): one rule
that fired wrongly is fixed, and three rules the bake-off proved missing are added.
  * PROCESS_LEAK / PLACEHOLDER now match on a BOUNDARY, not a substring. `deck.md` used
    to fire inside every honest citation of `canton-pitch-deck.md`, and `xxx` inside a
    hex-ish string or an `XXXX` redaction. Each needle compiles once (_boundary_pattern): a leading word
    character, dot or hyphen means the needle is part of a longer name (a placeholder
    needle is a template token, so only a letter or dot binds it: `555-XXX`,
    `TODO_OWNER`, `TBD2027`, `$XXXm`), and a trailing word character does too,
    while a trailing dot only counts when another word character follows (so
    `deck.md.bak` is a different file, and `… in deck.md.` at the end of a sentence is
    still the leak). A trailing hyphen is the needle itself (`TODO-OWNER`), a needle
    ending in punctuation takes no trailing guard (`AUDIENCE:X`), `acme corp` matches as
    a prefix, and word needles take a plural (`slide maps`).
  * H-INTERNAL-FILENAME (error): a filename-shaped token (`name.md`, `.html`, `.yaml`,
    `.yml`, `.toml`, `.py`, `.mjs`), path prefix or not, or a `§`-numbered section
    reference in the AUDIENCE-visible slide text. It matches a shape, not a list, so it
    needs no per-deck update. Only a filename inside a link is exempt: a `scheme://`
    URL, or a scheme-less `host.tld/…` one (`www.sec.gov/files/form-d.html`).
    Presenter notes (`.notes` / `.speaker-notes` inside a slide) are
    not audience-visible and are exempt, which is where a source trail belongs.
  * H-CONTRAST-FLOOR: a STATIC read of the deck's declared CSS. Each rule styling a
    required-content class (provenance/source/citation, the confidentiality footer
    including the `.slide::after` print footer, the eyebrow/kicker, the folio) has its
    `color` resolved through `var()` and compared against the nearest declared
    background. Below 4.5:1 is an error for provenance and the footer and a notice
    elsewhere. An unresolvable colour, or a pass measured only against an assumed white
    canvas, is a could-not-check notice, never a silent pass. Every finding prints the selector, the ratio to two decimals, and both hex
    values. A static read cannot see a computed style, so a green result here is NOT an
    accessibility pass: the render-time computed-style review is the authority.
  * H-CONTRACT-VOCAB (error): the staging contract's banned on-screen tokens are PARSED
    from `staging/references/vocabulary-map.md` (never a second copy here) and matched
    case-insensitively on a word boundary against audience-visible slide text. The
    message quotes the map's replacement shape. A missing map prints a SKIP notice and
    never passes silently.

Known limit, stated rather than hidden: this is a STATIC parser. A deck whose slides are
built by script (the staged bake-off decks build their DOM from a JS slide array) is
invisible to it — S-SLIDES reports 0 and everything downstream is noise. The deck-author
contract therefore requires slides as static markup; the render-and-look gate, not this
linter, is what exercises the live DOM.

Usage:
    python3 check_deck.py <deck>/index.html [--deck-md <deck>/deck.md]
                          [--brand <brand.toml> | --brand none]
                          [--config .deckcheck.json] [--quiet]

Exit: 0 clean (notices may print), 1 one or more errors, 2 usage/IO problem.

The checker harness adapts create-html-deck. Copyright (c) tt-a1i.
MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

from __future__ import annotations

import argparse
import colorsys
import html as htmlmod
import json
import re
import sys
from collections import Counter
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlparse

# ---------------------------------------------------------------- brand contract

GENERIC_FONTS = {
    # The generic fallback stack any brand may resolve to.
    "ui-monospace",
    "sfmono-regular",
    "menlo",
    "monospace",
    "system-ui",
    "sans-serif",
    "serif",
    "-apple-system",
    "blinkmacsystemfont",
    "inherit",
    "initial",
}


class Brand:
    """The parsed brand contract. `None` in its place means: brand gates SKIP."""

    def __init__(self, data: dict, path: Path) -> None:
        self.path = path
        ident = data.get("identity", {}) or {}
        self.org = str(ident.get("org", "")).strip()
        self.footer_enum: dict[str, str] = {}
        if ident.get("footer_internal"):
            self.footer_enum[str(ident["footer_internal"]).strip()] = "internal"
        if ident.get("footer_external"):
            self.footer_enum[str(ident["footer_external"]).strip()] = "external"

        typ = data.get("type", {}) or {}
        families = [typ.get("sans"), typ.get("mono"), *typ.get("extra_families", [])]
        self.font_allow = GENERIC_FONTS | {str(n).strip().lower() for n in families if n}

        pal = data.get("palette", {}) or {}
        self.canvas = str(pal.get("canvas", "")).strip().lower()
        self.text_primary = str(pal.get("text_primary", "")).strip().lower()

        rules = data.get("rules", {}) or {}
        self.accent_budget = int(rules.get("accent_budget", MAX_ACCENT_PER_SLIDE))
        self.no_remote = bool(rules.get("no_remote_assets", True))


def load_brand(path: Path) -> Brand:
    try:
        import tomllib
    except ImportError as exc:  # python < 3.11
        raise RuntimeError(
            "parsing brand.toml needs stdlib tomllib (python >= 3.11); "
            "upgrade python or run with --brand none"
        ) from exc
    with path.open("rb") as fh:
        return Brand(tomllib.load(fh), path)


def resolve_brand(arg: str | None, deck_path: Path) -> tuple[Brand | None, str]:
    """The skill's resolution order. Returns (brand-or-None, how-it-resolved)."""
    if arg == "none":
        return None, "brand gates skipped by --brand none"
    if arg:
        path = Path(arg).expanduser().resolve()
        if not path.is_file():
            raise RuntimeError(f"brand contract not found: {path}")
        return load_brand(path), f"explicit --brand {path}"
    # walk up from the deck to the consuming repo's root (nearest .git)
    probe = deck_path.parent
    while True:
        candidate = probe / "brand.toml"
        if candidate.is_file():
            try:
                return load_brand(candidate), f"discovered at {candidate}"
            except RuntimeError as exc:
                return None, f"found {candidate} but could not parse it ({exc}) — brand gates SKIP"
        if (probe / ".git").exists() or probe.parent == probe:
            break
        probe = probe.parent
    return None, "no brand contract discoverable — brand gates SKIP"


# ---------------------------------------------------------------- structural constants

LAYOUT_POOL = {
    "L-COVER",
    "L-STATEMENT",
    "L-TWO-COL",
    "L-TABLE",
    "L-CODE",
    "L-DIAGRAM",
    "L-STAT-ROW",
    "L-COMPARE",
    "L-SEQUENCE",
    "L-CLOSING",
}

# Layouts that carry no action title / key question by design.
CHROME_LAYOUTS = {"L-COVER", "L-CLOSING"}

VOID_TAGS = {
    "area", "base", "br", "col", "embed", "hr", "img", "input", "link",
    "meta", "param", "source", "track", "wbr",
}

PROCESS_LEAK = [
    "option a", "option b", "option c", "preview.md", "deck.md", "index.html",
    "skill.md", "audience:", "slide map", "style preset", "bold option",
    "safe option", "for internal sharing", "generated from",
]

PLACEHOLDER = [
    "lorem ipsum", "todo", "fixme", "acme corp", "john doe", "jane doe",
    "placeholder", "your company", "xxx", "tbd",
]


# Needles matched as a PREFIX: the placeholder is the stem, whatever word it grows into
# (`Acme Corp`, `Acme Corp.`, `Acme Corporation`).
PREFIX_NEEDLES = {"acme corp"}
# Needles whose plural `s` would turn them into ordinary English (`John does the review`).
NO_PLURAL_NEEDLES = {"john doe", "jane doe"}
# Placeholder needles made of one repeated letter: only a longer run of that letter
# (`XXXX`, a redaction) makes them something else, so a unit letter after one does not.
X_RUN_NEEDLES = {"xxx"}


# A letter, never a digit or underscore (`[^\W\d_]` is `\w` minus both).
_LETTER = r"[^\W\d_]"


def _boundary_pattern(needle: str, *, placeholder: bool = False) -> re.Pattern[str]:
    """One needle, matched only where it is not part of a longer name.

    Leading: no word character, dot or hyphen directly before it, so `deck.md` inside
    `canton-pitch-deck.md` (hyphen) or `pilot_deck.md` (word char) does not match, and a
    process phrase glued to a hyphen is another word (`fail-safe option`).
    Trailing: no word character after it, and a dot only when another word character
    follows it (`deck.md.bak`). A sentence-ending full stop still matches, and so does a
    trailing hyphen: `TODO-OWNER`, `TBD-2027`, `placeholder-image` and
    `preview.md-derived` are the needle itself. A needle that already ends in punctuation
    (`audience:`) takes no trailing guard (`AUDIENCE:INVESTORS`), and a PREFIX_NEEDLES
    entry takes none either (`Acme Corporation`).
    Placeholder needles (`placeholder=True`) are template tokens, so only a LETTER (or a
    dot before a word character) on either side makes them part of a longer word. A
    hyphen, underscore or digit is a boundary: `555-XXX-XXXX`, `Q3-TBD`, `Q3TBD`,
    `TODO_OWNER`, `OWNER_TODO`, `{{COMPANY_PLACEHOLDER}}`, `FIXME_LATER`, `TBD2027` and
    `TODO1` all match. `xxx` refuses only another `x` after it, so a unit or scale
    letter after it is still the placeholder (`$XXXm`, `$XXXk`), while an `XXXX` run
    stays a redaction (the leading letter guard refuses it from the other side).
    Plural: a needle whose last word is two or more letters also takes an `s`
    (`TODOs`, `slide maps`, `bold options`), which the substring match used to catch.
    Not a single-letter tail (`option as` is English) and not NO_PLURAL_NEEDLES.
    Whitespace inside a needle matches any run of whitespace.
    """
    body = r"\s+".join(re.escape(part) for part in needle.split())
    last_word = needle.split()[-1]
    if re.fullmatch(r"[a-z]{2,}", last_word) and needle not in NO_PLURAL_NEEDLES:
        body += "s?"
    if placeholder:
        lead = rf"(?<!{_LETTER}|\.)"
        trail = r"(?!x|\.\w)" if needle in X_RUN_NEEDLES else rf"(?!{_LETTER}|\.\w)"
    else:
        lead, trail = r"(?<![\w.\-])", r"(?!\w|\.\w)"
    if needle in PREFIX_NEEDLES or not needle[-1].isalnum():
        trail = ""
    return re.compile(rf"{lead}{body}{trail}", re.IGNORECASE)


PROCESS_LEAK_RE = [(needle, _boundary_pattern(needle)) for needle in PROCESS_LEAK]
PLACEHOLDER_RE = [(needle, _boundary_pattern(needle, placeholder=True)) for needle in PLACEHOLDER]

# H-INTERNAL-FILENAME: a SHAPE, not a list — any filename with an extension the house
# pipeline writes, or a `§`-numbered section reference (`house-brand-guide.md §0.5`).
# A path prefix does not hide it (`../staging/staging.yaml`, `/docs/x.md`, `a.md/b.html`):
# the only exemption is a filename inside a link a reader can follow — a `scheme://` URL
# or a scheme-less `host.tld/…` one
# (URL_RE, applied in check_internal_filenames).
INTERNAL_FILENAME_RE = re.compile(
    r"(?<![\w.\-])(?:[\w\-]+/)*[\w\-]+(?:\.[\w\-]+)*\.(?:md|html|yaml|yml|toml|py|mjs)(?!\w|\.\w)",
    re.IGNORECASE,
)
URL_RE = re.compile(
    r"\b[a-z][a-z0-9+.\-]*://\S+"
    # A scheme-less link is as followable (`example.org/whitepaper.html`,
    # `www.sec.gov/files/form-d.html`): a dotted host whose last label is letters, then
    # `/`. A filename extension is never taken for that last label, so `deck.md/index.html`
    # and `a.md/b.html` stay internal paths.
    r"|(?<![\w.\-/])(?:[a-z0-9](?:[a-z0-9\-]*[a-z0-9])?\.)+"
    r"(?!(?:md|html|yaml|yml|toml|py|mjs)/)[a-z]{2,}/\S*",
    re.IGNORECASE,
)
SECTION_REF_RE = re.compile(r"§+\s*\d+(?:\.\d+)*[a-z]?", re.IGNORECASE)

# Presenter-only containers inside a slide: in the slide's text for the process-leak
# rule, but not in the AUDIENCE-visible text the filename and vocabulary rules read.
NOTES_CLASSES = {"notes", "speaker-notes", "speaker_notes", "presenter-notes"}

# H-CONTRAST-FLOOR
CONTRAST_FLOOR = 4.5
PROVENANCE_PARTS = {"prov", "provenance", "source", "sources", "src", "citation", "citations", "cite", "credit", "credits"}
FOOTER_PARTS = {"footer", "confidential", "confidentiality"}
NOTICE_PARTS = {"eyebrow", "kicker", "overline", "folio"}
NOTICE_NAMES = {"slide-no", "slide-num", "slide-number", "page-no", "page-num"}

# H-CONTRACT-VOCAB: the map lives in the staging skill beside this one (owned by
# presentation-skills/02). This linter reads it; it never keeps its own copy.
VOCAB_MAP = Path(__file__).resolve().parent.parent.parent / "staging" / "references" / "vocabulary-map.md"

MAX_ACCENT_PER_SLIDE = 2
DECKMD_COVERAGE_FLOOR = 0.85

ERROR = "error"
NOTICE = "notice"


# ---------------------------------------------------------------- findings + ignores


class Findings:
    """Flat accumulator. Errors fail the run; notices report and pass."""

    def __init__(self, config: dict) -> None:
        detector = (config or {}).get("detector", {}) or {}
        self.ignore_rules = {r.lower() for r in detector.get("ignoreRules", [])}
        self.ignore_values = [
            (str(e.get("rule", "")).lower(), str(e.get("value", "")).lower())
            for e in detector.get("ignoreValues", [])
            if e.get("reason")  # an entry with no written reason is not an ignore
        ]
        self.rows: list[tuple[str, str, str]] = []
        self.suppressed: list[str] = []

    def _suppressed(self, rule: str, value: str | None) -> bool:
        rule_l = rule.lower()
        if rule_l in self.ignore_rules:
            return True
        val_l = (value or "").lower()
        return any(r == rule_l and (v == val_l or v in val_l) for r, v in self.ignore_values)

    def add(self, severity: str, rule: str, message: str, value: str | None = None) -> None:
        if self._suppressed(rule, value):
            self.suppressed.append(f"{rule}: {message}")
            return
        self.rows.append((severity, rule, message))

    def error(self, rule: str, message: str, value: str | None = None) -> None:
        self.add(ERROR, rule, message, value)

    def notice(self, rule: str, message: str, value: str | None = None) -> None:
        self.add(NOTICE, rule, message, value)

    @property
    def errors(self) -> list[tuple[str, str, str]]:
        return [r for r in self.rows if r[0] == ERROR]

    @property
    def notices(self) -> list[tuple[str, str, str]]:
        return [r for r in self.rows if r[0] == NOTICE]


# ---------------------------------------------------------------- parser


class Slide:
    def __init__(self, index: int, attrs: dict) -> None:
        self.index = index
        self.attrs = attrs
        self.layout = (attrs.get("data-layout") or "").strip()
        self.question = (attrs.get("data-question") or "").strip()
        self.title = ""
        self.text_parts: list[str] = []
        self.visible_parts: list[str] = []  # text_parts minus presenter notes
        self.accent_hits = 0

    @property
    def text(self) -> str:
        return " ".join(" ".join(self.text_parts).split())

    @property
    def visible_text(self) -> str:
        """What the AUDIENCE sees: the slide's text without its presenter notes."""
        return " ".join(" ".join(self.visible_parts).split())


class DeckParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.html_lang = ""
        self.has_viewport = False
        self.title_text = ""
        self._in_title = 0
        self.deck_footer: str | None = None
        self.ids: list[str] = []
        self.images: list[tuple[str, str | None]] = []
        self.slides: list[Slide] = []
        self.folios: list[str] = []

        self._stack: list[tuple[str, int]] = []          # (tag, line) for unclosed-tag check
        self._slide_depth: int | None = None
        self._cur: Slide | None = None
        self._svg_depth = 0
        self._heading_depth: int | None = None
        self._folio_depth: int | None = None
        self._folio_buf = ""
        self._in_style = 0
        self._in_script = 0
        self._notes_depth: int | None = None
        self.css_parts: list[str] = []
        self.classes_used: set[str] = set()
        self.tags_used: set[str] = set()

    # -- helpers
    @staticmethod
    def _classes(values: dict) -> set[str]:
        return set((values.get("class") or "").split())

    def handle_starttag(self, tag: str, attrs) -> None:
        values = {k: (v or "") for k, v in attrs}
        classes = self._classes(values)

        if tag not in VOID_TAGS:
            self._stack.append((tag, self.getpos()[0]))
        self.classes_used |= classes
        self.tags_used.add(tag)

        if tag == "html":
            self.html_lang = values.get("lang", "").strip()
        elif tag == "meta":
            name = values.get("name", "").lower()
            if name == "viewport":
                self.has_viewport = bool(values.get("content", "").strip())
            elif name == "deck-footer":
                self.deck_footer = htmlmod.unescape(values.get("content", "")).strip()
        elif tag == "title":
            self._in_title += 1
        elif tag == "style":
            self._in_style += 1
        elif tag == "script":
            self._in_script += 1

        if values.get("id"):
            self.ids.append(values["id"])
        if tag == "img":
            self.images.append((values.get("src", ""), values.get("alt")))

        if tag == "svg":
            self._svg_depth += 1

        if "slide" in classes and self._slide_depth is None:
            self._cur = Slide(len(self.slides) + 1, values)
            self.slides.append(self._cur)
            self._slide_depth = len(self._stack)

        if self._cur is not None:
            # first heading inside a slide is its title
            if tag in {"h1", "h2"} and not self._cur.title and self._heading_depth is None:
                self._heading_depth = len(self._stack)
            # accent budget: class-driven accent uses, outside the brand mark
            if self._svg_depth == 0 and ({"kicker", "accent"} & classes):
                self._cur.accent_hits += 1
            # presenter notes: slide text, but not audience-visible text. Only an
            # explicit notes class counts — never `hidden`, which decks also use for
            # build steps the audience does see later.
            if (
                self._notes_depth is None
                and tag not in VOID_TAGS
                and self._slide_depth is not None
                and len(self._stack) > self._slide_depth
                and NOTES_CLASSES & classes
            ):
                self._notes_depth = len(self._stack)

        if ({"folio", "slide-no"} & classes) and self._folio_depth is None:
            self._folio_depth = len(self._stack)
            self._folio_buf = ""

    def handle_endtag(self, tag: str) -> None:
        if tag == "title" and self._in_title:
            self._in_title -= 1
        if tag == "style" and self._in_style:
            self._in_style -= 1
        if tag == "script" and self._in_script:
            self._in_script -= 1
        if tag == "svg" and self._svg_depth:
            self._svg_depth -= 1

        depth = len(self._stack)
        # unwind to the matching open tag
        for i in range(len(self._stack) - 1, -1, -1):
            if self._stack[i][0] == tag:
                del self._stack[i:]
                break

        if self._heading_depth is not None and depth <= self._heading_depth:
            self._heading_depth = None
        if self._notes_depth is not None and depth <= self._notes_depth:
            self._notes_depth = None
        if self._folio_depth is not None and depth <= self._folio_depth:
            self.folios.append(" ".join(self._folio_buf.split()))
            self._folio_depth = None
        if self._slide_depth is not None and depth <= self._slide_depth:
            self._slide_depth = None
            self._cur = None

    def handle_data(self, data: str) -> None:
        if self._in_title:
            self.title_text += data
        if self._in_style:
            self.css_parts.append(data)
            return
        if self._in_script:
            return
        if self._folio_depth is not None:
            self._folio_buf += data
        if self._cur is not None:
            self._cur.text_parts.append(data)
            if self._notes_depth is None:
                self._cur.visible_parts.append(data)
            if self._heading_depth is not None:
                self._cur.title += data

    @property
    def unclosed(self) -> list[tuple[str, int]]:
        return [t for t in self._stack if t[0] not in {"html", "head", "body", "p", "li"}]


# ---------------------------------------------------------------- helpers


def strip_style_script(raw: str) -> str:
    out = re.sub(r"(?is)<style.*?</style>", " ", raw)
    return re.sub(r"(?is)<script.*?</script>", " ", out)


def visible_words(raw: str) -> set[str]:
    text = re.sub(r"(?s)<[^>]+>", " ", strip_style_script(raw)).lower()
    text = htmlmod.unescape(text)
    return set(re.findall(r"[a-z][a-z0-9\-]{2,}", text))


def local_asset(deck_path: Path, source: str) -> Path | None:
    parsed = urlparse(source)
    if parsed.scheme in {"http", "https", "data", "mailto"} or source.startswith("#"):
        return None
    if parsed.scheme == "file":
        return Path(unquote(parsed.path))
    return (deck_path.parent / unquote(parsed.path)).resolve()


def parse_deck_md(path: Path) -> list[tuple[int, str]]:
    """`## S07 · <action title>` (also accepts an em dash or hyphen separator)."""
    rows: list[tuple[int, str]] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        m = re.match(r"^##\s+S(\d+)\s*[·—–-]\s*(.+?)\s*$", line)
        if m:
            rows.append((int(m.group(1)), m.group(2)))
    return rows


# ---------------------------------------------------------------- rules


def check_structure(f: Findings, raw: str, p: DeckParser) -> None:
    if not re.search(r"<!doctype\s+html", raw, re.IGNORECASE):
        f.error("S-DOCTYPE", "missing HTML5 doctype")
    if not p.html_lang:
        f.error("S-LANG", 'missing <html lang="…">')
    if not p.has_viewport:
        f.error("S-VIEWPORT", "missing viewport meta tag")
    if not p.title_text.strip():
        f.error("S-TITLE", "missing non-empty <title>")
    if len(p.slides) < 2:
        hint = (
            " — if slides are built by script, this static linter cannot see them; "
            "the deck-author contract requires static slide markup"
            if len(p.slides) == 0 and re.search(r"(?i)class(?:name)?\s*=+\s*['\"]?slide", raw)
            else ""
        )
        f.error("S-SLIDES", f'expected >=2 elements with class "slide", found {len(p.slides)}{hint}')

    dupes = sorted(i for i, n in Counter(p.ids).items() if n > 1)
    if dupes:
        f.error("S-DUP-ID", f"duplicate element ids: {', '.join(dupes)}")

    if p.unclosed:
        detail = ", ".join(f"<{t}> (line {ln})" for t, ln in p.unclosed[:6])
        f.error("S-UNCLOSED", f"unclosed tags: {detail}")


def check_footer(f: Findings, raw: str, p: DeckParser, brand: Brand | None) -> str | None:
    footer = p.deck_footer
    if not footer:
        if brand is None:
            f.notice(
                "H-FOOTER-META",
                "SKIP (no brand contract): no deck-footer meta — this deck carries NO "
                "confidentiality footer; give one via a brand contract if it needs one",
            )
            return None
        f.error(
            "H-FOOTER-META",
            'missing <meta name="deck-footer" content="…"> — the house PDF renderer '
            "hard-errors without it, by design",
        )
        return None
    if brand is None:
        f.notice(
            "H-FOOTER-CLASS",
            f"SKIP (no brand contract): footer wording {footer!r} not checked against a brand enum",
        )
    elif footer not in brand.footer_enum:
        f.error(
            "H-FOOTER-CLASS",
            f"deck-footer {footer!r} is not one of the brand's footer strings: "
            + " | ".join(repr(k) for k in brand.footer_enum),
            value=footer,
        )
        return footer

    if not re.search(r"\.slide::after\s*\{", raw):
        f.error("H-FOOTER-PRINT", "no `.slide::after` print rule — the footer will not land on every PDF page")
    else:
        block = re.search(r"\.slide::after\s*\{(.*?)\}", raw, re.S)
        content = re.search(r'content\s*:\s*"([^"]*)"', block.group(1) if block else "")
        printed = htmlmod.unescape(content.group(1)) if content else ""
        if printed.strip() != footer:
            f.error(
                "H-FOOTER-PRINT",
                f"print footer {printed!r} does not match the declared deck-footer {footer!r}",
            )
    return footer


def check_geometry(f: Findings, raw: str) -> None:
    # The winning house contract: a fixed 1280x720 board, letterboxed (scaled to fit),
    # so screen, screenshot, and PDF page share one composition. See the skill's
    # "six skeleton decisions" — this replaced the composite contender's viewport-fluid pin.
    required = {
        "H-GEOMETRY": [
            (r"width\s*:\s*1280px", "the fixed 1280px stage width"),
            (r"height\s*:\s*720px", "the fixed 720px stage height"),
            (r"transform\s*:[^;}]*scale\(", "the letterbox scale-to-fit transform on the stage"),
            (r"position\s*:\s*absolute", "absolutely-positioned slides"),
            (r"inset\s*:\s*0", "`inset: 0` on the slide box"),
            (r"@media\s+print", "@media print block"),
            (r"@page\s*\{[^}]*size\s*:\s*1280px\s+720px", "@page { size: 1280px 720px }"),
            (r"page-break-after\s*:\s*always|break-after\s*:\s*page", "one page per slide in print"),
        ]
    }
    for rule, patterns in required.items():
        for pattern, label in patterns:
            if not re.search(pattern, raw, re.IGNORECASE | re.S):
                f.error(rule, f"missing house geometry: {label}", value=label)


def check_fonts(f: Findings, raw: str, deck_path: Path, brand: Brand | None) -> None:
    if brand is None:
        f.notice("H-FONT-PIN", "SKIP (no brand contract): font-family declarations not pinned")
    else:
        for decl in re.findall(r"font-family\s*:\s*([^;}\n]+)", raw, re.IGNORECASE):
            for token in decl.split(","):
                name = token.strip().strip("'\"").lower()
                if not name or name.startswith("var("):
                    continue
                if name not in brand.font_allow:
                    f.error(
                        "H-FONT-PIN",
                        f"font-family {name!r} is outside the brand contract's pinned set "
                        f"({', '.join(sorted(brand.font_allow - GENERIC_FONTS)) or 'none declared'})",
                        value=name,
                    )

    remote = _remote_reporter(f, brand)
    for url in re.findall(r"url\(\s*['\"]?([^'\")]+)['\"]?\s*\)", raw):
        if urlparse(url).scheme in {"http", "https"}:
            remote(f"remote asset in url(): {url}", url)
            continue
        path = local_asset(deck_path, url)
        if path is not None and not path.is_file():
            f.error("H-ASSET-EXISTS", f"missing local asset: {path}")


def _remote_reporter(f: Findings, brand: Brand | None):
    """Remote assets are an error unless the brand contract explicitly allows them
    (rules.no_remote_assets = false → notice). No contract keeps the error: the
    offline requirement is structural — the render pipeline copies the file."""
    if brand is not None and not brand.no_remote:
        return lambda msg, value=None: f.notice("H-NO-REMOTE", msg, value)
    return lambda msg, value=None: f.error("H-NO-REMOTE", msg, value)


def check_offline(f: Findings, raw: str, deck_path: Path, p: DeckParser, brand: Brand | None) -> None:
    remote = _remote_reporter(f, brand)
    for attr, url in re.findall(r'\b(src|href)\s*=\s*"([^"]+)"', raw):
        if urlparse(url).scheme in {"http", "https"}:
            remote(f"remote {attr}: {url}", url)
    if re.search(r"@import\s+url\(\s*['\"]?https?:", raw, re.IGNORECASE):
        remote("remote @import in stylesheet")

    for src, alt in p.images:
        if alt is None:
            f.error("H-ALT", f"image missing alt attribute: {src}")
        path = local_asset(deck_path, src)
        if path is not None and not path.is_file():
            f.error("H-ASSET-EXISTS", f"missing image asset: {path}")


def check_slides(f: Findings, p: DeckParser, brand: Brand | None) -> None:
    accent_budget = brand.accent_budget if brand else MAX_ACCENT_PER_SLIDE
    for s in p.slides:
        tag = f"slide {s.index:02d}"
        if not s.layout:
            f.error("H-LAYOUT-POOL", f"{tag}: no data-layout declared")
        elif s.layout not in LAYOUT_POOL:
            f.error(
                "H-LAYOUT-POOL",
                f"{tag}: data-layout {s.layout!r} is outside the closed pool "
                f"({', '.join(sorted(LAYOUT_POOL))})",
                value=s.layout,
            )

        content_slide = s.layout not in CHROME_LAYOUTS
        title = " ".join(s.title.split())
        if content_slide:
            if not s.question:
                f.notice("H-QUESTION", f"{tag}: no data-question — what job does this slide do?")
            if title and len(title.split()) < 4:
                f.notice(
                    "H-ACTION-TITLE",
                    f"{tag}: title {title!r} smells like a topic label, not a takeaway",
                    value=title,
                )
            if not title:
                f.error("H-ACTION-TITLE", f"{tag}: no <h1>/<h2> title found")

        if s.accent_hits > accent_budget:
            f.notice(
                "H-ACCENT-BUDGET",
                f"{tag}: accent used {s.accent_hits}x (budget {accent_budget})",
            )

        text = s.text
        for needle, pattern in PROCESS_LEAK_RE:
            if pattern.search(text):
                f.error("H-PROCESS-LEAK", f"{tag}: process language rendered on a slide: {needle!r}", value=needle)
        for needle, pattern in PLACEHOLDER_RE:
            if pattern.search(text):
                f.error("H-PLACEHOLDER", f"{tag}: placeholder content on a slide: {needle!r}", value=needle)


def check_internal_filenames(f: Findings, p: DeckParser) -> None:
    """H-INTERNAL-FILENAME: a raw internal filename or style-guide section number on the
    audience-visible face. The remedy is a citation a reader can resolve, not a rename."""
    for s in p.slides:
        tag = f"slide {s.index:02d}"
        text = s.visible_text
        urls = [u.span() for u in URL_RE.finditer(text)]
        seen: set[str] = set()
        for m in [*INTERNAL_FILENAME_RE.finditer(text), *SECTION_REF_RE.finditer(text)]:
            token = m.group(0)
            if any(start <= m.start() < end for start, end in urls):
                continue  # part of a link a reader can follow
            if token in seen:
                continue
            seen.add(token)
            kind = "section reference" if token.startswith("§") else "internal filename"
            f.error(
                "H-INTERNAL-FILENAME",
                f"{tag}: {kind} {token!r} on the slide face — rewrite the citation for a reader "
                "(what the source is, who published it, when), not the file or section it lives in; "
                "the source trail belongs in the presenter notes or deck.md",
                value=token,
            )


# ---------------------------------------------------------------- contrast (static CSS read)

NAMED_COLOURS = {
    "black": (0, 0, 0, 1.0),
    "white": (255, 255, 255, 1.0),
    "transparent": (0, 0, 0, 0.0),
}


def _css_rules(css: str) -> list[tuple[str, str, str]]:
    """(at-rule context, selector, declarations) for every style rule, one nesting level
    of at-rules deep. A brace-depth scan, not a CSS parser: enough for declared colours."""
    css = re.sub(r"/\*.*?\*/", " ", css, flags=re.S)
    rules: list[tuple[str, str, str]] = []

    def scan(text: str, context: str) -> None:
        i, n = 0, len(text)
        while i < n:
            brace = text.find("{", i)
            if brace < 0:
                return
            prelude = text[i:brace].strip()
            depth, j = 1, brace + 1
            while j < n and depth:
                if text[j] == "{":
                    depth += 1
                elif text[j] == "}":
                    depth -= 1
                j += 1
            body = text[brace + 1 : j - 1]
            if prelude.startswith("@"):
                if re.match(r"@(media|supports|layer|container)\b", prelude, re.I):
                    scan(body, prelude)
            else:
                rules.append((context, prelude, body))
            i = j

    scan(css, "")
    return rules


def _declarations(body: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for decl in body.split(";"):
        if ":" in decl:
            name, value = decl.split(":", 1)
            out[name.strip().lower()] = re.sub(r"\s*!important\s*$", "", value.strip(), flags=re.I)
    return out


def _resolve_var(value: str, props: dict[str, str], depth: int = 0) -> str:
    if depth > 10:
        return value

    def sub(m: re.Match[str]) -> str:
        name, fallback = m.group(1).strip(), (m.group(2) or "").strip()
        return props.get(name, fallback)

    resolved = re.sub(r"var\(\s*(--[\w\-]+)\s*(?:,\s*([^()]*))?\)", sub, value)
    return resolved if resolved == value else _resolve_var(resolved, props, depth + 1)


def _parse_colour(value: str) -> tuple[int, int, int, float] | None:
    """First colour in a declaration value: #rgb[a], #rrggbb[aa], rgb[a]() or a few names."""
    m = re.search(r"#([0-9a-f]{8}|[0-9a-f]{6}|[0-9a-f]{3,4})\b", value, re.I)
    if m:
        h = m.group(1)
        if len(h) in (3, 4):
            h = "".join(c * 2 for c in h)
        alpha = int(h[6:8], 16) / 255 if len(h) == 8 else 1.0
        return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16), alpha
    m = re.search(r"rgba?\(\s*([^)]*)\)", value, re.I)
    if m:
        parts = [x for x in re.split(r"[\s,/]+", m.group(1).strip()) if x]
        try:
            rgb = [
                round(float(x[:-1]) * 2.55) if x.endswith("%") else round(float(x))
                for x in parts[:3]
            ]
            alpha = 1.0
            if len(parts) > 3:
                a = parts[3]
                alpha = float(a[:-1]) / 100 if a.endswith("%") else float(a)
        except ValueError:
            return None
        if len(rgb) == 3:
            return rgb[0], rgb[1], rgb[2], alpha
    m = re.search(r"hsla?\(\s*([^)]*)\)", value, re.I)
    if m:
        parts = [x for x in re.split(r"[\s,/]+", m.group(1).strip()) if x]
        try:
            hue = float(re.sub(r"deg$", "", parts[0], flags=re.I)) / 360 % 1
            sat, light = (float(x.rstrip("%")) / 100 for x in parts[1:3])
            alpha = 1.0
            if len(parts) > 3:
                a = parts[3]
                alpha = float(a[:-1]) / 100 if a.endswith("%") else float(a)
        except (ValueError, IndexError):
            return None
        r, g, b = colorsys.hls_to_rgb(hue, light, sat)
        return round(r * 255), round(g * 255), round(b * 255), alpha
    for word in re.findall(r"[a-z]+", value.lower()):
        if word in NAMED_COLOURS:
            return NAMED_COLOURS[word]
    return None


def _hex(rgb: tuple[int, int, int]) -> str:
    return "#{:02X}{:02X}{:02X}".format(*rgb)


def _luminance(rgb: tuple[int, int, int]) -> float:
    def channel(c: int) -> float:
        v = max(0, min(255, c)) / 255
        return v / 12.92 if v <= 0.04045 else ((v + 0.055) / 1.055) ** 2.4

    r, g, b = (channel(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast_ratio(fg: tuple[int, int, int], bg: tuple[int, int, int]) -> float:
    hi, lo = sorted((_luminance(fg), _luminance(bg)), reverse=True)
    return (hi + 0.05) / (lo + 0.05)


def _gate_for(selector: str, p: DeckParser, footer_rule: bool) -> tuple[str, str] | None:
    """(severity, what) if the selector's subject styles a required-content element
    that appears in this deck's markup, else None."""
    if footer_rule:
        return ERROR, "confidentiality footer"
    subject = re.split(r"\s*[>+~]\s*|\s+", selector.strip())[-1]
    subject = re.sub(r"::?[\w\-]+(\([^)]*\))?", "", subject)  # drop pseudo-classes/elements
    classes = set(re.findall(r"\.([\w\-]+)", subject))
    element = re.match(r"^([a-z][a-z0-9]*)", subject)
    element_name = element.group(1).lower() if element else ""

    def parts(name: str) -> set[str]:
        return set(re.split(r"[-_]", name.lower()))

    used = classes & p.classes_used
    if element_name in ("cite", "footer") and element_name in p.tags_used and not classes:
        return (ERROR, "provenance") if element_name == "cite" else (ERROR, "confidentiality footer")
    for c in used:
        if parts(c) & PROVENANCE_PARTS:
            return ERROR, "provenance"
    for c in used:
        if parts(c) & FOOTER_PARTS:
            return ERROR, "confidentiality footer"
    for c in used:
        if (parts(c) & NOTICE_PARTS) or c.lower() in NOTICE_NAMES:
            return NOTICE, "eyebrow/kicker/folio"
    return None


def check_contrast(f: Findings, p: DeckParser) -> None:
    """H-CONTRAST-FLOOR: a STATIC read of declared CSS, never a computed style."""
    rules = [r for r in _css_rules("\n".join(p.css_parts)) if "print" not in r[0].lower()]
    if not rules:
        return
    props: dict[str, str] = {}
    for _, _, body in rules:
        for name, value in _declarations(body).items():
            if name.startswith("--"):
                props[name] = value

    def colour_of(decls: dict[str, str], *names: str) -> tuple[int, int, int, float] | None:
        for name in names:
            if name in decls:
                c = _parse_colour(_resolve_var(decls[name], props))
                if c is not None:
                    return c
        return None

    # the declared backgrounds, by bare selector
    backgrounds: dict[str, tuple[int, int, int, float]] = {}
    for _, selector, body in rules:
        bg = colour_of(_declarations(body), "background-color", "background")
        if bg is not None and bg[3] > 0:
            for sel in selector.split(","):
                backgrounds[sel.strip()] = bg

    canvas_order = [".slide", ".stage", "body", "html", ":root"]
    assumed_white = "no declared page background — assumed the browser default white"

    def canvas() -> tuple[tuple[int, int, int], str]:
        for sel in canvas_order:
            bg = backgrounds.get(sel)
            if bg is not None and bg[3] >= 1:
                return bg[:3], sel
        return (255, 255, 255), assumed_white

    def nearest_bg(selector: str, decls: dict[str, str]) -> tuple[tuple[int, int, int], str]:
        own = colour_of(decls, "background-color", "background")
        base, where = canvas()
        if own is not None and own[3] > 0:
            return _blend(own, base), "its own rule"
        # walk the selector's ancestors, innermost first: `.card .prov` → `.card`
        chain = [c for c in re.split(r"\s*[>+~]\s*|\s+", selector.strip()) if c][:-1]
        for anc in reversed(chain):
            anc = re.sub(r"::?[\w\-]+(\([^)]*\))?", "", anc)
            if anc in backgrounds:
                return _blend(backgrounds[anc], base), anc
        # a pseudo-element paints over its own element's box
        host = re.sub(r"::?[\w\-]+(\([^)]*\))?$", "", selector.strip())
        if host != selector.strip() and host in backgrounds:
            return _blend(backgrounds[host], base), host
        return base, where

    footer_meta = p.deck_footer
    reported: set[str] = set()
    unresolved: set[str] = set()
    passed_on_assumed: list[str] = []
    for _context, selector_list, body in rules:
        decls = _declarations(body)
        if "color" not in decls:
            continue
        for selector in (s.strip() for s in selector_list.split(",")):
            footer_rule = bool(
                re.fullmatch(r"\.slide::?after", selector)
                and re.search(r'content\s*:\s*"[^"]+"', body)
                and (footer_meta is None or footer_meta in htmlmod.unescape(body))
            )
            gate = _gate_for(selector, p, footer_rule)
            if gate is None or selector in reported:
                continue
            fg = colour_of(decls, "color")
            if fg is None:
                # could-not-check, never a silent pass (hsl() parses; oklch(), lab(),
                # most named colours and an unresolved var() do not)
                if selector not in unresolved:
                    unresolved.add(selector)
                    f.notice(
                        "H-CONTRAST-FLOOR",
                        f"{selector} ({gate[1]}): colour {_resolve_var(decls['color'], props)!r} "
                        "is not resolvable by the static read — contrast NOT checked; use a "
                        "hex/rgb()/hsl() value or confirm it in the render-time review",
                        value=selector,
                    )
                continue
            bg, bg_from = nearest_bg(selector, decls)
            fg_rgb = _blend(fg, bg)
            ratio = contrast_ratio(fg_rgb, bg)
            if ratio >= CONTRAST_FLOOR:
                if bg_from == assumed_white and selector not in passed_on_assumed:
                    passed_on_assumed.append(selector)
                continue
            reported.add(selector)
            severity, what = gate
            message = (
                f"{selector} ({what}): contrast {ratio:.2f}:1 — {_hex(fg_rgb)} on {_hex(bg)} "
                f"(background from {bg_from}) is below the {CONTRAST_FLOOR}:1 floor. "
                "Static read of declared CSS only: it cannot see a cascade it does not model, "
                "an inline style, or a class the CSS never defines — the render-time "
                "computed-style review is the authority, and a pass here is not an accessibility pass"
            )
            f.add(severity, "H-CONTRAST-FLOOR", message, value=selector)
    if passed_on_assumed:
        f.notice(
            "H-CONTRAST-FLOOR",
            f"{', '.join(passed_on_assumed)}: passed only against an ASSUMED white canvas — no "
            f"background is declared on {'/'.join(canvas_order)} or on an ancestor in the "
            "selector, so a canvas painted on another wrapper is not modelled; contrast NOT "
            "confirmed — declare the canvas on .stage/.slide or confirm it in the render-time review",
        )


def _blend(c: tuple[int, int, int, float], base: tuple[int, int, int]) -> tuple[int, int, int]:
    """Composite a colour with alpha over an opaque base."""
    r, g, b, a = c
    if a >= 1:
        return r, g, b
    return tuple(round(a * x + (1 - a) * y) for x, y in zip((r, g, b), base))  # type: ignore[return-value]


# ---------------------------------------------------------------- contract vocabulary


def load_vocab_map(path: Path) -> list[tuple[str, str]]:
    """(banned token, replacement shape) rows from the vocabulary map's table.

    The map's contract: column 1 is the banned token, verbatim, in backticks. A row
    whose first cell does not START with a backticked token is prose (the map's
    catch-all "any other staging.yaml key name" row) and is not a token. The replacement
    shape is read from the column whose header starts "Replacement", not by position.
    """
    rows: list[tuple[str, str]] = []
    header: list[str] | None = None
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line.startswith("|"):
            if rows:  # the token table has ended
                break
            header = None
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        if header is None:
            if cells and cells[0].lower().startswith("banned token"):
                header = [c.lower() for c in cells]
            continue
        if all(re.fullmatch(r":?-+:?", c) for c in cells if c):
            continue
        m = re.match(r"`([^`]+)`", cells[0])
        if not m:
            continue
        shape_idx = next((i for i, h in enumerate(header) if h.startswith("replacement")), len(cells) - 1)
        shape = cells[shape_idx] if shape_idx < len(cells) else ""
        rows.append((m.group(1), shape))
    return rows


def _vocab_pattern(token: str) -> re.Pattern[str]:
    """Word-boundary, case-insensitive. Words separated by whitespace match any run of
    whitespace; around the pipeline's `·` delimiter the whitespace is optional."""
    words = token.split()
    body = re.escape(words[0])
    for prev, word in zip(words, words[1:]):
        body += (r"\s*" if "·" in (prev, word) else r"\s+") + re.escape(word)
    return re.compile(rf"(?<![\w\-]){body}(?![\w\-])", re.IGNORECASE)


def check_contract_vocab(f: Findings, p: DeckParser, map_path: Path = VOCAB_MAP) -> None:
    """H-CONTRACT-VOCAB: pipeline-internal staging vocabulary printed on a slide."""
    if not map_path.is_file():
        f.notice(
            "H-CONTRACT-VOCAB",
            f"SKIP: contract-vocabulary map not found at {map_path} — banned on-screen tokens "
            "NOT checked (this is not a pass; restore the staging skill's vocabulary-map.md)",
        )
        return
    vocab = load_vocab_map(map_path)
    if not vocab:
        f.notice(
            "H-CONTRACT-VOCAB",
            f"SKIP: no banned-token rows parsed from {map_path} — banned on-screen tokens NOT checked",
        )
        return
    patterns = [(token, shape, _vocab_pattern(token)) for token, shape in vocab]
    for s in p.slides:
        tag = f"slide {s.index:02d}"
        text = s.visible_text
        for token, shape, pattern in patterns:
            m = pattern.search(text)
            if not m:
                continue
            shown = m.group(0)
            f.error(
                "H-CONTRACT-VOCAB",
                f"{tag}: contract vocabulary {shown!r} on screen (banned token `{token}` in "
                f"{map_path.name}) — replace it with the thing that happened: {shape}",
                value=token,
            )


def check_folios(f: Findings, p: DeckParser) -> None:
    """Accepts either form: `.folio` "N / N" pairs, or the skeleton's `.slide-no` "NN"."""
    if not p.folios:
        f.error("H-FOLIO", "no `.folio` / `.slide-no` markers found — every slide needs its number")
        return
    pairs: list[tuple[int, int | None]] = []
    for text in p.folios:
        m = re.match(r"^(\d+)\s*/\s*(\d+)$", text)
        if m:
            pairs.append((int(m.group(1)), int(m.group(2))))
            continue
        m = re.match(r"^(\d+)$", text)
        if m:
            pairs.append((int(m.group(1)), None))
            continue
        f.error("H-FOLIO", f"unparseable folio marker: {text!r}")
        return
    n = len(p.slides)
    totals = {t for _, t in pairs if t is not None}
    if [c for c, _ in pairs] != list(range(1, n + 1)) or (totals and totals != {n}):
        f.error("H-FOLIO", f"folio sequence does not match {n} slides: {pairs}")


def check_deck_md(f: Findings, raw: str, p: DeckParser, deck_md: Path) -> list[str]:
    rows = parse_deck_md(deck_md)
    if not rows:
        f.error("P-DECKMD-COUNT", f"no `## S<N> · <title>` slide rows found in {deck_md.name}")
        return []
    if len(rows) != len(p.slides):
        f.error(
            "P-DECKMD-COUNT",
            f"{deck_md.name} declares {len(rows)} slides; index.html renders {len(p.slides)}",
        )
    rendered = visible_words(raw)
    title_words: set[str] = set()
    for _, title in rows:
        title_words |= set(re.findall(r"[a-z][a-z0-9\-]{2,}", title.lower()))
    if title_words:
        covered = len(title_words & rendered) / len(title_words)
        if covered < DECKMD_COVERAGE_FLOOR:
            missing = sorted(title_words - rendered)[:10]
            f.error(
                "P-DECKMD-COVER",
                f"only {covered:.0%} of deck.md title vocabulary appears in the render "
                f"(floor {DECKMD_COVERAGE_FLOOR:.0%}); missing e.g. {missing}",
            )
    return [t for _, t in rows]


def check_pure_tones(f: Findings, raw: str, brand: Brand | None) -> None:
    black_hint = (
        " — expected: this brand's contract pins the canvas at #000000; ignore with a reason"
        if brand and brand.canvas in {"#000", "#000000"}
        else " — universal dark-theme craft prefers a softened black; keep it only as a deliberate pin"
    )
    white_hint = (
        " — expected: this brand's contract pins text_primary at #ffffff; ignore with a reason"
        if brand and brand.text_primary in {"#fff", "#ffffff"}
        else " — consider a softened white unless the brand pins it"
    )
    if re.search(r"#000\b|#000000\b", raw, re.IGNORECASE):
        f.notice("C-PURE-BLACK", f"pure #000 present{black_hint}", value="#000000")
    if re.search(r"#fff\b|#ffffff\b", raw, re.IGNORECASE):
        f.notice("C-PURE-WHITE", f"pure #fff present{white_hint}", value="#ffffff")


# ---------------------------------------------------------------- main


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("deck", type=Path, help="the deck's index.html")
    ap.add_argument("--deck-md", type=Path, default=None, help="sibling deck.md (defaults to <deck-dir>/deck.md)")
    ap.add_argument(
        "--brand",
        default=None,
        help="brand contract (brand.toml) path, or 'none' to skip brand gates; "
        "default: auto-discover (deck dir up to repo root, then the skill's shipped default)",
    )
    ap.add_argument("--config", type=Path, default=None, help="scoped-ignore config (defaults to .deckcheck.json beside the skill)")
    ap.add_argument("--quiet", action="store_true", help="suppress the ghost deck")
    args = ap.parse_args()

    deck_path = args.deck.expanduser().resolve()
    if not deck_path.is_file():
        print(f"ERROR: deck not found: {deck_path}", file=sys.stderr)
        return 2

    config: dict = {}
    candidates = [args.config] if args.config else [
        deck_path.parent / ".deckcheck.json",
        deck_path.parent.parent / ".deckcheck.json",
    ]
    config_used = None
    for candidate in candidates:
        if candidate and candidate.is_file():
            try:
                config = json.loads(candidate.read_text(encoding="utf-8"))
            except json.JSONDecodeError as exc:
                print(f"ERROR: bad ignore config {candidate}: {exc}", file=sys.stderr)
                return 2
            config_used = candidate
            break

    try:
        brand, brand_source = resolve_brand(args.brand, deck_path)
    except RuntimeError as exc:
        if args.brand:  # an explicit contract that cannot be used is a hard error
            print(f"ERROR: {exc}", file=sys.stderr)
            return 2
        brand, brand_source = None, f"{exc} — brand gates SKIP"
    if brand is None:
        print(
            "NOTICE [BRAND] no brand contract in force"
            f" ({brand_source}); brand gates (footer wording, font pin) SKIP —"
            " structural checks still rule. A brand-neutral deck carries no"
            " confidentiality footer unless one is given."
        )

    raw = deck_path.read_text(encoding="utf-8")
    parser = DeckParser()
    parser.feed(raw)
    parser.close()

    f = Findings(config)
    check_structure(f, raw, parser)
    check_footer(f, raw, parser, brand)
    check_geometry(f, raw)
    check_fonts(f, raw, deck_path, brand)
    check_offline(f, raw, deck_path, parser, brand)
    check_slides(f, parser, brand)
    check_internal_filenames(f, parser)
    check_contrast(f, parser)
    check_contract_vocab(f, parser)
    check_folios(f, parser)
    check_pure_tones(f, raw, brand)

    deck_md = args.deck_md or (deck_path.parent / "deck.md")
    if deck_md.is_file():
        check_deck_md(f, raw, parser, deck_md)
    else:
        f.notice("P-DECKMD-COUNT", f"no deck.md beside the deck ({deck_md}) — story parity not checked")

    for severity, rule, message in f.errors:
        print(f"ERROR  [{rule}] {message}", file=sys.stderr)
    for severity, rule, message in f.notices:
        print(f"NOTICE [{rule}] {message}")

    if not args.quiet:
        print("\nGHOST DECK — read these as one paragraph; they must carry the argument alone:")
        for s in parser.slides:
            title = " ".join(s.title.split()) or "(no title)"
            print(f"  {s.index:02d} [{s.layout or '?'}] {title}")
        print()

    if f.suppressed:
        print(f"SUPPRESSED: {len(f.suppressed)} finding(s) via {config_used}")

    if f.errors:
        print(
            f"FAIL: {len(f.errors)} error(s), {len(f.notices)} notice(s) "
            f"across {len(parser.slides)} slides",
            file=sys.stderr,
        )
        return 1

    if brand is not None:
        footer_class = brand.footer_enum.get(parser.deck_footer or "", "unknown")
        brand_bit = f"brand {brand.org or brand.path.name} ({brand_source}); footer class {footer_class}"
    else:
        brand_bit = "brand NONE — brand gates skipped"
    print(
        f"PASS: {len(parser.slides)} slides; {brand_bit}; "
        f"{len(parser.images)} images; {len(f.notices)} notice(s); contract present"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
