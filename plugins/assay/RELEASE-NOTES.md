# Assay plugin — release notes

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
  (PARITY §4), and the "Home, as of v0.1.0" paragraphs name oit because they are
  precisely a statement about where the canonical copy lives until cutover (PARITY §2).

### Notes

- `version` in `.claude-plugin/plugin.json` is `0.1.0`; the matching git tag is a
  human-gated act and is not created by this change.
- Namespacing is the structural fix for the shadowing problem (issue #221): a personal
  `~/.claude` skill of the same bare name can no longer shadow the plugin one.
