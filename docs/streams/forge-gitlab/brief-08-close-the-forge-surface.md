---
brief: assay:assay:forge-gitlab:08
title: Close the forge surface — enumerated operations, no passthrough, shell-exec ban
why: >-
  A constrained typed surface is stronger than an ambient full-CLI one: the spec's governing
  requirement is that the forge profile be at least as secure as the existing GitHub controls,
  and shelling `gh`/`glab` (or exposing a generic `Do(endpoint)` escape hatch) re-opens the
  entire CLI/API surface the interface was meant to fence. This brief makes the fence
  enforceable — the Forge exposes ONLY the enumerated operations the assay workflows need, no
  arbitrary-endpoint method survives on either backend, and a lint/test asserts zero
  `exec.Command("gh"|"glab", …)` anywhere in tools/desk — so "limit the tool to our workflows,
  not a wide-open CLI" is a checked property, not a convention.
wave: 3
depends: ["forge-gitlab/02", "forge-gitlab/07"]
unblocks: ["forge-gitlab/11"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-26 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (parity table — a constrained surface is the stronger side), §6 (interface FROZEN at consumed operations; additions require a consuming tool)"
  - "docs/streams/forge-gitlab/brief-07-github-forge-go-gh.md — the go-gh GitHub backend the newly-enumerated ops land on"
  - "docs/streams/forge-gitlab/brief-02-gitlab-forge-impl.md — the GitLab backend the ban applies to symmetrically (no glab shell, no arbitrary-endpoint method)"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op set the closed surface is measured against"
  - "freshness-checked 2026-08-26 @ e89cf5a — 20 `exec.Command(\"gh\", …)` call sites remain across tools/desk/cmd/** (deskflip, deskboard, deskmerge, deskdispatch, deskroster, deskadvisory, deskpushguard, issueboard, scanloop, repohardenguard, deskdisposition), and fanoutloop reaches an arbitrary endpoint via `gh api -X DELETE repos/…/git/refs/…` (tools/desk/cmd/fanoutloop/land.go) — a passthrough the enumerated surface must replace with a typed op"
exec-tier: strong
exec-tier-why: "a security-control brief where the deliverable IS the control: a gap in the ban's detection (a missed callsite form, a passthrough method left reachable) ships a false-clean, which is exactly the failure the parity requirement forbids (question c)."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the ban lint/test + any newly-enumerated ops)"
  - "tools/desk/cmd/*: fixed-here (residual gh call sites route through the interface or are removed)"
version: 1
id: 932f1239-5355-4be2-9f61-bcd7682931a4
---

# Brief 08 — Close the forge surface

## Context
files:
- `tools/desk/internal/deskkit/forge.go` — the frozen interface: confirm NO generic
  `Do`/`Raw`/`APIRequest`/arbitrary-endpoint method exists; add typed ops for the residual
  shell callsites that are genuine forge operations (e.g. a `DeleteRef(repo, ref)` to retire
  fanoutloop's `gh api -X DELETE`, a `ListPullRequests`/`SetLabels`/`ReadyForReview` op where
  a tool shells one) — each addition carries its consuming tool in this change (spec §6).
- `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` — implement each newly
  enumerated op on both backends (GitHub via go-gh per brief 07; GitLab via REST v4 per brief
  02); neither backend exposes an arbitrary-endpoint escape hatch.
- `tools/desk/internal/deskkit/forge_surface_test.go` (new) — the two enforcement tests
  (shell-exec ban + no-passthrough), runnable in CI.
- `tools/desk/cmd/**` — the residual `exec.Command("gh"|"glab", …)` forge callsites
  (deskflip, deskboard, deskmerge, deskdispatch, deskroster, deskadvisory, deskpushguard,
  issueboard, scanloop, repohardenguard, deskdisposition, fanoutloop) move onto the interface
  or are removed; the `askassay` gh-probe entry that only exists to check the `gh` binary is
  installed is dropped, since no tool requires the binary any more.

single-point-of-failure: the ban's detection logic is the one control — if it misses a
callsite spelling or a reachable passthrough, it ships a false-clean. It is backed by a
second, independent layer: the interface's method set is asserted by reflection to equal the
committed inventory (no method takes an arbitrary endpoint/path), so a passthrough that slips
the string-level ban is still caught by the shape check (two layers, different mechanisms).

