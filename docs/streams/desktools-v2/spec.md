# desktools-v2 — scoping document

**Status:** approved — ruled 2026-09-21 on #1319.
**Routes-to:** docs/streams/desktools-v2/

Per `spec/lifecycle-v1.md` §8.2 a
`draft` document is the plan of record for nothing and no downstream control watches it. The
stream that cites it (`docs/streams/desktools-v2/README.md`) is therefore `status: parked`:
its briefs are authored and kept, but shelved out of Next-up and never dispatched until a
human rules this document `approved` (§8.4 `draft → approved` rides the ruling PR) and flips
the stream `active`. `approved` is the human's call, not this session's.

> **Why `draft`, not the word "proposed".** `spec/lifecycle-v1.md` §8.1 defines the
> machine-readable header states as exactly `draft | approved | routed`. A `**Status:**`
> line whose first token is none of those leaves the document *unclassified* (legacy) and a
> conforming detector ignores it. `draft` is the methodology's token for "a working proposal,
> not yet ruled" — the thing "proposed" means in prose — so it is used here rather than an
> unclassified literal. Nothing is faked `approved`.

## 1. The problem — the forge abstraction leaks

The desk tools were written ad-hoc, one verb at a time, while the desk flows were still
changing week to week. That was the right call then: a tool built to a flow that is about to
change should not be over-architected. The flows have since solidified, and the ad-hoc
origin now shows as a recurring, single-shape bug: **GitHub-specific facts, and ambient
credentials, are reached PAST the forge seam, and each such reach generates its own defect.**

There already is a seam. `tools/desk/internal/deskkit/forge.go` declares one `Forge`
interface — the ~four-dozen operations a shipping desk tool consumes — and two complete
backends implement it: `forge_github.go` (seated on the official `go-gh` REST/GraphQL
client, handed an already-minted installation token, refusing an empty one) and
`forge_gitlab.go`. The interface is deliberately FROZEN at the operations a shipping tool
consumes, with token minting and budget/secret/rate wrapping held OUTSIDE it.

**The desk verbs are mostly on the seam already** (desk audit, 2026-09-16, `origin/main`):
`deskflip`, `deskreply`, `deskboard`, `deskpr`, `deskfile`, `deskpost` and `deskclose`'s
reads route through `forge_github.go`, and `forgeban` reddens the build on a new `gh`/`glab`
shell-out. What remains is not one bug but three shapes, each its own filed issue or audited
site:

| Issue / site | The reach-past-the-seam |
|---|---|
| #1145 | `cellctl gen_shims`' `HOME` override hides `gh`'s ambient credential from any shimmed verb that shells `gh`. |
| #1146 | `deskdispatch` attaches the token only to a child literally named `gh`, not to a script that itself shells `gh`. |
| #628 | an inherited `GH_TOKEN` forces ONE installation across every scan repo — the scanloop-in-container break. |
| #1019 | `deskclose` hardcodes the `pullRequest` GraphQL query shape and so cannot target an issue. |
| #1201 | `deskpushguard` hardcodes the remote name `"origin"`. |
| #884 | the push-transport guard does not expand `insteadOf` / `pushInsteadOf`, so a rewritten URL evades it. |
| **statusgen** | **the largest remaining `gh` dependency: a SEPARATE Go module shelling `gh` directly** — 26 `exec.Command("gh", …)` sites across 15 non-test files (re-counted 2026-09-17 on `origin/main`; among them `issues.go:577`, `autoflip.go:535,:615,:656,:666`, `autonomy.go:451,:479`, `scanissues.go:114`) — and NOT under the `forgeban` control at all. Its migration is **already owned and in progress elsewhere**: `forge-neutral/18` routes each read through the `deskread` verb (`statusgen/forgeread.go`). v2 does not redo that work; it brings `statusgen/**` under the ban so the zero `forge-neutral/18` reaches cannot regress (§2 Principle 2). |
| #305 / #834 / #838 | the forge-CLI shell-exec ban is a permit-register RATCHET (`tools/desk/internal/forgeban/allowlist.go`, `const allowedInvocationCeiling` — **now 5**, down from 24), not a closure-to-zero. |
| #1223 | the read path still shells `gh` where it could run a native installation-token client (the pilot; see §4). |

