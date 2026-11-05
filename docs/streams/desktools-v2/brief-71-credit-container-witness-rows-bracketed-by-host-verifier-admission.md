---
brief: assay:assay:desktools-v2:71
title: Credit container witness rows bracketed by host verifier admission
why: "A Linux container is the only Linux runner many verifiers have, but container-sandboxed rows either run unattested or attempt admission where the forge is unreachable, so reviewers reject them."
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "Changes which runs count as admitted independent-verification witnesses (proposal 11 independent verification; proposal 13 verifier transaction step 3). Human acceptance of the bracket rule is required at done."
decision-issue: 4071
ruling: "https://github.com/medici-finance/assay/issues/4071#issuecomment-6349598734"
design: DR-container-witness
issues: []
schema: brief-v2
version: 2
authored: 2026-10-22 by statusgen newbrief
sources:
  - 'desktools-v2: Credit container witness rows bracketed by host verifier admission'
  - "driver ruling, 2026-10-22, for ratification at merge (its public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22): Linux container runs count as Linux witnesses when the host runs verifier admission immediately before and after the container run; admission inside a network-less container is the refused shape"
  - docs/brief-rules.md (rule 45)
  - docs/proposals/non-ci-architecture/13-migration-and-decisions.md (§The verifier transaction, end to end, step 3)
  - docs/proposals/non-ci-architecture/29-extensible-json-contracts.md (worker/verifier attestations row)
  - freshness-checked 2026-10-22 @ 9a2c6ba2bc85558d77c4227984d93a728a7f7e94 (origin/main)
exec-tier: strong
exec-tier-why: "Admission and witness credit are an independent-verification boundary; the fail-closed path and the bracket binding need adversarial review."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go, statusgen/verifieradmission.go, statusgen/verifysandbox.go: follow-up desktools-v2/71"
  - "docs/brief-rules.md rule 45: follow-up desktools-v2/71"
  - "plugins/assay/skills/verify-desk/SKILL.md and plugins/assay/skills/pr-review-desk/SKILL.md, plus the generated Cursor mirror: follow-up desktools-v2/71"
  - "tools/desk/cmd/deskevidence/ landing test of a bracketed witness: follow-up desktools-v2/71"
  - "cross-repository (version 2) admission receipts in container mode: out-of-scope (refused here; a later brief may admit them)"
  - "container image build, runtime selection and an adopter's runner scripts: out-of-scope (adopter-owned)"
---

# Brief 71 — Credit container witness rows bracketed by host verifier admission

## Context

files: statusgen/verifyrun.go, statusgen/verifyrun_test.go, statusgen/verifieradmission.go, statusgen/verifieradmission_test.go, statusgen/verifysandbox.go, statusgen/verifysandbox_test.go, docs/brief-rules.md, plugins/assay/skills/verify-desk/SKILL.md, plugins/assay/skills/pr-review-desk/SKILL.md, tools/desk/cmd/deskevidence/, tools/desk/internal/deskkit/verifierattestation.go, statusgen/verifyflip.go, statusgen/evidenceactor.go, tools/desk/cmd/deskread/main.go, tools/desk/internal/deskkit/ — docs/brief-rules.md is rule 45; tools/desk/cmd/deskevidence/ is test only; verifierattestation.go is read only; verifyflip.go and evidenceactor.go are read only (the per-line history read Task 8 reuses); deskread's main.go and its tests gain one additive field on the `trust` kind (Task 8); tools/desk/internal/deskkit/ gains one new test file (row 11; no code change).

facts:
- Rule 45 (`docs/brief-rules.md:639`) admits `--sandbox=container-netns` as an explicit opt-in for a runner already inside a `--network none` container (`:649`-`:661`). Every sandboxed row records `sandbox=unshare` or `sandbox=container-netns` in its witness Result cell. `witnessSandboxOf` reads it (`statusgen/verifysandbox.go:148`).
- `sandboxRefusal` (`verifysandbox.go:124`) refuses container-netns only in the CI lane (`ciLane`, `:118`) and with `--in-container`. It never looks at admission. `verifyrun` calls it first (`statusgen/verifyrun.go:1805`).
- Admission is conditional. `verifierAdmission` (`statusgen/verifieradmission.go:18`) requires it only when `DESK_LOOP` contains `verify` (`:19`) or `git rev-parse --git-path assay-verifier-attestation.json` names an existing file (`:20`-`:25`). Otherwise it returns no attestation and the run proceeds unattested (`:27`-`:28`). When it is required it runs `deskdispatch --check-verifier` in-process (`:30`). Inside a network-less container that is the refused shape the ruling names, because the check cannot reach the forge. When the container does not mount a linked worktree's git directory, the `git rev-parse` probe fails and an undeclared run is silently unattested.
- `verifierAdmissionReceipt` (`verifieradmission.go:40`) already parses the check's JSON output with no network. Version 1 yields `Verification-Attestation: <repo>#<issue> run=… source=… brief=… model=… tier=…`. Version 2 (cross-repository) needs absolute `BriefRoot` and `SourceHome` paths on the host (`:57`-`:61`). `verifyrun` appends the attestation line under the witness table (`verifyrun.go:2022`-`:2023`) and skips admission under `--check` (`:1828`).
- The landing path rewrites witness bytes. `deskevidence` runs `stampEvidencePrincipal` on every landing (`tools/desk/cmd/deskevidence/deskevidence.go:445`, `:459`), which inserts `(on-behalf-of human:<subject>)` into the Runner cell of each added App row that has none, placed immediately before any trailing runner-source tag such as `(git-config)` (`principalstamp.go:78`, `:152`-`:156`; `needsPrincipalStamp` at `:171`). `verifyrun` renders that annotation itself only when a principal resolves (`statusgen/verifyrun.go:236`), which a network-less container without the host roster usually cannot do. The annotation is always a parenthetical, so `runnerKey` (`statusgen/verifiedrunneragree.go:74`) already ignores it for runner comparison. Brief 73 widens the rewrite to restamping planted annotations.
- verify-desk §Pre-work admission (`SKILL.md:128`-`:164`) requires the check before any row and says verifier output files belong outside the admitted worktree, since any extra file, ignored files included, refuses admission. Lines `:373`-`:378` make verifyrun the execution witness. pr-review-desk's SKILL has no container, sandbox or Linux-witness text, so a reviewer has no rule that credits a container row.

Design-fit: proposal 11 makes independent verification "a disposable runner without
publisher credentials … and a separately authorized publisher"
(`11-target-architecture.md:281`). Proposal 13 step 3 runs the work "in a disposable
sandbox with explicit inputs". A network-less container fits both, provided admission
stays with the host, which holds the forge view. The ruling moves admission to the host
on both sides of the run. Inside the container, `verifyrun` reads the host's receipt as
an explicit input and runs no admission of its own. Proposal 29's worker/verifier
attestations row keeps "a closed authority core … and explicit required-proof
capabilities". The bracket binds three things the host can check: the admission
(receipt before, check-verifier after), the bytes the container ran (a tree digest
computed on the host at the open, by the container before any row, and on the host
again at the close) and the witness it closes (a hash of the witness on the bracket
line). The container bracket is therefore a required-proof capability on the
attestation: a reader that does not support it refuses to credit the row. It is not an
optional perf-v4 extension that a reader may skip. The verifier attestation format in
deskkit is unchanged. No new verb is added: the credit check of Task 8 is a read-only flag
on `verifyrun --bracket-digest`, and its forge reads (the record, its body history and its
label history) go through `deskread`, the read verb statusgen already reaches the forge
through. The one change to `deskread` is an additive field on its existing `trust` kind
(Task 8(b)); it adds no kind and no forge operation.

single-point-of-failure: the host close step's three-way tree-digest match (host at open, container before its rows, host at close), which binds the bytes the container ran to the admitted home — backed by the host's post-run admission check (the home and the binding did not move), the image digest that pins the `verifyrun` binary computing the in-container digest, and the landing-phase check `deskevidence` already repeats at `--root`. On the credit side, the bracket's digest is keyless and is computed from the brief's own bytes, so a match shows only that the bracket and its span agree with each other. Credit rests on Task 8: the dispatcher-owned attestation record, read under `ReadVerifierAttestation`'s rules, which a brief editor cannot write; the brief and source binding; and per-line provenance, which reads the commit author git records. That provenance is tamper-evident, not tamper-proof: whoever can commit to the brief can write the verifier's author address. The check turns such a forgery into a named, diffable act in commit metadata and push events, but it does not prevent it. Server-side rules on who may push to the brief's branch are what close that residual, and they are not part of this brief.

## Human decision

For ratification at merge: driver ruling, 2026-10-22, as in sources; its public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22. Still for acceptance at done: the
bracket form below and the image-digest requirement. Recommended: require the image
digest on every credited container row. A run whose digest cannot be read is
could-not-run, not a pass. Alternative: make the digest advisory. That is easier to adopt,
but then a reviewer cannot tell two container images apart after the run. The image digest
also matters to the tree binding: the in-container tree digest is computed by the
`verifyrun` binary in that image, so an unpinned image could report a digest it did not
compute.

Default if no answer: the recommended shape (host receipt plus host close-step bracket; image digest required on every credited container row) after 2026-10-24.

The default above fixes only the design the implementer builds to at dispatch, including
the tree binding of Tasks 1, 3 and 6 (over the receipt `tree` that Task 2 records). It never substitutes for the `gate: human` acceptance at done,
which still decides whether the bracket rule credits container rows.

