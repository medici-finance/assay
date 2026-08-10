---
brief: methodology/41
title: 'Phase-1 board provisioning — reconciler and platform-repo generate a Next-up board from the pinned statusgen release'
why: >-
  The multi-repo dispatcher (methodology/40) can only work streams in repos that generate a board.
  oit and assay-toolkit now do; reconciler and platform-repo have streams but no board, so their
  streams stay invisible to dispatch — active-looking but never worked. The "where does statusgen
  come from" half of this brief is DONE: it is a pinned release binary from assay-toolkit
  (assay-dogfood/03), verified by tag+sha256 in `.assay-versions`. What is left is provisioning the
  two remaining repos as consumers of that mechanism, so every product repo emits a board.
wave: 0
depends: []
# Deliberately NOT ["methodology/40"] — do not "restore" it. 40 declares `depends: []` and is
# designed to skip un-provisioned repos "with a logged note, so it degrades gracefully as repos
# come online": 41 ENRICHES 40's repo set, it does not gate 40.
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
revised: >-
  2026-07-26 — Task 1 (single-source mechanism) removed as delivered by assay-dogfood/03; facts,
  Verify rows and effort re-derived against current main.
  2026-07-27 — platform-repo's vendored `statusgen/` fork found and given an explicit disposition
  in Task 1 (it had been omitted, which made the readiness row read greenfield); Verify row 5
  replaced (it was satisfied by the tree it was written against) and rows added for reconciler's
  `.assay-versions` and its manual→generated STATUS.md switch; `unblocks:` corrected back to `[]`.
sources:
  - "human:<name> 2026-07-16: 'each repo with streams to regenerate its status (ideally the statusgen should come from assay-toolkit repo). then when this is done, I want the current batch-fanout to read them.'"
  - "[methodology/40](./brief-40-multirepo-fanout-dispatch.md) — the consumer: multi-repo dispatch reads whatever repos are dispatchable; this brief makes the remaining repos dispatchable."
  - "assay-dogfood/03 (done, reviewed 2026-07-20) — SETTLED the single-source mechanism: statusgen ships as a tagged release binary from assay-toolkit, consumers verify by sha256. Supersedes this brief's original options A–D."
  - "`.assay-versions` + .github/workflows/statusgen.yml in this repo — the reference consumer implementation the adopters copy (App-token mint → release-asset download → sha256 verify)."
  - "[assay-selfcontain/02](https://github.com/example-org/oit/blob/main/docs/streams/assay-selfcontain/brief-02-move-tool-sources.md) — moves the tool SOURCES to assay-toolkit and names reconciler a consumer. Ownership boundary stated in Context below."
  - "example-org/reconciler#27 — reconciler-side tracking item (re-checked OPEN 2026-07-27); Task 2 here is its substance."
  - "[I-three-cell-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-14-three-cell-split-per-cell-desks-master-aggregator.md) — the phase-3 per-cell direction this infrastructure serves"
  - "example-org/platform-repo @ `85678942` — the 2026-07-16 bootstrap commit that vendored `statusgen/` and `lint.yml`; the fork Task 1 must dispose of"
  - "freshness-checked 2026-07-27 against each repo's `main` via the GitHub API (not a local checkout — sibling checkouts here are stale)"

---

# Brief 41 — Board provisioning for the remaining product repos

## Context
files: primarily OTHER repos (each provisioned via its own PR): `example-org/platform-repo`
and `example-org/reconciler`. The only in-repo change is the methodology/40 registration
(Task 3). This brief authors the plan; implementation lands as one PR per target repo.

