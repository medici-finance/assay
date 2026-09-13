---
brief: assay:assay:harness-portability:14
title: Code de-house — land the stream's tool and packaging deliverables in the public tree
why: >-
  This stream's planning record is public; its code and doc deliverables are not. Briefs
  01/02/06/07/12 name paths — `tools/harnessgen`, `tools/plugindrift`, the bundle's
  `SOURCES.yaml`/`PARITY.md`, the `.codex-plugin` manifest, the capability matrices, the
  smoke protocol — that do not exist in this repository, so their Verify tables cannot be
  run here at all. A non-implementer's fresh run at public main (2026-09-05) returned
  FAIL-as-written for 01 and could-not-check for 02/06/07; the PASSes on record were taken
  against a different tree, which is a different claim than the public briefs make. Until
  the deliverables live in the tree the briefs name, "Assay runs natively on Codex and
  Cursor" is an assertion this repository has no way to check.
wave: 6
depends: ["harness-portability/06", "harness-portability/12"]
unblocks: ["harness-portability/01", "harness-portability/02", "harness-portability/06", "harness-portability/07", "harness-portability/12"]
effort: L
gate: human
gate-why: >-
  The deliverable is a publication: 44 files authored in a private tree are landed in a
  public one. Publication is the one act in this stream that git does not undo — a
  withheld identifier that reaches a public commit is only removable by a history rewrite,
  and only if nobody fetched it first. The measured surface is not hypothetical: a local
  token sweep of the exact candidate set on 2026-09-06 exited non-zero, with 14 distinct
  withheld-token classes across 7 of the 44 files. The human is confirming (1) that the
  neutralisation rewrote the provenance narrative rather than merely deleting the sentences
  that carried it, so the files still say what they are for; (2) that all three leak layers
  ran and agreed, not just the one the author drove; and (3) that the residual house-tree
  copies are being retired deliberately in a follow-on, not orphaned.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-06 by harness-portability de-house authoring dispatch (assay-worker-app)
sources: ["the ruling recorded on the house tracker (2026-09-05, relayed 2026-09-06): split-deliverable briefs verify only against the tree the brief names as the deliverable home; 01/02/06/07/12 HELD at implemented, no public Evidence PRs, until the code de-house lands", "verify-desk's fresh non-implementer runs at public main 203dac5 (2026-09-05): hp/01 FAIL-as-written (deliverable absent), hp/02 and hp/06 could-not-check (tools absent), hp/07 could-not-check, hp/12 passing only against the private tree", "docs/streams/harness-portability/README.md re-home note (2026-08-26): 'The briefs code deliverables ... are a sequenced follow-on de-house (the same shape statusgen and desk-tools followed: source -> public, then the source tree consumes the released binary)'", "brief-12-cursor-third-column.md 'Tool de-house note' and brief-13's own re-home note, both naming this move as the owed follow-on", "the Verify tables of briefs 01, 02, 06, 07 and 12, read in full 2026-09-06 — the authoritative source for WHICH paths must exist here, used to correct the ruling's informal list", "measured 2026-09-06: local token sweep (legacy engine, house token map) over the 44-file candidate set exported from the private tree — exit 2, 14 distinct token classes, 7 files affected, 37 files already clean", "freshness-checked 2026-09-06 @ public 4a46934: none of the 44 candidate paths exists in this repository; plugins/assay/references/*.md, plugins/assay/skills/**, plugins/assay/hooks/inject-resident-rules.sh, docs/adopting-assay.md, tools/freshness and statusgen ARE already here and are therefore not in scope"]
design: DR-harness-code-dehouse
consumers: ["freshness.yaml: follow-up harness-portability/14 (five registrations added; the file itself is already public and is amended, not moved)", "docs/streams/harness-portability/README.md: follow-up harness-portability/14 (status row, wave 6, cross-repo table)", "docs/streams/harness-portability/brief-02-drift-debt-authority-flip.md rows 2/2a: follow-up harness-portability/14 (the rows measure a file this brief moves, and the count they assert is already stale against the source tree — see Task step 6)", "the private tree's copies of the same 44 files: out-of-scope (retiring them is a house-side change to a repository this brief does not touch; this brief is additive-only on the public side, so a red gate here leaves the private tree working)", "tools/skillslint PARITY check and the private tree's own CI: out-of-scope (they consume the private copies, which this brief leaves in place)", "the public Evidence runs for 01/02/06/07/12: follow-up (each is its own verify-desk item once this lands; naming them here is what makes the unblock traceable)"]
exec-tier: strong
exec-tier-why: >-
  (b) cross-component and cross-repository correctness — the acceptance test is that five
  other briefs' Verify tables, written against a different tree, become runnable here, so
  every moved path must satisfy commands this brief does not contain; and the
  neutralisation must be complete across all 7 affected files at once, since one missed
  instance fails the whole gate.
