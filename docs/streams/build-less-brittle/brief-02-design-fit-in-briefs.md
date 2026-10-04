---
brief: assay:assay:build-less-brittle:02
title: "design-fit: in every new brief — owner, contract, retires, weight, why-add"
why: >-
  A brief today says what to build and how to check it, but never where the change belongs or
  what it replaces. So a symptom brief reliably yields a local patch, and a deletion is never
  written because no brief asks for one. A five-key design-fit block in every new brief's
  Context makes the author answer "which module owns this, what does it retire, and how much
  weight does it add" before any worker starts, and gives the reviewer something to hold the
  diff to.
wave: 1
depends: ["build-less-brittle/01"]
unblocks: ["build-less-brittle/04", "build-less-brittle/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 rows 2 and 8, §4.2"
  - "spec/brief-v1.md §4.1 (consumers: and layering: precedents), §3.2 exec-tier"
  - "plugins/assay/skills/author-brief/SKILL.md (template, rule 9, dispatch checklist)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no design-fit, retires or why-add key in spec/, docs/brief-template.md or the author-brief skill"
exec-tier: strong
exec-tier-why: "(b) one convention lands consistently in three artifacts (spec, template, skill), downstream copies re-sync on their own pin bump, and the skill must end net ≤ 0 lines."
domain: complicated
consumers:
  - "spec/brief-v1.md §4.1: fixed-here"
  - "docs/brief-template.md: fixed-here"
  - "plugins/assay/skills/author-brief/SKILL.md: fixed-here"
  - "downstream project copies of the author-brief skill (synced bundles, byte-parity twins): out-of-scope (each adopter re-syncs on its own pin bump)"
---

# Brief 02 — design-fit: in every new brief

## Context

files:
- `spec/brief-v1.md`: §4.1 (Context section) gains the `design-fit:` block. §3.2's
  `exec-tier` row gains question (d).
- `docs/brief-template.md`: the Context block gains `design-fit:`.
- `plugins/assay/skills/author-brief/SKILL.md`: the template block, rule 9 (d), and the dispatch
  checklist.
- `changelog/build-less-brittle-02.md` (planned)

facts:
- The block (spec §4.2, verbatim keys): `owner`, `contract`, `retires`, `weight`, `why-add`.
  `contract` names an `S-<slug>` row of `docs/contracts.md` §"Semantic owners" (brief 01), or
  `none — <why>`. `weight` is a signed delta per ratcheted dimension (verbs, flags, refusals,
  rule-text lines). `why-add` is REQUIRED when any delta is positive and `n/a` otherwise.
- It is a Context line, like `consumers:` and `layering:`. It is **not** frontmatter, so the
  JSON schemas and `statusgen conform` are untouched. **No lint is added.** The grammar
  settles first, the same precedent the author-brief skill records for `consumers:`.
- Required on every NEW brief. Legacy briefs are not back-filled.
- Two rules the text must carry (spec §4.2): (1) consolidate meaning, preserve independent
  enforcement; (2) retiring a control at a trust boundary names the layer still refusing the
  threat, with a Verify row proving it with the retired layer absent. Tests pinning a retired
  refusal retire with it.
- New `exec-tier` question (d): "Is this a design brief raised by an error-class trigger?"
  yes → `strong`.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): author-brief SKILL.md 768, docs/brief-template.md 218. The
  checklist has 9 items. It must stay at single digits by merging an item, not appending
  (the skill's own rule).

design-fit:
  owner: spec/brief-v1.md §4.1 (the brief body contract)
  contract: none — briefs have no semantic-owner row; the brief spec is its own decision record
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (author-brief SKILL.md must not grow)
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The skill edit is **net ≤ 0 lines**. Offset additions by moving incident narrative to a
  findings link or cutting restatements that duplicate `spec/brief-v1.md`. Never cut a rule's
  operative sentence to make room.

## Task

1. `spec/brief-v1.md` §4.1: add a paragraph after the `layering:` paragraph defining
   `design-fit:` (the facts above). Add (d) to the `exec-tier` derivation in §3.2's row text.
2. `docs/brief-template.md`: add the `design-fit:` block to the Context example, with an
   example `contract: S-eligibility`.
3. `plugins/assay/skills/author-brief/SKILL.md`: add the block to the template. Add (d) to
   rule 9. **Merge** checklist items 8 and 9's neighbour so the list stays ≤ 9, with one item
   reading "New component, or any weight delta > 0 → `layering:`/`design-fit:` answered;
   `why-add` names what removal was considered". Offset every added line.
4. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 1–4 gate presence. Row 5 dereferences the
example contract id. Row 6 is the net ≤ 0 weight row. Row 7 checks question (d) landed in both files. Row 8 is the neighbour row (the
shared-value trigger that reads Context text). Row 9 corroborates the `consumers:` claims against
the delivering change itself, pinned so it still has that diff to read after merge (re-authored
in version 2, #1902).

| # | Command | Expect |
|---|---------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | ≥ `2` |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | ≥ `5` |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | ≥ `5` |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | ≤ `9` (and ≥ `1`: the checklist still exists) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | `1` (the template's example cites a row that exists) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | `2` (question (d) landed in both the skill and the spec) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | `ok` (neighbour: the consumers trigger still does not fire on ordinary Context lines such as the new block) |
| 9 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 274ece128 && statusgen --consumers --root "$d" --brief build-less-brittle/02 --base 274ece128~1; s=$?; rm -rf "$d"; exit $s` | exit 0; output is `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` (the check runs in a throwaway shared clone checked out at 274ece128, the squash that delivered this brief in #1842, with the base pinned to its parent, so the diff it reads is exactly the delivering change: never main's later commits, never the runner's own working tree. The three `fixed-here` entries are corroborated by that diff; the one UNCHECKED entry is the `out-of-scope` downstream-copies line, which names no path in this repo and stays the reviewer's call per brief-rule 9, never a pass) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### Verification — build-less-brittle/02 @ 024c87b01aba (non-implementer verifier, 2026-09-30 UTC)

What moved since the last run: this is the first verify run. The brief had no Evidence rows and there was no earlier verify-outcome record for 02. The implementation is squash commit 274ece12809994a5334342a02bdbc8aa9b1d350f (#1842), parent def62cbf50bc6acd4110da1cb3a5e2417e1df5a4. Merged main was re-fetched and had not moved from 024c87b01aba8f6c7dd7ccd939e647a9b936be09.

Every command ran from the repo root of a detached worktree at merged main, with `KUBECONFIG=/dev/null`. statusgen on PATH is v1.0.29. Wherever a `--root` flag says `"$PWD"`, the run passed the absolute worktree path, which equals `$PWD` at the repo root. Pipes are escaped as `\|` for the table.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | ≥ 2 | exit 0, printed `3`. Discharges Verify row 1 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | ≥ 5 | exit 0, printed `5`. The matches are the five design-fit keys at template lines 99–103 (example `contract: S-eligibility`). Discharges Verify row 2 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | ≥ 5 | exit 0, printed `5`. The matches are the five template keys at skill lines 179–183. Discharges Verify row 3 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | ≤ 9 and ≥ 1 | exit 0, printed `9`. Checklist items 1–9 are at skill lines 597–610, and item 8 now carries design-fit. Discharges Verify row 4 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | 1 | exit 0, printed `1` with id=S-eligibility. The row exists at docs/contracts.md line 165. Discharges Verify row 5 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0, printed `NET-OK`. impl resolved to 274ece128 and base to 274ece128~1. The skill went from 772 lines to 771, a net change of -1. Discharges Verify row 6 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | 2 | exit 0, printed `2`. The question appears at skill line 368 (rule 9 (d)) and in the spec §3.2 exec-tier row at line 64. Discharges Verify row 7 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8a | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | ok | exit 0, printed `ok  github.com/medici-finance/assay/statusgen 0.154s`. Run as authored. Discharges Verify row 8 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8b | `cd statusgen && go test -v -run TestSharedValueTriggerIsNarrow -count=1 .` | --- PASS, no SKIP | exit 0. The test printed `--- PASS` and `ok`, with no SKIP line. This is the -v cross-check for Verify row 8 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9a | `statusgen --consumers --root "$PWD" --brief build-less-brittle/02; echo "exit=$?"` | exit=0 at the PR head | Printed `exit=2` with `COULD-NOT-CHECK: ... is not in the diff against 024c87b01aba...` on merged main, as the row predicts: the brief is not in main's own diff. The witness below also ran the as-authored `--root .` form. This is the merged-main arm of Verify row 9 and discharges nothing | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9b | `git checkout --detach 274ece12809994a5334342a02bdbc8aa9b1d350f && statusgen --consumers --root "$PWD" --brief build-less-brittle/02 --base def62cbf50bc6acd4110da1cb3a5e2417e1df5a4; echo "exit=$?"; git checkout --detach 024c87b01aba8f6c7dd7ccd939e647a9b936be09` | exit=0, no disproved claim | Printed `exit=0`: 3 CORROBORATED (spec/brief-v1.md §4.1, docs/brief-template.md and the author-brief SKILL.md, each fixed-here), 0 disproved, and 1 UNCHECKED (downstream copies, out-of-scope, not in this diff). This is the tool's own printed recipe for a merged brief: the squash commit against its parent is the PR's net diff. A statusgen built from source at 024c87b gave the same result, exit 0 and 3/0/1. Discharges Verify row 9 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Execution witness from `statusgen verifyrun --root <abs worktree> --brief docs/streams/build-less-brittle/brief-02-design-fit-in-briefs.md --dry-run`. It exited 1 and wrote nothing back. The table is kept exactly as emitted:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | pass exit=0 | sha256:544b328f1fe1 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --consumers --root . --brief build-less-brittle/02; echo "exit=$?"` | fail exit=0 | sha256:8748211bab8e | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |

Findings:
- **F1 (the witness cannot judge row 9).** verifyrun printed `row 9: fail ... exit 0, expected 1`. It read its expected exit code from the phrase "Exit 1 names the disproved claim" in the Expect cell, but the cell's own pass condition is `exit=0`. Two further problems make the row unusable for an exit-status witness. First, the trailing `; echo "exit=$?"` forces the shell to exit 0 whatever statusgen returns. Second, on merged main the as-authored row is could-not-check by design (arm 9a, statusgen exit 2). So the witness row 9 `fail` is an artifact of how the row is written, not a disproved consumers claim; arm 9b shows 0 disproved at the PR's net diff. The fix is to re-author row 9 to run against the squash parent (`--base <impl>~1` at `<impl>`), drop the `echo`, and state a single expected exit code. Any flip that is gated on the witness will refuse this row until then.
- **F2 (checklist wording differs from the Task, no Verify row).** Task 3 prescribed item wording beginning "New component, or any weight delta > 0 → `layering:`/`design-fit:` answered". Landed item 8 instead reads "Every new brief → `design-fit:` answered; any weight delta > 0 → `why-add` names what removal was considered. New component → `layering:` …". The meaning is the same or stricter, since it requires design-fit on every new brief, which matches the facts. No Verify row checks this wording.
- **F3 (deliverables with no Verify row).** Two deliverables have no Verify row: the changelog fragment (changelog/build-less-brittle-02.md) and the two design-fit rules in spec §4.1 (consolidate meaning while preserving independent enforcement; retiring a trust-boundary control names the layer that still refuses the threat). I read both by hand and both are present at spec §4.1 items 1–2. They are recorded here as work that maps to no Verify row.
- **F4 (unstable output hash).** The witness output hash for row 8 differs between runs (e13dc78d8ff3, then 544b328f1fe1) because `go test` prints wall-clock timing. The pass/fail result is stable; only the hash changes.

Risk-bearing value. The trigger did not fire: the risk frontmatter is present with regulatory, customer, irreversible and sensitive-data all `no`, and the diff (274ece128) touches only spec/brief-v1.md, docs/brief-template.md, the author-brief skill, the changelog and this stream's docs, none of them risk-classed paths. I still enumerated the literals the diff introduces:
- checklist cap `9` items (skill line 615 wording "single digits"; count 9 at lines 597–610)
- skill net rule-text lines `≤ 0` (772 → 771)
- exec-tier mapping `any yes → strong` (skill line 135)
- why-add threshold `delta > 0` (skill lines 183 and 607)

All four are rule-text knobs that an edit and a republish can reverse, so they rank last and need no derivation.

rows_passed=9 rows_total=9

RISK-VALUE: N/A — the risk trigger did not fire (frontmatter present, irreversible: no, no risk-classed path in 274ece128). Enumerating over the 274ece128 diff found only reversible rule-text literals (checklist cap 9, net lines ≤ 0, any yes → strong, delta > 0), ranked last per kit §4 step 3. There is no irreversible act: this is a docs/spec convention change.

VERIFY: PASS

### Non-implementer verifier run — 2026-10-02 claude-opus-5-5-verifier

SHA cross-check: the worktree HEAD is cca9028244d9b3bae6501f11df6ea6ebf043ed66 and the forge's main is cca9028244d9b3bae6501f11df6ea6ebf043ed66. They are equal, and the remote-tracking main ref in the worktree resolves to the same commit. The worktree was detached and clean before and after the run.

Environment: darwin/arm64, go1.27.1, statusgen v1.0.30, `KUBECONFIG=/dev/null`. Every command ran from the repo root in a fresh `bash -c` subshell. Pipes are escaped as `\|` for the table. This run is against version 2 of the Verify table (row 9 re-authored, #1902). The delivering change is squash 274ece12809994a5334342a02bdbc8aa9b1d350f (#1842), parent def62cbf50bc6acd4110da1cb3a5e2417e1df5a4.

Expectations written from the brief text before running anything: spec §4.1 carries a design-fit paragraph naming why-add; the brief template and the author-brief skill each carry all five keys (owner, contract, retires, weight, why-add); the template's example contract id is S-eligibility and resolves to exactly one row of the semantic-owner index; the skill checklist stays at 9 items or fewer; the skill did not grow across the delivering change; exec-tier question (d) is present in both the skill and the spec §3.2 row; the shared-value trigger test still passes; the three fixed-here consumers are corroborated by the delivering diff and the one out-of-scope entry stays unchecked.

| # | Command | Expect | Result (exit + real output line) | Date | Runner |
|---|---------|--------|----------------------------------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | ≥ 2 | exit 0, printed `3`. The matching lines are spec lines 216, 221 and 226. Discharges Verify row 1 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | ≥ 5 | exit 0, printed `5`. The five keys sit at template lines 99–103 under the design-fit line at 98. Discharges Verify row 2 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | ≥ 5 | exit 0, printed `5`. The five keys sit at skill lines 179–183 under the design-fit line at 178. Discharges Verify row 3 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | ≤ 9 and ≥ 1 | exit 0, printed `9`. Items 1–9 are at skill lines 598–611; item 8 (line 608) carries design-fit and why-add. Discharges Verify row 4 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | 1 | exit 0, printed `1`. The id resolved to S-eligibility, whose row is at line 165 of the contracts document. Discharges Verify row 5 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0, printed `NET-OK`. impl resolved to 274ece128, base to its parent; the skill went from 772 lines to 771, net -1. Discharges Verify row 6 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | 2 | exit 0, printed `2`. The question is at skill line 368 (rule 9 (d)) and in the spec §3.2 exec-tier row at line 64. Discharges Verify row 7 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | ok | exit 0, printed `ok  github.com/medici-finance/assay/statusgen 0.418s`. A second run with -v printed `--- PASS: TestSharedValueTriggerIsNarrow (0.00s)` and no SKIP line. Discharges Verify row 8 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |
| 9 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 274ece128 && statusgen --consumers --root "$d" --brief build-less-brittle/02 --base 274ece128~1; s=$?; rm -rf "$d"; exit $s` | exit 0; `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` | exit 0, printed `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The header line named base def62cbf50bc (the parent of 274ece128). CORROBORATED: spec §4.1, the brief template, the author-brief skill, each fixed-here. UNCHECKED: the downstream-copies out-of-scope entry, which stays the reviewer's call and is not counted as a pass. Discharges Verify row 9 | 2026-10-02 | assay-verifier-app[bot] @ cca9028244d9 (claude-opus-5-5-verifier) (on-behalf-of human:ian) |

rows_passed=9 rows_total=9

Execution witness: `statusgen verifyrun --root <repo root> --brief docs/streams/build-less-brittle/brief-02-design-fit-in-briefs.md --dry-run` exited 0 and wrote nothing back (the tool confirmed the table was not written; the worktree stayed clean). Per-row result, as emitted:

| # | Witness result | Output hash |
|---|----------------|-------------|
| 1 | pass exit=0 | sha256:1121cfccd591 |
| 2 | pass exit=0 | sha256:f0b5c2c2211c |
| 3 | pass exit=0 | sha256:f0b5c2c2211c |
| 4 | pass exit=0 | sha256:2e6d31a5983a |
| 5 | pass exit=0 (exit-status only) | sha256:4355a46b19d3 |
| 6 | pass exit=0 (exit-status only) | sha256:458c4e39effe |
| 7 | pass exit=0 (exit-status only) | sha256:53c234e5e847 |
| 8 | pass exit=0 (exit-status only) | sha256:10a969b33800 |
| 9 | pass exit=0 | sha256:0bbdd84b1b34 |

Witness total: 9 pass, 0 fail. No row is classed check:ci, so nothing needed a network-off Linux sandbox. For rows 5–8 the witness reported that it judged the exit status only, because nothing else in the Expect cell is machine-decidable; the printed values for those rows are confirmed by hand in the Evidence table above.

Differences from the earlier verifier block in this brief (2026-09-30, at 024c87b01aba):
- Row 9: the earlier block split it into two arms and the witness recorded `fail` for the as-authored command. With the version-2 row the single command exits 0 by hand and the witness records `pass`. The earlier finding F1 is resolved by the re-authored row.
- Row 8: the witness hash differs (10a969b33800 now, 544b328f1fe1 before). The pass result is the same.
- Rows 1–7: same exit codes, same printed values, same witness hashes. Skill line numbers moved by later commits (checklist now at 598–611; the skill is 777 lines at this head), which does not affect row 6 because that row measures the delivering commit against its parent.
- statusgen is v1.0.30 here, v1.0.29 in the earlier block.

Findings:
- **F1 (row 8 witness hash is unstable).** `go test` prints wall-clock timing, so the output hash for row 8 changes on every run. The pass/fail result is stable. A later `--check` that compares hashes for this row would see a mismatch that means nothing.
- **F2 (checklist wording differs from the Task, no Verify row covers it).** Task 3 prescribed an item reading "New component, or any weight delta > 0 → layering:/design-fit: answered". The landed item 8 reads "Every new brief → design-fit: answered; any weight delta > 0 → why-add names what removal was considered. New component → layering: …". This is the same or stricter and matches the brief's fact that the block is required on every new brief.
- **F3 (deliverables with no Verify row, read by hand).** The two design-fit rules (consolidate meaning while preserving independent enforcement; retiring a trust-boundary control names the layer still refusing the threat) are present in spec §4.1 as numbered items 1 and 2. The changelog fragment was delivered in 274ece128 and has since been folded into the top-level changelog by the release aggregation commit at this head, so the fragment file itself is no longer in the tree; that is expected release behaviour, not a missing deliverable.
- **F4 (offsets removed restatement, not operative rules).** The skill's net -1 came from two cuts: the layering defaults paragraph in the template, replaced by a pointer to spec §4.1, and an incident narrative under the dereference rule (skill line 420), replaced by a pointer to the brief-rules reference rule 43. Both targets exist and carry the removed content (spec §4.1 lines 180–201; brief-rules rule 43 at line 869). No operative rule sentence was lost.
- No invented scope: the delivering diff touches only the spec, the template, the skill, the changelog fragment and this stream's own docs. The spec diff also re-worded the neighbouring consumers paragraph to say it is a frontmatter schema field, which is a correction adjacent to the new paragraph rather than new behaviour.
- No flaky row was observed: every row gave the same exit code by hand and under the witness.

Risk-bearing value. The fail-safe trigger did not fire: the risk frontmatter is present with all four answers `no`, the delivering diff touches no risk-classed path, and it changes no value the repo's standing constraints pin as hard. The enumeration was done anyway, over the 274ece128 diff plus the brief's Deliverables:
- complexity-question count = `four` @ plugins/assay/skills/author-brief/SKILL.md:135 and :361, and spec/brief-v1.md:64 (was three)
- exec-tier mapping = `any yes → strong` @ plugins/assay/skills/author-brief/SKILL.md:135
- why-add threshold = `delta > 0` @ plugins/assay/skills/author-brief/SKILL.md:183 and :608; "positive" @ spec/brief-v1.md:226
- checklist cap = `9` items, stated as "single digits" @ plugins/assay/skills/author-brief/SKILL.md:616 (count 9 at lines 598–611)
- skill net rule-text lines = `≤ 0` (772 → 771 across 274ece128)
- template example weight = `verbs 0, flags 0, refusals 0, rule-text lines 0` @ docs/brief-template.md:102

Rank: every entry is authoring guidance text. If one is wrong, a brief is authored with a wrong tier or a missing why-add, and the fix is an edit and a republish; nothing is irreversible. The highest-ranked is the why-add threshold, because it decides when an author must justify added weight; the rest are reversible knobs that rank last and need no derivation.

RISK-VALUE: DERIVED — why-add threshold = `delta > 0` @ plugins/assay/skills/author-brief/SKILL.md:183 — the stream spec §4.2 (its line 146) states why-add is "required when any weight delta is positive" and the brief's facts repeat it; zero is the only boundary consistent with a ratchet whose purpose is to make any growth carry a justification, and a zero or negative delta adds nothing to justify. The spec text at spec/brief-v1.md:226 and the checklist item at skill line 608 use the same boundary.

VERIFY: PASS — all nine Verify rows passed by hand at cca9028244d9 and all nine passed in the dry-run execution witness, with no row failing and none left without a result.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | pass exit=0 | sha256:1121cfccd591 | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | pass exit=0 | sha256:53c234e5e847 | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | pass exit=0 | sha256:b60da6f56f87 | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |
| 9 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 274ece128 && statusgen --consumers --root "$d" --brief build-less-brittle/02 --base 274ece128~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:0bbdd84b1b34 | 2026-10-02 | assay-verifier-app[bot] @ cd435f006a68 (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier run — VERIFY: PASS — 2026-10-04 claude-opus-5-5-verifier

Run against merged main d6662bbc6b4fc64be615baf14a010da126aef16a in a detached, clean home worktree (the verifier's home worktree), with `KUBECONFIG=/dev/null`. Environment: darwin/arm64, go1.27.1, statusgen v1.0.31. Every row ran from the repo root in a fresh bash subshell; row 9's `mktemp -d` was pointed (via TMPDIR) at a scratch directory inside the verifier's home worktree and removed by the row itself. Pipes are escaped as `\|` for the table. Brief frontmatter: gate `model`; risk `{regulatory: no, customer: no, irreversible: no, sensitive-data: no}`; irreversible `no`. No row is risk-bearing, so the per-row-in-isolation rule for ≥ 4 risk-bearing rows does not apply.

Delivering change: squash 274ece12809994a5334342a02bdbc8aa9b1d350f (#1842, trailer `Brief: build-less-brittle/02`), parent def62cbf50bc6acd4110da1cb3a5e2417e1df5a4. A later docs-only commit 0f62549b1 (#1908) re-authored Verify row 9; it delivers no deliverable. Declared deliverables: spec/brief-v1.md (§4.1 design-fit paragraph, §3.2 exec-tier question (d)); docs/brief-template.md (Context design-fit block); plugins/assay/skills/author-brief/SKILL.md (template block, rule 9 (d), merged checklist item); changelog/build-less-brittle-02.md (planned fragment).

Expectations written from the brief text before running anything: spec §4.1 names design-fit and why-add; the template and the skill each carry the five keys owner, contract, retires, weight, why-add; the template's example contract is S-eligibility and resolves to exactly one semantic-owner row; the skill checklist stays at 9 items or fewer; the skill does not grow across the delivering change; question (d) appears in both the skill and the spec; the shared-value trigger still does not fire on ordinary Context text; the three fixed-here consumers are corroborated by the delivering diff and the out-of-scope line stays unchecked.

Vacuity probe: rows 1, 2, 3, 4, 5 and 7 were re-run against the four deliverable files as they stood at the pre-brief parent def62cbf50bc (exported into a scratch directory). Rows 1, 2, 3 printed 0 and exited 1, row 5 found no contract id and exited 1, row 7 printed 0 and exited 1, so those rows discriminate. Row 4 printed 9 and exited 0 on the pre-brief tree as well.

| Verify row discharged | Command | Expect | Observed | Result | Date | Runner |
|---|---|---|---|---|---|---|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | ≥ 2 | exit 0, printed `3`. On the pre-brief tree: printed `0`, exit 1 | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | ≥ 5 | exit 0, printed `5` (the five keys at template lines 99–103 under the design-fit line at 98; example `contract: S-eligibility`). On the pre-brief tree: printed `0`, exit 1 | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | ≥ 5 | exit 0, printed `5` (skill lines 179–183 under the design-fit line at 178). On the pre-brief tree: printed `0`, exit 1 | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | ≤ 9 and ≥ 1 | exit 0, printed `9`. VACUOUS as a presence check: the pre-brief tree also printed `9`, exit 0, so the row only bounds growth. Hand check: item 8 at skill line 608 reads "Every new brief → design-fit: answered; any weight delta > 0 → why-add names what …", so the merge landed | pass (bound only) | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | 1 | exit 0, printed `1`; id resolved to S-eligibility, whose row is at line 165 of the contracts document. On the pre-brief tree: no id, exit 1 | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0, printed `NET-OK`. impl resolved to 274ece128, base to its parent; the skill went from 772 to 771 lines (net -1) across the delivering change | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | 2 | exit 0, printed `2` (skill line 368, rule 9 (d); spec line 64, §3.2 exec-tier row). On the pre-brief tree: printed `0`, exit 1 | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` (run with `-timeout 300s` added) | ok | exit 0, printed `ok  github.com/medici-finance/assay/statusgen 0.361s`. VACUOUS for this brief: the test feeds two hard-coded strings to the trigger, never reads any design-fit text, and neither the test file nor the trigger code is touched by 274ece128, so it passes identically on the pre-brief tree. Direct check of the neighbour claim instead: the seven trigger phrases in the statusgen consumers source matched 0 lines (case-insensitive) in the landed design-fit blocks of the template, the skill and spec §4.1 | pass (vacuous; neighbour claim confirmed by hand) | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |
| 9 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 274ece128 && statusgen --consumers --root "$d" --brief build-less-brittle/02 --base 274ece128~1; s=$?; rm -rf "$d"; exit $s` | exit 0; `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` | exit 0, printed `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`; header named base def62cbf50bc. CORROBORATED: spec §4.1, the brief template, the author-brief skill (each fixed-here). UNCHECKED: the downstream-copies out-of-scope line, the reviewer's call and not a pass. The row is pinned to the delivering commit, so it does not depend on main's head by design | pass | 2026-10-04 | claude-opus-5-5-verifier @ d6662bbc6b4f |

Hand checks of deliverables with no Verify row: the two design-fit rules sit in spec §4.1 as numbered items 1 and 2 (spec lines 237 and 240), and item 2 carries the Verify row with the retired layer absent plus the line that tests pinning a retired refusal retire with it. The changelog fragment was delivered in 274ece128 and has since been folded into the top-level changelog by release aggregation (CHANGELOG line 59), so the fragment file is absent from this tree as expected.

rows_passed=9 rows_total=9 (rows 4 and 8 pass but are vacuous as described; the deliverables they point at were confirmed by hand)

Execution witness: `statusgen verifyrun --brief docs/streams/build-less-brittle/brief-02-design-fit-in-briefs.md` exited 0 and appended the witness to the brief's Evidence section. `statusgen verifyrun --check` on the same path exited 0 with this summary:

docs/streams/build-less-brittle/brief-02-design-fit-in-briefs.md: 9 pass, 0 fail, 0 could-not-run/missing (of 9 Verify rows)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | pass exit=0 | sha256:1121cfccd591 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | pass exit=0 | sha256:53c234e5e847 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | pass exit=0 | sha256:9aeea5a53c3e | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 274ece128 && statusgen --consumers --root "$d" --brief build-less-brittle/02 --base 274ece128~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:0bbdd84b1b34 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |

The `+dirty` suffix on the runner cells comes from the untracked scratch directory inside the verifier's home worktree; no tracked file differed from d6662bbc6 when the witness ran.

Risk-bearing value. The fail-safe trigger does not fire: the risk frontmatter is present with all four answers `no`, the delivering diff touches only spec/brief-v1.md, docs/brief-template.md, the author-brief skill, the changelog fragment and this stream's docs (no risk-classed path), and it changes no value the repo's standing constraints pin as hard. The enumeration was done anyway, over the 274ece128 diff plus the brief's Deliverables, with line numbers at d6662bbc6:
- complexity-question count = `four` @ plugins/assay/skills/author-brief/SKILL.md:135 and :361; spec/brief-v1.md:64
- exec-tier mapping = `any yes → strong` @ plugins/assay/skills/author-brief/SKILL.md:135
- why-add threshold = `delta > 0` @ plugins/assay/skills/author-brief/SKILL.md:183 and :608; "positive" @ spec/brief-v1.md:226 and docs/brief-template.md:103
- checklist cap = `9` items, stated as "single digits" @ plugins/assay/skills/author-brief/SKILL.md:616 (count 9)
- template example weight = `verbs 0, flags 0, refusals 0, rule-text lines 0` @ docs/brief-template.md:102
- skill net rule-text lines = `≤ 0` (brief design-fit weight; measured 772 → 771 across 274ece128)

Rank: every entry is authoring-guidance text. If one is wrong, a brief is authored at the wrong tier or without a why-add, and an edit plus a republish fixes it; nothing is irreversible. The why-add threshold ranks highest because it decides when an author must justify added weight. The rest are reversible knobs that rank last and need no derivation.

RISK-VALUE: DERIVED — why-add threshold = `delta > 0` @ plugins/assay/skills/author-brief/SKILL.md:183 — the stream spec (docs/streams/build-less-brittle/spec.md:146) requires why-add "when any weight delta is positive", and its §3 row 2 (line 83) makes `n/a` legal exactly when the delta is zero. A ratchet exists so that any growth carries a justification, and a zero or negative delta adds nothing to justify, so zero is the only consistent boundary. spec/brief-v1.md:226, docs/brief-template.md:103 and the checklist item at skill line 608 use the same boundary.

VERIFY: PASS — all nine Verify rows passed by hand at d6662bbc6 and in the execution witness (9 pass, 0 fail, 0 could-not-run). Rows 4 and 8 are vacuous as presence checks, and their deliverables were confirmed by hand.

## Review
Gate: model (from frontmatter). The reviewer checks that the skill's offsets removed narrative or
restatement, never an operative rule sentence.
