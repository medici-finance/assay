---
id: DR-cross-repo-witness
date: "2026-11-05"
title: "Witness a cross-repository brief's check:ci rows by extending the container bracket to a second, read-only, digest-bound tracking tree (option 1); proposed as a desk-decided default, not yet ratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2: a Linux host lane for cross-repository check:ci rows. Not chosen. The host lane already admits version-2 receipts on a Linux host with `unshare --net`, so the new part would be dedicated Linux host infrastructure for hosts that have none. That lane isolates the network only: the home stays writable, the host's identity and stored logins remain in reach, no image digest is recorded, and no receipt can show that the rows ran in a closed environment. It also leaves the actual gap open, because the host lane runs every row in the deliverable home too."
  - "Option 3: reshape each affected brief's rows so they need no tracking checkout, or split the brief so tracking-repository rows move to a brief homed there. Not chosen as the default. It is per-brief, every row change is a reviewed Verify-table change (a driver act, and a protected table also needs the acceptance-transition route), and it does not generalise: the next cross-repository brief with a tracking-repository row reopens the same gap. It stays available case by case."
accepted:
  - "Proposed, not ruled. The open step digests the isolated tracking checkout the host admission check has already verified, records it in the bracket receipt as `tracking_tree`, and marks the receipt with an explicit bracket version. The container and the close step each recompute that digest and refuse on any mismatch. The tracking checkout is mounted read-only beside the home and never inside it."
  - "Proposed, not ruled. A row runs in the tracking tree only when its Verify table says so in a per-row `Root` cell (`home` by default, or `tracking`), on the precedent of the `Shell` column. The root is never guessed from the command text. A `tracking` row that a binary reading the `Root` column finds no bound tracking tree for is could-not-run, never a fail and never run in the home. A binary that predates the column ignores it and runs every row in the home, so no `Root: tracking` cell may land in a brief before the pinned chain reads it, and a reader that credits must refuse such a row from a span without the new digest line (see Migration and version negotiation)."
  - "Unchanged: `--network none`, the read-only home and receipt mounts, the host UID/GID as the container user, no credential passed into the bracket, the image digest on every credited row, the home tree digest, the attestation line naming both repositories and both commits, and the credit rule (the tracking repository is the brief's own; the source repository is in the configured repository set)."
  - "Threat-model delta: rows may now read and execute any file of the tracking tree, not just the brief. Those bytes come from the same repository and the same attested commit as the brief whose Verify commands already run in the bracket, and they are digest-bound and read-only."
  - "Implementation is a separate brief. This record changes no code, no skill text and no rule text."
---

**Status:** draft

**Approval state: placeholder-unratified. This is not an approved design
record.** It is a desk-decided default: the pull request that adds it declares the choice in
its `## Desk-decided` section, and the driver ratifies it, or chooses another option, by ruling
on the decision. No ruling comment exists yet, so `decided-by` keeps the public placeholder and
the record carries no `ruling:` link. The [register README](README.md) puts the ruling before
the record lands with its `ruling:` link, and says `statusgen --corroborate` fails a record
still awaiting its ruling on the pull request that adds it with `placeholder-unratified`. The
`corroborate` check on this pull request did not evaluate the record (the step returns early
for a pull request that touches no findings entry), so a green result there says nothing
about the ruling state. Until the driver rules, read the record as a proposal. No brief cites
it in `design:` yet. The implementation brief that does must wait for the ratification.

## Context

A brief can be tracked in one repository (the tracking repository, which holds the brief and
its attestation record) and delivered in another (the delivery repository). Its verifier
admission receipt is then version 2 (cross-repository). The attestation line names the
tracking repository as `repo#issue`, the delivery repository as `source-repo=`, the delivery
commit as `source=` and the tracking commit as `tracking-source=`.

