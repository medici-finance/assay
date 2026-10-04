---
brief: "assay:assay:desk-supervision:22"
title: "Configure provider, model and effort per cell role"
why: "Floating aliases and inherited effort settings change desk behavior without a deliberate cell configuration change. Resolve explicit provider/model/effort assignments at the launch boundary and prohibit Opus 5."
wave: 0
depends: []
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
gate-why: "An opt-in launcher configuration change with offline negative-path tests; no merge or review authority changes."
issues: []
schema: "brief-v2"
authored: "2026-09-20 by model policy implementation session"
sources: ["docs/cellctl.md", "docs/cellctl-model-policy.md", "direct operator request 2026-09-20"]
consumers: ["tools/cellctl/cellctl: fixed-here", "docs/cellctl.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Must keep model aliases, provider credentials, effort and child defaults coherent across two harnesses."
domain: "complicated"
version: 1
id: "9e8796ad-37d2-421b-9a33-4cac0d2808ad"
---

# Brief 22 — Configure provider, model and effort per cell role

## Context

Home: medici-finance/assay. Implement the operator-authorized model mapping independently
of review-rule changes and scheduler work. The existing `cellctl` is the launch boundary;
do not make another scheduler or change a running desk from a prompt.

files:
- tools/cellctl/cellctl
- tools/cellctl/examples/model-policy.json
- tools/cellctl/tests/model-policy.test.py
- tools/cellctl/tests/scrubbed-cell.test.sh
- docs/cellctl.md
- docs/cellctl-model-policy.md
- changelog/cellctl-model-policy.md

facts:
- Legacy model selection is per harness but provider selection is cell-wide or per invocation.
- A floating model alias can advance versions independently of the launcher.
- Claude and Codex have different effort and child-default configuration surfaces.
- Existing provider tier work in PR 1353 overlaps alias environment construction; preserve legacy behavior and coordinate the policy arm when merging that work.

## Read first

[Cell model policy](../../cellctl-model-policy.md), the existing launcher, provider/harness
regression suites, and the official harness references linked in the policy document.

## Interface contract

`CELL_MODEL_POLICY` points to an operator-owned schema-1 JSON file. Explicit role assignments
select provider and tier; each provider tier pins a model ID, effort and supported levels.
Legacy behavior remains when absent. Policy mode refuses unknown/unmapped/denied requests,
unsupported effort, conflicting local allowlists and unsupported launch paths. Prohibit
Opus 5, including explicit child requests; map the example Opus tier to 4.8. Show/check/desk/up
must agree on the selected tuple. The launch record includes a policy hash, never credentials.

Propagate Claude aliases, child default and effort; pass Codex model/effort/child defaults
through CLI overrides. Preserve permission envelopes and credential custody. Codex explicit
child overrides and administrator-managed settings are documented limits, not claimed controls.

Single-point-of-failure: the harness applies local model restrictions — launcher validation
and explicit-request hooks sit above it; provider-side managed restrictions remain operator-owned.
This is configuration correctness, not containment of an agent that can execute a shell.

## Task

1. Implement the opt-in policy in standalone cellctl; keep its single-file release packaging.
2. Refuse invalid whole-cell plans before opening role windows. Preserve legacy shell suites.
3. Add offline request, mapping, effort, settings-conflict, credential and actual exec-argv tests.
4. Update the named operator docs and example; no site generator is involved in these pages.
5. Release/install through the existing procedure. Enable the policy and inspect a parent and
   child transcript before broadening rollout; source completion alone is not live adoption.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `python3 tools/cellctl/tests/model-policy.test.py` | named tests pass, including mixed-provider show/up/desk agreement and recording-harness exec arguments |
| 2 | check:ci | `bash tools/cellctl/tests/provider.test.sh` | exit 0; existing provider credential and override behavior preserved without policy |
| 3 | check:ci | `bash tools/cellctl/tests/harness.test.sh` | exit 0; existing Claude/Codex launch behavior preserved without policy |
| 4 | check:ci | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | all three suites exit 0; no legacy pin, override or persistence regression |
| 5 | check:ci | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 |

Pre-mortem → detection: aliases drift to a prohibited model: direct/child/allowlist negative
cases in row 1. Wrong credential adapter: recorded mixed-provider exec in row 1. Effort
silently defaults: unsupported-effort and exact argument checks in row 1. These are offline
configuration checks; actual provider inference remains a rollout check, never a fabricated PASS.

## Evidence

Implementation-session local tests recorded in the draft PR. Independent verification pending.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 tools/cellctl/tests/model-policy.test.py` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/provider.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/harness.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash -n tools/cellctl/cellctl` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |

Verifier notes (2026-09-27, assay-verifier-app[bot], non-implementer, merged main 874d56de38a7; implementing commit ef102f7f7, PR #1388). The witness above is could-not-run for every row because check:ci rows need the Linux network-off sandbox and this host is darwin. The same five commands were also run by hand on the host, in a scrubbed environment (env -i keeping only HOME, PATH, TMPDIR, USER, LANG; KUBECONFIG=/dev/null):

- Row 1: exit 0. "Ran 17 tests ... OK". The named tests include test_desk_show_and_up_agree_on_mixed_providers, test_real_exec_argv_and_env, test_child_mapping_allowlist_and_hooks, test_direct_and_alias_requests, test_ban_cannot_be_removed_by_omitting_deny, test_unsupported_efforts and test_competing_allowlist_refuses_before_launch.
- Row 2: exit 0. 76 ok lines, "provider.test.sh: OK". In the unscrubbed parent agent-session shell the same suite exits 1 with 2 FAILED. The failing checks are "no provider: ANTHROPIC_BASE_URL is NOT exported" and "ANTHROPIC_API_KEY is left alone and no model vars are exported". The recorded launch env showed the caller's own ANTHROPIC_BASE_URL and ANTHROPIC_MODEL values passed through unchanged, because the suite's baseline arm does not unset ambient ANTHROPIC_* variables. That is a test-hermeticity gap from before this brief (both assertions come from earlier provider work), not a regression from it. CI runners do not carry those variables.
- Row 3: exit 0. "harness.test.sh: OK".
- Row 4: exit 0. model-namespace, model-override and cell-set each print OK.
- Row 5: exit 0 with no output.

Risk-bearing value enumeration (diff scope: tools/cellctl/cellctl policy arm, tools/cellctl/examples/model-policy.json, plus the brief Deliverables). Risk metadata is present with every field "no", so no trigger fires. The enumeration is recorded anyway:

1. built-in ban = ["*opus-5*", "*opus5*"] @ tools/cellctl/cellctl:740. This is the brief's only hard prohibition. Reversible.
2. the-desk rule = refuse any model containing "opus" @ tools/cellctl/cellctl:810. Reversible.
3. example deny = "*opus-5" @ tools/cellctl/examples/model-policy.json:4. Commit 647202b63 changed it from "*opus-5*". Reversible.
4. example strong tier = "claude-opus-4-8[1m]" @ tools/cellctl/examples/model-policy.json:44. Reversible.
5. alias map = {fable: top, opus: strong, sonnet: mid, haiku: fast} @ tools/cellctl/cellctl:742. Reversible.
6. effort level sets = claude {low, medium, high, xhigh, max} and codex {minimal, low, medium, high, xhigh} @ tools/cellctl/cellctl:769. The GLM 5.3 and Kimi K3 restriction {low, high, max} is @ tools/cellctl/cellctl:773. Reversible.
7. anthropic base URL = "https://api.anthropic.com" @ tools/cellctl/cellctl:835. Set in policy mode only. Reversible.

Every entry is an operator configuration value. An edit and a new release undo any of them.

RISK-VALUE: DERIVED. banned += ["*opus-5*", "*opus5*"] @ tools/cellctl/cellctl:740. The brief says "Prohibit Opus 5, including explicit child requests". The patterns are matched with fnmatchcase against the lowercased ID with the [1m] suffix removed, so they catch claude-opus-5, its [1m] form, its -5-0 and -5.0 forms, and the compact opus5 spelling. The resolver checks them for tier maps, for direct and alias requests, and for PreModelSwitch and child requests. Row 1's test_ban_cannot_be_removed_by_omitting_deny covers the omitted-deny case.

RISK-VALUE: NAMED, NOT DERIVED. the-desk non-Opus rule @ tools/cellctl/cellctl:810, and the same ban's trailing wildcard at :740. The brief text does not state either rule. Current main's model policy doc says the prohibition ends at the end of the token, so Opus 5.5 (claude-opus-5-5) is allowed and the-desk has a version floor at Opus 5.5. That describes the Go launcher after 647202b63. The bash launcher still matches claude-opus-5-5 against "*opus-5*" and still refuses every Opus model for the-desk. Open question for the desk: are the bash launcher and the doc meant to agree? This does not fail any Verify row.

Observations:
- Task 5 (live release and a parent/child transcript inspection) is a rollout check. It is not in the Verify table, and this offline pass did not perform it.
- The changelog fragment named in the Context file list was folded into CHANGELOG.md by the v1.0.16 changelog roll-up. That is the expected lifecycle, not a missing deliverable.

VERIFY: PASS. All five rows exited 0 on the host in a scrubbed environment. The statusgen witness is could-not-run on darwin, so it needs a Linux-runner witness if the flip requires a machine witness.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 tools/cellctl/tests/model-policy.test.py` | fail exit=1 | sha256:4c715e5f784d | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/provider.test.sh` | pass exit=0 | sha256:fa9017b35232 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/harness.test.sh` | fail exit=1 | sha256:edf9abd91704 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | fail exit=1 | sha256:cef9c18e8b1d | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `python3 tools/cellctl/tests/model-policy.test.py` | named tests pass, incl. mixed-provider show/up/desk agreement and recording-harness exec arguments | exit 1. "Ran 17 tests ... FAILED (failures=1)". 16 ok; test_real_exec_argv_and_env FAIL with "cellctl: desk: fetch of origin main in (fixture repo) failed and wrote no FETCH_HEAD — refusing to boot on a stale main" (launcher exit 3). Same result at the parent of 3be9befd7 is OK (17/17); at 3be9befd7 (#1853) it is FAILED (1), so #1853 broke the fixture. With the fixture's fetch stub changed to write FETCH_HEAD (throwaway clone), 17/17 OK incl. test_real_exec_argv_and_env and test_desk_show_and_up_agree_on_mixed_providers | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `bash tools/cellctl/tests/provider.test.sh` | exit 0; provider credential and override behavior preserved without policy | exit 0. "provider.test.sh: OK" | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `bash tools/cellctl/tests/harness.test.sh` | exit 0; Claude/Codex launch behavior preserved without policy | exit 0. "harness.test.sh: OK" (darwin host has tmux on PATH) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | all three suites exit 0 | exit 0. "model-namespace.test.sh: OK", "model-override.test.sh: OK", "cell-set.test.sh: OK" (darwin host has tmux and herdr on PATH) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 | exit 0, no output | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — builtin ban (oracle) banned += ["*opus-5*", "*opus5*"] @ tools/cellctl/testdata/cellctl-shell-oracle.sh:755 — the brief says "Prohibit Opus 5, including explicit child requests"; fnmatch on the lowercased, [1m]-stripped ID with a trailing wildcard catches every Opus 5.x spelling, and row 1's test_ban_cannot_be_removed_by_omitting_deny passes on both hosts. It over-covers (also refuses Opus 5.5), which is the safe direction for a prohibition.
RISK-VALUE: NAMED, NOT DERIVED — builtin ban (shipped Go launcher) m.Banned += "*opus-5", "*opus5", "*opus-5-0", "*opus-5.0" @ tools/desk/cmd/cellctl/policy.go:252 — this set is NOT exercised by any Verify row (all rows run the bash oracle), and its completeness against every Opus 5.0 ID form was not derived; question #1937.
RISK-VALUE: NAMED, NOT DERIVED — example strong tier "model": "claude-opus-4-8[1m]" @ tools/cellctl/examples/model-policy.json:44 and example deny "*opus-5" @ tools/cellctl/examples/model-policy.json:4 — matches the brief's "map the example Opus tier to 4.8"; operator-editable example, reversible, no further derivation attempted; question #1938.

Notes:
- BLOCKED. Hand rows 2-5 pass; row 1 fails on merged main because the suite's fetch stub writes no FETCH_HEAD, which the launcher refuses since #1853 (the parent of that commit passes 17/17; with a stub that writes FETCH_HEAD, 17/17 pass): a test-fixture regression, bug #1936. The Linux witness passes rows 2 and 5; rows 3 and 4 fail there only because the suites need host tmux (and herdr), a hermeticity gap in the rows as authored, tracked with the other row re-authors at #1927. `statusgen brief --check-verified` with a hypothetical flip exits 1 against main's Evidence and with this witness appended. Two RISK-VALUE lines are NAMED, NOT DERIVED (questions #1937 and #1938); the Verify table covers only the bash oracle, not the shipped Go launcher.
- Grounded expectation was written before reading tests: opt-in CELL_MODEL_POLICY, example mapping Opus to 4.8, offline policy suite, legacy suites unchanged, docs updated. Main drift since authoring: the bash launcher moved to tools/cellctl/testdata/cellctl-shell-oracle.sh (#1739) and the shipped launcher is now the Go program tools/desk/cmd/cellctl, which carries its own policy implementation (policy.go, policy_enforce.go) and Go tests that port the oracle cases.
- Row 1 is a check-definition failure: it fails as authored on darwin and Linux because #1853 (3be9befd7, "refuse to boot on a stale FETCH_HEAD after a failed fetch") removes FETCH_HEAD before the fetch, and the suite's fetch stub exits 0 without writing it. Bisected: parent of 3be9befd7 is 17/17 OK, 3be9befd7 is FAILED (1). The policy substance passes when the stub writes FETCH_HEAD (17/17 OK). No CI workflow runs tools/cellctl/tests, which is why the regression landed unnoticed. Fix belongs in the test fixture (the stub fetch should write FETCH_HEAD), not in the launcher.
- Rows 3 and 4 are hermeticity gaps in the check definition: the suites depend on host tmux (and herdr for cell-set) being on PATH. They pass on darwin because this host has both installed, and pass on Linux with no-op stubs. As authored they fail on a clean Linux runner, so the check:ci witness cannot go green without either provisioning those tools in the runner or stubbing them inside the suites.
- Main's Evidence row 5 records the pre-#1739 command; check-verified correctly treats it as stale.
- The Verify table proves the bash oracle only. The shipped Go launcher diverges from the oracle on the Opus ban pattern set and on the-desk (version floor at Opus 5.5 vs refusing every Opus). The docs describe the Go behaviour. No Verify row covers the Go policy arm; worth a follow-up row (for example the Go package's policy tests) so the verified claim covers what ships.
- Task 5 (release, enable, inspect a parent and child transcript) is a rollout check outside the Verify table; this offline pass did not perform it and makes no claim about live adoption.
- No credentials were minted or used; no forge writes; claim not released.

VERIFY: BLOCKED

### Non-implementer verifier run: 2026-10-02T22:39:33Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Hand run on a darwin host in a scrubbed environment (env -i keeping HOME, PATH, USER, LANG; KUBECONFIG=/dev/null; TMPDIR redirected to a scratch directory outside the checkout). Gate: model; all four risk answers no. Every row was executed exactly as authored.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `python3 tools/cellctl/tests/model-policy.test.py` | named tests pass, including mixed-provider show/up/desk agreement and recording-harness exec arguments | exit 0. "Ran 19 tests ... OK". With -v, 19 of 19 print "ok", including test_desk_show_and_up_agree_on_mixed_providers, test_real_exec_argv_and_env, test_ban_cannot_be_removed_by_omitting_deny, test_child_mapping_allowlist_and_hooks, test_unsupported_efforts and test_competing_allowlist_refuses_before_launch | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `bash tools/cellctl/tests/provider.test.sh` | exit 0; existing provider credential and override behavior preserved without policy | exit 0. "provider.test.sh: OK" (76 ok lines, no FAIL) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `bash tools/cellctl/tests/harness.test.sh` | exit 0; existing Claude/Codex launch behavior preserved without policy | exit 0. "harness.test.sh: OK" (host has tmux on PATH; with tmux removed from PATH the same suite exits 1, "harness.test.sh: 6 FAILED") | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | all three suites exit 0; no legacy pin, override or persistence regression | exit 0. "model-namespace.test.sh: OK", "model-override.test.sh: OK", "cell-set.test.sh: OK" (host has tmux and herdr on PATH; with both removed from PATH the chain exits 1 at the first suite, "model-namespace.test.sh: 2 FAILED") | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 | exit 0, no output | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief ... --dry-run`, statusgen v1.0.31, throwaway HOME, exit 2), per row:

- Row 1: could not be executed on this host. check:ci rows need the network-off sandbox (`unshare --net`, a Linux facility); this host is darwin.
- Row 2: could not be executed on this host, same reason.
- Row 3: could not be executed on this host, same reason.
- Row 4: could not be executed on this host, same reason.
- Row 5: could not be executed on this host, same reason.

Witness result: 0 of 5 rows executed. The dry run wrote nothing to the brief and left no untracked file in the worktree.

Findings:

- By hand, 5 of 5 rows pass as authored on this host. That is a change from the 2026-09-30 run, where row 1 failed.
- Row 1 is fixed on main. Commit 20864edfd (#1944) made the suite's fetch stub write FETCH_HEAD and added two tests (test_fetch_stub_writes_fetch_head, test_no_fetch_stub_exits_without_fetch_head), so the suite is now 19 tests, up from 17. Bug #1936 is still open although its fix is merged; it looks closable, which is the desk's call.
- Rows 3 and 4 still pass only because the host has tmux and herdr installed. Re-run with a PATH that has neither, row 3 exits 1 (6 FAILED) and row 4 exits 1 (model-namespace 2 FAILED); row 1 still passes 19/19 on that PATH. So a clean Linux runner would still fail rows 3 and 4, as the 2026-09-30 Linux witness recorded. This is tracked as an open checklist item on #1927 (open).
- No row passes vacuously. Row 1's named tests were confirmed individually with -v. Rows 2 to 4 print per-assertion ok/FAIL lines and no skips. Row 5 is a syntax check of the bash oracle only; the file reached its current path in b227b4076 (#1739), a rename of the launcher added by the implementing commit ef102f7f7, so the row proves the oracle parses and nothing more.
- The Verify table still covers only the bash oracle. The shipped Go launcher has its own policy arm and no row runs it (open question #1937; open checklist item on #1927).
- No workflow under .github/workflows references tools/cellctl/tests, so none of these suites run in CI.
- The changelog fragment named in the Context file list is absent on main; the 2026-09-27 notes record it as folded into CHANGELOG.md by a release roll-up.
- Task 5 (release, enable, inspect a parent and child transcript) is a rollout check outside the Verify table; this offline pass did not perform it.
- Blocker #1800 (darwin host has no network-off sandbox) is open and unchanged. #1491 (in-container witness defects) is open.

Inputs changed since the 2026-09-27 outcome record (sha 874d56de38a7): every declared input differs. All eleven file inputs have a different sha256 (the brief, docs/cellctl-model-policy.md, docs/cellctl.md, the example model-policy.json, and the seven suites cell-set, harness, model-namespace, model-override, model-policy, provider, scrubbed-cell), and the tool moved from v1.0.27 to v1.0.31. Changes on those paths since that sha: #1739 (oracle moved to testdata), #1760, #1762, #1631/#1650, #1853, #1944, #1945, #1981, #2007, plus the two Evidence landings on the brief (#1783, #1950).

Risk-bearing values. Risk metadata is present with every field "no" and irreversible "no"; no trigger fires. Enumeration re-checked at this sha (scope: bash oracle policy arm, example policy file, Go launcher policy arm):

1. oracle built-in ban: banned + ["*opus-5*", "*opus5*"] @ tools/cellctl/testdata/cellctl-shell-oracle.sh:757. Reversible by an edit and a release.
2. example deny: "*opus-5" @ tools/cellctl/examples/model-policy.json:4. Operator-editable example. Reversible.
3. example strong tier: "claude-opus-4-8[1m]" @ tools/cellctl/examples/model-policy.json:44. Reversible.
4. Go alias map: {fable: top, opus: strong, sonnet: mid, haiku: fast} @ tools/desk/cmd/cellctl/policy.go:32. Reversible.
5. Go built-in ban: since #1945 this is no longer a pattern list. It is the version match in isBannedOpus50 @ tools/desk/cmd/cellctl/model.go:61, applied to every value. Reversible.

RISK-VALUE: DERIVED — oracle ban banned + ["*opus-5*", "*opus5*"] @ tools/cellctl/testdata/cellctl-shell-oracle.sh:757 — the brief says "Prohibit Opus 5, including explicit child requests"; the trailing wildcard on the lowercased ID catches every Opus 5 spelling, and row 1's test_ban_cannot_be_removed_by_omitting_deny passes. It over-covers (also refuses Opus 5.5), the safe direction for a prohibition.
RISK-VALUE: NAMED, NOT DERIVED — Go built-in ban isBannedOpus50 @ tools/desk/cmd/cellctl/model.go:61 — replaced the four-pattern list the 2026-09-30 run named; no Verify row executes it and its completeness was not derived here; question #1937 (open).
RISK-VALUE: NAMED, NOT DERIVED — example strong tier "claude-opus-4-8[1m]" @ tools/cellctl/examples/model-policy.json:44 and example deny "*opus-5" @ tools/cellctl/examples/model-policy.json:4 — matches the brief's "map the example Opus tier to 4.8"; no further derivation attempted; question #1938 (open).

No credentials were minted or used; no forge writes; claim left in place.

VERIFY: BLOCKED — 5 of 5 rows pass by hand on darwin, but the check:ci execution witness executed 0 of 5 rows on this host (#1800, open), and rows 3 and 4 depend on host tmux/herdr so they would still fail on a clean Linux runner (#1927, open)

## Review

Gate: model. Examine fallback/settings precedence, credential routing and the negative tests.
Do not infer deployment or independent verification from implementer test output.
