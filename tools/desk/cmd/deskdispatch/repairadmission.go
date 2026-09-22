package main

// repairadmission.go — enforce the reservation at the DISPATCH BOUNDARY (example-stream/18).
//
// example-stream/05 reserved a floor of worker slots for resume/rework and example-stream/17
// made a failed verification a durable REPAIR obligation in the rework class. Both only ever
// PRINTED the reservation: fanoutloop's `classes: … (fresh capped at k by reservation)` line is
// advice a caller can ignore, and a busy dispatcher that ignored it filled every reserved slot
// with fresh work while a repair waited. This file makes deskdispatch — the existing agent-facing
// claim/worktree boundary, the one place every worker launch already passes through — refuse a
// fresh dispatch that would steal a reserved repair slot.
//
// THE DECISION vs THE SERIALIZATION. deskkit.EvaluateAdmission is the pure rule (width, reserve,
// occupancy, runnable demand -> admit/hold). This file supplies the rule's inputs from the
// authoritative sources and SERIALIZES the count->decide->claim window across dispatchers with a
// compare-and-swap LEASE in the existing claim backend, so two dispatchers on two hosts cannot
// both admit into the last slot. A process-local mutex would serialise one machine and nothing
// across two — the exact case that double-admits — so the lease is a real forge ref, taken through
// the same claim tool the item claim goes through, and bounded by that tool's own TTL on a crash.
//
// RECOVERY ORDER (interface contract). reserve (lease) -> claim (the caller's stepClaim) ->
// release (lease). A crash before the item claim leaks nothing: no item claim means no occupancy,
// and the lease expires by TTL. A crash after the item claim is a correctly-occupied slot — the
// item-claim CAS makes a resume idempotent (a second acquire of the same key is rejected as
// already-held), so no worker is duplicated. Admission is NEVER reported before BOTH the lease and
// the item claim are established: enforceAdmission hands the caller a release closure only on
// ADMIT, and the caller releases the lease only AFTER stepClaim has placed the durable item claim.
//
// OPT-IN. The whole gate is inert unless ASSAY_REPAIR_ADMISSION=on (deskkit.RepairAdmissionEnabled).
// Off — the shipped default — dispatch behaves exactly as before this brief: no lease, no reads, no
// hold. That is the rollback path (clear the env, or pin the prior binary) and the reason this is a
// strictly additive change to the dispatch flow.
//
// SCOPE / BYPASS LIMITS (documented, not hidden).
//   - This gate enforces the REWORK (repair) reservation, whose runnable demand is the
//     example-stream/17 repair-obligation sidecar — a local file, readable inside the offline
//     envelope. The RESUME reservation stays ADVISORY (fanoutloop still prints it): its orphan-PR
//     demand is a forge read outside this gate's envelope, so this gate does not fabricate a resume
//     demand it cannot authoritatively read.
//   - Occupancy is the live dispatch-claim set of the TARGET repo's claim namespace, minus the
//     admission lease. A multi-repo worker pool is therefore counted per repo namespace, not summed
//     across repos; over-counting is the safe direction (it holds fresh), under-counting is not, so
//     stale (past-TTL) claims are excluded and everything else present is counted.
//   - A raw harness launch that does NOT go through deskdispatch is outside this enforcement claim.
//     The gate lives at the dispatch verb; a worker started by hand bypasses it, exactly as it
//     bypasses the durable claim, the worktree ceremony and the model stamp.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// admissionLoop is the loop whose width/reserve this gate reads. deskdispatch's worker kit is the
// worker-desk pool; the reservation is a property of that loop (width.go).
const admissionLoop = "worker-desk"

// repairObligationLeaseTTL mirrors example-stream/17's fanoutloop repair lease horizon (45m):
// the window a repair obligation's worker lease is honoured before a dead claim makes the SAME
// obligation assignable again. The gate reads runnable demand through the same Assignable rule the
// rework source uses, so a repair under a live lease is (correctly) NOT counted as unmet demand.
const repairObligationLeaseTTL = 45 * time.Minute