Its `check:ci` rows must run network-off. A host without `unshare --net`, such as a darwin
desk host, records them could-not-run. The only network-off lane on such a host is
the container admission bracket ([DR-container-witness](DR-container-witness.md)): host
open, `--sandbox=container-netns` container run, host close.

### What #4619 reported, and what has changed since

#4619 recorded the bracket's open step refusing every version-2 receipt in container mode
(observed on statusgen v1.0.63). [DR-container-witness](DR-container-witness.md) had accepted
that refusal: "Cross-repository (version 2) admission receipts are refused in container mode.
A later brief may admit them." This record does not edit that accepted bullet (the register is
append-only). It records what came after.

#4625 (merged 2026-10-27, first released in v1.0.64) made the bracket admit version-2 receipts,
and #4627 made `--bracket-digest --credit` credit such spans. Verified on main at
`d497f2a73`:

- `statusgen/containerbracket.go:46-55` states the version-2 bracket. The brief is the
  tracking repository's file, so the home's tree digest does not cover it. Each step requires
  the brief to lie outside the home, at the binding's brief path, holding exactly the bytes
  the binding's `BriefSHA256` names.
- `statusgen/containerbracket.go:297-344` (`crossRepoBindingProblem`) enforces that: two
  different owner/name repositories, a 64-hex run, a 40-hex `source` and `tracking-source`,
  the shared binding judgment, the brief outside the home, and the brief's sha256.
- `statusgen/containerbracket.go:540-564` (open) and the close step at lines 771-792 require the
  host check's admitted deliverable home to be `--root`. The close step also requires the
  host binding to equal the receipt's.
- `plugins/assay/skills/verify-desk/SKILL.md:227-233` states the same rule for desks.

So a cross-repository brief's rows that run in the deliverable home are container-witnessable
today. A host whose installed statusgen predates v1.0.64 still sees the original refusal.

### The gap that remains

**Every lane runs a cross-repository brief's rows in the deliverable home, and no lane binds
the tracking tree beyond the brief file.**

- Host lane: `statusgen/verifyrun.go:2439-2449` (`rowRoot`) runs rows in the admitted
  deliverable home, and `statusgen/verifieradmission.go:62-80` (`admittedRowRootRefusal`)
  refuses any other root before a row runs.
- Container lane: `statusgen/verifyrun.go:2392-2416` admits from the receipt and runs rows
  under `--root`, which `crossRepoBindingProblem` requires to be the home that does not
  contain the brief. The only tracking-repository bytes the bracket binds are the brief's
  (`BriefSHA256`). The rest of the tracking checkout is neither digested nor reachable as a
  row root.
- Launcher: `statusgen/verifyrun.go:2460-2469` refuses `--in-container` for a
  cross-repository run outright: "the container would mount that home without the tracking
  brief, and its inner run would not carry this run's admission".
- Adjacent refusals: `--acceptance-record` requires same-repository admission
  (`statusgen/verifyrun.go:2456-2458`), and `--history-pack` requires a home-relative brief
  (`docs/container-baseline-history.md`, last paragraph).

The host admission check already verifies the isolated tracking checkout. In
`tools/desk/internal/deskkit/verifiercrossrepo.go:312-385`, it must be the attested tracking
home, must not overlap the deliverable home, must be a checkout of the tracking repository,
and must pass the same local checks as a version-1 home at `tracking-source` (lines
379-384). The trusted state exists on the host. The bracket does not carry it into the
container.

The practical result, reported on the tracking side but not reproduced for this record
(**unverified here**): a cross-repository brief whose `check:ci` rows read files that only
the tracking repository holds stays held. Its deliverable-home rows can be witnessed in a
closed bracket, but its tracking-repository rows have no lane. Running them under the host
lane would record false failures, because they would run in the wrong tree.

## Options

### 1. Container bracket with both trees read-only and both commits bound (chosen)

Extend the version-2 bracket so the tracking checkout becomes a second bound tree in the
container, mounted read-only and digested at all three steps. Rows may name it as their
root. `--network none` stays.

