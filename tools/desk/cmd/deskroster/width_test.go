package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readWidthFile reads the raw stored entry, so a test can assert on what actually landed on
// disk rather than on what the reader chose to report.
func readWidthFile(t *testing.T, home, loop string) *deskkit.WidthEntry {
	t.Helper()
	p := filepath.Join(home, ".config", "assay", "roster", "width", loop+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading %s: %v", p, err)
	}
	var e deskkit.WidthEntry
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("decoding %s: %v", p, err)
	}
	return &e
}

// TestWidth_RoundTrip is the basic contract: a width set through the write path is read back
// by the read path, keyed by LOOP and with no session id involved on the read side. That
// last part is the whole reason this is not a beacon field — the desk that sets a width and
// the window that reads it are different sessions.
func TestWidth_RoundTrip(t *testing.T) {
	home := rosterSetup(t)
	now := time.Now()

	canonical, err := setWidth("pr-review-desk", 8, "coordinator-session", now)
	if err != nil {
		t.Fatalf("setWidth: %v", err)
	}
	if canonical != "pr-review-desk" {
		t.Errorf("canonical = %q, want pr-review-desk", canonical)
	}

	e := readWidthFile(t, home, "pr-review-desk")
	if e.Width != 8 || e.Loop != "pr-review-desk" || e.SetBy != "coordinator-session" {
		t.Errorf("stored entry = %+v, want width 8 on pr-review-desk set by coordinator-session", e)
	}

	got, fresh, err := deskkit.LoadWidth("pr-review-desk", now)
	if err != nil || !fresh {
		t.Fatalf("loadWidth: fresh=%v err=%v", fresh, err)
	}
	if got.Width != 8 {
		t.Errorf("read back width %d, want 8", got.Width)
	}
}

// TestWidth_SetterRoleIsNotWrittenToItsOwnBeacon is the defect the file-per-loop design
// exists to prevent. Setting another window's width must leave the SETTER's beacon alone —
// a coordinator that renames itself to the desk it is steering has broken the roster.
func TestWidth_SetterRoleIsNotWrittenToItsOwnBeacon(t *testing.T) {
	home := rosterSetup(t)
	writeTestBeacon(t, home, Beacon{Session: "test-session", Role: "the-desk"})

	if err := cmdSet([]string{"--role", "pr-review-desk", "--width", "8"}); err != nil {
		t.Fatalf("set --width: %v", err)
	}

	b := readTestBeacon(t, home, "test-session")
	if b.Role != "the-desk" {
		t.Errorf("the setter's own beacon Role became %q — steering another desk's pool must not "+
			"relabel the session doing the steering", b.Role)
	}
}

// TestWidth_ExpiresToTheDefault: a width outlives neither its TTL nor the session that set
// it. Past the window the entry is treated as ABSENT, so the loop reads its shipped default
// again rather than honouring a number no live session is holding open.
func TestWidth_ExpiresToTheDefault(t *testing.T) {
	rosterSetup(t)
	set := time.Now().Add(-2 * deskkit.WidthTTL)
	if _, err := setWidth("worker-desk", 12, "dead-session", set); err != nil {
		t.Fatalf("setWidth: %v", err)
	}

	_, fresh, err := deskkit.LoadWidth("worker-desk", time.Now())
	if err != nil {
		t.Fatalf("loadWidth: %v", err)
	}
	if fresh {
		t.Errorf("a width stored %s ago is still reported as in force; it must decay to the default "+
			"so a coordinator that died cannot leave a pool permanently wide", 2*deskkit.WidthTTL)
	}

	// And just inside the window it IS honoured — otherwise this test would pass on a reader
	// that reported everything as expired.
	if _, err := setWidth("worker-desk", 12, "live-session", time.Now().Add(-deskkit.WidthTTL/2)); err != nil {
		t.Fatalf("setWidth: %v", err)
	}
	e, fresh, err := deskkit.LoadWidth("worker-desk", time.Now())
	if err != nil || !fresh {
		t.Fatalf("a width set half a TTL ago must be in force: fresh=%v err=%v", fresh, err)
	}
	if e.Width != 12 {
		t.Errorf("in-force width = %d, want 12", e.Width)
	}
}

