# Changelog

All notable changes to this repository — the canonical, releasing home for the
shared Assay tools (statusgen, desk-tools, drainloop) and the `assay` plugin —
are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this repo versions the
whole umbrella with a plain `vX.Y.Z` tag (see `.github/workflows/release.yml`),
so one section covers every shipped tool at that version.

Every notable change records itself as one small **fragment** file under
`changelog/` (`changelog/<slug>.md`) BEFORE it merges — one file per PR, so
concurrent PRs never collide on a shared section (the `changelog-check` CI leg
enforces this; a genuinely non-notable PR carries the `changelog:skip` label). At
release time the release workflow AGGREGATES the fragments (sorted, deduped) into
a dated `## vX.Y.Z — <date>` heading here and into the published release notes,
then clears `changelog/` — descriptive highlights, never a raw commit list. See
`changelog/README.md`.

## Unreleased

Pending notable changes are recorded as one-file-per-PR fragments under
`changelog/` (see `changelog/README.md`), aggregated into a dated section
here at release time. This section is written only by the release workflow;
do not add highlight bullets to it directly.

## v0.25.0 — 2026-09-03

### Added
- **`desksupervise`** — the liveness *observer* that finally supplies `internal/loopengine`'s
  fully-coded, fully-inert liveness taxonomy (`ObservableProbe`, `LivenessPolicy`) with real
  probes. `internal/loopengine/probes.go` adds `AuditProbe`, `BranchProbe`, `PRProbe` (each
  three-state — a probe that cannot reach its source reports could-not-check, never no-life),
  composed by `HouseProbes()`, plus `ClassifyLiveness`/`Disposition`, the taxonomy re-exported
  for a reader outside the engine's own in-flight tracker. `desksupervise tick` classifies
  every `state=dispatched` dispatch claim into `ALIVE` / `NEVER-STARTED` /
  `HEARTBEAT-EXPIRED` / `OVER-WALL-CAP` / `COULD-NOT-CHECK`, releasing a wedged claim
  (`RECLAIM-ELIGIBLE`) or landing a budget-blowing one `BLOCKED-TIMEOUT` (a filed
  `help wanted` issue, never re-dispatched blind) — turning a worker stuck behind the
  120-minute stale-claim backstop into a logged, minutes-scale reclaim with no human in the
  loop. `--dry-run` classifies and prints only; `run --interval` loops `tick` forever,
  mirroring `deskwt prune --interval`. `--claims-fixture`/`--observations-fixture` bypass the
  live claim tool and the forge/audit file entirely, so the whole classification path runs
  offline. `deskkit.PullRequest` gains an `UpdatedAt` field (GitHub and GitLab both wired) as
  the forge read PRProbe needs. See `tools/desk/README.md`'s tool-reference row and
  `docs/streams/desk-supervision/brief-01-observable-probes-and-observer.md`.
- A **retrospective input feed** that emits the four-part input set — churn
  trend, gate yield, per-stage ledger, and budget status — as generated/logged
  output a cadence retrospective consumes.
- Custody: `ForgeFor` obtains the resolved role's already-minted token from the existing
  per-forge path — GitHub via the `desktoken` mint-or-reuse path (`RoleTokenForRepo`),
  GitLab by reading the `gitlab-<role>.token` file a prior rotation produced — and never
  falls back to an ambient credential. A missing or insecurely-permissioned (non-0600)
  custody file is refused, naming the remedy. `SetGitHubCustodyMinter` is an installable
  seam a caller that already mints its own GitHub App tokens in-process can plug its
  existing, tested minter into, rather than this package growing a second implementation.
- Per-stream quality **error-budgets** (`qualgen/consumers`) in an alarm
  posture: a breach raises an alarm record rather than a dashboard line, and a
  budget refuses to arm until the stream has at least two measured windows
  (could-not-measure, never armed at zero).
- The worker prompt kit (`common-clauses.md`) now carries the workpad rule: keep one
  workpad per PR, no separate done/summary comments.
- `deskkit.ForgeFor(repo, role)` — the first resolver that can hand a desk tool a `Forge`
  backend at all. Two complete backends (`GitHubForge`, `GitLabForge`) have existed with no
  constructor, no config key, and no consumer; this is that missing answer, and the ONLY
  function in the tree allowed to construct either backend (enforced by
  `TestForgeSingleConstructionSite`'s AST walk plus an independent grep, and backed by the
  existing `forge-surface-control.yml` shell-exec/passthrough CI job). Resolution reads the
  repo's forge from a new roster key, `ASSAY_REPO_FORGES` (`owner/name=github` or
  `owner/name=gitlab`, full slug only — a bare basename is refused, unlike the display-only
  `ASSAY_REPO_ALIASES`), falls back to the origin remote's host when the mapping is
  unambiguous (`github.com`/`gitlab.com` only), and otherwise refuses could-not-check naming
  the repo and the configuration that would resolve it. There is no parameter, flag, or
  environment variable by which a caller supplies the forge itself
  (`TestForgeForRejectsCallerSuppliedForge`).
- `deskpost`'s `comment` verb is wired end-to-end through `ForgeFor` as the
  proof-of-reachability: the actual `POST .../comments` call now goes through the resolved
  `Forge.PostComment`, authenticated via `deskpost`'s own existing App-token mint installed
  as the custody minter above — every precondition read on the same command still runs on
  the pre-existing client, unchanged. `deskpost` carries no `forgeban` permit row, so this
  step moves that ratchet by zero; it only proves the resolver is reachable before any later
  brief's migration claim rests on it.
- `deskreply --workpad` upserts ONE marked progress comment per PR — finds the newest
  unresolved comment authored by the worker identity carrying the `<!-- assay:workpad -->`
  marker and edits it in place, or creates the first one; `--dry-run` reports which without
  writing. Never edits a human's or a minimised comment.
