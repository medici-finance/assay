package loopengine

// probes.go — the three house ObservableProbe implementations liveness.go's taxonomy has
// been waiting for since it was added (measured 2026-09-02 @ 30c9934: `grep -rln
// ObservableProbe tools/desk/cmd` returned zero files). liveness.go's own header names the
// three sources: audit.jsonl lines attributable to a dispatch, the claim's recorded branch
// gaining commits, and PR creation/updates. This file supplies them, each three-state per
// the ObservableProbe contract:
//
//	(obs, nil)  — checked cleanly; obs.At is the newest sign of life, zero if none seen
//	(_,   err)  — COULD-NOT-CHECK, never read as no-life (the conservative-reclaim rule)
//
// # Item identity: Payload, not a new field
//
// Item carries no dedicated dispatch-identity fields (owner/PR/branch) — Payload is the
// existing generic template-variable bag, and fanoutloop/scanloop already read "repo",
// "pr" and "branch" out of it (cmd/fanoutloop/dispatch.go, cmd/scanloop/lane.go). The
// PayloadSessionTag / PayloadRepo / PayloadPR / PayloadBranch / PayloadHeadSHA constants
// below name the same convention for the three probes here — no new Item field, no engine
// edit. A probe whose relevant key is ABSENT from Payload reports cleanly-silent
// (Observation{}, nil), never an error: a missing identity field is a configuration gap in
// the caller, not an unreachable source.
//
// # HouseProbes: the future-driver seam
//
// HouseProbes() composes all three into one *ObservableProbes with production wiring, so a
// future driver sets `Config.Observe = loopengine.HouseProbes()` and nothing else in
// engine.go/liveness.go changes. Every probe constructor (NewAuditProbe / NewBranchProbe /
// NewPRProbe) also takes its data source as an explicit seam, so probes_test.go exercises
// the three-state contract with canned data and no filesystem/network dependency — the
// offline envelope.
//
// # BranchProbe's in-process memory (a documented limit)
//
// A ref's current SHA carries no timestamp of ITS OWN — "gaining commits" is a comparison
// against the last-seen SHA, not a fact `ls-remote` states outright. NewBranchProbe closes
// over that comparison in a BranchSHAStore. HouseProbes()'s production wiring uses an
// in-memory store, which is exactly correct for a caller that keeps ONE process alive
// across polls (the engine's own Run(), or `desksupervise run --interval`): the store
// persists for the process's life, the same way livenessTracker's own state does. A
// caller that re-execs a fresh process every tick (`desksupervise tick` run from an
// external scheduler, one process per invocation) has no prior SHA to compare against on
// its first read of any item after the process restarts — a known, stated limit, not a
// silent one; see cmd/desksupervise's own doc comment on `tick`.

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// Payload* name the Item.Payload keys the house probes read to attribute activity to a
// dispatch. They are not a new Item field — see the file doc above.
const (
	// PayloadSessionTag is the claim's recorded owner — matched against
	// deskkit.Entry.SessionTag by AuditProbe.
	PayloadSessionTag = "sessionTag"
	// PayloadRepo is the "owner/name" repo slug BranchProbe and PRProbe address.
	PayloadRepo = "repo"
	// PayloadPR is the PR number (decimal string) PRProbe reads and AuditProbe may
	// additionally match against (paired with PayloadRepo).
	PayloadPR = "pr"
	// PayloadBranch is the branch BranchProbe polls.
	PayloadBranch = "branch"
	// PayloadHeadSHA is a head commit SHA AuditProbe may match against.
	PayloadHeadSHA = "headSHA"
)

// --- AuditProbe ---

// AuditEntry is the subset of deskkit.Entry AuditProbe reads. Kept as its own small shape
// (rather than importing deskkit.Entry into every AuditLoader signature) so a caller that
// is not deskkit-backed — a test, or a future non-file audit sink — can supply an
// AuditLoader with no deskkit dependency at all.
type AuditEntry struct {
	TS         string
	SessionTag string
	Repo       string
	PR         *int
	HeadSHA    *string
}

// AuditLoader reads the full audit history. Production points it at deskkit.LoadEntries
// (adapted by loadHouseAuditEntries); a test injects a canned slice.
type AuditLoader func() ([]AuditEntry, error)

