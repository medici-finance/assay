package main

// reconcileholdinc.go — the lint hold's incremental evaluation (#2615).
//
// The hold (reconcilehold.go) asks, for every row a reconcile run would move,
// whether the move adds a status-keyed lint PROBLEM. Asked naively that is one
// whole-board pass of every registered rule per row: a backfill that moves 117
// rows on a 716-brief board ran the registry 118 times, re-reading every brief
// file, glob and git history each time, and took over an hour.
//
// Nothing about the question needs that. A row move changes ONE Status cell, and
// each registered rule's output depends on that cell in a way that can be stated
// exactly, so each rule re-derives its post-move output from the moved row's
// stream alone and the hold's verdict — the multiset difference against the
// board as it would be written — is computed from those outputs unchanged:
//
//   - stream-local rules (checkScoped, checkBriefFiles, checkArchivedPlaceholders,
//     verifyRowClassProblems): a row's Status reaches the output only through
//     lines about that row's own stream. The rule run over the one stream before
//     and after the move gives the change, which is applied to the whole-board
//     output (each rule sorts its output, so the result is the sorted multiset).
//   - designGateProblems: register head plus one block per stream, in stream
//     order; only the moved stream's block is recomputed.
//   - deployTransitionProblems: reads the Status of the briefs DEPLOY records
//     name; recomputed in full only when the moved stream is one of them.
//   - attributionProblems: skips every row that is not verified or done before
//     it reads anything, so a move between other states leaves it unchanged.
//   - checkPlaceholderFiles, danglingSatisfiesProblems: never read a row's Status.
//   - driveRegionLintProblems: re-renders each drive-snapshot region from the
//     board as staged; the render reuses one board load for the run, with the
//     moved row's Status mirrored into it after checking the staged README
//     parses to exactly that change, and one read of the run's claims and
//     heartbeat (the world outside the tree does not change mid-run).
//
// A rule with no incremental form (a rule added later, or a test's planted rule)
// is evaluated over the whole board on every move, as before. Any case outside
// these statements — two streams answering to one key, duplicate brief numbers,
// an unsorted rule output, a staged README that does not parse to the expected
// move — falls back to the whole-board evaluation for that rule. holdOptions.
// reference forces the whole-board path for every rule; the parity tests compare
// the two, and with crossCheck every incremental move is compared against a full
// evaluation and a difference is an error, never a silent pass.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// holdCrossCheck turns on crossCheck for every reconcile run that does not pass
// its own holdOptions. False in the binary; the package tests set it, so every
// hold they exercise is also checked against the whole-board evaluation.
var holdCrossCheck = false

// holdStats is the test hook that counts the hold's work (#2615).
type holdStats struct {
	// FullRuleEvals counts evaluations of one registered rule over the whole
	// board (the cross-check's own evaluations are not counted).
	FullRuleEvals int
	// Moves counts rows the hold evaluated.
	Moves int
}

// holdOptions configures one reconcileWrites run.
type holdOptions struct {
	stats      *holdStats // nil: not collected
	reference  bool       // evaluate every rule over the whole board on every move (the pre-#2615 path)
	crossCheck bool       // compare every move's incremental evaluation with a whole-board one
	progress   io.Writer  // nil: silent; else progress lines (stderr in the CLI)
}

// holdMove is one row move under evaluation.
type holdMove struct {
	si       int    // index of the moved row's stream in lintEnv.streams
	row      *Brief // the moved row
	key, num string // the stream key and number tryMove was asked for
	prev, to string // the row's Status before and during the move
	staged   bool   // the move's README edit is staged, so a tree-reading rule sees it
}

// holdRuleState is one rule's incremental evaluator for one hold run.
type holdRuleState interface {
	// before runs with the row still at m.prev and nothing staged.
	before(m *holdMove)
	// after runs with the row at m.to and its README staged, and returns the
	// rule's whole-board output on that board; ok=false asks for a whole-board
	// evaluation instead.
	after(m *holdMove) (out []string, ok bool)
	// settle reports whether the move was kept, and the rule's output on the
	// moved board (from after, or the whole-board evaluation it asked for).
	settle(m *holdMove, kept bool, out []string)
}

// holdIncremental builds a rule's incremental evaluator: it evaluates the rule
// once over e's board and returns that output with the state that re-derives
// it per move. A nil state means the rule is evaluated in full on every move.
type holdIncremental func(e *lintEnv, r statusKeyedLintRule) (baseline []string, st holdRuleState)

