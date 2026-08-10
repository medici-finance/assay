---
brief: desk-tools/01
title: deskkit foundation — shared config/audit/kill-switch/rate-limit/version + human install path
wave: 0
depends: []
unblocks: ["desk-tools/02", "desk-tools/03", "desk-tools/04", "desk-tools/05", "desk-tools/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md", "INTAKE [I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  Every desk tool needs identical safety plumbing (kill switch, audit, rate limits, pinned-binary
  versioning). Building it once, tested, means briefs 02-05 cannot each reinvent it with holes —
  the whole zero-prompt design rests on this layer being right.
---

# Brief 01 — deskkit foundation

## Context
files: create `../assay-toolkit/tools/desk/internal/deskkit/` (new), the tools/desk README (created by this brief),
`Makefile` (repo root — add `desk-install` target; create Makefile if absent),
reference `../assay-toolkit/statusgen/` for how a Go tool is wired into this repo's Go workspace
(`go.work` at repo root)
facts:
- Constraints implemented here (scoping.md, verbatim contract): **C-1** (version stamping +
  human install), **C-4** (fixed repo set), **C-5** (audit + rate limit + idempotency store),
  **C-6** (kill switch), **C-10** (fail-closed helpers + canonical exit codes).
- Canonical exit codes (README "Shared conventions"): 3 disabled, 4 rate-limited, 5 refused by
  constraint, 6 precondition unverifiable, 0 success/idempotent-no-op.
- Fixed repo set (C-4): `oit`, `example-org/agent-runtime`,
  `example-org/medici-examples` — plus, per human:<name> 2026-07-10 (the review/verify gates cover the
  report/product repos): `medici-finance/assay-toolkit` (the surviving repo of the
  assay-tools→assay-toolkit merge), `medici-finance/reconciler`, `medici-finance/decks`,
  `medici-finance/proposals`. Compiled in. No env/flag may override it. ([F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md); set expanded
  post-`implemented` — Verify table unchanged, so no demotion.)
- Audit file: `~/.claude/desk-tools/audit.jsonl`, append-only from the tools' perspective
  (open O_APPEND; never truncate/rewrite). Line schema:
  `{"ts":RFC3339,"tool":str,"verb":str,"argsDigest":sha256-hex-of-args,"repo":str,"pr":int|null,"headSHA":str|null,"result":"ok|noop|refused|disabled|ratelimited|unverifiable","detail":str,"sourceSHA":str,"builtAt":str,"sessionTag":str}`.
  sessionTag = `$CLAUDE_SESSION_ID` if set, else `"unknown"`.
- Kill switch (C-6): file `~/.claude/desk-tools/DISABLED` OR env `DESK_TOOLS_DISABLED=1` →
  every tool exits 3 after writing an audit line with result=disabled. Checked BEFORE any other
  action, including reads.
- Rate limit (C-5): outward-write verbs only; ≤10 per tool per rolling hour, counting ALL
  attempts (any result — a refusal-loop must trip it), `ts ≥ now-1h`, grouped by tool, counted
  from the audit file itself. Breach → exit 4.
- Idempotency (C-5): per-verb keys (scoping v2): `ready` (repo,pr,head); `review`
  (repo,pr,head,verdict); `comment` (repo,pr,head,bodyDigest); `pr-create`
  (repo,branch,head). **Only prior entries with `result ∈ {ok,noop}` count as done** — a
  refusal followed by state change is a legitimate retry. Noop prints what it deduplicated
  against. Audit line for outward writes is appended AFTER the remote call succeeds; outward
  verbs hold an `flock` on the audit file across check→call→append (stale lock 60s → exit 6).
- Failure states (C-5): missing file/dir = empty history (bootstrap, dir 0700/file 0600);
  unreadable/malformed line = exit 6 for that lookup, printing the recovery procedure (human
  moves file to `audit.jsonl.corrupt-<ts>`; tools never truncate/rewrite). Deletion fails
  open — recorded residual; expose `FirstTS()` so deskboard can banner a suspicious reset.
- Secret scan (C-3, shared): `bodycheck` lives HERE in deskkit (deskpost/deskpr/deskreply all
  consume it): refuse `ghp_`/`github_pat_`/`ghs_`/`gho_`, `AKIA[0-9A-Z]{16}`, PEM headers,
  `eyJ` 3-dot JWT shapes, `sops`/`ENC[` markers, ≥32-char base64ish runs EXCEPT exactly 40- or
  64-char lowercase-hex (git SHAs pass). Test vectors both directions are deliverables.
