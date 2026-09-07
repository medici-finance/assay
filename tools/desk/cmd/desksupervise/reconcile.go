package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// reconcile.go (cmd) — the ELIGIBILITY-RECONCILIATION step `tick` runs FIRST, before the
// liveness sweep. For every state=dispatched claim it joins three reads (loopengine.Eligibility)
// and, for an INELIGIBLE claim, STOPS the run — arming the per-run stop flag and, for a TERMINAL
// verdict only, releasing the claim for re-dispatch — then skips it from the liveness step.
// A claim whose reconciliation could not be READ (could-not-check) is kept running, logged
// BLIND(<source>), and retried next tick: the same three-state rule the liveness step obeys.
//
// This makes "a merged or closed PR is DONE, stop, never push its branch again" a mechanical
// check that fires within one observer interval instead of a sentence a worker has to remember.
//
// Stop + release reuse the SAME seams the liveness reclaim uses (armRunStopFunc / reclaimFunc),
// so this step adds no new write primitive and its dry-run suppression is testable without a
// real forge — exactly as actions.go's reclaim/file seams are.

// eligibilitySource resolves one claim's reconciliation verdict for this tick. The fixture
// path reads the claim's embedded eligibilityFixture; the live path (liveEligibilitySource)
// joins real forge/git reads.
type eligibilitySource func(claim claimRecord) (loopengine.EligibilityVerdict, error)

// reconcileResult is one claim's reconciliation outcome, for a caller (tick's audit count, or
// a test) that wants the structured record rather than a parsed stdout line.
type reconcileResult struct {
	Claim   claimRecord
	Verdict loopengine.EligibilityVerdict
	Blind   bool
	Source  string // set when Blind: the read that could not be performed
	Action  string // STOP+RELEASE | STOP | none (blind)
}

// reconcile classifies every claim's eligibility once, executing (or, under dryRun, only
// printing) the STOP/RELEASE implied by an INELIGIBLE verdict, and returns the ELIGIBLE subset
// for the liveness step plus whether any claim was BLIND (the caller maps that to exit 6). One
// rendered line is written to out per non-eligible claim; an eligible claim prints nothing here
// (the liveness step prints its line).
//
// A non-blind error from the source (a config error in the tick's own input, not a liveness or
// eligibility question) aborts the WHOLE tick as could-not-check — the same fail-closed shape
// sweep uses for a malformed claim record.
func reconcile(claims []claimRecord, src eligibilitySource, now time.Time, dryRun bool, reclaim reclaimFunc, arm armRunStopFunc, out io.Writer) (eligible []claimRecord, results []reconcileResult, anyBlind bool, err error) {
	runTag := fmt.Sprintf("desksupervise-%d", now.UnixNano())

	for _, claim := range claims {
		verdict, verr := src(claim)
		if verr != nil {
			source, ok := loopengine.BlindSource(verr)
			if !ok {
				return eligible, results, anyBlind, deskkit.Unverifiable("claim "+claim.Key+": reconcile read failed", verr)
			}
			// Could-not-check: keep the run, do not act, and count it blind (exit 6). The
			// line deliberately does NOT contain the substrings INELIGIBLE or STOP — a blind
			// read is not a verdict.
			anyBlind = true
			results = append(results, reconcileResult{Claim: claim, Blind: true, Source: source, Action: "none"})
			fmt.Fprintf(out, "%s  RECONCILE-BLIND last=none via=reconcile action=BLIND(%s)\n", claim.Key, source)
			continue
		}
		if verdict.Eligible() {
			eligible = append(eligible, claim)
			continue
		}

		action := "STOP"
		release := false
		if verdict.Terminal() {
			action = "STOP+RELEASE"
			release = true
		}
		if aerr := runReconcileAction(claim, verdict, release, dryRun, reclaim, arm, runTag); aerr != nil {
			return eligible, results, anyBlind, deskkit.Unverifiable("claim "+claim.Key+": reconcile action failed", aerr)
		}
		results = append(results, reconcileResult{Claim: claim, Verdict: verdict, Action: action})
		fmt.Fprintf(out, "%s  INELIGIBLE(%s) via=reconcile action=%s\n", claim.Key, verdict.Reason, action)
	}
	return eligible, results, anyBlind, nil
}

