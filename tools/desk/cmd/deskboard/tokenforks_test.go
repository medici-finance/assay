package main

// tokenforks_test.go — desk-tools brief 25, Verify row 6.
//
// The board's reads resolve a forge PER REPOSITORY: forgeFor is called from board.go,
// health.go, stalled.go, zeroci.go and prstate.go, once per repo per read, and each call
// reaches deskkit.ForgeFor -> the custody hook installed in forge.go -> the role-token
// lookup. Before the memo (brief 25, #1036) each of those forked the `desktoken` binary:
// 140 forks per `deskboard actions --delta` tick over 10 repositories, 99.7% of which
// reported "reused cached".
//
// The assertion is a NUMBER, not narration: deskkit.RoleTokenMints() counts the forks this
// process actually performed, so N resolutions over M accounts must cost M.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestBoardReadsForkTokenMinterOncePerOwner(t *testing.T) {
	// Six repositories across two accounts — N > M, so a per-repo fork and a per-owner
	// fork give different numbers and the row can fail.
	repos := []string{
		"acct-one/alpha", "acct-one/beta", "acct-one/gamma",
		"acct-two/delta", "acct-two/epsilon", "acct-two/zeta",
	}
	const wantOwners = 2

	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001\n" +
		"ASSAY_ALLOWED_REPOS=" + strings.Join(repos, ":ci:private,") + ":ci:private\n" +
		"ASSAY_REPO_FORGES=" + strings.Join(repos, "=github,") + "=github\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("DESK_LOOP", "the-desk")
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	// The PRODUCTION binding: the board's own lookup seam, so this exercises the real path
	// rather than a stub of it.
	oldMint := mintTokenFn
	mintTokenFn = deskkit.RoleTokenForRepo
	t.Cleanup(func() { mintTokenFn = oldMint })

	// A minter that writes a per-account token file and records every fork. This is the
	// process boundary the memo removes: in production it is exec.Command("desktoken", ...).
	tokenDir := t.TempDir()
	var mu sync.Mutex
	var forks []string
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mu.Lock()
		forks = append(forks, role+"/"+owner)
		mu.Unlock()
		p := filepath.Join(tokenDir, role+"-token-"+owner)
		if err := os.WriteFile(p, []byte("stub-token-"+owner), 0o600); err != nil {
			return "", "", err
		}
		return p, "", nil
	})
	t.Cleanup(restore)

	before := deskkit.RoleTokenMints()

	// Three passes over the six repositories: eighteen resolutions, the shape of a board
	// tick that reads open PRs, then health, then stalled.
	for pass := 0; pass < 3; pass++ {
		for _, repo := range repos {
			if _, _, err := forgeFor(repo); err != nil {
				t.Fatalf("forgeFor(%s) pass %d: %v", repo, pass, err)
			}
		}
	}

	got := deskkit.RoleTokenMints() - before
	if got > wantOwners {
		mu.Lock()
		seen := append([]string(nil), forks...)
		mu.Unlock()
		t.Fatalf("18 forge resolutions over %d accounts forked the token minter %d times, want at most %d; forks: %v",
			wantOwners, got, wantOwners, seen)
	}
	if got != wantOwners {
		t.Fatalf("expected exactly %d forks (one per account), got %d", wantOwners, got)
	}
}
