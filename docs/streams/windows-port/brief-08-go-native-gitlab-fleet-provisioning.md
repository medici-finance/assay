---
brief: assay:assay:windows-port:08
title: Go-native GitLab fleet provisioning — retire the bash+curl+jq script's Windows dependency
why: >-
  A Windows adopter on GitLab cannot complete an install in native PowerShell today, and the
  adoption docs say so outright: "The script itself is **bash + curl + jq**. On native Windows run
  it from Git-Bash or WSL, not from PowerShell" (`docs/adopting-assay-gitlab.md:200-201`), echoed in
  the Windows prerequisites (`docs/adopting-assay.md:904-906`). That is a 1167-line shell script
  standing between a Windows adopter and a working desk fleet — the single largest remaining reason
  the Windows path is not three commands. The same gap has a GitHub-side twin: the `create-labels`
  PRIMITIVE is nine hand-run `gh label create` invocations
  (`docs/adopting-assay.md:738-753`) with no forge-neutral equivalent, so a GitLab adopter's labels
  live only inside that bash script. Porting the provisioning to the Go desk-tools — which
  `windows-port/00` already made cross-compile for Windows — removes the Git-Bash prerequisite and
  puts the credential files under the same owner-only ACL custody the rest of the toolchain
  already enforces on Windows.
wave: 1
depends: ["windows-port/00", "windows-port/02"]
unblocks: ["windows-port/09"]
effort: L
gate: human
gate-why: >-
  This brief MINTS AND WRITES LIVE CREDENTIALS: seven GitLab personal access tokens for seven
  service accounts, persisted to disk as `gitlab-<role>.token` files that every desk verb then
  reads. `sensitive-data: yes` follows directly — a custody defect here does not fail a test, it
  leaks a live `api`/`write_repository` token for an entire fleet. What the human is confirming,
  specifically: (1) the PAT custody model on Windows — the `## Human decision` fork below, which
  fixes what "0600-equivalent" means on NTFS for every future Windows adopter; (2) that the
  credential never reaches a process argument, an environment variable, a log line, or an error
  message — only a path is ever printed; (3) that a partial provisioning run REFUSES and reports
  rather than leaving half a fleet with live tokens nobody is tracking. None of the three is a
  correctness property a model can sign off on its own evidence.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
