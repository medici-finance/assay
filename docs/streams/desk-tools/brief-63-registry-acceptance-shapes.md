---
brief: assay:assay:desk-tools:63
title: "Admit the open false-positive shapes as registry entries, each with the refusals that bound it, and cap word-shaped admission"
why: >-
  Five benign shapes still stop desk writes and are unblocked only by an audited scan override
  or by renaming text that cannot be renamed: Go build-info module hashes, platform-suffixed
  digest assignments, sops age-armor lines seen in a diff fragment, Kubernetes qualified label
  keys, and `+`-joined identifier lists. In the other direction, one admission is too wide: a
  long run that splits into word-like pieces is admitted however random it is. Once the
  registry exists, each of these is one declared entry with its context, its admitted
  examples and the refusals it must keep, not one more predicate in a chain. This brief adds
  those entries, makes the whole-artifact grammars work on a diff by anchoring them to the
  file the diff line comes from, adds an entropy cap to word-shaped admission, and records the
  ruled refusal of labelled 32-hex digests as a row that keeps refusing.
wave: 2
depends: ["desk-tools/61", "desk-tools/62"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  Every entry here makes the trust-boundary scan admit runs it refuses today, and the entropy
  cap makes it refuse runs it admits today. An admission that is one character too wide lets a
  credential reach a public body or branch; a cap that is too tight stops ordinary writes. The
  human picks which entries land before work starts, and reads the security review of each
  entry's bounds before the ready flip.
decision-trigger: start
issues: [4284, 4723, 2321, 5257, 3196, 4097, 1642, 929]
outcome: none
schema: brief-v2
version: 1
authored: 2026-11-10 by a desk-dispatched authoring session (error-class design for issue 4284; the acceptance shapes)
sources:
  - "medici-finance/assay#4284 — the class: benign shapes cleared by enumerated exception; the hypothesised invariant is an enumerated, tested class list whose every entry carries a refused-before and admitted-after repro set"
  - "medici-finance/assay#4723 — go version -m module hashes (h1: then 44 base64 characters) refused in the tab-separated and escaped forms"
  - "medici-finance/assay#2321 — OCI image digests; re-probed 2026-11-10 @ 450ac4047: name:tag@sha256 and name@sha256 forms already scan clean, a platform-suffixed digest assignment of the form ARG <NAME>_SHA256_<ARCH>=<64 hex> still refuses (a 70-character run), because the digest-key check requires the name to end in the digest word"
  - "medici-finance/assay#5257 — an age-armor line from a sops file refused in a branch diff; the sops grammar admits only a whole sanctioned document, and a diff fragment is never one"
  - "medici-finance/assay#1642 — the 2026-10-05 occurrence: a Kubernetes label key of the dns-prefix/camelCase form (35-character run), unrenamable because it is an upstream operator's API; the ratified ruling on that issue keeps refusing its case (a), the leading-acronym identifier"
  - "medici-finance/assay#4097 — word decomposition admits a long run that splits into word-like pieces, with no entropy cap"
  - "medici-finance/assay#3196 part 1 and the ruling on medici-finance/assay#3280 (option A: keep refusing labelled 32-hex digests), with the guidance follow-through on medici-finance/assay#5156"
  - "medici-finance/assay#929 part 2 — the vendor-example allowlist ask, put to the human here as an option with default no"
  - "medici-finance/assay#2009 — a test expression of `+`-joined identifiers refused as a 47-character run"
  - "freshness-checked 2026-11-10 @ 450ac4047 (origin/main) by probing a scratch copy of the scanner"
exec-tier: strong
exec-tier-why: "Each entry is a trust-boundary widening that must be bounded by refusals a reviewer can audit, and the artifact anchor joins the diff index to whole-file grammars without making the scanner diff-aware."
domain: complicated
consumers:
  - "tools/desk/README.md: fixed-here (the rendered benign-shape table gains the new entries and the cap)"
  - "docs/contracts.md: fixed-here (R-body-secret-scan's tests column names the new tests)"
  - "tools/desk/cmd/deskpr/deskpr.go: fixed-here (the branch-diff scan passes the file-anchored admitted lines)"
id: 1d39514d-4cea-4416-8f87-84af9eecf4cb
---

# Brief 63 — Admit the open false-positive shapes as registry entries

## Dependencies

- **desk-tools/61** — the registry, its bounded-entry test and the recorded golden verdicts.
  Every admission here is a registry entry, so this brief cannot start before it.
- **desk-tools/62** — the diff line index. The artifact anchor maps each diff line to its file,
  side and file line through it.
- desk-tools/64 decides which diff lines the branch-diff scan reads. The anchor works on
  whatever lines it reads, so this brief stands for every answer to 64.
- The worker change in flight for the `KEY=<value>` exemption (#1642's 2026-11-10 occurrence,
  #5825, #5974) is not duplicated. If it has landed, it is a registry entry already. If not, it
  lands as one, in its own PR.

## Context

files:
- `tools/desk/internal/deskkit/benignshapes.go` (planned) — created by desk-tools/61; new entries and the cap
- `tools/desk/internal/deskkit/benignshapes_test.go` (planned) — created by desk-tools/61; the `TestShape` (planned) tests
- `tools/desk/internal/deskkit/testdata/benign-golden.txt` — changed only on the lines the delta file names
- `tools/desk/internal/deskkit/testdata/benign-golden-delta.txt` (planned) — NEW
- `tools/desk/internal/deskkit/testdata/corpus/` — new fixtures only; no existing fixture is edited
- `tools/desk/internal/deskkit/mutations.json` — two new mutations
- `tools/desk/cmd/deskpr/deskpr.go` — the branch-diff scan passes file-anchored admitted lines
- `tools/desk/cmd/deskpr/armoranchor_test.go` (planned) — NEW
- `tools/desk/cmd/deskpr/armoranchor-mutations.json` (planned) — NEW
- `tools/desk/README.md`, `docs/contracts.md`
- **add** — `changelog/<branch-slug>.md` — one fragment

facts (probed 2026-11-10 @ 450ac4047 on a scratch copy):
- The high-entropy run is `[A-Za-z0-9+/=]{32,}`, so `:`, `.`, `_`, `-`, `@` and tabs end a run.
  A Go build-info hash is a 44-character run that begins after `h1:`. A digest assignment's run
  starts after the last `_` of its name, which is why `<NAME>_SHA256_<ARCH>=` presents the
  architecture word and not the digest word.
- Scanning clean today: OCI references with `@sha256:`, whole go.sum files, CamelCase names of
  word-shaped segments, dash-joined Kubernetes keys, and a whole sanctioned sops document.
- Refused today, and benign: the build-info line in its tab and escaped forms, the
  platform-suffixed digest assignment, a Kubernetes key with a camelCase name after a DNS
  prefix, a sops age-armor fragment in a diff, and `+`-joined identifiers.
- Refused today, and must stay refused: a random base64 run after a DNS prefix and a slash, a
  labelled 32-hex md5 digest (ruled), random base64 and hex of every length, and digests of the
  wrong length.
- The existing positive fixture with a token behind a leading `+` already bounds `+` handling;
  the plus-joined entry must keep it refusing.

layering: no new structure beyond one input to the scan. The artifact anchor is the one
cross-module join. deskpr asks deskkit for the admitted spans of each touched file, using
the registry's artifact entries. It maps them onto stripped-diff lines through the desk-tools/62
index, and passes the scan a set of pre-admitted lines, each tagged with the entry that admitted
it. The scanner treats that set as one more registry context, `anchored`. It never parses a diff.

design-fit:
  owner: tools/desk/internal/deskkit/benignshapes.go — the registry desk-tools/61 creates
  contract: S-publication-scan
  retires: [the scan overrides these five shapes need today, the whole-surface-only reading of the sops grammar on diffs]
  weight: "verbs 0, flags 0; registry entries +5 (one is the anchored context); refusals +1 (the word-salad cap); one scan input (the anchored line set)"
  why-add: "Each entry is the only alternative to a recurring override or an impossible rename. Removal was considered: widening an existing entry instead (for example, letting the digest-key entry accept any trailing word), and refused, because a wider entry is bounded by fewer refusals than a new narrow one."

## Human decision

The desk's secret scan stops several kinds of ordinary text that are not secrets: Go module
hashes printed by the Go toolchain, checksum settings with a platform suffix such as AMD64,
encrypted-file armor lines seen in a code diff, Kubernetes label keys with a camelCase name,
and lists of identifiers joined by `+`. Today each one needs an audited override or a rename.
The scan also lets through a long run that happens to split into word-like pieces however
random it is. This change admits the benign shapes, each with refusals it must keep, and adds
a randomness cap to the word-piece rule. Labelled 32-character hexadecimal digests stay
refused, as already ruled. The decision is which of these land.

Options:
1. **All five admissions and the cap, and no vendor-example list** — every admission ships
   with at least two same-length refusals that must keep refusing. Any currently admitted run
   the cap starts refusing is listed for review.
2. **Only the three digest and armor admissions and the cap** — the Kubernetes key and the
   `+`-joined list keep needing an override. This is narrower, but the override habit stays for
   those two shapes.
3. **Option 1, plus an exact-string list of vendor documentation example credentials** — admits
   the documented example strings verbatim and nothing near them. This removes some overrides on
   documentation, but it puts credential-shaped strings into the scanner's own source as data.

Default if no answer: none — blocks until answered. Option 1 is the recommendation. A
widening of a credential control is never adopted by silence.

## Ground rules
- Ruled option A on the decision issue, which is option 1 above: https://github.com/medici-finance/assay/issues/4284#issuecomment-6774321215
  The ruling admits go build-info hashes, platform-suffixed digests from the closed
  architecture set, file-anchored sops armor, and Kubernetes qualified keys with `+`-joined
  identifiers. It counts those last two as one group, so its four shapes are this brief's five
  admissions. Each ships with paired refusals, plus the word-shaped entropy cap. It adopts no
  vendor-example list, and the labelled 32-hex digest stays refused. Options 2 and 3 are not
  taken.
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement only the entries the recorded human answer selects. An entry outside it is a STOP.
- Never edit an existing corpus fixture. Never write a credential-shaped literal anywhere. Build
  bounding runs from a fixed seed at test time, or split them with the corpus seam.
- Do not touch the literal credential arms, the run class or the threshold.
- Each entry is a separate commit with its tests, so a reviewer can read one widening at a time.

## Task
1. **Go build-info hash** (#4723), context `line`. Admit a run only when it is exactly 44
   characters, ends in `=`, decodes as base64 to 32 bytes, and follows `h1:` on a line whose
   preceding fields are a module path and a Go version or pseudo-version, whether separated by
   tabs, spaces or literal `\t` escapes. Bounds: the same run with 43 and 45 characters, the
   same run without a module and version before `h1:`, and a random run after `h1:` that does
   not decode to 32 bytes.
2. **Platform-suffixed digest assignment** (#2321), context `run`. Extend the digest-key
   assignment so that a name ending in a digest word followed by one platform word from a closed
   set (AMD64, ARM64, X86_64, AARCH64, I386, PPC64LE, S390X, RISCV64) is accepted, with the value
   still exactly 40 or 64 hex. Credential stems in the name still refuse. Bounds: a credential
   stem plus the platform suffix, an unlisted suffix, 63 and 65 hex, and a base64 value. Add one
   acceptance case recording that the two OCI reference forms already scan clean.
3. **File-anchored artifact context** (#5257). Export a deskkit function that runs the
   registry's `artifact` entries over one file's bytes and returns the admitted line ranges,
   each tagged with its entry. In deskpr's branch-diff scan, read each touched file at HEAD for
   added and context lines. For removed lines, read it at the merge-base of the default ref and
   HEAD, the side the symmetric branch diff numbers removed lines against. Never read it at the
   default-branch tip, which deskpr's own output also calls the base. Map each admitted range through the desk-tools/62 index, and
   pass the resulting stripped-diff lines as the `anchored` set. The set exempts only the
   high-entropy loop; the literal arms still run on every anchored line. An unreadable file, a
   file over the size cap the outbound check already uses, or a file that is not a sanctioned
   sops document anchors nothing. Bounds: a random 64-character run in an ordinary YAML file in
   the same diff, a plaintext high-entropy value in a sops file's unencrypted key, an
   armor-shaped block in a file that is not a sanctioned sops document, and a provider-token
   literal on an anchored line.
4. **Kubernetes qualified key** (#1642, 2026-10-05), context `run` with the raw line. Admit a
   run that follows a DNS subdomain prefix (two or more dot-separated lowercase labels) and
   whose part after the `/` is a camelCase or dotted name of word-shaped segments under the
   existing identifier rule, including its leading-acronym bound. Bounds: a random base64 tail
   after the same prefix, a prefix of one label, and a ruled leading-acronym name.
5. **`+`-joined identifiers** (#2009), context `run`. Admit a run that splits on `+` into two or
   more segments, each word-shaped under the existing identifier rule. Bounds: one random
   segment among word-shaped ones, a run with a leading `+` (the existing positive fixture),
   and a random base64 run that contains `+`.
6. **Labelled 32-hex digests stay refused** (#3196 part 1, ruled on #3280). Add
   `TestShapeMd5StaysRefused` (planned), asserting that a generated 32-hex run labelled as an md5 digest
   still refuses with the high-entropy rule. The digest entry's `Doc` line names the
   alternative the ruling gives, a sha256 digest.
7. **Entropy cap on word-shaped admission** (#4097). Admission by word decomposition also
   requires the run's Shannon entropy per character to be at or below a cap. Set the cap as the
   highest value observed over the corpus negatives and the named benign fixtures that the word
   rule admits, plus 0.1 bits. Never calibrate on generated golden inputs: a generated run the
   word rule admits today is exactly the #4097 case the cap must refuse. A generated input the
   cap flips from clean to refused appears in the golden delta file, naming the cap. `TestShapeWordSaladCap` (planned) generates at least 200 word-salad runs
   from a fixed seed, built from random letters cut into word-length pieces, and asserts that
   at least 95% refuse while every corpus negative still scans clean.
8. **Paired refusals.** `TestShapePairedRefusals` (planned) asserts that every entry this brief adds has
   at least two `Bounds`, one of them a random run of the same length in the same context. 61's
   `TestBenignEntriesBounded` (planned) then checks each entry's widening flips one of them.
9. **Golden delta.** Regenerate the golden. Write `benign-golden-delta.txt` with one line per
   changed input: `<id>` TAB `<old>` TAB `<new>` TAB `<entry-or-cap>`. `TestShapeGoldenDelta` (planned)
   reads the base golden from `BENIGN_GOLDEN_BASE`. It skips when the variable is unset, so the
   whole-package run stays green, and row 3 asserts a PASS line, so a skip never satisfies it. It
   asserts the delta file is exactly the difference, that every refused-to-clean line
   names an entry this brief adds, that every clean-to-refused line names the cap, and that no
   clean-to-refused line comes from a corpus input.
10. **Fixtures.** Add one negative and one positive corpus fixture per admission, with names
    under 32 characters (for example `neg-go-buildinfo-hash.txt`, `pos-buildinfo-wrong-len.txt`,
    `neg-digest-key-arch.txt`, `pos-digest-key-cred-stem.txt`, `neg-k8s-qualified-key.txt`,
    `pos-k8s-key-random-tail.txt`, `neg-plus-joined-ident.txt`, `pos-plus-joined-random.txt`,
    and `pos-word-salad-run.txt`), using the seam for every run.
11. **Mutations.** Add two mutations to `mutations.json`, named `word-salad cap removed` and
    `platform suffix set opened`, and `TestShape` (planned) to its `test` selector. Add
    `armoranchor-mutations.json` with four mutants: an unreadable file anchors everything, the
    anchored set also skips the literal arms, removed lines are anchored against the HEAD file,
    and removed lines are anchored against the default-branch tip.
12. Name the new tests in R-body-secret-scan in `docs/contracts.md`. The README table is
    rendered from the registry (desk-tools/61). Change every consumer entry to `fixed-here`.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestShape' -count=1 -v > "${TMPDIR:-/tmp}/dt63-shape.log" 2>&1 && go test ./internal/deskkit/ -run '^TestBenignEntriesBounded$' -count=1 -v >> "${TMPDIR:-/tmp}/dt63-shape.log" 2>&1 && grep -F -e '--- PASS: TestShapeGoBuildInfo ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapeDigestKeyArch ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapeK8sQualifiedKey ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapePlusJoinedIdent ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapeMd5StaysRefused ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapeWordSaladCap ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestShapePairedRefusals ' "${TMPDIR:-/tmp}/dt63-shape.log" && grep -F -e '--- PASS: TestBenignEntriesBounded ' "${TMPDIR:-/tmp}/dt63-shape.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt63-shape.log"` | exit 0; every named PASS line and no FAIL: each entry admits its shape, keeps its paired refusals, and widening any entry flips a refusal |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffArmor' -count=1 -v > "${TMPDIR:-/tmp}/dt63-anchor.log" 2>&1 && grep -F -e '--- PASS: TestDiffArmorAnchor ' "${TMPDIR:-/tmp}/dt63-anchor.log" && grep -F -e '--- PASS: TestDiffArmorFailsClosed ' "${TMPDIR:-/tmp}/dt63-anchor.log" && grep -F -e '--- PASS: TestDiffArmorLiteralArms ' "${TMPDIR:-/tmp}/dt63-anchor.log"` | exit 0; three named PASS lines: an armor line in a sanctioned sops file's diff is admitted, an unreadable or unsanctioned file anchors nothing, and a provider token on an anchored line still refuses |
| 3 | check:ci +flow | `test -n "${DELIVERY_BASE:-}" && b=$(mktemp) && trap 'rm -f "$b"' 0 && git show "$DELIVERY_BASE:tools/desk/internal/deskkit/testdata/benign-golden.txt" > "$b" && cd tools/desk && BENIGN_GOLDEN_BASE="$b" go test ./internal/deskkit/ -run '^TestShapeGoldenDelta$' -count=1 -v > "${TMPDIR:-/tmp}/dt63-delta.log" 2>&1 && grep -F -e '--- PASS: TestShapeGoldenDelta ' "${TMPDIR:-/tmp}/dt63-delta.log"` | exit 0; one PASS line: every verdict that changed is named in the delta file by the entry or the cap that changed it, and no corpus input became refused |
| 4 | check:ci +mutation | `O=$(mktemp) && P=$(mktemp) && trap 'rm -f "$O" "$P"' 0 && cd tools/desk && grep -F -e 'word-salad cap removed' internal/deskkit/mutations.json && grep -F -e 'platform suffix set opened' internal/deskkit/mutations.json && go run ./cmd/muhar -j 2 -spec internal/deskkit/mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e ' 0 NOT CAUGHT, 0 could-not-mutate.' "$O" && go run ./cmd/muhar -spec cmd/deskpr/armoranchor-mutations.json > "$P" 2>&1 && grep -F -e 'Totals: 4 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$P"` | exit 0; the two named scan mutations exist, every scan mutation is caught, and all four anchor mutants are caught |
| 5 | check +flow | `test -n "${DELIVERY_BASE:-}" && test -n "${DELIVERY_HEAD:-}" && test -f tools/desk/internal/deskkit/testdata/benign-golden-delta.txt && git diff --quiet --diff-filter=MDR "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/testdata/corpus && a=$(mktemp) && trap 'rm -f "$a"' 0 && git diff --name-only --diff-filter=A "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/testdata/corpus > "$a" && n=$(grep -c -e '/pos-' -e '/neg-' "$a") && [ "$n" -ge 9 ]` | exit 0; no existing fixture was modified, deleted or renamed, and at least nine fixtures were added |
| 6 | check:ci +neighbour | `cd tools/desk && test -f internal/deskkit/testdata/benign-golden-delta.txt && go test ./internal/deskkit/ -count=1 -v > "${TMPDIR:-/tmp}/dt63-all.log" 2>&1 && grep -F -e '--- PASS: TestBodycheckPositives ' "${TMPDIR:-/tmp}/dt63-all.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt63-all.log" && go test ./cmd/deskpr -count=1 && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt63-anch.log" 2>&1 && grep -F -e '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt63-anch.log"` | exit 0; every positive fixture still refuses, the rest of deskkit and deskpr pass, and every mutation anchor lands |
| 7 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-63-registry-acceptance-shapes.md && grep -F -e 'TestShapePairedRefusals' docs/contracts.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/63 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the contract row, the README table and deskpr are in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| An entry admits a credential of the same length in the same context | row 1 (`TestShapePairedRefusals` (planned) and `TestBenignEntriesBounded` (planned)) |
| The build-info entry admits any 44-character run after `h1:` | row 1 (`TestShapeGoBuildInfo` (planned), the no-module bound) |
| The platform suffix becomes a way to hide a credential-stem name | row 1 (`TestShapeDigestKeyArch` (planned)), row 4 (`platform suffix set opened`) |
| The anchor admits lines of a file that is not a sanctioned document, or fails open when the file cannot be read | row 2 (`TestDiffArmorFailsClosed` (planned)), row 4 (anchor mutants) |
| An anchored line hides a provider token | row 2 (`TestDiffArmorLiteralArms` (planned)), row 4 |
| A removed line is anchored against the wrong version of the file (HEAD, or the default-branch tip after main moved) | row 4 (two anchor mutants) |
| The Kubernetes or `+` entry re-admits the ruled leading-acronym case | row 1 (`TestShapeK8sQualifiedKey` (planned), `TestShapePlusJoinedIdent` (planned)) |
| The cap is too tight and starts refusing ordinary text | row 3 (no corpus input flips; every flip is listed), row 6 |
| The cap is too loose and admits word salad | row 1 (`TestShapeWordSaladCap` (planned)), row 4 (`word-salad cap removed`) |
| A verdict changes with no entry to account for it | row 3 |
| An existing fixture is edited so it passes | row 5 |
| The md5 ruling is quietly reversed | row 1 (`TestShapeMd5StaysRefused` (planned)) |
| The closed platform set or a DNS-prefix rule is plausible but admits more than the record needs | No row; review-only: the security reviewer reads each `Doc` and each bound against the issue it cites |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human. Before it leaves `todo`, the design needs an approved `DR-<slug>` record cited by
`design:` (brief-rules 48), and the human answer above must be recorded. The implementation PR
needs a `Security-Review: pass` at its head before any ready flip. The reviewer reads the delta
file line by line. Every Verify row is risk-bearing (the brief answers `sensitive-data: yes`),
so each row is written to fail on the delivery base. The verifier records it with `verifyrun
--fail-first` against the parent of the delivering merge.
