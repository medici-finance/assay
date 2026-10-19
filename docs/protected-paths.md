# Protected verifier paths

Brief: verify-integrity/01. A worker PR can edit the brief's own Verify table, a CI workflow,
a client-side guard or a golden fixture in the same diff as the code those things check —
and, absent this check, nothing notices. `deskpathguard check` is the reviewer-side control
that closes that gap; this page states the protected set, the exemptions and the boundary of
what the check does and does not do, once, so the check, `pr-review-desk` and `verify-desk`
can all cite it rather than restate it.

## The protected set (initial)

A diff touches a protected path when it touches any of:

- a brief's `## Verify` table (detected from the diff itself — the hunk that changed a line
  between the `## Verify` heading and the next `## ` heading; see
  `tools/desk/cmd/deskpathguard/verifysection.go`)
- `.github/workflows/**`
- `.claude/guardrails/**`
- `tools/skillslint/**`
- any path under a `verify.d/**` scripted-rows directory
- golden fixtures under `**/testdata/**`

## The label rule

`deskpathguard check <owner/repo> <number>` applies the `wrote-to-the-test` label — and
prints a `gate-forced: wrote-to-the-test` line the status transition reads — when ALL of the
following hold:

1. the PR author is **not** the desk or verifier identity (the roster's `RoleBotIdentity`
   for those two roles) — this is a full exemption regardless of what the diff touches, so a
   desk- or verifier-authored regen/re-baseline PR is never labelled no matter which paths it
   carries;
2. the diff touches at least one protected path (above);
3. the diff also touches at least one file that is **neither** a brief file
   (`docs/streams/<stream>/brief-*.md`) **nor** a fixture file (`**/testdata/**`).

Condition 3 is what makes a pure-authoring diff (brief file(s) only, or a testdata-only
re-baseline) exempt on its own, without needing a separate identity check: a diff that touches
only brief and/or fixture files can never satisfy it.

A PR carrying a `regen:`-prefixed label is a fourth, independent exemption — it short-circuits
before the three conditions above are evaluated at all.

## Three states, never two

`deskpathguard check` reports one of `checked-clean`, `checked-failed` or `could-not-check` —
matching the verify-integrity stream's own rule. A diff that could not be read (a deleted or
unreadable head) is **`could-not-check`** (exit 6): it is never rendered as clean, and the
label is never applied on a could-not-check verdict. `checked-clean` and `checked-failed` both
exit 0 when the check ran — so a caller reads the `state=` line, not the exit code alone, to tell
them apart (`checked-failed` also prints `gate-forced: wrote-to-the-test`). The exit code is still
a signal for the write: without `--dry-run`, a `checked-failed` check whose label write fails
after the state line exits 5 (refused) or 6 (could-not-check) and prints no `applied:` line — the
label is **not** on the PR, and a caller who read only the `state=` line would wrongly conclude
the gate was forced. A non-zero exit with no `state=` line means the check did not run, which is
`could-not-check`.

## What this check does NOT do

- It does not run the house ruleset / branch-protection half — that is a repo-admin change
  out of this brief's scope, tracked separately, and lands once applied.
- It does not itself change what `deskflip` requires. A `wrote-to-the-test`-labelled PR is
  forced to `gate: human` at the STATUS TRANSITION (the brief's own status-machine, not the
  ready-flip) — per the 2026-09-09 ruling that a human-gate block sits at the status
  transition, never at the ready-flip. `deskflip`'s own conditions are unchanged; see
  `tools/desk/cmd/deskflip`'s test suite, which this brief adds no new case to (there is
  nothing there to change).
- The `## Verify`-table detection (condition 1's first bullet) is a best-effort heuristic
  over the diff text, not a byte-exact section parse — see the doc comment on
  `verifySectionTouched` in `tools/desk/cmd/deskpathguard/verifysection.go` for its bound.
  The lower layer that does not share this approximation is verify-desk's merge-base
  re-derivation, next.

## The re-derivation step (verify-desk)

`deskpathguard rederive --root <checkout> --brief <path/to/brief.md>` is verify-desk's
pre-change re-read on a labelled brief: it resolves the merge-base of `HEAD` and
`origin/main`, reads the brief's `## Verify` table as it stood there, and compares it against
the table at `HEAD`. Every row present at `HEAD` whose `#` cell also exists at the merge-base
runs with the **merge-base row's own command and expect** — an edit to an existing row's
command at `HEAD` is bypassed, never trusted. A row whose `#` cell has no match at the
merge-base is reported **`author-added`**, never `pass` — the row the worker itself added is
never treated as a witness of anything.

This is the single-point-of-failure's second layer (see the brief's `facts:`): the reviewer
check above is the ONE control until the house ruleset lands; this re-derivation is what
catches an edit the check missed, because it reads git history rather than a label.

