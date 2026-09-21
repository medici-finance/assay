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
version: 1
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

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/ ./internal/forgeban/` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/ -run 'TestOutbound' -v` | output contains the literal line `--- PASS: TestOutboundConformance` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 3 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewRefusesWithheldIdentifierOnPublicTarget -v` | output contains `--- PASS: TestNewRefusesWithheldIdentifierOnPublicTarget`. Fail-first: on the unfixed code `deskfile new` FILES the issue (the fake forge records one `FileIssue`) — quote that red run in the PR body |
| 4 | `cd tools/desk && go test ./cmd/deskfile/ -run TestNewPassesSameBodyOnPrivateTarget -v` | output contains `--- PASS: TestNewPassesSameBodyOnPrivateTarget` — the same text, private target, one `FileIssue` recorded |
| 5 | `cd tools/desk && go test ./cmd/deskpr/ -run TestPushRefusesWithheldNameInAddedTestComment -v` | output contains `--- PASS: TestPushRefusesWithheldNameInAddedTestComment` and the test asserts the push seam recorded ZERO pushes. Fail-first: on the unfixed code the push proceeds |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestOutboundForgeWrapsEveryWriteMethod -v` | output contains `--- PASS: TestOutboundForgeWrapsEveryWriteMethod` — the completeness layer, independent of the conformance fixtures |
| 7 | `cd tools/desk && go test -timeout 10m ./internal/forgeban/ ./internal/deskkit/ -run 'Test(ForgeSingleConstructionSite)$' -v && go test ./internal/forgeban/ -run TestNoBackendTypeOutsideDeskkit -v` | output contains BOTH `--- PASS: TestForgeSingleConstructionSite` and `--- PASS: TestNoBackendTypeOutsideDeskkit` — the structural layer: one construction site, and no cmd package can name a backend type to build or unwrap one |
| 8 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/deskkit/outbound-mutations.json` | the harness's own Totals line reports every mutation KILLED and none survived; `internal/deskkit/outbound-mutations.json` (planned) carries at least: the decorator removed from `ResolveForge`'s return, one text-carrying method dropped from the decorator, the visibility test inverted, and the e-mail allow-list emptied — the fail-first evidence a reviewer re-runs. (`go run` flattens the exit code, so assert on the Totals line, not the status) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestOverrideAuditRowHoldsDigestNotText -v` | output contains `--- PASS: TestOverrideAuditRowHoldsDigestNotText` |
| 10 | `grep -rn --include='*.go' --exclude='*_test.go' -e 'deskkit.ScanSurface' -e 'deskkit.BodyCheck' -e 'deskkit.SelfContainCheck' tools/desk/cmd/deskfile tools/desk/cmd/deskpost tools/desk/cmd/deskreply tools/desk/cmd/deskevidence; test $? -eq 1` | exit 0 and no line printed — the per-verb scan calls are GONE from the verbs whose writes all cross the decorator (the removal, not just the addition). `deskpr` is excluded by design: its `--check` pre-flight is the recorded second caller |
| 11 | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data: yes — the brief decides what personal data and which withheld
identifiers the tools will refuse to publish, and who may override that). MANDATORY human
sign-off; the human rules the override policy in `## Human decision`. Reviewer answers, in the
verdict: what single control stands between a withheld identifier and a public repository, and
which Verify row proves a lower layer catches it with the upper one bypassed (row 6 with the
fixtures untouched; row 8's mutants). Rows 3-5 are the three incident fixtures; row 4 is the
private-target control. Reviewer records verdict + date in the stream README table.