decision-trigger: creation
issues: []
schema: brief-v2
authored: 2026-09-11 by windows-port authoring session (driver ask, 2026-09-11)
sources:
  - "driver's ask (2026-09-11): a Go-native desk verb replacing tools/create-fleet-gitlab.sh so no Git-Bash/WSL is needed on Windows — role service accounts + PATs, gitlab-<role>.token files with an owner-only Windows ACL, GITLAB_API_BASE recorded, plus a forge-neutral create-labels path for GitLab; dry-run first, refuse on partial state"
  - "docs/adopting-assay-gitlab.md:200-201 — 'The script itself is bash + curl + jq. On native Windows run it from Git-Bash or WSL, not from PowerShell.'"
  - "docs/adopting-assay.md:904-906 — the Windows prerequisites list Git-Bash/WSL for GitLab fleet provisioning explicitly: 'Native PowerShell cannot run it.'"
  - "tools/create-fleet-gitlab.sh:1-60 — the script's own header: seven service accounts, memberships, PATs, avatars, protected branch, approvals, protected tags, merge gates, labels; the REST v4 endpoint list is enumerated once at lines 37-55"
  - "tools/create-fleet-gitlab.sh:79-87 (ROLE_TABLE) — the seven roles and their access levels/scopes: reviewer/worker/verifier/desk (developer:30), issue-loop/intake-loop (reporter:20), board-writer (developer:30); scopes api or api,write_repository"
  - "tools/create-fleet-gitlab.sh:746-750 — the custody write today: `( umask 077; printf '%s' \"$token\" > \"$token_file\" )` then `chmod 0600`, then a path-only echo ('path printed, value never echoed')"
  - "tools/create-fleet-gitlab.sh:311,333,403 — the token is passed to curl through a 0600 `curl -K` config file minted under `umask 077`, NEVER on the command line or in the environment"
  - "tools/create-fleet-gitlab.sh:67-68,205,270-272 — the --dry-run contract: 'NEVER hits live infrastructure unless invoked without --dry-run and with GITLAB_TOKEN set; --dry-run makes zero network calls'"
  - "tools/create-fleet-gitlab.sh:14-20 — the tier-safety property to preserve: the protected-branch step never leaves `main` unprotected across a failure, and every settings step runs even when an earlier one fails, with failures collected and reported before a non-zero exit"
  - "tools/desk/internal/deskkit/custodyowner_windows.go:19-25 — the Windows ACL custody pattern this brief must reuse: windowsFileACLModel(path) then evaluateCustodyACL, NOT a mode-bit check"
  - "tools/desk/internal/deskkit/custodyacl.go:1-28 — why: 'os.FileMode's permission bits are synthetic on Windows — a normal file reads 0666 — so the unix 0600 test rejects a token file that is correctly locked by an owner-only ACL (#667)'; a could-not-determine input REFUSES rather than passing"
  - "tools/desk/internal/deskkit/custodyowner_unix.go — the POSIX half behind the same boundary, so a port keeps one guarantee with two implementations rather than two guarantees"
  - "docs/adopting-assay-gitlab.md:176-198 — the token-file NAMING mismatch the port should close: the script writes `<prefix>-<role>-bot.token`, `desktoken --forge gitlab <role>` reads `gitlab-<role>.token`, and today the adopter links or copies them by hand (Windows: copies, 'no ln -s required')"
  - "docs/adopting-assay-gitlab.md:203-231 — GITLAB_API_BASE: required, no fallback, a plain environment variable read via os.Getenv at call time, and explicitly NOT a roster.env key (roster.env only recognises the ASSAY_* allowlist and fails closed on an unrecognised key in that namespace)"
  - "docs/adopting-assay.md:702-755 (PRIMITIVE: create-labels) — the GitHub-only label primitive: nine `gh label create` calls for review-request, six raised-by:<role> stamps, and the authorization-needed/approval-needed PR-state pair"
  - "tools/create-fleet-gitlab.sh:143 (LABEL_TABLE) and :1104-1114 — the GitLab label creation that exists ONLY inside this script, via POST /projects/:id/labels, project-scoped and only inside the --project block"
  - "docs/streams/windows-port/portability-audit.md:38 — the audit's only mention of create-fleet-gitlab.sh is the `/tmp` row; it was never triaged as an adopter-facing needs-port surface, which is the gap this brief closes"
  - "freshness-checked 2026-09-11 @ 35316469 (origin/main): tools/create-fleet-gitlab.sh is 1167 lines of bash; no Go desk verb provisions a GitLab fleet; `ls tools/desk/cmd/` shows no fleet/labels command"
consumers:
  - "tools/desk/cmd/: follow-up windows-port/08 (this brief; the new verb's package flips to fixed-here when the implementation lands it)"
  - "tools/desk/internal/deskkit/custodyacl.go: out-of-scope (this brief CALLS the existing owner-only ACL evaluation; changing the custody decision itself is a different, security-gated change)"
  - "tools/create-fleet-gitlab.sh: out-of-scope (the bash script stays as the reference implementation and the Unix path until the Go verb has run a real provisioning; retiring it is a separate decision, not this brief's)"
  - "docs/adopting-assay-gitlab.md: follow-up windows-port/09 (the doc collapse owns replacing the Git-Bash/WSL prerequisite with the verb)"
  - "docs/adopting-assay.md: follow-up windows-port/09 (the Windows prerequisites list and the create-labels PRIMITIVE's forge-neutral note)"
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/09 (§Scope's acquisition-only wording is 09's edit)"
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (c). (a): the custody model on Windows is a design decision the facts do not
  pre-specify — it is the `## Human decision` fork. (c): this is credential-handling code where a
  subtle error survives the brief's own tests — a token that reaches an argv, an environment
  variable, a debug log, or a file whose DACL was never tightened all produce a green run and a
  leaked fleet credential.
