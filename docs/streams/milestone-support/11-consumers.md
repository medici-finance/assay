# Milestone projection pickup

Source: `265798703c0f940d62649595719f1ccc7d1da292` (main at pickup); brief 11
version 1, with the delivered milestone loader (/03), readiness evaluator (/07)
and reference/claim adapters (/08). Everything is consumed in process: the
projection reads the loader's `MilestonePlan` and the evaluator's
`ReadinessResult`, and deskboard decodes through `statusgen/streamview`. No
dependency, executable, daemon, store or credential owner is added. Disposition:
continue the approved bounded integration. Emitting a stream view from the
`statusgen` command line, and advertising `milestone-v1` from the production
reader, remain separate follow-ups listed below.

The reader inventory was refreshed before changing the contract
(`git grep -n 'stream-view/v1\|statusgen/streamview\|streamViewSeed'`). These are
dispositions, not completion or independent verification evidence.

| Consumers | Disposition |
|---|---|
| `statusgen/streamview` (contract, bindings, decode, negotiate) | fixed here: `stream-view/v2` adds the mandatory `milestones` section; v1 decodes unchanged; unknown or missing version refused before the body is read; bindings dereference milestone sources and evidence |
| `statusgen/streammilestones.go` (new producer) | fixed here: pure projection of loader and readiness results; acceptance is never inferred from work, so production (which supplies no acceptance observation) projects could-not-check; offers v2 only for opted-in streams |
| `statusgen/streammission.go` (`streamViewSeed`) | unchanged by design: the v1 seed stays byte-identical for streams that did not opt in |
| `statusgen/readmetable.go` (Briefs region) | unchanged: the Briefs table keeps its ownership and markers; the milestone region is a separate file and marker pair |
| `statusgen/readmemilestones.go`, `regen.go`, `main.go` | fixed here: separately marked generated region, rewritten by `--regen`, drift and misplaced markers reported by `--lint`; no region for non-opted-in, fixture or outline streams |
| `tools/desk/cmd/deskboard/` | fixed here: `milestones` verb decodes the versioned projection in process, refuses unsupported contracts, shows source coverage and unknowns, quotes untrusted text, withholds inaccessible locators, collapses aliases |
| `tools/desk/askassay` read-verb allow-list, `tools/desk/README.md` | fixed here: `milestones` classified read-only; verb row and section added; inventory and parity tests updated |
| `docs/stream-view-contract.md`, `docs/streams/README.md`, `docs/contracts.md` | fixed here: v2, negotiation, the milestone section and the generated region documented |
| `tests/stream-brief/32.sh` | fixed here: its version-substitution mutation now targets a version no build decodes (v3), since v2 is supported |
| `statusgen` command line emission of a stream view (`--stream-view`) | follow-up: neither the v1 seed nor `streamViewVersioned` has a production emitter yet; deskboard reads documents a producer or a test supplies |
| production reader advertisement (`statusgenReaderCaps`) | follow-up /10 and /14: `milestone-v1` is still unadvertised, so a real opted-in stream projects an unknown plan with no records (could-not-check), never an invented milestone |
| acceptance and subject observation in production (`Stream.MilestoneReadiness`) | follow-up /13: the lint scan retains readiness, but no subject or acceptance record is supplied, so acceptance stays could-not-check |
| wire `aliases` | tolerant-ingest only: milestone records carry no alias spellings, so the producer emits none; deskboard folds any it receives |
| brief 13 trace and report | follow-up /13: consumes this projection; exact runtime readers enumerated at its pickup |
| external viewers of README milestone text | could-not-check: none in this repository; the region is documented as generated and carries no live state |

No parent or sibling scratch is acknowledged. The generated README region carries
plan-time facts only (scope, prerequisites, evidence, reverse membership, coverage)
because live work, acceptance and availability change without a commit and a
committed copy would drift from lint's fresh render.
