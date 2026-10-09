# intake-exit-v1 — the intake exit record

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `tools/desk/cmd/scanloop/exitrecord.go` (issue lane) and
`statusgen/intakeexits.go` (intake register export).

Every inbound item the intake desk triages leaves by exactly one of five tracked exits. This
record writes down that one decision as a small, text-free line: what came in, which exit it
took, who decided and at what tier, when, and what it became. Two writers produce it, in two Go
modules, and both follow this one schema:

- **Issue lane — `scanloop`** (`tools/desk/cmd/scanloop/exitrecord.go`). `Land` writes one
  `mechanical` record per inbound item a pass carries out. The `scanloop land` verb writes the
  `judgment` record after a session routes a parked item by hand.
- **Intake register — `statusgen --intake-exits --json`** (`statusgen/intakeexits.go`). It exports
  one record per triaged, stamped intake entry, mapped from the entry's `disposition` (see
  `spec/registers-v1.md` §5.4), and ends with a summary line.

Both modules carry a `TestIntakeExitSchema_MatchesDoc` test. Each test reads the two tables below
and fails if its record type's JSON keys, or its closed role set, differ from them in either
direction. Change this document and both writers together, or neither.

## Where the records live

| Writer | Location | Committed? |
|---|---|---|
| `scanloop` (issue lane) | `<desk state dir>/intake-exits.jsonl`, by default `$HOME/.config/assay/intake-exits.jsonl`, one JSON object per line, file mode 0600, appended under the desk audit lock | No: local state on the desk host |
| `statusgen --intake-exits --json` (intake register) | standard output, one JSON object per line, then the summary object | No: derived on demand from the committed register |

The register stamp that feeds the export (`triaged`, `triaged-by`, `triager-tier` in an intake
entry's frontmatter) **is** committed. That is why every identity-bearing value is drawn from the
closed role set below and never from a free pattern: a pattern that admits a role slug also admits
a person's login.

## Fields

Every record carries exactly these keys, in this order. A key that has no value is written as an
empty string, never omitted.

| Key | Type | Meaning |
|---|---|---|
| `schema` | string | Always `intake-exit-v1`. |
| `source` | `issue` \| `intake` | Which front door the item came through. |
| `item` | string | `owner/repo#N` for an issue, `I-<slug>` for an intake entry. |
| `repo` | `owner/repo` | The repository the item belongs to. The intake writer leaves it empty only when it cannot resolve the checkout's `origin` to an `owner/repo` path. |
| `exit` | one of the five exits | `placeholder`, `bug`, `finding`, `needs-decision` or `rejected-watching`. `unrouted` is never an exit. |
| `detail` | `rejected` \| `watching` \| empty | Present only when `exit` is `rejected-watching`, and required then. |
| `artifact` | typed ref \| empty | What the item became (see "Typed refs" below). Required unless `exit` is `rejected-watching`. |
| `decided_by` | `mechanical` \| `judgment` | `mechanical` when the classifier computed the exit, `judgment` when a session chose it. |
| `triager_role` | closed role set | Who decided: a canonical desk loop name, or `driver` for a stamp a human wrote by hand. |
| `triager_tier` | `none` \| `any` \| `strong` | `none` for a `mechanical` record; `any` or `strong` for a `judgment` record. Never a vendor model name. |
| `opened` | RFC3339 \| empty | When the item came in the front door: the inbound event time for an issue, the entry's `date` for an intake entry. Empty when unknown. Never later than `triaged`. |
| `triaged` | RFC3339 | When the exit was decided. |
| `kind` | classifier reason \| empty | For an issue: the classifier's reason, one of `new-issue`, `update`, `unreadable-placeholder-state`, `no-scan-target`, `scan-target-outside-write-boundary`. Empty for an intake entry. |
| `trust` | admission state \| empty | For an issue: the trust gate's admission state (`ADMITTED`, `QUARANTINED`, `COULD-NOT-CHECK`). Empty for an intake entry. |
| `session_tag` | string \| empty | Join key: the same value the desk audit log's `sessionTag` carries for the writing session. |
| `dispatch_ref` | string \| empty | Join key to the dispatch that worked the item: `<claim_key>@YYYYMMDDTHHMMSSZ.<12 lowercase hex>`, where `<claim_key>` is the dispatch claim's key (for example `example-repo--example-stream--33`). Allowed in clear here only because the record file is local state; never copy it onto a public surface. A record that cannot know it joins by `item`. |

## Closed role set

`triager_role` in a record and `triaged-by` in an intake entry's stamp take exactly one of these
values. Writers and the register lint check membership in this list, never a pattern. A value
that is not listed, including any person's login, is refused by the writer and is a `--lint`
PROBLEM in the register.

| Role | Meaning |
|---|---|
| `the-desk` | The coordinator loop. |
| `worker-desk` | The dispatch loop. |
| `pr-review-desk` | The pull-request review loop. |
| `verify-desk` | The verification loop. |
| `intake-desk` | The front-door loop; the role every `mechanical` record carries. |
| `driver` | A human triaged by hand. Only a register stamp carries it; `scanloop land` takes its role from the running loop. |

## Typed refs

`artifact` is one of these shapes and nothing else. Free text is refused.

| Shape | Example | Used for |
|---|---|---|
| `<stream>/<NN>` | `example-stream/33` | a brief in a stream (`placeholder`) |
| `<stream>` | `example-stream` | a stream an intake entry was scoped into (`placeholder`; only the register export writes it, and `scanloop` refuses it) |
| `owner/repo#N` | `example-org/tracker#12` | an issue or pull request on a named repo |
| `#N` | `#12` | an issue on the record's own `repo` |
| `F-<slug>` | `F-desk-emits-briefs` | a finding entry (`finding`) |
| `scan-pr:owner/repo` | `scan-pr:example-org/tracker` | a new draft scan PR whose number the lane did not read back (`placeholder`) |

A coalesced scan PR whose number is known is written as `owner/repo#N`.

## Never recorded

A record never carries, in any key: an issue or entry title; an issue, comment or entry body;
comment text; an author's or assignee's login or name; a human's reply prose; a prompt, a
transcript, or a vendor model name. The writers have no key to put them in, and the schema test
fails if one is added.

These records describe how the front door routes work, so that a cheaper decision model can be
measured against the current one. They are never used to rank people or agents.

## Summary line (`statusgen` export only)

The export ends with one object that is not a record:

```json
{"stamped": 2, "unstamped": 1, "unmapped": 0}
```

Every triaged entry (any disposition other than `new`) lands in exactly one count:

- `unmapped`: its disposition has no exit mapping, for example `decision-needed` with no
  `decision-issue`, or a value outside the register's vocabulary. It is counted, never guessed,
  whether or not it carries a stamp.
- `unstamped`: it maps, but it does not carry a complete, valid stamp (all three of `triaged`,
  `triaged-by`, `triager-tier`). It gets no record, because who decided and when are never
  guessed. Entries triaged before the stamp existed land here; there is no backfill.
- `stamped`: it maps and is stamped, so it is exported as one record.

An untriaged entry (`disposition: new`) is in none of the three counts.