version: 1
id: e3b5f1aa-b73a-40e7-b011-179b99ea51cf
---

# Brief 08 — Go-native GitLab fleet provisioning

## Context

files:
- **create** a new command under `tools/desk/cmd/` (name it for what it does — fleet provisioning
  — and say the chosen name in the PR body), plus its tests.
- **create** the forge-neutral label path — either as a mode of that command or as a sibling
  command; it must serve BOTH forges, since the GitHub side exists today only as nine hand-run
  `gh label create` lines in a doc.
- **create** `changelog/<branch-slug>.md` — this repo enforces a per-PR fragment.
- **do NOT** edit `tools/create-fleet-gitlab.sh` — it stays as the reference implementation and
  the Unix path; retiring it is a separate decision.
- **do NOT** edit `tools/desk/internal/deskkit/custodyacl.go` or `custodyowner_*.go` — this brief
  CALLS that guarantee, it does not change it.
- **do NOT** edit `docs/adopting-assay-gitlab.md`, `docs/adopting-assay.md`, or
  `plugins/assay/skills/install/SKILL.md` — `windows-port/09` owns the doc collapse.

facts:
- **The seven roles, their access levels and their scopes** are a table in the script
  (`tools/create-fleet-gitlab.sh:79-87`), not prose: `reviewer:developer:30:api`,
  `worker:developer:30:api,write_repository`, `verifier:developer:30:api,write_repository`,
  `desk:developer:30:api`, `issue-loop:reporter:20:api`, `intake-loop:reporter:20:api`,
  `board-writer:developer:30:api,write_repository`. The port carries the table across unchanged;
  a scope widened in translation is a privilege escalation nobody asked for.
- **The REST surface is enumerated once, at `tools/create-fleet-gitlab.sh:37-55`** — group resolve
  and tier probe, service-account list/create, PAT mint, membership get/create, `PUT /user/avatar`
  as the role's OWN PAT, project resolve, protected-branch get/patch/delete/post, approvals
  get/post, protected-tags get/post, project `PUT` for the merge checks, and
  `POST /projects/:id/labels`. That list is the port's contract; it is not re-derived from the
  code body.
