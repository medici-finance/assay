package deskkit

import (
	"errors"
	"testing"
)

func TestPublicRepoGate(t *testing.T) {
	// Subtests covering the gate's decision table.

	// (a) public repo, no +1 at all → refused, exit 5
	t.Run("public_no_reaction", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions:  nil,
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal, got nil")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (a2) public repo, reactions exist but none is +1 → refused
	t.Run("public_no_plus_one", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "eyes"},
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "rocket"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal, got nil")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (b) public repo, +1 from ada with user.type=="User" and correct numeric id → allowed
	t.Run("public_ada_plus_one", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "+1"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	// (c) public repo, +1 from a Bot actor → refused
	t.Run("public_bot_plus_one", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "assay-reviewer-app[bot]", Type: "Bot"}, Content: "+1"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal, got nil")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (d) public repo, +1 from example-org (type=="User", NOT on human allowlist) → refused
	t.Run("public_example_org_plus_one_refused", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "example-org", Type: "User"}, Content: "+1"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal for example-org +1, got nil — type check alone is NOT sufficient")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (e) public repo, non-+1 reaction from ada → refused
	t.Run("public_ada_non_plus_one", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "eyes"},
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "rocket"},
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "heart"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal, got nil")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (e2) public repo, ada login with WRONG numeric id → refused (login-recycling defence)
	t.Run("public_ada_wrong_id", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: 99999}, Content: "+1"},
			},
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected refusal for ada login with wrong numeric id, got nil — login recycling defence")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	// (f) private repo, no reaction → allowed (gate does not apply)
	t.Run("private_no_reaction", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "private",
		}
		err := PublicRepoGate(f, "example-org", "private-repo", 42)
		if err != nil {
			t.Fatalf("private repo should pass gate, got %v", err)
		}
	})

	// internal is org-visible, not private: gated exactly like public, not a pass-through.
	// Security review on #310 found the pre-fix denylist (`visibility != "public"`) let
	// "internal" fall through ungated — this pins the corrected allowlist behaviour.
	t.Run("internal_repo_requires_plus_one", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "internal",
			reactions:  nil,
		}
		err := PublicRepoGate(f, "example-org", "some-repo", 42)
		if err == nil {
			t.Fatal("expected refusal for internal repo with no qualifying +1, got nil")
		}
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	t.Run("internal_repo_ada_plus_one_passes", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "internal",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "+1"},
			},
		}
		err := PublicRepoGate(f, "example-org", "some-repo", 42)
		if err != nil {
			t.Fatalf("internal repo with a qualifying ada +1 should pass, got %v", err)
		}
	})

	// The allowlist is case-insensitive and trims whitespace on the SAFE side only: it
	// never lets a re-cased or padded "private" fall into the gated branch, and it never
	// lets a re-cased or padded "public"/"internal" fall OUT of it either.
	t.Run("visibility_private_case_and_whitespace_insensitive", func(t *testing.T) {
		for _, v := range []string{"private", "PRIVATE", "Private", " private ", "private\n"} {
			f := &stubRepoInfoFetcher{visibility: v}
			if err := PublicRepoGate(f, "example-org", "private-repo", 42); err != nil {
				t.Fatalf("visibility %q: expected private repo to pass gate, got %v", v, err)
			}
		}
	})

	t.Run("visibility_public_case_and_whitespace_still_gated", func(t *testing.T) {
		for _, v := range []string{"PUBLIC", "Public", "PuBlIc", "public ", " public", "public\n"} {
			f := &stubRepoInfoFetcher{visibility: v, reactions: nil}
			err := PublicRepoGate(f, "example-org", "example-k8s", 42)
			if err == nil {
				t.Fatalf("visibility %q: expected refusal (no qualifying +1), got nil — case/whitespace bypassed the gate", v)
			}
			if !IsRefused(err) {
				t.Fatalf("visibility %q: expected Refused (exit 5), got %v", v, err)
			}
		}
	})

	// Any value that is not recognised as private/public/internal fails closed rather than
	// being treated as private. This is the security-review fix: the pre-fix code took
	// `visibility != "public"` as its private-equivalent branch, so an unrecognised or
	// future API value passed through ungated.
	t.Run("visibility_unrecognised_fails_closed", func(t *testing.T) {
		for _, v := range []string{"internal-preview", "public_read", "unknown-future-value", "restricted"} {
			f := &stubRepoInfoFetcher{visibility: v}
			err := PublicRepoGate(f, "example-org", "some-repo", 42)
			if err == nil {
				t.Fatalf("visibility %q: expected Unverifiable, got nil", v)
			}
			if !IsUnverifiable(err) {
				t.Fatalf("visibility %q: expected Unverifiable (exit 6), got %v", v, err)
			}
		}
	})
}

