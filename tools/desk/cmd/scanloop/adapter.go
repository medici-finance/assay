package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// ScanLoop is the intake desk's adapter onto the frozen drain contract. It adds NOTHING to the
// engine: the heterogeneity this desk brings — a monitor script as the queue source, a trust gate at
// the queueing boundary, a coalesce window, a five-exit ledger — all lives in these six methods and
// in the files beside this one. If the frozen Loop interface ever needed a new hook to fit intake,
// the compile-time assertion at the bottom of this file would stop compiling, which is the
// contract-erosion tripwire.
type ScanLoop struct {
	// Root is the checkout the scan is rooted at and the placeholder state is read from.
	Root string
	// ScanTarget is the repo the placeholder delta is COMMITTED to and the scan PR is opened
	// against. It is ONE repo, not the inbound item's repo: the scan is whole-scope, and a
	// placeholder for an issue in any repo of the scan scope lands in this repo (a foreign-repo
	// issue carrying a `<repo-name>-issue-NN` stem so numbers from different repos never collide).
	// Conflating the two was the defect this field exists to prevent — it made the write boundary
	// look like a property of the INBOUND repo, so an issue on a read-only-scope repo dead-ended.
	ScanTarget string
	// WorktreeBase is the ABSOLUTE directory scan worktrees are cut under. It must sit outside
	// Root — the isolation guard refuses otherwise.
	WorktreeBase string
	// Policy is the coalesce window.
	Policy CoalescePolicy
	// Scope is the intake SCAN scope. nil reads the roster.
	Scope []string
	// Monitor supplies one poll cycle. nil means NO inbound surface is wired, and SelectQueue
	// refuses rather than reporting an empty queue — an unread surface is never an empty one.
	Monitor func() (*MonitorReport, error)
	// Probe is the trust gate's reader. nil leaves every item COULD-NOT-CHECK (fail closed).
	Probe TrustProbe
	// OpenPR reports this session's currently-open scan PR. nil means none is known, which the
	// coalesce policy reads as "cut a fresh one" — the bounded direction.
	OpenPR func() (*OpenScanPR, error)
	// Exec is the lanes' process seam. nil = run for real.
	Exec Exec
	// Write is the lanes' file seam (the captured title and body). nil = write for real.
	Write WriteFile
	// Emit is where dispatch instructions and lane steps are printed. nil = stdout.
	Emit io.Writer
	// Feeder obtains the structured Result for an EMITTED judgment item — the routing decision a
	// model tier makes. nil means the judgment lane has no result path, and Dispatch refuses
	// rather than inventing a routing.
	Feeder func(loopengine.Item, loopengine.Tier, LaneOutcome) (loopengine.Result, error)
	// DryRun prints every lane step without running it.
	DryRun bool
	// Now is the clock seam.
	Now func() time.Time

	mu         sync.Mutex
	report     *MonitorReport
	admissions []Admission
	queued     []string
	parked     map[string]bool
	dispatched map[string]bool
	outcomes   map[string]LaneOutcome
	ledger     *ExitLedger
	// members maps a DISPATCH item to the INBOUND items it covers. The scan-carrier dispatch is a
	// batch — one whole-scope scan covering every new inbound issue this pass — while the ledger is
	// still keyed on the inbound items, because it is those that have to leave by an exit.
	members map[string][]string
	// passBranch is computed ONCE per pass. It used to be derived per item from a minute-granular
	// clock, which meant two items in one pass produced the SAME branch name: the second
	// `worktree add -b` failed, the failure surfaced as a dispatch error, and the item was then
	// reported as a leak. Batching removed the intra-pass collision; making this a per-pass value
	// (second-granular, with a random suffix) removes the cross-pass one too.
	passBranch string
}

// LoopName is the canonical loop identity this consumer presents in DESK_LOOP, and therefore the
// name its per-loop stop flag is scoped to.
const LoopName = "intake-desk"

func (s *ScanLoop) Name() string { return LoopName }

func (s *ScanLoop) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *ScanLoop) emit() io.Writer {
	if s.Emit != nil {
		return s.Emit
	}
	return os.Stdout
}

