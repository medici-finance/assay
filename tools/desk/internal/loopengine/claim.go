package loopengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ExitAuthorIsRunner is the DISTINCT exit code for the author != runner structural
// refusal. It is intentionally outside deskkit's canonical set (3 disabled / 4
// rate-limited / 5 refused / 6 unverifiable) so a caller can tell "refused because the
// dispatch target is the item's own implementer" apart from every other refusal — the
// guard is a typed exit code, never a prompt-carried rule.
const ExitAuthorIsRunner = 7

// AuthorIsRunnerError is the typed refusal the engine raises when it is asked to dispatch
// an item to the identity that implemented it (verifier == author is void; arch doc §1.1
// "author != runner becomes a typed guard"). It carries a distinct ExitCode.
type AuthorIsRunnerError struct {
	ItemID    string
	Runner    string
	BriefPath string
}

func (e *AuthorIsRunnerError) Error() string {
	return fmt.Sprintf("refused: author == runner for %s — %q implemented it and may not verify it (brief %s)",
		e.ItemID, e.Runner, e.BriefPath)
}

// ExitCode returns the distinct author==runner exit code.
func (e *AuthorIsRunnerError) ExitCode() int { return ExitAuthorIsRunner }

// CheckAuthorRunner is the structural author != runner guard: a pure comparison between the
// item's recorded implementer and the identity the engine would dispatch as (runnerID).
// Returns *AuthorIsRunnerError (exit 7) when they match; nil otherwise. An empty runnerID
// or empty Implementer disables the check (the adapter owns attribution in that case).
func CheckAuthorRunner(it Item, runnerID string) error {
	if runnerID == "" || it.Implementer == "" {
		return nil
	}
	if it.Implementer == runnerID {
		return &AuthorIsRunnerError{ItemID: it.ID, Runner: runnerID, BriefPath: it.BriefPath}
	}
	return nil
}

// ExitCodeOf maps a loopengine error to a process exit code. It recognizes the distinct
// author==runner code (7) and otherwise delegates to deskkit.ExitCodeOf (0/3/4/5/6, failing
// closed on any unexpected error). A tool's main() uses this to compute its status.
func ExitCodeOf(err error) int {
	if err == nil {
		return deskkit.ExitOK
	}
	var are *AuthorIsRunnerError
	if errors.As(err, &are) {
		return are.ExitCode()
	}
	return deskkit.ExitCodeOf(err)
}

// Claim/ReleaseClaim are now THIN DELEGATES over the canonical deskkit claim primitive
// (deskkit.Acquire/Release, Kind="dispatch") — the flock-backed lock promoted to the canonical
// primitive. The engine's public behaviour is unchanged: (true,nil) acquired,
// (false,nil) live collision, (false,err) fail-closed. The atomic create-or-reclaim, the
// stale-claim policy (arch doc §4: ">120 min, no branch"), and the in-place reclaim
// (never Remove-then-create) now live in deskkit/claim.go, which additionally serialises
// the whole decision under a directory-wide flock. The on-disk shape moved from
// {itemID,runner,branch,claimed} to the canonical {kind,item,owner,branch,ts}; it is read
// back tolerantly by both deskkit.List and claimRecord below, so nothing that inspects an
// existing on-disk claim breaks during rollout.

// toClaimConfig maps loopengine's Config onto the deskkit primitive's ClaimConfig,
// carrying the ClaimsDir / StaleClaim / branch-liveness semantics across unchanged.
func toClaimConfig(cfg Config) deskkit.ClaimConfig {
	return deskkit.ClaimConfig{
		ClaimsDir:    cfg.ClaimsDir,
		StaleClaim:   cfg.StaleClaim,
		BranchActive: cfg.BranchActive,
	}
}

// claimPath returns the claim file path for an item in cfg.ClaimsDir. It MUST stay
// byte-identical to deskkit.claimPath (which the delegate actually writes through) so a
// claim written by Claim is found at this path — the loopengine tests read it here.
func claimPath(cfg Config, it Item) string {
	safe := strings.NewReplacer("/", "_", string(os.PathSeparator), "_", "..", "_").Replace(it.ID)
	return filepath.Join(cfg.ClaimsDir, safe+".claim")
}

// claimRecord is a TOLERANT reader of an on-disk claim, kept for loopengine's own tests
// and any local inspection. The canonical writer is now deskkit, whose on-disk shape is
// {kind,item,owner,branch,ts}; this reader also accepts the historical loopengine shape
// {itemID,runner,branch,claimed}. Field names map: item|itemID → ItemID, owner|runner →
// Runner, ts|claimed → Claimed.
type claimRecord struct {
	ItemID  string
	Runner  string
	Branch  string
	Claimed string
}