// TestWidth_CorruptEntryIsUnverifiableNotAbsent: a malformed file must NOT read as "nobody
// set a width". Reporting corruption as absence would silently revert a width the desk is
// relying on while every instrument said the default was deliberate.
func TestWidth_CorruptEntryIsUnverifiableNotAbsent(t *testing.T) {
	home := rosterSetup(t)
	dir := filepath.Join(home, ".config", "assay", "roster", "width")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "worker-desk.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, fresh, err := deskkit.LoadWidth("worker-desk", time.Now())
	if err == nil {
		t.Fatal("a corrupt width entry must be an error, never a silent 'no width set'")
	}
	if fresh {
		t.Error("a corrupt entry must not be reported as fresh")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Errorf("corrupt entry must be Unverifiable (exit %d), got exit %d: %v",
			deskkit.ExitUnverifiable, deskkit.ExitCodeOf(err), err)
	}
}

// TestWidth_OverBudgetIsRefusedAndStoresNothing is the exit-5 half at the CLI boundary, plus
// the property that makes a refusal safe to act on: a refused width leaves the store
// untouched, so the operator knows the previous width is still in force without checking.
func TestWidth_OverBudgetIsRefusedAndStoresNothing(t *testing.T) {
	home := rosterSetup(t)
	if _, err := setWidth("pr-review-desk", 5, "desk", time.Now()); err != nil {
		t.Fatalf("seeding an admissible width: %v", err)
	}

	max, _, err := deskkit.MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth: %v", err)
	}
	over := strconv.Itoa(max + 1)

	err = cmdSet([]string{"--role", "pr-review-desk", "--width", over})
	if err == nil {
		t.Fatalf("set --width %s must be refused (max is %d)", over, max)
	}
	if !deskkit.IsRefused(err) {
		t.Errorf("over-budget width must be Refused (exit %d), got exit %d: %v",
			deskkit.ExitRefused, deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(max)) {
		t.Errorf("the refusal must name the accepted maximum %d; got: %s", max, err.Error())
	}
	if e := readWidthFile(t, home, "pr-review-desk"); e.Width != 5 {
		t.Errorf("a REFUSED width changed the store to %d; the previous width (5) must survive a refusal", e.Width)
	}
}

// TestWidth_SetRefusesWithoutARoleAndRefusesAWorkEntry pins the two shapes --width must not
// silently accept. A width with no loop is a number with no pool; a width mixed into a work
// entry is two unrelated writes riding one command, and the beacon half would then be the
// one carrying the role.
func TestWidth_SetRefusesWithoutARoleAndRefusesAWorkEntry(t *testing.T) {
	rosterSetup(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no role", []string{"--width", "4"}},
		{"mixed with a work entry", []string{"--role", "worker-desk", "--width", "4", "--repo", "tracker", "--pr", "1", "--what", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdSet(tc.args)
			if err == nil || !deskkit.IsRefused(err) {
				t.Errorf("cmdSet(%v) = %v, want a Refused", tc.args, err)
			}
		})
	}
}

// TestWidth_UnknownLoopIsRefused: --role here names a LOOP, and an unrecognised one must not
// quietly create a width file nothing will ever read.
func TestWidth_UnknownLoopIsRefused(t *testing.T) {
	home := rosterSetup(t)
	err := cmdSet([]string{"--role", "reviewer", "--width", "4"}) // an App role, not a loop name
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("an App role is not a loop name and must be refused; got %v", err)
	}
	if !strings.Contains(err.Error(), "pr-review-desk") {
		t.Errorf("the refusal must show the real loop set so the right spelling is discoverable; got: %s", err.Error())
	}
	if _, serr := os.Stat(filepath.Join(home, ".config", "assay", "roster", "width")); serr == nil {
		t.Error("a refused set created the width store; a refusal must write nothing at all")
	}
}

// TestWidth_ReadVerbUnsetReportsTheDefault: the ordinary case. `deskroster width --role X`
// with nothing stored must print the shipped default, not zero and not an error.
func TestWidth_ReadVerbUnsetReportsTheDefault(t *testing.T) {
	rosterSetup(t)
	if err := cmdWidth([]string{"--role", "worker-desk"}); err != nil {
		t.Fatalf("width --role worker-desk with nothing stored: %v", err)
	}
	// The value itself is asserted through the deskkit reader the verb uses, which is the
	// same single source; this case is about the verb not erroring on an empty store.
	got, err := deskkit.EffectiveWidth("worker-desk", 0, 0, false)
	if err != nil {
		t.Fatalf("EffectiveWidth: %v", err)
	}
	want, _ := deskkit.DefaultWidth("worker-desk")
	if got != want {
		t.Errorf("unset width = %d, want the default %d", got, want)
	}
}

