---
brief: assay:assay:forge-neutral:03
title: Write verbs A — deskpost, deskreply and deskflip onto the resolver
why: >-
  These three verbs carry the review loop's outward writes: the verdict, the reply, and the
  ready flip. Two of them shell `gh` (seven of the permit register's twenty-four rows are
  deskflip's and deskreply's alone) and one builds raw requests against a hardcoded GitHub
  host. Until they take their forge from the resolver, the review loop is a GitHub-only loop —
  and on any other forge the only way to post a verdict is a hand-built call with none of the
  verbs' guards.
wave: 2
depends: ["forge-neutral/01"]
unblocks: ["forge-neutral/06", "forge-neutral/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver and the refusal contract these verbs consume"
  - "tools/desk/internal/forgeban/allowlist.go — the seven permit rows this brief retires and the ratchet that must come down with them"
  - "docs/streams/forge-gitlab/pilot-report.md steps A9, B4, B6 — the verdict note, the approval and the ready flip as performed by hand on GitLab, i.e. exactly what these verbs must do instead"
  - "freshness-checked 2026-09-02 @ deae247 — deskflip shells gh at flip.go:325,453,675,681,854,870,914,941; deskreply at exec.go:79; deskpost builds net/http against deskkit.GitHubAPIBase (github.go:33) with markReadyForReview on a GraphQL node id (github.go:874)"
exec-tier: strong
exec-tier-why: "a transport swap under a security-relevant path where a divergence in draft semantics, review-state mapping or pagination survives happy-path tests but changes what the review gate believes (questions b and c)."
domain: complicated
consumers:
  - "tools/desk/cmd/deskpost: fixed-here"
  - "tools/desk/cmd/deskreply: fixed-here"
  - "tools/desk/cmd/deskflip: fixed-here"
  - "tools/desk/internal/forgeban/allowlist.go: fixed-here (eight rows removed, ceiling lowered to 16 — the amendment's MEASURED figure, superseding the frozen 17)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: out-of-scope (the skill names the verbs, not their transport; no skill text changes when a verb's backend does)"
version: 1
id: 416b7ef5-5d82-417f-b831-dd2928c60d6c
---

# Brief 03 — Write verbs A: deskpost, deskreply, deskflip

> **Amended 2026-09-05** per desk ruling on #454 (rows 3/5/8/11): row 3 forbids
> REMOVING or LOOSENING any assertion — NOT freezing the test files; re-pointing a test's
> transport (and editing/adding its `_test.go`) is expressly allowed, provided each retired
> `gh`-argv assertion gains a named successor on the HTTP-transport recorder as a 1:1 map in
> the diff. Row 8 is scoped to NON-TEST files only — `harness_test.go`'s `apiBaseURL`
> assignment moves with the harness. The ceiling (rows 4–5) is now MEASURED, not pinned:
> `(gh launch sites on main at this sha) − 8` = **16** today (the seven deskflip rows —
> `flip`, `ensureLabelSwap`, `readPR`, `readReviews`, `readChangedFiles`, `readHead`,
> `readLabelEvents` — plus the one deskreply row), which supersedes every frozen `17` /
> "seven rows" figure elsewhere in this brief (`why`, `consumers`, Task step 5). Row 11's
> new-transport test lives under `cmd/deskflip/` and is exempt from row 3's no-weakening
> rule. See #454.

## Context
files:
- `tools/desk/cmd/deskpost/github.go`, `tools/desk/cmd/deskpost/ready.go` — the `net/http`
  paths move onto the resolver.
- `tools/desk/cmd/deskreply/exec.go`, `tools/desk/cmd/deskreply/deskreply.go` — the shelled
  `gh` comment path.
- `tools/desk/cmd/deskflip/flip.go`, `tools/desk/cmd/deskflip/exec.go` — six shelled paths.
- `tools/desk/internal/forgeban/allowlist.go` — seven permit rows removed, ceiling lowered.

**Why the risk answers are all `no` even though `tools/desk/cmd/deskpost/` is a security path.**
This brief changes TRANSPORT, not custody: the acting identity for all three verbs is
unchanged, because `deskpost` and `deskflip` already mint their reviewer App installation
token and refuse an ambient fallback, and `deskreply` already mints a worker token. The
custody question those mints answer was settled under the human gate in `forge-neutral/01`;
nothing here re-opens it. The verbs whose acting identity DOES change are in
`forge-neutral/04`, which is human-gated for exactly that reason. Verify row 10's zero-write
assertion is what holds this claim honest.

single-point-of-failure: the resolver is the one control deciding which forge and which
identity each write goes to. The second, independent layer is the permit register's ratchet
(`allowlist.go:56`) plus the shell-exec ban: a migration that "moves" a call site by leaving
the old one reachable fails the ban even though every functional test passes, because the ban
walks launch sites rather than behavior.

facts:
- `deskflip` holds SIX permit rows — `flip`, `ensureLabelSwap`, `readReviews`,
  `readChangedFiles`, `readHead`, `readLabelEvents`
  (`tools/desk/internal/forgeban/allowlist.go:101,106,115,121,126,131`); `deskreply` holds
  one (`allowlist.go:155`). `deskpost` holds none — it is `net/http`, not a shell-out, so it
  moves the ratchet by zero.
- `deskflip` already mints a reviewer App installation token and refuses an ambient fallback
  (`tools/desk/cmd/deskflip/exec.go:26,51`) — so its identity blocker is already answered and
  it is the cheapest of the identity-class rows to retire.
- `deskreply` mints a worker installation token via `desktoken`
  (`tools/desk/cmd/deskreply/exec.go:30,82`), with a documented wrong-installation hazard at
  `exec.go:89`.
- The enumerated operations these three need already exist on both backends: `PostComment`
  (`forge.go:198`), `PostReview` (`:200`), `MarkReadyForReview` (`:203`), `ReviewsAtHead`
  (`:181`), `ListChangedFiles` (`:184`), `ChecksAtHead` (`:186`), `GetPullRequest` (`:177`).
  No interface addition is needed for them.
- Labels are NOT an enumerated operation. `deskflip`'s `ensureLabelSwap` and `deskpost`'s
  `ensureLabel` (`tools/desk/cmd/deskpost/github.go:669`) therefore need a typed op added WITH its
  consuming call site in the same change, per the freeze rule (`forge.go:169-172`).
- `MarkReadyForReview` takes a node id (`forge.go:203`); GitHub uses the GraphQL node id
  (`tools/desk/cmd/deskpost/github.go:874`), and `GitLabForge` encodes a synthetic one
  (`forge_gitlab.go:302,309`). Callers must obtain it from `GetPullRequest`, never construct
  it.
- Review-state vocabulary differs: on GitLab CE the at-head property lives in the note body,
  not in the approval — the rule is `docs/streams/forge-neutral/identity.md` (planned),
  created by forge-neutral/02, §corroboration. This brief consumes the transport, not the
  corroboration rule; 08 consumes the rule.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green **unmodified**. A migration that edits its own
  tests to pass has verified nothing.
- Delete the dead shell helpers; do not leave them dormant. A reachable old path is not a
  migration.

## Task
1. Route `deskpost`'s forge operations through `deskkit.ForgeFor`, deleting `apiBaseURL` and
   the hand-rolled request construction in `tools/desk/cmd/deskpost/github.go`. Its App-mint identity
   path moves to the resolver's custody binding.
2. Route `deskreply`'s comment write through `PostComment`; delete the `gh` helper in
   `tools/desk/cmd/deskreply/exec.go` and its permit row.
3. Route `deskflip`'s six paths: `pr ready` → `MarkReadyForReview` (node id from
   `GetPullRequest`), the review read → `ReviewsAtHead`, the changed-files read →
   `ListChangedFiles`, the head read → `GetPullRequest`, the checks read → `ChecksAtHead`.
   Delete each shell helper and its permit row.
