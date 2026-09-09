# Report packs — a named install unit for periodic reporting tools

A **report pack** is a periodic reporting tool that an adopter can install the same way
they install the board generator: pin a version, run one `init`, and obtain a committed
report — **without building anything from source**. This document is the normative contract
for what makes a tool a report pack. It is the general form of the pattern the board
generator (`statusgen`) already proves; `qualgen` (the quality trend view, `QUALITY.md`) is
the reference pack.

The pattern is not invented here. It is the [`.assay-versions`](distribution.md) pin model
plus the single-writer, generated-workflow shape the board generator ships, named once so
each new report (the quality view now, others later) is a member of a checkable class rather
than a bespoke re-invention that reaches no adopter.

## Why name it

The house runs several periodic reporting instruments. One of them — the board — ships as a
pinned, sha256-verified release binary that the installer acquires and wires. Another shipped
to nobody: its workflow built the tool **from the source of the repository under test**, with
a header explaining that a released binary would lag the very source a pull request is
changing. That rationale is correct for the repository that *produces* the tool and
generalizes to no adopter — an adopter has no such source to lag, and building from source is
exactly the non-sanctioned acquisition channel the distribution model retires. Naming the pack
contract turns "self-hosted house CI" into "installable by anyone", once, for every report.

## The four criteria

A tool is a report pack if, and only if, it satisfies all four. Each is mechanically
checkable, so pack membership is a property a test can assert rather than a claim.

1. **Released, pinned artifact.** The tool is a member of the umbrella release: per-platform
   binaries plus a sha256 for each, and an adopter pins it with a per-artifact line in
   [`.assay-versions`](distribution.md) following the `<artifact>-<platform>` convention
   (e.g. `qualgen-linux-amd64 <tag> <sha256>`). This is acquisition **channel E** — the
   sha256-pinned release binary — and no other channel is sanctioned for an adopter
   (see [`adopting-assay.md`](adopting-assay.md) and `statusgen/channels.go`, the one declared
   channel source).

2. **Generated workflow and config.** The tool emits its own CI workflow and any config it
   needs through `<tool> init` — **generated, never a template the adopter copies**. A
   generated workflow cannot drift from the tool that reads it, and `assay:upgrade-assay` can
   re-emit it on an upgrade. `init` never overwrites an existing file; it fills gaps.

3. **Single-writer committed output.** The tool's committed report has exactly one writer.
   On a pull request the tool **renders to stdout and discards**; only the push-to-main path
   writes the committed file, and that path is gated by a writer environment variable the
   tool's own code enforces — a `--write` with the writer variable unset is **refused**, so
   CI, not a filename convention, is what makes CI the only writer. A branch never modifies
   the committed report.

4. **Operator values from config.** Every operator-specific value the tool needs loads from
   run-time configuration, never compiled in, and is never read from anything inside a pull
   request. Absent configuration fails closed (the value is treated as unset, not defaulted),
   exactly as the board generator's house knobs already resolve.

## The producing-repo seam

The pack contract deliberately admits **two** consumers of the same tool:

- **Adopters consume the pinned binary** (criterion 1). They never build the tool from source;
  they pin a released `qualgen-<platform>` and run `qualgen init`.
- **The producing repository self-hosts.** The repository that *is* the tool's source keeps
  building it from that source in its own workflow, because a released binary would lag the
  pull request changing the source it reports on. This is not a violation of criterion 1 — it
  is the one repository for which the released binary is the wrong artifact, and it is named
  as an accepted, tool-producing exception, not as an adopter channel.

State the seam explicitly wherever a pack is wired, so the next author does not rediscover it:
the same tool is installed one way by everyone and self-hosted by exactly one repository.

## Conformance

The channel-conformance sweep (`statusgen`'s `--lint` advisory, the one declared channel
source in `statusgen/channels.go`) reads a pack's adopter-facing surfaces and flags any that
still teach a non-sanctioned acquisition channel — a vendored copy, a `go install`/`go run` of
the module, a build-from-source invocation. A conformant pack teaches only channel E on those
surfaces; the producing-repo self-host is not an adopter surface and is not scanned. A pack
that has joined the release and teaches channel E therefore **reads as conformant**, while a
planted build-from-source or `go run` of a released pack tool still reddens the sweep — which
is what keeps the check load-bearing.

## The reference pack: `qualgen`

`qualgen` renders `docs/quality/QUALITY.md`, the quality trend view. It satisfies all four
criteria: it ships per-platform release binaries with checksums (pin `qualgen-<platform>` in
`.assay-versions`); `qualgen init` emits an adopter workflow and pin scaffold; `qualgen report`
renders to stdout on a pull request and writes the committed view only when
`QUALGEN_QUALITY_WRITER=ci` authorizes it; and its baselines and knobs load from configuration.
The `medici-finance/assay` repository itself remains the producing-repo exception — its own
quality workflow keeps building `qualgen` from source per the seam above.
