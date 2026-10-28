---
brief: assay:assay:statusgen:38
title: "One-time v1-to-v2 witness backfill where the witnessed content is still fetchable"
why: "Thirty verified briefs are stuck on v1 witnesses whose inputs never changed. Where the witnessed commit is in the commit list of the merged change request that landed the verifier's Evidence, and the brief declares its inputs, a v2 record can be derived from that commit's tree without re-running anything. Every other brief stays v1 and is re-witnessed."
wave: 3
depends: ["statusgen/37"]
unblocks: ["desk-tools/51", "verification-handoff/06", "statusgen/35"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: [2026, 3105, 3658]
schema: brief-v2
outcome: none
version: 2
authored: "2026-10-20 by the-desk (design brief for the witness-staleness class)"
gate-why: "A backfilled record asserts what an old run read without re-running it. The driver decides whether that assertion may release a brief or only document it."
decision-trigger: start
sources:
  - "docs/proposals/fixed-delivery-verification/README.md — fixed-delivery completion amendment, 2026-10-27; planning only"
  - "freshness-checked 2026-10-27 @ 9a58a792c; existing PR 4579 and source/caller work map"
  - "medici-finance/assay#3105 — the stuck verified-to-done report"
  - "medici-finance/assay#3658 — squash-orphaned record shas; the 95 existing records are append-only and are not rewritten"
  - "docs/streams/statusgen/brief-36-content-bound-witness-schema-v2-reader.md and docs/streams/statusgen/brief-37-content-bound-witness-schema-v2-writers.md — the schema, seams and writers"
  - "`statusgen --root . --auto-flip-model --dry-run` at 75cc11f14: 30 REFUSED, 0 flipped"
  - "freshness-checked 2026-10-20 @ 75cc11f1423301ea8ba0b111d48c8d5b8a303537 (origin/main)"
exec-tier: strong
exec-tier-why: "It writes evidence derived from history. Each refusal path (unmerged source, ambiguous or unlisted commit, not verifier-authored, undeclared, dirty, acceptance changed, environment-dependent) must be proven to leave the brief at v1."
domain: complicated
consumers:
  - "tools/desk/cmd/deskevidence/ (the `backfill` subcommand): follow-up statusgen/38"
  - "tools/desk/internal/deskkit/forge.go and its adapters (merged state and full commit list of a change request, per forge): follow-up statusgen/38"
  - "docs/streams/statusgen/README.md (rollout section: which briefs were backfilled, which await a re-witness): follow-up statusgen/38"
---

# Brief 38 — One-time v1-to-v2 witness backfill where the witnessed content is still fetchable

## Context

files:
- `tools/desk/cmd/deskevidence/deskevidence.go`
- tools/desk/cmd/deskevidence/backfill.go (planned)
- tools/desk/cmd/deskevidence/backfill_test.go (planned)
- `tools/desk/internal/deskkit/forge.go`
- `tools/desk/internal/deskkit/forge_github.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- tools/desk/internal/deskkit/witnessseam.go (planned, statusgen/37)
- tools/desk/cmd/deskevidence/testdata/backfill/ (planned)
- `docs/streams/statusgen/README.md`
- changelog/statusgen-witness-v2-backfill.md (planned)

facts (source and date in frontmatter):
- At 75cc11f14 the dry run refuses 30 verified briefs (build-less-brittle 02, 04, 05, 06, 09, 10, 11; contributor-trust 04; desk-supervision 17, 18, 19, 27; desk-tools 15, 23; desktools-v2 05, 09, 12, 14, 15, 61; graph-execution 09, 20; harness-portability 06; iso-9001 03; quality 19; statusgen 15, 17, 19, 22; windows-port 00). Four claims in build-less-brittle 04, 05, 06 and 09 are refused because the Verify row's command changed after the witness. Eight of the briefs have `+dirty` witnesses.
- A v1 Runner cell names a short commit. A squash merge leaves that commit off main, but the forge keeps the merged change request's commit list and head ref (GitHub pull-request head refs (refs/pull/<N>/head), GitLab merge-request head refs (refs/merge-requests/<iid>/head)), so the commit is usually still fetchable. The change request that landed the verifier's Evidence line is found by blaming that line to its first-parent landing commit; statusgen/36's seam reports it as `landing_cr`.
- Authenticity of a v1 witness rests on who authored its Evidence line (statusgen's Evidence-actor check, tamper-evident rather than tamper-proof) and on the existing independence and verifier-floor reads; statusgen/36's seam reports both (`evidence_actor`, `independent`).
- A `+dirty` v1 witness ran on content that no commit holds, so its inputs cannot be recovered.
- No v1 run was in a network-less runner, and no v1 run recorded its tool versions. statusgen/36's environment policy decides whether a back-filled record may release; under that brief's default (option 2), and under its option 4, none does.

layering: deskevidence orchestrates (find candidates, confirm the source change request through the forge-neutral `Forge` interface, fetch on opt-in into a private ref namespace, land an Evidence change). Identity, authorship, the landing change request and the v1 gap come only from `statusgen --coverage --json`; the digests only from `statusgen witnessdigest --json --rev <commit>`. statusgen/36's reader judges the result. No new judge.

single-point-of-failure: the claim that the resolved commit is the one the v1 witness ran on. Behind it, each tripping on its own signal: (1) the source change request is not chosen by anyone; it is the one that landed the verifier-authored Evidence line, and the forge must report it merged, so its commit list is frozen; (2) the full commit must be the only entry of that list starting with the cell's prefix, and the prefix must also be unambiguous among all local objects, so a planted decoy refuses rather than wins; (3) the Verify row's Command, Expect and Class at that commit must equal the current ones; (4) statusgen/36's reader re-checks every digest at the item revision and verifies the back-fill's own change request from its landing commit. A v1 Evidence line forged by a verifier identity is out of scope by statusgen/36's trust assumption (staleness, not forgery).

design-fit:
  owner: tools/desk deskevidence (orchestration), statusgen/evaluation (digests)
  contract: "R-witness-schema (statusgen/36); a back-filled record carries `origin: backfill`, `source_cell`, `source_cr` (the merged change request it was resolved through), `tools_source`, the v1 outcome cells and the verifier's identity, all inside the record id, and its own `cr` bound and verified like any record"
  retires: ["the stuck-brief backlog in medici-finance/assay#3105 where the witnessed content is still fetchable"]
  weight: "one subcommand (`deskevidence backfill`, dry-run by default, never fetching unless asked); flags +2 (`--apply`, `--fetch`); one Forge read (a change request's commit list); no new module or service"
  why-add: "Re-running thirty briefs' rows by hand is the alternative; a derived record is cheaper and is still judged by the same reader."
  reuse-check: "Reuses statusgen/36's seams (identity, Evidence actor, landing change request, digests), statusgen/37's witnessseam, LandedCommit and change-request binding, the forge-neutral change-request ref mapping deskdispatch already uses, and deskevidence's Evidence landing path."

## Human decision

Old verification runs recorded only a commit. For many of them that commit can still be fetched and its files hashed, which shows what the files were when the run happened. It cannot show which tool versions the run used or whether it touched the network.

This question only matters if statusgen/36's environment ruling is its option 1 or option 3. Under statusgen/36 option 2 (its default) or option 4, back-filled records are never released, whichever option is chosen here.

Options:
1. Recommended: write a back-filled record with tool versions taken from the pins committed at that commit, marked `tools_source: inferred-from-pins`, and let the reader's environment policy decide whether it releases.
2. Write back-filled records for audit only: tool versions unknown (`tools_source: none`), so the reader always reports them as needing a re-run.
3. Do not back-fill; re-run every stuck brief instead.

Default if no answer: option 2 after 2026-11-03.

Option 2 writes the records and releases nothing by back-fill, so no release is inferred while the question is open.

Option 1 is expected to release few or none of the stuck briefs. `plugins/assay/paired-versions.yaml` is an implicit input of every row (statusgen/36), recorded by its blob id at the witnessed commit, so any later change to that file refuses the back-fill with `witness-input-changed`, even when the row's own inputs are unchanged. The file changed 69 times between 2026-08-01 and the authoring base, nearly daily, so almost every stuck brief's witnessed commit predates a pin change. Choosing option 1 keeps a release path for the rare back-fill whose pin file is still current; it does not clear the backlog, which needs re-runs either way.

## Read first

Read statusgen/36 and statusgen/37 in full, `witnessseam.go` (planned), the Evidence landing path in `tools/desk/cmd/deskevidence/deskevidence.go`, the change-request ref mapping in `tools/desk/cmd/deskdispatch/dispatch.go`, and the Evidence-actor check in `statusgen/evidenceactor.go`. Read the recorded rulings on statusgen/36 and this brief. Re-run `statusgen --root . --auto-flip-model --dry-run` on fresh main; the stuck set will have drifted from the 30 in Context as Evidence lands, which is expected. If statusgen/37 has not merged, report NEEDS_CONTEXT.

## Ground rules

- Feature branch and draft PR only. Never push to main, merge, release or dispatch a workflow. Merge main in, never rebase.
- Stop at `implemented`. The verifier identity runs `--apply` and lands the Evidence change; the implementer does not.
- Fixture repositories and fake forges only in tests.
- Never resolve a commit against a ref anyone can still push to, and never fetch without `--fetch`.
- Append only. Never rewrite or delete a v1 cell, an Evidence row or a verify-outcome record.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not guess.

## Interface contract

`deskevidence backfill [--brief <stream>/<NN>]... [--fetch] [--apply]`. Dry run by default: it prints one line per Verify row with `backfilled`, or `stays-v1: <reason>`, and changes nothing in the working tree, the index or the refs. For each v1 witness, in order (the first failing step decides the reason):

1. **Shape.** `stays-v1: dirty` for `+dirty` or `+unknown`; `stays-v1: undeclared` when the brief has no parseable `inputs:`; `stays-v1: env` for an `+env` row.
2. **Authorship.** From `statusgen --coverage --json`: the claim's `evidence_actor` must be `verifier` and `independent` must be true; otherwise `stays-v1: not-verifier-authored`. The record's `runner` is the seam's `runner` value; deskevidence never parses the Runner cell.
3. **Acceptance.** The claim's v1 gap must not be a command-, expect- or class-changed gap, and `statusgen witnessdigest --json --rev <commit> --row <id>` must give the same acceptance digests as at the item revision; otherwise `stays-v1: acceptance-changed`.
4. **Source change request.** `landing_cr` from the seam names the change request that landed the Evidence line. The forge must report it merged (closed without merging is refused); otherwise `stays-v1: source-unmerged` or, with no recognised landing form, `stays-v1: source-unknown`.
5. **Commit.** The full commit comes only from that merged change request's commit list as the forge returns it (full shas). It must be the only entry starting with the cell's prefix, and the prefix must be unambiguous among all local objects (`git rev-parse --disambiguate`); otherwise `stays-v1: ambiguous`. A prefix in no entry is `stays-v1: not-in-source`. A ref is never used to resolve the prefix.
6. **Fetch.** If the full commit is not present locally: without `--fetch`, `stays-v1: needs-fetch`. With `--fetch`, it first prints each full sha and source change request it will fetch, fetches by full sha into `refs/assay-backfill/<id>/`, and deletes that namespace before exit, including on error. Fetch failure is `stays-v1: unfetchable`.
7. **Record.** Digests come from `statusgen witnessdigest --json --rev <commit>`. The record carries every statusgen/36 field: `origin: backfill`, `source_cell` (the v1 cell text), `source_cr`, `runner` and `runner_source` from the seam, `result`, `exit`, `output_sha256` and `date` from the v1 witness row, the acceptance digests, `informational.commit` (the full commit from step 5), `runner_class: host`, `network: available`, `dirty_paths: []`, and `cr: pending`. When the digests hold an `absent` entry (a declared input missing at the commit), it writes nothing and reports `stays-v1: input-absent`, because statusgen/36 refuses a recorded `absent`. Tools follow the Human decision: option 1 fills them from `plugins/assay/paired-versions.yaml` at the commit with `tools_source: inferred-from-pins`; option 2 writes `tools_source: none`.
8. **Apply.** With `--apply`, it appends a v2 Evidence row beside the v1 row, citing the new record and carrying the v1 witness row's Command cell unchanged (this brief's step 3 has shown that cell equals the Verify row's Command, whose digest the record holds, so statusgen/36's step-3 Command binding holds), and updates the README rollout section. The verifier identity then binds `cr` to the back-fill's own Evidence change request with statusgen/37's forge-verified `--bind-cr`, so the reader verifies it like any record.

Release is decided only by statusgen/36's reader. A back-filled record can release only when the recorded rulings are statusgen/36 option 1 or 3 AND this brief's option 1, and every reader step holds; under any other combination it is reported as `witness-rerun-owed`.

## Task

### Fixed-delivery completion amendment — 2026-10-27

This is a planning amendment under [fixed-delivery
verification](../../proposals/fixed-delivery-verification/README.md), not runtime implementation or
activation. Preserve the original Verify table and Evidence. The changed fixed-subject,
historical-subject and row-reuse policy waits for the precise existing authority decisions; an old
decision token grants none of these changes. Existing approved work retains its original contract.
At pickup, pin this brief/version, the controlling contracts, actual producer/consumer versions and
open PRs; adapt or hold only the affected slice. No new evaluator, record store, desk, tracer or
qualification stage is authorized.

**Owner: bounded historical compatibility and witness backfill.** Consume /37's independently
produced historical binding and /36's D/A eligibility; no new historical resolver, approval path or
global grandfather exception. Code fixtures may use known metadata while /31 prepares actual legacy
metadata dispositions; missing declarations still hold the affected record, not the whole feature.

- Classify each original-table cohort before credit: correctly bound applicable observations; named
  unpinned consumer rows; bare unscoped consumer rows; wrong explicit ranges/changed criteria;
  unknown W/conditions; and completed records. Named/unscoped compatibility requires the exact
  approved historical-subject policy. Unchanged command text is not proof that adding D preserves
  effective subject. - Preserve all old observations, output hashes, execution revisions and
  outcomes. Recover v2 content records only where original provenance/content really exists and the
  environment policy permits credit. With unknown equivalence or lost artifacts, retain the
  observation and arrange fresh independent proof at D through the existing Verify owner; never
  manufacture a historical run. - A meaningful consumer observation must use independently bound D
  and its exact parent. Original empty-diff passes get no retroactive consumer credit. Changed
  commands remain on the exact protected route; already verified-but-not-done entries continue
  against admitted D/A, while completed records retain existing cutoff rules. - Extend the existing
  backfill fixtures with desktools-v2/23 and apps-installer/10 shapes (original PRs 4559/4561,
  tracked on 4560), the approved-amendment boundary in desk-tools/38 (4562/4563), and
  desk-supervision/13 with two disproved routes. No case is automatically cleared from reported
  counts. Hand off actual active cohort execution/reporting to /35, retained failures to handoff/02
  and repair rechecks to handoff/05.

### Existing task obligations

1. Implement `deskevidence backfill` as above, using only `witnessseam.go` (planned) for statusgen reads and digests.
2. Add a `ChangeCommits(repo, id)` read (full shas) and the merged state to the forge-neutral layer for GitHub and GitLab, with fake-forge tests on both.
3. Add fixtures: a squash-orphaned witness present only in its merged source change request; a locally present witness; a `+dirty` witness; a brief without `inputs:`; a changed Command, a loosened Expect and a changed Class; an `+env` row; a source change request closed without merging; a v1 row authored by a non-verifier identity; a declared input missing at the witnessed commit; a decoy commit planted locally that shares the cell's short prefix; a prefix in no entry of the source commit list; and an unfetchable commit.
4. Add a rollout section to the README listing, after a dry run on main, how many of the stuck briefs back-fill and how many await a re-witness, by reason.
5. Add changelog/statusgen-witness-v2-backfill.md. Replace each `follow-up statusgen/38` entry in `consumers:` with `fixed-here` in the same change.

## Verify

Named tests and fixtures are planned deliverables. A missing selector is a failure.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg38-r1.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestBackfillOrphanViaCR$' ./cmd/deskevidence/ > "$out" 2>&1 && grep -F -- '--- PASS: TestBackfillOrphanViaCR ' "$out")` | exit 0; named PASS. On both fake forges, a squash-orphaned witness is resolved through its merged source change request's commit list, fetched with `--fetch` into `refs/assay-backfill/`, and back-filled; the namespace is gone afterwards. The test runs statusgen's dry run on the fixture repository once per ruling pair: with statusgen/36 option 1 or 3 and this brief's option 1 the row is released; with statusgen/36 option 2 or 4, or with this brief's option 2, it is reported as `witness-rerun-owed`. |
| 2 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg38-r2.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestBackfillStaysV1$' ./cmd/deskevidence/ > "$out" 2>&1 && grep -F -- '--- PASS: TestBackfillStaysV1 ' "$out")` | exit 0; named PASS. Each fixture prints `stays-v1` with its own reason and writes nothing: `dirty`, `undeclared`, `env`, `not-verifier-authored`, `acceptance-changed` (Command, Expect and Class each), `source-unmerged`, `ambiguous` (the planted local decoy sharing the prefix), `not-in-source`, `needs-fetch` (without `--fetch`), `unfetchable` and `input-absent`. No case resolves a prefix through a ref. |
| 3 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg38-r3.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestBackfillAppendOnly$' ./cmd/deskevidence/ > "$out" 2>&1 && grep -F -- '--- PASS: TestBackfillAppendOnly ' "$out")` | exit 0; named PASS. After `--apply`, the brief's previous Evidence section is a byte prefix of the new one and every existing verify-outcome record is byte-unchanged. |
| 4 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg38-r4.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestBackfillChangedInput$' ./cmd/deskevidence/ > "$out" 2>&1 && grep -F -- '--- PASS: TestBackfillChangedInput ' "$out")` | exit 0; named PASS. A back-fill whose declared input changed after the witnessed commit is written, and statusgen's dry run refuses it under every ruling pair, never releasing it. Under statusgen/36 option 1 or 3 the gap is `witness-input-changed`, as it is for one whose only later change is to `plugins/assay/paired-versions.yaml`; one whose declared directory lost a file after the witnessed commit refuses with `witness-input-set-changed`. Under statusgen/36 option 2 or 4 each of these reports `witness-rerun-owed`, because statusgen/36's step 6 precedes its input comparison and the first failing step names the gap. An unchanged one is reported as `witness-rerun-owed` unless the rulings are statusgen/36 option 1 or 3 and this brief's option 1. |
| 5 | check +dereference | `d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg38-dry.XXXXXX") && git status --porcelain > "$d/before" && git for-each-ref > "$d/refs-before" && (cd tools/desk && GOWORK=off go build -o "$d/de" ./cmd/deskevidence) && "$d/de" backfill > "$d/out" 2>&1; rc=$?; git status --porcelain > "$d/after"; git for-each-ref > "$d/refs-after"; grep -c -e '^backfilled' -e '^stays-v1: ' "$d/out"; test "$rc" -eq 0 && cmp -s "$d/before" "$d/after" && cmp -s "$d/refs-before" "$d/refs-after"` | exit 0; prints a positive count. The dry run on main, without `--fetch`, leaves `git status --porcelain` and every ref unchanged and classifies every row of the stuck briefs. The counts match the README rollout section. |
| 6 | check +dereference | `statusgen --root . --consumers --brief statusgen/38 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge only; exit 0. The implementation diff corroborates the routing in `consumers:`. At implementation replace this provisional command with the actual delivery and first-parent pins and exact summary counts, per `docs/consumer-delivery-checks.md`. |

## Pre-mortem and detection

- A back-fill hashes the wrong commit, for example a planted commit sharing the short prefix. Caught by row 2's decoy and not-in-source cases; the prefix is resolved only against a merged change request's frozen commit list.
- A back-fill resolves through a change request that never merged, or one chosen by whoever runs the tool. Caught by row 2's source-unmerged case; the source is derived from the landing of the Evidence line.
- A back-fill launders an implementer-authored v1 pass. Caught by row 2's not-verifier-authored case.
- A back-fill releases after an Expect was loosened. Caught by row 2's acceptance-changed cases.
- A `+dirty` witness is back-filled from its parent commit, asserting content it never read. Caught by row 2.
- A back-fill rewrites history. Caught by row 3.
- A back-fill releases a witness whose inputs have since changed. Caught by row 4; the reader re-checks at the item revision.
- A back-fill releases under a ruling that forbids it. Caught by rows 1 and 4, which run every ruling pair.
- The dry run fetches or leaves refs behind. Caught by row 5's porcelain and ref comparison, and by row 1's namespace check.
- The rollout section claims counts that do not match the tool. Caught by row 5.
- A back-filled row was environment-dependent but not marked `+env`. Not caught here; it is governed by statusgen/36's environment policy, and under statusgen/36 option 2 or 4, or this brief's option 2, no back-fill releases.

## Evidence

<!-- Independent verifier records the subject revision, environment, command, exit, named result and date. This authoring change executes no implementation check. -->

## Review

Gate: human. The reviewer answers which single control stands between a wrong commit and a releasable back-fill, and whether row 2's decoy case proves the lower layer (local ambiguity) refuses with the upper one (the merged commit list) bypassed. Reject any of these:
- a back-fill of a `+dirty` witness;
- a back-filled record whose digests did not come from `statusgen witnessdigest`;
- an existing record or Evidence row changed;
- a back-fill that releases unless the recorded rulings are statusgen/36 option 1 or 3 and this brief's option 1;
- a commit resolved through a ref, an unmerged change request, or a short prefix that is ambiguous among local objects;
- a back-fill of a v1 row not authored by a verifier identity;
- a fetch without `--fetch`, or one that leaves refs behind;
- inferred tool versions not marked `tools_source: inferred-from-pins`.
