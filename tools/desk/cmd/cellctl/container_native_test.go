package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestBinaryNativeContainerRegistrationAndMigration(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "containers.json")
	cells := filepath.Join(root, "cells")
	cfg := cellcontainer.Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]cellcontainer.Cell{"sample": {Repo: "example-org/example-repo", Incoming: filepath.Join(root, "incoming"), Roles: map[string]cellcontainer.Role{"desk": {Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: "sample-desk", Config: filepath.Join(root, "config"), AppKey: filepath.Join(root, "key.pem"), ClaudeToken: filepath.Join(root, "model-token")}}}}}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(config, b, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(wantOK bool, args ...string) string {
		t.Helper()
		cmd := exec.Command(cellctlBinary(t), args...)
		cmd.Env = append(os.Environ(), "CELLS_ROOT="+cells, "DRY_RUN=1", "CELL_CONTAINER_CONFIG=", "CELL_CONTAINER_LAUNCHER=", "CELL_MODEL_POLICY=", "CELL_PROVIDER=")
		b, err := cmd.CombinedOutput()
		if (err == nil) != wantOK {
			t.Fatalf("%v: %v\n%s", args, err, b)
		}
		return string(b)
	}
	run(true, "new", "sample", "--kind", "container", "--container-config", config)
	out := run(true, "up", "sample")
	for _, s := range []string{"native container desk", "harness=codex model=test-model", "--pull never"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %s: %s", s, out)
		}
	}
	out = run(true, "up", "sample", "--model", "override-model")
	if !strings.Contains(out, "CELL_MODEL=override-model") {
		t.Fatal(out)
	}
	out = run(false, "container-run", "sample", "the-desk", "claude", "opus")
	if !strings.Contains(out, "Opus") && !strings.Contains(out, "opus") {
		t.Fatal(out)
	}
	run(true, "status", "sample")
	run(true, "down", "sample")
	// Native mode must never silently fall back to an external executable.
	run(true, "set", "sample", "CELL_CONTAINER_LAUNCHER=/bin/echo")
	run(false, "up", "sample")
	run(true, "set", "sample", "CELL_KIND=container", "CELL_CONTAINER_LAUNCHER=", "CELL_CONTAINER_CONFIG="+config)
	run(true, "up", "sample")
}
func TestNativeContainerLockRejectsActiveHostSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = deskkit.TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
	assertDies(t, "active host lock", func() { f := containerLock(path); f.Close() })
}

