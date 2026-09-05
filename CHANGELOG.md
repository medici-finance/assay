# Changelog

All notable changes to this repository — the canonical, releasing home for the
shared Assay tools (statusgen, desk-tools, drainloop) and the `assay` plugin —
are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this repo versions the
whole umbrella with a plain `vX.Y.Z` tag (see `.github/workflows/release.yml`),
so one section covers every shipped tool at that version.

Every notable change records itself as one small **fragment** file under
`changelog/` (`changelog/<slug>.md`) BEFORE it merges — one file per PR, so
concurrent PRs never collide on a shared section (the `changelog-check` CI leg
enforces this; a genuinely non-notable PR carries the `changelog:skip` label). At
release time the release workflow AGGREGATES the fragments (sorted, deduped) into
a dated `## vX.Y.Z — <date>` heading here and into the published release notes,
then clears `changelog/` — descriptive highlights, never a raw commit list. See
`changelog/README.md`.

## Unreleased

Pending notable changes are recorded as one-file-per-PR fragments under
`changelog/` (see `changelog/README.md`), aggregated into a dated section
here at release time. This section is written only by the release workflow;
do not add highlight bullets to it directly.

## v0.26.0 — 2026-09-05

### Added
- **Board-vs-witness drift comparator (interim).** `statusgen regen --readmes`
  with an online `--repo` folds the PR witnesses through the lifecycle derivation
  and prints a NOTICE for every board cell that disagrees with the derived state,
  making drift visible before a cell is hard-flipped to witness-written. Offline it
  is inert — a could-not-check is never rendered as a drift.
