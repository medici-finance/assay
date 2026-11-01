# Witness readers — who reads a Runner cell, a witness record, a verify-outcome record or a claim revision

Maintained for statusgen/36. `TestWitnessReaderCensus` (in `statusgen/`) walks every non-test Go file under `statusgen/` and `tools/desk/`, and fails when a file matches the census pattern but is not listed here, when a listed file does not exist or no longer matches, or when a disposition is not one of the three below.

Dispositions:

- `judges through the library` — the file is `statusgen/evaluation`, or hands the cell and the record reader to it and takes its verdict.
- `reads informational fields only` — reads an identity token, a Result or state cell, a path, or a count; never a revision, a tree token, a record id or a staleness verdict. A file that only mentions the words (a comment, an unrelated `Runner`) is listed here with that noted.
- `migrates in statusgen/37` — writes or interprets a cell or record in a way the v2 writer changes; until statusgen/37 lands it either fails closed on a v2 cell (noted) or is untouched by it.

The census pattern (kept in sync with `witnessCensusPattern` in `statusgen/witnessreaders_test.go`) is: a Runner cell (`Runner cell`, `.Runner`, `witnessCells`, `witnessTreeOf`, `ParseRunnerCell`), a witness record (`ParseWitnessRecord`, `WitnessRecord`, `witness_schema_version`), a verify-outcome record (`verify-outcome`, `OutcomeRecord`, `outcome-record`) or a claim's revision (`Claim.Revision`, `coverageClaimJSON`, `evaluation.Claim`).

