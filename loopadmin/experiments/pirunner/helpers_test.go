//go:build pirunner

package pirunner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
)

// With the tag set, a missing Node toolchain or an unbuilt bridge is a hard
// failure, never a skip: the brief's rows forbid any SKIP line.
func bridgeScript(t testing.TB) (node, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("pirunner: node not on PATH: %v", err)
	}
	script, err = filepath.Abs(filepath.Join("node", "dist", "bridge.js"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("pirunner: bridge not built (npm --prefix loopadmin/experiments/pirunner/node run build): %v", err)
	}
	return node, script
}

func newHost(t testing.TB) *Host {
	t.Helper()
	node, script := bridgeScript(t)
	h, err := OpenHost(t.TempDir(), node, script)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Logf("close: %v", err)
		}
		if t.Failed() {
			t.Logf("bridge stderr:\n%s", h.Stderr())
		}
	})
	return h
}

func subject(t *testing.T) conformance.Subject {
	a := &Adapter{H: newHost(t)}
	return conformance.Subject{Adapter: a, Control: a}
}

func pilotReq(caller, id string, gen uint64) runner.LaunchRequest {
	req := conformance.StandingRequest(id)
	if caller == "controller" {
		req = conformance.WorkflowRequest(id)
	}
	req.Authority.Generation = gen
	return req
}

func ctxT(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func held(h *Host, tool string) func() bool {
	return func() bool { return h.Counters.Snapshot().ToolHeld[tool] > 0 }
}