// TestWidth_ReadVerbRequiresARole: guessing which loop's number to report is how a loop ends
// up running someone else's width.
func TestWidth_ReadVerbRequiresARole(t *testing.T) {
	rosterSetup(t)
	err := cmdWidth(nil)
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("width with no --role = %v, want a Refused", err)
	}
}

// TestWidth_RetiredLoopNameSharesOneEntry: a session still presenting the retired name must
// read and write the SAME width as one presenting the canonical name. Two files for one pool
// is the drift the loop-name equivalence class exists to prevent.
func TestWidth_RetiredLoopNameSharesOneEntry(t *testing.T) {
	home := rosterSetup(t)
	if _, err := setWidth("batch-fanout", 10, "desk", time.Now()); err != nil {
		t.Fatalf("setWidth via the retired name: %v", err)
	}
	e := readWidthFile(t, home, "worker-desk")
	if e.Width != 10 {
		t.Errorf("a width set under the retired name landed as %+v; it must canonicalise to worker-desk", e)
	}
	if _, serr := os.Stat(filepath.Join(home, ".config", "assay", "roster", "width", "batch-fanout.json")); serr == nil {
		t.Error("a second file was written under the retired name — one pool, one entry")
	}
}

// --- example-stream/05: `deskroster width --role L --reserve resume=N,rework=M` -------------

// TestParseReserve_ParsesPairsAndRejectsMalformed pins the flag-value grammar independently of
// the width bound (CheckReserve), so a malformed --reserve string is caught before any width is
// even resolved.
func TestParseReserve_ParsesPairsAndRejectsMalformed(t *testing.T) {
	got, err := parseReserve("resume=2,rework=1")
	if err != nil {
		t.Fatalf("parseReserve: %v", err)
	}
	if got["resume"] != 2 || got["rework"] != 1 {
		t.Errorf("parseReserve(\"resume=2,rework=1\") = %v, want resume:2,rework:1", got)
	}

	if got, err := parseReserve(""); err != nil || len(got) != 0 {
		t.Errorf("parseReserve(\"\") = %v, %v, want an empty map and no error", got, err)
	}

	for _, bad := range []string{"resume", "resume=", "resume=two", "resume:2"} {
		if _, err := parseReserve(bad); err == nil || !deskkit.IsRefused(err) {
			t.Errorf("parseReserve(%q) = %v, want a Refused", bad, err)
		}
	}
}

// TestSetReserve_RoundTrip proves the write half stores the reservation on the SAME width
// entry the width read already resolves — capturing the width in force (the shipped default,
// here) rather than leaving it unset.
func TestSetReserve_RoundTrip(t *testing.T) {
	home := rosterSetup(t)
	canonical, width, err := setReserve("worker-desk", map[string]int{"resume": 3, "rework": 1}, "coordinator", time.Now())
	if err != nil {
		t.Fatalf("setReserve: %v", err)
	}
	if canonical != "worker-desk" {
		t.Errorf("canonical = %q, want worker-desk", canonical)
	}
	wantWidth, _ := deskkit.DefaultWidth("worker-desk")
	if width != wantWidth {
		t.Errorf("setReserve captured width %d, want the current effective width %d", width, wantWidth)
	}

	e := readWidthFile(t, home, "worker-desk")
	if e.Reserve["resume"] != 3 || e.Reserve["rework"] != 1 {
		t.Errorf("stored reserve = %v, want resume:3,rework:1", e.Reserve)
	}
	if e.Width != wantWidth {
		t.Errorf("stored width = %d, want the captured effective width %d", e.Width, wantWidth)
	}

	got, source, rerr := deskkit.ResolvedReserve("worker-desk")
	if rerr != nil {
		t.Fatalf("ResolvedReserve: %v", rerr)
	}
	if got["resume"] != 3 || got["rework"] != 1 {
		t.Errorf("ResolvedReserve after set = %v (source=%q), want resume:3,rework:1", got, source)
	}
}