*Binding it needs:* a tracking-tree digest in the bracket receipt, recomputed by the
container before and after the rows and by the host at close, and recorded in the witness
span so it falls inside the bracket digest. Both commits are already in the attestation line.

*Consequences:* it is general and covers every future cross-repository brief. It needs
statusgen changes in the open, container and close steps, a Verify-table column, and lint.
Briefs that already exist need one `Root` cell per tracking-repository row (see open
question 2).

### 2. A Linux host lane for cross-repository check:ci rows

Provide Linux hosts so `unshare --net` is available, and run cross-repository `check:ci`
rows on the host lane.

*Why it weakens isolation compared with the bracket:*

- **Network.** `unshare --net` removes the network for the row's process tree. That much is
  equivalent.
- **Writable state.** The home and the rest of the host file system stay writable to the
  row. Nothing mounts them read-only. A change is caught later, by admission at landing,
  not prevented.
- **Identity.** The row runs as the host user, with that user's stored logins and
  configuration in reach. `admittedRowEnv` (`statusgen/verifieradmission.go:144-170`) strips
  forge-token variables and pins git config, but its own comment says it cannot remove a
  stored login: "Containment is a runner that holds no credential but the verifier's own."
- **No read-only mounts and no image.** No digest-pinned image is recorded, and no tree
  digest brackets the run.
- **No proof of the environment.** A host-lane receipt shows that admission passed. It cannot
  show that the rows ran in a closed environment.

*Consequences:* it is new infrastructure, which is the driver's call. It is unavailable on
the hosts that have the problem. And it does not close the gap: the host lane also runs
every row in the deliverable home (`rowRoot`), so a tracking-repository row still has no
root.

### 3. Reshape the affected briefs' rows

Rewrite each affected row so it needs nothing from the tracking checkout, or split the
brief so its tracking-repository rows move to a brief homed in the tracking repository.

*Binding it needs:* none new. Each reshaped brief uses the existing version-1 or version-2
bracket.

*Consequences:* the work is per-brief. A Verify-row change is a reviewed Verify-table change
and never an Evidence-only edit (`docs/brief-rules.md`, the `Shell` column paragraph), so
each one is a driver act. A protected table also needs the acceptance-transition route.
Rewriting a row can lose the property it checks. The change does not generalise: each new
cross-repository brief with a tracking-repository row reopens the gap. It stays the fallback
for a single brief while option 1 is being built.

## Recommendation: option 1, as the desk-decided default

### Receipt fields and binding

Today's attestation binding (version 2) is unchanged. It already carries `Repo`,
`SourceRepo`, `Source`, `TrackingSource`, `BriefSHA256` and, host-side,
`TrackingHomeSHA256`. The change is to the **bracket receipt file** that `--open-bracket`
writes (`bracketReceipt`, `statusgen/containerbracket.go:131-139`):

- `bracket`: an explicit integer bracket version. Absent means today's bracket. `2` means
  the tracking tree is bound.
- `tracking_tree`: `sha256:<64 hex>`, which is `homeTreeDigest`
  (`statusgen/containerbracket.go:417-483`) over the isolated tracking checkout. The open
  step computes it on the host, after the host check has admitted that checkout at
  `tracking-source`.

**Hashed:** the tracking checkout's regular files and symlinks, excluding the top-level
`.git`, by the same function and rules as the home.

**Recomputed by the container:** before any row and again after the rows, over the tracking
root. The tracking root is derived from the `--brief` path the step already validates: the
resolved brief path minus its `/<binding brief path>` suffix. No new flag can point it
elsewhere.

**Recomputed at close:** on the host, over the tracking checkout the re-run host check names.

**Recorded in the witness:** one `Tracking-Tree-Digest:` line in the span, after
`Tree-Digest:`. It is therefore inside the `witness=` bracket digest, and `--bracket-digest`
must count it, as in `span-lines=…,tracking-tree-digest:1`.

