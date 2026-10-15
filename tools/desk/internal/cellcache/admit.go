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
//     passing the same entry checks the walk applies to their top level;
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
		// margins below are for.
		if !st.ModTime().Before(rep.After.At) {
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
