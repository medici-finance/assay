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

files: statusgen/verifyrun.go, statusgen/verifieradmission.go, statusgen/verifysandbox.go and their tests; docs/brief-rules.md (rule 45); plugins/assay/skills/verify-desk/SKILL.md; plugins/assay/skills/pr-review-desk/SKILL.md; tools/desk/cmd/deskevidence/ (test only); tools/desk/internal/deskkit/verifierattestation.go (read only); statusgen/verifyflip.go and statusgen/evidenceactor.go (read only: the per-line history read Task 8 reuses); tools/desk/cmd/deskread/main.go and its tests (one additive field on the `trust` kind, Task 8); tools/desk/internal/deskkit/ (one new test file, row 11; no code change)

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

## Review

Gate: human (gate-why above). Independent correctness and security review of the exact head.
