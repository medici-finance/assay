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
brief, a noncommit/moving delivery spelling, an invalid parent range, links/submodules or
archive-omitted blobs are could-not-check (exit 2). It does not choose a historical commit
for the author: identifying the actual delivery remains an independently reviewed claim.

Every re-authored row asserts its exact summary, including unchecked count. Exit 0 means
no disproved claim; a delivery with no corroborated claim returns 2. Unchecked exclusions
remain review obligations. Neither a successful path-touch check nor this mode establishes
that the consumer list is complete or that a consumer's behavior is correct.

For an undelivered brief, mark the branch check explicitly **pre-merge**, name its brief
and pin `--base` to the branch fork. At implementation, replace that provisional row with
the actual delivery and parent pins before requesting post-merge verification. Do not
invent a SHA, silently choose the oldest/latest matching history entry, widen the base to
make the tree pass, or rewrite historical Evidence. Separate authoring and implementation
commits are supported by delivery mode; inherited entries are evaluated only on that
explicit historical path. This addresses the mechanism described in #1281, without claiming
to close that issue's separate stale consumer declarations.

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
