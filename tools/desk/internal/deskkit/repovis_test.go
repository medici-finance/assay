package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// The public-repo WRITE gate authorizes an outward write by the REPOSITORY, not the item:
// a public/internal repo listed in the allowed-repos set with the `:public` token passes,
// and the former per-item `+1` reaction check is gone. These tests drive the decision
// table over (live visibility × configured entry) with the package stub fetcher — no
// network, no reactions surface.

// gateRoster lists one :public repo and one :private repo, so the configured-visibility
// read the gate depends on has both a positive and a negative entry to consult.
const gateRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_ALLOWED_REPOS=example-org/pubrepo:ci:public,example-org/privrepo:ci:private
`

// gatePatternRoster admits example-org/* by PATTERN only. A pattern carries no visibility
// policy (it widens IsAllowedRepo alone), so a pattern-matched public repo must still refuse.
const gatePatternRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_ALLOWED_REPOS=example-org/*
`

// TestPublicRepoGateListedPublicPasses — a repo listed with :public passes for every write
// shape (the signature no longer distinguishes create / comment / verdict), against both a
// live "public" and a live "internal" read.
func TestPublicRepoGateListedPublicPasses(t *testing.T) {
	installRoster(t, gateRoster)

	// The three write shapes are the SAME call now that the issue-number parameter is gone;
	// running all three documents that no caller shape reintroduces it.
	for _, shape := range []string{"create", "comment", "verdict"} {
		t.Run("public_"+shape, func(t *testing.T) {
			f := &stubRepoInfoFetcher{visibility: "public"}
			if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
				t.Fatalf("%s-shaped call on a listed :public repo (live public) should pass, got %v", shape, err)
			}
		})
	}

	t.Run("internal_listed_public_passes", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibility: "internal"}
		if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
			t.Fatalf("listed :public repo, live internal should pass, got %v", err)
		}
	})

	// Case/whitespace on the live read must not knock a listed :public repo out of the pass.
	t.Run("public_recased_still_passes", func(t *testing.T) {
		for _, v := range []string{"PUBLIC", " public", "Public\n"} {
			f := &stubRepoInfoFetcher{visibility: v}
			if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
				t.Fatalf("visibility %q on a listed :public repo should pass, got %v", v, err)
			}
		}
	})
}

// TestPublicRepoGateUnlistedPublicRefuses — the NEGATIVE control. A live-public repo that
// is not listed :public refuses (exit 5): absent from the set, matched only by an owner/*
// pattern, or an entirely unconfigured set. This is what proves the gate was REPLACED, not
// removed — without it, a gate that authorized every public repo would pass the happy path.
func TestPublicRepoGateUnlistedPublicRefuses(t *testing.T) {
	t.Run("unlisted", func(t *testing.T) {
		installRoster(t, gateRoster)
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "unlisted")
		if !IsRefused(err) {
			t.Fatalf("unlisted live-public repo: got %v, want Refused/exit 5", err)
		}
		if !containsRemedy(err) {
			t.Fatalf("refusal message names no remedy: %v", err)
		}
	})

	t.Run("pattern_only", func(t *testing.T) {
		installRoster(t, gatePatternRoster)
		// Sanity: the pattern DOES admit the repo to the write set...
		if !IsAllowedRepo("example-org/anything") {
			t.Fatal("pattern roster should admit example-org/anything to IsAllowedRepo")
		}
		// ...but carries no :public policy, so the gate still refuses a public write.
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "anything")
		if !IsRefused(err) {
			t.Fatalf("pattern-only live-public repo: got %v, want Refused/exit 5 — a pattern must not authorize a public write", err)
		}
	})

	t.Run("unconfigured_set", func(t *testing.T) {
		installRoster(t, rosterTrustButNoScope)
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "pubrepo")
		if !IsRefused(err) {
			t.Fatalf("unconfigured allowed-repos set, live-public: got %v, want Refused/exit 5 — nothing is listed", err)
		}
	})
}

// TestPublicRepoGateRosterDriftRefuses — row 5, the lower-layer proof. The roster entry
// (the new UPPER layer) claims :private for a repo the forge now reports public; the LIVE
// read (the LOWER layer) refuses rather than passing on the stale roster claim. The gate
// requires the live read AND the configured claim to agree.
func TestPublicRepoGateRosterDriftRefuses(t *testing.T) {
	installRoster(t, gateRoster)
	// privrepo is configured :private; the forge reports it public (someone flipped it).
	f := &stubRepoInfoFetcher{visibility: "public"}
	err := PublicRepoGate(f, "example-org", "privrepo")
	if !IsRefused(err) {
		t.Fatalf("roster drift (configured :private, live public): got %v, want Refused/exit 5 — the stale claim must not authorize", err)
	}
}

