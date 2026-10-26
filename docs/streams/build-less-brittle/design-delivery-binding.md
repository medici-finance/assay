# Design record: bind consumer acceptance obligations to immutable delivery provenance

This is the design record delivered by build-less-brittle/14. It is a design only: it
changes no checker, no Verify table, no Evidence and no protected route. Every rule it
proposes lands in a later implementation brief, and each of those carries its own
design-fit block and review.

**Class.** A Verify row whose consumer check names no delivering change. Such a row can be
satisfied only before the merge and can never pass on merged main. Every instance has the
same shape: a post-delivery obligation with no immutable delivery coordinate in the
authored contract. The coordinate does not exist when the row is authored, and adding it
afterwards is a change to a protected acceptance row.

**Freshness.** Read on `origin/main` at commit `e4d2346d8b51` (full id in the census block
below), on 2026-10-25: `docs/consumer-delivery-checks.md`, the
`R-consumer-delivery-range` and `R-original-table-closure` rows of `docs/contracts.md`,
`docs/protected-paths.md` §"Explicit human-approved acceptance transitions",
`statusgen/consumerdelivery.go`, the delivery-coordinate reader in
`statusgen/protectedclosure.go`, the `consumer-delivery-range` rule in
`statusgen/verifyrows.go`, the derived-path tests in `statusgen/consumerderived_test.go`,
and the delivering-PR resolver in `statusgen/autoflip.go` and `statusgen/deliveryfiles.go`.
Since the brief was authored, two merged changes have touched the lint file: #4140, which
separates frozen historical notices from actionable lint findings, and #3877, which runs
the lift in the desk executors. Neither binds a delivery into the authored contract. #4139
is still an open draft. The design question is therefore still open.

## Instance census

Each row below is a class instance that was repaired recently or is still open. The
delivery column gives the mainline commit that actually delivered the brief. The route
column says how the missing coordinate was recovered. Full commit ids are in the block
after the table, in table order.

