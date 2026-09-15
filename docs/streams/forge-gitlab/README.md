---
stream: forge-gitlab
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
board: generated
---

# forge-gitlab Stream

Run the Assay fleet on **GitLab Enterprise** with guarantees at least as strong as
the existing GitHub controls — per control, verified on a live deployment, even
where the mechanism is completely different. The methodology core (briefs,
registers, lifecycle, board, statusgen) is already forge-agnostic; this stream
delivers the identity/permission/CI layer (service accounts, protected-branch
gates, a locked ci-config project) and the tooling seam (a `Forge` interface in
deskkit with `github` extracted as-is and `gitlab` implemented against REST v4,
plus rotate-on-mint token custody). See [spec.md](spec.md) for the accepted
design, the per-control security-parity table, and the tier ruling of 2026-08-30
in its §1: Community Edition is conforming for the core lane with two named,
disclosed degradations, Premium is the hardening, Ultimate is refinement.

Coordination note: [desktools-go-git](../desktools-go-git/README.md) refactors
the same tools' **git-binary** seam while this stream refactors their **forge-API**
seam. The seams are disjoint, but both touch `tools/desk/**` broadly — land
forge-gitlab/01's extraction either before desktools-go-git's migration waves or
rebased across them; never concurrently with an in-flight migration brief of the
same tool.

## Edition — CE-first

**Ruled 2026-08-30: Community Edition is conforming for the core lane** (01-05, 07, 08), with
two named, disclosed degradations — identity-granular protected branches (on CE the Maintainer
role set is the allowlist) and enforced approval rules (on CE approvals are advisory, so
verdict-before-merge is human-merge-only plus the desk's refusal to flip without an at-head
verdict). Premium is the hardening that makes both server-enforced; Ultimate is refinement
(brief 06). Neither is a prerequisite for the core lane. The binding wording is
[spec.md](spec.md) §1, which §3 scopes to those two degradations and no others.
[edition-matrix.md](edition-matrix.md) is the evidence: per operation and per control, with a
GitLab docs citation per row, which tier each thing needs. Its finding is that every operation
the `Forge` interface performs is Free-tier, so the tooling and the pilot run on CE; what is
tier-gated is a handful of *guarantees* — the two above (Premium), plus external status checks,
custom roles and the instance token-lifetime policy (Ultimate) — each with a named CE fallback
the desk tools own.

Minimum tier per brief (the `tier:` line in each brief's front-matter, with the detail in its
`## Edition` section):

| Brief | 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08 | 09 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Minimum tier | free | free | free | free | free | ultimate | free | free | free | free | free | free | free | free | free | free | free |

