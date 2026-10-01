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
version: 2
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

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go vet ./internal/deskkit/ ./internal/forgeban/` | exit 0 |
| 2 | check | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run 'TestOutbound' -v` | output contains the literal line `--- PASS: TestOutboundConformance` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 3 | check | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewRefusesWithheldIdentifierOnPublicTarget -v` | output contains `--- PASS: TestNewRefusesWithheldIdentifierOnPublicTarget`. Fail-first: on the unfixed code `deskfile new` FILES the issue (the fake forge records one `FileIssue`) — quote that red run in the PR body |
| 4 | check | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewPassesSameBodyOnPrivateTarget -v` | output contains `--- PASS: TestNewPassesSameBodyOnPrivateTarget` — the same text, private target, one `FileIssue` recorded |
| 5 | check | `cd tools/desk && go test ./cmd/deskpr/ -run TestPushRefusesWithheldNameInAddedTestComment -v` | output contains `--- PASS: TestPushRefusesWithheldNameInAddedTestComment` and the test asserts the push seam recorded ZERO pushes. Fail-first: on the unfixed code the push proceeds |
| 6 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundForgeWrapsEveryWriteMethod -v` | output contains `--- PASS: TestOutboundForgeWrapsEveryWriteMethod` — the completeness layer, independent of the conformance fixtures |
| 7 | check | `cd tools/desk && go test -timeout 10m ./internal/forgeban/ ./internal/deskkit/ -run 'Test(ForgeSingleConstructionSite)$' -v && go test ./internal/forgeban/ -run TestNoBackendTypeOutsideDeskkit -v` | output contains BOTH `--- PASS: TestForgeSingleConstructionSite` and `--- PASS: TestNoBackendTypeOutsideDeskkit` — the structural layer: one construction site, and no cmd package can name a backend type to build or unwrap one |
| 8 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/deskkit/outbound-mutations.json` | the harness's own Totals line reports every mutation KILLED and none survived; `internal/deskkit/outbound-mutations.json` (planned) carries at least: the decorator removed from `ResolveForge`'s return, one text-carrying method dropped from the decorator, the visibility test inverted, and the e-mail allow-list emptied — the fail-first evidence a reviewer re-runs. (`go run` flattens the exit code, so assert on the Totals line, not the status) |
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestOverrideAuditRowHoldsDigestNotText -v` | output contains `--- PASS: TestOverrideAuditRowHoldsDigestNotText` |
| 10 | check | `grep -rn --include='*.go' --exclude='*_test.go' -e 'deskkit.ScanSurface' -e 'deskkit.BodyCheck' -e 'deskkit.SelfContainCheck' tools/desk/cmd/deskfile tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskevidence; test $? -eq 1` | exit 0 and no line printed — the per-verb scan calls are GONE from the verbs whose writes all cross the decorator (the removal, not just the addition). `deskpr` is excluded by design: its `--check` pre-flight is the recorded second caller |
| 11 | check | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |
| 12 | check | `cd tools/desk && go test ./internal/deskkit/ -run '^TestOutboundGitLab(NoReply|TypedNotes)$' -v` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); both top-level tests PASS; GitLab.com no-reply and reserved documentation-host shapes (generic self-managed recognition remains pending scope on issue 1836), internal target and explicit MR note route; refusal sends no request (desktools-v2/12 GitLab row) |
| 13 | check | `cd tools/desk && go test ./internal/deskkit/ -run '^TestOutboundWindowsMachinePaths$' -v` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); named top-level TestOutboundWindowsMachinePaths PASS; public drive-letter and UNC machine paths refuse using IsAbsFor. PENDING owner scope clarification on issue 1836: this test and production scanner extension are not supplied by the incomplete preparation (desktools-v2/12 Windows row) |

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

## Review
Gate: human (sensitive-data: yes — the brief decides what personal data and which withheld
identifiers the tools will refuse to publish, and who may override that). MANDATORY human
sign-off; the human rules the override policy in `## Human decision`. Reviewer answers, in the
verdict: what single control stands between a withheld identifier and a public repository, and
which Verify row proves a lower layer catches it with the upper one bypassed (row 6 with the
fixtures untouched; row 8's mutants). Rows 3-5 are the three incident fixtures; row 4 is the
private-target control. Reviewer records verdict + date in the stream README table.
