package main

// cicheck_test.go — desk-supervision/31's flow proof: a real deskflip run, whose Forge comes
// from the production construction path (ResolveForge → OutboundChecked over the GitHub
// backend, pointed at the recording stub; HOME — and so the state dir — is a temp dir), leaves
// CI-check records for the head it evaluated, and its verdict is the verdict it reaches with
// recording disabled.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// flipOutcome is everything a flip run decides or does, minus timing.
type flipOutcome struct {
	rc       int
	flipped  bool
	mutated  []string
	requests []string
}

func runFlipForCI(t *testing.T, rollup []rollupEntry) flipOutcome {
	t.Helper()
	s := newStub()
	s.rollup = rollup
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	rc := run([]string{"7", "--repo", privateCIRepo})
	var reqs []string
	for _, r := range s.requests {
		reqs = append(reqs, r.Method+" "+r.Path+"?"+r.Query)
	}
	return flipOutcome{rc: rc, flipped: s.flipped(), mutated: s.mutated(), requests: reqs}
}

func TestFlip_LeavesCICheckRecords(t *testing.T) {
	cases := map[string]struct {
		rollup     []rollupEntry
		conclusion string
	}{
		"green head flips": {
			rollup: []rollupEntry{
				{ID: "5150", Name: "test", Status: "completed", Conclusion: "success",
					StartedAt: "2026-10-08T10:00:00Z", CompletedAt: "2026-10-08T10:02:00Z"},
				{Context: "external-scan", State: "success", CreatedAt: "2026-10-08T10:03:00Z"},
			},
			conclusion: "success",
		},
		"red head is refused": {
			rollup: []rollupEntry{
				{ID: "5151", Name: "test", Status: "completed", Conclusion: "failure",
					StartedAt: "2026-10-08T10:00:00Z", CompletedAt: "2026-10-08T10:02:00Z"},
				{Context: "external-scan", State: "success", CreatedAt: "2026-10-08T10:03:00Z"},
			},
			conclusion: "failure",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var on, off flipOutcome
			t.Run("recording", func(t *testing.T) {
				on = runFlipForCI(t, c.rollup)
				recs, err := deskkit.LoadCIChecks()
				if err != nil {
					t.Fatalf("LoadCIChecks: %v", err)
				}
				var run, status bool
				for _, r := range recs {
					if r.HeadSHA != headSHA || r.Repo != privateCIRepo {
						t.Errorf("a record for another head/repo: %+v", r)
						continue
					}
					switch r.Kind {
					case deskkit.CICheckKindRun:
						run = r.Name == "test" && r.Conclusion == c.conclusion && r.Attempt != ""
					case deskkit.CICheckKindStatus:
						status = r.Name == "external-scan" && r.Conclusion == "success"
					}
				}
				if !run || !status {
					t.Errorf("the flip run left no record of the check run (%v) or the status (%v) it "+
						"evaluated: %+v", run, status, recs)
				}
			})
			t.Run("not recording", func(t *testing.T) {
				defer deskkit.SetCICheckRecording(false)()
				off = runFlipForCI(t, c.rollup)
				home, _ := os.UserHomeDir()
				if _, err := os.Stat(filepath.Join(home, ".config", "assay", "ci-checks.jsonl")); !os.IsNotExist(err) {
					t.Errorf("recording disabled, yet a ci-checks file exists (stat err %v)", err)
				}
			})
			if on.rc != off.rc || on.flipped != off.flipped {
				t.Errorf("verdict with recording (rc %d, flipped %v) != without (rc %d, flipped %v)",
					on.rc, on.flipped, off.rc, off.flipped)
			}
			if !reflect.DeepEqual(on.mutated, off.mutated) {
				t.Errorf("mutations differ with recording:\n on: %v\noff: %v", on.mutated, off.mutated)
			}
			if !reflect.DeepEqual(on.requests, off.requests) {
				t.Errorf("forge requests differ with recording:\n on: %v\noff: %v", on.requests, off.requests)
			}
			if name == "green head flips" && !on.flipped {
				t.Errorf("the green case did not flip (rc %d) — the fixture no longer exercises a flip", on.rc)
			}
			if name == "red head is refused" && on.flipped {
				t.Errorf("the red case flipped — the fixture no longer exercises a refusal")
			}
		})
	}
}
