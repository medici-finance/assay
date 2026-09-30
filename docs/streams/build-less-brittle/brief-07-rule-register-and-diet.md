---
brief: assay:assay:build-less-brittle:07
title: Rule register (owner, invariant, justifying issue, catch source) and the monthly rule diet
why: >-
  Every incident adds a rule, a refusal or a skill paragraph, and nothing ever reviews whether
  that rule still earns its cost. There is no list of rules, owners or the invariant each one
  protects, so a rule that misfires is patched by another rule instead of being questioned. A
  register seeded with the rules behind the recent fix-caused-next-bug chains, plus a monthly
  diet that asks the driver keep-or-retire in one reply, gives rules a way to leave. It never
  deletes a rule automatically.
wave: 1
depends: ["build-less-brittle/01", "build-less-brittle/03"]
unblocks: ["build-less-brittle/06", "build-less-brittle/10", "build-less-brittle/11"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 7, §4.6, §4.7"
  - "docs/contracts.md §Semantic owners (brief 01): every rule row cites the S- row it enforces"
  - "public issues behind the chain-implicated rules: #1145 #1274 #1631 #1650 (credential), #1371 #1470 #1617 #1623 (transport), #1459 #1497 #1498 (model floor), #1339 #1419 #1502 #1558 #1641 (delivery), #1335 #1395 #1564 #1571 #1500 (corroboration), #1642 #1643 (secret heuristic), #1601 #1602 (same-head re-approval)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no rule register exists; the only catch counting is an operator-side gate-telemetry report, whose classes all read fires=0 with configuration unset"
exec-tier: strong
exec-tier-why: "(a) choosing each seed rule's invariant and serving S- row is judgement the facts do not pre-specify; (b) paths are swept across tools/desk and statusgen."
domain: complicated
---

# Brief 07 — Rule register and the monthly rule diet

## Context

files:
- `docs/contracts.md`: add `## Rule register` and `## Rule diet (monthly)` after brief 01's section.
- `changelog/build-less-brittle-07.md` (planned)

single-point-of-failure: the diet role's author check on the `retire …; keep rest` reply (the
driver's own login, read from the forge) is the ONE control that starts a retirement. Behind it: a
retirement is a design brief with its own review and the driver's merge, and a trust-boundary row
also needs spec §4.2 rule 2's Verify row and worker kit §2's `gate: human` path (spec §4.7).

facts:
- **Row schema:** `id (R-<slug>) | rule (one line) | enforced at (path[:symbol]) | serves (S-<slug>) | owner (module or role) | invariant | justifying issue(s) | catch source | last reviewed`.
  The catch source is a named telemetry class, a test name that proves it can fire, or `none`.
- **Seed scope** (not a back-fill): the rules implicated in the public chains listed in
  `sources`, plus the weight ceiling (`R-weight-ceiling`, `tools/desk/internal/weight/ceiling.txt` (planned),
  brief 03). Candidate enforcement sites at f7bde6bfa (re-verify each):
  - model-capability floor: `tools/desk/internal/deskkit/modelfloor.go`, `tools/desk/cmd/deskflip/main.go`
  - high-entropy body scan: `tools/desk/internal/deskkit/bodycheck.go`, `tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go`
  - new-issue budget: `tools/desk/cmd/deskfile/deskfile.go` (`defaultNewRate`)
  - inherited-token precedence: `tools/desk/cmd/desktoken/desktoken.go`, `tools/desk/cmd/deskdispatch/dispatch.go`
  - `Authors:` trailer / authoring-PR classification: `tools/desk/cmd/deskdispatch/authoring.go`, `tools/desk/cmd/deskflip/flip.go`
  - same-head re-approval: `tools/desk/cmd/deskflip/main.go`, `tools/desk/cmd/deskpost/review.go`
  - placeholder ruling stamps: `statusgen/decisionruling.go`
  - forge-CLI ceiling: `tools/desk/internal/forgeban/allowlist.go`
  Expect 12–20 rows.
- **Three-state catch status** for the diet: `could-not-check` (the counter is blind or absent),
  `proven-able-to-fire` (zero catches, but a named test shows it firing), and
  `zero-without-proof`. Zero catches is an **alarm, never a deletion**.
- **One typed reply.** The monthly diet files ONE decision issue listing candidate rows. The
  options are fixed at filing; an edit supersedes the issue with a new one. The reply shape is
  `retire R-a R-b; keep rest`, or `keep`, and it counts only from the driver's own login (a
  project value), read from the forge, never from the text (spec §4.7). A reply from any other
  login is quarantined and never acted on. Default if unanswered: keep. A retirement becomes a
  design brief (usually bundled per class), never an in-loop edit.
