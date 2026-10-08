package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifierAttestationGitLabIssueContract(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6.1-sol")
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=gitlab:example-desk:1,verifier=gitlab:example-verifier:2\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	title, body, state := "", "", "opened"
	var labels []string
	var events []map[string]any
	var paths []string
	edited := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		encode := func(v any) { json.NewEncoder(w).Encode(v) }
		issue := func() map[string]any {
			return map[string]any{"id": 41, "iid": 41, "title": title, "description": body, "state": state, "labels": labels, "web_url": "https://example.invalid/attestation/41", "author": map[string]any{"id": 1, "username": "example-desk"}}
		}
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/api/graphql"):
			nodes := []any{map[string]any{"body": "closed", "createdAt": "2026-09-01T10:00:00Z"}}
			if edited {
				nodes = append(nodes, map[string]any{"body": "changed the description", "createdAt": "2026-09-01T10:01:00Z"})
			}
			encode(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{"comments": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}}, "activity": map[string]any{"nodes": nodes, "pageInfo": map[string]any{"hasPreviousPage": false}}}}}})
		case r.Method == "POST" && strings.HasSuffix(p, "/issues/41/notes"):
			w.WriteHeader(201)
			encode(map[string]any{"id": 1, "body": "attestation complete"})
		case r.Method == "POST" && strings.HasSuffix(p, "/issues"):
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			title, _ = in["title"].(string)
			body, _ = in["description"].(string)
			w.WriteHeader(201)
			encode(issue())
		case r.Method == "GET" && strings.HasPrefix(p, "/api/v4/users/"):
			encode(map[string]any{"id": 1, "username": "example-desk", "bot": true})
		case r.Method == "GET" && strings.HasSuffix(p, "/issues/41"):
			encode(issue())
		case r.Method == "GET" && strings.HasSuffix(p, "/issues/41/resource_label_events"):
			if events == nil {
				encode([]any{})
			} else {
				encode(events)
			}
		case r.Method == "GET" && strings.HasSuffix(p, "/labels"):
			encode([]any{})
		case r.Method == "POST" && strings.HasSuffix(p, "/labels"):
			w.WriteHeader(201)
			encode(map[string]any{"id": 1})
		case r.Method == "PUT" && strings.HasSuffix(p, "/issues/41"):
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if in["state_event"] == "close" {
				state = "closed"
			}
			for _, key := range []string{"remove_labels", "add_labels"} {
				raw, _ := in[key].(string)
				for _, name := range strings.Split(raw, ",") {
					if name == "" {
						continue
					}
					action := "add"
					if key == "remove_labels" {
						action = "remove"
						var kept []string
						for _, l := range labels {
							if l != name {
								kept = append(kept, l)
							}
						}
						labels = kept
					} else {
						labels = append(labels, name)
					}
					events = append(events, map[string]any{"id": len(events) + 1, "label": map[string]any{"name": name}, "action": action, "user": map[string]any{"username": "example-desk", "id": 1}, "created_at": "2026-09-01T10:00:00Z"})
				}
			}
			encode(issue())
		default:
			t.Errorf("unexpected provider endpoint %s %s", r.Method, p)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Issue != 41 || receipt.Binding.Model != "gpt-6.1-sol" {
		t.Fatalf("bad receipt %+v", receipt)
	}
	for _, path := range paths {
		if strings.Contains(path, "merge_requests") {
			t.Fatalf("issue attestation used MR endpoint: %s", path)
		}
	}
	// Closing updates the issue but is not a description edit. A real description
	// system note is a refusal, using the existing supported trust-events contract.
	edited = true
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
		t.Fatal("edited GitLab description admitted")
	}
}
func TestAttestationExcludedDuringOpenWindowBothForges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "number": 1, "iid": 1, "title": VerifierAttestationTitle + "fixture-run", "state": "opened", "user": map[string]any{"login": "example-desk[bot]"}, "author": map[string]any{"username": "example-desk"}}, {"id": 2, "number": 2, "iid": 2, "title": "ordinary work", "state": "opened", "user": map[string]any{"login": "example-human"}, "author": map[string]any{"username": "example-human"}}, {"id": 3, "number": 3, "iid": 3, "title": VerifierAttestationTitle + "ordinary-user", "user": map[string]any{"login": "example-human"}, "author": map[string]any{"username": "example-human"}}})
	}))
	defer srv.Close()
	repo := ForgeRepo{Owner: "example-org", Name: "one"}
	for name, f := range map[string]Forge{"github": &GitHubForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}, "gitlab": &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}} {
		provider := ""
		if name == "gitlab" {
			provider = "gitlab:"
		}
		plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk="+provider+"example-desk:1,verifier="+provider+"example-verifier:2\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
		got, err := f.ListOpenIssues(repo)
		if err != nil {
			t.Fatalf("%s %v", name, err)
		}
		if len(got) != 2 || got[0].Title != "ordinary work" || got[1].Number != 3 {
			t.Fatalf("%s intake included run record: %+v", name, got)
		}
	}
}
