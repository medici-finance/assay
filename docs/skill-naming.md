# Claude Code Skill Naming Convention

**Status:** proposed · **Applies to:** `.claude/skills/*` in the adopter's own repos and the `assay:` plugin bundle.
**One-line rule:** *kebab-case; the name's shape tells you the category; methodology skills carry no
project domain token, project skills do.*

## 0. Why this exists

Our skills grew one at a time, so the set *looks* inconsistent — verb-noun (`author-brief`,
`batch-fanout`), role/noun (`the-desk`, `verify-desk`), domain-tool (`payments-auth-doctor`, `deploy-ops`).
The finding from surveying the current project skills (twelve at time of writing): **they are more
consistent than they appear.** Three natural categories already exist; nobody had *named* them. This
doc names them, so a new skill slots in without debate. It also flags the two outliers — `batch-fanout`
and `issue-loop`, both desk roles named by mechanism — and renames them (§5) so there is no exception
to carry.

## 1. Casing and grammatical form

- **Always kebab-case**, lowercase, ASCII. No spaces, camelCase, or underscores.
- **No version numbers in names** — skills upgrade in place; version lives in the plugin manifest / SKILL body.
- **Form is chosen by category.** There are exactly three:

| Category | What it is | Form | Examples |
|---|---|---|---|
| **Role / desk** | A standing window an agent *becomes*. Two kinds: the three **brief-pipeline automation loops** that each own a lifecycle phase (`worker-desk`, `pr-review-desk`, `verify-desk`) and the single **coordinator** (`the-desk`) that orchestrates them — not itself a loop. (A fourth role skill, `issue-loop`, runs the inbound/issue queue — a separate loop that feeds briefs into the pipeline; it is also a rename candidate to `issue-desk` in §5.) | **`<role>-desk` suffix** (noun) | `worker-desk`, `pr-review-desk`, `verify-desk`; coordinator `the-desk` |
| **Action** | Do one thing and stop | **`verb-noun`** | `author-brief`, `ship-release`, `smoke-test` |
| **Domain / tool** | Project-specific diagnostics/ops over a named subsystem | **`<domain>-<thing>` prefix** | `payments-auth-doctor`, `payments-query`, `deploy-ops` |

Picking a category when authoring:
- The agent *runs as* it for a whole session (a persona/window) → **role**, suffix `-desk`.
- It names a project subsystem a user says out loud ("the payments auth thing", "deploy") → **domain**,
  prefix with that subsystem token (`payments-`, `deploy-`, `billing-`, `billing-`).
- Otherwise → **action**, lead with the verb.

## 2. Prefixes vs. flat — the decision

**Use category *affixes*, not a flat free-for-all — but let each category use the affix that reads
naturally, rather than forcing one universal prefix.**

- **Domain/tool skills → leading prefix** (`payments-*`, `deploy-*`). Highest-value affix: these skills are
  numerous, project-specific, and the prefix makes them **sort together** and signals "only makes sense
  in this repo." Reserved domain tokens: `payments-`, `deploy-`, `billing-`, `billing-`.
- **Role skills → trailing `-desk` suffix.** A leading `desk-` prefix was rejected: the loop roles ship
  as `assay:<name>`, so the plugin namespace already groups them (`assay:verify-desk`), and the noun
  reads better as a suffix (`verify-desk`, not `desk-verify`).
- **Action skills → flat verb-noun.** No affix; a verb-led name is already self-describing and these
  don't form a family worth grouping.

**Tradeoff, plainly:** a single universal prefix scheme buys perfect alphabetical grouping and zero
ambiguity, at the cost of uglier common-case names, redundancy with the `assay:` namespace, and a mass
rename of already-wired skills. The hybrid takes the grouping win exactly where skills are numerous and
repo-bound (domains) and leans on the namespace everywhere else — so **almost nothing is renamed** (§5).

## 3. Namespacing — `assay:<name>` vs `.claude/skills/<name>`

Two homes, and the **name itself signals which**:

| | Methodology / loop skills | Project-local skills |
|---|---|---|
| **Home** | `assay` plugin → surface as **`assay:<name>`** | repo **`.claude/skills/<name>`** |
| **Domain token?** | **None** — domain-neutral (`author-brief`, `verify-desk`, `the-desk`) | **Carries one** (`payments-`, `deploy-`, `billing-`, `ship-release`, `smoke-test`, `debug-agent`) |
| **Portable?** | Yes — any repo adopting Assay gets them | No — assumes this repo's own runtime stack |
| **Test** | "would another team running Assay want this?" → **assay** | "does it name *our* subsystem?" → **local** |

- The `assay:` prefix is **structural, not stylistic**: it prevents a personal `~/.claude` skill of the
  same bare name from shadowing the project one (see the plugin `skills/README.md`).
- **Membership rule:** a methodology skill must be domain-neutral. If you want a project domain token in the name of
  an `assay:` skill, it belongs in `.claude/skills/` instead.
- The canonical `assay:` skills (`author-brief`, `worker-desk`, `pr-review-desk`, `verify-desk`,
  `the-desk`) are the five core members; repo entries for them are thin pointers to the canonical
  bodies.

