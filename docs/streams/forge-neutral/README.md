---
stream: forge-neutral
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# forge-neutral Stream

Make the **desk verbs the only sanctioned forge write path, on GitHub and on GitLab
alike** — so that a session's writes are bounded by the verbs' guards rather than by
whatever its credential happens to allow.

Origin: the driver's direction of 2026-09-02 — *"we need a stream to have our tooling be
useful on both GitLab and GitHub. The curl/jq path is a boundary problem: it lets any
session do whatever the token allows, even though the Apps/PATs themselves limit
things."* The evidence that turned that direction into a plan is
[`forge-gitlab`'s live pilot report](../forge-gitlab/pilot-report.md) (run 2026-09-02
against a real GitLab group), whose §2 records that **every write in the round trip was a
hand-built `curl` call** because no desk verb has a GitLab backend, and whose deviations
D-1, D-2, D-3 and D-9 name the identity and substrate gaps this stream closes.

## Purpose

Three claims, each measured rather than assumed. The matrix below is the measurement.

**1. The verbs are the boundary; a raw API call has no boundary at all.**
`deskkit` is described in its own package doc as *"the shared safety plumbing for the
desk-tools suite … kill switch, audit log, rate limit, idempotency store, secret scan,
version stamp, and the canonical exit-code contract"*
(`tools/desk/internal/deskkit/doc.go:1-8`), and the forge surface those verbs reach through
is deliberately closed: fifteen enumerated operations, no generic `Do`/`Raw` method, no
caller-supplied endpoint (`tools/desk/internal/deskkit/forge.go:173-220`), with a
CI-enforced shell-exec ban and no-passthrough shape check
(`.github/workflows/forge-surface-control.yml:1-25`). A session that holds a role token file
and reaches the forge directly gets none of that: no audit line, no rate limit, no secret
scan of the body it posts, no enumerated operation set, no refusal it must respect. It can
perform any write the credential allows — including writes the verbs are built to refuse.
The App/PAT permission set is a *ceiling*, not the control; the verbs are the control. So:
**any forge the fleet runs on must be reachable THROUGH the verbs, and the verbs must
refuse rather than fall back to a raw call.** The pilot is the counter-example that proves
the point — on GitLab there was no verb to use, so the round trip ran on `curl`, and every
guard above was absent for the duration.

**2. Identity is forge-shaped, and today that shape is hardcoded.**
The trust roster keys identities as `[role=]slug[:id]` and then accepts exactly two
renderings of a login — `<slug>[bot]` and `app/<slug>`
(`tools/desk/internal/deskkit/rosterconfig.go:783-800`); the commit-identity preflight
requires the GitHub noreply form `<bot-user-id>+<slug>[bot]@users.noreply.github.com`
(`tools/desk/internal/deskkit/preflight.go:730,752-754`); statusgen's Evidence-actor lint
resolves an accepted verifier by parsing that same address
(`statusgen/evidenceactor.go:165,225-247`); `--auto-flip-model` composes the reviewer login
by appending a literal `"[bot]"` (`statusgen/autoflip.go:179-185`); and `verifyrun` derives
its `Runner` cell from `GITHUB_ACTIONS`/`GITHUB_ACTOR` or local git config
(`statusgen/verifyrun.go:621-645`). A GitLab service account is none of those things — it is
a numeric bot user with a `service_account_group_<id>_*@noreply.gitlab.com` commit address.
The consequence is measured, not predicted: on the pilot, with Evidence committed by a
distinct non-implementing verifier account, the lint reported *"0 row(s) are backed"* — **a
correctly verified row was indistinguishable from a self-attested one**
([pilot D-3](../forge-gitlab/pilot-report.md)), and the witness row named a host-derived
handle rather than the acting account ([D-9](../forge-gitlab/pilot-report.md)). A check that
cannot recognise the acting identity does not fail loudly; it fails open.