- `deskroster width --role <loop> --reserve resume=N,rework=M` sets a per-class concurrency RESERVATION beside a loop's pool width, riding the same stored entry and decaying with it; plain `deskroster width --role <loop>` now prints `width=<n> reserve=resume:2,rework:0 (source=default|set, expires=...)`. `fanoutloop plan` classifies its queue into resume / rework / fresh and prints `classes: resume=<n> rework=<n> fresh=<n> (fresh capped at <k> by reservation)` — a floor that protects orphan-PR resumes and `Awaiting implementer rework` rows from being crowded out by fresh dispatch under a full pool, and never idles a slot when nothing reserved is waiting. `plan` also now sources `Awaiting implementer rework` board rows directly (previously a manual board read). worker-desk ships a default reservation of `resume=2`. `deskboard throughput` reports the same reservation as an extra column beside the width it never subtracts from.
- `internal/deskkit/workpad.go`: the marker, the fixed-section template (`Render`/`Parse`),
  and `Stamp(worktree, sha)` for the environment-stamp line — never a machine path.
- `qualgen sweep` — a standing, current-tree code-slop forensic sweep lane:
  configured external linters nominate suspects (leg 1), a pluggable
  `AgentVerifier` adjudicates each new suspect with emitter-side evidence
  enforcement (leg 2), and an evidenced, report-only markdown artifact is
  rendered per run (leg 3). Incremental by fingerprint — a rerun over an
  unchanged tree re-verifies nothing — and read-only against the target repo.
  Ships an offline scripted `Fixture` reference verifier; a live coding-agent
  adapter is configuration.
- `qualgen` closes the quality loop: a pluggable issue-filer (`qualgen/filer`,
  with a GitHub Issues reference adapter and a first-class dry-run) turns
  above-threshold hotspots and duplicate-block clusters into **advisory,
  budgeted** refactor items — one per distinct target, degrading to dry-run/log
  once the filing budget is spent, and never self-dispatching work.
- `statusgen --assayscore --json`: a composite **AssayScore** — the geometric mean of four 0–100 sub-scores (Speed, Value, Flow, Quality) computed from the existing brief-flow metrics. Speed and Value normalize against trailing-90-day bands; Flow and Quality are bounded ratios. A dimension that cannot be measured is **excluded** from the mean (never coerced to 0), and the composite is flagged `incomplete` when any dimension is missing — an honest three-state read rather than a silently deflated score.
- `statusgen` §8 lifecycle-routing support: a `**Status:**`/`**Routes-to:**` header reader plus the §8.1 grammar, §8.3 Routes-to, and §8.5 owed-detector lint rules (each finding carries a stable `[rule-tag]`), and a new `statusgen --owed-issues` emit-mode that files one marker-deduped issue per approved-but-uncited routing doc (idempotent, part of the `--decision-issues` family). Ships `docs/workflow-templates/authoring-owed.yml`, an adopter-installable main-push watcher for the emitter. Unclassified/legacy specs are ignored, never rounded up, so `--lint` stays green on existing trees.

### Fixed
- Desk-tools reclaim and house-PR probe paths now obtain their git-forge backend through the single resolver (`ForgeFor`) instead of constructing a GitHub backend directly, restoring the single-construction-site invariant its release-gating test enforces. Behavior is unchanged for GitHub repos (the same per-owner installation token is used); forge kind is now resolver-determined rather than hardcoded.
- `deskboard` and `deskflip` no longer fail closed under a `checks:read`-only identity
  (the reviewer App). gh's built-in `statusCheckRollup` JSON field selects a
  `checkSuite.workflowRun` sub-field — a link to the Actions run, not a check conclusion —
  that requires `actions:read`; under an identity without that scope it 403s and takes the
  whole read down with no salvageable output. `deskboard`'s bulk open-PR read (`prs` /
  `actions`) then exited 6 on the first repository alphabetically, blinding the entire
  cross-repository board, and `deskflip`'s single-PR state read refused to flip any private
  PR. Both reads are now hand-authored `gh api graphql` queries that request the status
  rollup contexts WITHOUT `checkSuite`/`workflowRun`; every conclusion these tools classify
  on (`CheckRun.status`/`conclusion`, `StatusContext.state`) is covered by `checks:read`
  alone, so neither read depends on a scope the tool's identity is not guaranteed to hold.
- `release.yml`'s changelog roll (write the dated section, clear the fragments) now commits and pushes under the assay-board-writer App — the identity that already writes STATUS.md straight to `main` and carries the ruleset bypass — instead of the default `GITHUB_TOKEN`, which the PR-only + leak-sweep-required ruleset rejected on v0.23.0 and v0.24.0 and left the roll to be hand-filed as a PR each time.
- `rosterconfig.go`'s known-key set, echo, and refusal message all recognise the new
  `ASSAY_REPO_FORGES` key, so a deployment that sets it does not fail the whole roster
  closed on the unregistered-`ASSAY_*`-key refusal.

## v0.24.0 — 2026-09-03

### Added
- **`clusterguard`** — an exec-boundary shim for cluster CLIs. Installed as a directory of
  symlinks (`kubectl`, `flux`, `helm`, `talosctl`, `k9s`) on the front of a session's `PATH`, it
  refuses every shimmed CLI by default with exit `5`, records both verdicts to
  `<config-home>/clusterguard.log`, and execs the real CLI only when an operator shell exported
  `ASSAY_ALLOW_CLUSTER` — `=1` for read-only verbs, `=mutate` for everything, any other value
  refused rather than guessed. This catches what a command-text permission rule cannot: a cluster
  call made from inside a committed script never matches a text rule, but it still resolves the
  CLI name on `PATH`. Read-only classification is a per-CLI **allowlist**, so an unclassified verb
  is treated as mutating; `k9s` has no read-only lane at all, being an interactive TUI that can
  mutate from inside the session. A stop flag can only make the guard stricter — an armed kill
  switch refuses (exit `3`) rather than making a refusal-guard stop intercepting, which would fail
  open. Its limits are stated rather than implied: an absolute-path invocation is never
  intercepted (there is a test asserting that bypass exists), and the guard is not a network
  boundary. Contract, verdict table and limits: `tools/desk/README.md`; install notes:
  `docs/adopting-assay.md`.