The common thread is one sentence: **a `Forge` exists, but reaching past it — with a
GitHub-specific fact, or with an ambient credential — is possible, so it keeps happening.**
v2's thesis is to make each such reach *structurally impossible*, not to fence them one at a
time. The three design principles in §2 are how.

## 2. Design principles (first-class — every brief inherits these)

### Principle 1 — CUSTODY is the foundation

Identity, not transport, is what v2 is really about. Three claims, stated so no brief has to
re-derive them:

- **(a) Explicit minted-token custody only — never an ambient CLI credential.** An action
  runs under the identity whose App key the *environment* holds. A verb is handed an
  explicitly-minted installation token and REFUSES an empty/unminted one; it never resolves
  whatever credential happens to be active (the `gh` keyring, an inherited `GH_TOKEN`). The
  App/PAT permission set is a ceiling; the minted-token handoff is the control.
- **(b) The re-minted credential in-environment is the App PRIVATE KEY (PEM) + installation
  id — not the short-lived token. So KEY PRESENCE IS THE CUSTODY BOUNDARY.** What an
  environment can *become* is decided by which minting key (PEM) it holds, because the
  short-lived token is derived on demand from that key. Custody is therefore a property of
  key provisioning, not of token handling: an environment with no role's PEM can mint
  nothing, whatever tokens drift through it.
- **(c) The safety property is "each environment holds exactly one role's minting key and
  nothing else."** This is a provisioning discipline. The desk containers approximate it
  already. The **desktop is the MOST confused environment, not a benign one**: it holds a
  human's ambient CLI token *plus* possibly several role PEMs at once, so ambient-fallback
  there silently runs as whoever was last active. The goal is to make the desktop **behave
  like a locked container** — explicit-only, no ambient fallback — so the same custody
  invariant holds whether a verb runs in a container or on the desktop.

