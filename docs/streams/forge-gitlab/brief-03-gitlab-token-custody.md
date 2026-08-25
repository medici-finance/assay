---
brief: forge-gitlab/03
title: GitLab token custody — rotate-on-mint + expiry backstop in desktoken
why: >-
  GitLab PATs are long-lived, and the security-parity ruling forbids shipping a custody
  downgrade from GitHub's minted short-lived tokens. Rotate-on-mint closes the gap: every
  mint atomically invalidates the previous token, so at most one credential per role is
  ever valid and a captured token dies at the next mint — parity by a different
  mechanism, which is exactly what the profile promises.
wave: 2
depends: ["forge-gitlab/01"]
unblocks: ["forge-gitlab/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §5 (rotate-on-mint, expiry backstop, file custody)"
  - "freshness-checked 2026-08-24 @ 5c4a67d — desktoken has no --forge flag and no GitLab path"
exec-tier: strong
exec-tier-why: "credential machinery where a subtle error (stale token left valid, value leaked to argv/logs) survives the happy path (question c)."
domain: complicated
---

# Brief 03 — GitLab token custody

## Context
files:
- `tools/desk/cmd/desktoken/` — `--forge gitlab <role>` path.
- `tools/desk/internal/deskkit/forge_gitlab.go` (planned) — created by forge-gitlab/02; consumes the token-file contract.

single-point-of-failure: rotation atomicity is the one control keeping a single valid
credential — backed by the expiry backstop (an unrotated token dies on its own within
the policy window), which fails for a different reason in a different component.

facts:
- Rotation: `POST /personal_access_tokens/self/rotate` returns a NEW token and
  invalidates the caller's token atomically; the new value MUST be written 0600 to the
  role's token file BEFORE the old value is discarded from memory — a write failure
  after rotation is a lockout, so write-then-verify, and on failure print the recovery
  path (re-issue via group owner), never the token.
- File contract (unchanged from GitHub custody): `<config>/gitlab-<role>.token`, mode
  0600, desktoken prints the PATH only — never the value, never env, never argv.
- Expiry: rotation sets expiry per the group policy; document 7-day RECOMMENDED in the
  command help. Do not implement policy enforcement — that is group configuration,
  covered by forge-gitlab/04's runbook and verified in forge-gitlab/05.
- Concurrency: roles are single-window; a second concurrent mint invalidates the
  first's token BY DESIGN — say so in help text rather than adding locking.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per
  the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `desktoken --forge gitlab <role>`: read current token file → rotate via API →
   write-verify new value 0600 → print path.
2. Refusal behaviors mirror the GitHub path: wrong file mode, missing file, non-file
   custody all refuse with named remedies.
3. Unit tests with a fixture server: rotation success, write-failure lockout message
   (no token in output), mode refusal.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/desktoken/... -v` | exit 0; output contains `PASS` |
| 2 | `! (go run ./tools/desk/cmd/desktoken --forge gitlab worker 2>&1 \| grep -qE -e 'glpat-' -e '^[A-Za-z0-9_-]{30,}$')` | exit 0 — no token-shaped value in any output path (dereference: run against the fixture env from the test README) |
| 3 | `go test ./tools/desk/cmd/desktoken/... -run TestRotateInvalidatesOld -v` | exit 0; fixture asserts old token rejected after mint |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Reviewer answers: with rotation bypassed, does the expiry backstop still bound
exposure (negative-path check on the layered design)?
