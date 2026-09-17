# Reviewer write boundary — the claim store resolved in the tool layer, and duties that follow it

**Status:** draft
**Issue:** #1267 · **Stream:** [forge-neutral](README.md) · **Authored:** 2026-09-17 ·
**Base read:** `c67cc371` (origin/main)

No code lands against this document until its status is `approved`. Approval is a human act.
Every open design fork is a lettered question in [§10](#10-open-questions); the briefs that
implement this spec ([20–30](README.md#briefs)) build on the recommended defaults and say which
question they would change under.

## 1. Problem

Every desk role is required to hold the same three write grants. `requiredDuties` lists
`pull_requests`, `issues` and `contents` at write for all roles
(`tools/desk/internal/deskkit/preflight.go:782-786`), and the boot check `app-scopes-vs-duties`
refuses a role whose installation grant is missing one (`preflight.go:852-865`). The stated
reason for `contents: write` is *"land commits (Evidence, status)"* (`preflight.go:785`) —
verifier and coordinator work. The reviewer role lands no commit.

The reviewer has one use for repository write. A review dispatch takes the per-review dispatch
claim first (`tools/desk/cmd/deskdispatch/dispatch.go:228-241`), under the credential of the
dispatching role, which for the review kit is `reviewer` (`dispatch.go:876`,
`dispatch.go:1337-1342`; `tools/desk/internal/deskkit/modelstamp.go:627`). The claim is a git
ref on the forge, created by a compare-and-swap push
(`tools/desk/cmd/deskclaim-ref/main.go:17-23`, `claim.go:32`). Writing any ref needs repository
write.

So the identity that approves a change can also move the head it approves. The approval rule
"approved at the current head" assumes it cannot. Today that is a convention every session
honours, not a write boundary. It matters more as role keys move to per-role custody: a review
process then holds only the reviewer credential and cannot borrow another role's.

## 2. Direction

Recorded on #1267 (driver, 2026-09-17):

1. The reviewer's grant is narrowed. On GitHub this is achievable and is the target.
2. On GitLab Free, where the forge has no per-user branch protection, a reviewer that keeps
   repository write is admissible only with the tradeoff stated in the docs and at boot.
3. Which path a claim takes is resolved in the desk-tool layer, not in skill prose; a skill
   body reads the same whatever is resolved.
4. Spec first, then briefs. No code before the spec is approved.

**The step back this spec is built on.** The forge-ref claim exists to arbitrate dispatchers
on machines that share nothing but the forge. A cell normally shares more than that — a host,
a volume, or a desk service. Where it does, the claim does not need to live on the forge, and
then **no role needs repository write for claims, on any forge or plan tier.** The question
stops being "how does the reviewer's claim reach the forge" and becomes "where does this cell
keep its claims". This reframing reached the authoring session as a relay of the driver's
question; it is not yet recorded on #1267 ([§10 K](#10-open-questions)).

Out of scope: how verdicts, comments and the ready flip are posted.

## 3. What was read

### 3.1 In this tree (all read at `c67cc371`)

| # | Fact | Where |
|---|---|---|
| T1 | One duty list for every role; `contents: write` is in it; the duty set takes no role | `tools/desk/internal/deskkit/preflight.go:782-786`, `794-801`, `852-866` |
| T2 | On a GitLab-forge repo the grant check is not-applicable: a PAT records no per-mint permission listing | `preflight.go:802-832` |
| T3 | The review dispatch takes its claim as the `reviewer` role; every other dispatch takes it as `desk` | `tools/desk/cmd/deskdispatch/dispatch.go:872-918`, `1337-1342` |
| T4 | The claim tool already isolates its storage behind `claimStore` — seven methods, three-state results, one production implementation (forge ref over in-process git smart-HTTP) | `tools/desk/cmd/deskclaim-ref/claim.go:97-119`, `claim.go:132`, `gogit.go` |
| T5 | The claim tool's `show` / `list` output is a wire contract parsed by other tools | `deskclaim-ref/claim.go:34-38`; `deskdispatch/dispatch.go:965-1008` |
| T6 | A machine-local, lock-serialised, exclusive-create claim primitive already exists and is field-hardened, including its fail-closed rule (a lock that cannot be held or a file that cannot be read is exit 6, never "assume free") | `tools/desk/internal/deskkit/claim.go:1-25`, `147-190`, `217-323` |
| T7 | **Dispatch claims were machine-local once, and were moved to the forge on 2026-08-13 because two desks on different machines double-dispatched** | `tools/desk/cmd/deskclaim/main.go:30-35` |
| T8 | Claims are also READ outside the claim tool, directly from the forge: the supervisor's live enumeration, the verdict stamp's liveness read, the fan-out loop's release, the loop engine's local ref read; two skills tell the reader to `git ls-remote` the claim namespace | `tools/desk/cmd/desksupervise/live.go:35`, `tools/desk/cmd/deskpost/claimliveness.go:41-61`, `tools/desk/cmd/fanoutloop/land.go:81-154`, `tools/desk/internal/loopengine/writescope_io.go:31-65`, `plugins/assay/skills/pr-shepherd/SKILL.md:27-41`, `plugins/assay/skills/worker-desk/SKILL.md:140` |
| T9 | Two forge claim namespaces exist (`refs/dispatch/*` written by the dispatch claim tool; `refs/heads/dispatch/*` used by the Go readers and the release path); the divergence awaits a ruling. On GitLab a claim outside `refs/heads` cannot be released through any API — the problem [brief 05](brief-05-claim-layer-forge-shape.md) and [`claim-shape.md`](claim-shape.md) record | `deskclaim-ref/main.go:56-66`, `tools/desk/internal/deskkit/claimref.go:50-57` |
| T10 | The forge serving a repo is resolved in one place, never chosen by a caller; refusal is the only fallback | `tools/desk/internal/deskkit/forgeresolve.go:9-40` |
| T11 | A signed cross-desk message layer exists (envelope, short-TTL identity assertion, lane ACL); its gateway is config-off by default and its within-cell socket is loopback. Considered as the carrier for the served store and not used: the served store needs member-to-service reachability across hosts and no role-level assertion (§4.2) | `tools/desk/internal/comms/doc.go`, `tools/desk/cmd/commsgw/main.go:1-10` |
| T12 | The session roster is machine-local state (`<config home>/roster/<session>.json`) — it cannot show what another host is dispatching | `tools/desk/cmd/deskroster/main.go:1-10` |
| T13 | The pr-review-desk skill states the grant set inline | `plugins/assay/skills/pr-review-desk/SKILL.md:660` |

### 3.2 What the adopter docs say about topology — this decides the default store

| # | Statement | Where | Status |
|---|---|---|---|
| A1 | The documented way to run a cell is **one machine**: "how that cell runs on one machine: a persistent `deskd`, one window per desk role" | `docs/cellctl.md:3-6`, `docs/adopting-assay.md:1642-1643` | established |
| A2 | Two cells on one laptop "share nothing but the binaries" — a cell's state is one directory under the cells root | `docs/cellctl.md:21-41`, `48`, `54-55` | established |
| A3 | Container desks are one image per desk with a **per-desk** work volume; "Volumes are per-desk — two desks never share a writable working tree" | `docs/docker.md:152-165`, `containers/README.md:160` | established — so a shared claims volume does not exist today and would be a new mount |
| A4 | Compose and Kubernetes definitions for the five desks are planned, not shipped (one StatefulSet, one replica, one volume per desk) | `docs/streams/desk-containers/README.md:37-38`, `docs/streams/desk-containers/brief-06-k8s-manifests.md:32` | established (as plans) |
| A5 | A per-cell persistent service (`deskd`) is part of the documented cell, but its source is not in this tree (it is built from a separate console component) | `docs/cellctl.md:37-38`; `tools/cellctl/cellctl:1990` | established; what that service can host is **could-not-check** from here |
| A6 | Several uncoordinated machines dispatching one repo's queue is a case that **has occurred** and is the reason for the forge claim; one skill still describes the claim as what arbitrates "the cross-machine race" | T7; `plugins/assay/skills/the-desk/SKILL.md:163-167` | established |
| A7 | No adopter document states a supported-topology list (single box / pods / several machines) | searched `docs/adopting-assay.md`, `docs/cellctl.md`, `docs/docker.md`, `docs/how-assay-works.md`, `docs/distribution.md` | established by absence in the pages searched — the docs describe one machine and per-desk containers, and promise nothing about several machines |

Reading: the documented, supported shape is a single host (A1, A2). Containers are per-desk
volumes on what the docs present as one Docker host (A3). Multi-machine is real but
undocumented (A6, A7). That supports a **`file` store as the default for a fresh single-host
adopter**, with the forge ref kept for the multi-machine case — and it means the default must
never be applied silently to an install that might be the multi-machine case (§7).

### 3.3 Store behaviour

| # | Claim | Status | How checked |
|---|---|---|---|
| S1 | Exclusive create plus a directory lock gives mutual exclusion between processes on one host's local filesystem | established — in-tree | T6: the primitive and its race tests ship in this tree and gate the non-dispatch claim kinds today |
| S2 | The same guarantees hold on a network filesystem (NFS, SMB) or a cluster read-write-many volume | **could-not-check** | not measured; not asserted from memory. Brief 20 measures what it can; anything unmeasured stays unsupported for the `file` store |
| S3 | A Docker named volume mounted into several containers on one host behaves as S1 | **could-not-check** | not measured. Brief 20 |
| S4 | The tool can reliably detect that a directory is on a network filesystem, on every supported OS | **could-not-check** | not investigated. Brief 20; the guard in §6 does not depend on it |

### 3.4 Forge behaviour — relevant to the forge-ref store only

| # | Claim | Status | How checked |
|---|---|---|---|
| F1 | GitHub rulesets target branches and tags (and, separately, pushes) | established — documentation | GitHub Docs, *About rulesets*, *Creating rulesets for a repository*, read 2026-09-17 |
| F2 | *Restrict creations / updates / deletions* limit the operation on matching refs to actors with bypass permission | established — documentation | GitHub Docs, *Available rules for rulesets*, read 2026-09-17 |
| F3 | A GitHub App can be a ruleset bypass actor; targets take include and exclude `fnmatch` patterns | established — documentation | GitHub Docs, *Creating rulesets for a repository*, read 2026-09-17 |
| F4 | A ruleset cannot target a ref outside `refs/heads/` and `refs/tags/` | could-not-check | the pages read are silent; absence of a statement is not a statement. Brief 20 |
| F5 | With the restrict rules in force, a non-bypass App token is refused on every write route to a branch (git push, contents endpoint, update-branch), while a create under an excluded claim prefix succeeds | could-not-check | needs a fixture and two App credentials. Brief 20 measures; brief 26 pins it |
| F6 | Ruleset availability by plan and repository visibility | could-not-check | the pages read state the organisation-level plan condition only. Brief 26 dereferences it |
| F7 | GitHub offers no App permission narrower than `contents: write` for writing a ref in one namespace | could-not-check this session | carried from #1267 and `docs/adopting-assay.md:873-876`; not re-read. Brief 20 |
| F8 | Whether a base-repository token with `contents: write` can update a fork-hosted change head, which base rulesets do not govern | could-not-check | not read. Brief 20 |
| F9 | GitLab protected branches and tags accept wildcards; when several rules match, the most permissive applies | established — documentation | GitLab Docs, *Protected branches*, *Protected tags*, read 2026-09-17 |
| F10 | GitLab per-user / per-group push allow lists need Premium or Ultimate; Free offers role levels only | established — documentation and this repository's live read | same pages; `docs/adopting-assay-gitlab.md:67` (HTTP 400 on Free, 2026-09-02) |
| F11 | A GitLab wildcard protection stops a Developer *creating* a matching branch | could-not-check | the page read does not address creation. Brief 20 |
| F12 | A ref outside `refs/heads` / `refs/tags` can be pushed to a GitLab project | could-not-check | open since 2026-09-07 as L10 in [`claim-shape.md`](claim-shape.md). Brief 20 |
| F13 | A GitLab PAT with scope `api` and no `write_repository` (how the reviewer is provisioned — `docs/adopting-assay-gitlab.md:114`) can push over git HTTPS | could-not-check | the token pages read carry no scope-to-transport statement. Brief 20 |
| F14 | A GitLab member must hold Developer or higher to approve a merge request | could-not-check this session | consistent with `docs/adopting-assay-gitlab.md:114`, page not read. Brief 27 |

## 4. The claim store — one seam, three backends

The seam already exists (T4). It is lifted from the claim tool's `main` package into `deskkit`
as `ClaimStore`, unchanged: the same six verbs, the same holder encoding
(`dispatch-claim <id> owner=… state=… branch=…`), the same two TTLs (20 min claimed, 120 min
dispatched), the same exit codes (0 / 5 / 6), and byte-identical `show` / `list` output (T5).
A conformance suite runs every backend through one table of cases so the three cannot drift.

### 4.1 Filesystem store, direct — `file`

A directory under the cell's state root (or a mounted volume the cell's dispatching desks
share). Acquire is an exclusive create of the claim file; advance and steal are
compare-and-replace against the content hash last read; release is compare-and-delete. All
four run under the directory lock discipline T6 already ships, and inherit its invariant: a
lock that cannot be held or a file that cannot be read is exit 6.

**What it does not give.** It is invisible to any machine that does not share the directory.
Two machines each running their own `file` store for the same repo will both acquire the same item and
neither will know (T7 is this exact failure). It carries no guarantee on a network filesystem
(S2) and none has been measured on a shared container volume (S3). The guard is §6.

### 4.2 The same store, served — `service`

Not a second implementation. There is one claim-store logic over a directory and two ways to
reach it: **direct** (`file` — every member shares the directory or a mounted volume) and
**served** — a small serve mode in this repository's desk tools that exposes acquire /
progress / release / steal / show / list over HTTP in front of a `file` store on its own
disk. A fuller cell service elsewhere may implement the same API; the client does not care
which answers ([§10 D](#10-open-questions)).

Properties, stated because each one is load-bearing:

- **Member-initiated only.** Every exchange is a request from a member and a response to it.
  The service never calls a member; liveness is the existing TTLs, exactly as on the other
  stores. The requirement is therefore **one-way reachability** — every member can reach the
  service — not two-way.
- **Placement rule.** The service runs where the **least-reachable member can still reach
  it**. A laptop-hosted service is not reachable from cluster pods behind NAT; a cluster- or
  otherwise-hosted service is reachable from a laptop. A laptop-only cell — including its own
  local containers, through a mounted volume — needs no service at all: `file` covers it.
- **No forge credential, no minting.** The serve mode holds cell coordination state only. It
  adds no key-custody surface: it never reads an App key or a role token and cannot write to
  a forge.
- **Auth: a cell token.** A shared secret read from a 0600 file on both sides
  (`DESK_CLAIM_TOKEN_FILE`), presented as a bearer credential and compared in constant time.
  It never appears in a message, a log line or the tree. A request without it, or with the
  wrong one, is refused before any field is parsed.
- **Bind default: loopback.** The serve mode listens on loopback unless an address is
  configured. A non-loopback bind without TLS configured is refused at start, because the
  cell token would cross the network in clear.
- **Accept set.** Only the six verbs; only keys that pass the claim key grammar
  (`deskdispatch/dispatch.go:94-98`); only repos in the roster's allowed set. Each request
  writes one audit line: member-declared role, verb, key, outcome.
- **Unreachable → fail closed.** No dispatch. A connection failure, a timeout, a lost or
  malformed reply is `unverifiable` (exit 6) — never "acquired", never "free", and never a
  silent fall-back to another store.

**What it does not give.** It puts a service on the dispatch critical path. The cell token
authenticates membership of the cell, not the role: the owner recorded on a claim is what the
member says it is, as it is on every store today.

### 4.3 Forge-ref store — `forge-ref`

Today's behaviour, kept as an **explicit opt-in** for the one case the others cannot cover:
several uncoordinated machines dispatching one repo's queue with no shared volume and no
shared service.

**What it does not give.** It is the only store that needs repository write, so it is the
only store under which the reviewer's grant cannot simply be dropped — §8 applies to it and
only to it. It carries the GitLab release problem and the unresolved namespace split (T9).

## 5. Resolution

One function, `ResolveClaimStore(repo)`, called by `deskdispatch` step 1, by the claim tool,
and by every claim reader (T8). No command-line flag selects a store.

Inputs, proposed names in the existing style:

| Key | Where | Meaning |
|---|---|---|
| `ASSAY_CLAIM_STORE` | roster | `file` \| `service` \| `forge-ref`. One value per cell; an optional `<owner/repo>=<store>` form for a cell whose repos genuinely differ. Strictly parsed; an unknown value is a refusal |
| `ASSAY_CLAIM_DIR` | roster | the `file` store's directory. Default `<config home>/dispatch-claims` — under a cell launcher the config home is already the cell's own |
| `DESK_CLAIM_SERVICE` | exported by the cell launcher | the claim service base URL. Never committed |
| `DESK_CLAIM_TOKEN_FILE` | exported by the cell launcher | path of the 0600 file holding the cell token. The value is never in the environment, a message or the tree |
| `ASSAY_CLAIM_SINGLE_HOST` | roster | `yes` — the operator's declaration that this cell's repos are dispatched from this host only. Required by the guard in §6 |

Order:

1. `ASSAY_CLAIM_STORE` set → that store. Its own preconditions must hold (`file`: §6 guard;
   `service`: `DESK_CLAIM_SERVICE` and `DESK_CLAIM_TOKEN_FILE` set and the service answering; `forge-ref`: the acting role holds
   repository write) or the resolution **refuses** — it never moves on to another store.
2. Unset, first release: `forge-ref`, with a boot NOTICE naming the key and the scaffold
   default. This preserves every existing install's behaviour (§7).
3. Unset, after the release window ([§10 B](#10-open-questions)): refusal naming the key.

A fresh adopter never reaches step 2: the scaffold writes `ASSAY_CLAIM_STORE=file` and the
single-host declaration at cell creation ([§10 A](#10-open-questions)).

Every refusal is exit 6, happens before any worktree is cut, names the store, the missing
precondition and the remedy. A store that fails mid-operation is `unverifiable`; there is no
cross-store fallback anywhere.

## 6. Guards — one cell, one store

**Cross-host guard (`file` store).** The tool cannot see other hosts (T12). What it can do:

- require `ASSAY_CLAIM_SINGLE_HOST=yes` before a `file` store resolves — an operator who has
  not said "this is the only host" does not get a store that is only correct if it is;
- print, on every boot, that the declaration is **declared, not verified** — the tool names
  that it cannot tell ([§10 C](#10-open-questions));
- read the forge claim namespaces (a read; needs no repository write) and **refuse** if a
  live forge claim exists for the repo: that is positive evidence of another dispatcher
  using a different store.

**Mixed-store refusal.** The claims directory (served or not) carries a store marker (cell
name, store kind). A resolver that finds a marker disagreeing with the resolved store, a
`forge-ref` resolver that finds live `file` claims for the repo, or a `file` / `service`
resolver that finds live forge claims, refuses. Stores are never merged and never read in
union.

**Second layer.** None of this replaces the supervisor's staleness reclaim, which frees a
slot on elapsed time plus no live branch — a different signal in a different component — nor
branch-as-claim, which takes over once the worker's branch is pushed and is on the forge
under every store.

## 7. Migration — a fleet mid-transition must not double-dispatch

- Existing installs are on `forge-ref` and have no key set. Upgrading changes nothing (§5
  step 2).
- Moving a cell to `file` or `service` is a **drain, not a merge**: quiesce dispatch, let
  live claims release or expire, set the key on **every** dispatching process of the cell,
  resume. The mixed-store refusal is what makes a half-done change loud instead of silent.
- One cell, one store. Two cells sharing a repo is already outside the cell model (a cell is
  "accountable for its own repo set"); if it exists, both must be on `forge-ref` or share one
  `service`.
- Claim readers (T8) move onto the seam **before** any non-forge store can be selected
  (brief 22). Otherwise the supervisor and the verdict stamp would read an empty forge
  namespace and report every slot free.

## 8. Per-role duties, store-aware

`requiredDuties` becomes `dutiesFor(role, store)`.

| Role | `pull_requests` | `issues` | repository write | Why |
|---|---|---|---|---|
| desk, worker, verifier, issue-loop, intake-loop | write | write | write | they land commits or branches under every store |
| **reviewer**, store `file` or `service` | write | write | **read** | the reviewer writes no ref; the worktree fetch needs read |
| **reviewer**, store `forge-ref` | write | write | **write** | the claim is a ref the reviewer writes; bounded server-side (§9) |

- An unknown role keeps the full set. The direction is one way: store → duty → compare to
  grant. A missing grant never lowers a duty.
- The boot check's detail line names the store: `claim store: file (<dir>; single-host
  declared, not verified)`, `claim store: service (reachable)`,
  `claim store: forge-ref (repository write held; server-side bound: could-not-check — run
  the hardening audit)`.
- A reviewer holding **more** than its duty is a NOTICE, not a failure — the order of
  operations (§11) passes through that state on purpose.
- On GitLab the grant comparison stays not-applicable (T2); the store line and notices are
  still printed.

## 9. Server-side bound and the GitLab Free tradeoff — forge-ref store only

Under `file` and `service` this section does not apply: the reviewer holds no repository
write on any forge or tier.

**GitHub.** A dedicated ruleset pair (branches, tags), separate from default-branch protection
so its bypass list cannot weaken that one: target all, excluding the claim prefix when claims
live under `refs/heads/`; restrict creations, updates, deletions (F2); bypass = the writing
roles' Apps and human writers (F3); the reviewer App absent. Second layer: hardening-audit
rows (pair present, reviewer absent from bypass). Third: the ready flip refuses a change whose
head branch is inside the claim prefix. Not covered until measured: a fork-hosted head (F8).

**GitLab.** Ultimate: a custom role without push. Premium: wildcard protection of `*` branches
and tags with a per-account allow list (F9, F10), plus a more permissive rule for the claim
prefix. Free: role levels only — a level that refuses the reviewer refuses every
Developer-level role — so **the reviewer keeps repository write**.

**The Free tradeoff, in adopter language** (brief 29 lands it verbatim):

> **On GitLab Free with forge-stored claims, your reviewer account can also push to
> merge-request branches.** Assay keeps "who approves" and "who writes" as separate accounts.
> If your desks run on one machine, or share a claim service, use the file or service claim
> store: the reviewer account then needs no write access at all, on any GitLab tier. Only if
> several separate machines dispatch the same repository do claims have to live on GitLab
> itself, and then the reviewer account must be able to write them. GitLab Free has no
> per-account push rule, so in that one setup the separation rests on the reviewer being a
> separate account with its own credential, the desk tools never pushing as it, and every
> commit on a merge request naming the account that pushed it. The default branch stays
> protected. Premium (per-account protected branches) or Ultimate (a reviewer role with no
> push) lets GitLab refuse the push outright.

Surfaced in `docs/adopting-assay-gitlab.md` and as a boot **NOTICE** on the reviewer role
whenever the store is `forge-ref` and the declared GitLab profile is Free — every boot.

## 10. Open questions

Each has a recommended default. The briefs are authored on the defaults.

| # | Question | Options | Recommended default |
|---|---|---|---|
| **A** | Default store for a fresh single-host adopter | `file` / `service` / `forge-ref` | **`file`**, written by the scaffold together with the single-host declaration. It matches the documented topology (§3.2 A1–A2), needs no service and no repository write |
| **B** | Does `forge-ref` remain supported, or is it deprecated after a release window? | (1) supported indefinitely as an explicit opt-in; (2) deprecated once `service` has shipped for a window | **(1).** It is the only store for machines that share nothing but the forge, a case that has occurred (T7). What ends after one release window is the *implicit* default for an unset key, not the store |
| **C** | Cross-host guard when it cannot tell | (1) refuse `file` without the operator's single-host declaration, then NOTICE every boot that it is declared, not verified; (2) NOTICE only; (3) refuse always unless a probe proves single-host | **(1).** (2) reproduces T7 silently for anyone who copies a config; (3) is unimplementable (T12) |
| **D** | Does the serve mode ship in this repository's desk tools, or is it left to an external cell service? | (1) ship it here, minimal — six verbs over HTTP, cell token, loopback default, over the `file` store; (2) specify the API only and leave every implementation external | **(1).** Without an in-tree implementation the `service` store cannot be verified from this tree, and the per-cell service the docs mention is not built here (A5). An external service may still implement the same API |
| **E** | Local-store mechanics: replace-by-rename, or rewrite in place under the directory lock as the shipped primitive does? | rename / in-place | **in place under the lock** — reuse T6 rather than add a second discipline; the conformance suite pins the observable behaviour either way |
| **F** | For `forge-ref`, which namespace does the server-side bound assume? | settle the namespace first / bound for both | **bound for both** — an exclude for an unused prefix is inert, so this work does not wait on that ruling |
| **G** | Does the boot check fail, or NOTICE, when `forge-ref` is in force and the hardening audit has no fresh clean result? | fail / NOTICE | **NOTICE** for the first release; failing would stop every existing install at re-pin |
| **H** | Container desks on one host: a mounted claims volume (`file`) or the serve mode (`service`)? | volume / service | **volume, once S3 is measured clean** — a laptop-only cell should need no service (§4.2 placement rule). The volume holds claims only, never a working tree, so the per-desk working-volume property (A3) stands. Until S3 is measured, container cells use `service`. Pods on a cluster use `service` |
| **I** | Do roles other than the reviewer get narrowed duties here? | yes / no | **no** — the table is role-keyed so they can be later |
| **J** | How is the GitLab tier known for the Free NOTICE, given a PAT's reach is not observable offline (T2)? | a roster-declared profile / a live settings read (needs Maintainer) / a probe write | **roster-declared profile** — the other two need a wider grant, or a write made to learn whether writes work |
| **K** | The step back in §2 reached this spec as a relay. Is it recorded on #1267 by the driver before approval? | yes / no | **yes** — approval of this spec should cite it |

## 11. Order of operations

No review desk goes dark at any step: the tools accept the old grant and the old store
throughout, and the grant changes last.

1. **Spec approved** (human).
2. **Measure** (brief 20).
3. **Seam and resolver** (21) — `forge-ref` only, behaviour unchanged. **Readers onto the
   seam** (22). **Store-aware duties** (25) — a reviewer still holding write boots clean.
4. **File store and guards** (23); **service store** (24); **forge-ref bounds** (26, 27) —
   in parallel.
5. **Scaffold defaults** (28), then **docs and store-neutral skills** (29).
6. **Release**; adopters **re-pin**. Existing human-gated processes.
7. **Per cell: drain, set the store key, resume** (§7).
8. **Prove on a fixture** (brief 30): reviewer at repository read boots clean and dispatches
   under `file`; under `forge-ref` with the bound, a branch push is refused by the forge while
   a claim succeeds.
9. **The operator narrows the grant.** A human act, per installation, and the last step:
   reduce the reviewer to repository read (`file` / `service`), or apply the server-side
   bound (`forge-ref`). Re-mint fresh afterwards (`preflight.go:835-838`). No tool performs or
   prompts it.

Rollback at step 9 is restoring the grant; rollback at step 7 is the same drain in reverse.

## 12. Same seam later — listed, not designed here

Claims are the only state this spec moves. Other cell-scoped state also lives per machine
today and will meet the same question the moment a cell spans hosts. Listed so the seam is
built with them in view; none is designed here.

| State | Where it lives today | Where |
|---|---|---|
| Session roster (who is working what) | `<config home>/roster/<session>.json`, "runtime machine-local state" | `tools/desk/cmd/deskroster/main.go:7-8` |
| Stop flags — all loops, per loop, per dispatched run | files in the desk-tools state directory | `tools/desk/internal/deskkit/killswitch.go:17-22`, `68-75`, `113` |
| Outward-write budgets | derived from the machine's own audit log | `tools/desk/internal/deskkit/ratelimit.go:26`, `tools/desk/internal/deskkit/audit.go:295` |
| Worker-pool width | a store under the state directory | `tools/desk/internal/deskkit/widthstore.go:47` |
| Acknowledgement beacons | files under the state directory | `tools/desk/internal/deskkit/ackbeacon.go:43` |
| Non-dispatch claim kinds (route / file / close / verify) | `<config home>/claims`, machine-local by design | `tools/desk/cmd/deskclaim/main.go:26-35` |

## 13. Verify rows that prove the lower layer

| # | Row | Brief |
|---|---|---|
| V1 | Conformance: every backend passes one table — acquire, refuse-live, reclaim-stale, progress, release, steal, `show` / `list` byte parity | 21, 23, 24 |
| V2 | Two processes racing one key on a `file` store: exactly one acquires, the other exits 5 | 23 |
| V3 | `file` without the single-host declaration → refused; with it → NOTICE on every boot | 23, 25 |
| V4 | Mixed stores: a live forge claim present while resolving `file` → refused before any worktree; and the reverse | 23 |
| V5 | A reviewer credential with **no repository write** dispatches a review under `file` and under `service`; the supervisor and the verdict-stamp liveness read see that claim | 22, 23, 24 |
| V6 | Service down → dispatch exits 6 before any worktree; lost reply → unverifiable, no fallback | 24 |
| V7 | `forge-ref`, GitHub fixture, bound applied, reviewer credential, plain git: branch push **refused by the forge**; claim acquire **succeeds** | 26, re-run in 30 |
| V8 | `forge-ref` + GitLab profile Free → the NOTICE fires on every reviewer boot; absent under `file` | 25 |
| V9 | The boot check names the store; a reviewer at repository read is clean under `file` and refused under `forge-ref` | 25 |
| V10 | No skill body names a store, a claim namespace, or a per-store grant | 29 |

## 14. Documents that change

| Document | Change | Brief |
|---|---|---|
| `docs/adopting-assay.md` — App inventory, `setup-reviewer-app`, *The required duty set* | store-aware duty table; the reviewer's narrowed shape as the default for a single-host cell; the forge-ref bound; the retired-recommendation note rewritten | 29 |
| per-role App manifests (the permission sets the Apps installer stream's manifest flow emits — `docs/streams/apps-installer/design.md`) | reviewer manifest: repository read by default, write only for `forge-ref` | 29 (text) |
| `docs/adopting-assay-gitlab.md` | tier table and reviewer row per §9; the §9 text verbatim; the profile key | 29 |
| `docs/enforcement-model.md` — *The identity set* | "one list, identically" becomes store-aware; the write boundary described as a layer | 29 |
| `docs/cellctl.md`, `docs/docker.md`, `containers/README.md` | the claim store per cell kind; supported topologies stated for the first time (closes A7) | 28, 29 |
| `tools/desk/README.md` | claim tool, resolver, keys, preflight row | 21, 23, 24, 25 |
| `plugins/assay/skills/` — pr-review-desk, worker-desk, pr-shepherd, the-desk | remove grant lists, claim namespaces and `git ls-remote` claim reads; point at the claim tool's `show` / `list` and the boot check. Store-neutral | 29 |
| the public website's Apps and adoption pages | mirror the adopter guides; they live in the site's own repository and the companion change is tracked there | — |
