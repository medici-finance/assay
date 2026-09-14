# Assay plugin — release notes

## v0.3.0 (prepared; tag creation is a human act)

**Second harness: the bundle now runs natively on Codex CLI (harness-portability).**
This is the version that records Assay's second first-class harness. The method text is
harness-neutral (one source, capability vocabulary in the skill bodies — HP/04); the
resident rules are generated per-harness (HP/05); and the Codex packaging
(`.codex-plugin/`, the `AGENTS.md` resident-rules fragment) is generated and byte-checked
in CI (HP/06). What ships in this bump:

- **Per-harness capability bindings** — `references/codex.md` and
  `references/claude-code.md` map each neutral capability to its harness mechanism and
  carry the **ruled degradation posture per skill** (Decision C,
  `docs/harness-portability-ruling.md`). Both are now freshness-registered (45-day leash)
  so they rot on a clock against vendor behaviour rather than by incident.
- **Codex adoption path** — `docs/adopting-assay.md` gains "Running Assay on Codex": the
  install (marketplace arm + the `.agents/skills/` file-placement arm), the `AGENTS.md`
  resident-rules fragment, the `[features] multi_agent` config step, the sandbox posture
  that decides which skills refuse, and the degradation expectations. The **per-skill
  degradation table is POINTED AT, not reproduced** — `references/codex.md` stays its one
  home (brief 07 task 2: copied, generated or referenced, never re-typed), and the runbook
  carries the rule the table applies: three guarantees that never degrade, convenience that
  degrades only by saying so. *(Written up as issue #763; the first draft of this bullet
  overstated the doc, which carried no Codex section at v0.3.0-prepared time.)*
- **Live smoke protocol** — `docs/codex-smoke-protocol.md`, the scripted live-harness
  acceptance checklist (seven minimum steps, each with an `Expect:` observable) whose run
  log is the artifact the human gate signs.

**What this bump does NOT claim.** CI cannot run Codex. Structural truth (the neutrality
lint, the resident-rules byte-compare, the Codex packaging coverage) is proven in CI;
**behavioural truth is proven only on a live Codex session**, which does not exist yet.
The first `docs/codex-smoke-runs/<date>-<codex-version>.md` run log — the stream's
acceptance evidence — is **BLOCKED pending a Codex-capable runner Ian provides or
sanctions**. Until it is signed, the honest state is "ready for the live run", not "done".

**Target scope: Codex CLI only** (ruling A1). The hosted Codex App is a later,
separately-ruled surface (HP/08).

> The version in `.claude-plugin/plugin.json` is `0.3.0`; the matching git tag is a
> human-gated act and is not created by this change. [`SOURCES.yaml`](./SOURCES.yaml)
> (`bundle-version: "0.3.0"`) is authoritative for provenance;
> [`PARITY.md`](./PARITY.md) carries the human second-harness record.

## v0.2.0 (prepared; tag creation is a human act)

**The bundle is the canonical home of the method text (harness-portability/02,
2026-08-11).** The five loop skills are no longer ports of an upstream: the port
relationship ended by authority flip, and `SOURCES.yaml` declares them
`canonical:` — with porting history in `notes:` and `PARITY.md`, and no source to
drift against. Consumers read the bundle: the tracker's desk skill bodies were removed at
the assay-selfcontain/08 consumer cutover (its CLAUDE.md names the home), and the
`~/.claude` stubs are thin pointers.

**What the flip does NOT buy — say it plainly.** Ending the port relationship
ends the *measurement* along with it. After the flip the five `canonical:` rows
carry no source, so `plugindrift` has nothing to compare them against and cannot
report `behind`, `moved` or `unreachable` for them — a bundle body can be edited
arbitrarily and no check reddens. What survives for those five is the coverage
rule only (a `skills/*/SKILL.md` present on disk but declared in none of
`files:`/`canonical:`/`unported:` is a hard error, exit 2). This repo's
`.claude/skills/` copies are **operating copies with no declared relationship to
the bundle and nothing checking one**: measured 2026-08-13 against
`refs/remotes/origin/main`, all seven bundled bodies differ from their
`.claude/skills` namesakes, and on the pre-flip pins all five flip targets read
BEHIND by 1/8/18/6/12 commits. Re-establishing a check across those two in-repo
trees — necessarily a substitution-aware comparison, not a byte-diff, because the
bundle copies are scrubbed ports — is follow-up work this release does not do.

