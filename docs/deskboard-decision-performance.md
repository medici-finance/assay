# Deskboard decision observations — issue #3373

`dispatch` and `throughput` retain their canonical queue builders, caps and admission
rules. This change removes repeated decision discovery and issue reads within one
admission pass. It does not replace the board classifier, introduce a scheduler or
deploy a shared service.

## Observation algorithm and authority

1. Read each local brief and evaluate its execution tier before remote work.
2. Resolve the existing role-bound forge once per repository that needs a decision read.
3. Lazily discover decision markers once per repository, through a fixed typed mode of
   `SearchIssues`. Walk complete pages or report unknown. Include closed and decided
   issues: the lowest matching issue must retain its existing meaning.
4. Index discovered marker bodies by exact spelling. A brief selects its literal id
   plus the same optional legacy spelling that `deskdispatch` searches; another
   project's colon marker is not an alias. Reconfirm membership
   using fresh `GetIssue` observations and the same lowest-numbered-candidate helper
   as `deskdispatch`. A search body's marker alone cannot hold a row.
5. Share each issue observation and canonical `BriefDecisionWait` result within this
   pass. Read remaining issues under the existing global six-worker pool. Fold results
   in input order, including held and could-not-check explanations.

A failed discovery never becomes an empty result. An explicit `decision-issue` does
not depend on discovery. State, labels, label re-application, immutable ratifier identity,
comment history completeness and missing-ratifier handling retain their existing
evaluator. Unknown rows remain visible and named, as before. Every new admission pass
starts with fresh observations; no ruling is persisted or reused across sweeps.

The discovery mode retains the existing admitted, read-only forge seam and provider
clients. It accepts no caller qualifiers. GitHub checks `total_count`,
`incomplete_results`, bodies, duplicate identities and next-page signals, bounded by
the provider's ten-page/1,000-result search window. GitLab walks its existing bounded
issue-page window, checks continuity and rejects duplicate or missing pages. Neither
adapter retries through a different identity or permission scope. GitHub marker
pages are sorted by creation time ascending; pagination completeness checks still
apply because sorting does not make a multi-request read transactional.

The dispatcher's selective lookup uses the same typed mode, with an opaque brief
spelling. Each backend owns its query syntax: GitLab uses description search,
not GitHub's `in:body` qualifier. Both paths filter exact marker spellings before
the same fresh membership check. Native adapter fixtures cover selective/shared
parity and retain GitLab's refusal when an issue and merge request share a number.
No live GitLab execution is claimed.

Discovery failures carry closed diagnostic categories: result/page limit,
incomplete response/pagination, local query refusal, missing minted credential, rate limit,
authentication/authorization, or other transport/response failure. A GitHub
short-page overrun is incomplete rather than a result-cap failure. Both shared
and selective paths validate opaque tokens before provider work, including
embedded quote/backslash and Unicode whitespace/control rejection. Raw provider bodies and locators are not rendered in
row reasons. A failure remains unknown rather than an empty healthy queue. Missing-token guards on both adapters retain their existing exit-6 `Unverifiable` contract and no-network/no-ambient-fallback behavior. A typed cause supplies the safe credential-absence label; it grants no identity and mints no token.

Scope remains the measured repeated-read regression. `throughput` still needs the
canonical state/label/history/ruling checks to determine its decision-hold count;
it does not substitute a weaker shortcut. The historical App-search symptom has
not been reproduced or root-caused, so the PR references issue #3373 without
closing it or claiming that symptom resolved.

The observation owner is in `deskkit`; `deskboard` owns only bounded scheduling and
presentation. This is an in-process step toward a shared read view, compatible with
desktools-v2's typed query boundary and the proposed shared-observation architecture.
It does not claim the future mirror, service, custody transition or event projection
has shipped. Downstream actors retain independent authority checks before effects.

## Matched perf-v3 experiment

The repeatable `TestAdmissionPerfV3` fixture runs the actual admission path, native
GitHub adapter and perf-v3 process wrapper/writer/reader. Only the remote forge is
replaced. It supplies twelve distinct, human-gated brief rows in one repository,
complete unruled decision histories, one marker-discovery page and 8 ms of injected
latency per HTTP request. All twelve rows must remain held with zero unknowns.