## Ground rules

Normal worker admission, one isolated worktree and one draft PR. Tests use fixtures and
fakes. No container engine, network or forge access is needed to run them. Do not relax
`sandboxRefusal`, the loopback precheck, the CI-lane refusal or any admission refusal.
Stop at implemented.

## Task

1. Tree digest. Define one function, used by every step below: sha256 over the sorted
   list of `<path> NUL <mode> NUL <sha256 of content or link target> NUL` for every regular
   file and symlink under the root, excluding the top-level `.git` entry. Admission already
   refuses any untracked or ignored file in the home, so on an admitted home this set is
   exactly the tracked tree at `source`. Rendered as `sha256:<64 hex>`.
   Witness digest. Define a second function, exposed read-only as `statusgen verifyrun
   --bracket-digest --witness <file>`, used by the close step and by the credit rule.
   A witness table header is a line equal, after trimming, to the first line of
   `witnessHeader` (`statusgen/verifyrun.go:151`) or of `witnessHeaderFailFirst`
   (`statusgen/failfirst.go:74`). A bracket's span is fixed by the bracket's position
   alone: it starts at the nearest witness table header ABOVE that position and ends at
   the line immediately before it. For an existing `Admission-Bracket:` line the position
   is that line; for the close step, which has no line yet, it is the line it is about to
   append. The span covers that one table and the `Tree-Digest:`, `Container-Image:` and
   `Verification-Attestation:` lines under it. Other witness tables in the same file or
   Evidence block, above or below, are outside it, so a brief with several witness tables
   needs no extra marker. A position with no witness table header above it has no span.
   The header and every table row are split into cells exactly as `cellSpans` splits them
   (`tools/desk/cmd/deskevidence/principalstamp.go:219`-`:242`, the boundaries statusgen's
   `splitRowEscaped` at `statusgen/verifyrows.go:60` reads): on `|`, but never on an
   escaped `\|`, dropping at most one blank cell at each end. A Command cell that contains
   `\|` therefore stays one cell and never shifts which cell is the Runner cell. Use the
   same splitter on both sides, never a plain split on `|`.
   The span is normalised before hashing: trailing whitespace stripped, blank lines
   dropped, lines joined with LF, and each table row's Runner cell (the column the span's
   header names `Runner`) rewritten in three steps. First, every occurrence of exactly
   `(on-behalf-of human:<subject>)` is removed, where `<subject>` is one or more characters
   other than whitespace, `(` and `)`, and the match is case-sensitive. No looser form is
   removed: a parenthetical such as `(on-behalf-of <desk role>)`, `(On-Behalf-Of human:x)`
   or `(on-behalf-of human: x)` stays in the hash. Second, every run of spaces and tabs
   inside the cell becomes one space. Third, the cell is trimmed. The bytes between the
   Runner cell's two `|` delimiters are then replaced by one space, the normalised value
   and one space, and every other byte of the row is kept. The whitespace step is what
   makes the digest identical before and after landing. `verifyrun` writes a trailing
   source tag such as `(git-config)` after the runner (`statusgen/verifyrun.go:239`-`:241`),
   and `deskevidence` inserts ` (on-behalf-of human:<subject>)` immediately before that tag
   (`tools/desk/cmd/deskevidence/principalstamp.go:151`-`:156`). For example the container
   cell `example-verifier[bot] @ <tree> (git-config)` lands as
   `example-verifier[bot] @ <tree>  (on-behalf-of human:<subject>)(git-config)`.
   Removing the stamp alone would leave a double space before `(git-config)`; collapsing
   whitespace restores the container's bytes. The exact-form stamp is therefore outside the
   hash: landing it, or restamping one exact-form annotation to another under brief 73,
   never changes the digest, while any other byte in the span does.
   Output: `--bracket-digest` prints one line per `Admission-Bracket:` line in the file, in
   file order, naming the bracket it checked:
   `bracket=L<bracket line> header=L<header line> witness=sha256:<64 hex>
   recorded=<the bracket's witness= value> verdict=<v>
   span-lines=tree-digest:<n>,container-image:<n>,attestation:<n>`. The three counts are
   the number of `Tree-Digest:`, `Container-Image:` and `Verification-Attestation:` lines
   inside that bracket's span. A bracket with no span prints `header=none witness=none`
   and `span-lines=none`. Task 7's exactly-one check is read from this output rather than
   counted by hand. The counts drive the verdict. The verdict is `match` only when the
   computed and recorded digests are equal, every count is 1, the bracket's `tree=`
   equals that span's one `Tree-Digest` value, the bracket line has exactly the form the
   close step writes (Task 6), its `run=` and `source=` equal that span's attestation's
   `run=` and `source=`, and its `opened=` and `closed=` are UTC RFC 3339 times with
   `opened` strictly before `closed`. The span digest does not cover the bracket line itself, so
   these checks are what catch an edit to the bracket line after landing. It is `mismatch` otherwise, including a span
   whose digests are equal but where any count is other than 1; the verb then also names
   the reason on standard error. It is `no-table` when no witness table header lies above
   the bracket, and `shared`, in place of `match` or `mismatch`, when another bracket in
   the file has the same nearest header. It exits 0 only when every bracket's verdict is
   `match`, exits 1 when any verdict is `mismatch`, `no-table` or `shared`, and exits 2
   when the file cannot be read or carries no `Admission-Bracket:` line. A container row is credited only when it lies inside exactly one bracket's
   span. Rows under a table that two brackets both cover lie in two spans and are not
   credited; that is what `shared` reports. The digest has no key and is computed from the
   file's own bytes, so anyone who can edit the file can write a bracket whose verdict is
   `match`: `verdict=match` shows only that a bracket is consistent with its own span, never
   who wrote it or which run it belongs to. It is necessary for credit, not sufficient;
   Task 8 decides credit. The `--bracket-digest` usage text says so.
2. Host open step: `statusgen verifyrun --open-bracket --root <home> --brief <brief> --out
   <file>`. It runs `deskdispatch --check-verifier` on the host, refuses if `--out` lies
   inside the home, computes the home's tree digest, and writes the receipt file: the check's
   JSON verbatim, `tree` (the digest) and `opened` (UTC timestamp). A refused check writes
   nothing and exits non-zero.
3. `verifyrun` gains `--admission-receipt <file>`, valid only with
   `--sandbox=container-netns`: the open step's file, mounted read-only. With container-netns,
   `verifyrun` never runs `deskdispatch`. It parses the embedded check output with
   `verifierAdmissionReceipt`, requires version 1 (version 2 is refused, naming the limit),
   requires the receipt's brief to equal the brief being run, and emits the attestation line
   as today. Before any row runs it computes the tree digest of its `--root` and refuses
   unless it equals the receipt's `tree`. After the last row it computes the digest again;
   if the two differ the run writes no attestation, `Tree-Digest` or bracket-ready witness
   and exits non-zero as could-not-run. This covers a mount that is a copy rather than the
   home, which the host close step cannot see. On a match it records
   `Tree-Digest: sha256:<…>` on the witness, between the table and the attestation line.
4. `--sandbox=container-netns` with no `--admission-receipt` is a usage refusal before
   any row runs. This applies whatever `DESK_LOOP` says and whether or not the
   linked-worktree git directory is visible. Unattested container runs end.
5. `verifyrun` gains `--container-image-digest sha256:<64 hex>`, required with
   `--sandbox=container-netns` and supplied by the host. It is emitted on the witness as
   `Container-Image: sha256:<…>`.
6. Host close step: `statusgen verifyrun --close-bracket --root <home> --brief <brief>
   --admission-receipt <file> --witness <file>`. It runs `deskdispatch --check-verifier` on
   the host and compares the result's run, source, brief, model and tier with the
   `Verification-Attestation:` line inside the span that ends at the append position (Task
   1). It recomputes the home's tree digest and requires it to equal both the receipt's
   `tree` and that span's `Tree-Digest`. Lines of these kinds outside the span (another
   table's) are ignored, and a span carrying more than one `Verification-Attestation:`,
   `Tree-Digest:` or `Container-Image:` line is refused. It refuses a witness that already
   carries an `Admission-Bracket` line. On a full match it appends one line:
   `Admission-Bracket: run=<run> source=<source> tree=sha256:<…> witness=sha256:<…>
   opened=<UTC> closed=<UTC> before=receipt after=check-verifier`, where `witness` is the
   witness digest of Task 1 over the span ending just above the line and `opened` comes
   from the receipt. It refuses when no witness table header lies above the append
   position. On any
   mismatch or refusal it appends nothing and exits non-zero. A row credited without this
   line is not a credited container row.
