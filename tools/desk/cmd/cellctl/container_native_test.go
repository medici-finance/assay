package main

import (
	"encoding/json"
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
	cfg := cellcontainer.Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]cellcontainer.Cell{"sample": {Repo: "example-org/example-repo", Incoming: filepath.Join(root, "incoming"), Roles: map[string]cellcontainer.Role{"desk": {Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: "sample-desk", Config: filepath.Join(root, "config"), AppKey: filepath.Join(root, "key.pem")}}}}}
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
