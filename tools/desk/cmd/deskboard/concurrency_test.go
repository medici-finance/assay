package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPolicyDriftNeverFailsTheRun pins the ONE caller of the shared pool that is deliberately
// NOT fail-closed.
//
// The drift probe rides inside `actions` — the desk's primary read — and its contract, stated
// in cmdPolicyDrift's own doc comment, is that a repo whose visibility could not be read is
// left OUT of the observed map, where VisibilityDrift reports it NOT OBSERVED. That is still
// loud (an unchecked repo IS drift), and it is the whole reason the probe must not kill the
// board: turning one repo's unreadable metadata into a dead `actions` is the false economy
// #295's branch-health probe already rejected.
//
// Moving the probe onto the pool is exactly where that contract could be lost without anyone
// noticing, because the pool's default is fail-closed. So this drives a repo's visibility read
// to FAIL and asserts two things the mutation "make the policy-drift probe fail-closed" would
// break: the call still returns, and the unreadable repo is reported as drift rather than as a
// pass.
func TestPolicyDriftNeverFailsTheRun(t *testing.T) {
	installFakeForge(t)
	// Every repo's configured visibility is served correctly EXCEPT one, whose read is made
	// to fail — the state the probe must survive.
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
	scope := deskkit.AllowedRepos()
	if len(scope) < 2 {
		t.Skip("the configured roster has fewer than two repos; this control needs one to fail and one not to")
	}
	failing := scope[0]
	t.Setenv("DESKBOARD_GH_FAIL_REPO", failing)

	// If the probe had been made fail-closed there would be nothing to return here at all.
	alarm := assessPolicyDrift()

	if len(alarm.Scope) != len(scope) {
		t.Fatalf("scope = %v, want every configured repo (%d)", alarm.Scope, len(scope))
	}
	// The repo whose visibility could not be read must appear as drift — NOT OBSERVED is
	// drift, never a guessed pass.
	joined := strings.Join(alarm.Drift, " ")
	if !strings.Contains(joined, failing) {
		t.Errorf("the repo whose visibility read FAILED (%s) is not reported as drift: %v\n"+
			"an unchecked repo is drift, never a pass", failing, alarm.Drift)
	}
	// And a repo that WAS read must not be dragged into the drift list with it: the probe is
	// per-repo, and a pooled sweep that lost one worker's result would show up here.
	for _, r := range scope[1:] {
		if strings.Contains(joined, r) {
			t.Errorf("a repo whose visibility WAS read (%s) is reported as drift: %v", r, alarm.Drift)
		}
	}
}

// TestPerRootPoolFailsClosed is the fail-closed control for the per-root pool.
//
// `dispatch` and `awaiting` read their roots concurrently now. A root that could not be read
// must fail the WHOLE verb — a queue silently missing one root's briefs is a board that says
// "nothing to dispatch here" about work it never looked at, and it is indistinguishable from a
// genuinely drained root. The mutation "drop the per-root pool's fail-closed propagation" is
// what this catches.
func TestPerRootPoolFailsClosed(t *testing.T) {
	for _, verb := range []string{"dispatch", "awaiting"} {
		t.Run(verb, func(t *testing.T) {
			installFakeStatusgen(t)
			tracker, _ := twoRoots(t)
			// Make the statusgen invocation for ONE root fail. The shim matches on the
			// --root path, so this fails exactly one of the two.
			t.Setenv("FAKE_SG_FAIL_MATCH", tracker)

			var out, errb bytes.Buffer
			code := run([]string{verb}, &out, &errb)
			if code == deskkit.ExitOK {
				t.Fatalf("%s exited 0 with one root unreadable — a partial queue was reported as the queue\nstdout=%s", verb, out.String())
			}
			if !strings.Contains(errb.String(), tracker) && !strings.Contains(out.String(), tracker) {
				t.Errorf("%s failed but did not name the root that could not be read\nstderr=%s", verb, errb.String())
			}
		})
	}
}

// TestThroughputSharedRootFailureBlindsBothStages is the blind-stage control for the shared
// root resolution.
//
// throughput.go's own rule is BLIND IS NEVER GREEN: a stage whose depth could not be read is
// could-not-check and is excluded from bottleneck selection, never counted as zero. Sharing
// the root resolution between the dispatch and verify depths creates exactly one new way to
// break that — a single failure now feeds TWO stages, and blinding only one of them would
// leave a reader believing the other's depth was real. The mutation "let a failed shared root
// resolution blind only ONE of the two stages it fed" is what this catches.
func TestThroughputSharedRootFailureBlindsBothStages(t *testing.T) {
	installFakeForge(t)
	installFakeStatusgen(t)
	// An UNRESOLVABLE root configuration: the shared resolution fails before either depth
	// reader is reached.
	t.Setenv(deskkit.RootsEnv, "example-org/tracker=/nonexistent/root/for/this/test")

	var out, errb bytes.Buffer
	_ = run([]string{"throughput", "--json"}, &out, &errb)

	var rep struct {
		Stages []struct {
			Stage string `json:"stage"`
			Depth *int   `json:"depth"`
			Blind string `json:"blind"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("throughput --json is unparseable: %v\n%s%s", err, out.String(), errb.String())
	}
	seen := map[string]bool{}
	for _, st := range rep.Stages {
		seen[st.Stage] = true
		if st.Stage != "dispatch" && st.Stage != "verify" {
			continue
		}
		// A stage whose depth was never read must be BLIND — never a measured number, and
		// above all never a zero, which would read as a drained queue and steer the desk to
		// widen some other loop.
		if st.Depth != nil {
			t.Errorf("the %s stage reports depth %d although the read that would have produced it "+
				"never ran — blind is never a counted number", st.Stage, *st.Depth)
		}
		if !strings.Contains(st.Blind, "shared stream-root resolution failed") {
			t.Errorf("the %s stage is blind but does not name the shared resolution as what failed: %q\n"+
				"a reader must not be left thinking the other stage's depth is real because this one's was",
				st.Stage, st.Blind)
		}
	}
	for _, stage := range []string{"dispatch", "verify"} {
		if !seen[stage] {
			t.Errorf("the %s stage is missing from the report entirely — a stage omitted from the "+
				"list reads as a stage with no queue", stage)
		}
	}
}
