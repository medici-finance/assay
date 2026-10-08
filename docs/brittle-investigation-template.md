---
module: cmd/example
s-row: S-example
class-issue: "#0"
mark-date: "2026-01-05"
divergence: drifted
recommendation: reconcile
next-act: example/02
end-state: "the first monthly pass after example/02 merges does not nominate cmd/example, and class #0 records no counted instance at or after example/02's merge revision"
tier: strong
date: "2026-01-12"
---

# Brittle investigation template

A **brittle investigation** is the next act a brittle mark binds to (`docs/contracts.md`
§Brittle marks). It reads what a module was for, what has happened to it since, and says
which of three things is true: the code drifted from the intent, the intent stopped matching
the need, or the intent is right and the implementation wrong. Its output is a
recommendation with a deletion bundle, not a patch. It sits beside `docs/brief-template.md`:
a brief says what to build, an investigation says whether the module needs building again.

**How to use this file.** Copy it to `docs/investigations/<yyyy-mm-dd>-<module-slug>.md`
(`<module-slug>` is the module path with `/` turned into `-`, e.g. `cmd-example`). Keep the
one frontmatter block and its keys, replace the values, and under each of the six sections
replace the instruction and the worked example with your own reading. The frontmatter above
is the worked example's, filled in. Everything named `example` in this file (`cmd/example`,
`S-example`, `DR-example-token`, the briefs `example/01` and `example/02`, the policy file
`docs/example-write-policy.md`, class issue `#0`, and the revisions `r1`–`r10`) is fictional;
a real investigation writes full commit ids where the example writes `r<N>`.

**Trigger.** A class issue labelled both `design-owed` and `brittle`. It rides the strong-tier
un-briefed-issue lane that a design-owed class already uses (`worker-desk` §Un-briefed
issues). With `brittle` present the deliverable is this investigation file **first**; a design
brief follows **second, and only** when the recommendation is `redesign`. The mark is made at
the monthly pass, so the trigger is that pass's output, never a CI event.

**Bounds.** Strong tier, read-only on the code, one file out. The history table in
`## What happened` holds the window's fix commits plus the originating commits, and nothing
else. **NEEDS_CONTEXT rule:** when the originating brief cannot be found (no `Brief:` trailer
on the first commits, none named by the PR that merged them, no brief file), write no investigation
file: report `NEEDS_CONTEXT` on the class issue, naming each read that came back empty. Never
reconstruct an intent from the code; the code is what the intent is being checked against.

**Frontmatter keys.**

