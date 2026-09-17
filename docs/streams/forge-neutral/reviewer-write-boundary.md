# Reviewer write boundary — the claim store resolved in the tool layer, and duties that follow it

**Status:** draft
**Issue:** #1267 · **Stream:** [forge-neutral](README.md) · **Authored:** 2026-09-17 ·
**Base read:** `c67cc371` (origin/main)

No code lands against this document until its status is `approved`. Approval is a human act.
Every design fork is a lettered question in [§10](#10-questions-and-rulings) — all but one now
ruled by the driver (2026-09-17, recorded on the pull request that carries this spec); the briefs that
implement this spec ([20–25, 28–31](README.md#briefs)) are written on the rulings recorded in §10.
Briefs 26 and 27 were withdrawn with the forge-ref store (§10 D); the numbering gap is kept.

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
keep its claims". The step back is recorded on #1267 (relayed there on 2026-09-17).

**Ruled since (2026-09-17, §10).** The forge-ref store is not kept as an opt-in: it is
**removed**, over one release window. Two stores remain — `file` for plain host processes and
`service` for anything in a container or pod, and for any cell that spans hosts. Point 2 of
the direction above therefore has no lasting case: once forge-ref is gone, no reviewer keeps
repository write on any forge or tier.

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
| A3 | Container desks are one image per desk with a **per-desk** work volume; "Volumes are per-desk — two desks never share a writable working tree" | `docs/docker.md:152-165`, `containers/README.md:160` | established — and ruled (§10 H): container desks do not share a claims volume; they use `service` |
| A4 | Compose and Kubernetes definitions for the five desks are planned, not shipped (one StatefulSet, one replica, one volume per desk) | `docs/streams/desk-containers/README.md:37-38`, `docs/streams/desk-containers/brief-06-k8s-manifests.md:32` | established (as plans) |
| A5 | A per-cell persistent service (`deskd`) is part of the documented cell, but its source is not in this tree (it is built from a separate console component) | `docs/cellctl.md:37-38`; `tools/cellctl/cellctl:1990` | established; what that service can host is **could-not-check** from here |
| A6 | Several uncoordinated machines dispatching one repo's queue is a case that **has occurred** and is the reason for the forge claim; one skill still describes the claim as what arbitrates "the cross-machine race" | T7; `plugins/assay/skills/the-desk/SKILL.md:163-167` | established |
| A7 | No adopter document states a supported-topology list (single box / pods / several machines) | searched `docs/adopting-assay.md`, `docs/cellctl.md`, `docs/docker.md`, `docs/how-assay-works.md`, `docs/distribution.md` | established by absence in the pages searched — the docs describe one machine and per-desk containers, and promise nothing about several machines |

Reading: the documented, supported shape is a single host (A1, A2). Containers are per-desk
volumes (A3). Multi-machine is real but undocumented (A6, A7). Hence the ruled defaults:
`file` for a fresh single-host adopter running plain host processes, `service` for anything
containerised and for any cell that spans hosts. A6 is also why the forge-ref store cannot
simply vanish in one release: an install that depends on it today has to be told, and given a
window (§7).

### 3.3 Store behaviour

| # | Claim | Status | How checked |
|---|---|---|---|
| S1 | Exclusive create plus a directory lock gives mutual exclusion between processes on one host's local filesystem | established — in-tree | T6: the primitive and its race tests ship in this tree and gate the non-dispatch claim kinds today |
| S2 | The same guarantees hold on a network filesystem (NFS, SMB) or a cluster read-write-many volume | **could-not-check** | not measured; not asserted from memory. It matters only if a host process points `file` at such a path, which §6 guards against. Brief 20 measures what it can reach; anything unmeasured stays unsupported |
| S4 | The tool can reliably detect that a directory is on a network filesystem, on every supported OS | **could-not-check** | not investigated. Brief 20; it decides how much of §6's filesystem guard can be a refusal rather than a notice |

(S3 — a Docker volume shared between containers — was withdrawn: containers use `service`,
§10 H, so the question no longer decides anything.)

### 3.4 Forge behaviour — no longer pursued

The first draft listed fourteen forge-behaviour claims (ruleset targeting, restrict rules,
bypass actors, GitLab wildcard protection and tier limits, custom-ref pushes), each marked
established or could-not-check, because a server-side bound was going to be built around a
reviewer that kept repository write under the forge-ref store. That store is being removed
(§10 D), so nothing is built on those claims and brief 20 no longer measures them. Two facts
from that list still inform §9's note and are restated there with their status. The open row
L10 in [`claim-shape.md`](claim-shape.md) stays open and is not this work's to close.

## 4. The claim store — one seam, two backends, one legacy store on its way out

The seam already exists (T4). It is lifted from the claim tool's `main` package into `deskkit`
as `ClaimStore`, unchanged: the same six verbs, the same holder encoding
(`dispatch-claim <id> owner=… state=… branch=…`), the same two TTLs (20 min claimed, 120 min
dispatched), the same exit codes (0 / 5 / 6), and byte-identical `show` / `list` output (T5).
A conformance suite runs every backend through one table of cases so they cannot drift.

**Which store a process uses is decided by where it runs** (§10 H): a plain host process may
use `file`; anything in a container or pod uses `service`, permanently.

### 4.1 Filesystem store, direct — `file`

A directory under the cell's state root, for plain host processes. Acquire is an exclusive
create of the claim file; advance and steal compare against the content hash last read and
rewrite in place; release is compare-and-delete. All run under the directory lock discipline
T6 already ships (§10 E), and inherit its invariant: a lock that cannot be held, or a file that
cannot be read or is torn, is `unverifiable` (exit 6) — never "free".

**What it does not give.** It is invisible to any machine that does not share the directory.
Two machines each running their own `file` store for the same repo will both acquire the same
item and neither will know (T7 is this exact failure). It carries no guarantee on a network
filesystem (S2). The guards are §6.

### 4.2 The same store, served — `service`

Not a second implementation. There is one claim-store logic over a directory and two ways to
reach it: **direct** (`file`) and **served** — a small serve mode in this repository's desk
tools (§10 D) that exposes acquire / progress / release / steal / show / list over HTTP in
front of a `file` store on its own disk. A fuller cell service elsewhere may implement the
same API; the client does not care which answers.

Properties, stated because each one is load-bearing:

- **Member-initiated only.** Every exchange is a request from a member and a response to it.
  The service never calls a member; liveness is the existing TTLs. The requirement is
  therefore **one-way reachability** — every member can reach the service — not two-way.
- **Placement rule.** The service runs where the **least-reachable member can still reach
  it**. A laptop-hosted service is not reachable from cluster pods behind NAT; a cluster- or
  otherwise-hosted service is reachable from a laptop. A cell of plain host processes on one
  machine needs no service at all: `file` covers it.
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

### 4.3 Forge-ref — the legacy store, removed after one release window

Today's behaviour. It is **not a supported choice** going forward (§10 D, B): it is the only
store that needs repository write, it carries the GitLab release problem and the unresolved
namespace split (T9), and the two stores above cover every topology the adopter docs describe.

Removal takes one release window (§7): release **N** ships `file` and the serve mode, and
keeps forge-ref reachable only as what an **unset** key resolves to, under a boot NOTICE that
names the removal release. Release **N+1** deletes the store.

**What is lost, stated plainly.** Several machines that share nothing but the forge, with no
host able to run a service the others can reach, have no supported store after N+1. The
placement rule in §4.2 is the answer offered to that topology: one reachable serve mode.

## 5. Resolution

One function, `ResolveClaimStore(repo)`, called by `deskdispatch` step 1, by the claim tool,
and by every claim reader (T8). No command-line flag selects a store.

| Key | Where | Meaning |
|---|---|---|
| `ASSAY_CLAIM_STORE` | roster | `file` \| `service`. One value per cell; an optional `<owner/repo>=<store>` form for a cell whose repos genuinely differ. Strictly parsed; an unknown value — `forge-ref` included — is a refusal that prints the two valid values |
| `ASSAY_CLAIM_DIR` | roster | the `file` store's directory. Default `<config home>/dispatch-claims` — under a cell launcher the config home is already the cell's own |
| `DESK_CLAIM_SERVICE` | exported by the cell launcher | the claim service base URL. Never committed |
| `DESK_CLAIM_TOKEN_FILE` | exported by the cell launcher | path of the 0600 file holding the cell token. The value is never in the environment, a message or the tree |
| `ASSAY_CLAIM_SINGLE_HOST` | roster | `yes` — the operator's declaration that this cell's repos are dispatched from this host only. Required before `file` resolves (§6) |

Order:

1. `ASSAY_CLAIM_STORE` set → that store. Its preconditions must hold (`file`: the §6 guards;
   `service`: both service keys set and the service answering) or the resolution
   **refuses** — it never moves on to another store.
2. Unset, **release N only** — the one-window legacy resolution: forge-ref, exactly as today,
   with a boot NOTICE on every dispatching role naming `ASSAY_CLAIM_STORE`, the two valid
   values, and the release in which the unset key stops resolving. `forge-ref` cannot be
   *selected*; it is only what silence means for one window.
3. Unset, **release N+1 onward** — refusal, printing the two valid values. The forge-ref code
   is gone.

A fresh adopter never reaches step 2: the scaffold writes the key at cell creation — `file`
plus the single-host declaration for a host cell, `service` for a container cell (§10 A, H).

Every refusal is exit 6, happens before any worktree is cut, and names the store, the missing
precondition and the remedy. A store that fails mid-operation is `unverifiable`; there is no
cross-store fallback anywhere.

## 6. Guards

**Cross-host guard (`file`).** The tool cannot see other hosts (T12). Ruled (§10 C):

- `file` does not resolve without `ASSAY_CLAIM_SINGLE_HOST=yes` — an operator who has not said
  "this is the only host" does not get a store that is only correct if it is;
- every boot prints that the declaration is **declared, not verified**.

**Container guard.** A process that detects it is running in a container or pod refuses
`file` and names `service` (§10 H). Where detection is not possible the rule still binds the
scaffolds and the docs; the tool says what it could not determine rather than guessing.

**Filesystem guard (`file`).** A host process might point `ASSAY_CLAIM_DIR` at a network
filesystem, where S2 is could-not-check. Proposed, and the one fork not yet ruled
([§10 L](#10-questions-and-rulings)): **refuse** when the directory is positively identified
as a network filesystem; print a **NOTICE on every boot** when the filesystem type cannot be
determined ("filesystem type not determined — the file store is supported on local disks
only"). Refusing on "cannot tell" would stop legitimate single-host cells on any OS where
detection is unimplemented (S4); staying silent would hide an unsupported setup.

**Mixed-store refusal — during the window.** While forge-ref still exists (release N), a cell
half-way through its drain must not double-dispatch. A `file` / `service` resolver that finds
a live forge claim for the repo refuses (a read; needs no repository write), and the legacy
resolution refuses if the default claims directory holds a live claim for the repo. The claims
directory, served or not, carries a store marker (cell name, store kind) and a disagreeing
marker refuses. Stores are never merged and never read in union. In release N+1 the
forge-side half of this check is deleted with the store.

**Second layer.** None of this replaces the supervisor's staleness reclaim, which frees a
slot on elapsed time plus no live branch — a different signal in a different component — nor
branch-as-claim, which takes over once the worker's branch is pushed and is on the forge
under every store.

## 7. Migration — two releases, and readers before writers

- **Readers first.** Claim readers (T8) are on the seam (brief 22) **before any writer
  switches store**. Otherwise the supervisor and the verdict stamp read an empty forge
  namespace and report every held slot free. This ordering is a dependency edge in the
  briefs, not advice.
- **Release N.** Ships the seam, the readers on it, `file`, the serve mode, store-aware
  duties, the scaffolds and the docs. Existing installs have no key set and keep today's
  behaviour under the removal NOTICE. Nothing forces a switch in N.
- **Per cell, during the window: a drain, not a merge.** Quiesce dispatch, let live claims
  release or expire, set the key on **every** dispatching process of the cell, resume. The
  mixed-store refusal makes a half-done change loud instead of silent.
- **Release N+1.** Deletes the forge-ref store and the forge-side mixed-store read; an unset
  key is a refusal printing the two valid values. An install that ignored the NOTICE for a
  whole window stops at boot with the remedy in front of it — it does not double-dispatch.
- One cell, one store. Two cells sharing a repo is already outside the cell model (a cell is
  "accountable for its own repo set"); if it exists, they share one `service`.

## 8. Per-role duties, store-aware

`requiredDuties` becomes `dutiesFor(role, store)`.

| Role | `pull_requests` | `issues` | repository write | Why |
|---|---|---|---|---|
| desk, worker, verifier, issue-loop, intake-loop | write | write | write | unchanged in this work; audited and narrowed afterwards (§12) |
| **reviewer**, store `file` or `service` | write | write | **read** | the reviewer writes no ref; the worktree fetch needs read |
| **reviewer**, legacy resolution (release N only) | write | write | **write** | the claim is still a ref the reviewer writes. This row is deleted in N+1 |

- An unknown role keeps the full set. The direction is one way: store → duty → compare to
  grant. A missing grant never lowers a duty.
- The boot check's detail line names the store: `claim store: file (<dir>; single-host
  declared, not verified)`, `claim store: service (reachable)`, or — release N only —
  `claim store: forge-ref (legacy; removed in <release> — set ASSAY_CLAIM_STORE)`.
- A reviewer holding **more** than its duty is a NOTICE, not a failure — the order of
  operations (§11) passes through that state on purpose.
- On GitLab the grant comparison stays not-applicable (T2); the store line and notices are
  still printed.

## 9. Forge-ref during the removal window — what an operator should know

Nothing new is built to harden a store that is being deleted. The first draft specified a
GitHub ruleset pair, a GitLab wildcard-protection shape and an adopter-facing GitLab Free
tradeoff text for a reviewer that kept repository write; all three are withdrawn with the
store (briefs 26 and 27; §10 F, G, J). For the one window in which the legacy resolution
still exists:

- **The reviewer still holds repository write until the cell switches.** The identity that
  approves a change can still move the head it approves; that stays a convention, exactly as
  it is today, until the key is set. The boot NOTICE says so on every boot.
- **Switching is the fix, on every forge and tier.** After the drain, the reviewer needs
  read only. There is no tier-dependent case left: the GitLab Free limitation (no per-account
  push rule — established, `docs/adopting-assay-gitlab.md:67`) only ever mattered while the
  reviewer had to write a ref.
- **An operator who wants a forge-side bound for the window can already express one** with
  the forge's own branch rules (GitHub rulesets restricting creations, updates and deletions
  to bypass actors — established from documentation read 2026-09-17; whether they cover every
  write route was never measured and is **could-not-check**). This spec ships no template and
  no audit row for it.
- **Narrow last.** Do not reduce the reviewer's grant while the cell still resolves to the
  legacy store: the boot check refuses, naming both remedies.

## 10. Questions and rulings

Rulings are the driver's, relayed by the desk and recorded on the pull request that carries
this spec, all dated **2026-09-17**. None of them approves the spec.

| # | Question | Ruling |
|---|---|---|
| **A** | Default store for a fresh single-host adopter | **RULED: `file`**, written by the scaffold together with the single-host declaration |
| **B** | Does forge-ref remain supported? | **RULED: no — removed**, over one release window (with D) |
| **C** | Cross-host guard when it cannot tell | **RULED:** refuse `file` without the explicit declaration, then NOTICE on every boot "declared, not verified" |
| **D** | Does the serve mode ship in this repository's desk tools? | **RULED: yes, minimal — and the forge-ref store is removed**, not kept as an opt-in |
| **E** | File-store write mechanics | **RULED:** rewrite in place under the existing directory lock; a torn file reads as unverifiable, fail closed |
| **F** | Which forge-ref namespace the server-side bound assumes | **MOOT** — there is no server-side bound; forge-ref is removed (D) |
| **G** | Fail or NOTICE when the forge-ref bound is unaudited | **MOOT** — same reason |
| **H** | Container desks on one host: claims volume or `service`? | **RULED: `service` for anything in a container or pod, permanently**; `file` is for plain host processes only. Departs from the first draft's default; the shared-volume measurement is dropped |
| **I** | Do roles other than the reviewer get narrowed duties? | **RULED: yes — every role is audited and narrowed, as a follow-on wave after the reviewer change is live** (§12). Briefs 20–30 keep reviewer-only scope. Departs from the first draft's default |
| **J** | How the GitLab tier is known for the Free notice | **MOOT** — there is no Free notice; the tradeoff disappears with forge-ref (§9) |
| **K** | Is the step back in §2 recorded on #1267? | **SATISFIED** — recorded there as a desk relay, 2026-09-17 |
| **—** | Removal schedule (asked after D) | **RULED: one release window.** N ships `file` + serve mode, unset key → forge-ref under a NOTICE naming the removal release; N+1 deletes the store and refuses an unset key |
| **L** | **OPEN.** `file` pointed at a network filesystem by a host process | Recommended default: **refuse** when positively identified as a network filesystem; **NOTICE every boot** when the type cannot be determined (§6). Brief 23 is written on this default |

## 11. Order of operations

No review desk goes dark at any step: the tools accept the old grant and the legacy store
through release N, and the grant changes last.

1. **Spec approved** (human).
2. **Measure** (brief 20).
3. **Seam and resolver** (21) — legacy resolution only, behaviour unchanged. Then **readers
   onto the seam** (22) — before any writer can switch — and **store-aware duties** (25).
4. **File store and guards** (23); then the **served store** (24).
5. **Scaffold defaults** (28), then **docs and store-neutral skills** (29).
6. **Release N**; adopters **re-pin**. Existing human-gated processes.
7. **Per cell: drain, set the store key, resume** (§7).
8. **Prove on a fixture** (brief 30): a reviewer at repository read boots clean and
   dispatches under `file` and under `service`; with the same credential a branch push is
   refused by the forge for lack of write access.
9. **The operator narrows the grant.** A human act, per installation: reduce the reviewer to
   repository read. Re-mint fresh afterwards (`preflight.go:835-838`). No tool performs or
   prompts it.
10. **Release N+1** removes forge-ref (brief 30's second half) once the window has run.

Rollback during the window is restoring the grant and reversing the drain. After N+1 there is
no forge-ref to return to; rollback is re-pinning N.

## 12. After cutover — the other roles

Ruled (§10 I): every remaining role — desk, worker, verifier, issue-loop, intake-loop — is
audited for the repository writes it actually performs and narrowed to them. This is a
**follow-on wave behind brief 30**, so the reviewer change does not wait on it.

- One measurement brief, [31](brief-31-remaining-roles-write-audit.md), inventories each
  role's real forge writes with the same method brief 20 uses for the reviewer.
- The per-role duties briefs are **authored from its findings, later**. None is authored now:
  what each role can drop is not known until 31 has run, and a duties brief written on a
  guess is the failure brief 20 exists to prevent.
- The duty table in §8 is already role-keyed, so narrowing a role is a table row plus its
  tests, not a redesign.

## 13. Same seam later — listed, not designed here

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

## 14. Verify rows that prove the lower layer

| # | Row | Brief |
|---|---|---|
| V1 | Conformance: every backend passes one table — acquire, refuse-live, reclaim-stale, progress, release, steal, `show` / `list` byte parity | 21, 23, 24 |
| V2 | Two processes racing one key on a `file` store: exactly one acquires, the other exits 5 | 23 |
| V3 | `file` without the single-host declaration → refused; with it → NOTICE on every boot | 23, 25 |
| V4 | During the window: a live forge claim present while resolving `file` → refused before any worktree; and the reverse | 23 |
| V5 | A reviewer credential with **no repository write** dispatches a review under `file` and under `service`; the supervisor and the verdict-stamp liveness read see that claim | 22, 23, 24 |
| V6 | Service down → dispatch exits 6 before any worktree; lost reply → unverifiable, no fallback | 24 |
| V7 | `file` inside a container → refused naming `service`; `file` on a positively identified network filesystem → refused; undetermined filesystem → NOTICE every boot | 23 |
| V8 | Release N: unset key → legacy resolution plus the removal NOTICE on every boot, and `forge-ref` as an explicit value is refused. Release N+1: unset key → refused printing the two valid values, and no forge-ref code remains | 21, 30 |
| V9 | The boot check names the store; a reviewer at repository read is clean under `file` / `service` and refused under the legacy resolution | 25 |
| V10 | No skill body names a store, a claim namespace, or a per-store grant | 29 |
| V11 | With the narrowed reviewer credential and plain git, a branch push is **refused by the forge** for lack of write access | 30 |

## 15. Documents that change

| Document | Change | Brief |
|---|---|---|
| `docs/adopting-assay.md` — App inventory, `setup-reviewer-app`, *The required duty set* | store-aware duty table; the reviewer at repository read as the target shape; the removal window and its NOTICE; the retired-recommendation note rewritten | 29 |
| per-role App manifests (the permission sets the Apps installer stream's manifest flow emits — `docs/streams/apps-installer/design.md`) | reviewer manifest: repository read | 29 (text) |
| `docs/adopting-assay-gitlab.md` | reviewer row: no repository write needed once the store key is set, on every tier. **No Free-tier tradeoff text is added** — the tradeoff is removed, not documented (§9) | 29 |
| `docs/enforcement-model.md` — *The identity set* | "one list, identically" becomes store-aware; the write boundary described as a layer | 29 |
| `docs/cellctl.md`, `docs/docker.md`, `containers/README.md` | the claim store per cell kind — host processes `file`, containers and pods `service`; supported topologies stated for the first time (closes A7) | 28, 29 |
| `docs/UPGRADING.txt`, release notes for N and N+1 | the removal window, the drain, the refusal in N+1 | 30 |
| `tools/desk/README.md` | claim tool, resolver, keys, preflight row | 21, 23, 24, 25 |
| `plugins/assay/skills/` — pr-review-desk, worker-desk, pr-shepherd, the-desk | remove grant lists, claim namespaces and `git ls-remote` claim reads; point at the claim tool's `show` / `list` and the boot check. Store-neutral | 29 |
| the public website's Apps and adoption pages | mirror the adopter guides; they live in the site's own repository and the companion change is tracked there | — |

## 16. A note on the stream README's brief rows

The briefs table in the stream README is generated. When these briefs were first added, the
tree's brief generator could not produce this stream's brief shape (#1280 tracks the defect),
so the first eleven rows were added by hand in the exact shape the generator prints. They have
since been replaced by tool output: the table as committed is what
`statusgen regen --readmes --root .`, built from this tree, writes from the briefs'
frontmatter.
