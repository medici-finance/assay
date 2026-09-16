# desktools-v2 — scoping document

**Status:** draft — proposed 2026-09-16; not yet ruled. Per `spec/lifecycle-v1.md` §8.2 a
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
| **statusgen** | **the largest remaining dependency: a SEPARATE binary with NO native forge client, shelling `gh` directly** — `scanissues.go:114,:870`, `issues.go:577`, `autoflip.go:535,:615,:656,:666`, `autonomy.go:451,:479` — and NOT under the `forgeban` control at all. It forked its own ambient-custody forge access, which is exactly why `scanloop` breaks in a container. |
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
migration `desktools-v2/08` inherits it too.

### Principle 2 — the READ PATH covers statusgen

"Retire `gh` in the read path" (#1223) is NOT done when the desk verbs are done. **statusgen
is the higher-value target**: a separate binary that shells `gh` directly across its scan,
auto-flip and autonomy reads (§1 table) with its own ambient-custody forge access and no
native client. It is the `scanloop`-blind root (#628 / the container break): `scanloop`
invokes `statusgen --scan-issues`, which shells `gh`, so a container with no ambient `gh`
credential reads an empty queue and goes blind.

The read path is done when **statusgen CONSUMES the shared `deskkit` Forge library** instead
of its own `gh` calls. Caveat stated, not dodged: statusgen is a separate Go module today
(`github.com/medici-finance/assay/statusgen`) and `deskkit` lives under `tools/desk/internal/`
— an *internal* package another module cannot import. So this needs **`deskkit` (the Forge
interface + backends) promoted to a properly importable shared library first** — which is
itself part of "built properly", and is `desktools-v2/07`. The statusgen migration is a
**port, not a call-site swap** (there is no go-gh client to route through yet); it is
`desktools-v2/08`.

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
promotes `deskkit` to a shared library, migrates statusgen and the unrouted leak sites onto
it, and exposes purpose-built access-pattern queries.** It is not a second fork of the tools
and not a rewrite of the two backends — both are complete and stay.

Five architectural commitments (the three principles, plus the two mechanics they need):

1. **ONE seam, and only the two backends may speak GitHub/GitLab.** Every transport, identity
   handoff, query construction, remote name, and subprocess name passes through
   `deskkit.Forge`. (Principle 3's "no raw query crosses the interface" is the query half of
   this.)
2. **A ban-lint that makes the reach-past a red build, not a code-review catch** — and that
   covers statusgen, which is not under `forgeban` today.
3. **`deskkit` promoted to an importable shared Forge library**, so a second module
   (statusgen first) can consume it — the "built properly" packaging that Principle 2 needs.
4. **Custody-first native clients** (Principle 1): explicit minted-token, key-presence
   boundary, refuse-ambient — on the desk read path and on statusgen.
5. **Incremental, tool-by-tool migration, with the old path REMOVED as each lands.**

## 4. Boundary with the sibling streams — no duplication

- **`forge-neutral`** (active) — makes the desk verbs the only sanctioned forge *write* path
  on GitHub *and* GitLab, and adds the resolver/custody that decides *which* forge and *which*
  identity performs a write (`forge-neutral/01`). **v2 depends on its resolver and never
  re-implements it.** v2's own contribution is the *ban* (extended to statusgen), the
  *importable shared library*, the *native read clients*, and the *access-pattern query
  layer*.
- **`desktools-go-git`** (active) — moves the desk tools' *git* operations off the `git`
  binary onto `go-git`. The push-guard remote-name / `insteadOf` gaps (#1201, #884) sit at the
  edge of its territory; v2 owns them from the *forge-assumption* angle and coordinates the
  transport half.
- **`desk-tools`** (active) — the general planning board for the current `tools/desk/` suite.
  v2 is the *architectural* successor for the forge-abstraction slice specifically.

**Litmus for "belongs to v2":** the work makes reaching past the `Forge` seam impossible (the
ban-lint), makes `deskkit` importable, migrates a `gh`-shelling read (desk or **statusgen**)
onto a custody-first native client, migrates a named forge-assumption leak site, or adds a
purpose-built access-pattern query. Write-path verb migration is forge-neutral's; git-transport
migration is desktools-go-git's.

## 5. The pilot and the reframed centre of gravity

#1223 ("retire `gh` shell-out in the read path for a native installation-token client") is the
first concrete migration. The desk audit (2026-09-16) reframed where the weight is: the desk
verbs are ~mostly migrated, with five sanctioned `gh` exceptions that are token-custody
decisions rather than transport gaps (`deskadvisory/advisory.go:183` `gh auth token`,
`deskdigest/exec.go:47`, `deskmerge/exec.go:114` write-only, `deskdisposition/exec.go:30`,
`deskpushguard/main.go:409`). The **higher-value read-path target is statusgen** (Principle 2),
which is why the stream now carries the importable-library (`07`) and statusgen-migration
(`08`) briefs. #1223's read-path fix is only real once it reaches statusgen and unblocks
`scanloop`.

## 6. Definition of done for the stream

The stream is done when: the ban-lint is wired and **failing** (not advisory) with a count of
zero reach-past sites outside the two backends, **and it covers statusgen**; `deskkit` is an
importable shared library; **statusgen reads through it under explicit minted-token custody,
not `gh`**; every issue in §1's table is closed by a landed migration whose old path was
removed in the same change; the custody invariant (Principle 1) holds on the desktop as in a
container; and the access-pattern query layer serves at least the board/review-queue sweep as
one measured, single-snapshot round-trip.

## 7. Open questions for the approver

1. **Scope of the ban-lint's third pattern** — start narrow (subprocess `gh`, remote
   `"origin"`, `pullRequest`/`mergeRequest` GraphQL blocks) and widen by evidence, or specify
   the full pattern up front?
2. **`deskkit` promotion shape** — move the Forge package out of `tools/desk/internal/` into a
   new shared module (`github.com/medici-finance/assay/forgekit` or similar), or a shared
   package within a restructured single module? `desktools-v2/07` proposes the minimal
   importable surface and defers the module topology to the approver.
3. **Native-client boundary with forge-neutral** — v2's native read clients consume a minted
   token; minting stays in the identity layer (forge-neutral). Confirm this split.
4. **desktools-go-git handoff for #1201/#884** — v2 owns the forge-assumption half; confirm.