// nativeFixture registers a native cell with a coordinator and a worker role and returns its
// loaded Cell and container configuration.
func nativeFixture(t *testing.T, roles string) (*Cell, *cellcontainer.Config) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"config", "incoming"} {
		if err := os.Mkdir(filepath.Join(root, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(root, "key.pem")
	if err := os.WriteFile(key, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	role := func(volume string) cellcontainer.Role {
		return cellcontainer.Role{Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: volume, Config: filepath.Join(root, "config"), AppKey: key}
	}
	cfg := cellcontainer.Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]cellcontainer.Cell{"sample": {Repo: "example-org/example-repo", Incoming: filepath.Join(root, "incoming"), Roles: map[string]cellcontainer.Role{"desk": role("sample-desk"), "worker": role("sample-worker")}}}}
	b, _ := json.Marshal(cfg)
	config := filepath.Join(root, "containers.json")
	if err := os.WriteFile(config, b, 0600); err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(root, "cells")
	t.Setenv("CELLS_ROOT", cells)
	for _, k := range []string{"DRY_RUN", "CELL_CONTAINER_CONFIG", "CELL_CONTAINER_LAUNCHER", "CELL_MODEL_POLICY", "CELL_PROVIDER"} {
		t.Setenv(k, "")
	}
	newNativeContainer(cells, "sample", "example-org/example-repo", config, roles, "")
	loaded, err := cellcontainer.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	return loadCell("sample"), loaded
}

// runningInspection is Docker's view of a container started from p.
func runningInspection(id string, p *cellcontainer.Plan) map[string]any {
	env := []string{}
	for k, v := range p.Env {
		env = append(env, k+"="+v)
	}
	var mounts []map[string]any
	for _, m := range p.Mounts {
		mounts = append(mounts, map[string]any{"Type": m.Type, "Name": m.Source, "Source": m.Source, "Destination": m.Target, "RW": !m.ReadOnly})
	}
	return map[string]any{
		"Id": id, "Name": "/" + p.Name, "Image": p.Image,
		"State":  map[string]any{"Status": "running", "Running": true},
		"Config": map[string]any{"User": "501:501", "Tty": true, "OpenStdin": true, "Env": env, "Cmd": []string{p.Action}, "Labels": map[string]string{"io.assay.cell": p.CellName, "io.assay.role": p.RoleKey}},
		"HostConfig": map[string]any{"ReadonlyRootfs": true, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges"}, "PidsLimit": 256, "Memory": 4 << 30, "NanoCpus": 2e9,
			"NetworkMode": p.Network, "Tmpfs": map[string]string{"/tmp": "rw,nosuid,nodev,size=512m,mode=1777"}},
		"Mounts":          mounts,
		"NetworkSettings": map[string]any{"Networks": map[string]any{p.Network: map[string]any{}}},
	}
}

// fakeDocker answers container listing/inspection by container name and records stops.
func fakeDocker(t *testing.T, running map[string]map[string]any, stopped *[]string) func(string) cellcontainer.Engine {
	t.Helper()
	return func(string) cellcontainer.Engine {
		byID := map[string]map[string]any{}
		for _, s := range running {
			byID[s["Id"].(string)] = s
		}
		return cellcontainer.Engine{Output: func(a ...string) ([]byte, error) {
			switch {
			case a[0] == "container" && a[1] == "ls":
				name := strings.TrimSuffix(strings.TrimPrefix(a[5], "name=^/"), "$")
				if s, ok := running[name]; ok {
					return []byte(s["Id"].(string)), nil
				}
				return nil, nil
			case a[0] == "container" && a[1] == "inspect":
				return json.Marshal([]any{byID[a[2]]})
			case a[0] == "image" && a[1] == "inspect":
				return []byte(`[{"Config":{}}]`), nil
			case a[0] == "stop":
				*stopped = append(*stopped, a[1])
				return nil, nil
			}
			t.Fatalf("unexpected docker call %v", a)
			return nil, nil
		}}
	}
}

// A container launched with desk --model is stopped by down even though the registered pin
// differs, and a refused role never prevents the remaining roles from being handled.
func TestNativeDownHandlesOverrideLaunchAndContinuesPastRefusal(t *testing.T) {
	c, cfg := nativeFixture(t, "worker-desk the-desk")
	desk, err := cfg.Plan("sample", "desk", "", "override-model")
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cfg.Plan("sample", "worker", "", "")
	if err != nil {
		t.Fatal(err)
	}
	deskID, workerID := strings.Repeat("d", 64), strings.Repeat("e", 64)
	foreign := runningInspection(workerID, worker)
	foreign["Config"].(map[string]any)["Labels"] = map[string]string{"io.assay.cell": "other", "io.assay.role": "worker"}
	var stopped []string
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = fakeDocker(t, map[string]map[string]any{desk.Name: runningInspection(deskID, desk), worker.Name: foreign}, &stopped)

	assertDies(t, "down with an unverified worker container", func() { c.nativeContainer("down") })
	if len(stopped) != 1 || stopped[0] != deskID {
		t.Fatalf("stopped %v, want only the override-launched coordinator %s", stopped, deskID)
	}
}

func TestNativeStatusReportsOverrideLaunch(t *testing.T) {
	c, cfg := nativeFixture(t, "the-desk")
	desk, err := cfg.Plan("sample", "desk", "", "override-model")
	if err != nil {
		t.Fatal(err)
	}
	var stopped []string
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = fakeDocker(t, map[string]map[string]any{desk.Name: runningInspection(strings.Repeat("d", 64), desk)}, &stopped)
	c.nativeContainer("status")
	if len(stopped) != 0 {
		t.Fatalf("status stopped %v", stopped)
	}
}

// The console command reaches tmux as separate arguments, so no shell parses a model value.
func TestNativeConsoleArgvHasNoShellString(t *testing.T) {
	c := &Cell{Name: "sample", Dir: "/cells/sample"}
	model := "model'; echo injected; #"
	argv := c.nativeConsoleArgv(&cellcontainer.Plan{Harness: "codex", Model: model}, "the-desk")
	want := []string{"env", "CELLS_ROOT=/cells", selfPath(), "container-run", "sample", "the-desk", "codex", model}
	if strings.Join(argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("got %q want %q", argv, want)
	}
}

// The external launcher contract has no status verb; only the native runtime answers it.
func TestBinaryExternalLauncherStatusRefused(t *testing.T) {
	root := t.TempDir()
	cells := filepath.Join(root, "cells")
	marker := filepath.Join(root, "launcher-ran")
	launcher := filepath.Join(root, "launcher")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\necho \"$@\" > "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command(cellctlBinary(t), args...)
		cmd.Env = append(os.Environ(), "CELLS_ROOT="+cells, "DRY_RUN=", "CELL_CONTAINER_CONFIG=", "CELL_CONTAINER_LAUNCHER=", "CELL_MODEL_POLICY=", "CELL_PROVIDER=")
		return cmd.CombinedOutput()
	}
	if out, err := run("new", "sample", "--kind", "container", "--repo", "example-org/example-repo", "--launcher", launcher); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	out, err := run("status", "sample")
	if err == nil {
		t.Fatalf("status reached the external launcher: %s", out)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("external launcher was invoked for status")
	}
}

