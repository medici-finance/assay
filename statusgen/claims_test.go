package main

import "testing"

func TestParseBranchClaim(t *testing.T) {
	cases := []struct {
		branch string
		token  string
		num    string
		wantOK bool
	}{
		{"fix/ledger-hardening-06-idempotency", "ledger-hardening", "06", true},
		{"fix/ledger-hardening-07-batch-cid-backfill", "ledger-hardening", "07", true},
		{"feature/frontend-03-intent-dashboard", "frontend", "03", true},
		{"docs/frontend-14-auth-e2e-closeout", "frontend", "14", true},
		{"fix/privacy-01-price-feed-read", "privacy", "01", true},
		{"fix/privacy-02-batch-caller-token", "privacy", "02", true},
		{"fix/methodology-metrics-01-status-historian", "methodology-metrics", "01", true},
		{"methodology/brief-05", "methodology", "05", true},
		{"methodology/brief-05-something", "methodology", "05", true},
		{"methodology-metrics/brief-04", "methodology-metrics", "04", true},
		{"fix/ledger-hardening-12a-foo", "ledger-hardening", "12a", true},
		{"fix/issue-110", "", "", false}, // 3 digits: \d{2}[a-z]? requires exactly 2 (+opt letter)
		// parseBranchClaim is purely syntactic — "issue" is not a real stream
		// token, but that's resolveStreamToken's job (see TestClaimedBriefs).
		{"fix/issue-73", "issue", "73", true},
		// no-ops: don't match either branch shape at all.
		{"main", "", "", false},
		{"chore/post-merge-evidence", "", "", false},
		{"ci/self-hosted-runners-only", "", "", false},
		{"worktree-verify-desk", "", "", false},
	}
	for _, c := range cases {
		token, num, ok := parseBranchClaim(c.branch)
		if ok != c.wantOK || token != c.token || num != c.num {
			t.Errorf("parseBranchClaim(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.branch, token, num, ok, c.token, c.num, c.wantOK)
		}
	}
}

func TestResolveStreamToken(t *testing.T) {
	streams := []*Stream{
		mkStream("ledger-hardening", "active", "P0"),
		mkStream("privacy-hardening", "active", "P1"),
		mkStream("methodology", "active", "P1"),
		mkStream("methodology-metrics", "active", "P1"),
	}

	if name, ok := resolveStreamToken(streams, "ledger-hardening"); !ok || name != "ledger-hardening" {
		t.Errorf("exact match: got (%q, %v)", name, ok)
	}
	if name, ok := resolveStreamToken(streams, "privacy"); !ok || name != "privacy-hardening" {
		t.Errorf("unique prefix match: got (%q, %v), want privacy-hardening", name, ok)
	}
	if _, ok := resolveStreamToken(streams, "nonexistent"); ok {
		t.Error("nonexistent token must not resolve")
	}

	// "methodology" is itself an exact stream name AND a prefix of
	// "methodology-metrics" — exact match must win, not the prefix.
	if name, ok := resolveStreamToken(streams, "methodology"); !ok || name != "methodology" {
		t.Errorf("exact match must win over prefix match: got (%q, %v)", name, ok)
	}

	// Ambiguous prefix: two streams sharing a token prefix must not resolve.
	ambiguous := []*Stream{
		mkStream("frontend-old", "active", "P1"),
		mkStream("frontend-new", "active", "P1"),
	}
	if _, ok := resolveStreamToken(ambiguous, "frontend"); ok {
		t.Error("ambiguous prefix must not resolve")
	}
}

func TestClaimedBriefs(t *testing.T) {
	streams := []*Stream{
		mkStream("ledger-hardening", "active", "P0"),
		mkStream("privacy-hardening", "active", "P1"),
		mkStream("methodology", "active", "P1"),
	}
	branches := []string{
		"main",
		"fix/ledger-hardening-06-idempotency",
		"fix/privacy-01-price-feed-read",
		"methodology/brief-05",
		"fix/issue-110",          // no-op: doesn't match either pattern
		"fix/unknownstream-01-x", // no-op: token doesn't resolve to a stream
	}
	claimed := claimedBriefs(streams, branches)
	want := map[string]bool{
		"ledger-hardening/06":  true,
		"privacy-hardening/01": true,
		"methodology/05":       true,
	}
	if len(claimed) != len(want) {
		t.Fatalf("claimedBriefs = %v, want %v", claimed, want)
	}
	for k := range want {
		if !claimed[k] {
			t.Errorf("missing claim %q in %v", k, claimed)
		}
	}
}
