---
brief: assay:assay:forge-neutral:23
title: File claim store — claims in a directory the cell shares, with the single-host declaration and the mixed-store refusal
why: >-
  A cell whose desks share a host needs nothing from the forge to arbitrate its own
  dispatches, yet today it must give every dispatching role repository write to do so. A
  directory store removes that need on every forge and tier. It is also exactly the design
  that once double-dispatched when two machines each had their own directory, so it ships only
  together with the guards that make that case a refusal instead of a silent race.
wave: 3
depends: ["forge-neutral/21"]
unblocks: ["forge-neutral/24", "forge-neutral/28", "forge-neutral/30"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "tools/desk/internal/deskkit/claim.go:147-190 and 217-323 — the shipped directory lock and exclusive-create primitive this store reuses"
  - "tools/desk/cmd/deskclaim/main.go:30-35 — why dispatch claims left the machine-local directory on 2026-08-13; the failure the guards exist for"
  - "tools/desk/cmd/deskroster/main.go:1-10 — the roster is machine-local, so the tool cannot observe another host"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (c): mutual exclusion under concurrency, where a subtle error survives every single-process test"
gate-why: >-
  This brief adds a place to keep dispatch claims that is invisible to any machine not
  sharing it — claim custody. The human confirms the store is refused without the operator's
  single-host declaration, that the boot output says the declaration is not verified, that
  live claims on the forge make it refuse, and that it is documented as unsupported on
  filesystems brief 20 did not measure clean.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/claimstore_file.go: follow-up forge-neutral/23 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/claimstore.go (resolver preconditions): follow-up forge-neutral/23 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md: follow-up forge-neutral/23 (this brief; flips to fixed-here when the implementation edits the path)"
  - "the boot check's store line and notices: follow-up forge-neutral/25"
  - "cell scaffolds that write the keys: follow-up forge-neutral/28"
version: 1
id: adab60fb-cc64-4c4f-95bf-11739ca9d449
---

# Brief 23 — File claim store and guards

## Context
files:
- `tools/desk/internal/deskkit/claimstore_file.go` (planned), `claimstore_file_test.go` (planned)
- `tools/desk/internal/deskkit/claimstore.go` (planned) — brief 21's resolver file, present once
  that brief has landed; this brief adds the preconditions for `file` and the mixed-store check.
- `tools/desk/internal/deskkit/mutations.json`; `tools/desk/README.md`;
  `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the exclusive create under the directory lock — behind it, the
supervisor's staleness reclaim (elapsed time plus no live branch; a different signal in a
different component) and branch-as-claim once the worker's branch is pushed, which is on the
forge under every store. For the cross-host case the single control is the operator's
declaration, and the layer behind it is the forge-side mixed-store read; a second host that
also runs a `file` store and holds no forge claims is NOT detectable (spec T12) — stated, not
hidden.

facts:
- Shape: the spec's §4.1. One file per claim key under `ASSAY_CLAIM_DIR` (default
  `<config home>/dispatch-claims`); same holder encoding, TTLs, verbs, exit codes and
  `show`/`list` lines as the forge store.
- Acquire = exclusive create; advance / steal = compare against the content hash last read,
  then rewrite in place; release = compare-and-delete; all under the directory lock
  (spec §10 E default). A lock that cannot be held or a file that cannot be read is exit 6.
- Preconditions to resolve `file` (spec §6): `ASSAY_CLAIM_SINGLE_HOST=yes`; the directory
  exists with owner-only permissions and a store marker (cell name, store kind) that agrees;
  a forge-side read of both claim namespaces shows no live claim for the repo. The forge read
  needs read access only; if it cannot be performed the result is could-not-check → refuse.
- The reverse check: resolving `forge-ref` while the default claims directory holds a live
  claim for the repo → refuse.
- Supported filesystems are exactly those brief 20 measured clean. The README lists them and
  says everything else is unsupported for this store; use `service`.

## Human decision
A cell whose desks all run on one computer can keep its dispatch claims in a folder on that
computer instead of on the code-hosting platform. Then no role needs write access to the
repository just to record a claim. The risk is known from experience: if a second computer
also dispatches the same repository with its own folder, neither sees the other and the same
work is started twice. The tools cannot see other computers. The proposal therefore refuses
the folder unless the operator has declared in configuration that this is the only computer,
repeats at every start that this is declared and not verified, and refuses if it finds claims
for the repository on the platform, which would mean another dispatcher is using different
storage. The decision is whether that protection is sufficient.

Options:
1. **Approve as specified.**
2. **Approve with a notice only, no required declaration** — easier setup; a copied
   configuration reproduces the known double-start silently.
3. **Reject the folder store** — cells use the served store or the platform only.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Do not weaken the directory-lock wait or the fail-closed returns in the shipped primitive to make a test faster.

## Task
1. Implement the `file` backend on the shipped lock and exclusive-create primitive; add it to
   the conformance table.
2. Implement the three preconditions and the reverse check in the resolver; each refusal is
   exit 6 before any worktree, naming the key or the evidence found and the remedy (drain,
   then switch — spec §7).
3. Race test: N processes, one key, exactly one acquires, the rest exit 5.
4. Mutation entries: (a) replace the exclusive create with a plain create; (b) skip the
   forge-side mixed-store read.
5. README: the store, the keys, the supported-filesystem list from brief 20, the drain.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s -v` | exit 0; the `file` backend passes every row `forge-ref` passes, including `show`/`list` byte parity (spec V1) |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestFileStoreRaceExactlyOneAcquires' -count=5 -timeout 300s` | exit 0 on all five runs (spec V2) |
| 4 | check:ci +mutation | the `mutations.json` entry named `filestore-plain-create` | row 3 goes RED |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestFileStoreRefusedWithoutSingleHostDeclaration' -count=1 -timeout 120s` | exit 0 — refused, exit 6, message names `ASSAY_CLAIM_SINGLE_HOST` (spec V3) |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestMixedStoresRefuse' -count=1 -timeout 120s -v` | exit 0; subtests: live forge claim while resolving `file` → refused; live file claim while resolving `forge-ref` → refused; forge read impossible → refused as could-not-check (spec V4) |
| 7 | check:ci +mutation | the `mutations.json` entry named `filestore-skips-mixed-store-read` | row 6's first subtest goes RED |
| 8 | check +flow +dereference | With a config home whose reviewer grant records repository **read**, `ASSAY_CLAIM_STORE=file` and the declaration set: run a review dispatch against a fixture repository, then `deskclaim-ref show fixture--issue-1` and `desksupervise status` | dispatch reports `store file`; both readers show the holder; no ref was written to the fixture (spec V5) |
| 9 | check | `(cd statusgen && go build -o /tmp/statusgen-fn23 .) && /tmp/statusgen-fn23 --root . --consumers --brief forge-neutral/23` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| Two dispatchers on one host both acquire | rows 3, 4 |
| A second host with forge-stored claims runs beside a file store | rows 6, 7 |
| A second host with its OWN file store runs beside this one | **no row** — not detectable (spec T12). Recorded review-only: the declaration, the every-boot notice (brief 25) and the adopter docs (brief 29) are the controls, and the spec says so in words |
| The store is used on a network filesystem where exclusive create is not atomic | review-only plus README: only filesystems brief 20 measured clean are listed as supported |
| Output differs from the forge store and breaks a parser | row 2 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; where claims are kept and who can see them). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between two hosts and a double dispatch? (The operator's declaration; beneath it the forge-side mixed-store read — and nothing for a second file store, which the spec states.) Is that acceptable?
2. Does any row prove the lower layer with the upper bypassed? (Row 4 removes the atomic create; row 7 removes the mixed-store read; row 8 runs with a credential that cannot write to the forge.)
