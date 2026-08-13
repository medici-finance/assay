package deskkit

// Tests for the SHARED callout plumbing. They pin the contract every caller's
// fail-closed rule is written against: Run returns an error for every way the
// question can go unanswered, and it returns within its deadline.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixtureCallout(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "callout")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return p
}

func TestCalloutRunHappyPath(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "read line\necho \"saw:$line\"\necho oops >&2\n")}
	res, err := c.Run("hello\n")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "saw:hello" {
		t.Fatalf("stdout = %q — stdin did not reach the callout", res.Stdout)
	}
	if res.Stderr != "oops" {
		t.Fatalf("stderr = %q — a caller cannot quote the callout's own diagnostic", res.Stderr)
	}
}

func TestCalloutRunPassesArgsWithoutShell(t *testing.T) {
	// The argument contains shell metacharacters. exec runs the binary directly, so
	// they must arrive VERBATIM — a callout that received them word-split or expanded
	// would mean the guard's own inputs are shell-interpreted somewhere.
	c := Callout{Path: fixtureCallout(t, "printf '%s' \"$1\"\n")}
	arg := "a b; rm -rf /; $(echo pwn) *"
	res, err := c.Run("", arg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != arg {
		t.Fatalf("argument arrived as %q, want %q — something interpreted it", res.Stdout, arg)
	}
}

// TestCalloutRunErrorsWhenUnanswered is the contract every fail-closed
// caller depends on: `err != nil` is the WHOLE condition, so there is no failure mode
// a caller can forget to enumerate.
func TestCalloutRunErrorsWhenUnanswered(t *testing.T) {
	notExec := filepath.Join(t.TempDir(), "not-exec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\necho allow\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	groupWritable := fixtureCallout(t, "echo allow\n")
	if err := os.Chmod(groupWritable, 0o775); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	writableDir := t.TempDir()
	if err := os.Chmod(writableDir, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	inWritableDir := filepath.Join(writableDir, "callout")
	if err := os.WriteFile(inWritableDir, []byte("#!/bin/sh\necho allow\n"), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	for name, c := range map[string]Callout{
		"empty path":         {Path: ""},
		"relative path":      {Path: "relative/callout"},
		"missing file":       {Path: filepath.Join(t.TempDir(), "nope")},
		"a directory":        {Path: t.TempDir()},
		"not executable":     {Path: notExec},
		"group-writable":     {Path: groupWritable},
		"group-writable dir": {Path: inWritableDir},
		"non-zero exit":      {Path: fixtureCallout(t, "exit 9\n")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Run(""); err == nil {
				t.Fatalf("%s did not error — a fail-closed caller would treat it as an answer", name)
			}
		})
	}
}

// TestCalloutRunTimeoutIsReal is the WaitDelay regression.
//
// Because Run pipes stdout/stderr through writers, Wait blocks until the write end
// closes — and killing the callout on the deadline does NOT close it when the callout
// left a child holding the pipe. Without cmd.WaitDelay a 150ms deadline against a
// callout whose child slept 30s returned in 30s: the timeout was advertised and not
// enforced, which for a fail-closed caller means the guard hangs rather than refuses.
func TestCalloutRunTimeoutIsReal(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "sleep 30\necho allow\n"), Timeout: 150 * time.Millisecond}
	start := time.Now()
	_, err := c.Run("")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a callout that never answered returned no error")
	}
	if !strings.Contains(err.Error(), "did not answer within") {
		t.Fatalf("the error does not name the timeout: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Run took %s for a 150ms deadline — the deadline is not enforced against a "+
			"callout whose child holds the output pipe", elapsed)
	}
}

// TestCalloutRunBoundsOutput — a callout that streams is not answering the question,
// and how much memory the guard spends on it must not be the adopter's to choose.
func TestCalloutRunBoundsOutput(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "i=0\nwhile [ $i -lt 4000 ]; do printf '0123456789012345678901234567890123456789\\n'; i=$((i+1)); done\n")}
	res, err := c.Run("")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Stdout) > maxCalloutOutput {
		t.Fatalf("stdout = %d bytes, want at most %d", len(res.Stdout), maxCalloutOutput)
	}
}
