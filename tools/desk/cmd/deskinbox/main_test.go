package main

// main_test.go — end-to-end `run()` dispatch: exit codes, mode routing, and the
// not-yet-ported refusals, against the fake-Forge/fetchDetail seams (no network).

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func withFetchDetail(t *testing.T, fn func(deskkit.ForgeRepo, int) issueDetail) {
	t.Helper()
	prev := fetchDetail
	fetchDetail = fn
	t.Cleanup(func() { fetchDetail = prev })
}

func TestRunTableMode(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 1, Title: "an urgent item", Labels: []string{"urgent"}, CreatedAt: "2026-01-01T00:00:00Z", URL: "https://example.invalid/1"},
		}},
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "#1") || !strings.Contains(stdout.String(), "an urgent item") {
		t.Errorf("table output missing expected row: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "deskinbox: 1 item(s) across 1 repo(s)") {
		t.Errorf("table output missing summary line: %q", stdout.String())
	}
}

func TestRunWalkMode(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 5, Title: "the decision", Labels: []string{"needs-decision"}, CreatedAt: "2026-01-01T00:00:00Z", URL: "https://example.invalid/5"},
		}},
	})
	withFetchDetail(t, func(deskkit.ForgeRepo, int) issueDetail {
		return issueDetail{Body: "## Context\nsomething needs deciding\n"}
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"walk", "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"example-org/example-repo#5", "Context", "something needs deciding", "Options", "Reply shape", "Verification"} {
		if !strings.Contains(out, want) {
			t.Errorf("walk output missing %q:\n%s", want, out)
		}
	}
}

func TestRunWalkModeEmptyQueue(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: nil},
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"walk", "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("empty queue must still exit 0, got %d", rc)
	}
	if !strings.Contains(stdout.String(), "0 item(s)") {
		t.Errorf("want a positive '0 item(s)' statement, got %q", stdout.String())
	}
}

func TestRunWalkModeItemOutOfRange(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 1, Title: "one", Labels: []string{"urgent"}, CreatedAt: "2026-01-01T00:00:00Z"},
		}},
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"walk", "--item", "5", "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitRefused {
		t.Fatalf("out-of-range --item must refuse (exit %d), got %d", deskkit.ExitRefused, rc)
	}
	if !strings.Contains(stderr.String(), "out of range") {
		t.Errorf("want an out-of-range message, got %q", stderr.String())
	}
}

func TestRunRepoFailureIsUnverifiableNotSilent(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/bad-repo": {err: deskkit.Unverifiable("simulated failure", nil)},
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"example-org/bad-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("a failed repo read must exit unverifiable (%d), got %d", deskkit.ExitUnverifiable, rc)
	}
	if !strings.Contains(stdout.String(), "INCOMPLETE") {
		t.Errorf("summary must say INCOMPLETE, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "QUERY FAILED") {
		t.Errorf("stderr must name the failed query, got %q", stderr.String())
	}
}

func TestRunHTMLAndFlowAreRefusedNotSilentlyIgnored(t *testing.T) {
	for _, flag := range []string{"--html", "--flow"} {
		var stdout, stderr bytes.Buffer
		args := []string{flag}
		if flag == "--html" {
			args = append(args, "out.html")
		}
		rc := run(args, &stdout, &stderr, time.Now())
		if rc != deskkit.ExitRefused {
			t.Errorf("%s: want refused (%d), got %d", flag, deskkit.ExitRefused, rc)
		}
		if !strings.Contains(stderr.String(), "not yet ported") {
			t.Errorf("%s: want a 'not yet ported' message naming the oracle fallback, got %q", flag, stderr.String())
		}
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := run([]string{"--help"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("--help must exit 0, got %d", rc)
	}
	if !strings.Contains(stdout.String(), "deskinbox") {
		t.Errorf("help text missing the command name: %q", stdout.String())
	}
}

func TestRunNoRepoResolvableRefuses(t *testing.T) {
	// No repo args, no .assay/repos.txt in cwd, and (in this build/test sandbox) no
	// resolvable git origin remote for the test binary's working directory — the
	// oracle's own "no repos to query" refusal, never a guessed empty success.
	//
	// This exercises the SAME code path TestRunTableMode/TestRunWalkMode take when repos
	// ARE resolvable; it is skipped rather than asserted when the test's own working
	// directory happens to sit inside a repo with a resolvable origin (this checkout does),
	// which would make "no repos resolved" the wrong expectation for reasons unrelated to
	// the code under test.
	t.Skip("environment-dependent (this checkout's own origin remote resolves); covered directly by TestResolveReposEmptyWhenNothingResolves")
}
