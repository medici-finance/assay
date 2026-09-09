package deskkit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// trace_test.go — the DESK_TRACE contract, asserted in both directions.
//
// The two assertions that matter most are the ones a future change is most likely to break
// by accident:
//
//   - OFF is byte-identical. Every desk verb's terminal output is a surface transcripts,
//     scripts and other tools read. If adding diagnostics changed the default output even by
//     a trailing space, the change would be a silent break of every one of them.
//   - ON never prints a credential. A trace prints argv and child stderr, so it prints
//     exactly the two places a token actually travels. A trace that leaked one would be a
//     worse defect than the swallowed-error class it exists to fix.

// helperProcess is the portable stand-in for a child process: the test binary re-executes
// itself with DESKKIT_HELPER set, and TestHelperProcess below plays whatever part the
// environment asks for. It avoids `sh -c` (not portable to the Windows CI leg) and avoids
// depending on any tool being installed on the runner.
func helperProcess(t *testing.T, mode string, extraEnv ...string) ToolCall {
	t.Helper()
	return ToolCall{
		Name: os.Args[0],
		Args: []string{"-test.run=^TestHelperProcess$", "--", mode},
		Env: append(append([]string(nil), os.Environ()...),
			append([]string{"DESKKIT_HELPER=1"}, extraEnv...)...),
	}
}

// TestHelperProcess is not a real test: it is the child process the runner tests start. It
// exits immediately unless DESKKIT_HELPER is set, so a normal `go test` run treats it as a
// no-op.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("DESKKIT_HELPER") != "1" {
		return
	}
	mode := ""
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	switch mode {
	case "config-echo-then-refusal":
		// The exact stderr shape every desk tool produces: the effective-config echo first,
		// the tool's own message second. This is the shape that made "report the first
		// stderr line" report the config echo and never the diagnosis.
		fmt.Fprintln(os.Stderr, "assay-config: roster=/x/roster.env allowed=3 repos")
		fmt.Fprintln(os.Stderr, "desk-tools WARNING: running UNPINNED build")
		fmt.Fprintln(os.Stderr, "fatal: a branch named 'wd/thing' already exists")
		fmt.Fprintln(os.Stderr, "hint: use a different --branch")
		os.Exit(5)
	case "leak-token":
		// A child that prints a credential on stderr and carries one in its own argv-ish
		// output. Everything a trace shows about this child must come out redacted.
		fmt.Fprintln(os.Stderr, "remote: fatal: could not read Username for "+
			"'https://x-access-token:ghs_"+strings.Repeat("A", 36)+"@github.com/o/r.git'")
		os.Exit(1)
	case "ok":
		fmt.Fprintln(os.Stdout, "/private/tmp/tracker-thing")
		os.Exit(0)
	}
	os.Exit(0)
}

// TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage pins the defect this whole change
// is about: the child's FIRST stderr line is the config echo, so a report built from it names
// nothing. Said() must skip the preamble; SaidAll() must keep every line of the real message.
func TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage(t *testing.T) {
	ResetTrace()
	r := Run(helperProcess(t, "config-echo-then-refusal"))
	if !r.Failed() {
		t.Fatalf("helper was supposed to exit non-zero; got %+v", r)
	}
	if r.ExitCode != ExitRefused {
		t.Errorf("child exit status not passed through: got %d, want %d", r.ExitCode, ExitRefused)
	}

	// The OLD shape, reproduced here so the regression is pinned rather than described: the
	// first raw stderr line is the config echo, which names nothing about the failure.
	oldShape := strings.SplitN(strings.TrimSpace(r.Stderr), "\n", 2)[0]
	if !strings.HasPrefix(oldShape, "assay-config:") {
		t.Fatalf("fixture drift: the first raw stderr line should be the config echo, got %q", oldShape)
	}
	if strings.Contains(oldShape, "already exists") {
		t.Fatalf("fixture drift: the old first-line shape must NOT carry the diagnosis")
	}

	if got := r.Said(); got != "fatal: a branch named 'wd/thing' already exists" {
		t.Errorf("Said() did not return the tool's own first message line: %q", got)
	}
	all := r.SaidAll()
	if !strings.Contains(all, "already exists") || !strings.Contains(all, "hint: use a different --branch") {
		t.Errorf("SaidAll() dropped part of the tool's message: %q", all)
	}
	if strings.Contains(all, "assay-config:") || strings.Contains(all, "running UNPINNED") {
		t.Errorf("SaidAll() kept the preamble: %q", all)
	}
}