**3. The substrate around the verbs is GitHub-keyed too.**
`statusgen init` scaffolds `.github/workflows/assay-statusgen.yml` and nothing else
(`statusgen/init.go:84`), so a GitLab adopter's board has no single writer at all, while
`--lint` still exits `0` with a NOTICE (`statusgen/claimdecay.go:95-97`) — a silently
degraded check (`#349`). The cross-machine dispatch claim is a `refs/dispatch/*` ref whose
*release* has no GitLab expression: `DeleteRef` answers *"could-not-check: GitLab exposes no
general ref-delete endpoint … a claim held outside refs/heads has no CE equivalent and is
NOT reported released"* (`tools/desk/internal/deskkit/forge_gitlab.go:1227-1240`). `cellctl`
requires a GitHub App PEM and mints installation tokens against hardcoded `api.github.com`
(`tools/cellctl/cellctl:285,134-152`). The public leak gate's verdict is a GitHub **commit
status** (`.github/workflows/leaksweep-pattern.yml:1-27`). And the front door is the same
shape: the install skill has **zero** GitLab mentions, states *"Prerequisite: two GitHub
accounts"* (`plugins/assay/skills/install/SKILL.md:60`) and acquires its pinned binaries with
`gh release download` (`:90,:109`, and `plugins/assay/skills/adopt/SKILL.md:31`), so an
adopter on a GitLab-only box stalls at the front door, before any of the above is reached.

**How this stream measures itself.** The blockers are not this stream's discovery; they are
already enumerated in the tree. `tools/desk/internal/forgeban/allowlist.go` is a permit
register of the surviving forge-CLI call sites — 22 resolved `gh` rows plus 2 unresolved-argv
rows — locked by a ratchet (`const allowedInvocationCeiling = 24`, `allowlist.go:56`) that
fails both when the list grows *and* when a migration forgets to lower it. Its header reduces
every remaining row to two blocker shapes (`allowlist.go:21-40`): **(identity)** the tool
reaches the forge under the caller's ambient CLI credential by documented design, and both
backends refuse an unminted token *"so that no desk write can silently run as whatever
identity happens to be active"* — routing it through the seam is therefore a token-custody
decision, not a transport change; and **(no-op)** the operation has no enumerated `Forge`
method, and the freeze rule forbids adding one speculatively. This stream is the set of briefs
that answer those rows, and **the ceiling is its progress metric: 24 at `deae247`, 10 at the
end of wave 3.**

**Scope boundary.** [`forge-gitlab`](../forge-gitlab/README.md) built the seam and the two
backends; this stream **wires the fleet onto it** and makes the wiring the only path. It
does not re-litigate the GitLab security-parity ruling (that is `../forge-gitlab/spec.md` §1
and §3) and it adds no new forge backend.

## Measured matrix — verb × forge path × identity assumption

Measured on this branch's base, `deae247` (2026-09-02). `Forge` = reaches the forge through
`deskkit.Forge`; `gh` = shells the `gh` binary; `net/http` = hand-rolled requests against
`deskkit.GitHubAPIBase`; `git` = plain git, already forge-neutral.

### The head finding

**Nothing constructs a `Forge`.** Both backends are complete —
`GitHubForge` (`tools/desk/internal/deskkit/forge_github.go`) and `GitLabForge`
(`tools/desk/internal/deskkit/forge_gitlab.go`) implement all fifteen methods — and:

- `grep -rn 'GitHubForge{\|GitLabForge{' tools/ --include='*.go' | grep -v _test.go` → **0 hits**.
- No `NewForge` / `ForgeFor` / `ResolveForge` / `NewGitHubForge` / `NewGitLabForge`
  constructor exists anywhere in `deskkit`. There is no resolver to call.
- The only non-`deskkit` consumer of the interface is `fanoutloop`'s dispatch sink
  (`tools/desk/cmd/fanoutloop/land.go:70-97`), whose constructor
  `newForgeDispatchSink` is called **only from a test**
  (`cmd/fanoutloop/fanoutloop_test.go:561`); in production the field is nil and the sink
  returns *"no forge wired into the sink"* (`land.go:93-94`). The file says so itself:
  *"wired only at cutover, on the owner's call"* (`land.go:35`).
