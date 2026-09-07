package main

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// captureStderr swaps os.Stderr for a pipe while fn runs and returns what fn wrote to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = old
	return <-done
}

// TestExplainFlagPrintsFinding is Verify row 7: with --explain a secret-scan refusal also
// prints a scan-explain line naming the rule and line (never the span); WITHOUT the flag
// the refusal message is byte-identical to today's and no explain line is emitted. It runs
// against the PR-body scan, which refuses before any network call.
func TestExplainFlagPrintsFinding(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	// A 40-char synthetic base64 secret (mixed-case, no slash) on the body's single line.
	secret := "Qx7pLk2wZt9mNc4bYf6RhVs8" + "Ju3XoAeG5idWn1Dz"
	base := []string{"--title", "fix: a change", "--body-min", "leak: " + secret}

	var errPlain, errEx error
	plainOut := captureStderr(t, func() { errPlain = cmdCreate(append([]string{}, base...)) })
	exOut := captureStderr(t, func() { errEx = cmdCreate(append(append([]string{}, base...), "--explain")) })

	// Both directions refuse (exit 5).
	if !deskkit.IsRefused(errPlain) || !deskkit.IsRefused(errEx) {
		t.Fatalf("both must refuse; plain=%v explain=%v", errPlain, errEx)
	}
	// The refusal MESSAGE is byte-identical with and without --explain — the flag only
	// controls a separate stderr line, never the error text.
	if errPlain.Error() != errEx.Error() {
		t.Fatalf("--explain changed the refusal message:\n plain:   %q\n explain: %q", errPlain.Error(), errEx.Error())
	}
	// The refusal carries the structured finding in BOTH cases (the finding is always
	// attached; --explain only decides whether it is printed).
	var f *deskkit.ScanFinding
	if !errors.As(errEx, &f) {
		t.Fatalf("refusal carried no ScanFinding: %v", errEx)
	}
	if f.Rule != "high-entropy-run" {
		t.Fatalf("finding rule = %q, want high-entropy-run", f.Rule)
	}

	// Default output does not carry a scan-explain line.
	if strings.Contains(plainOut, "scan-explain") {
		t.Fatalf("default (no --explain) output carried an explain line:\n%s", plainOut)
	}
	// --explain prints the rule id and line number, and NEVER the offending span.
	if !strings.Contains(exOut, "scan-explain: rule=high-entropy-run") {
		t.Fatalf("--explain did not print the finding:\n%s", exOut)
	}
	if !strings.Contains(exOut, "line=1") {
		t.Fatalf("--explain did not name the line number:\n%s", exOut)
	}
	if strings.Contains(exOut, secret) {
		t.Fatalf("--explain LEAKED the offending span:\n%s", exOut)
	}
}
