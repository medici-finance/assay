---
brief: desk-tools/05
title: "Escape-valve `Decide()` primitive in deskkit — enum-bounded agent consults for deterministic loops"
why: >-
  Deterministic loops hit situations code can't classify (is this FAIL rot or regression? is this
  refusal retryable?), and today they either guess badly — a reviewer-retry incident tripped a
  15-minute breaker by retrying a refusal 5× — or stop. A bounded ask-an-agent primitive gives
  every desk loop judgment at logged decision points without ever growing the loop's action space.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-17 by a re-scope session (pr-shepherd); re-homed to the desk-tools board 2026-08-26
sources:
  - "Direction for an agentic escape valve with a limited vocabulary, for all the desks."
  - "The incident where reviewer subagents retrying a refusal 5× tripped the per-repo breaker (the RETRY|REROUTE|ESCALATE case this primitive exists for)."
  - "The drain engine this plugs into and the guardrail module that is its natural neighbour."
exec-tier: strong
exec-tier-why: "the contract (vocabulary grammar, reserved verbs, journal shape) is a design decision every desk loop inherits."
---

# Brief 05 — Escape-valve `Decide()` primitive in deskkit

## Dependencies
The drain engine and guardrail module this originally referenced have landed outside this
stream, so no typed `depends:` edge remains (the source frontmatter already carried none). This
brief lands the primitive + contract only; consumer adoption is a separate follow-up.

## Context
files: `tools/desk/internal/deskkit/decide.go` (new) + `tools/desk/internal/deskkit/decide_test.go`,
`docs/streams/desk-tools/decide.md` (planned) (new — the contract doc).

single-point-of-failure: the enum validator — behind it: the fail-closed default (an invalid or
absent answer never acts, it falls to the pre-declared conservative option) and the
per-item/per-hour call budget (a confused loop degrades to its no-valve behavior, it never
spins). Independent: they fail for different reasons in different places.

facts:
- signature shape: `Decide(question, context, vocabulary, default, budget)` → member of
  vocabulary; the agent ADVISES, the loop ACTS — only among moves it could already make.
- the consulted agent is read-only: structured context in, one enum member + one-line
  justification out; never a shell, never tools.
- RESERVED VERBS: no vocabulary may contain a member equivalent to a human-gate action
  (approve/flip/merge/ready/sign/close-gate) — build the deny-list into the constructor, reject
  at registration, not at answer time.
- every call journals: question, context digest, answer, justification, elapsed — to the loop's
  journal (audit-line pattern).
- injection posture: context includes untrusted repo text; enum validation bounds an injection to
  picking a wrong-but-safe branch, and anything malformed lands on default.
- the valve is an OPTIMIZATION: every consumer must run correctly with the valve disabled (env
  kill-switch).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `deskkit.Decide` per the facts: constructor validating vocabulary (non-empty,
   default ∈ vocabulary, reserved-verb deny-list), agent dispatch with timeout, enum validation,
   fail-closed default, journal entry, per-item + per-hour budgets, kill-switch.
2. Write `docs/streams/desk-tools/decide.md` (planned): the contract, the reserved-verb list, and two
   worked examples (verify FAIL triage: REBASELINE|REGRESSION|RETRY|ESCALATE; refusal handling:
   RETRY|REROUTE|ESCALATE).
3. Do NOT wire consumers here — verifyloop/reviewer adoption are their own follow-ups; this brief
   lands the primitive + contract only.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go test -run TestDecide ./internal/deskkit/` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test -run TestDecideInvalidAnswer ./internal/deskkit/ && go test -run TestDecideTimeout ./internal/deskkit/ && go test -run TestDecideBudgetSpent ./internal/deskkit/` | exit 0; each case resolves to the DEFAULT, journaled (chained single-pattern runs; `-run` compiles RE2, so a table-cell alternation would match nothing) |
| 3 | check:ci | `cd tools/desk && go test -run TestDecideReservedVerbs ./internal/deskkit/` | exit 0; vocabulary containing "merge" refused at construction |
| 4 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Rows 2–3 are the negative-path rows (fail-closed + deny-list, independently).

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — VERIFY: PASS — 2026-08-26 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `b734dab`
Runner != implementer. Own isolated worktree off `origin/main`, OFFLINE (`KUBECONFIG=/dev/null`). gate: model, all risk no. Deliverables present: tools/desk/internal/deskkit/decide.go + decide_test.go + the contract doc. `tools/desk` and `statusgen` are their own modules.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `cd tools/desk && go test -run TestDecide ./internal/deskkit/` | exit 0 | exit 0 — ok deskkit 0.257s | 2026-08-26 | opus-4.8[1m]-verifier |
| 2 | `go test -run TestDecideInvalidAnswer && TestDecideTimeout && TestDecideBudgetSpent` | exit 0; each resolves to DEFAULT, journaled | exit 0 all three — InvalidAnswer/Timeout/BudgetSpent present + green | 2026-08-26 | opus-4.8[1m]-verifier |
| 3 | `go test -run TestDecideReservedVerbs ./internal/deskkit/` | exit 0; a vocabulary containing a human-gate verb (e.g. merge) refused at construction | exit 0 — subtests PASS incl. merge, cased-Merge, approve, flip, ready, sign, close-gate, ready-flip | 2026-08-26 | opus-4.8[1m]-verifier |
| 4 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0 — LINT: PASS (NOTICE-only; none for this item) | 2026-08-26 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — the reserved-verb deny-list + reserved-phrases @ tools/desk/internal/deskkit/decide.go:70,86 — the set equals the human-gate action roots the house forbids a model from taking (approve/flip/merge/ready/sign/close-gate per CLAUDE.md), enforced at construction; TestDecideReservedVerbs proves merge/cased-Merge are refused at registration. The 30s default timeout and per-hour budget window are reversible operational knobs, rank last.

Note (not a Verify FAIL, for the reviewer): the contract doc landed at tools/desk/internal/deskkit/decide.md (co-located with the code), not the brief's stated docs/streams/desk-tools/decide.md — a location discrepancy, content present, lint clean.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
