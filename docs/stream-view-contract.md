# Stream view contract (v1)

A **stream view** is one stream's state as a single, versioned, self-describing
document: who the stream is, what it is for, where it stands, what changed in
a stated window, what needs a human, and — for every one of those sections —
where the content came from and whether it could be read at all.

This page is the contract. The Go types live in the importable package
`github.com/medici-finance/assay/statusgen/streamview` (statusgen itself is a
`main` package and cannot be imported); the producer-side parsing lives in
statusgen. The contract defines **shape and invariants only**: it introduces
no readiness or progress algorithm and no persistent store.

Contract identifier: **`stream-view/v1`** (`streamview.Version`).

## Identity

A stream's canonical key is its **owning repository plus its stream slug**:

```json
"identity": {
  "key": {"repo": "example-org/repo-a", "slug": "shared"},
  "display_name": "Shared Stream"
}
```

- `key.repo` is `owner/name`. It comes from the stream README's `repo:`
  frontmatter when declared, else from the repository the producer was
  configured to read the root as. A declared repo that disagrees with the
  producer's, or no repo at all, is an error — an identity is never guessed.
- `key.slug` is the stream directory name (the README's `stream:`).
- Two repositories that both carry a stream named `shared` are two streams:
  `example-org/repo-a:shared` and `example-org/repo-b:shared`
  (`Key.String()`). `streamview.SameStream` compares keys only.
- `display_name` is the README H1 heading up to its em-dash tagline.
  Presentation only — it never identifies, and two streams may share one.

### Rename

v1 defines **no rename record**. Renaming a stream (a new slug, or a move to
another repository) produces a **new identity**. History does not transfer —
not by matching display name, mission text or brief titles —
`streamview.ContinuesHistory(prev, next)` is true only for an identical key.
A consumer that wants to show continuity across a rename needs an explicit
migration record, which a later contract version may add; until then, two
keys are two streams.

### Cell and access context

Authorization context sits **beside** the key, never in it:

```json
"context": {"cell": "cell-one"}
```

The same stream read from two cells has byte-identical `identity`. A consumer
must not key caches, history or comparisons on `context`.

## Versioning and negotiation

- Every document carries `"contract": "stream-view/v1"` as its first field.
- `streamview.Negotiate(offered, accepted)` returns the **first** entry of the
  consumer's `accepted` list (its preference order) that the producer offers.
  No common version is an error — never a silent fall back to the producer's
  newest or oldest.
- Within one version the field set is fixed. `streamview.Decode` reads the
  version first and returns `*UnsupportedVersionError` for an unknown or
  missing version *before* decoding the body, then decodes strictly (an
  unknown field is an error) and validates every invariant below. A consumer
  never "best-effort" reads a version it does not support.

### Precedence

Where two sources could supply one value, the contract fixes the order:

| Value | Order |
|---|---|
| owning repo | README `repo:` frontmatter → producer-configured repo → error (never guessed) |
| mission | valid authored `mission:` block → legacy README prose outcome → absent |
| contract version | consumer preference order (first accepted that is offered) |

A present-but-invalid authored mission does **not** fall through to legacy
prose: it is reported as could-not-check with its diagnostics.

## Sections and availability

Every section carries a `provenance` object:

```json
"provenance": {
  "availability": "available",
  "sources": [{"repo": "example-org/repo-a", "path": "docs/streams/shared/README.md",
               "revision": "<full 40- or 64-hex commit id>"}],
  "observed_at": "2026-10-01T12:00:00Z",
  "reason": "",
  "diagnostics": []
}
```

| `availability` | Meaning | Content | `sources` | `reason` |
|---|---|---|---|---|
| `available` | read in full | allowed | required | optional |
| `partial` | read, but some of the section could not be produced | allowed | required | required |
| `could-not-check` | the input was missing, unreadable or invalid | **forbidden** | optional | required |
| `not-assessed` | no producer for this section in this revision | **forbidden** | optional | required |

**Missing never reads as a value.** A could-not-check or not-assessed section
that carries content is invalid — an unread decision source is not "nothing
needs you", an unread history is not "nothing changed". Source revisions are
full commit ids, never abbreviations or branch names, so every recorded value
can be re-read at exactly the revision it came from.

The sections are `mission`, `current_state`, `changes`, `needs_you`,
`evidence` and `outcomes`. `current_state` copies the stream status and every
brief's number and status **verbatim** from the source (brief id =
`<slug>/<num>`), with per-status counts; it never normalises or re-derives a
status. Frontier and holds need the eligibility evaluator; a producer that
does not run it marks the section `partial` with that reason.

## Mission metadata

A stream README may carry an optional `mission:` block in its frontmatter.
Legacy streams need **no migration**: without the block, the mission section
is the README H1 tagline (origin `legacy-prose`) — or `absent` — and
**success criteria, commitments and exclusions stay absent**. They exist only
when authored; the producer never derives them from prose.

