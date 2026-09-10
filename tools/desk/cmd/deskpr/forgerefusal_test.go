package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestDeskprRefusesWithoutMintedToken is the negative-path guard for the retired ambient
// fallback (brief Verify row 10). When the resolver's custody binding yields no token — exactly
// what the real ForgeFor does when githubCustodyMint refuses an empty ghToken — deskpr must
// REFUSE and perform NO forge write, never fall through to whatever gh identity is active. The
// recording fake proves zero forge ops ran, and the git recorder proves no push preceded the
// forge auth.
//
// Fail-first: the whole forge path is net-new, so there is no pre-fix state to redden. The
// guard this pins is "no fall-through": a mutation that dropped `if ferr != nil { return ferr }`
// from cmdCreate — letting a nil forge through, or reaching for an ambient identity — reddens
// this test, because a push and/or a forge op would then be recorded and the exit would be OK.
func TestDeskprRefusesWithoutMintedToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work) // installs the recording fake into forgeForFn (curForge)

	// Model the resolver refusing because the custody binding produced no token — the message
	// is the shape ForgeFor emits (deskkit.Refused), and the ambient path it names is the one
	// this migration retired. curForge stays the recording fake so we can assert it saw nothing.
	rec := curForge
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return nil, deskkit.ForgeRepo{}, deskkit.Refused(
			"cannot obtain the worker App installation token — ForgeFor never falls back to an ambient gh-CLI identity")
	}

	rc := run([]string{"create", "--title", "no token", "--body-min", "should refuse\nBrief: fixture/01"})
	if rc == deskkit.ExitOK {
		t.Fatal("create SUCCEEDED with no minted token — it must refuse, never fall back to an ambient identity")
	}
	if rec.createCalls != 0 || rec.openCalls != 0 || rec.getCalls != 0 {
		t.Fatalf("a forge op ran despite the custody refusal (create=%d open=%d get=%d) — there must be no "+
			"fall-through write", rec.createCalls, rec.openCalls, rec.getCalls)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatalf("create PUSHED despite the custody refusal — no write may precede forge auth; git calls: %v",
			gitCalls(*calls))
	}
}

// TestDeskprUpdateRefusesWithoutMintedToken is the same guard on the update verb.
func TestDeskprUpdateRefusesWithoutMintedToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")

	rec := curForge
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return nil, deskkit.ForgeRepo{}, deskkit.Refused(
			"cannot obtain the worker App installation token — ForgeFor never falls back to an ambient gh-CLI identity")
	}

	rc := run([]string{"update"})
	if rc == deskkit.ExitOK {
		t.Fatal("update SUCCEEDED with no minted token — it must refuse, never fall back")
	}
	if rec.openCalls != 0 {
		t.Fatalf("update queried the forge despite the custody refusal (open=%d)", rec.openCalls)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatalf("update PUSHED despite the custody refusal; git calls: %v", gitCalls(*calls))
	}
}

// TestDeskprNeverShellsGH is the #274 grep-gap guard, at the unit level: the migrated verbs
// perform their reads and writes through the forge, so a create run records NO `gh` subprocess
// in the argv recorder (only git and desktoken). It complements the source-level ban test.
func TestDeskprNeverShellsGH(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	_ = run([]string{"create", "--title", "no gh", "--body-min", "forge only\nBrief: fixture/01"})
	for _, c := range *calls {
		if len(c) > 0 && strings.HasSuffix(c[0], "gh") && !strings.HasSuffix(c[0], "desktoken") {
			// filepath.Base would be "gh" exactly for the forge CLI; desktoken/git are fine.
			base := c[0]
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[i+1:]
			}
			if base == "gh" {
				t.Fatalf("deskpr shelled `gh` after the migration: %v", c)
			}
		}
	}
}
