//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestStoragePreclaimRecovery(t *testing.T) {
	cell, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := cellcache.Resolve(cell, func(k string) string {
		switch k {
		case "CELL_GO_CACHE":
			return "on"
		case "CELL_GO_CACHE_BYTES":
			return "1"
		case "CELL_GO_CACHE_MIN_FREE":
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := cellcache.Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "build", "full"), []byte("over-budget"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kv := range p.Env() {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	// Invalid options prove storage holds before validation, credentials, claims,
	// prompts or worktrees. No task state exists to duplicate on a later retry.
	for _, kit := range []string{"worker", "worker-objective", "review", "verifier"} {
		err = dispatch(dispatchOpts{kit: kit})
		var held *cellcache.Deferred
		if !errors.As(err, &held) || deskkit.ExitCodeOf(err) != 6 || held.Report.Outcome != "storage-deferred" {
			t.Fatalf("%s bypassed preclaim hold: %v", kit, err)
		}
	}
	if err = lease.Finish(true); err != nil {
		t.Fatal(err)
	}
	if err = storageAdmission(false, ""); err != nil {
		t.Fatal("space recovery did not admit same operation", err)
	}
	err = dispatch(dispatchOpts{})
	var held *cellcache.Deferred
	if err == nil || errors.As(err, &held) {
		t.Fatalf("recovery bypassed normal dispatch validation: %v", err)
	}
}

func TestStorageVolumes(t *testing.T) {
	for _, dry := range []bool{false, true} {
		for _, failing := range []string{"checkout", os.TempDir()} {
			calls := 0
			err := storageVolumes("checkout", 100, dry, func(path string) (uint64, error) {
				calls++
				if path == failing {
					return 99, nil
				}
				return 100, nil
			})
			if err == nil || !strings.Contains(err.Error(), failing) || deskkit.ExitCodeOf(err) != 6 {
				t.Fatalf("dry=%v failing=%s: %v", dry, failing, err)
			}
			if failing == os.TempDir() && calls != 2 {
				t.Fatalf("temp volume not checked: %d", calls)
			}
		}
	}
	if err := storageVolumes("checkout", 100, false, func(string) (uint64, error) { return 100, nil }); err != nil {
		t.Fatal(err)
	}
	if err := storageVolumes("checkout", 100, false, func(string) (uint64, error) { return 0, errors.New("unreadable") }); err == nil {
		t.Fatal("unreadable volume admitted")
	}
	if err := storageVolumes("checkout", 0, false, func(string) (uint64, error) { t.Fatal("disabled floor measured"); return 0, nil }); err != nil {
		t.Fatal(err)
	}
}
func TestStorageFloorWithoutManagedCaches(t *testing.T) {
	t.Setenv(cellcache.EnvPolicy, "")
	t.Setenv("DESK_MIN_FREE_BYTES", "18446744073709551615")
	for _, kit := range []string{"worker", "worker-objective", "review", "verifier"} {
		err := dispatch(dispatchOpts{kit: kit, root: t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "storage-deferred") || deskkit.ExitCodeOf(err) != 6 {
			t.Fatalf("%s: %v", kit, err)
		}
	}
	t.Setenv("DESK_MIN_FREE_BYTES", "invalid")
	if err := storageAdmission(false, ""); err == nil {
		t.Fatal("invalid floor admitted")
	}
}
