# verify-wake-v1 — verification wake receipts

Status: implemented (desk-supervision/16; join keys and the required failure receipt, desk-supervision/27). Scheduling evidence only — a receipt can never grant
`verified`/`done`, satisfy acceptance, or authorize a write. The per-item claim and the
reviewer/verifier identity gates remain the barriers a receipt sits beside.

## Why

A verifier run that ends `verify-fail` or `blocked` was re-listed as a DISPATCH candidate every
plan and re-run every pass, reproducing the identical non-verdict, even when nothing its outcome
depends on had changed. A **wake receipt** keeps that failure VISIBLE while making the next
expensive run depend on a CHECKABLE change: it records what was observed, the inputs it was
observed against, the class of blocker, and the condition that must change before re-running is
worth a verifier slot.

This governs **failed/blocked retries only**. A `verified` outcome whose flip is stuck is the
stuck-flip lane's concern; a longitudinal `blocked-until:` window and an online verify lane keep
their existing buckets.

## Where it lives

A receipt is a **versioned extension of the verify-outcome record** — since #882
(desk-supervision/24) one NEW file per outcome under `docs/streams/verify-outcomes/<stream>/`,
named by `RecordName(content)`; before that, one appended line in the shared
`docs/streams/verify-outcomes.jsonl` (still read, unioned in, for as long as that log is not yet
retired — see desk-supervision/24's Task step 8). Either shape's content is exactly the legacy row
(`ts`, `brief`, `outcome`, `sha`) plus these additive keys, so:

- an older reader that knows only those four keeps working, ignoring the additive fields;
- a legacy record (no `wake_schema`) parses as an **incomplete** receipt and stays visibly
  unclassified — never a fabricated hold;
- the receipt is REQUIRED on every new `verify-fail` / `blocked` record (the writer refuses one
  without a complete receipt, naming the first missing field; see "Failure records" below) and on
  any record that claims `wake_schema: "verify-wake-v1"`;
- the additive keys never invalidate or shrink an existing record: records are IMMUTABLE (a
  correction is a NEW record — a fresh `ts`/digest — never an edit of an existing one), and the
  legacy log they replace stayed append-only for the same reason.

## Fields (`wake_schema: "verify-wake-v1"`)

| Field | Meaning |
|---|---|
| `receipt_id` | stable ID; a duplicate ID is the SAME event, never a second one |
| `repo`, `brief` | owner/repo and `<stream>/<NN>` the receipt is for |
| `verifier` | the TRUSTED-WRITER identity (the engine's RunnerID / signed verdict principal), never a free-text assertion |
| `outcome` | observed outcome (`verify-fail` / `blocked` / …) |
| `rows` | the Verify-row numbers this receipt holds (empty = the whole brief) |
| `inputs` | declared input scope: `input-key -> revision`. For a brief file's own `file:<path>` key this is the revision AS IT LANDS — the copy on the target branch at write time, which already carries this landing's own Evidence append (the writer reads it from the forge and refuses a mismatch; #882 Task step 3(c)) — never the pre-Evidence copy hashed at wake-evaluation time. For every other declared deliverable, the revision observed at receipt time |
| `tool_version` | applicable tool version at receipt time |
| `blocker_kind` | one of the closed set below |
| `blocker_ref` | issue/PR/action the blocker points at |
| `wake_predicate` | the checkable wake condition, one of the closed set below |
| `deadline` | RFC3339/date, for `declared-deadline-reached` |
| `recheck_reason` | required for `explicit-recheck-with-reason` |
| `prs` | the merged changes that delivered the brief: a list of `{repo, number, merge_sha}`, or — on a landed record, written by the writer only — `{withheld}` for a change the register's audience may not see. **Verifier-asserted, writer-dereferenced** (see "Join keys") |
| `prs_unknown_reason` | closed set (`no-pr-delivery`); required exactly when `prs` is an explicit empty list, and valid only then |
| `verifier_tier` | `any` or `strong` — the tier the run was admitted at; never a vendor or model name. Must equal the admitted binding's tier when it carries one |
| `started`, `finished` | RFC3339 UTC; `started <= finished <= ts` |
| `rows_passed`, `rows_total` | how many Verify rows passed, out of how many the brief has |
| `row_results` | optional list of `{row, result}` (`pass` / `fail` / `could-not-run` / `deferred`) covering ALL `rows_total` rows; its length equals `rows_total` and its `pass` count equals `rows_passed` |
| `dispatch_ref` | optional `<claim_key>@<YYYYMMDDTHHMMSSZ>.<12 lowercase hex>` (prefix 1-226 printable ASCII bytes, no `@`). **Self-reported and stays so** — the writer checks the shape, never that the dispatch happened |
| `dispatch_ref_sha256` | written by the writer only, on a private register, in place of `dispatch_ref`; never valid input |

Input revisions cover the **Verify definition**, the **relevant deliverable/dependency
revisions**, and the **applicable tool version**. Keys are opaque to the evaluator; the
already-authorized reader interprets them. The offline reader understands `tool`,
`file:<repo-relative-path>`, and `scope:<canonical-declaration>`. Each scope digest
records the admitted declaration's path kinds and sorted membership, including a
literal planned path's absence. Directory/glob member additions or removals and
planned-file creation can therefore wake work without a pre-existing content key.
File keys still hash the original admitted observation Source; the brief's key
still hashes landed Evidence, and the supplied tool-version contract is retained.

The [declaration contract](../../declared-input-scope.md) is shared by derivation,
all-file coverage, Source-to-Evidence shape checks and the root reader. A new
declaration, kind or membership between Source and Evidence requires fresh admission
and observation. No target-branch repair replaces the original observed bytes.
Historical receipts stay immutable. If today's declarations need scope keys a
historical receipt lacks, the root reader reports could-not-check and requires a
fresh admission/observation; it cannot assert an unchanged hold. An independently
observed input change still fires even when another scope could not be checked.
Older readers encountering the new scope keys report could-not-check on unknown
keys. Custom readers must opt into the optional scope-completeness interface to
provide this authoring-aware check; opaque reader semantics otherwise stay intact.

### Blocker kinds (closed set) → next actor

`implementation` → worker · `check-definition` → brief-author · `human-action` → human ·
`environment` → operator · `unknown` → verifier (reclassify). A value outside the set makes the
receipt incomplete.

### Wake predicates (closed set)

- `relevant-input-changed` — a declared input's current revision differs from the recorded one.
- `referenced-action-completed` — the referenced external action has completed.
- `declared-deadline-reached` — a declared calendar deadline is at/after now.
- `explicit-recheck-with-reason` — an operator asked for a recheck, with a stated reason.

## Join keys

A new `verify-wake-v1` record also carries the keys that join an outcome to what delivered it and
to the run that produced it: `prs` (or an explicit `prs: []` with `prs_unknown_reason`),
`verifier_tier`, `started` and `finished`. The other join keys are optional. A legacy-shape
`verified` record needs none of them — but any it does carry are checked exactly as below.

- **`prs` and `dispatch_ref` are verifier-asserted.** The writer dereferences each `prs` entry
  against the forge before landing — the change exists, is merged, its merge commit equals
  `merge_sha`, and that commit is in the record's history — and refuses (exit 5) the first entry
  that fails one of those checks, each with its own refusal. A forge read that cannot be made
  (error, a merged change with no merge commit reported, an unparseable change URL, a token that
  cannot read the repository) is could-not-check (exit 6), never a pass. GitHub answers 404 for a
  private repository the token cannot read, so a not-found answer for an entry whose repository is
  not the register is could-not-check (exit 6), not "does not exist"; only a not-found answer in the
  register's own repository is the exit-5 refusal. `dispatch_ref` is only
  shape-checked.
- **"Ancestry" by forge.** On GitHub, a change in the register's own repository must have its merge
  commit as an ancestor of the record's `sha`; a change in another repository, an ancestor of that
  repository's default-branch tip. GitLab has no compare read here, so the writer instead requires
  the change to have merged into the repository's default branch no later than the record's `ts`.
  That fallback is WEAKER: it does not show the record's `sha` contains the merge commit.
- **Key uniqueness.** A record in which any JSON object repeats a key, or carries two keys that
  differ only in letter case (`brief` / `Brief`), is refused naming the key — the record would
  otherwise resolve to one value in the writer's check and another in the next reader. Keys are
  spelled exactly as in the table.
- **Failure records.** A `verify-fail` or `blocked` record must carry a COMPLETE receipt: the
  writer names the first missing field, in the order `wake_schema`, `receipt_id`, `blocker_kind`,
  `wake_predicate`, then the predicate's own input (`inputs`, `blocker_ref`, `deadline` or
  `recheck_reason`). "Complete" means the receipt is well-formed in this sense and, on a
  `verify-wake-v1` record, that the join keys above are present.
- **Redaction.** The writer rewrites every record for the register it lands in, from the register's
  LIVE visibility read at write time (an allow-list: any unrecognised or unreadable value is
  treated as non-private, and a read error is exit 6):
  - a **private** register keeps `prs` in clear (repository names spelled as the forge spells
    them) and replaces `dispatch_ref` with `dispatch_ref_sha256`, the sha256 of the full ref;
  - any other register keeps a `prs` entry in clear only when its own repository is exactly
    `public`; every other entry becomes `{withheld: sha256("<repo>#<number>@<merge_sha>")}`, and
    `dispatch_ref` is dropped and not hashed.

  Redaction runs before the outbound content scan, so a private repository named in `prs` or
  `dispatch_ref` is rewritten, not refused. Only those two fields are rewritten: a private name in
  any other free-text field (for example `blocker_ref`) is not, and the outbound content scan or the
  public-repo gate refuses the record instead. A private register keeps every `prs` entry in clear,
  on the assumption that the register's audience may see every repository the verifier delivered to.
  A rewritten record is re-encoded compactly with its object keys in sorted order and without HTML
  escaping of `<`, `>` and `&`, so its bytes may differ from the submitted ones in key order and whitespace; a record
  with nothing to rewrite lands byte for byte. Caveat: the register's visibility is read at write time only. A private
  register later made public carries its earlier records in its git history, with the clear `prs`
  and the `dispatch_ref_sha256` digest.
- **Target tie.** Redaction must not move a record: the brief and `ts` of the written record equal
  the submitted ones byte for byte, and the final path is still the one the verifier's attestation
  admitted. Otherwise the landing is refused (exit 5) and nothing is written.

## Evaluation — the four states

The evaluator (`deskkit.WakeReceipt.EvaluateWake`) is pure: all external observation arrives
through an **already-authorized reader** (`deskkit.WakeInputs`) and the clock is injected. This
work adds **no production probes** — the offline reader answers revision queries from the local
tree (content hashes + the tool version) and NEVER observes an external action (that stays
could-not-check until an authorized online reader supplies it).

| State | When | Scheduling effect |
|---|---|---|
| **hold** | a complete receipt whose wake condition is not met | a VISIBLE `wait` row naming the blocker + next actor; excluded from costly dispatch |
| **fire** | the wake condition was met (input/tool/Verify-def changed, action completed, deadline reached, explicit recheck) | dispatch the affected work |
| **could-not-check** | a declared input could not be read | surfaced as could-not-check — never rounded up to unchanged, never an empty queue or a pass |
| **unclassified** | a legacy or incomplete receipt (missing schema/id, unknown vocabulary, or an empty input scope for `relevant-input-changed`) | eligible for exactly one ordinary classification pass; never a fabricated hold |

Invariants the tests pin (`tools/desk/cmd/verifyloop/wake_test.go`, `tools/desk/internal/deskkit/verifywake_test.go`):

- An unchanged receipt stays a WAIT across a process restart (the receipt lives in the record;
  the classifier is stateless).
- An **unrelated** commit does not wake work when the declared inputs are known unchanged; an
  **incomplete input scope cannot establish unchangedness**, so it refuses to claim a hold.
- A **partial** hold lets a newly-runnable row dispatch while the held rows are recorded as
  explicitly unrun — and **no partial result closes the whole brief** (Land flips only on a
  whole-brief PASS).
- A duplicate `receipt_id` (the latest record per brief wins — the newest `ts`, never file or
  line position) does not create another event.

## Migration

- **No backfill.** Records landed before the join keys existed are not rewritten; a reader treats
  every join key as absent on them.
- **Legacy rows** (pre-v1) stay valid and are read exactly as before; they classify as
  `unclassified` (one pass), which reproduces today's behaviour — the first complete receipt a
  run produces begins the wake scheduling.
- **Rollback** is the previous released binary/skill pin: the additive fields are ignored by the
  older reader, and receipts are never erased to make a rollback look clean.
- **Explicit recheck** is the operator's escape hatch: an `explicit-recheck-with-reason` receipt
  wakes the work regardless of the other predicates, with the reason recorded.