// TestPublicRepoGatePrivateAndUnreadable — the private arm is unchanged, and every
// unreadable/unrecognised live read fails closed at exit 6.
func TestPublicRepoGatePrivateAndUnreadable(t *testing.T) {
	installRoster(t, gateRoster)

	t.Run("private_passes", func(t *testing.T) {
		// A live-private read passes regardless of the configured entry — the private arm
		// returns before the configured-visibility check.
		for _, repo := range []string{"pubrepo", "privrepo", "not-in-set"} {
			f := &stubRepoInfoFetcher{visibility: "private"}
			if err := PublicRepoGate(f, "example-org", repo); err != nil {
				t.Fatalf("%s: live-private repo should pass regardless of config, got %v", repo, err)
			}
		}
	})

	t.Run("private_case_and_whitespace", func(t *testing.T) {
		for _, v := range []string{"PRIVATE", " private ", "private\n"} {
			f := &stubRepoInfoFetcher{visibility: v}
			if err := PublicRepoGate(f, "example-org", "not-in-set"); err != nil {
				t.Fatalf("visibility %q should pass as private, got %v", v, err)
			}
		}
	})

	t.Run("visibility_read_error_exit6", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibilityErr: errors.New("connection refused")}
		err := PublicRepoGate(f, "example-org", "pubrepo")
		if !IsUnverifiable(err) {
			t.Fatalf("visibility read error: got %v, want Unverifiable/exit 6", err)
		}
	})

	t.Run("visibility_empty_exit6", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibility: ""}
		err := PublicRepoGate(f, "example-org", "pubrepo")
		if !IsUnverifiable(err) {
			t.Fatalf("empty visibility: got %v, want Unverifiable/exit 6", err)
		}
	})

	t.Run("visibility_unrecognised_exit6", func(t *testing.T) {
		for _, v := range []string{"internal-preview", "restricted", "unknown-future-value"} {
			f := &stubRepoInfoFetcher{visibility: v}
			err := PublicRepoGate(f, "example-org", "pubrepo")
			if !IsUnverifiable(err) {
				t.Fatalf("visibility %q: got %v, want Unverifiable/exit 6", v, err)
			}
		}
	})
}

// containsRemedy reports whether the refusal message names the allowed-repos remedy, so a
// refusal is actionable rather than a bare "no".
func containsRemedy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, EnvAllowedRepos) && strings.Contains(msg, ":public")
}

func TestFetchRepoVisibility(t *testing.T) {
	t.Run("returns_visibility", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibility: "public"}
		v, err := FetchRepoVisibility(f, "example-org", "example-k8s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "public" {
			t.Fatalf("got %q, want %q", v, "public")
		}
	})

	t.Run("propagates_error", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibilityErr: errors.New("boom")}
		_, err := FetchRepoVisibility(f, "example-org", "example-k8s")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestIsBlessAuthorityIDStrictRejectsZero — the helper is still used by the item-level
// author-trust surfaces (deskclose/deskmerge/deskdigest), which this brief does NOT touch;
// this pins the strict/lenient distinction so the two variants cannot be quietly collapsed.
func TestIsBlessAuthorityIDStrictRejectsZero(t *testing.T) {
	cases := []struct {
		login string
		id    int64
		want  bool
	}{
		{"ada", fixtureBlessID, true},
		{"Ada", fixtureBlessID, true}, // login match is case-insensitive, as in trust.go
		{"ada", 0, false},             // THE point: no id is not a pass
		{"ada", 99999, false},
		{"example-org", fixtureBlessID, false},
		{"", fixtureBlessID, false},
	}
	for _, c := range cases {
		if got := IsBlessAuthorityIDStrict(c.login, c.id); got != c.want {
			t.Fatalf("IsBlessAuthorityIDStrict(%q, %d) = %v, want %v", c.login, c.id, got, c.want)
		}
	}
	// And the lenient variant still admits zero — if this ever changes, the two helpers
	// have merged and the strict call sites lost their distinction.
	if !IsBlessAuthorityID("ada", 0) {
		t.Fatal("IsBlessAuthorityID no longer admits id==0 — it and IsBlessAuthorityIDStrict have collapsed " +
			"into one; either delete the strict variant deliberately or restore the arm")
	}
}
