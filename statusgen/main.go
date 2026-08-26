package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// run operates in one of four modes:
//   - "write" (default): regenerate and write STATUS.md.
//
// Every mode reports two classes of hard problem — see the classification
// comment in the body. Both classes print as PROBLEM: and
// both exit 1; only the blocking class suppresses generation.
//   - "check": verify the committed STATUS.md byte-matches a fresh regen (exit 1 on drift).
//   - "lint":  run every source check AND build the view (a true superset of
//     generation), but with NO STATUS.md read, write, or drift comparison. This
//     is the PR-side gate — STATUS.md has a single writer (main's CI), so
//     branches must never depend on or touch it. Callers
//     going through main() get the CLAUDE.md budget check by default in this
//     mode (resolveBudgetSpecs) — run() itself only checks
//     whatever budget specs it is handed.
//   - "record": diff current brief status against docs/streams/.history.jsonl
//     and append any transitions. Single-writer, same as STATUS.md itself —
//     wired ONLY into main's status-regen CI, run AFTER STATUS.md regenerates.
//     Never invoked by --lint.
func run(root, mode string, budget []string, changed []string, scope string) int {
	// Reset the phase-4 drive-dashboard render inputs each run (one run() call
	// per root under --root repeats; a leftover from a previous root must never
	// leak its drive section into the next board).
	activeDriveStatuses = nil
	activeDriveHeartbeat = ""
	// Word-budget checks run FIRST — a budget violation is a
	// hard PROBLEM just like any other source-check failure. Malformed specs
	// were already caught in main() before reaching run().
	//
	// A budget failure does NOT short-circuit the remaining phases.
	// It used to: the run reported `FAIL 1` and the link check never ran, which
	// read as "65 problems just got fixed" to anyone diffing lint output across
	// two trees — the dominant way this output is used in review. The budget
	// check shares no state with any later phase, so the short-circuit bought
	// nothing and silently changed WHICH checks ran. Every independent phase now
	// runs and the union is reported.
	var budgetProblems []string
	if len(budget) > 0 {
		bp, err := checkBudget(root, budget)
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			finalVerdict(mode, 1)
			return 1
		}
		budgetProblems = bp
	}
	streams, findings, err := loadStreams(root)
	if err != nil {
		// Unreadable sources genuinely do stop everything downstream — but the
		// budget findings are already in hand, so report them rather than lose
		// them to the early return.
		for _, p := range budgetProblems {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		finalVerdict(mode, len(budgetProblems)+1)
		return 1
	}
	// Attach issue-loop placeholders (schema: placeholder-v1) as synthetic Briefs
	// BEFORE any check or view build, so the whole pipeline — check, Next-up
	// eligibility/scoring, emit — treats them as first-class rows.
	// Malformed placeholder files are surfaced by checkPlaceholderFiles below.
	attachPlaceholders(streams)
	// Product-scoping: per-stream checks may be
	// restricted to a single product (serves:) so one product's PR is not
	// red-gated by another product's stream. An explicit --scope wins; otherwise
	// CI's --changed set auto-derives the scope (deriveScope). checkStreams feeds
	// ONLY the per-stream checks — the whole-house `streams` still drives the view
	// build below, so lint stays a true superset of the post-merge regen.
	effScope := scope
	if effScope == "" {
		effScope = deriveScope(streams, changed)
	}
	checkStreams := streams
	if effScope != "" {
		checkStreams = filterStreamsByServes(streams, effScope)
	}
	// Per-stream/per-brief checks run against the scoped set; the findings
	// affects: known-stream validation resolves against the FULL stream set so a
	// single-product PR never falsely flags a finding referencing another
	// product's stream as "unknown stream".
	problems, notices := checkScoped(checkStreams, streams, findings)
	// A root that loaded cleanly but discovered zero streams.
	// Checked against the FULL stream set, never checkStreams — an
	// --scope-filtered subset being empty is normal (a single-product PR) and
	// must not trip this; this is about the ROOT contributing nothing at all.
	if len(streams) == 0 {
		msg := emptyRootMessage(root)
		if allowEmptyRoot {
			notices = append(notices, msg)
		} else {
			problems = append(problems, msg)
		}
	}
	// Budget findings lead the report — same ordering as before,
	// now as part of the union rather than instead of it.
	problems = append(budgetProblems, problems...)
	// Problem classification. TWO classes. Both are reported
	// as PROBLEM: lines, both still exit 1 — nothing is downgraded, silenced or
	// swallowed. They differ in ONE respect: whether they suppress generation.
	//
	//   problems (BLOCKING) — DEFAULT, and everything lives here unless proven
	//     otherwise. Any check that reads a file the board also reads. If one of
	//     these fails the board data itself is suspect, so a board built from it
	//     — or a status transition derived from it and appended to the
	//     append-only history — could record something wrong. These keep
	//     short-circuiting.
	//
	//   offBoardProblems (NON-BLOCKING) — exactly ONE member: darSyncCheck. Its
	//     inputs are provably disjoint from docs/streams/** and CLAUDE.md — the
	//     board's own sources — so it cannot make STATUS.md or .history.jsonl
	//     wrong. Suppressing the regen over one of its disagreements would cost
	//     the generated artifact and the history append for nothing.
	//
	// The membership test is per-CHECK-INPUT, not per-check-topic, and the two
	// link lints are the cautionary tale. They look like repo hygiene, but both
	// take docFiles(root) — CLAUDE.md plus EVERY *.md under docs/**, which is a
	// SUPERSET of the board's sources, including every stream README. They are
	// also the ONLY checks that catch a README row pointing at a brief file that
	// does not exist: checkBriefFiles, verifySectionProblems and
	// attributionProblems all iterate briefFilePaths(s), a glob over files that
	// EXIST, so a phantom row is invisible to them. Classifying the link lints
	// off-board would let a `done` row with no brief behind it into a committed
	// STATUS.md and into the append-only history. They stay BLOCKING, and
	// TestPhantomBriefRowStillBlocks pins that.
	//
	// Why the split matters at all: STATUS.md has a single writer (main's CI). A
	// suppressed write leaves a STALE generated artifact on main that still
	// claims to be current, and a suppressed --record permanently loses
	// transitions from an append-only log. Both are worse than a red-but-current
	// board. What would be worse still is hiding a genuine integrity failure —
	// which is why every problem is still printed and the exit code is unchanged,
	// and why the off-board class is kept to the single check that provably
	// cannot indicate board wrongness.
	var offBoardProblems []string
	// No .git directory at all (a `git archive` export) means
	// every git-history-dependent check below runs degraded or is skipped
	// outright — register ID grandfathering, the tombstone-deletion and
	// field-gutting guards, the human-stamp gate, claim filtering. Individual
	// checks report their own degradation where it changes their verdict (see
	// grandfatheredBaseFallbackNotices, registerBaseFallbackNotices,
	// humanStampProblems); this NOTICE fires unconditionally and up front so
	// the caveat is visible even when no individual check happens to have
	// data to mis-fire on. Differential lint output — "is this red mine or
	// pre-existing?" — is unsound across a run against this kind of tree: the
	// fix is a real worktree (`git worktree add`), not `git archive`.
	if hasNoGitDir(root) {
		notices = append(notices, "git metadata unavailable: this tree has no .git directory (e.g. a `git archive` export) — checks that depend on git history run degraded or are skipped outright, so PROBLEM/NOTICE counts from this run are NOT comparable to a run against a real worktree. Use `git worktree add` to compare tree states, not `git archive`.")
	}
	// serves: coverage — a stream with no product tag can never be scoped;
	// NOTICE it so the taxonomy stays complete as streams are added.
	notices = append(notices, servesCoverageNotices(streams)...)
	// checkStreams drives WHICH briefs are validated (product-scoped under
	// --changed/--scope); `streams` (the full house set) resolves cross-stream
	// depends:/unblocks: refs so a valid dependency on an out-of-scope or paused
	// stream is not falsely reported "unknown stream". Mirrors the
	// checkScoped(checkStreams, streams, findings) split above.
	briefProblems, briefNotices := checkBriefFiles(checkStreams, streams)
	problems = append(problems, briefProblems...)
	notices = append(notices, briefNotices...)
	// #1250 ordering-gate lint (dependency-graph-design.md §6): flags gate-shaped
	// README/brief prose ("no X before Y", "blocked on", "gated on") whose real
	// ordering prerequisite is encoded in no `depends:`/`unblocks:` edge, so an
	// unencoded gate surfaces as a worklist NOTICE instead of silently dispatching
	// a worker past it. NOTICE severity this phase (§6.3 Phase A) — flips to
	// PROBLEM once `gates:`/`feathers:` exist for authors to reach for.
	notices = append(notices, orderingGateNotices(checkStreams)...)
	// consumers: routing claims (brief-rule 9). Offline
	// half only: a follow-up naming a brief that does not exist is DISPROVED by
	// the stream tables and is a hard problem; anything needing a diff to settle
	// is a NOTICE here and the gate is `--consumers`, which judges it against the
	// branch's own diff. See consumers.go for why the severities split that way.
	consumerProblems, consumerNotices := consumersCheck(root, checkStreams)
	problems = append(problems, consumerProblems...)
	notices = append(notices, consumerNotices...)
	placeholderProblems, placeholderNotices := checkPlaceholderFiles(checkStreams)
	problems = append(problems, placeholderProblems...)
	notices = append(notices, placeholderNotices...)
	// The per-stream done/ archive checks — a NOTICE for a
	// retired placeholder still at the stream root (archive candidate) and a
	// PROBLEM for any non-done brief/placeholder parked under done/. Additive,
	// outside the tombstone guard (registers.go).
	archiveProblems, archiveNotices := checkArchivedPlaceholders(checkStreams)
	problems = append(problems, archiveProblems...)
	notices = append(notices, archiveNotices...)
	// Stream-LEVEL archive-ready surface (#947): a NOTICE per stream whose every
	// brief is done (and which carries at least one real README-table brief), so
	// the desk sees the whole stream is an archive candidate. Detect, don't
	// auto-flip — the archival itself stays a desk confirmation. Notice-only, so
	// it never changes the exit code; the stream-level counterpart to the
	// retired-placeholder NOTICE above.
	notices = append(notices, streamArchiveReadyNotices(checkStreams)...)
	notices = append(notices, freshnessCheckNotices(checkStreams)...)
	// Numbering-space collisions in docs/brief-rules.md (desk-hardening/05, #54):
	// two branches allocating the same rule number in parallel merge cleanly and
	// leave a citation ("brief-rule 26") that resolves to two different rules. This
	// is the BRANCH-LOCAL half and it says so in its own output: on a branch it can
	// only read that branch's copy, so it sees the collision after main already
	// carries both. `statusgen mergecheck` runs the same detector over the
	// trial-merged tree, which is where it can see it first. NOTICE severity, per
	// mergedstatus.go's precedent — the corpus carried 2 collisions when the check
	// was written. Declared source: statusgen/numberspace.go.
	notices = append(notices, briefRuleNumberNotices(root)...)
	// Merged-PR / status reconciliation (#270): a merge that
	// names <stream>/<NN> in its branch or subject, against a README row still at
	// todo/in-progress, means Next-up is offering work that already landed. NOTICE
	// only this phase — promotion to PROBLEM is a later ruling, after the standing
	// backlog of drifted rows is reconciled. Skipped outright on a tree with no
	// .git (the unconditional NOTICE above already states that caveat); otherwise
	// three-state, with the git read's own error surfaced as could-not-check.
	if !hasNoGitDir(root) {
		mergedPRs, mergedErr := mergedPRsFromGit(root)
		notices = append(notices, mergedPRStatusNotices(checkStreams, mergedPRs, mergedErr)...)
	}
	// Evidence-actor (desk-apps/07, F-verify-self-attest): a `verified`/`done`
	// row is backed only when an ACCEPTED actor — the roster-bound verifier App or
	// a roster-known human — committed at least one line of its `## Evidence`
	// section. Everything else on a verified row is prose the verifying session
	// wrote about itself. NOTICE only this phase: 92 of 141 rows were measured
	// unbacked at adoption, and arming a PROBLEM against that would red every
	// unrelated PR (the mergedstatus.go precedent, one line above). Skipped on a
	// tree with no .git, exactly like the reconciliation above; every other
	// unreachable input reports could-not-check by name rather than clean.
	if !hasNoGitDir(root) {
		notices = append(notices, evidenceActorNotices(root, checkStreams)...)
	}
	// `repo:` frontmatter validation: form + one-repo-per-
	// root agreement. Runs on the FULL stream set, not the scoped subset — repo
	// ownership is a property of the root, so a product-scoped PR must not be able
	// to hide a conflicting declaration in an out-of-scope stream. Inert when no
	// stream declares a repo:, which is why adding it changes nothing for a repo
	// that has not adopted the field.
	rootRepoName, repoProblems := rootRepo(streams)
	problems = append(problems, repoProblems...)
	problems = append(problems, registerIntegrityProblems(root)...)
	// The register field-gutting guard inside registerIntegrityProblems compares
	// against the merge-base with origin/main. When that ref is unresolvable the
	// base falls back to HEAD and already-committed gutting is compared against
	// itself — a silent fail-open. Say so rather than run degraded in silence
	// (review NOTE); advisory, never a hard problem.
	notices = append(notices, registerBaseFallbackNotices(root)...)
	// Channel-conformance sweep (distribution/05): advisory NOTICE per
	// adopter-facing surface still teaching a non-sanctioned acquisition
	// channel, plus an explicit could-not-check line for anything the sweep
	// could not read and an explicit line for every KNOWN-ACCEPTED deviation.
	// Advisory by design — it never changes the exit code — but it always
	// prints a summary, including on a clean run, so "passed" and "did not
	// run" are distinguishable. Declared source: statusgen/channels.go.
	notices = append(notices, channelConformanceNotices(root)...)
	// Human-stamp sole-writer gate: verify-gate-close.yml
	// is the sole permitted writer of human:<name> sign-off stamps in stream-README
	// Verified/Reviewed cells. This --lint rule flags any stamp gained on a PR
	// branch relative to the merge-base with origin/main. Armed only on branches
	// (merge-base != HEAD); inert on main itself. Gated to --lint mode only:
	// in write/check/record modes the gate is not meaningful (there is no PR branch
	// to compare against).
	if mode == "lint" {
		hp, hn := humanStampProblems(root, streams)
		problems = append(problems, hp...)
		notices = append(notices, hn...)
		// Stale-issue alarm (methodology-metrics/28): a NOTICE + board line when
		// an open issue has been sitting past the threshold, mirroring the intake-
		// debt alarm applied to issues. gh-GUARDED: skipped (no NOTICE) when gh is
		// absent or fails, so the offline --lint gate never gains a hard network
		// dependency. Advisory only — never a hard problem. Gated to --lint mode so
		// the STATUS.md regen path stays offline/deterministic.
		if n := openIssueDebtNotice(staleIssueDaysCfg); n != "" {
			notices = append(notices, n)
		}
	}
	// T9: when origin/main is unresolvable, grandfatheredIDs returns empty and
	// idFormatProblems fires numeric-regression PROBLEMs against every
	// legitimately-landed legacy entry, with messages that do not name the
	// real cause. This NOTICE states it.
	notices = append(notices, grandfatheredBaseFallbackNotices(root)...)
	// The single off-board check — the only NON-BLOCKING problem class. Its
	// inputs are outside the board's own sources (docs/streams/** and CLAUDE.md),
	// which the board never reads, so a disagreement still fails the run but does
	// not suppress generation. What the check inspects is defined by the build
	// (see darSyncCheck's definition); it may be a no-op.
	darProblems, darNotices := darSyncCheck(root, changed)
	offBoardProblems = append(offBoardProblems, darProblems...)
	notices = append(notices, darNotices...)
	problems = append(problems, attributionProblems(checkStreams)...)
	problems = append(problems, verifySectionProblems(checkStreams)...)
	// Reverse-orphan (distribution/13 Task E-a): a README brief ROW whose brief
	// FILE is absent is a phantom brief. checkBriefFiles guards the forward
	// direction (a file with no row); this guards the reverse (a row with no
	// file), which every file-iterating check above is blind to. Three-state: a
	// phantom row is a hard PROBLEM, an unreadable stream dir is a could-not-check
	// NOTICE.
	reverseOrphanP, reverseOrphanN := reverseOrphanProblems(checkStreams)
	problems = append(problems, reverseOrphanP...)
	notices = append(notices, reverseOrphanN...)
	// *-private → do-not-copy publication assertion (distribution/13 Task E-b):
	// a stream directory whose basename matches *-private MUST resolve to a
	// do-not-copy disposition in docs/publication-manifest.yaml, making the
	// naming convention a machine-checked invariant so a private stream can never
	// silently ship to a public tree. Inert where no manifest and no *-private
	// stream exist (e.g. a public repo); three-state otherwise.
	privDoNotCopyP, privDoNotCopyN := privateStreamDoNotCopyProblems(root, checkStreams)
	problems = append(problems, privDoNotCopyP...)
	notices = append(notices, privDoNotCopyN...)
	// Verify row CLASSES (verdict-lane/02): an unknown class is a hard PROBLEM
	// (a row that routes nowhere is exempt from every gate), and a check:ci/check
	// SCRIPTED row on a non-todo brief whose verify.d script is missing or not
	// executable is a hard PROBLEM (a runner cannot re-execute an absent file).
	// A todo brief listing its planned scripts is exempt — see rowclass.go.
	problems = append(problems, verifyRowClassProblems(checkStreams)...)
	// Reviewer-conspicuity (verdict-lane/02): when THIS PR's diff touches any
	// docs/streams/*/verify.d/** script, raise a conspicuous NOTICE naming each
	// one so the reviewer — the trust anchor for verify scripts, with no freeze
	// rule behind them — assesses the change as executable code, not a docs edit.
	// Inert with no --changed set, exactly like the DAR check.
	notices = append(notices, verifyScriptDiffNotices(changed)...)
	// Unfailable Verify rows: a row whose command is structurally
	// incapable of failing manufactures evidence. NOTICE this phase — the rules
	// fire on briefs already on main, many of them closed, and rewriting a closed
	// brief's Verify table to green the gate is the very falsification the check
	// exists to catch. Flip to a hard problem once the active streams are clean.
	notices = append(notices, unfailableRowNotices(checkStreams)...)
	// Missing EXECUTION WITNESS (ground-truth/01, #284): a brief the README
	// calls verified/done whose Evidence carries no record that a Verify row was
	// actually executed. NOTICE this phase for the same reason the rule above is
	// one — every brief closed before the witness existed lacks one by
	// construction, and hand-writing witnesses into closed briefs to green the
	// gate would manufacture exactly the evidence the witness exists to replace.
	// Flip to a hard problem once the active streams are backfilled.
	notices = append(notices, witnessNotices(checkStreams)...)
	// CONTRADICTED status cell (ground-truth/02, #284): a brief the README
	// calls verified/done whose own Evidence carries a witness recording a
	// FAILURE. Distinct from the NOTICE above, and a harder severity on
	// purpose — that one is about a MISSING record (the whole inherited
	// corpus), this one is about a record that says the opposite of the
	// cell. Transition-scoped like unrunGateChecks below: only a closure
	// this branch made is a PROBLEM. See witnessgate.go.
	wgProblems, wgNotices := witnessGateChecks(root, checkStreams)
	problems = append(problems, wgProblems...)
	notices = append(notices, wgNotices...)
	// Dead-link lint. BLOCKING: docFiles(root) is CLAUDE.md plus every
	// *.md under docs/**, so its inputs INCLUDE every stream README and brief
	// file. It is also the only check that catches a README row whose brief file
	// does not exist — see the classification comment above.
	docs, docWalkProblems := docFiles(root)
	// An unreadable docs subtree is a could-not-check, not zero problems: surface
	// it so the lint fails instead of passing on a truncated file set
	// (docs/three-state-instrument-rule.md).
	problems = append(problems, docWalkProblems...)
	problems = append(problems, linkProblems(root, docs)...)
	// Identifier dereference (mistake-proofing/02): a backticked TEST or FUNCTION
	// name cited in CLAUDE.md or a brief must resolve against the tree, exactly as
	// a backticked FILE path already must. Lands ADVISORY (NOTICE) —
	// identifierDereferenceFatal gates the flip to a hard PROBLEM once the
	// inherited corpus census is fixed/waived. See linkcheck.go.
	idProblems, idNotices := identifierDereferenceCheck(root, docs)
	problems = append(problems, idProblems...)
	notices = append(notices, idNotices...)
	// Register-reference link lint: for every markdown link
	// whose text is F-NN/I-NN, verify target file exists and frontmatter id
	// matches. Bare refs are never checked. BLOCKING for the same reason — it
	// takes the same docFiles(root) superset.
	rp, rn := registerRefProblems(root, docs)
	problems = append(problems, rp...)
	notices = append(notices, rn...)
	// Point-quality (I-08): unbacked verified/done
	// rows are a NOTICE, never a hard problem — this is a rendering/
	// visibility gap-closer, not a new lifecycle gate.
	notices = append(notices, qualityNotices(checkStreams)...)
	// UNRUN board state (absorbing F-impl-claims-unproven):
	// a verified/done brief closed over a risk-bearing Verify row that has no
	// completed Evidence row behind it. BLOCKING class — it reads brief files,
	// the same sources the board reads, and it can make a `done` row wrong.
	// Scoped to the TRANSITION: only a closure this branch made is a PROBLEM;
	// the pre-existing backlog surfaces as NOTICEs (see unrun.go's header for
	// why, and why that is not fail-open).
	unrunProblems, unrunNotices := unrunGateChecks(root, checkStreams)
	problems = append(problems, unrunProblems...)
	notices = append(notices, unrunNotices...)
	// Standing-alarm / flood NOTICEs (ISA-18.2): surface
	// findings past the age threshold (and register floods) so the desk/retro sees
	// them without a manual FINDINGS.md scan. Advisory only — never hard problems.
	notices = append(notices, standingAlarmNotices(findings, currentAlarmConfig(), nowFunc())...)
	// Finding→control closure NOTICEs (coder-skills-review/03): surface every
	// recurring-class finding whose bug class has not yet landed a permanent
	// control. Advisory-first — a NOTICE, never a hard PROBLEM, because a hard gate
	// over an unclassified backlog only manufactures false-positives. Resolves a
	// control's `<stream>/<NN>` brief reference against the FULL stream set (not
	// the product-scoped checkStreams) so the control lands regardless of scope.
	notices = append(notices, findingControlNotices(findings, streams, nowFunc())...)
	// Verification-debt alarm: the Awaiting queue
	// is the throughput valve — fire a NOTICE when depth crosses threshold
	// or exceeds the total done count.
	if n := debtNotice(checkStreams); n != "" {
		notices = append(notices, n)
	}
	// Intake untriaged-age alarm: surface untriaged intake
	// entries past the 3-day threshold. Offline/deterministic — keys on
	// disposition: new only and uses the in-git date frontmatter.
	intakeAlarmResult := IntakeAlarmResult{}
	intakeEntries, intakeErr := loadIntake(root)
	if intakeErr != nil {
		// Best-effort: a broken intake dir produces a diagnostic NOTICE but never
		// a hard failure — the board must still build. Crucially, mark the result
		// Unreadable so the ARTIFACT's intake line reads could-not-check rather
		// than "the front door is clear" — the NOTICE alone went to stderr while
		// the board contradicted it (docs/three-state-instrument-rule.md).
		intakeAlarmResult = IntakeAlarmResult{Unreadable: true, Reason: intakeErr.Error()}
		notices = append(notices, fmt.Sprintf("intake register COULD-NOT-CHECK: %v — the board's intake line reads could-not-check, not a clean front door", intakeErr))
	} else {
		intakeAlarmResult = intakeAlarm(intakeEntries, nowFunc())
		if n := intakeDebtNotice(intakeAlarmResult); n != "" {
			notices = append(notices, n)
		}
		// Surface per-entry bad-date NOTICEs (named ID + offending value) so a
		// typo'd date field never silently exempts an entry from the age alarm.
		notices = append(notices, intakeAlarmResult.BadDates...)
		// Surface decision-needed entries without a decision-issue.
		// Advisory only — the entry may have been flipped moments before the issue
		// is filed, but it must not stay that way.
		notices = append(notices, intakeDecisionIssueNotices(intakeEntries)...)
		// Root-level entry file coexisting with an already-split layout
		// (issue-loop/15): advisory only, names each file that has not yet
		// moved into its disposition subdir.
		notices = append(notices, intakeRootFileNotices(root, intakeEntries)...)
		// Surface scoped→<stream> entries whose target stream was never authored
		// and have aged past the threshold: the intake-desk tier gate only
		// QUEUES stream/brief authoring by flipping the entry — nothing drains the
		// queue, so a flip can dangle indefinitely with no alarm. Advisory only,
		// keyed on the full known-stream set so a target that IS a real stream never
		// fires. Uses `streams` (the full house set), not the product-scoped subset.
		notices = append(notices, intakeScopedUnauthoredNotices(intakeEntries, streams, nowFunc())...)
	}

	// BLOCKING problems short-circuit before the view is built — a broken source
	// set can't produce a meaningful Next-up. Notices still surface. Off-board
	// problems are reported here too, so a run that stops early still lists
	// everything it found (the "which checks ran" property).
	if len(problems) > 0 {
		emitNotices(notices)
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		for _, p := range offBoardProblems {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		fmt.Fprintln(os.Stderr, boardProvenanceLine(len(problems)+len(offBoardProblems)))
		finalVerdict(mode, len(problems)+len(offBoardProblems))
		return 1
	}

	// Build the generated view now — this also exercises applyFindings/nextUp/
	// emit, so --lint is a true superset of generation minus the STATUS.md byte
	// compare: a PR that would crash main's post-merge regen fails here instead.
	applyFindings(streams, findings)
	// The critical tier's reviewer-finding arm reads the same findings (phase 3).
	// Set explicitly every run so a prior invocation's value can never leak in; nil
	// is the inert default. It only takes effect when a drive is active (nextUp).
	activeFindings = findings
	for _, s := range streams {
		rel, _ := filepath.Rel(root, s.Dir)
		s.LastTouch = gitLastTouch(root, rel)
	}
	// Claim filtering answers the board's whole question — "what is unclaimed and
	// dispatchable". When the remote read fails, that answer is NOT available, and
	// the run says so (NOTICE + in-board banner below) instead of quietly emitting
	// the superset as if it had filtered.
	claimed, claimSource := resolveClaims(root, streams)
	// Per-brief staleness clock: read each brief's own
	// last recorded transition from the historian so aging measures from the
	// brief's history, not the stream's git touch. A missing/unreadable log just
	// yields an empty map — every brief then falls back to stream LastTouch.
	briefTouch := map[string]time.Time{}
	if entries, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		briefTouch = LastTransitionTime(entries)
	}
	// Stale-implemented alarm (F-impl-claims-unproven rec 3): an `implemented`
	// brief with none of
	// its Verify rows corroborated, aged past the threshold. Lives HERE rather
	// than in the check phase because it needs both the historian's per-brief
	// transition times and the git LastTouch fallback, neither of which is
	// populated until this point.
	notices = append(notices, staleImplementedNotices(checkStreams, briefTouch, nowFunc())...)
	// Drive campaign (methodology-metrics/45). Loaded from docs/roadmap/drives/
	// and wired into the nextUp package var BEFORE the board is built. loadDrives
	// NEVER returns an error: an absent directory is inert (byte-identical
	// baseline), and any malformed/expired/over-concurrent manifest degrades to
	// fail-neutral WARN lines here (surfaced as NOTICEs — exit 0, NEVER a PROBLEM;
	// a drive manifest must never be able to freeze the board). nowFunc() is the
	// ONE sanctioned wall-clock input (UTC-day granularity — see drives.go).
	activeDriveSet = loadDrives(root, streams, nowFunc())
	notices = append(notices, activeDriveSet.Warnings...)
	// The claim set travels WITH the record of whether it could be read: the
	// per-stream max-concurrent capping needs the difference between "nothing
	// is claimed" and "we could not look", and nu.Claims is set from the same
	// value rather than by a separate assignment a caller can forget.
	nu := nextUp(streams, ClaimView{Claimed: claimed, Source: claimSource}, briefTouch)
	// Drive anti-Goodhart coverage NOTICEs (a drive covering > threshold of the
	// eligible board self-taxes) surface on --lint too, not only in the artifact.
	notices = append(notices, nu.DriveCoverageNotices...)
	// Honesty gate (brief-44 Verify row 3): a boosted Next-up pick shown without
	// the active-drive banner is a PROBLEM (rc≠0) — the drive term must always be
	// displayed decomposed and attributed. nextUp sets the banner whenever a shown
	// pick is boosted, so this pins that invariant; it is NOT the fail-neutral
	// path (a malformed manifest applies zero boost and so has no boosted pick to
	// gate).
	if dp := driveBannerProblems(nu); len(dp) > 0 {
		emitNotices(notices)
		for _, p := range dp {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		fmt.Fprintln(os.Stderr, boardProvenanceLine(len(dp)))
		finalVerdict(mode, len(dp))
		return 1
	}
	// A degraded claim read is announced on stderr AND carried into the emitted
	// board by nu.Claims. --require-claims escalates it to a hard failure for
	// callers that must never dispatch from an unfiltered board.
	if n := claimSource.Notice(nu.Eligible); n != "" {
		notices = append(notices, n)
		if requireClaims {
			emitNotices(notices)
			fmt.Fprintln(os.Stderr, "PROBLEM:", "claim filtering could not be established and --require-claims is set: "+claimSource.Reason)
			fmt.Fprintln(os.Stderr, boardProvenanceLine(1))
			finalVerdict(mode, 1)
			return 1
		}
	}
	// Span-of-control overflow is a WIP-pressure alarm, surfaced as a --lint
	// NOTICE as well as an in-STATUS line.
	if nu.Overflow() {
		notices = append(notices, fmt.Sprintf(
			"Next-up overflow: %d of %d eligible shown, %d held back (%s) — WIP pressure",
			len(nu.Picks), nu.Eligible, nu.HeldBack(), heldBackReason(nu)))
	}
	// Could-not-check on a stream that asked to serialize: report it as its own
	// NOTICE. It is NOT the same condition as the board-wide degraded claim
	// read — that one says the board is a superset; this one says specific
	// streams are being withheld BECAUSE of it.
	if len(nu.SerializedUnknown) > 0 {
		notices = append(notices, fmt.Sprintf(
			"Next-up could-not-check: %d stream(s) declare max-concurrent (%s) but claim filtering did not run — "+
				"in-flight is unknowable, so they are held back to zero rather than offered unserialized",
			len(nu.SerializedUnknown), strings.Join(nu.SerializedUnknown, ", ")))
	}
	// Decision-surface check (the "top-of-Next-up" leg):
	// a gate:human brief PICKED for Next-up while still `todo` is about to be
	// dispatched into its human gate — surface the missing decision issue NOW,
	// before the wait turns invisible. Scoped to actual picks (not the whole
	// todo backlog) so the register is not flooded (review); dispatched/
	// awaiting statuses are covered status-wide in checkBriefFiles.
	notices = append(notices, nextUpDecisionNotices(nu)...)
	emitNotices(notices)
	// Off-board problems: reported in full, byte-identical to the
	// blocking path, and they still set the exit code below. Generation is NOT
	// suppressed by them — the banner says so out loud so nobody reads a written
	// STATUS.md as "the run passed".
	if len(offBoardProblems) > 0 {
		for _, p := range offBoardProblems {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		tail := "they do not suppress anything in this mode."
		if mode == "write" || mode == "record" {
			tail = "the generated views were regenerated rather than left stale, " +
				"because an off-board failure cannot make the board wrong."
		}
		fmt.Fprintf(os.Stderr,
			"statusgen: %d off-board problem(s) above are outside the board's own sources. "+
				"The run FAILS (exit 1) and they must still be "+
				"fixed — but %s\n",
			len(offBoardProblems), tail)
		fmt.Fprintln(os.Stderr, boardProvenanceLine(len(offBoardProblems)))
	}

	// Awaiting-age: render how long each awaiting row has sat in
	// its current status, from the historian. Best-effort: a missing/unreadable
	// log renders "—" everywhere, never an error — the board must still build.
	var ages map[string]string
	// entered is the RAW per-brief "entered its current awaiting status" time —
	// the same derivation the rendered ages come from, kept unrendered so the
	// per-stream age-at-gate metric can ORDER by it rather than by a display
	// string. A nil map (unreadable historian) is honest: ages render "—", never 0.
	var entered map[string]time.Time
	if hist, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		cur := make(map[string]string)
		for _, s := range streams {
			for _, b := range s.Briefs {
				if b.Status == "implemented" || b.Status == "verified" {
					cur[s.Name+"/"+b.Num] = b.Status
				}
			}
		}
		entered = awaitingEnteredAt(hist, cur)
		ages = awaitingAges(hist, cur, nowFunc())
	}
	// Age at the human gate (methodology-metrics/38) — computed OUTSIDE the
	// history guard on purpose: a stream with a brief at the human gate must
	// still appear on the board when the historian could not be read, wearing a
	// "—", rather than vanishing from the section as if nothing were waiting.
	gateAges := oldestHumanGateAges(streams, entered, nowFunc())

	// Drive dashboard render inputs (methodology-metrics phase 4): the active
	// drives' frontier/state plus the git-derived last-regen heartbeat. The
	// heartbeat is deterministic per committed tree (the committed STATUS.md
	// blob's provenance) — no fresh wall-clock read into the board bytes.
	activeDriveStatuses = driveStatuses(activeDriveSet, streams, claimed, nowFunc())
	activeDriveHeartbeat = driveHeartbeatLine(root)

	out := emit(streams, findings, nu, ages, gateAges, intakeAlarmResult, briefTouch, rootRepoName)
	if mode == "lint" {
		// The PR-side gate is UNCHANGED: off-board problems still fail --lint,
		// with the same count and the same exit code as before this split. Lint
		// never writes anything, so there was never a write to preserve here.
		finalVerdict(mode, len(offBoardProblems))
		if len(offBoardProblems) > 0 {
			return 1
		}
		return 0 // sources valid + view builds; STATUS.md never read or written
	}
	if mode == "record" {
		// single-writer append; never reached from --lint. The append happens
		// (the board sources passed every check that reads them), and the run
		// still exits 1 if anything off-board failed.
		if code := recordHistory(root, streams); code != 0 {
			return code
		}
		if len(offBoardProblems) > 0 {
			return 1
		}
		return 0
	}
	statusPath := filepath.Join(root, "STATUS.md")
	if mode == "check" {
		// The verdict counts DRIFT only — an off-board problem is not drift, and
		// folding it in would print "CHECK: DRIFT 2" over a byte-perfect
		// STATUS.md (reviewer N4). It still sets the exit code below.
		drift := 0
		existing, _ := os.ReadFile(statusPath)
		if string(existing) != out {
			fmt.Fprintln(os.Stderr, staleGeneratedFileMsg("STATUS.md", root))
			drift++
		}
		if rp := checkRegisterViews(root); len(rp) > 0 {
			for _, p := range rp {
				fmt.Fprintln(os.Stderr, "PROBLEM:", p)
			}
			drift += len(rp)
		}
		// A stale binary regenerates a different board, so drift ("this generated
		// file is out of date") is the most direct red a differently-versioned
		// binary produces — stamp which binary reported it (#186).
		if drift > 0 {
			fmt.Fprintln(os.Stderr, boardProvenanceLine(drift))
		}
		finalVerdict(mode, drift)
		if drift > 0 || len(offBoardProblems) > 0 {
			return 1
		}
		return 0
	}
	if err := os.WriteFile(statusPath, []byte(out), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println("wrote", statusPath)
	if err := writeRegisterViews(root); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println("wrote register views (INTAKE.md, FINDINGS.md)")
	if len(offBoardProblems) > 0 {
		return 1
	}
	return 0
}

// runRoots is the multi-root driver. It runs the SAME
// per-root pipeline (run) once per root, so every check and every derived datum
// — registers, historian, ages, counts, budget, links — comes from that root's
// own tree; nothing is ever computed from roots[0] and applied to the rest.
//
// Ordering: the cross-root pre-pass (duplicate stream names, duplicate repo:
// declarations) runs FIRST, and its blast radius is the COLLIDING roots only.
// Ambiguous ownership makes every downstream number attributable to the wrong
// repo — but only for the roots that actually clash, so those are quarantined
// (announced, and their boards deliberately left untouched) while every
// uninvolved root is boarded normally. Failing all N roots over a clash between
// two of them silently stops regenerating boards that are perfectly well-defined,
// and an un-regenerated board looks exactly as current as a fresh one. The run
// still exits non-zero: a quarantine is a failure, not a warning.
//
// Budget specs resolve PER ROOT: effectiveBudgetSpecs drops the auto-applied
// default when that root has no CLAUDE.md, so a second root without one is not
// red-gated by the first root's convention. An EXPLICIT --budget is honored
// against every root (a caller asking for a check gets it everywhere), never
// against the first one only.
//
// A single root takes the original path verbatim — same call, same verdict
// printing — so 1-arg behavior is byte-identical.
func runRoots(roots []string, mode string, budget []string, changed []string, scope string) int {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	if len(roots) == 1 {
		return run(roots[0], mode, effectiveBudgetSpecs(mode, budget, roots[0]), changed, scope)
	}

	crossProblems, quarantined := crossRootProblems(roots)
	for _, p := range crossProblems {
		fmt.Fprintln(os.Stderr, "PROBLEM:", p)
	}
	if len(crossProblems) > 0 {
		fmt.Fprintln(os.Stderr, boardProvenanceLine(len(crossProblems)))
	}

	// The cross-root problems are part of the ONE verdict line, exactly as the
	// per-root ones are — seeded into the accumulator before the loop so a
	// collision can never be masked by clean roots that follow it.
	total := len(crossProblems)
	verdictAccum = &total
	// Restored on EVERY exit from here, panic included. A panic mid-loop would
	// otherwise leave the accumulator dangling non-nil for the rest of the
	// process, and the next finalVerdict would silently swallow its line instead
	// of printing it.
	defer func() { verdictAccum = nil }()
	exit := 0
	if len(crossProblems) > 0 {
		exit = 1
	}
	covered := 0
	for _, root := range roots {
		// PROBLEM/NOTICE lines carry no root prefix (they are produced deep in
		// the per-check helpers), so the driver frames each root's block. Without
		// this a two-root run prints two undifferentiated streams of findings.
		// Printed BEFORE the root is attempted, so it enumerates configured roots
		// rather than successful ones — hence the coverage line after the loop.
		fmt.Fprintf(os.Stderr, "statusgen: === root %s ===\n", root)
		if quarantined[root] {
			// Named explicitly rather than skipped in silence: the whole point of
			// quarantining is that this root's board is now stale, and a stale
			// board is indistinguishable from a current one by inspection.
			fmt.Fprintln(os.Stderr, quarantinedRootMsg(root))
			continue
		}
		if code := run(root, mode, effectiveBudgetSpecs(mode, budget, root), changed, scope); code != 0 {
			exit = 1
			continue
		}
		covered++
	}
	// Say what was actually covered. The per-root banners above are printed
	// BEFORE each attempt, so on their own they report configured roots, not
	// successful ones — a partially-failed run is otherwise legible only by
	// counting banners against PROBLEM blocks. Worded as "without error" rather
	// than "covered" because run() returns non-zero both for a root it could not
	// load and for a root it read fine and found problems in; this line does not
	// pretend to tell those apart.
	fmt.Fprintf(os.Stderr, "statusgen: %d/%d root(s) completed without error\n", covered, len(roots))
	// Restores printing for the single line below. The deferred reset above is
	// the panic-path safety net, not this.
	verdictAccum = nil
	finalVerdict(mode, total)
	return exit
}

// runGateScores loads streams, computes the historian-backed gate scores for all
// awaiting briefs, and emits them as a JSON array.
// Each row: {brief, score, blockedCount, stream, status}. Sorted by score
// descending. STATUS.md-free — never reads or writes the generated board.
func runGateScores(root string) int {
	// loadHydratedStreams (not bare loadStreams + attachPlaceholders): the
	// gate score reads Brief.Value and Brief.Depends, both of which are
	// hydrated from brief-file frontmatter ONLY as a side effect of
	// checkBriefFiles. Skipping that step made --gate-scores drop the value
	// weight and the unblocks term (issue #266).
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	briefTouch := map[string]time.Time{}
	if entries, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		briefTouch = LastTransitionTime(entries)
	}
	gates := gateScores(streams, briefTouch)
	// The root's declared repo, carried on every row so a
	// cross-repo aggregator (deskboard nextup) can attribute a brief to its repo
	// from the DATA rather than from whichever root it happened to invoke. A
	// malformed/conflicting declaration is a hard PROBLEM in --lint; here it just
	// yields "" and the field is omitted.
	repo, _ := rootRepo(streams)
	// JSON rows for deskboard consumption.
	type row struct {
		Brief        string `json:"brief"`
		Score        int    `json:"score"`
		BlockedCount int    `json:"blockedCount"`
		Stream       string `json:"stream"`
		Status       string `json:"status"`
		Repo         string `json:"repo,omitempty"`
	}
	rows := make([]row, 0, len(gates))
	for _, g := range gates {
		rows = append(rows, row{
			Brief:        g.Stream.Name + "/" + g.Brief.Num,
			Score:        g.Score,
			BlockedCount: g.BlockedCount,
			Stream:       g.Stream.Name,
			Status:       g.Brief.Status,
			Repo:         repo,
		})
	}
	out, err := json.Marshal(rows)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(out))
	return 0
}

