---
brief: assay:assay:windows-port:16
title: deskfleet renew + the Go-owned fleet tables — port the GitLab PAT renewal, single-source the role table in Go
why: >-
  A Windows adopter on GitLab can provision a fleet natively once windows-port/08's `deskfleet` lands,
  but renewing it still takes bash. Every role PAT expires after 7 days by default
  (`FLEET_PAT_DAYS=7`, `tools/fleet-gitlab-roles.sh:41`), and the only one-shot renewal is
  `tools/renew-fleet-gitlab-tokens.sh`, a 714-line bash + `glab` + `jq` script added by #1635 after
  brief 08 was written. So a Windows fleet needs Git-Bash or WSL every week, not just once. #1635 also
  moved the role table into a second bash file (`tools/fleet-gitlab-roles.sh`) that the two scripts
  source. The Go verb then carries a third copy, kept in step only by a parity test. This brief
  ports the renewal into `deskfleet` under the same custody guarantees as brief 08's provisioning.
  It also makes Go the owner of the role, label and project-settings tables, so brief 17 can retire
  the bash path without losing anything.
wave: 2
depends: ["windows-port/08"]
unblocks: ["windows-port/17"]
effort: L
gate: human
gate-why: >-
  The ported verb ROTATES LIVE CREDENTIALS. A rotation invalidates the role's previous PAT the moment
  GitLab accepts it, so it cannot be undone (`irreversible: yes`). The new secret is written to disk
  as the file every desk verb reads (`sensitive-data: yes`). A custody defect does not fail a test.
  It leaks a live `api`/`write_repository` token, or it leaves a live desk session holding a revoked
  one. What the human confirms: (1) the `## Human decision` ruling on explicit role records;
  (2) that the renewal writes under exactly the custody model windows-port/08 records for
  provisioning, one standard and not two; (3) that no credential reaches argv, the environment, a
  log line or an error, and that the owner credential is read from a custody-checked file and never
  from a flag or env var; (4) that every preflight refusal happens before the first rotation, so a
  refused run has rotated nothing.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
