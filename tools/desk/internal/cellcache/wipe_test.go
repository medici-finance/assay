//go:build darwin || linux

package cellcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWipeBelowBudgetPreservesInstalledPrograms(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/entry", 128)
	material(t, p, "mod/module/file", 128)
	material(t, p, "gopath/bin/keep", 128)
	if r, err := Wipe(p, true); err != nil || r.Reclaimed != 0 {
		t.Fatalf("dry run: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build/entry")); err != nil {
		t.Fatal("dry run removed cache", err)
	}
	r, err := Wipe(p, false)
	if err != nil || r.Reclaimed != 256 || *r.After.LogicalBytes != 0 {
		t.Fatalf("wipe: %+v %v", r, err)
	}
	for _, name := range []string{"gopath/bin/keep", "owner", "lock"} {
		if _, err = os.Stat(filepath.Join(p.Root, name)); err != nil {
			t.Fatal(name, err)
		}
	}
}
func TestWipeRefusesLiveConsumerWithoutPressure(t *testing.T) {
	p := fixture(t)
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	material(t, p, "build/live", 128)
	if r, err := Wipe(p, false); err == nil || r.Outcome != "storage-deferred" {
		t.Fatalf("active wipe accepted: %+v %v", r, err)
	}
	if _, err = os.Stat(filepath.Join(p.Root, "build/live")); err != nil {
		t.Fatal("live cache removed", err)
	}
}