The open point that stood here — spec.md section 1 declaring Free/CE non-conforming, which the
matrix's per-feature citations did not support as written — was ruled on 2026-08-30
(#219) as stated above. Spec.md section 1 carries the ruling; the matrix
carries its evidence; brief 04's Verify row 3 was re-baselined onto the amended sentence.

## Finish line — "the review desk works on GitLab"

The clause the stream is judged on, stated as an observable end state rather than a feeling
that the tooling is close:

> On a GitLab-resolved project, one review window completes a full review tick — **see the
> queue** (`deskboard actions`), **dispatch a reviewer with its queue label**
> (`deskdispatch --kit review --pr`), **post a correctness verdict** (`deskpost review`),
> **file an escalation** (`deskfile new`), **edit its workpad** (`deskreply --workpad`),
> **land Evidence** (`deskevidence`) and **flip the change ready** (`deskflip`) — with
> **zero hand-built API calls and zero forge-CLI invocations anywhere in the tick**, under
> per-role service-account credentials, on **Community Edition**.

Three clauses carry the weight, and each exists because something already passed without it:

- **Zero hand-built calls.** The 2026-09-02 live pilot completed its round trip and proved the
  *forge* worked rather than that the *verbs* did — every write in it was hand-built, so every
  guard the verbs carry was absent for its duration ([pilot-report.md](pilot-report.md) §2).
  A row satisfied by a hand-built call is could-not-check, never a pass.
- **Per-role service-account credentials.** A tick driven under an operator's own identity
  measures the operator, not the fleet.
- **Community Edition.** The tier the 2026-08-30 ruling declares conforming for the core lane
  (spec.md §1). The two disclosed degradations stand; a third discovered during the walk is
  escalated for a ruling, never absorbed.

**Proven how.** Two instruments, because one is not enough. The **live** half is the walk
itself, recorded as a per-verb conformance table with an exit code per row plus at least one
NEGATIVE row in which a write the verbs are built to refuse is refused — a table of successes
has verified the credential, not the boundary. The **offline** half is the recorded GitLab
fixture set in the backend's golden tests, so every mechanical fact in the table can be
re-established by a later reader with no live project. `forge-gitlab/16` is the brief that
runs both and is the only thing that may mark this finish line reached.

**Proven on:** not yet — `forge-gitlab/16` is unstarted (2026-09-14).

## Sequencing note — why this stream is planned, not fanned out

Issue #1071 recorded the pattern rather than another bug: fixing
the reviewer-identity mismatch (#1056) made identity matching start
working, which made a latent board classifier defect (#1067) reachable for the first time. The
second defect did not exist independently of the first fix — it was *behind* it. That is the
shape of the remaining review-desk work: **the fixes uncover the next latent defect in
sequence.** Discovering that sequence one field report at a time costs a full adopter round
trip per link, and the adopter is the instrument.

This plan is the desk's acceptance of #1071's stated default, and it carries two standing
rules:

1. **A GitLab review-desk issue routes to this plan first.** Before a fix is dispatched, the
   issue is placed on the map below — against a brief, or explicitly out of scope with a
   reason. An issue that is genuinely a one-off says so on the map; what it may not do is get
   fanned out without the map being consulted, because the map is where the *next* latent
   defect is predicted rather than discovered.
2. **A code read is not a field verdict.** The authoring pass for this plan re-measured the
   whole standing backlog against the tree and found that most of it looks delivered. Looking
   delivered is why those issues are routed to `forge-gitlab/16`'s close-out rather than
   closed here: the instrument that closes a field report is a field run.

The pattern showed itself again while this document was being written. The head of the open
path was `forge-gitlab/13` when the pass started; the single-site fix for its instance merged
partway through, the board became visible, and the head moved one step down the ceremony to
`forge-gitlab/14` — the next verb in the tick, whose defect was already on file and already
waiting. That is what a sequenced initiative looks like from the inside, and it is the argument
for the map: the step after the one being fixed is knowable in advance.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Forge interface extraction in deskkit — github impl pinned by goldens](brief-01-forge-interface-extraction.md) | 1 | L | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #222 @ fa65252d19f35341109f56c8a521466dd140f548) |
| 02 | [gitlab forge implementation — MRs, notes, approvals, statuses over REST v4](brief-02-gitlab-forge-impl.md) | 2 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #273 @ 167dadea6e583cc44c04c29898c55086ffa9696a) |
| 03 | [GitLab token custody — rotate-on-mint + expiry backstop in desktoken](brief-03-gitlab-token-custody.md) | 2 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #273 @ 167dadea6e583cc44c04c29898c55086ffa9696a) |
| 04 | [Fleet provisioning script + adopter doc + ci-config-project runbook](brief-04-provisioning-and-adopter-doc.md) | 3 | M | done | 2026-09-05 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #481 @ bbdd8747858e29f80f0fe105801fda9a02ddf7f0) |
| 05 | [Live pilot — one brief round-tripped on a real GitLab group, security-parity table walked](brief-05-live-pilot-parity-walk.md) | 4 | M | implemented | — | — |
| 06 | [Ultimate refinements — custom reviewer role + external-status-check verdict lane](brief-06-ultimate-refinements.md) | 5 | M | done | 2026-09-11 opus-4.8[1m]-verifier (offline rows 1,3 PASS; live row-2 push-rejection proof deferred per Ian ruling #838) | 2026-09-12 assay-reviewer-app[bot] (approved PR #839 @ 14d67b17b42ade253f07fa65d3beeddb0307b23e) |
| 07 | [GitHub forge backend on go-gh — retire the exec-`gh` shell path](brief-07-github-forge-go-gh.md) | 2 | M | done | 2026-09-10 opus-4.8[1m]-verifier (assay 48b978bb; 5/5 rows PASS; go-gh backend, unset-token refuses no-ambient-fallback proven hitCount==0; 42 golden scenarios) | 2026-09-11 assay-reviewer-app[bot] (approved PR #878 @ 353845668ede560a672dd0986c0687435c37b9ff) |
| 08 | [Close the forge surface — enumerated operations, no passthrough, shell-exec ban](brief-08-close-the-forge-surface.md) | 3 | M | implemented | — | — |
| 09 | [GitLab reviewer write path — deskpost verdict/comment/ready + deskfile/desktoken PAT auth](brief-09-gitlab-reviewer-write-path.md) | 4 | M | done | 2026-09-11 opus-4.8[1m]-verifier (assay 8953d38d; 7/7 + §6 consumer PASS; §1 landed #846, prior FAIL was stale) | 2026-09-11 assay-reviewer-app[bot] (approved PR #878 @ 353845668ede560a672dd0986c0687435c37b9ff) |
| 10 | [GitLab trust-events + commit author-login for the deskpost trust read](brief-10-gitlab-trust-events.md) | 5 | M | implemented | — | — |
| 11 | [Guard-read custody — the last gh shell-outs onto the Forge seam](brief-11-guard-read-custody.md) | 4 | M | todo | — | — |
| 12 | [GitLab hardening reads — repohardenguard kinds on the GitLab backend](brief-12-gitlab-hardening-reads.md) | 5 | M | todo | — | — |
| 13 | [Board reads degrade per row, never per sweep — the GitLab empty-field class](brief-13-board-reads-degrade-per-row.md) | 3 | M | todo | — | — |
| 14 | [The public-repo gate reads the forge that serves the repo — every verb, not one](brief-14-public-repo-gate-on-the-resolved-forge.md) | 3 | M | todo | — | — |
| 15 | [The GitLab runbook's missing keys — forge binding, board-push credential, source-pin lane](brief-15-gitlab-runbook-missing-keys.md) | 4 | M | implemented | — | — |
| 16 | [The review-tick conformance walk — one live GitLab review tick, every verb, zero hand-built calls](brief-16-review-tick-conformance-walk.md) | 5 | M | todo | — | — |
| 17 | [An enforceable merge gate on GitLab Free — the unresolved review thread](brief-17-resolved-thread-merge-gate.md) | 5 | M | implemented | — | — |
<!-- statusgen:briefs:end -->

## Critical path

The stream now carries **three tracks**. Two are historical and closed or nearly so; the third
is the open one, and it is the one the finish line above is about. Read the open track first.

### Track A — the review desk on GitLab (OPEN; this is where the work is)

```
   (02 backend, 04 runbook, 09 write path — all landed)
                  │
      ┌───────────┼───────────┐
      ▼           ▼           ▼
    fg/14       fg/13       fg/15            fg/14  HEAD — the gates two review verbs still
   (gates)      (board)    (runbook)                 send to the wrong forge
      └───────────┼───────────┘              fg/13  the board's degrade CLASS, behind the one
                  ▼                                  arm repaired on 2026-09-14
               fg/16  ← the finish line              fg/15  the three keys the runbook never names
                       (one live review tick, human-gated)
```

One-line path: `{13, 14, 15} → 16`.

**The head of the open path is `forge-gitlab/14`, and the blocker is verified, not assumed.**
Two of the seven verbs in the tick still resolve the forge for every operation on the change
and then build a *separate*, GitHub-only HTTP client for the public-repo gate's live visibility
read:

- `tools/desk/cmd/deskreply/deskreply.go:223`
- `tools/desk/cmd/deskevidence/deskevidence.go:256`

On a GitLab-resolved project that read is addressed to a host that has never heard of the
project; the gate correctly fails closed on the resulting foreign error, and the verb has no
working path (#1066). That is the review desk's workpad and its Evidence landing — two of the
tick's seven steps — while every other verb in the same session is healthy against the same
project. The same defect on the draft-change verb (#1054) is what made it unusable there until
PR #1060 landed on 2026-09-14; these two are the untouched siblings the reviewer found at
discovery.

**The head moved during this authoring pass, which is the sequencing note's point made live.**
When this plan was started the head was `forge-gitlab/13`: on GitLab a review approval carries
no commit sha (`tools/desk/internal/deskkit/forge_gitlab.go` leaves the commit id empty **by
design**, and says at length why stamping one would manufacture an at-head verdict nobody gave),
the board's benign-merge arm asked for a diff against that empty sha, and the per-change refusal
was returned as a **whole-sweep** error — `deskboard actions` exiting 6 with empty stdout and one
line of diagnosis (#1067). PR #1068 merged at 19:34Z on 2026-09-14, mid-pass, and that arm now
degrades its row to the safe side with a diagnostic naming which endpoint was unreadable. The
board is visible again, so the next step in the ceremony became the blocker within the hour.

`forge-gitlab/13` stays on the plan and stays wave 3, re-baselined onto what merged: the
INSTANCE is closed, the CLASS is not. `classifyPR` still carries five whole-sweep error returns
reached with per-change inputs, of which exactly one is now guarded. The other four are the next
blank board, and PR #1068's guard is the reference shape they are brought up to. (#1067 itself
is still open and needs a close, not work.)

**Two tempting-but-wrong heads, both checked:**

- **`forge-gitlab/11`.** It is the stream's nominal head-of-path and this plan's PR trailer, and
  it is the head of *Track C*, not of the review desk. It moves the hardening guard and the
  roster's display reads onto the seam. No review tick calls either of them, so sequencing the
  review desk behind 11 would block a live desk on `repohardenguard`. Checked in the tree: the
  hardening read operation does not exist, no `auditor` role exists, and the guard still shells
  a forge CLI — 11 is genuinely unstarted and genuinely off this path.
- **The adopter boot backlog** (`#655`, `#667`, `#668`, `#671`, `#676`, `#677`, `#678`, `#642`).
  Eight open field reports say a GitLab desk cannot finish booting. Re-measured against the tree
  on 2026-09-14: the boot mint takes the GitLab custody path, the inapplicable grant check has
  its own fourth state, the custody permission test is behind an OS boundary with a
  Windows-native half, the child process keeps the Windows home variable, the worktree
  provisioner covers every loop and has a GitLab commit-identity path, the build script's parse
  error is gone, and the two runbook keys are documented. **The code is there; nobody has
  replayed it.** That is why the backlog routes to `forge-gitlab/16`'s close-out rather than
  becoming wave-1 work — the open issues are an unreplayed backlog, not a blocked boot, and
  putting them at the head would sequence the whole stream behind work that is already done.

### Track B — the conformance pilot chain (historical)

`forge-gitlab/01` (interface extraction — zero-behavior-change, golden-pinned) ->
`forge-gitlab/02` (gitlab implementation) -> `forge-gitlab/04` (provisioning +
docs) -> `forge-gitlab/05` (live pilot — **human gate**: a real Premium/Ultimate
group, real credentials, and the security-parity walk recorded as Evidence).

The head is genuinely 01 and it is unblocked: the desk tools and deskkit are in
this repository and no upstream release gates the extraction. The pacing item is
**05** — nothing may claim GitLab support before the pilot round-trips one brief
todo→done and walks the parity table on live settings. The tempting-but-wrong
first step is writing the GitLab REST code first (02 before 01): without the
extracted interface and goldens, the GitHub behavior has no pinned contract to
stay equal to, and every later refactor re-litigates it.

### Track C — the surface-hardening sub-track

Parallel to the pilot chain runs a **surface-hardening sub-track** off the same
seam: `forge-gitlab/07` (re-seat the GitHub backend on the official `go-gh`
library, retiring the exec-`gh` forge path) then `forge-gitlab/08` (close the
surface — enumerated operations only, no arbitrary-endpoint passthrough on
either backend, and a checked ban on `gh`/`glab` shelling across `tools/desk`).
It depends only on the interface (01) and the two backends (02 for the symmetric
`glab` side, 07 for the GitHub side); it does not gate — and is not gated by —
the live pilot (05). It is where the spec's "constrained typed surface is
*stronger* than an ambient full-CLI surface" (§3) becomes shipped, enforced
configuration rather than a design intention. Its Verify row 3 (closure-to-zero) FAILED on
2026-09-10 (#834): the ban shipped as a ratchet with three real `gh` sites still permitted. The
driver ruled closure-to-zero stands; `forge-gitlab/11` is the custody design that closes it and
`forge-gitlab/12` finishes the hardening guard on GitLab. 08 re-verifies when 11 lands.
That custody design's own human gate is now RULED (#857, closed `human-decided` 2026-09-11):
option 1 — the dedicated read-only `auditor` identity — approved as proposed, plus one addition,
that the adopter documentation and the public website are updated alongside. `DR-forge-gitlab-11`
is APPROVED citing that ruling, and the brief carries the docs half as a deliverable with its own
Verify rows, so it is dispatchable; the website half is a companion change in the site repo,
tracked separately.

## Dependency waves

- **Wave 1** — `forge-gitlab/01` (no dependencies; the seam + goldens everything
  else builds on).
- **Wave 2** — `forge-gitlab/02`, `forge-gitlab/03` (depend on 01;
  parallelizable — the API implementation and the credential machinery are
  separable deliverables).
- **Wave 3** — `forge-gitlab/04` (depends on 02 + 03; provisioning script,
  adopter doc, ci-config-project runbook).
- **Wave 4** — `forge-gitlab/05` (depends on 04; the human-gated conformance
  pilot).
- **Wave 5** — `forge-gitlab/06` (depends on 05; Ultimate-tier refinements,
  scoped by what the pilot surfaces).

Surface-hardening sub-track (parallel, off the interface — not on the pilot
critical path):

- **Wave 2** — `forge-gitlab/07` (depends on 01; GitHub backend re-seated on
  `go-gh`, exec-`gh` forge path retired — parallelizable with 02/03).
- **Wave 3** — `forge-gitlab/08` (depends on 07 + 02; enumerated surface, no
  passthrough on either backend, checked `gh`/`glab` shell-exec ban).
- **Wave 4** — `forge-gitlab/11` (depends on 02 + 03 + 08; human-gated — the token-custody
  design behind 08's closure-to-zero: the last `gh` shell-outs onto the seam, a read-only
  `auditor` identity for the hardening guard, one enumerated hardening-read op, and the adopter
  docs that enumerate roles and permission sets gaining the auditor entry; design record
  `DR-forge-gitlab-11`, APPROVED at the gate on #857).
- **Wave 5** — `forge-gitlab/12` (depends on 11; the GitLab hardening-read kinds and the
  per-forge checklist rows).

Review-desk track (Track A — the open work; off the same backend, not gated by the pilot and
not gating it):

- **Wave 3** — `forge-gitlab/14` (depends on 02; the public-repo gate reads the resolved forge
  at every site, not two of three — **the head of the open path**) and `forge-gitlab/13`
  (depends on 02; board reads degrade per row, never per sweep — the class behind the one arm
  repaired on 2026-09-14). Parallelizable: one touches two command call sites and a deletion,
  the other the board's classifier; no shared surface.
- **Wave 4** — `forge-gitlab/15` (depends on 04, whose runbook it edits; the forge-binding key,
  the board-push credential, the source-pin lane).
- **Wave 5** — `forge-gitlab/16` (depends on 13 + 14 + 15; **human-gated** — the live review
  tick and the adopter-backlog close-out. The only brief that may mark the finish line reached).
- **Wave 5** — `forge-gitlab/17` (depends on `forge-gitlab/09` + `forge-gitlab/14`; the enforceable merge gate on GitLab Free —
  a marker discussion thread opened with every merge request, released by the reviewer's at-head
  approve, re-armed by request-changes or a new head, behind the Free-tier project setting
  `only_allow_merge_if_all_discussions_are_resolved`; `deskflip`'s GitLab gate keys on it and
  never on the Premium approval-rules route. Beside 16, not under it: 16 walks whatever gate is
  live on its day; 17 gives the Free-tier walk a server-side merge block to walk).

## Issue → brief map

Every open issue on this repo that touches the GitLab review desk, its adopter path, or this
stream, mapped to **exactly one** brief or explicitly out of scope with a reason. Standing rule
1 in the sequencing note above says a new GitLab review-desk issue is added here before it is
dispatched. Measured 2026-09-14 @ `2c67b34f`.

### On the open path

| Issue | Routes to | Note |
|---|---|---|
| #1071 | the plan itself | The notification that asked for this plan; its exit is this document, and `forge-gitlab/16` is the finish line it asked to see named |
| #1067 | no brief — **awaiting close**; the class is `forge-gitlab/13` | The instance was fixed by PR #1068, merged 2026-09-14 at 19:34Z, mid-authoring of this plan. The issue is still open and needs a close. `forge-gitlab/13` carries the CLASS the instance belongs to: four further whole-sweep returns in the same classifier, unguarded |
| #1066 | `forge-gitlab/14` — **head of the open path** | Two named sites, both live on main; a third (the release verb) found during this pass and carried into 14 as a ruling, not a sweep |
| #719 | `forge-gitlab/15` | The board-push credential subsection, and the removal of the pointer that sends the reader to the wrong section for the wrong credential |
| #896 | `forge-gitlab/15` | Its scan-scope half is already in the GitLab runbook; its source-pin half is not, and is 15's third section |
| #795 | `forge-gitlab/15` + `forge-gitlab/16` | The pin-grammar reader now accepts both field orders, so the residue is the runbook (15); the rest is field ceremony re-run by the walk (16) |
| #798 | `forge-gitlab/16` | Its verdict-write and escalation-filing halves are served on the current tree; its queue-label half is served in code and was recorded by the reporter as **unproven in production**, which is a walk row |
| #651, #652 | `forge-gitlab/16` | The front-door docs landed for the other forge's runbook; what remains is the GitLab-side confirmation the walk produces |
| #655, #667, #668, #671, #676, #677, #678, #642 | `forge-gitlab/16` close-out | All eight look delivered against the tree (see Track A's second tempting-but-wrong head for what was re-measured). They are routed to the walk because a code read cannot close a field report |
| #1054, #1056 | no brief — **awaiting close** | Fixed by PR #1060 and PR #1058, both merged 2026-09-14; the issues are still open and need a close, not work |
| #1091 | its 403 fix is a one-site defect, not a brief; the CLASS is `forge-gitlab/17` | Filed 2026-09-14 by a GitLab adopter cell on a Free project: the review-state read consults the Premium approval-configuration route first, gitlab.com Free answers 403 (not the 404 the tree degrades on), and the whole read fails closed — verdict verb aborts, board exits 6, flip refuses. The instance fix (treat 403 like 404 on that one route) is a sibling; `forge-gitlab/17` is the design that stops the flip depending on that route at all, by giving Free a server-enforced merge condition the desk drives |

### Out of scope for the review-desk finish line

| Issue | Why |
|---|---|
| #834, #838 | `forge-gitlab/08`'s closure-to-zero FAIL and `forge-gitlab/06`'s deferred live row. Track C and Track B respectively — already owned by existing briefs, and neither is on a review tick |
| #395, #305, #274 | The go-gh migration and shell-exec-ban reports. Owned by `forge-gitlab/07` and `forge-gitlab/08`; Track C |
| #992 | The run/gate-approval verb series. `forge-neutral` briefs 14–18, a different stream with its own plan |
| #823 | Board-generation mechanics (a hand-maintained table going stale after merge). Real, and this pass hit it — brief 10's row read `todo` while its work merged on 2026-09-11 — but the fix is a statusgen/skill change, not a GitLab one. The row is corrected here; the mechanism is not |
| #895 | The avatar family suite has no board-writer tile. A GitLab roster surfaced it, but it is icon tooling and no review tick calls it |
| #786 | Hardening the fleet provisioning script's credential handling. A `forge-gitlab/04` follow-up on the provisioning path, not the review path; it is a real security item and should not be folded into a conformance walk |
| #641 | A one-line umbrella ("Windows isn't POSIX") whose concrete instances are #667 and #642, both routed above. Nothing left to route separately |
| #611 | A formatting nit on a test file. No relation to this stream beyond the file's name |

**Interlock with `forge-neutral`.** That stream's `10` (a round trip driven entirely by desk
verbs) and `11` (install on a box with no forge CLI) are both `todo` and both sit beside
`forge-gitlab/16` rather than under it: 16 proves the **review** tick on GitLab, `forge-neutral/10`
proves the **full brief lifecycle** through the verbs, and `forge-neutral/11` proves the
**install**. None may be pre-credited from another. `forge-neutral/18` is in-progress and moves
statusgen off the forge CLI; it does not gate 16, and 16 does not gate it.