decision-trigger: creation
outcome: none
issues: [1630, 1646]
schema: brief-v2
version: 1
authored: 2026-09-25 by windows-port authoring session (driver ask, 2026-09-25 — Go-only GitLab fleet work for Windows adopters)
sources:
  - "driver's ask (2026-09-25): Windows adopters should never need bash for GitLab fleet work; port the renewal into deskfleet with brief 08's custody properties, and make deskfleet own the role table"
  - "tools/renew-fleet-gitlab-tokens.sh:16-27 — the group-Owner authority model and its seven endpoints (GET groups/:id, GET user, GET groups/:id/members/all/:user_id, GET groups/:id/service_accounts, GET …/personal_access_tokens?state=active, POST …/:token_id/rotate, POST …/personal_access_tokens); never instance admin"
  - "tools/renew-fleet-gitlab-tokens.sh:29-33 — order of operations: every check runs before the first mutation; any preflight problem aborts with nothing rotated"
  - "tools/renew-fleet-gitlab-tokens.sh:35-44 — secret custody: owner-only temp file in the destination's directory, exactly-one-token-line validation, atomic rename; a symlinked destination is written THROUGH to a target that must sit in --out-dir"
  - "tools/renew-fleet-gitlab-tokens.sh:46-50,702 — not globally atomic: stop at the first failed role and print the `--only` resume list"
  - "tools/renew-fleet-gitlab-tokens.sh:76-83,712 — IN_USE_WINDOW_DAYS=1: a PAT used within the window is skipped unless --rotate-in-use; null last_used_at = never used; a record missing the key fails closed"
  - "tools/renew-fleet-gitlab-tokens.sh:85-140 — the flag surface: --hostname, --group, --prefix, --role ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE], --out-dir, --duration (1d..365d, :201), --only, --rotate-in-use, --dry-run; auth is whatever `glab` is logged in as (:134-136)"
  - "tools/renew-fleet-gitlab-tokens_test.sh:222-615 — the behavioural contract as tests T1-T18 (rotate all seven, weeks duration, create fallback, duplicate refusal, non-Owner/subgroup/unauthenticated refusal, malformed secret and listing, unwritable out-dir, partial run + resume, dry-run zero mutations, symlink custody, single-source table, argument preflight, explicit --role records, default 7d, empty/paginated listing, no instance-admin probe, in-use protection)"
  - "tools/fleet-gitlab-roles.sh:25-53 — ROLE_TABLE (seven rows), FLEET_PAT_DAYS=7, and the naming helpers fleet_username (<prefix>-<role>-bot), fleet_pat_name (assay-<role>-fleet), fleet_token_file (gitlab-<role>.token)"
  - "tools/create-fleet-gitlab.sh:79-89 — the provisioner sources the same file; :121 MERGE_ACCESS_LEVEL=40; :127-128 PROTECTED_TAG_GLOB='*' / PROTECTED_TAG_CREATE_LEVEL=40; :142-152 LABEL_TABLE; :807-825 protect_body (premium arrays incl. allowed_to_unprotect 50; free scalars push 0 / merge 40 / allow_force_push false); :964 approvals body; :1046 merge-check body; :758 the NOTICE that sends an operator to the bash renewal"
  - "PR #1572 (windows-port/08's implementation, open draft at authoring, head be0a55835): tools/desk/cmd/deskfleet/tables.go holds fleetRoles/fleetLabels/settings constants inside package main; tables_parity_test.go compares roles, labels and PAT days against the bash sources but NOT the protected-branch, protected-tag, approvals or merge-check values, and SKIPS as could-not-check when tools/create-fleet-gitlab.sh is absent"
  - "tools/desk/internal/avatar/generate.go:93 (and its test at avatar_test.go:53) — a second Go list of the seven role names (the avatar glyph table), maintained by hand"
  - "freshness-checked 2026-09-25 @ 798c88868 (origin/main): renew-fleet-gitlab-tokens.sh and fleet-gitlab-roles.sh present (714 and 53 lines); no deskfleet on main (tools/desk/cmd/ has no fleet command; PR #1572 unmerged)"
  - "post-review freshness recheck 2026-09-25 @ 0fa823227 (origin/main): PR #1572 MERGED (merge commit 6ced25a97e9a7ad47b2a287fe1c37b7a31a821be); tools/desk/cmd/deskfleet/ now carries tables.go, tables_parity_test.go, provision.go, labels.go, avatars.go, client.go, custody.go/custody_unix.go/custody_windows.go, project.go and fake_test.go"
consumers:
  - "tools/desk/cmd/deskfleet/: follow-up windows-port/16 (this brief; the renew subcommand and the move of the tables out of package main land here)"
  - "tools/desk/internal/: follow-up windows-port/16 (this brief; the new importable fleet-table package is created here)"
  - "tools/desk/internal/avatar/: follow-up windows-port/16 (this brief; its role list is checked against the Go-owned table)"
  - "tools/desk/README.md: follow-up windows-port/16 (this brief; documents `deskfleet renew`)"
  - "docs/adopting-assay-gitlab.md: follow-up windows-port/16 (this brief adds `deskfleet renew` to §2g beside the script; windows-port/17 removes the script)"
  - "tools/renew-fleet-gitlab-tokens.sh: follow-up windows-port/17 (stays unedited here as the parity oracle; retired there)"
  - "tools/fleet-gitlab-roles.sh: follow-up windows-port/17 (stays unedited here as the parity oracle; retired there)"
  - "tools/create-fleet-gitlab.sh: follow-up windows-port/17 (stays unedited here as the parity oracle; retired there)"
  - "tools/desk/internal/deskkit/custodyacl.go: out-of-scope (this brief CALLS the existing owner-only custody evaluation and the verdict helper windows-port/08 adds; changing either is a separate, security-gated change)"
