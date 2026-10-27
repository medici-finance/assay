---
brief: assay:assay:apps-installer:10
title: "`deskavatar --tier team` emits a third tile for `<org>-review`, proofed at 20 px against read and act"
why: >-
  The team tier now installs three Apps (apps-installer/09), but the avatar generator still
  makes two tiles, so the review App would land with GitHub's default avatar — and a reviewer
  approval that looks like the act bot's comment at 20 px in a timeline defeats the point of a
  distinct approver. A third tile, proofed against the other two, keeps the three bots telling
  apart where humans actually see them.
wave: 0
depends: []
unblocks: ["apps-installer/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-10-15 by the-desk (house-the-desk-20261015T140050Z)
sources:
  - "apps-installer/09 — the team tier gains `<prefix>-review`, bound to the reviewer role."
  - "./design.md §5 avatars: Read + Act is inversion, not hue; every set is proofed at 20 px; deterministic."
  - "freshness-checked 2026-10-15 @ 3bf2bf99b (origin/main) — `specsFor(org, TierTeam, …)` in tools/desk/internal/avatar/generate.go returns two tiles (`-read`, `-act`, both GlyphStar); the family reviewer tile uses GlyphDisc."
consumers:
  - "tools/desk/internal/avatar/generate.go: fixed-here"
  - "tools/desk/cmd/deskavatar/main.go: fixed-here"
  - "docs/desk-tools/deskavatar.md: fixed-here"
  - "docs/streams/apps-installer/design.md: fixed-here"
version: 1
id: a45786fd-217c-4794-9e43-1391edcd0d95
---

# Brief 10 — Team-tier review avatar

## Context
files:
- `tools/desk/internal/avatar/generate.go` (team case of `specsFor`), its tests.
- `tools/desk/internal/avatar/avatar_test.go` (`TestGenerateTeamNamesAndCount` asserts exactly 2
  team tiles and the names read/act — update it to 3 and add `-review`).
- `tools/desk/internal/avatar/testdata/golden/team-20px.png` (the 20 px strip golden `TestGolden20px` compares against; it gains a third tile and must be regenerated).
- `tools/desk/cmd/deskavatar/main.go` (the `--tier` help text: "team (read+act)").
- `docs/desk-tools/deskavatar.md` (`--tier team` line); `docs/streams/apps-installer/design.md` §5
  (the Read + Act bullet).

facts:
- Team tiles today (generate.go @ 3bf2bf99b): read = dark body `colBodyDark`, accent frame and
  accent GlyphStar; act = accent body, dark frame, dark-ink GlyphStar. Accent is
  `hslToRGB(seedHue(org), 0.72, 0.58)`.
- The proof (proof.go): every pair is rendered at 20 px; a pair fails when mean-colour ΔE (CIE76)
  is below 25 AND silhouette IoU is above 0.6. A failing set writes nothing and exits 5.
- The family reviewer glyph is GlyphDisc. GlyphRing is a separate fixed glyph, assigned to the
  family intake-loop role (`familyRoles` in generate.go); it is not a collision fallback in the
  code today, so using it here is this brief's own choice.
- Two existing tests break when a third tile is added, by design: `TestGenerateTeamNamesAndCount`
  (count and names) and `TestGolden20px` (the team strip golden). Both are updated here, never
  skipped or deleted.
- Output files are `<org>-review.svg` / `.png` beside the existing two.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never lower the proof thresholds to make a set pass.

## Task
1. Add a third team tile, `App: org + "-review"`, in the adopter's hue, with the reviewer's
   family glyph (GlyphDisc). Choose body/frame/glyph colours so the set passes the proof against
   both read and act; if no colouring with GlyphDisc passes, use GlyphRing, and say which in the
   commit message.
2. The read and act tiles stay byte-identical (same org → same bytes as before this change).
   Capture their SVG goldens from origin/main BEFORE editing generate.go.
3. Update `TestGenerateTeamNamesAndCount` to three tiles. Regenerate `team-20px.png` with the
   package's golden-update path and attach the before/after strip images to the PR description, so
   the reviewer judges the new golden by eye rather than trusting the regeneration.
4. Update the help text, deskavatar.md and design.md §5 to describe three team tiles.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && rm -rf /tmp/av10 && /tmp/deskavatar --org example --tier team --out /tmp/av10 --sizes 200 && ls /tmp/av10 \| grep -cE -e '^example-read\.' -e '^example-act\.' -e '^example-review\.'` | 6 | check +flow |
| 2 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeam.*Proof' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeam.*Proof'` | ≥ 1 — the three-tile team set passes the 20 px proof for org logins `example`, `acme`, `a`, and a 39-character login | check |
| 3 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeamReadActUnchanged' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeamReadActUnchanged'` | 1 — the read and act SVG bytes for org `example` equal golden files captured from origin/main before this change | check +dereference |
| 4 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofRejects' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestProofRejects'` | ≥ 1 — a forced review tile identical to read (same glyph and colours) makes `Generate` return a `*ProofError` naming both Apps | check +mutation |
| 5 | `cd tools/desk && go test ./internal/avatar/ -count=1` | exit 0 — the whole package, including the updated `TestGenerateTeamNamesAndCount` and `TestGolden20px` against the regenerated team strip | check |
| 6 | `statusgen --root . --consumers --brief apps-installer/10` | exit 0 — routing claims corroborated against the diff | check |

## Evidence
<!-- appended at implementation time: one row per Verify item -->

### 2026-10-21 independent verification (fresh verifier, admitted run)

Observed source: 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (merged main, detached home). Delivery: squash commit f52610f4e036de6b8e01e16c6f7aadde168b5b25 on the first-parent chain, single parent 59246f0b7a95c5ac600ef209d6731fd378125e6e. Dispatcher admission check passed before any row ran (Verification-Attestation run 968ae11e4bc4a988b79eeb0846106dab27761570f10095f226893c7d60f4ca98, brief sha256 760e5af3d376b3ed0795893f5d415e3b08874f591205a97f4f43d200d90b8fa2). Toolchain: go1.27.2 darwin/arm64, git 2.56.0.

Verification-Table: delivery=f52610f4e036de6b8e01e16c6f7aadde168b5b25 base=59246f0b7a95c5ac600ef209d6731fd378125e6e original=59246f0b7a95c5ac600ef209d6731fd378125e6e

Table re-derivation: 6 rows, 0 author-added. The delivery changed only row 4's Class (check to check +mutation); every Command and Expect is byte-identical to the base. The Verify section is unchanged between the delivery and the observed source. The proof thresholds file is untouched by the delivery.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && rm -rf /tmp/av10 && /tmp/deskavatar --org example --tier team --out /tmp/av10 --sizes 200 && ls /tmp/av10 \| grep -cE -e '^example-read\.' -e '^example-act\.' -e '^example-review\.'` | 6 | exit 0; printed 6 (example-act, example-read, example-review, each as .svg and .png) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeam.*Proof' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeam.*Proof'` | ≥ 1 | exit 0; printed 2 (TestTeamThreeTileProof and TestTeamProofEveryHue both PASS) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeamReadActUnchanged' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeamReadActUnchanged'` | 1 | exit 0; printed 1 | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofRejects' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestProofRejects'` | ≥ 1 | exit 0; printed 1 (TestProofRejectsReviewLikeRead PASS) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./internal/avatar/ -count=1` | exit 0 | exit 0; ok github.com/medici-finance/assay/tools/desk/internal/avatar 2.516s | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `statusgen --root . --consumers --brief apps-installer/10` | exit 0 | exit 2 (statusgen v1.0.54): COULD-NOT-CHECK, the committed Evidence carried no Verification-Table marker naming the delivery, so no claim was corroborated or disproved. Could-not-run accounting: this row earns no credit at this source. Supplemental probe (no acceptance credit): the same command with --delivery f52610f4e036de6b8e01e16c6f7aadde168b5b25 --base 59246f0b7a95c5ac600ef209d6731fd378125e6e exited 0 with 4 corroborated, 0 disproved, 0 unchecked | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row 6 diagnostic (no acceptance credit): re-running the row 6 command with this block present but uncommitted exited 0 and reported 0 corroborated, 0 disproved, 4 unchecked, because the uncommitted brief edit puts the brief in the working-tree diff; unchecked is not corroborated, so row 6 stays without credit until the marker is committed.

Risk-bearing value enumeration (diff scope: generate.go, main.go and the tests of the delivery; the item is not irreversible and carries full risk metadata, all no, so this is for completeness):

- reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183; review glyph = GlyphDisc @ generate.go:185; review glyph colour colGlyphLite = "#EAF0FF" (pre-existing, compose.go:15, reused). All are cosmetic and reversible by an edit and a redeploy; the proof thresholds (proofDeltaE = 25.0, proofIoU = 0.6, proof.go lines 12 and 13) were not changed by the delivery.

RISK-VALUE: DERIVED — reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183 — a reversible cosmetic tone choice; the brief's acceptance is the unchanged 20 px proof, which rows 2 and 5 show passing across every hue, with thresholds untouched by the delivery.

**VERIFY: BLOCKED** — 5/6 rows pass; row 6 exited 2 COULD-NOT-CHECK at this source because the committed Evidence had no Verification-Table marker. This block supplies that marker; re-run row 6 once it lands.

### 2026-10-22 corrective re-pass (fresh verifier, admitted run; supersedes named statements of the 2026-10-21 section)

Observed source: 05dc9fbc88f2b1b23cd78bebc8634c564327eca4 (merged main, detached home, clean before the run). Dispatcher admission check passed before any row ran and again after the witness was written (Verification-Attestation medici-finance/assay#4019, run 9bffeb5e922f00ada84db069ac0bebb16038e03c6b7aa2680b9736a70b72d387, brief sha256 760e5af3d376b3ed0795893f5d415e3b08874f591205a97f4f43d200d90b8fa2, model claude-opus-5-5, tier strong). Toolchain: go1.27.2 darwin/arm64, git 2.56.0.

Source currency: `git diff --stat 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c..05dc9fbc88f2b1b23cd78bebc8634c564327eca4` over the delivered paths (the changelog fragment, the deskavatar doc, this brief, the stream design note, the deskavatar command and its test, and the whole avatar package) printed nothing: the delivered code and this brief are unchanged since the prior observed source.

Tools: every tool a row invokes was built from the observed source into a scratch directory outside the home, and that directory was put first on PATH for the run, so row 6 ran this tree's statusgen, not an installed release:

```sh
(cd statusgen && go build -o <scratch>/bin/statusgen .)
(cd tools/desk && go build -o <scratch>/bin/deskpathguard ./cmd/deskpathguard)
PATH=<scratch>/bin:$PATH statusgen verifyrun --brief docs/streams/apps-installer/brief-10-team-review-avatar.md
PATH=<scratch>/bin:$PATH statusgen verifyrun --check docs/streams/apps-installer/brief-10-team-review-avatar.md
```

Table re-derivation: `deskpathguard rederive --main 59246f0b7a95c5ac600ef209d6731fd378125e6e` reports 6 rows, 0 author-added; every row's base Command and Expect equal the current table, and only row 4's class differs (base check, current check +mutation). No row is classed check:ci, so the darwin network-off sandbox gap (medici-finance/assay#4010) affects no row here.

Verification-Table: delivery=f52610f4e036de6b8e01e16c6f7aadde168b5b25 base=59246f0b7a95c5ac600ef209d6731fd378125e6e original=59246f0b7a95c5ac600ef209d6731fd378125e6e

Execution witness (written by `statusgen verifyrun` in write mode at the observed source; copied here unchanged):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && rm -rf /tmp/av10 && /tmp/deskavatar --org example --tier team --out /tmp/av10 --sizes 200 && ls /tmp/av10 \| grep -cE -e '^example-read\.' -e '^example-act\.' -e '^example-review\.'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeam.*Proof' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeam.*Proof'` | pass exit=0 | sha256:53c234e5e847 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeamReadActUnchanged' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeamReadActUnchanged'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofRejects' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestProofRejects'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/avatar/ -count=1` | pass exit=0 | sha256:bc3846af1f2a | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief apps-installer/10` | fail exit=2 | sha256:0924b02dd9af | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4019 run=9bffeb5e922f00ada84db069ac0bebb16038e03c6b7aa2680b9736a70b72d387 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/apps-installer/brief-10-team-review-avatar.md model=claude-opus-5-5 tier=strong
`statusgen verifyrun --check` on the witnessed brief: 5 pass, 1 fail, 0 could-not-run/missing (of 6 Verify rows); 0 author-added with no acceptance credit; exit 1.

Output inspection (the witness records hashes only). Rows 1 to 4 end in a count; the sha256 of each expected count followed by a newline equals the recorded hash: row 1 "6" = 06e9d52c1720, row 2 "2" = 53c234e5e847, rows 3 and 4 "1" = 4355a46b19d3. Row 5 prints the package's ok line. Row 6's primary output was reproduced byte for byte in a scratch clone at the same source (combined output sha256 0924b02dd9af, equal to the witness):

```text
statusgen: --consumers: COULD-NOT-CHECK: assay:assay:apps-installer:10 is not in the diff against 05dc9fbc88f2b1b23cd78bebc8634c564327eca4, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved.
```

Row 6 accounting: the witness records `fail exit=2`, but no routing claim was disproved; the instrument could not check, because the only committed marker naming the delivery is in the unmerged Evidence PR. Could-not-run accounting: row 6 earns no acceptance credit at this source. Supplemental probe (no acceptance credit), same scratch clone: the row command with `--delivery f52610f4e036de6b8e01e16c6f7aadde168b5b25 --base 59246f0b7a95c5ac600ef209d6731fd378125e6e` exited 0, 4 corroborated, 0 disproved, 0 unchecked.

Row 6 blocker: medici-finance/assay#1915 (open; post-merge `statusgen --consumers --brief` rows without a pinned delivery cannot pass). The cause matches its 2026-10-20 comment, which records this exact post-merge ordering case: "it exits 2, COULD-NOT-CHECK: 'the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved.'", that `statusgen verifyrun` "records that exit as `fail exit=2`", and that "Once a verifier Evidence block carrying the Verification-Table marker is committed, and origin/main contains that commit, the bare command exits 0". Next actor: once the marker is on main, a fresh verifier re-runs row 6 at the landing commit.

Corrections to the 2026-10-21 section (line numbers in this brief as of that Evidence PR's head a55274932aecb3dc1754e89d46abcd7c30c9b378):

- Lines 97 to 104 (the prose row table): it carried no execution witness. Superseded by the verifyrun witness table above; rows 1 to 5 now carry `pass exit=0` with output hashes, and row 6 carries `fail exit=2`.
- Line 104 (row 6) and line 114 (the VERIFY line): neither named a filed blocker. Superseded: the blocker is medici-finance/assay#1915, quoted above.
- Line 91 (attestation): that run's attestation, medici-finance/assay#3842, records Tier "any" (label dispatched-tier:any), while the Evidence PR is labelled dispatched-tier:strong. This re-pass was dispatched and admitted at tier strong (attestation medici-finance/assay#4019, Tier "strong", label dispatched-tier:strong), and its admission is the one this addendum relies on.

Risk-bearing value enumeration (diff scope: the delivery's generate.go and deskavatar main.go changes; tests and docs carry no runtime constant; the item is not irreversible and carries full risk metadata, all no):

- reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183 (introduced).
- review tile App name = org + "-review" @ tools/desk/internal/avatar/generate.go:185 (introduced; the file name of the third tile).
- review glyph = GlyphDisc @ tools/desk/internal/avatar/generate.go:185 (introduced for the team tier; the family reviewer glyph at generate.go:92).
- review glyph colour colGlyphLite = "#EAF0FF" @ tools/desk/internal/avatar/compose.go:15 (pre-existing, reused).
- proof thresholds proofDeltaE = 25.0 and proofIoU = 0.6 @ tools/desk/internal/avatar/proof.go:12 and :13 (named in the brief; unchanged by the delivery, whose diff does not touch proof.go).

Ranking: all are cosmetic and reversible by an edit and a redeploy; the thresholds rank first, because lowering them would let indistinct tiles through, and they are unchanged. The tone ratio ranks next.

RISK-VALUE: DERIVED — reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183 — a reversible cosmetic tone choice; the brief's acceptance is the unchanged 20 px proof (proofDeltaE = 25.0, proofIoU = 0.6 at proof.go:12-13, untouched by the delivery), which rows 2 and 5 show passing for every hue.

**VERIFY: BLOCKED** — 5/6 rows pass under the verifyrun witness at 05dc9fbc88f2; row 6 is could-not-check (witness fail exit=2, 0 disproved), blocked on medici-finance/assay#1915 until the Verification-Table marker reaches main.
### 2026-10-26 re-pass at the current source (fresh verifier, admitted run)

Observed source: f2e232039c99455b0b9a7dacfe639f9f68b164d2 (merged main, detached home, clean before and after the run). Delivery: squash commit f52610f4e036de6b8e01e16c6f7aadde168b5b25 (#3171) on the first-parent chain, single parent 59246f0b7a95c5ac600ef209d6731fd378125e6e. Dispatcher admission check passed before any row ran (Verification-Attestation #4556, run 31eaba7bee4d0b6f2f289a755419be8631b51bec2f9b49b919b74962fd6cacac, brief sha256 c3b8985408b34a3eea1b7835574ed24afa9c77c8ce57e1946e9d8ec5ba7e16a4, model claude-opus-5-5, tier strong). Toolchain: go1.27.2 darwin/arm64, git 2.56.0, statusgen v1.0.63 (installed release).

Source currency: `git diff --stat 05dc9fbc88f2b1b23cd78bebc8634c564327eca4..f2e232039c99455b0b9a7dacfe639f9f68b164d2` over the delivered code and docs (the avatar package, the deskavatar command, the deskavatar doc) printed nothing; the only changes under the stream directory since the prior observed source are Evidence landings (this brief's 2026-10-22 section, which put the Verification-Table marker on main, and an unrelated brief in the same stream).

Table re-derivation: `deskpathguard rederive --main 59246f0b7a95c5ac600ef209d6731fd378125e6e` reports 6 rows, 0 author-added; every row is from-base with its Command and Expect unchanged. No row is classed check:ci, so the darwin network-off sandbox gap does not apply. No row carries a risk-bearing, live, mutating or end-to-end tag and every brief-wide risk field is no, so `--fail-first` selected no row at base (Base cell `not-selected`).

Verification-Table: delivery=f52610f4e036de6b8e01e16c6f7aadde168b5b25 base=59246f0b7a95c5ac600ef209d6731fd378125e6e original=59246f0b7a95c5ac600ef209d6731fd378125e6e

Execution witness (written by `statusgen verifyrun --fail-first --base 59246f0b7a95c5ac600ef209d6731fd378125e6e` at the observed source; copied here unchanged):

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && rm -rf /tmp/av10 && /tmp/deskavatar --org example --tier team --out /tmp/av10 --sizes 200 && ls /tmp/av10 \| grep -cE -e '^example-read\.' -e '^example-act\.' -e '^example-review\.'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
| 2 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeam.*Proof' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeam.*Proof'` | pass exit=0 | sha256:53c234e5e847 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
| 3 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeamReadActUnchanged' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestTeamReadActUnchanged'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
| 4 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofRejects' -count=1 -v 2>&1 \| grep -c -- '--- PASS: TestProofRejects'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
| 5 | `cd tools/desk && go test ./internal/avatar/ -count=1` | pass exit=0 | sha256:9c01145ff420 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
| 6 | `statusgen --root . --consumers --brief apps-installer/10` | pass exit=0 | sha256:15b833db4d20 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=f2e232039c99 |
Verification-Attestation: medici-finance/assay#4556 run=31eaba7bee4d0b6f2f289a755419be8631b51bec2f9b49b919b74962fd6cacac source=f2e232039c99455b0b9a7dacfe639f9f68b164d2 brief=docs/streams/apps-installer/brief-10-team-review-avatar.md model=claude-opus-5-5 tier=strong
`statusgen verifyrun --check` on the witnessed brief: docs/streams/apps-installer/brief-10-team-review-avatar.md: 6 pass, 0 fail, 0 could-not-run/missing (of 6 Verify rows); 0 author-added with no acceptance credit; exit 0.

Output inspection (the witness records hashes only). Rows 1 to 4 end in a count; the sha256 of each expected count followed by a newline equals the recorded hash: row 1 "6" = 06e9d52c1720 (example-read, example-act and example-review, each as .svg and .png), row 2 "2" = 53c234e5e847 (TestTeamThreeTileProof and TestTeamProofEveryHue both PASS; the first covers the logins example, acme, a and a 39-character login, asserting the length), rows 3 and 4 "1" = 4355a46b19d3 (TestTeamReadActUnchanged; TestProofRejectsReviewLikeRead). Row 5 printed `ok github.com/medici-finance/assay/tools/desk/internal/avatar`. Row 6, re-run at the same source, exited 0 and printed:

```text
consumers corroboration — base 59246f0b7a95c5ac600ef209d6731fd378125e6e..f52610f4e036de6b8e01e16c6f7aadde168b5b25 (immutable delivered snapshot), 1 brief(s)
  CORROBORATED  tools/desk/internal/avatar/generate.go: fixed-here
  CORROBORATED  tools/desk/cmd/deskavatar/main.go: fixed-here
  CORROBORATED  docs/desk-tools/deskavatar.md: fixed-here
  CORROBORATED  docs/streams/apps-installer/design.md: fixed-here
summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing
```

Row 6 resolution: the 2026-10-22 section recorded row 6 as could-not-check because no committed Verification-Table marker named the delivery (the case tracked in #1915). That marker is now on main, the bare row command derives the delivered snapshot from it, and row 6 passes with every routing claim corroborated and none disproved.

Supplemental dereference of row 3 (no acceptance credit): the deskavatar command built from the delivery's parent 59246f0b7a95c5ac600ef209d6731fd378125e6e, run with `--org example --tier team --sizes 20`, wrote example-read.svg and example-act.svg byte-identical (cmp) to the committed goldens team-example-read.svg and team-example-act.svg, so the goldens row 3 compares against really are the pre-change bytes.

Risk-bearing value enumeration (diff scope: the delivery's generate.go and deskavatar main.go changes, plus the literals the brief names; tests and docs carry no runtime constant; the item is not irreversible and carries full risk metadata, all no):

- reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183 (introduced).
- review tile App name = org + "-review" @ tools/desk/internal/avatar/generate.go:185 (introduced; the third tile's file name).
- review glyph = GlyphDisc @ tools/desk/internal/avatar/generate.go:185 (introduced for the team tier).
- review glyph colour colGlyphLite = "#EAF0FF" @ tools/desk/internal/avatar/compose.go:15 (pre-existing, reused).
- proof thresholds proofDeltaE = 25.0 and proofIoU = 0.6 @ tools/desk/internal/avatar/proof.go:12 and :13 (named in the brief; the delivery does not touch proof.go).
- tier help text "team (read+act+review)" @ tools/desk/cmd/deskavatar/main.go (string only, no behaviour).

Ranking: all are cosmetic and reversible by an edit and a redeploy. The proof thresholds rank first, because lowering them would let indistinct tiles through; they are unchanged by the delivery. The tone ratio ranks next.

RISK-VALUE: DERIVED — reviewBody mix ratio = 0.55 @ tools/desk/internal/avatar/generate.go:183 — a reversible cosmetic tone choice that places the review body between read's dark body and act's accent body; the brief's acceptance is the unchanged 20 px proof (proofDeltaE = 25.0, proofIoU = 0.6 at proof.go:12-13, untouched by the delivery), which rows 2 and 5 show passing for every one of the 360 hues.

**VERIFY: PASS** — 6/6 rows pass under the verifyrun witness at f2e232039c99; row 6, blocked in the 2026-10-22 section, now passes with 4 corroborated and 0 disproved.

### 2026-10-27 closure: BLOCKED (check-definition)

The verified closure was refused by the closure lint: Verify row 6 [consumer-delivery-range] names no immutable delivery snapshot (no pinned `--delivery` and first-parent `--base`), so it cannot be the basis of a closure. The row-status flip was reverted to `implemented`. Every other row's witness above stands. The amendment of row 6 is tracked in #4560; it follows the protected-amendment path. The supplemental probe earns no row credit.

**VERIFY: BLOCKED**: row 6, check-definition, #4560.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