// repairSidecarRel is the append-only repair-obligation projection example-stream/17 writes,
// relative to a configured root. It is the authoritative RUNNABLE-DEMAND source for the rework
// class — the same file fanoutloop's repair source reads.
var repairSidecarRel = filepath.Join("docs", "streams", "repair-obligations.jsonl")

// admissionBackend is the seam the gate reads its authoritative inputs and its serialization lease
// through. Production binds it to claimToolAdmissionBackend (below); tests inject a fake with
// in-memory state to drive concurrency, crash and could-not-check paths without a forge.
type admissionBackend interface {
	// candidateClass resolves the dispatch item's class from the AUTHORITATIVE work source — a
	// caller-supplied kit/flag can never relabel fresh work as a repair (Verify row 3).
	candidateClass() (deskkit.AdmissionClass, error)
	// reservation returns the loop's effective width and per-class reserve floor.
	reservation() (width int, reserve map[string]int, err error)
	// occupancy counts LIVE admitted worker claims in the scheduling scope (lease excluded).
	occupancy() (int, error)
	// reservedDemand returns runnable demand per reserved class and the top waiting item's name.
	reservedDemand() (demand map[string]int, waiting string, err error)
	// acquireLease serializes the count->decide->claim window. held=false with a nil error means
	// another live dispatcher holds the lease (retry next tick); a non-nil error is could-not-check.
	acquireLease() (held bool, err error)
	// releaseLease releases a lease this dispatcher acquired.
	releaseLease() error
}

// newAdmissionBackend builds the live backend for a dispatch. It is a package var so a test swaps
// the whole backend for a fake; production always resolves the claim-tool backend.
var newAdmissionBackend = func(o dispatchOpts, plan dispatchPlan, repo string, auth claimAuth) admissionBackend {
	return &claimToolAdmissionBackend{o: o, plan: plan, repo: repo, auth: auth, now: time.Now()}
}

// admissionEnabledFn is the opt-in probe, a seam so a test drives both states without touching the
// process environment mid-run.
var admissionEnabledFn = deskkit.RepairAdmissionEnabled

