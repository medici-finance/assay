package main

// tokenforks_test.go — desk-tools brief 25, Verify row 7.
//
// checkAppToken performs the App-token lookup TWICE per run: once explicitly, so the
// condition can refuse with the role and the token PATH, and once inside ResolveForge,
// whose GitHub custody calls the hook exec.go installs — which calls the same seam again.
// exec.go's own comment claimed that letting the resolver repeat it "would mint twice per
// run"; before brief 25 (#1036) that is precisely what happened, because each lookup forked
// the `desktoken` binary.
//
// Now the second lookup is a memo hit. The explicit call stays — it is the only thing that
// carries the token path into the condition's message — and the run forks once.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestFlipForksTokenMinterOncePerRun(t *testing.T) {
	plantForgeRoster(t)
	t.Setenv("DESK_LOOP", flipRole)

	// The PRODUCTION binding, so both lookups run through the real path.
	oldMint := mintTokenFn
	mintTokenFn = deskkit.RoleTokenForRepo
	t.Cleanup(func() { mintTokenFn = oldMint })

	tokenDir := t.TempDir()
	tokenPath := filepath.Join(tokenDir, "reviewer-token-100000004")
	if err := os.WriteFile(tokenPath, []byte("stub-token-flip_token"), 0o600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var forks []string
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mu.Lock()
		forks = append(forks, role+"/"+owner)
		mu.Unlock()
		return tokenPath, "", nil
	})
	t.Cleanup(restore)

	before := deskkit.RoleTokenMints()

	fr := deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}
	fg, res, err := checkAppToken(flipOpts{pr: 7, repo: fr.Slug()}, fr)
	if err != nil {
		t.Fatalf("checkAppToken: %v", err)
	}
	if fg == nil {
		t.Fatal("checkAppToken returned no forge")
	}
	if res.Kind != deskkit.ForgeGitHub {
		t.Fatalf("resolved forge kind = %q, want github", res.Kind)
	}

	got := deskkit.RoleTokenMints() - before
	if got != 1 {
		mu.Lock()
		seen := append([]string(nil), forks...)
		mu.Unlock()
		t.Fatalf("one flip run forked the token minter %d times, want 1 — the explicit condition lookup "+
			"and the resolver's custody lookup must share one fork; forks: %v", got, seen)
	}

	// The condition's OK line must still name the token PATH: that message is the reason
	// the explicit lookup was kept rather than deleted. say() writes to stderr, so the
	// second run is captured off a pipe.
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	_, _, err = checkAppToken(flipOpts{pr: 7, repo: fr.Slug()}, fr)
	_ = w.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("second checkAppToken: %v", err)
	}
	if !strings.Contains(string(out), tokenPath) {
		t.Fatalf("the app-token condition's OK line no longer names the token path; got: %s", out)
	}
}
