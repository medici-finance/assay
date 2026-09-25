package main

// html_test.go — unit/CLI coverage for `deskinbox html` beyond html_parity_test.go's
// byte-for-byte page-assembly check: argument handling, end-to-end dispatch against a fake
// Forge + stubbed statusgen/deskboard (no network, no real binary), and the self-contained-
// page property (Verify row 4) at the file-on-disk level.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func stubStatusgenDeskboard(t *testing.T) {
	t.Helper()
	withLookPath(t, map[string]string{"statusgen": "/fake/statusgen", "deskboard": "/fake/deskboard"})
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		switch {
		case contains(args, "--bottleneck"):
			return []byte(`{"constraint":"","stages":[]}`), nil, nil
		case contains(args, "--intake-debt"):
			return []byte(`{"state":"measured","untriaged":0}`), nil, nil
		case contains(args, "--net-flow"):
			return []byte(`{"state":"ok","streams":[]}`), nil, nil
		case contains(args, "throughput"):
			return []byte(`{"bottleneck":"","stagesRead":0,"stagesTotal":4,"advice":"nothing to widen","stages":[]}`), nil, nil
		}
		return nil, nil, errors.New("unexpected reader invocation")
	})
}

func TestRunHTMLNeedsAnOutputPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := run([]string{"html"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitRefused {
		t.Fatalf("want refused (%d), got %d", deskkit.ExitRefused, rc)
	}
	if !strings.Contains(stderr.String(), "needs an output path") {
		t.Errorf("want a needs-an-output-path refusal, got %q", stderr.String())
	}
}

func TestRunHTMLEndToEnd(t *testing.T) {
	stubStatusgenDeskboard(t)
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 5, Title: "the decision", Labels: []string{"needs-decision"}, CreatedAt: "2026-01-01T00:00:00Z", URL: "https://example.invalid/5"},
		}},
	})
	withFetchDetail(t, func(deskkit.ForgeRepo, int) issueDetail {
		return issueDetail{Body: "## Context\nsomething needs deciding\n\n## Options\nA. Do it\nB. Don't\n"}
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "inbox.html")
	var stdout, stderr bytes.Buffer
	rc := run([]string{"html", out, "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote 1 card(s)") {
		t.Errorf("want a wrote-N-cards line, got %q", stdout.String())
	}

	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("expected the page to be written: %v", err)
	}
	got := string(page)

	for _, want := range []string{
		"<!doctype html>",
		"example-org/example-repo#5",
		"something needs deciding",
		"Flow — how the system is performing",
		"<title>Assay inbox — decisions waiting</title>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("decision page missing %q", want)
		}
	}
	for _, forbidden := range []string{"url(", "<script", " src="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("decision page contains forbidden substring %q (Verify row 4)", forbidden)
		}
	}
}

func TestRunHTMLEmptyQueueStillWritesPage(t *testing.T) {
	stubStatusgenDeskboard(t)
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: nil},
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "inbox.html")
	var stdout, stderr bytes.Buffer
	rc := run([]string{"html", out, "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("expected the page to be written: %v", err)
	}
	if !strings.Contains(string(page), "Nothing is waiting on the driver right now.") {
		t.Errorf("empty-queue page missing the positive empty statement, got:\n%s", page)
	}
}

func TestRunHTMLRepoFailureIsUnverifiableButFlowSectionUnaffected(t *testing.T) {
	stubStatusgenDeskboard(t)
	withForge(t, map[string]*fakeForge{
		"example-org/bad-repo": {err: deskkit.Unverifiable("simulated failure", nil)},
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "inbox.html")
	var stdout, stderr bytes.Buffer
	rc := run([]string{"html", out, "example-org/bad-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("want unverifiable (%d), got %d", deskkit.ExitUnverifiable, rc)
	}
	if !strings.Contains(stdout.String(), "INCOMPLETE") {
		t.Errorf("want the summary to say INCOMPLETE, got %q", stdout.String())
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("a partial decision queue still writes the page it could build: %v", err)
	}
	if !strings.Contains(string(page), "Flow — how the system is performing") {
		t.Error("a failed issue query must not blank the Flow section, which reads a different source entirely")
	}
}

func TestRunHTMLFlowReaderFailureNeverReddensExitCode(t *testing.T) {
	// The oracle's own rule (assay-inbox.sh:112-118): a blind Flow section is reported in
	// the summary, but the html MODE's exit code stays a statement about the DECISION queue.
	withLookPath(t, map[string]string{"statusgen": "/fake/statusgen", "deskboard": "/fake/deskboard"})
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return nil, []byte("flag provided but not defined: -bottleneck"), &fakeExitError{code: 2}
	})
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: nil},
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "inbox.html")
	var stdout, stderr bytes.Buffer
	rc := run([]string{"html", out, "example-org/example-repo"}, &stdout, &stderr, time.Now())
	if rc != deskkit.ExitOK {
		t.Fatalf("a blind Flow section alone must not redden the html exit code; want 0, got %d", rc)
	}
	if !strings.Contains(stdout.String(), "the Flow section is INCOMPLETE") {
		t.Errorf("want the CLI summary to name the incomplete Flow section, got %q", stdout.String())
	}
	page, _ := os.ReadFile(out)
	if !strings.Contains(string(page), "the Flow section below is INCOMPLETE") {
		t.Errorf("want the PAGE's own summary to name the incomplete Flow section too, got:\n%s", page)
	}
}