// enforceAdmission is the serialized dispatch-boundary reservation gate. It runs BEFORE the item
// claim (stepClaim) and returns:
//
//   - (release, nil) on ADMIT — the caller proceeds to stepClaim, then calls release() once the
//     item claim is placed (the count->decide->claim window is closed). release is never nil on
//     admit.
//   - (nil, Refused) on HOLD — a reserved slot is protected; the message names the waiting repair.
//   - (nil, Unverifiable) on any could-not-check — occupancy/demand/class/lease unreadable — NEVER a
//     fabricated free slot.
//
// When the gate is opted out it returns (nil, nil): the caller sees no release and behaves exactly
// as it did before this brief.
func enforceAdmission(o dispatchOpts, plan dispatchPlan, repo string, auth claimAuth) (release func(), err error) {
	on, version := admissionEnabledFn()
	if !on {
		return nil, nil
	}
	be := newAdmissionBackend(o, plan, repo, auth)

	// 1 — SERIALIZE. Take the admission lease first, so the occupancy count and the item claim
	//     that follows it are one atomic window across dispatchers. A lease held by another live
	//     dispatcher is could-not-serialize NOW (retry next tick), never a guessed free slot; an
	//     error taking it is could-not-check and fails closed.
	held, lerr := be.acquireLease()
	if lerr != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): the serialization lease could not be established — fail closed, "+
				"never assume a free slot", stepAdmission, version), lerr)
	}
	if !held {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): another dispatcher holds the serialization lease for this "+
				"scope — retry next tick; admitting past an unresolved lease is exactly the multi-host race "+
				"the lease exists to stop", stepAdmission, version), nil)
	}
	releaseLease := func() {
		if rerr := be.releaseLease(); rerr != nil {
			// A lease we could not release is bounded by the backend's TTL and is reported, never
			// silent — but it does not fail an otherwise-good admission, whose item claim is the
			// durable occupancy.
			o.say("%s WARNING: admission lease not released (%v) — it expires by the claim backend's TTL",
				stepAdmission, rerr)
		}
	}

	// 2 — READ every input from its authoritative source. Any could-not-check releases the lease
	//     and fails closed (exit 6): an unreadable occupancy or demand must not fabricate a slot or
	//     lose the queue.
	class, cerr := be.candidateClass()
	if cerr != nil {
		releaseLease()
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): the item's class could not be resolved from the work source",
			stepAdmission, version), cerr)
	}
	width, reserve, rerr := be.reservation()
	if rerr != nil {
		releaseLease()
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): the loop's width/reservation could not be read", stepAdmission, version), rerr)
	}
	occ, oerr := be.occupancy()
	if oerr != nil {
		releaseLease()
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): live occupancy could not be read — fail closed, never assume a "+
				"free slot", stepAdmission, version), oerr)
	}
	demand, waiting, derr := be.reservedDemand()
	if derr != nil {
		releaseLease()
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"step %s (admission, policy %s): runnable reserved demand could not be read — fail closed, never "+
				"lose the repair queue", stepAdmission, version), derr)
	}

	// 3 — DECIDE (pure). The item claim has NOT been placed yet, so occupancy is the OTHER live
	//     workers; the decision is what the gate holds the lease to make atomic with the claim.
	verdict, reason := deskkit.EvaluateAdmission(deskkit.AdmissionInputs{
		Class:          class,
		Width:          width,
		Reserve:        reserve,
		Occupancy:      occ,
		RunnableDemand: demand,
		WaitingItem:    waiting,
	})
	if verdict == deskkit.AdmissionHold {
		releaseLease()
		return nil, deskkit.Refused(fmt.Sprintf(
			"step %s (admission, policy %s): %s. This is the reservation example-stream/05 declared and "+
				"example-stream/17 filled, now enforced at the dispatch boundary instead of merely printed — "+
				"dispatch the waiting repair, or free a slot, then re-run.",
			stepAdmission, version, reason))
	}

	o.say("%s OK (policy %s): %s admitted — %s", stepAdmission, version, class, reason)
	// ADMIT — hand the lease release to the caller. The lease stays HELD through stepClaim so the
	// count->decide->claim window is atomic; the caller releases it once the item claim is the
	// durable occupancy.
	return releaseLease, nil
}

// ---- the live backend -------------------------------------------------------------------------

// claimToolAdmissionBackend reads occupancy and holds the serialization lease through the SAME
// claim tool the item claim goes through (auth-scoped exactly as stepClaim's acquire is), and reads
// runnable rework demand + the candidate's class from the example-stream/17 repair sidecar.
type claimToolAdmissionBackend struct {
	o    dispatchOpts
	plan dispatchPlan
	repo string
	auth claimAuth
	now  time.Time
}

// leaseKey is the admission lease's claim key: the scheduling scope (repo alias + loop) plus a fixed
// `--admission` leaf, so it lives in the same refs/dispatch namespace as the item claims, collides
// across dispatchers (the CAS the serialization rests on), and is trivially excluded from the
// occupancy count by its suffix.
func (b *claimToolAdmissionBackend) leaseKey() string {
	return shortRepo(b.repo) + "--" + admissionLoop + "--admission"
}

func (b *claimToolAdmissionBackend) acquireLease() (bool, error) {
	key := b.leaseKey()
	// The claim tool's own acquire reclaims a stale (past-TTL) lease on its next call, so a crashed
	// lease-holder self-heals here with no special path — the same two-phase TTL the item claim uses.
	r := runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool,
		append([]string{"acquire", key, "--repo", b.repo}, b.auth.args...)...)
	if r.err == nil {
		return true, nil
	}
	switch exitCodeOf(r.err) {
	case deskkit.ExitRefused:
		// Refused == a live holder owns the lease (a stale one would have been reclaimed above).
		// That is another dispatcher mid-admission: not our slot to take now, and NOT an error.
		return false, nil
	default:
		return false, fmt.Errorf("acquire admission lease %s in %s: %s", key, b.repo, r.run.Said())
	}
}

