---
brief: assay-dogfood/03
title: statusgen as a pinned release — build/tag in assay-toolkit, consumers verify by hash
wave: 1
depends: ["assay-dogfood/01"]
unblocks: ["assay-dogfood/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md))
sources: ["INTAKE [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)", "INTAKE [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) (phase ② — statusgen first: biggest integrity win, least friction)", "tools/statusgen (the source being relocated)", "docs/streams/desk-tools/scoping.md C-1 (pinned-binary precedent)", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  statusgen is the methodology's derivation engine — the single most integrity-sensitive
  agent-writable artifact (red-team [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md): the linter itself lives in the repo the agents
  work). Moving its source of truth to assay-toolkit and consuming it as a hash-verified
  pinned release is [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)'s phase ②, sequenced first because it buys the biggest integrity
  win with the least workflow friction.
---

# Brief 03 — statusgen as a pinned release

## Context
files: `../assay-toolkit/tools/statusgen/` (relocated source), `../assay-toolkit` release
process; consumer side in THIS repo: `../oit/.github/workflows/status-regen.yml` + any workflow
invoking statusgen, `Makefile`/docs invocations, a pin file (e.g. `.assay-versions`)
facts:
- Source MOVES (not forks): after cutover, `tools/statusgen` in this repo becomes a stub
  README pointing upstream + the pin file. Transition period: both exist, releases are
  canonical, in-repo copy frozen (any in-repo statusgen edit during transition = lint
  PROBLEM via a tripwire the implementer adds to CI grep — cheap, temporary).
- Release artifact: tagged semver in assay-toolkit, binaries for darwin-arm64 + linux-amd64
  (CI runners), `checksums.txt` in the release. Consumers fetch by tag, verify sha256
  against the PINNED hash committed in the consumer repo — the pin file carries tag + hash,
  so a re-tagged release cannot silently substitute a binary.
- Consumer invocation change: `go run ./tools/statusgen` call sites move to the pinned
  binary path (CI: download+verify step; local: documented install location). Keep ONE
  documented fallback (`go run` against the assay-toolkit checkout) for offline/bootstrap.
- Circularity ([I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)): statusgen lints THIS repo's briefs including this brief — during
  transition the frozen in-repo copy keeps doing that job until the release path is green,
  then the pinned binary takes over. Record the switchover commit in Evidence.
- The dev/prod deploy machinery is untouched — statusgen is repo-tooling only.
- Consumers to enumerate (shared-value rule): status-regen.yml (main CI single-writer),
  PR lint gate, verify-gate workflows (`--verify-issues`/`--close-verify`), local desk
  usage documented in CLAUDE.md, deskboard (desk-tools/02, future). Each gets a row in the
  implementer's consumer table routed fixed-here / follow-up / out-of-scope.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. assay-toolkit pushes are human:<name>'s
  per brief 01's permission model. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Relocate statusgen source into assay-toolkit (history note in the commit, not a blind
   copy); wire its release build (goreleaser or a plain workflow — match assay-toolkit' size,
   don't gold-plate) producing tagged binaries + checksums.
2. Consumer side (this repo): pin file with tag+sha256; CI steps swap to
   download+verify+run; local-use docs updated; the transition tripwire per facts.
3. Consumer table per the shared-value rule (facts, last bullet) in the PR description.
4. Flow-level verify: a full PR-lint run and a main status-regen run each complete using
   ONLY the pinned binary (no `go run ./tools/statusgen` in the executed path).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `shasum -a 256 -c <(grep statusgen .assay-versions)` (or equivalent pin-check command the brief lands) | OK |
| 2 | CI run of the PR lint gate at this brief's head | green, log shows download+verify step, no `go run ./tools/statusgen` |
| 3 | `grep -rn "go run ./tools/statusgen" .github/workflows/ \| wc -l` | 0 |
| 4 | main's next status-regen run post-merge | green via pinned binary (recorded in Evidence by the verifier) |
| 5 | pinned-binary `statusgen --lint` on this repo; echo $? | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item;
     include the assay-toolkit release tag + checksum and the switchover commit. -->

### Non-implementer verifier run — VERIFY: PASS (→ `done`) — glm-5.2-verifier, 2026-07-23

Isolated worktree off `origin/main` (`d26b0ebb`); assay-toolkit verified against a FRESH clone `/private/tmp/verify-ad03-toolkit` @ `aa41ce74` (release tag commit `365e7eca`; local sibling not used). gate: model + irreversible: no.

| # | Command | Exit | Key output | Result |
|---|---|---|---|---|
| 1 | pin-check (sha256 of release linux-amd64 vs `.assay-versions`) | 0 | computed `fb2d3d65…9036` == pin exactly (the literal `shasum -c <(...)` won't run as-written — pin format is `name tag hash #comment`; the CI curl+sha256sum compare is the equivalent pin-check, replicated manually) | PASS |
| 2 | PR lint gate at brief head | 0 | statusgen.yml run on PR #661 head = success: `statusgen v0.1.0 verified (sha256: fb2d3d65…)` → `/tmp/statusgen --lint …`; no `go run ./tools/statusgen` | PASS |
| 3 | `grep -rn "go run ./tools/statusgen" .github/workflows/ \| wc -l` | 0 | 0 (workflows use the pinned binary) | PASS |
| 4 | main status-regen post-merge | 0 | latest run (2026-07-23) = success, green via the pinned binary | PASS |
| 5 | pinned-binary `statusgen --lint` | 0 | exit 0 (NOTICE-level only, no ERROR) | PASS |

**VERIFY: PASS — all 5 rows offline-green.** Release `statusgen/v0.1.0` published 2026-07-20 (darwin-arm64 + linux-amd64 + `checksums.txt`); both binary hashes match `checksums.txt` AND the consumer pin; CI downloads + verifies the pinned binary; main status-regen green via the pin; no `go run ./tools/statusgen` in any workflow. Switchover = PR #661 merge `6f0c55a2` (2026-07-20).

**Non-blocking flags:** (1) the consumer-table covers the 4 CI workflows + CLAUDE.md + runbooks, but `.claude/skills/*/SKILL.md` (intake-desk, the-desk, batch-fanout, author-brief, dailies) still instruct agents to `go run ./tools/statusgen` — agent-guidance docs pointing at the frozen (still-functional) in-repo copy, not CI paths (row 3 is workflows-only = 0), so no Verify failure; transition staleness worth a follow-up. (2) The frozen `../assay-toolkit/statusgen/` stub + 79 `.go` files + tripwire are correctly in place.

Reviewer-App approval on PR #661 (`assay-reviewer-app` APPROVED 2026-07-20).

## Review
Gate: model. Reviewer confirms the hash pin is committed consumer-side (not fetched from the
release it verifies — that would verify nothing), the fallback path is documented but not
default, and the consumer table covers every enumerated call site.