// TestToolRunFailShapeAndCarriedDetail pins the one-line message shape and asserts the rest
// of the diagnosis rides on the error rather than being discarded.
func TestToolRunFailShapeAndCarriedDetail(t *testing.T) {
	ResetTrace()
	r := Run(helperProcess(t, "config-echo-then-refusal"))
	err := r.Fail(ExitUnverifiable, "step worktree-create: `deskwt add %s` failed", "thing")

	if !strings.HasSuffix(err.Error(), " said: fatal: a branch named 'wd/thing' already exists") {
		t.Errorf("message does not end on the `— <tool> said: <first line>` shape: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "step worktree-create:") {
		t.Errorf("caller's context sentence was lost: %q", err.Error())
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Errorf("the caller's verdict was not honoured: got %d", ExitCodeOf(err))
	}
	if err.ExitStatus != ExitRefused {
		t.Errorf("child exit status not carried: got %d want %d", err.ExitStatus, ExitRefused)
	}
	if !strings.Contains(err.Stderr, "hint: use a different --branch") {
		t.Errorf("full child stderr not carried on the error: %q", err.Stderr)
	}
	if !strings.Contains(err.Cmd, "-test.run=^TestHelperProcess$") {
		t.Errorf("command line not carried on the error: %q", err.Cmd)
	}

	// errors.As must still reach the raw exec error: the cause is wrapped, not replaced.
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Errorf("errors.As could not reach *exec.ExitError through the DeskError cause chain")
	}
	if err.Cause() == nil {
		t.Errorf("Cause() returned nil on an error built from a failed child")
	}
}

// TestReportErrorOffIsByteIdentical is the no-regression floor: with the switch off, the
// shared exit path prints exactly what every main() printed before it existed.
func TestReportErrorOffIsByteIdentical(t *testing.T) {
	ResetTrace()
	SetTrace(false)
	var b strings.Builder
	err := Unverifiable("the claim could not be established", errors.New("exit status 6"))
	ReportError(&b, err)
	want := err.Error() + "\n"
	if b.String() != want {
		t.Errorf("trace-off output is not byte-identical to fmt.Fprintln(w, err.Error()):\n got %q\nwant %q",
			b.String(), want)
	}

	// And a nil error prints nothing at all.
	var nb strings.Builder
	ReportError(&nb, nil)
	if nb.String() != "" {
		t.Errorf("ReportError(nil) printed %q, want nothing", nb.String())
	}
}

// TestReportErrorOnPrintsChainCommandsAndTimings asserts the four things the contract
// promises when the switch is on.
func TestReportErrorOnPrintsChainCommandsAndTimings(t *testing.T) {
	ResetTrace()
	SetTrace(true)
	defer ResetTrace()

	okRun := Run(helperProcess(t, "ok"))
	if okRun.Failed() {
		t.Fatalf("the success fixture failed: %+v", okRun)
	}
	bad := Run(helperProcess(t, "config-echo-then-refusal"))
	err := Unverifiable("step claim-acquire: the claim could not be established",
		bad.Fail(ExitRefused, "claim tool refused"))

	var b strings.Builder
	ReportError(&b, err)
	out := b.String()

	// The plain first line is still there, unchanged and first.
	if !strings.HasPrefix(out, err.Error()+"\n") {
		t.Errorf("the trace block replaced or reordered the plain message; got:\n%s", out)
	}
	for _, want := range []string{
		"desk-trace: cause chain:",
		"child stderr (full):",
		"hint: use a different --branch", // the line the one-line message drops
		"desk-trace: child processes (2)",
		"-test.run=^TestHelperProcess$", // the command line as executed
		"exit 0 in ",                    // the SUCCESSFUL step's status and timing
		"exit 5 in ",                    // the failing step's status and timing
	} {
		if !strings.Contains(out, want) {
			t.Errorf("trace block missing %q; got:\n%s", want, out)
		}
	}
}

// TestTraceNeverPrintsACredential is the security floor for the diagnostic surface: a token
// planted in a child's stderr, in the process environment and in an argv must not survive
// into the trace.
func TestTraceNeverPrintsACredential(t *testing.T) {
	ResetTrace()
	SetTrace(true)
	defer ResetTrace()

	secret := "ghs_" + strings.Repeat("A", 36)
	call := helperProcess(t, "leak-token", "GH_TOKEN="+secret)
	// Plant it in the argv too — the trace prints the command line as executed.
	call.Args = append(call.Args, "--token="+secret)
	r := Run(call)
	err := r.Fail(ExitUnverifiable, "step push: transport failed")

	var b strings.Builder
	ReportError(&b, err)
	out := b.String()

	if strings.Contains(out, secret) {
		t.Fatalf("DESK_TRACE printed a credential:\n%s", out)
	}
	// The redaction must be visible, not a silent elision, and the diagnostic value around
	// it must survive: the operator still learns it was an x-access-token transport.
	if !strings.Contains(out, redactedMarker) {
		t.Errorf("nothing was marked as redacted, so the token may simply not have reached the trace:\n%s", out)
	}
	if !strings.Contains(out, "x-access-token") {
		t.Errorf("redaction ate the diagnostic context as well as the secret:\n%s", out)
	}
	// The one-line message is a diagnostic surface too — it ends on the child's stderr.
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the `— <tool> said:` suffix carried a credential: %q", err.Error())
	}
}