```yaml
mission:
  version: 1
  outcome: A returning reader can tell what this stream is for and how success is shown.
  success:
    - criterion: The acceptance report records a passing run at a pinned revision.
      evidence:
        - docs/evidence/acceptance.md            # repository-relative path
        - example-org/repo-a#12                  # forge issue or pull request
    - criterion: The follow-up report is published.
      evidence:
        - {path: docs/evidence/follow-up.md, planned: true}   # not written yet
        - https://example.org/streams/shared     # external page (https only)
  commitments:
    - Keep the stream readable without the chat history.
  exclusions:
    - No overall progress percentage.
```

| Key | Required | Shape |
|---|---|---|
| `version` | yes | integer; `1` is the only supported mission version |
| `outcome` | yes | non-empty string |
| `success` | no | list of `{criterion, evidence}`; `criterion` required |
| `commitments` | no | list of strings |
| `exclusions` | no | list of strings |

An evidence entry is a string (a path, an `owner/name#N` forge reference, or
an `https://` URL) or a `{path, planned}` mapping. A path must be
repository-relative with no `..` segment; it is qualified with the stream's
owning repo in the view. A bare `#N`, a non-https scheme or an absolute path
is rejected.

### Diagnostics

Every defect in an authored block is collected (not just the first) and
reported as a statusgen lint **PROBLEM** of the form
`<stream>: invalid mission metadata — <diagnostic>`, for example:

- `mission: must be a mapping …`
- `mission: unknown key "sucess" …` — a misspelled key is never dropped silently
- `mission.version: is required …` / `unsupported mission version 2 …`
- `mission.outcome: is empty`
- `mission.success: must be a list …` / `mission.success[0].criterion: is required`
- `… evidence[0]: bare "#12" is ambiguous …` / `only https URLs …`

In the view, an invalid block yields a mission section with availability
`could-not-check`, the diagnostics, and no content.

## Changes window

`changes.window` is always stated, whatever the section's availability, so an
unread window is never mistaken for a quiet one. v1 supports exactly one
window: kind `trailing`, label `last 24 hours`, spanning exactly 24 hours and
ending at the view's observation time (`streamview.TrailingWindow`). A
"since your last visit" window needs per-reader state, which v1 does not
define; any other kind, label or span is invalid.

## Evidence references and dereference

`streamview.EvidenceRef` has three kinds: `path` (repo + repository-relative
path, optionally `planned`), `forge` (repo + number) and `url` (https).

`streamview.CheckBindings(view, resolver)` dereferences every binding a view
records against its **owning** repository through a caller-supplied
`Resolver` (the package ships none, so it stays offline):

- each section source must name a resolvable repo, a revision present in it
  and a path present at that revision; it is **stale** when the path's object
  at the repository head differs from the recorded one;
- a path evidence ref resolves at the revision its record pins, else at the
  mission section's revision for that repo; an absent path is a problem;
- a `planned` path is reported as planned, and is a problem only if it already
  exists (the planned marker is then stale);
- forge and URL refs are reported as **unchecked** — residue for the caller,
  never a pass.

## Consumer

```go
v, err := streamview.Decode(data)          // or Decode(data, "stream-view/v1")
var uv *streamview.UnsupportedVersionError
if errors.As(err, &uv) { /* refuse: uv.Got, uv.Supported */ }
```

`streamview.Encode` validates before marshalling, so a producer cannot emit an
invalid document; equal views encode to equal bytes.

## Acceptance

`bash tests/stream-brief/32.sh <case>` runs the offline acceptance cases
(`legacy`, `identity`, `negative`, `flow`, `dereference`, `mutation`) against
the fixtures in `statusgen/testdata/streamview`. `dereference` also checks the
source map below against the checkout; `mutation` exits 0 only after every
seeded defect is detected by its named assertion and the genuine fixtures
pass. A missing prerequisite exits 2 (could-not-check).

## Source map

Rows marked `existing` must resolve in this checkout (file present, symbol
found); rows marked `planned` must still be absent.

| File | Symbol | State |
|---|---|---|
| `statusgen/streamview/contract.go` | `const Version = "stream-view/v1"` | existing |
| `statusgen/streamview/contract.go` | `func Negotiate(` | existing |
| `statusgen/streamview/contract.go` | `func Decode(` | existing |
| `statusgen/streamview/contract.go` | `func ContinuesHistory(` | existing |
| `statusgen/streamview/contract.go` | `type AccessContext struct` | existing |
| `statusgen/streamview/contract.go` | `func TrailingWindow(` | existing |
| `statusgen/streamview/bindings.go` | `func CheckBindings(` | existing |
| `statusgen/model.go` | `MissionDiagnostics []string` | existing |
| `statusgen/parse.go` | `func parseMissionBlock(` | existing |
| `statusgen/streammission.go` | `func streamIdentity(` | existing |
| `statusgen/streammission.go` | `func streamViewSeed(` | existing |
| `statusgen/streammission.go` | `func missionProblems(` | existing |
| `statusgen/roadmap_streampage.go` | `func streamOutcome(` | existing |
| `statusgen/multiroot.go` | `func rootRepo(` | existing |
| `statusgen/main.go` | `missionProblems(streams)` | existing |
| `statusgen/main.go` | `"stream-view"` | planned |
| `statusgen/streamview/resolver_git.go` | — | planned |

The planned rows are the next steps: a statusgen flag that emits stream views
for every stream, and a git-backed `Resolver`. Neither is part of v1's
contract surface.
