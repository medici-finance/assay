# VERIFY-OUTCOMES register — one file per verify outcome

This directory is the VERIFY-OUTCOMES register: one verify-outcome record per file, under a
per-stream subdirectory. It is a register, not a stream — stream discovery skips it.

## Why this exists (#882)

Every verify Evidence PR used to append one line to a single shared
`docs/streams/verify-outcomes.jsonl`. The forge merges pull requests server-side with no
`merge=union` driver, so every landing turned every sibling Evidence PR touching that one path
CONFLICTING. Writing each outcome as its own new file means concurrent Evidence PRs never touch
the same path: two PRs only ever pick the same path when they carry byte-identical content (the
same outcome), and two identical adds merge cleanly.

## Layout

```
docs/streams/verify-outcomes/<stream>/<NN>-<YYYYMMDDTHHMMSSZ>-<digest12>.json
```

- `<stream>` and `<NN>` come from the record's own `brief` field (`<stream>/<NN>`).
- `<YYYYMMDDTHHMMSSZ>` is the record's own `ts`, compacted, so a directory listing reads in
  time order.
- `<digest12>` is the first 12 hex digits of the SHA-256 of the file's exact bytes (the
  record's one JSON line plus a trailing newline) — the uniqueness term, since one brief is
  legitimately recorded twice at one merged sha (a re-issued outcome after an Evidence
  correction note).

Each file's content is exactly one JSON object, unchanged schema from the row this register
replaces (`ts`, `brief`, `outcome`, `sha`, and the optional verify-wake-v1 receipt fields).
Records are **immutable**: a correction is a NEW record (a fresh `ts`/digest), never an edit of
an existing one.

## Reading

Read through the choke-point reader for your module — never open a record path directly:

- `tools/desk`: `internal/deskkit.ReadVerifyOutcomes` + `.LatestPerBrief`.
- `statusgen` (a separate Go module, its own copy): `readVerifyOutcomeRecords` +
  `latestOutcomePerBrief` in `verifyoutcomes.go`.

Both readers also union in any legacy `docs/streams/verify-outcomes*.jsonl` line still present
from before the migration, deduped by content digest, so a record present in both layouts is
counted once. "Latest outcome per brief" is always a `ts` comparison — never a line-position or
file-listing-order read.

## Writing

Only `deskevidence --outcome-record <local-file>` writes here, as the verifier App. An existing
path with identical bytes is a noop; an existing path with different bytes is refused — see
`tools/desk/README.md`.

## Migration

The shared log this register replaces is migrated with `statusgen outcomes split --root <dir>`
(idempotent; `--check` audits without writing). The log itself stays in place, frozen, until
every open PR that still touches it has landed — see `migrations/0004-*.md`.
