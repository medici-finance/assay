# Validation — the third activity, distinct from review and verification

ISO 9001 clause 8.3.4 names three design-and-development controls, and insists an auditor
be able to tell them apart:

- **Review** — is this well-built? A quality judgement on the finished change, from an
  identity the implementer cannot write on its own behalf.
- **Verification** — do the outputs meet the inputs? A non-implementer re-runs the brief's
  `## Verify` table on merged trunk; merging is explicitly not verification.
- **Validation** — did the change achieve the purpose the requirement existed for, in the
  setting it will be used in?

Assay separated and structurally enforced the first two from the start. Validation was the
missing third activity — [`docs/iso9001-mapping.md`](iso9001-mapping.md) row 8.3.4 said so
in the repo's own words, and warned that answering all of 8.3.4 with "we code-review
everything" draws a finding. This page names validation as an activity so it is no longer
missing, and is honest about the part of it that remains the adopter's.

## What validation asks that verification does not

Verification asks *did we build the thing right* — do the outputs satisfy the inputs the
brief wrote down. Validation asks *did we build the right thing* — does the result meet the
**requirement** the work existed for, in its real setting. The two come apart exactly when
a change is correct against its brief and still does not achieve the ask: the Verify table
is green, the review passed, and the requirement is still not met.

Validation could not be named before the requirement object existed, because validation is
against a requirement's **acceptance criteria**. The REQUIREMENTS register
([`../spec/registers-v1.md`](../spec/registers-v1.md) §6, entries under
`docs/streams/requirements/`) is that object: a recorded ask, with the `acceptance:`
criteria that would settle whether the ask was met, ranked by `impact`. A brief cites the
requirement it was written against with the reserved `satisfies:` key. Validation is the
walk from a landed change back along that citation to the requirement's acceptance
criteria, asking whether each is actually met in use — not whether a Verify row for it went
green.

## Its mechanical floor

Validation is a judgement, but it is not floorless. Its mechanical floor is
[`brief-rules.md`](brief-rules.md) rule 43: a brief whose deliverable makes checkable
factual claims MUST carry at least one **dereferencing** Verify row — a row that resolves
something real (fetches the link, runs the command and compares the actual output against
the claim, checks the documented property against the live system), and so CAN fail on a
deliverable that is wrong-but-well-formed. A presence/formatting row cannot fail on a
confident falsehood in the right section at the right word count; a dereferencing row can.
That row is validation-flavoured — it tests the claim against the world, not the document
against a template — and it is where a requirement's acceptance criterion, written as "a
statement some observation would settle", becomes an observation.

So the floor is: **for each acceptance criterion that a dereferencing row could settle, a
brief satisfying that requirement should carry the row that settles it.** The row is the
verifiable part of validation; the register makes the criterion it validates against
rankable and rollable-up rather than trapped inside one brief's table.

## What validation does NOT establish

Named honestly, so the activity is not over-claimed:

- **A green dereferencing row is not full validation.** It settles the criteria an
  observation can settle. A requirement whose acceptance turns on human judgement, field
  use, or a setting the test harness cannot reproduce is validated by a person in that
  setting, and the row cannot stand in for them.
- **Validation is intended-use-specific and is therefore inherently the adopter's act**
  ([`iso9001-mapping.md`](iso9001-mapping.md) closing section). Nobody can validate for a
  use they are not in. What the tooling does is make it *cheap*: a known-answer corpus run
  in the adopter's own environment produces a dated record on their side, in their name.
  Where this repo ships such fixtures, that is what they are for.
- **The board does not measure it.** Like every lifecycle state, a validation record is an
  authored artifact with consistency linting, not an observation of ground truth
  ([`lifecycle.md`](lifecycle.md) §"The honest claim about the board"). It records that
  the walk was done and what it concluded; it does not independently observe the product in
  use.

## Where it sits in the lifecycle

Validation is a *post-merge* activity, alongside verification and after review — it needs
the change to have landed and, for the criteria that need a real setting, to be in use. It
is distinct from the design-approval gate ([`lifecycle.md`](lifecycle.md),
[`../spec/lifecycle-v1.md`](../spec/lifecycle-v1.md) §4.4), which sits *before* work begins
and asks whether the design was the right one to build. Design approval asks the question
up front; validation answers it after the fact, against the requirement's own criteria.
