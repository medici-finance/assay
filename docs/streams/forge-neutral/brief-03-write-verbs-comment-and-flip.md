---
brief: forge-neutral/03
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
schema: brief-v1
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
  - "tools/desk/internal/forgeban/allowlist.go: fixed-here (seven rows removed, ceiling lowered to 17)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: out-of-scope (the skill names the verbs, not their transport; no skill text changes when a verb's backend does)"
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
5. Lower `allowedInvocationCeiling` from 24 to **17** and remove the seven retired rows.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && git stash list >/dev/null; go test ./cmd/deskpost/... ./cmd/deskreply/... ./cmd/deskflip/... -count=1` | exit 0 — all three suites green |
| 3 | `git diff origin/main -- tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskflip -- '*_test.go'` | **no assertion is removed or loosened** (this row forbids weakening coverage, NOT editing the test files). Re-pointing a test's transport during this migration is EXPRESSLY ALLOWED — editing or adding a verb's `_test.go` is expected. The diff must read as a **1:1 map**: each retired `gh`-argv assertion has a **named successor assertion on the HTTP-transport recorder** with the same request shape (method, path, body fields, auth-header presence). A diff that DROPS an argv assertion with no named successor, or WEAKENS one (fewer fields checked, a dropped read-count counter), FAILS this row. Reviewer confirms the map is complete and reviewable. |
| 4 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows `= 16` — the MEASURED ceiling from row 5, not a frozen literal |
| 5 | **Measure the ceiling, do not hard-code it.** The measuring command counts the `gh` launch sites on main at the amendment sha: `git fetch origin && grep -c '::gh"' <(git show refs/remotes/origin/main:tools/desk/internal/forgeban/allowlist.go)` (**24** at the amendment sha). The ceiling is that count **− 8** (the seven deskflip rows + the one deskreply row this brief retires) = **16 today**. Then `cd tools/desk && go test ./internal/forgeban/... -count=1 -v` | exit 0; the ratchet test passes at the MEASURED ceiling — it fails if the register is longer OR shorter than the ceiling. **Record the measuring command and its count in the Evidence row**, so a future launch-site drift is a re-measure, not a brief rewrite. |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 — no `gh`/`glab` launch remains in these three verbs, and the label op added no passthrough |
| 7 | `grep -rn -e '"gh"' tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskflip --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — independent cross-check of row 6 by a different instrument |
| 8 | `grep -rn -e 'apiBaseURL' tools/desk/cmd/deskpost --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — `apiBaseURL` is absent from **non-test files** under `cmd/deskpost` (the hardcoded host binding is gone, not merely unused). Scope is NON-TEST files only: `harness_test.go`'s `apiBaseURL` assignment MOVES WITH THE HARNESS (a test file — allowed, and required for the deskpost suite to compile). |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestLabelOpBothBackends -count=1 -v` | exit 0; the new label op runs against both recorded backends' fixtures with the same scenario names |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestFlipRefusesUnsupportedForge -count=1 -v` | **negative path**: with the resolver returning a forge whose backend cannot serve the ready flip, `deskflip` exits could-not-check (class 6) naming forge and operation, and performs NO write — asserted by a recording transport that must show zero write calls |
| 11 | `cd tools/desk && go test ./cmd/deskflip/... -run TestNodeIDNotConstructed -count=1 -v` | **negative path**: the flip path obtains its node id from `GetPullRequest` and refuses a locally-composed one, so a GitLab synthetic id cannot be forged by string-building. This new-transport test lives under `cmd/deskflip/`, beside the suite it extends; **row 3's no-weakening rule does NOT cover it** — adding a test that exercises the new transport is expected, not a frozen-file violation. |
| 12 | `grep -c 'label' docs/streams/forge-gitlab/inventory.md` | ≥ 1 — the new op is recorded in the frozen inventory with its consuming verbs |
| 13 | `statusgen --root . --consumers --brief forge-neutral/03` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

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
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — this brief changes
transport and adds one enumerated operation, it does not change who may write or what is
believed). Reviewer records verdict + date in the stream README table.