// TestSetReserve_OverBudgetIsRefusedAndStoresNothing is the exit-5 half of Verify row 4 at the
// CLI boundary: a reservation that would swallow the width is refused and leaves the store
// untouched, the same safety property TestWidth_OverBudgetIsRefusedAndStoresNothing pins for a
// refused width.
func TestSetReserve_OverBudgetIsRefusedAndStoresNothing(t *testing.T) {
	home := rosterSetup(t)
	width, _, err := deskkit.ResolvedWidth("worker-desk")
	if err != nil {
		t.Fatalf("ResolvedWidth: %v", err)
	}

	_, _, err = setReserve("worker-desk", map[string]int{"resume": width}, "coordinator", time.Now())
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("a reservation equal to the width must be Refused, got %v", err)
	}
	if _, serr := os.Stat(filepath.Join(home, ".config", "assay", "roster", "width", "worker-desk.json")); serr == nil {
		t.Error("a refused --reserve wrote the width store; a refusal must write nothing at all")
	}
}

// TestSetWidth_NarrowingBelowExistingReserveIsRefused pins the safety net in setWidth: narrowing
// a loop's width out from under an already-stored reservation (so the reservation would now
// swallow the new width) must be refused, leaving the wider width — and the reservation — in
// force, rather than silently landing a width/reserve pair CheckReserve would refuse together.
func TestSetWidth_NarrowingBelowExistingReserveIsRefused(t *testing.T) {
	home := rosterSetup(t)
	if _, _, err := setReserve("pr-review-desk", map[string]int{"resume": 4}, "coordinator", time.Now()); err != nil {
		t.Fatalf("seeding a reservation: %v", err)
	}
	e := readWidthFile(t, home, "pr-review-desk")
	seededWidth := e.Width

	_, err := setWidth("pr-review-desk", 4, "coordinator", time.Now()) // 4 == the reservation itself: swallows it
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("narrowing to a width the existing reservation would swallow must be Refused, got %v", err)
	}
	after := readWidthFile(t, home, "pr-review-desk")
	if after.Width != seededWidth {
		t.Errorf("a refused narrowing changed the stored width to %d; the previous width (%d) must survive", after.Width, seededWidth)
	}
}

// TestCmdWidth_PlainReadPrintsWidthAndReserve pins the new compound stdout line
// (`width=<n> reserve=<classes> (source=..., expires=...)`), replacing the bare integer this
// verb used to print alone — a behaviour change the brief calls for explicitly, so it is pinned
// here rather than left to `--verbose` alone.
func TestCmdWidth_PlainReadPrintsWidthAndReserve(t *testing.T) {
	rosterSetup(t)
	out := captureStdout(t, func() {
		if err := cmdWidth([]string{"--role", "worker-desk"}); err != nil {
			t.Fatalf("cmdWidth: %v", err)
		}
	})
	wantWidth, _ := deskkit.DefaultWidth("worker-desk")
	wantLine := fmt.Sprintf("width=%d reserve=resume:2,rework:0 (source=default, expires=n/a)", wantWidth)
	if !strings.Contains(out, wantLine) {
		t.Errorf("cmdWidth plain read = %q, want it to contain %q", out, wantLine)
	}
}

// TestCmdWidth_ReserveFlagWritesThenPlainReadReflectsIt exercises the verb end to end: setting
// via --reserve, then reading it back with the plain (no-flag) form.
func TestCmdWidth_ReserveFlagWritesThenPlainReadReflectsIt(t *testing.T) {
	rosterSetup(t)
	if err := cmdWidth([]string{"--role", "worker-desk", "--reserve", "resume=3,rework=1"}); err != nil {
		t.Fatalf("cmdWidth --reserve: %v", err)
	}
	out := captureStdout(t, func() {
		if err := cmdWidth([]string{"--role", "worker-desk"}); err != nil {
			t.Fatalf("cmdWidth: %v", err)
		}
	})
	if !strings.Contains(out, "reserve=resume:3,rework:1 (source=set") {
		t.Errorf("cmdWidth plain read after --reserve = %q, want it to show the SET reservation", out)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was written. Used
// only by the small set of tests that assert on cmdWidth's printed line rather than on the
// store it reads.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, rerr := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if rerr != nil {
			break
		}
	}
	return string(buf)
}
