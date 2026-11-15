package deskkit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The verify list of #6510, one test per row. Every test runs under setup(t), so the lease and
// the audit log are temp-dir fixtures — the real HEARTBEAT and audit.jsonl are never touched.

// backdatedLease plants a HEARTBEAT whose mtime is `age` ago and returns its path and mtime.
func backdatedLease(t *testing.T, dir string, age time.Duration) (string, time.Time) {
	t.Helper()
	writeStaleHeartbeat(t, dir, age)
	path := filepath.Join(dir, "HEARTBEAT")
	return path, leaseMtime(t, path)
}

func leaseMtime(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.ModTime()
}

// lastAuditRow returns the final row of the fixture audit log.
func lastAuditRow(t *testing.T, dir string) Entry {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()
	var last Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		if err := json.Unmarshal(sc.Bytes(), &last); err != nil {
			t.Fatalf("decode audit row: %v", err)
		}
		n++
	}
	if n == 0 {
		t.Fatal("audit log has no rows")
	}
	return last
}

func actRow(result string) Entry {
	return Entry{Tool: "deskpost", Verb: "comment", Result: result, Detail: "posted comment"}
}

// regression: #6510
func TestRenewFreshOnOutwardAct(t *testing.T) {
	dir := setup(t)
	path, before := backdatedLease(t, dir, 2*time.Hour)
	if err := Guard(); err != nil {
		t.Fatalf("Guard on a fresh lease: %v", err)
	}
	if err := LogOutwardAct(actRow(ResultOK)); err != nil {
		t.Fatalf("LogOutwardAct: %v", err)
	}
	after := leaseMtime(t, path)
	if !after.After(before.Add(time.Hour)) {
		t.Fatalf("lease not renewed: mtime %v -> %v", before, after)
	}
	row := lastAuditRow(t, dir)
	if !strings.HasSuffix(row.Detail, " | heartbeat=renewed") {
		t.Fatalf("audit row does not name the renewal: Detail=%q", row.Detail)
	}
	if !strings.HasPrefix(row.Detail, "posted comment") {
		t.Fatalf("renewal note must be appended, not prepended: Detail=%q", row.Detail)
	}
}

// regression: #6510
func TestStaleLeaseNotRevived(t *testing.T) {
	dir := setup(t)
	path, before := backdatedLease(t, dir, 25*time.Hour)
	// The guard's refusal is unchanged: a stale lease still refuses.
	if err := Guard(); err == nil {
		t.Fatal("Guard passed on a stale lease")
	}
	if err := LogOutwardAct(actRow(ResultOK)); err != nil {
		t.Fatalf("LogOutwardAct: %v", err)
	}
	if after := leaseMtime(t, path); !after.Equal(before) {
		t.Fatalf("stale lease was touched: mtime %v -> %v", before, after)
	}
	if row := lastAuditRow(t, dir); !strings.HasSuffix(row.Detail, "heartbeat=stale-not-renewed") {
		t.Fatalf("audit row does not record the stale lease: Detail=%q", row.Detail)
	}
	if err := Guard(); err == nil {
		t.Fatal("Guard passed after an act on a stale lease — the lease was revived")
	}
}

// regression: #6510
func TestNonOKResultNoRenew(t *testing.T) {
	for _, res := range []string{ResultNoop, ResultDryRun, ResultRefused, ResultUnwritten,
		ResultUnverifiable, ResultDisabled, ResultRateLimited, ResultContended} {
		t.Run(res, func(t *testing.T) {
			dir := setup(t)
			path, before := backdatedLease(t, dir, 2*time.Hour)
			if err := LogOutwardAct(actRow(res)); err != nil {
				t.Fatalf("LogOutwardAct: %v", err)
			}
			if after := leaseMtime(t, path); !after.Equal(before) {
				t.Fatalf("%s act renewed the lease: mtime %v -> %v", res, before, after)
			}
			if row := lastAuditRow(t, dir); strings.Contains(row.Detail, "heartbeat=") {
				t.Fatalf("%s row carries a renewal note: Detail=%q", res, row.Detail)
			}
		})
	}
}

// regression: #6510
func TestReadOrMintNoRenew(t *testing.T) {
	dir := setup(t)
	path, before := backdatedLease(t, dir, 2*time.Hour)
	// Reads, mints, boots and ticks log ResultOK through plain Log: never a renewal.
	for _, e := range []Entry{
		{Tool: "deskboard", Verb: "board", Result: ResultOK},
		{Tool: "desktoken", Verb: "mint", Result: ResultOK},
		{Tool: "deskclaim", Verb: "progress", Result: ResultOK},
	} {
		if err := Log(e); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	if after := leaseMtime(t, path); !after.Equal(before) {
		t.Fatalf("a read/mint renewed the lease: mtime %v -> %v", before, after)
	}
}

// regression: #6510
func TestAbsentLeaseNotCreated(t *testing.T) {
	dir := setup(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := LogOutwardAct(actRow(ResultOK)); err != nil {
		t.Fatalf("LogOutwardAct: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "HEARTBEAT")); !os.IsNotExist(err) {
		t.Fatalf("HEARTBEAT was created by an act (stat err=%v)", err)
	}
	if row := lastAuditRow(t, dir); strings.Contains(row.Detail, "heartbeat=") {
		t.Fatalf("feature-off row carries a renewal note: Detail=%q", row.Detail)
	}
}

// regression: #6510
func TestStaleAfterGuardNoTouch(t *testing.T) {
	dir := setup(t)
	path, _ := backdatedLease(t, dir, 23*time.Hour)
	if err := Guard(); err != nil {
		t.Fatalf("Guard on a fresh lease: %v", err)
	}
	// The lease lapses between the guard check and the act's renewal.
	lapsed := time.Now().Add(-(HeartbeatStaleDuration + time.Minute))
	if err := os.Chtimes(path, lapsed, lapsed); err != nil {
		t.Fatal(err)
	}
	before := leaseMtime(t, path)
	if err := LogOutwardAct(actRow(ResultOK)); err != nil {
		t.Fatalf("LogOutwardAct: %v", err)
	}
	if after := leaseMtime(t, path); !after.Equal(before) {
		t.Fatalf("a lease that lapsed after Guard was revived: mtime %v -> %v", before, after)
	}
}

// regression: #6510
func TestLeaseFreshMatchesGuard(t *testing.T) {
	// The renewal side's freshness test agrees with the guard's own reading of the same file
	// on both sides of the 24h line: a lease heartbeatState calls stale is never fresh here.
	for _, age := range []time.Duration{time.Minute, HeartbeatStaleDuration - time.Minute,
		HeartbeatStaleDuration + time.Minute, 48 * time.Hour} {
		dir := setup(t)
		path, _ := backdatedLease(t, dir, age)
		armed, _, err := heartbeatState()
		if err != nil {
			t.Fatalf("heartbeatState: %v", err)
		}
		if fresh := leaseFresh(time.Since(leaseMtime(t, path))); fresh == armed {
			t.Fatalf("age %v: leaseFresh=%v but guard stale=%v", age, fresh, armed)
		}
	}
}
