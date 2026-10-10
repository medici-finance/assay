package main

// cachepressure.go — prune's CELL-CACHE pressure pass (Step D).
//
// The defect this pass exists for: a cell's managed Go cache
// (tools/desk/internal/cellcache) is pressure-checked at launch admission and
// at dispatch time, but nothing checks it on a cadence. A long-lived cell
// whose launches are rare can sit over its byte budget — or the filesystem
// under its free-space floor — indefinitely, and crash-custody records
// accumulate without bound. This pass is the backstop: every prune sweep
// runs cellcache.Check over each marked cache root under the cells root, and
// reaps the root of a cell directory this same sweep has just removed.
//
// Enforcement is entirely the package's own semantics — reclaim only under
// byte-budget or free-space pressure, never while a consumer holds a lease,
// never a warm purge; this pass adds the cadence and nothing else.
//
// Discovery is by the MARKER, not by cell.env: a candidate is
// `<cell>/go-cache` for a direct child of the cells root (the same
// enumeration `cellctl ls` uses), and cellcache.LoadMarkedPolicy accepts it
// only when the ownership marker and the root's own recorded policy agree.
// The policy the sweep enforces is read back from the root's report — the
// one the cell's launches actually run under — never re-derived from a second
// source, and never invented: a marked root with no readable report is
// could-not-check, not "default budget". A custom CELL_GO_CACHE_ROOT outside
// the cells root is out of the sweep's sight by construction; `cellctl cache
// <cell> clean` remains its instrument.
//
// Orphan reaping is bound to the sweep's own removals: when a worktree the
// sweep just removed (either arm, deregistration verified) IS the canonical
// directory a discovered root's trust domain binds to, the sweep's own gates
// are the external proof cellcache.Reap asks for, and the root — if one
// survives outside the removed tree — is removed. The lease set is re-read
// under the root's lock at act time; a root with an active record is refused.
//
// Output discipline follows the desk noise-floor contract, matching the
// branch-GC pass: a sweep that found every cache within policy prints NOTHING
// for this pass; the summary and tick lines gain a cache segment only when
// the pass reclaimed, reaped, or found pressure it could not clear, and every
// could-not-check is a warning line. --dry-run runs the pressure checks in
// the package's own dry mode (no initialization, no report rewrite, no
// removal) and prints the would-act rows.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

// cacheResult is the cache pass's half of a sweep result. The counts stay
// apart for the same reason the branch-GC counts do: the summary line is how
// an operator tells pressure-that-was-cleared from pressure-that-persists,
// and one merged number cannot say which happened.
type cacheResult struct {
	ran            bool     // false when --no-cache or --branches-only skipped the pass
	checked        int      // marked roots pressure-checked this sweep
	reclaimed      int      // roots reclaimed under pressure
	reclaimedBytes int64    // logical bytes the reclamations reported
	deferred       int      // roots still over budget / under floor (active consumers, or a dry run)
	reaped         []string // orphaned roots removed with their reaped owner (root paths)
	plan           []string // the dry-run would-act rows, and nothing else
}

// notable reports whether the pass acted or found pressure — the condition
// under which the summary and tick lines carry a cache segment at all.
func (r cacheResult) notable() bool {
	return r.reclaimed > 0 || r.deferred > 0 || len(r.reaped) > 0
}

// summarySegment is the cache pass's counts, rendered WITHOUT a leading
// separator so the one-shot summary line (", "-joined) and the interval tick
// line (" "-joined) each attach it in their own grammar. The caller prints it
// only when notable() — silence when every cache is within policy is the
// contract.
func (r cacheResult) summarySegment() string {
	return fmt.Sprintf("cache-checked %d, cache-reclaimed %d (%d logical bytes), cache-deferred %d, cache-orphans-reaped %d",
		r.checked, r.reclaimed, r.reclaimedBytes, r.deferred, len(r.reaped))
}

// cellsRootForSweep resolves which cells root the pass sweeps: the
// --cells-root flag when given, then $CELLS_ROOT, then the same per-user
// data-directory default cellctl compiles. A cells root that does not exist
// is a machine with no cells — the pass checks zero roots and stays silent,
// exactly as `cellctl ls` treats it. "" means no resolution was possible and
// skips the pass.
func cellsRootForSweep(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("CELLS_ROOT"); v != "" {
		return v
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "assay", "cells")
}

