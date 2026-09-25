---
brief: assay:assay:windows-port:17
title: Go-only GitLab fleet — prove deskfleet live, then retire the bash fleet scripts and every doc that names them
why: >-
  Once windows-port/08 and windows-port/16 land, the GitLab fleet has two complete implementations:
  the Go `deskfleet` verb and three bash scripts (`create-fleet-gitlab.sh`,
  `renew-fleet-gitlab-tokens.sh`, and the `fleet-gitlab-roles.sh` table they share). Two
  implementations of a credential-minting tool are two custody surfaces to review, two role tables
  to keep equal, and two runbooks. Windows adopters are still told Git-Bash or WSL is a fallback,
  which keeps a bash dependency in the documented path. Brief 08 kept the bash script "until the Go
  verb has run a real provisioning; retiring it is a separate decision". This brief is that
  decision's work. It proves `deskfleet` against a real GitLab instance, confirms parity one last
  time, then deletes the scripts and rewrites every doc and runbook that names them. After it, no
  GitLab fleet operation needs bash.
wave: 5
depends: ["windows-port/16", "windows-port/09"]
unblocks: []
effort: M
gate: human
gate-why: >-
  Two risk answers are yes. `customer: yes`: adopters run these scripts from their own checkout, and
  deleting them removes a documented path. That includes the Git-Bash/WSL fallback windows-port/09
  keeps at an adopter's request (#1646, which also asked that bash stay the Unix source of truth).
  An adopter pinned to the scripts meets the removal on their next upgrade. `sensitive-data: yes`:
  the live proof mints and rotates real fleet PATs. What the human confirms: (1) the `## Human
  decision` ruling on the paid-tier lane that exists only in bash; (2) that the live proof really
  ran, on native Windows, and its class outcomes are recorded; (3) that nothing is deleted on a
  could-not-check live proof. `irreversible: no`: the deletion is a git revert away, and the proof
  runs against a disposable group whose tokens are revoked afterwards.
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: yes}
decision-trigger: creation
outcome: none
issues: [1646]
schema: brief-v2
version: 1
authored: 2026-09-25 by windows-port authoring session (driver ask, 2026-09-25 — Go-only GitLab fleet work for Windows adopters)
sources:
  - "driver's ask (2026-09-25): prove deskfleet live, then delete create-fleet-gitlab.sh, fleet-gitlab-roles.sh and renew-fleet-gitlab-tokens.sh and update every doc and runbook naming them, including windows-port/09's Git-Bash/WSL fallback wording; retirement never proceeds on a could-not-check live proof"
  - "docs/streams/windows-port/brief-08-go-native-gitlab-fleet-provisioning.md:60,86-87 — brief 08 keeps the bash script as the reference implementation and Unix path 'until the Go verb has run a real provisioning; retiring it is a separate decision, not this brief's'"
  - "docs/streams/windows-port/brief-09-three-command-install-docs.md:182-188 (Task 6) — keeps `tools/create-fleet-gitlab.sh` from Git-Bash or WSL as one clearly labelled fallback line, requested on #1646"
  - "windows-port/16 (assay:assay:windows-port:16) routes tools/create-fleet-gitlab.sh, tools/renew-fleet-gitlab-tokens.sh and tools/fleet-gitlab-roles.sh here as follow-ups: they stay unedited there as the parity oracle and are retired by this brief"
  - "#1646 — the filing that asked for a native Windows fleet path while keeping the bash script as the Unix source of truth and Git-Bash/WSL as a fallback"
  - "tools/create-fleet-gitlab.sh:106-117,470-500 — the `--tier ultimate` lane (custom reviewer role via POST /groups/:id/member_roles + member binding; external status check via POST /projects/:id/external_status_checks; --status-check-url/--status-check-name) — present only in bash; PR #1572's deskfleet usage has no --tier flag"
  - "tools/desk/internal/deskkit/forge_gitlab.go:1997-2001 — the verdict lane's could-not-check message tells the operator to run `create-fleet-gitlab.sh --tier ultimate`: a Go consumer of the script by name"
  - "tools/create-fleet-gitlab_test.sh and tools/renew-fleet-gitlab-tokens_test.sh — offline bash suites for the two scripts; no workflow under .github/workflows runs either (grep, 2026-09-25)"
  - "reference sweep, 2026-09-25 @ 798c88868 — `git grep -l -e create-fleet-gitlab -e fleet-gitlab-roles -e renew-fleet-gitlab-tokens` outside CHANGELOG.md, changelog/ and docs/streams/: docs/adopting-assay-gitlab.md (:12,:121,:188-228,:410-432,:757-800,:861), docs/adopting-assay.md (:259,:827,:894,:909,:1095-1097), plugins/assay/skills/adopt/SKILL.md (:51,:54), plugins/assay/skills/install/SKILL.md (:51,:86,:88), tools/desk/internal/deskkit/forge_gitlab.go (:1997-2001), plus the five tools/*fleet* files themselves"
  - "PR #1572 (windows-port/08 implementation, open draft at authoring): tools/desk/cmd/deskfleet/tables_parity_test.go reads the bash sources and SKIPS as could-not-check when tools/create-fleet-gitlab.sh is absent — after deletion that is a silent skip unless removed"
  - "freshness-checked 2026-09-25 @ 798c88868 (origin/main): all five tools/*fleet*.sh files present; no deskfleet on main; windows-port/08, /09 and /16 all todo"
  - "post-review freshness recheck 2026-09-25 @ 0fa823227 (origin/main): PR #1572 (windows-port/08's implementation) MERGED (merge commit 6ced25a97e9a7ad47b2a287fe1c37b7a31a821be); all five tools/*fleet*.sh files still present (this brief, not #1572, retires them); windows-port/09 and /16 remain todo"