The two credential-custody briefs cite this framing: `desktools-v2/03` (the native read
client's custody contract) and `desktools-v2/06` (installation-token scoping); the statusgen
custody proof in `desktools-v2/08` inherits it too.

### Principle 2 — the READ PATH covers statusgen, across the `deskread` verb boundary

"Retire `gh` in the read path" (#1223) is NOT done when the desk verbs are done. statusgen
shells `gh` directly across its scan, auto-flip, autonomy and corroboration reads (§1 table).
It is the `scanloop`-blind root (#628): `scanloop` invokes `statusgen --scan-issues`, which
shells `gh`, so a container with no ambient `gh` credential reads an empty queue and goes
blind.

**How statusgen reaches the seam is already decided, and v2 adopts that decision rather than
reopening it.** statusgen is its own Go module and does not import `deskkit`; the header of
`statusgen/forgeread.go` records that this "is deliberate and it stays: the seam is reached by
RUNNING the desk-tools read verb (`deskread`) and parsing its JSON, not by linking a package."
That is the driver's direction of 2026-09-14 as recorded in `forge-neutral/18` ("desk-tools
owns the seam"), and it is landed code, not a proposal: `tools/desk/cmd/deskread/` exists,
`forgeReader` defaults to an offline three-state reader, and the first statusgen read has
already moved onto it. `deskread` authenticates as the session's minted App role through the
`deskkit` resolver and has no ambient-credential fallback, so a read that moves onto it
inherits Principle 1's custody posture with no work in statusgen itself.

An earlier draft of this document proposed the opposite — promote `deskkit` to an importable
shared library and have statusgen link it. **That proposal is withdrawn.** It contradicted a
recorded direction and the tree, and it duplicated `forge-neutral/18`, which owns the
statusgen migration end to end (its completion test is zero forge-CLI sites in `statusgen/`).
The two briefs that carried it are gone: `desktools-v2/07` is withdrawn (its number is not
reused) and `desktools-v2/08` is re-scoped to the part no other brief owns.

What v2 contributes to the statusgen read path is therefore enforcement and proof, not
migration:

- **The ban covers `statusgen/**`** (`desktools-v2/02`). statusgen has never had a permit row
  because `forgeban` counts desk-tools call sites only; the v2 counter includes it, so
  `forge-neutral/18`'s progress is visible as a falling count.
- **The zero is held** (`desktools-v2/08`). Once `forge-neutral/18` reaches zero, the
  statusgen half of the counter flips from advisory to failing, and a container-shaped proof
  — no `gh` on `PATH`, no ambient credential, only the role's minting key — shows the scan
  reads a real queue or reports could-not-check, never an empty one. `forge-neutral/18` is
  `gate: model` with every risk answer `no` and does not carry that proof.

### Principle 3 — PURPOSE-BUILT QUERIES (typed access-pattern operations)

The v2 Forge library exposes typed **access-pattern** operations — e.g. *review-queue
snapshot*, *head-sha for these PRs*, *board sweep read* — each backend implementing the
operation with ONE tuned query (a GitHub GraphQL document, a GitLab equivalent) rather than a
generic per-item call. Three reasons, each a class of pain v2 attacks:

- **(a) N+1 → one round-trip** — latency: a board sweep that made one call per PR makes one.
- **(b) Fewer calls = rate-limit headroom** — this attacks the recurring
  secondary-rate-limit blindness class (a desk that trips the secondary limit reads empty and
  cannot tell empty from blind).
- **(c) One GraphQL read is a single CONSISTENT snapshot** — which directly serves the
  fresh-views / freshness property: N sequential REST calls can straddle a main-advance and
  *tear* (half the reads see the old head, half the new), where one snapshot cannot.

Two cautions baked into the brief, not left to the implementer:

- **GitHub GraphQL has a query-cost / point budget.** Access-pattern queries are tuned to
  MINIMAL cost, not maximal fetch, and each is MEASURED — calls and points before/after, one
  variable at a time.
- **Raw queries never cross the `Forge` interface.** Callers see typed operations and typed
  results; the GraphQL document lives inside the backend, so a GitHub-specific query cannot
  leak past the seam (the same rule Principle-2 enforcement — the ban-lint — checks).

This layer is `desktools-v2/09`.

## 3. What v2 is (and is not)

**v2 is the enforcement-and-native-client layer that makes reaching past the seam impossible,
holds statusgen at zero once its sibling migration lands, migrates the unrouted leak sites,
exposes purpose-built access-pattern queries, and puts one outbound-write check at the write
seam (§8).** It is not a second fork of the tools
and not a rewrite of the two backends — both are complete and stay.

Six architectural commitments (the three principles, the two mechanics they need, and the
outbound-write check of §8):

1. **ONE seam, and only the two backends may speak GitHub/GitLab.** Every transport, identity
   handoff, query construction, remote name, and subprocess name passes through
   `deskkit.Forge`. (Principle 3's "no raw query crosses the interface" is the query half of
   this.)
2. **A ban-lint that makes the reach-past a red build, not a code-review catch** — and that
   covers statusgen, which is not under `forgeban` today.
3. **The cross-module boundary is a VERB, not a package.** A second module reaches the seam
   by running `deskread` and parsing its versioned envelope (Principle 2); `deskkit` stays
   internal. v2 enforces that boundary, it does not replace it.
4. **Custody-first native clients** (Principle 1): explicit minted-token, key-presence
   boundary, refuse-ambient — on the desk read path and on statusgen.
5. **Incremental, tool-by-tool migration, with the old path REMOVED as each lands.**
6. **One outbound-write check, at the one place every write already passes** (§8): what a
   deployment may write to a forge is enforced by the tools, keyed on the target's visibility,
   not remembered per verb or carried as prose in a skill.

## 4. Boundary with the sibling streams — no duplication

- **`forge-neutral`** (active) — makes the desk verbs the only sanctioned forge *write* path
  on GitHub *and* GitLab, adds the resolver/custody that decides *which* forge and *which*
  identity performs a write (`forge-neutral/01`), **and owns statusgen's forge path as well**:
  `forge-neutral/07` (acting identity, implemented), `forge-neutral/08` (forge-aware reads,
  implemented) and `forge-neutral/18` (statusgen off `gh` through the `deskread` verb,
  in progress). **v2 depends on its resolver and on `forge-neutral/18`'s migration and
  re-implements neither.** An earlier draft of this section said forge-neutral owned only the
  write path and that no v2 brief duplicated a sibling brief; both statements were wrong —
  the draft's statusgen-migration brief duplicated `forge-neutral/18` under an incompatible
  design. The overlap is resolved in forge-neutral's favour: it keeps the migration, and v2's
  statusgen work shrinks to the ban coverage and the held zero (`desktools-v2/02`, `/08`).
  v2's own contribution is the *ban* (extended to statusgen), the *custody contract* on the
  native read client, the *access-pattern query layer*, and the *outbound-write check*.
- **`desktools-go-git`** (active) — moves the desk tools' *git* operations off the `git`
  binary onto `go-git`. The push-guard remote-name / `insteadOf` gaps (#1201, #884) sit at the
  edge of its territory; v2 owns them from the *forge-assumption* angle and coordinates the
  transport half.
- **`desk-tools`** (active) — the general planning board for the current `tools/desk/` suite.
  v2 is the *architectural* successor for the forge-abstraction slice specifically.

**Litmus for "belongs to v2":** the work makes reaching past the `Forge` seam impossible (the
ban-lint, including its `statusgen/**` coverage), codifies custody on a desk-side native read,
migrates a named forge-assumption leak site, adds a purpose-built access-pattern query, or
belongs to the single outbound-write check. Migrating a statusgen read is `forge-neutral/18`'s. Write-path verb migration is forge-neutral's; git-transport
migration is desktools-go-git's.

## 5. The pilot and the reframed centre of gravity

#1223 ("retire `gh` shell-out in the read path for a native installation-token client") is the
first concrete migration. The desk audit (2026-09-16) reframed where the weight is: the desk
verbs are ~mostly migrated, with five sanctioned `gh` exceptions that are token-custody
decisions rather than transport gaps (`deskadvisory/advisory.go:183` `gh auth token`,
`deskdigest/exec.go:47`, `deskmerge/exec.go:114` write-only, `deskdisposition/exec.go:30`,
`deskpushguard/main.go:409`). The **higher-value read-path target is statusgen** (Principle 2),
and that migration is `forge-neutral/18`'s, through `deskread`. #1223's read-path fix is only
real once it reaches statusgen and unblocks `scanloop`; v2's part in that is to make the
result measurable (`02`) and irreversible (`08`), not to perform it a second time.

## 6. Definition of done for the stream

The stream is done when: the ban-lint is wired and **failing** (not advisory) with a count of
zero reach-past sites outside the two backends, **and it covers statusgen**; **statusgen's
forge reads run through `deskread` under explicit minted-token custody, not `gh`, and that
zero is a failing check**; every issue in §1's table is closed by a landed migration whose old path was
removed in the same change; the custody invariant (Principle 1) holds on the desktop as in a
container; the access-pattern query layer serves at least the board/review-queue sweep as
one measured, single-snapshot round-trip; and every outward write passes the one outbound
check of §8, with the ban-lint proving no write path is constructed around it.

## 7. Open questions for the approver

1. **Scope of the ban-lint's third pattern** — start narrow (subprocess `gh`, remote
   `"origin"`, `pullRequest`/`mergeRequest` GraphQL blocks) and widen by evidence, or specify
   the full pattern up front?
2. **Which stream holds the statusgen zero** — this document assigns the post-migration
   enforcement (`desktools-v2/08`) to v2 because the counter is v2's. The alternative is to
   fold that brief into `forge-neutral` behind `forge-neutral/18`. Either is coherent; what
   must not recur is two streams each owning the migration.
3. **Native-client boundary with forge-neutral** — v2's native read clients consume a minted
   token; minting stays in the identity layer (forge-neutral). Confirm this split.
4. **desktools-go-git handoff for #1201/#884** — v2 owns the forge-assumption half; confirm.
5. **Outbound-check overrides (§8)** — whether a withheld-identifier or house-callout refusal
   on a public target may be overridden from the verb at all. `desktools-v2/10` proposes not,
   and puts the question to the human gate.

## 8. Outbound writes — one check at the write seam

The driver's direction of 2026-09-17: a deployment's rules about what may be written to a
forge should be enforced at a lower layer, in a discrete format, by the desk tools — not by
skill prose an agent has to remember.

### 8.1 What happened (one deployment, one day, stated without its identifiers)

1. **An issue filed on a public repository named identifiers that deployment withholds.** It
   was filed through `deskfile new`, which passed every check it runs.
2. **A pull request's diff carried a withheld repository name in a comment in a test file.**
   Nothing on the push path looked for it; it passed review and was caught only by a
   merge-gate sweep afterwards, which costs a review round trip per attempt because that gate
   deliberately does not say what it matched.
3. **The body scan that protects PR bodies is not applied uniformly.** The table below is the
   evidence. Incident 1 is its first row; incident 2 is its `deskpr create/update` diff cell.

All three are the same defect: each check is called by each verb that remembered to call it.

### 8.2 Which outward verb runs which check today

Established from `tools/desk/cmd/*` on `origin/main`, 2026-09-17. **Secret** is the credential
and entropy scan (`deskkit.ScanSurface` / `BodyCheck`, `bodycheck.go:313,329`).
**Self-contained** is the public-target scan (`deskkit.SelfContainCheck`, `selfcontain.go:218`),
which also carries the **withheld-identifier** category (`WithheldIdentifiers`,
`selfcontain.go:160`) — there is no separate withheld call, so a verb without the one has
neither. It applies only when the target is not known-private (`SelfContainApplies`,
`selfcontain.go:200`). **Override** is the audited `--force-scan-override "<reason>"` path
(`HandleScanRefusal`, `scanoverride.go`). No verb runs a personal-data check of any kind.

| Verb (write) | Surface | Secret | Self-contained + withheld (public targets) | Override offered |
|---|---|---|---|---|
| `deskfile new` (`FileIssue`, `deskfile.go:661`) | issue title + body | yes (`deskfile.go:545,548`) | **no** | no ("No override flag exists", `deskfile.go:540`) |
| `deskfile attach` (`PostCommentTyped`, `:812`) | comment body | yes (`:780`) | **no** | no |
| `deskpr create` (`CreateDraftChange`, `deskpr.go:335`) | PR title + body | yes (`deskpr.go:178,720`) | yes (`deskpr.go:232`) | yes |
| `deskpr create` / `update` (the push) | branch name | yes (`deskpr.go:724`) | **no** | yes |
| `deskpr create` / `update` (the push) | branch diff | secret arms + ruling-claim guard only (`deskpr.go:762,766`) | **no** | yes |
| `deskpr create` / `update` (the push) | commit messages | **no** | **no** | — |
| `deskpr edit` (`EditChange`, `edit.go:273`) | PR title + body | yes (`edit.go:98,105`) | yes (`edit.go:221`) | yes |
| `deskreply` reply and `--workpad` (`deskreply.go:321`, `workpad.go:100,243`) | reply body | yes (`deskreply.go:162`) | yes (`deskreply.go:172`) | yes |
| `deskpost comment` (`comment.go:186,189`) | comment body | yes (`comment.go:54`) | yes (`comment.go:64`) | no |
| `deskpost review` (`forgeclient.go:303`) | review body | yes (`review.go:128`) | yes (`review.go:136`) | no |
| `deskevidence` (`WriteFile`, `deskevidence.go:370,422`) | file content (added lines) | yes (`deskevidence.go:281`) | **no** | no |
| `deskevidence` (`CreateDraftChange`, `:442`) | tool-composed PR title + body | **no** | **no** | — |
| `deskclose` (`PostCommentTyped`, `github.go:189`) | tool-composed pre-close comment, which can name a cross-repository canonical target | **no** | **no** | — |
| `deskprovenance` (`main.go:238,241`) | tool-composed comment | **no** | **no** | — |
| `desklabel`, `deskpost label`, `deskflip`, `deskdispatch`, `deskfile new`, `deskclose superseded` (`ApplyLabels`) | label names | **no** | **no** | — |
| `deskpushguard` (pre-push hook) | commit messages, ref names, diff | **no** — it guards lineage and merged-branch pushes, and scans no text | **no** | — |

Read by target visibility: on a **private** target every row's self-containment cell is
inert by design, so the columns differ only on a **public or unknown** target — which is
exactly where the gaps are. Five of sixteen rows run it; issue filing, issue comments,
file writes, every tool-composed body, labels, and everything that leaves by `git push`
do not. The override is just as uneven: `deskpr` and `deskreply` offer it, while `deskfile`,
`deskpost` and `deskevidence` scan and refuse with no audited way through — the shape that
has pushed workers off the sanctioned transport before (`scanoverride.go` header).

### 8.3 The design

**One function, called in one place.** Every `Forge` a desk tool holds is constructed at a
single site — `ResolveForge` (`forgeresolve.go:451`), a property already pinned by
`TestForgeSingleConstructionSite` (`forgeresolve_test.go:282`). The backend it returns is
wrapped there by a checking decorator whose text-carrying write methods (`FileIssue`,
`PostComment`, `PostCommentTyped`, `EditComment`, `CreateDraftChange`, `EditChange`,
`PostReview`, `ApplyLabels`, `WriteFile`) call one function, `OutboundCheck`, before
delegating. A new verb cannot forget the check because it never holds an unchecked `Forge`.
This keeps `forge.go`'s own rule — body checks WRAP the interface and live in neither
backend — and moves the wrap from "each tool, around each call" to "once, where the `Forge`
is made". Text that leaves by `git push` instead of a `Forge` call (added diff lines, commit
messages in the pushed range, ref names) reaches the same function from the two places a push
is already inspected: `deskpr`'s pre-push scan and the `deskpushguard` hook.

**Keyed on the target's visibility.** The allowed-repos roster already states it per
repository — `ASSAY_ALLOWED_REPOS` entries are `owner/name[:ci|:no-ci][:public|:private]`
(`rosterconfig.go:86-91`; the `Visibility` type, `config.go:34-75`), read by `RepoVisibility` (`config.go:174`) with
no network call, and `VisibilityRiskClassed` (`config.go:197`) already fails closed:
everything except a known-private repository is treated as public.

| Layer (compiled, generic, ships with the tools) | Targets |
|---|---|
| secret / entropy scan | all |
| personal-data pass — e-mail addresses and phone-number shapes, with an allow-list for forge no-reply addresses and reserved example domains | all |
| self-containment — references that only resolve inside a private deployment | public and unknown |
| withheld identifiers — the deployment's configured set | public and unknown |

**It refuses; it never rewrites.** A refusal names the rule id, the surface and the line. The
author's text is never edited, redacted or "fixed" by the tool: a silent rewrite changes what
an author said under their identity, and a redaction that misses by one character still
publishes. Overrides reuse the existing audited `--force-scan-override "<reason>"` shape,
offered uniformly by every verb instead of by two of them (`desktools-v2/10`).

**Deployment-specific vocabulary is never compiled in.** `selfcontain.go` already holds that
line for the withheld set. A deployment whose rules need more than a list of identifiers — its
own token map, its own sweep — supplies an executable, on the callout plumbing three gates
already share (`tools/desk/internal/deskkit/callout.go`, `tools/desk/internal/deskkit/riskcallout.go`,
`tools/desk/internal/deskkit/untrustscan.go:432`, `tools/desk/cmd/writeguard/callout.go`): an absolute-path executable that is not group- or
world-writable, one JSON object on stdin, fail-closed on every way the question can go
unanswered, and ONLY-WIDENS — compiled checks answer first and a callout can add a block but
never clear one. The three differ in their ANSWER format (`allow` / `block <reason>` for the
write guard; a JSON object for the risk classifier and the inbound scan); the outbound callout
takes the write guard's, since its question is the same yes/no (`desktools-v2/11`).

**What the forge never sees.** A refusal is local: stderr may name the rule, the line and the
span so the author can act. Nothing about a match — not the span, not a callout's reason
text — is written to the forge, and the audit log records the rule id and a digest of the
content, never the content.
