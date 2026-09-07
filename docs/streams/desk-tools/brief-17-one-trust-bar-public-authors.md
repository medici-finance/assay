---
brief: desk-tools/17
title: "One trust bar for public-repo authors — `deskboard` classifies on the same predicate `deskpost` gates on"
why: >-
  Two desk tools answer the same question — "may this PR enter the review loop?" — with two
  different predicates. `deskboard`'s PR classifier applies a STRICTER bar on a risk-classed
  (public/internal/unknown) repo than `deskpost` applies before it posts the verdict, so a
  trusted shared automation login authoring a PR on a public repo is listed as
  EXTERNAL / UNBLESSED by the board — no ACTION row, invisible to the review loop — while
  `deskpost` would happily post a verdict on that same PR if anything ever reached it. The
  work stalls with no failure anywhere: the board says "nothing to review", the queue drains
  to zero, and a real PR sits unreviewed until a human notices. One question deserves one
  answer, and the answer belongs in one place.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief RELAXES an author-trust bar on public repositories: after it, every login in the
  configured trusted-logins set is a trusted AUTHOR for review/verdict purposes on a public
  repo, where today only role Apps and mapped humans are. The relaxation is a recorded
  maintainer ruling (2026-09-06), and the reasoning behind it is that review-trust and
  merge-authority are different questions — a model verdict on a public PR changes nothing a
  human does not then merge. But a trust-boundary change is not a brief a model signs off on
  its own reading of, however mechanical the diff: the human is confirming that the ruling as
  implemented still refuses an UNLISTED author (Verify row 4, the negative control) and that
  no OTHER gate silently inherited the widening.
issues: []
schema: brief-v1
authored: 2026-09-06 by an authoring session, from a maintainer ruling recorded 2026-09-06
sources:
  - "Maintainer ruling, 2026-09-06: align the board's author bar to the post gate. Every login in the configured trusted-logins set (and the blessing authority) is a trusted AUTHOR on public repos for review/verdict purposes; the stricter public-only bar is removed. Merge and decision authority are unchanged and remain the human's — a branch-protection matter, not a review-trust matter."
  - "freshness-checked 2026-09-06 @ 0af8093 (origin/main) — `tools/desk/cmd/deskboard/board.go` § classifyPR still swaps the predicate on `deskkit.VisibilityRiskClassed(repo)`; `tools/desk/internal/deskkit/trust.go` § `TrustedPublicAuthor` is unchanged; `tools/desk/cmd/deskpost/github.go` § `trustGate` still admits on `deskkit.TrustedAuthorID` alone with no visibility branch. The divergence is live."
  - "Existing tests that pin today's behaviour and must be re-pinned, not deleted wholesale: `tools/desk/internal/deskkit/trustpublic_test.go`, `tools/desk/cmd/deskboard/publicauthorgate_test.go`."
exec-tier: strong
exec-tier-why: >-
  (c) trust plumbing — a subtle error here widens or narrows who may have a verdict posted in
  the reviewer App's name, and a happy-path test on a role-App author passes either way; and
  (b) correctness is the AGREEMENT of two components' predicates, not one component's output.
consumers:
  - "tools/desk/cmd/deskpost/github.go (trustGate / prTrustGate): out-of-scope — deskpost is the reference bar this brief aligns TO; it is read, asserted against, and left unchanged."
  - "tools/desk/internal/deskkit/trust.go (TrustedHumanAuthor): out-of-scope for BEHAVIOUR — the accountable-human axis (the board's review-neglect metric) keeps excluding shared machine accounts and is not widened here; only its doc comment, which defines its excluded set by reference to the retired function, is rewritten to stand on its own terms."
  - "tools/desk/README.md § trust gate, § quarantine visibility: follow-up in this brief's own Task step 4 (the documented bar must not outlive the code)."
---

# Brief 17 — One trust bar for public-repo authors

## Dependencies
None.

## Context

files:
- `tools/desk/cmd/deskboard/board.go` (`classifyPR` — the predicate swap is removed)
- `tools/desk/internal/deskkit/trust.go` (`TrustedPublicAuthor` retired; `TrustedHumanAuthor`'s
  doc comment rewritten to define its own excluded set)