consumers:
  - "tools/create-fleet-gitlab.sh: follow-up windows-port/17 (this brief; deleted)"
  - "tools/create-fleet-gitlab_test.sh: follow-up windows-port/17 (this brief; deleted)"
  - "tools/renew-fleet-gitlab-tokens.sh: follow-up windows-port/17 (this brief; deleted)"
  - "tools/renew-fleet-gitlab-tokens_test.sh: follow-up windows-port/17 (this brief; deleted)"
  - "tools/fleet-gitlab-roles.sh: follow-up windows-port/17 (this brief; deleted — the Go-owned table from windows-port/16 is the only copy)"
  - "tools/desk/cmd/deskfleet/tables_parity_test.go: follow-up windows-port/17 (this brief; the bash-comparing tests are removed, the golden test from windows-port/16 stays)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: follow-up windows-port/17 (this brief; the --tier ultimate message is re-pointed per the ruling)"
  - "docs/adopting-assay-gitlab.md: follow-up windows-port/17 (this brief; §2, §2b, §2g and the parity table rewritten around deskfleet)"
  - "docs/adopting-assay.md: follow-up windows-port/17 (this brief; the Windows prerequisites lose the fleet Git-Bash/WSL fallback, the create-labels and approvals notes name the verb)"
  - "plugins/assay/skills/adopt/SKILL.md: follow-up windows-port/17 (this brief)"
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/17 (this brief)"
  - "CHANGELOG.md: out-of-scope (released history; it records what shipped under the old names and is not rewritten)"
  - "docs/streams/: out-of-scope (brief, finding and pilot records are history; a done brief's Verify/Evidence is never edited to drop a name it ran against)"
exec-tier: strong
exec-tier-why: >-
  Question (b): the retirement sweeps a pattern across docs, two skills and one Go message, and a
  missed site leaves an adopter a command that no longer exists. Question (a): the paid-tier lane's
  fate is a design choice the facts leave open, which is why it is the `## Human decision`.
domain: complicated
id: a164160b-9849-4188-a296-2fde438db66b
---

# Brief 17 — Go-only GitLab fleet: prove live, then retire the bash scripts

## Context

files:
- **delete** `tools/create-fleet-gitlab.sh`, `tools/create-fleet-gitlab_test.sh`,
  `tools/renew-fleet-gitlab-tokens.sh`, `tools/renew-fleet-gitlab-tokens_test.sh`,
  `tools/fleet-gitlab-roles.sh`.
