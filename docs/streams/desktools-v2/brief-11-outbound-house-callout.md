---
brief: assay:assay:desktools-v2:11
title: a house callout for the outbound-write check — deployment vocabulary stays out of the shipped tools
why: >-
  The compiled outbound check is generic on purpose: it cannot know one deployment's withheld
  names, and compiling them in would make the shipped binary the disclosure. A configured list
  of identifiers covers the simple case, but a deployment whose rules are a token map, a sweep
  over its own register, or anything with structure would have to grow that list into a pattern
  language on a write gate. Three gates in the tools already solve this the same way: the
  deployment supplies an executable and the tool asks it. This brief adds a fourth on the same
  plumbing, so a house can run its own sweep over every outward write before it leaves, and none
  of its vocabulary ever ships with, or is written by, the public tools.
wave: 3
depends: ["desktools-v2/10"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-outbound-callout
issues: []
schema: brief-v2
authored: 2026-09-17 by desktools-v2 authoring session
sources:
  - "#2111 — Cobra/Viper adoption; CLI compatibility amendment 2026-10-03 (spec §9)"
  - "docs/streams/desktools-v2/spec.md §8.3 — deployment-specific vocabulary is never compiled in; the callout contract; what the forge never sees"
  - "docs/streams/desktools-v2/brief-10-one-outbound-write-check.md (desktools-v2/10, brief id assay:assay:desktools-v2:10, whose consumers list routes the house callout here) — OutboundCheck and OutboundWrite, which this brief extends at its END"
  - "tools/desk/internal/deskkit/callout.go — the shared plumbing: :46 DefaultCalloutTimeout = 5s, :51 calloutWaitDelay, :56 maxCalloutOutput = 64 KiB, Callout.Run (absolute path, regular file, executable, not group/world-writable, no shell, error on every unanswered case). Run sets no cmd.Env, so a callout inherits the caller's whole environment"
  - "tools/desk/cmd/writeguard/callout.go — ASSAY_WRITEGUARD_CALLOUT: one JSON object on stdin with a version field; first stdout token allow | block <reason>; anything else blocks; unset = compiled indicators only; ONLY-WIDENS enforced structurally by asking the callout only after the compiled checks declined to block"
  - "tools/desk/internal/deskkit/riskcallout.go — ASSAY_RISK_CALLOUT: same exec and fail-closed rules on its own self-contained wrapper (:53 5s), argv `classify`, a JSON answer; untrustscan.go:113,:432 — ASSAY_UNTRUSTSCAN_CALLOUT: on deskkit.Callout, a JSON answer that can only ADD markers; :290 semgrepTimeout = 60s is the tree's precedent for a bound well under an agent watchdog"
  - "tools/desk/internal/deskkit/rosterconfig.go:135,:200,:370-371 — callout paths are roster keys; an ASSAY_ key the parser does not recognise refuses the WHOLE roster, so a new key must be added to the known set in the same change"
  - "freshness-checked 2026-09-17 @ 57509073 — every file:line above read at that commit. MEASURED the same day on one developer machine: a token-map sweep (a compiled sweeper reading a YAML map) over one 16 KiB body took 0.01 s wall, three runs; the same sweeper over this whole repository (about 2,800 files) took 11.8 s — past the shared 5 s default, which is why the bound is configurable for the diff-sized `file` kind. COULD-NOT-CHECK: the same sweep through an interpreter-based or multi-engine sweeper, and any sweep inside a CPU-limited container"
gate-why: >-
  This brief lets an executable the deployment supplies decide whether a write leaves, hands
  that executable the full text of every outward write, and adds a REQUIRED mode that refuses
  public writes when the executable is absent. A wrong default either blocks every desk in a
  deployment (required, but the pod never mounted it) or silently runs compiled-only while the
  operator believes their sweep is in force. The human is confirming the fail-closed rules, the
  required mode, and that the callout receives text but never a credential.
decision-trigger: start
decision-issue: 2375
ruling: "https://github.com/medici-finance/assay/issues/2375#issuecomment-6065679413"
exec-tier: strong
exec-tier-why: >-
  (c) — safety plumbing where a subtle error survives the happy path: an `allow` that reaches a
  compiled verdict, a timeout that is a timeout on nothing, an environment that leaks a minted
  token to a third-party executable.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/outbound.go: follow-up desktools-v2/11 (this brief; the callout is consulted at the end of OutboundCheck)"
  - "tools/desk/internal/deskkit/rosterconfig.go: follow-up desktools-v2/11 (this brief; three new recognised roster keys — a shared-value change: every tool that loads the roster must recognise them in the same release or it refuses the roster)"
  - "tools/desk/internal/deskkit/callout.go: follow-up desktools-v2/11 (this brief; an explicit-environment option on Callout — the three existing callers keep today's behaviour)"
  - "tools/desk/README.md and the adopter-facing callout documentation: follow-up desktools-v2/11 (this brief; the stdin contract and an example executable using invented values)"
  - "every deployment's own callout executable and token map: out-of-scope (supplied by the deployment; nothing of the kind is added to this repository)"
version: 2
id: be893ed6-782f-4000-9675-c03ddff205b2
---

# Brief 11 — a house callout for the outbound-write check

## Context

files:
- `tools/desk/internal/deskkit/outbound.go` (planned) — created by `desktools-v2/10`; this brief adds the consult step.
- `tools/desk/internal/deskkit/outboundcallout.go` (planned) (new) + `tools/desk/internal/deskkit/outboundcallout_test.go` (planned) (new).
- `tools/desk/internal/deskkit/callout.go` — an explicit-environment option.
- `tools/desk/internal/deskkit/rosterconfig.go` — the new keys in the known set and the P3 echo.
- `tools/desk/internal/deskkit/testdata/outbound-callout/` (planned) — stub executables.
- `tools/desk/README.md` — the contract.
- `changelog/feat-desktools-v2-11.md` — the per-PR fragment this repository requires (the name the delivering change used).

single-point-of-failure: for a deployment's own vocabulary the ONE control is this callout — by
design nothing compiled knows those words. Two layers stand behind it. The compiled layer of
`desktools-v2/10` runs FIRST and is unaffected by anything the callout says, so a broken or
hostile callout can never make a write LESS checked than a deployment with none. And the
deployment's merge-gate sweep over the merged tree stays as the out-of-band layer: it fails on
a different signal (content on the forge) in a different component (CI) from a client-side
executable. REQUIRED mode exists so the first of those cannot silently be the only one.

facts:
- **Key:** `ASSAY_OUTBOUND_CALLOUT` — an ABSOLUTE path, a roster key resolved like
  `ASSAY_WRITEGUARD_CALLOUT`. Two companions: `ASSAY_OUTBOUND_CALLOUT_REQUIRED`
  (`public` or unset) and `ASSAY_OUTBOUND_CALLOUT_TIMEOUT` (a Go duration, 1s-60s).
- **Plumbing:** `deskkit.Callout` — absolute path, regular file, executable, not group- or
  world-writable, parent directory checked, no shell, output capped at 64 KiB, `WaitDelay` so
  the timeout is real.
- **Request — ONE JSON object on stdin:**
  `{"version":1,"verb":"deskfile new","role":"worker","repo":"example-org/example-repo","visibility":"public","kind":"issue","fields":[{"name":"title","text":"…"},{"name":"body","text":"…"}]}`.
  `visibility` is `public`, `private` or `unknown`. `kind` is `desktools-v2/10`'s set.
  `version` bumps only when a field's MEANING changes. No argv carries text.
- **Answer:** the write guard's — first stdout token `allow`, or `block <reason>`. Anything
  else is a block: a non-zero exit, a timeout, empty output, oversize output, an unknown first
  token. The reason is shown on stderr as `refused: house.callout at <field> — <reason>`.
- **ONLY-WIDENS, structurally:** `OutboundCheck` returns a compiled refusal before the callout
  is asked. There is no value the callout can print that reaches a compiled verdict, so `allow`
  needs no defending. The callout is asked only about writes the compiled layer passed.
- **Unset vs broken.** Key unset → compiled layer only, byte for byte `desktools-v2/10`'s
  behaviour: publishing this must not block every deployment that has not adopted one.
  Key set and anything wrong — missing file, bad mode, non-zero exit, timeout — → the write is
  REFUSED (exit 5) naming which failure, so "my callout said no" and "my callout is broken" read
  differently. A configured callout is never skipped.
- **REQUIRED mode.** With `ASSAY_OUTBOUND_CALLOUT_REQUIRED=public`, a write to a public or
  unknown-visibility target REFUSES when no callout is configured. Private targets are
  unaffected. This closes the recorded residual of the write-guard callout — a roster that fails
  validation collapses to unconfigured and the operator believes a callout is in force — for
  the one case where that belief matters most. Because a collapsed roster would also lose the
  REQUIRED key, the requirement is ALSO honoured from the environment, and every outward verb's
  startup echo prints both the callout path and the required mode.
- **Timeout: 5 s default, up to 60 s by configuration.** Measured, not assumed: a compiled
  token-map sweep over a 16 KiB body took 0.01 s, so the shared 5 s default has three orders of
  magnitude of headroom for body-sized text and stays the default. The `file` kind on the push
  path can carry a multi-megabyte diff (the same sweeper over a whole repository of about 2,800 files
  took 11.8 s), and an interpreter-based or multi-engine sweeper was
  could-not-check, so the bound is configurable up to 60 s (the inbound scan's own ceiling, well
  under an agent watchdog) rather than raised for everyone. A timeout is a refusal.
- **The callout receives text and nothing else.** It runs with an explicit environment —
  `PATH`, `HOME`, `TMPDIR`, `LANG` — and never the caller's, because the caller may hold a
  minted forge token in its environment and the executable is third-party to the tools.
- **Containers.** The executable and whatever data it reads must be IN the image or on a
  read-only mount; a projected volume needs a mode that is executable and not group-writable
  (`0555`), or the existing mode check refuses it. A minimal image has no interpreter, so a
  shell-script callout that works on a desktop is "cannot be spawned" in the pod — which is a
  refusal, never a skip. The callout needs no network and no token. With REQUIRED set and the
  mount absent, every public write from that pod refuses and says why: the intended fail-closed
  outcome, and the reason the startup echo names the path.
- **Nothing about a match is written to the forge.** The reason text is the deployment's and may
  name a withheld token. It goes to stderr only. The audit row records `house.callout`, the
  field name and the content digest — NOT the reason and NOT the text. No verb composes a
  comment, label or body from a refusal.
- **Override.** A `house.callout` block follows whatever `desktools-v2/10`'s human ruling says
  for `withheld.identifier` — it is the same class of rule. A broken-callout refusal is never
  overridable: the remedy is to fix or unset the callout, which is a human's configuration.
- Out of scope: any deployment's executable or token map; collapsing the risk callout's
  private wrapper onto `deskkit.Callout`; a callout on reads.

## Human decision
The outbound-write check in the desk tools is generic: it cannot know the names one deployment
withholds, and those names must never be compiled into or shipped with public tools. The
proposal lets a deployment point the tools at its own executable. Before any write leaves the
machine, after the built-in checks have passed it, the tools hand that executable one JSON
object — the verb, the role, the target repository and its visibility, the kind of write, and
the text being written — and it answers allow, or block with a reason. It can add a block; it
can never clear one the built-in checks raised. The reason is shown locally and is never
written to the forge or the audit log.

Options:
1. **As proposed** — unset means built-in checks only; configured-but-broken refuses the write;
   a deployment may set a REQUIRED mode under which public-target writes refuse when no
   executable is configured; 5 second default timeout, configurable to 60; the executable gets
   the text but a scrubbed environment with no credential.
2. **No REQUIRED mode** — simpler; a misconfigured deployment silently runs built-in checks only
   and finds out from its merge gate, a review round trip later.
3. **REQUIRED by default for public targets** — strongest, but every adopter with no executable
   is refused on their first public write, which makes publishing the tools a breaking change.
4. **Hold** — handing full write text to a deployment-supplied executable needs more design.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Every stub, fixture and documented example uses invented values (`example-withheld-slug`,
  `example-org/example-internal`). No real deployment's executable, token map or vocabulary is
  added to this repository, in a test or anywhere else.
- If greening requires letting a callout answer clear a compiled refusal, or skipping a
  configured callout on failure: STOP — `BLOCKED-ON-HUMAN — security-gate removal`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. The three roster keys, recognised by the parser in the same change, echoed at startup.
2. The consult step at the END of `OutboundCheck`; the request envelope; the answer parse.
3. The explicit-environment option on `Callout`, used here; existing callers unchanged.
4. REQUIRED mode, honoured from roster and environment.
5. Stub executables under `testdata/outbound-callout/` (allow, block, exit 3, sleep past the
   deadline, print nothing, print 100 KiB, print `maybe`, dump its environment) and the tests
   the Verify rows name.
6. Document the contract with one example executable that greps the `text` fields for words
   read from a file the deployment owns.
7. Measure one callout round trip on the implementer's machine and record it in the PR body.

### Conformance — shared by every verb, extending `desktools-v2/10`'s table

| # | Configuration | Write | Expect |
|---|---|---|---|
| H1 | key unset | any row of `desktools-v2/10`'s table | that table's result, unchanged |
| H2 | callout answers `block example-house-rule` | clean body, public | refused `house.callout`; zero forge calls; the reason is on stderr and in NO audit field |
| H3 | callout answers `allow` | body naming the configured withheld slug, public | STILL refused `withheld.identifier`, and the callout was never executed |
| H4 | key set, file missing / mode `0775` / exit 3 / sleeps past the deadline / empty / 100 KiB / prints `maybe` | clean body, public | refused each time, the message naming WHICH failure |
| H5 | REQUIRED=`public`, key unset | clean body, public and unknown | refused; the same body to a private target passes |
| H6 | callout dumps its environment; caller holds a token variable | any | the dump carries no token variable |
| H7 | callout answers `block` | a commit message, on the push path | refused before any push |

## Cobra/Viper integration — callout settings retain restricted sources

The new callout keys remain validated by the existing roster parser and fail-closed callout boundary. The bounded CLI owners authored by desktools-v2/15 bind only admissible sources through Viper; no blanket environment binding, config search or CLI flag may silently override these protected settings. Help/version must neither invoke the callout nor read its secret-bearing input. Record all consuming commands in cli-migration.json when /15 lands; this brief retains ownership of callout policy and does not fork the parser migration.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run TestOutboundCalloutConformance -v` | output contains the literal line `--- PASS: TestOutboundCalloutConformance` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutAllowCannotClearCompiledBlock -v` | output contains `--- PASS: TestCalloutAllowCannotClearCompiledBlock` and the test asserts the stub recorded ZERO executions — only-widens proven structurally. Fail-first: with the consult step moved ahead of the compiled layer this is RED |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestBrokenCalloutBlocks -v` | output contains `--- PASS: TestBrokenCalloutBlocks` with one sub-test per failure in H4. Fail-first: with the error branch returning nil this is RED |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRequiredCalloutAbsentRefusesPublicOnly -v` | output contains `--- PASS: TestRequiredCalloutAbsentRefusesPublicOnly` |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundCalloutEnvironmentCarriesNoToken -v` | output contains `--- PASS: TestOutboundCalloutEnvironmentCarriesNoToken` |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutReasonNeverReachesForgeOrAudit -v` | output contains `--- PASS: TestCalloutReasonNeverReachesForgeOrAudit` — the recording fake forge and the audit log are both searched for the stub's reason string |
| 8 | `cd tools/desk && go test ./cmd/deskfile/ ./cmd/deskpr/ -run 'TestHouseCallout' -v` | output contains `--- PASS: TestHouseCalloutBlocksIssueFiling` and `--- PASS: TestHouseCalloutBlocksPushOnCommitMessage` — the FLOW rows: the block is reached through two real verbs, not only through the function |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterRecognisesOutboundCalloutKeys -v` | output contains `--- PASS: TestRosterRecognisesOutboundCalloutKeys` — a roster carrying the three keys loads; without the parser change it refuses the whole roster |
| 10 | `grep -c 'ASSAY_OUTBOUND_CALLOUT' tools/desk/README.md` | exit 0; count >= 3 (the three keys are documented) |
| 11 | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run TestOutboundCalloutConformance -v` | pass exit=0 | sha256:6cd399f72855 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutAllowCannotClearCompiledBlock -v` | pass exit=0 | sha256:99995354145c | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestBrokenCalloutBlocks -v` | pass exit=0 | sha256:3c5e8e5a8f80 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRequiredCalloutAbsentRefusesPublicOnly -v` | pass exit=0 | sha256:a495ea9bfb45 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundCalloutEnvironmentCarriesNoToken -v` | pass exit=0 | sha256:88fcba86e612 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutReasonNeverReachesForgeOrAudit -v` | pass exit=0 | sha256:216437636925 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskfile/ ./cmd/deskpr/ -run 'TestHouseCallout' -v` | pass exit=0 | sha256:7177da306ca6 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterRecognisesOutboundCalloutKeys -v` | pass exit=0 | sha256:6ab5ba102c36 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -c 'ASSAY_OUTBOUND_CALLOUT' tools/desk/README.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --consumers --root .` | pass exit=0 | sha256:d77e78fa27a9 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2529 run=833edec95be1d57610ce8fd0a5718b119bfd27a8f40ce399f13731094de7a7f1 source=a4bc0c0cfe0634bef4a4c52c1b7078f1b3f7fd3c brief=docs/streams/desktools-v2/brief-11-outbound-house-callout.md model=claude-opus-5-5 tier=any

### Verify pass — 2026-10-11, dispatched non-implementer verifier (verify-desk, assay-verifier-app[bot])

Ran against merged main at a4bc0c0cfe0634bef4a4c52c1b7078f1b3f7fd3c (the delivering change is #2383, merge commit 8284cf0296c8; the human ruling is option 1 of #2375, recorded in DR-outbound-callout). Runner: darwin/arm64, go1.27.1, git 2.56.0, statusgen v1.0.37. Admission: `deskdispatch --check-verifier` exit 0 before any row and again after this edit. The Verify table carries no check:ci rows, so the Linux-only network-off sandbox did not apply; every row ran on the host. No storage-deferred Go cache lock flakes were seen in any run. Every row was run by hand first; the witness table above is the second, tool-driven run of the same eleven commands.

| # | Command | Expected | Observed (exit + key output) | Date / runner |
|---|---------|----------|------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | exit 0 | exit 0, no output | 2026-10-11 assay-verifier-app[bot] |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run TestOutboundCalloutConformance -v` | literal `--- PASS: TestOutboundCalloutConformance` | exit 0; `--- PASS: TestOutboundCalloutConformance (3.35s)` with sub-tests H1, H2, H3, H4 (7 sub-cases), H5 (roster / environment / environment survives a collapsed roster), H6, H7 (ref / commit / file) all PASS | 2026-10-11 assay-verifier-app[bot] |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutAllowCannotClearCompiledBlock -v` | PASS line; stub records zero executions; RED with consult moved ahead | exit 0; `--- PASS: TestCalloutAllowCannotClearCompiledBlock (0.00s)`. The test asserts the stub's run counter is 0 and the compiled withheld.identifier refusal; the counter is proven live by other tests that require it to rise. Fail-first, in a scratch copy outside the home: house step moved ahead of the compiled layer gave `--- FAIL` with "the callout was executed 1 times for a write the compiled layer had already refused" | 2026-10-11 assay-verifier-app[bot] |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestBrokenCalloutBlocks -v` | PASS line, one sub-test per H4 failure; RED with error branch returning nil | exit 0; `--- PASS: TestBrokenCalloutBlocks (2.85s)` with sub-tests missing_file, mode_0775, exit_3, sleeps_past_the_deadline, prints_nothing, prints_100_KiB, prints_maybe all PASS. Fail-first, scratch copy: the Run-error branch returning nil gave `--- FAIL` on missing_file, mode_0775, exit_3 and sleeps_past_the_deadline (the other three fail through their own branches, as designed) | 2026-10-11 assay-verifier-app[bot] |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRequiredCalloutAbsentRefusesPublicOnly -v` | PASS line | exit 0; `--- PASS: TestRequiredCalloutAbsentRefusesPublicOnly (0.07s)` (from_the_roster, from_the_environment, environment_survives_a_collapsed_roster). Extra mutation, scratch copy: dropping the private-target exemption gave `--- FAIL` on from_the_roster and from_the_environment | 2026-10-11 assay-verifier-app[bot] |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundCalloutEnvironmentCarriesNoToken -v` | PASS line | exit 0; `--- PASS: TestOutboundCalloutEnvironmentCarriesNoToken (0.06s)`. Extra mutation, scratch copy: passing a nil environment (inherit the caller's) gave `--- FAIL`, the dump showing the three invented token variables the test plants | 2026-10-11 assay-verifier-app[bot] |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutReasonNeverReachesForgeOrAudit -v` | PASS line; fake forge and audit log searched for the reason | exit 0; `--- PASS: TestCalloutReasonNeverReachesForgeOrAudit (0.68s)` with 10 sub-cases (well-formed block, colon, BLOCKED, upper-case ALLOW, deny, second-line words, terminal escape, non-zero exit, empty, oversized) all PASS | 2026-10-11 assay-verifier-app[bot] |
| 8 | `cd tools/desk && go test ./cmd/deskfile/ ./cmd/deskpr/ -run 'TestHouseCallout' -v` | `--- PASS: TestHouseCalloutBlocksIssueFiling` and `--- PASS: TestHouseCalloutBlocksPushOnCommitMessage` | exit 0; `--- PASS: TestHouseCalloutBlocksIssueFiling (0.80s)` (deskfile) and `--- PASS: TestHouseCalloutBlocksPushOnCommitMessage (1.55s)` (deskpr; commit and file sub-tests); also `--- PASS: TestHouseCalloutOffVocabularyNeverLogged` | 2026-10-11 assay-verifier-app[bot] |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterRecognisesOutboundCalloutKeys -v` | PASS line | exit 0; `--- PASS: TestRosterRecognisesOutboundCalloutKeys (0.00s)`. Independently, the live roster echo printed all three keys (path unset, REQUIRED unset, timeout unset with 5s default) during admission | 2026-10-11 assay-verifier-app[bot] |
| 10 | `grep -c 'ASSAY_OUTBOUND_CALLOUT' tools/desk/README.md` | exit 0; count >= 3 | exit 0; `6` (the three keys in the key table plus the install recipe) | 2026-10-11 assay-verifier-app[bot] |
| 11 | `statusgen --consumers --root .` | exit 0; no routing claim disproved | exit 0; "no brief files in the diff against a4bc0c0cfe06 — nothing to corroborate". On a clean detached tree at merged main this row is vacuous. Supplemental run with the base set to the parent of the decision-record commit (so the brief file is in the diff): exit 0, 0 corroborated and 0 disproved; that diff left all five consumers entries untouched, so it carries no evidence either way. Checked by hand instead: outbound.go's OutboundCheck ends with the house step after the compiled verdict; rosterconfig.go adds the three keys to the known set and the startup echo; callout.go adds an Env field that only the new caller sets (the write-guard and untrusted-scan callers construct Callout without it); the README documents the stdin contract and an example using invented values; no deployment executable or token map was added (fixtures and README scanned, invented values only) | 2026-10-11 assay-verifier-app[bot] |

**Risk-bearing value enumeration** (trigger: risk sensitive-data: yes). Literals introduced or named by the diff, ranked by what breaks if wrong; all are reversible by an edit and a release (irreversible: no), but a wrong value here leaks a deployment's withheld text or a credential before anyone can undo it, so the top three are derived.

- RISK-VALUE: DERIVED — outboundCalloutEnv allow-list = {"PATH", "HOME", "TMPDIR", "LANG"} @ tools/desk/internal/deskkit/outboundcallout.go:205 — the brief's facts name exactly these four and nothing else; none is a credential variable; the slice is never nil, so Callout.Env replaces rather than extends the caller's environment (callout.go:131-132); the nil-env mutation reddened row 6, so the bound is load-bearing.
- RISK-VALUE: DERIVED — REQUIRED scope `RepoVisibility(w.Repo) != VisibilityPrivate` @ tools/desk/internal/deskkit/outboundcallout.go:271, with environment tightening `s.Required = true` @ outboundcallout.go:146 — the ruling (option 1 of #2375) requires public-target writes to refuse with no executable; the brief extends that to unknown visibility, and "not private" is the only expression that covers both while leaving private targets alone; the environment can only add the requirement, matching the brief's collapsed-roster rationale. Mutation dropping the private exemption reddened row 5.
- RISK-VALUE: DERIVED — MaxOutboundCalloutTimeout = 60 * time.Second @ tools/desk/internal/deskkit/outboundcallout.go:61 (with DefaultCalloutTimeout = 5 * time.Second @ callout.go:46 and MinOutboundCalloutTimeout = 1 * time.Second @ outboundcallout.go:60) — the ruling fixes "5 second default timeout, configurable to 60"; 60s equals the tree's existing inbound-scan ceiling semgrepTimeout = 60 * time.Second @ untrustscan.go:290; the brief's measurement (0.01 s for a 16 KiB body) gives the 5s default three orders of headroom; a timeout is a refusal, so a short bound fails closed, not open.
- RISK-VALUE: NAMED, NOT DERIVED — maxCalloutOutput = 64 << 10 @ tools/desk/internal/deskkit/callout.go:56 — shared and unchanged by this diff; the brief cites it but gives no derivation of why 64 KiB rather than another bound; oversize output refuses, so any value fails closed.
- RISK-VALUE: NAMED, NOT DERIVED — maxCalloutReasonRunes = 300 @ tools/desk/internal/deskkit/outboundcallout.go:71 — a terminal-only display cap; the brief states no bound; reversible operational knob, ranked last.
- Also enumerated, ranked last (identifiers and protocol constants, no derivation needed): RuleHouseCallout = "house.callout" @ outboundcallout.go:55; outboundCalloutRequestVersion = 1 @ outboundcallout.go:65; outboundCalloutRequiredPublic = "public" @ outboundcallout.go:68; separator set ",;\t\n\r" @ outboundcallout.go:97; calloutWaitDelay = 250 * time.Millisecond @ callout.go:51; the three key names @ rosterconfig.go:210, 215, 218.

Gate: this item is gate: human with risk sensitive-data: yes. This Evidence is for the human sign-off. It is not a model sign-off, and the item's status is left where it is.

Verification-Attestation: medici-finance/assay#2529 run=833edec95be1d57610ce8fd0a5718b119bfd27a8f40ce399f13731094de7a7f1 source=a4bc0c0cfe0634bef4a4c52c1b7078f1b3f7fd3c brief=docs/streams/desktools-v2/brief-11-outbound-house-callout.md model=claude-opus-5-5 tier=any

**VERIFY: PASS** — 11/11 rows observed passing at a4bc0c0cfe06; fail-first checks for rows 3 and 4 reddened as the brief states; row 11 is vacuous on a clean tree and its consumers claims were checked by hand. Routed to the human gate (sensitive-data), not flip-eligible.

### Container execution witness — 2026-10-20, dispatched non-implementer verifier (verify-desk, assay-verifier-app[bot])

Mechanical re-run of all eleven Verify rows at merged main 4fa40dd33e06c69298b4c437d71e13acb3f90e9c, under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Runner: local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built from golang:1.27.2-trixie, git 2.47), started with `--network none`; `statusgen verifyrun --sandbox container-netns` with the engine's default seccomp profile; statusgen and the desk tools built from this tree inside the container. Admission: `deskdispatch --check-verifier` exit 0 before the run and again after this edit. This pass does NOT supersede the standing 2026-10-13 fail verdict for this brief, recorded in the unmerged medici-finance/assay#2824. That run's fail-first witness, with Base cells taken at the delivery parent b7217d55004a, found rows 1-9 and 11 green at base (non-discriminating) and only row 10 red. It recorded risk-row fail-first acceptance as 1/11, on the check-definition blocker medici-finance/assay#2802, which is still open. This witness re-runs the head commands only and carries no Base cells, so it does not answer that blocker. Once medici-finance/assay#2824 lands, both blocks stand in date order. verifyrun exit 0: 11 pass, 0 fail, 0 could-not-run. For rows 2 to 9 the witness asserts exit status and records an output hash; the literal `--- PASS:` lines those rows name are carried by the by-hand substance below.

The substance is the existing by-hand Evidence block above (Verify pass of 2026-10-11 at source a4bc0c0cfe0634bef4a4c52c1b7078f1b3f7fd3c), which observed every row passing with 0 fail, including the literal PASS lines and the fail-first mutations for rows 3 and 4. Its risk-value enumeration still holds at this source: outboundcallout.go is byte-identical between the two sources, and callout.go changed only its process-run call (RunProcess), leaving DefaultCalloutTimeout = 5s at callout.go:46 and maxCalloutOutput = 64 << 10 at callout.go:56 unchanged.

Verification-Attestation: medici-finance/assay#3591 run=1c0ad25372163af3638976e01111aed6849778155760dab98af3ce57b50f0f07 source=4fa40dd33e06c69298b4c437d71e13acb3f90e9c brief=docs/streams/desktools-v2/brief-11-outbound-house-callout.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run TestOutboundCalloutConformance -v` | pass exit=0 sandbox=container-netns | sha256:3e0deb5ec255 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutAllowCannotClearCompiledBlock -v` | pass exit=0 sandbox=container-netns | sha256:6c91f2e11980 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestBrokenCalloutBlocks -v` | pass exit=0 sandbox=container-netns | sha256:15b578fa6d00 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRequiredCalloutAbsentRefusesPublicOnly -v` | pass exit=0 sandbox=container-netns | sha256:58b1b1b3ad70 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundCalloutEnvironmentCarriesNoToken -v` | pass exit=0 sandbox=container-netns | sha256:0df4082c5433 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCalloutReasonNeverReachesForgeOrAudit -v` | pass exit=0 sandbox=container-netns | sha256:a7a1c75d2afe | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskfile/ ./cmd/deskpr/ -run 'TestHouseCallout' -v` | pass exit=0 sandbox=container-netns | sha256:7cf578fbc91a | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterRecognisesOutboundCalloutKeys -v` | pass exit=0 sandbox=container-netns | sha256:8290621b6b60 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -c 'ASSAY_OUTBOUND_CALLOUT' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:06e9d52c1720 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:05ccf73ce950 | 2026-10-20 | assay-verifier-app[bot] @ 4fa40dd33e06 (on-behalf-of human:ian) (forge-identity) |

**VERIFY: BLOCKED** — check-definition (medici-finance/assay#2802). Head commands are 11/11 exit 0 in the container witness at 4fa40dd33e06, on top of the 2026-10-11 by-hand substance, but risk-row fail-first acceptance is still open. The 2026-10-13 FAIL in medici-finance/assay#2824 found rows 1-9 and 11 non-discriminating at base, and this witness has no Base cells, so a simulated verified flip at this source reds `statusgen --lint` (LINT: FAIL, a no-Base-cell PROBLEM on each of rows 1-11). This item is also gate: human with sensitive-data: yes, so it stays routed to the maintainer sign-off on medici-finance/assay#2775 and is not flip-eligible.

## Review
Gate: human (sensitive-data: yes — a deployment-supplied executable receives the text of every
outward write and decides whether it leaves). MANDATORY human sign-off; the human rules the
options in `## Human decision`. Reviewer answers, in the verdict: what single control stands
between a deployment's withheld word and a public repository (this callout), and which row
proves a lower layer holds with it bypassed (row 3: the compiled layer refuses with the callout
saying allow; row 5: REQUIRED refuses with no callout at all). Rows 3-7 are negative-path rows.
Reviewer records verdict + date in the stream README table.
