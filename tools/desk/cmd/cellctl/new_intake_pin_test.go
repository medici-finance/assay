package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// intakePinLine is the per-role pin every freshly scaffolded cell.env carries for the intake desk:
// the skill reserves its judgment work for a strong tier, and the scaffold's default pin is mid.
const intakePinLine = "DESK_MODEL_intake_desk=opus"

// regression: intake-class-routing/05
func TestSeedIntakeTierStrong(t *testing.T) {
	policy, err := parseProviderCatalog(defaultProviders, "embedded providers.json", "")
	if err != nil {
		t.Fatalf("the embedded seed is not a valid policy: %v", err)
	}
	if len(policy.Providers) == 0 {
		t.Fatal("the seed declares no providers")
	}
	for name, p := range policy.Providers {
		for role, want := range map[string]string{"intake-desk": "strong", "worker-desk": "mid"} {
			got, err := policy.Resolve(role, name, "", "")
			if err != nil {
				t.Fatalf("%s/%s does not resolve: %v", name, role, err)
			}
			if got.Tier != want {
				t.Errorf("%s/%s resolves to tier %q, want %q", name, role, got.Tier, want)
			}
			if wantModel := p.Tiers[want].Model; got.Model != wantModel {
				t.Errorf("%s/%s resolves to model %q, want the %s tier model %q", name, role, got.Model, want, wantModel)
			}
		}
	}
}

// scaffoldedCellEnv scaffolds one cell of each kind `new` supports and returns name -> cell.env.
func scaffoldedCellEnvs(t *testing.T) map[string]string {
	t.Helper()
	repo, cells := houseForgeWorld(t)
	launcher := filepath.Join(t.TempDir(), "launcher")
	must(t, os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	yaml := filepath.Join(t.TempDir(), "cells.yaml")
	must(t, os.WriteFile(yaml, []byte("cells: []\n"), 0o600))
	roots := "o/r=" + repo
	scaffolds := map[string][]string{
		"k8s-github": {"--repo", repo, "--cells-yaml", yaml, "--orgs", "example", "--deskd-app-pem", yaml},
		"k8s-gitlab": {"--forge", "gitlab", "--group", "g", "--repo", repo, "--cells-yaml", yaml},
		"house":      {"--kind", "house", "--repo", repo, "--roots", roots},
		"container":  {"--kind", "container", "--repo", "example-org/example-repo", "--launcher", launcher},
		"scrubbed":   {"--kind", "scrubbed", "--repo", repo, "--repo-slug", "o/r", "--roots", roots},
	}
	out := map[string]string{}
	for name, args := range scaffolds {
		cell := strings.ReplaceAll(name, "_", "-")
		captureStdout(t, func() {
			if code := inproc(append([]string{"new", cell}, args...)...); code != 0 {
				t.Fatalf("new %s exited %d", name, code)
			}
		})
		raw, err := os.ReadFile(filepath.Join(cells, cell, "cell.env"))
		must(t, err)
		out[name] = string(raw)
	}
	return out
}

func countLine(body, line string) int {
	n := 0
	for _, l := range strings.Split(body, "\n") {
		if l == line {
			n++
		}
	}
	return n
}

// regression: intake-class-routing/05
func TestNewPinsIntakeStrong(t *testing.T) {
	for kind, body := range scaffoldedCellEnvs(t) {
		t.Run(kind, func(t *testing.T) {
			if n := countLine(body, intakePinLine); n != 1 {
				t.Fatalf("cell.env carries %q %d times, want exactly once:\n%s", intakePinLine, n, body)
			}
			p := filepath.Join(t.TempDir(), "cell.env")
			must(t, os.WriteFile(p, []byte(body), 0o600))
			c := &Cell{Env: effectiveCellEnv(p, nil)}
			if got := c.resolveRoleModel("intake-desk", "claude"); !got.OK || got.Model != "opus" || got.Src != "DESK_MODEL_intake_desk" {
				t.Errorf("intake-desk resolves to %+v, want opus from DESK_MODEL_intake_desk", got)
			}
			// The pin is the one intake line: another non-coordinator role still takes the default.
			if got := c.resolveRoleModel("worker-desk", "claude"); !got.OK || got.Model != "sonnet" || got.Src != "DESK_MODEL_DEFAULT" {
				t.Errorf("worker-desk resolves to %+v, want sonnet from DESK_MODEL_DEFAULT", got)
			}
		})
	}
}

// regression: intake-class-routing/05
func TestIntakePinSurvivesReset(t *testing.T) {
	late := resetNow.Add(365 * 24 * time.Hour)
	for kind, body := range scaffoldedCellEnvs(t) {
		t.Run(kind, func(t *testing.T) {
			// An aged DEFAULT record makes the reset run its whole plan rather than return early on
			// an empty record; the intake line has no record, which reads as an explicit pin.
			c := resetCell(t, body, map[string]pinRecord{
				"DESK_MODEL_DEFAULT": {Kind: pinKindDefault, Value: "sonnet", At: stamp(30)},
			})
			moves, _, err := c.resetAgedDefaultPins(week, late, true, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range moves {
				if m.Key == "DESK_MODEL_intake_desk" {
					t.Errorf("the reset moved the intake pin: %+v", m)
				}
			}
			if n := countLine(fileOf(t, c), intakePinLine); n != 1 {
				t.Errorf("after the reset cell.env carries %q %d times, want once:\n%s", intakePinLine, n, fileOf(t, c))
			}
			if got := envLine(t, c, "DESK_MODEL_intake_desk"); got != "opus" {
				t.Errorf("after the reset DESK_MODEL_intake_desk = %q, want opus", got)
			}
		})
	}
}

// setNotice runs `cellctl set <cell> <args...>` in process and returns its exit code and what it
// wrote to stdout and stderr.
func setNotice(t *testing.T, cell string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code = inproc(append([]string{"set", cell}, args...)...)
		})
	})
	return code, stdout, stderr
}

