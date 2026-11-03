# WITNESSES register — one v2 witness record per run

This directory is the WITNESSES register: one witness-v2 record per file. It is a register,
not a stream — stream discovery skips it.

## Layout

```
docs/streams/witnesses/<stream>/<NN>/<record-id>.json
```

- `<stream>/<NN>` is the record's own `brief` field.
- `<record-id>` is the SHA-256 of the record's canonical encoding, the same id the Runner
  cell's `rec:` token names (`<identity> @ [w2] cr:<N> rec:<record-id>`).

A record lands in the same change request as the Evidence row that cites it. Records are
**immutable**: a re-run is a NEW record with a new id, never an edit of an existing one.

## Reading

`statusgen --coverage` judges a v2 cell against its record; the reader contract is in
[`docs/witness-readers.md`](../../witness-readers.md).

## Why this README declares itself

Readers that predate this register's reserved name still skip the directory, because this
README carries the register self-declaration (`spec/registers-v1.md` §7). Without it, those
readers stop on the first record with "stream directory witnesses has no README.md".