7. Amend rule 45 and verify-desk §Pre-work admission with the bracket sequence: host open
   step, container run with receipt and image digest, host close step, within one dispatch
   and with no home edits between. Amend pr-review-desk with the credit rule: a
   `sandbox=container-netns` row counts as a Linux witness only when the row lies inside
   exactly one `Admission-Bracket` line's span (Task 1), and that same span, not merely
   the same Evidence block, carries exactly one `Verification-Attestation:` line, exactly
   one `Tree-Digest:` line and exactly one `Container-Image:` digest line. The bracket's
   `tree` must equal that span's `Tree-Digest`, never a `Tree-Digest` belonging to another
   table. `statusgen verifyrun --bracket-digest` run on the landed Evidence block must
   report `verdict=match` and `span-lines=tree-digest:1,container-image:1,attestation:1` on that bracket's own `bracket=` line (never a raw `sha256sum`,
   which the landing stamp would break), and the bracket's `opened` must be strictly before `closed`.
   The same run with `--credit --source <the source under verification>` (Task 8), on the
   landed brief at the commit being judged, must report `credit=credited` on that same
   `bracket=` line. Otherwise it is rejected as unattested. Rule 45, verify-desk and
   pr-review-desk each state that `verdict=match` shows only that the bracket is
   consistent with its span, not who wrote it, and that a row is credited only when all
   of Task 8's conditions hold: the bracket line and every span line were landed by the
   verifier's landing step, each line's commit author matched to the roster's verifier on
   the author email's numeric id (a login-only match is could-not-check); the span's
   attestation equals the line rendered from the attestation record for that run, a
   record in the brief's own repository whose author is the dispatcher, never the
   verifier, admitted under Task 8(b)'s rules; and that attestation names the brief that
   contains it and the source under verification. A bracket and span written outside the landing step, a
   bracketed block copied from another brief or an earlier source, or an attestation that
   does not match its record is not credit, even when the digest matches. State the known
   limits: version 2 receipts are refused; a linked worktree's git directory is not mounted
   in the container, so rows that need `git` history must run on the host; the
   in-container digest is only as trustworthy as the pinned image that computes it;
   credit needs the brief's full history and a forge read, so a shallow clone or an
   unreachable forge is could-not-check, never credit; and Task 8's per-line provenance
   reads the commit author git records, which is tamper-evident, not tamper-proof
   (`statusgen/evidenceactor.go:65`-`:74`): whoever can commit to the brief can write the
   verifier's author address. The check makes that forgery a named, diffable act. Only
   server-side rules on who may push to the brief's branch prevent it, and a verifier
   roster entry with no numeric id leaves provenance could-not-check.
   Regenerate the Cursor mirror.
8. Credit check: `statusgen verifyrun --bracket-digest --witness <brief> --credit --source
   <40 hex>`, a read-only flag on the existing verb. `--credit` requires `--source` and a
   `--witness` file inside a git work tree. Each bracket's output line gains a final field
   `credit=credited`, `credit=not-credited` or `credit=could-not-check`, and the reason is
   named on standard error. Without `--credit` the output and exit status of Task 1 are
   unchanged. A bracket is `credited` only when its verdict is `match` and all three of
   these hold:
   a. Provenance. The bracket line and every line of its span were landed by the
      verifier's landing step. The history read that establishes this is the one the
      verified-flip judge makes for a PASS stamp (`flipProvenance`,
      `statusgen/verifyflip.go:756`-`:776`): on the brief file's committed `HEAD`,
      `git blame --line-porcelain -L <header line>,<bracket line> -- <brief>` run through
      `historyGit` (`statusgen/evidenceactor.go:932`), read with
      `blameRawLines(dir, out, false)` (`:763`) so that each record's author is its commit
      object's own, and judged line by line as `flipJudgeBlame` (`verifyflip.go:829`)
      judges: the output must hold exactly one record per line from the header through
      the bracket, and every record's author must classify as the roster's bound verifier
      under `evidenceActorPolicyFromRoster` (`evidenceactor.go:290`), which matches the
      commit author email's numeric id against the roster's `verifier=` binding and never
      trusts a display name. The match must be id-pinned (`idPinned`,
      `evidenceactor.go:262`): when the roster's verifier entry carries no numeric id, or
      the verifier is on a forge whose commit address carries none, the match is
      login-only and provenance is `could-not-check`, never `credited`. Unlike the flip
      judge, a roster human does not qualify: in normal operation the verifier identity
      writes a brief only through `deskevidence`, which checks that the landed commit's
      author is the verifier App (`tools/desk/cmd/deskevidence/deskevidence.go:73`). Blame
      reads the author recorded in each commit, not the identity that pushed it, so this
      condition is tamper-evident, not tamper-proof (`evidenceactor.go:65`-`:74`; Task 7's
      known limits). This is the per-line test, not
      the Evidence-actor lint's any-line test, under which one verifier line anywhere in
      Evidence covers every other line. Any span or bracket line whose working-tree bytes
      differ from `HEAD`'s is `not-credited`, because an uncommitted line has no author.
      A record by any other identity is `not-credited`. A shallow clone
      (`isShallowRepository`, `evidenceactor.go:947`), an unavailable roster, a record
      count other than the line count, or a record with no readable author is
      `could-not-check`.
   b. Record. The span's `Verification-Attestation:` line equals, byte for byte, the
      line rendered from the attestation record for that run. That record is the forge
      issue the line names (`<repo>#<issue>`), and `<repo>` must be the brief's own
      repository; a record in any other repository is `not-credited`. It is dispatcher-owned: the dispatcher
      writes it before the verifier's work, and the verifier does not
      (`tools/desk/internal/deskkit/verifierattestation.go:331`-`:338`). It is admitted
      only under every rule `ReadVerifierAttestation` applies
      (`verifierattestation.go:501`-`:592`):
      - its author is the dispatcher under `attestationAuthority("verifier")`
        (`tools/desk/internal/deskkit/workerattestation.go:92`-`:101`): the roster binds
        both a dispatcher and a verifier identity, the two are distinct actors, and the
        author is the dispatcher. A record written by the verifier is refused, and so is
        a roster that binds the two roles to one actor.
      - it is an issue, not a pull request; its title is exactly
        `[verification-attestation] <run>`; and its body is exactly the `verifierBody`
        rendering of the binding it carries, decoded strictly, with no unknown field.
      - the binding passes the field checks at `:535`-`:559`: an accepted version,
        `repo` equal to the repository read, hex `run`, `source` and digests, a valid
        evidence target, a valid model stamp label and a parseable checkout.
      - its body history is known, complete and never edited
        (`attestationRecordObserved`, `workerattestation.go:280`-`:292`).
      - it is closed, and a second read made after the history read still shows the same
        author, title, body and closed state (`:565`-`:581`).
      - the dispatcher's model stamp in its label history is present and equals the
        binding's model and tier (`:582`-`:589`).
      statusgen does not link deskkit, and `deskread`'s kinds are transport only: they
      map no author or edit time to accept or refuse
      (`tools/desk/cmd/deskread/main.go:23`-`:25`). So the check reads the record with
      `deskread issue`, its body history with `deskread trust` and its label history with
      `deskread label-events`, and applies these rules itself. The `trust` kind today
      carries `bodyEditedAt` and `complete` but not whether the forge reported body
      history at all (`main.go:883`-`:887`, against `TrustPayload.BodyHistoryKnown`,
      `tools/desk/internal/deskkit/forge.go:1157`-`:1158`), so an absent `bodyEditedAt`
      cannot be told apart from an unreported one. Task 8 adds that value to the `trust`
      kind's output as an additive field, `bodyHistoryKnown`, carried as the forge gave
      it. The check refuses a record whose `bodyHistoryKnown` is false or absent, as
      `attestationRecordObserved` does. Row 11 holds it to
      `ReadVerifierAttestation` on shared fixtures. The expected line is the admitted
      binding rendered as `VerifierReceipt.EvidenceBinding` renders it (`:487`-`:491`).
      That is the same comparison `deskevidence` makes before it writes an outcome
      record (`tools/desk/cmd/deskevidence/outcomebinding.go:93`-`:96`). The files under
      `docs/streams/verify-outcomes/` carry no `run=` and are written after landing, so
      they are not the per-run record. A record any rule refuses, or a rendered line
      that differs in any field (run, source, brief, model or tier), is `not-credited`.
      A failed read is `could-not-check`.
   c. Binding. The attestation's `brief=` equals the repository-relative path of the
      `--witness` file, so a block copied from another brief does not count. The
      attestation's `source=`, and therefore the bracket's (Task 1), equals `--source`,
      so a block from an earlier source of the same brief does not count. The bracket
      line carries no `brief=` of its own; it is bound to the brief through its span's
      attestation, which Task 1's verdict ties to the bracket's `run=` and `source=`.
   It exits 0 only when every bracket is `credited`, 1 when any bracket is
   `not-credited` (including any whose verdict is not `match`), and otherwise 2 when any
   is `could-not-check` or when the file cannot be read or carries no
   `Admission-Bracket:` line. Tests reach no network: they use fixture git repositories
   with fixture author identities, a fixture roster, and a fake `deskread` on `PATH`. The
   record fixtures the fake serves sit in one shared fixture directory,
   `statusgen/testdata/credit-records/`, each with its expected admit or refuse, so that
   row 11's deskkit test reads the same files. A `deskread` test pins the new
   `bodyHistoryKnown` field for both forge backends.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd statusgen && go test -run '^TestContainerNetnsRequiresAdmissionReceipt$' -count=1 -v .` | exit 0 and named PASS; with `DESK_LOOP` unset and no attestation file, container-netns without a receipt is refused before any row runs and no witness row is written; mutation: restore the conditional `required` path for container-netns — the named test must fail |
| 2 | check:ci | `cd statusgen && go test -run '^TestContainerNetnsReceipt$' -count=1 -v .` | exit 0 and named PASS; a valid version 1 receipt yields the same attestation line the in-process path renders and a `Tree-Digest` line; a version 2 receipt, a brief mismatch, an incomplete receipt, a missing image digest and a mounted tree whose digest differs from the receipt's `tree` (one file's content changed, one file added) are each refused before any row runs; a row that changes a file under the mounted root makes the run exit non-zero with no attestation or `Tree-Digest` line; no `deskdispatch` process is started (fake on PATH records zero calls) |
| 3 | check:ci | `cd statusgen && go test -run '^TestCloseBracket$' -count=1 -v .` | exit 0 and named PASS; a matching host check appends one `Admission-Bracket:` line carrying `tree=`, `witness=` (equal to `--bracket-digest` of the span above it), `opened=` and `closed=`; the witness digest is unchanged when an `(on-behalf-of human:<p>)` parenthetical is added to or replaced in a Runner cell, changes when a Result, Command or Date cell changes, and changes when a non-exact form such as `(on-behalf-of <desk role>)` or `(On-Behalf-Of human:x)` is added; a Runner cell carrying a `(git-config)` source tag digests the same unstamped and with the stamp inserted before the tag; in a file with two witness tables, each bracketed, `--bracket-digest` prints one line per bracket naming its own `bracket=` and nearest `header=` line, and a change in the first table changes only the first bracket's digest; a second bracket under the same table reports `verdict=shared` for both and exits non-zero; a bracket with no witness table header above it reports `verdict=no-table`; each printed line ends, after `verdict=`, with the span's `span-lines=tree-digest:<n>,container-image:<n>,attestation:<n>` counts, and a bracket with no witness table header above it prints `span-lines=none`; a bracket whose `witness=` equals its span's digest but whose span holds two `Tree-Digest:`, two `Container-Image:` or two `Verification-Attestation:` lines reports that count as 2 with `verdict=mismatch` and exits 1, and a second `Container-Image:` or `Verification-Attestation:` line added after the close likewise reports `verdict=mismatch`, so the row is not credited; a bracket whose `opened=` equals or follows its `closed=` reports `verdict=mismatch`; a witness `Tree-Digest` that differs from the home's recomputed digest, a home whose digest changed between open and close, a changed source, a different run, a witness that already carries a bracket line, a span whose only `Tree-Digest:` or `Verification-Attestation:` line belongs to another table, a span with two `Tree-Digest:` lines, a span with two `Container-Image:` lines, a span with two `Verification-Attestation:` lines, a receipt opened at or after the close time, or a refused host check each appends nothing and exits non-zero |
| 4 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBracketedWitnessSurvivesLanding$' -count=1 -v` | exit 0 and named PASS; a bracketed container witness fixture with unannotated App Runner cells, at least one of which ends in a runner-source tag such as `(git-config)`, and at least one row whose Command cell contains an escaped pipe (`grep -e a \| wc -l`), is passed through `deskevidence`'s landing stamp (`stampEvidencePrincipal` with a fixture roster through the existing `principalSubjectFn` seam, no forge), which adds the annotation before the tag; `statusgen verifyrun --bracket-digest`, built from this branch by the test, on the stamped block reports `verdict=match` and its `witness=` equals the bracket's; the same fixture with one Result cell edited does not; mutation: make the digest split on every `|`, escaped or not; the escaped-pipe row's stamp is then not normalised away, the stamped block reports `verdict=mismatch`, and the named test must fail |
| 5 | check:ci | `cd statusgen && go test -run '^TestSandbox' -count=1 -v .` | exit 0 and all seven existing `TestSandbox*` tests PASS; the existing loopback precheck, CI-lane and `--in-container` refusals are unchanged |
| 6 | check:ci | `grep -n -e 'Admission-Bracket' -e 'credit=credited' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md` | all three files match both patterns: each names the bracket and states that a row is credited only on `credit=credited`, not on `verdict=match` alone |
| 7 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift reported |
| 8 | check | `statusgen --root . --consumers --brief desktools-v2/71 --delivery f7ad98d584842aef0001ddf22ee83b9e56e4f91f --base 0191789433dcb3d43618ac0b43e35bdede3e8c84` | pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |
| 9 | check | `statusgen --root . --lint` | exit 0; no PROBLEM line names this brief |
| 10 | check:ci +mutation | `(cd statusgen && credit_out=$(mktemp) && trap 'rm -f "$credit_out"' 0 && go test -run '^TestBracketCredit$' -count=1 -v . > "$credit_out" 2>&1 && grep -q -- '--- PASS: TestBracketCredit ' "$credit_out")` | exit 0 and named PASS. The fixture setup is a git repository, a roster that binds distinct dispatcher and verifier identities with the verifier pinned by numeric id, and a fake `deskread` on `PATH` serving the shared record fixtures (no network). Base case: a bracketed container witness committed by the verifier identity, whose attestation equals the line rendered from an admitted dispatcher-written record and names this brief and `--source`, reports `credit=credited` and exits 0. Every negative changes exactly one condition from the base case. Each reports `verdict=match` and `credit=not-credited` and exits 1, unless it says otherwise. Condition (a): the same bytes committed by an identity that is not the verifier, which without `--credit` still reports `verdict=match` and exits 0; a span in which only the attestation line, or only the bracket line, is the verifier's and the rest is another identity's; one span line rewritten by another identity after landing; a bracket and span present only in the working tree; a roster human's commit; and a verifier roster entry with no numeric id, which reports `credit=could-not-check` and exits 2. Condition (b), all verifier-committed with this brief and source: a record whose rendered line differs from the span's in run, in source, in brief, in model and in tier (one case each); a record in another repository; a record whose author is the verifier rather than the dispatcher; a roster binding dispatcher and verifier to one actor; a record whose title is not exact; a record whose body has an unknown field or is not the exact rendering; a record whose body history shows an edit, is incomplete, or carries no `bodyHistoryKnown`; a record still open; and a record whose dispatcher model stamp is missing or names another model or tier. Condition (c), verifier-committed, with a record that matches the span line: a block copied into this brief whose attestation and record name another brief; and a block whose attestation, record and bracket name a source other than `--source`. A shallow clone, a missing roster and a failing `deskread` each report `credit=could-not-check` and exit 2. Mutations, one per condition, each making the named test fail: (a) replace the per-line author check with the any-line test, which credits the attestation-line-only case; (b) skip the record check, which credits the record-mismatch cases; (c) skip the `brief=` check, which credits the copied-block case; and skip the `source=` check, which credits the other-source case |
| 11 | check:ci | `(cd tools/desk && parity_out=$(mktemp) && trap 'rm -f "$parity_out"' 0 && go test ./internal/deskkit/ -run '^TestVerifierAttestationCreditParity$' -count=1 -v > "$parity_out" 2>&1 && grep -q -- '--- PASS: TestVerifierAttestationCreditParity ' "$parity_out")` | exit 0 and named PASS; every record fixture row 10 serves, read from the same shared fixture directory and fed to `ReadVerifierAttestation` through a fake forge, is admitted or refused exactly as that fixture's expectation says, which is what `--credit` must also do with it; a fixture that one reader admits and the other refuses fails the test |

