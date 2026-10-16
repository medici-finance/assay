package cellcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// AdmitMaxAge bounds how old the recorded measurement may be for the lock-free
// pre-check to lean on it. A locked pass (and every consumer enrollment)
// records a fresh report, so a cache in regular use refreshes itself; an older
// or future-dated estimate is not trusted and the full check runs.
const AdmitMaxAge = 5 * time.Minute

// AdmitMtimeSlack is how far before the recorded measurement a top-level
// build/ or mod/ modification time must lie for the pre-check to treat the
// directory as unchanged since that measurement. Filesystem timestamps and the
// clock that stamped the report are different clocks: Linux stamps files from
// a coarse kernel clock that trails time.Now by up to a scheduler tick, and
// some filesystems keep one or two second resolution. Without the margin, an
// entry added in the instants after a measurement can carry a timestamp that
// reads as BEFORE it, and an over-budget cache is admitted on a stale
// estimate (the lock-free path skipped the locked pass that recovers it). A
// directory touched inside the margin is unproven and takes the locked path;
// one that has been quiet longer is unaffected, so the cost is a single
// locked pass after a change.
const AdmitMtimeSlack = 2 * time.Second

// admitEnv holds the Admit seams. They are instance-local so a test can make
// the free-space read fail, move the clock, or observe whether the locked path
// ran, without touching a global or the host cache.
type admitEnv struct {
	free   func(string) (uint64, error)
	now    func() time.Time
	locked func(Policy, bool) (Report, error)
}

// Admit is Check for a caller that only needs to know whether the cache is
// within policy (deskdispatch's storage admission), not to measure it.
//
// Check takes the root's custody lock before it measures anything, and under
// that lock walks build/ and mod/ in full. Every desk dispatches through that
// one lock, so on a large cache a pass that ends checked-clean and changes
// nothing still serialises every dispatch for the length of the walk. The lock
// exists to exclude consumer enrollment while whole roots are removed; a pass
// that removes nothing does not need it.
//
// Admit therefore tests, without the lock and without a walk, whether
// pressure is possible at all. It admits only when ALL of these read cleanly:
//
//   - the root is the owned, marked root, with build/ and mod/ (when present)
//     passing the same entry checks the walk applies to their top level and
//     not modified at or after the recorded measurement (less AdmitMtimeSlack,
//     since file timestamps and the report's clock can disagree);
//   - the last recorded pass is a clean, non-dry, full-measurement report for
//     this root and trust domain, no older than AdmitMaxAge;
//   - its logical size is at most three quarters of the budget; and
//   - statfs shows free space at least the floor plus a quarter of the budget.
//
// Anything it cannot read, parse or prove is "not cheap-clean" and runs Check
// unchanged. Unreadable never means admitted. A cheap admission records no
// report and takes no lock; the next locked pass or enrollment refreshes the
// estimate. Report.Admission says which path produced the verdict.
//
// Like Check, this is a measurement, not a reservation against concurrent
// writers: the margins above are the headroom for growth since the estimate.
func Admit(p Policy, dry bool) (Report, error) {
	return admit(p, dry, admitEnv{free: diskFree, now: time.Now, locked: Check})
}

// AdmitRecorded is CheckRecorded for a caller that DISCOVERED the root (a
// sweep walking a cells root, deskwt prune's cache pass) and, like Admit, only
// needs to know whether the cache is within policy.
//
// CheckRecorded takes the root's custody lock and walks build/ and mod/ in
// full on every call, so a sweep that finds every cache clean still serialises
// against every launch and dispatch for the length of the walk. AdmitRecorded
// reads the root's recorded policy without the lock (LoadMarkedPolicy, the
// same advisory read the sweep already makes) and runs the Admit pre-check on
// it. It admits only on the conditions Admit documents; a root it cannot
// prove cheap-clean (unmarked or unreadable report, stale or dirty estimate,
// size or free space near the thresholds) runs CheckRecorded unchanged, which
// re-reads the policy under the lock. Unreadable never means admitted.
//
// The pre-check's budget is the recorded policy's, the one the locked path
// would enforce; a policy recorded between the two reads is picked up by the
// next pass, within AdmitMaxAge.
func AdmitRecorded(root string, dry bool) (Report, error) {
	return admitRecorded(root, dry, admitEnv{
		free: diskFree, now: time.Now,
		locked: func(_ Policy, d bool) (Report, error) { return CheckRecorded(root, d) },
	})
}