The baseline is production source `cfe5c927ec7049728f4b97306beaedf43a444b9f` with only
the identical test harness added. The repaired production source and harness are
`093503fdf65bebb7a54d8532d683727601100638`. Subsequent test, report and mutation-anchor commits leave the measured production Go source and performance harness unchanged. This cohort
includes the independent review repairs for exact spelling, typed selective lookup,
classified safe diagnostics and stable GitHub paging.
Both local test binaries carry their respective source stamp. Five independent
processes per variant were run in alternating before/after order; `-test.count=5`
in one process is unsuitable because process-init time would accumulate.

| Quantity | Before | After |
|---|---:|---:|
| HTTP attempts, every sample | 60 | 37 |
| Search attempts, every sample | 12 | 1 |
| Other REST attempts, every sample | 36 | 24 |
| GraphQL attempts, every sample | 12 | 12 |
| Process wall time, ms | 593, 694, 734, 590, 599 | 82, 93, 75, 78, 74 |
| Median process wall time, ms | 599 | 78 |
| Own process CPU, ms | 31, 35, 40, 30, 34 | 26, 24, 22, 23, 25 |
| Median own process CPU, ms | 34 | 24 |

That is **38.3% fewer HTTP attempts** and **87.0% less median elapsed time in this
latency fixture**. It is not a production speedup claim. Small CPU samples and their
variance do not establish a general CPU improvement. Unique explicit issue references
save no discovery calls; repositories needing more pages or having many shared
decisions have different savings. The full CLI's root/queue/review work is outside
this bounded experiment and is not subtracted from its production telemetry.

Native request observations exactly match the fixture's independent HTTP witness.
The per-process records have measured request coverage and no delegated work. REST
points are unsupported; GraphQL points remain unknown on this local endpoint. The
unchanged twelve GraphQL requests are not reported as zero-cost or measured savings.
No wall/CPU totals from overlapping parent/child records or legacy schemas are added.
Telemetry consent is confined to test processes and their disposable fixture homes;
no operator consent, workflow grouping, audit ledger or live pin changes.

Binary SHA-256 values:

- Before: `ffebc60bf0fa6f942a0c490d5249903e091169ac8dbc4bcdfae6518341c741a8`
- After: `1dc9c24512914ae7ed40c5997ce5bc91a2c75286394eef93523ca3db6cf4e0d4`

To reproduce from each respective source tree with the same test file:

```sh
cd tools/desk
go test -c ./cmd/deskboard -o /absolute/scratch/board.test \
  -ldflags '-X github.com/medici-finance/assay/tools/desk/internal/deskkit.SourceSHA=<full-source-commit>'
ASSAY_TELEMETRY=1 /absolute/scratch/board.test \
  -test.run '^TestAdmissionPerfV3$' -test.v
```

Each invocation prints one schema-qualified `ADMISSION_PERF_V3` record. Repeat as
fresh processes, alternating variants. These are local builds, not installed releases.

## Regression qualification

Tests compare shared observations with the single-brief evaluator and independently
assert the required ruling outcomes. They cover overlapping concurrent readers,
fresh rulings on a second sweep, both brief-id spellings, current marker membership,
lowest issue identity, explicit references, repository isolation, incomplete discovery,
both native adapters' pagination, execution tier before remote work, and stable output
under deliberately different completion order.

The dedicated classic mutation harness is:

```sh
cd tools/desk
go run ./cmd/muhar -j 2 -spec cmd/deskboard/decision-admission-mutations.json
```

The repaired source qualifies twenty-seven mutations: a healthy pristine baseline and
positive control, twenty-seven caught faults, zero survivors and zero could-not-mutate.
Cross-project aliases, lost diagnostic class, raw provider-detail leakage,
GitHub syntax sent to GitLab, unstable paging, local-refusal/page-bound labels,
quote/backslash rejection, shared/selective validation parity, all four native missing-token origins, their safe label and the preserved unverifiable cause chain are included explicitly.

Existing review snapshots, conditional transports, per-PR reuse, quota floors,
root resolution, queue semantics, trust controls and perf-v3 observation machinery
are retained. No existing test is retired.
