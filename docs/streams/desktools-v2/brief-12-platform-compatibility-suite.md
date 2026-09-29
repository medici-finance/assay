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
authored: 2026-09-29 by the desk, scoping trusted issue #1836
sources:
  - "#1836 — the full gap catalog this brief scopes: §1 cross-cutting findings, §2 per-brief gaps, §3 known issues, §4 the T1–T10 suite design, §5 acceptance"
  - "tools/desk/cmd/cellctl/cell.go:451-463 — rootsValid requires a leading '/', re-read at authoring; the same HasPrefix shape sits at new.go:280, set.go:149, cell.go:317 per #1836 §3.1"
  - "tools/desk/cmd/deskfleet/project.go:266 — configureApprovals treats any non-200/201 from POST /projects/:id/approvals as fatal, re-read at authoring; deskkit already degrades on the same CE 404 (hardening_read_approvals_ce_404.golden.json, reviews_at_head_ce_404_degrades.golden.json)"
  - "tools/desk/cmd/cellctl/shims.go — present at authoring; the Go shim generator brief 06's stale row 4 should target instead of the retired shell oracle"
  - "docs/streams/desktools-v2/brief-04-deskclose-authorization-read-kind.md — the verified model to copy: GitLab goldens (close_issue_typed_*, list_comments_typed_*)"
  - "freshness-checked 2026-09-29 @ b89b39572 — the three file:line citations above re-read at that commit; every other file:line in this brief is quoted from #1836 and is re-confirmed at implementation time, which the relevant Verify rows enforce"
exec-tier: strong
exec-tier-why: >-
  (a) the conformance harness is a design act — one shared case list driven over two backend
  fakes, where a case wired to one backend only is the exact failure mode this brief exists to
  catch; (b) the custody-ACL model and the env-resolution contract encode policy (which SIDs
  are tolerated, which env unset states refuse) that a weak port gets subtly wrong while every
  happy-path test stays green; (c) correctness is cross-component — deskkit, cellctl, deskfleet,
  hookinstall and the Forge backends all move together.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit (NEW pathabs.go, custodymodel.go — planned): follow-up desktools-v2/12 (this brief)"
  - "tools/desk/cmd/cellctl: follow-up desktools-v2/12 (this brief; the four POSIX-only path checks move onto IsAbsFor)"
  - "tools/desk/cmd/deskfleet: follow-up desktools-v2/12 (this brief; CE tier-gap handling + privateTempDir in the custody tests)"
  - "tools/desk/internal/forge_gitlab + the conformance harness: follow-up desktools-v2/12 (this brief)"
  - "docs/streams/desktools-v2 briefs 03, 05, 06, 08, 09, 10: follow-up desktools-v2/12 (this brief amends their Verify tables)"
  - "desktools-v2/13 (the CI legs, forge-ban symmetry and the lint half of #1836): sibling, no dependency either way"
version: 1
id: 968c76a1-05b5-4185-825c-c4464df1648e
---

# Brief 12 — platform compatibility suite

## Context

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

## Deliverables

1. **T2 — path semantics as string tables.** One helper `deskkit.IsAbsFor(goos, p string) bool`
   covering POSIX, drive-letter (`C:\a`, `C:/a`), UNC (`\\srv\share\a`), and the relative and
   drive-relative forms; table-tested for `linux`/`darwin`/`windows` over the input set in
   #1836 §4 T2, including `<owner>/<repo>=<path>` entries containing `:` and `\`. The four
   POSIX-only checks in cellctl (`cmd/cellctl/cell.go:451-463` rootsValid, `new.go:280`,
   `set.go:149`, `cell.go:317`) move onto it. Owns the cellctl non-POSIX-path gap from #1836
   §3-new-1.
