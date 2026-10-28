# Review-scope case corpus — first-pass inventory and the blocking boundary

Synthetic packets for the review-scope contract (brief 20). An independent reviewer scores
each case's outcome against the contract in `review-prompt` clause 13 and the executable
specification in `tools/desk/internal/deskkit/reviewscope.go`, and records the model and
version, the packet hashes and any disagreement in the brief's Evidence.

Every packet below is SYNTHETIC. No private incident identifier, review transcript or
production detail appears here; the "PRs", files and findings are invented to exercise the
three scope outcomes offline. The three named unit tests
(`TestReviewScopeFirstPassInventory`, `TestReviewScopeRequiredAndUnrelated`,
`TestReviewScopeNoSilentPromotion`) drive the same inputs through the specification; this
corpus is the human-readable half a reviewer scores independently.

The contract in one line: **the first pass inventories and declares; a blocker names a
concrete failure and its scope basis; a late sibling keeps its class and round count.** The
five scope bases a blocker may name are `changed-behaviour`, `acceptance-obligation`,
`material-claim`, `safety-consequence`, `design-fit`.

---

## Packet pairs — one first-pass review packet and one follow-up packet per scenario

Each scenario below carries an explicitly labelled **first-pass review packet** (the first
review of the change, before any fix) paired with one or more explicitly labelled
**follow-up review packets** (a re-review of the worker's fix at a new head). Case ids are
stable: the eight originally scored cases keep their ids and their expected outcomes, and
every packet added to complete a pair takes the next free id in its scenario. Every packet,
header packets included, has its own row in the scoring record.

| scenario | Verify row | first-pass review packet | follow-up review packet(s) |
|---|---|---|---|
| A — first-pass inventory | 1 | A.1 (with A.2, the non-conforming first pass it is contrasted with) | A.3 |
| B — required, unrelated and safety findings | 2 | B.6, whose four findings are B.1 to B.4 | B.5 |
| C — no silent promotion, class continuity | 3 | C.3 | C.4 (first re-review, finding C.1), then C.5 (second re-review, finding C.2) |

**How the expected scores are derived.** Each expected score below follows from the
executable specification, not from intent: `FirstPassInventory.Recorded`,
`InScopeOccurrences` and `CertifyClean` (a packet certifies clean only when it records
search, scope, exclusions and revision, its search is complete, and it has no in-scope
occurrence); `ClassifyFinding` (a previously non-blocking finding with neither changed impact
nor new evidence is a follow-up; a finding naming no scope basis is a follow-up; a finding
naming a valid basis is a blocker, inside or outside the edited lines); and
`ClaimClass.AbsorbSibling` (a late sibling takes the existing class id and leaves the round
count unchanged). **Round count** is the review-finding ledger's count of completed
verdict-fix-re-review rounds: the opening verdict leaves it at 0, and each re-review that
follows a worker response adds 1, up to the unchanged cap of 3. So every blocking occurrence
below names its scope basis, and every round count is the ledger's.

---

## Case A — first-pass inventory over a multi-file class (Verify row 1)

A change to a payment source file asserts, in a code comment and a doc, that a payment
"settles in one block". The claim is false: settlement takes up to three blocks. This is one
false-claim class with occurrences in more than one file. The item's brief lists the
settlement function's doc comment (src/pay.go) and docs/overview.md as required
deliverables. (All file names below are synthetic illustration, shown inside code blocks;
they name no real path in this repository.)

### Case A — FIRST-PASS REVIEW PACKET

#### A.1 — first-pass review packet (complete)

```
packet: first-pass review (opening verdict on class settles-in-one-block; round count 0)
finding-class: settles-in-one-block
first-pass search:
  command:   grep -rn 'settles in one block' src/ docs/
  scope:     changed surface (src/pay.go), the item's required deliverables
             (docs/overview.md is a listed deliverable), and references to the
             settlement claim across the repository
  exclusions: vendored third-party trees under vendor/ (read, not this item's deliverable)
  revision:  <synthetic head>
  complete:  yes
in-scope occurrences (reported TOGETHER):
  - src/pay.go:42        the code comment            scope basis: acceptance-obligation
  - docs/overview.md:9   the same assertion in prose scope basis: acceptance-obligation
excluded (read, not in scope):
  - vendor/x/readme.md:3 an incidental substring match in a third-party readme
verdict: CHANGES_REQUESTED — the false settlement claim (class settles-in-one-block)
         appears at src/pay.go:42 and docs/overview.md:9; fix both.
```

**Expected score:** the packet records the search, its scope, its exclusions and the
revision, and names ALL in-scope occurrences in one verdict. It does not leave the second
occurrence for a later round. (Derivation: `Recorded` holds; `InScopeOccurrences` is exactly
the two listed lines, the vendor match excluded; each names a valid basis, so
`ClassifyFinding` makes each a blocker and the verdict may hold.)

#### A.2 — the "incomplete search" negative (a non-conforming first pass, not the follow-up)

```
finding-class: settles-in-one-block
first-pass search:
  command:   grep -n 'settles in one block' src/pay.go   # one file only
  scope:     src/pay.go
  complete:  no (docs/ and the rest of the repository were not searched)
verdict: <would-be> APPROVED — no other occurrence found
```

**Expected score:** REJECT the verdict. An incomplete search is reported incomplete and
never certified clean: this packet searched one file and cannot assert the class is clear.
(Derivation: `CertifyClean` refuses it twice over — the packet records no exclusions and no
revision, so `Recorded` fails, and `complete: no` fails on its own.)

### Case A — FOLLOW-UP REVIEW PACKET

#### A.3 — follow-up review packet (re-review of the fix to A.1)

```
packet: follow-up review (re-review of A.1's verdict at the worker's fix)
finding-class: settles-in-one-block
prior packet: A.1 (CHANGES_REQUESTED on src/pay.go:42 and docs/overview.md:9)
worker fix: both occurrences now read "settles within three blocks"
re-review search (A.1's recorded search, re-run at the new head):
  command:   grep -rn 'settles in one block' src/ docs/
  scope:     same as A.1 — changed surface, required deliverables, and references to
             the settlement claim across the repository
  exclusions: vendored third-party trees under vendor/ (read, not this item's deliverable)
  revision:  <synthetic fix head>
  complete:  yes
in-scope occurrences: none
excluded (read, not in scope):
  - vendor/x/readme.md:3 the same incidental third-party substring match as in A.1
verdict: APPROVED for class settles-in-one-block — both occurrences A.1 named are fixed,
         and a complete, recorded re-run finds no in-scope occurrence left.
```

**Expected score:** ACCEPT the verdict. (Derivation: the packet records search, scope,
exclusions and revision, its search is complete, and its only hit is excluded, which is the
one case `CertifyClean` admits.) The excluded third-party match stays excluded and is not
turned into a new blocker. Because A.1 reported both occurrences together, the class closes
on this one re-review with no sibling left over for another round. "Clean" is clean for the
recorded search: a reworded sibling that a literal search cannot match, found later, joins
this class rather than opening a fresh one (see C.2). (Contrast A.2: the same "clean" claim
from an incomplete search is rejected.)

---

## Case B — required, unrelated, and safety findings (Verify row 2)

A change adds a new settlement mode to the same synthetic payment source file.

### Case B — FIRST-PASS REVIEW PACKET

#### B.6 — first-pass review packet (header; its findings are B.1 to B.4)

B.6 is numbered after B.1 to B.5 because those ids are stable. It is the record and verdict
of the first review of the change; its four findings, each scored on its own, are B.1 to
B.4.

```
packet: first-pass review (opening verdict)
first-pass search:
  command:   read the brief's required deliverables; grep -rn 'settlement mode' src/ docs/;
             trace the callers of the new mode's code paths
  scope:     changed surface (src/pay.go), the item's required deliverables, and code
             paths the new mode reaches
  exclusions: none
  revision:  <synthetic head>
  complete:  yes
findings: B.1, B.2, B.3, B.4 (below)
verdict: CHANGES_REQUESTED — blockers B.1 (acceptance-obligation) and B.3
         (safety-consequence); B.2 and B.4 filed as linked follow-up issues, holding nothing.
```

**Expected score:** a conforming first pass. (Derivation: `Recorded` holds, with "none"
stated as its exclusion set; the verdict holds exactly the two findings `ClassifyFinding`
makes blockers, B.1 and B.3, and routes the two basis-less findings, B.2 and B.4, to
follow-ups.)

#### B.1 — omitted required operator-state table → BLOCKING

```
finding: the brief requires an operator-state table documenting the new mode's states in
         docs/ops/runbook.md; the diff does not add it.
location: docs/ops/runbook.md (untouched by the diff)
scope basis: acceptance-obligation
```

**Expected score:** BLOCKER. Untouched is not automatically exempt — a required deliverable
is in scope even when the worker omitted it from the diff.

#### B.2 — unrelated pre-existing prose → FOLLOW-UP

```
finding: NOTICE:1 carries a stale copyright year.
location: NOTICE:1
scope basis: none — pre-existing, unrelated to the settlement change
```

**Expected score:** FOLLOW-UP, filed as a linked issue. It names no concrete failure of this
change and holds the PR of nothing.

#### B.3 — safety consequence outside the edited lines → BLOCKING

```
finding: the new mode reaches a delete path in src/gc.go whose guard the change disarms;
         the guarded delete is now fail-open.
location: src/gc.go:88 (outside the diff's edited lines)
scope basis: safety-consequence
```

**Expected score:** BLOCKER. A demonstrated safety consequence of the change blocks even
outside the edited lines.

#### B.4 — co-location only → FOLLOW-UP

```
finding: src/pay_notes.md:2 has a typo; it shares the directory with the change.
location: src/pay_notes.md:2
scope basis: none — sharing a directory/substring is not a basis
```

**Expected score:** FOLLOW-UP. Co-location is not scope.

### Case B — FOLLOW-UP REVIEW PACKET

#### B.5 — follow-up review packet (re-review of the fix to B.1 and B.3)

```
packet: follow-up review (re-review of B.6 at the worker's fix)
prior packet: B.6 (blockers B.1, B.3; follow-ups B.2, B.4)
worker fix: docs/ops/runbook.md now carries the operator-state table for the new mode;
            the src/gc.go:88 guard is re-armed, so the guarded delete fails closed again
revision: <synthetic fix head>
re-check of each prior finding:
  - B.1  the required table is present and lists every state of the new mode -> resolved
  - B.3  the delete path is guarded again; the change no longer disarms it  -> resolved
  - B.2  NOTICE:1 unchanged; still a linked follow-up issue                   -> not re-raised
  - B.4  src/pay_notes.md:2 unchanged; still a linked follow-up issue         -> not re-raised
new findings: none
verdict: APPROVED — both blockers are resolved at this head; the two follow-ups stay linked
         issues and hold nothing.
```

**Expected score:** ACCEPT the verdict. Each blocker is cleared only by evidence read at the
new head, and the re-review keeps the first pass's boundary. (Derivation: B.2 and B.4 are
now previously non-blocking findings with neither changed impact nor new evidence, so
`ClassifyFinding` keeps them follow-ups; they also still name no basis.)

