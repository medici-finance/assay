---
brief: assay:assay:forge-neutral:34
title: deskread CI workflow-token transport — an explicit, CI-only, read-only opt-in beside the App custody default
why: >-
  statusgen's CI-only modes (the pull-request corroboration job, the model auto-flip job, and
  the transcribe lanes an adopter runs) read the forge under the workflow's own job token today,
  through `gh`. forge-neutral/18 has to move every one of those reads onto `deskread`, but
  `deskread` only ever authenticates as a minted desk App role, and the CI jobs hold no desk
  role custody that `deskread` can resolve. So 18 cannot finish without either giving every CI
  job an App credential for a read role or giving `deskread` a narrow way to read under the token
  the job already holds. The driver's ruling on #2253 chose the second. This brief adds that way
  and fences it: opt-in per call, refused outside CI, refused for anything that writes, bound to
  the job's own repository, and recorded in the output, so the desk's no-ambient-token rule keeps
  holding everywhere else.
wave: 1
depends: []
unblocks: ["forge-neutral/18"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief relaxes a security posture on purpose. Today `deskread` refuses to read under any
  credential it did not mint for a named desk App, and that refusal is what keeps a read verb
  from silently running as whatever token happens to sit in its environment. The brief adds one
  exception: under an explicit flag, inside a CI job, for read-only kinds, against the job's own
  repository, `deskread` reads with the workflow's job token handed to it in a dedicated
  variable. That token is a credential (sensitive-data: yes). The human confirms five things.
  (1) The opt-in is explicit and never a fallback: no flag means the App custody path runs
  exactly as today, and a flag with no token is refused, not degraded. (2) The threat model is
  accidental or ambient use in an honest CI job, not a local caller who deliberately forges the
  CI environment. Against such a forger only the read-only forge and the closed kind set hold:
  the CI check, the repository binding and the run id all come from the same forgeable
  environment, the token-shape check proves only "not a personal or OAuth token" and passes every
  App installation token, and the job's read-only `permissions:` block binds only a genuine job
  token, and only in this repository's two statusgen jobs. The residual (reads, through
  `deskread`, under an installation token the forger already holds) is acceptable. (3) The
  identity record (transport, repository and run id in the envelope, never the token) tells which
  transport an honest CI read took; its repository and run id are unverified claims copied from
  the environment. (4) Under the flag inside CI, `deskread` loads its roster from the job's
  environment instead of the config-home file, so it can activate in a CI job with no config
  home. (5) The ambient-token ratchet rises from 4 to 5 permits against its stated target of 0.
