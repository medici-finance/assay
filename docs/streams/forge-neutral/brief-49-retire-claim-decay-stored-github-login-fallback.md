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
version: 3
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
  - "docs/streams/desktools-v2/brief-08-hold-statusgen-at-zero.md: fixed-here (prose-only child ownership correction; Task, Verify and gates unchanged)"
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
- `docs/streams/desktools-v2/brief-08-hold-statusgen-at-zero.md` (prose-only ownership correction)
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh tools/desk/scripts/forge-ban.sh` | pass exit=0 | sha256:deebaebc1d5f | 2026-11-06 | assay-verifier-app[bot] @ 8592383261fb (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief forge-neutral/49 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:d1d8367d382d | 2026-11-06 | assay-verifier-app[bot] @ 8592383261fb (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5558 run=a3ecc13aaf0b06cda1bc7169eeba875467543ca429b9e54eac941e78cde5eebc source=8592383261fb8f59c261b785cae5f22169654852 brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayNoStoredLogin$' . > "$o" && grep -F -- '--- PASS: TestDecayNoStoredLogin ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b514dee5226d | 2026-11-06 | assay-verifier-app[bot] @ 8592383261fb (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayTokenStaysOnItsHost$' . > "$o" && grep -F -- '--- PASS: TestDecayTokenStaysOnItsHost ' "$o")` | pass exit=0 sandbox=container-netns | sha256:4d49385845dd | 2026-11-06 | assay-verifier-app[bot] @ 8592383261fb (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:27598ceee2eaa30e2851ab3f18afc14ee7e8128caf4757c1f91d8f2452d12392
Container-Image: sha256:732786a33ebfa6dc1d80d396781df1f31c5331f67a8c0bd97518785f5a72b173
Verification-Attestation: medici-finance/assay#5558 run=a3ecc13aaf0b06cda1bc7169eeba875467543ca429b9e54eac941e78cde5eebc source=8592383261fb8f59c261b785cae5f22169654852 brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=a3ecc13aaf0b06cda1bc7169eeba875467543ca429b9e54eac941e78cde5eebc source=8592383261fb8f59c261b785cae5f22169654852 tree=sha256:27598ceee2eaa30e2851ab3f18afc14ee7e8128caf4757c1f91d8f2452d12392 witness=sha256:7719c0854f246f10a2f62ac5c89013e7346e7786d6f1cefa48e0a44c1ed6eb60 opened=2026-11-06T14:05:53Z closed=2026-11-06T14:06:53Z before=receipt after=check-verifier

**VERIFY: BLOCKED** — row 4 could-not-check at merged main (check-definition); rows 1-3 pass.

Verifier claude-opus-5-5 (strong, non-implementer) at merged main 8592383261fb, statusgen v1.0.75, KUBECONFIG=/dev/null, GH_TOKEN unset. Delivery: PR 5391, merge 32f85d97060e, first parent 6bceac0fa40e. Rows 1 and 2 ran in the container admission bracket (verify-runner v1.0.75, network none, sandbox container-netns, bracket-digest verdict=match); rows 3 and 4 ran on the host.

Row 3 reads 74 sites (desk 36, statusgen 38), class a statusgen=30; the same scan at the first parent reads 75 sites, statusgen 39, class a statusgen=31, which is the Expect.

Row 4 exits 2 with COULD-NOT-CHECK: after the merge, the merge-base of origin/main and HEAD is HEAD itself (8592383261fb), so the diff is empty and the run carries no evidence about the brief's claims. This is not a disproof. The row's own Expect says the mainline delivery and its first parent must be pinned before post-merge verification, and the command does not pin them. The repair is a row definition that pins merge 32f85d97060e and first parent 6bceac0fa40e (a reviewed check-definition change, owner: brief owner). Supplemental observation, no acceptance credit: the same command at the delivery head 877a9a25c1d7 with base 6db77ad394e4 exits 0 with 8 corroborated, 0 disproved, 2 unchecked.

Fail-first: owed on no row. The brief's risk fields are all no and no row is marked risk-bearing, live, mutating or end-to-end. A fail-first run at base 6bceac0fa40e was refused by the tool because that base predates the brief (exit 2, no rows ran).

Supplemental production-mutation probes (restore the CLI fallback, treat missing as empty, drop host separation) were blocked by the session's write guard and abandoned. They carry no acceptance credit and no Verify row depended on them.

RISK-VALUE: DERIVED public and operator-endpoint variable order GH_TOKEN then GITHUB_TOKEN, statusgen/claimdecay_github.go:68, unchanged by the delivery.
RISK-VALUE: DERIVED derived-enterprise host takes only GH_ENTERPRISE_TOKEN then GITHUB_ENTERPRISE_TOKEN, statusgen/claimdecay_github.go:70, so a lookalike host never receives the public credential.
RISK-VALUE: NAMED reversible knobs, request timeout 30s at statusgen/claimdecay_github.go:57 and page ceiling 1000 at statusgen/claimdecay_github.go:52, unchanged by the delivery.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayNoStoredLogin$' . > "$o" && grep -F -- '--- PASS: TestDecayNoStoredLogin ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b514dee5226d | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayTokenStaysOnItsHost$' . > "$o" && grep -F -- '--- PASS: TestDecayTokenStaysOnItsHost ' "$o")` | pass exit=0 sandbox=container-netns | sha256:4d49385845dd | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6029 run=7316aada845c35754cec52072f9b148c71a2874969d6d1c2cd49ec51134ca6d0 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=7316aada845c35754cec52072f9b148c71a2874969d6d1c2cd49ec51134ca6d0 source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:449a0af09143973f1685c3df284360310e7b627da4fe50804cb3af81b8227e6f opened=2026-11-10T10:30:22Z closed=2026-11-10T10:32:01Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh tools/desk/scripts/forge-ban.sh` | pass exit=0 | sha256:1c1cfd806f3e | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief forge-neutral/49 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:bd96b2ab0e45 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6029 run=7316aada845c35754cec52072f9b148c71a2874969d6d1c2cd49ec51134ca6d0 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — row 4 could-not-check at merged main (check-definition, medici-finance/assay#5564 still open); rows 1-3 pass.

Re-verification by claude-opus-5-5 (strong, non-implementer) at merged main 612750a12406, statusgen v1.0.81, KUBECONFIG=/dev/null, GH_TOKEN unset. Delivery: PR 5391, merge 32f85d97060e, first parent 6bceac0fa40e. Rows 1 and 2 ran in the container admission bracket (verify-runner v1.0.81 from the release pin, network none, sandbox container-netns, fresh module cache, bracket-digest verdict=match). Rows 3 and 4 ran on the host.

Row 3 at 612750a12406 reads 68 sites (desk 37, statusgen 31), class a statusgen=22, exit 0. The scan script is unchanged since before the delivery. Its Expect pins counts to the pickup source, so the same script was also run on exported trees: the first parent reads 75 sites, statusgen 39, class a statusgen=31; the delivery merge reads 74, 38 and 30. That is the Expect's one fewer subprocess. The later drop at main comes from other work and is no /18 cutover claim.

Row 4 exits 2 with COULD-NOT-CHECK, unchanged from the earlier run. After the merge, the merge-base of origin/main and HEAD is HEAD itself, so the diff is empty and the run carries no evidence about the brief's claims. This is not a disproof. The row's own Expect says the delivery and its first parent must be pinned for post-merge verification. The command does not pin them, and the row definition is unchanged at this source. The repair is still the reviewed check-definition change tracked in medici-finance/assay#5564 (next actor: brief owner).

Supplemental, no acceptance credit: at main, githubDecayToken reads only the named token variables and has no executable lookup or subprocess; the delivery diff removes the os/exec import, the lookup and the stored-login subprocess.

Fail-first: owed on no row. Risk fields are all no, and no row is marked risk-bearing, live, mutating or end-to-end.

RISK-VALUE: DERIVED — public and operator-endpoint vars = GH_TOKEN, GITHUB_TOKEN @ statusgen/claimdecay_github.go:68 — Task 1 requires these explicit names and this order unchanged; the delivery diff leaves the line as context.
RISK-VALUE: DERIVED — derived-enterprise vars = GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN @ statusgen/claimdecay_github.go:70 — a host guessed from origin gets only an enterprise credential, so a lookalike host never receives the github.com token; unchanged and exercised by row 2.
RISK-VALUE: NAMED, NOT DERIVED — githubDecayClient Timeout = 30s @ statusgen/claimdecay_github.go:57 and githubDecayMaxPages = 1000 @ statusgen/claimdecay_github.go:52 — reversible operational knobs, not changed by the delivery, ranked last.

## Review
Gate: model. Apply the security-review skill to the authentication lookup boundary and trace selected tokens to the existing request sink. Confirm the CLI path is absent, explicit tokens stay on their current host, missing data remains unavailable and every parent obligation remains. The original /18 delivery is still partial after this contribution.
