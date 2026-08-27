# statusgen `--graph`: multi-hop evaluation

`statusgen --graph dot|jsonl` emits the typed graph the tool already parses every
run — streams, briefs, findings, intake entries and referenced issues, with
`contains` / `depends` / `unblocks` / `affects` / `sources` / `issues` edges — as
a derived-only export. No new store, cache, or graph DB is introduced: the graph
is rebuilt from the parse tree for the cost of a render, and the export never
reads or writes `STATUS.md` or any generated register view.

This note answers the question the brief posed before any heavier machinery is
proposed: **is the multi-hop value real?** It does so by working three real desk
questions by hand first, then checking the export's answer against the hand
derivation.

The export IDs are the existing typed IDs — a brief is `<stream>/<NN>`, a finding
is its `F-<slug>` id, an intake entry its `I-<slug>` id — so an answer read off
the JSONL is stated in the same vocabulary the board and the register spec use.

## How a multi-hop answer is read off the export

Each JSONL line is one `{"kind":"node",...}` or `{"kind":"edge","type":...,"from":...,"to":...}`
object. A multi-hop question is a graph walk over the edge lines:

- **downstream of a finding** = follow `affects` (finding → brief), then
  `unblocks` (brief → brief) transitively.
- **prerequisites of a brief** = follow `depends` (brief → brief) transitively.
- **register entries a brief derives from** = the `sources` edges out of that
  brief.

No query engine is needed for the current corpus: `grep` over the edge lines plus
a hand (or one-line script) transitive walk answers each one.

## Q1 (required) — which briefs are downstream of a finding?

**Transitive finding-impact closure.** The assay tree does not yet carry a
per-entry `docs/streams/findings/` register (findings still live in the legacy
`docs/streams/FINDINGS.md`, which is not the per-entry format this export reads),
so the worked case uses the representative fixture exercised by
`graph_test.go` — the mechanism is identical on real per-entry findings data.

Fixture:

- finding `F-token-expiry-bug` `affects` `alpha/02`
- `alpha/02` `unblocks` `alpha/03`
- `alpha/03` unblocks nothing

**Hand-derived expected answer.** Start at the finding, cross its one `affects`
edge to `alpha/02`, then take `unblocks` transitively: `alpha/02` → `alpha/03`,
and `alpha/03` has no outgoing `unblocks`. Downstream set = **{alpha/02,
alpha/03}**.

**Export's answer.** From the JSONL:

```
{"kind":"edge","type":"affects","from":"F-token-expiry-bug","to":"alpha/02"}
{"kind":"edge","type":"unblocks","from":"alpha/02","to":"alpha/03"}
```

`affects` from `F-token-expiry-bug` yields `{alpha/02}`; the transitive `unblocks`
walk adds `alpha/03`. Closure = **{alpha/02, alpha/03}** — matches the hand
derivation. The edge direction (finding → brief, brief-unblocks-brief) is pinned
by `TestGraphAffectsEdgeDirection`, so the walk cannot silently invert.

## Q2 — what must land before a brief can start?

**Transitive prerequisite closure**, worked on **real assay data**: the
`derived-board` stream (`--root <assay> --graph jsonl`).

`depends` edges emitted for `derived-board`:

```
03 -> 01, 03 -> 02, 04 -> 03, 05 -> 01, 05 -> 02, 06 -> 04, 07 -> 05, 07 -> 06
```

**Hand-derived expected answer** for `derived-board/07`: 07 → {05, 06}; 05 →
{01, 02}; 06 → {04}; 04 → {03}; 03 → {01, 02}. Prerequisite closure = **{01, 02,
03, 04, 05, 06}** — every earlier brief in the stream.

**Export's answer.** The transitive `depends` walk from `derived-board/07` over
the eight edges above reaches exactly `derived-board/{01,02,03,04,05,06}` —
matches. Today this question is answered by a model re-reading each brief file's
frontmatter; the export answers it from one edge scan.

## Q3 — which register entries does a brief derive from?

**Single-hop `sources`**, on the fixture: `alpha/03` names `F-token-expiry-bug`
in its `sources:` prose.

**Hand-derived expected answer.** One `sources` edge, `alpha/03` →
`F-token-expiry-bug`. A `sources` entry that names an *unknown* id, or is pure
prose, must produce **no** edge (typed-ID fidelity).

**Export's answer.** `{"kind":"edge","type":"sources","from":"alpha/03","to":"F-token-expiry-bug"}`
— and `TestGraphSourcesTypedIDFidelity` confirms an unknown id yields no edge. So
the provenance link is queryable only where it is a real typed reference, never
fabricated from narrative text.

## Verdict

The derived export answers all three multi-hop questions correctly for the cost
of a single render, reusing the IDs the board already speaks and adding no
persistent state. That is the whole value proposition realised: the latent graph
is now queryable without a store, a cache, or a graph DB. At the current corpus
size a `grep` + transitive-walk over the JSONL is more than fast enough, and the
byte-deterministic output means two exports diff cleanly.

**Recommendation: no further machinery.** Do not introduce a graph database or a
standing index on the strength of this. Revisit only if the corpus grows by an
order of magnitude, or if an interactive desk tool needs sub-second repeated
queries that a per-invocation re-parse can no longer serve — at which point this
same derived export is the input a cache or index would be built from, not
something it replaces.
