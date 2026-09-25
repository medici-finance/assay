package main

// html_test.go — unit/CLI coverage for `deskinbox html` beyond html_parity_test.go's
// byte-for-byte page-assembly check: argument handling, end-to-end dispatch against a fake
// Forge + stubbed statusgen/deskboard (no network, no real binary), and the self-contained-
// page property (Verify row 4) at the file-on-disk level.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(out); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("decision page must be written owner-only (0600): stat=%v err=%v", fi.Mode().Perm(), err)
		}
	}
}

func TestRunHTML_EmptyQueueStillWritesPage(t *testing.T) {
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

func TestRunHTML_RepoFailure_IsUnverifiableButFlowSectionUnaffected(t *testing.T) {
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

func TestRunHTML_FlowReaderFailure_NeverReddensExitCode(t *testing.T) {
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

// hostilePayload is an attribute-breakout + element-injection + scheme payload. Every place it
// lands on the page must come out as inert text: no `<script`/`<img` element, no raw `"`
// closing an attribute.
const hostilePayload = `"><script>alert(1)</script><img src=x onerror=alert(2)>' javascript:x`

// TestPageEscapesHostileInput feeds hostilePayload through every body-derived card field and
// every free-text flow-model field buildDecisionPage/buildFlowOnlyPage render, and asserts the
// page carries none of it as markup. The parity fixtures use benign text (one title with
// `<`, `&` and `"`), so without this test an escaping regression in a field they do not
// exercise would pass.
func TestPageEscapesHostileInput(t *testing.T) {
	var doc rawDoc
	fx := flowFixtures()[1] // the blind fixture: its rows carry Blind/FlowBlind text to overwrite
	if err := json.Unmarshal([]byte(fx.raw), &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	m := interpretFlow(doc)
	m.AsOf, m.Since, m.Advice, m.Note, m.Bottleneck = hostilePayload, hostilePayload, hostilePayload, hostilePayload, hostilePayload
	m.Sources = []string{hostilePayload}
	m.Blind = []string{hostilePayload}
	for i := range m.Rows {
		m.Rows[i].Cell, m.Rows[i].FlowBlind, m.Rows[i].CapacityNote = hostilePayload, hostilePayload, hostilePayload
		for j := range m.Rows[i].Stages {
			st := &m.Rows[i].Stages[j]
			st.Blind, st.CountSource, st.QueueBlind, st.Dwell = hostilePayload, hostilePayload, hostilePayload, hostilePayload
		}
	}
	cards := []cardData{{
		Item:  item{Repo: hostilePayload, Number: 1, URL: "javascript:alert(3)", Title: hostilePayload},
		Index: 1, Total: 1,
		Rendered: rendered{
			Context: []string{hostilePayload}, Options: []option{{Letter: hostilePayload, Text: hostilePayload, Recommended: true}},
			OptionsStated: true, Reply: hostilePayload, Verification: hostilePayload,
		},
		Class: hostilePayload, ClassEvidence: hostilePayload,
	}}
	pages := map[string]string{
		"decision page":  buildDecisionPage(hostilePayload, cards, m),
		"flow-only page": buildFlowOnlyPage(m),
	}
	for name, page := range pages {
		for _, bad := range []string{"<script", "<img", "onerror=alert(2)>", "x' javascript", "javascript:alert(3)"} {
			if strings.Contains(page, bad) {
				t.Errorf("%s: hostile input reached the page as markup — found %q", name, bad)
			}
		}
	}
	if !strings.Contains(pages["decision page"], `<a href="#">`) {
		t.Error("a non-http(s) card URL must render as href=\"#\"")
	}
	if !strings.Contains(pages["decision page"], "&quot;&gt;&lt;script&gt;") {
		t.Error("expected the payload to appear escaped as text (&quot;&gt;&lt;script&gt;)")
	}
}

// TestSafeHref pins the scheme allow-list: http(s) pass through unchanged (so a real forge URL
// renders exactly as the oracle renders it), every other scheme becomes "#".
func TestSafeHref(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.invalid/issues/1": "https://example.invalid/issues/1",
		"HTTP://example.invalid/x":         "HTTP://example.invalid/x",
		"javascript:alert(1)":              "#",
		" javascript:alert(1)":             "#",
		"data:text/html,<b>":               "#",
		"":                                 "#",
		"/relative":                        "#",
	} {
		if got := safeHref(in); got != want {
			t.Errorf("safeHref(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPageFilesAreOwnerOnly: the decision page embeds issue bodies and comments from every
// repo queried (private ones included), so both writers create it 0600, not world-readable.
func TestPageFilesAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: POSIX mode bits are not meaningful on Windows")
	}
	stubStatusgenDeskboard(t)
	dir := t.TempDir()
	flowOut := filepath.Join(dir, "flow.html")
	var stdout, stderr bytes.Buffer
	if rc := run([]string{"flow", "--html", flowOut, "--root", "."}, &stdout, &stderr, time.Now()); rc != 0 {
		t.Fatalf("flow --html: want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	fi, err := os.Stat(flowOut)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("flow page mode = %o, want 600", perm)
	}
}