// runReconcileAction executes (or, under dryRun, only PRINTS) the STOP an INELIGIBLE verdict
// implies, and journals the decision as SUPERSEDED when it actually ran. Terminal → arm the
// per-run stop, RELEASE the claim, journal. Held → arm the per-run stop and journal, but do
// NOT release (a human or another desk owns the next move; releasing would let a fresh worker
// re-dispatch straight into the held state). Arming precedes releasing for the same reason the
// liveness reclaim arms first: a still-live wedged worker must be halted by Layer A before its
// claim is freed.
func runReconcileAction(claim claimRecord, verdict loopengine.EligibilityVerdict, release, dryRun bool, reclaim reclaimFunc, arm armRunStopFunc, runTag string) error {
	reason := verdict.Reason
	if dryRun {
		if release {
			fmt.Fprintf(os.Stderr, "[dry-run] would arm STOP.run.%s, release dispatch claim %s, journal SUPERSEDED reason=%s\n", claim.Key, claim.Key, reason)
		} else {
			fmt.Fprintf(os.Stderr, "[dry-run] would arm STOP.run.%s, journal SUPERSEDED reason=%s (HELD — claim NOT released)\n", claim.Key, reason)
		}
		fireAfterRun(claim, true)
		return nil
	}
	if aerr := arm(claim, fmt.Sprintf("desksupervise reconcile: INELIGIBLE(%s) %s (%s)", reason, claim.Key, runTag)); aerr != nil {
		return aerr
	}
	if release {
		if rerr := reclaim(claim); rerr != nil {
			return rerr
		}
	}
	loopengine.JournalObserverDecision("desksupervise", loopengine.EventLand, claim.Item, claim.Tier, "SUPERSEDED", "reason="+reason, runTag)
	fireAfterRun(claim, false)
	return nil
}

// fixtureEligibilitySource is the offline eligibilitySource: it reads each claim's embedded
// eligibilityFixture. A claim with NO block is Eligible (not a reconcile fixture — defer to the
// liveness step), which is what lets the liveness-only fixtures pass through this step untouched.
func fixtureEligibilitySource() eligibilitySource {
	return func(c claimRecord) (loopengine.EligibilityVerdict, error) {
		if c.Eligibility == nil {
			return loopengine.Eligible, nil
		}
		return loopengine.Eligibility(toLoopClaim(c, ""), c.Eligibility.readers(c))
	}
}

// toLoopClaim projects a cmd claimRecord onto the loopengine.ClaimRecord the pure Eligibility
// reasons over. The claim's dispatched owner is its Holder (the identity the current-holder
// read is compared against).
func toLoopClaim(c claimRecord, root string) loopengine.ClaimRecord {
	return loopengine.ClaimRecord{
		Key:    c.Key,
		Item:   c.Item,
		Root:   root,
		Repo:   c.Repo,
		PR:     c.PR,
		Holder: c.Owner,
	}
}

// errFixtureUnreadable is the canned could-not-check a *Unreadable fixture flag injects, so the
// BLIND(<source>) path is reachable offline.
var errFixtureUnreadable = errors.New("fixture: read forced could-not-check")

// readers builds the three injected reads from the fixture block. Each *Unreadable flag makes
// that one read fail (could-not-check); otherwise the zero value of each field is the eligible
// reading.
func (f *eligibilityFixture) readers(c claimRecord) loopengine.EligibilityReaders {
	return loopengine.EligibilityReaders{
		Claim: func(key string) (loopengine.ClaimRecord, error) {
			if f.ClaimUnreadable {
				return loopengine.ClaimRecord{}, errFixtureUnreadable
			}
			holder := c.Owner
			if f.ClaimReleased {
				holder = ""
			} else if f.ClaimHolder != "" {
				holder = f.ClaimHolder
			}
			return loopengine.ClaimRecord{Key: key, Holder: holder}, nil
		},
		BoardRow: func(root, item string) (loopengine.Status, error) {
			if f.BoardUnreadable {
				return "", errFixtureUnreadable
			}
			st := f.BoardStatus
			if st == "" {
				st = "in-progress"
			}
			return loopengine.Status(st), nil
		},
		PR: func(repo string, n int) (loopengine.PRState, error) {
			if f.PRUnreadable {
				return loopengine.PRState{}, errFixtureUnreadable
			}
			state := f.PRState
			if state == "" {
				state = "open"
			}
			return loopengine.PRState{
				Kind:        deskkit.ParsePRKind(state),
				Disposition: deskkit.DispositionVerdict(f.PRDisposition),
				Labels:      f.PRLabels,
			}, nil
		},
	}
}