// sweepCellCaches runs one cache-pressure pass: pressure-check every marked
// root under the cells root, and reap the root of any cell directory this
// sweep's worktree arms removed. reapedPaths are the RESOLVED paths the sweep
// removed and deregistered this pass (both arms) — under --dry-run they are
// the would-remove set, and the reap is planned, never executed.
func sweepCellCaches(cellsRoot string, dryRun bool, reapedPaths []string) (cacheResult, []string) {
	res := cacheResult{ran: true}
	var warns []string
	if cellsRoot == "" {
		res.ran = false
		return res, warns
	}

	// The orphan set, keyed by trust domain: the domain is a one-way hash of
	// the canonical cell directory, so "this root belongs to that reaped
	// directory" is a question only the same derivation can answer.
	orphans := map[string]string{}
	for _, rt := range reapedPaths {
		orphans[cellcache.DomainFor(rt)] = rt
	}

	entries, err := os.ReadDir(cellsRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			warns = append(warns, "could-not-check: cells root "+cellsRoot+" is unreadable — no cache root was pressure-checked: "+errText(err))
		}
		return res, warns
	}
	for _, en := range entries {
		st, serr := os.Stat(filepath.Join(cellsRoot, en.Name()))
		if serr != nil || !st.IsDir() {
			continue
		}
		// Canonicalize the cell directory first, exactly as cellcache.Resolve
		// does when it builds the default root, so the recorded policy's root
		// comparison is canonical-to-canonical.
		cell, cerr := filepath.EvalSymlinks(filepath.Join(cellsRoot, en.Name()))
		if cerr != nil {
			continue
		}
		p, lerr := cellcache.LoadMarkedPolicy(filepath.Join(cell, "go-cache"))
		if lerr != nil {
			warns = append(warns, "could-not-check: cache root of cell "+en.Name()+": "+errText(lerr))
			continue
		}
		if p == nil {
			continue
		}

		// Orphan reap. The sweep removed the cell's own directory this pass,
		// under its own gates; the lease re-check inside Reap is at act time,
		// not from this sweep's earlier view.
		if owner, orphan := orphans[p.Domain]; orphan {
			if dryRun {
				res.plan = append(res.plan, "  REAP cache root "+p.Root+" — owning directory "+owner+" is a would-remove of this sweep (the lease set is re-checked at act time)")
				continue
			}
			removed, rerr := cellcache.Reap(*p)
			if rerr != nil {
				warns = append(warns, "could-not-check: orphaned cache root "+p.Root+" (owner "+owner+" reaped this sweep): "+errText(rerr))
				continue
			}
			if removed {
				res.reaped = append(res.reaped, p.Root)
			}
			// Not removed and no error: the root lived inside the removed
			// directory and went with it — nothing left to do.
			continue
		}

		res.checked++
		rep, cerr := cellcache.Check(*p, dryRun)
		if cerr == nil {
			// checked-clean: within policy. Silent.
		} else if rep.Outcome == "storage-deferred" {
			// Pressure the pass could not clear: a live consumer holds the
			// cache (or, under --dry-run, the removal was suppressed). Either
			// way it belongs on the count line — pressure that persists is
			// exactly what this pass exists to surface.
			res.deferred++
			if dryRun {
				row := "  RECLAIM cache root " + p.Root + " — " + rep.Reason
				if len(rep.SkippedActive) > 0 {
					row = fmt.Sprintf("  HOLD cache root %s — under pressure but %d active consumer record(s): the lease set is the authority; nothing reclaims a live consumer",
						p.Root, len(rep.SkippedActive))
				}
				res.plan = append(res.plan, row)
			}
		} else {
			reason := rep.Reason
			if reason == "" {
				reason = errText(cerr)
			}
			warns = append(warns, "could-not-check: cache root of cell "+en.Name()+": "+reason)
		}
		if rep.Reclaimed > 0 {
			res.reclaimed++
			res.reclaimedBytes += rep.Reclaimed
		}
	}
	sort.Strings(res.reaped)
	return res, warns
}

// renderCacheDetail prints what the noise floor allows a sweep to say per
// root: every orphaned root it reaped (rare and destructive, so named, never
// counted into silence) and the dry-run plan. The per-root pressure detail a
// live sweep does not print is in each root's own report.json.
func renderCacheDetail(w io.Writer, res cacheResult) {
	for _, root := range res.reaped {
		fmt.Fprintf(w, "  reaped orphaned cache root %s — its owning directory was removed by this sweep and no consumer lease held\n", root)
	}
	for _, row := range res.plan {
		fmt.Fprintln(w, row)
	}
}
