# auto-triage — scoping document

**Status:** draft — proposed 2026-09-16; not approved. This stream is scaffolded
`status: parked` precisely because the approval this header would otherwise assert has
not happened. Approval is a human act, stamped by the merge of the pull request that
flips this header to `approved` and the README to `status: active`, per
`spec/lifecycle-v1.md` §8.4. Nothing here fakes it.

<!-- Note on the header value: the direction that commissioned this doc asked for
     `**Status:** proposed`. `proposed` is not a state statusgen's §8.1 lifecycle grammar
     classifies (it recognizes draft / approved / routed), so it would read as an
     unclassified legacy document and emit a NOTICE. `draft` is the recognized
     not-yet-approved state, and it is exactly what a `parked` stream "may cite" under the
     `stream-source` lint (statusgen/streamcap.go). So this doc is `draft` — the same
     meaning as "proposed", in the grammar the tools actually read. -->

## 1. The problem, stated from observation

This repository runs an all-stop discipline on purpose. When a whole-tree gate reddens —
a red `main`/post-merge check, or an ambient defect on `main` that every open PR inherits
through its own gate — the intended response is that work stops until someone looks. That
is not a bug to design away. A whole-tree gate going red is the signal that *something
somewhere is wrong*, and the all-stop is what makes the signal impossible to ignore.

**The direction that commissioned this stream is explicit on that point.** Faced with the
seam where one unrelated nit reddens every open PR (a gofmt drift in one file failing
Verify-table rows across unrelated PRs), the authoring direction states: *do not scope the
gate down to the PR's own diff.* That is the direction this stream was commissioned under,
not a ruling on record elsewhere in this repository — a sweep of the issues this stream
cites (#611, #1119, #612, #880, #536) found no comment recording it as a ruling, so it is
cited here as the commissioning direction, checkable on its own merits (the DR below argues
it independently), not as an appeal to a prior authority. The all-stop is desired. What is
missing is not a narrower gate — it is an **automated response** to the gate the moment it
fires.

Today that response is manual. Each time a whole-tree gate reddens on ambient churn,
a human or a desk hand-triages it: reads the failing log, finds the culprit check and the
specific failing `file:line`, decides whether the fix is mechanical or needs judgement,
files the bug, and either opens a fixing PR or routes it. This is toil, it recurs, and it
is exactly the same shape every time. The public record of that toil:

- **#611** — a gofmt nit in one test file (pre-existing on `main`, in no live brief's
  diff) reddened unrelated Verify-table rows across PRs; hand-filed.
- **#1119** — gofmt drift in one file "trips unrelated Verify-table rows"; hand-filed,
  same class as #611.
- **#612** — a load-induced timing flake fails the whole-module `go test` on the first
  attempt (re-run passes); a judgement call, not a mechanical fix.
- **#880** — stale remediation text on `main`; a mechanical text fix.
- **#536** — a duplicated reducer; a mechanical consolidation.

The same manual pattern ran again the day this stream was scoped: two `main`-reds (a stray
scratch file reddening a boundary gate, and backtick paths reddening a link check) were
each hand-triaged into a filed issue plus a fixing PR. That hand-pattern — read the gate,
name the culprit, file, fix-or-route — is what this stream automates.

## 2. The principle (what this stream does and does NOT do)

**KEEP the all-stop gate. Automate the RESPONSE.** These are the load-bearing invariants;
every brief inherits them:

1. **Never weaken, narrow, or skip a gate.** No brief here scopes a whole-tree gate to the
   PR's own diff, silences it, or edits gate configuration to make a red go green. The
   automation *responds to* the signal; it never dims it. (This is the same carve-out the
   PR-review discipline already states: greening a check by removing or weakening a control
   is never the fix.)
2. **A red whole-tree gate triggers an automated response** with four steps:
   1. **Identify the culprit** — parse the failing gate's output into `check` +
      the specific `PROBLEM`/failing `file:line`.
   2. **Classify** the culprit as **mechanical** (a deterministic, low-judgement fix —
      gofmt, a stray file, a stale string) or **judgement** (needs a human/design call —
      a flake, a duplicated abstraction, an ambiguous failure).
   3. **Mechanical → act:** file the bug issue AND open a *fixing draft PR* automatically.
   4. **Judgement → route:** file the bug issue AND route it with a recommended default,
      never a silent auto-fix.
3. **A red gate never goes invisible.** A whole-tree gate that reddens and receives *no*
   responder action within a bounded window is itself escalated. (A red gate that went
   silent is a recorded failure mode this stream exists to prevent — an alert that never
   fired is worse than no alert, because it manufactures false confidence.)
4. **The human still merges every fix.** The automation opens **draft** PRs and files
   issues; it never merges, never flips a PR ready, never self-approves. Merge authority is
   unchanged and remains the human's.
