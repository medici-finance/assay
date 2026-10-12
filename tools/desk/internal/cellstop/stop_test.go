//go:build darwin || linux

package cellstop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("CELL_STOP_ORPHAN_PARENT") == "1" {
		binary, _ := os.Executable()
		cmd := exec.Command(binary)
		env := []string{}
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, "CELL_STOP_ORPHAN_PARENT=") {
				env = append(env, kv)
			}
		}
		cmd.Env = append(env, "CELL_STOP_FIXTURE=1")
		if err := cmd.Start(); err != nil {
			os.Exit(2)
		}
		fmt.Println(cmd.Process.Pid)
		cmd.Process.Release()
		os.Exit(0)
	}
	if os.Getenv("CELL_STOP_FIXTURE") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestStopOnlyAttributableConsumers(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache with spaces")
	start := func(path string) *exec.Cmd {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary)
		cmd.Env = append(os.Environ(), "GOCACHE="+path, "CELL_STOP_FIXTURE=1")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
		return cmd
	}
	owned := start(filepath.Join(cache, "build"))
	foreign := start(filepath.Join(root, "foreign", "build"))
	deadline := time.Now().Add(time.Second)
	var env map[string]string
	for {
		var err error
		env, err = processEnvironment(owned.Process.Pid)
		if err != nil {
			t.Fatal("fixture attribution unavailable", err)
		}
		if env["GOCACHE"] == filepath.Join(cache, "build") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture cache attribution missing; relevant keys=%v", env)
		}
		time.Sleep(10 * time.Millisecond)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(binary)
	parent.Env = append(os.Environ(), "GOCACHE="+filepath.Join(cache, "build"), "CELL_STOP_ORPHAN_PARENT=1")
	raw, err := parent.Output()
	if err != nil {
		t.Fatal(err)
	}
	orphanPID, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	orphanEnv, err := processEnvironment(orphanPID)
	if err != nil || orphanEnv["GOCACHE"] != filepath.Join(cache, "build") {
		t.Fatal("orphan attribution", err)
	}
	t.Cleanup(func() {
		if all, e := inventory(); e == nil {
			if p, ok := all[orphanPID]; ok && within(p.env["GOCACHE"], cache) {
				terminate(p, true)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Stop(ctx, Scope{Cell: "fixture", Directory: root, CacheRoot: cache}); err != nil {
		t.Fatal(err)
	}
	all, err := inventory()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all[owned.Process.Pid]; ok {
		t.Fatal("owned consumer still running after stop")
	}
	if _, ok := all[orphanPID]; ok {
		t.Fatal("detached orphan still running after stop")
	}
	if _, ok := all[foreign.Process.Pid]; !ok {
		t.Fatal("unrelated process was stopped")
	}
}
func TestScopeExactDirectoryBoundary(t *testing.T) {
	s := Scope{Cell: "fixture", Directory: "/cells/fixture", CacheRoot: "/cells/fixture/cache"}
	if s.owns(process{env: map[string]string{"GOCACHE": "/cells/fixture/cache-other/build"}}) {
		t.Fatal("prefix neighbor attributed")
	}
}