exec-tier: strong
exec-tier-why: >-
  Question (c): credential-rotation code, where a token that reaches an argv, an env var, a log line
  or a loosened file, or a rotation that runs before a preflight refusal, still gives a green stub
  run. Question (b): the tables move across package boundaries, and three readers (provision, renew,
  avatars) must agree on them.
domain: complicated
id: 4fe790c9-8daa-4d17-9987-207436c0bb7f
---

# Brief 16 — `deskfleet renew` + the Go-owned fleet tables

## Context

files:
- **create** `tools/desk/cmd/deskfleet/renew.go` (planned) and `renew_test.go` (planned) (the `renew` subcommand), plus
  any stub-server additions to windows-port/08's `fake_test.go` (planned).
- **create** one importable package under `tools/desk/internal/` (name it for what it holds, the
  fleet tables, and give the name in the PR body). **Move** `fleetRoles`, `fleetLabels`, the PAT-day
  default, the naming helpers and the project-settings constants into it from
  `tools/desk/cmd/deskfleet/tables.go` (planned) — created by windows-port/08.
- **edit** `tools/desk/cmd/deskfleet/{main.go,provision.go,labels.go,tables.go,tables_parity_test.go}`
  so they read the moved package and so the parity tests cover the settings they miss today.
- **edit** `tools/desk/internal/avatar/avatar_test.go` (or add a test beside it) so the avatar role
  list is checked against the Go-owned table.
