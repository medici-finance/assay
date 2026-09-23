package main

// claimstore_test.go — the claim-store seam: the claim tool obtains its store from
// deskkit.ResolveClaimStore. Unset key → the forge-ref store exactly as before, plus the removal
// NOTICE printed AFTER the verb's own output; a configured store that does not resolve → exit 6
// with the forge store never built.

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
		deskkit.ClaimStoreFile:     {deskkit.EnvClaimStore + "=" + deskkit.ClaimStoreFile, "ships the file store"},
		deskkit.ClaimStoreService:  {deskkit.EnvClaimStore + "=" + deskkit.ClaimStoreService, "ships the served store"},
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
