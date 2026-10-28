# Statusgen canonical Git reads: qualification boundaries

The shared owner is `gitcore/`, with `gitcore/gittest/` retaining its test-only fixture
role. Statusgen uses the existing run-owned reader, rather than adding a second cache,
Git library or revision resolver. Supported reads use Go; positively observed unsupported
profiles select the original native operation before reading. A real read error is
uncertainty and never triggers native retry. Ordinary environment/replace semantics and
historically isolated provenance semantics remain separate.

The raw-author path reads encoded bytes and preserves the existing policy parser.
Cold ordinary, packed, linked-worktree and alternate fixtures clear test-local memo
entries; successful upper-call tests poison native execution. Typed header, timestamp,
diff and incomplete-graph controls are distinct: one counterexample does not retire
unrelated qualified APIs. Generic blob rendering, history/blame selection, all-best-base
and ancestry/diff verdicts remain native where the selected library differs. Their
actual counterexamples are retained in `gitcore/{headerread,readgap,diffread,graphfault}_test.go`
and `statusgen/gogit_history_test.go`.

`statusgen/git-launches.json` inventories every construction found by the final AST
counter, with its actual expression, disposition and caller owner. `moved` at a seam
means the supported path uses Go; compatibility branches can still execute Git.
Conditional compatibility definitions remain `retained` even when their supported upper
flow has moved. The finite revision-caller pass replaces the old broad deferred labels
with qualified subset ports or concrete remaining capability/public-output boundaries.
This inventory is not human acceptance.
Literal definitions, conservative unresolved-executable constructions and seam callers
are separate ceilings. These are static counts, not measured runtime launches.
The counter's own immutable-base reads are counted explicitly. The same final counter
finds 91 literal definitions, 12 conservative dynamic constructions and 88 seam callers
at delivering main `558f3e56d95e8e105a508a8a5dcb5723340fce81`; the candidate after the
finite caller pass has 84, 12 and 103 respectively. Additional typed compatibility seams
increase the static caller inventory. These counts provide no runtime launch or latency
claim and are not interchangeable with the original two-pattern census.

Canonical mutation runs require a faithful disposable full repository export, from the
spec module working directory, using `-j 1`. Module-only parallel copies cannot preserve
the lifted owner's sibling commands/targets or repository-layout guards. Do not label
that parallel route qualified. Capture healthy baseline, distinct caught control,
mutants, exact pre/post source hashes and restoration; preserve every failed experiment.

The original six brief Verify commands and independent Evidence/Review remain unchanged.
No timing result, canonical analysis PASS, packaging PASS, human acceptance or independent
verification is asserted by this note. Runtime alignment was authorized after the first
Go-analysis acquisition failed at Go 1.25 versus source Go 1.27; acquisition with Go
1.27.1 subsequently succeeded. Canonical policy admission and complete scans are separate
work. No allowance ID translation or expansion is authorized. The aligned runtime and analyzer
controls do not themselves admit a changed analysis policy or open a closed policy gate.

Desktools-v2/68 and PR #4541 remain with their owner. Only the two exact /76 test routes
amend the shared CLI registry; they do not change the independent workload route.

`TestRevisionCallerFlow` exercises actual supported upper callers with native revision
queries blocked; `TestRevisionPortProfiles` covers native profile/error separation when
present in the final source. Existing generic-blob, graph, timestamp and history controls
remain at their original canonical owners. Recursive symbolic refs, generic tree-object
resolution, configurable short IDs and public NOTICE/error framing additionally have
retained native/API diagnostic evidence; those private observations are not represented
as committed tests or as proof of unrelated operations. The rejected RawCommit batch
port preserved healthy bytes but changed the public failure text, so it remains native.