- `tools/desk/internal/deskkit/trustpublic_test.go` (re-pinned to the new bar, or replaced)
- `tools/desk/cmd/deskboard/publicauthorgate_test.go` (re-pinned; the negative control stays)
- `tools/desk/README.md` (§ trust gate — the "PUBLIC repos have a higher author bar" text)

facts (all read at `0af8093`, 2026-09-06):
- `classifyPR` computes `authorTrusted := deskkit.TrustedAuthor(p.Author.Login)` and then
  OVERWRITES it with `deskkit.TrustedPublicAuthor(p.Author.Login)` whenever
  `deskkit.VisibilityRiskClassed(repo)` is true. `VisibilityRiskClassed` is fail-closed:
  only a KNOWN-private repo answers false, so public, internal AND unknown repos all take
  the stricter branch.
- `TrustedPublicAuthor` admits ONLY (a) a configured bot slug in its `<slug>[bot]` /
  `app/<slug>` rendering and (b) a login in the human-name→login map. It deliberately
  refuses a login admitted to the trusted-logins set as a plain human — that refusal is the
  behaviour this brief removes.
- `deskpost`'s gate is `trustGate` (`tools/desk/cmd/deskpost/github.go`), which admits on
  `deskkit.TrustedAuthorID(login, id)` — the trusted-logins set, with the numeric id checked
  where the surface carries one — and has NO visibility branch at all. It is the bar to align to.
- The board's PR list comes from the `gh` CLI, whose JSON carries `author.login` and no
  numeric user id (`prBase.Author` has a `Login` field only). `TrustedAuthorID(login, 0)` is
  documented to fall back to login-only on exactly such a surface, so `TrustedAuthor(login)`
  is the correct call here and is NOT a weakening relative to `deskpost` on the same input.
- The blessing path is unchanged and untouched: an author who fails the bar still reaches the
  review loop if the blessing authority has blessed the item (`prBlessed`), and an unblessed
  one is still listed under EXTERNAL / UNBLESSED with no ACTION row.
- An unconfigured roster trusts nobody. Every predicate in `trust.go` returns false when
  `EffectiveConfig().Configured()` is false, and that property must survive this change.
- `TrustedHumanAuthor` is a DIFFERENT axis (which PRs count as desk-review neglect) and its
  doc comment currently defines its excluded set by pointing at `TrustedPublicAuthor`. Its
  behaviour must not change; only the dangling reference is rewritten.

single-point-of-failure: after this change the ONE control standing between an untrusted
author and a desk-posted verdict on a public repo is the trusted-logins roster itself.
Layers behind it, each failing on a different signal in a different component: (1) the
outward-write gate — an outward write on a public repo is refused unless the repo is in the
configured write set (`IsAllowedRepo` + the public-repo gate), so a widened AUTHOR set still
cannot reach a repo the operator never listed; (2) unconditional risk-classing of public
repos — every PR on a public repo needs a `Security-Review: pass` at head before any flip,
whatever the diff touches, which is a content check rather than an identity check; (3) branch
protection and human merge — a verdict is not a merge, and no desk tool merges. The roster
itself is configured from outside every ref the tools evaluate, so a pull request cannot
author its own admission.

## Ground rules
- NEVER git push, trigger workflows, or contact the forge from a test or a Verify row.
- Stop at `implemented` — you do not set verified/done.
- Do NOT widen any OTHER predicate to match. `TrustedHumanAuthor` and `trustedContentAuthor`
  keep their present semantics exactly; a diff that touches their logic is out of scope.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Classifier.** In `classifyPR`, delete the `VisibilityRiskClassed` branch so the author
   bar is `deskkit.TrustedAuthor(p.Author.Login)` on every repo. Replace the block comment
   with one that states the new rule and WHY it is safe (review-trust is not merge-authority;
   the outward-write gate, the unconditional public-repo `Security-Review` requirement, and
   branch protection are the layers behind it). Leave the blessing fallback and the
   EXTERNAL / UNBLESSED row untouched.