facts (verified on each repo's `main` via the GitHub API, 2026-07-27):
- **The single-source mechanism is DECIDED and SHIPPED — do not re-open it.** statusgen is consumed
  as a **pinned release binary**: `.assay-versions` carries `statusgen <tag> <sha256>`, and CI
  downloads the release asset from `medici-finance/assay-toolkit` and hard-fails on checksum
  mismatch. This landed as assay-dogfood/03 (status `done`, reviewed 2026-07-20). The in-repo
  `../assay-toolkit/statusgen/` copy is **frozen** — .github/workflows/statusgen.yml carries a tripwire that
  exits 1 on any `^tools/statusgen/.*\.go$` in a PR diff. An earlier revision of this brief asked
  the implementer to choose between four mechanisms (submodule / published module / CI
  fetch-and-run / copy); the answer is none of them, and that section is gone.
- **Readiness.** oit ✅ board. assay-toolkit ✅ board (`STATUS.md` present on `main`; the bootstrap
  bug is fixed — assay-toolkit#82 and #83 both closed). **reconciler** ❌ — one workflow (`ci.yml`),
  no `.assay-versions`, `STATUS.md` present but hand-maintained (it opens *"Update whenever a phase
  item lands… Last updated: 2026-07-16"*), and `docs/streams` holds a single directory
  (`code-review-2026-07-23`). Reconciler carries **no** vendored statusgen — its `tools/` holds one
  file, `render-report-pdf.cjs` — so on the tool it is genuinely greenfield. **platform-repo** ❌
  board, and **NOT greenfield** — see the next fact, which is the one that changes the work.
- **platform-repo already vendors statusgen, and its "one workflow" IS the statusgen workflow.**
  Do not read `lint.yml` as unrelated CI. `example-org/platform-repo` `main` carries a complete
  vendored statusgen **source module** at repo root — `statusgen/`, 28 files (`main.go`, `checks.go`,
  `nextup.go`, `linkcheck.go`, … + tests + `testdata`), `go.mod` declaring
  `module github.com/medici-finance/assay-toolkit/statusgen` — and `lint.yml` is exactly
  `cd statusgen && go run . --root .. --lint` on `ubuntu-latest`. It was landed by `85678942`
  (2026-07-16, *"bootstrap: platform-repo"*) and untouched since, so it predates the pin badly:
  its `main.go` declares three flags only — `--root` (a plain string, **not** repeatable), `--check`,
  `--lint`. No `--budget`, no `--changed`, no `--record`, no `--version`, no darsync, no multi-root
  `runRoots`, no findings state machine. That is a **fourth divergent copy of the tool this brief
  exists to stop copying**, sitting in the seat the new workflow wants. Provisioning around it would
  leave platform-repo running two statusgens at two versions in two workflows, which will disagree
  — the same way oit's frozen `../assay-toolkit/statusgen/` and the v0.5.0 pin already disagree. Task 1 carries
  an explicit disposition for it; that disposition is **on the critical path, not cleanup**.
- **platform-repo's streams are already on `main`** (`docs/streams/{README,FINDINGS,INTAKE}.md` +
  the `platform-build` stream). The original brief gated Task 2 on branch `feat/platform-build-stream`
  landing; that branch is gone and its content is merged, so the coordination step is **discharged** —
  provision directly, no branch-owner handshake needed.
- **statusgen `--root` is repeatable** (`repository root (default "."; repeatable — one STATUS.md per
  root, assay-selfcontain/01)`), backed by a `runRoots` driver and a `crossRootProblems` guard.
  Relevant to methodology/40, which may prefer one multi-root invocation over N per-repo workflows —
  but **this brief provisions per-repo workflows**, because each repo owns its own `STATUS.md` under
  the single-writer rule. Flagged so 40 can make that call with the fact in hand.

**Ownership boundary vs assay-selfcontain/02.** That brief MOVES tool sources into assay-toolkit and
names reconciler a consumer. This brief does NOT move any source; it provisions **boards** in two
repos against the already-shipped release mechanism. If selfcontain/02 lands first, this brief is
unaffected — it consumes releases either way. Do not duplicate its source-relocation work here.

**Venue question — settle before dispatch (desk/human:<name> call, not the implementer's).**
`../oit/docs/streams/assay-selfcontain/README.md` § "Locked decisions (human:<name>, 2026-07-25)" schedules the
`methodology` stream to MOVE to assay-toolkit. This brief lives in that stream. It may be better
authored/executed in assay-toolkit. The brief is held at `todo` until that is answered; the work
itself is unaffected by where it is tracked.

## Task
1. **Provision platform-repo** (`example-org/platform-repo`) — **and dispose of its vendored
   `statusgen/` in the same PR.** Add `.assay-versions` (pin the same tag+sha256 oit currently pins)
   and a `statusgen.yml` modelled on **this repo's** .github/workflows/statusgen.yml — App-token
   mint → release-asset download → sha256 verify → run. Its streams are already on `main`, so the
   board should generate on the first run. Include the bootstrap-safe guard
   (`git status --porcelain -- STATUS.md`, NOT `git diff --quiet`) so the first board commits.
   **The vendored copy is not optional to handle** — pick one, and say which in the PR body:
   - **Retire (preferred).** Delete `statusgen/` and rewrite `lint.yml` to run the pinned binary
     (or fold its `--lint` into the new `statusgen.yml` and delete `lint.yml`). Nothing else in that
     repo imports the module — confirm with a grep before deleting.
   - **Freeze.** If something does depend on it, keep the directory but stop running it: point
     `lint.yml` at the pinned binary and add oit's tripwire step (exit 1 on any
     `^statusgen/.*\.go$` in the PR diff), so the fork cannot drift further.
   What is NOT acceptable is leaving `lint.yml` running the 2026-07-16 fork beside the pinned
   binary. Note the flag gap while you are there: the fork has no `--budget` / `--changed`, so
   whatever replaces `lint.yml` should adopt CI's real invocation, not the fork's three flags.
2. **Provision reconciler** (`example-org/reconciler`, per reconciler#27): same two files
   (`.assay-versions` + `statusgen.yml`) — no vendored copy to dispose of there — then switch
   `STATUS.md` from the hand-maintained doc to the generated board (single-writer = main CI).
   That switch is the half that can silently not happen; it has its own Verify row (7).
   Its `docs/streams` currently holds one stream — VERIFY the board generates non-empty from that
   one stream rather than assuming; if it does not, report rather than backfill streams here.
3. **Register the newly-dispatchable repos** in methodology/40's board-bearing repo set (the table at
   `brief-40-multirepo-fanout-dispatch.md` line ~40). Both repos are **already rows** in that table —
   as *not-dispatchable* rows (`no — provisioning in-flight`, `no — needs tool+workflow`). The
   deliverable is therefore **flipping those two rows' readiness cells** to the same `**yes**` token
   oit's row already uses, and correcting their `statusgen on main` / `board on main` /
   regen-command cells — not appending lines.
   **While you are in that table, re-derive the whole thing** — four other cells are stale as of
   2026-07-27: assay-toolkit's checkout base cell reads a stale path (no such
   path), its gating condition still reads *"after #83 merges"* (#83 is merged), reconciler's source
   of truth reads `docs/reconciler-agentic-spec` (branch deleted), and oit's regen command reads
   `go run ./tools/statusgen --root .` (that copy is frozen; CI runs the pinned binary). Fixing two
   cells in a table where four others are wrong just books a second pass.

## Ground rules
- NEVER push to `main` / trigger workflows / merge. One draft PR per target repo; stop at `implemented`.
- Each target repo is a SEPARATE repo (not `~/.claude`) — isolate in an **owned worktree of that repo**;
  no out-of-repo declaration needed.
- **Do NOT edit `../assay-toolkit/statusgen/**` in this repo** — frozen, CI-tripwired (see Context).
- NEEDS_CONTEXT over guessing. Two known unknowns, both plausibly human:<name>'s to provision — surface them,
  do not invent credentials or infrastructure:
  - the adopter repos need the **`REVIEWER_APP_PRIVATE_KEY` secret** and the assay-reviewer-app
    installation to reach the private assay-toolkit releases;
  - oit's workflow uses `runs-on: medici-builder` (self-hosted ARC). Confirm whether the adopter
    repos can target those runners or must use `ubuntu-latest`. Datapoint, not an answer:
    platform-repo's existing `lint.yml` runs on `ubuntu-latest` today — which says nothing about
    whether `medici-builder` is *reachable* from it, only that nobody has needed it yet.

## Verify (executable — no prose-only DoD items)
Rows 1–7 take the two PR numbers this brief produces. Set them once from the Evidence block:
`PLATFORM_PR=<n>` and `RECONCILER_PR=<n>` — the implementer records both in Evidence, and the
verifier exports them before running the table.

Row 9 needs the pinned binary on `PATH`; there is no `statusgen` in this repo to `go run` (the
in-repo copy is frozen and diverges). Install it the way CI does — read the tag+sha256 from
`.assay-versions`, resolve the `statusgen-<os>-<arch>` asset on that release of
`medici-finance/assay-toolkit` (private: needs the assay-reviewer-app installation token, minted as
in .github/workflows/statusgen.yml § *Generate assay-toolkit access token*), download it with
`Accept: application/octet-stream`, verify the sha256 against that release's `checksums.txt` (its
`statusgen-linux-amd64` line must equal the pin), `chmod +x`, put it on `PATH`, and confirm
`statusgen --version` prints the pinned tag.

| # | Command | Expect |
|---|---------|--------|
| 1 | `gh pr view $PLATFORM_PR --repo example-org/platform-repo --json files -q '.files[].path' \| grep -F '.github/workflows/statusgen.yml'` | the workflow lands in platform-repo |
| 2 | `gh pr view $PLATFORM_PR --repo example-org/platform-repo --json files -q '.files[].path' \| grep -Fx '.assay-versions'` | it consumes the pinned release, rather than vendoring a statusgen copy |
| 3 | `gh pr diff $PLATFORM_PR --repo example-org/platform-repo \| grep -F 'git status --porcelain'` | the bootstrap-safe guard is present (not `git diff --quiet`) |
| 4 | `gh pr view $PLATFORM_PR --repo example-org/platform-repo --json files -q '.files[].path' \| grep -E -e '^statusgen/' -e '^\.github/workflows/lint\.yml$'` | at least one match — Task 1's disposition of the **vendored** statusgen actually happened (files deleted under `statusgen/`, and/or `lint.yml` rewritten/removed). Silence means the 2026-07-16 fork was left running beside the pin. Then read the PR body: it must name which disposition (retire / freeze) was taken. |
| 5 | `gh pr view $RECONCILER_PR --repo example-org/reconciler --json files -q '.files[].path' \| grep -F '.github/workflows/statusgen.yml'` | the workflow lands in reconciler |
| 6 | `gh pr view $RECONCILER_PR --repo example-org/reconciler --json files -q '.files[].path' \| grep -Fx '.assay-versions'` | reconciler consumes the pinned release too (row 2's counterpart — untested before, and the half most easily skipped) |
| 7 | `gh pr view $RECONCILER_PR --repo example-org/reconciler --json files -q '.files[].path' \| grep -Fx 'STATUS.md'` | match — Task 2's manual→generated switch happened. Reconciler's `STATUS.md` on `main` is the hand-maintained phases doc; a PR that never touches it has left the second half undone. Then eyeball the diff: it either commits the generated board (first line `<!-- GENERATED FILE`) or removes the hand-maintained doc for main's CI to write. |
| 8 | `grep -E -e '^ *. .reconciler. ' -e '^ *. .platform-repo. ' docs/streams/methodology/brief-40-multirepo-fanout-dispatch.md \| grep -c -- '\*\*yes\*\*'` | `2` — both rows in 40's board-bearing table now read dispatchable. **Measured on this PR's head: matches 2 rows, prints `0`, exit 1** — the row fails until Task 3 is done. (The previous version counted *mentions* of the two repo names and already returned `10`: both are already rows in that table, as not-dispatchable ones — it was satisfied by the tree it was written against.) The `.` stand in for the `\|` and backtick delimiters on purpose: a literal `\|` inside a `grep -E` pattern is alternation, not a pipe character, and would match every line. |
| 9 | `statusgen --root . --lint --budget CLAUDE.md:2850 --changed changed.txt` | exit 0 — the in-repo methodology/40 edit passes lint. Uses the **pinned binary** exactly as CI invokes it (see the install note above); `changed.txt` = this PR's changed paths. Not `go run ./tools/statusgen` — that copy is frozen and diverges from the pin. |

## Evidence
<!-- filled by a non-implementer at verify time: one row per Verify item.
     Record PLATFORM_PR and RECONCILER_PR here so rows 1–7 are reproducible, and record which
     disposition (retire / freeze) platform-repo's vendored statusgen/ received. -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