// NewAuditProbe returns an ObservableProbe over an audit trail: the newest entry
// timestamped strictly after `since` whose sessionTag / repo+pr / headSHA attributes it to
// `it` (via the Payload* keys). An entry with none of those three attribution keys set on
// `it.Payload` cannot be attributed to anything, ever — that is a caller configuration gap,
// reported as cleanly-silent rather than as an error. An unreadable audit source IS an
// error (could-not-check) — audit.jsonl existing-but-unreadable must never be read as "no
// audit activity happened".
func NewAuditProbe(load AuditLoader) ObservableProbe {
	return func(it Item, since time.Time) (Observation, error) {
		sessionTag := strings.TrimSpace(it.Payload[PayloadSessionTag])
		repo := strings.TrimSpace(it.Payload[PayloadRepo])
		pr := strings.TrimSpace(it.Payload[PayloadPR])
		headSHA := strings.TrimSpace(it.Payload[PayloadHeadSHA])
		if sessionTag == "" && (repo == "" || pr == "") && headSHA == "" {
			return Observation{}, nil
		}
		entries, err := load()
		if err != nil {
			return Observation{}, deskkit.Unverifiable("AuditProbe: cannot read the audit trail for "+it.ID, err)
		}
		var best Observation
		for _, e := range entries {
			ts, perr := time.Parse(time.RFC3339, e.TS)
			if perr != nil || !ts.After(since) {
				continue
			}
			matched := (sessionTag != "" && e.SessionTag == sessionTag) ||
				(repo != "" && pr != "" && e.Repo == repo && e.PR != nil && strconv.Itoa(*e.PR) == pr) ||
				(headSHA != "" && e.HeadSHA != nil && *e.HeadSHA == headSHA)
			if matched && ts.After(best.At) {
				best = Observation{At: ts, What: "audit line"}
			}
		}
		return best, nil
	}
}

// loadHouseAuditEntries adapts deskkit.LoadEntries to the AuditLoader shape HouseProbes
// wires into NewAuditProbe.
func loadHouseAuditEntries() ([]AuditEntry, error) {
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return nil, err
	}
	out := make([]AuditEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, AuditEntry{TS: e.TS, SessionTag: e.SessionTag, Repo: e.Repo, PR: e.PR, HeadSHA: e.HeadSHA})
	}
	return out, nil
}

// --- BranchProbe ---

// BranchLister lists a remote repo's refs, refname (e.g. "refs/heads/topic") -> resolved
// SHA (hex). Production points it at an in-process go-git ls-remote-equivalent
// (gitcore.List, see houseBranchLister); a test injects a canned map.
type BranchLister func(repoSlug string) (map[string]string, error)

// BranchSHAStore remembers the last-seen SHA per item, across probe calls, so a SHA change
// (not mere existence) is what proves life — see the file doc's "BranchProbe's in-process
// memory" note.
type BranchSHAStore interface {
	Get(itemID string) (sha string, ok bool)
	Set(itemID, sha string)
}

// memBranchSHAStore is the default in-process store: correct for any caller that keeps one
// process alive across polls, and what every test uses.
type memBranchSHAStore struct {
	mu   sync.Mutex
	seen map[string]string
}

// NewMemBranchSHAStore returns a fresh in-process BranchSHAStore.
func NewMemBranchSHAStore() BranchSHAStore {
	return &memBranchSHAStore{seen: map[string]string{}}
}

func (s *memBranchSHAStore) Get(itemID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sha, ok := s.seen[itemID]
	return sha, ok
}

func (s *memBranchSHAStore) Set(itemID, sha string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[itemID] = sha
}

// NewBranchProbe returns an ObservableProbe over a claim's recorded branch: it lists the
// repo, resolves "refs/heads/<branch>", and treats a SHA different from the store's
// last-seen value for this item as a fresh observation AT THE TIME OF THIS CALL — ls-remote
// reports current state, never history, so the observation time is the poll time, not an
// earlier push time. The first read for an item (no prior entry in store) is always
// reported fresh: there is nothing yet to compare against, so a first sighting is itself
// the sign of life. A missing branch (present repo, absent ref) is cleanly-silent, not an
// error — it may simply not exist yet. A lister error (network/auth failure) IS an error.
func NewBranchProbe(list BranchLister, store BranchSHAStore) ObservableProbe {
	return func(it Item, since time.Time) (Observation, error) {
		branch := strings.TrimSpace(it.Payload[PayloadBranch])
		repo := strings.TrimSpace(it.Payload[PayloadRepo])
		if branch == "" || repo == "" {
			return Observation{}, nil
		}
		refs, err := list(repo)
		if err != nil {
			return Observation{}, deskkit.Unverifiable("BranchProbe: cannot list "+repo+" for branch "+branch, err)
		}
		sha, ok := refs["refs/heads/"+branch]
		if !ok {
			return Observation{}, nil
		}
		last, hadLast := store.Get(it.ID)
		store.Set(it.ID, sha)
		if !hadLast || last != sha {
			return Observation{At: time.Now().UTC(), What: "branch sha moved"}, nil
		}
		return Observation{}, nil
	}
}