Pre-mortem: a container run that is still unattested when admission is not detected is
caught by row 1. Re-running admission inside the container, or accepting a receipt for a
different brief, or running a mounted tree that is not the admitted one, is caught by
row 2. A bracket that closes on a moved home, on a witness from a different tree, or on a
witness that was already bracketed is caught by row 3; a bracket line moved to another
witness no longer matches its `witness` hash. An honest row rejected because the landing
stamp changed the hashed bytes, including the double space a stamp leaves before a
runner-source tag, is caught by rows 3 and 4. A bracket checked against the wrong
witness table, two brackets crediting one table, or a span carrying a duplicate
`Container-Image:` or attestation line, is caught by row 3. A copied mount changed during
the run is caught by row 2's after-rows digest. Loosening the existing sandbox refusals
while adding the flags is caught by row 5. A bracket and span written outside the landing
step, by hand or in another identity's commit, which still report `verdict=match` because
the digest is keyless, is caught by row 10, as are a bracketed block copied from another
brief or an earlier source and an attestation that does not match the record for its
run. Row 10 isolates each condition, with one mutation per condition. A record written
by the verifier itself, edited after it was written, or missing the dispatcher's stamp is
caught by row 10. A `--credit` record check that drifts from `ReadVerifierAttestation`
is caught by row 11. A credit rule that names `verdict=match` alone is caught by row 6.
Mutation check for row 1: restore the conditional `required` path for container-netns and
confirm row 1 goes red. Whether the receipt and close step together prove "immediately
before and after" is for review to judge, with the human gate as the backstop.

The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

### Postmerge verification, 2026-10-22, source 8f103c4712

Evidence only for a gate: human brief; status is left where it is. Source 8f103c4712 (merged main, contains delivery merge f7ad98d584 and its 37-file diff). Pre-work admission check exit 0.

