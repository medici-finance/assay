package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