func (s *ScanLoop) scope() []string {
	if len(s.Scope) > 0 {
		return s.Scope
	}
	return deskkit.ScanRepos()
}

// Ledger is the pass's exit ledger, created on first use.
func (s *ScanLoop) Ledger() *ExitLedger {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ledger == nil {
		s.ledger = NewExitLedger()
	}
	return s.ledger
}

// Admissions is the last gate pass — every inbound item with its verdict, including the ones that
// were not queued. Quarantined items are VISIBLE here by design.
func (s *ScanLoop) Admissions() []Admission {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Admission(nil), s.admissions...)
}

// Report is the last monitor cycle, for the blindness lines the plan and run outputs must carry.
func (s *ScanLoop) Report() *MonitorReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.report
}

// Queued returns the item IDs this pass admitted.
func (s *ScanLoop) Queued() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.queued...)
}

// park records a dispatch EMITTED for a model tier with no result path. It is neither landed nor
// lost. Parking is recorded against the INBOUND items the dispatch covers, because those are what
// the ledger and the leak check speak about.
func (s *ScanLoop) park(dispatchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.parked == nil {
		s.parked = map[string]bool{}
	}
	for _, m := range s.membersOf(dispatchID) {
		s.parked[m] = true
	}
}

// Parked names the items emitted for routing this pass, sorted.
func (s *ScanLoop) Parked() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.parked))
	for id := range s.parked {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// exited is the set the leak check runs against: everything queued MINUS what was emitted for
// routing and minus what a live claim elsewhere took. An item still awaiting a routing decision has
// not leaked — it has not finished — and calling it a leak would make the leak check cry wolf until
// nobody reads it.
func (s *ScanLoop) exited() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, id := range s.queued {
		if s.parked[id] {
			continue
		}
		if !s.dispatched[id] {
			continue // never dispatched (claimed elsewhere, or the pass stopped) — not this pass's exit to owe
		}
		out = append(out, id)
	}
	return out
}

// SelectQueue is the deterministic inbound read: one monitor cycle across the rostered scan scope,
// then the trust gate, then the admitted items in the monitor's own order (oldest update first
// within a repo, repos in roster order — the poller's ordering, not a second ranking invented here).
//
// The gate runs HERE, not at dispatch. An item that was never queued cannot be dispatched by a
// later bug; an item queued "for visibility" and filtered downstream is one missing branch away
// from being acted on.
func (s *ScanLoop) SelectQueue() ([]loopengine.Item, error) {
	if s.Monitor == nil {
		return nil, deskkit.Unverifiable("scanloop: no inbound surface is wired — "+
			"an unread surface is COULD-NOT-CHECK, never an empty queue", nil)
	}
	if err := deskkit.ScanScopeError(); err != nil {
		return nil, err
	}
	report, err := s.Monitor()
	if err != nil {
		return nil, err
	}

	// SCOPE FILTER, before the trust gate. The poller's state directory outlives any one roster,
	// so a repo dropped from the scan scope can still have a baseline sitting there and still emit
	// events. Queueing one would be this drain acting on a surface its configuration says it no
	// longer owns — a NARROWING silently undone by leftover state. Out-of-scope events stay
	// visible and counted, exactly like a quarantined one.
	inScope, outOfScope := splitByScope(report.Inbound, s.scope())
	adm := ApplyTrustGate(inScope, s.Probe)
	for _, o := range outOfScope {
		adm = append(adm, Admission{
			Item:  o,
			State: AdmissionQuarantined,
			Why: "outside the configured intake scan scope — a leftover baseline in the poller's state " +
				"dir can outlive the roster entry that created it; visible and counted, never routed",
		})
	}

	// BATCHING. The scan is WHOLE-SCOPE — one run derives the placeholder delta for every issue in
	// the scan scope at once — so dispatching it once per inbound item would run the same
	// whole-scope scan N times, against one branch and one PR. That is not merely wasteful: the
	// second dispatch collides with the first one's branch and PR, and every collision surfaces as
	// a dispatch error and then as a false "front door leaked" line. So every mechanical item this
	// pass admits is folded into ONE scan dispatch, and the ledger keeps recording one exit per
	// INBOUND item, because it is the inbound items that have to leave.
	var judgment []loopengine.Item
	var batch []Admission
	var queued []string
	members := map[string][]string{}

	for _, a := range adm {
		if !a.Admitted() {
			continue
		}
		id := a.Item.ID()
		queued = append(queued, id)
		lane, kind := s.classify(a)
		if lane == LaneScanCarrierPR {
			batch = append(batch, a)
			continue
		}
		it := s.judgmentItem(a, kind)
		members[it.ID] = []string{id}
		judgment = append(judgment, it)
	}

	var items []loopengine.Item
	if len(batch) > 0 {
		scan := s.scanBatchItem(batch)
		ids := make([]string, 0, len(batch))
		for _, a := range batch {
			ids = append(ids, a.Item.ID())
		}
		members[scan.ID] = ids
		items = append(items, scan) // the deterministic, mechanical work goes first
	}
	items = append(items, judgment...)

	s.mu.Lock()
	s.report = report
	s.admissions = adm
	s.queued = queued
	s.members = members
	s.mu.Unlock()
	return items, nil
}

