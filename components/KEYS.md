# `components/KEYS.md` — the authoritative key catalogue

Keys are namespaced `assay.<area>.<name>` so two components cannot collide on a
local name (the composability model's §3, the paper's §6.6 key-collision problem
solved by namespacing). Adopter-defined keys use their own top-level namespace
and MUST NOT start with `assay.`.

This file is the authoritative list that
[`component-model.md`](../docs/streams/composability/component-model.md) §3 links
to. Every key a `component.yaml` puts under `provides:` should appear here with
its providing component and a one-line meaning; the `deskmanifest lint`
(`tools/desk/cmd/deskmanifest/`) resolves every `inject` key against the
`provides` set the manifests declare.

## Foundation and forge

| Key | Provided by | Meaning |
|---|---|---|
| `assay.forge` | `assay/forge-github` | a forge (GitHub) reachable with the roster's identities |
| `assay.streams` | `assay/streams-scaffold` | `docs/streams/` exists with a README and the stream layout |
| `assay.board` | `assay/statusgen` | `STATUS.md` is generated and linted |
| `assay.registers` | `assay/registers-scaffold` | FINDINGS / INTAKE / RETRO per-entry files exist |
| `assay.ci.statusgen` | `assay/ci-statusgen` | the board lint runs on every push |
| `assay.main-guard` | `assay/main-guard` | `core.hooksPath` + the pre-commit main-guard installed |
| `assay.labels` | `assay/labels` | the `review-request` / `raised-by:*` label set exists on the forge |
| `assay.reviewer-identity` | `assay/reviewer-app` | the App whose review is a verdict |

## Trust surface and extensions

The trust surface is ONE fail-closed key; each adopter extension is its own key,
so a broken extension deactivates only its dependents (§6.2), never the trust
gate. Additional `assay.roster.ext.<name>` keys follow the same pattern.

| Key | Provided by | Meaning |
|---|---|---|
| `assay.roster.trust` | `assay/roster` | the fail-closed trust surface (`ASSAY_BLESS_LOGIN`, `ASSAY_TRUSTED_LOGINS`, `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`, `ASSAY_HUMAN_LOGIN_MAP`) |
| `assay.roster.ext.risk-callout` | `assay/roster` | adopter risk-path callout (which paths force a security review) |
| `assay.roster.ext.writeguard-callout` | `assay/roster` | adopter write-guard callout |
| `assay.roster.ext.repo-aliases` | `assay/roster` | adopter repo-alias map |
| `assay.roster.ext.release-repo` | `assay/roster` | adopter release-home repo |
| `assay.roster.ext.scan-repos` | `assay/roster` | adopter intake-scan repo set |

## Desk verbs, roles, and hooks

| Key | Provided by | Meaning |
|---|---|---|
| `assay.desk.verbs` | `assay/desk-tools` | the pinned desk binaries on PATH |
| `assay.desk.role.the-desk` | `assay/the-desk` | the coordinator role procedure is installed |
| `assay.desk.role.intake-desk` | `assay/intake-desk` | the intake role procedure is installed |
| `assay.desk.role.pr-review-desk` | `assay/pr-review-desk` | the PR-review role procedure is installed |
| `assay.desk.role.verify-desk` | `assay/verify-desk` | the verify role procedure is installed |
| `assay.desk.role.worker-desk` | `assay/worker-desk` | the work-dispatch role procedure is installed |
| `assay.desk.role.pr-shepherd` | `assay/pr-shepherd` | the worker-side PR-shepherd role procedure is installed |
| `assay.hooks.session-start` | `assay/hooks` | resident rules + board state injected at boot |
| `assay.hooks.pre-tool` | `assay/hooks` | the write guard on tool calls |

## Methodology and utility skills

| Key | Provided by | Meaning |
|---|---|---|
| `assay.skill.adopt` | `assay/adopt` | the adoption-routing methodology skill is installed |
| `assay.skill.author-brief` | `assay/author-brief` | the brief-authoring methodology skill is installed |
| `assay.skill.install` | `assay/install` | the turnkey installer skill is installed |
| `assay.skill.upgrade-assay` | `assay/upgrade-assay` | the upgrade skill is installed |
| `assay.skill.ask-decision` | `assay/ask-decision` | the escalation skill is installed |
| `assay.skill.pdfingest` | `assay/pdfingest` | the PDF-ingest utility skill is installed |

## Exclusively-bound

| Key | Provided by | Meaning |
|---|---|---|
| `assay.harness` | `assay/harness-claude-code`, `assay/harness-codex`, `assay/harness-cursor` | the agent harness the skills and hooks are shaped for, provided with a `flavour` per adapter; exactly one ACTIVE provider at a time (§9). |

`assay.harness` is `exclusive: true` — `deskmanifest lint` (`tools/desk/cmd/deskmanifest/`)
refuses a tree with more than one ACTIVE provider (§9): "`assay.harness has 2 ACTIVE
providers: …`". Before this repo's own desired-state record exists (§7, still planned),
which adapter is ACTIVE is decided by each provider's `evidence` marker — the adopting
repo's own installed-shape file (`.claude-plugin/marketplace.json` for claude-code, root
`AGENTS.md` for codex, root `.cursor/rules` for cursor); see the three adapters'
`components/harness-*/component.yaml`.

## Additions beyond `component-model.md` §3

The inventory in `docs/adopting-assay.md` §2 needed keys §3's initial table did
not carry:

- `assay.forge` was in §3; `assay.roster.ext.*` were shown only as a template
  (`assay.roster.ext.<name>`). This catalogue expands the template into the five
  concrete extension keys the roster actually provides today
  (`risk-callout`, `writeguard-callout`, `repo-aliases`, `release-repo`,
  `scan-repos`).
- `assay.desk.role.<role>` was a template in §3; expanded here into the six
  concrete role keys (the five desk roles plus `pr-shepherd`).
- `assay.skill.<name>` is **new** — §3 had no key for the methodology / utility
  skills (`adopt`, `author-brief`, `install`, `upgrade-assay`, `ask-decision`,
  `pdfingest`), which are units in the §2 inventory but are neither desk roles nor
  forge/scaffold units. Each provides its own `assay.skill.<name>`.