decision-trigger: creation
decision-issue: 2315
issues: [2315]
schema: brief-v2
version: 2
outcome: none
id: a6651bd7-de82-48d4-b80e-38c7d6663222
authored: 2026-10-06 by forge-neutral authoring session (the #2253 ruling); revised 2026-10-06 for the correctness and security reviews of #2314
sources:
  - "#2253 comment 6013956713: the question this brief answers. statusgen's CI-only modes (`--corroborate`, `--auto-flip-model`, the transcribe lanes) read under the workflow's job token, `deskread` reads only under a minted App role, so moving those reads onto `deskread` needs a decision about which credential CI reads under"
  - "#2253 comment 6014345604: the driver's ruling on #2253, option (a). `deskread` gains an explicit, opt-in transport that authenticates with the CI workflow token, for read-only kinds only, so the CI-only modes can move their `gh` reads onto it. The ruling asks for this as its own gated brief"
  - "#2315: the decision issue that puts this brief's exact shape to the human"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: Verify row 3 (zero forge-CLI launch sites in non-test statusgen), which the CI-lane sites cannot meet without this transport, and row 15 (`TestForgeSurfaceUnchangedByDeskread`), which this brief must keep green because it adds no `Forge` operation"
  - "#2313 (draft, forge-neutral/33): the `Forge` reads and `deskread` kinds statusgen's remaining sites need. Its scope fence leaves `deskread`'s identity untouched and defers the CI identity to #2253. This brief is the other half: it adds the identity, no reads"
  - "docs/streams/forge-neutral/README.md, 'Shared conventions the briefs inherit': refusal not fallback; negative-path rows are mandatory; no hand-built API call is evidence"
  - "tools/desk/cmd/deskread/forge.go:46-51: the only identity path today, `sessionRoleFn(\"deskread\")` (= `deskkit.SessionTokenRole`) then `deskkit.ForgeFor`, with an Unverifiable refusal that names 'never an ambient forge-CLI identity'; :19-24 and :28-35: `forgeAPIBase`, a test seam that is empty in production, handed to `deskkit.SetGitHubCustodyMinter`"
  - "tools/desk/cmd/deskread/main.go:74 (`readKinds`), :81 (`perIssueKinds`), :127 (`SetToolClass(ClassForTool(false))`, roster from the config-home file only), :129 (`CheckVerbActivation` before `run`)"
  - "tools/desk/internal/deskkit/activation.go:384 (`CheckVerbActivation`): refuses an inactive owning component; tools/desk/component.yaml requires `assay.roster.trust`, which resolves only when the roster loads as configured. Checked offline at e6cb7d2a0: `deskread issues` built from this tree, run from the checkout with an empty config home and the CI variables plus roster variables in the environment, exits 6 with `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`"
  - "tools/desk/internal/deskkit/forgeresolve.go:289 (`SetGitHubCustodyMinter`), :302 (`custody`), :316 (`githubCustody`, whose base comes from the installed minter or is empty, meaning the default), :730 (`ForgeFor`), :743 (`ResolveForge`), :765-767 (the one construction site, each backend wrapped by `OutboundChecked`)"
  - "tools/desk/internal/deskkit/forge.go:37 (`GitHubAPIBase`, the single home of the GitHub host literal) and :46 (`GitHubBaseURLOrDefault`). deskkit has no GitHub Enterprise base configuration: an enterprise base reaches a backend only through an installed custody minter"
  - "tools/desk/internal/deskkit/outboundforge.go:14-22 (the write/read classification, as a doc comment only), :31 (`outboundForge` embeds `Forge`, so every method it does not override delegates), :39 (`OutboundChecked`); tools/desk/internal/deskkit/outbound_test.go:530 (`obClass`, the only machine-readable classification, test-only) and :567 (`TestOutboundForgeWrapsEveryWriteMethod`, which walks `Forge` by reflection)"
  - "tools/desk/internal/deskkit/ambienttoken_guard_test.go:38-41 (`ambientTokenVars`: GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN, GITLAB_TOKEN), :44-46 (the permit register, TARGET: 0), :61 (`ambientTokenReadCeiling = 4`), :63 (`TestNoAmbientEnvTokenRead`)"
  - "tools/desk/internal/deskkit/roletokenguard_test.go:66 (`githubMinterNames`) and :88 (`githubMinterAllow`): the confined-link register pattern Task 5 reuses for the new constructor"
  - "tools/desk/internal/deskkit/rosterconfig.go:960 (`InCI`, which checks GITHUB_ACTIONS only) and :1008 (`ClassForTool`, whose doc comment records the forged-`GITHUB_ACTIONS` residual for a ci-eligible tool)"
  - "tools/desk/internal/deskkit/forgeresolve_test.go:285 (`TestForgeSingleConstructionSite`); tools/desk/internal/deskkit/forge_surface_deskread_test.go:35 (`TestForgeSurfaceUnchangedByDeskread`)"
  - ".github/workflows/assay-statusgen.yml: workflow default `permissions: contents: read` (:66-67); job `corroborate` `contents: read, pull-requests: read` (:115-117), `GH_TOKEN: ${{ github.token }}` (:207), `--corroborate --pr` (:229); job `model-autoflip` `contents: read` (:347-348), an in-job board-writer App token minted at :350-355 (a second installation token in the same job), `GH_TOKEN` (:395), `--auto-flip-model` (:411)"
  - "statusgen/init.go:459-460: the scaffolded adopter workflow sets workflow-level `permissions: contents: write`; :482 hands `GH_TOKEN: ${{ github.token }}` to a step. An adopter's job token is therefore not read-scoped today"
  - "GitHub's documented token prefixes (installation tokens, including the Actions job token AND every other GitHub App installation token, begin `ghs_`; personal tokens `ghp_` / `github_pat_`; OAuth `gho_`; user-to-server `ghu_`). This is a review-only fact: no row can prove a vendor's prefix scheme, and tools/desk/internal/deskkit/bodycheck.go:16 already treats these prefixes as the token vocabulary"
  - "freshness-checked 2026-10-06 @ e6cb7d2a0 (origin/main): 22 `exec.Command(\"gh\"` sites in the CI-lane statusgen files (autoflip.go 8, corroborate.go 4, citationcorroborate.go 2, decisiongateanchor.go 1, trustgate.go 1, transcribescan.go 2, transcribeverdict.go 2, scanissues.go 2); `ambientTokenReadCeiling = 4`; `deskread` has three kinds (`issues`, `trust`, `comments`) and envelope schema 1"
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (c). It decides which credential a desk verb may read under, and the
  dangerous failures pass every happy-path test: an env-only opt-in that turns on whenever CI
  exports a token, a base URL taken from the environment that sends the job token to another
  host, a read-only wrapper that misses a newly added write method, a refusal that quietly
  falls through to the custody path and reports success, or a second caller of the new
  constructor that skips every check.
domain: complicated
consumers:
  - "tools/desk/cmd/deskread: follow-up forge-neutral/34 (this brief's implementation: the `--ci-workflow-token` flag, the CI tool class, the CI-transport kind allowlist, the identity record in the envelope, and the refusals of Task 2-4)"
  - "tools/desk/internal/deskkit/forgeresolve.go: follow-up forge-neutral/34 (this brief's implementation: the second, read-only constructor beside `ResolveForge` with its own shape and repository checks, Task 1)"
  - "tools/desk/internal/deskkit/readonlyforge.go: follow-up forge-neutral/34 (this brief's implementation: the default-deny decorator, Task 1)"
  - "tools/desk/internal/deskkit/forgeresolve_test.go, tools/desk/internal/deskkit/roletokenguard_test.go, tools/desk/internal/deskkit/ambienttoken_guard_test.go: follow-up forge-neutral/34 (this brief's implementation: each guard learns the new constructor, seam or variable and still fails on anything else, Task 5)"
  - "tools/desk/cmd/deskread/main.go `ciTransportKinds`: follow-up forge-neutral/18 (34 opens the set at `issues`, `trust`, `comments`. Every kind a CI-lane site needs beyond those, including the pull-request kinds forge-neutral/33 adds to `readKinds`, is added to `ciTransportKinds` by 18, one reviewed entry per kind, in the same diff that moves the site, with 18's own row asserting the kind is a read on `readKinds`. 33 does not touch this set; its scope fence leaves `deskread`'s identity alone)"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: fixed-here (its `depends:` gains this brief, because its row 3 cannot reach 0 for the CI-lane sites without this transport)"
  - "docs/streams/forge-neutral/README.md: fixed-here (the board row for this brief and the critical-path note `34 → 18`)"
  - "statusgen/forgeread.go, statusgen/autoflip.go, statusgen/corroborate.go, statusgen/citationcorroborate.go, statusgen/decisiongateanchor.go, statusgen/trustgate.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go, statusgen/scanissues.go: follow-up forge-neutral/18 (moving each CI-lane site onto `deskread --ci-workflow-token` is 18's Task and row 3; a read of another repository lands in `partial` and 18 routes it to custody or could-not-check; this brief touches no statusgen file)"
  - ".github/workflows/assay-statusgen.yml: follow-up forge-neutral/18 (handing the job token to `deskread` in the dedicated variable, and supplying the roster variables `deskread` activates from, is 18's workflow edit; the token is never handed over in a `pull_request_target` or `workflow_run` job; this brief changes no workflow)"
  - "statusgen/init.go: follow-up forge-neutral/18 (the scaffolded adopter workflow carries the same `GH_TOKEN: ${{ github.token }}` line, `:482`, under workflow-level `contents: write` at `:459-460`. When 18 moves the scaffold onto the transport it also narrows `permissions:` to read-only on every job that hands the token to `deskread`, with a row of its own. Until then the server-side read-only layer is ABSENT for adopters, and this brief claims it only for this repository's two jobs)"
  - "statusgen/decisionruling.go: out-of-scope (`:643` runs `gh auth token`, which is credential acquisition and not a read; this transport serves reads only)"
  - "adopter scan-transcribe workflows: out-of-scope (they live in adopter repositories; 18's scaffold change is what reaches them)"
---

# Brief 34 — deskread CI workflow-token transport

## Context

forge-neutral/18 moves statusgen off `gh` and onto `deskread`. Most of statusgen's remaining
`gh` calls sit in modes that only ever run inside CI:
- `--corroborate`, in the `corroborate` job of the statusgen workflow;
- `--auto-flip-model`, in its `model-autoflip` job;
- the transcribe lanes behind an adopter's scan-transcribe workflow.

Each of those jobs hands `gh` the workflow's own job token (`GH_TOKEN: ${{ github.token }}`).
`deskread` cannot read under that token. Its only identity path resolves this session's desk
App role and mints that App's installation token, and it refuses everything else by design
(`cmd/deskread/forge.go:46-51`). The CI jobs hold no custody for a desk read role. The one App
token a job mints today, the board-writer token in `model-autoflip`
(`assay-statusgen.yml:350-355`), is that job's write identity for the regenerated board, and
the ruling did not choose to reuse it for reads. So the CI-lane sites have nowhere to go, and
18's row 3 cannot reach 0.

#2253 put that question to the driver. The driver's ruling on #2253 chose option (a):
`deskread` gains an explicit, opt-in transport that reads with the CI job token, for read-only
kinds only. This brief adds that transport and nothing else. It adds no `Forge` operation, no
`deskread` read kind, and it touches no statusgen file and no workflow. Moving the sites onto
the transport stays 18's job, and the reads statusgen still lacks are forge-neutral/33's.

files:
- `tools/desk/cmd/deskread/main.go`: the flag, the tool-class selection, the CI-transport kind
  allowlist, the identity record in the envelope, the usage text.
- `tools/desk/cmd/deskread/forge.go`: the transport switch between the custody path (default,
  unchanged) and the CI path.
- `tools/desk/internal/deskkit/forgeresolve.go`: the second constructor, `ReadOnlyForgeForCIToken`
  (name is a suggestion), beside `ResolveForge`, and its API-base test seam.
- `tools/desk/internal/deskkit/readonlyforge.go` (new): the default-deny decorator.
- Tests beside each: `tools/desk/cmd/deskread/*_test.go`,
  `tools/desk/internal/deskkit/readonlyforge_test.go` (new), and the guard tests named in Task 5.

facts:
- **Identity today**: `deskread` calls `deskkit.SessionTokenRole("deskread")`, then
  `deskkit.ForgeFor(repo, role)` → `ResolveForge` → `custody` → `githubCustody`, which mints the
  role's App installation token. There is no other path and no ambient fallback.
- **Roster class and activation**: `deskread` declares `ClassForTool(false)` (`main.go:127`), so
  its roster comes from the config-home file only. `main` then calls `CheckVerbActivation`
  (`main.go:129`), which refuses with exit 6 when the owning component's required key
  `assay.roster.trust` does not resolve. A CI job supplies the roster only as environment
  variables, so today `deskread` exits 6 in a CI job with no config home before `run` is
  reached. Checked offline at e6cb7d2a0 (see `sources:`).
- **CI detection available today**: `deskkit.InCI()` (`rosterconfig.go:960`) checks
  `GITHUB_ACTIONS=true` and nothing else. Every environment variable, including
  `GITHUB_REPOSITORY` and `GITHUB_RUN_ID`, is forgeable by whoever runs the process.
- **Token shape**: every GitHub App installation token begins `ghs_`. That includes the Actions
  job token, a desk App token minted on a workstation, and the board-writer token minted inside
  the `model-autoflip` job. A prefix check therefore proves "not a personal, OAuth or
  user-to-server token" and nothing more.
- **The one construction site**: `ResolveForge` builds every backend and wraps each in
  `OutboundChecked` (`forgeresolve.go:765-767`). `TestForgeSingleConstructionSite` fails on a
  backend literal anywhere else. `ResolveForge` accepts no caller credential: it obtains tokens
  only through `custody`, and `custody` and `githubCustody` are confined links
  (`roletokenguard_test.go:66`, `:88`).
- **API base**: `githubCustody` takes its base from the installed custody minter, which for
  `deskread` is `forgeAPIBase`, a test seam that is empty in production (`forge.go:19-24`).
  Empty resolves to `GitHubAPIBase` (`deskkit/forge.go:37`, `:46`). deskkit has no GitHub
  Enterprise base setting.
- **Write classification**: there is no runtime classification table. The write/read split is a
  doc comment (`outboundforge.go:14-22`) plus the test-only `obClass` map
  (`outbound_test.go:530`). `outboundForge` embeds `Forge` and overrides only the text-checked
  writes (`outboundforge.go:31`), so the no-text writes and every read reach the backend through
  the embedding. `TestOutboundForgeWrapsEveryWriteMethod` fails when a `Forge` method has no
  `obClass` row.
- **Ambient-token guard**: `TestNoAmbientEnvTokenRead` fails on any function under `tools/desk`
  that reads GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN or
  GITLAB_TOKEN, outside a ratcheted register of 4 permits whose stated target is 0.
- **Server-side layer**: in this repository's statusgen workflow the default is
  `contents: read`; the `corroborate` job holds `contents: read, pull-requests: read`; the
  `model-autoflip` job holds `contents: read`. A genuine job token minted under those blocks
  cannot write repository content, pull requests or issues, whatever the client does. The layer
  binds only a genuine job token, and only in those two jobs: the adopter scaffold sets
  workflow-level `contents: write` (`statusgen/init.go:459-460`).
- **Exit codes** (shared `deskkit`): 0 ok, 3 disabled, 5 refused (`ExitRefused`), 6 unverifiable
  (`ExitUnverifiable`).

single-point-of-failure: against a deliberately forged CI environment, the read-only forge decorator together with the closed `ciTransportKinds` set — behind it, NONE that a forger cannot also satisfy. In front of it sit layers that stop accidental and ambient use in an honest CI job, not a forger: the explicit flag and dedicated variable (no accidental activation), the token-shape check (refuses a person's or an OAuth credential, passes every App installation token), and the CI check, the repository binding and the job's read-only `permissions:` block (the first two read the same forgeable environment; the last binds only a genuine job token, and only in this repository's `corroborate` and `model-autoflip` jobs). NONE is stated rather than designed away: the one signal a forger cannot produce, an Actions OIDC token signed by the forge, needs `id-token: write` on every job that uses the transport (a permissions widening this brief rules out) and a network key fetch before the check. The residual is bounded: through `deskread`, reads only, of the kinds on the list, under an installation token the caller already holds and could use directly.

### Threat model

The transport defends against **accidental or ambient use in an honest CI job**: a job token
that ends up in `deskread`'s environment without anyone asking for it, a person's token handed
over by mistake, a later kind that writes or reads widely becoming CI-eligible without review,
a job token sent to a host the environment names, and a refusal that quietly degrades onto the
custody path. Each of those has a layer and a row below.

It does **not** defend against a local caller who deliberately forges `GITHUB_ACTIONS`,
`GITHUB_RUN_ID` and `GITHUB_REPOSITORY` and passes the flag with an installation token it
already holds, for example a desk App token minted on a workstation. That caller passes the CI
check, the shape check and the repository binding, because it chose the "own" repository. What
still holds is that `deskread` will only read, only the listed kinds, and never under a
personal or OAuth token. The envelope's identity record then says `ci-workflow-token` with the
forger's repository and run id: those two fields are claims copied from the environment and are
never verified.

### Risk-files cross-read

The declared paths sit under `tools/desk/internal/deskkit/` and `tools/desk/cmd/deskread/`, both
security-trigger paths, and the brief touches credential handling. The four answers were read
against them, not defaulted:
- **sensitive-data: yes.** The brief handles a credential: it reads the job token from an
  environment variable and sends it as an `Authorization` header. It never prints, logs or
  stores it, and Verify rows 2 and 10 prove it does not appear in stdout or stderr.
- **regulatory: no.** No regulated data, no compliance surface; the reads are issue, comment and
  review metadata a CI job can already read with `gh`.
- **customer: no.** No customer-facing behaviour changes. Without the flag every byte of
  `deskread`'s behaviour is today's.
- **irreversible: no.** The change is a flag and a constructor, removable by revert. The
  transport cannot write, so nothing it does needs undoing.

gate: human follows from sensitive-data: yes, and independently from the fact that this
relaxes a stated security rule.

## Human decision

A read tool used by the automation today refuses to read under any credential except one
minted for a named automation account. That refusal is what stops it from silently running as
whatever token happens to be in its environment. Several checks that only run inside CI jobs
cannot use this tool, because a CI job holds no such account for reading; they use the job's
own short-lived token through a separate command-line client instead. In this repository those
jobs' tokens are read-only on the server; in the workflow the install scaffold writes for
adopters they are not, until the follow-on change narrows it. The previous ruling chose to let
the tool read under the CI job token, for reads only. This decision ratifies the exact shape of
that exception before it is built.

As briefed, the exception applies only when all of these hold: the caller passes an explicit
flag; the token arrives in a dedicated variable, never the usual token variables; the process
sees the CI environment markers; the token has the shape of an app installation token, not a
personal or OAuth one; the repository read is the one the environment names as the job's own;
the job was not started by a `pull_request_target` event; and the read is one of the read-only
kinds on a fixed list. The forge client it builds refuses every write, and refuses any method
nobody has classified yet.

What those checks are worth is not uniform. The flag and the dedicated variable stop the token
being used by accident. The shape check stops a person's own token, but every app installation
token passes it. The CI markers, the "own repository" and the run id all come from the
environment, so anyone running the tool on their own machine can set them. Against that person,
the only things that still hold are "reads only" and "only the listed kinds". They would be
reading with an installation token they already hold and could use directly, so the exception
gives them nothing new. The output records that the CI token was used, with the repository and
run id; those two values are copied from the environment and are not verified.

Two further relaxations come with the shape and are named here so they are ratified, not
inferred:
- Under the flag inside CI, the tool reads its configuration of trusted accounts from the job's
  environment instead of from a file in the home directory, because a CI job has no such file
  and the tool otherwise refuses to start. Without the flag, it stays file-only.
- The register of functions allowed to read a token from the environment grows from 4 entries
  to 5, against a stated target of 0. The new entry is the one function that reads the
  dedicated variable.

Without the flag, nothing changes.

Options:
1. **Approve as briefed.** The transport is built exactly as above; the CI checks then move
   onto the tool in a follow-on change.
2. **Approve with changes.** Name the change (for example: require a signed CI identity token
   so a forged environment is refused, which needs one more permission on each job; keep the
   tool file-only and have the workflow write the configuration file instead; drop the
   token-shape check; widen to other repositories). The brief is revised before dispatch.
3. **Hold.** The CI checks keep using the separate command-line client; the follow-on change
   cannot finish its last step.

Default if no answer: none — blocks until answered.

## Ground rules

- NEVER git push, trigger workflows, or call a live forge. Every forge interaction in this
  brief's tests is an `httptest` server.
- Stop at `implemented`. You do not set verified or done.
- Add no `Forge` operation and no `deskread` read kind. `TestForgeSurfaceUnchangedByDeskread`
  stays green with its `want` list unedited.
- Touch no file under `statusgen/` and no file under `.github/workflows/`. Moving sites onto the
  transport is forge-neutral/18's work.
- Change nothing on the custody path. Without `--ci-workflow-token`, `deskread`'s identity
  resolution, roster class and output stay byte-identical to the base, except for the additive
  `identity` object in Task 4.
- Never read GH_TOKEN, GITHUB_TOKEN or any other name in `ambientTokenVars`. Never take the API
  base URL from the environment or a flag.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not bypass
  activation, and do not guess.

## Task

### 1. A read-only constructor and a default-deny decorator in `deskkit`

Add, in `forgeresolve.go`, a second constructor beside `ResolveForge`:

```go
func ReadOnlyForgeForCIToken(repo ForgeRepo, jobRepo, token string) (Forge, ForgeResolution, error)
```

It reads no environment variable itself, and it holds its own checks so that a caller cannot
skip them. In order, each before any network call:
1. **Token shape**: refuses (`Refused`) a token that does not begin `ghs_`. The reason names
   what the check proves: "not an app installation token (a personal, OAuth or user-to-server
   token is never accepted)".
2. **Repository binding**: refuses unless `repo.Slug()` equals `jobRepo`, case-insensitive, and
   `jobRepo` is a non-empty `owner/name`.
3. **Forge kind**: resolves it with the same `resolveForgeKind` (no caller-supplied forge) and
   refuses any kind other than GitHub. A GitLab job token is a different credential with
   different scopes, and is out of this brief.
4. **API base**: `GitHubBaseURLOrDefault(ciTokenAPIBase)`, where `ciTokenAPIBase` is an
   unexported package variable, empty in production. Its only setter is
   `SetCITokenAPIBaseForTest(base string) (restore func())`, exported so `deskread`'s flow tests
   can point it at an `httptest` server, and confined to `_test.go` callers by Task 5. No
   environment variable, flag or config key reaches it. `GITHUB_API_URL` is never read.
5. Returns `ReadOnly(OutboundChecked(&GitHubForge{Token: token, BaseURL: base}, "ci-workflow-token"))`.

Add `readonlyforge.go`: a `ReadOnly(Forge) Forge` decorator, **default-deny by construction**:
- The decorator type holds the inner forge in a NAMED field. It does not embed `Forge`, so it
  delegates nothing implicitly. A `var _ Forge = (*readOnlyForge)(nil)` assertion makes a method
  added to `Forge` later a compile error in this file until someone writes it out and decides.
- It implements every `Forge` method explicitly. Each method the `obClass` table marks `read`
  delegates to the inner forge. Every other method returns a `Refused` error naming the method
  and "the CI workflow-token transport is read-only", and never touches the inner forge.
- The classification is not derived at runtime. Completeness comes from the compiler (no
  embedding) plus row 5, which checks every method's behaviour against `obClass` and fails when
  a method classified as a write delegates, or when the type gains an embedded field.

### 2. The opt-in and activation in `deskread`

- Add a boolean flag `--ci-workflow-token`. It is the only switch. No environment variable,
  config key or roster entry turns the transport on.
- Under the flag, read the token from exactly one dedicated variable,
  `DESKREAD_CI_WORKFLOW_TOKEN` (the workflow sets it to `${{ github.token }}`). Empty or unset →
  `ExitRefused` with the `no-token` reason (Task 3), naming the variable. Never read GH_TOKEN or
  GITHUB_TOKEN as a substitute.
- Without the flag, a set `DESKREAD_CI_WORKFLOW_TOKEN` is ignored: the custody path runs, and
  one stderr line says the variable was ignored because the flag was absent. That line names the
  variable, never its value. The token is never used implicitly.
- Under the flag, there is no fallback to custody. A refusal is a refusal.
- **Tool class.** Replace `SetToolClass(ClassForTool(false))` with
  `SetToolClass(toolClassFor(os.Args[1:]))`. `toolClassFor` returns `ClassForTool(true)` when
  the arguments contain `--ci-workflow-token`, and `ClassForTool(false)` otherwise. Because
  `ClassForTool(true)` is `ClassCI` only when `InCI()` holds, the roster comes from the
  environment only under the flag inside CI. Without the flag the class is exactly today's.
  Under the CI transport `deskread` resolves no acting role, so this does not touch the rule
  that acting tools stay file-only. `CheckVerbActivation` stays where it is and is never
  bypassed: under the flag in CI it now sees the environment roster the job supplies.

### 3. Refusals, all before any network call

Under `--ci-workflow-token`, refuse with `ExitRefused` and one reason line on stderr of the form
`deskread: refused [ci-transport:<layer>]: <reason>`. The `<layer>` tag is fixed per refusal, so
each Verify row asserts its own tag and cannot pass on another layer's refusal:
1. `no-token`: `DESKREAD_CI_WORKFLOW_TOKEN` is empty or unset (Task 2).
2. `outside-ci`: `deskkit.InCI()` is false, or `GITHUB_RUN_ID` is not a non-empty run of
   digits, or `GITHUB_REPOSITORY` is not a non-empty `owner/name`. The reason states that CI is
   detected by these variables and that they are not proof.
3. `event`: `GITHUB_EVENT_NAME` is `pull_request_target`. This is a guard against honest misuse
   (handing the token to a binary in a job that runs on behalf of a fork), not against a forger.
4. `token-shape`: the token does not begin `ghs_`. A `ghp_`, `github_pat_`, `gho_` or `ghu_`
   token, or anything else, is refused. `deskread` checks this before building the forge, and
   the constructor checks it again (Task 1).
5. `kind`: the kind is not on `ciTransportKinds`, a closed set in `main.go` separate from
   `readKinds`. At authoring it holds `issues`, `trust` and `comments` (every current kind is a
   read). A kind added to `readKinds` later is NOT on it until a reviewed diff adds it, and a
   test pins that every entry is also in `readKinds`. forge-neutral/18 owns each addition its
   CI-lane sites need (see `consumers:`).
6. `repository`: every target repository must equal `GITHUB_REPOSITORY` (case-insensitive). A
   `--repo` that names any other repository is not read: it lands in the envelope's `partial`
   list with reason "the CI workflow-token transport reads only the job's own repository", and
   is never sent the token. A per-issue kind whose `--issue` names another repository is refused
   outright with this tag. The constructor's own binding (Task 1) refuses the same mismatch if
   `deskread`'s check is ever bypassed.

### 4. The identity record

Add an additive, omitempty `identity` object to the envelope. Schema stays 1: statusgen's
envelope struct declares only the fields it consumes, so an added field is ignored by the
existing consumer.
- Custody path: `{"transport": "app-custody", "role": "<role>"}`.
- CI path: `{"transport": "ci-workflow-token", "repository": "<GITHUB_REPOSITORY>", "runId": "<GITHUB_RUN_ID>"}`.

Write one stderr line naming the transport, the same way, on every run. The token value never
appears in stdout, stderr or the envelope. Update the usage text: the identity paragraph names
both transports, states that the CI one is opt-in, CI-only, read-only and same-repository, and
states that its repository and run id are copied from the environment and are not verified.

### 5. Guards learn the new path and still fail on anything else

- `TestForgeSingleConstructionSite`: allow exactly the one new literal in
  `ReadOnlyForgeForCIToken`, and require it to be wrapped by both `OutboundChecked` and
  `ReadOnly`. Any other backend literal still fails.
- `roletokenguard_test.go`: `custody` and `githubCustody` stay confined to their present callers.
  The new constructor must not call either, nor any other name in `githubMinterNames`.
- **New confinement guard, `TestCITokenConstructorConfined` (planned)**, in the same pattern as
  `githubMinterNames` / `githubMinterAllow`: walk `tools/desk` and fail on any reference to
  `ReadOnlyForgeForCIToken` outside its own definition, except one allow-list entry for the one
  `deskread` function on the CI path. Fail also on any reference to `SetCITokenAPIBaseForTest`
  from a non-`_test.go` file. The scanner takes its root as a parameter, so the negative case
  runs it over a fixture tree with a planted second caller in another `cmd` package and asserts
  it is reported.
- `ambienttoken_guard_test.go`: add `DESKREAD_CI_WORKFLOW_TOKEN` to `ambientTokenVars`, and add
  exactly one reasoned permit for the one `deskread` function that reads it. Raise
  `ambientTokenReadCeiling` from 4 to 5 in the same diff (ratified by the human decision above).
  The permit's reason names the flag, the CI check and the read-only constructor. This keeps
  the new read visible in the ratchet, not hidden behind an unlisted name.
- `TestForgeSurfaceUnchangedByDeskread`: unchanged and green.

## Verify (executable — no prose-only DoD items)

The Class column uses the stream's convention:
- `check:ci` runs in CI.
- `check` is a local or grep-level assertion.
- `+dereference` resolves a claim instead of counting presence.
- `+flow` exercises verb → constructor → backend against an `httptest` server.
- `+mutation` is a mutation demonstration.

Every negative row below sets the environment so that every layer EXCEPT the one under test is
satisfied (flag present, an installation-shaped test token in `DESKREAD_CI_WORKFLOW_TOKEN`,
`GITHUB_ACTIONS=true`, a digit `GITHUB_RUN_ID`, `GITHUB_REPOSITORY` equal to the target, a kind
on `ciTransportKinds`, no `pull_request_target` event), and asserts its own
`[ci-transport:<layer>]` tag on stderr as well as exit 5 and zero server requests. Exit 5 and
zero requests alone are shared by every refusal and prove nothing about which layer fired.

Test names are this brief's planned deliverables:
- `TestCITransportRefusedWithoutFlag` (planned)
- `TestCITransportRefusedOutsideCI` (planned)
- `TestCITransportRefusesWriteMethods` (planned)
- `TestCITransportRefusesUnlistedKind` (planned)
- `TestCITransportRefusesNonInstallationToken` (planned)
- `TestCITransportBindsJobRepository` (planned)
- `TestCustodyPathUnchangedByCITransport` (planned)
- `TestCITransportRecordsIdentityNotToken` (planned)
- `TestReadOnlyForgeRefusesEveryWriteMethod` (planned)
- `TestCITransportFlagWithoutTokenRefused` (planned)
- `TestCITransportPinsAPIBase` (planned)
- `TestCITransportToolClass` (planned)
- `TestCITransportActivatesWithEnvRoster` (planned)
- `TestCITokenConstructorConfined` (planned)

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./cmd/deskread/ ./internal/deskkit/ && go test ./...` | exit 0 |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusedWithoutFlag$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r2.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusedWithoutFlag' "${TMPDIR:-/tmp}/b34-r2.out"` | exit 0, `--- PASS:` printed. **Negative path (a), no opt-in**: every other layer satisfied (CI variables, matching repository, listed kind, `DESKREAD_CI_WORKFLOW_TOKEN` set to an installation-shaped test token), but no flag. The run takes the custody path. The `httptest` server's recorded `Authorization` headers never contain the CI token. Stderr carries the "ignored" line, which names the variable and does not contain the token value |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusedOutsideCI$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r3.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusedOutsideCI' "${TMPDIR:-/tmp}/b34-r3.out"` | exit 0, `--- PASS:` printed. **Negative path (b), outside CI**: every other layer satisfied (flag, installation-shaped token, listed kind, `--repo` equal to the repository the case sets where it sets one). Table cases `GITHUB_ACTIONS` unset, `GITHUB_ACTIONS=false`, `GITHUB_RUN_ID` empty, `GITHUB_RUN_ID=abc`, `GITHUB_REPOSITORY` empty: each exits 5 with the `[ci-transport:outside-ci]` tag and the `httptest` server records zero requests. One more case, `GITHUB_EVENT_NAME=pull_request_target` with every CI variable valid, exits 5 with the `[ci-transport:event]` tag and zero requests |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCITransportRefusesWriteMethods$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r4a.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesWriteMethods' "${TMPDIR:-/tmp}/b34-r4a.out" && go test ./cmd/deskread/ -run '^TestCITransportRefusesUnlistedKind$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r4b.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesUnlistedKind' "${TMPDIR:-/tmp}/b34-r4b.out"` | exit 0, both `--- PASS:` lines printed. **Negative path (c), write kind**: on a forge built by `ReadOnlyForgeForCIToken`, `PostComment`, `FileIssue`, `ApplyLabels`, `CloseIssue` and `DeleteRef` each return a `Refused` error and the `httptest` server records zero requests. **Unlisted kind**: every other layer satisfied, a kind registered in a test-only `readKinds` entry but absent from `ciTransportKinds` exits 5 with the `[ci-transport:kind]` tag and zero requests |
| 5 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReadOnlyForgeRefusesEveryWriteMethod$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r5.out" 2>&1 && grep -F -e '--- PASS: TestReadOnlyForgeRefusesEveryWriteMethod' "${TMPDIR:-/tmp}/b34-r5.out"` | exit 0, `--- PASS:` printed. The test walks `Forge` by reflection. Every method `obClass` does not mark `read` is called with zero-value arguments and must return `Refused` with zero server requests; every `read` method must reach the inner forge. It also asserts that the decorator's struct type has no embedded (anonymous) field. Mutations M1 and M1b turn it red |
| 6 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusesNonInstallationToken$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r6.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesNonInstallationToken' "${TMPDIR:-/tmp}/b34-r6.out"` | exit 0, `--- PASS:` printed. **Negative path**: every other layer satisfied (flag, valid CI variables, matching repository, listed kind). Tokens prefixed `ghp_`, `github_pat_`, `gho_`, `ghu_` and an unprefixed string each exit 5 with the `[ci-transport:token-shape]` tag and zero server requests. A deskkit-level case calls `ReadOnlyForgeForCIToken` directly with a `ghp_` token and a matching `jobRepo`, and gets `Refused` with zero requests, so the constructor holds with `deskread` bypassed. Mutation M4 turns it red |
| 7 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportBindsJobRepository$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r7.out" 2>&1 && grep -F -e '--- PASS: TestCITransportBindsJobRepository' "${TMPDIR:-/tmp}/b34-r7.out"` | exit 0, `--- PASS:` printed. **Negative path**: every other layer satisfied. With `GITHUB_REPOSITORY=o/a` and `--repo o/a,o/b`, the server sees requests for `o/a` only, and `o/b` appears in `partial` with the binding reason. A `trust` read whose `--issue` names `o/b` exits 5 with the `[ci-transport:repository]` tag. A deskkit-level case calls `ReadOnlyForgeForCIToken` for `o/b` with `jobRepo` `o/a` and gets `Refused` with zero requests. Mutation M5 turns it red |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportFlagWithoutTokenRefused$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r8a.out" 2>&1 && grep -F -e '--- PASS: TestCITransportFlagWithoutTokenRefused' "${TMPDIR:-/tmp}/b34-r8a.out" && go test ./internal/deskkit/ -run '^TestCITransportPinsAPIBase$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r8b.out" 2>&1 && grep -F -e '--- PASS: TestCITransportPinsAPIBase' "${TMPDIR:-/tmp}/b34-r8b.out"` | exit 0, both `--- PASS:` lines printed. **Negative path**: every other layer satisfied, the flag with `DESKREAD_CI_WORKFLOW_TOKEN` unset exits 5 with the `[ci-transport:no-token]` tag, even with `GH_TOKEN` and `GITHUB_TOKEN` set to valid-shaped tokens, and the custody minter seam is never called. In the deskkit test, with `ciTokenAPIBase` empty the built backend's base is `GitHubAPIBase`, and with `GITHUB_API_URL` naming a second `httptest` server that server records zero requests |
| 9 | check:ci +dereference | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCustodyPathUnchangedByCITransport$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r9.out" 2>&1 && grep -F -e '--- PASS: TestCustodyPathUnchangedByCITransport' "${TMPDIR:-/tmp}/b34-r9.out"` | exit 0, `--- PASS:` printed. Without the flag, `sessionRoleFn` and the custody minter seam are each called once per run, the server sees the minted token, and the envelope minus `identity` is byte-equal to a golden captured at the base |
| 10 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRecordsIdentityNotToken$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r10.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRecordsIdentityNotToken' "${TMPDIR:-/tmp}/b34-r10.out"` | exit 0, `--- PASS:` printed. A successful CI-transport read's envelope carries `identity.transport == "ci-workflow-token"`, the repository and the run id; the custody run's carries `"app-custody"` and the role. The token string appears in neither stdout nor stderr of either run |
| 11 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestNoAmbientEnvTokenRead$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11a.out" 2>&1 && grep -F -e '--- PASS: TestNoAmbientEnvTokenRead' "${TMPDIR:-/tmp}/b34-r11a.out" && go test ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11b.out" 2>&1 && grep -F -e '--- PASS: TestForgeSingleConstructionSite' "${TMPDIR:-/tmp}/b34-r11b.out" && go test ./internal/deskkit/ -run '^TestForgeSurfaceUnchangedByDeskread$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11c.out" 2>&1 && grep -F -e '--- PASS: TestForgeSurfaceUnchangedByDeskread' "${TMPDIR:-/tmp}/b34-r11c.out" && go test ./internal/deskkit/ -run '^TestOutboundForgeWrapsEveryWriteMethod$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11d.out" 2>&1 && grep -F -e '--- PASS: TestOutboundForgeWrapsEveryWriteMethod' "${TMPDIR:-/tmp}/b34-r11d.out"` | exit 0, each `--- PASS:` line printed. The guards pass with the new constructor and permit, and the `Forge` surface is unchanged |
| 12 | check | `grep -n -E 'ambientTokenReadCeiling = 5' tools/desk/internal/deskkit/ambienttoken_guard_test.go && grep -c -F '"DESKREAD_CI_WORKFLOW_TOKEN"' tools/desk/internal/deskkit/ambienttoken_guard_test.go` | exit 0; the ceiling line prints, and the count is at least 1 (the variable is in the guarded set) |
| 13 | check +dereference | `awk '/^  (corroborate\|model-autoflip):$/{j=$1} j && /^    permissions:/{p=1; next} p && /^      [a-z-]+:/{print j, $0} p && !/^      /{p=0; j=""}' .github/workflows/assay-statusgen.yml > "${TMPDIR:-/tmp}/b34-r13.out" && grep -q '^corroborate: ' "${TMPDIR:-/tmp}/b34-r13.out" && grep -q '^model-autoflip: ' "${TMPDIR:-/tmp}/b34-r13.out" && ! grep -v ': read$' "${TMPDIR:-/tmp}/b34-r13.out"` | exit 0: each of the two jobs prints at least one permission line (so a renamed job key fails the row instead of passing it vacuously), and every printed line ends in `read`. This proves the server-side layer for this repository's two jobs only; adopter jobs are 18's (see `consumers:`). Run at the implementing head; this brief does not change the file |
| 14 | check | `git fetch -q origin main && git diff --name-only "$(git merge-base FETCH_HEAD HEAD)" HEAD -- statusgen .github/workflows \| wc -l` | prints `0` (against a freshly fetched `main`, so a stale local remote cannot report `0`) |
| 15 | check:ci | `cd statusgen && go build -o "${TMPDIR:-/tmp}/b34-statusgen" . && cd .. && "${TMPDIR:-/tmp}/b34-statusgen" --root . --lint` | exit 0, `LINT: PASS` |
| 16 | check:ci | `git fetch -q origin main && "${TMPDIR:-/tmp}/b34-statusgen" --root . --consumers --brief forge-neutral/34 --base "$(git merge-base FETCH_HEAD HEAD)"` | exit 0; no routing claim disproved by the diff |
| 17 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportToolClass$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r17.out" 2>&1 && grep -F -e '--- PASS: TestCITransportToolClass' "${TMPDIR:-/tmp}/b34-r17.out"` | exit 0, `--- PASS:` printed. `toolClassFor` returns `ClassCI` for flag plus `GITHUB_ACTIONS=true`, and `ClassWrite` for flag without CI, CI without flag, and neither. **Negative path**: the CI-without-flag case keeps the custody path file-only. Mutation M6 turns it red |
| 18 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportActivatesWithEnvRoster$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r18.out" 2>&1 && grep -F -e '--- PASS: TestCITransportActivatesWithEnvRoster' "${TMPDIR:-/tmp}/b34-r18.out"` | exit 0, `--- PASS:` printed. The test builds `deskread` and runs it from a checkout with an empty config home, the CI variables and a roster in the environment. With the flag and a deliberately non-installation token it exits 5 with the `[ci-transport:token-shape]` tag and no `inactive` line: activation passed and `run` was reached, with no network call. **Negative path**: the same environment without the flag exits 6 with the `assay/desk-tools inactive` line, as at the base |
| 19 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCITokenConstructorConfined$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r19.out" 2>&1 && grep -F -e '--- PASS: TestCITokenConstructorConfined' "${TMPDIR:-/tmp}/b34-r19.out"` | exit 0, `--- PASS:` printed. The real tree has exactly the one allowed caller of `ReadOnlyForgeForCIToken` and no non-test caller of `SetCITokenAPIBaseForTest`. **Negative path**: over a fixture tree with a planted second caller in another `cmd` package, and a planted non-test call to the seam, the scanner reports both |

## Named mutations

- **M1**, row 5: the decorator embeds `Forge` instead of holding it in a named field, and its
  `EditComment` method is deleted, so `EditComment` delegates through the embedding. Row 5 must
  go red on both the embedded-field assertion and the server request.
- **M1b**, row 5: `EditComment` is written out but delegates to the inner forge. Row 5 must go
  red on the server request.
- **M2**, row 2: the opt-in is read from the environment alone (`DESKREAD_CI_WORKFLOW_TOKEN` set
  ⇒ transport on). Row 2 must go red, because the CI token would reach the server without the
  flag.
- **M3**, row 8: the CI path falls back to custody when the variable is empty. Row 8 must go red
  on the custody-minter call.
- **M4**, row 6: the shape check is removed from both `deskread` and the constructor. Row 6 must
  go red, because a `ghp_` token reaches the server.
- **M5**, row 7: the repository binding is removed from both `deskread` and the constructor. Row
  7 must go red, because `o/b` is requested under the token.
- **M6**, row 17: `toolClassFor` returns `ClassForTool(true)` whatever the arguments. Row 17
  must go red on the CI-without-flag case.

## Pre-mortem → detection map

*"This shipped and was wrong. What went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| The transport turns on whenever CI exports a token, with no flag | rows 2, M2 |
| A forged `GITHUB_ACTIONS=true` on a workstation lets a person's token through | row 6 (shape, with the CI check passing), M4. The CI check itself is row 3. The repository binding is NOT a layer against this fault: it reads the same forged environment |
| A forged CI environment plus an App installation token the caller already holds | **accepted residual**, named in the SPOF note and the human decision: reads only (rows 4, 5) and listed kinds only (row 4). The identity record carries the forger's repository and run id, which the usage text and the brief state are unverified |
| A newly added negative row passes on another layer's refusal | every negative row asserts its own `[ci-transport:<layer>]` tag with every other layer satisfied (rows 3, 4, 6, 7, 8) |
| The job token is sent to a host named in the environment | row 8 |
| A write method added to `Forge` later is not refused by the decorator | the compiler (no embedding; row 1) and rows 5, M1, M1b |
| A new `deskread` kind that writes or reads widely becomes CI-eligible without review | row 4 (`ciTransportKinds` is separate and closed); 18 adds each kind it needs by reviewed diff |
| Another `cmd` package builds a token-bearing forge and skips every check | the constructor holds its own shape and repository checks (rows 6, 7), and row 19 confines its callers |
| The CI path quietly falls back to custody, or custody quietly gains a CI fallback | rows 8, 9, M3 |
| Activation refuses a roster-less CI runner, so the transport never runs | rows 17, 18 |
| The custody path starts reading its roster from the environment | rows 17, 18 (negative paths), M6 |
| The job token is handed to `deskread` in a `pull_request_target` job | row 3 (`event` case); 18's workflow edit never hands it over in such a job (`consumers:`) |
| The auto-flip mode reads another repository (the owning repo of a brief) under the job token | row 7. The read lands in `partial`, which forge-neutral/18 must route to custody or could-not-check; that routing is 18's concern, named in `consumers:` so 18's author sees it |
| The token leaks into output, a log line or the envelope | rows 2, 10 |
| The ambient-token guard is satisfied by an unlisted variable name, hiding the new read | row 12 |
| The guard tests are weakened instead of extended | rows 11, 19; Review reads the test diff |
| This brief moves statusgen sites or edits a workflow ahead of forge-neutral/18 | row 14 |
| The job's `permissions:` block is widened later, so the server-side layer weakens | row 13 at this head only; a later widening is a workflow change that review must catch |
| An adopter's job token is write-scoped, so the server-side layer is absent there | **no row here**: stated in `facts:` and the SPOF note, and handed to 18 (scaffold narrowing with its own row, `consumers:`) |
| GitHub changes its token-prefix scheme | **no row**. Review-only: the prefix fact is the vendor's, cited in `sources:` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review

Gate: human (from frontmatter; sensitive-data: yes). The human decision above, tracked on
#2315, is ratified before dispatch, and an approved design record (`design: DR-forge-neutral-34`)
is added to this brief's frontmatter from that ratification, as the design-approval gate
requires before the brief can move to in-progress.

A reviewer of this core change answers both questions in the verdict:
1. What is the single control between a forged environment and a read under the wrong
   credential, and is it acceptable? (Expected answer: against a deliberate forger, the
   read-only decorator with the closed kind set; the shape check stops only a personal or OAuth
   token; the CI check, the repository binding and the server-side job scope defend honest CI,
   not a forger. Acceptable because the residual is reads under a token the forger already
   holds.)
2. Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?
   (Expected: rows 3, 4, 6, 7 and 8 each satisfy every other layer and assert their own refusal
   tag; rows 6 and 7 also call the constructor directly with `deskread` bypassed; rows 4 and 5
   prove the decorator refuses writes however the forge was reached.)

The reviewer also confirms from the diff that no file under `statusgen/` or
`.github/workflows/` changed, and that `TestForgeSurfaceUnchangedByDeskread`'s `want` list is
unedited.