// liveEligibilitySource is the PRODUCTION eligibilitySource. Like live.go, it is not — and,
// under the offline envelope, cannot be — exercised by this session's Verify table; it is
// written to the conventions this tree already uses and is exactly as honest as those let it
// be: a failure on ANY read surfaces as a could-not-check *BlindError (keep the run), never a
// guessed verdict.
//
//   - Claim: the consumer's own tools/dispatch-claim.sh `show <key> --repo <repo>` (the same
//     external contract readLiveClaims and cmd/deskdispatch already shell to), parsing owner=.
//   - BoardRow: the item's Status cell on refs/remotes/origin/main, read in-process with
//     gitcore.FileAt and parsed by loopengine.ParseBriefRowStatus (the pure parser this
//     brief's reconcile_test covers offline).
//   - PR: deskkit.Forge.GetPullRequest for the claim's PR, resolved through the single
//     ForgeFor construction site exactly as actions.go's doReclaim resolves the forge.
//
// KNOWN GAP: deskkit.PullRequest.State reduces to open|closed, folding a MERGED PR into
// "closed", so the live reader reports reason `pr-closed` for a merged PR. Both are
// IneligibleTerminal with the identical STOP+RELEASE action, so the DECISION is correct; only
// the reason TAG loses the merged/closed distinction the fixture path proves independently.
// Surfacing merged distinctly is follow-on work (a merged_at read), flagged rather than
// silently assumed.
func liveEligibilitySource(root string) eligibilitySource {
	return func(c claimRecord) (loopengine.EligibilityVerdict, error) {
		readers := loopengine.EligibilityReaders{
			Claim:    liveShowClaimReader(root, c.Repo),
			BoardRow: liveBoardRowReader(root),
			PR:       livePRReader(),
		}
		return loopengine.Eligibility(toLoopClaim(c, root), readers)
	}
}

// liveBoardRowReader reads the item's Status cell on origin/main. It resolves the stream board
// README under root as docs/streams/<stream>/README.md, where <stream> is the item's path minus
// its trailing /NN.
func liveBoardRowReader(root string) func(rootArg, item string) (loopengine.Status, error) {
	return func(rootArg, item string) (loopengine.Status, error) {
		stream := item
		if i := strings.LastIndex(item, "/"); i >= 0 {
			stream = item[:i]
		}
		boardPath := filepath.ToSlash(filepath.Join("docs", "streams", stream, "README.md"))
		repo, err := gitcore.Open(root)
		if err != nil {
			return "", fmt.Errorf("open %s: %w", root, err)
		}
		content, err := repo.FileAt("refs/remotes/origin/main", boardPath)
		if err != nil {
			return "", fmt.Errorf("read %s at origin/main: %w", boardPath, err)
		}
		return loopengine.ParseBriefRowStatus(content, item)
	}
}

// livePRReader reads a PR's reconcile-relevant state through the forge. A PR number of 0 (a
// claim dispatched before its PR opened) is not a forge question — there is nothing merged or
// closed to find — so it reads as an open PR with no held signal (eligible on the PR axis); the
// claim and board reads still gate it.
func livePRReader() func(repo string, n int) (loopengine.PRState, error) {
	return func(repoStr string, n int) (loopengine.PRState, error) {
		if n <= 0 {
			return loopengine.PRState{Kind: deskkit.PROpen}, nil
		}
		owner, name, ok := strings.Cut(repoStr, "/")
		if !ok || owner == "" || name == "" {
			return loopengine.PRState{}, fmt.Errorf("repo %q is not owner/name", repoStr)
		}
		role, _, rerr := deskkit.SessionTokenRole("desksupervise")
		if rerr != nil {
			return loopengine.PRState{}, fmt.Errorf("cannot resolve a token role: %w", rerr)
		}
		repo := deskkit.ForgeRepo{Owner: owner, Name: name}
		forge, ferr := deskkit.ForgeFor(repo, role)
		if ferr != nil {
			return loopengine.PRState{}, fmt.Errorf("cannot resolve a forge for %s: %w", repoStr, ferr)
		}
		pr, perr := forge.GetPullRequest(repo, n)
		if perr != nil {
			return loopengine.PRState{}, fmt.Errorf("GetPullRequest %s#%d: %w", repoStr, n, perr)
		}
		disp := deskkit.ReadDispositionIndex(pr.Labels, nil)
		var verdict deskkit.DispositionVerdict
		if disp.State == deskkit.DispositionCheckedFailed {
			verdict = disp.Record.Verdict
		}
		return loopengine.PRState{
			Kind:        deskkit.ParsePRKind(pr.State),
			Disposition: verdict,
			Labels:      pr.Labels,
		}, nil
	}
}

// liveShowClaimReader builds the claim reader the live source actually uses: it knows the repo
// (from readLiveClaims), so it can issue the repo-scoped `show`. It compares the current owner=
// against the dispatched holder.
func liveShowClaimReader(root, repo string) func(key string) (loopengine.ClaimRecord, error) {
	return func(key string) (loopengine.ClaimRecord, error) {
		script := filepath.Join(root, filepath.FromSlash(dispatchClaimScriptRel))
		out, serr := showClaim(script, key, repo)
		if serr != nil {
			return loopengine.ClaimRecord{}, fmt.Errorf("%s show %s: %s: %w", dispatchClaimScriptRel, key, strings.TrimSpace(out), serr)
		}
		return loopengine.ClaimRecord{Key: key, Holder: claimShowField(claimOwnerFieldRe, out)}, nil
	}
}
