---
brief: assay:assay:desktools-v2:12
title: platform compatibility suite — Windows and GitLab semantics as pure-logic tests runnable on Linux/macOS
why: >-
  Briefs 01–11 specify and test every new contract (the native read client, token scoping,
  push guards, access-pattern operations, the outbound-write check) against GitHub shapes on
  POSIX only. The same classes of fault are already being hit from the field on Windows hosts
  and self-managed GitLab CE instances — path validation that rejects every non-POSIX path,
  custody checks that false-refuse on inherited temp-dir ACLs, provision runs that die on a CE
  tier gap, ambient-credential forms (GITLAB_TOKEN, CI_JOB_TOKEN, %APPDATA% configs) the
  negative tests never model. Almost all of it is testable as pure logic on a Linux runner:
  only a thin OS-specific reader ever needs a Windows machine. This brief builds that suite,
  and writes the rows that invoke it into the Verify tables of the briefs whose contracts it
  covers.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1836, 641, 642, 1604, 1621, 1573, 865, 1411, 1412, 1415, 1477]
schema: brief-v2
outcome: none
authored: 2026-09-29 by the desk, scoping trusted issue #1836
sources:
  - "#1836 — the full gap catalog this brief scopes: §1 cross-cutting findings, §2 per-brief gaps, §3 known issues, §4 the T1–T10 suite design, §5 acceptance"
  - "tools/desk/cmd/cellctl/cell.go:451-463 — rootsValid requires a leading '/'; the same HasPrefix shape sits at cell.go:317 (l), new.go:280 (launcher) and set.go:149 (v) — all four re-read 2026-09-30"
  - "tools/desk/cmd/cellctl/container.go:27 — the launcher is executed (exec.Command on CELL_CONTAINER_LAUNCHER), so the launcher check gates a binary cellctl runs; re-read 2026-09-30"
  - "tools/desk/cmd/deskfleet/project.go:266-294 — configureApprovals treats any non-200/201 from POST /projects/:id/approvals as fatal, and its Free-tier degrade warns 'do not count approvals as a server-enforced gate on this tier', re-read 2026-09-30; deskkit already degrades on the same CE 404 (hardening_read_approvals_ce_404.golden.json, reviews_at_head_ce_404_degrades.golden.json)"
  - "tools/desk/internal/deskkit/custodyacl.go:38 evaluateCustodyACL — already a build-tag-free pure model of the Windows custody decision, reached from ClassifyCustodyOwnerOnly (custodyverdict_windows.go:17) via classifyCustodyModel (custodyverdict.go:73), which records the invariant 'WITHOUT a second decision procedure'; TestEvaluateCustodyACL (custodyacl_test.go:16) and TestClassifyCustodyModel (custodyverdict_test.go:11) already table-test it on Linux — re-read 2026-09-30"
  - "tools/desk/internal/deskkit/selfcontain.go:117 reAbsMachinePath — the absolute-machine-path class recognises POSIX home and temp prefixes only; no drive-letter or UNC form, re-read 2026-09-30"
  - "docs/streams/desktools-v2/brief-04-deskclose-authorization-read-kind.md — the verified model to copy: GitLab goldens (close_issue_typed_*, list_comments_typed_*)"
  - "freshness-checked 2026-09-30 against current main — every file:line above re-read at that commit; other file:line references are quoted from #1836 and re-confirmed at implementation time, which the relevant Verify rows enforce"