- **Generated Briefs table in stream READMEs (`board: generated`).** A stream
  README whose frontmatter carries `board: generated` opts its Briefs table into a
  marker-wrapped generated region (`<!-- statusgen:briefs:begin -->` …
  `<!-- statusgen:briefs:end -->`). The new `statusgen regen --readmes` verb
  writes the authoring columns (#, title, wave, effort) from the brief frontmatter
  and is idempotent; a hand edit to those columns is a `statusgen --lint` PROBLEM,
  the same single-writer discipline `STATUS.md` already has (rule 47). Everything
  outside the markers is left byte-for-byte untouched.
- **REQUIREMENTS register** — a third append-only register recording what the product was
  asked to do: the ask, who asked, an ordered `impact` axis, acceptance criteria a Verify
  row could be written against, and a `proposed → accepted → satisfied → withdrawn`
  lifecycle. Entries are per-entry files under `docs/streams/requirements/<slug>.md` with
  `REQ-<slug>` ids. Specified in `spec/registers-v1.md` §6; `statusgen --lint` parses and
  shape-validates every entry, and flags a missing or out-of-vocabulary `impact` by value.
- **Standing truth-suite workflow.** Runs the repo's test corpus plus the full release
  mutation gate — all seven specs, including the deskmerge sweep (unsharded here, since
  this suite is push+daily rather than a per-PR long pole) — on push to the default
  branch and on a daily schedule, reporting three-state. Delivered staged at
  `ci/staged-workflows/truth-suite.yml` because no App holds workflow-push permission;
  a maintainer promotes it to `.github/workflows/truth-suite.yml` to activate it.
- **Test policy independent of any one brief (`docs/test-policy.md`).** A methodology-level
  policy that guards the baseline the per-brief Verify table does not: it names the four test
  **tiers** (unit / integration / live / drill) and maps each onto the existing Verify row
  `Class` vocabulary, states the **regression floor** as a property (a merged change may not
  reduce the set of behaviours the standing suite asserts) and explains why a coverage
  percentage is not that floor, classifies **flakes** into three actioned classes with no
  "ignore" bucket (a quarantined test reports `could-not-check`, never pass), defines the
  **standing truth suite** as the CI-owned baseline distinct from delta-probing Verify rows,
  and adds **plan-in-PR** for effort-M/L briefs as a rework signal that is not a gate.
- **`pdfingest` skill** — first-pass ingestion of PDFs and office documents
  (DOCX/PPTX/XLSX/HTML/EPUB) into LLM-ready markdown via Docling: layout-aware
  reading order, real table structure, and OCR for scans. Two-tier by design —
  deterministic Docling extraction first, a vision pass only where Docling's
  output is insufficient (figures, dense math, mangled layout). Ships a
  `docling-serve` client (`plugins/assay/scripts/pdfingest.sh`) and a
  self-contained `SETUP.md` covering all four install rungs.
- **`satisfies:` on a brief** — an optional `brief-v1` frontmatter key citing the
  requirements a brief was written against (`REQ-<slug>`, or `<alias>:REQ-<slug>` through
  the existing repo-alias registry). It rides the existing `brief-v1` schema, so no pinned
  consumer has to be upgraded to keep linting a tree that uses it.
- **`statusgen newbrief`** — the brief-authoring front door (mistake-proofing/05, B1): a generator that emits a lint-clean brief skeleton so the fields the format DERIVES stop being typed. The gate is computed from the four risk questions and never accepted as a supplied value — in non-interactive mode an unanswered risk question is a refusal, not a defaulted "no". The wave is derived from `--depends` (a nonexistent dependency is refused, never a dangling edge), the inverse `unblocks:` edge is written into every named dependency in the same change (atomically — all targets or none), and the freshness stamp is produced by a fetch the tool performs (a failed fetch stamps nothing and reports could-not-check, never an invented value). It never overwrites an existing file, and refuses a Verify command that carries no code span, does not tokenize, or holds an unsubstituted placeholder. The `author-brief` skill now points at it as the "start here" step. It is a source-level device that sits ALONGSIDE the earlier mistake-proofing lints, not a replacement for any of them.
- **deskcomms cross-cell verb allow-set** — the coordinator-to-coordinator
  (the-desk ↔ the-desk) lane now carries the four ruled read-only/advisory verbs
  `status`, `metrics`, `help-offered`, `focus-on` (previously the cross-cell verb
  set shipped empty / fail-closed). `deskcomms send` gains an identity-independent
  cross-cell-verb preflight gate that reads the compiled ACL (never a second copy)
  and refuses any other cross-cell verb fail-fast with a distinct refusal, before
  identity or parse; the lane ACL's `Allow` stays the authoritative reach + verb
  check. `--verb focus-on` is documented as advisory — the receiving desk may
  decline it. None of the four mutates state on the receiving cell.
- A `check-paired-versions.sh` guard (with an offline test) now asserts the pairing holds, both
  locally and in CI: the manifest's `plugin` must equal `plugin.json`'s `version`, every pinned
  `statusgen` and `desk-tools` tag must be the SAME tag, and every `sha256` must be 64 lowercase
  hex — so a re-pin that leaves the two files disagreeing (the drift that once shipped adopters a
  stale tool) fails the check instead of merging.
- CI now runs the forge-surface control (`TestForgeSingleConstructionSite`) on every PR, gating that no forge resolver is constructed outside the single sanctioned construction site.
- CI now runs the pin-consistency controls on pull requests, so a drifted `.assay-versions` / paired-version pin is caught in review rather than after merge.
- Cross-reference from `docs/brief-rules.md` and an honesty note in `docs/how-assay-works.md`
  recording that a Verify table proves the delta, not the baseline.
- Desk lifecycle hooks: optional `after_create` / `before_run` / `after_run` / `before_remove` commands defined in a trusted-operator `<StateDir>/hooks.yaml` (no path override), fired at `deskwt add` (after_create, fatal — rolls the worktree back on failure), `deskwt remove`/`prune` (before_remove, logged), `deskdispatch` between worktree-create and prompt-emit (before_run, fatal — releases the claim and emits no prompt on failure), and `desksupervise` on release/land (after_run, logged). Each runs via `/bin/sh` with a per-hook timeout and a secret-scrubbed env; an absent file is a no-op and a malformed one fails closed.
- File the apps-installer stream — the `deskapps` one-sitting GitHub App installer (App Manifest flow, tiers, resumable across the throttle) and `deskavatar`, designed across `design.md` and 8 briefs.
- Forge-qualified bot identity: `ASSAY_TRUSTED_BOT_SLUGS` entries may now carry a forge (`[role=]<forge>:<slug-or-login>[:id]`), and commit-identity checks derive the expected author address per-forge — GitHub keeps the bot-user-id noreply form, GitLab is matched against the service-account noreply shape and never stamped with a GitHub-shaped address. An entry whose declared forge disagrees with the repository's is refused before any credential is read; an unqualified entry is read as `github` with the inference recorded so inferred and explicit stay distinguishable.
- New shared guardrail `default-forward-reversibility` across all five desk-role skills (the-desk,
  worker-desk, pr-review-desk, verify-desk, intake-desk) and `.claude/guardrails/GUARDRAILS.md`. It
  encodes the driver's reversibility test: before parking an item on the driver, ask whether a wrong
  guess is still caught by a gate the driver controls (a draft PR awaiting merge, a filed issue, a
  flip a human must still make). If yes, the desk default-forwards — authors, dispatches, opens the
  DRAFT PR, makes the best-guess call, and NOTIFIES, filing the `needs-decision`/`question` issue as
  a notification rather than a park. It stops only for a fixed one-way / outside-the-gate set (merge,
  a ready-flip that is not the role's, an unauthorized `main` push, a tag or release, weakening a
  security control, secrets/PII/exploit exposure, money movement, identity/auth changes, durable-data
  loss, and anything that leaves the repo).
- Per-run stop signal: a `STOP.run.<key>` flag halts a single dispatched run without touching any other, checked below the loop-wide `DISABLED` > `STOP` > `STOP.<loop>` precedence (it can only add a refusal, never mask a loop-wide one). The run key is recorded on the worktree at dispatch, so every desk verb reads it from cwd with no agent cooperation. `desksupervise stop <key> --reason R` arms it (audited) and `status --stops` lists armed run-stops; the worker-desk sweep reads them and stops the matching worker via the new `stop-worker` capability.
- Requirement **traceability** is now checked. `statusgen --lint` runs three corpus-wide checks over the REQUIREMENTS register and the `satisfies:` brief citations: `orphan-requirement` (NOTICE — an `accepted` requirement no brief cites), `untraced-brief` (NOTICE — a forward brief in a `traced: true` stream that cites nothing), and `dangling-satisfies` (PROBLEM — a `satisfies:` naming a `REQ-<slug>` no register entry defines). The two NOTICEs never change the exit code, so a corpus authored before the register existed is never red-gated.
- Run-time non-author verdict assertion (`deskkit.AssertNonAuthorVerdict`, wired into `deskpost review` / `security-review`): before a verdict is posted, the desk tool compares the posting identity against the author of the head commit and refuses on equality, naming both. It is a second, independent layer behind the forge's own "an author cannot approve their own PR" refusal — a different component (the desk tool), a different time (verdict-post time), and a different signal (identity equality against the certified head) — so it catches the collapse a reduced-identity install can create, where the forge's authorship-keyed refusal may not fire. The check is three-state: an unreadable head-commit author falls back to the PR author and warns, never a silent pass.
- Stream READMEs may opt into the `untraced-brief` check with a `traced: true` frontmatter key; absent it, the check never fires over that stream.
- The four standing desk roles (`the-desk`, `pr-review-desk`, `intake-desk`, `verify-desk`) now name the `capability:durable-monitor` binding in their liveness-contract standing-loop and watcher-window prose — so the durable cross-turn wake each window relies on reads as a named capability (with its best-effort / fixed-cadence-sweep backstop) rather than plain prose.
- The umbrella release now cross-compiles Windows binaries. `release.yml` builds
  `statusgen-windows-amd64.exe` / `statusgen-windows-arm64.exe` and packages
  `desk-tools-windows-amd64.tar.gz` / `desk-tools-windows-arm64.tar.gz` (each `cmd/*`
  binary suffixed `.exe` on the windows legs only), all on the existing Linux release
  runner — Go cross-compiles the Windows targets natively, no Windows host. `checksums.txt`
  now covers all ten assets, so consumers can pin each Windows platform by sha256 in
  `.assay-versions`; the adopter-scaffold example gained illustrative
  `statusgen-windows-amd64` / `statusgen-windows-arm64` pin lines (placeholder hashes —
  the real ones are harvested from the published release). Unblocks the Windows install
  path, CI leg, and adopter-doc work (windows-port/03–05).
- Verify-lane activation: an evidence-automerge path plus the verify-gate open/close workflows land the verifier's Evidence rows behind reviewer approval and the leak-sweep gate, and a front-door drift gate plus a board-reconcile schedule keep the generated board honest. A `changelog-check` workflow now requires a `changelog/` fragment (or the maintainer `changelog:skip` label) on every notable PR.
- `deskclaim` gains a fail-closed branch-liveness probe and a read-only `stale` verb, so a claim recorded with `--branch` can be reclaimed through the tool (under the directory-wide lock) instead of a hand-delete that bypasses it. A branch is INACTIVE (reclaimable) only when every readable signal — a worktree checkout and the owner's roster beacon — says so; any unreadable signal is treated as ACTIVE. `stale` reports 0 (none) / 5 (live) / 6 (unreadable) and never mutates the claim.
- `deskgit push --as <role>` and `deskgit fetch --as <role>` — authenticated git transport from a role's App token file, the sanctioned replacement for hand-retyped credential-helper recipes. Push is fixed to the current branch (never main or a detached HEAD), refuses `--force`/`--delete`/`--prune`/`--mirror`/`--tags`/`--no-verify` by name, and never writes the token to the audit line.
- `desksupervise status [--json] [--stops]` reports a read-only runtime snapshot of the supervisor: per-claim liveness (reusing the same taxonomy `tick` runs, without acting), armed run-stops, and observation state. Three-state throughout — a claim it cannot read is `COULD-NOT-CHECK` (listed in `blind_sources`, snapshot exits 6), timers show `n/a` never `0s`, and token usage is `could-not-check` by design rather than zero. `run --interval` writes the snapshot to `<StateDir>/supervise/status.json` atomically each tick.
- `desktoken coverage <role> [--repo <slug>] [--json]` — a read-only verb that lists the repositories each of a role's App installations can see. Tokens are minted into memory only (no cache, no token or JWT printed); a repo page that cannot be read is exit 6 rather than a silently-short list, and `--repo` returns exit 0/5 for seen/not-seen.
- `docs/enforcement-model.md`: a new page stating, in the honest §1a voice, what the identity separation *enforces* versus what it only *attributes* — including an enumeration of the identity set grounded in the code's `requiredDuties`/preflight, what breaks if each identity is merged into another, and the two independent layers behind the load-bearing implementer↔reviewer separation.
- `statusgen --requirements-rollup [--since <date>] [--json]` — the per-release ask→work→evidence rollup: each requirement, its acceptance criteria, the briefs that cite it with their board status and Evidence, and a three-state verdict (`satisfied` only when every backing brief is `done`, else `partial` or `could-not-check`). It reports what was authored, not re-measured — an input to `--export-evidence`, not a second bundler.
- `statusgen brief <stream/NN>` resolves a brief item key to its file path, parsed frontmatter, and board-row status as JSON (or `--text`) — read-only, reusing the same parsers `--lint` runs. Handles not-found and ambiguous keys explicitly (a duplicate or missing `brief-NN-*` exits non-zero and names what it found, with no JSON body), and accepts multiple keys in one call.
- `statusgen init` gains `--dry-run` (preview the scaffold — paths and bodies —
  without writing) and accepts the target as a positional directory; the scaffolded
  CI workflow regenerates the stream README tables alongside `STATUS.md`.
- `statusgen init` now detects the target's forge from its `origin` remote (or an explicit `--forge github|gitlab`) and scaffolds the matching CI half: a GitLab remote gets a `.gitlab-ci.yml` running the same two single-writer halves (lint on merge requests, board regen + commit on the default branch) instead of the inert `.github/workflows/assay-statusgen.yml`, and the closing next-steps text names whichever file was actually written (#349).
- `tools/ci-load/activation/` stages the workflow half of a per-push CI fan-out reduction (`desk-supervision/09`): post-change copies of `ci.yml`, `plugin-drift.yml`, `assay-statusgen.yml`, `assay-qualgen.yml` and `evidence-automerge.yml`, a unified diff, and a README with the copy command, verification and rollback. The identity that authored it holds no `workflows` permission, so landing them is a human copy. `tools/ci-load/pathsemantics.py` is the offline negative control for the filters: it reads the `paths-ignore` list out of the staged files and asserts GitHub's "skip only when EVERY changed file is ignored" rule on five diff shapes, three of which must not skip.
- `tools/create-fleet-gitlab.sh` now provisions two free-tier project controls it previously left unset (issue #346 comment 1 §4): **protected release tags** (`POST /projects/:id/protected_tags` with the scalar `create_access_level: 40` — never the Premium `allowed_to_create` array that caused this issue's original 400 — so only a human owner can create or move a tag) and the **all-discussions-resolved merge gate** (`only_allow_merge_if_all_discussions_are_resolved: true`, set in the same `PUT` as the pipeline gate). Both are idempotent/read-back checked: the protected-tags rule is a named no-op when it already exists, and both merge-check fields are read back off `GET /projects/:id` and reported three-state (a value that did not take is a recorded failure, a value that could not be read is could-not-check, not a pass).
- `tools/pairedversions` — a fail-closed guard for the plugin↔statusgen pairing, so the
  re-pin cannot be skipped silently again. It asserts that `plugin.json`'s version matches
  the manifest's `plugin`, that each paired tag is a *published* release of its release home,
  and that every pinned sha256 equals that release's own `checksums.txt` entry. A
  could-not-check reddens the run rather than passing it, and one invocation reports every
  disagreement at once. `make paired-versions` runs it; the CI workflow that makes it a gate
  is staged for a human to land at `tools/pairedversions/activation/plugin-drift.yml`.

### Fixed
- A lifecycle hook that outruns its `timeout_ms` is now killed as a whole process group, and `RunHook` no longer blocks past the timeout waiting on an output pipe a surviving grandchild still holds open. Previously the kill reached only the `/bin/sh` the hook was launched as, so on any host whose `/bin/sh` forks rather than exec's the last command (Debian `dash`, as in the Linux CI image), the hook's own children ran on unreaped and a 200ms budget could take the full length of the hung command to return.
- Content a register directory holds that no parser reads is no longer invisible. Because stream discovery skips the register, a file that is neither a requirement entry nor the register's `README.md` — a loose non-Markdown file, or a subdirectory — was read by nothing at all; each one is now a single PROBLEM naming the file. Dot-prefixed housekeeping files such as `.gitkeep` are not register content and stay silent.
- The adopter front door no longer installs a stale tool. `plugins/assay/paired-versions.yaml`
  had been left pinned for plugin `0.4.0` and statusgen `v0.13.0` while the shipped plugin
  moved to `0.5.0` — so a clean `assay:install` resolved a statusgen many minors behind the
  skills it ships alongside. The manifest is re-pinned to plugin `0.5.0` / statusgen `v0.25.1`,
  with every per-platform sha256 refreshed from that release's published `checksums.txt`.
- The dead-claim decay pass no longer reports its GitHub-only nature as a transient "unavailable this run" NOTICE on a GitLab remote. On a definitively non-GitHub (`gitlab`) `origin` it emits a distinct "NOT APPLICABLE on this forge" message and skips the `gh` shell-out entirely, so a GitLab adopter's lint no longer reads green while implying a CLI authentication that would never help (#349).
- The floor's present-but-UNREADABLE refusal now names the CAUSE it found instead of listing every cause it might have found: the untrusted-applier case names the login that applied the stamp and the dispatcher identity the floor would have accepted, and the content case says which half of the stamp is missing or conflicting. The two have different remedies (re-stamp the PR vs. correct the labels), and telling them apart previously meant reading the timeline API by hand.
- The missing-fragment CI failure now names the exact fix — it derives the suggested `changelog/<slug>.md` path from the PR head branch and prints a copy-pasteable `printf … > changelog/<slug>.md` command plus a `MISSING:` line with the base/head SHAs — instead of only saying a fragment is absent.
- The model-capability floor now trusts the dispatch attestation the dispatcher itself writes. `deskdispatch`'s stamp step shelled out to `gh` with no App token, so the `dispatched-model:` / `dispatched-tier:` labels landed under whatever credential the calling shell held — another role's App, or the operator's own login. The floor's applier-aware reader (in `deskpost review`/`ready` and `deskflip`) accepts a stamp only from the App bound to the dispatcher role, so it read every stamped PR as a non-dispatcher stamp and refused it, while an *unstamped* PR proceeded on the absent-attestation NOTICE: present-but-untrusted was worse than absent, and review verdicts and ready-flips could only be completed through the loudly-logged incident-recovery override. The stamp step now mints the dispatcher App's installation token and applies both labels under it, and refuses to apply a label at all when that identity cannot be established (no stamp is safer than an untrusted one). The floor itself is unchanged and was not loosened; the dispatcher role is now a single declared constant (`deskkit.DispatcherRole`) that both the writer and the reader project from, so the two cannot name different identities again.
- `deskboard` no longer reports a trusted human maintainer's own open PR as
  review-desk neglect. A non-draft PR with no reviewer-App verdict at head used to
  classify `NEEDS-REVIEW` and, after 30 minutes, trip the `UNREVIEWED` neglect
  alarm on every sweep — a false alarm that recurred for every human-gated brief
  closure (the maintainer fills the decision table in their own PR and merges it;
  the review desk deliberately does not dispatch a model reviewer on a human's own
  ratified ruling). Such a PR now classifies as a distinct `HUMAN-OWNED` row ("the
  author owns and merges it") and is kept out of the `NEEDS-REVIEW`/`RE-REVIEW`
  dispatch gate and the `UNREVIEWED` count, while its CI/mergeability columns still
  render. Only the accountable-human set qualifies (the mapped humans of
  `ASSAY_HUMAN_LOGIN_MAP` plus the blessing authority) — App-authored and
  shared-machine-account PRs stay `NEEDS-REVIEW` and remain in the neglect metric.
  The `reviewloop` reactor gives `HUMAN-OWNED` a matching no-op disposition.
- `deskboard`'s freshness banner no longer reports a permanent `STALE-UNKNOWN` when
  run from a consumer checkout. The drift check keyed on the in-tree
  `origin/main:tools/desk` git ref, which stopped resolving once the desk tools moved
  out of consumer trees to their release home — so every consumer run was
  could-not-check forever, unable to tell "the binary is behind the source" from "the
  source is not in this tree". The check now compares the running binary's embedded
  `releaseTag` against the `desk-tools` tag the consumer pins in its own
  `.assay-versions` (the source that resolves where the binary actually runs), keeps
  the in-tree `tools/desk` ref as a fallback for the source repo, and reports
  `could-not-check` only when neither source resolves — naming which one was missing.
- `deskboot`'s board summary reported "no Next-up section" on every boot: `summariseBoard` matched the heading as the hyphenated `next-up`, but statusgen emits it as `## Next up` (a space), so the section was never found and the boot line understated the queue as empty. It now accepts both spellings, and the regression test asserts row counting under the real `## Next up` heading so the two tools stay locked together.
- `deskflip`'s single-PR read query (`flipPRGraphQL`) had an unbalanced brace (one extra `}`), which `gh api graphql` rejected at parse time — the pr-open-draft condition failed closed on every PR, so no flip could run. Removed the extra brace and added a hermetic delimiter-balance test over both GraphQL query constants (`flipPRGraphQL`, `openPRsGraphQL`), since the gh-stubbing suite never executes the query strings and could not catch a brace typo.
- `deskpost`'s unit suite (`cmd/deskpost`) is now deterministic regardless of the checkout it runs in. Its fake harness pins every allowed repo's forge in the fixture roster (`ASSAY_REPO_FORGES`), so `ForgeFor` resolves the forge from configuration (resolution step a) instead of falling through to the ambient `git remote get-url origin` of whatever checkout ran the suite. Previously every write-path test (comment/review/ready) passed only where the running checkout's origin host mapped to `github.com` and returned Unverifiable (exit 6) everywhere else — e.g. an offline worktree whose origin was an unrecognised host — which is how 18 tests were red on `main` while CI (a github-origin checkout) stayed green. No production verification path changed; the resolver's fail-closed behaviour is unchanged. (#415)
- `deskpr create/update/edit` now mint the token for the session's own App role (resolved from the loop identity) instead of always the worker App. Under `DESK_LOOP=verify-desk` this mints the verifier App, so verify-desk Evidence PRs keep the correct verifier authorship (PR author == Evidence-commit author). The worker App remains the default only when no loop role is set, and that fallback is announced rather than silent.
- `deskpushguard`'s register-id collision check no longer refuses a legitimate push when a branch MODIFIES an existing `docs/streams/findings/` or `docs/streams/intake/` entry that an in-flight sibling branch also touched at the SAME path. Two branches editing one file is a merge concern that resolves to a single file on merge — it never produces the duplicate id statusgen's authoritative gate reds on — so flagging it as a register-id collision over-fired and blocked valid pushes. The guard now skips the identical-path pair while still refusing the genuine added-vs-added case, where two DIFFERENT files independently claim the same id.
- `deskpushguard`'s register-id collision check no longer refuses a push that merely edits an existing findings/intake entry. It now treats an `id:` as a *claim* only when the id is NEW relative to `origin/main` — an id already present on `origin/main` in the same file (the entry was edited for an unrelated reason, e.g. a repointed backtick, without touching its id) is a pre-existing entry and cannot collide. It also confirms a colliding sibling ref is still LIVE on `origin` (a single `git ls-remote`, run only on the rare collision path) before refusing, so a stale remote-tracking ref left by a merged-and-deleted branch is dropped instead of reported; each surviving refusal now states whether the source ref is live or its liveness could not be verified. Previously any branch touching an existing entry collided with every remote-tracking ref that carried it — an unsatisfiable refusal, since renaming an id already on main is the actual defect.
- `desktoken` now accepts group-readable private keys (0440/0640) so a Secret-mounted key in a non-root pod — root-owned, read through `fsGroup`, therefore necessarily 0440 — can mint instead of failing closed with exit 6 on every tick. The check is now a bit rule, not the literal 0600: a key that is readable by others or writable by group/others is still refused, and the refusal names the rule and the observed mode. The token cache and GitLab PAT custody file the tool writes itself keep their exact-0600 checks. (#388)
- `deskwt add` no longer counts a `prunable` (directory-gone) worktree as a live branch holder — such stale registrations are skipped so a same-name add can reclaim the branch, while a branch that is genuinely checked out somewhere or carries unpushed commits is still refused.
- `statusgen --lint` no longer walks the REQUIREMENTS register directory as a stream. A tracking root whose `docs/streams/requirements/` holds entry files and no `README.md` was reported as `stream directory requirements has no README.md` and failed the lint, even though a register carries per-entry files and never a stream README with a brief table. The register is a reserved name in stream discovery, and the whole-tool lint now pins that: a README-less register lints clean, and a register README is allowed where present rather than required.
- `statusgen --scan-issues` no longer reports a partial scan as a clean board. A run that could not read one or more of its configured repos (GitHub rate limit, a 404/unresolvable repo, or an auth failure) previously exited `0` printing the byte-identical `no changes — nothing to create or retire` line, so a cron or desk could report a clean intake lane through an entire rate-limit window while the placeholder backlog silently regrew. Such a run now exits `2` (statusgen's could-not-check code), suppresses the clean-board line, and every run — even a genuinely empty one — prints a `read N of M configured repos` summary that names each unread repo and why it was skipped.
- deskkit circuit breaker no longer trips a healthy *quiet* loop. A `noop` result — the
  tool confirming the desired state already holds, the shape of an idempotent verb a standing
  loop re-asserts on every quiet tick — is now neutral: invisible to the breaker's
  consecutive-non-progress meter, neither tripping it nor resetting it (the same treatment
  `dryrun` already gets). Previously five consecutive quiet ticks opened the breaker with
  nothing having failed, after which the refusals it produced were themselves non-progress and
  the run never reset. Only `refused`/`unwritten` now advance the run (#180).

### Changed
- No security or leak workflow is touched. `leaksweep-control.yml` and `leaksweep-pattern.yml` keep `paths: "**"`, their runner and their concurrency groups byte-for-byte, and `leak-sweep` — the only status check either branch ruleset requires — is posted by a gate outside these workflows and runs on every pull request unconditionally.
- The Ask-Assay numbers-rule layer moved out of `tools/desk/internal/askassay` to `tools/desk/askassay` (carrying `chart/` and `report/`) so another module can import it. Pure relocation plus import-path updates — no behaviour change; the single exec-site guard's ledger key was repointed to the moved file so the control stays green.
- The `satisfies:` citation and the REQUIREMENTS traceability checks move from **reserved, not gating** to gating (spec `registers-v1.md` §6.5, `brief-v1.md` §3.3). Consumers pick the checks up on their next `.assay-versions` statusgen pin bump.
- The board lint now treats the D1 MUTATION obligation as a MERGE GATE, not an
  advisory notice (mistake-proofing/06). When this branch's diff changes a
  check-shaped control a brief declares, that brief MUST carry a `+mutation`
  Verify row or the lint refuses — promoting the methodology's sharpest
  requirement (a control must be shown to fire) from a prose MUST to a machine
  gate. flow and dereference stay advisory; only mutation is promoted. The gate
  is transition-scoped by construction — only a brief whose own file the branch
  edited is evaluated — so the 300-plus inherited tables are never made fatal, and
  a diff whose branch base cannot be read fails closed (could-not-check refuses,
  distinct from "nothing owed").
- The check-shaped path set is now an explicit, narrow, rationale-carrying
  ENUMERATION in source (lint/check source, desk guard, CI workflow, reviewed
  verify script) rather than one over-firing inline regex, with its coverage
  boundary recorded beside it. The failure message names the triggering path,
  points at the `tools/desk/cmd/muhar` mutation harness as the recommended way to
  produce the demonstration, and states that it checks the row's PRESENCE not its
  adequacy — it adds a floor and does not replace the reviewer. This check carries
  its own positive control (the rule applied to itself).
- The declared fixture-corpus exemption (`.statusgen-fixtures`) is now **bounded and announced**. A marker is honoured only strictly under `docs/streams/<corpus>/`: one placed at the repo root, over `docs/`, or over `docs/streams/` itself would switch the link check and the `human:<name>` stamp scan off for every stream at once, so it is refused — inert in the resolver (so `--corroborate` is unaffected even though no lint runs there) and reported by `--lint` as a PROBLEM naming the marker and where it belongs. Every honoured corpus is now announced on every run of both `--lint` and `--corroborate` with `NOTICE: fixture corpus exempted: <path> (N files) …`, so an exemption is never silent. Only the link/backticked-path check and the stamp scan skip a declared corpus; the leak sweep, register lints, numbering-collision detection and the board generator still read every one of its files. Documented in `statusgen/README.md`.
- The desk improve-pane side-effect register no longer asserts that a `statusgen --bottleneck` write under `docs/reports/` leaves the path unclassified and drives the publication-manifest check to fail. A covering `docs/reports/**` withhold row now exists in the publication disposition manifest, so the created file classifies and the check passes — the write side effect itself, and the guard on it, are unchanged.
- The dispatched-worker prompt kit now binds Verify runs to be BOUNDED inside the
  agent: a worker runs targeted (`go test -run '<TestName>' ./<pkg>/...`) or
  single-package tests with an explicit `-timeout`, and never the whole-module
  `go test ./...` — a full-module run can overrun the agent's watchdog, which kills
  the agent mid-row and strands the work. The full suite is left to CI, which has no
  such watchdog. The same clause requires a worker to PUSH before starting a long
  Verify row, so a row that overruns the watchdog costs the row and not the branch.
  `deskdispatch`'s emitted-prompt test pins both halves of the new clause.
- The per-PR changelog fragment requirement is now stated on every implementer-facing surface, not just enforced by the CI gate: the deskdispatch worker kit gained a changelog-fragment clause (detection-based — inert where a repo carries no `changelog/README.md`), and the worker-desk, pr-shepherd, and author-brief skills each name it.
- The plugin's `paired-versions.yaml` is re-pinned to statusgen / desk-tools **v0.25.1** and
  plugin **0.5.0** — both sides of the pairing move to the same tag, with fresh per-platform
  sha256 pins harvested from the v0.25.1 release checksums.
- The staged workflows select triggers more precisely without changing what any workflow checks: `ci.yml`'s `push` leg is scoped to `main` (it fired on every branch and tag, duplicating its own `pull_request` leg 1:1) and both legs skip diffs confined to `docs/`, `changelog/`, `CHANGELOG.md` and `STATUS.md`; `ci.yml` gains the concurrency group it never had, cancelling superseded pull-request runs only; `plugin-drift.yml` gains the same filter on its `pull_request` leg while its `push: main` rot-detection leg keeps none; `assay-statusgen.yml` and `assay-qualgen.yml` cancel superseded pull-request runs while their single-writer `main` jobs stay uncancellable; `evidence-automerge.yml` no longer starts a self-hosted job for pull requests its own unchanged guard would decline. Measured on a 120-run sample: about 32 of roughly 145 self-hosted jobs removed, and a docs-only pull-request push falls from 12 self-hosted jobs to 5.
- `docs/adopting-assay.md` now states the true cost of adoption **above the fold** — accounts, the load-bearing identity pair, supported platform, and supported harness — so a reader learns the real shape on the page rather than at step four. It documents that a formally-supported *minimal* identity path (and whether a single-person single-repo adopter is supported) is a live human decision (`#463`) and deliberately does not publish a collapsed identity topology as "supported" until that is ruled.
- `escalation-labels` guardrail and resident rule R8 now say a `question` parks an item only when the
  fork is one-way; a reversible item proceeds on its stated default with the label riding on it. The
  desk boot, autonomous-drive, file-and-exit, needs-decision, and ask-decision passages are aligned
  to run the reversibility test first, so a reversible fork moves on its default instead of waiting.
- `pr-review-desk` now reviews **in parallel by default**. Within a risk-classed
  PR the correctness and security lanes dispatch in the same turn (the board's
  `SECURITY-REVIEW-REQUIRED` row is a missed-dispatch alarm, never the trigger),
  and across PRs every actionable `(PR, lane)` fills a free slot in one dispatch
  turn up to the pool width — refill fills all free slots, not one. Dispatch keys
  are lane-suffixed (`<alias>--pr-<N>` / `<alias>--pr-<N>--security`).
- `statusgen --lint` now says out loud that requirement traceability is **reserved, not
  gating**: it emits a NOTICE naming what is parsed and what is deliberately not checked.
  An absent `satisfies:` is never flagged, a citation naming a requirement that does not
  exist is not an error, and no exit code changes on either — the enforcing checks are a
  separate change.
- forge-neutral/03 brief (`docs/streams/forge-neutral/`): amended the Verify DoD to remove three internal contradictions — row 3 now forbids weakening any test assertion rather than freezing the test files (re-pointing a test's transport is allowed with a named 1:1 successor assertion), row 8 is scoped to non-test files, the permit-register ceiling is measured (`launch sites − 8`, 16 today) instead of a frozen literal, and row 11's new-transport test is exempt from the freeze.

## v0.25.0 — 2026-09-03

### Added
- **`desksupervise`** — the liveness *observer* that finally supplies `internal/loopengine`'s
  fully-coded, fully-inert liveness taxonomy (`ObservableProbe`, `LivenessPolicy`) with real
  probes. `internal/loopengine/probes.go` adds `AuditProbe`, `BranchProbe`, `PRProbe` (each
  three-state — a probe that cannot reach its source reports could-not-check, never no-life),
  composed by `HouseProbes()`, plus `ClassifyLiveness`/`Disposition`, the taxonomy re-exported
  for a reader outside the engine's own in-flight tracker. `desksupervise tick` classifies
  every `state=dispatched` dispatch claim into `ALIVE` / `NEVER-STARTED` /
  `HEARTBEAT-EXPIRED` / `OVER-WALL-CAP` / `COULD-NOT-CHECK`, releasing a wedged claim
  (`RECLAIM-ELIGIBLE`) or landing a budget-blowing one `BLOCKED-TIMEOUT` (a filed
  `help wanted` issue, never re-dispatched blind) — turning a worker stuck behind the
  120-minute stale-claim backstop into a logged, minutes-scale reclaim with no human in the
  loop. `--dry-run` classifies and prints only; `run --interval` loops `tick` forever,
  mirroring `deskwt prune --interval`. `--claims-fixture`/`--observations-fixture` bypass the
  live claim tool and the forge/audit file entirely, so the whole classification path runs
  offline. `deskkit.PullRequest` gains an `UpdatedAt` field (GitHub and GitLab both wired) as
  the forge read PRProbe needs. See `tools/desk/README.md`'s tool-reference row and
  `docs/streams/desk-supervision/brief-01-observable-probes-and-observer.md`.
- A **retrospective input feed** that emits the four-part input set — churn
  trend, gate yield, per-stage ledger, and budget status — as generated/logged
  output a cadence retrospective consumes.
- Custody: `ForgeFor` obtains the resolved role's already-minted token from the existing
  per-forge path — GitHub via the `desktoken` mint-or-reuse path (`RoleTokenForRepo`),
  GitLab by reading the `gitlab-<role>.token` file a prior rotation produced — and never
  falls back to an ambient credential. A missing or insecurely-permissioned (non-0600)
  custody file is refused, naming the remedy. `SetGitHubCustodyMinter` is an installable
  seam a caller that already mints its own GitHub App tokens in-process can plug its
  existing, tested minter into, rather than this package growing a second implementation.
- Per-stream quality **error-budgets** (`qualgen/consumers`) in an alarm
  posture: a breach raises an alarm record rather than a dashboard line, and a
  budget refuses to arm until the stream has at least two measured windows
  (could-not-measure, never armed at zero).
- The worker prompt kit (`common-clauses.md`) now carries the workpad rule: keep one
  workpad per PR, no separate done/summary comments.
- `deskkit.ForgeFor(repo, role)` — the first resolver that can hand a desk tool a `Forge`
  backend at all. Two complete backends (`GitHubForge`, `GitLabForge`) have existed with no
  constructor, no config key, and no consumer; this is that missing answer, and the ONLY
  function in the tree allowed to construct either backend (enforced by
  `TestForgeSingleConstructionSite`'s AST walk plus an independent grep, and backed by the
  existing `forge-surface-control.yml` shell-exec/passthrough CI job). Resolution reads the
  repo's forge from a new roster key, `ASSAY_REPO_FORGES` (`owner/name=github` or
  `owner/name=gitlab`, full slug only — a bare basename is refused, unlike the display-only
  `ASSAY_REPO_ALIASES`), falls back to the origin remote's host when the mapping is
  unambiguous (`github.com`/`gitlab.com` only), and otherwise refuses could-not-check naming
  the repo and the configuration that would resolve it. There is no parameter, flag, or
  environment variable by which a caller supplies the forge itself
  (`TestForgeForRejectsCallerSuppliedForge`).
- `deskpost`'s `comment` verb is wired end-to-end through `ForgeFor` as the
  proof-of-reachability: the actual `POST .../comments` call now goes through the resolved
  `Forge.PostComment`, authenticated via `deskpost`'s own existing App-token mint installed
  as the custody minter above — every precondition read on the same command still runs on
  the pre-existing client, unchanged. `deskpost` carries no `forgeban` permit row, so this
  step moves that ratchet by zero; it only proves the resolver is reachable before any later
  brief's migration claim rests on it.
- `deskreply --workpad` upserts ONE marked progress comment per PR — finds the newest
  unresolved comment authored by the worker identity carrying the `<!-- assay:workpad -->`
  marker and edits it in place, or creates the first one; `--dry-run` reports which without
  writing. Never edits a human's or a minimised comment.
- `deskroster width --role <loop> --reserve resume=N,rework=M` sets a per-class concurrency RESERVATION beside a loop's pool width, riding the same stored entry and decaying with it; plain `deskroster width --role <loop>` now prints `width=<n> reserve=resume:2,rework:0 (source=default|set, expires=...)`. `fanoutloop plan` classifies its queue into resume / rework / fresh and prints `classes: resume=<n> rework=<n> fresh=<n> (fresh capped at <k> by reservation)` — a floor that protects orphan-PR resumes and `Awaiting implementer rework` rows from being crowded out by fresh dispatch under a full pool, and never idles a slot when nothing reserved is waiting. `plan` also now sources `Awaiting implementer rework` board rows directly (previously a manual board read). worker-desk ships a default reservation of `resume=2`. `deskboard throughput` reports the same reservation as an extra column beside the width it never subtracts from.
- `internal/deskkit/workpad.go`: the marker, the fixed-section template (`Render`/`Parse`),
  and `Stamp(worktree, sha)` for the environment-stamp line — never a machine path.
- `qualgen sweep` — a standing, current-tree code-slop forensic sweep lane:
  configured external linters nominate suspects (leg 1), a pluggable
  `AgentVerifier` adjudicates each new suspect with emitter-side evidence
  enforcement (leg 2), and an evidenced, report-only markdown artifact is
  rendered per run (leg 3). Incremental by fingerprint — a rerun over an
  unchanged tree re-verifies nothing — and read-only against the target repo.
  Ships an offline scripted `Fixture` reference verifier; a live coding-agent
  adapter is configuration.
- `qualgen` closes the quality loop: a pluggable issue-filer (`qualgen/filer`,
  with a GitHub Issues reference adapter and a first-class dry-run) turns
  above-threshold hotspots and duplicate-block clusters into **advisory,
  budgeted** refactor items — one per distinct target, degrading to dry-run/log
  once the filing budget is spent, and never self-dispatching work.
- `statusgen --assayscore --json`: a composite **AssayScore** — the geometric mean of four 0–100 sub-scores (Speed, Value, Flow, Quality) computed from the existing brief-flow metrics. Speed and Value normalize against trailing-90-day bands; Flow and Quality are bounded ratios. A dimension that cannot be measured is **excluded** from the mean (never coerced to 0), and the composite is flagged `incomplete` when any dimension is missing — an honest three-state read rather than a silently deflated score.
- `statusgen` §8 lifecycle-routing support: a `**Status:**`/`**Routes-to:**` header reader plus the §8.1 grammar, §8.3 Routes-to, and §8.5 owed-detector lint rules (each finding carries a stable `[rule-tag]`), and a new `statusgen --owed-issues` emit-mode that files one marker-deduped issue per approved-but-uncited routing doc (idempotent, part of the `--decision-issues` family). Ships `docs/workflow-templates/authoring-owed.yml`, an adopter-installable main-push watcher for the emitter. Unclassified/legacy specs are ignored, never rounded up, so `--lint` stays green on existing trees.

### Fixed
- Desk-tools reclaim and house-PR probe paths now obtain their git-forge backend through the single resolver (`ForgeFor`) instead of constructing a GitHub backend directly, restoring the single-construction-site invariant its release-gating test enforces. Behavior is unchanged for GitHub repos (the same per-owner installation token is used); forge kind is now resolver-determined rather than hardcoded.
- `deskboard` and `deskflip` no longer fail closed under a `checks:read`-only identity
  (the reviewer App). gh's built-in `statusCheckRollup` JSON field selects a
  `checkSuite.workflowRun` sub-field — a link to the Actions run, not a check conclusion —
  that requires `actions:read`; under an identity without that scope it 403s and takes the
  whole read down with no salvageable output. `deskboard`'s bulk open-PR read (`prs` /
  `actions`) then exited 6 on the first repository alphabetically, blinding the entire
  cross-repository board, and `deskflip`'s single-PR state read refused to flip any private
  PR. Both reads are now hand-authored `gh api graphql` queries that request the status
  rollup contexts WITHOUT `checkSuite`/`workflowRun`; every conclusion these tools classify
  on (`CheckRun.status`/`conclusion`, `StatusContext.state`) is covered by `checks:read`
  alone, so neither read depends on a scope the tool's identity is not guaranteed to hold.
- `release.yml`'s changelog roll (write the dated section, clear the fragments) now commits and pushes under the assay-board-writer App — the identity that already writes STATUS.md straight to `main` and carries the ruleset bypass — instead of the default `GITHUB_TOKEN`, which the PR-only + leak-sweep-required ruleset rejected on v0.23.0 and v0.24.0 and left the roll to be hand-filed as a PR each time.
- `rosterconfig.go`'s known-key set, echo, and refusal message all recognise the new
  `ASSAY_REPO_FORGES` key, so a deployment that sets it does not fail the whole roster
  closed on the unregistered-`ASSAY_*`-key refusal.

## v0.24.0 — 2026-09-03

### Added
- **`clusterguard`** — an exec-boundary shim for cluster CLIs. Installed as a directory of
  symlinks (`kubectl`, `flux`, `helm`, `talosctl`, `k9s`) on the front of a session's `PATH`, it
  refuses every shimmed CLI by default with exit `5`, records both verdicts to
  `<config-home>/clusterguard.log`, and execs the real CLI only when an operator shell exported
  `ASSAY_ALLOW_CLUSTER` — `=1` for read-only verbs, `=mutate` for everything, any other value
  refused rather than guessed. This catches what a command-text permission rule cannot: a cluster
  call made from inside a committed script never matches a text rule, but it still resolves the
  CLI name on `PATH`. Read-only classification is a per-CLI **allowlist**, so an unclassified verb
  is treated as mutating; `k9s` has no read-only lane at all, being an interactive TUI that can
  mutate from inside the session. A stop flag can only make the guard stricter — an armed kill
  switch refuses (exit `3`) rather than making a refusal-guard stop intercepting, which would fail
  open. Its limits are stated rather than implied: an absolute-path invocation is never
  intercepted (there is a test asserting that bypass exists), and the guard is not a network
  boundary. Contract, verdict table and limits: `tools/desk/README.md`; install notes:
  `docs/adopting-assay.md`.
- A `forge-neutral` planning stream: a waved eleven-brief phase plan for making the desk verbs the only sanctioned forge write path on **both** GitHub and GitLab. It starts from a measured matrix — verb × forge-path × identity-assumption, cited by `file:line` — whose head finding is that `deskkit.Forge` has two complete backends and **no production consumer at all**: neither `GitHubForge` nor `GitLabForge` is constructed anywhere outside tests, no resolver exists to pick one, and the only `--forge` selector in the suite is a session-settable custody switch on `desktoken`. The plan is scored against the in-tree permit register's ratchet (`forgeban`, ceiling 24), which it drives to 10 with the ten surviving rows named rather than left implicit. It delivers a config-resolved forge contract with refusal-never-fallback semantics, forge-qualified trust-roster identity (today's roster hardcodes `<slug>[bot]` / `app/<slug>` renderings and a GitHub noreply commit-email shape), verb-by-verb wiring waves for the write then the read verbs, the claim layer's GitLab shape (`DeleteRef` already answers could-not-check outside `refs/heads`), the statusgen half (Evidence-actor, `verifyrun` runner stamping, `--auto-flip-model` corroboration, a non-GitHub CI scaffold), the substrate (a leak-gate verdict surface on merge requests, `cellctl`), an install path that needs no `gh` on `PATH` (plain-HTTPS binary acquisition verified against the sha256 pin file, forge-neutral two-principals prerequisites, per-forge CORE primitives), and a closing conformance round trip driven entirely by desk verbs with zero hand-built API calls plus a negative-path walk of the writes the verbs refuse. Planning docs only — no tool behavior changes yet.
- A `windows-port` planning stream: a waved five-brief phase plan for native Windows support of the Assay tools (release build matrix, install path with a surfaced PowerShell-vs-Go-installer fork, portability audit, a Windows CI leg, and the adoption-doc delta), with the end state being a Windows adopter running the pinned release, CI-proven on Windows. Planning docs only — no tool behavior changes yet.
- A deterministic `(action, class, risk)` -> Tier assignment table lands in `cmd/commsloop` (`assign.go` + declared source `assign.yaml`, diffed against the compiled table), keyed on the comms prose router's closed action vocabulary — model assignment for cell-comms dispatch is now a compiled table lookup with an audit trail: absent triples refuse, there is no runtime default tier.
- Live evidence replaces two claims that had rested on documentation badges: an author approving its own merge request returns `HTTP 201` on free tier despite `merge_requests_author_approval: false` being stored, and rotate-on-mint is proved end to end — the superseded token returns `HTTP 401 invalid_token` on `GET /user` while its replacement carries a seven-day `expires_at`.
- The PR body's link trailer (`Brief: <stream>/<NN>` / `Issue: #<N>`) is not editable through the new verb. The replacement body must carry exactly one, and when the PR's current body already carries one, the replacement's must be identical to it — the derived board's edge from a PR to its work item cannot be re-pointed or dropped after every gate that checked it has run. A current body carrying no trailer may gain one, which is the pre-trailer migration `deskpr update`'s refusal already tells the worker to perform.
- The `intake-desk` skill gains a scored-triage convention: every triage exit records an `impact`/`risk`/`effort` label triple with a one-line per-axis rationale (judgment recorded, never computed in CI); human-facing surfaces order SLA-ESCALATE items first, then impact-desc / risk-desc / effort-asc, then the existing urgency-then-age — unlabelled items sort exactly as today (#294).
- The first real-history `qualgen` mine of this repo lands: `docs/quality/{metrics.jsonl,mine.json}` over the full 717-commit history, so CI renders live M1 numbers (copy/paste, churn, hotspots, bus-factor, coupling) plus the instruction reference-validity and doc↔code staleness trends into `QUALITY.md`, replacing the all-"not measured" placeholder board — with committer identities hashed in the ownership shares so no raw email/slug reaches the published artifact (#272).
- The provisioner gives each service account its role icon instead of seven indistinguishable Gravatar identicons. Since a group Owner cannot set a bot's avatar (`PUT /users/:id` is admin-only), each account sets its own via `PUT /user/avatar` at the moment its token is minted, reading that token from its `0600` file through a `curl` config file so it never reaches argv. `--avatars-dir` supplies your own icons, `--no-avatars` skips the step, and `--avatars-only` re-skins an existing fleet from the token files already on disk without minting or creating anything. A missing icon is a notice, not a failed run.
- The walk separates the two populations that a tier discussion tends to blur: the tier ceilings the 2026-08-30 CE ruling already anticipates (identity-granular protected branches, enforced approval rules, audit events, push rules, custom roles, the server-side token-lifetime policy) from four controls that need no licence and were simply never applied on the pilot deployment — most importantly `Allowed to merge` on `main`, left at Developer, which lets every service account merge and so collapses the human-merge-only fallback the whole CE posture discharges onto.
- `assay-statusgen.yml` gains a `model-autoflip` job: after each push to main, `statusgen --auto-flip-model` advances `gate: model` briefs from `verified` to `done` only when the reviewer App's approval sits at the merging PR's head; anything it cannot corroborate stays `verified` and fails the run loudly.
- `deskpr edit --body-file F [--title T]` corrects an OPEN pull request's own body, and optionally its title, through the gates `deskpr create` already runs over the text it publishes: the secret scan with the same audited `--force-scan-override`, the exactly-one-trailer grammar, the public-repo self-containment scan, the write rate limit and the public-repo `+1` gate, plus the kill switch and loop-identity checks every outward verb faces. It writes an audit row, pushes nothing, and finds the PR the way `update` does — so a branch with no open PR, and equally a merged or closed one, is a refusal rather than a write. Before this, a rework finding of the shape "the PR body says X, it should say Y" had no desk verb at all: a worker either left the description wrong or fell back to a raw `gh pr edit` that ran no gate and left no record.
- `deskpr edit` posts one short comment on the PR naming which surfaces changed. A body or title edit moves no head SHA, so a review monitor keyed on the head records no event for it and the correction is invisible to the loop that has to act on it; the comment is that event. An edit that lands while its notice cannot be posted exits 6 stating both facts, because exit 0 would claim the review desk had been told when it had not.
- `docs/contracts.md` documents the three-part schema-first contract pattern (versioned machine-readable artifact + source-side coverage gate + consumer-side conformance run), written over the brief-v1 frontmatter contract and parameterized for the consumer install seam as a second instance.
- `docs/records-and-retention.md` states, in one per-class table, which artifacts in this repo are records rather than documents — register entries, briefs and their `## Evidence` sections, stream README `Verified`/`Reviewed` cells, generated views, execution witnesses, released artifacts and checksums, exported evidence bundles — naming where each lives, who may write it, how an unintended alteration is detected, what mechanism enforces that, and how long it is kept. States the retention rule already in force (kept for the life of the repository; withdrawal is a tombstone, never a deletion) as a description of current practice, and is explicit that an adopter's own retention period, disposition, and legal-hold obligations are theirs to state. Cross-linked from `docs/registers.md` and registered in a new `freshness.yaml` for periodic re-review against the sources it depends on.
- `docs/streams/forge-gitlab/pilot-report.md` records the first live GitLab pilot: a free-tier gitlab.com group provisioned with seven service accounts, an Assay tracking root seeded and reviewed through the role chain as merge requests, and spec §3's security-parity table walked control-by-control against live API reads rather than against the runbook. Every row cites an endpoint, an id, a SHA or an HTTP status; anything the run could not observe is recorded as `could-not-check`.
- `internal/runnertable` is extracted from `cmd/verifyloop` (behavior-preserving — its own tests still pass) so the pinned tier->runner table has a second consumer, plus a new pinned DECIDER runner entry with a boot-time containment attestation for the comms prose router and outbound gate.
- `qualgen/reflex` graduates quality's M4 methodology-reflexivity layer (spec
  §7): gate-yield accounting (§7.1) joins per-lane pre-merge catches against
  M3-attributed escapes into catch-rate/escape-rate/latency-cost readouts, and
  the ritual-effectiveness natural-experiment joins (§7.2) score cost per
  durable KLOC by model tier × brittleness band and Verify-depth vs escape
  rate — every readout routed through a single brittleness-band-stratified,
  confounders-carrying emit gate (`stratify.EmitRitual`) so a natural
  experiment is never presented as a causal claim, with three-state
  could-not-measure/could-not-join resolution throughout (quality/12).
- `qualgen/riskscore` graduates a learned JIT defect-prediction model (Kamei-style
  diffusion/size/history/author features, temporal-split logistic regression)
  that always carries the §9.1 hand-weighted heuristic decomposition alongside
  it as fallback and explanation — a future-leak-proof training split, a
  could-not-learn fallback under a thin corpus (never a fabricated learned
  zero), and an honest learned-vs-heuristic comparison on held-out AUC
  (quality/15).
- `statusgen enforcement-status` renders the live authoring-guidance rules the lint actually enforces — derived from the lint registry and reported three-state (enforced / not-enforced / could-not-check) so the coverage boundary is explicit — and a new `skillslint` `ENFORCEMENT-BLOCK` check compares that fresh render against the committed enforcement block in the authoring-guidance skill, failing closed when the two drift, so documented guidance can no longer silently diverge from what the lint enforces (mistake-proofing/04).
- `statusgen` gains typed Verify-row OBLIGATION classes (`+mutation`, `+flow`,
  `+dereference`, `+neighbour`) as a second closed set on the `Class` cell,
  orthogonal to the existing WHO-EXECUTES values and encoded in a compound
  `<execution> +<obligation>` cell so the table's column set is unchanged and the
  legacy column-less hinge is untouched; an unknown obligation token is FATAL
  exactly as an unknown class is. A diff-scoped derivation
  (`statusgen/obligationderivation.go`) reuses the existing consumer-routing
  branch-diff helper — no new diff machinery, no network reach — to evaluate only
  a brief whose own file the branch edited, deriving owed obligations from the
  branch diff, declared paths, and task prose and emitting an advisory NOTICE for
  each owed-but-absent obligation; an unavailable diff is reported as
  could-not-check, never as "nothing owed". Presence is the control; adequacy
  stays the reviewer's call. Lands advisory (`obligationDerivationFatal = false`)
  per the phasing recorded in the source header (mistake-proofing/03).
- `tools/cellctl/cellctl` — one bash script that scaffolds, checks, starts and stops a single Assay cell on one machine: `new` (cell directory + the four custody hand steps it deliberately leaves to a human), `check` (per-precondition ok/MISS, `bin/deskd` and `bin/deskcli` both), `deskd` (attended-only, one installation token per org, a `--once` go/no-go before the persistent run), `desk` (one role window on its own worktree), and `up`/`down` (a tmux cockpit, `the-desk` included by default; `up` carries the operator's attended affirmation to the `deskd` window only when `up` is itself attended — a terminal on stdin or an explicit `CELL_ATTENDED=1` — and otherwise says why it did not stand it). The session keeps the operator's real `HOME` — the harness keys its login, plugins and memory by it — and only the desk verbs see the cell config-home, through a generated `shim/` directory placed first on `PATH`. Every role window launches on a model pinned in `cell.env` (`DESK_MODEL_DEFAULT`, plus `DESK_MODEL_<role>` overrides) rather than the CLI default, and carries one name — `<cell>-<short role>` — as both its roster beacon and its session display name. Documented in `docs/cellctl.md`, with a pointer from the adopter runbook's multi-cell topology section (#327).
- `tools/create-fleet-gitlab_test.sh` — an offline suite that puts a fake `curl` on PATH and drives the script's real control flow with no GitLab, credential or network: the guarded protect step's restore path, the tier fallback, the avatar uploads, and the read-backs.
- `windows-port/00` — a new wave-0 brief for the `_unix.go`/`_windows.go` build-tag split of the eight unix-only `syscall` sites in `statusgen/` and `tools/desk/` (a process-group kill, two `Stat_t` roster-owner checks and five `flock` copies), each Windows variant required to degrade explicitly and visibly rather than silently.

### Fixed
- A failed settings step no longer aborts the steps after it. Every step runs, the failures are collected under the HUMAN-ONLY REMAINDER block, and the script exits non-zero — previously one `400` on the protect step also skipped the pipeline-succeeds gate below it.
- Both settings steps are now judged on a read-back rather than on a status code. The three decided protected-branch fields are read back and printed at provisioning time, and approval settings that a tier silently ignores — a `201` that stores nothing — are reported as `failed-at-tier` instead of trusted, with a further notice where the read-back itself cannot be believed because approval rules are unavailable.
- Every outward desk verb (`deskflip`, `deskpost`, `deskpr`, `deskreply`, `deskfile`,
  `deskevidence`) refuses when `$DESK_LOOP` is unset, in the same words `deskboot` already
  used. With the variable unset a `STOP.<loop>` flag a human is holding matches nothing, so
  the halt silently failed and the verb kept writing.
- `deskboard prs` reports a watched repository the App installation cannot resolve as
  per-repo could-not-check (`repoCoverage` in the JSON, a `COULD NOT CHECK` line on the
  table) instead of failing the whole sweep. That failure is permanent for such a repo,
  so aborting on it cost the board every other repo's rows. Every other read failure —
  401, rate limit, timeout, parse — still fails the run closed.
- `deskboard`'s READ path now authenticates as the session role's GitHub App
  installation instead of falling through to the HOME keyring: every `gh` call is
  handed the cached installation token for the account the call targets, resolved once
  per account. Under a desk config home that is not the operator's own, the reads
  previously came back 401 for every private repository — and on the GraphQL path an
  unusable keyring account can return a bogus rate-limit error or an empty result, an
  absence that reads like an answer. A session with no loop identity keeps the ambient
  credential and says so on stderr; the outward verbs take the opposite rule.
- `deskdispatch`'s `worktree-create` step reports what `deskwt` actually said, whole. It quoted only the FIRST line of the child's stderr — which is always the effective-config echo — so a branch collision reached the operator as `worktree-create failed (assay-config: …)` and sent them chasing a phantom claim problem for several acquire/steal cycles. The step now drops the known preamble (the config echo, the unpinned-build warning) and prints the tool's own message verbatim, and a `deskwt` **refusal** passes through as a refusal (5) instead of being flattened into unverifiable (6) — a decision the operator can act on, not a state to retry.
- `deskflip` refuses (exit 5, naming the role and the token path) when the review role's
  App installation token cannot be minted or read, and every forge call it makes runs
  under that token. It previously proceeded on the operator's ambient `gh` login, so the
  ready-flip and its queue labels were written under a human identity and read afterwards
  as a human decision.
- `deskflip`'s test binary now installs its fixture roster in `TestMain`, the way every sibling command package does, instead of relying on the roster each test plants as its first statement. Three identity tests built their stub with a helper evaluated in the composite literal — before that statement ran — so they resolved the reviewer-role binding from whatever config home the machine happened to have: green on a developer's box with a real `roster.env`, red on any runner without one. The release lane's `go test ./...` was the only gate that ran these tests at all, so the whole desk-tools leg failed there with "the fixture roster does not bind the reviewer role" while the PR-time lane (build + vet only) stayed green.
- `deskwt add` no longer dies on a stale local branch. Worktrees share one refs store, so a branch left behind by an abandoned dispatch (`git worktree remove` does not delete the branch it was on) blocked every later `add` that derived the same name, as a bare exit 6. The collision is now resolved by name: a leftover that is checked out in no worktree and carries no commit its upstream (or `--base`) lacks is **reclaimed** — deleted with a compare-and-delete against the sha the proof was taken on, then recreated — with an audit line naming the branch, its old sha and the ref it was measured against. A branch **checked out in another worktree** is refused (5) naming that worktree's path; a branch **ahead** of its comparison ref is refused (5) naming the commit count, because unfinished work is not this tool's to delete. There is still no force verb anywhere in `deskwt`.
- `docs/adopting-assay-gitlab.md` §0.1 now states the ruled edition stance (#219): Free / Community Edition is conforming for the core lane with its degradations disclosed — the earlier wording ("a pilot lane, not a conforming deployment") contradicted the ruling. The disclosed-degradation table grows from the two rows the edition matrix named to the seven the live pilot measured, each with what stands in for the control on Free and the tier that closes it.
- `tools/create-fleet-gitlab.sh`'s protected-branch step no longer leaves the tracking root writable when it fails. It reads the group's tier and sends the Premium `allowed_to_*` arrays only where they exist — below that the three fields Free actually accepts, reporting the omitted push allowlist as `failed-at-tier, remediation: Premium` — and it never removes the existing rule before the replacement is known to apply: an already-correct rule is a no-op, a force-push-only difference is a `PATCH`, and where a delete-and-recreate is unavoidable a refused re-create immediately re-applies the rule that was read. The intended `merge_access_level` is named as 40 (Maintainers) in that recovery path, so a hand repair cannot reproduce the 30 that let every Developer service account merge its own merge request.

### Changed
- Deviation D-6 is rewritten around the part that generalises beyond this deployment: an unprotect-then-fail defect does not only leave a branch open for a window, it hands a human a manual re-protection to compose against an API whose free-tier field set differs from the script's. Getting `push_access_level` right while getting `merge_access_level` one level too low is both the natural mistake and an invisible one afterwards — the branch still reads "protected" and `can_push` still reads `false`. The provisioner's fix should therefore re-create the rule with the intended levels and read all three fields back, so neither the failure path nor the recovery path can leave a weaker rule than the script would have written.
- Recorded from the final merge: with no pipeline configured, GitLab asks the human to confirm merging unverified changes. That prompt is the only place in the whole round trip where the absence of the pipeline gate becomes visible to a person — everywhere else a merge request with no checks at all looks exactly like one whose checks passed, which is the state an adopter inherits by default from the provisioner's early exit.
- The `changelog-check` PR gate now enforces the fragment convention (`changelog/<slug>.md` with at least one highlight bullet, or `changelog:skip`) and refuses direct `## Unreleased` edits; `release.yml` aggregates the fragments into the release highlights, refuses to cut with nothing to aggregate, and rolls them into a dated `CHANGELOG.md` section in the release commit.
- The `windows-port` stream's "the Go binaries are already portable, no source change needed" premise is retired: it was measured on 2026-08-07 as a harness claim, not a `GOOS=windows` one, and neither module cross-compiles for Windows today. `windows-port/01` (release build matrix) keeps its two-file scope but now depends on `00`, and the stream's critical path becomes `00 → 01 → 03 → 05`. Planning docs only — no tool behavior changes yet.
- The brief's round trip is recorded step by step with actor, mechanism, artifact and timestamp: the seed merged by the human, then the worker's deliverable and `Draft:` merge request, the reviewer's head-pinned verdict note and approval (author and approver are different service accounts, dereferenceable from the ids), and the desk identity's ready flip. Verify row 4 remains `could-not-check` — board regeneration is gated on the next human merge, and is recorded as not run rather than assumed.
- The dispatched REVIEWER kit now requires every path-existence claim to be resolved in the
  PR's OWN repository at the PR head, to name the tree it was resolved against, and to be
  reported as could-not-check when it cannot — a reviewer that checked paths in the
  dispatching desk's checkout reported files as missing that were present in the PR's
  repository.
- The loop-to-App-role table moved into `deskkit`, so the identity a window presents and
  the identity its calls carry are read from one place.
- The post-merge half of the round trip is recorded and, more usefully, its shape is: the verifier's Evidence row and the board regeneration both have to travel as merge requests, because `main` is push = No one for every identity, and neither has a GitHub-style direct-to-main carve-out. The pilot shows the cost is containable by stacking rather than serialising — the board's change targets the Evidence branch, so the pair is one human sitting, and the stacking is forced by the data (the board is derived from the register the Evidence flips) rather than chosen for convenience.
- The round trip completed: one brief went `todo` → `verified` on a Free-tier group through five distinct role identities and four merge requests, and all four of the brief's own Verify rows are now checked-clean against the live system. The clearest single piece of evidence is the tracking project's history, which the report renders as a table — every content commit is a distinct role service account, every merge commit is the human, and no identity appears in both columns. That alternation is the CE posture rendered as history rather than as settings, and it is also why the merge-access misconfiguration mattered: for the first two merges of that log it was a convention the server was not enforcing.
- The trust-roster gap is now demonstrated on a real row rather than on an empty roster: with a distinct, non-implementing verifier service account's Evidence committed, lint still reports the row as backed by no accepted verifier actor. A correctly-verified GitLab row is indistinguishable from a self-attested one, because the check has no GitLab identity to recognise.
- Two further deviations recorded from the live run. The execution-witness generator stamps a runner derived from the local machine rather than from the forge identity that acted, so on a non-GitHub deployment the witness row and the Evidence row disagree by construction — annotated in place rather than hand-corrected, since editing a generated witness is the manufactured evidence it exists to prevent. And a service account's avatar can only be set from that account's own credential on SaaS (`PUT /users/:id` as owner returns 403, `PUT /user/avatar` as the bot returns 200), a shape the provisioner's single-owner-token model does not have.
- `docs/adopting-assay-gitlab.md` no longer tells free-tier groups to stop: a measured §0.1 records that the full identity model provisions on gitlab.com Free (service accounts are free since 18.11) while the write-path controls (board-writer allowlist, required/prevent-author approvals, token-expiry policy, audit events) are failed-at-tier with their Premium remediation — a pilot lane, not a conforming deployment.
- `docs/adopting-assay.md` now recommends the App permission set the desk tools actually require. The reviewer App's `contents: read-only` recommendation is **retired**: the boot preflight's `app-scopes-vs-duties` check applies one uniform `requiredDuties` set — `pull_requests: write`, `issues: write`, `contents: write` — to every role and refuses to run a role whose App lacks any of them, so the old advice provisioned a reviewer that could not boot. The guide states plainly why: *author ≠ approver* is held by the reviewer being a distinct identity nobody without its private key can post as, and by the forge rejecting a self-approval — not by a withheld `contents` scope, which never governed review posting at all. A deployment that wants to separate "may review" from "may land on the default branch" is pointed at branch protection instead. The `setup-reviewer-app` Verify clause that asserted the *absence* of `contents: write` is replaced with a positive check of all three duties plus a `--fresh` re-mint caveat (a cached token re-reads its old grant). The narrower CI-read trio (`checks`/`statuses`/`actions: read`) stays role-scoped, and the "do not grant it to verifier / inbound-lane Apps" guidance is unchanged.
- `docs/streams/forge-gitlab/pilot-report.md` records the round trip through the ready flip and closes the finding it opened. The `Allowed to merge` misconfiguration the walk found on the pilot's `main` was repaired mid-run — the protection rule was re-created with `merge_access_level: 40` (Maintainers), and every service account now reads `user.can_merge: false` while the human owner reads `true`. Row 10 carries both reads rather than being quietly re-measured, and the overall verdict is recounted to four PASS, two could-not-check, eight failed-at-tier.
- `spec/brief-v1.md` is brought current with the reference validator: the "Describes reference implementation" header now names the released `statusgen` version instead of a long-stale one, and the five optional frontmatter fields the validator has grown but the spec never documented — `domain`, `blocked-by`, `homed-in`, `measures`, and `parallel-streams` — are now specified with their exact value sets and flagging rules.
- §2 now says where the role-token store is (the config-home the desk verbs read), that the script's `<prefix>-<role>-bot.token` files must be linked to `desktoken`'s `gitlab-<role>.token` names, that the owner PAT is a legacy `api`-scope token (fine-grained unproven), and how to recover `main` when the free-tier protect step fails after unprotecting it.

## v0.23.0 — 2026-09-02

### Added
- A model-capability floor gates authority-bearing desk writes: `deskflip`, `deskpost ready`, and `deskpost` review verdicts now refuse a session whose dispatch is attested below the strong tier — keyed on the dispatcher-applied model+tier label stamp (self-applied stamps are worthless), failing closed. Unattested (human / pre-attestation) lanes proceed with a notice, and an incident-recovery override is logged loudly (#278).
- A peer-auth desk-comms backbone lands (`tools/desk/internal/comms/`): a `cellmsg-v1` envelope that parses-or-refuses, ed25519 sender-identity assertions (mint/verify, single-use, TTL-bounded), and a compiled lane ACL that is deny-by-default — cross-cell reach and human-gate verbs ship refused until a recorded ruling (#276).
- New `qualgen/dorajoin` package: the DORA join — a quality denominator (durable-change volume) and a traced-CFR refinement reported alongside incident-based CFR, joined to a pluggable `DeliveryMetricsSource` (a file-based reference adapter ships in-tree) on PR number / merge SHA / stream-task-ID, three-state throughout and never emitting a bare traced rate without its trace-rate and evidence-tier split.
- The changelog discipline is ACTIVE: the changelog-check PR leg gates merges and release.yml refuses an empty Unreleased section, lifting highlights into the release body (#266 activation, #269).
- `deskcomms` gives the desks their client surface onto the local cell gateway: `send` runs a fail-fast preflight (reserved-verb → identity → parse → lane-ACL → bodycheck → ratelimit → mint → submit) that CALLS the same `internal/comms` parse/ACL the gateway re-runs authoritatively, then signs and submits; `poll`/`ack` read this session's own per-role mailbox (ack moves, never deletes). Sender identity comes from session context, never a flag; enforcement stays the gateway's; there is no local-spool fallback, so an unreachable gateway fails closed rather than fabricating delivery (#299).
- `deskpost` attaches mechanical verdict-time triage labels to agent PRs — a `size:S/M/L` class over changed lines (generated files excluded) and a three-state `surface:core/std` tier read from a repo's `.assay-surfaces` globs — advisory only (nothing gates on them; an unreadable surface is could-not-check, never assumed) (#277).
- `qualgen check <paths>` screens named files for brittleness signals (stronger-tier, add-coverage, coupling-partner, reference-rot) as an always-advisory, exit-0 pass over the mined M1/M2 families — the per-file complement to the corpus-wide mine (#275).
- `qualgen pr <n>` emits a generic per-touched-file risk-feature feed (hotspot percentile, traced defect density with its trace-rate, ownership top-share, missing coupling partners) as JSON — no weighting or combined outcome of its own, so a consumer's own config decides what to do with the numbers.
- `qualgen/attribution` implements M3 stage attribution: it assembles a deterministic, content-addressed dossier per traced defect and names the stage the defect escaped at — `spec` / `brief` / `implementation`, or `untraceable` when the provenance chain is broken (never binned into a stage) — plus a `review-escape` overlay naming the lanes that approved the inducing change; the stage call is judgment-classified and spot-auditable against the fixed dossier, with a pluggable provenance-linkage adapter (a generic commit→issue reference adapter ships as the default) and an append-only per-stage defect ledger correctable only by tombstone amendment.
- `qualgen` mines the instruction-brittleness M1 family for real: instruction reference-validity and doc↔code staleness now render into `QUALITY.md`'s trend view behind a new `--instruction-globs` flag, replacing the family's placeholder — and an unconfigured run reports could-not-measure, never a silent zero (#271).
- `qualgen`'s M4 session-forensics join lands: a pluggable `TelemetrySource` interface plus a file-based reference adapter (`qualgen/telemetry`), and a read-only join over the M1/M2 corpus (`qualgen/m4`) correlating harness telemetry (retries, refusals, …) against churn and defect outcomes, with three-state coverage reported beside every correlation — code only, no telemetry source wired in (quality/13).
- `skillslint` also emits an advisory context-budget `NOTICE` for any instruction file over a word threshold (3,000 for `SKILL.md`, 5,000 for `CLAUDE.md`), flagging context-bloat candidates. The NOTICE is advisory only — it prints to stderr and never changes the exit code.
- `skillslint` now runs a byte-level invisible-character / Trojan-Source lint over the instruction surfaces (`plugins/assay/skills/**/*.md`, and `.claude/skills/**/*.md`, `plugins/assay/resident-rules.md`, `CLAUDE.md` where present). It rejects — by Unicode category, not an enumerated blacklist that would miss members — the whole `unicode.Cf` format category (bidi controls and directional marks incl. LRM/RLM/ALM, zero-width, invisible math operators, soft hyphen, the Unicode Tag block used for LLM ASCII smuggling, and non-leading U+FEFF), the variation selectors (U+FE00–U+FE0F, U+E0100–U+E01EF, and the Mongolian free variation selectors U+180B–U+180D/U+180F), a curated set of other invisibles that are neither Cf/VS/Cc (U+034F combining grapheme joiner, the Hangul fillers U+115F/U+1160/U+3164/U+FFA0, the Khmer inherent vowels U+17B4/U+17B5, U+2800 braille blank, and the U+2028/U+2029 line/paragraph separators), the assigned Unicode Default_Ignorable_Code_Point property as its own durable property-based branch (so every assigned-DI codepoint flags even if reclassified out of a category), any C0/C1 control outside tab/newline/carriage-return, and invalid UTF-8 — each reported with file, line, column and codepoint. Unassigned/reserved Default_Ignorable and visible Zs space separators (ordinary space, NBSP, …) are deliberately left legal. Printable non-ASCII (accented text, arrows, box drawing, an emoji whose base glyph carries its own presentation) stays legal — the check targets invisibility, not foreignness. This catches the exact payload class a human reviewing the rendered text cannot see.
- `statusgen conform` validates brief frontmatter against a versioned, machine-readable brief-v1 contract (`schemas/brief-v1.json`, JSON Schema draft 2020-12) embedded in the binary — required keys, field types, and closed value sets, reported three-state (checked-clean / checked-failed naming file+field / could-not-check, fail-closed) and distinct from `--lint`'s methodology rules; a `schema:` marker newer than the embedded contract is a version mismatch, not a field error. `conform --emit-schema` prints the embedded schema so the artifact is reproducible from any pinned binary, and a source-side coverage test derives the required-key and value sets from the reference validator's own tables so the schema and validator cannot drift without CI failing.
- `statusgen` gains a DECLARED, fail-closed fixture-corpus exclusion: a directory that drops a `.statusgen-fixtures` marker at its root opts its whole subtree out of both the `--lint` link check (dead-link / backticked-path / identifier-dereference / register-ref) and `--corroborate`'s `human:<name>` stamp scan, so eval/fixture corpora of captured run-outputs stop redding on their legitimate forward-references. The exclusion is DECLARED (marker-only, never inferred from a path name or `testdata`/`fixtures` convention) and FAIL-CLOSED (no marker on disk → the subtree is scanned exactly as a live brief); live briefs are untouched.
- `tools/create-fleet-gitlab.sh` idempotently provisions the Assay fleet's seven per-role GitLab service accounts, memberships, and PATs, plus a project's protected-`main` and approval settings; paired with `docs/adopting-assay-gitlab.md`, the GitLab-profile adopter walkthrough (ci-config-project runbook, token custody, tier ladder) cross-linked from `docs/adopting-assay.md` (forge-gitlab/04, #288).

### Fixed
- The desk's CI-rollup readers now evaluate the LATEST run per check NAME, mirroring branch protection's own "latest run per context" rule, so a superseded run — an older CANCELLED predecessor, or a stale QUEUED orphan left by a push + pull_request double-trigger — no longer counts against a PR whose current run for that name is green. This lands as one shared `deskkit.LatestRunPerName` reducer called by all three surfaces — `deskflip`'s ready-flip gate, `deskboard`'s CI-state render, and `deskkit.ReduceCIVerdict` — so the flip gate and the board can no longer diverge on the same double-triggered PR (one flipping it ready while the other still renders it CI-fail). The gate is not relaxed anywhere: a name whose current latest run is red, cancelled, or pending still reddens or blocks (#282, #289).
- `statusgen --record`'s DORA-timing recorder no longer fails silently when its authenticated `gh` reads (restore episodes, PR lead times) all fail — it emits a loud, distinct `DEGRADED` signal naming the failed read and the substrate path, instead of returning a no-op indistinguishable from a healthy quiet day (so a persistently token-less `--record` CI can no longer leave `.dora-timing.jsonl` silently never accruing); still fail-open, never fabricates (#279).

### Changed
- Changelog highlights are now recorded as per-PR fragment files under `changelog/` instead of shared `## Unreleased` edits: `changelog-check` greens on a fragment (or `changelog:skip`) and refuses a direct `## Unreleased` edit, and the release workflow aggregates fragments into the dated section and release Highlights, then clears `changelog/`.
- The `assay:verify-desk` skill body gains three neutral verification-quality controls — derive-from-base-branch grounding (derive what should exist before reading the work), per-row fan-out for large Verify tables (≥4 risk-bearing rows run as isolated per-row sub-verifications), and Evidence↔Verify-row scope-traceability (unmapped verified work is flagged as invented scope) — plus an anti-gaming rule to re-derive expected values from the brief rather than the work under test.

## v0.22.0 — 2026-08-31

### Added
- CI grows five control legs (#255): a forge-surface control sweep, a leak-sweep
  pattern sweep, per-plugin shell suites, a gating skillslint leg, and a
  QUALITY.md render check — each exercising a control that `go build`/`go vet`
  alone would leave un-run.
- A quality trend view: churn-vs-durable, hotspot and brittleness reporting land
  behind a single-writer `QUALITY.md` (quality/01–06: #245–#248, #252, #254).
- A per-loop pool-width knob for the desk loops (#226).
- A deterministic verdict runner (#242).
- Roster-from-deployment resolution (#256).
- A PR-body self-containment scan (#227).
- `inbox --flow` / `--walk` / `--html` views (#225, #233).
- A two-role superseded lane for `deskclose` (#232).
- B-SZZ inducing-commit tracing plus derived defect metrics land in `qualgen`
  (#261).
- Spec-routing §8 spec-lifecycle enforcement: a linter and an authoring-owed
  emitter (#267).
- The CHANGELOG discipline itself — a per-notable-PR `## Unreleased` highlight
  line, with the release-time roll and its CI enforcement staged (#266).

### Fixed
- The board archives cleanly: statusgen now resolves streams under
  `docs/archive/` as known depends / unblocks / affects targets (#259), so
  archiving a finished stream no longer reds valid references from still-active
  work.
- The verify queue stops lying: `verifyloop` defers `blocked-until` briefs,
  buckets online-lane and human-gated work out of DISPATCH, and reads
  qualifier-carrying `## Verify (…)` headings (#251, #253, #257).
- Board regeneration no longer races on concurrent pushes: regen-push is
  serialized (#221).
- A latent drift-registry test red is fixed (#262): `statusgen`'s
  `blockingIssueLabels` is registered as a declared exception, greening the
  release-only (desk-tools) test leg.
- The archive fallback extends to the markdown link/backtick check (#264), so
  references into `docs/archive/` stay green there too.
- `muhar -j 0` auto-parallelism is capped at 2 mutants in flight (#268) — the
  release test leg is memory-bounded by construction; every mutation still runs.

### Changed
- `pr-shepherd` is de-housed into the `assay` plugin so adopters get it too
  (#234).
- GitLab forge support closes out with a forge tier matrix (#222, #230, #231).

### Consumer action
- Pin `statusgen` at ≥ this release to lint boards that reference archived
  streams under `docs/archive/` (#259).