Network-off credit for the check:ci rows goes only through the admitted offline container route, run once. It refused with exit 1 before any container started: `admitted-runner: refused: execution bytes/mode differ from source: scripts/windows-bootstrap-hashcheck-smoke.ps1` (that file carries eol=crlf; the raw-byte closure check sees the CRLF checkout against the LF blob, a known open gap of that route, tracked in #4112; #4106 is related and retires that one file only). No witness was produced, so every check:ci row and its mutation obligation is could-not-check, covered by #4112. The host results in the supplemental column come from the same row commands run on the host with GOPROXY=off; they show the named tests exist and pass, and give no acceptance credit.

| # | Class | Command | Acceptance result | Supplemental host result (no credit) |
|---|---|---|---|---|
| 1 | check:ci +mutation | `cd statusgen && go test -run '^TestContainerNetnsRequiresAdmissionReceipt$' -count=1 -v .` | could-not-check: admitted offline route refused at its closure check; mutation obligation not exercised | exit 0, `--- PASS: TestContainerNetnsRequiresAdmissionReceipt (0.28s)` |
| 2 | check:ci | `cd statusgen && go test -run '^TestContainerNetnsReceipt$' -count=1 -v .` | could-not-check: admitted offline route refused at its closure check | exit 0, `--- PASS: TestContainerNetnsReceipt (0.47s)` |
| 3 | check:ci | `cd statusgen && go test -run '^TestCloseBracket$' -count=1 -v .` | could-not-check: admitted offline route refused at its closure check | exit 0, `--- PASS: TestCloseBracket (0.84s)` |
| 4 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBracketedWitnessSurvivesLanding$' -count=1 -v` | could-not-check: admitted offline route refused at its closure check; mutation obligation not exercised | exit 0, `--- PASS: TestBracketedWitnessSurvivesLanding (3.14s)` |
| 5 | check:ci | `cd statusgen && go test -run '^TestSandbox' -count=1 -v .` | could-not-check: admitted offline route refused at its closure check | exit 0, seven PASS lines: PrecheckLoopback, PrecheckExtraIface, RunRefusedNoExec, WitnessRecordsMode, CILaneRefuses, FlagRefusals, OfReadsResultOnly |
| 6 | check:ci | `grep -n -e 'Admission-Bracket' -e 'credit=credited' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md` | could-not-check: admitted offline route refused at its closure check | exit 0; Admission-Bracket / credit=credited match lines per file: brief-rules 1 / 3, verify-desk 3 / 2, pr-review-desk 1 / 2 |
| 7 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | could-not-check: admitted offline route refused at its closure check | exit 0, `deskinstall --harness cursor --check: clean` |
| 8 | check | `statusgen --root . --consumers --brief desktools-v2/71 --base $(git merge-base origin/main HEAD)` | could-not-check: exit 2 on merged main, `COULD-NOT-CHECK: assay:assay:desktools-v2:71 is not in the diff against 8f103c4712…`; the row's Expect is pre-merge on the implementation branch, which a postmerge home cannot reproduce | probe with base d8b64df87c (merge-base of the delivery merge's parents): exit 0, `0 corroborated, 0 disproved, 6 unchecked` |
| 9 | check | `statusgen --root . --lint` | pass: exit 0, `LINT: PASS`, no PROBLEM line names this brief | none |
| 10 | check:ci +mutation | `(cd statusgen && credit_out=$(mktemp) && trap 'rm -f "$credit_out"' 0 && go test -run '^TestBracketCredit$' -count=1 -v . > "$credit_out" 2>&1 && grep -q -- '--- PASS: TestBracketCredit ' "$credit_out")` | could-not-check: admitted offline route refused at its closure check; four mutation obligations not exercised | exit 0, no output (grep -q found the PASS line) |
| 11 | check:ci | `(cd tools/desk && parity_out=$(mktemp) && trap 'rm -f "$parity_out"' 0 && go test ./internal/deskkit/ -run '^TestVerifierAttestationCreditParity$' -count=1 -v > "$parity_out" 2>&1 && grep -q -- '--- PASS: TestVerifierAttestationCreditParity ' "$parity_out")` | could-not-check: admitted offline route refused at its closure check | exit 0, no output (grep -q found the PASS line) |

