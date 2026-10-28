---
brief: assay:assay:statusgen:37
title: "Content-bound witness schema v2: the writers and the desk readers"
why: "Once the reader exists, the two writers must produce v2 witnesses and verify-outcome records, and every tools/desk reader must judge them through statusgen's seam instead of commit reachability. This ends new squash-orphaned records (medici-finance/assay#3658) without changing a single v1 record."
wave: 2
depends: ["statusgen/36"]
unblocks: ["statusgen/38"]
effort: L
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: [2026, 3105, 3658]
regression-of: "medici-finance/assay#2304"
schema: brief-v2
outcome: none
version: 1
authored: "2026-10-20 by the-desk (design brief for the witness-staleness class)"
gate-why: "It changes what evidence the verifier writes and how the desk loops read it. A writer that records the wrong scope or runs a row with network access while claiming none would feed stale passes to the reader."
decision-trigger: start
sources:
  - "medici-finance/assay#3658 — squash-orphaned verify-outcome record shas; content binding chosen over required merge commits"
  - "medici-finance/assay#2026 — the witness-staleness class"
  - "docs/streams/statusgen/brief-36-content-bound-witness-schema-v2-reader.md — the schema, record layout, seams and judgment this brief writes to"
  - "docs/streams/decisions/DR-desk-tools-45.md — additive, versioned fields; a missing or unknown version is a gap"
  - "tools/desk/internal/deskkit/forge.go — the forge-neutral `Forge` interface (GitHub and GitLab adapters)"
  - "statusgen/verifyincontainer.go — the digest-pinned harness container runner"
  - "freshness-checked 2026-10-20 @ 75cc11f1423301ea8ba0b111d48c8d5b8a303537 (origin/main)"
exec-tier: strong
exec-tier-why: "Two writers in two Go modules, a hermetic runner mode, a forge read on two adapters and a version-skew proof across releases must agree with one reader."
domain: complicated
consumers:
  - "statusgen/verifyrun.go and statusgen/verifyincontainer.go (v2 writer, hermetic container mode, `--cr` and `--bind-cr`): follow-up statusgen/37"
  - "tools/desk/cmd/deskevidence/ (outcomerecord.go, outcome.go, outcomejoin.go, landed.go, origin.go, plainread.go, verifiedgate.go, decisiongate.go, principalstamp.go, receiptvalidate.go, deskevidence.go): follow-up statusgen/37"
  - "tools/desk/internal/deskkit/ (verifyoutcomes.go, verifyoutcomejoin.go, verifywake.go, repairobligation.go, principal.go, forge.go and its GitHub and GitLab adapters): follow-up statusgen/37"
  - "tools/desk/cmd/verifyloop/briefscan.go and tools/desk/cmd/fanoutloop/repair.go: follow-up statusgen/37"
  - "tools/desk/cmd/deskdispatch/dispatch.go (mentions the `+dirty` witness mark; disposition per the census in statusgen/36): follow-up statusgen/37"
  - "plugins/assay/skills/verify-desk/SKILL.md and docs/evidence-bundle.md (how a witness is recorded): follow-up statusgen/37"
---

# Brief 37 — Content-bound witness schema v2: the writers and the desk readers

## Context