// houseBranchLister is the production BranchLister: an in-process go-git remote listing
// (gitcore.List), no external git binary, no credential helper. Anonymous (public) read —
// a private repo surfaces as a lister error, which is correctly could-not-check, never
// no-life. Assumes a GitHub-hosted remote (github.com/<repoSlug>.git); a forge-neutral
// cutover is future work (see the forge-neutral stream), not this brief.
func houseBranchLister(repoSlug string) (map[string]string, error) {
	refs, err := gitcore.List(gitcore.ListOpts{URL: "https://github.com/" + repoSlug + ".git"})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(refs))
	for _, r := range refs {
		out[string(r.Name())] = r.Hash().String()
	}
	return out, nil
}

// --- PRProbe ---

// PRReader reads one PR's updated-at timestamp. Production points it at
// housePRReader (deskkit's Forge read path); a test injects a canned function.
// found=false means the PR does not exist (never seen, or deleted) — treated as
// cleanly-silent, not an error, since a not-yet-existing PR is not a failed check.
type PRReader func(repoSlug string, number int) (updatedAt time.Time, found bool, err error)

// NewPRProbe returns an ObservableProbe over a claim's recorded PR: the PR's own
// updated-at, reported as a fresh observation when it is after `since`. A read error
// (auth, network, malformed response) is could-not-check.
func NewPRProbe(read PRReader) ObservableProbe {
	return func(it Item, since time.Time) (Observation, error) {
		repo := strings.TrimSpace(it.Payload[PayloadRepo])
		prStr := strings.TrimSpace(it.Payload[PayloadPR])
		if repo == "" || prStr == "" {
			return Observation{}, nil
		}
		number, cerr := strconv.Atoi(prStr)
		if cerr != nil || number <= 0 {
			return Observation{}, nil // an unparseable PR number is a configuration gap, not a probe failure
		}
		updatedAt, found, err := read(repo, number)
		if err != nil {
			return Observation{}, deskkit.Unverifiable("PRProbe: cannot read "+repo+" PR "+prStr, err)
		}
		if !found || updatedAt.IsZero() || !updatedAt.After(since) {
			return Observation{}, nil
		}
		return Observation{At: updatedAt, What: "PR updated"}, nil
	}
}

// housePRReader is the production PRReader: an App installation token minted for the PR's
// owner (deskkit.SessionTokenRole / RoleTokenForOwner — the same read-path token
// resolution deskboard uses) reading through deskkit.Forge.GetPullRequest. Unlike
// deskboard's own read path, there is deliberately NO ambient-identity fallback here: a
// token this probe cannot mint is a could-not-check, never a silent "proceed unauthenticated
// and treat a 401 as no update" — the conservative-reclaim rule binds probes harder than it
// binds a display board.
func housePRReader(repoSlug string, number int) (time.Time, bool, error) {
	owner, name, ok := strings.Cut(repoSlug, "/")
	if !ok || owner == "" || name == "" {
		return time.Time{}, false, deskkit.Unverifiable("housePRReader: "+repoSlug+" is not owner/name", nil)
	}
	role, _, rerr := deskkit.SessionTokenRole("desksupervise")
	if rerr != nil {
		return time.Time{}, false, rerr
	}
	token, _, terr := deskkit.RoleTokenForOwner(role, owner)
	if terr != nil {
		return time.Time{}, false, terr
	}
	forge := &deskkit.GitHubForge{Token: token}
	pr, gerr := forge.GetPullRequest(deskkit.ForgeRepo{Owner: owner, Name: name}, number)
	if gerr != nil {
		if deskkit.IsForgeNotFound(gerr) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, gerr
	}
	if pr.UpdatedAt == "" {
		return time.Time{}, true, nil
	}
	ts, perr := time.Parse(time.RFC3339, pr.UpdatedAt)
	if perr != nil {
		return time.Time{}, false, deskkit.Unverifiable("housePRReader: unparseable updated_at "+pr.UpdatedAt, perr)
	}
	return ts, true, nil
}

// --- HouseProbes ---

// HouseProbes composes the three house probes with their production wiring into one
// *ObservableProbes, so a future driver sets `Config.Observe = loopengine.HouseProbes()`
// and nothing else in engine.go/liveness.go changes. Call it once per long-running process
// (it owns the in-process BranchSHAStore the branch probe needs) — see the file doc's
// "BranchProbe's in-process memory" note.
func HouseProbes() *ObservableProbes {
	return &ObservableProbes{
		AuditScan:   NewAuditProbe(loadHouseAuditEntries),
		BranchMoved: NewBranchProbe(houseBranchLister, NewMemBranchSHAStore()),
		PRActivity:  NewPRProbe(housePRReader),
	}
}