version: 1
id: f176c2e5-d180-420a-a96c-84a8befcda4b
---

# Brief 14 — Code de-house: land the stream's tool and packaging deliverables in the public tree

## Context

**single-point-of-failure:** the author's local token sweep (`tools/leaksweep` + the private
token map) is the only control that runs *before* bytes leave the machine — behind it stand two
more, failing for different reasons in different components: the control-based `leak-sweep`
commit status, produced by a workflow in a *different repository* under a *different token*
against a map this repository never sees; and this repository's own
`.github/workflows/leaksweep-pattern.yml`, a gitleaks pattern matcher that carries **no token
list at all** and catches secret-*shaped* content by pattern, on every commit, forever. Layer 1
misses a token the map does not list; layer 2 misses nothing the map lists but cannot run
locally; layer 3 misses named identifiers but catches shapes neither of the others models. See
"Defense in depth" below.

files:
- **add (31)** `tools/harnessgen/` (10 files), `tools/harnesslint/` (13 files),
  `tools/plugindrift/` (8 files) — three self-contained Go modules, each with its own `go.mod`.
- **add (10)** `plugins/assay/SOURCES.yaml` (planned), `plugins/assay/PARITY.md` (planned),
  `plugins/assay/RELEASE-NOTES.md` (planned), `plugins/assay/.codex-plugin/plugin.json`,
  `plugins/assay/codex/AGENTS-assay.md` (planned), `plugins/assay/codex/packaging.md` (planned),
  `plugins/assay/cursor/assay.mdc`, `plugins/assay/cursor/packaging.md` (planned),
  `plugins/assay/resident-rules.md` (planned), `plugins/assay/hooks/resident-rules.payload.txt`.
- **add (3)** `docs/research/codex-harness-capabilities.md` (planned),
  `docs/research/cursor-harness-capabilities.md` (planned), `docs/codex-smoke-protocol.md` (planned).
- **amend** `freshness.yaml` — five registrations (the two capability matrices and the three
  binding files), each `last-reviewed` + `max-age-days: 45` + `upstreams: []`.
- **amend** `docs/streams/harness-portability/README.md` — status row for 14, wave 6, the
  cross-repo table, and the "Tool de-house note" wording the briefs currently carry.
- **amend** `docs/streams/harness-portability/brief-02-drift-debt-authority-flip.md` — rows 2
  and 2a only, per Task step 6.

facts:
- `total-candidate-files: 44` (verified 2026-09-06 by exporting the exact path set from the
  private tree at its `origin/main` and counting; the count is reproducible from the file list
  above: 31 + 10 + 3).
- `already-clean: 37 of 44` — the local token sweep on 2026-09-06 reported `checked-clean` for
  every file of `tools/harnessgen`, `tools/harnesslint`, `plugins/assay/.codex-plugin`,
  `plugins/assay/codex/`, `plugins/assay/cursor/`, `plugins/assay/resident-rules.md` (planned),
  `plugins/assay/hooks/resident-rules.payload.txt` and both capability matrices.
- `needs-neutralising: 7 files` — `plugins/assay/PARITY.md` (planned), `plugins/assay/SOURCES.yaml` (planned),
  `plugins/assay/RELEASE-NOTES.md` (planned), `tools/plugindrift/marketplace.go` (planned),
  `tools/plugindrift/drift_test.go` (planned), `tools/plugindrift/main.go` (planned),
  `docs/codex-smoke-protocol.md` (planned). **Every instance is prose** — a provenance narrative, a
  comment citing a private issue, a flag help-string, and one internal document filename in a
  link. None is structural: no schema key, no identifier, no test fixture depends on a withheld
  value, so paraphrase suffices and no indirection layer is required (measured 2026-09-06; this
  is the fact that makes this brief L rather than a redesign).
- `neutralisation-vocabulary-already-exists`: `plugins/assay/PARITY.md` (planned) itself records the
  substitution ruleset a prior brief applied and made permanent (private issue reference to a
  bare `#N`, a private org/repo pair to `<owner>/assay`, a private repo root to
  `<tracker-repo-root>`, a sibling private repo to `../repo-b`, a personal handle to
  `human:<name>`). Reuse it; do not invent a second vocabulary.
- `SOURCES.yaml has no remaining source pins`: on the private tree today `files:` is the empty
  list and there are **zero** `source:` blocks and **zero** `commit:` lines — the authority flip
  and a later convergence brief moved every entry to `canonical:`. The withheld tokens in that
  file are therefore all in `notes:` history, not in live pins. This is also why brief 02's
  Verify rows 2 and 2a are already stale (Task step 6).
- `CI needs no edit for the new modules`: `.github/workflows/ci.yml` discovers each Go module by
  walking for `go.mod` and building + vetting from its own root, "so a new module is covered
  without editing this file" (read 2026-09-06). It runs `go build` + `go vet`, **not** `go test`
  — which is why Verify rows 4–6 below run the module suites explicitly.
