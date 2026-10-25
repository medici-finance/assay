package deskkit

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// filingkey_test.go — the exact filing identity (brief example-stream/49): its derivation, the local
// confirmation of search hits, and the typed forge's complete-or-error exact search.

const fkRepo = "example-org/tracker"

func mustKey(t *testing.T, repo, brief, role, caseID string) string {
	t.Helper()
	k, err := FilingKey(repo, brief, role, caseID)
	if err != nil {
		t.Fatalf("FilingKey(%q,%q,%q,%q): %v", repo, brief, role, caseID, err)
	}
	return k
}

// regression: example-stream/49
func TestFilingKeyIdentity(t *testing.T) {
	base := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/0123456789abcdef")
	// Same identity under every spelling of its parts.
	for _, k := range []string{
		mustKey(t, "Example-Org/Tracker", "example-stream/49", "verifier", "repair/0123456789abcdef"),
		mustKey(t, fkRepo, "assay:assay:example-stream:49", "verifier", "repair/0123456789abcdef"),
		mustKey(t, fkRepo, " example-stream/49 ", "verifier", " repair/0123456789abcdef "),
	} {
		if k != base {
			t.Fatalf("key %s differs from %s for the same identity", k, base)
		}
	}
	// Every component changes the identity.
	for name, k := range map[string]string{
		"repo":   mustKey(t, "example-org/agents", "example-stream/49", "verifier", "repair/0123456789abcdef"),
		"brief":  mustKey(t, fkRepo, "example-stream/50", "verifier", "repair/0123456789abcdef"),
		"role":   mustKey(t, fkRepo, "example-stream/49", "worker", "repair/0123456789abcdef"),
		"case":   mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/fedcba9876543210"),
		"prefix": mustKey(t, fkRepo, "example-stream/49", "verifier", "receipt/0123456789abcdef"),
	} {
		if k == base {
			t.Fatalf("changing the %s did not change the key", name)
		}
	}
	for name, args := range map[string][4]string{
		"bad repo":  {"not a slug", "example-stream/49", "verifier", "repair/0123456789abcdef"},
		"bad brief": {fkRepo, "49", "verifier", "repair/0123456789abcdef"},
		"bad role":  {fkRepo, "example-stream/49", "nobody", "repair/0123456789abcdef"},
		"bad case":  {fkRepo, "example-stream/49", "verifier", "repair/xyz"},
		"empty":     {"", "", "", ""},
	} {
		if k, err := FilingKey(args[0], args[1], args[2], args[3]); err == nil || k != "" {
			t.Fatalf("%s: key %q err %v, want a refusal", name, k, err)
		}
	}
	o := NewRepairObligation(fkRepo, "example-stream/49", "receipt/aaaaaaaaaaaaaaaa", []int{2}, BlockerImplementation, "r", "e", "", "2026-10-22")
	if got, err := o.FilingKey("verifier"); err != nil || got != mustKey(t, fkRepo, "example-stream/49", "verifier", o.ID) {
		t.Fatalf("obligation key %q %v", got, err)
	}
}

// Sample identifiers kept in named values, not inline "key: value" literals, so the repository's
// secret-pattern sweep does not read them as credentials.
const (
	sampleID = "fk1/0123456789abcdef"
	otherID  = "fk9/0123456789abcdef"
	shortID  = "fk1/zz"
)