- **edit** `tools/desk/README.md` and `docs/adopting-assay-gitlab.md` §2g (add the verb; leave the
  script's text in place, because brief 17 removes it).
- **create** `changelog/<branch-slug>.md`.
- **do NOT** edit `tools/renew-fleet-gitlab-tokens.sh`, `tools/fleet-gitlab-roles.sh`,
  `tools/create-fleet-gitlab.sh` or their `_test.sh` suites. In this brief they are the parity
  oracle. windows-port/17 retires them.
- **do NOT** edit `tools/desk/internal/deskkit/custodyacl.go` or `custodyowner_*.go`.

facts:
- **Prerequisite state (2026-09-25).** windows-port/08's implementation, `deskfleet` (`provision`,
  `labels`), was PR #1572 — open and unmerged at authoring; it **MERGED to main 2026-09-25** at
  `6ced25a97e9a7ad47b2a287fe1c37b7a31a821be`. The landed-on-main half of the prerequisite is now
  satisfied; re-read `tools/desk/cmd/deskfleet/` on main at pickup (it now carries `tables.go`,
  `tables_parity_test.go` with `TestRoleTableMatchesBash`, `TestLabelTableMatchesBash` and
  `TestPATDaysDefaultMatchesBash`, `provision.go`, `labels.go`, `avatars.go` and the custody
  helpers), not the old PR branch. The other half of the Ground rules' gate — the `## Human
  decision` below being ruled — is still open.
- **The role table, carried unchanged** (`tools/fleet-gitlab-roles.sh:25-33`):
  `reviewer:developer:30:api`, `worker:developer:30:api,write_repository`,
  `verifier:developer:30:api,write_repository`, `desk:developer:30:api`,
  `issue-loop:reporter:20:api`, `intake-loop:reporter:20:api`,
  `board-writer:developer:30:api,write_repository`. Naming: username `<prefix>-<role>-bot`, PAT name
  `assay-<role>-fleet`, file `gitlab-<role>.token`; default lifetime 7 days (`:41-53`). A scope
  widened in translation is a privilege escalation.
- **The renewal's contract is its test suite.** `tools/renew-fleet-gitlab-tokens_test.sh` T1-T18
  (`:222-615`) enumerates the behaviour. Port the behaviour, not the bash. The Go tests name each T
  they carry, so a reviewer can walk the 18 against the port.
- **Authority, in Go.** The script authenticates as whatever `glab` is logged in as and "never
  reads, stores or passes that credential" (`:134-136`). A Go verb has no `glab`, so it reads the
  group-Owner credential the way `deskfleet provision` does: from `--owner-token-file`, which must
  pass the owner-only custody check. The credential is never a flag value or an env var. It then
  checks Owner (access level 50) on the TOP-LEVEL group before any mutation (`:498`), and never
  probes instance administration (no `application/settings`, no `users?username=`).
- **Base URL, in Go.** `--hostname` is replaced by `GITLAB_API_BASE`, read with `os.Getenv` at call
  time with no default host, the same as `deskfleet provision`. It is unset → refuse before any
  network contact.
- **`--dry-run` differs between the two subcommands, and the help text must say so.** Provision's
  `--dry-run` makes ZERO network calls (windows-port/08). The renewal's `--dry-run` cannot plan
  without reading the live PAT listings (rotate, create or skip-in-use per role), so it keeps the
  script's meaning (`:129-131`): every preflight GET, zero mutating requests, zero files written.
- **Custody (layer 1 + layer 2), reused from windows-port/08.** The replacement secret goes
  straight from the HTTP response into a file CREATED restricted (O_EXCL + 0600 on unix; CREATE_NEW
  with an owner-only DACL on Windows, via the same create helper `deskfleet` already uses) in the
  destination's directory. It is validated as exactly one token-shaped value, then atomically
  renamed onto the destination, and `deskkit`'s owner-only custody verdict is read back on the
  written file before the role is reported renewed. An inconclusive or definite custody result is
  handled exactly as windows-port/08's recorded ruling handles it for provisioning. Do not re-derive
  an ACL opinion.
- **Symlinked destinations** (the script's T11). A `gitlab-<role>.token` that is a link is written
  THROUGH to its target. A target outside `--out-dir`, or two roles resolving to one target, is a
  preflight refusal.
- **What #1572's parity tests miss.** They compare roles, labels and the PAT-day default against
  bash, but not the project settings: `main` protection (premium arrays with
  `allowed_to_unprotect` 50; free scalars `push_access_level` 0, `merge_access_level` 40,
  `allow_force_push` false; `create-fleet-gitlab.sh:807-825`), protected tags (`*`, create level
  40; `:127-128`), approvals (`merge_requests_author_approval` false,
  `merge_requests_disable_committers_approval` true; `:964`), and the two merge checks (`:1046`).
  They also SKIP when the bash sources are absent. That is right for a mutation-harness copy of
  the tree. It is also why the parity guard must become a golden test before brief 17 deletes the
  oracle.

single-point-of-failure: the owner-only custody check on each REPLACED `gitlab-<role>.token` is the
one control between a freshly rotated fleet PAT and any other principal on the machine. The layers
behind it are the same ones windows-port/08 established, and they stay independent. (1) The
replacement is CREATED restricted (never widened then tightened) and atomically renamed, so the
old file's looser ACL is never inherited. (2) `deskkit`'s custody verdict is read back from the
filesystem on the written file before success is reported. That is a different component on a
different signal. (3) Out of band and already shipped, every desk verb re-checks custody when it
READS the token. Row 7 proves layer 2 with layer 1 bypassed. A second, separate single point is
the ORDER of operations: every refusal must happen before the first rotation. Row 8 breaks each
preflight input and asserts zero mutating requests reached the stub.

## Human decision
<!-- gate: human, decision-trigger: creation — self-contained; no links, paths or brief refs. -->
The automation fleet's access tokens expire weekly and are renewed by a tool that rotates each
role's live token and writes the new one to a file. The renewal is being rebuilt as part of the
main compiled toolchain so it runs natively on Windows. The fleet's list of roles, with each
role's permission level and token scopes, is now meant to live in exactly one place in that
toolchain, so the tool that creates the fleet and the tool that renews it can never disagree.

The existing renewal also accepts EXPLICIT role records on its command line: a role name, account
name, token name, token scopes and file name, supplied by the operator. A record can override a
standard role's row, including its scopes, or add a role that is not in the standard list at all
(an "auditor", say). This lets an installation that did not use the standard naming still renew.
It also means a token's scopes can be set, or widened, outside the single list.

Pick one:

1. **Keep explicit records, but only for NAMING.** An operator may override a standard role's
   account name, token name and file name. Scopes always come from the single list, and a role not
   in the list is refused. Pros: non-standard naming still works, and no token's permissions can
   differ from the one reviewed list. Cons: an installation that added its own extra roles can no
   longer renew them with this tool and must rotate those tokens by hand.
2. **Keep explicit records exactly as today**, scopes and extra roles included. Pros: nothing an
   operator does today stops working. Cons: the single list is single in name only. Any renewal
   can set any scopes, and a typo in a scope list widens a live token's permissions with nothing
   to catch it.
3. **Drop explicit records.** Only the standard list and naming are renewable. Pros: the smallest,
   most auditable tool. Cons: any installation with non-standard naming must re-provision or rotate
   by hand.

Recommendation: **Option 1.** Naming is where installations legitimately differ. Permissions are
what a single reviewed list exists to hold still.

Default if no answer: none — blocks until answered, because it fixes which token permissions
the tool is allowed to grant.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- **You run OFFLINE against live infrastructure.** Nothing you run may contact a GitLab instance,
  read-only included. Every Verify row below runs against a local stub. The live renewal is
  windows-port/17's row, not this brief's.
- **Never put a credential in argv, an environment variable, a log line, or an error string.** Only
  a path is ever printed. Tests assert that the sentinel is absent. They never print it.
- **Do NOT weaken a custody or access-control check to make a test pass.** That is
  `BLOCKED-ON-HUMAN` + `needs-decision`, not a shortcut.
- **Do NOT begin until windows-port/08's implementation is on main and the `## Human decision` is
  ruled.** Report NEEDS_CONTEXT if picked up before either.