files:
- `statusgen/verifyrun.go`
- `statusgen/verifyrun_test.go`
- `statusgen/verifyincontainer.go`
- `statusgen/main.go`
- `tools/desk/cmd/deskevidence/outcomerecord.go`
- `tools/desk/cmd/deskevidence/outcome.go`
- `tools/desk/cmd/deskevidence/outcomejoin.go`
- `tools/desk/cmd/deskevidence/landed.go`
- `tools/desk/cmd/deskevidence/receiptvalidate.go`
- `tools/desk/cmd/deskevidence/deskevidence.go`
- `tools/desk/internal/deskkit/verifyoutcomes.go`
- `tools/desk/internal/deskkit/verifyoutcomejoin.go`
- `tools/desk/internal/deskkit/verifywake.go`
- `tools/desk/internal/deskkit/repairobligation.go`
- `tools/desk/internal/deskkit/forge.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- `tools/desk/cmd/verifyloop/briefscan.go`
- `tools/desk/cmd/fanoutloop/repair.go`
- tools/desk/internal/deskkit/witnessseam.go (planned)
- tools/desk/internal/deskkit/testdata/witnessv2/ (planned)
- statusgen/witnesswriter-mutations.json (planned)
- `plugins/assay/skills/verify-desk/SKILL.md`
- `docs/evidence-bundle.md`
- docs/witness-readers.md (planned, statusgen/36)
- changelog/statusgen-witness-v2-writers.md (planned)

facts (source and date in frontmatter):
- verifyrun writes the witness row as `| id | command | result | sha256:<output> | date | <runner> @ <commit-short>[+dirty|+unknown] (on-behalf-of) (source) |` from `treeSHA`; the identity before `@` is what the attribution reads use. The container runner (`--in-container`) is digest-pinned but passes a credential env-file and does not set `--network none`, so today no run is provably network-free.
- deskevidence writes verify-outcome records under `docs/streams/verify-outcomes/<stream>/` with `wake_schema: verify-wake-v1`, a top-level `sha`, an `inputs` map (SHA-256 of the brief file, the tool tag) and `prs: [{merge_sha, number, repo}]`. deskkit's readers join records by `sha`; #3658 counts 95 of 379 such shas unreachable from main.
- `deskkit.Forge.GetPullRequest` returns head, state and mergeability for GitHub and GitLab, but no landed (merge or squash) commit.
- tools/desk is a separate Go module and cannot import statusgen's `package main`. statusgen/36 adds the seams: `statusgen witnessdigest --json` and the versioned `witness` field on `statusgen --coverage --json`.
- Desk tools and statusgen ship together in one release, so a version skew means an older release reading a newer record.

layering: statusgen's verifyrun writes v2 witness cells and records with digests from the `statusgen/evaluation` library. deskevidence writes v2 outcome records with digests obtained from `statusgen witnessdigest --json`. Every tools/desk reader judges through `statusgen --coverage --json` and keeps its own code to transport only. The forge-neutral `Forge` interface verifies a change-request binding before it is written (open, targets the default branch, contains the record and the brief) and resolves its landed commit, which is recorded as informational.

single-point-of-failure: the runner's network isolation decides whether a run is recorded `network: none`; a runner that claims isolation it does not have would mark an environment-dependent row as releasable. Behind it: (1) row 13 proves at runtime, with real docker, that a connection attempted from inside the hermetic container fails while the same probe succeeds without `--hermetic`, so the composed arguments are tested by their effect and not only by their text; (2) the row's own `+env` mark and statusgen/36's environment rule refuse such rows independently of the runner's claim. A forged `network: none` written by hand is out of scope by statusgen/36's trust assumption (staleness, not forgery).

design-fit:
  owner: statusgen verifyrun (witness writer) and tools/desk deskevidence (outcome-record writer), both through the statusgen/evaluation digest owner
  contract: "R-witness-schema (statusgen/36): the cell keeps the executing identity, and the record binds identity, outcome, acceptance digests, dirty paths and a forge-verified `cr` inside its id; verify-wake-v2 records: `schema: 2`, `cr`, the witness record ids, `informational.commit` in place of a top-level `sha`"
  retires: ["joining verify-outcome records by commit reachability", "the top-level `sha` as a join key for new records", "per-reader parsing of Runner cells in tools/desk"]
  weight: "flags +3 (`verifyrun --cr`, `verifyrun --bind-cr`, `verifyrun --hermetic`); one Forge read field (landed commit); one record schema value; no new verb, module or service"
  why-add: "The reader in statusgen/36 can only release what a writer records; without these writers every new record is still squash-fragile."
  reuse-check: "Reuses the digest-pinned harness runner, the Forge interface's existing GitHub and GitLab adapters, and statusgen/36's seams. Does not add a desk-side digest implementation."

## Human decision

New verification runs will record which change request delivered them and a digest of what each row read. A run is marked network-free only when it ran in the pinned container with networking switched off and no credentials passed in. The question is the default runner for the verifier.

Options:
1. Recommended: the verifier runs rows in the network-less container by default and on the host only when asked. Host runs are still recorded, and the release policy decided for the reader governs them.
2. The verifier keeps the host as its default and uses the container only when asked.
3. Hold the writers until the observed-read check exists.

Default if no answer: option 2 after 2026-11-03.

Under option 2 the writers record v2 and no runner default changes. Under statusgen/36's default policy, host runs then never release by digest, so the verifier must ask for the container to produce a releasable record.

## Read first

Read statusgen/36 in full, then `runVerifyrun`, `treeSHA` and the cell writer in `statusgen/verifyrun.go`, the identity reads in `statusgen/attribution.go` and `statusgen/verifiedrunneragree.go` that the cell must keep serving, `composeDockerArgs` in `statusgen/verifyincontainer.go`, the record writer in `tools/desk/cmd/deskevidence/outcomerecord.go`, the join in `tools/desk/internal/deskkit/verifyoutcomejoin.go`, and the `Forge` interface and `PullRequest` type in `tools/desk/internal/deskkit/forge.go`. If statusgen/36 has not merged, or its seams differ from its Interface contract, report NEEDS_CONTEXT.

## Ground rules

- Feature branch and draft PR only. Never push to main, merge, release or dispatch a workflow. Merge main in, never rebase.
- Stop at `implemented`. A different, non-implementing identity owns verification and Evidence.
- Fixture repositories and fake forges only; no network in tests.
- Never rewrite a verify-outcome record, witness record or Evidence row that is on the default branch. The only rewrite allowed is `--bind-cr` replacing a `pending` record and its cell on an unmerged Evidence branch. v1 records are read exactly as today.
- No tools/desk package computes a digest or parses a Runner cell; it calls the statusgen seams.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not guess.

## Interface contract

- **verifyrun.** Writes only v2 from this release on, and only after a self-check that the same binary's reader accepts the record it wrote (round trip). The cell is `<runner> @ [w2] cr:<id> rec:<record-id>` followed by the on-behalf-of and runner-source parentheticals, exactly as statusgen/36 specifies. `<runner>` is the same executing identity verifyrun writes in a v1 cell today, so the attribution, independence, verifier-floor and runner-agreement reads are unchanged. Per row it writes the full statusgen/36 record, with every field inside the id:
  - `runner` and `runner_source`, equal to the cell;
  - `result`, `exit`, `output_sha256` and `date`, equal to the witness row's own cells;
  - `command_sha256`, `expect_sha256` and `class_sha256` of the Verify row as the run read it, the Command text normalised as statusgen/36 specifies. The witness row's Command cell is written from the same Command text the run executed and digested, so statusgen/36's step 3 (`witness-command-cell-mismatch`) holds for every record verifyrun writes, and the round-trip self-check refuses one where it does not;
  - `informational.commit`, the full object id of the commit the run read (40 or 64 hex characters, never abbreviated; statusgen/36 reports any other form as an empty revision);
  - the input digests, computed by the library at the working tree it ran on (`git hash-object --stdin-paths`, so a dirty tree is hashed as read). A required path missing from that tree writes no passing witness: statusgen/36 refuses a recorded `absent`, so verifyrun exits could-not-run with `witness-input-absent: <path>` instead. Likewise a required path that is a symlink writes no passing witness (`witness-input-symlink: <path>`), because statusgen/36 refuses it: its blob id is the link text, which does not change when the target does. Under statusgen/36 Human decision option 4, every record is written at whole-tree scope with its `tree_digest`;
  - `dirty_paths`: every non-bookkeeping path modified, deleted or untracked when the row ran. verifyrun prints each one that lies outside the row's required set, because statusgen/36 refuses that record (`witness-dirty-outside-inputs`); the record is still written, so the refusal is visible rather than silent.
- **Inputs changed during the run.** verifyrun digests the required set immediately before and immediately after each row. If any digest differs, it writes no passing witness for that row and exits could-not-run with `witness-inputs-changed-during-run: <first path>`.
- **Change-request binding.** `--cr <id>` and `--bind-cr <id> --brief <path>` set `cr` only to a value verified through the deskkit `Forge` adapter (GitHub and GitLab): the change request is open, targets the default branch, its head contains the commit that added the record being bound, and its diff changes both that record and the citing brief. The identity verifyrun would write now must equal the record's `runner`, so only the identity that ran the row binds it. `--bind-cr` is refused on the default branch and for any record already reachable from the default branch. Because `cr` is inside the record id, binding writes a new record file and a new id, rewrites the matching `cr:pending rec:<old-id>` cells in that brief, removes the pending record, and changes nothing else. Without `--cr` the cell says `cr:pending`. When the forge cannot be read, or any check fails, the flag refuses with a named reason and writes nothing; it never writes an unverified value. statusgen/36's reader re-verifies the binding offline from the landing commit, so this is the first of two independent checks.
- **Hermetic container mode.** `--in-container --hermetic` adds `--network none`, passes no credential env-file, and binds the mounted repository's git config files read-only over the read-write work mount. Before starting, it reads that config with includes followed and refuses with `hermetic-config-credentials: <key>` (the key only, never its value) when it holds a `credential.*` key, an `http.extraheader` or URL-scoped `http.<url>.extraheader`, or a remote or `url.*` value with embedded userinfo. It refuses with `hermetic-config-include: <key>` when the config holds any `include.path` or `includeIf.*.path` key, because an included file is neither bound read-only nor covered by the bind. It records `runner_class: harness-container`, `network: none` and the image digest. Every other run records `network: available`. The no-credentials claim covers exactly these two sources, the credential env-file and the repository's git config; a secret in a working-tree file is outside it.
- **Outcome records.** deskevidence writes `wake_schema: verify-wake-v2` with `schema: 2`, `cr: {repo, id}` set by the same forge-verified rule as `--bind-cr` (else `pending`), the ids of the witness records it covers, the v2 digest fields from `statusgen witnessdigest --json`, and `informational: {commit, landed_commit}`. There is no top-level `sha`, so a v1-only reader cannot join a v2 record by commit and refuses it.
- **Change-request resolution (#3658).** `PullRequest` gains `LandedCommit` (GitHub merge-commit sha, GitLab `merge_commit_sha` or `squash_commit_sha`), filled by both adapters. It is informational and never a join key.
- **Desk readers.** verifyloop, fanoutloop, deskevidence and deskkit judge records and witnesses through `statusgen --coverage --json` and match records by `(cr, record-id)`. A missing or unknown `witness_schema_version` or record schema is a named could-not-check, never a pass.

## Task

1. Implement the v2 writer (identity in cell and record, outcome fields, acceptance digests, `dirty_paths`, before-and-after input digests), the forge-verified `--cr` and `--bind-cr`, and `--hermetic` in statusgen, using only the library from statusgen/36 and the deskkit `Forge` adapters through a statusgen-side interface that tests replace with a fake forge.
2. Add `LandedCommit` to `PullRequest` and fill it in the GitHub and GitLab adapters.
3. Switch deskevidence's record writer to verify-wake-v2 and add `witnessseam.go` in deskkit as the only caller of the statusgen seams.
4. Move the readers named in `consumers:` to the seam, keeping v1 behaviour unchanged. Update `docs/witness-readers.md` (planned) dispositions.
5. Update `plugins/assay/skills/verify-desk/SKILL.md` and `docs/evidence-bundle.md` to describe v2 recording and `--bind-cr`.
6. Write `statusgen/witnesswriter-mutations.json`: a failing control, and mutants that drop `--network none`, keep the env-file, drop the read-only git config bind, skip the git config credential check, skip the git config include refusal, write a passing witness for a symlink input, hash the committed tree instead of the working tree, skip the round-trip self-check, write a top-level `sha`, write the runner class in place of the identity in the cell, omit `runner` from the record, omit the outcome fields, digest a Command text other than the one written in the witness row's Command cell, omit `dirty_paths`, skip the after-run digest, bind `cr` without the forge check, and bind under an identity other than the record's `runner`. Each must be caught by a named test.
7. Add changelog/statusgen-witness-v2-writers.md. Replace each `follow-up statusgen/37` entry in `consumers:` with `fixed-here` in the same change.

## Verify

Named tests, fixtures and the mutation manifest are planned deliverables. A missing selector is a failure.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r1.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunWritesV2$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunWritesV2 ' "$out")` | exit 0; named PASS. The cell carries the executing identity before `@`, then `[w2]` and the full 64-hex record id; the record holds every statusgen/36 field, its `runner`, `result`, `exit`, `output_sha256` and `date` equal the cell and the witness row, its acceptance digests equal the Verify row, its `command_sha256` equals the digest of the witness row's own normalised Command cell, and its `informational.commit` is the full object id of the commit read; the same binary's reader accepts it. A deliberately unreadable record makes verifyrun exit could-not-run and write nothing, and so does a declared input missing from the working tree, with `witness-input-absent` naming it, and a declared input that is a symlink, with `witness-input-symlink` naming it. |
| 2 | check:ci +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r2.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunDirtyDigest$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunDirtyDigest ' "$out")` | exit 0; named PASS. On a dirty tree the recorded digest is of the edited file as read and `dirty_paths` lists it; committing that edit and squash-merging it releases, and committing a different edit refuses. A dirty edit outside the required set is listed in `dirty_paths`, printed by verifyrun, and refused by the reader with `witness-dirty-outside-inputs`. |
| 3 | check:ci +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r3.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunHermetic$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunHermetic ' "$out")` | exit 0; named PASS. The composed docker arguments for `--hermetic` contain `--network none`, no `--env-file`, and a read-only bind for each git config file of the mounted repository; a run without `--hermetic` records `network: available`. Fixture repositories whose config holds a credential helper, an `http.extraheader`, a URL-scoped `http.<url>.extraheader`, or a remote URL with embedded userinfo each make `--hermetic` refuse with `hermetic-config-credentials` naming the key, and the output contains no fixture secret value; one whose config holds an `include.path` refuses with `hermetic-config-include`. |
| 4 | check:ci +flow | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r4.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunBindCR$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunBindCR ' "$out")` | exit 0; named PASS. Against a fake forge, `--bind-cr` writes a new record whose only changed field is `cr`, gives it a new id, rewrites only the matching `cr:pending` cells and removes the pending record; the digests are unchanged. Each is refused with a named reason and writes nothing: a change request that is closed or merged, targets another branch, whose head lacks the commit that added the record, or whose diff does not change both the record and the brief; a forge that cannot be read; an identity that differs from the record's `runner`; a run on the default branch or for a record reachable from it; and a cell already bound to another id. |
| 5 | check:ci +flow +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r5.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestWitnessSquashFlow$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestWitnessSquashFlow ' "$out")` | exit 0; named PASS. End to end in a fixture repository: write a witness on a branch, bind the change request, squash-merge, delete the branch and prune; `--auto-flip-model --dry-run` releases the brief. Repeating with a one-byte change to a declared input on main refuses. |
| 6 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r6.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestOutcomeRecordV2$' ./cmd/deskevidence/ > "$out" 2>&1 && grep -F -- '--- PASS: TestOutcomeRecordV2 ' "$out")` | exit 0; named PASS. A new record carries `verify-wake-v2`, `schema: 2`, `cr`, digests matching `statusgen witnessdigest --json` for the same tree, and no top-level `sha`. |
| 7 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r7.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestLandedCommitBothForges$' ./internal/deskkit/ > "$out" 2>&1 && grep -F -- '--- PASS: TestLandedCommitBothForges ' "$out")` | exit 0; named PASS. Against fake GitHub and GitLab servers, a merge-committed and a squash-merged change both resolve `LandedCommit`; an open change resolves none and is not an error. |
| 8 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r8.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestDeskReadsV2ViaSeam$' ./internal/deskkit/ ./cmd/verifyloop/ ./cmd/fanoutloop/ ./cmd/deskevidence/ > "$out" 2>&1 && grep -c -- '--- PASS: TestDeskReadsV2ViaSeam ' "$out" \| grep -qx 4)` | exit 0; four named PASS lines, one per package (deskkit, verifyloop, fanoutloop, deskevidence). Each reader matches a v2 record by `(cr, record-id)` through a stub seam; a missing or unknown `witness_schema_version` is a named could-not-check, and a v1 record is read as today. |
| 9 | check:ci +mutation | `(cd tools/desk && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r9.XXXXXX") && GOWORK=off go test -count=1 -v -run '^(TestDeskWitnessSeamOnly\|TestDeskSeamFlagsPlant)$' ./internal/arch/ > "$out" 2>&1 && grep -F -- '--- PASS: TestDeskWitnessSeamOnly ' "$out" && grep -F -- '--- PASS: TestDeskSeamFlagsPlant ' "$out")` | exit 0; both PASS. Class guard (fail-first): no tools/desk package other than `deskkit/witnessseam.go` (planned) invokes `statusgen witnessdigest` or `--coverage --json`, and none parses a Runner cell or computes a digest. The plant fixture is flagged. The Evidence records the pre-fix red run with a planted site. |
| 10 | check +mutation +flow | `d=; trap 'test -n "$d" && git worktree remove --force "$d/old" 2>/dev/null; test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg37-skew.XXXXXX") && git worktree add --detach "$d/old" v1.0.52 && (cd "$d/old/tools/desk" && GOWORK=off go build -o "$d/" ./cmd/verifyloop ./cmd/deskevidence) && (cd tools/desk && DESK_OLD_BIN_DIR="$d" GOWORK=off go test -count=1 -v -run '^TestDeskSkewOldReader$' ./internal/deskkit/ > "$d/out" 2>&1) && grep -F -- '--- PASS: TestDeskSkewOldReader ' "$d/out"` | exit 0; named PASS. The release before this brief (v1.0.52 at authoring; re-pin at implementation) reads a fixture v2 record and refuses or reports could-not-check; it never counts it as a pass. The test fails, never skips, when `DESK_OLD_BIN_DIR` is unset. |
| 11 | check +mutation | `d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg37-muhar.XXXXXX") && (cd tools/desk && GOWORK=off go build -o "$d/muhar" ./cmd/muhar) && (cd statusgen && "$d/muhar" -spec witnesswriter-mutations.json -j 4 > "$d/out" 2>&1); rc=$?; tail -1 "$d/out"; test "$rc" -eq 0 && grep -qE '^Totals: [1-9][0-9]* caught, 0 NOT CAUGHT, 0 could-not-mutate' "$d/out"` | exit 0. The failing control and every mutant are caught. |
| 12 | check:ci | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r12.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestWitnessReaderCensus$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestWitnessReaderCensus ' "$out")` | exit 0; named PASS. Every census entry's disposition matches the code after the migration, with no entry left at "migrates in statusgen/37". |
| 13 | check +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r13.XXXXXX") && docker info > /dev/null 2>&1 && SG_REQUIRE_DOCKER=1 GOWORK=off go test -count=1 -v -run '^TestVerifyrunHermeticEgress$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunHermeticEgress ' "$out")` | exit 0; named PASS. Real docker, the pinned harness image. The test opens a TCP listener on the host and mounts a static probe binary it builds into the container. Under `--in-container --hermetic` the probe's connections to `host.docker.internal` and to the bridge gateway on the listener's port both fail, and the listener records no connection. Control: the same probe without `--hermetic` connects, so the probe itself works. Inside the hermetic container no variable named in the credential env-file is set, and a write to the mounted repository's git config fails. Without docker the row exits non-zero, and with `SG_REQUIRE_DOCKER=1` the test fails rather than skips. |
| 14 | check:ci +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg37-r14.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunInputsChangedDuringRun$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunInputsChangedDuringRun ' "$out")` | exit 0; named PASS. A fixture row whose command edits one of its own declared inputs makes verifyrun write no passing witness and exit could-not-run with `witness-inputs-changed-during-run` naming that path; the same row without the edit writes a passing witness. |
| 15 | check +dereference | `statusgen --root . --consumers --brief statusgen/37 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge only; exit 0. The implementation diff corroborates the routing in `consumers:`. At implementation replace this provisional command with the actual delivery and first-parent pins and exact summary counts, per `docs/consumer-delivery-checks.md`. |

## Pre-mortem and detection

- verifyrun writes v2 before a reader that understands it ships. Prevented by `depends: statusgen/36` and caught by row 1's round-trip self-check.
- The writer hashes the committed tree while the row read edited files. Caught by row 2.
- A run claims `network: none` but had network or credentials. Caught at runtime by row 13 (a real connection attempt from inside the container fails, with a control that connects, and the git config is not writable) and at argument level by row 3, which also refuses a git config holding credentials. A secret in a working-tree file is outside the claim and not caught; statusgen/36's environment rule refuses `+env` rows regardless.
- The writer drops the executing identity from the cell or the record, so an implementer's run counts as independent. Caught by row 1 and its mutants, and by statusgen/36 row 16.
- A row edits its own inputs and the after-state is recorded as what was read. Caught by row 14.
- A dirty edit outside the declared inputs is invisible. Caught by row 2 and statusgen/36's `witness-dirty-outside-inputs`.
- `--bind-cr` writes an unverified change request, binds under another identity, alters digests, runs after landing, or rebinds an already bound witness. Caught by row 4; statusgen/36's offline landing-commit check refuses an unverified binding independently.
- The squash flow still fails end to end because one piece consults the commit. Caught by row 5.
- A new record still keys on `sha`, so #3658 recurs. Caught by row 6.
- Change-request resolution works on one forge only. Caught by row 7.
- A desk reader keeps its own interpretation, or treats an unknown version as a pass. Caught by rows 8 (all four reader packages) and 9.
- An older desk release counts a v2 record as a pass. Caught by row 10.
- A guard can never fire. Caught by row 11.

## Evidence

<!-- Independent verifier records the subject revision, environment, command, exit, named result and date. This authoring change executes no implementation check. -->

## Review

Gate: human. The reviewer answers which single control stands between a run with network access and a `network: none` record, and whether row 13 (runtime) or statusgen/36's environment rule proves the lower layer refuses with the upper one bypassed. Reject any of these:
- a tools/desk digest or Runner-cell parser;
- a v2 record with a top-level `sha` used as a join key;
- a writer that can emit a schema its own reader rejects;
- a v1 record or Evidence row rewritten, or any record rewritten once it is on the default branch;
- a cell or record without the executing identity;
- a `cr` written without forge verification, or bound by an identity other than the record's `runner`;
- a hermetic claim proven only by argument inspection.
- a hermetic run whose git config is writable inside the container, holds credentials, or includes another file;
- a passing witness written for a required path missing from the working tree.