- The one `--forge` selector in the whole suite is a **session-settable flag** on
  `desktoken` (`tools/desk/cmd/desktoken/desktoken.go:454`), and it selects a *token-custody
  path*, not a backend — it never constructs a `GitLabForge` (`desktoken.go:475-482` →
  `cmd/desktoken/gitlab.go:163`). There is no `ASSAY_FORGE` anywhere, no `forge:` config key
  in any yaml/json/toml, and no verb reads a forge from repo config.

So the seam is real, its backends are real, and the fleet does not touch it —
`tools/desk/internal/deskkit/forge.go:169` calls `Forge` *"the single seam every desk tool
reaches a forge through"*, which is the design intent, not the measured state. `#274` reports
the same thing from the other side (the `go-gh` migration's call-site half did not land);
measured from this side it is stronger: **the seam has no production consumer at all**, so
"which forge" is not yet a question any verb can be asked.

Two properties sit **deliberately outside** the interface and must be bound elsewhere
(`forge.go:10-17`): token minting/identity — *"a `Forge` is handed an already-minted token;
it never mints one"* — and the budget/rate-limit/breaker/body-check wrappers the calling verb
applies around a call. Brief 01 is where those bindings are specified; getting them wrong is
how a resolver quietly becomes a way to act as the wrong identity.

### Write verbs

| Verb | Forge path | Forge-selectable? | Identity assumption |
|---|---|---|---|
| `deskpr` | `gh` — `runCmd(dir, "gh", …)` (`cmd/deskpr/exec.go:87`), PR create at `cmd/deskpr/deskpr.go:294` | no | `GH_TOKEN` injected into the shelled `gh` (`cmd/deskpr/exec.go:54`); App-minted worker token |
| `deskreply` | `gh` (`cmd/deskreply/exec.go:79`, token at `:43`) | no | same as `deskpr` |
| `deskflip` | `gh` — `pr ready` (`cmd/deskflip/flip.go:325`), `pr edit` (`:675,:681`), `gh api …/pulls/<n>/reviews` (`:870`), checks (`:453,:914`) | no | reviewer App login; GitHub review-state vocabulary |
| `deskfile` | `gh` (`cmd/deskfile/exec.go:50`), issue create at `cmd/deskfile/deskfile.go:473` | no | `--raised-by <role>` resolved against the roster |
| `deskclose` | `gh` — `runGH` (`cmd/deskclose/exec.go:48`), close at `cmd/deskclose/github.go:195`, authority read at `cmd/deskclose/authority.go:129` | no | roster `worker=`/`reviewer=` bindings (`cmd/deskclose/superseded.go:39`) |
| `deskpost` | `net/http` — `apiBaseURL = deskkit.GitHubAPIBase`, *"deliberately NO env var or flag override"* (`cmd/deskpost/github.go:26-33`); App installation mint at `:185` | no | GitHub App JWT → installation token |
| `deskevidence` | `net/http` — same binding (`cmd/deskevidence/github.go:25-29`); installation mint at `:220`; Evidence written via the **Contents API** (`:358`) | no | verifier App bot login (`cmd/deskevidence/github.go:44-46`); GitHub App custody end to end |
| `deskrelease` | `net/http` (`cmd/deskrelease/github.go:23,109`); creates tag refs at `:194-196` | no | `desktoken desk --repo <slug>` installation token (`github.go:38`) |
| `deskdigest`, `deskdisposition`, `deskmerge`, `deskdispatch`, `scanloop` | `gh` (`cmd/deskdigest/exec.go:47`; `cmd/deskdisposition/exec.go:30`; `cmd/deskmerge/exec.go:56` + `git push` at `merge.go:322`; `cmd/deskdispatch/dispatch.go:631-632`; `cmd/scanloop/lane.go:275`) | no | mixed: `TrustedAuthorID`, ambient credential, or none |
| `desktoken` | GitHub App mint **or** GitLab PAT rotate (`cmd/desktoken/gitlab.go:163`) | **yes — but by flag, and it is a custody switch, not a backend switch** (`cmd/desktoken/desktoken.go:454,475-482`) | GitHub side needs `AppID(role)` + PEM (`desktoken.go:493`); GitLab side needs neither, and reads `GITLAB_API_BASE` with no fallback (`gitlab.go:54-55`) |

### Read verbs and loops

| Verb | Forge path | Forge-selectable? | Identity assumption |
|---|---|---|---|
| `deskboard` | `gh` (`cmd/deskboard/board.go:143`) | no | role App installation token per account; roster bots + `ASSAY_HUMAN_LOGIN_MAP` (`board.go:39,1750`) |
| `issueboard` | `gh` (`cmd/issueboard/board.go:147`) | no | roster logins |
| `scanloop` | `gh` (`cmd/scanloop/trust.go:159`, `cmd/scanloop/lane.go:275`) | no | trust gate over GitHub logins |
| `reviewloop` / `verifyloop` / `commsloop` | via `loopengine` + the verbs they drive | no | inherited from the driven verb |
| `deskdispatch` | `gh` for labels (`cmd/deskdispatch/dispatch.go:631-632`); claim delegated to the consumer repo's dispatch-claim script (`:54`, argv at `:440`) | no | claim key grammar `<repo>--<stream>--<NN>` (`:667`) |
| `fanoutloop` | `git` for local claim reads (`internal/loopengine/writescope_io.go:47`); **`Forge.DeleteRef`** for release (`cmd/fanoutloop/land.go:99`) | n/a — forge is nil in production | `refs/dispatch/*` namespace (`land.go:107`) |
| `deskroster`, `deskpushguard`, `deskadvisory`, `repohardenguard` | `gh` (`cmd/deskroster/roster.go:214,231`; `cmd/deskpushguard/main.go:401`; `cmd/deskadvisory/advisory.go:183` — `gh auth token`, i.e. the **ambient** credential; `cmd/repohardenguard/check.go:46` — rulesets and App permissions) | no | GitHub logins, ruleset and App-permission vocabulary with no GitLab analogue yet |
| `deskgit` | `git` only, host deliberately **not** bound to `github.com` (`cmd/deskgit/deskgit.go:320`) | n/a | holds no credentials (`cmd/deskgit/main.go:58`) — the one already-neutral verb |
| `deskclaim`, `deskcomms`, `deskscanbody`, `desksourceguard`, `deskverdict`, `deskversion`, `deskmigrate`, `deskpins`, `deskwt`, `verifyloop`, `commsloop`, `opmetrics`, `muhar`, `upgrade-assay`, `writeguard` | `local` — no forge reach | n/a | `deskwt` builds the bot commit identity `<botUserID>+<slug>[bot]@…` (`cmd/deskwt/roleinit.go:44,143-144`) |

**The register, not a grep, is the authoritative count.** `forgeban` permits **22 resolved
`gh` call sites** plus 2 unresolved-argv rows, ceiling **24**
(`tools/desk/internal/forgeban/allowlist.go:56`), each with a `TODO(forge-surface)` naming its
blocker. Distribution: `deskflip` 6 · `deskroster` 2 · `deskadvisory`, `deskboard`,
`deskclose`, `deskdigest`, `deskdispatch`, `deskdisposition`, `deskfile`, `deskmerge`,
`deskpr`, `deskpushguard`, `deskreply`, `issueboard`, `repohardenguard`, `scanloop` 1 each ·
plus `cmd/scanloop/lane.go::RealExec` and `internal/askassay/probe.go::execRead` as
unresolved-argv rows (`allowlist.go:227,240`).

### statusgen

| Surface | Forge path | Identity assumption |
|---|---|---|
| `--auto-flip-model` | `gh pr view` (`statusgen/autoflip.go:518`) + `gh api repos/<r>/pulls/<n>/reviews` (`:528`), commit→PR at `:397` | reviewer login composed as `<slug>` + literal `"[bot]"` (`:179-185`) |
| `verifyrun` | none (local) | `GITHUB_ACTIONS`/`GITHUB_ACTOR`, else git config; GitHub noreply parsed at `verifyrun.go:641-645`; runner derived at `:1152`, refusal at `:1154` |
| Evidence-actor lint | `git blame` only (`statusgen/evidenceactor.go:443`) — already forge-neutral in *mechanism* | accepted actor = roster `verifier=<slug>[:<id>]` (`:238`) matched against the GitHub noreply address regex (`:165`); could-not-check wrapper at `:527-529`; noted dead in CI at `:128-131` |
| `init` | writes `.github/workflows/assay-statusgen.yml` only (`statusgen/init.go:84`); workflow body at `:274` shells `gh release download` (`:311,:340`) | commits as `statusgen@users.noreply.github.com` (`:349`) |
| dead-claim decay | `gh pr list` (`statusgen/claimdecay.go:43`); NOTICE at `:95-97`, `LINT: PASS` regardless | — |
| `--scan-issues`, `--transcribe-*`, `--dora`, trust gate, corroborate | `gh api` / `gh issue list` / `gh api graphql` (`scanissues.go:102,810`; `transcribescan.go:72,104,132`; `doratiming.go:644,670,705`; `trustgate.go:204`; `corroborate.go:618,646,753`) | GitHub URL and GraphQL shapes hardcoded |
| network client | `const githubAPIBase = "https://api.github.com"` — *"the ONLY network code in statusgen"* (`statusgen/ghfetch.go:3,45`) | — |

### Substrate

| Surface | State at `deae247` |
|---|---|
| Dispatch claim | Ref namespace `refs/dispatch/*` (`cmd/fanoutloop/land.go:107`). **Create/read are plain git** — `for-each-ref` (`internal/loopengine/writescope_io.go:47`), `git ls-remote origin 'refs/dispatch/*'` in the skills — so already forge-neutral. **Release is not**: `DeleteRef` maps only `heads/<branch>` on GitLab and refuses the `dispatch/` namespace as could-not-check (`internal/deskkit/forge_gitlab.go:1227-1240`; op 15 in `../forge-gitlab/inventory.md:46`) |
| `cellctl` | `--deskd-app-pem` is a **required** flag on `new` (`tools/cellctl/cellctl:285,287-288`); `deskd` signs an RS256 App JWT (`:134-138`) and mints per-org installation tokens against `https://api.github.com` (`:140,147-150,152`); `new` symlinks the operator's `gh` config (`:293`) |
| Leak gate | Two GitHub workflows; the strong control half runs privately and posts its verdict as the `leak-sweep` **commit status** (`.github/workflows/leaksweep-pattern.yml:1-27`). No merge-request equivalent exists |
| Forge-surface enforcement | `.github/workflows/forge-surface-control.yml` — shell-exec ban, no-passthrough shape check, permit-register ratchet. **This is the control this stream extends**, not one it replaces |
| GitLab CI | **No `.gitlab-ci.yml` anywhere in the repo** (`git ls-files \| grep -c gitlab-ci` → `0`) |
| Install path | `plugins/assay/skills/install/SKILL.md` — **0** GitLab mentions; *"Prerequisite: two GitHub accounts"* (`:60`); binaries via `gh release download` (`:90,:109`). `plugins/assay/skills/adopt/SKILL.md:28` lists the eight CORE primitives, `:31` binds `install-statusgen` to `gh release download`. `docs/adopting-assay.md:12` declares itself *"GitHub-shaped throughout (Apps, rulesets, `gh`)"*. `statusgen/init.go` — **0** GitLab references |

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Forge resolution contract — the forge comes from repo config, and refusal is the only fallback](brief-01-forge-resolution-contract.md) | 1 | M | implemented | — | — |
| 02 | [Forge-qualified identity — roster entries, bot renderings, review corroboration](brief-02-forge-qualified-identity.md) | 2 | M | todo | — | — |
| 03 | [Write verbs A — deskpost, deskreply, deskflip onto the resolver](brief-03-write-verbs-comment-and-flip.md) | 2 | M | todo | — | — |
| 04 | [Write verbs B — deskpr, deskfile, deskclose, deskevidence onto the resolver](brief-04-write-verbs-issues-and-evidence.md) | 2 | M | todo | — | — |
| 05 | [Claim layer — the GitLab shape of `refs/dispatch/*` and its release](brief-05-claim-layer-forge-shape.md) | 2 | M | todo | — | — |
| 06 | [Read verbs — deskboard, issueboard, scanloop, reviewloop on the seam](brief-06-read-verbs-on-the-seam.md) | 3 | M | todo | — | — |
| 07 | [statusgen acting identity — Evidence-actor and `verifyrun` name the forge identity that acted](brief-07-statusgen-acting-identity.md) | 3 | M | todo | — | — |
| 08 | [statusgen forge-aware — `init` CI scaffold, auto-flip corroboration, honest claim decay](brief-08-statusgen-forge-aware.md) | 4 | M | todo | — | — |
| 09 | [Substrate — leak-gate verdict on merge requests, `cellctl` forge-aware `new`/`up`](brief-09-substrate-leakgate-and-cellctl.md) | 3 | M | todo | — | — |
| 10 | [Conformance — one round trip driven entirely by desk verbs, and the writes they refuse](brief-10-conformance-round-trip.md) | 5 | M | todo | — | — |
| 11 | [Install without `gh` — binary acquisition, forge-neutral prerequisites, per-forge primitives](brief-11-install-without-gh.md) | 5 | M | todo | — | — |

