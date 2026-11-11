# Consumer checks after delivery

A post-merge check must name both endpoints of the delivering change. The ordinary
`statusgen --consumers` mode judges new branch claims and deliberately leaves inherited
entries unchecked, so an Evidence edit cannot put an old implementation on trial against
a documentation-only diff. That protection remains in force.

For a historical delivery use the current statusgen binary with `--consumers`, a named
`--brief`, `--delivery` set to the full immutable delivering commit, and `--base` set to
its full first-parent SHA. Both pins belong in the authored Verify command. The checker
requires complete local history and delivery ancestry, reads the delivered tree and routes,
and evaluates inherited claims against that delivery's own diff. It runs no old binary,
executes no snapshot code and opens no network connection. Missing history, an ambiguous
brief, a noncommit/moving delivery spelling, an invalid parent range, unsafe links/submodules or
archive-omitted blobs are could-not-check (exit 2). With both pins on the command line it
does not choose a historical commit for the author: identifying the actual delivery remains
an independently reviewed claim.

Every evaluated delivery writes a separate `PRODUCER-DELIVERY: COULD-NOT-CHECK`
qualification to stderr. The stdout report stays byte-identical, preserving its header
and pinned report hashes. The routing counts on stdout and exit status concern declared consumer routes
only. In particular, `follow-up` corroborates that a target brief exists and references
back; it does not require that the selected diff contains the producer's implementation.
Two different producer deliveries can therefore corroborate the same follow-up routes.
Even `fixed-here` establishes path changes, not producer identity or implementation
completeness. Independently inspect the actual delivery mapping and relevant changes
before using these results as acceptance evidence. This limit also applies when the
delivery pin is derived from committed Evidence below.

## Unchanged links inside the delivered tree

An unrelated symlink no longer makes every historical consumer check unreadable.
Only an unchanged link whose complete target chain names existing entries inside
that same immutable snapshot may resolve. Changed links, absolute or escaping
links, broken, cyclic or excessive-depth chains, submodules, and links on board inputs (`docs`,
`docs/streams/**` and `docs/archive/**`) remain could-not-check. Link targets come
from the delivered archive, never the current checkout or a file outside it.

The bounded archive reader writes regular files exclusively and never creates
filesystem links. It checks the complete entry roster, rejects special or duplicate archive
entries and missing blobs (including `export-ignore` omissions), and resolves path
components in order so a parent step after a directory alias cannot hide an escape.
Every chain must resolve inside the immutable tree. The extracted regular-file names
and types must match the tree exactly on the host, or the result is could-not-check.
Site patterns match tracked entries and their directories, never paths through aliases.
There is no skip or borrowed-file fallback. SHA bounds and acceptance limits are unchanged.

## A bare `--consumers --brief <id>` on merged main

With a named `--brief` and neither `--base` nor `--delivery`, a brief the branch diff does
not carry (merged main, where HEAD equals the base ref, or no base ref at all) used to be
could-not-check forever, because no diff can corroborate a claim made by a merged change.
It now derives the pin instead, from two local sources in a fixed order: first the literal
`Verification-Table: delivery=<sha> base=<sha> original=<sha>` marker in the brief's
**committed** Evidence at HEAD, then, only when that Evidence carries no marker at all, the
checked-out history (below). Explicit `--delivery`/`--base` flags outrank both. The recorded triple is authenticated by the same reader the
protected-table closure uses (the delivery is an ancestor of HEAD on its first-parent chain,
and the recorded base is its exact baseline). The pinned base is then computed as the
delivery's own first parent, never copied from the marker, and the run is the delivery mode
above (immutable snapshot, exact summary counts, same exit codes).

