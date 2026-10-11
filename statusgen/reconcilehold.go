package main

// reconcilehold.go — the lint hold on `reconcile --apply` (derived-board/04).
//
// --apply writes a brief row's Status cell. Several lint rules key on that cell
// (the design-approval gate is the one live today: a risk-gated brief authored
// after the cutover may not sit at in-progress-or-later without a design record).
// A writer that moves a row into a state the lint rejects produces a tree its own
// lint step then fails on, which wedges an unattended job. The hold makes the
// writer ask the lint before keeping a write: for each row the write would move,
// it moves the row in the loaded board AND stages the edited README in memory
// (readmeStage: stream-README reads return the staged bytes, so rules that
// re-read the tree — the drive-snapshot region check — see the move too, with
// nothing written to disk), re-runs every Status-keyed lint rule, and if any
// PROBLEM appears that was not there before, unstages the edit and HOLDS the
// row — reported with the PROBLEM it would cause, never kept. The read-only
// form of reconcile runs the same evaluation, so it reports the same holds. Nothing in the lint changes
// and nothing is invented (no design record is created, no gate is skipped);
// the held row stays exactly what the board said until a human supplies what
// the rule needs.
//
// statusKeyedLintRules is the registry the hold evaluates. Its completeness is
// pinned by TestStatusKeyedLintRulesRegistered (an AST walk of run()'s
// problem-producing callees that read a row's Status), and its effect by
// TestApplyThenLintAddsNoProblem (the FULL lint over the written tree gains no
// PROBLEM). The job's own lint step stays the last, loud backstop.

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// stagedReadmes is the in-memory README content the hold evaluates a move
// against: readStreamREADME returns a staged entry in place of the file on
// disk. Entries exist only while reconcileWrites runs (readmeStage.clear); the
// lock is there because parallel tests load boards concurrently, each under
// its own root.
var stagedReadmes = struct {
	sync.RWMutex
	m map[string]string
}{m: map[string]string{}}

func stageKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// readStreamREADME reads a stream README, preferring content staged by the
// lint hold over the bytes on disk.
func readStreamREADME(path string) ([]byte, error) {
	stagedReadmes.RLock()
	c, ok := stagedReadmes.m[stageKey(path)]
	stagedReadmes.RUnlock()
	if ok {
		return []byte(c), nil
	}
	return os.ReadFile(path)
}

// readmeStage is one reconcileWrites run's set of staged READMEs.
type readmeStage struct{ keys map[string]bool }

func newReadmeStage() *readmeStage { return &readmeStage{keys: map[string]bool{}} }

// put stages content for path and returns the undo that puts back what was
// staged before (or nothing).
func (s *readmeStage) put(path, content string) (undo func() error) {
	key := stageKey(path)
	stagedReadmes.Lock()
	prev, had := stagedReadmes.m[key]
	stagedReadmes.m[key] = content
	stagedReadmes.Unlock()
	s.keys[key] = true
	return func() error {
		stagedReadmes.Lock()
		defer stagedReadmes.Unlock()
		if had {
			stagedReadmes.m[key] = prev
		} else {
			delete(stagedReadmes.m, key)
		}
		return nil
	}
}

// clear drops every entry this run staged.
func (s *readmeStage) clear() {
	stagedReadmes.Lock()
	defer stagedReadmes.Unlock()
	for k := range s.keys {
		delete(stagedReadmes.m, k)
	}
}

// lintEnv is the loaded board a status-keyed rule evaluates: the same inputs
// run() assembles for its per-stream checks (active streams with issue-loop
// placeholders attached, the archived-inclusive edge universe, findings).
type lintEnv struct {
	root     string
	streams  []*Stream
	edge     []*Stream
	findings []Finding
	baseline []string // status-keyed PROBLEMs of the board as currently written; valid once ready
	ready    bool
	opts     holdOptions
	stats    *holdStats
	states   []holdRuleState // per registered rule; nil entries are evaluated over the whole board (reconcileholdinc.go)
}

// loadLintEnv loads root the way run() does before its per-stream checks.
func loadLintEnv(root string) (*lintEnv, error) {
	streams, findings, err := loadStreams(root)
	if err != nil {
		return nil, err
	}
	attachPlaceholders(streams)
	archived, _ := loadArchivedStreams(root) // an unreadable archive only narrows edge resolution; run() treats it as a NOTICE too
	return &lintEnv{root: root, streams: streams, edge: edgeResolutionUniverse(streams, archived), findings: findings,
		opts: holdOptions{crossCheck: holdCrossCheck}}, nil
}

