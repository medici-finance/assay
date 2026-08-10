---
brief: desk-tools/10
title: 'deskpushguard — git pre-push hook (Go) refuses a push to a MERGED/CLOSED PR branch'
wave: 1
depends: ["desk-tools/01"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-11 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-11: encode the merged-branch-push rule in Go instead of the model's head", "CLAUDE.md 'A merged/closed PR is DONE — follow-up is a new branch, never a push to the old one' (the prose rule this mechanizes)", "incidents 2026-07-10/11: orphaned pushes to merged branches #318, #333, reconciler #3 — three in one session", "the layer-c deny-hooks pattern (force-push/kubectl blocked mechanically — the model this follows)", "freshness-checked 2026-07-11 @ post-#327 main"]
why: >-
  'A merged PR is DONE; follow-up is a new branch' is a prose rule, and prose rules fail
  invisibly — it was violated three times in one session (orphaned commits on merged
  branches, each needing rescue). The fix that already works for force-push and kubectl is a
  mechanical hook: git refuses the push, so no session's memory or discipline is in the loop.
  This is the model applying to ITSELF the mechanize-the-prose-rule lesson it spent the
  session applying to everyone else.
---

# Brief 10 — deskpushguard (pre-push merged-branch hook)

## Context
files: `../assay-toolkit/tools/desk/cmd/deskpushguard/` (NEW; may use `../assay-toolkit/tools/desk/internal/deskkit` for audit
only — this guard must run even when deskkit's kill-switch is armed, so it does NOT call
Guard()); the desk-install Makefile target (installs the hook); a tracked hook template
`../assay-toolkit/tools/desk/hooks/pre-push`
facts:
- **Hook contract:** git invokes `.githooks/pre-push <remote-name> <remote-url>` (F-13
  core.hooksPath = .githooks) with one line per ref on STDIN:
  `<local-ref> <local-sha> <remote-ref> <remote-sha>`. The guard reads stdin, extracts each
  pushed branch (basename of local-ref / remote-ref), and for each runs `gh pr view <branch>
  --repo <origin-derived> --json state,number`.
- **Block condition (the ONLY one):** exit non-zero — aborting the whole push — iff a branch's
  PR state is `MERGED` or `CLOSED`. Message: `refusing: PR #<n> for <branch> is <state> — a
  merged/closed PR is DONE; open a NEW branch for follow-up (CLAUDE.md).` Exit 0 (allow) for
  OPEN, no-PR-found, or any gh/network error.
- **Fail-OPEN on ambiguity — deliberate inversion of desk-tools C-10.** For outward writes,
  fail-closed is right; for a push GUARD, blocking every push when gh is flaky is worse than
  the rare orphan. The guard fires ONLY on positive MERGED/CLOSED confirmation. Document this
  inversion explicitly (it is the one place in the suite that fails open, and why).
- **Escape hatch:** env `DESKPUSHGUARD_OFF=1` skips the check (for the deliberate, rare case
  of intentionally pushing to a closed branch) — its use prints a stderr warning naming the
  branch+state so the override is never silent.
- **Install (C-1 human act):** the desk-install target copies `../assay-toolkit/tools/desk/hooks/pre-push`
  (a 2-line shim that execs the built `deskpushguard`) into `.githooks/` (F-13
  core.hooksPath) — committed directly as `.githooks/pre-push`, covers ALL worktrees
  automatically. Idempotent; refuses to clobber a pre-existing non-deskpushguard pre-push
  hook without `--force` (names it).
- Repo scope: the deskkit fixed set + the two medici-finance repos in gate scope (#273) —
  wherever the desk pushes. `gh`-dependent by nature; never wired into any offline path.

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl. Leave commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Implement deskpushguard: parse stdin refs, derive origin repo, query gh, block only on
   MERGED/CLOSED, honor DESKPUSHGUARD_OFF with a warning; audit each decision (deskkit.Log
   without Guard()).
2. The tracked `../assay-toolkit/tools/desk/hooks/pre-push` shim + the desk-install install step (idempotent,
   no-clobber-without-force, per-repo).
3. Tests (fake gh): MERGED branch → exit non-zero + message; OPEN → exit 0; no-PR → exit 0;
   gh error → exit 0 (fail-open) with a stderr note; DESKPUSHGUARD_OFF=1 on a MERGED branch →
   exit 0 + warning; multi-ref push where one ref is MERGED → whole push blocked.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskpushguard/... -count=1` | exit 0; includes every Task-3 case |
| 2 | `printf 'refs/heads/merged-fixture x refs/heads/merged-fixture y\n' \| DESKPUSHGUARD_FAKE_STATE=MERGED go run ./tools/desk/cmd/deskpushguard origin https://github.com/example-org/oit.git; echo $?` | non-zero; message names the branch + MERGED |
| 3 | `printf 'refs/heads/open-fixture x refs/heads/open-fixture y\n' \| DESKPUSHGUARD_FAKE_STATE=OPEN go run ./tools/desk/cmd/deskpushguard origin https://github.com/example-org/oit.git; echo $?` | 0 |
| 4 | `DESKPUSHGUARD_OFF=1 ...merged-fixture...; echo $?` | 0 with a stderr override warning |
| 5 | `go vet ./tools/desk/... && statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verifier run — VERIFY: FAIL (stays `implemented`) — glm-5.2-verifier, 2026-07-22

Isolated worktree off origin/main `ef1de62a` (`.claude/worktrees/agent-a419a041047831746`); shared checkout not touched. (Desk-tools code unchanged between `ef1de62a` and current main `6e0943b9` — a status-regen commit — so the verdict holds.)

| # | Command | Exit | Key output | Result |
|---|---|---|---|---|
| 1 | `go test ./tools/desk/cmd/deskpushguard/... -count=1` | 0 | 16 tests PASS — MERGED/CLOSED blocked; OPEN/no-PR/gh-error/garbage fail-open; `OFF` warns; multi-ref blocked-if-any-merged; fake-state MERGED/OPEN/OFF | PASS |
| 2 | `printf 'refs/heads/merged-fixture x refs/heads/merged-fixture y\n' \| DESKPUSHGUARD_FAKE_STATE=MERGED go run ./tools/desk/cmd/deskpushguard origin url; echo $?` | 0 | `deskpushguard: cannot derive repo from remote "url": cannot parse repo from URL: url — allowing push (fail-open)` — expected non-zero `refusing: … MERGED` | **FAIL** |
| 3 | `printf 'refs/heads/open-fixture x refs/heads/open-fixture y\n' \| DESKPUSHGUARD_FAKE_STATE=OPEN go run ./tools/desk/cmd/deskpushguard origin url; echo $?` | 0 | same fail-open repo-parse message (NOT the OPEN path) | PASS\* |
| 4 | `printf 'refs/heads/merged-fixture x refs/heads/merged-fixture y\n' \| DESKPUSHGUARD_OFF=1 go run ./tools/desk/cmd/deskpushguard origin url; echo $?` | 0 | `deskpushguard: DESKPUSHGUARD_OFF=1 — skipping guard for branch merged-fixture` | PASS |
| 5 | `go vet ./tools/desk/... && go run ./tools/statusgen --root . --lint; echo $?` | 0 | vet clean; statusgen lint OK (advisory NOTICEs only) | PASS |

\* Row 3 exits 0 but via fail-open, not the OPEN path.

**VERIFY: FAIL — stays `implemented`.** Row 2 as literally written exits 0 (expected non-zero). The manual smoke commands (rows 2–3) pass the bare placeholder `url` as the remote-URL arg; the implementation calls `deriveRepo(remoteURL)` (`main.go:98`) **before** the `DESKPUSHGUARD_FAKE_STATE` seam (`main.go:216-223`), so `parseRepo("url")` rejects the unparseable placeholder → fail-open (`main.go:101`) and the seam is never reached.

**Implementation proven sound** — (a) row 1's unit tests (`TestFAKESTATE_MERGED_Blocked` / `_OPEN_Allowed` / `_OFF_MERGED_Warns`) invoke `run()` with a valid URL, so `deriveRepo` succeeds and every Task-3 case fires; (b) row 2's command with `url` swapped for the real repo URL emits exactly `refusing: PR #42 for merged-fixture is MERGED — a merged/closed PR is DONE; open a NEW branch for follow-up (CLAUDE.md).` (exit 1).

**Root cause = brief Verify-table defect** (the `url` literal is an unparseable placeholder), NOT an implementation defect. Filed as **#1047** — amend Verify rows 2–3 to pass a parseable remote URL (as the unit tests do), then re-verify → flip.

### Non-implementer verify — VERIFY: PASS — glm-5.2-verifier, merged main `78c01343`, 2026-07-25

**#1047 CLOSED-completed** via PR #1050 (commit `0e820769`): the placeholder `url` swapped for `https://github.com/example-org/oit.git`, so `deriveRepo` succeeds and the `DESKPUSHGUARD_FAKE_STATE` seam is actually exercised. All 5 rows PASS.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskpushguard/... -count=1` | 0 | 16/16 PASS — parseRepo/parseRef; MERGED/CLOSED blocked (ExitRefused); OPEN/no-PR/garbage fail-open; gh-error fail-open; DESKPUSHGUARD_OFF warns; multi-ref blocked-if-any-merged; empty stdin; FAKESTATE MERGED blocked / OPEN allowed / OFF+MERGED warns; --version; unparseable remote fail-open | 2026-07-25 | glm-5.2-verifier |
| 2 | `DESKPUSHGUARD_FAKE_STATE=MERGED go run ./tools/desk/cmd/deskpushguard origin https://github.com/example-org/oit.git` (merged-fixture ref) | 1 | `refusing: PR #42 for merged-fixture is MERGED …` (exit 5 = `ExitRefused`) — PASS | 2026-07-25 | glm-5.2-verifier |
| 3 | `DESKPUSHGUARD_FAKE_STATE=OPEN go run …` (open-fixture ref) | 0 | deriveRepo succeeded, fake-state seam reached, `prInfo{OPEN}` → not blocked → ExitOK (genuine OPEN path, not the prior fail-open coincidence) — PASS | 2026-07-25 | glm-5.2-verifier |
| 4 | `DESKPUSHGUARD_OFF=1 DESKPUSHGUARD_FAKE_STATE=MERGED go run …` | 0 | `deskpushguard: DESKPUSHGUARD_OFF=1 — skipping guard …` (override warns, never silent) — PASS | 2026-07-25 | glm-5.2-verifier |
| 5 | `go vet ./tools/desk/... && go run ./tools/statusgen --root . --lint` | 0 | vet clean; statusgen lint OK (advisory NOTICEs only — none reference desk-tools/10) — PASS | 2026-07-25 | glm-5.2-verifier |

**Risk-bearing values.** `ExitRefused = 5` (`deskkit/exitcodes.go:25`, used via the constant at `main.go:129` — DERIVED, deskkit shared exit-code taxonomy); the fail-open semantics at `main.go:99-102 / 108-112 / 247-250` (DERIVED — the C-10 inversion, proven by `TestUnparseableRemoteAllowedFailOpen` / `TestGHErrorAllowedFailOpen` / `TestGarbageOutputAllowedFailOpen`). The verifier returned **NAMED, NOT DERIVED** for three items, all **reversible tooling / external-contract references** out of scope for derivation (risk-value procedure step 3): the GitHub state enum `MERGED`/`CLOSED` (`main.go:117`) — an external API-contract constant matched to `gh pr view --json state`, proven correct by the 16 passing tests using those exact strings (not a magic number, not derivable in-repo); the install-path coupling `/opt/desk-tools/bin/deskpushguard` (`hooks/pre-push:6` ↔ Makefile target); and the `DESKPUSHGUARD_OFF` / `_FAKE_STATE` env names. Recorded here (not buried); no question-issue filed.

## Review
Gate: model. Reviewer confirms (a) it blocks ONLY on positive MERGED/CLOSED (fail-open
otherwise, and the C-10 inversion is documented), (b) a multi-ref push is blocked if ANY ref
is merged, (c) the override warns and never silently passes, (d) installs to .githooks/ (F-13
core.hooksPath), covers all worktrees, (e) it does NOT call deskkit.Guard() (must run even
when the desk-tools kill-switch is armed — a stopped desk still must not orphan commits).