func (b *claimToolAdmissionBackend) releaseLease() error {
	key := b.leaseKey()
	r := runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool,
		append([]string{"release", key, "--repo", b.repo}, b.auth.args...)...)
	if r.err != nil {
		return fmt.Errorf("release admission lease %s in %s: %s", key, b.repo, r.run.Said())
	}
	return nil
}

func (b *claimToolAdmissionBackend) occupancy() (int, error) {
	r := runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool,
		append([]string{"list", "--repo", b.repo}, b.auth.args...)...)
	if r.err != nil {
		return 0, fmt.Errorf("list dispatch claims in %s: %s", b.repo, r.run.Said())
	}
	return countLiveClaims(r.stdout, b.leaseKey()), nil
}

// countLiveClaims parses the claim tool's `list` output (one `HELD <id> — <msg> at=<date> age=<Nm>`
// line per present claim) and counts the LIVE worker slots: every HELD line whose id is neither the
// admission lease nor a stale (past-TTL) claim. Stale claims are a dead worker's ref the tool
// reclaims on its next acquire, so they are effectively free and excluded — over-counting occupancy
// is the safe direction (it holds fresh), so nothing else present is dropped.
func countLiveClaims(listOut, leaseKey string) int {
	n := 0
	for _, line := range strings.Split(listOut, "\n") {
		line = strings.TrimSpace(line)
		id, ok := heldClaimID(line)
		if !ok {
			continue
		}
		if id == leaseKey || strings.HasSuffix(id, "--admission") {
			continue // the serialization lease is not a worker slot
		}
		if stale, _, _ := holderIsStale(line); stale {
			continue // a dead worker's reclaimable claim is not an occupied slot
		}
		n++
	}
	return n
}

// heldClaimID pulls the claim id out of a `dispatch-claim: HELD <id> — …` list/show line. ok=false
// for any line that is not a HELD row (the tool's `(no dispatch claims…)`, a FREE row, a blank).
func heldClaimID(line string) (string, bool) {
	i := strings.Index(line, "HELD ")
	if i < 0 {
		return "", false
	}
	rest := strings.TrimSpace(line[i+len("HELD "):])
	if rest == "" {
		return "", false
	}
	// The id is the first whitespace-delimited token after HELD.
	if j := strings.IndexAny(rest, " \t"); j >= 0 {
		return rest[:j], true
	}
	return rest, true
}

func (b *claimToolAdmissionBackend) reservation() (int, map[string]int, error) {
	width, _, werr := deskkit.ResolvedWidth(admissionLoop)
	if werr != nil {
		return 0, nil, werr
	}
	reserve, _, rerr := deskkit.ResolvedReserve(admissionLoop)
	if rerr != nil {
		return 0, nil, rerr
	}
	return width, reserve, nil
}

// reservedDemand reads the example-stream/17 repair sidecar across the configured roots, folds
// the append-only log to the current obligation per immutable key, and counts the ASSIGNABLE
// (runnable-now) ones as the rework class's runnable demand. A waiting-external obligation is not
// assignable and so contributes nothing — which is exactly why an external hold never reserves a
// slot. The resume class's demand is an orphan-PR forge read outside this gate's offline envelope,
// so it is deliberately absent from the map (advisory-only, see the file header), never fabricated.
func (b *claimToolAdmissionBackend) reservedDemand() (map[string]int, string, error) {
	obligs, err := b.reconciledObligations()
	if err != nil {
		return nil, "", err
	}
	demand := map[string]int{}
	waiting := ""
	for _, o := range obligs {
		if o.Assignable(b.now, repairObligationLeaseTTL) {
			demand[string(deskkit.AdmissionRework)]++
			if waiting == "" {
				waiting = obligationLabel(o)
			}
		}
	}
	return demand, waiting, nil
}