func TestGateFailsClosed(t *testing.T) {
	// Every failure mode is exit 6 (Unverifiable), never exit 0.

	// Visibility lookup error → exit 6
	t.Run("visibility_error", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibilityErr: errors.New("connection refused"),
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected Unverifiable, got nil")
		}
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
	})

	// Visibility returns empty string (unrecognised) → exit 6
	t.Run("visibility_empty", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "",
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected Unverifiable for empty visibility, got nil")
		}
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
	})

	// Reactions lookup error → exit 6
	t.Run("reactions_error", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility:   "public",
			reactionsErr: errors.New("403 rate limit exceeded"),
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if err == nil {
			t.Fatal("expected Unverifiable, got nil")
		}
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
	})

	// No issue/PR number → exit 6
	t.Run("no_issue_number", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", 0)
		if err == nil {
			t.Fatal("expected Unverifiable, got nil")
		}
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
	})

	// Negative issue number → exit 6
	t.Run("negative_issue_number", func(t *testing.T) {
		f := &stubRepoInfoFetcher{
			visibility: "public",
		}
		err := PublicRepoGate(f, "example-org", "example-k8s", -1)
		if err == nil {
			t.Fatal("expected Unverifiable, got nil")
		}
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
	})
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

// TestGateRefusesBlessLoginWithNoID — the residual from an earlier review.
//
// IsBlessAuthorityID carries an `id == 0` compatibility arm for surfaces that genuinely parse
// no id. A reactions payload is NOT such a surface: login and id come from the SAME
// object, so a zero id means the read failed. Admitting it degrades the numeric pin
// back to a login-only check — exactly the weakness the id was added to close, and a
// recycled `ada` login would then satisfy the gate. The call site therefore uses
// IsBlessAuthorityIDStrict, and this pins it.
func TestGateRefusesBlessLoginWithNoID(t *testing.T) {
	f := &stubRepoInfoFetcher{
		visibility: "public",
		reactions: []Reaction{
			{User: ReactionUser{Login: "ada", Type: "User", ID: 0}, Content: "+1"},
		},
	}
	err := PublicRepoGate(f, "example-org", "example-k8s", 42)
	if err == nil {
		t.Fatal("a +1 from login \"ada\" carrying NO numeric id satisfied the gate — " +
			"the id pin has degraded to a login-only check (use IsBlessAuthorityIDStrict here)")
	}
	if !IsRefused(err) {
		t.Fatalf("expected Refused (exit 5), got %v", err)
	}
}

// TestIsBlessAuthorityIDStrictRejectsZero — the helper itself, so the distinction between the
// two variants cannot be quietly collapsed back into one.
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
	// And the lenient variant still admits zero — if this ever changes, the two
	// helpers have merged and the strict call sites lost their distinction.
	if !IsBlessAuthorityID("ada", 0) {
		t.Fatal("IsBlessAuthorityID no longer admits id==0 — it and IsBlessAuthorityIDStrict have collapsed " +
			"into one; either delete the strict variant deliberately or restore the arm")
	}
}
