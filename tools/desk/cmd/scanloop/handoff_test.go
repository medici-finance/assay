package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntakeHandoffDisposition(t *testing.T) {
	t.Run("production run retains quiet work", func(t *testing.T) {
		state := isolatedDeskHome(t)
		if err := os.WriteFile(filepath.Join(state, "roster.env"), []byte(fixtureRoster+"\nASSAY_REPO_FORGES=medici-finance/assay=github\n"), 0600); err != nil {
			t.Fatal(err)
		}
		deskkit.ReloadConfig()
		srv := httptest.NewServer((&scanForge{}).handler(t))
		defer srv.Close()
		oldBase := forgeAPIBase
		forgeAPIBase = srv.URL
		t.Cleanup(func() { forgeAPIBase = oldBase })
		stubIdentity(t, func(string) (string, string, error) { return "issue-loop", "intake-desk", nil }, func(string, string) (string, string, error) { return "x-test-token", "/cache/x-test-token", nil })
		dir := armedStateDir(t)
		if err := os.WriteFile(filepath.Join(dir, "medici-finance__assay.state"), []byte("medici-finance/assay#161 2026-08-24T11:58:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		script := inboundPoller(t)
		if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		err := cmdRun([]string{"--root", t.TempDir(), "--scan-target", "medici-finance/assay", "--worktree-base", t.TempDir(), "--state-dir", dir, "--monitor", script, "--now", "2026-08-24T12:00:00Z", "--dry-run"}, &out)
		if err != nil || !strings.Contains(out.String(), "EMITTED") || !strings.Contains(out.String(), "#161") {
			t.Fatalf("production quiet work lost: err=%v output=%s", err, &out)
		}
	})

	t.Run("accountable dispositions and partial-write retry", func(t *testing.T) {
		stateDir := isolatedDeskHome(t)
		root := t.TempDir()
		oldRoot := handoffRoot
		handoffRoot = func() (string, error) { return root, nil }
		t.Cleanup(func() { handoffRoot = oldRoot })
		briefDir := filepath.Join(root, "docs", "streams", "repair")
		if err := os.MkdirAll(briefDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(briefDir, "brief-01-author.md"), []byte("---\nbrief: repair/01\nexec-tier: strong\n---\nAuthor the scoped work.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		companion := filepath.Join(t.TempDir(), "handoff.json")
		h := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "medici-finance/assay#77", Opened: "2026-08-24T11:50:00Z", Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: "repair", Successor: "repair/01", Evidence: "medici-finance/assay#77"}
		land := func(h issueflow.IntakeHandoff) error {
			raw, err := json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(companion, raw, 0600); err != nil {
				t.Fatal(err)
			}
			return cmdLand([]string{"--item", h.Item, "--opened", h.Opened, "--exit", "bug", "--artifact", "repair/01", "--tier", "strong", "--handoff", companion, "--handoff-reader", issueflow.IntakeHandoffSchema}, "intake-desk", &strings.Builder{})
		}
		// Simulate failure of the second write after v1 lands. The exact retry
		// must populate the companion without writing another v1 exit.
		if err := os.Mkdir(filepath.Join(stateDir, handoffRecordsFile), 0700); err != nil {
			t.Fatal(err)
		}
		if err := land(h); err == nil {
			t.Fatal("companion failure was swallowed")
		}
		before := recordFileBytes(t, stateDir)
		if before == "" {
			t.Fatal("did not exercise partial success")
		}
		if err := os.Remove(filepath.Join(stateDir, handoffRecordsFile)); err != nil {
			t.Fatal(err)
		}
		if err := land(h); err != nil {
			t.Fatal(err)
		}
		if err := land(h); err != nil {
			t.Fatal(err)
		}
		records, err := readHandoffRecords(filepath.Join(stateDir, handoffRecordsFile))
		if err != nil || len(records) != 1 || recordFileBytes(t, stateDir) != before {
			t.Fatalf("retry: %v %v", records, err)
		}
		// A retry naming a DIFFERENT exit keeps the immutable first exit and says so.
		raw, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(companion, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := cmdLand([]string{"--item", h.Item, "--opened", h.Opened, "--exit", "finding", "--artifact", "repair/01", "--tier", "strong", "--handoff", companion, "--handoff-reader", issueflow.IntakeHandoffSchema}, "intake-desk", &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "already left by bug") || !strings.Contains(out.String(), "--exit finding was NOT recorded") || recordFileBytes(t, stateDir) != before {
			t.Fatalf("differing exit silently ignored: %q", out.String())
		}
		for _, tc := range []struct {
			name string
			edit func(*issueflow.IntakeHandoff)
			pass bool
		}{
			{"scoping requires strong author", func(h *issueflow.IntakeHandoff) {
				h.Disposition = issueflow.NeedsScoping
				h.Route = "authoring"
				h.AuthorTier = "any"
			}, false},
			{"linked strong author", func(h *issueflow.IntakeHandoff) {
				h.Disposition = issueflow.NeedsScoping
				h.Route = "authoring"
				h.AuthorTier = "strong"
			}, true},
			{"missing successor file", func(h *issueflow.IntakeHandoff) { h.Successor = "repair/99" }, false},
			{"no successor", func(h *issueflow.IntakeHandoff) { h.Successor = "" }, false},
			{"delivered source is not implementation", func(h *issueflow.IntakeHandoff) { h.Delivered = true }, false},
			{"delivered source qualification", func(h *issueflow.IntakeHandoff) { h.Delivered = true; h.Route = "qualification" }, true},
			{"external wake missing", func(h *issueflow.IntakeHandoff) {
				h.Disposition = issueflow.External
				h.Route = "external"
				h.Successor = ""
				h.ExternalRole = "operator"
				h.RequiredAction = "install approved source"
			}, false},
			{"external explicit obligation", func(h *issueflow.IntakeHandoff) {
				h.Disposition = issueflow.External
				h.Route = "external"
				h.Successor = ""
				h.ExternalRole = "operator"
				h.RequiredAction = "install approved source"
				h.WakeCondition = "installation evidence recorded"
			}, true},
			{"role spoofing", func(h *issueflow.IntakeHandoff) { h.DecidingRole = "verify-desk" }, false},
			{"source event changed", func(h *issueflow.IntakeHandoff) { h.Opened = "" }, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				next := h
				next.Revision++
				tc.edit(&next)
				err := land(next)
				if (err == nil) != tc.pass {
					t.Fatalf("pass=%v err=%v", tc.pass, err)
				}
				if tc.pass {
					h.Revision++
				}
			})
		}
		if recordFileBytes(t, stateDir) != before {
			t.Fatal("companion revisions rewrote immutable v1")
		}
	})

	t.Run("restart retains quiet and closed observations", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		scope := []string{"medici-finance/assay"}
		baseline := filepath.Join(dir, "medici-finance__assay.state")
		if err := os.WriteFile(baseline, []byte("medici-finance/assay#77 2026-08-24T11:50:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		poll := func() (*MonitorReport, error) {
			// Issue closure removes it from the next complete open-issue listing.
			if err := os.WriteFile(baseline, nil, 0600); err != nil {
				return nil, err
			}
			return &MonitorReport{}, nil
		}
		for attempt := 0; attempt < 2; attempt++ {
			loop := passLoop(t, "")
			loop.Monitor = retainingMonitor(dir, scope, poll)
			items, err := loop.SelectQueue()
			if err != nil || len(items) != 1 {
				t.Fatalf("restart %d: items=%v err=%v", attempt, items, err)
			}
			if len(loop.Queued()) != 1 || loop.Queued()[0] != "medici-finance/assay#77" {
				t.Fatal(loop.Queued())
			}
			lane, _ := loop.classify(loop.Admissions()[0])
			if lane != LaneRouting {
				t.Fatalf("replay created another scan carrier: %s", lane)
			}
		}
	})
	t.Run("unread observation refuses before cursor advances", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, observationFile), []byte("broken"), 0600); err != nil {
			t.Fatal(err)
		}
		called := false
		monitor := retainingMonitor(dir, []string{"medici-finance/assay"}, func() (*MonitorReport, error) { called = true; return &MonitorReport{}, nil })
		if _, err := monitor(); err == nil || called {
			t.Fatalf("err=%v polled=%v", err, called)
		}
	})
	t.Run("settled judgment discharges replay on the next pass", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		scope := []string{"medici-finance/assay"}
		baseline := filepath.Join(dir, "medici-finance__assay.state")
		if err := os.WriteFile(baseline, []byte("medici-finance/assay#77 2026-08-24T11:50:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		settled := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "medici-finance/assay#77", Opened: "2026-08-24T11:50:00Z", Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: "repair", Successor: "#77", Evidence: "medici-finance/assay#77"}
		if err := appendHandoffRecord(settled); err != nil {
			t.Fatal(err)
		}
		pollOut := ""
		poll := func() (*MonitorReport, error) { return ParseMonitorOutput(pollOut), nil }
		// Pass 1 reports a new, unjudged issue; it is retained after the poll.
		pollOut = "INBOUND: medici-finance/assay#78 2026-08-24T12:00:00Z\n"
		if err := os.WriteFile(baseline, []byte("medici-finance/assay#77 2026-08-24T11:50:00Z\nmedici-finance/assay#78 2026-08-24T12:00:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		pass := func() []string {
			t.Helper()
			loop := passLoop(t, "")
			loop.Monitor = retainingMonitor(dir, scope, poll)
			if _, err := loop.SelectQueue(); err != nil {
				t.Fatal(err)
			}
			pollOut = ""
			return loop.Queued()
		}
		for n := 1; n <= 3; n++ {
			q := pass()
			if strings.Contains(strings.Join(q, ","), "#77") {
				t.Fatalf("pass %d: settled #77 queued again: %v", n, q)
			}
			if len(q) != 1 || q[0] != "medici-finance/assay#78" {
				t.Fatalf("pass %d: unjudged #78 must stay owed: %v", n, q)
			}
		}
		// Recording an accountable judgment for #78 discharges it as well.
		dup := settled
		dup.Item, dup.Opened, dup.Disposition, dup.Route, dup.Successor = "medici-finance/assay#78", "2026-08-24T12:00:00Z", issueflow.Duplicate, "duplicate", "#77"
		if err := appendHandoffRecord(dup); err != nil {
			t.Fatal(err)
		}
		if q := pass(); len(q) != 0 {
			t.Fatalf("settled judgments still queued: %v", q)
		}
		// A newer source event for a judged issue is a new key: it is owed again.
		pollOut = "INBOUND: medici-finance/assay#77 2026-08-24T13:00:00Z\n"
		if q := pass(); len(q) != 1 || q[0] != "medici-finance/assay#77" {
			t.Fatalf("newer event on a judged issue lost: %v", q)
		}
	})

	t.Run("replay is bounded per pass and defers the rest", func(t *testing.T) {
		isolatedDeskHome(t)
		t.Setenv("INBOUND_MONITOR_BURST_CAP", "3")
		dir := t.TempDir()
		var lines strings.Builder
		for n := 1; n <= 5; n++ {
			fmt.Fprintf(&lines, "medici-finance/assay#%d 2026-08-24T11:5%d:00Z\n", 100+n, n)
		}
		if err := os.WriteFile(filepath.Join(dir, "medici-finance__assay.state"), []byte(lines.String()), 0600); err != nil {
			t.Fatal(err)
		}
		loop := passLoop(t, "")
		loop.Monitor = retainingMonitor(dir, []string{"medici-finance/assay"}, func() (*MonitorReport, error) { return &MonitorReport{}, nil })
		if _, err := loop.SelectQueue(); err != nil {
			t.Fatal(err)
		}
		if q := loop.Queued(); len(q) != 3 || q[0] != "medici-finance/assay#101" {
			t.Fatalf("replay not bounded oldest-first: %v", q)
		}
		if rep := loop.Report(); rep == nil || len(rep.Deferred) != 1 || !strings.HasPrefix(rep.Deferred[0], "medici-finance/assay 2 ") {
			t.Fatalf("deferred replay not reported: %+v", rep)
		}
	})

	// regression: an accountable rejection (a judgment-decided rejected-watching exit)
	// had no way out of replay, so with the oldest-first replay cap such items took
	// every slot on every pass and starved newer owed events.
	t.Run("accountable rejection discharges replay and cannot starve newer owed events", func(t *testing.T) {
		isolatedDeskHome(t)
		t.Setenv("INBOUND_MONITOR_BURST_CAP", "2")
		dir := t.TempDir()
		scope := []string{"medici-finance/assay"}
		baseline := filepath.Join(dir, "medici-finance__assay.state")
		if err := os.WriteFile(baseline, []byte("medici-finance/assay#80 2026-08-24T11:50:00Z\nmedici-finance/assay#81 2026-08-24T11:51:00Z\nmedici-finance/assay#82 2026-08-24T11:52:00Z\nmedici-finance/assay#83 2026-08-24T11:53:00Z\nmedici-finance/assay#84 2026-08-24T11:54:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		rejected := judgmentFixture("medici-finance/assay#80", "2026-08-24T11:50:00Z", ExitRejectedWatching)
		rejected.Detail = "rejected"
		plantRecord(t, rejected)
		watched := judgmentFixture("medici-finance/assay#81", "2026-08-24T11:51:00Z", ExitRejectedWatching)
		watched.Detail = "watching"
		plantRecord(t, watched)
		// A MECHANICAL exit never discharges: only an accountable judgment does.
		mech := judgmentFixture("medici-finance/assay#82", "2026-08-24T11:52:00Z", ExitRejectedWatching)
		mech.Detail, mech.DecidedBy, mech.TriagerTier = "rejected", decidedMechanical, tierNone
		plantRecord(t, mech)
		// A judgment on a DIFFERENT event of the same issue never discharges this one.
		other := judgmentFixture("medici-finance/assay#83", "2026-08-24T11:30:00Z", ExitRejectedWatching)
		other.Detail = "rejected"
		plantRecord(t, other)

		pollOut := ""
		poll := func() (*MonitorReport, error) { return ParseMonitorOutput(pollOut), nil }
		pass := func() (*ScanLoop, []string) {
			t.Helper()
			loop := passLoop(t, "")
			loop.Monitor = retainingMonitor(dir, scope, poll)
			if _, err := loop.SelectQueue(); err != nil {
				t.Fatal(err)
			}
			pollOut = ""
			return loop, loop.Queued()
		}
		for n := 1; n <= 3; n++ {
			loop, q := pass()
			if strings.Join(q, ",") != "medici-finance/assay#82,medici-finance/assay#83" {
				t.Fatalf("pass %d: rejected/watched must leave replay and the owed events must be presented: %v", n, q)
			}
			// #84 is behind the cap of two, owed, and named as deferred rather than lost.
			if rep := loop.Report(); rep == nil || len(rep.Deferred) != 1 || !strings.HasPrefix(rep.Deferred[0], "medici-finance/assay 1 ") {
				t.Fatalf("pass %d: deferred line wrong: %+v", n, rep)
			}
		}
		// A newer source event on a rejected issue is a new key: it is owed again.
		pollOut = "INBOUND: medici-finance/assay#80 2026-08-24T13:00:00Z\n"
		if _, q := pass(); !strings.Contains(strings.Join(q, ","), "medici-finance/assay#80") {
			t.Fatalf("newer event on a rejected issue lost: %v", q)
		}
	})

	t.Run("retained unreadable or untrusted observation retires", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		scope := []string{"medici-finance/assay"}
		baseline := filepath.Join(dir, "medici-finance__assay.state")
		if err := os.WriteFile(baseline, []byte("medici-finance/assay#77 2026-08-24T11:50:00Z\nmedici-finance/assay#78 2026-08-24T11:51:00Z\nmedici-finance/assay#79 2026-08-24T11:52:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		pollOut := ""
		poll := func() (*MonitorReport, error) {
			// Deleted or closed issues leave the complete open-issue listing.
			if err := os.WriteFile(baseline, nil, 0600); err != nil {
				return nil, err
			}
			return ParseMonitorOutput(pollOut), nil
		}
		calls := map[int]int{}
		probe := func(_ string, n int) (string, time.Time, []deskkit.ContentEvent, bool, error) {
			calls[n]++
			switch n {
			case 77: // deleted: the forge answers a definite not-found
				return "", time.Time{}, nil, false, fmt.Errorf("cannot read the author: %w", &deskkit.ForgeAPIError{Status: http.StatusNotFound, Method: "GET", Path: "/repos/x/issues/77"})
			case 78: // untrusted opener, read cleanly, unblessed
				return "mallory", time.Time{}, nil, true, nil
			default: // transient: never retired, keeps failing closed
				return "", time.Time{}, nil, false, fmt.Errorf("cannot read the author: %w", &deskkit.ForgeAPIError{Status: http.StatusBadGateway, Method: "GET", Path: "/repos/x/issues/79"})
			}
		}
		pass := func() *ScanLoop {
			t.Helper()
			loop := passLoop(t, "")
			loop.Probe = probe
			loop.Monitor = retainingMonitor(dir, scope, poll)
			if _, err := loop.SelectQueue(); err != nil {
				t.Fatal(err)
			}
			pollOut = ""
			return loop
		}
		first := pass()
		if blind := couldNotCheck(first.Admissions()); strings.Join(blind, ",") != "medici-finance/assay#77,medici-finance/assay#79" {
			t.Fatalf("discovery pass must stay fail-closed: %v", blind)
		}
		for n := 2; n <= 3; n++ {
			loop := pass()
			if blind := couldNotCheck(loop.Admissions()); strings.Join(blind, ",") != "medici-finance/assay#79" {
				t.Fatalf("pass %d: blind=%v (gone item must not blind every pass; transient must)", n, blind)
			}
			for _, a := range loop.Admissions() {
				if a.Item.Number == 78 || a.Item.Number == 77 {
					t.Fatalf("pass %d: retired item replayed: %+v", n, a)
				}
			}
		}
		if calls[77] != 1 || calls[78] != 1 || calls[79] != 3 {
			t.Fatalf("retired items still re-probed every pass: %v", calls)
		}
		obs, err := readObservations(dir)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, e := range obs.Events {
			got[e.Item] = e.Retired
		}
		if got["medici-finance/assay#77"] != "gone" || got["medici-finance/assay#78"] != "quarantined" || got["medici-finance/assay#79"] != "" {
			t.Fatalf("terminal dispositions not recorded: %v", got)
		}
		// A newer event (say, a blessing comment) is a new key and is read again.
		pollOut = "INBOUND: medici-finance/assay#78 2026-08-24T13:00:00Z\n"
		if loop := pass(); calls[78] != 2 || len(loop.Admissions()) != 2 {
			t.Fatalf("newer event on a retired issue lost: calls=%v admissions=%v", calls, loop.Admissions())
		}
	})

	t.Run("degraded repo never retires a not-found item", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "medici-finance__assay.state"), []byte("medici-finance/assay#77 2026-08-24T11:50:00Z\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= 2; n++ {
			loop := passLoop(t, "")
			loop.Probe = probeReturning("", time.Time{}, nil, false, &deskkit.ForgeAPIError{Status: http.StatusNotFound})
			loop.Monitor = retainingMonitor(dir, []string{"medici-finance/assay"}, func() (*MonitorReport, error) {
				return ParseMonitorOutput("MONITOR-DEGRADED: medici-finance/assay token mint FAILED\n"), nil
			})
			if _, err := loop.SelectQueue(); err != nil {
				t.Fatal(err)
			}
			if blind := couldNotCheck(loop.Admissions()); len(blind) != 1 {
				t.Fatalf("pass %d: a not-found under a degraded read was retired: %v", n, blind)
			}
		}
	})

	t.Run("replay still checks current trust", func(t *testing.T) {
		isolatedDeskHome(t)
		dir := t.TempDir()
		loop := passLoop(t, "")
		loop.Probe = nil
		loop.Monitor = retainingMonitor(dir, []string{"medici-finance/assay"}, func() (*MonitorReport, error) {
			return ParseMonitorOutput("INBOUND: medici-finance/assay#77 2026-08-24T11:50:00Z\n"), nil
		})
		items, err := loop.SelectQueue()
		if err != nil || len(items) != 0 || len(loop.Admissions()) != 1 {
			t.Fatalf("items=%v err=%v admissions=%v", items, err, loop.Admissions())
		}
	})
}
