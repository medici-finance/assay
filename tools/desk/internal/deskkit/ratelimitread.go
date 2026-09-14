package deskkit

// ratelimitread.go — the BOUNDED read behind pointsFor (#1035).
//
// THE DEFECT. `pointsFor` loaded and converted the ENTIRE audit ledger before any meter
// looked at it — a second full parse on every outward write, inside the audit flock, on top
// of the one `Guard` already paid and the one the outward-write flow pays for idempotency.
//
// THE OBSERVATION. Every question the meters actually ask is bounded by construction:
// `chargedInWindow` discards everything older than `rateWindow` (one hour), and `breakerRun`
// walks newest-first and BREAKS at the first in-scope entry that is not non-progress, so it
// reads only as far back as the trailing consecutive-refusal run and never past `BreakerTrip`
// members. What was unbounded was the READER, not the questions.
//
// THE RULE. This reader walks the ledger backwards and stops only when the answer is FULLY
// DETERMINED — which is to say, when no entry older than the cursor could change any meter's
// verdict. Where determinacy cannot be established — a hard cap reached, a malformed line, an
// unparseable timestamp, an unreadable file — it returns ok=false and `pointsFor` falls back
// to `LoadEntries()`, which then produces the SAME slice, and the same refusal messages, the
// whole-file parse always produced. It never narrows a meter.
//
// WHY A TIME HORIZON ALONE WOULD HAVE BEEN WRONG, and this is the reason the stop rule is
// shaped the way it is: `BreakerTrip` consecutive refusals spread over a week, with the newest
// one a minute ago, is an OPEN breaker today. A reader that stopped at the one-hour budget
// window would have counted one refusal, found the run under the trip, and admitted the write.
// A bounded read whose bound is not the meter's own termination condition is a fail-OPEN.
//
// THE ONE ASSUMPTION IT MAKES, stated rather than buried: audit timestamps are
// non-decreasing in file order. They are written by `Log` from `time.Now()` on an append-only
// file, so this holds unless the host's clock steps backwards; `boundedSkewMargin` absorbs a
// small step, and a large one is a `deskaudit recover`-shaped problem rather than a silent
// one — the file's order is still what every reader, bounded or not, reports.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// boundedReadMaxLines / boundedReadMaxBytes are the FAIL-CLOSED cap, not a horizon: a
// bounded read that has not reached determinacy by here discards what it has and the caller
// re-reads the whole ledger. They are generous on purpose — reaching them means the ledger
// has no progress entry in 200k lines, which is a diagnosis, not a budget. They are vars
// only so a test can lower them to exercise the fallback; nothing wires them to an env var
// or a flag, for the same reason dirOverride is not wired to one.
var (
	boundedReadMaxLines = 200_000
	boundedReadMaxBytes = int64(200 << 20)
)

// boundedSkewMargin is how far PAST each cutoff the reader keeps going before it calls that
// cutoff settled, so a second-resolution timestamp or a small clock step cannot strand an
// in-window entry on the far side of the cursor.
const boundedSkewMargin = 15 * time.Minute

// breakerWalk is one scope's consecutive-non-progress walk, evaluated INCREMENTALLY as the
// reverse reader hands it entries newest-first. It mirrors breakerRun's rules exactly:
// ignored results are invisible (neither counted nor resetting), progress terminates the
// walk, and non-progress extends it up to the trip.
type breakerWalk struct {
	trip       int
	run        int
	terminated bool
	// needed is set when this scope has a non-progress, non-ignored member recent enough
	// that its breaker could still be OPEN. A scope whose newest such member is older than
	// BreakerCooldown is closed whatever its run length (breakerOpen returns nil once the
	// cooldown has elapsed), so its walk need not be followed to the end.
	needed bool
}

func (w *breakerWalk) observe(result string, recent bool) {
	if breakerIgnores(result) {
		return
	}
	if nonProgress(result) && recent {
		w.needed = true
	}
	if w.terminated {
		return
	}
	if !nonProgress(result) {
		w.terminated = true // progress resets the run — breakerRun's `break`
		return
	}
	w.run++
	if w.run >= w.trip {
		w.terminated = true // a longer run trips identically; nothing older can change it
	}
}

