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
| Act blocks paste safely in zsh (hard) | every act block (an `sh`, `bash`, `zsh` or `shell` fence defining a `driver_act…()` function) in a `*.md` under `plugins/` | `actblock.go` |

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

A copy has no end marker, so sync proves where each copy ends by content: the
lines at the copy's anchor must equal the current canonical text (already
synced, no write) or the block's text in an earlier committed or staged
revision of the source. When nothing matches, as with a hand-edited copy or a
tree with no git history, sync reports could-not-check and leaves the file
alone. If you will edit the source again before committing, `git add` it after
each sync so the next sync can match the copies it wrote. Otherwise restore the
sites with `git checkout` and sync once.

Two narrower cases stay content-limited even so.

The first is genuinely **ambiguous**, not merely "an older revision": when the
longest known text matching at the anchor is not also the newest one matching
there. Two matching texts always nest, one a prefix of the other, so this
means a newer text of the block (the current canonical text counts as the
newest) is a strict prefix of an older one: the block once shrank by dropping
trailing lines, and the copy still matches both sides of that shrink. The
bytes then cannot tell a copy still genuinely at the older, longer text apart
from a copy at the newer, shorter text followed by unrelated content —
possibly a local, site-specific rule someone added right after the block —
that happens to equal the longer text's own tail. Any trailing-line removal is
such a shrink, so the first sync after one is ambiguous. `git add` does **not**
prevent it — the tie comes from history, not from anything staging can fix —
so sync **refuses by default**: could-not-check, naming the file, the two
lengths that matched, and the exact line-range span the longest-match rule
would have removed. That block is not written; another, unambiguous block in
the same file still is.

The refusal is not one-off. Once such a shrink is committed, every later sync
of that block refuses too, whatever the later edit, for as long as the copy
still matches both texts — for example while a dropped line is kept as
site-local text right under the block. To get past it, either verify the
ambiguity by hand (`git diff` on the site) and re-run with
`--allow-ambiguous-extent`, which takes the longest match for **every**
ambiguous block in that run and records each as a `note:` on `--sync`; or
separate the site-local text from the block (for example with a blank line)
so the copy no longer matches the older text.

A block that only ever **grew** is not ambiguous. After a committed
append-grow the older, shorter text is a prefix of the newer one, so both match
at a synced copy, but the longest match is also the newest, which is what a
synced copy holds; later edits of that block rewrite normally. What this
accepts: content right under a copy that exactly equals the lines a later grow
added is treated as part of the block, and a later edit replaces it. That
happens by two routes. One is a copy that missed a sync, still at an older text,
and so at least two revisions behind its source. The other is more ordinary: a
site-local line that a later grow promotes, verbatim, into the canonical block.
From that grow on, the copy is byte-identical to the grown text, so the check
mode reports it as synced and nothing about it looks stale. A later edit of the
block then replaces that line, which is defensible because it became canonical,
but it is no longer the site's own text.