func scaffoldHouse(t *testing.T, harness string) (cells, cellDir string) {
	t.Helper()
	repo, cells := houseForgeWorld(t)
	captureStdout(t, func() {
		if code := inproc("new", "h", "--kind", "house", "--repo", repo, "--roots", "o/r="+repo); code != 0 {
			t.Fatalf("new exited %d", code)
		}
	})
	cellDir = filepath.Join(cells, "h")
	if harness != "" {
		code, _, errOut := setNotice(t, "h", "CELL_HARNESS="+harness)
		if code != 0 {
			t.Fatalf("set CELL_HARNESS=%s exited %d: %s", harness, code, errOut)
		}
	}
	return cells, cellDir
}

// regression: intake-class-routing/05
func TestSetShadowedPinNotice(t *testing.T) {
	t.Run("provider defaults", func(t *testing.T) {
		cells, cellDir := scaffoldHouse(t, "")
		shared := filepath.Join(cells, "providers.json")
		// No shared file yet: the cell resolves through the pin, which it reads, so no notice.
		code, out, errOut := setNotice(t, "h", "DESK_MODEL_intake_desk=opus")
		if code != 0 || strings.Contains(errOut, "NOT read") || !strings.Contains(out, "[set] DESK_MODEL_intake_desk:") {
			t.Fatalf("no provider defaults: code=%d stdout=%q stderr=%q; want success and no notice", code, out, errOut)
		}
		must(t, os.WriteFile(shared, defaultProviders, 0o600))
		code, out, errOut = setNotice(t, "h", "DESK_MODEL_intake_desk=sonnet")
		if code != 0 {
			t.Fatalf("exit code %d, want 0; stderr=%q", code, errOut)
		}
		if !strings.Contains(out, "[set] DESK_MODEL_intake_desk: opus -> sonnet") {
			t.Errorf("the write did not happen or lost its success line: %q", out)
		}
		if got := envLine(t, &Cell{Dir: cellDir}, "DESK_MODEL_intake_desk"); got != "sonnet" {
			t.Errorf("cell.env holds %q, want the written sonnet", got)
		}
		for _, want := range []string{"DESK_MODEL_intake_desk", "NOT read", shared, filepath.Join(cellDir, "providers.json")} {
			if !strings.Contains(errOut, want) {
				t.Errorf("the notice lacks %q:\n%s", want, errOut)
			}
		}
		// A per-cell override with one key moves the role to the strong tier model.
		must(t, os.WriteFile(filepath.Join(cellDir, "providers.json"),
			[]byte(`{"providers":{"anthropic":{"desks":{"intake-desk":{"tier":"strong"}}}}}`), 0o600))
		c := loadCell("h")
		policy, _, err := c.cellModelPolicy()
		if err != nil || policy == nil {
			t.Fatalf("cellModelPolicy = %v, %v", policy, err)
		}
		got, err := policy.Resolve("intake-desk", "", "", "")
		if err != nil || got.Tier != "strong" || got.Model != policy.Providers["anthropic"].Tiers["strong"].Model {
			t.Errorf("with the override intake resolves to %+v (%v), want the strong tier model", got, err)
		}
	})

	t.Run("harness namespace", func(t *testing.T) {
		_, cellDir := scaffoldHouse(t, "codex")
		code, out, errOut := setNotice(t, "h", "DESK_MODEL_intake_desk=opus")
		if code != 0 || !strings.Contains(out, "[set] DESK_MODEL_intake_desk:") {
			t.Fatalf("code=%d stdout=%q stderr=%q; want a normal write", code, out, errOut)
		}
		if got := envLine(t, &Cell{Dir: cellDir}, "DESK_MODEL_intake_desk"); got != "opus" {
			t.Errorf("cell.env holds %q, want the written opus", got)
		}
		for _, want := range []string{"DESK_MODEL_intake_desk", "NOT read", "codex", "CODEX_MODEL_"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("the notice lacks %q:\n%s", want, errOut)
			}
		}
		// The class is any pin in a namespace the harness does not read, not the one key above.
		code, _, errOut = setNotice(t, "h", "CURSOR_MODEL_intake_desk=some-model")
		if code != 0 || !strings.Contains(errOut, "CURSOR_MODEL_intake_desk") || !strings.Contains(errOut, "NOT read") {
			t.Errorf("a cursor pin on a codex cell: code=%d stderr=%q; want a notice naming it", code, errOut)
		}
		// The role-sugar call that names --harness picks the namespace on purpose: no notice.
		code, _, errOut = setNotice(t, "h", "worker-desk", "--harness", "claude", "--model", "sonnet")
		if code != 0 || strings.Contains(errOut, "NOT read") || strings.Contains(errOut, "could not check") {
			t.Errorf("role sugar naming --harness: code=%d stderr=%q; want success and no notice", code, errOut)
		}
		// The pin the codex harness does read gets no notice.
		code, _, errOut = setNotice(t, "h", "CODEX_MODEL_intake_desk=gpt-5.6-terra")
		if code != 0 || strings.Contains(errOut, "NOT read") || strings.Contains(errOut, "could not check") {
			t.Errorf("a pin the cell reads: code=%d stderr=%q; want success and no notice", code, errOut)
		}
	})
}
