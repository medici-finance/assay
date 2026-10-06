---
brief: assay:assay:forge-neutral:34
title: deskread CI workflow-token transport — an explicit, CI-only, read-only opt-in beside the App custody default
why: >-
  statusgen's CI-only modes (the pull-request corroboration job, the model auto-flip job, and
  the transcribe lanes an adopter runs) read the forge under the workflow's own job token today,
  through `gh`. forge-neutral/18 has to move every one of those reads onto `deskread`, but
  `deskread` only ever authenticates as a minted desk App, and a CI runner has no App custody to
  mint from. So 18 cannot finish without either giving every CI job an App credential or giving
  `deskread` a narrow way to read under the token the job already holds. The driver's ruling on
  #2253 chose the second. This brief adds that way and fences it: opt-in per call, refused
  outside CI, refused for anything that writes, bound to the job's own repository, and recorded
  in the output, so the desk's no-ambient-token rule keeps holding everywhere else.
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
  variable. That token is a credential (sensitive-data: yes). The human confirms three things.
  (1) The opt-in is explicit and never a fallback: no flag means the App custody path runs
  exactly as today, and a flag with no token is refused, not degraded. (2) "Outside CI" is
  refused on environment signals the brief admits are forgeable, and the independent layers
  behind that check (installation-token shape, same-repository binding, a read-only forge
  that refuses every write method, the job's read-only `permissions:` block on the server)
  are an acceptable answer to that forgeability. (3) The identity record — transport, repository
  and run id in the envelope, never the token — is enough to tell afterwards which identity
  performed a read.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
outcome: none
id: a6651bd7-de82-48d4-b80e-38c7d6663222
authored: 2026-10-06 by forge-neutral authoring session (the #2253 ruling)
sources:
  - "#2253 comment 6013956713: the question this brief answers. statusgen's CI-only modes (`--corroborate`, `--auto-flip-model`, the transcribe lanes) read under the workflow's job token, `deskread` reads only under a minted App role, so moving those reads onto `deskread` needs a decision about which credential CI reads under"
  - "#2253 comment 6014345604: the driver's ruling on #2253, option (a). `deskread` gains an explicit, opt-in transport that authenticates with the CI workflow token, for read-only kinds only, so the CI-only modes can move their `gh` reads onto it. The ruling asks for this as its own gated brief"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: Verify row 3 (zero `exec.Command(\"gh\"` in non-test statusgen), which the CI-lane sites cannot meet without this transport, and row 15 (`TestForgeSurfaceUnchangedByDeskread`), which this brief must keep green because it adds no `Forge` operation"
  - "#2313 (draft, forge-neutral/33): the four `Forge` reads and `deskread` kinds statusgen's remaining sites need. Its scope fence leaves `deskread`'s identity untouched and defers the CI identity to #2253. This brief is the other half: it adds the identity, no reads"
  - "docs/streams/forge-neutral/README.md, 'Shared conventions the briefs inherit': refusal not fallback; negative-path rows are mandatory; no hand-built API call is evidence"
  - "tools/desk/cmd/deskread/forge.go:46-51: the only identity path today, `sessionRoleFn(\"deskread\")` (= `deskkit.SessionTokenRole`) then `deskkit.ForgeFor`, with an Unverifiable refusal that names 'never an ambient forge-CLI identity'"
  - "tools/desk/cmd/deskread/main.go:74 (`readKinds`), :81 (`perIssueKinds`), :127 (`SetToolClass(ClassForTool(false))`, roster from the config-home file only)"
  - "tools/desk/internal/deskkit/forgeresolve.go:302 (`custody`), :316 (`githubCustody`), :730 (`ForgeFor`), :743 (`ResolveForge`), :765-767 (the one construction site, each backend wrapped by `OutboundChecked`)"
  - "tools/desk/internal/deskkit/outboundforge.go:39 (`OutboundChecked`) and its write-method classification; tools/desk/internal/deskkit/outbound_test.go:567 (`TestOutboundForgeWrapsEveryWriteMethod`, which walks `Forge` by reflection)"
  - "tools/desk/internal/deskkit/ambienttoken_guard_test.go:39-41 (`ambientTokenVars`: GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN, GITLAB_TOKEN), :46 (the permit register), :61 (`ambientTokenReadCeiling = 4`), :63 (`TestNoAmbientEnvTokenRead`)"
  - "tools/desk/internal/deskkit/rosterconfig.go:960 (`InCI`, which checks GITHUB_ACTIONS only) and :1008 (`ClassForTool`)"
  - "tools/desk/internal/deskkit/forgeresolve_test.go:285 (`TestForgeSingleConstructionSite`); tools/desk/internal/deskkit/forge_surface_deskread_test.go:35 (`TestForgeSurfaceUnchangedByDeskread`)"
  - ".github/workflows/assay-statusgen.yml: workflow default `permissions: contents: read` (:66-67); job `corroborate` `contents: read, pull-requests: read` (:115-117), `GH_TOKEN: ${{ github.token }}` (:207), `--corroborate --pr` (:229); job `model-autoflip` `contents: read` (:347-348), `GH_TOKEN` (:395), `--auto-flip-model` (:411)"
  - "GitHub's documented token prefixes (installation tokens, including the Actions job token, begin `ghs_`; personal tokens `ghp_` / `github_pat_`; OAuth `gho_`; user-to-server `ghu_`). This is a review-only fact: no row can prove a vendor's prefix scheme, and tools/desk/internal/deskkit/bodycheck.go:16 already treats these prefixes as the token vocabulary"
  - "freshness-checked 2026-10-06 @ e6cb7d2a0 (origin/main): 22 `exec.Command(\"gh\"` sites in the CI-lane statusgen files (autoflip.go 8, corroborate.go 4, citationcorroborate.go 2, decisiongateanchor.go 1, trustgate.go 1, transcribescan.go 2, transcribeverdict.go 2, scanissues.go 2); `ambientTokenReadCeiling = 4`; `deskread` has three kinds (`issues`, `trust`, `comments`) and envelope schema 1"
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (c). It decides which credential a desk verb may read under, and the
  dangerous failures pass every happy-path test: an env-only opt-in that turns on whenever CI
  exports a token, a base URL taken from the environment that sends the job token to another
  host, a read-only wrapper that misses a newly added write method, or a refusal that quietly
  falls through to the custody path and reports success.
domain: complicated
consumers:
  - "tools/desk/cmd/deskread: follow-up forge-neutral/34 (this brief's implementation: the `--ci-workflow-token` flag, the CI-transport kind allowlist, the identity record in the envelope, and the refusals of Task 2-4)"
  - "tools/desk/internal/deskkit/forgeresolve.go: follow-up forge-neutral/34 (this brief's implementation: the second, read-only constructor beside `ResolveForge`, Task 1)"
  - "tools/desk/internal/deskkit/readonlyforge.go: follow-up forge-neutral/34 (this brief's implementation: the decorator that refuses every write method, Task 1)"
  - "tools/desk/internal/deskkit/forgeresolve_test.go, tools/desk/internal/deskkit/roletokenguard_test.go, tools/desk/internal/deskkit/ambienttoken_guard_test.go: follow-up forge-neutral/34 (this brief's implementation: each guard learns the new constructor or variable and still fails on anything else, Task 5)"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: fixed-here (its `depends:` gains this brief, because its row 3 cannot reach 0 for the CI-lane sites without this transport)"
  - "docs/streams/forge-neutral/README.md: fixed-here (the board row for this brief and the critical-path note `34 → 18`)"
  - "statusgen/forgeread.go, statusgen/autoflip.go, statusgen/corroborate.go, statusgen/citationcorroborate.go, statusgen/decisiongateanchor.go, statusgen/trustgate.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go, statusgen/scanissues.go: follow-up forge-neutral/18 (moving each CI-lane site onto `deskread --ci-workflow-token` is 18's Task and row 3; this brief touches no statusgen file)"
  - ".github/workflows/assay-statusgen.yml: follow-up forge-neutral/18 (handing the job token to `deskread` in the dedicated variable is 18's workflow edit; this brief changes no workflow)"
  - "statusgen/init.go: follow-up forge-neutral/18 (the scaffolded adopter workflow carries the same `GH_TOKEN: ${{ github.token }}` line, `:482`, and moves with 18's workflow edit)"
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
(`cmd/deskread/forge.go:46-51`). A CI runner has no App custody to mint from, so the CI-lane
sites have nowhere to go, and 18's row 3 cannot reach 0.

#2253 put that question to the driver. The driver's ruling on #2253 chose option (a):
`deskread` gains an explicit, opt-in transport that reads with the CI job token, for read-only
kinds only. This brief adds that transport and nothing else. It adds no `Forge` operation, no
`deskread` read kind, and it touches no statusgen file and no workflow. Moving the sites onto
the transport stays 18's job, and the reads statusgen still lacks are forge-neutral/33's.

files:
- `tools/desk/cmd/deskread/main.go`: the flag, the CI-transport kind allowlist, the identity
  record in the envelope, the usage text.
- `tools/desk/cmd/deskread/forge.go`: the transport switch between the custody path (default,
  unchanged) and the CI path.
- `tools/desk/internal/deskkit/forgeresolve.go`: the second constructor, `ReadOnlyForgeForCIToken`
  (name is a suggestion), beside `ResolveForge`.
- `tools/desk/internal/deskkit/readonlyforge.go` (new): the decorator that refuses every write
  method.
- Tests beside each: `tools/desk/cmd/deskread/*_test.go`,
  `tools/desk/internal/deskkit/readonlyforge_test.go` (new), and the three guard tests named in
  Task 5.

facts:
- **Identity today**: `deskread` calls `deskkit.SessionTokenRole("deskread")`, then
  `deskkit.ForgeFor(repo, role)` → `ResolveForge` → `custody` → `githubCustody`, which mints the
  role's App installation token. There is no other path and no ambient fallback.
- **Roster class**: `deskread` declares `ClassForTool(false)` (`main.go:127`), so its roster comes
  from the config-home file only, never from the environment. This brief keeps that.
- **CI detection available today**: `deskkit.InCI()` (`rosterconfig.go:960`) checks
  `GITHUB_ACTIONS=true` and nothing else. Every environment variable is forgeable by whoever
  runs the process.
- **The one construction site**: `ResolveForge` builds every backend and wraps each in
  `OutboundChecked` (`forgeresolve.go:765-767`). `TestForgeSingleConstructionSite` fails on a
  backend literal anywhere else. `custody` and `githubCustody` are confined links
  (`roletokenguard_test.go`).
- **Write classification**: `outboundforge.go` already sorts every `Forge` method into
  text-checked writes, no-text writes and reads, and `TestOutboundForgeWrapsEveryWriteMethod`
  walks the interface by reflection so a new method cannot go unclassified.
- **Ambient-token guard**: `TestNoAmbientEnvTokenRead` fails on any function under `tools/desk`
  that reads GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN or
  GITLAB_TOKEN, outside a ratcheted register of 4 permits.
- **Server-side layer**: in the statusgen workflow the default is `contents: read`; the
  `corroborate` job holds `contents: read, pull-requests: read`; the `model-autoflip` job holds
  `contents: read`. A job token minted under those blocks cannot write repository content,
  pull requests or issues, whatever the client does.
- **Exit codes** (shared `deskkit`): 0 ok, 3 disabled, 5 refused (`ExitRefused`), 6 unverifiable
  (`ExitUnverifiable`).

single-point-of-failure: the CI check (an environment signal, forgeable) — behind it, four
independent layers: the installation-token shape check, the same-repository binding, the
read-only forge decorator that refuses every write method in `deskkit`, and the job's read-only
`permissions:` block enforced by the forge server.

### Risk-files cross-read

The declared paths sit under `tools/desk/internal/deskkit/` and `tools/desk/cmd/deskread/`, both
security-trigger paths, and the brief touches credential handling. The four answers were read
against them, not defaulted:
- **sensitive-data: yes.** The brief handles a credential: it reads the job token from an
  environment variable and sends it as an `Authorization` header. It never prints, logs or
  stores it, and Verify row 10 proves it does not appear in stdout or stderr.
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
cannot use this tool because a CI job has no such account; they use the job's own short-lived,
read-scoped token through a separate command-line client instead. The previous ruling chose to
let the tool read under the CI job token, for reads only. This decision ratifies the exact
shape of that exception before it is built.

As briefed, the exception applies only when all of these hold: the caller passes an explicit
flag; the token arrives in a dedicated variable, never the usual token variables; the process
sees the CI environment markers (which anyone can set, so the next three layers exist); the
token has the shape of an installation token, not a personal one; the repository read is the
job's own; and the read is one of the read-only kinds on a fixed list. The forge client it builds
refuses every write. The output records that the CI token was used, with the repository and run
id, and never the token. Without the flag, nothing changes.

Options:
1. **Approve as briefed.** The transport is built exactly as above; the CI checks then move
   onto the tool in a follow-on change.
2. **Approve with changes.** Name the change (for example: require a second CI marker, drop
   the token-shape check, widen to other repositories). The brief is revised before dispatch.
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
  base URL from the environment.
- If anything is unclear or contradicts repo state (for example, if `CheckVerbActivation` refuses
  a roster-less CI runner before `run` is reached), report NEEDS_CONTEXT. Do not bypass
  activation, and do not guess.

## Task

### 1. A read-only constructor in `deskkit`

Add, in `forgeresolve.go`, a second constructor beside `ResolveForge`:

```go
func ReadOnlyForgeForCIToken(repo ForgeRepo, token string) (Forge, ForgeResolution, error)
```

It:
1. resolves the forge kind with the same `resolveForgeKind` (no caller-supplied forge);
2. refuses (`Refused`) any kind other than GitHub — a GitLab job token is a different
   credential with different scopes, and is out of this brief;
3. takes the API base from the same place `githubCustody` gets it for this repo (the deskkit
   GitHub default, or the configured enterprise base), never from `GITHUB_API_URL` or any other
   environment variable, so a forged environment cannot redirect the token to another host;
4. returns `ReadOnly(OutboundChecked(&GitHubForge{Token: token, BaseURL: base}, "ci-workflow-token"))`.

Add `readonlyforge.go`: a `ReadOnly(Forge) Forge` decorator. Every method that
`outboundforge.go` classifies as a write (text-checked or no-text) returns a `Refused` error
naming the method and "the CI workflow-token transport is read-only", and never calls the
inner forge. Every read method passes through. The classification is not copied: the
decorator derives "is a write" from the same table `OutboundChecked` uses, so a write method
added later is refused by default.

### 2. The opt-in in `deskread`

- Add a boolean flag `--ci-workflow-token`. It is the only switch. No environment variable,
  config key or roster entry turns the transport on.
- Under the flag, read the token from exactly one dedicated variable,
  `DESKREAD_CI_WORKFLOW_TOKEN` (the workflow sets it to `${{ github.token }}`). Empty or unset →
  `ExitRefused`, with a message naming the variable. Never read GH_TOKEN or GITHUB_TOKEN as a
  substitute.
- Without the flag, a set `DESKREAD_CI_WORKFLOW_TOKEN` is ignored: the custody path runs, and
  one stderr line says the variable was ignored because the flag was absent. The token is never
  used implicitly.
- Under the flag, there is no fallback to custody. A refusal is a refusal.

### 3. Refusals, all before any network call

Under `--ci-workflow-token`, refuse with `ExitRefused` and a reason line, in this order:
1. **Outside CI**: `deskkit.InCI()` is false, or `GITHUB_RUN_ID` is not a non-empty run of
   digits, or `GITHUB_REPOSITORY` is not a non-empty `owner/name`. The message states that CI
   is detected by these variables.
2. **Token shape**: the token does not begin `ghs_` (installation token). A `ghp_`,
   `github_pat_`, `gho_` or `ghu_` token, or anything else, is refused: the transport exists
   for the job token, never for a person's credential.
3. **Kind**: the kind is not on `ciTransportKinds`, a closed set in `main.go` separate from
   `readKinds`. At authoring it holds `issues`, `trust` and `comments` (every current kind is a
   read). A kind added to `readKinds` later is NOT on it until a reviewed diff adds it, and a
   test pins that every entry is also in `readKinds`.
4. **Repository binding**: every target repository must equal `GITHUB_REPOSITORY`
   (case-insensitive). A `--repo` that names any other repository is not read: it lands in the
   envelope's `partial` list with reason "the CI workflow-token transport reads only the job's
   own repository", and is never sent the token. A per-issue kind whose `--issue` names another
   repository is refused outright.

### 4. The identity record

Add an additive, omitempty `identity` object to the envelope. Schema stays 1: statusgen's
envelope struct declares only the fields it consumes, so an added field is ignored by the
existing consumer.
- Custody path: `{"transport": "app-custody", "role": "<role>"}`.
- CI path: `{"transport": "ci-workflow-token", "repository": "<GITHUB_REPOSITORY>", "runId": "<GITHUB_RUN_ID>"}`.

Write one stderr line naming the transport, the same way, on every run. The token value never
appears in stdout, stderr or the envelope. Update the usage text: the identity paragraph names
both transports and states that the CI one is opt-in, CI-only, read-only and same-repository.

### 5. Guards learn the new path and still fail on anything else

- `TestForgeSingleConstructionSite`: allow exactly the one new literal in
  `ReadOnlyForgeForCIToken`, and require it to be wrapped by both `OutboundChecked` and
  `ReadOnly`. Any other backend literal still fails.
- `roletokenguard_test.go`: `custody` and `githubCustody` stay confined to their present callers.
  The new constructor must not call either.
- `ambienttoken_guard_test.go`: add `DESKREAD_CI_WORKFLOW_TOKEN` to `ambientTokenVars`, and add
  exactly one reasoned permit for the one `deskread` function that reads it. Raise
  `ambientTokenReadCeiling` from 4 to 5 in the same diff. The permit's reason names the flag,
  the CI check and the read-only constructor. This keeps the new read visible in the ratchet,
  not hidden behind an unlisted name.
- `TestForgeSurfaceUnchangedByDeskread`: unchanged and green.

## Verify (executable — no prose-only DoD items)

The Class column uses the stream's convention:
- `check:ci` runs in CI.
- `check` is a local or grep-level assertion.
- `+dereference` resolves a claim instead of counting presence.
- `+flow` exercises verb → constructor → backend against an `httptest` server.
- `+mutation` is a mutation demonstration.

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
- `TestCITransportBaseURLNotFromEnv` (planned)

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./cmd/deskread/ ./internal/deskkit/ && go test ./...` | exit 0 |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusedWithoutFlag$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r2.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusedWithoutFlag' "${TMPDIR:-/tmp}/b34-r2.out"` | exit 0, `--- PASS:` printed. **Negative path (a), no opt-in**: with `GITHUB_ACTIONS=true`, a valid run id and repository, and `DESKREAD_CI_WORKFLOW_TOKEN` set to an installation-shaped test token, a run WITHOUT the flag takes the custody path. The `httptest` server's recorded `Authorization` headers never contain the CI token, and stderr carries the "ignored" line |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusedOutsideCI$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r3.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusedOutsideCI' "${TMPDIR:-/tmp}/b34-r3.out"` | exit 0, `--- PASS:` printed. **Negative path (b), outside CI**: table cases `GITHUB_ACTIONS` unset, `GITHUB_ACTIONS=false`, `GITHUB_RUN_ID` empty, `GITHUB_RUN_ID=abc`, `GITHUB_REPOSITORY` empty. Each exits 5 (`ExitRefused`), and the `httptest` server records zero requests |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCITransportRefusesWriteMethods$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r4a.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesWriteMethods' "${TMPDIR:-/tmp}/b34-r4a.out" && go test ./cmd/deskread/ -run '^TestCITransportRefusesUnlistedKind$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r4b.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesUnlistedKind' "${TMPDIR:-/tmp}/b34-r4b.out"` | exit 0, both `--- PASS:` lines printed. **Negative path (c), write kind**: on a forge built by `ReadOnlyForgeForCIToken`, `PostComment`, `FileIssue`, `ApplyLabels`, `CloseIssue` and `DeleteRef` each return a `Refused` error and the `httptest` server records zero requests. A kind registered in a test-only `readKinds` entry but absent from `ciTransportKinds` exits 5 under the flag |
| 5 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReadOnlyForgeRefusesEveryWriteMethod$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r5.out" 2>&1 && grep -F -e '--- PASS: TestReadOnlyForgeRefusesEveryWriteMethod' "${TMPDIR:-/tmp}/b34-r5.out"` | exit 0, `--- PASS:` printed. The test walks `Forge` by reflection, calls every method classified as a write with zero-value arguments, and asserts `Refused` with zero server requests. It also asserts every read method reaches the inner forge. Mutation M1 turns it red |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRefusesNonInstallationToken$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r6.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRefusesNonInstallationToken' "${TMPDIR:-/tmp}/b34-r6.out"` | exit 0, `--- PASS:` printed. **Negative path**: tokens prefixed `ghp_`, `github_pat_`, `gho_`, `ghu_` and an unprefixed string each exit 5 with zero server requests |
| 7 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportBindsJobRepository$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r7.out" 2>&1 && grep -F -e '--- PASS: TestCITransportBindsJobRepository' "${TMPDIR:-/tmp}/b34-r7.out"` | exit 0, `--- PASS:` printed. **Negative path**: with `GITHUB_REPOSITORY=o/a` and `--repo o/a,o/b`, the server sees requests for `o/a` only; `o/b` appears in `partial` with the binding reason; a `trust` read whose `--issue` names `o/b` exits 5 |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportFlagWithoutTokenRefused$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r8a.out" 2>&1 && grep -F -e '--- PASS: TestCITransportFlagWithoutTokenRefused' "${TMPDIR:-/tmp}/b34-r8a.out" && go test ./internal/deskkit/ -run '^TestCITransportBaseURLNotFromEnv$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r8b.out" 2>&1 && grep -F -e '--- PASS: TestCITransportBaseURLNotFromEnv' "${TMPDIR:-/tmp}/b34-r8b.out"` | exit 0, both `--- PASS:` lines printed. **Negative path**: the flag with `DESKREAD_CI_WORKFLOW_TOKEN` unset exits 5, even with `GH_TOKEN` and `GITHUB_TOKEN` set to valid-shaped tokens, and the custody minter seam is never called. A second server named in `GITHUB_API_URL` records zero requests |
| 9 | check:ci +dereference | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCustodyPathUnchangedByCITransport$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r9.out" 2>&1 && grep -F -e '--- PASS: TestCustodyPathUnchangedByCITransport' "${TMPDIR:-/tmp}/b34-r9.out"` | exit 0, `--- PASS:` printed. Without the flag, `sessionRoleFn` and the custody minter seam are each called once per run, the server sees the minted token, and the envelope minus `identity` is byte-equal to a golden captured at the base |
| 10 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportRecordsIdentityNotToken$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r10.out" 2>&1 && grep -F -e '--- PASS: TestCITransportRecordsIdentityNotToken' "${TMPDIR:-/tmp}/b34-r10.out"` | exit 0, `--- PASS:` printed. A successful CI-transport read's envelope carries `identity.transport == "ci-workflow-token"`, the repository and the run id; the custody run's carries `"app-custody"` and the role. The token string appears in neither stdout nor stderr of either run |
| 11 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestNoAmbientEnvTokenRead$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11a.out" 2>&1 && grep -F -e '--- PASS: TestNoAmbientEnvTokenRead' "${TMPDIR:-/tmp}/b34-r11a.out" && go test ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11b.out" 2>&1 && grep -F -e '--- PASS: TestForgeSingleConstructionSite' "${TMPDIR:-/tmp}/b34-r11b.out" && go test ./internal/deskkit/ -run '^TestForgeSurfaceUnchangedByDeskread$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11c.out" 2>&1 && grep -F -e '--- PASS: TestForgeSurfaceUnchangedByDeskread' "${TMPDIR:-/tmp}/b34-r11c.out" && go test ./internal/deskkit/ -run '^TestOutboundForgeWrapsEveryWriteMethod$' -count=1 -v > "${TMPDIR:-/tmp}/b34-r11d.out" 2>&1 && grep -F -e '--- PASS: TestOutboundForgeWrapsEveryWriteMethod' "${TMPDIR:-/tmp}/b34-r11d.out"` | exit 0, each `--- PASS:` line printed. The guards pass with the new constructor and permit, and the `Forge` surface is unchanged |
| 12 | check | `grep -n -E 'ambientTokenReadCeiling = 5' tools/desk/internal/deskkit/ambienttoken_guard_test.go && grep -c -F '"DESKREAD_CI_WORKFLOW_TOKEN"' tools/desk/internal/deskkit/ambienttoken_guard_test.go` | exit 0; the ceiling line prints, and the count is at least 1 (the variable is in the guarded set) |
| 13 | check +dereference | `awk '/^  (corroborate\|model-autoflip):$/{j=$1} j && /^    permissions:/{p=1; next} p && /^      [a-z-]+:/{print j, $0} p && !/^      /{p=0; j=""}' .github/workflows/assay-statusgen.yml` | every printed line ends in `read`, and none in `write`: the two CI jobs that will use the transport hold read-only job tokens on the server side. Run at the implementing head; this brief does not change the file |
| 14 | check | `git diff --name-only "$(git merge-base origin/main HEAD)" HEAD -- statusgen .github/workflows \| wc -l` | prints `0` |
| 15 | check:ci | `cd statusgen && go build -o "${TMPDIR:-/tmp}/b34-statusgen" . && cd .. && "${TMPDIR:-/tmp}/b34-statusgen" --root . --lint` | exit 0, `LINT: PASS` |
| 16 | check:ci | `"${TMPDIR:-/tmp}/b34-statusgen" --root . --consumers --brief forge-neutral/34 --base "$(git merge-base origin/main HEAD)"` | exit 0; no routing claim disproved by the diff |

## Named mutations

- **M1**, row 5: the `ReadOnly` decorator hard-codes its write list instead of deriving it from
  the `outboundforge.go` classification, and one write method (for example `EditComment`) is left
  off. The reflection walk must go red on the uncovered method.
- **M2**, row 2: the opt-in is read from the environment alone (`DESKREAD_CI_WORKFLOW_TOKEN` set
  ⇒ transport on). Row 2 must go red, because the CI token would reach the server without the
  flag.
- **M3**, row 8: the CI path falls back to custody when the variable is empty. Row 8 must go red
  on the custody-minter call.

## Pre-mortem → detection map

*"This shipped and was wrong. What went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| The transport turns on whenever CI exports a token, with no flag | rows 2, M2 |
| A forged `GITHUB_ACTIONS=true` on a workstation lets a person's token through | row 6 (shape) and row 7 (repository binding) as independent layers; row 3 for the CI check itself; the residual — a stolen `ghs_` token used locally — is read-only (row 4, 5) and same-repository |
| The job token is sent to a host named in the environment | row 8 |
| A write method added to `Forge` later is not refused by the decorator | rows 5, M1 |
| A new `deskread` kind that writes or reads widely becomes CI-eligible without review | row 4 (`ciTransportKinds` is separate and closed) |
| The CI path quietly falls back to custody, or custody quietly gains a CI fallback | rows 8, 9, M3 |
| The auto-flip mode reads another repository (the owning repo of a brief) under the job token | row 7. The read lands in `partial`, which forge-neutral/18 must route to custody or could-not-check; that routing is 18's concern, named here so 18's author sees it |
| The token leaks into output, a log line or the envelope | row 10 |
| The ambient-token guard is satisfied by an unlisted variable name, hiding the new read | row 12 |
| The guard tests are weakened instead of extended | row 11; Review reads the test diff |
| This brief moves statusgen sites or edits a workflow ahead of forge-neutral/18 | row 14 |
| Activation refuses a roster-less CI runner, so the transport never runs | **no row**. Ground rules require NEEDS_CONTEXT, and forge-neutral/18's first CI run shows it |
| The job's `permissions:` block is widened later, so the server-side layer weakens | row 13 at this head only; a later widening is a workflow change that review must catch |
| GitHub changes its token-prefix scheme | **no row**. Review-only: the prefix fact is the vendor's, cited in `sources:` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review

Gate: human (from frontmatter; sensitive-data: yes). The human decision above is ratified
before dispatch, and an approved design record (`design: DR-forge-neutral-34`) is added to this
brief's frontmatter from that ratification, as the design-approval gate requires before the
brief can move to in-progress.

A reviewer of this core change answers both questions in the verdict:
1. What is the single control between a forged environment and a read under the wrong
   credential, and is it acceptable? (Expected answer: the CI check, behind token shape,
   repository binding, the read-only decorator and the server-side job scope.)
2. Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?
   (Expected: rows 6 and 7 run with a passing CI check, and row 4 proves the decorator refuses
   writes regardless of how the forge was reached.)

The reviewer also confirms from the diff that no file under `statusgen/` or
`.github/workflows/` changed, and that `TestForgeSurfaceUnchangedByDeskread`'s `want` list is
unedited.
