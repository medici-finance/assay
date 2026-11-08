package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// capLedger plants audit rows into the throwaway HOME's ledger at explicit instants.
type capLedger struct {
	t    *testing.T
	path string
	next int
}

func newCapLedger(t *testing.T) *capLedger {
	t.Helper()
	rosterSetup(t)
	dir, err := deskkit.StateDir()
	if err != nil {
		t.Fatalf("state dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return &capLedger{t: t, path: filepath.Join(dir, "audit.jsonl"), next: 100}
}

func (l *capLedger) row(at time.Time, tool, verb, repo string, pr *int, result string) {
	l.t.Helper()
	e := deskkit.Entry{TS: at.UTC().Format(time.RFC3339), Tool: tool, Verb: verb, Repo: repo, PR: pr,
		Result: result, Detail: "planted", ArgsDigest: "0", SourceSHA: "dev", BuiltAt: "dev", SessionTag: "t"}
	b, err := json.Marshal(e)
	if err != nil {
		l.t.Fatal(err)
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		l.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		l.t.Fatal(err)
	}
}

// creates plants n successful `deskpr create` rows on repo at `at`, each with its own number.
func (l *capLedger) creates(at time.Time, repo string, n int) {
	for i := 0; i < n; i++ {
		l.next++
		pr := l.next
		l.row(at, "deskpr", "create", repo, &pr, deskkit.ResultOK)
	}
}

// evidence plants n unnumbered deskevidence writes on repo at `at`.
func (l *capLedger) evidence(at time.Time, repo string, n int) {
	for i := 0; i < n; i++ {
		l.row(at, "deskevidence", "land", repo, nil, deskkit.ResultOK)
	}
}

func capRun(t *testing.T, now time.Time, args ...string) (string, error) {
	t.Helper()
	old := capacityNow
	capacityNow = func() time.Time { return now }
	t.Cleanup(func() { capacityNow = old })
	var b bytes.Buffer
	err := runCapacity(&b, args)
	return b.String(), err
}

func mustContain(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func mustLack(t *testing.T, out string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(out, b) {
			t.Errorf("output carries %q:\n%s", b, out)
		}
	}
}

// regression: F-landing-capacity
func TestFlowCapacity(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	const alpha, beta = "acme/alpha", "acme/beta"

	t.Run("report and enforced meter agree", func(t *testing.T) {
		l := newCapLedger(t)
		l.creates(now.Add(-10*time.Minute), alpha, 17)
		c, err := deskkit.ReadLoopCapacity("worker-desk", alpha, 0, 0, now)
		if err != nil {
			t.Fatal(err)
		}
		if c.Snapshot.Remaining != 3 || c.Items != 3 || c.Blocked {
			t.Fatalf("remaining=%d items=%d blocked=%v, want 3/3/false", c.Snapshot.Remaining, c.Items, c.Blocked)
		}
		// Spend exactly the reported remainder: the gate admits every one of them and
		// refuses the next, and the report flips to exhausted at the same write.
		for i := 0; i < 3; i++ {
			if err := deskkit.AllowPRCreateAt(alpha, now); err != nil {
				t.Fatalf("write %d of the reported 3 refused by the gate: %v", i+1, err)
			}
			l.creates(now, alpha, 1)
		}
		if err := deskkit.AllowPRCreateAt(alpha, now); !deskkit.IsRateLimited(err) {
			t.Fatalf("gate admitted a write past the reported remainder: %v", err)
		}
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--in-flight", "0", "--depth", "5")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "meter new-PR", "20/20 used, 0 remaining exhausted", "BLOCKED until", "advice: no widening")
		mustLack(t, out, "may widen", "remaining writes: 0 (gate admits")
	})

	t.Run("reset window boundary", func(t *testing.T) {
		l := newCapLedger(t)
		old := now.Add(-59 * time.Minute)
		l.creates(old, alpha, 20)
		l.creates(now.Add(-61*time.Minute), alpha, 5) // outside the window: never counted
		snap, err := deskkit.ReadMeters(deskkit.MeterTarget{Gate: deskkit.GatePRCreate, Repo: alpha}, now)
		if err != nil {
			t.Fatal(err)
		}
		if snap.Admits || snap.Remaining != 0 {
			t.Fatalf("20 creates in the window must read exhausted, got admits=%v remaining=%d", snap.Admits, snap.Remaining)
		}
		free := snap.FreeAt
		if want := old.Add(time.Hour + time.Second); !free.Equal(want) {
			t.Fatalf("freeAt=%s, want %s", free, want)
		}
		for _, at := range []time.Time{free.Add(-time.Second), free} {
			s, err := deskkit.ReadMeters(deskkit.MeterTarget{Gate: deskkit.GatePRCreate, Repo: alpha}, at)
			if err != nil {
				t.Fatal(err)
			}
			gate := deskkit.AllowPRCreateAt(alpha, at)
			if s.Admits != (gate == nil) {
				t.Fatalf("at %s report admits=%v but gate=%v", at, s.Admits, gate)
			}
		}
	})

	t.Run("breaker open then half-open", func(t *testing.T) {
		l := newCapLedger(t)
		for i := 0; i < deskkit.BreakerTrip; i++ {
			l.row(now.Add(-time.Minute), "deskpr", "create", alpha, nil, deskkit.ResultRefused)
		}
		c, err := deskkit.ReadLoopCapacity("worker-desk", alpha, 0, 0, now)
		if err != nil {
			t.Fatal(err)
		}
		gate := deskkit.AllowPRCreateAt(alpha, now)
		if !c.Blocked || c.Items != 0 || !deskkit.IsRateLimited(gate) {
			t.Fatalf("open breaker: blocked=%v items=%d gate=%v", c.Blocked, c.Items, gate)
		}
		later := now.Add(deskkit.BreakerCooldown)
		c2, err := deskkit.ReadLoopCapacity("worker-desk", alpha, 0, 0, later)
		if err != nil {
			t.Fatal(err)
		}
		if gate := deskkit.AllowPRCreateAt(alpha, later); gate != nil || c2.Blocked {
			t.Fatalf("cooled breaker: report blocked=%v gate=%v", c2.Blocked, gate)
		}
		if c2.Snapshot.Remaining != 1 {
			t.Fatalf("half-open breaker admits ONE probe; report says %d remaining", c2.Snapshot.Remaining)
		}
	})

	t.Run("concurrent reservations and heterogeneous cost", func(t *testing.T) {
		l := newCapLedger(t)
		cap := deskkit.UnnumberedCapFor("deskevidence")
		l.evidence(now.Add(-5*time.Minute), alpha, cap-4) // 4 writes left = 2 verify items
		c, err := deskkit.ReadLoopCapacity("verify-desk", alpha, 0, 0, now)
		if err != nil {
			t.Fatal(err)
		}
		if c.WritesPerItem != 2 || c.Items != 2 {
			t.Fatalf("verify item costs 2 writes: items=%d per=%d", c.Items, c.WritesPerItem)
		}
		// Three items already in flight reserve 6 writes; only 4 remain. No negative and no
		// spare capacity.
		c, err = deskkit.ReadLoopCapacity("verify-desk", alpha, 3, 0, now)
		if err != nil {
			t.Fatal(err)
		}
		if c.Items != 0 {
			t.Fatalf("reserved past the remainder must leave 0 items, got %d", c.Items)
		}
		// The unnumbered meter is the one deskevidence's gate enforces.
		if err := deskkit.AllowWriteAt("deskevidence", alpha, 0, now); err != nil {
			t.Fatalf("4 writes left but gate refuses: %v", err)
		}
		out, err := capRun(t, now, "--role", "verify-desk", "--repo", alpha, "--in-flight", "3", "--depth", "4")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "3 item(s) x 2 = 6 write(s)", "further items that fit: 0", "proposed: 4 item(s) x 2 = 8 write(s) — does NOT fit")
		mustLack(t, out, "may widen")
	})

	t.Run("partial provider evidence remains unknown", func(t *testing.T) {
		newCapLedger(t)
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha)
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "provider/cross-host capacity: unknown", "scope: this host's audit ledger only", "as of 2026-11-06T12:00:00Z")

		l := newCapLedger(t)
		f, _ := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		f.WriteString("{not json\n")
		f.Close()
		out, err = capRun(t, now, "--role", "worker-desk", "--repo", alpha+","+beta, "--depth", "9")
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("an unreadable ledger must be could-not-check (exit 6), got %v", err)
		}
		mustContain(t, out, "COULD-NOT-CHECK")
		mustLack(t, out, "may widen", "remaining writes:", "further items that fit")
	})

	t.Run("held or human-only depth cannot widen", func(t *testing.T) {
		newCapLedger(t)
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--depth", "0", "--held", "9", "--continuation", "4")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "advice: no widening", "9 held on a human decision/action, 4 continuation", "reported only")
		mustLack(t, out, "may widen")

		// 8 in flight (assumed: the width) + 2 waiting = 10 lanes of work; held items add none.
		out, err = capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--depth", "2", "--held", "9")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "advice: may widen to 10 (bound by the actionable depth): deskroster set --role worker-desk --width 10")

		out, err = capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--depth", "30")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "advice: may widen to 12 (bound by the roster maximum)")
	})

	t.Run("landing capacity bounds the widen", func(t *testing.T) {
		l := newCapLedger(t)
		l.creates(now.Add(-time.Minute), alpha, 10) // 10 left; 8 reserved in flight → 2 more items
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--depth", "9")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "advice: may widen to 10 (bound by landing capacity on acme/alpha (2 more item(s) after 8 reserved))")
	})

	t.Run("planted second exhausted target", func(t *testing.T) {
		l := newCapLedger(t)
		l.creates(now.Add(-time.Minute), alpha, 2)
		l.creates(now.Add(-time.Minute), beta, deskkit.RateLimitPerPRPerHour)
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha+","+beta, "--depth", "12")
		if err != nil {
			t.Fatal(err)
		}
		if deskkit.AllowPRCreateAt(alpha, now) != nil || !deskkit.IsRateLimited(deskkit.AllowPRCreateAt(beta, now)) {
			t.Fatal("fixture: alpha must admit and beta refuse")
		}
		mustContain(t, out, "repo acme/beta: gate pr-create", "advice: no widening — bound by the landing write budget on acme/beta refuses now")
		mustLack(t, out, "may widen")
	})

	t.Run("argument checks", func(t *testing.T) {
		newCapLedger(t)
		out, err := capRun(t, now, "--role", "worker-desk", "--repo", alpha, "--in-flight", "-2")
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || out != "" {
			t.Fatalf("--in-flight below -1 must be refused before any reading, got %v / %q", err, out)
		}
		out, err = capRun(t, now, "--help")
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, out, "-in-flight", "-continuation")
	})
}
