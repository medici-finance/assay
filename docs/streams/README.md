# streams/

Each initiative is a **stream** — a directory under `docs/streams/<stream>/` with a
README (frontmatter + a brief table) and self-contained `brief-NN-*.md` files.
Append-only registers live alongside: `FINDINGS.md` (knowledge that invalidates a
brief) and `INTAKE.md` (raw ideas).

`statusgen` reads these to generate the `STATUS.md` board and to lint the set in CI.
See the `example/` stream for the shape; delete it once you have your own.