// --- Disposition: the taxonomy re-exported for an external claim reader ---

// Disposition mirrors livenessDisposition's four-way verdict, exported so a caller outside
// package loopengine — desksupervise, which reads claims through the forge rather than the
// engine's own in-flight tracker — can classify against the IDENTICAL taxonomy evaluate()
// runs internally. The iota order matches livenessDisposition's exactly (TestClassifyMatchesEngineOrder
// pins it), which is what makes the Disposition(...) conversion in Classify safe.
type Disposition int

const (
	Alive               Disposition = iota // within every timer — leave it running
	ReclaimNeverStarted                    // schedule-to-start expiry (never booted)
	ReclaimHeartbeat                       // heartbeat-gap expiry (stopped emitting)
	BlockedStartToClose                    // start-to-close wall cap (running too long)
)

// String renders the observer-facing status token for each Disposition — the vocabulary
// `desksupervise tick`'s output line uses (cmd/desksupervise).
func (d Disposition) String() string {
	switch d {
	case Alive:
		return "ALIVE"
	case ReclaimNeverStarted:
		return "NEVER-STARTED"
	case ReclaimHeartbeat:
		return "HEARTBEAT-EXPIRED"
	case BlockedStartToClose:
		return "OVER-WALL-CAP"
	default:
		return "UNKNOWN"
	}
}

// ClaimClock is the caller-owned liveness clock for one externally-tracked claim: the
// exported counterpart of livenessState's three timestamps, for a reader (desksupervise)
// that is not the engine's own livenessTracker.
type ClaimClock struct {
	DispatchedAt time.Time
	StartedAt    time.Time
	LastObserved time.Time
}

// ClassifyLiveness applies pol's taxonomy to one claim's clock at `now`, given this cycle's
// cross-probe observation (obs — a zero Observation means "checked cleanly, nothing seen").
// It is evaluate()'s exact rule, reused rather than re-implemented, so an external reader
// gets a taxonomy verdict byte-for-byte identical to what the engine's own sweepLiveness
// would have computed from the same clock. (Named ClassifyLiveness, not Classify — retry.go
// already exports Classify for the dispatch-error taxonomy; the two are unrelated
// classifiers over unrelated domains.) The caller is responsible for the could-not-check
// branch: ClassifyLiveness assumes the probe ran cleanly this cycle (observed=true) — a
// caller whose probe errored reports COULD-NOT-CHECK itself and never reaches this function
// for that cycle (see liveness.go's own "observed is the conservative-reclaim hinge").
func ClassifyLiveness(pol LivenessPolicy, clock ClaimClock, tier Tier, now time.Time, obs Observation) Disposition {
	s := &livenessState{
		tier:         tier,
		dispatchedAt: clock.DispatchedAt,
		startedAt:    clock.StartedAt,
		lastObserved: clock.LastObserved,
	}
	s.observe(obs.At)
	return Disposition(pol.evaluate(s, now, true))
}

// Exported journal event-kind aliases for JournalObserverDecision's kind argument — the
// SAME wire vocabulary journalEvent uses (journal.go), so an observer-driven decision
// parses through parseJournalRecord exactly like one the engine's own Run() loop journals.
const (
	EventReclaim = journalReclaim // a claim was freed (schedule-to-start or heartbeat expiry)
	EventLand    = journalLand    // landed blocked-timeout (start-to-close expiry)
)

// JournalObserverDecision writes one desksupervise scheduling decision to the shared audit
// stream, in journalEvent's identical wire shape (Tool="loopengine", Detail=journalRecord
// JSON) — so recover.go's replay treats a decision desksupervise made exactly like one the
// engine's own Run() loop would have journalled. runTag is a per-invocation stamp
// (desksupervise has no Run() of its own, so this is NOT a Run()-lineage runID — recover.go's
// "prior run of me" reasoning never keys off it).
func JournalObserverDecision(loopName, kind, item, tier, outcome, note, runTag string) {
	rec := journalRecord{Loop: loopName, Item: item, RunID: runTag, Tier: tier, Outcome: outcome, Note: note}
	detail, merr := json.Marshal(rec)
	if merr != nil {
		return
	}
	_ = deskkit.Log(deskkit.Entry{
		Tool:   "loopengine",
		Verb:   kind,
		Result: journalResultFor(kind),
		Detail: string(detail),
	})
}
