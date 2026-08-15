package deskkit

// Tests for the risk-classification callout: the
// calloutClassifier / unionClassifier pair in riskclassifier.go and riskcallout.go's
// exec/timeout/fail-closed plumbing.
//
// Every callout fixture here is either the checked-in testdata script (used to
// exercise BOTH of its answers) or a scratch script this test writes into its own
// temp dir. None of them is anybody's real risk-classification policy: what is
// under test is the DESK's half of the Q1 contract — that a configured callout is
// consulted, that its answer reaches the classifier, and that every way it can
// fail to answer classes the diff rather than clearing it.

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// riskCalloutRepo is a private, in-set repo whose changed files below the
// compiled trigger set are used to isolate the CALLOUT's contribution from the
// pattern classifier's.
const riskCalloutRepo = "acme/private-widget"

// riskCalloutCleanFile matches no compiled trigger for riskCalloutRepo — any
// classification a test observes on it must come from the callout, not the
// pattern classifier.
var riskCalloutCleanFile = []string{"README.md"}

// riskCalloutRoster installs a roster carrying one in-set private repo and,
// unless calloutPath is "", a configured ASSAY_RISK_CALLOUT.
func riskCalloutRoster(t *testing.T, calloutPath string) {
	t.Helper()
	r := goldenRoster()
	r[EnvAllowedRepos] = riskCalloutRepo + ":ci:private"
	if calloutPath != "" {
		r[EnvRiskCallout] = calloutPath
	}
	withRoster(t, r)
}

// writeRiskCalloutFixture writes an executable POSIX shell script into its own
// fresh, non-writable-by-others temp dir and returns its absolute path — the
// shape riskCalloutExecutableCheck requires.
func writeRiskCalloutFixture(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod fixture dir: %v", err)
	}
	p := filepath.Join(dir, "callout")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return p
}

// riskCalloutTestdataFixture resolves the checked-in fixture script to an
// absolute path (ASSAY_RISK_CALLOUT and runRiskCallout both require one).
func riskCalloutTestdataFixture(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "riskcallout", "fixture-answer.sh"))
	if err != nil {
		t.Fatalf("resolving the fixture path: %v", err)
	}
	if fi, err := os.Stat(abs); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("fixture-answer.sh missing or not executable at %s (err=%v)", abs, err)
	}
	return abs
}

// --- Verify row 1: every failure mode asserts classed=true -------------------

// TestCalloutNonZeroExit — a callout that exits non-zero answers classed=true.
func TestCalloutNonZeroExit(t *testing.T) {
	riskCalloutRoster(t, writeRiskCalloutFixture(t, "cat >/dev/null\nexit 7\n"))
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("a non-zero-exit callout was not classed — a fail-closed gate must never treat an unanswered question as clean")
	}
	if !strings.Contains(reason, "did not answer") {
		t.Fatalf("reason does not name the failure: %q", reason)
	}
}

// TestCalloutGarbageStdout — a callout that exits 0 but prints unparseable
// output answers classed=true. Exit 0 alone is not "answered": the output must
// also parse as the Q1 response shape.
func TestCalloutGarbageStdout(t *testing.T) {
	riskCalloutRoster(t, writeRiskCalloutFixture(t, "cat >/dev/null\necho 'not json at all'\n"))
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("garbage stdout was not classed")
	}
	if !strings.Contains(reason, "unparseable") {
		t.Fatalf("reason does not name the failure: %q", reason)
	}
}