- `the capability vocabulary lives in this README`: `tools/harnesslint` reads the closed
  capability set from the `<!-- assay:capability-vocabulary -->` block in
  `docs/streams/harness-portability/README.md`. That block is already public and already
  correct, so `harnesslint` finds its input here on arrival — no relocation of the vocabulary is
  in scope.
- `nothing is removed from the private tree by this brief`: the move is a **copy-in**. The
  private copies keep working, their CI keeps passing, and a red gate on this PR costs nothing
  but this PR.

**Not in scope, and why (the ruling's informal list, corrected against what is real):**

| Named in the ruling | Reality on 2026-09-06 | Disposition |
|---|---|---|
| `SOURCES.yaml`, `PARITY.md`, `.codex-plugin`, `cursor/`, `resident-rules` at repo root | None exists at a repo root; all five live under `plugins/assay/` | **Corrected path**, kept |
| "the smoke protocol + freshness registrations" | The protocol is one file (`docs/codex-smoke-protocol.md` (planned)); the registrations are five *entries* in a `freshness.yaml` that is already public | Protocol kept; registrations become an **amend**, not a move |
| `tools/harnessgen`, `tools/plugindrift` | Both real | Kept |
| `docs/research/codex-harness-capabilities.md` (planned) | Real | Kept |
| *(not named)* `tools/harnesslint` | Required by brief 12 row 6 — without it that row cannot run | **Added** to the list |
| *(not named)* `plugins/assay/RELEASE-NOTES.md` (planned) | Required by brief 07 rows 5 and 6 | **Added** |
| *(not named)* `docs/research/cursor-harness-capabilities.md` (planned) | Required by brief 12 row 10 | **Added** |
| *(not named)* `plugins/assay/codex/`, `plugins/assay/hooks/resident-rules.payload.txt` | The generated Codex packaging and resident payload the `codex`/`resident` generator verbs compare against | **Added** |
| `plugins/assay/references/*.md`, `plugins/assay/skills/**`, `plugins/assay/hooks/inject-resident-rules.sh`, `docs/adopting-assay.md`, `tools/freshness`, `statusgen` | **Already in this repository** | **Dropped** — nothing to move |
| `docs/research/jcode-desk-harness-capabilities.md` (planned) | Brief 09's deliverable, not one of the five held briefs | **Dropped** — out of scope |
| `docs/codex-smoke-runs/` | Does not exist in either tree; it is what brief 07 row 7 produces when the live environment exists | **Dropped** — nothing to move |
| Retiring the private tree's copies | A change to a repository this brief does not touch | **Dropped** — named follow-on, see `consumers:` |

## Read first

- `docs/streams/harness-portability/README.md` — the re-home note and the closed capability
  vocabulary block.
- The Verify tables of briefs 01, 02, 06, 07 and 12 in this directory. They are the
  specification: a path is in scope if and only if one of their commands names it.
- `.github/workflows/leaksweep-pattern.yml` and `.github/workflows/leaksweep-control.yml` — the
  two in-repo leak layers, and what each does and does not model.

## Ground rules

- NEVER git push to `main`, trigger workflows, or run mutating `kubectl`. Draft PR only.
- Stop at `implemented` — you do not set verified/done.
- **Never paste a sweep report anywhere.** The report names withheld tokens by construction; it
  stays on the machine that produced it. Report exit code and counts, never content.
- **A red leak gate is a STOP, not a puzzle to route around.** Do not weaken, allowlist, or
  narrow any leak control to get green — that is the `BLOCKED-ON-HUMAN` carve-out.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

**Stage A — the 37 already-clean files (one PR).** Copy `tools/harnessgen/`,
`tools/harnesslint/`, `plugins/assay/.codex-plugin/`, `plugins/assay/codex/`,
`plugins/assay/cursor/`, `plugins/assay/resident-rules.md` (planned),
`plugins/assay/hooks/resident-rules.payload.txt`, `docs/research/codex-harness-capabilities.md` (planned)
and `docs/research/cursor-harness-capabilities.md` (planned) into this tree at the same relative paths, and
add the five `freshness.yaml` registrations. Sweep, then push. This PR alone makes briefs 01 and
12 runnable and takes 06 most of the way.

**Stage B — the 7 files carrying withheld tokens (a second PR).** Copy
`plugins/assay/SOURCES.yaml` (planned), `plugins/assay/PARITY.md` (planned), `plugins/assay/RELEASE-NOTES.md` (planned),
`tools/plugindrift/`, and `docs/codex-smoke-protocol.md` (planned), **rewriting** every withheld instance
first, per steps 2–4. Staging B separately is deliberate: a leak-sweep red on the narrative files
must not hold the 37 clean ones.

Then, in order:

1. **Verify the source set before copying anything.** For each of the 44 paths, confirm it
   exists at the private tree's `origin/main` and does *not* already exist here. A path that is
   already here is a merge, not a copy, and is a NEEDS_CONTEXT stop — this brief asserts the
   target set is empty and that assertion was measured, not assumed.

2. **Rewrite the provenance narrative — do not delete it.** In `PARITY.md`, `SOURCES.yaml` and
   `RELEASE-NOTES.md`, apply the substitution ruleset `PARITY.md` itself records (see `facts:`).
   The porting history must survive the rewrite: after it, a reader must still be able to tell
   which skills were ported, which were flipped to canonical, when, and why. Deleting the
   sentences is the failure mode this step exists to forbid — it passes every leak gate and
   destroys the record the files exist to hold.

3. **Neutralise the three `tools/plugindrift` instances.** Two are comments citing a private
   issue by `<repo>#N` — reduce to a bare `#N` or drop the citation for a description of the
   ruling. One is the `--root` flag's help string naming a private repository — make it name the
   role (`the repository root to scan`), not an instance.

4. **Fix `docs/codex-smoke-protocol.md` (planned)'s one instance**: it links the adoption runbook under
   the private tree's filename. In this repository that document is `docs/adopting-assay.md`.
   Re-point the link and re-read the surrounding sentence — the step must still describe the
   install path the runbook actually prescribes here.

5. **Wire nothing that already works.** `ci.yml` discovers the three new `go.mod` roots on its
   own; `freshness.yaml` is the only registration surface. Do **not** add a workflow, a Makefile
   target, or a `.assay-surfaces` glob for these tools in this brief — an unmeasured CI addition
   is a separate change with its own review.

6. **Correct brief 02's rows 2 and 2a, and say so in the PR body.** Both rows count `commit:`
   lines in `plugins/assay/SOURCES.yaml` (planned) (expecting `2`, with a baseline control expecting `7`).
   The file this brief moves has **zero**, and zero `source:` blocks — the pins those rows
   measure were retired by a later convergence brief. Update the expectations to what the moved
   file actually asserts, and keep the rows discriminating: the assertion must still be a
   structural count that goes red if a `source:` pin reappears, and 2a must still read a baseline
   at which the count differs. Do not delete the rows and do not restate the pass narratively —
   a row that cannot fail is worse than a row that is wrong, because it reads green.

7. **Sweep before every push**, per Verify row 2, and let the public gate speak for itself
   (row 3). A `leak-sweep` red carries no detail on the PR by design: read the detail from the
   operator's private detail channel and fix exactly what it names. Do not guess from the diff —
   a guess that fixes the wrong site and re-plants the token elsewhere costs a full gate cycle
   each time, and the gate runs on its own cadence.

### Defense in depth — the three layers, and how each is proven

| Layer | Control | Fails on | Component |
|---|---|---|---|
| 1 | `tools/leaksweep` + the private token map, run locally on the staged tree | a **named** withheld identifier present in the map | the author's machine, pre-push |
| 2 | the control-based `leak-sweep` commit status | the same class, re-derived from a map this repo cannot read, on a schedule this session does not drive | a different repository, a different token |
| 3 | `leaksweep-pattern.yml` (gitleaks, **no token list**) + `leaksweep-control.yml` (the in-tree structural disclosure controls) | secret-**shaped** content and a withheld path re-entering a shipping file — neither modelled by 1 or 2 | this repository's CI, on every commit, permanently |

They are independent in the sense rule 10 requires: different signal, different component,
different failure time. Layer 1 is blind to anything the map omits; layer 3 is blind to a plain
English repository name; layer 2 is blind while offline. Verify rows 2a and 3a break each of the
two locally-runnable layers on purpose and prove it goes red.

### Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A withheld identifier survives in one of the 7 rewritten files | row 2 — local sweep exit 0 over the staged tree |
| The sweep is run against the wrong tree, or the matcher silently matches nothing, and reads clean | row 2a — a planted canary in a scratch copy must make the same invocation exit non-zero |
| Secret-shaped content nobody modelled rides along in a moved file | row 3a — gitleaks over a canary-planted scratch copy, using this repo's own config, no token map |
| The public control gate disagrees with the local sweep | row 3 — the `leak-sweep` status at PR head must be green, and it is not this session's to drive |
| Files land but the generated packaging no longer matches this tree's skills and references | rows 6, 6a — the three `--check` verbs, plus a bend-it mutation |
| A module lands that does not build or whose tests never ran here | rows 4, 5 |
| The neutralisation passes the gates by deleting the provenance narrative rather than rewriting it | **no row** — recorded review-only. Whether the porting history still says what it is for is an adequacy judgement; the reviewer answers it against Task step 2, and `gate-why` names it as the human's first confirmation. |
| The five downstream briefs still cannot run because a path nobody listed is missing | row 10 — every path their Verify commands name must resolve here |
| Brief 02's stale rows are carried over unchanged and its Evidence run fails as written | row 11 |

## Verify (executable — no prose-only DoD items)

Run from the repository root of a checkout of this branch unless a row says otherwise.
`$SWEEP` is the locally built `tools/leaksweep` binary and `$TOKENS` the private token map;
both live outside this repository and are named, not embedded, on purpose.

| # | Command | Expect |
|---|---------|--------|
| 1 | `n=0; for p in tools/harnessgen tools/harnesslint tools/plugindrift plugins/assay/SOURCES.yaml plugins/assay/PARITY.md plugins/assay/RELEASE-NOTES.md plugins/assay/.codex-plugin/plugin.json plugins/assay/codex plugins/assay/cursor plugins/assay/resident-rules.md plugins/assay/hooks/resident-rules.payload.txt docs/research/codex-harness-capabilities.md docs/research/cursor-harness-capabilities.md docs/codex-smoke-protocol.md; do test -e "$p" \|\| { echo "MISSING $p"; n=$((n+1)); }; done; echo "missing=$n"; test "$n" -eq 0; echo $?` | `missing=0` then `0` — every declared path arrived. The loop reports each miss by name, so a partial land is legible rather than a bare non-zero |
| 1a | `git ls-files tools/harnessgen tools/harnesslint tools/plugindrift plugins/assay/codex plugins/assay/cursor plugins/assay/.codex-plugin \| wc -l` | `36` — the six added directories contribute exactly 10 + 13 + 8 + 2 + 2 + 1 tracked files. A count, not a presence test: row 1 passes on a directory containing one file, this one does not |
| 2 | `rm -rf /tmp/hp14tree && mkdir -p /tmp/hp14tree && git archive HEAD \| tar -x -C /tmp/hp14tree && "$SWEEP" run --tree /tmp/hp14tree --tokens "$TOKENS" --engines legacy > /tmp/hp14sweep.out 2>&1; echo "sweep-exit=$?"; grep -c '^checked-failed' /tmp/hp14sweep.out \|\| true` | `sweep-exit=0` and a `checked-failed` count of `0`. The tree is exported from `HEAD` with `git archive` so no `.git` directory and no untracked scratch is swept. **Record the exit code and the count in Evidence; never the report body** — it names withheld tokens by construction |
| 2a | **Canary — the sweep can fail** (positive control for row 2): `rm -rf /tmp/hp14canary && cp -R /tmp/hp14tree /tmp/hp14canary && printf '%s\n' "$CANARY" >> /tmp/hp14canary/docs/codex-smoke-protocol.md && "$SWEEP" run --tree /tmp/hp14canary --tokens "$TOKENS" --engines legacy > /dev/null 2>&1; echo "canary-exit=$?"; rm -rf /tmp/hp14canary` | `canary-exit=2` — with one known token from the map planted, the *same* invocation goes red. Without this row, row 2's `0` is indistinguishable from a sweep that read nothing. `$CANARY` is any single entry of the map, supplied at run time and never written into this file |
| 3 | The `leak-sweep` commit status on this PR's head is `success` — `gh pr view --json statusCheckRollup \| jq -r '.statusCheckRollup[] \| select(.context=="leak-sweep" or .name=="leak-sweep") \| .state // .conclusion'` | `SUCCESS` — the control-based gate, produced elsewhere under a token this session does not hold, agrees with row 2. It posts on its own cadence; a pending status is a wait, and a red one is read from the operator's private detail channel, never guessed from the diff |
| 3a | **Pattern layer, independently** (positive control included): `gitleaks detect --source /tmp/hp14tree --config .gitleaks.toml --no-git --exit-code 9 > /dev/null 2>&1; echo "clean-exit=$?"; rm -rf /tmp/hp14gl && cp -R /tmp/hp14tree /tmp/hp14gl && printf 'AKIAIOSFODNN7EXAMPLE\n' >> /tmp/hp14gl/docs/codex-smoke-protocol.md && gitleaks detect --source /tmp/hp14gl --config .gitleaks.toml --no-git --exit-code 9 > /dev/null 2>&1; echo "planted-exit=$?"; rm -rf /tmp/hp14gl` | `clean-exit=0` then `planted-exit=9` — layer 3 passes on the real tree and fires on secret-shaped content, with **no token map involved**. The planted string is a vendor-published example key, not a withheld value, so this row is safe to read and safe to repeat |
| 4 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go build ./... && GOFLAGS=-buildvcs=false go vet ./... ) \|\| rc=1; done; echo "build-vet=$rc"` | `build-vet=0` — the three modules build and vet from their own roots, which is exactly what `ci.yml`'s module walk will do |
| 5 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go test ./... ) \|\| rc=1; done; echo "tests=$rc"` | `tests=0` — `ci.yml` runs build+vet only, so the suites must be proven here. `tools/plugindrift`'s suite reaches the network; a no-network run reports its own failure rather than a silent pass |
| 6 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) && (cd tools/harnessgen && GOWORK=off go run . cursor --check --root ../..) && (cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | `0` — all three generator verbs agree that the committed packaging matches **this** tree's `plugins/assay/skills/**` and `plugins/assay/references/*.md`. This is the row that proves the move is coherent rather than merely complete: the generators were written against one tree and are now checking another |
| 6a | **Mutation — a `--check` verb can fail**: `GOWORK=off go build -C tools/harnessgen -o /tmp/hp14gen . && printf '\nBENT\n' >> plugins/assay/cursor/assay.mdc && /tmp/hp14gen cursor --check > /tmp/hp14r6a.out 2>&1; echo "bent-exit=$?"; git checkout -- plugins/assay/cursor/assay.mdc; /tmp/hp14gen cursor --check > /dev/null 2>&1; echo "restored-exit=$?"` | `bent-exit=1` naming `assay.mdc`, then `restored-exit=0`. Built binary, not `go run`: `go run` flattens every non-zero status to `1`, and this tool's `2` (could-not-check) must stay distinguishable from its `1` (drift) |
| 7 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills && /tmp/hl870 bindings plugins/assay/references; echo $?` | `0` — the neutrality lint reads the closed capability vocabulary from this stream's own README block and finds this tree's skills and bindings conformant |
| 8 | `for k in docs/research/codex-harness-capabilities.md docs/research/cursor-harness-capabilities.md plugins/assay/references/codex.md plugins/assay/references/claude-code.md plugins/assay/references/cursor.md; do grep -qF "$k" freshness.yaml \|\| echo "NOT REGISTERED $k"; done > /tmp/hp14r8.out; test ! -s /tmp/hp14r8.out; echo "registered=$?"; rm -f /tmp/hp14fr.out; (cd tools/freshness && GOWORK=off go run . --root ../..) > /tmp/hp14fr.out 2>&1; grep -cE '^FRESH +(docs/research/(codex\|cursor)-harness-capabilities\.md\|plugins/assay/references/(codex\|claude-code\|cursor)\.md)' /tmp/hp14fr.out` | `registered=0` then `5` — all five registered, and each reports its **own** `FRESH` line. The tool's overall exit code is not load-bearing: an unrelated stale artifact elsewhere reddens the whole run regardless of these five. The `rm -f` guards against grepping a leftover output file from an earlier invocation |
| 8a | **Mutation — the leash can fail**: `(cd tools/freshness && GOWORK=off go run . --as-of 2027-06-01 --root ../..) > /tmp/hp14r8a.out 2>&1; grep -cE '^STALE +(docs/research/(codex\|cursor)-harness-capabilities\.md\|plugins/assay/references/(codex\|claude-code\|cursor)\.md)' /tmp/hp14r8a.out` | `5` — force-aged past the 45-day leash, all five of *these* lines flip to `STALE`, proving row 8's `FRESH` match is a real read and not a path-token count that is invariant under aging |
| 9 | `statusgen --lint --root .; echo $?` | `0` — PASS, with no PROBLEM. Build `statusgen` from this repo's own `statusgen/` directory rather than trusting a binary on `PATH`: a locally installed statusgen older than `plugins/assay/paired-versions.yaml`'s pinned tag is a stale oracle and its PROBLEMs cannot be trusted |
| 10 | **The acceptance row — the five held briefs become runnable here.** For each of briefs 01, 02, 06, 07 and 12, extract every repository-relative path literal appearing in its Verify table and resolve it against this tree: `n=0; for p in $(sed -n '/^## Verify/,/^## Evidence/p' docs/streams/harness-portability/brief-0{1,2,6,7}-*.md docs/streams/harness-portability/brief-12-*.md \| grep -oE '(tools\|plugins\|docs\|statusgen)/[A-Za-z0-9._/-]+' \| sed 's/[.,)]*$//' \| sort -u); do case "$p" in *codex-smoke-runs*) continue;; esac; test -e "$p" \|\| { echo "UNRESOLVED $p"; n=$((n+1)); }; done; echo "unresolved=$n"` | `unresolved=0`. `docs/codex-smoke-runs/` is excluded by name and by name only: it is brief 07 row 7's BLOCKED live-run output, which no de-house can produce. Any other unresolved path is a file this brief failed to list, and it is reported by name |
| 11 | **Brief 02's corrected rows still discriminate**: `sed -n '/^## Verify/,/^## Evidence/p' docs/streams/harness-portability/brief-02-drift-debt-authority-flip.md > /tmp/hp14r11.txt; grep -c 'commit: \[0-9a-f\]{40}' /tmp/hp14r11.txt; a=$(grep -cE '^ +commit: [0-9a-f]{40}$' plugins/assay/SOURCES.yaml \|\| true); echo "actual-pins=$a"` | the grep count is `>= 2` (rows 2 and 2a both still carry a structural count assertion — neither was deleted) and `actual-pins=0` matches what row 2 now expects. A row rewritten to assert whatever the file happens to contain is only honest while it can still go red: adding a `source:` block with a `commit:` line to a scratch copy must move the count |
| 12 | `git grep -n '<<<<<<<' -- . \| wc -l` and `git diff --stat origin/main...HEAD -- ':(exclude)docs/streams/harness-portability'` | `0` conflict markers; the diffstat touches **only** the paths this brief declares under `files:` — no incidental edit rode along in a 44-file copy |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Rows 2 and 2a: record the EXIT CODE and the checked-failed COUNT only.
     The sweep report body names withheld tokens and never appears here, in the
     PR, or in any comment. "verified" requires this section filled by someone
     who did NOT implement. -->

### Non-implementer verifier run — 2026-09-12 sonnet-5-verifier (verify-desk dispatch), FIRST verify pass — **VERIFY: PARTIAL**

Runner ≠ implementer. Own temp worktree off origin/main, `KUBECONFIG=/dev/null`. This brief's Evidence table was completely empty before this pass. Deliverable (the code-dehouse PR) is merged; human gate ratified per its own decision record. Tooling used: tools/leaksweep built fresh from a sibling methodology checkout; token map from that same checkout's docs/leak-sweep-tokens.yaml. Never quoted the map's contents.

| # | Command | Expected | Observed | Date / Runner |
|---|---|---|---|---|
| 1 | path-existence loop (14 paths) | missing=0, 0 | missing=0, 0 — PASS | 2026-09-12 sonnet-5-verifier |
| 1a | git ls-files count over 6 dirs | 36 | **38** — checked-failed. tools/harnesslint now has 15 tracked files, not 13: a later-merged PR (harness-portability/15) added 2 files to that module. Stale literal, not a defect in 14's own landing | 2026-09-12 sonnet-5-verifier |
| 2 | leaksweep over a git archive export | sweep-exit=0, checked-failed=0 | exit=0, checked-failed=0 (73 checked-clean, 0 could-not-check) — PASS | 2026-09-12 sonnet-5-verifier |
| 2a | canary (planted known token) | canary-exit=2 | exit=2 — PASS | 2026-09-12 sonnet-5-verifier |
| 3 | leak-sweep status at the merge-PR head | SUCCESS | SUCCESS — PASS | 2026-09-12 sonnet-5-verifier |
| 3a | gitleaks clean then planted a fake AWS-shaped key | clean-exit=0, planted-exit=9 | clean-exit=0, **planted-exit=0** — checked-failed. Root cause isolated: the pinned gitleaks version ships a built-in global allowlist regex that suppresses this exact vendor-example-shaped literal regardless of the repo's own config. Confirmed the repo's own config plus this exact pinned version correctly fires (exit 9) on a different, non-example-suffixed fake. This is a defect in the brief's chosen canary literal, not in the repo's leak-sweep wiring | 2026-09-12 sonnet-5-verifier |
| 4 | build+vet 3 modules | build-vet=0 | 0 — PASS | 2026-09-12 sonnet-5-verifier |
| 5 | test 3 modules | tests=0 | **tests=1** — checked-failed. harnessgen and plugindrift suites fail: a skill (human-runsheet) added by a later-merged PR (desk-skills/04, post-dates this brief) is not accounted for in the packaging coverage roster / SOURCES.yaml — real drift on current main, not a defect at landing time | 2026-09-12 sonnet-5-verifier |
| 6 | 3 generator --check verbs | 0 | **1** — checked-failed. codex --check and cursor --check both could-not-check (exit 2) on the same human-runsheet coverage gap; resident --check alone is clean | 2026-09-12 sonnet-5-verifier |
| 6a | mutation control | bent-exit=1, restored-exit=0 | bent-exit=2, restored-exit=2 — could-not-check. The coverage-gap precondition failure masks the mutation signal entirely; the intended drift signal can't be observed until the coverage gap above is fixed | 2026-09-12 sonnet-5-verifier |
| 7 | harnesslint bodies+bindings | 0 | **1** — checked-failed. bodies: clean. bindings: 3 violations — same human-runsheet skill missing its degradation-cell row in all three reference matrices | 2026-09-12 sonnet-5-verifier |
| 8 | freshness registration + FRESH count | registered=0, 5 | registered=0, 5 — PASS | 2026-09-12 sonnet-5-verifier |
| 8a | freshness force-age mutation | 5 | 5 — PASS | 2026-09-12 sonnet-5-verifier |
| 9 | statusgen --lint (built locally) | 0 | 0, LINT: PASS, only NOTICE-level lines (no PROBLEM) — PASS | 2026-09-12 sonnet-5-verifier |
| 10 | acceptance row — 5 held briefs' paths resolve | unresolved=0 | **unresolved=5** — checked-failed as literally run, but all 5 are false positives of the row's own extraction regex: 3 are truncated duplicates (the regex charset stops mid-match on an escaped dot inside two other briefs' own Verify-row commands — the correctly-spelled paths ARE present and separately captured); 2 are a scratch canary that another brief's own row creates-then-removes within one command line, never a real deliverable path. Manual check: every genuine path the five briefs' Verify tables name does resolve here | 2026-09-12 sonnet-5-verifier |
| 11 | a sibling brief's rows still discriminate | count>=2, actual-pins=0 | count=2, actual-pins=0 — PASS | 2026-09-12 sonnet-5-verifier |
| 12 | conflict markers + diffstat scope | 0 markers | conflict-marker search = 13 repo-wide, not brief-scoped — all 13 are legitimate: conflict-marker-detection test fixtures and prose in two briefs and a shepherd skill quoting the literal marker as documentation; none are real unresolved conflicts. Diffstat half is moot: this worktree's HEAD equals origin/main post-merge — that half of the row is designed for a live PR branch | 2026-09-12 sonnet-5-verifier |

`RISK-VALUE: DERIVED` — max-age-days = 45 @ freshness.yaml (5 new occurrences) — every max-age-days entry in this file (6 of 6, including the pre-existing entry) uses exactly 45; this brief reuses the house's one existing convention for untracked-upstream docs rather than inventing a value. Ranks last: a documentation staleness leash is a reversible operational knob, fully undoable by edit+redeploy — not the irreversible act this brief's gate concerns. The brief's actual irreversible surface (44 files crossing the private-to-public boundary) is not governed by any single literal; its correctness is instead what rows 2/2a/3/3a exist to prove (see row 3a finding above — that proof is currently incomplete for the pattern-layer control).

**Brief's own three Review questions, answered:**
1. Single control between fault and damage, and is it acceptable? At this head: the local-sweep layer ran and passed (rows 2/2a, both genuinely proven); the control-based status layer ran and passed (row 3); the pattern-layer scanner technically ran clean but its own positive control (row 3a) does not fire as specified — so that layer's ability to catch something is unproven by this table, even though it is wired and executing on every commit. Acceptable for the merge that already happened (the two layers that actually gated it both proved themselves), but the defense-in-depth claim is currently only two of three layers independently proven.
2. Does any row prove a lower layer catches the fault with the upper layer bypassed? Row 2a: yes, cleanly. Row 3a: no — it does not actually exercise the failure mode it claims to, for the reason above.
3. Did the neutralisation rewrite the provenance narrative or delete it? Rewrote it. Spot-checked the porting-history doc: the porting history, dates, and rationale for each skill's canonical/ported status all read intact and coherent under the neutral vocabulary, consistent with the substitution ruleset the file itself documents.

**VERIFY: PARTIAL.** Core deliverable landed, builds, and the security-critical sweep/control-status rows (2, 2a, 3) genuinely pass. Two distinct problem classes surfaced, neither a defect in this brief's own diff: (a) real drift since landing (rows 5, 6, 6a, 7) from a later-merged PR adding a skill without regenerating packaging/lint coverage — blocks the acceptance claim that the held briefs become runnable/green here until packaging is regenerated and reference-matrix cells updated; (b) Verify-table mechanics defects, not repo defects (rows 1a stale count, 3a wrong canary literal, 10 regex false positives, 12 unscoped grep) — worth a follow-up correction to the brief's own commands, but they don't indicate anything wrong with what was published.

Per frontmatter `gate: human`: this verifier does not sign off and status does not change. Evidence-only.

## Review

Gate: **human** (from frontmatter; `irreversible: yes`, `sensitive-data: yes`). Reviewer records
verdict + date in the stream README table.

Because this is a publication, the reviewer answers three questions in the verdict, in addition
to the usual ones:

1. **What is the single control between the fault and the damage, and is that acceptable?**
   Name which of the three layers actually ran at the reviewed head, and which did not.
2. **Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?**
   Rows 2a and 3a are the intended answers; a table that only walks the happy path through all
   three at once has verified exactly one.
3. **Did step 2 rewrite the provenance narrative or delete it?** Read `PARITY.md`,
   `SOURCES.yaml` and `RELEASE-NOTES.md` in the diff and confirm a reader can still tell which
   skills were ported, which were flipped to canonical, when, and why. This is the one obligation
   no command in the table checks, by design.

A `Security-Review:` verdict is recorded separately from the correctness review — this is a
public repository, and the publication question is its own review, not a paragraph inside
another one.