**Refusals (all before credit, all naming the field):** the container's start digest does not
equal `tracking_tree`. The container's end digest differs from its start digest, in which
case no attestation, `Tree-Digest` or `Tracking-Tree-Digest` line is written, as for the
home today. The close digest does not equal both the receipt's and the witness's. A
`bracket: 2` receipt with no `tracking_tree`. A `bracket: 2` span with no
`Tracking-Tree-Digest` line, or with two of them. A `Tracking-Tree-Digest` line under a
version-1 attestation.

### Mount model

Paths below are the documented container layout's conventions, not new requirements. Only
the relations between them are binding.

- Home (deliverable checkout): read-only at `/work`, as today.
- Tracking checkout: read-only, at a path outside `/work` (for example `/tracking`), so
  that `/tracking/<binding brief path>` is the `--brief` argument. The step refuses if
  either tree contains the other, mirroring the host check's overlap refusal
  (`verifiercrossrepo.go:331`).
- Receipt (and history pack, where used): read-only under `/input`, outside both trees.
- Witness: a new file under a writable `/output`, outside both trees, as today.

**Row roots:** a `home` row (the default) runs with the home as its working directory,
exactly as today. A `tracking` row runs with the tracking root as its working directory.
The row environment carries no extra variable that names the other tree. The concrete mount
points are not part of the contract (see open question 5), so in the first implementation a
row reads and executes only files under its own root. A row that needs files from both trees
is not supported.

**No git directory in either mount.** The tracking checkout's `.git` is not covered by the
tree digest (`homeTreeDigest` skips a top-level `.git`, whether it is a gitfile or a
directory), and a full clone keeps a remote URL or an extra HTTP header, and so possibly a
credential, in `.git/config`. A row could read that file and print it into a witness that
lands as Evidence. The open step therefore refuses when the tracking root's top-level `.git`
is a directory and accepts only a gitfile (a linked worktree), or the mount masks the
entry. Which of the two is for the implementation brief to state and test. The home has the
same exposure today, so a linked follow-up may apply the same rule to it.

### What stays unchanged

`--network none`. The read-only home and receipt mounts. The container run as the host
UID/GID on POSIX hosts (`docs/container-baseline-history.md`). No credential and no
`--env-file` passed into the bracket: container mode never derives identity, and the Runner
cell is the host's (`statusgen/verifyrun.go:2506-2523`). The image digest on every credited
row. The home tree digest at all three steps. The attestation line, which already names both
repositories and both commits. The credit rule (`SKILL.md:230-233`). A read-only root file
system for the container is **not stated in the shipped docs** (**unverified**). This record
neither adds nor assumes it.

### Failure modes

- **Second tree missing.** The tracking root does not exist in the container, or the brief
  does not resolve under it. Container admission is refused before any row, and the run is
  could-not-run.
- **Commit mismatch.** The tracking checkout's HEAD is not `tracking-source`. The host check
  refuses at open and at close (`verifiercrossrepo.go:379-384`, which reruns the version-1 local checks against the tracking tree at `tracking-source`). Inside the container there is
  no git history to compare (the mount carries no `.git` directory, see Mount model), so
  inside it the digest is the binding.
- **Tracking tree dirty.** A modified, untracked or ignored file in the tracking checkout
  fails the host check's version-1 local checks at open. Inside the container, any change
  after open fails the start digest. A change made by a row fails the end digest. *The
  untracked/ignored case comes from reading the reused checks, not from running them*
  (**unverified**).
- **Trees overlap**, or the tracking root equals the home: refused.
- **`tracking` row without a bound tracking tree, read by a `Root`-aware binary** (a
  version-1 brief, a `bracket`-absent receipt, or the host lane before it adopts the same
  rule): could-not-run, with the reason. Never a fail, and never run in the home. This bullet
  covers only binaries that read the `Root` column. The next bullets cover the ones that do not.
