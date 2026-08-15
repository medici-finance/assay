# The bodycheck golden corpus

Every artifact in this directory is a **sanctioned house artifact or a synthetic
credential**, checked in so the shared secret scan is measured against real material
instead of asserted accurate. It is the corpus #781 asked for: that issue
catalogued nine structurally different false-positive shapes and argued — correctly —
that patching each shape as its own content pattern would miss the tenth.

Two directions, both release-blocking (`docs/desk-tools-gate-bar.md`):

| prefix | expectation | pinned by |
|---|---|---|
| `neg-*` | the scan MUST NOT flag it | `TestBodycheckCorpus` |
| `pos-*` | the scan MUST flag it | `TestBodycheckPositives` |

A change that clears the negatives by widening what the positives admit fails the bar.
That is not a slogan: #410 measured a 3% false-negative class on the one
credential format the path rule names, after a comment had asserted the class impossible.

## File format

    # corpus: <artifact name>
    # expect: clean | refused
    # refs:   <issues this artifact is drawn from>
    # shape:  <one line: what structural shape this is>
    # why:    <one line: why the expectation is what it is>
    <payload — everything after the last leading `#` line>

The header is machine-read (`corpus_test.go`); `expect` and the filename prefix must
agree, and a disagreement is itself a test failure.

## The `{{+}}` seam

Payloads carry a `{{+}}` seam wherever a long base64 run or a marker literal would
otherwise sit contiguous on the page. The loader deletes every seam before scanning, so
the bytes the scan sees are exact.

The seam is not cosmetic and it is not paranoia. `deskpr` secret-scans the branch diff
before it pushes — **including removed lines** — so a corpus of credential-shaped and
lockfile-shaped material written out contiguously would refuse the very PR that adds it,
under whichever scanner is currently installed rather than the one in the diff. This is
the same bootstrap the split fixtures in `bodycheck_test.go` work around, recorded there
at `scanSecret40`, and it is #380 in miniature: a guard that cannot be
maintained through the tooling it guards gets edited around instead of fixed.

## No real secrets, ever

Every positive artifact is **synthetic or a published example value** — AWS's own
documented example key, an obviously-fake token, a PEM body of repeated filler. Nothing
here is a live credential, and nothing here is an expired one either: an expired
credential is still a credential someone has to reason about, and a corpus is exactly the
place that reasoning gets skipped. If you add a positive artifact, generate it.
