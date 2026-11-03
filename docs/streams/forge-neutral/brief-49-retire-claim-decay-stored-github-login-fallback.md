---
brief: assay:assay:forge-neutral:49
title: Retire claim-decay stored GitHub login fallback
why: "Claim decay can still read an unrelated stored CLI login and start a credential subprocess when no explicit token exists. Retire that fallback now as already required by the approved shared-library transition, leaving missing evidence visibly unavailable."
wave: 0
depends: []
unblocks: ["forge-neutral/18"]
split-from: forge-neutral/18
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
id: fe04a55d-04af-4fb1-b046-f0158330f4de
authored: 2026-11-03 by statusgen newbrief
design: DR-forge-library
sources:
  - docs/streams/decisions/DR-forge-library.md
  - docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-03 against App-fetched main 678d7ea5feccd434daddc65741aec34f7d258679"
exec-tier: strong
exec-tier-why: "A credential-source retirement must retain host separation and distinguish unavailable input from a read complete enough to decay claims."
domain: complicated
consumers:
  - "statusgen/claimdecay_github.go: fixed-here"
  - "statusgen/claimdecay_github_test.go: fixed-here"
  - "statusgen/storedlogin_test.go: fixed-here (claim-decay retirement control only; other guard/ruling tests unchanged)"
  - "statusgen/forgecredential.go: fixed-here (comment-only retirement inventory; no guard logic edits)"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: fixed-here (parent consumes this contribution; its entire original Verify table remains)"
  - "docs/streams/forge-neutral/README.md: fixed-here (bounded contribution and source-count inventory)"
  - "changelog/claim-decay-no-stored-login.md: fixed-here"
  - "tools/desk/scripts/forge-ban.sh: out-of-scope (existing scan unchanged; observe its count, never raise a ceiling or weaken a control)"
  - "statusgen/decisionruling.go and authenticated reader/library migration: out-of-scope (forge-neutral/35 and desktools-v2/77 retain ownership)"
---

# Brief 49 — Retire claim-decay stored GitHub login fallback

## Context
files:
- `statusgen/claimdecay_github.go`
- `statusgen/claimdecay_github_test.go`
- `statusgen/storedlogin_test.go` (TestStoredLoginClaimDecay only)
- `statusgen/forgecredential.go` (comment-only retirement inventory)
- `docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md`
- `docs/streams/forge-neutral/README.md`
- `changelog/claim-decay-no-stored-login.md` (planned)

Read the existing tools/desk/scripts/forge-ban.sh without changing it.

facts:
- At source 678d7ea5, githubDecayToken still calls exec.LookPath("gh") and exec.Command("gh", "auth", "token", "--hostname", host). These are one lookup and one possible subprocess, represented by two counted gh lines.
- Parent /18 Task 6's 25 October amendment already requires this exact retirement. The parent is genuinely partial/in-progress; all its other reads, CI kind composition and complete cutover remain owed.
- The existing host test distinguishes public and derived enterprise credentials, but its final subtest still accepts a stored CLI login. Preserve the host checks while replacing that retired behavior with refusal.
- Missing credentials already return an error before githubBranchPRPages sends any request. That must remain could-not-check, never a complete-empty branch population.
- This contribution adds no credential reader, role, custody, endpoint, service, private-data population or activation. Existing explicit token inputs remain unchanged until /77; only the CLI fallback retires.

## Read first and architecture pickup

Read /18's full current Task/Verify and its DR-forge-library amendment, the current credential and request path, docs/library-first.md and docs/go-engineering.md. Follow implementation-pickup and the security-review skill before effects and review. Record source and guideline revisions, inspected caller flow and continue/adapt/hold in the PR. No new library or transport is needed to delete this path.

## Ground rules
- Own isolated worktree and draft PR. Export KUBECONFIG=/dev/null; fake transports and synthetic tokens only. No live infrastructure, credential discovery, main push, merge, release or workflow dispatch.
- Preserve the complete parent /18 Verify table, original obligations and historical Evidence. Do not claim /18 implemented, its other reads migrated or the shared library/cutover active.
- Do not edit /35's ruling/sign-off controls, CI identities or read-kind permissions. Keep explicit public/enterprise token selection and operator endpoint semantics.
- Do not weaken any security control, test or forge-ban ceiling. Existing host checks stay; the retired positive fallback case becomes a no-launch/no-request refusal control.
- Stop at implemented; independent verification and the parent's remaining delivery retain their own owners.