- **Old container binary, new receipt.** Go's JSON decoding ignores unknown fields, so an
  old runner would run without writing `Tracking-Tree-Digest`. The new close step refuses
  that span, because the receipt says `bracket: 2`. The failure is closed, at close.
- **All-old chain, brief already carrying `Root` cells.** This case is verified, and it is
  open unless the migration rules below hold. At `d497f2a73`, `verifyRowTable`
  (`statusgen/verifyrows.go:1747-1772`) finds columns by header name and knows only
  `command`, `expect`, `#`, `class` and `shell`, so a `Root` header is silently ignored.
  `rowRoot` (`statusgen/verifyrun.go:2439-2449`) then runs every row in the home. An old open
  writes no `bracket` field, and an old close requires no `Tracking-Tree-Digest` line
  (the span-line counts are `tree-digest`, `container-image` and `attestation`,
  `statusgen/verifyrun.go:2035`). So an all-old chain runs a `Root: tracking` row in the
  home, binds nothing for the tracking tree, and can credit the row when its command happens
  to pass in the home (an absence check, or a grep that expects no match). Nothing the old
  binaries do refuses it. Only the rules in Migration and version negotiation close it.

### Threat-model delta

**Today:** inside the bracket, the tracking repository contributes the brief's bytes, bound
by `BriefSHA256`, and the Verify commands lifted from them. Those commands already execute
arbitrary code in the container, with read access to the home.

**With option 1:** rows may also read and execute any file of the tracking checkout, such as
a script that a `tracking` row invokes. The new surface is the rest of the tracking tree. It
is limited as follows:

- **Same principal.** The bytes come from the repository and the attested commit that
  already supply the commands. A malicious tracking tree could already run any command
  through its own brief. It gains files to run, not a capability.
- **Read-only and digest-bound.** A write attempt fails on the mount. Any change the mount
  misses fails the end digest, so no attestation is written.
- **No network and no credential.** There is no new egress path and no new secret to read.
- **No authority over the home.** The home is still digest-bound at all three steps, and its
  mount is still read-only.
- **No change to credit.** The tracking repository must still be the brief's own, and the
  source repository must be in the configured set.

What a malicious tracking tree still cannot do: change which commits the attestation names,
alter the witness after the rows end (the witness file is new and is written by statusgen
after the rows), or have a `tracking` row credited, by a chain that reads the `Root` column and the new
credit rule, from a bracket that did not bind the tracking tree. A chain that predates the
column is outside that sentence. The landing-order rule in Migration and version negotiation
is what keeps such a chain from meeting a `Root: tracking` cell.

### Migration and version negotiation

- **Attestation binding versions 1 and 2 keep their meaning**, field for field.
- **A receipt without `bracket` keeps exactly today's meaning:** home-bound only. Its
  version-2 runs keep running home rows. Read by a binary that knows the `Root` column, its
  `tracking` rows are could-not-run.
- **A `bracket` value the reader does not know is refused**, never read as the nearest known
  one.
- **Witness spans:** a span without `Tracking-Tree-Digest` is read exactly as today. Already
  landed brackets keep their `witness=` digest and their credit.
- **`Root` column:** absent or empty means `home`, so every existing table runs byte-for-byte
  as today. `statusgen --lint` hard-errors on an unknown value, as it does for `Shell`.
- **The `--in-container` refusal for cross-repository runs stays** (see open question 3).
- **Landing order for `Root` cells.** No `Root` column and no `Root: tracking` cell lands in
  any brief until the statusgen and desk-tools release that reads the column in every step
  (lint, open, container run, close, credit and the landing check) is the pinned release.
  Adding a `Root` cell is a reviewed Verify-table change (open question 2), so the review of
  that change checks the pin. The implementation brief carries the pin bump as the
  precondition of its first `Root` cell. This rule is the only control for the window in
  which an old chain might meet such a cell, because an old binary cannot be made to refuse
  a column it does not read.
