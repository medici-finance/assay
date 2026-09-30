---
brief: assay:assay:forge-neutral:21
title: Claim store seam — one interface in deskkit, resolved from cell configuration, refusing rather than falling back
why: >-
  The dispatch claim can only live on the forge today, so every role that dispatches must hold
  repository write — including the reviewer, which needs it for nothing else. Lifting the
  claim store behind one deskkit interface with a resolver is what lets a cell keep its claims
  somewhere that needs no forge write, and it keeps the choice out of skill prose. This brief
  lands the seam with only today's forge store behind it, reachable solely as what an unset
  key means, so behaviour is unchanged for every adopter.
wave: 2
depends: ["forge-neutral/20"]
unblocks: ["forge-neutral/22", "forge-neutral/23", "forge-neutral/25"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
design: DR-forge-neutral-21
decision-issue: 1552
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them"
  - "tools/desk/cmd/deskclaim-ref/claim.go:97-119 — the existing `claimStore` interface this brief lifts unchanged"
  - "tools/desk/cmd/deskdispatch/dispatch.go:228-241 and 872-918 — step 1 and `resolveClaimAuth`, which become store-aware"
  - "tools/desk/internal/deskkit/forgeresolve.go:9-40 — the resolution contract the new resolver mirrors: no caller-supplied choice, refusal is the only fallback"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "questions (b) and (c): the claim is the fleet's mutual exclusion, and a resolver that falls back instead of refusing double-dispatches without failing any happy-path test"
gate-why: >-
  This brief moves the code that decides WHERE a dispatch claim is kept and therefore under
  which credential, if any, it is written — claim custody on the identity chain. The human
  confirms the resolver takes no flag, refuses on every unmet precondition instead of moving
  to another store, cannot be told to select the forge store, and leaves existing installs
  exactly as they are apart from one notice.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/deskclaim-ref: fixed-here (the forge store implements deskkit.ClaimStore; the store comes from the resolver)"
  - "tools/desk/cmd/deskdispatch/dispatch.go: fixed-here (step 1 resolves the store pre-claim; the credential is minted only when the store needs one)"
  - "tools/desk/internal/deskkit/rosterconfig.go (the new keys): fixed-here (the three keys, recognised and strictly parsed)"
  - "tools/desk/README.md: fixed-here (the resolver, the keys, the two valid values, the removal NOTICE, the legacy script's scope)"
  - "claim readers outside the claim tool: follow-up forge-neutral/22"
  - "removal of the forge store and of the unset-key resolution: follow-up forge-neutral/32"
  - "tools/dispatch-claim.sh in consumer repositories: out-of-scope (the legacy script speaks the forge store only, is reached solely when the Go claim tool is absent, and leaves with that store)"
version: 1
id: f354fb2d-860e-474d-8a44-fbebeb942739
---

# Brief 21 — Claim store seam and resolver

## Context
files:
- `tools/desk/internal/deskkit/claimstore.go` (planned), `claimstore_test.go` (planned),
  `claimstore_conformance_test.go` (planned) — the interface, the resolver, the shared
  conformance table.
- `tools/desk/internal/deskkit/rosterconfig.go` — `ASSAY_CLAIM_STORE`, `ASSAY_CLAIM_DIR`,
  `ASSAY_CLAIM_SINGLE_HOST`, strictly parsed.
- `tools/desk/cmd/deskclaim-ref/claim.go`, `gogit.go`, `main.go` — the forge store becomes one
  implementation of the deskkit interface.
- `tools/desk/cmd/deskdispatch/dispatch.go` — step 1 asks the resolver; the role credential is
  minted only when the resolved store needs one.
- `tools/desk/internal/deskkit/mutations.json` — one entry.
- `tools/desk/README.md`; `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the resolver's order — behind it, each store's own atomic create (a
wrong resolution still cannot yield two holders inside one store) and the mixed-store refusal
of brief 23 (two stores live for one repo during the window is detected from the forge side, a
different signal in a different component).

facts:
- The seam is the spec's §4; the keys, the order and the refusal are its §5.
- Valid values are `file` and `service`. **`forge-ref` is not a valid value**: set explicitly
  it is refused like any unknown value, printing the two valid ones. The forge store is
  reachable only as the legacy resolution of an **unset** key, for one release window.
- This brief wires the legacy resolution only. `file` and `service` parse as valid and resolve
  to a refusal naming the release that ships them ("the release that ships the file store" /
  "… the served store" — not a brief id, which the shipped-corpus guard keeps out of
  `tools/desk`), so a key set early fails loudly.
- Unset key → the forge store, plus a NOTICE on every boot of a dispatching role that names
  `ASSAY_CLAIM_STORE`, the two valid values, and the release in which an unset key stops
  resolving. The release name is a constant set when release N is cut, not prose.
- The conformance table (spec V1) is written here against the forge store's in-memory double,
  so briefs 23 and 24 add a backend, not a test design.
- `show` / `list` output is a wire contract (`deskclaim-ref/claim.go:34-38`); it must stay
  byte-identical.
- Exit codes: 5 refused, 6 unverifiable (`tools/desk/internal/deskkit/exitcodes.go`).

## Human decision
Dispatch claims are today always stored on the code-hosting platform, which forces every role
that dispatches work to hold write access to the repository. The proposal introduces one place
in the tools that decides where a cell keeps its claims, read from the cell's configuration.
Two kinds of storage will be valid: a folder on the computer, and a small claim service. The
platform storage used today is being retired over one release; in this step it is kept only as
what happens when nothing is configured, with a notice at every start naming the setting to add
and the release in which the old behaviour ends. It cannot be selected on purpose. Nothing else
changes for current installs. The rules being approved: no command-line switch selects the
storage, and if the configured storage cannot be used the tools stop rather than quietly use a
different one.

Options:
1. **Approve as specified.**
2. **Approve, but refuse immediately when nothing is configured** — no notice period; every
   existing install stops at its next upgrade until someone edits its configuration.
3. **Reject** — claims stay on the platform only, and the reviewer keeps write access.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- No new forge CLI shell-out and no passthrough method; the forge-surface control must stay green.

## Task
1. Lift the claim store interface into `deskkit` as `ClaimStore`, semantics unchanged.
2. Add the three roster keys with strict parsing; an unknown store value — `forge-ref`
   included — is a refusal printing the two valid values.
3. Add `ResolveClaimStore(repo)` per the spec's §5: returns the store, its name, and a
   provenance string naming each input and its source. A test asserts no exported symbol and
   no flag accepts a store choice.
4. `deskdispatch` step 1 and the claim tool obtain their store from the resolver. A refusal
   happens before any worktree is cut and before any credential is minted.
5. Step report line: `claim-acquire OK: … store forge-ref (legacy), authenticated by …`.
6. The removal NOTICE, with the release name as one constant.
7. Conformance table + mutation entry (make an unmet precondition fall through to the legacy
   resolution instead of refusing).
8. README: the resolver, the keys, the two valid values, the removal NOTICE, the legacy
   script's scope.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v` | exit 0; subtests: unset → legacy + NOTICE; `forge-ref` explicit → refused printing `file` and `service`; unknown value → refused; `file`/`service` → refused naming the release that ships them (spec V8, release-N half) |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStoreNeverFallsBack' -count=1 -timeout 120s` | exit 0 — an explicit store whose precondition fails is exit 6, and the resolved name is never another store |
| 4 | check:ci +mutation | the `mutations.json` entry named `claimstore-unmet-precondition-falls-through` | row 3 goes RED |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s` | exit 0 — the shared table passes for the forge store (spec V1) |
| 6 | check:ci +flow | `cd tools/desk && go test ./cmd/deskdispatch/ -run 'TestDispatchRefusesBeforeWorktreeOnStoreRefusal' -count=1 -timeout 120s` | exit 0 — zero child processes, no worktree, no token minted |
| 7 | check:ci +neighbour | `cd tools/desk && go test ./cmd/deskclaim-ref/ -count=1 -timeout 300s` | exit 0 — the existing suite passes unchanged (wire format and `show`/`list` parity) |
| 8 | check:ci | `cd tools/desk && go test ./internal/forgeban/ -count=1 -timeout 300s` | exit 0 |
| 9 | check +dereference | `cd tools/desk && go build -o /tmp/deskclaim-ref-fn21 ./cmd/deskclaim-ref && /tmp/deskclaim-ref-fn21 --help` and compare with the keys, the two valid values and the removal NOTICE text quoted in `tools/desk/README.md` | every key name, both values and the NOTICE line in the README appear verbatim in the tool's real output |
| 10 | check | `(cd statusgen && go build -o /tmp/statusgen-fn21 .) && /tmp/statusgen-fn21 --root . --consumers --brief forge-neutral/21` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| An unusable configured store quietly becomes the forge store | rows 3, 4 |
| The forge store can be selected on purpose and so outlives its window | row 2 |
| A refusal arrives after the worktree is cut, leaving debris | row 6 |
| Lifting the interface changes output the readers parse | row 7 |
| A `--claim-store` flag creeps in "for debugging" | row 2's no-selector subtest; review |
| An existing install changes behaviour at upgrade | row 2's unset-key subtest |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 89042b8fcc7e (claude-opus-5-5[1m]) (on-behalf-of human:ian) (forge-identity))

Execution witness — statusgen verifyrun, appended verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStoreNeverFallsBack' -count=1 -timeout 120s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 4 | `mutations.json` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskdispatch/ -run 'TestDispatchRefusesBeforeWorktreeOnStoreRefusal' -count=1 -timeout 120s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskclaim-ref/ -count=1 -timeout 300s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/forgeban/ -count=1 -timeout 300s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go build -o /tmp/deskclaim-ref-fn21 ./cmd/deskclaim-ref && /tmp/deskclaim-ref-fn21 --help` | pass exit=0 | sha256:76d6f6357bd3 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd statusgen && go build -o /tmp/statusgen-fn21 .) && /tmp/statusgen-fn21 --root . --consumers --brief forge-neutral/21` | fail exit=2 | sha256:44b9349a51e0 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |

#### Direct runs and risk-value review — 2026-09-25 (assay-verifier-app[bot] @ 89042b8fcc7e (claude-opus-5-5[1m]) (on-behalf-of human:ian) (forge-identity))

The execution witness above could not execute the check:ci rows 2–8 on this darwin host (no
network-off sandbox). The same commands were executed directly on the host at 89042b8fcc7e,
NOT hermetically (network available, Go module proxy disabled with GOPROXY=off, GOWORK=off).
These rows are supplementary evidence; the hermetic re-execution of rows 2–8 on a Linux runner
is still owed and is what the witness rows record.

| # | Command | Expected | Observed (exit + key line) | Date / runner |
|---|---------|----------|----------------------------|---------------|
| 2 | go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v (in tools/desk) | exit 0; unset → legacy + NOTICE; forge-ref explicit refused; unknown refused; file/service refused naming the shipping release | exit 0 — `--- PASS: TestResolveClaimStore`, 13/13 subtests PASS incl. unset_key_resolves_to_the_legacy_forge-ref_store_with_the_removal_NOTICE, forge-ref_set_explicitly_is_refused_printing_file_and_service, an_unknown_value_is_refused_printing_file_and_service, file_and_service_are_valid_but_refused_naming_the_release_that_ships_them, no_exported_symbol_and_no_flag_accepts_a_store_choice | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 3 | go test ./internal/deskkit/ -run 'TestResolveClaimStoreNeverFallsBack' -count=1 -timeout 120s | exit 0 | exit 0 — ok .../tools/desk/internal/deskkit; 4/4 subtests PASS (file/service not shipped; file/service shipped but precondition fails) | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 4 | mutation claimstore-unmet-precondition-falls-through (mutations.json old→new applied by hand, 1 match), then row 3 | row 3 goes RED | exit 1 — `--- FAIL: TestResolveClaimStoreNeverFallsBack`, all 4 subtests FAIL, FAIL .../tools/desk/internal/deskkit; source restored byte-for-byte afterwards and row 3 re-ran exit 0 (`ok`) | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 5 | go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s | exit 0 | exit 0 — `ok`; forge-ref (in-memory double): 8/8 conformance cases PASS | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 6 | go test ./cmd/deskdispatch/ -run 'TestDispatchRefusesBeforeWorktreeOnStoreRefusal' -count=1 -timeout 120s | exit 0 | exit 0 — ok .../tools/desk/cmd/deskdispatch; 5/5 subtests PASS | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 7 | go test ./cmd/deskclaim-ref/ -count=1 -timeout 300s | exit 0 | exit 0 — ok .../tools/desk/cmd/deskclaim-ref | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 8 | go test ./internal/forgeban/ -count=1 -timeout 300s | exit 0 | exit 0 — ok .../tools/desk/internal/forgeban | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]), host, non-hermetic |
| 9 | deskclaim-ref --help compared with the tools/desk README claim-store section | every key, both values and the NOTICE line verbatim | exit 0 — ASSAY_CLAIM_STORE, ASSAY_CLAIM_DIR, ASSAY_CLAIM_SINGLE_HOST, `file`, `service` all present; the README NOTICE line matches the help output verbatim (fixed-string match count 1) | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]) |
| 10 | statusgen --root . --consumers --brief assay:assay:forge-neutral:21 --base 17e884ab0b0c at the squash merge 5aa382100ef9 (#1562) | exit 0 | exit 0 — `summary: 4 corroborated, 0 disproved, 3 unchecked`; the 3 UNCHECKED are the follow-up / out-of-scope entries (forge-neutral/22, forge-neutral/32, the legacy dispatch-claim script). The witness row 10 exit 2 is statusgen's COULD-NOT-CHECK on merged main ("not in the diff against 89042b8f…"), a limitation of running the consumers gate after merge, not a disproved claim | 2026-09-25 assay-verifier-app[bot] (claude-opus-5-5[1m]) |

**Side finding (brief prose only).** The brief's Context facts say "Exit codes: 5 refused, 6
unverifiable". The implementation makes every resolver refusal exit 6, which is what the spec
states ("Every refusal is exit 6", reviewer-write-boundary §5), what DR-forge-neutral-21 records,
and what Verify row 3 expects. The code follows the spec; the brief's facts line is stale.

**Risk-bearing values** — enumerated over the diff 17e884ab..5aa382100 (non-test Go sources and
the tools/desk README), ranked by irreversibility. The item is `irreversible: no`,
`sensitive-data: yes` (claim custody).

- RISK-VALUE: DERIVED — ClaimStoreFile = "file", ClaimStoreService = "service" (forge-ref excluded from the valid set) @ tools/desk/internal/deskkit/claimstore.go:115-117 — the spec §5 key table names exactly two valid values and makes an explicit forge-ref a refusal, as ruled in DR-forge-neutral-21 (option 1).
- RISK-VALUE: DERIVED — NeedsForgeCredential = true (legacy forge-ref resolution) @ tools/desk/internal/deskkit/claimstore.go:268 — forge-ref claims are refs written on the forge, so that store needs repository write (spec access table: reviewer under the legacy resolution = write); it is the only resolution that mints a credential for the claim.
- RISK-VALUE: DERIVED — ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:31 (existing constant, bound to every resolver refusal at claimstore.go:245-257) — the spec states "Every refusal is exit 6".
- RISK-VALUE: DERIVED — ASSAY_CLAIM_SINGLE_HOST accepted value "yes" @ tools/desk/internal/deskkit/rosterconfig.go:1806 — the spec makes it an operator declaration whose only value is `yes`; strict parsing refuses any other value.
- RISK-VALUE: NAMED, NOT DERIVED — ClaimStoreLegacyRemovalRelease = "N+1" @ tools/desk/internal/deskkit/claimstore.go:126 — this is a placeholder, not a release. The concrete value can only be set when release N is cut, and nothing outside the README, rosterconfig.go:1745 and a test references the constant, so no release guard enforces its replacement.
- Ranked last, no derivation required (reversible; the file store is not shipped yet): default claims directory "dispatch-claims" @ tools/desk/internal/deskkit/claimstore.go:358, matching the spec's stated default.

**Open question for the human (sign-off).** RISK-VALUE: NAMED, NOT DERIVED — ClaimStoreLegacyRemovalRelease = "N+1" @ tools/desk/internal/deskkit/claimstore.go:126: what sets this constant at the release-N cut, and should the release tooling refuse a cut while it still reads "N+1"? Without that, the removal NOTICE could ship naming a non-release.

VERIFY: PASS (local tier, evidence only). Rows 1 and 9 pass under the witness; rows 2–8 pass on
direct host execution with the mutation going red as designed; row 10 corroborates at the merge
diff. The witness's check:ci rows 2–8 still need hermetic re-execution. Gate human: the status
stays `implemented`; sign-off is the human's.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; claim custody and credential selection). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a wrong resolution and two holders of one claim? (The resolver's order; beneath it each store's atomic create and the mixed-store refusal.)
2. Does any row prove the lower layer with the upper bypassed? (Row 6 removes every usable store and proves nothing durable happens; row 7 proves the ref-level behaviour is untouched.)
