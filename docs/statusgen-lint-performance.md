# Statusgen lint: bounded ordering-scan improvement

Issue #4360. The optimization reduces repeated regex work in the offline ordering-gate scan. It preserves the existing gate grammar, first-rule priority, exact matched byte span, dependency-reference split, negations, waivers, generated fences and notice severity. All other lint checks still run.

The scan derives mandatory first literal prefixes from Go's existing `regexp/syntax` tree. Ordinary ASCII lines without any possible prefix avoid the ordered regex scan. Candidates still use the original ordered rules. Unicode text and patterns with a nullable or unsupported first expression use the exact original matcher directly. No separate gate-word list, parser, policy authority, cache or online read is introduced.

Existing file-read memo validation, content-hash parse memoization and run-scoped git batching remain unchanged. A same-process source edit is still observed; no freshness, source, witness, Evidence, identity, closure or verification control is weakened.

## Measurements

The inspected baseline source was `61eb62a4a61bce0f2e0c69b98f1906de80400316`. Measurements are from one local host. The independent full-command comparison used the same 7,276-file corpus, with every file hash checked before and after each alternating pair. All ten invocations returned zero and produced byte-identical stdout and stderr. Each invocation was a fresh process; filesystem cache and other host work were uncontrolled.

| Measurement | Baseline | Candidate | Interpretation |
|---|---:|---:|---|
| Full CLI wall, median of five runs | 13.903 s | 12.292 s | No firm whole-command speedup; paired median reduction 2.40%, range -9.57% to 19.66% |
| Full CLI process-tree CPU, median of five runs | 22.332 s | 21.321 s | Includes git children; paired median change 0.085%, effectively neutral |
| Warm matcher, median of three benchmark samples | 10.171 us/line | 1.349 us/line | 86.7% reduction in this matched synthetic matcher workload only |
| Ordering scan cumulative sampled CPU | 1.89 s | 0.88 s | Direct lint test-entrypoint profile; hot-path attribution, not full CLI qualification |

The CPU profile invokes the existing direct lint test seam and is useful for identifying the hot path. The actual command comparison retains default CLI checks. The warm benchmark is separate from the fresh-process cohort. These results do not establish equivalent fleet savings, wasted CPU, dollar savings or human elapsed time saved.

An optional native selected-tool-profile experiment in an isolated configuration refused before lint because its required roster configuration was absent. Its authentic refusal records are retained as refusals, not performance measurements. No configuration was copied, identity changed, override used or refusal retried. Native own-process CPU and selected-profile phase comparison remain unqualified in this lane.

## Qualification and architecture boundary

Regression tests compare the actual production matcher against the prior ordered implementation across existing Markdown fixtures and adversarial case, Unicode, malformed bytes, nullable patterns, priority, waiver/fence and same-process edit cases. Focused race and vet checks pass. All canonical mutation anchors remain plantable. The local mutation harness has a healthy baseline, a distinct caught positive control and five caught mutations, zero survivors and zero could-not-mutate. This is local qualification, not truth-suite CI matrix enforcement.

The complete standalone statusgen suite found one existing fail-first fixture setup failure; the original baseline and candidate both reproduce it. The independent verifyrun lane owns its test-only fixture correction. The composed suite must qualify the final published combination; this note does not call the standalone failing run green.

The future architecture keeps pure contracts and mechanical helpers separate from admission and effects. This change keeps the ordering policy with its existing statusgen owner. A common-library extraction of these pure rejection mechanics needs a second actual consumer and end-to-end compatibility evidence. Shared recording already has its canonical `perfrec` module; the separate shared producer migration must retain consent, attribution and default behavior. These measurements neither implement that migration nor relabel historical records.