// A launch plan protects the cell directory (cell.env, the console socket): a directory mount
// that is, or contains, the cell directory or the cells root is refused before launch.
func TestNativePlanProtectsCellDirectory(t *testing.T) {
	c, _ := nativeFixture(t, "the-desk")
	for _, dir := range []string{c.Dir, filepath.Dir(c.Dir)} {
		p := c.nativeContainerPlan("the-desk", "", "")
		if err := p.CheckFiles(); err != nil {
			t.Fatalf("unchanged plan refused: %v", err)
		}
		p.Mounts[2].Source = dir
		if err := p.CheckFiles(); err == nil || !strings.Contains(err.Error(), "contains a protected path") {
			t.Fatalf("mount of %s accepted: %v", dir, err)
		}
	}
}

// failingDocker wraps fakeDocker so that every call whose first argument is verb fails.
func failingDocker(next func(string) cellcontainer.Engine, verb string) func(string) cellcontainer.Engine {
	return func(host string) cellcontainer.Engine {
		e := next(host)
		inner := e.Output
		e.Output = func(a ...string) ([]byte, error) {
			if a[0] == verb {
				return nil, errors.New(verb + " failed")
			}
			return inner(a...)
		}
		return e
	}
}

// down refuses (and exits nonzero) when stopping a verified container fails.
func TestNativeDownRefusesFailedStop(t *testing.T) {
	c, cfg := nativeFixture(t, "the-desk")
	desk, err := cfg.Plan("sample", "desk", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var stopped []string
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = failingDocker(fakeDocker(t, map[string]map[string]any{desk.Name: runningInspection(strings.Repeat("d", 64), desk)}, &stopped), "stop")
	assertDies(t, "down with a failed stop", func() { c.nativeContainer("down") })
}

// A role that cannot be resolved is reported, and the remaining roles are still handled.
func TestNativeDownContinuesPastUnconfiguredRole(t *testing.T) {
	c, cfg := nativeFixture(t, "the-desk")
	desk, err := cfg.Plan("sample", "desk", "", "")
	if err != nil {
		t.Fatal(err)
	}
	c.Roles = []string{"verify-desk", "the-desk"}
	deskID := strings.Repeat("d", 64)
	var stopped []string
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = fakeDocker(t, map[string]map[string]any{desk.Name: runningInspection(deskID, desk)}, &stopped)
	assertDies(t, "down with an unconfigured role", func() { c.nativeContainer("down") })
	if len(stopped) != 1 || stopped[0] != deskID {
		t.Fatalf("stopped %v, want the configured coordinator %s", stopped, deskID)
	}
}

// check exits nonzero when any role's preflight fails.
func TestNativeCheckExitsNonzeroOnFailure(t *testing.T) {
	c, _ := nativeFixture(t, "the-desk")
	var stopped []string
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = failingDocker(fakeDocker(t, nil, &stopped), "image")
	assertDies(t, "check with a missing image", func() { c.nativeContainer("check") })
}