// TestCalloutTimeout — a callout that never answers within riskCalloutTimeout
// answers classed=true, and the enforcement is REAL: a backgrounded grandchild
// holding the output pipe open must not make Run block past the deadline (the
// WaitDelay regression — see riskcallout.go's package doc comment).
func TestCalloutTimeout(t *testing.T) {
	riskCalloutRoster(t, writeRiskCalloutFixture(t, "cat >/dev/null\nsleep 30 &\nwait $!\n"))
	start := time.Now()
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	elapsed := time.Since(start)
	if !classed {
		t.Fatal("a callout that never answered was not classed")
	}
	if !strings.Contains(reason, "did not answer within") {
		t.Fatalf("reason does not name the timeout: %q", reason)
	}
	if elapsed > riskCalloutTimeout+5*time.Second {
		t.Fatalf("Classify took %s against a %s timeout — the deadline is not enforced against a "+
			"callout whose child holds the output pipe open", elapsed, riskCalloutTimeout)
	}
}

// TestCalloutMissingBinary — a configured path that does not exist on disk
// answers classed=true.
func TestCalloutMissingBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	riskCalloutRoster(t, missing)
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("a missing callout binary was not classed")
	}
	if !strings.Contains(reason, "did not answer") {
		t.Fatalf("reason does not name the failure: %q", reason)
	}
}

// TestCalloutNotExecutable — a configured path that exists but lacks the
// executable bit answers classed=true, same as a missing binary.
func TestCalloutNotExecutable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "callout")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	riskCalloutRoster(t, p)
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("a non-executable callout was not classed")
	}
	if !strings.Contains(reason, "did not answer") {
		t.Fatalf("reason does not name the failure: %q", reason)
	}
}

// TestCalloutGroupWritableRefused — a callout that anything in its group can
// overwrite is refused exactly like a missing binary: whoever can write it
// chooses this gate's verdict, so running it anyway would not be safer than not
// asking.
func TestCalloutGroupWritableRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	p := writeRiskCalloutFixture(t, "cat >/dev/null\necho allow\n")
	if err := os.Chmod(p, 0o775); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	riskCalloutRoster(t, p)
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("a group-writable callout was not classed")
	}
	if !strings.Contains(reason, "did not answer") {
		t.Fatalf("reason does not name the failure: %q", reason)
	}
}

// TestCalloutEmptyPathFailsClosed pins the Task's "empty configured path mid-run"
// failure mode directly against the exec primitive: runRiskCallout must never
// treat an empty path as "nothing to do" — that decision belongs ONLY to
// calloutClassifier.Classify (which reads EffectiveConfig() and abstains on a
// genuinely unset ASSAY_RISK_CALLOUT). Anything lower in the stack that receives
// an empty path treats it as a failure.
func TestCalloutEmptyPathFailsClosed(t *testing.T) {
	if _, err := runRiskCallout("", `{"repo":"x","changedFiles":[]}`); err == nil {
		t.Fatal("runRiskCallout(\"\", …) returned no error — an empty path must fail closed, not silently succeed")
	}
}

// --- Verify row 2: the union only-widens --------------------------------------

// TestUnionOnlyWidensCalloutFalsePatternTrue — a callout answering "not
// classed" can never clear a compiled pattern hit. This is the load-bearing
// only-widens proof: if the union let a callout's false suppress a pattern
// true, this would go green on the wrong verdict.
func TestUnionOnlyWidensCalloutFalsePatternTrue(t *testing.T) {
	riskCalloutRoster(t, riskCalloutTestdataFixture(t))
	t.Setenv("RISKCALLOUT_FIXTURE_ANSWER", "not-classed")

	classed, reason := defaultRiskClassifier.Classify(riskCalloutRepo, []string{compiledTrigger})
	if !classed {
		t.Fatal("a callout answering not-classed CLEARED a compiled pattern hit — only-widens is broken")
	}
	if !strings.Contains(reason, "security path") {
		t.Fatalf("the pattern classifier's own reason did not survive the union: %q", reason)
	}
}

