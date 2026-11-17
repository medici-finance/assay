---
id: DR-statusgen-32
date: "2026-11-16"
title: "Board drift loop closure (statusgen/32): delivered and re-homed todo rows are held out of Next-up and dispatch through the existing classifier, on merged Brief: trailers only, with unknown history kept could-not-check; proposed, not ratified"
consequence: major # The transcriber's classification.
decided-by: "human:<name>"
alternatives:
  - "A separate dispatch allowlist. Ruled out by the brief's Task 1: the existing foreign-owned and already-delivered classifier is reused in eligibility and board reconciliation."
  - "Treating a brief's discussion of foreign ownership as an ownership declaration. Ruled out by the brief's Task 1; text-heuristic classes stay notice-only and `homed-in:` remains the explicit declaration (the delivering pull request)."
  - "Counting an `Authors:` trailer as delivery. Ruled out by the brief's Task 2."
  - "Treating missing forge or history state as no delivery. Ruled out by the brief's Task 2: it stays could-not-check."
  - "A second writer for the Verified actor in the flip path. Not taken: `tools/desk/cmd/deskevidence/flip.go` is unchanged because inspection showed it passes the verifier identity only as a binding check against a stamp statusgen already derives from the accepted run (the delivering pull request's decision on the flip path)."
  - "Inventing targets for ambiguous register links. Ruled out by the brief's Task 4: only a link that maps uniquely to its durable entry is rewritten; ambiguous and unmapped links are listed."
accepted:
  - "A subject-only match to a brief is reported as unconfirmed and is not held (the delivering pull request)."
  - "Old runner-attribution notices remain as historical records where correcting them would rewrite closed evidence (the brief's Task 3)."
  - "Evidence, Verify table rows, fences and code spans are never touched by the register-link repair (the delivering pull request)."
---

## State and provenance

**Approval state: proposed, not ratified.** The `decided-by` stamp above is the public
placeholder and names nobody. This record carries no `ruling:` link, so `statusgen --corroborate`
reports it `placeholder-unratified` until the driver posts, on the decision issue, an unedited
comment from the driver's own login that names `DR-statusgen-32`. The `ruling:` link to that comment, and
the brief's `design:` key, are added only after it exists.

**No human decision is recorded yet.** The brief's decision issue,
[#5836](https://github.com/medici-finance/assay/issues/5836), is open with no ruling. The brief's
`## Human decision` offers three options: approve the implementation as verified, request the
identified corrections before approval, or hold the change. Its stated default is none: it
blocks until answered, and no sign-off is inferred. This record does not choose among them.

**Written after implementation.** statusgen/32 was delivered by
[#4137](https://github.com/medici-finance/assay/pull/4137), merged 2026-10-24, before this record
existed. It is written on demand under the ruling on #5145, which limits back-filled design
records to held briefs that block other work: statusgen/35 declares `depends:` on statusgen/32
and is `todo`.

This record does not mint a decision. It is taken from the brief, the delivering pull request and
the code on main; nothing here adds rationale beyond what those record.

## The design

As the brief asks and #4137 delivered:

1. A todo row delivered by a merged pull request carrying a `Brief:` trailer (with no
   partial-delivery claim), or a re-homed pointer row, is held out of Next-up and the dispatch
   queue and named in an "Already delivered or re-homed" banner.
2. Delivery evidence is read from merge history in `statusgen/mergedstatus.go`: the commit graph
   from `git rev-list --parents` and messages from one length-prefixed `git cat-file --batch`, so
   no commit text is parsed for structure or reaches git as an option. A failed read holds no
   merged-delivery row and reports could-not-check.
3. The Verified actor derives from the latest accepted strict-PASS run in the recorded Evidence;
   the attribution notice names the cell that record derives.
4. `--register-links` (`statusgen/registerrefs.go`) rewrites a typed register link that points at
   a generated view only when exactly one entry carries the id.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not choose among the brief's three decision options. It does not record a verification
result or move the brief's board row. It does not decide statusgen/35.