| File | Disposition | Note |
|---|---|---|
| `statusgen/acceptancetransition.go` | migrates in statusgen/37 | reads the tree token of a Runner cell as a commit and refuses a cell whose token is not an object id, so a v2 cell fails closed until statusgen/37 migrates it |
| `statusgen/attribution.go` | reads informational fields only | identity token and Result cell for attribution and the verifier floor; takes the whole token, never splits on the marker |
| `statusgen/autoflip.go` | judges through the library | consumes the library verdict (`--coverage` result) to gate an automatic flip; reads no cell |
| `statusgen/awaiting_bucket.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record; removes a read-set token before its could-not-check test, so the token never unsettles a run row |
| `statusgen/briefevidence.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/citationcorroborate.go` | reads informational fields only | identity token and Result cell for attribution and the verifier floor; takes the whole token, never splits on the marker |
| `statusgen/containerbracket.go` | migrates in statusgen/37 | bracket of container-run witness cells carries the v1 tree token; the v2 form is written by statusgen/37 |
| `statusgen/coverage.go` | judges through the library | hands the Runner cell and the record reader to the library; judges nothing itself |
| `statusgen/coverage_json.go` | judges through the library | hands the Runner cell and the record reader to the library; judges nothing itself |
| `statusgen/coverage_witness.go` | judges through the library | hands the Runner cell and the record reader to the library; judges nothing itself |
| `statusgen/deliveryfiles.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/evaluation/coverage.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evaluation/coverage_api.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evaluation/declared_api.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evaluation/identity.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evaluation/readset.go` | judges through the library | the library itself: read-set token and file grammar, the compile-input floor rule and the read-set judgement (build-less-brittle/15) |
| `statusgen/evaluation/witness.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evaluation/witness_json.go` | judges through the library | the library itself: cell parser, record parser, digest, judgement |
| `statusgen/evidenceactor.go` | reads informational fields only | identity token and Result cell for attribution and the verifier floor; takes the whole token, never splits on the marker |
| `statusgen/evidenceshape.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/failfirst.go` | reads informational fields only | reads the Result, Command or state cells of a witness row through witnessCells, never the revision or marker |
| `statusgen/briefdoc/registers.go` | reads informational fields only | shared reserved-register name declaration, moved from load.go; reads no record, cell or revision and performs no file I/O |
| `statusgen/main.go` | judges through the library | hands the Runner cell and the record reader to the library; judges nothing itself |
| `statusgen/outcomessplit.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/principal.go` | reads informational fields only | renders or round-trips a Runner cell without interpreting the marker |
| `statusgen/protectedclosure.go` | reads informational fields only | reads the Result, Command or state cells of a witness row through witnessCells, never the revision or marker |
| `statusgen/readset_adapter.go` | judges through the library | reads a witness's read-set file and its introducing commit, exports the witness tree for the floor's `go list`, and hands the library the files it lists or why it could not list them; the library decides the claim |
| `statusgen/readset_platform.go` | migrates in statusgen/37 | writes the v1 Runner cell's read-set token (the could-not-check form off Linux) through the library's token writer |
| `statusgen/reconcilefold.go` | reads informational fields only | renders or round-trips a Runner cell without interpreting the marker |
| `statusgen/unrun.go` | reads informational fields only | reads the Result, Command or state cells of a witness row through witnessCells, never the revision or marker |
| `statusgen/verifiedrunneragree.go` | reads informational fields only | identity token and Result cell for attribution and the verifier floor; takes the whole token, never splits on the marker |
| `statusgen/verifyclosure.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/verifyflip.go` | migrates in statusgen/37 | matches `<login> @ <hex sha>` with its own pattern on a verdict run; a v2 marker is not hex, so a v2 cell fails closed until statusgen/37 migrates it |
| `statusgen/verifyissues.go` | reads informational fields only | reads the Result, Command or state cells of a witness row through witnessCells, never the revision or marker |
| `statusgen/verifyoutcomes.go` | reads informational fields only | classifies a path as a verify-outcome record or board bookkeeping; reads no field of a record |
| `statusgen/verifyrun.go` | migrates in statusgen/37 | writes the v1 Runner cell and Evidence row; the v2 writer is statusgen/37 (reader side only here: witnessCells and witnessTreeOf hand the cell to the library) |
| `statusgen/verifysandbox.go` | reads informational fields only | reads the Result, Command or state cells of a witness row through witnessCells, never the revision or marker |
| `tools/desk/cmd/deskdispatch/dispatch.go` | migrates in statusgen/37 | dispatch packet and outcome-record path guidance for the verifier; migrates with the record in statusgen/37 |
| `tools/desk/cmd/deskdispatch/outcomepath.go` | migrates in statusgen/37 | dispatch packet and outcome-record path guidance for the verifier; migrates with the record in statusgen/37 |
| `tools/desk/cmd/deskdispatch/prompt.go` | migrates in statusgen/37 | dispatch packet and outcome-record path guidance for the verifier; migrates with the record in statusgen/37 |
| `tools/desk/cmd/deskevidence/decisiongate.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/deskevidence.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/landed.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/origin.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/outcome.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/outcomebinding.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/outcomejoin.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/outcomerecord.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/plainread.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/principalstamp.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/readset.go` | reads informational fields only | reads the read-set digest a Runner cell names and the local file at that path, and refuses the landing before any forge call; never a revision, a tree token or a staleness verdict (build-less-brittle/15) |
| `tools/desk/cmd/deskevidence/receiptvalidate.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/rowattribution.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/deskevidence/verifiedgate.go` | migrates in statusgen/37 | verify-outcome record writer and landing gate; adopts the witness reference in statusgen/37 |
| `tools/desk/cmd/fanoutloop/repair.go` | reads informational fields only | mentions a verify-outcomes sidecar in a repair marker; reads no cell, record or revision |
| `tools/desk/cmd/verifyloop/briefscan.go` | migrates in statusgen/37 | reads verify-outcome records and Evidence rows to queue verification; migrates with the record in statusgen/37 |
| `tools/desk/cmd/verifyloop/dispatch.go` | migrates in statusgen/37 | reads verify-outcome records and Evidence rows to queue verification; migrates with the record in statusgen/37 |
| `tools/desk/cmd/verifyloop/pending.go` | migrates in statusgen/37 | reads verify-outcome records and Evidence rows to queue verification; migrates with the record in statusgen/37 |
| `tools/desk/cmd/verifyloop/verdictpayload.go` | migrates in statusgen/37 | reads verify-outcome records and Evidence rows to queue verification; migrates with the record in statusgen/37 |
| `tools/desk/internal/celllaunch/session.go` | reads informational fields only | matches on the word Runner for a dispatch claim owner or a launch path; not a witness Runner cell |
| `tools/desk/internal/deskkit/claim.go` | reads informational fields only | matches on the word Runner for a dispatch claim owner or a launch path; not a witness Runner cell |
| `tools/desk/internal/deskkit/principal.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/deskkit/repairobligation.go` | reads informational fields only | mentions a verify-outcomes sidecar in a repair marker; reads no cell, record or revision |
| `tools/desk/internal/deskkit/roleworktreeidentity.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/deskkit/verifierattestation.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/deskkit/verifyoutcomejoin.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/deskkit/verifyoutcomes.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/deskkit/verifywake.go` | migrates in statusgen/37 | verify-outcome record layer and join keys; adopts the witness reference in statusgen/37 |
| `tools/desk/internal/loopengine/claim.go` | reads informational fields only | matches on the word Runner for a dispatch claim owner or a launch path; not a witness Runner cell |