// splitByScope partitions events by whether their repo is in the configured intake scan scope. An
// EMPTY scope puts everything out of scope — the same fail-closed direction the roster reader takes,
// and never "no scope means all repos".
func splitByScope(items []Inbound, scope []string) (in, out []Inbound) {
	set := make(map[string]bool, len(scope))
	for _, r := range scope {
		set[r] = true
	}
	for _, it := range items {
		if set[it.Repo] {
			in = append(in, it)
		} else {
			out = append(out, it)
		}
	}
	return in, out
}

// classify decides one admitted inbound event's lane, from LOCAL state only, so the queue's shape
// does not depend on a second network read that could disagree with the one that produced it.
//
// EVERY reason a mechanical lane must not run is decided HERE, at classification time, and never
// inside the lane. A lane that refuses is a dispatch ERROR — it is counted as a failure, it flags
// the item as unexited, and because the condition is a standing one (a repo outside the write
// boundary does not become writable by being retried) it repeats on every pass forever. A lane the
// classifier never selects costs nothing.
func (s *ScanLoop) classify(a Admission) (LaneName, string) {
	// (1) Can the mechanical lane write at all? The scan's write destination is the SCAN TARGET,
	// one repo — not the inbound item's repo. An inbound issue on a repo that is in the intake READ
	// scope but outside the write boundary is perfectly ordinary work: its placeholder lands in the
	// scan target under a repo-stemmed name. What is NOT ordinary is the scan target itself being
	// unwritable, and that is a property of the pass, not of the item.
	if strings.TrimSpace(s.ScanTarget) == "" {
		return LaneRouting, "no-scan-target"
	}
	if !deskkit.IsAllowedRepo(s.ScanTarget) {
		return LaneRouting, "scan-target-outside-write-boundary"
	}
	known, knownErr := HasPlaceholder(s.Root, a.Item.Repo, a.Item.Number)
	switch {
	case knownErr != nil:
		// Could not read local state. The bounded direction is JUDGMENT: emit it for a model tier
		// rather than let a mechanical lane act on state it could not read.
		return LaneRouting, "unreadable-placeholder-state"
	case known:
		// An item we already have state for is an UPDATE — a comment, a resumed worker, an answered
		// decision. What it means is a judgment, never a computation.
		return LaneRouting, "update"
	}
	return LaneScanCarrierPR, "new-issue"
}

// judgmentItem renders one admitted event that a model tier must route.
func (s *ScanLoop) judgmentItem(a Admission, kind string) loopengine.Item {
	return loopengine.Item{
		ID:       a.Item.ID(),
		Gate:     "model",
		ExecTier: "strong",
		Payload: map[string]string{
			"repo":    a.Item.Repo,
			"number":  strconv.Itoa(a.Item.Number),
			"lane":    string(LaneRouting),
			"kind":    kind,
			"author":  a.Author,
			"trust":   string(a.State),
			"updated": renderTime(a.Item.UpdatedAt),
		},
	}
}