Earlier revisions of this document said `git add` closed the ambiguity window
entirely, and called a committed prefix-shrink "common, unambiguous" — both
statements were wrong. A later revision treated any two matching lengths as
ambiguous, which refused every edit of a block that had ever grown; that is
wrong too. This default (refuse, with an explicit opt-in) and the recency
condition are this project's own reversible choices, not a settled
cross-project ruling (#1692).

The second is narrower still: a shrink of an edit that is never committed or
staged is could-not-check only when nothing at the anchor matches; if the
shrunk text still matches as a prefix of what is on disk, the copy reads as
already synced and content the shrink dropped, but never registered anywhere
sync can see, is left in place rather than guessed away.

Every site file a `--sync` run touches is read at most once, and that same
read is what both the proven extent and the eventual write are built from;
immediately before writing, the file is re-read and compared against that
original read, and the write itself goes through a temp file plus atomic
rename rather than an in-place truncate-then-write. This closes a
read-compute-write race that let concurrent `--sync` runs corrupt a file
(medici-finance/assay#1692).

### 5. Act blocks paste safely in zsh

An act block is the fenced shell block the `ask-decision` skill's §Act tells a
desk to hand the driver: pasting it must only print what it would do. zsh, the
default macOS login shell, does not treat `#` as a comment at an interactive
prompt unless `interactive_comments` is set, and it is unset by default. There a
comment line is part of a command: a `;` ends it, and a backtick span or `$(...)`
after it runs. The same goes for text after a `#` on a code line. Even a
plain-text comment line is a command named `#` that fails, and that failure is
not inert where a status is read: after `&&` or `||`, in an `if`, `elif`,
`while` or `until` condition, or last in a tested group or function, it can
change which branch runs. After a line that ends in a backslash its text joins
the command before it, in every shell.

The control on a block's first paste is the comment rule: plain text, and in
the header only. A terminal
hands zsh a paste as one bracketed paste, and zsh reads all of it before running
its first line, so the zsh comment guard
`[ -n "${ZSH_VERSION-}" ] && setopt interactive_comments` covers only later
pastes and lines typed after it, never the paste that carries it. The
`ACT-BLOCK` check holds every act block under `plugins/` (an `sh`, `bash`, `zsh`
or `shell` fence that defines a function named `driver_act…`) to five rules:

1. every full-line comment holds only letters, digits, spaces, tabs and
   `. , : - / _ + = #`, indented or not, and no code line carries a trailing
   `#` comment, so a comment's words run nothing even where zsh reads it as a
   command (rule 2 keeps its failing status from mattering).
   Two checks find a trailing comment, and a line either one flags fails.
   The guarantee is the per-line floor: it flags any `#` straight after a
   blank, a tab, a carriage return or one of `;`, `&`, `|`, `(`, `)`, `<`,
   `>`, a backtick, `!` or `-` on a code line. Those characters end every bash
   and zsh operator (the `-` of `>&-`, the `!` of zsh's `>!` and `&!`
   included), and the floor reads no quote, span or heredoc state, so no
   misreading can hide such a `#`. It flags one inside quotes or a heredoc body
   too (`echo "a;#b"` fails; write it another way). So `echo dry;#;echo live`
   and `echo dry >&-#;echo live` are flagged (a first zsh paste runs
   `echo live`), while `a#b`, `${#T}`, `${T#x}`, `$#`, `$((16#ff))` and `\#`
   are not. A scanner, a best-effort reader rather than a shell parser, reads
   the block whole: it adds a `#` that starts a word on a continuation line,
   and it fails the block on the constructs where its reading could part from
   a shell's. It models single quotes, double quotes, `$'...'` (with backslash
   escapes), `$"..."`, `$( )` with nested `( )`, arithmetic (`$(( ))`, `(( ))`,
   `$[ ]`), `${ }`, `$$`, backtick spans, backslash escapes and line
   continuations, here-strings (`<<<`), and heredocs (`<<WORD` and `<<-WORD`,
   `WORD` bare or quoted, several on one line): a quoted-delimiter body is
   data, and an unquoted body is read like a double-quoted string, so `$( )`
   and backticks in it are code. Text in a comment opens no quote. It fails
   the block on: a block, or an unquoted heredoc body, that ends inside a
   quote, a span, a heredoc with no closing line, or a line continuation;
   `\'` inside `$'...'` (dash ends the quote there); a `'` inside `${ }`; the
   word `case` inside `$( )`; a `<<` with no delimiter word, or with a `$` or
   a backtick in it (`<<$'EOF'`); a heredoc whose `$( )` or backtick span
   closes on its line (`x=$(cat <<EOF)`), or whose line ends inside a span
   opened after it (`cat <<EOF $(`); a `<<` in arithmetic (`$((1<<2))`,
   `(( x << 2 ))`); a `(( ))` or `$(( ))` closed by a single `)`; and a `${`
   followed by a blank or `|` (bash 5.3's `${ cmd; }`). It does not follow
   `eval`, `sh -c` or aliases;
2. a full-line comment stands only in the header: every non-blank line before
   it is the guard line or another `#` line. Anywhere else, inside the act
   function, after the call, or after any code line, it is refused, and so is
   a `#`-led line inside a multi-line quote or heredoc body, which errs strict.
   In the header only the guard comes before it, so no status reads the
   comment's failure and no continuation reaches it. A step inside the act
   function opens with `echo '<n>. <text>'` instead;
3. the block's first non-blank line is the zsh comment guard;
4. the act function has a per-act name, `driver_act_<id>`, never the bare
   `driver_act`, so a block that fails to parse leaves no earlier act's
   function under the name the driver is told to type;
5. every `read` is the whole safe shape on one line,
   `NAME=; read -rs NAME || exit N` or `NAME=; read -rs NAME || { ...; exit N; }`:
   the clear comes first in command position (never after `&&`, `||` or a
   pipe), the read carries `-r` and `-s` and no other option, and the failure
   branch ends with `exit N`, `N` from 1 to 255, closing the list. The shape
   must run in the act function's own shell: not inside a subshell, `$( )`,
   backticks, a pipeline or a background job, nor in another function, whether
   that opens on the read's line or on another one. A shell whose `read` has no
   `-s` fails without assigning, so an inherited value would pass as the
   secret, and an `exit` in a child shell ends only that child. Any word that
   is `read` once quotes and backslashes are removed (`read`, `\read`,
   `"read"`, `r''ead`) counts as a read wherever it sits, so the rule errs
   strict.

A violation is exit 1, naming the file and line. Finding no act block at all is
could-not-check (exit 2), never a pass: the `ask-decision` example must exist.
The check reads only the examples the plugin ships. An act block a desk writes
at run time gets no lint. Its fence finder matches the opening character and a
closing run at least as long, but applies no indentation or list rule, so a
four-space-indented example is still checked, which errs strict. The read rule
tokenizes the block whole (quotes, backslashes and line continuations, `$( )`,
backticks, `( )`, `{ }`, pipes, `&` and the compound commands), so a subshell or
pipe that opens or closes on another line is seen. It does not see a read run
through `eval`, `sh -c` or a command name built from an expansion, and it takes
a full line that starts with `#` as a comment even inside a multi-line quoted
string; rule 2 then refuses that line unless it sits in the header.

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