// candidateClass resolves the item's class from the authoritative repair source: the item is a
// REWORK candidate only when it matches an ASSIGNABLE repair obligation (by immutable id or brief).
// Anything else — including a caller that dispatched a fresh brief under a worker kit while claiming
// it is a repair — resolves to FRESH and is subject to the floor. That is the "forged repair class
// is refused" property: the class is read from the obligation store, never from the invocation.
func (b *claimToolAdmissionBackend) candidateClass() (deskkit.AdmissionClass, error) {
	obligs, err := b.reconciledObligations()
	if err != nil {
		return "", err
	}
	for _, o := range obligs {
		if !o.Assignable(b.now, repairObligationLeaseTTL) {
			continue
		}
		if itemMatchesObligation(b.o.item, o) {
			return deskkit.AdmissionRework, nil
		}
	}
	return deskkit.AdmissionFresh, nil
}

// reconciledObligations reads every configured root's repair sidecar and folds the whole
// append-only log to the current obligation per immutable key. A genuine read error on a configured
// root is could-not-check (never a silently dropped root); a missing sidecar is simply no
// obligations. It mirrors fanoutloop's collectRepairObligations, reimplemented here because that
// reader lives in the fanoutloop command package.
func (b *claimToolAdmissionBackend) reconciledObligations() ([]deskkit.RepairObligation, error) {
	roots, rerr := admissionRoots(b.o.root)
	if rerr != nil {
		return nil, rerr
	}
	var log []deskkit.RepairObligation
	for _, r := range roots {
		rows, err := readRepairSidecar(filepath.Join(r.Path, repairSidecarRel))
		if err != nil {
			return nil, err
		}
		log = append(log, rows...)
	}
	return deskkit.ReconcileObligations(log), nil
}

// admissionRoots resolves the roots to sweep for repair obligations: the configured DESK_ROOTS map
// when it is EXPLICITLY set (the cross-repo source, the same map fanoutloop's repair source uses),
// else the item's own root as a single-root fallback. Keying the fallback on whether DESK_ROOTS is
// set — not on ConfiguredRoots returning empty (it never does; unset yields the compiled default
// paths) — is what makes a single-repo dispatch read the item's OWN sidecar rather than a compiled
// default path that is not this checkout. A malformed DESK_ROOTS is a caller error surfaced as
// could-not-check, never a partial sweep.
func admissionRoots(itemRoot string) ([]deskkit.RootConfig, error) {
	if strings.TrimSpace(os.Getenv(deskkit.RootsEnv)) == "" {
		return []deskkit.RootConfig{{Path: itemRoot}}, nil
	}
	roots, err := deskkit.ConfiguredRoots()
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return []deskkit.RootConfig{{Path: itemRoot}}, nil
	}
	return roots, nil
}

// readRepairSidecar reads one append-only repair sidecar into its raw (un-reconciled) rows. A
// malformed or non-obligation line is skipped (ParseRepairObligation reports ok=false); a missing
// file is not an error (a root that never landed a failed verification simply has no obligations).
func readRepairSidecar(path string) ([]deskkit.RepairObligation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []deskkit.RepairObligation
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if o, ok := deskkit.ParseRepairObligation([]byte(line)); ok {
			out = append(out, o)
		}
	}
	return out, nil
}

// itemMatchesObligation reports whether the dispatched item key names this obligation — by its
// immutable id or its brief. Both are the keys the rework source stamps onto the dispatched item
// (repair.go: item.ID = obligation.ID, payload brief = obligation.Brief), so either identifies the
// same repair regardless of which the loop passed as the item key.
func itemMatchesObligation(item string, o deskkit.RepairObligation) bool {
	item = strings.TrimSpace(item)
	if item == "" {
		return false
	}
	return item == strings.TrimSpace(o.ID) || item == strings.TrimSpace(o.Brief)
}

// obligationLabel names an obligation for a HOLD message: its brief when it has one, else its
// immutable id. It never includes reproduction text or any private detail — only the public
// identifier a reader needs to find the waiting repair.
func obligationLabel(o deskkit.RepairObligation) string {
	if b := strings.TrimSpace(o.Brief); b != "" {
		return "repair " + b
	}
	return "repair obligation " + strings.TrimSpace(o.ID)
}
