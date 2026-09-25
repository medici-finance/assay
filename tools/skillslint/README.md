# skillslint

Four offline checks over the plugin tree, run as one command. Exit code 0 clean,
1 a real violation, 2 could-not-check — and 2 is a failure, never a quiet pass.
One of the four (the context-budget NOTICE) is advisory and never moves the exit
code; the other three are gating.

```
make skillslint                      # the check form (runs with --root ../..)
make guardrail-sync                  # regenerate every guardrail copy
cd tools/skillslint && go run . --root ../..
cd tools/skillslint && go run . --skills-dir <dir>   # adopter reach: structural + conformance ONLY
cd tools/skillslint && go test ./... -count=1
```

`tools/skillslint` is its own Go module, so it is built and tested from inside its
own directory (`cd tools/skillslint && go test ./...`), not from the repo root.

## What it checks

| Check | Scope | Source |
|---|---|---|
| Skill-file structure | `plugins/assay/skills/*/SKILL.md` | `lint.go` |
| Per-skill frontmatter conformance limits (hard) + soft body/bundle budgets (advisory) | `plugins/assay/skills/*/SKILL.md`, or `<dir>/*/SKILL.md` under `--skills-dir` | `conformance.go` |
| Invisible-character / Trojan-Source (hard) + context-budget NOTICE (advisory) | the instruction surfaces (below) | `hidden.go` |
| Unresolved house values | **every `*.md` under `plugins/`** | `housevalue.go` |
| Shared-guardrail derive-or-diff | every declared guardrail copy | `guardrail.go` |

### 1. Skill-file structure

Per `SKILL.md`: the frontmatter block is present, **loads as a YAML mapping**,
carries a `name:` that is a non-empty string equal to the directory name and a
`description:` that is a non-empty string, and the body makes no bare
"unforgeable" / "tamper-evident" overclaim about a review, App or gate.