// scanBatchItem folds every mechanical item of this pass into ONE whole-scope scan dispatch.
//
// Its ID is keyed on the SCAN TARGET, not on the issues, and that is the claim key the engine's
// Claim() takes. It is the right key: what must not happen twice at once is a whole-scope scan
// against one target, and a per-issue claim never protected that. The scanner is idempotent — it
// derives the delta from the repo's own state — so serialising the scan is the whole requirement.
func (s *ScanLoop) scanBatchItem(batch []Admission) loopengine.Item {
	ids := make([]string, 0, len(batch))
	for _, a := range batch {
		ids = append(ids, a.Item.ID())
	}
	sort.Strings(ids)
	return loopengine.Item{
		ID:       "scan:" + s.ScanTarget,
		Gate:     "model",
		ExecTier: "strong",
		Payload: map[string]string{
			"repo":    s.ScanTarget,
			"lane":    string(LaneScanCarrierPR),
			"kind":    "scan-batch",
			"count":   strconv.Itoa(len(ids)),
			"members": strings.Join(ids, " "),
		},
	}
}

// membersOf returns the INBOUND items a dispatch item covers. A dispatch with no recorded
// membership covers itself — the identity case, so every caller can treat the two uniformly.
func (s *ScanLoop) membersOf(dispatchID string) []string {
	if m, ok := s.members[dispatchID]; ok && len(m) > 0 {
		return append([]string(nil), m...)
	}
	return []string{dispatchID}
}

func renderTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// TierPolicy is the MECHANICAL/JUDGMENT split, and nothing else.
//
//   - TierLocal — the item's disposition is fully determined by the scan itself (a new inbound
//     issue with no placeholder yet). The scan-carrier lane executes it.
//   - TierSession — everything else. The item is EMITTED for a model tier to route through the five
//     exits. This loop supplies the queue, the trust verdict and the ledger; the routing test and
//     the exit choice are not computed here, because a loop that guessed them would be confidently
//     wrong in the direction that compounds through every worker the resulting brief spawns.
//
// There is no TierHuman lane: this desk's human gate is the review of the PRs it opens and the
// decision queue it files into, both downstream of the drain rather than inside it.
func (s *ScanLoop) TierPolicy(it loopengine.Item) (loopengine.Tier, error) {
	if LaneName(it.Payload["lane"]) == LaneScanCarrierPR {
		return loopengine.TierLocal, nil
	}
	return loopengine.TierSession, nil
}

// reachableTiers is the set TierPolicy can emit — what a runner table would be validated against at
// boot. Intake reaches TierLocal and TierSession and never TierCheap or TierHuman.
func (s *ScanLoop) reachableTiers() []loopengine.Tier {
	return []loopengine.Tier{loopengine.TierLocal, loopengine.TierSession}
}

// Dispatch runs the item's lane through the seam and returns a Handle. A mechanical lane completes
// inline (its result is the lane's own outcome); a judgment lane EMITS and waits for the structured
// routing decision a model tier feeds back.
func (s *ScanLoop) Dispatch(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	open, err := s.openPR()
	if err != nil {
		return nil, err
	}
	lane := SelectLane(it, tier, s.Exec, s.Write)
	req := LaneRequest{
		Item:     it,
		Tier:     tier,
		Root:     s.Root,
		Worktree: s.worktreeFor(it),
		Branch:   s.branchFor(it),
		Open:     open,
		Policy:   s.Policy,
		Now:      s.now(),
		DryRun:   s.DryRun,
	}
	if open != nil && open.Branch != "" {
		decision, _ := s.Policy.Decide(open, req.Now)
		if decision.Act() == CoalesceInto {
			req.Branch = open.Branch
		}
	}

	outcome, lerr := lane.Execute(req)
	fmt.Fprintf(s.emit(), "\n=== DISPATCH %s (tier=%s lane=%s) ===\n", it.ID, tier, lane.Name())
	for _, step := range outcome.Steps {
		fmt.Fprintf(s.emit(), "  %s\n", step)
	}
	fmt.Fprintf(s.emit(), "=== END DISPATCH ===\n")

	s.mu.Lock()
	if s.outcomes == nil {
		s.outcomes = map[string]LaneOutcome{}
	}
	if s.dispatched == nil {
		s.dispatched = map[string]bool{}
	}
	s.outcomes[it.ID] = outcome
	for _, m := range s.membersOf(it.ID) {
		s.dispatched[m] = true
	}
	s.mu.Unlock()

	if lerr != nil {
		return nil, lerr
	}

	done := make(chan loopengine.Result, 1)
	if lane.Name() == LaneRouting {
		if s.Feeder == nil {
			return nil, newAwaitingRouting(it.ID)
		}
		go func() {
			r, ferr := s.Feeder(it, tier, outcome)
			if ferr != nil {
				r = loopengine.Result{Item: it, Verdict: loopengine.VerdictBlocked}
			}
			if r.Item.ID == "" {
				r.Item = it
			}
			done <- r
		}()
		return &handle{item: it, done: done}, nil
	}

	done <- loopengine.Result{
		Item:     it,
		Verdict:  loopengine.VerdictPass,
		Artifact: outcome.Artifact,
	}
	return &handle{item: it, done: done}, nil
}