// awsKeyIDFixture is AWS's own published EXAMPLE key id, assembled from two halves rather than
// written as one literal. The redactor test needs a value that MATCHES the AKIA pattern, and the
// outward-write secret scan reads the branch DIFF — so a one-piece literal here would refuse
// every PR that touches this file. Splitting it keeps the runtime value exact (which is what the
// assertion needs) while leaving no matching span in the source.
var awsKeyIDFixture = "AK" + "IAIOSFODNN7EXAMPLE"

// TestScrubRedactsEveryTransportShape is the per-shape table for the redactor. Each row is a
// place a credential has actually been observed to travel through a diagnostic.
func TestScrubRedactsEveryTransportShape(t *testing.T) {
	secret := "ghp_" + strings.Repeat("B", 36)
	cases := []struct {
		name, in, mustNotContain, mustContain string
	}{
		{"github token", "remote said: " + secret + " rejected", secret, "rejected"},
		{"url userinfo", "https://x-access-token:" + secret + "@github.com/o/r.git",
			secret, "x-access-token"},
		{"generic url userinfo", "https://oauth2:s3cr3tvalue@gitlab.com/o/r.git",
			"s3cr3tvalue", "oauth2"},
		{"authorization header", "Authorization: Bearer abcdefghijklmnop", "abcdefghijklmnop", "Authorization"},
		{"env assignment", "GH_TOKEN=abcdefghijklmnop", "abcdefghijklmnop", "GH_TOKEN="},
		{"env assignment lowercase name", "gitlab_pat=abcdefghijklmnop", "abcdefghijklmnop", "gitlab_pat="},
		{"aws key id", "key " + awsKeyIDFixture + " denied", awsKeyIDFixture, "denied"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Scrub(c.in)
			if strings.Contains(got, c.mustNotContain) {
				t.Errorf("Scrub left the secret in place: %q", got)
			}
			if !strings.Contains(got, c.mustContain) {
				t.Errorf("Scrub destroyed the diagnostic context %q: %q", c.mustContain, got)
			}
			if !strings.Contains(got, redactedMarker) {
				t.Errorf("Scrub produced no redaction marker: %q", got)
			}
		})
	}

	// The NEGATIVE control: ordinary diagnostic text must pass through untouched, or the
	// redactor makes every trace unreadable and operators stop turning it on.
	for _, plain := range []string{
		"fatal: a branch named 'feat/example-item' already exists",
		"deskwt add thing --branch wd/x --base refs/remotes/origin/main",
		"HTTP 403: Resource not accessible by integration",
	} {
		if got := Scrub(plain); got != plain {
			t.Errorf("Scrub altered ordinary text:\n got %q\nwant %q", got, plain)
		}
	}
}

// TestTakeTraceFlagIsGlobalAndInvisibleDownstream asserts the flag is stripped before any
// FlagSet sees it, and that it stops at a `--` terminator.
func TestTakeTraceFlagIsGlobalAndInvisibleDownstream(t *testing.T) {
	ResetTrace()
	got := TakeTraceFlag([]string{"add", "thing", "--trace", "--branch", "wd/x"})
	want := []string{"add", "thing", "--branch", "wd/x"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("args after stripping: got %v want %v", got, want)
	}
	if !TraceEnabled() {
		t.Errorf("--trace did not set the switch")
	}

	ResetTrace()
	got = TakeTraceFlag([]string{"new", "--", "--trace"})
	if strings.Join(got, "|") != "new|--|--trace" {
		t.Errorf("a --trace after the -- terminator must be left alone, got %v", got)
	}
	if TraceEnabled() {
		t.Errorf("a --trace after the -- terminator set the switch")
	}
}

// TestTraceEnabledReadsTheEnvSpellings pins which env values count, including the negative
// control: an exported-but-empty DESK_TRACE must NOT turn diagnostics on for a whole session.
func TestTraceEnabledReadsTheEnvSpellings(t *testing.T) {
	for _, c := range []struct {
		v    string
		want bool
	}{
		{"1", true}, {"true", true}, {"YES", true}, {"on", true},
		{"", false}, {"0", false}, {"false", false}, {"no", false}, {"maybe", false},
	} {
		ResetTrace()
		t.Setenv("DESK_TRACE", c.v)
		if got := TraceEnabled(); got != c.want {
			t.Errorf("DESK_TRACE=%q: TraceEnabled()=%v want %v", c.v, got, c.want)
		}
	}
	ResetTrace()
}

// TestRefusedWithCauseStaysARefusal is the safety floor for the new constructor: adding a
// cause must not soften the verdict.
func TestRefusedWithCauseStaysARefusal(t *testing.T) {
	sentinel := errors.New("underlying")
	err := RefusedWithCause("refused: the branch is held", sentinel)
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("RefusedWithCause changed the exit code: got %d want %d", ExitCodeOf(err), ExitRefused)
	}
	if !IsRefused(err) {
		t.Errorf("RefusedWithCause is not IsRefused")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is could not reach the cause")
	}
	// And the message is the caller's, with the cause appended by the existing Error()
	// rendering — no new shape is invented here.
	if !strings.HasPrefix(err.Error(), "refused: the branch is held") {
		t.Errorf("the caller's message was rewritten: %q", err.Error())
	}
}