// statusKeyedLintRule is one lint rule run() applies whose verdict can change
// when a brief row's Status cell changes.
type statusKeyedLintRule struct {
	name     string // the run() callee, as TestStatusKeyedLintRulesRegistered matches it
	problems func(e *lintEnv) []string
	// incremental is the rule's per-move form (reconcileholdinc.go): how a row
	// move's effect on it is derived without re-running it over the whole board.
	// nil evaluates the rule over the whole board on every move.
	incremental holdIncremental
}

// statusKeyedLintRules lists every Status-keyed PROBLEM producer run() calls,
// each invoked exactly as run() invokes it.
var statusKeyedLintRules = []statusKeyedLintRule{
	{"checkScoped", func(e *lintEnv) []string { p, _ := checkScoped(e.streams, e.edge, e.findings); return p }, streamLocal},
	{"checkBriefFiles", func(e *lintEnv) []string { p, _ := checkBriefFiles(e.streams, e.edge); return p }, streamLocal},
	{"checkPlaceholderFiles", func(e *lintEnv) []string { p, _ := checkPlaceholderFiles(e.streams); return p }, statusInvariant},
	{"checkArchivedPlaceholders", func(e *lintEnv) []string { p, _ := checkArchivedPlaceholders(e.streams); return p }, streamLocal},
	{"designGateProblems", func(e *lintEnv) []string { return designGateProblems(e.root, e.streams) }, designGateIncremental},
	{"deployTransitionProblems", func(e *lintEnv) []string { return deployTransitionProblems(e.root, e.streams) }, deployIncremental},
	{"attributionProblems", func(e *lintEnv) []string { p, _ := attributionProblems(e.streams); return p }, attributionIncremental},
	{"verifyRowClassProblems", func(e *lintEnv) []string { return verifyRowClassProblems(e.streams) }, streamLocal},
	{"danglingSatisfiesProblems", func(e *lintEnv) []string { return danglingSatisfiesProblems(e.root, e.streams) }, statusInvariant},
	// Reads the tree itself (it re-renders each drive-snapshot region from disk),
	// which is why a move is evaluated with the README already written.
	{"driveRegionLintProblems", func(e *lintEnv) []string { p, _ := driveRegionLintProblems(e.root, nowFunc()); return p }, driveIncremental},
}

// statusKeyedNotRows names run() callees the registry completeness test sees
// reading a `.Status` that is NOT a brief row's Status cell, with the reason.
var statusKeyedNotRows = map[string]string{
	"requirementRegisterProblems": "reads a requirement-register entry's own status field, never a brief row",
}

// problems evaluates every registered rule over the whole board, in registry
// order — the hold's reference answer. Not counted in holdStats.
func (e *lintEnv) problems() []string {
	var out []string
	for _, r := range statusKeyedLintRules {
		out = append(out, r.problems(e)...)
	}
	return out
}

// initHold evaluates the board as currently written: once per rule, through
// the rule's incremental form when it has one and the board allows it.
func (e *lintEnv) initHold() error {
	e.ready = true
	e.states = make([]holdRuleState, len(statusKeyedLintRules))
	incremental := !e.opts.reference && incrementalEligible(e.streams)
	var base []string
	for i, r := range statusKeyedLintRules {
		var out []string
		if incremental && r.incremental != nil {
			out, e.states[i] = r.incremental(e, r)
		} else {
			out = e.fullEval(r)
		}
		base = append(base, out...)
	}
	e.baseline = base
	if e.opts.crossCheck {
		if full := e.problems(); !equalStrings(full, base) {
			return fmt.Errorf("lint hold self-check: incremental evaluation diverged from the whole-board lint on the board as written:\nincremental: %q\nwhole-board: %q", base, full)
		}
	}
	return nil
}

// afterMove returns each rule's output on the board with m applied.
func (e *lintEnv) afterMove(m *holdMove) [][]string {
	outs := make([][]string, len(statusKeyedLintRules))
	for i, r := range statusKeyedLintRules {
		if st := e.states[i]; st != nil {
			if out, ok := st.after(m); ok {
				outs[i] = out
				continue
			}
		}
		outs[i] = e.fullEval(r)
	}
	return outs
}