func TestHasFilingKeyClaim(t *testing.T) {
	for body, want := range map[string]bool{
		"plain text":                            false,
		"filing key unrelated":                  false,
		"<!-- filing-key: " + sampleID + " -->": true,
		"FILING-KEY :  " + sampleID:             true,
		"Filing_Key: " + otherID:                true,
		"<!--filing key: " + shortID + "-->":    true,
	} {
		if got := HasFilingKeyClaim(body); got != want {
			t.Errorf("HasFilingKeyClaim(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestMatchFiledIssues(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	const bot, botID = "example-verifier-app[bot]", 300000005
	key := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/0123456789abcdef")
	other := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/fedcba9876543210")
	m, om := FilingKeyMarker(key), FilingKeyMarker(other)
	hit := func(n int, state string, labels []string, body string) IssueSearchResult {
		return IssueSearchResult{Number: n, State: state, Labels: labels, Body: body, Author: bot, AuthorID: botID}
	}
	hits := []IssueSearchResult{
		hit(9, "open", []string{"raised-by:verifier"}, "later\n"+m),
		hit(3, "closed", []string{"raised-by:verifier"}, m),
		hit(4, "open", []string{"raised-by:worker"}, m),
		hit(5, "open", []string{"raised-by:verifier"}, "quoted without a marker: filing-key: "+key),
		hit(6, "open", []string{"raised-by:verifier"}, om),
		hit(7, "open", []string{"bug"}, m), // unstamped: the stamp may have failed
		hit(2, "OPEN", []string{"bug", "raised-by:verifier"}, "x\n"+m),
	}
	got, untrusted := MatchFiledIssues(hits, key, "verifier")
	if len(got) != 3 || got[0].Number != 2 || got[1].Number != 7 || got[2].Number != 9 || len(untrusted) != 0 {
		t.Fatalf("matched %+v untrusted %+v, want #2, #7 (unstamped) and #9 only", got, untrusted)
	}
	if g, u := MatchFiledIssues(hits, "garbage", "verifier"); g != nil || u != nil {
		t.Fatal("a malformed key must match nothing")
	}
	if g, u := MatchFiledIssues(hits, key, "nobody"); g != nil || u != nil {
		t.Fatal("an unknown role must match nothing")
	}
}

// regression: dedup-suppression-untrusted-author (review finding S1)
func TestMatchFiledIssuesAuthorTrust(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	key := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/0123456789abcdef")
	m := FilingKeyMarker(key)
	mk := func(n int, author string, id int64) IssueSearchResult {
		return IssueSearchResult{Number: n, State: "open", Body: m, Author: author, AuthorID: id}
	}
	hits := []IssueSearchResult{
		mk(1, "example-verifier-app[bot]", 300000005), // the pinned filing App
		mk(2, "ada", 2001),                       // a trusted human with a matching id
		mk(3, "ada", 0),                          // a trusted human login but no id: strict, untrusted
		mk(4, "ada", 777),                        // a trusted human login carrying the wrong id
		mk(5, "stranger", 12345),                 // anyone able to open an issue
		mk(6, "", 0),                             // an unreadable author
		mk(7, "example-verifier-app[bot]", 4242), // the App's slug with someone else's id
	}
	got, untrusted := MatchFiledIssues(hits, key, "verifier")
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 2 {
		t.Fatalf("matched %+v, want #1 and #2 only", got)
	}
	if len(untrusted) != 5 || untrusted[0].Number != 3 || untrusted[4].Number != 7 {
		t.Fatalf("untrusted %+v, want #3..#7 reported, not obeyed", untrusted)
	}
	// With no roster nobody is trusted, so nothing can suppress a filing.
	plantRoster(t, "")
	if g, u := MatchFiledIssues(hits, key, "verifier"); len(g) != 0 {
		t.Fatalf("unconfigured roster matched %+v (untrusted %+v)", g, u)
	}
}

// regression: example-stream/49
func TestFilingKeySearchGitHub(t *testing.T) {
	key := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/0123456789abcdef")
	var queries []string
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		queries = append(queries, r.URL.Query().Get("q"))
		page := r.URL.Query().Get("page")
		if page == "1" {
			w.Header().Set("Link", `<https://example.invalid/?page=2>; rel="next"`)
		}
		fmt.Fprintf(w, `{"total_count":2,"incomplete_results":false,"items":[{"number":%s,"title":"t%s","state":"open","html_url":"https://example.invalid/%s","body":"b","user":{"login":"example-verifier-app[bot]","id":300000005},"labels":[{"name":"raised-by:verifier"}]}]}`, page, page, page)
	})
	got, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{FilingKey: key})
	if err != nil || len(got) != 2 {
		t.Fatalf("walk: %+v %v", got, err)
	}
	if got[1].Number != 2 || got[1].Title != "t2" || got[1].State != "open" || got[1].URL != "https://example.invalid/2" || len(got[1].Labels) != 1 ||
		got[1].Author != "example-verifier-app[bot]" || got[1].AuthorID != 300000005 {
		t.Fatalf("the exact search must return the issue it reconciles to: %+v", got[1])
	}
	want := fmt.Sprintf(`repo:%s is:issue is:open "filing-key: %s" in:body`, forgeTestRepo.Slug(), key)
	for _, q := range queries {
		if q != want {
			t.Fatalf("query %q, want %q", q, want)
		}
	}
}

// regression: example-stream/49
func TestFilingKeySearchGitHubIncomplete(t *testing.T) {
	key := mustKey(t, fkRepo, "example-stream/49", "verifier", "repair/0123456789abcdef")
	for name, body := range map[string]string{
		"partial":        `{"total_count":1,"incomplete_results":true,"items":[{"number":1,"body":"x"}]}`,
		"missing_total":  `{"incomplete_results":false,"items":[]}`,
		"search_ceiling": `{"total_count":1001,"incomplete_results":false,"items":[]}`,
		"early_empty":    `{"total_count":2,"incomplete_results":false,"items":[]}`,
		"pull_request":   `{"total_count":1,"incomplete_results":false,"items":[{"number":1,"body":"x","pull_request":{}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) { fmt.Fprint(w, body) })
			if got, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{FilingKey: key}); err == nil || got != nil {
				t.Fatalf("a partial read became an answer: %+v %v", got, err)
			}
		})
	}
	// A complete empty read is a definite "not filed", distinct from the unknowns above.
	gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		fmt.Fprint(w, `{"total_count":0,"incomplete_results":false,"items":[]}`)
	})
	if got, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{FilingKey: key}); err != nil || len(got) != 0 {
		t.Fatalf("empty complete read: %+v %v", got, err)
	}
	// The mode refuses a malformed key and refuses to combine with free text.
	for _, in := range []SearchIssuesInput{
		{FilingKey: "fk1/zz"},
		{FilingKey: key, Query: "stale oracle"},
		{FilingKey: key, DecisionGateMarkers: true},
	} {
		if _, err := gh.forge().SearchIssues(forgeTestRepo, in); err == nil || !strings.Contains(err.Error(), "filing-key") {
			t.Fatalf("%+v must be refused as a filing-key misuse, got %v", in, err)
		}
	}
}