- A `forge-neutral` planning stream: a waved eleven-brief phase plan for making the desk verbs the only sanctioned forge write path on **both** GitHub and GitLab. It starts from a measured matrix — verb × forge-path × identity-assumption, cited by `file:line` — whose head finding is that `deskkit.Forge` has two complete backends and **no production consumer at all**: neither `GitHubForge` nor `GitLabForge` is constructed anywhere outside tests, no resolver exists to pick one, and the only `--forge` selector in the suite is a session-settable custody switch on `desktoken`. The plan is scored against the in-tree permit register's ratchet (`forgeban`, ceiling 24), which it drives to 10 with the ten surviving rows named rather than left implicit. It delivers a config-resolved forge contract with refusal-never-fallback semantics, forge-qualified trust-roster identity (today's roster hardcodes `<slug>[bot]` / `app/<slug>` renderings and a GitHub noreply commit-email shape), verb-by-verb wiring waves for the write then the read verbs, the claim layer's GitLab shape (`DeleteRef` already answers could-not-check outside `refs/heads`), the statusgen half (Evidence-actor, `verifyrun` runner stamping, `--auto-flip-model` corroboration, a non-GitHub CI scaffold), the substrate (a leak-gate verdict surface on merge requests, `cellctl`), an install path that needs no `gh` on `PATH` (plain-HTTPS binary acquisition verified against the sha256 pin file, forge-neutral two-principals prerequisites, per-forge CORE primitives), and a closing conformance round trip driven entirely by desk verbs with zero hand-built API calls plus a negative-path walk of the writes the verbs refuse. Planning docs only — no tool behavior changes yet.
- A `windows-port` planning stream: a waved five-brief phase plan for native Windows support of the Assay tools (release build matrix, install path with a surfaced PowerShell-vs-Go-installer fork, portability audit, a Windows CI leg, and the adoption-doc delta), with the end state being a Windows adopter running the pinned release, CI-proven on Windows. Planning docs only — no tool behavior changes yet.
- A deterministic `(action, class, risk)` -> Tier assignment table lands in `cmd/commsloop` (`assign.go` + declared source `assign.yaml`, diffed against the compiled table), keyed on the comms prose router's closed action vocabulary — model assignment for cell-comms dispatch is now a compiled table lookup with an audit trail: absent triples refuse, there is no runtime default tier.
- Live evidence replaces two claims that had rested on documentation badges: an author approving its own merge request returns `HTTP 201` on free tier despite `merge_requests_author_approval: false` being stored, and rotate-on-mint is proved end to end — the superseded token returns `HTTP 401 invalid_token` on `GET /user` while its replacement carries a seven-day `expires_at`.
- The PR body's link trailer (`Brief: <stream>/<NN>` / `Issue: #<N>`) is not editable through the new verb. The replacement body must carry exactly one, and when the PR's current body already carries one, the replacement's must be identical to it — the derived board's edge from a PR to its work item cannot be re-pointed or dropped after every gate that checked it has run. A current body carrying no trailer may gain one, which is the pre-trailer migration `deskpr update`'s refusal already tells the worker to perform.
- The `intake-desk` skill gains a scored-triage convention: every triage exit records an `impact`/`risk`/`effort` label triple with a one-line per-axis rationale (judgment recorded, never computed in CI); human-facing surfaces order SLA-ESCALATE items first, then impact-desc / risk-desc / effort-asc, then the existing urgency-then-age — unlabelled items sort exactly as today (#294).
- The first real-history `qualgen` mine of this repo lands: `docs/quality/{metrics.jsonl,mine.json}` over the full 717-commit history, so CI renders live M1 numbers (copy/paste, churn, hotspots, bus-factor, coupling) plus the instruction reference-validity and doc↔code staleness trends into `QUALITY.md`, replacing the all-"not measured" placeholder board — with committer identities hashed in the ownership shares so no raw email/slug reaches the published artifact (#272).
- The provisioner gives each service account its role icon instead of seven indistinguishable Gravatar identicons. Since a group Owner cannot set a bot's avatar (`PUT /users/:id` is admin-only), each account sets its own via `PUT /user/avatar` at the moment its token is minted, reading that token from its `0600` file through a `curl` config file so it never reaches argv. `--avatars-dir` supplies your own icons, `--no-avatars` skips the step, and `--avatars-only` re-skins an existing fleet from the token files already on disk without minting or creating anything. A missing icon is a notice, not a failed run.
- The walk separates the two populations that a tier discussion tends to blur: the tier ceilings the 2026-08-30 CE ruling already anticipates (identity-granular protected branches, enforced approval rules, audit events, push rules, custom roles, the server-side token-lifetime policy) from four controls that need no licence and were simply never applied on the pilot deployment — most importantly `Allowed to merge` on `main`, left at Developer, which lets every service account merge and so collapses the human-merge-only fallback the whole CE posture discharges onto.
- `assay-statusgen.yml` gains a `model-autoflip` job: after each push to main, `statusgen --auto-flip-model` advances `gate: model` briefs from `verified` to `done` only when the reviewer App's approval sits at the merging PR's head; anything it cannot corroborate stays `verified` and fails the run loudly.
- `deskpr edit --body-file F [--title T]` corrects an OPEN pull request's own body, and optionally its title, through the gates `deskpr create` already runs over the text it publishes: the secret scan with the same audited `--force-scan-override`, the exactly-one-trailer grammar, the public-repo self-containment scan, the write rate limit and the public-repo `+1` gate, plus the kill switch and loop-identity checks every outward verb faces. It writes an audit row, pushes nothing, and finds the PR the way `update` does — so a branch with no open PR, and equally a merged or closed one, is a refusal rather than a write. Before this, a rework finding of the shape "the PR body says X, it should say Y" had no desk verb at all: a worker either left the description wrong or fell back to a raw `gh pr edit` that ran no gate and left no record.
- `deskpr edit` posts one short comment on the PR naming which surfaces changed. A body or title edit moves no head SHA, so a review monitor keyed on the head records no event for it and the correction is invisible to the loop that has to act on it; the comment is that event. An edit that lands while its notice cannot be posted exits 6 stating both facts, because exit 0 would claim the review desk had been told when it had not.
- `docs/contracts.md` documents the three-part schema-first contract pattern (versioned machine-readable artifact + source-side coverage gate + consumer-side conformance run), written over the brief-v1 frontmatter contract and parameterized for the consumer install seam as a second instance.
- `docs/records-and-retention.md` states, in one per-class table, which artifacts in this repo are records rather than documents — register entries, briefs and their `## Evidence` sections, stream README `Verified`/`Reviewed` cells, generated views, execution witnesses, released artifacts and checksums, exported evidence bundles — naming where each lives, who may write it, how an unintended alteration is detected, what mechanism enforces that, and how long it is kept. States the retention rule already in force (kept for the life of the repository; withdrawal is a tombstone, never a deletion) as a description of current practice, and is explicit that an adopter's own retention period, disposition, and legal-hold obligations are theirs to state. Cross-linked from `docs/registers.md` and registered in a new `freshness.yaml` for periodic re-review against the sources it depends on.
- `docs/streams/forge-gitlab/pilot-report.md` records the first live GitLab pilot: a free-tier gitlab.com group provisioned with seven service accounts, an Assay tracking root seeded and reviewed through the role chain as merge requests, and spec §3's security-parity table walked control-by-control against live API reads rather than against the runbook. Every row cites an endpoint, an id, a SHA or an HTTP status; anything the run could not observe is recorded as `could-not-check`.
- `internal/runnertable` is extracted from `cmd/verifyloop` (behavior-preserving — its own tests still pass) so the pinned tier->runner table has a second consumer, plus a new pinned DECIDER runner entry with a boot-time containment attestation for the comms prose router and outbound gate.
- `qualgen/reflex` graduates quality's M4 methodology-reflexivity layer (spec
  §7): gate-yield accounting (§7.1) joins per-lane pre-merge catches against
  M3-attributed escapes into catch-rate/escape-rate/latency-cost readouts, and
  the ritual-effectiveness natural-experiment joins (§7.2) score cost per
  durable KLOC by model tier × brittleness band and Verify-depth vs escape
  rate — every readout routed through a single brittleness-band-stratified,
  confounders-carrying emit gate (`stratify.EmitRitual`) so a natural
  experiment is never presented as a causal claim, with three-state
  could-not-measure/could-not-join resolution throughout (quality/12).
- `qualgen/riskscore` graduates a learned JIT defect-prediction model (Kamei-style
  diffusion/size/history/author features, temporal-split logistic regression)
  that always carries the §9.1 hand-weighted heuristic decomposition alongside
  it as fallback and explanation — a future-leak-proof training split, a
  could-not-learn fallback under a thin corpus (never a fabricated learned
  zero), and an honest learned-vs-heuristic comparison on held-out AUC
  (quality/15).
- `statusgen enforcement-status` renders the live authoring-guidance rules the lint actually enforces — derived from the lint registry and reported three-state (enforced / not-enforced / could-not-check) so the coverage boundary is explicit — and a new `skillslint` `ENFORCEMENT-BLOCK` check compares that fresh render against the committed enforcement block in the authoring-guidance skill, failing closed when the two drift, so documented guidance can no longer silently diverge from what the lint enforces (mistake-proofing/04).
- `statusgen` gains typed Verify-row OBLIGATION classes (`+mutation`, `+flow`,
  `+dereference`, `+neighbour`) as a second closed set on the `Class` cell,
  orthogonal to the existing WHO-EXECUTES values and encoded in a compound
  `<execution> +<obligation>` cell so the table's column set is unchanged and the
  legacy column-less hinge is untouched; an unknown obligation token is FATAL
  exactly as an unknown class is. A diff-scoped derivation
  (`statusgen/obligationderivation.go`) reuses the existing consumer-routing
  branch-diff helper — no new diff machinery, no network reach — to evaluate only
  a brief whose own file the branch edited, deriving owed obligations from the
  branch diff, declared paths, and task prose and emitting an advisory NOTICE for
  each owed-but-absent obligation; an unavailable diff is reported as
  could-not-check, never as "nothing owed". Presence is the control; adequacy
  stays the reviewer's call. Lands advisory (`obligationDerivationFatal = false`)
  per the phasing recorded in the source header (mistake-proofing/03).
- `tools/cellctl/cellctl` — one bash script that scaffolds, checks, starts and stops a single Assay cell on one machine: `new` (cell directory + the four custody hand steps it deliberately leaves to a human), `check` (per-precondition ok/MISS, `bin/deskd` and `bin/deskcli` both), `deskd` (attended-only, one installation token per org, a `--once` go/no-go before the persistent run), `desk` (one role window on its own worktree), and `up`/`down` (a tmux cockpit, `the-desk` included by default; `up` carries the operator's attended affirmation to the `deskd` window only when `up` is itself attended — a terminal on stdin or an explicit `CELL_ATTENDED=1` — and otherwise says why it did not stand it). The session keeps the operator's real `HOME` — the harness keys its login, plugins and memory by it — and only the desk verbs see the cell config-home, through a generated `shim/` directory placed first on `PATH`. Every role window launches on a model pinned in `cell.env` (`DESK_MODEL_DEFAULT`, plus `DESK_MODEL_<role>` overrides) rather than the CLI default, and carries one name — `<cell>-<short role>` — as both its roster beacon and its session display name. Documented in `docs/cellctl.md`, with a pointer from the adopter runbook's multi-cell topology section (#327).
- `tools/create-fleet-gitlab_test.sh` — an offline suite that puts a fake `curl` on PATH and drives the script's real control flow with no GitLab, credential or network: the guarded protect step's restore path, the tier fallback, the avatar uploads, and the read-backs.
- `windows-port/00` — a new wave-0 brief for the `_unix.go`/`_windows.go` build-tag split of the eight unix-only `syscall` sites in `statusgen/` and `tools/desk/` (a process-group kill, two `Stat_t` roster-owner checks and five `flock` copies), each Windows variant required to degrade explicitly and visibly rather than silently.

### Fixed
- A failed settings step no longer aborts the steps after it. Every step runs, the failures are collected under the HUMAN-ONLY REMAINDER block, and the script exits non-zero — previously one `400` on the protect step also skipped the pipeline-succeeds gate below it.
- Both settings steps are now judged on a read-back rather than on a status code. The three decided protected-branch fields are read back and printed at provisioning time, and approval settings that a tier silently ignores — a `201` that stores nothing — are reported as `failed-at-tier` instead of trusted, with a further notice where the read-back itself cannot be believed because approval rules are unavailable.
- Every outward desk verb (`deskflip`, `deskpost`, `deskpr`, `deskreply`, `deskfile`,
  `deskevidence`) refuses when `$DESK_LOOP` is unset, in the same words `deskboot` already
  used. With the variable unset a `STOP.<loop>` flag a human is holding matches nothing, so
  the halt silently failed and the verb kept writing.
- `deskboard prs` reports a watched repository the App installation cannot resolve as
  per-repo could-not-check (`repoCoverage` in the JSON, a `COULD NOT CHECK` line on the
  table) instead of failing the whole sweep. That failure is permanent for such a repo,
  so aborting on it cost the board every other repo's rows. Every other read failure —
  401, rate limit, timeout, parse — still fails the run closed.
- `deskboard`'s READ path now authenticates as the session role's GitHub App
  installation instead of falling through to the HOME keyring: every `gh` call is
  handed the cached installation token for the account the call targets, resolved once
  per account. Under a desk config home that is not the operator's own, the reads
  previously came back 401 for every private repository — and on the GraphQL path an
  unusable keyring account can return a bogus rate-limit error or an empty result, an
  absence that reads like an answer. A session with no loop identity keeps the ambient
  credential and says so on stderr; the outward verbs take the opposite rule.
- `deskdispatch`'s `worktree-create` step reports what `deskwt` actually said, whole. It quoted only the FIRST line of the child's stderr — which is always the effective-config echo — so a branch collision reached the operator as `worktree-create failed (assay-config: …)` and sent them chasing a phantom claim problem for several acquire/steal cycles. The step now drops the known preamble (the config echo, the unpinned-build warning) and prints the tool's own message verbatim, and a `deskwt` **refusal** passes through as a refusal (5) instead of being flattened into unverifiable (6) — a decision the operator can act on, not a state to retry.
- `deskflip` refuses (exit 5, naming the role and the token path) when the review role's
  App installation token cannot be minted or read, and every forge call it makes runs
  under that token. It previously proceeded on the operator's ambient `gh` login, so the
  ready-flip and its queue labels were written under a human identity and read afterwards
  as a human decision.
- `deskflip`'s test binary now installs its fixture roster in `TestMain`, the way every sibling command package does, instead of relying on the roster each test plants as its first statement. Three identity tests built their stub with a helper evaluated in the composite literal — before that statement ran — so they resolved the reviewer-role binding from whatever config home the machine happened to have: green on a developer's box with a real `roster.env`, red on any runner without one. The release lane's `go test ./...` was the only gate that ran these tests at all, so the whole desk-tools leg failed there with "the fixture roster does not bind the reviewer role" while the PR-time lane (build + vet only) stayed green.
- `deskwt add` no longer dies on a stale local branch. Worktrees share one refs store, so a branch left behind by an abandoned dispatch (`git worktree remove` does not delete the branch it was on) blocked every later `add` that derived the same name, as a bare exit 6. The collision is now resolved by name: a leftover that is checked out in no worktree and carries no commit its upstream (or `--base`) lacks is **reclaimed** — deleted with a compare-and-delete against the sha the proof was taken on, then recreated — with an audit line naming the branch, its old sha and the ref it was measured against. A branch **checked out in another worktree** is refused (5) naming that worktree's path; a branch **ahead** of its comparison ref is refused (5) naming the commit count, because unfinished work is not this tool's to delete. There is still no force verb anywhere in `deskwt`.
- `docs/adopting-assay-gitlab.md` §0.1 now states the ruled edition stance (#219): Free / Community Edition is conforming for the core lane with its degradations disclosed — the earlier wording ("a pilot lane, not a conforming deployment") contradicted the ruling. The disclosed-degradation table grows from the two rows the edition matrix named to the seven the live pilot measured, each with what stands in for the control on Free and the tier that closes it.
- `tools/create-fleet-gitlab.sh`'s protected-branch step no longer leaves the tracking root writable when it fails. It reads the group's tier and sends the Premium `allowed_to_*` arrays only where they exist — below that the three fields Free actually accepts, reporting the omitted push allowlist as `failed-at-tier, remediation: Premium` — and it never removes the existing rule before the replacement is known to apply: an already-correct rule is a no-op, a force-push-only difference is a `PATCH`, and where a delete-and-recreate is unavoidable a refused re-create immediately re-applies the rule that was read. The intended `merge_access_level` is named as 40 (Maintainers) in that recovery path, so a hand repair cannot reproduce the 30 that let every Developer service account merge its own merge request.

### Changed
- Deviation D-6 is rewritten around the part that generalises beyond this deployment: an unprotect-then-fail defect does not only leave a branch open for a window, it hands a human a manual re-protection to compose against an API whose free-tier field set differs from the script's. Getting `push_access_level` right while getting `merge_access_level` one level too low is both the natural mistake and an invisible one afterwards — the branch still reads "protected" and `can_push` still reads `false`. The provisioner's fix should therefore re-create the rule with the intended levels and read all three fields back, so neither the failure path nor the recovery path can leave a weaker rule than the script would have written.
- Recorded from the final merge: with no pipeline configured, GitLab asks the human to confirm merging unverified changes. That prompt is the only place in the whole round trip where the absence of the pipeline gate becomes visible to a person — everywhere else a merge request with no checks at all looks exactly like one whose checks passed, which is the state an adopter inherits by default from the provisioner's early exit.
- The `changelog-check` PR gate now enforces the fragment convention (`changelog/<slug>.md` with at least one highlight bullet, or `changelog:skip`) and refuses direct `## Unreleased` edits; `release.yml` aggregates the fragments into the release highlights, refuses to cut with nothing to aggregate, and rolls them into a dated `CHANGELOG.md` section in the release commit.
- The `windows-port` stream's "the Go binaries are already portable, no source change needed" premise is retired: it was measured on 2026-08-07 as a harness claim, not a `GOOS=windows` one, and neither module cross-compiles for Windows today. `windows-port/01` (release build matrix) keeps its two-file scope but now depends on `00`, and the stream's critical path becomes `00 → 01 → 03 → 05`. Planning docs only — no tool behavior changes yet.
- The brief's round trip is recorded step by step with actor, mechanism, artifact and timestamp: the seed merged by the human, then the worker's deliverable and `Draft:` merge request, the reviewer's head-pinned verdict note and approval (author and approver are different service accounts, dereferenceable from the ids), and the desk identity's ready flip. Verify row 4 remains `could-not-check` — board regeneration is gated on the next human merge, and is recorded as not run rather than assumed.
- The dispatched REVIEWER kit now requires every path-existence claim to be resolved in the
  PR's OWN repository at the PR head, to name the tree it was resolved against, and to be
  reported as could-not-check when it cannot — a reviewer that checked paths in the
  dispatching desk's checkout reported files as missing that were present in the PR's
  repository.
- The loop-to-App-role table moved into `deskkit`, so the identity a window presents and
  the identity its calls carry are read from one place.
- The post-merge half of the round trip is recorded and, more usefully, its shape is: the verifier's Evidence row and the board regeneration both have to travel as merge requests, because `main` is push = No one for every identity, and neither has a GitHub-style direct-to-main carve-out. The pilot shows the cost is containable by stacking rather than serialising — the board's change targets the Evidence branch, so the pair is one human sitting, and the stacking is forced by the data (the board is derived from the register the Evidence flips) rather than chosen for convenience.
- The round trip completed: one brief went `todo` → `verified` on a Free-tier group through five distinct role identities and four merge requests, and all four of the brief's own Verify rows are now checked-clean against the live system. The clearest single piece of evidence is the tracking project's history, which the report renders as a table — every content commit is a distinct role service account, every merge commit is the human, and no identity appears in both columns. That alternation is the CE posture rendered as history rather than as settings, and it is also why the merge-access misconfiguration mattered: for the first two merges of that log it was a convention the server was not enforcing.
- The trust-roster gap is now demonstrated on a real row rather than on an empty roster: with a distinct, non-implementing verifier service account's Evidence committed, lint still reports the row as backed by no accepted verifier actor. A correctly-verified GitLab row is indistinguishable from a self-attested one, because the check has no GitLab identity to recognise.
- Two further deviations recorded from the live run. The execution-witness generator stamps a runner derived from the local machine rather than from the forge identity that acted, so on a non-GitHub deployment the witness row and the Evidence row disagree by construction — annotated in place rather than hand-corrected, since editing a generated witness is the manufactured evidence it exists to prevent. And a service account's avatar can only be set from that account's own credential on SaaS (`PUT /users/:id` as owner returns 403, `PUT /user/avatar` as the bot returns 200), a shape the provisioner's single-owner-token model does not have.
- `docs/adopting-assay-gitlab.md` no longer tells free-tier groups to stop: a measured §0.1 records that the full identity model provisions on gitlab.com Free (service accounts are free since 18.11) while the write-path controls (board-writer allowlist, required/prevent-author approvals, token-expiry policy, audit events) are failed-at-tier with their Premium remediation — a pilot lane, not a conforming deployment.
- `docs/adopting-assay.md` now recommends the App permission set the desk tools actually require. The reviewer App's `contents: read-only` recommendation is **retired**: the boot preflight's `app-scopes-vs-duties` check applies one uniform `requiredDuties` set — `pull_requests: write`, `issues: write`, `contents: write` — to every role and refuses to run a role whose App lacks any of them, so the old advice provisioned a reviewer that could not boot. The guide states plainly why: *author ≠ approver* is held by the reviewer being a distinct identity nobody without its private key can post as, and by the forge rejecting a self-approval — not by a withheld `contents` scope, which never governed review posting at all. A deployment that wants to separate "may review" from "may land on the default branch" is pointed at branch protection instead. The `setup-reviewer-app` Verify clause that asserted the *absence* of `contents: write` is replaced with a positive check of all three duties plus a `--fresh` re-mint caveat (a cached token re-reads its old grant). The narrower CI-read trio (`checks`/`statuses`/`actions: read`) stays role-scoped, and the "do not grant it to verifier / inbound-lane Apps" guidance is unchanged.
- `docs/streams/forge-gitlab/pilot-report.md` records the round trip through the ready flip and closes the finding it opened. The `Allowed to merge` misconfiguration the walk found on the pilot's `main` was repaired mid-run — the protection rule was re-created with `merge_access_level: 40` (Maintainers), and every service account now reads `user.can_merge: false` while the human owner reads `true`. Row 10 carries both reads rather than being quietly re-measured, and the overall verdict is recounted to four PASS, two could-not-check, eight failed-at-tier.
- `spec/brief-v1.md` is brought current with the reference validator: the "Describes reference implementation" header now names the released `statusgen` version instead of a long-stale one, and the five optional frontmatter fields the validator has grown but the spec never documented — `domain`, `blocked-by`, `homed-in`, `measures`, and `parallel-streams` — are now specified with their exact value sets and flagging rules.
- §2 now says where the role-token store is (the config-home the desk verbs read), that the script's `<prefix>-<role>-bot.token` files must be linked to `desktoken`'s `gitlab-<role>.token` names, that the owner PAT is a legacy `api`-scope token (fine-grained unproven), and how to recover `main` when the free-tier protect step fails after unprotecting it.

## v0.23.0 — 2026-09-02

### Added
- A model-capability floor gates authority-bearing desk writes: `deskflip`, `deskpost ready`, and `deskpost` review verdicts now refuse a session whose dispatch is attested below the strong tier — keyed on the dispatcher-applied model+tier label stamp (self-applied stamps are worthless), failing closed. Unattested (human / pre-attestation) lanes proceed with a notice, and an incident-recovery override is logged loudly (#278).
- A peer-auth desk-comms backbone lands (`tools/desk/internal/comms/`): a `cellmsg-v1` envelope that parses-or-refuses, ed25519 sender-identity assertions (mint/verify, single-use, TTL-bounded), and a compiled lane ACL that is deny-by-default — cross-cell reach and human-gate verbs ship refused until a recorded ruling (#276).
- New `qualgen/dorajoin` package: the DORA join — a quality denominator (durable-change volume) and a traced-CFR refinement reported alongside incident-based CFR, joined to a pluggable `DeliveryMetricsSource` (a file-based reference adapter ships in-tree) on PR number / merge SHA / stream-task-ID, three-state throughout and never emitting a bare traced rate without its trace-rate and evidence-tier split.
- The changelog discipline is ACTIVE: the changelog-check PR leg gates merges and release.yml refuses an empty Unreleased section, lifting highlights into the release body (#266 activation, #269).
- `deskcomms` gives the desks their client surface onto the local cell gateway: `send` runs a fail-fast preflight (reserved-verb → identity → parse → lane-ACL → bodycheck → ratelimit → mint → submit) that CALLS the same `internal/comms` parse/ACL the gateway re-runs authoritatively, then signs and submits; `poll`/`ack` read this session's own per-role mailbox (ack moves, never deletes). Sender identity comes from session context, never a flag; enforcement stays the gateway's; there is no local-spool fallback, so an unreachable gateway fails closed rather than fabricating delivery (#299).
- `deskpost` attaches mechanical verdict-time triage labels to agent PRs — a `size:S/M/L` class over changed lines (generated files excluded) and a three-state `surface:core/std` tier read from a repo's `.assay-surfaces` globs — advisory only (nothing gates on them; an unreadable surface is could-not-check, never assumed) (#277).
- `qualgen check <paths>` screens named files for brittleness signals (stronger-tier, add-coverage, coupling-partner, reference-rot) as an always-advisory, exit-0 pass over the mined M1/M2 families — the per-file complement to the corpus-wide mine (#275).
- `qualgen pr <n>` emits a generic per-touched-file risk-feature feed (hotspot percentile, traced defect density with its trace-rate, ownership top-share, missing coupling partners) as JSON — no weighting or combined outcome of its own, so a consumer's own config decides what to do with the numbers.
- `qualgen/attribution` implements M3 stage attribution: it assembles a deterministic, content-addressed dossier per traced defect and names the stage the defect escaped at — `spec` / `brief` / `implementation`, or `untraceable` when the provenance chain is broken (never binned into a stage) — plus a `review-escape` overlay naming the lanes that approved the inducing change; the stage call is judgment-classified and spot-auditable against the fixed dossier, with a pluggable provenance-linkage adapter (a generic commit→issue reference adapter ships as the default) and an append-only per-stage defect ledger correctable only by tombstone amendment.
- `qualgen` mines the instruction-brittleness M1 family for real: instruction reference-validity and doc↔code staleness now render into `QUALITY.md`'s trend view behind a new `--instruction-globs` flag, replacing the family's placeholder — and an unconfigured run reports could-not-measure, never a silent zero (#271).
- `qualgen`'s M4 session-forensics join lands: a pluggable `TelemetrySource` interface plus a file-based reference adapter (`qualgen/telemetry`), and a read-only join over the M1/M2 corpus (`qualgen/m4`) correlating harness telemetry (retries, refusals, …) against churn and defect outcomes, with three-state coverage reported beside every correlation — code only, no telemetry source wired in (quality/13).
- `skillslint` also emits an advisory context-budget `NOTICE` for any instruction file over a word threshold (3,000 for `SKILL.md`, 5,000 for `CLAUDE.md`), flagging context-bloat candidates. The NOTICE is advisory only — it prints to stderr and never changes the exit code.
- `skillslint` now runs a byte-level invisible-character / Trojan-Source lint over the instruction surfaces (`plugins/assay/skills/**/*.md`, and `.claude/skills/**/*.md`, `plugins/assay/resident-rules.md`, `CLAUDE.md` where present). It rejects — by Unicode category, not an enumerated blacklist that would miss members — the whole `unicode.Cf` format category (bidi controls and directional marks incl. LRM/RLM/ALM, zero-width, invisible math operators, soft hyphen, the Unicode Tag block used for LLM ASCII smuggling, and non-leading U+FEFF), the variation selectors (U+FE00–U+FE0F, U+E0100–U+E01EF, and the Mongolian free variation selectors U+180B–U+180D/U+180F), a curated set of other invisibles that are neither Cf/VS/Cc (U+034F combining grapheme joiner, the Hangul fillers U+115F/U+1160/U+3164/U+FFA0, the Khmer inherent vowels U+17B4/U+17B5, U+2800 braille blank, and the U+2028/U+2029 line/paragraph separators), the assigned Unicode Default_Ignorable_Code_Point property as its own durable property-based branch (so every assigned-DI codepoint flags even if reclassified out of a category), any C0/C1 control outside tab/newline/carriage-return, and invalid UTF-8 — each reported with file, line, column and codepoint. Unassigned/reserved Default_Ignorable and visible Zs space separators (ordinary space, NBSP, …) are deliberately left legal. Printable non-ASCII (accented text, arrows, box drawing, an emoji whose base glyph carries its own presentation) stays legal — the check targets invisibility, not foreignness. This catches the exact payload class a human reviewing the rendered text cannot see.
- `statusgen conform` validates brief frontmatter against a versioned, machine-readable brief-v1 contract (`schemas/brief-v1.json`, JSON Schema draft 2020-12) embedded in the binary — required keys, field types, and closed value sets, reported three-state (checked-clean / checked-failed naming file+field / could-not-check, fail-closed) and distinct from `--lint`'s methodology rules; a `schema:` marker newer than the embedded contract is a version mismatch, not a field error. `conform --emit-schema` prints the embedded schema so the artifact is reproducible from any pinned binary, and a source-side coverage test derives the required-key and value sets from the reference validator's own tables so the schema and validator cannot drift without CI failing.
- `statusgen` gains a DECLARED, fail-closed fixture-corpus exclusion: a directory that drops a `.statusgen-fixtures` marker at its root opts its whole subtree out of both the `--lint` link check (dead-link / backticked-path / identifier-dereference / register-ref) and `--corroborate`'s `human:<name>` stamp scan, so eval/fixture corpora of captured run-outputs stop redding on their legitimate forward-references. The exclusion is DECLARED (marker-only, never inferred from a path name or `testdata`/`fixtures` convention) and FAIL-CLOSED (no marker on disk → the subtree is scanned exactly as a live brief); live briefs are untouched.
- `tools/create-fleet-gitlab.sh` idempotently provisions the Assay fleet's seven per-role GitLab service accounts, memberships, and PATs, plus a project's protected-`main` and approval settings; paired with `docs/adopting-assay-gitlab.md`, the GitLab-profile adopter walkthrough (ci-config-project runbook, token custody, tier ladder) cross-linked from `docs/adopting-assay.md` (forge-gitlab/04, #288).

### Fixed
- The desk's CI-rollup readers now evaluate the LATEST run per check NAME, mirroring branch protection's own "latest run per context" rule, so a superseded run — an older CANCELLED predecessor, or a stale QUEUED orphan left by a push + pull_request double-trigger — no longer counts against a PR whose current run for that name is green. This lands as one shared `deskkit.LatestRunPerName` reducer called by all three surfaces — `deskflip`'s ready-flip gate, `deskboard`'s CI-state render, and `deskkit.ReduceCIVerdict` — so the flip gate and the board can no longer diverge on the same double-triggered PR (one flipping it ready while the other still renders it CI-fail). The gate is not relaxed anywhere: a name whose current latest run is red, cancelled, or pending still reddens or blocks (#282, #289).
- `statusgen --record`'s DORA-timing recorder no longer fails silently when its authenticated `gh` reads (restore episodes, PR lead times) all fail — it emits a loud, distinct `DEGRADED` signal naming the failed read and the substrate path, instead of returning a no-op indistinguishable from a healthy quiet day (so a persistently token-less `--record` CI can no longer leave `.dora-timing.jsonl` silently never accruing); still fail-open, never fabricates (#279).

### Changed
- Changelog highlights are now recorded as per-PR fragment files under `changelog/` instead of shared `## Unreleased` edits: `changelog-check` greens on a fragment (or `changelog:skip`) and refuses a direct `## Unreleased` edit, and the release workflow aggregates fragments into the dated section and release Highlights, then clears `changelog/`.
- The `assay:verify-desk` skill body gains three neutral verification-quality controls — derive-from-base-branch grounding (derive what should exist before reading the work), per-row fan-out for large Verify tables (≥4 risk-bearing rows run as isolated per-row sub-verifications), and Evidence↔Verify-row scope-traceability (unmapped verified work is flagged as invented scope) — plus an anti-gaming rule to re-derive expected values from the brief rather than the work under test.

## v0.22.0 — 2026-08-31

### Added
- CI grows five control legs (#255): a forge-surface control sweep, a leak-sweep
  pattern sweep, per-plugin shell suites, a gating skillslint leg, and a
  QUALITY.md render check — each exercising a control that `go build`/`go vet`
  alone would leave un-run.
- A quality trend view: churn-vs-durable, hotspot and brittleness reporting land
  behind a single-writer `QUALITY.md` (quality/01–06: #245–#248, #252, #254).
- A per-loop pool-width knob for the desk loops (#226).
- A deterministic verdict runner (#242).
- Roster-from-deployment resolution (#256).
- A PR-body self-containment scan (#227).
- `inbox --flow` / `--walk` / `--html` views (#225, #233).
- A two-role superseded lane for `deskclose` (#232).
- B-SZZ inducing-commit tracing plus derived defect metrics land in `qualgen`
  (#261).
- Spec-routing §8 spec-lifecycle enforcement: a linter and an authoring-owed
  emitter (#267).
- The CHANGELOG discipline itself — a per-notable-PR `## Unreleased` highlight
  line, with the release-time roll and its CI enforcement staged (#266).

### Fixed
- The board archives cleanly: statusgen now resolves streams under
  `docs/archive/` as known depends / unblocks / affects targets (#259), so
  archiving a finished stream no longer reds valid references from still-active
  work.
- The verify queue stops lying: `verifyloop` defers `blocked-until` briefs,
  buckets online-lane and human-gated work out of DISPATCH, and reads
  qualifier-carrying `## Verify (…)` headings (#251, #253, #257).
- Board regeneration no longer races on concurrent pushes: regen-push is
  serialized (#221).
- A latent drift-registry test red is fixed (#262): `statusgen`'s
  `blockingIssueLabels` is registered as a declared exception, greening the
  release-only (desk-tools) test leg.
- The archive fallback extends to the markdown link/backtick check (#264), so
  references into `docs/archive/` stay green there too.
- `muhar -j 0` auto-parallelism is capped at 2 mutants in flight (#268) — the
  release test leg is memory-bounded by construction; every mutation still runs.

### Changed
- `pr-shepherd` is de-housed into the `assay` plugin so adopters get it too
  (#234).
- GitLab forge support closes out with a forge tier matrix (#222, #230, #231).

### Consumer action
- Pin `statusgen` at ≥ this release to lint boards that reference archived
  streams under `docs/archive/` (#259).
