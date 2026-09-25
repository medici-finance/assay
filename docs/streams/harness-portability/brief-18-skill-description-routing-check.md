---
brief: assay:assay:harness-portability:18
title: Skill description routing check — TF-IDF collision NOTICE plus a rank-1 routing fixture with a ratchet
why: >-
  A harness picks which skill to load by matching the user's words against each skill's
  description. When two descriptions share vocabulary (desk, loop, queue, review, verify), the
  wrong skill loads, and nothing in CI notices. The bundle already works around this by hand:
  `the-desk`'s description has to say "Do NOT load this for a WORKER…". Brief 17 enforces
  length limits and cannot see this failure. This brief adds a deterministic check that scores
  descriptions against each other and against a fixture of realistic prompts. It records
  today's routing accuracy as a baseline, and after one release a drop below that baseline
  fails CI.
wave: 1
depends: ["harness-portability/17"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-09-23 by intake-desk authoring dispatch
sources:
  - freshness-checked 2026-09-23 @ 5cb8039a7 (origin/main) — `tools/skillslint` has no description-similarity or routing check (main.go flags are `--root` and `--sync` only); no routing fixture exists
  - "prior art (MIT), read 2026-09-23 at https://github.com/addyosmani/agent-skills/tree/bcab6a1b8503100e8618c3b4e32cc78de43de769 — evals/README.md §The three tiers (tier 2: positive prompts rank their skill, negatives do not, descriptions must not near-collide) and scripts/run-evals.js (TF-IDF with idf = ln(1 + n/(1+df)), skill name tokens weighted 2x, COLLISION_WARN = 0.5, COLLISION_ERROR = 0.75, `--min-rank1` floor run in CI below a checked-in baseline)"
  - "baseline measured 2026-09-23 @ 5cb8039a7 with a throwaway script (not committed) using the method in Task 1 over the 14 `plugins/assay/skills/*/SKILL.md` descriptions: 91 pairs, top five — pr-review-desk~worker-desk 0.324, adopt~install 0.300, pr-review-desk~verify-desk 0.254, install~upgrade-assay 0.251, the-desk~verify-desk 0.250; no pair >= 0.50"
  - "indicative routing probe, same date and method: 20 hand-written paraphrase prompts, 19/20 rank-1; the miss was a pr-shepherd prompt (\"this PR was abandoned, finish addressing its review comments\") ranking pr-review-desk first"
  - plugins/assay/skills/the-desk/SKILL.md frontmatter @ 5cb8039a7 — description carries negative routing text naming worker-desk, pr-review-desk and verify-desk
consumers:
  - "tools/skillslint (routing.go, routing_test.go, main.go, README.md, routing/**, testdata/routing/**): follow-up harness-portability/18 (this brief; flips to fixed-here when the implementation lands the check)"
  - "plugins/assay/skills/*/SKILL.md descriptions: out-of-scope (read only; rewording any description is a follow-up once the check's numbers exist, not part of this brief)"
  - ".github/workflows/ci.yml: out-of-scope (the existing skillslint job runs the tool with `--root`; the routing check runs by default and needs no workflow edit)"
exec-tier: strong
exec-tier-why: >-
  (a): the fixture prompts must be paraphrases a user would actually type, not the description's
  own trigger phrases. An echo fixture scores 100% and measures nothing, and no test can tell
  the two apart.
---

# Brief 18 — Skill description routing check

## Context
files: `tools/skillslint/routing.go` (planned), `tools/skillslint/routing_test.go` (planned),
`tools/skillslint/routing/<skill>.yaml` (planned), `tools/skillslint/routing/baseline.yaml` (planned) — the bundle fixture,
`tools/skillslint/testdata/routing/**` (planned), `tools/skillslint/main.go`,
`tools/skillslint/README.md`, `changelog/harness-portability-18-skill-description-routing-check.md` (planned)
facts:
- sibling: `harness-portability/17` adds per-skill length limits and a repeatable `--skills-dir <dir>`
  that runs skillslint's structural checks over `<dir>/*/SKILL.md`. This brief reuses that flag, so
  it depends on 17. Without it, this brief would have to add a second flag for the same concept.
- skillslint conventions (main.go @ 5cb8039a7): exit 0 clean, 1 violation, 2 could-not-check;
  advisory output is a `skillslint: NOTICE:` line on stderr and never moves the exit code.
  Descriptions are read with the strict yaml.v3 frontmatter load `LintSkills` already does.
- baseline @ 5cb8039a7 (pre-17): top pair 0.324, none >= 0.50. 17 shortens the `install` and
  `pr-review-desk` descriptions, so the recorded baseline is re-measured after 17 lands, at
  implementation time. The numbers in `sources:` are the pre-17 reference.
- known limit: this is a lexical approximation. Negation is invisible to it, so `the-desk`'s
  "Do NOT load this for … worker-desk, pr-review-desk, verify-desk" RAISES its similarity to those
  three skills. Semantic routing is the behavioural tier, which is out of scope.
- supply chain: the prior art is re-implemented in Go from its documented method. No file, prompt or
  skill text is copied or vendored from it. Third-party skill content is instruction supply, and
  the repo's trust gate already quarantines untrusted inbound content
  (`plugins/assay/skills/the-desk/SKILL.md`, trust gate paragraph). The README credits the project
  by URL and does nothing more.
- out of scope: model-based (behavioural) evals; changing any description text.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Scoring, fixed so the baseline reproduces.** Tokenize: lowercase, split on `[^a-z0-9]+`, drop
   tokens under 3 chars and this stoplist (a Go constant, listed in the README): `a an and any are as
   at be before by for from in into is it its my need needs of on or our so that the them this to use
   want we when with you your help me i not do does`. No stemming. idf = ln(1 + N/(1+df)) over the
   skill set being checked; cosine over tf*idf vectors.
2. **Collision NOTICE.** Score every description pair (description text only). A pair >= 0.50
   prints one NOTICE naming both skills and the score to 3 decimals. The threshold is the prior
   art's warn level. The check has no error level, so collision never moves the exit code.
3. **Routing fixture.** `routing/<skill>.yaml` per bundle skill: `positive:` 3–5 prompts,
   `negative:` 2 entries `{prompt, owner}` where `owner` is the skill that should win. The routing
   document per skill is its name tokens twice plus its description. A positive passes when its
   skill ranks 1st. A negative passes when `owner` outranks this skill. Report per-skill and
   overall rank-1 accuracy as `rank-1 overall: <hit>/<total>`.
4. **Ratchet.** The fixture's baseline.yaml holds `rank1: <hit>/<total>` (the measured value at landing)
   and `mode: notice`. Measured below the baseline gives a NOTICE in `notice` mode and an Issue
   (exit 1) in `enforce` mode. `--routing-enforce` forces enforce mode for one run. A bundle skill
   with no fixture file is handled the same way. **Flip condition:** after the first umbrella release
   tag cut once the implementation's own changelog fragment,
   `changelog/harness-portability-18-skill-description-routing-check.md` (planned) — Task 7's
   fragment, never this brief's own `(spec)` fragment — is on `main`, a follow-up PR changes
   `mode:` to `enforce` and cites that tag. Never lower `rank1` to make a regression pass.
5. **Flags.** Under `--root` the check runs by default over the bundle and `routing/`. With 17's
   `--skills-dir`, the collision check runs over that directory. Routing runs only when
   `--routing-fixture <dir>` is also given (a directory holding `<skill>.yaml` + `baseline.yaml`).
   `--routing-report` prints the top-5 pairs and the per-skill table to stdout.
6. Test fixtures under `testdata/routing/`: `collide/` (two skills `alpha-collide`, `beta-collide`
   with near-identical descriptions) and a 3-skill `mini/` with fixture + baseline (this fixture also
   omits one skill's routing file, to exercise the missing-fixture-file path from Task 4). Add a Go
   test named `TestRoutingSwapDropsBelowBaseline` that swaps two real bundle descriptions in memory
   and asserts rank-1 drops below the baseline: a NOTICE in notice mode, an Issue in enforce mode.
7. README section (method, stoplist, thresholds, flip condition, prior-art credit by URL) + changelog
   fragment with one `### Added` bullet.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/skillslint && go build -o /tmp/skillslint-hp18 . && /tmp/skillslint-hp18 --skills-dir testdata/routing/collide` | exit 0; stderr contains `NOTICE`, `alpha-collide` and `beta-collide` |
| 2 | `cd tools/skillslint && go build -o /tmp/skillslint-hp18 . && /tmp/skillslint-hp18 --root ../.. --routing-report > /tmp/hp18-a.txt && /tmp/skillslint-hp18 --root ../.. --routing-report > /tmp/hp18-b.txt && cmp /tmp/hp18-a.txt /tmp/hp18-b.txt && grep -F "rank-1 overall: $(sed -n 's/^rank1: //p' routing/baseline.yaml)" /tmp/hp18-a.txt` | exit 0; the two stdout captures are byte-identical; the `rank-1 overall:` line printed to stdout equals `rank1:` in the committed `routing/baseline.yaml` (planned) |
| 3 | `rm -rf /tmp/hp18-swap && cp -R plugins/assay/skills /tmp/hp18-swap && mv /tmp/hp18-swap/worker-desk/SKILL.md /tmp/hp18-swap/w.md && mv /tmp/hp18-swap/verify-desk/SKILL.md /tmp/hp18-swap/worker-desk/SKILL.md && mv /tmp/hp18-swap/w.md /tmp/hp18-swap/verify-desk/SKILL.md && perl -pi -e 's/^name: verify-desk$/name: worker-desk/' /tmp/hp18-swap/worker-desk/SKILL.md && perl -pi -e 's/^name: worker-desk$/name: verify-desk/' /tmp/hp18-swap/verify-desk/SKILL.md && (cd tools/skillslint && go build -o /tmp/skillslint-hp18 .) && /tmp/skillslint-hp18 --skills-dir /tmp/hp18-swap --routing-fixture tools/skillslint/routing` | exit 0 (notice mode); stderr contains `NOTICE` and `below baseline` |
| 4 | after row 3: `/tmp/skillslint-hp18 --skills-dir /tmp/hp18-swap --routing-fixture tools/skillslint/routing --routing-enforce` | exit 1 (mutation: swapped descriptions redden the ratchet in enforce mode) |
| 5 | `cd tools/skillslint && go test ./... -count=1` | exit 0 |
| 6 | `cd tools/skillslint && go test -run '^TestRoutingSwapDropsBelowBaseline$' -v -count=1 . > /tmp/hp18-t.txt && grep -F -- '--- PASS: TestRoutingSwapDropsBelowBaseline' /tmp/hp18-t.txt` | exit 0; the named test ran and passed (chained with `&&`, not piped, and the PASS line is grepped: `-run` on a missing name exits 0 with "no tests to run") |
| 7 | `grep -c -i -e 'stoplist' -e 'threshold' -e 'flip condition' -e 'addyosmani/agent-skills' tools/skillslint/README.md; ls tools/skillslint/testdata/routing/mini/*.yaml \| wc -l` | first command prints >= 4 (README covers method, stoplist, thresholds, flip condition and prior-art credit by URL); second prints the `mini/` fixture count, one skill short of the full set (the missing-fixture-file path from Task 4 is exercised) |
| 8 | `curl -fsSL -o /tmp/hp18-run-evals.js https://raw.githubusercontent.com/addyosmani/agent-skills/bcab6a1b8503100e8618c3b4e32cc78de43de769/scripts/run-evals.js && grep -c -e 'COLLISION_WARN = 0.5' -e 'COLLISION_ERROR = 0.75' -e "indexOf('--min-rank1')" /tmp/hp18-run-evals.js` | exit 0; prints `3` (the cited prior-art thresholds resolve at the pinned SHA) |
| 9 | `find tools/skillslint -name '*.js'` | no output (nothing vendored from the prior art) |
| 10 | `statusgen --consumers --brief harness-portability/18 --root . --base "$(git merge-base origin/main HEAD)"` | exit 0; no routing claim disproved |
| 11 | `statusgen --lint --root .` (built from this repo's `statusgen/`) | exit 0; no PROBLEM line naming this brief |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Scoring is non-deterministic (map iteration order leaks into ties or output) | row 2 |
| The ratchet never fires, or fires as exit 1 before the flip | rows 3, 4 |
| Collision NOTICE never prints | row 1 |
| Fixture prompts echo the descriptions' own trigger phrases, so accuracy is inflated | review-only — the reviewer samples fixtures for verbatim reuse of description text |
| The Task 6 swap test (`TestRoutingSwapDropsBelowBaseline`) is left out entirely | row 6 (`-run` on a missing name still exits 0, so the row greps the `--- PASS:` line) |
| Task 7's README section is thin, or the missing-fixture-file path from Task 4 is never exercised | row 7 |
| Prior-art thresholds misquoted | row 8 |
| Baseline recorded from a pre-17 tree | row 2 (printed value must equal the recorded one on the branch head, which includes 17) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). The reviewer records verdict + date in
the stream README table and answers: do the fixture prompts read as user asks rather than as
copies of description text?