## Task
1. Remove githubDecayToken's gh executable lookup and stored-login subprocess fallback. Remove now-unneeded import/helper code only when it has no other caller. Keep explicit token names, precedence, public-versus-derived-enterprise host separation, endpoint behavior and typed unavailable results unchanged.
2. Add TestDecayNoStoredLogin through the actual credential-to-branch-read flow with a sentinel gh executable that would return a usable synthetic token if invoked. Without an explicit correct token, assert zero sentinel invocations, zero provider requests and an unavailable/error result; prior claim data cannot become a fresh complete-empty observation. Cover public, derived enterprise and explicit operator endpoint cases, with stored-login state both allowed and refused by the existing verification context.
3. Retain TestDecayTokenStaysOnItsHost and its explicit-token positive and cross-host refusal cases. Replace only its retired gh-fallback positive subtest with the stronger refusal. Prove both supported explicit-token variables' precedence and that enterprise credentials never reach github.com or the converse.
4. Plant and restore the actual removed production fallback, and separately a missing-credential-to-complete-empty mutation. Each must fail the appropriate named control. Retain final passing runs and exact fail-first output; do not count compilation failures as caught mutations.
5. Record the two quoted-gh-line retirement under the parent census and the one-subprocess reduction under the unchanged forge-ban instrument and update the bounded stream inventory plus fragment. Keep parent /18 acceptance byte-identical and its lifecycle partial; no second owner or full-cutover claim.

## Interface contract

No exported API, reader operation, credential authority or wire format changes. The existing private selector returns only an explicitly supplied, correctly scoped token or its existing missing-credential outcome. Its caller remains responsible for returning unavailable before any request. The shared forge-library move remains /75 and /77; /35 still owns the other ruling credential fallback.

## Verify

All fixtures are offline and synthetic. Named PASS output is mandatory; a missing/skipped selector is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayNoStoredLogin$' . > "$o" && grep -F -- '--- PASS: TestDecayNoStoredLogin ' "$o")` | Public, derived enterprise and explicit operator endpoints with no appropriate explicit token fail unavailable before zero requests and zero sentinel gh invocations. Existing claims are not replaced by a fresh complete-empty read. Restoring the actual CLI fallback or treating missing credentials as complete-empty fails the production flow control. |
| 2 | check:ci +flow +mutation | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayTokenStaysOnItsHost$' . > "$o" && grep -F -- '--- PASS: TestDecayTokenStaysOnItsHost ' "$o")` | Existing explicit-token reads succeed with correct precedence; wrong-host credentials fail before requests, and the old fallback case now refuses without invoking gh. Removing host separation fails a real source mutation; no former host refusal is weakened. |
| 3 | check +dereference | `sh tools/desk/scripts/forge-ban.sh` | Exit 0 under unchanged scan/ceilings. Record the source-pinned before/after counts: forge-ban counts one fewer subprocess (class a 31 to 30 and statusgen total 39 to 38 at the pickup source); the parent quoted-gh-line census separately loses the lookup and subprocess lines, two in all. Remaining statusgen sites remain explicitly incomplete and are not a passing /18 cutover claim. |
| 4 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/49 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Pre-merge on the implementation branch: exit 0 corroborates actual edits; follow-ups prove routing only, exclusions remain unchecked. Pin the actual mainline delivery and first parent before post-merge verification; no producer-identity or complete-cutover inference. |

## Pre-mortem and detection

- CLI fallback survives under a different branch: row 1 sentinel observes the actual production flow with a valid synthetic login available.
- Missing credentials become a usable empty listing: row 1 requires unavailable and unchanged prior claim state; the missing-as-empty production mutation must fail.
- The retirement changes host selection or token precedence: row 2 preserves positive and foreign-host controls plus a host-separation mutation.
- Two counted lines get called full cutover or a weakened ceiling hides a new launch: row 3 records the unchanged scan and review compares the parent acceptance table byte for byte.

## Evidence
<!-- Independent verifier appends exact source, command, output and negative-control evidence. No acceptance is asserted by authoring or implementation. -->

## Review
Gate: model. Apply the security-review skill to the authentication lookup boundary and trace selected tokens to the existing request sink. Confirm the CLI path is absent, explicit tokens stay on their current host, missing data remains unavailable and every parent obligation remains. The original /18 delivery is still partial after this contribution.
