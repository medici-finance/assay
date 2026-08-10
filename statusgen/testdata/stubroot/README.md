# stubroot — synthetic second root for multi-root statusgen

A **committed, populated, synthetic** repo root used as the second `--root` when
exercising multi-root statusgen (assay-selfcontain/01). It is not a copy of any
real stream and never will be: duplicate stream names across roots are exactly
what multi-root HARD-ERRORS on, so a fixture cloned from a live stream would make
the documented verify procedure require the thing the implementation forbids.

It is deliberately **populated** rather than empty. An empty second root proves
only that statusgen survives finding nothing; the behaviour multi-root exists for
— per-root filtering, per-root registers, per-root Next-up — is only observable
when the second root actually has streams, briefs, findings and intake entries of
its own.

Contents:

```
stubroot/
├── docs/streams/multiroot-fixture/   # one stream, unique name, three briefs
├── docs/streams/findings/            # this root's OWN findings register
└── docs/streams/intake/              # this root's OWN intake register
```

`STATUS.md`, `docs/streams/FINDINGS.md` and `docs/streams/INTAKE.md` are the
GENERATED boards for this root. They are gitignored, not committed: a fixture that
carries a checked-in copy of its own output invites a stale copy to be mistaken
for the expected value.

Run against it:

```bash
cd statusgen
go run . --root .. --root testdata/stubroot          # emits both boards
go run . --root .. --root testdata/stubroot --lint   # lints both roots
```

The stream's `repo:` frontmatter names a repo that does not exist. That is
intentional — it is a *fixture* repo id, and statusgen only ever validates its
FORM (`<owner>/<name>`), never resolves it over the network.
