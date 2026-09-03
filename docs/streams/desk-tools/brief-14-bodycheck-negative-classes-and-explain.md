---
brief: desk-tools/14
title: "bodycheck — three measured false-positive classes into the negative corpus, plus `--explain`"
why: >-
  The shared secret scan refuses every desk write it guards — PR bodies, titles, branch
  names, diffs, comments — and a refusal it cannot explain is a refusal the caller works
  around. A 24-hour sweep of fifteen desk-role and worker session transcripts found one PR
  body refused six times (and then breaker-tripped) for a sibling-repo doc PATH whose
  filename carried a 32-hex segment; other sessions lost rounds to long numeric slash-lists
  of issue numbers and to a documented `kind: Secret` example inside a README fence. Each
  refusal names only the run LENGTH, so the caller guesses which span tripped it. Two more
  reported classes — long Go test names and `sha256:` digest pins — were probed against the
  current classifier and are already clean; they are not in scope. This brief pins the three
  that refuse as negative-corpus fixtures, fixes them without widening what the positive
  corpus admits, and adds an `--explain` that names the rule and the location so a refused
  caller can act on the first round.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — probed in a worktree with a throwaway test calling `deskkit.BodyCheck` on eleven synthetic bodies: REFUSED — a doc path whose last segment is a 32-hex string (`…/2026-08-30-<32hex>.md`, reported as a 32-char run), a doc path with a 32-hex DIRECTORY segment (reported as a 63-char run: the segment plus the filename read as one run), `#101/102/104/…`-style numeric slash-lists of ten and fifteen four-digit groups (49- and 74-char runs), and `kind: Secret` inside a fenced block with a placeholder value (decryptedK8sSecret). CLEAN — a 58-character CamelCase Go test identifier in prose, a `-run '^Test…$'` line, `sha256:<64hex>` in a Dockerfile `ARG` and in an image reference, and a UUID path segment."
  - "The classifier and its rules: `tools/desk/internal/deskkit/bodycheck.go` — § isGitSHA (exactly 40/64 hex only), § isPathLike (slash-aware opaque budget; the 3.07% measurement that forbids widening it), § decryptedK8sSecret, § looksLikeWords, and the refusal text listing the exemptions."
  - "The corpus contract this extends: `tools/desk/internal/deskkit/testdata/corpus/README.md` (neg-*/pos-* prefixes, the header format, the `{{+}}` seam, the no-real-secrets rule) and `corpus_test.go` (`TestBodycheckCorpus`, `TestBodycheckPositives`, `TestBodycheckCorpusCanFail`)."
  - "The callers that surface the refusal and would carry `--explain`: `tools/desk/cmd/deskpr/deskpr.go` § scan surfaces (PR body, title, branch name, diff), `deskpost`, `deskreply`, `deskevidence` (BodyCheck sites)."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
exec-tier: strong
exec-tier-why: "(c) a security classifier — a rule widened to clear a negative can admit a
  credential shape the positive corpus does not yet carry, and every test stays green."
---

# Brief 14 — bodycheck: three measured false-positive classes into the negative corpus, plus `--explain`

## Dependencies
None.

## Context

single-point-of-failure: **the positive corpus** — it is the only thing that says a classifier
change did not widen what the scan admits. This brief keeps it the control and strengthens it
rather than routing around it: every negative fixture added here is paired with a positive
fixture of the SAME shape carrying a credential, so a rule that clears the negative by shape
alone reds the pair. Behind the corpus stands `deskpr`'s diff scan, which runs the same
classifier on every line the branch adds or removes — a second site, same rule, so NOT an
independent layer; the brief says so rather than counting it.

risk note — this brief declares `tools/desk/internal/deskkit/bodycheck.go`, which is on the
security-path trigger list, while answering all four risk questions "no". The answers stand
and here is why: every rule change is bounded by a paired positive fixture and a boundary test
that must stay RED-on-secret (Verify rows 3 and 5), so a widening cannot land green; and the
`--explain` surface prints a rule id, a line number and a redacted shape, never the span. A
reviewer who finds a rule that clears its negative by shape alone, or an explain line that
carries span bytes, flips `sensitive-data` to yes and takes the human gate.

