---
brief: assay:assay:desktools-v2:10
title: one outbound-write check at the forge write seam, keyed on the target's visibility
why: >-
  What a deployment may write to a forge is enforced today by whichever verb remembered to call
  whichever scan. deskfile files issues on public repositories with no self-containment or
  withheld-identifier check at all; nothing on the push path looks for a withheld name in a
  diff, a commit message or a branch name; tool-composed comments and labels are never looked
  at; two verbs offer the audited override and three refuse with no way through. In one
  deployment on one day that produced a public issue naming withheld identifiers and a withheld
  repository name in a test-file comment that only a merge-gate sweep caught, after review.
  The fix is one function every outward write passes before it leaves the machine, placed where
  every write already goes, so a new verb cannot forget it and a rule lives in the tools rather
  than in prose an agent has to remember.
wave: 2
depends: ["desktools-v2/01"]
unblocks: ["desktools-v2/11"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
design: DR-desktools-v2-10
decision-issue: 1319
authored: 2026-09-17 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §8 — the driver's direction of 2026-09-17, the three incidents stated without their identifiers, the verb × check table (§8.2) and the design (§8.3)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — its '## Outward writes' table is the checklist this brief ticks against"
  - "tools/desk/internal/deskkit/forgeresolve.go:438,:451,:469-473 — ForgeFor / ResolveForge: the ONE site that constructs a backend; forgeresolve_test.go:282 TestForgeSingleConstructionSite pins that"
  - "tools/desk/internal/deskkit/forge.go:8-17 — body checks WRAP the interface and live in neither backend; :218-303 the write operations"
  - "tools/desk/internal/deskkit/bodycheck.go:313,:329 (BodyCheck / ScanSurface), selfcontain.go:160,:200,:218 (WithheldIdentifiers / SelfContainApplies / SelfContainCheck), scanoverride.go (ScanOverrideFlag = force-scan-override, HandleScanRefusal, the digest-only audit row, the non-overridable impersonation guard)"
  - "tools/desk/internal/deskkit/config.go:34-75,:174,:197 and rosterconfig.go:86-91 — the allowed-repos roster states public|private per repository; RepoVisibility reads it with no network call; VisibilityRiskClassed treats everything except known-private as public"
  - "tools/desk/cmd/deskpr/deskpr.go:712-768 (scanWrite: title, branch name, diff secret arms, ruling-claim guard on added lines) and tools/desk/cmd/deskpushguard/main.go — the two places a push is already inspected"
  - "freshness-checked 2026-09-17 @ 57509073 — every file:line above and every cell of spec §8.2 read at that commit; no function named OutboundCheck exists; no verb runs any e-mail or phone-number check"
gate-why: >-
  This brief changes what the desk tools REFUSE to write, for every verb at once, and it touches
  the override path. Two ways to get it wrong survive a happy-path suite: a check that is too
  eager starves every desk of its write budget on false refusals (that has happened before with
  file:line references), and a check that is wired one layer too high is skipped by the next
  verb. The human is confirming the refusal policy per layer and per target visibility, and
  which refusals may be overridden from the verb at all.
decision-trigger: start
exec-tier: strong
exec-tier-why: >-
  (a) the personal-data shapes and the refuse/notice split are design decisions the facts bound
  but do not fully specify; (b) correctness is cross-component — one function reached from a
  Forge decorator, from deskpr's push scan and from the pre-push hook; (c) it is safety plumbing
  where a wrapper that misses one method passes every test that does not call that method.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit (NEW outbound.go, outboundforge.go, personaldata.go — planned): follow-up desktools-v2/10 (this brief)"
  - "tools/desk/internal/deskkit/forgeresolve.go: follow-up desktools-v2/10 (this brief; ResolveForge returns the checked Forge)"
  - "tools/desk/cmd/deskfile, deskpr, deskreply, deskpost, deskevidence, deskclose, deskprovenance, desklabel, deskflip, deskdispatch: follow-up desktools-v2/10 (this brief; each verb's own scan calls are DELETED once the seam check covers them, and each gains the one override flag)"
  - "tools/desk/cmd/deskpushguard: follow-up desktools-v2/10 (this brief; the hook calls the same function over commit messages, ref names and added lines)"
  - "tools/desk/internal/forgeban: follow-up desktools-v2/10 (this brief; the ban that proves no write path is built around the check)"
  - "statusgen, tools/cellctl: out-of-scope (neither writes to a forge through a Forge; statusgen's writes are local files)"
  - "the house callout for this check: follow-up desktools-v2/11"
version: 3
id: 72fc4a5f-6e8f-4c4e-807d-608df5501950
---

# Brief 10 — one outbound-write check at the forge write seam

## Context

files:
- NEW `tools/desk/internal/deskkit/outbound.go` (planned) — `OutboundWrite`, `OutboundCheck`,
  the rule ids.
- NEW `tools/desk/internal/deskkit/outboundforge.go` (planned) — the checking `Forge` decorator.
- NEW `tools/desk/internal/deskkit/personaldata.go` (planned) — the generic personal-data pass.
- `tools/desk/internal/deskkit/forgeresolve.go` — `ResolveForge` returns the decorator.
- `tools/desk/cmd/deskpr/deskpr.go` (`scanWrite`), `tools/desk/cmd/deskpushguard/` — the push path.
- every `tools/desk/cmd/*` verb in spec §8.2 — its own scan calls removed, the override flag added.
- `tools/desk/internal/forgeban/` and `tools/desk/internal/deskkit/forgeresolve_test.go` — the proof.
- NEW `tools/desk/internal/deskkit/outbound-mutations.json` (planned) — the fail-first entries, run by `muhar`.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

single-point-of-failure: the ONE control is `OutboundCheck` running before a write leaves. Three
independent layers stand around it, each failing for a different reason in a different place.
(1) STRUCTURE — a verb cannot hold an unchecked `Forge`: `ResolveForge` is the only
construction site and it returns the decorator; `TestForgeSingleConstructionSite` and the
`forgeban` rule below redden the BUILD if a second site or an unwrapped return appears.
(2) COMPLETENESS — `TestOutboundForgeWrapsEveryWriteMethod` walks the `Forge` interface by
reflection and fails when a method that takes text is not routed through the check, so a write
operation added later cannot be forgotten either. (3) OUT OF BAND — a deployment's own
merge-gate sweep over the merged tree, which exists today and stays: it sees what reached the
forge whatever the client did. This brief makes that last layer the backstop it should be,
instead of the first thing to notice.

facts:
- **The seam.** `OutboundWrite{Tool, Verb, Role, Repo, Visibility, Kind, Fields []OutboundField}`,
  where a field is `{Name, Text}` and `Kind` is one of `issue`, `change`, `comment`,
  `review`, `label`, `file`, `commit`, `ref`. The decorator builds it for `FileIssue`,
  `PostComment`, `PostCommentTyped`, `EditComment`, `CreateDraftChange`, `EditChange`,
  `PostReview`, `ApplyLabels` and `WriteFile`, calls `OutboundCheck`, and delegates only on
  nil. Writes that carry no author text (`CloseIssue*`, `ReopenIssue`, `MarkReadyForReview`,
  `SetMergeHold`, `DeleteRef`) pass straight through and are listed as such in the
  completeness test, so "carries no text" is a recorded decision per method, not an omission.
- **The push path.** Text that leaves by `git push` never crosses a `Forge`. `deskpr`'s
  `scanWrite` and the `deskpushguard` hook call the SAME `OutboundCheck` with kinds `commit`
  (each message in the pushed range), `ref` (the branch name) and `file` (ADDED diff lines
  only — a removed line cannot introduce a disclosure, and scanning removals refuses the very
  branch that deletes one). The existing whole-diff secret arms keep their breadth.
- **Visibility is the key, and it is already configured.** `RepoVisibility(repo)`; the public
  layers run when `VisibilityRiskClassed(repo)` is true, i.e. for public AND unknown targets.
  No network read: a gate that must reach the forge fails open when the forge is down.
- **Layers and rule ids** — compiled, generic, no deployment value in the binary:

  | Rule id | Layer | Targets | Verdict |
  |---|---|---|---|
  | `secret.*` | the existing credential / entropy scan, unchanged | all | refuse |
  | `voice.ruling-claim` | the existing impersonation guard, unchanged | all | refuse, never overridable |
  | `pii.email` | an e-mail address outside the allow-list | all | refuse |
  | `pii.phone` | `+` and 8-15 digits with optional separators (E.164 shape) | all | refuse |
  | `pii.phone-ambiguous` | a separator-grouped 10-11 digit run with no `+` | all | NOTICE only |
  | `selfcontain.*` | the existing self-containment categories, unchanged | public, unknown | refuse / notice as today |
  | `withheld.identifier` | the configured `ASSAY_WITHHELD_IDENTIFIERS` set | public, unknown | refuse; NOTICE when unset, as today |

- **The e-mail allow-list is generic and compiled**: forge no-reply addresses
  (`*@users.noreply.github.com`, `noreply@github.com`, `*@noreply.gitlab.com`), the reserved
  documentation domains (`example.com`, `example.org`, `example.net`, and the `.example`,
  `.invalid`, `.test` TLDs), and the no-reply addresses of the roster's own bot identities.
  Nothing else. A deployment that needs more says so through the callout (`desktools-v2/11`),
  never through a compiled list.
- **It refuses and never rewrites.** The refusal is `refused: <rule id> at <field>:<line> — …`,
  exit 5. No redaction mode, no "post with the span removed" flag.
- **One override, offered by every verb.** `--force-scan-override "<reason>"` — the existing
  flag (`ScanOverrideFlag`), whose value IS the reason (12 characters minimum), audit-logged
  with the rule id and a DIGEST of the content before the write, fatal if the row cannot be
  written. Today `deskfile`, `deskpost` and `deskevidence` refuse with no such path; they
  gain it. The identity on the row is self-reported, so the override is forensics, not
  authorization — which is why the human decision below is about which rule ids accept it.
- **Nothing about a match reaches the forge.** stderr may name the rule, the line and the span.
  The audit row carries rule id and digest only. No refusal, notice or override ever composes a
  forge write.
- **The ban.** `forgeban` gains one rule: outside `internal/deskkit`, no non-test file may
  name `GitHubForge` or `GitLabForge` as a type, composite literal or conversion target, and
  no file may type-assert a `Forge` back to a backend. With the single construction site, that
  is the proof no write path bypasses the check; it fails the build, it is not a count.
- Out of scope: the house callout (`desktools-v2/11`); any deployment's vocabulary; changing
  what the secret scan or the self-containment categories match; reading visibility live.

## Human decision
The desk tools are about to refuse more, for every verb at once. Today each write verb runs
whichever checks its author remembered: issue filing on a public repository runs no disclosure
check at all, nothing looks at commit messages or branch names, and only two verbs offer the
audited override. The proposal is one check every outward write passes before it leaves the
machine. It refuses with a rule id and a location and never edits the author's text. For every
target it looks for credentials, e-mail addresses and phone-number shapes (forge no-reply
addresses and reserved example domains are allowed). For public targets, and targets whose
visibility is not stated, it also looks for references that only resolve inside a private
deployment and for the identifiers the deployment has configured as withheld.

The override is the part that needs a human. The existing flag takes a written reason and
leaves an audit row, but the identity on that row is self-reported, so any session that hits a
refusal can take it.

Options:
1. **Layered overrides (proposed)** — credential, personal-data and self-containment refusals
   stay overridable with the audited flag, because their false positives are real and a block
   with no way through has pushed work off the sanctioned transport before. A withheld-identifier
   refusal on a public target is NOT overridable from the verb: the way through is to reword, or
   for a human to change the configured set. Consequence: an agent cannot publish a withheld
   identifier by writing a reason; a human edits configuration to allow one.
2. **Everything overridable, audited** — uniform and simplest; a withheld identifier can reach a
   public repository on any session's written reason, and the audit row is how it is found
   afterwards.
3. **Nothing new overridable** — personal-data and withheld refusals are hard stops everywhere.
   Strongest, and the most likely to strand work on a false positive such as a phone-shaped id.
4. **Hold** — the layers or the visibility key need more design first.

Default if no answer: none — blocks until answered.

Ruled 2026-09-21 (#1319): option 1 — layered overrides.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- This brief MOVES checks; it must not drop one. If any existing scan, category or guard would
  match less after the change than before, STOP — `BLOCKED-ON-HUMAN — security-gate removal`,
  label `needs-decision`.
- Every fixture uses invented values: `example-org/example-internal`, `example-withheld-slug`,
  `person@corp.example`. No real deployment's repository, stream or identifier appears in a
  test, a comment, a commit message or the PR — that is the defect this brief exists to stop.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `OutboundCheck` and its rule ids, composing the existing scans unchanged plus the
   personal-data pass; the refuse/notice split per the table.
2. The decorator, returned by `ResolveForge`; the completeness test over the interface.
3. The push path: `scanWrite` and the hook call the same function for `commit`, `ref` and
   added-line `file` content.
4. Give every verb the one override flag through `HandleScanRefusal`; implement the override
   policy the human chose.
5. DELETE each verb's own `ScanSurface` / `BodyCheck` / `SelfContainCheck` call once the seam
   covers it, in the same change, and show per verb that the same fixture is refused before and
   after. Leave `deskscanbody` and the `--check` paths calling the function directly — a
   pre-flight that runs before a `Forge` exists is a second caller, not a bypass.
6. The `forgeban` rule.
7. The conformance table below as ONE table-driven test, `TestOutboundConformance`, and the
   `muhar` mutation spec that reddens it.

### Conformance — the same table for every verb

`TestOutboundConformance` runs every row against every text-carrying write method of the
decorator and against the push path, so a verb has no conformance of its own to get wrong.

| # | Text | Target | Expect |
|---|---|---|---|
| C1 | a body naming `example-withheld-slug` (configured withheld) | public | refused, `withheld.identifier`, no delegate call |
| C2 | the same body | private | passes, delegate called once |
| C3 | the same body | visibility not stated | refused (unknown is treated as public) |
| C4 | an added diff line, a comment in a test file, naming `example-org/example-internal` (private per the roster) | public | refused, `selfcontain.*`, before any push |
| C5 | `person@corp-mail.dev` | private | refused, `pii.email` (and `person@corp-mail.test` passes: a reserved TLD) |
| C6 | `1234567+example-bot[bot]@users.noreply.github.com` | public | passes |
| C7 | `+1 415 555 0100` | private | refused, `pii.phone` |
| C8 | `415-555-0100` | public | passes with a NOTICE, `pii.phone-ambiguous` |
| C9 | a label name carrying the withheld slug | public | refused, kind `label` |
| C10 | a commit message carrying the withheld slug | public | refused, kind `commit`, before any push |
| C11 | any refused row, with the override and a 12-character reason | per the human's ruling | overridable rule ids pass and leave one audit row holding rule id + digest and NOT the text; non-overridable ids still refuse |
| C12 | any refused row | any | the recording fake forge saw ZERO calls and the refusal text appears in no composed body |

Coverage boundary: "one check" means every `Forge` write and deskpr's push. A push that does
not go through deskpr (the generic push, merge and verify-loop tools, a hand-typed `git push`)
is checked only where the `deskpushguard` pre-push hook is installed, and that hook fails open
on could-not-check. Raw API writers that never hold a `Forge` (fleet provisioning, including
the label names and descriptions it publishes from its compiled label table; the release
tagger's tag refs) are outside this brief's "Outward writes" inventory and are not covered.

## Amendment (#2027, 2026-10-02, brief stays `implemented`)

Verify rows 2–9 and 11–13 are re-authored in version 3 so each one fails when the property it
names is false. As written in version 2 they could not: rows 2–9, 12 and 13 stated an "output
contains `--- PASS: …`" expectation the runner never checks, so each passed on the `go test`
exit status alone, and a `-run` selector that matches nothing (a renamed, deleted or not yet
written test) prints "no tests to run" and exits 0. Row 11 ran `statusgen --consumers` with no
base, which on merged main has no diff naming this brief and cannot check anything. Row 11's
property is narrower than the others' (no routing claim in the `consumers:` list is disproved);
what it can and cannot see is stated in its bullet below and in its Expect.

What changed, and what did not:

- Every `go test` row now captures the run, counts the anchored top-level `--- PASS: <name> (`
  line itself and prints one decidable line (`rc=0 pass=1` and its variants), and the closing
  assertion (the run's status, then a grep for each named `--- PASS:` line) fails the row unless
  the run succeeded and every named test passed; the printed line pins each count to one. `-count=1`
  defeats the test cache. The selectors and packages are the version 2 ones, anchored to the
  exact name where the row names one test; row 2 keeps its prefix selector, which runs every outbound test in the package.
- Row 5 also counts the `pushes != 0` assertion inside its test, the ZERO-pushes property its
  Expect names. Row 7 counts both structural tests. Row 8 reads the mutation harness's own
  healthy line and Totals line and requires the four named mutation classes to be CAUGHT.
- Row 11 judges the brief as it stands in the tree under test, against a fixed base: the parent
  of e7e9f35d3, the commit that first added this brief file (#1229). At that base the file does
  not exist, so no entry is inherited and six of the seven entries are judged — a `follow-up` must
  name a brief that exists and references back, a `fixed-here` path must exist and appear in the
  diff. The seventh, the `out-of-scope` entry, stays UNCHECKED: it names directories (`statusgen`,
  `tools/cellctl`) and the tool's `out-of-scope` check is not directory-aware, so an edit under
  `statusgen/` does not turn this row red. The command asserts the exact summary line as
  well as the exit status, so a run that judges nothing (every entry UNCHECKED, exit 0), a run
  that cannot reach the base, and a disproved entry are all red. It cannot see consumer wiring
  removed from the code: five entries route `follow-up desktools-v2/10` to this brief itself, and
  a self-route corroborates on the brief's own text. With the decorator removed from both
  `ResolveForge` returns, row 11 stays green while rows 2, 6, 7 and 8 turn red.
- Row 11's first version-3 form, a clone pinned at #1919's squash with a pinned base, read no
  part of the tree under test and printed the same all-UNCHECKED line for every state of the
  repository; review of #2044 showed it green on three planted false states. It is replaced by
  the form above.
- Row 13 counts the tests that actually RAN under an anchored selector. Its test,
  TestOutboundWindowsMachinePaths, and the drive-letter/UNC scanner extension it exercises
  belong to desktools-v2/12 (its Windows row). When this amendment was authored the test did
  not exist and the row printed `rc=0 run=0 pass=0` and was red, where version 2 passed it on
  "no tests to run". desktools-v2/12 has since landed the test (#2033), so the row now prints
  `rc=0 run=1 pass=1` and is green; a selector that matches nothing, or a skipped test, turns
  it red again. It is neither loosened nor dropped here.
- Rows 1 and 10, every Expect property, the Task, the `consumers:` list, and the Evidence below
  are unchanged. The 2026-10-01 Evidence was recorded against version 2 rows, and so is any
  other Evidence block whose commands are the version 2 ones, whichever order it lands in
  relative to this amendment: such a block is not evidence for the version 3 rows, which need
  a fresh run.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 2–9 and 11–13 were re-authored in
version 3 (#2027, see the Amendment above) so a missing selector or a false property turns the
row red; row 13 is red if its test (added in #2033) is missing or skipped. Row 11 needs `statusgen` on PATH
and history back to e7e9f35d3 (not a depth-1 clone); without either it is red, never green.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go vet ./internal/deskkit/ ./internal/forgeban/` | exit 0 |
| 2 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/deskkit/ -run 'TestOutbound' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestOutboundConformance (' <<<"$o"); echo "rc=$rc pass=$p"; test "$rc" = 0 && grep -q -e '^--- PASS: TestOutboundConformance (' <<<"$o"` | exit 0; output is `rc=0 pass=1`. The command counts the top-level `--- PASS: TestOutboundConformance (` line itself and fails unless the run succeeded AND that line appears once, so a `-run` selector matching nothing, a skipped test or a renamed test prints `pass=0` and the row is red. Every other `TestOutbound*` test in the package must pass too: the run's own status is part of the witness |
| 3 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./cmd/deskfile/ -run '^TestNewRefusesWithheldIdentifierOnPublicTarget$' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestNewRefusesWithheldIdentifierOnPublicTarget (' <<<"$o"); echo "rc=$rc pass=$p"; test "$rc" = 0 && grep -q -e '^--- PASS: TestNewRefusesWithheldIdentifierOnPublicTarget (' <<<"$o"` | exit 0; output is `rc=0 pass=1`: the refusal test ran and passed at top level. It asserts the fake forge filed nothing on the public target, the refusal names the withheld-identifier rule, and a written override reason does not take it through. The fail-first red run on the unfixed code (`deskfile new` FILES the issue, one recorded `FileIssue`) was the implementer's PR-body obligation; it is not part of this row's verdict |
| 4 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./cmd/deskfile/ -run '^TestNewPassesSameBodyOnPrivateTarget$' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestNewPassesSameBodyOnPrivateTarget (' <<<"$o"); echo "rc=$rc pass=$p"; test "$rc" = 0 && grep -q -e '^--- PASS: TestNewPassesSameBodyOnPrivateTarget (' <<<"$o"` | exit 0; output is `rc=0 pass=1`: the same text, private target, one `FileIssue` recorded (the test fails when the private target records no `FileIssue`) |
| 5 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./cmd/deskpr/ -run '^TestPushRefusesWithheldNameInAddedTestComment$' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestPushRefusesWithheldNameInAddedTestComment (' <<<"$o"); b=$(sed -n -e '/^func TestPushRefusesWithheldNameInAddedTestComment(/,/^}/p' tools/desk/cmd/deskpr/outbound_test.go); z=$(grep -c -e 'if pushes != 0 {' <<<"$b"); echo "rc=$rc pass=$p zero=$z"; test "$rc" = 0 && grep -q -e '^--- PASS: TestPushRefusesWithheldNameInAddedTestComment (' <<<"$o" && test "$z" = 1` | exit 0; output is `rc=0 pass=1 zero=1`: the test passed at top level AND its body still carries the `if pushes != 0 {` assertion that the push seam recorded ZERO pushes (`zero=` counts that assertion inside the test function, so deleting it turns the row red although the test would still pass). The fail-first run (the push proceeds on the unfixed code) was the implementer's PR-body obligation; it is not part of this row's verdict |
| 6 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/deskkit/ -run '^TestOutboundForgeWrapsEveryWriteMethod$' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestOutboundForgeWrapsEveryWriteMethod (' <<<"$o"); echo "rc=$rc pass=$p"; test "$rc" = 0 && grep -q -e '^--- PASS: TestOutboundForgeWrapsEveryWriteMethod (' <<<"$o"` | exit 0; output is `rc=0 pass=1` — the completeness layer, independent of the conformance fixtures |
| 7 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/forgeban/ ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -v 2>&1 && go test -count=1 -timeout 10m ./internal/forgeban/ -run '^TestNoBackendTypeOutsideDeskkit$' -v 2>&1); rc=$?; s=$(grep -c -e '^--- PASS: TestForgeSingleConstructionSite (' <<<"$o"); n=$(grep -c -e '^--- PASS: TestNoBackendTypeOutsideDeskkit (' <<<"$o"); echo "rc=$rc site=$s ban=$n"; test "$rc" = 0 && grep -q -e '^--- PASS: TestForgeSingleConstructionSite (' <<<"$o" && grep -q -e '^--- PASS: TestNoBackendTypeOutsideDeskkit (' <<<"$o"` | exit 0; output is `rc=0 site=1 ban=1`: BOTH structural tests ran and passed at top level — one construction site, and no cmd package can name a backend type to build or unwrap one. Either test missing, skipped or renamed prints a zero and the row is red |
| 8 | check +mutation | `o=$(cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/deskkit/outbound-mutations.json 2>&1); h=$(grep -c -x -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' <<<"$o"); q=$(grep -c -e '^  CAUGHT  *the decorator removed from ResolveForge' -e '^  CAUGHT  *one text-carrying method dropped from the decorator' -e '^  CAUGHT  *the visibility test inverted' -e '^  CAUGHT  *the e-mail allow-list emptied' <<<"$o"); t=$(sed -n -e 's/^Totals: [0-9]* caught, \([0-9]*\) NOT CAUGHT, \([0-9]*\) could-not-mutate\.$/survived=\1 couldnot=\2/p' <<<"$o"); echo "healthy=$h required=$q $t"; test "$h" = 1 && test "$q" = 5 && test "$t" = 'survived=0 couldnot=0'` | exit 0; output is `healthy=1 required=5 survived=0 couldnot=0`. `healthy=1`: the harness printed its own healthy line (baseline green, positive control caught); a broken harness prints no verdicts. `survived=` and `couldnot=` are read from the harness's Totals line, so a mutation that survived, one that could not be applied, or a missing Totals line breaks the match. `required=5` counts the CAUGHT lines of the mutations this row requires `internal/deskkit/outbound-mutations.json` to carry: the decorator removed from `ResolveForge`'s return (two mutants, the GitHub and the GitLab return), one text-carrying method dropped from the decorator, the visibility test inverted, and the e-mail allow-list emptied — the fail-first evidence a reviewer re-runs. `go run` flattens the exit status, so the row asserts on the printed lines and the closing `test` gives the row its status |
| 9 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/deskkit/ -run '^TestOverrideAuditRowHoldsDigestNotText$' -v 2>&1); rc=$?; p=$(grep -c -e '^--- PASS: TestOverrideAuditRowHoldsDigestNotText (' <<<"$o"); echo "rc=$rc pass=$p"; test "$rc" = 0 && grep -q -e '^--- PASS: TestOverrideAuditRowHoldsDigestNotText (' <<<"$o"` | exit 0; output is `rc=0 pass=1` |
| 10 | check | `grep -rn --include='*.go' --exclude='*_test.go' -e 'deskkit.ScanSurface' -e 'deskkit.BodyCheck' -e 'deskkit.SelfContainCheck' tools/desk/cmd/deskfile tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskevidence; test $? -eq 1` | exit 0 and no line printed — the per-verb scan calls are GONE from the verbs whose writes all cross the decorator (the removal, not just the addition). `deskpr` is excluded by design: its `--check` pre-flight is the recorded second caller |
| 11 | check | `o=$(statusgen --consumers --root . --brief desktools-v2/10 --base e7e9f35d3~1); rc=$?; l=$(grep -e '^summary: ' <<<"$o"); echo "rc=$rc $l"; test "$rc" = 0 && test "$l" = 'summary: 6 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'` | exit 0; output is `rc=0 summary: 6 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The tool judges this brief's `consumers:` list as it stands in the tree under test, against the diff from the parent of e7e9f35d3 (the commit that first added this file, #1229), so none is inherited and six of the seven entries are judged. The row is red when any routing claim is DISPROVED (a `follow-up` naming a brief that does not exist, a `fixed-here` path that is absent or not in the diff: the tool exits 1 and the summary shows `1 disproved` or more), when the tool cannot run (no `statusgen`, history too shallow to reach the base: a nonzero `rc`), and when it judges nothing: a summary of `0 corroborated, 0 disproved, 7 unchecked` exits 0 from the tool but is NOT a pass, and the closing `test` on the exact summary line turns it red. Of the six corroborations, five are entries that route `follow-up desktools-v2/10` to this brief itself and corroborate on its own text; the sixth is desktools-v2/11 referencing back. The one UNCHECKED entry is the `out-of-scope` exclusion of statusgen and tools/cellctl, whose reason is the reviewer's call; it names directories and the tool's `out-of-scope` check (an edited site contradicts it) is not directory-aware, so editing a file under statusgen/ does NOT turn this row red. LIMIT: this row cannot see consumer wiring removed from the code. With the decorator removed from both `ResolveForge` returns this row stays green; rows 2, 6, 7 and 8 turn red |
| 12 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/deskkit/ -run '^TestOutboundGitLab' -v 2>&1); rc=$?; a=$(grep -c -e '^--- PASS: TestOutboundGitLabTypedNotes (' <<<"$o"); b=$(grep -c -e '^--- PASS: TestOutboundGitLabNoReply (' <<<"$o"); echo "rc=$rc notes=$a noreply=$b"; test "$rc" = 0 && grep -q -e '^--- PASS: TestOutboundGitLabTypedNotes (' <<<"$o" && grep -q -e '^--- PASS: TestOutboundGitLabNoReply (' <<<"$o"` | exit 0; output is `rc=0 notes=1 noreply=1`: both named top-level tests ran and passed (a missing, skipped or renamed test prints a zero and the row is red); GitLab.com no-reply and reserved documentation-host shapes (generic self-managed recognition remains pending scope on issue 1836), internal target and explicit MR note route; refusal sends no request (desktools-v2/12 GitLab row) |
| 13 | check | `o=$(cd tools/desk && go test -count=1 -timeout 10m ./internal/deskkit/ -run '^TestOutboundWindowsMachinePaths$' -v 2>&1); rc=$?; r=$(grep -c -x -e '=== RUN   TestOutboundWindowsMachinePaths' <<<"$o"); p=$(grep -c -e '^--- PASS: TestOutboundWindowsMachinePaths (' <<<"$o"); echo "rc=$rc run=$r pass=$p"; test "$rc" = 0 && test "$r" = 1 && grep -q -e '^--- PASS: TestOutboundWindowsMachinePaths (' <<<"$o"` | exit 0; output is `rc=0 run=1 pass=1`: the named top-level test actually RAN once and passed; public drive-letter and UNC machine paths refuse using IsAbsFor. A selector that matches nothing prints `run=0 pass=0` and the row is red, never green on "no tests to run". The test and the scanner extension it exercises landed with #2033 (desktools-v2/12's Windows row), so the row is green now; it is red again if the test is deleted or renamed (the selector then matches nothing) or if it is skipped. Scope of the Windows machine-path rule is still pending owner clarification on issue 1836 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Verify run — 2026-10-01 — post-merge, main @ 4b1f8fc8bfaf (implementing merge b69252cc8e30, #1919)

What moved since the last run: nothing — this is the first verify pass; the brief carried no earlier Evidence block. Run read-only in a detached worktree cut from `refs/remotes/origin/main` (4b1f8fc8bfaf6520e26fe0659f944d22e43e3aa5, which contains b69252cc8e306cab797778b0b19c1b1b59b9fc8e), go1.27.1 darwin/arm64, `KUBECONFIG=/dev/null`, no network writes. All commands below run from the repository root.

**This brief is `gate: human` with `risk: sensitive-data: yes`. This run is EVIDENCE ONLY: the verifier does not sign it off and does not flip its status. The human closes the gate.**

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/ ./internal/forgeban/` | exit 0 | Verify row 1. exit 0; no output from build or vet. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run 'TestOutbound' -v` | literal line `--- PASS: TestOutboundConformance` | Verify row 2. exit 0; `--- PASS: TestOutboundConformance (0.86s)` present; 164 conformance subtests PASS covering rows C1-C11 on every target (C12 is asserted inside every refused subtest and once over all composed bodies); 0 FAIL, 0 SKIP; `ok ... internal/deskkit 1.393s`. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewRefusesWithheldIdentifierOnPublicTarget -v` | `--- PASS:` line for that test | Verify row 3. exit 0; the `--- PASS:` line for the test named in Command is present (0.13s); 0 SKIP; `ok ... cmd/deskfile`. The fail-first red on unfixed code is PR-body evidence outside the merged tree; this row asserts the merged behaviour. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewPassesSameBodyOnPrivateTarget -v` | `--- PASS:` line for that test | Verify row 4. exit 0; the `--- PASS:` line for the test named in Command is present (0.07s); 0 SKIP; `ok ... cmd/deskfile`. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskpr/ -run TestPushRefusesWithheldNameInAddedTestComment -v` | `--- PASS:` line for that test; zero pushes recorded | Verify row 5. exit 0; the `--- PASS:` line for the test named in Command is present (0.44s); 0 SKIP; `ok ... cmd/deskpr`. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundForgeWrapsEveryWriteMethod -v` | `--- PASS:` line for that test | Verify row 6. exit 0; the `--- PASS:` line is present with 11 subtests PASS: refuses-before-delegate for each of the nine text-carrying methods (ApplyLabels, CreateDraftChange, EditChange, EditComment, FileIssue, PostComment, PostCommentTyped, PostReview, WriteFile) plus ForgeFor-wraps-github and ForgeFor-wraps-gitlab; 0 SKIP. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test -timeout 10m ./internal/forgeban/ ./internal/deskkit/ -run 'Test(ForgeSingleConstructionSite)$' -v && go test ./internal/forgeban/ -run TestNoBackendTypeOutsideDeskkit -v` | BOTH `--- PASS:` lines | Verify row 7. exit 0; the single-construction-site `--- PASS:` line is present (deskkit, 0.15s; the forgeban package matches no test for that selector, as expected); the backend-type ban `--- PASS:` line is present (0.15s) with 14 planted-red and 5 clean subtests PASS; 0 SKIP. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/deskkit/outbound-mutations.json` | Totals: every mutation killed, none survived; spec carries the four named mutants | Verify row 8. exit 0 (52s); `Harness healthy: baseline GREEN, positive control CAUGHT.`; `Totals: 14 caught, 0 NOT CAUGHT, 0 could-not-mutate.` The spec carries all four required mutants, each CAUGHT: decorator removed from the ResolveForge return (GitHub and GitLab arms), a text-carrying method dropped (PostReview, and ApplyLabels), visibility test inverted, e-mail allow-list emptied. It also carries "withheld register made overridable", CAUGHT. Worktree clean afterwards. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestOverrideAuditRowHoldsDigestNotText -v` | `--- PASS:` line for that test | Verify row 9. exit 0; the `--- PASS:` line for the test named in Command is present (0.01s); 0 SKIP. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `grep -rn --include='*.go' --exclude='*_test.go' -e 'deskkit.ScanSurface' -e 'deskkit.BodyCheck' -e 'deskkit.SelfContainCheck' tools/desk/cmd/deskfile tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskevidence; test $? -eq 1` | exit 0, nothing printed | Verify row 10. exit 0; grep printed no line, so the per-verb scan calls are absent from the four verbs. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11a | `statusgen --consumers --root "$(git rev-parse --show-toplevel)"` | exit 0; no routing claim disproved | Verify row 11, as authored (`--root .` written as the absolute toplevel, because the write guard refuses a relative root). exit 0, but the result is vacuous: `consumers: no brief files in the diff against 4b1f8fc8bfaf… — nothing to corroborate`. On merged main HEAD equals the base, so this form cannot look at the brief. Re-run as 11b. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11b | `statusgen --consumers --root "$(git rev-parse --show-toplevel)" --base "$(git rev-parse b69252cc8e30^1)" --brief desktools-v2/10` | exit 0; no routing claim disproved | Verify row 11, corrected to diff against the implementing merge's first parent (036a5c1c39ef). exit 0; `summary: 0 corroborated, 0 disproved, 7 unchecked`. The tool leaves all 7 UNCHECKED because the PR did not edit the consumers block. Checked by hand against that diff's file list, all 7 hold. outbound.go, outboundforge.go, personaldata.go (and outboundpush.go) are new in internal/deskkit. forgeresolve.go wraps both returns. Each of the ten listed verbs is touched. The deskpushguard hook gains outbound.go. forgeban gains backendtype.go. statusgen and tools/cellctl are untouched. The callout stays for brief 11. No claim is disproved. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Risk-bearing value enumeration.** The trigger fires on `sensitive-data: yes`. It covers every literal inside the outbound-write check that the merge b69252cc8e30 introduces or changes, plus the policy values it relies on, all at main 4b1f8fc8bfaf. Paths are under tools/desk/internal/deskkit/.

| Rank | Literal @ file:line | If wrong | Undo by edit + redeploy? |
|---|---|---|---|
| 1 | overridable() = `f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier` @ outbound.go:212, with rule ids `"voice.ruling-claim"` @ outbound.go:63 and `"withheld.identifier"` @ outbound.go:74 | a written reason publishes a withheld identifier to a public repository | no — a publication cannot be recalled |
| 2 | outboundPublicLayers = SelfContainApplies @ outbound.go:218 → `RepoVisibility(repo) != VisibilityPrivate` @ config.go:198, gated by `!EffectiveConfig().Configured()` → false @ selfcontain.go:201-202 | public layers bypassed on a public or unknown target | no (disclosure) |
| 3 | rePhoneE164 = `\+[0-9](?:[ .()-]?[0-9]){5,20}` @ personaldata.go:34; digit bounds `n < 8 \|\| n > 15` @ personaldata.go:122; rePhoneGrouped @ personaldata.go:37; the `+`-preceded skip @ personaldata.go:129 | an international phone number reaches any target | no (personal data disclosed) |
| 4 | emailAllowedDomains = users.noreply.github.com, noreply.gitlab.com, example.com, example.org, example.net @ personaldata.go:42-48; emailAllowedTLDs = example, invalid, test @ personaldata.go:51; emailAllowedExact = the GitHub no-reply address @ personaldata.go:54; roster bot addresses @ personaldata.go:83-86 | a personal address passes as allowed | no (disclosure) |
| 5 | pass-through classification: OpenMergeHold, RunWorkflow, ApproveGate are not text-checked, beyond the brief's five pass-through families @ outboundforge.go:18-21 | text carried by a workflow input escapes the check | no, if a workflow publishes it |
| 6 | withheldMarker = `"withheld register identifier"` @ scanoverride.go:158, string-coupled to the refusal category at selfcontain.go:367 and :377 | the pre-flight (`--check`) path would let the override through. The seam still refuses. | yes |
| 7 | minOverrideReason = 12 @ scanoverride.go:54 (unchanged by this merge) | weaker forensic reasons | yes — reversible knob, ranks last |

Derivations, from the brief text and DR-desktools-v2-10 (ruling #1319, option 1), not from the code:

- **Rank 1 is DERIVED.** DR-desktools-v2-10 accepts that credential, personal-data and self-containment refusals are overridable. A withheld identifier on a public or unknown target is not overridable, and neither is a ruling claim. outbound.go:212 encodes exactly that. The "withheld register made overridable" mutant is CAUGHT (row 8). A verifier probe, a test file added through `go test -overlay` so the tree was never written, ran HandleScanRefusal on a real withheld refusal with a 25-character reason. Result: still refused, "does not apply to withheld.identifier". The pre-flight path therefore matches the ruling too.
- **Rank 2 is DERIVED.** The brief says the public layers run when VisibilityRiskClassed is true, for public AND unknown targets, with no network read. config.go:198 is the fail-closed `!= VisibilityPrivate`. The probe confirms an unlisted target is refused for a withheld identifier and a private target is not. The configured-roster precondition at selfcontain.go:201-202 predates this merge (identical at b69252cc8e30^1), so it is "as today". It does mean that on an install with no configured roster the public layers are inactive altogether.
- **Rank 3, digit bounds 8 and 15: DERIVED.** The brief table says "`+` and 8-15 digits", and 15 is the E.164 maximum.
- **Rank 3, separator rule: NOT DERIVED, and contradicted by the brief.** The brief says "with optional separators". `[ .()-]?` admits at most ONE separator character between two digits. A probe ran OutboundCheck through an overlay test on both a public and a private target. Input: the C7 fixture number rewritten in the common form with the area code in parentheses (`+`, country code, space, `(`three digits`)`, space, three digits, hyphen, four digits). Result: err=nil and no notice on both targets. The same happens for a UK-style number with a parenthesised trunk zero, and for a number with double spaces. The ambiguous-shape NOTICE is also suppressed: the grouped regex's leftmost match starts on the digit right after `+` and is discarded at personaldata.go:129, and Go's non-overlapping scan does not retry at the parenthesis. The C7 fixture form, with single spaces only, is refused as specified.
- **Rank 4 is DERIVED.** The list matches the brief's compiled allow-list entry for entry: forge no-reply domains, the reserved example domains and TLDs, and the roster's bot addresses. Suffix matching covers GitLab's users.noreply subdomain. `.localhost` (also RFC 6761) is absent, consistent with the brief's "Nothing else". The "allow-list emptied" mutant is CAUGHT.
- **Rank 5 is NAMED, NOT DERIVED.** The brief and the DR list five pass-through families. The decorator also passes OpenMergeHold, RunWorkflow (whose Inputs are a free-text map) and ApproveGate straight through. A code comment justifies this, and the completeness test records it. No brief or DR line records the decision.
- **Rank 6 is DERIVED** as consistent: the probe shows the marker matches the live refusal text. No unit test exercises the withheld branch of HandleScanRefusal directly.
- **Rank 7 is DERIVED** from the brief ("12 characters minimum").

RISK-VALUE: DERIVED — overridable() = `f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier` @ tools/desk/internal/deskkit/outbound.go:212 — matches the #1319 option-1 ruling as recorded in DR-desktools-v2-10; mutant CAUGHT; the pre-flight branch was probed and still refuses.
RISK-VALUE: DERIVED — `RepoVisibility(repo) != VisibilityPrivate` @ tools/desk/internal/deskkit/config.go:198 (via outbound.go:218) — the brief's "public AND unknown" key, fail-closed, no network read.
RISK-VALUE: DERIVED — emailAllowedDomains / emailAllowedTLDs / emailAllowedExact @ tools/desk/internal/deskkit/personaldata.go:42-54 — entry-for-entry the brief's compiled allow-list.
RISK-VALUE: NAMED, NOT DERIVED — rePhoneE164 = `\+[0-9](?:[ .()-]?[0-9]){5,20}` @ tools/desk/internal/deskkit/personaldata.go:34 — the 8-15 digit bound is derived, but the one-separator-per-gap rule contradicts the brief's "optional separators": an international number with a parenthesised area code passes with no refusal and no notice on any target (probe). Missing: a pattern admitting grouped separators, plus a conformance row for that shape.
RISK-VALUE: NAMED, NOT DERIVED — pass-through set {OpenMergeHold, RunWorkflow, ApproveGate} @ tools/desk/internal/deskkit/outboundforge.go:18-21 — wider than the brief's five pass-through families; no brief or DR line records it.

Scope findings (verified work that maps to no Verify row):
- F1, a defect in new coverage, not a regression: the pii.phone false negative described above. No check of this kind existed before #1919, so nothing was dropped.
- F2: the merge also changes tools/desk/cmd/deskroster (liveness.go now resolves its fetcher through deskkit.ForgeAccountFetcher) and tools/desk/internal/gitcore (new CommitMessage). The brief's consumers block names neither. This is an omitted consumer, which the consumers gate cannot see by design.
- F3: the HandleScanRefusal withheld branch (scanoverride.go:158-200) has no dedicated unit test. A probe shows its behaviour is correct today.

rows_passed=11 rows_total=11

VERIFY: PASS — every Verify row passes on merged main 4b1f8fc8bfaf. This is EVIDENCE ONLY for a `gate: human`, `sensitive-data: yes` item: the status is left where it is for the human to close, and the pii.phone separator gap (F1) is routed to that human as a NAMED, NOT DERIVED risk value.

### Non-implementer verifier run — VERIFY: FAIL — EVIDENCE ONLY (gate: human) — 2026-10-02 claude-opus-5-5-verifier

Merged SHA run on: main @ cd435f006a68f4ad56d316e38be0f8751a5f933c (contains the implementing merge b69252cc8e30, #1919, and the later brief change f78a1c5db18e, #1997). Read-only in a detached worktree cut from the fetched main, go1.27.1 darwin/arm64, statusgen v1.0.30, `KUBECONFIG=/dev/null`, every `go test` under a throwaway HOME, no forge writes. All commands run from the repository root.

**This brief is `gate: human` with `risk: sensitive-data: yes`. This run is EVIDENCE ONLY: a model does not sign it off and does not flip its status. The human closes the gate.**

**What moved since the last run (2026-10-01 @ 4b1f8fc8bfaf, 11 rows, PASS).** Commit f78a1c5db18e (#1997) changed this brief in two places: frontmatter `version: 1` became `version: 2`, and the Verify table gained rows 12 and 13 (the GitLab row and the Windows machine-path row carried over from desktools-v2/12). Rows 1-11 are textually unchanged. The same commit added the test file platform_outbound_test.go (the two tests row 12 selects) and pathabs.go (IsAbsFor) under tools/desk/internal/deskkit. It did not add the test row 13 selects, and row 13's own Expect cell says so. The production files of the outbound check (outbound.go, outboundforge.go, outboundpush.go, personaldata.go, selfcontain.go, scanoverride.go, config.go, outbound-mutations.json) have no commit between 4b1f8fc8bfaf and this head.

**Expectations written from the brief text before running.** Rows 1-11: as on 2026-10-01 (build and vet clean; the conformance line present; the two issue-filing fixtures; the push fixture with zero pushes; the completeness layer; the structural layer; every mutant killed; the digest-only audit row; no per-verb scan call left; consumers exit 0). Row 12: two top-level tests pass, covering a no-reply address on the hosted GitLab domain and on a reserved documentation host, a note on an internal target routed to the merge-request notes endpoint, and a refusal that sends zero requests. Row 13: expected to FAIL as authored — the Expect cell demands a named `--- PASS:` line and states "a missing selector is failure", while also stating that the test and the scanner extension "are not supplied".

| # | Command | Expect | Observed (exit code + real output) | Date | Runner |
|---|---|---|---|---|---|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/ ./internal/forgeban/` | exit 0 | Verify row 1. exit 0; build and vet printed nothing. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run 'TestOutbound' -v` | literal line `--- PASS: TestOutboundConformance` | Verify row 2. exit 0; `--- PASS: TestOutboundConformance (2.76s)` present. Eight top-level tests matched and all PASS (NumberHintOnDecorator, NumberHintRow, WritesCarryNumber, Conformance, ForgeWrapsEveryWriteMethod, WriteFileChecksBranch, GitLabTypedNotes, GitLabNoReply); 196 PASS lines, 0 FAIL, 0 SKIP; `ok ... internal/deskkit 3.512s`. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewRefusesWithheldIdentifierOnPublicTarget -v` | `--- PASS:` line for that test | Verify row 3. exit 0; `--- PASS: TestNewRefusesWithheldIdentifierOnPublicTarget (0.21s)`; `ok ... cmd/deskfile 1.017s`. The fail-first red on the unfixed code is PR-body evidence outside the merged tree; this row asserts the merged behaviour. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewPassesSameBodyOnPrivateTarget -v` | `--- PASS:` line for that test | Verify row 4. exit 0; `--- PASS: TestNewPassesSameBodyOnPrivateTarget (0.13s)`; `ok ... cmd/deskfile 0.942s`. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskpr/ -run TestPushRefusesWithheldNameInAddedTestComment -v` | `--- PASS:` line for that test; zero pushes recorded | Verify row 5. exit 0; `--- PASS: TestPushRefusesWithheldNameInAddedTestComment (0.48s)`; `ok ... cmd/deskpr 1.432s`. The test's stderr shows the refusal `refused: selfcontain.private-repository-name at widget_test.go:3` for a file write to a public target. Read in the test source: it counts push invocations and fails on any count other than zero, and fails if a draft change was opened. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundForgeWrapsEveryWriteMethod -v` | `--- PASS:` line for that test | Verify row 6. exit 0; `--- PASS: TestOutboundForgeWrapsEveryWriteMethod (0.01s)` with 11 subtests PASS (12 PASS lines), 0 FAIL, 0 SKIP. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test -timeout 10m ./internal/forgeban/ ./internal/deskkit/ -run 'Test(ForgeSingleConstructionSite)$' -v && go test ./internal/forgeban/ -run TestNoBackendTypeOutsideDeskkit -v` | BOTH `--- PASS:` lines | Verify row 7. exit 0; `--- PASS: TestForgeSingleConstructionSite (0.21s)` (deskkit; the forgeban package prints `[no tests to run]` for that selector, which is expected — the test lives in deskkit) and `--- PASS: TestNoBackendTypeOutsideDeskkit (0.18s)` with its subtests; 21 PASS lines, 0 FAIL, 0 SKIP. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/deskkit/outbound-mutations.json` | Totals: every mutation killed, none survived; spec carries the four named mutants | Verify row 8. Run in a scratch copy of the tree at this SHA, because the harness edits source in place. exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`; `Totals: 14 caught, 0 NOT CAUGHT, 0 could-not-mutate.` All four required mutants are in the spec and CAUGHT: decorator removed from the ResolveForge return (GitHub arm and GitLab arm), a text-carrying method dropped (PostReview, and ApplyLabels), visibility test inverted, e-mail allow-list emptied. Also CAUGHT: withheld register made overridable, audit row carrying the refusal text, push path not reading commit messages, push path not reading added lines, and four item-number mutants. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestOverrideAuditRowHoldsDigestNotText -v` | `--- PASS:` line for that test | Verify row 9. exit 0; `--- PASS: TestOverrideAuditRowHoldsDigestNotText (0.01s)`; `ok ... internal/deskkit 0.477s`. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `grep -rn --include='*.go' --exclude='*_test.go' -e 'deskkit.ScanSurface' -e 'deskkit.BodyCheck' -e 'deskkit.SelfContainCheck' tools/desk/cmd/deskfile tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskevidence; test $? -eq 1` | exit 0, nothing printed | Verify row 10. exit 0; grep printed no line. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `statusgen --consumers --root .` | exit 0; no routing claim disproved | Verify row 11. exit 0, as authored. The result is vacuous on merged main: `consumers: no brief files in the diff against cd435f006a68… — nothing to corroborate`. Supplementary runs with `--base` set to the first parent of #1919 and to the parent of #1997, `--brief desktools-v2/10`: both exit 0 with `summary: 0 corroborated, 0 disproved, 7 unchecked`. No claim is disproved; the tool judged none of the seven. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestOutboundGitLab' -v` | named `--- PASS:` lines; both top-level tests PASS | Verify row 12. exit 0; `--- PASS: TestOutboundGitLabTypedNotes (0.01s)` with subtests for the public and the internal target, and `--- PASS: TestOutboundGitLabNoReply (0.00s)` with subtests `gitlab.com` and `gitlab.example`; 0 FAIL, 0 SKIP; `ok ... internal/deskkit 0.499s`. Read in the test source: the public-target note is refused with zero requests sent; the internal-target note sends exactly one request to the merge-request notes path. The documentation-host subtest passes through the reserved `.example` top-level name, not through any self-managed host recognition, as the Expect cell says. | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestOutboundWindowsMachinePaths$' -v` | named `--- PASS:` line (a missing selector is failure); drive-letter and UNC machine paths refuse on a public target | Verify row 13. FAIL against its own Expect. exit 0, but the whole output is `testing: warning: no tests to run` / `PASS` / `ok ... internal/deskkit 0.271s [no tests to run]`. No `--- PASS:` line exists. No function of that name exists anywhere under tools/desk. A probe shows the behaviour is also absent, not only the test (finding F1). | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

**Witness dry-run.** `statusgen verifyrun --dry-run --brief docs/streams/desktools-v2/brief-10-one-outbound-write-check.md`, run from the worktree root under a throwaway HOME: exit 0; 13 rows, 13 `pass exit=0`, 0 fail, 0 in the third state; closing line `--dry-run: the table above was NOT written to` the brief. Output hashes: row 1 `sha256:e3b0c44298fc`, 2 `28f2bce05723`, 3 `84ccefdc231a`, 4 `d350840cfdbe`, 5 `ed3ab8d1354d`, 6 `e8b268b029f5`, 7 `4c5861ee225b`, 8 `5b7f3887757d`, 9 `1d2fcef6f1b1`, 10 `e3b0c44298fc`, 11 `1312cf609631`, 12 `6b77537f8be3`, 13 `91bb6e07c486`. For rows 2-9, 12 and 13 the tool printed `expect: exit-status only (nothing else in the Expect cell is machine-decidable — the output hash is the record a reviewer weighs)`. The witness therefore reports row 13 as pass while the hand run shows it selected no test: the witness count (13/13) and the hand count (12/13) disagree, and the hand count is the correct one. The dry-run executes row 8 in the worktree itself; the harness restored every file and `git status --short` was empty afterwards.

**Tags.** Row 8 is the only tagged row (`+mutation`); honoured by the scratch-copy run above. No row carries `+flow`, `+dereference` or `+neighbour`. Probes below were added through `go test -overlay`, so the tree was never written.

**Findings.**

- F1 (row 13, behaviour absent). On a public target the outbound check does not refuse a Windows drive-letter path written with backslashes, nor a UNC path. Probe: a comment body carrying each shape, sent through the checking decorator to the public fixture target, returned err=nil and the recording fake saw one delegate call each. The same drive-letter path written with forward slashes is refused, but only because it happens to contain the unix home root the existing pattern already matches. The scanner's machine-path pattern (tools/desk/internal/deskkit/selfcontain.go:117) lists unix roots only. IsAbsFor (tools/desk/internal/deskkit/pathabs.go:7) has one production caller, in the cell launcher's path check; the outbound scanner does not call it. Row 13's Expect ("refuse using IsAbsFor") describes work that is not on main.
- F2 (row 13, false-green row shape). The row's command exits 0 when its selector matches nothing, and the witness can only judge exit status. A written witness for this table would record row 13 as pass. The row needs either the delivered test plus a command that asserts on the PASS line, or removal until its scope is ruled on #1836.
- F3 (rows 2-7, 9, 12, same shape, currently true greens). Each Expect asserts on a `--- PASS:` line that the command itself does not check; a renamed or deleted test would leave these rows green in the witness. Row 8 has the same property through `go run`. Making each command self-asserting (pipe the output through a fixed-string match on the named line) would close it.
- F4 (row 11, vacuous on merged main). As authored the row diffs the head against itself and corroborates nothing; with an explicit base the tool leaves all seven claims unjudged. Unchanged from the 2026-10-01 run.
- F5 (carried, still present). The international phone pattern admits one separator character between digits. Probe at this head: the conformance fixture number is refused on public and private targets; the same number with the area code in parentheses, a number with a parenthesised trunk zero, and a number with doubled spaces all pass with no refusal on both targets.
- F6 (stale witness). The committed Evidence block is the 2026-10-01 run over 11 rows at 4b1f8fc8bfaf. It predates rows 12 and 13 and matches the current 13-row table for no row at the current head.
- F7 (carried). The pass-through set in the decorator is wider than the brief's five families (it adds OpenMergeHold, RunWorkflow, ApproveGate); the decision is recorded in a code comment and the completeness test, not in the brief or the design record.

**Risk-bearing value enumeration.** Trigger: `sensitive-data: yes`. Scope: every literal the outbound check introduced in #1919, plus what #1997 added for rows 12 and 13. The #1919 production files are unchanged since the 2026-10-01 enumeration; each line number below was re-read at this head. Paths are under tools/desk/internal/deskkit/.

| Rank | Literal @ file:line | If wrong | Undo by edit + redeploy? |
|---|---|---|---|
| 1 | overridable() = `f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier` @ outbound.go:212; rule ids `"voice.ruling-claim"` @ outbound.go:63, `"withheld.identifier"` @ outbound.go:74 | a written reason publishes a withheld identifier to a public repository | no |
| 2 | outboundPublicLayers = SelfContainApplies @ outbound.go:218, resolving to `RepoVisibility(repo) != VisibilityPrivate` @ config.go:198, behind `!EffectiveConfig().Configured()` returning false @ selfcontain.go:201-202 | public layers skipped on a public or unstated target | no |
| 3 | reAbsMachinePath roots = the two unix home roots, two private temp roots and one temp worktree prefix @ selfcontain.go:117; no drive-letter or UNC arm | a Windows machine path reaches a public target | no |
| 4 | rePhoneE164 = `\+[0-9](?:[ .()-]?[0-9]){5,20}` @ personaldata.go:34; digit bounds `n < 8 \|\| n > 15` @ personaldata.go:122; rePhoneGrouped @ personaldata.go:37; the `+`-preceded skip @ personaldata.go:129 | an international phone number reaches any target | no |
| 5 | emailAllowedDomains (two forge no-reply domains, three reserved example domains) @ personaldata.go:42-48; emailAllowedTLDs = example, invalid, test @ personaldata.go:51; emailAllowedExact @ personaldata.go:54; roster bot addresses @ personaldata.go:83-86 | a personal address passes as allowed | no |
| 6 | pass-through classification adding OpenMergeHold, RunWorkflow, ApproveGate @ outboundforge.go:18-21 | text carried by a workflow input leaves without a check | no, if a workflow publishes it |
| 7 | IsAbsFor windows arm: `len(p) >= 3`, letter, `:`, separator; or two leading separators with `len(p) > 2` @ pathabs.go:13-16 | not reached by the outbound check at all today | yes |
| 8 | withheldMarker = `"withheld register identifier"` @ scanoverride.go:158 | pre-flight path would accept the override; the seam still refuses | yes |
| 9 | minOverrideReason = 12 @ scanoverride.go:54 | weaker forensic reasons | yes — reversible knob, ranks last |

RISK-VALUE: DERIVED — overridable() = `f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier` @ tools/desk/internal/deskkit/outbound.go:212 — the option-1 ruling on #1319 recorded in this brief: credential, personal-data and self-containment refusals overridable, withheld identifier and ruling claim not. The mutant that drops the second clause is CAUGHT (row 8).
RISK-VALUE: DERIVED — `RepoVisibility(repo) != VisibilityPrivate` @ tools/desk/internal/deskkit/config.go:198 (via outbound.go:218) — the brief's "public AND unknown" key, fail-closed, no network read; the inverted-visibility mutant is CAUGHT. Note the roster-configured precondition at selfcontain.go:201-202: with no configured roster the public layers do not run at all.
RISK-VALUE: NAMED, NOT DERIVED — reAbsMachinePath root list @ tools/desk/internal/deskkit/selfcontain.go:117 — row 13 of the current table requires drive-letter and UNC machine paths to refuse on a public target; the list has no such arm and the probe shows both shapes pass. Missing: the scanner extension and its test, pending the scope ruling on #1836.
RISK-VALUE: NAMED, NOT DERIVED — rePhoneE164 = `\+[0-9](?:[ .()-]?[0-9]){5,20}` @ tools/desk/internal/deskkit/personaldata.go:34 — the 8-15 digit bound (personaldata.go:122) is derived from the brief's rule table and the E.164 maximum, but the one-separator-per-gap rule contradicts the brief's "optional separators": a number with a parenthesised area code passes on every target (probe, F5). Missing: a pattern admitting grouped separators and a conformance row for that shape.
RISK-VALUE: DERIVED — emailAllowedDomains / emailAllowedTLDs / emailAllowedExact @ tools/desk/internal/deskkit/personaldata.go:42-54 — entry for entry the brief's compiled allow-list ("Nothing else"); the emptied-list mutant is CAUGHT; row 12 confirms the hosted-GitLab no-reply shape by suffix match.
RISK-VALUE: NAMED, NOT DERIVED — pass-through set {OpenMergeHold, RunWorkflow, ApproveGate} @ tools/desk/internal/deskkit/outboundforge.go:18-21 — wider than the brief's five pass-through families; no brief or design-record line states it, so there is nothing to derive it from.

**What is owed before the human sign-off can be the only thing left.** (a) Row 13: deliver the Windows machine-path refusal and its test, or have the owner rule on #1836 that the row leaves this brief. (b) Re-author rows 2-9, 12 and 13 so the command asserts what the Expect cell asserts. (c) After (a) and (b), a fresh witness over the 13-row table at the then-current head, replacing the stale 11-row block.

rows_passed=12 rows_total=13

VERIFY: FAIL

### 2026-10-03 desk dispatch — 13/13 version 3 Verify rows pass on main @ b3677d6da493; Evidence only (gate: human, sensitive-data)

Merged SHA run on: main @ b3677d6da493 (the forge's main head at run time is 829185322 and differs from it only in STATUS.md). The tree contains the implementing merge b69252cc8e30 (#1919), the Windows machine-path work (#2033), the brief-frontmatter id exemption (#2024) and the version 3 Verify rows (#2044). Read-only in a detached worktree cut from the fetched main, go1.27.1 darwin/arm64, statusgen v1.0.31, KUBECONFIG=/dev/null, every command under a throwaway HOME, no forge writes. The worktree was clean (git status empty) after every run, including the mutation harness.

**This brief is gate: human with risk: sensitive-data: yes. This run is EVIDENCE ONLY: a model does not sign it off and does not flip its status. The README row stays implemented and the human closes the gate (decision issue #1912).**

**What moved since the last verdict (2026-10-02 @ cd435f006a68, 12/13, FAIL).** (1) #2033 landed TestOutboundWindowsMachinePaths and the Windows drive-letter and UNC arms of the machine-path scan, so the earlier row 13 failure and its "behaviour absent" finding are closed. (2) #2044 re-authored rows 2-9 and 11-13 as version 3: each command now asserts its own named PASS line, and row 11 judges against a fixed base. This closes the earlier false-green and vacuous-row findings. (3) #2024 added one exemption to the session-id arm, for the brief-v2 frontmatter id line of a brief file (outbound.go, outboundforge.go, outboundpush.go, selfcontain.go). The phone-separator gap and the wider pass-through set are carried unchanged. #1989 tracks both.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 as authored: go build and go vet over deskkit and forgeban, in tools/desk | exit 0 | exit 0; build and vet printed nothing. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as authored (version 3): go test -count=1 over deskkit with the TestOutbound prefix selector, counting the Conformance PASS line | exit 0; rc=0 pass=1 | exit 0; printed rc=0 pass=1. A detail rerun of the same go test shows nine top-level TestOutbound tests, all PASS (Conformance 0.93s, plus ForgeWrapsEveryWriteMethod, the two GitLab tests, WindowsMachinePaths and four others), 229 PASS lines, 0 FAIL, 0 SKIP. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as authored (version 3): the anchored deskfile withheld-identifier public-target refusal test | exit 0; rc=0 pass=1 | exit 0; printed rc=0 pass=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as authored (version 3): the anchored deskfile same-body private-target test | exit 0; rc=0 pass=1 | exit 0; printed rc=0 pass=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as authored (version 3): the anchored deskpr added-test-comment push refusal test, plus the count of the zero-pushes assertion in its body | exit 0; rc=0 pass=1 zero=1 | exit 0; printed rc=0 pass=1 zero=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as authored (version 3): the anchored completeness test over the Forge interface | exit 0; rc=0 pass=1 | exit 0; printed rc=0 pass=1. Falsifiability probe: the same command with the test name changed to one that does not exist printed rc=0 pass=0 and exited 1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as authored (version 3): the single-construction-site test and the backend-type ban test | exit 0; rc=0 site=1 ban=1 | exit 0; printed rc=0 site=1 ban=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as authored (version 3): go run of muhar -j 1 over the outbound-mutations.json spec, asserting the healthy line, the five required CAUGHT lines and the Totals line | exit 0; healthy=1 required=5 survived=0 couldnot=0 | exit 0; printed healthy=1 required=5 survived=0 couldnot=0. Detail rerun: Harness healthy: baseline GREEN, positive control CAUGHT. Totals: 14 caught, 0 NOT CAUGHT, 0 could-not-mutate. The required mutants (decorator removed from the GitHub and the GitLab return of ResolveForge, PostReview dropped from the decorator, visibility test inverted, e-mail allow-list emptied) are all CAUGHT, as are ApplyLabels dropped, the withheld register made overridable, the audit row carrying the text, both push-path blindings and four item-number mutants. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as authored (version 3): the anchored digest-not-text override audit row test | exit 0; rc=0 pass=1 | exit 0; printed rc=0 pass=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as authored: grep for the three per-verb scan calls in deskfile, deskpost, deskreply and deskevidence, expecting grep status 1 | exit 0, nothing printed | exit 0; grep printed no line. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | Verify row 11 as authored (version 3): statusgen --consumers for this brief against base e7e9f35d3~1, asserting the exact summary line | exit 0; rc=0 summary: 6 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing | exit 0; printed rc=0 summary: 6 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing. Falsifiability probe: the same command with base HEAD printed rc=2 and an empty summary, and exited 1. As the row states, it cannot see consumer wiring removed from the code; rows 2, 6, 7 and 8 cover that. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | Verify row 12 as authored (version 3): the two TestOutboundGitLab tests | exit 0; rc=0 notes=1 noreply=1 | exit 0; printed rc=0 notes=1 noreply=1. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | Verify row 13 as authored (version 3): the anchored TestOutboundWindowsMachinePaths test, counting its RUN and PASS lines | exit 0; rc=0 run=1 pass=1 | exit 0; printed rc=0 run=1 pass=1. Overlay probe (go test -overlay, tree never written): a drive-letter path under the Users root written with backslashes, and a UNC host-and-share path, are both found by the machine-path scan, and an https URL is not. | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |

**Witness dry-run.** statusgen verifyrun --dry-run --brief on this brief, from the worktree root under a throwaway HOME: exit 0; 13 rows, 13 pass exit=0, 0 fail; the closing line says the table was NOT written to the brief. Output hashes: row 1 e3b0c44298fc, 2-4 70d9c12f19f5, 5 4b527b57f117, 6 70d9c12f19f5, 7 afe8c5fe9b59, 8 12fe8626451f, 9 70d9c12f19f5, 10 e3b0c44298fc, 11 209b0c111963, 12 171440e11a18, 13 9a11c543eb8e. The witness count (13/13) and the hand count (13/13) agree. Because every version 3 command asserts its own property, the witness's exit-status judgement now carries that property.

**Tags.** Row 8 is the only tagged row (+mutation). The dry-run ran it in the worktree, and the harness restored every file. No row carries +flow, +dereference or +neighbour.

**Context files.** Every planned NEW file exists at this head: outbound.go, outboundforge.go, personaldata.go (and outboundpush.go) and outbound-mutations.json under tools/desk/internal/deskkit, plus backendtype.go in forgeban. The named existing files (forgeresolve.go, forgeresolve_test.go, deskpr.go, the deskpushguard directory) exist too. The per-PR changelog fragment has been folded into CHANGELOG.md.

**Findings.**

- F1 (carried, tracked #1989). The international phone pattern allows only one separator character between digits. An overlay probe at this head shows the conformance fixture shape (single spaces) is refused as pii.phone. A number with a parenthesised area code, a number with a parenthesised trunk zero, and a number with doubled spaces all return no finding, so no refusal and no notice. personaldata.go has no commit since the prior verdict.
- F2 (carried, tracked #1989). The decorator's pass-through set is wider than the brief's five no-text families. It adds OpenMergeHold, RunWorkflow and ApproveGate, and only a code comment and the completeness test record that choice.
- F3 (new since the prior verdict, #2024). The session-id arm now skips exactly one line: the brief-v2 frontmatter id line of a file under docs/streams whose name matches brief-*.md, when the id is a lowercase dashed UUID and the frontmatter has a single id key. Each condition fails closed. This narrows an existing self-containment category for one line shape. The narrowing came from a separate merged change, not from this brief.
- F4 (housekeeping). #2027 (the Verify-row falsifiability report) is still open, although #2044 merged the re-authored rows that this run exercises.

**Risk-bearing value enumeration.** Trigger: sensitive-data: yes. Scope: every literal the outbound check introduced in #1919, plus what #2033 and #2024 added to it since the prior verdict. Every line number was re-read at b3677d6da493. Paths are under tools/desk/internal/deskkit.

| Rank | Literal @ file:line | If wrong | Undo by edit + redeploy? |
|---|---|---|---|
| 1 | overridable() = f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier @ outbound.go:220; rule ids "voice.ruling-claim" @ outbound.go:63, "withheld.identifier" @ outbound.go:74 | a written reason publishes a withheld identifier to a public repository | no |
| 2 | outboundPublicLayers = SelfContainApplies @ outbound.go:226, which is RepoVisibility(repo) != VisibilityPrivate @ config.go:198, behind a not-Configured early return false @ selfcontain.go:314-315 | the public layers are skipped on a public or unstated target | no |
| 3 | reAbsMachinePath (five unix roots) @ selfcontain.go:125; reWinUsersPath (drive letter, colon, Users root) @ selfcontain.go:146; reWinUNCPath (two or more separators, a host of at least two characters, a share) @ selfcontain.go:147; each Windows match confirmed by IsAbsFor windows @ selfcontain.go:406 | a machine path reaches a public target | no |
| 4 | reBriefFrontmatterID (a top-level id key followed by a lowercase dashed UUID, bare or double-quoted, anchored at both ends) @ selfcontain.go:173-174; single-key rule keys != 1 in briefIDExemptLine | a session UUID passes on a brief's id line | no |
| 5 | rePhoneE164 = \+[0-9](?:[ .()-]?[0-9]){5,20} @ personaldata.go:34; digit bounds n < 8 or n > 15 in personalDataScan; rePhoneGrouped @ personaldata.go:37 | an international phone number reaches any target | no |
| 6 | emailAllowedDomains (two forge no-reply domains, three reserved example domains) @ personaldata.go:42-48; emailAllowedTLDs = example, invalid, test @ personaldata.go:51; emailAllowedExact (the GitHub no-reply address) @ personaldata.go:54 | a personal address passes as allowed | no |
| 7 | pass-through classification adding OpenMergeHold, RunWorkflow, ApproveGate @ outboundforge.go:18-21 | text carried by a workflow input leaves without a check | no, if a workflow publishes it |
| 8 | withheldMarker = "withheld register identifier" @ scanoverride.go:158 | the pre-flight path would accept the override, though the seam still refuses | yes |
| 9 | minOverrideReason = 12 @ scanoverride.go:54 | weaker forensic reasons | yes; a reversible knob, so it ranks last |

Verdict lines (file paths are under tools/desk/internal):

RISK-VALUE: DERIVED — overridable() = f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier @ deskkit/outbound.go:220 — the option-1 ruling on #1319 recorded in this brief: credential, personal-data and self-containment refusals are overridable; a withheld identifier and a ruling claim are not. The mutant that makes the withheld register overridable is CAUGHT (row 8).
RISK-VALUE: DERIVED — RepoVisibility(repo) != VisibilityPrivate @ deskkit/config.go:198 (via outbound.go:226) — the brief's "public AND unknown" key: fail-closed, no network read. The inverted-visibility mutant is CAUGHT. Carried note: with no configured roster (selfcontain.go:314-315) the public layers are skipped entirely. That behaviour predates #1919.
RISK-VALUE: DERIVED — reWinUsersPath / reWinUNCPath @ deskkit/selfcontain.go:146-147 — row 13's stated property (public drive-letter and UNC machine paths refuse, using IsAbsFor) is met. Each candidate is confirmed by IsAbsFor windows, so the class has one definition of absolute. The overlay probe finds both shapes and does not flag an https URL.
RISK-VALUE: DERIVED — reBriefFrontmatterID @ deskkit/selfcontain.go:173-174 — a brief-v2 frontmatter id is a public identifier the board resolves (this brief carries one), not a session. The exemption needs a brief path, a fenced frontmatter, exactly one id key, a lowercase dashed UUID, and the same line at the same number in the scanned text. Every other UUID in the same text still refuses. Residual: a session UUID typed into a brief's id line would pass that one line.
RISK-VALUE: NAMED, NOT DERIVED — rePhoneE164 = \+[0-9](?:[ .()-]?[0-9]){5,20} @ deskkit/personaldata.go:34 — the 8-15 digit bound is derived from the brief's rule table and the E.164 maximum. The one-separator-per-gap rule contradicts the brief's "optional separators": a parenthesised area code passes on every target (probe, F1). Missing: a pattern that admits grouped separators, and a conformance row for that shape (#1989).
RISK-VALUE: DERIVED — emailAllowedDomains / emailAllowedTLDs / emailAllowedExact @ deskkit/personaldata.go:42-54 — this is the brief's compiled allow-list, entry for entry ("Nothing else"). The emptied-list mutant is CAUGHT.
RISK-VALUE: NAMED, NOT DERIVED — pass-through set {OpenMergeHold, RunWorkflow, ApproveGate} @ deskkit/outboundforge.go:18-21 — wider than the brief's five pass-through families. No brief or design-record line states it, so there is nothing to derive it from (#1989).

rows_passed=13 rows_total=13

**VERIFY: PASS — 13/13 version 3 rows pass on main b3677d6da493 and the witness agrees; Evidence only for a gate: human, sensitive-data item, so the status stays implemented for the human sign-off on #1912, with two NAMED, NOT DERIVED values routed via #1989**

## Review
Gate: human (sensitive-data: yes — the brief decides what personal data and which withheld
identifiers the tools will refuse to publish, and who may override that). MANDATORY human
sign-off; the human rules the override policy in `## Human decision`. Reviewer answers, in the
verdict: what single control stands between a withheld identifier and a public repository, and
which Verify row proves a lower layer catches it with the upper one bypassed (row 6 with the
fixtures untouched; row 8's mutants). Rows 3-5 are the three incident fixtures; row 4 is the
private-target control. Reviewer records verdict + date in the stream README table.
