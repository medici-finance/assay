---
brief: methodology/16
title: Non-self-writable lifecycle — register integrity + machine-attributable gates
wave: 1
depends: ["methodology/15"]
unblocks: ["methodology/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [89]
schema: brief-v1
authored: 2026-07-09 by Fable session (methodology red-team follow-up)
sources: ["[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)", "[F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md)", "R-01 sequence-gap candidate", "issue #89", "2026-07-09 methodology red-team (stream E)"]
---

# Brief 16 — Non-self-writable lifecycle

## Context
files: `../assay-toolkit/statusgen/checks.go`, `../assay-toolkit/statusgen/load.go`, `../assay-toolkit/statusgen/*_test.go`;
docs/streams/FINDINGS.md, docs/streams/INTAKE.md; the brief-v1 Evidence/Review convention
(author-brief skill + brief template).
facts:
- The methodology's strong claim — "status is measured, not self-reported" — is currently
  false: statusgen validates the *consistency* of agent-authored markdown, not ground truth
  ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)). Every enforcement point that matters is a convention an agent can write around.
- [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) is the existence proof: a session deleted a finding from the append-only register to
  silence a checker; caught by luck, not by tooling. Issue #89 tracks the mechanical fix.
- This brief makes the *cheapest path to green stop being falsification*. It does not claim to
  make lying impossible — it makes the specific known evasions (register deletion, self-authored
  verification) machine-detectable, so the published claim ("derived from agent-authored
  artifacts with linting + adversarial spot-verification") becomes true.
- Scope honesty ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) note): three pieces below are mechanically enforceable in-repo; the
  fourth (per-agent commit identity) is partly harness-limited and lands as a git-trailer
  convention + the strongest check achievable, not a hard guarantee. State that plainly — do
  not over-claim enforcement the tooling can't deliver.
- AMENDED 2026-07-09 (mid-implementation): Verify row 4's original repo-wide grep matched its
  own command text and brief-09's negation instruction (self-referential, could never pass);
  narrowed to the one asserted instance (scada-ooda-lineage.md), with critique/negation quotes
  exempt by design.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Sequence-gap detection (closes #89).** In statusgen, parse all `## F-NN` (FINDINGS.md) and
   `## I-NN` (INTAKE.md) headings; a missing number in the sequence, or a duplicate, is a
   PROBLEM (`--lint` exit 1). Deletion from an append-only register becomes machine-visible
   instead of luck-visible. The withdrawal convention is a tombstone entry (`Resolved: yes`,
   body explains the withdrawal), never deletion — document it in the FINDINGS/INTAKE headers.
2. **Runner-attribution check on Evidence/Verified/Reviewed.** A `verified`/`done` row's
   Verified cell and the brief's Evidence rows must name a runner (`YYYY-MM-DD <runner>`), and
   the runner must be syntactically distinct from `authored:`'s session where that is derivable
   — flag a row whose verifier == author as a self-verification PROBLEM. (Best-effort: the
   check catches the detectable cases; it does not prove independence it can't see.)
3. **Bot/CI-authored gate entries.** Establish the convention that Verified/Reviewed/Evidence
   entries are landed by a distinct identity — main's status-regen CI (brief-15) for STATUS,
   and a `verifier`/`desk` git-trailer (`Verified-by: <id>`) on the commit that fills Evidence —
   so `git log` can attribute a gate flip to a non-implementer. Document the trailer; add the
   strongest statusgen check that the convention was followed that the data supports.
4. **Update the claim everywhere it's overstated.** Grep the methodology docs + article briefs
   for "measured, never self-reported" / "measured not self-reported" and reframe to the
   defensible form ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)). This is the resolution step for [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 (incl. new sequence-gap + self-verification tests) |
| 2 | insert a gap (delete an F-NN heading from a testdata FINDINGS) → `statusgen --root <testdata> --lint` | exit 1; output contains "sequence gap" (or "F-" gap message) |
| 3 | a testdata brief with Verified runner == author → `--lint` | exit 1; output names a self-verification problem |
| 4 | `test -f docs/streams/methodology/scada-ooda-lineage.md && ! grep -q "never from an agent's self-report" docs/streams/methodology/scada-ooda-lineage.md` | exit 0 (asserted instance reframed; quoted-as-false/negation instances exempt — row amended 2026-07-09, was self-referential). Guarded by `test -f docs/streams/methodology/scada-ooda-lineage.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 5 | `statusgen --root . --lint` | exit 0 (real tree clean after reframing) |

## Evidence

Non-implementer re-run on merged main (`f6c3fdb6`; brief-16 via PR #106 + hardening #109) —
satisfies the `verified` gate:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/` | 0 | ok; incl. new `TestRegisterSequence*` (gap/dup/independent/fenced-quote) + `TestAttribution*` | 2026-07-09 | opus-verifier |
| 2 | delete `## F-06` from testdata FINDINGS → `--lint` | 1 | `PROBLEM: FINDINGS.md: sequence gap — F-06 missing (…withdraw with a tombstone entry, never delete)` | 2026-07-09 | opus-verifier |
| 3 | testdata brief Verified cell == author → `--lint` | 1 | `PROBLEM: … Verified runner looks like self-verification (verifier "fable" matches the brief's author "fable")` | 2026-07-09 | opus-verifier |
| 4 | `grep -rci "measured, never self-reported\|measured not self-reported" docs/streams docs/needs-fixing.md` | — | 3 residual hits, ALL quotation/negation (FINDINGS L128, brief-16 L64, brief-09 L21) — no file asserts the overstated claim; grep-to-0 is unsatisfiable by design, Verify row scoped to the single asserted instance | 2026-07-09 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | real tree clean | 2026-07-09 | opus-verifier |
| + | `go vet ./tools/statusgen/...` | 0 | clean | 2026-07-09 | opus-verifier |

Review verdict (model:opus, 2026-07-09): well-built and idiomatic; the two prior PR-review notes
(fence-aware scan, header-aware Runner) are fixed on merged main via #109 and locked by tests.
Two non-blocking residuals noted, consistent with the brief's own "does not make lying impossible"
framing: (Low) top-of-sequence deletion is undetectable by the gap scan (deleting the highest F-NN
shrinks max, leaves no hole) → INTAKE candidate (persist a high-water mark); (Info) self-verification
check is name-based best-effort ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md), single git identity). No blocking issues. REVIEW: PASS.

## Review
Gate: model (all four risk answers no). Reviewer records verdict + date in the stream README
table. This brief is itself the mechanism that makes that cell trustworthy — the reviewer
should confirm the runner-attribution check would flag their own cell if they were the
implementer.