exec-tier: strong
exec-tier-why: >-
  (a) the conformance harness is a design act — one shared case list driven over two backend
  fakes, where a case wired to one backend only is the exact failure mode this brief exists to
  catch; (b) the env-resolution contract encodes policy (which unset states refuse) that a weak
  port gets subtly wrong while every happy-path test stays green, and the custody work must
  EXTEND the one enforced decision procedure rather than grow a second one beside it;
  (c) correctness is cross-component — deskkit, cellctl, deskfleet, hookinstall and the Forge
  backends all move together.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit (NEW pathabs.go — planned; new cases in the existing custody tables): follow-up desktools-v2/12 (this brief)"
  - "tools/desk/internal/custodytest (NEW — planned; the PrivateTempDir test helper): follow-up desktools-v2/12 (this brief)"
  - "tools/desk/cmd/cellctl (NEW pathcheck.go — planned): follow-up desktools-v2/12 (this brief; the four POSIX-only path checks move onto cellPathCheck, which refuses every UNC and device spelling for roots and launcher)"
  - "tools/desk/cmd/deskfleet: follow-up desktools-v2/12 (this brief; CE tier-gap handling + PrivateTempDir in the custody-dependent tests)"
  - "tools/desk/internal/forge_gitlab + the conformance harness: follow-up desktools-v2/12 (this brief)"
  - "docs/streams/desktools-v2 briefs 03, 05, 08, 09, 10: follow-up desktools-v2/12 (this brief amends their Verify tables)"
  - "docs/streams/desktools-v2 brief 06 (row 4 re-target, Windows shim story, GitLab identity issues): follow-up desktools-v2/13 (the single owner of brief 06's re-derivation; this brief does not edit brief 06)"
  - "desktools-v2/13 (the CI legs, forge-ban symmetry and the lint half of #1836): out-of-scope (sibling brief; no dependency either way)"
version: 3
id: 968c76a1-05b5-4185-825c-c4464df1648e
---

# Brief 12 — platform compatibility suite

## Context

files:
- NEW `tools/desk/internal/deskkit/pathabs.go` (planned) + its test — `IsAbsFor`.
- `tools/desk/cmd/cellctl/cell.go`, `new.go`, `set.go` — the four POSIX-only path checks move onto one shared check; NEW `tools/desk/cmd/cellctl/pathcheck.go` (planned) — `cellPathCheck`, `IsAbsFor` plus the UNC/device refusal, with its test; NEW env-resolution test.
- `tools/desk/internal/deskkit/custodyacl_test.go`, `custodyverdict_test.go` — one new case each; the custody decision code is not edited.
- NEW `tools/desk/internal/custodytest/` (planned) — `PrivateTempDir` and its test; the custody-dependent `tools/desk/cmd/deskfleet` and `tools/desk/cmd/desktoken` tests move onto it.
- NEW conformance and ambient-decoy tests in `tools/desk/internal/deskkit/`.
- `tools/desk/cmd/deskpushguard/` — NEW hook-forwarding test (tests only).
- `tools/desk/cmd/deskfleet/project.go` — CE tier-gap degrade in `configureApprovals`, with its test.
- `docs/streams/desktools-v2/brief-03-*.md`, `brief-05-*.md`, `brief-08-*.md`, `brief-09-*.md`, `brief-10-*.md` — Verify-table amendments.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

Every desktools-v2 contract so far is verified against GitHub shapes on a POSIX shell. Issue
#1836 catalogs the result: a Windows-only compile break is first seen at release time; a
GitLab reach-around grows while the forge-ban counter stays green; Verify rows that hardcode
`/tmp` false-fail under a Windows witness instead of being marked POSIX-only; and the
ambient-credential negative tests model `GH_TOKEN` + `HOME` while the same threat arrives as
`GITLAB_TOKEN`, `CI_JOB_TOKEN`, a glab keyring, or an `%APPDATA%` config on a host where `HOME`
is normally unset.