Accounting: 11 rows; 1 pass (row 9); 10 could-not-check (rows 1-8, 10, 11; the check:ci rows among them per #4112); 0 failed.

RISK-VALUE: DERIVED — onBehalfOfStampRe = `\(on-behalf-of human:[^\s()]+\)` @ statusgen/containerbracket.go:796 — matches Task 1's exclusion exactly: literal case-sensitive `(on-behalf-of human:`, a subject of one or more characters other than whitespace and parentheses, closing `)`; looser forms stay in the hash.
RISK-VALUE: DERIVED — sha256DigestRe = `^sha256:[0-9a-f]{64}$` @ statusgen/containerbracket.go:57 — sha256 is 32 bytes, 64 lowercase hex as hex.EncodeToString renders it; the brief's `sha256:<64 hex>` form.
RISK-VALUE: DERIVED — receipt version gate `b.Version == 2` refused, `b.Version != 1` refused @ statusgen/containerbracket.go:130 and :133 — Task 3 admits version 1 only and refuses version 2 by name.

**VERIFY: BLOCKED** — 1 / 11 rows passed at source 8f103c4712; check:ci rows lack network-off credit (admitted route closure refusal, #4112), row 8 is a pre-merge-only check

### Postmerge verification, 2026-10-23, source fbe0331e2f

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run '^TestContainerNetnsRequiresAdmissionReceipt$' -count=1 -v .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run '^TestContainerNetnsReceipt$' -count=1 -v .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run '^TestCloseBracket$' -count=1 -v .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBracketedWitnessSurvivesLanding$' -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run '^TestSandbox' -count=1 -v .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n -e 'Admission-Bracket' -e 'credit=credited' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief desktools-v2/71 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:597086b84c9d | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:99134007746b | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd statusgen && credit_out=$(mktemp) && trap 'rm -f "$credit_out"' 0 && go test -run '^TestBracketCredit$' -count=1 -v . > "$credit_out" 2>&1 && grep -q -- '--- PASS: TestBracketCredit ' "$credit_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd tools/desk && parity_out=$(mktemp) && trap 'rm -f "$parity_out"' 0 && go test ./internal/deskkit/ -run '^TestVerifierAttestationCreditParity$' -count=1 -v > "$parity_out" 2>&1 && grep -q -- '--- PASS: TestVerifierAttestationCreditParity ' "$parity_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ fbe0331e2fc9 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4204 run=d7ffd6c489a1d6bcdb3d03c8ce0eaffb47c3da023e0a341b710addffc3884e89 source=fbe0331e2fc9f5c822c4edeafb8a18865c916828 brief=docs/streams/desktools-v2/brief-71-credit-container-witness-rows-bracketed-by-host-verifier-admission.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — 1/11 rows witnessed pass (row 9); no acceptance obligation observed failing. Rows 1-7, 10 and 11 (check:ci) are could-not-run: the container-netns lane refused before executing any row (could-not-attribute, below) and this darwin host has no unshare --net. Row 8 is could-not-check, not an implementation failure (below). No Admission-Bracket line exists, so no container row is credited.

Verifier: claude-opus-5-5, tier strong, non-implementer, against merged main source fbe0331e2fc9 (attestation medici-finance/assay#4204), installed desk-tools / statusgen v1.0.57, darwin arm64 host. This is the first qualification attempt of the v1.0.57 container-netns lane (brief-rule 45). The admitted-home check (deskdispatch --check-verifier) passed before work, with checkout "autocrlf=input eol=native"; the eol=crlf refusal recorded at medici-finance/assay#4112 did not recur.

Container lane, attempt by attempt (disclosed per the metrics gap medici-finance/assay#4163):

- Attempt 1 — discarded, uncredited. verifyrun --open-bracket succeeded (opened 2026-10-23T13:21:40Z, tree=sha256:d6f7e8a57312fdf1e7506911bd3a00c732819792d5ab411f974808de36b2bf14). The container run (image ghcr.io/medici-finance/verify-runner:v1.0.57@sha256:1b0f3a9a32a452b618fcf6761571af35f33239416482b7f748210f7b772566f3, --platform linux/amd64, --network none) exited 2 with "statusgen verifyrun: container admission refused; no Verify rows executed: the mounted root <home> digests to sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, not the sha256:d6f7e8a57312fdf1e7506911bd3a00c732819792d5ab411f974808de36b2bf14 the host recorded at the open step. The container is not running the admitted home" (home path elided as <home>). Cause was the verifier's own docker -v bind spelling, which mounted an empty directory (the digest is the empty-input sha256). The lane's tree check worked as designed. Nothing was written. The receipt was set aside and the run repeated with a read-only --mount type=bind.
- Attempt 2 — the recorded attempt. verifyrun --open-bracket succeeded (opened 2026-10-23T13:22:53Z, same tree digest). The container run passed container admission (the tree matched) and then exited 2 with no witness written: "statusgen verifyrun: could-not-attribute — no executing identity is available (no GITHUB_ACTOR under GitHub Actions, no git user.name/user.email in this repo). Refusing to write a witness with no runner: an unattributed witness is not a witness, and inventing a placeholder would make the one field naming who ran this the one field anybody could have written. Set the repo's git identity and re-run." The admitted home is a linked git worktree whose gitdir is outside the mounted tree, so git config cannot be read inside the container and the runner cannot be derived. The verifier did not mount the gitdir or inject an identity into the container, because either would put un-admitted state into the run. This is recorded as a lane defect for linked-worktree homes: the runner identity needs to reach the container through the admission receipt or a sanctioned input.
- verifyrun --close-bracket was refused: "statusgen verifyrun --close-bracket: refused, nothing appended: open <out>/witness.md: no such file or directory" (output path elided as <out>; no witness to close). verifyrun --bracket-digest --witness on this brief: "carries no Admission-Bracket: line, so there is nothing to check" (exit 2).

The table above is the host verifyrun witness for the same source. Its check:ci rows are could-not-run because sandbox=unshare is unavailable on darwin.

Row 8 (fail exit=2 in the table): origin/main and merge-base(origin/main, HEAD) both resolve to the verified source, so the diff is empty and the row reports 0 corroborated, 0 disproved, 6 unchecked ("unchanged since the merge-base — this branch did not make this claim"). Its Expect is pre-merge on the implementation branch, so it cannot be discharged after merge. Classed could-not-check (check-definition), not FAIL (blocker: #4209). A no-credit probe with --base 0191789433dc (first parent of the delivering merge f7ad98d58, PR medici-finance/assay#4078) exited 0 but still reported 0 corroborated, 6 unchecked.

Supplemental host probe, NO CREDIT: rows 1-7, 10 and 11 were run on the darwin host without any network sandbox. They ran on a git-archive copy of the source with GOPROXY=off, GOFLAGS=-mod=mod, real Go caches and a throwaway HOME. All 9 exited 0 (1-5 printed --- PASS for each named test; 6 matched lines in all three files; 7 printed "deskinstall --harness cursor --check: clean"; 10 and 11 grep-confirmed --- PASS). This is not a hermetic witness, earns no credit and does not change the verdict. It only shows that nothing observed contradicts the obligations.

Risk-bearing values (the brief's risk fields are all "no" and it is not irreversible; enumerated over the delivering diff's non-test statusgen container bracket and credit code):
- RISK-VALUE: DERIVED — mode = "100644" / "120000" / "100755" @ statusgen/containerbracket.go:249/256/262 — these are git's canonical tree-entry modes for regular, symlink and executable files, so the tree digest matches git's own file-type model.
- RISK-VALUE: DERIVED — sha256DigestRe = ^sha256:[0-9a-f]{64}$ @ statusgen/containerbracket.go:57 — a SHA-256 digest is 32 bytes, which is 64 lowercase hex characters.
- RISK-VALUE: DERIVED — hex40Re = ^[0-9a-f]{40}$ @ statusgen/containerbracket.go:59; creditHexCommitRe = ^([0-9a-f]{40}|[0-9a-f]{64})$ @ statusgen/containercredit.go:286 — git object ids are 40 hex characters (SHA-1) or 64 (SHA-256 object format).
- RISK-VALUE: NAMED, NOT DERIVED — attestation Version != 1 refused (and Version == 2 refused for container runs) @ statusgen/containerbracket.go:133, statusgen/containercredit.go:419-421 — a schema-version pin. It is reversible by edit and release, and no spec text fixes the container lane to version 1 beyond the refusal message.
- RISK-VALUE: NAMED, NOT DERIVED — deskread timeout = 90*time.Second @ statusgen/containercredit.go:603 — a reversible operational knob, ranked last and not derived.

Blocker: environment. The container-netns lane cannot attribute a runner for a linked-worktree home. Needed capability: verifyrun deriving the runner from the admission receipt or another sanctioned host-supplied input. Next actor: a desk-tools worker (blocker: #4208); the desk re-dispatches once it ships. Row 8 needs a post-merge-dischargeable definition (pinned --delivery/--base) (blocker: #4209). The verifier did not flip status (gate: human).


### Independent verification — 2026-10-23 (source 3e24567bbc4e1c53c508e4f74d8ad04d3364d494)

Non-implementer verifier `assay-verifier-app[bot]` (on-behalf-of human:ian), model claude-opus-5-5, tier strong, run `b51e4bc1200e08008ab5fb5a7509f34926bbe7d55cecd5731ac5080595e6fea7` (attestation medici-finance/assay#4245). Merged main `3e24567bbc4e1c53c508e4f74d8ad04d3364d494`, clean detached checkout, git 2.56.0, `core.autocrlf=input`, `eol=native`. Host tools: `statusgen v1.0.58`; `deskdispatch sourceSHA=832f9c8f releaseTag=v1.0.58`; go1.27.2 darwin/arm64. Runner image `ghcr.io/medici-finance/verify-runner:v1.0.58@sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204`, local image Id `sha256:767f6a3545748b57a85f85dc80216e97203a39aa07e2ef137483d197dd60b175` (linux/amd64), pulled by digest, run with `--network none`, the checkout mounted read-only, no gitdir mount, no injected identity, no locally built binary. `deskdispatch --check-verifier` exited 0 before the work and again before this hand-off (same binding: run, source, model, tier, issue 4245).

This is a re-verification after the approved row-8 command amendment (amendment `4c63244c4665af3bc1ec2319379f1bafd5dd4c8d`, PR #4213; approval medici-finance/assay#4227 comment 6373814726 by the ratifying human (human:ian), unedited). The earlier Evidence attempts in PR #4114 (sources 8f103c4712 and fbe0331e2f, both BLOCKED; the second on the v1.0.57 could-not-attribute refusal, #4208) are context only and are not credited here.

#### Container-netns lane (rule 45): bracketed witness

Open: `statusgen verifyrun --open-bracket` exit 0 — `opened 2026-10-23T23:44:48Z tree=sha256:f1b1e71a00b5fcb41274cb1805f8159ab6b1549239c329dca97b0694552fa9c4 runner=assay-verifier-app[bot] (forge-identity)`. One container run (23:45:05Z–23:46:35Z), no discarded attempt, exit 1 (rows 8 and 9, below), empty stderr. Close: `--close-bracket` exit 0. `--bracket-digest`: `bracket=L17 header=L1 witness=sha256:b651b412d83ff82125fca8329ddcb57798a3533af0ce1cf069ad74027e1a0612 recorded=sha256:b651b412d83ff82125fca8329ddcb57798a3533af0ce1cf069ad74027e1a0612 verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`. The closed witness follows byte for byte:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run '^TestContainerNetnsRequiresAdmissionReceipt$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:043f118b7ec2 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run '^TestContainerNetnsReceipt$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:34850e7d2ac8 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run '^TestCloseBracket$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:5399fa48563a | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBracketedWitnessSurvivesLanding$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:9b5e80a4b207 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run '^TestSandbox' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:34a7a0cccb8e | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n -e 'Admission-Bracket' -e 'credit=credited' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:39bfd148f696 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief desktools-v2/71 --delivery f7ad98d584842aef0001ddf22ee83b9e56e4f91f --base 0191789433dcb3d43618ac0b43e35bdede3e8c84` | fail exit=2 sandbox=container-netns | sha256:97ec05843a44 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | fail exit=5 sandbox=container-netns | sha256:ebb49aa52600 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd statusgen && credit_out=$(mktemp) && trap 'rm -f "$credit_out"' 0 && go test -run '^TestBracketCredit$' -count=1 -v . > "$credit_out" 2>&1 && grep -q -- '--- PASS: TestBracketCredit ' "$credit_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd tools/desk && parity_out=$(mktemp) && trap 'rm -f "$parity_out"' 0 && go test ./internal/deskkit/ -run '^TestVerifierAttestationCreditParity$' -count=1 -v > "$parity_out" 2>&1 && grep -q -- '--- PASS: TestVerifierAttestationCreditParity ' "$parity_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:f1b1e71a00b5fcb41274cb1805f8159ab6b1549239c329dca97b0694552fa9c4
Container-Image: sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204
Verification-Attestation: medici-finance/assay#4245 run=b51e4bc1200e08008ab5fb5a7509f34926bbe7d55cecd5731ac5080595e6fea7 source=3e24567bbc4e1c53c508e4f74d8ad04d3364d494 brief=docs/streams/desktools-v2/brief-71-credit-container-witness-rows-bracketed-by-host-verifier-admission.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=b51e4bc1200e08008ab5fb5a7509f34926bbe7d55cecd5731ac5080595e6fea7 source=3e24567bbc4e1c53c508e4f74d8ad04d3364d494 tree=sha256:f1b1e71a00b5fcb41274cb1805f8159ab6b1549239c329dca97b0694552fa9c4 witness=sha256:b651b412d83ff82125fca8329ddcb57798a3533af0ce1cf069ad74027e1a0612 opened=2026-10-23T23:44:48Z closed=2026-10-23T23:46:51Z before=receipt after=check-verifier

Rows 8 and 9 fail in the container only because they are `check` rows that read git history (`--consumers --delivery/--base` and `--lint`), and the gitdir is deliberately not mounted (a documented limit of this lane). They are not credited from the container. They were run on the host under the approved transition (next block). `verdict=match` is not credit: per rule 45, the nine container rows (1–7, 10, 11) are credited only when `statusgen verifyrun --bracket-digest --witness <brief> --credit --source 3e24567bbc4e1c53c508e4f74d8ad04d3364d494` returns `credit=credited` on the landed brief. That check reads the landing commit, so it cannot run before landing, and it was not run.

#### Host run under the approved acceptance transition (rows 8 and 9)

Descriptor (kept outside the checkout):

```text
Verification-Table: delivery=f7ad98d584842aef0001ddf22ee83b9e56e4f91f base=d8b64df87c1d94a61ca398fb5371acfe81569f78 original=deee32957a70aa7e7a6bbae14f004284ab1b5fa4
Acceptance-Transition: amendment=4c63244c4665af3bc1ec2319379f1bafd5dd4c8d approval=https://github.com/medici-finance/assay/issues/4227#issuecomment-6373814726
```

The verifier checked the coordinates independently. Delivery f7ad98d5 is on HEAD's first-parent chain; its two parents' unique merge-base is d8b64df8 (= base); deee3295 is an ancestor of base; amendment 4c63244c is on the first-parent chain after delivery. The approval body names exactly this repo, brief, delivery, base, original, amendment and `rows=8`.

Disclosed attempt (uncredited): the first host invocation passed `--brief` as a repository-relative path and was refused before any row executed: `protected table: acceptance transition: cannot resolve brief path` (exit 2). The cause is `statusgen/acceptancetransition.go:191`: `filepath.Rel(root, canonicalPath)` fails when the brief path is relative and the toplevel is absolute. The documented example `--brief docs/streams/example/...` hits the same failure. The verifier re-issued the identical command with the absolute brief path. The checker then read the approval, accepted it as the exact transition payload (no hold, no refusal) and executed the canonical plan (`--dry-run`: nothing was written to the checkout).

Host result: rows 1–7, 10, 11 could-not-run (check:ci requires `unshare --net`, which is unavailable on darwin; they are covered by the container lane above). Rows 8 and 9:

```text
| 8 | `statusgen --root . --consumers --brief desktools-v2/71 --delivery f7ad98d584842aef0001ddf22ee83b9e56e4f91f --base 0191789433dcb3d43618ac0b43e35bdede3e8c84` | pass exit=0 | sha256:2243e380259d | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e1c53c508e4f74d8ad04d3364d494 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:cc4ab06a7fb2 | 2026-10-23 | assay-verifier-app[bot] @ 3e24567bbc4e1c53c508e4f74d8ad04d3364d494 (on-behalf-of human:ian) (forge-identity) |
```

Primary output was inspected by re-running each command directly (same output hash). Row 8: 4 CORROBORATED, 0 disproved, 2 UNCHECKED (version-2 receipts and adopter-owned image/runtime, both out-of-scope and reviewer-owned), `completeness is not proved`. This matches Expect. Row 9: `LINT: PASS`, zero PROBLEM lines, none naming this brief. Notices name this brief only for the `risk-files-crossread` advisory (deskkit path versus all-"no" risk answers; the gate is already `human`) and for `gotest-run-vacuous` on rows 1–5. Rows 10 and 11 already assert `--- PASS:`.

#### Other checks

`deskpathguard rederive --root <checkout> --brief <brief>`: exit 6, `could-not-check: cannot rederive Verify table: base: Verify section line 7: row width differs from header (escape cell pipes as \|)`. The protected-table transition above is the sanctioned re-derivation path for this brief. This line is recorded and not relied on.

Row 6 content was read in full, not only as an exit code. docs/brief-rules.md, the verify-desk skill and the pr-review-desk skill each name `Admission-Bracket` and state "Credit a row only on `credit=credited`, never on `verdict=match` alone".

Runner-cell gate: every Runner cell in both tables reads `assay-verifier-app[bot] @ <sha> (on-behalf-of human:…) (forge-identity)`. No cell was hand-edited.

#### Risk-bearing values (enumerated, ranked, derived)

RISK-VALUE: onBehalfOfStampRe `\(on-behalf-of human:[^\s()]+\)` @ statusgen/containerbracket.go:948 — DERIVED from Task 1's case-sensitive exact stamp form; looser spellings stay inside the hashed cell (whitespace collapse `[ \t]+` @ :950 per Task 1).
RISK-VALUE: sha256DigestRe `^sha256:[0-9a-f]{64}$` @ statusgen/containerbracket.go:61 — DERIVED: a 32-byte sha256 is 64 lowercase hex characters.
RISK-VALUE: hex40Re `^[0-9a-f]{40}$` @ statusgen/containerbracket.go:63 and creditHexCommitRe `^([0-9a-f]{40}|[0-9a-f]{64})$` @ statusgen/containercredit.go:286 — DERIVED: SHA-1 and SHA-256 git object id lengths.
RISK-VALUE: tree modes "100644"/"100755"/"120000" @ statusgen/containerbracket.go:333/346/340 — DERIVED: git's canonical blob, executable and symlink tree modes (Task 1 tree digest).
RISK-VALUE: receipt version gate (2 refused naming the limit; anything but 1 refused) @ statusgen/containerbracket.go:205/208 and statusgen/containercredit.go:419/421 — DERIVED from Task 3.
RISK-VALUE: `closed.Truncate(time.Second).After(opened)` @ statusgen/containerbracket.go:640 — DERIVED: "opened strictly before closed" at RFC 3339 second precision (Tasks 1 and 6).
RISK-VALUE: deskread record timeout `90*time.Second` @ statusgen/containercredit.go:603 — NAMED, NOT DERIVED (reversible knob, ranked last).

#### Accounting

11 rows. All 11 obligations were observed meeting their Expect under a sanctioned lane in this run. Rows 1–7, 10 and 11 pass in the bracketed container witness (`verdict=match`); their credit is deferred to `--credit` on the landed brief. For the go-test rows, the mutation obligations were exercised only as far as the named tests encode them. Rows 8 and 9 pass on the host under the approved transition. No obligation was observed failing.

Two items keep this from closing as a PASS now. (1) Rule 45 forbids crediting the nine container rows on `verdict=match` alone, and `--credit` needs the landed brief. (2) This run's witnesses span two tables: the container table records rows 8 and 9 as fail, and the host table records rows 1–7, 10 and 11 as could-not-run. The closure audit reads the latest witness per row, so whichever table comes last masks the other. How these combine into one closure record is a lane question for the desk, not something a verifier should settle by editing either table.

**VERIFY: BLOCKED** — credit-timing and evidence-assembly block, not an obligation failure. All 11 rows were observed passing; credit for the nine container rows waits on `--credit` against the landed brief. Status is not flipped (`gate: human`).

Qualifier added 2026-10-24: row 8 is routing-only corroboration (medici-finance/assay#4244). Its four CORROBORATED entries are `follow-up desktools-v2/71` self-routes, and the delivery diff `0191789433dc..f7ad98d58484` touches neither `statusgen/verifieradmission.go` nor `statusgen/verifysandbox.go`. So "This matches Expect" for row 8 above, and the "all 11" counts in the two lines above, mean that the routing output matched Expect. They do not mean the admission code was independently corroborated. The table cells are unchanged.

### Independent verification — 2026-10-24 (source 385cb29e5682dcb9a886b298076f722b4823ec1b)

Non-implementer verifier `assay-verifier-app[bot]` (on-behalf-of human:ian), model claude-opus-5-5, tier strong, run `0263a46c7a3a3393d2502217faba8d4d625d321831a06a73e31d836f7bab46ac` (attestation medici-finance/assay#4293). Merged main `385cb29e5682dcb9a886b298076f722b4823ec1b`, clean detached checkout, git 2.56.0, `core.autocrlf=input`, `eol=native`. Host tools: `statusgen v1.0.59`; `deskdispatch sourceSHA=dff97978 releaseTag=v1.0.59`. Runner image `ghcr.io/medici-finance/verify-runner:v1.0.59@sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e` (local image Id `sha256:756b81ba91a8627b30524316a3baabed654a0369f9331a31e77546ead69c19ad`, linux/amd64, `statusgen v1.0.59`, go1.27.2 inside), run with `--pull never --network none --read-only --cap-drop ALL`, the checkout bind-mounted read-only, no gitdir mount, no injected identity, no locally built binary. `deskdispatch --check-verifier` exited 0 before the work and again before this hand-off (same binding: run, source, model, tier, issue 4293).

Prior attempts, disclosed and not credited here (recorded earlier in this Evidence section):
- 2026-10-22, source 8f103c4712: BLOCKED, 1/11 rows passed. The admitted offline route was refused at its closure check (#4112).
- 2026-10-23, source fbe0331e2f (statusgen v1.0.57, attestation #4204): BLOCKED, 1/11 rows passed. The container lane returned could-not-attribute on a linked-worktree home (#4208). Row 8 could only be checked before merge (#4209).
- 2026-10-23, source 3e24567bbc (statusgen v1.0.58, attestation #4245): BLOCKED. All 11 rows were observed passing (row 8 as routing-only corroboration, medici-finance/assay#4244), but the container and host tables each recorded the other's rows as fail or could-not-run, so whichever landed last masked the other (#4247). The driver ruled option A, `verifyrun --rows` (https://github.com/medici-finance/assay/issues/4247#issuecomment-6377158517), shipped as desktools-v2/74 in v1.0.59. This run uses it: the container and host selections are disjoint and together cover all 11 rows.

#### Host run under the approved acceptance transition (rows 8 and 9)

The host run came first, on the clean home, because a transition run refuses a dirty source. The container witness file was written and closed outside the home, so the home was still clean at that point. The descriptor below was passed with `--acceptance-record` (kept outside the checkout), together with `--acceptance-forge` and the absolute brief path (#4246). The verifier checked the coordinates independently: delivery f7ad98d5 is an ancestor of HEAD; delivery and amendment 4c63244c are both on HEAD's first-parent chain; the merge-base of delivery's two parents is d8b64df8 (= base); and deee3295 is an ancestor of base. verifyrun read the approval and executed the canonical plan. The table below is the one it wrote (`statusgen verifyrun --brief <abs brief> --root <home> --rows 8,9 --acceptance-record <descriptor> --acceptance-forge`, exit 0; rows 1–7, 10 and 11 printed `not selected`).

Verification-Table: delivery=f7ad98d584842aef0001ddf22ee83b9e56e4f91f base=d8b64df87c1d94a61ca398fb5371acfe81569f78 original=deee32957a70aa7e7a6bbae14f004284ab1b5fa4
Acceptance-Transition: amendment=4c63244c4665af3bc1ec2319379f1bafd5dd4c8d approval=https://github.com/medici-finance/assay/issues/4227#issuecomment-6373814726

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 8 | `statusgen --root . --consumers --brief desktools-v2/71 --delivery f7ad98d584842aef0001ddf22ee83b9e56e4f91f --base 0191789433dcb3d43618ac0b43e35bdede3e8c84` | pass exit=0 | sha256:2243e380259d | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682dcb9a886b298076f722b4823ec1b (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:a621a2dd6336 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682dcb9a886b298076f722b4823ec1b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4293 run=0263a46c7a3a3393d2502217faba8d4d625d321831a06a73e31d836f7bab46ac source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desktools-v2/brief-71-credit-container-witness-rows-bracketed-by-host-verifier-admission.md model=claude-opus-5-5 tier=strong

Row 8 was re-run directly and gave the same output hash (`sha256:2243e380259d`). Summary line: `4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. Both UNCHECKED entries (version-2 receipts, and the adopter-owned image and runtime) are declared out of scope, and the reviewer weighs them. **Row 8 is routing-only corroboration (F3; interim rule medici-finance/assay#4244).** All four CORROBORATED entries are `follow-up desktools-v2/71` self-routes, so they show only that each named surface routes to this brief. They do not show that the delivery changed it. The delivery diff `0191789433dc..f7ad98d58484` (37 files) does not touch `statusgen/verifieradmission.go` or `statusgen/verifysandbox.go`, even though the first entry names both. Row 8's pass therefore means only that its Expect (exit 0 with this summary) was met. It is not evidence that those two files implement anything. Their behaviour is covered by rows 1, 2 and 5, which pass in the container below.

Row 9 was re-run directly: `LINT: PASS`, zero PROBLEM lines. Its output hash differs from the witness because the brief had gained this run's host table by then. The NOTICEs naming this brief are the pre-existing Verify-table authoring notices (`risk-files-crossread`, a `+dereference` verify-obligation, `gotest-run-vacuous` on rows 1–5). For rows 1–5, every named test function exists at source: `TestContainerNetnsRequiresAdmissionReceipt`, `TestContainerNetnsReceipt` and `TestCloseBracket` in `statusgen/containerbracket_test.go`, `TestSandbox*` (7 functions) in `statusgen/verifysandbox_test.go`, and `TestBracketedWitnessSurvivesLanding` in `tools/desk/cmd/deskevidence/bracketlanding_test.go`. The `-run` filters therefore match real tests. Rows 10 and 11 already assert `--- PASS:`.

#### Container-netns lane (rule 45): bracketed witness (rows 1–7, 10, 11)

Open: `statusgen verifyrun --open-bracket` exit 0, `opened 2026-10-24T17:06:53Z tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 runner=assay-verifier-app[bot] (forge-identity)`. One container run (17:07:03Z–17:08:51Z) with `--rows 1,2,3,4,5,6,7,10,11`, no discarded attempt, exit 0, empty stderr; rows 8 and 9 printed `not selected`. Close: `--close-bracket` exit 0. `--bracket-digest` on the closed file: `bracket=L15 header=L1 witness=sha256:a507b1f846105713bdb631dd0e062465f17c413a21302304e3bd67cec3e1ba3e recorded=sha256:a507b1f846105713bdb631dd0e062465f17c413a21302304e3bd67cec3e1ba3e verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`, exit 0. The closed witness follows byte for byte:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run '^TestContainerNetnsRequiresAdmissionReceipt$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:2bf3dbcd46cd | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run '^TestContainerNetnsReceipt$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:051d05edfffb | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run '^TestCloseBracket$' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:180d4d02e15e | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBracketedWitnessSurvivesLanding$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:b3b855c11332 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run '^TestSandbox' -count=1 -v .` | pass exit=0 sandbox=container-netns | sha256:fddd8c65a52c | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n -e 'Admission-Bracket' -e 'credit=credited' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:0b4800eb3a49 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd statusgen && credit_out=$(mktemp) && trap 'rm -f "$credit_out"' 0 && go test -run '^TestBracketCredit$' -count=1 -v . > "$credit_out" 2>&1 && grep -q -- '--- PASS: TestBracketCredit ' "$credit_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd tools/desk && parity_out=$(mktemp) && trap 'rm -f "$parity_out"' 0 && go test ./internal/deskkit/ -run '^TestVerifierAttestationCreditParity$' -count=1 -v > "$parity_out" 2>&1 && grep -q -- '--- PASS: TestVerifierAttestationCreditParity ' "$parity_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4293 run=0263a46c7a3a3393d2502217faba8d4d625d321831a06a73e31d836f7bab46ac source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desktools-v2/brief-71-credit-container-witness-rows-bracketed-by-host-verifier-admission.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=0263a46c7a3a3393d2502217faba8d4d625d321831a06a73e31d836f7bab46ac source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:a507b1f846105713bdb631dd0e062465f17c413a21302304e3bd67cec3e1ba3e opened=2026-10-24T17:06:53Z closed=2026-10-24T17:09:00Z before=receipt after=check-verifier

`verdict=match` is not credit. Under rule 45, the nine container rows are credited only when `statusgen verifyrun --bracket-digest --witness <brief> --credit --source 385cb29e5682dcb9a886b298076f722b4823ec1b` returns `credit=credited` on the landed brief. Condition 1 blames the landing commit, so before landing the check can only return not-credited for uncommitted lines. The check must be re-run on the landed brief, and this verdict's credit for rows 1–7, 10 and 11 stands only on that result.

Row 6 content was read in full, not only as an exit code: `docs/brief-rules.md`, the verify-desk skill and the pr-review-desk skill each name `Admission-Bracket` and require `credit=credited`.

Runner-cell gate: every Runner cell in both tables reads `assay-verifier-app[bot] @ <sha> (on-behalf-of human:ian) (forge-identity)`, as verifyrun wrote it. No cell was hand-edited.

#### Risk-bearing values (enumerated, ranked, derived; re-checked at 385cb29e)

RISK-VALUE: onBehalfOfStampRe `\(on-behalf-of human:[^\s()]+\)` @ statusgen/containerbracket.go:948 — DERIVED from Task 1's case-sensitive exact stamp form; looser spellings stay inside the hashed cell (whitespace collapse `[ \t]+` @ :950 per Task 1).
RISK-VALUE: sha256DigestRe `^sha256:[0-9a-f]{64}$` @ statusgen/containerbracket.go:61 — DERIVED: a 32-byte sha256 is 64 lowercase hex characters.
RISK-VALUE: hex40Re `^[0-9a-f]{40}$` @ statusgen/containerbracket.go:63 and creditHexCommitRe `^([0-9a-f]{40}|[0-9a-f]{64})$` @ statusgen/containercredit.go:286 — DERIVED: SHA-1 and SHA-256 git object id lengths.
RISK-VALUE: tree modes "100644"/"100755"/"120000" @ statusgen/containerbracket.go:333/346/340 — DERIVED: git's canonical blob, executable and symlink tree modes (Task 1 tree digest).
RISK-VALUE: receipt version gate (2 refused naming the limit; anything but 1 refused) @ statusgen/containerbracket.go:205/208 and statusgen/containercredit.go:419/421 — DERIVED from Task 3.
RISK-VALUE: `closed.Truncate(time.Second).After(opened)` @ statusgen/containerbracket.go:640 — DERIVED: "opened strictly before closed" at RFC 3339 second precision (Tasks 1 and 6).
RISK-VALUE: deskread record timeout `90*time.Second` @ statusgen/containercredit.go:603 — NAMED, NOT DERIVED (reversible knob, ranked last).

#### Accounting

11 rows. Every row was observed meeting its Expect: rows 1–7, 10 and 11 in the bracketed container witness (`verdict=match`), and rows 8 and 9 on the host under the approved transition. For the go-test rows, the mutation obligations were exercised only as far as the named tests encode them. Row 8 is routing-only corroboration, as stated above. No obligation was observed failing.

The closure audit refuses the assembled run, so this cannot be recorded as a PASS. With a strict PASS verdict marker in place, `statusgen verifyrun --check <brief>` exits 2: `refused — protected table: refusing: sha mismatch: rows of the latest PASS name 385cb29e5682dcb9a886b298076f722b4823ec1b and 385cb29e5682`. Without a verdict line it exits 2: `refused — protected table: acceptance transition: a fresh completed independent run is required`. The cause is in the tools, not in the rows. Under `--acceptance-record`, the host run writes the full 40-hex HEAD into its Runner cells (`statusgen/verifyrun.go:2430`, the transition branch that sets `executionTree` from `rev-parse HEAD`). The container rows take the receipt's 12-hex `runner_tree`. `flipLatestPass` (`statusgen/verifyflip.go:510`) requires one identical Runner cell across every row of a PASS run. So when a protected table mixes container rows with host rows, as desktools-v2/74 (`--rows`) prescribes, it can never close. Every other way to assemble it is also refused: the container cannot take `--acceptance-record` (host only), and a whole-table host run leaves rows 1–7, 10 and 11 could-not-run on darwin (#1800). No witness cell was edited to make the forms agree.

**VERIFY: BLOCKED** — tool defect, not an obligation failure. All 11 rows were observed passing (the container bracket gives `verdict=match`; rows 8 and 9 pass on the host under the transition; row 8 is routing-only, medici-finance/assay#4244). The closure audit cannot close a protected table whose run mixes a container bracket (12-hex runner tree from the receipt) with a transition host run (40-hex HEAD). Next actor: a desk-tools worker, to make the two Runner-cell sha forms agree (blocker: #4299). Then re-dispatch. Status is not flipped (`gate: human`).


## Review

Gate: human (gate-why above). Independent correctness and security review of the exact head.
