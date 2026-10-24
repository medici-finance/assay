---
brief: assay:assay:desk-tools:54
title: Ship one normalised claim-sweep helper and make a claim-gone verdict paste its command
why: >-
  A documentation-heavy pull request needed ten review rounds to clear one false claim. The
  reviewer and the worker each checked "is this claim still anywhere?" with their own
  ad-hoc grep, each narrower than the other, and copies of the claim that had been
  wrapped or reflowed across two lines matched neither. Every round found another
  survivor. One committed, whitespace-normalised sweep that both sides run, plus a
  verdict rule that the exact command is pasted, turns "I checked" into a check anyone
  can rerun, and makes a false claim cost one round instead of ten.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-24 by statusgen newbrief
sources:
  - "driver ruling, 2026-10-23: commit one normalised claim-sweep helper that reviewer and worker both run, and amend the reviewer kit's clause 8 so a claim-gone verdict names the instrument and pastes the exact command"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md — clause 8 (body and Verify table re-checked against the current diff) and clause 9 (a claim-is-false finding is swept) name no sweep instrument; a verdict saying a claim is gone carries no rerunnable command"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §8 — the false-claim bullet says fix the whole claim class, but names no instrument for proving the class is gone; worker-prompt-objective.md carries no false-claim bullet at all"
  - "plugins/assay/scripts/ — bundle scripts ship with paired *.test.sh suites that the plugin-shell-suites CI job runs automatically; no claim-sweep script exists"
  - "freshness-checked 2026-10-24 @ 8e63e7fd0 (origin/main) — no file in the tree is named claim-sweep, and neither review-prompt.md nor either worker kit contains the string claim-sweep"
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: one instrument contract stated consistently in
  the helper, its test suite, two review-kit clauses and both worker kits, with a pin test
  holding the kit wording to the helper's real flags and exit codes.
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: follow-up desk-tools/54 (this brief; clauses 8 and 9 name the helper and require the pasted rerun line — flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: follow-up desk-tools/54 (this brief; the §8 false-claim bullet runs the same helper and pastes its rerun line in the fix reply — flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md: follow-up desk-tools/54 (this brief; Scope and reporting gains the same false-claim sweep sentence, byte-identical to the worker kit's — flips to fixed-here when the implementation edits the path)"
---

# Brief 54 — Ship one normalised claim-sweep helper and make a claim-gone verdict paste its command

## Context
files: plugins/assay/scripts/claim-sweep.sh (new, mode 100755), plugins/assay/scripts/claim-sweep.test.sh (new), tools/desk/cmd/deskdispatch/references/review-prompt.md, tools/desk/cmd/deskdispatch/references/worker-prompt.md, tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md, tools/desk/cmd/deskdispatch/claimsweep_test.go (new), changelog/desk-tools-54.md (new)
facts:
- reviewer kit: `tools/desk/cmd/deskdispatch/references/review-prompt.md`, embedded into deskdispatch (`//go:embed references/*.md` in kits.go). Clause 8 is the heading `## 8. Body and Verify table are re-checked against the CURRENT diff`; clause 9 is `## 9. A "claim is false" finding is swept, not just its cited line`. Both sit outside the lane markers, so every review lane renders them. The fleet receives kit text on the next desk-tools release.
- render helpers for a pin test: `kitText("review")` returns the raw kit; `reviewKitLanes()` lists the lanes; `reviewKitForLane(kit, lane)` returns the text a reviewer on that lane receives. `kitparity_test.go` has `clauseBody(kit, title)`, which returns the body under a heading up to the next heading or rule.
- worker kits: `worker-prompt.md` `## 8. Scope and reporting` carries the bullet beginning `**Fix a false-claim finding by its whole claim CLASS`; `worker-prompt-objective.md` `### Scope and reporting` has no false-claim bullet.
- bundle scripts: `plugins/assay/scripts/*.sh`, each with a paired `*.test.sh`. The CI job `plugin-shell-suites` runs every tracked `plugins/assay/scripts/*.test.sh`. Suite style (see `tick-summary.test.sh`): `ok()` prints two spaces, `ok`, three spaces and the case name; `no()` prints `  FAIL <name>`; the last line is `<N> passed, <M> failed`; exit 1 on any failure; bash 3.2 (macOS) and GNU both supported; no network.
- review-kit clause 5 (could-not-check is never an approval), common clause C4 (three-state instruments) and clause 10 (a committed tool makes no network or forge call by default) all bind the helper and the new kit text.
design-fit:
  owner: the review kit (tools/desk/cmd/deskdispatch/references/review-prompt.md clauses 8 and 9) owns what a claim-gone verdict must show; the helper is the instrument that clause names
  contract: none — the change is kit doctrine plus one bundle script; no semantic-owner row covers review-verdict evidence
  retires: [per-session ad-hoc greps as the evidence for a claim-gone verdict or a claim-class fix reply]
  weight: "+1 script, +1 shell suite, +1 Go pin test; flags +4 (--pattern, --tree, --pr-body, --case-sensitive); refusals 0; rule-text lines about +12 across three kit files"
  why-add: no committed instrument exists, and the failure is two parties using different instruments, so text in the clause alone cannot fix it. A desk-tools Go verb was considered and rejected as heavier than the job (a release cycle for a text sweep); one bash and awk script in the bundle reaches reviewer and worker the same day the bundle updates.

Pre-mortem — how this could pass review and still fail:

| Failure | Caught by |
|---|---|
| The helper normalises only spaces, so a claim split by a `//` or `>` continuation still escapes | Verify 1 (`comment-continuation`, `blockquote-continuation` cases) and Verify 2 (a real wrapped claim found at its first line) |
| A missing tree or empty pattern list prints nothing and exits 0, reading as "claim gone" | Verify 1 (`no-pattern-exits-2`, `not-a-git-tree-exits-2`, `missing-pr-body-exits-2`) |
| The `rerun:` line is not actually rerunnable (quoting lost) | Verify 1 (`rerun-line-reproduces`: the suite evals the printed line and compares output) |
| The kit names the helper in clause 9 but clause 8 still accepts a bare "claim is gone" | Verify 3 and Verify 6 (clause 8 body, every lane) |
| The two worker kits drift apart on the sentence | Verify 6 (byte-identical sentence in both kits) |
| The reviewer runs the PR head's copy of the helper, so a PR can edit the instrument that judges it | Verify 3 (clause 8 names the installed bundle's copy) |

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The helper is offline: no `gh`, `curl`, `git fetch` or token minting. A caller that wants a PR body checked fetches it first and passes the file.

## Task
1. **Write `plugins/assay/scripts/claim-sweep.sh` (planned)** (bash plus POSIX awk; must run under bash 3.2 on macOS and under GNU tools; mode 100755). Interface:
   - `-e PATTERN` / `--pattern PATTERN`, repeatable, at least one required. Each pattern is a fixed string, not a regex.
   - `--tree DIR`, default the git top level of the current directory. Scans every tracked file (`git -C DIR ls-files -z`), reading the working-tree contents so an uncommitted fix is seen. Binary files (NUL byte in the first 8 KiB) are skipped and counted.
   - `--pr-body FILE`, optional: also scans FILE; hits report as `pr-body:<line>`.
   - `--case-sensitive`; the default is case-insensitive.
   - Normalisation, applied to the scanned text and to each pattern the same way: per line, strip leading whitespace and then any leading comment or quote markers (`//`, `#`, `>`, `*`, `--`, `;`) and the whitespace after them; join the lines of a file with one space; collapse every run of whitespace to one space. Each match reports `<path>:<line>` of the ORIGINAL line where the match starts, and which pattern matched.
   - Output: one `hit: <pattern> <path>:<line>` line per match; then `scanned: <N> files, skipped: <M> binary`; `tree: <HEAD sha> (<clean or uncommitted changes>)`; and a final `rerun: <command>` line that, pasted into a shell from any directory, reruns the same sweep (absolute tree path, every pattern shell-quoted).
   - Exit codes: 0 = no pattern found anywhere; 1 = at least one hit; 2 = could-not-check (no pattern given, DIR is not a git work tree, the PR-body file cannot be read, or zero files were scanned). Exit 2 prints a reason to stderr and never prints a `rerun:` line that would read as a clean result.
2. **Write `plugins/assay/scripts/claim-sweep.test.sh` (planned)** in the bundle's suite style (temporary git repos under `${TMPDIR:-/tmp}`, `--case <name>` to run one case, final `<N> passed, <M> failed`). It must include at least these cases, by these names: `wrapped-markdown-line`, `comment-continuation`, `blockquote-continuation`, `tabs-and-runs`, `case-insensitive-default`, `case-sensitive-flag`, `absent-claim-exits-0`, `uncommitted-fix-seen`, `pr-body-hit`, `two-patterns-reported-separately`, `binary-file-skipped-and-counted`, `no-pattern-exits-2`, `not-a-git-tree-exits-2`, `missing-pr-body-exits-2`, `rerun-line-reproduces` (evals the printed `rerun:` line from another directory and compares its output and exit code with the first run), `offline-no-forge-calls` (runs with a PATH whose `gh`, `curl` and `git-remote-https` stubs fail loudly if called).
3. **Amend review-kit clause 8** (`review-prompt.md`): add one bullet stating that a verdict asserting a claim is gone from the tree (a false claim fixed, a stale phrase removed, a rename completed) names its instrument and makes it reproducible: it runs `claim-sweep.sh` from the INSTALLED assay bundle's `scripts/` directory (never the PR head's copy, so a PR cannot edit the instrument that judges it) and pastes the exact `rerun:` line and the `tree:` line into the verdict. A claim-gone assertion with no pasted command, or one resting on exit 2, is could-not-check under clause 5, never an approval. Pasting the result without the command does not meet the bar.
4. **Amend review-kit clause 9**: one sentence pointing the whole-diff and repository sweep at the same helper, with every phrasing of the claim the reviewer can name passed as a separate `-e`, and the patterns listed in the finding so the worker can reuse them.
5. **Amend both worker kits**: append to the `worker-prompt.md` §8 false-claim bullet, and add as a new bullet under `worker-prompt-objective.md` `### Scope and reporting`, the same sentence, byte-identical: before replying that a false-claim class is fixed, run the installed bundle's `claim-sweep.sh` with the reviewer's patterns (plus any phrasing you found) against your pushed tree, and paste its `rerun:` line in the finding reply; exit 1 means the class is not fixed yet, exit 2 means you could not check.
6. **Pin the kit text** with `tools/desk/cmd/deskdispatch/claimsweep_test.go` (planned), test `TestClaimSweepKitPin` (planned): for every lane in `reviewKitLanes()`, the rendered clause 8 body contains `claim-sweep.sh` and `rerun:` and the phrase `could-not-check`; the clause 9 body contains `claim-sweep.sh`; the worker sentence appears verbatim in both worker kits; and every flag the kit text names for the helper (`--pattern` or `-e`, `--tree`, `--pr-body`) is one the script's usage text names. Read the script from `../../../../plugins/assay/scripts/claim-sweep.sh` relative to the package directory.
7. **Changelog fragment** `changelog/desk-tools-54.md` (planned) describing the new helper and the clause 8 rule.
8. Flip this brief's three `consumers:` routes to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `o="${TMPDIR:-/tmp}/cs54-1.log"; bash plugins/assay/scripts/claim-sweep.test.sh > "$o" 2>&1 && grep -F -e '  ok   wrapped-markdown-line' "$o" && grep -F -e '  ok   comment-continuation' "$o" && grep -F -e '  ok   blockquote-continuation' "$o" && grep -F -e '  ok   rerun-line-reproduces' "$o" && grep -F -e '  ok   not-a-git-tree-exits-2' "$o" && grep -F -e '  ok   missing-pr-body-exits-2' "$o" && grep -F -e '  ok   offline-no-forge-calls' "$o" && grep -E -e '^[0-9]+ passed, 0 failed$' "$o"` | exit 0; the named cases pass and the suite ends with `0 failed`. Fails on main: the suite file does not exist. |
| 2 | check +dereference | `d=$(mktemp -d "${TMPDIR:-/tmp}/cs54-2.XXXXXX") && git -C "$d" init -q && printf 'Intro.\nThe gate is fully\n  automatic today.\n' > "$d/a.md" && git -C "$d" add a.md && git -C "$d" -c user.name=v -c user.email=v@example.invalid -c commit.gpgsign=false commit -qm a && o="$d.out"; bash plugins/assay/scripts/claim-sweep.sh --tree "$d" -e 'gate is fully automatic' > "$o" 2>&1; test $? -eq 1 && grep -F -e 'a.md:2' "$o" && grep -E -e '^rerun: ' "$o"` | exit 0; a claim wrapped over two lines is found, reported at its first line (`a.md:2`), exit 1, and a `rerun:` line is printed. Fails on main: the script does not exist. |
| 3 | check | `awk '/^## 8\./{on=1;next} /^## 9\./{on=0} on && /claim-sweep\.sh/{a=1} on && /rerun:/{b=1} on && /installed/{c=1} END{exit !(a && b && c)}' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo clause8-ok` | exit 0; prints `clause8-ok`. Fails on main: clause 8 never mentions `claim-sweep.sh`. |
| 4 | check | `awk '/^## 9\./{on=1;next} /^## 10\./{on=0} on && /claim-sweep\.sh/{n++} END{exit !(n>=1)}' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo clause9-ok` | exit 0; prints `clause9-ok`. Fails on main. |
| 5 | check | `awk '/^## 8\./{on=1;next} /^## 9\./{on=0} on && /claim-sweep\.sh/{n++} END{exit !(n>=1)}' tools/desk/cmd/deskdispatch/references/worker-prompt.md && awk '/^### Scope and reporting/{on=1;next} /^#/{on=0} on && /claim-sweep\.sh/{n++} END{exit !(n>=1)}' tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md && echo worker-ok` | exit 0; prints `worker-ok`. Fails on main: neither worker kit names the helper. |
| 6 | check +flow | `cd tools/desk && go test ./cmd/deskdispatch -run '^TestClaimSweepKitPin$' -count=1 -v > "${TMPDIR:-/tmp}/cs54-6.log" 2>&1 && grep -F -e '--- PASS: TestClaimSweepKitPin ' "${TMPDIR:-/tmp}/cs54-6.log"` | exit 0; the pin test ran and passed (a `-run` that matches nothing exits 0 without the `--- PASS` line, so the grep is the check). Fails on main: the test does not exist. |
| 7 | check | `o="${TMPDIR:-/tmp}/cs54-7.log"; bash plugins/assay/scripts/claim-sweep.sh --tree "$(mktemp -d "${TMPDIR:-/tmp}/cs54-7.XXXXXX")" -e anything > "$o" 2>&1; test $? -eq 2 && ! grep -E -e '^rerun: ' "$o" && echo could-not-check-ok` | exit 0; prints `could-not-check-ok`; a directory that is not a git tree exits 2 and prints no `rerun:` line. Fails on main: the script does not exist (exit 127). |
| 8 | check +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && ! grep -F -e ': follow-up desk-tools/54' docs/streams/desk-tools/brief-54-claim-sweep-helper.md && o="${TMPDIR:-/tmp}/cs54-8.log"; statusgen --root . --consumers --brief desk-tools/54 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; post-merge, with `DELIVERY_HEAD` set to the implementation PR's merge commit and `DELIVERY_BASE` to its first parent: every consumer route reads `fixed-here` and the delivery diff corroborates all three, exact summary `3 corroborated, 0 disproved, 0 unchecked`. Either variable unset fails the row (could-not-check, never a pass). Fails on this authoring branch: the routes are still `follow-up`. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
