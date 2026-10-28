# What Assay is and is not

## Does adopting Assay make us compliant?

**No.** Adopting Assay does not confer, contribute to, or substitute for certification of
any management system. Nothing it produces is an audit opinion. Assay supplies delivery
artifacts; the management system and its assessment remain yours.

## What it is

Assay is a delivery methodology and toolchain that make a **recorded, exportable process**
available for inspection. Its evidence is an **input to a review**. The
[clause map](iso9001-mapping.md) distinguishes machine checks from advisory conventions:
an advisory record establishes that something was claimed, not that it was done.

## What you can cite — and its limits

| Shipped artifact | What it establishes | What it does not establish |
|---|---|---|
| [Evidence bundles](evidence-bundle.md) | Exported authored records, with file hashes and an explicit omissions list; release-keyed packs follow declared release scope. | Ground truth, adequate coverage, or completeness without reading the omissions and exit code. An empty declared release scope is possible. |
| [Tool-validation pack](../tools/toolvalidation/header.go), emitted by the [release workflow](../.github/workflows/release.yml) | Recorded mutation demonstrations for the declared desk-tools refusal gates, as at that release. | Coverage of every control, adequacy of the selected mutations, a re-run interval, or a rule for earlier results when a gate is later found blind. |
| [Release authorization record](../.github/workflows/release.yml) | The release notes' `authorized-by` line records the authorizing identity resolved by the release path. | The line is not an approval gate. The gated release environment holds the actual authorization. |
| [Release provenance](iso9001-mapping.md#84--86--supply-chain-and-release-control) | The workflow now emits SLSA build-provenance attestations and Sigstore bundles for published assets; `deskinstall` verifies them. | Provenance identifies the build workflow, not source correctness or intended-use validation. Pre-change releases lack bundles and remain sha256-only; a checksum detects substitution, not the builder's identity. |
| [Records and retention statement](records-and-retention.md) | This repository's record classes, writers, alteration-detection mechanisms and current repository-life retention practice. | An approved retention policy, prevention of every alteration, or your retention periods, disposition rules and legal holds. |

## Determinism and judgement

A model is in the loop: identical inputs are not guaranteed to produce identical judgements.
The board is derived from agent-authored artifacts with consistency linting and independent
re-verification; it is **not measured from ground truth** ([lifecycle](lifecycle.md#the-honest-claim-about-the-board)).
The deterministic checks — lint and the declared mutation gates — are distinct from those
judgements. The [exporter](evidence-bundle.md#determinism-and-exit-codes) produces byte-identical
bundles only with the same binary, repository commit, date range and explicit generation
timestamp. That reproducibility does not make the recorded claims true or the model deterministic.

## What is not provided

The [clause map](iso9001-mapping.md) identifies no delivered-defect notification path for a
shipped defect that could have produced wrong past results, and no stated support lifetime
or end-of-support policy for a released version. Assay does not provide validation for your
intended use or your quality policy, objectives, internal-audit programme or management review.

A vendor's own certificate would not validate this tool for your intended use; it would be
one input to your supplier evaluation. This repository holds no certificate and claims none.
Validation is intended-use-specific and inherently your act: nobody can perform it for you.
The [validation guidance](validation.md) and fixtures can help you produce your own record;
they cannot replace the judgement and evidence from your setting.

These claim boundaries are prose maintained by review; this page adds no enforcement.