**Drift-check result (2026-08-13):** for **the five flipped skills** `make plugin-drift`
reports **0 behind, 0 unreachable, 0 unaccounted** — they carry no origin, so
there is nothing left for them to drift against. That is the claim this release
makes, and it is scoped to those five.

It is **not** a repo-wide clean bill: `dailies` and `intake-desk` (added to the
bundle by later, independent briefs — assay-selfcontain/10 and education/04) are
NOT part of this flip, remain pinned `files:` rows, and both currently report
`behind` (1 and 14 commits as of 2026-08-13). So the bundle as a whole reports
`DRIFT (behind 2) across 2 origin(s)` and `--fail-on-drift` exits **1**. Ending
those two rows' port relationship is a later brief's scope — see `PARITY.md`.

**Final re-sync (2026-08-11):** `batch-fanout` and `pr-review-desk` were
re-ported from the current canonical heads (batch-fanout: deskreply tool-wiring
table + issue-shaped claim-key rule; pr-review-desk: sensitive-findings channel
wording); the other three were already in-sync. All five flipped files were
byte-identical to the canonical `.claude/skills` bodies **on 2026-08-11, at the
moment of the flip**. That is a historical statement, not a standing property:
by 2026-08-13 those `.claude/skills` bodies had moved on again (1/8/18/6/12
commits past the pre-flip pins) and nothing re-checks the pairing — see "What
the flip does NOT buy" above.

**Skill rename — `batch-fanout` → `worker-desk`** (methodology/46). The work-dispatch desk is now
named for its function, not its mechanism, landing the four-loop taxonomy
`intake-desk → worker-desk → pr-review-desk → verify-desk`. The renamed skill is one of the five
now declared `canonical:` above.

- Bundled path `skills/batch-fanout/SKILL.md` → `skills/worker-desk/SKILL.md`; the skill surfaces
  as **`assay:worker-desk`** (was `assay:batch-fanout`). **Breaking for anyone invoking the old
  namespaced name.** The v0.1.0 table below is left as written — it records what v0.1.0 shipped.
- Invocation triggers are unchanged ("fan out the next batch" etc. still load the skill).

> The version in `.claude-plugin/plugin.json` stays `0.1.0` pending the human
> release gate; the bundle version here and in `SOURCES.yaml` is `0.2.0`.
> [`PARITY.md`](./PARITY.md) is the human record; [`SOURCES.yaml`](./SOURCES.yaml)
> is authoritative for provenance.

## v0.1.1 (prepared; tag creation is a human act)

**dailies skill added to the bundle** (assay-selfcontain/10 — bundle membership: dailies = IN).

- `skills/dailies/SKILL.md` — a straight, byte-current re-port of the canonical body at
  `medici-finance/assay` `.claude/skills/dailies/SKILL.md`, pinned in
  [`SOURCES.yaml`](./SOURCES.yaml) at commit `02a0afdd` (blob `e7bf18d6`, as-of 2026-08-09). Ported
  **as-is** per the bundle's documented staged-regression convention — the house-specific content is
  **not** hand-scrubbed here; de-personalisation/parameterisation is the deferred
  **assay-selfcontain/09** pass, and dailies is its largest item (it hard-codes the report roster, the
  tracker output paths, real PR numbers, and the house portfolio names).
- Coverage now **6 pinned, 2 unported, 0 unaccounted** (was 5 pinned).
- **Drift check** (`make plugin-drift`, as-of 2026-08-09): the new `skills/dailies/SKILL.md` row
  reports `IN-SYNC` (bundle blob == recorded blob `e7bf18d6`). The bundle as a whole reports
  `DRIFT (behind 1, in-sync 5)` — the `behind 1` is the pre-existing `pr-review-desk` snapshot gap
  against `ff57ba34`, not dailies, and is out of scope for this change.

## v0.1.0 (prepared; tag creation is a human act)

First versioned cut of the Assay methodology as a Claude Code plugin. Install via
`/plugin marketplace add <path-or-repo>` then install `assay`; skills surface
**namespaced** as `assay:<name>`.