| # | Brief, row | Delivery (first parent) | How the coordinate was missing or wrong | Recovered by, route |
|---|---|---|---|---|
| 1 | build-less-brittle/02 row 9 | `274ece128099` (`def62cbf50bc`) | Bare `--consumers --brief`, judged against the PR head only; on merged main the brief is not in the diff | Worker PR #1908 (`0f62549b1e25`) pinned the row to a clone at the delivery against its parent, with a mid-flight `version:` bump. This predates the acceptance-transition route |
| 2 | build-less-brittle 05, 09, 11, 12, 13 consumer rows | resolved per row at run time | Unpinned rows read COULD-NOT-CHECK on merged main | #2300 (`24886d98ad2a`) re-authored each row to find its delivery in shell from the `Brief:` trailer in commit messages on first-parent main. This is option C below, done by hand in every row |
| 3 | measured-status/02 | `a11ac41674c0` (#2054) | Bare form. The delivering squash commit's **message** carries **no** `Brief:` trailer, so a commit-message search cannot find it. The PR body of #2054 does carry `Brief: measured-status/02` | The definition-row migration in #3428 (`c24f792beac4`), which also added delivery mode and the `consumer-delivery-range` lint |
| 4 | desk-tools/37 row 3 | `e28201023c74` (#3331) | Bare form, exit 2 on merged main (reported on #1915) | The same #3428 migration: the delivery was identified by hand |
| 5 | desk-tools/38 row 3 | `eded7ba6e446` (#3332) | Bare form. The delivering squash commit's **message** carries `Brief: desk-tools/37`, which **names the wrong brief**. The PR body of #3332 correctly carries `Brief: desk-tools/38` | The same #3428 migration: identified by hand, against the commit-message trailer |
| 6 | desk-supervision/27 row 8 | `6df93b442c5a` (`4696d598e588`) | Bare `--consumers --root .`, no brief, no base | Worker PR #3516 (`ecb8c74a4ae8`) pinned an explicit delivery and base |
| 7 | desktools-v2/63 row 4 | `3937f39ae2d0` (`b787e831d0be`) | Bare form. The derived path from #3574 (`70cce9547cf2`) needs a committed `Verification-Table:` marker, and there was none until the verifier wrote one | The verifier landed its own marker in Evidence PR #3773 (`8c656759ef3b`) and re-ran at the landing commit. The verifier chose the delivery, which is the limit of option D |
| 8 | desktools-v2/65 row 4 (#3826) | `578a9f39d73e` (`1b00388fb9c5`) | Bare form, exit 2 after merge. Recognized as strike two in this class | A protected acceptance transition, #4228 (`e2b00ebe0f49`): Command only, `version:` incremented, with the driver's approval of the exact payload, under the ruling recorded on #1915 |
| 9 | desktools-v2/71 row 8 (#4209) | `f7ad98d58484` (`0191789433dc`) | `--base $(git merge-base origin/main HEAD)`, which is HEAD on merged main, so the diff is empty | A protected acceptance transition, #4213 (`4c63244c4665`), under the same ruling |
| 10 | graph-execution/07 row 8 | `5ea3f4a8bbd6` (`983327a4aee0`) | `--base` is pinned but there is no `--delivery` snapshot, so the lint raises a NOTICE | **Open.** The brief is held on the row's definition, and the next step named on #1915 is a re-authored row |

The census excludes two kinds of entry in `docs/consumer-delivery-checks.md` §"Known #1915
instances". Some are open holds whose corrected range exposes a real disproved claim
(desk-supervision/13) or whose only claim is unchecked (measured-status/03). The others are
provisional pre-merge rows for briefs that are not yet delivered. These are not instances of
a missing coordinate. The protected-transition mechanism itself landed in #3470
(`af335157314f`).

```text
e4d2346d8b51fbc893551107bc2b48df4d3f2c63
274ece12809994a5334342a02bdbc8aa9b1d350f
0f62549b1e253afd1316c5f0a5fdab0767a01983
24886d98ad2a880d594b670fa10dab8cf44c3ce4
a11ac41674c09aadb2a940bc7d1edbcd5e4c1402
c24f792beac4fc6b4c437b69d75095d21ffb968c
e28201023c74b5f407ae98f9833ee4f79999f1db
eded7ba6e4461a3afac436ff7196987d83268bb3
6df93b442c5abeffcf57ea1576b8acb932c00556
4696d598e58820806d0bcf8c9b2dd87f29115400
ecb8c74a4ae8462e4f9388e43dfbbb745d8b36be
3937f39ae2d00d150633f2c22568a79d1f5efb5a
b787e831d0be847f1ca81cc48496c8c0deb01fe5
70cce9547cf2540d24e7d923f507ec5bb2e6ea7b
8c656759ef3b8fceb48954647de82e059f450553
578a9f39d73ec48bdc07fbdfd3938edb463e7b9b
1b00388fb9c5bd17c0817ae3a49751607b0d9cae
e2b00ebe0f495037dcfd1483429cfe8ef3c24570
f7ad98d584842aef0001ddf22ee83b9e56e4f91f
0191789433dcb3d43618ac0b43e35bdede3e8c84
4c63244c4665af3bc1ec2319379f1bafd5dd4c8d
5ea3f4a8bbd6e84527cd55887889f231245aeed3
983327a4aee06a0435ccd0c8c3b50cdde0c72a66
af335157314f884136dd2b8344c57f0b481d8af9
```

The block above, together with this brief's Verify row 3, proves that each id exists and
is a commit object. It does not prove that a listed delivery is the real delivery. That is
a review claim. Each delivery was read here from its merge record: the merged PR's merge
commit or squash commit, which is on main's first-parent chain, with the first parent
shown in the table.

What the census shows: every recovery was done by hand, by an author, a worker or the
verifier, once per row. The two most recent recoveries each needed a human approval of an
exact payload. Two deliveries (rows 3 and 5) defeat a search of commit-message trailers in
the git graph, and one (row 7) could pass only after the verifier had chosen the delivery
itself. Neither row 3 nor row 5 defeats the delivering-PR resolver that the recommended
design reuses. That resolver reads `Brief:` trailers from merged PR bodies, never from
commit messages, and both PR bodies name the right brief. The defect in both is that the
squash commit message does not carry the PR body's trailer.

## Alternatives

**Criterion.** Each option is judged on one test, which has five parts. Does the option keep
the delivery identity independent of both the brief's author and the checker? Does it fail
closed when its source is unavailable? Does it keep nonzero corroboration and explicit
unchecked exclusions? Does it remove the per-brief protected amendment for briefs authored
after it lands? And what does it cost?

| Option | Independent of author | Independent of checker | Fails closed | Corroboration and unchecked kept | Removes the per-brief amendment |
|---|---|---|---|---|---|
| A. Status quo: per-row pins, the lint, protected command correction | no: the author, or the implementer, writes the SHA into the Command | yes | yes (delivery mode) | yes | **no**: the delivery does not exist at authoring time, so a correct post-merge row always needs an amendment |
| B. A delivery-binding record written at merge by an identity that is neither author nor checker, referenced symbolically from the row | yes, if the writer is a separate identity and the store is write-restricted on the server | yes | yes, if a missing record means could-not-check | yes: evaluation stays in delivery mode | **yes**: the Command text is final when authored |
| C. Derive the delivery from the git graph alone (the first mainline first-parent commit that adds the declared files or carries the trailer in its commit message) | partly: the trailer is text the author writes | **no**: the checker would choose the delivery at check time | only on ambiguity, and ambiguity is common | yes | yes, when the derivation is unambiguous |
| D. The committed `Verification-Table:` marker as the sole source (today's derived path) | yes | **no**: the verifier who runs the checker writes the marker, and a marker naming a later mainline commit is accepted | yes (pinned by Verify row 5) | yes | yes for bare-form rows, but the identity remains a reviewed claim, and the row passes only after the marker lands (census row 7) |

**Option C in detail.** The git graph holds commit messages, not PR bodies, so option C can
only search commit-message trailers. The census shows where those fail. The measured-status/02
delivery's commit message has no trailer. The commit messages of both the desk-tools/37
delivery and the desk-tools/38 delivery carry `Brief: desk-tools/37`, so a commit-message
search for /38 finds nothing, and a search for /37 finds two candidates. (The PR bodies of
#2054, #3331 and #3332 each name the right brief; the forge-side resolver in the recommended
design reads those, which is one reason it reads forge records rather than the graph.) "The
first commit that adds the declared files" fails for most briefs, because they modify files
that already exist. "The newest commit that touches them" picks up later follow-ups. #2300 is
option C done in shell, row by row. It works only where the commit message's trailer is
present and correct. If option C is computed inside the checker, the checker picks the
delivery it then judges, and nothing independent stands behind that choice.

**Option D in detail.** This path is shipped and safe for what it claims: the base is bound
and the delivery is not. The derived-path test that pins the recorded delivery's own first
parent shows that a marker naming a *later* mainline delivery is accepted, and the run
then judges that later commit. This is correct as today's behaviour, and it is the gap in
option D.

**Result.** Only option B meets every part of the criterion. Option B is not an alternative
to C and D. It is where C and D belong: C's attribution is run **once, at merge, by a third
identity**, and its answer is frozen. D's marker is kept as a corroborating cross-check
rather than the source. Option A remains the route for rows that are already merged.

## Recommended design

1. **Binding point: at merge, by main's CI.** On every push to `main`, a main-branch CI job
   builds statusgen from the trusted base's own source, as the board workflow already does,
   and runs a new verb (working name `statusgen delivery-bind`). For each brief whose
   delivering PR now resolves, the verb records one binding. It finds the delivering PR with
   the resolver statusgen already uses for model auto-flip: the `Brief:`-trailer resolver,
   then the newest-first walk of the brief file's history, the `files:`-overlap rule, the
   App approval at the merged head, and the brief's `delivery:` partial declarations. Both
   resolver paths attribute a PR by the `Brief:` trailers in its **PR body**, read from the
   forge; no path reads a commit message. The trailer resolver is a forge search of merged
   PR bodies for the literal trailer, used as a prefilter only: each hit is re-read and its
   body trailers parsed, and a body last edited after the merge is could-not-check. The
   history walk maps each commit that touched the brief file to its merged PR and judges
   that PR's body the same way. A PR whose body names a different brief is walked past and
   never credited; the `files:`-overlap rule only demotes a PR that is already credited, and
   never promotes one. The binding records:
   - the brief id and its `version:`;
   - the PR number;
   - the mainline first-parent commit that landed the PR (a merge commit or a squash
     commit);
   - that commit's first parent;
   - the resolver's grounds: whether the PR body's trailer matched, the declared files the
     PR touched, and the approving review's head.

   No binding is written when:
   - the resolver returns could-not-check;
   - the PR is declared partial;
   - a binding already exists for the brief.

   The verb is idempotent and never moves a binding.
2. **Storage: an annotated tag in a reserved namespace** (working name
   `refs/tags/assay-delivery/<stream>/<NN>`). The tag points at the delivery commit, and
   its message carries the fields above as fixed `key=value` lines. A forge tag ruleset on
   the namespace lets only the CI identity create these tags, and blocks updating and
   deleting them. This store was chosen for four reasons:
   - it is native to git and is fetched with history;
   - a tag cannot name an object that does not exist;
   - the server enforces that the tag cannot change;
   - no file in the tree holds the binding, so no PR can edit it.

   The choice of writer identity and store is owed to the driver (decision 2 below).
3. **How a row refers to it.** A post-merge consumer row is written as
   `statusgen --root . --consumers --brief <id> --delivery bound`, with no `--base` and no
   SHA (`bound` is a working spelling). The checker reads the binding for `<id>` from the
   local tag, computes the base as the delivery's first parent (never copying it), and
   runs today's delivery mode unchanged: immutable snapshot, exact summary counts,
   unchecked entries reported, and exit 2 when nothing is corroborated. The
   `consumer-delivery-range` lint accepts `--delivery bound` as an immutable delivery
   reference. The Command text is complete when the brief is authored and nothing in it
   changes at merge, so no protected amendment arises.
4. **When the binding is missing, ambiguous or contradicted, the result is exit 2,
   COULD-NOT-CHECK, never a pass and never a fallback range.** The checker refuses in each of
   these cases:
   - no binding tag for the brief is present locally (the message says to fetch the
     bindings, and the checker attempts no fetch);
   - more than one binding exists for the brief;
   - the tag is lightweight or its message is malformed;
   - an id is short;
   - the tag's target is not a commit;
   - the brief field in the tag differs from `--brief`;
   - the history is shallow;
   - the delivery is not on HEAD's first-parent chain, or is not reachable from
     `refs/remotes/origin/main` (this reuses the existing delivery-coordinate reader);
   - the delivery diff touches none of the brief's declared `files:`;
   - the brief's committed Evidence has a `Verification-Table:` marker that names a
     different delivery.

   The last check is option D kept as a cross-check: a disagreement between the verifier's
   record and the binding is a refusal, not a choice between them.
5. **Rows that are already merged stay on the protected route.** Rows with explicit pins
   keep them. Bare-form rows keep today's marker-derived meaning. A row that needs
   correcting still goes through the human-approved acceptance transition, as #4228 and
   #4213 did. The binder may record bindings for briefs that are already merged, because a
   binding is only a record, but it gives no credit to a row whose Command does not name
   `--delivery bound`. Rewriting an existing row to the symbolic spelling is itself a
   protected command correction. Whether the bare form should also cross-check against a
   binding when one exists is owed to the driver (decision 3 below).
6. **How it works with the open prospective-delivery PR, #4139.** #4139's lifecycle preflight
   is bound to the PR head. It validates the prospective delivery before ready-to-merge,
   using the reconciliation classifiers. The binder applies the same delivering-PR
   classification after the merge. The two run in sequence: the preflight predicts that
   this PR will be brief X's delivery, and the binder records that it was. If the preflight
   passes and the binder declines (for example, the approval at the merged head cannot be
   corroborated), no binding is written and the row reads could-not-check. That
   disagreement is a signal to investigate, not a pass. #4139 can also lint every
   `--delivery bound` row of a brief the PR delivers, so the authored contract is complete
   before the merge. This design does not absorb #4213. That protected command correction
   has merged, its row names explicit pins, and the design leaves it unaffected.
7. **Before the merge.** `--delivery bound` on an implementation branch finds no binding and
   exits 2. A brief that also needs a pre-merge pass keeps a separate, explicitly
   `pre-merge` row under the authoring contract in `docs/consumer-delivery-checks.md`.

**What this retires.** For briefs authored after the mechanism lands, it retires the manual
recovery of delivery coordinates by hand edit or acceptance transition (census rows 1–9),
and the per-row shell resolution of the delivery (census row 2). Nothing is removed by this
record.

## Independent layers

Three controls guard the delivery identity. Each is in a different component and fails on a
different signal.

| Layer | Component | Failure signal | What it catches | What stands behind it |
|---|---|---|---|---|
| L1. The binder | Main-branch CI job, reading the forge's merge, review and PR records | Resolver attribution: PR-body trailer match, declared-files overlap, App approval at the merged head, partial declarations | A delivery chosen by the brief's author or by the verifier. A follow-up that merges later (the binding is frozen first). A wrong commit-message trailer: the desk-tools/38 squash commit's message carries `Brief: desk-tools/37`, but the resolver never reads commit messages, and #3332's body names /38, so the trailer resolver credits #3332 directly. A PR body that names another brief is walked past and never credited, because files overlap only demotes a credited PR; no binding is written, and the row reads could-not-check. A PR body edited after its merge is could-not-check | L2, then L3 |
| L2. The checker's offline graph cross-check | statusgen consumer checker, in the verifier's checkout | Git topology, the declared-files overlap of the delivery diff, and agreement with the committed marker | A binding that names a commit off main's first-parent chain, a commit that has not landed, a commit whose diff touches none of the declared files, or a commit that contradicts the verifier's own recorded marker | L3 and review |
| L3. A server-side ruleset on the binding namespace | Forge rules | The identity of the ref writer | An author, worker or verifier who creates, moves or deletes a binding. The checker cannot repeat this check offline | Review |

The following are not layers:
- running the resolver again at check time, which repeats L1;
- the `consumer-delivery-range` lint, which checks only the shape of a row;
- a second test of the same predicate.

**Single point of failure, answered honestly.** The one control between a delivery identity
that is wrong but consistent and a false pass is **L1's attribution rule**. Suppose the
resolver credits a PR that is real, approved, on the mainline, touches the declared files
and carries the right trailer in its PR body, but is not the implementation. Three examples
are a follow-up that merged before the actual delivery, a partial slice that was not
declared partial, and a follow-up whose body names the brief correctly when the actual
delivering PR's body names another brief (the delivery is walked past, so the follow-up is
credited instead). L2 and L3 both pass such a PR. Two things own that residue. The first is review of the
Evidence: the brief's reviewer answers which single control stands between a wrong identity
and a false pass. The second is the `delivery:` `covers: partial` declaration, which the
resolver already honours.

When L1 is unavailable, because CI is down or a forge read fails, no binding exists and
the row reads exit 2. Recovery is to re-run the binder, which is idempotent and never
writes a second tag. A binding is never written by hand, and L3 refuses one.

If L3 is misconfigured, L2 still catches a forged binding that is inconsistent with the
graph. It does not catch a forged tag that names a plausible delivery. For that reason
L3's activation is part of decision 2, and it is not assumed.

## Decisions owed

Each of these touches protected-table authority or approval semantics. They are put to the
driver, not settled here. The first option in each is the proposed default.

- Decision 1, approval semantics — needs-decision medici-finance/assay#4380: does a row whose Command reads `--delivery bound` count as a complete, unchanged original obligation when a later merge-time binding fixes what it names, so that briefs authored after the mechanism lands need no per-brief acceptance transition? (a) yes, for new rows only; (b) yes, but each binding also needs a human approval comment in the transition payload form; (c) no, keep explicit per-row pins and the transition route.
- Decision 2, binding writer and store — needs-decision medici-finance/assay#4381: which identity writes delivery bindings, and which server-side rule protects them? (a) the main-branch CI workflow identity, writing annotated tags in a reserved namespace under a create-only tag ruleset; (b) a dedicated binding App that is not the worker, reviewer or verifier, with the same tag store; (c) a generated file committed to main by its single CI writer, protected by a lint on the commit author rather than by a server rule.
- Decision 3, existing bare-form rows — needs-decision medici-finance/assay#4380 (second question, https://github.com/medici-finance/assay/issues/4380#issuecomment-6398480713): when a binding exists, should today's bare `--consumers --brief` derived path also require the committed marker to agree with it? (a) no, the evaluation of existing rows is unchanged, and bindings affect only rows that name them; (b) yes, a disagreement is could-not-check for every record, including ones already verified; (c) yes, but only for records not yet verified or done.

## Implementation briefs

None of these is written here. Authoring them is a separate PR, made after the decisions
above are settled.

| # | Title | Owning module | Depends | Effort | Risk (regulatory / customer / irreversible / sensitive-data) | Verify row proving the lower layer with the upper one bypassed |
|---|---|---|---|---|---|---|
| I-1 | Record a delivery binding at merge from the delivering-PR resolver | `statusgen` (the resolver in `autoflip.go`/`deliveryfiles.go`, new binder verb) | decisions 1, 2 | M | no / no / no / no | A fixture in the desk-tools/38 shape (the squash commit's message carries a trailer naming another brief; the PR body names this brief) is bound to that PR, which pins that the binder attributes by the PR body and never by a commit-message search. A fixture whose PR body names another brief writes no binding, even though the PR touches the declared files: it is walked past, never credited on files overlap alone, and I-1 does not change that attribution rule. A fixture with the forge read unavailable writes no binding, and the verb exits could-not-check |
| I-2 | Accept `--delivery bound` in the consumer checker and the range lint, with the graph and marker cross-checks | `statusgen/consumerdelivery.go`, `statusgen/verifyrows.go` | I-1, decisions 1, 3 | M | no / no / no / no | With the binder bypassed, a hand-made fixture tag that names a side-branch commit, a commit touching none of the declared files, or a commit that contradicts the committed marker reads exit 2 COULD-NOT-CHECK |
| I-3 | Staged CI job and tag ruleset for delivery bindings | `ci/staged-workflows/`, forge configuration (documented; activation is the maintainer's) | I-1, decision 2 | S | no / no / no / no | With the CI writer bypassed, a binding tag pushed by a non-CI identity is refused by the forge. This is a human-run row: it is could-not-check offline |
| I-4 | Author new post-merge consumer rows with `--delivery bound` | author-brief kit, `docs/consumer-delivery-checks.md` | I-2 | S | no / no / no / no | A planted bare `--base` row in a new brief is still a `consumer-delivery-range` finding with the kit's guidance in place |

**Ordering.** Verify rows 4 and 5 of build-less-brittle/14 pin today's derived-path behaviour.
That includes the fact that a marker naming a later mainline delivery is accepted, and the
fact that the path refuses to pass when its source is unavailable. This brief must therefore
be verified before any implementation brief that changes that behaviour merges, and I-2
under decision 3 (b) or (c) is such a brief. After that merge, rows 4 and 5 going red is the
expected effect of the successor, not a defect in this record.
