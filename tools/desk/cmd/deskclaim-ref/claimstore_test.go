package main

// claimstore_test.go — the claim-store seam: the claim tool obtains its store from
// deskkit.ResolveClaimStore. Unset key → the forge-ref store exactly as before, plus the removal
// NOTICE printed AFTER the verb's own output; a valid store this build does not ship → the same
// forge-ref store under a NOTICE naming the configured store (medici-finance/assay#2753); a
// configured value that does not resolve → exit 6 with the forge store never built.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestClaimToolUnsetKeyUsesTheForgeStoreAndPrintsTheNoticeLast(t *testing.T) {
	f := newStore()
	f.writeFails = true // the verb's own refusal must stay the FIRST stderr line
	run, _, se := harness(t, f)

	if rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitUnverifiable {
		t.Fatalf("acquire rc = %d, want 6; err=%s", rc, se.String())
	}
	lines := strings.Split(strings.TrimRight(se.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("want the verb's refusal then the NOTICE, got:\n%s", se.String())
	}
	if !strings.HasPrefix(lines[0], "dispatch-claim: unverifiable: could not create the claim") {
		t.Errorf("first stderr line %q is not the verb's own message — a dispatcher quotes that line", lines[0])
	}
	if want := "dispatch-claim: " + deskkit.ClaimStoreLegacyNotice; lines[len(lines)-1] != want {
		t.Errorf("last stderr line = %q, want the removal NOTICE %q", lines[len(lines)-1], want)
	}
}

func TestClaimToolConfiguredStoreThatDoesNotResolveNeverBuildsTheForgeStore(t *testing.T) {
	for v, wants := range map[string][]string{
		deskkit.ClaimStoreForgeRef: {deskkit.EnvClaimStore, deskkit.ClaimStoreFile, deskkit.ClaimStoreService},
		"nfs":                      {deskkit.EnvClaimStore, deskkit.ClaimStoreFile, deskkit.ClaimStoreService},
	} {
		t.Run(v, func(t *testing.T) {
			withClaimRoster(t, map[string]string{deskkit.EnvClaimStore: v})
			f := newStore()
			run, so, se := harness(t, f)
			built := 0
			buildStore = func(_, _ string) (deskkit.ClaimStore, error) { built++; return f, nil }

			if rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitUnverifiable {
				t.Fatalf("rc = %d, want 6; out=%s err=%s", rc, so.String(), se.String())
			}
			if built != 0 || len(f.claims) != 0 {
				t.Fatalf("the forge store was built %d time(s) / holds %d claim(s) — a configured store must "+
					"never fall back to it", built, len(f.claims))
			}
			for _, want := range wants {
				if !strings.Contains(se.String(), want) {
					t.Errorf("the refusal does not name %q:\n%s", want, se.String())
				}
			}
		})
	}
}

// TestClaimToolUnshippedFallback — ASSAY_CLAIM_STORE=file (or service) is valid but not
// shipped in this build: the verb runs against the forge store exactly as an unset key's would,
// and the last stderr line is the NOTICE naming the configured store.
// regression: #2753
func TestClaimToolUnshippedFallback(t *testing.T) {
	for v, ships := range map[string]string{
		deskkit.ClaimStoreFile:    "ships the file store",
		deskkit.ClaimStoreService: "ships the served store",
	} {
		t.Run(v, func(t *testing.T) {
			withClaimRoster(t, map[string]string{deskkit.EnvClaimStore: v, deskkit.EnvClaimSingleHost: "yes"})
			f := newStore()
			run, so, se := harness(t, f)
			built := 0
			buildStore = func(_, _ string) (deskkit.ClaimStore, error) { built++; return f, nil }

			if rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
				t.Fatalf("rc = %d, want 0 (an unshipped store is ignored); out=%s err=%s", rc, so.String(), se.String())
			}
			if built != 1 || len(f.claims) != 1 {
				t.Fatalf("the forge store was built %d time(s) / holds %d claim(s), want 1 / 1", built, len(f.claims))
			}
			lines := strings.Split(strings.TrimRight(se.String(), "\n"), "\n")
			last := lines[len(lines)-1]
			if last == "dispatch-claim: "+deskkit.ClaimStoreLegacyNotice {
				t.Fatalf("the UNSET notice was printed for a key the operator did set:\n%s", se.String())
			}
			for _, want := range []string{"dispatch-claim: NOTICE: " + deskkit.EnvClaimStore + "=" + v, "does not ship", ships} {
				if !strings.Contains(last, want) {
					t.Errorf("last stderr line does not carry %q: %q", want, last)
				}
			}
		})
	}
}

// The --help text carries every key, both valid values and the NOTICE, verbatim — the README
// quotes them from here (Verify row 9 compares the two).
func TestHelpNamesTheClaimStoreKeysValuesAndNotice(t *testing.T) {
	_, _, se := harness(t, newStore())
	if rc := run([]string{"--help"}); rc != exitOK {
		t.Fatalf("--help rc = %d, want 0", rc)
	}
	for _, want := range []string{
		deskkit.EnvClaimStore, deskkit.EnvClaimDir, deskkit.EnvClaimSingleHost,
		deskkit.ClaimStoreFile + " | " + deskkit.ClaimStoreService,
		deskkit.ClaimStoreLegacyNotice,
	} {
		if !strings.Contains(se.String(), want) {
			t.Errorf("--help is missing %q:\n%s", want, se.String())
		}
	}
}
