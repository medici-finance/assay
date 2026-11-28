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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayNoStoredLogin$' . > "$o" && grep -F -- '--- PASS: TestDecayNoStoredLogin ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b514dee5226d | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayTokenStaysOnItsHost$' . > "$o" && grep -F -- '--- PASS: TestDecayTokenStaysOnItsHost ' "$o")` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715
Container-Image: sha256:3cfd931fc8fb2277df25eafbc68eccfe84ce7d7b92379b927b8c1841d99854d1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6967 run=c94bb8180414d888e4233f3163516f15084a85a6f60fa700db4e5ae25469596b source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c94bb8180414d888e4233f3163516f15084a85a6f60fa700db4e5ae25469596b source=9c845402c8c00d7c14bda75cd5222fff907fd05d tree=sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715 witness=sha256:a3fd571abb5d86010a53149612a6457b011d63cad221d82eae80d3e757e3f11e opened=2026-11-18T17:53:58Z closed=2026-11-18T17:57:22Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh tools/desk/scripts/forge-ban.sh` | could-not-run | host execution refused by the session's permission classifier (Undermining Oversight); stopped without retry | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief forge-neutral/49 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | could-not-run | host execution refused by the session's permission classifier (Undermining Oversight); stopped without retry | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6967 run=c94bb8180414d888e4233f3163516f15084a85a6f60fa700db4e5ae25469596b source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — row 1 pass; row 2 environment-held (admitted-row markers refuse the test's explicit tokens); rows 3-4 could-not-run (host permission refusal). No obligation was observed failing.

Re-verification by claude-opus-5-5 (strong, non-implementer) at merged main 9c845402c8c0, statusgen v1.0.87, verify-runner v1.0.87 image named above, KUBECONFIG=/dev/null, GH_TOKEN unset. Delivery: PR 5391, merge 32f85d97060e, first parent 6bceac0fa40e. Rows 1 and 2 ran in the container admission bracket (network none, read-only root, fresh module cache, bracket-digest verdict=match).

Row 2 is held by its environment. The grep found no PASS line because the test failed. Inside an admitted Verify row the environment carries the grep-pin git config variables (GIT_CONFIG_COUNT=2 with grep.patternType and grep.extendedRegexp) that admittedRowEnv appends. Since PR 6749 (issue medici-finance/assay#6746, commit 0184ed5f117a, after the 2026-11-10 pass), githubDecayCredential treats those pins as an admitted context and takes only the run-scoped credential, so the test's own explicit GH_TOKEN and GH_ENTERPRISE_TOKEN values are never read. Seven subtests fail with "could-not-check: inside an admitted Verify row the forge is read only with the verifier's run-scoped read-only credential, and the run handed this row none". The test does not clear those pins, so it cannot pass inside any admitted row. Supplemental probes in the same image, no acceptance credit: run directly without the admitted-row variables, all nine subtests pass, including stored-login-is-not-a-credential-source; with those variables set by hand, the same seven fail. The delivered behaviour is unchanged. The row fails only in admission. Next actor: the owner of the PR 6749 change (test hermeticity) or the brief owner (row definition).

Rows 3 and 4 were refused on the host before any command ran, by the session's permission classifier: "Permission for this action was denied by the Claude Code auto mode classifier. Reason: [Undermining Oversight]." That refusal is a stop, so there was no retry. Row 4 also keeps the check-definition hold tracked in medici-finance/assay#5564: after the merge, the merge-base is HEAD.

Supplemental, no acceptance credit: at main, statusgen/claimdecay_github.go has no os/exec import, executable lookup or gh subprocess (its only "gh" text is in a comment at line 9).

Fail-first: owed on no row. Risk fields are all no, and no row is marked risk-bearing, live, mutating or end-to-end.

RISK-VALUE: DERIVED — public and operator-endpoint vars = GH_TOKEN, GITHUB_TOKEN @ statusgen/claimdecay_github.go:83 — Task 1 requires these explicit names and this order unchanged; the line moved from 68 when PR 6749 landed, with its content unchanged.
RISK-VALUE: DERIVED — derived-enterprise vars = GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN @ statusgen/claimdecay_github.go:85 — a host guessed from origin gets only an enterprise credential, so a lookalike host never receives the github.com token; content unchanged.
RISK-VALUE: NAMED, NOT DERIVED — githubDecayClient Timeout = 30s @ statusgen/claimdecay_github.go:59 and githubDecayMaxPages = 1000 @ statusgen/claimdecay_github.go:54 — reversible operational knobs, not changed by the delivery, ranked last.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayNoStoredLogin$' . > "$o" && grep -F -- '--- PASS: TestDecayNoStoredLogin ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b514dee5226d | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout=90s -v -run '^TestDecayTokenStaysOnItsHost$' . > "$o" && grep -F -- '--- PASS: TestDecayTokenStaysOnItsHost ' "$o")` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7716 run=e756574f8eff79492925b18de74150ec8937fec4ba4c1d7164a108b109784d32 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e756574f8eff79492925b18de74150ec8937fec4ba4c1d7164a108b109784d32 source=77ff0f2222a28df926c38ad30fea4a368173712e tree=sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6 witness=sha256:96e493b38b1eb8e1594a0f5111dca09b84da5f5d761fb0043bff4140e8906618 opened=2026-11-25T05:55:15Z closed=2026-11-25T05:57:03Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh tools/desk/scripts/forge-ban.sh` | pass exit=0 | sha256:fdcebd3e2e07 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief forge-neutral/49 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:810c6b8f9250 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7716 run=e756574f8eff79492925b18de74150ec8937fec4ba4c1d7164a108b109784d32 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/forge-neutral/brief-49-retire-claim-decay-stored-github-login-fallback.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — rows 1 and 3 pass; row 2 environment-held in the admitted container (medici-finance/assay#7022 open); row 4 could-not-check at merged main (check-definition, medici-finance/assay#5564 open). No delivered obligation was observed failing.

Re-verification by claude-opus-5-5 (strong, non-implementer) at merged main 77ff0f2222a2, statusgen v1.0.94, verify-runner v1.0.94 image named above (digest-only reference, the house release pin), KUBECONFIG=/dev/null, forge token variables unset. Delivery: PR 5391, merge 32f85d97060e, first parent 6bceac0fa40e, second parent 877a9a25c1d7. Rows 1 and 2 ran in the container admission bracket (network none, read-only root, fresh module cache, bracket-digest verdict=match). Rows 3 and 4 ran on the host through statusgen verifyrun with a row selector. No source file this brief delivers has changed since the 2026-11-18 run.

Row 2 is held by its environment, as tracked in medici-finance/assay#7022. The test fails only because an admitted Verify row carries the grep-pin git config variables, which the credential selector treats as an admitted context, so the test's explicit token fixtures are never read. Supplemental probes in the same image, no acceptance credit: with GIT_CONFIG_COUNT=0 all nine subtests pass, including stored-login-is-not-a-credential-source; with the two grep pins set by hand, seven fail with "could-not-check: inside an admitted Verify row the forge is read only with the verifier's run-scoped read-only credential, and the run handed this row none". The two that still pass are github-com-never-gets-enterprise-token and stored-login-is-not-a-credential-source.

Row 3 at 77ff0f2222a2 reads 69 sites (desk 40, statusgen 29), class a statusgen=20, exit 0. The scan script is unchanged since the first parent. Its Expect pins counts to the pickup source, so the same script was also run on exported trees: the first parent reads statusgen 39, class a statusgen=31; the delivery merge reads 38 and 30. That is the Expect's one fewer subprocess. The later drop at main comes from other work and is no /18 cutover claim.

Row 4 exits 2 with COULD-NOT-CHECK, unchanged from the earlier runs: after the merge the merge-base of origin/main and HEAD is HEAD itself, so the diff is empty and the run carries no evidence about the brief's claims. This is not a disproof. The repair is the reviewed check-definition change tracked in medici-finance/assay#5564 (next actor: brief owner).

Supplemental, no acceptance credit: at main, statusgen/claimdecay_github.go has no os/exec import, executable lookup or subprocess. The delivery diff of that file only removes the import, the lookup, the stored-login subprocess and the login hint in two error messages.

Fail-first: owed on no row. Risk fields are all no, and no row is marked risk-bearing, live, mutating or end-to-end.

RISK-VALUE: DERIVED — public and operator-endpoint vars = GH_TOKEN, GITHUB_TOKEN @ statusgen/claimdecay_github.go:83 — Task 1 requires these explicit names and this order unchanged; the delivery diff leaves the line as context.
RISK-VALUE: DERIVED — derived-enterprise vars = GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN @ statusgen/claimdecay_github.go:85 — a host guessed from origin gets only an enterprise credential, so a lookalike host never receives the github.com token; content unchanged.
RISK-VALUE: NAMED, NOT DERIVED — githubDecayClient Timeout = 30s @ statusgen/claimdecay_github.go:59 and githubDecayMaxPages = 1000 @ statusgen/claimdecay_github.go:54 — reversible operational knobs, not changed by the delivery, ranked last.

## Review
Gate: model. Apply the security-review skill to the authentication lookup boundary and trace selected tokens to the existing request sink. Confirm the CLI path is absent, explicit tokens stay on their current host, missing data remains unavailable and every parent obligation remains. The original /18 delivery is still partial after this contribution.
