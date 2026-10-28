# Audited board cohort

This fixture binds the ten changed rows of [PR 3200](https://github.com/medici-finance/assay/pull/3200)
at head `a1d5037539688ef46cf2dbc8be380a40ecfa1d35`, merge base
`0d6a1bb5bc835c2fe80d0b23da53faba23964abf`. Membership comes from the
zero-context tree diff plus exact brief/witness excerpts from its body. Both full
review tables independently corroborate those identities:
[correctness 5592762972](https://github.com/medici-finance/assay/pull/3200#pullrequestreview-5592762972)
and [security 5592834194](https://github.com/medici-finance/assay/pull/3200#pullrequestreview-5592834194).
Their full bodies were read at pickup, not inferred from their verdicts.

`board-cohort.json` keeps membership, expected dispositions, and primary evidence
in separate fields. The test joins identities before accepting dispositions;
missing, extra, duplicated or mismatched identities fail. It does not classify
prose. Every cited delivery body, forge file list, local merge first-parent file
set and current brief obligations was read at source
`5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7`. Each recorded file list equals its local
merge set. Full body hashes identify the source bodies; only the necessary exact
scope excerpts and matcher lines are published. `body_lines` are original
one-based line locations. `brief_sha256` binds the entire source brief before this
repair; `obligations` retains its Task/Definition-of-Done blocks. `delivery_repair`
contains exact statusgen/26 before/after blobs from base `5dcdff9fb` and delivery
`92315c7fa`; the byte check permits only the declared entry and version increment
between those immutable snapshots, preserving their Task/Verify/Evidence bytes.
It does not hash or freeze the living brief. Current declarations still pass
through the real readers; a positive control permits later comment, version,
Evidence and additional-partial maintenance without changing #3759's meaning.

The existing ten-row review dispositions are independently retained here:

| Brief | Actual delivery | Scope and remaining boundary |
|---|---|---|
| apps-installer/10 | [3171](https://github.com/medici-finance/assay/pull/3171) | Reviewed installer delivery |
| brief-quality/01 | [1719](https://github.com/medici-finance/assay/pull/1719) | Full design delivery; its follow-on implementation is separate |
| contributor-trust/09 | [980](https://github.com/medici-finance/assay/pull/980) | Reviewed public intake delivery |
| desk-containers/06 | [3729](https://github.com/medici-finance/assay/pull/3729) | Source/offline checker only; no deployed runtime claim |
| desk-tools/27 | [1196](https://github.com/medici-finance/assay/pull/1196) | Completes the missing check/hint slice after 1062; VERIFY FAIL remains separate |
| desk-tools/37 | [3331](https://github.com/medici-finance/assay/pull/3331) | Bounded test/oracle delivery; later failed verification is retained |
| graph-execution/28 | [3728](https://github.com/medici-finance/assay/pull/3728) | Source implementation; independent Verify still owed |
| graph-execution/32 | [3307](https://github.com/medici-finance/assay/pull/3307) | Bounded corpus/report, no production mechanism |
| measured-status/01 | [1726](https://github.com/medici-finance/assay/pull/1726) | Reviewed scoped delivery |
| statusgen/26 | [3759](https://github.com/medici-finance/assay/pull/3759) | Partial sub-split (a); Tasks 3/4 and Verify 2 remain owed |

## The positive full witness is real

[1062](https://github.com/medici-finance/assay/pull/1062), merge
`ea99e24abe001f24b9b3c7ffe6dc4934a0c088ba`, delivered the pooled loops, shared
root resolution, throughput derivation, pin resolution, gate ordering, help
handling and deskpost hints. Its actual body explicitly leaves two deliverables
owed. [1196](https://github.com/medici-finance/assay/pull/1196), merge
`a4700d2bb73a9c048c69eb6be0b2e95c5b1703e2`, completes `deskpr --check` and the
remaining refusal hints. The recorded ten actual paths include `deskpr`'s
create/update/edit gates and `offlinehint_test.go`, plus `deskreply`'s dry-run
and hint paths. Source inspection confirmed the check returns before token
minting and the reply rehearsal returns before posting; the existing
`offlinehint_test.go` exercises the refusal contracts. The brief's recorded DoD
and both actual merge sets provide the comparison, not the generic
`deliveryFiles` test stub. No historical timing or test result is re-certified.

1062 is retained as comparison evidence (`matcher_input: false`), not silently
reclassified as another audited row. This fixed cohort is not a census of all
prior desk-tools/27 deliveries. 1196 is the audited full witness. The separate
real [2295](https://github.com/medici-finance/assay/pull/2295) identity is combined
with the existing exact-path/patch `evidence2295.json` seam: it must be excluded
while 1196 survives a later synthetic partial and VERIFY FAIL.

The other members are also bounded projections, not complete candidate histories.
The full pickup collection's merged trailers include contributor-trust/09
[980, 949, 941], desk-tools/27 [2295, 1196, 1062, 1052], and desk-tools/37
[3768, 3331]. This fixture omits 949, 941, 1052 and 3768; 1062 is comparison-only
and 2295 is a separate Evidence control. The coverage join certifies the fixed
board pairs and supplied controls, not those members' full histories. This
limitation is separate from the complete statusgen/26 projection below.

## Current eligibility and collection receipt

The pickup REST collection walked every `state=all` page to exhaustion: 2,020
pulls, highest PR 3805, completed by `2026-10-21T00:08:42Z`, no unread page.
Its private raw receipt hash and the frozen cutoff are in `collection`. This is
our own pickup collection; no sibling proof's corpus or scan log was used.
The complete input and this bounded public projection produced identical
statusgen/26 results through **both existing owners**:

- `prRecordsFromPulls`: merged trailer 3759 only.
- `backfillCandidates`: potential body/branch matches 3803, 3794, 3759, 3745,
  3457, 3449, 3435, 3431, 3190, in descending order.

For those records the fixture preserves number, state, draft, merged time,
merge SHA, head SHA/ref, every actual `Brief:` line and every body line containing
`statusgen/26` or `statusgen-26`. It also keeps 3745's actual `Issue: #3109` line.
These excerpts preserve the owners' inputs; they invent no trailers or prose.
The source-bound declaration makes 3759 in-progress. `backfillBase` then refuses
fallback for that cell: the nine potential body matches are **not eligible
current witnesses**. In particular 3745 remains an interim issue-only step.
No declaration for a hypothetical fallback state is inferred. A changed cutoff
or newly eligible unknown scope requires a fresh receipt and HOLD, not a default
full disposition. Tests of legitimate historical fallback use the existing
separate fixture and never strip 3759's real trailer.

## Reproduce and mutate

From the repository root:

```sh
(cd statusgen && go test . -run '^TestBoardCohort' -count=1 -timeout 60s -v)
python3 statusgen/testdata/witness-admission/mutate-board-cohort.test.py
```

Run the mutation script in a disposable owned checkout. It records a green
baseline, runs every cohort test against temporary live-source comment/version/
Evidence/additional-partial maintenance, then removes only the source entry,
observes both the named
coverage failure and the real false implemented witness, and restores exact
source bytes. The second-site plant is synthetic apps-installer/10 PR 97, older
than the reviewed witnesses and absent from both dispositions and declarations.
Disabling the test-only coverage join must make its control fail. Go overlays
also disable the existing write-refusal and Evidence-exclusion boundaries to
prove those controls discriminate. They never edit production Go bytes. The
script ends with a restored green run and byte-identity checks. Private raw logs
stay outside the repository; the script prints their location. Child Go tests
receive an explicit build/runtime environment allow-list, with Go environment-file
loading and downloads disabled; forge-token variables and arbitrary caller Go
flags are not forwarded. Local fake-server listeners remain part of the tests.

`cohortRoot` copies the real statusgen/26 source and uses the existing minimal
model-gate template for the other rows. The latter avoids unrelated evolving
lint/Verify obligations; it is not semantic evidence for those deliveries. Their
actual obligations remain separately recorded and independently reviewed. File
and shape readers use existing hermetic seams. Invalid fixture provenance stops
the test evidence join; it is not a new runtime provenance gate. Production
unread/malformed metadata floors and zero writes run separately with coverage
bypassed. All nine full cells must actually be applied; zero work cannot pass.

## Limits retained

Correctness's historical reconcile attempt timed out with lookedAt=false and
could-not-check. Security separately reproduced the ten actual pairs with
lookedAt=true, no unread input, within 74 wouldApply / 57 held / 525 cells /
ten unknown cells. Both outcomes remain, with their distinct provenance.
This source-only proof neither certifies another historical candidate nor clears
the wider 34/87 audit. Issue 2703 stays open. After source review and human merge,
only the normal producer regenerates the board; fresh trust, independent full
reviews and human board merge precede independent graph-execution/28 Verify
and graph-execution/29. Kubernetes, Linux deployment and Windows remain
unqualified. Worker tests are not independent verification.
