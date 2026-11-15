package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Lease renewal — the writing half of the HEARTBEAT dead-man lease (#6510).
//
// heartbeatState (killswitch.go) is the READING half and is unchanged by this file: a lease
// older than HeartbeatStaleDuration is STOP-ALL in Guard, and only a human touch revives it.
// What this file adds is the rule the driver ruled on #6510: a cell that is PROGRESSING keeps
// its own lease fresh. "Progressing" means one thing here — a desk verb completed an outward
// mutating act the forge confirmed — and the renewal is made in the same call that writes that
// act's audit row, so the row says the lease was renewed.
//
// Four properties are load-bearing, and each is a test in heartbeatrenew_test.go:
//
//  1. Only a successful outward act renews. LogOutwardAct renews for a ResultOK row and for no
//     other result: a refusal, a failure, a noop, a dry run or a rate-limited attempt leaves the
//     lease untouched. Reads, mints, boots and ticks never call LogOutwardAct at all — they log
//     through plain Log — so liveness alone never renews. A loop that only polls still lapses.
//  2. A stale lease is never revived by a desk. renewHeartbeat re-reads the lease's age at
//     renewal time and touches it only while it is still fresh, by the SAME predicate the guard
//     refuses on. A lease that went stale between the guard check and the renewal stays stale,
//     and the next verb's Guard refuses as it always has.
//  3. An absent file stays feature-off. The renewal is os.Chtimes, which changes the times of an
//     existing file and never creates one; a missing HEARTBEAT is left missing.
//  4. Renewal can never refuse or fail the act. The act has already landed when this runs, so a
//     lease that cannot be read or touched is recorded on the row as could-not-check and the
//     verb's outcome is unchanged. Guard, not this file, is what enforces the lease.

// LeaseRenewal is the outcome of one renewal attempt, as the act's audit row records it.
type LeaseRenewal string

const (
	// LeaseOff — no HEARTBEAT file: the feature is off, nothing was touched, nothing recorded.
	LeaseOff LeaseRenewal = ""
	// LeaseRenewed — the lease was fresh and its mtime was set to now.
	LeaseRenewed LeaseRenewal = "renewed"
	// LeaseStaleKept — the lease was already stale at renewal time and was left as it was: a
	// lapsed lease is revived only by a human touch.
	LeaseStaleKept LeaseRenewal = "stale-not-renewed"
	// LeaseNotChecked — the lease could not be read or touched; it was not renewed.
	LeaseNotChecked LeaseRenewal = "could-not-check"
)

// heartbeatRenewNow is the clock the renewal stamps. A test hook in the dirOverride mould —
// production uses time.Now; it is deliberately NOT wired to any env var or flag.
var heartbeatRenewNow = time.Now

// leaseFresh is the renewal side's freshness test. It is the exact complement of the guard's
// refusal predicate in heartbeatState (`age > HeartbeatStaleDuration` refuses), so a lease the
// guard would refuse is never one this side renews. TestLeaseFreshMatchesGuard pins the two
// together at the boundary.
func leaseFresh(age time.Duration) bool {
	return !(age > HeartbeatStaleDuration)
}

// renewHeartbeat renews a FRESH HEARTBEAT lease and reports what it did. It never creates the
// file and never touches a stale one. See the file comment for the contract.
//
// The window between the age read and the touch is the time two syscalls take. A lease that
// crosses the 24h line inside that window is renewed by at most that much; a lease removed in
// it is not re-created (Chtimes on a missing path fails, and that is recorded as could-not-check).
func renewHeartbeat() LeaseRenewal {
	dir, err := deskDir()
	if err != nil {
		return LeaseNotChecked
	}
	path := filepath.Join(dir, "HEARTBEAT")
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LeaseOff
		}
		return LeaseNotChecked
	}
	if !leaseFresh(time.Since(fi.ModTime())) {
		return LeaseStaleKept
	}
	now := heartbeatRenewNow()
	if err := os.Chtimes(path, now, now); err != nil {
		return LeaseNotChecked
	}
	return LeaseRenewed
}

// LogOutwardAct writes the audit row of an OUTWARD MUTATING act — a branch push, a PR create or
// edit, a review or comment post, an issue file or attach, an Evidence landing, a ready flip —
// exactly as Log does, and, when the row records the act's success (Result == ResultOK), first
// renews a fresh HEARTBEAT lease and appends the outcome to the row's Detail as
// ` | heartbeat=<outcome>`.
//
// Call it ONLY from the one place a mutating verb writes its terminal row, and only for an act
// the forge has confirmed. A read, a mint, a boot, a tick, a claim step or a dry-run row goes
// through Log, never through this: that is what keeps liveness from renewing the lease. The
// note is APPENDED, never prepended, because some meters key on a Detail prefix (deskfile's
// create-sent marker).
func LogOutwardAct(e Entry) error {
	if e.Result == ResultOK {
		if r := renewHeartbeat(); r != LeaseOff {
			e.Detail = strings.TrimSpace(e.Detail + " | heartbeat=" + string(r))
		}
	}
	return Log(e)
}
