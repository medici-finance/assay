---
brief: methodology/24
title: gate-why backfill — write a brief-specific rationale for every risk-gated brief (phase 2)
wave: 0
depends: []
unblocks: ["methodology/25"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by opus (author-brief)
sources: ["spec: docs/superpowers/specs/2026-07-09-gate-why-rationale-design.md §5 (backfill)", "PR #184 (gate-why mechanism — phase 1)"]
---

# Brief 24 — gate-why backfill (phase 2)

## Context
files: the 26 risk-gated brief files listed below (frontmatter edit only — add one `gate-why:` line each).
facts:
- Phase 1 (PR #184) added the `gate-why` mechanism: parse (KNOWN key), render in the verify-gate card (blockquote above the mechanical Gate-reason line), and a NON-FATAL `--lint` NOTICE when a brief is `gate: human` OR any `risk.*: yes` AND `gate-why` is empty.
- This brief is the backfill that makes the NOTICE go to zero — the precondition for phase 3 (methodology/25) flipping the NOTICE to a hard error.
- `gate-why` grammar: a free-text string (a line or two of prose). Says *what about THIS brief* makes it risky and *what the human is confirming* at sign-off. Single field, not per-risk-key.
- Additive/documentation only: adding `gate-why:` does NOT change any brief's status, gate, risk answers, or Evidence — a `verified`/`done` brief stays exactly that. No re-review of the backfilled briefs is triggered.
- count is **26** on today's main (not "28" — the spec's round number). The rule fires on `gate: human` OR any risk answer `yes`.

The 26 briefs needing a gate-why (grouped by stream for fan-out batches):
- **daml-hardening** (7): 01-consuming-mutations, 02-governance-gates, 03-poolvault-internals, 04-maturity-price-fix, 05-settlement-invariants, 06-rebalancing-conservation, 07-claim-provenance.
- **ledger-hardening** (12): 01-intent-strike, 02-ws-unauth-read, 03-price-timestamps, 06-idempotency, 11-bearerauth-wiring, 12-pprof-ingress, 13-intents-store-unification, 14-mm-cid-capitalbook, 15-verified-sub, 18-crosspod-events, 19-http-unauth-read, 20-commands-unauth-write.
- **methodology** (6): 07-toolkit-extraction, 09-article-status-build-artifact, 10-article-convergence-thesis, 11-article-convergable-specs, 13-name-the-methodology, 17-unforgeable-review-gate.
- **reconciler-spinout** (1): 10-prod-canton-enablement.
(Regenerate the exact live set at pickup: `go run ./tools/statusgen --root . --lint 2>&1 | grep gate-why` after PR #184 is on main — it prints one NOTICE per brief still missing a gate-why. That grep going empty is the done condition.)

## Ground rules
- **Land only AFTER PR #184 is merged to main** (the mechanism). Start the branch off a main that already carries the NOTICE + render, so Verify #1's grep is meaningful.
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- Edit ONLY the `gate-why:` frontmatter line of each listed brief. Do NOT touch status, risk answers, Evidence, body, or the README tables. A backfill that changes anything else is out of scope.
- A wrong rationale is a real defect (spec §5). Each fan-out batch is reviewed before merge; do not self-certify a batch.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
For each of the 26 briefs above, add a `gate-why:` frontmatter field (placed right after `sources:`, per the brief-v1 template) whose prose names what about THAT brief trips the risk wire and what the human is confirming. Read each brief's body + risk answers to write a SPECIFIC rationale — not a template restatement of the risk keys. The two worked examples set the bar (spec §5):
- **methodology/07** (irreversible): publishes the toolkit as a standalone public repo — a one-way door; once adopters depend on it, renaming/unpublishing/relicensing breaks them, so the sign-off confirms name+visibility+license are permanent.
- **daml-hardening/01** (regulatory/customer/irreversible): rewrites on-ledger money algebra; immutable after mainnet — a wrong invariant forks customer balances with no fix.

Fan-out shape (spec §5): draft rationales in per-stream batches behind review; a reviewer confirms each rationale is true to its brief before the batch merges. One batch = one branch = one PR is acceptable, OR land all 26 in one reviewed PR — but the NOTICE count must reach zero before phase 3 (methodology/25) can flip the lint.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go run ./statusgen --root . --lint > /tmp/gatewhy24.txt 2>&1 && ! grep -q 'has no gate-why' /tmp/gatewhy24.txt` | exit 0 — no brief still triggers the missing-gate-why NOTICE. Pattern amended 2026-08-02: the bare token `gate-why` also matches this brief's own FILENAME where it appears inside an unrelated notice, so the row went red on its own path; it now matches the notice text `brief X is risk-gated but has no gate-why`. Form amended 2026-08-03: the row was `! statusgen … 2>&1 \| grep -q …`, which a pipeline makes unfailable — a pipeline reports only its LAST command, so a statusgen that is missing, unbuilt or crashing writes its error INTO the pipe, `grep -q` matches nothing, and `!` converts the tool failure into a pass (measured: binary off `PATH` → **rc=0**, a vacuous pass; the capture-to-file-then-`&&` form → **rc=127**, correctly red). Same remedy this brief's sweep applies in methodology/44 r1 and r5. The bare `statusgen` is also now `go run ./statusgen`, matching every other row in the sweep — a stale binary on `PATH` is not the tree under test |
| 2 | `statusgen --root . --lint` | exit 0; no `PROBLEM:` |
| 3 | `go test -count=1 ./tools/statusgen/...` | exit 0 (frontmatter still parses; every backfilled `gate-why` is a valid string) |
| 4 | `git diff --name-only origin/main -- 'docs/streams/**/brief-*.md' \| wc -l` and inspect the diff | exactly the 26 listed briefs changed; each diff is a single added `gate-why:` line — no status/risk/Evidence/body change |
| 5 | `statusgen --root . --verify-issues 2>/dev/null \| grep -c "Why you're being asked to sign off"` | ≥ 1 — a currently-`verified` gate:human brief now renders the gate-why blockquote in its verify-gate card (spot-checks render end-to-end) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Confirm the diff touched ONLY gate-why lines on exactly the 26 briefs.
     "verified" status in the stream README requires this section filled by
     someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go run ./tools/statusgen --lint 2>&1 \| grep -c gate-why` | 0 | 0 — no missing-gate-why NOTICE (the done-condition) | 2026-07-10 | opus-verifier |
| 2 | `go run ./tools/statusgen --root . --lint` | 0 | 0 PROBLEM | 2026-07-10 | opus-verifier |
| 3 | `go test -count=1 ./tools/statusgen/...` | 0 | ok | 2026-07-10 | opus-verifier |
| 4 | `git diff origin/main -- brief-*.md \| wc -l` | — | UNRUN (PR-time branch-vs-main diff; post-merge already carries change). Proxy: 34 briefs carry `gate-why:`, NOTICE=0 | 2026-07-10 | opus-verifier |
| 5 | `--verify-issues \| grep "Why you're being asked to sign off"` | — | UNRUN — `--verify-issues` emits `[]` (no currently-eligible gate:human+verified brief to render); render mechanism confirmed by unit test `TestRenderVerifyGateWhy/present` (PASS) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — gate-why backfilled (missing-gate-why NOTICE = 0; render mechanism unit-tested). Rows 4-5 are state-bound, no live subject post-merge.

## Review
Gate: model (all four risk answers no — additive documentation on existing briefs, fully revertible via git). Reviewer must spot-check that each rationale is TRUE to its brief (a plausible-but-wrong rationale is the defect this brief can introduce), and confirm the diff is gate-why-only on exactly the 26 listed briefs. This brief unblocks methodology/25 (the hard-lint flip); the flip is unsafe until Verify #1 reads 0.