files:
- `tools/desk/internal/deskkit/bodycheck.go` (three rule refinements; the explain surface)
- `tools/desk/internal/deskkit/testdata/corpus/neg-doc-path-hex-segment.txt` (NEW)
- `tools/desk/internal/deskkit/testdata/corpus/neg-issue-number-slash-list.txt` (NEW)
- `tools/desk/internal/deskkit/testdata/corpus/neg-k8s-secret-template-in-fence.txt` (NEW)
- `tools/desk/internal/deskkit/testdata/corpus/pos-hex-token-wearing-a-doc-path.txt` (NEW)
- `tools/desk/internal/deskkit/testdata/corpus/pos-digit-token-wearing-slashes.txt` (NEW)
- `tools/desk/internal/deskkit/testdata/corpus/pos-k8s-secret-literal-in-fence.txt` (NEW)
- `tools/desk/internal/deskkit/corpus_test.go` (the catalogued-shape list gains three entries)
- `tools/desk/cmd/deskpr/`, `tools/desk/cmd/deskpost/`, `tools/desk/cmd/deskreply/` (`--explain`
  plumbing: the refusal carries the explanation; no verb changes its verdict)
- `tools/desk/README.md` (the explain output shape)

facts:
- the three refused classes, with the rule that refuses each (probe of 2026-09-02):
  1. **doc path with a hex segment** — `../<repo>/docs/streams/<stream>/2026-08-30-<32hex>.md`
     and `…/findings/<32hex>/README.md`: the high-entropy-run rule; `isPathLike`'s opaque
     budget rejects a segment that is pure hex of any length other than 40/64.
  2. **numeric slash-list** — `#101/102/104/117/…` (ten four-digit groups): the high-entropy-run rule; segments
     are 4-digit runs, which `isShortDigitRun` admits only up to a short length and only a few
     times per run.
  3. **`kind: Secret` template in a fence** — `decryptedK8sSecret` refuses any `data:` /
     `stringData:` value that is not an `ENC[…]` envelope.
- **the fixes, each narrower than the class it clears — the reviewer's test is that each
  paired positive stays refused**:
  1. a segment that is EXACTLY 32 lowercase hex characters, **followed by a doc extension
     (`.md`, `.txt`, `.json`, `.yaml`, `.yml`) or by `/` and a further word-shaped segment**,
     inside a run whose other segments are word-shaped (the existing `looksLikeWords`), is a
     path segment. A bare 32-hex run, a 32-hex run with no word-shaped neighbour, and any
     other length (31, 33, 48, 63) stay refused — an MD5-shaped token in prose is exactly the
     positive shape.
  2. a run consisting ONLY of `#`-optional 1–6-digit groups separated by `/` (and optionally
     `,`/space-free), with every group numeric, is an issue-number list. One non-numeric
     character anywhere in the run keeps the current verdict; a run of 8-digit+ groups stays
     refused (that is a numeric token wearing slashes, the AWS-example lesson in `isPathLike`).
  3. a `kind: Secret` document is a TEMPLATE, not a decrypted secret, when EVERY `data:` /
     `stringData:` value is a placeholder: an angle-bracket token `<…>`, a `${…}` or `{{…}}`
     template expression, or the literal words `REDACTED`/`PLACEHOLDER`. One literal value
     among placeholders keeps the refusal. Fence or no fence does not matter — the reviewer
     should expect the fix at the value level, never a "skip fenced blocks" rule (a fence is
     where a real manifest gets pasted).
- `--explain` (the surface, not a new verdict): the refusal error gains a structured part —
  rule id (`high-entropy-run`, `decrypted-k8s-secret`, `pem-block`, `sops-…`, …), the 1-based
  LINE of the first offending span, its LENGTH, and a **redacted shape** (first 2 + last 2
  characters with the middle as `…`, and a character-class summary such as `hex`, `digits+/`,
  `base64`). The span itself is NEVER printed — the refusal must not become the leak. The
  desk verbs that scan (`deskpr`, `deskpost`, `deskreply`) print the structured part when
  `--explain` is passed and omit it otherwise, so existing transcripts do not change shape.