func admitRecorded(root string, dry bool, env admitEnv) (Report, error) {
	advisory, err := LoadMarkedPolicy(root)
	if err != nil || advisory == nil {
		if err == nil {
			err = fmt.Errorf("no marked cache root at %s", root)
		}
		rep, lerr := env.locked(Policy{Root: root}, dry)
		rep.Admission = "locked: policy not readable: " + err.Error()
		var d *Deferred
		if errors.As(lerr, &d) {
			d.Report.Admission = rep.Admission
		}
		return rep, lerr
	}
	return admit(*advisory, dry, env)
}

func admit(p Policy, dry bool, env admitEnv) (Report, error) {
	r, why := cheapClean(p, dry, env)
	if why == "" {
		return r, nil
	}
	rep, err := env.locked(p, dry)
	rep.Admission = "locked: " + why
	var d *Deferred
	if errors.As(err, &d) {
		d.Report.Admission = rep.Admission
	}
	return rep, err
}

// cheapClean returns a checked-clean report and "" when the lock-free test
// clears the cache, or a zero report and the reason it could not.
func cheapClean(p Policy, dry bool, env admitEnv) (Report, string) {
	no := func(format string, a ...any) (Report, string) {
		return Report{}, fmt.Sprintf(format, a...)
	}
	if err := p.Validate(); err != nil {
		return no("policy: %v", err)
	}
	r, err := os.OpenRoot(p.Root)
	if err != nil {
		return no("cache root not readable: %v", err)
	}
	defer r.Close()
	st, err := r.Stat(".")
	if err != nil {
		return no("cache root not readable: %v", err)
	}
	dev, _, err := identity(st)
	if err != nil {
		return no("cache root identity: %v", err)
	}
	owner, err := readMetaBounded(r, dev, "owner", 1024)
	if err != nil || string(owner) != schema+"\n"+p.Domain+"\n" {
		return no("cache ownership not proven")
	}
	b, err := readMetaBounded(r, dev, "report.json", 64<<10)
	if err != nil {
		return no("no readable recorded measurement: %v", err)
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil || rep.Schema != schema {
		return no("recorded measurement not parseable")
	}
	if rep.Policy.Domain != p.Domain || rep.Policy.Root != p.Root {
		return no("recorded measurement belongs to another root or domain")
	}
	if rep.DryRun || rep.Outcome != "checked-clean" || len(rep.Failures) > 0 {
		return no("last recorded pass was not a clean full measurement")
	}
	est := rep.After.LogicalBytes
	if est == nil || *est < 0 {
		return no("recorded measurement has no size")
	}
	now := env.now().UTC()
	age := now.Sub(rep.After.At)
	if rep.After.At.IsZero() || age < 0 || age > AdmitMaxAge {
		return no("recorded measurement is stale or future-dated")
	}
	for _, name := range []string{"build", "mod"} {
		st, err := r.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return no("%s not readable: %v", name, err)
		}
		if err = checkEntry(st, dev); err != nil || !st.IsDir() {
			return no("%s is not an owned directory", name)
		}
		// A top-level change since the recorded pass (a root recreated, an
		// entry added or removed directly in it) makes the estimate unproven.
		// Writes deeper down do not touch this mtime; those are what the
		// margins below are for. The comparison leaves AdmitMtimeSlack of
		// room because file timestamps and the report's clock can disagree.
		if !st.ModTime().Before(rep.After.At.Add(-AdmitMtimeSlack)) {
			return no("%s changed after the recorded measurement", name)
		}
	}
	if *est > p.Budget/4*3 {
		return no("recorded size %d is within a quarter of budget %d", *est, p.Budget)
	}
	free, err := env.free(p.Root)
	if err != nil {
		return no("available bytes: %v", err)
	}
	need := p.Floor + uint64(p.Budget/4)
	if need < p.Floor || free < need {
		return no("free space %d is within the margin of floor %d", free, p.Floor)
	}
	at := now
	size := *est
	return Report{
		Schema: schema, Policy: p, Filesystem: filesystemName(dev), Outcome: "checked-clean",
		DryRun: dry, Admission: "precheck",
		Before: Measurement{At: at, LogicalBytes: &size, AvailableBytes: &free},
		After:  Measurement{At: at, LogicalBytes: &size, AvailableBytes: &free},
	}, ""
}