// incrementalEligible reports whether e's board satisfies the conditions the
// incremental forms rely on: every stream key (directory name and frontmatter
// name) names one stream, and no stream lists a brief number twice.
func incrementalEligible(streams []*Stream) bool {
	owner := map[string]*Stream{}
	for _, s := range streams {
		for _, k := range []string{filepath.Base(s.Dir), s.Name} {
			if o, ok := owner[k]; ok && o != s {
				return false
			}
			owner[k] = s
		}
		seen := map[string]bool{}
		for _, b := range s.Briefs {
			if seen[b.Num] {
				return false
			}
			seen[b.Num] = true
		}
	}
	return true
}

// fullEval evaluates one rule over e's whole board, counted.
func (e *lintEnv) fullEval(r statusKeyedLintRule) []string {
	if e.stats != nil {
		e.stats.FullRuleEvals++
	}
	return r.problems(e)
}

// streamEnv is e narrowed to the one stream at si, for a stream-local rule.
func (e *lintEnv) streamEnv(si int) *lintEnv {
	return &lintEnv{root: e.root, streams: e.streams[si : si+1], edge: e.edge, findings: e.findings}
}

// --- stream-local rules ------------------------------------------------------

type streamLocalState struct {
	e     *lintEnv
	r     statusKeyedLintRule
	cur   []string         // the rule's whole-board output, sorted
	local map[int][]string // the rule over one stream, as of the board as it would be written
	pre   []string
	post  []string
	incr  bool // the last after() answered incrementally
}

func streamLocal(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	out := e.fullEval(r)
	if !sort.StringsAreSorted(out) {
		return out, nil
	}
	return out, &streamLocalState{e: e, r: r, cur: out, local: map[int][]string{}}
}

func (st *streamLocalState) before(m *holdMove) {
	pre, ok := st.local[m.si]
	if !ok {
		pre = st.r.problems(st.e.streamEnv(m.si))
		st.local[m.si] = pre
	}
	st.pre = pre
}

