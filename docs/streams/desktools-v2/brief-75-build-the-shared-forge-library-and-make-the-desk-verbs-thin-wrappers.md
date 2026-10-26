---
brief: assay:assay:desktools-v2:75
title: "Build the shared forge library and make the desk verbs thin wrappers"
why: "Every desk verb reaches the forge through internal deskkit code that no other module can link, so statusgen runs a separate process and keeps its own clients. The driver's ruling asks for one shared library for reads and writes that owns the role-and-repo credential, with deskread and the write verbs as thin wrappers and the remaining gh/glab launches held to at most 7 by a down-only ratchet."
wave: 2
depends: ["desktools-v2/58"]
unblocks: ["desktools-v2/77"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-library
issues: [2395]
schema: brief-v2
outcome: none
version: 1
id: f6e03aed-326e-4cb0-aa40-3194727e140c
authored: 2026-10-25 by desktools-v2 authoring session
sources:
  - "docs/streams/decisions/DR-forge-library.md — the record of the driver's ruling (https://github.com/medici-finance/assay/issues/2395#issuecomment-6404972196)"
  - docs/library-first.md
  - docs/streams/desktools-v2/spec.md — Principle 2 amendment of 2026-10-25
  - docs/streams/desktools-v2/brief-58-extract-the-role-credential-provider-and-retire-internal-desktoken-callouts.md
  - docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - "freshness-checked 2026-10-25 @ d6a2ff244 (origin/main)"
exec-tier: strong
exec-tier-why: "The change moves the code that holds a role credential into a package other modules can link. A wrapper that still works when the library is bypassed, or a constructor that quietly accepts an ambient token, passes every happy-path test; only lower-layer negative controls and parity against the current verbs show the custody survived the move."
domain: complicated
gate-why: "sensitive-data is yes: the library holds minted App tokens for private repositories and becomes linkable by a second module. The human confirms the five carried custody records hold unchanged inside the library, the credential is unreadable by callers with the verb layer bypassed, the placement and module path, the starting value of the gh/glab ratchet and whether skill shell scripts count toward the 7."
decision-trigger: start
consumers:
  - "forgeclient/ (planned module; final path decided at this brief's gate): follow-up desktools-v2/75"
  - "tools/desk/internal/credential/ (desktools-v2/58's planned provider): follow-up desktools-v2/75 (lifted out of internal so the library can own it)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go, forgeresolve.go, roletoken.go: follow-up desktools-v2/75 (delegate to the library; deskkit keeps config, guard and audit adapters)"
  - "tools/desk/cmd/deskread, deskpost, deskpr, deskreply, deskfile, deskevidence, deskrun and the other forge verbs: follow-up desktools-v2/75 (become thin wrappers; inventory actual call sites at pickup)"
  - "tools/desk/scripts/forge-ban.sh, tools/desk/internal/forgeban/allowlist.go: follow-up desktools-v2/75 (the combined gh/glab ratchet)"
  - "tools/desk/go.mod, forgeread/: follow-up desktools-v2/75 (forgeread's root package stays dependency-free; its adapters are composed, not moved)"
  - "statusgen/: follow-up desktools-v2/77 (this brief adds no statusgen call; /77 swaps the reader)"
  - "docs/library-first.md, docs/contracts.md, tools/desk/README.md: follow-up desktools-v2/75"
---

# Brief 75 — Build the shared forge library and make the desk verbs thin wrappers

## Context

files:
- `forgeclient/` (planned module: `go.mod`, client constructor, typed operations, tests)
- `tools/desk/internal/credential/` (planned by desktools-v2/58; lifted here)
- `tools/desk/internal/deskkit/forge.go`, `forge_github.go`, `forge_gitlab.go`,
  `forgeresolve.go`, `roletoken.go`, `ambienttoken_guard_test.go`
- `tools/desk/cmd/deskread/` and the forge write verbs under `tools/desk/cmd/`
- `tools/desk/scripts/forge-ban.sh`, `tools/desk/internal/forgeban/allowlist.go`
- `tools/desk/go.mod`, `forgeread/go.mod`
- `.github/workflows/forge-surface-control.yml`
- `docs/library-first.md`, `docs/contracts.md`, `tools/desk/README.md`
- `changelog/shared-forge-library.md` (planned)

single-point-of-failure: the library client's constructor and its private credential field —
the one place a token meets a forge request. Behind it: the forge's own App installation scope,
which refuses an out-of-scope repository whatever the client does; the forge-ban and
ambient-token scans over the library's tree; and rows 3 and 4, which prove the refusal with the
verb layer bypassed.

facts (d6a2ff244, 2026-10-25):
- `tools/desk/internal/deskkit/` owns the `Forge` interface, both backends and `ForgeFor`;
  it is internal to the desk module, so statusgen cannot import it.
- `forgeread/` is a separate root module whose root package is dependency-free and whose
  `adapters/` take an admitted `Access` with no credential (forge-neutral/36, row 11).
- desktools-v2/58 plans the role credential provider at `tools/desk/internal/credential/`;
  that path is internal and must move for the library to own the credential.
- gh/glab launch sites, counted with `forge-ban.sh`'s launch classes: desk-tools Go 5, cellctl
  shell 1, skill shell scripts under `plugins/assay/` 17, statusgen Go 30, `glab` 0. desk-tools
  plus statusgen is 36; with the skill scripts it is 53.
- `allowedInvocationCeiling = 6` at `tools/desk/internal/forgeban/allowlist.go:94`. The one
  direct `exec.Command("gh", …)` in `tools/desk` non-test code is `cmd/deskapps/identity.go:24`.

design-fit:
  owner: shared forge library (desktools-v2)
  contract: S-identity (role and repository credential custody, unchanged; the library becomes its holder)
  retires: [deskkit-internal forge backends as the only implementation, per-verb credential plumbing, statusgen's deskread process bridge (via /77)]
  weight: "verbs 0, flags 0; one module; refusals unchanged"
  why-add: "The ruling asks for exactly one library for reads and writes; without it statusgen keeps a process bridge and its own clients."

## Read first

`docs/streams/decisions/DR-forge-library.md`, the five carried records it links,
`docs/library-first.md`, desktools-v2/58 and forge-neutral/36. Refresh every count in facts at
pickup; they are dated measurements.

## Human decision

The ruling sets the rules; this gate accepts the implementation of them and settles what the
ruling left open.

Options:
1. **Accept the demonstrated library** — accept the module path, the constructor and the
   carried custody as shown, the ratchet's starting value and the answer on skill scripts;
   grant no new identity, scope, service or deployment.
2. **Hold or narrow** — name the boundary that is not demonstrated (for example a carried rule
   that changed, or a credential reachable by a caller) and revise before acceptance.

Questions the implementer puts with the evidence: whether `forgeclient/` is the right path;
whether the 17 skill shell launches count toward the 7 or are a separate surface; which launch
sites remain and why each cannot use the library.

Default if no answer: none — blocks until answered.

## Ground rules

Feature branch and draft PR only. Fake providers, synthetic repositories and temporary homes;
no live forge contact, no credential provisioning, no release. Carry DR-desktools-v2-03,
DR-forge-neutral-34, -14, -17 and DR-forge-gitlab-11 unchanged: a test that pinned the old
behaviour keeps pinning it against the library. Do not export `deskkit` wholesale and add no
generic request, URL or token interface. Do not reopen the frozen `Forge` operation set. Stop
at implemented; an independent verifier owns Evidence.

## Interface contract

`New(role, repo, …)` (name final at implementation) builds one client for one role and one
repository from the credential provider; it refuses an empty or unminted token and never reads
an ambient token. The token is an unexported field with no accessor, no `String`/`Format`
exposure and no serializable form. Callers see enumerated typed operations and typed results:
no raw URL, request, GraphQL document or token. A client for repository A refuses an operation
on repository B. The CI workflow-token transport (DR-forge-neutral-34) is a separate,
explicit, read-only constructor. Verb CLIs keep their flags, outputs and exit classes.

## Task

1. Create the library module (`forgeclient/`, proposed) composing `forgeread`'s adapters and
   lifting desktools-v2/58's provider out of `internal/`. `forgeread`'s root package gains no
   dependency.
2. Move the GitHub and GitLab read and write implementations behind the client. `deskkit`
   keeps config, guard and audit adapters and delegates its `Forge` methods to the library;
   delete duplicates.
3. Turn `deskread` and every forge write verb into thin wrappers: parse flags, build a client,
   call one typed operation, print. Inventory each verb as migrated or deferred with a reason.
4. Carry the five custody records: each existing test that pins one keeps passing against the
   library, and each gets a lower-layer twin that bypasses the verb.
5. Add the combined gh/glab ratchet: one counter over desk-tools and statusgen non-test code,
   starting at the count measured at pickup, failing on any rise. Once statusgen reaches 0
   (desktools-v2/77 and forge-neutral/18), the ceiling asserts at most 7. Report the skill
   scripts separately until the gate decides whether they count.
6. Update the library-first table, contracts and the tool README; add the fragment.

## Verify

Named tests are planned deliverables. Each command needs its exact PASS line; a missing
selector is failure. Mutations are planted in the production owner, never a copy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeclient && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestClientVerbParity$' ./... > "$out" && grep -F -- '--- PASS: TestClientVerbParity ' "$out")` | exit 0; named PASS; for each migrated verb operation, the library result over a fake provider equals the pre-change verb output fixture for both forges; one planted mapping change fails |
| 2 | check:ci +flow +mutation | `(cd forgeclient && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestClientNoAmbientToken$' ./... > "$out" && grep -F -- '--- PASS: TestClientNoAmbientToken ' "$out")` | exit 0; named PASS; with sentinel values in every forge-token variable and `gh` on `PATH` returning a sentinel, a client with no minted token refuses and no sentinel reaches the fake provider; planting an environment fallback in the constructor fails the test |
| 3 | check:ci +flow +mutation | `(cd forgeclient && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestClientRepoScopeLowerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestClientRepoScopeLowerBoundary ' "$out")` | exit 0; named PASS; with no verb involved, a client built for repository A refuses reads and writes on B before any request reaches the fake provider, while A succeeds; removing the scope comparison fails the test |
| 4 | check:ci +mutation | `(cd forgeclient && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestClientCredentialUnreadable$' ./... > "$out" && grep -F -- '--- PASS: TestClientCredentialUnreadable ' "$out")` | exit 0; named PASS; the client's formatted, JSON and error forms never contain the token, and the package exports no accessor; adding an exported getter or a `%v`-visible field fails the test |
| 5 | check:ci +flow | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCarriedCustodyRecords$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCarriedCustodyRecords ' "$out")` | exit 0; named PASS; one subtest per carried record (DR-desktools-v2-03, DR-forge-neutral-34, -14, -17, DR-forge-gitlab-11) runs its existing refusal against the library directly and through the verb |
| 6 | check +mutation | `sh tools/desk/scripts/forge-ban.sh` | exit 0 on the clean tree, and the output names the combined gh/glab launch count with statusgen included. A planted `exec.Command("gh", …)` in a new statusgen or library file makes the script exit non-zero |
| 7 | check:ci | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeSurfaceUnchangedByDeskread$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestForgeSurfaceUnchangedByDeskread ' "$out")` | exit 0; named PASS; the frozen `Forge` operation set gains no method |
| 8 | check +dereference | `(cd forgeread && GOWORK=off go list -deps . \| { grep -x -e net/http -e os/exec \|\| [ $? -eq 1 ]; } \| wc -l)` | output `0`; the forgeread root package still links no transport or process launch after the library composes its adapters |
| 9 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/75 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge, on the implementation branch: exit 0; the declared routing corroborates the actual diff; out-of-scope judgments remain UNCHECKED and reviewer-owned; a could-not-check is never recorded as a pass |

## Pre-mortem and detection

- A verb keeps its own transport beside the library, so two implementations drift: row 1.
- The constructor accepts an ambient token when none is minted: row 2.
- The scope check lives only in the verb, so a library caller reaches another repository: row 3.
- The token leaks through a log line, an error or a debug dump: row 4.
- A carried rule is rewritten in passing: row 5.
- Launches move into the library instead of retiring, or statusgen gains one: row 6.
- The library widens the frozen operation set: row 7.
- Composing the adapters drags a transport into the forgeread root package: row 8.

Whether the remaining launch sites are the right ones, and whether skill scripts count, are
review judgments at the human gate.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human. Review answers both: (1) the single control between a leaked or misrouted token and
a private repository is the client's constructor binding — is the forge-side installation scope
behind it acceptable? (2) do rows 3 and 4 prove the lower layer refuses with the verb layer
bypassed? Approval grants no new identity, scope or deployment.
