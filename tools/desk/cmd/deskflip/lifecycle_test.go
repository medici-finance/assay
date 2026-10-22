package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestLifecyclePreflightDeliveryRequiresPositiveSuccess(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check []rollupEntry
		pass  bool
	}{
		{name: "absent"},
		{name: "pending", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "in_progress"}}},
		{name: "failed", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "failure"}}},
		{name: "skipped", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "skipped"}}},
		{name: "neutral", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "neutral"}}},
		{name: "current success", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "success"}}, pass: true},
		{name: "forge status success", check: []rollupEntry{{Context: condLifecyclePreflight, State: "success"}}, pass: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLifecyclePreflight("Brief: example/01\n", []fileInfo{{Path: "src/code.go"}}, tc.check, headSHA, nil)
			if (err == nil) != tc.pass {
				t.Fatalf("error=%v, pass=%v", err, tc.pass)
			}
		})
	}
}

func TestLifecyclePreflightExemptsOnlyNonDeliveries(t *testing.T) {
	for _, body := range []string{"Issue: #7\n", "Authors: example/01\n", ""} {
		if err := checkLifecyclePreflight(body, []fileInfo{{Path: "code.go"}}, nil, headSHA, nil); err != nil {
			t.Fatal(err)
		}
	}
	files := []fileInfo{{Path: "docs/streams/example/brief-01-new.md", Status: "added"}}
	if err := checkLifecyclePreflight("Brief: example/01\n", files, nil, headSHA, nil); err != nil {
		t.Fatal(err)
	}
	for _, delivery := range []string{"Brief: example/01\n", "Cross-repo-brief: sibling:example:01\n", "Brief: example/01\nIssue: #7\n", "Partial delivery.\n\nBrief: example/01\n"} {
		if err := checkLifecyclePreflight(delivery, []fileInfo{{Path: "code.go"}}, nil, headSHA, nil); err == nil {
			t.Fatalf("delivery declaration incorrectly exempted: %q", delivery)
		}
	}
	files[0].Status = "modified"
	if err := checkLifecyclePreflight("Brief: example/01\n", files, nil, headSHA, nil); err == nil {
		t.Fatal("editing an existing brief exempted as authoring")
	}
}

func TestDeliveryCannotFlipWithGreenApprovedButMissingLifecycleCheck(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	plantBriefRoot(t, privateCIRepo, "example-stream", "15", nonRisk)
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, condLifecyclePreflight) || len(s.mutated()) > 0 {
		t.Fatalf("out=%s mutated=%v", out, s.mutated())
	}
}

const nonRisk = "---\nstatus: todo\ngate: model\nrisk: {sensitive-data: no, destructive: no, privilege: no, broad-impact: no}\n---\n"

func TestDeliveryFlipsOnlyWithCurrentHeadLifecycleSuccess(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	plantBriefRoot(t, privateCIRepo, "example-stream", "15", nonRisk)
	s.rollup = append(s.rollup, rollupEntry{Name: condLifecyclePreflight, Status: "completed", Conclusion: "success"})
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	if !s.flipped() {
		t.Fatal("did not flip")
	}
	if !s.saw("GET", "/commits/"+headSHA+"/check-runs") {
		t.Fatal("check was not read at current head")
	}
}

func TestLifecyclePreflightBodyEditInvalidatesOldSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, started, edited string
		unreadable, pass      bool
	}{
		{name: "older result", started: "2026-01-01T00:00:00Z", edited: "2026-01-02T00:00:00Z"},
		{name: "current declaration", started: "2026-01-03T00:00:00Z", edited: "2026-01-02T00:00:00Z", pass: true},
		{name: "missing run time", edited: "2026-01-02T00:00:00Z"},
		{name: "malformed edit time", started: "2026-01-03T00:00:00Z", edited: "unknown"},
		{name: "unreadable edit time", unreadable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func() (string, error) {
				if tc.unreadable {
					return "", fmt.Errorf("offline fixture")
				}
				return tc.edited, nil
			}
			checks := []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "success", StartedAt: tc.started}}
			err := checkLifecyclePreflight("Brief: example/01\n", []fileInfo{{Path: "code.go"}}, checks, headSHA, read)
			if (err == nil) != tc.pass {
				t.Fatalf("error=%v, pass=%v", err, tc.pass)
			}
		})
	}
}

func TestDeliveryDeclarationIntroducedDuringGatingRefuses(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Issue: #7\n"
	s.body2 = "Brief: example-stream/15\n"
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, condLifecyclePreflight) || len(s.mutated()) > 0 {
		t.Fatalf("out=%s mutated=%v", out, s.mutated())
	}
}
