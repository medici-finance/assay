# Review-scope case corpus — first-pass inventory and the blocking boundary

Synthetic packets for the review-scope contract (brief 20). An independent reviewer scores
each case's outcome against the contract in `review-prompt` clause 12 and the executable
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
four scope bases a blocker may name are `changed-behaviour`, `acceptance-obligation`,
`material-claim`, `safety-consequence`.

---

## Case A — first-pass inventory over a multi-file class (Verify row 1)

A change to `src/pay.go` asserts, in a code comment and a doc, that a payment "settles in one
block". The claim is false: settlement takes up to three blocks. This is one false-claim
class with occurrences in more than one file.

### A.1 — first-pass review packet (complete)

```
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
  - src/pay.go:42        the code comment
  - docs/overview.md:9   the same assertion in prose
excluded (read, not in scope):
  - vendor/x/readme.md:3 an incidental substring match in a third-party readme
verdict: CHANGES_REQUESTED — the false settlement claim (class settles-in-one-block)
         appears at src/pay.go:42 and docs/overview.md:9; fix both.
```

**Expected score:** the packet records the search, its scope, its exclusions and the
revision, and names ALL in-scope occurrences in one verdict. It does not leave the second
occurrence for a later round.

### A.2 — the "incomplete search" negative

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

---

## Case B — required, unrelated, and safety findings (Verify row 2)

A change adds a new settlement mode to `src/pay.go`.

### B.1 — omitted required operator-state table → BLOCKING

```
finding: the brief requires an operator-state table documenting the new mode's states in
         docs/ops/runbook.md; the diff does not add it.
location: docs/ops/runbook.md (untouched by the diff)
scope basis: acceptance-obligation
```

**Expected score:** BLOCKER. Untouched is not automatically exempt — a required deliverable
is in scope even when the worker omitted it from the diff.

### B.2 — unrelated pre-existing prose → FOLLOW-UP

```
finding: NOTICE:1 carries a stale copyright year.
location: NOTICE:1
scope basis: none — pre-existing, unrelated to the settlement change
```

**Expected score:** FOLLOW-UP, filed as a linked issue. It names no concrete failure of this
change and holds the PR of nothing.

### B.3 — safety consequence outside the edited lines → BLOCKING

```
finding: the new mode reaches a delete path in src/gc.go whose guard the change disarms;
         the guarded delete is now fail-open.
location: src/gc.go:88 (outside the diff's edited lines)
scope basis: safety-consequence
```

**Expected score:** BLOCKER. A demonstrated safety consequence of the change blocks even
outside the edited lines.

### B.4 — co-location only → FOLLOW-UP

```
finding: src/pay_notes.md:2 has a typo; it shares the directory with the change.
location: src/pay_notes.md:2
scope basis: none — sharing a directory/substring is not a basis
```

**Expected score:** FOLLOW-UP. Co-location is not scope.

---

## Case C — no silent promotion, and class continuity (Verify row 3)

### C.1 — a previously non-blocking occurrence is not promoted by an unrelated edit

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

### C.2 — a late sibling retains its original class and round count

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

| case | expected outcome | scored outcome | model / version | agree? |
|---|---|---|---|---|
| A.1 | complete packet names all in-scope occurrences together | | | |
| A.2 | incomplete search rejected — never certified clean | | | |
| B.1 | omitted required operator table — BLOCKER | | | |
| B.2 | unrelated pre-existing prose — FOLLOW-UP | | | |
| B.3 | safety consequence outside edited lines — BLOCKER | | | |
| B.4 | co-location only — FOLLOW-UP | | | |
| C.1 | no promotion without changed impact/new evidence | | | |
| C.2 | late sibling keeps class + round count | | | |

Any disagreement between the scored and expected outcome prevents acceptance.