2. **Retire `TrustedPublicAuthor`.** Delete the function from `trust.go` once it has no
   callers (confirm with `grep -rn TrustedPublicAuthor tools/`). Rewrite `TrustedHumanAuthor`'s
   doc comment so its excluded set is defined on its own terms — it still excludes bot
   renderings and a shared machine account admitted as a plain human — without referring to
   the deleted function.
3. **Tests.**
   - `trustpublic_test.go`: replace the narrowing assertions with the new contract. A login in
     the trusted-logins set is a trusted author on any repo; an unlisted login is not; an
     unconfigured roster trusts nobody.
   - `publicauthorgate_test.go`: a trusted SHARED automation login authoring a PR on a
     public/risk-classed repo is REVIEWABLE — it produces an ACTION row and does NOT appear in
     the EXTERNAL / UNBLESSED list. Keep and strengthen the negative control: an unlisted
     login on the same repo is still quarantined, and is admitted only by a blessing.
   - Add a PARITY test asserting the board and the post gate agree: for a table of logins
     (role App, mapped human, trusted shared account, unlisted account, empty login) the
     board's admission decision equals `deskkit.TrustedAuthorID(login, 0)`. This is the test
     that fails if the two predicates diverge again.
4. **Docs.** Update `tools/desk/README.md`'s trust-gate section: the "PUBLIC repos have a
   higher author bar" bullet is replaced by one bar plus a sentence naming the layers that
   remain (write set, `Security-Review` at head, branch protection / human merge). Update the
   `deskboard` usage text if it states the old bar.
5. **Changelog fragment** under `changelog/` (one bullet, `### Changed`).
6. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestTrustedSharedLoginIsReviewableOnPublicRepo$' -count=1` | exit 0 — a trusted shared automation login on a risk-classed repo yields an ACTION row and no EXTERNAL / UNBLESSED entry |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestBoardAndPostGateAgreeOnAuthorTrust$' -count=1` | exit 0 — the parity table passes for every login class |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestUnlistedAuthorStillQuarantined$' -count=1` | exit 0 — the NEGATIVE control: an unlisted login on a public repo is still EXTERNAL / UNBLESSED with no ACTION row, and is admitted only by a blessing |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTrustedAuthorUnconfiguredFailsClosed$' -count=1` | exit 0 — an unconfigured roster trusts nobody |
| 6 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole suite, including the untouched accountable-human and blessing tests |
| 7 | check:ci | `grep -rn 'TrustedPublicAuthor' tools/ > /tmp/tpa.out; test ! -s /tmp/tpa.out` | exit 0 — no residual reference, in code, tests or comments |
| 8 | check:ci | `gofmt -l tools/desk/cmd/deskboard tools/desk/internal/deskkit > /tmp/b17-fmt.out; test ! -s /tmp/b17-fmt.out` | exit 0 |
| 9 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The branch is removed but the board still quarantines an unlisted author's PR by accident of a second predicate | row 3 (parity table includes the unlisted class) |
| The widening sweeps in the unlisted/fork author too — the gate is removed rather than aligned | row 4, the negative control |
| The two predicates drift apart again in a later change | row 3 — the parity test is the standing guard, not a one-time assertion |
| An unconfigured roster starts trusting everyone because the fail-closed arm was refactored away | row 5 |
| `TrustedHumanAuthor`'s excluded set is quietly widened while its doc comment is edited | row 6 (its own tests are unchanged and must still pass) |
| A stale doc keeps promising a higher public bar that no longer exists | row 7 (comment references) + Task step 4; adequacy of the prose stays review-only |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: human. The reviewer answers two questions in the verdict: (1) what is the single control
now standing between an untrusted author and a desk verdict on a public repo, and are the
layers named in the Context's single-point-of-failure note actually independent of it?
(2) does row 4 fail if the change is over-applied — that is, would a diff that removed the
quarantine entirely be caught? A Verify table on which only trusted logins are exercised has
verified the widening and nothing else.
