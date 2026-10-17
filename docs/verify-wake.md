# Verify wake: when a held brief is re-run

A verify run that does not pass (`verify-fail` or `blocked`) lands an outcome record that
carries a wake receipt (`wake_schema: verify-wake-v1`). `deskevidence --outcome-record`
derives the receipt when it lands the record:

- `inputs` holds one `file:<path>` key per path in the brief's `## Context` `files:` list that
  exists at the admitted observation's Source commit. A listed directory contributes every
  file under it. The keys also include the brief itself, hashed as its Evidence lands, and
  the tool version. The record's `sha` names that Evidence commit, which may follow a repair;
  hashes of observed content ensure an existing-file content repair wakes the failed work.
  A change to declared paths, path kinds or a declared directory's file membership between
  Source and Evidence requires fresh independent dispatch and verification at the current
  target branch before recording an outcome. The old run cannot provide hashes for new files;
  existing input-coverage checks remain enforced.
- `blocker_kind` and `blocker_ref` come from the caller and are required. `blocker_ref` must be
  a reference: `#N`, `<owner>/<repo>#N`, or an issue, pull-request or CI-run URL. A
  placeholder such as `to file` is refused with exit 5. A run URL is accepted, but a run has
  no open or closed state, so the planner always reads it as could-not-check.
- `repo` is set to the repository the record lands in when the caller gave none, so a bare
  `#N` is read in that repository.

A record that already carries a `verify-wake-v1` receipt is validated as written and is not
derived again. It lands only when the receipt is complete: a `receipt_id`, a `blocker_kind`
and a `wake_predicate` from their closed sets, a reference `blocker_ref`, and the field its
predicate needs (`inputs`, `deadline` or `recheck_reason`). Otherwise it is refused with exit
5, naming the field, and nothing lands. For an input-change receipt, supplied input hashes
must also match the admitted Source and landed Evidence brief. Deadline and explicit-action
predicates retain their existing contracts.

`verifyloop plan` then decides, per brief, whether its latest record is worth another run. It
makes two independent reads, and each one can return could-not-check:

- **Inputs.** Every declared input is hashed again at the current tree and compared with the
  receipt.
- **Blocker issue.** The issue named by `blocker_ref` is read through the forge as the verifier
  App. A bare `#N` is read in the record's `repo`, or, when the record has none, in the plan
  root's own repository (its `origin` remote). The read is could-not-check, never "closed", in
  these cases: there is no token, the token is rejected, the reference is not an issue or
  change, the state is unknown, or no repository can be resolved.
- **Repository set.** A reference whose repository is outside the configured repository set
  (the same set the board reader enforces) is could-not-check and is never read. No credential
  is requested for it. Before the first read in each repository, the plan prints that
  repository to stderr. `plan --no-forge` makes no forge call, so every blocker read is
  could-not-check.

## The hold rule

First match wins.

| Receipt | Inputs | Blocker issue | Result |
|---|---|---|---|
| explicit-recheck receipt newer than the hold | any | any (not read) | dispatchable: "recheck: <reason>" |
| absent or incomplete | — | — | dispatchable once: "classification pass, writes a receipt" |
| complete | changed | any | dispatchable: "inputs changed: <path>" |
| complete | unchanged | closed | dispatchable: "blocker closed: <ref>" |
| complete | unchanged | open | **wait** — next actor from the blocker class |
| complete | could-not-check | any | held, surfaced as could-not-check |
| complete | unchanged | could-not-check | held, surfaced as could-not-check |

The explicit recheck is the first row so that it overrides every hold below it. A recheck
receipt older than the hold has been superseded by the hold.

Every held line names the blocker reference, the wake condition and the next actor (the
blocker class decides who acts next: `implementation` is the worker, `check-definition` is the
brief author, `human-action` is a human, `environment` is the operator, `unknown` is the
verifier). The plan ends with one summary line:

```
verify-desk plan: wait=<n> dispatchable=<n> could-not-check=<n>
```

`wait` counts briefs held on an open blocker or awaiting an Evidence PR merge. Briefs held because a read could not be made are
counted under `could-not-check`, never under `wait` or `dispatchable`.

Receipts whose predicate is `referenced-action-completed` or `declared-deadline-reached`
keep their own evaluation, which is unchanged by this rule.

## Pending Evidence PRs

The plan joins both single-root and configured-root queue items to open changes through
the same allowlisted verifier-role forge source as blocker reads. A matching brief is
listed as `awaiting-merge #N` only when the change has a pinned, independent verifier author,
a same-repository immutable head, a complete changed-file read, and real Evidence carrying
the exact closed, never-edited, dispatcher-stamped pre-work attestation. Fenced examples,
unrelated briefs, unbound actors and incomplete cross-repository associations establish no hold.
Missing files, unreadable populations and unverifiable provenance are could-not-check.
`--no-forge` cannot establish absence of pending Evidence, so its queue is could-not-check.

Without a failed/blocked wake receipt, pending Evidence holds only at its exact attested
source (both sources for a cross-repository run). With a receipt, the existing writer
re-derives the original observation's input map; narrowed or substituted maps are rejected.
The existing hold evaluator then decides inputs, blocker, deadline, action, explicit recheck
and partial-row wakes. Only the brief's landed Evidence hash is substituted after both
current and pending plans match the attested plan. Other input keys retain their existing
meaning, including the conservative whole-source key for a cross-repository deliverable.
Declared input membership changes also wake. A newer landed outcome supersedes an older
pending receipt. This scheduling state grants no acceptance or lifecycle authority.

Consumers: `cmdPlan` renders the shared `VerifyLoop.SelectQueue` result; its single-root and
multi-root arms join pending Evidence once after scanning. The engine adapter refuses a
pending or unreadable item at `Dispatch`, before interim or native runners. The two explicit
`verdict` executors use the board-only `scanAwaiting` to execute a requested fresh observation;
they do not discover PRs or turn pending scheduling evidence into a verdict. The stream scan
and outcome readers retain their existing board and record responsibilities.
