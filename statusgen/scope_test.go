package main

import (
	"strings"
	"testing"
)

func streamsFixture() []*Stream {
	return []*Stream{
		{Name: "assay-launch", Serves: "assay"},
		{Name: "methodology", Serves: "assay"},
		{Name: "app-hardening", Serves: "example-app"},
		{Name: "example-service-spinout", Serves: "example-service"},
		{Name: "infra-split", Serves: "platform"},
		{Name: "untagged-stream", Serves: ""},
	}
}

func TestDeriveScope(t *testing.T) {
	streams := streamsFixture()
	cases := []struct {
		name    string
		changed []string
		want    string
	}{
		{"empty changed-set → whole house", nil, ""},
		{"single assay stream → assay", []string{"docs/streams/assay-launch/README.md"}, "assay"},
		{"two streams, same product → that product", []string{"docs/streams/assay-launch/README.md", "docs/streams/methodology/brief-01.md"}, "assay"},
		{"two products → unscoped", []string{"docs/streams/assay-launch/README.md", "docs/streams/app-hardening/brief-02.md"}, ""},
		{"platform-only change → unscoped (cross-cutting)", []string{"docs/streams/infra-split/README.md"}, ""},
		{"product + platform → unscoped (platform broadens)", []string{"docs/streams/assay-launch/README.md", "docs/streams/infra-split/README.md"}, ""},
		{"tooling change → unscoped", []string{"tools/statusgen/scope.go"}, ""},
		{"infra file → unscoped", []string{"infra/dev/service.yaml"}, ""},
		{"register file (not a stream dir) → unscoped", []string{"docs/streams/findings/2026-07-18-x.md"}, ""},
		{"top-level register file → unscoped", []string{"docs/streams/INTAKE.md"}, ""},
		{"untagged stream changed → unscoped", []string{"docs/streams/untagged-stream/README.md"}, ""},
		{"unknown stream dir → unscoped", []string{"docs/streams/nope/README.md"}, ""},
	}
	for _, c := range cases {
		if got := deriveScope(streams, c.changed); got != c.want {
			t.Errorf("%s: deriveScope=%q want %q", c.name, got, c.want)
		}
	}
}

func TestFilterStreamsByServes(t *testing.T) {
	streams := streamsFixture()
	got := filterStreamsByServes(streams, "assay")
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	// assay streams + the shared (platform) stream are in-scope; other products out.
	for _, want := range []string{"assay-launch", "methodology", "infra-split"} {
		if !names[want] {
			t.Errorf("expected %s in assay scope", want)
		}
	}
	for _, notWant := range []string{"app-hardening", "example-service-spinout"} {
		if names[notWant] {
			t.Errorf("did not expect %s in assay scope", notWant)
		}
	}
}

// TestCheckScopedFindingsAffectsFullUniverse is the regression: a
// single-product PR scopes the per-stream checks to that product, but a
// finding's affects: may legitimately reference any product's stream. The
// known-stream existence check must resolve against the FULL universe, so no
// out-of-scope-but-real stream is ever flagged "unknown stream".
func TestCheckScopedFindingsAffectsFullUniverse(t *testing.T) {
	// Full universe: two products' streams, both real.
	all := []*Stream{
		{Name: "app-hardening", Dir: "/repo/docs/streams/app-hardening", Status: "active", Priority: "P1", Serves: "example-app"},
		{Name: "methodology", Dir: "/repo/docs/streams/methodology", Status: "active", Priority: "P1", Serves: "assay"},
	}
	// Scope to example-app: only app-hardening drives per-stream checks.
	scoped := filterStreamsByServes(all, "example-app")
	// A finding legitimately affecting the out-of-scope methodology stream, plus
	// one referencing a genuinely nonexistent stream (must still flag).
	findings := []Finding{
		{ID: "F-01", Affects: []string{"methodology"}},
		{ID: "F-99", Affects: []string{"ghost-stream"}},
	}
	problems, _ := checkScoped(scoped, all, findings)

	for _, p := range problems {
		if strings.Contains(p, "F-01") && strings.Contains(p, "unknown stream") {
			t.Errorf("out-of-scope but real stream falsely flagged unknown: %q", p)
		}
	}
	// The genuinely unknown stream is still caught — scoping must not blind the check.
	foundGhost := false
	for _, p := range problems {
		if strings.Contains(p, "F-99") && strings.Contains(p, "unknown stream") {
			foundGhost = true
		}
	}
	if !foundGhost {
		t.Errorf("genuinely unknown stream should still flag unknown-stream: %v", problems)
	}

	// Sanity: the whole-house entry point (check == checkScoped(all, all)) also
	// leaves the real stream unflagged and still catches the ghost.
	wholeProblems, _ := check(all, findings)
	for _, p := range wholeProblems {
		if strings.Contains(p, "F-01") && strings.Contains(p, "unknown stream") {
			t.Errorf("whole-house check falsely flagged real stream: %q", p)
		}
	}
}

func TestServesCoverageNotices(t *testing.T) {
	notices := servesCoverageNotices(streamsFixture())
	if len(notices) != 1 {
		t.Fatalf("expected 1 coverage notice (the untagged stream), got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "untagged-stream") {
		t.Errorf("notice should name the untagged stream, got %q", notices[0])
	}
}
