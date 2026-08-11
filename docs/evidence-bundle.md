# Evidence Bundle -- export format + SOC2 change-management mapping

The `statusgen --export-evidence <from> <to>` subcommand produces a deterministic
gzip-compressed tarball of the methodology artifacts whose authored/dated timestamps
fall within the given date range. It collects the briefs, stream READMEs, register
entries, and Evidence blocks that a human reviewer (or an external auditor) would ask
for from an agent-heavy delivery process, with a machine-verifiable manifest.

The honest framing: the bundle evidences the **recorded** process, derived from
authored artifacts. It is an **input to** a compliance review, not a compliance
artifact in itself. It is **not an audit opinion** and does not attest ground truth.
Several of the controls it maps to are **advisory** rather than machine-enforced --
the [SOC2 mapping](#soc2-change-management-mapping) marks which is which, and that
column is the first thing to read.
An auditor can verify that a brief states it was verified by a non-implementer, but
the bundle does not independently prove the verification was actually performed by a
separate identity -- that corroboration is the reviewer-App / commit-author check
that happens in CI, which is a separate concern.

## Bundle format

A single `.tgz` (gzip-compressed tar) file containing:

- **Brief files** (`docs/streams/<stream>/brief-<NN>[-slug].md`) -- every brief whose
  `authored:` frontmatter date falls within the range. The file is included as-is, so
  its `## Evidence`, `## Verify`, and `## Task` sections are preserved.
- **Register entries** -- intake entries (`docs/streams/intake/*.md`) and findings
  entries (`docs/streams/findings/*.md`) whose `date:` frontmatter falls within the
  range. Included as-is. A repo with no such directory produces an `omitted` entry, not
  a silently empty section.
- **Stream READMEs** (`docs/streams/<stream>/README.md`) -- the README of every stream
  that contributed at least one in-range brief. This is where the lifecycle `Status`
  column and the `Verified` / `Reviewed` cells live; without it the bundle could not
  evidence who verified what or what state a brief reached.

  **Included whole, not date-filtered.** The table covers every brief in the stream,
  including briefs outside the requested range. The rows carry no per-row date to
  filter on, so a consumer reading a README row must check the brief's own `authored:`
  date before treating that row as in-range evidence.
- **`manifest.json`** -- a generated index with:
  - `generated` -- ISO 8601 timestamp of generation
  - `version` -- the statusgen release tag (`dev` for an unstamped build)
  - `files` -- per-file entry with `path` (repo-relative) and `sha256` (hex-encoded
    SHA-256 of the file content as stored in the bundle)
  - `omitted` -- **every artifact the exporter expected but could not bundle**, each
    with a `path` and a `reason` (a register directory that does not exist in this
    repo, an entry whose frontmatter or `date:` will not parse, a stream README that is
    missing). Read this first: it is the bundle's own statement of what it is missing.

### Determinism and exit codes

Every tar header carries a fixed zero modtime and files are stored in alphabetical path
order, so **the same statusgen binary, run against the same repo at the same commit with
the same date range and the same `-generated` value, produces a byte-identical bundle**:

```bash
statusgen --root .. --export-evidence 2026-07-01 2026-07-31 -o a.tgz -generated 2026-08-03T00:00:00Z
statusgen --root .. --export-evidence 2026-07-01 2026-07-31 -o b.tgz -generated 2026-08-03T00:00:00Z
cmp a.tgz b.tgz   # identical
```

Without `-generated` the timestamp comes from the clock, and two runs differ **only** in
the manifest's `generated` field -- the file set and every content hash still match
byte-for-byte. Reproducing a bundle for comparison therefore requires passing the
`generated` value from the manifest of the bundle you are checking against.

Exit codes: `0` complete · `1` error · **`3` exported but INCOMPLETE** -- something the
bundle was expected to contain could not be collected, and `manifest.json`'s `omitted`
array says what. A non-zero exit here is deliberate: a silently incomplete compliance
bundle is a worse outcome than a failed export.

## SOC2 change-management mapping

SOC2's Change Management common criterion (**CC8.1** in the 2017 trust services
criteria) asks whether the organization **authorizes, designs, tests, approves, and
implements changes** to infrastructure, software, and procedures in a controlled manner.
The artifact set this bundle exports speaks to each of those concerns.

> **Confirm the criterion identifiers before using this page with an auditor.** The
> mapping below is written against CC8.1 as stated above. We do not hold a licensed copy
> of the AICPA trust services criteria, so the criterion numbering here has not been
> checked against the authoritative text, and no claim is made about how the 2022
> revision numbers it. Treat the *left-hand column as our reading of the concern*, not as
> a verified citation.

### How to read the Enforcement column

This is the load-bearing column, and it exists because the rest of this repo was
corrected for exactly the failure it prevents -- describing a convention in language that
implies a platform control (the enforced-vs-advisory distinction the rest of this doc draws).

- **Enforced** -- a CI check fails and the artifact cannot pass lint. Machine-checked.
- **Advisory** -- a convention the adopting team honours and which the bundle *records*, but
  which nothing prevents a participant from bypassing. An advisory row is evidence that
  something was **claimed**, not that it was **done**.

