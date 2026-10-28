# Shared write policy and the writeguard adapter

**Status:** source implementation contract, 2026-10-20, approved for bounded
extraction in [#3752](https://github.com/medici-finance/assay/issues/3752#issuecomment-6299695731).
Installed hooks, publication and managed qualification remain separate gates.
The exact source/API/profile/build handoff is in [the census](write-policy-consumers.md).

## Outcome and ownership

One importable evaluator owns write-isolation decisions. The package is
`github.com/medici-finance/assay/deskcore/writepolicy`, inside the existing pure
`deskcore` module. It is separate from `deskcore/policy`'s approved role/forge grants:
local workspace isolation cannot grant forge authority. Publish compatible module
versions and qualify actual external consumers; a repository-relative replace is
source integration, not release proof. Do not create a second shared-core module.

The `S-write-policy` row in [contracts.md](contracts.md) records the core owner;
its marker baseline and checked consumers move with the implementation. `S-worktree` remains allocation, identity and lifecycle;
process leases, scratch preservation, push authorization and credentials keep their
existing owners. A workspace allocation grants no additional permission.
Existing authorized Git effects may need allocation-specific administration/object/ref
writes outside the checkout. Their adapter retains that scoped authority; generic
agent writes to protected store/config/control paths do not inherit it. Qualify an
owned staged commit as well as refusal of foreign refs/config; do not make ordinary
Git work impossible by declaring the entire common directory unwritable.

## Core and adapters

| Layer | Owns | Forbidden capability |
|---|---|---|
| `deskcore/writepolicy` | Typed operation/path checks, pure shell-command classification, exemption rules, sentinel protection and structured outcomes/reasons | Reading environment/config/files, resolving symlinks through I/O, Git, clocks, process/callout execution, network or credentials |
| `cmd/writeguard` | Existing stdin/JSON hook protocol; configuration and activation; trusted Git/path/claim observations; optional callout execution; mapping outcomes to harness responses | A second policy implementation or new authority source |
| Managed launcher/harness adapter | Map validated allocation, repository, generation, session and sandbox roots into evaluator inputs | Inferring an allocation from cwd, path prefix or an agent-supplied identifier; widening sandbox roots to satisfy a hook |
| Harness/container | Actual write-access confinement | Treating a library verdict as a filesystem sandbox |

`Classify(Request, LexicalContext)` freezes explicit lexical context, path-like
command tokens and literal masking in an immutable classification.
`Evaluate(Evaluation)` accepts that classification with its ID, protected roots,
explicit path/claim/context/callout observations, and the named profile. It returns
`Outcome` with decision, reason, schema revision, details and outstanding observation
requests. [policy.go](../deskcore/writepolicy/policy.go) is the exact Go surface;
[census and handoff](write-policy-consumers.md#implemented-api-and-profile-handoff)
describes the request/observation round trip and versions.
No `workspace`, desk-internal or command import enters the core. Adapters map a
validated workspace record to values; the values do not authenticate themselves.

Classification cannot observe the filesystem or execute a shell to resolve an
expression. The adapter resolves the requested paths (including nearest existing
ancestors for new files), supplies those observations, and reports failed or dynamic
resolution explicitly. Independent tests must cover this round trip, not just a
lexical path comparison. Revalidate relevant identity at the effect boundary;
a pre-tool observation is not protection against all later filesystem races.

## Behavior and compatibility

The evaluator distinguishes `allow`, `deny` and `unknown`, with stable reason codes
and applicable policy/evaluator revisions. Unknown is not a clean evaluation. The
adapter owns protocol mapping, so extraction preserves these existing distinctions:

- Legacy malformed hook input or Git-discovery failure yields no decision, as today.
- A configured callout can only add refusals; timeout, invalid answer or execution
  failure blocks. Preserve configuration-source restrictions and report the existing
  invalid-roster/unconfigured fallback rather than silently claiming it fail-closed.
- The shared-homed exemption requires the current claim and home/cwd conditions.
  Sentinel writes are checked before the exemption. Preserve expiry, scoping and
  path-alias handling; do not let an agent mint or renew its own exemption.
- Compiled shell checks remain heuristic; unknown interpreter/script effects are
  not newly claimed as covered. Preserve activation and missing-binary behavior.
- Managed admission requires independently validated allocation/session/repository
  identity and compatible versions. Missing or conflicting mandatory evidence holds
  that managed operation with a reason; it never falls back to legacy permission.
  This source profile is not a qualified or activated managed deployment.

A deny from the core cannot be cleared by a callout or adapter. Keep existing hook
JSON/exit/stdout behavior, including structured JSON denial on exit zero. Managed
decisions do not replace OS/harness confinement or operator activation gates.

## Consumers and migration

| Consumer | Disposition / owner | Retirement proof |
|---|---|---|
| Existing `cmd/writeguard` and command-hook wiring | desktools-v2/66 extracts policy and retains binary/protocol | Old command-local decision functions removed; common fixtures exercise import and built binary |
| Target local harness adapter | graph-execution/22 consumes /66 directly | Actual adapter/library decisions and independent sandbox refusal remain required now that the legacy CLI parser work (desktools-v2/31) is withdrawn |
| CLI parsing of writeguard and other guards | Unchanged: the desktools-v2/31 parser port is withdrawn and writeguard keeps its current parser | The hook adapter reaches the same evaluator; other guards retain their own meanings |
| Managed launch, tool hooks and host/container path mapping | managed workspace harness integration | Admitted allocation flows to hook and sandbox; missing/stale/foreign context refuses before launch/write; legacy hook integration still works |
| Other actual duplicate write-isolation evaluators | /66 discovers and records each site; migrate bounded sites there or route a named follow-up | Source-bound census with no unowned site or competing policy; calling an allocator alone does not require a writepolicy import |
| Statusgen checkout/audit lifecycle | shared workspace-library migration | Uses the workspace owner, not a blanket import of write policy; only a discovered duplicate evaluator adds a migration obligation |
| Push, credential, scratch, process and retirement guards | Out of scope: independent semantic/trust boundaries | Existing tests and enforcement remain; no blanket consolidation |

The implementation census includes current and planned consumers, source/query
revisions, exclusions, adapter owner and retirement condition. It lives at the
`docs/write-policy-consumers.md`; it is a checked inventory of sites, not
another semantic-owner registry. Public census/contracts contain neutral source
locators only. Coordinated consumers require exact producer API/profile versions;
installed hook/pin activation follows separate adopter review.

## Acceptance and quality

Preserve the existing source-bound regression cases, including false positives and
known coverage limits. Use independent expected outcomes, then compare the direct
library and real built hook on identical observations: shared write denied, owned
worktree allowed, symlink/ancestor escape, leading cd, literal/heredoc text, unresolved
shell target, shared exemption, sentinel write and configured-callout failure.
Exercise POSIX and supported Windows semantics; unsupported native observations
remain unknown. Record path/Git/callout invocation counts and elapsed time for the
same fixture workload before/after; no performance saving or new budget is invented.

Core tests run without filesystem/environment/clock access. Boundary checks inspect
transitive imports and effect calls with a planted I/O helper as a negative control;
behavioral fixtures also prove explicit observations determine the answer. A separate
external-module consumer builds with `GOWORK=off`; library/hook/version fixtures
must fail on a skipped deny, callout clearing a deny, sentinel exemption reordering,
missing managed binding or a duplicate local evaluator. Qualify actual release
contents and supported sandbox integrations before activating managed consumers.