2. **T3 — environment resolution with an injected env.** Using cellctl's existing `e.Get`
   abstraction, test config-home, gh/glab config and harness-home resolution under
   `goos=windows` semantics for: `HOME` unset with `USERPROFILE`/`APPDATA` set (#642); both
   set; neither set — which must REFUSE, never fall back to `/`.
3. **T4 — custody ACL as a pure model.** Model a DACL as `[]ACE{SID, mask, inherited, deny}`
   and make `CustodyVerdict(dacl, ownerSID)` a pure function; table-test it on Linux over the
   cases in #1836 §4 T4 (owner-only allow; inherited `BUILTIN\Users` read refuse;
   `Authenticated Users`/`Everyone` refuse; deny-before-allow ordering; SYSTEM and
   Administrators per a pinned policy). Only the Win32 DACL reader stays behind
   `//go:build windows`. Add `privateTempDir(t)` (strips inheritance on Windows, plain
   `t.TempDir()` elsewhere) and move the custody-dependent deskfleet/desktoken tests onto it,
   so `TestFleetPartialRun` and `TestFleetProtectedBranchNeverUnprotected` stop depending on
   the host temp ACLs (#641, #1604, #1621).
4. **T5 — two-backend conformance for the v2 contracts.** Extend the `forge_gitlab_golden`
   pattern into `TestContractConformance/{github,gitlab}/<case>`: one shared case list,
   `httptest` fakes on both sides. Mandatory GitLab response shapes, each a named case: CE 404
   on Premium endpoints (approvals, approval rules, push rules); Free-tier 403; empty or
   absent `last_pipeline` (#1411); `X-Next-Page` pagination; URL-encoded nested subgroup paths
   (`group%2Fsub%2Fproject`); `internal` visibility; Files API 400 on update (#1412);
   CreateDraftChange 400 with its message (#1415); MR note versus issue note (#865). Every op
   the suite covers either has its GitLab case or an explicit `unsupported: <reason>` case
   row. Brief 04's goldens are the model.
5. **T6 — ambient-credential negative tests on both forges.** Decoy values for `GH_TOKEN`,
   `GITHUB_TOKEN`, `GITLAB_TOKEN`, `CI_JOB_TOKEN`, `HOME`, `USERPROFILE`, `APPDATA` pointing
   at decoy config dirs; `gh`/`glab` stubs on `PATH` that fail the test if executed; assert
   the fake server saw only the minted token in `Authorization`/`PRIVATE-TOKEN` and never a
   decoy.
6. **T7 — hook-wrapper forwarding.** Generate the Windows hook pair via `writeHooks` and
   string-assert that `pre-push.cmd` forwards `%*`, so the remote name (argv[1]) that brief
   05's fix depends on reaches `deskpushguard.exe`.
7. **T10 — tier-gap handling in deskfleet.** The fake GitLab server returns 404 on
   `POST /projects/:id/approvals`; assert a failed-at-tier NOTICE, a continued run, and exit 0
   for that step — consistent with deskkit's existing CE-404 degrade goldens. Owns the
   deskfleet CE-approvals-404 gap from #1836 §3-new-2.
8. **Verify-table amendments** — briefs 03, 05, 06, 08, 09 and 10 each gain the rows #1836 §2
   names for them, wired to the tests above:
   - **03**: run the custody negative tests as `{github,gitlab}` subtests; widen the decoy env
     matrix per T6 (adds `USERPROFILE`/`APPDATA`, `GITLAB_TOKEN`, `CI_JOB_TOKEN`).
   - **05**: GitLab URL shapes in both `…RewrittenToSSH` tests (self-managed host, non-22 SSH
     port, nested subgroup scp form, `oauth2` https user) plus the T7 `%*` forwarding row.
   - **06**: row 4 re-targeted from the retired shell oracle to the Go shim generator in
     `cmd/cellctl` (`shims.go` resolves `CELLCTL_GH_AMBIENT` before the HOME swap); the
     Windows shim story stated, or marked out of scope with an owner.
   - **08**: a GitLab-bound scan repo (`ASSAY_REPO_FORGES=<repo>=gitlab`) case in
     `TestScanNeverEmptyWithoutForgeBinary`; the PATH stub built with the `.exe` form under
     `GOOS=windows`.
   - **09**: the GitLab contract stated (bounded request count + consistency rule; CE needs
     REST + `X-Next-Page` where GitHub is one GraphQL round-trip); the round-trip test run per
     backend with the CE-404 / Free-403 / empty-`last_pipeline` fixtures from T5.
   - **10**: GitLab fixtures — the real no-reply shapes (`<id>-<user>@users.noreply.gitlab.com`,
     and a self-managed host under `users.noreply.`), an `internal`-visibility target, an MR
     note versus issue note write; the conformance table run over both backend fakes.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test -run TestIsAbsFor ./internal/deskkit/ -v` | output contains `--- PASS: TestIsAbsFor` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 2 | `cd tools/desk && go test -run 'TestEnvResolution' ./cmd/cellctl/ -v` | output contains `--- PASS`, including the neither-set refusal case |
| 3 | `cd tools/desk && go test -run 'TestCustodyVerdict' ./internal/deskkit/ -v` | output contains `--- PASS`, including the inherited-read refuse and deny-before-allow cases |
| 4 | `cd tools/desk && go test -run 'TestContractConformance' ./... -v 2>&1 | grep -c '^    --- PASS: TestContractConformance/gitlab/'` | prints a count ≥ 9 (every mandatory GitLab shape from deliverable 4 present and passing) |
| 5 | `cd tools/desk && go test -run 'TestAmbient' ./... -v` | output contains `--- PASS` for the decoy-matrix tests on both forges; the PATH stubs report never having run |
| 6 | `cd tools/desk && go test -run 'TestPrePushCmdForwards' ./... -v` | output contains `--- PASS` (the generated `pre-push.cmd` forwards `%*`) |
| 7 | `cd tools/desk && go test -run 'TestConfigureApprovals' ./cmd/deskfleet/ -v` | output contains `--- PASS`, including the CE-404 case: NOTICE printed, run continues, step exits 0 |
| 8 | `cd tools/desk && go test -run 'TestFleetPartialRun|TestFleetProtectedBranchNeverUnprotected' ./cmd/deskfleet/ -v` | both PASS on the pure-model custody path via `privateTempDir` |
| 9 | `grep -rn 'HasPrefix(path, "/")' tools/desk/cmd/cellctl --include='*.go' | grep -v _test.go; test $? -eq 1` | exit 0 — no POSIX-only absoluteness check remains in cellctl (the four named sites use `IsAbsFor`) |
| 10 | `for b in 03 05 06 08 09 10; do f=$(ls docs/streams/desktools-v2/brief-$b-*.md); grep -qi gitlab "$f" || echo "MISSING gitlab row: $b"; grep -qi windows "$f" || echo "MISSING windows row: $b"; done` | prints nothing — each of the six briefs carries its GitLab and Windows-semantics rows per deliverable 8 |

## DoD

- All ten Verify rows pass on Linux (the suite's home) and rows 1–3, 8 pass under
  `GOOS=windows go test` where the package compiles there.
- No production behavior change except deliverables 1 (cellctl accepts Windows path shapes),
  3 (custody verdict identical on POSIX, model-driven on Windows) and 7 (deskfleet degrades on
  a CE tier gap instead of dying) — each covered by its own rows above.
- Every test the six amended Verify tables name exists and is the test those rows invoke.