- **The custody write today, and what it does NOT do on Windows.**
  `tools/create-fleet-gitlab.sh:746-750` writes the token under `umask 077` and then `chmod 0600`,
  printing only the path. `chmod` is exactly what the repo's own Windows work says does not work:
  `custodyacl.go:15-25` — "os.FileMode's permission bits are synthetic on Windows — a normal file
  reads 0666 — so the unix 0600 test rejects a token file that is correctly locked by an owner-only
  ACL (#667)", and `docs/adopting-assay.md:914-926` tells the adopter outright not to document
  `chmod` as the Windows fix.
- **The pattern to reuse, verbatim in shape.**
  `tools/desk/internal/deskkit/custodyowner_windows.go:19-25` is the whole Windows half:
  `windowsFileACLModel(path)` → `evaluateCustodyACL(path, model)`, behind a `//go:build windows`
  boundary whose unix twin is `custodyowner_unix.go`, with an identical signature so callers stay
  platform-agnostic. `evaluateCustodyACL` REFUSES on a could-not-determine input rather than
  passing (`custodyacl.go:21-25,29-40`). The port's job is to CALL that, on the file it just wrote,
  before declaring the provisioning successful — not to re-derive an ACL opinion.
- **The credential never touches argv or the environment.** The script routes it through a 0600
  `curl -K` config file minted under `umask 077` (`:311,333,403`) and unsets it after writing
  (`:749`). A Go port has a stronger native form — an `Authorization`/`PRIVATE-TOKEN` header on an
  `http.Request` — and must use it; it must NOT reintroduce the credential as a subprocess
  argument, an env var, or a formatted error string.
- **`--dry-run` is a hard offline contract, not a preview.** `tools/create-fleet-gitlab.sh:67-68`:
  "NEVER hits live infrastructure unless invoked without `--dry-run` and with `GITLAB_TOKEN` set;
  `--dry-run` makes zero network calls." Preserve it exactly — it is what makes this brief
  verifiable at all by an offline agent.
- **Partial state is the named hazard, and the script already has an answer.**
  `tools/create-fleet-gitlab.sh:14-20`: the protected-branch step "NEVER leaves `main` unprotected
  across a failure — it prefers a no-op or a PATCH, and where a DELETE+POST is unavoidable it
  re-applies the rule it read if the POST is refused"; every settings step runs even when an
  earlier one fails, failures are collected, reported at the end, and the script exits non-zero.
  The port inherits both properties.
- **`GITLAB_API_BASE` has no fallback, by design.** `docs/adopting-assay-gitlab.md:203-215`: every
  GitLab-side token operation refuses before any network contact without it, because "unlike
  GitHub's fixed `api.github.com`, GitLab is commonly self-hosted, so a default host would risk
  sending a role's live PAT to a guessed target". It is read via `os.Getenv("GITLAB_API_BASE")` at
  call time and is explicitly NOT a `roster.env` key — `roster.env` recognises only the `ASSAY_*`
  allowlist and fails the whole shared file closed on an unrecognised key in that namespace
  (`:218-222`). "Records" it therefore means: emit the exact export line for the operator's shell,
  never write it into `roster.env`.
- **The token-file naming mismatch this port should close.** The script writes
  `<prefix>-<role>-bot.token`; `desktoken --forge gitlab <role>` looks for `gitlab-<role>.token`;
  today the adopter links (Unix) or copies (Windows) them by hand
  (`docs/adopting-assay-gitlab.md:176-198`). The driver's ask names `gitlab-<role>.token` as the
  written name — write what the reader reads, and the manual step disappears.
- **The GitHub label primitive has no forge-neutral twin.** `docs/adopting-assay.md:702-755` is nine
  hand-run `gh label create` invocations: `review-request`, six `raised-by:<role>` stamps
  (`desk`, `worker`, `reviewer`, `verifier`, `issue-loop`, `intake-loop`), and the
  `authorization-needed` / `approval-needed` PR-state pair. The GitLab equivalents exist only
  inside `create-fleet-gitlab.sh`'s `LABEL_TABLE` (`:143`, applied at `:1104-1114`), project-scoped
  and reachable only inside its `--project` block. The doc states the cost of a missing label:
  "it degrades silently, and the information the label carried is lost".
- **The audit did not cover this surface.** `docs/streams/windows-port/portability-audit.md:38`
  mentions `create-fleet-gitlab.sh` only in the `/tmp` row, inheriting its script's disposition. No
  row triages it as an adopter-facing `needs-port`. That omission is why a Windows GitLab adopter
  still hits a bash prerequisite after the whole stream shipped — record it as a finding rather
  than silently patching over it.

single-point-of-failure: the owner-only custody check on the written `gitlab-<role>.token` file is
the ONE control between a minted fleet PAT and any other principal on the machine reading it.
Two independent layers behind it, and the design must keep them independent rather than collapsing
to one: (1) the file is CREATED restricted — an owner-only DACL applied at creation (the Windows
equivalent of `umask 077` before the write, not a widen-then-tighten), so a crash between create
and check never leaves a world-readable token on disk; (2) `deskkit.VerifyCustodyOwnerOnly` is
called on the written file BEFORE the path is reported as usable, and it refuses a
could-not-determine input — a different component, on a different signal (the DACL as READ BACK
from the filesystem, versus the intent expressed at creation), catching the case where the
creation-time restriction did not take effect. Layer 3, out-of-band and already shipped: every
desk verb re-runs the same custody evaluation when it READS the token
(`tools/desk/internal/deskkit/custodyowner_windows.go`), so a file whose ACL is loosened after
provisioning is refused at use time by code this brief does not touch. Row 8 proves layer 2 with
layer 1 bypassed.

## Human decision
<!-- gate: human, decision-trigger: creation — filed as a self-contained decision issue.
     Written to be decided from THIS text alone: no links, no repo paths, no brief refs. -->
Provisioning a team of automation accounts mints one long-lived access token per role and writes
each to a file on the operator's machine. Every automation verb then reads its own token from that
file. On Unix the rule is simple and long-settled: create the file readable and writable by its
owner only, and refuse to read one that is not. Windows has no equivalent of those permission
bits — a file that looks correctly locked down by the Unix rule can be wide open, and a file that
is genuinely locked down can look wrong — so the rule has to be restated in terms of the Windows
access-control list. The tooling already has an owner-only access-list check used when a token is
READ. What is being decided is what happens at the moment a token is WRITTEN, and how strict to be
when the answer cannot be established.

Pick one before the provisioning is built. The choice fixes the credential-handling posture every
future Windows operator inherits, and it is not cheaply reversible once tokens are in the field.

Options:

1. **Create restricted, verify, and refuse the whole run if the verification cannot be
   established.** The file is created with an access list granting only its owner (plus the
   operating system's own trusted administrative accounts), the tool immediately reads the access
   list back, and if it cannot confirm owner-only access — for any reason, including "this
   filesystem cannot report it" — the run stops, reports which credential is affected, and tells
   the operator the credential must be revoked. Pros: the strictest posture, and it matches how
   the same tooling already behaves when READING a credential, so an operator never meets two
   different standards for the same file. A credential that cannot be proven private is treated as
   compromised, which is the correct default for a long-lived access token. Cons: an operator on
   an unusual filesystem — a network share, a synchronised folder, some container mounts — may be
   unable to provision at all, with a refusal rather than a workaround. Expect support questions.

2. **Create restricted, verify, and WARN rather than refuse when the verification cannot be
   established.** Identical up to the point of an inconclusive read-back, at which the tool prints
   a prominent warning naming the file and continues. Pros: an operator on an unusual filesystem
   can still finish. Cons: the warning is printed once into a long provisioning log for seven
   credentials and will be missed; and it introduces a second, weaker standard than the one the
   read path applies, so a credential provisioned with a warning may be refused later at use time
   anyway — the operator meets the failure at a worse moment, with the token already minted and
   live.

3. **Do not write credentials to files on Windows at all — hand them to the operating system's
   own credential store** and have the verbs read from there. Pros: no file permissions question
   exists; it is the platform-native answer and the store is designed for exactly this. Cons: it
   is a second, platform-specific storage mechanism to build and maintain alongside the file path
   every other platform uses; every reading verb needs a matching change; and it is a much larger
   piece of work than the provisioning this decision is attached to — it would need its own plan.

Recommendation: **Option 1.** It makes the write path match the read path exactly, so there is one
standard rather than two, and it fails at the safest moment — before a credential is reported as
usable — rather than after it is in the field. Option 3 is a reasonable future direction and
should be recorded as such, but pinning this work behind it would stall a straightforward port
behind a much larger one.

Also confirm, in the same ruling: when a run fails partway through, having already minted some
credentials, should the tool attempt to revoke what it minted, or stop and report exactly which
credentials exist and must be revoked by hand? Attempting revocation is more automatic but can
itself fail halfway; reporting is always truthful but leaves work for the operator.

Default if no answer: none — blocks until answered. The credential-handling posture cannot be
guessed; guessing bakes in exactly the decision this gate exists to make.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- **You run OFFLINE against live infrastructure.** No command or script you run may contact a
  GitLab instance, read-only included. Everything in the Verify table below is exercisable against
  a local stub or under `--dry-run`; anything that needs a live group is could-not-check plus
  `BLOCKED-ON-HUMAN` on the PR, never a probe.
- **Never put a credential in argv, an environment variable, a log line, or an error string.** Only
  a PATH is ever printed. A test that prints a fixture token is still a test that taught the code
  to print a token — assert on the absence, do not demonstrate the presence.
- **Do NOT weaken the custody check to make a test pass.** Per the security-gate rule, removing or
  softening a custody or access-control assertion is `BLOCKED-ON-HUMAN` + `needs-decision`, not a
  shortcut — even if it is the fix for a red check, and even if this brief seems to ask for it.
- **Do NOT begin building until the `## Human decision` fork is ruled** (recorded on the decision
  issue). Report NEEDS_CONTEXT if picked up before the ruling.
- Do not widen a role's access level or token scopes in translation. The table is carried, not
  re-derived.
- Stop at `implemented` — you do not set verified/done. This is `gate: human`; a model does not
  sign it off.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **(Human gate) Obtain the custody ruling** (and the partial-run answer) recorded on the decision
   issue before writing credential-handling code.
2. **Build the verb with `--dry-run` as the default-safe mode**, preserving the existing contract
   exactly: `--dry-run` enumerates every action it WOULD take and makes zero network calls. The
   enumeration names each role, access level, scopes, the membership, the PAT, and each label.
3. **Port the role table verbatim** from the seven `ROLE_TABLE` rows, including access levels
   (30/20) and scopes (`api`, `api,write_repository`).
4. **Implement the REST calls over `net/http`** against the endpoint list at the script's
   `:37-55`, with the credential carried as a request HEADER — never a subprocess argument, never
   an environment variable, never interpolated into a URL.
5. **Refuse before any network contact when `GITLAB_API_BASE` is unset**, reading it via
   `os.Getenv` at call time. Do not invent a default host. On success, EMIT the export line the
   operator must add to the shell that runs the desk verbs; do NOT write it into `roster.env`.
6. **Write each credential as `gitlab-<role>.token`** in the resolved config home — the name
   `desktoken --forge gitlab <role>` already reads — so the manual link/copy step disappears.
7. **Apply the ruled custody model at creation, then verify the file as written** by calling the
   existing `deskkit` owner-only custody evaluation on it, before the path is reported as usable.
   Handle the inconclusive case exactly as the ruling says.
8. **Refuse on partial state, per the ruling**, and inherit the script's two stated properties:
   the protected-branch step never leaves `main` unprotected across a failure, and every settings
   step runs even when an earlier one fails, with failures collected, reported by name, and a
   non-zero exit.
9. **Add the forge-neutral label path.** One label set, two forge backends: the nine GitHub labels
   from the `create-labels` PRIMITIVE and their GitLab twins from the script's `LABEL_TABLE`, in
   ONE table in code so the two forges can never carry different sets. Idempotent — an existing
   label is a no-op, not an error.
10. **File a finding** recording that the portability audit never triaged
    `tools/create-fleet-gitlab.sh` as an adopter-facing `needs-port` surface, so the omission is
    on the record rather than silently patched.
11. **Add the changelog fragment** (`changelog/<branch-slug>.md`).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./cmd/<verb>/ && go vet ./cmd/<verb>/` (substitute the chosen command name) | exit 0 | `check` |
| 2 | **It cross-compiles for Windows** (the whole point of the port): `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/<verb>/` | exit 0 | `check` |
| 3 | **`--dry-run` makes ZERO network calls** — `cd tools/desk && go test ./cmd/<verb>/ -run 'TestFleetDryRunMakesNoNetworkCalls' -count=1 -timeout 120s`; the test injects an HTTP transport that FAILS the test on any request | exit 0 | `check +mutation` |
| 4 | **`--dry-run` enumerates all seven roles with their access levels and scopes** — `-run 'TestFleetDryRunEnumeratesRoleTable'` | exit 0; the enumeration names all seven roles, and each role's access level and scope string matches the ported table exactly | `check` |
| 5 | **Refuses before any network contact with `GITLAB_API_BASE` unset** — `-run 'TestFleetRefusesWithoutAPIBase'`; the same failing transport is installed | exit 0; a refusal naming the variable, and the transport was never reached | `check +mutation` |
| 6 | **No credential reaches argv, the environment, a log, or an error** — `-run 'TestFleetTokenNeverEscapes'`: run the full flow against a stub server returning a known sentinel token, capture stdout+stderr and every `exec` argv and env the process constructs, and assert the sentinel appears in NONE of them | exit 0 | `check +mutation` |
| 7 | **The token file is written under the ruled custody model and verified** — `-run 'TestFleetTokenFileCustody'` | exit 0; asserts the file is created restricted (not widened-then-tightened) and that the `deskkit` owner-only evaluation was called on it before the path was reported | `check` |
| 8 | **NEGATIVE PATH — custody verification failure is not survivable** — `-run 'TestFleetRefusesOnCustodyFailure'`: with layer 1 bypassed (the file created with a permissive model in the fixture), the run must behave exactly as the ruling requires (refuse, or warn-and-continue) and must NOT report the credential as usable on a refusal | exit 0 | `check +mutation` |
| 9 | **Fail-first for rows 6, 7 and 8** — run each against a mutation that removes the property it asserts (log the token; skip the custody call; treat a custody error as nil) and capture the RED | all three observed FAILING, pasted under `## Fail-first` in the PR body with the mutation each ran against | `check +mutation` |
| 10 | **Written name matches the read name** — `-run 'TestFleetWritesGitlabRoleTokenName'` | exit 0; the file for role `<r>` is `gitlab-<r>.token`, the exact name `desktoken --forge gitlab <role>` resolves | `check +flow` |
| 11 | **Partial-run behaviour per the ruling** — `-run 'TestFleetPartialRun'`: a stub that succeeds for the first N roles and fails for the next | exit 0; the tool behaves as ruled, names every credential that exists and must be dealt with, collects rather than aborts on the settings steps, and exits non-zero | `check` |
| 12 | **`main` is never left unprotected across a failure** — `-run 'TestFleetProtectedBranchNeverUnprotected'`: a stub that refuses the POST half of a delete+post | exit 0; the rule the tool read is re-applied, and the stub records `main` as protected at every observation point | `check +mutation` |
| 13 | **One label table, two forges** — `-run 'TestLabelsForgeParity'` | exit 0; the GitHub and GitLab backends are driven from the SAME table, and the test asserts the emitted set on each forge contains `review-request`, all six `raised-by:*` stamps, and the `authorization-needed`/`approval-needed` pair | `check +flow` |
| 14 | **Label creation is idempotent** — `-run 'TestLabelsIdempotent'`: a stub reporting the label already exists | exit 0; a no-op, not an error, and the run still exits 0 | `check` |
| 15 | **Dereference the ported role table against its source** (catches a well-formed table with a widened scope): `sed -n '/^ROLE_TABLE=/,/^'"'"'$/p' tools/create-fleet-gitlab.sh \| grep -E '^[a-z-]+:' \| sort` and compare, field for field, against the table the row-4 enumeration printed | the two agree exactly on role, access level and scope for all seven rows — no scope added, none dropped | `gate:model +dereference` |
| 16 | **Dereference the endpoint set against its source**: compare the paths the stub server observed across rows 3-14 against the enumerated list at `tools/create-fleet-gitlab.sh:37-55` | every observed path is in the enumerated list; any path outside it is a new REST surface the reviewer must weigh | `gate:model +dereference` |
| 17 | **A live provisioning run against a real GitLab group** — a human, in their own identity, with `--dry-run` first and then for real | the dry run's enumeration matches what the real run did; the seven token files exist with owner-only access; `desktoken --forge gitlab <role>` reads one successfully. **BLOCKED for any agent** — this contacts live infrastructure; it is the human gate's own row | `gate:human` |
| 18 | Consumers routing corroborated by the diff (run on the implementer's branch): `statusgen --root . --consumers windows-port/08; echo $?` | `0` | `check` |
| 19 | Board lint stays clean: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Row 17 is a LIVE row:
     an agent records it as could-not-check with the reason and never greens it from the
     stub rows. gate: human — the flip is not a model's. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). The reviewer's questions, answered from
evidence and not from reading: (1) What is the single control standing between a minted fleet PAT
and another principal on the machine, and is it acceptable? The Context names it and names two
layers behind it; row 8 is what proves the second layer catches with the first bypassed. (2) Does
any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? Rows 3, 5, 6,
8 and 12 each bypass or break one layer and assert the next one still holds; row 9 proves each of
those rows reddens when the property is removed, so none of them is a green lamp wired to nothing.
(3) Separately, and not derivable from the code: was the custody fork ruled BEFORE the build, and
does the implementation match the ruling — including the inconclusive-read-back case and the
partial-run answer?