func (st *streamLocalState) after(m *holdMove) ([]string, bool) {
	st.incr = false
	st.post = st.r.problems(st.e.streamEnv(m.si))
	if !sort.StringsAreSorted(st.post) {
		return nil, false
	}
	if equalStrings(st.pre, st.post) {
		st.incr = true
		return st.cur, true
	}
	count := map[string]int{}
	for _, p := range st.cur {
		count[p]++
	}
	for _, p := range st.pre {
		count[p]--
	}
	for _, p := range st.post {
		count[p]++
	}
	out := make([]string, 0, len(st.cur)+len(st.post))
	for p, n := range count {
		if n < 0 {
			return nil, false // the one-stream output is not part of the whole: evaluate in full
		}
		for ; n > 0; n-- {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	st.incr = true
	return out, true
}

func (st *streamLocalState) settle(m *holdMove, kept bool, out []string) {
	if kept {
		st.cur = out
		if st.incr {
			st.local[m.si] = st.post
		} else {
			delete(st.local, m.si)
		}
	}
	st.pre, st.post, st.incr = nil, nil, false
}

// --- rules a move cannot change, or changes only in a known case -------------

// reuseState returns cur unless full(m) says the move can change the rule.
type reuseState struct {
	cur  []string
	full func(m *holdMove) bool
}

func (st *reuseState) before(*holdMove) {}

func (st *reuseState) after(m *holdMove) ([]string, bool) {
	if st.full != nil && st.full(m) {
		return nil, false
	}
	return st.cur, true
}

func (st *reuseState) settle(_ *holdMove, kept bool, out []string) {
	if kept {
		st.cur = out
	}
}

// statusInvariant is a rule that never reads a brief row's Status.
func statusInvariant(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	out := e.fullEval(r)
	return out, &reuseState{cur: out}
}

// attributionIncremental: attributionProblems skips every row that is not
// verified or done before reading anything about it.
func attributionIncremental(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	out := e.fullEval(r)
	attributed := func(s string) bool { return s == "verified" || s == "done" }
	return out, &reuseState{cur: out, full: func(m *holdMove) bool { return attributed(m.prev) || attributed(m.to) }}
}

// decisionGateIncremental: the decision-gate hold (decisiongatehold.go) judges
// the status move, relabel or drop of a gate: human brief, and refuses records
// it cannot read. A row's Status reaches its output only through that row's own
// brief, so a move of any other row leaves it unchanged; the row of a gate:
// human brief (or of a record the hold cannot read) is evaluated in full.
func decisionGateIncremental(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	out := e.fullEval(r)
	watched := map[string]bool{}
	for k, b := range gateSnapshotOnDisk(e.root) {
		if b.Gate == gateHumanValue || b.Unreadable != "" || b.FMErr {
			watched[k] = true
		}
	}
	return out, &reuseState{cur: out, full: func(m *holdMove) bool {
		if len(watched) == 0 {
			return false
		}
		s := e.streams[m.si]
		num := normBriefNum(m.num)
		return watched[filepath.Base(s.Dir)+"/"+num] || watched[s.Name+"/"+num]
	}}
}

// deployIncremental: deployTransitionProblems reads only the Status of briefs
// a DEPLOY record names (resolveBriefStatus, by stream name).
func deployIncremental(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	out := e.fullEval(r)
	entries, err := parseDeploysDir(e.root)
	if err != nil {
		return out, &reuseState{cur: out} // unreadable register: the rule reports its shape PROBLEMs only
	}
	named := map[string]bool{}
	for _, d := range entries {
		if d.Kind != deployKindDeploy {
			continue
		}
		// The same filter deployTransitionProblems applies before it resolves a
		// ref; a ref it resolves names "<stream>/<NN>", stream first.
		ref := strings.TrimSpace(d.Brief)
		if ref == "" || !deployBriefRefRe.MatchString(ref) {
			continue
		}
		named[strings.SplitN(ref, "/", 2)[0]] = true
	}
	return out, &reuseState{cur: out, full: func(m *holdMove) bool { return named[e.streams[m.si].Name] }}
}

// --- designGateProblems ------------------------------------------------------

type designGateState struct {
	e     *lintEnv
	head  []string
	known map[string]bool
	parts [][]string
	next  []string
}

func designGateIncremental(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	if e.stats != nil {
		e.stats.FullRuleEvals++
	}
	head, known, ok := designGateRegister(e.root)
	if !ok {
		return head, &reuseState{cur: head} // unreadable register: the gate reports the shape PROBLEMs only
	}
	st := &designGateState{e: e, head: head, known: known, parts: make([][]string, len(e.streams))}
	for i, s := range e.streams {
		st.parts[i] = designGateStreamProblems(s, known)
	}
	return st.concat(-1, nil), st
}

func (st *designGateState) concat(si int, part []string) []string {
	out := append([]string(nil), st.head...)
	for i, p := range st.parts {
		if i == si {
			p = part
		}
		out = append(out, p...)
	}
	return out
}

func (st *designGateState) before(*holdMove) {}

func (st *designGateState) after(m *holdMove) ([]string, bool) {
	st.next = designGateStreamProblems(st.e.streams[m.si], st.known)
	return st.concat(m.si, st.next), true
}

func (st *designGateState) settle(m *holdMove, kept bool, _ []string) {
	if kept {
		st.parts[m.si] = st.next
	}
	st.next = nil
}

// --- driveRegionLintProblems -------------------------------------------------

// driveLintCtx renders drive-snapshot sections the way driveSnapshotSection
// does, from one board load and one read of the run's claims and heartbeat.
type driveLintCtx struct {
	root      string
	loaded    bool
	streams   []*Stream
	loadErr   error
	claimed   map[string]bool
	claimsOK  bool
	heartbeat string
	hbOK      bool
	calls     int
}

func (c *driveLintCtx) section(slug string, now time.Time) (string, int) {
	return driveLintSection(c, slug, now)
}

// Keep the renderer on the package-local call graph used by the Status-keyed
// rule registry scanner, including when lint reuses an already hydrated board.
func driveLintSection(c *driveLintCtx, slug string, now time.Time) (string, int) {
	c.calls++
	if !c.loaded {
		c.loaded = true
		c.streams, _, c.loadErr = loadHydratedStreams(c.root)
	}
	if c.loadErr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: --drive-snapshot could not read the board:", c.loadErr)
		return "", 2
	}
	streams := cloneStreamRows(c.streams)
	if !c.claimsOK {
		c.claimed, _ = resolveClaims(c.root, streams)
		c.claimsOK = true
	}
	claimed := make(map[string]bool, len(c.claimed))
	for k, v := range c.claimed {
		claimed[k] = v
	}
	ds := loadDrives(c.root, streams, now)
	for _, w := range ds.Warnings {
		fmt.Fprintln(os.Stderr, "NOTICE:", w)
	}
	if !c.hbOK {
		c.heartbeat = driveHeartbeatLine(c.root)
		c.hbOK = true
	}
	section, ok := driveSnapshotRender(ds, streams, claimed, slug, c.heartbeat, now)
	if !ok {
		return "", 2
	}
	return section, 0
}

// cloneStreamRows copies each stream and its row slices, so nothing a render
// does to what it is handed can reach the next render.
func cloneStreamRows(streams []*Stream) []*Stream {
	out := make([]*Stream, len(streams))
	for i, s := range streams {
		c := *s
		c.Briefs = append([]Brief(nil), s.Briefs...)
		c.Placeholders = append(c.Placeholders[:0:0], s.Placeholders...)
		out[i] = &c
	}
	return out
}

type driveHoldState struct {
	e        *lintEnv
	r        statusKeyedLintRule
	ctx      *driveLintCtx
	cur      []string
	broken   bool    // the mirror cannot be trusted: evaluate in full from here on
	pre      *Stream // the moved row's stream README as parsed before the move was staged
	mirror   *Brief  // the ctx row the current move set, restored on rollback
	mirrorTo string
}

func driveIncremental(e *lintEnv, r statusKeyedLintRule) ([]string, holdRuleState) {
	if e.stats != nil {
		e.stats.FullRuleEvals++
	}
	ctx := &driveLintCtx{root: e.root}
	now := nowFunc()
	out, _ := driveRegionLint(e.root, func(slug string) (string, int) { return ctx.section(slug, now) })
	if ctx.calls == 0 {
		// No region was rendered: nothing the rule read depends on a README row.
		return out, &reuseState{cur: out}
	}
	st := &driveHoldState{e: e, r: r, ctx: ctx, cur: out}
	if ctx.loadErr != nil || !incrementalEligible(ctx.streams) {
		st.broken = true
	}
	return out, st
}

func (st *driveHoldState) ctxRow(m *holdMove) (*Stream, *Brief) {
	for _, s := range st.ctx.streams {
		if filepath.Base(s.Dir) != m.key && s.Name != m.key {
			continue
		}
		for i := range s.Briefs {
			if s.Briefs[i].Num == m.num {
				return s, &s.Briefs[i]
			}
		}
	}
	return nil, nil
}

func (st *driveHoldState) before(m *holdMove) {
	st.pre = nil
	if st.broken || !m.staged {
		return
	}
	if s, _ := st.ctxRow(m); s != nil {
		st.pre, _ = parseStreamREADME(filepath.Join(s.Dir, "README.md"))
	}
}

func (st *driveHoldState) after(m *holdMove) ([]string, bool) {
	if st.broken || !m.staged {
		return nil, false
	}
	s, row := st.ctxRow(m)
	if row == nil || row.Status != m.prev || st.pre == nil || !st.mirrorExact(s, row, m) {
		st.broken = true
		return nil, false
	}
	row.Status = m.to
	st.mirror, st.mirrorTo = row, m.prev
	now := nowFunc()
	out, _ := driveRegionLint(st.e.root, func(slug string) (string, int) { return st.ctx.section(slug, now) })
	return out, true
}

// mirrorExact reports whether a reload of the board would differ from the
// render's board only by the moved row's Status: the stream README parsed after
// staging equals its parse before staging with that one cell changed, and the
// render's README rows match that parse cell for cell. The render's stream
// also carries one synthetic row per placeholder file, appended after the
// README rows (attachPlaceholders); staging writes only the README, so those
// tail rows are left out of the comparison, and a move that lands on one is
// not mirrored.
func (st *driveHoldState) mirrorExact(s *Stream, row *Brief, m *holdMove) bool {
	staged, err := parseStreamREADME(filepath.Join(s.Dir, "README.md"))
	n := len(s.Briefs) - len(s.Placeholders)
	if err != nil || n < 0 || len(staged.Briefs) != len(st.pre.Briefs) || len(staged.Briefs) != n {
		return false
	}
	for i := n; i < len(s.Briefs); i++ {
		if &s.Briefs[i] == row || s.Briefs[i].Schema != "placeholder-v1" {
			return false
		}
	}
	want := *st.pre
	want.Briefs = append([]Brief(nil), st.pre.Briefs...)
	for i := range s.Briefs[:n] {
		have := s.Briefs[i]
		status := have.Status
		if &s.Briefs[i] == row {
			status = m.to
		}
		w := &want.Briefs[i]
		if w.Num != have.Num || w.Title != have.Title || w.RawCell != have.RawCell || w.Wave != have.Wave ||
			w.Verified != have.Verified || w.Reviewed != have.Reviewed || w.Effort != have.Effort {
			return false
		}
		if &s.Briefs[i] == row {
			if w.Status != m.prev {
				return false
			}
			w.Status = m.to
		} else if w.Status != status {
			return false
		}
	}
	return reflect.DeepEqual(&want, staged)
}

func (st *driveHoldState) settle(_ *holdMove, kept bool, out []string) {
	if !kept && st.mirror != nil {
		st.mirror.Status = st.mirrorTo
	}
	if kept {
		st.cur = out
		if st.mirror == nil {
			st.broken = true // a kept move the render's board did not mirror
		}
	}
	st.mirror, st.pre = nil, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