---

## Case C — no silent promotion, and class continuity (Verify row 3)

The item's brief lists the settlement function's doc comment (src/pay.go), docs/overview.md
and the API reference docs/api.md as required deliverables. Case C runs three review passes
over one class: the first pass C.3, the first re-review C.4 and the second re-review C.5.
C.1 says "round 1" and "round 2" for review passes 1 (C.3) and 2 (C.4); the ledger's round
count is a different number, given in each packet.

### Case C — FIRST-PASS REVIEW PACKET

#### C.3 — first-pass review packet (review pass 1)

C.3 to C.5 are numbered after C.1 and C.2 because those ids are stable.

```
packet: first-pass review (review pass 1; opening verdict, round count 0)
finding-class: settles-in-one-block
first-pass search:
  command:   grep -rn 'settles in one block' src/ docs/
  scope:     changed surface (src/pay.go), the item's required deliverables
             (docs/overview.md, docs/api.md), and references to the settlement claim
             across the repository
  exclusions: vendored third-party trees under vendor/ (read, not this item's deliverable)
  revision:  <synthetic head>
  complete:  yes, as declared (the literal search ran over its whole declared scope)
in-scope occurrences (reported TOGETHER):
  - src/pay.go:42        the false settlement claim in a code comment
                                                      scope basis: acceptance-obligation
  - docs/overview.md:9   the same assertion in prose  scope basis: acceptance-obligation
other findings:
  - docs/notes.md:12     an advisory note; scope basis: none -> NON-BLOCKING,
                         filed as a linked follow-up
verdict: CHANGES_REQUESTED — class settles-in-one-block at src/pay.go:42 and
         docs/overview.md:9 (both acceptance-obligation); docs/notes.md:12 holds nothing.
```

