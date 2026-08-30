# skillslint

Three offline checks over the plugin tree, run as one command. Exit code 0 clean,
1 a real violation, 2 could-not-check — and 2 is a failure, never a quiet pass.

```
make skillslint                      # the check form (runs with --root ../..)
make guardrail-sync                  # regenerate every guardrail copy
cd tools/skillslint && go run . --root ../..
cd tools/skillslint && go test ./... -count=1
```

`tools/skillslint` is its own Go module, so it is built and tested from inside its
own directory (`cd tools/skillslint && go test ./...`), not from the repo root.

## What it checks

| Check | Scope | Source |
|---|---|---|
| Skill-file structure | `plugins/assay/skills/*/SKILL.md` | `lint.go` |
| Unresolved house values | **every `*.md` under `plugins/`** | `housevalue.go` |
| Shared-guardrail derive-or-diff | every declared guardrail copy | `guardrail.go` |

### 1. Skill-file structure

Per `SKILL.md`: frontmatter present, `name:` present and equal to the directory
name, `description:` present and non-empty, and no bare "unforgeable" /
"tamper-evident" overclaim about a review, App or gate. Parsing is deliberately
line-oriented rather than strict YAML — see the comment at the top of `lint.go`
for why a strict parser would be wrong about this corpus.

### 2. Unresolved house values — the WHOLE plugin tree

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

### 3. Shared-guardrail derive-or-diff

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

## Not wired into a workflow

No workflow in `.github/workflows/` calls this tool today; it runs as
`make skillslint`. Wiring the gate is tracked separately
(`docs/streams/mistake-proofing/brief-04-derived-enforcement-status.md`), and
widening the check does not change that.