- **edit** `tools/desk/cmd/deskfleet/tables_parity_test.go` (planned) — created by windows-port/08 — (remove the bash-comparing tests and
  their skip helper), and `tools/desk/internal/deskkit/forge_gitlab.go` (the `--tier ultimate`
  message, per the ruling).
- **edit** `docs/adopting-assay-gitlab.md`, `docs/adopting-assay.md`,
  `plugins/assay/skills/adopt/SKILL.md`, `plugins/assay/skills/install/SKILL.md`, and any other
  file the sweep in row 6 finds at pickup.
- **create** `changelog/<branch-slug>.md` (a `### Removed` note naming the three scripts and the
  verb that replaces each).
- **do NOT** edit `CHANGELOG.md` or anything under `docs/streams/`. Those are records.

facts:
- **Prerequisite state (2026-09-25).** windows-port/08's implementation (PR #1572, open and
  unmerged at authoring) **MERGED to main 2026-09-25** at `6ced25a97e9a7ad47b2a287fe1c37b7a31a821be`.
  windows-port/16 (renewal plus the Go-owned tables) and windows-port/09 (the doc collapse that
  adds the labelled fallback line) remain `todo`. This brief needs all three merged, and only
  windows-port/08's landing is satisfied so far. The fallback line it removes is written by 09.
- **What the Go verb deliberately does differently.** These are not parity gaps, and the runbook
  rewrite must describe them. `--gitlab-url` becomes `GITLAB_API_BASE` (no default host). The
  owner credential comes from `--owner-token-file`, not `GITLAB_TOKEN`. Token files land as
  `gitlab-<role>.token` in the config home, so the link/copy step is gone. There is no default
  web fetch of avatar icons. The renewal's `--hostname` becomes `GITLAB_API_BASE`, and `glab` is
  no longer a prerequisite.
- **What exists ONLY in bash** (the gap this brief must not delete silently). `--tier ultimate`,
  with `--status-check-url` and `--status-check-name`, registers the paid-tier custom reviewer role
  and the external status check the verdict lane posts to (`create-fleet-gitlab.sh:470-500`). The
  forge seam's own could-not-check message sends the operator to that flag
  (`forge_gitlab.go:1997-2001`). Its fate is the `## Human decision`.
- **The parity guard must survive the deletion.** After the bash files go, #1572's
  `…MatchesBash` tests would SKIP as could-not-check. That is a green lamp wired to nothing. They
  are removed in the same commit as the deletion. The windows-port/16 golden test
  (`TestFleetTablesGolden` (planned)) is the guard that remains, and row 3 proves it reddens on a missing
  role.
- **Git-Bash stays for one thing.** The Claude Code SessionStart hooks still need `bash` + `jq`
  (`docs/adopting-assay.md` Windows prerequisites). Only the fleet-provisioning half of that
  prerequisite goes. The hooks half stays, and row 8 guards it.

single-point-of-failure: the live proof (row 1) is the one control between "the Go verb passes
stub tests" and "adopters lose the only implementation that has ever run against a real instance".
The layers behind it are independent. (1) Parity against the bash sources at the pre-deletion
commit (row 2) compares tables, not behaviour, so it fails for a different reason. (2) The golden
test that stays after deletion (row 3) catches a later edit rather than a translation error.
(3) The reference sweep (rows 5-6) catches a doc that still sends an adopter to a deleted command.
None of the three can stand in for row 1. Row 1 is could-not-check → NOTHING is deleted.

## Human decision
<!-- gate: human, decision-trigger: creation — self-contained; no links, paths or brief refs. -->
The automation fleet on GitLab has been provisioned and renewed by shell scripts. A compiled tool
now does both natively on Windows as well as Unix, and this work deletes the scripts so there is
one implementation to review and document. One capability exists only in the scripts: an optional
hardening mode for GitLab's top paid tier. It creates a custom reviewer role that can approve but
never push, and it registers the external status check that the automated review verdict is
posted to. The review tooling's own error message tells operators to use that script mode when the
status check is missing. The adopter who asked for the native Windows tool also asked that the
scripts remain the Unix reference.

