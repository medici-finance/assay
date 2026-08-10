# Streams — assay-toolkit tracking root

This directory is the [statusgen](../../statusgen/README.md)-driven tracking root for
the assay-toolkit repo. It exists so the `lint` check (`cd statusgen && go run . --root
.. --lint`) has a `docs/streams` to read — without it, statusgen aborts with
`reading ../docs/streams: no such file or directory` on every PR.

## Layout

| Path | Purpose | Writer |
|------|---------|--------|
| `<stream>/README.md` + `brief-NN-*.md` | A stream and its briefs (brief-v1 schema — see [`docs/brief-template.md`](../brief-template.md), [`docs/brief-rules.md`](../brief-rules.md)) | implementers |
| `FINDINGS.md` | Append-only findings register (`## F-NN — date — title`); knowledge that invalidates/updates a brief | anyone, append-only |
| `INTAKE.md` | Append-only intake register (`## I-NN — date — title`); front door for raw ideas | anyone, append-only |
| `STATUS.md` (repo root) | Generated board | **main CI only** (never hand-edit) |

Registers are append-only and sequence-contiguous — statusgen flags gaps and duplicates;
withdraw an entry with a tombstone, never by deletion. A working example of a populated
tree lives under [`examples/adopter-scaffold/docs/streams/`](../../examples/adopter-scaffold/docs/streams/).

Streams are tracked here; the assay-product roadmap seeded the first ones. The generated
board at `STATUS.md` is the current index — read it there rather than
listing streams in this file, which would go stale on every new stream.
