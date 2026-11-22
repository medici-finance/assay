# The brand contract — `brand.toml`

Deck-author is parameterized on a **brand contract**: one TOML file that
carries everything brand-specific the skill and its linter need. The skill's
core is brand-neutral; a brand is an *instance* of this schema. Supply a contract from the project or pass an explicit path.

TOML because the linter must stay stdlib-only: `tools/check_deck.py` parses
the contract with `tomllib` (python ≥ 3.11) and takes no dependency to do it.

## Resolution order

Stated once here, implemented identically in SKILL.md and `check_deck.py`:

1. An **explicit path** given by the user/caller (`--brand <path>` to the
   linter; a named file in the brief to the skill).
2. **`brand.toml` at the consuming repo's root** — the linter walks up from
   the deck directory to the nearest `.git` (or the filesystem root).
3. **No contract at all** (`--brand none`, or nothing discoverable): the
   linter's brand gates downgrade to SKIP-with-notice; structural checks
   still rule. The skill runs its abbreviated interview or proceeds
   brand-neutral (see SKILL.md) — and a brand-neutral deck carries **no
   confidentiality footer** unless one is given.

## Schema, annotated

Every key shown; optional keys marked. Paths are relative to the directory
holding the `brand.toml`.

```toml
[identity]
org = "Example Studio"                      # the organisation name, as rendered
# The confidentiality footer strings — the closed enum the linter pins the
# <meta name="deck-footer"> against. The footer wording is the deck-class
# marker: internal vs external. Either may be omitted if the brand has no
# such class, but a brand with neither ships decks with no footer gate.
footer_internal = "Example Studio — Internal · Confidential"
footer_external = "Example Studio — Proprietary & Confidential"

[type]
sans = "IBM Plex Sans"              # font FAMILY names — the linter allowlists
mono = "IBM Plex Mono"              # these plus the generic fallback stack
# extra_families = ["…"]            # optional: additional allowed families
# Relative paths to the font files the skeleton @font-face's. State variable
# vs static in the licence/notes — mis-declaring a variable font's weight
# range makes browsers synthesise bold (the silent-failure trap).
sans_files = ["fonts/ipx-sans.woff2", "fonts/ipx-sans-italic.woff2"]
mono_files = ["fonts/ipx-mono-400.woff2", "fonts/ipx-mono-500.woff2"]
licence = "IBM Plex, © IBM Corp., SIL OFL 1.1 — see fonts/NOTICE.md"

[palette]                           # named hex values, all optional but
canvas = "#000000"                  # canvas/surface/text/accent are what the
surface = "#0A0A0A"                 # skeleton derivation (SKILL.md) consumes
code_surface = "#060606"            # optional
hairline = "#1A1A1A"
hairline_2 = "#252525"              # optional second hairline tier
text_primary = "#FFFFFF"            # text tiers: primary / secondary / muted
text_secondary = "#AAAAAA"
text_muted = "#666666"
accent = "#3366FF"                  # THE accent — the one frequent colour
accent_light = "#5C85FF"            # optional accent tints
accent_dark = "#1A4DCC"
good = "#00CC66"                    # functional colours — used only
bad = "#FF4444"                     # functionally (pass/fail/warn),
warn = "#FFCC00"                    # never decoration

[marks]
# Where the brand's marks live: paths to mark SVGs, or the skeleton filename
# when the marks ship as an inline <symbol> library (the Example Studio pattern —
# plate-stripped symbols, because the raw brand files are app avatars whose
# baked-in plate reads as a grey box on a dark slide).
symbols = "deck-skeleton-external.html"
# marks = ["marks/logo.svg", …]     # optional: standalone mark files
usage = "Gold appears ONLY inside the mark; watermark at opacity .05."
# Mark-own colours exempt from the accent rule: the mark may carry them,
# slide content may not.
exempt_colors = ["#D4A843"]

[rules]
accent_budget = 2                   # max visible accent uses per slide
gradients = false                   # gradient text/fills allowed?
no_remote_assets = true             # url(http…)/src=http… is an error when true
```

## What consumes which section

| Section | SKILL.md (skeleton derivation, render rules) | check_deck.py |
|---|---|---|
| `[identity]` | footer wording on every slide + the `deck-footer` meta | `H-FOOTER-CLASS` enum |
| `[type]` | `@font-face` block, heading/body faces | `H-FONT-PIN` allowlist |
| `[palette]` | the token block (`:root` custom properties) | pure-tone notices contextualised |
| `[marks]` | symbol library, watermark, mark placement | accent exemption context |
| `[rules]` | accent discipline, gradient/shadow bans | `H-ACCENT-BUDGET` budget, `H-NO-REMOTE` severity |

## Writing a new instance

Place the contract at the project root or pass its path with `--brand`. Keep assets with the contract. Use the same fonts, colors and footer wording in the HTML and its export.
