# forge-neutral — conformance round-trip report

**Brief:** [forge-neutral/10](brief-10-conformance-round-trip.md) · **Run date:** NOT YET RUN —
supervised live run pending (#1553) · **Substrate:** NOT YET RUN — supervised live run pending
(#1553).

> **This file is a SKELETON. No row in it has been run, and no row is credited.** Every result
> cell below reads `NOT YET RUN — supervised live run pending (#1553)`. The brief's human gate
> was answered "Approve as briefed" on #1553; the live round trip is to be performed as a
> SUPERVISED human run against the pilot deployment, and the operator of that run replaces each
> result cell with what the run actually showed. What IS here is the row set, the verb
> invocation each row expects, and the criterion each row is judged by — so the run fills cells
> rather than inventing rows, and so a reader can see before the run which rows exist and could
> therefore be omitted.
>
> **Reading rule for anyone verifying brief 10.** The tables below already satisfy Verify row 1's
> table count by construction. That count is NOT evidence of a run. None of Verify rows 1–5 may be
> credited while any result cell in this file still reads `NOT YET RUN`, and a row still reading
> that phrase after the run is a row that was not run — it becomes `COULD-NOT-CHECK`, never PASS.

**What this file is, once filled.** The conformance record the brief asks for, in the shape of
the 2026-09-02 pilot report ([`../forge-gitlab/pilot-report.md`](../forge-gitlab/pilot-report.md)):
every claim cites a verb invocation and its exit code, or an endpoint and its status code, read off
the live deployment on the run date. Where the instrument did not look, or could not, the row says
`COULD-NOT-CHECK` — never PASS.

**The baseline this run is measured against** — the pilot's §2, quoted: *"Every write in §1 was a
hand-built `curl` call against REST v4 or a raw `git` push. The desk verbs the GitHub lane uses —
`deskpr`, `deskpost`, `deskflip`, `deskfile`, `deskevidence` — have no GitLab backend, so none of
them was reachable for any step."* This run is the one where that sentence stops being true, or
records exactly where it does not.

**Verdicts used below.** `PASS` · `FAILED` · `COULD-NOT-CHECK`. In the round-trip table a step
completed by anything other than the named verb — a hand-built API call, a forge CLI, a raw push to
a protected branch — is not a PASS whatever the outcome on the project: it is a deviation (§5) and
the step's verdict is `FAILED`. In the negative-path table a refusal that did not fire, or that
another verb routed around, is `FAILED`, never omitted.

**Naming.** As in the pilot report: the deployment's group and project are named by numeric id,
service accounts by role plus numeric user id. No token value, token file path content, or private
name appears in this file.

## 0. The substrate

| Thing | Live read (expected instrument) | Value |
|---|---|---|
| Forge, edition, tier, version | the version endpoint and the group's plan, read with the owner credential (read-only) | NOT YET RUN — supervised live run pending (#1553) |
| Group | group read by numeric id: visibility, token-lifetime policy | NOT YET RUN — supervised live run pending (#1553) |
| Tracking project | project read by numeric id: visibility, default branch, merge method, CI config path | NOT YET RUN — supervised live run pending (#1553) |
| Human owner | project member list: the one member at Owner (50) / Maintainer (40) or above | NOT YET RUN — supervised live run pending (#1553) |
| Role service accounts | project member list + each role credential's own user read: role, numeric id, access level, `bot: true` | NOT YET RUN — supervised live run pending (#1553) |
| Protected default branch | protected-branches read: push = No one, merge = Maintainers (human only), force-push off | NOT YET RUN — supervised live run pending (#1553) |
| Forge resolution | the tracking root's forge configuration as the verbs resolve it (verb output naming the resolved forge and host) | NOT YET RUN — supervised live run pending (#1553) |
| Trust roster | the roster entries the verbs resolve for each role, forge-qualified (`<role>=gitlab:<username>:<id>`) | NOT YET RUN — supervised live run pending (#1553) |
| Desk-tools and statusgen build | `<verb> --version` for every verb used, and `statusgen --version`: release tag and source SHA | NOT YET RUN — supervised live run pending (#1553) |
| Leak gate on merge requests | the project's CI configuration carries the pipeline-side leak-sweep job ([`gitlab-ci-half.md`](gitlab-ci-half.md)) | NOT YET RUN — supervised live run pending (#1553) |

## 1. The round trip — one brief `todo → done`, verbs only

The brief carried is a throwaway `gate: model` brief in a pilot stream of the tracking project,
whose single Verify row is a file-existence check on the deliverable. Every forge write below is a
desk verb invocation; the only acts that are not are the human's merges, which are the human's by
design and carry no verb.

| Step | Actor (role, id) | Mechanism — verb, arguments, exit code | Artifact id | Timestamp (UTC) | Verdict |
|---|---|---|---|---|---|
| R1 · dispatch + claim | desk | `deskdispatch <key> --repo <group>/<project>` under the desk loop, which runs `deskclaim-ref acquire <key>` as its claim step; the run records the ref the claim actually landed at (`deskclaim-ref` acquires under `refs/dispatch/`, while [`claim-shape.md`](claim-shape.md) chose `refs/heads/dispatch/` and its create-side cutover is pending) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R2 · claim visible | desk | `deskclaim-ref show <key>` (read) — the claim R1 took is present on the remote | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R3 · deliverable + draft change | worker | `deskpr create --title <T> --body-file <F>` under the worker loop, from the worker's clone — pushes the feature branch, opens the change as a draft (`Draft:`) and opens its merge-hold thread | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R4 · leak-gate verdict | CI | the pipeline-side leak-sweep job's result on the change's head (read, not written) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R5 · reviewer verdict | reviewer | `deskpost review <group>/<project> <iid> --verdict approve --head <full sha> --body-file <F>` under the review loop | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R6 · ready flip | desk (review role) | `deskflip <iid> --repo <group>/<project>` under the review loop — requires R5's verdict at the current head | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R7 · human merge | the human owner | the owner merges the change into the protected default branch (no verb — merge is the human's) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R8 · Verify row on merged default branch | verifier | `statusgen verifyrun` for the brief, in a fresh clone checked out at the R7 merge commit — never the change's own head | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R9 · Evidence row + status flip | verifier | `deskevidence <group>/<project> <default branch> --evidence-file <brief path> --root <clone>` under the verify loop — with the default branch not writable, the Evidence travels as its own draft change on a side branch; the run records the side branch and change id | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R10 · board regeneration | board-writer (or the identity actually used — recorded) | `statusgen --root .` on R9's branch, the regenerated board carried as a change STACKED on R9's change (pilot D-8) — the run names the verb that carried it and the identity it acted as, or records a deviation | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R11 · verdicts on R9 and R10 | reviewer | `deskpost review … --verdict approve --head <sha>` on each change; `deskflip <iid>` on each | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R12 · human merges of R9 then R10 | the human owner | merge R9's change, then R10's (retargeted onto the default branch once R9 lands) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R13 · claim release | desk | `deskclaim-ref release <key> --repo <group>/<project> --token-file <role credential file>` — the claim is deleted through the verb | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| R14 · done | the human owner | the brief's `verified → done` transition on the default branch, by the lane the deployment's gate names | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |

### The round trip in one read

`git log` on the tracking project's default branch, in a fresh clone taken after the last merge:
every content commit a distinct role service account, every merge commit the human.

| Commit | Author | What |
|---|---|---|
| NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |

## 2. The negative-path walk — the writes the verbs refuse

Each attempt is made with only the verbs available and a role credential present. Expected: the
verb refuses, with its exit-code class and a message naming the reason AND what would make it
succeed. A refusal that another verb routes around is a succeeded write and is recorded `FAILED`;
so is a refusal whose message does not say what would make it succeed.

| # | Attempted write | Verb invocation (identity) | Expected refusal | Observed exit | Observed message | Routable by another verb? | Verdict |
|---|---|---|---|---|---|---|---|
| N1a | a write to the default branch | `deskpr create` with the clone checked out on the default branch (worker) | refused (5) before any push; message names the default branch | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N1b | a write to the default branch | `deskevidence <group>/<project> <default branch> …` without the verify loop's default-branch opt-in (verifier) | refused (5) before any write; message names the opt-in | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N1c | a write to the default branch | `deskgit push` with the default branch checked out (worker) | refused (5); message names the default branch | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N2 | a merge | `deskmerge` against an approved, ready change (desk) | refused; merge is the human's | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N3 | a ready flip with no at-head verdict | `deskflip <iid>` on a draft change with no reviewer verdict at its current head (review loop) | refused (5); message names the unresolved at-head verdict | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N3b | a ready flip after a push moved the head past the verdict | `deskflip <iid>` on a change whose verdict is pinned to a superseded head (review loop) | refused (5); message names the stale head | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N4a | a comment posted with no loop identity | `deskpost comment` with the loop identity unset | refused (5) before any write; message names the missing loop identity | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N4b | a comment posted as an unrecognised loop identity | `deskpost comment` with an unknown loop name | refused before any write; message names the unknown loop | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N4c | a comment posted as an identity the roster binds to the other forge | `deskpost comment` with the role's roster entry qualified `github:` on this GitLab project | refused (5); message names the forge disagreement | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N4d | a comment posted as an identity the roster does not bind | `deskpost comment` with the roster carrying no binding for the acting role | refused before any write; message names the missing binding | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N5a | an operation the resolved forge cannot serve | a `deskboard` view whose read the GitLab backend declares a gap (a ref comparison) | could-not-check (6, or a per-row could-not-check the run quotes); never an empty success | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| N5b | an operation the resolved forge cannot serve | a `deskboard` view needing open-change search on GitLab | could-not-check (6, or a per-row could-not-check the run quotes); never an empty success | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |

## 3. The boundary — measured, not asserted

What a session holding a role token file could do if it reached the forge directly rather than
through a verb, beside what the verbs refused for the same operation. This is the stream's thesis
as a measurement: the left half is the forge's own server-side answer to the role credential; the
right half is the verb's. A write the forge ALLOWS for a role credential is not a verb failure —
it is the size of the gap the verbs close client-side, and it is recorded as such. Every probe
write targets a throwaway branch or change created for the probe and removed after it.

| # | Operation | Role credential | Direct forge call — endpoint | Forge status observed | Verb for the same operation | Verb result (exit) | Verdict |
|---|---|---|---|---|---|---|---|
| B1 | push to the default branch | worker | git push over HTTPS to the default branch | NOT YET RUN — supervised live run pending (#1553) | N1's verb | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B2 | merge an approved change | worker, reviewer, desk, verifier, board-writer | merge endpoint on a throwaway approved change, per role | NOT YET RUN — supervised live run pending (#1553) | N2's verb | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B3 | approve one's own change | worker (the change's author) | approve endpoint on the worker's own throwaway change | NOT YET RUN — supervised live run pending (#1553) | `deskpost review` APPROVE as the author | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B4 | un-draft a change with no verdict | worker | update endpoint stripping the draft marker | NOT YET RUN — supervised live run pending (#1553) | N3's verb | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B5 | comment as the role, outside any verb's gates | worker | notes endpoint on a change | NOT YET RUN — supervised live run pending (#1553) | N4's verb | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B6 | delete another role's claim | worker | branches delete endpoint on a claim branch the worker does not hold | NOT YET RUN — supervised live run pending (#1553) | `deskclaim-ref release` for a claim held by another session | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| B7 | change protected-branch settings | every role | protected-branches endpoint (write) | NOT YET RUN — supervised live run pending (#1553) | no verb offers it | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |

## 4. Brief Verify rows

| Row | Command | Expect | Result | Citation |
|---|---|---|---|---|
| 1 | `grep -c '^[\|] '` over this file | ≥ 20, the four tables present — and every result cell filled from the run (see the reading rule above) | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 2 | `grep -c 'curl'` over this file, read as a list | only the baseline citation and the boundary table | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 3 | `grep -c 'COULD-NOT-CHECK'` over this file | a stated count, argued if zero | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 4 | §1's `Mechanism` column | every row names a desk verb and an exit code; any endpoint is a §5 deviation | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 5 | §2 | one row per enumerated refusal with exit-code class and message; a succeeded refusal recorded FAILED | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 6 | `statusgen --root . --lint` in a clone of the tracking root on the merged default branch | `LINT: PASS`, exit 0, Evidence-actor reports the row BACKED | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 7 | the brief's Evidence row on the merged default branch | committed by the verifier identity; witness `Runner` names that identity with source `forge identity` | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 8 | `git ls-remote origin` filtered to BOTH claim namespaces (`refs/dispatch/*` and `refs/heads/dispatch/*`), after the run | the round-tripped brief's claim is absent from each | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 9 | the leak gate's verdict on R3's change | present and readable, or the pipeline job's result cited | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |
| 10 | `statusgen --root . --consumers --brief forge-neutral/10` | exit 0 | NOT YET RUN — supervised live run pending (#1553) | NOT YET RUN — supervised live run pending (#1553) |

## 5. Deviations

NOT YET RUN — supervised live run pending (#1553). Each step the run could not complete through
its named verb is recorded here as `D-n`: what was attempted, the verb's exit code and message, and
what was NOT done instead. The run does not reach for a hand-built call to finish the walk; a step
with no verb path is a deviation and the walk records it and moves on.

## 6. Token hygiene during this run

NOT YET RUN — supervised live run pending (#1553). The pilot report's §6 discipline is inherited:
no token value printed, passed in an argv, or written inside a checkout; per-role credentials
`0600` outside every checkout; the owner credential used for reads only. The operator records here
how each of those held on the run, including for the §3 probes, which are the one place a role
credential reaches the forge outside a verb.
