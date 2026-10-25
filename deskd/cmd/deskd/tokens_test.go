package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

// TestEnvTokenSourceUnchanged is the service contract Verify row 6: with
// --tokens-dir absent (the flag's default, ""), [buildTokenSource] must
// resolve EXACTLY as [adapter.ResolveTokens] always has — the laptop
// `cellctl deskd` path, which mints tokens attended and passes them as env,
// must keep working untouched by this brief's reload contract.
func TestEnvTokenSourceUnchanged(t *testing.T) {
	env := map[string]string{
		"DESKD_GITHUB_TOKEN":             "tok-double-default",
		"DESKD_GITHUB_TOKEN_EXAMPLE_ORG": "tok-double-firstOwner",
	}
	getenv := func(k string) string { return env[k] }
	owners := []string{"example-org", "example-other"}

	src := buildTokenSource("", getenv)
	if _, ok := src.(adapter.EnvTokenSource); !ok {
		t.Fatalf("buildTokenSource(\"\", ...) = %T, want adapter.EnvTokenSource — the absent-flag path must be the unchanged env source", src)
	}

	gotByOwner, gotMissing := src.Resolve(owners)
	wantByOwner, wantMissingOwners := adapter.ResolveTokens(owners, getenv)
	if !reflect.DeepEqual(gotByOwner, wantByOwner) {
		t.Fatalf("EnvTokenSource.Resolve byOwner = %v, want exactly adapter.ResolveTokens's %v", gotByOwner, wantByOwner)
	}

	var gotMissingOwners []string
	for _, m := range gotMissing {
		gotMissingOwners = append(gotMissingOwners, m.Owner)
	}
	sort.Strings(gotMissingOwners)
	sort.Strings(wantMissingOwners)
	// Compared as joined strings rather than reflect.DeepEqual on the slices
	// themselves: adapter.ResolveTokens returns a nil slice when nothing is
	// missing, and an empty-but-non-nil one here would otherwise be a false
	// mismatch that has nothing to do with the property under test.
	if strings.Join(gotMissingOwners, ",") != strings.Join(wantMissingOwners, ",") {
		t.Fatalf("EnvTokenSource.Resolve missing owners = %v, want exactly adapter.ResolveTokens's %v", gotMissingOwners, wantMissingOwners)
	}

	// Positive control: a --tokens-dir value switches the source, so the test
	// above is not passing merely because buildTokenSource ignores the flag.
	if _, ok := buildTokenSource("/run/deskd/tokens", getenv).(adapter.FileTokenSource); !ok {
		t.Fatalf("buildTokenSource with --tokens-dir set did not return adapter.FileTokenSource")
	}
}