- the corpus file header and `{{+}}` seam rules are mandatory for every new fixture; the
  positive fixtures are synthetic (generated hex, generated digits, a made-up literal).
- `TestBodycheckCorpusCoversEveryCataloguedShape` enumerates shapes; the three new negative
  classes are added to its list so a deleted fixture fails the build.

## Ground rules
- No fixture may carry a real credential, expired or otherwise (corpus README).
- The positive corpus is never edited to pass; a positive that starts passing is a finding.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Fixtures first**: the three `neg-*` files from the probe bodies (headers per the corpus
   README) and the three paired `pos-*` files — run the corpus test and record that the
   negatives FAIL and the positives PASS before any classifier change (that is the
   fail-first witness the Evidence section expects).
2. **Rules** 1–3 per the facts, each as small as its statement; comment each with the shape
   it clears and the paired positive that bounds it.
3. **Explain surface**: a `ScanFinding` (rule, line, length, shape) attached to the refusal
   error via `errors.As`; the three desk verbs gain `--explain` printing it.
4. **Tests**: corpus green both directions; the `CanFail` sentinel still fails; a unit test per
   rule with the boundary cases named in the facts (31/33-hex, an 8-digit group, one literal
   among placeholders); an explain test asserting the span bytes are absent from the output
   and the line number is right on a multi-line body.
5. **README**: the explain shape and the sentence that it never prints the span.
6. **Nothing else.** No change to `deskpr`'s diff-scan scope; no new scan surface.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBodycheckCorpus$' -count=1` | exit 0 — every `neg-*` including the three new ones is clean |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBodycheckPositives$' -count=1` | exit 0 — every `pos-*` including the three paired ones is refused |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBodycheckCorpusCanFail$' -count=1 && go test ./internal/deskkit/ -run '^TestBodycheckCorpusCoversEveryCataloguedShape$' -count=1` | exit 0 — the harness can still fail, and the three shapes are catalogued |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestHexPathSegmentBoundaries$' -count=1 && go test ./internal/deskkit/ -run '^TestIssueNumberSlashListBoundaries$' -count=1 && go test ./internal/deskkit/ -run '^TestK8sSecretTemplateBoundaries$' -count=1` | exit 0 — the NEGATIVE controls: 31/33-hex, a bare 32-hex run, an 8-digit group, one literal among placeholders — each still refused |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestExplainNamesRuleAndLineNeverSpan$' -count=1` | exit 0 — rule id and line number present; the offending bytes absent from the explain text |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskpr/ -run '^TestExplainFlagPrintsFinding$' -count=1` | exit 0 — with `--explain` the refusal carries the finding; without it the message is byte-identical to today's |
| 8 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole module including every other scan-site test |
| 9 | check:ci | `gofmt -l tools/desk/internal/deskkit tools/desk/cmd/deskpr tools/desk/cmd/deskpost tools/desk/cmd/deskreply > /tmp/bc-fmt.out; test ! -s /tmp/bc-fmt.out` | exit 0 |
| 10 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Rule 1 clears every 32-hex run | row 5 (bare 32-hex) + row 3 (paired positive) |
| Rule 2 admits an all-digit token with slashes | row 5 (8-digit group) + row 3 |
| Rule 3 becomes "skip fences" | row 5 (literal among placeholders, inside a fence) + row 3 |
| Explain prints the span | row 6 |
| A verb's default output changes shape and breaks a transcript parser | row 7 |
| A new fixture is committed without the seam and `deskpr` refuses the PR that adds it | review-only — the corpus README rule; the implementer runs the diff scan locally before pushing |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). Row 1 of the Task
     (fail-first corpus run) is recorded here too. -->

## Review

Gate: model (all four risk answers no). Model-gated because the control that makes a classifier
change safe is mechanical and present: the positive corpus (row 3) plus the paired positives
and boundary cases (row 5) stay green, and a change that clears a negative by widening is red
on those rows before a reviewer reads a line. The reviewer confirms each of the three rules is
stated as narrowly as the facts say, that each paired positive is the same SHAPE as its
negative, and that row 6 proves the explain text cannot itself leak.