// Land records exactly ONE tracked exit per INBOUND item and makes it durable. It is the drain's
// whole point: an inbound item that lands with no exit, or with two, is refused here.
//
// A batched scan dispatch lands one exit for EACH inbound item it covered. The dispatch is batched
// because the scan is whole-scope; the ledger is not, because the property being held is about the
// items that came in the front door, not about the mechanism that carried them out.
func (s *ScanLoop) Land(r loopengine.Result) error {
	s.mu.Lock()
	outcome := s.outcomes[r.Item.ID]
	members := s.membersOf(r.Item.ID)
	s.mu.Unlock()

	exit, err := ExitOf(outcome.Exit, resultExit(r))
	if err != nil {
		return err
	}
	for _, member := range members {
		rec := ExitRecord{
			ItemID:   member,
			Exit:     exit,
			Lane:     outcome.Lane,
			Artifact: firstNonEmpty(r.Artifact, outcome.Artifact),
			Verdict:  r.Verdict,
		}
		if err := s.Ledger().Record(rec); err != nil {
			return err
		}
		if s.DryRun {
			fmt.Fprintf(s.emit(), "[dry-run] exit-ledger: %s -> %s (%s)\n", rec.ItemID, rec.Exit, rec.Lane)
			continue
		}
		if err := auditExit("scanloop", rec); err != nil {
			return err
		}
	}
	return nil
}

// OnIdle is the next poll. The monitor is the refresh — SelectQueue runs a fresh cycle every time —
// so there is nothing to re-scan here, and the engine's stop-flag check on the cycle boundary is
// what ends the drain.
func (s *ScanLoop) OnIdle() error { return nil }

func (s *ScanLoop) openPR() (*OpenScanPR, error) {
	if s.OpenPR == nil {
		return nil, nil
	}
	return s.OpenPR()
}

// passName is the ONE name this pass's scan branch and scan worktree are built from. It is computed
// once and cached, and it is unique in three independent ways.
//
// The bug it closes: the name used to be derived per item from a minute-granular clock frozen for
// the whole pass, so two items in one pass produced the SAME branch. `git worktree add -b <name>`
// then failed on the second, and `git worktree remove` does not delete a branch, so the collision
// also survived into the next pass within the same minute. Batching removes the intra-pass case;
// second granularity plus a random suffix removes the cross-pass one, including two sessions
// starting in the same second.
func (s *ScanLoop) passName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.passBranch == "" {
		s.passBranch = s.now().UTC().Format("2006-01-02-150405") + "-" + newPassToken()
	}
	return s.passBranch
}

// newPassToken is a package var so a test can make the name deterministic. Production is
// crypto/rand: a name that two concurrent sessions can both produce is not a unique name.
var newPassToken = func() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A failed random read must not silently produce a CONSTANT suffix — that would reinstate
		// the collision this token exists to remove. Fall back to nanosecond precision, which is
		// still monotone-ish per process and never a fixed string.
		return strconv.FormatInt(time.Now().UnixNano()%0xffffff, 16)
	}
	return hex.EncodeToString(b[:])
}

