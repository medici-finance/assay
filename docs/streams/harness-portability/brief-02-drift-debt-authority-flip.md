---
brief: harness-portability/02
title: Kill the drift debt — re-sync the bundle, flip the canonical home
why: >-
  The bundled skills are a port of an upstream source repo, measured tens of commits behind at
  authoring time, detected by plugindrift on every run and never re-ported. A second harness doubles
  any hand-ported surface, so before the method text gains a second consumer it must have exactly ONE
  home. Re-sync once, then flip authority: the bundle becomes canonical, the upstream repo consumes
  thin pointers, and the port relationship — the thing that drifts — ceases to exist.
wave: 0
depends: []
unblocks: ["harness-portability/04"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "plugindrift run 2026-08-07: the-desk / batch-fanout / verify-desk / pr-review-desk each tens of commits behind the pinned upstream commit; author-brief primary source (a local-git `~/.claude` checkout) UNREACHABLE by construction", "the 'which surface wins' ruling (a maintainer ruling, 2026-08-03): the bundled skill is authoritative on the METHOD; and the bundle is becoming the home of the method while the upstream repo becomes a consumer", "plugins/assay/PARITY.md re-sync procedure (steps 1-7)", "freshness-checked 2026-08-07: no re-port has landed since the v0.1.0 cut (the SOURCES.yaml source pins are unchanged)"]
consumers: ["the upstream repo's `.claude/skills/{the-desk,batch-fanout,verify-desk,pr-review-desk}/SKILL.md`: out-of-scope (the consumer cutover already landed on the upstream repo — the four bodies were removed outright, stronger than thin pointers; an upstream-repo change carrying the cross-repo pairing, not this PR's diff)", "`~/.claude/skills/author-brief/SKILL.md`: fixed-here (out-of-repo, rule-7 protocol — applied LAST, committed in the ~/.claude stopgap repo)", "the upstream repo's project-wrapper author-brief SKILL.md: out-of-scope (it is already a wrapper layering project specifics over the core; it keeps pointing at the core's new home)", "plugins/assay/SOURCES.yaml + PARITY.md: fixed-here (direction inverts; rows move to canonical-here declarations)", "tools/plugindrift: fixed-here only if the SOURCES.yaml restructure needs a schema addition; otherwise unchanged"]
exec-tier: strong
exec-tier-why: >-
  (a)+(b): re-porting tens of commits of upstream deltas requires judging, per hunk,
  method-content (port it) vs house-specific content (leave it, record why) — the
  PARITY.md intentional-changes discipline — and the authority flip is a cross-repo,
  cross-artifact consistency change.
---

# Brief 02 — Kill the drift debt, flip the canonical home

