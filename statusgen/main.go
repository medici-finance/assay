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
	briefProblems, briefNotices := checkBriefFiles(checkStreams)
	problems = append(problems, briefProblems...)
	notices = append(notices, briefNotices...)
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
	notices = append(notices, freshnessCheckNotices(checkStreams)...)
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
	// Unfailable Verify rows: a row whose command is structurally
	// incapable of failing manufactures evidence. NOTICE this phase — the rules
	// fire on briefs already on main, many of them closed, and rewriting a closed
	// brief's Verify table to green the gate is the very falsification the check
	// exists to catch. Flip to a hard problem once the active streams are clean.
	notices = append(notices, unfailableRowNotices(checkStreams)...)
	// Dead-link lint. BLOCKING: docFiles(root) is CLAUDE.md plus every
	// *.md under docs/**, so its inputs INCLUDE every stream README and brief
	// file. It is also the only check that catches a README row whose brief file
	// does not exist — see the classification comment above.
	problems = append(problems, linkProblems(root, docFiles(root))...)
	// Register-reference link lint: for every markdown link
	// whose text is F-NN/I-NN, verify target file exists and frontmatter id
	// matches. Bare refs are never checked. BLOCKING for the same reason — it
	// takes the same docFiles(root) superset.
	rp, rn := registerRefProblems(root, docFiles(root))
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
		// Best-effort: a broken intake dir produces a diagnostic NOTICE
		// but never a hard failure — the board must still build.
		notices = append(notices, fmt.Sprintf("intake register unreadable: %v", intakeErr))
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
		finalVerdict(mode, len(problems)+len(offBoardProblems))
		return 1
	}

	// Build the generated view now — this also exercises applyFindings/nextUp/
	// emit, so --lint is a true superset of generation minus the STATUS.md byte
	// compare: a PR that would crash main's post-merge regen fails here instead.
	applyFindings(streams, findings)
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
	nu := nextUp(streams, claimed, briefTouch)
	nu.Claims = claimSource
	// A degraded claim read is announced on stderr AND carried into the emitted
	// board by nu.Claims. --require-claims escalates it to a hard failure for
	// callers that must never dispatch from an unfiltered board.
	if n := claimSource.Notice(nu.Eligible); n != "" {
		notices = append(notices, n)
		if requireClaims {
			emitNotices(notices)
			fmt.Fprintln(os.Stderr, "PROBLEM:", "claim filtering could not be established and --require-claims is set: "+claimSource.Reason)
			finalVerdict(mode, 1)
			return 1
		}
	}
	// Span-of-control overflow is a WIP-pressure alarm, surfaced as a --lint
	// NOTICE as well as an in-STATUS line.
	if nu.Overflow() {
		notices = append(notices, fmt.Sprintf(
			"Next-up overflow: %d of %d eligible shown, %d held back (span-of-control cap %d) — WIP pressure",
			len(nu.Picks), nu.Eligible, nu.HeldBack(), nu.Span))
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
	}

	// Awaiting-age: render how long each awaiting row has sat in
	// its current status, from the historian. Best-effort: a missing/unreadable
	// log renders "—" everywhere, never an error — the board must still build.
	var ages map[string]string
	if hist, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		cur := make(map[string]string)
		for _, s := range streams {
			for _, b := range s.Briefs {
				if b.Status == "implemented" || b.Status == "verified" {
					cur[s.Name+"/"+b.Num] = b.Status
				}
			}
		}
		ages = awaitingAges(hist, cur, nowFunc())
	}

	out := emit(streams, findings, nu, ages, intakeAlarmResult, briefTouch, rootRepoName)
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
			fmt.Fprintln(os.Stderr, "STATUS.md is out of date — run: go run ./tools/statusgen")
			drift++
		}
		if rp := checkRegisterViews(root); len(rp) > 0 {
			for _, p := range rp {
				fmt.Fprintln(os.Stderr, "PROBLEM:", p)
			}
			drift += len(rp)
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
// declarations) runs FIRST and short-circuits. Ambiguous ownership makes every
// downstream number attributable to the wrong repo, so there is nothing worth
// emitting until it is resolved.
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

	if problems := crossRootProblems(roots); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "PROBLEM:", p)
		}
		finalVerdict(mode, len(problems))
		return 1
	}

	total := 0
	verdictAccum = &total
	// Restored on EVERY exit from here, panic included. A panic mid-loop would
	// otherwise leave the accumulator dangling non-nil for the rest of the
	// process, and the next finalVerdict would silently swallow its line instead
	// of printing it.
	defer func() { verdictAccum = nil }()
	exit := 0
	covered := 0
	for _, root := range roots {
		// PROBLEM/NOTICE lines carry no root prefix (they are produced deep in
		// the per-check helpers), so the driver frames each root's block. Without
		// this a two-root run prints two undifferentiated streams of findings.
		// Printed BEFORE the root is attempted, so it enumerates configured roots
		// rather than successful ones — hence the coverage line after the loop.
		fmt.Fprintf(os.Stderr, "statusgen: === root %s ===\n", root)
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
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	attachPlaceholders(streams)
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

	// --root is REPEATABLE: each occurrence adds a root,
	// and the run emits one STATUS.md per root filtered to that root's streams.
	// Omitting it entirely keeps the historical default of ".".
	var roots rootFlags
	// The `(default ".")` is spelled out because flag.Var — unlike flag.String —
	// prints no default in usage output, and the default is unchanged.
	flag.Var(&roots, "root", `repository root (default "."; repeatable — one STATUS.md per root)`)
	checkMode := flag.Bool("check", false, "verify STATUS.md is current instead of writing it")
	lintMode := flag.Bool("lint", false, "run all checks without reading or writing STATUS.md (defaults --budget to "+defaultBudgetSpec+" unless overridden)")
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
	// Issue scanner — self-contained sub-command that reads OPEN
	// issues across the fixed repo set (gh) and WRITES a placeholder brief per
	// unhandled one. Never invoked by --lint (no network dependency on the offline
	// gate); never pushes or mutates GitHub issues (read-only there).
	scanIssuesMode := flag.Bool("scan-issues", false, "emit placeholder briefs for unhandled OPEN issues across the fixed repo set (reads gh; writes files, never STATUS.md)")
	scanDryRun := flag.Bool("dry-run", false, "with --scan-issues: list what WOULD be created without writing")
	closeVerifyID := flag.String("close-verify", "", "flip <stream>/<NN> verified→done with a human:<name> sign-off (refuses if not verified/gate:human)")
	registerLinksFlag := flag.Bool("register-links", false, "backfill: rewrite bare F-NN/I-NN tokens in brief files to linked form")
	span := flag.Int("span", defaultSpanOfControl, "Next-up span-of-control cap: max items shown (default 20 — agent-worked queue, not the human EEMUA-191 7±2)")
	overflowT := flag.Int("overflow-threshold", -1, "eligible-brief count above which Next-up flags overflow; <0 = same as --span")
	requireClaimsFlag := flag.Bool("require-claims", false, "fail (exit 1, nothing written) instead of emitting a degraded board when the origin claim read fails")
	// FINDINGS alarm-KPI knobs (ISA-18.2). Standalone
	// block so sibling statusgen flag PRs merge trivially.
	alarmsMode := flag.Bool("alarms", false, "print FINDINGS alarm KPIs (rate, standing-alarm age, flood) and exit")
	standingAge := flag.Int("standing-age-days", defaultStandingAgeDays, "standing-alarm age threshold in days (default ~1 retro-cycle)")
	flood := flag.Int("flood-threshold", defaultFloodThreshold, "active-unresolved finding count above which the register floods (ISA-18.2)")
	// DORA metrics emitter — self-contained sub-command.
	// DORA metrics are DIAGNOSTIC, per-project — never a target or scorecard.
	// NOTE: --dora reuses the shared --since flag (declared below for --trend) —
	// they are mutually-exclusive sub-commands, so one --since serves both;
	// declaring it twice would panic (flag redefined).
	doraMode := flag.Bool("dora", false, "emit the 5 DORA metrics (throughput + instability) as a system; diagnostic, not a target")
	doraJSON := flag.Bool("json", false, "machine-readable JSON output. Used with --dora or --code")
	doraSeries := flag.Bool("series", false, "time series (per-period buckets) instead of a single aggregate. Used with --dora or --code")
	doraBy := flag.String("by", "", "--dora grouping dimension: stream | goal")
	// --trend: SCADA historian view over the status log.
	trendMode := flag.Bool("trend", false, "roll the status-transition log up into a time-series historian view (does not read/write STATUS.md)")
	// --code: code-efficiency metrics from ledger artifacts (git + issue register).
	codeMode := flag.Bool("code", false, "emit code-efficiency metrics (SLOC delta, churn, defect density, change spread, review depth)")
	since := flag.String("since", "", "--trend period start / --dora window start / --code window start (YYYY-MM-DD)")
	weekly := flag.Bool("weekly", false, "--trend: bucket by ISO week (default)")
	daily := flag.Bool("daily", false, "--trend: bucket by day")
	historyPath := flag.String("history", "", "--trend: history log path (default docs/streams/.history.jsonl, relative to --root)")
	// Corroboration check: self-contained sub-command,
	// same STATUS.md-free discipline as the verify-gate modes. Network-dependent
	// by nature — never wired into the offline lint gate.
	consumersMode := flag.Bool("consumers", false, "corroborate the consumers: routing claims of the briefs this branch touches against its own diff (exit 1 = a claim is DISPROVED, 2 = the diff could not be taken)")
	consumersBase := flag.String("base", "origin/main", "--consumers: base ref for the three-dot diff")
	consumersBrief := flag.String("brief", "", "--consumers: check only this brief (<stream>/<NN>); default = every brief file in the diff")
	corroborateMode := flag.Bool("corroborate", false, "check human:<name> stamps against PR reviews/comments for corroboration (requires --pr)")
	corroboratePRs := flag.String("pr", "", "comma-separated PR numbers (required with --corroborate)")
	// Daily factory-floor bottleneck report: per-stage
	// WIP + dwell, constraint location, shift detection, prescribed ToC action.
	// Self-contained diagnostic sub-command — never reads or writes STATUS.md.
	bottleneckMode := flag.Bool("bottleneck", false, "emit the daily factory-floor bottleneck report (per-stage WIP + dwell, constraint, shift, action)")
	roadmapMode := flag.Bool("roadmap", false, "render the roadmap deck overview page (docs/reports/roadmap/index.html)")
	launchMode := flag.Bool("launch", false, "print launch-readiness rollup — transitive depends: of the go-live gate (assay-launch/05) with live status (never reads/writes STATUS.md)")
	launchTarget := flag.String("launch-target", "assay-launch/05", "target brief for --launch (default assay-launch/05)")
	// Evidence-bundle export (gtm/05): deterministic tarball of briefs, registers,
	// and Evidence blocks in a date range, with a generated manifest.json.
	// Takes two positional args <from> <to> (YYYY-MM-DD) and requires -o <path>.
	// -o is parsed from flag.Args() because positional args stop flag parsing.
	exportEvidenceMode := flag.Bool("export-evidence", false, "export an evidence bundle tarball for the given date range (positional <from> <to>; -o <path>; optional -generated <RFC3339> for byte-reproducible output)")
	// Gate-score emitter: JSON for deskboard consumption.
	gateScoresMode := flag.Bool("gate-scores", false, "emit awaiting-queue gate scores as JSON (brief, score, blockedCount, stream, status)")
	// Gate-effectiveness telemetry: override rate, catch
	// rate, ceremonial-gate detection. Self-contained diagnostic sub-command,
	// same STATUS.md-free discipline as --dora/--trend/--bottleneck. --root
	// points at ONE window's fixture/data directory (see gatetelemetry.go).
	gateTelemetryMode := flag.Bool("gate-telemetry", false, "emit gate-effectiveness telemetry (override rate, catch rate, ceremonial-gate detection) for one window's --root")
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
			"--verify-issues":   *verifyIssuesMode,
			"--decision-issues": *decisionIssuesMode,
			"--scan-issues":     *scanIssuesMode,
			"--close-verify":    *closeVerifyID != "",
			"--alarms":          *alarmsMode,
			"--dora":            *doraMode,
			"--code":            *codeMode,
			"--trend":           *trendMode,
			"--bottleneck":      *bottleneckMode,
			"--roadmap":         *roadmapMode,
			"--launch":          *launchMode,
			"--export-evidence": *exportEvidenceMode,
			"--gate-scores":     *gateScoresMode,
			"--register-links":  *registerLinksFlag,
			"--gate-telemetry":  *gateTelemetryMode,
			// --consumers takes ONE git diff, against one root's HEAD. Narrowing
			// to the first root would corroborate one repo's claims and report
			// the others as clean without looking at them.
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
	// Issue scanner: self-contained, same STATUS.md-free discipline
	// as the verify-gate modes. READ-only against GitHub — it lists issues and
	// writes local placeholder files; it never creates or mutates an issue.
	if *scanIssuesMode {
		os.Exit(runScanIssues(*root, *scanDryRun, ghIssueLister, issueCommentLister, ghIssueBlessChecker))
	}
	if *closeVerifyID != "" {
		os.Exit(runCloseVerify(*root, *closeVerifyID))
	}
	if *alarmsMode {
		os.Exit(runAlarms(*root))
	}
	// DORA emitter — self-contained sub-command, same
	// STATUS.md-free discipline as the verify-gate modes above.
	if *doraMode {
		by := strings.ToLower(strings.TrimSpace(*doraBy))
		if by == "stream" || by == "goal" {
			if *doraSeries {
				os.Exit(runDoraSeriesGrouped(*root, *since, by, *doraJSON))
			}
			os.Exit(runDoraGrouped(*root, *since, by, *doraJSON))
		}
		if *doraSeries {
			period := "weekly"
			if *daily {
				period = "daily"
			}
			os.Exit(runDoraSeries(*root, *since, period, *doraJSON))
		}
		os.Exit(runDora(*root, *since, *doraJSON))
	}
	// Code-efficiency emitter — self-contained sub-command,
	// same STATUS.md-free discipline as the DORA emitter above.
	if *codeMode {
		os.Exit(runCode(*root, *since, *doraJSON, *doraSeries))
	}
	if *trendMode {
		period := "weekly"
		if *daily {
			period = "daily"
		}
		if *weekly {
			period = "weekly"
		}
		os.Exit(runTrend(*root, *historyPath, *since, period))
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
		os.Exit(runBottleneck(*root))
	}
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
	// Gate-effectiveness telemetry: self-contained,
	// STATUS.md-free, same discipline as --dora/--trend/--bottleneck above.
	if *gateTelemetryMode {
		os.Exit(runGateTelemetry(*root))
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
	os.Exit(runRoots(resolvedRoots, mode, budgetSpecs, changedPaths, *scopeFlag))
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