- Do not widen a role's access level or scopes in translation. The table is moved, not re-derived.
- Stop at `implemented`. This is `gate: human`, and a model does not sign it off.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Don't guess.

## Task
1. **Move the tables into one importable package** under `tools/desk/internal/`: roles, labels,
   the PAT-day default, the naming helpers (`<prefix>-<role>-bot`, `assay-<role>-fleet`,
   `gitlab-<role>.token`) and the project-settings constants. `deskfleet provision`, `labels` and
   `renew` all read it. No Go file outside `_test.go` may read, parse or name
   `fleet-gitlab-roles.sh`.
2. **Close the avatar list's drift.** Add a test that fails when the avatar glyph table's role set
   differs from the Go-owned role table's, in either direction.
3. **Extend the parity tests** so every settings value in the facts above is compared against its
   bash literal: `main` protection (both forms), protected tags, approvals and merge checks. Name
   every bash-comparing test `Test…MatchesBash`. Add a **golden** test
   (`TestFleetTablesGolden` (planned)) that pins every role, label and settings value to an independent
   literal copy inside the test. It must not depend on the bash files, so it keeps guarding after
   brief 17 deletes them.
4. **Add `deskfleet renew`** with the flag surface `--group`, `--prefix`, `--owner-token-file`,
   `--out-dir`, `--duration` (`<N>d`/`<N>w`, 1d..365d, default the table's 7 days), `--only`,
   `--rotate-in-use`, `--dry-run`, plus explicit role records as ruled. Behaviour, T1-T18:
   group-Owner authority on a top-level group; rotate the active PAT matched by its stable name;
   create one only when none is active; REFUSE in preflight on more than one active PAT of a name,
   a malformed or empty listing, a record missing `last_used_at`, an unwritable `--out-dir`, a
   non-Owner, a subgroup, or an out-of-dir or shared symlink target; merge paginated listings; skip
   in-use PATs (1-day window) unless `--rotate-in-use`; stop at the first failed role and print the
   `--only` resume list; report only role, path and outcome.
5. **Re-point provision's pre-existing-account NOTICE** at `deskfleet renew`.
6. **Document** `renew` in `tools/desk/README.md`, and add it to `docs/adopting-assay-gitlab.md`
   §2g as the native path beside the script.
7. **Changelog fragment.**

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskfleet/ ./internal/...` | exit 0 | `check` |
| 2 | **Cross-compiles for Windows:** `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskfleet/` | exit 0 | `check` |
| 3 | **Renew `--dry-run` makes zero mutations:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewDryRun$' -count=1 -v > o3.out; s=$?; grep -q -- '--- PASS: TestRenewDryRun (' o3.out && test $s -eq 0` (the stub FAILS the test on any non-GET request; the `-v` + PASS-line grep proves the named test actually ran — `go test -run` with no match also exits 0) | exit 0; the plan names rotate, create or skip-in-use per role, and no file under `--out-dir` changed | `check +mutation` |
| 4 | **Refuses before any network contact with `GITLAB_API_BASE` unset:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewRefusesWithoutAPIBase$' -count=1 -v > o4.out; s=$?; grep -q -- '--- PASS: TestRenewRefusesWithoutAPIBase (' o4.out && test $s -eq 0` | exit 0; a refusal naming the variable, and the transport was never reached | `check +mutation` |
| 5 | **No credential escapes:** `cd tools/desk && go test ./cmd/deskfleet/ -run 'TestRenewTokenNeverEscapes' -count=1`. It covers the owner credential and the rotated sentinel across stdout, stderr, every error, every constructed argv and env, and leftover temp files | exit 0 | `check +mutation` |
| 6 | **Replacement written restricted, atomically, and verified:** `cd tools/desk && go test ./cmd/deskfleet/ -run 'TestRenewCustody' -count=1` | exit 0; created restricted (never widened then tightened); renamed onto the destination; custody verdict read before `rotated` is reported; symlink written through; out-of-dir and shared targets refused in preflight | `check` |
| 7 | **NEGATIVE PATH — layer 1 bypassed:** `cd tools/desk && go test ./cmd/deskfleet/ -run 'TestRenewRefusesOnCustodyFailure' -count=1` (the fixture creates the file permissively) | exit 0; the behaviour matches windows-port/08's ruling, and a refused role is never reported renewed | `check +mutation` |
| 8 | **Every refusal precedes the first rotation:** `cd tools/desk && go test ./cmd/deskfleet/ -run 'TestRenewPreflight' -count=1`. It covers duplicate active PAT, non-Owner, subgroup, unauthenticated owner file, malformed secret, malformed listing, missing `last_used_at`, empty listing, unwritable out-dir, bad `--duration`, and `--only` naming an unknown role | exit 0; each case exits non-zero, the stub recorded ZERO `rotate`/create POSTs, and no destination changed | `check +mutation` |
| 9 | **Partial run and resume:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewPartialRun$' -count=1 -v > o9.out; s=$?; grep -q -- '--- PASS: TestRenewPartialRun (' o9.out && test $s -eq 0` | exit 0; the run stops at the failing role and prints `--only <remaining>`; the resume renews exactly those roles | `check` |
| 10 | **In-use protection, pagination, no admin probe:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewBehaviour$' -count=1 -v > o10.out; s=$?; grep -q -- '--- PASS: TestRenewBehaviour (' o10.out && test $s -eq 0` | exit 0; in-use role skipped by default and rotated with `--rotate-in-use`; a null `last_used_at` rotates; a two-page listing rotates the page-two PAT; no `application/settings` or `users?username=` path observed | `check` |
| 11 | **Explicit role records as ruled:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewRoleRecords$' -count=1 -v > o11.out; s=$?; grep -q -- '--- PASS: TestRenewRoleRecords (' o11.out && test $s -eq 0` | exit 0; behaviour matches the recorded ruling (under Option 1, a scope override and an unknown role are both refused) | `check +mutation` |
| 12 | **Parity against the bash sources, none skipped, every required name present:** `cd tools/desk && go test ./cmd/deskfleet/ -run 'MatchesBash' -count=1 -v > parity.out; s=$?; for t in TestRoleTableMatchesBash TestLabelTableMatchesBash TestPATDaysDefaultMatchesBash TestMainProtectionMatchesBash TestProtectedTagsMatchesBash TestApprovalsMatchesBash TestMergeChecksMatchesBash; do grep -q -- "--- PASS: $t (" parity.out \|\| exit 1; done; ! grep -q -- '--- SKIP' parity.out && test $s -eq 0` | exit 0; each of the seven named tests shows PASS (roles, labels, PAT days — carried from #1572 — plus the four this brief adds: `main` protection covering both forms, protected tags, approvals and merge checks), none skipped | `check +dereference` |
| 13 | **Golden guard independent of bash:** `cd tools/desk && go test ./cmd/deskfleet/ ./internal/... -run 'TestFleetTablesGolden' -count=1` | exit 0 | `check` |
| 14 | **Avatar list tracks the table:** `cd tools/desk && go test ./internal/avatar/ -run '^TestAvatarRoleListMatchesFleetTable$' -count=1 -v > o14.out; s=$?; grep -q -- '--- PASS: TestAvatarRoleListMatchesFleetTable (' o14.out && test $s -eq 0` | exit 0; the named drift test exists and passes | `check +neighbour` |
| 15 | **Fail-first for rows 5, 6, 7, 8, 12 and 13:** re-run each against a mutation that removes what it asserts. The mutations: log the sentinel; skip the custody read-back; treat a custody error as nil; move one preflight check after the first rotate; delete the `board-writer` row from the Go table; set the Go merge access level to 30 | every run observed FAILING. The deleted-role mutation must redden BOTH row 12 and row 13. Paste the output under `## Fail-first` in the PR body with each mutation named | `check +mutation` |
| 16 | **No Go production file reads the bash role table:** `git grep -n -e fleet-gitlab-roles -- 'tools/desk/*.go' ':!*_test.go'; test $? -eq 1` | exit 0 (grep found nothing). Positive control: `git grep -q -e fleet-gitlab-roles -- 'tools/desk/*_test.go'` exits 0, because the parity test still reads it | `check` |
| 17 | **Dereference the renewal endpoints:** compare the paths the stub observed across rows 3-11 with the list at `tools/renew-fleet-gitlab-tokens.sh:19-25` | every observed path is on that list. Any other path is a new REST surface the reviewer must weigh | `gate:model +dereference` |
| 18 | Consumers routing corroborated by the diff (implementer's branch): `statusgen --root . --consumers windows-port/16` | exit 0 | `check` |
| 19 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |
| 20 | **Renew → read flow:** `cd tools/desk && go test ./cmd/deskfleet/ -run '^TestRenewedTokenReadByDesktoken$' -count=1 -v > o20.out; s=$?; grep -q -- '--- PASS: TestRenewedTokenReadByDesktoken (' o20.out && test $s -eq 0`. After a stubbed renewal, the token is loaded through the same `deskkit` read path `desktoken --forge gitlab <role>` uses (`--no-rotate`, since this reads, not rotates) | exit 0; the reader resolves `gitlab-<role>.token`, its custody check passes, and it yields the rotated sentinel, compared in memory and never printed | `check +flow` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). The live renewal is
     windows-port/17's row, not this brief's; gate: human — the flip is not a model's. -->

## Review
Gate: **human** (from frontmatter — `irreversible: yes`, `sensitive-data: yes`). The reviewer
answers two questions. (1) What single control stands between a rotated PAT and another principal
on the machine, and is it acceptable? The Context names it; row 7 proves layer 2 with layer 1
bypassed, and row 8 proves the ordering control. (2) Does a lower layer catch the fault with the
upper one bypassed? Rows 7, 8 and 15 are that proof. Separately: was the explicit-records fork
ruled before the build, and does row 11 match the ruling?
