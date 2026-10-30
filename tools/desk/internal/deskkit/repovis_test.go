package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// The public-repo WRITE gate authorizes an outward write by the REPOSITORY, not the item:
// a public repo listed in the allowed-repos set with the `:public` token passes (an internal
// repo needs the `:internal` token — see TestGateInternalToken),
// and the former per-item `+1` reaction check is gone. These tests drive the decision
// table over (live visibility × configured entry) with the package stub fetcher — no
// network, no reaction fixtures.

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
// shape (the signature no longer distinguishes create / comment / verdict) against a live
// "public" read. A live "internal" read on the same :public entry now REFUSES: the configured
// claim must name the live value (TestGateInternalToken).
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

	t.Run("internal_listed_public_refuses", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibility: "internal"}
		if err := PublicRepoGate(f, "example-org", "pubrepo"); !IsRefused(err) {
			t.Fatalf("listed :public repo, live internal: got %v, want Refused/exit 5 — the claim must name the live value", err)
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
		// A live private read passes regardless of the configured entry — the private arm
		// returns before the configured-visibility check.
		for _, repo := range []string{"pubrepo", "privrepo", "not-in-set"} {
			f := &stubRepoInfoFetcher{visibility: "private"}
			if err := PublicRepoGate(f, "example-org", repo); err != nil {
				t.Fatalf("%s: live private repo should pass regardless of config, got %v", repo, err)
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

// stubVisibilityForge is a minimal Forge test double for ForgeRepoInfoFetcher: it embeds a
// nil Forge (every unoverridden method panics loudly rather than silently) and overrides only
// RepoVisibility, recording the coordinate it was asked about.
type stubVisibilityForge struct {
	Forge         // nil — ForgeRepoInfoFetcher must call nothing else
	visibility    string
	visibilityErr error
	gotRepo       ForgeRepo
	calls         int
}

func (s *stubVisibilityForge) RepoVisibility(repo ForgeRepo) (string, error) {
	s.calls++
	s.gotRepo = repo
	return s.visibility, s.visibilityErr
}

// TestForgeRepoInfoFetcherDelegatesToResolvedForge — assay#1054's regression coverage at
// the deskkit level (cmd/deskpr's gatewired_test.go pins the same property end to end
// through a real command invocation). ForgeRepoInfoFetcher must be a THIN adapter: it asks
// the (owner, repo) coordinate it was given, translated into a ForgeRepo, of whichever Forge
// it wraps — never a second, hardcoded client of its own — and it must not swallow or
// reshape an error the wrapped Forge returns (the gate's fail-closed posture depends on the
// error reaching PublicRepoGate unchanged).
func TestForgeRepoInfoFetcherDelegatesToResolvedForge(t *testing.T) {
	t.Run("delegates the coordinate and the answer", func(t *testing.T) {
		fake := &stubVisibilityForge{visibility: "private"}
		fetcher := ForgeRepoInfoFetcher{Forge: fake}

		got, err := fetcher.RepoVisibility("acme", "widgets")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "private" {
			t.Fatalf("got %q, want %q", got, "private")
		}
		if fake.calls != 1 {
			t.Fatalf("wrapped Forge.RepoVisibility called %d times, want 1", fake.calls)
		}
		if fake.gotRepo != (ForgeRepo{Owner: "acme", Name: "widgets"}) {
			t.Fatalf("wrapped Forge asked about %+v, want acme/widgets", fake.gotRepo)
		}
	})

	t.Run("propagates a read error unchanged (fail-closed)", func(t *testing.T) {
		wantErr := errors.New("boom: forge unreachable")
		fake := &stubVisibilityForge{visibilityErr: wantErr}
		fetcher := ForgeRepoInfoFetcher{Forge: fake}

		_, err := fetcher.RepoVisibility("acme", "widgets")
		if err == nil {
			t.Fatal("expected the wrapped Forge's error to propagate, got nil")
		}
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want it to wrap %v", err, wantErr)
		}
	})

	// The whole point of this type (assay#1054): it must satisfy PublicRepoGate's fetcher
	// contract so a caller can hand the gate whichever Forge it already resolved, instead of
	// constructing a separate GitHub-only HTTPRepoInfoFetcher regardless of the actual forge.
	t.Run("satisfies PublicRepoGate end to end", func(t *testing.T) {
		installRoster(t, gateRoster)
		fake := &stubVisibilityForge{visibility: "public"}
		fetcher := ForgeRepoInfoFetcher{Forge: fake}
		if err := PublicRepoGate(fetcher, "example-org", "pubrepo"); err != nil {
			t.Fatalf("PublicRepoGate via ForgeRepoInfoFetcher: %v", err)
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

// internalGateRoster carries one entry per visibility token plus a bare entry, so every
// (live visibility x configured token) cell of the internal arm has a repo to read.
const internalGateRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_HUMAN_LOGIN_MAP=ada:ada
ASSAY_ALLOWED_REPOS=example-org/inner:ci:internal,example-org/pubrepo:ci:public,example-org/privrepo:ci:private,example-org/bare
`

// TestGateInternalToken — live visibility `internal` has its own allowed-repos token. The
// gate accepts a live-internal repo ONLY on an explicit `:internal` entry, refuses
// `:internal` on a live-public repo, and keeps the internal repo under every public-form
// requirement (risk-classing, the self-containment scan, the neutral on-behalf-of form).
//
// regression: #4978
func TestGateInternalToken(t *testing.T) {
	installRoster(t, internalGateRoster)

	t.Run("internal_token_on_live_public_refuses", func(t *testing.T) {
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "inner")
		if !IsRefused(err) {
			t.Fatalf(":internal entry, live public: got %v, want Refused/exit 5 — :internal must never authorize a public write", err)
		}
		if !containsRemedy(err) {
			t.Fatalf("refusal names no :public remedy: %v", err)
		}
	})

	t.Run("live_internal_without_internal_token_refuses", func(t *testing.T) {
		for _, repo := range []string{"bare", "privrepo", "pubrepo", "not-in-set"} {
			f := &stubRepoInfoFetcher{visibility: "internal"}
			err := PublicRepoGate(f, "example-org", repo)
			if !IsRefused(err) {
				t.Fatalf("%s, live internal: got %v, want Refused/exit 5", repo, err)
			}
			if msg := err.Error(); !strings.Contains(msg, "example-org/"+repo+":internal") || !strings.Contains(msg, EnvAllowedRepos) {
				t.Fatalf("%s: refusal does not name the :internal remedy: %v", repo, err)
			}
		}
	})

	t.Run("live_internal_with_internal_token_passes", func(t *testing.T) {
		for _, v := range []string{"internal", "INTERNAL", " internal\n"} {
			f := &stubRepoInfoFetcher{visibility: v}
			if err := PublicRepoGate(f, "example-org", "inner"); err != nil {
				t.Fatalf("live %q on an :internal entry should pass, got %v", v, err)
			}
		}
	})

	t.Run("internal_keeps_public_form", func(t *testing.T) {
		const repo = "example-org/inner"
		if RepoVisibility(repo) != VisibilityInternal {
			t.Fatalf("RepoVisibility(%q) = %v, want internal", repo, RepoVisibility(repo))
		}
		if !publicFormRequired(repo) {
			t.Fatalf("publicFormRequired(%q) = false — an internal repo must take the public form", repo)
		}
		if !VisibilityRiskClassed(repo) {
			t.Fatalf("VisibilityRiskClassed(%q) = false — an internal repo must be risk-classed", repo)
		}
		if !SelfContainApplies(repo) {
			t.Fatalf("SelfContainApplies(%q) = false — an internal repo must be self-containment scanned", repo)
		}
	})
}
