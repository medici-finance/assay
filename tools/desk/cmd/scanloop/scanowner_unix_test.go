//go:build unix

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6423
//
// TestCoalesceRefusesUnownedHeld — a retained scan worktree directory the invoking user does not own
// is never reused, even when it is otherwise the registered worktree, clean and at the carrier tip.
// Another account is stood in through the uid seam; a real second account is not available here.
func TestCoalesceRefusesUnownedHeld(t *testing.T) {
	f, held := newHeldFixture(t, scanWorktreeLeaf+"retained")
	prev := scanCurrentUID
	scanCurrentUID = func() int { return os.Getuid() + 1 }
	t.Cleanup(func() { scanCurrentUID = prev })

	_, err := f.coalesce(t)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "owned by uid") {
		t.Fatalf("err = %v, want a refusal naming the directory's owner", err)
	}
	assertHeldUntouched(t, f, held, "seed.txt")
}