## Critical path

```
                                                              ┌─> forge-neutral/10
forge-neutral/01 ─> forge-neutral/02 ─> forge-neutral/07 ─> 08 ┤   (verbs-only round trip)
 (resolver +         (forge-qualified     (Evidence-actor +  │ │
  custody +           roster + review      verifyrun name    │ └─> forge-neutral/11
  refusal)            corroboration)       who acted)        │     (install on a box
                                          (08 = init scaffold +    with no `gh`)
                                           auto-flip corroboration)
```

One chain, forking only at the last hop. **10** is the pacing item for the *fleet* claim; **11**
is the pacing item for the *adopter* claim. Neither may be pre-credited from the other: a fleet
that round-trips a brief on GitLab still leaves a GitLab-only adopter unable to install, and an
install that completes proves nothing about whether the verbs then work.

The chain is real, not conventional. 08's auto-flip has to recognise a reviewer identity on the
configured forge, which is 07's roster-parity deliverable inside statusgen; 07's actor matching
needs 02's forge-qualified grammar; and 02 has nothing to agree with until 01 has decided which
forge serves a repo. Break any link and the one after it is describing an identity no code path
can resolve.

**Smallest unblocking move:** add a single resolver in `deskkit` that takes a repo and
returns the `Forge` for it, sourced from repo configuration, and route **one** write verb
through it end to end.