func (e *lintEnv) settleMove(m *holdMove, kept bool, outs [][]string) {
	for i, st := range e.states {
		if st != nil {
			var out []string
			if outs != nil {
				out = outs[i]
			}
			st.settle(m, kept, out)
		}
	}
}

// findRow returns the loaded brief row for stream/num, keyed by the stream's
// directory name (the same key briefStreamNum yields and the README path uses).
func (e *lintEnv) findRow(stream, num string) *Brief {
	_, row := e.findRowAt(stream, num)
	return row
}

// findRowAt is findRow with the index of the row's stream in e.streams.
func (e *lintEnv) findRowAt(stream, num string) (int, *Brief) {
	for si, s := range e.streams {
		if filepath.Base(s.Dir) != stream && s.Name != stream {
			continue
		}
		for i := range s.Briefs {
			if s.Briefs[i].Num == num {
				return si, &s.Briefs[i]
			}
		}
	}
	return -1, nil
}

// transitionProblems is the pure, in-memory evaluation: the PROBLEM lines
// moving stream/num to `to` would ADD to the status-keyed lint. The move is
// never kept. found is false when the lint model carries no such row — a
// could-not-check the caller must hold on, never a pass.
func (e *lintEnv) transitionProblems(stream, num, to string) (added []string, found bool) {
	added, found, _ = e.tryMove(stream, num, to, nil, false)
	return added, found
}

// tryMove moves stream/num to `to` and returns the PROBLEM lines that adds
// (multiset difference against the board as currently written). stage, when
// non-nil, stages the matching README edit (readmeStage.put) and returns its
// undo; it runs BEFORE the rules re-evaluate, so a rule that re-reads the tree
// sees the move. With keepIfClean and nothing added, the move is kept (the row
// and the staged README, so later rows are judged against the board as it
// would be written); otherwise both are restored to their exact prior state.
func (e *lintEnv) tryMove(stream, num, to string, stage func() (undo func() error, err error), keepIfClean bool) (added []string, found bool, err error) {
	si, row := e.findRowAt(stream, num)
	if row == nil {
		return nil, false, nil
	}
	if !e.ready {
		if err := e.initHold(); err != nil {
			return nil, true, err
		}
	}
	if e.stats != nil {
		e.stats.Moves++
	}
	prev := row.Status
	m := &holdMove{si: si, row: row, key: stream, num: num, prev: prev, to: to, staged: stage != nil}
	for _, st := range e.states {
		if st != nil {
			st.before(m)
		}
	}
	undo := func() error { return nil }
	if stage != nil {
		if undo, err = stage(); err != nil {
			e.settleMove(m, false, nil)
			return nil, true, err
		}
	}
	row.Status = to
	outs := e.afterMove(m)
	var after []string
	for _, o := range outs {
		after = append(after, o...)
	}
	if e.opts.crossCheck {
		if full := e.problems(); !equalStrings(full, after) {
			row.Status = prev
			e.settleMove(m, false, outs)
			_ = undo()
			return nil, true, fmt.Errorf("lint hold self-check: incremental evaluation diverged from the whole-board lint for %s/%s -> %s:\nincremental: %q\nwhole-board: %q", stream, num, to, after, full)
		}
	}
	seen := map[string]int{}
	for _, p := range e.baseline {
		seen[p]++
	}
	for _, p := range after {
		if seen[p] > 0 {
			seen[p]--
			continue
		}
		added = append(added, p)
	}
	if len(added) == 0 && keepIfClean {
		e.baseline = after
		e.settleMove(m, true, outs)
		return nil, true, nil
	}
	row.Status = prev
	e.settleMove(m, false, outs)
	if uerr := undo(); uerr != nil {
		return added, true, fmt.Errorf("restoring %s/%s after a held move: %w", stream, num, uerr)
	}
	return added, true, nil
}

// heldRow is one witnessed row --apply did NOT write because writing it would
// add a lint PROBLEM (or the lint model could not see it). It is reported —
// text and --json — so the gap is visible and owned, never silently dropped.
type heldRow struct {
	ID       string   `json:"id"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Path     string   `json:"readme"`
	Witness  string   `json:"witness"`
	Problems []string `json:"problems"`
}

func couldNotCheckRow(id string) string {
	return fmt.Sprintf("could-not-check: %s has a README row but no row in the lint's stream model, so the write cannot be evaluated against the lint", id)
}