| SOC2 concern | Bundle evidence | Enforcement | Where it lives |
|---|---|---|---|
| **Authorization** | Every brief carries a required `gate:` field (`model` or `human`), and a brief with any `risk:` answer `yes` must be `gate: human`. Both are lint-enforced (`brieffile.go` `requiredBriefKeys`; the `anyYes && gate != human` check). A `gate-why:` rationale is required **only** for risk-gated briefs (`gate: human` or any `risk: yes`) -- an unremarkable `gate: model` brief carries none, and that is compliant. | **Enforced** (presence + risk/gate consistency) | Brief frontmatter |
| **Design & specification** | The brief's `## Task` / `## Context` sections capture what was designed and why, with `sources:` frontmatter linking to the design input. Lint requires the keys to be present; it cannot assess whether the content is adequate. | **Enforced** (presence only) | Brief body |
| **Testing** | The `## Verify` section is a table of command + expected output, and the stream README's Verified cell names a dated runner. | **Advisory.** Nothing witnesses execution: no command is run, no exit code or output hash is recorded, so a Verify table is a **self-reported** claim that the commands were run and passed. An execution witness is an open, unbuilt proposal. | Brief body + stream README row |
| **Segregation of duties** | `verified` requires an independent (non-implementer) Evidence row and a Verified runner that does not look like self-verification -- both are real lint checks (`attribution.go`). | **Enforced as a string check, advisory as a control.** Lint compares *authored names* in self-written text; it does not corroborate them against commit authorship or a signed identity. A single participant writing both cells satisfies it. | Stream README table + brief Evidence section |
| **Monitoring** | Register entries (findings and intake) are an append-only log of knowledge-invalidations and inbound requests. `statusgen --lint` enforces **duplicate-id detection** and a **tombstone check** -- a register entry that has ever existed on main but is absent from the working tree is a lint failure, so entries cannot be quietly deleted (`registers.go`, `registerIntegrityProblems`). | **Enforced** (duplicate-id + deletion/tombstone) | Register directories in the bundle |
| **Approval & implementation** | The lifecycle column tracks each brief through `todo` -> `in-progress` -> `implemented` -> `verified` -> `done`; the Evidence section records what shipped. | **Advisory.** The Status cell is hand-maintained prose in a table. Lint constrains its *vocabulary* (a bare lifecycle token), not its *truthfulness* -- nothing ties `verified` to a merged PR or a passing check. | Stream README table |

There is one further limit that sits underneath every row and is not visible in the
table: **the merge gate itself is advisory.** Where branch protection and rulesets are not
available or not configured on your repos, the review criteria the tooling honours -- an
approving review at the current head, green checks, findings addressed -- are not
preconditions GitHub enforces. The bundle can show that a review
happened; it cannot show that merging was impossible without one.

**What the bundle does not claim:**

- It does not prove that the command in a Verify row was actually executed -- only
  that a runner recorded its expected output.
- It does not prove the runner identity is genuinely separate from the implementer --
  the date+runner convention is an authored claim; corroboration against commit
  authorship (the reviewer-App check in CI) is a separate step that the bundle does
  not perform.
- It does not assess whether the test coverage was adequate, or whether the evidence
  claims match reality -- it only bundles what was written.
- It does not claim to be complete on its own say-so. Check `manifest.json`'s `omitted`
  array and the exporter's exit code (`3` = incomplete) before treating a bundle as a
  full picture of the range.
- It does not evidence that the process was **enforced**. Rows marked Advisory above
  record a claim the adopting team made, not a control that would have stopped a participant who
  declined to honour it.

These limitations are inherent to any system where the evidence is co-located with the
work product. The value is that the bundle makes the **recorded** process
machine-visible and exportable, so an auditor can inspect it without needing access to
the repo or knowledge of the methodology's file conventions. A v-next direction is
cryptographic signing of the manifest (noting commit SHA and a detached signature) but
that is out of scope for this iteration.

## Usage

```bash
# Export all artifacts between 2026-07-01 and 2026-07-17
cd statusgen && go run . --root .. --export-evidence 2026-07-01 2026-07-17 -o /tmp/bundle.tgz

# Same, but byte-reproducible: supply the generation timestamp
cd statusgen && go run . --root .. --export-evidence 2026-07-01 2026-07-17 \
  -o /tmp/bundle.tgz -generated 2026-08-03T00:00:00Z

# READ THIS FIRST: what the bundle knows it is missing
tar -xzf /tmp/bundle.tgz -O manifest.json | jq '.omitted'

# Verify the bundle contains a manifest
tar -tzf /tmp/bundle.tgz | grep manifest.json

# Inspect the manifest
tar -xzf /tmp/bundle.tgz -O manifest.json | head -30
```

The `--export-evidence` mode is a self-contained subcommand like `--dora` or
`--trend`: it does not read or write `STATUS.md`, and it accepts exactly one
`--root`.