- Version stamping (C-1): package variables `sourceSHA`, `builtAt` set via
  `-ldflags "-X ...deskkit.SourceSHA=<sha> -X ...deskkit.BuiltAt=<ts>"` by the Makefile target;
  `deskkit.Version()` returns them; unset (a `go run` invocation) reads `"unpinned"` — tools
  print a WARNING to stderr when unpinned so drift is visible in every transcript.
- Install (C-1, human:<name> 2026-07-10): `sudo make desk-install` → `/opt/desk-tools/bin/`, root-owned
  0755 — the sudo password IS the manual permission gate; agents cannot write there. The target
  also writes `../assay-toolkit/tools/desk/MANIFEST.sha256` (committed) for drift verification. The Makefile
  target must therefore work under sudo (build as the invoking user via SUDO_USER or build-then-
  sudo-copy; implementer picks, documents it, and tests the non-sudo build half).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state (e.g. no `go.work`, a Makefile conflict):
  report NEEDS_CONTEXT, don't guess (this suite's own C-10 applies to building it, too).

## Task
1. Create `../assay-toolkit/tools/desk/internal/deskkit` with, as separate files: `config.go` (repo set constant +
   `IsAllowedRepo`), `killswitch.go` (`Guard() error` — the mandatory first call), `audit.go`
   (`Log(Entry) error`, O_APPEND, creates dir 0700/file 0600 on first use), `ratelimit.go`
   (`AllowWrite(tool string) error` reading the audit file), `idempotent.go`
   (`AlreadyDone(repo string, pr int, head, verb string) bool`), `version.go`
   (`SourceSHA`, `BuiltAt`, `Version()`, `WarnIfUnpinned(io.Writer)`), `exitcodes.go`
   (named constants 0/3/4/5/6 with doc comments quoting their meaning).
2. `Guard()` semantics: kill switch → audit(result=disabled) → return typed error the caller
   maps to exit 3. Every helper that cannot verify its input (unreadable audit file, malformed
   line, missing HOME) returns a typed `Unverifiable` error → exit 6 — NOT a silent default (C-10).
3. Wire `tools/desk` into the repo Go workspace the same way `tools/statusgen` is wired
   (inspect `go.work` and statusgen's module layout; mirror it — do not invent a new layout).
4. `make desk-install` (run via sudo by the HUMAN): builds every `../assay-toolkit/tools/desk/cmd/*` (briefs 02-05 add cmds; target must work
   with zero cmds today — guard the glob) into `/opt/desk-tools/bin/` with the ldflags stamp + writes MANIFEST.sha256.
   Document in the tools/desk README: install is a HUMAN act; agents never run it (C-1).
5. Table-driven tests for every helper, including the NEGATIVE paths (C-9): kill-switch armed →
   Guard errors; 30 writes logged in the past hour → AllowWrite refuses; duplicate
   (repo,pr,head,verb) → AlreadyDone true; corrupted audit line → Unverifiable (not skipped);
   repo outside the fixed set → IsAllowedRepo false; audit file is O_APPEND (write twice,
   assert both lines survive).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0; includes the negative-path tests named in Task 5 |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `DESK_TOOLS_DISABLED=1 go test ./tools/desk/internal/deskkit -run Guard -count=1 -v 2>&1 \| grep -c disabled` | ≥1 (guard path exercised) |
| 4 | `make desk-build && ls tools/desk/dist/` | exit 0 (build half runs unprivileged; no cmds yet is OK — target must not fail on empty) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/... -count=1` | 0 | `ok github.com/medici/desk/internal/deskkit 0.331s` | 2026-07-10 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | clean | 2026-07-10 | opus-verifier |
| 3 | `DESK_TOOLS_DISABLED=1 go test ...deskkit -run Guard` | 0 | disabled-guard path exercised (2 matches) | 2026-07-10 | opus-verifier |
| 4 | `make desk-build && ls tools/desk/dist/` | 0 | `no tools/desk/cmd/* yet — nothing to build (ok)`; dist empty | 2026-07-10 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | NOTICEs only, exit 0 | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — deskkit foundation builds/tests clean; disabled-guard works; make target is a safe no-op pre-cmd.

## Review
Gate: model. This is safety plumbing: reviewer must check the negative tests actually assert
REFUSAL (not just absence of success) and that no helper silently defaults on error (C-10).