// TestUnionOnlyWidensPatternFalseCalloutTrue is the other half: a callout
// answering "classed" fires even when the pattern classifier found nothing.
func TestUnionOnlyWidensPatternFalseCalloutTrue(t *testing.T) {
	riskCalloutRoster(t, riskCalloutTestdataFixture(t))
	t.Setenv("RISKCALLOUT_FIXTURE_ANSWER", "classed")

	classed, reason := defaultRiskClassifier.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("a callout answering classed did not classify a diff the pattern classifier missed")
	}
	if !strings.Contains(reason, "fixture: adopter policy flagged") {
		t.Fatalf("the callout's own reason did not survive the union: %q", reason)
	}
}

// --- Verify row 3: a nonexistent configured path classes AND echoes loudly ---

// TestCalloutMissingPathEchoesConfiguredValue is Verify row 3: with
// ASSAY_RISK_CALLOUT=/nonexistent, classification answers true (proven above by
// TestCalloutMissingBinary's shape) AND the effective-config echo names the
// exact configured path, never silently substituting "(unset)" — a fail-closed
// gate must be loud about WHICH path it could not consult.
func TestCalloutMissingPathEchoesConfiguredValue(t *testing.T) {
	const nonexistent = "/nonexistent"
	riskCalloutRoster(t, nonexistent)

	classed, _ := defaultRiskClassifier.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("ASSAY_RISK_CALLOUT=/nonexistent did not classify — fail-closed is broken")
	}

	found := false
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		if strings.HasPrefix(l, "assay-config: "+EnvRiskCallout+"=") {
			found = true
			if !strings.Contains(l, nonexistent) {
				t.Fatalf("the %s echo line does not name the configured path: %q", EnvRiskCallout, l)
			}
		}
	}
	if !found {
		t.Fatalf("no %s line in the effective-config echo", EnvRiskCallout)
	}
}

// --- Verify row 4: unset is byte-identical to the unset baseline ---------

// TestCalloutUnsetContributesNothingToUnion — with ASSAY_RISK_CALLOUT unset,
// the union's verdict and reason are IDENTICAL to the bare pattern classifier's,
// for every case the pre-existing riskpath tables exercise. This is the
// behavior-identical proof for brief 02: adding the callout member must not
// move a single verdict until an adopter configures one.
func TestCalloutUnsetContributesNothingToUnion(t *testing.T) {
	riskCalloutRoster(t, "")

	for _, files := range riskReasonFileSets {
		wantClassed, wantReason := patternClassifier{}.Classify(riskCalloutRepo, files)
		gotClassed, gotReason := defaultRiskClassifier.Classify(riskCalloutRepo, files)
		if gotClassed != wantClassed || gotReason != wantReason {
			t.Fatalf("files=%v: union(unset callout) = (%v,%q), pattern alone = (%v,%q) — "+
				"an unset callout must not move the verdict", files, gotClassed, gotReason, wantClassed, wantReason)
		}
	}
}

// TestCalloutClassifierAbstainsWhenUnset pins calloutClassifier's OWN half of
// the contract directly: with no callout configured, it answers (false,
// notRiskClassed) — "nothing to add" — rather than (true, …) or a panic.
func TestCalloutClassifierAbstainsWhenUnset(t *testing.T) {
	riskCalloutRoster(t, "")
	classed, reason := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if classed {
		t.Fatalf("calloutClassifier classified with no callout configured: reason=%q", reason)
	}
	if reason != notRiskClassed {
		t.Fatalf("calloutClassifier's abstain reason = %q, want %q", reason, notRiskClassed)
	}
}

// --- positive control: the union classifies correctly with the callout both
// present and absent, over a range of repos/files -----------------------------

