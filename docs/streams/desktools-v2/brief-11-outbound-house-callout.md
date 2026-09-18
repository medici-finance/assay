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
issues: []
schema: brief-v2
authored: 2026-09-17 by desktools-v2 authoring session
sources:
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
version: 1
id: be893ed6-782f-4000-9675-c03ddff205b2
---

# Brief 11 — a house callout for the outbound-write check

## Context

files:
- `tools/desk/internal/deskkit/outbound.go` (planned) — created by `desktools-v2/10`; this brief adds the consult step.
- NEW `tools/desk/internal/deskkit/outboundcallout.go` (planned) + `_test.go`.
- `tools/desk/internal/deskkit/callout.go` — an explicit-environment option.
- `tools/desk/internal/deskkit/rosterconfig.go` — the new keys in the known set and the P3 echo.
- `tools/desk/internal/deskkit/testdata/outbound-callout/` (planned) — stub executables.
- `tools/desk/README.md` — the contract.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

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
  path can carry a multi-megabyte diff (the same sweeper over a whole repository of about 2,800
  files took 11.8 s), and an interpreter-based or multi-engine sweeper was
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

## Review
Gate: human (sensitive-data: yes — a deployment-supplied executable receives the text of every
outward write and decides whether it leaves). MANDATORY human sign-off; the human rules the
options in `## Human decision`. Reviewer answers, in the verdict: what single control stands
between a deployment's withheld word and a public repository (this callout), and which row
proves a lower layer holds with it bypassed (row 3: the compiled layer refuses with the callout
saying allow; row 5: REQUIRED refuses with no callout at all). Rows 3-7 are negative-path rows.
Reviewer records verdict + date in the stream README table.
