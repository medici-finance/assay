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
  - "tools/desk/cmd/deskclaim-ref: follow-up forge-neutral/21 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskdispatch/dispatch.go: follow-up forge-neutral/21 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/rosterconfig.go (the new keys): follow-up forge-neutral/21 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md: follow-up forge-neutral/21 (this brief; flips to fixed-here when the implementation edits the path)"
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
  to a refusal naming the brief that ships them, so a key set early fails loudly.
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
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v` | exit 0; subtests: unset → legacy + NOTICE; `forge-ref` explicit → refused printing `file` and `service`; unknown value → refused; `file`/`service` → refused naming the shipping brief (spec V8, release-N half) |
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

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; claim custody and credential selection). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a wrong resolution and two holders of one claim? (The resolver's order; beneath it each store's atomic create and the mixed-store refusal.)
2. Does any row prove the lower layer with the upper bypassed? (Row 6 removes every usable store and proves nothing durable happens; row 7 proves the ref-level behaviour is untouched.)
