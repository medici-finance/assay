package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lpAdopted / lpNotAdopted are the two server answers the adoption predicate reads: the
// base branch's required status checks with and without lifecycle-preflight.
func lpAdopted() ([]string, error)    { return []string{"build", condLifecyclePreflight}, nil }
func lpNotAdopted() ([]string, error) { return []string{"build"}, nil }

func lpRun(c rollupEntry) rollupEntry {
	c.Name = condLifecyclePreflight
	if c.AppSlug == "" {
		c.AppSlug = lifecycleProducerApp
	}
	return c
}

func TestLifecyclePreflightDeliveryRequiresPositiveSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		check      []rollupEntry
		pass       bool
	}{
		{name: "absent", want: "no successful"},
		{name: "pending", check: []rollupEntry{lpRun(rollupEntry{Status: "in_progress"})}, want: "not completed"},
		{name: "failed", check: []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "failure"})}, want: "not completed"},
		{name: "skipped", check: []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "skipped"})}, want: "not completed"},
		{name: "neutral", check: []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "neutral"})}, want: "not completed"},
		{name: "current success", check: []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "success"})}, pass: true},
		// S1: a commit status records no producer App, so it is never evidence.
		{name: "forge status success", check: []rollupEntry{{Context: condLifecyclePreflight, State: "success"}}, want: "commit status"},
		// S1: a check run from any other App, or one whose producer the forge did not
		// report, is not the lifecycle verdict.
		{name: "other app", check: []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "success", AppSlug: "some-bot"})}, want: `"some-bot"`},
		{name: "unreported app", check: []rollupEntry{{Name: condLifecyclePreflight, Status: "completed", Conclusion: "success"}}, want: `""`},
		{name: "later run from other app", check: []rollupEntry{
			lpRun(rollupEntry{Status: "completed", Conclusion: "success", CompletedAt: "2026-01-01T00:00:00Z"}),
			lpRun(rollupEntry{Status: "completed", Conclusion: "success", CompletedAt: "2026-01-02T00:00:00Z", AppSlug: "some-bot"}),
		}, want: `"some-bot"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adopted, err := checkLifecyclePreflight("Brief: example/01\n", []fileInfo{{Path: "src/code.go"}}, tc.check, headSHA, lpAdopted, nil)
			if (err == nil) != tc.pass || adopted != tc.pass {
				t.Fatalf("adopted=%v error=%v, pass=%v", adopted, err, tc.pass)
			}
			if err != nil && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// F8: every Brief:/Cross-repo-brief: declaration is a delivery; statusgen alone decides the
// authoring-only exemption, so deskflip no longer exempts an authoring-shaped diff.
func TestLifecyclePreflightExemptsOnlyNonDeliveries(t *testing.T) {
	for _, body := range []string{"Issue: #7\n", "Authors: example/01\n", ""} {
		if adopted, err := checkLifecyclePreflight(body, []fileInfo{{Path: "code.go"}}, nil, headSHA, lpAdopted, nil); err != nil || adopted {
			t.Fatalf("%q: adopted=%v err=%v", body, adopted, err)
		}
	}
	authoring := []fileInfo{{Path: "docs/streams/example/brief-01-new.md", Status: "added"}}
	if _, err := checkLifecyclePreflight("Brief: example/01\n", authoring, nil, headSHA, lpAdopted, nil); err == nil {
		t.Fatal("an authoring-shaped Brief: diff was exempted by deskflip; statusgen owns that exemption")
	}
	for _, delivery := range []string{"Brief: example/01\n", "Cross-repo-brief: sibling:example:01\n", "Brief: example/01\nIssue: #7\n", "Partial delivery.\n\nBrief: example/01\n"} {
		if _, err := checkLifecyclePreflight(delivery, []fileInfo{{Path: "code.go"}}, nil, headSHA, lpAdopted, nil); err == nil {
			t.Fatalf("delivery declaration incorrectly exempted: %q", delivery)
		}
	}
}

// F3: the condition binds only where the base branch requires lifecycle-preflight.
func TestLifecyclePreflightAdoptionPredicate(t *testing.T) {
	success := []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "success"})}
	for _, tc := range []struct {
		name     string
		required func() ([]string, error)
		checks   []rollupEntry
		adopted  bool
		refuse   bool
	}{
		{name: "not adopted, check absent: today's behaviour", required: lpNotAdopted},
		{name: "no required checks at all", required: func() ([]string, error) { return nil, nil }},
		{name: "adopted, check absent: refuse", required: lpAdopted, refuse: true},
		{name: "adopted, check succeeded", required: lpAdopted, checks: success, adopted: true},
		{name: "required set unreadable: refuse", required: func() ([]string, error) { return nil, fmt.Errorf("offline fixture") }, checks: success, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adopted, err := checkLifecyclePreflight("Brief: example/01\n", []fileInfo{{Path: "code.go"}}, tc.checks, headSHA, tc.required, nil)
			if (err != nil) != tc.refuse || adopted != tc.adopted {
				t.Fatalf("adopted=%v err=%v, want adopted=%v refuse=%v", adopted, err, tc.adopted, tc.refuse)
			}
		})
	}
}

// S1: a PR that edits a workflow or action definition can run its own job named
// lifecycle-preflight under the expected App, so its success cannot vouch for that PR.
func TestLifecyclePreflightWorkflowDiffCannotVouch(t *testing.T) {
	success := []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "success"})}
	for _, f := range []fileInfo{
		{Path: ".github/workflows/lifecycle-preflight.yml", Status: "added"},
		{Path: ".github/actions/setup/action.yml", Status: "modified"},
		{Path: "ci/moved.yml", Status: "renamed", PreviousFilename: ".github/workflows/old.yml"},
	} {
		files := []fileInfo{{Path: "code.go"}, f}
		if _, err := checkLifecyclePreflight("Brief: example/01\n", files, success, headSHA, lpAdopted, nil); err == nil || !strings.Contains(err.Error(), "workflow or action definition") {
			t.Fatalf("%+v: err=%v, want a workflow-diff refusal", f, err)
		}
		if adopted, err := checkLifecyclePreflight("Brief: example/01\n", files, nil, headSHA, lpNotAdopted, nil); err != nil || adopted {
			t.Fatalf("%+v not adopted: adopted=%v err=%v", f, adopted, err)
		}
	}
}

func TestDeliveryCannotFlipWithGreenApprovedButMissingLifecycleCheck(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	s.requiredChecks = []string{condLifecyclePreflight}
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
	s.requiredChecks = []string{condLifecyclePreflight}
	s.rollup = append(s.rollup, lpRun(rollupEntry{Status: "completed", Conclusion: "success"}))
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
			checks := []rollupEntry{lpRun(rollupEntry{Status: "completed", Conclusion: "success", StartedAt: tc.started})}
			_, err := checkLifecyclePreflight("Brief: example/01\n", []fileInfo{{Path: "code.go"}}, checks, headSHA, lpAdopted, read)
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
	s.requiredChecks = []string{condLifecyclePreflight}
	s.rollup = append(s.rollup, lpRun(rollupEntry{Status: "completed", Conclusion: "success"}))
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, condLifecyclePreflight) || len(s.mutated()) > 0 {
		t.Fatalf("out=%s mutated=%v", out, s.mutated())
	}
}

// S1, end to end: a successful lifecycle-preflight check run from another App satisfies
// checks-green's presence test but is not the lifecycle verdict.
func TestDeliveryCannotFlipOnForeignProducerLifecycleRun(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	s.requiredChecks = []string{condLifecyclePreflight}
	s.rollup = append(s.rollup, lpRun(rollupEntry{Status: "completed", Conclusion: "success", AppSlug: "some-bot"}))
	plantBriefRoot(t, privateCIRepo, "example-stream", "15", nonRisk)
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, `created by App "some-bot"`) || len(s.mutated()) > 0 {
		t.Fatalf("out=%s mutated=%v", out, s.mutated())
	}
}

// F3, end to end: a base branch that does not require lifecycle-preflight keeps today's
// behaviour — a delivery flips with no lifecycle check at all, and says why.
func TestDeliveryFlipsWhenLifecycleNotAdopted(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	plantBriefRoot(t, privateCIRepo, "example-stream", "15", nonRisk)
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !s.flipped() || !strings.Contains(out, "(not adopted)") {
		t.Fatalf("flipped=%v out=%s", s.flipped(), out)
	}
}

// F6: the observed success judged the FIRST body. A delivery body that changes between the
// two reads is unjudged even though a current-head success is present, so the flip refuses.
func TestDeliveryBodyChangedDuringGatingRefusesDespiteSuccess(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.pr.Body = "Brief: example-stream/15\n"
	s.body2 = "Brief: example-stream/16\n"
	s.requiredChecks = []string{condLifecyclePreflight}
	s.rollup = append(s.rollup, lpRun(rollupEntry{Status: "completed", Conclusion: "success"}))
	plantBriefRoot(t, privateCIRepo, "example-stream", "15", nonRisk)
	out := captureStderr(t, func() {
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, "changed during gating") || len(s.mutated()) > 0 {
		t.Fatalf("out=%s mutated=%v", out, s.mutated())
	}
}