### Recording closure after re-derivation

The independent verifier puts one live marker in the current Evidence run, before
its final verdict (full immutable Git object IDs, not branch names):

```text
Verification-Table: delivery=<sha> base=<sha> original=<sha>
```

`delivery` is the landed implementation commit on the observed HEAD's first-parent
chain. Other reachable commits do not establish mainline delivery. For a squash delivery, `base` is
its exact parent; for a two-parent merge, it is the parents' unique merge-base.
Before using a two-parent merge baseline, the evaluator also requires the first-parent
main acceptance table to equal the divergence table; otherwise independent
re-derivation is required.
`original` is the actual pre-change re-derivation revision. It must be an ancestor
of `base`, and its complete parsed Verify table must equal the table at `base`.
If main changed that table while the delivery was in flight, closure refuses;
retain the independent re-derivation and resolve the discrepancy explicitly.
Missing objects, root/octopus deliveries and ambiguous merge bases refuse too.

The current Verify section must still equal the delivered section. Every original
row ID remains mandatory, with its original Command, Expect and class; removing or
renumbering one refuses. An edited command needs a witness of the original command,
and an edited Expect cannot change the required original process exit. As with
ordinary witnesses, output assertions require independent inspection of the recorded
output hash and primary output. The audit does not reconstruct output from a hash.

Each added row's latest matching witness reports `author-added; execution exit=0
sandbox=unshare; no acceptance credit` (with the actual observed exit and sandbox).
It is a supplemental disposition, never a passing acceptance witness. Retain the
normal witness columns and primary execution evidence. A later pass claim does not
override this accounting. The execution command does not automatically rewrite a
current-table run into a re-derived run: the verifier must retain the original
commands and results before producing the closure record.

`verifyrun --check`, `brief --check-verified`, `verifyclosure`, the verified flip,
the reconciliation fold and lint use the same protected-table evaluator. Lint's
fail-first and row-strength checks retain the original acceptance table. Human row
gates, verifier authorship, admission and full closure lint remain required. The
marker proves local immutable object contents and ancestry; it is not a forge
identity attestation, permission or a replacement for independent verification.
This local history proof is relative to the checked HEAD: the independent verifier
still identifies the actual delivery; it does not bind that identity to a forge PR.
Run protected closure from a landed-mainline checkout; a branch that imports it
only through a merge can fail closed.

Coverage observes the same original acceptance rows. Its typed
`AcceptanceRevision` records the validated original definition revision; the
evaluator reads that actual historical Expect and compares it to the selected
original Expect. The witness keeps its execution revision and all content and
dependency checks. Invalid provenance creates an explicit held claim, including
when a separate pattern supplies other passing claims.

Retain previous Evidence runs. Each protected run contains its own marker and
witnesses, followed by its final strict `**VERIFY: PASS**` verdict. Closure,
audit, coverage, flip, reconciliation and lint select that same latest run, never
borrowing an earlier marker or witness. Duplicate live markers within one run, a latest failed or
malformed verdict, and unfinished live content after the last PASS refuse
closure. A caller with an already-selected run retains full Evidence through
the shared context adapter: earlier protection still requires current provenance,
while ordinary unmarked records audit only their selected run. Passing the selected
text alone loses that context and is not a valid protected closure input.

Marker classification and validation use the same canonical marker scan.
The shared selected-run audit preserves full-record classification: protected
records retain full history, ordinary records audit only their selected run, and
a selected string that would newly classify as protected refuses as ambiguous.
Ordinary canonical auditing can normalize selected text internally; no assumption
that comment removal is idempotent grants an exemption or older-row credit.

### Explicit human-approved acceptance transitions

The immutable-delivery rule above remains the default. A versioned brief may opt
into one later, separately landed command correction. This is a new human act,
not an authority inferred from merging a source edit, incrementing `version:`, a
label, a local record, an APPROVED review, or a bot relaying a person's words.

The bounded first implementation supports changing commands of existing original
rows while retaining their exact Expect, class, shell, order and obligations.
Changed commands must be single code spans without surrounding prose. Tables
must have explicit row IDs under a first-column `#` header. Original rows cannot
be added, dropped or renumbered. Supplemental rows cannot change or gain credit.
An unchanged row still owes its original command and Expect even if the delivering
worker edited it. Coverage records acceptance sources per row: original revision
for unchanged obligations, approved amendment for the corrected ones.