With no marker, the delivery is the one first-parent commit of HEAD that names
`Brief: <stream>/<NN>` as a trailer: in its own message (a squash or direct commit), or in a
commit it merged (a merge commit, whose own message is title-only on GitHub). Trailers are
read fence-aware, and the id must match exactly (`alpha/010` does not name `alpha/01`). The
pin is that commit's first parent, so a merge delivery is the merge commit against the
mainline it landed on, the same range the marker path pins. Nothing is fetched and no forge is
asked, so this works from a verifier home detached at `origin/main`. Zero naming commits,
several (a later follow-up PR that carries the same trailer makes the history ambiguous), a
shallow repository, an octopus merge, or a candidate not reachable from the fetched mainline
ref is could-not-check (exit 2), and the refusal names what was searched. A stderr line names
the derived delivery and which source supplied it.
The search reads through the in-process Git library (one validated walk of every commit
reachable from HEAD, with its parents and message) and starts no new Git process. A
repository profile the library does not qualify, or a commit whose message it cannot read
exactly, is could-not-check too: the remedy is the marker or explicit `--delivery`/`--base`,
never a native fallback.

It is could-not-check (exit 2), never an unpinned diff and never a verdict, when the brief is
not committed at HEAD, has no marker and no single history candidate, a code-span or
malformed marker (a malformed marker never falls through to history), short ids, more than
one distinct delivery, a delivery that is not on HEAD's first-parent chain, a base that
is not its baseline, or a delivery that is not reachable from the fetched mainline ref
(`refs/remotes/origin/main`; with no such ref nothing can be shown to have landed). No
command-line value, environment variable or working-tree edit can supply the delivery: a
dirty marker is not read, and every git read of the derivation runs without the
repository-locating variables (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`,
`GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`), so the
checked-out directory alone names the repository. An explicit `--base` is untouched and keeps
the branch-diff meaning above, and a branch whose diff does carry the brief is checked
against that diff exactly as before. Without `--brief` nothing is derived.

Limit: the base is bound and the delivery is not. The base is computed from the named
delivery, never copied from the marker, but the delivery itself is chosen by whoever wrote
the committed marker, or, on the history path, by whoever wrote the `Brief:` trailer: it is checked against the git graph (on HEAD's first-parent chain,
landed on the mainline ref, with its exact baseline), not against the brief's declared files
or any record the verifier does not author. A marker that names a later mainline commit is
accepted, so a reviewer still owns the claim that the named commit is the real delivery.
The derivation reads every marker in the committed Evidence and refuses distinct triples;
the protected-table closure reads only the current run's marker. Evidence that has not yet
recorded a marker falls back to the history search, which is could-not-check unless exactly
one first-parent commit names the brief. The `consumer-delivery-range` lint below is
unchanged and still asks for explicit pins in authored rows.

A row re-authored under the
[immutable-delivery rule](protected-paths.md#explicit-human-approved-acceptance-transitions)
asserts its exact summary, including unchecked count. A row amended under an explicit
command-only acceptance transition
([protected paths](protected-paths.md#explicit-human-approved-acceptance-transitions))
changes only its Command and keeps its original Expect, class, shell and order, so its
Expect might not name the exact summary. The amended Command can assert the exact summary
itself, exiting non-zero on any other summary, which an unchanged exit-0 Expect still
accepts. Where it does not, the exact summary, including unchecked count, is a review
obligation, not a changed Expect. Exit 0 means no disproved claim; a
delivery with no corroborated claim returns 2. Unchecked exclusions remain review
obligations. Neither a successful path-touch check nor this mode establishes
that the consumer list is complete or that a consumer's behavior is correct.

For an undelivered brief, mark the branch check explicitly **pre-merge**, name its brief
and pin `--base` to the branch fork. At implementation, replace that provisional row with
the actual delivery and parent pins before requesting post-merge verification. Do not
invent a SHA, silently choose the oldest/latest matching history entry, widen the base to
make the tree pass, or rewrite historical Evidence. Separate authoring and implementation
commits are supported by delivery mode; inherited entries are evaluated only on that
explicit historical path. The derived path above addresses the merged-main failure described
in #1281, without claiming to close that issue's separate stale consumer declarations.

The shared Verify-row lint reports `consumer-delivery-range` for a missing explicit base,
a delivery without a brief, and a base-only check without a preceding parsed historical
checkout of the same root or explicit pre-merge scope. A fixed base by itself is insufficient. It reports existing defects as NOTICEs; a new verified/done
closure promotes this rule to a PROBLEM. It is a conservative shape check, not a shell
interpreter or a proof that a chosen SHA is the right delivery. The planted row regression
ensures the real corpus reader still invokes it.

## Known #1915 instances

Definition rows only were edited. Existing Evidence, witnesses and completed sign-offs
remain historical records. These are reproduction results, not independent verification
or permission to close a brief.

| Brief | Delivery | Corroborated / disproved / unchecked | Disposition |
|---|---|---|---|
| apps-installer/02 | 08d96323a459 | 3 / 0 / 1 | migrated; exclusion still owed |
| contributor-trust/01 | 01da8dfd6625 | 3 / 0 / 1 | migrated; exclusion still owed |
| contributor-trust/02 | 849f93e5f506 | 5 / 0 / 1 | migrated; exclusion still owed |
| contributor-trust/04 | 2083064fdeb2 | 3 / 0 / 1 | migrated existing bounded row to the shared mode |
| contributor-trust/06 | a5c828683251 | 3 / 0 / 1 | migrated; exclusion still owed |
| desk-supervision/09 | dfbd883d58f1 | 1 / 0 / 3 | migrated; original checks of exclusions retained |
| desk-supervision/13 | d8552d943f3c | 2 / 2 / 1 | **open hold on #1915**: delivered routes name untouched sets.go and ackbeacon.go; expected success retained, so the corrected range exposes a real red |
| desk-containers/08 | 21a3e1474c61 | 4 / 0 / 5 | bounded authoring/deferred claims only; does not attest implementation |
| desk-containers/09 | 58a019271393 | 3 / 0 / 1 | migrated; exclusion still owed |
| desk-containers/15 | 6cb553e1a177 | 3 / 0 / 3 | migrated; three unchecked obligations retained |
| measured-status/02 | a11ac41674c0 | 1 / 0 / 0 | migrated; implements a delivery with no Brief trailer |
| desk-tools/23 | b2050d8c735a | 1 / 0 / 4 | migrated; exclusions remain owed |
| desk-tools/37 | e28201023c74 | 4 / 0 / 0 | migrated actual implementation #3331, rather than later verification-control repairs |
| desk-tools/38 | eded7ba6e446 | 8 / 0 / 7 | migrated actual implementation #3332 (its incorrect `Brief: desk-tools/37` trailer does not identify /38); seven review obligations retained |
| measured-status/03 | 0842782 | 0 / 0 / 1 in the existing bounded branch check | **open hold on #1915**: Option 2 routes only to issue #1722; no corroborated consumer exists. Retain the unchecked obligation and existing row; do not manufacture a consumer pass |
| graph-execution/04, desk-containers/11, harness-portability/18 | not delivered | not checked | provisional pre-merge rows; implementation owns actual endpoint pins under the authoring contract above |
| cellctl-windows/00 | 2a6460e85c78 | not rerun here | existing separate check-definition issue #2045 retains custody |
| harness-portability/02, desktools-v2/14 | historical closed records | not rerun here | closed acceptance preserved; no retrospective edit or renewed pass claimed |
| build-less-brittle/02, /04, /05, /06, /09, /11, /12, /13 | existing bounded rows | not rerun here | prior repairs retained; no new acceptance claim |
| desk-supervision/12 | no authorized retirement delivery | not checked | protected retirement remains held under #1247; this repair grants no control removal |

The source census at the assignment base found 282 `--consumers` definition lines in
Verify sections across 21 stream families (Evidence excluded). Many belong to future or
closed work; this PR is not a mass rewrite of their acceptance. The new lint makes the
remaining unbounded forms visible, and the authoring contract above owns future pins.
The still-active holds above keep #1915 open. The separate future-row contract and
unchecked review obligations must be discharged before a complete class-closure claim.
