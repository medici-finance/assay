package deskkit

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// extcatalogue_test.go — the CLASS guard for roster-key classification. The defect class: a key
// that decides WHO acts (which login is trusted, which credential a write runs as) catalogued as an
// EXTENSION key, where a malformed value degrades one feature instead of refusing the roster.
// ASSAY_RUN_CREDENTIALS was one instance (reclassified to a trust key by a driver ruling). This
// guard fails on ANY key that reaches the extension catalogue without a deliberate edit here.

// trustKeysNeverExtension — the roster keys that decide who acts. None may ever be an extension key.
var trustKeysNeverExtension = []string{
	EnvBlessLogin,
	EnvTrustedLogins,
	EnvTrustedBotSlugs,
	EnvAllowedRepos,
	EnvHumanLoginMap,
	EnvRunCredentials,
	EnvStampTrustedLogins,
}

// extCatalogueAllowList — the extension catalogue, committed. Adding a key to extKeyNames fails
// the guard until it is added here too, so every new extension key gets a reviewed classification
// decision instead of arriving by default.
var extCatalogueAllowList = map[string]string{
	EnvRiskCallout:       "risk-callout",
	EnvWriteguardCallout: "writeguard-callout",
	EnvRepoAliases:       "repo-aliases",
	EnvReleaseRepo:       "release-repo",
	EnvScanRepos:         "scan-repos",
	// ASSAY_REPO_FORGES also feeds which minted credential a forge write uses (the forge
	// backend it selects). It stays an extension key by its own design; whether it too should
	// be a trust key is an open question the run-credentials ruling did not decide.
	EnvRepoForges:         "repo-forges",
	EnvChannelDriftTarget: "channel-drift-target",
	EnvHomeRepo:           "home-repo",
}

// extCatalogueViolations checks a catalogue against both rules and returns one line per breach.
func extCatalogueViolations(catalogue map[string]string) []string {
	var out []string
	for _, env := range trustKeysNeverExtension {
		if name, ok := catalogue[env]; ok {
			out = append(out, fmt.Sprintf("%s is a trust key but is catalogued as extension key %q", env, name))
		}
	}
	for env, name := range catalogue {
		if want, ok := extCatalogueAllowList[env]; !ok {
			out = append(out, fmt.Sprintf("%s (%q) is not on the committed extension allow-list", env, name))
		} else if want != name {
			out = append(out, fmt.Sprintf("%s is catalogued as %q, allow-list says %q", env, name, want))
		}
	}
	for env := range extCatalogueAllowList {
		if _, ok := catalogue[env]; !ok {
			out = append(out, fmt.Sprintf("%s is on the allow-list but missing from the catalogue", env))
		}
	}
	sort.Strings(out)
	return out
}

func TestExtCatalogueAllowList(t *testing.T) {
	if v := extCatalogueViolations(extKeyNames); len(v) != 0 {
		t.Fatalf("extension catalogue drifted from its committed classification:\n  %s", strings.Join(v, "\n  "))
	}
}

// TestExtCatalogueGuardCatchesPlant — the positive control: the guard is not vacuous. A trust key
// planted into a copy of the catalogue, and an unreviewed new key, must each be reported.
func TestExtCatalogueGuardCatchesPlant(t *testing.T) {
	for _, plant := range []struct{ env, name, want string }{
		{EnvRunCredentials, "run-credentials", "is a trust key"},
		{EnvStampTrustedLogins, "stamp-trusted-logins", "is a trust key"},
		{"ASSAY_SOME_NEW_KEY", "some-new-key", "not on the committed extension allow-list"},
	} {
		planted := map[string]string{}
		for k, v := range extCatalogueAllowList {
			planted[k] = v
		}
		planted[plant.env] = plant.name
		v := extCatalogueViolations(planted)
		if !strings.Contains(strings.Join(v, "\n"), plant.want) {
			t.Errorf("planted %s: guard reported %v, want a line containing %q", plant.env, v, plant.want)
		}
	}
	if v := extCatalogueViolations(extCatalogueAllowList); len(v) != 0 {
		t.Errorf("the allow-list itself violates the guard: %v", v)
	}
}