The amendment must be on the observed HEAD's first-parent chain after delivery.
A squash amendment has one parent. A two-parent amendment needs a unique merge
base whose acceptance table equals pre-merge main. Intermediate acceptance edits,
ambiguous bases and octopus merges refuse. The amendment increments the existing
brief-v2 `version:` by exactly one; the current version and Verify section must
still match it. Historical Evidence must survive unchanged through amendment and
remain a prefix of the current record. No earlier result receives new credit.

A human recognized by the existing external human-login roster posts an unedited
comment on a same-repository decision issue. That issue contains
`<!-- acceptance-transition: docs/streams/example/brief-01-example.md -->`.
The comment contains exactly the following payload, with actual full Git IDs and
all changed row IDs in table order:

```text
Approve acceptance-transition v1
repo=example-org/example-repo
brief=docs/streams/example/brief-01-example.md
delivery=<sha>
base=<sha>
original=<sha>
amendment=<sha>
rows=2,3
```

The existing ruling resolver reads the comment and its parent issue directly.
It verifies repository, issue membership, unedited timestamps and the mapped
non-bot author. The transition owner additionally requires that exact payload.
Missing, edited, stale, bot-authored, cross-brief or unverifiable approval refuses.
The human approves a specific immutable amendment, not arbitrary future changes.

A coordinate-only descriptor contains these two literal lines:

```text
Verification-Table: delivery=<sha> base=<sha> original=<sha>
Acceptance-Transition: amendment=<sha> approval=https://github.com/example-org/example-repo/issues/42#issuecomment-123
```

The independent verifier can execute it with:

```sh
statusgen verifyrun --root . --brief docs/streams/example/brief-01-example.md \
  --acceptance-record transition.txt --acceptance-forge
```

Keep the descriptor outside the clean execution checkout. The host checks admission
and the immutable approved plan before executing. It runs every original obligation
(including unchanged original commands) plus supplemental rows, emits actual output
hashes and full immutable execution revisions, and labels supplementals without
acceptance credit. The verifier still inspects primary output and appends the strict
final verdict. Execution alone never creates an independent closure verdict.
Every original row needs a fresh witness at or after amendment; the shared audit
requires committed independent provenance and preserves source/admission checks.
For example, `sh check.sh` may become `sh check.sh --no-pull` with identical Expect.
This does not authorize pulling images or network access during execution.

`--acceptance-forge` permits read-only approval lookups; it grants no approval.
The target is printed before first contact. It is available on the main statusgen
lint/coverage/status paths and `brief`, `verifyrun --check`, `verifyclosure`,
`verifyflip`, and `reconcile`. Use the same flag on `deskevidence` (including
`flip`): its production child audits, outcome check and lint receive it explicitly
in argv. No environment variable enables these reads. Without opt-in, transitioned
records remain held while unamended records retain their existing offline behavior.
Each authoritative evaluation corroborates the ruling afresh; there is no local
approval cache or supplied approval boolean.

Before promoting a transitioned brief, the adopting repository must upgrade its
pinned binary and explicitly enable `--acceptance-forge` on **every authoritative
CI invocation** that will read that record: lint, coverage/status regeneration,
model auto-flip, reconciliation and their retry commands. Supply the existing
`GH_TOKEN`/`GITHUB_TOKEN` read credential and external mapped-human configuration
through that workflow's established credential boundary; do not expose privileged
credentials to untrusted branch code. The repository's workflow approval is the
opt-in. A desk command's flag does not change later CI jobs. An old pinned binary
cannot receive this flag, and a default offline CI invocation will hold the record
or fail lint. Verify the upgraded workflow can corroborate the exact ruling before
committing a final transition verdict. This source change does not activate any
repository's workflows or provide any actual amendment approval.

Transition execution inside `--in-container` or with cross-repository admission is
unsupported and refuses. The fenced runner gains no network access. Chained
transitions, row retirement, changed assertions/classes and table-layout migrations
also remain unsupported. Human closure gates, independent review and release/pin
requirements remain separate; this mechanism performs none of those actions.
