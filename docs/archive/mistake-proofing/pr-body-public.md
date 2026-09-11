# docs(streams): file the `mistake-proofing` stream — the tooling half of the poka-yoke spec

Plan-only. This pull request adds one stream directory and nothing else: a README and six
self-contained briefs. No code changes, no lint changes, no tag, and `STATUS.md` is deliberately
NOT committed (generated, single-writer = main CI).

## What this is

`docs/mistake-proofing.md` landed on `main` as doctrine: source / in-process / downstream ×
control / warning × who can bypass it, seven normative device rules (D1–D7) and ten brief-authoring
rules (B1–B10). Most of that doctrine describes what this repo already does. This stream is the
plan for the part it does **not** do — the six rules whose devices are, today, prose in a markdown
file.

| Spec rule | Brief |
|---|---|
| B3 — risk answers cross-read against declared paths | 01 |
| B4 — named identifiers dereference | 02 |
| B2 (+ D7) — Verify rows carry a typed obligation class | 03 |
| B9 — authoring guidance's enforcement claims derived, never hand-copied | 04 |
| B1 — scaffold, don't type; the generator front door | 05 |
| D1 — a control must be shown to fire, promoted to a lint obligation | 06 |

B5/B7/B8 are pure process (spec §5 step 3) and belong in the desk skills, not the lint. B6 and B10
need decisions this stream does not make. D2–D6 are already practised across the tool surface; each
new check here inherits them.

## Shape of the plan

Critical path is `01 → 03 → 05`, head at **01**, waves `[01, 02] → [03, 04] → [05, 06]`.

01 is the head because it is the brief that first teaches the lint to read a brief's declared
`files:` line at all — that line is parsed by nothing today. 03's flow-row obligation derives from
those same declared paths, and 05's scaffolder must derive `gate` through the same cross-read or it
bakes the exact defect B3 names into the front door. The spec's own adoption ladder puts B1 last
for that reason; the waves follow the ladder.

04 sits behind 01 rather than beside it because both write the same generated region of the
authoring guidance: two pull requests writing one generated block are each green alone and red
merged.

## Conventions every brief in the stream inherits

- **Three states, always (D2).** Every new check reports PASS / PROBLEM / could-not-check and
  refuses when it cannot establish its condition. A control that fails open under error is a warning
  wearing a control's label.
- **Phase NOTICE → PROBLEM, and name the flip (D4).** Land advisory, census the inherited corpus,
  flip fatal on a named date or condition. No permanent NOTICEs.
- **Presence is the control; adequacy is review (D7).** Each check enforces that an artifact is
  there; none judges whether it is any good, and each failure message must say which half it covers.
- **Rule-tag every emitted message**, so the 30-day firing audit can attribute it and brief 04 can
  derive from it.
- **Positive control on every check (D1)** — including 06's check on itself.

## Dereferencing

Every brief carries at least one DEREFERENCE row that greps the current source for the ABSENCE of
the thing it proposes, plus rows that prove each named seam really exists. All of them were executed
against `origin/main` @ `657cab1` on 2026-08-25, from the repo root:

| Brief | Absence rows (expected to invert at implementation) | Seam rows (exist today) |
|---|---|---|
| 01 | `RiskPathTriggered` unreachable from the lint | `RiskPathTriggersFor`, the separate lint Go module, the shared coupling vector, the topology trigger declarations |
| 02 | no identifier-shaped matcher in the lint | `buildBacktickRe`, `plannedRe`, the bare-filename skip |
| 03 | no obligation class in the closed set | `knownRowClasses`, `legacyRowClass`, `verifyRowClassProblems`, `changedPathsSince` |
| 04 | no workflow calls the byte-diff tool | the stale enforcement claim is live in the authoring guidance, the check that contradicts it ships, the guardrail block delimiter, `ruleTagFor`, `unattributed:` |
| 05 | no brief generator in the lint or the guidance | the dispatcher's known-subcommand list, the never-overwrite convention, `tokenizeCommand` |
| 06 | no mutation obligation in the row-class source | the mutation harness directory, the firing audit's COLD marker |

A row that still reports absence after the work lands means the work did not land.

## Verification of this pull request

- The lint was run against a checkout of `main` with this stream staged in: `LINT: PASS`, zero
  PROBLEMs, and zero notices attributable to any file in the stream.
- The board regenerates cleanly with the stream present: it appears in the platform roll-up
  (`0/6`, active, P2) and briefs 01 and 02 enter Next up at wave 0.
- `STATUS.md` is not part of this diff.

## Reviewer questions

1. Is the wave graph honest — is 01 genuinely the head, or does something upstream block it?
2. Is 04's placement behind 01 (shared generated region) the right call, or is it over-serialised?
3. Does every brief's NOTICE → PROBLEM phasing commit to a flip, rather than leaving a permanent
   advisory?
4. Does any brief overreach from presence into adequacy — i.e. claim to check quality that only a
   reviewer can judge?
