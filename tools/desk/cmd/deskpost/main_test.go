package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestVersion(t *testing.T) {
	_, _ = setupFake(t) // isolate; version does no network
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("version exit = %d, want 0", code)
	}
}

func TestUsageNoArgs(t *testing.T) {
	_, _ = setupFake(t)
	if code := run(nil); code != 2 {
		t.Fatalf("no-args exit = %d, want 2 (usage)", code)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"merge", "example-org/examples", "1"}); code != 2 {
		t.Fatalf("unknown subcommand exit = %d, want 2", code)
	}
}

// TestKillSwitchExit3 exercises the kill switch on a real subcommand: DISABLED=1 →
// exit 3, one disabled audit line, and NO network call (fake records zero hits).
func TestKillSwitchExit3(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	code := run([]string{"ready", "example-org/tracker", "1"})
	if code != deskkit.ExitDisabled {
		t.Fatalf("disabled exit = %d, want %d", code, deskkit.ExitDisabled)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected NO network hits while disabled, got %v", f.hits)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultDisabled {
		t.Fatalf("last audit result = %q, want disabled", got)
	}
}

func TestReviewMissingHeadIsUsage(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "r.md", "## Review\n\nVerdict: approve\n")
	// no --head
	code := run([]string{"review", "example-org/tracker", "1", "--verdict", "approve", "--body-file", bf})
	if code != 2 {
		t.Fatalf("missing --head exit = %d, want 2 (usage)", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review should be posted on a usage error")
	}
}

func TestBadVerdictFlag(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "r.md", "## Review\n\nVerdict: approve\n")
	code := run([]string{"review", "example-org/tracker", "1", "--verdict", "maybe", "--head", testHead, "--body-file", bf})
	if code != 2 {
		t.Fatalf("bad --verdict exit = %d, want 2", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review posted on a bad verdict flag")
	}
}

func TestUnpinnedWarning(t *testing.T) {
	_, errBuf := setupFake(t)
	run([]string{"version"})
	if !strings.Contains(errBuf.String(), "UNPINNED") {
		t.Fatalf("expected UNPINNED warning on stderr, got %q", errBuf.String())
	}
}