> **Snapshot as of 2026-07-17 — released 2026-08-02.** The five loop skills are ported from a source
> commit dated **2026-07-17**; this release ships about two and a half weeks later. The bundle is
> therefore **known to be behind its source on the day it shipped** — **136 commits** touched the five
> source paths in that window (ancestry-measured, `git rev-list --count <snapshot>..main -- <path>`; an
> earlier date-filtered count of 131 undercounted by five). [`PARITY.md`](./PARITY.md) carries the
> measured gap, names the specific rules that are known-behind (the stop-flag check, the hourly hygiene
> tick, the re-probe-primary-state rule, verify-desk's risk-bearing-value section, the four-shape
> fail-first rule, and several newer HARD GATEs) **and two superseded rule VALUES the bundle still
> ships — the per-stream draw cap (2 here, 4 at source) and span-of-control (7 here, 20 at source)** —
> and gives the re-sync procedure. **Read it before adopting.** Re-porting from current
> `main` is deliberately out of scope for v0.1.0 and is a separate piece of work.
>
> **The parity audit is section-granular.** `PARITY.md` accounts for every dropped `##` section; it has
> *not* line-diffed the bodies of retained sections, so a rule dropped inside a surviving section is
> not systematically covered. Two such drops were found and restored (`PARITY.md` change 12); more may
> exist.
>
> The date above is a snapshot of a moving target, so do not trust it to stay accurate — check it.
> [`SOURCES.yaml`](./SOURCES.yaml) is the **authoritative** record of what each bundled file was
> ported from (commit, blob sha, as-of date); the dates quoted here and in `PARITY.md` restate it, and
> **`SOURCES.yaml` wins if they ever disagree.** `make plugin-drift` re-fetches every source and
> reports how far behind each file has fallen since this release. It also asserts that every bundled
> `skills/*/SKILL.md` is accounted for — pinned to an upstream, or declared as authored here — so the
> "in-sync" it reports is about the whole bundle and not just the part somebody remembered to pin.

### What's in

**The five loop skills** (ported per [`PARITY.md`](./PARITY.md) — copy + adapt, no rewrite):

| Skill | Namespaced as | Role |
|-------|---------------|------|
| the-desk | `assay:the-desk` | Coordinator — arbitrates across streams |
| pr-review-desk | `assay:pr-review-desk` | Pre-merge review loop |
| verify-desk | `assay:verify-desk` | Post-merge verification |
| batch-fanout | `assay:batch-fanout` | Work dispatch — fan out Next-up to workers |
| author-brief | `assay:author-brief` | Brief authoring methodology (portable core) |

**The resident-rules SessionStart hook** — `hooks/hooks.json` +
`hooks/inject-resident-rules.sh`, injecting the 10 project-agnostic operating rules
(evidence-not-claims, isolation, neutral-dispatch wording, out-of-repo protocol,
no-attribution, model-tier awareness, redaction, push policy, shared-value discipline,
class-sweep) as a `systemMessage`. Rules only — the skill bodies carry the rationale.

Two further skills ship alongside from other streams: `assay:adopt` and
`assay:market-intelligence`.

### What's explicitly NOT in

- **No Go binaries.** statusgen, deskpost and the rest of `tools/` stay behind
  desk-tools C-1's `sudo make desk-install` gate and the pinned-release hash-check
  (assay-dogfood/03). Plugins do not ship binaries.
- **No project wrappers.** Repo-local thin wrappers (e.g. an in-repo
  `.claude/skills/author-brief` that delegates to the portable core) stay in their
  own repos. The plugin carries the portable core only; concrete tool paths, board
  commands, and repo lists are the wrapper's job.
- **No consumer cutover.** This release only makes the artifact exist and be
  installable. Repos switching to consume it is assay-dogfood/04.
- **No project-specific config.** No repo slugs, trust rosters, App/install IDs, credential paths,
  personal or persona names, or deploy specifics — those are compiled-in tool concerns, not plugin
  content. Where a rule needs a concrete shape to point at, the bundle uses a placeholder
  (`<reviewer-app>[bot]`, `$REVIEWER_TOKEN_PATH`, `<regenerate board>`). **Two named exceptions,
  disclosed rather than hidden:** the `go run .claude/skills/pr-review-desk/*.go` invocations are
  illustrative examples of tools the consuming repo supplies and do not work out of the box
  (PARITY §4), and the "Home, as of v0.1.0" paragraphs name the upstream source repo
  because they are precisely a statement about where the canonical copy lives until
  cutover (PARITY §2).

### Notes

- `version` in `.claude-plugin/plugin.json` is `0.1.0`; the matching git tag is a
  human-gated act and is not created by this change.
- Namespacing is the structural fix for the shadowing problem (issue #221): a personal
  `~/.claude` skill of the same bare name can no longer shadow the plugin one.