## 4. Description rules (restating CLAUDE.md, with teeth)

Descriptions **load into every session and every subagent** — a resident cost. Per CLAUDE.md:
*descriptions load every session: **triggers only, never workflow summaries.***

- **Include:** when to invoke — the plain-language asks, error strings, symptoms; optionally one
  "prefer this over X" line.
- **Exclude:** how the skill works, what it reads, its steps, persona lore. That belongs in the SKILL
  **body**, which loads only on invoke (the cheap place for detail).

**Good** (triggers only): *"Use WHENEVER someone wants to check whether the JSON API is up, read/verify
active records… Triggers: 'is the API up', '400 Missing required field', 413 on
/v2/records, CORS blocked from localhost:5173."*

**Bad** (workflow leaking in): *"…Reads STATUS.md Next-up (already priority + staleness + 2-per-stream-
capped), dispatches one worker per brief, and hands the resulting draft PRs to the pr-review-desk
window."* — that second sentence is a workflow summary; move it to the body.

**Audit finding — description-trim (non-breaking) is the real migration action:** `batch-fanout`,
`pr-review-desk`, `verify-desk`, and `the-desk` all carry step-by-step workflow + persona narration in
their descriptions. Trim them to triggers-only. The domain skills (`payments-*`, `deploy-ops`, `ship-release`)
are already close to the model.

## 5. Migration / rename map

Renaming a skill is a **breaking change** — it breaks slash invocations, cross-references, thin-pointer
delegations, and prose that names it. Bar: *rename only when the current name actively misleads.* By
that bar, **almost everything is kept** — the scheme mostly ratifies what exists.

| Current name | Category | Verdict | Reason |
|---|---|---|---|
| `author-brief` | action | **KEEP** | Textbook verb-noun; canonical `assay:` skill. |
| `batch-fanout` | role | **RENAME → `worker-desk`** | A `-desk` role named by mechanism, not function. It owns the **work/implementation phase** (dispatches workers to implement briefs), so `worker-desk` names its function like the review/verify loops name theirs. It is the first of the three **brief-pipeline automation loops** — `worker-desk` (implement) → `pr-review-desk` (review) → `verify-desk` (verify) — that the coordinator `the-desk` orchestrates. The "fan out the next batch" trigger lives in the *description*, so invocation is unaffected by the dir rename. |
| `payments-auth-doctor` | domain | **KEEP** | Perfect `payments-*` prefix; `-doctor` = the diagnose-and-fix idiom. |
| `payments-query` | domain | **KEEP** | Perfect `payments-*` prefix. |
| `debug-agent` | action/domain | **KEEP** | Verb-noun, reads fine. |
| `deploy-ops` | domain | **KEEP** | `deploy-*` prefix; `-ops` clear. |
| `issue-loop` | role | **RENAME → `issue-desk`** | A `-desk` role named by mechanism, not function — same pattern as `batch-fanout`. Runs the inbound/issue-queue (the front-door twin of `pr-review-desk`), feeding briefs into the pipeline. By convention belongs with the `-desk` suffix. |
| `pr-review-desk` | role | **KEEP** | Correct `-desk` suffix. |
| `ship-release` | action | **KEEP** | Correct verb-noun. |
| `smoke-test` | action | **KEEP** | Established test-type noun. |
| `the-desk` | role (coordinator) | **KEEP** | The **coordinator** that orchestrates the three brief-pipeline automation loops — NOT itself a loop. The definite article marks the singular orchestrator; `/the-desk` entry point. |
| `verify-desk` | role | **KEEP** | Correct `-desk` suffix. |

**Net: two renames** — `batch-fanout` → `worker-desk` and `issue-loop` → `issue-desk`, so the role
skills are consistently `<role>-desk`, the three **brief-pipeline automation loops** are consistently
`<phase>-desk` (`worker-desk` / `pr-review-desk` / `verify-desk`), and `the-desk` is the coordinator
that runs them — no exception to carry. `issue-desk` is the inbound loop (issues → briefs), feeding
the pipeline rather than part of it. Everything else is kept. The rename is a coordinated change (skill
dir + `name:` field + the handful of by-name refs); sequence it after in-flight PRs that target the
`batch-fanout` skill path land, to avoid churn. The other high-value, **non-breaking** migration is the
**description trim** in §4.

**New skills going forward:** standing role → `<role>-desk`; diagnostic/ops over a subsystem →
`<subsystem>-<thing>` reusing a reserved domain token; one-shot → `verb-noun`; methodology skill →
domain-neutral name, lands in the `assay` plugin, not `.claude/skills/`.

## 6. Where this convention lives

Canonical home: **this file** (`assay/docs/skill-naming.md`), linked from the plugin's
`plugins/assay/skills/README.md`. Each consuming repo keeps a single resident pointer line in its
`CLAUDE.md` (rule stays resident, rationale in the doc) — e.g.:

> *Skill names follow the naming convention (`../assay/docs/skill-naming.md`): kebab-case;
> `<role>-desk` for standing roles, `verb-noun` for actions, `<domain>-<thing>` for project subsystems;
> methodology skills are domain-neutral and ship in the `assay:` plugin, project skills carry a domain
> token. Descriptions are triggers only.*