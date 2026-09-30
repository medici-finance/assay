# Fixture contracts

## Semantic owners — one meaning, one home

| id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run) |
|---|---|---|---|---|---|---|
| S-thing | A thing. | `tools/desk/internal/owner/owner.go` — `Thing`. | none | `tools/desk/internal/dup/dup.go:4`: a declared copy (`git grep -n 'a\|b'` found it). | none | none |
| S-far | A meaning owned outside the walked tree. | `statusgen/far.go` | none | none | none | none |
| S-wide | A meaning whose cells mention the whole tree in prose. | `statusgen/wide.go` | none | `tools/desk/internal/pkg`: a listed package directory. The search ran over `tools/desk`, `tools/desk/cmd` and `tools/desk/internal/` and found nothing else. | none | none |

**How a brief cites this.** Fixture text after the table.

## Rule register