> **Note (re-home):** this brief's deliverables — the drift checker `tools/plugindrift`, the
> `plugins/assay/SOURCES.yaml` (planned) manifest, `plugins/assay/PARITY.md` (planned), and the bundled skill bodies —
> live in the neutral-core method-text layer that arrives with the sequenced tool/method-text
> de-house (see the stream README's re-home note). The Verify/Evidence commands below run in that
> source tree; the design record is re-homed here.

## Context

files:
- **amend** `plugins/assay/skills/{author-brief,the-desk,batch-fanout,verify-desk,pr-review-desk}/SKILL.md` — re-ported to current source
- **amend** `plugins/assay/SOURCES.yaml` (planned) + `plugins/assay/PARITY.md` (planned) — direction inversion
- **amend** `plugins/assay/RELEASE-NOTES.md` (planned) — bundle version bump + as-of
- **sibling PR (upstream repo)** `.claude/skills/{the-desk,batch-fanout,verify-desk,pr-review-desk}/SKILL.md` — thin pointers to the bundle

out-of-repo files: `~/.claude/skills/author-brief/SKILL.md` (rule-7 protocol: this
declaration is the claim — at most ONE out-of-repo brief in flight across all streams;
stage the edit as a diff in the PR; apply to the live file LAST, immediately before
`implemented`; commit the applied edit in the `~/.claude` stopgap repo)

facts:
- Measured drift (2026-08-07, `go run ./tools/plugindrift`): the-desk, batch-fanout,
  verify-desk and pr-review-desk each **tens of commits** behind the pinned upstream commit;
  author-brief primary source (a local-git `~/.claude` checkout) **UNREACHABLE** by
  construction, cross-reference behind.
- The re-sync procedure is written: `plugins/assay/PARITY.md` (planned) steps 1–7 (drift check →
  re-fetch → heading diff THEN body diff → port → update SOURCES.yaml commit/blob/as-of
  via `gh api ... -q .sha` → update PARITY → bump version). Follow it; its two recorded
  in-section regressions (a `git -C` scoping guard flattened, a mergeability
  precondition lost) are the class to watch for.
- The flip is enactment of a RATIFIED direction, not a new decision: the "which surface
  wins" ruling (a maintainer ruling, 2026-08-03) makes the bundled skill authoritative
  on the METHOD, and names the home trajectory explicitly.
- After the flip, SOURCES.yaml's five `files:` rows become canonical-here declarations
  (the existing `unported:` list with reasons, or a new `canonical:` list if plugindrift
  needs the distinction) — so `plugindrift` reports **0 behind, 0 unreachable, 0
  unaccounted** for those five and `--fail-on-drift` exits 0 when filtered to them.
  **Correction (2026-08-13, at implementation).** This authoring-time fact was wrong in
  two ways and the Verify/Evidence rows record the measured truth instead. (a) The
  `--fail-on-drift exits 0` half is repo-wide and is FALSE: `dailies` and
  `intake-desk` joined the bundle after this brief was authored, are pinned `files:`
  rows outside the flip, and both drift — strict mode exits 1. The zero-count claim
  holds only when filtered to the five files this brief flips. (b) The zeroes for
  those five mean "no comparison is performed", not "a comparison passed" — a
  `canonical:` row has no `source:`, so no drift verdict of any kind is computed for
  it. That is the intended consequence of ending the port relationship, and it is
  stated as a cap rather than left to read as a clean bill (Verify rows 8/8a).
  The reverse direction (upstream pointer staleness) does not meaningfully drift: a thin
  pointer carries no method text. The upstream repo already uses this exact pattern for
  its user-level skill deltas.
- Upstream merge discipline: the sibling PR obeys the upstream repo's single-writer and
  merge-serialize rules; it is a draft PR, human-merged, per the standard loop.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Branch push
  + draft PR is the deliverable shape; commits only per task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Port judgement is recorded, never silent**: every upstream hunk NOT ported gets a
  PARITY.md intentional-changes row with a reason. A silent drop is the in-section
  regression class PARITY already documents twice.
- The out-of-repo edit is applied LAST (see declaration above) — never incrementally.

## Task

1. **Re-sync**: run PARITY.md's procedure against the upstream repo's `main` (pin the
   exact commit you port from) for the four upstream-homed skills, and against the
   `~/.claude` repo for author-brief's portable core. Heading diff, then body diff of
   surviving sections.
2. **Flip authority**: restructure SOURCES.yaml so all five skills are declared
   canonical-in-this-bundle (with the porting history preserved in `notes:`); update
   PARITY.md's narrative from "snapshot of upstream" to "canonical home + consumers";
   bump the bundle version in RELEASE-NOTES.md with the drift-check result.
3. **Sibling upstream PR**: convert the four upstream skill bodies to thin pointers (name,
   trigger description, pointer to the bundle path + any upstream-specific deltas kept
   locally — mirror the upstream repo's existing thin-pointer pattern). Cite this PR's head
   SHA in the bundle-repo PR body and vice versa (the cross-repo pairing convention).
4. **Out-of-repo (LAST)**: `~/.claude/skills/author-brief/SKILL.md` gains a one-line
   canonical-home pointer to the bundle (its body may stay as the user-level convenience
   copy per the skill-org preference); commit in the stopgap repo.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `go run ./tools/plugindrift > /tmp/hp02r1.out 2>&1; grep -E -e 'BEHIND' -e 'UNREACHABLE' -e 'MOVED' /tmp/hp02r1.out \| grep -cE -e author-brief -e the-desk -e batch-fanout -e verify-desk -e pr-review-desk \|\| true` | count `0` — **none of the five flipped files** is behind, unreachable or moved after the flip, because they have no origin left to drift against. **Scope:** the count is deliberately filtered to the five files this brief flips. `dailies` and `intake-desk` were added to the bundle by later, independent briefs after this brief's scope was set; they remain pinned `files:` rows and DO report `behind`, so an unfiltered count is `2`, not `0`. Ending their port relationship is a later brief's scope. (`\|\| true` neutralises grep's exit-1 on the zero-match success path.) The exit code is NOT load-bearing here: `plugindrift` exits `0` even with drift present unless `--fail-on-drift` is passed (row 3 is the strict gate) — the count is the only evidence this row carries |
| 1a | **Positive control for row 1** — `git worktree remove --force /tmp/hp02-ctrl 2>/dev/null; git worktree prune 2>/dev/null; rm -rf /tmp/hp02-ctrl /tmp/hp02r1a.out; git -c advice.detachedHead=false worktree add /tmp/hp02-ctrl refs/remotes/origin/main > /dev/null 2>&1 \|\| { echo could-not-check; exit 1; }; (cd /tmp/hp02-ctrl && go run ./tools/plugindrift > /tmp/hp02r1a.out 2>&1); grep -E 'BEHIND' /tmp/hp02r1a.out \| grep -cE -e author-brief -e the-desk -e batch-fanout -e verify-desk -e pr-review-desk; git worktree remove --force /tmp/hp02-ctrl` | count `>= 4` — filtered to the same five files row 1 scopes, so control and assertion measure the same set. On the pre-change baseline (origin/main, run from a throwaway worktree) the same checker reports BEHIND, proving row 1's zero is the checker passing, not the checker broken. The leading `remove --force`/`prune`/`rm -rf` clears any worktree left registered by a prior run — `git worktree add` fails 128 against an already-registered path and a bare `&&` chain would then silently grep a stale output file. Requires network (the checker fetches GitHub); no network → the `\|\|` binds to `could-not-check`, never a silent stale-file green |
| 2 | `grep -cE '^ +commit: [0-9a-f]{40}$' plugins/assay/SOURCES.yaml \|\| true` | output `2` — exactly the two out-of-scope pinned rows (`dailies`, `intake-desk`) still carry a `source:` pin; none of the five flipped rows does. This form asserts the fact structurally (no literal SHA), so it decays only when the pinned set really changes (`\|\| true` neutralises grep's exit-1 on a zero-match) |
| 2a | **Positive control for row 2** — `git show refs/remotes/origin/main:plugins/assay/SOURCES.yaml \| grep -cE '^ +commit: [0-9a-f]{40}$'` | `7` (`> 2`) — the five flipped rows are pinned on the baseline and unpinned on the branch, so row 2's `2` measures removal. **The base ref is spelled `refs/remotes/origin/main` verbatim on purpose:** a checkout can carry a stale local branch literally named `origin/main`, so the bare form is ambiguous — git warns `refname 'origin/main' is ambiguous` and resolves it to the stray, silently reading a different baseline tree |
| 3 | `go run ./tools/plugindrift --fail-on-drift > /tmp/hp02r3.out 2>&1; echo $?; grep -E -e 'BEHIND' -e 'UNREACHABLE' -e 'MOVED' /tmp/hp02r3.out \| grep -cE -e author-brief -e the-desk -e batch-fanout -e verify-desk -e pr-review-desk \|\| true` | the trailing count is `0` — **strict mode finds no drift row naming any of the five flipped files.** The exit code is `1`, and that is expected, not a failure of this brief: strict mode is repo-wide and the two out-of-scope pinned rows (`dailies`, `intake-desk`) still drift. **This row does NOT claim repo-wide strict-mode green.** Repo-wide `--fail-on-drift` exit `0` becomes reachable only when those two rows are re-pinned or flipped in turn |
| 4 | Sibling PR exists on the upstream consumer repo (`gh pr list ... --state open --search "thin pointer bundle" --json number,title \| jq length`) | `>= 1`, and the bundle-repo PR body contains that PR's number + head SHA — check with `gh pr view --json body` run from this branch (no PR number argument: `gh` infers the current branch's open PR) |
| 5 | `grep -qiE 'canonical home' plugins/assay/PARITY.md && ! grep -qF 'the upstream copy remains canonical' plugins/assay/PARITY.md; echo $?` | `0` — PARITY names the bundle itself (not the upstream copy) the canonical home per task 2's own wording, AND the pre-flip inverted phrasing ("the upstream copy remains canonical until the consumer cutover") is gone. Bare token presence does not discriminate here: `canonical` and `consumer` are ALREADY present pre-flip, describing the pre-flip direction — see row 5a |
| 5a | **Positive control for row 5** (base ref verbatim — see row 2a) — `git show refs/remotes/origin/main:plugins/assay/PARITY.md > /tmp/hp02r5a.out; grep -qiE 'canonical home' /tmp/hp02r5a.out && ! grep -qF 'the upstream copy remains canonical' /tmp/hp02r5a.out; echo $?` | `1` — row 5's assertion is red on the pre-brief baseline (no "canonical home" phrase exists yet, and the inverted phrasing is present), proving row 5's eventual `0` is the narrative actually flipping, not a token that was there all along |
| 6 | **Out-of-repo applied + committed (machine-specific: the owner's machine)** — `git -C ~/.claude log --oneline -1 -- skills/author-brief/SKILL.md` | newest commit is this brief's pointer edit, dated on/after the PR merge. On any other machine this row is BLOCKED (machine-specific), not green. **Premise note (2026-08-13):** the owner ruling of 2026-08-13 — skill content lives in the repo, `~/.claude` files stay thin pointers — landed after this row was written and has already been applied by the owner's own migration commit. The surviving obligation is the pointer, not the body |
| 7 | **Neighbour row** — `cd tools/plugindrift && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp02r7.out 2>&1; echo $?` | `0` — the checker's own suite still passes against the restructured SOURCES.yaml schema |
| 7a | **Gate the first review missed** — `cd tools/plugindrift && out="$(gofmt -l . 2>&1)"; rc=$?; n="$(printf '%s' "$out" \| grep -c .)"; f="$(ls *.go \| grep -c .)"; echo "gofmt-rc=$rc unformatted=$n inspected=$f"` | `gofmt-rc=0 unformatted=0 inspected=4` — the CI job for this package runs a gofmt check that the Verify table did not name, so Evidence could read green while the gate stayed red. This form reports gofmt's own exit status AND the number of files it actually saw, so neither a failed run nor an empty inspection set can read as a pass |
| 8 | **The one check that survives the flip, with its own fail-first proof** — `go build -o /tmp/hp02pd ./tools/plugindrift && mkdir -p plugins/assay/skills/zzz-ctl && printf -- '---\nname: zzz-ctl\ndescription: control\n---\nbody\n' > plugins/assay/skills/zzz-ctl/SKILL.md; /tmp/hp02pd > /tmp/hp02r8.out 2>&1; echo "control-exit=$?"; rm -rf plugins/assay/skills/zzz-ctl; /tmp/hp02pd > /dev/null 2>&1; echo "restored-exit=$?"` | `control-exit=2` then `restored-exit=0` — coverage is the ONLY mechanical check the five flipped rows still have, and it can fail: an undeclared `skills/*/SKILL.md` is a hard error (exit 2) in advisory mode. Built binary, not `go run` — `go run` flattens every non-zero status to 1 and cannot tell 2 from 1 |
| 8a | **Negative control — the cap, stated as a measurement** — `go build -o /tmp/hp02pd ./tools/plugindrift && cp plugins/assay/skills/the-desk/SKILL.md /tmp/hp02td.bak && printf '\nBENT-CONTROL-LINE\n' >> plugins/assay/skills/the-desk/SKILL.md; /tmp/hp02pd --fail-on-drift > /tmp/hp02r8a.out 2>&1; echo "bent-exit=$?"; grep -c 'the-desk' /tmp/hp02r8a.out \|\| true; cp /tmp/hp02td.bak plugins/assay/skills/the-desk/SKILL.md; git diff --quiet plugins/assay/skills/the-desk/SKILL.md; echo "restored=$?"` | `bent-exit=1` (unchanged — it is 1 before the bend too, from the two out-of-scope pinned rows), grep count `0`, `restored=0`. **This row is expected to show the checker NOT reacting**, and that is the point: after the flip a canonical bundle body can be edited arbitrarily and `plugindrift` says nothing, because the row has no source to compare against. The brief buys the end of the port relationship at the price of the drift signal for those five; RELEASE-NOTES v0.2.0 states the same cap in prose |
| 8b | **Coverage fails closed, not open** — `go build -o /tmp/hp02pd ./tools/plugindrift && mv plugins/assay/skills /tmp/hp02skills.bak; /tmp/hp02pd > /tmp/hp02r8b.out 2>&1; echo "noglob-exit=$?"; mv /tmp/hp02skills.bak plugins/assay/skills; mv plugins/assay/SOURCES.yaml /tmp/hp02src.bak; /tmp/hp02pd > /tmp/hp02r8b2.out 2>&1; echo "nomanifest-exit=$?"; mv /tmp/hp02src.bak plugins/assay/SOURCES.yaml` | `noglob-exit=2` and `nomanifest-exit=2` — the two ways this check could silently degrade to a pass, both measured. A `skills/*/SKILL.md` glob matching **zero** files does not read as "nothing unaccounted, clean": the checker exits 2. An unreadable manifest exits 2, not `0 unaccounted`. Both are could-not-check (2), distinct from drift (1) and from clean (0) — which is why row 8 and this row use the built binary: `go run` collapses every non-zero status to 1, so the three-state result is unreadable through `go run` |
| 9 | **The brief's own routing claims, corroborated by machine** — `cd statusgen && go build -o /tmp/hp02sg . && cd .. && /tmp/hp02sg --root . --consumers --brief harness-portability/02 --base "$(git merge-base refs/remotes/origin/main HEAD)"; echo "exit=$?"` | `exit=0` and a summary reading `0 corroborated, 0 disproved, 5 unchecked`. Exit 1 would mean a `consumers:` routing claim is DISPROVED by this branch's own diff; exit 2 means the diff could not be taken. **The five UNCHECKED are not passes:** this brief's `consumers:` block is unchanged since the merge-base, so *this* branch's diff is not evidence about it — `statusgen` says so per entry. The base is pinned to the fork point so the row is reproducible regardless of where `main`'s tip has since moved |

## Evidence

Verified 2026-08-14 by opus-4.8[1m]-verifier (non-implementer) against merged main (the flip landed via the drift-debt PR).

| # | Command | Exit | Key output |
|---|---------|------|-----------|
| 1 | plugindrift coverage of the 5 flipped files | 0 | 0 BEHIND — "5 canonical, 2 pinned, 2 unported, 0 unaccounted"; only dailies + intake-desk (out of scope) drift |
| 2 | SOURCES.yaml commit-pin count | 0 | 2 — only dailies, intake-desk pinned; none of the 5 flipped rows |
| 3 | plugindrift --fail-on-drift, count among the 5 | 0 | trailing count 0 (exit 1 expected — repo-wide strict flags the 2 out-of-scope rows) |
| 4 | sibling consumer conversion | 0 | upstream consumer PR MERGED 2026-08-08 (four upstream bodies removed outright — stronger than thin pointers) |
| 5 | PARITY names the bundle canonical, inverted phrasing absent | 0 | pass |
| 7 / 7a | `go test ./tools/plugindrift`; gofmt | 0 | ok; unformatted=0 inspected=4 |
| 8 / 8a / 8b | coverage mutation controls | — | all fail closed as designed; tree clean after each |

RISK-VALUE: DERIVED — the authority flip: `plugins/assay/SOURCES.yaml` (planned) declares the five method-text skills (author-brief, the-desk, worker-desk [the renamed batch-fanout], verify-desk, pr-review-desk) canonical (no source pin); coverage reads "5 canonical, 2 pinned, 2 unported, 0 unaccounted"; plugindrift rows 1 and 3 report 0 drift among them. Sibling consumer cutover DERIVED MERGED. Positive-control rows 1a/2a/5a/9 read degenerate ONLY because the brief is already merged into origin/main (the pre-flip baseline they compare against is gone); row 9's substantive claim (0 disproved) reproduces via statusgen's merged-brief recipe. Row 6 machine-BLOCKED (no git repo at the user config path here) per its own guard.

VERIFY: PASS — primary assertions all pass; no assertion disproved.

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go run ./tools/plugindrift ... \| grep -cE -e author-brief -e the-desk -e batch-fanout -e verify-desk -e pr-review-desk \|\| true` | 0 | count `0` — no drift row names any of the five flipped files. Coverage line: `9 bundled skills/*/SKILL.md — 2 pinned, 5 canonical, 2 unported, 0 unaccounted`. The unfiltered count is `2`, and the whole-bundle verdict is `PLUGINDRIFT: DRIFT (behind 2)` — both rows are `dailies` and `intake-desk`, which later, independent briefs added to the bundle as pinned `files:` rows after this brief's scope was set | 2026-08-13 | opus-4.8[1m] |
| 1a | positive control — throwaway worktree at `refs/remotes/origin/main`, plugindrift, filtered BEHIND count | 0 (grep) | **count `5`** — meets the `>= 4` expectation. Re-run against the current baseline: **all five** flip targets are BEHIND — whole-bundle verdict `DRIFT` across the origins. The control does its job: the same checker, same filter, reports BEHIND on the pre-flip baseline and `0` after the flip, so row 1's zero is the checker passing rather than the checker blinded | 2026-08-13 | opus-4.8[1m] |
| 2 | `grep -cE '^ +commit: [0-9a-f]{40}$' plugins/assay/SOURCES.yaml \|\| true` | 2 | count `2` — the only `source:` pins left in the manifest are the two out-of-scope rows (`dailies`, `intake-desk`); none of the five flipped rows carries one. The structural form carries the claim without a literal SHA so it does not die silently against production source | 2026-08-14 | opus-4.8[1m] |
| 2a | positive control — `git show refs/remotes/origin/main:plugins/assay/SOURCES.yaml \| grep -cE '^ +commit: [0-9a-f]{40}$'` | 7 (`> 2`) | count **`7`** on `main` vs `2` on the branch — the five flipped rows are pinned on the baseline and unpinned here, so row 2's `2` measures removal and not a manifest that never pinned them. Base ref spelled `refs/remotes/origin/main` verbatim — the bare form emits `warning: refname 'origin/main' is ambiguous` in a checkout carrying a stale local branch of that literal name | 2026-08-14 | opus-4.8[1m] |
| 3 | `go run ./tools/plugindrift --fail-on-drift ... \| grep -cE ... \|\| true` | count `0` | trailing count `0` — strict mode names no drift row among the five flipped files. Exit code is `1`, as the corrected expectation states, and both drift rows are the out-of-scope pinned pair (`dailies`, `intake-desk`). This corrects an earlier reading of `exit 0` / "strict mode passes for the first time" that was taken while those two rows were being dropped from the check; it was never a true repo-wide green, and the claim has been withdrawn | 2026-08-13 | opus-4.8[1m] |
| 4 | sibling consumer-conversion PR on the upstream repo | 0 | the consumer conversion already landed (merged 2026-08-08, "the upstream repo becomes a consumer of the bundle skills home") — the four bodies were REMOVED outright (stronger than thin pointers) and the upstream repo's CLAUDE.md carries the breadcrumb. This PR's body cites that PR + head SHA per the cross-repo pairing convention | 2026-08-11 | worker |
| 5 | `grep -qiE 'canonical home' plugins/assay/PARITY.md && ! grep -qF 'the upstream copy remains canonical' plugins/assay/PARITY.md; echo $?` | 0 | `canonical home` present (opening section names the bundle the canonical home); banned inverted phrasing absent (the superseded historical record reworded to past tense) | 2026-08-11 | worker |
| 5a | positive control — same assertion against `refs/remotes/origin/main:plugins/assay/PARITY.md` | 1 | exit `1` — baseline PARITY contains the inverted phrase (superseded record), so the assertion is red pre-flip. Row 5's assertion on the branch is exit `0`, so the control separates the two sides. Base ref made verbatim — the bare form is ambiguous | 2026-08-14 | opus-4.8[1m] |
| 6 | out-of-repo applied + committed — `git -C ~/.claude log --oneline -1 -- skills/author-brief/SKILL.md` | 0 | applied LAST per rule-7: one-line canonical-home pointer added to `~/.claude/skills/author-brief/SKILL.md` and committed in the `~/.claude` stopgap repo; newest commit on the file is this brief's pointer edit | 2026-08-11 | worker (the owner's machine) |
| 7 | `cd tools/plugindrift && GOFLAGS=-buildvcs=false go test ./...` | 0 | `ok .../tools/plugindrift` — suite green against the restructured `canonical:` schema (validate/coverage cases added) | 2026-08-13 | opus-4.8[1m] |
| 7a | `cd tools/plugindrift && gofmt-rc/unformatted/inspected report` | gofmt-rc=0 unformatted=0 inspected=4 | `gofmt-rc=0 unformatted=0 inspected=4` — no unformatted file, gofmt itself exited 0, and it actually looked at 4 Go files. The instrument can now report its own failure (an earlier `gofmt -l . \| wc -l` form could not) | 2026-08-14 | opus-4.8[1m] |
| 8 | coverage positive control — throwaway undeclared skill dir, built binary run, then removed | 2 then 0 | `control-exit=2` with the checker printing `coverage: bundled but unaccounted for` naming the throwaway path; after removal `restored-exit=0`. The surviving check reddens on demand — it is a check, not a decoration. Tree confirmed clean after restore | 2026-08-13 | opus-4.8[1m] |
| 8a | negative control — `BENT-CONTROL-LINE` appended to a canonical body, strict mode run, file restored | 1 | `bent-exit=1`, `grep the-desk count=0`, `restored=0`. Baseline `unbent-strict-exit=1` as well — identical exit, no `the-desk` row either way. **The checker did not notice the bend.** This is the measured cap of the flip: the five `canonical:` rows have no source, so no comparison runs for them and only the coverage rule (row 8) still bites | 2026-08-13 | opus-4.8[1m] |
| 8b | coverage fails closed — bundle skills dir moved aside, then the manifest moved aside, built binary run against each | 2 then 2 | `noglob-exit=2`, `nomanifest-exit=2`. Zero-file glob prints `coverage: no files match ... — nothing to check, which is never a pass`; unreadable manifest prints `cannot read plugins/assay/SOURCES.yaml`. Neither degenerates to `0 unaccounted`. `go run` collapses could-not-check into fail, so a `make` target shelling `go run` cannot read this three-state result | 2026-08-14 | opus-4.8[1m] |
| 9 | `statusgen --root . --consumers --brief harness-portability/02 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | exit=0, 0 disproved | `exit=0`, `summary: 0 corroborated, 0 disproved, 5 unchecked`. No routing claim in this brief's frontmatter is contradicted by the branch's own diff. All five read UNCHECKED with the tool's own reason — unchanged since the merge-base, so this branch's diff is not evidence about it. UNCHECKED is not a pass and is recorded as such | 2026-08-14 | opus-4.8[1m] |

## Review

Gate: **model** (from frontmatter). The reviewer's focus: the port-judgement record —
every unported upstream hunk has a PARITY intentional-changes row; spot-check two of the
largest deltas for the in-section regression class (a guard flattened, a precondition
dropped) PARITY documents. The flip itself is enactment of a ratified ruling; what needs
judgement is the porting.