// boundedPointsFor is pointsFor's bounded arm. ok=false means "not determined" and is the
// caller's signal to re-read the whole ledger — never an answer.
func boundedPointsFor(want string, now time.Time) (mine []auditPoint, ok bool) {
	paths, err := segmentPaths()
	if err != nil {
		return nil, false
	}

	windowCutoff := now.Add(-rateWindow).Add(-boundedSkewMargin)
	coolCutoff := now.Add(-BreakerCooldown).Add(-boundedSkewMargin)

	walks := map[string]*breakerWalk{"*": {trip: BreakerBackstopTrip}}
	windowSettled := false
	lines, read := 0, int64(0)
	undetermined := false

	// done reports that no entry older than the cursor can change any meter's verdict:
	// every budget window is settled, and every breaker walk that could still be open has
	// terminated the way breakerRun would have terminated it.
	done := func() bool {
		if !windowSettled {
			return false
		}
		for _, w := range walks {
			if w.needed && !w.terminated {
				return false
			}
		}
		return true
	}

	for i := len(paths) - 1; i >= 0 && !undetermined && !done(); i-- {
		serr := scanBackwards(paths[i], func(line []byte) bool {
			lines++
			read += int64(len(line))
			if lines > boundedReadMaxLines || read > boundedReadMaxBytes {
				undetermined = true
				return false
			}
			var e Entry
			if json.Unmarshal(line, &e) != nil {
				// Do NOT diagnose here: fall back, and let LoadEntries raise the
				// refusal it has always raised, naming the file and the line number.
				undetermined = true
				return false
			}
			ts, perr := time.Parse(time.RFC3339, e.TS)
			if perr == nil && ts.Before(windowCutoff) {
				windowSettled = true
			}
			if CanonicalToolKeyOr(e.Tool) != want {
				return !done() // another tool's line moves the cursor and nothing else
			}
			if perr != nil {
				// An unparseable timestamp on THIS tool's line is fail-closed in
				// pointsFrom; fall back so that refusal is raised there, once.
				undetermined = true
				return false
			}
			p := auditPoint{ts: ts, result: e.Result, repo: e.Repo, pr: e.PR}
			mine = append(mine, p)

			recent := !ts.Before(coolCutoff)
			for _, key := range breakerScopeKeys(p) {
				w := walks[key]
				if w == nil {
					w = &breakerWalk{trip: BreakerTrip}
					walks[key] = w
				}
				w.observe(e.Result, recent)
			}
			walks["*"].observe(e.Result, recent)
			return !done()
		})
		if serr != nil && !os.IsNotExist(serr) {
			return nil, false // unreadable segment: LoadEntries will say why
		}
	}
	if undetermined {
		return nil, false
	}

	// Collected newest-first; the meters want the ledger's own append order.
	for l, r := 0, len(mine)-1; l < r; l, r = l+1, r-1 {
		mine[l], mine[r] = mine[r], mine[l]
	}
	return mine, true
}

// breakerScopeKeys names the per-target scopes an entry belongs to: its repo (the
// AllowWriteRepoWide walk, checkBreakerRepo) and its repo+PR bucket (the AllowWriteAt walk,
// checkBreaker). A nil PR and a zero PR are ONE bucket, exactly as checkPRBudget and
// checkBreaker treat them — two encodings of the same thing, written by different tools.
func breakerScopeKeys(p auditPoint) []string {
	pr := 0
	if p.pr != nil {
		pr = *p.pr
	}
	return []string{"r:" + p.repo, fmt.Sprintf("p:%s#%d", p.repo, pr)}
}

// pointsFrom reduces already-loaded entries to what the meters need. It is the whole-parse
// arm of pointsFor, kept as its own function so the bounded arm and the fallback cannot
// disagree about the reduction — only about how many lines they had to read to perform it.
func pointsFrom(entries []Entry, want, tool string) ([]auditPoint, error) {
	var mine []auditPoint
	for _, e := range entries {
		if CanonicalToolKeyOr(e.Tool) != want {
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil {
			return nil, Unverifiable(
				fmt.Sprintf("audit entry for %q has an unparseable ts %q — run `deskaudit recover` (quarantines the bad line and carries good entries forward; a plain move resets the budget + idempotency)", tool, e.TS),
				perr)
		}
		mine = append(mine, auditPoint{ts: ts, result: e.Result, repo: e.Repo, pr: e.PR})
	}
	return mine, nil
}

// sortPoints puts the meters' input in the ledger's append order. Stable so entries sharing
// a whole-second RFC3339 timestamp keep it — the breaker's "consecutive" walk depends on it.
func sortPoints(mine []auditPoint) {
	sort.SliceStable(mine, func(i, j int) bool { return mine[i].ts.Before(mine[j].ts) })
}