5. **The automation acts only on trusted-provenance CI output.** A `push`-to-`main` or
   `schedule`-triggered run is eligible input; a `pull_request`-triggered run is not — its
   log/diagnostic text is contributor-authored on a contribution the repository does not
   control, and the automation refuses it wholesale rather than partially trusting it (see
   `DR-auto-triage`'s "input-authorization boundary" section).

## 3. Why a new stream (and not an existing one)

Checked against the live corpus on `origin/main` @ `e9fa19d3` (2026-09-16):

- **`desk-supervision`** supervises the *drain-engine runner* — stall detection, kill,
  mid-run eligibility reconcile, per-class caps for live desk workers. It is about a
  running agent going silent, not about a CI gate going red. Adjacent, not a home.
- **`statusgen/brief-05`** already treats `main-red` as a machine-derived **ordering key**
  for the critical tier ("statusgen already knows CI-red"). That is a *consumer* of the
  signal for prioritisation — it ranks a `main-red` fix once one exists; it does not
  produce the triage. This stream produces what that tier then ranks.
- No stream owns "automate the response to an all-stop CI signal." So: a new stream,
  slug `auto-triage`, scaffolded `status: parked` with this `draft` spec until a human
  approves it (the not-yet-approved representation that keeps the `stream-source`/board
  lint clean without faking approval — a `parked` stream is exempt from the active-stream
  approved-spec requirement).

## 4. Components

| # | Component | Path (proposed) | Brief |
|---|-----------|-----------------|-------|
| 1 | Culprit identifier — parse a failing whole-tree gate into `check` + `file:line` + mechanical/judgement class | `tools/autotriage/` (parser + classifier, planned) | 01 |
| 2 | Mechanical responder — file the bug + open a fixing draft PR, autonomously | `tools/autotriage/` (responder, planned) | 02 |
| 3 | Judgement responder — file the bug + route with a recommended default | `tools/autotriage/` (router, planned) | 03 |
| 4 | Never-invisible watchdog — escalate a red gate that no responder acted on within N minutes | `.github/workflows/` + `tools/autotriage/` (planned) | 04 |

*(Exact paths, binary vs workflow split, and the trigger surface are each brief's to fix;
the table records intent, not a committed layout.)*

## 5. What the culprit identifier can and cannot see (the real head of the work)

The response is only as good as step 1, and step 1 is bounded by what CI actually exposes.
Verified against the live workflows @ `e9fa19d3`:

- **Readable, machine-parseable:** the `ci` workflow's `build-test`, `plugin-shell-suites`
  and `skillslint` jobs, plus `truth-suite`, `changelog-check`, `pin-consistency`,
  `plugin-drift`, `assay-statusgen` — their logs name the failing file and the diagnostic
  (gofmt diff, `go vet`/test failure, a statusgen `PROBLEM` line with a `[tag]`). These are
  where the culprit + `file:line` can be extracted deterministically.
- **Deliberately opaque:** the `leak-sweep` commit status (posted by `leaksweep-pattern`).
  Its public surface says only "withheld content detected" — the matching token and
  `file:line` are withheld *by design*. The identifier therefore **cannot** produce a
  mechanical fix for a `leak-sweep` red; it can name the check but not the culprit, so a
  `leak-sweep` red **always** routes to the judgement responder, never the auto-fixer, and
  the automation never guesses a withheld token from the diff.

**This is the true head of the critical path.** The tempting first step — "wire the
auto-fixer" — is dead on arrival without a classifier that (a) parses heterogeneous CI
output and (b) knows which reds it must *not* attempt to fix. Brief 01 is that classifier,
and it is where the mechanical/judgement boundary is drawn.

## 6. Boundary conditions (non-goals)

- **No gate is weakened.** Restated as an explicit Definition-of-Done line in every brief:
  the deliverable must not narrow, silence, or edit any gate to make a red go green.
- **No merge, no ready-flip, no self-approval.** The automation's write authority is
  bounded to: filing an issue, opening a *draft* PR, posting a routing recommendation, and
  escalating. Merge and ready-flip stay human.
- **No auto-fix of anything the identifier cannot fully pin.** A red whose culprit cannot
  be resolved to a concrete `file:line` + a deterministic fix is routed to judgement, never
  auto-fixed. `leak-sweep` is the standing example.
- **No new gate.** This stream adds a *responder*, not another all-stop.
- **The classifier is honour-bound conservative.** When mechanical-vs-judgement is
  genuinely ambiguous, it classifies **judgement** (route to a human), never mechanical.
  A false "mechanical" opens a wrong draft PR; a false "judgement" merely asks a human —
  the asymmetry is deliberate.
- **The path the mechanical responder may touch is an allow-list, not a deny-list.**
  Brief 02's refused-path set is an enumeration (CI/repo config, ownership files, the gate
  tooling itself, instruction surfaces other automation reads, its own arming config),
  checked against the CANONICALIZED path — never a single named directory checked against
  the literal string.
- **The mechanical remedy set is closed.** Brief 02 may apply exactly one of three named
  deterministic transforms (gofmt, stray-file deletion, exact-substring replace); anything
  else is refused to the judgement responder rather than freely generated.
- **Parsed CI text is sanitized before republication.** Every responder and the watchdog
  truncate, fence, and render inert any CI-derived text before it enters a filed artifact —
  it is data quoted from a log, never live content.
- **Filing/opening is bounded by a ceiling, not only by dedupe.** Each responder enforces an
  absolute cap on its own open artifacts and a per-window rate, failing closed to escalation
  rather than to further authoring when either is hit.

## 7. The human-gate posture and its decision record

Briefs 02, 03 and 04 each let the automation take an **autonomous outbound action** (open a
draft PR, file an issue, post a routing default, escalate) *without a fresh human act per
action*. That widens what the automation may do on its own. Widening standing autonomous
authority is a design-approval decision, not something a model signs off on its own reading
of a mechanical diff — so those three briefs are `gate: human` and each cites the
design-decision record **`DR-auto-triage`** (`docs/streams/decisions/DR-auto-triage.md`),
authored here as **PROPOSED** (no ruling recorded). Brief 01 (the identifier) is read-only
classification and is `gate: model`.

## 8. Sequencing

Four briefs in three waves; see [README.md](README.md) for the status table, dependency
waves, and critical path. The pacing item is **brief 01** (the culprit identifier /
classifier): every responder builds on its output, and it is where the
mechanical/judgement boundary — and the `leak-sweep`-is-never-mechanical rule — is
decided. Its head was verified against the live workflows (§5): the readable jobs expose
`file:line`, and the one opaque gate (`leak-sweep`) has a defined, non-guessing fallback,
so 01 has no hidden upstream blocker.