- **Trust-boundary rows are seeded able to fire.** Several seed rows are security controls (the
  high-entropy body scan, the model-capability floor, inherited-token precedence, same-head
  re-approval, the forge-CLI ceiling). They fire rarely by design, so each is seeded with a named
  test as its catch source and starts `proven-able-to-fire`, never `zero-without-proof`.
  Retiring such a row is not only a design brief: it carries spec §4.2 rule 2 (name the layer
  that still refuses the threat and prove it with the retired layer absent) and worker kit §2
  (`gate: human`).
- **Candidates:** `zero-without-proof`; a false-positive share above half of its recorded
  fires; orphans (no `S-` row); rules whose justifying issue is closed as not-planned.
- New rules add a row in the same PR. Review stage 06 checks this.
- **Later rows this stream adds** (not seeded here; each lands with its own brief): the brittle
  mark's two-key rule as `R-brittle-mark` (08, catch source: the monthly pass's report) and the
  three fitness-function rows `R-dep-direction`, `R-hub-allowlist`, `R-one-implementation`
  (10, catch source: the test names). The monthly diet and the monthly brittle pass (08) run
  together: one role, one cadence, two tables.

design-fit:
  owner: docs/contracts.md (the semantic index and its enforcing rules live together)
  contract: S-semantic-index
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0 (docs outside the ratcheted set)
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Record rules as they are. Do not propose retirements in this brief. The first diet does that.
- Public tree: public issue numbers only, never a private tracker reference.

## Task

1. Add `## Rule register`: the row schema, the rule "a new rule adds a row in the same PR", and
   the seed rows. Each seed row cites a path verified at fresh main and a real public issue.
   Each seed row that guards a trust boundary names a test that shows it firing as its catch
   source.
2. Add `## Rule diet (monthly)`: the three-state status, the candidate criteria, the single
   decision-issue shape with its immutable options and reply grammar, the keep default, and
   "retirement is a design brief", plus: a trust-boundary retirement also carries spec §4.2
   rule 2 and worker kit §2 (`gate: human`). State that the cadence and the running role are project
   values the project layer names.
3. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 1–3 gate presence. Rows 4–6 dereference
every row's links: the `S-` row it serves, its path, and its issue.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c -e '^## Rule register$' -e '^## Rule diet (monthly)$' docs/contracts.md` | `2` |
| 2 | `sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -cE '^[\|] *R-[a-z0-9-]+ *[\|]'` | ≥ `12` |
| 3 | `sed -n '/^## Rule diet/,$p' docs/contracts.md \| grep -c -e zero-without-proof -e proven-able-to-fire -e could-not-check` | ≥ `3` |
| 4 | `reg=$(sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md); out=$(echo "$reg" \| grep -oE 'S-[a-z0-9-]+' \| sort -u \| while read s; do grep -qE "^[\|] *$s " docs/contracts.md \|\| echo "ORPHAN $s"; done); test -z "$out" && echo SERVED \|\| echo "$out"` | `SERVED` (every cited S- row exists in the semantic index) |
| 5 | `out=$(sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -oE -e 'tools/desk/[A-Za-z0-9_./-]+\.go' -e 'tools/desk/[A-Za-z0-9_./-]+\.txt' -e 'statusgen/[A-Za-z0-9_./-]+\.go' \| sort -u \| while read p; do test -e "$p" \|\| echo "MISSING $p"; done); test -z "$out" && echo CLEAN \|\| echo "$out"` | `CLEAN` |
| 6 | `sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -E '^[\|] *R-' \| grep -vqE '#[0-9]+' && echo UNJUSTIFIED \|\| echo ALL-JUSTIFIED` | `ALL-JUSTIFIED` (every row cites an issue; row 2 proves rows exist, so this cannot pass vacuously) |
| 7 | `grep -cE '^[\|] *R-weight-ceiling ' docs/contracts.md` | `1` |
| 8 | `d=$(sed -n '/^## Rule diet/,$p' docs/contracts.md); echo "$d" \| grep -c 'gate: human'; echo "$d" \| grep -c '§4.2 rule 2'` | two counts, each ≥ `1` (a trust-boundary retirement names both the proven remaining layer and the human gate) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### Verification — 2026-09-30 (assay-verifier-app[bot] @ 35496323b8fc (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verify on merged main 35496323b8fc591651e44537baf51206cc22bbdd of delivery #1846 (6fa25631b). The pass grounded on the brief text before reading the diff. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm, `--network none`, statusgen built from main's own source) and passed 8/8. Second table: the hand run on the host.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## Rule register$' -e '^## Rule diet (monthly)$' docs/contracts.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 2 | `sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -cE '^[\|] *R-[a-z0-9-]+ *[\|]'` | pass exit=0 | sha256:54183f4323f3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 3 | `sed -n '/^## Rule diet/,$p' docs/contracts.md \| grep -c -e zero-without-proof -e proven-able-to-fire -e could-not-check` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 4 | `reg=$(sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md); out=$(echo "$reg" \| grep -oE 'S-[a-z0-9-]+' \| sort -u \| while read s; do grep -qE "^[\|] *$s " docs/contracts.md \|\| echo "ORPHAN $s"; done); test -z "$out" && echo SERVED \|\| echo "$out"` | pass exit=0 | sha256:237652ee7ebd | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 5 | `out=$(sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -oE -e 'tools/desk/[A-Za-z0-9_./-]+\.go' -e 'tools/desk/[A-Za-z0-9_./-]+\.txt' -e 'statusgen/[A-Za-z0-9_./-]+\.go' \| sort -u \| while read p; do test -e "$p" \|\| echo "MISSING $p"; done); test -z "$out" && echo CLEAN \|\| echo "$out"` | pass exit=0 | sha256:0b98843240a0 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 6 | `sed -n '/^## Rule register$/,/^## Rule diet/p' docs/contracts.md \| grep -E '^[\|] *R-' \| grep -vqE '#[0-9]+' && echo UNJUSTIFIED \|\| echo ALL-JUSTIFIED` | pass exit=0 | sha256:458e300b621a | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 7 | `grep -cE '^[\|] *R-weight-ceiling ' docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 8 | `d=$(sed -n '/^## Rule diet/,$p' docs/contracts.md); echo "$d" \| grep -c 'gate: human'; echo "$d" \| grep -c '§4.2 rule 2'` | pass exit=0 | sha256:8d58f634a5b5 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |

| # | Verify row | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | row 1 as written (exact command in the witness table above) | 2 | exit 0; output `2` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 2 | row 2 as written (exact command in the witness table above) | >= 12 | exit 0; output `17` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 3 | row 3 as written (exact command in the witness table above) | >= 3 | exit 0; output `5` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 4 | row 4 as written (exact command in the witness table above) | SERVED | exit 0; output `SERVED` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 5 | row 5 as written (exact command in the witness table above) | CLEAN | exit 0; output `CLEAN` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 6 | row 6 as written (exact command in the witness table above) | ALL-JUSTIFIED | exit 0; output `ALL-JUSTIFIED` — PASS (not vacuous: row 2 counted 17 rows) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 7 | row 7 as written (exact command in the witness table above) | 1 | exit 0; output `1` — PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 8 | row 8 as written (exact command in the witness table above) | two counts, each >= 1 | exit 0; output `2` then `2` — PASS (both hits sit in the diet section proper, contracts.md lines 232-295, before the brief-08 "## Brittle marks" heading at 296) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` on linux, network-off, 8/8 pass at 35496323b8fc.

RISK-VALUE: DERIVED — the reply authority is the driver's own login, read from the forge author field and counted only while unedited (docs/contracts.md lines 264-267). This is the brief's pinned SPOF control. It reuses the unedited-ruling test that R-dr-ruling-link applies, and the doc names the layers behind it.
RISK-VALUE: DERIVED — the unanswered default is keep (docs/contracts.md line 275). It is fail-safe by construction and is the brief's pinned default.
RISK-VALUE: DERIVED — the false-positive candidate threshold is more than half of recorded fires (docs/contracts.md line 253). It matches the brief's pinned figure, and crossing it only nominates a row; it never retires one.

Notes:
- All 19 path-and-symbol references and all 34 named catch-source tests resolve at this SHA. The catch-source packages pass offline, 10 of 10 ok.
- All 36 cited issue numbers resolve on this repo.
- R-new-issue-budget (#955, closed not_planned) and R-weight-ceiling both carry serves none, so they meet the diet candidate criteria from day one. They are for the first diet run; this is not a defect.
- R-shim-ambient-token's catch source is a shell test that no CI workflow runs, and the row says so. It is a candidate for CI wiring.

VERIFY: PASS

## Review
Gate: model (from frontmatter). The reviewer spot-checks three seed rows: does the stated
invariant match what the cited code refuses?