- **Credit-side refusal, whichever binary wrote the span.** A row whose brief declares
  `Root: tracking` is not credited from a span that lacks a `Tracking-Tree-Digest` line,
  whether or not a `bracket: 2` receipt exists. A reader that knows the column applies it to
  a span written by any earlier binary. This covers a `Root: tracking`
  cell that landed early and was run by an old chain, once a new reader sees the witness. It
  does not undo a credit an old reader already gave, so the landing-order rule stays the
  primary control.
- **New-format span read by an older `--bracket-digest --credit`.** The older reader counts
  only the `tree-digest`, `container-image` and `attestation` lines
  (`statusgen/containerbracket.go:930-944`). It digests a `Tracking-Tree-Digest` line as part
  of the span but cannot check it, and credit then rests on the close step and the
  forge-attributed record, which are unchanged. The implementation brief states whether an
  older reader must refuse a span carrying a line it does not know.

### Tests and mutations the implementation brief must carry

Fail-first: each new refusal is shown red against the pre-change code, or by a committed
mutation entry. Near-miss refusals, each a separate case:

- tracking digest off by one file
- tracking digest correct at start but changed by a row
- `bracket: 2` without `tracking_tree`
- a span with two `Tracking-Tree-Digest` lines
- a `Tracking-Tree-Digest` line under a version-1 attestation
- a tracking root inside the home, and the home inside the tracking root
- a brief path whose suffix matches but whose root is a different checkout
- an unknown `bracket` value
- an unknown `Root` value at lint
- a `tracking` row under a version-1 brief, which is could-not-run, not fail
- a committed old-format witness span (no `Tracking-Tree-Digest` line) next to a brief
  whose row declares `Root: tracking`, which the credit step must refuse
- a tracking root whose top-level `.git` is a directory, which the open step refuses (or the
  mount masks it, as the brief chooses)

Positive cases:

- a version-2 bracket whose `home` and `tracking` rows both pass and are credited
- a receipt without `bracket`, which reproduces today's witness byte-for-byte

Mutation entries that must turn the suite red:

- skipping the start digest
- skipping the end digest
- dropping the line from the bracket digest
- accepting a missing `tracking_tree`
- dropping the credit-side refusal for a `Root: tracking` row without the span line

## Open questions for the driver

1. **Row-root declaration.** Is the per-row `Root` cell the right form, or should
   tracking-repository rows be split into a brief homed in the tracking repository (option 3)?
   Default: the `Root` cell.
2. **Is adding a `Root` cell a definition change?** Default: yes, as for `Shell`. It is a
   reviewed Verify-table change, and a protected table's existing acceptance route applies.
3. **The `--in-container` launcher** (`verifyrun.go:2460-2469`). Default: it stays refused
   for cross-repository runs, and the bracket is the supported lane.
4. **Landing order.** Is the rule that no `Root` cell lands before the pinned chain reads
   the column (Migration and version negotiation) the right control for the old-chain
   window, or should the first implementation hold every `Root` cell behind a further gate?
   Default: the landing-order rule, plus the credit-side refusal.
5. **Mount layout.** Should the relation between the two mount points become part of the
   contract so a row can reach the other tree by a relative path? Default: no. A row reads
   only its own root, and the layout stays unspecified.
6. **Out of scope** unless the driver says otherwise: briefs that bind further sibling
   sources (#4876), `--history-pack` fail-first for cross-repository briefs, and
   `--acceptance-record` for cross-repository runs. Each would get its own record.

## What this record does not decide

Implementation is a separate brief. This record changes no code, no skill text, no rule text
and no Verify table. It does not loosen any existing refusal, including `sandboxRefusal`, the
version-2 binding checks, the `--in-container` refusal and the credit rule. It does not
verify any brief or move any board row. It does not amend
[DR-container-witness](DR-container-witness.md).
`consequence: major`: if this design is wrong, a row that ran against unbound
tracking-repository bytes could be credited as an independent-verification witness.
