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
label is never applied on a could-not-check verdict.

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