**The head is 01, and the reason is measured, not assumed.** The tempting first step is the
identity work — the pilot's loudest failure is D-3, a correctly-verified row reading as
self-attested — and the second most tempting is "give the verbs a `--forge` flag". Both are
dead ends at `deae247`:

- A `--forge` flag is the wrong shape *and* has nothing to bind to. Verified:
  `grep -rn 'GitHubForge{\|GitLabForge{' tools/ --include='*.go' | grep -v _test.go`
  returns nothing, no `New*Forge` constructor exists in `deskkit`, and the interface's only
  production consumer (`cmd/fanoutloop/land.go:70-97`) is reached solely from a test and
  errors with *"no forge wired into the sink"* when it is not. Two complete backends, zero
  callers. Until a verb can **obtain** a Forge, "which forge" is not a question the code can
  be asked.
- Identity work (02) is real but it is downstream: the Evidence-actor lint and `verifyrun`
  must name *the identity that acted through a verb*, and no verb acts through the seam yet.
  Fixing the roster first would leave the roster describing identities no code path
  consults.

01 is genuinely unblocked: both backends are in this repository, the enumerated surface is
frozen and CI-enforced (`.github/workflows/forge-surface-control.yml`), and nothing upstream
gates writing a resolver.

**Pacing items: 10 and 11.** Nothing may claim the fleet runs on GitLab until one brief
round-trips `todo → done` with every forge write performed by a desk verb and **zero**
hand-built API calls — the exact property the 2026-09-02 pilot could not have
(`pilot-report.md` §2), and the only check that distinguishes "the verbs support GitLab" from
"GitLab support was described". And nothing may claim the *tooling* is useful on GitLab until
the install completes on a box with no `gh` on `PATH` (11), since today the very first
primitive is `gh release download`.