The design principle (from #1836 §4): each Windows or GitLab assumption becomes **pure logic
testable on a Linux runner** — path semantics as string tables, environment resolution with an
injected env, the custody DACL as a data model, forge contracts as a two-backend conformance
table over `httptest` fakes. Only the thin OS-specific readers (the Win32 DACL read, the
`.cmd` hook) remain platform-bound, and those get string-level tests here plus the CI legs in
desktools-v2/13.

**The custody model already exists — this brief extends it, it does not add one.** #1836 §4 T4
proposed a new `[]ACE` type and a `CustodyVerdict(dacl, ownerSID)` function. That premise is
stale: `evaluateCustodyACL` (`internal/deskkit/custodyacl.go:38`) is already the pure,
build-tag-free model, `ClassifyCustodyOwnerOnly` on Windows reaches it through
`classifyCustodyModel`, and both carry Linux-run table tests that already cover owner-only
allow, the `Everyone` / `Authenticated Users` / `BUILTIN\Users` refusals, an inherited group
read refused and named as inherited, deny ACEs that never widen access, and SYSTEM and
Administrators as the pinned trusted writers (`rosterowner_windows.go`). `CustodyVerdict` is
also already an exported struct type in `deskkit`. A second decision function beside the
enforced one would let the tested model and the production model diverge — `custodyverdict.go`
records the invariant "WITHOUT a second decision procedure" — so deliverable 3 adds only the
cases the existing tables genuinely lack, plus the temp-dir helper the tests need.

## Deliverables

1. **T2 — path semantics as string tables.** One helper `deskkit.IsAbsFor(goos, p string) bool` (planned)
   covering POSIX, drive-letter (`C:\a`, `C:/a`), UNC and device forms, and the relative and
   drive-relative forms (`C:rel`, `\rooted`, `a/b`); table-tested for `linux`/`darwin`/`windows`
   over the input set in #1836 §4 T2, including `<owner>/<repo>=<path>` entries containing `:`
   and `\`. The input set also carries every UNC and device spelling, each with its expected
   value:
   - `\\srv\share\a` — `windows` true; `linux`/`darwin` false.
   - `//srv/share/a` — true on all three (on POSIX it is an ordinary absolute path).
   - `\\?\UNC\srv\share\a` — `windows` true; `linux`/`darwin` false.
   - `\\?\C:\a` — `windows` true; `linux`/`darwin` false.
   - `\\.\pipe\x` — `windows` true; `linux`/`darwin` false.

   The four POSIX-only checks in cellctl — `cmd/cellctl/cell.go:461` (rootsValid, for
   `CELL_ROOTS`), and `cell.go:317`, `new.go:280`, `set.go:149` (all three for
   `CELL_CONTAINER_LAUNCHER`) — move onto one shared helper, `cellPathCheck(goos, p string)
   error` (planned), in NEW `cmd/cellctl/pathcheck.go` (planned), which applies `IsAbsFor` plus the
   refusal below.

   **cellctl refuses every UNC and device spelling, for the stream roots and for the
   launcher.** `IsAbsFor` reports those paths absolute (that is their string semantics), so
   the refusal is cellctl's own. It is keyed on the shape every such spelling shares, not on
   one spelling: **a path whose first two characters are both separators, `/` or `\` in any
   mix**. That covers `\\srv\share\a`, `//srv/share/a`, `\/srv/share`, `\\?\UNC\srv\share\a`,
   `\\?\C:\a`, `\\.\pipe\x` and their forward-slash forms. A refusal keyed on a leading `\\`
   alone would not do: `//srv/share/a` already passes today's leading-`/` checks at all four
   sites (re-read 2026-09-30), and on Windows it is the same UNC path. The refusal applies on
   every host, not only Windows — POSIX leaves a leading `//` implementation-defined, and one
   cell.env can be read on several hosts. The error names `UNC or device path` and the reason:
   a root on a network share makes every desk tool authenticate to that host over SMB, and a
   launcher there is a binary cellctl executes (`container.go:27`) from a remote host. A POSIX
   absolute root and a Windows drive-letter root (`C:\a`, under `goos=windows`) stay accepted.
   `TestCellPathCheck` (planned) in `cmd/cellctl` pins this with two subtest groups, `roots`
   and `launcher`, and exactly these subtest names: `<group>/<c>_refused` for each `<c>` in
   `backslash_unc` (`\\srv\share\a`), `slash_unc` (`//srv/share/a`), `mixed_sep_unc`
   (`\/srv/share`), `extended_unc` (`\\?\UNC\srv\share\a`), `extended_drive` (`\\?\C:\a`) and
   `device_ns` (`\\.\pipe\x`), each asserting the `UNC or device path` error under both
   `goos=linux` and `goos=windows`; plus `roots/posix_abs_accepted`,
   `roots/drive_letter_accepted` and `launcher/posix_exec_accepted`.
   Owns the cellctl non-POSIX-path gap from #1836 §3-new-1.
2. **T3 — environment resolution with an injected env.** Using cellctl's existing `e.Get`
   abstraction, a table test `TestEnvResolution` (planned) covering config-home, gh/glab config and
   harness-home resolution under `goos=windows` semantics for: `HOME` unset with
   `USERPROFILE`/`APPDATA` set (#642); both set; neither set — which must REFUSE, never fall
   back to `/`. The neither-set case is the subtest `windows/neither_set_refuses`.
3. **T4 — custody: extend the existing model's tables; add the temp-dir helper.** No new ACE
   type, no new decision function, no production change to the custody check.
   - Add the one case both tables lack: a deny ACE for a foreign SID placed BEFORE an allow ACE
     granting that same SID read. Expected verdict: **REFUSE** — in `TestEvaluateCustodyACL` an
     error naming read-capable access, in `TestClassifyCustodyModel` `CustodyRefused`. Case name
     in both tables: `a deny before a foreign read allow still refuses`. This pins the rule at
     `custodyacl.go:60` ("a deny narrows access; it can never widen it") against an ordering
     reading where a preceding deny would cancel a later foreign allow.
   - The existing owner-equals-invoking-user refusal (`custodyacl.go:47`, case `a file owned by
     another user is refused`) stays in the tables unchanged; this brief must not remove or
     weaken any existing custody case.
   - Add `PrivateTempDir(t)` in a new shared test-helper package `internal/custodytest`, whose
     package doc states it is for `_test.go` files only (row 8 checks no non-test file imports
     it, so its permissive-elsewhere temp directory never reaches a shipped path): on
     Windows it creates a directory whose DACL is protected (inheritance stripped) and grants
     only the invoking user; elsewhere it is `t.TempDir()`. Its own test `TestPrivateTempDir` (planned)
     writes a 0600 file there and asserts the platform custody check reports it Verified. Move
     the custody-dependent deskfleet and desktoken tests onto it, so `TestFleetPartialRun` and
     `TestFleetProtectedBranchNeverUnprotected` stop depending on the host temp ACLs (#641,
     #1604, #1621). On a Linux runner this proves the helper and the wiring; the Windows
     behavior is proven by the `windows-latest` leg desktools-v2/13 deliverable 4 adds.
4. **T5 — two-backend conformance for the v2 contracts.** Extend the `forge_gitlab_golden`
   pattern into `TestContractConformance` (planned), subtests `{github,gitlab}/<case>`, in `internal/deskkit`: one
   shared case list, `httptest` fakes on both sides. Mandatory GitLab response shapes, each a
   named case with exactly this subtest name: `ce_404_approvals`, `ce_404_approval_rules`,
   `ce_404_push_rules` (CE 404 on the Premium endpoints); `free_tier_403`;
   `last_pipeline_empty` and `last_pipeline_absent` (#1411); `x_next_page` (pagination);
   `nested_subgroup` (URL-encoded `group%2Fsub%2Fproject`); `internal_visibility`;
   `files_api_400` (Files API 400 on update, #1412); `draft_change_400` (CreateDraftChange 400
   with its message, #1415); `mr_note_vs_issue_note` (#865). Every op the suite covers either
   has its GitLab case or an explicit `unsupported: <reason>` case row. Brief 04's goldens are
   the model.
5. **T6 — ambient-credential negative tests on both forges.** A new test
   `TestAmbientDecoyMatrix` (planned) in `internal/deskkit` with `github` and `gitlab` subtests (the name
   is deliberately distinct from the fourteen existing `TestAmbient*` tests, which already pass
   and prove nothing about this deliverable). Decoy values for `GH_TOKEN`, `GITHUB_TOKEN`,
   `GITLAB_TOKEN`, `CI_JOB_TOKEN`, `HOME`, `USERPROFILE`, `APPDATA` pointing at decoy config
   dirs; `gh`/`glab` stubs on `PATH` that fail the test if executed; assert the fake server saw
   only the minted token in `Authorization`/`PRIVATE-TOKEN` and never a decoy.
6. **T7 — hook-wrapper forwarding.** `TestPrePushCmdForwardsArgs` (planned) in `cmd/deskpushguard`
   generates the Windows hook pair via `writeHooks` and string-asserts that `pre-push.cmd`
   forwards `%*`, so the remote name (argv[1]) that brief 05's push-guard fix reads reaches
   `deskpushguard.exe`.
7. **T10 — tier-gap handling in deskfleet, keyed on 404 only.** Today `configureApprovals`
   (`cmd/deskfleet/project.go:266-271`) fails the run on any non-200/201 from
   `POST /projects/:id/approvals`, and the Free-tier path degrades only after a read-back shows
   the write was ignored, warning "do not count approvals as a server-enforced gate on this
   tier" (`project.go:278-294`). The new CE degrade is keyed on **404 from that endpoint only**:
   the CE shape for an endpoint the tier lacks, reached after the project itself has resolved.
   401, 403 and every 5xx still fail the run exactly as today. On an instance that supports
   approvals, those are a token-scope, permission or server fault; turning them into exit 0
   would leave the prevent-author and prevent-committer approval settings, a two-person
   control, unapplied while the run reports a tier gap. `TestConfigureApprovals` (planned) in
   `cmd/deskfleet` carries:
   - `gitlab_ce_404_degrades`: the fake GitLab server returns 404 on the POST. Assert a
     failed-at-tier NOTICE that carries the same "do not count approvals as a server-enforced
     gate on this tier" warning as the Free-tier path, a continued run, and exit 0 for that
     step — consistent with deskkit's existing CE-404 degrade goldens.
   - Negative controls `gitlab_401_still_fails`, `gitlab_403_still_fails` and
     `gitlab_500_still_fails`: the same fake returns that status. Assert the step fails the run
     (the existing `approval settings write failed` failure) and prints no tier-gap NOTICE.

   Owns the deskfleet CE-approvals-404 gap from #1836 §3-new-2.
8. **Verify-table amendments** — briefs 03, 05, 08, 09 and 10 each gain at least one GitLab row
   and one Windows-semantics row (#1836 §5), wired to the tests above. Each added row's Expect
   cell ends with the trace marker `(desktools-v2/12 GitLab row)` or `(desktools-v2/12 Windows
   row)`, so the rows are findable and row 10 below can check them. **Brief 06 is not amended
   here** — desktools-v2/13 deliverable 5 is the single owner of brief 06's re-derivation,
   including its GitLab and Windows rows.
   - **03** (todo): GitLab — run the custody negative tests as `{github,gitlab}` subtests.
     Windows — widen the decoy env matrix per T6 (adds `USERPROFILE`/`APPDATA`). Both rows name
     tests brief 03's implementer creates.
   - **05** (todo): GitLab — the GitLab URL shapes in both `…RewrittenToSSH` tests (self-managed
     host, non-22 SSH port, nested subgroup scp form, `oauth2` https user). Windows — the T7
     `TestPrePushCmdForwardsArgs` (planned) row (this brief creates that test).
   - **08** (todo): GitLab — a GitLab-bound scan repo (`ASSAY_REPO_FORGES=<repo>=gitlab`) case in
     `TestScanNeverEmptyWithoutForgeBinary` (planned) — brief 08 creates it. Windows — the PATH stub built with the `.exe` form
     under `GOOS=windows` (the `PATHEXT` case). Both name brief 08's own test.
   - **09** (already implemented): GitLab — the GitLab contract stated (bounded request count +
     consistency rule; CE needs REST + `X-Next-Page` where GitHub is one GraphQL round-trip),
     and the round-trip test run per backend with the CE-404 / Free-403 / empty-`last_pipeline`
     fixtures from T5. Windows — a row compiling the access-pattern test binary for Windows
     (`GOOS=windows GOARCH=amd64 go test -c`). Brief 09 is implemented, so THIS brief creates
     the per-backend test its new GitLab row names; existing rows and their Evidence are not
     edited.
   - **10** (todo): GitLab — fixtures for the real no-reply shapes
     (`<id>-<user>@users.noreply.gitlab.com`, and a self-managed host under `users.noreply.`),
     an `internal`-visibility target, and an MR note versus issue note write, with the
     conformance table run over both backend fakes. Windows — the absolute-machine-path class
     recognises drive-letter (`C:\Users\…`) and UNC (`\\host\share\…`) paths on a public target,
     using `IsAbsFor` from deliverable 1 (today `selfcontain.go:117` knows POSIX prefixes only).
     Both name tests brief 10's implementer creates. The Windows row is self-contained without
     a `depends` edge on brief 10: this brief adds it in the same PR that lands deliverable 1,
     so wherever the row exists `IsAbsFor` exists too. If brief 10 is already implemented when
     this brief lands, THIS brief creates the tests both new rows name, as for 09.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestIsAbsFor$' -v ./internal/deskkit/ > "$d/r1.out" 2>&1 && grep -E -e '^--- PASS: TestIsAbsFor \(' "$d/r1.out"` | prints the top-level `--- PASS: TestIsAbsFor` line (exit 0). The `&&` keeps `go test`'s status, so a failing test — even one where only some subtests fail — exits non-zero; the grep is anchored at column 0, so an indented subtest PASS line never satisfies it; a missing test (`no tests to run`) prints nothing and exits 1 |
| 2 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestEnvResolution$' -v ./cmd/cellctl/ > "$d/r2.out" 2>&1 && grep -E -e '^--- PASS: TestEnvResolution \(' "$d/r2.out" && grep -E -e '--- PASS: TestEnvResolution/windows/neither_set_refuses \(' "$d/r2.out"` | prints the top-level PASS line and the neither-set refusal subtest's PASS line (exit 0); a failing test exits non-zero at `go test` |
| 3 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestEvaluateCustodyACL$' -v ./internal/deskkit/ > "$d/r3a.out" 2>&1 && go test -run '^TestClassifyCustodyModel$' -v ./internal/deskkit/ > "$d/r3b.out" 2>&1 && grep -E -e '^--- PASS: TestEvaluateCustodyACL \(' "$d/r3a.out" && grep -E -e '^--- PASS: TestClassifyCustodyModel \(' "$d/r3b.out" && grep -E -e '--- PASS: TestEvaluateCustodyACL/a_deny_before_a_foreign_read_allow_still_refuses \(' "$d/r3a.out" && grep -E -e '--- PASS: TestClassifyCustodyModel/a_deny_before_a_foreign_read_allow_still_refuses \(' "$d/r3b.out" && grep -E -e '--- PASS: TestEvaluateCustodyACL/a_file_owned_by_another_user_is_refused \(' "$d/r3a.out"` | prints five PASS lines (exit 0): both whole tables pass, the new deny-ordering case REFUSES in both, and the owner-equals-invoking-user refusal is still pinned; exits 1 on today's tree (the deny-ordering case does not exist yet) |
| 4 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestContractConformance$' -v ./internal/deskkit/ > "$d/r4.out" 2>&1 && grep -q -E -e '^--- PASS: TestContractConformance \(' "$d/r4.out" && miss=0 && for c in ce_404_approvals ce_404_approval_rules ce_404_push_rules free_tier_403 last_pipeline_empty last_pipeline_absent x_next_page nested_subgroup internal_visibility files_api_400 draft_change_400 mr_note_vs_issue_note; do grep -q -E -e "--- PASS: TestContractConformance/gitlab/$c \(" "$d/r4.out" \|\| { echo "MISSING gitlab/$c"; miss=1; }; done && test $miss -eq 0` | exit 0 and nothing printed — `go test` passed, the top-level PASS line is present at column 0 (so no failing `github/...` or `gitlab/...` case is masked), and every mandatory GitLab shape from deliverable 4 is present by name and passing (the per-case match is indent-agnostic and ends at ` (`, so a nested name cannot stand in); a missing case prints `MISSING gitlab/<case>` and exits 1; a failing test exits non-zero before the loop |
| 5 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestAmbientDecoyMatrix$' -v ./internal/deskkit/ > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestAmbientDecoyMatrix \(' "$d/r5.out" && grep -E -e '--- PASS: TestAmbientDecoyMatrix/github \(' "$d/r5.out" && grep -E -e '--- PASS: TestAmbientDecoyMatrix/gitlab \(' "$d/r5.out"` | prints three PASS lines (exit 0): the whole test, then its `github` and `gitlab` subtests exactly (a nested `github/<x>` line does not match); the test itself fails if a `gh`/`glab` PATH stub ran or a decoy reached the fake server; exits 1 on today's tree, where no such test exists |
| 6 | check | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestPrePushCmdForwardsArgs$' -v ./cmd/deskpushguard/ > "$d/r6.out" 2>&1 && grep -E -e '^--- PASS: TestPrePushCmdForwardsArgs \(' "$d/r6.out"` | prints the top-level PASS line (exit 0) — the generated `pre-push.cmd` forwards `%*`; a failing test exits non-zero at `go test` |
| 7 | check | `d=$(mktemp -d) && cd tools/desk \|\| exit 1; miss=0; go test -run '^TestConfigureApprovals$' -v ./cmd/deskfleet/ > "$d/r7.out" 2>&1 \|\| { echo "FAIL go test"; miss=1; }; grep -q -E -e '^--- PASS: TestConfigureApprovals \(' "$d/r7.out" \|\| { echo "MISSING TestConfigureApprovals"; miss=1; }; for c in gitlab_ce_404_degrades gitlab_401_still_fails gitlab_403_still_fails gitlab_500_still_fails; do grep -q -E -e "--- PASS: TestConfigureApprovals/$c \(" "$d/r7.out" \|\| { echo "MISSING $c"; miss=1; }; done; f=$(grep -r -l -F -e 'func TestConfigureApprovals(' cmd/deskfleet --include='*_test.go'); { test -n "$f" && grep -q -F -e 'server-enforced gate on this tier' $f; } \|\| { echo "MISSING notice assertion"; miss=1; }; test $miss -eq 0` | exit 0 and nothing printed: the whole test passed, the CE-404 degrade and all three negative controls (401, 403, 500) are present by name and passing, and the test file asserts the "server-enforced gate on this tier" NOTICE. A degrade on any non-404 status fails its `_still_fails` control, so the row prints `FAIL go test` and exits 1; today it prints `MISSING` lines and exits 1 |
| 8 | check +flow | `d=$(mktemp -d) && cd tools/desk && go test -run '^TestPrivateTempDir$' -v ./internal/custodytest/ > "$d/r8.out" 2>&1 && grep -E -e '^--- PASS: TestPrivateTempDir \(' "$d/r8.out" && grep -r -l -F -e 'custodytest.PrivateTempDir(' cmd/deskfleet --include='*_test.go' && grep -r -l -F -e 'custodytest.PrivateTempDir(' cmd/desktoken --include='*_test.go' && { grep -r -l -F -e 'internal/custodytest"' . --include='*.go' --exclude='*_test.go'; test $? -eq 1; }` | prints the PASS line, then at least one `_test.go` file under `cmd/deskfleet/`, then at least one under `cmd/desktoken/` — one grep per directory, so either alone exits 1 — then nothing more, exit 0: no non-test `.go` file imports `internal/custodytest` (an importer is printed and the row exits 1). Exits non-zero on today's tree. On Linux this proves the helper and its wiring only — the Windows behavior is proven on desktools-v2/13's `windows-latest` leg |
| 9 | check +dereference | `grep -r -n -E -e 'HasPrefix\([a-zA-Z_]+, "/"\)' tools/desk/cmd/cellctl --include='*.go' --exclude='*_test.go'; test $? -eq 1` | exit 0 and nothing printed — no POSIX-only leading-slash check remains in cellctl under ANY variable name (today it prints four sites: `cell.go:317`, `cell.go:461`, `new.go:280`, `set.go:149`) |
| 10 | check | `miss=0; for b in 03 05 08 09 10; do f=$(ls docs/streams/desktools-v2/brief-$b-*.md); grep -q -F -e '(desktools-v2/12 GitLab row)' "$f" \|\| { echo "MISSING gitlab row: $b"; miss=1; }; grep -q -F -e '(desktools-v2/12 Windows row)' "$f" \|\| { echo "MISSING windows row: $b"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — each of the five briefs carries its GitLab and Windows-semantics rows per deliverable 8 (today it prints ten `MISSING` lines and exits 1) |
| 11 | check | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |
| 12 | check | `d=$(mktemp -d) && cd tools/desk \|\| exit 1; miss=0; go test -run '^TestCellPathCheck$' -v ./cmd/cellctl/ > "$d/r12.out" 2>&1 \|\| { echo "FAIL go test"; miss=1; }; grep -q -E -e '^--- PASS: TestCellPathCheck \(' "$d/r12.out" \|\| { echo "MISSING TestCellPathCheck"; miss=1; }; for t in $(for g in roots launcher; do for c in backslash_unc slash_unc mixed_sep_unc extended_unc extended_drive device_ns; do echo "$g/${c}_refused"; done; done) roots/posix_abs_accepted roots/drive_letter_accepted launcher/posix_exec_accepted; do grep -q -E -e "--- PASS: TestCellPathCheck/$t \(" "$d/r12.out" \|\| { echo "MISSING $t"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — every UNC and device spelling (`\\srv\share`, `//srv/share`, mixed separators, `\\?\UNC\`, `\\?\C:\`, `\\.\`) is refused both as a cell root and as `CELL_CONTAINER_LAUNCHER`, and a POSIX root, a drive-letter root and a POSIX launcher stay accepted. Today it prints `MISSING TestCellPathCheck` and fifteen `MISSING <group>/<case>` lines and exits 1 |
| 13 | check | `n=$(grep -c -F -e 'cellPathCheck(' tools/desk/cmd/cellctl/cell.go); m=$(grep -c -F -e 'cellPathCheck(' tools/desk/cmd/cellctl/new.go); k=$(grep -c -F -e 'cellPathCheck(' tools/desk/cmd/cellctl/set.go); echo "cell=$n new=$m set=$k"; test "$n" -ge 2 && test "$m" -ge 1 && test "$k" -ge 1` | prints the three counts and exits 0 — all four path sites (`rootsValid` and the launcher check in `cell.go`, the launcher checks in `new.go` and `set.go`) call the one shared check that row 12 tests, so no site keeps a private check that row 12 does not see. Today it prints `cell=0 new=0 set=0` and exits 1 |

## DoD

- All thirteen Verify rows pass on Linux (the suite's home). Rows 1–3 and 8 also pass on the
  `windows-latest` leg that desktools-v2/13 deliverable 4 adds, once that leg lands.
  **NOTE — when that Windows proof runs.** Today `.github/workflows/windows-ci-leg.yml`
  triggers only on a `v*` tag push and on `workflow_dispatch`, not on `pull_request`. Unless
  desktools-v2/13's staged workflow patch adds a `pull_request` trigger and a human applies
  it, the Windows pass for rows 1–3 and 8 is proven at release time (or by a manual dispatch),
  not on this brief's PR; the implementing PR says which. This is a gap in when the proof
  runs, not in what is proven.
- No production behavior change except deliverables 1 (cellctl accepts Windows drive-letter
  path shapes, and refuses every UNC and device spelling for both the stream roots and
  `CELL_CONTAINER_LAUNCHER` with a named reason — rows 9, 12, 13), 2 (cellctl resolves the
  operator home USERPROFILE-first on Windows and refuses when neither home variable is set,
  instead of deriving a relative or root-anchored config path — row 2), 7 (deskfleet degrades on
  a CE tier gap, keyed on a 404 from the approvals endpoint only, instead of dying — row 7) and
  the brief-10 Windows row of deliverable 8 (the public-target absolute-machine-path class also
  recognises drive-letter and UNC paths via `IsAbsFor` — brief 10's own Verify table),
  each covered by its own rows. Deliverable 3 changes tests and
  adds one test helper; the custody decision itself is untouched.
- Every test named by THIS brief's own Verify rows exists at merge and is the test the row
  invokes. Rows this brief adds to briefs 03, 05, 08 and 10 name tests those briefs' own
  implementers create (each amended brief's own Verify table is where that obligation is
  checked); the one amended brief already implemented (09) gets its per-backend test from this
  brief.