Pick one:

1. **Port the paid-tier mode into the compiled tool first, then retire the scripts.** A small
   follow-on piece of work is written for the port. This retirement waits for it, and the error
   message then names the compiled tool. Pros: no adopter loses a capability, and there is one
   implementation. Cons: the retirement waits on one more piece of work.
2. **Retire now and make the paid-tier mode a documented manual step.** The runbook gives the two
   one-time API calls to run by hand, and the error message points at that section. Pros: the
   retirement lands now. Cons: a security hardening that was scripted and verified becomes a
   hand step, which is exactly how controls get skipped. It affects only top-tier adopters.
3. **Do not retire.** Keep the scripts as the Unix path, as the adopter asked, with the compiled
   tool as the Windows path and parity tests guarding drift. This work is closed without deleting
   anything. Pros: nothing is removed from anyone. Cons: two credential-minting implementations
   stay under review forever, and Windows guidance keeps a bash fallback.

Recommendation: **Option 1.** It is the only option that removes the bash dependency without
removing a verified security control.

Default if no answer: none — blocks until answered, because deleting an adopter-facing tool
needs a human's decision.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- **Row 1 is a human's live run, never an agent's.** An agent does not contact a GitLab instance,
  read-only included. If no instance is available, row 1 is `could-not-check`. Then the deletion
  commit is NOT made, and you report `BLOCKED-ON-HUMAN`.
- **The live proof's record carries CLASS outcomes only.** No hostname, URL, group path, project
  path, account name or token name appears in the PR, Evidence or any comment.
- **Do NOT delete anything before the ruling, the live proof and the pre-deletion parity row (row
  2) are all recorded.** Order is the control.
- **Do NOT rewrite records.** `CHANGELOG.md` and `docs/streams/**` keep the old names.
- Do not keep a script "just in case" or move it elsewhere in the tree. Under Option 1 or 2, retired
  means deleted.
- Stop at `implemented`. This is `gate: human`.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Don't guess.

## Task
1. **Obtain the ruling** on the decision issue. Under Option 1, stop and report NEEDS_CONTEXT until
   the port brief exists and is merged (add it to `depends:`). Under Option 3, close this brief
   as superseded and delete nothing.
2. **Live proof (human, row 1).** From a native Windows shell (PowerShell, not Git-Bash/WSL),
   against a disposable top-level group on a real GitLab instance: `deskfleet provision --dry-run`,
   then the real run with `--project`, then `deskfleet renew --dry-run`, then the real renewal, then
   one role's `desktoken --forge gitlab --no-rotate` read (`--no-rotate`, because the bare form
   rotates the PAT rather than reading it). Revoke the minted tokens afterwards. Record the
   class outcomes in the PR body under `## Live proof` BEFORE the deletion commit.
3. **Parity at the pre-deletion commit (row 2).** Record the commit SHA in the PR body.
4. **Delete** the five files. In the same commit, remove the `…MatchesBash` tests and their
   bash-source skip helper from `tables_parity_test.go` (planned).
5. **Rewrite every reference** row 6 finds. Each command the docs give becomes its `deskfleet`
   equivalent with the real flags. The windows-port/09 fallback line for fleet provisioning is
   removed, and the SessionStart-hooks Git-Bash prerequisite is kept. §2b becomes whatever the
   ruling makes it. Re-point `forge_gitlab.go`'s message per the ruling.