**What the ratchet reads at each wave.** 24 at `deae247` → 17 after 03 (`deskreply` 1 +
`deskflip` 6) → 14 after 04 (`deskpr`, `deskfile`, `deskclose`) → 10 after 06 (`deskboard`,
`issueboard`, `scanloop` ×2). The **ten rows that remain are declared out of scope and named**
rather than left implicit: `deskadvisory`, `deskdigest`, `deskdisposition`, `deskmerge`
(identity — each runs under an ambient credential by documented design, so migrating them is a
custody decision this stream does not take), `deskroster` ×2, `deskpushguard`,
`repohardenguard`, `deskdispatch` (no-op — branch→PR resolution, ruleset and App-permission
reads, and label writes have no enumerated `Forge` method and no settled GitLab mapping), and
`internal/askassay/probe.go::execRead` (unresolved argv). Each is a follow-up brief, not a
silent remainder.

**Related open work, not on this path.** `#274` (the `go-gh` call-site migration for
`deskpr`/`deskfile`/`deskclose`) overlaps brief 04's GitHub half: 04 subsumes it for those
verbs by routing them through the resolver, and cites it. `#349` (statusgen's GitHub-only
scaffold and silently-degraded claim decay) is brief 08. `#346` (provisioner defects) and
`#348` (adopter doc) are `forge-gitlab`'s and are not re-opened here.

