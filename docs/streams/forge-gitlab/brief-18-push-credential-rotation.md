---
brief: assay:assay:forge-gitlab:18
title: Rotate the board-push credential in one process — the new value never reaches a shell, a transcript, a log or argv
why: >-
  On GitLab the board regeneration job pushes with a project or group access token held in a
  masked, protected CI/CD variable. Access tokens expire, and when this one did (#2910) the
  job reported a racing default branch five times over instead of an expired credential. Part
  A of #2910, the push-failure classifier, makes that failure name itself. This brief is part B:
  there is no supported way to replace the token. Today a human mints the new value in the
  forge's settings page and pastes it into the variable, or a desk would have to carry it
  through a shell or a transcript. This brief designs one in-process path that rotates the
  token, writes the new value straight into the variable, keeps the variable masked, protected
  and (where chosen) hidden, and proves the new token works and is not about to expire before
  the next regeneration needs it. The value exists only in the forge and in the memory of one
  process.
wave: 5
depends: ["forge-gitlab/15"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The brief handles a live push credential to the default branch (sensitive-data: yes). The new
  verb holds a repository-write token in memory and writes it into a CI/CD variable, and the
  credential it runs under can rotate project access tokens and read the project's unhidden
  CI/CD variables. The human confirms the choices in `## Human decision`. These are the
  mechanism; the token level; who holds that rotator credential and whether a desk may run
  the verb; the token lifetime and the minimum remaining lifetime the proof demands; whether
  the variable is made hidden; and what counts as proof when the forge refuses
  self-introspection to a repository-only token. The design-approval record is owed before
  this brief leaves `todo`.
decision-trigger: creation
issues: [2910]
schema: brief-v2
version: 1
id: 22ce73f5-1c40-4fc9-9f58-fcad9624341d
supersedes: []
gates: []
feathers: []
authored: 2026-10-14 by worker-desk authoring session (#2910 part B, design only)
sources:
  - "#2910: the field report. The regen job pushed with an expired board-push token; every attempt answered HTTP Basic access denied, a direct token read answered 401 Token is expired, and the retry loop printed `push rejected: main moved` five times before blaming a fast-moving default branch. The issue asks for two things: a credential check before the push that names expired, revoked, wrong scope, non-fast-forward and other auth failures distinctly (part A, shipped as its own pull request), and a rotation path that keeps the new value inside GitLab, preserves masked and protected, and proves the new token active and unexpired before the next regen (part B, this brief)"
  - "docs/adopting-assay-gitlab.md §2c (Board-push credential — `STATUSGEN_PUSH_TOKEN`): token kind is a project or group access token, never a human PAT and never a §1 role-fleet token; scope `write_repository` only, never `api`; Maintainer (40) under a protected default branch; the variable masked AND protected, because a masked-only variable reaches merge-request pipelines"
  - "docs/adopting-assay-gitlab.md §5 (Token custody rules): rotate-on-mint, at most one valid credential, a short expiry backstop as a human-only setting, owner-only files, path-only printing, never an env variable or argv; §2g: the role-PAT renewal script, bash plus `glab`, which writes each new value to an owner-only file. That pattern does not fit a value whose destination is a CI/CD variable, not a file"
  - "tools/desk/cmd/deskfleet/main.go:1-45 and client.go:37-55, :159-175: the Go-native GitLab provisioning verb. Credentials come from FILES only and are never printed. `GITLAB_API_BASE` has no default host, plain http is accepted for loopback only, and no redirect is followed because a followed redirect would forward `PRIVATE-TOKEN` to whatever host the Location names. fleet_test.go:115 `TestFleetTokenNeverEscapes` is the sentinel pattern this brief reuses; cmd/deskfleet/mutations.json is its muhar spec"
  - "statusgen/init.go:775-793 (`initGitlabCI`): the scaffolded regen job's unset-token refusal and its push. Part A owns any change there; this brief changes nothing in it"
  - "GitLab REST API, as documented by the vendor at authoring and NOT dereferenced offline (Verify rows 19 and 20 dereference them on the target instance): project and group access token list, create, get-by-id, rotate (returns the new value once and revokes the predecessor) and revoke; personal access token self-information (`GET /personal_access_tokens/self`, fields id, active, revoked, expires_at, scopes); project CI/CD variables create, update and get (masked, protected, environment scope; hidden settable only at creation); Git smart-HTTP receive-pack advertisement, which requires write access"
  - "freshness-checked 2026-10-14 @ 9a8df91dc (origin/main): `deskfleet` has subcommands `provision` and `labels` and no push-token path; no non-test file under tools/desk/cmd/deskfleet imports os/exec; no tool in the tree writes `STATUSGEN_PUSH_TOKEN`; windows-port/16 (`deskfleet renew`, role PATs) is `todo`"
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (c). Who holds the rotator credential and what counts as proof are design
  decisions the facts do not settle. The dangerous mistakes all pass a happy-path test: a debug
  line that prints the value, a variable update that silently drops the protected flag, a
  liveness probe sent to a host named in a server response, an install failure that reports
  success, or a proof that reads the old token's record.
domain: complicated
outcome: none
tier: free
consumers:
  - "tools/desk/cmd/deskfleet/main.go: follow-up forge-gitlab/18 (the `push-token` subcommand, its usage text and dispatch; this brief's own Task 1 and 6, flipped to fixed-here by the implementing change)"
  - "docs/adopting-assay-gitlab.md: follow-up forge-gitlab/18 (§2c gains the rotation procedure; §5 gains one line saying the board-push credential rotates through this verb, not the role-PAT path; this brief's own Task 6, flipped to fixed-here by the implementing change)"
  - "statusgen/init.go: out-of-scope (the regen job's pre-push credential check and its distinct expired, revoked and wrong-scope outcomes are part A of #2910, in its own pull request; this brief adds nothing to the scaffold)"
  - "tools/renew-fleet-gitlab-tokens.sh: out-of-scope (it rotates role PATs, a different credential class; §2c forbids pointing the push variable at a role token)"
  - "docs/streams/windows-port/brief-16-deskfleet-renew-and-the-go-owned-fleet-tables.md: out-of-scope (coordination only: both add a `deskfleet` subcommand and edit its usage text; whichever lands second merges main)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the tier table, the wave line and the issue map)"
---

# Brief 18 — Rotate the board-push credential in one process

## Context

The scaffolded GitLab `statusgen-regen` job pushes the regenerated board with
`STATUSGEN_PUSH_TOKEN`. `docs/adopting-assay-gitlab.md` §2c fixes what that credential is: a
project or group access token with `write_repository` only, at Maintainer, held in a CI/CD
variable that is masked AND protected. It also fixes what it is not: a human PAT, or a role-fleet
token minted by `desktoken --forge gitlab <role>`. §5 fixes how this repository treats every
credential it rotates: rotate-on-mint, at most one valid credential, a short expiry, and the
value never in an env variable, argv or output.

Nothing applies §5 to this credential. The role-PAT renewal (§2g, and its Go port
windows-port/16) ends in an owner-only file on the operator's machine. The board-push token
does not belong in a file. It belongs in a forge-side variable that only protected pipelines
receive. So today rotation is a human pasting a fresh value from the forge's settings page
into the variable. A desk asked to help can do no better than carry the value through a shell
or its own transcript. #2910 is what happens when nobody rotates: the token expired and the
regen job misreported it as a race.

This brief designs `deskfleet push-token`. It is one Go process. It reads the rotator
credential from an owner-only file, rotates the access token, and installs the returned value
into the variable over the same TLS connection pool. It then re-reads the variable's flags and
proves the new token is live, with a minimum remaining lifetime. The value never leaves that
process except as the body of the variable write and as the credential of the two liveness
probes, both sent to the configured API host only.

**Relationship to part A.** Part A of #2910 is the push-failure classifier in the regen job,
shipped as its own pull request. It is not a brief, and this brief does not depend on it to be
built. The two are designed as independent layers for one property: the token in the variable
works when the regen job needs it. This verb proves liveness at rotation time, from the
operator's side, under the rotator credential and the new token. Part A checks it again at
every regen, from inside the job, under the token itself. If this verb is never run, or its
proof is skipped, part A still names an expired or revoked token instead of a racing default
branch.

files:
- `tools/desk/cmd/deskfleet/pushtoken.go` (new): the `push-token` subcommand, its `rotate` and
  `check` modes, the redacting secret type, preflight, install, read-back, containment and
  proof.
- `tools/desk/cmd/deskfleet/pushtoken_test.go` (new) and `tools/desk/cmd/deskfleet/fake_test.go`:
  the fake GitLab gains the access-token, variable, self-information and receive-pack routes.
- `tools/desk/cmd/deskfleet/main.go`: usage text and dispatch.
- `tools/desk/cmd/deskfleet/mutations.json`: the named mutations below.
- `docs/adopting-assay-gitlab.md`: §2c rotation procedure; one line in §5.
- `changelog/forge-gitlab-18-push-credential-rotation.md` (planned).

layering: flat tool, one new file in an existing command. The meaningful rules are pure
functions over decoded responses: preflight predicates, the flag comparison and the proof
comparator. The external effects stay in deskfleet's existing `apiClient`. No new package and
no deskkit change, because no other verb needs a CI-variable writer (Task 1, rows 3 and 6).

design-fit:
  owner: tools/desk/cmd/deskfleet (the Go owner of GitLab fleet provisioning, its API client and its credential-file custody)
  contract: none — no semantic-owner row covers CI-variable credential custody; S-identity is about which identity a session resolves to, not how a stored credential is replaced
  retires: ["the by-hand paste of a rotated value into the CI/CD variable (§2c), for rotation and first creation"]
  weight: "verbs +1 (`deskfleet push-token`, two modes); flags +6 (`--project`, `--rotator-token-file`, `--token-name`, `--days`, `--min-days`, `--create`; `--dry-run` reused); refusals +9 (Task 2); rule-text lines about +25 in §2c"
  why-add: >-
    It cannot live in the role-PAT renewal script or its port. Those end in an owner-only
    file, and a shell transport would put the value into a variable-update command's argv or
    stdin. It cannot live in the regen job, because writing a CI/CD variable needs `api` scope,
    which §2c forbids for the push token. The removal considered instead is retiring the stored
    credential altogether, letting the job push with its own job token where the instance
    supports that. It is Human decision choice 1, option C, and it would retire this verb. It
    is not the default because instance support and its interaction with protected branches
    are not established.

single-point-of-failure: the verb's in-process handling of the new value, the only place the plaintext exists outside the forge — behind it, three layers that fail on different signals in different components: (1) the forge's own masked and protected flags on the variable, re-read after every write and contained by revoking the new token when the read-back disagrees, so the value is redacted from job logs and reaches only protected-branch pipelines whatever the verb sent; (2) forge-side lifetime, because rotation revokes the predecessor and the new token carries a short expiry, so a value that escapes dies at the next rotation or at expiry; (3) for liveness, part A's pre-push credential check inside the regen job, which uses the token itself at regen time and names an expired or revoked token even when this verb's proof never ran.

facts:
- **Credential shape** (§2c, 2026-10-14 @ 9a8df91dc): a project or group access token,
  `write_repository` only, Maintainer (40); the variable masked AND protected. The push token
  cannot call the REST API, and must not be given `api`.
- **Rotator credential**: rotating a project access token and writing a project CI/CD variable
  both need a Maintainer-or-above credential with `api` scope. That credential can also read
  every unhidden variable of the project. Who holds it is Human decision choice 3.
- **deskfleet conventions** (2026-10-14 @ 9a8df91dc, `main.go:1-45`, `client.go`): credentials
  from files only, checked by `deskkit.ClassifyCustodyOwnerOnly`; `GITLAB_API_BASE` required,
  https or loopback http; redirects never followed; `--dry-run` makes zero network calls; exit 0
  ok, 1 a step failed, 2 usage, 5 refused. This brief adds 6 for could-not-check
  (`deskkit.ExitUnverifiable`), in `check` mode only.
- **Vendor behaviour this design leans on** (documented by the vendor at authoring, dereferenced
  on the target instance by rows 19 and 20, not offline):
  - rotate returns the new value once and revokes the predecessor at once;
  - hidden can be set only when a variable is created, and a hidden variable's value is never
    returned by the API;
  - updating a variable without a flag may reset that flag, so the verb always sends both;
  - the receive-pack advertisement answers 200 only for a credential that may write the
    repository. It does not test protected-branch rights, which are decided per ref update;
  - whether `GET /personal_access_tokens/self` answers a token whose only scope is
    `write_repository` is NOT established. Human decision choice 6 sets what happens if it
    does not.
- **Part A**: a separate pull request on #2910 that adds the regen job's pre-push credential
  check. This brief neither edits nor tests it (Ground rules).

### Threat model

Defended:
- The new value escapes through stdout, stderr, the run report, a log line, the process
  environment, argv, a temp file, or (when a desk runs the verb) the session transcript.
- The variable loses masked, protected or hidden during the update.
- The value is sent to a host other than the configured API host: a response-supplied clone
  URL, a redirect, a cleartext base.
- A token is installed that is revoked, inactive, wrongly scoped or close to expiry, and this
  is found only when the next regen fails.
- A rotation fails halfway: rotated but not installed, or installed but with wrong flags.
- A desk runs the verb by accident, or under an ambient credential.

Not defended, accepted as the trust root:
- Whoever holds the rotator credential. A Maintainer with `api` can rotate this token, read
  the project's unhidden variables and push directly. The verb narrows how that power is used
  for this one job; it does not reduce the power. Choice 3 decides who holds it.
- A compromised machine that can read the verb's process memory.
- A compromised forge.

Every defended item maps to a Verify row in the pre-mortem table below.

### Risk-files cross-read

`files:` sits under `tools/desk/cmd/deskfleet/`, a credential-handling path. The four answers
were read against it, not defaulted:
- **sensitive-data: yes.** The verb holds a repository-write credential in memory, and the
  rotator credential it runs under has `api` scope.
- **regulatory: no.** No regulated data is involved.
- **customer: no.** The only behaviour that changes is how one CI credential is replaced.
- **irreversible: no.** Rotation revokes the predecessor on purpose. A failed install is
  recovered by running the verb again, and the containment revoke is recovered by `--create`.
  Nothing outside the token and the variable changes.

## Edition

Minimum GitLab tier: **free** on a self-managed instance. The vendor documents project and group
access tokens on every self-managed tier, and as a paid feature on its hosted service. Rotate
and hidden variables need a recent server version. Neither tier nor version is asserted here
for any instance: row 19 reads them off the target before the decision is ratified. If the
target lacks hidden variables, Human decision choice 5 falls back to masked and protected,
which is §2c's floor.

## Human decision

The automation's board job pushes with a repository-write access token kept in a masked,
protected CI/CD variable. These tokens expire. Today the only way to replace one is for a
person to create a new value in the forge's settings page and paste it into the variable, or
for an automated session to carry the value through a shell or its own transcript. The proposal
is one command that rotates the token and writes the new value into the variable inside a
single process. The value is never printed, never placed in a command line, an environment
variable or a file, and never shown to the person or session running the command. The command
keeps the variable masked and protected, and proves the new token is live and not about to
expire. Six choices need a human answer. Each has a recommended default.

1. **Mechanism.** (A) A new subcommand of the existing Go provisioning tool that calls the
   forge's rotate endpoint and writes the variable in-process. (B) Extend the existing shell
   renewal script: rejected, because it would put the value into a variable-update command's
   arguments or input. (C) Stop storing a credential at all, if the instance can let the job
   push with its own short-lived job token: this would retire the command, but instance support
   and its protected-branch behaviour are not established. **Recommended: A**, with C recorded
   as a later investigation.
2. **Token level.** A project access token (one project) or a group access token (every
   project in the group). **Recommended: project**, the smaller blast radius.
3. **Who holds the rotator credential.** Rotating needs a Maintainer credential with full API
   scope, which can also read the project's unhidden CI/CD variables. (A) A person's own
   credential in an owner-only file, and that person runs the command. (B) A dedicated rotator
   service account whose credential sits in automation custody, so an automated session may
   run it. (C) An existing automation role account: rejected, because no role should gain
   project-variable access as a side effect. **Recommended: A**. B is a later decision once A
   has run cleanly.
4. **Lifetime and threshold.** **Recommended: 30-day tokens; the command refuses more than 90;
   the proof requires at least 7 days remaining and an expiry no earlier than one day before
   the requested date**; the read-only check mode alarms below 7 days.
5. **Hidden.** Hidden variables can never be read back through the UI or API, but the flag can
   be set only when the variable is created. **Recommended: yes**, by one human-run
   delete-and-recreate through the command's create path where the instance supports it;
   otherwise masked and protected, which is the existing floor.
6. **Proof when self-introspection is refused.** If the forge will not answer the "describe
   this token" endpoint for a repository-only token, (A) accept the rotator-side read of the
   same token record plus a live push-permission probe as proof, and say so in the output; or
   (B) fail the rotation. **Recommended: A**. Any other self-introspection failure still fails.

Options:
1. **Approve as briefed.** All six recommended defaults. The command is built as designed, and
   a design record is written from this answer.
2. **Approve with changes.** Name the choice and the option. The brief is revised before
   dispatch.
3. **Hold.** The credential keeps being rotated by hand, and its expiry is surfaced only by the
   board job's own failure.

Default if no answer: none — blocks until answered.

## Ground rules

- NEVER git push, trigger workflows, or call a live forge. Every forge interaction in this
  brief's tests is an `httptest` server, and the fake value is a sentinel string.
- Stop at `implemented`. You do not set verified or done. Rows 19 and 20 are human-run.
- Touch nothing under `statusgen/`, `.github/` or `ci/`. The regen job and its pre-push check
  belong to part A of #2910. Do not reimplement, test or wait for them here.
- Add no input path for any credential other than a file named by a flag. No stdin, no argv
  value, no environment variable. `GITLAB_API_BASE` is a URL, not a credential, and stays as
  `deskfleet` reads it today.
- Spawn no subprocess. Do not shell out to `git`, `glab` or `curl`; the receive-pack probe is
  an HTTP request made by the same client.
- Change nothing on the role-PAT paths (`provision`, `labels`, the renewal script,
  `desktoken`). Raise no ratchet ceiling in deskkit.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not guess a vendor
  behaviour: a fake server encodes the documented one, and rows 19 and 20 check it live.

## Task

### 1. The subcommand and its secret type

```
deskfleet push-token rotate --project <path-or-id> --rotator-token-file <file> --token-name <name>
                            [--days N] [--min-days N] [--create] [--dry-run]
deskfleet push-token check  --project <path-or-id> --rotator-token-file <file> --token-name <name>
                            [--min-days N]
```

- The variable key is the constant `STATUSGEN_PUSH_TOKEN`, not a flag, so the verb cannot be
  pointed at another variable.
- The returned value is decoded straight into a dedicated unexported type. Its `String`,
  `GoString`, `Format`, `MarshalJSON` and `MarshalText` all render `[redacted]`. The only way to
  read the bytes is one unexported method, which feeds two consumers: the variable-write body
  encoder and the probe request's credential. The byte slice is zeroed when the run ends.
- `pushtoken.go` calls no `os.Setenv`, `os.WriteFile`, `os.Create` or `os.OpenFile`, and no
  non-test file of the package imports `os/exec`. Row 3 guards this.

### 2. Refusals before any network contact (exit 5, zero requests)

`GITLAB_API_BASE` unset, not absolute, or not https outside loopback. The rotator file is not
owner-only, or is missing. `--days` is outside 1..90. `--min-days` is not below `--days`. A
`--token` or `--value` style flag is a usage error (exit 2), because the flag set has none. The
ambient variables `GITLAB_TOKEN` and `PRIVATE_TOKEN` are never read: setting them changes
nothing.

### 3. `rotate`: preflight reads, under the rotator credential

All of these come before any write. Any failure exits 5 with zero writes.
- **The variable.** Read the variable's metadata (any value field in the response is discarded
  unread). It must exist with environment scope `*`, masked true and protected true. The
  `hidden` state is recorded to be preserved. If the variable is missing, refuse unless
  `--create`.
- **The token.** List the project's active access tokens. Exactly one must carry
  `--token-name`, with scopes exactly `[write_repository]` and access level 40. Zero is refused
  unless `--create`. Two or more are refused, with a message saying the forge holds more than
  one valid push credential and the extras should be revoked by hand.
- **The branch.** The project's default branch must be protected. Otherwise a protected
  variable never reaches the regen job.

### 4. `rotate`: rotate (or create), install, read back, contain

1. Rotate the token with `expires_at` = today + `--days`. With `--create` and no active token,
   create one instead: name, `[write_repository]`, level 40, the same expiry. Keep the new
   token's id. The value lives only in the secret type.
2. Update the variable with value, `masked: true`, `protected: true` and scope `*` sent
   explicitly on every write. When creating the variable with `--create`, also send
   `masked_and_hidden: true` if Human decision choice 5 is yes and the instance supports it.
   Retry a transport error or 5xx at most twice. On final failure, exit 1 with
   `rotated-not-installed`. The report says the variable still holds the revoked predecessor
   and that re-running `rotate` is the recovery, because the new token is active and is found
   by name.
3. **Read back.** Re-read the variable. It must show masked true, protected true, scope `*` and
   the recorded hidden state. If a value is returned (an unhidden variable), compare it to the
   secret in constant time without printing either. **Containment**: on any read-back
   mismatch, revoke the new token by id at once, then exit 1 with `contained: revoked`, naming
   the field that disagreed. The variable then holds a dead value, and recovery is fixing the
   flags and re-running with `--create`.

### 5. `rotate` and `check`: the proof

`rotate` exits 0 only if all three probes pass. Each prints one line with the probe name and
the fields it compared, never a value:
- **by-id**, under the rotator credential, reading the new token's record: active true, revoked
  false, scopes exactly `[write_repository]`, `expires_at` no earlier than the requested date
  minus one day, and at least `--min-days` away.
- **self**, under the new token, `GET /personal_access_tokens/self`: the same fields, and its id
  equals the new id, so the proof cannot pass on the predecessor's record. A 403 naming
  insufficient scope prints `self: not-permitted-by-scope` and counts as passed only under
  Human decision choice 6 A. Any other failure fails the run.
- **push**, under the new token, `GET <API host>/<project path>.git/info/refs?service=git-receive-pack`
  with HTTP Basic: it must answer 200. The probe URL is built from `GITLAB_API_BASE`'s scheme
  and host and the project's path. A clone URL from any response is never used. A 3xx is a
  failure.

`check` makes reads only: no POST, PUT or DELETE. It finds the token by name and runs the by-id
probe against `--min-days`. Exit 0 healthy, 1 below threshold, revoked, inactive or wrongly
scoped, 6 when a read fails. It is the out-of-band monitor between rotations. It shares the
by-id comparator with `rotate`, so against a comparator defect it is not independent; part A
is the independent layer.

### 6. Report, docs, mutations

- The report, on stdout, has one line per step: project, variable key, flags, token name, new
  token id, new `expires_at`, outcome. No value is in it.
- Every network mode prints its target before its first request, as `deskfleet` already does.
- `--dry-run` enumerates the steps with zero requests.
- `docs/adopting-assay-gitlab.md` §2c: a "Rotating it" subsection with the procedure, the
  chosen defaults from the Human decision, the containment and recovery outcomes, and the
  `check` mode. §5: one line saying this credential rotates through `deskfleet push-token`, not
  the role-PAT path.
- `cmd/deskfleet/mutations.json`: add M1-M8 below. The existing control stays.

## Verify (executable — no prose-only DoD items)

Classes follow the stream convention: `check:ci` runs in CI, `check` is local, `gate:human` is
run by the human against the target instance after the decision. `+flow` exercises
verb → client → fake forge end to end. `+mutation` names the muhar mutation that turns it red.
`+dereference` checks a claim against the system it names. Every negative row satisfies every
layer except the one under test, and asserts the specific step or field name in the output, so
it cannot pass on another layer's refusal. Every planted fake value is a sentinel string that
the row also greps for in all captured output.

Test names are this brief's planned deliverables, all under `tools/desk/cmd/deskfleet`:
`TestPushTokenNeverEscapes` (planned),
`TestPushTokenSourceGuard` (planned),
`TestPushTokenRedacts` (planned),
`TestPushTokenRefusesInputs` (planned),
`TestPushTokenPreflight` (planned),
`TestPushTokenPreservesFlags` (planned),
`TestPushTokenReadbackCatches` (planned),
`TestPushTokenProvesLiveness` (planned),
`TestPushTokenSelfScope` (planned),
`TestPushTokenProbeHostBound` (planned),
`TestPushTokenInstallFails` (planned),
`TestPushTokenCheckMode` (planned),
`TestPushTokenDryRun` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./cmd/deskfleet/ && go test ./cmd/deskfleet/ -count=1 > "${TMPDIR:-/tmp}/b18-r1.out" 2>&1 && grep -E -e '^ok' "${TMPDIR:-/tmp}/b18-r1.out"` | exit 0; output contains one `ok` line for the deskfleet package |
| 2 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenNeverEscapes$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r2.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenNeverEscapes' "${TMPDIR:-/tmp}/b18-r2.out"` | exit 0, `--- PASS:` printed. A full `rotate` against the fake forge, plus the `rotated-not-installed` and `contained: revoked` runs. Neither the new-value sentinel nor the rotator sentinel appears in stdout, stderr, the report, the process environment, or any file created under the out dir or `TMPDIR` during the run. The fake records the new value in exactly three places: the variable-write body, and the credential of the self and push probe requests, all on the fake's host. Mutation M1 turns it red |
| 3 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenSourceGuard$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r3.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenSourceGuard' "${TMPDIR:-/tmp}/b18-r3.out"` | exit 0, `--- PASS:` printed. A `go/parser` walk: no non-test file in the package imports `os/exec`, and `pushtoken.go` calls none of `os.Setenv`, `os.WriteFile`, `os.Create`, `os.OpenFile`. **Negative path**: the same walker, run over a fixture file that plants each forbidden call, reports every one |
| 4 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenRedacts$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r4.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenRedacts' "${TMPDIR:-/tmp}/b18-r4.out"` | exit 0, `--- PASS:` printed. The secret type rendered through the verbs `%v`, `%+v`, `%#v`, `%s`, `%q`, through `json.Marshal` and through `encoding.TextMarshaler`, alone and inside a struct, yields `[redacted]` and never the sentinel. Mutation M2 turns it red |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenRefusesInputs$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r5.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenRefusesInputs' "${TMPDIR:-/tmp}/b18-r5.out"` | exit 0, `--- PASS:` printed. Cases: base unset, non-https non-loopback base, rotator file mode 0644, rotator file missing, `--days 0`, `--days 91`, `--min-days` equal to `--days`. Each exits 5 with zero fake requests. A `--token` flag exits 2. `GITLAB_TOKEN` and `PRIVATE_TOKEN` set to valid-looking sentinels with no rotator file still exit 5, and neither sentinel reaches the fake |
| 6 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenPreflight$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r6.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenPreflight' "${TMPDIR:-/tmp}/b18-r6.out"` | exit 0, `--- PASS:` printed. **Negative paths**, each with every other precondition met: variable unmasked; variable unprotected; variable scope not `*`; variable missing without `--create`; no active token by name without `--create`; two active tokens by name; scopes `write_repository` plus `api`; scopes `read_repository`; access level 30; default branch unprotected. Each exits 5 naming the failed check, and the fake records zero rotate, create, variable-write and revoke requests. Mutation M7 turns it red |
| 7 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenPreservesFlags$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r7.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenPreservesFlags' "${TMPDIR:-/tmp}/b18-r7.out"` | exit 0, `--- PASS:` printed. The fake implements update-resets-omitted-flags. Every variable write the fake records carries masked true, protected true and scope `*`. A hidden variable stays hidden, its value is never returned, and the run still exits 0 on the self and push probes. With `--create` and the hidden choice on, the create body carries the hidden flag. Mutation M3 turns it red |
| 8 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenReadbackCatches$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r8.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenReadbackCatches' "${TMPDIR:-/tmp}/b18-r8.out"` | exit 0, `--- PASS:` printed. **Lower layer with the upper bypassed**: the verb sends correct flags, but the fake stores and reports protected false, then masked false, then hidden false on a hidden variable, then a different value. Each case exits 1 with `contained: revoked` naming the field. The fake records one revoke of the new token id and holds no active token by that name afterwards. Mutations M4 and M8 turn it red |
| 9 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenProvesLiveness$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r9.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenProvesLiveness' "${TMPDIR:-/tmp}/b18-r9.out"` | exit 0, `--- PASS:` printed. Healthy run: exit 0 and three proof lines, `by-id`, `self` and `push`. **Negative paths**, one fault each: by-id revoked; by-id inactive; by-id scopes widened; by-id expiry 3 days away with `--min-days 7`; by-id expiry 5 days earlier than requested; self id equal to the predecessor's id; push probe 401; push probe 403. Each exits 1 naming the probe and field, never a value. Mutation M5 turns it red |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenSelfScope$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r10.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenSelfScope' "${TMPDIR:-/tmp}/b18-r10.out"` | exit 0, `--- PASS:` printed. A self answer of 403 insufficient scope, with by-id and push healthy, exits 0 and prints `self: not-permitted-by-scope`. A self answer of 401, or 403 with any other message, exits 1 |
| 11 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenProbeHostBound$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r11.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenProbeHostBound' "${TMPDIR:-/tmp}/b18-r11.out"` | exit 0, `--- PASS:` printed. The fake's project record names a clone URL on a second `httptest` server B. The push probe still goes to the API host: the fake records it, a precondition without which B's count proves nothing, and B records zero requests. A push probe that the fake answers with a 3xx to B exits 1, and B records zero requests. Mutation M6 turns it red |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenInstallFails$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r12.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenInstallFails' "${TMPDIR:-/tmp}/b18-r12.out"` | exit 0, `--- PASS:` printed. The variable write answers 500 three times: the run exits 1 with `rotated-not-installed` and the re-run instruction, after exactly three write attempts. A second run against the same fake state rotates the active token again, installs, proves and exits 0. At the end the fake holds exactly one active token by that name |
| 13 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenCheckMode$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r13.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenCheckMode' "${TMPDIR:-/tmp}/b18-r13.out"` | exit 0, `--- PASS:` printed. `check` makes zero POST, PUT and DELETE requests. Healthy exits 0; revoked exits 1; a token list answering 500 exits 6. **Lower layer with the upper bypassed**: the fake is seeded directly with a token expiring in 2 days, as if installed without `rotate`'s proof. `check` with `--min-days 7` exits 1 naming `expires_at`. Mutation M5 turns it red |
| 14 | check:ci | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestPushTokenDryRun$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r14.out" 2>&1 && grep -F -e '--- PASS: TestPushTokenDryRun' "${TMPDIR:-/tmp}/b18-r14.out"` | exit 0, `--- PASS:` printed. `rotate --dry-run` names every step of Task 3-5 and the fake records zero requests |
| 15 | check:ci | `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestFleetTokenNeverEscapes$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r15a.out" 2>&1 && grep -F -e '--- PASS: TestFleetTokenNeverEscapes' "${TMPDIR:-/tmp}/b18-r15a.out" && go test ./cmd/deskfleet/ -run '^TestNoAPIHostLiteral$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r15b.out" 2>&1 && grep -F -e '--- PASS: TestNoAPIHostLiteral' "${TMPDIR:-/tmp}/b18-r15b.out" && go test ./internal/deskkit/ -run '^TestNoAmbientEnvTokenRead$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r15c.out" 2>&1 && grep -F -e '--- PASS: TestNoAmbientEnvTokenRead' "${TMPDIR:-/tmp}/b18-r15c.out"` | exit 0, each `--- PASS:` line printed. The existing escape, host-literal and ambient-token guards stay green |
| 16 | check | `git fetch -q origin main && git diff --name-only "$(git merge-base FETCH_HEAD HEAD)" HEAD -- statusgen .github ci tools/desk/internal/deskkit/ambienttoken_guard_test.go tools/renew-fleet-gitlab-tokens.sh \| wc -l` | prints `0`: no scaffold, workflow or staged-workflow file, no ambient-token ratchet and no role-PAT script changed |
| 17 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskfleet/mutations.json` | exit 0. Baseline green, control caught, and M1-M8 each `CAUGHT`, with zero `NOT_CAUGHT` and zero could-not-mutate |
| 18 | check:ci +dereference | `cd tools/desk && go build -o "${TMPDIR:-/tmp}/b18-deskfleet" ./cmd/deskfleet && cd ../.. && "${TMPDIR:-/tmp}/b18-deskfleet" --help > "${TMPDIR:-/tmp}/b18-r18.out" 2>&1; grep -F -e 'deskfleet push-token rotate' "${TMPDIR:-/tmp}/b18-r18.out" && grep -F -e 'deskfleet push-token rotate' docs/adopting-assay-gitlab.md && grep -F -e 'deskfleet push-token check' docs/adopting-assay-gitlab.md && "${TMPDIR:-/tmp}/b18-deskfleet" push-token rotate --project p --rotator-token-file /nonexistent --token-name t --value x > "${TMPDIR:-/tmp}/b18-r18b.out" 2>&1; test "$?" -eq 2` | exit 0. The usage line the runbook quotes is the one the built binary prints, both modes are in §2c, and a value-bearing flag is a usage error on the real binary |
| 19 | gate:human +dereference | `GITLAB_API_BASE="$PT_API_BASE" deskfleet push-token check --project "$PT_PROJECT" --rotator-token-file "$PT_ROTATOR_FILE" --token-name "$PT_TOKEN_NAME"` | Run by the human on the target instance before ratifying, after reading the instance version and tier off its help page. Exit 0 or 1, with a `by-id` line and no value printed. The human records the version, whether hidden variables are offered, and the verdict on Human decision choices 5 and 6 |
| 20 | gate:human +dereference | `GITLAB_API_BASE="$PT_API_BASE" deskfleet push-token rotate --project "$PT_PROJECT" --rotator-token-file "$PT_ROTATOR_FILE" --token-name "$PT_TOKEN_NAME"` | Run by the human after implementation. Exit 0, three proof lines and a flags line showing masked, protected and the chosen hidden state. The terminal shows no value. The next `statusgen-regen` run on the default branch completes its push with no credential failure from part A's pre-push check |
| 21 | check | `statusgen --root . --consumers` | exit 0 on the implementing change; the two entries routed `follow-up forge-gitlab/18` are flipped to `fixed-here` there and corroborate against its diff, and the out-of-scope entries carry a reason |

## Named mutations

Each one goes into `cmd/deskfleet/mutations.json`.
- **M1**, row 2: a debug line prints the secret's raw bytes after rotate.
- **M2**, row 4: `String` on the secret type returns the raw value.
- **M3**, row 7: the variable-write body omits `protected`. Against the reset-on-omit fake, the
  read-back would also catch it; row 7 asserts the request body itself.
- **M4**, row 8: the read-back flag comparison is deleted.
- **M5**, rows 9 and 13: the `--min-days` comparison in the shared by-id comparator is deleted.
- **M6**, row 11: the push probe URL is taken from the project record's clone URL.
- **M7**, row 6: the preflight scope check accepts a scope list that contains
  `write_repository`, instead of one that equals it.
- **M8**, row 8: the containment revoke is deleted, so a mismatched read-back exits 1 and leaves
  the new token active.

## Pre-mortem → detection map

*"This shipped and was wrong. What went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| The new value is printed by a debug line, an error wrap or the report | rows 2, 4; M1, M2 |
| The value reaches argv, the environment, a temp file or a subprocess | rows 2, 3; Ground rules |
| A desk transcript shows the value because the desk ran the verb | row 2 (stdout and stderr are the transcript); row 20 live |
| The variable loses protected or masked because an update omitted it | row 7, M3 |
| The forge stores different flags than were sent, and the verb reports success | row 8, M4; containment revokes the token, M8 |
| Hidden is lost on update | rows 7, 8 |
| The value is sent to a host named in a response or a redirect | row 11, M6 |
| A cleartext base sends the rotator credential or the value in the clear | row 5 |
| An ambient token is used instead of the rotator file | rows 5, 15 |
| A widened or wrong token is rotated and installed | row 6, M7 |
| The installed token is revoked, inactive or close to expiry, and nobody knows until the regen | row 9, M5; row 13 out of band; part A in the job |
| The proof reads the predecessor's record and passes | row 9 (self id must equal the new id) |
| Self-introspection is refused for a repository-only token, so every rotation fails | row 10; Human decision choice 6; row 19 live |
| Rotated but the install failed, and the verb reports success or loses the token | row 12 |
| Rotation silently runs during `--dry-run` | row 14 |
| The implementation edits the regen job or weakens a guard | rows 15, 16 |
| The runbook quotes a usage line the binary does not have | row 18 |
| A consumer of the change is left unrouted | row 21 |
| The vendor behaviour the fake encodes is wrong on the target instance | rows 19, 20 (human, live); **no CI row**, because offline tests cannot prove vendor behaviour |
| The rotator credential holder misuses it | **no row**: accepted trust root (Threat model; Human decision choice 3) |
| Part A's pre-push check misclassifies an expired token | **no row here**: part A's own pull request carries its tests; this brief does not re-test another change |

## Dispatch checklist

```
[x] 1. Rows discriminate: rows 8, 9, 11 and 13 fake a forge that misbehaves after the verb acts correctly, so a confident wrong verb goes red.
[x] 2. Facts dated: repo facts @ 9a8df91dc on 2026-10-14; vendor facts marked undereferenced, with rows 19 and 20 dereferencing them.
[x] 3. Self-contained: endpoints, flags, exit codes, refusals and probe rules are all in Task.
[x] 4. Risk answers match files: a credential path; sensitive-data yes, the rest read and recorded no.
[x] 5. gate-why names the rotator credential's power and the six choices.
[x] 6. Effort M, exec-tier strong (a, c).
[x] 7. No shared value changes; consumers names the scaffold as part A's.
[x] 8. design-fit answered; why-add names the retire-the-credential alternative.
[x] 9. Pre-mortem run; three review-only rows carry their reason.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review

Gate: human (from frontmatter; sensitive-data: yes). The Human decision is ratified before
dispatch. A design-approval record, `design: DR-<slug>`, written from that ratification, is added
to this brief's frontmatter before it can move to in-progress. The brief carries no `design:`
key at authoring because no human has decided yet.

A reviewer of this credential-handling design answers both questions in the verdict:
1. What single control stands between a fault and the damage, and is it acceptable? Expected
   answer: for confidentiality, the verb's in-process handling of the value. The forge's
   masked and protected flags, checked by read-back with containment, and the short forge-side
   lifetime stand behind it. For liveness, the verb's three-probe proof, with part A's
   in-job check and the `check` mode behind it.
2. Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?
   Expected answer: row 8 (the verb sends correct flags, the forge stores wrong ones, and
   read-back plus containment catch it); row 13 (a token installed without `rotate`'s proof is
   caught by `check`); row 11 (a response that names another host is not obeyed, and the probe
   still goes to the configured host). Part A's in-job layer is proven by its own pull request,
   not here.

The reviewer also confirms from the diff that nothing under `statusgen/`, `.github/` or `ci/`
changed, and that no credential input other than `--rotator-token-file` exists.