func (r *claimRecord) UnmarshalJSON(data []byte) error {
	var a struct {
		// canonical (deskkit)
		Item   string `json:"item"`
		Owner  string `json:"owner"`
		Branch string `json:"branch"`
		TS     string `json:"ts"`
		// legacy loopengine
		ItemID  string `json:"itemID"`
		Runner  string `json:"runner"`
		Claimed string `json:"claimed"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	r.ItemID = firstNonEmpty(a.ItemID, a.Item)
	r.Runner = firstNonEmpty(a.Runner, a.Owner)
	r.Branch = a.Branch
	r.Claimed = firstNonEmpty(a.Claimed, a.TS)
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Claim is the SINGLE MANDATORY ENTRY POINT for dispatch, and the home of the
// dedupe-at-start guarantee stated in doc.go: an item ID is dispatched at most once
// concurrently across all consumers sharing the claims dir, for dispatchers that route
// through here. A dispatcher that bypasses Claim is outside the guarantee.
//
// It runs TWO checks, in this order, and reports which ones it actually ran:
//
//  1. cfg.WorkEvidence — the external-evidence probe (open/merged PRs naming the item,
//     the board row). A claim file only records "this dispatcher is working on it"; it
//     is not evidence that the work is untaken. When the probe is UNSET, Claim consults
//     only the claims dir and announces that bound on cfg.Progress — no silent caps.
//  2. deskkit.Acquire (Kind="dispatch") — the flock-backed atomic create-or-reclaim.
//
// Returns:
//
//   - (true, nil)  claim acquired (fresh, or a STALE claim reclaimed), bounded by
//     whichever checks above actually ran;
//   - (false, nil) do NOT dispatch — a live claim is held elsewhere, or WorkEvidence
//     reported the item already taken (a "DEDUP <id> — <why>" line names which);
//   - (false, err) COULD-NOT-CHECK: the lock could not be held, the claim could not be
//     read/written, or WorkEvidence could not reach its source. Fail closed — an
//     unreachable check is NEVER "assume free" (#146), and a caller must not read this
//     as a soft acquired.
//
// The probe runs BEFORE the flock on purpose: a network call must not be made while a
// directory-wide lock is held. The consequence is stated in doc.go — probe-then-lock is
// not one atomic step.
func Claim(cfg Config, it Item) (bool, error) {
	if cfg.ClaimsDir == "" {
		return false, fmt.Errorf("loopengine: Claim needs a ClaimsDir")
	}
	if taken, err := checkWorkEvidence(cfg, it); err != nil || taken {
		return false, err
	}
	ok, err := deskkit.Acquire(toClaimConfig(cfg), deskkit.Claim{
		Kind:   deskkit.KindDispatch,
		Item:   it.ID,
		Owner:  cfg.RunnerID,
		Branch: cfg.Branch,
	})
	if ok {
		cfg.logf("claimed: %s", it.ID)
	}
	return ok, err
}

// WorkEvidence is the external-evidence probe Claim consults before it will return
// "acquired". It answers ONE question — "is this item already taken by evidence outside
// the claims dir?" — and answers it in three states, never two:
//
//	(true,  why, nil)  taken; why NAMES the evidence (e.g. "open PR #722 (2026-08-12)")
//	(false, _,   nil)  checked, and nothing says the item is taken
//	(_,     _,   err)  COULD-NOT-CHECK — the source was unreachable/unreadable
//
// The third state is the load-bearing one. A probe that cannot reach the GitHub API MUST
// return an error, NOT (false, "", nil): reporting "nothing found" when nothing could be
// looked at is the fail-open dedupe that this contract exists to prevent. Claim turns an
// errored probe into deskkit.Unverifiable (exit 6) and refuses to acquire.
//
// The engine deliberately does NOT implement a probe: what counts as evidence is
// per-consumer (worker-desk looks at open+merged PRs naming the brief and the board row;
// verifyloop looks at the verify queue), and hard-coding GitHub into the engine would put
// a network dependency inside the frozen contract. The engine states the shape and the
// three-state obligation; the consumer supplies the source.
type WorkEvidence func(it Item) (taken bool, why string, err error)

// checkWorkEvidence runs cfg.WorkEvidence and maps it onto Claim's return. It reports
// (taken, err); Claim refuses on either. With no probe configured it emits the BOUND on
// cfg.Progress — the "no silent caps" rule: a check that narrowed what it consulted says
// so in its own output rather than letting the caller infer a wider guarantee.
func checkWorkEvidence(cfg Config, it Item) (bool, error) {
	if cfg.WorkEvidence == nil {
		cfg.logf("claim: BOUND %s — evidence consulted is the claims dir ONLY (no WorkEvidence probe configured); open/merged PRs and the board row were NOT checked", it.ID)
		return false, nil
	}
	taken, why, err := cfg.WorkEvidence(it)
	if err != nil {
		cfg.logf("claim: COULD-NOT-CHECK %s — work-evidence probe failed: %v (fail closed; NOT acquired)", it.ID, err)
		return false, deskkit.Unverifiable("loopengine: Claim could not check work evidence for "+it.ID, err)
	}
	if taken {
		if why == "" {
			why = "work-evidence probe reported the item taken but named no evidence"
		}
		cfg.logf("DEDUP %s — %s", it.ID, why)
		return true, nil
	}
	return false, nil
}

// ReleaseClaim removes an item's claim (used when a dispatch fails after claiming, so the
// slot is handed back rather than parked behind a stale claim). A missing file is a no-op.
func ReleaseClaim(cfg Config, it Item) error {
	return deskkit.Release(toClaimConfig(cfg), it.ID)
}