Parsing is a real YAML load (`gopkg.in/yaml.v3`), not a line scan. It used to be
a line scan, and that let two shipped skills carry frontmatter no YAML parser
could load — a plain `description:` scalar containing a colon-space, which YAML
reads as a nested mapping — while this lint reported PASS on both (#1115). A
harness that builds its skill roster by loading that document sees no name and
no description, so the skill never surfaces.

The repair for such a description is a folded block scalar:

```yaml
description: >-
  Load ONLY on an explicit desk-boot request: the user types `/the-desk`.
```

Keep the description text byte-identical and change only its quoting — it is
adopter-facing trigger text the harness matches on, so rewording it to dodge the
colon changes behaviour that the lint was never asking to change.

### 1a. Per-skill frontmatter conformance limits

The structural check above asks whether `name:` and `description:` are present
and readable; it says nothing about their LENGTH or SHAPE. Two shipped skills
(`install`, `pr-review-desk`) exceeded the 1024-character description limit
both the [agentskills specification](https://agentskills.io/specification) and
the Codex CLI enforce, and this lint reported PASS on both — the gap
`harness-portability/17` closes.

**Hard limits (exit 1):**

| Limit | Bound | Source |
|---|---|---|
| `description` length | ≤ 1024 Unicode characters, counted with `utf8.RuneCountInString` (never bytes) | agentskills `description`; Codex `MAX_CATALOG_SKILL_DESCRIPTION_CHARS` truncates a description at 1021 chars + `"..."`; an older Codex CLI refuses to load the skill at all ([openai/codex#13941](https://github.com/openai/codex/issues/13941)) |
| `name` length | ≤ 64 characters | agentskills `name` |
| `name` pattern | `^[a-z0-9]+(-[a-z0-9]+)*$` | agentskills `name` (lowercase letters/digits, hyphen-separated, no leading/trailing/consecutive hyphen) |

`name == directory` and the strict-YAML-load checks above are unchanged and are
not duplicated here — this half only bounds the length/shape of values those
checks already require to be present and readable.

**Soft budgets (advisory NOTICE, stderr, never move the exit code):**

| Budget | Bound | Source |
|---|---|---|
| Body size | > 8000 bytes | Codex truncates an agent-plugin skill body past `MAX_SKILL_PROMPT_BYTES` |
| Body length | > 500 lines | agentskills: "keep your main SKILL.md under 500 lines" |
| Body size (approx tokens) | > 5000 tokens, at 4 bytes/token (Codex's own `APPROX_BYTES_PER_TOKEN`) | agentskills: "< 5000 tokens recommended" |
| Bundle description total | summed description characters across every linted skill > 8000 | Codex's skills-list budget when the model's context window is unknown (`DEFAULT_SKILL_METADATA_CHAR_BUDGET`) |

**The budget ruling (recorded in `harness-portability/17`'s brief, reversible).**
The bundle-wide budget is a NOTICE, not a failure: the 8000-character figure
only applies when the context window is unknown — a known window instead gets
2% of it in tokens, a much larger figure for any window Codex plausibly runs —
and when it does bind, Codex degrades by shortening descriptions rather than
refusing. Cutting trigger text from every skill to satisfy a fallback path
would harm triggering on every harness for a soft, degrading limit. Only the
per-skill HARD limits above — where a harness truncates or refuses one skill
outright — gate the build.

**Adopter reach — `--skills-dir <dir>`.** The structural check and this
conformance half are the only two of skillslint's checks that generalize past
this repo's own fixed `plugins/assay/skills/` layout (the house-value,
hidden-character, guardrail and enforcement-block halves all check THIS repo's
own tree). `--skills-dir <dir>` (repeatable) runs ONLY those two checks over
`<dir>/*/SKILL.md` and exits 0/1/2 the same way `--root` does; zero matched
files is exit 2, never a quiet pass.

### 2. Invisible-character / Trojan-Source lint + context-budget NOTICE

The structural check above reads the header the way the harness does; it says
nothing about the raw bytes underneath. A skill or instruction file can carry a
Unicode payload — a bidi override that reorders how a line *renders*, a zero-width
joiner spliced mid-word, a stray control character — that a human reviewing the
rendered text cannot see, yet the model reads on activation. This half reads a
different signal (the bytes) so the layer catches what human review is built to
miss.

**Hard (exit 1), by Unicode category — never an enumerated blacklist** (an
enumeration inevitably misses a member of the class):

- The **whole `unicode.Cf` format category.** This subsumes every invisible
  formatting vector at once: bidi controls (U+202A–U+202E, U+2066–U+2069) *and*
  the directional marks the Trojan-Source family also uses (LRM U+200E, RLM
  U+200F, ALM U+061C); zero-width (U+200B–U+200D, U+2060); the invisible math
  operators (U+2061–U+2064); the soft hyphen (U+00AD); and the Unicode **Tag
  block** (U+E0001, U+E0020–U+E007F — the canonical LLM ASCII-smuggling vector).
  U+FEFF is Cf too and is rejected **except** as the file's leading BOM.
- **Variation selectors** (U+FE00–U+FE0F, U+E0100–U+E01EF, and the Mongolian free
  variation selectors U+180B–U+180D, U+180F). These are `Mn`, not `Cf`, so they
  are covered explicitly — a run of them appended to a carrier glyph is the
  variation-selector steganography channel.
- The **assigned Default_Ignorable_Code_Point property** (`defaultIgnorable`), as
  its own property-based branch — the *durable basis* of the control. The DI
  property is Unicode's own definition of "a renderer may show this as nothing",
  i.e. the invisible/zero-width class itself; targeting the whole property (not an
  enumeration of the codepoints seen so far) closes the class so an adversarial
  probe finds nothing. It is a curated table because Go ships no stdlib DI
  RangeTable and no single `General_Category` equals DI (members span Cf, Mn, Lo,
  Zl, Zp). **Bar: assigned DI only** — unassigned/reserved DI (U+2065,
  U+FFF0–FFF8, U+E0000, and the reserved tag/VS-supplement gaps) is deliberately
  left legal: it carries no payload today and rejecting reserved space is churny.
- A curated **`otherInvisibles`** set that renders to nothing yet is neither Cf,
  VS nor Cc: U+034F combining grapheme joiner, the Hangul fillers (U+115F, U+1160,
  U+3164, U+FFA0), the Khmer inherent vowels (U+17B4, U+17B5), U+2800 braille
  blank, and the line/paragraph separators (U+2028, U+2029). `invisible ⊆ Cf ∪ VS
  ∪ Cc` is false; this closes it. It is a codepoint list, not a category, because
  no single Unicode category means "invisible".
- Any **C0/C1 control** outside `\t \n \r`, and **invalid UTF-8**.

**Deliberately legal — Zs space separators** (the ordinary space, U+00A0 NBSP,
U+2000–U+200A, U+202F, U+205F, U+3000): these are *visible* whitespace, not an
invisible-smuggling class, and rejecting them would false-positive on every
ordinary space. Left out by decision, not omission.

Each violation names file, line, column and codepoint (`U+202E RIGHT-TO-LEFT
OVERRIDE`). Printable non-ASCII — accented names, arrows, box drawing, an emoji
whose base glyph carries its own presentation — stays legal: the check targets
invisibility, not foreignness. One consequence worth stating: an emoji written
with an explicit variation selector (e.g. `⚠️` = U+26A0 U+FE0F) is flagged; the
fix is the base glyph alone (`⚠`).

**Advisory (never exit-affecting): a context-budget NOTICE.** A file over a word
threshold (`SKILL.md` 3,000, `CLAUDE.md` 5,000) prints
`skillslint: NOTICE: <path>: <n> words (budget <t>) — context-bloat candidate` to
stderr; larger instruction files correlate with more hallucination, so it is worth
a human's eye. It is a judgment call, so it stays advisory and moves no exit code.

**Scope — the instruction surfaces:** every `*.md` under `plugins/assay/skills/`
and under `.claude/skills/`, plus `plugins/assay/resident-rules.md` and a top-level
`CLAUDE.md`, wherever each exists in the linted root.

### 3. Unresolved house values — the WHOLE plugin tree

The plugin ships more adopter-facing prose than skill bodies: the harness
references under `plugins/assay/references/`, the per-directory READMEs, the
command docs. All of it is read by repos that are not this house, so all of it
must name the driver with the neutral `human:<name>` token (or a
`capability:<name>` binding) rather than resolving it to whoever drives it here.

**Scope: every `*.md` under `plugins/`, at any depth.** It used to be the skill
bodies alone, which is how a resolved house value sat in a reference file and
passed lint — found by a reviewer, not by CI (#236). Widening the walk is #238.

**Neutral by construction.** The check carries no list of real names; it could
not ship to adopters if it did, and a name list is the artefact `human:<name>`
exists to abolish. It detects the *shape* — a proper-name-shaped token standing
in a **driver position**. Three positions are recognised, because they are the
three the corpus uses for the neutral token:

| Position | Neutral | Violation |
|---|---|---|
| dated attribution | `(human:<name>, 2026-07-20)` | `(Somebody, 2026-07-20)` |
| possessive | `human:<name>'s ruling` | `Somebody's ruling` |
| driver lead-in | `driver human:<name>` | `driver Somebody` |

The dated attribution may wrap across one line break (the commonest real shape);
it is reported on the line the **name** is on.

Two things are out of scope by shape, not by allowlist: a single letter
("track B's") and an all-caps identifier (`PR`, `CI`, `README`, `R6`). Stated
limitation: a name shouted in all caps is therefore not detected — the
alternative is teaching the tool which capitalised words are people, which is
exactly the name list it must not carry. That residue is a review's to catch.

**The allowlist.** Genuine product, tool, platform and project nouns
(`Cursor`, `GitHub`, `Claude Code`, `App`, …) live one-per-line in
[`driver-allowlist.txt`](driver-allowlist.txt), checked in next to the tool and
embedded into the binary with `go:embed`. Embedding is deliberate: the file
belongs to the *tool*, not to the `--root` being linted, and a lint that cannot
find its own data file must not degrade to a quiet pass. Editing it is still a
one-line edit to one checked-in file.

A **person's name never belongs in the allowlist.** The fix for a person's name
in the driver position is `human:<name>`; an allowlist entry that names a human
defeats the check and is a review finding.

**Report shape:** `file:line` plus the offending span and the position that
matched, on stderr, the same shape as the other two checks:

```
skillslint: plugins/assay/references/example.md: line 9: "Somebody" (dated attribution: "Somebody, 2026-08-26") is a proper-name-shaped token in the driver position — …
HOUSE-VALUES: FAIL — 1 unresolved house value(s) across 22 markdown file(s) under plugins/
```

### 4. Shared-guardrail derive-or-diff

Any rule more than one skill must state verbatim has one declared home,
`.claude/guardrails/GUARDRAILS.md`. This half byte-diffs every copy against it
(`make skillslint`) and regenerates them (`make guardrail-sync`). Edit the
source, never a copy.

## Fixtures

`testdata/plugintree/` holds a matched pair of fake roots:

- `unresolved/` — a reference and a README carrying a **placeholder** proper name
  in all three driver positions. The lint must fail on it.
- `neutral/` — the same files byte-for-byte, with `human:<name>` in place of that
  token. The lint must pass on it.

The pair is the positive control: a test pins them to differ by the token alone,
so the red arm cannot start passing for reasons that have nothing to do with the
name. Both fixtures also carry the legitimate capitalised words
(`Cursor's`, `GitHub's`, `Claude Code's`, `track B's`, `(R6, 2026-07-10)`) that
must never be reported.

`testdata/conformance/` holds the `--skills-dir` fixtures for the frontmatter
conformance limits, each a directory of one or more `<name>/SKILL.md` skills:

- `desc-1025/` — one skill, an ASCII description of 1025 characters. Must fail.
- `desc-1024-multibyte/` — one skill, a 1024-character (≥ 2048-byte) description.
  Must pass — characters, not bytes, are counted.
- `name-mismatch/` — one skill whose `name:` does not equal its directory. Must
  fail (the existing name==dir check, unrelated to conformance).
- `name-pattern/` — one skill named `Bad--Name` (uppercase, consecutive hyphen).
  Must fail the agentskills name pattern.
- `budget-over/` — nine valid skills whose descriptions individually stay under
  the 1024-character hard limit but sum past the 8000-character bundle budget.
  Must pass (exit 0) with a bundle NOTICE — the budget is advisory.

## Not wired into a workflow

No workflow in `.github/workflows/` calls this tool today; it runs as
`make skillslint`. Wiring the gate is tracked separately
(`docs/archive/mistake-proofing/brief-04-derived-enforcement-status.md`), and
widening the check does not change that.
