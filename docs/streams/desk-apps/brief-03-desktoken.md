---
brief: desk-apps/03
title: desktoken — key-parameterized token minter (generalizes mint-reviewer-token.go)
why: >-
  Role = key custody only works if minting a role's token is a single, audited, key-parameterized
  call. Today each role would copy mint-reviewer-token.go; desktoken makes "which key do you hold"
  the whole identity mechanism — one tool, one audit path, every role.
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-apps/04", "desk-apps/05", "desk-apps/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (mint-reviewer-token.go generalizes to a key-parameterized minter)", "~/.claude/skills/pr-review-desk/mint-reviewer-token.go (the 179-line pattern to generalize — read-only reference)", "docs/streams/desk-tools/scoping.md (C-1…C-10 inherited via deskkit)", "desk-tools/03 deskpost (absorbs the reviewer minter today — future consolidation noted)"]
---

# Brief 03 — desktoken (key-parameterized token minter)

## Context
files: create `../assay-toolkit/tools/desk/cmd/desktoken/` (new); uses `../assay-toolkit/tools/desk/internal/deskkit` (desk-tools/01).
facts:
- Generalizes `mint-reviewer-token.go` (read it for the exact JWT→installation-token flow): RS256-sign
  a short JWT (iat −60s, exp +9min, iss = App ID) with the role's PEM → `POST
  /app/installations/<installID>/access_tokens` → ~60-min token; never print it.
- **Role-parameterized:** `desktoken <role> [--repo <slug>] [--ttl]`. `<role> ∈ {reviewer, verifier,
  worker, desk, issue-loop, intake-loop}`. Reads `~/.config/adopter/<role>-app.pem` (0600). App ID
  resolved via `deskkit.AppID(role)` — env `<ROLE>_APP_ID` (e.g. `REVIEWER_APP_ID`) else
  `~/.config/adopter/apps.env`; no source default (App IDs are never baked in). Install ID
  auto-picked by target owner (the-org vs medici-finance) from the repo slug; `<ROLE>_INSTALL_ID`
  overrides.
- Per-install cache: `~/.config/adopter/<role>-token[-<installID>]` (reuse if <50 min). 0600.
- Inherits deskkit: C-5 audit (one line per mint), C-6 kill-switch (first action + before any key
  read), C-10 fail-closed (unverifiable install/key → exit 6). Exit codes per desk-tools convention.
- **Relationship to deskpost:** desk-tools/03 (deskpost) absorbed the reviewer minter inline.
  desktoken is the generalized form; a future consolidation points deskpost at desktoken internally.
  Out of scope here — noted, not done.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. Implement `desktoken` per facts; `deskkit.Guard()` first; audit every mint; never print the token.
2. Tests (PATH-shim fake API recording argv): per-role key resolution; install auto-pick by owner;
  cache reuse (<50 min) + expiry (>50 min) refresh; `<ROLE>_INSTALL_ID` override; key-missing/0600-
  violation → exit 6; kill-switch → exit 3; **the token value never appears in stdout/audit/logs**
  (a test asserts this).

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/desktoken/... -count=1` | exit 0; incl. the never-print-token test |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `DESK_TOOLS_DISABLED=1 go run ./tools/desk/cmd/desktoken reviewer --repo oit; echo $?` | 3 |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `3d3708ad`, 2026-07-16). All 4 rows RUN.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/desk/cmd/desktoken/... -count=1` | 0 | `ok …/cmd/desktoken 1.111s`; `TestTokenNeverPrintedToOutput` PASS |
| 2 | `go vet ./tools/desk/...` | 0 | clean |
| 3 | `DESK_TOOLS_DISABLED=1 go run ./tools/desk/cmd/desktoken reviewer --repo oit; echo $?` | 1 / 3 | kill-switch fires correctly — `go run` masks the child exit to 1 with `exit status 3` on stderr; the built binary exits **3** as expected. Row-command nit: `$?` via `go run` is always 1 — amend to use the built binary or `grep stderr "exit status 3"` |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | advisory NOTICEs only |

VERIFY: PASS.

## Review
Gate: model + `/security-review` (credential handling). Reviewer confirms: the role parameterization
is sound, the token is never printed/logged, cache files are 0600, and it fails closed on any
key/install ambiguity (C-10).