## Dependency waves

- **Wave 1** — `forge-neutral/01`. The resolver, the per-forge custody binding, and the
  refusal contract. Everything else depends on it.
- **Wave 2** — `forge-neutral/02`, `03`, `04`, `05` (all depend only on 01, all
  parallelizable): identity, the two write-verb wiring briefs, and the claim layer.
- **Wave 3** — `forge-neutral/06` (reads; depends on 01 + 03 for the established wiring
  shape), `forge-neutral/07` (statusgen's acting identity; depends on 01 + 02),
  `forge-neutral/09` (substrate; depends on 01 + 02).
- **Wave 4** — `forge-neutral/08` (statusgen's CI scaffold and auto-flip; depends on 01, 02
  and 07, whose roster parity inside statusgen it consumes).
- **Wave 5** — `forge-neutral/10` (conformance round trip; depends on 03, 04, 05, 06, 07, 08,
  09) and `forge-neutral/11` (install without `gh`; depends on 01, 02, 08).

One-line path: `01 → 02 → 07 → 08 → {10, 11}`.

## Shared conventions the briefs inherit

- **Refusal, never fallback.** Every forge-touching path that cannot be served on the
  configured forge returns a `deskkit.Unverifiable` could-not-check naming the gap — the
  shape `GitLabForge.DeleteRef` already uses
  (`tools/desk/internal/deskkit/forge_gitlab.go:1233-1238`). Silently doing the GitHub thing,
  or dropping to a raw request, is the failure this stream exists to remove.
- **The surface stays closed.** No brief adds a generic/passthrough method or a new
  `gh`/`glab` shell-out; `forge-surface-control.yml`'s three controls must stay green, and a
  brief that needs a new operation adds it to the frozen inventory with its consuming verb.
- **Negative-path rows are mandatory.** Every guard gets a Verify row that proves it fires,
  not only one that proves the happy path completes.
- **No hand-built API call is evidence.** A pilot or Verify row satisfied by `curl` proves
  the forge works, not that the verbs do — which is precisely the gap
  [`pilot-report.md` §2](../forge-gitlab/pilot-report.md) records.