4. **Labels.** Add ONE typed label operation to `Forge` with its two consuming call sites
   (`deskflip`'s `ensureLabelSwap`, `deskpost`'s `ensureLabel`) in this same change, implement
   it on both backends (GitHub labels ↔ GitLab MR labels), extend
   `docs/streams/forge-gitlab/inventory.md` with the new op and its consumers, and confirm the
   no-passthrough shape check still passes. If the GitLab mapping turns out not to be
   1:1, the operation returns could-not-check naming the gap — it does not approximate.
5. Lower `allowedInvocationCeiling` from 24 to the MEASURED ceiling (**16** today, per the
   amendment's row 5) and remove the eight retired rows.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check | `cd tools/desk && git stash list >/dev/null; go test ./cmd/deskpost/... ./cmd/deskreply/... ./cmd/deskflip/... -count=1` | exit 0 — all three suites green |
| 3 | check | `git diff origin/main -- tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskflip -- '*_test.go'` | **no assertion is removed or loosened** (this row forbids weakening coverage, NOT editing the test files). Re-pointing a test's transport during this migration is EXPRESSLY ALLOWED — editing or adding a verb's `_test.go` is expected. The diff must read as a **1:1 map**: each retired `gh`-argv assertion has a **named successor assertion on the HTTP-transport recorder** with the same request shape (method, path, body fields, auth-header presence). A diff that DROPS an argv assertion with no named successor, or WEAKENS one (fewer fields checked, a dropped read-count counter), FAILS this row. Reviewer confirms the map is complete and reviewable. |
| 4 | check | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows `= 16` — the MEASURED ceiling from row 5, not a frozen literal |
| 5 | check | **Measure the ceiling, do not hard-code it.** The measuring command counts the `gh` launch sites on main at the amendment sha: `git fetch origin && grep -c '::gh"' <(git show refs/remotes/origin/main:tools/desk/internal/forgeban/allowlist.go)` (**24** at the amendment sha). The ceiling is that count **− 8** (the seven deskflip rows + the one deskreply row this brief retires) = **16 today**. Then `cd tools/desk && go test ./internal/forgeban/... -count=1 -v` | exit 0; the ratchet test passes at the MEASURED ceiling — it fails if the register is longer OR shorter than the ceiling. **Record the measuring command and its count in the Evidence row**, so a future launch-site drift is a re-measure, not a brief rewrite. |
| 6 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 — no `gh`/`glab` launch remains in these three verbs, and the label op added no passthrough |
| 7 | check | `grep -rn -e '"gh"' tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskflip --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — independent cross-check of row 6 by a different instrument |
| 8 | check | `grep -rn -e 'apiBaseURL' tools/desk/cmd/deskpost --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — `apiBaseURL` is absent from **non-test files** under `cmd/deskpost` (the hardcoded host binding is gone, not merely unused). Scope is NON-TEST files only: `harness_test.go`'s `apiBaseURL` assignment MOVES WITH THE HARNESS (a test file — allowed, and required for the deskpost suite to compile). |
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestLabelOpBothBackends -count=1 -v` | exit 0; the new label op runs against both recorded backends' fixtures with the same scenario names |
| 10 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestFlipRefusesUnsupportedForge -count=1 -v` | **negative path**: with the resolver returning a forge whose backend cannot serve the ready flip, `deskflip` exits could-not-check (class 6) naming forge and operation, and performs NO write — asserted by a recording transport that must show zero write calls |
| 11 | check | `cd tools/desk && go test ./cmd/deskflip/... -run TestNodeIDNotConstructed -count=1 -v` | **negative path**: the flip path obtains its node id from `GetPullRequest` and refuses a locally-composed one, so a GitLab synthetic id cannot be forged by string-building. This new-transport test lives under `cmd/deskflip/`, beside the suite it extends; **row 3's no-weakening rule does NOT cover it** — adding a test that exercises the new transport is expected, not a frozen-file violation. |
| 12 | check | `grep -c 'label' docs/streams/forge-gitlab/inventory.md` | ≥ 1 — the new op is recorded in the frozen inventory with its consuming verbs |
| 13 | check | `statusgen --root . --consumers --brief forge-neutral/03` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |
| 14 | check +mutation | In `checkAppToken` (`tools/desk/cmd/deskflip/flip.go`) disable the app-token condition's refusal — change its `if err != nil {` to `if false && err != nil {` — then run `cd tools/desk && go test ./cmd/deskflip/ -run TestNoAppToken -count=1`; restore the file and re-run | exit **1** on the mutant: `TestNoAppTokenRefusalNamesTheRoleAndThePath` fails, because with the condition disabled the refusal no longer names `app-token`, the role, or the token path an operator has to go fix. exit 0 again after restoring. **This is the mutation row for the control this brief MOVED.** Before the migration, "never the ambient forge identity" was ALSO backstopped by a `ghToken == ""` check inside each verb's `runCmd` (`tools/desk/cmd/deskflip/exec.go`, `tools/desk/cmd/deskreply/exec.go`); those helpers are deleted with the CLI they guarded. The mutant is worth running for its SECOND result too: `TestNoAppTokenRefusesAndNeverTouchesTheForge` still PASSES on it, and makes no forge call — because the resolver's own custody step refuses independently, in a different component, on a different signal. The deleted backstop was replaced by a layer, not merely removed |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The `Forge` calls land but the old `gh` helper stays reachable, so nothing actually moved (the `#274` shape) | rows 6 + 7 (a Go launch-site walk and a grep — two instruments) + row 8 |
| The migration is "made to pass" by editing the verbs' own tests | row 3 |
| The ratchet is not lowered, so the gain is never locked in and the next brief inherits a false baseline | rows 4 + 5 |
| A label op is added speculatively without its call sites, breaking the freeze rule | row 9 + row 12 (the inventory row names the consumers) |
| A GitLab label mapping is approximated rather than refused where it does not fit | row 9 runs the same scenario names on both backends; a divergence is a named failing scenario |
| The unsupported-forge path degrades to a raw request instead of refusing | row 10, which asserts zero write calls, not merely a non-zero exit |
| The node id is composed locally, so a synthetic GitLab id is built by string concatenation and drifts from the backend's encoding | row 11 |
| Draft semantics diverge (GitHub `draft` field vs GitLab `Draft:` title prefix) so a flip silently no-ops | rows 2 + 9 — the backends' own golden/fixture scenarios cover it; a behavioral divergence is a named failing scenario rather than a silent pass |
| The verdict body's secret scan or audit line is lost when the transport changes | **no row here** — the guards WRAP the call (`forge.go:10-17`) and are unchanged by a transport swap; the Review gate confirms the wrapper still encloses the new call site |

## Evidence
### Non-implementer verifier run — VERIFY: FAIL (row 8 — deskpost's hardcoded-host REST path not removed; migration incomplete, #558) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `67abbac`

Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`); desk rows module-scoped from `tools/desk`. Frontmatter: `gate: model`, all risk `no`, `irreversible: no`. Landing commit `f756f19` (routes deskflip + deskreply through the forge resolver — title names deskflip+deskreply, NOT deskpost).

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | tools/desk go build ./... && go test ./... | exit 0 | build exit 0; go test FAIL ONLY on the pre-existing deskkit red (#555), unrelated to the forge transport. COULD-NOT-CHECK (build green; brief suites green in row 2) | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | go test ./cmd/deskpost/... ./cmd/deskreply/... ./cmd/deskflip/... | exit 0, three suites green | exit 0 — ok deskpost, deskreply, deskflip | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | git diff of the verbs' *_test.go (retired gh-argv asserts → HTTP-recorder successors) | no assertion removed/loosened; 1:1 re-point | removed lines are old gh-argv asserts; new deskreply/forgerecorder_test.go (+161) + forgelabel_test.go (+270) add the transport-recorder successors — consistent with the #454 re-pointing amendment | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | grep allowedInvocationCeiling allowlist.go | = 16 | exit 0 — const allowedInvocationCeiling = 16 @ allowlist.go:64 | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | measure gh launch sites + go test ./internal/forgeban | ratchet passes at measured ceiling | live `::gh"` count on merged main = 16 (24 at amendment − 8 retired); go test ./internal/forgeban exit 0 ok | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | go test ./internal/deskkit -run no-forge-cli-shellout && forge-no-passthrough | exit 0 | exit 0 — both PASS (forge-no-passthrough incl. 5 subtests) | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | grep -rn '"gh"' <verbs> excl _test.go | 0 | 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | grep -rn 'apiBaseURL' cmd/deskpost excl _test.go | 0 (hardcoded host binding gone) | **FAIL — 9.** cmd/deskpost/github.go still declares `var apiBaseURL = deskkit.GitHubAPIBase` (:32) + builds hand-rolled requests against it (doJSONRetry :340, IssueReactions :956, mint :184). Task 1 (delete apiBaseURL + hand-rolled request construction) NOT completed; deskpost's mutating writes were routed but the host-bound REST path remains reachable — the "#274 shape" on deskpost. Filed #558 | 2026-09-06 | opus-4.8[1m]-verifier |
| 9 | go test ./internal/deskkit -run label-op-both-backends | exit 0, both backends | exit 0 — PASS (4 scenarios each /github + /gitlab) | 2026-09-06 | opus-4.8[1m]-verifier |
| 10 | go test -run flip-refuses-unsupported-forge | exit 0, zero writes on refuse | exit 0 — PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 11 | go test ./cmd/deskflip -run node-id-not-constructed | exit 0, node id from GetPullRequest | exit 0 — PASS (3 subtests) | 2026-09-06 | opus-4.8[1m]-verifier |
| 12 | grep -c label docs/streams/forge-gitlab/inventory.md | ≥1 | 6 | 2026-09-06 | opus-4.8[1m]-verifier |
| 13 | statusgen --consumers --brief forge-neutral/03 | exit 0 | COULD-NOT-CHECK — exit 2, aborts on docs/streams/decisions/README.md "no frontmatter" (#557), never reaches this brief | 2026-09-06 | opus-4.8[1m]-verifier |
| 14 | MUTATION: flip checkAppToken guard, run TestNoAppToken, restore | mutant exit 1; restored exit 0 | COULD-NOT-CHECK — the mutation Edit was blocked by the permission classifier; per no-evasion the verifier did not substitute another write tool. Baseline (unmutated) no-app-token refusal tests present + PASS; mutant half unverified | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — allowedInvocationCeiling = 16 @ tools/desk/internal/forgeban/allowlist.go:64 (changed 24→16) — 24 gh launch sites at the amendment sha − 8 rows this brief retires (7 deskflip + 1 deskreply) = 16; the forgeban ratchet passes exactly at 16 (fails if longer OR shorter), and the live ::gh" count on merged main is 16.`
`RISK-VALUE: NAMED, NOT DERIVED — label-op success codes 422 (already-present) / 404 (already-absent) in the new Forge label op — encode GitHub's idempotency semantics; label-op-both-backends exercises the scenarios but a passing test is not the derivation, and confirming 422/404 are the correct idempotent-success codes against the live GitHub + GitLab label APIs is offline-unavailable.`

**VERIFY: FAIL — row 8.** deskflip + deskreply are correctly routed through the forge resolver (rows 2-7, 9-12 PASS; ratchet at 16), but the merge left deskpost's `apiBaseURL` + hand-rolled hardcoded-host REST in cmd/deskpost/github.go reachable (row 8 = 9, expected 0) — brief task 1 incomplete, the "#274 shape" on deskpost. rows 6/7 miss it because deskpost was always net/http, never a gh shellout. Filed #558. Rows 1/13/14 additionally could-not-check (deskkit red #555; --consumers block #557; blocked mutation write). Status stays `implemented`; flips once #558 fixed and row 8 == 0. CFR sidecar row appended (verify-fail).

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local)

Target: merged `origin/main` @ `a91bffd0` (the head at verify time; main has since advanced to `13814ff8` by the unrelated #814 plugin-manifest stamp — this Evidence lands from `13814ff8`). Offline (`KUBECONFIG=/dev/null`, go1.26.5) in an isolated worktree; runner ≠ implementer. gate: model, risk all=no.

| # | Command (in `tools/desk`) | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | 0 | build 0; full `./...` green, no FAIL/panic | PASS |
| 2 | `go test ./cmd/deskpost/... ./cmd/deskreply/... ./cmd/deskflip/...` | 0 | ok deskpost 27.8s, deskreply 8.0s, deskflip 7.3s | PASS |
| 3 | diff vs origin/main of the three verbs' `*_test.go` + assertion-strength inspection | 0 | properly-scoped verb-dir diff EMPTY on merged main; 0 `"gh"`-argv assertions remain in the verb test files; HTTP-recorder successors present (`cmd/deskreply/forgerecorder_test.go`, `internal/deskkit/forgelabel_test.go`) — 1:1 re-point confirmed | PASS |
| 4 | `grep -n allowedInvocationCeiling internal/forgeban/allowlist.go` | 0 | `const allowedInvocationCeiling = 9` @ :64 (NOT the brief's stale 16 — drift note below) | PASS |
| 5 | measure `::gh"` + `go test ./internal/forgeban/...` | 0 | measured count = 9 (`grep -c '::gh"' <(git show refs/remotes/origin/main:…/allowlist.go)`); ratchet test PASS; `forgeban_test.go:330` asserts `len(AllowedInvocations)==allowedInvocationCeiling` (both 9) — fails if longer OR shorter | PASS |
| 6 | `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | 0 | both ok — no gh/glab launch remains in the three verbs; label op added no passthrough | PASS |
| 7 | `grep '"gh"'` three verbs, non-test | — | `0` | PASS |
| 8 | `grep 'apiBaseURL' cmd/deskpost`, non-test | — | `0` — the hardcoded-host REST binding is gone (this was the sole blocker of the 2026-09-06 run @ 67abbac, #558; now fixed) | PASS |
| 9 | `TestLabelOpBothBackends` | 0 | 4 scenarios × {github, gitlab}, identical scenario names both backends, all PASS | PASS |
| 10 | `TestFlipRefusesUnsupportedForge` (negative) | 0 | `refuses_could_not_check_and_writes_nothing` PASS; `writeRecorder` asserts ZERO write calls on refuse; refusal is could-not-check naming forge+op | PASS |
| 11 | `TestNodeIDNotConstructed` (negative) | 0 | 3 subtests PASS — node id from `GetPullRequest`, refuses an opaque/composed id, no composed id reaches the flip | PASS |
| 12 | `grep -c label docs/streams/forge-gitlab/inventory.md` | — | `12` (≥1) — the label op recorded in the frozen inventory | PASS |
| 13 | `statusgen --root . --consumers --brief forge-neutral/03` | — | COULD-NOT-CHECK — offline verifier's shared-home writeguard (statusgen writes STATUS.md; exemption human-only) + statusgen v1.0.6 brief-v2 gap. All `fixed-here` consumers corroborated by rows 6/7/8 (verbs off gh/apiBaseURL) + 4/5 (ceiling lowered) | COULD-NOT-CHECK |
| 14 | MUTATION: `checkAppToken` (`cmd/deskflip/flip.go`) `if err != nil` → `if false && err != nil`; run `TestNoAppToken`; restore; re-run | mutant 1 / restored 0 | Mutant: `TestNoAppTokenRefusalNamesTheRoleAndThePath` FAILS (refusal no longer names app-token/role/path); `TestNoAppTokenRefusesAndNeverTouchesTheForge` still PASSES on the mutant. Restored → both PASS, worktree clean | PASS |

**Risk-bearing value (ENUMERATE → RANK → DERIVE):**
- `RISK-VALUE: DERIVED — allowedInvocationCeiling = 9 @ tools/desk/internal/forgeban/allowlist.go:64.` Measuring command `grep -c '::gh"' <(git show refs/remotes/origin/main:…/allowlist.go)` → 9; register `AllowedInvocations` length = 9 = ceiling; ratchet passes exactly at 9. Ranked #1 (a security ratchet — a loosened value re-permits a banned shell-out). This is a STRONGER ban than the brief's stale 24−8=16: forge-neutral 04/06/13 (since merged) retired further launch sites, ratcheting 16→9. A lower ceiling re-permits nothing (tightens as designed); row 4's literal moving 16→9 is expected drift, and the ratchet TEST (row 5) is the arbiter — it passes.
- `RISK-VALUE: could-not-check-against-live (not a file:line literal) — the label-op idempotency success codes and the class-6 could-not-check exit encode forge protocol semantics exercised by TestLabelOpBothBackends / TestFlipRefusesUnsupportedForge; confirming them against the LIVE GitHub/GitLab APIs is offline-unavailable. Recorded-fixture scenarios pass identically on both backends. Not a pinned constant at a file:line, so not a blocking F-28 flag; an online-lane confirmation, not an un-derived risk value.`

**Defense-in-depth (gate: model):** Row 14 proves TWO independent layers behind "never the ambient forge identity." (1) The app-token refusal is a LIVE control — disabling its condition makes `TestNoAppTokenRefusalNamesTheRoleAndThePath` fail. (2) A SECOND independent layer catches the same fault on a different signal in a different component: with the mint-guard bypassed, `TestNoAppTokenRefusesAndNeverTouchesTheForge` still PASSES and makes zero forge calls, because the resolver's own custody step refuses independently. The deleted per-verb `ghToken==""` backstop was replaced by a layer, not merely removed.

**Scope-traceability:** no work maps to no row; all `consumers: fixed-here` claims (deskpost, deskreply, deskflip, allowlist.go) corroborated by rows 4-8. Row 3's raw diff surfaced only unrelated #814 content (a double-`--` pathspec artifact); the three verb dirs are unchanged vs origin/main.

**VERDICT: PASS** — rows 1–12 + mutation row 14 PASS on merged main; row 13 COULD-NOT-CHECK (statusgen environment limit + brief-v2 gap, non-blocking, consumer routing corroborated by other rows). Risk-value ceiling DERIVED. Flip-eligible (gate: model, all risk no).

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — this brief changes
transport and adds one enumerated operation, it does not change who may write or what is
believed). Reviewer records verdict + date in the stream README table.