// worktreeFor is the pass's isolated scan worktree. The path is ABSOLUTE and outside the target
// checkout; both are re-checked by the lane's own guard before anything runs.
func (s *ScanLoop) worktreeFor(loopengine.Item) string {
	base := s.WorktreeBase
	if base == "" {
		abs, err := filepath.Abs(s.Root)
		if err != nil {
			return ""
		}
		base = filepath.Dir(abs)
	}
	return filepath.Join(base, "intake-scan-"+sanitizeSegment(s.passName()))
}

// branchFor is the pass's scan branch. It is time-suffixed on purpose: the bounded coalesce window
// means one session can cut more than one scan branch in a day, so a day-granular name would
// collide with the sealed PR's branch.
func (s *ScanLoop) branchFor(loopengine.Item) string {
	return "chore/intake-scan-" + s.passName()
}

func sanitizeSegment(s string) string {
	repl := strings.NewReplacer("/", "-", "#", "-", string(filepath.Separator), "-", "..", "-")
	return repl.Replace(s)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// placeholderNameRe matches a placeholder file for issue NN, with or without the repo-name stem
// foreign-repo placeholders carry so numbers from different repos never collide.
var placeholderNameRe = regexp.MustCompile(`^(?:[a-z0-9][a-z0-9-]*-)?issue-([0-9]+)\.md$`)

// HasPlaceholder reports whether local state already carries a placeholder for this item. It is a
// LOCAL read by design: the mechanical/judgment split must not depend on a second network call that
// could disagree with the one that produced the queue.
//
// A missing scan directory is "no placeholder", not an error — a checkout that has never been
// scanned genuinely has none. An unreadable one IS an error, and the caller routes the item to
// judgment rather than letting a mechanical lane act on state it could not read.
func HasPlaceholder(root, repo string, number int) (bool, error) {
	dir := filepath.Join(root, deskkit.ScanDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, deskkit.Unverifiable("scanloop: cannot read the placeholder state under "+dir, err)
	}
	want := strconv.Itoa(number)
	short := repo
	if _, name, ok := strings.Cut(repo, "/"); ok {
		short = name
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := placeholderNameRe.FindStringSubmatch(e.Name())
		if m == nil || m[1] != want {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), "-issue-"+want+".md")
		if stem == e.Name() || stem == short {
			// Either the bare `issue-NN.md` form (the home repo) or this repo's own stem.
			return true, nil
		}
	}
	return false, nil
}

// awaitingRoutingError is what Dispatch returns for a JUDGMENT item when no result path for the
// routing decision is wired. It is a refusal to a direct caller — this loop will not invent one of
// the five exits — and the drain pass recognises the type and PARKS the item instead of failing the
// whole pass: the emission already happened, and a model tier routing it in the same session is the
// normal case, not an error.
type awaitingRoutingError struct {
	ID string
	// err carries the deskkit exit code so a caller that does not know this type still gets the
	// documented refusal (exit 5) rather than the catch-all unverifiable.
	err error
}

func newAwaitingRouting(id string) *awaitingRoutingError {
	return &awaitingRoutingError{
		ID: id,
		err: deskkit.Refused("scanloop: " + id + " was EMITTED for routing and no result feeder is wired. " +
			"Which of the five exits it takes is a judgment call; this loop refuses to invent one."),
	}
}

func (e *awaitingRoutingError) Error() string { return e.err.Error() }
func (e *awaitingRoutingError) Unwrap() error { return e.err }

// handle is the in-flight tracker. A mechanical lane's result is already on the channel when the
// handle is returned; a judgment lane's arrives when the model tier's routing is fed back. The
// engine treats both identically.
type handle struct {
	item loopengine.Item
	done chan loopengine.Result
}

func (h *handle) Done() <-chan loopengine.Result { return h.done }
func (h *handle) Item() loopengine.Item          { return h.item }

// compile-time assertion: ScanLoop is a valid drain-engine consumer. If intake ever needed a new
// hook to fit, THIS line stops compiling.
var _ loopengine.Loop = (*ScanLoop)(nil)
