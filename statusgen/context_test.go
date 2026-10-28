package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
)

const ctxJourney = "testdata/contexthandoff/journeys/base"

// ctxCopyJourney copies the base snapshot into a temp dir a case may edit.
func ctxCopyJourney(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "snapshot")
	if err := os.CopyFS(dir, os.DirFS(ctxJourney)); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ctxRun runs the built binary and returns stdout, stderr and the exit code.
func ctxRun(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"context"}, args...)...)
	cmd.Env = []string{"PATH=", "HOME=" + t.TempDir()} // nothing to read from the environment
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("statusgen context %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

func ctxWriteTemp(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ctxDiscoverReceipt runs `context discover --json` and decodes its output with
// the statusgen/19 package.
func ctxDiscoverReceipt(t *testing.T, bin, snapshot string) (*contexthandoff.Handoff, []byte) {
	t.Helper()
	out, errOut, code := ctxRun(t, bin, "discover", "--snapshot", snapshot, "--json")
	if code != 0 {
		t.Fatalf("discover: exit %d\n%s", code, errOut)
	}
	h, err := contexthandoff.DecodeHandoff([]byte(out))
	if err != nil {
		t.Fatalf("discover output does not decode as a context handoff: %v\n%s", err, out)
	}
	return h, []byte(out)
}

// regression: statusgen/20 Verify row 9 — a CLI that drifts from the package:
// the built binary's receipt must decode with the statusgen/19 package and
// check clean against the snapshot it came from.
func TestContextCLIDiscoverRoundTrip(t *testing.T) {
	bin := buildStatusgenBinary(t)
	h, raw := ctxDiscoverReceipt(t, bin, ctxJourney)
	if !bytes.Equal(bytes.TrimSpace(raw), h.Canonical()) {
		t.Errorf("discover --json is not the canonical receipt")
	}
	s, err := contexthandoff.LoadSnapshot(os.DirFS(ctxJourney), contexthandoff.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := contexthandoff.Discover(s)
	if err != nil {
		t.Fatal(err)
	}
	if lib.Digest() != h.Digest() {
		t.Errorf("CLI receipt %s differs from the package's %s", h.Digest(), lib.Digest())
	}

	receipt := ctxWriteTemp(t, "receipt.json", raw)
	out, errOut, code := ctxRun(t, bin, "check", "--snapshot", ctxJourney, receipt, "--json")
	var rep contexthandoff.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("check --json: %v\n%s%s", err, out, errOut)
	}
	if code != 0 || rep.Outcome != contexthandoff.OutcomePass || !rep.Current || rep.Receipt != h.Digest() {
		t.Errorf("check on its own receipt: exit %d, report %+v\n%s", code, rep, errOut)
	}
	out, _, code = ctxRun(t, bin, "recheck", "--snapshot", ctxJourney, receipt)
	if code != 0 || !strings.Contains(out, "recheck "+h.Digest()+": current") {
		t.Errorf("recheck on its own receipt: exit %d\n%s", code, out)
	}
}

// regression: statusgen/20 Verify row 10 — an architecture receipt that reads
// as product alignment. Discovery through the CLI must state that it covers
// architecture only and that product alignment is unchecked.
func TestContextArchitectureCoverageNotProductAlignment(t *testing.T) {
	bin := buildStatusgenBinary(t)
	h, _ := ctxDiscoverReceipt(t, bin, ctxJourney)
	var scope *contexthandoff.Query
	for i, q := range h.Discovery.Queries {
		if q.Rule == contexthandoff.QueryCoverageScope {
			scope = &h.Discovery.Queries[i]
		}
	}
	if scope == nil || scope.Parameters["scope"] != "architecture" || scope.Parameters["product-alignment"] != "unchecked" ||
		len(scope.Results) != 1 || scope.Results[0] != "architecture" {
		t.Errorf("coverage-scope query = %+v, want architecture with product alignment unchecked", scope)
	}
	found := false
	for _, p := range h.Readiness.Prerequisites {
		if p == contexthandoff.ArchitectureOnly {
			found = true
		}
	}
	if !found || !strings.Contains(contexthandoff.ArchitectureOnly, "product alignment unchecked") {
		t.Errorf("prerequisites %v do not state product alignment unchecked", h.Readiness.Prerequisites)
	}
	out, _, code := ctxRun(t, bin, "discover", "--snapshot", ctxJourney)
	if code != 0 || !strings.Contains(out, "scope: architecture-only coverage: product alignment unchecked") {
		t.Errorf("discover text: exit %d, want the architecture-only scope line\n%s", code, out)
	}
	for _, claim := range []string{"product alignment: aligned", "product-aligned", "aligned to the product"} {
		if strings.Contains(strings.ToLower(out), claim) {
			t.Errorf("discover text claims %q", claim)
		}
	}
}

// regression: statusgen/20 Verify row 11 — a pickup recheck that trusts the
// prior read set misses guideline, owner, version and replacement changes and
// new consumers, and treats an unreadable input as clean.
func TestPickupContextRecheck(t *testing.T) {
	bin := buildStatusgenBinary(t)
	_, raw := ctxDiscoverReceipt(t, bin, ctxJourney)
	receipt := ctxWriteTemp(t, "receipt.json", raw)

	editJSON := func(t *testing.T, dir string, f func(map[string]any)) {
		t.Helper()
		p := filepath.Join(dir, "snapshot.json")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		f(v)
		if b, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write := func(t *testing.T, dir, rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	edge := func(v map[string]any) map[string]any { return v["edges"].([]any)[0].(map[string]any) }

	cases := []struct {
		name    string
		edit    func(t *testing.T, dir string)
		outcome string
		code    int
		scope   string // a finding scope the outcome must name
		class   string
		reason  string // and a fragment of its reason
	}{
		{"guideline added", func(t *testing.T, dir string) {
			write(t, dir, "sources/guides/docs/engineering/errors.md", "# Errors\n\nWrap every error with its input.\n")
		}, "reconcile", 1, "guideline:guides:docs/engineering/errors.md", contexthandoff.FindingReconcile, "guideline added"},
		{"guideline changed", func(t *testing.T, dir string) {
			write(t, dir, "sources/guides/docs/engineering/go.md", "# Go\n\nRun gofmt and go vet before every commit.\n")
		}, "reconcile", 1, "guideline:guides:docs/engineering/go.md", contexthandoff.FindingReconcile, "guideline changed"},
		{"approved owner changed", func(t *testing.T, dir string) {
			editJSON(t, dir, func(v map[string]any) {
				edge(v)["subject"] = "example:repo-b:beta:03"
				edge(v)["locator"] = "repo-b:docs/streams/beta/brief-03-cache-warmup.md"
			})
		}, "reconcile", 1, "subject:example:repo-b:beta:01", contexthandoff.FindingReconcile, "owner, producer version or planned replacement changed"},
		{"producer version changed", func(t *testing.T, dir string) {
			editJSON(t, dir, func(v map[string]any) { edge(v)["version"] = "v2" })
		}, "reconcile", 1, "subject:example:repo-b:beta:01", contexthandoff.FindingReconcile, "owner, producer version or planned replacement changed"},
		{"replacement declared", func(t *testing.T, dir string) {
			editJSON(t, dir, func(v map[string]any) {
				v["edges"] = append(v["edges"].([]any), map[string]any{
					"subject": "example:repo-b:beta:03", "edge": "replaces", "term": "ledger-export",
					"locator": "repo-b:docs/streams/beta/brief-03-cache-warmup.md", "version": "v2",
				})
			})
		}, "reconcile", 1, "subject:example:repo-b:beta:03", contexthandoff.FindingReconcile, "owner, producer version or planned replacement changed"},
		{"new consumer outside old membership", func(t *testing.T, dir string) {
			write(t, dir, "sources/repo-b/docs/streams/delta/brief-07-ledger-archive.md", "# Brief 07 — Ledger archive\n\n## Task\n\nKeep every ledger-export file for seven years.\n")
		}, "reconcile", 1, "subject:example:repo-b:delta:07", contexthandoff.FindingReconcile, "outside the prior read set"},
		{"unrelated edit", func(t *testing.T, dir string) {
			write(t, dir, "sources/repo-a/docs/streams/alpha/brief-05-audit-log-rotation.md", "# Brief 05 — Audit log rotation\n\n## Task\n\nRotate the audit log daily.\n")
		}, "current", 0, "file:repo-a:docs/streams/alpha/brief-05-audit-log-rotation.md", contexthandoff.FindingPreserved, "unrelated edit"},
		{"unavailable required input", func(t *testing.T, dir string) {
			if err := os.RemoveAll(filepath.Join(dir, "sources/guides")); err != nil {
				t.Fatal(err)
			}
		}, "hold", 3, "source:guides", contexthandoff.FindingHold, "could-not-check"},
		{"draft-only rival", func(t *testing.T, dir string) {
			s, err := contexthandoff.LoadSnapshot(os.DirFS(dir), contexthandoff.DefaultLimits)
			if err != nil {
				t.Fatal(err)
			}
			rival, err := contexthandoff.Discover(s)
			if err != nil {
				t.Fatal(err)
			}
			rival.Target.State, rival.Target.Approval = contexthandoff.TargetProposed, nil
			rival.Readiness.Facts.ProposalApproved = false
			rival.Readiness.NextAct = "explore"
			b, err := json.Marshal(rival)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contexthandoff.DecodeHandoff(b); err != nil {
				t.Fatalf("fixture rival is not contract-valid: %v", err)
			}
			write(t, dir, "proposals/draft.json", string(b))
			editJSON(t, dir, func(v map[string]any) { v["proposals"] = []any{"proposals/draft.json"} })
		}, "current", 0, "proposal:proposals/draft.json", contexthandoff.FindingIgnored, "unapproved rival"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := ctxCopyJourney(t)
			c.edit(t, dir)
			out, errOut, code := ctxRun(t, bin, "recheck", "--snapshot", dir, receipt, "--json")
			var got struct {
				Outcome  string                   `json:"outcome"`
				Findings []contexthandoff.Finding `json:"findings"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("recheck --json: %v\n%s%s", err, out, errOut)
			}
			if code != c.code || got.Outcome != c.outcome {
				t.Errorf("exit %d outcome %q, want %d %q; findings %+v", code, got.Outcome, c.code, c.outcome, got.Findings)
			}
			ok := false
			for _, f := range got.Findings {
				if f.Scope == c.scope && f.Class == c.class && strings.Contains(f.Reason, c.reason) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("findings %+v, want %s %s (%s)", got.Findings, c.class, c.scope, c.reason)
			}
		})
	}
}

// ctxBoolPtr prints an optional JSON boolean, naming an absent field.
func ctxBoolPtr(b *bool) string {
	if b == nil {
		return "absent"
	}
	return fmt.Sprint(*b)
}

// regression: a receipt whose read set was truncated at the result limit
// rechecked as "current" and checked as "pass" (exit 0) while printing
// "coverage: partial". Partial coverage, on either side, is a hold (exit 3).
func TestContextCLITruncatedCoverageHolds(t *testing.T) {
	bin := buildStatusgenBinary(t)
	const consumer = "sources/repo-b/docs/streams/beta/brief-02-export-consumer.md"
	appendEdit := func(t *testing.T, dir string) {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(dir, consumer), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString("\nAn added sentence that changes what the consumer must do.\n"); err != nil {
			t.Fatal(err)
		}
	}
	copyDir := func(t *testing.T, src string) string {
		t.Helper()
		dst := filepath.Join(t.TempDir(), "snapshot")
		if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	type verbOut struct {
		Outcome  string                   `json:"outcome"`
		Current  *bool                    `json:"current"`
		Findings []contexthandoff.Finding `json:"findings"`
	}
	run := func(t *testing.T, verb, dir, receipt string) (verbOut, int, string) {
		t.Helper()
		out, errOut, code := ctxRun(t, bin, verb, "--snapshot", dir, receipt, "--json")
		var got verbOut
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("%s --json: %v\n%s%s", verb, err, out, errOut)
		}
		text, _, _ := ctxRun(t, bin, verb, "--snapshot", dir, receipt)
		return got, code, text
	}

	// The reviewer's reproduction: 2100 filler files push the impacted briefs
	// past the read-set limit; then one impacted brief is edited.
	s1 := ctxCopyJourney(t)
	for i := 1; i <= 2100; i++ {
		p := filepath.Join(s1, "sources/repo-a/aaa", fmt.Sprintf("f%04d.md", i))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fmt.Sprintf("# filler %04d\n", i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, errOut, code := ctxRun(t, bin, "discover", "--snapshot", s1, "--json")
	if code != 3 {
		t.Fatalf("discover over a truncated snapshot: exit %d, want 3\n%s", code, errOut)
	}
	r1 := ctxWriteTemp(t, "r1.json", []byte(raw))
	s2 := copyDir(t, s1)
	appendEdit(t, s2)
	for _, verb := range []string{"recheck", "check"} {
		got, code, text := run(t, verb, s2, r1)
		if code != 3 || got.Outcome != "hold" || !strings.Contains(text, "coverage: partial") {
			t.Errorf("%s over partial coverage: exit %d outcome %q, want 3 hold; findings %+v\n%s", verb, code, got.Outcome, got.Findings, text)
		}
		if strings.Contains(text, ": current") || strings.Contains(text, ": pass (") || strings.Contains(text, "current: true") {
			t.Errorf("%s over partial coverage reads as complete:\n%s", verb, text)
		}
		if verb == "check" && (got.Current == nil || *got.Current || !strings.Contains(text, ": hold (current: false)")) {
			t.Errorf("check over partial coverage: --json current %s, want false, and text naming hold (current: false)\n%s", ctxBoolPtr(got.Current), text)
		}
	}

	// Control: the same edit with complete coverage reconciles (exit 1).
	r0raw, errOut, code := ctxRun(t, bin, "discover", "--snapshot", ctxJourney, "--json")
	if code != 0 {
		t.Fatalf("discover over the base journey: exit %d\n%s", code, errOut)
	}
	r0 := ctxWriteTemp(t, "r0.json", []byte(r0raw))
	s3 := ctxCopyJourney(t)
	appendEdit(t, s3)
	got, code, text := run(t, "recheck", s3, r0)
	if code != 1 || got.Outcome != "reconcile" || !strings.Contains(text, "reconcile: subject:example:repo-b:beta:02: the brief changed since the prior receipt") {
		t.Errorf("control recheck: exit %d outcome %q, want 1 reconcile naming the brief change\n%s", code, got.Outcome, text)
	}
	if got, code, text := run(t, "check", s3, r0); code != 1 || got.Outcome != "refuse" || got.Current == nil || *got.Current {
		t.Errorf("control check: exit %d outcome %q current %s, want 1 refuse, current false (stale receipt)\n%s", code, got.Outcome, ctxBoolPtr(got.Current), text)
	}
}

// Help and usage errors resolve no setting and read nothing: help names the
// verbs and exits 0 even with a snapshot that does not exist.
func TestContextHelpOffline(t *testing.T) {
	bin := buildStatusgenBinary(t)
	missing := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{{"--help"}, {"discover", "--help", "--snapshot", missing}, {"check", "--help"}} {
		out, _, code := ctxRun(t, bin, args...)
		if code != 0 || !strings.Contains(out, "--snapshot") && !strings.Contains(out, "discover") {
			t.Errorf("%v: exit %d\n%s", args, code, out)
		}
	}
	if _, errOut, code := ctxRun(t, bin, "discover"); code != 2 || !strings.Contains(errOut, "--snapshot DIR is required") {
		t.Errorf("discover without --snapshot: exit %d\n%s", code, errOut)
	}
	if _, _, code := ctxRun(t, bin, "discover", "--snapshot", missing); code != 1 {
		t.Errorf("discover on a missing snapshot: exit %d, want 1", code)
	}
	if _, _, code := ctxRun(t, bin); code != 2 {
		t.Errorf("bare context: exit %d, want 2", code)
	}
}
