package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The sweep's filer is the second shell-to-deskfile site in the comms tools.
// It must speak the same `deskfile new` argv contract as the quarantine filer
// (it is checked against the same fake), or every sweep finding fails to file.
//
// regression: #2726
func TestSweepFilerArgvMatches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake deskfile is a POSIX shell script")
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "commsqueue", "testdata", "fake-deskfile.sh"))
	if err != nil {
		t.Fatalf("read fake deskfile: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deskfile"), src, 0o755); err != nil {
		t.Fatalf("install fake deskfile: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(dir, "filed.log")
	t.Setenv("FAKE_DESKFILE_LOG", log)

	f := Finding{Kind: FindingLaneViolation, ID: "m-9", Detail: "worker-desk -> verify-desk is not a permitted lane"}
	if err := (deskfileSweepFiler{Repo: "owner/repo"}).File("cell-a", f); err != nil {
		t.Fatalf("deskfileSweepFiler.File = %v, want deskfile to accept the filing", err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("fake deskfile recorded nothing: %v", err)
	}
	for _, want := range []string{"repo=owner/repo\n", "raised-by=worker\n", "m-9", "not a permitted lane"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("filing missing %q:\n%s", want, raw)
		}
	}
}