// TestPositiveControlUnionBothStates walks a small matrix proving the union
// reaches the SAME classification decisions RiskPathTriggered always made, in
// both the callout-absent and callout-present-but-clean states — the
// regression a behavior-identical brief exists to rule out.
func TestPositiveControlUnionBothStates(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"compiled trigger fires", []string{compiledTrigger}, true},
		{"clean file does not fire", riskCalloutCleanFile, false},
	}

	t.Run("callout unset", func(t *testing.T) {
		riskCalloutRoster(t, "")
		for _, c := range cases {
			if got := RiskPathTriggered(riskCalloutRepo, c.files); got != c.want {
				t.Fatalf("%s: RiskPathTriggered = %v, want %v", c.name, got, c.want)
			}
		}
	})

	t.Run("callout present and clean", func(t *testing.T) {
		riskCalloutRoster(t, riskCalloutTestdataFixture(t))
		t.Setenv("RISKCALLOUT_FIXTURE_ANSWER", "not-classed")
		for _, c := range cases {
			if got := RiskPathTriggered(riskCalloutRepo, c.files); got != c.want {
				t.Fatalf("%s: RiskPathTriggered = %v, want %v (a clean callout answer must not change "+
					"the pattern classifier's own verdict)", c.name, got, c.want)
			}
		}
	})

	t.Run("callout present and flags everything", func(t *testing.T) {
		riskCalloutRoster(t, riskCalloutTestdataFixture(t))
		t.Setenv("RISKCALLOUT_FIXTURE_ANSWER", "classed")
		for _, c := range cases {
			if !RiskPathTriggered(riskCalloutRepo, c.files) {
				t.Fatalf("%s: RiskPathTriggered = false with a callout flagging everything — only-widens is broken", c.name)
			}
		}
	})
}

// TestCalloutReceivesTheRequestEnvelope pins the wire contract: one JSON object
// on stdin carrying exactly `repo` and `changedFiles`, and the process is
// invoked as `<path> classify` — an adopter's callout is written against this
// shape, so a silent change to either breaks every deployed callout.
func TestCalloutReceivesTheRequestEnvelope(t *testing.T) {
	out := filepath.Join(t.TempDir(), "seen.json")
	seenArgs := filepath.Join(t.TempDir(), "seen-argv")
	c := writeRiskCalloutFixture(t, "printf '%s' \"$1\" > '"+seenArgs+"'\ncat > '"+out+"'\necho '{\"riskClassed\":false,\"reason\":\"ok\"}'\n")
	riskCalloutRoster(t, c)

	files := []string{"a.txt", "b/c.txt"}
	classed, reason := (calloutClassifier{}).Classify(riskCalloutRepo, files)
	if classed {
		t.Fatalf("unexpected classed=true: %s", reason)
	}

	argv, err := os.ReadFile(seenArgs)
	if err != nil || string(argv) != "classify" {
		t.Fatalf("callout was not invoked with the 'classify' argument: %q (err=%v)", argv, err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the callout received no stdin: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"repo":"` + riskCalloutRepo + `"`,
		`"changedFiles":["a.txt","b/c.txt"]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("the request envelope is missing %s:\n%s", want, got)
		}
	}
}

// TestCalloutOutputBounded proves a callout that streams past
// maxRiskCalloutOutput does not blow up this process's own memory footprint —
// it is read, capped, and (since capped output is not valid JSON here) still
// fails closed rather than hanging.
func TestCalloutOutputBounded(t *testing.T) {
	c := writeRiskCalloutFixture(t, "cat >/dev/null\ni=0\nwhile [ $i -lt 4000 ]; do "+
		"printf '0123456789012345678901234567890123456789\\n'; i=$((i+1)); done\n")
	riskCalloutRoster(t, c)
	classed, _ := calloutClassifier{}.Classify(riskCalloutRepo, riskCalloutCleanFile)
	if !classed {
		t.Fatal("streamed (non-JSON) output was not classed")
	}
}

// riskCalloutMaxOutputSanity guards the constant itself against an accidental
// zero, which would make riskCalloutLimitedWriter discard everything.
func TestRiskCalloutMaxOutputPositive(t *testing.T) {
	if maxRiskCalloutOutput <= 0 {
		t.Fatalf("maxRiskCalloutOutput = %s, want > 0", strconv.Itoa(maxRiskCalloutOutput))
	}
}