facts:
- Spec §6 freeze rule is the authority for every op added here: the interface stays frozen at
  the operations a SHIPPING tool consumes; each newly-enumerated op MUST land with its
  consuming callsite converted in the same change — no speculative ops.
- The ban targets forge-CLI INVOCATIONS, not string mentions: `exec.Command`/
  `exec.CommandContext` (and the tools' `runCmd`/`run` wrappers) invoking `gh` or `glab`.
  `askassay`'s read-only gh GUARD validates an agent's ad-hoc gh argv — it does not invoke
  gh — and is out of scope except for dropping the now-unnecessary binary-present probe entry.
  Legitimate non-invocation references (test fixtures, the ban's own pattern literal) must not
  trip the check — detect callsites structurally / exclude `_test.go`, do not naive-grep.
- No arbitrary-API method on EITHER backend: no `Do(endpoint)`, no `api(path)` passthrough,
  no exported raw-request method. fanoutloop's `gh api -X DELETE repos/…/git/refs/…` is the
  live example — it becomes a typed `DeleteRef` op, not a generic call.
- Symmetric on GitLab (brief 02): the same tests assert zero `glab` shell (currently trivially
  true — preventive) and no arbitrary-endpoint method on `forge_gitlab.go`, so the closed
  surface is a property of the interface, not of one backend.
- This is the parity deliverable named in spec §3: a constrained typed surface is the STRONGER
  side of the GitHub-controls comparison, not merely equal — an ambient full-CLI/keyring
  surface is what it replaces.
- Coordination (README): touches `tools/desk/**` broadly; land after brief 07 (GitHub on
  go-gh) and brief 02 (GitLab backend) so every residual op has a typed home, and rebase
  across any in-flight desktools-go-git migration of a shared tool rather than running
  concurrently.

## Edition
Minimum GitLab tier: **free** (Community Edition). The typed ops this brief adds land on
Free-tier endpoints — `DeleteRef` on the Branches API (`Tier: Free, Premium, Ultimate`,
https://docs.gitlab.com/api/branches/), and every other residual forge callsite on the table-A
operations already established as Free. Nothing degrades on CE.

Worth stating because it inverts the usual direction: this brief's control — a closed,
enumerated surface with no arbitrary-endpoint escape hatch — is the profile's one *stronger*
-than-GitHub claim, and it is enforced by the fleet's own tests rather than by a licensed
GitLab feature. It costs nothing at any tier, which makes it the cheapest parity gain in the
stream (edition-matrix.md, tables A and C6).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Inventory the residual `gh`/`glab` shell callsites under `tools/desk` and classify each:
   forge operation (→ enumerate + route through the interface) vs. non-forge (out of scope,
   record why). Extend `inventory.md` with the added ops and their consuming tools.
2. Add the typed ops the forge callsites need to `forge.go`; implement on both backends
   (go-gh + REST v4); convert each callsite; delete the dead shell helpers. Drop askassay's
   gh-binary probe entry.
3. Write the shell-exec ban test (fails on any `gh`/`glab` invocation in tools/desk non-test
   code) and the no-passthrough test (reflects the interface method set against the committed
   inventory; fails on any arbitrary-endpoint method on either backend).
4. Keep every affected tool's existing tests green unmodified.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `cd tools/desk && GOWORK=off go test ./... -run TestNoForgeCLIShellout -v` | exit 0; output contains `PASS` — the ban test passes because no `gh`/`glab` invocation remains |
| 3 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | `0` — independent cross-check of the ban across the whole desk tree |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | exit 0; the test reflects `deskkit.Forge`'s method set against `inventory.md` and FAILS on any generic/arbitrary-endpoint method (`Do`/`Raw`/`api`) on the interface or either backend |
| 5 | `go doc ./tools/desk/internal/deskkit Forge \| grep -cE -e 'Do\(' -e 'Raw\(' -e 'APIRequest\(' -e 'Call\('` | `0` — no arbitrary-request method surfaces in the interface's godoc |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — VERDICT: FAIL (held at `implemented`)

Target: merged `origin/main` @ `48b978bb08c468fec52c015d280285698fc362bd` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer. gate: model, risk all=no. Go rows run from `tools/desk` (the sole module).

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | 0 | all packages `ok`; no FAIL/panic | PASS |
| 2 | `go test ./... -run TestNoForgeCLIShellout -v` | 0 | PASS — but log: "13 shipped call site(s) still invoke a forge CLI, across 9 registered declarations… ratchet ceiling 9; 19 could-not-check". Passes because residuals are ALLOWLISTED, not because none remain | PASS (literal) / rationale FALSE |
| 3 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e '…"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | — | **`4`** (expected `0`): `tools/desk/cmd/repohardenguard/check.go:46` (ghRun), `tools/desk/cmd/deskroster/roster.go:229` (ghViewPR), `tools/desk/cmd/deskroster/roster.go:246` (ghListOpenPRs) — 3 REAL `gh` invocations — + 1 comment in `tools/desk/internal/forgeban/forgeban.go:173` | **FAIL** |
| 4 | `go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | 0 | all 5 subtests PASS; the frozen surface is **37 operations**, every one tabulated in inventory.md (reflection vs inventory + endpoint-param AST check + both-backends-export-nothing-extra + `ValidateRefPath` 17 refusal cases) | PASS |
| 5 | `go doc ./internal/deskkit Forge \| grep -cE -e 'Do\(' -e 'Raw\(' -e 'APIRequest\(' -e 'Call\('` | — | `0` — no arbitrary-request method in the interface godoc | PASS |

**Two halves, split outcome:**
- **No-passthrough half (rows 4/5): PASS, robustly.** `deskkit.Forge` is a genuinely closed enumerated interface (37 ops, all in inventory.md), no generic/arbitrary-endpoint escape hatch by four independent mechanisms; row 5 godoc grep = 0 is a second instrument. No escape hatch.
- **Shell-exec-ban half (rows 2/3): FAILS the table as written.** Row 3 (whole-tree grep, expect 0) returns 4 (3 real `gh` invocations in `deskroster`/`repohardenguard` + 1 comment). Row 2 passes only because `internal/forgeban` ALLOWLISTS them (ratchet `allowedInvocationCeiling = 9` + 19-entry could-not-check register) — contradicting row 2's rationale ("passes because no gh/glab invocation remains").

**Risk-bearing value:** `RISK-VALUE: DERIVED (structural) — no-passthrough is a shape guarantee (37 enumerated ops, 0 arbitrary-endpoint methods), not a numeric constant.` `RISK-VALUE: NAMED — allowedInvocationCeiling = 9 @ tools/desk/internal/forgeban/allowlist.go — this ratchet IS the shell-exec ban's current strength; the brief's model assumed ceiling = 0 (full closure), merged main ships a ratchet at 9 + a 19-entry could-not-check register. This is the load-bearing security constant for the ban half and it is 9, not 0.`

**Defense-in-depth (gate: model):** confirmed for both properties, and it is exactly what surfaced the FAIL — the ban's two independent instruments DISAGREE: row 2 (AST+literal source scan, allowlist-reconciled) passes while row 3 (naive grep, no allowlist) returns 4. Row 3 exists to expose precisely this. No-passthrough's two instruments (row 4 reflection + row 5 godoc) agree clean.

**Why a divergence, not a plain code bug:** the allowlist header + this house's CLAUDE.md record that routing the ambient-credential tools (deskroster/repohardenguard) through the seam is a token-custody decision deferred to separate briefs, and that some residuals need enumerated ops spec §6's freeze rule forbids adding speculatively. Shipping a ratchet instead of closure-to-zero is defensible — but the brief's Verify row 3 assumes full closure, so it does not pass as written.

**VERDICT: FAIL** on row 3 (row 2 passes literally but its DoD rationale is false); rows 1/4/5 PASS (the no-passthrough / enumerated-surface half is fully and robustly delivered). Held at `implemented` — a desk/human decision is owed (filed `#834`): amend Verify row 3 to reflect the accepted ratchet+custody-deferral, OR treat the residual `gh` callsites in `deskroster`/`repohardenguard` as open work (follow-up brief). Not flip-eligible against the table as written.
### Verify run — 2026-09-15, non-implementer dispatched verifier (sonnet-5-verifier, local) — VERDICT: PASS

Target: merged `origin/main` @ `0bf1166acb741f0ab27305d54ea92c9f2747514a`. Offline (`KUBECONFIG=/dev/null`)
in an isolated worktree; runner ≠ implementer. gate: model, risk all=no. Go rows run from
`tools/desk` (the sole module). Re-run following the 2026-09-10 FAIL (see the prior Evidence
entry above and a known open verifier finding, row 3) — this run finds the previously-cited gap
CLOSED, not reproduced; see "Divergence from the 2026-09-10 run" below.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | 0 | all packages `ok` on repeat runs; build clean. One transient `FAIL` was observed in `internal/loopengine` on a single `go test ./...` pass (a claim/liveness-recovery race, unrelated to any forge/`gh`/`glab` file this brief touches) — reproduced as flaky, not stable: 3/3 targeted re-runs of `./internal/loopengine/...` alone passed, and a second full `-count=1` full-suite run passed clean. Not chased per this brief's scope (out-of-scope package; C4 could-not-check would apply if it were in scope, but it is not) | PASS |
| 2 | `GOWORK=off go test ./... -run TestNoForgeCLIShellout -v` | 0 | `PASS`; log: "forge-CLI ban: 5 shipped call site(s) still invoke a forge CLI, across 5 registered declarations… ratchet ceiling 5; 19 exec site(s)… could-not-check" — ceiling down from 9 (2026-09-10) to 5 | PASS |
| 3 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e '…"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | 0 | **`0`** (2026-09-10 observed `4`: 3 real invocations in `deskroster`/`repohardenguard` + 1 comment). Cross-checked against the documented grep-shim false-clean risk: reran with the unaliased binary (`command grep`) — same `0` — and with a planted canary (`exec.Command("gh", "pr")`) inside a multi-file recursive directory tree processed through the same shim path, which the shim correctly reported. The `0` is real, not a shim false-clean | **PASS** |
| 4 | `go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | 0 | all 5 subtests PASS; frozen surface is now **45 operations** (up from 37 on 2026-09-10), all tabulated in `docs/streams/forge-gitlab/inventory.md` | PASS |
| 5 | `go doc ./internal/deskkit Forge \| grep -cE -e 'Do\(' -e 'Raw\(' -e 'APIRequest\(' -e 'Call\('` | — | `0` | PASS |

**Divergence from the 2026-09-10 run — the row-3 gap in the known finding appears closed, not
re-baselined.** The prior FAIL (Evidence entry above, filed as a known open verifier finding —
see the coordinator's dispatch note) cited 3 real `gh` invocations at `repohardenguard/check.go:46`
and `deskroster/roster.go:229,246`. On this merged main: `repohardenguard/check.go` now carries a
header comment reading "The forge-gitlab guard-read-custody brief moved the fetcher from a shelled
`gh api <endpoint>` onto the [Forge seam]"; `deskroster/roster.go`'s `ghViewPR`/`ghListOpenPRs` are
now doc-commented as reading "via the Forge seam" with no `exec.Command` call in either function.
`internal/forgeban/allowlist.go`'s own header confirms both migrations explicitly: "deskroster's
two READ rows… are also gone… REPO-HARDENING READS are no longer in that list either — the
forge-gitlab guard-read-custody brief gave them their own enumerated op (`RepoHardeningRead`, op
40)… the ceiling came down by one more." The ceiling dropped 9 → 5, and the remaining 5
`AllowedInvocations` rows (`deskadvisory/ghToken`, `deskdigest/runGH`, `deskdisposition/gh`,
`deskmerge/runGH`, `deskpushguard/fetchPR`) are all ambient-CLI-identity reads under documented
token-custody deferral (per the allowlist header), none of them a literal
`exec.Command("gh", …)`/`exec.Command("glab", …)` form — which is exactly why row 3's naive-grep
instrument (which only matches that literal form) now reads `0` while row 2's structural instrument
still (correctly) reports 5 real invocations under ratchet. The two instruments no longer disagree
about the SET being empty vs. non-empty in the way the 2026-09-10 divergence showed — they now
agree residual invocations exist (5, structurally detected) but disagree on WHICH TABLE ROW that
number satisfies: row 3 as literally written (naive grep, literal-argv form only) is `0` and PASSES;
row 2 (structural, ratchet-aware) also PASSES because those 5 are allowlisted.

This is NOT the same disagreement as 2026-09-10 (that one was "3 real invocations the grep catches
but the allowlisted test doesn't count"); it is now "0 invocations in the grep's literal-form
target, 5 invocations only a stricter structural check can see and which are formally ratcheted."
Per the brief's own text (spec §6 freeze rule; the allowlist's stated token-custody deferral for
`deskadvisory`/`deskdigest`/`deskdisposition`/`deskmerge`/`deskpushguard`), row 3 as literally
specified in the Verify table now passes on its own terms — this is not this verifier re-litigating
the ratchet-vs-closure-to-zero policy question the known finding raised; it is that the specific
callsites which made row 3 fail on 2026-09-10 (deskroster, repohardenguard) have since been migrated
by `forge-gitlab/11` and no longer exist in any form the naive grep — or any grep — can find.

**Risk-bearing value:**
`RISK-VALUE: NAMED — allowedInvocationCeiling = 5 @ tools/desk/internal/forgeban/allowlist.go:84`
— irreversibility rank 1 (this is the ratchet the ban's current strength is measured against; a
raise is a reviewable diff per the file's own header, a mismatch fails `TestForgeNoPassthrough`'s
sibling ratchet-consistency test). Derived from first principles: the value fell from 9 (2026-09-10)
by exactly 4 (the `repohardenguard` row + the 2 `deskroster` rows + one further row cited in the
allowlist header as "the ceiling came down by one more" for the same repo-hardening migration),
consistent with the 3 real grep-caught invocations closing plus one additional structurally-tracked
row for the same migration.
`RISK-VALUE: DERIVED (structural) — no-passthrough (rows 4/5) remains a shape guarantee, now over
45 enumerated operations (up from 37), 0 arbitrary-endpoint methods on either backend.`

**Defense-in-depth (gate: model):** confirmed for both properties. No-passthrough's two independent
instruments (row 4 reflection/AST-shape check + row 5 godoc grep) agree clean, as before. The
shell-exec ban's two instruments (row 2 structural/ratchet-aware, row 3 naive literal grep) now
BOTH pass — row 2 because the 5 residual invocations are formally allowlisted under a shrinking
ratchet with named token-custody blockers, row 3 because none of the 5 residuals take the literal
`exec.Command("gh"/"glab", …)` form the naive check targets. Both checks independently verified
against a false-clean risk this run: row 3's `0` was cross-checked against the documented
multi-file grep-shim false-clean hazard (canary test, see above) rather than trusted on a single
read.

**VERDICT: PASS.** All 5 Verify rows pass on merged main as of 2026-09-15
(`0bf1166acb741f0ab27305d54ea92c9f2747514a`). This differs from the 2026-09-10 FAIL and from this
dispatch's own working assumption (an open verifier finding, cited as still-open, predicted a
reproduced FAIL on row 3) — the underlying code changed since 2026-09-10 (forge-gitlab/11 landed
and migrated the specific callsites that caused the prior FAIL), not the Verify table. **This
verifier does not flip status** (no flip instruction was given for a PASS outcome, and a
status/board flip on a public-repo item is left to the coordinating desk); it lands this Evidence
and reports the PASS + the closed gap for a human/desk decision on: (a) flipping
`forge-gitlab/08` implemented→verified, and (b) whether the still-open finding this dispatch cited
as motivation can now be closed or needs re-scoping to the 5 remaining ratcheted rows specifically.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers the defense-in-depth questions: the string-level ban is the upper layer —
does the reflection/shape check independently catch a passthrough method that evades the
callsite grep (e.g. a raw request reachable through an exported helper)? And is the ban's
detection provably resistant to a re-introduced callsite in a spelling the test does not
match — i.e. does it enumerate `exec` forms structurally rather than by one literal pattern?
