package adapter_test

import (
	"context"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// recordingFetcher is a fake EscalationFetcher that records every repo it was
// asked to fetch, so a test can prove WHICH client the router dispatched to.
type recordingFetcher struct {
	id    string
	calls []adapter.Repo
}

func (f *recordingFetcher) FetchEscalations(_ context.Context, repo adapter.Repo) adapter.FetchResult {
	f.calls = append(f.calls, repo)
	// A checked-clean, empty slice — enough to tell "this client answered"
	// apart from the router's own could-not-check for an unmapped owner.
	p := domain.Probe{Source: "fake:" + f.id, Method: domain.MethodLocalTable, Window: "test", Limit: 0}
	return adapter.FetchResult{
		Repo:       repo,
		Health:     adapter.RepoHealth{Repo: repo.String(), Assessment: domain.Clean(p)},
		Candidates: domain.NewCount(0, p),
		Derived:    domain.NewCount(0, p),
	}
}

func TestRoutingFetcher(t *testing.T) {
	ctx := context.Background()

	t.Run("mapped owner routes to its client", func(t *testing.T) {
		firstOwner := &recordingFetcher{id: "firstOwner"}
		r := adapter.NewRoutingFetcher(map[string]adapter.EscalationFetcher{
			"example-org": firstOwner,
		})

		repo := adapter.Repo{Owner: "example-org", Name: "console"}
		fr := r.FetchEscalations(ctx, repo)

		if fr.Health.Assessment.Confidence != domain.CheckedClean {
			t.Fatalf("mapped owner should route to its client (checked-clean), got %q: %s",
				fr.Health.Assessment.Confidence, fr.Health.Assessment.Reason)
		}
		if len(firstOwner.calls) != 1 || firstOwner.calls[0] != repo {
			t.Fatalf("expected exactly one call for %v, got %v", repo, firstOwner.calls)
		}
	})

	t.Run("unmapped owner returns could-not-check, never a fallback fetch", func(t *testing.T) {
		firstOwner := &recordingFetcher{id: "firstOwner"}
		r := adapter.NewRoutingFetcher(map[string]adapter.EscalationFetcher{
			"example-org": firstOwner,
		})

		repo := adapter.Repo{Owner: "example-other", Name: "tracker"}
		fr := r.FetchEscalations(ctx, repo)

		if fr.Health.Assessment.Confidence != domain.CouldNotCheck {
			t.Fatalf("unmapped owner must be could-not-check, got %q", fr.Health.Assessment.Confidence)
		}
		if fr.Health.Assessment.Reason == "" {
			t.Fatal("could-not-check must carry a reason naming the owner")
		}
		// The one client we DO have must not have been consulted — no
		// silent fallback to another org's token.
		if len(firstOwner.calls) != 0 {
			t.Fatalf("unmapped owner must NOT fall back to another org's client; got calls %v", firstOwner.calls)
		}
		// Counts must be underivable, not a published zero.
		if _, ok := fr.Candidates.Value(); ok {
			t.Fatal("Candidates for an unmapped owner must be could-not-check, not a clean 0")
		}
		if _, ok := fr.Derived.Value(); ok {
			t.Fatal("Derived for an unmapped owner must be could-not-check, not a clean 0")
		}
	})

	t.Run("two orgs route to two distinct clients, no cross-use", func(t *testing.T) {
		firstOwner := &recordingFetcher{id: "firstOwner"}
		secondOwner := &recordingFetcher{id: "secondOwner"}
		r := adapter.NewRoutingFetcher(map[string]adapter.EscalationFetcher{
			"example-org":   firstOwner,
			"example-other": secondOwner,
		})

		repoM := adapter.Repo{Owner: "example-org", Name: "console"}
		repoJ := adapter.Repo{Owner: "example-other", Name: "tracker"}
		r.FetchEscalations(ctx, repoM)
		r.FetchEscalations(ctx, repoJ)

		if len(firstOwner.calls) != 1 || firstOwner.calls[0] != repoM {
			t.Fatalf("firstOwner client should have seen only %v, got %v", repoM, firstOwner.calls)
		}
		if len(secondOwner.calls) != 1 || secondOwner.calls[0] != repoJ {
			t.Fatalf("secondOwner client should have seen only %v, got %v", repoJ, secondOwner.calls)
		}
		if firstOwner == secondOwner {
			t.Fatal("the two orgs must route to two DIFFERENT clients")
		}
	})
}

func TestResolveTokens(t *testing.T) {
	t.Run("org-specific token wins", func(t *testing.T) {
		env := map[string]string{
			"DESKD_GITHUB_TOKEN":             "default-tok",
			"DESKD_GITHUB_TOKEN_EXAMPLE_ORG": "example-tok",
		}
		got, missing := adapter.ResolveTokens([]string{"example-org"}, mapGetenv(env))
		if len(missing) != 0 {
			t.Fatalf("no owner should be missing, got %v", missing)
		}
		if got["example-org"] != "example-tok" {
			t.Fatalf("org-specific token should win, got %q", got["example-org"])
		}
	})

	t.Run("legacy default fills an org with no specific token", func(t *testing.T) {
		env := map[string]string{"DESKD_GITHUB_TOKEN": "default-tok"}
		got, missing := adapter.ResolveTokens([]string{"example-org", "example-other"}, mapGetenv(env))
		if len(missing) != 0 {
			t.Fatalf("default should cover both, missing %v", missing)
		}
		if got["example-org"] != "default-tok" || got["example-other"] != "default-tok" {
			t.Fatalf("both owners should get the default, got %v", got)
		}
	})

	t.Run("org with neither specific nor default is reported missing, fail-closed", func(t *testing.T) {
		// Only firstOwner has a token; secondOwner has neither a specific one nor a default.
		env := map[string]string{"DESKD_GITHUB_TOKEN_EXAMPLE_ORG": "example-tok"}
		got, missing := adapter.ResolveTokens([]string{"example-org", "example-other"}, mapGetenv(env))
		if len(missing) != 1 || missing[0] != "example-other" {
			t.Fatalf("example-other should be reported missing, got %v", missing)
		}
		if got["example-org"] != "example-tok" {
			t.Fatalf("firstOwner should still resolve, got %q", got["example-org"])
		}
		// The missing org must NOT be silently mapped to another org's token.
		if _, present := got["example-other"]; present {
			t.Fatal("a missing org must be absent from the map, never mapped to a foreign token")
		}
	})
}

func TestEnvTokenName(t *testing.T) {
	cases := map[string]string{
		"example-org":   "DESKD_GITHUB_TOKEN_EXAMPLE_ORG",
		"example-other": "DESKD_GITHUB_TOKEN_EXAMPLE_OTHER",
		"assay":         "DESKD_GITHUB_TOKEN_ASSAY",
	}
	for owner, want := range cases {
		if got := adapter.EnvTokenName(owner); got != want {
			t.Errorf("EnvTokenName(%q) = %q, want %q", owner, got, want)
		}
	}
}

func mapGetenv(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}
