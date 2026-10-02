---
brief: assay:assay:forge-gitlab:16
title: The review-tick conformance walk — one live GitLab review tick, every verb, zero hand-built calls
why: >-
  The stream can say the review desk works on GitLab only once one review tick has actually run
  there, driven end to end by desk verbs with no hand-built API call and no forge CLI anywhere in
  it. That is the same bar the earlier live pilot set and could not meet — its round trip ran on
  hand-built calls because no verb had a GitLab backend, so every guard the verbs carry was absent
  for its duration. Since then each verb has landed, one field report at a time, and ten adopter
  issues describe boots and ticks that were blocked by defects whose fixes are now in the tree with
  the issues still open: nobody has replayed them. This brief is the finish line and the close-out
  in one motion — walk the tick, record a per-verb verdict, and give every standing adopter issue a
  delivered-or-still-open answer from a live run instead of a code read.
wave: 5
depends: ["forge-gitlab/13", "forge-gitlab/14", "forge-gitlab/15"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The walk needs a live GitLab project, live per-role service-account credentials, and it performs
  real writes on a real change — a verdict, a label, an escalation issue, an Evidence commit and a
  ready-flip. The human chooses the project, authorizes the credentials, and signs the resulting
  per-verb conformance table, which is the artifact every downstream claim that the review desk
  runs on GitLab will cite. It is also the run that decides which standing adopter issues are
  closed, so a wrong verdict here closes a real defect on a false green — a judgement no model
  self-certifies.
decision-trigger: start
issues: [1071]
schema: brief-v2
authored: 2026-09-14 by forge-gitlab wave-plan authoring session
sources:
  - "#1071 — the notification this plan answers: the remaining review-desk work is a sequenced initiative whose fixes uncover the next latent defect, and the default it names is a wave plan with an explicit finish line. This brief IS that finish line"
  - "docs/streams/forge-gitlab/pilot-report.md §2 — the earlier live round trip, whose every write was a hand-built call because no desk verb had a GitLab backend. That is the counter-example this walk must not repeat, and the reason the acceptance bar is zero hand-built calls rather than a working outcome"
  - "docs/streams/forge-neutral/README.md — the shared convention this walk inherits verbatim: a row satisfied by a hand-built call proves the forge works, not that the verbs do"
  - "#798 — the field report whose three blockers are the walk's first three rows: the verdict write, the escalation filing, and the queue label on the live change. Its first two are served on the current tree; its third is served in code and explicitly recorded by the reporter as unproven in production"
  - "#795 — the round before it, same ceremony order, and the source of the idle-gate row"
  - "#1067 — the board-visibility report; the walk cannot start until its row passes, which is why the board brief is a dependency rather than a row here"
  - "#655, #667, #668, #671, #676, #677, #678, #642, #651, #896 — the standing adopter backlog this walk closes out. Each was measured against the current tree during this plan's authoring and each looks delivered from a code read; a code read is not the instrument this brief uses"
  - "docs/streams/forge-gitlab/spec.md §1 (Community Edition is conforming for the core lane with two disclosed degradations and no third without a recorded ruling), §3 (parity per control), §7 (the conformance gate)"
  - "docs/streams/forge-gitlab/edition-matrix.md — every operation the interface performs is Free-tier, so the walk runs on Community Edition; what is tier-gated is a handful of guarantees, each with a named fallback"
  - "freshness-checked 2026-09-14 @ 2c67b34f — the verdict verb selects a typed backend on a non-default-forge resolution; label apply ensures the project label then applies it; the role-login resolver renders the bare service-account username on GitLab; the draft-change verb's visibility gate reads the resolved backend"
exec-tier: strong
exec-tier-why: "the deliverable is an end-to-end verdict across every verb, two identities and a live forge, where a single verb quietly falling back to a hand-built call or an ambient credential would leave the walk green and the claim false (question b); and the close-out half must distinguish delivered from merely-unreproduced for ten issues, which is a judgement, not a check (question a)."
domain: complex
tier: free
consumers:
  - "docs/streams/forge-gitlab/pilot-report.md: fixed-here (a review-tick appendix carrying the per-verb conformance table and the walk's transcript-free evidence — exit codes, the change and issue references created, and the negative-path refusals)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the status row and the finish-line statement's proven-on date)"
  - "docs/adopting-assay-gitlab.md: follow-up forge-gitlab/16 (this brief — the parity statement section gains the review-tick row once the walk records a verdict; no runbook instruction changes)"
  - "the standing adopter issues named in sources: fixed-here for the close-out verdict only — the walk records delivered-or-still-open per issue; CLOSING them is a desk act on the recorded verdict, not part of this brief's diff"
version: 1
id: dcaabe12-8668-4bba-9722-5e8a3f7787c8
---

# Brief 16 — The review-tick conformance walk

## Context

The finish line for this stream's review-desk half is one sentence, and this brief is the only
thing that can say it has been reached:

> On a GitLab-resolved project, one review window completes a full review tick — see the queue,
> dispatch a reviewer, post a correctness verdict, file an escalation, edit its workpad, land
> Evidence, and flip the change ready — with **zero** hand-built API calls and **zero** forge-CLI
> invocations anywhere in the tick, under per-role service-account credentials, on Community
> Edition.

Every clause is load-bearing. "Zero hand-built calls" is the clause the earlier live round trip
failed: it completed, and it proved the forge worked rather than that the verbs did, because
every write in it was hand-built and therefore carried none of the guards the verbs exist to
apply. "Community Edition" is the tier the stream's own ruling says is conforming for the core
lane. "Per-role service-account credentials" is what distinguishes a tick from an operator
driving the tools under their own identity.

The second half of the brief is the close-out. Ten adopter issues describe boots and ticks
blocked by defects, and every one of them reads as delivered against the current tree. A code
read is not evidence that a live boot finishes — it is evidence that the code changed. The walk
converts each into a verdict an issue can be closed on, or into a named residue that becomes a
new brief.

files:
- `docs/streams/forge-gitlab/pilot-report.md` — a review-tick appendix: the per-verb conformance
  table, the per-issue close-out verdicts, and the negative-path results. No transcripts, no
  host names, no credentials, no project identifiers beyond what the report already carries.
- `docs/streams/forge-gitlab/README.md` — the status row and the finish-line proven-on date.
- `changelog/forge-gitlab-16-review-tick-conformance-walk.md` (planned).

single-point-of-failure: the walk's own honesty is the one control — a verb that quietly fell
back would leave a green table. It is backed by two layers that fail on different signals in
different components: (a) the tick is run with no forge CLI on the executing path at all, so a
fallback cannot succeed even if one were attempted — the failure is a missing binary, not a
judgement; and (b) a negative-path row that performs a write the verbs are built to REFUSE and
records the refusal, so the table distinguishes "the verbs worked" from "the credential worked".
A walk that only records successes has verified the credential, not the boundary.

facts:
- Acceptance bar: zero hand-built API calls, zero forge-CLI invocations, per-role credentials,
  Community Edition. A row satisfied any other way is recorded as could-not-check, never as a
  pass.
- Verb set for one tick, in ceremony order: board read, review dispatch with its queue label,
  correctness verdict, escalation filing, workpad edit, Evidence landing, ready-flip.
- The queue-label step is served in code — the label apply ensures the project label exists and
  then applies it — and was recorded by the field reporter as unproven in production. It is
  therefore a walk row, not a code brief.
- Tier: Community Edition. The two disclosed degradations stand and are recorded as such in the
  table; the walk must not discover a third without escalating rather than recording it.
- The close-out set is the ten standing adopter issues named in `sources:`. Each gets exactly
  one of: delivered (with the row that proves it), still-open (with what was observed), or
  could-not-check (with what prevented the observation).
- Offline half: every row that CAN be proven against the recorded GitLab fixture set in the
  backend's golden tests is ALSO recorded offline, so a later reader can re-establish the
  mechanical half without a live project.

## Human decision

A conformance walk is about to be run that proves whether the review desk works on GitLab. It
needs a real GitLab project on a self-managed Community Edition instance, per-role service
account credentials for that project, and permission to perform real writes on a real change
there: post a review verdict, apply a queue label, file an escalation issue, add a comment, land
an evidence commit, and mark the change ready for review.

The walk also decides the fate of ten standing adopter reports. Each of them describes something
that was broken; each now looks fixed when the code is read. The walk will record, per report,
whether it is genuinely fixed on a live system or only appears so. Closing a report on a wrong
verdict puts a real defect back in front of the next adopter.

What is needed: a project to run on, authorization for the credentials, and a decision about how
much the walk may write.

Options:

1. **Run the full walk on a disposable project, with all writes permitted.** The whole tick runs
   as designed, every verb is exercised, and the negative-path refusals are observed. This gives
   the strongest verdict and the cleanest close-out. Consequence: real writes happen, so the
   project must be one where that is acceptable; a disposable project created for the walk is the
   intended shape. This is the recommended option.
2. **Run the walk read-only, and record every write step as could-not-check.** Nothing is
   written. Consequence: the walk proves the reads and proves nothing about the boundary the
   writes go through, which is the half that matters — so the finish line is not reached and the
   ten reports stay open. Choose this only if no project can be made available.
3. **Run the walk on an existing working project with writes limited to a scratch change.**
   A middle path. Consequence: the escalation-filing and evidence steps still write to the real
   project, so the blast radius is smaller but not zero, and the resulting table has to say which
   steps were narrowed.

Default if no answer: none — blocks until answered. The walk cannot be started on an assumed
project or an assumed credential.

## Edition
Minimum GitLab tier: **free**. The walk runs on Community Edition deliberately, because that is
the tier the stream's ruling declares conforming for the core lane. The two disclosed
degradations are recorded in the table as degradations, not as failures. A THIRD degradation
discovered during the walk is escalated for a ruling and recorded as blocked — never absorbed
into the table as if it had always been disclosed.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands outside the walk's own
  authorized project. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No hand-built API call and no forge CLI anywhere in the tick, not even to unstick a step. A
  step that cannot be completed through a verb is a could-not-check row and the walk continues;
  routing around it destroys the only thing the walk measures.
- No host name, credential, project identifier or transcript in the recorded appendix beyond
  what the existing report already carries.

## Task
1. Provision or obtain the walk's project per the human decision, following the GitLab runbook
   as written — deviations from the runbook are themselves findings and are recorded.
2. Run one full review tick in ceremony order, recording per verb: the exit code, whether the
   step completed through a verb, and the observable outcome on the project.
3. Run the negative-path steps: at least one write the verbs are built to refuse, and record the
   refusal. A table with no refusals in it has not tested the boundary.
4. Record the offline half: for every row with a fixture equivalent, the backend golden test
   that establishes the same mechanical fact, so a later reader can re-check without a project.
5. Give each of the ten standing adopter reports a delivered / still-open / could-not-check
   verdict with the row that supports it. Any still-open residue becomes a named follow-up, not
   a sentence in the appendix.
6. Write the appendix and update the stream board's finish-line statement with the proven-on
   date.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -cE '^[\|] verb ' docs/streams/forge-gitlab/pilot-report.md` | `7` or more — one row per verb in the ceremony order, each carrying an exit code. Rows in the review-tick appendix begin with the literal cell text `verb`, so this counts the appendix's rows and not every table in the report | check |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v -timeout 300s` | exit 0; output contains `PASS` — the offline half of every row with a fixture equivalent re-establishes without a live project | check +flow |
| 3 | `! grep -q -e 'satisfied by a hand-built call' -e 'satisfied by a forge CLI' docs/streams/forge-gitlab/pilot-report.md` | exit 0 — no appendix row records itself as satisfied by a hand-built call or a forge CLI. Gating on the exit status, not on a count, because a count of zero exits non-zero on the success path | check |
| 4 | the walk's own negative-path step: perform one write the verbs refuse, through the verb | non-zero exit; the refusal names the control that fired, and the appendix records both | gate:human |
| 5 | `grep -c -e 'delivered' -e 'still-open' -e 'could-not-check' docs/streams/forge-gitlab/pilot-report.md` | `10` or more — every standing adopter report in the close-out table carries exactly one verdict | check |
| 6 | for each report the walk marks delivered, read the appendix row it cites and confirm that row's recorded outcome actually supports the verdict | every delivered verdict cites a row whose outcome supports it; a delivered verdict whose cited row does not, or that cites none, is re-recorded as could-not-check | gate:human +dereference |
| 7 | `statusgen --root . --consumers` | exit 0 | check |
| 8 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A verb fails mid-tick and the walker completes the step by hand, leaving a green table | row 3 + the executing path carrying no forge CLI at all, so the fallback cannot succeed |
| Every row is a success, so the table proves the credential works rather than that the boundary does | row 4 (the mandatory refusal row) |
| A standing report is marked delivered because the code looks right, not because the walk observed it | row 6 (a delivered verdict must cite the row that proves it) |
| The walk runs on a tier above Community Edition and the result over-claims | recorded in the appendix header; the tier claim is review-only, and the reviewer is asked for it below |
| A third degradation is discovered and quietly written into the table as disclosed | no row — Ground rules require escalation; a table row naming an undisclosed degradation is a review bounce |
| The appendix leaks a host name or credential | no row — the existing report's redaction convention applies and is review-only |

### Dispatch checklist
```
[x] 1. Rows discriminate — row 3 goes red on a hand-built rescue; row 6 goes red on a verdict asserted from a code read.
[x] 2. Facts dated — the current-tree state of each verb was re-measured 2026-09-14 @ 2c67b34f and is recorded in sources:.
[x] 3. Self-contained — the finish-line sentence, the acceptance bar, the verb set and the close-out set are all here.
[x] 4. Risk answers match files: — live credentials and real writes on a real project make sensitive-data yes, which is what forces the human gate; the other three are no because the walk changes no product behaviour and every write is reversible on a disposable project.
[x] 5. gate-why substantive — names the credentials, the writes, and the close-out judgement the human is signing.
[x] 6. Effort honest — one tick plus ten close-out verdicts plus the appendix: M. It is not L because the product work is in the wave below it.
[x] 7. No shared value changes; consumers: names the runbook follow-up and the close-out boundary (the walk records the verdict; closing is a desk act).
[x] 8. Pre-mortem run; the three review-only items are named.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers both core-path questions: (1) what single control stands between a verb quietly
falling back and a green conformance table, and is it acceptable? (2) does any row prove a
refusal fired with the happy path bypassed, or does the table only walk successes end to end?
