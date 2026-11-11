//go:build pirunner

package pirunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The shared mutation gate (loopadmin/mutate) runs untagged tests, so against
// this package it would select nothing and read every mutation as not killed.
// This test-owned runner copies the module with its relative perfrec
// dependency, links the pinned node_modules, applies one replacement,
// rebuilds the bridge in the copy and runs the named test with the tag.

type negMutation struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Old    string `json:"old"`
	New    string `json:"new"`
	Run    string `json:"run"`
	Assert string `json:"assert"`
}

type negMap struct {
	Baseline  string        `json:"baseline_run"`
	Mutations []negMutation `json:"mutations"`
}

// Verdicts. Only "killed" counts; everything else that is not a clean pass
// is invalid and fails the control.
const (
	vKilled        = "killed"
	vSurvived      = "survived"
	vBuild         = "invalid:build"
	vTimeout       = "invalid:timeout"
	vSkip          = "invalid:skip"
	vZeroSelected  = "invalid:zero-selected"
	vWrongAssert   = "invalid:wrong-assertion"
	vMissingTarget = "invalid:no-match"
)

var testLine = regexp.MustCompile(`(?m)^\s*--- (PASS|FAIL|SKIP): (\S+)`)

// classify reads one tagged go test run. top is the top-level test the run
// pattern selects.
func classify(out string, timedOut bool, top, assert string) string {
	switch {
	case timedOut || strings.Contains(out, "panic: test timed out"):
		return vTimeout
	case strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]") || strings.Contains(out, "build-bridge-failed"):
		return vBuild
	case strings.Contains(out, "--- SKIP"):
		return vSkip
	case strings.Contains(out, "no tests to run") || !hasResult(out, top):
		return vZeroSelected
	}
	for _, m := range testLine.FindAllStringSubmatch(out, -1) {
		if m[1] == "FAIL" && m[2] == top {
			if assert != "" && !strings.Contains(out, assert) {
				return vWrongAssert
			}
			return vKilled
		}
	}
	return vSurvived
}

func hasResult(out, top string) bool {
	for _, m := range testLine.FindAllStringSubmatch(out, -1) {
		if m[2] == top {
			return true
		}
	}
	return false
}

func topTest(run string) string {
	first := strings.SplitN(run, "/", 2)[0]
	return strings.Trim(first, "^$")
}

func TestPiPilotNegativeControls(t *testing.T) {
	t.Run("Classifier", func(t *testing.T) {
		const top = "TestPiPilotX"
		for _, c := range []struct {
			name, out string
			timeout   bool
			want      string
		}{
			{"killed", "--- FAIL: TestPiPilotX (0.1s)\n    x_test.go:1: the assertion\nFAIL\n", false, vKilled},
			{"survived", "--- PASS: TestPiPilotX (0.1s)\nok\n", false, vSurvived},
			{"compile", "# pkg\nx.go:1: undefined: y\nFAIL\tpkg [build failed]\n", false, vBuild},
			{"bridge-build", "build-bridge-failed: tsc error\n", false, vBuild},
			{"skip", "--- SKIP: TestPiPilotX (0.0s)\n--- FAIL: TestPiPilotX\n", false, vSkip},
			{"zero", "testing: warning: no tests to run\nok\n", false, vZeroSelected},
			{"other-test", "--- FAIL: TestOther (0.1s)\nFAIL\n", false, vZeroSelected},
			{"timeout", "", true, vTimeout},
			{"panic-timeout", "panic: test timed out after 1s\n--- FAIL: TestPiPilotX\n", false, vTimeout},
			{"wrong-assert", "--- FAIL: TestPiPilotX (0.1s)\n    x_test.go:1: something else\n", false, vWrongAssert},
		} {
			if got := classify(c.out, c.timeout, top, "the assertion"); got != c.want {
				t.Errorf("%s: %s, want %s", c.name, got, c.want)
			}
		}
	})

	t.Run("Mutations", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pirunner", "negative-controls.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m negMap
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if len(m.Mutations) < 5 {
			t.Fatalf("map has %d mutations; the brief names five required controls", len(m.Mutations))
		}
		base := runMutated(t, nil, m.Baseline)
		for _, top := range []string{"TestPiPilotRestart", "TestPiPilotEffects", "TestPiPilotAuthority", "TestPiPilotStopAndObserve", "TestPiPilotAccounting"} {
			if v := classify(base.out, base.timedOut, top, ""); v != vSurvived {
				t.Fatalf("baseline %s: %s, want a clean pass\n%s", top, v, base.out)
			}
		}
		for _, mu := range m.Mutations {
			r := runMutated(t, &mu, mu.Run)
			v := r.verdict
			if v == "" {
				v = classify(r.out, r.timedOut, topTest(mu.Run), mu.Assert)
			}
			t.Logf("negative-control %s: %s (%.1fs)", mu.Name, v, r.secs)
			if v != vKilled {
				t.Errorf("mutation %s not killed: %s\n%s", mu.Name, v, tail(r.out, 3000))
			}
		}
	})
}

type mutRun struct {
	out      string
	timedOut bool
	verdict  string
	secs     float64
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func runMutated(t *testing.T, mu *negMutation, run string) mutRun {
	t.Helper()
	t0 := time.Now()
	mod, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	copyTree(t, mod, filepath.Join(root, "loopadmin"))
	copyTree(t, filepath.Join(mod, "..", "perfrec"), filepath.Join(root, "perfrec"))
	nodeDir := filepath.Join(root, "loopadmin", "experiments", "pirunner", "node")
	if err := os.Symlink(filepath.Join(mod, "experiments", "pirunner", "node", "node_modules"), filepath.Join(nodeDir, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if mu != nil {
		p := filepath.Join(root, "loopadmin", filepath.FromSlash(mu.File))
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(src), mu.Old); n != 1 {
			return mutRun{verdict: vMissingTarget, out: fmt.Sprintf("old text occurs %d times in %s", n, mu.File)}
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(src), mu.Old, mu.New, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	node, _ := bridgeScript(t)
	tsc := exec.CommandContext(ctx, node, filepath.Join("node_modules", "typescript", "bin", "tsc"), "-p", "tsconfig.json")
	tsc.Dir = nodeDir
	if out, err := tsc.CombinedOutput(); err != nil {
		return mutRun{out: "build-bridge-failed: " + string(out), secs: time.Since(t0).Seconds()}
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "pirunner", "-count=1", "-v", "-timeout", "240s", "-run", run, "./experiments/pirunner")
	cmd.Dir = filepath.Join(root, "loopadmin")
	cmd.Env = append(os.Environ(), "GOWORK=off", "KUBECONFIG=/dev/null")
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err = cmd.Run()
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	return mutRun{out: buf.String(), timedOut: timedOut, secs: time.Since(t0).Seconds()}
}

// copyTree copies a module tree, leaving out VCS data, installed packages
// and build output (the copy rebuilds its own bridge).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "dist") {
			return filepath.SkipDir
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