// emitNotices prints deduplicated, sorted NOTICE lines to stderr. Notices are
// advisory (exit 0); they never affect the return code.
func emitNotices(notices []string) {
	sort.Strings(notices)
	for _, n := range notices {
		fmt.Fprintln(os.Stderr, "NOTICE:", n)
	}
}

// budgetFlags is a repeatable --budget flag that accumulates "path:maxwords" specs.
type budgetFlags []string

func (b *budgetFlags) String() string { return strings.Join(*b, ", ") }
func (b *budgetFlags) Set(v string) error {
	*b = append(*b, v)
	return nil
}

// statusgenVersion is the release tag this binary was built from, stamped at
// link time by the release workflow:
//
//	go build -ldflags "-X main.statusgenVersion=statusgen/v0.3.0"
//
// It defaults to "dev" for a local `go build`/`go run`, which is the honest
// answer: an unstamped binary is not a pinned release.
//
// This exists so a consumer can CHECK the pin rather than assume it. Consumers
// pin statusgen by tag + sha256 in `.assay-versions`; until now nothing could
// ask a running binary which release it is, so a stale install produced a stale
// board with no signal.
var statusgenVersion = "dev"

func main() {
	// `statusgen --version` / `statusgen version` — pure introspection, answered
	// before flag parsing.
	//
	// Recognised as the SOLE argument only: `--version --lint` or
	// `--root . --version` fall through to the flag parser and are a usage error
	// (exit 2), because a version query combined with real work has no defined
	// meaning — do you get the version, the work, or both? Consumers checking a
	// pin invoke it alone, which is the form the pin-check contract needs.
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(statusgenVersion)
		os.Exit(0)
	}

	// `statusgen verifyrun` — positional subcommand (like `init` below) that
	// EXECUTES a brief's Verify rows and writes an execution witness into its
	// Evidence section, or audits an existing one (`--check`).
	//
	// Intercepted before flag parsing for the same reason `init` is: it owns its
	// own flag namespace. That matters more here than it does for `init` — the
	// parent parser already has a `--check` and a `--brief`, and verifyrun's
	// mean something else entirely. Sharing them would give one flag two
	// definitions, which is how a caller ends up auditing a witness they meant
	// to write.
	//
	// It is a WRITE-capable, subprocess-spawning sub-command, so it is never run
	// as part of `--lint`: the lint is offline and side-effect-free, and a check
	// that executes arbitrary commands lifted out of a markdown table has no
	// business inside it. `--lint` only NOTICEs the ABSENCE of a witness (see
	// witnessNotices); producing one is always an explicit invocation.
	if len(os.Args) > 1 && os.Args[1] == "verifyrun" {
		os.Exit(runVerifyrun(os.Args[2:], os.Stdout, os.Stderr))
	}

	// `statusgen mergecheck` — the MERGE-TIME RE-CHECK (desk-hardening/05, #54).
	// Re-asks "is this branch still correct?" against the TRIAL-MERGED tree rather
	// than the branch's own, which is the only tree that can show a semantic merge
	// collision (see mergecheck.go's header).
	//
	// Intercepted before flag parsing for the same reason `verifyrun` and `init`
	// are: it owns its own flag namespace, and its `--base`/`--head`/`--exec` mean
	// nothing to the parent parser.
	//
	// Never part of `--lint`. The lint is offline and side-effect-free; mergecheck
	// shells out to git, can fetch, and with --exec runs a command over an
	// extracted tree. `--lint` carries only the branch-local NOTICE half
	// (briefRuleNumberNotices), which is explicitly labelled as unable to see a
	// cross-branch collision.
	if len(os.Args) > 1 && os.Args[1] == "mergecheck" {
		os.Exit(runMergecheck(os.Args[2:], os.Stdout, os.Stderr))
	}

	// `statusgen shardcheck` — positional subcommand (like `verifyrun` above)
	// that decides whether a brief's declared `parallel-streams:` split may be
	// dispatched to concurrent workers (methodology/43, shardcheck.go).
	//
	// Intercepted before flag parsing for verifyrun's reason: it owns --brief
	// and --root with its own meanings, and it is a DISPATCH-TIME question
	// (does this split collide against the current tree?) rather than a board
	// question, so it has no business inside --lint. --lint validates only the
	// declaration's shape; this validates the plan against the files.
	if len(os.Args) > 1 && os.Args[1] == "shardcheck" {
		os.Exit(runShardcheck(os.Args[2:], os.Stdout, os.Stderr))
	}

	// `statusgen backfill` — the ONE-OFF status-historian replayer
	// (agentic-metrics/03, backfill.go). Walks each stream README's git history,
	// diffs each commit's status table through diffHistory, and PREPENDS the
	// reconstructed pre-seed transitions to docs/streams/.history.jsonl
	// (idempotent on {brief,to,sha}; live rows preserved byte-for-byte).
	//
	// Intercepted before flag parsing for verifyrun's reason: it owns its own
	// --root/--dry-run namespace. Emphatically NOT part of --lint/--record: the
	// lint is offline and side-effect-free, and this shells out to git across
	// the whole history and writes the log. It is a manual, one-time invocation,
	// never wired into CI — the live single-writer path (recordHistory) is the
	// only scheduled writer of the historian.
	if len(os.Args) > 1 && os.Args[1] == "backfill" {
		os.Exit(runBackfill(os.Args[2:], os.Stdout, os.Stderr))
	}

	// `statusgen init` — positional subcommand (like `git init`) that scaffolds the
	// streams structure into a repo. Intercepted before flag parsing so first-run
	// users get the natural `statusgen init` UX; `--root DIR` targets DIR.
	if len(os.Args) > 1 && os.Args[1] == "init" {
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		var initRoots rootFlags
		fs.Var(&initRoots, "root", "repository root to scaffold")
		fs.Parse(os.Args[2:])
		// Scaffolding is inherently one-repo-at-a-time; taking the first of
		// several silently would scaffold the wrong tree.
		resolved, err := resolveRoots(initRoots)
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			os.Exit(2)
		}
		if len(resolved) > 1 {
			fmt.Fprintf(os.Stderr, "statusgen: init accepts exactly one --root; got %d — scaffold one repo at a time\n", len(resolved))
			os.Exit(2)
		}
		os.Exit(runInit(resolved[0]))
	}

	// UNKNOWN POSITIONAL SUBCOMMAND — fail closed (#1075).
	//
	// Every genuine positional subcommand (verifyrun, mergecheck, shardcheck,
	// init; plus the sole-argument version/--version handled at the top) has been
	// intercepted above. What remains here as os.Args[1] is either a flag (begins
	// with "-", which the flag parser below owns) or a bare non-flag token — and a
	// bare non-flag token can now only be a misspelled or nonexistent subcommand.
	//
	// The default mode is regenerate, a WRITE. Letting an unrecognised token fall
	// through to it means `statusgen mergcheck` (typo) or `statusgen mergecheck`
	// against a binary too old to have the subcommand runs a tree-mutating
	// regenerate and reports success — the false-green shape
	// docs/three-state-instrument-rule.md forbids: a pass for a question never
	// asked. So we diagnose the unknown token and the valid set to stderr and exit
	// non-zero, writing nothing. Regenerate stays the default only for the
	// genuinely no-positional (flags-only) invocation.
	if len(os.Args) > 1 {
		first := os.Args[1]
		if first != "" && !strings.HasPrefix(first, "-") {
			fmt.Fprintf(os.Stderr, "statusgen: unknown subcommand %q\n", first)
			fmt.Fprintln(os.Stderr, "known subcommands: init, verifyrun, mergecheck, shardcheck, backfill, version")
			fmt.Fprintln(os.Stderr, "(for the default regenerate, pass flags only — e.g. --root DIR, --check, --lint)")
			os.Exit(2)
		}
	}

	// --root is REPEATABLE: each occurrence adds a root,
	// and the run emits one STATUS.md per root filtered to that root's streams.
	// Omitting it entirely keeps the historical default of ".".
	var roots rootFlags
	// The `(default ".")` is spelled out because flag.Var — unlike flag.String —
	// prints no default in usage output, and the default is unchanged.
	flag.Var(&roots, "root", `repository root (default "."; repeatable — one STATUS.md per root)`)
	checkMode := flag.Bool("check", false, "verify STATUS.md is current instead of writing it")
	lintMode := flag.Bool("lint", false, "run all checks without reading or writing STATUS.md (defaults --budget to "+defaultBudgetSpec+" unless overridden)")
	lintAuditMode := flag.Bool("lint-audit", false, "30-day check-firing audit (statusgen/01): sample daily commits, tally per-rule PROBLEM/NOTICE firings, flag COLD (0-firing, un-tested) rules as retirement candidates — read-only, advisory, never retires a rule")
	allowEmptyRootFlag := flag.Bool("allow-empty-root", false, "allow a root whose docs/streams exists but resolves to 0 streams (default: hard PROBLEM, same class as a missing/unreadable docs/streams); with this flag it downgrades to a NOTICE, for a root that has genuinely adopted the methodology but has not authored a stream yet")
	var budget budgetFlags
	flag.Var(&budget, "budget", "word-budget check: relpath:maxwords (repeatable); overrides --lint's default of "+defaultBudgetSpec)
	recordMode := flag.Bool("record", false, "append brief status transitions to docs/streams/.history.jsonl (main CI only)")
	verifyIssuesMode := flag.Bool("verify-issues", false, "emit JSON for newly-eligible verify-gate (gate:human + verified) briefs")
	existingMarkers := flag.String("existing-markers", "", "file of already-existing verify-gate issue markers (one per line, or raw issue bodies)")
	// Decision issues — self-contained sub-command that emits
	// JSON for gate:human briefs at implemented/verified that lack an open
	// needs-decision issue. Same STATUS.md-free discipline as verify-issues.
	decisionIssuesMode := flag.Bool("decision-issues", false, "emit JSON for newly-eligible needs-decision (gate:human + implemented/verified) briefs")
	decisionMarkers := flag.String("decision-markers", "", "file of already-existing decision-issue markers (one per line, or raw issue bodies)")
	// Drive lifecycle issues (methodology-metrics phase 2) — self-contained,
	// STATUS.md-free, offline. Emits the drive tracking issue + aged operator-act
	// issues + the @operator ping decision as JSON, same discipline as decision-issues.
	driveIssuesMode := flag.Bool("drive-issues", false, "emit JSON for active-drive tracking + aging operator-act issues (methodology-metrics drives phase 2)")
	// Board-freeze watchdog (methodology-metrics phase 4): the INDEPENDENT
	// out-of-band meta-alarm. Reads ONLY the heartbeat's freshness (STATUS.md
	// commit-time / mtime, NEVER content) and fails LOUD past 2× the regen
	// cadence: rc 1 + a BOARD FROZEN alarm + one board-freeze issue payload as
	// JSON. rc 0 = fresh (silent); rc 2 = could-not-check. Does NO board build,
	// so a board-build PROBLEM can neither abort nor silence it.
	watchdogMode := flag.Bool("watchdog", false, "board-freeze watchdog: alarm (rc 1 + JSON issue payload) when STATUS.md freshness exceeds 2× the regen cadence; does NO board build")
	driveMarkers := flag.String("drive-markers", "", "file of already-existing drive-issue markers (tracking/act/ping; one per line, or raw issue bodies/comments)")
	// Sign-off digest (methodology-metrics/38) — the BATCH view over
	// --verify-issues' per-brief cards: one body listing EVERY brief awaiting a
	// human sign-off, oldest-first, each with its recorded Evidence link. Same
	// STATUS.md-free, fully offline discipline as verify-issues. Surfacing only:
	// it reports, and never closes, flips, or nudges anything.
	signoffDigestMode := flag.Bool("signoff-digest", false, "print the human-gate sign-off digest: every brief awaiting a human sign-off, oldest-first, with Evidence links (exits non-zero with a could-not-check body when its inputs cannot be read — never an empty digest). With --json: a leak-safe aggregate (counts + oldest age only, no brief ids/titles) for the publish pipeline")
	// Issue scanner — self-contained sub-command that reads OPEN
	// issues across the fixed repo set (gh) and WRITES a placeholder brief per
	// unhandled one. Never invoked by --lint (no network dependency on the offline
	// gate); never pushes or mutates GitHub issues (read-only there).
	scanIssuesMode := flag.Bool("scan-issues", false, "emit placeholder briefs for unhandled OPEN issues across the fixed repo set (reads gh; writes files, never STATUS.md)")
	scanDryRun := flag.Bool("dry-run", false, "with --scan-issues / --transcribe-scan / --auto-flip-model: list what WOULD happen without writing (the --transcribe-scan `--check` surface)")
	// --transcribe-scan is the SAME-REPO scan transcriber (scan-lane/01, R-7): it
	// re-derives the issue-loop placeholder delta for the home repo directly on the
	// candidate tree, gated by the R-7 clause-1 trust predicate and enactment gate.
	// It ships INERT — it evaluates no clause until R-7's sign-off resolves. Unlike
	// --scan-issues it is a SERVER-SIDE lane, so it is NOT forced file-only: it reads
	// the roster from the CI transport when in Actions (scanClassForMode's default
	// branch), exactly like every mode except --scan-issues. --dry-run is its
	// no-write "--check" surface.
	transcribeScanMode := flag.Bool("transcribe-scan", false, "same-repo scan transcriber (R-7): re-derive the issue-loop placeholder delta on the candidate tree behind the clause-1 trust predicate + enactment gate; INERT until R-7 is signed. --dry-run = --check")
	// --transcribe-verdict is the VERIFY VERDICT transcriber (verdict-lane/03, R-6):
	// it sweeps open verifier-authored verdict issues and lands the byte-bounded R-6
	// delta (Evidence appends + model-tier status flips) on the candidate tree behind
	// the full clause battery — authorship (cl.1), RS256 signature + body-edit
	// timeline (cl.2), byte-bounds + irreversible/human-stamp refusal (cl.4–5),
	// check:ci network-off re-execution (cl.6), the cl.9 flood tripwire and the cl.10
	// main-health hold — and the R-6 enactment gate. It ships INERT: it evaluates no
	// clause until R-6's sign-off resolves. Like --transcribe-scan it is a server-side
	// lane (not forced file-only) and --dry-run is its no-write "--check" surface.
	transcribeVerdictMode := flag.Bool("transcribe-verdict", false, "verify verdict transcriber (R-6): land the Evidence-append + model-tier-flip delta from signed verifier verdict issues on the candidate tree behind authorship + RS256 signature + check:ci re-execution + the enactment gate; INERT until R-6 is signed. --dry-run = --check")
	verdictPubkey := flag.String("pubkey", "", "--transcribe-verdict: verifier public-key PEM path; falls back to the ASSAY_VERIFIER_PUBKEY variable (PEM or base64-of-PEM)")
	closeVerifyID := flag.String("close-verify", "", "flip <stream>/<NN> verified→done with a human:<name> sign-off (refuses if not verified/gate:human)")
	// Model-path auto-flip (methodology-metrics/39). The gate:human counterpart
	// is --close-verify above, and the two never meet: this mode's candidate
	// filter is `gate: model` and nothing else. There is deliberately NO
	// companion flag that skips the SHA corroboration — see autoflip.go.
	autoFlipModelMode := flag.Bool("auto-flip-model", false, "flip gate:model briefs verified→done from the reviewer App's APPROVED review at the merged head (records PR#+SHA; refuses on any SHA mismatch; never touches gate:human)")
	registerLinksFlag := flag.Bool("register-links", false, "backfill: rewrite bare F-NN/I-NN tokens in brief files to linked form")
	span := flag.Int("span", defaultSpanOfControl, "Next-up span-of-control cap: max items shown (default 20 — agent-worked queue, not the human EEMUA-191 7±2)")
	overflowT := flag.Int("overflow-threshold", -1, "eligible-brief count above which Next-up flags overflow; <0 = same as --span")
	requireClaimsFlag := flag.Bool("require-claims", false, "fail (exit 1, nothing written) instead of emitting a degraded board when the origin claim read fails")
	// FINDINGS alarm-KPI knobs (ISA-18.2). Standalone
	// block so sibling statusgen flag PRs merge trivially.
	alarmsMode := flag.Bool("alarms", false, "print FINDINGS alarm KPIs (rate, standing-alarm age, flood) and exit")
	standingAge := flag.Int("standing-age-days", defaultStandingAgeDays, "standing-alarm age threshold in days (default ~1 retro-cycle)")
	flood := flag.Int("flood-threshold", defaultFloodThreshold, "active-unresolved finding count above which the register floods (ISA-18.2)")
	// Methodology metric emitters retained after the oss-replacement/06 DevLake
	// split (Ian ruling #1213, DevLake feeds INTO the retained roadmap pages).
	// The standalone commodity DORA/velocity/code-efficiency CLI surface
	// (--dora/--dora-series/--trend/--code) was removed; the grouped-DORA core
	// the roadmap consumes lives in roadmapdora.go. --autonomy/--issues/--cynefin
	// are newer methodology modes and are retained. Shared knobs
	// (--since/--json/--series/--weekly/--daily/--history) are declared once here.
	// BACK-COMPAT NOTE: --dora and --trend were later re-added below as aliases
	// (--trend == --verif-backlog; --dora re-emits the retained grouped-DORA core)
	// because the pinned daily-harvest/v0.1.0 collector still calls them; --dora-series
	// and --code stay removed (no caller). See the alias block after --history.
	verifBacklogMode := flag.Bool("verif-backlog", false, "roll the status-transition log up into the awaiting-verification backlog curve (impl+verif standing count over time; does not read/write STATUS.md)")
	autonomyMode := flag.Bool("autonomy", false, "emit the step-3 adoption-ladder gauges (autonomy ratio ×2 variants, token efficiency, deterministic-gate share) as a system; reuses --since / --json; diagnostic, never a target or per-person scorecard")
	ladderMode := flag.Bool("ladder", false, "emit the adoption-ladder POSITION indicator (mm/42): one computed step 0–4 from behavioral axes (autonomy ratio, gate share, dispatch autonomy, token efficiency) + the binding-constraint axis; degrades to an explicit 'unmeasured range' (never a silent zero) when the private mm/40 opmetrics day-file is absent, so it ships publicly; reuses --since / --json; diagnostic, never a target or per-person scorecard")
	issuesMode := flag.Bool("issues", false, "emit issue metrics (counts, age/sitting-time, internal-vs-external, by-raising-desk) as a system; diagnostic, not a target")
	staleIssueDays := flag.Int("stale-issue-days", defaultStaleIssueDays, "--issues/--lint: age in days past which an open issue trips the stale-issue alarm (default 7)")
	teamLogins := flag.String("team-logins", "", "--issues: extra comma-separated team/internal logins beyond the roster trusted logins + bots")
	cynefinMode := flag.Bool("cynefin", false, "classify active work by Cynefin domain (clear/complicated/complex/chaotic): distribution, drift, and a Disorder list of untagged briefs; reuses --json / --weekly / --daily (does not read/write STATUS.md)")
	doraJSON := flag.Bool("json", false, "machine-readable JSON output. Used with --issues / --autonomy / --ladder / --cynefin / --bottleneck / --intake-debt")
	doraSeries := flag.Bool("series", false, "time series (per-period buckets) instead of a single aggregate. Used with --issues")
	since := flag.String("since", "", "period start (YYYY-MM-DD) for --verif-backlog / --autonomy / --ladder / --issues")
	weekly := flag.Bool("weekly", false, "bucket by ISO week (default) for --verif-backlog / --cynefin")
	daily := flag.Bool("daily", false, "bucket by day for --verif-backlog / --cynefin")
	historyPath := flag.String("history", "", "history log path (default docs/streams/.history.jsonl, relative to --root) for --verif-backlog")
	// BACK-COMPAT ALIASES (v0.14.0 regression fix). The pinned daily-harvest/v0.1.0
	// collector still shells out to `statusgen -dora` and `statusgen -trend`, which
	// v0.14.0 removed — so daily-harvest dies on "flag provided but not defined" for
	// every consumer. Re-add both so old callers work unchanged; the new flags stay
	// primary.
	//   --trend  == --verif-backlog: the historian roll-up runTrend was renamed
	//               runVerifBacklog (same signature, same awaiting-verification
	//               backlog curve). A pure alias.
	//   --dora   emits the grouped-DORA core (roadmapdora.go/computeDoraGrouped) that
	//               survived the DevLake split (Ian #1213). The standalone DORA CLI
	//               was removed; --dora is re-exposed over the retained computation
	//               (see doracli.go), reusing --since / --json.
	doraMode := flag.Bool("dora", false, "back-compat alias (daily-harvest/v0.1.0): emit grouped-DORA metrics (per-stream throughput+instability) from the historian; the standalone DORA CLI moved to DevLake (Ian #1213). Reuses --since / --json / --by")
	doraBy := flag.String("by", "stream", "--dora grouping dimension: stream | goal")
	trendMode := flag.Bool("trend", false, "back-compat alias for --verif-backlog (daily-harvest/v0.1.0): roll the status-transition log up into the awaiting-verification backlog curve. Reuses --since / --daily / --weekly / --history")
	// --roadmap: the internal roadmap-deck overview + per-stream pages. RETAINED
	// (Ian ruling #1213): DevLake feeds INTO these pages; the grouped-DORA tile
	// they render is computed in roadmapdora.go.
	roadmapMode := flag.Bool("roadmap", false, "render the roadmap deck overview page (docs/reports/roadmap/index.html)")
	// Corroboration check: self-contained sub-command,
	// same STATUS.md-free discipline as the verify-gate modes. Network-dependent
	// by nature — never wired into the offline lint gate.
	consumersMode := flag.Bool("consumers", false, "corroborate the consumers: routing claims of the briefs this branch touches against its own diff (exit 1 = a claim is DISPROVED, 2 = the diff could not be taken)")
	consumersBase := flag.String("base", remoteMainRef, "--consumers: base ref for the three-dot diff")
	consumersBrief := flag.String("brief", "", "--consumers: check only this brief (<stream>/<NN>); default = every brief file in the diff")
	corroborateMode := flag.Bool("corroborate", false, "check human:<name> stamps against PR reviews/comments for corroboration (requires --pr)")
	corroboratePRs := flag.String("pr", "", "comma-separated PR numbers (required with --corroborate)")
	// Daily factory-floor bottleneck report: per-stage
	// WIP + dwell, constraint location, shift detection, prescribed ToC action.
	// Self-contained diagnostic sub-command — never reads or writes STATUS.md.
	bottleneckMode := flag.Bool("bottleneck", false, "emit the daily factory-floor bottleneck report (per-stage WIP + dwell, constraint, shift, action). With --json: a side-effect-free machine-readable emitter (no dated file written) for the publish pipeline")
	intakeDebtMode := flag.Bool("intake-debt", false, "emit the intake front-door debt aggregate (untriaged count, over-threshold, oldest age). With --json: a leak-safe counts-only object for the publish pipeline (no entry ids/dates)")
	launchMode := flag.Bool("launch", false, "print launch-readiness rollup — transitive depends: of the go-live gate (assay-launch/05) with live status (never reads/writes STATUS.md)")
	launchTarget := flag.String("launch-target", "assay-launch/05", "target brief for --launch (default assay-launch/05)")
	// Evidence-bundle export (gtm/05): deterministic tarball of briefs, registers,
	// and Evidence blocks in a date range, with a generated manifest.json.
	// Takes two positional args <from> <to> (YYYY-MM-DD) and requires -o <path>.
	// -o is parsed from flag.Args() because positional args stop flag parsing.
	exportEvidenceMode := flag.Bool("export-evidence", false, "export an evidence bundle tarball for the given date range (positional <from> <to>; -o <path>; optional -generated <RFC3339> for byte-reproducible output)")
	// Gate-score emitter: JSON for deskboard consumption.
	gateScoresMode := flag.Bool("gate-scores", false, "emit awaiting-queue gate scores as JSON (brief, score, blockedCount, stream, status)")
	// Next-up (DISPATCH queue) emitter: JSON for deskboard consumption. Distinct
	// population from --gate-scores — the same claim-filtered, capped nextUp()
	// selection the STATUS.md board shows (todo/in-progress, unclaimed, eligible),
	// plus the held-back decomposition. Reuses --span / --overflow-threshold /
	// --require-claims.
	nextUpMode := flag.Bool("next-up", false, "emit the DISPATCH queue as JSON: the claim-filtered, capped Next-up selection (todo/in-progress, unclaimed, eligible) plus the held-back decomposition (eligible/shown/heldByStreamCap/heldBySpan/claimsKnown). NOT --gate-scores, which is the awaiting-verification backlog")
	// Gate-effectiveness telemetry: override rate, catch
	// rate, ceremonial-gate detection. Self-contained diagnostic sub-command,
	// same STATUS.md-free discipline as --dora/--trend/--bottleneck. --root
	// points at ONE window's fixture/data directory (see gatetelemetry.go).
	gateTelemetryMode := flag.Bool("gate-telemetry", false, "emit gate-effectiveness telemetry (override rate, catch rate, ceremonial-gate detection) for one window's --root")
	// Opt-in fleet-drift telemetry (gtm/08): anonymized, counts-only, OFF BY
	// DEFAULT. Armed ONLY when --telemetry is passed AND ASSAY_TELEMETRY=1 is in
	// the environment (double opt-in — no CI vendor default can flip it). When
	// armed, an ordinary --lint/write run prints and would send the payload;
	// --telemetry-dry-run prints it and never sends. See telemetry.go / docs/telemetry.md.
	telemetryMode := flag.Bool("telemetry", false, "opt in to anonymized, counts-only statusgen telemetry (also requires ASSAY_TELEMETRY=1 in the environment); OFF by default")
	telemetryDryRun := flag.Bool("telemetry-dry-run", false, "with --telemetry and ASSAY_TELEMETRY=1, print the telemetry payload and never send it")
	// Product-scoping. --changed: a file of changed
	// repo-relative paths (one per line, CI passes the PR diff) — path-scopes the
	// DAR check (31) and auto-derives the product scope (32). --scope: an explicit
	// product override (serves:), skipping auto-derivation. Both apply to --lint;
	// absent = today's whole-house behavior (main regen never passes them).
	changedFile := flag.String("changed", "", "file of changed repo-relative paths (one per line); path-scopes the DAR check and auto-derives --scope")
	scopeFlag := flag.String("scope", "", "restrict per-stream lint to one product (serves:): example-app|example-service|assay|platform; overrides --changed derivation")
	flag.Parse()

	// The roster class is chosen ONCE, from the invoked mode, BEFORE any check can
	// read the roster — scanEffectiveConfig caches on first use, so a later call
	// would be ignored. See scanClassForMode: --scan-issues is the acting mode and
	// stays file-only; every other mode reads the CI transport when, and only when,
	// it is actually running in a GitHub Actions job.
	scanSetToolClass(scanClassForMode(*scanIssuesMode))

	// P3: echo the effective roster ONCE per run, for EVERY mode, on stderr.
	//
	// It was previously echoed only from runScanIssues, so the mode that actually
	// runs in the consumer's CI — --lint — printed nothing about the configuration
	// it was judging with. That is the mode where an invisible roster does the most
	// damage: the human-login map decides whether a Verified cell clears the
	// verifier floor, and an unreadable map turns a passing brief into a PROBLEM on
	// a PR that never touched it. Whoever reads that CI log needs the roster in the
	// same log.
	scanEchoEffectiveConfig(os.Stderr)

	resolvedRoots, rootErr := resolveRoots(roots)
	if rootErr != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", rootErr)
		os.Exit(2)
	}
	// `root` is the SINGLE-root view, used by every self-contained sub-command
	// below. Those sub-commands are one-repo diagnostics (a DORA window, a
	// historian trend, one repo's gate scores); none of them has multi-root
	// semantics today, so multi-root is REFUSED rather than narrowed to the first
	// root. Silently narrowing is the exact failure this brief exists to prevent
	// — a whole repo's work disappearing from a board with no signal
	// (a lesson from a security review). Run them once per root.
	root := &resolvedRoots[0]
	if len(resolvedRoots) > 1 {
		if name := singleRootOnlySubcommand(map[string]bool{
			"--verify-issues":      *verifyIssuesMode,
			"--decision-issues":    *decisionIssuesMode,
			"--drive-issues":       *driveIssuesMode,
			"--signoff-digest":     *signoffDigestMode,
			"--scan-issues":        *scanIssuesMode,
			"--transcribe-scan":    *transcribeScanMode,
			"--transcribe-verdict": *transcribeVerdictMode,
			"--close-verify":       *closeVerifyID != "",
			"--auto-flip-model":    *autoFlipModelMode,
			"--alarms":             *alarmsMode,
			"--verif-backlog":      *verifBacklogMode,
			"--trend":              *trendMode, // back-compat alias of --verif-backlog
			"--dora":               *doraMode,
			"--autonomy":           *autonomyMode,
			"--ladder":             *ladderMode,
			"--issues":             *issuesMode,
			"--cynefin":            *cynefinMode,
			"--intake-debt":        *intakeDebtMode,
			"--roadmap":            *roadmapMode,
			"--bottleneck":         *bottleneckMode,
			"--launch":             *launchMode,
			"--export-evidence":    *exportEvidenceMode,
			"--gate-scores":        *gateScoresMode,
			"--next-up":            *nextUpMode,
			"--register-links":     *registerLinksFlag,
			"--gate-telemetry":     *gateTelemetryMode,
			"--telemetry-dry-run":  *telemetryDryRun,
			// --consumers takes ONE git diff, against one root's HEAD. Narrowing
			// to the first root corroborates one repo's claims and reports
			// the others clean, unread.
			"--consumers": *consumersMode,
		}); name != "" {
			fmt.Fprintf(os.Stderr,
				"statusgen: %s accepts exactly one --root; got %d. Run it once per root — no sub-command narrows to the first root.\n",
				name, len(resolvedRoots))
			os.Exit(2)
		}
	}

	// Wire the alarm-KPI thresholds before any run — the
	// --lint standing-alarm NOTICE and the --alarms view both read these.
	if *standingAge >= 0 {
		standingAgeDays = *standingAge
	}
	if *flood >= 0 {
		floodThreshold = *flood
	}
	// Wire the stale-issue threshold before any run — the --lint stale-issue
	// NOTICE reads it (the --issues emitter takes the flag value directly).
	if *staleIssueDays >= 0 {
		staleIssueDaysCfg = *staleIssueDays
	}

	// Wire the Next-up span-of-control knobs before any
	// run. The overflow threshold defaults to the span cap: overflow == more
	// eligible briefs than the span shows.
	if *span > 0 {
		spanOfControl = *span
	}
	if *overflowT >= 0 {
		overflowThreshold = *overflowT
	} else {
		overflowThreshold = spanOfControl
	}
	// Fail-closed opt-in for claim filtering. Default off:
	// the board must still render, wearing its degradation. A caller that
	// dispatches from the board sets this and gets exit 1 instead.
	requireClaims = *requireClaimsFlag
	// Fail-closed opt-in for a zero-stream root. Default off: a
	// root that resolves to 0 streams is a hard PROBLEM, matching the three
	// adjacent cases (missing/unreadable docs/streams, nonexistent root) that
	// already fail closed.
	allowEmptyRoot = *allowEmptyRootFlag

	// Budget flags: validate spec syntax early — malformed
	// specs are a usage error and should exit before any checks run.
	budgetSpecs := []string(budget)
	for _, spec := range budgetSpecs {
		if _, _, err := parseBudgetSpec(spec); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			os.Exit(2)
		}
	}

	// verify-gate modes are self-contained sub-commands: they do not read or
	// write STATUS.md and never run the full source-check suite.
	if *verifyIssuesMode {
		os.Exit(runVerifyIssues(*root, *existingMarkers))
	}

	// Decision issues: self-contained, same STATUS.md-free
	// discipline as verify-issues. Emits JSON for gate:human briefs at
	// implemented/verified lacking an open needs-decision issue.
	if *decisionIssuesMode {
		os.Exit(runDecisionIssues(*root, *decisionMarkers))
	}
	// Drive lifecycle issues (methodology-metrics phase 2): self-contained, same
	// STATUS.md-free discipline as decision-issues. Emits the active-drive tracking
	// issue + aged operator-act issues + the @operator ping decision as JSON. A
	// malformed/expired manifest is fail-neutral (NOTICE + empty array), never an
	// error — a drive can no more fail this command than it can freeze the board.
	if *driveIssuesMode {
		os.Exit(runDriveIssues(*root, *driveMarkers))
	}
	// Board-freeze watchdog (methodology-metrics phase 4): self-contained,
	// STATUS.md-free, offline. It performs NO board build — the whole point is
	// that the alarm must not share the failure mode it detects. Its fail-loud
	// polarity (rc 1 + alarm + issue) is deliberate and out-of-band: it alarms
	// without aborting any board write, so it can never itself become the freeze
	// it detects.
	if *watchdogMode {
		os.Exit(runWatchdog(*root))
	}
	// Sign-off digest: the roll-up over the per-brief cards. Self-contained,
	// STATUS.md-free, offline. Non-zero exit means could-not-check — never an
	// empty digest standing in for an all-clear.
	if *signoffDigestMode {
		os.Exit(runSignoffDigest(*root, *doraJSON))
	}
	// Issue scanner: self-contained, same STATUS.md-free discipline
	// as the verify-gate modes. READ-only against GitHub — it lists issues and
	// writes local placeholder files; it never creates or mutates an issue.
	if *scanIssuesMode {
		os.Exit(runScanIssues(*root, *scanDryRun, ghIssueLister, issueCommentLister, ghIssueBlessChecker))
	}
	// Same-repo scan transcriber (scan-lane/01, R-7): self-contained,
	// STATUS.md-free. The workflow's "run" step. INERT until the R-7 sign-off
	// resolves; --dry-run is the no-write "--check" surface.
	if *transcribeScanMode {
		os.Exit(runTranscribeScan(*root, *scanDryRun,
			ghIssueLister, issueCommentLister, ghAuthorResolver, ghIssueBlessChecker, ghCommentResolver))
	}
	// Verify verdict transcriber (verdict-lane/03, R-6): self-contained,
	// STATUS.md-free. The workflow's "run" step. INERT until the R-6 sign-off
	// resolves; --dry-run is the no-write "--check" surface.
	if *transcribeVerdictMode {
		os.Exit(runTranscribeVerdict(*root, *scanDryRun, *verdictPubkey,
			ghIssueLister, ghVerdictIssueResolver, hermeticCheckCIRunner,
			ghVerdictMainHealth(*root), ghCommentResolver))
	}
	if *closeVerifyID != "" {
		os.Exit(runCloseVerify(*root, *closeVerifyID))
	}
	// Model-path auto-flip: same self-contained, STATUS.md-free discipline as
	// the verify-gate modes. Reads gh (reviews) and git (history); writes only
	// stream README rows, and only for gate:model briefs whose merge PR carries
	// a live App approval at the merged head.
	if *autoFlipModelMode {
		os.Exit(runAutoFlipModel(*root, *scanDryRun))
	}
	if *alarmsMode {
		os.Exit(runAlarms(*root))
	}
	// Awaiting-verification backlog curve — self-contained sub-command, same
	// STATUS.md-free discipline as the verify-gate modes above. This is the
	// METHODOLOGY metric rehomed out of the split-out trend.go
	// (oss-replacement/06): the standing lead-time-debt count over time. The
	// commodity DORA/velocity/code-efficiency CLI surface that used to sit beside
	// it is now DevLake's; the grouped-DORA core the roadmap consumes stays in
	// roadmapdora.go (Ian ruling #1213 — DevLake feeds INTO the retained roadmap).
	// --trend is the v0.14.0 back-compat alias of --verif-backlog (daily-harvest/
	// v0.1.0 still calls `statusgen -trend`); it runs the identical handler.
	if *verifBacklogMode || *trendMode {
		period := "weekly"
		if *daily {
			period = "daily"
		}
		if *weekly {
			period = "weekly"
		}
		os.Exit(runVerifBacklog(*root, *historyPath, *since, period))
	}
	// --dora is the v0.14.0 back-compat alias (daily-harvest/v0.1.0 still calls
	// `statusgen -dora`): it emits the grouped-DORA core retained in roadmapdora.go.
	// Self-contained diagnostic sub-command — never reads or writes STATUS.md.
	if *doraMode {
		os.Exit(runDora(*root, *since, strings.ToLower(strings.TrimSpace(*doraBy)), *doraJSON))
	}
	// Autonomy / token / gate-share emitter (mm/41) — self-contained
	// diagnostic sub-command. Reuses the shared --since window and --json flag.
	if *autonomyMode {
		os.Exit(runAutonomy(*root, *since, *doraJSON))
	}
	// Adoption-ladder POSITION indicator (mm/42) — self-contained diagnostic
	// sub-command built on the same behavioral axes as --autonomy. Ships
	// publicly: a missing private opmetrics day-file degrades to an explicit
	// 'unmeasured range', never an error and never a silent zero.
	if *ladderMode {
		os.Exit(runLadder(*root, *since, *doraJSON))
	}
	// Issue metrics emitter — self-contained sub-command, same
	// STATUS.md-free discipline as the modes above.
	if *issuesMode {
		os.Exit(runIssues(*root, *doraJSON, *doraSeries, *staleIssueDays, *teamLogins))
	}
	// Cynefin-domain view: self-contained diagnostic sub-command — domain
	// distribution, drift, and the Disorder list. Same STATUS.md-free discipline
	// as --dora/--trend; a lens, never a gate.
	if *cynefinMode {
		period := "weekly"
		if *daily {
			period = "daily"
		}
		if *weekly {
			period = "weekly"
		}
		os.Exit(runCynefin(*root, period, *doraJSON))
	}
	if *corroborateMode {
		os.Exit(runCorroborate(*corroboratePRs))
	}
	// consumers corroboration — self-contained
	// sub-command; never reads or writes STATUS.md. Exits 1 when the branch diff
	// DISPROVES a routing claim, 2 when the diff itself could not be taken.
	if *consumersMode {
		os.Exit(runConsumers(*root, *consumersBase, *consumersBrief))
	}
	// Bottleneck report: self-contained diagnostic
	// sub-command — per-stage WIP + dwell, constraint location, shift detection,
	// prescribed ToC action. Same STATUS.md-free discipline as --dora/--trend.
	if *bottleneckMode {
		os.Exit(runBottleneck(*root, *doraJSON))
	}
	// Intake front-door debt emitter (agentic-metrics/02): self-contained,
	// offline, STATUS.md-free — same discipline as --bottleneck --json.
	if *intakeDebtMode {
		os.Exit(runIntakeDebt(*root, *doraJSON))
	}
	// Roadmap deck overview + per-stream pages — RETAINED (Ian ruling #1213).
	// DevLake feeds INTO these internal pages; roadmapdora.go computes the DORA
	// tiles they render today.
	if *roadmapMode {
		os.Exit(runRoadmap(*root))
	}
	// Launch readiness rollup (assay-launch/04): self-contained diagnostic sub-command,
	// same STATUS.md-free discipline as --dora/--trend/--roadmap.
	if *launchMode {
		os.Exit(runLaunch(*root, *launchTarget))
	}
	// Gate-score emitter: self-contained JSON output for
	// deskboard consumption. Same STATUS.md-free discipline as --dora/--trend.
	if *gateScoresMode {
		os.Exit(runGateScores(*root))
	}
	// Next-up (DISPATCH queue) emitter: self-contained JSON output for deskboard
	// consumption. Same STATUS.md-free discipline as --gate-scores; the span/overflow
	// and require-claims knobs are already wired above.
	if *nextUpMode {
		os.Exit(runNextUp(*root))
	}
	// Gate-effectiveness telemetry: self-contained,
	// STATUS.md-free, same discipline as --dora/--trend/--bottleneck above.
	if *gateTelemetryMode {
		os.Exit(runGateTelemetry(*root))
	}
	// Opt-in telemetry dry-run (gtm/08): self-contained, STATUS.md-free preview
	// of the anonymized payload. Requires the full double opt-in even to PREVIEW,
	// so the arming rule is uniform and there is a single, testable "off" state.
	// Never sends.
	if *telemetryDryRun {
		if !telemetryArmed(*telemetryMode) {
			fmt.Fprintf(os.Stderr,
				"telemetry: not armed — pass --telemetry AND set %s=1 to preview the payload. Nothing collected.\n",
				telemetryEnvVar)
			os.Exit(0)
		}
		runTelemetry(*root, true)
		os.Exit(0)
	}
	// Evidence-bundle export (gtm/05): self-contained sub-command, same
	// STATUS.md-free discipline as --dora/--trend. Single-root only.
	// -o is parsed from flag.Args() because the Go flag package stops
	// flag parsing at the first non-flag (positional) argument.
	if *exportEvidenceMode {
		args := flag.Args()
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "statusgen: --export-evidence requires two positional arguments: <from> <to> (YYYY-MM-DD)")
			os.Exit(2)
		}
		from, to := args[0], args[1]
		output := ""
		generatedArg := ""
		for i := 2; i+1 < len(args); i++ {
			switch args[i] {
			case "-o":
				output = args[i+1]
			case "-generated":
				// Optional explicit manifest timestamp. Supplying it makes the
				// bundle byte-reproducible (brief-05: "generation timestamp
				// passed in — statusgen stays deterministic"); omitting it uses
				// the clock, so only manifest.generated differs between runs.
				generatedArg = args[i+1]
			}
		}
		if output == "" {
			fmt.Fprintln(os.Stderr, "statusgen: --export-evidence requires -o <output>")
			os.Exit(2)
		}
		var generated time.Time
		if generatedArg != "" {
			g, err := time.Parse(time.RFC3339, generatedArg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "statusgen: -generated must be RFC3339 (e.g. 2026-08-03T00:00:00Z), got %q\n", generatedArg)
				os.Exit(2)
			}
			generated = g
		}
		os.Exit(runEvidenceExport(*root, from, to, output, generated))
	}
	if *registerLinksFlag {
		n, err := backfillRegisterRefs(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "register-links:", err)
			os.Exit(1)
		}
		fmt.Printf("register-links: %d bare references linked across brief files\n", n)
		return
	}
	// Lint-audit (statusgen/01) — read-only advisory sub-command: sample daily commits
	// over 30 days, tally per-rule firing counts, flag COLD rules. Never retires a
	// rule itself and never gates CI.
	if *lintAuditMode {
		os.Exit(runLintAudit(*root))
	}

	mode := "write"
	switch {
	case *lintMode:
		mode = "lint"
	case *checkMode:
		mode = "check"
	case *recordMode:
		mode = "record"
	}
	// Budget specs are resolved PER ROOT inside runRoots (bare --lint defaults to
	// the same spec CI enforces, and drops itself on a root with no
	// CLAUDE.md) — resolving here would apply the first root's answer to all of
	// them.

	// Product-scoping inputs. --scope must name a known product; an
	// unknown value is a usage error, not a silent no-op.
	if *scopeFlag != "" && !validServes[*scopeFlag] {
		fmt.Fprintf(os.Stderr, "statusgen: --scope %q is not a known product (example-app|example-service|assay|platform)\n", *scopeFlag)
		os.Exit(2)
	}
	var changedPaths []string
	if *changedFile != "" {
		raw, err := os.ReadFile(*changedFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen: --changed:", err)
			os.Exit(2)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if p := strings.TrimSpace(line); p != "" {
				changedPaths = append(changedPaths, p)
			}
		}
	}
	code := runRoots(resolvedRoots, mode, budgetSpecs, changedPaths, *scopeFlag)
	// Opt-in telemetry (gtm/08): only after an ordinary lint/write run, and only
	// when armed by the double opt-in. Each root emits its own anonymized,
	// counts-only payload; telemetry never changes the run's exit code (a
	// collection/send failure is reported and swallowed inside runTelemetry).
	if telemetryArmed(*telemetryMode) {
		for i := range resolvedRoots {
			runTelemetry(resolvedRoots[i], false)
		}
	} else if *telemetryMode {
		// --telemetry given but ASSAY_TELEMETRY!=1: telemetry stays OFF. Say so
		// once, so the second, deliberate switch is discoverable rather than a
		// silent no-op.
		fmt.Fprintf(os.Stderr,
			"telemetry: --telemetry given but %s is not \"1\" — telemetry stays OFF (both are required).\n",
			telemetryEnvVar)
	}
	os.Exit(code)
}

// singleRootOnlySubcommand returns the flag name of the first ENABLED sub-command
// that has no multi-root semantics, or "" when none is enabled. Iteration is over
// a sorted key list so the refusal message is deterministic when a caller enables
// two at once.
func singleRootOnlySubcommand(enabled map[string]bool) string {
	names := make([]string, 0, len(enabled))
	for name := range enabled {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if enabled[name] {
			return name
		}
	}
	return ""
}