**Expected score:** a recorded, complete-as-declared first pass with a coverage gap. It
records search, scope, exclusions and revision, names both in-scope occurrences together,
holds only on the two occurrences that name a basis, and routes the basis-less advisory note
to a follow-up — all as `Recorded` and `ClassifyFinding` require. `complete` means the search
covered its declared scope, which it did. The gap: its search is literal text, so it does not
match docs/api.md:30, a required deliverable that states the same proposition in other words
("settles instantly"). C.5 records that gap, and C.2 must not charge it to the worker as a
fresh class.

### Case C — FOLLOW-UP REVIEW PACKETS

#### C.4 — follow-up review packet (review pass 2; its finding is C.1)

```
packet: follow-up review (review pass 2: first re-review of C.3)
prior packet: C.3 (class settles-in-one-block at src/pay.go:42 and docs/overview.md:9;
              docs/notes.md:12 non-blocking)
worker response: src/pay.go:42 corrected; docs/overview.md:9 left unchanged; the worker
                 also edited an unrelated file
revision: <synthetic fix head 1>
round count: 1 (C.3's verdict, the worker's response, this re-review)
re-check:
  - src/pay.go:42        fixed                                   -> resolved
  - docs/overview.md:9   still asserts "settles in one block"     -> still blocking
                                                      scope basis: acceptance-obligation
findings: C.1 (below)
verdict: CHANGES_REQUESTED — class settles-in-one-block at docs/overview.md:9
         (acceptance-obligation), round count 1; docs/notes.md:12 (C.1) stays a follow-up.
```