- `module` — the marked module, exactly as the mark row in `docs/contracts.md` names it.
- `s-row` — the `S-` row that owns the module's meaning, or `none` when no row names it.
- `class-issue` — the `error-class` issue carrying `design-owed` and `brittle`.
- `mark-date` — the monthly pass that made the mark (the row's `since` cell).
- `divergence` — exactly one of `drifted`, `intent-changed`, `intent-right, implementation-wrong`, `none`.
- `recommendation` — exactly one of `reconcile`, `redesign`, `accept`, `clear`.
- `next-act` — a brief id (`reconcile`, and the design brief on `redesign`), a `DR-` id
  (`accept`, and the amendment on `redesign`), or `clear`.
- `end-state` — one line: what the next monthly pass must show for the mark to clear.
- `tier` — always `strong`.
- `date` — the day this file was written.

**The three divergences** (exactly one is chosen; `none` is a legal answer that clears the mark):

- `drifted` — the intent holds; the fixes moved the code off it.
- `intent-changed` — the need moved; the intent as written is no longer what the module must do.
- `intent-right, implementation-wrong` — the intent holds and the original implementation
  never met it.
- `none` — no divergence: the counted instances do not trace to the module's shape.

**The three recommendations**, each with its next act and its owner (`clear` is the fourth outcome):

- `reconcile` — bring the code back to the intent. Next act: one fix brief whose `retires:`
  lists every layer the fixes added that the intent does not need (the deletion bundle).
  Owner: the stream that owns the module's `S-` row. Fowler's refactor-first default. A
  bundle entry that is a security control or its CI assertion is human-gated: name it in a
  `needs-decision` issue beside the fix brief; it never retires on the owner's lane alone.
- `redesign` — the intent must change. Next act: a DR amendment and a design brief (title
  given here), preferring a strangler seam over a rewrite; a rewrite is proposed only when this
  investigation shows the seam cannot be cut (Foote & Yoder's "Reconstruction" is the last
  resort). Owner: the DR's design-approval authority for the amendment; the design brief is
  authored at strong tier.
- `accept` — the drift is the better design. Next act: amend the DR and the `S-` row so the
  record matches the code, and clear the mark. Nothing is coded. Owner: the DR's
  design-approval authority.
- `clear` — only with divergence `none`. Next act: `clear` (nothing is authored); the mark
  clears by the §Brittle marks clearing rule. Owner: the monthly pass, which reads the `end-state:` line.

`accept` and `clear` code nothing, and they are real outcomes, not consolation prizes: an
investigation that can only recommend work is a patch generator with extra steps. Choose
them whenever the evidence says so. The usual pairing is `drifted` → `reconcile` (or `accept`
when the drift is the better design), `intent-changed` → `redesign` (or `accept` when the code
already serves the new need and only the record lags), `intent-right, implementation-wrong`
→ `reconcile`, `none` → `clear`; `## Options` argues the pairing, it does not assume it.

**Replayable reading.** The file records the inputs its conclusions were read at, so a later
reader can tell whether they still hold. In `## Intent`, the labelled fields `as-of:`,
`source-revisions:`, `dependency-references:`, `reconciled-assumptions:`,
`unresolved-questions:`, `evidence-gaps:` and `supersedes:` carry them. Number each conclusion
(`C1`, `C2`, …) and end it with `rests on:` naming the evidence, assumption or reference it
depends on. Mark what you saw `observed:` and what you make of it `read as:`. Missing evidence
stays an `evidence-gaps:` entry: never infer it, and never round it to a pass.

**Reuse and revalidation.** The reading is reusable only at its recorded inputs. Before a later
act reuses it (the fix brief, the DR amendment, or on `redesign` the refactor oracle), run
`git diff --stat <as-of> refs/remotes/origin/main -- <each dependency-reference path>`. A
changed reference makes every conclusion that rests on it `revalidate`, even when no file
of the module changed: a conclusion depends on what it cites, not on which files the module
touches. Never edit the old file. Write a superseding revision (a new dated file whose
`supersedes:` names the old one), or record an explicit affected-scope revalidation where the
reuse happens (naming the old file, the changed reference, the conclusions re-checked and the
result). An empty `dependency-references:` means coverage is unknown: reuse is could-not-check,
never clean.

## Intent

What was the module for? Read the originating brief and the last redesign brief, the `DR-`
record and the `S-` row, and quote them; never paraphrase. Reads, in order:

1. Originating commits, for each file the mark row names (the nominated file first):
   `git log --follow --reverse --format='%H %s' -- <path> | head -3`
2. The `Brief:` trailer on each: `git show -s --format=%B <sha> | grep -E '^Brief:'`. None
   there (a squash merge usually keeps the trailer only in the PR body): find the PR `<N>` that
   merged it with the forge's commit-to-PR lookup,
   `gh api repos/<owner>/<repo>/commits/<sha>/pulls --jq '.[0].number'` (or the forge's
   equivalent), and read the trailer from that PR's body:
   `gh pr view <N> --json body --jq .body | grep -E '^Brief:'`. Offline, take `<N>` from the
   subject: the `(#N)` suffix when `<sha>` is itself on `git rev-list --first-parent refs/remotes/origin/main`
   (a squash commit), else the `Merge pull request #N` subject of the oldest merge commit in
   `git log --first-parent --ancestry-path --merges --format='%H %s' <sha>..refs/remotes/origin/main`.
   A PR body with no `Brief:` line may still name the brief: a `<stream>/<NN>` id in the PR
   title, or an `Issue:` trailer whose issue names it. Record which of these the id came from.
   No PR resolves, or the PR names no brief: an `evidence-gaps:` entry for that commit, never a guess.
3. The brief file: `ls docs/streams/<stream>/brief-<NN>-*.md`. Quote its `why:` and the facts
   of its `## Context` that state what the module must do.
4. The last redesign brief: list every commit that touched the file, with its trailer, by
   `git log --format='%h %ad %(trailers:key=Brief,valueonly,separator=%x2C)' --date=short refs/remotes/origin/main -- <path>`.
   A line with no brief (two fields) is not dropped: resolve its PR and the brief it names as
   in read 2, and a commit that still resolves to no brief is an `evidence-gaps:` entry, so the
   list's gaps are visible. The newest brief on the resolved list whose
   `design-fit:` `owner:` names this module is the last redesign. None: record
   `last-redesign: none`, and the originating brief is the intent.
5. The `S-` row: `grep -n '^| S-' docs/contracts.md | grep -F '<module path>'`; then the
   `DR-<slug>` record it names, `docs/streams/decisions/DR-<slug>.md`.

Reads 1–3 finding no brief is the NEEDS_CONTEXT case above. Then fill the reading record:

- `as-of:` the revision every read ran at (`git rev-parse refs/remotes/origin/main`).
- `source-revisions:` the revision of each source read, and the class issue's comment state.
- `dependency-references:` every file, policy, rule row or decision record a conclusion rests on
  that is not one of the module's own files, each at its revision. Empty means unknown coverage.
- `reconciled-assumptions:` each assumption the reading needed, numbered `A1`, `A2`, …, with what
  settles it and the reference it rests on.
- `unresolved-questions:` what the record does not settle, numbered `Q1`, `Q2`, ….
- `evidence-gaps:` each source that was unavailable, with what it would have settled.
- `supersedes:` the investigation file this one replaces, or `none`.

**Worked example.**

- `as-of:` r7 (`refs/remotes/origin/main` on 2026-01-12).
- `source-revisions:` r7 for every file read; class `#0` as of 2026-01-12 (latest block: g2, at r6).
- `dependency-references:` `docs/streams/decisions/DR-example-token.md` @ r7 (the no-copy
  decision); the `S-example` row in `docs/contracts.md` @ r7 (the owner);
  `docs/example-write-policy.md` @ r7, policy P1 (a write budget of 5000 token reads per hour).
- `reconciled-assumptions:`
  - A1: a token read per write fits the write budget: P1 allows 5000 reads per hour and class
    `#0`'s g2 block records a peak of 900 writes per hour. Rests on: `docs/example-write-policy.md` @ r7.
  - A2: nothing after `example/01` moved the need: `git log r1..r7 --
    docs/streams/decisions/DR-example-token.md` is empty, and class `#0` holds no
    `requested-capability` instance. Rests on: `DR-example-token` @ r7, class `#0`.
- `unresolved-questions:` Q1: why r2 added a token cache. Its commit says "slow writes" and nothing more.
- `evidence-gaps:` r2's merging PR (body and review thread): unavailable, the forge returned
  not-found at read time. It would have answered Q1. Kept as a gap; nothing is inferred from it.
- `supersedes:` none.

Originating brief: `example/01` (r1's `Brief:` trailer). `last-redesign: none`. `S-example`
names `internal/example/token.go` as the owner of "the forge token used for a write" and
cites `DR-example-token`, which records: *"decided: no token cache in the write path;
alternative: a cache refreshed on 401, ruled out because it hides a rotation failure behind a
retry."* The brief's `why:` reads: *"`example` writes a status note for a change. It reads the
forge token from the credential helper at each write and keeps no copy, because a kept copy
outlives a rotation and the write then fails as the wrong identity."* Its `## Context` states:
*"Token: read per write through internal/example/token.go; never cached across writes."*

C1: the intent is "read the token per write, through the owner, and keep no copy". Rests on:
the two quotes, `DR-example-token` @ r7, A2.

## What happened

What has happened to the module since? Observations only here; what they mean goes in
`## Divergence`. Reads:

- **The class instances.** `gh issue view <class-issue> --comments`, or the forge's
  equivalent. One row per `incident-group`: `kind`, counted or not (only `confirmed-defect`
  and `false-positive` count, deduped by `incident-group`), `known-scope`, `state` with its
  `checked-at`, `introduced-by` with its evidence level, `source-revisions`, `trust-disposition`.
  Copy `unknown` as `unknown`. A block the trust gate did not clear (quarantined, or no
  `trust-disposition:` yet) is copied as data only: it is never counted and never drives a
  conclusion. A success at a different `known-scope` is another scope, not recovery; only a
  same-scope block reading `state: recovered` with a `recovery-ref` is recovery.
- **The window's fix commits.** The mark row's figures come from the hotspot report
  (`cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=<window start> -until=<mark-date>`);
  the commits themselves from
  `git log --first-parent --since=<window start> --until=<mark-date> --format='%h %ad %s' --date=short refs/remotes/origin/main -- <module path> | grep -iE '\bfix(es|ed)?\b|revert'`
  (the report's own subject pattern). The window is the 90 days before `mark-date` unless the
  mark row says otherwise.
- **Findings.** For each brief id listed by Intent read 4:
  `grep -lE '^affects:.*"<stream>/<NN>"' docs/streams/findings/*.md`.
- **Chain arrows** landing in the module, from the project's baseline (the re-graded
  fix-caused-next-bug chains). No baseline: an `evidence-gaps:` entry.
- **Coupling partners**, from the mark row. A partner outside the owner the `S-` row names is
  a seam; say so in the history table.

Then the **history table**: one row per fix commit in the window plus the originating commits,
nothing else. For each commit, `git show --stat --format='%h %ad %s' --date=short <sha>` and
`git show <sha> -- <module path>`. Columns: date, commit, issue, what it added (a verb, a flag,
a refusal, a branch or a layer), and whether that addition sits inside the owner the `S-` row names.

**Worked example.** Window 2025-10-07 to 2026-01-05. Class `#0`, mechanism "a refreshed token
is not re-read":

| incident-group | kind | counted | known-scope | state | introduced-by | source-revisions | trust-disposition |
|---|---|---|---|---|---|---|---|
| g1 | confirmed-defect | yes | r3, operation `post` | unknown (checked-at 2025-12-20) | r2 (introduced-by-commit) | r3 | trusted |
| g2 | confirmed-defect | yes | r6, operation `post` | active (checked-at 2026-01-04) | r2 (shared-mechanism) | r6 | trusted |
| g3 | intended-control | no | r6, the stale-cache refusal's text | — | — | r6 | trusted |

observed: g1's block also records a success at r5 on operation `edit`. That is another
`known-scope`, not recovery; no same-scope re-check of `post` at r3 exists, so g1 stays
`unknown` (an `evidence-gaps:` entry above).

| date | commit | issue | what it added | inside `S-example`'s owner? |
|---|---|---|---|---|
| 2025-09-15 | r1 (originating; `Brief: example/01`) | — | the verb; a per-write token read through `internal/example/token.go` | yes |
| 2025-10-20 | r2 `fix(example): slow writes, cache the token` | none recorded (PR unavailable) | layer: a token cache, `cmd/example/cache.go` | no |
| 2025-12-08 | r4 `fix(example): refresh the cached token on 401` | g1 | branch: refresh-and-retry on 401 | no |
| 2026-01-02 | r6 `fix(example): refuse a stale cache, add --no-cache` | g2, g3 | flag: `--no-cache`; refusal: the stale-cache refusal | no |

observed: findings, none (`example/01` is named by no `affects:` line). Chain arrows, one: the
baseline grades r2 → g1 at `introduced-by-commit`. Coupling: `cmd/example/write.go` co-changes
with `internal/forgeclient/retry.go` in 5 of 7 commits; that partner is outside the owner, so
r4's retry branch crosses a seam.

## Divergence

Which divergence holds? Choose exactly one, and cite the evidence that settles it:

- `drifted` — settled when C1 still describes the need (no dated ruling, DR amendment or
  consumer requirement after the originating brief moved it), the originating commit met the
  intent (its Verify and Evidence at that revision), and the counted instances trace
  (`introduced-by`) to layers the fixes added outside the intent.
- `intent-changed` — settled only by a dated artifact after the originating brief that moved
  the need: a ruling, a DR amendment, a consumer's new requirement, `requested-capability`
  instances. With no such artifact it is an `unresolved-questions:` entry, not a reading.
- `intent-right, implementation-wrong` — settled when the intent holds and the counted
  instances trace to the originating commits themselves: the first implementation never met
  the intent, and the fixes are attempts to reach it.
- `none` — settled when the counted instances do not share a cause in the module's shape:
  each fix closed its own cause and added no layer outside the owner. It clears the mark.

Write the divergence as `read as:` over `observed:` lines, then the **strongest competing
explanation** and the **next discriminating check** that tells the two apart: a command or an
artifact. Run it if it is a read. If it cannot run, or rests on an `evidence-gaps:` entry, say
so: the reading is then provisional on that gap, and the gap stays open.

**Worked example.**

observed: r2, r4 and r6 each added a layer outside the owner (history table); the counted
instances g1 and g2 trace to r2, the cache, the very alternative `DR-example-token` ruled out;
r1's Evidence passed its no-copy row at r1.
read as: `drifted`.
C2: the code drifted from the intent. Rests on: C1, A2, history rows r2/r4/r6, g1 and g2's
`introduced-by`.
competing explanation: `intent-changed`. The "slow writes" r2 fixed may have been a real move
in the need, a budget the per-write read broke; the cache would then be the new requirement.
discriminating check: does a per-write read fit the write budget? At r7, yes: A1 (5000 reads per
hour allowed, a peak of 900). The budget did not need the cache. Q1 stays open, since r2's PR is
an evidence gap, but it does not decide the reading: no recorded need moved, and A1 holds.

### Intervening-change example

At r9 (2026-01-20) a different brief changes `docs/example-write-policy.md` from P1 to P2: a
token read now costs ten units of the same 5000-per-hour budget, so a peak of 900 writes needs
9000. The next act's assembly starts at r10: the fix brief `example/02` here, and the refactor
oracle had the recommendation been `redesign`.

observed: `git diff --stat r7 r10 -- cmd/example internal/example` is empty: no file of the
module changed. `git diff --stat r7 r10 -- docs/example-write-policy.md
docs/streams/decisions/DR-example-token.md docs/contracts.md` shows the policy file changed.

read as: A1 fails under P2. C2 loses its answer to the competing explanation, so C2, C3 and C4
are `revalidate` at r10 and are **not current**. C1 and the history table are observations at
r7 and stand.

Why file non-overlap is not enough: C2 rests on A1, and A1 rests on the policy by reference.
The module's files never mention the policy, so no diff over them can see the change; only the
`dependency-references:` list does.

What happens next: this file stays as written, as-of r7. `example/02` is not authored from its
C2–C4. A superseding revision, `docs/investigations/2026-01-21-cmd-example.md` with
`supersedes: docs/investigations/2026-01-12-cmd-example.md`, re-reads the divergence under P2.
`intent-changed` is now a live candidate; if it holds, the recommendation becomes `redesign` and
the oracle is assembled from the superseding revision, never from this one.

## Options

Weigh each recommendation against the reading, in this order: `reconcile` first (Fowler's
refactor-first default), then `redesign` (a strangler seam before a rewrite; a rewrite only
when the seam cannot be cut, Foote & Yoder's "Reconstruction" being the last resort), then
`accept` (the record moves to the code), then `clear` (only on divergence `none`). For each:
what it would retire, what it would keep, and what it would cost. Name the **deletion
bundle**: every layer in the history table that the intent does not need, with its commit.

**Worked example.**

- `reconcile`: retire the token cache (r2), the refresh-on-401 branch (r4), the `--no-cache`
  flag and the stale-cache refusal (r6); keep the per-write read through the owner. One fix
  brief; the budget holds by A1.
- `redesign`: not supported. The intent holds (C1, A2); amending `DR-example-token` to admit a
  cache would re-admit the alternative it ruled out, and g1 and g2 are the failure it predicted.
- `accept`: rejected. The drift is not the better design: g1 and g2 are its cost.
- `clear`: not available; the divergence is not `none`.

C3: the deletion bundle is {`cmd/example/cache.go` (r2), the 401 refresh branch (r4), the
`--no-cache` flag and the stale-cache refusal (r6)}. Rests on: the history table, C1, and C2
through A1 (the cache may go only while the budget holds without it). The stale-cache refusal
guards only the cache retired with it, so it is no security control and needs no `needs-decision`.

## Recommendation

Exactly one of `reconcile`, `redesign`, `accept` or `clear`, matching the frontmatter, with the
conclusion it rests on. Add `single-point-of-failure:` when the module is on a core surface (the
project layer's definition): the one control the recommended design depends on, and the layer
behind it or `NONE`. Repeat the frontmatter's `end-state:` line: what the next monthly pass must
show for the mark to clear, checkable by that pass without asking anyone.

**Worked example.**

C4: `reconcile`. Rests on: C2, C3.
`single-point-of-failure:` not required; `cmd/example` is on no core surface in this example's project.
`end-state:` the first monthly pass after `example/02` merges does not nominate `cmd/example`,
and class `#0` records no counted instance at or after `example/02`'s merge revision.

## Next act

The act, its id and its owner, from the recommendation list above. Link the act to what it should
change: `expected-observable:` (what will be seen when it worked), `acceptance-test:` (the check
that shows it, at the scope the instances failed at), and `outcome: pending` until that check
has run on merged code. A rehearsal or a PR closed unmerged is not recovery, and an instance's
`unknown` state stays `unknown` until a same-scope re-check.

**Worked example.**

`next-act:` `example/02`, a fix brief titled "cmd/example: read the token per write again",
whose `retires:` is C3's deletion bundle. Owner: the stream that owns `S-example`.
`expected-observable:` a token rotated between two `post` writes makes the second write as the
new identity: the operation g1 and g2 failed on.
`acceptance-test:` `example/02`'s Verify row that rotates the token between two `post` writes and
asserts the second write's identity, shown failing before the fix.
`outcome: pending`: no applicable verification exists yet; g1 stays `unknown`.
