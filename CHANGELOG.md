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

## v1.0.35 — 2026-10-09

### Added
- A read that fails while the verifier packet's commit list is built is stated in a fixed sentence of the tool's own; a test fails each of the provider's reads in turn and holds the reader's error text out of the file (#2439).
- A staged copy of the board workflow (`ci/staged-workflows/assay-statusgen.yml`) adds an hourly `reconcile` job, pending maintainer promotion into `.github/workflows/`. It writes the generated Briefs tables in stream READMEs by pull request only: it flips Status cells witnessed by a merged PR's `Brief:` trailer, re-renders the tables, and carries any change as one draft pull request on `board/reconcile`. It never pushes the default branch, scheduled runs have their own concurrency group so they cannot displace a pending push regen, and the schedule runs only in this repository, never a fork. Each tick recomputes from the default branch, so a change on the default branch never wedges it; it refuses to overwrite a branch carrying a commit whose subject or change is not its own, anywhere on the branch (the check reads a commit's subject and what it changes, renames included, not who made it, and a failed read fails the tick); and it fails when the remote, the pull requests or the PR list cannot be read. Its pull request is the open one from this repository's own branch into the default branch, whoever opened it: a fork's same-named branch, or a pull request from that branch into another base, never stands in for it, and two candidates fail the tick. Each push rewrites the PR title and text from that tick's report, and the report, including any held rows, is printed to the job log. `statusgen` runs holding only the read token; the board-writer App token is minted only when there is a commit to publish, narrowed to contents and pull-request writes, and only the step that pushes and opens the PR holds it. The job uses the same isolation as the corroborate job (a job-local, checksum-verified Go toolchain and caches, a pinned gh), with Go workspace files off so none left above its temporary directory is read, and reads no git state an earlier job on the runner could leave: it clones the default branch fresh under the job's temporary directory instead of checking out into the workspace, uses git and gh configuration of its own with hooks off, pushes to the forge by URL, and binds its credential helper to that host. Once the file is promoted, the board workflow's pull-request lint runs the job's tests.
- A worker packet shows every value it did not write — a title, a login, a branch, a label, a
  check or file name, a finding's id and state, an error's text — inside a code span the value
  cannot end, and quotes longer text between boundary lines. It reads a finding record only
  from a review the forge attributes to the reviewer identity: a review by any other account
  is listed under its own heading as not the reviewer's, and when that identity cannot be
  resolved no review is listed as the reviewer's, no record is read and no body is quoted.
- Both worker kits gain three clauses about how a run gathers and waits — batch independent
  reads into one request, read the packet first when the assignment names one, and wait on a
  check or a review in one bounded command — none of which changes what a worker must do or
  wait for.
- Every `deskread` envelope carries an additive `identity` object (`app-custody` with the role, or `ci-workflow-token` with the repository and run id), and one stderr line names the transport. The token never appears in output.
- In a worker packet for an open change, a finding record in a review at an earlier commit is
  judged against that review's commit: `resolved` with no evidence commit of its own is not a
  resolution at the current head, and the line says so. Comments are quoted newest first, so
  the packet's cap leaves out the oldest; each comment, and each review by another account,
  says whether its author is the reviewer identity, on the trusted list, not on it, or not
  checked because no list is configured; a comment by an account that is neither is quoted
  only up to 16 KiB (#2439).
- Review kit clause 20 and the `pr-review-desk` skill make the lane round cap bind: from a lane's fourth round, a finding first raised then, in code unchanged since the lane's first review, is advisory or an arbiter hand-off, except a safety-relevant finding, which still blocks. A late finding the reviewer cannot place with confidence blocks, the cap never changes a security verdict, and a finding whose evidence did not exist at the first review (the base branch moved, the description was edited, a check result at the head, a could-not-check gap, an undeclared decision) is not a late finding.
- Text taken from the change is marked as data in the packet two ways. Multi-line text sits between two boundary lines that carry a random value made for that one file; inside them, a line that would begin with three less-than signs is prefixed with `[quoted] `, and line-break characters other than the newline are written as `\uXXXX`, so no quoted line begins with a boundary mark. Single-line values (title, branch, check and file names, logins, error texts) are written in code spans in which a backtick is shown as an apostrophe. The packet's opening lines tell the reader both.
- The `// regression: #<N>` doc-comment tag (also `F-<slug>`, `class #<N>`) on the line above a test says which incident it pins; seven existing regression tests carry it. Only a line holding refs of those shapes and nothing else is a tag: doc-comment prose that happens to wrap onto a `regression: ` line is not.
- The outbound-write check can consult a deployment's own executable, the house callout. Set `ASSAY_OUTBOUND_CALLOUT` in the roster to an absolute path; every outward write the compiled checks have passed is put to it AFTER them (a push: once for the branch name, once per commit message, once for the added lines), as one JSON object on stdin with the text as plain JSON strings (no HTML escaping), and it answers `allow` or `block <reason>`, the first word exact and lower case. It can only narrow: a compiled refusal is never put to the callout, and a callout `allow` clears nothing. Any failure of the callout (missing, writable by group or world, non-zero exit, timeout, empty or oversized output, an answer that is neither word) refuses the write, and the message names the failure. Every refusal is the `house.callout` rule and is not overridable by `--force-scan-override`. Everything the callout printed — a block's reason, an answer outside the vocabulary, its own stderr — goes to stderr only, on one line, never to the audit log, the error text or the forge. The callout runs with an environment of exactly `PATH`, `HOME`, `TMPDIR` and `LANG`, holding the calling process's own values; a callout sets its own `PATH` and its own locale, as the example does (`PATH=/usr/bin:/bin`, `LC_ALL=C`), so the caller's `PATH` or `LANG` cannot decide its answer. When the callout answers `allow`, anything it wrote to stderr is still printed to stderr.
- The review kit gains two clauses on every lane. Clause 17 tells a reviewer to read a file in the fewest requests (normally one read, whole; one targeted read of a large file the PR barely touches), to send independent reads in one request, and to gather a PR's state in one command, because every request re-reads the whole conversation. Clause 18 tells a reviewer to read a prepared review packet first when the assignment carries a `Packet:` line; it is inert when no such line is present, only the dispatcher's assignment block can arm it, and the PR's head and every check are still read from the forge before the verdict (#2437).
- The verifier kit gains clause 7, "Packet first — a reading aid, never evidence": read the packet first, whole, in one read, fetch only what it lacks, and never copy a packet line into an observed cell or count a row as run because its command is quoted. A row's result comes only from running the row at the verified commit (#2439).
- The verifier packet finds a brief's Evidence and Verify headings the way the repository's own readers do, with the white space around the line taken off first, so an indented heading no longer leaves an Evidence section in the packet. It no longer says it holds no result: it says what it cut, and where it finds no Evidence heading it says it could not determine whether the carried text records an earlier result. The result-like-column check now covers every table in the carried text, and a numbered or emphasised Evidence title cuts too (#2439).
- The verifier packet writes every value it read from the repository and shows at its own level — the brief's path, a commit id, the brief id — with `packet.Code`, so a path chosen by a brief's author cannot close its code span or start a line of its own; copied text stays between a boundary pair, where a line that would begin like a boundary line is shown with `[quoted] ` in front (#2439).
- The verifier packet's commit list says how many commits it left out when it stops at its limit of six, and says when no commit message was searched because the item key gives no brief id. The verifier kit's packet clause says what to do when one read does not return the whole packet (#2439).
- The worker-desk skill adds a rule for new dependencies, plugins and MCP servers: before install, file an issue with an agent-generated risk summary covering source, permissions and persistence. The minimum release age is 7 days, and a younger release also needs a driver `bless`. The worker dispatch prompt carries the same rule.
- Versioned source obligations and project applicability: `spec/project-obligations-v1.md`, `schemas/project-obligations-v1.json` and an offline loader library in `statusgen` (tested, not yet called by any lint check, command or workflow) that bind a permitted source revision to existing requirement ids and accept an applicability decision only as a link to an existing decision record, bound by an injective digest to the exact subject, including the whole decision proposal. The digest-bearing decision record and its corroboration receipt are one admitted file; a decision id claimed by two files is held, not resolved by file order. The digest binds the record as it stands in the evaluated tree, not the moment its issue closed; the spec states that limit. Source text enters a model only when permitted, forged or stale approvals and dangling requirement references are refused, and unresolved applicability holds a complete claim. Extends the requirements register with the link contract (`registers-v1.md` section 6.6).
- `ASSAY_OUTBOUND_CALLOUT_REQUIRED=public` refuses a write to a public or unknown target when no callout is configured; private targets are unaffected. `ASSAY_OUTBOUND_CALLOUT_TIMEOUT` sets the deadline (1s to 60s, default 5s). All three keys are recognised by the roster parser in `deskkit` and in `statusgen`; a set but malformed value refuses outward writes rather than reading as unconfigured. The startup echo shows the timeout and marks the path `INVALID` when any of the three keys is malformed. The contract, and an example executable that decodes the JSON text before matching (so a listed word containing `&`, `<`, `>`, `"` or `\`, or a phrase split by a line break, is matched, and which answers `allow` only when its match ran over every text field and found nothing: a missing or failing tool refuses the write, and a write with non-ASCII text is matched whatever locale the caller runs in), are in `tools/desk/README.md`.
- `cellctl`: a machine-wide cell defaults file. `$CELLS_ROOT/defaults.env`, in `cell.env`'s
  grammar, sets a key once for every cell under that cells root — `CELL_GO_CACHE=on` is the
  motivating case. A value comes from the highest layer that sets it: cellctl's compiled default,
  then the process environment, then `defaults.env`, then the cell's own `cell.env`, so one cell
  opts out with a line in its own file. `cellctl set` resolves through the same layers and still
  writes the cell's `cell.env` only; `cellctl show` labels a value the file supplied. It applies
  to every cell kind and adds no variable to a scrubbed or container launch. See
  `docs/cellctl.md`.
- `cellctl`: a template for the defaults file, listing the keys. `cellctl new` writes
  `defaults.env` into a cells root that has none, from a template in which every machine-wide
  key is a commented-out `# KEY=<compiled default>` line under its description, followed by the
  keys the file refuses and why. The new verb `cellctl defaults init` writes the same file for a
  cells root that already has cells, and `cellctl defaults print` writes the template to
  standard output to read or to diff an existing file against. An existing file is never
  overwritten, appended to or replaced; when something that is not a regular file is already
  at the path, `new` leaves it and prints a notice. The template is generated from one registry
  of keys and prints every key in it except four internal variables cellctl sets for its own
  child processes, which are listed in `docs/cellctl.md` only. A test fails when cellctl reads
  a key the registry does not classify, for the reads it recognises: `os.Getenv` /
  `os.LookupEnv` calls and cellctl's own environment readers. A scan of the whole environment,
  a `$VAR` inside a file value and a variable only a child process reads are outside it.
- `cellctl`: in `defaults.env`, a line with an empty value sets nothing. `KEY=`, `KEY=""` and
  a value that expands to nothing are skipped for every key, so the layer below stands — an
  uncommented template line with no value cannot displace a value the environment sets, such
  as the model policy path. A line with a value outranks the environment, also when the value
  is the compiled default. The file therefore cannot blank a variable for every cell; a
  cell's own `cell.env` still can, and its empty assignments behave as before.
- `cellctl`: per-role starting context (#2438). A cell can set `CELL_ROLE_CONTEXT` to a JSON
  declaration that, per role, switches off plugins, skills, built-in tools, agent types,
  connectors and instruction files, points the role at its own memory directory, appends a role
  instruction file, and installs agent definitions for what the role dispatches. `cellctl desk`
  applies it at launch and prints what it applied, in every window `cellctl up` opens; `cellctl
  check` reports what each role will start with and refuses a declaration naming something
  missing. No key grants a plugin, permission, hook, connector or model. Two keys do more than
  remove context and are bounded: `plugins_off` drops a plugin whole, hooks included (the role's
  own plugin is refused, and `check`, a dry run and the launch name every plugin switched off),
  and `memory_dir` is a write location that must resolve below `<cell-dir>/memory/`. A repeated
  key is refused. So is a declaration or named file in `<cell-dir>/worktrees/`, in
  `<cell-dir>/memory/` or in a tree a role worktree links to, decided by file identity and not by
  how the path is spelled. A cell that declares nothing launches and checks exactly as before.
  Ships a `builtin:lean-reviewer` agent definition (shell and file read/write only), which
  roughly halved a dispatched agent's starting context in measurement. See
  `docs/cellctl-role-context.md`.
- `cellctl`: the defaults file is read strictly. A file that cannot be read, a line that is not
  an assignment, or a key that is not a machine-wide lever refuses, naming the key, the file and
  the line. Refused are the keys that name or scope one cell (`CELL`, `CELL_KIND`, `CELL_REPO`,
  `CELL_REPO_SLUG`, `CELL_ROOTS`, `ROLES`, a cell's `deskd` address, session, cache root,
  container binding and forge binding); the switches for one run (`CELLS_ROOT`, `DRY_RUN`,
  `CELL_ATTENDED`, `DESK_MODEL_OVERRIDE`); the locations and host variables cellctl follows from
  the launching shell (`HOME`, `USERPROFILE`, `XDG_DATA_HOME`, the config homes, `PATH`, `TERM`,
  `LANG`); and cellctl's own internal variables. A key cellctl does not read is carried as a
  `cell.env` line would be.
- `cellctl`: what a defaults file changes. A file that sets no key — none at all, or the
  template as written — changes no resolved value and no launch. `cellctl check` gains two rows
  (the file read and the keys it sets; the managed Go cache's state and the layer that set
  `CELL_GO_CACHE`): a cells root with a defaults file prints them for every cell, and one
  without prints them only for a cell that sets `CELL_GO_CACHE`.
- `deskclose` checks the item again after every verified close. If the close call returned without error but the item still reads open, the run now fails (exit 6) and files one repair issue on the item's repo, deduplicated by exact title. The repair issue is a charged write: it passes the outward-write budget and records its own audit line, and a refusal from the budget leaves the close failed with nothing filed. Re-running a close on an item that was already closed still exits cleanly as an idempotent no-op.
- `deskdispatch --kit review --pr <N>` now states each lane's round in the reviewer's assignment: the round number, the head of the lane's first review, and whether the round is `Scope: DELTA` or `Scope: FULL PASS` with the reason. A delta round covers the lane's previous findings, whatever its previous verdict recorded as could-not-check, not run or incomplete, the diff between the previously reviewed head and the new head, the description check, and the head-level duties that stand in every round. The scope is a full pass when the lane has no verdict of its own, its latest verdict is already at the dispatched head, the inter-head diff cannot be computed, the delta is more than 20 commits or more than 10 of the change's own paths, a merge in the delta changed a file the change touches or its merged-in side did (whichever way a conflict was resolved), the change touches a path it did not touch at the previously reviewed head, one of the lane's verdicts names in its own text or typed finding block a different head than the forge records for it or carries a typed finding block that cannot be read, or any of that could not be determined. A verdict that names no full commit id on a recognised head line or in its typed block is taken at the forge's record. Review kit clause 19 binds the reviewer to do the full pass when it finds the stated scope wrong, when its lane's previous verdict names another head than the assignment gives, and when that verdict calls itself incomplete.
- `deskdispatch --kit review --pr N` writes a review packet: one owner-only Markdown file holding the change's facts and description, the checks at its head, the earlier verdicts, the whole diff, each touched file as it reads after the change, and the brief when one resolves. The assignment gains one line, `Packet: <absolute path>`. Only a review posted by the reviewer identity is listed or counted as an earlier verdict — this lane's in full, the rest as an index; a review by any other account is listed under its own heading as not a verdict, and when the reviewer identity cannot be resolved nothing is listed as one. Caps are stated in the file and anything over one is listed by name and size instead of being cut short. Building it can never fail a dispatch, and it needs no flag or configuration (#2437).
- `deskdispatch --kit verifier` writes a verifier packet: one owner-only Markdown file holding the brief's gate and risk frontmatter lines, the brief text up to its Evidence heading, each Verify row's `#` and Command cells exactly as written, the size of any earlier Evidence section, and up to six commits that changed or name the brief with the paths each changed. The assignment gains one line, `Packet: <absolute path>`. It is a reading aid only: the dispatcher runs no row, the packet leaves out the brief from its first Evidence heading on, the Expect cells from the command list and every commit subject, message and date, a brief whose carried text has a table with a result-like column gets no packet at all, and the file is never written inside the verifier home. Building it can never fail a dispatch (#2439).
- `deskdispatch` holds a review dispatch before the claim, exit 5 with a first line beginning `review dispatch HELD`, when a check the base branch requires has failed on every latest report under its name, the description's `## Weight` section declares a head that is not the head on the forge, the change carries `needs-decision` with no ruling recorded since the label was applied, or the lane's latest verdict at that head blocked only on rulings not yet recorded. A hold takes no claim, cuts no worktree and writes no reviewer prompt; its first line is the dispatch audit row's detail. A gate read that fails dispatches the reviewer. `--dry-run` skips the read.
- `deskdispatch` writes a packet for worker dispatches too (#2439): one owner-only read-ahead
  file named on the assignment's `Packet:` line. An implementing run gets the run's facts, the
  brief or issue, the board status of the briefs it depends on, the repository's instruction
  files and build entry points, and the files the brief names; a run dispatched with `--pr`
  onto an open change gets the change, how it stands against its base, check states, every
  review with the findings it records, the newest comments and the diff. Caps are tighter than
  the review packet's (32 KiB an item, 192 KiB in all) because a worker run re-reads the packet
  on every later request. A packet that cannot be built never fails the dispatch.
- `deskevidence` has a new composition test, `TestRefuseIntroducedProblem`, for its statusgen PROBLEM-diff pre-flight in the `--brief-path` landing shape. It runs the production pre-flight with a lint stub that reads the tree, checks that the second lint saw exactly the bytes to be committed, and covers both refusing a landing that adds a PROBLEM and passing one that only meets a PROBLEM already in the tree. The direct shape (`--evidence-file` under `--root`, no `--brief-path`) is not covered, because there the gate cannot refuse; that gap is tracked in #2460. New mutation entries back the test.
- `deskinstall` now verifies each release asset's build attestation before placing anything: after the sha256 pin check it fetches `<asset>.sigstore.json` from the same release and refuses unless the Sigstore bundle proves SLSA provenance for those exact bytes, signed by this repository's release workflow (run by the push of the pinned tag, or by `workflow_dispatch` from `main`) and recorded in the transparency log. The check is mandatory, so there is no `--no-attest` or other opt-out. Assets that have no bundle are refused, and a bundle download over 1 MiB is refused while it is being read.
- `deskpost finish review|security-review … --claim <claim key>` is the one command a reviewer ends with. It posts the verdict through the existing verdict verb, confirms the review is recorded at the reviewed head, releases the dispatch claim, and prints one result line. It stops at the first step that fails and says what was and was not done. `--claim` must be the key of this change and of the lane whose verdict is posted, or nothing is posted. The release writes its own audit row (#2437).
- `deskread --ci-workflow-token` reads over the job's own workflow token: an explicit, CI-only, read-only opt-in beside the App custody default. The token comes from `DESKREAD_CI_WORKFLOW_TOKEN` only and must be an app installation token. It serves the `issues`, `trust` and `comments` kinds, for the job's own repository only, and every refusal names its layer as `[ci-transport:<layer>]` (exit 5).
- `gitcore.PathHistory` walks a history once and reports, for each commit, what a path held there and at each parent, so a caller need not resolve every commit separately.
- `internal/packet` is the kit-neutral half of the dispatch packet, so another kit adds a packet by registering an ordered list of named sections.
- `statusgen --transcribe-verdict` re-reads what it applied before it reports success. Every Evidence line must be inside its brief's `## Evidence` section, and the board loader must read every flipped row as `verified` with the stamp that was written. If any check fails, it prints `POSTCONDITION FAILED` and exits 1. The R-6 enactment gate and the trust checks are unchanged.
- `statusgen reconcile --backfill --json` without `--apply` now reports, under `wouldApply`, the stream README rows `--apply` would write, and writes nothing. The rows come from the same function that performs the write, so the read-only report and the write cannot disagree. Each row (in `wouldApply` and in `applied`) now also carries the whole table line before and after the edit (`rowBefore`, `rowAfter`). The pr-review-desk carve-out B reproduction step uses this read-only run by default, with the token from `--token-file` holding the reviewer role's own credential; the `--apply` form in a throwaway worktree stays valid. What admits a promotion row is unchanged.
- `tools/desk/internal/testledger`: a report of test functions that left the tree between two revisions or directories, and whether a `Retires-test:` trailer said why. It lists every deletion (with the test's `// regression:` tag when it has one) and rename (paired by an identical body or a shared tag), untrailed or with each covering trailer's commit and reason; a kept test whose tag lost a ref; and the brief Verify rows that still name a departed test. A trailer never hides a departure, so the report reads `clean` only when nothing departed. Run it from a current-main checkout with `go test ./internal/testledger/ -run TestReportTestLedger -v -args -base=<rev|dir> -head=<rev|dir>`, naming revisions by full commit id. It never fails on what it finds; a range it cannot read, or a name that is both a directory and a revision, is `could-not-check`.
- statusgen brief 18 (authoring only) and design-decision record `DR-gate-rederive`
  (`docs/streams/decisions/`). The record transcribes the maintainer's answer on #2405, option
  c, detect only: the issue scan never changes a stored placeholder gate. The brief specifies
  the notice that answer asks for: `--scan-issues` prints one `NOTICE` for each open issue
  whose labels and title derive `gate: human` while its placeholder reads `gate: model`,
  writes nothing and leaves the exit code alone. Raising the gate stays a hand edit, and the
  first run of the notice lists the existing placeholders to look at. The desk's scan drain
  discards the scan's output, so the notice is read by running the scan directly (a
  `--dry-run` scan prints it and writes nothing); relaying it is not part of the brief.

### Fixed
- On a forge with a merge-hold, `deskpost review` run again after its verdict was recorded no longer exits 0 without looking at the hold. A run can land a verdict and not complete the hold step — the hold write fails, or the forge accepts the post and its answer is lost. The next run posts nothing and checks the hold against the recorded verdict: it re-arms the hold for a request-changes that is the reviewer identity's newest verdict of that lane at the head, re-arms a hold resolved at another head, and exits non-zero where the hold does not match and it cannot tell what to do, or cannot read the hold. It never releases a hold on that path; a hold an approve failed to release is released by posting the approve again.
- The desk's adopter callouts (the writeguard, untrust-scan and risk-classification callouts) now resolve a configured executable path once and make every check against the file it resolves to: its type, its executable and writable mode bits, and the mode of the directory that really holds it. The directory holding the configured name, and the directory holding each link on the way to the file, are checked too, whether the link is a file's own name or a linked directory in the middle of a path, so a callout reached through a symbolic link is refused when any of those directories is group- or world-writable. A refusal names the directory by its real path. A callout configured as a plain path, or as a link into a safe directory, runs as before. The risk-classification callout now uses the same shared check rather than its own copy.
- `cellctl set`: a `$VAR` reference in a `cell.env` or `defaults.env` value is expanded as a
  launch expands it, against the process environment and the lines above it, so `set` no longer
  judges a cell on a value the cell does not boot with.
- `cellctl`: a cell with a nested name (`team/demo`) is re-entered under the cells root its
  launch used. The model-policy hook, a container cell's console, a managed scratch task and
  the cadence and comms panes took the cell directory's parent as the cells root, which for a
  nested cell is a different directory, so the later process looked for the cell, the shared
  provider defaults and the machine-wide defaults file in the wrong place. Each is now handed
  the launch's cells root, and the hook refuses a cell directory that is not under the root it
  was given. Not changed: the `deskd` stand pane and the precheck of a scheduled run still
  name the cell by its `CELL=` value and take the cells root from their own environment.
- `deskpost review` and `deskpost security-review` no longer post a second copy of a verdict when the same command runs again from a session whose audit log does not hold the first post. The check against the change's own reviews now compares the two bodies with the on-behalf-of line the tool appends removed from both, so it matches a review the tool itself posted.
- `deskpr update --pr N` no longer refuses every push to a PR whose published head already carries another App's commit: like the default and `--branch` forms, it now judges only the commits the push adds beyond the PR's verified live head. A foreign commit among the added commits is still refused, an unreadable or diverged head still judges the whole range, and `--pr N --check` still judges the whole range and names `--branch <head-branch> --check` as the offline way to narrow it (#2432).
- `pr-review-desk` skill, carve-out B: the delivery-repo bullet now states where a brief's
  `deliverable_repo:` alias sits in the reading order (after `homed-in:`, before the stream
  README's `repo:`), resolved through `docs/streams/graph-repos.yaml` at the same fetched ref.
  An alias that does not resolve, or one that disagrees with `homed-in:`, is could-not-check
  and bounces the row; it never falls through to the board repo. Same wording as carve-out C
  (#2430).

### Changed
- **Merge ordering:** a `deskinstall` built from this change refuses every release that does not publish attestation bundles, and no release pinned today publishes them. That includes the documented Windows route that builds `deskinstall` from source (`go build ./cmd/deskinstall` from a clone): after this merges, that build refuses every currently pinned release until the release workflow uploads `<asset>.sigstore.json` beside each asset and the pins move to a release that carries them. `docs/adopting-assay.md` says so and names the interim routes.
- **worker-desk skill and Claude Code reference: how a session below strong tier launches a
  strong-tier worker.** The skill said a strong item "goes only to session-tier", which left a
  worker window running at a provider's mid tier with no stated way to dispatch one: its default
  launches landed on the mid model and stopped at the kit's pickup check. The skill now says such
  a session names the strong tier explicitly in the launch, confirms before the dispatch that the
  slot it names is pinned to a strong-tier model, stamps that pinned id, and holds the item when it
  cannot confirm; the Claude Code reference states that a `model` alias is a slot whose model the
  launch pins and carries no tier claim by itself, where the session reads the pin, the form the
  stamp takes, and which documents own the launcher's maps. No tool, kit text or gate changes.
- A `deskdispatch --kit review --pr <N>` run without `--dry-run` now reads the forge before it claims, which adds forge requests to each one.
- A worker dispatch is handed what its kind of run needs (#2439). The kind is derived from the
  dispatch: `--pr` onto an open change is a run that works that change, every other worker
  dispatch implements a brief or an issue. The assignment's action half follows the kind (a
  run on an open change is told to push to it and never to open a second one), kit text
  marked for one kind is not quoted to the other, and the objective kit's own copy of the
  common clauses is no longer quoted a second time. Unmarked kit text — nearly all of it —
  still reaches both kinds.
- Implementer kits (`worker`, `worker-objective`) §9: tag every fail-first test, give every deleted or renamed test function a `Retires-test: <Name> — <why>` trailer (`— renamed <New>; <why>` for a rename), re-point the Verify rows the report names, and paste a non-empty report under `## Tests retired`.
- Plan library-first fact-reader and evaluation APIs, with explicit custody boundaries and consumer migration ownership. This change authors implementation work; it does not ship the SDKs.
- Review kit §4 and `pr-review-desk`: run the report from a current-main checkout against the merge-base and judge each line (still pinned, and by which test; is the trailer's reason sound; Verify rows re-pointed). An unjustified departure, trailed or not, is a `test-evidence` finding.
- The `pr-review-desk` skill no longer tells the desk to ask a resumed reviewer for a delta review: the desk hands on the round the dispatcher's assignment states.
- `cellctl defaults --help` prints the usage and exits 0.
- `cellctl set`: the keys accepted without `--force` are now every key the registry classifies
  as a machine-wide or per-cell setting, instead of a separate hand-kept list. Thirteen keys
  that needed `--force` no longer do: `CELL_GO_CACHE`, `CELL_GO_CACHE_BYTES`,
  `CELL_GO_CACHE_MIN_FREE`, `CELL_GO_CACHE_ROOT`, `CELL_SCRATCH_MAX_AGE`,
  `CELL_SCRATCH_MAX_BYTES`, `CELL_FF_ROOTS`, `CELLCTL_DESKWT`, `CELLCTL_GENERATED_FILES`,
  `CELLCTL_ORCA_TIMEOUT`, `CLAUDE_CODE_AUTO_COMPACT_WINDOW`, `DESK_TOOLS_BIN` and
  `GITHUB_HOST`. No key that was accepted is now refused.
- `cellctl`: a cell name must be a path under the cells root. `cellctl new`, `cellctl set` and
  every verb that loads a cell refuse an absolute path, `.`, and a name that leaves the root
  through `..` (`../other/x`), naming the name and the root. Such a cell used to scaffold and
  load from outside the root, with a different defaults file and provider catalog beside it. A
  nested name (`team/demo`) and a symlinked cell directory are unaffected. A cell that was
  made outside the root is reached by pointing `CELLS_ROOT` at its parent directory.
- `deskdispatch --kit review` emits the review kit cut for the dispatched lane. The security claim key of the PR under review (`<label>--pr-<N>--security`) gets the security cut, in which a pass only the correctness lane runs (design fit, the board-row flip check, the same-head re-approve exemptions, the prompt-audit procedure) is replaced by a short note naming its owner, and the correctness verdict's steps for an undeclared desk decision by the one line that applies to a security verdict; the bare PR key and its `--correctness` form get the correctness cut; any other key, including a `--security` key whose PR is not the one dispatched, gets the whole kit, which carries every procedure and none of those notes. No rule is dropped from a lane it binds: the limits on a finding either lane may post (the design-fit bounds, what a well-formed Status cell is, never executing PR content) stay on both lanes, and a test table pins each section to its lanes. An adopter addendum that points a security reviewer at the procedure text of one of those four passes now points at text that reviewer is not given (#2437).
- `statusgen reconcile --apply` now runs without `--backfill`, writing only rows witnessed by a `Brief:` trailer. It holds any row whose write would add a lint problem (for example a risk-gated brief with no design record), reports it with the problem it would cause, and leaves it unwritten. It exits 3 when it could not read the pull requests, and writes nothing.

## v1.0.34 — 2026-10-09

### Added
- Design-decision record `DR-forge-neutral-34` (`docs/streams/decisions/`): a transcription of the
  driver's ruling on decision issue #2315, which answered option 1 — approve as briefed the
  `deskread` CI workflow-token transport, an explicit, CI-only, read-only opt-in beside the App
  custody default. Brief forge-neutral/34 now cites it through `design:`, which is what the
  design-approval gate reads. The record does not decide the implementing pull request (#2377)
  or the brief's later sign-offs.
- `deskevidence --outcome-record` now writes the verify-wake receipt on every `verify-fail` or `blocked` record. The receipt's inputs are one `file:` hash per path in the brief's `files:` list that exists at the record's sha (every file under a listed directory), the brief as it lands, and the tool version. `blocker_kind` and `blocker_ref` are required. A `blocker_ref` that is not an issue or change reference, such as `to file`, is refused with exit 5. A `verified` record carries no receipt.
- `deskevidence --outcome-record` stamps the landing repository as the record's `repo` when the caller gave none. A record that already carries a verify-wake-v1 receipt lands only if that receipt is complete; otherwise it is refused with exit 5, naming the missing field.
- `docs/verify-wake.md` documents the hold rule.
- `verifyloop plan --no-forge` reads no blocker issue, so every held brief's blocker is could-not-check.
- `verifyloop plan` holds a non-pass brief until something it depends on moves, following the hold table in `docs/verify-wake.md`. It hashes the declared inputs again, and it reads the blocker issue through the forge as the verifier App. A brief whose inputs are unchanged and whose blocker is open prints a `WAIT` line naming the blocker, the wake condition and the next actor. A new summary line reads `verify-desk plan: wait=<n> dispatchable=<n> could-not-check=<n>`. A missing or rejected token, or any other failed read, is could-not-check, never a closed blocker. A newer explicit-recheck receipt still overrides a hold.
- `verifyloop plan` reads a blocker only in a repository inside the configured repository set. A reference outside the set is could-not-check, and no credential is requested for it. Each repository is named on stderr before its first read. A bare `#N` in a record with no `repo` resolves in the plan root's `origin` repository.

### Changed
- The `STATUS.md` Awaiting board now sorts each brief into one of four owned queues (Awaiting human gate, Awaiting implementer rework, Environment-blocked, Runner-pending) or into Desk-actionable, which keeps only rows that need judgement. Every row names its owner and next act, and the headline shows a count for each owner. A row whose Evidence or verify-outcome records cannot be read renders as could-not-check instead of being placed in a bucket. The rules are in `docs/board.md`. The `Env-blocked` heading is renamed `Environment-blocked`. A recorded blocker (`verify-fail`, `fail`, `blocked`, `needs-context`) routes by its kind: `implementation` and `check-definition` to workers, `human-action` to the driver, `environment` to the operator. A recorded hold stays a blocker whatever the Evidence says. A recorded fail is dropped only when the Evidence's live PASS was written on a later day than the record; when the two cannot be ordered, the row is could-not-check. A human-gated brief with a recorded blocker stays with the driver, and its next act names the blocker instead of the sign-off. A runner-pending row needs at least one row left to run, and an unrecognised or future-dated outcome record is could-not-check. Could-not-check rows count toward the verification-debt measure, so an unreadable input cannot switch the alarm off.
- The pr-review-desk skill's generated-table bounce gains carve-out C. A one-row hand flip to `implemented` is admitted for a brief delivered into another repo when the delivering PR could not carry the brief's trailer. The flip PR carries that row and nothing else; it names one merged delivering PR, which must have merged after the brief was authored and added at least one file the brief's `files:` list names; and every concrete file in that list exists on the delivery repo's main. Carve-outs A and B are unchanged.

## v1.0.33 — 2026-10-08

### Added
- **Offline acceptance runner** `tests/stream-brief/32.sh` (legacy, identity, negative, flow, dereference, mutation) with fixtures under `statusgen/testdata/streamview`.
- **Optional authored `mission:` block** in stream README frontmatter (outcome, success criteria with evidence references, commitments, exclusions). Legacy streams need no migration; an invalid block is a statusgen lint problem with explicit diagnostics and is never replaced by README prose.
- **Stream view contract v1** (`statusgen/streamview`, `docs/stream-view-contract.md`): an importable, versioned wire type for one stream's state — repository-qualified identity (`owner/name` + slug, cell context kept outside the key, renames start a new identity), per-section provenance with explicit availability (`available`, `partial`, `could-not-check`, `not-assessed`; missing never reads as a value), a fixed trailing 24-hour changes window, version negotiation, a strict `Decode` consumer, and `CheckBindings` to dereference recorded revisions and evidence in the repositories they name. Every recorded revision must be a full commit id, and the window must end at the view's `generated_at`.
- A findings entry can now carry an effectiveness record (`effectiveness:`, `effectiveness-date:`, `effectiveness-by:`, required together or not at all). `statusgen --lint` fails a `resolved: yes` finding dated on or after 2026-10-08 that has none, and advises on earlier ones. The check covers presence and attribution only, not whether the named command would catch a recurrence.
- A source or bundle dated after the caller's "now" answers could-not-check, never fresh.
- Design-decision record `DR-cellctl-cobra` (`docs/streams/decisions/`): a transcription of the
  driver's ruling on decision issue #2240, which answered option 1 — accept the preserved
  source/admission boundaries and the listed help and parse changes of the `cellctl` Cobra and
  Viper migration, subject to independent review and all required validation passing. Brief
  desktools-v2/16 now cites it through `design:`, which is what the design-approval gate reads.
  The ruling came after the implementing pull request (#2391) was opened, and the record says
  so; it does not decide that pull request's review or its merge.
- Design-decision record `DR-outbound-callout` (`docs/streams/decisions/`): a transcription of
  the driver's ruling on #2375, which answered option 1 ("As proposed") for the deployment-supplied
  executable the outbound-write check may consult. Brief desktools-v2/11 now cites it through
  `design:`, which is what the design-approval gate reads. The record states that the
  implementing pull request #2383 was built before the ruling, and it does not decide that pull
  request's review or merge.
- Existing quality evidence informs a scoped remediation decision; high complexity or touching a file does not automatically authorize a refactor.
- Existing results carry the fields statusgen's remaining forge sites read: `PullRequest.MergeCommitSHA` (merged changes only, never an open change's test-merge SHA), `Issue.ClosedBy` (closed issues only, so a reopened issue no longer names its former closer), `ChangedFile.Patch` with `PatchAbsent` stated when the forge withheld a diff, `Comment.UpdatedAt` (empty when unread, never the creation time), and `ChangeRef.Author`, `BaseRef`, `CrossRepo` and `HeadRepo` (an unreadable fork stays empty, never same-repository or the base). Author `Type` stays empty wherever the forge did not resolve it.
- House-cell startup opens one configured comms service surface in tmux, Herdr or Orca; shutdown stops its supervised processes and preserves mailbox data.
- Its queries tell present, known-negative, stale and could-not-check apart.
- New pure Go module `deskcore` (no third-party dependencies) with four effect-free packages:
- Opt-in cell-owned Go caches persist across task homes, with byte budgets, filesystem free-space floors, active-consumer protection, JSON cleanup reports and retryable storage deferral before dispatch claims work.
- The App inventory in `docs/adopting-assay.md` now records the **workflow App**: the single identity holding `workflows: write`, scoped to exactly `contents: write` + `workflows: write` + `pull_requests: write` + `metadata: read` with zero webhook events, and with `administration`, `actions`, `checks`/`statuses: write`, `members` and secrets/variables withheld. It stands beside the desk-role Apps as a capability, not a desk role or a tier.
- The forge seam grows four typed reads under the freeze rule (forge-neutral brief 33), each with a `deskread` kind as its consuming call site and both backends pinned on recorded fixtures: `ListIssues` (any state, one optional label, the issue population only, with `Incomplete` and `PageCap` when it overflows the page ceiling), `IssueStateEvents` (close and reopen events with their actors, plus the changes that closed the issue, with `Complete=false` on overflow or on any closer whose merged state cannot be read), `ListChangeCommits` (a change's commit SHAs, `Complete=false` when the change's own commit count is larger than the list) and `RepoDefaultBranch` (an empty answer is could-not-check, never `main`).
- The fork-test block is read by a strict, fail-closed grammar (`tools/desk/README.md`):
  fenced code blocks and HTML comments are blanked first, following CommonMark for those two
  constructs (a fence closes only on the same character, at least as many repetitions and
  under 4 columns of indent; an unclosed comment hides everything from its line to the end,
  wherever on the line it opens; a blanked line stays as an empty line, so it never joins what
  came before it to what came after it). The heading must be followed directly by the block,
  and the block is the contiguous run of column-zero `<lowercase-key>: <content>` lines. Prose
  between the heading and the first key line is refused as a malformed block, naming that
  problem. The strip models only fences and comments; a backstop drops the declared subject
  when the raw body has more than one fork-test heading, when a reading of the raw body
  disagrees with the stripped reading, or when the raw body opens a raw-HTML block kind a
  renderer drops whole (processing instruction, declaration, CDATA). One known limit is
  stated: a `subject:` line placed directly under the block's last line, with no blank line,
  joins the block.
- The host contract for a split run, stated in `verifyloop` usage: the host writes its own dispatch record (repo, head sha, dispatch time) before starting the fence and runs the composer with `--repo`/`--sha` from it; signs only after the composer exited 0 with exactly one `sha256=` value and the fence has fully exited; passes all five binding flags, taking repo, head and both time bounds from its own dispatch record and only the digest from the composer; takes the signed body from stdout, never from a `.out` the fence could write; and signs only a payload moved by rename into a directory the composing uid cannot write. The binding is checked only at signing: the host refuses a payload whose digest, repo, head or ts does not match its own dispatch record. It does not limit what the payload's entries claim, and nothing after signing checks head or ts again (statusgen's transcriber checks only the repo), so a hostile row can still invent results that the host then signs.
- The notice lane reads its reversible signal from the block's own `subject:` line alone,
  never the title or body prose, and only when the bounded block carries exactly one such
  line; none or several admit nothing. A shape-only needle (`lint level`, `lint severity`,
  `notice or error`, `port-or-drop`) never admits and vetoes any content needle beside it,
  in its plural spellings too (`-s`, `-es`, and `-ies` for a needle ending in `y`).
  Needles match on word boundaries, with runs of spaces, underscores, dots, slashes and
  hyphens treated as one space. The CI check/job backstop reads the subject the same way, and
  any non-ASCII character in the subject fails closed.
- The reader refuses unknown schemas, inputs over 16 MiB, and any key the schema would refuse.
- Two workable options plus a gate the driver still holds can file on a NOTICE LANE
  (`desk-decided`, off the driver's queue, with the shared `desk-r3-decision v1` marker)
  rather than `needs-decision`. Admission fails closed: the item must carry a positive,
  content-bearing R-3 reversible signal (docs wording, typo, phrasing, a table column — never
  a shape-only `tool default` / `default value` / `flag default` / `rename the`, and never a
  `lint level` / `lint severity` / `notice or error` / `port-or-drop` example either, which is
  always a classification question about some check or job, named or not) AND no one-way
  signal — a one-way caller
  label (`human-only`, `security`, `gate:human`), a `deskkit.HumanOnlySignals` needle, or a
  `deskkit.OneWayPatterns` match (merge, ready-flip, main push, tag/release, weakening a
  security control, secrets/keys/PII, money, identity/auth, deleting or overwriting data,
  sending or publishing outside, live infrastructure, App permissions, approval authority,
  `gate:human`, direct writes to main, draft-to-ready, required reviews, 2FA/MFA, hooks and
  signatures, the trust gate and roster, owners and roles, archive/transfer/visibility,
  auto-closing driver-owned items, charges, and turning a control off). A one-way term
  outranks a reversible one, and the `ruled-check:` line is read for one-way terms (exempt
  only from the `ruling` needle). The same one-way check refuses `--no-fork` and withholds
  the re-routes from the fewer-than-two-options refusal. The patterns are a floor, not a
  guarantee: review of the filing and the digest's veto window are the layers around them.
  `deskdigest` lists notice-lane items in its desk-decisions section with their veto date,
  and drops a `desk-decided` item from the Queue only when that section lists it.
- `assay:author-spec` creates, adopts or revises sufficient specifications before brief decomposition, with inherited contracts, behavioural evidence, compatibility and proportionate design review.
- `cmd/muhar` runs a new `TestSpecAnchorsLand` check on every PR. It dry-runs every control and mutation edit in every muhar spec in the repo against the file that edit names. A stale anchor, or a spec with no control, now fails CI on the PR that causes it. Before, it only showed up as could-not-mutate on a later push-to-main run.
- `config`: a resolver for the desk's tunable knobs:
- `containers/scripts/git-floor-check.sh` proves the git floor: it runs at base-image build time, and with an image ref it checks an already-built image. It fails if any git on `PATH` is below 2.41 or if `--attr-source` is rejected or ignored.
- `deskcore/archtest` holds architecture checks that run as tests in that directory (CI does not run them yet: #2208):
- `deskevidence flip --root <worktree> --brief <stream>/<NN> --sha <sha> [--dry-run]`: wraps `statusgen verifyflip` and lands the flip as a LOCAL commit on the worktree's branch. Both author and committer are the verifier App's bound bot identity, so the draft PR carrying the flip passes the publish-identity gate unchanged. It refuses on main/master, on a detached HEAD, when changes are staged, when the README differs from HEAD, when the branch already holds commits the publish-identity gate would refuse, when the result fails the verified-closure check, or when the change is more than one README line. It also refuses if the COMMITTED flip introduces any `statusgen --lint` PROBLEM, and takes that commit back. Nothing is pushed. The existing Evidence landing on main is unchanged.
- `deskfile new` refuses a caller `--label desk-decided` and a caller body already carrying
  the `desk-r3-decision v1` marker in any spelling the digest reads, or broader
  (`--force-new` included): only the notice lane writes either. The marker's reader pattern
  is declared once, in `deskkit`.
- `deskfile new` requires a `### Fork test` block on every `needs-decision` filing (the
  workable options, the default, the human-held gate that catches a wrong guess, and the
  search proving the question was not already ruled), refusing a filing with fewer than two
  workable options and naming three `--no-fork` re-routes
  (`brief-contradicts-artifact` | `wrong-repo` | `tool-false-positive`) instead.
- `deskread` gains eight kinds over these reads, all transport only: per-repo `issue-list` (`--state open|closed|all`, default open, and `--label`), `changes` (`--state merged|closed|all`, required) and `default-branch`; per-issue `issue` and `issue-states`; per-change `change`, `change-commits` and `change-files` (with `complete` reconciled against the change's own file count). The `comments` kind takes `--change` as well as `--issue`, reads exactly the target named and never retries under the other, and now emits `databaseId`, `authorType`, `url` and `updatedAt` when the seam has them; a comment with none of them reads byte-identical to before. Every addressing mistake exits 5 before any forge call, and the shell-exec ban's ceiling does not move.
- `deskverdict sign` gains `--expect-sha256`, `--expect-repo`, `--expect-head`, `--not-before` and `--not-after`. Each is optional, each one passed is checked before the signer key is resolved, and a mismatch, or an empty or unparseable value, is a refusal (exit 5) with nothing on stdout.
- `docs/brittle-investigation-template.md`: the task template for a brittle-marked module. A strong-tier session reads the module's original intent (brief, decision record, `S-` row) and what has happened since (class instances, fix commits, findings), picks one divergence (`drifted`, `intent-changed`, `intent-right, implementation-wrong`, or `none`) and one recommendation (`reconcile` with a deletion bundle, `redesign`, `accept`, or `clear`), and gives each conclusion the evidence it rests on, so a later policy change shows which conclusions need revalidation. Includes a worked example.
- `docs/investigations/`: where investigations land, one dated file per module.
- `docs/streams/desk-supervision/workflow-app-scope.md` states that App's granted and withheld permissions, its duty (author a workflow-only PR, which a human merges), the invariant that no other App holds `workflows: write`, and the two checks that prove it. They read an installation's granted set from GitHub endpoints that exist: `GET /app/installations/{installation_id}` under the App's JWT, `GET /orgs/{org}/installations` read by an org owner (which also enumerates every installed App), or the `permissions` object returned when an installation token is minted, the last only for an un-narrowed mint, since a mint that requests a subset returns only that subset. Installation scope and key custody are recorded as open readings.
- `domain`: shared identifiers, revisions, facts, requests, the six result kinds and receipts, plus `DecodeStrict`, the one strict JSON decoder: keys must match the contract exactly (case-sensitive), no key may appear twice at any depth, required keys must be present and not null, and nothing may follow the value.
- `facts`: a reader for the `desk-facts-v1` fact-bundle contract, published as a JSON Schema at `deskcore/schemas/desk-facts-v1.json`.
- `policy`: a skeleton for approved-policy data. An approved policy is built only by decoding the exact bytes whose digest was approved; the type has no exported fields, so nothing else can build one.
- `spec/brief-flow-event-v1.md` and `schemas/brief-flow-event-v1.json`: a brief-flow event contract keyed by the brief's stable uuid, with effective-dated aliases, canonical fact ids and digests, source precedence, role-only actors, and refusal of unknown mandatory members and of person identifiers.
- `statusgen verifyflip --brief <stream>/<NN> --sha <sha> --runner <login> [--dry-run]`: the sanctioned `implemented → verified` board flip for a `gate: model` brief (#2074). The Verified stamp is derived from the latest recorded strict `**VERIFY: PASS**` and that run's Date/Runner cells, never typed. It refuses (exit 1) on a verdict, sha or runner mismatch, a non-strict or quoted PASS, `gate: human`, any `risk: yes`, `irreversible`, a held or failing Evidence line, a Verify row without a passing execution witness, or a rewrite that would touch anything but the row's Status and Verified cells. It also refuses a Verify row classed `gate:human` (or of an unknown class), Date/Runner rows recorded after the PASS, a date that is not `YYYY-MM-DD` or is after today, a witness from an earlier run, a README that is not a regular file, and a PASS marker or Date/Runner row whose line `git blame` traces to a commit not authored by the roster's bound verifier or a roster human. No roster, a shallow clone or an ambiguous brief file is could-not-check (exit 2). `Reviewed` is never touched. An `on-behalf-of human:<name>` qualifier stays in the Evidence and is left out of the stamp, because `verify-gate-close.yml` is the only writer of `human:` stamps.
- `statusgen verifyrun --sandbox=container-netns`: an explicit opt-in for a runner already inside a `--network none` container, where the default seccomp profile denies `unshare --net`. It skips the `unshare` wrapper, first checks that loopback is the only interface up (any other interface up refuses the run, every row `could-not-run`), records `sandbox=<mode>` in each sandboxed row's witness Result cell, and is refused in the CI lane, which keeps the `unshare` path (#2350).
- `statusgen` reference implementation: event validation, a fact store with corroboration and held conflicts, a deterministic stage reducer that keeps first milestones apart from episodes and never infers completion from a merged pull request, a read-only board-historian adapter, and a portable JSONL export with a digest manifest that refuses git work trees.
- `tests/brief-flow/07.sh`: offline acceptance runner (identity, stage, compatibility, flow, role identity, mutation) over synthetic fixtures with hand-written expectations; mutation mode turns each reducer rule and each trust-boundary guard red, one at a time, before the genuine code passes.
- `verifyloop verdict --unsigned-out <file>` runs the Awaiting queue's check/check:ci Verify rows, composes ONE canonical verdict-v1 payload over them and writes it UNSIGNED to a new file, printing one line that carries only its sha256. It resolves, opens and reads no key (not `--pem`, which it refuses alongside `--dry-run` and `--window`, as it refuses an empty `--unsigned-out` rather than falling back to the signed path; not `VERIFIER_PEM`; not any config-home `verifier-app.pem`), never prints a fenced verdict body, and on any non-zero exit leaves no file of its own and prints no digest. The rows can therefore run in a fence that never holds the verifier key; signed on the host, the result is byte-identical to the combined path's body for the same rows.
- a per-role variable must use the canonical role suffix, and a second setting for the same knob and role in one layer is refused;
- a pure package must not reach a process, network or effectful package, or any third-party package, through any import chain;
- a pure package must not read the clock, load a time zone, do console I/O through `fmt`, decode JSON outside `DecodeStrict`, or compute an age outside the one helper that refuses a future time;
- a pure package, and any module package it reaches, may import directly only the standard packages on a short allow list, so `os`, `syscall`, `path/filepath` and any other package that can touch the filesystem are refused without being named;
- a type with a constructor must not have exported fields.
- budgets and rates default to off and are counted per role.
- code outside `config` must not read the environment or hard-code a knob's value;
- desk-tools brief 29 is authored (authoring only, human-gated): `verifyloop verdict --unsigned-out <file>` runs a brief's check/check:ci rows and writes the canonical, unsigned verdict-v1 payload to a new file, printing its sha256, without resolving, opening or reading any key, so the rows can run in a fenced container that never holds the verifier key and the host signs afterwards. The brief states the trade plainly: the key never leaves the host, but the row commands share the container with the composer and so control the payload; the host-side binding confines a forged payload to the repo, head and time window the host recorded when it dispatched the run. The composer prints the digest only, never a repo, head or ready-to-run sign command. It extends `deskverdict sign` with optional `--expect-sha256`, `--expect-repo`, `--expect-head`, `--not-before` and `--not-after` (each refused before any key is read), and on every `--payload` call a regular-file-only payload read that neither follows a link nor blocks on a FIFO, on unix a check that the payload's directory is not a link and no other user can write it, and an exclusive `.out` write that never follows a link. It writes down the host's compose-to-sign contract: the host passes all five flags, taking repo, head and both time bounds from its own dispatch record and only the digest, printed exactly once, from the composer. Its Verify rows prove the mode never opens a key at any location the key resolver consults (`VERIFIER_PEM`, `$ASSAY_CONFIG_HOME`, `~/.config/assay`), that the host-signed result is byte-identical to today's combined path, and that the signer refuses a mismatched, linked, non-regular or exposed-directory payload without hanging.
- desk-tools brief 30 (authoring only): the `verify-desk` skill's `## Landing` moves to the verdict-transcription lane — `verifyloop verdict` signs, the project's verdict filer files a verify-verdict issue, `statusgen --transcribe-verdict` lands it on `main` — with `deskevidence` demoted to documented break-glass and a pull-request route for outcome records, irreversible-brief Evidence and Evidence that is not PASS. The shared push-policy sentence (edited at its guardrail source and regenerated into the five desk skills) and the `deskevidence` rate-limit carve-out's rationale follow. `gate: human`, decided at creation; the precondition (a lane armed with a real verdict landed end to end) is attested by the human, not a typed edge.
- every value records where it came from;
- forge-neutral brief 33 (authoring only): the `Forge` reads statusgen's remaining forge-CLI sites need — all-state issue lists, an issue's close/reopen history, a change's own commits and the repository's default branch, plus merge-commit, author, base, fork and per-file patch fields — each consumed by a `deskread` kind. forge-neutral/18 now depends on it.
- forge-neutral brief 34 (authoring only, human-gated): an explicit `deskread --ci-workflow-token` transport that reads with the CI job token, refused outside CI, refused for a personal or OAuth token or a write, bound to the job's own repository, and recorded in the envelope's identity (repository and run id copied from the environment, unverified), so statusgen's CI-only modes can move their reads onto `deskread`. forge-neutral/18 now depends on it.
- forge-neutral brief 35 (authoring only): the ruling resolver moves onto `deskread` in its own human-gated brief, with a User-only author rule and no credential of its own; forge-neutral/18's row 3 excludes `statusgen/decisionruling.go`, which 35 owns.
- out-of-bounds values are refused, naming the knob, the value and the layer;
- precedence is environment `ASSAY_DESK_<KNOB>`, then the config file, then the compiled default;
- statusgen brief 16 (authoring only, human-gated): an issue whose excluded label was removed by anything other than a rostered human is held out of dispatch until the ratifying identity has commented on it since the label was applied. Both issue scanners hold the placeholder instead of creating or re-activating it, the issue board shows the issue as `AWAIT` instead of offering it as un-briefed work, and an unreadable or incomplete label history holds too. The brief puts three settings of the rule to a human decision (the release signal, and whether the hold covers all system-state labels or only the decision-owed ones) and records as open the ageing of held rows and a durable record of holds.
- statusgen brief 17 (authoring only): the issue scanner derives `gate: human` on a placeholder whose issue carries the triage label `risk:high` when the placeholder is first written, by one entry in the base risk-label set; `risk:med` and `risk:low` do not gate. A label set after the placeholder exists does not change its gate: for that case the brief adds an instruction to the triage session, not a mechanism. Whether a stored gate is ever re-derived, and whether existing placeholders are back-filled, is open and tracked in #2405; whether the keyword list should grow stays open in the brief.

### Fixed
- **`cellctl up` on Windows now hands herdr and orca panes a line PowerShell can run.** The role command (and the first deskd window) was always rendered with POSIX single quotes, which PowerShell reads as a string expression and rejects with `Unexpected token 'desk'`. On Windows the herdr and orca arms — and the dry-run lines and hand-start NOTICEs they print — now render the same argv with the `&` call operator and PowerShell single-quote literals; the tmux arm, and every non-Windows host, keeps the POSIX line byte for byte.
- A verifier home passed under another spelling of the same directory (letter case on macOS or Windows, slash direction on Windows) now prepares and checks as that home instead of being refused at Prepare.
- A witness taken on a branch head is credited after a squash merge only when some commit on the item's history carries its tree outside `docs/streams/**` and `STATUS.md`; a branch squash-merged after its base moved, or never merged, is `wrong-revision`, and a shallow clone or an incomplete search is `could-not-check`.
- Add `deskdispatch --stamp-only` so the coordinator can attest the worker model and tier selected by the original dispatcher after its PR opens without allocating another dispatch. Shared stamp writes now verify the standing label pair and appliers before reporting success.
- Add dispatcher-owned pre-work verifier attestation with exact run/source/brief/model binding, fail-closed execution and Evidence admission, and recovery of the same record.
- Brief desk-supervision/10's Verify rows 5 and 6 no longer call `gh api /installation/permissions`, which GitHub does not serve. They now read the grant, the subscribed events and the installation's repository selection from the endpoints above, name where the installation id and the list of other installed Apps come from, and treat a set that cannot be enumerated as could-not-check.
- Consequence for the verifier image: the base image used to ship git 2.39.5, and this repo tracks `*.ps1` files with `eol=crlf`, so every in-image verifier run would have been refused. #2319 fixed #2318 by building a newer git into the base image (git 2.41 or later is now checked at image build), so a verifier image rebuilt from the current base renders converted files and is not refused.
- Dispatch kits now state the exact workpad marker and the four section headings, and point to a copyable template in `deskreply --help`; the `--workpad` marker refusal points to that template instead of an internal library. A source-checkout renderer example now rejects unknown JSON keys.
- Document and test the worker-desk to coordinator post-open handoff without granting child workers stamp authority.
- Evidence admission is bound to its landing target. A run attested for one brief admits only that brief, its own stream-index row, or an outcome record keyed to it, and a direct Evidence commit carries the run binding in its message and audit line.
- Five `tools/desk` files that were not gofmt-formatted are formatted, and a new module-root test fails on any unformatted Go file in the module, so formatting drift no longer lands silently.
- Honor help-wanted labels during live worker eligibility reconciliation.
- House desk launches now own task scratch, reclaim acknowledged completed output, retain bounded failure diagnostics, and conservatively recover abandoned tasks. New scratch commands provide bounded source snapshots, explicit evidence handoff, cleanup reports, and legacy inventory.
- Keep non-gating advisory comments separate from stale-PR dispositions, retain explicit holds and ownership checks, and reserve NEEDS-REBASE for actual base/conflict defects.
- Maintainer follow-up for S-2237-receipt-custody; details retained in internal notes.
- Pin actor separation and detached-source admission with guard-removal mutations; keep verifier outputs outside the admitted source tree and bind intake record filtering to the dispatcher author.
- Resident redaction rules now keep private and org-internal forge, tracker and CI locators inside their trust boundary; cross-boundary issues and comments use opaque role and number references.
- Route stale verification failures to independent re-verification using local main history, and distinguish a missing sign-off card from a current failure.
- Scratch cleanup preserves bare Git repositories and detached metadata alongside ordinary worktrees, including under retention pressure.
- Scratch sources now require a working-tree root and share admission across revision lookup, snapshots, and declared inputs. Inputs remain within that source repository.
- Test and env-isolation hardening for verifier admission and admitted Verify runs: an admitted run's rows execute without the caller's git location and config variables or shell startup variables; rows and admission's own git reads both run with no system config, no terminal prompt and no askpass helper, and a launch's global config naming the home's own file is kept; and each defence-in-depth branch of the work tree checks is now pinned by a test that fails when the branch is removed.
- The `git blame` reader behind the Evidence-actor lint now counts a line as content when text precedes two or more HTML comments on it (for example `text <!-- a --> <!-- b -->`). Before, the second comment reset the verdict and the line's author dropped out of the attributed set. `statusgen verifyflip` no longer reads that filtered set at all: it checks every file line behind the PASS marker and its Date/Runner rows, one by one. It also blames every content line from the PASS run's first line to the end of Evidence, because the witness check reads every table row of the run whatever its header says, and which PASS counts as latest depends on everything after it. Because a deleted line leaves no blame record, it also refuses when the commit that wrote the PASS marker, or any commit after it, changed the Evidence section and is not the bound verifier's or a roster human's, so a later FAIL that someone else deleted, struck through or quoted is caught. That walk follows every parent of every merge, so a deletion done by a merge resolution is caught too: a merge is left unjudged only when its Evidence is one parent's and no other parent's side added anything since their merge base (a merge of unrelated histories, with no merge base, is always judged). A brief that is not at the same path in every commit since the PASS was recorded (moved or removed) is could-not-check, because blame follows a rename and a path-limited walk does not. It also refuses Evidence that differs from HEAD in the working tree. Every author `statusgen verifyflip` and the Evidence-actor lint judge is the author recorded in the commit object itself, never the identity `git blame` prints, so author classification is independent of repository identity rewriting. A commit whose author header another git reader could see differently (a declared non-UTF-8 encoding, a second author header, whitespace around the name or address, or a malformed line) is could-not-check (exit 2) in `statusgen verifyflip`, never a verdict. In the Evidence-actor lint the same header never skips a row: its lines read as an identity no roster entry matches, so a new closure over them is still a PROBLEM. A merge is left unjudged only when every one of its merge bases shows the other side made no net change. The provenance reads ignore the runner checkout's local state: replacement objects, grafts, a configured blame ignore-revisions list and a configured diff algorithm cannot change what they see.
- The `tools/desk` engine tests no longer redden under whole-module `go test ./...` load: every wait for `loopengine.Run` in `internal/loopengine` and `cmd/commsloop` now uses a 60-second wedge ceiling instead of a 2-5 second one, and a new module-root test keeps short literal deadlines out of every test that drives the engine.
- The desk base image now ships git 2.56.0, built from a pinned, sha256-checked upstream release, instead of bookworm's 2.39.5. Verifier admission needs `git --attr-source` (2.41+), so verifier homes built on the old base refused every brief with a converted file (#2318).
- The desk preflight's write-transport check no longer reads a 502 or 503 as a REJECTED identity when the forge's URL happens to contain `401` or `403` (for example a test server on port 14010). Rejection is judged on the wording of the transport's answer: only the address is blanked (a URL, from its scheme to the first quote, bracket, comma, pipe, whitespace or colon that is not a port; or a dotted, `localhost` or bracketed-IPv6 `host:port`), so denial wording that touches a URL through punctuation is still a rejection. Status codes are whole tokens, so `401`/`403` inside a longer number is not a status. A dotted name followed directly by `:401` or `:403` reads as a port, and a status glued to letters (`HTTP403`) is not read as a status.
- The deskkit mutation gate is green again. Eleven mutation anchors in seven muhar specs had gone stale after refactors moved the guards they target. They are re-anchored to the guards' current text, and no mutation was weakened or dropped. The cellcache spec also gains the positive control it lacked, without which muhar discarded every run of it.
- The old-git probe while rendering attested files now treats a home as an old git only when git rejects `--attr-source` itself; any other probe failure is reported as could-not-check.
- The regression floor's wrapped shell suites (the fleet-hardening and shim-credential fixtures) no longer fail a passing run under host load: they share one named, finite 4-minute deadline instead of a 60-second one sized to idle speed, the floor runner and the brief's Verify row bound each `go test` run above it, and a new class guard keeps every wrapped suite on that budget. A hung fixture still fails at its deadline.
- The scan now fails closed (exit 2, never "clean") on the fail-open paths found by the security review of this change. Each one has a fixture in `layer-secret-scan.test.sh` and a mutant in `layer-secret-scan.mutate.sh` that reverts that one step and must fail its fixture.
- The stream index is a source file until Evidence landing, and then admits only the attested brief's own row's lifecycle cells; the row-scoped table core moved into deskkit so admission and row-scoped landings share one definition.
- The verifier dispatch kit and `deskdispatch --kit verifier`'s assignment now ask for the bold `**VERIFY: PASS**` / `**VERIFY: FAIL**` marker, with any qualifier after the closing asterisks. They used to ask for an unbolded verdict, which the statusgen gate never reads, so a verifier that followed the kit exactly wrote a PASS that never advanced the item. A new test reads the gate's own regex from statusgen and fails if any dispatch kit or the assembled verifier prompt asks for a verdict that regex does not match.
- The worktree allocator supports a pinned commit plus an explicit remote upstream, creating the checkout and branch tracking separately and rolling back if tracking fails.
- Verifier admission binds the home's git work tree: git's resolved work tree must be the home itself, and a configured work tree (`core.worktree`, at any scope including the worktree's own config) or a bare setting refuses, so a Verify row's git cannot read files other than the ones admission compared.
- Verifier admission binds the home's index to the attested tree, so a staged removal, swap or addition that `git grep` or `git ls-files` would read refuses, and renders every file with the attested commit's own attributes (`--attr-source`) and the checkout conversion pinned in the dispatch record. A home's own attributes file refuses, and the global and system attributes files, a configured attribute tree, inherited `GIT_*` variables and later conversion config are not read, so none can make planted bytes compare equal. Rendering a converted file needs git 2.41 or later: an older git refuses any file that differs from its blob, and the refusal names this git's limit (it cannot tell a converted file from a changed one) rather than asserting a change. Tests that need a newer git feature skip on an older git.
- Verifier admission compares every worktree file's bytes and mode with the attested commit, read with replacement objects disabled, and refuses a home that carries replacement objects or a filter driver on a changed path. Index flags and clean filters can no longer hide an edited input.
- Worker resume dispatches now use the open change’s source branch and verified head on GitHub and GitLab, refusing uncertain sources instead of falling back to main.
- `DR-workflow-app-landing` carries a note that neither an explicit `workflow_dispatch` nor a desk run by itself shows a human started the run, so the human-initiated property has to be enforced where the workflow App's credential is minted; key custody is left as an open reading for desk-supervision/11. The record's status is unchanged.
- `assay-install.sh pin` fills the bare `statusgen` line the same way: from the pairing manifest's `statusgen-linux-amd64` line, by name, whatever `--platform` it runs for. It used to take the pinned platform's digest, so an install on Apple silicon wrote the darwin-arm64 digest into the bare line and a later `upgrade-assay` flipped it back. A manifest with no linux-amd64 line now makes the pin refuse, naming the bare line, with the pin file left untouched.
- `cellctl set` preserves literal comms manifest paths containing spaces or shell metacharacters.
- `containers/scripts/layer-secret-scan.sh` now extracts every image layer into its own directory instead of one merged filesystem, so a key written by an earlier layer is detected even when a later layer overwrites that path or replaces it with a directory. Before this, whether such an image passed depended on which layer blob happened to be extracted last. The mutation test adds overwrite, sandwiched-overwrite, and directory-replacement fixtures, and a committed mutation script shows that a merged-view scanner fails each of them.
- `desk-containers/02` Verify rows 5 and 6 can now pass. Row 5 builds the base image from the repository root with `--platform linux/amd64`; before, it used `containers/base` as the build context, and the build stopped before the layer scan ran. Row 6 passes `--consumers` an explicit `--base`, the parent of the commit that authored the brief. Without it, the row reported could-not-check on merged main because the brief's claims were not in the diff.
- `deskboard`'s concurrency brief now has real tests behind every Verify row: pooled `prs`, `stalled` and drift output is proven byte-identical to a one-at-a-time run, `throughput` is proven to resolve its roots exactly once, a failing repo or root is proven to be named (the lowest-index one when two fail), and a blind `throughput` stage is proven never to count as read. `deskwt` and `deskfile` gain runtime tests that a `--help` request writes no audit row, while a bad flag still refuses and still records one.
- `deskdispatch` accepts a `--pr N` dispatch onto a brief's tracking repo when the brief's deliverable resolves elsewhere, so a brief-authoring change can be reviewed with `--repo`/`--root` on its own repo and `--brief` still passed (a `gate: human` brief stays detected). The tracking repo must be named for the item — by the brief's brief-v2 id or an `<alias>:` prefix on the item key; a tracking alias that only defaults to the registry's own `self` is refused, with the refusal naming that fix. Every other registry mismatch is still the hard fail. (#2355)
- `deskdispatch` releases its claim on every abort after the claim is placed — including a failed decision-issue gate, model stamp, prompt assembly or prompt write — so a re-run is no longer refused behind a claim no agent holds. (#2355)
- `deskdispatch`: a human-gated dispatch from a repo that does not carry `tools/decision-issue.sh` still refuses (exit 6, before the claim), but the refusal now names every location tried, in the resolver's order and as absolute paths even when `--root` is defaulted — the helper path under `--root` or the authoritative `--claim-root`, and that no pure-Go port exists on PATH — plus the `--claim-root` way out. The claim tool's refusal is phrased through the same helper, and a class guard fails if a new wrapped `tools/*.sh` helper skips it or lists a PATH port the resolver never looks up.
- `deskevidence` lands from the dispatched verifier home: `--root <home>` is where admission is checked, a `--brief-path` fragment may be an absolute path outside that home, and a landing from any other root refuses with a message naming the verifier home. The documented landing and refresh forms are tested end to end through the real admission reader.
- `deskevidence`'s draft lane (the Evidence row or outcome record it opens as a draft change when the default branch takes no direct write, as on GitLab) now opens the desk's merge-hold marker thread on that change, the same way `deskpr create` does. Verifier Evidence drafts no longer get stuck: `deskflip` used to refuse them with "carries no merge-hold marker thread" even after the reviewer approved. Every desk verb now opens its change through a single helper that also opens the hold, and a structural test fails on any reach of the raw change-opening method in the desk tools that skips it: a direct call or a method value, inside a function or a package-level variable. Evidence drafts opened before this fix still need a hold added by hand or a re-created change.
- `deskflip` (and the `deskboard` view of the same decision) no longer reads a re-review CHANGES_REQUESTED as a mixed rejection when its typed finding block lists earlier code findings as `resolved` at the CR's head beside the one open body finding, so a documented same-head re-approve after a body-only fix clears the block without a human dismissal. `deskpost ready`'s external-prerequisite exemption reads the block the same way. Only `resolved` with evidence at the CR's head retires an entry; a resolution recorded at an older head, and every other finding state, still blocks.
- `deskmonitor inbound` and `inbound-monitor.sh` no longer report a repo with more than 500 open issues as `MONITOR-DEGRADED` on every cycle. The read now walks the whole open set up to a ceiling (`INBOUND_MONITOR_LIMIT`, default now 2000) and asks for one row past it: a read is truncated only when it holds MORE than the ceiling, so a set of exactly the ceiling is the whole set. The truncation message now reads "returned more than --limit N". The default sits below the forge client's own open-issue page guard (GitHub: 100 pages of 100 rows, open pull requests included; GitLab: 25 pages of 100 issues), so the verb reads one row past it whole. A listing that fills that guard is refused by the forge client and `deskmonitor inbound` reports it as a failed read, keeping the previous baseline, whatever the ceiling.
- `evidence-automerge`: the refusal classifier now judges only the latest run per check, so a superseded CANCELLED run (one replaced by a later run of the same check) no longer reddens the `enable` check and strands a ready, approved Evidence PR without auto-merge. A check run is identified by its workflow and job name together, and a commit status by its context, so a same-named job in another workflow never hides a failure. A check run without a workflow name is never de-duplicated (fail-closed). A check whose latest run is red or cancelled still reddens.
- `statusgen verifyrun` refuses a `--root` below its repository's toplevel (exit 2, nothing written): rows run in `--root`, coverage judges their paths from the toplevel, and the witness does not record where a row ran. Coverage evaluated from a root below the toplevel, or over a tree that tracks two names one case-insensitive checkout opens as the same file, keeps the conservative scope.
- `statusgen` coverage no longer refuses a Verify witness as `wrong-revision` because an unrelated file changed (a changelog fragment, a release stamp): with no dependency manifest, a row whose command's text establishes exactly what it reads (a closed set of read-only commands on plain repository paths) is judged by those paths and their `.gitattributes`; every other row keeps the conservative scope, and the reason says why.
- `tools/desk` tests no longer fail intermittently with `TempDir RemoveAll cleanup: ... directory not empty`. Every package whose tests run git now installs, from its `TestMain`, a git template that turns off automatic background maintenance (`maintenance.auto=false`, `gc.auto=0`) in each repository the tests create, so a detached repack can no longer race `t.TempDir` cleanup. Fixtures whose child environment drops every inherited `GIT_*` variable (the regression floor's shell and #727 fixtures) pass their own copy of the template explicitly, and `internal/gitexec`'s fixture writes the two keys itself. A structural test fails any new git-using test package that skips the hook, including one whose tests reach git only through the package's own non-test launcher. Test-only; no tool behaviour changes.
- `upgrade-assay` keeps the trailing `# …` comment on every pin line it rewrites; it used to drop it.
- `upgrade-assay` now fills each bare, CI-facing pin line (`statusgen <tag> <sha256>`, a bare `desk-tools` line, …) with the `<artifact>-linux-amd64` asset's digest, chosen by asset name. It used to take the digest of the platform it ran on, so a re-pin run on an Apple-silicon machine wrote the darwin-arm64 digest into the line CI verifies and turned CI's checksum step red. If the release's `checksums.txt` has no linux-amd64 asset for a bare line, the verb now refuses before migrating or writing anything instead of borrowing another platform's digest.
- `upgrade-assay` prints each carry-forward warning once on apply, and none on a dry-run; it used to print it twice.
- desktools-v2/16 states its no-answer default in the Human decision section (it blocks until answered), so the dispatch decision gate can admit it.
- statusgen's tests no longer inherit the caller's `DESK_LOOP`. Run from a verify-desk session, one test used to hit verifyrun's pre-work verifier admission, which execs the real `deskdispatch`, and failed; the test binary now clears the variable for every test, and a test pins that (#2297).
- windows-port/00's Verify row 2 no longer rejects a valid OS-list constraint such as `//go:build darwin || linux`. Instead of matching two literal `//go:build` spellings, it now asks `go list` (with `GOOS=windows`, on both amd64 and arm64, cgo on) whether each `_unix.go` / `_unix_test.go` file under statusgen and tools/desk is left out of the windows build. It still fails on a missing constraint or one that admits windows, and refuses to pass when it finds no file to check (#2297).
- windows-port/00's Verify row 7 drops its expired temporary `-skip` of two deskkit tests, which now pass, and asserts a `--- PASS:` line for each so the row cannot pass with either skipped (#2297).
- windows-port/00's Verify row 8 no longer reddens on files Go already leaves out of the windows build by a GOOS filename suffix, such as `readmemo_ctime_linux.go`. Instead of dropping hits by file name, it now asks `go list` (with `GOOS=windows`, on both amd64 and arm64, cgo on) whether each file with a unix-only syscall line is left out of the windows build. It fails on any such file the windows build compiles, whatever its name, and refuses to pass when the grep finds nothing to check (#2297).

### Changed
- Brief authoring connects existing layering and semantic-owner declarations to component contracts, forbidden shortcuts, consumer compatibility and discriminating boundary checks. Cell maps aid discovery without granting authority or requiring a taxonomy schema; small tools can remain flat.
- Implementer kits (`worker`, `worker-objective`): a worker that finds its item's error class already records a merged fix stops at **strike two** and posts a design note on the class issue instead of a second fix; only a `bleed` reply that names that item, is for a production-down or security fix, and comes from the driver's own login lifts the stop (the class stays `design-owed`). Every PR body now carries a measured `## Weight` section. The defect-class clause now asks the worker to close a class by removing the hazardous path or making it unrepresentable, with a guard only when removal is infeasible (reported as weight).
- Reactivate the ISO 9001 workstream at P2 and align pending brief instructions with owner-controlled prioritization.
- The R-3 human-only and reversible keyword lists moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`, `ReversibleSignals`) so `deskfile`'s notice-lane
  gate and `deskdigest`'s classifier consult one definition; `deskdigest` no longer reads
  the tool-written `## Desk-decided` block when classifying.
- The `--dora-timing`, brief-flow and `--flow` target-repo lookup no longer falls back to `gh repo view`. When neither `$GITHUB_REPOSITORY` nor the checkout's `origin` resolves, the result is the existing could-not-check.
- The `desk-decided` label is created from one shared spec
  (`deskkit.DeskDecidedLabelColor` / `DeskDecidedLabelDescription`) by both `deskpr` and
  `deskfile`, whichever reaches a repo first.
- The desk base image now pins every base image and its BuildKit frontend by tag and digest, and checks the Go, `gh` and Node tarballs against pinned per-architecture sha256s (`sha256sum -c`, immediately before unpacking), as it already did for git. A static test fails if any pin or check is dropped, masked or separated from its unpack, and its class guard fails on the unpinned-fetch forms `containers/README.md` lists. The README lists every pin, says which inputs (the apt packages) are not pinned to exact bytes, and gives the bump procedure (#2320).
- The desktools-v2 board row for brief 12 now reads `implemented`: its deliverable merged in #2033, which left the row at `todo`.
- The learned riskscore model's minimum training corpus is now derived from its feature vector (10 labeled examples per predictor times the 15 predictors, so 150) instead of a bare literal 40. The multiplier is borrowed from the events-per-variable rule of thumb, but the floor counts total labeled changes of both classes, not defect events, so meeting it does not guarantee 10 defect events per predictor. Below the floor a score stays heuristic-only with a could-not-learn status; the model graduates only once the corpus reaches it, and the floor follows the vector if a feature is added or removed.
- The outbound self-containment check no longer refuses a synthetic fixture UUID in a file. A UUID whose first group is exactly eight zeros (`00000000-…`) is admitted in any file's content or path (tests, testdata, schema examples, spec text), decided by content alone: a randomly minted v4 session id starts that way with probability about 2^-32. Every other UUID is still refused, including a first group of all zeros but one digit (`00000001-…`), and the same id in a PR body, comment, title or commit message is still refused. The brief-frontmatter `id:` exemption is unchanged.
- The preflight write-transport check no longer spawns `git push --dry-run`. It proves reachability with an authenticated in-process List of the landing repo and keeps the same clean / failed / could-not-check verdict.
- `ask-decision`: a relayed ruling that approves a `gate: human` brief still `todo` names its re-dispatch owner (the dispatching desk and the claim key); when the decision issue may then close is `intake-desk`'s §Close authority.
- `cellctl --version` on a source build now reports `dev-<commit>` (the first 12 hex characters of the commit it was built from, plus `-dirty` for a modified tree) instead of a bare `dev`, so two checkout builds from different commits can be told apart. A release build still prints its stamped tag unchanged, and a build with no VCS stamp still prints `dev`.
- `count-git-exec.sh` now also counts `exec.CommandContext(<ctx>, "git", ...)` spawns.
- `deskdispatch` accepts `--repo` at a repo the brief itself declares as a deliverable: an
  `[alias]` tag on an entry of the brief's `files:` list, resolved through the alias registry,
  admits that repo for a brief whose home alias names a different tracking repo. The claim keeps
  the tracking key and, like every claim, lands in `--repo`; `--root` must still be a checkout of
  `--repo`. An undeclared `--repo`, an unknown or unpublished tag, a tag outside the `files:`
  list (including one in a fenced example block), or a brief with an explicit
  `deliverable_repo`/`homed-in` keeps the existing HARD FAIL.
- `deskpr` and `verifyloop`'s durable-Evidence step now push in-process (`gitcore.Push`) with the role App's token held in memory and sent only to the resolved forge's canonical URL. No credential helper, keychain entry or `insteadOf` rewrite takes part, a push that would need force is rejected, and the checkout's pre-push hook still runs first.
- `deskverdict sign`, on every `--payload` call with or without the new flags, now refuses (exit 5) a payload that is not a regular file (a link, FIFO, device or directory) or, on unix, sits in a directory that is a link, is writable by group or other, or is not owned by the signing user. It opens the payload without following a link or blocking on a FIFO and reads it once. It creates the `.out` sibling exclusively and never writes through a link, so an existing or unwritable `.out`, which used to be overwritten or skipped with a note (exit 0), now exits 5 with the signed body on stdout only.
- `intake-desk` §Close authority: when the tracker a decided issue's close names is a `gate: human` brief still `todo`, the close waits until that brief's dispatch lands (`deskdispatch` exit 0 under its claim key, or its draft PR open) or its dispatch refusal is filed and linked.
- `statusgen --cluster-pending-queue` now marks each entry with a `completion` mode and echoes its `gate`: only a `gate: model` brief with no risk axis answered `yes` is `flip` (the pod runner may land Evidence and complete the verified flip); a `gate: human` or risk-flagged brief is `evidence-only` (the runner lands Evidence and the human signs). Unknown or missing gate/risk shapes fail closed to `evidence-only`.
- `statusgen --lint` reads each unchanged brief and stream directory from disk once per run instead of once per check. Before this change it made about 11,000 file reads on a 420-brief tree; now it makes 461. Each repeat read is checked against a stat stamp (size, mtime, ctime, inode, device). A file written in the last three seconds is always read again from disk. The lint verdict and output are unchanged.
- `statusgen brief --check-verified` now refuses a verified closure of a brief with a Verify row classed `gate:human` unless the Verified cell names a `human:<name>`.
- `worker-desk`: a decision-gate exit 6 that repeats across more than one brief in one tick is filed that tick as one issue naming every blocked brief, excluding briefs where the helper itself refused (status 5, which still means repair the brief); the decision-issue helper is never hand-run around the refusal.
- `worker-desk`: a re-dispatch on a PR whose open finding class is at round 2 or later runs at strong tier, and a strike-two report returns the item to intake as `design-owed`.
- build-less-brittle briefs 05, 09, 11, 12 and 13: the `statusgen --consumers` Verify row now pins the check to the delivering change (resolved from its `Brief:` trailer on first-parent main, diffed against its parent in a throwaway clone; the PR head against its merge-base before the merge), so it runs green after the merge instead of reporting COULD-NOT-CHECK. Two sibling rows were fixed for the execution witness: build-less-brittle/09 row 6 no longer dies of SIGPIPE under `pipefail`, and build-less-brittle/05 row 6 states its Expect as an exact output line (#1915).
- forge-neutral/33 row 29 states the lone-target exit 6; forge-neutral/35 names the signature layer in front of the edit flag, the three CI-transport kinds its STOP needs, and the verdict-issue read in its title; the stream README's wave-6 text and row-3 count match the briefs.
- statusgen: a `gate: human` dependency at `implemented` whose Evidence carries a strict `**VERIFY: PASS**` that is present in the raw text and also a live verdict (outside struck, fenced and quoted text), with no later FAIL now satisfies `depends:` edges, so its dependents become eligible while it waits for human sign-off. `gates:` and `feathers:` edges still wait for `verified`/`done`; a FAIL, a missing verdict, a strict PASS that is only struck, fenced or quoted (whatever loose-form PASS prose sits outside it), a PASS marker that forms only once a struck span is removed, a non-human gate, or any other status stays unsatisfied. An unrouted HELD or could-not-check line in the target's Evidence does not hold the `depends:` edge; the human sign-off still refuses on it. The roadmap stream pages, the stalled-critical signal and the flow eligible-at replay still read `depends:` as `done`/`verified` only. The worker-desk skill's dispatch-exclusion line states the same rule.
- worker-desk: a class issue labelled `brittle` dispatches at strong tier with the investigation template as its deliverable; a design brief follows only on `redesign`.

## v1.0.32 — 2026-10-04

### Added
- A class guard over the desk tree refuses any new function that reads a forge token (`GH_TOKEN`, `GITHUB_TOKEN` and the like) from the process environment unless a reviewed permit names it.
- A class guard refuses any shipped function that looks a comment up by id on the untyped first-page comment read; such lookups must use the typed read, which walks the whole thread.
- A pull request read now reports whether its head lives in the base repository or in a fork, on both the GitHub and GitLab backends.
- CI now runs the findings-register transition check: `statusgen --corroborate` in its own `corroborate` job of `assay-statusgen.yml` (statusgen/06 deliverable 7, #1855). No PR-authored code runs in that job. The guard binary is built from the merge commit's base parent, with a job-local, checksum-pinned Go toolchain and caches (`GOENV=off`, `GOTOOLCHAIN=local`) and the pinned `gh`, and the PR tree is read only as data. Before building, the job checks that this parent is the merge-base of the base branch with the merge commit, so the guard compares against the commit it was built from, and that the event's base sha is its ancestor. A base retarget or a PR edit re-runs the job, no later event cancels a pending run of it, and a retarget during a run fails it. It runs on PRs that touch `docs/streams/findings/`, and fails closed when the base cannot be read or tied, or no human-login map is configured. Residual: under `pull_request` the workflow file itself comes from the PR, so a PR that edits `assay-statusgen.yml` can change this job; that edit is caught by human review of workflow changes, not by the job.
- CLI routing registry (`docs/streams/desktools-v2/cli-migration.json`) and its check (`tools/desk/internal/clicontract`): every Go command and script launcher in the tree is discovered independently and routed to an owning migration brief or a classified exclusion; TestCLIInventory fails on an unrouted, stale, orphaned or over-budget row, and TestCLIOwnerMigrated runs each migrated package's contract tests.
- Check descendant files for directory readers on both workflow events, and retain rejecting controls. Give preserved shell fixtures bounded runtime headroom with a cancellation control.
- Design record `DR-desktools-v2-03` for the read-path custody ruling.
- Design-decision record `DR-cellctl-go-port` (`docs/streams/decisions/`): the operator's
  ruling that `cellctl` is ported from bash to Go, with the bash kept as the oracle until the
  Go port reaches parity. Brief desk-containers/10 now cites it through `design:`, which is what
  the design-approval gate reads before that brief can move to `in-progress`. The record
  describes the tree as it stands (the Go `cellctl` already ships in the release tarball since
  #1377) and does not decide the brief's cutover sign-off, which stays its own human gate.
- Desk CLI adapter (`tools/desk/internal/cli`): a fresh Cobra tree and command-local Viper per invocation, help and version answered before any config, credential or effect, allowlisted configuration sources with unset kept apart from empty, no secrets on argv, and parse errors exiting 2 (desktools-v2/15).
- Findings-register parks are capped: a `parked-until` that is added or extended may run at most 90 days past the date of the commit that sets it (a value not yet committed counts as set today, and a commit dated in the future counts as today). A later date is a `--lint` PROBLEM and MISSING in the `--corroborate` register lane, whoever authorized it. A landed park left as it is, or narrowed, is not re-judged.
- Register 26 inherited desk-tool and statusgen behaviors with parent-counterfactual evidence, a manifest guard, and fixture-only regression entry points.
- Route a durable task controller, runner conformance/adapter, immutable candidates, bounded specialist review and exact-candidate publication into nine implementation briefs.
- Rule-register rows `R-ambient-token-read` and `R-signoff-read-whole-thread` in `docs/contracts.md`.
- Run every floor fixture that starts git, directly or through a shell, without the caller's GIT_* variables, so a GIT_DIR exported by a git hook cannot redirect fixture writes; a hostile-GIT_DIR control and a structural guard pin it.
- Stage the additive statusgen full-test CI case; its enforcement remains held until the maintainer applies the workflow patch.
- Start every floor runner's go tool through one wrapper that clears the caller's GIT_* variables and global git config, so reused manifest rows cannot write to a repository an exported GIT_DIR names; a runner-level hostile-GIT_DIR control, a choke-point guard and a mutation mode pin it.
- The shared CLI contract (`docs/streams/desktools-v2/cli-contract.md`) and migration briefs desktools-v2/18–54, one complex command or up to five simple ones each, with desktools-v2/17 now depending on all of them.
- The shared loop-admin runner protocol (`spec/loop-admin-runner-v1.md`, `schemas/loop-admin-runner-v1.json`) and the independent `loopadmin` Go module that implements it (`loopadmin/runner`): start, observe, cancel and reconcile under a stable launch identity, strict standing-desk and workflow-stage modes, capability declarations with mandatory-field refusal, a generation fence on result acceptance, credential exclusion that decodes and walks extension values (nested keys, escaped tokens, Basic credentials, auth, bearer, private-key and access-key slots in any spelling or prefix, a secret written after a setting word, URLs carrying a password), with a short list of described exceptions (`auth: disabled`, `auth_method`, `pallbearer`, a port with an `@` in the query), strict decoding (exact-case keys, no repeated key, no null for a defined field or a list element (refused at decode), no blank tool name (the schema refuses empty, validation refuses whitespace too), nothing after the request), terminal states that absorb any later report, a result that must echo its invocation identity and a reported model, refusals that never echo request or result content, adapter and fence errors sealed to the contract's sentinel (still matchable with `errors.Is`) on every verb, and records the caller and the adapter cannot rewrite after the call. Unknown outcomes, missing usage and unconfirmed cancels stay unknown. It ships an in-memory fake adapter, an offline conformance kit, a fixture that bypasses the contract's fence to show the caller's own claim-generation check still refuses a fenced result, and a mutation gate. No provider client or provider call. The module's CI job is staged at `ci/staged-workflows/loopadmin.yml` pending maintainer promotion.
- `assay:cut-release` — a release-cutting skill in the public bundle, written against the single `vX.Y.Z` umbrella release. It sets the order of the chain: four live preconditions, then both release-workflow runs handed to the driver as exact commands — the dry run first, with its dry-run input set explicitly because the workflow defaults it to off, then the publish — since both wait at the release environment's human approval and record their dispatcher as the release's authorizer. Publication inputs such as the image push stay off unless the driver decides otherwise. Hashes are taken only from the published `checksums.txt`, then every consumer is re-pinned. The chain is done only when every consumer is green on the new pin. A bad release is fixed forward; a tag is never moved. Adopters cutting their own releases use the same skill. It is registered in the skills README, SOURCES.yaml, the Codex and Cursor packaging rosters, the per-harness capability tables, and the Codex smoke protocol's skill list.
- `statusgen --corroborate --pr N` gains a findings-register transition lane: for a PR that touches `docs/streams/findings/`, every caution-removing move of a finding's `resolved` / `affects` / `ack` / `parked-until` since the PR merge-base needs a human named in the entry's authorizing key to have approved or approved-by-comment on that PR. An anchor already on the entry no longer authorizes a later change by itself. Whether a PR touches the register is read from the local tree against the merge-base, so a forge file listing that stops short (it caps at 3000 files) cannot hide a touched entry. An unresolvable merge-base with a listing that cannot be shown complete, or an unparseable touched entry, fails closed.

### Fixed
- A plain `statusgen --lint` (no `--forge`) no longer reaches the forge: before this change it ran the claim read (`git ls-remote --heads origin`) and dead-claim decay (`gh pr list`, or the GitLab API). Offline, the claim set now reads could-not-check, and `--require-claims` fails closed on it. The offline-lint test now records every forge/network process and HTTP request, so a read that starts and then fails is caught.
- A role transport failure after the URL writes restores the worktree's prior push destination, and no inherited credential helper answers inside the retained worktree.
- Desk-tools tests that depend on a newer git now skip with a named reason on an older one instead of failing for the wrong reason. On a Linux verify witness whose image ships git 2.39, the `deskevidence` merge-tree proxy test reports "needs git >= 2.43 for merge-tree --attr-source; have 2.39.5", and the `deskwt add --role` / `role-init` transport tests (plus one `deskkit` push-transport case) report that they need git 2.46 for the empty-entry URL list reset. On git that meets the floor every one of these tests still runs in full. A new stdlib-only `internal/gitversion` package holds the shared `RequireGit` guard.
- Deskboard no longer shows MERGE-NOW for a ready, risk-classed PR that has a correctness approval at head but no `Security-Review: pass` at head. The security hold used to apply only to drafts, so a ready PR that took a new head and a correctness re-approval, with no fresh security pass, read MERGE-NOW. Such a row now reads SECURITY-REVIEW-REQUIRED whether it is draft or ready, and the note says the review is required "before FLIP or merge". A security pass recorded at an older head still does not count.
- Deskboard requests another review when any commit on a PR branch's first-parent chain after its reviewed head touches the PR's own files, including fixes next to keep-current merges, edits later reverted, and own-file conflict resolutions in the branch's merges. Main-side commits brought in by a keep-current merge, including other branches' catch-up merges, are not read, so a pure keep-current merge stays merge-current. Incomplete history, or a chain that does not reach the reviewed head, degrades to another review.
- Isolate App credential configuration tests from inherited configuration overrides and guard new credential tests against the same fixture leak.
- Keep malformed beacon data intact on refusal and document recovery and the need to upgrade every writer sharing the state directory.
- Resync the staged Windows CI leg with live and reject undeclared staged workflow drift, including an orphaned pending companion, before promotion.
- Role transport refuses unsupported explicit HTTP/HTTPS service endpoints before URL or credential provisioning.
- Serialize shared roster-beacon updates across processes and publish complete files atomically, preserving receipts, resource vitals and concurrent work changes.
- The `deskevidence` merge-tree negative control now requires the conflict exit code (1). Before, it accepted any non-zero exit, so a usage error (exit 129 on old git) kept it green without the proxy ever running.
- The body/diff scanner's decrypted-Kubernetes-Secret rule no longer refuses a ConfigMap's plaintext `data:` just because a correctly sops-encrypted Secret sits elsewhere in the same text. A `data:`/`stringData:` mapping is now skipped only when a YAML parse proves that its owning object has exactly one `kind`, that the kind is not Secret, that the object carries no other key the Kubernetes decoders could read as its kind (a tagged key, or one that differs from `kind` only in case), and that no Secret, typed list such as `SecretList`, or object whose kind the parse cannot read as a plain name, encloses it. The proof has to hold in every way the text can be read: as a diff (only whole new-file hunks are parsed), as a whole YAML stream, and as markdown fences. Every line that declares a Secret has to be accounted for by that same parse. Anything the parse cannot prove keeps the earlier refusal. That includes text that does not parse, mappings in modification hunks, duplicate or merged kinds, readings that disagree, any YAML anchor, alias or merge key (one node can then belong to more than one object), any text holding a line-break character other than LF or CRLF (NEL, LINE SEPARATOR, PARAGRAPH SEPARATOR, or a lone CR), which would make the parser's line numbers disagree with the scanner's, and any text that is not valid UTF-8 or holds a NUL (a UTF-16 byte-order mark makes the parser decode a different text from the one the scanner reads). One consequence: editing an existing Secret beside a new ConfigMap still refuses.
- The desk secret scan no longer refuses a CamelCase identifier carrying a version segment (a `V` plus one or two digits between words, as in a Go test name built on `V1`) as a high-entropy run. Other capital-plus-digit shapes, a 3+ digit group, a second acronym-or-version unit in the same name (including an acronym that closes a name carrying a version), and every random or hex token of the same length are still refused.
- The desk-containers board row for brief 10 now reads `implemented`, where it had stayed `todo`. The Go port of `cellctl` landed in #1377.
- The evidence auto-merge shell guard detects bare-command status captures under errexit, with controls for explicit shells and scoped capture. Every workflow step now starts its own scan context whatever its first key, so a step never inherits the previous step's `shell:`; long-form `set -o errexit`, `shell: sh`, commented or exported captures and commented block headers are covered too. A staged CI workflow runs both offline shell suites when workflow copies or their tests change.
- The evidence-automerge `Request auto-merge` step no longer dies before its refusal classifier runs. Under the default `bash -e` shell a refused `enablePullRequestAutoMerge` ended the step at the capture line, so a refusal the classifier calls benign (unstable or clean status) still reddened the run with no log of GitHub's answer; the capture now records the real exit status and the classifier always decides. The live `.github/workflows/evidence-automerge.yml` is promoted in this change (byte-identical to the staged copy) and also carries the earlier answer log that echoes GitHub's reply before the classifier decides; the ci-load activation copy is rebuilt as that live file plus only its pre-filter `if:`. A new offline test runs the extracted step under `bash -e` against a failing `gh` stub, with a reverted-fix mutation and a guard that flags any other capture-then-`$?` step in the staged, activation and live workflows.
- The public-repo self-containment scan no longer refuses a body over a hyphenated compound that merely contains the scratch-worktree prefix word (a finding-block class label, say). A scratch worktree name still refuses at the start of the text, after whitespace or punctuation, and as a `/` path segment (#2080).
- The public-repo self-containment scan now recognises Windows absolute machine paths — a drive-letter path under the Users root and a UNC path naming a host and share, with either separator direction and any run of separators (so a forward-slash, mixed or string-escaped spelling refuses too), each confirmed with `IsAbsFor("windows", …)` — as the same "absolute machine path" refusal the POSIX roots already raise; bare roots, placeholders, URLs and a doubled separator inside a path stay tolerated.
- The regression floor's hostile Git repository setup now disables automatic maintenance
  and garbage collection for its own setup commands. Isolation failures report every
  changed relative path, mode and content hash, including all `.git` metadata, without
  printing file bodies or symlink targets. Both shell and floor-runner checks still fail
  on any tree change.
- The review-finding ledger no longer merges findings from different review lanes that reuse an id. The correctness and security reviewers each number their own findings, so on a PR reviewed by both, `A3` named two unrelated findings and the derived ledger folded them into one (one lane's class, evidence and state leaking into the other). Findings, their per-class round counters and their arbiter packets are now keyed by lane and id. A reviewer record speaks only for its own lane (a block naming another lane is reported could-not-check), a worker reply's stated lane wins, and a lane-less worker reply attaches to the one lane holding the id or is reported could-not-check when two lanes hold it. The lane vocabulary is closed (`correctness`, `security`, `fact-check`, `fail-first`, case-folded); `deskpost` refuses `/` in an id, class or lane at write time, and the ledger re-applies the same rules on read — a finding whose id or class carries `/` is reported could-not-check and ignored, and a block lane outside the vocabulary is reported and ignored. A reviewer record whose lane cannot be established (a body claiming both verdicts) carries a reserved record lane no block can name, so it can still raise an open finding but never resolves either lane's finding. `deskpost` refuses a review whose finding block states a lane other than the posting verb's own: `security-review` admits only `security`, and `review` only `correctness` (a deep-set lane stated through `review` would be keyed under correctness and read back as could-not-check). A record whose own lane is outside the vocabulary is reported could-not-check and not attributed. The ledger reports could-not-check when one thread mixes laned and lane-less reviewer records. `reviewloop`'s rendered and compact ledgers print the bare id and class with a separate `lane=` field, so a successor agent that copies them reuses the finding instead of forking it. `deskpost`'s content-defect check reads a review's lane through the same classifier as its verdict gate, and fails closed — reports an open defect — on a block the reviewer write gate would refuse, on a record with no established lane, and on any could-not-check entry in the derived ledger; such an entry is never retired, so it withholds the exemption on that PR from then on. A structural test fails if any reader builds a ledger record without a lane.
- Verify rows that could not fail, or that failed on their success path or passed when the checked stage never ran (#1862). Rows ending `; echo "exit=$?"` always exit 0, so a backticked `exit=0` Expect checked nothing; their Expect now reads `output is `exit=0``. Zero-count rows that pipe a `grep` into a later stage (the 21 rows #1862 names, plus graph-execution/01 row 5) exit 1 on success under `pipefail`; each grep stage now tolerates only the no-match status, the Expect is `output is `0``, and a missing path, a failed upstream command or an absent script fails the row. Single-file `grep -c` rows state `exit 1` with the printed `0`. Each re-written Expect carries a dated note, and each touched brief's `version` is bumped.
- `cellctl new --kind house` works on a Windows host without the symlink privilege: when a symlink is refused for lack of it, a directory is linked by junction and a file by hardlink, and the gh CLI config is linked from `%APPDATA%\GitHub CLI`. A scaffold that still fails now removes exactly the paths it created, so a retry is no longer blocked by a half-made cell, and ends with a one-line remediation. A cell directory that already existed is never cleaned up.
- `cellctl new` on Windows writes `CELL_REPO`, `CELL_ROOTS` and the other paths it is handed into `cell.env` (plus the cell-directory paths the k8s scaffold derives) with forward slashes (`C:/src/x`), and the `cell.env` loader on Windows reads a `\` before a byte `printf %q` never escapes (a letter, a digit, `#%+-./:=@_~`) as a path separator rather than a shell escape, while every `%q`-quoted value still loads as written — a `C:\…` path no longer loads back as `C:src…` or trips `malformed CELL_ROOTS path`. `cellctl set`'s kind-change precondition now reads values through the same loader.
- `cellctl` stops and surfaces failures from an installed `deskwt role-init` instead of silently creating an unwired worktree.
- `deskack` reports malformed receipts as correctable usage errors (exit 2), preserves the 12-word cap, and detects misplaced receipt flags. Desk skills explain correction before continuing; guard and identity failures still stop the pass.
- `deskboard actions` reports risk before correctness approval, enabling concurrent correctness and security review dispatch while retaining fail-closed inputs and independent ready checks.
- `deskpr --help` now documents `--decided F` on `create` and `edit` (which already accepted it): the numbered `decision:` / `alternative:` / `cost:` file grammar, the `## Desk-decided` section it writes or replaces, the `desk-decided` label it applies together with it, the refusals it can raise, and that a one-way or human-gated call is never eligible. Each verb's USAGE line now lists every flag that verb accepts, so `deskpr update` shows its `--pr`/`--branch`, `--root`, `--explain` and `--force-scan-override` too. The `deskflip` desk-decided refusals, the desk README and the worker and reviewer prompt kits name the full remedy, `deskpr edit --body-file <the PR's current body> --decided F`. Two new `deskpr` tests guard the class per verb: each verb's USAGE line must name exactly the flags that verb registers (read from the source by parsing it, for every flag constructor and every way a flag set is bound, with any flag set it cannot bind reported rather than skipped), and every `deskpr <verb> ... --flag` remedy named in the desk's Go sources or shipped skill and reference text must use only flags that verb registers.
- `deskpr update` and `deskpr edit` no longer resolve the PR from the worktree's branch name alone. `deskpr update --pr N` (or `--branch B`) pushes HEAD to the head branch the forge reports for that PR, so a worktree on a differently named branch, or a detached HEAD, can push a follow-up when another worktree holds the PR's branch. The push is refused unless the PR's head branch is in this repository, the remote already holds that branch at the head commit the forge reports, HEAD descends from that commit, and the commit is not already in the default branch. With `--pr N` the publish-identity check judges the whole range, so a PR that already carries another identity's commits is refused. `deskpr edit --pr N` now also runs from a feature branch or detached HEAD that is not the PR's own, including one with no commits ahead of the default branch, but only for a PR in this repository whose body already carries a link trailer. A trailer-less or fork PR is still edited only from its own checkout, and a checkout on the default branch or with staged changes is still refused.
- `deskreply`, `desktoken`, `deskpost` and `deskwt remove`/`prune` now treat a multi-token `--help` (for example `deskreply <repo> <n> --help`) as a successful help request: the usage screen prints, the exit code is 0, and no `refused` row is written to the audit ledger. A new structural test fails if any other desk verb's subcommand flag parse lacks the same recognition.
- `deskwt role-init` now provisions App transport for new and reused role worktrees, preserving the parent checkout's disabled push destination.
- `deskwt role-init` refreshes the requested role's credential before the initial HTTPS fetch and overrides inherited helpers for that command without changing source checkout configuration. SSH origins use a derived HTTPS App endpoint; multiple fetch URLs retain Git's first-URL behavior. The fetch isolates inherited authentication while preserving TLS/proxy settings. Local and explicit offline initialization remain supported.
- `statusgen --corroborate` resolves the PR merge-base only from the fully-qualified `refs/remotes/origin/<base>`, with no short-name or bare-branch fallback, so a tag named `origin/main` or a local branch named `main` can no longer make the PR its own base. The ref must exist EXACTLY (`git show-ref --verify`) and merge-base runs on its object id: with the ref absent, git would otherwise expand the full name to a ref such as `refs/tags/refs/remotes/origin/main`. The offline register guards and the other hard-coded `merge-base` reads of `refs/remotes/origin/main` (register-ID grandfathering, stream-cap, unrun gate, Verify-row obligation derivation) resolve it the same way, memoised for the length of one run so a `--lint` resolves the ref once; the revisions an operator supplies (`--consumers --base`, the diff-lint base, `mergecheck --base`) still go to git by name. The findings-register lane reads the tree from the repository top level whatever directory it runs from, refuses a base the PR's tree is identical to while the forge lists changed files, treats only a missing entry file as a deletion, and prints entry text on one report line.
- `statusgen verifyrun --in-container` no longer refuses `could-not-attribute` on a host whose git identity is set only in global config (the common Windows setup). The launcher resolves `user.name`/`user.email` on the host and carries them into the container as ephemeral git-config env, name-only, so the identity never appears in the printed `docker` argv; a value containing a control character is refused on the host before docker runs.
- `statusgen verifyrun --in-container` now forwards the host's `GITHUB_ACTIONS` and `GITHUB_ACTOR` into the pinned harness container (name-only `-e`, so values never appear in the printed `docker` command). Before this, a GitHub Actions run whose checkout had no git identity was refused inside the container with `could-not-attribute` (exit 2), even though the host could attribute the run. This turned the `windows-ci-leg` `verify-in-container` witness step red (#2092). A structural test now fails if the runner derivation reads an environment variable that the launcher does not forward.
- `statusgen verifyrun`: the `check:ci` network-off sandbox now brings loopback up inside its fresh network namespace, so rows whose tests start a local server (`httptest`, a fixture listener on 127.0.0.1) no longer fail with `connect: network is unreachable`. Nothing else is opened: the in-namespace helper proves the namespace is not the caller's and that no non-loopback interface is up or addressed before it runs the row, and any failure there makes the row could-not-run — never a run outside the sandbox.
- `statusgen` evidence coverage judges a reused witness by comparing the witness's tree with the item's over the paths it speaks for, not by commit ancestry: a witness written on a squash-merged branch is no longer refused as `wrong-revision` when no such path differs. The witness commit must still share history with the item's revision (an unrelated root is `wrong-revision`). A witness token that is absent from the clone, resolves through a ref name, or names a non-commit object is `could-not-check`, naming why. The scope is unchanged: with no dependency manifest, any differing path outside `docs/streams/**` and `STATUS.md` still refuses.
- desktools-v2/10 Verify rows 2–9 and 11–13 now fail when the property they name is false: each `go test` row asserts its own anchored `--- PASS:` lines and prints one decidable line instead of passing on the exit status or on "no tests to run", and the mutation row reads the harness's healthy and Totals lines. The consumers row judges the brief in the tree under test against a base from before the brief existed and asserts the exact summary line, so a disproved routing claim, an unrunnable check and an all-UNCHECKED run are each red; it cannot see consumer wiring removed from the code (with the decorator removed from `ResolveForge`, rows 2, 6, 7 and 8 turn red and the consumers row stays green). Row 13 counts the Windows machine-path test that actually ran, so a missing or skipped test is red.
- desktools-v2/14 Verify row 8's Expect now leads with `exit 0` and no longer says "exit 1" for the inner `go test` (#2077). The Expect parser reads the first `exit N` outside code spans and does not match "exits 0", so it took the inner mutation run's "exit 1" as the row's required status and scored a passing hand-run as a fail.
- statusgen's HELD/could-not-check scan (verify-gate card, model autoflip, `--close-verify`) no longer reads the words inside an inline code span, so quoted tool output such as `` `no record is held under the data grant` `` stops suppressing a clean PASS. A span is excluded only where it certainly renders as inline code: an unterminated or mismatched backtick strips nothing, a span never crosses a table-cell pipe, a backtick that may close a span opened earlier in the paragraph leaves the rest of that paragraph read in full, a `<` that may open an autolink or raw HTML, a backtick that may sit inside a bare URL or `www.` address that GitHub turns into a link, and a link destination, title or reference label that may hold a backtick each leave the text after them read in full, a line that may be a link reference definition or that holds a bare carriage-return line ending strips nothing, only a line of spaces and tabs ends a paragraph, and a span holding only `HELD` or `could-not-check` still counts (#2100).
- statusgen's fixture commits no longer depend on the machine's git identity (#2181). The `runGitEnv` test helper now defaults a fixed author and committer identity, which a caller can still override, so `TestParkHorizonFromCommit` and `TestParkHorizonFutureCommit` pass on a runner with no global `user.name` / `user.email`. Before this, the release workflow's `test (statusgen)` job failed there with "Author identity unknown". Test-only; no change to the statusgen binary.

### Changed
- A typed comment-thread read now walks a pull request's whole comment thread, as it already did for an issue, so a sign-off posted after the first 100 comments is found instead of being refused as deleted. A thread longer than the page cap, or a next page with no cursor, is a could-not-check. This covers deskmerge's and deskclose's sign-off reads.
- Add `deskroster repos --scope roots` to report the configured board-root map, and use it for worker-desk boot coverage so an explicit `DESK_ROOTS` override reaches both inventory and queue readers.
- Add an optional, disabled Laya subprocess boundary and offline artifact/input validation stubs; model bundle, sandbox and activation remain subject to owner approval.
- Add opt-in Go supervision and launcher context for within-cell desk communications, Windows named-pipe transport, and `deskcomms poll --json` with full message payloads. See `docs/cellctl-comms.md` for provisioning and cutover prerequisites.
- An unmeasured gate-share axis or ladder rung now names its cause (`listing-cap`, `deadline`, `gh-failed`, `malformed`) instead of the generic `gh-unreadable`, so a window too large for the reader no longer reads as an authentication failure.
- Author ISO briefs 08–11 for source records, project review packets, change impact and offline qualification.
- Bind existing instance, recovery, dispatch and experiment work to the same execution contracts, retaining independent review and live-adoption gates.
- Bound autonomy gate-share reads to the reporting window and fetch check rollups per pull request; capped, timed-out, or incomplete reads remain unmeasured.
- CI: `tools/desk` is cross-compiled for Windows and macOS on every PR; the native Windows desk suite runs on the Windows leg at release tags and manual dispatch.
- Classify the six briefs that carry launch, dispatch, identity, budget or reviewer-verdict authority as human-gated, the classification the stream's existing dispatch and ownership briefs already carry.
- Codex house desks now run under the Go cadence supervisor by default (five-minute interval, 20-minute pass budget), including per-role model-policy routing. Explicit cadence settings and interactive opt-out retain precedence; other harnesses keep their defaults.
- Document preservation-first retention and quiescent archive requirements without deleting receipts, stable locks or abandoned publication files.
- Enforce abstention, every stdout/stderr copy bound and inherited-pipe/expired-result deadlines at the optional process boundary.
- Extend desktools-v2 with Cobra/Viper adoption for every maintained CLI: a complete migration inventory with bounded owners, a cellctl pilot, and a suite-wide behavior gate.
- Harden the local comms endpoint: the gateway requires an owner-only socket directory and a 0600 socket (a current-user-owned pipe on Windows), clients verify the endpoint's owner before writing, and a clean gateway stop removes the socket so it can restart. Standalone gateways now also require the owner-only socket directory.
- Keep valid roster table rows visible when a beacon read or prune fails, return exit 6 and label uncertain ownership explicitly.
- Only `parked-by` authorizes a park add or extension, in `--lint` and in the `--corroborate` register lane. `authorized-by` keeps authorizing `resolved` / `affects` / `ack` transitions, but no longer a park.
- Order the local harness adapter after the shared supervisor and its single process runner, and name the supervisor's local execution journal as the second permitted local store in the scope exception.
- Preserve parked status, existing control ownership and human authority; no runtime capability is claimed.
- Refuse roster-beacon and lock leaf symlinks, Windows reparse points and non-regular files; use strict beacon parsing for supervision resource reads.
- Require offline, side-effect-free command help and preserve configuration precedence, credential custody and existing automation contracts throughout the migration.
- Skip desk communication polling when the project's recorded state explicitly keeps the lane
  pre-cutover. Desk skills continue their work-queue sweeps using the existing pre-cutover
  hand-off path; enabled-lane failures still stop the pass. Unknown or conflicting comms state
  now stops only comms (hand-offs go through the tracker, reported once) and never halts the
  sweep; the guardrail names where the record is read, that an absent `comms:` key (or no cell
  topology) reads as disabled, and that an unreadable or unparseable topology file is unknown state.
- Specify one shared loop-admin execution component for standing desks and workflow stages, with desk-only adoption independent of graph storage.
- Specify versioned source applicability and review preparation over existing requirements and evidence exports.
- Stream board: `desktools-v2/13` flipped `todo` to `implemented` — its delivery PR #1996 merged with the `Brief: desktools-v2/13` trailer and the row was never flipped; what #1996 itself listed as still owed (the human-applied `ci.yml` cross-compile leg, triage comments on the closed issues, and the `desk-containers` row 10 correction) is what the brief's Verify rows 1, 13 and 11 will show at verification. `desktools-v2/03` stays `todo`: #1914 delivered only the settled part of that brief, the rest is held on its open design decision, and the lint refuses the risk-gated row at `implemented` until an approved decision record is cited.
- Stream board: `desktools-v2/14` (regression floor) flipped `todo` to `implemented` — its delivery PR #2004 merged but the row still read `todo`, so a delivered brief stayed dispatchable. Its Verify rows run at verification.
- Stream boards: the commsloop router risk-derivation row (measured-status 03) now reads `implemented`, matching the derivation record and follow-up already merged.
- The CI that `statusgen init` scaffolds now fetches that base for its `--lint` job: the GitHub workflow's lint checkout sets `fetch-depth: 0`, and the GitLab `statusgen-lint` job sets `GIT_DEPTH: "0"` and fetches `main` into `refs/remotes/origin/main` before `statusgen --lint`. Without it, a default shallow checkout has no `refs/remotes/origin/main`, so once the repo records its first finding, `--lint` fails on every PR, including PRs that never touch the register. `statusgen init` never overwrites an existing file, so **an adopter already scaffolded must make the same edit by hand**: add `with: fetch-depth: 0` to the lint job's `actions/checkout` step in `.github/workflows/assay-statusgen.yml`, or on GitLab add `variables: GIT_DEPTH: "0"` and the `git fetch` line to `statusgen-lint` in `.gitlab-ci.yml`.
- The `sops-block` secret-scan refusal now names its remedy: cite sops material by `path:line` and describe it in words, never quote a `sops` footer or an `ENC[…]` envelope. Every surface and every outbound write kind gets the same message, so a reviewer whose verdict tripped the rule has a sanctioned way to state the finding. The rule itself is unchanged: a quotation is still refused, inside a code fence too. This change adds no flag and no exemption; the existing audited `--force-scan-override` is a human act, and a reviewer never passes it to get a verdict through. The review prompt, the `pr-review-desk` verdict-format reference and `deskpost`'s usage say the same (#2060).
- The forge-surface ban script's Go matcher now counts wrapper calls (`runCmd`, `runCmdIn`, `execCommand`, `exec.CommandContext`) whose binary is `gh`, not only direct `exec.Command("gh", …)`.
- `--lint` fails closed when the exact `refs/remotes/origin/main` does not resolve to a merge-base in a git checkout with findings entries: it refuses register transitions with one PROBLEM that `--changed` never scopes away, instead of comparing against HEAD (where a committed transition compares with itself and passes) behind a degraded NOTICE. A tree with no `.git` directory still gets the degraded NOTICE, since the guard is skipped there rather than compared against HEAD. CI jobs that run `--lint` need the base branch fetched (`actions/checkout` with `fetch-depth: 0`, or `git fetch origin main`).
- `cellctl` resolves the operator home in one place: `USERPROFILE` first on Windows (then `HOME`), `HOME` alone elsewhere — the variable Go's `os.UserHomeDir` reads on each platform. It refuses with a named reason, never echoing the value, when that home is unset, relative, non-local or the filesystem root, instead of deriving a relative or root-anchored config, gh, Claude or Codex path. A home on a non-Windows host that carried only `USERPROFILE` used to resolve and now refuses. Explicit overrides (`ASSAY_CONFIG_HOME`, `GH_CONFIG_DIR`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME`) still resolve with no home, and `%APPDATA%\GitHub CLI` is the gh default on Windows. `cellctl new` resolves the home before it creates the cell directory, so a refusal leaves no half-built cell. A class guard fails any new home read outside the resolver, in any spelling: the cell env accessors, `os.Getenv`/`os.LookupEnv`, `os.UserHomeDir`, a direct map index, or a named constant key.
- `desktools-v2/13` Verify row 7 now matches the Windows `tools/desk` test step itself, so it fails until the staged workflow patch is applied (it exited 0 before, matching only a comment).
- `pr-review-desk`: a content-scan refusal on a verdict body is now an explicit STOP. The desk never rewords, re-encodes, splits, trims or resends the refused body. It files the refusal at discovery, records on the PR that a verdict is withheld, and keeps dispatching the queue. The scan override is the maintainer's alone and exists only for the rules the tool lets it waive; on `voice.ruling-claim` and `withheld.identifier`, which no flag waives, the filing asks the maintainer for a ruling instead. Only the reviewer may re-issue its own verdict, with `path:line` citations in place of quotations. The reviewer kit (`deskdispatch --kit review` §12) carries the reviewer's half. A new scenario test pins the skill section's wording and checks a table of reword-after-refusal transcripts against the rule it states.
- `statusgen --lint` starts about 70% fewer git processes (276 to 79 on this repository) with identical lint output: blob reads share one `git cat-file --batch` per run, repeated merge-base questions are memoised, and the per-flag `git log -S` age check is one `git log -p`.
- `statusgen/bench/lintbench.sh` generates a 400+ brief fixture and times an offline `--lint` for two binaries, best of three.
- deskmerge reads a pull request's state and its sign-off comment through the in-process forge client, using a token minted for the session's App role and for the repository being read. It no longer shells `gh` under the operator's ambient identity. An unresolvable role, a failed mint or an empty token is a could-not-check, and no request is sent.
- deskmerge's exit codes on its sign-off and pull-request reads: a sign-off comment that is no longer on its item exits 5 (refused), not 6; a pull request whose head repository was deleted exits 6 (could-not-check) instead of being refused as a fork; a `/pull/N` link whose number is an issue exits 6 with "could not resolve".

## v1.0.31 — 2026-10-02

### Added
- Advice can only restrict.
- Agentic admission draft (`spec/agentic-admission-v1.md`, `schemas/agentic-assessment-v1.json`, `deskkit` `EvaluateAgenticAdmission`). It is a pure, deterministic policy that maps hard facts and recorded advice to one of five dispositions: bounded-agent-work, supervised-agent, human-led, discovery-only or blocked.
- Malformed input fails closed: empty bindings, invalid or oversized subjects and inputs, and out-of-vocabulary values. Reason codes never echo an assessed string, and a refused subject is never echoed into the result or its `PolicyResult` projection.
- Mandatory graph gates are the union of the brief's risk verdict and the disposition's.
- Nothing activates it yet; dispatch wiring is a separate gated change.
- Unknown readiness holds implementation.
- discovery-only stands only on a discovery grant whose read scope covers the subject; otherwise the result is blocked.

### Fixed
- Forge-ref claim writes fail closed without a complete matching receive-pack acknowledgment; HTTP refusals retain their original status marker. Optional trace receipts retain bounded request and pack metadata without claim payloads or credentials.
- The build-less-brittle/04 and /06 Verify tables now reach a verdict after the briefs land. Each `consumers:` row runs in a throwaway clone pinned to the delivering squash, and Expect names exit 0 and the exact summary line. Row 9 of /06 prints one gated count. Row 7 braces its revision variables, so zsh can no longer pass it at 0 ≤ 0. Row 3 keeps its scratch files in the checkout. What each row proves is unchanged.
- The outbound check no longer refuses a brief's own frontmatter `id:` as a session id (#2022). The exemption covers exactly one shape: a lowercase UUID that is the whole value of the single `id:` line inside the frontmatter of a `docs/streams/**/brief-*.md` file. The check reads the file's full content to establish that shape, on the push path, the forge file write and the evidence pre-flight. A UUID in brief prose, in any other file, in a commit message or in a PR body is still refused.
- The publish-identity gate in `deskpr update` now judges only the commits the push adds, so a PR
  whose head already carries a commit by another trusted App can be updated. The narrowing fails
  closed: a missing, stale, diverged or oddly-spelled remote tip falls back to the whole
  `origin/<base>..HEAD` range, and `update` re-checks against the forge's live PR head before it
  pushes.
- `deskpost` no longer carries the retired per-item `+1` reaction gate's read path: the orphaned reaction-read method on its forge-backed adapter is gone, the adapter's interface comment now names only the live-visibility read the `:public` write gate consumes, and a structural guard test fails if any reaction/award read reappears anywhere in `deskpost`.

### Changed
- Define Go launch and session contracts for the native Windows cellctl work, with strict record parsing, independently bound launch permissions and compiled process fixtures. Production Windows integration remains separate work.
- `deskboard actions` no longer skips review of a PR authored by a trusted or blessed human: the `HUMAN-OWNED` action is retired, so such a PR is `NEEDS-REVIEW` (or `RE-REVIEW` on an advanced head) like any other admitted PR and counts toward the UNREVIEWED neglect alarm. The trust gate (trusted roster login, else a current blessing) is now the only authorship filter: an unblessed author stays quarantined in EXTERNAL / UNBLESSED, and an unreadable trust read or roster still keeps the PR out of dispatch. `reviewloop` drops the matching table key, so an older board that still emits `HUMAN-OWNED` reads as an unknown action and fails closed (#2028).
- ci: promote the staged Windows CI leg and the winparity PowerShell 5.1 parse check into live workflows.

## v1.0.30 — 2026-10-01

### Added
- A new `forgeban` rule stops any package outside `internal/deskkit` from naming a backend type, so no code can build an unchecked backend or unwrap the checked one.
- Document the native Windows implementation plan for `cellctl`, with dependency-ordered briefs covering Orca and Herdr launchers, Docker lifecycle, credential custody, and Windows acceptance checks. Runtime support remains planned.
- Draft continuing-operations specification and parked stream proposal, with fourteen delivery outlines covering user journeys, portable measures, expected impact, outcome review, adopter packaging and cockpit integration. These are review materials, not released runtime capabilities.
- Every new brief now carries a `design-fit:` block in its Context (`owner`, `contract`, `retires`, `weight`, `why-add`), so the author answers which module owns the change, what it retires and how much weight it adds before a worker starts. Defined in `spec/brief-v1.md` §4.1, with an example in `docs/brief-template.md` and the author-brief skill template.
- Every write verb takes the same audited `--force-scan-override`. Credential, personal-data and self-containment refusals can be overridden, and each override leaves an audit row that holds the rule id and a digest of the text, never the text itself. A withheld identifier on a public or unknown target, and a ruling claim, can never be overridden.
- Hotspot report for `tools/desk`: a test-only package (`internal/hotspot`) that ranks files by churn × indentation complexity over first-parent git history and reports temporal-coupling pairs (`go test ./internal/hotspot/ -run TestPrintHotspots -v -args -since=… -until=… -top=N`). A shallow clone reports could-not-check, never an empty ranking. No verb or flag is added to any shipped binary.
- Known limit of the critical tier's inputs: a brief's `issues:` list (the main-red fix linkage), a README stamp cell, and a findings entry's `control:` are repo text that an ordinary reviewed PR can write. Only the red itself (`--main-health`) and the stamp authority set (roster configuration) come from outside the tree, so tier membership is as trustworthy as the review and merge of those files.
- Negative-path tests pin the custody contract of the desk's native GitHub read client: a read with no minted token is refused before any request, even with a `GH_TOKEN`, `GH_ENTERPRISE_TOKEN` or gh login present, and the installation a read's token is minted for is the account of the repo being read, never one `GH_REPO` or `GH_TOKEN` names. A committed mutation spec proves each check is load-bearing.
- New `ReviewQueueSnapshot` Forge operation. It returns every open change together with its reviews in one backend round-trip: a single GraphQL query on GitHub. On GitLab it is degraded: it marks each change incomplete, so reviews are still read per item.
- One outbound-write check now runs on every Forge write the desk tools make, and on every push made through `deskpr` or from a checkout with the `deskpushguard` pre-push hook installed, before the write leaves the machine. It combines the credential scan, the ruling-claim guard, a new personal-data pass (e-mail addresses and international phone numbers) and, for public or unlisted targets, the self-containment categories and the withheld register. The target's configured visibility decides which layers run.
- Platform path, custody ordering, private fixture directory, hook argument forwarding shared forge contracts, backend read boundaries, GitLab outbound note and rewrite cases, and two-forge ambient credential checks. The compatibility brief remains incomplete pending owner scope clarification for environment resolution and Windows outbound-machine-path recognition.
- Review gains a **design-fit stage** that runs before the correctness pass on any PR that grows a ratcheted weight dimension, adds an `R-` rule row, touches a brittle-marked module, or reddens an `internal/arch` test. Each trigger applies only where the repository carries the instrument it reads; a missing instrument is recorded as could-not-check, never as a design-fit finding. It asks three questions: is the change in the right layer, could removal fix the symptom instead, and are `retires:`/`why-add:` true. A "no" answer is a finding with the new scope basis `design-fit` (`deskkit.BasisDesignFit`, pinned to the review kit's `reviewscope` block). The class starts **advisory**: the finding is recorded and does not hold the PR. Promoting it to `blocking` later is a one-cell edit in the pr-review-desk finding-class register.
- The `observe` evidence kind (`spec/workflow-pattern-v1.md`, `schemas/workflow-pattern-v1.json`):
  a signal watched over a window after a change lands, declared only where a deploy
  exists.
- The check sits in two places. The Forge that `ResolveForge` returns wraps every write that carries text, so a verb cannot hold an unchecked Forge. The push path (`deskpr` before it pushes, and the `deskpushguard` pre-push hook) checks the branch name, every commit message and the added lines of the diff. A file written through the Forge checks the branch it lands on the same way. A comment, review or edit on a numbered item carries that number into the check, so a bare `#N` above it is still named as a probable cross-repo reference.
- The worker-desk skill states the drive worker floor: for drive work, the floored `deskboard dispatch` reading wins over the wider `fanoutloop plan` board queue.
- Verify rows can name their command explicitly: in a Command cell that mixes prose and code spans, a code span starting `cmd:` (for example `` `cmd: go test ./pkg/ -count=1` ``) is the command. `statusgen verifyrun`, the check:ci re-execution lane, `newbrief`, the row lint, `verifyloop` and `deskrebaseline` all take the first `cmd:` span. A cell whose first span is already its command needs no marker. A cell without the marker lifts exactly what each tool lifted before, so no existing row changes command. A marker counts only where the rendered brief shows it as code: one between escaped backticks, or in a cell whose prose carries raw HTML, a comment, a link, an image, a dollar in any spelling (`$`, `\$`, or a character reference such as `&#36;`, which GitHub can render as math) or any character reference, or in a cell where a span's opening backticks are fused to the text before them (an autolink or `~~` can swallow them), is ignored, and the marker span itself must be set apart by whitespace or the cell edge (plain punctuation may follow it), so hidden text cannot replace the command a reader sees.
- Work-input dependencies (graph-execution/03 work-input amendment): a manifest's
  policy, build and environment dependencies are fingerprinted at the witness's commit
  and the item's revision. A changed one holds only the claims that depend on it, as
  `wrong-revision` naming it; one that cannot be fingerprinted is `could-not-check`. A
  reused ancestor `pass` names its derivation and keeps the witness's own revision, so
  an old receipt is never retargeted to a new subject.
- `TestOracleShellcheckClean` in `tools/desk/cmd/cellctl` runs `shellcheck` over the whole oracle inside `go test`, so wherever `shellcheck` is installed a new finding at any site is a red test. CI does not install `shellcheck` yet, so there the test skips, and a non-verbose `go test` prints that skip as a plain `ok`; the install step is tracked in #1875. A planted-SC2086 positive control makes a broken or stubbed `shellcheck` fail the test rather than pass it; with no `shellcheck` on PATH the test skips as could-not-check.
- `cellctl` manages local Docker cells directly in Go through `CELL_CONTAINER_CONFIG`, including registration, preflight, status and shutdown. Native cells no longer need an external host launcher.
- `deskapps init --manifest <file>` — registers a single arbitrary GitHub App (e.g. a
  leak-sweep gate App) from a manifest JSON file instead of a tier's fixed App set, through
  the same loopback bind, state-nonce, callback/conversion/PEM-write and identity-mismatch
  machinery as `--tier`, keyed by the App's manifest name rather than a desk role. Mutually
  exclusive with `--tier`; refuses a manifest that specifies its own `redirect_url` or
  `hook_attributes.url`.
- `deskapps init` — the loopback web app that drives GitHub's App Manifest flow: it posts
  the chosen tier's manifest (`team`: read+act; `family`: one App per desk role) to GitHub's
  own new-App page, exchanges the returned code for the App's credentials, and writes the
  private key (0600, never printed or logged), `apps.env` and the role→App bindings. Binds
  `127.0.0.1` only, refuses a `/callback` whose state does not match a pending row, and refuses
  a conversion whose App owner is not the account the operator named (`gh` login or `--org`), failing closed when either owner is empty.
  The posted manifest carries no `hook_attributes` key (this flow sets no webhook URL, and
  GitHub's schema rejects a url-less `hook_attributes`).
- `deskboard dispatch` applies the same floor across every configured root, summing in-flight drive work, and names the floor in its held-back line. Its merged queue keeps the critical tier ahead of score, so a critical row takes the shared headroom before routine drive rows, and each row it offers is tagged `drive:<slug>` / `critical:<arm>`. The cross-root floor needs statusgen at this version on every root: a root on an older statusgen reports no cap, and with no root reporting one no cross-root floor applies.
- `deskboard`'s actions sweep now reads each repo's open PRs and their reviews with this operation, instead of one review read per PR. On GitHub that means 1 call per repo instead of 1 + N. Each PR's head and its reviews now come from one consistent snapshot, and the board output is unchanged.
- `docs/contracts.md` gains a **Rule register**: one row per enforced rule (owner, invariant, the semantic-index row it serves, justifying public issue, and the test or telemetry that shows it can fire), seeded with 17 rows: the rules from the recent fix-caused-next-bug chains, plus the weight ceiling. A new rule now adds its row in the same PR.
- `docs/contracts.md` gains a **monthly Rule diet**: a three-state catch status (`could-not-check` / `proven-able-to-fire` / `zero-without-proof`), candidate criteria, one immutable decision issue per month answered by `retire R-a R-b; keep rest` or `keep` from the driver's own login, keep by default, and retirement only through a design brief (`gate: human` plus a proven remaining layer for trust-boundary rules).
- `docs/contracts.md` gains a `## Brittle marks` section: the two-key rule (the metric nominates, the class defect history confirms), the `watch` state, the mark table and the clearing rule. No module is marked yet.
- `statusgen --auto-flip-model` refuses the `verified` → `done` flip when a brief's
  evidence coverage is not released — checked offline, before any live review fetch.
- `statusgen --coverage [--json]` — the evidence coverage rule (graph-execution/03):
  for every brief, the union of its own Verify rows and (when it is bound to a
  workflow-pattern-v1 node) that node's mandatory evidence must each resolve `pass`
  at the item's revision before the brief is `released`; a missing, errored,
  could-not-check, wrong-revision, or failing claim holds it, with the reason.
  A witness counts at the item's revision when it names that tree, or an ancestor
  of it with no change since to the paths it speaks for. Without a complete work-input
  dependency manifest that is everything outside `docs/streams/**` and `STATUS.md`
  (a `files:` declaration alone never narrows it); a complete manifest may narrow it
  to the brief's `files:` (now and at the witness's commit) plus the claim's source
  dependencies, unless a declared entry names no real path. It never covers the files
  verify and regeneration write, and renames count against their old path. A witness
  whose Verify row cannot be read at its own revision is `could-not-check`.
- `statusgen --lint` NOTICEs a prose Command cell whose first code span is a mention rather than a command, such as a function name, a file, an `owner/repo`, or a word ahead of the real command (`prose-led-command`). It also NOTICEs a cell that carries two `cmd:` markers (`cmd-marker-ambiguous`). It NOTICEs a marker that replaces a command-shaped first span (`cmd-marker-overrides-command`) and a `cmd:` span that is not honoured (`cmd-marker-not-honoured`). It NOTICEs a marked command that cannot fail, such as `true`, `:`, `exit 0` or a lone `echo` (`cmd-marker-vacuous`). All five are advisory, so main stays green. A closed brief's `prose-led-command` rows collapse into one summary NOTICE.
- `statusgen lint --check citation-alias-resolution` resolves every aliased ref (`<alias>:<stream>/<NN>`, `<alias>#<NNN>`, `<cell>:<alias>:<stream>/<NN>`) in a brief's `sources:`, `consumers:` and `## Evidence` through `docs/streams/graph-repos.yaml`. An alias the registry does not define is a PROBLEM naming it. `statusgen lint` runs named checks only; an unknown check name exits 2 and never passes.
- `statusgen verify-gate-close --ref <ref> [--dry-run]` does the human done-close by a ref in any brief form. It resolves the alias through the registry first: exit 5 when the alias is unknown or belongs to another repo, 6 when the tree has no registry.
- `statusgen`'s board-honesty detector gains a seventh phantom class, `sibling-merge-unreconciled`: a `todo`/`in-progress` brief whose deliverable lands in a sibling repo (`homed-in:`, `deliverable_repo:`, or a `../<basename>/` path prefix, resolved through `docs/streams/graph-repos.yaml`) can now be checked against that sibling's own first-parent git history — reading both the commit subject and body, so an id named only in a commit's body or trailer is still caught. Matching also honors a `tracked-in: <alias>#<N>` frontmatter entry, matched against an `Issue: #<N>` trailer or GitHub closing keyword in the sibling's history — for a sibling PR that deliberately withholds the brief id itself; without a declared `tracked-in:` entry such a PR is a checked-clean miss. A match is a NOTICE (never a PROBLEM) naming the sibling, the short sha, the subject and which key matched — a prompt to read the merged change Task by Task, never proof of delivery. A checked-failed `todo` row is held out of Next-up (`MergedElsewhere`, the `homed-in`/`HomedElsewhere` shape) and listed in STATUS.md's "Merged in a sibling repo — check before dispatch" section; an `in-progress` row is surfaced but never excluded. A structured `delivery:` claim in the brief's own frontmatter acknowledges a specific merged PR — `covers: partial` releases the hold, `covers: full` with the cell still unlanded keeps it with a quieter NOTICE. New `statusgen phantoms --class sibling-merge-unreconciled --root R` verb: exits 0/1/2 without touching the regen/`--lint` path, for a CI row or desk sweep that wants a red signal narrower than the whole board-honesty family. The sibling read is opt-in on `--lint`, regen, `--next-up` and `--roadmap`: pass `--sibling-merge` or set `ASSAY_SIBLING_MERGE=1` (exactly `1`); without it `--lint` prints at most one `not-checked:` NOTICE and reads no other checkout. The `phantoms` verb needs no opt-in. Only siblings named in the operator's map — `--sibling-root <owner>/<repo>=<path>` (repeatable) or the `DESK_ROOTS` environment variable already used by the desk tools — are ever read; the tree's registry can narrow that set but never widen it, and a registry `repo:` value that is not a strict `<owner>/<name>` is skipped with a NOTICE naming only its entry index. NOTICE text is rendered without control characters.
- `tools/desk/internal/arch`: architectural fitness functions that run as an ordinary Go test. No package under `internal/` may import a command, and no command may import another command (`R-dep-direction`). The hub package `internal/deskkit` may import only the packages listed in `hub-allow.txt` (`R-hub-allowlist`). Every function marked `// semantic: S-<slug>` must sit in its row's owner or listed duplicates, and each row's count may not exceed its ceiling in `markers.txt` (`R-one-implementation`). In a copy of `tools/desk` without `docs/contracts.md`, the third rule reports could-not-check instead of passing.
- statusgen `--next-up`: while a drive is active, the drive worker-pool floor (`driveWorkerCap`, 6) binds the dispatch queue. It offers at most the cap minus the drive work already in flight (claimed items the drive covers). It withholds the rest and counts them in `heldByDriveWorkerCap`, and it offers no drive pick when claims could not be read (`driveWorkerUnknown`). With no active drive the payload is unchanged.
- statusgen drives critical tier, main-red arm: a new `--main-health` input (`green`, or `red:<owner/repo#N>[,...]` naming the issues that track the red main) lifts a main-red fix into the tier. A main-red fix is an issue placeholder (`placeholder-v1`) for a tracking issue, or a brief whose `issues:` lists one. statusgen still reads no live CI. When no input is given and a drive is active, the board and `--next-up` (`mainHealth`) report could-not-check instead of a silent green.
- statusgen drives critical tier, stamped-security arm: the ratified authority set is read from the new roster key `ASSAY_CRITICAL_STAMP_AUTHORITIES` instead of a compiled-in placeholder. When the key is unset, the arm grants nothing, the effective-config echo shows it as unset, and `--lint` names every stamp it cannot honour. The desk tools recognise the key. **Do not add the key to a shared roster until every reader of that roster (statusgen and the desk tools) is pinned to a release that carries it:** an older reader refuses the whole roster configuration on the unknown `ASSAY_` key.

### Fixed
- A boot fetch that fails outright (an expired credential, say) now refuses with exit 3, names the likely cause and the way to recover, and releases the fetch lock instead of leaving it for the next boot to wait out.
- A missing container console can be recreated and attached to the same verified running container. Repeated matching model arguments reconnect, failed Docker inspection remains an error, and startup failures retain their console diagnostics.
- A new module-root test, `tools/desk/peakassert_test.go`, is a lexical tripwire: it fails on any `_test.go` file in the `tools/desk` module that asserts a sampled concurrency peak (`if max… < 2`). The two remaining sites are allow-listed and tracked for conversion.
- A witness row for a command that contains backticks now fences the command with a longer backtick run. Before, `--check` read back a truncated command and reported the row as changed.
- Activation picks between duplicate component ids deterministically (first root-relative path wins; the rest are reported as skipped) instead of depending on walk and sort order.
- Capture mergecheck archive and extractor diagnostics independently, wait for both processes before reading them, and close the pipeline when archive startup fails.
- Directory-mount containment compares file identity when a plan is built and before launch, so another spelling of a protected directory (symlink, case or normalization variant, filesystem alias) is refused. The operator's home directory and the cell's own directory are protected too, and a container with device requests (such as GPUs) is refused on reconnect.
- Manifest discovery (desk-verb activation and `deskmanifest lint`) no longer reads manifests from a nested clone, linked worktree or submodule under the checkout, so a stray nested copy can no longer change the parent tree's activation or surface as duplicate component ids.
- Native containers are checked for more runtime settings (entrypoint, extra environment, groups, ports, user/IPC namespaces), the cell network must carry the cell label, and directory mounts are validated against protected paths and the Docker socket; `/` is refused. Model pins with control characters are refused, and the console runner is started without a shell.
- New class guard `TestNoPersonDatedRulingCite` walks the published `tools/desk` tree and fails on any "<Name>'s <YYYY-MM-DD>" ruling citation. A positive control (`TestRulingCiteMatcherIsLive`) and a file-count floor make an empty result mean "checked clean", never "looked at nothing".
- Re-review dispatch allocates fresh detached worktrees while preserving previous evidence and the original claim and attestation checks.
- Review dispatch and stamp liveness share configured-alias and repository-basename claim families. Unknown prefixes and mismatched PR numbers refuse before claim acquisition; existing keys stay unchanged.
- Review-floor diagnostics name searched claim families. Restamp output now distinguishes label provenance from live review authority.
- Tests in `tools/desk/internal/deskkit` that gate on a fixture through `skipIfFixtureAbsent` can no longer skip unnoticed on a full checkout. When the tree carries `.github/workflows/`, a missing fixture fails the test unless the test is on a committed register of known-absent fixtures, listed with the exact path. The full-checkout probe fails closed, and a control drives the helper itself end to end, so disarming the helper's wiring or its probe turns the suite red. Tests with their own bespoke skip logic are outside this guard.
- The Codex smoke protocol's Step 3 skill list now names all fourteen bundled skills (it had missed `system-demo`), and its run-log skeleton records the bundle version from `SOURCES.yaml`. A new `tools/harnessgen` test keeps Step 3's list equal to `plugins/assay/skills/`, so a skill added or removed without updating the protocol fails CI. The protocol's run-status section now lists the 2026-09-12 and 2026-09-13 runs and says what still needs a live run.
- The `cellctl` model-policy test suite passes again. Its offline stand-in for `git fetch` now writes a `FETCH_HEAD` the way a real fetch does, so the launcher's refusal to boot on a fetch that wrote none (#1853) no longer fails the launch test. A new check in the suite fails if any test's git stub exits from its fetch branch without writing `FETCH_HEAD`.
- The `commsloop` tests no longer leave an untracked `mailbox/` directory in the source tree: the refusal test now roots its loop under a temp dir, and the package test run fails if any test writes a new path into the package source directory.
- The `internal/deskkit` mutation gate is green again. #1919 changed the machine-shape arm of the public-repo self-containment scan to use `FindStringIndex`, and the mutation that disarms that arm still targeted the old `FindString` line. The edit stopped applying, so the gate reported `could-not-mutate`. The mutation now targets the current line and disarms the same arm, and the suite catches it.
- The cellctl bash oracle (`tools/cellctl/testdata/cellctl-shell-oracle.sh`) is `shellcheck`-clean again (#1355). Its seven `value_in "$x" $<LIST>_VALUES` calls word-split the value list on purpose, and the `policy_preflight` subshell keeps its policy variables local on purpose; each site now carries a `# shellcheck disable=` directive naming why, instead of an SC2086/SC2030/SC2031 finding. No behaviour change in the oracle or the Go program.
- The check:ci verdict re-execution lane now runs the command lifted from the Verify cell, not the raw cell text. Before, the shell read the cell's backticks as command substitution.
- The desk-tools release-stamp guard (`TestVersionStampedFromReleaseWorkflow`) now reads `.github/workflows/release.yml`, where the desk-tools build step actually stamps `deskkit.ReleaseTag`. It used to read a `release-desk.yml` this repository never had, so it skipped on every run and could never fail. It now checks the build step itself: the stamp must sit inside the `LDFLAGS` assignment, fed the resolved release tag, and `LDFLAGS` must reach `go build` without being reassigned first. A stamp that survives only in a comment or an `echo`, or is fed the commit instead of the release tag, fails it, and a mutation control proves it goes red on each way the stamp can break.
- The desktools-v2 forge-ban baseline is refreshed from a stale 53 to 63, the count measured on current main after the one regression was fixed, and a site-by-site ledger in the seam contract explains the change. No Verify row reads that baseline any more: every row that counts reach-around sites compares its brief's delivering commit against that commit's own merge parent and prints its own verdict, so sites other PRs add later cannot move it. The forge-gitlab host-literal row counts code lines only, so comments naming the host no longer hold it above zero.
- The desktools-v2/01 reach-around inventory is re-derived against the current tree: it now lists all 29 statusgen `gh` call sites in 16 files (new: the `gh auth token` fallback in `--corroborate`'s ruling client and two `--auto-flip-model` PR-shape reads), the sixth forgeban permit (the preflight ambient-login probe, ceiling 5 to 6), the Go cell launcher's generated `gh` shim and wrapper, the monitor script's per-owner token read, the preflight probe's second `gh` launch (`gh auth token`), and two hand-built GitHub REST calls outside the backends (`deskinbox`'s comment reader and `deskfleet`'s label create). The shape (c) sweep command now matches a `/repos/` path anywhere in a literal, and every line it prints is a row or is accounted for. Every file:line citation is re-anchored, resolved rows are marked, and the commands that derive each group are written into the document so the next drift is a diff.
- The push-transport gate in `deskpr create`/`update` and `deskwt add` now decides from the URL git will really push to (`git remote get-url --push --all`), with `insteadOf` and `pushInsteadOf` rewrites applied (#884). An https remote that a rule rewrites to SSH is refused, and the refusal names the rule and the remedy that fits it — both steps when removing the rule alone would not clear it, and a `set-url` line that still runs on a multi-valued pushurl. A remote with no url, which git resolves to its bare name, stays could-not-check (exit 6).
- The reactive-activation refusal names the component by its declared id (`could-not-check: assay/desk-tools inactive — …`) instead of doubling the namespace (`assay/assay/desk-tools`).
- The staged Windows CI leg's fail-closed step for `statusgen verifyrun --in-container` can fail again. Since the real harness digest was pinned, the step no longer exercised the pin control and treated any non-zero exit as a refusal. It now plants a placeholder digest in a scratch copy of the manifest, requires the launcher's own refusal (exit 2 plus its `refusing to run` line), and checks that the committed pin still gets past the control. A new statusgen test flags any workflow that runs the in-container launcher against the committed tree.
- Validate cell roots and launchers with host path semantics while refusing all UNC and device spellings, and warn specifically on a GitLab CE approvals endpoint gap while retaining authentication, permission and server failures.
- When the longest known text matching at the anchor is not also the newest one
  matching there — a newer text of the block is a strict prefix of an older
  one, so the copy could genuinely still be at the older, longer text, or it
  could be at the newer, shorter text followed by unrelated content, possibly a
  local, site-specific rule, that happens to equal the longer text's own tail;
  the bytes alone cannot tell — `--sync` now **refuses by default**:
  could-not-check, naming the file, the two lengths that matched and the exact
  line-range span the longest-match rule would have removed; that block is not
  written. Once such a prefix-shrink is committed, every later sync of that
  block refuses the same way while the copy still matches both texts. A block
  that only grew is not ambiguous (the longest match is also the newest), so
  its later edits rewrite normally.
  An earlier version of this fix took the longest match anyway and only
  printed a `note:`, and claimed `git add` right after each sync closed the
  window; neither held up (the tie comes from committed history, not from
  anything staging affects, and the same note fired on every ordinary edit
  too). The longest-match rewrite is still reachable, deliberately, via the
  new `--allow-ambiguous-extent` flag (which applies to every ambiguous block
  in the run and is refused without `--sync`), and is still recorded as a
  `note:` when taken that way. This default (refuse, with an explicit opt-in) is this
  project's own choice among the review's options, not a settled
  cross-project ruling. A shrink of an uncommitted, unstaged edit that is
  itself trimmed again before ever being committed or staged is still
  could-not-check only when nothing at the anchor matches (README §4).
- `--sync` used to prove a copy's removal extent against one read of the site
  file, then splice that proof into a second, later read taken only at write
  time, with no check that the two reads still agreed, and wrote in place
  (truncate-then-write). Concurrent `--sync` runs could corrupt a file this
  way. Each site file is now read at most once per run, that same read backs
  both the proof and the write, the file is re-read and compared against it
  immediately before writing (refusing rather than guessing if it changed),
  and the write itself goes through a temp file plus atomic rename.
- `ASSAY_RUN_CREDENTIALS` is now a roster **trust** key, not an extension key: it chooses which
  credential a desk write runs as, so a malformed entry now refuses the whole roster (every desk
  tool refuses, naming the key) instead of disabling only `deskrun`. Unset and valid values
  behave as before. `ResolveRunCredential` also refuses on its own when the roster is refused,
  whether or not the caller checked first, and a class guard pins the extension catalogue to a
  committed allow-list, so no trust key can land in it again.
- `cellctl desk` no longer boots on a previous boot's `main` when the checkout's `FETCH_HEAD` is read-only. git could not rewrite the file, the fetch failed, and the old sha was read. `FETCH_HEAD` is now removed before the fetch, and a boot whose `FETCH_HEAD` cannot be removed is refused (exit 3, fetch lock released).
- `cellctl`'s built-in Opus 5.0 model ban now matches by version rather than by fixed spelling, so a 5.0 model ID carrying a trailing date, provider tail or variant name (`claude-opus-5-20260101`, `claude-opus-5@20260101`) is refused like `claude-opus-5`; the-desk's Opus 5.5 floor no longer reads such a date as a minor version. `claude-opus-5-5` and the other 5.x IDs stay allowed.
- `deskavatar`'s golden-strip test (`TestGolden20px`) now compares decoded pixels exactly instead of PNG file bytes, so it no longer fails on a machine whose Go release encodes the same image to a different deflate stream (seen as a macOS-only failure; the cause was Go 1.27's `compress/flate`, not the platform). A new structural guard fails any desk-tools test that byte-compares a committed compressed file.
- `deskfleet` now takes its default GitHub API base from the shared `deskkit.GitHubAPIBase` constant instead of restating the host string, so the command no longer adds a site to the forge reach-around count. A package test fails if the host string comes back as code in `deskfleet`'s non-test sources (#1864).
- `deskpushguard` now judges a push against the remote git is actually pushing to (the hook's first argument), not an assumed `origin` (#1201). This covers the foreign-commit base, the register-id base, its sibling candidates and its liveness probe, and the URL fallback. With no remote name, or no `main` on the pushed remote, it reports `COULD-NOT-CHECK` and never falls back to `origin`.
- `deskreply` and `deskpr edit` no longer refuse a worktree that sits exactly on the pull request's head commit under a different branch name, or on a detached HEAD. A checkout now counts as the PR's own when its branch IS the PR's head branch or its HEAD commit IS the PR's head commit (exact match only; unpushed commits on top are still refused with a push-first remedy). The PR must still be open. `deskpr edit` gains `--pr N` to name the PR when the branch name cannot, and its detached-HEAD refusal without `--pr` now points at that flag (#1901).
- `docs/deploy-model.md`'s `may-deploy` field description narrowed to human-only, matching what the shipped `authority:` enforcement actually checks — the prose previously described a broader field (allowing a named role or automation identity) than the code accepts.
- `skillslint --sync` (`make guardrail-sync`) now rewrites a guardrail copy only
  when the lines at its anchor match a known text of that block byte-for-byte:
  either the current canonical text (already synced, so no write) or its text in
  an earlier committed or staged revision of `.claude/guardrails/GUARDRAILS.md`.
  It removes exactly the matched lines. A copy it cannot match, such as a
  hand-edited one or one in a tree with no git history, is reported as
  could-not-check and left alone. Before this change a grown block swallowed
  trailing content that was never part of it (the #1687 incident), a shrunk block
  left stale lines behind, and a second sync or a sync after committing the source
  edit did the same (#1690).
- `status` and `down` act on the running container's own harness and model, so a per-launch override is reported and stopped, and one role's refusal no longer skips the others. `check` reports every role's file and Docker check failures together.
- `statusgen --auto-flip-model` now credits the PR that delivered a brief, found by its `Brief:` trailer. Before, it credited the newest PR that touched the brief file. Two cases followed from that. A verify Evidence PR (docs/streams-only, authored by the roster's `verifier=` App, merged on a human approval) refused every brief whose rows landed by PR. A trailer-less stream-authoring PR left a brief at could-not-check. Every credited PR, and every later non-Evidence change to the brief file, must still carry the reviewer App's approval at its merged head, wherever git log lists it. A PR found by its trailer is credited only when it merged into the default branch and its body was last edited at or before its merge; a later or unreadable edit is could-not-check naming the PR. A file renamed into docs/streams/ from outside keeps a verifier-authored PR from counting as an Evidence landing (#1838).
- `statusgen --close-verify` no longer drops the repo alias of a `<cell>:<repo>:<stream>:<NN>` id. Before this fix, an id naming another repo flipped this tree's same-numbered brief to `done`. Aliased ids now resolve through the registry and are refused unless they name this tree's own alias.
- `statusgen verifyrun` no longer executes a Verify row whose first code span is a mention (a row `--lint` flags `prose-led-command`). It records the row could-not-run with the rule named. Before, it ran the mention, and a word such as `gh` exits 0, so the row recorded a pass for a check that never ran. Marking the command with `cmd:` clears it. No witness on main records a pass for such a row. The check:ci verdict re-execution lane refuses such a row under clause-6 without running it, and `verifyrun --check` reports an older pass witness on it as could-not-run.
- `statusgen` no longer drops every row after an unterminated `<!--` in a brief's `## Evidence` section. The witness, attribution and unrun readers strip complete comments only, so a later witness table is still read. An unterminated opener in a `## Verify` or `## Evidence` section is now a lint PROBLEM. `statusgen brief --check-verified`, `statusgen verifyclosure` and `statusgen verifyrun --check` do not run the lint, so each refuses a closure or audit over such a section through one shared check. The "section has content" check still treats an unterminated opener as consuming the rest of the section.
- `statusgen`'s HELD/could-not-check Evidence scan no longer refuses a
  genuinely clean PASS whose prose merely reports the ABSENCE or COUNT of a
  held state ("VERIFY: PASS ... no could-not-check", "summary: 0 HELD") — only
  an actual, un-negated disposition still contradicts the PASS. This NARROWS a
  flip-refusal detector (the model autoflip, the verify-gate card and
  closeVerify's `verified` path), so the excusal fails closed and what may
  precede a cue is an allowlist. A "no"/"not"/"zero" cue excuses only after
  the line start, a list marker, a count label ("summary:"), a clause break
  (",", ";", ".", "(", an em or en dash, "→") or a linking word ("is", "with",
  "and", …) — so not right after "?", "=", "|", ")", "-", a non-count ":"
  label (bold or plain), struck text or any other word ("rc zero HELD"). A
  "0" cue excuses only in a count position: the line start, a list marker, a
  count label, or right after a verdict-count item ("7 PASS, 0 HELD"). An
  occurrence followed — past whitespace, emphasis, or "," ";" ":" "." "("
  ")" "→" or a dash — by a hold-reason word ("pending", "awaiting",
  "waiting", "until", "because", "blocked", "due", "for"), or by a colon
  past only whitespace or emphasis ("0 HELD: human read owed"), is never
  excused: "0 HELD. pending runner", "0 HELD → pending runner" and
  "0 HELD) pending runner" all refuse, and so does a reason word that opens
  the next sentence ("0 HELD. For the record, …"), which is the fail-closed
  side. The reason word's own boundary accepts a trailing underscore as well
  as a normal word boundary, so a markdown-emphasised reason ("0 HELD
  _pending_", "0 HELD __pending__") is still detected — underscore is a word
  character, so a bare `\b` would otherwise miss it. Residuals: a clause
  break or linking word admits whatever precedes it ("runner available — no
  HELD" is excused), a hold reason that uses none of the reason words ("0
  HELD — runner offline") is not detected, and each physical line is judged
  on its own.
- `tools/desk` loopengine `TestDrain` no longer fails its concurrency assertion under load. It proved the pool runs items concurrently by sampling the in-flight peak after a short sleep. Each drill item now waits on a bounded barrier until a second item is in flight and the engine's first fill pass has returned, so a correct pool always passes, an uncapped pool fails the `PoolSize` bound every run, and a pool that runs items one at a time fails with a named message after 3s rather than waiting on the barrier indefinitely (#1951). The barrier does not address the separate engine-stop deadline `TestDrain` can hit under heavy host load.
- `tools/desk` source no longer carries the origin's operating record: a dated, person-attributed ruling citation in the topology publication-boundary test is now cited generically, and the session-email fixtures and doc-comments use a neutral example handle that keeps the same address shape, so the email allowlist tests still go red when the classifier is broken.
- `verifyloop` and `deskrebaseline` now honour the `cmd:` marker. Before, they ran the whole prose cell.
- build-less-brittle/08 Verify row 8a (the findings loader rejects a malformed entry) no longer fails on its own success path. The row plants a bad findings file so `statusgen --lint` fails on purpose, but it piped that lint straight into `grep -c`, and the witness runs every row under `pipefail`, so the lint's status failed the row even though the count was right. The row now captures the lint's output, asserts that the lint returned `1`, and counts the parse-error line from the captured output (#1862).
- statusgen drives critical tier, reviewer-finding arm: the remediation that an unresolved finding names in `control:` now reaches the tier. Before, the arm keyed only on `affects:`, and those briefs are excluded from Next-up by design (StaleRef).
- statusgen drives: the critical tier's high-unblocks arm now counts only reciprocated `depends:` edges (the target must also list the dependent in `unblocks:`), so one-sided edges that pass `--lint` at NOTICE tier can no longer lift a brief into the tier that ranks above every score. The ordinary Next-up score still counts every declared edge.
- statusgen's dependency-edge reciprocity lint now sees brief-v2 files: a v2 brief's hierarchical id is reduced to its `<stream>/<NN>` form before it is compared against `depends:`/`unblocks:` refs, so v2 self-loops are PROBLEMs and one-sided v2 edges are NOTICEs, as on brief-v1. The same fix applies to the ordering-gate lint's edge graph and the follow-up back-reference check, and a new guard test keeps any other raw use of a brief id as a lookup key out.
- windows-port/00's Verify row 8 (no unix-only syscall use outside a platform-split file) no longer fails on build-constrained `_unix_test.go` files: its exclusion now covers `_unix_test.go` and `_windows_test.go` as well as `_unix.go` and `_windows.go`. Row 2 now also checks that every `_unix_test.go` file carries an explicit `//go:build` constraint, so the wider exclusion cannot hide an unconstrained file. Rows 2 and 8 now lead with their command, so `statusgen verifyrun` runs the real check (#1454).
- windows-port/03's Verify rows 2, 3, 5a, 6 and 8 are now exact commands with machine-checkable expectations, so `statusgen verifyrun` runs each one as written. Row 5a's fail-first mutation and row 6's success-line capture run through a `go test -overlay` and leave the working tree untouched (no `git stash`). Row 8 reads the latest `windows-ci-leg` run and checks that its bootstrap smoke passed on a head that carries the current bootstrap scripts; producing that run is still a maintainer dispatch (#1890).
- windows-port/08 Verify row 17, the live GitLab provisioning row, now opens with the human's literal command sequence (`deskfleet provision … --dry-run`, the real run, then `desktoken --forge gitlab <role>`). Its placeholders are left for the human to fill in, so the execution witness records the row could-not-run and never executes it, where before it ran a bare `--dry-run` fragment and recorded a false fail (#1795). The row is still the `gate:human` live row, and what it checks is unchanged.
- windows-port/10 Verify rows 3 and 8 are re-baselined so they still prove something on merged main. Row 3 now tests the refusal against a scratch placeholder pin. Row 8 is pinned to the implementing commit instead of a merge-base that resolves to HEAD itself.
- windows-port/11 Verify row 7 (no hard-coded `"/bin/bash"` under `tools/desk/cmd/scanloop/`) no longer fails on its own success path. The witness runs rows under `pipefail`, and `git grep` exits 1 when it matches nothing, so the old `git grep … | wc -l` row failed exactly when the property held. The row now tolerates only the no-match status, proves the directory exists at HEAD, and gates on `output is \`0\`` so a reintroduced literal fails the output check (#1699).
- windows-port/13 Verify row 7 now runs the `consumers:` gate against the commit that implemented the brief and passes only on a `CORROBORATED` line (one or more corroborated, 0 disproved). The old row ignored its own brief argument, ran on merged main where there is nothing to judge, and passed on a trailing `echo $?`, so it also passed with a DISPROVED claim.
- windows-port/13's no-shell-outs Verify row (row 3) now checks only that brief's own nine `deskinbox` shipping files, and fails on any `os/exec` import or `"make"`/`"jq"` literal in them. It had failed since windows-port/15 added `flow.go`'s sanctioned statusgen/deskboard exec to the same package; windows-port/15's own row 5 already scopes that exec (#1833).

### Changed
- Add opt-in Go-supervised host desk cadence with bounded passes, persistent checkpoints, role exclusion and honest status; support Cursor Agent alongside Codex in Herdr/Orca terminal launches.
- Add separate advisory GitLab reach-around columns and Verify-row portability notices; stage Windows platform checks for human application.
- An intake class instance block now carries the work input the next desk needs: `mechanism:`, `known-scope:`, `source-revisions:`, `unresolved-questions:` and `next-action:`, plus `source-origin:` and `trust-disposition:` for any reused reporter text. Older blocks stay readable (`scope`, `source-ref`, `open-questions` and `next-step` read as the renamed fields). A repeat report enriches the record without adding a count, a class issue or another dispatch. Text the trust gate held back stays source data and never becomes an instruction when copied downstream. A new `### Work-input triage example` in the intake skill walks mirrored reports, a quarantined quote, recovery and a genuinely new incident through one class. The skill ends no longer than it started.
- Bind packet and usage records to credential-reference and source-trust rules. Make intake/investigation field presence and oracle example placement checkable, and use per-run temporary output files in the new work-input Verify commands. These refine planned briefs; no runtime or delivery status changes.
- CI now runs the `harnessgen`, `harnesslint` and `plugindrift` test suites in the `build-test` job, and a new gating `harnesslint` job checks the shipped skill bodies and binding references for harness neutrality against the real plugin tree (harness-portability/15).
- Define matched cost/time and failure-recovery experiments; no operational gate or provider rollout is enabled.
- Design a six-dimensional brief assessment, independent acceptance review, and outcome-analysis stream. Proposed controls remain inactive pending policy rulings and implementation.
- Intake now files a machinery defect by error class. A symptom of an open `error-class` issue is attached to it as an instance block (`kind`, `incident-group`, `module:`, evidence and recovery fields, plus `open-questions` and `next-step`); a repeat report of the same incident appends evidence under its `incident-group` without adding a count or opening another class issue, while a separate occurrence takes a new `incident-group` and counts. A new class issue opens only when a second symptom shares the mechanism. At 3 counted instances, the class's 2nd merged fix, or the first instance in a `brittle`-marked module, the class is labelled `design-owed` and its symptom placeholders park as `blocked`; production-down and security symptoms never park, and the driver's `bleed` reply un-parks one. The class issue's own placeholder stays parked until the trigger, and a parked placeholder that the scanner reactivates to `todo` is parked again.
- Make intake, investigation and refactor-oracle handoffs preserve source scope and revalidate intervening changes without adding a runtime dependency to build-less-brittle.
- Refine existing class records and investigation handoffs with scoped evidence, explicit gaps and pending outcomes. Preserve the current recurrence rules, independent rollout and no-new-tool constraints.
- Route structured desk inputs, revision-aware evidence reuse, duplicate-event handling and durable budget accounting through the existing graph-execution briefs.
- Scope Codex command homes, startup isolation and roster capacity to its adapter; preserve native Windows home, executable and PATH semantics. Interactive role ownership now remains dirty across a launcher crash until reconciliation.
- The `deskpushguard` pre-push hook now runs the personal-data layer on every push from a checkout where it is installed, private targets and hand-typed pushes included. A commit that adds a real e-mail address, including in a trailer, now exits 5 unless `DESKPUSHGUARD_SCAN_OVERRIDE` is set.
- The review desk's recurrence rule routes a finding raised three or more times to its `error-class` issue instead of proposing another guardrail, and the worker desk dispatches a `design-owed` class issue's placeholder row at strong tier with the deliverable its body line names, never code: `Design-owed: deliverable a brief per author-brief, strong tier.`, or the brittle investigation for a first instance in a `brittle` module. That deliverable starts from the class record instead of rediscovering it. A class on a risk-bearing surface stays parked until the driver rules: a proceed ruling unparks it, and a decline returns its symptoms to `todo`. All three skills end no longer than they started.
- The review kit's clauses 3–15 are renumbered 4–16, and every cross-reference now uses the new numbers. The pr-review-desk skill now sends weight growth to a strong-tier correctness reviewer, and gains a growth-approval step: before approving, the reviewer checks that a `# grow` line cites a comment made by the driver's own login. The rule is registered as `R-design-fit-basis` in `docs/contracts.md`. Neither the kit nor the skill got longer.
- Verify Cursor skills and permissions in the actual role workspace before unattended passes.
- `deskfile`, `deskpost`, `deskreply` and `deskevidence` no longer run their own copies of the scan. Their writes pass through the shared check, which refuses at least everything the old per-verb calls refused.
- `deskmanifest lint` names each nested checkout it skipped on a `SKIPPED:` line, so a clean verdict states which directories it did not inspect. Exit codes are unchanged.
- `exec-tier` derivation gains question (d), "Is this a design brief raised by an error-class trigger?" — yes means `strong`. The author-brief dispatch checklist folds `design-fit:` into its layering item and stays at nine items; the skill ends shorter than it started.
- `plugins/assay/paired-versions.yaml` is re-pinned from v1.0.24 to v1.0.29. This covers every section: the statusgen and desk-tools per-platform sha256 pins come from the v1.0.29 release's `checksums.txt`, and the harness image digest is the registry digest of `desk-tools:v1.0.29`. **This moves the statusgen and desk-tools binaries adopters install from v1.0.24 to v1.0.29.** The harness comment no longer calls the digest a placeholder.
- `spec/lifecycle-v1.md` §2.4: `verified` now additionally requires coverage to be
  `released`.

## v1.0.29 — 2026-09-28

### Changed
- `statusgen --lint` no longer fails a backticked path that has since been moved or retired when it sits inside a brief's record sections — `## Evidence` (tables, verifier-run write-ups, witness rows) and `## Proof it can fail` mutation records. Those sections record what was true when they ran, so a later retirement no longer forces rewriting history or annotating every line. Task, Deliverables, Context, Verify-table cells and all other prose are still existence-checked, and dead markdown links inside record sections are still reported.

## v1.0.28 — 2026-09-28

### Added
- "Question k of n" now counts genuine items only, and nothing screened out is silently
  dropped. Every `--walk` question prints a tail line of counts. `--walk --screened` lists
  every screened item with its evidence: the ruling's date and author, or the parsed
  default and its gate. The table and `--html` renderings carry a `class` on every row and
  hide nothing. `--no-screen` turns the screen off in one flag and restores the pre-screen
  output.
- The `ask-decision` skill states the four classes and what the desk does with each before
  its five-part format, which now applies to every GENUINE item. The skill walks with the
  bash oracle, the only renderer that runs the screen. Under `deskinbox walk`, which does
  not classify yet, the skill's two floors ("never ask twice", "never ask what the desk can
  answer") stay manual checks.
- The desk secret scan refuses GitLab `glpat-` access tokens (#1642).
- The deskkit exit-code table (`ExitOK`/`ExitDisabled`/`ExitRateLimited`/`ExitRefused`/`ExitUnverifiable`) now carries its derivation of record — codes start at 3 to leave 1/2 to the shell's own error codes, with one code per refusal class — pinned by `TestExitCodeTableMatchesDerivation` and a mutation-spec entry that fails if a value drifts.
- The reviewer prompt kit gains a clause asking reviewers to name any desk-taken decision a
  pull request does not declare; the worker prompt kit gains the matching `--decided`
  guidance, and the `default-forward-reversibility` guardrail now names the declaration step
  (with `pr-shepherd` added as a site).
- The screen's roster comes from the environment, else from the owner-only
  `${ASSAY_CONFIG_HOME:-~/.config/assay}/roster.env`, and never from the current
  directory. Lists split and compare like the Go roster reader. With no roster, no class is
  entered and a NOTICE names the classes that were switched off. An item the screen cannot
  read is always genuine.
- The table and `--html` now make one `gh issue view` per item, because the screen must
  read an item to classify it; a failed detail fetch makes them exit `2`. `--no-screen`
  restores the old cost.
- The writer additionally refuses a `verify-wake-v1` receipt whose `inputs` omits a declared
  deliverable, whose `blocker_ref` is not a real issue/PR/run reference, or whose brief hash is
  not the brief as it lands — three recurring defects reviewers previously caught by hand.
- Three planned briefs in the desk-supervision stream close the Evidence-PR conflict churn for good. They cover one immutable file per verify outcome, with writer-side receipt validation (desk-supervision/24); carrying a correctness approval across a merge of main that leaves a draft PR's diff byte-identical, which is human-gated, depends on 24 and includes a human-landed guard on the Evidence auto-merge lane (desk-supervision/25); and one Evidence PR per verify tick (desk-supervision/26) (#882).
- Two new windows-port briefs plan Go-only GitLab fleet work, so Windows adopters no longer need bash. Brief 16 ports the GitLab PAT renewal into `deskfleet renew` and makes Go the single owner of the fleet's role, label and project-settings tables. Brief 17 first proves `deskfleet` against a real GitLab instance. It then retires `tools/create-fleet-gitlab.sh`, `tools/renew-fleet-gitlab-tokens.sh` and `tools/fleet-gitlab-roles.sh`, along with every doc that names them. Both briefs are human-gated.
- Worker kits and the `author-brief` skill ask for identifiers under 32 characters, and for long identifiers to be described rather than quoted in PR bodies (#1642).
- `ASSAY_STAMP_TRUSTED_LOGINS` roster key: an explicit, fail-closed stamp-authority allowance — the trusted human logins whose `dispatched-*` label applications the model-floor's actor check honours in addition to the bound dispatcher slugs (#336). Unset vouches for nobody beyond the dispatcher slugs; entries must already be trusted logins; an unconfigured roster vouches for nobody.
- `assay-inbox.sh --walk` classifies every item before asking, and puts only `genuine`
  items to the driver. The other classes are entered only on positive evidence from a
  trusted identity. `already-ruled`: after the newest ask (the last comment by a roster
  App, else the body's own Options), the ratifying identity (`ASSAY_BLESS_LOGIN`, and no
  other human) has posted an unedited ruling that names an offered option or ratifies. A
  question, a refusal, a "hold", or a bare desk relay is never a ruling. `no-fork`: the
  issue has a trusted author and its Options section parses to exactly one entry (with no
  Options section, a fork-test block with one `option:` line counts). `reversible-default`:
  the issue has a trusted author and carries a live `caught-by:`, a `default:`, no
  `class:` line and no one-way term.
- `build-less-brittle` stream: a scoping document and thirteen briefs that change the processes that decide what desk code gets written (intake, brief authoring, the worker and review) rather than the tools themselves. Symptoms file against `error-class` issues, and a class that recurs gets a design brief instead of another point fix. Every new brief carries a `design-fit:` block, a worker stops at the second fix in a class, and every PR reports measured `## Weight`. A design-fit review stage and a weight ratchet land advisory. A semantic-owner index and rule register go in `docs/contracts.md`, with a monthly rule diet that never deletes anything by itself. Later briefs add a hotspot-driven `brittle` mark with a bound investigation, architectural fitness functions, tagged regression tests with a `Retires-test:` trailer, a refactor oracle, and an incident-time refactor run that asks the driver at most one typed decision. Every typed reply counts only from the driver's own login. Design only; no tool behaviour changes in this PR.
- `deskdispatch --kit review`: clause 15 ("Scoped prompt-audit — on a PR that changes prompt
  text"), a scoped prompt-audit step the reviewer runs on any PR
  that changes a `**/SKILL.md` file, a `**/references/*.md` file (a skill's own, a bundle-level
  reference, or a dispatched kit itself), or any `CLAUDE.md` — posts High/Medium
  findings only, under `Prompt-audit (scoped):`, governed by clause 12's blocking boundary.
- `deskevidence --outcome-record` lands a verify outcome as its own new file under
  `docs/streams/verify-outcomes/<stream>/`, ending the class of Evidence PRs going
  `CONFLICTING` whenever a sibling landed (every Evidence PR used to append one line to a
  single shared `docs/streams/verify-outcomes.jsonl`, which the forge merges server-side with
  no `merge=union` driver).
- `deskfleet labels` — creates the nine fleet labels (`review-request`, the six `raised-by:<role>` stamps, and the `authorization-needed` / `approval-needed` pair) on a GitHub repository or a GitLab project from one shared table. A label that already exists is left as it is. It prints the target it is about to write to before its first request.
- `deskfleet provision` — Go-native GitLab fleet provisioning, so a Windows adopter no longer needs Git-Bash or WSL to stand up a desk fleet. It creates the seven role service accounts, their group memberships and their personal access tokens, and writes each token to `gitlab-<role>.token`, the file `desktoken --forge gitlab <role>` already reads. Each file is created owner-only and read back through the owner-only custody check before its path is printed. An inconclusive read-back prints a warning naming the file; a file that is definitely not owner-only stops the run. A run that fails partway through stops, reports every token it minted (by role, account, token name and id; never the value) for manual revocation, names every account it created that holds no token and every request whose outcome it could not check, and revokes nothing. With `--project` it also configures the protected `main` branch, MR approvals, protected release tags, the merge checks and the fleet labels. With `--avatars-dir` each new account sets its own avatar; an icon must be a regular file (a symbolic link is refused, never followed) of at most 200 KiB, and a refused icon is a recorded failure with nothing uploaded for it. Without `--avatars-dir`, or with the script's `--no-avatars`, the avatar step is skipped and named in the summary, and `--avatars-only` sets the avatars later. Each role's token is read back from its `gitlab-<role>.token` under the same link rule `desktoken` applies to that file: a link to a file in the same directory is followed, and any other link is refused, with nothing sent for that role. `--dry-run` makes no network calls, and a real run refuses without `GITLAB_API_BASE`. `tools/create-fleet-gitlab.sh` stays as the reference implementation, and a test compares the verb's role table, label table and default token lifetime with the script's own, so the two cannot drift apart.
- `deskflip` gains a `desk-decided` condition: it refuses the ready-flip when the
  `desk-decided` label and body block disagree, or when a reviewer's `Undeclared-desk-decision:`
  finding at the current head has not been cleared by a fresh verdict in the same review lane
  (a security verdict never clears a correctness finding). Absence of a block alone is never
  refused.
- `deskinbox html OUT.html [owner/repo ...]` — the whole decision queue as one self-contained HTML file (inline CSS, no scripts, no external assets, light/dark via `prefers-color-scheme`), rendering the same five-part cards `deskinbox walk` prints, plus a Flow section. `deskinbox flow [--root PATH ...] [--since YYYY-MM-DD]` prints the pipeline flow model — per-stage count, queue-against-pool-slots ratio, dwell and the named bottleneck — derived from `statusgen --bottleneck`/`--intake-debt`/`--net-flow` and `deskboard throughput`, never probing a new source; a reader that fails renders `could-not-check`, never `0`. `deskinbox flow --html OUT.html` renders the same model as a self-contained inline-SVG stage diagram with an equivalent `<table>` alongside it. This completes the Go port of the oracle's renderings started in windows-port/13 — every rendering now has a Windows-native path with no `bash`/`jq` dependency. `deskinbox` does not run the oracle's screen yet, so `assay:inbox` keeps `--walk` and `--html` on `assay-inbox.sh` (its cards carry a class line) and uses `deskinbox walk`/`deskinbox html` where the oracle cannot run; `deskinbox html` writes the oracle's `--no-screen` page. Page files are written owner-only (`0600`), a non-http(s) card link renders as `#`, and `ASSAY_STATUSGEN`/`ASSAY_DESKBOARD` may only name a `statusgen`/`deskboard` binary.
- `deskpr create|edit --decided <file>` declares a desk-taken reversible default as a
  `## Desk-decided` PR-body section (decision:/alternative:/cost: triples) plus the
  `desk-decided` label, so the driver sees at merge time which pull requests carry a choice
  the desk made rather than one already ruled.
- `deskrestamp`: first-class re-stamp verb that removes a foreign-applied `dispatched-*` stamp and re-applies the same (model, tier) pair under the session's dispatcher App, preserving content and posting a per-PR record comment naming both the original applier(s) and the re-stamp actor. It refuses — before any write, checked per label so a half-swap timeline cannot slip through — to re-attest a label unless its standing application is BOTH the roster's own blessing authority (the driver's own login, read dynamically) AND timestamped strictly before the ruling's cutoff (2026-09-27T00:00:00Z, the #336 legacy backlog): an App/bot, any other login (trusted or not), a post-cutoff application, or a present label the timeline cannot attribute is refused, so the re-stamp cannot become a laundering path for content nobody vouches for.
- `deskroster liveness` now checks GitLab-backed repos: it looks up each configured GitLab service-account identity with `GET /api/v4/users?username=<name>`, classifying an id mismatch against the pinned id RECLAIMED (checked first, whatever the new account's state) and any non-`active` state (e.g. `deactivated`) SUSPENDED — never reported as alive, and checked even when the identity is unpinned. An empty result classifies DELETED (the notice says no matching account was returned, not that it no longer exists), with a caveat: GitLab hides `blocked`/`banned`/`ldap_blocked` accounts from a non-admin token's user search entirely, so an empty result can also mean the account is hidden, not gone, and the notice says so. A GitLab response missing `state` altogether is could-not-check, never alive (#1667). Previously it printed a GitHub-only notice and checked zero identities on any non-GitHub repo.
- `docs/contracts.md` gains a "Semantic owners — one meaning, one home" section: eleven
  `S-<slug>` rows (readiness/eligibility, delivery, identity, claims, worktree lifecycle,
  decision acceptance, the review verdict/ready flip, publication scanning, status
  derivation, exit codes, and the index itself) recording today's owner, its known
  duplicate implementations by path, and its legitimate enforcement points, with an
  amendment rule tying a row change to its decision record. A brief's `design-fit:
  contract:` key can now name an `S-<slug>` id.
- `pr-review-desk` §"The reviewer's bar": a matching bullet pointing at the new clause.
- `qualgen` gains the re-fix rate metric (quality/19): a `RegressionLinkage` seam
  (explicit `regression-of:` or a shared defect-class label) joins traced
  SZZ fixes against earlier fixes, and `qualgen report` renders a new
  `## Re-fix rate (regression-suite effectiveness)` trend section — report-only,
  gating nothing until it has been measured across ≥ 2 windows.
- `skillslint` gates the per-skill frontmatter hard limits Codex and the agentskills spec enforce (`description` ≤ 1024 characters, `name` ≤ 64 characters matching `^[a-z0-9]+(-[a-z0-9]+)*$`) as exit-code-bearing findings, and reports the soft body/bundle budgets (8000 bytes, 500 lines, ~5000 tokens, 8000-character bundle description total) as advisory NOTICEs; a new repeatable `--skills-dir <dir>` flag runs just the structural + conformance checks over an adopter's own skills directory (`harness-portability/17`).
- `statusgen --lint` now flags a Verify row that runs `go test -run` with no
  `--- PASS` assertion in the same command (`gotest-run-vacuous`, advisory) —
  `go test` exits 0 and prints "no tests to run" whether or not the named test
  exists, is built, or was ever renamed away, so an unasserted row is a vacuous
  pass from the day it ships. A closed (`done`/`verified`) brief's rows are
  summarised in one run-wide NOTICE instead of flagged per row, so historical
  records are surfaced without being rewritten.
- `statusgen outcomes split --root <dir> [--check]` migrates the legacy log to the new
  per-file layout, idempotently.
- `tools/desk/internal/weight`: a Go test package that counts the desk tools' verbs,
  flags, refusal-constructor call sites and shared skill/reference rule-text lines, and
  ratchets them against a committed `ceiling.txt` (currently `# mode: advisory`, so growth
  is logged as `GROWTH-NOTICE` rather than failing CI). `go test ./internal/weight/` runs
  as part of the existing `go test ./...` CI step — no workflow change needed.
- `tools/lint-human-decision-defaults.sh` — an offline, repo-wide enumerator for the same grammar, so the defect class can be swept in one pass instead of discovered one refusal at a time at dispatch.
- `tools/lint-human-decision-defaults_test.sh` — an offline fixture test for the lint's grammar and trigger-gating, including a regression case for the placeholder-exemption fix above.
- harness-portability/18 (spec): a deterministic skillslint routing check. It scores every pair of
  skill descriptions with TF-IDF cosine and prints a NOTICE for any pair that competes for the
  same prompts. It also scores a checked-in fixture of positive and negative prompts per skill
  for rank-1 routing accuracy against a recorded baseline. That baseline starts as a NOTICE and
  becomes an exit-1 ratchet one release later. The check also runs over an adopter's own skills
  through `--skills-dir`.

### Fixed
- **(round 2, #1803 review)** The writer now re-checks the RESOLVED record path against
  `docs/streams/verify-outcomes/<stream>/<file>.json`'s exact shape after `RecordName` computes
  it — an independent, defense-in-depth guard sharing no code with the name/regex validation
  that already existed, so a regression in that validation alone cannot silently escape the
  sandbox.
- **(round 2, #1803 review)** Verify row 13's fixture is corrected: it previously could not fail
  even when the deliverable check read the brief from the wrong tree (HEAD instead of the
  record's `sha`), because the discriminating file was absent at the record's `sha` under either
  read. A second, planted file closes that gap.
- **(round 2, #1803 review)** `--outcome-record` refuses a record whose `ts` is more than 5
  minutes ahead of the writer's own clock, and `LatestPerBrief`/`LatestPerBriefAt` never let a
  future-dated record on disk win the newest-`ts` comparison — it is reported could-not-check
  instead. Before this, an unbounded future `ts` could permanently shadow every later, genuine
  outcome for its brief.
- 12 `gate: human` briefs' `## Human decision` sections carried a default line the decision-gate parser (`tools/decision-issue.sh`'s `has_parseable_default`) could not parse — missing entirely, or close-but-not-matching the literal grammar. Every offender now carries the literal `Default if no answer: none — blocks until answered` (#1673); none needed a dated default, since none had both a default and its date already stated in the author's own text.
- A "not logged in" answer only counts as the clean state once `gh auth token` (a local call) confirms no stored credential is readable either. An empty `GH_CONFIG_DIR` makes `gh api user` report "not logged in" while `gh auth token` still returns an OS-keyring login, and a verb shim whose `gh` wrapper hands that to `gh` as `GH_TOKEN` brings the human back. That state is could-not-check, and the credential is never echoed. Exporting `GH_TOKEN` as the role's minted App token clears it.
- A bare `deskclaim-ref show|list|release` run through a cell shim with no `GH_TOKEN` of its own now fails closed with `no forge token`, where before it ran as the operator's ambient login. Pass `--token-file`, or export the role's token.
- Added the missing `system-demo` row to the per-skill degradation table in the Claude Code
  binding (`plugins/assay/references/claude-code.md`). The row was left out when the skill's
  cells were added to the Codex and Cursor bindings, so `harnesslint bindings
  plugins/assay/references` exited 1 on main.
- Model-policy validation now applies the Kimi K3 low/high/max effort restriction to `k3-256k` as well as `k3`; the 256K model ID previously escaped the check.
- Re-fix linkage fails closed on unresolvable data: an errored or unverifiable
  linkage check (an explicit-path error, a `regression-of:` reference that
  matches no identified fix — including one unmatched reference alongside
  matched ones — an earlier fix's unreadable labels, an unknown mined-repo
  identity behind a repo-qualified `regression-of:`) is could-not-measure —
  never a fabricated measured non-re-fix — whenever no resolved candidate
  satisfies the ordering rule; a matched candidate that does satisfy it still
  earns the measured re-fix.
- Re-spelled three Verify rows in harness-portability brief 12 (rows 5, 9 and 11) so the
  execution witness runs the command each row means. No Expect was loosened, and row 11 now
  fails when either Cursor freshness entry is STALE.
- Report rendering escapes a could-not-measure Reason for the markdown table
  (pipes escaped, newlines flattened), so adapter error text or frontmatter
  values can no longer break a table row.
- The candidate's file list is read from the paginated REST endpoint and checked against the PR's `changed_files` count. A truncated list is could-not-check. `gh pr view --json files` stops at 100 entries and was previously trusted as complete (#1691).
- The desk secret scan no longer refuses CamelCase identifiers that carry a plural acronym (`PRs`, `IDs`), a numeronym (`K8s`, `I18n`, `L10n`, `A11y`) or a closing acronym on top of one mid-run acronym, nor a git SHA assigned to an ALL-CAPS env var whose name ends in a closed list of digest keys (`SHA`, `SHA1`, `SHA256`, `DIGEST`, `CHECKSUM`, `COMMIT`, or `HEX` directly behind one, as in a Dockerfile `ARG BASE_DIGEST_HEX=<sha256>`) and contains no credential stem (#1642).
- The deskkit mutation corpus no longer reports `could-not-mutate` for the "#203 — drop the FORWARD anchor" entry. The entry now targets the acronym forward anchor as it stands after the plural/closing-unit rework (the mid-run `startsCamelWord` check in `shortAcronymUnit`), so the truth-suite mutation gate exercises that guard again and reads `65 caught, 0 NOT CAUGHT, 0 could-not-mutate`.
- The harness-portability jcode and SpecMem spike findings (`docs/research/jcode-desk-harness-capabilities.md`, `docs/research/specmem-portability-spike.md`) now live in this repository and are registered in `freshness.yaml`. When the harness-portability stream moved here, its briefs came across but these two research docs did not, so the Verify rows of harness-portability/09 and /10 that check for the docs and their freshness entries failed on main (#393).
- The shim and the cell's `gh` wrapper export the ambient credential rather than passing it to `env` as an argument, so it never appears in a process's argv. The wrapper also drops `CELLCTL_GH_AMBIENT` before it runs `gh`.
- The verify-outcomes append-only sidecar's raised write-size cap (#1338) is retired along
  with the file it existed for; a single outcome record is nowhere near the general cap.
- Verify rows in desk-tools/18, forge-neutral/13 and 14, and windows-port/01, 08 and 09 now open with their runnable command as the first code span. The execution witness (`statusgen verifyrun`) runs the intended check instead of a label, a file name or a flag (exit 127 / false fail), and the `--consumers` rows are pinned to the implementing commit so they are not vacuous on merged main (#1805, #1786, #1795). Check definitions only; no row checks anything new and no Expect was weakened.
- Walking past a candidate never removes the approval requirement. Every PR the walk reaches must carry the reviewer App's APPROVED review at its own merged head, so an unapproved later change to a brief blocks the flip as before (#1691).
- `BriefRegressionLinkage.RegressionOf` no longer drops a present-but-unparseable
  `regression-of:` value just because another brief the same (untrailered) fix
  commit touched happened to parse cleanly — the adapter now errors
  (could-not-measure upstream) whenever ANY touched brief's value is
  unparseable, not only when every touched brief's value is.
- `cellctl` shims no longer export the operator's ambient `gh` login as `GH_TOKEN` to every desk verb. The token now travels as `CELLCTL_GH_AMBIENT`, and only the cell's `gh` wrapper turns it into `GH_TOKEN`, for a `gh` child that has no token of its own. So a `gh` subprocess still authenticates (#1145), and no desk verb mistakes the human credential for an operator override.
- `desk-shell.md` §"Authenticated transport": replaced the manual credential-helper recipe with
  the two verbs that retired it, `deskgit fetch` and `deskgit push` in their role-bound
  `--as <role>` form, and a pointer to `tools/desk/README.md` for their flags and exit codes;
  these verbs have no raw fallback.
- `deskclaim-ref`: when the server refuses a dispatch-claim create or release, the error now includes the server's own message after the status word. The git transport's receive-pack request negotiates the sideband, so a refusal no longer arrives as a bare `failure` with its reason discarded. `gitcore` also exposes the refusal in structured form (`PushRefUpdateVerdict`, and a typed `RefRejectedError` on delete), and a structural guard fails on any receive-pack request built without the capture.
- `deskdispatch` honours an inherited `GH_TOKEN` only after checking that it is the dispatching role's App (a GitHub `viewer` read compared with the roster binding; on GitLab, a match with the role's PAT custody). Any other identity is ignored with a NOTICE, and the role token is minted instead. A token whose identity cannot be read makes the dispatch refuse before any claim.
- `deskdispatch`'s identity check for an inherited `GH_TOKEN` sends its one `viewer` read only to the host the role's own GitHub credential would use. A repo mapped to GitLab, or whose origin is not github.com, is refused before any request.
- `deskevidence` no longer lets a stale local copy of a stream README revert an unrelated
  brief row. When the target's remote content carries the generated Briefs table (the
  `<!-- statusgen:briefs:begin/end -->` region), `--row <NN>` (repeatable) is now required and
  the committed content is rebased onto the remote — only the named row(s)' lifecycle cells
  (Status / Verified / Reviewed) come from the local file; the named rows' authoring cells,
  every other row and everything outside the markers come from the remote unchanged. A row
  that differs is reported (`stale-local: row <NN> ...`), never silently dropped. Duplicated
  row keys, malformed named rows, and a README whose marker does not parse are refused rather
  than guessed at. The constraint is enforced twice — against the pre-check's fetch, and again
  by the write op against a fresh fetch taken immediately before the commit — and the write
  itself is conditional on that fresh fetch's content id (new `WriteFileInput.ExpectedSHA`),
  so a table change after the re-check is refused by the forge rather than overwritten.
  Targets with no such table (brief-path merges, `.jsonl` sidecars) are unaffected.
- `deskinbox walk` and `deskinbox html` now pick and label the "latest desk note" the same way the `assay-inbox.sh` oracle does. The REST comment reader spells a GitHub App's login `<slug>[bot]`, while `gh` spells it `<slug>`. Because of that, every App's comment was treated as a desk note and printed under the wrong name. The reader now strips the `[bot]` suffix when it reads each comment. The parity tests now give each side its own login shape, and a new guard test fails if any code in the package builds a comment without that normalisation (#1797).
- `deskpost ready` now enforces the `desk-decided` ready-flip condition that `deskflip` enforces (#1694). Before this fix, a pull request that `deskflip` refused (label and `## Desk-decided` block disagreeing, a block that does not parse, or a reviewer `Undeclared-desk-decision:` finding standing at the current head) could still be flipped ready with `deskpost ready`. Both verbs now call one shared check, `deskkit.DeskDecidedRefusal`. `deskpost ready` runs it on its first read and again on its pre-mutation re-read of the pull request and its reviews, as `deskflip` does.
- `deskroster liveness` no longer reports every trusted bot login as DELETED: it now probes a bot identity at its `"<slug>[bot]"` REST rendering, matching how GitHub's `GET /users/{login}` actually resolves a GitHub App's bot account (#1665).
- `deskroster preflight`'s `ambient-identity` check no longer needs a human `gh` login for a desk to boot. "No usable ambient human identity" is now the clean state: `gh` not logged in with no stored credential behind it, a credential that answers 401 on `/user` (it authenticates as no one), or an App/integration token that answers 403 there. Before, the real probe reported each of these as could-not-check, so the only green state was a logged-in blessing human, which is the one state where a tool fall-through acts as the maintainer. A human login, the blessing login included, is now a non-blocking WARNING notice naming the login. A bot/App slug stays checked-failed. `gh` missing or unrunnable (exec failure, timeout, a server or rate-limit answer) stays could-not-check. The transport half, where every credential source git consults for the landing push URL must be the minted App token, is unchanged. The remediation now starts with clearing the ambient credential and never recommends logging a human in.
- `install` and `pr-review-desk`'s `SKILL.md` descriptions are shortened under the 1024-character hard limit so a Codex CLI no longer silently truncates or refuses either skill (`harness-portability/17`).
- `skills/README.md`: dropped the stale hardcoded `v0.1.0` heading pin (actual `plugin.json`
  version has moved on).
- `statusgen --auto-flip-model` no longer credits a PR that did not deliver the brief. A candidate PR is credited only when its single `Brief:` trailer names this brief. The resolver walks past a PR that names a different brief, an `Authors:`-only authoring PR, and a bulk-shaped PR (a `docs/streams/**`-only diff, or 3 or more `Brief:`/`Authors:` trailers) to older commits. A PR with no `Brief:` trailer, or with several, is could-not-check (#1691).
- `system-demo/SKILL.md` §"Inspiration": dropped the unmaintained inspection date from the
  credited source, keeping the credit and the standing rule.
- `tools/lint-human-decision-defaults.sh`'s placeholder exemption applied to every `decision-trigger`, not just `spec` as `tools/decision-issue.sh` itself gates it — a `start`-trigger comment-only section reported clean while the real gate refuses it. The lint now checks `decision-trigger` before exempting.
- `windows-port/brief-14`'s `decision-trigger: start` was corrected to `decision-trigger: spec` — its Task section already has the executor author `## Human decision` at pickup, which is `spec`'s own definition, and `start` made `deskdispatch --gate-human` refuse the brief on its still-unauthored placeholder section.

### Changed
- A docs-only Verify PR is no longer credited as a brief's delivering PR. The walk continues past it to the delivery PR. A brief whose delivery PR never touched the brief file, or carries no `Brief:` trailer naming it, now stays `verified` with a structural NOTICE (exit 0) instead of flipping to `done`. It is not refused (#1691).
- A push URL git cannot normalize or with an empty host, a pattern git would skip as unparseable, a netrc file that exists but cannot be read, or one that holds a NUL byte (curl splices the next line onto it) and nothing else reddens is could-not-check, never clean.
- Bookkeeping only: the same-identity attribution hard-reject (a brief whose
  authoring commit and every commit that touched its Evidence section share one
  git identity now fails the committer-identity cross-check as a hard PROBLEM
  instead of a NOTICE) is already announced under `CHANGELOG.md`'s `v1.0.24`
  entry for `statusgen`'s git-committer-identity cross-check. This fragment adds
  no new behavior; it exists only because this PR's own `Brief:` trailer is the
  board's delivery witness for that landed change.
- Corrected framing (review F2): merging this PR does **not** flip board row
  `measured-status/05` from `todo` to `implemented`. The push-to-main regen job
  (`.github/workflows/assay-statusgen.yml`) only runs `statusgen --root .` and
  commits the regenerated `STATUS.md`; it never invokes `reconcile`. The
  reconcile step that derives a brief's lifecycle cell from its `Brief:`-trailer
  PR history is staged, not applied — `.github/assay-statusgen.reconcile.patch`.
  The row moves to `implemented` in a follow-up, once that reconcile step is
  wired into CI (or via the carve-out B same-repo path), using this PR's merged
  `Brief:` trailer as the witness.
- Cost of the ruling: a false red when the host or `default` appears only in a comment, a macro or a value, or when a line is longer than 4095 bytes counting its newline even if it holds neither word. The operator clears it by editing their netrc. Covered readers: libcurl 7.61.0 through 7.85.0 (lines read whole by the word rules, longer lines by the long-line rule) and 7.86.0 onward (which drop an over-long line or stop at it, never split it). Not covered, and the check may read green there: libcurl 7.60.0 and older, which read a line in 255-byte pieces (git has required 7.61.0 or later since git 2.48), and any reader that separates tokens on a letter, digit, `.`, `-` or `_` within a line it reads whole, drops more than one byte after a quote, or decodes multi-byte escapes.
- Desk-role skills (`intake-desk`, `the-desk`, `verify-desk`, `worker-desk`) state current rules directly, without references to superseded versions; `intake-desk` trigger list restores `triage the front door` alongside the collapsed categories.
- Git-Bash/WSL is no longer needed to install or to provision a GitLab fleet: `deskfleet provision` / `deskfleet labels` are native, and `tools/create-fleet-gitlab.sh` is a labelled fallback. It is still needed for Claude Code's SessionStart hooks and for GitLab bulk PAT renewal (`tools/renew-fleet-gitlab-tokens.sh`, bash + `glab`) until `windows-port/16`. The `create-labels` primitive now points at the forge-neutral `deskfleet labels` verb, keeping the `gh label create` lines as the GitHub-native equivalent.
- Parked the `iso-9001` stream (README `status: parked`) to free attention-budget for higher-priority work; its briefs are kept as authored and it re-activates by a README `status:` flip.
- Residual: inside `docs/streams/verify-outcomes.jsonl`, a 64-lowercase-hex value under a key ending in `.<ext>` (1–5 lowercase letters, e.g. `"client_secret.go":"<64 hex>"`) is not reported by `generic-api-key`. Dedicated rules (GitHub tokens, AWS keys, private keys) still apply there.
- Reword a verify-desk skill example to drop internal stream names; the live-probe section now describes the stranded-brief class generically.
- Scoped loosening of the `pattern-sweep` (gitleaks) leak control, authorized by the maintainer's ruling on #1793 (option A): a new `.gitleaks.toml` allowlist stops `generic-api-key` from reporting verify-wake-v1 receipt input digests in `docs/streams/verify-outcomes.jsonl` when the digested file's path contains `key` or `token`. It applies to that one file only (path anchored at both ends) and only to a `<file path>.<ext>":"<64-hex sha256>"` match. An identifier-keyed value such as `"api_key":"<64 hex>"` in the same file, and the same digest shape in any other file, still fail the leg.
- Stream boards: `composability/01` and `forge-gitlab/13` flipped `todo` to `implemented` — `composability/01`'s delivery PR #957 carries the row's sole `Brief:` trailer; `forge-gitlab/13` carries that trailer on two merged PRs (#1084, which shipped the per-row-degrade code and its three tests, and #1191, the same-brief follow-up that corrected #1084's changelog/board-row framing and is `statusgen reconcile`'s own witness for the row) — the rows were never flipped. `forge-gitlab/16` stays `todo`: its merged PR (#1155) delivered the offline half only, and the live review-tick walk it gates on has not run (every live cell in the conformance table reads could-not-check).
- The `ambient-identity` preflight check's credential-chain half now matches how git itself resolves the credential for a push: `credential.<url>` patterns with no host (`""`, `/`, a path, a scheme-only URL) apply the way git's partial-URL fallback applies them; every URL `git remote get-url --push --all` lists is judged, not only the first; and a helper passes only when it IS the desk's App token helper over the minted token file (`deskkit.AppTokenHelper`, which `deskwt role-init` now builds from too), not merely a command that mentions that file, with the helper value matched untrimmed, as git reads it (a leading blank makes git run it as `git credential-…`, which answers nothing). The detail names the command git would actually run.
- The bash `cellctl` script moved from `tools/cellctl/cellctl` to `tools/cellctl/testdata/cellctl-shell-oracle.sh`, with a banner saying it is a test oracle and not the launcher. The launcher is the Go program in `tools/desk/cmd/cellctl`, the binary releases ship. Neither implementation changed behaviour. The usage test, the parity harness and the behavioural suites now read the new path, and `docs/cellctl.md` and `docs/adopting-assay.md` name the Go program as the launcher (#1732).
- The model-capability floor's actor check (`deskflip`, `deskpost`, `deskautolane`, and the `deskdispatch` stamp step) now reads `IsStampAuthorityLogin` — the bound dispatcher slugs plus the roster-configured allowance — so a legitimately dispatched PR stamped by an allowed trusted login is no longer refused a verdict.
- The netrc read is a state-free scan (driver ruling on arbiter packet #1622, option 1). It reads red when the push host or the keyword `default` occurs anywhere in the file, ASCII case-insensitively, delimited only by the start or end of the file or by any byte that is not an ASCII letter, digit, `.`, `-` or `_`. It keeps no quote state and no per-reader separator table, so a byte one libcurl reader treats as a separator and another does not (VT, FF, a byte >= 0x80 before a `"`) can no longer open a quote in the scan that hides a real entry (SR-1614-8). Three add-only rules follow it: the same match over the file with every backslash removed (a quoted host spelled with escapes such as `\.`); a match right after a closing quote and one more byte (the byte every libcurl before 8.21 drops there); and a red on any line longer than 4095 bytes counting its newline, because libcurl 7.61.0 through 7.85.0 read netrc with `fgets` into a 4096-byte buffer (`lib/netrc.c`, `char netrcbuffer[4096]`) and tokenize such a line in pieces, so a piece boundary can start or end a token inside a word (SR-1614-9). None can turn a red green.
- The verify-desk skill now carries an Evidence-PR state table over verdict, at-head and mergeability. Its rows are exclusive and exhaustive, with one owner each. The verify desk merges main, one PR at a time and oldest first, into a `CONFLICTING` Evidence PR whose latest verdict is clear or has been answered by a push, whether or not its approval is at the current head. It stops once the PR is mergeable, while an unanswered finding stands, or once it is merged. It never merges main into unreviewed PRs, and an `UNKNOWN` mergeability is re-read, never acted on. Every landing appends to one shared outcomes log, and the forge's server-side merge ignores that log's `merge=union` attribute. The pr-review-desk skill takes the matching rows: first reviews regardless of mergeability, and a priority re-review of the delta as soon as the PR is mergeable again (#882).
- The verify-desk skill now spells out three rules for a verify-wake receipt that reviewers kept bouncing: `inputs` declares the brief's deliverable files, `blocker_ref` is a real issue or PR reference (file the bug first), and the brief's own `file:` revision is the hash of the brief as it lands, Evidence append included, so the receipt holds instead of firing on landing (#882).
- `build-less-brittle` stream priority raised from P1 to P0 (+1000 priority weight in Next-up scoring). Briefs 01 and 03 (wave 0, no dependencies) already led this repo's Next-up at P1; the order is unchanged. The stream now falls under the `P0-finding` roadmap health rule. No brief content changes.
- `cellctl desk` (Go launcher, host Claude launches) now exports `CLAUDE_CODE_AUTO_COMPACT_WINDOW=200000` so a long-running desk session compacts before its context grows unbounded; a value in `cell.env` or the launching environment overrides it.
- `cellctl desk` under a model policy no longer exports `CLAUDE_CODE_EFFORT_LEVEL` to a Claude launch (neither in the launch environment nor in the `--settings` env block) and scrubs an inherited ambient copy alongside `MAX_THINKING_TOKENS`. Effort now travels only as `--effort`, so a child agent's `effort:` frontmatter can raise that agent above the session level instead of being pinned to it by the env var. The Codex arm is unchanged.
- `cellctl` (Go launcher): the seeded Kimi provider defaults and the example model policy now pin the **strong** and **mid** tiers to `k3-256k` (the provider's "K3-256k" model, chosen for sustained windows); **top** and **fast** keep `k3[1m]`. Existing `providers.json` files are never overwritten, so this reaches new installs only.
- `deskevidence` refuses (exit 5) any write to a shared appended `docs/streams/*.jsonl` log,
  by shape, not just the retired verify-outcomes name.
- `docs/adopting-assay.md` § CI-proven status corrected: the Windows CI leg is **live** at `.github/workflows/windows-ci-leg.yml` (`windows-port/04` is `done`), not "staged, pending promotion". It runs on `v*` tag pushes and `workflow_dispatch` only, not per PR. The staged copy under `ci/staged-workflows/` is noted as carrying further not-yet-promoted work, not a stale duplicate.
- `docs/adopting-assay.md` § Windows adopters now leads with the three-command install (`bootstrap-windows.ps1` → `deskinstall --harness cursor` → `assay:install` inside Cursor). It also states the open gap that nothing in the sequence places the first `deskinstall` on a clean host, with two interim routes. The previous fifteen-step manual route is preserved complete as a labelled appendix, and the Cursor install steps have one authoritative account (`adopting-assay.md`), with `plugins/assay/references/cursor.md` reduced to a pointer.
- `measured-status` stream activated (`parked` → `active`, P1): its scoping is approved and its briefs join the Next-up board.
- `plugins/assay/skills/install/SKILL.md` § Scope widened from "the acquisition arm is the only OS-specific piece" to all three Windows-specific arms — binary acquisition, Cursor harness placement (`deskinstall --harness cursor`, run before the skill; its step 5 confirms it with `--check`), and GitLab fleet provisioning (`deskfleet`, `gate: human`, now named on the NEVER-autonomous list) — while keeping the two honesty caveats (the `bash`+`jq` SessionStart-hook workaround and the BLOCKED native `windows/arm64` smoke).
- `pr-review-desk`: the generated-table bounce's carve-out B now admits a same-repo brief's
  existing row as well as a cross-repo one, on the same bar. The row is promoted Status-only from
  `todo`/`in-progress` to `implemented` with its `Verified`/`Reviewed` stamps untouched, and only
  when `statusgen reconcile --backfill --apply` run on main (with the board repo as `--repo` for a
  same-repo brief) reproduces it byte-identically. The reviewer still checks, on every row, that
  the brief's named files and symbols exist on the delivery repo's main. A row with a trailer-only
  witness is admitted only when that check passes, a hunk that touches any row carve-out B does not
  admit bounces whole, and statusgen-source PRs stay outside it. Previously a same-repo row always
  bounced, and the only compliant path was to carry the whole reconcile output, which also
  promotes backfill-only rows whose work has not landed.
- commsloop router: the hardcoded `risk = false` passed to `Assign` now carries a `// Derivation:` block proving why the literal is sound today (the cellmsg-v1 envelope has no risk field for a sender to set), with the envelope wiring that would let the value be derived tracked as a follow-up issue — the `risk: yes -> tier: human` backstop's silent disablement is now on record rather than implicit.
- verify-desk and pr-review-desk: sibling Evidence PRs writing ONLY the new per-file layout no
  longer conflict on outcomes; a `CONFLICTING` verdict on such a PR now means a real content
  conflict. Both skills add a TRANSITION-WINDOW clause: a PR still appending to the shared
  `docs/streams/verify-outcomes.jsonl` log still conflicts with every sibling PR that also
  touches it, exactly as before, until #1802 retires the log — resolve that case by merging main
  locally (the `merge=union` driver still applies to a local merge).

## v1.0.27 — 2026-09-25

### Added
- A **publish-identity gate** now refuses, at the push boundary, to publish a commit whose
  author or committer is not the session role's bound bot identity. `deskpr create`,
  `deskpr update` and `deskevidence` run it before any network write (and `deskpr --check`
  reports it as a local gate): every commit in `refs/remotes/origin/<base>..HEAD` must be
  authored **and** committed by the role's identity — a GitHub bot-USER-id noreply address,
  a GitLab service-account noreply shape, or a trusted GitLab session address — or the write
  is refused (exit 5) naming the commit, the identity found, the identity expected, and the
  remedy. Forge-created merge commits (GitHub's "Merge pull request" / "Update branch") are
  exempt; there is no override flag. This is the push-time layer that stops a worktree with a
  stale `user.*` (a shared checkout, a manual `git worktree add`, an editor's git) from
  publishing commits attributed to the wrong actor, however the worktree acquired its
  identity.
- A byte guard, `TestPS1EncodingIs51Safe` in `tools/desk/internal/deskkit`,
  fails CI when any `*.ps1` in the tree contains a byte above `0x7F` and does not start with a
  UTF-8 BOM. It runs in the existing build-test job, so no workflow change is needed to make it
  gate. The staged Windows CI leg (`ci/staged-workflows/windows-ci-leg.yml`) also gains a real
  Windows PowerShell 5.1 `ParseFile` check over every tracked `.ps1`; it takes effect once a
  maintainer promotes the file. The byte guard covers the encoding class only; other
  5.1-only parse errors, such as PowerShell 7 operators, are caught by the promoted
  `ParseFile` step and by nothing at PR time.
- A new `statusgen` stream brief (14) plans an advisory `--lint` rule for a Verify row
  whose `go test -run` selector can pass on "no tests to run", because the row never
  asserts that the named test actually ran.
- A structural class guard (`TestGitHubMinterReachedOnlyFromForgeArms`) walks `tools/desk` and fails on any reference to a link of the GitHub App mint chain, whether a call or a function value, outside a reviewed allow-list keyed by file, function and name. The links are `RoleTokenForOwner` / `RoleTokenForRepo`, the resolver's GitHub arm `githubAppRoleToken`, the custody pass-throughs above it (`custody`, `githubCustody`), and beneath it the primitive `mintRoleToken` / `tokenMinter` and the per-process token memo (`lookupRoleTokenMemo`, `roleTokenMemo`). The arm may be called only by the three GitHub-resolving entry points and by `githubCustody`. `custody` may be called only by the two functions that resolve the forge before it (`ResolveForge`, `ForgeGitEndpointFor`). The exported minter may be called only by the arm and by `cellctl deskd`'s forge-switched GitHub arm.
- Before a policy launch, `cellctl` (Go) refuses user, project (and parent), local and managed Claude settings whose `availableModels` widens the policy, and any `modelOverrides`. It rechecks the role worktree after the worktree is created. `cellctl up` preflights every role (credential, harness on PATH, Claude version floor, settings scan) before it opens any window, and `cellctl check` reports the same preflight as one row per role.
- Briefs can declare `budget:` beside `effort:` (an amount with its unit — `400k tokens`, `25 USD`) and `outcome:` (the requirement id the brief should move, or `none`). `statusgen --lint` flags a unitless budget, an outcome that is not a registered requirement id, and — as an advisory NOTICE — a newly authored brief that names no outcome at all.
- Class guards for the on-behalf-of trailer in both modules (`TestOnBehalfOfRenderedOnlyThroughTheResolver` in the desk tools, `TestWitnessOnBehalfOfRenderedOnlyThroughTheRenderer` in statusgen). They fail on a second renderer, on `OnBehalfOfPrefix` used to compose a trailer, on a literal target repo, and on any resolver call site missing from a reviewed allow-list.
- Design-decision records can carry a `ruling:` link to the human's ruling comment on the record's decision issue. `statusgen --corroborate` resolves the link through the forge and passes only when the comment exists, was never edited, names the record by its `DR-<slug>` id, sits on this record's decision issue in the same repository, and was written by a mapped human (not a bot). A ruling corroborates only the human who wrote it, never another name in the same `decided-by`. Each failure has a named reason, and `statusgen --lint` checks the link's shape (registers-v1 §7.5).
- Dispatch claims are now kept in whatever store the roster names: a new `ASSAY_CLAIM_STORE` roster key (`file` or `service`), with `ASSAY_CLAIM_DIR` and `ASSAY_CLAIM_SINGLE_HOST`. `deskclaim-ref` and `deskdispatch` both ask one resolver, and no flag selects the store. A configured store that cannot be used is refused (exit 6) before any worktree is cut or credential minted, and is never replaced by another store. The `file` and `service` stores are not in this release, so setting either is refused and the message names the release that ships it ("the release that ships the file store" / "the served store").
- New `deskinbox` verb (`tools/desk/cmd/deskinbox`): a Go port of `assay-inbox.sh`'s
  `table` (default) and `walk` renderings — the `ask-decision` skill's actual entry point
  — with no `bash`/`jq`/`make` dependency, so it builds and runs on native Windows. The
  five-part decision format builder is byte-parity tested against the oracle's own jq
  program, extracted verbatim at test time.
- New `deskmonitor` verb (`deskmonitor inbound`, `deskmonitor pr`): Go ports of the intake and review
  desks' stateful pollers (`inbound-monitor.sh`, `pr-monitor.sh`) with no `bash`/`gh`/`jq`
  dependency, so a desk role's inbound surface runs on native Windows. Same arguments, knobs, state
  files, exit codes and output grammar; a parity test runs each script and the verb over recorded
  forge fixtures and diffs them. The pr poller reports PRs opened against an empty baseline as
  `opened` (the script reports them as `closed`).
- New `desktick` verb: the tick summary-line grammar (`regexp` / `validate` / `check`), the Go port
  of `tick-summary.sh`, with its published regexp pinned equal to the script's.
- New `gate: human` brief `desk-supervision/23`: Evidence-only changes land on a PR-required main without a PR. A validator admits only Evidence rows and outcome-log appends that the verifier App wrote (a staging-ref ruleset plus a forge-verified writer binding; the lander's own merges of main into a staging ref must equal the true merge of their parents), a dedicated lander App is the only identity that may skip the PR rule and re-checks scope and writer itself before landing, a post-land audit selected by the push halts the lane on a mismatch, or on an input it cannot read, in a lander push and only a human can clear the halt, the verifier App keeps no write to main, every job holding a key runs on a fresh GitHub-hosted runner, and a rejected or stranded landing falls back to the batch Evidence PR.
- Planned work for GitOps installs of the desk containers: an opt-in unattended mode that runs each desk as a tick-mode CronJob, and copyable Argo CD and Flux install examples pinned to a release tag (desk-containers briefs 12-14).
- The `worker-desk` skill gains a budget checkpoint: at 80% of a brief's `budget:` the worker files `help wanted` with the escalation packet instead of continuing silently. It adds a filing and never skips or satisfies a review, Verify row or human gate.
- The roster keys `ASSAY_AUTOAPPROVE_AREAS`, `ASSAY_AUTOAPPROVE_EJECT_LINE`,
  `ASSAY_AUTOAPPROVE_FPY_FLOOR` and `ASSAY_AUTOAPPROVE_DAILY_CAP` are recognised by both the desk
  tools and statusgen, so a roster that sets them no longer refuses the whole configuration.
- The worker-desk skill's cockpit-aware worktree step honours `ASSAY_COCKPIT` when set (`supacode` / `herdr` / `orca` / `plain`, with `tmux` meaning `plain`): a named cockpit whose CLI is not on PATH is a refusal naming it, never a silent substitute; a named cockpit that cannot pin the base and per-item path still falls through to the plain `git worktree add`. Unset, it keeps the presence-on-PATH order.
- Three forge-seam ops on both backends — `RunWorkflow`, `ApproveGate`, `RunStatus` (inventory
  rows 49–51). GitHub resolves the run a dispatch created by a correlation read and refuses an
  ambiguous match rather than guessing; an approval is resolved against the run's pending gates
  and refuses a name that matches none. On GitHub an App credential cannot approve a
  required-reviewer gate (approval needs `Deployments: write`, and required reviewers are users
  or teams), so `deskrun approve` there reports could-not-check and writes nothing.
  GitLab starts pipelines with the pipeline trigger token and approves on the gate shape the roster declares (`manual-job` or `environment`).
- Three new `quality` stream briefs plan a regression suite. Brief 17 sets a
  `TestRegression_<repo>_<issue>` naming convention and adds a CI gate that goes red when a
  regression test fails, when the regression-test count drops against the base, or when the
  test selector runs nothing. Brief 18 adds a stub-coverage report, report-only at first,
  listing test seams that every test stubs and none runs in production form. Brief 19 adds
  a qualgen re-fix metric: how often a fix repairs a defect an earlier fix had already
  addressed. The briefs are planning only; the tools, the workflow and the metric land
  when each brief is implemented.
- `TestDefectClassClauseIsOneWordingAcrossImplementerKits` in `tools/desk/cmd/deskdispatch`
  fails when the defect-class clause is missing from either implementer kit or its wording
  differs between them.
- `cellctl desk` (and every window `cellctl up` opens) exports the cell's resolved cockpit as `ASSAY_COCKPIT` — `tmux`, `herdr` or `orca`, never `auto` — and refuses an explicit cockpit that is not available instead of exporting something else; `up` threads the cockpit it resolved onto each window, and `cellctl check` prints the value and the worker-desk worktree arm it selects. Both the shell oracle and the Go port.
- `cellctl` (Go) now enforces the model policy at runtime, not only at launch. A policy launch passes Claude `--settings` with an `availableModels` allowlist of the provider's pinned IDs, plus `PreModelSwitch` and `PreToolUse(Agent|Task)` hooks. The hooks call back into `cellctl model-policy hook` and block (exit 2) a switch to an unpinned or denied model, or a child agent that is unmapped, denied, or cannot run at the parent's effort. A hook that cannot load the cell or its policy also blocks, and so does one whose policy no longer matches the SHA-256 the window launched with (restart the window to adopt a changed policy). The hook command line ends in `|| exit 2`, so a removed or non-executable hook binary blocks too. A hook cannot be made to hang instead: it refuses a policy path that is not a regular file, and it exits 2 by itself after 5 seconds (each hook entry sets a 30-second `timeout`).
- `deskautolane`, the verb of a narrow auto-approve lane: a PR is admitted by category (every
  changed path inside an area a named human opted in, no tripwire) and ejected, one-way, by a
  demotion score recomputed at each gate. It ships inert: the lane is closed unless all four new
  `ASSAY_AUTOAPPROVE_*` roster keys are set, every write it can make requires a signed `R-8`
  ruling line resolved to the blessing authority, and this release carries no merge mutation
  (`merge --dry-run` reports what a merge would need and writes nothing; `merge` refuses at
  `merge-write`). The enactment gate reads the register through the forge at the default
  branch and requires the sign-off comment, on a thread in the register's own repo, to be a
  User's explicit `Enact: R-8` acceptance with no rejection in it. An ejection latches both in
  the local audit log and through the reviewer App's marked comment on the PR, and the lane
  admits only PRs based on the default branch.
- `deskboot` now ALARMS a red operating-envelope preflight: it files one `to:desk` issue naming each failing check and its state (never the checks' local details, since the issue may land in a public repository), under the `### Evidence` fence deskfile's blocker-evidence gate requires, deduped by a marker per role per day, so a red envelope is no longer heard only by the window that printed it. Only deskfile's title-dedupe refusal counts as already filed; any other refusal is a warning and is retried on the next red boot. The alarm never changes the boot's verdict, and the boot's refusal text reports what the alarm actually did.
- `deskdispatch --rework`: a rework row whose PR is already MERGED dispatches as a follow-up on a new branch instead of re-cutting the merged branch.
- `deskdispatch` resolves `--brief` once (absolute, else under `--root`, else under `--claim-root`) and feeds that one file to the human-gate detection, the decision script and the prompt, so a `gate: human` brief kept in the tracking checkout still fires its decision gate; a `--brief` that resolves nowhere is refused. Registry alias keys, `repo:` values and `self:` are grammar-checked before use.
- `deskdispatch` resolves a cross-repo item's deliverable repo through the alias registry (`graph-repos.yaml`) — from the brief's `deliverable_repo`/`homed-in` or an `<alias>:` item-key prefix — and hard-fails before anything durable when that repo is not `--root`'s own; an unregistered alias is refused by name. The claim, the token and the prompt's `deskpr create --root <tracking checkout>` hint follow the resolved repo.
- `deskfile new --correction "<message>" --label skill-bug --section … --reading …` composes
  a **skill-bug** issue from this session's last `deskack` receipt (receipt line, correction
  verbatim, `$DESK_LOOP`, skill+section, and the desk's reading) — the tool composes the body,
  not the desk — and refuses when no receipt was recorded in the last 30 minutes.
- `deskfile new` gains a **blocker-evidence gate**: a filing labelled `needs-decision`,
  `help wanted` or `question` is a blocker claim and is refused (exit 5) unless its body
  carries an `### Evidence` heading followed by a fenced block. `human-only` is an act, not a
  claim, and is not gated; `attach` observations are unaffected; the refusal takes the
  audited `--force-new --reason` bypass.
- `deskflip` and `deskboard` accept a same-head APPROVE over a standing CHANGES_REQUESTED in exactly one new case: a documented body-edit re-verification. The CR declares `Blocked-On-Body: <finding-id> <body-digest>` as its only blocker, and the later APPROVE documents `Resolved-Body-Finding:`, `Body-Reread-Digest:` (which must match the live PR body and differ from the CR's) and `CI-Green-At: <head>`, and the forge's own record of the body's last edit must be later than the CR. Any other same-head approval is still refused, and `deskboard` still shows it as `SUSPECT-APPROVAL`.
- `deskpr` accepts a third link trailer, `Authors: <stream>/<NN>[, …]`, for a PR that only authors briefs. It names the briefs the PR writes without claiming to deliver them, so the dispatcher, `fanoutloop plan` and the derived board no longer treat a merged authoring PR as the brief's delivery. Every entry must resolve to a brief file under `--root`.
- `deskrebaseline` — a verifier verb that turns a provably-intact-but-stale `## Verify` row into a
  one-row re-baseline draft PR instead of the Nth duplicate "stale Verify" issue. It classifies a
  failing row and fails CLOSED: a row is re-baselined only when git history POSITIVELY proves the
  work intact, and everything else — a real behaviour change, a deliverable gone with no rename
  hop, any row of a risk-bearing brief, or anything unproven — is refused and filed as today. In
  this first cut only the `safe:rename` class fires (pinned path gone, a single git rename hop to
  an existing file); the `safe:count` and `safe:idiom` classifier arms ship as not-yet-wired
  scaffolding and are marked *not yet implemented* in `docs/rebaseline.md`, so an unwired class
  falls through to refuse, never launders. The verb never merges and never lands on `main`; the
  re-baseline is a PR, reviewed by the reviewer App and merged by a human. Dry-run by default
  (prints the classification and git evidence, creates no branch — note dry-run executes the
  row's command as the classification probe, under a forced `KUBECONFIG=/dev/null`); `--open`
  pushes the `rebaseline/<stream>-<NN>-row-<K>` branch and opens the draft PR via `deskpr create`.
  Before any write, `--open` refuses unless three things hold: the brief resolves inside
  `--root` (ids resolve under `--root` first), `HEAD` is the fetched `refs/remotes/origin/main`,
  and the checkout is clean. The commit names only the brief's path, so the verb never writes
  another checkout and never sweeps unrelated staged work into the one-row PR.
  The verify-desk skill now runs it on a stale-class FAIL before filing. See `docs/rebaseline.md`.
- `deskroster preflight` gains a sixth envelope check, `ambient-identity`: the ambient `gh` login a tool fall-through would use must be the blessing human (not a bot/App slug, not a non-blessing login), and every credential source git consults for the landing remote's push URL — the ordered credential-helper chain across all config scopes, plus an embedded URL credential or an Authorization `http.extraHeader`, which git uses ahead of any helper — must be the minted App token, so a competing earlier helper reads red instead of hiding behind the last-configured one. Not-applicable on a GitLab-forge repo.
- `deskrun` — a new outward-write verb that starts a workflow run (`deskrun <owner/repo>
  <workflow> --ref <r> [-f k=v …]`), clears ONE named deployment gate on it (`deskrun approve`),
  or reads its lifecycle (`deskrun status`), under a per-repo **run credential** the roster binds
  in the new `ASSAY_RUN_CREDENTIALS` key. The dedicated `release-runner` credential proceeds; a
  repo bound to `human:<name>` is refused (exit 5) before anything is minted, so no ambient
  `gh`/`glab` credential is ever read; an unbound repo is could-not-check (exit 6).
- `desktoken` recognises the `release-runner` role (its own App on GitHub; on GitLab a trigger
  token, which `desktoken` never tries to self-rotate).
- `docs/adopting-assay.md` documents the from-source Windows signing loop, the tightly-scoped
  Trusted-Root import (code-signing EKU, CurrentUser, this-machine-only) and how to remove it, the
  honest AV limit (signing names the publisher but does not clear ML/heuristic verdicts — a
  per-machine folder exception on the install dir does), and notes signed **release** assets as a
  documented follow-on that needs a real certificate.
- `docs/streams/forge-neutral/conformance-report.md` — the skeleton of the forge-neutral
  conformance record: the substrate, round-trip, negative-path, boundary and Verify-row tables,
  each row naming the verb invocation it expects, and every result cell reading
  `NOT YET RUN` until the supervised live run fills it. No row is credited.
- `metrics-harvest cost` reduces per-desk, per-model cost telemetry into a cost-per-passed-Verify-row line; a desk with no cost telemetry renders `could-not-check`, never `0`.
- `plugins/assay/commands/inbox.md` and `plugins/assay/skills/ask-decision/SKILL.md` now
  invoke `deskinbox`/`deskinbox walk` for those two renderings, with the bash oracle kept
  as the documented fallback and as the current renderer for `--html`/`--flow` (split to
  windows-port/15).
- `scripts/build-windows.ps1` gains an **opt-in** `-Sign` switch that Authenticode-signs the
  built Windows PE files with a local self-signed code-signing certificate, before
  `desk-manifest` hashes them. Off by default (unsigned builds are unchanged); resolves the cert
  from `-CertThumbprint`, `$env:ASSAY_CODESIGN_THUMBPRINT`, or subject `CN=Assay local tools`, and
  **fails closed** with a setup snippet when `-Sign` is set but no code-signing cert is found.
- `statusgen --instrument-audit` classifies every declared statusgen flag WIRED / COLD / DARK by grepping its consumer roots; `--lint` NOTICEs each DARK flag (0 consumers and declared >30 days ago) as a retirement candidate. Read-only and advisory — nothing is retired by the tool. `--json` and `--roots` supported.
- `tools/renew-fleet-gitlab-tokens.sh` renews every configured Assay role PAT on a GitLab instance in one run. For each role it rotates the live PAT, found by its stable name. It creates a PAT only when the role has none, and it refuses when a role has more than one active PAT with that name. It then replaces each `gitlab-<role>.token` atomically through an owner-only temp file, and on the documented symlink layout it writes through the link. It runs as an Owner of the top-level group named by `--group`, through `glab api` on the group service-account PAT endpoints. It never requires or probes for instance administrator. A role whose active PAT was used inside the in-use window is skipped by default and reported, not rotated — `--rotate-in-use` overrides it. Skipped roles are counted apart from renewed ones, and a run that skipped any role exits 3, never 0. A PAT listing is judged on its parsed JSON: one with no JSON value (zero bytes or whitespace only), a non-array or a malformed record fails closed instead of being read as "no active PAT". Every check runs before the first rotation. The run fails closed on malformed output, stops at the first failed role and prints the `--only` list that resumes it. `--dry-run` prints the plan. Secrets never appear in argv, logs or the report (#1630).
- deskkit's `Account` carries the forge's actor `Type` where the read reports one (GitHub's
  comment read), so a gate can refuse an App or Bot artifact.
- harness-portability/17 (spec): a skillslint conformance rule that fails CI when a shipped
  skill's `description` exceeds 1024 characters or its `name` breaks the agentskills 64-char /
  lowercase-hyphen / matches-directory rules (the limits Codex truncates or refuses on), with
  body size and the bundle-wide description budget reported as advisory NOTICEs, a
  `--skills-dir` flag so adopters can run it over their own skills, and the two over-limit
  descriptions (`install`, `pr-review-desk`) shortened.

### Fixed
- An on-behalf-of relay no longer counts as a human's authority. The offline readers of a finding's `authorized-by:` and `parked-by:`, of a deploy record's `authority:` and `rollback-approver:`, and of a decision record's `decided-by:` now remove the relay (marker and principal) before they look for a `human:` token. A relay therefore cannot pass the lint while the corroboration lane, which strips the marker, has nothing to gate. This also closes the login form of the relay in a deploy `authority:` and a `decided-by:`.
- The GitHub App token minter is now reached only after the forge serving the repo is resolved, or found unresolved with no origin remote in hand (the existing GitHub default for roster-only callers). `deskwt role-init` selects its credential through the shared forge-aware resolver (`deskkit.ResolveRoleCredential`) instead of its own forge branch. `deskgit push/fetch --as` refuses a GitLab-served origin before any token is minted, where it previously minted a GitHub App token for that origin. `scanloop`'s poller identity records a GitLab-served owner as a named keyring fallback instead of minting for it. GitHub repos on `github.com` behave as before.
- The `--corroborate` decision-record lane now re-reads `decided-by:` for every record under `docs/streams/decisions/` a PR adds or edits, not only files named `DR-<slug>.md`. It covers the same file set the register lint and the design gate read, so a record whose file name is not DR-shaped can no longer carry an ungated `decided-by:`. The lane binds a `ruling:` link to the record's frontmatter `id:` (the id the design gate resolves), not to its file name, so such a record can be ruled on; a record with no valid `DR-<slug>` id is refused with a detail that says so, and `--lint` reports it.
- The `deskkit` test suite can no longer write to the operator's real desk-tools state directory (the audit log the scan overrides are reviewed from): a package-wide guard refuses any test that resolves it, by any path spelling (relative, case-variant, symlinked, or not yet created), and fails the run, naming the test. The fixture home now redirects `USERPROFILE` as well as `HOME`, and the scan-override test uses the hermetic `setup` fixture. Production audit behaviour is unchanged.
- The `statusgen --lint` principal-attribution check and the `--corroborate` lanes now accept an on-behalf-of principal in either form the writers stamp: a configured login or a configured neutral name. Before this, a public-target trailer read as an unrecognised principal or as an uncorroborated `human:<name>` stamp.
- The witness reads the checkout's git remote as a veto on the login form. A stream README that states a `:private` repo, in a checkout whose origin is not known-private, now gets the neutral name.
- Token-custody checks no longer follow a symlink at the custody path. The GitHub App token cache refuses any link there: `desktoken` will not hand out a link that is present when it checks the cache as a cached token, and will not write a minted token through it. GitLab custody (`ForgeFor`, the boot preflight probe and `desktoken --forge gitlab`) still accepts the documented same-directory `gitlab-<role>.token` link and refuses a link that resolves outside the custody directory. For that followed link, the regular-file check, the owner-only check (0600 on unix, the owner-only ACL on Windows) and the read all take the resolved target, not the link.
- When the caller holds the target's origin remote (`deskwt role-init`, `deskgit push/fetch --as`), the GitHub App token is handed back only when the origin's host is exactly `github.com`. Any other origin host is refused before any mint: a lookalike, a trailing-dot or userinfo-shaped URL, a self-hosted GitLab or GitHub instance, or an origin that does not parse to a host. This holds whether the roster is silent or maps the repo to `github`, and the refusal names `ASSAY_REPO_FORGES`. A token minted against `github.com` authenticates nowhere else. `deskwt role-init` on a local-path origin still wires no helper and no longer resolves a credential for it. `deskgit push/fetch --as` on a local-path origin is now refused.
- `deskclaim-ref acquire` names the server's refusal when a claim create is rejected and the follow-up read finds no holder, instead of only "rejected but no claim exists" — distinguishing a lost compare-and-swap from the forge refusing the credential.
- `deskdispatch` no longer refuses a fresh dispatch of a brief because the docs-only PR that authored it carries its `Brief:` trailer. A PR is set aside as authoring, not delivery, only when it adds the brief's own file and touches nothing but stream board READMEs, brief files and changelog fragments. A PR that touches any other path, including a document delivered under `docs/streams/`, still refuses, and so does one that does not add the brief's file. A file list that cannot be read or proven complete holds the dispatch as could-not-check.
- `deskdispatch`'s phantom check now tells the operator a MERGED representing PR already
  delivered the brief and points any follow-up fix at a fresh `Issue:` claim key
  (`<repo>--issue-<N>`) instead of reusing stale "resume the PR" wording that has nothing
  to resume.
- `deskgit fetch` now gates on origin's fetch URL as git itself resolves it (`git remote get-url --all origin`), including worktree- and global-scope values, empty-value list resets and insteadOf rewrites of the url list. The earlier read of the repository config file alone could pass an allowed repo while git fetched from another one. `deskgit push` decides its repo on the same read, but that read is the fetch url list: where git pushes (`remote.origin.pushurl`, `pushInsteadOf`) is not part of it, and this change does not gate it. A multi-valued origin url list is refused (exit 5) before any fetch or push runs.
- `deskgit push/fetch --as` binds the token to every origin URL git resolves for the verb, not only to the first `remote.origin.url`. It asks git itself (`git remote get-url [--push] --all origin`), so every `pushurl` value, every `url` value, and `insteadOf` / `pushInsteadOf` rewrites from every config scope are covered. If any destination names another repo, has a host other than exactly `github.com`, or is cleartext `http://`, the verb is refused before any mint (`deskkit.GitHubRoleTokenForDestinations`). Before this change, a `pushurl` on another host received the GitHub App token.
- `deskgit push/fetch --as` now binds the token where it is answered, too. The ephemeral `GIT_ASKPASS` script, which answered every prompt from every host, is replaced by one credential helper under the host-scoped key `credential.https://github.com.helper`, which itself answers only an `https` request for `github.com`. `push --as` also pins `--no-recurse-submodules`. Before this change, a submodule remote reached through push recursion (`push.recurseSubmodules`, `submodule.recurse`) and a user-bearing `http.proxy` each received the GitHub App token.
- `deskmerge` gates its fetch on git's resolved origin URL and its push on git's resolved push
  destinations, read in the scratch worktree the push leaves from; a multi-valued list or a
  destination naming another project is refused, in the dry run as in the real run.
- `deskpr create`/`update` now decide the repo and the push destination on what git itself
  resolves (`git remote get-url [--push] --all origin`: every config scope, `insteadOf` /
  `pushInsteadOf`, multi-valued lists) instead of a read of the repository config file alone.
  The push is refused (exit 5), before any token is minted, unless git's resolved push URL list
  is exactly one https URL naming the origin repo (or a local path). The refusal names each
  value, the config key, scope and file it came from, and a one-line worktree-scoped remedy — a
  disabled-push sentinel or a second, inherited `pushurl` value no longer fails late or pushes
  twice.
- `deskrelease` no longer hardcodes `/opt/desk-tools/bin/desktoken`; it
  resolves the co-located `desktoken` binary next to the running `deskrelease`
  binary, falling back to a `PATH` lookup only when no co-located binary is
  found.
- `deskwt role-init` now picks the role's credential by the forge that serves the repo: a GitLab-served repo reads the role's provisioned `gitlab-<role>.token` custody file instead of calling the GitHub App minter (which failed asking for `DESK_APP_ID`). A missing custody file is refused, naming the file, and never falls back to the GitHub minter. GitHub repos behave as before.
- `docs/glossary.md`'s `register` entry now lists all four registers
  (FINDINGS, INTAKE, REQUIREMENTS, DECISIONS), matching `spec/registers-v1.md`
  §1 — it previously omitted DECISIONS.
- `fanoutloop plan` no longer lists a `todo` brief as LANDED-UNRECONCILED because the merged docs-only PR that wrote it carried `Brief:`. `deskdispatch` already applied this check. The planner now reads the changed files of each PR naming a queued brief and ignores a PR that only authored it. A PR whose file list cannot be read still counts as delivering the brief.
- `file` is now in the combined desk-tools image, so the PE32-check Verify rows no longer
  exit 127 in the harness container.
- `scanloop run --dry-run` no longer advances the inbound poller's per-repo baselines. A live dry-run now polls a throwaway copy of the state dir, removed when the pass ends, so the preview still reports the real delta and the next real `run` still sees it. If the copy cannot be made, the pass is refused with exit 5 before anything is polled. It never falls back to the real state dir.
- `scripts/bootstrap-windows.ps1` no longer uses the PowerShell 7-only `? :` ternary operator
  in its `-Arch` default. Windows PowerShell 5.1 has no ternary, so the documented
  `powershell -File scripts/bootstrap-windows.ps1` install step could not parse the script;
  the default is now an `if`/`else` subexpression that both 5.1 and 7 accept.
- `scripts/build-windows.ps1` parses under Windows PowerShell 5.1 again: the seven em dashes
  added with the opt-in `-Sign` switch are now ASCII `--`. Without a BOM, 5.1 reads the file
  as Windows-1252, where an em dash's `0x94` byte is a string delimiter, so the one inside a
  `throw "..."` string broke the parse. `scripts/bootstrap-windows.ps1` and
  `scripts/windows-bootstrap-hashcheck-smoke.ps1` had the same em dashes and are ASCII now too.
- `statusgen --corroborate` no longer reads quoted notation as a claim that a human acted.
  Two kinds of line used to fail the check falsely. The first is a line on the removed (`-`)
  side of a hunk in a diff inside a committed `.patch` or `.diff` file; neither the stamp
  scan nor the citation scan reads it now. The second is `human:<name>` text in a test
  source file (a closed extension list plus a test-file name convention such as
  `x_test.go` or `x.test.sh`) or on a YAML `#` line; the stamp scan skips it. Record files,
  non-test programs and scripts, YAML value lines, other extensions, and the added side,
  context and pre-hunk preamble of an embedded patch are still scanned. Every skipped stamp
  or citation is listed in a `NOT-A-CLAIM` section of the run output.
- `statusgen verifyrun` no longer writes a human's forge login into the on-behalf-of annotation of a witness row on a public repo. The witness now follows the same visibility split as the desk write verbs: the roster's neutral name on any repo the roster does not state is `:private` (read from the brief's stream `repo:` frontmatter; unstated counts as public), the login only on a known-private one, and no annotation at all when no neutral name is configured.
- `tools/skillslint` gained an advisory `posix-token` NOTICE row (never
  exit-affecting) that flags a skill body spelling `mktemp`/`/tmp/`/`~/.config`
  literally outside a fenced "unix example" block, so the fix above does not
  silently regress.
- `windows-port/00` Verify rows 13, 14 and 15 are now self-proving on merged main. Before, each one diffed `HEAD` against `git merge-base origin/main HEAD`, which is `HEAD` itself once the brief has merged, so every run passed whether or not the property held. The rows are now pinned to the implementing PR's merge commit (branch head vs. its own base), their exit code is the verdict, and each fails on a planted defect. Row 15 no longer calls `statusgen --consumers`, which cannot corroborate this brief from its implementing diff (the brief file is not in that diff); it checks each `consumers:` routing claim against the pinned range directly.
- statusgen board-honesty: the `dehoused` phantom-class check now keys on the row's own
  board cell and the brief body, never the whole stream README. Previously a single row
  whose title/README merely mentioned "de-housed" (for example a brief *about* de-housing
  code) flagged every other `todo` row in that stream `NON-DISPATCHABLE (dehoused)` and
  dropped them from Next-up — the same false-positive class the `re-homed` check was already
  narrowed against (statusgen #709). No detector reads the stream README any more.

### Changed
- A MERGED PR found by the phantom check is refused as delivered (pointing at `--rework`) rather than with a "resume" hint; tests now pin that the phantom check precedes the admission gate, the token mint and the claim.
- Added `deskpushguard hook-install`, which writes the pre-push hook shim(s)
  into `.githooks/` resolving the guard binary from PATH instead of a
  hardcoded `/opt/desk-tools/bin` literal, and writes the `pre-push.cmd` pair
  on a Windows target. `make desk-hook-install` and
  `scripts/build-windows.ps1`'s `Target-DeskHookInstall` now call it.
- All three `--corroborate` lanes (stamps, citations, decision records) now read the diff
  through one walker. The guard test `TestCorroborateDiffWalkersShareOneWalker` fails if
  another function in the package walks the diff with its own loop.
- De-POSIXed the desk-role skill prose: `pr-review-desk`, `pr-shepherd`, and
  `worker-desk` now name the scratch-file and config-home MECHANISMS
  (`desk-shell.md` §Scratch files, §Config home) instead of spelling
  `mktemp`/`/tmp/`/`~/.config` literally, so a dispatched session on a
  non-POSIX shell has a mechanism to follow rather than a command to improvise.
  `deskboard` gained an `--out <path>` flag (a portable substitute for a shell
  `>` redirect) and `reviewloop plan` accepts a no-op `--dry-run` flag.
- Known residual, to close before the lane gains a merge write: the forge's path history is simplified, so a merge that restores an old register can hide the text change it reverts. An older acceptance revived that way, with no later acceptance on the thread, is not refused.
- New `plugins/assay/references/desk-common.md` states, once, the procedure the desk-role skills had each repeated word for word: the liveness contract, worktree hygiene, and the driver-act runsheet entry. `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk` and `verify-desk` now point to it under the same headings, so in-body `§` references still resolve. A sentence that belongs to only one desk stays in that desk's body. Hard gates stay in each body, and the generated guardrail blocks are unchanged.
- New follow-up brief `docs/streams/windows-port/brief-15-inbox-html-flow-port.md` carries
  the `--html` and `--flow` renderings split off from windows-port/13; windows-port/14 now
  depends on windows-port/15 (not /13) for the `deskinbox flow` step its Windows leg runs.
- On GitLab, a typed comment read (`ListCommentsTyped`) of a thread that still has more notes after the page cap is now could-not-check instead of returning the oldest 2,500 notes as the whole thread.
- The GitLab fleet role table (role, scopes, and the naming of usernames, PATs and token files) now lives in `tools/fleet-gitlab-roles.sh`, which both `tools/create-fleet-gitlab.sh` and the new renewal script source. It is no longer copied inline in the provisioner. The shared `FLEET_PAT_DAYS=7` default there is now the single source for both scripts' PAT lifetime, matching spec.md §5's 7-day expiry backstop: the renewal script's `--duration` default moved from 30d to `${FLEET_PAT_DAYS}d`.
- The Windows adoption docs now lock the **folder** that holds the roster and role tokens with an inheritable owner-only ACL (`icacls <dir> /inheritance:r /grant:r "%USERNAME%:(OI)(CI)F"`), then each existing file. A lock on the file alone does not last: the desk tools re-create credential files (GitLab rotation, first GitHub App mint), and a new file takes its folder's inheritable ACL.
- The `author-brief` skill gains rule 14: a fix/bug brief carries an optional `regression-of:`
  frontmatter key (the earlier fix's issue reference or commit sha, when one exists) and a
  mandatory fail-first class-guard Verify row tagged `+mutation`. `regression-of:` is
  tolerated as an unrecognised key today; no lint validates it yet.
- The `verified → done` close also refuses when the Evidence records a `VERIFY: FAIL` that no later strict `**VERIFY: PASS**` marker answers. A prose mention of a pass ("will record VERIFY: PASS once green") does not answer a FAIL.
- The forge seam gains two reads for the time check, on both backends: `ListFileCommits` (a file's commit history at a ref) and `ListCommitChanges` (the changes behind a commit).
- The four desk skills (`worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk`) carry the
  blocker-evidence rule as a sibling of the idle HARD GATE and a correction-capture clause
  under the receipt rule; `worker-desk` holds the one definition, the other three point at it.
- The install prerequisite is now two distinct principals with a per-forge mechanism table (GitHub Apps / GitLab service accounts), and the CORE primitives are stated per forge; `docs/adopting-assay.md` and `docs/adopting-assay-gitlab.md` read as the two halves of one install flow.
- The portability audit gains the rows its first pass missed: `pr-monitor.sh`, `tick-summary.sh`,
  the harness Monitor invocation path, and the `jq` prerequisite.
- The shipped `cellctl` provider catalog seed (`providers.json`, created by `cellctl providers init`) now pins the Anthropic `strong` tier (the Opus alias `pr-review-desk` and `verify-desk` run on) to `claude-opus-5-5[1m]` instead of `claude-opus-4-8[1m]`. An existing `$CELLS_ROOT/providers.json` is never overwritten; operators edit theirs by hand.
- The verify-desk skill says how to clear a hold before a flip: run the row and strike the earlier line through, or route it to a named follow-up.
- The worker dispatch kits (`worker` and `worker-objective`) gain a bug-fix clause: a fix names
  its defect CLASS in a `## Defect class` PR section, adds a guard that fails if any other site
  repeats the defect (modelled on an allow-list structural test over every caller of a
  hazardous primitive), and shows that guard red against a deliberately planted second
  instance. A test of the reported instance alone no longer discharges a fix. The
  `worker-desk` skill states the same obligation.
- Windows token custody now matches the Unix `0600` rule: on Windows, a credential file must be readable and writable only by its owner (plus SYSTEM / Administrators). `VerifyCustodyOwnerOnly` refuses a token file whose ACL grants any other principal read access — including a group such as Everyone, Authenticated Users or Users, and a grant inherited from the parent folder, which the refusal names — in addition to the write access it already refused. An unreadable security descriptor or an entry it cannot interpret still refuses. The roster owner check and the Unix path are unchanged (#1604).
- With `ASSAY_CLAIM_STORE` unset, claims stay on the forge exactly as before. Every `deskclaim-ref` and `deskdispatch` run now prints a NOTICE that this default will be removed, and names the release it goes in. Setting `forge-ref` explicitly is refused. The `claim-acquire OK` line names the store (`store forge-ref (legacy)`). When `deskclaim-ref` cannot open the forge store, its stderr line now reads `unverifiable: claim store for <repo>: the forge-ref store cannot be opened: <cause>` (previously `unverifiable: <cause>`); the exit code is unchanged.
- `assay:install` no longer needs `gh` (or `glab`): the pinned statusgen and desk-tools binaries are fetched over plain HTTPS and sha256-verified against `.assay-versions`, with a mismatch **or an unreadable digest** refusing the install. The download is HTTPS-only on the initial URL and on every redirect hop — a non-HTTPS URL or redirect is refused with nothing written — and the pinned sha256 remains the integrity check. The new `plugins/assay/scripts/assay-install.sh` (`classify` / `pin` / `acquire` / `rehearse`) is the executable form of the flow, and `rehearse` runs it end to end against a scratch copy of the target.
- `desk-build` now removes-then-writes each `dist\*.exe` so a sign → rebuild → re-sign loop works
  where recent Go's `go build -o <existing.exe>` refuses to overwrite a signed PE.
- `deskautolane`'s App-token condition mints through `deskkit.GitHubRoleToken`, the forge-aware
  GitHub arm, instead of calling the raw App minter. The token, its scope and its custody file
  are unchanged; a repo the roster binds to another forge is now refused before any mint.
- `deskautolane`'s enactment gate narrows toward the ruling's text, and every change only narrows what enacts the lane. The acceptance comment must sit on the one sign-off thread named by the new optional roster key `ASSAY_AUTOAPPROVE_SIGNOFF_THREAD`, which must be an issue; a comment on any other thread, or a pull-request permalink, is refused, and an unset thread is could-not-check. The comment must be created after the latest change to R-8's text above its Sign-off line that the register's path history records. That time is the merging PR's `merged_at`, never a commit date. A Sign-off-only change does not move it, and a recorded text change with no merged PR behind it is refused. The comment must also be the blessing authority's newest acceptance on the sign-off thread, read whole. `Enact: R-8` must be the body's first non-empty line, typed bare from the first column, which is stricter than a ruling text that ignores leading whitespace.
- `deskpr create` also refuses the mirror case: an `Authors:` line on a branch whose diff is NOT provably authoring-only for every listed id (including a rename, which the writer-side gate cannot classify and so refuses rather than trusts). `deskflip`'s security lane carries the binding half of the same check (`deskkit.AuthorsRiskFromBody`), so an `Authors:` PR that actually delivers code or a document for a `gate: human` / `risk: yes` brief is still risk-classed at flip time even if the writer-side gate was bypassed or predates it.
- `deskpr create` refuses a `Brief:` line on a branch that only authors that brief: the branch adds the brief's file and touches only stream board READMEs, brief files and changelog fragments. The refusal names the `Authors:` line to use. A PR that authors a brief and also delivers work keeps `Brief:`.
- `deskwt add --role <role>` now gives the new worktree the role App's own transport instead of the one it inherits from the shared checkout (an SSH origin, an operator's `pushurl` sentinel): at worktree scope it resets `remote.origin.pushurl` and `remote.origin.url` with an empty entry and sets both to `https://<host>:443/<owner>/<name>.git`, wires the role's host-scoped App credential helper, resolves an SSH host alias with `ssh -G`, and refuses (exit 5, worktree rolled back) unless git then resolves exactly that URL for fetch and push. `deskdispatch` passes the dispatched agent's role, so every dispatched worktree gets it. The shared checkout's config is never touched.
- `docs/streams/windows-port/brief-13-inbox-verb-port.md` narrowed to the table+walk scope
  actually delivered here, and corrected two factual errors found at pickup: the oracle
  script invokes `make` zero times (not four, as the brief's earlier draft claimed), and
  the skill body that names the script lives at `plugins/assay/commands/inbox.md`, not the
  nonexistent `plugins/assay/skills/inbox/SKILL.md` the earlier draft cited.
- `pr-review-desk`: the generated-table bounce now admits a second, narrow carve-out — a
  cross-repo brief's existing row, promoted Status-only from `todo`/`in-progress` to
  `implemented`, admitted only when `statusgen reconcile --backfill --apply --repo <delivery repo>`
  run on main reproduces it byte-identically. The delivery repo is read from the brief
  (`homed-in:`, else the stream's `repo:`, else the board repo), never from the PR, and must
  differ from the board repo: a same-repo row still bounces. Every admitted row, whether its
  witness is a `Brief:`-trailer PR or a backfill branch/body match, also needs the reviewer's
  code-existence check in the delivery repo, recorded in the verdict. Every other change inside
  the generated table still bounces.
- `scanloop run` arms the `deskmonitor inbound` verb from PATH instead of running
  `inbound-monitor.sh` through a hard-coded `/bin/bash`. `--monitor <path.sh>` /
  `ASSAY_INBOUND_MONITOR` still arm the script, as an explicit parity mode through a `bash` found on
  PATH. The default monitor state dir now follows the OS temp dir (`%TMP%` on Windows).
- `statusgen --close-verify` now runs the HELD/could-not-check read on the `verified → done` close path as well as on `implemented → done`. On the `verified` path the read is keyed on the row's `verified` status, so it runs whether the pass is written as the strict `**VERIFY: PASS**` marker, in a looser form, or not at all. A brief flipped to `verified` over an un-routed HELD/could-not-check line can no longer be closed to `done` while that line stands. A hold that a later run resolved still refuses until the earlier line is struck through (`~~…~~`) or routed to a follow-up: supersession is not inferred.
- `statusgen --corroborate` now gates every decision record a pull request adds or edits. A `decided-by: "human:<name>"` placeholder with no resolvable `ruling:` link is MISSING-CORROBORATION (`placeholder-unratified`). Previously the placeholder was never read. Records already merged are not re-checked until a pull request edits them.
- deskflip's checks-green conclusion set now delegates to the shared `deskkit.ConclusionGreen`,
  the same set the lane's `ci-nonsuccess` signal reads. Behaviour is unchanged.
- windows-port/09: keep Git-Bash/WSL as one clearly labelled GitLab provisioning fallback (never a prerequisite), and carve the Manual appendix's numbered steps out of Verify row 9's expectation.

## v1.0.26 — 2026-09-23

### Added
- New brief harness-portability/16, Codex long-context cap. It plans `model_auto_compact_token_limit`
  for every `cellctl` Codex desk launch, the same key in the Codex packaging with a lint that fails
  without it, a compact-or-reboot point for standing desks, and a `deskdispatch` warning for
  oversized prompts. The goal is to keep long Codex desk sessions from crossing the 272K-token
  long-context pricing band without anyone noticing.
- `deskcalibrate`: a monthly reviewer-calibration verb. `deskcalibrate sample` draws a
  reproducible, seed-recorded sample of the PRs the reviewer App APPROVED in the prior
  month and REFUSES a re-review whose model vendor equals the reviewer role's own
  (`ASSAY_REVIEWER_VENDOR`) — a same-vendor re-review measures two instances of one model,
  not an independent judge. `deskcalibrate report` renders the agreement metric as an
  explicit numerator/denominator FRACTION, never a bare percentage (verify-integrity/10).
- `deskkit`: a typed decision-assessment envelope (`AssessmentRequest`/`Prediction`/
  `PolicyResult`, `spec/decision-assessment-v1.md`, `schemas/decision-assessment-v1.json`)
  layered on top of the existing `Decide`/`Advice` consult, keeping calibrated
  probabilistic advice strictly separate from a deterministic policy record.
  `ValidatePrediction` rejects unknown labels, NaN/Inf or out-of-range probabilities,
  invalid normalization, mismatched subject/digests, an uncalibrated label carrying a
  synthesized probability, and inapplicable calibration. `PredictionAdvisor` projects a
  validated `Prediction` into the existing `Advice` contract with zero change to
  `Decide`'s fail-closed default, budget, timeout, journal or reserved-verb rules
  (graph-execution/10).
- `deskread` serves two per-issue read kinds, `trust` (`Forge.IssueTrustEvents`) and `comments` (`Forge.ListCommentsTyped` on an issue), addressed as `--issue owner/name#N`. Their envelope is keyed by repo and number and keeps the same partial-is-a-result contract as the `issues` kind. The `issues` kind's output is unchanged.
- `statusgen`: per-finding-class **reversal-rate** mining joined to the gate-yield
  accounting, with the two-month demotion rule — a class whose reversal rate exceeds 50%
  for two consecutive months is marked advisory; a later month under 50% restores it.

### Fixed
- `deskdispatch`'s decision gate now runs the consumer `tools/decision-issue.sh` under the dispatching role's credential (`GH_TOKEN`, plus `GITLAB_TOKEN` on a GitLab-served repo, in the script's environment) from the same single resolution the claim step uses, instead of whatever forge login was ambient. The credential is resolved before the claim, so a mint failure stops the dispatch with nothing durable taken, and the step refuses (exit 6) rather than start the script with no credential handed over and none exported. The `decision-gate OK` line names the credential source.
- `docs/streams/desk-containers/brief-01-base-image.md`'s Verify row 1 build command now
  uses the working root-context form (`docker build -f containers/base/Dockerfile -t
  assay-desk-base:dev .`, run from the repo root) instead of the broken
  `containers/base`-context form, which failed because the Dockerfile `COPY`s
  `plugins/assay/` from the context root. `containers/README.md` also gains a note that a
  plain (non-buildx) arm64-host build needs `--build-arg TARGETARCH=arm64`.
- `inbound-monitor.sh` takes an explicit read identity, `--token-file OWNER=PATH` (a 0600 installation-token file, read in place and never copied), which outranks its `gh` keyring fallback. Under a replaced `HOME` such as a desk cell's, that fallback resolves to no usable account and 401s every repo. `scanloop run` now hands the poller the running role's already-minted token file for each owner in scope. Owners without one keep the keyring path exactly as before, and each fallback is printed with its reason. An unusable token file is a precondition failure, never a silent fallback to the keyring.
- `statusgen --scan-issues` — the scan `scanloop run` shells in its scan lane — no longer shells out to `gh` for any forge read. The trust-gate blessing read (was `gh api graphql`) and the un-block comment read (was `gh api --paginate`) now go through the `deskread` verb on the native `Forge` seam, completing what #1223 started for the open-issue list. Under the replaced `HOME` a scanloop pass runs with, those two reads returned `gh: HTTP 401` on every rostered repo; the native client attaches the per-installation App token explicitly on every request. (#1255)

### Changed
- Roster schema: `ASSAY_REVIEWER_VENDOR` / `ASSAY_VERIFIER_VENDOR` are recognised keys in
  both `statusgen` and the desk tools, so a roster carrying them no longer collapses the
  configuration on the unknown-key refusal.
- The GitHub backend's issue-thread read (`ListCommentsTyped` on an issue) now follows the comment connection's cursor to the end of the thread, capped at 20 pages. A thread longer than the cap, or a thread that reports another page without giving a cursor for it, is now could-not-check, where before it was cut to its first 100 comments without any warning. Reads of pull-request comment threads are unchanged.
- `pr-review-desk` skill: a finding-class register with a `blocking`/`advisory` status and
  the reversal-rate demotion rule wired to the monthly calibration report.

## v1.0.25 — 2026-09-23

### Added
- Release by merge (design + staged implementation): a prepared release PR (title
  `release: vX.Y.Z` + a `RELEASE: vX.Y.Z` body marker) becomes the release cut when a
  maintainer MERGES it — the merge is the human gate and the recorded authorizer. A staged
  `release-on-merge.yml` detects the merge and creates the plain `vX.Y.Z` and umbrella
  `assay/vX.Y.Z` tags at the merge commit with a GitHub App token (a GITHUB_TOKEN-created tag
  would not trigger the build), and a staged twin of `release.yml` resolves the tag-push
  authorizer from the merged PR's `merged_by.login` and refuses to release any `v*` tag whose
  commit is not a merged release PR — closing the iso-9001/04 tag-push traceability gap.

### Fixed
- A dispatched agent's worktree no longer INHERITS the shared checkout's git commit identity.
  `deskdispatch`'s worktree-create step now stamps the DISPATCHED agent's own role commit
  identity (worker/reviewer/verifier, mapped from `--kit`) into the new worktree's own
  worktree-scoped config, so a verifier dispatched from a desk checkout commits — and reports
  its runner — under the verifier App, not the desk App. Before this, the worktree carried
  whatever `user.name`/`user.email` the shared `.git/config` held, and `statusgen verifyrun`
  stamped that wrong identity into every Evidence witness Runner cell (silent misattribution).
  A `--kit` whose role has no roster commit identity is now REFUSED pre-claim (exit 5) naming
  the kit, the role and the roster key, and the worktree-create OK line prints
  `identity=<slug> <bot-user-id>`.
- `cellctl`'s the-desk model policy now gates Opus by a VERSION FLOOR instead of a fixed
  allowlist: the coordinator ACCEPTS any Opus tier at or above **5.5** (`claude-opus-5-5`,
  `claude-opus-5-6`, `claude-opus-6`, and a future `claude-opus-9`, including `[1m]` and
  `-`/`.` spellings) and REFUSES anything below it (the bare `opus` alias, Opus 5.0 —
  `claude-opus-5` / `-5-0` / `-5.0` — and older tiers such as Opus 4.8). A future Opus tier
  therefore auto-qualifies as the-desk's top tier with no code edit — a deliberate, documented
  trade (a floor adopts a future Opus sight-unseen; the "Opus 5.0 was a bad tier despite its
  number" lesson makes that a choice, reversible by raising the floor). The built-in deny
  patterns stay anchored at end-of-token (`*opus-5` / `*opus5` / `*opus-5-0` / `*opus-5.0`), and
  the "which Opus tiers the-desk refuses" decision lives in one shared helper (`isOpusPin`) that
  both the legacy resolver and the policy resolver consult, so the carve-out cannot fork between
  sites.
- `deskclose superseded` on a PULL REQUEST no longer reports a close it did not perform. The
  close is now **read back** after the call — the item is re-fetched and its state confirmed
  `closed` before success is printed — so a state-change request that returns without error but
  leaves the item open (an issue-shaped `state_reason` PATCH on a pull request's number returns
  `422`, which was swallowed after the confirmation comment posted) is caught. When the comment
  posted but the close did not take, the run reports `partial: comment posted, close refused: …`
  with the forge's own error body and exits `6` (could-not-check), never success. The read-back
  lives in the shared close path, so every permanent-close lane (`superseded`, `duplicate`,
  `review-request`, `triage`, `self-withdraw`, manifest rows) gets it; the deliberately transient
  `verify-gate-refire` close, which the repository's verify-gate-close workflow reopens, opts out.
- `deskclose` flag parsing now accepts the single-dash spelling of a long value flag (e.g.
  `-by`, which Go's `flag` package treats identically to `--by`) in every argument order. The
  positional splitter previously recognised only the double-dash spelling, so `superseded <pr>
  -R … -by …` tripped `flag needs an argument: -by` — it dropped the value that was present and
  mis-read it as a second item number — a failure that looked "environmental" because it
  depended only on the dash spelling typed.
- `deskpost`'s model-capability-floor stamp age-out no longer refuses (verdict) or misreads (flip)
  every risk-classed review on a reviewed PR. The age-out now keys on the REVIEWER's
  review-dispatch claim family (`refs/dispatch/<short>--pr-<N>[--<suffix>]`) in the namespace
  claims actually live in — not the PR body's worker `Brief:` claim (released the moment the
  worker finishes) and not the empty `refs/heads/dispatch/*` namespace the old reader probed. A new
  `Forge.MatchingRefs` op does the prefix listing (GitHub `git/matching-refs`; GitLab CE is a
  could-not-check the review lane never reaches). Reader-side only — the acquire/writer namespace is
  untouched, and full reader+writer convergence remains the issue-708 follow-up.

### Changed
- `check-paired-versions.sh`'s single-tag assertion stays as it is on `main` (no harness
  exemption) per the maintainer's ruling (option 1): re-pin `statusgen`/`desk-tools` to a real
  `v1.0.24` release instead of carving the harness image out of the rule.
- `deskwt add` now takes `--role R` and, given it, stamps that desk role's App commit identity
  into the new worktree (the same shared resolver `role-init` uses); an unbound role is refused
  (exit 5) before the worktree is created. WITHOUT `--role`, `deskwt add` now CLEARS the new
  worktree's `user.name`/`user.email` (an empty worktree-scoped value that shadows the shared
  config), so a bare `deskwt add` worktree can never silently commit under an inherited identity
  — a commit there fails closed until an identity is set. The commit-identity resolver
  (GitHub/GitLab shape choice + fail-closed refusals) is extracted to one shared helper,
  `deskkit.RoleWorktreeCommitIdentity`, so `role-init`, `deskwt add --role` and `deskdispatch`
  cannot resolve one role to three identities.
- `statusgen:`/`desk-tools:` re-pinned to the published `v1.0.24` release — tag and every
  per-platform sha256 harvested from that release's `checksums.txt` — so all three
  `paired-versions.yaml` sections share one tag again and `check-paired-versions.sh` passes
  unchanged.
- harness: pin the v1.0.24 desk-tools image digest (`plugins/assay/paired-versions.yaml`
  `harness.tag`/`harness.digest`), replacing the fail-closed `PENDING-HARVEST` placeholder now that
  the image has been published and its digest harvested from two independent registry reads.

## v1.0.24 — 2026-09-22

### Added
- A `verify-in-container` Windows CI leg (staged, pending maintainer promotion) proves the execution
  witness actually lands through the container; a held `windows-verify-in-container` job records the
  native-Windows-host proof as BLOCKED pending a Windows runner with a Linux-container Docker backend.
- Add the portable `assay:system-demo` skill for outcome-led storyboards and seekable system demonstrations, with explicit evidence labels and an illustrative authoring example.
- Typed forge seam op `ListChanges(repo, states)` — reads a repo's changes (PRs ↔ MRs) in the requested lifecycle states, keeping `MERGED` distinct from `CLOSED`, with bounded most-recently-updated pagination that reports `Incomplete` rather than hand back a silent partial. GitHub (GraphQL, states variable) and GitLab (`state=all` narrowed client-side) backends, both under the closed forge surface (no forge CLI).
- `ASSAY_GITLAB_DISPLAY_NAMES` roster key (`<username>=<display name>`; entries separated by `;` or newline) — the offline fallback that lets statusgen's Evidence-actor gate accept a GitLab verifier's Evidence commit. statusgen consumes it; the desk tools recognise it and resolve the username online instead.
- `CONTRIBUTING.md` gains a "What must not appear in an issue or pull request" section
  covering secrets and tokens, personal data, private references, unsanitised
  transcripts/logs/screenshots, and anything captured from a system the reader cannot see. It
  binds a human contributor and an AI agent acting for one alike, and states the
  close-the-PR-and-recut fix path for a leak already pushed.
- `deskreconcile` — a desk-side board-reconcile writer. It fetches `origin/main`
  into an isolated worktree, runs `statusgen reconcile --backfill --apply` (the
  only writer of a stream README Status cell: `todo`/`in-progress` →
  `implemented`, real merged-PR witness only), and — only when a stream README
  changed — commits ONLY those README files as ONE commit on the fixed branch
  `board/reconcile` and opens or UPDATES exactly one draft PR titled
  `chore(board): reconcile`. `--dry-run` reports the rows it would flip and writes
  nothing. This runs the scheduled-reconcile job from a verb the desk/worker App
  can run, removing the CI-workflow dependency that #1175 is blocked on (no App
  may push the workflow change). (#1339)
- `statusgen newbrief` gains a `--shell` flag (`sh` default / `cmd` / `pwsh`) so a
  native-Windows Verify row can carry its shell marker at authoring time. It
  **refuses** a native-Windows `--verify-command` (e.g. `findstr …`) under the
  default `sh` shell — pointing the author at POSIX-izing it (`grep -F`,
  forward-slash paths) or declaring the shell — and, when `--shell cmd`/`pwsh` is
  given, emits the row with its `| # | Shell | Command | Expect |` marker attached
  in the same pass. This enforces at the brief-authoring front door the rule the
  author-brief guidance already states: a Windows-shaped row must never land as the
  default `bash -o pipefail` row with no marker, since an Evidence-only PR cannot
  rewrite the Verify table to fix it. The default POSIX row keeps its
  Shell-column-less shape byte-for-byte. (#1466, #1424)
- `statusgen verifyrun --in-container` runs a brief's Verify rows inside the pinned harness
  container instead of on the host — the supported execution-witness runner on Windows, where a
  native `pipefail` bash is unreliable (the WSL-launcher case that made verifyrun record
  could-not-run for a whole table). It reads the harness image from the new `harness:` block of
  `plugins/assay/paired-versions.yaml`, resolves it by its sha256 **digest** (never a floating
  `latest`), bind-mounts the checkout at `/work`, maps `--user` to the host uid:gid on POSIX so the
  Evidence the container writes lands host-owned, and forwards credentials only as an `--env-file`
  path (never baked or logged). It **refuses fail-closed** on a `latest` tag or an absent/placeholder
  digest — an un-digest-pinned image is never run.
- `system-demo` skill: two player rules from first real use — a fixed stage (constant-size aspect-ratio box, fixed-height caption and evidence-label bands, controls at a constant position, so navigating scenes never moves the controls or reflows the page) and a distinct stage surface (backdrop contrasting with the host page in the host's own design tokens, so the player reads as an embedded presentation viewport). Both added to the "Deliver and check" checks and mirrored in `references/storyboard.md`.
- windows-port briefs 11–14: the desk-role runtime paths a native-Windows adopter still cannot run (the inbound and PR pollers, the tick emitter, the inbox engine, the push-guard hook and the POSIX wording in the desk skills) are now scoped as Go verbs with parity oracles and a PATH-scrubbed Windows CI job that proves them.

### Fixed
- Added the `system-demo` skill to the Codex and Cursor packaging coverage rosters
  (`plugins/assay/codex/packaging.md`, `plugins/assay/cursor/packaging.md`) as
  `packaged`, plus its degradation cell in both harness binding files
  (`plugins/assay/references/codex.md`, `plugins/assay/references/cursor.md`) —
  `harnessgen codex --check` / `harnessgen cursor --check` were failing with an
  unaccounted-for-skill coverage error.
- Evidence-actor now accepts a roster-known GitLab HUMAN verifier via GitLab's private commit noreply address (`<user-id>-<username>@users.noreply.<host>`, id-pinned) the way it already accepted the GitHub noreply form.
- Evidence-actor on GitLab: a verifier service account whose commit carries the account's DISPLAY name (not its username) can now back a `verified`/`done` row, unblocking `implemented → verified` on GitLab (#1477). Two accepting paths: `deskevidence` resolves the landed commit's account to its username ONLINE via the typed forge (`GET /users?search=`, never a forge CLI); `statusgen --lint` accepts the commit OFFLINE when the roster declares the account's display name in the new `ASSAY_GITLAB_DISPLAY_NAMES` map. A forge read that cannot resolve the account is could-not-check, never a pass and never a rejection.
- Include Bash in the combined desk-tools image and trust its explicit `/work`
  checkout mount for Git attribution while retaining the nonroot runtime user.
- Origin-remote parsing is now one shared parser (`deskkit.ParseRemoteRepo` / `OriginRepoSlug`) instead of three copy-pasted `parseRepo` functions plus a fourth regex in preflight. The parser accepts every remote shape git accepts — `git@host:owner/repo`, `ssh://git@host[:port]/owner/repo`, `https://host/owner/repo`, and the ssh HOST-ALIAS form `host:owner/repo` — and reads the RAW configured value (no `insteadOf` expansion), so a checkout using an ssh alias can be worktree-created and have PRs opened against it. It also parses the rewritten/hybrid form `https://host/git@alias:owner/repo` leniently to `owner/repo` (the shape a prepended base URL bakes onto an scp string), rather than failing with the misleading "cannot parse owner/repo" that upstream reported as "branch already exists". A hybrid whose trailing pair is genuinely ambiguous is refused with a message naming the remote string and the expected shape.
- Reformatted `tools/desk/cmd/deskdispatch/phantom_test.go` with `gofmt` (comment-alignment whitespace only) so package-wide `gofmt -l` checks used as Verify rows elsewhere stop failing on this pre-existing, unrelated drift.
- Refuse new verified outcome receipts until the target branch contains the
  verified/done row, dated verifier stamp, and passing execution witnesses, and
  the same checkout passes lint. Evidence-only landings retain implemented status
  without recording a completed verification; failure receipts remain available.
- Require explicit shell selection when authoring native Windows Verify commands;
  prefer POSIX commands and forward-slash paths for portable checks.
- The combined `desk-tools` CI-runner image now installs `bash`, so `statusgen verifyrun`'s `bash -o pipefail` Verify rows actually execute when `--in-container` re-invokes verifyrun inside the image. Previously the Alpine final stage shipped only git/gh/ca-certificates and every POSIX row recorded could-not-run for lack of a pipefail-capable shell.
- `deskdispatch`'s pre-claim phantom check now goes live: its represented-PR transport reads the repo's open+merged changes through the new typed seam op and refuses a fresh worker dispatch whose brief already has an open or merged PR (matched on the PR body's `Brief:` trailer, not a branch name) (#1339).
- `deskdispatch`'s worktree-create failure hint no longer tells the operator to hunt for a
  merged/open PR when `deskwt add` actually failed on its own origin-remote resolution
  (`cannot parse origin repo …` / `cannot parse owner/repo …`) — that class now gets its own
  hint pointing at the checkout's origin remote, and a message matching neither the
  branch-exists nor the origin-parse pattern now gets no guessed cause at all instead of the
  old unconditional "branch already existing" fallback.
- `deskdispatch`'s worktree-create failure message now routes through the same
  scrub pass as every other diagnostic, and `ToolRun.FailVerbatim` scrubs the
  message it is given before it becomes part of the error. A caller that
  composes its own `FailVerbatim` message from a child process's raw stderr can
  no longer let a credential-shaped string on that stderr reach the operator
  unredacted, on any `DESK_TRACE` setting. (#1440)
- `deskevidence` refuses to land an `"outcome":"verified"` row on
  `docs/streams/verify-outcomes.jsonl` unless the landing tree presents a
  lint-valid `verified` closure for that brief — the Verified stamp AND a passing
  execution witness for every Verify row. Previously the sidecar recorded
  `verified` on a PASS run unconditionally, so a brief whose Evidence was filled
  while its board stayed `implemented` (no flip, or no witness) still got a
  `verified` row; `verifyloop` then bucketed the mismatch as a stuck-flip (#1309)
  and review refused to merge it. The acceptance decision lives in a new
  read-only `statusgen verifyclosure --brief <stream>/<NN> [--root <dir>]`
  sub-command that reuses the board's own Status/Verified read and the existing
  witness audit (`checkWitnesses`), so the criteria are defined in one place. The
  gate is scoped to `verified`: a `verify-fail` row (and every other outcome) is
  never gated, and a brief the check could not evaluate refuses the landing as
  could-not-check rather than passing silently.
- `fanoutloop plan` now reconciles every fresh Next-up row against the repo's open+merged PRs and
  routes by state instead of blindly offering the row (#1339): a row whose brief already MERGED is
  listed under a new `LANDED-UNRECONCILED` heading (with its PR number) and never dispatched — its
  board cell just never reconciled after the merge — a row with an OPEN PR is routed to the resume
  lane, and only unrepresented rows are dispatched. The match is keyed on each PR's `Brief:`
  trailer, never a branch name. A could-not-check read (or an unresolvable repo) HOLDS the fresh
  lane with a `FRESH LANE HELD:` line rather than offering rows on an unverified forge. `plan` takes
  an optional `--repo <owner/name>` (defaults to the configured-roots map for `--root`, else the
  checkout's origin remote). The PR-list transport is a typed forge op deferred to the cutover — the
  closed forge surface ships no forge-CLI call — so until it is wired the shipped `plan` performs no
  forge read; the classification, repo resolution and one-read reduction are in place.
- `fanoutloop plan`'s already-represented reconciliation now goes live too: its `representedPRs` transport is wired to the same typed seam read, so `plan` routes a fresh row whose brief already has an open PR to resume and a merged one to landed-unreconciled instead of offering it for fresh dispatch (#1339).
- `issueboard`'s escalation clock no longer fails the WHOLE board when one
  decision-owed issue's comment thread exceeds a single page. The clock now reads
  the whole thread through a new bounded, paginated forge read
  (`Forge.IssueContentEvents`) instead of sharing the trust gate's deliberately
  single-page read, which had returned could-not-check (exit 6) — and took down
  the board for every scanned repo — the moment one thread overflowed 100
  comments. A thread that even bounded pagination cannot walk to the end degrades
  that ONE row conservatively (rendered ESCALATE with a could-not-check marker)
  while the rest of the board renders; an overflowed thread is never read as "no
  escalation owed". (#2844)
- `statusgen verifyrun --in-container` now marks exactly the bind-mounted `/work` tree safe for the inner git (`safe.directory=/work` via ephemeral `GIT_CONFIG_*` env), so attribution no longer fails on a Windows Docker backend where the mount is root-owned under the unprivileged container user. The trust is scoped to `/work` only — never a global `safe.directory=*`.
- `statusgen verifyrun` now runs a `cmd`-shell Verify row under a raw Windows
  command line built as `cmd /d /s /c "<row>"`, instead of letting `os/exec`
  escape each argument. The default escaping wrapped the row in an extra quote
  pair and backslash-escaped the row's own inner quotes; `cmd /s /c` strips only
  the outer pair, so a native-Windows row such as
  `findstr /c:"…" docs\…` reached `findstr` with broken quoting and exited 1
  under verifyrun even though the identical line passes when typed at a prompt.
  `sh` and `pwsh` rows are unchanged. (#1424)
- `statusgen`'s issue scanner no longer silently drops or retypes a GitHub
  label whose text is a YAML type keyword or number (`null`, `~`, `true`,
  `false`, `yes`, `no`, `on`, `off`, `123`, `1.5`) when it writes the
  `labels: [...]` flow list into a generated placeholder's frontmatter. Each
  label is now explicitly string-tagged on write, so it always re-parses back
  as the original string instead of being resolved to `null` (dropped), a
  boolean, or a number. (#1431)
- `verifyPassHeldContradiction` (the `**VERIFY: PASS**`/`HELD` contradiction check
  introduced in PR #1304) no longer launders a same-line, un-routed
  `HELD`/`could-not-check` mention that trails a genuinely routed one. Routing is now
  bound to each `HELD`/`could-not-check` occurrence individually — a routing keyword
  and reference must occur at or after that occurrence's own position — instead of to
  the line as a whole, closing the same proximity-laundering class PR #1244 closed in
  the predecessor `entryIsHeld` mechanism. (Issue: #1444)
- harness-portability brief 12 (Cursor third column): retargeted the stale Verify
  probe (row 8 + its positive control) at the file where the de-house actually
  landed the adopter-facing Cursor install scenario — `docs/adopting-assay.md`
  ("Running Assay on Cursor") — after `adopt/SKILL.md` became a thin router, so the
  brief's Verify table again runs clean end-to-end against the public tree.

### Changed
- Claude cell launches and container images disable next-prompt suggestions by default, including scrubbed cells and provider-backed sessions.
- The Evidence-actor rejection for a GitLab service-account commit whose name matches neither the username nor a declared display name now names the display-name-vs-username gap and its remedy, instead of reading as a wrong-account tamper signal.
- The model-capability floor is now **RISK-CONDITIONAL on an unstamped PR** for a review
  verdict. An unstamped PR (no dispatch stamp, a `dispatched-tier:any` stamp, or a stamp
  that has aged out) still proceeds with a NOTICE on a NON-risk PR, exactly as before — a
  human-driven or unattested lane is not bricked. But a review verdict is a
  security-review-bearing write, so on a **risk-classed** PR (every public-repo PR, or a diff
  touching a security path) an unstamped verdict now **REFUSES**: a security-review-bearing
  verdict must carry a trustable attestation of the tier that produced it, and a stamp anyone
  could self-apply — or the absence of one — is not attestation. This closes the hole where the
  floor was strict against an honest below-tier stamp yet permissive against no stamp at all.
  The review lane's own trustable-stamp path (`deskdispatch --kit review`) is what lets a
  correctly-run risk-classed review clear the floor; the risk determination reuses the same
  signal the ready-flip's security-review gate reads, not a second scheme. The strong, `any`,
  aged-out and override cases are otherwise untouched, and the loud incident-recovery override
  still bypasses the floor for an unstamped risk-classed verdict.
- `SECURITY.md` and the pull-request template point to the new list: already-published
  sensitive content is reported through the private vulnerability channel, and the PR
  template asks an agent-assisted author to confirm the check.
- `docs/streams/forge-neutral/reviewer-write-boundary.md` §3.1 gains a reviewer-role forge-write
  inventory (repository write is the dispatch claim only — every other reviewer site is read,
  PR write or issue write) and a claim-reader inventory covering every site that reads, lists
  or releases a dispatch claim outside the claim tool. §3.3's S2 and S4 rows move from
  could-not-check to measured results: the local-disk race probe is established (exactly one
  of 16 winners), a network filesystem could not be reached under this agent's isolation floor
  and stays could-not-check, and filesystem-type / container detection are measured on darwin
  and (via a local container) on linux.
- `statusgen`'s committer-identity cross-check now uses a precise Evidence-section
  signal for its escalation candidates, replacing the whole-file "most recent commit
  touching the path" proxy the desk's PR-review round-1 decision (assay#1277) asked
  to be narrowed. `evidenceSectionTouchedByOtherIdentity` (`gitinfo.go`) asks whether
  an identity OTHER than the brief's author ever touched the `## Evidence` section
  specifically, anywhere in its history — not just whichever commit happens to be
  newest against the whole file. This closes two false-escalation shapes found live
  against this repo's own tree during review: (1) a later, unrelated, repo-wide
  mechanical commit (e.g. a brief-schema migration) that never touched Evidence at
  all resetting the whole-file signal past a genuine independent verification commit;
  (2) a later same-identity commit that appends a caveat/addendum *inside* the
  Evidence section after independent verification already landed (e.g. the
  implementer recording a security-review residual), which a narrower
  "most-recently-touched-Evidence" signal still misread as self-verification.
  `statusgen --lint` against this repo's own tree went from 37 false hard `PROBLEM`s
  to 0 after this refinement.
- `statusgen`'s git-committer-identity cross-check (`attribution.go`) now escalates a
  same-identity author/verifier pair to a hard `PROBLEM` — not just a `NOTICE` — when
  (a) the repo's brief history carries more than one git identity (so identity is
  genuinely discriminating) and (b) the brief's Verified/Evidence tokens self-label as
  independent (the token layer alone would have passed it). This closes the
  security-hardening/27 Task 2/4(b) gap tracked as #1116: a same-identity pair that
  avoids the free-text "implementer" token previously only ever produced a `NOTICE`.
  A repo whose entire checked brief history shares one git identity (a solo-maintainer
  or single-App-identity workflow) is unaffected — that case stays a `NOTICE`, by
  design, since identity cannot discriminate there.
- `system-demo` skill: added portable authoring rules from a field test outside Assay — distinguish demo beats from a host system's own phase/step numbering, an optional `00` cover beat before the promise, scene-level evidence-mode honesty (the label follows the pixels, not the bibliography), visible legends for meaning-bearing marks, a fixed-height top-aligned heading/caption row and the "no `overflow: hidden` on an annotated panel" layout rule, comparative one-request/many-paths stories, capture and rehearsal traps (true-width iframe overflow measurement, an in-page self-test over `--dump-dom`, absolute capture paths, exported-frame badges), and the finish-artifact set (scene manifest, transcript, evidence ledger, raster provenance, layout-invariant design record).
- `windows-port/00` Verify rows 8 and 10 re-baselined onto current main. Row 8's unix-only-syscall-leak grep now excludes comment lines and `_windows.go` files, so it flags only real syscall use rather than reddening on a prose comment. Row 10 now asserts the ACL-based Windows roster-owner enforcement (`evaluateRosterACL` — owner SID + DACL, refuse-on-unreadable, nil-DACL-as-world-writable) that superseded the earlier loud-skip `NOTICE` stub.
- ci(staged): re-base the Windows CI leg staged copy onto the live file so promotion is a byte-for-byte copy — the staged `ci/staged-workflows/windows-ci-leg.yml` now carries the live file's later changes (the version-tag-only trigger and the lint job's `fetch-depth: 0`) alongside the `--in-container` execution-witness jobs, so a maintainer's verbatim copy over the live file no longer reverts them.
- test fixture: neutralize an example product-config key

## v1.0.23 — 2026-09-21

### Added
- Verify tables may declare a per-row **shell** in an optional `Shell` column
  (`sh` / `cmd` / `pwsh`, default `sh`). `statusgen verifyrun` dispatches each row
  to its declared shell — `bash -o pipefail -c` for `sh` (every inherited row,
  unchanged), `cmd /d /s /c` for `cmd`, `powershell -NoProfile -Command` for
  `pwsh` — so a native-Windows row such as `findstr /c:"…" a\b.md`, which only
  works under `cmd.exe`, keeps its authored meaning instead of failing with a
  bash-level error under Git-for-Windows bash. The shell is declared, never
  guessed from the command text.
- `cellctl` recognises `ASSAY_REPAIR_ADMISSION=on|off` as a cell.env key, so the
  dispatch-boundary repair-admission gate can be turned on durably for a cell rather than only
  via a one-off shell `export`. `cellctl set` accepts `on`/`off` only (a malformed value is
  refused, and `--force` does not lift the value rule), `cellctl show` and `DRY_RUN=1 cellctl
  desk` surface it, and every desk the cell launches carries it in its environment. `off` and
  unset are identical: the key is simply absent from the launched environment.
- `statusgen verifyrun` selects a platform-appropriate shell when the default `bash` cannot
  bootstrap: an explicit `ASSAY_VERIFY_SHELL` override first, then Git-for-Windows bash at its
  well-known install paths, and could-not-run (never a silent pass) if neither works.
  `pipefail` semantics are preserved on whatever shell is finally used — a candidate that does
  not support `-o pipefail` is treated as unusable — and it never falls back to PowerShell or
  `cmd`. On Linux/macOS a working `bash` on PATH runs every row exactly as before, at the cost
  of one cheap probe per run.

### Fixed
- A native-Windows Verify row that passes under `cmd` but fails under bash no
  longer blocks a brief's closure with a false `fail exit=1`: it is either run
  under its declared shell (`cmd`/`pwsh`), or, when that shell is unavailable on
  the runner's OS (a `cmd`/`pwsh` row on Linux/macOS), recorded **could-not-run**
  with the reason — never `fail`, and never silently rewritten into another
  shell. `cmd`/PowerShell exit codes are read faithfully.
- `deskclose` now sends the REST `not_planned` state reason when closing superseded, duplicate or triaged issues, avoiding a validation failure after the closing comment has posted.
- `fanoutloop plan` no longer offers `Awaiting implementer rework` board rows that have
  already moved on. The rework lane now cross-checks each row against its own stream
  README Status cell (read from the same `origin/main` ref the board is read from) and
  drops any row that has left the awaiting-rework state — the rework already landed
  (`done`) or the deliverable was reset (`todo`) — the same rendered-board lag the Next-up
  lane already guards against.
- `fanoutloop plan`'s already-represented exclusion (a brief that already has an open or
  merged pull request, matched on the PR's `Brief:` trailer rather than a derived branch
  name) now also covers `Awaiting implementer rework` rows, so a rework row whose
  deliverable already merged is not offered for a fresh dispatch. Orphan resumes and
  durable repair obligations stay exempt, since a representing PR is expected there rather
  than a phantom.
- `statusgen --lint` now flags an unrecognised `Shell` marker (a typo like `bash`
  or `powershell`) as a hard PROBLEM at authoring time, so a marker the tool
  cannot resolve is never silently treated as the default.
- `statusgen --scan-issues` now quotes issue labels correctly in the generated
  `placeholder-v1` frontmatter. A label containing a YAML flow-indicator character
  (for example a trailing `?`) was previously emitted unquoted inside the
  `labels: [...]` flow sequence, producing frontmatter that failed to parse
  (`did not find expected ',' or ']'`) and reddening lint on every scan. The label
  list is now rendered through the YAML encoder, so each element is quoted exactly
  when — and only when — YAML requires it.
- `statusgen verifyrun` no longer records a Verify row as a false `fail` when the shell
  itself never started. On native Windows, `bash.exe` on PATH is often the WSL launcher; with
  no WSL distro installed it exits 1 with `execvpe(/bin/bash) failed` *before* the row's own
  command runs. verifyrun now resolves and probes the shell once per run and, when no
  pipefail-capable POSIX shell can be started, records the affected rows as **could-not-run**
  (with the reason) rather than `fail` — so a shell-bootstrap failure is never mistaken for a
  genuine product-check failure that a human then has to roll back.

### Changed
- Brief authoring now records a justified component structure, the rules and effects it separates, and how verification checks the boundary. The `domain-core` and `flat tool` defaults allow justified alternatives; adapter count prompts reconsideration rather than mandatory extraction.

## v1.0.22 — 2026-09-21

### Added
- New reader and fail-closed decision in `deskkit` (`External-Prereq-Only:` on the CR,
  `Cleared-Prereq:` on the re-approve), the sibling of the existing check-only exemption.
- The desk's ready gate now recognises a **same-head external-prerequisite exemption**: a
  standing `CHANGES_REQUESTED` whose only blockers were external prerequisites (an upstream
  PR that had not merged, a decision that had not been made) can clear at an unchanged head —
  with no synthetic no-op push — once the reviewer declares it in a typed record and every
  named prerequisite is independently re-verified from fresh evidence at flip time.

### Fixed
- GitLab `ChecksAtHead` now publishes the head pipeline as the required `pipeline` status
  context even when the commit document's `last_pipeline` is empty or stamped with a
  different SHA — the ordinary `merge_request_event` shape. It falls back to the same by-SHA
  read (`pipelines?sha=`) the board already uses, still reconciling on the exact head SHA, so
  `deskflip` checks-green no longer refuses a genuinely green MR pipeline. A head with no
  pipeline from either source stays could-not-check, never a pass.
- The GitLab forge adapter now preserves GitLab's own structured error body (`message` /
  `error`) on every write and read verb: `mapErr` carries the forge's message onto the
  `ForgeAPIError` and into the rendered refusal, so a rejection that used to reach the caller
  as a bare `HTTP 400` now names the actual cause (a permission message, a validation
  message, a missing branch). Redaction rules are unchanged; the body is only control-stripped
  like every other forge-origin string this tree renders.
- `CreateDraftChange` now rides out GitLab's transient post-push "source branch does not
  exist" rejection with a bounded, same-identity retry (3 attempts, 2s apart) for that one
  specifically identified condition — the race where a just-pushed branch is readable through
  Git and the branches API but the merge-request create briefly still 400s. Every other error,
  including every other 400, is surfaced on the first response and never retried; exhausting
  the bounded attempts is reported as could-not-check, never rounded up to a pass.
- `deskevidence` on a GitLab-hosted repo now lands Evidence into an **existing**
  brief. The GitLab file-write path probed the not-yet-created target branch for
  the file, always missed it, and issued a create (`POST`) that GitLab rejects
  `HTTP 400` when the path already exists on the base — so post-merge verify could
  produce a PASS but never land the Evidence row or the `implemented → verified`
  flip. The existence probe now reads the branch the write is based on (the start
  branch, for the inline side-branch lane), so an existing path is updated with a
  `PUT` and only a genuinely new path is created with a `POST`. (#1412)

### Changed
- `deskboard` surfaces a declared external-prerequisite re-review as `EXTERNAL-PREREQ-REVIEW`
  rather than `SUSPECT-APPROVAL`, and `reviewloop` routes the new action; the board, planner
  and ready gate agree on what the row is. The three-round finding cap, the independent
  security review, the check-only exemption, and the human merge/ready authority are unchanged.
- `deskpost ready` clears the unchanged-head `CHANGES_REQUESTED` refusal only for a declared,
  fully re-verified external-prerequisite rejection; it still fails closed on a wrong
  revision, a prerequisite predating the rejection, a later revocation, an unrelated object,
  unreadable evidence, a standing `Security-Review: fail`, or any code/content finding.

## v1.0.21 — 2026-09-21

### Added
- Persistent review findings survive agent replacement and restarts. A reviewer
  verdict or worker reply may now carry a versioned `review-finding/v1` block
  (embedded additively in the forge body, invisible to a legacy reader), and
  `reviewloop` derives the outstanding findings, disputed responses and per-class
  round counts from those durable records rather than from an agent's memory —
  identical after a replacement or restart because the ledger is a pure function
  of the records.
- The Go `cellctl` launcher reads shared provider model and desk-effort defaults from `CELLS_ROOT/providers.json`, with partial per-cell overrides. `cellctl providers init` creates the editable catalog without overwriting it, and Claude launches export the selected provider's Fable, Opus, Sonnet and Haiku mappings. Existing complete model policies retain precedence.
- `reviewloop plan --records <thread.json>` renders the derived finding ledger for
  one PR: outstanding blocking findings, per-class rounds against the existing
  cap, the single arbiter packet at the cap, and every could-not-check reason.

### Fixed
- statusgen's brief-parse memo now keys on a hash of the file's content instead of
  an `(mtime, size)` stamp. The old stamp could not tell two same-size versions of
  a brief apart when a coarse-granularity filesystem recorded both writes under one
  mtime tick, so a length-preserving in-place edit (e.g. flipping a `gates:` target
  from one brief to another of equal-length id) could be served from the stale
  pre-edit parse. This made `TestEligibilityDeclarationChangesDispatch` flake on the
  self-hosted release runner while passing on nanosecond-mtime macOS, and — more
  importantly — could have let any consumer read a stale gate/eligibility verdict for
  a brief edited during a run. The cache is now correct on every filesystem regardless
  of its timestamp resolution.

### Changed
- Review scope is now bounded by a declared first pass and an impact-based blocking
  boundary. A reviewer inventories the related occurrences of a false-claim class on
  the first pass — recording the search, its scope, its exclusions and the input
  revision — and an incomplete search is reported incomplete, never certified clean.
  A blocking finding must name a concrete failure and its scope basis (changed
  behaviour, an explicit acceptance obligation, a material PR-body/Verify claim, or a
  demonstrated safety consequence of the change); unrelated pre-existing prose is
  routed to a follow-up instead of holding the PR. A missed sibling occurrence keeps
  its original claim class and round count, and a previously non-blocking occurrence
  cannot become blocking merely because another file was edited — a promotion requires
  changed impact or new evidence, explicitly recorded. The existing three-round cap
  and the independent security review are unchanged.
- `deskpost review` and `deskreply` validate an embedded finding block before any
  network call: a worker reply cannot author a reviewer's resolution of a blocking
  finding or hand-assert the arbitration cap, and a blocking finding must carry a
  concrete reproduction or evidence-based explanation. Bodies with no block are
  unaffected.

## v1.0.20 — 2026-09-21

### Added
- The gate serialises admission across dispatchers with a compare-and-swap lease in the existing
  claim backend (so two hosts cannot both admit into the last slot), resolves an item's class from
  the authoritative repair-obligation store (a caller cannot relabel fresh work as a repair), and
  treats a waiting-external repair as non-runnable so it never idles a usable slot. Unreadable
  occupancy or demand is a visible could-not-check (exit 6), never a fabricated free slot.
- `deskdispatch` can now ENFORCE the worker pool's repair (`rework`) reservation at the dispatch
  boundary, opt-in via `ASSAY_REPAIR_ADMISSION=on` (recorded policy `repair-admission-v1`). When
  on, a **fresh** dispatch that would drop the free slots to or below the reserved floor while a
  repair obligation is runnable is refused (exit 5, naming the waiting repair); the repair itself is
  admitted. Previously the reservation was only PRINTED by `fanoutloop plan`, so a caller could
  ignore it and fill every reserved slot with fresh work while a repair waited.

### Changed
- Ships **OFF**: with `ASSAY_REPAIR_ADMISSION` unset, `deskdispatch` behaves exactly as before —
  the reservation stays advisory and no dispatch is held. That unset state is also the rollback.
  Repair-obligation records stay additive and readable by an older reader.

## v1.0.19 — 2026-09-21

### Added
- Durable **repair obligations** (`repair-obligation-v1`): a failed or blocked verifier run now
  records a structured, idempotent obligation — keyed by (repo, brief, source receipt, failing
  rows) — that survives the reporting agent. The immutable key means a duplicate delivery, a lost
  acknowledgement, or a process restart all reconcile to the SAME obligation rather than
  manufacturing a second, and an expired worker lease returns it to the queue for a replacement
  worker without duplicating the work. Obligation states (needs-assignment, repairing,
  awaiting-review/merge/reverification, waiting-external, resolved) are scheduling state, never
  acceptance.
- `worker-desk` (fanoutloop) reads outstanding repair obligations across the configured roots as
  a rework source: an actionable, still-unresolved obligation is dispatched like any other rework
  item, carrying its reproduction and expected behaviour so the worker starts from the failure —
  and, when the original deliverable PR has merged, the repair opens a FRESH follow-up branch in
  the correct deliverable repo instead of resuming immutable history.

### Changed
- A repair obligation resolves ONLY on a valid INDEPENDENT verification at the repaired revision.
  A merge wakes reverification but does not close the obligation; a same-actor pass (the worker
  that produced the repair) and a wrong-revision pass are both refused. Worker completion, issue
  closure and merge alone can never resolve an implementation obligation.
- desktools-v2 stream unparked: spec approved and brief-10 override policy ruled (layered overrides) — #1319

## v1.0.18 — 2026-09-21

### Added
- Verification **wake receipts** (`verify-wake-v1`): a failed or blocked verifier run records a
  checkable wake condition — the inputs it observed, the blocker class, and what must change
  before re-running is worth a slot. An unchanged receipt keeps the failure visible as a `wait`
  row (naming its blocker and next actor) but no longer consumes a verifier dispatch every pass;
  a changed relevant input, tool version, Verify definition, or completed action wakes it, while
  an unrelated change does not.
- `--model-top/mid/fast <m>` flags on `cellctl desk`/`up` override a provider's per-tier models for one run (refused without a provider); `--set` persists them as `CELL_PROVIDER_<NAME>_MODEL_<TIER>` cell defaults, and `up` threads them onto every role window
- `.github/workflows/forge-surface-control.yml`: an advisory step running the new counter
  alongside the existing shell-exec ban / no-passthrough / single-construction-site checks.
- `cellctl` per-tier provider models: a provider may name a different model for one tier (`CELL_PROVIDER_<NAME>_MODEL_TOP|_MID|_FAST`, cell.env line or preset) — the tier's role windows launch on it and the matching `ANTHROPIC_DEFAULT_*_MODEL` alias maps to it, else the flat provider model
- `docs/streams/desktools-v2/seam-contract.md`: the one-page statement of the v2 seam
  contract — the four GitHub-fact classes (`gh` subprocess, hardcoded `"origin"`,
  `pullRequest`/`mergeRequest` GraphQL block, `api.github.com` host literal) and where each may
  legitimately appear (the two `Forge` backends and their tests, plus `forge.go`'s
  `GitHubAPIBase` for the host literal).
- `tools/desk/scripts/forge-ban.sh`: a portable (macOS + Linux) advisory counter for
  reach-around sites, covering `tools/desk/**`, `tools/cellctl/**`, `plugins/assay/**` and
  `statusgen/**` (the last of which is not under the existing `forgeban` register), reporting
  desk/statusgen counts separately and per class; `--baseline` records the total to
  `docs/streams/desktools-v2/forge-ban-baseline.txt`.
- the `glm` preset maps the MID (sonnet) slot to `glm-5.3-flash[1m]`, so mid-tier desk windows and every sonnet ask inside any window run the flash variant while the top tier keeps the full model; an operator-set flat model suppresses preset tier splits, and `cellctl check` prints the sonnet slot as its own row when it differs

### Changed
- `verifyloop plan` classifies a failed/blocked brief with an unchanged wake receipt into a new
  `wait` bucket instead of re-dispatching it. Unreadable declared inputs stay visibly
  could-not-check (never rounded up to unchanged), and legacy or incomplete receipts stay
  eligible for one classification pass. A partial hold lets a newly-runnable row dispatch while
  the held rows are recorded as explicitly unrun — no partial result closes the whole brief.

## v1.0.17 — 2026-09-20

### Added
- A `**VERIFY: PASS**` marker is no longer a flip signal on its own when the same Evidence
  entry also reads `HELD` or `could-not-check` on a row that is not genuinely routed —
  enforced in both the model autoflip (`autoflip.go`) and the verify-gate card/`closeVerify`
  (`verifyissues.go`). The routed-row exclusion anchors on `unrun.go`'s own definition
  (`routingKeywordRe` + `routingRefRe`: a routing phrase such as "deferred to" PLUS a
  corroborating reference), not a bare substring "deferred", so negated prose
  ("NOT deferred to anyone, still broken") fails CLOSED instead of silently suppressing the
  contradiction.
- A new DEPLOYS register (`docs/streams/deploys/`, reference implementation
  `statusgen/deploygate.go`): a DEPLOY record's `brief:` precondition (the carried brief
  must be `verified` or `done`, never merely `implemented`) is a hard `--lint` PROBLEM when
  unmet or dangling; the rollback grammar (`rollback: none-accepted` requires a named
  `rollback-approver`) is validated; and an undrilled RUNBOOK drill row is reported
  `could-not-check` — visible, never a silent pass and never a hard failure. Reuses the
  existing `blocked-by: env` marker for a deploy waiting on an environment, rather than
  minting a second one.
- The deploy model (sdlc/06): `docs/deploy-model.md` specifies environments as a typed
  declaration, deploy as a gated transition on its own record (never a sixth brief-lifecycle
  state — `spec/lifecycle-v1.md` §9), the rollback obligation (a stated reverse path, or an
  explicitly accepted absence naming an approver — reconciled against, never contradicting,
  `docs/distribution.md`'s "there is no rollback" release statement), and runbooks as a
  typed recovery artefact with drill rows whose Evidence is filled by whoever ran the drill.
- `TestVerifyrunPipelineExit` pins a measured pipe-masked false-clean
  (`<bad cmd> 2>/dev/null | head -c1`) directly, alongside the existing pipefail regression
  coverage.
- `deskevidence` gains `--dry-run`: it mints the verifier App token, resolves the forge,
  fetches the remote content, merges/scans it and runs the statusgen PROBLEM-diff guard —
  every gate that can refuse a landing still runs — then prints the commits-API landing plan
  (create/update, target path, branch, sha256, row delta) and stops before the write-rate-limit
  spend and the write itself. There is no local-git fallback anywhere in this tool: an
  unmintable verifier App token refuses at the mint step, `--dry-run` or not.
- `deskroster set` gains optional resource-vitals flags (`--tokens`, `--context-pct`,
  `--session-age-seconds`, `--subagents`, `--model`) so a desk session can self-report its
  own tokens/context/age/subagent/model state onto its roster beacon, each field
  three-state (measured / could-not-check / unset) and never a fabricated zero.
- `deskroster set` refuses a `--session` value that does not resolve to a single path
  segment (no `/`, no `..`), closing a beacon-path-join hardening gap identified in
  security review. `desksupervise status` applies the same single-segment check to the
  claim `holder` before joining it into the roster beacon read path, so a holder carrying
  `/` or `..` renders `could-not-check` rather than reading a file outside the roster
  directory.
- `desksupervise status --json` fills the previously-reserved `tokens` stub with a full
  `resource` block per claim, joined from the claim holder's own roster beacon (or a new
  `--beacons-fixture` for offline Verify runs); a claim with no readable beacon renders
  every resource field `could-not-check`.
- `hasVerifyPass` now matches a ratified bold `**VERIFY: (PASS|FAIL)**` regex instead of a
  fixed substring, so a real verifier line carrying prose before its closing `**`
  (`**VERIFY: PASS (4/4 offline-runnable rows)**`) is recognised; `BLOCKED` still never
  matches.
- `statusgen --export-audit-pack --release <tag>`: a release-keyed audit pack, walking
  release -> brief -> requirement -> Evidence/review verdict via `docs/release-notes/<tag>.md`'s
  optional scope frontmatter. Reuses `--export-evidence`'s existing `manifest.json` shape
  verbatim and refuses to write when an independent completeness comparison against the
  sdlc/02 rollup disagrees, naming both counts. See `docs/evidence-bundle.md`'s release-keyed
  section.
- `statusgen --flow` (graph-execution/07): per-brief `eligible_to_start` /
  `active_work_time` / `external_wait` / `verification_time` durations derived
  from the historian, fleet-wide medians (gated on `gtSmallN`), `ci_slot_saturation`
  (real network access only with `--forge`, never on credential presence alone)
  and `gate_catch_override` (re-emitted from `--gate-telemetry`'s own sources),
  with an environment stamp on every report. `--bottleneck`'s stage-age heuristic
  is unchanged and renders unaffected beside it.
- `statusgen --lint` gains `witnessAbsenceGateChecks`: a `verified`/`done` closure THIS
  branch makes with no execution witness (`statusgen verifyrun`) for one or more Verify
  rows is now a hard PROBLEM, merge-base scoped exactly like the existing contradiction
  and UNRUN gates — a pre-existing closure at the merge-base stays the per-stream NOTICE
  it already got.
- `topology.yaml` gains a strict-parse `comms:` key (`tools/desk/internal/topology`'s
  `CommsMode`) — one of the three independent off-switches (topology key, `ASSAY_COMMS_*`
  env, deployed gateway) the cell-comms enablement contract requires. Absent reads as
  disabled; an unrecognised value is a parse error naming the line; the key is declarative
  only and wires nothing on its own.

### Fixed
- `deskclose` derives the authorizing comment's kind from its own permalink (`/pull/` → change,
  `/issues/` → issue, falling back to change only on could-not-check) instead of always reading
  the change/pull-request thread — a human ruling recorded on an ISSUE now authorizes instead of
  coming back could-not-check every time (#1019).
- `tools/desk/cmd/cellctl` (the shipped Go binary) now honours `CELL_MODEL_POLICY`: `set`/`show`
  accept and display the key, the policy JSON is schema-validated (harness/tier shape, exact
  model IDs, per-harness effort levels, deny list), per-role provider/model/effort resolution
  drives `desk` and `DRY_RUN=1` dry-run output (including the policy file's sha256), effort
  propagates into the Claude/Codex launch env and argv, a denied model (e.g. `*opus-5*`) refuses
  the launch outright, and a child-model request is resolved and effort-checked against the same
  provider's tiers. Previously the Go binary silently ignored the key entirely (assay#1390).

### Changed
- Planning only: no runtime behavior, model installation or operational authority changes.
- Prefer an optional CPU-first Laya evaluation; retain deterministic graph operation and
  existing human gates. Amend unimplemented coverage/recovery/replay contracts in place.
- Route a proposed graph extension into ten bounded briefs for instances, local advice,
  admission, Cell recovery, evidence exports and lifecycle links.
- `cmd/commsgw`: `TestInertWithoutAllKeys` proves each `ASSAY_COMMS_*` key refuses
  individually, not just all-absent.
- `commsgw`'s README documents the full three-part enablement contract and reflects a
  2026-09-17 human ruling on this cutover decision (Option 2, "Interim rung first", over
  the recorded full-enable target — the ruling itself is recorded outside this public
  repo; tracked publicly as #1289): receive-and-route live, every execution a proposed
  dispatch a person fires, full autonomous enablement not implemented by this change.
- `docs/evidence-bundle.md` and `statusgen/README.md` now describe witness *absence* as
  merge-base scoped (grandfathered NOTICE vs. hard PROBLEM for a new post-pin closure),
  matching `spec/lifecycle-v1.md` §2.4 — they previously asserted the flat pre-PR "NOTICE,
  not a block" behaviour unconditionally.
- `internal/topology`: `TestTopologyComms` / `TestTopologyCommsPositiveControl` pin the
  `comms:` key's parse and drift-detection behaviour.
- `spec/lifecycle-v1.md` §2.4's "no execution witness" sentence is now date-bounded to
  closures before statusgen v1.0.13; `plugins/assay/skills/verify-desk/SKILL.md` names
  `statusgen verifyrun` as the run step and the commit-with-Evidence step explicitly.
- `spec/lifecycle-v1.md` §7.1 clause 2 names the new identity check and its exact scope (a
  post-cutover Evidence commit, evaluated per-transition against `closedAtBase`) so the spec
  never claims more independence than the lint enforces; the Verified cell and any Evidence
  commit outside that scope remain attribution-on-text, not identity, as before.
- `statusgen`'s Evidence-actor check (desk-apps/07, F-verify-self-attest) is now merge-base
  scoped, the same shape `unrunGateChecks` and `witnessGate` already use: a `verified`/`done`
  brief whose Evidence section is not backed by the roster's verifier role is still a NOTICE
  when the closure predates `merge-base(HEAD, origin/main)` (the pre-cutover backlog), but is
  now a PROBLEM naming the actual rejected identity when the closure is one this branch newly
  made. The deliberate-spoof/tamper subclass (an Evidence commit dressed as the verifier —
  right name, wrong-or-absent account id) gets the SAME new-vs-backlog scoping and the stronger
  disposition: a new-closure impostor is a build-blocking PROBLEM naming the TAMPER signal, not
  the NOTICE it previously always was, while a backlog impostor stays a NOTICE. A shallow/grafted
  clone or an unresolvable merge-base still renders as could-not-check, never as either a pass or
  a failure.

## v1.0.16 — 2026-09-20

### Added
- **CONTRIBUTING.md states the trust bar for submissions from unrecognised accounts**
  (contributor-trust/06, completing the part deferred from the earlier partial): what the
  provenance card measures and what it deliberately never measures, the four trust tiers and
  what each one unlocks, how a maintainer's comment admits an item, and the disclosure
  boundary — the tier model is published, who holds which tier is not. `docs/contributor-trust.md`
  gains the provenance-card hop, so a contributor can walk `CONTRIBUTING.md` to the tier model
  to the description of what the card measures with no dead link on the way.
- On a reap the stale local branch is deleted too, when it is equal to or behind its upstream,
  with the non-force `git branch -d` — so a later `deskwt add --branch` cuts fresh from origin
  instead of colliding with a leftover ref. git's own merged-into-upstream refusal is a second,
  independent layer over the ancestry check the sweep already made; there is still no `--force`
  anywhere in the verb. (#1375)
- Plan six incremental review and verification repairs: verification wake conditions, durable worker obligations, repair admission, persistent review findings, first-pass scope and external-prerequisite reverification. Reuse existing verifier repair work and the current review round cap; this change authors briefs without activating new runtime behavior.
- Release notes now close with an `authorized-by` line naming who authorized the cut — the dispatch actor on a dispatched release, an explicit not-recorded on a tag push, never blank — with the honest boundary stated beside it: it records authorization, not who or what built the artifact, and the pipeline carries no signature or provenance attestation. A source-coupling test (with a mutation positive control) reddens if the wiring is dropped from the release workflow.
- Review lane sets keyed on the pull-request author's contributor-trust tier (`deskkit.LanesFor`, and the dispatch selection path `deskkit.ReviewLanesForAuthor`): `unknown` and `blessed-once` authors are reviewed with a claims-versus-diff fact check and a mandatory fail-first reproduction beside the correctness and security lanes; `contributor` and `maintainer` authors keep today's standard path unchanged.
- The `deskdispatch` review-lanes dispatch reference (`tools/desk/cmd/deskdispatch/references/review-lanes.md`): the per-tier lane sets, the fact-check output contract, and the fail-first reproduction's two required records — held to the lane table by test.
- The fact-check claims contract (`deskkit.ClaimState`, `deskkit.ExtractClaims`): every body claim carries exactly one of `confirmed` / `contradicted` / `unverified`, and `unverified` is never rounded to `confirmed`.
- `ask-decision` gains a §"Ratification — a relay is not yet a ruling" section: the five-part
  relay-comment template, the rule that ratification is an act only the driver can perform
  (handed over as a `human-runsheet` entry), the not-a-wait-state rule for the desk, and the
  post-ratification amendment-dispatch step.
- `cellctl` can select a provider, pinned model and effort per desk from one JSON policy, with Claude alias/child settings, Codex reasoning defaults and whole-cell launch preflight. Policy mode prohibits Opus 5 and the example maps Opus to 4.8.
- `deskverdict sign`/`verify` gain a `--key verifier|issue-loop` role selector: it picks
  WHICH role's key is used, never where a key comes from. Both roles' public keys stay
  repo/Actions VARIABLES (`ASSAY_VERIFIER_PUBKEY` / `ASSAY_ISSUE_LOOP_PUBKEY`) — no key
  material of any kind is committed to the tree for either role. The signed block now
  declares its signing role, and `verify` refuses a block whose declared role differs
  from `--key`, before any signature arithmetic runs.
- `deskwt prune --dry-run --reap-dead-sessions` prints the full plan — path, session (or
  `unowned`), `REAP`/`KEEP`, and the reason — for every registered worktree, including the ones
  the identity refusals put out of reach, and changes nothing. (#1375)
- `deskwt prune --reap-dead-sessions` (default OFF) clears the worktrees a dead desk session
  leaves behind — the shape no earlier sweep could reach, because a session that died with
  work in flight sits on an unmerged branch that the merge gate reads as active work and
  holds forever. Since git permits one worktree per branch, every later resume of such a
  branch failed at worktree-create and the drain queue wedged on its own leftovers. The arm
  removes a worktree only when no LIVE session owns it (its lock names a session the roster
  shows is gone, or it carries no lock at all) AND removing it is provably lossless: the tree
  is clean with untracked files COUNTED, and HEAD is already reachable from its upstream or
  from `refs/remotes/origin/main`. A live session's lock still holds its worktree
  unconditionally, and anything dirty, unpushed or unverifiable is listed with the reason that
  held it rather than removed. (#1375)
- `docs/streams/desktools-v2/inventory.md`: a frozen, file:line-accurate inventory of every
  site that reaches past the `Forge` seam (`tools/desk/internal/deskkit/forge.go`) with a
  GitHub-specific fact or an ambient credential — 69 sites across `statusgen/**`,
  `tools/desk/**`, `tools/cellctl/**`, `plugins/assay/` skill scripts, and
  `.github/workflows/**` — reconciled against the existing `forgeban` permit register, routed
  to the migrating brief that owns each site, and flagging the largest undocumented finding: a
  17-site hand-rolled GitHub REST+GraphQL client living in `deskpost/github.go` outside both
  sanctioned backends. No code changed; this is the `desktools-v2` stream's audit brief.
- `statusgen --transcribe-scan-delta`: the R-7 clause-4 cross-repo scan-delta verify path.
  It sweeps open issues on the home repo for a payload signed with the issue-loop role key,
  behind the same R-7 enactment gate as `--transcribe-scan`, and checks container-author
  identity, the role-declared RS256 signature, body-unedited timeline, a per-entry API
  re-check where readable, and same-repo-entry refusal — each layer naming the clause it
  refuses under. Ships INERT; adds no new arming path.
- `the-desk` gains one pointer line at its escalation-labels step naming the new section, so the
  coordinator body routes relays to the template instead of re-inventing the comment shape.

### Fixed
- The `On-behalf-of:` composite-identity trailer no longer stamps a human's forge login
  onto writes to a **public** repo. The one shared resolver every write verb calls
  (`deskpost`, `deskreply`, `deskpr`, `deskfile`, `deskevidence`, `deskflip`) now chooses
  the form from the target repo's configured visibility: the roster's neutral name for
  that human on a public target, the login on a repo the roster states is `:private`. The
  split fails closed towards the neutral name — an unstated visibility, a repo admitted
  only by an `owner/*` pattern, and an unnamed target all take it — and the target repo is
  now a mandatory argument to the resolver, so no verb can resolve an identity without
  saying where the write lands. A public-target write refuses (exit 5) rather than fall
  back to the login when the roster carries no neutral name for the principal.
- The `adopt` skill now routes Codex CLI installs end to end: the two install arms, the
  generated `AGENTS-assay.md` resident-rules fragment step, the dispatch-config note, and
  the sandbox refusal floor — the adopt-side deliverable of harness-portability/06 (#872).
- The `truth-suite` mutation gate for `internal/deskkit` is green again. The per-tier review-lane reference check reached its reference document by climbing to the repository root and descending again, so it resolved in a full checkout but not in the isolated module copy `muhar` gives each worker when more than one mutation is in flight — the harness read a red baseline it could not attribute and discarded the whole run, reddening `main` on every push. The reference now resolves inside the module, and a new assertion holds it there so the path cannot drift back out.
- The alias is always passed to `ssh -G` after an explicit `--` end-of-options marker, and a
  leading `-` on the alias is refused outright before any resolution is attempted, so a
  flag-shaped host segment parsed from a remote URL can never be read as an `ssh` option.
- `deskclaim-ref` now resolves an scp-like `origin` remote whose host is an SSH config `Host`
  alias (`git@alias:owner/name.git`) by shelling `ssh -G <alias>` and taking its `hostname`
  line, instead of dialing the unresolvable alias literally as an HTTPS host. (#1371)
- `deskdispatch --kit verifier` cuts its worktree DETACHED off `origin/main` under its own `verify-<item>` name (`deskwt add --detach`, new) and never touches the brief's feature branch, so a delivered brief's `feat/<id>` sitting in a stale worker worktree can no longer refuse the verify pass. The worker-kit worktree-create refusal now distinguishes a branch CHECKED OUT and active in another worktree from a branch that merely exists (delivered). (#1309)
- `deskdispatch` **resumes onto the change's own branch.** A dispatch that names an
  already-open change (`--pr <N>`) now cuts its worktree from that branch's own remote ref
  (refreshed first), instead of cutting a fresh branch of the same name off the mainline —
  which left the worktree at main's tip while the change's commits lived only on its remote
  branch, so a resuming agent silently started from the wrong commit. A fresh dispatch, and
  the read-only review and verifier lanes, still cut from the mainline.
- `deskwt add` **judges a branch collision against the branch's own remote counterpart**, not
  against the mainline. A branch this tool created tracks the ref it was cut FROM, so every
  real feature branch read as "unfinished work, not a leftover" and its collision was refused
  permanently — even when the local branch was identical to its own pushed counterpart. A
  branch carrying commits beyond that counterpart is still refused, now naming the counterpart
  rather than the mainline. The `@{upstream}` lookup behind that comparison is also fixed: it
  used a ref spelling git rejects, so the upstream arm never resolved at all.
- `deskwt role-init` wires the WORKTREE-scoped credential helper for the role's App token (chain reset at worktree scope, one inline helper reading the 0600 token file; re-wired on reuse) alongside the commit identity, then runs the role's own preflight against the provisioned worktree — red is exit 6, so a sibling-root boot whose first https fetch would fail "could not read Username" stops here instead of hiding that root's queue. (#1309)
- `verifyloop plan` (and the drain's queue read) is MULTI-ROOT: with `DESK_ROOTS` set and no `--root`, it iterates every configured stream root — the same map `deskboard` reads — runs the envelope preflight PER ROOT (a red or unreadable sibling is reported and skipped, never a whole-pass abort; the exit is then 6 because the plan is not the whole queue) and names the root on every printed item. An explicit `--root` narrows to one; with `DESK_ROOTS` unset the single-root read is unchanged. (#1309)
- `verifyloop plan` buckets a brief whose latest `verify-outcomes.jsonl` row is `verified` and whose Evidence is filled as `stuck-flip` — a finding to file / point the flip at — instead of re-listing it as DISPATCH item 1 on every pass. (#1309)
- `verifyloop plan` derives the online-lane / longitudinal signals per Verify ROW from the Command cell only, never from Expect prose, and defers rows rather than briefs: a brief with any offline-runnable row stays DISPATCH and names the rows to record as explicitly unrun; only a brief whose every row is non-runnable is bucketed. Explicit `verify-lane:` / `blocked-until:` markers still win. (#1309)
- `verifyloop plan` emits a second dispatchable class, `DISPATCH-FOR-EVIDENCE`, after the `DISPATCH` set: a human-gated / risk-flagged brief whose Evidence is still empty and which nothing else withholds from an offline run — Evidence rows plus the outcome sidecar row only, flip never. A human-gated brief whose Evidence is already gathered stays awaiting-human. (#1309)
- `verifyloop plan` treats a board row whose brief file cannot be found or read as `could-not-check` — never dispatchable — instead of classifying its zero-value frontmatter as a risk-clear, model-gated brief. (#1309)

### Changed
- The per-platform tarballs now carry a real `cellctl` built for their own platform, the windows
  legs included, where before every platform got the same shell script. The Windows build
  compiles today but is UNPROVEN: standing a Windows cell up belongs to the windows-port stream.
- The prune summary and audit line now carry two further counts, `dead-session-reaped` and
  `branches-deleted`, so a drained repo can be told from a stuck one at a glance. (#1375)
- `--lock-ttl` is accepted with `--reap-dead-sessions` as well as with
  `--reclaim-stale-locks`; alone it is still refused rather than silently inert. (#1375)
- `cellctl new` no longer writes internal stream identifiers into the README and roster it
  scaffolds. The grammar, custody and check sentences keep their meaning; only the citations are
  gone, so a scaffolded cell no longer carries pointers to material an adopter cannot read.
- `cellctl` is now a Go program (`tools/desk/cmd/cellctl`), built and shipped like every other
  desk verb instead of being a `sed`-stamped shell script copied into the release tarball. The
  shell launcher stays in the tree as the parity ORACLE: `tools/cellctl/tests/parity.test.sh`
  diffs both implementations' `DRY_RUN=1` plans across every kind × harness × cockpit × verb —
  200 cells, byte for byte, including the whole tree `new` scaffolds — and the existing
  behavioural suites now run against either implementation through a `CELLCTL` override.
- `cellctl` now declares its tool class and writes the P3 effective-config echo to stderr once per
  run, like every other desk verb that reads the roster — it consults the cell home's roster to
  answer `check`'s write-authorisation rows, so a narrowing of that surface is now visible at run
  time rather than only in a diff. The class is the write class: the cell's config-home file is the
  only admissible source, never the environment. Scripts that parse `cellctl` output should read
  stdout, which is unchanged; the echo is stderr-only and the parity oracle normalises it away.
- derived-board/03 brief: Verify rows 4 and 5 re-baselined after three
  consecutive verifier cycles tripped on stale anchors — row 4's reconcile
  lookup is now id-shape-tolerant (ids went hierarchical
  `assay:assay:<stream>:<NN>` at the brief-v2 flag-day), and row 5 expects the
  post-#1251 eligibility-evaluator wording now that `gates:` is actively
  gating instead of reserved.
- forge-neutral/19: closes the series #992 tracks by stating, surface by surface, which
  human-only forge actions are genuinely server-side-enforced and which are not. Documents
  that merge-to-protected-branch is currently enforced only by desk-App convention (any App
  holding `pull_requests: write` can already post an approving review and merge — no
  server-side rule stops it), and specifies a `human-approved` required-status-check
  workflow contract to close that gap (workflow file and the required-check ruleset edit are
  named as human/repo-admin follow-on work, not landed here). Confirms workflow-file pushes,
  rulesets, CI variables, and App installs are already server-side-enforced today on both
  GitHub and GitLab (GitLab's `.gitlab-ci.yml` needs a protected-branch + CODEOWNERS rule in
  place of GitHub's dedicated `workflows` permission scope, since GitLab has no scope-level
  equivalent).

## v1.0.15 — 2026-09-19

### Added
- A rotation-aware `verify-outcomes*.jsonl` glob-union reader in `statusgen`, so a future
  date-sharded rotation of the verify-outcomes sidecar has read-side support in place before
  any rotation is attempted.
- Every per-run `cellctl` choice is now a flag, persistable and readable (#1303): `desk`/`up`
  accept `--kind`, `--cockpit`, `--harness`, `--provider`, `--model` for one run; `--set` persists
  every override given in that invocation (each to its own `cell.env` key, one backup first);
  `cellctl set <cell> --kind/--cockpit/--harness/--provider` is sugar for the matching
  `KEY=VALUE` under the same validation; a kind change refuses before writing when the target
  kind's precondition (`CELL_CONTAINER_LAUNCHER`, `CELL_ROOTS`, `CELL_REPO_SLUG`) is missing; and
  a new `cellctl show <cell>` prints each effective value with its source
  (`flag` / `cell.env` / `default`).
- `cellctl` gains built-in provider presets `kimi` and `glm` for running the claude harness against
  Anthropic-compatible endpoints (#1303): `--provider kimi|glm` on `desk`/`up`/`set` works with no
  `cell.env` line beyond the operator exporting `KIMI_API_KEY` / `ZAI_API_KEY` in their shell
  (cellctl never stores or prints a token value); a new `CELL_PROVIDER_<NAME>_MODEL` key (preset
  defaults `k3[1m]` / `glm-5.3[1m]`) is the model a provider window runs absent `--model` or a
  per-role pin; the launch unsets `ANTHROPIC_API_KEY` and exports `ANTHROPIC_MODEL` plus the three
  `ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL` aliases; `cellctl check`/`show` report the endpoint,
  the token env var's name, set/unset and the model, each tagged preset/cell.env; codex + provider
  is refused.

### Fixed
- `deskevidence` no longer refuses appends to `docs/streams/verify-outcomes.jsonl` past the
  general 256 KiB per-file cap — the append-only aggregate sidecar now carries its own
  documented 4 MiB ceiling (#1338).
- `statusgen --corroborate` no longer reads the on-behalf-of attribution form (`on-behalf-of human:<login>` in a Runner cell or prose, and the `On-behalf-of:` trailer) as a `human:<name>` sign-off stamp or an acceptance citation; only sign-off vocabulary is judged, so App-authored Evidence rows carrying the principal the attribution lint requires no longer red the checker (#1335).

## v1.0.14 — 2026-09-19

### Added
- `DR-forge-neutral-19` design-decision record: transcribes the driver's ruling on #1256
  that merge to the protected branch is not server-side enforced today and that the fix
  shape is the `human-approved` required status check `forge-neutral/19` specifies; the
  brief now cites it via `design:`. Record-only — the workflow file and the ruleset entry
  stay the repo admin's follow-on acts.
- `capability:cadence-tick` joins the closed capability vocabulary: the scheduled recurring prompt that wakes a desk-role window on the clock, bound per harness in `plugins/assay/references/{claude-code,codex,cursor}.md` (Claude Code: the recurring-prompt loop the coordinator window already arms; Codex/Cursor: the launcher's or an outer scheduler's tick-mode interval, stated as a degradation).

### Changed
- `deskroster`: the verify-desk default width is now 6 (the measured safe width, equal to its declared ceiling) instead of the sequential 1 it decayed back to after the one-hour width TTL.
- `verify-desk` skill: arming the cadence tick is a REQUIRED, named boot step (a window that cannot arm it says `could-not-check` and files it, never runs keystroke-driven); "never end a turn with a non-empty dispatchable queue" plus a printed stand-down checklist replace the round-summary-as-stopping-point; a desk-set width is re-asserted on every tick so it no longer decays mid-drain; the stale-heartbeat case is named in the default-forward list as a STOP class, never a question for the driver (#1310).

## v1.0.13 — 2026-09-19

### Added
- A scrubbed cell is single-occupancy, enforced by two independent layers: a private-socket tmux
  session and an atomic-`mkdir` session lock; a second `desk` while one is live is refused (exit
  4) naming the running pid and session.
- Draft scoping doc `docs/streams/forge-neutral/reviewer-write-boundary.md` and briefs forge-neutral/20–25 and 28–31: where a cell keeps its dispatch claims becomes a store resolved in the desk-tool layer — a directory on the host (`file`, plain host processes) or the same store served over HTTP (`service`, anything in a container or pod) — so the reviewer role needs repository read only. The forge-ref claim store is planned for removal over one release window: release N keeps it only as what an unset `ASSAY_CLAIM_STORE` resolves to, under a boot notice; release N+1 deletes it and refuses an unset key. Authoring only — no tool behaviour changes until the spec is approved and the briefs land.
- Every desk write verb (`deskpost`, `deskreply`, `deskpr`, `deskfile`, `deskevidence`,
  `deskflip`) now stamps an `On-behalf-of: human:<login>` composite-identity trailer —
  GitLab's "service account on behalf of `@human`" model, generalised to the shared-App
  fleet — resolved exclusively from the roster's config-home file (never an environment
  variable) and refusing (exit 5) rather than writing without one. `--dry-run` prints the
  trailer it would have written. See `docs/on-behalf-of.md`.
- New **`server-controls`** stream (parked, pending human scope approval): the server-side control
  posture, reframed. It replaces the impossible ask "provision fine-grained privileges" with the
  three primitives a forge actually offers — a uniform ruleset menu, one readable ruleset API
  (retiring classic protection, `#1020`), and required status checks reported by a runner the
  policed party cannot control. Ships a scoping doc, the design record `DR-server-controls`,
  and five briefs: a uniform-ruleset audit, readable standardization, the required-check enforcement
  pattern (with its self-attestation caveat), a credential/identity decision-dependency note
  (`#900`/`#903`/`#942`), and a reference cross-operator anti-collusion check for the `#997` residual
  that remains after `require_last_push_approval` already enforces author≠approver.
- New **desktools-v2** stream (proposed — `status: parked`, citing a `**Status:** draft`
  scoping doc): the architectural rebuild of the desk tools' forge access, on three first-class
  principles — **custody** (explicit minted-token only, key-presence as the custody boundary,
  the desktop made to behave like a locked container), **the read path covers statusgen across
  the `deskread` verb boundary** (the migration stays with the sibling `forge-neutral` brief
  that owns it; this stream brings `statusgen/**` under the ban and then holds the zero), and
  **purpose-built queries** (typed access-pattern operations, one tuned query per backend:
  N+1 → one consistent snapshot).
- New `auto-triage` stream (`status: parked`, draft spec): a scoping doc + four briefs that
  automate the RESPONSE to an all-stop CI signal — identify the culprit, file the bug, and
  either open a fixing draft PR (mechanical) or route it (judgement), with a never-invisible
  watchdog escalating any red no responder acted on. Authoring-only; nothing is implemented,
  no gate is weakened, and the three autonomous-action briefs are `gate: human` pending
  `DR-auto-triage` approval. (#1119)
- New stream `fresh-views` (parked, pending approval): a scoping doc plus 6 `todo` briefs
  proposing that every derived view — a dispatch plan, a board snapshot, the open-PR set, a
  mergeability verdict, a mirrored version string, a reconciled lifecycle cell, a landed sha —
  be treated as a pure function of `main` at a known sha, stamped with its input sha, and made
  to refuse (not guess) when that input is stale. Authoring only; implements nothing. (#334)
- Next-up (`eligibleBase`) and the drive frontier (`briefFrontierState`) now
  read the evaluator's verdict for a brief-v1/v2 brief's `depends:`/`gates:`
  decision, instead of walking `depends:` in isolation — closing a gap where a
  brief-v2 brief fell through to the legacy whole-wave rule and its
  `depends:`/`gates:` were never consulted at all.
- Scoping doc plus ten briefs. Authored-only; no tool changed. Boundaries with `forge-neutral`,
  `desktools-go-git`, and `desk-tools` are stated in the scoping doc.
- The same stream scopes **one outbound-write check at the forge write seam**, keyed on the
  target repository's configured visibility, with a deployment-supplied callout on the existing
  callout plumbing — so what may be written to a forge is enforced by the tools rather than by
  skill prose. The scoping doc tabulates which outward verb runs which check today.
- `--lint` NOTICEs a `[eligibility-could-not-check]` line naming any brief
  held by an unresolvable edge (an unpublished cross-repo alias, an absent
  sibling checkout, a forge-backed target), so the gap is visible on a full
  lint run, not only in a dispatcher's output.
- `cellctl check` gains scrubbed-specific rows: config-home real-directory + 0700 mode, every PEM
  regular/non-symlink/0600, the roster's `ASSAY_ALLOWED_REPOS` scoped to exactly one repo, and
  harness login proven under the cell's own home.
- `cellctl new --kind scrubbed`: a host-local harness cell whose launch environment is fully
  COMPOSED (`env -i` plus an explicit allowlist) rather than inherited — nothing from the
  launching shell reaches the harness. Its own real (never symlinked) config home, scoped to
  exactly one repo, with its own harness login.
- `cellctl smoke <cell>`: a one-shot, tool-free, read-only readiness probe for a scrubbed cell —
  the harness answers `READY` or the verb names what it said instead.
- `cellctl status <cell>`: `running <session>` / `stopped` / `stale-lock <pid>` — a read, not a
  check.
- `desk-supervision` stream: three new briefs (13-15, renumbered from 10-12 to clear a collision
  with the workflow-App-landing lane that landed on main as 10-12) scoping the **worker-operations
  vitals** delta — a self-reported `resource` block (context-%, tokens, session age,
  subagents, model) filling the reserved `desksupervise-status-v1` `tokens` stub, a
  budget-driven graceful recycle of a healthy-but-full worker, and a local supervisor host
  (`cellctl` + a fleet-vitals aggregation contract) so supervision reaches non-k8s operator
  desks.
- `deskdispatch --kit worker-objective`: an alternative objective-plus-status-map worker
  prompt kit, measured two-arm against the procedural `worker` kit with `tools/skillbench`
  over a five-task fixture set (`tools/skillbench/fixtures/worker-kit/`); not the default,
  `--kit worker` is unchanged. Report and decision: `docs/streams/desk-supervision/08-report.md`.
- `deskpathguard check` — a PR that touches a protected verifier path (a brief's `## Verify`
  table, `.github/workflows/**`, `.claude/guardrails/**`, `tools/skillslint/**`, a
  `verify.d/**` scripted-rows directory, or `**/testdata/**`) alongside a non-brief,
  non-fixture file is labelled `wrote-to-the-test` and force-gated to `gate: human` at the
  status transition, unless the author is the desk/verifier identity, the PR carries a
  `regen:` label, or the diff is pure authoring (brief/fixture files only). See
  `docs/protected-paths.md`.
- `deskpathguard rederive` — verify-desk's pre-change re-read: reports a Verify-table row
  present only at HEAD (not at the merge-base with `origin/main`) as `author-added`, and
  runs every other row using the merge-base's own command/expect text.
- `docs/statusgen-lint-reach.md`: a short contract stating exactly what
  `statusgen --lint` may reach on the network, with and without `--forge`.
- `docs/streams/decisions/DR-workflow-app-landing.md` and a three-brief
  `desk-supervision/10 → 11 → 12` chain proposing that a workflow change land as
  a single workflow-only pull request the workflow App writes, replacing the
  staged-copy hand-landing that stalls and drifts. Author-only: nothing is
  implemented, the record is `proposed`, and the briefs are `blocked` pending a
  human ruling.
- `docs/streams/measured-status/`: authored the `measured-status` stream — a scoping doc, six
  `todo` briefs, and a `DR-independence-gate` design-decision record scoping the
  derive-not-assert and verifier-independence fixes for #1216, #1171, #1065, #862, #1116, #336.
- `pr-review-desk` runs the check at every new head before an APPROVE verdict; `verify-desk`
  runs the re-derivation on a labelled brief before any row.
- `spec/workflow-pattern-v1.md`: the workflow-pattern schema — a node contract
  (kind, role, inputs, outputs, evidence, effects, budget, outcomes), risk
  class as a declared `risk-input`, and the integration-check `join` node —
  plus `schemas/workflow-pattern-v1.json`, so a workflow's shape can be
  reviewed as one versioned artifact and validated by an independent tool.
- `spec/workflow-patterns/implementation-v1.yaml` and `research-v1.yaml`: the
  two workflow patterns the fleet already runs, stated as reviewed pattern
  files rather than left implicit across desk-skill procedure text.
- `statusgen --eligibility` (`--json` for the full structure): the eligibility
  evaluator computes, per brief, `eligible` / `held` / `eligible-with-notice`
  from its `gates:`/`feathers:`/`depends:` declarations — three-state
  (`satisfied` / `unsatisfied` / `could-not-check`), offline by construction.
- `statusgen --lint --changed-only <paths>`: a local pre-push convenience, on
  the same plumbing `--changed` already has, that demotes a pre-existing
  defect outside a stated path set — in the DAR-sync, stream-cap,
  stream-source, register-integrity and verify-script-diff checks — from
  PROBLEM to NOTICE, and prints a banner naming exactly that (every check
  still runs across the whole tree). Refuses outright, non-zero, with no
  override, when it detects it is running inside the CI gate.
- `statusgen --lint` flags an App-authored Evidence row whose on-behalf-of annotation
  names a login outside the roster's human map as a hard PROBLEM, and one with no
  annotation at all as a hard PROBLEM once dated at or after the write path's own
  landing date (a NOTICE for a row grandfathered from before it — no write path existed
  yet to stamp it).
- `statusgen patterns --lint [--root DIR]`: validates every
  `spec/workflow-patterns/*.yaml` file against the schema and five MUST rules
  — `pattern-effect-target-not-owned` (a non-`effect`-kind node's effect
  target must be among its own outputs), `pattern-effect-exceeds-role` (a node
  cannot declare an effect kind its role does not hold),
  `pattern-review-same-role` (a review node cannot share the role of whoever
  it is reviewing), `pattern-join-not-check` (the integration check must be a
  `check`-kind node), and `pattern-risk-input-missing-verdict` (all four
  risk-class verdicts must be mapped). All five are registered in
  `statusgen enforcement-status`.
- `statusgen verifyrun`'s witness Runner cell annotates an App/bot runner with
  `on-behalf-of human:<login>` when a principal resolves.
- `the-desk` and `intake-desk` now state a shared carve-out: when an inbound issue is authored by the driver identity itself, its body reads as an instruction to the desk, and it names no existing work item, the coordinator (`the-desk`) acts on it directly — receipt comment, dispatch behind draft PRs, and the intake register entry filed in the same deliverable PR — instead of it routing to `intake-desk`. (#1258)
- `tools/release/check-spec-header-version.sh`: a release-time check that
  `spec/brief-v1.md`'s `Describes reference implementation:` version matches
  the tag being cut — the release-time floor brief-13's Task 4 named but
  never shipped, closing the recurring staleness class (v0.8.0-vs-v0.19.0,
  then v0.22.0-vs-v1.0.9). Wiring it into `release.yml`'s `guard` job is
  **staged, not yet activated** under `tools/release/` (see
  `tools/release/README.md`) — the worker-desk App cannot push under
  `.github/workflows/` (server-side workflows-scope block); a
  workflows-capable identity applies `tools/release/release.yml.patch` to
  activate it. (#1192)
- `topology.yaml`'s `apps:` role names are now compiled into a derivation
  (`topologyAppRoles`) bound to the source by `TestTopologyValuesMatchSource`,
  the same derive-or-diff convention as the rest of `topologyvalues.go`.

### Fixed
- A `fixed-here` claim that is genuinely DISPROVED now names WHICH check actually failed —
  `tree-lookup` (does the path resolve under the root?) vs `diff-lookup`/`diff-deletion-lookup`
  (does the diff touch it, or name it as a delete/rename?) — instead of one conflated sentence
  that left a reader unable to tell which predicate returned false. (#1077)
- Added test coverage proving `--lint` (no `--forge`) makes no forge process
  start against a fixture that genuinely exercises dead-claim decay's forge
  read, and that every check reading through the run's forge reader renders
  could-not-check offline rather than a fabricated clean result.
- The `capability:dispatch-worker` row of `plugins/assay/references/codex.md` and the Codex
  dispatch config step in `docs/adopting-assay.md` §3 are corrected against **codex-cli 0.154.0**:
  the `[features] multi_agent` flag has graduated (`codex features list` reports `stable`/`true`)
  and no longer gates the subagent tools — a child spawns with it set `false` — so the retired
  "`multi_agent` off → dispatch unavailable" reading is replaced. The convenience-degradation
  floor still stands as the design contract, now keyed to the reachable trigger: the `[agents]
  max_concurrent_threads_per_session` concurrency cap. The dependent per-skill degradation rows
  are re-worded from "if `multi_agent` is off" to "where parallel dispatch is unavailable" for
  consistency. (#939)
- The `statusgen-board` workflow now also runs on `changelog/**`, so a release commit that clears changelog fragments a brief still cites reddens the lint on its own commit instead of on an unrelated later PR (#722).
- `cellctl gen_shims`: a shimmed desk verb's `gh` subprocess now authenticates. gh's ambient
  credential (keychain on macOS, hosts.yml-adjacent elsewhere) is keyed to the REAL `HOME`, so it
  is resolved *before* the shim swaps `HOME` to the cell home, then threaded through as `GH_TOKEN`
  — the verb's own config/state stays isolated to the cell exactly as before. An explicit
  `GH_TOKEN`/`GH_ENTERPRISE_TOKEN` already set by the caller is never overridden.
- `deskpushguard`'s foreign-commit/merge-masquerade base check no longer hardcodes the remote
  name `"origin"` when resolving the pushed branch's base (`refs/remotes/<remote>/main`) or
  excluding a branch's own already-published commits. It now resolves the ACTUAL push-target
  remote from the pre-push hook's own `<remote-name>` argument (falling back to `"origin"` only
  when that argument is absent), so a worktree whose `origin` remote points at a different repo
  than the branch actually being pushed no longer has every genuine commit on the branch
  misreported as a "foreign commit dragged in from a sibling branch" against the wrong repo's
  history. (#1201)
- `docs/streams/forge-gitlab/README.md`'s Briefs table lists brief 13 (the
  per-row degrade for `classifyPR`'s whole-sweep error returns) as `todo` even
  though its fix, regression tests, and changelog fragment already merged to
  `main` in #1084 — the board-honesty `already-merged-unflipped` class. This
  PR does not hand-flip the Status cell: that table's single writer is
  `statusgen`, never a hand-committed hunk. Neither of `statusgen`'s two write
  paths performs the flip today — `regen --readmes` preserves lifecycle cells
  by design rather than deriving them, and `reconcile --backfill --apply` (the
  verb that would write one) is not wired into this repo's CI — so the row
  stays `todo` pending that follow-up. (The PR's own title previously read
  "flip board row to implemented" — retitled to match: no flip happens here.)
- `newbrief`'s own row write into a `board: generated` stream's Briefs table now reuses `regen --readmes`'s exact renderer instead of a separate hand-rolled writer, so the row it writes is byte-identical to a fresh regen and never immediately trips `--lint`'s "hand edit to a generated table" — closing the only remaining gap in adding a brief-v2 stream's row (`statusgen regen --readmes` already covers the row-less-brief case).
- `plugins/assay/references/standing-note.md` now carries the
  `<!-- assay:harnesslint non-matrix-reference — ... -->` declaration that its harness-neutral
  siblings `desk-shell.md` and `tick-contract.md` already carry. Without it, `tools/harnesslint`'s
  `bindings` mode mistook the file (added 2026-09-14) for a capability-binding matrix and checked
  it against the full closed vocabulary it was never written to satisfy, redding an independent
  `harnesslint bindings plugins/assay/references` pass. (#1182)
- `spec/brief-v1.md`'s header freshened from the stale `statusgen v0.22.0` to
  the actual current release, `v1.0.12`. (#1192)
- `statusgen --consumers`'s `fixed-here` gate no longer wrongly DISPROVES a site token written
  with Markdown emphasis — a path wrapped in backticks or `**bold**` — around it: the wrapping
  delimiters are now stripped before the token is resolved as a path, so a backticked entry
  corroborates exactly like its bare equivalent. (#1077)
- `statusgen newbrief` now DETECTS a target stream's brief schema off its existing briefs and, for a brief-v2 stream, emits `schema: brief-v2`, the hierarchical `<cell>:<repo-alias>:<stream>:<NN>` `brief:` id (resolved from `docs/streams/graph-repos.yaml`, the same registry `--lint` validates against), and derives the wave from the stream's OWN existing waves rather than a hardcoded 0 — several live streams (forge-neutral, desk-tools, …) start their first wave at 1, not 0.
- `tools/ci-load/activation/ci.yml`, `assay-statusgen.yml`, and `evidence-automerge.yml`
  refreshed against current `.github/workflows/` so the staged trigger/concurrency edit no
  longer silently reverts three independent fixes already landed on `main` (the `tools/desk`
  `go test ./...` leg, `fetch-depth: 0` on the statusgen lint checkout, and the
  evidence-automerge script-based refusal decision + default-branch checkout). `ci-load.diff`
  regenerated to match. (#1187)

### Changed
- **Behavior change, not just a new field:** a `brief-v2` todo brief stops
  being whole-wave gated. Previously every lower-wave sibling in the same
  stream had to be `done`/`verified` before a v2 brief was eligible; now a v2
  brief is gated by the evaluator's verdict on its own `depends:`/`gates:`
  alone, so an unfinished wave-0 sibling no longer holds it. This is a
  loosening on any brief-v2 tree with unsatisfied whole-wave gating but
  satisfied `depends:` — on this repo's own board it admits two previously
  held briefs (`apps-installer/02`, `desk-supervision/08`) to Next-up.
- Draft scoping doc `docs/streams/forge-neutral/reviewer-write-boundary.md` and its briefs amended for three rulings of 2026-09-17. A host process that points the `file` claim store at a network filesystem gets a notice on every boot — whether the filesystem is identified or its type cannot be determined — and is never refused on that ground; the single-host declaration remains the refusing guard (brief forge-neutral/23). Brief forge-neutral/30 becomes the release-N cutover only, and the release-N+1 deletion of the forge-ref claim store becomes its own human-gated brief, forge-neutral/32. The claim serve mode stands alone, with a six-verb contract specified independently of its transport and one conformance row proving it (brief forge-neutral/24). Authoring only — no tool behaviour changes.
- Every write verb that appends the on-behalf-of trailer (`AppendOnBehalfOf`) now strips
  any On-behalf-of line the caller-supplied body already contains, wherever it sits,
  before appending its own — a caller can no longer plant or shadow the annotation.
- The `gates:`/`feathers:` "(reserved, not gating)" `--lint` NOTICE is
  retired: those fields are executed as of this change, so restating
  "reserved" would be false.
- `LoadHistory` (the `docs/streams/.history.jsonl` reader) is memoised on
  `(path, mtime, size)`, the same shape the brief-file parse memo already
  uses — one `--lint` previously re-read and re-decoded the same history log
  from multiple call sites in a single run.
- `deskpr edit`'s noop compare now strips a prior on-behalf-of trailer from the PR's live
  body before comparing against the caller's replacement, so a trailer-only delta still
  noops instead of re-posting.
- `docs/codex-smoke-protocol.md` **Step 5** is re-baselined from the now-unreachable
  `multi_agent`-off precondition to forcing `max_concurrent_threads_per_session = 1`, per the
  2026-09-17 human ruling on `#939`. It asserts the serial-dispatch floor observably: the fan-out
  degrades to serial (non-overlapping child lifetimes) with every item completed and every
  guarantee — isolation, evidence, review — intact, and carries a re-open condition for when a
  future CLI re-gates dispatch. (#939)
- `docs/lifecycle.md` §Review gates names which `implementation-v1` pattern
  node each existing review gate is.

## v1.0.12 — 2026-09-17

### Added
- Regenerated `.github/assay-statusgen.reconcile.patch` (still staged, not
  applied — a workflow-file push needs the workflows scope) as an actual
  `git apply`-able unified diff; the prior version's bare `@@` hunk headers
  carried no line-range info and could not be applied as its own instructions
  said.
- The `statusgen --lint` PROBLEM-diff guard fails closed on a root it cannot evaluate: when
  `statusgen` exits nonzero with no `PROBLEM:` line (a structural failure — no
  `docs/streams` tree, a stream dir with no `README.md`, an incomplete checkout) the landing
  is could-not-check (exit 6) rather than treated as clean, so the guard cannot silently
  no-op against the scratchpad/bare-cwd roots the landing path is often handed.
- This repo's own `docs/streams/*/README.md` boards get a `statusgen reconcile
  --backfill --report` pass: `docs/streams/board-drift-2026-09-16.md` records
  every brief where the hand-said lifecycle cell disagrees with what PR history
  (plus the declared history-only backfill fallback) now derives, for a human to
  resolve by linking or accepting.
- `desk-containers` briefs 09–11 (#1193): a scrubbed host-local `cellctl` cell kind with `smoke`, `status`, a session lock and a stricter `check`; the Go port of `cellctl` on `deskkit` with the bash kept as a parity oracle (human-signed cutover); and the retirement of the out-of-tree bridge — `CELL_KIND=local` refused with its migration line, one `cellctl` on PATH.
- `deskclose triage -R <owner/repo> <N> --disposition {not-planned|human-decided}` closes a triaged idea-ISSUE through the resolved forge under the role App — the intake front door's "close, no fix PR" exit that had no sanctioned path (a raw `gh issue close --reason "not planned"` is denied to an auto-mode session). Because a close verb is an authority surface, each disposition authorizes on an artifact deskclose FETCHES and VERIFIES, never a caller flag: `not-planned` closes only on a `<!-- desk-triage v1 -->` marker comment on the issue that is authored by a roster-trusted account and not minimized (a bare marker string, a minimized one, or one by an untrusted author does not authorize); `human-decided` closes only on the human's own ruling comment on the issue (`--decision <url>`), fetched and its author verified as the roster-pinned blessing authority — a blanket ruling grant never stands in for it. `--tracker` is never authority: when given it must name an EXISTING item (verified by a read), and it is required for `human-decided` (the close NAMES the continuing work). A `needs-decision` item is refused in both dispositions and `not-planned` also refuses a `human-decided`-labelled item (the control that keeps the verb from closing an undecided or unruled item); a pull-request target is refused; an already-closed issue is an idempotent no-op; an unreadable state is could-not-check (exit 6), never a guessed close. No marker writer ships in this PR, so `not-planned` fails closed until intake stamps a trusted marker — by design, never a close on an unverified signal. The other four modes are unchanged. (#1207)
- `deskevidence` refuses a landing whose target path resolves outside `docs/streams/` —
  catching a stray root-level file before it lands, not after.
- `deskevidence` refuses an Evidence landing that would introduce a new `statusgen --lint`
  PROBLEM, diffed against the landing worktree before the change so a pre-existing red
  elsewhere in the repo never blocks a clean landing (exit 5, naming the PROBLEM lines).
- `deskpr create --check` runs every LOCAL gate a real create would run — flags, branch
  state, the `Brief:`/`Issue:` trailer, the secret scan, the public-repo self-containment
  scan, the push-transport gate — and stops before minting a token or opening any
  connection; `update` and `edit` gain the same flag for the gates that do not require the
  forge-held PR body. `deskreply`'s `--dry-run` is widened from the `--workpad` path to the
  plain reply path.
- `graph-execution` stream: an approved scoping document and eight briefs for executing the brief graph — an eligibility evaluator that makes `gates:`/`feathers:` gating with a stated reason, a versioned workflow-pattern schema with node execution contracts and three reviewed patterns (implementation, research, signal-triggered), a deterministic evidence coverage rule with an `observe` evidence kind, a recovery contract for effect-bearing nodes in `drainloop`, an offline two-pattern experiment on frozen fixtures, run records with a replay/learning loop, and flow instruments (service/wait split, CI-slot saturation, gate catch/override). Design only; no gate moves and no behaviour changes in this PR.

### Fixed
- The GitHub forge backend never hands back a shorter open-issue listing with a nil error: a body cut off mid-transfer or a zero-byte body is an error, and the page walk follows the forge's own `Link: rel="next"` instead of stopping on any short page. (#1032)
- The trailer grammar is now stated inline in the skills instead of citing `docs/streams/derived-board/spec.md`, a path `statusgen init` never writes into an adopter tree — so a reviewer asking "does `Issue:` substitute here?" has the answer in the skill they were given. The dead citation is swept from `worker-desk`, `verify-desk`, and `author-brief`. `upgrade-assay` and `install` now document the re-pin landing path (a re-pin files a tracking issue and carries `Issue: #<N>`), and `docs/desk-tools/deskpr.md` records that `deskpr edit` cannot change a trailer in place. (#1224)
- `composability/02`'s Verify row 1 (`grep -rn 'TODO composability/02' --include=component.yaml . | wc -l` = 0) had
  regressed to 5 on `main`: a merge race between PR #953 (this brief) and the concurrently-landed
  PR #952 (composability/04, harness-as-key) left three new harness manifests
  (`harness-claude-code`, `harness-codex`, `harness-cursor`) with placeholder `inverse:` text.
  Wrote the real, non-TODO reverse prose for those 5 apply steps — descriptive text only, no new
  `deskdisable` executor registered — and corrected the stream board's brief-02 row, which PR #953
  never flipped, from `todo` to `implemented`.
- `deskdispatch` claim-acquire no longer requires a GitHub App ID on a GitLab-only roster: when the target repo's forge resolves to GitLab (`ASSAY_REPO_FORGES=<slug>=gitlab`), the claim child is now authenticated with the same GitLab role PAT custody the other write verbs use (`deskpost`/`deskflip`), not the GitHub App installation-token minter there is no App to mint against. GitHub-resolved repos keep the App-mint path unchanged, and an explicit `GH_TOKEN` still wins for both. Previously a review (or worker) dispatch on a GitLab project failed closed with `no App ID for App "reviewer-app"` before any claim was taken. (#1203)
- `deskreply` refuses, before any read or write, a `--body-file` whose size exceeds the cap (decided from the file's metadata, so a runaway body is never loaded into memory), a body over the forge's 65,536-character comment limit, and a body carrying the workpad marker line more than once — the signature of a workpad rebuilt by appending its predecessor to itself. The `pr-shepherd` and `worker-desk` skills say to write the workpad body file fresh (`>`), never append (`>>`) or re-read the old workpad. (#1195)
- `desksupervise` (`status --stops` / `tick` / `run`) and the `BranchMoved` liveness probe now list refs authenticated as the session's role against the forge the roster names, instead of an anonymous read of a hardcoded `github.com` — so a private board root no longer fails with `authentication required: Repository not found`, and a GitLab forge is dialed with its own host and git username. The custody credential is presented only to the resolved forge kind's own canonical instance (`github.com` for GitHub, the `GITLAB_API_BASE` host for GitLab), never to the origin host of an unrelated checkout, so a cross-repo sweep can no longer send a GitHub App token to a GitLab host or vice versa; the host is validated as a bare hostname before use. A missing token, an unresolved forge, or an unknown instance host is could-not-check (exit 6), never an empty snapshot or a SaaS-host default. (#1197)
- `issueboard` no longer retires a placeholder because its issue was merely absent from the open-issue listing: `RETIRE` now rests on a positive per-issue `closed` read, an unreadable state is could-not-check (exit 6, issue named), and an absent issue that reads open proves the listing partial and refuses the whole sweep. A slow/partial third sweep had flipped ~236 still-open issues `NONE→RETIRE`. (#1032)
- `pr-review-desk` check 2 now accepts a PR body carrying **either** `Brief: <stream>/<NN>` **or** `Issue: #<N>`, matching the grammar `deskpr` already enforces (`deskkit.ParseTrailers`). Previously the reviewer named only the `Brief:` form and bounced any `Issue:`-trailered PR as if it had bypassed the gate — which made adopter re-pin PRs (a pin bump delivers no brief, so it carries `Issue: #<N>` by construction) unmergeable. The false "a trailer-less PR reaching review means the refusal was routed around" note is corrected: an `Issue:`-trailered PR satisfied the gate legitimately. (#1224)
- `statusgen --scan-issues` now reads a repo's OPEN issues through the native forge (via the `deskread` verb on the `Forge` seam) instead of shelling out to `gh issue list`. The native client attaches the correct per-installation App token explicitly per request, so the read is immune to the three ways the `gh` shell-out lost or mis-scoped its token: a replaced `HOME` hiding gh's ambient credential (#1145), a token attached only to a child literally named `gh` rather than to a script that itself shells `gh` (#1146), and one inherited `GH_TOKEN` forced across every scan repo so a repo on a different App installation 401s/404s (#628). This unblocks the intake-desk issue-lane drain (`scanloop`), which 401'd on every rostered repo. Supersedes the narrow env-passing patches implied by #1145, #1146, #628. (#1223)
- `statusgen reconcile`'s PR-trailer join matched a brief-v2 hierarchical id
  (`<cell>:<repo>:<stream>:<NN>`) against a PR's short `Brief: <stream>/<NN>`
  trailer as a bare string, so the two never matched — every PR-derived lifecycle
  cell on a brief-v2 tree stuck at `todo` no matter how many trailer-carrying PRs
  had merged. Both sides now reduce to the same `<stream>/<NN>` key before the
  join.
- `tools/desk` tests no longer leak into `$TMPDIR`: the six `TestMain`s that deferred the roster-fixture cleanup past `os.Exit` (`deskpr`, `deskfile`, `deskreply`, `deskroster`, `deskadvisory`, `deskpushguard`) now run it explicitly and fail the package if the fixture HOME survives; the host `GOMODCACHE`/`GOPATH`/`GOCACHE` are pinned before HOME is relocated so fake-binary builds stop filling each fixture with a fresh module and build cache; the cleanup makes the tree writable before removing it and reports a failure instead of dropping it; and the fake `desktoken` binaries (`deskpr`, `deskreply`, the fleet harness) write their token files inside a fixture directory that is removed. A static test in `deskkit` pins all three shapes. (#1195)

### Changed
- New `deskfile new --force-file --reason <r>` override raises the rate for one filing so a human can always raise an issue even when the rate is spent. It is distinct from `--force-new` (which bypasses the dedupe search): `--force-file` never weakens dedupe. The override is audit-logged with the reason and the filing identity, and the filing is still charged, so it neither resets nor erases the rate count — the next unoverridden `new` still sees the full history. The rate still counts over the audit log's session+tool+verb+repo fields, so a rotated session id leaves a forensic trail rather than erasing the count. (#1204)
- The `windows-ci-leg` workflow now runs only on version-tag pushes (`v*`) and on `workflow_dispatch`; the `push` and `pull_request` triggers that ran the Windows leg on every branch push and every PR are dropped. Each release still gets its Windows build proven at tag time, and a maintainer can run the leg on demand, without spending a `windows-latest` public runner on every push. (#1215)
- The measured top body/schema refusal classes in `deskpr` and `deskreply` now name the
  offline check that would have caught them for free (`deskpr … --check`, `deskreply …
  --dry-run`), continuing the same hint `deskpost`'s refusals already carry.
- `deskfile new`'s issue-filing cap is now a per-window **rate** (N filings per window) rather than a compiled per-session tally, and both knobs are env-fixable with no recompile: `ASSAY_DESKFILE_NEW_RATE` (integer) and `ASSAY_DESKFILE_NEW_WINDOW` (a Go duration such as `24h`), falling back to the shipped defaults (3 per 24h) when unset. An unparseable value falls back to the shipped default AND prints a `NOTICE` naming the bad value — it never silently disables the cap. When the env knobs RAISE the pace, the effective rate/window are recorded on the filing's audit line (`rate-config: <n> per <window> (env)`) so an env-raised filing is never byte-identical to a default-rate one.
- `pr-review-desk` generated-table bounce now admits a NARROW authoring case: a hunk that ADDS
  brand-new brief rows (modifying no existing row) is admitted when each added row is honest-base —
  `Status` = `todo`, empty `Verified`/`Reviewed` — AND the rows reproduce exactly under
  `statusgen regen --readmes` (run in a throwaway worktree at the PR head with the pinned CI
  `statusgen`, never one built from the untrusted tree). This unblocks brief-authoring PRs, which
  must carry the regenerated rows or `statusgen --lint` fails on the PR head (dangling
  depends/unblocks/consumers references). The honest-base check is what blocks forgery: regen
  PRESERVES the `Status`/`Verified`/`Reviewed` cells for any row in the region — including a row the
  PR just added — so "byte-identical to regen" can never certify those columns; only requiring
  `todo`/`—`/`—` on an added row (stamps come later, from the verifier/reviewer) does. Any change to
  an existing row, and any stamped or non-`todo` row inside the markers, still bounces. Reviewers
  never hand-fix the table.

## v1.0.10 — 2026-09-16

### Added
- **`forge-neutral` brief 19 — human-only surfaces made server-side (the closing brief of
  #992's five-brief series).** For each surface a human currently performs by hand — merge
  to a protected branch, a workflow-file push, ruleset/branch-protection edits, repo/CI
  variables, and App/OAuth installation — states plainly whether it is genuinely
  server-side-enforced today or held up by convention alone. Finds merge-to-protected-branch
  is the latter: this repo's role Apps hold `contents: write` + `pull_requests: write`,
  which is sufficient to merge outright, and the live ruleset read confirms
  `required_approving_review_count: 1` carries no restriction on which identity supplies the
  approval. Proposes a `human-approved` required status check (triggered on
  `pull_request_review`, checking the reviewing login against a trusted-human allow-list at
  the PR's head SHA) to close the gap server-side, and specifies fixture-only Verify rows
  that must never be pointed at this repo's own `main`. Doc only; no tool or workflow
  behaviour changes in this PR — the workflow file and the ruleset edit are named follow-on
  work for a human to land.
- A shared typed-reference parser (`deskkit.ParseItemRef`) and two typed forge operations
  (`ListCommentsTyped`, `CloseIssueTyped`) alongside the existing `GetIssueTyped` /
  `PostCommentTyped`, so a reference that resolves for one desk verb resolves for all of them
  rather than each growing its own spelling.
- Cell-gateway bypass battery (`go test ./cmd/commsgw/ ./cmd/commsloop/ -run Bypass`): cross-layer negative-path drills that inject a fault above one guard with the layer above it bypassed or fooled — client preflight skipped, unauthenticated / rogue-CA / forged / replayed / expired peers on the real mTLS + A2A transport, rate-limit breach, kill switch armed (real `deskkit.Guard`), prose injection on both directions, mis-routed dispatch against the role fence, budget exhaustion, containment escapes by a real ACP decider child, and violations planted past every inline layer for the sweep — each asserting the distinct refusal and the signal it leaves.
- Positive-path drills for the eight cross-desk hand-off shapes (advise, request-act flip, blocked, finding, request-act verify, routine relay, liveness, depends): delivered, pre-checks pass, queued for the right role's lane, the right role's fired session is allowed to act while the wrong role's is refused by its own profile.
- The commsloop fake ACP agent gains `act` and `contain` modes (a real `rawInput.command` tool call; a decider that attempts an fs read and a tool permission before answering).
- The review-tick conformance walk's OFFLINE half: the GitLab contract corpus
  (`TestForgeGitlabGolden`) now pins, per tick verb — board read, review dispatch, verdict,
  escalation filing, workpad edit, Evidence landing, ready-flip — at least one refusal the verb
  is built on, next to the success path it already pinned: a forbidden open-changes list, a
  project payload with no visibility field, an unnamed label, a forbidden dedupe search and
  issue create, a forbidden thread read, an absent Evidence target, an append-only shrink, a
  marker-only draft title, a merge request still draft after its marker is cleared, and an
  empty required-checks branch. Three mutation entries prove the new refusals are load-bearing.
- The stream's pilot report gains the per-verb conformance table (§7) with every LIVE cell
  left `could-not-check — live walk not yet run` and the offline column filled from the
  goldens; the issue → brief map covers the 2026-09-15 field reports.
- `CELL_FF_ROOTS=1` in `cell.env` makes `cellctl desk` fast-forward, at boot, every stream root that can move without a decision: on a branch, that branch has an upstream, the tree is clean, and it is 0 commits ahead. Off by default, because the roots are the operator's own checkouts rather than cell-managed worktrees. A root that is dirty, ahead, detached, or has no upstream is never moved and is named on stderr.
- `Forge.ReopenIssue(repo, number)` on both backends (inventory op 46) — `CloseIssue`'s inverse, one request on the issue endpoint, no state reason; golden-pinned on GitHub and GitLab.
- `cellctl check` now reports CELL_ROOTS stream-root drift: one row per root naming the branch, its upstream, and how far behind it is. Nothing in cellctl has ever advanced the roots, so a root sitting behind its upstream degrades every desk verb that reads it while raising no error anywhere. Reported as a non-fatal `warn` — staying current is an operator step, not a precondition cellctl can assert — so it does not change `check`'s exit code. Roots that are ahead or dirty are named too.
- `cellctl new --kind container` registers an existing container launcher for `ls`, `check`, `desk`, coordinator-only `up`, and `down`. Calls retain model-pin checks and exclude inherited forge/model credentials from the launcher's environment; no host worktrees or credential symlinks are created.
- `commsgw` journals every refused inbound — on both the loopback socket and the A2A transport — as one `kind:"refused"` line on the queue's `journal.log`: the distinct refusal kind, the lane pair and sender identity as presented, the gateway cell, a timestamp and a digest of the raw bytes; never the payload (#1165).
- `commsloop sweep` counts gateway refusals per presented sender and per presented destination lane and reports a `refusal-threshold` finding at or over `--refusal-threshold` (default 10 per sweep window; negative disables); the report line now carries `refused=N` (#1165).
- `deskclose self-withdraw` — the authoring App closes its OWN open draft (`--because abandoned`, or `--because superseded --by <ref>` recorded not verified), pinned by login AND roster bot id; refuses a non-draft, another author's change, a login-only match, an unpinned roster id, and anything carrying `needs-decision`. Cites no ruling and consults no disposition record: an author's own withdrawal, nothing wider.
- `deskclose verify-gate-refire` — the verifier session reopens, comments on, and re-closes a CLOSED issue carrying `verify-gate` (`--reason` mandatory) so the card's close event fires again; refuses every other role by name, an unlabelled item, and a pull request. Explicitly not the human sign-off: a bot's close of a verify-gate issue is reopened by the repository's verify-gate close workflow regardless.
- `desklabel add|rm <owner/repo> <number> <label>` — a role-keyed one-label verb. Every
  label-carrying write in the desk was bundled into a bigger verb's fixed set (deskflip's queue
  swap, deskclose's `superseded?` proposal, deskdisposition's `disposition:*` record), so a stale
  marker — the `superseded?` a dispute leaves behind — could only be cleared by a raw, unscoped
  forge call. `desklabel` sets or clears ONE label through the resolved forge under the session's
  own App role, against a closed vocabulary: the topology decision-owed labels (`needs-decision` /
  `question` / `needs-human`, read from the topology loader, never restated) plus `help wanted`
  for any role; `superseded?` and the `disposition:*` family for the worker; `authorization-needed` /
  `approval-needed` for the reviewer; `human-decided` refused for every role; anything else
  refused (exit 5) naming the label, its owner and the session's role. The check runs BEFORE any
  forge call; the role is read from the session (`DESK_LOOP`), never from a flag. The target
  kind comes from the seam's own read (`GetIssue`, or `GetIssueTyped` under `--kind issue|mr`
  for a GitLab project carrying both `#N` and `!N`); a present/absent label is a no-op with no
  write; `--dry-run` stops before the write; `vocabulary` prints the table. Both forges are
  covered end to end through the real backends, and the role-ownership guard carries a
  mutation map (`cmd/desklabel/mutations.json`).
- `docs/adopting-assay-gitlab.md` §5a: the GitLab hardening-checklist template (Community
  Edition rows, the `not available — Premium` / `— Ultimate` divergences, the Premium and
  Ultimate swaps) and the auditor's minimum project role per kind.
- `repohardenguard` now checks a GitLab project: the hardening-read operation serves five
  GitLab kinds — `project`, `protected-branches`, `protected-tags`, `push-rules`,
  `approvals` — each a fixed endpoint returning GitLab's own settings document (the two lists
  are walked page by page and refuse at the ceiling rather than hand back a partial array).
  A Premium-only route on Community Edition (`push-rules`, and `approvals` on some
  self-managed instances) arrives as a typed could-not-check naming the tier, never an empty
  document; the GitHub backend refuses the GitLab kinds by name and GitLab refuses the GitHub
  kinds, with zero requests either way. The guard's preflight reads the resolved forge's own
  document (`project` on GitLab) instead of the GitHub `repo` kind, and a checklist Field may
  index into a list (`push_access_levels.0.access_level`).

### Fixed
- A GitLab job declared `allow_failure: true` that fails no longer reddens a head whose
  pipeline succeeded: it maps to the neutral conclusion the forge-neutral reducers already read
  as non-blocking, so a job entry cannot contradict the pipeline entry beside it. A blocking
  job's failure still reddens.
- A `deskevidence` scan refusal now names the origin of the offending bytes — `added by --evidence-file:<line>` — and a landing whose branch copy already carries a secret-shaped run says so on stderr as `pre-existing in <path>:<line>`, so the operator no longer isolates the trigger by hand. Neither message carries the span (#1161).
- A degraded row's RENDERED text now carries the could-not-check reason that produced the
  degrade, not just a line on stderr — so an operator reading the board can see which row
  degraded and why.
- A source pin that genuinely names nothing — neither a release tag nor a 40-hex commit in either
  column — is refused with a reason that says which line it read and that the lane is channel D
  ("build from source and pin the commit, or install a release and pin `statusgen <tag> <sha256>`").
  The one verdict it can no longer give is "no statusgen pin", which sent an adopter looking for a
  missing line instead of at the unreadable one in front of them. The genuine no-pin refusal now
  names every shape it looked for, the source line included (#1122).
- Alternate object directories are now resolved through go-git's own
  `AlternatesFS` option. The filesystem it is given is rooted at the nearest
  common parent of the directories the repository itself declares it borrows
  from — not at the filesystem root — so the reach grows by exactly the subtree
  the repository names and no further.
- Dead-claim decay now runs on a GitLab-hosted project. The pass drops branches whose change
  has already merged or closed so they stop consuming their stream's dispatch cap, but it read
  change state only through `gh pr list` — a client a GitLab project has nothing to answer — so
  statusgen declined to run it there and a GitLab adopter's claims never decayed at all: every
  landed-but-undeleted branch held its brief off the board forever. The read is now routed by
  the forge behind `origin`: `gh` on GitHub (and on a remote that names neither forge, where
  "could not tell" is still not "confirmed not GitHub"), and the project's merge-request
  listing over the GitLab REST v4 API on GitLab. In a pipeline it needs no wiring — the
  predefined `CI_API_V4_URL`, `CI_PROJECT_ID` and `CI_JOB_TOKEN` are enough — and
  `STATUSGEN_GITLAB_TOKEN` (or `GITLAB_TOKEN`) overrides where an instance will not let the job
  token list merge requests (#1111).
- Dead-claim decay's GitHub reader no longer lets a fork's branch name decay a live claim. The
  pass drops branches whose pull request has merged or closed, and it keyed that on the bare
  `headRefName` of every PR `gh pr list` returned — forks included. A fork's head branch is named
  inside the fork and names nothing in the tracked repository, so a throwaway fork PR named after
  a live dispatch branch, then closed, decayed that live claim and a second worker was dispatched
  onto work already in flight. The reader now asks `gh` for `isCrossRepository`, `headRepository`
  and `headRepositoryOwner` and admits a PR as a decay candidate only when its head repository IS
  the tracked repository, with the same fail direction the GitLab arm took in #1135: a PR whose
  head repository cannot be read is not read as same-repo, and each such skip is counted and
  reported as a `could-not-check` line. Under-decay that says so, never over-decay (#1147).
- GitLab merge requests are readable as checks-green again: the GitLab backend now publishes
  the head **pipeline** into the check rollup as the status context the pipeline-gating project
  setting requires, so a successful merge-request pipeline satisfies `deskflip`'s checks-green
  condition instead of being refused as "a required check that did not report on this head at
  all". The required name and the published entry come from one constant, so the gate can no
  longer demand a verdict the backend never serves (#1125).
- GitLab: `ReviewsAtHead` returns reviews in ascending submitted order, approvals interleaved with notes, matching the GitHub backend and the order the `Forge` interface now documents. GitLab's notes endpoint answers newest-first, and every consumer reduces the slice as "the last decisive verdict governs" — so the reversed stream let the OLDEST verdict govern: an approval at a newer head never cleared an earlier request-changes, and an ordinary approve-then-reject at one head was reported as a suspected forged no-op approval. The notes walk is now pinned newest-first on the wire as well, so a thread that exceeds the page cap loses its oldest notes rather than the governing verdict.
- GitLab: `deskpost review --verdict approve` no longer reports a rejected credential when the reviewer identity has already approved the merge request (#1106). GitLab's `POST /projects/:id/merge_requests/:iid/approve` answers a bodyless HTTP 401 whenever the acting user "cannot approve" — and a user who has already approved cannot approve again — so the generic 401 handler sent operators off to rotate a healthy token while the verdict they wanted was already in force, and `deskboard` kept re-dispatching reviewers onto a change the App had already approved. The backend now disproves the credential story with reads it can make: `GET /user` and `GET …/merge_requests/:iid/approvals`. An approval this identity already holds is success-with-note (exit 0, the note names both endpoints, nothing is re-posted); a 401 on either read confirms the credential really is rejected and keeps the fail-closed refusal, now naming the confirming endpoint; a valid credential whose user is simply not an eligible approver refuses naming eligibility and the acting identity instead of the credential. A classification read that fails some other way stays could-not-check and says it could not be classified.
- GitLab: a reviewer's `request-changes` verdict is now a standing rejection the board and the flip gate can read (#1124). The merge-request NOTE is the verdict object on GitLab — the forge has no native request-changes object — and the read path reduced only the CORRECTNESS verdict line to a review state. A `deskpost security-review --verdict fail`, which submits REQUEST_CHANGES and whose body may carry only `Security-Review: fail`, therefore came back as an ordinary comment: `deskboard` reported "no bot APPROVED/CHANGES_REQUESTED at head", left the merge request at NEEDS-REVIEW, and kept re-dispatching a reviewer onto a change its own reviewer had already rejected. Both lanes now reduce to the state their GitHub twin produces, with `Security-Review: pass` deliberately staying COMMENTED so a security all-clear cannot erase a standing correctness rejection.
- Removed the stale STAGED/PENDING-PROMOTION banner comment from windows-ci-leg.yml.
- Step 5's Action also named the retired `dailies` skill as a dispatch-bearing fan-out
  example — the same stale-roster defect, one instance the initial pass missed. Swapped
  for `pr-review-desk`, consistent with the two skills already named earlier in the same
  sentence. (#938)
- The CI templates `statusgen init` scaffolds now carry the NON-secret half of the trust roster
  into both statusgen jobs, so the Evidence-actor check (which identity committed each
  verified brief's Evidence lines) no longer reports could-not-check on every board regen while
  the job stays green. On GitLab there is no environment transport for the roster, so the
  `.gitlab-ci.yml` template materialises the new CI/CD variable `STATUSGEN_ROSTER_ENV`
  (Variable or File type; logins, ids and role bindings only — a secret-shaped key makes the
  job refuse) into `$HOME/.config/assay/roster.env` with owner-only permissions before
  `statusgen` runs, and prints a `NOTICE` naming the variable when it is unset. The GitHub
  workflow template passes the five roster variables through from repository Actions
  variables (`vars.ASSAY_*`, never secrets) and reports roster presence in each job with a
  `::notice::` when `ASSAY_TRUSTED_BOT_SLUGS` is unset. The adopter docs name the variable on
  each forge and what it carries (#1110).
- The `the-desk` and `intake-desk` skills carry frontmatter a YAML parser can load. Both
  `description:` values were plain scalars containing a colon-space (`… on an explicit desk-boot
  request: the user types …`, `… one of five tracked exits: spec/brief …`), which YAML reads as a
  nested mapping, so the whole frontmatter document failed to load and a harness that builds its
  skill roster by loading it saw no name and no description — those two skills never surfaced.
  Each is now a folded block scalar (`description: >-`) with the description text unchanged
  byte-for-byte, because it is adopter-facing trigger text the harness matches on (#1115).
- The forge-CLI ban's permit register drops `deskdispatch`'s last `gh` row; the ratchet ceiling comes down to 5.
- The gitlab arm's token row is renamed from "deskd GitLab read token" to "GitLab cell token (deskd read + boot fetch credential)". Despite the `DESKD_` prefix, `DESKD_GITLAB_TOKEN_FILE` is also the credential `gitlab_cred_args` feeds to the boot fetch in `cellctl desk` and to the `gitlab_fetch_reachable` probe, both of which run with `DESKD=0`. The old name invited gating the row on `DESKD`, which would have broken the fetch on every `DESKD=0` gitlab cell.
- The staged-changes check now proves it can read the whole HEAD tree before it
  reports an answer, and returns an explicit could-not-check (`deskpr` exits
  unverifiable, naming `git repack -a` as the local repair) when it cannot. This
  closes the quieter half of the same defect: a truncated walk could also HIDE a
  genuinely staged deletion, so an unreadable object store could have produced a
  false CLEAN as easily as a false refusal. Real staged changes are still
  refused exactly as before.
- `cellctl check` no longer demands the deskd GitHub App key and `ORGS` on a cell running `DESKD=0`. Both exist solely to mint deskd's per-org installation tokens in `deskd_mint_github`, which is reachable only from `cellctl deskd`, so on a `DESKD=0` cell they were failing `check` over credentials nothing reads. They are now reported `n/a` there, and remain a MISS whenever `DESKD=1`. The gitlab arm is deliberately left ungated — see below.
- `cellctl check`'s no-deskd row said "not required on a house cell" for any cell reaching it, so a k8s cell with `DESKD=0` was told it was a house cell — directly contradicting the `kind=k8s` line in the same report. It now names the cell's own kind.
- `cellctl desk`: an existing role worktree is now **merged** up to the fetched `origin/main` at boot (a real two-parent merge when it carries local commits — never a rebase), instead of `--ff-only`-then-"left as is". Generated single-writer files (`STATUS.md`, `docs/streams/FINDINGS.md`; `CELLCTL_GENERATED_FILES`) are taken from main on conflict; any other conflict **stops the boot** with the paths named and nothing launched (#1157).
- `commsloop` now delivers every accepted, in-lane, routed message to the addressee role's mailbox (`commsqueue.DeliverToMailbox`), so `deskcomms poll` as that role sees it and `deskcomms ack <id>` clears it; other roles poll empty, a message refused at the routing boundary or quarantined by the router is never delivered, and the executor leg stays exactly as gated before (#1166).
- `containers/scripts/layer-secret-scan.sh` no longer flags toolchain material
  it never wrote as a secret: Go's own stdlib test fixtures, npm's bundled
  docs, and PEM-shaped strings compiled into `gpgv`/`libssh2`/`libgnutls` are
  now excluded by a narrow, commented path allowlist, the generic `sk-` key
  shape is gated to text-shaped content (no longer checked against compiled
  binaries), and each layer-filesystem hit is reported once instead of twice.
  Verified clean (`exit 0`) against a real build of the desk base image, with
  a new mutation-test fixture proving no bypass for a real secret at an
  ordinary path.
- `deskboard actions`' classifier (`classifyPR`) no longer fails the WHOLE sweep when a single
  open PR carries one unreadable change-level field. Four per-change reads — the PR's own
  reviews, the non-commit-resolution label probe, the own-files read and the reviewed-sha
  compare feeding the benign-merge check, and the changed-files read feeding risk
  classification — used to propagate a per-PR read failure as a whole-sweep error (exit 6,
  empty board, one line of diagnosis). Each now degrades only its OWN row, landing on the
  safe side (never the benign/cleared outcome), the same contract the changed-files
  truncation guard already documented for itself.
- `deskboard dispatch` / `awaiting`, and the dispatch stage of `throughput`, read a
  `statusgen-source` line as the pin it is. The resolver asked only for a bare `statusgen ` line
  and then this host's `statusgen-<os>-<arch>` line, so a source-channel adopter — one who builds
  from a pinned commit because no release binary is published for their platform or forge, and
  whose `.assay-versions` therefore carries `statusgen-source <40-hex-commit> channel-D` — was
  reported as having **no pin** and the verb exited 6. Next-up came back could-not-check on a pin
  file that pins statusgen. A source line now resolves: it reports the release tag it names when it
  has one (so the running-vs-pinned skew comparison still works), else the pinned commit. A release
  line still wins when a file carries both shapes, and a present-but-malformed release line still
  fails closed rather than being quietly replaced by a source line (#1122).
- `deskboard` reads a GitLab change's CI for real rather than classifying every merge request
  CI-UNKNOWN: the bulk board read maps the change's head pipeline — looked up by head SHA,
  through the same mapping the per-change read uses — so the board and the flip gate cannot
  reach different verdicts about one pipeline, and `reviewloop` emits a FLIP-VERB where it
  previously surfaced an uninterpretable rollup. A head with no pipeline, or a pipeline stamped
  with a different SHA, stays could-not-check and never a pass.
- `deskboard`'s `STALE:drift` banner names all three sides — the installed release, this worktree's pin, and origin/main's pin — and recommends the shim reinstall (`sudo make desk-install`) only when the installed release is the one behind main's pin; a worktree behind main gets the merge line instead, and an unreadable origin/main is reported as could-not-check (#1157).
- `deskboot` gains an eighth step, `worktree-current`, after `board-fetch`: HEAD must contain `FETCH_HEAD` and the worktree's `.assay-versions` must equal `FETCH_HEAD`'s, else exit 6 naming the one-line self-heal (`git merge refs/remotes/origin/main` in the worktree). A desk on a tree behind main is loud at boot, not blind an hour later (#1157).
- `deskclose` accepts **typed item references** on both the item it acts on and the target of
  `--of` / `--by`: `!N` names a merge request or pull request, `#N` and a bare `N` state no kind
  and leave the resolution to the forge, and a web URL states the kind in its own path — with
  `--kind` / `--of-kind` / `--by-kind` as the equivalent flags. On a project that numbers issues
  and merge requests in separate sequences, a bare number could name two different objects, so
  the read failed closed and told the caller to "use the typed operation for the kind you mean"
  — an operation `deskclose` did not expose, which left the whole supersession lane unreachable
  in both the proposing and the confirming role. Bare numbers keep that fail-closed behaviour,
  and the refusal now names the forms that exist (#1109).
- `deskclose` routes every read and write of a lane at the kind of object it actually read: the
  item read, the pre-close comment, the proposal-thread read, the back-reference and the close.
  The untyped close addressed only the issue sequence, so on a project carrying both an issue
  and a merge request at one number it would have closed the object the caller never named, and
  the untyped thread read returned another object's notes — or, on a single-sequence forge, an
  empty thread for any issue, which reads as "no proposal stands".
- `deskdispatch` claim-acquire now runs the claim child as the dispatching role: it mints (or reuses) that role's App token through the same seam its model-stamp step uses and hands it over as `--token-file <0600 path>` to `deskclaim-ref` or as `GH_TOKEN` in the child environment for the legacy `tools/dispatch-claim.sh`. An exported `GH_TOKEN` still wins; a mint refusal is exit 6 with no claim attempted — the claim tool is never run on the ambient `gh` login (#1151).
- `deskdispatch`'s model-stamp step (`--model`) now reads the change's present labels and its label history and writes the `dispatched-model:` / `dispatched-tier:` stamps through the resolved Forge under the lane's own dispatcher credential, on GitHub and GitLab alike — it no longer shells to the GitHub CLI, so a stamped review dispatch on a GitLab project completes and reaches the forge-neutral `authorization-needed` queue label instead of failing closed before it (#1154). The fail-closed semantics are unchanged (an unanswered read is could-not-check; a foreign stamp is removed and re-applied, never stamped over) and an identical stamp already standing under the dispatcher is now a stated no-op rather than a re-apply.
- `deskdisposition sweep` reads a repo's open changes through the forge that SERVES that repo
  instead of shelling `gh pr list`. On a GitLab project the old path asked GitHub about a slug
  that is not a GitHub repository, so the answer was "Could not resolve to a Repository with the
  name …" and the verb reported the project's whole PR queue as could-not-check (exit 6) — a
  GitLab adopter's orphan sweep could never look at all, and an unreadable queue is the one thing
  that must not read as an empty one. The read is now the enumerated `ListOpenChanges` op, which
  serves GitHub pull requests and GitLab merge requests alike; number, title and labels are the
  only fields the sweep classifies on, and GitLab's degraded open-change shape serves all three
  for real. Truncation is still reported, now from whichever ceiling clipped the page — the
  forge read's own cap or `--limit` (#1123).
- `deskevidence` now scopes its secret scan to the bytes the landing ADDS on every path, `--brief-path` included: the scan diffs the content about to be committed against the branch copy, so an Evidence block that re-quotes a line the brief already carries verbatim (a Verify row's own command, a fingerprint named in prose) no longer refuses on text that predates the landing — the failure that stalled a PASSED human-gated brief at `implemented` (#1161, completing #901 and #966).
- `deskpr` no longer refuses a clean shared-object checkout as
  "staged-but-uncommitted changes — commit them first". A checkout made with
  `git clone --shared` or `--reference` stores almost no objects of its own: it
  borrows them from the directory its `objects/info/alternates` names. The
  in-process git layer handed go-git a filesystem rooted at the checkout's own
  `.git`, which cannot see outside itself, so every borrowed object read as "not
  found" — and go-git's tree walk does not report that as an error. It turns a
  failed subtree read into an end-of-walk, so the walk stops early and the index
  entries whose HEAD-side counterparts vanished with it look like staged
  additions. A checkout `git status` called spotless was refused.
- `deskpushguard`'s register-id and foreign-commit pre-push checks no longer peg one CPU
  core indefinitely on a checkout with several remotes pointing at the same upstream repo.
  Both checks called `gitcore.Repo.IsAncestor` once per candidate remote branch (register-id
  check: once per push that touches a new register entry, regardless of the push's own
  size; foreign-commit check: once per commit ahead of `origin/main`) — that call resolves
  to go-git's native, unmemoized `Commit.IsAncestor`, which re-walks `origin/main`'s entire
  history from scratch on every single invocation. A checkout with N literal-duplicate
  remotes for the same repo multiplies the candidate-branch count by N directly, and a
  large main-catch-up merge multiplies the commit count on the foreign-commit side —
  together this could run for minutes without deciding. Both checks now walk
  `origin/main`'s ancestry exactly once per push and answer every subsequent
  "already merged?" question with an O(1) set-membership lookup instead. Measured on a
  synthetic fixture (800-commit `origin/main`, 20 never-merged sibling branches visible
  under 5 remote names): 5.1s+ before the fix, 129ms after.
- `desktoken --forge gitlab <role>` now SELF-CHECKS the rotated PAT with one live, read-only
  `GET /user` before printing the custody path. The rotation endpoint's 200 only says the forge
  issued the successor; in the field the caller's first read with it answered 401 while the
  on-disk token was valid seconds later (server-side propagation lag after self-rotation), and
  the mint path assumed that away. A self-check the forge does not answer 200 now exits 6
  naming the endpoint, the status the NEW token got and whether the PREVIOUS token was still
  accepted (lag: re-run the mint once) or rejected too (lockout: a group owner re-issues the
  PAT); the persisted path is not printed as good. One read per token, no retry loop, no
  sleep; rotate-on-mint custody is unchanged (#1142).
- `desktoken --forge gitlab <role>` now rotates THROUGH a symlinked custody path instead of over it: the new token is written to the link's resolved target and the link survives, so the provisioned token file is never left holding the invalidated value for a later re-link to hand back — the source of intermittent `401`s on the first API read after a successful rotation. The rotated value is also fsync'd before the command prints the custody path, and the read-back verification now reads back through the custody path so a rotation that broke the layout fails at mint time rather than at the next read (#1112).
- `docs/codex-smoke-protocol.md`'s preamble and Steps 1/3 named a stale nine-skill roster
  (including two skills — `dailies`, `market-intelligence` — no longer in the bundle) while
  the packaged bundle ships thirteen. The preamble, Step 1's `Expect:` line, Step 3's
  Action/Expect, and the run-log skeleton now name the correct count and the full current
  roster (`adopt`, `ask-decision`, `author-brief`, `human-runsheet`, `install`,
  `intake-desk`, `pdfingest`, `pr-review-desk`, `pr-shepherd`, `the-desk`, `upgrade-assay`,
  `verify-desk`, `worker-desk`), matching `plugins/assay/codex/packaging.md`'s
  `assay:codex-packaging` roster and `plugins/assay/skills/`. (#938)
- `docs/streams/forge-gitlab/brief-17-resolved-thread-merge-gate.md` cites its consumed changelog fragment through the `CHANGELOG.md` v1.0.9 section instead of the fragment file the release roll deleted, so the board regen on `main` no longer reds on a dangling backticked path (the lint-side question stays open on #722).
- `statusgen --close-verify` (the flip `verify-gate-close.yml` runs on a human close) now reads the Verified cell the `done` row would carry — the README cell, or on the implemented→done path the cell it stamps from the brief file's Evidence — plus the brief's Evidence rows, BEFORE it writes, and refuses when a runner is below the methodology/19 verifier floor. The refusal names the runner, the floor and the two-stamp remedy; a red `done` no longer lands on main after the human has signed (#1170).
- `statusgen` per-entry intake files (`docs/streams/intake/*.md`) now match their frontmatter keys case-insensitively — `Disposition:` / `DISPOSITION:` and every other `intakeEntry` field — instead of silently leaving the field empty and counting the entry as untriaged `new`; an owned key repeated in differing case is now a parse error naming the file rather than a silent first-wins. Sibling of the legacy-path fix in #920. (#931) — thanks @teddyvj
- `verify-gate-close.yml` relays a verifier-floor refusal onto the card as a comment (runner + floor + remedy) and REOPENS the card so the same human closes it again once the floor-tier re-verify stamp has landed; a bot close is reopened exactly as before, and every other refusal still leaves the card closed (#1170).

### Changed
- A dead-claim decay that could not look now says so where a reader will see it. It reported a
  stderr `NOTICE` while the board it wrote read perfectly clean — a two-state instrument, and
  the reason a forge on which the pass could never run went unnoticed. The run now prints
  `could-not-check: claims not decayed` with the reason that names which read failed, and the
  generated `STATUS.md` carries the matching banner at the head of its Next-up section, stating
  that those rows are a **subset**: briefs held behind already-merged branches are missing from
  the board, not absent from the backlog. The exit code is deliberately unchanged — an undecayed
  claim set hides work rather than handing one brief to two sessions, and failing every adopter
  run that has no forge credential would only train a desk to stop reading the instrument. The
  load-bearing fail direction is unchanged too: decay may only ever SHRINK the claim set, so an
  unreadable listing keeps the full open-branch set and never drops a live claim (#1111).
- Desk-role skills (the-desk, intake-desk, worker-desk, pr-review-desk, verify-desk, pr-shepherd) now name the `deskcomms send` / `poll` / `ack` lane verbs for every cross-desk hand-off, with five hand-off kinds (advise / request-act / blocked / finding / depends) mapped onto the shipped lane vocabulary, as one derived guardrail block (`comms-verbs`); the same-box session channel is documented as the pre-cutover fallback only.
- New fixture test `.github/scripts/verify-gate-close-floor.test.sh` mirrors the close workflow's refusal classification and runs it, with a statusgen built from the tree, against `statusgen/testdata/verifyfloor`; migration `migrations/0003-v1.0.9-to-v1.0.10-verify-gate-close-verifier-floor.md` records the workflow patch an adopter's copy needs (#1170).
- The `--lint` verifier-floor PROBLEMs name the two-stamp remedy instead of only the rule (#1170).
- The `verify-desk` skill states the two-stamp model for `gate: human` briefs: the routine drain runs at the local tier and lands the first stamp; ONE floor-tier re-verify (the single sanctioned pass above the local tier) re-runs the table, appends its Evidence rows and re-stamps the Verified cell with that pass leading the cell; only then is the sign-off card ready for the human. Model-gated briefs are unchanged (#1170).
- The shared body check's PGP-fingerprint exemption now also admits a 40-uppercase-hex run whose OWN line names it as a fingerprint (`fingerprint`, `fpr`, `pgp` or `gpg`, case-insensitive, as a standalone word — before or after the run), alongside the existing `pgp:`/`fp:` recipient-field anchor. The bound is unchanged in every other direction: exactly 40 uppercase hex, the word on the SAME line, and no annotation launders a mixed-case run (#1161).
- The two column layouts a `-source` pin line is written in — `<tag> <40-hex-commit>` and
  `<40-hex-commit> channel-D` — are interpreted in ONE place, `deskkit.SourcePin`, which selects
  through the same trailing-space prefix match every other pin reader uses. The drift banner's
  `desk-tools-source` reader now calls it instead of carrying its own copy, so the two readers of
  a source line cannot drift into disagreeing about what the same line says. `PlatformPinLookup`
  exposes the release-line read as its three real states (present / absent / fail-closed), which is
  what lets a caller fall through to another pin shape on absence ALONE (#1122).
- `deskclose manifest` is documented as the sanctioned human-ruled BATCH lane: the human's own ruling comment is the manifest's `authorized-by`, and the digest binds it to exactly the rows they saw. No behaviour change.
- `deskdispatch` prefers `deskclaim-ref` on PATH over the legacy `tools/dispatch-claim.sh` when both resolve; the script stays the fallback for a tree that predates the binary, and the `claim-acquire OK` line names which tool ran and how it authenticated (#1151).
- `deskdisposition`'s two READ verbs (`read`, `sweep`) are both on the forge seam, under the
  session-role App token the tool already minted for `read`; `set`'s writes still go out under
  the caller's ambient `gh` identity, unchanged, because routing a write through the seam changes
  WHO performs it. The forge-CLI register row for the tool is now write-only (#1123).
- `docs/adopting-assay-gitlab.md`: the `issue-loop` and `intake-loop` service accounts need **Developer (30)** with `api` + `write_repository`, not Reporter (20) — both lanes land their exits as draft merge requests, and GitLab refuses MR creation below Developer (medici-finance/assay#1107; first seen on an adopter's first intake MR as a bare HTTP 403). The `auditor` row stays at Reporter: it only reads.
- `references/desk-shell.md` gains the comms-lane transport section (markers, exit codes, the one-send form); house values (cell name, gateway address) stay deferred to the project layer.
- `skillslint` reads skill frontmatter with a real YAML parser instead of scanning lines, and
  fails a `SKILL.md` whose `---` block does not load, does not load as a mapping, or whose `name:`
  / `description:` is not a non-empty string. The line scan reported PASS on both broken skills
  above, so the defect shipped and an adopter refresh would have reintroduced it; the lint now
  reads the header the way a consumer does and names the repair (quote the value, or make it a
  folded block scalar, keeping the text unchanged) in the failure message (#1115).

## v1.0.9 — 2026-09-15

### Added
- **A tick contract for the five desk roles** — `plugins/assay/references/tick-contract.md`.
  When the harness passes `--tick`, or the environment carries `ASSAY_TICK=1` (compared
  exactly), a desk role runs ONE bounded pass — boot, one fresh sweep, act up to its width,
  wait bounded for what it dispatched, print a summary line, exit — arming no durable wake,
  scheduling no cadence and waiting in line for no answer. Absent both spellings every run is
  a standing window and behaves exactly as before, so the contract is inert until a caller
  asks for it. The five desk bodies each gain a short `## Tick mode` section, derived from one
  declared guardrail block rather than hand-copied, so the gating plugin-tree lint keeps all
  five byte-identical.
- **`desk-skills` brief 05 — standing-note reference.** Adds
  `plugins/assay/references/standing-note.md`, naming the nine-section schema
  (Boot · Monitors · Hands-off · Merged · Flipped · In-flight · Filed · Teammates · Board) for
  the loop-continuity note a desk-role session writes at each iteration boundary and before any
  long wait. The load-bearing rule: every row names the primary state to re-probe on resume,
  never the value it last observed, so a returning session re-checks live state instead of
  acting on a stale cache. Each of the five loop-role skill bodies (`the-desk`, `worker-desk`,
  `pr-review-desk`, `verify-desk`, `intake-desk`) gains one pointer line to the new reference;
  in `pr-review-desk` the pointer resolves the body's existing standing-note sentence instead of
  leaving it unanchored.
- **`deskread`** — a read-only desk verb that serves forge reads as versioned JSON, so a consumer
  outside the desk-tools module can reach the forge seam by running a process rather than shelling
  a forge CLI. It adds **no operation** to the frozen `Forge` interface: `deskread issues` is
  `ListOpenIssues`, which both backends already implement and golden-pin. `--repo` is repeatable
  and **one invocation serves the whole repo set**, read concurrently. A repo that could not be
  read lands in `partial` with its reason and is absent from `repos`, with the exit code still 0 —
  so a caller can always tell "no open issues" from "could not look"; only an all-unreadable set
  is could-not-check (exit 6).
- **`forge-neutral` brief 15 — `desklabel`, a role-keyed label verb, on the forge resolver.**
  Specifies a new verb that sets or clears one label at a time: any role may touch the shared
  escalation vocabulary (`question`, `help wanted`, `needs-decision`), a role may touch only
  the disposition markers its own lane already applies (worker: `superseded?` and the
  `disposition:*` family; reviewer: `authorization-needed`/`approval-needed`), and every other
  label — including `human-decided`, refused for every role — is refused outright (exit 5).
  Adds one new `Forge` operation, `ApplyIssueLabels`, because GitLab's existing `ApplyLabels`
  reconciles only a merge request's labels and a plain issue is a separate resource there
  (GitHub already serves both kinds from one endpoint, so its side is unchanged). Cites a real
  gap the design closes: `deskclose superseded --dispute` never removes the worker's
  `superseded?` proposal marker, and before this brief only a raw, unscoped label write could
  clear it. Doc only; no tool behaviour changes in this PR.
- **`plugins/assay/scripts/tick-summary.sh`** — the one executable form of the summary-line
  grammar (`tick role=… outcome=… swept=… acted=… filed=… duration=…`), with `regexp`,
  `validate` and `check` verbs, plus its hermetic case suite. Its cross-field rules are what
  keep the line honest: `could-not-check` requires `swept=-` and `noop` forbids it, so a pass
  that could not read its queue cannot produce a well-formed "queue was empty" line.
- **`statusgen --forge`** — the opt-in for forge-backed checks. Without it statusgen is **offline**:
  it starts no forge process and makes no network call, and a forge-backed check reports
  could-not-check as itself rather than reading green. The default reader answers could-not-check
  for every repo and returns no data entry for it, so there is no shape in which "did not look" is
  indistinguishable from "looked and found nothing".
- A **prune singleton**, so N desk windows booting together run ONE sweep. A non-blocking
  advisory lock (released by the kernel when its holder exits, so there is nothing to time
  out) plus a `--singleton-ttl` recency debounce, default 10m; `--singleton-ttl 0` /
  `--no-singleton` disable the debounce only. The lock fails closed and the TTL fails open:
  a missing, truncated, unparseable or future-dated stamp means SWEEP, so no leftover stamp
  can wedge prune. The `--interval` supervisor takes and releases it per tick, never for its
  lifetime.
- A **sequencing note** recording why this stream is planned rather than fanned out: its fixes
  uncover the next latent defect in sequence, so discovering that sequence one field report at a
  time costs an adopter round trip per link. It carries two standing rules — a GitLab review-desk
  issue routes to the plan's map before it is dispatched, and a code read is never a field verdict.
- A GitLab merge request now opens with a resolvable "merge-hold" discussion thread that
  blocks the merge button on every GitLab tier (`only_allow_merge_if_all_discussions_are_resolved`)
  until the reviewer's approve verdict releases it at the current head; a request-changes
  verdict or a new head re-arms it. `deskflip`'s reviewer-approved condition on GitLab now
  reads this thread directly and never consults the Premium approval-configuration route that
  answers 403 on GitLab Free.
- An **issue → brief map** placing every open GitLab review-desk, adopter-path and stream issue
  against exactly one brief, or out of scope with a reason.
- Four briefs: **13** (board reads degrade per row, never per sweep), **14** (the public-repo gate
  reads the forge that serves the repo, at every site), **15** (the three keys the GitLab runbook
  never names — forge binding, board-push credential, source-pin lane), and **16** (the human-gated
  live review-tick conformance walk and adopter-backlog close-out).
- The `forge-gitlab` stream board now states an explicit **finish line** for "the review desk works
  on GitLab" — which verbs, on which tier, under which credentials, and with which two instruments
  (a live per-verb conformance table carrying at least one refusal row, plus the offline fixture
  goldens) — so the claim can be checked rather than felt.
- The brief's performance half, measured rather than asserted: on this repository (24 streams,
  165 briefs) a `--lint` spends **16.7 s of its 23.7 s** inside 11 forge-CLI subprocesses — 13.9 s
  of that fetching every issue ever opened across ten configured repos to print one advisory line
  whose inputs are open issues only — and makes **254 git subprocesses**, of which one whole-tree
  authorship walk (0.03 s) replaces 144 `git log` plus 62 `git blame` invocations. A memo closes
  the 3,351 brief parses that re-read 172 files up to 23 times each. Target: zero forge-CLI calls,
  100 git subprocesses or fewer, and a 60 % or better wall-clock cut on a 400-brief tree.
- `.github/workflows/docker-publish.yml` now also builds and publishes the
  shared `desk-base` image and the five per-desk images (`intake-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk`), version-locked
  to a single build's tags and gated by `containers/scripts/layer-secret-scan.sh`
  before any push (brief desk-containers/03 Task 2). This content had been
  parked at `docs/streams/desk-containers/pending-docker-publish.yml` since
  the worker App's token cannot push a `.github/workflows/*` change; applied
  by hand onto the live workflow file (which carries no other changes since
  the content was parked) and the parked file removed.
- `Forge.GetIssueTyped(repo, number, kind)` and `Forge.PostCommentTyped(repo, number, kind, body)`
  — the typed read/write pair the bare `GetIssue` both-kinds refusal points at, implemented on
  both backends, with `TargetKind` (`issue` | `change`) and `ParseTargetKind` (accepts `issue`,
  `mr`, `pr`, `change`). Additive: the existing untyped methods keep their behaviour.
- `RepoHardeningRead` (op 40) on the `Forge` seam: a closed hardening-read kind vocabulary
  (`repo`, `rulesets`, `actions-workflow-permissions`, `actions-fork-pr-approval`,
  `actions-private-fork-pr`, `vulnerability-reporting`), validated before any request exists.
  GitHub implements all six; GitLab refuses each by name until forge-gitlab/12.
- `desk-containers/08` — a brief for a **tick contract**: when the harness passes `--tick`
  (or the environment carries `ASSAY_TICK=1`), a desk role runs ONE bounded pass — boot, one
  fresh sweep, act up to its width, wait bounded for what it dispatched, print a
  fixed-grammar summary line, exit — arming no durable wake and asking no human anything.
  The five desk skills are standing loops with no second mode, so a scheduled one-shot run
  of one can only ever end in its own `timeout`, with an empty log; the brief adds the
  missing mode as a contract stated once in a shared reference and derived into all five
  bodies, with the summary-line grammar as its machine-readable verdict channel. The
  standing-window behaviour is unchanged.
- `deskaudit tail [N]` — print the newest N ledger entries (default 10) across the daily
  segments, as the raw lines they are on disk.
- `deskfile new`'s per-session budget accounting (`chargedNewEntry`) now carries an explicit
  citation and regression tests (`TestBudgetBodyCheckRefusalDoesNotConsumeSlot`,
  `TestBudgetDedupeRefusalDoesNotConsumeSlot`) proving that a REFUSED `new` — a BodyCheck
  secret-scan hit or a dedupe match — is audited but does not consume the 3-per-24h
  session budget slot: only a write that reaches `gh issue create` may charge it. The
  existing `ResultRefused` exclusion in `chargedNewEntry` already implemented this; these
  tests close the coverage gap end-to-end and pin the behaviour against regression. (#955)
- `desktoken --no-rotate` — a read-only credential lookup that makes the same custody checks and
  prints the same path, but performs no rotation and no network contact. `deskfile check`, a dry run
  that files nothing, now uses it, so a parallel sweep of checks no longer drives one destructive
  rotation per call. Rotate-on-mint is unchanged for verbs that write.
- `desktoken` records an owner sidecar (`<token cache>.owner`) naming which App and which
  account a cached token belongs to, and caches a resolved installation id per (App, account)
  for 24 hours. Every fast path is a positive match on both halves; absence, ambiguity, a
  non-0600 file or a malformed account name falls through to full resolution. `--fresh` and
  `<PREFIX>_INSTALL_ID` are unaffected, and a 404 from the token exchange invalidates a cached
  installation id instead of failing for the rest of its TTL.
- `desktoken`'s `auditor` role: a dedicated, read-only identity (no write permission of any
  kind) minted/read through the existing per-role custody paths.
- `docs/streams/forge-gitlab/brief-17-resolved-thread-merge-gate.md` — a new brief giving the review desk an enforceable merge gate on GitLab Free: a marker discussion thread the draft-change verb opens on every new merge request, the reviewer's at-head approve verdict resolves, a request-changes verdict or a new head re-opens, and `deskflip` keys on — behind the Free-tier project setting `only_allow_merge_if_all_discussions_are_resolved`, never the Premium approval-rules route that answered 403 to a GitLab adopter cell on 2026-09-14 (#1091). The `Draft:` prefix stays as the human-facing signal; the human merge stays the outer gate.
- `forge-neutral/18` — a brief for taking **statusgen off the forge CLI**. statusgen is the last
  tool in the suite that reaches a forge on its own: a separate Go module that does not import
  `deskkit`, carrying 26 of its own forge-CLI shell-outs, so the enumerated operation set, the
  refusal-instead-of-fallback rule and the per-forge backend all stop at its module boundary. The
  brief adds to desk-tools exactly the read statusgen needs — `deskread`, a read-only verb over
  operations the seam ALREADY enumerates, so the frozen `Forge` surface is consumed rather than
  widened — and makes `--lint` offline by default, with every forge-backed check reporting
  could-not-check as itself rather than reading green because it stopped looking.
- `gitcore.OpenWith(dir, cache)` and `gitcore.NewObjectCache()` — open a repository through
  a caller-supplied object cache, so a pass over many worktrees of one repository decodes
  each object once. `gitcore.Open` is unchanged for every existing caller.
- desk-tools brief 23 is authored: an opt-in, local-only usage-and-timing record written by
  the shared desk substrate at one place, kept as a 7-day UTC history under
  `~/.config/assay/perf/` and pruned on write, with a closed non-PII field set and no
  free-text field at all — plus a `deskperf` read verb for per-tool p50/p90/max wall time,
  refusal ratios and `deskboot` per-step cost. It inherits `docs/telemetry.md`'s promise and
  its exact `ASSAY_TELEMETRY` switch, leaves the append-only audit ledger untouched, and
  ships no sender: a remote sink is named as follow-up, so only the on-disk shape is fixed.
- desk-tools brief 24 — the audit ledger's read cost: a bounded tail read for `Guard`'s
  last-entry question, a bounded reverse read for the rate limiter that falls back to the full
  parse whenever its answer is not yet determined, no audit row for a `desktoken` cache reuse,
  daily `audit.jsonl.<date>` rotation that deletes nothing and carries the counter and the
  idempotency store forward, and a `deskaudit tail` read verb (#1035).
- desk-tools brief 25 — one token lookup per owner per process: a memo in front of the role-token
  minter, and `desktoken` consulting its token cache BEFORE it resolves the installation id
  (#1036). Authored only; the implementation lands separately.
- desk-tools brief 26 is authored, from the measurement in #1037: `deskwt prune` — a boot step
  every desk window runs — spent 8–12 minutes at ~100 % CPU on a checkout with ~657 registered
  worktrees because it asks one question of one history once per candidate and shares nothing
  between the answers. The brief hoists a single `origin/main` walk into a per-sweep
  ancestor-hash set, moves the merge gate ahead of the full-worktree `Status()` it currently
  runs first, shares one object cache across the sweep, drops a commit count that was walked
  twice per candidate to render a skip string nothing parses, batches the per-removal
  `git worktree prune` + full worktree listing into one pass, makes `--dry-run` genuinely
  read-only, and adds a prune singleton whose lock fails closed while its TTL debounce fails
  open — so N windows booting together run one sweep, and no leftover stamp can wedge the next
  one. Every removal gate is preserved: the brief changes the order and the sharing of the
  work, never which worktrees are eligible for removal.

### Fixed
- **The public-repo write gate reads the forge that actually serves the repo, at every call
  site.** `deskreply` and `deskevidence` each resolved the correct forge backend for every
  other operation on a repo, then built a second, hardcoded GitHub-only client for the gate's
  live-visibility read — so on a GitLab-resolved project that read went to a host that had
  never heard of the project, and the gate failed closed for a reason unrelated to the repo's
  real authorization. Both sites now route the gate's read through the already-resolved
  backend (`deskkit.ForgeRepoInfoFetcher`), the same pattern the draft-change verb already
  used. The release verb's tag-cut gate is ruled to stay single-forge (it never resolves a
  forge backend at all) with the reasoning recorded at its call site. The superseded
  single-forge GitLab adapter this replaced is deleted, and a cross-command enumeration test
  now fails if any future call site builds a hardcoded fetcher outside the one ruled
  exception.
- A label write that names NO target is refused on both forges rather than defaulted to the
  merge-request route. A caller that forgot the target would otherwise reproduce this exact
  defect on GitLab while passing every GitHub test; the refusal is what makes the omission loud
  on the forge most contributors run.
- A mint that cannot take that lock **refuses before a second rotation is in flight** (exit 6) and
  names the recovery path, instead of rotating unserialised. A custody directory that cannot be
  written is now detected *before* the rotation rather than after it, so the role's existing token
  survives a misconfiguration instead of being spent on a rotation that could never have been saved.
- A subcommand `--help` is no longer charged to the append-only audit ledger as a refusal.
  `deskpr`, `deskwt`, `deskfile`, `desktoken`, `deskpost` and `deskreply` print usage and exit
  0, writing no row; a genuinely bad flag in the same position still refuses and still audits.
- An unestablished sha is now treated as what it is: a could-not-check that degrades ONE ROW to
  `RE-REVIEW`, the same safe side a truncated diff already degrades to, and the sweep carries on.
  "The change's own files are unchanged since the last review" is a claim nobody can make
  without both endpoints, so the benign classification is never the fallback.
- Generated GitLab CI scaffold (`statusgen/init.go`): the `STATUSGEN_PUSH_TOKEN` comment now
  points at the new board-push-credential subsection by name.
- GitHub behaviour is unchanged: a GitHub App still renders `<slug>[bot]`, and a bare App slug is
  still not a role login — the fail-close that stops a user named after an App slug from
  satisfying a role comparison.
- GitLab `ReviewsAtHead` no longer aborts the whole review read when the project
  approval-configuration route (`GET /projects/:id/approvals`, Premium+) answers 403 instead of
  404 — the shape gitlab.com's Free tier actually returns. A 403 there now degrades head-pinning
  only (same as the documented CE/Free 404 gap), but only when the per-MR approvals read that
  follows still succeeds, so a genuinely rejected credential (which 403s that read too) still
  fails the whole read closed. Previously every `deskpost review` / `security-review` on a
  gitlab.com Free-tier project aborted outright.
- GitLab `draft_status` now maps to MERGEABLE instead of UNKNOWN in the PR/MR mergeable-state
  read. `deskflip` re-evaluates its `mergeable` condition against a change that is still a
  draft (it un-drafts only once every other condition has held), and every change this desk
  opens starts life as a draft — so with the old mapping, the condition could never pass for
  the ordinary starting state of a fresh GitLab change. Every other policy-hold status keeps
  its existing UNKNOWN mapping unchanged.
- GitLab runbook (`docs/adopting-assay-gitlab.md`): named the `ASSAY_REPO_FORGES` forge-binding
  key beside the existing boot-time keys, added a dedicated board-push-credential subsection for
  `STATUSGEN_PUSH_TOKEN` (token kind, scope, minimum role, variable visibility) and corrected the
  runners section's pointer, which previously sent readers to the wrong section for the wrong
  credential, and added a source-pin-lane subsection cross-referencing the other forge's runbook
  for GitLab-plus-native-Windows adopters.
- Obtaining an App installation token no longer costs a process and an API round trip per
  forge read. `deskkit.RoleTokenForOwner` now memoises per `(role, account)` for the life of
  one process, bounded at 45 minutes — strictly inside `desktoken`'s own 50-minute reuse
  window — so a board read over N repositories forks `desktoken` once per ACCOUNT rather than
  once per read, and `deskflip` mints once per run instead of twice (#1036).
- Planned in the same brief: the drift self-check resolves only the bare `desk-tools` pin name,
  so a consumer pinning the per-platform `desk-tools-<os>-<arch>` line the distribution contract
  specifies reports a permanent could-not-check as STALE; body/schema refusals in `deskpost`,
  `deskpr` and `deskreply` never name the offline check that would have caught them; and a
  subcommand `--help` is charged to the append-only audit ledger as a refusal.
- The GitLab CI template `statusgen init --forge gitlab` emits, and `docs/adopting-assay-gitlab.md`,
  now require the regen job's push credential `STATUSGEN_PUSH_TOKEN` to be a **masked and
  protected** CI/CD variable; both previously said masked only. A masked-only variable is still
  injected into merge-request pipelines, which run the MR branch's own CI file, so a member who
  could open an MR could read the token and push to the default branch past the merge gate.
  Protected limits it to pipelines on protected refs — the default branch the regen job runs on,
  which the adopting doc's provisioning step already protects. The template's guidance comment and
  stop message, and the doc's UI and API creation forms (`protected=true`), carry the requirement;
  the scaffold test pins it. Found by a GitLab adopter cell's review.
- The GitLab forge backend now serves the bulk open-issue read, so `issueboard` and
  `deskboard queue` work on a GitLab-backed project instead of failing closed with
  could-not-check. The read was withheld on the ground that its summary is only ever
  consumed paired with the issue trust-events read — but that read has since been served
  on GitLab, so the pairing the refusal protected is exactly what was already available,
  and withholding the list was all that kept the issue lane blind. A GitLab adopter no
  longer has a working merge-request board next to a permanently blind issue lane.
- The `deskclose` mutation gate (`cmd/deskclose/mutations.json`) is load-bearing again. Its
  "the dispute posts its reason but never applies needs-decision" plant matched the old
  `addLabel(repo, n, spec)` call; when the label write started carrying the item's kind
  (`addLabel(repo, n, it, spec)`, so a GitLab issue is never labelled as the merge request
  sharing its number) the plant's text stopped resolving, muhar reported it could-not-mutate,
  and the truth-suite's `mutation-gate (cmd/deskclose/mutations.json)` leg went red on main —
  not because a guard failed, but because that one guard was no longer being tested. The plant
  now names the current call; the gate's baseline and control were green throughout.
- The benign keep-current classification still runs, and is still reachable, wherever both shas
  ARE established — pinned by a test alongside the two degrade cases, so a future change cannot
  quietly retire it in the name of never comparing.
- The cause was a seam between two correct halves. The review reduction deliberately folds "the
  head advanced past the review" and "one of the two shas was never established" into a single
  not-at-head answer, so an unread sha can never be mistaken for an at-head one. The
  benign-merge (`MERGE-CURR`) arm then read that one answer as if it always meant the first, and
  asked for a diff between the reviewed sha and the head — which, in the second case, has no
  endpoints to span.
- The drift self-check resolved only the bare `desk-tools` pin name, so a consumer pinning
  the per-platform `desk-tools-<os>-<arch>` line reported a permanent could-not-check as
  STALE. It now tries the per-platform artifact name as a second EXACT lookup — the bare line
  still wins when both are present, and the trailing-space prefix match is untouched.
- The label reconciliation now carries WHAT it is labelling: `LabelChange` names its target as
  an issue or a change (PR/MR), and every caller states it — `deskfile new` and `deskclose`'s
  issue arm label the issue, `deskflip`, `deskpost` and `deskdispatch` label the change. On
  GitLab an issue target goes to `PUT /projects/:id/issues/:iid` with the same
  `add_labels`/`remove_labels` reconciliation the MR route uses (and the same ensure-label
  step). GitHub shares one number space and one labels endpoint for both kinds, so its requests
  are unchanged.
- The login a desk ROLE is expected to act under is now resolved from the forge its roster entry
  declares, instead of always rendering GitHub's `<slug>[bot]`. On GitLab a service account is
  attributed by its bare username, so the expected reviewer login never matched an actual one —
  and because the actor comparison carries an is-an-App flag as well as a name, the two could not
  match even when the slug was identical. Every gate keyed on that comparison read "no verdict"
  for approvals that were really there.
- The open-issue walk follows GitLab's page-continuation header to exhaustion and
  **refuses** rather than truncating if a project is still paginating at the page ceiling.
  The issue summary carries no truncation field and the issue lane reads an issue's absence
  from the list as *closed*, so a silently short read would have retired tracking rows for
  issues that are still open.
- The review board reduced the same approval to no-verdict, classifying an approved merge request
  as NEEDS-REVIEW and firing its UNREVIEWED alarm on it indefinitely. The board and the flip gate
  share the expected login, so both surfaces are fixed by the same resolution rather than
  separately.
- The row's diagnostic now names WHICH sha was missing and on which change, instead of a bare
  `compare needs both base and head` that identified neither.
- This is a fix to the CONSUMER, not to any forge backend. On GitLab an approval carries no
  commit sha and — unless the project resets approvals on push — survives a push, so the GitLab
  backend reports no sha rather than stamping the current head; doing otherwise would
  manufacture exactly the at-head evidence the ready-flip gate exists to require. That reading
  is correct and is unchanged. GitLab verdicts only began reaching this code path once a desk
  role's expected login started resolving per-forge, which is why the arm had never been
  exercised there before.
- `--lint` got **much faster, without checking less**. Two changes account for it: the attribution
  cross-check now builds the first/last author of every path in **one** `git log --name-only` walk
  instead of a `git log` pair per brief, and `parseBriefFile` is memoised on (path, mtime, size)
  so the thirty-odd checks that each walk the brief tree stop re-parsing the same file. Measured on
  this repository (24 streams, 165 briefs): git subprocesses per `--lint` **254 → 131**, offline
  wall **6.05 s → 3.90 s**, and with the forge reads gone the same lint that took **23.66 s** now
  takes **3.90 s**. A path the walk cannot account for falls back to the per-path read and a file
  that changes mid-run is re-parsed, so neither is allowed to change a check's answer — only its
  cost.
- `GitLabForge.ReadMergeHold` no longer trusts a released reply's `Head` unless that specific
  reply's own author matches the discussion's resolver. GitLab does not lock a resolved
  discussion against further replies, so any project member with ordinary comment rights
  could previously post a correctly-shaped `assay-merge-hold: released` reply naming an
  unreviewed head into an already-resolved thread and have it read as "approved at current
  head." A released reply from anyone but the resolver is now ignored, reporting `Head: ""`,
  which the reviewer-approved condition already treats as a mismatch requiring re-arm/refusal.
- `cellctl check` on the `gitlab` arm gains a row proving `git ls-remote` succeeds against
  `CELL_REPO` with prompts disabled, using the same helper, so a broken or missing token is
  caught at check time rather than as a boot hang.
- `cellctl desk`'s claude-harness boot no longer prints a misleading "could not enable
  assay@assay — reinstall it" NOTICE when `claude plugin enable` fails only because the
  plugin is already enabled; the NOTICE is still printed for a genuine enable failure.
- `cellctl up --cockpit orca` registers and selects the cell's git CHECKOUT with Orca (`CELL_REPO`) instead of the cell directory, which Orca refuses (`Not a valid git repository`); every `orca terminal create --worktree path:<cell dir>` used to 404 with `selector_not_found` and the operator was told to open the role windows by hand. The `--cwd`-only orca fallback keeps working (the selected path is declared before either branch).
- `cellctl`'s boot fetch on a `gitlab` cell now runs with `GIT_TERMINAL_PROMPT=0` and an inline
  credential helper reading `DESKD_GITLAB_TOKEN_FILE`, instead of hanging on an interactive
  `Username for 'https://gitlab.com':` prompt (or failing with `fatal: could not read Username`)
  — the `github` arm is unchanged, since the operator's `gh` credential helper already answers
  for it.
- `deskboard actions` no longer fails the WHOLE sweep when one change carries a review verdict
  the forge could not pin to a commit. A single such change exited the command 6 with an empty
  stdout and the message `compare needs both base and head`, so a review desk lost sight of its
  entire queue — not just the affected row.
- `deskboard` / the desk tools' `StatusgenPin` reader no longer refuses "no statusgen pin" on a pin file written per `docs/adopting-assay.md` (per-platform `statusgen-<os>-<arch>` lines only): the bare `statusgen` line stays preferred, and when it is absent the reader falls back to the host platform's line (`.exe` on Windows). A malformed bare line still fails closed. `statusgen init` now scaffolds the bare `statusgen` line alongside the per-platform lines so a new adopter gets both (#1088).
- `deskdispatch --kit verifier` now emits a verifier-shaped Assignment header instead of the
  implementer's: no "Open the draft PR" scaffold, no `export DESK_LOOP=worker-desk`, and the
  dispatch claim is released once the verdict lands rather than on a branch push. Before the
  fix, `assemblePrompt` recognized only `--kit review` as non-worker, so `verifier` fell into
  the worker `else` branch and inherited its scaffold wholesale — a verifier agent following
  the prompt literally would have opened a spurious draft PR under the worker App's identity
  for a plain verify pass. (#1029)
- `deskfile attach` can now target a GitLab **issue** whose project also carries a merge request
  with the same number. GitLab numbers issues and merge requests in separate sequences, so `#4`
  and `!4` routinely both exist; the bare-number target read refused that case ("carries BOTH
  issue #N and merge request !N") and, with no way to state the kind, every such number was
  un-attachable (seen on a GitLab adopter cell: `--to 4` and `--to 5` both refused). The verb
  gains `--kind issue|mr` (default `issue` — attach is an observation on an issue; `pr` is an
  alias of `mr`), and the kind drives BOTH the target's state read and the note's endpoint
  (issue notes ↔ merge-request notes), so the check and the write address the same object.
  On GitHub (one number sequence) the kind is only validated against what the number is —
  asking for an issue at a pull request's number is could-not-check (exit 6), never a comment
  on the other kind. An unknown kind is refused (exit 5); the bare-number behaviour of other
  callers is unchanged.
- `deskfile new` on a GitLab project no longer stamps its labels on the MERGE REQUEST that
  happens to share the new issue's number, leaving the issue itself unlabelled. GitLab numbers
  issues and merge requests in two separate sequences, and the label write behind `deskfile`
  only knew how to address a merge request — so `to:<role>` addressing and the label-keyed
  dedupe both silently missed every issue it filed. Field evidence from a GitLab adopter cell:
  three `deskfile new` runs each labelled an unrelated, already-merged MR and left the issue
  unstamped.
- `deskflip` therefore refused `reviewer-approved` on a GitLab merge request that the rostered
  reviewer had APPROVED at the current head, so no GitLab change could ever be flipped
  ready-for-human.
- `deskflip`'s `mergeable` condition on GitLab no longer refuses forever on a brand-new draft
  merge request: `draft_status` and `discussions_not_resolved` are treated as non-blocking
  there, with the reviewer-approved condition immediately after doing the real gating. This
  supersedes an interim same-day fix that mapped `draft_status` to `MERGEABLE` in the shared
  GitLab merge-status mapping — that mapping is reverted to what it was, and the leniency
  moves to the one condition it belongs to.
- `deskflip`'s checks-green could-not-check, when a branch is protected but the admin-free
  rules API names no required contexts (the tell of CLASSIC branch protection, invisible to
  that API), now names the exact permission gap: the calling App token lacks
  `administration: read`, the permission the legacy branch-protection endpoint requires to see
  classic protection's required-checks list. Before the fix the message described only the
  mechanism and never the fix, so every draft PR on any classically-protected repo in the
  fleet read as an ordinary could-not-check and was re-litigated by a human on every flip
  attempt instead of being
  escalated once as a permission grant. `RequiredStatusChecks` itself was already correct —
  it tries the legacy endpoint first and uses it directly whenever it is readable (#760); a
  new fixture pins that direct-success path so a future change cannot regress it once the
  permission is granted. (#1020)
- `deskpost comment` gains `--kind issue|mr`, reusing the `TargetKind` / `GetIssueTyped` /
  `PostCommentTyped` typed forge operations, so it can target a GitLab merge request or issue
  explicitly when the same number names both (GitLab numbers the two in separate sequences).
- `deskpost review` now refuses a body that carries a `Security-Review:` line, mirroring the
  refusal `deskpost security-review` already applied to a body carrying a `Verdict:` line. The
  guard was one-directional: handed a security-lane body — which happened when two reviewer
  lanes dispatched to one PR shared a scratchpad and one lane's default body filename was read
  by the other — `review --verdict approve` submitted a `Security-Review: pass` as an
  **APPROVED** review. That is the exact same-head APPROVE shape the verb split exists to keep a
  security pass out of (a pass posts as COMMENTED so the flip gate can read it while GitHub's
  review roll-up, and any standing CHANGES_REQUESTED from the shared App, are left alone). The
  stray review had to be dismissed by hand. The refusal is exit 5 before any network call and
  names the verb to use; `--verdict request-changes` with a security body is refused the same
  way. The refusal uses the same emphasis-tolerant reader the flip gate uses, so a body carrying
  `Verdict: approve` plus `**Security-Review: pass**` — invisible to the strict write-side parse
  but read downstream as a security pass — is refused too; a `> `-quoted citation of the other
  lane's line still posts. Existing tests that posted security verdicts through `review` now use
  `security-review`.
- `deskpost`'s two largest refusal classes — a review body with no `## ` heading and one with
  no verdict line — now name the offline rehearsal (`--dry-run`) that would have caught them.
- `deskpr create`/`update`/`edit` now ask the public-repo gate's visibility read through the
  SAME resolved forge backend used for every other operation on the change, instead of a
  hardcoded GitHub-only client — a GitLab-resolved repo's visibility is now read from GitLab's
  own API rather than failing closed on a GitHub 401 for a project GitHub has never heard of
  (#1054).
- `deskpushguard`'s pre-push foreign-commit check no longer takes minutes on a checkout with many
  remote-tracking refs. `gitcore.RefsContaining` (the in-process port of `git for-each-ref
  --contains`) walked every ref's entire history independently — ~1000 refs over ~17k commits cost
  ~108s per commit ahead of `origin/main`, enough to blow the desk preflight's 45s write-transport
  probe and keep the review desk from booting. All refs now share one memoised walk, and when the
  repository carries a commit-graph file its generation numbers cut that walk off exactly (a
  topological invariant, never a date). Measured on that checkout: the full pre-push run dropped
  from ~194s to ~1.4s, and the call itself to well under a second. The answer is unchanged and
  still checked against real git's, now including annotated-tag peeling; an unanswerable question
  (target is not a readable commit, broken object store) is reported as an error rather than an
  empty list, so the guard hears could-not-check instead of "no ref contains it". A shallow
  clone's boundary commits are treated as parentless, exactly as git treats `.git/shallow`, so a
  push from a `--depth` checkout still gets a real answer rather than a could-not-check.
- `deskroster width --role <loop>` (the plain, non-`--verbose` read) no longer prints
  `(source=default, expires=n/a)` while a live width override is in force. The trailer was
  describing only the RESERVE field of the stored entry, so a plain `deskroster set --role
  <loop> --width N` — which stores no reserve — read as "default" on the very next read, while
  `--verbose` correctly reported `source="set by <session> at <time>"`; a coordinator reading
  the plain line took a live width for a lapsed one. The plain line now renders the same
  resolved source and expiry the verbose path computes: `(source=set-by:<session>,
  expires=<RFC3339>)` whenever the stored entry is fresh (width and reserve share that one
  TTL), and `(source=default, expires=n/a)` only when nothing is stored or the entry has
  decayed. `--verbose` output is unchanged.
- `desktoken --forge gitlab <role>` now **serialises** rotate-on-mint per role. Two mints for one
  role previously both called the GitLab self-rotation endpoint, which invalidates the token the
  caller presents — the loser got `401 invalid_token`, and the custody file could be left holding a
  revoked value with no live successor, recoverable only by a group owner re-issuing the PAT.
  Overlapping mints now queue on a per-role advisory lock held across read-current → rotate →
  write-verify, so each rotates from the value its predecessor persisted.
- `desktoken` consults its token cache before it resolves the installation id, so a warm cache
  hit makes no network call at all. Previously every "reuse cached token" still read the App
  key, signed a JWT and called `GET /app/installations` to rediscover an installation that
  changes only on install or uninstall.
- `deskwt prune --dry-run` is now genuinely read-only. It ran `git worktree prune` — and,
  with `--reclaim-stale-locks`, unlocked worktrees — before it ever reached the dry-run
  check. It now uses git's own `--dry-run` for the bookkeeping count, reports the locks it
  would retire without retiring them, and writes no singleton stamp.
- `docs/streams/forge-gitlab/brief-11-guard-read-custody.md` cites its consumed changelog fragment through the `CHANGELOG.md` v1.0.8 section instead of the fragment file the release roll deleted, so the board regen on `main` no longer reds on a dangling backticked path (the lint-side question stays open on #722).
- `fanoutloop plan` no longer offers Next-up rows whose own stream README Status cell has already
  moved past `todo`/`in-progress` (implemented/verified/done/blocked): `readNextUp` now
  cross-checks each row against its own stream README before offering it, closing the gap where
  STATUS.md's rendered `## Next up` table could lag a row's live status (medici-finance/assay#1028).
- `repohardenguard` no longer shells out to `gh api <endpoint>` — it reads through the typed
  `RepoHardeningRead`/`ReadFile` ops under the `auditor` identity, closing forge-gitlab/08's
  Verify row 3 (the whole-tree forge-CLI grep) to 0. The checklist's `Read` cell grammar moves
  from `gh api <endpoint>` to `read <kind>` / `read file <path>`; the old form is refused by
  name at parse time.
- `upgrade-assay` and `deskversion` no longer require a hand-materialised `releases/<vX.Y.Z>.yaml` composition manifest that no release publishes: when the manifest is absent they derive the umbrella's composition from the release's published `checksums.txt` — read from `releases/<vX.Y.Z>.checksums.txt` when materialised (offline), else — only under `--fetch` (opt-in, default off; the tools never reach the network without it and refuse naming the flag when neither local file exists), with `fetching <url>` printed to stderr before every contact — fetched from the release home for exactly that tag (`--release-home` re-points it). A hand-authored manifest still wins when present, and a present-but-unreadable one still refuses. Re-pinning now rewrites `<artifact>-<platform>` lines with that asset's own digest from `checksums.txt`, so an adopter pinned per platform can run the sanctioned re-pin verb at all (#1088).

### Changed
- **`deskboard` now applies the same author-trust bar on every repo, private or public**
  (desk-tools/17, #808). The board's PR classifier previously swapped in a STRICTER bar
  (`role App or mapped human only`) on any public/internal/unknown repo, diverging from
  `deskpost`'s gate — a trusted shared automation login authoring a PR on a public repo was
  invisible to the review loop even though `deskpost` would happily post a verdict on it.
  Both tools now answer "may this PR enter the review loop?" with the same predicate
  (`TrustedAuthor`); an unlisted author is still quarantined unless blessed, and merge
  authority is unchanged — a human still merges every PR.
- Brief 10's board row corrected from `todo` to `implemented`: its work merged on 2026-09-11 and
  the hand-maintained row never moved.
- The adopter docs now record the permission `deskflip`'s ready-flip actually needs: the
  **reviewer App requires `Administration: Read-only`**. `deskflip` reads a branch's required status
  checks through the legacy branch-protection endpoint first, and that endpoint is the only one that
  can read a required set the **rules API cannot express** — the rules API surfaces rulesets only,
  and within a ruleset only a `required_status_checks` rule carries contexts. Two different
  configurations therefore look identical to the gate (`protected: true`, no contexts named): a
  branch under **classic** protection, and a branch under a **ruleset that carries no
  `required_status_checks` rule** — say, one whose rules are only `deletion` and `non_fast_forward`.
  The second is the easier one to have by accident. In both, the gate fails closed to
  could-not-check by design and no PR on that repo is ever flipped ready.
  `docs/adopting-assay.md` gains a *Required checks a ruleset does not express* subsection under
  `setup-reviewer-app` — the mechanism, why failing closed is right, and **both** human remedies
  stated neutrally: grant the reviewer App `Administration: Read-only` (the durable fix, and the
  only one that works for a branch genuinely under classic protection), or add a
  `required_status_checks` rule to the branch's ruleset (no permission change needed, but it fixes
  only the ruleset case and changes what the forge enforces at merge time). The grant's sequence is
  spelled out — toggle, accept on each installation, then re-mint, since an issued token keeps its
  old scopes for `desktoken`'s 50-minute cache window. Every place that enumerated the reviewer
  App's grant now names the permission: the App inventory table, the post-install checklist, the
  primitive, the Verify read-back, and Scenario 2's fleet-wide install step.
  `docs/enforcement-model.md` records that role-scoped **reads** sit outside `requiredDuties`, so a
  missing one costs no boot, only every flip. Which roles: the reviewer App REQUIRES it (each
  instance separately — the grant is per App installation, so a cell's own reviewer twin needs its
  own); the desk App SHOULD have it for board reads of the required set; worker, verifier and the
  inbound-lane Apps do not. The `deskapps` tier manifests gain it too, so a future install is not
  born blocked. `docs/adopting-assay-gitlab.md` states the GitLab equivalent honestly: there is no
  toggle of that shape, the equivalent reads are protected-branch + external-status-check API calls
  under the plain `api` scope, and whether the reviewer's Developer level can make them is
  **could-not-check** until read back on the instance. (#1020)
- The audit ledger is no longer parsed whole to answer questions that need one line.
  `Guard()` reads the last entry by a bounded tail read, and the rate limiter reads
  backwards only as far as its own meters would have read — falling back to the full parse
  whenever its answer is not provably identical, so no budget, breaker or idempotency
  verdict changes. Measured on a 109 MB / 460,000-row ledger: the guard's p50 goes from
  883 ms to 0.09 ms (#1035).
- The critical path is re-cut into three tracks, with the open one first. **Its verified head is
  brief 14**: two of the seven verbs in a review tick still send the public-repo gate's live
  visibility read to a hardcoded GitHub client instead of the backend they already resolved, so
  on a GitLab project the desk's workpad and its Evidence landing have no working path.
  Two tempting-but-wrong heads are recorded with what was checked to rule each out.
- The desk-role bodies now say what a scheduled, one-shot invocation should do. Previously
  they described only the standing window, so a role invoked as a bounded job could end only
  in its own deadline, with nothing printed.
- The head moved while the plan was being written, and the plan says so. It started at brief 13 —
  on GitLab a review approval carries no commit sha, the board's benign-merge arm asked for a diff
  against that empty sha, and the per-change refusal came back as a whole-sweep error, blanking
  the queue. That arm was fixed mid-pass and the board is visible again, so the blocker moved one
  step down the ceremony within the hour. Brief 13 stays on the plan re-baselined onto what
  landed: the instance is closed, the class of four further unguarded whole-sweep returns is not.
- The ledger rotates daily into `audit.jsonl.<YYYY-MM-DD>` segments. Nothing is deleted and
  no state is reset: every reader — including `deskaudit recover` — spans the segments, so
  the rate-limit counter and the idempotency store see exactly the history they saw before.
- The stale-issue alarm on the `--lint` gate is now opt-in and reads **open issues only**, through
  the seam. It previously shelled one list-every-issue-ever call per configured repo, serially,
  under every `--lint` — measured at 13.9 s across ten repos on this repository — to print one
  advisory line whose inputs were open issues all along. It is also withheld entirely when any
  repo in the set could not be read: a debt count assembled from a subset understates the debt,
  and an understated alarm reads as "we looked and it is fine".
- `deskboard` brief 27 is authored: the plan to put `prs`, `stalled` and the always-on
  policy-drift probe onto the bounded worker pool `actions` and `health` already use, to make
  `throughput` resolve its roots once instead of re-running three whole verbs, and to evaluate
  `deskflip`'s conditions cheapest-first so the commonest refusal (`checks-green`) stops being
  the most expensive one to reach.
- `deskboard`'s last three serial repo loops run on the bounded worker pool `actions` and
  `health` already used: `prs`, `stalled` (repos AND, inside each, that repo's PRs) and the
  always-on policy-drift probe that rides inside `actions`. Measured back to back on one
  operating desk host with a ten-repo roster, structurally identical output: `stalled`
  84.82s → 12.63s, `prs` 19.82s → 6.63s, `actions` 33.59s → 24.04s.
- `deskflip` evaluates its conditions cheapest-first, as far as a recorded diagnostic rule
  allows: `mergeable` moves from seventh to fourth (it costs no forge read at all) and
  `model-floor` from fourth to seventh (it buys a paginated label-event timeline). A refusal
  on a CONFLICTING PR now costs one forge read instead of four. The check rollup at head,
  previously fetched twice by two conditions in the same run, is fetched once — each still
  reporting a failure under its own condition's name.
- `desktoken` no longer appends an audit row when it serves a cached token. One row per
  real mint, and every refusal and failure still recorded.
- `deskwt prune` removals are **batched**: one `git worktree prune` and one
  `git worktree list --porcelain` after the loop, instead of both per removal. The positive
  deregistration check is kept, not dropped — every removed path is verified against that
  single listing, and one that is still registered is reported by name and not counted as
  removed.
- `deskwt prune` — the boot-step sweep every desk window runs — no longer asks one question
  of one history once per candidate. It walks `origin/main` **once per sweep** into an
  ancestor-hash set (the per-candidate merge test becomes a HEAD resolve plus a map lookup,
  replacing go-git's unmemoized ancestor walk, which ran to exhaustion for the 19-in-20
  candidates that are genuinely unmerged); runs the merge gate **before** the full-worktree
  `Status()` it used to run first, so that walk happens only for candidates that can still
  be removed; shares **one object cache** across the sweep instead of building a fresh one
  on each of the 3–4 repository opens per worktree; and drops the commit COUNT from the
  "unpushed" skip string, which cost two full history walks per held worktree to render a
  number nothing parses. Measured on a synthetic repository with a 5,000-commit mainline:
  the per-candidate ancestry work alone was ~372 ms/candidate before; a whole sweep over
  600 worktrees is now ~2.8 s, with `origin/main` walked exactly once.
- `dispatch` and `awaiting` resolve their stream roots through one shared resolver and read
  them concurrently, and `throughput` resolves them once for the whole run instead of twice:
  47.65s → 29.10s, identical output. A failed shared resolution blinds BOTH stages it fed,
  each naming it — never a counted zero.
- `docs/adopting-assay.md` and `docs/adopting-assay-gitlab.md` document the new `auditor`
  identity (GitHub App permissions, GitLab role/token) per the #857 ruling's docs half, and
  also close a pre-existing gap where the `cell-issues` role was undocumented on both pages.

## v1.0.8 — 2026-09-14

### Added
- **A top/mid/fast tier-map fallback** for a role/harness with neither its own per-role pin nor that
  harness's default: `the-desk` resolves at `top`, every other role at `mid` (`fast` is defined but
  not auto-assigned). Compiled defaults — `fable`/`sonnet`/`haiku` for claude,
  `gpt-5.6-terra` (the one codex model id proven live, `docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md`)
  for all three codex tiers today — are each overridable in `cell.env` via
  `TIER_MODEL_<TIER>_<HARNESS>`. This is what lets a cell pinned Claude-only today boot
  `--harness codex` with a real, working model and no manual re-pin.
- **Per-harness model-pin namespaces.** `DESK_MODEL_<role>`/`DESK_MODEL_DEFAULT` are now explicitly
  the **claude** namespace (unchanged, backward compatible); codex gets its own —
  `CODEX_MODEL_<role>` per-role, `CODEX_MODEL_default` as its harness-wide fallback (no compiled
  default). The two are never cross-read.
- **`forge-neutral` brief 14 — run and gate-approval verbs (`deskrun`) onto the forge
  resolver.** Specifies `RunWorkflow`, `ApproveGate` and `RunStatus` on the `Forge`
  interface, both backends, plus the `deskrun` verb that wraps them: today a release or a
  gated CI run is started by a human's own ambient `gh` session because GitHub's
  `actions: write` permission cannot be scoped down to just dispatch-and-approve (it also
  cancels runs, deletes run logs, and disables workflows repo-wide). The brief documents
  `repository_dispatch` as an alternative trigger and states why it is not adopted as the
  default, prefers GitLab's narrow, start-only pipeline trigger token over a broader
  project token, and specifies `deskrun`'s refusal (exit 5) whenever the roster's
  run-credential binding resolves to a human rather than a dedicated role credential. Doc
  only; no tool behaviour changes in this PR — implementation is a separate, human-gated
  follow-on.
- **`forge-neutral` brief 16 — `deskclose` widened lanes.** Specifies three narrowly-scoped
  additions to `deskclose`'s closed mode set, each staying inside the identity model
  `forge-neutral/13` already put a human gate on rather than opening a new one: (a)
  `self-withdraw`, letting an author-App close its own superseded-or-abandoned draft pull
  request, gated by a login-AND-numeric-id authorship pin mirroring the blessing-authority
  check; (b) `verify-gate-refire`, a `verifier`-role-only reopen+close cycle scoped to
  `verify-gate`-labelled issues, whose inability to complete the human sign-off is enforced
  independently and server-side by `verify-gate-close.yml`, not by this lane's own gate; and
  (c) documentation of `deskclose manifest` as the already-shipped, sanctioned path for a
  human-ruled batch close, with no behavior change. Adds one new `Forge` operation
  (`ReopenIssue`, both backends).
- **forge-neutral/17 — `deskrun log`/`deskrun retry` brief**
  (`docs/streams/forge-neutral/brief-17-deskrun-log-retry.md`): specifies read-only run-log
  access (GitHub `actions: read` / GitLab `read_api`) as safe to grant broadly to both worker
  and reviewer Apps, and `deskrun retry` (GitHub `actions: write` / GitLab `api`) as
  roster-bound the same way `RunWorkflow` is — the same over-broad-scope shape, refusing
  (exit 5) rather than borrowing an ambient human credential when the roster binds the retry
  role to a human.
- An explicit `--model <m>` (or `DESK_MODEL_OVERRIDE`) still passes through verbatim to the selected
  harness on either arm — it is never routed through the namespace/tier resolution above.
- New `deskprovenance` verb and `internal/deskkit/provenance.go` gather a fixed set of
  mechanical signals about an unknown contributor's pull request (account age, fork-to-PR
  elapsed, cross-repository burst, prior merged/closed ratio, body-shape similarity, commit
  signature, build/dependency paths touched) and render them as a neutral, facts-only card —
  no score, no rating, no verdict. See `docs/contributor-provenance.md`. Posting the card
  publicly is pending the human ruling recorded on `docs/streams/decisions/DR-provenance-card.md`
  (contributor-trust/01).
- `cellctl --version` (also `cellctl version`) reports the umbrella release tag a packaged copy
  ships at, stamped into the tarball's copy at release time; a source checkout keeps reporting
  `dev`, honestly.
- `cellctl check` prints one `model pin: role=<role> harness=<harness> model=<m> (from <source>)`
  row per role the cell runs, on the cell's pinned harness — a role with no per-harness pin and no
  tier match is a `MISS` naming exactly what was checked, visible before boot rather than discovered
  as a startup failure.
- `cellctl set <cell> <role> [--harness claude|codex] --model <m>` — role-sugar that writes whichever
  namespace the ACTIVE harness uses (the flag given, else the cell's own `CELL_HARNESS`), so a codex
  call writes `CODEX_MODEL_<role>`, never `DESK_MODEL_<role>`. `cellctl desk ... --harness codex
  --model <m> --set` persists into the same namespace for a live boot.
- `cellctl` now ships inside `desk-tools-<platform>.tar.gz` (every platform gets the same file —
  it is a shell script, not a per-platform Go build) and `make desk-install`, sha-pinned by the
  umbrella `checksums.txt` like every other desk-tools asset. No more hand-copied script.
- `deskroster liveness --repo OWNER/NAME` — a read-only NOTICE surface that asks GitHub
  what it currently says about every trusted login the roster configures (deleted, renamed,
  reclaimed, or unpinned), without touching `TrustedAuthor`/`TrustedHumanAuthor`/`Blessed`'s
  pass/fail verdict.
- `statusgen reconcile --backfill --apply` now WRITES a witnessed `todo`/`in-progress` →
  `implemented` cell back into the brief's stream README `Status` column — closing the
  wiring gap where `--backfill [--report]` only ever reported drift, never applied it.
  The write fires only for a real merged-PR witness (a `Brief:` trailer or the declared
  backfill branch/body match), never touches `Verified`/`Reviewed` or any `human:<name>`
  sign-off stamp, and never writes `verified`/`done`. Idempotent — a re-run with nothing
  witnessed exits 0 having written nothing.
- `tools/cellctl/tests/model-namespace.test.sh` — a plain-bash, no-network test covering the
  tier-map fallback with no manual re-pin, the unaffected claude arm, per-role/default codex pins
  winning over the tier map, `--model` bypassing all resolution, the no-pin/no-tier-match refusal on
  both `desk` and `check`, the `TIER_MODEL_<TIER>_<HARNESS>` override, and the harness-aware
  `cellctl set` role-sugar form. `tools/cellctl/tests/harness.test.sh` is updated where it asserted
  the old (buggy) pass-through behavior.

### Fixed
- **`--harness codex` no longer passes a Claude model name straight to `codex -m`.** `cellctl
  desk`/`cellctl up` previously resolved a role's model from `DESK_MODEL_<role>`/`DESK_MODEL_DEFAULT`
  regardless of harness, so a cell pinned Claude-only (`fable`, `opus`, `sonnet`, ...) handed codex a
  name it does not understand, and the reverse (a codex-only pin reaching the claude arm) was
  equally broken (`#986`).
- **`cellctl up --cockpit herdr` now brings up a herdr window when none is open, instead of
  silently doing nothing.** Previously the herdr arm only ever added labelled tabs to whatever
  window herdr already had open (`herdr tab create --label <l>`) — with no window open, those tabs
  had nowhere to land and the operator had to open herdr by hand first (#985). `up` now checks
  first (`herdr workspace list` — herdr's own noun, verified live against herdr 0.8.2, for what
  this cockpit and #961 call a "window") and, finding none, starts one
  (`herdr workspace create --label <cell>-<the first window>`) before any tab create — the same
  trigger point and create-if-absent shape the tmux arm already uses for its `<cell>-cell` session
  (`tmux has-session || tmux new-session`). The new workspace's own auto-seeded default tab is
  dropped once the cell's real tabs exist in it (closing it any earlier closes the whole workspace
  with it — verified live). When a window is already open, behaviour is unchanged: no
  `workspace create` call, and tab create runs exactly as it did before this fix. A build that
  cannot list or create workspaces, or whose `workspace create` fails, refuses up-front naming
  herdr and the exact command tried — never opens no window at all. `--cockpit auto` still falls
  through to tmux when herdr is not installed, as before. `tools/cellctl/tests/herdr-orca-launch.test.sh`
  covers all three cases (no window / window already open / herdr absent or lacking the verb),
  each against fixture stubs shaped from live probing of a real herdr 0.8.2.
- The stale-verdict refusal's diagnostic commit now names the SAME decisive security review
  that governs the pass/fail decision, never a later off-head review that never actually
  governed anything. A standing `Security-Review: fail` at commit A followed by a LATER
  `Security-Review: pass` pinned at an off-head commit B previously made the refusal claim
  the verdict was "pinned at B" — a plain staleness framing — when the real reason to refuse
  was the standing fail at A. `securityVerdictStanding` and `lastSecurityVerdictCommit` now
  share one reduction so they can no longer name different reviews. (#988)
- `composability/00`'s Verify row 1 documented a `grep -c '^\| \`assay\.'` check that was
  false-permissive on both GNU and BSD grep: the escaped `\|` parses as a GNU alternation
  extension, so the pattern matched every line via its `^` branch and silently counted the
  whole file instead of `components/KEYS.md` table rows. Documented the corrected,
  unescaped-pipe form (confirmed to return the real row count, 30, instead of the file's
  total line count, 97) in a dated Evidence addendum on the already-`done` brief, per the
  house mid-flight-edit convention (#906).
- `consumedFragmentIndex.build()` (the `changelog/<slug>.md` consumed-release-fragment
  exemption, #722) now detects a shallow git clone (`git rev-parse --is-shallow-repository`)
  and treats it the same as a `git log` error, instead of reading its truncated history as a
  definitive "never tracked" answer. On CI's default shallow `actions/checkout`, that false
  read spuriously PROBLEMs a genuinely consumed changelog fragment on every PR, unrelated to
  the diff — this makes the failure an honest could-not-check instead of a silent wrong
  verdict. The companion fix — giving the `lint` and `windows-smoke` jobs `fetch-depth: 0` so
  CI actually greens on a real clone — is tracked separately, blocked on the worker App's
  standing no-workflow-write policy. (#999)
- `deskclose superseded`'s confirm/dispute now normalizes both the standing proposal's
  recorded target and the caller's `--by` value to `owner/repo#N` before comparing — a
  bare `--by 40` against a marker recorded as `owner/repo#40` used to read as a target
  DISAGREEMENT ("record and caller disagree") when it was only a format mismatch. A
  genuine disagreement now names both the recorded form and the expected form explicitly
  (#984).
- `deskdisposition read` no longer shells out to `gh pr view` at all: it mints the
  session-role App installation token and reads the record (labels + comment thread)
  through the resolved forge over REST, the same custody pattern `deskclose`/`deskfile`
  already use. Previously, `deskclose superseded`'s confirm path (a child process of an
  already-minted desk session with no usable ambient `gh` identity) got a bare
  `HTTP 401: Requires authentication` from the shelled `gh` call even though a valid
  App-token read was available (#984).
- `deskevidence`'s secret scan no longer re-scans a brief's WHOLE pre-existing body on an
  Evidence landing made without `--brief-path`. In that flow `--evidence-file` is the caller's
  own merged copy of the target file, so the scan previously treated every pre-existing byte —
  a Verify row's own quoted secret-shaped material — as part of THIS commit, forever. It now
  fetches the target's current remote content once, up front, and scans only the lines that
  are actually new relative to it (the same "added bytes only" scoping `--brief-path` merges
  already had). (#966)
- `deskflip`'s already-ready fast path no longer treats "not a draft, queue label already
  correct" as proof the review verdicts are current. It now re-runs the FULL condition gate —
  including the reviewer-approved and security-verdict lanes AT THE CURRENT HEAD — before
  reporting "nothing to do", and refuses (naming the stale lane and both the stale-verdict
  commit and the PR's current head) when a lane's last-seen verdict is behind the head.
  Before the fix, an already-ready PR could sit reading as mergeable indefinitely while new
  commits landed with content no review lane had seen. (#987)
- `docs/adopting-assay.md` §1's install arm A (plugin/marketplace) is recorded as demonstrated, not
  merely documented: a clean Codex home on 0.154.0 ran
  `codex plugin marketplace add` → `codex plugin add` → `codex plugin list` end to end against this
  repo's legacy manifest, with all twelve skills discoverable namespaced `assay:<name>`. (#939)
- `docs/adopting-assay.md` §3's `multi_agent_v2` could-not-check is narrowed: `codex features list`
  on 0.154.0 confirms the key exists (`stable`, defaults `false`); only the V2 tool-name behaviour
  itself remains unexercised. (#939)
- `internal/deskkit`'s secret/entropy scanner gained two narrow, closed exemptions so neither
  shape can trip a false positive again even when correctly scoped to newly-added text: a
  slash-list of short ALL-CAPS enum/status words (`PENDING/RUNNING/BLOCKED/DONE`), and a 32-hex
  run directly behind a hyphen and a recognised Kubernetes object-kind prefix (`pvc-<uid>`, the
  dashes-stripped rendering Kubernetes itself emits for a PersistentVolumeClaim's bound
  PersistentVolume). Both are bounded the same way every other exemption in this file is: a
  real secret pasted in either shape (mixed case, digits, or an unlisted prefix) still refuses.
  (#966)
- `plugins/assay/references/codex.md`'s `capability:isolate-workspace` row (and the two skill rows
  that derive from it, `pr-shepherd` and `worker-desk`) re-measured on **codex-cli 0.154.0**: the
  CLI now ships a managed-worktree mechanism (`codex exec --worktree`), so the prior `§3.9 absent`
  reading is stale. The floor behaviour is unchanged — still refuses under `workspace-write`,
  still runs under `danger-full-access` — only the mechanism classification updates. (#939)
- `tools/changelog/check.sh`'s `PR_NUMBER` validation now anchors the whole
  parameter (`[[ =~ ]]`) instead of anchoring per-line via a piped `grep -E`,
  closing an embedded-newline bypass that could launder one PR's number into
  matching another PR's proxy changelog fragment; the downstream proxy-lookup
  `grep -E` match gained its own newline guard as defense in depth. Not
  exploitable in production (`github.event.pull_request.number` cannot carry a
  newline) — a hardening fix plus a new regression test (`check_test.sh` P13).
- board: reconciled two merged-but-unflipped stream README rows
  (`desktools-go-git/04`, `desktools-go-git/07`) to `implemented` — both rows' delivery
  PRs (#951, #958) were already merged, reviewed, and green, but the board Status cells
  had never been flipped off `dispatched`.
- statusgen's human-stamp sole-permitted-writer check no longer false-positives on Windows: `relPath` now normalizes backslash separators before building git pathspecs, so a legitimately gate-written `human:<name>` sign-off stamp reads clean on `windows-smoke` the same way it already did on Linux/macOS (#1007).

### Changed
- Codex smoke run log for 2026-09-13: harness-portability/07 step 5 re-run under the #939 re-baseline (spawn tools present under `multi_agent=true`; fan-out held by the claim rule), recorded under `docs/codex-smoke-runs/` with transcript fingerprints.
- Two stream README rows (desktools-go-git/02, windows-port/04) return to `implemented`: their `human:reviewer` Reviewed cells had been written by the board-writer bot and the brief-v2 migration rather than by `verify-gate-close`, which `statusgen --lint` refuses; re-closing the two gate cards lets the gate write the stamps itself.

## v1.0.7 — 2026-09-13

### Added
- **New `contributor-trust` stream — being ready for external humans and agents on a public
  repository.** The first unsolicited fork pull requests from unrecognised accounts arrived on
  2026-09-12, and the inbound bar they met is binary: an identity is in the operator's roster
  or it is a stranger, one item is blessed or it is quarantined. Nine briefs in three waves
  add a four-tier vocabulary (`unknown → blessed-once → contributor → maintainer`) recorded in
  an operator-side per-repository ledger that never lands in this public tree, a structured
  blessing act with a scope, a reason and an audit row, review depth keyed on tier (a
  claims-versus-diff fact check and a fail-first reproduction for unknown authors), an audited
  fork continuous-integration posture with never-build-unblessed enforced by a check rather
  than by memory, a neutral provenance card that states mechanical signals and renders no
  verdict, a published contributor-facing statement of the whole bar with a pull-request
  template, a stated position on agent-authored contributions, a counted board view of
  external pull requests by tier and outcome, and release-note credit for the author a fork
  change came from — the release aggregator lifts fragment bullets and nothing else today, so
  a fix merged from a fork is credited nowhere. Critical path `02 → 03 → 05`; the six
  risk-gated briefs each cite one of five new PROPOSED design-decision records.
- **Per-forge leak-gate verdict surface** (`docs/streams/forge-neutral/leak-gate-shape.md`):
  where the disclosure verdict lands on each forge, whether it blocks, and the three-state
  contract — a missing verdict reads as could-not-check, never as a pass.
- **Pipeline-side leak-sweep job** for GitLab CI
  (`docs/streams/forge-neutral/gitlab-ci-half.md`): the free-tier compensator that runs the
  in-tree controls in the change's own pipeline and fails it, the blocking layer on GitLab CE.
- **Pull-request template and a changelog note for fork contributors** (contributor-trust/06,
  partial). `.github/PULL_REQUEST_TEMPLATE.md` asks two short questions on every pull request:
  which claims the description makes and how each was checked, and whether the change was
  produced with the help of an AI coding tool or agent — neither is checkable by any tool, so
  the value is that an honest answer is cheap and a false one is a specific statement a
  reviewer can point at. `CONTRIBUTING.md` gains a plain explanation of the fork
  changelog-fragment proxy: a fork pull request missing a fragment is not something the
  contributor has to fix, because a maintainer lands it on the base branch on their behalf.
  The rest of this brief — the trust-tier model, the provenance-comment disclosure, and
  `docs/contributor-trust.md` — is deferred pending the open decision on how much of the trust
  model to publish (`medici-finance/assay#963`).
- **Scoped brief `forge-gitlab/10` — the GitLab trust-events read + commit author-login.** Planning-only:
  the brief scopes (does not implement) bringing the GitLab backend's `PRTrustEvents` trust read and
  `GetCommit` author/committer-login resolution to parity with GitHub, so a GitLab review desk can form
  the trust verdict `deskpost`'s review precondition chain requires and read a commit's attributed
  identity. It closes one named blocker of the deskpost-verdict wiring (#798; write ops + reviewer PAT
  auth landed in #800) and adds brief-10 to the `forge-gitlab` stream README status table and
  minimum-tier matrix.
- **The `verifyrun` execution witness names the acting forge identity.** The witness `Runner` now
  resolves the git identity through the roster for the repo's forge ahead of the CI-env and git-config
  fallbacks, and records which source produced it (forge-identity / ci-env / git-config), so a stamped
  acting identity is distinguishable from a host-derived one. The no-identity and forbidden-runner-flag
  refusals are unchanged: the runner stays derived, never caller-supplied.
- **The public-repo trust gate can now run on a GitLab-resolved repo (#798).** A new
  `GitLabRepoInfoFetcher` adapter exposes the GitLab backend's visibility and reaction reads
  under the string-signature `RepoInfoFetcher` the gate consumes, so `PublicRepoGate` enforces
  the same +1-from-an-authorized-human requirement on GitLab that it does on GitHub — reusing the
  backend's existing, tested reads, adding no fall-open path (an unreadable visibility still fails
  closed).
- **The standing truth suite is live.** `truth-suite.yml` was promoted from `ci/staged-workflows/` to `.github/workflows/`: the baseline test corpus plus the release mutation gate now run on every push to the default branch and on a daily schedule, reporting three-state. Closes #740; completes sdlc/03 rows 6-7.
- **`--provider <name>` / `CELL_PROVIDER`: a model endpoint and credential switch, independent of
  `--model`.** `--model` only ever changed the model *name* — a non-Anthropic model
  (`--model glm-5.3`) still talked to Anthropic and failed, because nothing switched the API base
  or the credential. A provider name resolves to `CELL_PROVIDER_<NAME>_BASE_URL` and
  `CELL_PROVIDER_<NAME>_TOKEN_ENV` in `cell.env` — the latter names an environment variable
  (never a token value) the launching shell is expected to carry — and `cellctl desk`/`up` export
  `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN` from those before exec'ing `claude`, printing
  `provider=<name>` on the launch line and in `DRY_RUN=1` output. Missing any piece is a refusal
  naming exactly what's absent — the provider's base URL, its token-env variable name, or that
  variable being unset in this shell — never a silent fall-through to Anthropic. `CELL_PROVIDER` in
  `cell.env` is the default (unset = Anthropic, unchanged behaviour); `--provider` overrides it for
  one run, threading onto every role window `up` opens the same way `--model` does.
  `cellctl check` carries the default provider's three preconditions (n/a when unset, since a
  provider is opt-in), and `cellctl set` accepts `CELL_PROVIDER`/`CELL_PROVIDER_<NAME>_*` without
  `--force`. `tools/cellctl/tests/{down,herdr-orca-launch,provider}.test.sh` cover all of the above
  offline, against stubs shaped from live probes of the real herdr/orca binaries.
- **`cellctl up` is cockpit-aware: herdr, orca or tmux, chosen by what is on PATH.** A cell's role
  windows were stood one way only — a tmux window per role — so an operator running a cockpit with
  labelled tabs and a semantic agent state, or one with scheduled automations, still got tmux.
  `cell.env` now carries `CELL_COCKPIT` (default `auto`; `tmux` | `herdr` | `orca`), scaffolded by
  `cellctl new` and overridable per run with `cellctl up --cockpit <value>`. `auto` resolves by
  presence on PATH — herdr first, then orca, then tmux — never a flag someone has to remember; an
  `orca` binary whose desktop app does not answer a cheap, time-bounded probe **falls through** to
  tmux rather than failing, because its CLI is a thin client of that app. An **explicit** cockpit
  that is not available is a refusal naming exactly what is missing, never a silent fall-through.
  Herdr opens one labelled tab per window (`<cell>-<role>`), each started as a `claude`-kind agent
  under that label so the cockpit's agent state drives its sidebar per desk; orca opens one
  terminal per role running the same `cellctl desk` command, or — with the opt-in
  `cellctl up --automate '<cron>'` — one scheduled automation per role fronted by the exit-code
  precheck `cellctl check <cell>`, so a tick on a cell that is not fit to boot launches no model.
  Every `up`, `down` and `check` prints the resolved cockpit and the reason
  (`[cockpit] herdr (auto: on PATH)`, `[cockpit] tmux (orca on PATH but app unreachable)`), and
  `DRY_RUN=1 cellctl up <cell>` prints that plus the per-role commands and launches nothing.
  Nothing else about a cell changes with the cockpit: the per-role locked worktree, the roster
  beacon, the pinned model and the shim `PATH` are identical in all three, and tmux behaviour is
  unchanged.
- **`cellctl` gains a `house` cell kind (#845).** `cellctl new <cell> --kind house --repo <checkout>
  --roots '<owner>/<repo>=<abs path>,...'` scaffolds a LOCAL cell for the operator's own desks: the
  cell home reaches the operator's existing config home (roster, App keys) by one symlink and copies
  nothing, no `deskd` is required unless `cell.env` sets `DESKD=1`, and `cellctl desk <cell> <role>`
  boots each role in its own LOCKED worktree off a fresh `origin/main` with `DESK_ROOTS` (from
  `CELL_ROOTS`), `DESK_LOOP` and `DESK_SESSION=<cell>-<role>-<UTC stamp>` exported — the one-command
  replacement for the hand boot that could start a desk inside a shared checkout. `cellctl check`
  on a house cell proves the checkout, a PARSING roster, every root carrying `docs/streams/`, the
  desk verbs and the enabled plugin. `CELL_KIND` defaults to `k8s`, today's behaviour.
- **`cellctl` is forge-aware**: `cellctl new --forge github|gitlab`, with `--deskd-app-pem`
  and `--orgs` required on the github path only and a hand-provisioned GitLab role token store
  on the gitlab path; `deskd` and `check` follow per forge, and the hardcoded `api.github.com`
  host is replaced by the cell's configured forge endpoint.
- **`claimLiveness` is now a typed Forge op with a GitLab backend (#798).** The
  model-capability floor's stamp age-out read a PR's dispatch-claim ref through a GitHub-only
  hand-rolled REST call; it is now the enumerated `Forge.RefExists(repo, ref)` op, with the
  GitHub logic moved onto the seam and a GitLab backend that reads the ref through the Branches
  API (Free tier). Like `DeleteRef`, it validates the ref path before any request and reaches
  only the `heads/` namespace on GitLab CE — a ref outside it is a could-not-check refusal, never
  a guessed "absent" that would report a held claim as released. A 404 is the answer "absent"; a
  403 stays could-not-check.
- **`docs/adopting-assay.md` now carries "Running Assay on Codex"** — the per-harness adoption runbook that the v0.3.0 release note had claimed for a version that never shipped it. Six sections at the depth of the existing Cursor arm: the two install arms (the `codex plugin marketplace` path, and the `.agents/skills/` file-placement path that depends on nothing but `SKILL.md` discovery); the generated `AGENTS.md` resident-rules fragment and the `project_doc_max_bytes` cap that can silently truncate it; the `[features] multi_agent` config step with `[agents] max_concurrent_threads_per_session`; the `workspace-write` vs `danger-full-access` sandbox posture that decides which skills **refuse**; the degradation rule (three guarantees never degrade, convenience degrades only by saying so) pointing at `plugins/assay/references/codex.md` rather than duplicating its table; and the acceptance split — structural truth in CI, behavioural truth blocked on a live Codex session that has not happened.
- **`statusgen` recognises the acting FORGE identity (forge-neutral/07).** The roster parser now
  accepts the forge-qualified `[role=]<forge>:<slug-or-login>[:<id>]` grammar (unqualified reads as
  github, recorded as inferred), mirroring the desk-tools reader. The Evidence-actor lint matches an
  Evidence committer by the accepted verifier's forge — id-pinned on GitHub, GitLab service-account
  address shape + username on GitLab — so a correctly-verified GitLab row reads BACKED instead of the
  false "0 rows are backed" the GitLab pilot hit. A verifier bound to a forge the build does not
  understand is could-not-check naming the forge, never a pass.
- An https push url with no App credential helper configured now prints a stderr **NOTICE**
  (never a refusal): the ambient-identity shape one layer along, reported as could-not-check
  rather than as a pass.
- Authored `docs/streams/desk-tools/brief-22-trust-gate-account-liveness-notice.md`: a
  read-only, fail-closed liveness check for the trust gate's configured logins, surfaced as a
  `deskroster liveness` NOTICE only — it does not change who `TrustedAuthor`/`TrustedHumanAuthor`
  trust today (closes #933; auto-revocation is tracked as separate follow-up).
- Component-model ledger + `disable` verb (composability/02): `deskkit/ledger.go` is the append-only writer/reader for `.assay/ledger.jsonl`, recording what an outside apply step created so it can later be found and compensated. `deskdisable <component> [--dry-run] [--yes] [--cascade a,b,c]` replays a component's own apply steps in LIFO order — inside steps are reversed for real by a registered executor (assay/streams-scaffold, assay/main-guard, assay/registers-scaffold so far); outside steps are never auto-compensated in this build and print a human checklist line instead, enriched from the ledger. Refuses (touching nothing) when the component is unknown, when disabling it would strand an active dependent not listed in `--cascade`, or when an inside step has no registered automatic reverse.
- Contributor trust tiers (`unknown` < `blessed-once` < `contributor` < `maintainer`)
  and an operator-side ledger reader (`ResolveTier`), so a repeat contributor can
  carry a recorded standing instead of being assessed as a stranger on every
  submission. The ledger never lands as a file in this repository; see
  `docs/contributor-trust.md`. Inert on landing — no caller is wired to it yet.
- Every spawn is gated by the kill switch (checked before each spawn, not once per drain
  cycle) and a per-hour firing budget (`deskkit.AllowWrite`, scoped per dispatch target);
  an exhausted budget fires zero sessions and leaves the item to the engine's own
  retry/backoff rather than fabricating a result. Every spawn and every permission
  decision is one audit line, for the daily lane-violation sweep.
- Fetch over SSH stays allowed — `remote.origin.pushurl` is what the gate reads whenever it
  is set — and `deskpr edit`, which pushes nothing, is not gated. With `$DESK_LOOP` unset the
  gate is inert: a human at a terminal pushes under their own key.
- GitLab Ultimate refinements (forge-gitlab/06): `create-fleet-gitlab.sh --tier ultimate` scripts a custom reviewer role that cannot push (Reporter base + `admin_merge_request`) and registers an external-status-check verdict lane; each no-ops as could-not-check on a lesser instance rather than downgrading silently.
- GitLab backend now serves `PRTrustEvents`/`IssueTrustEvents` and resolves commit author/committer logins, so the `deskpost` trust gate can form a real verdict on GitLab instead of stopping at a could-not-check stub (forge-gitlab/10, #887 item 1).
- New portable methodology skill `plugins/assay/skills/human-runsheet/SKILL.md` — writes the acts
  owed to the driver (a guard refusal, a scope a role's token lacks, a permission its App must not
  hold, a human-only gate override) as exact `! <command>` runsheet entries, distinct from
  `ask-decision` (a decision) and `author-drive-plan` (a multi-session operation record). Each of
  the five desk-role skill bodies now points at it from its own escalation step.
- Recorded, with measurements, that **v1.0.6 supersedes v1.0.5 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 through v1.0.5 re-pins recorded. An
  adopter already on v1.0.0 through v1.0.5 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.6 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- Release notes credit the author a fork change came from: `release.yml` now resolves each changelog fragment to its pull request, decides roster-vs-external with the same roster variables the desk tools use, honours the documented opt-out marker, and passes the credits map to the aggregator so external contributions read `… — thanks @<login>` (contributor-trust/09). The release checkout is deepened so the resolver can see history.
- Release notes now **credit the external contributor** a change came from. At each
  cut the aggregator resolves every `changelog/` fragment back to the pull request
  it arrived on — from git alone — and appends ` — thanks @<login>` to that
  fragment's bullets when the author is somebody the operator's roster does not
  already list. A maintainer, a mapped human or a role automation account is never
  thanked, a contributor can opt out with `<!-- changelog-credit: no -->` on a line
  of its own in the pull-request body, and a credit that cannot be resolved is a
  named line in the release log — never a refused cut. See `changelog/README.md`,
  "Credit in the release notes".
- The GitLab forge seam can post lane verdicts as an external status check against the MR head SHA at Ultimate, with a three-state (could-not-check) fallback when the tier does not expose the endpoint.
- The codex arm execs `codex --sandbox danger-full-access -C <worktree> -m <model> "Invoke the
  \"assay:<role>\" skill now."`, with the same `DESK_LOOP`/`DESK_SESSION`/`DESK_ROOTS`/shim-`PATH`
  env the claude arm gets, the model from the same `DESK_MODEL_*` resolution, and the resident-rules
  fragment appended to the worktree's `AGENTS.md` idempotently (codex has no `SessionStart` hook).
  The the-desk/Opus refusal binds the claude arm only — codex refuses nothing and prints the
  resolved model.
- The first live Codex smoke run for harness-portability/07 is recorded at
  `docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md` — codex-cli 0.154.0 on OpenAI
  `gpt-5.6-terra`, run on the driver's workstation with a transcript excerpt per step.
  Both documented install arms resolved (the marketplace arm end to end, which the
  runbook still marks documented-not-demonstrated); the resident rules arrived through
  `AGENTS.md` with no tool call; all twelve packaged skills loaded their full body on
  by-name invocation; auto-trigger drew `worker-desk` unnamed; the isolation floor
  refused under `workspace-write` and created its worktree under `danger-full-access`;
  and `verify-desk` recorded command, exit code and output for a real Verify row. The
  dispatch step is **BLOCKED**: on 0.154.0 `features.multi_agent = false` no longer
  removes the subagent tools, so the degradation that step exists to observe cannot be
  provoked by the documented mechanism. No bundle file was edited to make any step
  pass; the drift is routed to issues, per the protocol.
- The forge interface's `CheckRun` now carries the forge's per-execution run `ID`
  (GitHub check-run id, GitLab pipeline-job id). An absent id maps to `""`, never
  `"0"`, so a citation of `0` can never match a run the forge never identified.
- The same reference states the full-length-SHA posting rule in one place: a shortened SHA is accepted by the shell and silently fails to register a verdict, which the flip gate then reads as no verdict at head.
- The v1.0.6 umbrella this pins to carries statusgen's scoped same-tag pin lint (#794): an
  exempt or non-umbrella artifact line in `.assay-versions` no longer makes the whole file
  unlintable, so an adopter carrying one can re-pin and lint clean again. The re-pinned
  desk-tools also accept the channel-D `desk-tools-source` pin shape (#797).
- `[dry-run]`/`[launch]` print `harness=<h>`, and a non-claude `DESK_SESSION` carries a `-codex`
  suffix, so the roster beacon shows which harness a window is on.
- `assay.harness` is now an exclusively-bound key (composability/04): the Claude
  Code, Codex, and Cursor delivery shapes are owned by three adapter
  components (`components/harness-{claude-code,codex,cursor}/component.yaml`)
  that each `provide: assay.harness` with a `flavour`; `deskmanifest lint`
  refuses a tree with more than one ACTIVE provider and its new `--activation`
  flag reports every component's computed ACTIVE/INACTIVE state.
- `cellctl check` gains a codex harness block (binary + version, authentication, `multi_agent`, the
  resident-rules fragment, skills discoverability) when `CELL_HARNESS=codex`; `n/a` on a claude
  cell.
- `cellctl desk`/`cellctl up` accept `--harness <claude|codex>` — a per-run choice of which harness
  a role window boots on, default `claude` (never touches `cell.env`); `up --harness` applies it to
  every role window it opens. `CELL_HARNESS` is the persisted `cell.env` pin, scaffolded by
  `cellctl new`.
- `cellctl desk`/`cellctl up` accept `--model <m>` — a per-run override of the `cell.env` model
  pin (`DESK_MODEL_OVERRIDE` is the equivalent env form); `up --model` applies it to every role
  window it opens. The `[launch]` line and `DRY_RUN=1` output print `model=<m> (override)` so the
  source of the value is visible in the transcript. The existing the-desk/Opus refusal applies to
  an override exactly as it does to a pin.
- `cellctl set <cell> KEY=VALUE [...]` persists a `cell.env` change in place (comments and ordering
  preserved), with a `cell.env.bak-<ts>` backup, a refusal on an unknown key unless `--force`, and
  the same the-desk/Opus refusal on `DESK_MODEL_the_desk`. `cellctl desk ... --model <m> --set` is
  sugar for "override this run and persist it".
- `changelog-check`: a notable fix from a pull request whose branch maintainers cannot commit to (a fork) can be recorded by a **proxy fragment** — a maintainer lands `changelog/pr-<N>-<slug>.md` on the base branch and re-runs the check by adding or removing a label — instead of being dropped from the notes under `changelog:skip`.
- `commsloop` gains an executor dispatch leg (`dispatch_native.go`, `roleprofile.go`):
  behind a `Native` flag (zero value = today's interim behavior, the rollback position —
  no production call site sets it), a dispatchable tier reaching `Dispatch` fires a real,
  role-fenced ACP session instead of the interim placeholder result. Each target desk
  role (`the-desk`/`intake-desk`/`worker-desk`/`pr-review-desk`/`verify-desk`) gets its
  own compiled `PermissionPolicy` + `FileAccessPolicy`; a role outside that closed set
  refuses dispatch outright — there is no permissive default profile.
- `commsloop` now routes **every** accepted inbound message through a contained prose
  router (`decide.go`) — no deterministic routing table, no fast path. A retired mechanical
  shortcut used to land report-shaped verbs (`status`/`metrics`/`help-offered`) done with no
  consult at all; every accepted message that clears the routing-boundary ACL re-check now
  reaches the SAME consult. The router picks one action from a closed set
  (`route-work-dispatch`/`route-review`/`route-verify`/`land-report`/`file-question-issue`/
  `escalate-human-issue`/`quarantine`, default `quarantine`) and never names a runner
  selection; `assign.go`'s compiled `(action, class, risk) -> Tier` table does that lookup
  deterministically. The router's reader runs under the same refuse-everything containment
  profile as the outbound prose gate: an empty filesystem root, and fs/terminal/tool
  callbacks refused and filed as containment anomalies.
- `credit-identity.sh` now returns `skip:bot` for any `<slug>[bot]` / `app/<slug>` login before consulting the roster, so an incomplete `ASSAY_TRUSTED_BOT_SLUGS` can no longer credit a house App as an external contributor.
- `deskavatar --tier family` now ships a seventh role tile, `board-writer`, so a
  roster that binds all seven fleet roles can get a proof-checked icon for it from
  the tool instead of hand-drawing one outside the 20 px legibility proof. The tile
  takes the only red hue in the palette (`#E5484D`) and a new pen glyph; it clears
  the pairwise 20 px proof against the existing six — separated from `worker` by
  silhouette and from every other role by colour (#895).
- `deskinstall --harness cursor --forge <github|gitlab> --repo <path>` places Cursor's
  install (skills + a sibling `references/` tree so `../../references/*.md` includes
  resolve, plus the generated `.cursor/rules/assay.mdc` when present) and writes the
  shared `AGENTS.md` bindings block, forge-substituted (`gh` on github, `glab` /
  `--forge gitlab` on gitlab). Idempotent; `--check` reports drift (missing / extra /
  content-differs) without writing.
- `deskmanifest lint` gained one more check: an outside `apply` step with no `ledger:` value is now a PROBLEM (component-model.md §5).
- `deskmanifest`'s manifest schema grows two `provides` attributes: `flavour`
  (also usable as an `inject` constraint, e.g. the hooks component now
  requires `assay.harness` at `flavour: claude-code`) and `evidence` (a
  repo-relative installed-shape marker that decides which adapter is ACTIVE
  ahead of the desired-state record).
- `deskpost review` / `security-review` / `comment` / `ready` now resolve the repo's forge and
  read their preconditions + land the verdict/comment/ready-flip through the typed Forge surface,
  so a GitLab-resolved repo no longer fails closed with `deskpost has no gitlab write backend`
  (the #772 follow-up). On GitLab an approve/request-changes verdict lands through `Forge.PostReview`
  — its first shipping consumer — mapping to a head-pinned approval + verdict note the read path
  sees at head; the GitHub path is unchanged.
- `deskpr create`, `deskpr update` and `deskwt add` now **refuse, fail-closed**, when the
  resolved PUSH url of `origin` is an SSH one (`ssh://…` or `git@host:path`) and the session
  presents a bot identity (`$DESK_LOOP` resolving to a role App). An SSH push authenticates
  with whatever key the machine's agent holds — a human's — so the forge recorded the HUMAN
  as the branch creator and the App's permission envelope was bypassed, however the commits
  were authored. The refusal names the config key, the url, the acting App, and the one-line
  `remote set-url --push` remedy with the equivalent https url computed for you.
- `desktoken` mints a seventh role, `cell-issues` — the write-issues App identity
  (`issues:write` + `metadata:read` only), selectable only by explicit name; no loop's
  default identity changes.
- `pr-review-desk` gains `references/re-anchor.md`: the five states a moved PR head puts a reviewed PR in — head moved under a standing verdict, a standing CHANGES_REQUESTED at the same head, a racy CONFLICTING while a gate is pending, a red check, and a non-commit fix at an unchanged head — each as one SIGNAL / PROBE / ACT / STOP row, so a re-anchor is a mechanical response instead of a fresh diagnosis each time.
- `scripts/bootstrap-windows.ps1` resolves the pinned tag + sha256 for the detected
  `windows-<arch>` from the committed `plugins/assay/paired-versions.yaml` manifest instead of
  demanding the sha256 as a mandatory, hand-transcribed parameter; `-Sha256` is now an optional
  override that must agree with the manifest or refuse. The script also writes the resolved
  install directory onto the current user's `PATH` (idempotent, segment-guarded) and additionally
  places the verified binary as `statusgen.exe`, so `statusgen --version` resolves by bare name
  in a new shell after step 1 of the Windows install.
- `scripts/windows-bootstrap-hashcheck-smoke.ps1` gained manifest-tamper and absent-platform-line
  negative-path assertions (with their own non-vacuity controls), alongside its existing
  hash-mismatch and override-disagreement checks.
- `statusgen reconcile --backfill --report` additionally writes
  `docs/streams/board-drift-<date>.md`: one row per brief where the last
  hand-edited (pre-generation) README cell disagrees with what the run
  derives, for a human to resolve by linking the PR or accepting the
  demotion.
- `statusgen reconcile --backfill` — the declared, reviewable history-only
  fallback promised by the v1.0.0 release note: when no PR carries a `Brief:`
  trailer, a merged PR whose branch name or body names the brief in
  `<stream>/<NN>` or `<stream>-<NN>` form counts as a witness, tagged so it
  reads apart from a real trailer link. A hand-asserted
  implemented/verified/done with neither renders `unknown` naming the
  hand-asserted state and commit — never a silent demotion to `todo`.
- `tools/desk/internal/deskkit`: component ACTIVATION (`activation.go`) — a component is
  ACTIVE iff every declared `inject.required` key resolves; an INACTIVE component's owning
  verb refuses with a three-state `could-not-check: assay/<component> inactive — <key>
  <reason>` before doing any work. Wired into every desk command's shared entrypoint
  (`deskkit.CheckVerbActivation`, called right after `EchoEffectiveConfig`).
- `windows-port` stream extended with four briefs (06-09) covering the **three-command Windows
  install**: a manifest-driven PowerShell bootstrap that resolves its own pinned tag + sha256 and
  writes PATH (06); a `deskinstall --harness cursor` mode that places the skills/references tree
  and the `AGENTS.md` bindings idempotently, with a `--check` drift report (07); a Go-native GitLab
  fleet-provisioning verb replacing the bash + curl + jq script so native Windows needs no
  Git-Bash/WSL (08, `gate: human` — it mints live access tokens); and the docs/scope delta that
  widens the install skill's Windows scope from acquisition-only to the whole install, collapses
  the adopter walkthrough, and corrects the stale "CI leg is staged" claim (09).
- forge-gitlab briefs 11 and 12 + design record `DR-forge-gitlab-11`: the token-custody design that
  closes forge-gitlab/08's shell-exec ban to zero — `deskroster` reads as the session role,
  `repohardenguard` reads as a dedicated read-only `auditor` identity through one enumerated
  `RepoHardeningRead(repo, kind)` op over a closed kind set (GitHub kinds in 11, GitLab kinds in 12).
  Authoring only; the human gate on 11 decides the custody shape before any code lands.

### Fixed
- **Restored the two Windows pin lines in `examples/adopter-scaffold/.assay-versions`** dropped
  by a later, unrelated rewrite of that file. windows-port/01 added
  `statusgen-windows-amd64`/`statusgen-windows-arm64` illustrative pin lines; the derived-board/06
  migration rewrote the whole file onto a new schema (umbrella line, fixture-placeholder digests)
  and did not carry the two lines forward, silently regressing the brief's Verify row 7. The two
  lines are re-added at the file's current pin tag, following the ONE TAG, ONE TREE rule and the
  same fixture-placeholder convention every other artifact line in the file already uses.
- **The herdr cockpit arm now matches herdr's real grammar.** `herdr agent start`'s
  `-- AGENT_ARG...` list is appended directly to the KIND's canonical executable (confirmed live
  against herdr 0.8.2: a bare `agent start <name> --kind claude --pane <id>` launches literally
  `claude`) — it is not a wrapping shell command, so `-- bash -lc "<cmd>"` never ran `<cmd>`. `up`
  now creates the tab (`herdr tab create --label <l>`), takes the pane id from the real
  `result.root_pane.pane_id` JSON shape, and drives it with `herdr pane run <pane_id> <cmd>` —
  verified live to actually execute the command and return its output. `down` looks the tab up by
  label via `herdr tab list` and closes it by `tab_id` (`herdr tab close <tab_id>` — real herdr
  0.8.2 has no `--label` on `close` at all, unlike the code that shipped before this fix assumed).
- **The orca cockpit arm is proven live, not merely "refuses when the app is closed."** Its
  create-a-terminal "where" flag is `--worktree <selector>` (`path:<dir>`), never `--cwd`/`--path`/
  `--directory` — confirmed live that a real orca advertises no such flag on `terminal create` at
  all — and that selector 404s (`selector_not_found`) until the path is registered once with
  `orca repo add --path <dir>` (idempotent; now called automatically before the first terminal).
  `down` now closes every terminal orca owns for the cell in one call
  (`orca terminal close --worktree path:<cell-dir> --all`) instead of only printing a by-hand
  notice. Along the way: `down_orca`'s own `local roles; roles="$(up_roles 0)" r` was a malformed
  `local` line (a stray `VAR=value r` command, not a second local variable) that made every
  `cellctl down --cockpit orca` die with `r: command not found` before this fix — undetected
  because nothing had ever driven the arm to completion.
- **The plugin manifest version now moves with every umbrella cut (#789).** Every tag through v1.0.6 shipped `plugins/assay/.claude-plugin/plugin.json` at `1.0.0` (and the marketplace entry at `0.1.0`); Claude Code keys its plugin cache on that string, so `claude plugin update` was a no-op and adopters kept the first `1.0.0` content they ever cached. The release workflow now stamps the manifest — plus the marketplace entry, `paired-versions.yaml`'s `plugin:` pairing, and the generated Codex / Cursor / SessionStart-hook artifacts — to `X.Y.Z` in the tree tag `vX.Y.Z` is cut from (`plugins/assay/scripts/stamp-plugin-version.sh`), proves it against the generators before tagging, refuses a tag whose `plugins/assay/**` changed since the previous tag while the manifest version stayed put, and leaves the default branch on the next patch version after each cut. Adopters still holding the `1.0.0` cache run `claude plugin uninstall assay@assay` then `claude plugin install assay@assay` once; from this release on `claude plugin update assay@assay` delivers new tags (see `docs/UPGRADING.txt`).
- **`cellctl down` no longer rejects itself.** The dispatcher always forwarded a phantom 3rd
  positional to `cmd_down` even on a bare `cellctl down <cell>` — an empty string quoted as one
  argument is still an argument, and `cmd_down`'s flag loop rejected it as
  `down: unexpected argument ''`. It also silently dropped any flag *value* past the first token
  (`--keep-deskd --cockpit tmux` lost `tmux`). `down` now forwards `"${@:2}"` the way `up` always
  has, so every flag combination reaches `cmd_down` intact.
- **`deskwt role-init` isolates EVERY desk role, in either spelling, from anywhere — and `deskboot` names that fix verbatim.** A desk window booted with its cwd in the shared checkout was refused by `deskboot` (correctly) with a remediation it could not run: the message spelled the fix as `deskwt role-init --role <loop-name>`, and `role-init` accepted only token roles, so the operator was left to hand-roll a worktree — or boot shared-homed, where the write guard then refuses every mutation the session's subagents attempt. `role-init` now takes the role positionally or as `--role`, in either vocabulary (token role `desk` / `worker` / `reviewer` / `verifier` / `issue-loop` / `intake-loop`, or the loop name `the-desk` / `worker-desk` / `pr-review-desk` / `verify-desk` / `intake-desk`, retired spellings included), adds `intake-loop` (the one `desktoken` role that had no worktree mapping), and takes `--repo-root <checkout>` so it runs from any cwd against the checkout named. It cuts the worktree from a FRESHLY FETCHED `origin/main` (a fetch that cannot run is could-not-check, exit 6; `--no-fetch` is the explicit opt-out), locks it, stamps the role's identity worktree-scoped, and prints the worktree's absolute path as its last stdout line — the launcher contract `cd "$(deskwt role-init <role> --repo-root <checkout>)"`. The shared checkout's index and `user.*` config are never written. `deskboot`'s shared-checkout refusal now prints `deskwt role-init <role> --repo-root <abs-path>` with the loop name it was given, the `cd "$(…)"` form, and `cellctl desk <cell> <role>` when `cellctl` is on PATH; what it refuses is unchanged.
- N/A — no defect fixed by this brief; see "Not migrated" below for a gap found and deliberately fenced rather than worked around.
- On GitLab, a `deskpost review --verdict approve|request-changes` verdict is now VISIBLE to
  `ReviewsAtHead` and to `deskflip`'s `reviewer-approved` gate. The write side already landed
  the verdict's reasoning as an MR NOTE carrying a `Verdict: approve|request-changes` line
  (an approve also POSTs a GitLab approval; a request-changes has no native GitLab review
  object at all), but the read side classified every non-system note as `COMMENTED`, so the
  gate reported "no APPROVED/CHANGES_REQUESTED correctness verdict" over a verdict that had
  really been rendered — the write and the read disagreed on the object (#798). `ReviewsAtHead`
  now reduces a verdict note to the review STATE a GitHub review of the same verdict reports
  (`APPROVED` / `CHANGES_REQUESTED`) via the new canonical `deskkit.CorrectnessNoteState`
  reader, so both sides agree on the same channel — the note — on GitLab CE and EE alike (where
  the approval object may not be head-pinned) and for request-changes, which has no approval
  object to read. The reducer mirrors the security-marker fence asymmetry (a fenced/quoted
  `Verdict: approve` is not a grant; a fenced `Verdict: request-changes` still blocks) and is
  identity-free — consumers still filter to the reviewer App login before acting.
- Pairs with #983, which closes the remaining fast-forward mutation-coverage gap; the two ship
  together.
- The shared `deskkit` secret scan no longer refuses a path that has a `+` immediately in
  front of it. `+` is in the base64 alphabet, so a regex quantifier written against a path
  (`grep -cE -e '^FRESH +plugins/assay/…/claude-code\.md'`, the Verify-row idiom) or
  a unified-diff add marker was read as the path's first character, and the path rule refuses
  any run containing `+` outright. Because `deskevidence` scans the whole merged brief before
  appending its Evidence row, a brief carrying such a Verify row could not receive an Evidence
  append through the sanctioned tool at all. The exemption is earned by the remainder being
  path-like — exactly one `+` is stripped, and a `+` on opaque token material still refuses.
- Verify rows across `harness-portability`, `forge-gitlab`, `statusgen` and `desk-tools` now
  resolve as literally written from the repo root. The rows shelled into per-tool Go modules
  root-relatively (`go run ./tools/freshness`, `go test ./tools/desk/internal/deskkit/`,
  `go test ./statusgen/`), which resolved only in a tree carrying a root `go.work` — the
  published tree carries none, so every such row died on `go: go.mod file not found` and had to
  be hand-substituted at verification time. Each row is now module-scoped
  (`cd <module> && GOWORK=off go …`, or a `go build -C <module>` whose binary runs from the root
  so path arguments keep their repo-root meaning), matching the form CI already uses. The
  explicit `GOWORK=off` makes a row behave identically in a tree that has a `go.work` and one
  that does not, so this class of breakage cannot pass in one tree and fail in the other again.
- `aggregate.py credits` now credits a fragment to the pull request that
  actually landed it: the adding commit's own `(#N)` / `Merge pull request #N`
  subject is read FIRST, and a merge is only accepted when its second parent
  contains the adding commit. Previously the resolver took the oldest merge on
  the ancestry path to `HEAD`, so a squash-landed fragment — and a fragment that
  never landed on a pull request at all — was credited to whatever unrelated
  pull request merged above it, collapsing unrelated fragments onto the same few
  recent numbers.
- `changelog-check` now reads a proxy fragment from the live tip of the PR's base branch instead of the base commit GitHub recorded when the PR was opened. That recorded sha never advances, so a `changelog/pr-<N>-<slug>.md` landed on the base branch *after* a fork PR opened — the only case the proxy path exists for — was invisible and the PR stayed red. (#923)
- `cmd/deskmerge`'s mutation-gate spec (`cmd/deskmerge/mutations.json`) is repointed at the
  current `gitcore`-based parent-check code, so `muhar` can plant its positive control and
  mutations again instead of exiting 2 before any verdict prints. (#979)
- `cmd/deskmerge`'s test suite now catches the "fast-forward instead of forcing a merge
  commit (`--no-ff` dropped)" mutation directly at the trial-merge step (`assess.go`),
  closing the shard 1/3 `NOT_CAUGHT` gap `#982` surfaced once the mutation-gate harness
  itself was fixed. (#979)
- `deskdispatch`'s `worktree-create` failure hint is now selected by kit. The brief-lane hint
  ("the brief's `feat/<id>` branch already exists — look for a merged/open PR") is meaningless on the
  review lane, which has no brief and no feat branch; the review lane now gets a hint that points at
  the reviewer-worktree lifecycle (reclaim the earlier reviewer worktree with `deskwt remove` before
  re-dispatching) instead of a phantom PR (#851).
- `deskevidence --brief-path` now secret-scans only the Evidence bytes it is adding, not the whole merged brief — a secret-shaped run already on the branch (a frontmatter `id:`, a Verify row's literal command, a fingerprint in prose) could permanently block every future Evidence append to that file.
- `deskfile`'s label-missing NOTICE now names a repo-appropriate remedy instead of a GitHub-only one, and its `--help` no longer claims a GitLab refusal that doesn't happen (#887 items 2, 3).
- `deskmerge`'s regenerable-conflict `add` stays on the git binary (fenced, not deferred): verified empirically that go-git's `Worktree.Add` does not clear a path's merge-conflict index stages (1/2/3) — the on-disk index still lists them as unmerged afterward, and a `gitcore.Commit` built from that index writes a tree with duplicate entries for the path (`git fsck`: `duplicateEntries`). Staging the resolved path with the git binary first, then committing via `gitcore`, produces a clean result; that's the sequence deskmerge now runs.
- `deskmerge`'s scratch-worktree family (`worktree` add/remove/prune) and its transport verbs (`fetch`/`push`) are untouched — explicitly out of scope for this brief (worktrees are the named follow-on stream's gap; fetch/push are briefs 05/06).
- `deskpost comment` can now annotate a **verify-gate sign-off card**. Those cards
  are filed by the repo's own `verify-gate-open` workflow, so their author is
  `github-actions[bot]` — an identity the trust gate refuses — and the refusal meant
  no desk could mark a card an inert duplicate or warn that closing it will not flip
  the brief's row, leaving the human closing it with no signal. The carve-out is the
  narrowest read that fixes that: the `comment` verb, on an **issue**, authored by the
  forge's Actions identity, carrying the **`verify-gate`** label. `review`,
  `security-review` and `ready` stay refused on such issues, an Actions-authored issue
  *without* the label stays refused for `comment` too, and the label admits nothing on
  an issue anyone else authored. Every other comment-path protection — the body size
  cap and secret scan, the repo gate, the write budget, the audit line — is unchanged.
- `deskwt remove` no longer refuses a detached-HEAD worktree whose commit is provably present on
  the remote (reachable from a remote-tracking ref). A review kit checks the PR/MR head out as a
  detached HEAD, so a reviewer worktree is by construction detached and never an ancestor of
  `origin/main`; the old blanket "detached ⇒ refuse" left it unreclaimable, and the next dispatch on
  the same lane key then failed worktree-create ("target already exists") — the review lane wedged
  permanently after one review (#851). "No upstream" and "not on the remote" are now treated as
  different questions: a detached HEAD whose commit is on NO remote is still refused (the
  never-remove-unpushed-work invariant is unchanged), while one whose commit is proven pushed is
  reclaimable, which unwedges the lane. `deskwt prune` continues to reclaim a reviewer worktree once
  its PR merges via its existing origin/main ancestor gate, and to leave an open PR's (unmerged)
  reviewer worktree in place.
- `deskwt`'s ambiguous-base ref-candidate enumeration (`ambiguousbase.go`) stays on the git binary: `gitcore`'s ref iteration does not surface a symbolic ref such as `refs/remotes/<name>/HEAD`, and that guard must see every real candidate to avoid under-reporting a genuine ambiguity.
- `git config --get`/`--list` reads stay on the git binary everywhere except `remote.<name>.url`: go-git's `Repository.Config()` does not merge a worktree-scoped `config.worktree` file, and this house's own tooling sets `user.name`/`user.email` at exactly that scope for per-worktree bot identity.
- `harness-portability/07`'s Verify row 6 (`docs/streams/harness-portability/brief-07-adoption-live-smoke.md`)
  read the RELEASE-NOTES version to match from `plugins/assay/.claude-plugin/plugin.json`'s
  `.version` (the umbrella/plugin-manifest version), which drifted out of step with
  `RELEASE-NOTES.md`'s bundle-content version headings and made the row fail as written on
  merged main with nothing wrong in the release notes themselves. Row 6 now reads
  `plugins/assay/SOURCES.yaml`'s `bundle-version` — the same authoritative source row 5
  already uses — so both rows key off one version scheme.
- `human-runsheet` is now accounted for in the Codex and Cursor packaging
  rosters and carries a degradation cell in all three capability-matrix
  reference files, clearing the coverage gap that failed `harnessgen codex`
  (exit 2) inside `release.yml`'s plugin-manifest-stamp step and blocked the
  v1.0.7 cut (#970).
- `internal/gitcore.Open` now tolerates `extensions.worktreeConfig` (a go-git v5.19.2 bug lowercases the extension name before checking its own mixed-case allowlist, so it wrongly refused to open almost every worktree in this house — every one this tooling provisions sets that extension) and correctly routes a linked worktree's reads through its shared common `.git` for remotes/branches/objects, not just the per-worktree admin directory.
- `statusgen --consumers --brief <id>` now resolves a `schema: brief-v2` brief
  under either the short `<stream>/<NN>` form or its own fully-qualified
  `<cell>:<repo>:<stream>:<NN>` form, instead of only an exact string match
  against the file's `brief:` field. Reuses `normalizeBriefKey` (the helper
  `verifyMarker`/`loadExistingMarkers`/`closeVerify` already use post flag-day,
  #840) so a verify-gate row keyed by the short form can corroborate a
  brief-v2 brief's `consumers:` claims instead of failing with
  "no brief-v1 file for ...".
- `statusgen --lint` no longer reddens a brief for citing its own changelog
  fragment after a release consumed it. A backticked `changelog/<slug>.md` path
  resolves when the repo runs the fragment convention, has cut a release, and the
  fragment is present in git history — so the release that correctly clears
  `changelog/` no longer turns every brief that listed its fragment PROBLEM-red.
  A mistyped or never-committed fragment path is still reported, and a tree with
  no readable git history reports the problem as could-not-check rather than
  silently exempting the path.
- `statusgen` verify-gate cards now treat a brief-v1 `<stream>/<NN>` key and a
  brief-v2 `<cell>:<repo>:<stream>:<NN>` key as ONE identity. The idempotency
  marker renders and matches in the canonical `<stream>/<NN>` form, and
  `--close-verify` accepts either form — so a v1→v2 migration no longer re-files a
  duplicate sign-off card for every already-carded brief.
- `statusgen`'s monolithic intake register parser (`parseIntakeLegacy`) now matches the `Disposition:` key case-insensitively and tolerates surrounding whitespace, preventing lowercase or mixed-case disposition lines (e.g. `disposition: accepted`) from being silently dropped and falling back to the untriaged `new` state (#915). — thanks @teddyvj
- `verify-gate-close.yml` now accepts a `<!-- verify-gate: ... -->` marker written in either
  brief-id grammar: the legacy `<stream>/<NN>[a]` form, or the `<org>:<alias>:<stream>:<NN>[a]`
  brief-v2 key form that `statusgen --verify-issues` now emits. Previously only the legacy form
  passed the workflow's grammar check, so a verify-gate issue carrying a brief-v2-keyed marker was
  rejected outright and its brief was never advanced to `done` on close. The extracted marker is
  normalised to the legacy form before the existing grammar check runs, so every later use in the
  step still sees exactly one shape (#804).

### Changed
- **Dead-claim decay states "not applicable on this forge" distinctly from "failed this run".** On a GitLab remote the decay — which reads PR state through the GitHub-only `gh` — says so with its own message and leaves the claim set unchanged, rather than reusing the transient "authenticate and regenerate" wording that reads as a passing check that just needs a retry.
- **Every `cellctl desk` window now exports `DESK_ROOTS` when `cell.env` carries `CELL_ROOTS`**, on
  k8s cells too, and says on stderr when it boots without one — a cell re-boot can no longer leave
  the desk verbs silently on their compiled placeholder topology. Role worktrees are locked at boot,
  the shared-fetch lock lives in the common git dir (so a `CELL_REPO` that is itself a linked
  worktree no longer waits 60s), and a `deskwt role-init` that supports the role is preferred for
  creating the tree so cellctl and the desk skills agree on its name.
- **Public-repo write gate: an allowed-repos entry tagged `:public` now authorizes outward writes.** A public (or `internal`) repository listed in the allowed-repos configuration as `owner/name:public` is authorized for every desk write verb — create, review, ready, reply, evidence and release — replacing the former per-item `+1` reaction check. The authorization is repository-scoped and decided once out-of-band. The gate reads the live forge visibility AND the configured `:public` claim and refuses (exit 5) unless they agree: a repo the forge reports public that is unlisted, matched only by an `owner/*` pattern, or tagged `:private` is refused; an unreadable or unrecognised live read fails closed (exit 6). This makes the FIRST pull request on a listed public repository openable — it has no issue/PR number, which the old check could never satisfy — while merge remains a human act.
- **The model-path auto-flip corroborates the reviewer's verdict per forge.** The accepted reviewer identity is derived from the forge-qualified trust-roster entry — the GitHub `<slug>[bot]` / `app/<slug>` renderings, or the bare GitLab service-account username — instead of appending a literal `[bot]` regardless of forge (which meant the flip could never match on GitLab). On GitHub a review whose `commit_id` is the merged head flips the row; on GitLab CE, where approvals persist across a push, it takes an approval **plus** a note by that identity pinning the head SHA. An approval that cannot be tied to the head leaves the row `verified` and is reported, never a silent flip.
- **The v0.3.0 note in `plugins/assay/RELEASE-NOTES.md` is now true.** It described a Codex install/fragment/`multi_agent` runbook and a "reproduced" degradation table in `docs/adopting-assay.md`; the doc contained neither. The runbook above supplies the first, and the note is corrected on the second: the per-skill table is **pointed at**, not copied, so `references/codex.md` stays its one home.
- **Two finished streams archived.** `mistake-proofing` (6/6 briefs done) and `spec-routing`
  (1/1) are closed — `status: done` and moved from `docs/streams/` to `docs/archive/`, the
  pairing statusgen enforces — so the active board and `statusgen --lint` stop carrying their
  archive-candidate notices. Cross-links into the moved files resolve through the existing
  `docs/archive/` fallback; the one absolute path reference (`tools/skillslint/README.md`) was
  retargeted.
- **`cellctl down` and `cellctl check` follow the cockpit.** `down` takes `--cockpit`, always tears
  the tmux session down, and closes what a non-tmux cockpit opened where that cockpit offers a verb
  for it — **naming what to close by hand where it does not**, never leaving it unsaid (orca's
  scheduled automations outlive `down` on purpose and are named rather than deleted). `check` gains
  a cockpit precondition row carrying the same resolution and reason, plus an orca-reachability row
  whenever orca is installed, so which surface a boot will use is answerable before booting. These
  cockpit CLIs move fast, so every verb and flag `cellctl` cannot see is probed from `--help` at run
  time: a herdr build with no `tab create` still gets labelled windows with a notice, and an orca
  build whose terminal-create verb or command flag is absent gets the exact per-role commands
  printed to run by hand rather than a guessed spelling. `tools/cellctl/tests/cockpit.test.sh`
  covers the selection matrix offline with stub cockpit binaries on a private PATH.
- **`deskflip` reads an absent required leak-gate verdict as could-not-check**: an otherwise-green
  rollup that is missing a branch-protection-required context (the `leak-sweep` status among them)
  no longer flips ready — absence of the verdict is never "no objection".
- **`statusgen init` scaffolds the CI half that matches the target's forge, and refuses to guess (forge-neutral/08).** A GitHub remote gets the GitHub workflow, a GitLab remote gets a `.gitlab-ci.yml` running the same two-half single-writer pipeline (lint on a change, regenerate-and-commit the board on the default branch). A readable remote whose host names neither forge now writes **no** CI half at all — with a message naming the host and pointing at `--forge` — rather than defaulting to a GitHub workflow the adopter cannot run; only a tree with no origin remote yet keeps the historical GitHub fallback. The closing next-steps text names whichever file was actually written.
- **`the-desk` scaffolds pinned to the top-tier model (`fable`) instead of `opus`, and `cellctl`
  refuses to run it on Opus.** All three `cellctl new` templates (k8s/github, k8s/gitlab, house)
  now write `DESK_MODEL_the_desk=fable` — the coordinator role is the one window that spends its
  tier on judgment, synthesis and arbitration across streams, and `opus` is no longer the top tier.
  `cellctl desk <cell> the-desk` (its `DRY_RUN=1` plan included) refuses when the resolved model —
  from `DESK_MODEL_the_desk` or, unset, `DESK_MODEL_DEFAULT` — is the `opus` alias or a
  `claude-opus-*` id, naming the resolved value and the variable to change; `cellctl check` shows
  the resolved the-desk model and flags an Opus pin the same way, as a MISS. Every other role's
  pin, `opus` included, is untouched — an operator can still pin `the-desk` itself to any
  non-Opus id.
- **the-desk: never-verified briefs are a work list.** The coordinator's board-sweep rule now
  states that the "Age at the human gate" table and every `implemented` brief with no Evidence
  entry are read as work, not background: a brief with dependents or older than 7 days is routed
  by name to the verify desk in the same sweep and recorded in the hand-off note. The table is
  render-only by construction, so the coordinator is the only reader that can turn it into an act.
- Added migration `migrations/0002-v1.0.6-to-v1.0.7-verify-gate-close-brief-v2-key.md` so an
  adopter running `upgrade-assay` across this span is told, as their release note, that their own
  copy of `verify-gate-close.yml` needs the same patch — the migration runner has no file-patch
  operation, so it records the owed step in `docs/UPGRADING.txt` and prints the patch instructions
  rather than applying them.
- An invalid/timed-out/budget-exhausted/valve-disabled consult resolves to the default
  action (`quarantine`), so `commsloop` is fail-closed whether or not a decider is even
  configured — mirroring the outbound gate's own posture.
- Every `component.yaml` in the tree carries a real `inverse:` (inside boundary) or `ledger:` + `compensation:` (outside boundary) for each apply step, replacing the brief-00 `TODO composability/02` placeholders. Every outside compensation in this tree defaults to `list-for-human` — none is marked `unattended: true`.
- Nothing widens: the App's grant is fixed server-side and untouched by this change, and the
  loop→role table still resolves no window to the write App implicitly.
- The `askassay` silent-cap register retires `deskroster`'s `--limit 50` row — the read is now
  bounded by the forge backend's declared page cap rather than a deskroster-local literal.
- The adopter-scaffold example gains a v1.0.6 composition manifest with real digests, and its
  notes now name v1.0.6 as the umbrella an upgrade moves to. The v1.0.0 through v1.0.5
  manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.6** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.6
  binaries and verifies them byte-for-byte.
- The remaining `repohardenguard` `gh api` site is recorded as **open work**, not a standing
  keep-as-CLI exception: its `forgeban` permit row is permitted only until the guard-read-custody
  brief lands, and the ban closes to zero with that brief.
- The roster loader now splits TRUST-surface validation (unchanged, fail-closed) from
  EXTENSION-key validation: a malformed `ASSAY_REPO_ALIASES`, `ASSAY_REPO_FORGES`,
  `ASSAY_RISK_CALLOUT`, `ASSAY_WRITEGUARD_CALLOUT`, `ASSAY_RELEASE_REPO`, or
  `ASSAY_SCAN_REPOS` value no longer collapses the WHOLE configuration to unconfigured — it
  is recorded per-key on the new `Config.Ext` map (`ExtKeyResult`), the affected field falls
  back to its own shipped default, and only a component that actually requires that
  extension key goes INACTIVE. The five trust surfaces (`ASSAY_BLESS_LOGIN`,
  `ASSAY_TRUSTED_LOGINS`, `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`,
  `ASSAY_HUMAN_LOGIN_MAP`) are unchanged: unset or malformed still refuses every trust-gated
  verb.
- The standing per-repo authorization sentinel file (the human-maintained public-repo opt-out list) is retired: its reader is gone, so the file no longer has any effect. Operators move each listed repository into the allowed-repos configuration as `owner/name:public`.
- `DR-forge-gitlab-11` is **approved**: the driver's ruling on the brief's decision-gate issue
  (#857, closed `human-decided`) is recorded in the record's body — option 1, the dedicated
  read-only `auditor` identity for `repohardenguard`, with `deskroster`'s display reads on the
  session's own role token and the admin-gated fields left as could-not-check rather than bought
  back with a write grant.
- `assign.go`'s `KnownActions` is now bound to the router's own declared action set by a
  diff test (`TestRouterActionsMatchAssignKnownActions`), replacing the earlier mirrored
  copy. The brief's literal spelling of the dispatch action (`route-work-ready`) collided
  with `deskkit.NewQuestion`'s construction-time reserved-verb deny-list (the `ready` token
  is reserved for the PR-ready-flip guard) — a real, boot-time refusal, not a hypothetical
  one — so it is spelled `route-work-dispatch` instead; same semantics, no collision,
  recorded at its definition.
- `changelog-check` now passes `HEAD_REF` and `PR_NUMBER` to `check.sh`: a red run names the exact `changelog/<branch>.md` to create instead of the literal `<slug>`, and the proxy-fragment path (`changelog/pr-<N>-<slug>.md` on the base branch greens PR N) is live in production.
- `commsloop sweep --cell <slug> [--since <dur>]`: the daily out-of-band lane-violation sweep. Re-derives lane legality and peer-auth validity for every quarantined message (`held/*.json`) and every landed message recorded in `journal.log` against the CURRENT compiled lane ACL and trust store, and reconciles spawned sessions against their routing decision, reporting a three-state checked-clean / checked-failed / could-not-check verdict with distinct exit codes. Findings are filed as issues, never fixed in place.
- `deskflip`'s `reviewer-approved` condition gains ONE narrowed exemption to the
  standing-`CHANGES_REQUESTED` block: a check-only CR. A re-approve at an unchanged
  head clears the block only when the CR declares `Blocked-On-Check: <check>` as its
  sole finding, a later approve from the same reviewer at the same head cites
  `Cleared-Check-Run: <id>`, and that run is in the rollup at that head, carries the
  check the CR named, is a completed success, and completed after the CR. Anything
  short of all five refuses exactly as before.
- `deskinstall`'s acquire→verify→place mode now names the `.assay/ledger.jsonl` path on a successful run (its own effects are inside the boundary, so it writes no line there, but the path is where a later outside-effect component would, and it is what `deskdisable` reads).
- `deskroster`'s two display reads (`ghViewPR`, `ghListOpenPRs`) now route through the
  enumerated `Forge` seam (`GetPullRequest`, `ListOpenChanges`) instead of shelling `gh`,
  closing two of the three residual forge-CLI call sites the shell-exec ban still permitted.
  The forge-CLI ratchet ceiling drops 9 → 7.
- `docs/adopting-assay-gitlab.md` §2 documents the `ASSAY_SCAN_REPOS` post-fleet-boot step
  (required, distinct from the `ASSAY_ALLOWED_REPOS` write boundary, verified via the
  `issueboard` stderr echo) so a green write-lane boot no longer looks complete while the
  GitLab issue lane is still could-not-check.
- `docs/adopting-assay-gitlab.md`: the Ultimate section moves from a human-only checklist to the scripted `--tier ultimate` path with verification commands (the reviewer-role negative push test and the required-status-check check).
- `docs/adopting-assay.md` channel-D section documents how to prove the running binary on a
  from-source Windows lane (no `.assay-versions` pin line exists there) and warns against
  reading a stale-pin `deskboard` as an all-clear board.
- `forge-gitlab/11` now carries the ruling's one addition as a DELIVERABLE: every adopter page that
  enumerates the `desktoken` roles or an App's permission set gains the `auditor` row and its
  minimal grant, backed by three new Verify rows — a `validRoles`-against-the-docs drift guard, a
  positive check that both adopter pages can be followed to provision the identity, and a negative
  control that no page documents a write permission or write-capable scope for it.
- `internal/gitcore` gained its first WRITE helpers: `Commit` (explicit `Parents` — a merge commit's shape is now a construction property, not something a separate `rev-list --parents` read has to verify after the fact), `CommitParents`, and `DeleteLocalRef`.
- `internal/gitcore` gained read helpers this migration needed: `Toplevel`/`CommonDir` (worktree/common-dir discovery), `AbbrevRefHEAD`/`SymbolicRefShortHEAD`/`SymbolicRefTarget`, `UpstreamRef`/`AheadCount`, `RemoteURL`, `CommitVerifyQuiet`, `HasStagedChanges`, `DirtyTrackedPorcelain`, `Diff`/`DiffSymmetric` (full unified diff, rename-aware), `TreeishID`, `LocalBranchNames`, and `RefsContaining`.
- `internal/gitcore` gained the per-commit field readers this migration needed: `Repo.CommitSubject` (`log -1 --format=%s`), `Repo.ParentHashes` (`log -1 --format=%P`), and `Repo.DiffNameStatus` (`diff --name-status`, rename-aware, returning the new `ChangeStatus` type).
- `registerid.go`'s `remoteHeadLiveness` (`git ls-remote --heads origin <name>`) stays on the git binary: it is a network transport call to verify a sibling ref's liveness against origin, not a local plumbing read, and this brief's own scope names only the plumbing reads.
- `tools/cellctl/tests/house-cell.test.sh` — a plain-bash, no-network test of the house kind against
  a fixture repo (scaffold, check, boot with a stubbed `claude`, lock, exported env, untouched
  `.git/config`, legacy `cell.env` still loads as k8s).
- `tools/cellctl/tests/model-pin.test.sh` — a plain-bash, no-network test covering the scaffolded
  default on all three kinds/forges, the dry-run and `check` refusal on `opus` and a full
  `claude-opus-*` id (including the `DESK_MODEL_DEFAULT` fallback path), acceptance of `fable` and
  a full `claude-fable-*` id, and that a non-the-desk role pinned to `opus` is left alone.
- desktools-go-git/03: migrated the read/plumbing git seams of `writeguard`, `desksourceguard`, `deskboard`, `deskscanbody`, `deskwt`, `deskgit`, `deskpr`, and `deskreply` (plus `deskkit`'s preflight probe reads) off the `git` binary onto in-process `gitcore` (go-git) — `rev-parse`, `symbolic-ref`, `for-each-ref`, `merge-base`/`is-ancestor`, `rev-list --count`, `diff` (including rename detection), and `remote get-url`/`ls-remote --get-url`. The tracked git-exec counter drops from 149 to 108 sites, below brief-01's recorded baseline of 117.
- desktools-go-git/04: migrated `deskpushguard`'s security-detection reads (foreign-commit/merge-masquerade laundering detection in `foreigncommit.go`, register-id collision scanning in `registerid.go`, and `main.go`'s local `remote get-url` fallback) off the `git` binary onto in-process `gitcore` (go-git) — `log`, `show`, `cat-file`, `ls-tree`, `branch -r`/`branch -r --contains`, `merge-base`/`is-ancestor`, `rev-list --count`, `diff --name-status`, and `rev-parse`. Verdicts are unchanged; a mandatory mutation test (`TestForeignCommitFlagged`) proves the migrated detector still flags an injected foreign commit RED. The tracked git-exec counter drops from 108 to 89 sites. `registerid.go`'s single remaining network transport probe (`git ls-remote` against origin, in `remoteHeadLiveness`) is deliberately left on the git binary — it migrates with brief 05/06's transport verbs, under that stream's human-gated security review.
- desktools-go-git/07: fenced `deskmerge`'s trial merge (`merge --no-ff --no-commit`, its `--diff-filter=U` conflict-path enumeration, `merge --abort`, and the regenerable-conflict `add`) through `internal/gitexec` as the sole sanctioned git-binary caller for the tool, under a narrow (tool, verb) allowlist entry. Migrated every OTHER `deskmerge` git verb to in-process `gitcore`: `rev-parse`, `merge-base`, the `--left-right --count`/`--parents` `rev-list` reads, the non-conflict `diff` reads (CI-contract drift, semantic-probe scoping), `remote get-url`, `commit`, and `update-ref -d`. The tracked git-exec counter drops from 108 to 96 sites.

## v1.0.6 — 2026-09-11

### Added
- **Scoped brief `forge-gitlab/09` — the GitLab reviewer write path.** Planning-only: the brief
  wires the review desk's verdict-and-escalation path (`deskpost review`/`security-review`/`comment`
  then `ready`, plus `deskfile`/`desktoken` reviewer auth) onto the typed Forge surface for a
  GitLab-resolved repo, using the provisioned role PAT rather than a GitHub App mint, with parity
  proven against the GitHub backend. It is the head of the field-check critical path (#795 §1 and §2).
- **`deskdispatch` applies the review-lane queue label `authorization-needed` when a reviewer is
  dispatched onto a change, forge-neutrally (#795 §4).** A new non-fatal `queue-label` step runs
  on a `--kit review` dispatch with `--pr` known and applies `authorization-needed` through the
  resolved forge's idempotent label ensure+apply, under the reviewer role's own credential (a
  GitHub App token or a GitLab PAT) — so a GitLab merge request now carries the same review-queue
  signal a GitHub pull request does, instead of an empty label set. A label the forge will not
  accept is a loud warning and the dispatch still stands (the label is a legibility aid, not a
  correctness gate), mirroring `deskflip`'s `approval-needed` swap. Both queue labels are now
  covered by GitLab forge golden tests for idempotent create-and-apply.
- **`desktoken` / `deskfile` reviewer-role (and every role) auth now works on a GitLab-resolved
  repo (#798 §2).** `desktoken <role> --repo <gitlab-slug>` with no explicit `--forge` now
  RESOLVES the forge from the repo and, on a definite GitLab resolution, takes the GitLab PAT
  custody path — instead of falling through to the GitHub App mint and dying with `no App ID for
  App "<role>-app"` (exit 6), the credential a PAT-backed GitLab bot never provisions (#772). So
  `deskfile check` on a GitLab repo reaches its dedupe search rather than a bare App-ID exit 6. A
  GitHub or could-not-check resolution still falls through to the App mint unchanged, and an
  absent custody PAT is a refusal — never an ambient-identity fallback.
- **`worker-desk` cockpit-aware dispatch gains a fourth (Orca) arm.** When `orca` is on PATH, the per-item worktree is cut with `orca worktree create`, and — only when a fanout coordination run already exists — `orca orchestration worker-start` lets the coordinator learn a worker's `worker_done` outcome without polling. That orchestration link is coordinator-notification only and never an escalation channel; plain `git worktree add` remains the default and fully-supported fallback. Purely additive to the existing Supacode / Herdr / plain arms.
- Recorded, with measurements, that **v1.0.5 supersedes v1.0.4 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 through v1.0.4 re-pins recorded. An
  adopter already on v1.0.0 through v1.0.4 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.5 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- The v1.0.5 umbrella this pins to carries statusgen's header-keyed base-cell resolution for
  the `--corroborate` pre-existing-stamp exemption (#785): the base cell is located by column
  HEADER NAME rather than by the branch's positional index, so a sign-off whose cell text is
  unchanged still reads `PRE-EXISTING` when the board table has been RE-SHAPED under it,
  instead of being re-gated. Every fail-closed guard is preserved, including the branch-column-
  absent-from-base case, which now carries its own committed regression test (#788).

### Fixed
- **`create-fleet-gitlab.sh` no longer exposes the group-owner token on the process table, and a
  GitLab API transport failure is now recorded in the run summary instead of silently aborting the
  run (#786).** The shared `gl_api` helper — which every settings step calls — had two defects, both
  pre-existing and fleet-wide (present since the helper was introduced, not a regression of the label
  change):
- **`deskboard`'s drift banner now recognises the `desk-tools-source <40-hex-commit> channel-D`
  pin shape (#795 §3).** A channel-D adopter writes the source commit in field 2 with a literal
  `channel-D` marker in field 3, but the reader only accepted the commit in field 3
  (`desk-tools-source <tag> <40-hex-commit>`). The commit went unrecognised, so a correctly
  pinned install reported `STALE-UNKNOWN … no readable desk-tools pin` and `reviewloop`'s idle
  gate sat at could-not-check. The reader now takes the 40-hex commit from whichever column
  holds it (field 3 preferred, else field 2); a line with a commit in neither column still falls
  through to the in-tree ref / could-not-check, so real drift detection is unchanged.
- It captured only curl's HTTP status, so a transport failure (DNS / TLS / connection refused, where
  curl exits non-zero) made the command substitution non-zero and, under `set -euo pipefail`,
  **hard-aborted the whole run before the failure ledger or the summary was written** — no
  diagnostic, no recorded step. It now uses `|| echo "000"` (the same pattern the avatar step uses),
  so the transport failure surfaces as the `000` status the caller's `record_failure` branch already
  handles: the failure is written to the ledger, the remaining settings steps still run, and the
  run reaches its summary and exits non-zero — recorded, not fatal.
- It passed the owner PAT to `curl` as a `PRIVATE-TOKEN:` header on the command line, where any
  local user could read it off the process table. It now mints the token into a `0600` `curl -K`
  config file under `umask 077` and passes `curl -K` — never argv — the same credential custody the
  avatar step already uses.
- Refreshed the four stale `deskclose` mutation specs (`tools/desk/cmd/deskclose/mutations.json`)
  so the `muhar` mutation harness is back to 11/11 mutations caught after the forge-neutral/13
  re-seat drifted them out of sync.

### Changed
- **The GitLab reviewer verdict-WRITE control is now proven end-to-end (#798 §1).** The GitLab
  backend's `PostReview` mapping — approve → verdict note + `/approve`; request-changes →
  `/unapprove` + a head-SHA verdict note — is covered by a round-trip test: a verdict written
  through the Forge is read back by `ReviewsAtHead` at head (approve and request-changes both
  visible to the read path), and a permission/tier 403 on the write surfaces could-not-check
  rather than a clean or laundered verdict.
- **`deskpr`, `deskfile` and `deskclose` now reach the forge through the resolver under a
  minted App identity, so they create, file and close on GitLab CE with the same trailer and
  gate behaviour as GitHub.** The three verbs were the last of the fleet's outward-write
  commands still shelling `gh` under whatever ambient CLI credential happened to be active. They
  are re-seated onto `ForgeFor`, which mints the session-role App token and refuses rather than
  falling through to an ambient identity. To make the migration possible the frozen forge seam
  gained four enumerated operations — a branch→change lookup, a change body/title edit, a
  repo-scoped issue text-search, and a read-only list-labels — each on both backends with a
  golden contract case. `deskpr`'s `--as-app=false` ambient fallback is retired (a run with no
  minted token now REFUSES, it does not fall back), and `deskfile`'s interim GitLab
  named-refusal (`#691`) is superseded now the backend serves GitLab. The forge-CLI permit
  ratchet drops by three. Authorized by the `#781` ruling (token-custody decision).
- **`statusgen --lint`'s same-tag pin check now honours a per-line exemption marker**, so
  `.assay-versions` stays lintable for adopters who legitimately pin one artifact on a different
  tag. A trailing `# same-tag: exempt — <reason>` comment removes that one line from the
  one-tag-one-tree grouping while keeping it a fully valid, lint-visible pin. It clears both real
  cases — a guard binary frozen on an earlier tag by a maintainer ruling, and a
  separate-repository artifact on its own release cadence the umbrella never ships. Every
  non-exempt artifact must still share one tag: a genuine undeclared mixed-tag state still
  PROBLEMs, and the exempt line's off-tag never leaks into that message.
- The adopter-scaffold example gains a v1.0.5 composition manifest with real digests, and its
  notes now name v1.0.5 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1, v1.0.2,
  v1.0.3 and v1.0.4 manifests stay: a tree pinned at any of them still has to resolve, and the
  brief-v1 → brief-v2 migration's span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.5** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.5
  binaries and verifies them byte-for-byte.

## v1.0.5 — 2026-09-10

### Added
- **`create-fleet-gitlab.sh` now provisions the desk labels a GitLab adopter's project needs,
  closing the last gap before `deskflip` can drive an MR to ready.** The fleet script created
  every account, protection and merge gate but never any labels, so a GitLab project had no
  `authorization-needed` / `approval-needed` queue-legibility pair (nor the `review-request`
  dispatch token or the `raised-by:<role>` provenance stamps). A missing label degrades
  **silently** on GitLab — `deskflip`'s `authorization-needed` → `approval-needed` swap fails,
  and `deskfile --raised-by` drops the stamp — so the script now creates the full set under
  `--project` via `POST /projects/:id/labels`, idempotently (a duplicate name answers 409, or
  400 "already exists" — both the success case for an ensure, matching the forge seam). This is
  the GitLab twin of the GitHub `create-labels` adoption primitive: the same names, colors and
  descriptions, so the two adoption profiles are label-parity. Colors are sent with the leading
  `#` GitLab requires. The GitLab adoption guide's by-hand table gains the matching endpoint row.
- Recorded, with measurements, that **v1.0.4 supersedes v1.0.3 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1, v1.0.2 and v1.0.3 re-pins recorded. An
  adopter already on v1.0.0, v1.0.1, v1.0.2 or v1.0.3 has no migration to run, only a re-pin,
  while an adopter on v0.28.0 upgrading straight to v1.0.4 still runs the brief-v1 → brief-v2
  migration on the way through rather than being skipped past it.
- The v1.0.4 umbrella this pins to carries statusgen's committer-identity cross-check on
  verified/done briefs (a second, git-derived signal beside the free-string attribution check,
  a NOTICE that degrades loudly when git cannot answer and never over-rejects an honest
  verification whose distinct runners share one identity), and the forge-neutral brief-13
  planning for the write verbs (`deskpr` / `deskfile` / `deskclose` re-seated onto the forge
  resolver — doc/plan only, no tool behaviour change).

### Fixed
- **`statusgen --corroborate` now locates a pre-existing stamp's base cell by column HEADER NAME, not by positional index — so a brief-v2 board migration that re-shapes the Briefs table no longer re-gates historical human sign-offs.** The migration drops the `Gate` column (header and cells) and re-orders columns after `Reviewed`, so the `Reviewed` cell sits at a DIFFERENT positional index on the branch than at the PR merge-base. The pre-existing exemption (#770) compared the stamp's cell against the base cell at the BRANCH's index, which on a re-shaped base read the wrong column and reported a byte-identical sign-off as `MISSING-CORROBORATION` again. The exemption now resolves the base cell by the branch column's header name (e.g. `Reviewed`) against the base table's own header row, falling back to the positional index only when the base table has no header row. Every fail-closed guard from #770 is unchanged — an unresolved non-board occurrence, a nil base, a missing brief row, an edited cell, and a branch column absent from the base header all still leave the stamp fully gated.
- Added a regression test (`TestPreExistingBranchColumnAbsentFromBaseFailsClosed`) pinning the `statusgen --corroborate` pre-existing-exemption fail-closed branch for when the stamp's branch column is ABSENT from the base table's header entirely — the exemption must stay gated (`MISSING-CORROBORATION`) rather than fall back to the branch's positional index and match an unrelated base cell. Tests-only follow-up to #785; no behaviour change.

### Changed
- The adopter-scaffold example gains a v1.0.4 composition manifest with real digests, and its
  notes now name v1.0.4 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1, v1.0.2 and
  v1.0.3 manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.4** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.4
  binaries and verifies them byte-for-byte.

## v1.0.4 — 2026-09-10

### Added
- **A committer-identity cross-check on verified/done briefs.** The attribution check's
  author/verifier comparison reads free strings an agent writes at will; it now layers a
  second, independent signal from git — comparing the git identity of a brief's authoring
  commit against that of the commit that most recently touched it (the Evidence-adding /
  status-flip commit). Where a repository's brief history uses more than one identity, a
  brief whose authoring and Evidence commits sit under a single identity is surfaced (the
  Evidence was not independently committed); where the whole history is one identity, one
  aggregate notice says so rather than emitting a per-brief non-signal. The cross-check is
  a NOTICE, never a hard failure — it never over-rejects an honest verification whose
  distinct runners commit under a shared identity — and it degrades LOUDLY, never
  silently, when git cannot answer (an untracked brief under a `.git` repo). A `git
  archive` export with no `.git` is skipped silently, having nothing to check.
- **`forge-neutral` brief 13 — write verbs C (`deskpr` / `deskfile` / `deskclose`) onto the
  forge resolver.** The code-aware follow-on that brief 04 (`#509`) was ruled down to leave
  undone: it plans the four operations these three verbs still lack (a branch→change lookup,
  a change body/title edit, an issue text-search, and a read-only list-labels), then re-seats
  each verb onto `ForgeFor` — the step that answers the identity-class permit rows instead of
  moving them, supersedes `deskfile`'s interim GitLab named-refusal (`#691`), and drops the
  forge-CLI ratchet by three. Doc/plan only; no tool behaviour changes in this PR.
- Recorded, with measurements, that **v1.0.3 supersedes v1.0.2 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 and v1.0.2 re-pins recorded. An
  adopter already on v1.0.0, v1.0.1 or v1.0.2 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.3 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- The v1.0.3 umbrella this pins to carries `statusgen conform`'s brief-v2 contract (the
  `--emit-schema --schema brief-v2` payload and the `schemas/brief-v2.json` it stays in
  lockstep with), the public flag-day corroboration addition (`statusgen --corroborate` now
  fires its human-stamp anchor for a decision record, `DR-<slug>.md`, not just a brief file),
  and desk-tools fixes (`deskdispatch` worker prompts now name their own loop identity and the
  exact `deskpr create` trailer, and the SessionStart banner resolves the plugin version from
  the manifest instead of a stale literal).

### Fixed
- **A refused `worktree-create` no longer leaves a phantom held claim.** This abort happens
  after the durable dispatch claim is acquired; it now releases that claim the same way the
  adjacent `deskwt add`-failed branch does, instead of returning with the claim still held —
  which had wedged the item behind a claim nobody was acting on until a human hand-deleted the
  ref. A new regression test pins that `deskwt` picking the Windows prefix and `deskdispatch`
  accepting it are one contract.
- **`deskboard`'s drift check now recognises a channel-D `desk-tools-source` pin, so a
  GitLab / native-Windows consumer sweep is no longer permanently STALE-UNKNOWN.** An
  adopter on channel D pins the desk-tools SOURCE line (`desk-tools-source <tag>
  <40-hex-commit>`, the shape `desksourceguard` already reads) rather than a `desk-tools`
  release line, and its consumer checkout carries no in-tree `tools/desk` ref either — so
  `staleState` fell straight through to could-not-check, which pinned `reviewloop`'s idle
  gate at COULD-NOT-CHECK on every tick and made a healthy board look stale forever. It now
  binds the running binary's stamped `sourceSHA` to the pinned commit the same way
  `desksourceguard`'s third agreement does (the stamp is a short SHA, so the full pinned
  commit must have it as a prefix): a match is `in-sync`, a mismatch is a MEASURED `drift`,
  and only a genuinely unreadable source line (a non-40-hex digest) with no in-tree ref
  falls back to could-not-check. This completes the fallback #185 opened — treating a real
  source pin as absent was that fallback landing short.
- **`deskdispatch --kit review` now emits the reviewed head-fetch refspec in the target
  repo's own forge shape, so a GitLab reviewer can check out the MR head.** The review kit
  hard-coded the GitHub coordinate `git fetch origin pull/<N>/head`; on a GitLab-served repo
  the head is advertised at `merge-requests/<iid>/head` (the MR's own `pipeline.ref`), so a
  reviewer that followed the prompt verbatim fetched a ref that does not exist and reviewed
  the worktree's `origin/main` cut rather than the change. The refspec now follows the forge
  resolved for the target repo — GitHub `pull/<N>/head`, GitLab `merge-requests/<N>/head` —
  resolved before the dispatch claim is taken, so a repo whose forge cannot be determined
  refuses the review dispatch (could-not-check) rather than handing over a coordinate the
  reviewer cannot use. The worker/implementer dispatch path, which emits no forge-shaped ref,
  is unchanged. Part of the GitLab-adopter portability family.
- **`deskdispatch` now accepts the Windows worktree home `deskwt` produces, so dispatch
  works on native Windows.** After `deskwt add` succeeded, the `worktree-create` step
  validated the reported home with a POSIX-only leading-slash test (`strings.HasPrefix(home,
  "/")`), which rejected the drive-rooted `C:\...\.claude\worktrees\...` path `deskwt`
  legitimately selects on Windows — aborting every dispatch (no worker, reviewer, or verifier
  could be started). The check now uses a portable absoluteness test (matching the one
  `brief.go` already uses), so the producer (`deskwt`, which picks the Windows prefix) and the
  consumer (`deskdispatch`, which accepts it) judge "absolute" the same way per OS. This is
  the consumer half of the earlier `deskwt`-side portability fixes.
- **`deskpost` and `deskflip` resolve the repo's forge BEFORE minting a GitHub App token, so
  a GitLab adopter no longer hits a misleading `set REVIEWER_APP_ID` error.** Both verbs used
  to mint a GitHub App installation token unconditionally as their first step, so on a
  GitLab-configured repo (`ASSAY_REPO_FORGES=…=gitlab`, authenticating with a PAT file rather
  than an App PEM) a review verdict or ready-flip died with `no App ID for role "reviewer":
  set REVIEWER_APP_ID` — a GitHub credential error that sent the operator hunting `apps.env`
  for a credential the GitLab lane never uses (medici-finance/assay#772).
- **`statusgen --corroborate` no longer re-gates a pre-existing human stamp that a table re-render merely re-emitted.** A `human:<name>` stamp whose Reviewed cell is byte-identical to the same brief's row at the PR merge-base is now reported `PRE-EXISTING` instead of `MISSING-CORROBORATION`, and it does not fail the run. Such a stamp was authored and corroborated on its own earlier PR; a board migration that re-renders whole status tables lands the unchanged row on an "added" diff line, and re-checking it against the migration PR's reviews was a category error that could block the migration. The exemption keys on the brief-id **row** (matched across the re-render, never on line position) and compares the stamp's own cell: a stamp that is NEW on the branch, or whose cell text was edited (date or name), stays fully gated exactly as before, and a genuinely new uncorroborated stamp cannot ride in on a pre-existing one that shares its name: every occurrence must match the base, and any occurrence that is not a board status-table row (prose, an `authorized-by:` line, an Evidence cell, frontmatter) — or an unresolvable merge-base — fails closed, leaving the stamp fully gated. The `human:reviewer` placeholder class is covered generically by the byte-identical rule — it is not special-cased.
- **`statusgen`'s runner-attribution check no longer treats a `(non-implementer)`
  self-label as proof of independence.** `implementerAttributed` used to strip the
  literal `non-implementer` before its substring test, so an Evidence Runner cell
  reading `worker (non-implementer)` — or a Verified cell naming `non-implementer` —
  passed the independence check outright. A session could certify its own verification
  as independent just by writing the label. The predicate now matches the token
  `implementer` in any spelling (case-insensitive), including the self-labelled
  `(non-implementer)`; an honest independent verifier names the runner that ran the
  table (e.g. `opus-verifier`) — a token with no `implementer` substring — which still
  counts. The self-verification error message now states plainly that a self-labelled
  `(non-implementer)` is still a self-assertion.
- **harness-portability/07 Verify rows 5 and 7 re-baselined to the current tree, so they
  test what they claim.** Row 5 read the bundle version from `.version` on the plugin
  manifest — plugin.json in the plugins/assay/.claude-plugin directory — and compared it
  to a live `origin/main` ref; in the public tree that field tracks the plugin-manifest /
  umbrella release (now `1.0.0`), not the bundle content version, and a live-ref compare
  can never discriminate on merged main. It now reads the authoritative `bundle-version`
  from `plugins/assay/SOURCES.yaml` and asserts it is at least `0.3.0`, the version that
  records Assay's second first-class harness — still discriminating (a regression exits
  `1`), with no moving anchor. Row 7's run-log check anchored `Result:` at column 0, but
  the run-log skeleton in `docs/codex-smoke-protocol.md` writes each verdict as an
  INDENTED `  Result:` line, so a complete run log would have counted zero and falsely
  FAILed the stream's acceptance row; the anchor is now `^[[:space:]]*Result:`. Row 7
  stays BLOCKED pending the live Codex run and the brief stays `implemented` — no status
  advance.
- A forge that cannot be **positively** resolved (no roster binding and no known origin host)
  is treated as could-not-check, never as GitLab, so every GitHub repo keeps its exact prior
  behaviour. Part of the GitLab-adopter portability family (siblings #773–#776).
- The `evidence-automerge` `enable` job no longer reddens when a ready-flip leaves a PR with
  nothing left for auto-merge to wait on. `enablePullRequestAutoMerge` is a request, not a
  merge: a just-flipped approved PR whose checks are all green already satisfies every merge
  requirement, so GitHub refuses the enable with "Pull request is in clean status". That
  refusal published a failed check — the same self-inflicted loop as the "unstable status"
  case. `tools/evidence-automerge/automerge-refusal.sh` now treats a "clean status" refusal
  as the fifth benign outcome (exit 0 with a `::warning::` reason); nothing here merges, so
  the PR simply awaits a human's merge click. The staged workflow's `Request auto-merge` step
  now also echoes the mutation's answer to the log before deciding, so a benign skip still
  records why it skipped (#586).
- `deskflip` checks-green no longer stays permanently could-not-check on tokens that
  lack the `administration` scope. When the legacy branch-protection required-status-checks
  endpoint answers `403`, `deskkit.RequiredStatusChecks` now re-resolves the required set
  through admin-free endpoints — `GET /repos/{o}/{r}/branches/{b}` (`.protected`) and, for a
  protected branch, `GET /repos/{o}/{r}/rules/branches/{b}` (each `required_status_checks`
  ruleset rule's contexts). Being a flip gate it fails CLOSED: only a positively unprotected
  branch is read as empty/green; a protected branch whose required set cannot be determined
  admin-free (classic branch protection, invisible to the rules API, or an unreadable endpoint)
  stays could-not-check rather than being read as green.
- `deskflip` now authenticates a GitLab flip from the GitLab PAT custody path (via the forge
  resolver) instead of the GitHub App mint; its reads and writes were already forge-neutral,
  so this was the last GitHub-shaped step in the flip.
- `deskpost` (review / security-review / comment / ready) now fails **closed** with an honest
  could-not-check that names the resolved forge — its verdict/flip precondition reads still
  run through a GitHub App-authenticated client, so the GitLab write path is the follow-up —
  rather than the GitHub App-ID mint error.

### Changed
- The adopter-scaffold example gains a v1.0.3 composition manifest with real digests, and its
  notes now name v1.0.3 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1 and v1.0.2
  manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.3** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.3
  binaries and verifies them byte-for-byte.

## v1.0.3 — 2026-09-10

### Added
- **`statusgen --corroborate` now accepts a decision record (`DR-<slug>.md`) through the
  decision-issue anchor.** The third human-stamp corroboration anchor — a linked, blessed-human-CLOSED
  `needs-decision` issue carrying the per-record `<!-- decision-gate: <id> -->` marker — previously
  fired only for a `human:<name>` stamp found in a `brief-<NN>.md` file. It now fires for a stamp in a
  `DR-<slug>.md` design-decision record under `docs/streams/decisions/` too, corroborating the
  record's `decided-by:` name against the CLOSER of the decision issue the record links. A DR's
  approving human ratifies by closing the `needs-decision` issue, not by signing the DR's own PR, so a
  concrete `decided-by: "human:<name>"` on a DR used to come back MISSING-CORROBORATION and the only
  sanctioned notation was the literal `human:<name>` placeholder that names nobody. The addition is
  strictly ADDITIVE: the two PR anchors (an APPROVED review, an explicit approval comment) and the
  original brief-file anchor are unchanged, and all three of the anchor's conditions — closed by the
  blessed login, marker naming THIS exact record, and the record linking the issue — remain
  independently required. The two record-id namespaces never collide (a brief id carries a `/`, a DR
  id never does).
- Authored the `docker-publish.yml` `desk-images` job (builds and publishes the
  shared `desk-base` image and the five per-desk images — `intake-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk` — version-locked
  to the same base tag, alongside the existing `desk-tools` image, gated by
  `containers/scripts/layer-secret-scan.sh` against all six images before any
  push). Parked at `docs/streams/desk-containers/pending-docker-publish.yml`
  pending application by a human with `workflows` permission — the worker
  App's push of `.github/workflows/*` is rejected on this repo.
- Recorded, with measurements, that **v1.0.2 supersedes v1.0.1 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 re-pin recorded. An adopter already
  on v1.0.0 or v1.0.1 has no migration to run, only a re-pin, while an adopter on v0.28.0
  upgrading straight to v1.0.2 still runs the brief-v1 → brief-v2 migration on the way through
  rather than being skipped past it.
- The migration this repo runs against itself now lives at
  `migrations/0001-v0.28.0-to-v1.0.0-derived-board.md`, so `deskmigrate` and
  `assay:upgrade-assay` can be dry-run against this tree the same way an adopter runs
  them against theirs.
- `deskdispatch` worker prompts now name the worker's own loop identity
  (`export DESK_LOOP=worker-desk`) so the desk write verbs (`deskpr create`,
  `deskfile`, `deskreply`) stop refusing with `$DESK_LOOP is unset`, and — for an
  issue-only item — the exact `deskpr create` trailer (`Issue: #<N>`) and the
  sanctioned verb to comment on the issue it was dispatched from
  (`deskfile attach -R <owner/repo> --to <N>`), replacing hand-rolled `gh` writes.
- `docs/UPGRADING.txt` — the append-only local record of which migrations have been
  applied to this tree.
- `schemas/brief-v2.json` — the machine-readable brief-v2 contract, alongside the
  existing brief-v1 one. It covers the whole brief-v1 surface plus the hierarchical
  `brief: <cell>:<repo>:<stream>:<NN>` id, the `version:` revision counter, and the
  reserved `id` / `supersedes` / `gates` / `feathers` / `verify` keys, and is held in
  lockstep with the reference validator by `TestBriefV2SchemaCoverage`.
- `statusgen conform --emit-schema --schema brief-v2` prints the brief-v2 contract, so
  every embedded artifact stays reproducible from a pinned build.

### Fixed
- **The SessionStart banner no longer hard-codes a stale plugin version.** It said
  `assay plugin v0.1.0` long after `plugins/assay/.claude-plugin/plugin.json` moved to `1.0.0`.
  `resident-rules.md`'s Header now carries a `{{VERSION}}` token that `harnessgen resident`/
  `harnessgen cursor` resolve from the plugin manifest at generation time — never a literal a
  human can forget to bump — and `--check` reddens if a manifest bump lands without
  regenerating. `inject-resident-rules.sh` now reads the generated payload file instead of
  carrying its own duplicate copy of the rules text, which had also silently drifted from the
  single source on rule 8's wording.
- **The desk body-check no longer refuses an uppercase-hex PGP key fingerprint as a
  possible secret.** A 40-char OpenPGP v4 fingerprint is written in UPPERCASE hex, but the
  high-entropy-run scanner exempted only LOWERCASE hex (git SHAs), so a body quoting a
  `.sops.yaml` recipient list (`pgp:`) or a sops metadata `fp:` field tripped the "40-char
  high-entropy run (possible secret)" refusal — blocking writes that merely referenced a
  PUBLIC key fingerprint. A new narrowly-anchored exemption admits a run that is EXACTLY 40
  uppercase hex ONLY when a `pgp:`/`fp:` recipient key precedes it (a `.sops.yaml` recipient
  entry or a sops `fp:` field), separated by nothing but YAML/JSON value scaffolding or a
  comma-list of fingerprints. The anchor is load-bearing and the check is not loosened for
  genuine secrets: a bare uppercase-hex run with no recipient key, a lowercase/mixed-case
  40-char run, and a real high-entropy token wearing the same field all still refuse (an AWS
  secret key, for instance, is 40 mixed-case base64 and never qualifies).
- De-flaked `cmd/fanoutloop` `TestPool` (and its sibling engine-integration tests) so a
  whole-module `go test ./...` no longer reddens intermittently under CPU load. The tests'
  wall-clock wedge safety-nets were calibrated to unloaded speed (5s); under the saturation of a
  full-module run their near-instant async conditions missed the deadline even though nothing was
  wedged. The deadlines now route through one load-tolerant `engineTestTimeout`, removing the
  timing/load assumption without weakening any assertion.
- `deskreply --workpad` now finds and edits its own prior workpad comment instead of
  appending a new one on every call. `GitHubForge.ListComments` re-suffixes a GraphQL Bot
  author's bare slug to the `<slug>[bot]` REST rendering, so a worker's own comment matches
  its own identity through `SameActor` and the one-workpad-per-PR upsert holds (#747).
- `statusgen conform` now selects its contract **per file** from that file's own
  `schema:` marker, so a tree migrated by `statusgen migrate brief-v1-to-v2` validates
  instead of reporting `could-not-check` for every brief and exiting 2 — which reddened
  the schema-contract check on the very PR that landed an adopter's flag day. A tree
  mid-migration holding both versions validates each file against the version it
  declares. The three-state behaviour is unchanged: a marker no embedded contract
  describes is still `could-not-check` / exit 2, and a brief-v2 field error is a real
  `checked-failed` / exit 1.

### Changed
- **Flag day: this repo's own board is now brief-v2.** All 148 briefs under
  `docs/streams/` were migrated from `schema: brief-v1` to `schema: brief-v2` by the
  declarative `v0.28.0 → v1.0.0` migration: each brief's `brief:` id becomes the
  hierarchical `<cell>:<repo>:<stream>:<NN>` form resolved through the alias registry,
  every brief gains `version: 1` and a once-minted uuid `id:`, and each of the 16 stream
  READMEs has its Briefs table wrapped in the generated-region markers with
  `board: generated` in its frontmatter. The lifecycle cells of every row were carried
  through unchanged — the migration re-shapes the board, it does not re-decide it.
- The adopter-scaffold example gains a v1.0.2 composition manifest with real digests, and its
  notes now name v1.0.2 as the umbrella an upgrade moves to. The v1.0.0 and v1.0.1 manifests
  stay: a tree pinned at either still has to resolve, and the brief-v1 → brief-v2 migration's
  span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.2** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.2
  binaries and verifies them byte-for-byte.

## v1.0.2 — 2026-09-10

### Added
- **`DESK_TRACE=1` — a diagnostic switch for the desk tools.** Turn it on (or pass a global
  `--trace`) and a failing verb prints the full cause chain, every child process it started with
  the command line as executed, that child's exit status and elapsed time, and the failing
  child's stderr in full. Credentials are redacted at one choke point — GitHub token prefixes,
  URL userinfo such as `https://x-access-token:…@`, `Authorization:` headers and secret-shaped
  environment assignments — and marked `<redacted>` rather than silently elided. With the switch
  off, output is byte-identical to before. Retrofitted onto `deskdispatch`, `deskwt`, `desktoken`
  and `deskfile`; other verbs are unaffected.
- Recorded, with measurements, that **v1.0.1 supersedes v1.0.0 as the upgrade target without
  superseding the flag day**. An adopter already on v1.0.0 has no migration to run — only a
  re-pin — while an adopter on v0.28.0 upgrading straight to v1.0.1 still runs the brief-v1 →
  brief-v2 migration on the way through rather than being skipped past it.

### Fixed
- **A board with an extra authoring column keeps its lifecycle cells through a
  re-render.** The Status / Verified / Reviewed columns were read back from fixed
  offsets, which is correct only for the canonical seven-column table. A board carrying
  an extra column (a `Gate` column between Effort and Status is the shape in the wild)
  had every lifecycle cell read one position to the left, so a re-render wrote the gate
  value into Status and the real status into Verified — silent loss of lifecycle state,
  on the one run hardest to notice because a hundred other files change with it. The
  columns are now keyed on the header names, as the board parser already does.
- **A brief whose title contains a `|` no longer breaks the board it is rendered into.**
  The generated Briefs table interpolated titles raw, so a title such as
  `` `--cadence weekly|monthly` `` emitted a row with one cell too many and the board's
  own parser then rejected the whole stream. Titles are now escaped for the cell they
  land in; the parser already understood the escaped form, so only the render half was
  missing.
- **A failed child's own message now reaches the operator on the first read.** Every desk tool
  opens stderr with its `assay-config:` echo, so a step report built from the first stderr line
  printed the echo and never the diagnosis — `deskdispatch` reported claim and roster failures as
  `(assay-config: …)`, and its worktree-path checks returned a bare `exit status 128` with
  nothing in it to search for. Failures now end on `— <tool> said: <the tool's own first line>`,
  with the rest carried on the error for the trace to print.
- **`deskclaim-ref` no longer silently dials `gitlab.com` on a self-hosted GitLab.** When go-git
  could not read the origin remote — the `worktreeConfig` extension it does not support, which git
  itself enables under the linked-worktree model the desks require — the tool fell back to the
  canonical SaaS host, so a self-hosted PAT was presented to `gitlab.com`, denied, and every claim
  verb exited `unverifiable` (6). That stalled `deskdispatch --kit review` at `claim-acquire`, so a
  review desk could not fill a reviewer slot. Three independent fixes:
- **`desktoken` says that stdout is a PATH.** A caller that used the output as a credential got
  `401 Bad credentials` from its next forge call, three processes downstream and naming nothing.
  A one-line NOTICE now accompanies the path on stderr, with the incantation that reads the
  value. Stdout is unchanged, so callers that pipe it are untouched.
- **`deskwt` and `deskfile` failures carry the command line and the child's exit status**, so a
  trace has something to show for them; `deskfile` can still tell a `401` from a `403` from a
  `429` on its fail-closed dedupe-search path.
- **`statusgen migrate brief-v1-to-v2` no longer refuses a tree that has a register.**
  The migration enumerated every directory under `docs/streams/` as a stream, so a
  REGISTER directory — which by design carries a README with no frontmatter and no
  Briefs table — aborted the whole flag day with `exit 5: no recognisable Briefs table`
  and left every real stream unmigrated. It now applies the same two rules stream
  discovery already applies: the reserved register names, and the "a register, not a
  stream" self-declaration.
- The generated Briefs table no longer TRUNCATES a board's own columns. A stream
  README whose table carried a column beyond the canonical seven — a trailing
  `What's landed`, an `Owner` column — lost that column entirely on the first
  `statusgen migrate` / `regen --readmes`, header and every cell, because the
  render emitted a fixed seven cells. Non-canonical columns are now carried
  through verbatim, appended after `Reviewed` in the order the source header
  lists them. `Gate` remains the one column the layout deliberately drops (it
  duplicates the brief's own `gate:` frontmatter key), and it is now dropped
  header-and-cells together rather than shifting the row.
- `deskclaim-ref` reads the origin remote through a `worktreeConfig`-aware path: it falls back to
  native `git remote get-url origin`, then to a direct parse of the common `.git/config` (which
  resolves a linked worktree's `commondir`), covering both the shared checkout and the role
  worktree.
- `deskkit.ForgeKindFromSlugAndHost` never defaults the host to the SaaS instance when the caller
  supplies none — a roster entry names the forge **software**, not the **instance** — and returns
  could-not-check instead of a guess.
- `deskmigrate` no longer reports a SILENT no-op on an unmigrated tree. When no
  migration covers the requested span but the tree still carries
  `schema: brief-v1` files under `docs/streams/`, it exits non-zero naming the
  migrations directory it looked in and the vendoring step, instead of printing
  `no migrations for vX -> vY (clean no-op)` at exit 0 — output an operator
  cannot tell from a completed migration. A genuinely migrated tree is still a
  clean no-op at exit 0.
- `statusgen` board-honesty: the `NON-DISPATCHABLE (re-homed)` NOTICE now keys on
  the ROW's own re-home marker (`[homed→…]`, `deliverable-repo:`, or explicit
  do-not-re-implement wording in the brief cell), never on a stream-level README
  inference. A stream whose README merely mentions re-homing no longer flags every
  file-less `todo` row as re-homed (statusgen #709).
- a fail-closed transport error now carries the host it dialed and the underlying cause —
  `could not create the claim refs/dispatch/<id>: <host>: <error>` — so the message attributes
  the failure instead of costing an operator a debug cycle on the wrong suspects.

### Changed
- Every `deskmigrate` run now prints the number of migrations SELECTED and the
  number of planned file actions, so "nothing matched the span" reads
  differently from "matched, and already applied".
- The adopter-scaffold example gains a v1.0.1 composition manifest with real digests, and its
  notes now name v1.0.1 as the umbrella an upgrade moves to. The v1.0.0 manifest stays: a tree
  pinned there still has to resolve, and the brief-v1 → brief-v2 migration's span ends at v1.0.0.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.1** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.1
  binaries and verifies them byte-for-byte.
- `deskkit` grows one shared subprocess runner (`deskkit.Run`) that replaces three divergent
  per-command runners, and a shared error-report path (`deskkit.ReportError`) that every
  retrofitted verb's `main()` uses. `DeskError` carries subprocess detail out of band and gains a
  `Cause()` accessor and a `RefusedWithCause` constructor; `Refused`'s signature is unchanged, and
  a refusal that gains a cause is still a refusal with the same exit code.

## v1.0.1 — 2026-09-09

### Added
- Stream WIP cap: the `stream-cap` lint caps the number of `status: active` streams in a root at the operator-set `ASSAY_STREAM_CAP`. A full lint only NOTICEs a standing over-cap (the daily regen never gates); a PR diff that adds an active stream past the cap with no offsetting park is a PROBLEM — no net new streams past the cap. Absent `ASSAY_STREAM_CAP`, the rule is inert (one NOTICE; no default number).
- `author-brief` skill now states the authoring-time consumer-routing rule: a brief-authoring PR
  declares future consumers but does not edit them, so a path the brief's own implementation will
  later touch routes to the deferred disposition (`follow-up <stream>/<NN>` at the brief itself),
  never `fixed-here` — with a worked wrong/right example.
- `parked` stream status: a shelved stream keeps its briefs but is excluded from Next-up and every dispatch view, renders under its own `## Parked` board heading, is counted separately from active, and re-activates by a README `status:` flip (itself subject to the cap).
- `stream-source` lint + stream README `spec:` field: a change that adds (or flips to) an active stream must cite in `spec:` a scoping doc whose header is `**Status:** approved` (spec/lifecycle-v1.md §8.1); a `parked` stream may cite a `draft`.

### Fixed
- Restored a v1.0.0 release-note entry that went missing when the harness-portability/15
  implementation PR overwrote the brief-authoring PR's changelog fragment instead of adding
  alongside it; the fragment was rolled up in its overwritten state, so the spec entry never
  reached the published notes.
- `statusgen --lint` no longer reds on main after a release. The harness-portability/15 brief
  named its changelog fragment as a backticked path, and the v1.0.0 roll aggregated that fragment
  into `CHANGELOG.md` and cleared the directory — leaving a backticked path to a file that is gone
  by design. The brief's deliverable claim and its Verify row now point at the delivered content
  in the `v1.0.0` section instead of at the consumed file, so the claim survives the roll that
  fulfils it. The class — a deliverable reference that a release deletes — is under
  needs-decision on #722.
- `statusgen` no longer reds a whole board over a legal table row. A briefs-table
  cell may hold a backslash-escaped pipe (`\|`) — in GitHub Flavored Markdown
  that is the only way to write a pipe inside a cell, and it applies inside a
  `code span` too — but the row splitter cut on every `|` byte, so such a row
  came out one cell too long and the exact cell-count check rejected it. Because
  a stream README parse error aborts the whole load, that one row turned every
  other check in the run into could-not-check. The splitter now treats an escaped
  pipe as cell content and keeps the escape sequence verbatim, so a
  parse-then-re-render round trip is byte-identical. A genuinely column-shifted
  row (a PR reference decorating the Status cell, a stray `||`) is still
  rejected — the count check is unchanged.

### Changed
- The `statusgen --consumers` gate's DISPROVED-`fixed-here` messages now name the deferred
  disposition as the fix, so an author whose authoring PR reddens the routing gate is pointed at
  the correct routing token instead of only being told the claim is contradicted.
- The adopter-scaffold example's v1.0.0 composition manifest carries the real release digests
  instead of fixture placeholders, so the upgrade target an adopter dry-runs against now shows
  the same values their own pin file will hold.
- The desk skill bodies now name **`deskclaim-ref`** as the default dispatch-claim tool —
  installed with desk-tools, verbs `acquire` / `progress` / `release` / `steal` / `show` /
  `list`, deskkit exit codes 0/5/6 — with "a repo may ship its own `tools/dispatch-claim.sh`,
  which `deskdispatch` prefers when the resolved root carries it" as the documented override.
  `worker-desk`, its `dispatch-runbook` reference and `pr-shepherd` previously described only
  the consumer script, which a green-field or native-Windows adopter never has.
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.0**, on all ten platform lines (darwin arm64/amd64, linux amd64, windows
  amd64/arm64 for each). Every digest was harvested from the release's own checksum manifest
  and compared field-for-field against it, so a cold `assay:install` resolves the v1.0.0
  binaries and verifies them byte-for-byte. `linux-arm64` stays deliberately unpinned in both
  sections — v1.0.0 publishes no such asset, and the acquisition refuses rather than guesses
  when a detected platform has no pin line.
- Those bodies also now state that a claim read must run BOTH listings: the claim tool acquires
  and lists in `refs/dispatch/*`, while `git ls-remote origin 'refs/heads/dispatch/*'` lists the
  branch refs the Go claim readers use — a known, unresolved divergence a single read can miss.

## v1.0.0 — 2026-09-09

### Added
- **Report packs** — a named, mechanically-checkable install unit for periodic reporting
  tools. A pack ships as a sha256-pinned release binary, emits its own CI via `<tool> init`,
  keeps its committed output single-writer, and loads operator values from config. The
  normative contract is `docs/report-packs.md` (linked from `docs/distribution.md`).
- A staged `ci.yml` patch (`tools/harnesslint/ci.yml.patch`) that runs the `harnessgen`,
  `harnesslint` and `plugindrift` suites in CI instead of only building and vetting them, and adds
  a job running the neutrality lint against the real bundle tree — the fixture-based unit suite
  never reads it.
- Adopting-Assay runbook now documents Windows, GitLab, and Cursor as a combined install path: a
  forge/harness/OS chooser table, the two Windows install lanes (channel E release vs channel D
  from-source), Windows config-home/PATH/child-process rules, the GitLab parity-vs-provisioning
  split with the Free/CE core-lane stance (ruling #219), group-not-personal-namespace and
  three-credential-classes guidance, and a Cursor copy-skills install (no marketplace). Supersedes
  #650.
- An only-widens house rule-pack seam (`ASSAY_UNTRUSTSCAN_CALLOUT`) can ADD detections but
  can never clear a built-in flag; a configured pack that fails to answer degrades a clean
  family to could-not-check. Built on the positive-control corpus from #634.
- Component manifests (`component.yaml`) for every unit of the `docs/adopting-assay.md` §2 inventory — the skills, hooks, desk tools, statusgen, and the forge/scaffold-side units — each declaring the keys it `provides` and `injects` and its ordered `apply` steps (composability/00, #624).
- Reproducibility is hash-pinned per entry (same table in, same bytes out) and inertness is
  asserted over the generated bytes (no live callout host, no credential shape). `untrustcorpus
  check` validates that every entry names an existing detector layer and no layer or entry is
  orphaned.
- Same-tag pin lint: `statusgen --lint` PROBLEMs a `.assay-versions` whose artifact tags differ (one tag, one tree — no version matrix).
- Six typed read ops on the frozen `Forge` interface, each with its `deskboard` call site
  in the same change: `ListRecentCommits`, `GetCommit`, `CompareRefs`, `SearchOpenChanges`,
  `ListWorkflowFiles`, and `ChangeDiff`. The commit reads map 1:1 on GitLab; the other four
  are could-not-check-with-gap there (a genuine non-1:1 each). `PullRequest` gains `MergedAt`
  and `Merged`, `IssueSummary` a `URL`, and `LabelEvent` a `CreatedAt`, each with its
  consumer; the combined-status total folds into `ChecksAtHead` rather than a new op.
- The brief-reading version gate: `statusgen`, `deskboard`, `deskpr`, `deskclaim` and `deskevidence` built below v1.0.0 refuse a `brief-v2` tree (exit 6), pointing at `assay:upgrade-assay`. Unstamped local builds behave as latest and are never gated.
- The first REAL migration, `0001-v0.28.0-to-v1.0.0-derived-board` (source umbrella v0.28.0 — the latest at cut time, ratified on medici-finance/assay#453), plus `docs/release-notes/v1.0.0.md` (same prose): what changes on an adopter's board, the `Brief:` trailer they must now write, and the `statusgen reconcile --backfill` step they add to their board workflow.
- The gate's reader runs under the refuse-everything containment profile of the pinned
  decider runner entry: an empty filesystem root and callback policies that refuse every
  fs / terminal / tool request and file the attempt as a containment anomaly.
- The scanner emits a NEUTRALISED rendering — every invisible/bidi/control codepoint
  escaped to a visible `\uXXXX`, the whole body fenced as inert data — for downstream
  quarantined reads (briefs 05/06) to consume in place of raw untrusted bytes.
- `--dora-limit` (default 500) caps each recorded timing series the feed aggregates. The cap
  is declared in the feed's own `caps` block, so a capped number is never published as though
  it covered the whole window, and a capped metric reports itself `partial` rather than
  `measured`.
- `commsgw` now screens **every** outbound send through a quarantined prose gate on the
  gateway send path, after the deterministic pre-checks pass. Within-cell and cross-cell
  sends alike are consulted with no risk-trigger predicate — an independent second layer
  that fails on a different signal than the tokens-only, slug-blind body scanner. The gate
  is advise-only (it never rewrites content): its Decide-shaped verdict is one of
  `clean-send` / `hold-for-human` / `refuse`, and any non-clean verdict HOLDS the message
  (held mailbox plus a filed issue carrying the payload DIGEST, never the raw payload) — a
  hold is never a silent drop or an auto-retry.
- `components/KEYS.md` — the authoritative namespaced-key catalogue, with each key's providing component and meaning.
- `deskavatar` — a new desk tool that generates the deterministic, on-brand
  avatar set an adopter uploads for their Assay Apps. It composes an
  octagon-framed SVG per App, rasterises it offline to PNG (pure-Go, no cgo), and
  runs a 20 px legibility proof (CIELAB ΔE + glyph-silhouette IoU) over the set
  before writing anything — refusing (exit 5) and naming the pair if two tiles
  would be indistinguishable in a PR timeline. Same org, same tier, same bytes.
  Importable as `internal/avatar.Generate` for the installer to call in-process.
- `deskclaim-ref` — a pure-Go port of the consumer dispatch-claim script (verbs
  `acquire`/`progress`/`release`/`steal`/`show`/`list`). It speaks the same durable,
  cross-machine claim protocol (the `refs/dispatch/<id>` ref namespace, holder encoding, and
  0/5/6 exit codes) with no shebang and no bash — the only claim path that runs native on a
  Windows adopter. Every forge access is an **in-process git-smart-HTTP** call over go-git —
  no `gh`/`glab`/any CLI and no external `git` process. A claim is minted as an annotated tag
  and placed with an explicit-old (server-side compare-and-swap) receive-pack push, so a create
  loses cleanly against an existing claim and an advance/steal loses against a value that moved
  underneath it (closing the races the previous `PATCH force=true` advance and DELETE-then-POST
  steal left open). The forge (github/gitlab) resolves from `ASSAY_REPO_FORGES` or the origin
  host, and the credential from `--token-file` or `GH_TOKEN`/`GITHUB_TOKEN`/`GITLAB_TOKEN`; the
  tagger date is client-stamped (mutual exclusion rests on the server-side CAS, not the clock).
- `deskkit.ForgeKindFor` resolves which forge serves a repo WITHOUT obtaining a credential or
  constructing a backend — the read a caller makes to branch on or refuse a forge while still
  filing under its own ambient identity, so `deskfile` can name the unsupported forge without
  taking on App-token custody (#687).
- `deskmanifest lint` (`tools/desk/cmd/deskmanifest`): discovers every `component.yaml`, resolves every `inject.required` key to a `provides`, checks version ranges, and reports dependency cycles from the declarations alone — three-state (exit 0 clean / 1 problems / 2 could-not-check). Wired into the board-lint CI job so an undeclared dependency is a CI failure instead of an outage.
- `deskmigrate` `statusgen-regen` op — a declarative, dry-runnable migration step that runs the pinned statusgen's `migrate` verb over the adopter tree (an unknown verb/target is refused, not run blind).
- `deskscanuntrusted` — a deterministic INBOUND exfil + injection pre-scanner (the inbound
  sibling of the outbound `bodycheck`). It reads untrusted content bytes with no model and
  no network and returns a three-state, fail-closed verdict across three independent
  detector families: exfil/callout markers (env reads, outbound sinks, secret-file paths),
  invisible-Unicode / bidi / imperative-lure injection markers, and a Semgrep code-exec leg
  (base64→exec, install-hook override, command overwrite, steganographic extract→exec).
  A missing Semgrep makes the code leg could-not-check, never clean.
- `docs/streams/graph-repos.yaml` reserves a sixth alias, `mp`, for the cell's platform repo. Like its withheld siblings it carries `repo: null` and `unpublished: true`, so a `mp:<stream>/<NN>` or `mp#<issue>` reference parses and lint-validates from this public tree while resolving past the alias stays a could-not-check. Reserving it here keeps the public alias SET identical to the private copy's, which is what the planned registry drift lint compares.
- harness-portability/15 (spec): the follow-up brief to hp/14 (#631) — wire the three de-housed
  Go modules' test suites (`harnessgen`, `harnesslint`, `plugindrift`) plus the real-tree
  harness-neutrality lint into public `ci.yml` so roster/version drift can no longer land green,
  scrub the four banned harness tokens flagged in `ask-decision`/`install` skill bodies, and
  declare `references/desk-shell.md` a non-matrix reference the `harnesslint bindings` check skips
  by declaration (not nineteen per-line suppressions).
- `harnesslint bindings` understands a reference file that DECLARES itself out of the
  per-harness binding matrix, via a one-line
  `<!-- assay:harnesslint non-matrix-reference — <reason> -->` marker. `references/desk-shell.md`
  — harness-neutral shell and transport mechanics, never a capability binding — carries the
  declaration and stops producing nineteen violations. The skip is narrow and loud: the reason is
  mandatory, every skipped file is named on stderr, an undeclared reference is still fully
  checked, and declaring every reference out is a could-not-check rather than a clean sweep.
- `opmetrics --transcripts` is now **repeatable** and defaults to **every `~/.claude*/projects`** profile on the machine, deduping a session synced across profiles so it is counted once. Zero readable roots is `could-not-check`, never a silent zero.
- `opmetrics` now emits an **attention-class breakdown** (`operator.attention_families`) alongside the relay families — counts of route / status / toil / correction / decision / idea / ack / other for each non-empty operator turn (classifier `opmetrics-relay/2`, day-file schema `opmetrics/2`). Every `opmetrics/1` key is unchanged, so existing readers keep working and the relay-ratio trend line is unbroken.
- `qualgen init` scaffolds the report pack into an adopter repo — a generated, single-writer
  quality-report workflow plus a `.assay-versions` pin — acquiring `qualgen` only as the
  pinned release binary.
- `qualgen` joins the umbrella release as per-platform binaries with checksums, so an adopter
  pins a `qualgen-<platform>` line in `.assay-versions` and obtains the quality-report tool
  without building from source.
- `scripts/build-windows.ps1`: a PowerShell counterpart to the root `Makefile` that
  builds and installs the desk-tools on Windows (`.exe` outputs, a per-user
  `%LOCALAPPDATA%\Assay\bin` install, `Get-FileHash` manifests) with no `nmake`,
  Visual Studio build tools, or `make` required — only PowerShell and the Go toolchain.
- `statusgen --corroborate`: a THIRD accepted corroboration anchor for a `gate:human`
  `human:<name>` stamp, alongside the existing APPROVED-review and approval-comment
  anchors on the brief's own PR. A stamp now also corroborates through the
  needs-decision channel — but ONLY when all of the following hold together: a
  needs-decision issue is CLOSED by the blessed human (`ASSAY_BLESS_LOGIN`), that
  issue carries the per-brief marker `<!-- decision-gate: <stream>/<NN> -->` naming
  THIS exact brief, AND the brief LINKS that issue. This aligns the lint with the
  sanctioned ratification channel — a `gate:human` decision recorded by closing its
  needs-decision issue rather than as a PR approval. The two PR anchors are
  unchanged; the new path is additive and fails closed (no bless login, no link,
  wrong closer, an open issue, or a marker for another brief all leave the stamp
  MISSING-CORROBORATION).
- `statusgen --dora-json` emits the frozen full-DORA publish feed — `change_lead_time`,
  `change_failure_rate` and `time_to_restore` — as byte-stable JSON on stdout, ready for a
  metrics publish pipeline to wrap verbatim. Deployment frequency is deliberately absent:
  it is sourced by a separate delivery-metrics platform, and two emitters writing one
  frozen key is a collision the shape cannot resolve.
- `statusgen --drive-snapshot <slug>` prints the `## Drive: <slug>` dashboard section on its own — the same render `drivedash` writes into `STATUS.md`, offline and `STATUS.md`-free — so a drive-plan file can carry a fenced, generated snapshot of the board's own truth instead of a hand-maintained "are we done yet" table that drifts and then lies. With the `--check <file>` modifier it compares that file's `drive-snapshot:begin … :end` region against a fresh render (exit `0` identical, `1` drift, `2` could-not-check), neutralising the volatile last-regen heartbeat so only a hand edit inside the fences counts as drift.
- `statusgen --lint` gains an **`opmetrics-stale` NOTICE** when the newest operator-load day-file under a root is older than three days — a root that has never carried one stays silent.
- `statusgen migrate brief-v1-to-v2 [--dry-run]` — the brief-v1 → brief-v2 flag-day migration: rewrites each brief's `schema:`, mints the hierarchical `<cell>:<repo>:<stream>:<NN>` id from `docs/streams/graph-repos.yaml`, adds `version: 1` and a uuid v4 `id:` where absent, and wraps each stream README's Briefs table in the generated-region markers with `board: generated`. Idempotent; refuses (exit 5) when the alias registry is absent.
- `tools/winparity` gained a second, fail-closed assertion alongside the target
  parity check: `scripts/build-windows.ps1` must be Windows PowerShell 5.1-clean
  (ASCII-only, no `>>>` in strings). The scan reads the file as bytes, so a
  regression reddens on the Linux CI leg — no PowerShell needed — before it
  reaches a native Windows host. The Windows script runs the same guard as a
  preflight (#678).
- `tools/winparity`: a fail-closed, three-state parity guard that asserts the Windows
  build script's target set equals the Makefile's `.PHONY` set, so the Windows build
  cannot silently fall behind (or run ahead of) the Unix target set. The Windows script
  runs it as a preflight; a staged `ci/staged-workflows/winparity.yml` is the Linux-CI half.
- `untrustcorpus` desk-tool + `internal/deskkit/untrustcorpus` package: a positive-control
  corpus for the untrusted-read sample corpus, defined as a codepoint/description table plus
  a deterministic generator that assembles each inert sample's bytes at run time — no decoded
  attack string is committed to the tree.

### Fixed
- Absent sibling checkouts are now scoped to the claim: at boot an unclaimed brief's missing cross-repo checkout is a NOTICE, not a boot-blocking failure, so one unclaimed cross-repo brief no longer bricks every loop's boot in a cell. Only the brief named by the new `--claimed-brief` flag turns its own absent sibling into a hard failure (#661).
- Corrected the channel-D `.assay-versions` guidance: channel D pins the git commit its CI clones
  and rebuilds and does **not** write a `-source` line; the `statusgen-source` / `desk-tools-source`
  pin is the separate release-provenance grammar `<artifact> <tag> <40-hex-commit-SHA>`.
- GitLab CE/Free review desks are no longer blinded by the missing Premium approval-config
  route. `ReviewsAtHead` now treats a 404 on `GET /projects/:id/approvals` as the documented
  CE gap and degrades head-pinning only — approvals are reported unpinned (advisory) and the
  head is taken from the verdict note SHA — instead of failing the whole review read closed.
  A 403 (tier gate) or 401 stays a could-not-check for the entire read. This lets
  `deskboard actions`/`reviews` classify NEEDS-REVIEW/RE-REVIEW on CE queues again.
- GitLab adopters: `deskboard` board reads (`actions`, `prs`, `stalled`, `throughput`) no longer fail could-not-check. The GitLab forge backend now serves the bulk open-change read in a degraded shape — real merge-request metadata so the review desk's NEEDS-REVIEW / RE-REVIEW trigger works, with the CI rollup and merge-state fields marked could-not-check per change so MERGE-NOW and FLIP stay withheld. This unblocks the whole `pr-review-desk` loop on a GitLab-backed repo without approximating any field the forge cannot assert.
- GitLab token custody now goes green on native Windows. The cold-mint probe (`deskroster preflight` / `deskboot`), the `ForgeFor` custody read, and the `desktoken --forge gitlab` rotate path all checked POSIX `0600` on the token file — but `os.FileMode`'s permission bits are synthetic on Windows (a normal file reads `0666`), so a `gitlab-<role>.token` locked down by an owner-only NTFS ACL was rejected as mode `666` and `token-mint-cold` stayed RED. Custody permission is now verified behind the OS boundary, exactly like roster loading: the unix `0600` test is unchanged, and on Windows the same owner-only ACL evaluation the roster owner check uses (owned by the invoking user, writable by no principal but the owner plus SYSTEM/Administrators) accepts an owner-only token file and still refuses any foreign write-capable principal. A `chmod 600` that reports `0600` without tightening the DACL is not rewarded (#667).
- The `ROUTE-HUMAN` bucket now states the Evidence-only lane explicitly: each risk-flagged
  member line carries its risk reason plus `Evidence-only (never flip-eligible)`, so a reader
  can see that a model may gather Evidence for the brief but never flips it.
- The forge-CLI ban ceiling falls 13 → 12 with `deskboard`'s permit row removed — `cmd/deskboard`
  now carries no `gh` literal at all.
- The generated Codex plugin manifest was two minor versions behind the Claude manifest it is
  generated from (`0.5.1` against `1.0.0`) and had been since the version bump merged —
  `harnessgen`'s committed-manifest and version-parity checks were red on `main` and nothing was
  running them. Regenerated, and the leg that catches the next one is wired.
- The model-capability floor no longer treats a DEAD dispatch's stamp as worse than no stamp at all. A `dispatched-*` stamp whose dispatch claim has been RELEASED now ages out: the cycle that applied it is over, so it attests nothing about the write in front of the floor, and the PR reads UNSTAMPED — the same proceed-with-NOTICE branch an unstamped PR has always taken. Previously a stamp left behind by a cycle that never posted its verdict refused every review verdict and every ready-flip on that PR forever, with no repair anyone could perform, while the identical PR carrying no stamp posted fine. Staleness is keyed on the claim being released rather than on a wall clock: a clock threshold short enough to release a dead stamp promptly also expires a live long-running dispatch out from under itself. Only a POSITIVE release ages a stamp out — a claim key that cannot be derived from the PR's link trailer, a presence read that failed, and a verb with no presence read at all are all could-not-check and leave the stamp exactly as it stood. `deskpost review` and `deskpost ready` resolve the claim; `deskflip` reports could-not-check, because the frozen Forge surface it writes through carries no ref-presence read (#486).
- The release `changelog-roll` retry loop placed `-c` after the `fetch` subcommand
  (`git fetch … -c http.extraheader=… origin main`), which git rejects at parse time
  and exits `129` on every retry, so the loop could never re-sync against a moving
  default branch. The re-sync step now orders the option before the subcommand
  (`git -c http.extraheader=… fetch … origin main`), matching the working tag-push
  and roll-push invocations elsewhere in the workflow (#312).
- The shipped `ask-decision` and `install` skill bodies no longer name Claude Code's plugin-root
  environment variable or its session-start hook event. The bodies name the neutral mechanism —
  a `<bundle>` placeholder for the installed bundle's directory, and "the session-start
  resident-rules injection channel" — and the Claude Code binding reference now carries the
  expansion for both, so a reader on any harness can still run the inbox and still knows which
  surface needs the documented Windows workaround.
- `deskboot` step 5 now logs the roster-preflight's OWN verdict when the envelope is
  red — the `preflight role=… RED n/5` summary and every `<check>=checked-failed: … →
  fix: …` remediation — instead of `firstLine`-ing the captured output, which always
  quoted the `assay-config: … configured=true` banner every desk tool prints first and
  left the failing check unknown in the pod log.
- `deskboot` — the token-mint step (step 6) is now forge-aware, closing the leftover boot half of the GitLab custody work. Previously it shelled `desktoken <role> --repo <slug>` with no forge, so a GitLab adopter's boot defaulted to the GitHub App mint path (demanding a `<role>-app.pem` / `apps.env` a GitLab deployment does not have) and could not finish even after #671 made its preflight envelope green. The step now resolves the repo's forge through the SAME single seam the preflight cold-mint check reads (`deskkit.ForgeKindForRepo`, wrapping the one resolver in the tree, #659: `ASSAY_REPO_FORGES` then the origin remote host), so the mint and the preflight that precedes it can never disagree. When the repo resolves to GitLab the mint passes `--forge gitlab`, taking desktoken's GitLab PAT custody path (rotate-on-mint against `gitlab-<role>.token`); an unresolved or GitHub forge mints with no flag, byte-for-byte the historical GitHub App behaviour. Fixes #676.
- `deskdispatch` now RELEASES the durable claim when the worktree-create step fails, instead of
  leaving it orphaned. An aborted `deskwt add` used to hold the claim it had just placed, wedging
  every later re-dispatch of that item behind a claim nobody was acting on. The failure report is
  reworded too: a worktree-create failure on a fresh dispatch is most often the brief's branch
  already existing (the brief is already delivered or in progress — a merged/open PR to look for),
  not the transient "fix the tree and re-run" fault the old message implied.
- `deskdispatch` now dispatches on a freshly adopted tree that carries **no**
  `tools/dispatch-claim.sh`: its claim-acquire step falls back to the pure-Go `deskclaim-ref`
  binary on PATH when the consumer script is absent, so a green-field (including
  native-Windows) adopter dispatches without the consumer script on disk and without a tribal
  `--claim-root`. A repo that still carries the script keeps using it unchanged, so a Go and a
  bash dispatcher collide on the same claim ref and never double-dispatch during the
  transition.
- `deskfile` now REFUSES with a named, actionable message (exit 5) when the target repo's
  configured forge (`ASSAY_REPO_FORGES`) is GitLab, instead of shelling `gh` against GitHub
  and surfacing a misleading GraphQL "Could not resolve to a Repository" that reads like a
  token or typo problem. Its issue operations (dedupe search, label-existence probe, issue
  create, issue comment) are GitHub-only until they are routed through the forge backend; the
  refusal names the forge and tells the operator to escalate rather than route around the
  gate with a bare `glab`/`gh` call (#687).
- `deskroster preflight` / `deskboot` — on a GitLab adopter the `app-scopes-vs-duties` check no longer reddens the boot envelope. That check reads a GitHub App installation grant, which a GitLab PAT does not have (a PAT's scopes are set by the group owner at provisioning and are not observable offline), so on a GitLab-forge repo it now reports a distinct **not-applicable** state instead of `could-not-check`. Not-applicable does NOT block the boot and is surfaced on its own summary line (never folded into the checked-clean tally), so a correctly provisioned GitLab fleet boots GREEN. The remediation now points at the GitLab group's Access Tokens page (confirm `api` / `write_repository`) and the citation is corrected from the stale GitHub-shaped `#571` to the live GitLab-forge issues. The GitHub path is unchanged — a real GitHub scope gap still evaluates and still fails the envelope.
- `deskroster preflight` — the `token-mint-cold` check is now forge-aware. On a GitLab adopter it no longer runs the GitHub App mint path (which demanded `<role>-app.pem` / `apps.env` a GitLab deployment does not have). The check resolves the repo's forge from `ASSAY_REPO_FORGES` (then the origin remote host) and, when it is GitLab, verifies the `desktoken --forge gitlab <role>` custody path READ-ONLY — the `gitlab-<role>.token` PAT is present (0600, non-empty) and `GITLAB_API_BASE` is set — WITHOUT rotating, so a boot probe never silently invalidates a live PAT. The remediation now names the GitLab custody path instead of GitHub App PEMs, and the `app-scopes-vs-duties` check no longer emits a GitHub installation-grant remediation for a GitLab credential.
- `deskroster preflight`'s `sibling-checkouts` check no longer assumes a flat `../<repo>` layout: a declared out-of-repo sibling is resolved through the configured roots (`DESK_ROOTS` / topology `<org>/<repo>` map) first, so a desk whose checkouts live elsewhere (e.g. a pod at `/workspace/<org>/<repo>`) still locates the sibling (#661).
- `deskwt add` and `role-init` now place a new worktree under the sanctioned prefix
  that is portable on the host OS: `/private/tmp/tracker-<name>` on POSIX, and
  `<repo-root>/.claude/worktrees/tracker-<name>` on Windows. Previously both always
  constructed the `/private/tmp` path, which on native Windows becomes a drive-rooted
  `\private\tmp\…` that fails the sanctioned-prefix check — so no desk worktree could be
  created and `deskboot` refused the shared checkout. Both prefixes were already in the
  allowlist; only the target selection was Unix-locked. The prefix guard is unchanged, so
  the isolation guarantee still holds on every platform (#656).
- `deskwt role-init` can now provision a worktree for EVERY desk role and stamp a GitLab commit identity. Previously `roleWorktreeConfig` mapped only `verifier`, so a GitHub adopter could `role-init` the verifier but nothing else; the four other bootable loops (`the-desk`, `worker-desk`, `pr-review-desk`, `intake-desk`) are now mapped by their App token role (`desk` / `worker` / `reviewer` / `issue-loop`), so `deskboot`'s "isolate first with `deskwt role-init --role <role>`" step works for all five (a drift guard keeps the map aligned to `deskkit.LoopTokenRoles`). Separately, on a GitLab-bound roster `role-init` no longer refuses outright: the service-account commit email embeds a group id and per-account suffix the roster cannot construct, so it now derives the worktree commit identity from the established GitLab two-identity mechanism — the trusted session / implementer address the deployment lists in `ASSAY_GITLAB_SESSION_EMAILS` (the same allowlist the commit-identity preflight accepts; a deployment committing AS the service account lists its provisioned noreply address there). It still NEVER falls back to the GitHub noreply shape for a GitLab account, and refuses loudly — naming `ASSAY_GITLAB_SESSION_EMAILS`, never a GitHub-shaped address — when no trusted GitLab commit address is configured (#677).
- `docs/adopting-assay-gitlab.md` now documents `GITLAB_API_BASE` — the REST v4 base
  `deskboot` / `deskroster preflight` and `desktoken --forge gitlab <role>` require, with no
  fallback. Covers where/how to set it (a plain environment variable, never a `roster.env`
  key), the expected value shape for self-hosted vs. gitlab.com SaaS, and a short trade-off
  note on whether it should instead be a `roster.env` key.
- `scripts/build-windows.ps1` now parses and runs under Windows PowerShell 5.1
  (`powershell.exe`), not only `pwsh` 7+. A `>>>` inside a `Write-Host` string
  that 5.1 lexes as a redirection operator is gone, and every em-dash (which
  5.1's default non-UTF-8 encoding mangled in error/log strings) is now an ASCII
  `--`, so the documented Windows rebuild path works on a host that has only
  `powershell.exe` (#678).
- `statusgen init --forge gitlab` no longer pretends CI is finished the moment `.gitlab-ci.yml` exists. The scaffolded GitLab jobs are untagged, and a self-hosted instance has no hosted `ubuntu-latest` equivalent — so where the fleet's runners are tagged (`run_untagged = false`, a common default) the pipeline fires but every job sits in `stuck_pending_no_matching_runners` and never starts. The template now carries a header comment naming the runner precondition and a commented `ADOPTER: runner` `tags:` placeholder under each job (mirroring the house GitHub templates; no instance-local tag is hardcoded), and the GitLab-only next-steps note tells the human that a job must LEAVE `pending` (reach `running`, or a terminal non-stuck failure) before CI counts as installed — a `pending` job is could-not-check, not success. Assay still registers no runner: that is instance-admin work, an explicit non-goal.
- `statusgen`'s DORA-timing recorder no longer shells out to the `gh` CLI. Its three reads —
  main-branch `actions/runs`, closed `pulls`, and a PR's `pulls/{n}/commits` — now go straight to
  GitHub's REST API over `net/http`, authenticated from `GH_TOKEN` (falling back to
  `GITHUB_TOKEN`), with the same pagination and the same field selection as before. On any runner
  image without `gh` on PATH every read had been failing with
  `exec: "gh": executable file not found in $PATH`; because the recorder is fail-open by design
  nothing went red, and `docs/streams/.dora-timing.jsonl` simply never accrued a record — the two
  DORA numbers that can only be answered from a recorded series (`change_lead_time`,
  `time_to_restore`) had no series to answer from. The failure shape is unchanged (a failed read
  still records NOTHING, never fabricates an interval, and never fails the record job); the
  `.dora-timing.jsonl` schema is unchanged (#699).
- `verifyloop plan` now fails safe on risk: any brief with `gate: human` or any risk answer
  `yes` (irreversible included) is bucketed under `awaiting-human / ROUTE-HUMAN`, never printed
  as a DISPATCH candidate. Previously an `irreversible: yes` brief whose gate was `model` was
  routed to a dispatchable tier and read as dispatchable — a gate that failed open on its most
  serious input. The decision is now made in two independent places (the tier policy routes
  irreversible to the human first; the queue classifier reads the brief's own gate/risk
  frontmatter), and the gate-value comparison is normalized so a re-cased or qualified `human`
  value (`Human`, `human — <qualifier>`) still counts as the human gate.
- corroborate: the decision-gate anchor's evidence string and comments no longer cite a tracker-internal reference; behaviour unchanged.

### Changed
- A DORA metric with no recorded history renders `{"computed": false, "state":
  "could-not-check", "value": null}` with `needs` naming what is missing — never a fabricated
  `0`. The lead-time and restore metrics read the recorded timing log; the change-failure
  number is an explicitly unlinked proxy over completed work, recorded reversions and open
  defect records, and is always `partial` because it lacks the change-to-incident linkage the
  canonical metric is defined over. A window with no completed work has no denominator and is
  `could-not-check`, not a 0% failure rate.
- Bumped the plugin to `1.0.0`. `paired-versions.yaml` stays pinned to the last real umbrella release (statusgen/desk-tools `v0.26.0`, real harvested `sha256`s) so `check-paired-versions.sh` stays green and no gate is touched: the v1.0.0 re-pin (bump both tags, harvest the per-platform `sha256`s from the published `v1.0.0` checksums.txt) is the cut-release skill's job AFTER the human pushes the `v1.0.0` tag — a hash for an uncut tag is never hand-typed.
- Migrated the `examples/adopter-scaffold` fixture to exercise the migration end to end: added its `graph-repos.yaml` alias registry, `releases/{v0.28.0,v1.0.0}.yaml` composition manifests, and an umbrella-pinned `.assay-versions` (source umbrella v0.28.0).
- The channel-conformance sweep now registers `qualgen` as a released pack tool, so a
  build-from-source or `go run` of it on an adopter surface reddens the sweep. The producing
  repository keeps self-hosting `qualgen` from source — the seam the pack contract names.
- The floor's trusted-applier set now admits the **reviewer** App alongside the desk App, and `deskdispatch --kit review` mints its stamp under the reviewer identity. The floor's real requirement is that the identity that DISPATCHED a session is the identity that stamped it — never the session itself — and the review lane broke that from the other end: `pr-review-desk` dispatches its own reviewers, so a correctly-run review could never carry a stamp the floor would accept, no matter what anyone did. Dispatcher and applier are the same identity again for that lane. The accepted cost is stated: a second App identity holds label-write on the `dispatched-*` labels. Nothing else widens — a worker App's, a human's, or any unbound identity's stamp still reads unreadable, and the strength semantics are untouched (#486).
- The outbound send path fails closed: a deterministic refusal is terminal and the gate is
  never consulted after it, while the gate's own default is `hold-for-human`, so a disabled
  valve (`DESK_DECIDE_DISABLED=1`), a spent budget, a timeout, an advisor error, or an
  injected/malformed answer all resolve to a hold rather than an ungated send.
- The recorder's `DEGRADED` line now names the HTTP status that actually failed
  (`restore-episode read: HTTP 401: Bad credentials`) and points at REST reachability and the
  token env vars, instead of telling the operator to "investigate gh availability" — a cause that
  can no longer apply. Which of the two reads could-not-check is still named separately, so a
  partial failure is not smeared into a total one (#699).
- The retained `--dora` / `--trend` grouped back-compat aliases are untouched; `--dora-json`
  is a new output mode over the same retained computation, not a revival of the removed
  standalone DORA CLI.
- `component-model.md` §3 now links to `components/KEYS.md` for the key catalogue instead of carrying an inline table.
- `deskboard` now reaches every forge read through the typed `Forge` seam: its last
  `gh` reads — PR search, commit-history listing, single-commit read, the combined-status
  total, the workflow-directory listing, plus the reviews / changed-files / compare /
  label-events / comments / contents / raw-diff / repo-visibility / verify-gate-issue reads —
  are migrated onto typed ops, and the `gh` choke point (`board.go`'s `ghRun`) is deleted.
- `deskdispatch` session-scopes the worktree DIR name it derives (`<item>-<session>`, from
  `$DESK_SESSION` / `$CLAUDE_SESSION_ID`, mirroring `deskwt role-init`'s `tracker-<prefix>-<sess>`),
  so a foreign session's leftover canonical dir (`/private/tmp/tracker-<item>`) can no longer
  dead-end an otherwise-valid dispatch with `deskwt add … target already exists`. The branch and
  claim key stay deterministic — they are the deliverable's cross-session identity; only the local
  scratch dir gains the suffix. With no resolvable session the name falls back to the bare
  item-derived form.
- `docs/adopting-assay-gitlab.md` gains a "Runners and job tags" section (previously zero mentions of runners), and the `assay:install` "Prove the install" step now requires, on a GitLab forge, that the first pipeline's job leaves `pending` before the install is called proven (#688).
- `statusgen --lint` gains two drive-plan honesty rules over `docs/roadmap/drives/*.md`: `drive-region-drift` PROBLEMs a plan file whose snapshot region has been hand-edited away from a fresh render, and `drive-without-manifest` PROBLEMs a plan file with no `<slug>.yaml` manifest beside it. Both are `--lint`-only, offline, and inert when no `docs/roadmap/drives/` directory exists. A top-level `.md` under `docs/roadmap/drives/` is now recognised as a drive-plan narrative file (governed by these rules) rather than warned about as a mistyped manifest.
- `statusgen`'s ladder/autonomy day-file reader tolerates the additive `opmetrics/2` schema with no metric change (proven by a v2-fixture schema-tolerance test).

## v0.28.0 — 2026-09-08

### Added
- **Acceptance/ruling citation corroboration** (`statusgen --corroborate`) — a second
  lane alongside the `human:<name>` stamp check. It reads FREE-PROSE and commit-message
  claims that a configured human ACCEPTED or RULED ON something (e.g. "&lt;name&gt; accepted
  this on #1583", "per &lt;name&gt;'s ruling") and requires a comment or review authored by
  that human on the cited issue/PR. An unlinked claim, or one with no such artifact on the
  cited issue/PR, is `MISSING-CORROBORATION` — the same non-zero exit the stamp lane uses.
  Detection is anchored on names an adopter has declared human in `ASSAY_HUMAN_LOGIN_MAP`,
  so it hardcodes no name and stays inert when unconfigured; the corroboration read is a
  live, possibly cross-repo lookup of the cited artifact, which is why it lives on the
  network-capable `--corroborate` verb rather than the offline `--lint` gate. A fetch that
  cannot complete (network, token, rate-limit, transient 5xx) is reported `COULD-NOT-CHECK`
  and does NOT fail the gate — an absence the check never observed is not rounded down to a
  fabricated `MISSING`; an observed HTTP 404 (the cited artifact genuinely does not exist)
  still reports `MISSING`, so a bogus ref stays fail-closed. Closes a laundering surface one
  over from the register stamp: a fabricated human acceptance written into a durable
  governance artifact (a runbook, a brief, a commit record) that no human artifact stands
  behind. Logic in `statusgen/citationcorroborate.go`.
- **Activate the staged CI workflows.** Promote two workflows from `ci/staged-workflows/`
  into `.github/workflows/`: the `evidence-automerge` leg (with the tolerant
  auto-merge-not-allowed skip, #579) and the `windows-ci-leg` — the first check in this repo
  to run on a Windows runner, asserting `statusgen --lint` exits 0 and an offline `--version`
  smoke passes on `windows-latest` (windows-port/04). The Windows leg runs against an LF
  checkout via the repo `.gitattributes` (#584). (#583)
- **DECISIONS register** — a fourth append-only register of design-decision records under
  `docs/streams/decisions/<slug>.md` with `DR-<slug>` ids: what was decided, the
  alternatives ruled out, the consequences accepted, an ordered `consequence` severity
  axis, and a `human:<name>` `decided-by` stamp (the design-approval authority). Specified
  in `spec/registers-v1.md` §7; a brief cites its record with the new `design:`
  `brief-v1` frontmatter key.
- **Design-approval gate** — a risk-gated brief (`gate: human`, or any `risk` answer
  `yes`) may no longer move to `in-progress` until it cites an approved
  **design-decision record**, so a wrong *design* is caught at authoring rather than only
  when the finished diff reaches the review gate. It is a precondition on the
  `todo → in-progress` edge, not a sixth lifecycle state. Specified in
  `spec/lifecycle-v1.md` §4.4; enforced by `statusgen --lint` (`designgate.go`),
  three-state (an unreadable register is `could-not-check`, never a silent pass).
- **Split-flag conservation gate** — splitting a brief may no longer silently
  DOWNGRADE its risk. A child brief must carry at least as strict a `gate` and at
  least as high each of the four canonical `risk` answers as the brief it was
  split from (gate = the stricter of parent and child; risk = MAX per key). A
  human-gated, irreversible parent split into a `gate: model`, everything-`no`
  shard — the move where a human gate is most likely to evaporate, because risk is
  a property of what the change does, not the size of the diff — is now a hard
  `statusgen --lint` PROBLEM (`splitflags.go`). The parentage is read from
  whichever signal is present: the numeric-stem convention (`02a` is a shard of
  `02`; when `02` was retired in the same change the strictest sibling shard sets
  the floor, so a faithful `02b` catches a downgraded `02a`/`02c`), or a new
  optional `split-from: <stream>/<NN>` `brief-v1` frontmatter key for splits whose
  lineage is not in the numbering (across streams, or renumbered). Fails safe
  toward more gating; three-state (an unresolved `split-from` is a
  `could-not-check` NOTICE, never a silent pass).
- **Threat model made mandatory for risk-gated briefs** — the `mistake-proofing.md` B5
  pre-mortem is now REQUIRED and RECORDED on a risk-gated brief, each failure mode mapped
  to the Verify row that catches it (`spec/brief-v1.md` §4.7, `docs/brief-rules.md`
  rule 49), wired into the brief's existing single-point-of-failure note rather than a
  second ceremony.
- **Validation named as the third activity** — `docs/validation.md` defines validation as
  distinct from review and verification (did the change achieve the purpose the
  requirement existed for, in its setting), anchored to the REQUIREMENTS register's
  acceptance criteria and to brief-rule 43's dereferencing row as its mechanical floor,
  honest about the intended-use part that remains the adopter's. This closes the gap
  `docs/iso9001-mapping.md` row 8.3.4 named in the repo's own words.
- **Windows-runtime hash-verify smoke for the PowerShell bootstrap (windows-port/03, decision
  #508).** A new `scripts/windows-bootstrap-hashcheck-smoke.ps1` exercises
  `scripts/bootstrap-windows.ps1`'s sha256 verify on `windows-latest`: a tampered checksum must
  REFUSE (throws on the mismatch, nothing installed), the pinned checksum installs, and a
  check-removed copy installs the tampered asset — proving the refusal is non-vacuous. It runs in
  a dedicated `windows-bootstrap-smoke` job (staged in `ci/staged-workflows/windows-ci-leg.yml`,
  maintainer-promoted), kept SEPARATE from the offline `windows-smoke` job because the bootstrap
  downloads a release asset — the sanctioned decision-#508 online exception, so `windows-smoke`'s
  offline envelope stays intact. Recorded as Verify row 8 on windows-port/03. (#595)
- **`--explain` on `deskpr`, `deskpost` and `deskreply`** — on a secret-scan refusal, an
  optional stderr line names the rule id and the 1-based line of the first offending span,
  its length, and a REDACTED shape (first two + last two characters, a character-class
  summary), so a refused caller can act on the first round instead of guessing which span
  tripped it. The line NEVER prints the offending span — the refusal must not become the
  leak — and without the flag the refusal message is byte-for-byte unchanged.
- **`deskaudit recover`** — the sanctioned, non-destructive corruption recovery for the shared
  audit log. A single malformed line (a partial append from `kill -9`, a disk-full write, a
  sync-tool rewrite) makes every desk tool refuse. Moving the whole file aside cleared the
  corruption but RESET load-bearing state — budgets returned to full and the idempotency store
  forgot every prior write, so re-runs posted duplicates. `deskaudit recover`
  (`deskkit.RecoverCorruptAudit`) instead quarantines only the malformed line into an
  `audit.jsonl.corrupt-<ts>` sidecar and carries every good entry forward under the audit
  flock, so the rate-limit counter and the idempotency store survive the recovery. The
  corruption-refusal messages now point at this verb instead of the destructive move.
- **`tools/claimguard`** — a heuristic lint for the "named third-party product,
  no citation at all" shape of unresolved outward claim (a CamelCase or
  ALL-CAPS-acronym-shaped token with no resolving URL within a token window,
  and no markdown-link anchor around it). Complements a separate
  link-resolution check, which covers a *present-but-dead* citation; this one
  covers the case where nothing was ever cited to resolve in the first place.
  Standalone Go module (`go run ./tools/claimguard <path>...`), hermetic
  (no network calls), unit-tested with a fail-first case and a passing case.
  Not wired into any CI gate yet — see `tools/claimguard/README.md` for scope
  and known limitations.
- A platform-independent roster-ACL decision function (`evaluateRosterACL`) with unit tests that inject ACL data, so the Windows security logic is exercised on every CI platform even though there is no Windows CI runner.
- Desk inbox: `deskfile new --to <role>` addresses an issue to a desk (stamps a
  `to:<role>` label, reusing the `--raised-by` role vocabulary). The addressee's own sweep
  leads with it — `fanoutloop plan` emits `to:worker` items first, and `issueboard issues`
  renders addressed items `ADDRESSED→<role>` in their own priority band.
- Four operations join the frozen `Forge` seam, each landing with the call sites that consume it (the freeze rule's same-change requirement) and with a contract case per backend: `ApplyLabels` reconciles a change's labels declaratively — ensure these exist, drop the stale members of these FAMILIES, apply these — so no caller needs a label-listing operation of its own; `ListLabelEvents` reads label APPLICATIONS with the actor that made each one (the applier is what separates a dispatcher's attestation from a self-applied stamp); `ListComments` and `EditComment` carry the find-or-create comment upsert. `PostComment` now returns a reference to the comment it created, so a write is answerable without a follow-up read.
- Four read operations join the frozen `Forge` seam, each landing with the call sites that consume it (the freeze rule's same-change requirement) and a contract case per backend: `ListOpenChanges` reads a repository's open changes with their CI status-check rollup in ONE bounded page, reporting whether the population was truncated at the cap in-band rather than as a confident count over an unknown remainder; `ListOpenIssues` reads open issues as classification summaries (and a forge that serves issues and changes from one number sequence filters the changes out itself); and `PRTrustEvents`/`IssueTrustEvents` read the trust gate's content events — the body-edit time, each comment/review's author identity and edit time, and whether the single bounded page overflowed. `Issue` grows a `Title` for the closed-issue title read. On a forge whose CI-rollup shape or GraphQL content-edit / numeric-actor-id semantics do not map one-to-one, each op returns could-not-check naming the gap rather than an approximation (the fail-closed direction on a trust gate).
- Harness-portability code de-house: the stream's tool and packaging deliverables now live in
  the public tree — three self-contained Go modules (`tools/harnessgen`, `tools/harnesslint`,
  `tools/plugindrift`), the bundle's provenance and packaging (`plugins/assay/SOURCES.yaml`,
  `PARITY.md`, `RELEASE-NOTES.md`, `.codex-plugin/plugin.json`, the generated `codex/` and
  `cursor/` packaging, `resident-rules.md` and its generated payload), the two capability
  matrices under `docs/research/`, and the Codex smoke protocol. The `harnessgen`/`harnesslint`
  generators are discovered by CI's existing Go-module walk, so "Assay runs natively on Codex and
  Cursor" is now checkable in this repository.
- New harness-neutral reference `plugins/assay/references/desk-shell.md` — the shell and transport
  mechanics every desk role re-derives (one call/one chain, workspace isolation and
  content-triggered write-guard refusals, per-commit inline commit identity, loop/session marker
  export, authenticated push/fetch transport, and role/repo coverage), stated as mechanism + signal
  + correct form with no house-specific values. Each of the six desk-role skill bodies (`the-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `intake-desk`, `pr-shepherd`) now points at it.
- Review kit (`review-prompt.md`): a "claim is false" finding now requires the reviewer to
  sweep the whole diff (and, where cheap, the repository) for every other instance of the
  same claim before signing off the fix, instead of checking only the cited file:line.
- The apps-installer stream README row 08 flips to `implemented` and points at the new spec.
- The bulk open-change read requests the rollup CONTEXTS without the `checkSuite { workflowRun }` sub-selection the built-in field hardcodes, so it no longer depends on an `actions:read` scope the board is not guaranteed — a scope narrowing carried over from the read it replaced.
- Two operations join the frozen `Forge` seam, both landing with `deskevidence` as the call site that consumes them (the freeze rule's same-change requirement) and with a contract case per backend: `WriteFile` writes a file's whole content on a branch, and `ReadFile` reads a file's content at a ref. `WriteFile` is deliberately fat — it folds the idempotency read (reporting a `Changed` flag rather than committing byte-identical content), the append-only shrink guard (refused post-fetch against the branch's current row count), and a default-branch writability probe whose sentinel routes the write onto a side branch with inline branch creation (`start_branch`) rather than a separate ref-create op.
- Windows CI leg (`windows-port/04`): a staged `windows-latest` job
  (`ci/staged-workflows/windows-ci-leg.yml`) that installs Go, builds `statusgen.exe`,
  asserts `statusgen --lint` exits 0 on Windows, and runs an OFFLINE `--version` desk-verb
  smoke — the first check in the repo to run on a Windows runner. The native windows/arm64
  smoke is held BLOCKED pending a `windows-11-arm` runner (never inferred from the amd64
  result), and a `workflow_dispatch` `failfirst` input demonstrates the leg reddens on a
  broken input. Staged for maintainer promotion into `.github/workflows/` (no App holds
  workflow-push permission).
- Windows adopter walkthrough (`windows-port/05`): `docs/adopting-assay.md` gains a
  **Windows adopters** section — the native-Windows arm of the install step every scenario
  references. It documents the pinned, sha256-verify-or-refuse install path (the
  `scripts/bootstrap-windows.ps1` first-install bootstrap + the Go-native `deskinstall`
  command), the `statusgen-windows-<arch>.exe` / `desk-tools-windows-<arch>.tar.gz` pins for
  `.assay-versions`, and the native-not-WSL claim (WSL noted only as a local-dev fallback).
- Windows install path (`windows-port/03`): a Go-native `deskinstall` command that
  mirrors the Unix acquire→verify→place flow — detects `windows-amd64`/`windows-arm64`,
  resolves the pinned tag + per-platform sha256 from `paired-versions.yaml` (never a
  floating ref), downloads the `statusgen-windows-<arch>.exe` and
  `desk-tools-windows-<arch>.tar.gz` assets, and **verifies each sha256, refusing on any
  mismatch before anything is placed** (nothing is installed on a bad hash).
- `ASSAY_GITLAB_SESSION_EMAILS` — an exact-match, roster-only allowlist of the GitLab
  commit-author addresses accepted as a session / implementer identity. It is additive
  and fail-closed: unset means the service-account noreply shape stays the only accepted
  GitLab commit email (unchanged behaviour), an unlisted ordinary address still fails,
  the cross-forge rejection is unchanged, and it is never consulted on a GitHub identity
  (the bot-USER-id guarantee is untouched). Echoed in the effective-config run output.
- `PullRequest` grows `Mergeable` (a three-value answer, never a bool — "not computed yet" must not collapse into either verdict), `Labels`, `URL` and `HeadRef`; the two CI-rollup entry types grow their recency stamps, so the latest-run-per-check-name reduction orders both forges' rollups by the same kind of fact rather than by list position.
- `RequiredStatusChecks(repo, branch)` joins the frozen `Forge` seam — the twentieth operation
  — landing with its one consuming call site (`deskflip`'s checks-green condition) per the
  freeze rule, with a contract case per backend. GitHub reads
  `branches/<branch>/protection/required_status_checks` (404 = nothing required = empty set,
  every other non-2xx = could-not-check), unioning the legacy `contexts` and the newer
  `checks[].context`; GitLab reads the all-tier `only_allow_merge_if_pipeline_succeeds`
  pipeline-gating setting. `PullRequest` grows `BaseRef` (the target branch) to feed it.
- `deskack "<restatement>"` — a new verb that prints a desk's receipt line
  (`ack <role>@<repo-short>: <restatement>`) for a human-typed message and appends a
  `{ts, role, repo, restatement}` record to the session's roster beacon. Refuses a
  restatement over twelve words.
- `deskdispatch --dry-run --worktree <path>` renders the previewed prompt against an
  operator-stated home worktree that already exists, at both placeholder sites, instead of
  the not-yet-known placeholder — retiring the by-hand substitution operators ran over
  dry-run prompt batches. The path is validated first, all three checks fail-closed
  (exit 5): it resolves under a sanctioned worktree prefix, it IS a registered git worktree
  of the item's own repo, and it is not the shared checkout. The flag is refused (exit 5) on
  a real dispatch, where the home is `deskwt`'s to name, and a verified path is echoed on the
  PLAN banner as `operator-supplied, verified` so a transcript shows it was checked, not guessed.
- `deskkit.BriefRiskFromBody(repo, body)` resolves the `Brief:` trailer to a brief file under
  the configured stream roots and reads its `gate:`/`risk:` frontmatter, returning the owning
  brief id and whether the brief's own declaration risk-classes the PR. `PullRequest` grows a
  `Body` field to feed it. (A later change can consolidate the local trailer splitter onto the
  shared canonicaliser once that lands.)
- `deskkit.RepresentedBriefs` / `BriefRepresentedPR` / `RepresentedBriefSet` reconcile a repo's pull-request list against brief ids by each PR's `Brief:` trailer (never a branch name), counting only OPEN and MERGED PRs; `ParsePRList` and the `PRRef` shape parse the transport's JSON. The reconciliation transports stay injectable and nil by default (the offline reference build performs no forge read, as the orphan sweep does), so the closed forge surface stays closed and the live read is wired at the human-gated cutover through a typed op.
- `desksupervise tick` now reconciles every in-flight dispatch claim's ELIGIBILITY before the
  liveness step: a run whose item became ineligible mid-run is STOPPED within one observer
  interval. Terminal cases — the item is FINISHED and no other party owns its claim (PR merged or
  closed, board row at `implemented`/`verified`/`done`, or the claim already released) — also
  release the claim for re-dispatch. Held cases — a human or another holder owns the next move
  (the claim reassigned to a different live holder, a `blocked` board row, a
  SUPERSEDED/RESOLVED-ELSEWHERE disposition, or a `needs-decision`/`question` label) — stop the
  run WITHOUT releasing it, so an unconditional ref delete can never re-free an item its new
  holder is working. A reconcile read that could-not-check keeps the run and retries next tick.
  This turns "a merged or closed PR is DONE, stop" from a rule a worker had to remember into a
  mechanical backstop.
- `desktoken` supports an optional **role→App binding** (`<ROLE>_APP=<app-name>` in
  `apps.env` or the environment), so a deployment can run fewer Apps than desk roles — the
  recommended two-App tier is one key that reads and one that writes — without symlinking or
  copying keys. The App-name is the stem for the role's PEM file, App ID and install ID keys;
  absent, it defaults to `<role>-app`, byte-identical to the previous layout. `desktoken
  --version` now prints the effective `bindings=` line, `role=app-name` per role.
- `docs/consuming-the-desk-go-module.md` — the pin contract for consumers of the `github.com/medici-finance/assay/tools/desk` submodule: it is versioned independently of the repository's bare `vX.Y.Z` tags, so those tags do not resolve (`invalid version: unknown revision tools/desk/vX.Y.Z`); pin a merged-commit pseudo-version instead, whose integrity record is the consumer's `go.sum`.
- `docs/streams/apps-installer/solo-identity.md` — the spec of record for **Solo**, the zero-App pilot tier: every desk role runs on the operator's own user token with the role as a *label* rather than an identity. It fixes the switch (`ASSAY_SOLO_LOGIN=<login>`, with the rejected alternatives recorded), a per-verb behaviour table (`desktoken`/`deskpost`/`deskpr`/`deskfile`/`deskevidence`/`deskflip`: run-as-user, run-with-role-label, or refuse), the trust gate under Solo (empty `ASSAY_TRUSTED_BOT_SLUGS`, operator admitted as human and bless authority), the two preflight checks (`app-scopes-vs-duties`, cold-mint) that are could-not-check by construction with the boot line that says so, the refusals (a repo whose ruleset requires a bot identity; a review of the operator's own pull request downgrading to `COMMENT`), and the exit criteria that tell an operator they have outgrown Solo. The `## Human decision` section carries the three options and cites the standing ruling on #467 (adopt as specified).
- `docs/streams/composability/` — a new stream that turns Assay's installable units into declared components. `component-model.md` is the draft of record (candidate `spec/component-v1.md`): one `component.yaml` per unit declaring the keys it provides and injects and its apply steps, each paired with an inverse inside the system boundary or a ledger line plus compensation outside it; a resolve/cycle lint; per-component activation so a rejected extension key deactivates only the components that inject it while the trust surface stays fail-closed; an install ledger and a `disable` verb; a desired-state record with a reconcile engine behind `deskmigrate` / `upgrade-assay`; and the harness as an exclusively-bound key. Six briefs in four waves (00 manifests + lint → 01 activation, 02 ledger + inverses, 04 harness key → 03 reconcile engine → 05 promote to `spec/`). Source model: arXiv 2608.25512, *A Programming Paradigm for Spatiotemporal Composability*, adopted as a discipline for a git-tree + forge substrate rather than as its runtime. Tracked on #624.
- `issueboard issues --to <role>` — a per-desk inbox view showing only that role's
  addressed items. Un-flagged, a `to:<role>` item is held out of the un-briefed
  (CREATE-PLACEHOLDER) work and, once aged past `--sla-days` with no comment from that
  role's App, ESCALATES — so an unread inbox surfaces without the addressee's cooperation.
- `plugins/assay/scripts/pr-monitor.sh` — a durable, stateful open-PR monitor for the review
  desk: per-repo head-sha / draft-state / state / merge-state baselines, silent seed on first
  sight, and one machine-parsable `PR-EVENT: <slug>#<num> <kind> <old> -> <new>` line per change
  (`opened | pushed | draft-flip | state | merge-state | closed`). It paces its reads
  (`ASSAY_MONITOR_PACE_SECONDS`, default 2) and caps a cycle (`ASSAY_MONITOR_MAX_REPOS_PER_CYCLE`,
  default 0 = all), and ends a cycle on a secondary-rate-limit / 429 signature without further
  calls, so the watcher can no longer become the tight-loop poll that trips the forge's limit.
- `pr-review-desk` gains three review rules that existed only in a private downstream copy of this same body, carried here so the canonical copy is the complete one: the **round cap + arbiter packet** (default N = 3 verdict→fix→re-review rounds on the SAME finding class on one PR; on round N+1 the reviewer files a `needs-decision` carrying a one-row-per-disputed-finding packet instead of another verdict, plus the recurrence-promotion rule for a finding raised three or more times across separate PRs); the **steady-state gating** check (a PR editing a skill body, a guardrail/hook, or a behavior-carrying tool-version pin bump should append a line to the repo's `.assay-warmup`, where that file exists); and the **decision-drift pass** (check the diff against three bounded records — the owning brief's Context, a findings-register entry naming the touched surface, a ruling on the PR's linked issues — and raise a contradiction as an ordinary finding citing the record by link).
- `scripts/bootstrap-windows.ps1`: a minimal PowerShell first-install bootstrap that
  fetches only `statusgen` and hash-verifies it before executing, keeping the
  security-critical hash-verify in one tested Go implementation.
- `statusgen --lint` now NOTICEs a `verified`/`done` brief whose `Verified` cell credits a runner other than the actor who ran a strict majority of its own `## Evidence` rows — the drift a legitimate Verify-table RE-RUN leaves behind when the register cell keeps naming the original verifier while the re-run stamped the Evidence with the actor who actually re-ran the rows. It names both actors and the row count and asks that the cell be updated to match the Evidence. Offline and tree-only; NOTICE severity, so it changes no exit code and weakens no existing verification-integrity assertion (F-verify-self-attest family).
- `statusgen`: docs for the DevLake hybrid metrics split (statusgen/11) — the one-page metric map (`docs/streams/statusgen/metric-map-11.md`) classifying every commodity + harvest metric into DevLake | ours | dropped, and the staged (not-applied) DevLake deployment spec + runbook (`docs/streams/statusgen/devlake/`) targeting the platform k8s cluster.
- `tools/desk/askassay/export_external_test.go` — an external-test-package guard that binds the consumer-facing identifiers of the ask-pane numbers-rule layer (the answer type, its `Render` entry point, the registry lookup, the answer/stamp constructors, the state constants), so an accidental un-export breaks this repository's own build rather than a downstream consumer's.
- `tools/desk/layout_test.go` — a layout guard that fails if the layer is moved back under an `internal/` path, which no compile error would catch because it keeps the module's own build green while silently breaking every external importer.
- `verify-desk` gains a **"Public repo (PR-required main) — Evidence lands by PR"** subsection under §Landing: the landing shape for a repo whose `main` refuses a direct App push (a branch ruleset requiring a PR, an approving review that is not the last pusher, and a named status check, with the verifier App on no bypass list). The direct-to-main `deskevidence` carve-out is NOT widened — on such a repo this desk does not push `main` at all. The subsection states the precondition (a recorded human ruling naming the repo and this shape), the server-side branch cut from the fetched remote head, `deskevidence` aimed at that branch one file per invocation with the Evidence rows and the `implemented → verified` flip riding the SAME PR, the author-identity requirement for the draft PR (an Evidence-landing lane keys on the verifier App having authored it, so a PR verb that mints a fixed role's token is the wrong tool), the self-contained-body rule for public repos, and the hand-off — the review desk owns the verdict and the ready-flip, this desk never approves, flips ready or merges. Land-as-each-verdict-arrives still applies: the PR replaces the push, not the cadence.
- `verify-desk` §Boot gains a step before the first queue read: **export the stream-root map** (`DESK_ROOTS="<owner>/<repo>=<path>,…"`, one entry per checkout carrying `docs/streams/`; the project layer states the value). The shipped compiled defaults are a placeholder topology, so with the variable unset a queue read either refuses outright or covers only the placeholder's roots — either way the cross-repo merge aborts and whole repos never appear in the Awaiting queue. Both outcomes are could-not-check, never an empty queue, and the step says to prove the map by reading back the `roots` array the queue prints.
- `worker-desk`: a cockpit-aware, PATH-detected variant of the per-item worktree-create step for
  fanout dispatch — `supacode repo worktree-new` / `herdr worktree create` / plain
  `git worktree add` (fallback). Additive and never required: selection is by command presence on
  PATH, only the worktree-create step changes, and the plain `git worktree add` path stays the
  default with no cockpit installed.

### Fixed
- **A PR whose sensitivity is declared in its brief frontmatter (`gate: human`, or any
  `risk:` flag `yes`) is now risk-classed by both `deskboard` and `deskflip`, even when its
  changed paths hit no compiled trigger.** Previously the owning brief never resolved, so the
  board marked such a PR `FLIP` and `deskflip` required no `Security-Review: pass` — a
  human-gated, sensitive-data change was flippable with no security verdict. `deskflip`'s
  security lane now REFUSES the ready flip on such a PR until a reviewer App posts
  `Security-Review: pass` at the current head (absence is never a pass).
- **A trailer-less App-authored PR now fails closed at the flip gate.** A PR carrying no
  `Brief:` / `Issue:` link trailer used to be risk-classed by its path, surface-label and
  visibility terms alone, so a risk-bearing change with no trailer could be marked FLIP with no
  `Security-Review` verdict. `deskpr` makes the trailer MANDATORY for App-authored PRs, so a
  trailer-less PR whose author is a role App (worker / desk / verifier / reviewer, per the
  roster) is an anomaly by construction. `deskflip` and `deskboard` now treat it as
  RiskClassed + Unverifiable: the flip REFUSES until a `Security-Review: pass` stands at head,
  and the board row says why (`trailer absent on App-authored PR`). A trailer-less
  HUMAN-authored PR keeps today's path / label / visibility behaviour — no new cost on
  maintainer PRs. The author test reuses the roster's own App-slug resolution (never a
  hard-coded login). (#587)
- **Audit budget no longer escapes through variant tool keys.** The per-tool outward-write
  budget and the audit trail are now keyed by a single CANONICAL tool identity, resolved the
  same way wherever `audit.jsonl` is read or written (`deskkit`, `audittoolkey.go`). A binary
  invoked under a variant name — a test build (`deskpost.test`), a locally built or renamed
  copy (`deskpr-322`, `deskpr-bin`), a `go run` binary — previously wrote audit lines under
  that variant spelling and so earned a fresh, uncounted budget while splitting the audit
  trail. Variant spellings now collapse onto their tool's one budget, and a key that resolves
  to no known tool is a loud `Unverifiable` at the write gate rather than a silent new bucket.
- **Cross-repo triage evidence now binds to the remote, not a bare sibling checkout** —
  `intake-desk`'s shared rules add an explicit clause: a triage/verification claim about
  another repo's current state must be resolved against that repo's remote (a forge read, or
  a sibling working copy fetched and SHA-confirmed current *this cycle*), never a local
  checkout read as-is. A stale sibling tree drifts arbitrarily far behind with no visible
  signal and a grep against it returns confident, precise, wrong evidence.
- **Next-up rows with a backtick-bracket-led title no longer keep a dead README link.** When a
  stream README status-table row's title began with a backticked bracketed tag (e.g.
  `` [`[assay]` …](./brief-…md) ``), statusgen copied the row into `STATUS.md`'s Next-up table
  with the stream-relative `./brief-…` target intact — dead from the repo root and reddening any
  snapshot that copied it. The title-cell unwrap used a `\[([^\]]+)\]\(…\)` regexp that stopped at
  the first inner `]`, so the outer link never unwrapped. It now unwraps with a bracket-DEPTH walk
  (matching `[`↔`]` and `(`↔`)` by depth), so a title whose link text itself contains brackets
  strips to a bare title like every other row. (#591)
- **The verifier Evidence log no longer serially re-conflicts.** Every verifier Evidence PR
  appends one self-contained JSON row to `docs/streams/verify-outcomes.jsonl`, so with several
  open at once each landing made the rest CONFLICTING on that append-only file (add/add), stalling
  the auto-merge lane and forcing a serial merge-main into each survivor. `.gitattributes` now
  marks the log `merge=union`, so a base-side append and a branch-side append both survive with no
  conflict markers. The rows are read by key (brief/ts/sha), never by position, so the interleaved
  order a union merge can produce is harmless; a consumer wanting chronological order sorts by
  `ts`. (#588)
- **`.gitattributes` pins the board's inputs to LF, so the Windows CI leg matches Linux.**
  `statusgen --lint` compares `STATUS.md`, `CLAUDE.md` and everything under `docs/streams/**`
  byte-for-byte against a fresh regeneration. A default Windows checkout rewrote those files
  with CRLF, so the `windows-latest` leg (windows-port/04) reddened on `statusgen --lint`
  while Linux passed on the identical tree. A repo `.gitattributes` now forces LF on checkout
  for the board's inputs (`*.md`, `docs/streams/**`) while keeping `.ps1`/`.psm1` on CRLF.
  (#584)
- **`bodycheck` clears three measured false-positive classes** without widening what the
  secret scan admits: a doc PATH whose filename or directory segment is an exactly-32-hex
  string (`…/2026-08-30-<32hex>.md`, `…/findings/<32hex>/README.md`), a slash-separated list
  of short issue numbers (`#101/102/104/…`), and a `kind: Secret` TEMPLATE whose every value
  is a placeholder (`<…>`, `${…}`, `{{…}}`, `REDACTED`, `PLACEHOLDER`). Each fix is bounded by
  a paired POSITIVE corpus fixture of the same shape carrying a credential — a length other
  than 32, a hex pair with no word-shaped neighbour, an 8-digit numeric token, or one literal
  value among placeholders — which must still refuse, so a rule that cleared its negative by
  shape alone reds its pair.
- **`deskflip` no longer flips a PR ready over a standing `Security-Review: fail` when a
  content-preserving head move launders the finding (#361).** A `Security-Review: fail` is a
  retraction of the reviewed *code*, not of a commit sha, so a resync / merge-from-main / any
  re-trigger that leaves the flagged code byte-identical must not clear it. The security-lane
  reduction (`securityVerdictStanding`, formerly `securityVerdictAtHead`) now keeps a fail
  **standing regardless of the commit it was posted against**: only a later
  `Security-Review: pass` **at the current head** — or a genuine content change a reviewer
  re-reviews and passes — clears it; a bare head-sha change clears nothing. A `pass` keeps its
  existing at-head binding (new code needs a fresh review), so the two verdict kinds are
  deliberately asymmetric across a head move — fail-safe in both directions. The explicit-fail
  rule already blocked risk-classed and non-risk-classed PRs alike; this fix is what makes that
  rule reachable after the head moves.
- **`deskpost ready` no longer flips a PR ready over a standing `Security-Review: fail`
  when a content-preserving head move launders the finding.** This is the companion to the
  `deskflip` fix (#529): the deskpost ready path carried the identical flaw. A
  `Security-Review: fail` is a retraction of the reviewed *code*, not of a commit sha, so a
  resync / merge-from-main / any re-trigger that leaves the flagged code byte-identical must
  not clear it. The security-verdict reduction (`securityVerdictStanding`, formerly
  `securityVerdictAtHead`) now keeps a fail **standing regardless of the commit it was posted
  against**: only a later `Security-Review: pass` **at the current head** — or a genuine
  content change a reviewer re-reviews and passes — clears it; a bare head-sha change clears
  nothing. A `pass` keeps its existing at-head binding (new code needs a fresh review), so
  the two verdict kinds are deliberately asymmetric across a head move — fail-safe in both
  directions.
- **board-honesty `re-homed` phantom no longer fires on a stream re-homed INTO
  the repo.** The `NON-DISPATCHABLE (re-homed)` notice matched the word
  "re-homed" anywhere in a stream's README, so a stream re-homed *into* this repo
  — whose README describes its own arrival — had every LIVE `todo` row falsely
  flagged non-dispatchable, telling the dispatcher to skip real work. The class
  now keys on the ROW's own record: it fires only on a POINTER row (the README
  marks it re-homed AND the row's brief file is gone), never on the stream's
  history. A present brief file is the live-work signal that overrides the
  README narrative. (#581)
- **evidence-automerge — an `enable`-only "unstable" refusal is a benign skip.** The staged
  `evidence-automerge` workflow's auto-merge request could not recover from its own failure:
  a refused `enablePullRequestAutoMerge` reddened the run, the red check run made the pull
  request UNSTABLE, and UNSTABLE made the next request refuse — so re-running the job could
  never clear it and only a head-advancing push did. The step now reads the pull request's
  status-check rollup on an "unstable" refusal and, when the only failing check is this
  workflow's own job, logs a `::warning::` and exits 0, so the run greens itself and the next
  pull-request or review event enables auto-merge. A refusal while any OTHER check is failing
  still reddens, and a rollup that cannot be read or parsed is could-not-check and also
  reddens. (#586)
- Preflight cold-mint now inherits the platform's home-defining variables
  (`USERPROFILE`, `HOMEDRIVE`, `HOMEPATH` alongside `HOME`) into the scrubbed
  child environment, so a Windows child mint can resolve `os.UserHomeDir()` and
  find `roster.env`. Previously the child reported the roster absent on an intact
  envelope (`%userprofile% is not defined`), because the scrub kept only `HOME`.
- Roster-permission check now validates Windows file security via ACLs instead of POSIX mode bits. On Windows `os.FileMode` is synthetic, so the old group/world-writable mode test misfired; the check now reads the roster's owner SID and DACL and refuses a roster the invoking user does not own or that grants write to any principal beyond the owner, SYSTEM, or Administrators. Unix keeps its existing mode-bit + owning-uid guarantee. Applies to both `statusgen` and the desk-tools `deskkit` roster loaders.
- The model-capability floor no longer refuses an authority-bearing write on a PR stamped `dispatched-tier:any`. The tier a dispatcher stamps is the brief schema's own `exec-tier:` value, in which `any` records that the item demanded no particular runner — the ABSENCE of a strength demand, never an attestation that a weak one was launched. The floor read it as "attested below the strong tier" and refused, so every `deskpost review`, `deskpost ready` and `deskflip` on a PR dispatched from an unremarkable brief was blocked, which is not the population the floor exists to catch. A readable, dispatcher-applied `any` stamp now takes the same outcome as an unstamped PR — proceed with a NOTICE — and the NOTICE names the label it read, so an operator can tell it from the unstamped case without going to the labels API. Exactly one thing loosened: a stamp that conflicts, is incomplete, or was applied by a non-dispatcher identity is still present-but-UNREADABLE and still refuses, `any` halves included, and a self-applied stamp still clears nothing. The below-floor refusal itself is untouched and stays live for a future rung between `any` and `strong`.
- The re-stamp recovery test in `tools/desk` (`-run TestReStamp`, `internal/deskkit/modelstampactor_test.go`) is re-based to the floor's `dispatched-tier:any` contract. When the model-capability floor started reading a dispatcher-applied `any` stamp as no strength claim (NOTICE, not refusal), the floor's own tests were re-based but this one was not, so it still used `any` as its below-floor example and went red on `main`. It now expects the NOTICE for `any` — and that the NOTICE names the label — and proves the recovery still does not admit a weak tier with the same synthetic rung the floor's rank test uses: the rung neither meets the floor nor claims no strength, and a stamp naming it still refuses end to end. No production code changed.
- The worker-dispatch path no longer spends a worker on a row whose brief already has a pull request. A dispatch derived its branch as `feat/<stream>-<NN>` while a live PR for the same item sits on the claim-key branch form (`feat/<repo>--<stream>--<NN>`), so a branch-name existence check missed it and the worker only re-derived that the PR already existed. Both halves of the dispatch path now reconcile on the PR body's `Brief:` link trailer instead of a branch name: `fanoutloop plan` gains an already-represented exclusion (`FanoutLoop.Represented`) that drops a fresh Next-up row whose brief has an OPEN or MERGED PR, and `deskdispatch` gains a pre-claim phantom check that refuses a fresh worker dispatch (`--kit worker`, no `--pr`) for a brief already represented — before the durable claim is taken, so a refusal wedges nothing. A CLOSED-unmerged PR does not represent its brief (the work was abandoned, the row is dispatchable again), and a PR list the transport cannot read is could-not-check, never rounded to no-PR-exists. The reconciliation canonicalizes the `Brief:` trailer through the same reduction the create-time validation uses, so a PR authored with the accepted colon spelling (`Brief: <stream>:<NN>`) matches its slash-form brief id and is not missed.
- When NEITHER an intake directory nor an `INTAKE.md` view exists, the intake set
  is genuinely undetermined and now renders as **could-not-check** rather than
  "clear", closing a three-state-instrument-rule gap where a missing register
  became a false negative with no could-not-check state.
- `deskboard` now re-flags a PR for **RE-REVIEW** when a standing `CHANGES_REQUESTED`
  at the current head is followed by a finding-relevant **non-commit** resolution — a
  `*:skip` resolution label added, or the PR body/title edited — after the last review.
  The re-review trigger was keyed on the head SHA alone, so a fix that changed no commit
  (a label add, a `body` edit) left the row `BLOCKED` indefinitely, invisible to the desk
  until a human flagged it. The head-sha trigger is unchanged for the common case; the
  label-add time is read from the `labeled` timeline events and the body-edit time from
  `lastEditedAt` (which moves only on a title/body edit), both compared against the last
  review's submitted time, so an unrelated update does not re-flag. The signal is
  self-limiting — once the re-review posts, its verdict time is newer than the label/edit —
  and a suspected forged no-op flip still takes precedence. The reviewer must still verify
  the check state at head, since a label/body edit does not re-run CI.
- `deskdispatch --kit review` now emits a REVIEW-shaped Assignment section instead of the
  implementer scaffold. The top of the emitted prompt previously told every dispatched
  reviewer to "Open the draft PR", run `deskpr create`, "Stop at `implemented`", self-register
  a PR number, and release its dispatch claim once its branch was pushed — directly
  contradicting the read-only review clauses that follow. A reviewer handed both could open a
  spurious draft PR for a PR that is already open, or review the fresh branch cut off `main`
  rather than the PR's head. The review Assignment is now read-only: it names the PR under
  review, says the reviewer opens no PR and pushes no branch, points the worktree at
  `pull/<N>/head`, and releases the claim once the verdict is posted. The `--kit worker`
  (implementer) Assignment is unchanged.
- `deskdispatch`'s model-stamp step now REPLACES a present-but-unreadable dispatch
  attestation with a clean one, instead of only clearing foreign-applied labels. A
  conflicting, stale, or malformed `dispatched-*` label left by an EARLIER run of the
  dispatcher itself (a second model slug, a stale tier, an out-of-vocabulary half) was not
  removed on re-dispatch, so the stamp stayed unreadable and every authority-bearing write
  (`deskpost review` / `security-review`, ready-flip) kept refusing — a deadlock a re-dispatch
  reported OK yet could not break. The new `deskkit.ReStampRemovals` clears every present
  `dispatched-*` label that is not part of the pair being applied, so a dispatcher-identity
  re-dispatch is a supported recovery for a corrupt stamp.
- `deskfile new`'s 3-per-repo-per-24h budget is now charged to the session tag of the agent that FILES, so each dispatched agent has its own 3. `deskkit.SessionTag()` read only the harness's session id, and a dispatched agent is a child process that inherits that id verbatim — every agent in a fan-out reported the dispatcher's. Keyed on it the budget covered the whole fan-out rather than an agent: the first agent to file three exhausted every sibling's budget, and the rest were refused having filed nothing, which is the opposite of the "file at discovery" motion the gate exists to encourage. `SessionTag()` now resolves `$DESK_SESSION` — the desk tools' own per-agent session id, which `deskwt` and `deskroster` already consulted ahead of the harness id — before falling back to `$CLAUDE_CODE_SESSION_ID` and the legacy `$CLAUDE_SESSION_ID`, so the tools agree on who "this session" is instead of answering it two ways. The cap is unchanged at 3 and the window at 24h: the bound stays per actor, and being dispatched alongside others buys no agent a larger budget. A human-driven session that sets no `$DESK_SESSION` is unaffected. Audit lines now attribute a fan-out's writes to the agents that made them rather than to the one session that launched them.
- `deskflip`'s `checks-green` condition now keys an ABSENT check rollup on the base branch's
  ACTUAL required status checks (branch protection's `required_status_checks`), not the coarse
  roster ci-tag. A change on a repo that runs CI but requires no check to merge — checks that
  never fire on App-authored PRs, or a branch with no required checks — is no longer refused
  forever: an empty required set makes an absent rollup GREEN (nothing gates the merge on a
  check), a non-empty set keeps it could-not-verify (the required checks have not reported),
  and a required-set that cannot be read stays could-not-check and REFUSES (fail closed). The
  non-empty-rollup behaviour is unchanged.
- `deskinstall` is registered in the canonical tool-key registry (`deskkit.canonicalToolKeys`), so its writes count against its own audit budget instead of refusing as an unregistered tool, and `TestRegistryCoversCmdBinaries` is green on main again.
- `deskroster preflight` on GitLab no longer fails `commit-identity` when the worktree
  commits as the documented session / implementer identity (a real GitLab user such as
  `ih-bot`) rather than as the role service account. The check now distinguishes the two
  identities: it accepts a commit email that is an explicitly trusted session address —
  listed in the new `ASSAY_GITLAB_SESSION_EMAILS` roster allowlist — in addition to the
  service-account noreply shape used when the worktree commits *as* the service account.
- `desktoken <role>` with no `--repo` now resolves the installation owner from the
  configured allowed-repo roster (`ASSAY_ALLOWED_REPOS`), the same source the other
  desk verbs use, instead of the shipped `example-org` topology placeholder. A single
  configured owner resolves automatically; an unconfigured or ambiguous (multi-owner)
  set now fails closed with a clear could-not-check message that names the placeholder
  and tells you to pass `--repo` — replacing the bare `rc=6` that read like an auth
  failure when the real cause was an unresolved owner.
- `internal/deskkit/mutations.json`: re-synced the "stop the refusal advertising
  the override" mutation's `old`/`new` text to the current
  `internal/deskkit/scanoverride.go` line (`RefusedFinding(scanErr.Error()+OverrideHint(), f)`),
  which had drifted after an earlier refusal-shape change left the mutation's
  recorded `old` text unmatched. `muhar` now KILLS every mutation in the spec
  with none `COULD_NOT_MUTATE`.
- `scanloop` no longer pushes a new placeholder batch onto a scan PR that has already been flipped
  ready-for-human. A flipped PR is push-quiet — a post-flip push re-signals the whole review loop for
  churn the reviewer has already sealed off — so the coalesce decision now honours a `--scan-pr-state`
  (`draft` / `ready`) reading: a flipped PR opens the NEXT scan PR regardless of the coalesce window,
  and a draft/ready state that cannot be read never coalesces (the same bounded direction the
  unreadable-age arm already takes).
- `statusgen --lint --changed <file>` (the PR-side gate) now path-scopes the
  register-integrity check to the diff, the same way the DAR and product-scope
  checks already honour `--changed`. Previously the register lint walked the whole
  register regardless of the diff, so a single pre-existing register defect on
  `main` — a duplicate id, an unparseable date, an invalid id, an unauthorized
  field-gutting, a malformed park — hard-failed the `statusgen` check on *every*
  open PR that touched `docs/streams/**`, even PRs that never touched the
  defective entry, and the stale red never cleared until the unrelated main-side
  defect was fixed. Now a defect on a register file the PR's own diff never
  changed demotes to a `NOTICE:` (surfaced, never silently dropped — it is
  already red on main's own status-regen, which owns it), while a defect the diff
  introduces or touches — its file in the `--changed` set — still fails. With no
  `--changed` set (a full-tree / main run) behavior is unchanged: every register
  defect remains a hard `PROBLEM`.
- `statusgen --next-up` / `--consumers` no longer abort with `no frontmatter: first line must be ---` when a register directory under `docs/streams/` (e.g. the DECISIONS register) carries a self-declaring, frontmatter-free README. Stream discovery now recognizes a register that declares itself one per `spec/registers-v1.md` §7 and skips it instead of parsing it as a stream board, so every `statusgen`-driven Verify row returns its real result rather than could-not-check.
- `statusgen verifyrun` now runs each Verify row's command under `bash -o pipefail`, so a failing left-hand stage in a pipeline (e.g. `<a check that fails> | head/tail/grep -c ...`) surfaces as the pipeline's own non-zero exit and the row is recorded `fail`. Previously the row scored `pass exit=0` on the trailing reader's exit — a false clean where a check that never really ran was witnessed as passing. Non-piped commands are unaffected.
- `statusgen` no longer renders "the front door is clear" over a missing intake
  register. When the per-entry `docs/streams/intake/` directory is absent, the
  intake-debt alarm now falls back to the monolithic `docs/streams/INTAKE.md`
  view — the same legacy fallback the findings register already has — so a repo
  whose intake still lives in the single-file register is read rather than
  silently rounded to zero untriaged. Previously a missing directory produced an
  empty entry set, which the board rendered as a confident "clear" over a
  register it never actually read.
- `statusgen`'s Evidence-actor check (the `--lint` tamper sensor that asks whether an accepted verifier actor committed each `verified`/`done` row's `## Evidence` section) no longer reports **"unbacked"** on a **shallow or grafted clone**. A `.git/shallow` graft truncates history, so `git blame` bottoms out at the graft boundary and attributes the pre-graft Evidence lines to the boundary commit (typically a status-regen/worker commit) rather than to the verifier who authored them — which read as self-attestation and, on the incident box, flipped the count from 52 to 246 "unbacked" after a single shallow fetch. The backing commit is *unreachable behind the graft*, not *absent*, so the check now detects the shallow/grafted clone (`git rev-parse --is-shallow-repository` plus the `boundary` marker git blame emits for the grafted commit) and reports those rows **COULD-NOT-CHECK**, never unbacked. A tamper sensor must not read a truncated history as a finding. The remedy it names is `git fetch --unshallow`; the sensor only reports honestly and does not attempt the fetch. Real tamper detection is unchanged: a genuinely-absent backing on a **full** clone still reports unbacked, a shallow clone whose backing commit is visible past the graft is still judged normally, and an impostor commit (a present commit dressed as the verifier) stays a tamper finding rather than being relaxed.
- `statusgen`: the `pubmanifest` test fixture now uses a reserved neutral slug instead of a house-shaped one, so the leak-sweep class check no longer matches the fixture.
- `tools/desk` compiles again on `main`: `cmd/issueboard/addressto_test.go` (the desk-inbox `to:<role>` addressee tests) referenced the retired `installFakeGH` gh-shim helper after `cmd/issueboard` migrated fully onto the `deskkit.Forge` seam, leaving the `issueboard` package uncompilable (`go vet` / `go test` red across the module while `go build` stayed green). The addressee tests now run through the same recorded fake-`Forge` seam (`installForge`) the rest of the package uses — issues as `deskkit.IssueSummary`, addressee comment history as a `deskkit.TrustPayload` from `IssueTrustEvents` — with every `to:<role>` behaviour (label rendering, ADDRESSED band, SLA escalation) asserted unchanged and no `gh`-CLI dependency reintroduced.
- `verify-desk`'s existing sibling-checkout rule is tightened the same way: "resync" now means
  confirmed-current (`HEAD` compared against `origin/main` after the fetch), because a silent
  `git fetch` failure leaves the tree exactly as stale as before — a mismatch is
  could-not-check for that row, never a row run against whatever the tree happened to hold.
- iso-9001/01's tool-validation Verify row 3 now pins the pack's seven declared controls (decoupled from the over-broad `tools/desk/*mutations*.json` glob), so drift stays detectable without the stale "exactly six" count.

### Changed
- **`deskboard` resolves a PR's owning brief from the body's `Brief:` trailer** (both the
  `<stream>/<NN>` slash form and the `<…>:<stream>:<NN>` colon form), falling back to
  branch-as-claim only when the body names no brief. The board's `riskClassed` and
  `deskflip`'s risk classification now UNION a brief term over the existing visibility,
  security-surface-label, and changed-path terms. The brief term only ever WIDENS — it never
  waives a gate another term set. A body with **no** `Brief:` trailer leaves the term silent
  (the other terms decide); a body **with** a `Brief:` trailer that cannot be
  resolved/read/parsed is **UNVERIFIABLE, fail closed** — risk-classed, so the flip is
  refused until a security review passes at head. You cannot prove a declared brief you could
  not read is not `gate: human` / `risk: yes`, so a declared-but-unreadable brief is never
  treated as clean (the same "a short read is UNVERIFIABLE, not clean" rule the changed-file
  gate already uses).
- **`deskflip` risk-classification is now unioned across visibility, the security-surface
  label, and the changed-path triggers (#361 item 3), never path alone.** A PR carrying
  `surface:core` is risk-classed — and therefore requires a `Security-Review: pass` at head —
  even when none of its changed paths hit the compiled trigger set. The label term is
  **additive only**: its presence can only ADD scrutiny and its absence never waives the gate,
  so the fail-open direction `riskpath.go` warns of (a label read that could *waive* the gate
  on a mislabeled PR) is not reachable — only the fail-closed, tightening direction is used.
- **evidence-automerge — repository auto-merge OFF is a benign skip.** The staged
  `evidence-automerge` workflow's "Request auto-merge" step now treats GitHub's
  "Auto merge is not allowed for this repository" response as a benign no-op
  (`exit 0`), exactly like the existing "already enabled" carve-out, instead of
  reddening the run. Enabling auto-merge is an optimisation, not the merge itself —
  the required review and status checks still gate the actual merge, and a human can
  merge directly — so a repository with the setting off is a skip, not a failure. (#579)
- **evidence-automerge — the refusal decision is extracted and unit-tested.** The
  four benign outcomes (accepted, already enabled, repository auto-merge off, `enable`-only
  unstable) and the everything-else-reddens rule now live in
  `tools/evidence-automerge/automerge-refusal.sh`, proved offline by
  `tools/evidence-automerge/automerge-refusal_test.sh` against fixture rollups — including a
  committed pre-fix reference impl that reds on the new cases. (#586)
- **windows-port/04 board row flipped to `implemented`.** The Windows CI leg is delivered — the
  staged `windows-ci-leg.yml` landed (#569) and was promoted into `.github/workflows/` (#583),
  where the `windows-smoke` job runs green at `d684440` on the LF checkout the repo
  `.gitattributes` provides (#584/#585). The status flip was omitted from those PRs and is
  recorded here; `gate: human` verification of the Verify rows remains a separate step. (#592)
- A failed claim release is loud. The sink records a release only after the forge has answered that the ref is gone; a refused delete, an expired credential, a backend that cannot serve the namespace, and a resolver that returns nothing are each a non-zero outcome naming the stuck claim key, and none of them prints or records a release. An already-released claim stays a silent no-op, and a could-not-check is never mistaken for one.
- On a forge whose default branch takes no direct write, a verified brief's Evidence row now lands on a side branch and opens a draft change (a reviewer verdict lands the row), instead of a direct commit — stated as a design rather than discovered on a pilot.
- Promote the staged evidence-automerge and windows-ci-leg workflows into .github/workflows (round 2).
- The `worker-desk` and `pr-shepherd` skill bodies now say which half of the per-PR changelog convention a PR owes from its DIFF rather than from taste: a notable code PR ships the fragment, a documentation-only or Evidence-only PR owes none. Where a repo's changelog check does not already classify documentation and Evidence PRs on its own, the `changelog:skip` waiver is ASKED FOR from the maintainer — never applied by automation, and never self-applied. `pr-shepherd` additionally tells a shepherd to establish what the PR owes BEFORE writing a fragment for it, so a documentation PR does not acquire a changelog entry describing a change that is not in its diff.
- The batch-fanout loop now obtains the forge for a landing from the resolver instead of holding nothing. Its claim-releasing sink was reachable only from a test, so in production the field was nil and every landing declined to release — a release path that is dead in production is not a release path. The sink is now built from the resolver, refuses to exist without one, and asks it for the *item's own* target repository, so a batch spanning repositories releases each claim where it was taken. The no-write dry run is now selected explicitly by the surface that needs it rather than being what a misconfigured deployment falls back to.
- The claim key is bounded where the ref is built: a key is exactly one path component under the claim namespace, so a key carrying a path separator is refused rather than flattened into a ref no reader lists. Widening the namespace did not widen the guard — a ref path shaped like an API path, an absolute URL, or a bare unnamespaced component is still refused before any request exists.
- The cross-machine dispatch claim moves into the branch namespace — `refs/heads/dispatch/<key>` — on both forges, and its release now round-trips on each. It used to live in a namespace of its own directly under `refs/`, which GitHub can delete and GitLab cannot: live reads against a running GitLab deployment return the *route-miss* 404 body for `…/repository/refs` and `…/repository/git/refs`, so there was no general ref endpoint to implement a release against at any tier — while the Branches API answers, accepts a URL-encoded separator in a branch name, and has a recorded live create-and-delete at `HTTP 201`/`HTTP 204`. A claim that can be taken and never given back is a slot lost for good, so the claim moved to the one namespace both forges serve rather than the backend acquiring a reach it cannot have. The namespace, its ref path builder and its key parser are now a single definition every writer and every reader derives from, so the place a claim is written and the place it is looked for cannot drift apart. The decision, the live reads it turns on, the rejected alternatives and the costs it accepts are recorded alongside the stream.
- The design-approval gate is **scoped** (a `gate: model` all-risks-`no` brief is
  untouched) and **grandfathered by authoring date** (it binds only briefs authored after
  the cutover), so a pin bump reds nothing already in flight. The gate proves an approved
  record with a human approver exists; it does not mechanically prove that approver
  differs from the brief's author (the attribution-not-identity limit,
  `spec/lifecycle-v1.md` §7.1.2).
- The desk boards no longer launch a forge CLI for their central reads. `issueboard` and `scanloop` move FULLY onto the seam — `issueboard`'s issue list, its RETIRE-row title read and its trust read; `scanloop`'s trust probe. `scanloop`'s coalesced title/body refresh moves from a raw CLI edit onto the sanctioned `deskpr edit` verb (which carries the same secret-scan, self-containment, rate-limit and re-review controls the lane's other writes already do), and its process seam moves onto a literal-argv dispatch over a closed toolset, so its launch site resolves at compile time and leaves the checker's unresolved-argv ledger. `deskboard`'s two hand-authored GraphQL reads (the bulk open-PR read and the PR/issue trust queries) move onto the typed operations; its peripheral read surface stays on the CLI behind ONE narrowed permit row whose exit is a declared follow-up brief.
- The five desk-role skills (`the-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`,
  `intake-desk`) now run `deskack` as the first, one-permitted line after any human-typed
  message, and route cross-desk hand-offs through `deskfile new --to <role>` rather than a
  typed relay through the human.
- The forge-CLI permit register loses the eight rows those two verbs held, and its ratchet comes down from 24 to 16. The register's own header no longer lists either verb under the identity-blocked heading: both already minted a token and already refused an ambient fallback, so their identity question was answered before this change and the transport swap was all that remained.
- The forge-CLI permit register loses the three rows those migrations retired (`issueboard`'s reader and `scanloop`'s two sites), and its ratchet comes down from 16 to 13. `deskboard`'s row is narrowed rather than removed, and the now-stale `scanloop` executor row leaves the unresolved-argv ledger.
- The four `ask-decision`/`install`/`pdfingest`/`upgrade-assay` degradation cells in
  `plugins/assay/references/{claude-code,codex,cursor}.md`, previously marked
  "proposed — pending the driver's ruling on #626", are now settled binding content: the
  ruling landed (#626), recorded in a new design-decision record
  (`docs/streams/decisions/DR-harness-code-dehouse.md`), and the stream README's row 14
  moves `blocked` → `implemented`.
- The migrated trust reads route through the SAME envelope reader (`itemFromEnvelope` + `collectEvents`) the raw-bytes CLI readers use, so the seam and the CLI cannot draw different blessings from one payload; an incomplete (overflowed) trust payload still fails closed to quarantine.
- The reviewer reference kit's verdict-mechanics clause gains ONE exemption to the same-head APPROVE-after-CHANGES_REQUESTED prohibition: where the only change since the block is a LABEL that turned a required check green, a same-head re-approve is a genuine re-verification and is permitted, provided its body names the label, names the check, and states that the diff is byte-identical to the one reviewed. The clause also states plainly what the exemption does not do — a flip gate compares head shas, a label moves no head, so the flip still refuses on its own terms and clearing the standing rejection remains with whoever owns that gate.
- The two Windows "deferred fast-follow" deferrals are retired: `docs/adopting-assay.md`'s
  Prerequisites and `plugins/assay/skills/install/SKILL.md` §Scope now point at the real
  Windows install path instead of stopping at Windows.
- The walkthrough mirrors the delivered state honestly, not aspirational parity: it lifts the
  SessionStart-hooks `documented-workaround` (install Git-Bash for `bash`+`jq`) verbatim from
  the portability audit, states the Windows CI leg as **staged (PR #569), pending a
  maintainer's promotion into `.github/workflows/`** rather than a live green check, and marks
  the native `windows/arm64` smoke **BLOCKED** pending an arm64 Windows runner (the arm64
  asset still ships cross-compiled + checksummed).
- `derived-board/03` Verify row 4 re-anchored from `desk-containers/02`/`PR #67` to `derived-board/02`/`PR #80`: the reconcile engine can only witness a brief whose merged deliverable PR carries a `Brief:` trailer, and `desk-containers/02`'s deliverable PR lives in another repository and carries none — so a `--repo medici-finance/assay` run correctly returns it `todo`. The engine is sound; the row now dereferences PR #80, whose body carries `Brief: derived-board/02`, and passes online.
- `desk-tools/10`: re-scoped the brief's two whole-module Verify rows to what the brief owns — row 9's `go test ./...` becomes the brief's own + consumer packages (`deskclaim`, `deskkit`, `loopengine`) with #555's two unrelated deskkit test reds `-skip`'d, and row 10's `gofmt -l` narrows to the touched files (`cmd/deskclaim/*` + `tools/desk/internal/deskkit/claim.go`); module-wide debt stays #555's and CI still runs the full suite.
- `desk-tools/12`: re-scoped Verify row 8's whole-directory `gofmt -l statusgen` to the brief's touched files (`statusgen/briefinfo.go statusgen/briefinfo_test.go`), so the row stops failing on the unrelated pre-existing `statusgen` files that flag only under a newer local gofmt (#555's module-wide drift); CI still runs the whole check.
- `deskevidence --brief-path` is now idempotent at the block level, not just the file level: a
  fresh Evidence block byte-equivalent (after normalising line endings, per-line trailing
  whitespace, and trailing blank lines) to the block already standing at the end of the brief's
  `## Evidence` section is a no-op — it prints `noop: Evidence block already present …`, exits 0,
  and commits nothing, instead of appending a duplicate. Equivalence is narrow: a re-run on a
  different date or runner, a one-character change, a partial (prefix) re-run, or a superset that
  adds new rows all count as new evidence and still land.
- `deskevidence` no longer signs a GitHub App JWT, performs the installation-token exchange, or builds `net/http` against a hardcoded API host. Its Evidence write goes through `WriteFile` and its `--brief-path` Evidence-section merge reads the remote brief through `ReadFile`, both under the verifier App's custody as the resolver hands it — the mint moves to the identity layer (`desktoken verifier`), and the dead JWT/Contents-API code is deleted rather than left dormant.
- `deskflip` and `deskreply` no longer launch a forge CLI. Every read and every write both verbs make now goes through the forge the resolver returns for the repository — a client bound to an explicitly minted App installation token, with refusal as the only fallback — so which forge serves a repository is configuration rather than an assumption compiled into each tool. `deskpost`'s mechanical verdict labels move to the same seam. The dead shell helpers are deleted rather than left dormant.
- `deskflip` gains a fail-closed check it could not express before: each CI rollup now carries the forge's own asserted total, so a rollup that serves fewer entries than the head claims is could-not-check rather than green — the same reconciliation its changed-file read has always had.
- `deskkit.ReStampRemovals(timeline, want, isDispatcher)` computes the re-stamp removal set as
  a superset of `ForeignStampLabels`. The model-capability floor's reader is UNCHANGED: a
  self-applied stamp and a genuinely below-floor tier still refuse — only the dispatcher can
  replace a corrupt stamp with a good one.
- `deskpost review --head` now states the accepted SHA form as "40- (or 64-) character
  lowercase-hex" in its usage error and in `tools/desk/README.md`, matching what
  `isFullSHA` actually accepts (40 for SHA-1, 64 for the SHA-256 object format) rather
  than only naming 40. Behaviour is unchanged; only the guidance text is now accurate.
- `deskpost` no longer binds a forge host literal of its own. Its App-token mint and
  its REST reads — the ones with no typed operation on the frozen `Forge` seam
  (repository contents, the head commit's author, the trust-gate GraphQL query and the
  present-label set) — now source their host through the forge module
  (`deskkit.GitHubBaseURLOrDefault`) rather than a package-level `apiBaseURL` bound to
  the host at init. The production host override is EMPTY, exactly as `deskflip`'s is,
  so the concrete literal lives in one place (the forge module) and never in a cmd
  package; the custody minter `deskpost` installs on the resolver returns that same
  empty override, so its writes and its reads share one seam. Transport only — every
  request `deskpost` makes, and the reviewer-App identity it makes them under, is
  unchanged at the wire. Completes forge-neutral/03 row 8 (the host binding is gone,
  not merely unused).
- `desktoken` now declares its tool class and echoes its effective config (the P3
  `assay-config:` lines) on stderr like every other roster-reading verb; its stdout
  remains the token path alone.
- `docs/adopting-assay.md` — the "Running Assay on Cursor" section now points at the scripted
  Cursor smoke protocol (a full desk loop: dispatch → isolated worktree → draft PR → one review
  cycle, both surfaces headless-first) and links the acceptance step to harness-portability
  brief 13, in place of its bare unlinked acceptance-step sentence. The brief 13 board row flips
  to `implemented` with the live desk-loop run (Verify row 8) held BLOCKED for a
  human-sanctioned Cursor session — never greened from the protocol text alone.
- `freshness.yaml` registers the two harness capability matrices and the three per-harness
  binding files under a 45-day re-review leash.
- `plugins/assay/scripts/inbound-monitor.sh` adopts the same `ASSAY_MONITOR_PACE_SECONDS` sleep
  between repo reads and the same stop-on-limit rule; its behaviour is otherwise unchanged.
- `spec/lifecycle-v1.md`, `spec/brief-v1.md` and `spec/registers-v1.md` carry the new
  gate, key and register with conformance clauses; `docs/iso9001-mapping.md` rows 8.3.2
  and 8.3.4 are updated to what is now true — all three of review, verification and
  validation are named — while still saying, in the same breath, that intended-use
  validation is the adopter's act.
- `statusgen` board-lint now names the fix inline when a Briefs-table Status cell
  is malformed: the `invalid status` PROBLEM and the row cell-count error both
  explain that the Status cell takes only a bare lifecycle token and that a PR
  reference belongs in the PR body / `Brief:` trailer, never in the cell. The
  cell-count check also now rejects a row with EXTRA cells (a decorated
  `implemented (#NN)` value plus a stray `||` shifts every column right), which
  previously slipped past the lint and surfaced as a confusing downstream error.
- `windows-port/00`: re-scoped Verify row 7's whole-module `go test ./...` in tools/desk to the split-affected packages (`internal/deskkit`, `internal/loopengine`, `cmd/deskpost`, `cmd/deskevidence`, `cmd/deskrelease` — the flock and owner-check sites), with #555's two unrelated deskkit test reds `-skip`'d; statusgen stays `./...` (its single root package is the split package). CI still runs the whole suite.
- gofmt-formatted 7 pre-existing unformatted files under `tools/desk`; no logic change.

## v0.27.0 — 2026-09-06

### Added
- **Windows is pinned, not deferred.** v0.26.0 is the first release to publish
  `statusgen-windows-{amd64,arm64}.exe` and `desk-tools-windows-{amd64,arm64}.tar.gz`, so the
  Windows arms of both the pairing manifest and the adopter scaffold carry real, published
  sha256 values. The scaffold's two all-zero placeholder digests — which would have failed an
  adopter's verification the moment a Windows install was attempted — are gone.
- **`commsgw`** — the per-cell message gateway: the one chokepoint every
  inbound cell message crosses. A deterministic pre-check pipeline (mTLS peer
  accept, envelope parse-or-refuse, signed-assertion verify, lane ACL incl. the
  cross-cell pair + verb allow-set, `Claim()` dedupe, per-sender rate/budget,
  kill switch) runs identically for cross-cell traffic (an A2A JSON-RPC server
  on the pinned `a2a-go` SDK, mTLS-fronted) and within-cell traffic (a loopback
  Unix socket matching `deskcomms`'s existing client wire shape). Config-off by
  default: every `ASSAY_COMMS_*` enable key is required, any one absent refuses
  to serve. Accepted messages are durably queued (`internal/commsqueue`) for
  `commsloop` to drain. On every accepted cross-cell message the gateway emits
  one deskd inbox item of kind `cross-cell`; an emission failure quarantines
  the message rather than dropping it.
- **`commsloop`** — the paired drain consumer: the fifth implementation of the
  frozen `loopengine.Loop` contract. Report-class messages (`status`,
  `metrics`, `help-offered`) land done+journaled with no session ever fired;
  everything else quarantines (held mailbox + a filed issue) pending the
  prose router. An independent, second lane-ACL check at the routing boundary
  (a different file from the gateway's own check) catches a message that
  somehow bypassed the gateway's precheck.
- `internal/comms/laneacl.yaml`'s `# OPEN DECISION` marker is replaced with a
  public-safe citation of the cross-cell verb ruling (date + bare decision
  record number only).
- `isReportClass` (which verbs land immediately vs. await the prose router) is
  a documented, reviewable judgment call pending that router's own arrival —
  see `cmd/commsloop/routing.go`.

### Fixed
- A PR whose `dispatched-model:` / `dispatched-tier:` stamp was once applied by the wrong identity can now be repaired. The model-capability floor's applier-aware reader treated ANY historical `labeled` event for a stamp label as the applier — and a GitHub timeline is append-only, so that made a foreign stamp permanent: the bound dispatcher could remove both labels and re-apply them under its own App, and `deskpost review`, `deskpost ready` and `deskflip` still found the original event and refused the write, naming a login that no longer held the stamp. The reader now resolves, for each stamp label the PR CURRENTLY carries, the actor of the LAST `labeled` event not superseded by an `unlabeled` of the same name, so a genuine re-stamp is honoured. Nothing is laundered: a foreign stamp that is still standing, a re-stamp by a foreign login, and a dispatcher stamp later overwritten by one all still refuse. Presence now comes from the PR's labels rather than from the events, so a superseded label no longer contributes stamp content and a truncated timeline read can no longer make a standing stamp look ABSENT (the only state that proceeds) — a present label the events cannot attribute is could-not-check, with its own refusal wording. The floor was not loosened in any other direction.
- The two known-key sets are now COUPLED rather than kept in step by comment. Both
  modules' readers expose their set (`scanKnownRosterKeys` / `knownRosterKeys`), the
  shared cross-tree vector file `statusgen/testdata/roster_coupling.json` declares the
  schema once, and each module's `TestRosterKeySchemaCoupling` asserts its own set equals
  that list exactly in BOTH directions. A key taught to one binary alone — or declared and
  taught to neither — cannot stay green. The two trees are separate Go modules and share
  no code, so the shared vector file is the binding a shared package cannot be.
- `ASSAY_WITHHELD_IDENTIFIERS` is now read from `roster.env` as well as from the
  environment, like every other `ASSAY_` key (environment first, roster second).
  It was environment-only while the roster parser merely *recognised* the key, so a
  house that configured its withheld register in `roster.env` — the documented home
  of every other value — got `withheld register identifiers NOT CHECKED` on every
  public write and the register category of the public-repo self-containment scan
  never ran, unless each shell invoking `deskpr`/`deskpost` also exported the
  variable. Unset in both sources is still a complete adopter configuration: the
  category degrades to a notice, which now names both places it looked.
- `deskdispatch`'s stamp step now REPLACES a foreign-applied stamp instead of no-opping on it. Labels are a set, so `--add-label` over an already-present label changed nothing and left the PR carrying the untrusted application; the step now removes each standing dispatched-* label it cannot attribute to the dispatcher, as the dispatcher, before applying its own — the removal and the re-application both logged in the step report.
- `statusgen` now recognises every `ASSAY_` roster key the desk tools recognise
  (`ASSAY_REPO_FORGES`, `ASSAY_RISK_CALLOUT`, `ASSAY_WITHHELD_IDENTIFIERS`,
  `ASSAY_ALLOW_CLUSTER`), so a roster that the desk verbs REQUIRE no longer makes
  `statusgen` report the whole trust roster unconfigured. Both binaries read the same
  `roster.env` and both fail closed on an unrecognised key in the `ASSAY_` namespace —
  correct for a typo, wrong for a sibling's key: while the two known-key sets disagreed,
  `ASSAY_REPO_FORGES` (the only way `deskpost` / `deskpr` / `deskfile` resolve a repo to a
  forge) made `statusgen --scan-issues` refuse fail-closed on every scan repo, and no
  roster edit could satisfy both tools at once. The four keys are **recognised, not
  applied**: `statusgen` consumes none of them, and the refusal for a genuinely unknown
  `ASSAY_` key is unchanged.

### Changed
- The plugin's `paired-versions.yaml` is re-pinned to statusgen / desk-tools **v0.26.0** and
  plugin **0.5.1** — both sides of the pairing move together, with every per-platform sha256
  refreshed from the v0.26.0 release's own `checksums.txt` rather than edited in place.
- `examples/adopter-scaffold/.assay-versions` moves off the long-stale `v0.9.1` pins to
  **v0.26.0**, so the scaffold an adopter copies no longer illustrates a tool seventeen releases
  behind the skills shipped beside it.

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