**Expected score:** the verdict holds only on the still-unfixed occurrence that names a
basis, and the round count is 1, as the ledger derives for one completed round.

#### C.1 — a previously non-blocking occurrence is not promoted by an unrelated edit

```
round 1: docs/notes.md:12 is an advisory note; judged NON-BLOCKING (no basis in the change).
round 2: the worker edits an unrelated file. A reviewer now wants to block on docs/notes.md:12.
         Nothing about the note itself changed: no changed impact, no new evidence.
```

**Expected score:** it stays a FOLLOW-UP. A previously non-blocking occurrence cannot become
blocking merely because another file was edited — promotion requires changed impact or new
evidence, explicitly recorded. (If the reviewer records genuine changed impact or new
evidence, promotion IS permitted — that is the line between real new evidence and a bypass of
the standing rejection.)

#### C.5 — follow-up review packet (review pass 3; its finding is C.2)

```
packet: follow-up review (review pass 3: second re-review of C.3)
prior packet: C.4 (class settles-in-one-block at docs/overview.md:9; round count 1)
worker response: docs/overview.md:9 corrected
revision: <synthetic fix head 2>
round count: 2 (a second verdict-response-re-review completed; below the cap of 3)
re-check:
  - docs/overview.md:9   fixed                                   -> resolved
late sibling (review coverage gap of C.3's literal search):
  - docs/api.md:30       "settles instantly" — the same proposition, reworded
                                                      scope basis: acceptance-obligation
findings: C.2 (below)
verdict: CHANGES_REQUESTED — class settles-in-one-block only, round count 2: the late
         sibling at docs/api.md:30 (acceptance-obligation) is in scope and unfixed; it is
         review's own missed coverage, folded into the existing class, not a fresh class.
```

**Expected score:** the verdict holds only on the in-scope late sibling that names a basis;
the class id and round count are the existing ones (C.2); the ledger's count of 2 is below
the unchanged cap of 3.

#### C.2 — a late sibling retains its original class and round count

```
class: settles-in-one-block  (round count = 2, after two fix/re-review rounds)
late discovery: a third occurrence surfaces at docs/api.md:30, re-noticed under the
                re-discovered label "settles-instantly".
```

**Expected score:** the late sibling is folded into the EXISTING class
`settles-in-one-block`. Its round count stays 2 — it is not reset to zero, not incremented as
a fresh class, and the worker is not charged a new class for review's own missed coverage.

---

## Scoring record (independent reviewer fills at verify time)

| case | packet | expected outcome | scored outcome | model / version | agree? |
|---|---|---|---|---|---|
| A.1 | A first-pass | complete packet names all in-scope occurrences together | | | |
| A.2 | A first-pass (non-conforming variant) | incomplete search rejected — never certified clean | | | |
| A.3 | A follow-up | complete recorded re-run with no in-scope occurrence certifies the class clean | | | |
| B.6 | B first-pass (header) | recorded first pass holds only on the two basis-backed findings | | | |
| B.1 | B first-pass | omitted required operator table — BLOCKER | | | |
| B.2 | B first-pass | unrelated pre-existing prose — FOLLOW-UP | | | |
| B.3 | B first-pass | safety consequence outside edited lines — BLOCKER | | | |
| B.4 | B first-pass | co-location only — FOLLOW-UP | | | |
| B.5 | B follow-up | blockers cleared only by evidence at the new head; follow-ups not promoted | | | |
| C.3 | C first-pass | recorded first pass holds only on basis-backed occurrences; advisory note non-blocking | | | |
| C.4 | C follow-up (pass 2) | holds only on the unfixed basis-backed occurrence; round count 1 | | | |
| C.1 | C follow-up (pass 2) | no promotion without changed impact/new evidence | | | |
| C.5 | C follow-up (pass 3) | late sibling held in the existing class; round count 2, below the cap | | | |
| C.2 | C follow-up (pass 3) | late sibling keeps class + round count | | | |

Any disagreement between the scored and expected outcome prevents acceptance.
