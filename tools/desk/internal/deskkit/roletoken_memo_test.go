package deskkit

// roletoken_memo_test.go — desk-tools brief 25, Verify rows 2 to 5.
//
// The rows this file implements are about a CACHE IN FRONT OF A CREDENTIAL, so they are
// written to fail the way that failure actually shows up: not as an error, but as the
// wrong identity's token coming back with no complaint at all. Row 5 is the
// single-point-of-failure row and asserts exactly that.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// countingMinter installs a token minter that writes a DISTINGUISHABLE token file per
// (role, owner) pair and counts how many times it was actually run. It returns the counter
// (read under its own lock) and restores the previous minter via t.Cleanup.
func countingMinter(t *testing.T) (calls func() int, seen func() []string) {
	t.Helper()
	dir := t.TempDir()
	var mu sync.Mutex
	var order []string
	restore := SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mu.Lock()
		order = append(order, role+"/"+owner)
		mu.Unlock()
		p := filepath.Join(dir, role+"--"+owner+".token")
		if err := os.WriteFile(p, []byte("stub-token-"+role+"_"+owner), 0o600); err != nil {
			return "", "", err
		}
		return p, "", nil
	})
	t.Cleanup(func() {
		restore()
		resetRoleTokenMemo()
	})
	resetRoleTokenMemo()
	return func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(order)
		}, func() []string {
			mu.Lock()
			defer mu.Unlock()
			return append([]string(nil), order...)
		}
}

// TestRoleTokenMemoForksOncePerOwner — Verify row 2. Twenty lookups of one pair fork once;
// spread over two accounts they fork twice. The token and path are identical on every hit,
// so a memo that returned a stale or empty value would be caught here too.
func TestRoleTokenMemoForksOncePerOwner(t *testing.T) {
	calls, _ := countingMinter(t)

	wantTok, wantPath, err := RoleTokenForOwner("reviewer", "acct-one")
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	for i := 0; i < 19; i++ {
		tok, path, err := RoleTokenForOwner("reviewer", "acct-one")
		if err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
		if tok != wantTok || path != wantPath {
			t.Fatalf("lookup %d returned %q/%q, want %q/%q", i, tok, path, wantTok, wantPath)
		}
	}
	if got := calls(); got != 1 {
		t.Fatalf("20 lookups of one (role, owner) pair forked the minter %d times, want 1", got)
	}
	if got := RoleTokenMints(); got != 1 {
		t.Fatalf("RoleTokenMints() = %d, want 1", got)
	}

	// The same twenty spread over two accounts: one fork each, not one in total.
	for i := 0; i < 20; i++ {
		owner := "acct-one"
		if i%2 == 1 {
			owner = "acct-two"
		}
		if _, _, err := RoleTokenForOwner("reviewer", owner); err != nil {
			t.Fatalf("mixed lookup %d: %v", i, err)
		}
	}
	if got := calls(); got != 2 {
		t.Fatalf("lookups over two accounts forked the minter %d times, want 2", got)
	}
}

// TestRoleTokenMemoRemintsPastMaxAge — Verify row 3. Both directions, plus the derivation
// of the constant itself: a memo that outlived desktoken's own reuse window could hand back
// a token the minter would have replaced.
func TestRoleTokenMemoRemintsPastMaxAge(t *testing.T) {
	// The constant must stay strictly under desktoken's cacheMaxAge (50 min), which is
	// itself under GitHub's ~60 min installation-token life. This is the assertion that
	// catches a later widening of the window.
	if roleTokenMemoMaxAge >= 50*time.Minute {
		t.Fatalf("roleTokenMemoMaxAge = %v, must be strictly under desktoken's 50m cacheMaxAge", roleTokenMemoMaxAge)
	}

	calls, _ := countingMinter(t)

	base := time.Now()
	roleTokenNow = func() time.Time { return base }
	t.Cleanup(func() { roleTokenNow = time.Now })

	if _, _, err := RoleTokenForOwner("worker", "acct-one"); err != nil {
		t.Fatalf("seed lookup: %v", err)
	}
	if got := calls(); got != 1 {
		t.Fatalf("seed forked %d times, want 1", got)
	}

	// 44 minutes on: still inside the window, still no fork.
	roleTokenNow = func() time.Time { return base.Add(44 * time.Minute) }
	if _, _, err := RoleTokenForOwner("worker", "acct-one"); err != nil {
		t.Fatalf("44m lookup: %v", err)
	}
	if got := calls(); got != 1 {
		t.Fatalf("a 44-minute-old memo entry forked the minter %d times, want 1 (it must be reused)", got)
	}

	// 46 minutes on: past the guard, so the minter runs again.
	roleTokenNow = func() time.Time { return base.Add(46 * time.Minute) }
	if _, _, err := RoleTokenForOwner("worker", "acct-one"); err != nil {
		t.Fatalf("46m lookup: %v", err)
	}
	if got := calls(); got != 2 {
		t.Fatalf("a 46-minute-old memo entry forked the minter %d times, want 2 (it must be re-minted)", got)
	}
}

