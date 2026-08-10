---
brief: methodology-metrics/08
title: Next-up claim-aware — exclude briefs with an open branch/PR
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [156]
schema: brief-v1
authored: 2026-07-09 by sonnet (issue #156)
sources: ["#156", "[I-15](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-statusgen-make-next-up-claim-aware-exclude-briefs-with-an-op.md) (INTAKE)", "tools/statusgen/nextup.go"]
---

# Brief 08 — Next-up claim-aware — exclude briefs with an open branch/PR

## Context
files: `../assay-toolkit/statusgen/nextup.go`, `../assay-toolkit/statusgen/gitinfo.go`, `../assay-toolkit/statusgen/main.go`.
facts:
- Next-up is a **read model with no claim**: it recomputes the same priority/staleness-ranked
  list from `docs/streams/` every time, so any two sessions reading it converge on the same top
  picks. Nothing removes a taken item.
- Real collision: PR #138 implemented frontend/15, reviewer-approved and CI-green; PR #152 then
  implemented the same brief from scratch (common ancestor = `main` only).
- Reproduced again while scoping this brief: `ledger-hardening/06`, `/07`,
  `privacy-hardening/01`, `/02` all appeared in a live Next-up read while already having open
  PRs (#145, #146, #155, #151).
- The repo's own convention — **one brief = one branch = one PR** — is already a claim lock;
  statusgen just doesn't consult it. No new register needed (deliberately not a `CLAIMS.md`/
  lease file — that reintroduces shared-file write contention and abandoned-claim reaping; a
  branch is a durable, self-expiring claim already).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `listRemoteBranches(root string) ([]string, bool)` (or equivalent) to
   `../assay-toolkit/statusgen/gitinfo.go`: `git ls-remote --heads origin`, bounded by a short timeout
   (e.g. 3s via `context.WithTimeout`). On any error/timeout, return `(nil, false)` — callers
   must treat this as "no claims known," never as "everything is claimed." Make it pluggable
   (package-level var or equivalent) so tests don't need a live network call.
2. Add branch → `stream/NN` claim parsing (new file, e.g. `../assay-toolkit/statusgen/claims.go`):
   - Recognize this repo's actual branch-naming conventions (verified against
     `git branch -r` at authoring time): `fix|feature|feat|docs|chore/<stream>-<NN>[-slug]`
     (e.g. `fix/ledger-hardening-06-idempotency`) and `<stream>/brief-<NN>[-slug]`
     (e.g. `methodology/brief-05`). Small, documented regex; an unmatched branch is a no-op —
     never hides an unrelated brief.
   - Resolve the captured stream token against the actual list of stream names: exact match
     first, else a **unique** hyphen-boundary prefix match (branches sometimes abbreviate —
     e.g. `fix/privacy-01-price-feed-read` → token `privacy` → stream `privacy-hardening`).
     An ambiguous or absent match is a no-op.
3. Thread a `claimed map[string]bool` (keyed `"<stream>/<NN>"`) through `eligible()` and
   `nextUp()` in `../assay-toolkit/statusgen/nextup.go`: a `todo`/`in-progress` brief with a live claim is
   dropped from the Next-up candidate set. It still appears in the stream's own brief table —
   only Next-up excludes it.
4. Wire it in `../assay-toolkit/statusgen/main.go`'s `run()`: compute the claim set once (gracefully
   degrading to empty on failure) and pass it into `nextUp()`. Applies to `write`, `check`, and
   `lint` modes alike (lint is documented as "a true superset of generation").
5. Document the two supported branch patterns (regex + examples) as a doc comment on the
   parsing function — that comment *is* the documentation the acceptance criteria ask for.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Claim` | exit 0 — parse/resolve unit tests (canonical + abbreviated-prefix + unmatched-branch cases) |
| 2 | `go test ./tools/statusgen/ -run NextUp` | exit 0 — a `todo` brief with a fixture claim is excluded from `nextUp()`'s picks; the same brief with the claim removed reappears |
| 3 | `go test ./tools/statusgen/ -run RemoteBranches` (or equivalent) | exit 0 — `listRemoteBranches` round-trips against a local bare-repo remote (no live network needed) |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` | exit 0, no hang, on a network-unreachable host (simulate via an unroutable/invalid remote or short timeout) |
| 5 | `statusgen --root .` on the real tree | Next-up omits `ledger-hardening/06`, `/07`, `privacy-hardening/01`, `/02` (all have open PRs today) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run Claim` | 0 | TestParseBranchClaim, TestClaimedBriefs, TestNextUpClaimAware PASS | 2026-07-10 | opus-verifier |
| 2 | `go test ./tools/statusgen/ -run NextUp` | 0 | ok | 2026-07-10 | opus-verifier |
| 3 | `go test ./tools/statusgen/ -run RemoteBranches` | 0 | bare-repo round-trip ok | 2026-07-10 | opus-verifier |
| 4 | `go vet && --lint` | 0 | vet clean; lint ~1.5s, no hang | 2026-07-10 | opus-verifier |
| 5 | regen omits claimed briefs (lh/06,07, ph/01,02) | 0 | all four absent from Next-up (now partly status-driven since authoring; claim branches still exist on origin; mechanism unit-proven by TestNextUpClaimAware) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — Next-up excludes briefs with an open claim branch; no lint hang. Row 5 scenario decayed with time but mechanism unit-verified.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
