---
brief: issue-loop/10
title: Reviewed issue-close via per-issue bugs/ carrier + daily bugs-gc prune
why: >-
  Closing a GitHub issue is a judgment call an agent must not make unilaterally (the
  coordinator never closes issues directly). Today that leaves two bad states: resolved
  issues rot open because no one is allowed to close them (issue triage 2026-07-15 found
  #419 and #457 both FIXED-NOT-CLOSED), or someone closes by hand with no reviewed record
  of WHY. This makes closing an issue a reviewed, auditable, conflict-free PR: a per-issue
  bugs/<N>.md carrier lets the reviewer judge the resolution claim, and the MERGE (not a
  manual click) closes the issue. A daily GC prunes the carrier so bugs/ self-empties. The
  whole record lives on GitHub (issue state + merged PR), so it survives the desks moving
  to separate machines, and per-file naming means parallel close-PRs never merge-conflict.
wave: 2
depends: ["methodology-metrics/22"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-15 by the-desk (human:<name> direction)
sources:
  - "human:<name> 2026-07-15: 'for closing issues... you shouldn't be doing that directly. open a PR to the reviewer with your feedback. it will review it, and close the PR if it agrees. this should close the issue'"
  - "human:<name> 2026-07-15: 'we need a bugs/ directory and the # in it to avoid merge-conflicts. and a way of deleting it once it's done'; 'add it to [the daily-jobs brief]'; 'the status-regen sounds good'"
  - "[methodology-metrics/22](../methodology-metrics/brief-22-daily-artifact-harvest.md) — the scheduled AI-free daily-harvest collector this GC step wires into; its extension model: new daily jobs opt in via their own brief adding a one-line wire-in, NOT by expanding mm/22"
  - "coordinator-never-closes-directly (desk rule); issue triage 2026-07-15 — #419/#457 FIXED-NOT-CLOSED with no allowed closer"
  - "desks-coordinate-via-github-not-local (memory) — the durable record is the closed issue + merged PR, not a tree file"
  - "freshness-checked 2026-07-15 @ ea54f844: no bugs/ dir exists; registers.go:131 tombstone guard is scoped to docs/streams/{intake,findings} only, so a repo-root bugs/ is auto-excluded (no guard change needed)"
gate-why: >-
  not risk-gated (gate model). Explicitly NOT touching the statusgen tombstone/integrity
  logic — that would be human-gate (integrity-check-changes-are-human-gate). This brief only ADDS a
  new repo-root dir the guard already does not cover; a Verify row proves the guard is untouched.
---

# Brief 10 — Reviewed issue-close via `bugs/` carrier + daily `bugs-gc` prune

## Context

files:
- `bugs/README.md` (new) — the convention doc
- `tools/bugs-gc/` (new) — Go tool, per repo Go-preference
- `../oit/.github/workflows/daily-harvest.yml` (planned) — mm/22's workflow; add ONE `bugs-gc` step (see Ground rules re: ordering)
- `CLAUDE.md` — Bug tracking section; replace "close by hand" with the close-PR flow

facts:
- **The close-PR flow**: to close issue #N, a worker opens a PR that ADDS `bugs/<N>.md` (the
  resolution claim + evidence) with `Closes #<N>` in the PR body. The reviewer judges the
  CLAIM (is #N genuinely resolved on main?). Agreement → approve → **merge** closes both PR
  and issue via the `Closes` linkage. Disagreement → close-unmerged; issue stays open. The
  agent never touches the issue's close button; the merge is the close.
- **GitHub mechanic (load-bearing)**: `Closes #N` fires ONLY on merge to the default branch.
  Closing a PR unmerged does NOT close the linked issue. The flow's "agree" path is therefore
  approve→MERGE, not close.
- **Per-file, not monolithic**: `bugs/<N>.md` keyed by issue number is globally unique, so two
  parallel close-PRs never touch the same file — no merge conflicts (same lesson as the
  per-entry findings/intake files vs the monolithic FINDINGS.md; #396/#282).
- **`bugs/<N>.md` is a TRANSIENT carrier, not a permanent record.** The durable history is the
  closed GitHub issue + merged PR. So once #N is CLOSED, its file has done its job and is pruned.
- **`bugs-gc`** (Go): lists `bugs/*.md`, resolves each issue's state via `gh issue view <N> --json state`,
  and DELETES every file whose issue is CLOSED. It prints exactly which files it pruned and which it
  kept (open) — never a silent cap. It commits with the `[skip-status-regen]` marker so it does not
  recurse the regen loop (same pattern status-regen/daily-harvest already use).
- **Guard exclusion is automatic**: the tombstone-not-delete guard iterates
  `[]string{"docs/streams/intake","docs/streams/findings"}` (registers.go:131). `bugs/` is a repo-root
  dir outside that set, so `bugs-gc`'s deletions do NOT trip `statusgen --lint`. No guard code change —
  a Verify row proves it (and NOT changing that logic keeps this brief off the human gate).
- **BUGS.md** stays as-is (curated historical record of fixed items); `bugs/` is the transient
  close-carrier, a distinct thing. This brief does not migrate or retire BUGS.md.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- **Ordering vs mm/22**: `daily-harvest.yml` is created by methodology-metrics/22 (PR #425, not yet
  merged). Do NOT recreate it. If mm/22 has merged when you pick this up, add the one `bugs-gc` step to
  the existing `daily-harvest.yml`. If mm/22 has NOT merged, deliver `bugs-gc` + `bugs/` + the convention
  and CLAUDE.md changes, and stage the harvester wire-in as a clearly-marked diff/patch in the PR to
  apply once mm/22 lands (do not fabricate the workflow). Report which path you took.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `bugs/README.md` (new) — document the convention: `bugs/<issue#>.md` is a transient reviewed-close
   carrier; frontmatter `issue`, `verdict` (FIXED-NOT-CLOSED | FIXED-BY-THIS-PR | WONTFIX | DUPLICATE),
   `fixed-by` (commit/PR), `evidence`; body = the close feedback. State plainly: the file is pruned by
   `bugs-gc` once the issue closes; the durable record is the GitHub issue + merged PR.
2. **`tools/bugs-gc`** (Go) — the prune tool above: `--dry-run` (print prune/keep, delete nothing) and
   default (delete closed-issue files). Deterministic, AI-free, `gh`-backed. Unit-tested with issue-state
   injected (no live `gh` in the test).
3. **Wire into `daily-harvest.yml`** — one `bugs-gc` step (per Ground-rules ordering), `[skip-status-regen]`
   marker, `ubuntu-latest` (lightweight, per repo rule).
4. **`CLAUDE.md` Bug tracking** — replace hand-close with the close-PR flow; state the coordinator never
   closes issues directly; note `bugs/<N>.md` carrier + `bugs-gc` prune.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f bugs/README.md && grep -qi "transient" bugs/README.md; echo $?` | prints `0` |
| 2 | `cd tools/bugs-gc && go build ./...` | exit 0 |
| 3 | `go test ./tools/bugs-gc/...` | exit 0; a test asserts a CLOSED-issue file is a prune candidate and an OPEN-issue file is kept |
| 4 | `go run ./tools/bugs-gc --dry-run` (with a fixture bugs/999999.md (new) for a known-closed issue) | output lists bugs/999999.md (new) under prune; exits 0; deletes nothing |
| 5 | `rm -f bugs/999999.md && statusgen --lint; echo "lint-exit=$?"` | no `PROBLEM:` line mentions `bugs/`; deleting a `bugs/` file does not add a tombstone PROBLEM |
| 6 | `grep -rn "bugs-gc" .github/workflows/daily-harvest.yml` (only if mm/22 merged; else check the staged patch) | one step invokes `bugs-gc`; OR the PR carries the staged wire-in diff |
| 7 | `grep -qi "bugs/<N>" CLAUDE.md \|\| grep -qi "close-PR" CLAUDE.md; echo $?` | prints `0` (CLAUDE.md documents the close-PR flow) |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `bfba03ca`, 2026-07-17; PR #567 merged
2026-07-16). **VERIFY: PASS — all 7 rows.**

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `test -f bugs/README.md && grep -qi "transient" bugs/README.md; echo $?` | 0 | prints `0` — README present, "transient" on L1/3/43 |
| 2 | `cd tools/bugs-gc && go build ./...` | 0 | silent success |
| 3 | `go test ./tools/bugs-gc/...` | 0 | `ok github.com/medici/bugs-gc 0.205s`; `TestCollect_ClosedPruned_OpenKept` PASS (CLOSED→prune, OPEN→keep); 12/12 pass, mock-injected `issueLister` (no live `gh`) |
| 4 | `go run ./tools/bugs-gc --dry-run` (+ fixture bugs/999999.md) | 0 | exits 0, deletes nothing. **Caveat:** fixture #999999 does NOT exist (brief assumed closed) → tool resolves UNKNOWN, skips with warning (safe `default` branch, `main.go:85-87`/`101-104`). Row-4 **intent** verified against 11 real CLOSED issues (35,42,47,163,165,191,299,337,420,548,555) listed under prune, `keep: 0`. |
| 5 | `rm -f bugs/999999.md && go run ./tools/statusgen --lint` | 0 | `lint-exit=0`; 0 `PROBLEM:` lines; no `bugs/` mention (tombstone guard ignores `bugs/` deletions) |
| 6 | `grep -rn "bugs-gc" .github/workflows/daily-harvest.yml` | 0 | `bugs-gc:` job **LIVE** (mm/22 merged — not a staged patch): job@78, `go run ./tools/bugs-gc`@105, `[skip-status-regen]`@111, race-safe push retry@113-122. **Sanctioned deviation:** `runs-on: medici-builder` (not the brief's `ubuntu-latest`) — overriding repo rule (CLAUDE.md: all jobs medici-builder since #154); comment @81-85 cites the 2026-07-17 3am ubuntu-latest failure. |
| 7 | `grep -qi "bugs/<N>" CLAUDE.md \|\| grep -qi "close-PR" CLAUDE.md; echo $?` | 0 | prints `0` — CLAUDE.md:101 documents the close-PR flow (`bugs/<N>.md` carrier + `tools/bugs-gc` prune) |

**VERIFY: PASS.** The brief's row-4 precondition ("#999999 is closed") is factually wrong — #999999 does
not exist — but the row's *intent* (`--dry-run` exits 0, deletes nothing, lists CLOSED-issue files under
prune) is fully verified against 11 real closed issues; no code change needed (the fixture choice should be
amended to a real closed issue in a cosmetic follow-up; does not affect the verdict).

**Incidental defect (outside the Verify table, filed as #712):** `tools/bugs-gc/.gitignore` pattern
`tools/bugs-gc/bugs-gc` is repo-root-relative written from a subdir `.gitignore` → does NOT match the
compiled binary (`git check-ignore` = NOT ignored); `go build` leaves it untracked. Low impact (CI uses
`go run`). Fix = pattern `bugs-gc`.

## Review
Gate: model. Reviewer records verdict + date in the issue-loop README table.