6. **Changelog fragment** (`### Removed`, one bullet per script naming its replacement).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | **LIVE proof, by a human in their own identity** (Task 2): provision dry-run then real, renew dry-run then real, one role's `desktoken --forge gitlab --no-rotate` read, all from native Windows PowerShell | recorded as classes only: dry-run enumeration equals the actions taken; 7/7 accounts created or present; 7/7 token files written with custody verdict owner-only; 9/9 labels; `main` protection, tags, approvals and merge checks read back as intended; 7/7 rotated (or each skip named by role); the `desktoken` read succeeded; OS class `windows/amd64` native; tier class free, premium or ultimate. **could-not-check ⇒ rows 4-9 do not run and nothing is deleted** | `gate:human +flow` |
| 2 | **Parity at the pre-deletion commit** (the SHA named in the PR body): `cd tools/desk && go test ./cmd/deskfleet/ -run 'MatchesBash' -count=1 -v > parity.out; s=$?; for t in TestRoleTableMatchesBash TestLabelTableMatchesBash TestPATDaysDefaultMatchesBash TestMainProtectionMatchesBash TestProtectedTagsMatchesBash TestApprovalsMatchesBash TestMergeChecksMatchesBash; do grep -q -- "--- PASS: $t (" parity.out \|\| exit 1; done; ! grep -q -- '--- SKIP' parity.out && test $s -eq 0` | exit 0; each of the seven named tests (roles, access levels/scopes, labels, PAT days, `main` protection, tags, approvals and merge checks) shows PASS, none skipped | `check +dereference` |
| 3 | **Fail-first: the guard that remains catches a missing role.** On the final head, delete one row (e.g. `board-writer`) from the Go role table, then run `cd tools/desk && go test ./cmd/deskfleet/ ./internal/... -run 'TestFleetTablesGolden' -count=1` | observed FAILING on the mutation (paste under `## Fail-first` in the PR body); exit 0 once the mutation is reverted | `check +mutation` |
| 4 | **The five files are gone:** `test ! -e tools/create-fleet-gitlab.sh && test ! -e tools/create-fleet-gitlab_test.sh && test ! -e tools/renew-fleet-gitlab-tokens.sh && test ! -e tools/renew-fleet-gitlab-tokens_test.sh && test ! -e tools/fleet-gitlab-roles.sh` | exit 0 | `check` |
| 5 | **No bash-comparing test is left to skip:** `git grep -n -e MatchesBash -e requireBashSources -- tools/desk; test $? -eq 1` | exit 0 (nothing found) | `check` |
| 6 | **Reference sweep, records excluded:** `git grep -n -e create-fleet-gitlab -e fleet-gitlab-roles -e renew-fleet-gitlab-tokens -- . ':!CHANGELOG.md' ':!changelog/' ':!docs/streams/'; test $? -eq 1` | exit 0 (nothing found). Positive control: `git grep -q -e create-fleet-gitlab -- docs/streams/windows-port/` exits 0 | `check` |
| 7 | **The documented commands exist as written:** build with `cd tools/desk && go build -o deskfleet.bin ./cmd/deskfleet`, run `./deskfleet.bin provision --help` and `./deskfleet.bin renew --help`, and compare with every `deskfleet` flag the edited runbook and skill sections name | every flag named in the docs appears in the help. A documented flag the verb lacks is a FAIL | `gate:model +dereference` |
| 8 | **Git-Bash survives only for the hooks:** `git grep -n -i -e git-bash -- docs/adopting-assay.md docs/adopting-assay-gitlab.md` | every remaining hit concerns the SessionStart hooks (or `bash`+`jq` for them). None concerns fleet provisioning or renewal. At least one hit exists | `gate:model +neighbour` |
| 9 | **Go and board stay green:** `cd tools/desk && go build ./... && go vet ./... && go test ./cmd/deskfleet/ ./internal/deskkit/ -count=1` | exit 0 | `check` |
| 10 | Consumers routing corroborated by the diff: `statusgen --root . --consumers windows-port/17` | exit 0 | `check` |
| 11 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended by a NON-implementer: one row per Verify item. Row 1 is transcribed from the
     PR body's `## Live proof` as class outcomes only; an agent records it could-not-check
     if it was not run, and never greens it from rows 2-11. gate: human. -->

## Review
Gate: **human** (from frontmatter — `customer: yes`, `sensitive-data: yes`). The reviewer
answers two questions. (1) What single control stands between "passes stubs" and "deleted the
only live-proven implementation"? It is row 1, and the Context names the layers behind it.
(2) Does a lower layer catch the fault with the upper one bypassed? Row 3 proves the remaining
guard reddens with the bash oracle gone. Row 6's positive control proves the sweep pattern
matches. Separately: was the paid-tier fork ruled before anything was deleted, and does §2b plus
the `forge_gitlab.go` message match the ruling?