// TestRoleTokenMemoDoesNotCacheFailures — Verify row 4. All four failure shapes, each
// followed by a success that must FORK AGAIN, and each refusal's text asserted so the memo
// cannot be shown to have changed what a failure says.
func TestRoleTokenMemoDoesNotCacheFailures(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.token")
	if err := os.WriteFile(goodPath, []byte("stub-token-good"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(dir, "empty.token")
	if err := os.WriteFile(emptyPath, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		fail     func(role, owner string) (string, string, error)
		wantText string
	}{
		{
			name: "minter error",
			fail: func(string, string) (string, string, error) {
				return "", "boom: the minter refused", fmt.Errorf("exit status 6")
			},
			wantText: "cannot mint the reviewer App installation token for acct-one",
		},
		{
			name:     "empty path",
			fail:     func(string, string) (string, string, error) { return "", "", nil },
			wantText: "the token minter returned no path",
		},
		{
			name: "unreadable file",
			fail: func(string, string) (string, string, error) {
				return filepath.Join(dir, "does-not-exist.token"), "", nil
			},
			wantText: "cannot read the reviewer App installation token",
		},
		{
			name:     "empty token",
			fail:     func(string, string) (string, string, error) { return emptyPath, "", nil },
			wantText: "is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRoleTokenMemo()
			var calls int
			restore := SetRoleTokenMinter(func(role, owner string) (string, string, error) {
				calls++
				if calls == 1 {
					return tc.fail(role, owner)
				}
				return goodPath, "", nil
			})
			defer func() {
				restore()
				resetRoleTokenMemo()
			}()

			_, _, err := RoleTokenForOwner("reviewer", "acct-one")
			if err == nil {
				t.Fatalf("first lookup should have failed")
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("refusal text changed: got %q, want it to contain %q", err.Error(), tc.wantText)
			}
			if ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("refusal exit code = %d, want %d", ExitCodeOf(err), ExitUnverifiable)
			}

			tok, _, err := RoleTokenForOwner("reviewer", "acct-one")
			if err != nil {
				t.Fatalf("second lookup after a failure: %v — a memoised failure would pin the refusal", err)
			}
			if tok != "stub-token-good" {
				t.Fatalf("second lookup returned %q, want the freshly minted token", tok)
			}
			if calls != 2 {
				t.Fatalf("the minter ran %d times, want 2 — a failure must never be memoised", calls)
			}
		})
	}
}

// TestRoleTokenMemoNeverCrossesIdentities — Verify row 5, the single-point-of-failure row
// and a NEGATIVE control. Two roles over two accounts, interleaved, each asserted to get
// ITS OWN token. Keying the memo on the role alone, or on the owner alone, reddens this.
func TestRoleTokenMemoNeverCrossesIdentities(t *testing.T) {
	calls, order := countingMinter(t)

	type want struct{ role, owner string }
	combos := []want{
		{"reviewer", "acct-one"},
		{"worker", "acct-one"},
		{"reviewer", "acct-two"},
		{"worker", "acct-two"},
	}

	// Interleaved twice over, so a memo that answered from a narrowed key would be asked
	// for a pair it has already seen under the other half.
	for pass := 0; pass < 3; pass++ {
		for _, c := range combos {
			tok, path, err := RoleTokenForOwner(c.role, c.owner)
			if err != nil {
				t.Fatalf("%s/%s pass %d: %v", c.role, c.owner, pass, err)
			}
			wantTok := "stub-token-" + c.role + "_" + c.owner
			if tok != wantTok {
				t.Fatalf("%s/%s pass %d got token %q, want %q — the memo returned ANOTHER identity's credential",
					c.role, c.owner, pass, tok, wantTok)
			}
			if !strings.Contains(filepath.Base(path), c.role+"--"+c.owner) {
				t.Fatalf("%s/%s pass %d got path %q, which is not this pair's token file", c.role, c.owner, pass, path)
			}
		}
	}

	if got := calls(); got != len(combos) {
		t.Fatalf("four (role, owner) pairs over three passes forked the minter %d times, want %d; order: %v",
			got, len(combos), order())
	}
}
