package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// glRecordFake is a stateful GitLab instance for one pre-work attestation record, issue 41,
// authored by the bound dispatcher. It stores a created description the way GitLab does:
// WITHOUT its trailing newline (#6085), so a reader comparing byte for byte against the
// rendering it filed refuses its own record. edited plants a real description-change note.
type glRecordFake struct {
	srv          *httptest.Server
	title, body  string
	state        string
	labels       []string
	events       []map[string]any
	paths        []string
	edited       bool
	createdBytes string
}

func newGLRecordFake(t *testing.T) *glRecordFake {
	t.Helper()
	g := &glRecordFake{state: "opened"}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.paths = append(g.paths, r.Method+" "+r.URL.Path)
		encode := func(v any) { json.NewEncoder(w).Encode(v) }
		issue := func() map[string]any {
			return map[string]any{"id": 41, "iid": 41, "title": g.title, "description": g.body, "state": g.state, "labels": g.labels, "web_url": "https://example.invalid/attestation/41", "author": map[string]any{"id": 1, "username": "example-desk"}}
		}
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/api/graphql"):
			nodes := []any{map[string]any{"body": "closed", "createdAt": "2026-09-01T10:00:00Z"}}
			if g.edited {
				nodes = append(nodes, map[string]any{"body": "changed the description", "createdAt": "2026-09-01T10:01:00Z"})
			}
			encode(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{"comments": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}}, "activity": map[string]any{"nodes": nodes, "pageInfo": map[string]any{"hasPreviousPage": false}}}}}})
		case r.Method == "POST" && strings.HasSuffix(p, "/issues/41/notes"):
			w.WriteHeader(201)
			encode(map[string]any{"id": 1, "body": "attestation complete"})
		case r.Method == "POST" && strings.HasSuffix(p, "/issues"):
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			g.title, _ = in["title"].(string)
			g.createdBytes, _ = in["description"].(string)
			// GitLab keeps the description without its trailing newline.
			g.body = strings.TrimSuffix(g.createdBytes, "\n")
			w.WriteHeader(201)
			encode(issue())
		case r.Method == "GET" && strings.HasPrefix(p, "/api/v4/users/"):
			encode(map[string]any{"id": 1, "username": "example-desk", "bot": true})
		case r.Method == "GET" && strings.HasSuffix(p, "/issues/41"):
			encode(issue())
		case r.Method == "GET" && strings.HasSuffix(p, "/issues/41/resource_label_events"):
			if g.events == nil {
				encode([]any{})
			} else {
				encode(g.events)
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
				g.state = "closed"
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
						for _, l := range g.labels {
							if l != name {
								kept = append(kept, l)
							}
						}
						g.labels = kept
					} else {
						g.labels = append(g.labels, name)
					}
					g.events = append(g.events, map[string]any{"id": len(g.events) + 1, "label": map[string]any{"name": name}, "action": action, "user": map[string]any{"username": "example-desk", "id": 1}, "created_at": "2026-09-01T10:00:00Z"})
				}
			}
			encode(issue())
		default:
			t.Errorf("unexpected provider endpoint %s %s", r.Method, p)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *glRecordFake) forge() *GitLabForge {
	return &GitLabForge{Token: glTestToken, BaseURL: g.srv.URL, Client: g.srv.Client()}
}

// checkStored pins the fake's premise: the record was filed with its trailing newline and is
// read back without it, so the round trip below exercises the dropped byte.
func (g *glRecordFake) checkStored(t *testing.T) {
	t.Helper()
	if !strings.HasSuffix(g.createdBytes, "}\n") || g.body+"\n" != g.createdBytes {
		t.Fatalf("the fake did not model GitLab's dropped trailing newline: filed %q, stored %q", recordTail(g.createdBytes), recordTail(g.body))
	}
}

func recordTail(s string) string {
	if len(s) > 8 {
		return s[len(s)-8:]
	}
	return s
}

func (g *glRecordFake) noMergeRequestRoute(t *testing.T) {
	t.Helper()
	for _, path := range g.paths {
		if strings.Contains(path, "merge_requests") {
			t.Fatalf("issue attestation used MR endpoint: %s", path)
		}
	}
}

// regression: #6085
func TestVerifierAttestationGitLabIssueContract(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6.1-sol")
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=gitlab:example-desk:1,verifier=gitlab:example-verifier:2\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	g := newGLRecordFake(t)
	f := g.forge()
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	g.checkStored(t)
	if receipt.Issue != 41 || receipt.Binding.Model != "gpt-6.1-sol" {
		t.Fatalf("bad receipt %+v", receipt)
	}
	g.noMergeRequestRoute(t)
	// The retained home reads the same record back through the same admission path.
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
		t.Fatalf("GitLab record read back from the retained home refused: %v", err)
	}
	// Exactly one dropped newline is tolerated: any other byte change still refuses, and the
	// refusal names the specific reason.
	stored := g.body
	g.body = stored + " "
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil || !strings.Contains(err.Error(), "not the exact rendering of its binding") {
		t.Fatalf("a GitLab body with a trailing space admitted or refused without its reason: %v", err)
	}
	g.body = stored
	// Closing updates the issue but is not a description edit. A real description
	// system note is a refusal, using the existing supported trust-events contract.
	g.edited = true
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
		t.Fatal("edited GitLab description admitted")
	}
}

// regression: #6085
func TestWorkerAttestationGitLabIssueContract(t *testing.T) {
	home, brief := workerFixture(t)
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=gitlab:example-desk:1,worker=gitlab:example-worker:3\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	g := newGLRecordFake(t)
	f := g.forge()
	b, err := PrepareWorkerBinding(f, workerInput(home, brief))
	if err != nil {
		t.Fatal(err)
	}
	r, err := IssueWorkerAttestation(f, b)
	if err != nil {
		t.Fatal(err)
	}
	g.checkStored(t)
	if r.Issue != 41 || r.Binding != b || g.state != "closed" {
		t.Fatalf("bad receipt %+v state=%s", r, g.state)
	}
	g.noMergeRequestRoute(t)
	repo := ForgeRepo{Owner: "example-org", Name: "one"}
	if got, err := ReadWorkerAttestation(f, repo, 41); err != nil || got.Binding != b {
		t.Fatalf("GitLab worker record read back refused: %+v %v", got, err)
	}
	stored := g.body
	// Each body still decodes to the same binding, so only the rendering check can refuse it.
	for name, body := range map[string]string{
		"one other byte":        strings.Replace(stored, `"Version": 1`, `"Version":  1`, 1),
		"a trailing CRLF":       stored + "\r\n",
		"two trailing newlines": stored + "\n\n",
	} {
		g.body = body
		_, err := ReadWorkerAttestation(f, repo, 41)
		if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "worker target is not the exact dispatcher-authored run record: has a body that is not the exact rendering of its binding") {
			t.Errorf("%s: admitted or refused without its reason: %v", name, err)
		}
	}
	g.body = stored
	g.edited = true
	if _, err := ReadWorkerAttestation(f, repo, 41); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("edited GitLab worker record admitted: %v", err)
	}
}
func TestAttestationExcludedDuringOpenWindowBothForges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "number": 1, "iid": 1, "title": VerifierAttestationTitle + "fixture-run", "state": "opened", "user": map[string]any{"login": "example-desk[bot]"}, "author": map[string]any{"username": "example-desk"}}, {"id": 2, "number": 2, "iid": 2, "title": "ordinary work", "state": "opened", "user": map[string]any{"login": "example-human"}, "author": map[string]any{"username": "example-human"}}, {"id": 3, "number": 3, "iid": 3, "title": VerifierAttestationTitle + "ordinary-user", "user": map[string]any{"login": "example-human"}, "author": map[string]any{"username": "example-human"}}, {"id": 4, "number": 4, "iid": 4, "title": WorkerAttestationTitle + "fixture-run", "state": "opened", "user": map[string]any{"login": "example-desk[bot]"}, "author": map[string]any{"username": "example-desk"}}, {"id": 5, "number": 5, "iid": 5, "title": WorkerAttestationTitle + "ordinary-user", "user": map[string]any{"login": "example-human"}, "author": map[string]any{"username": "example-human"}}})
	}))
	defer srv.Close()
	repo := ForgeRepo{Owner: "example-org", Name: "one"}
	for name, f := range map[string]Forge{"github": &GitHubForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}, "gitlab": &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}} {
		provider := ""
		if name == "gitlab" {
			provider = "gitlab:"
		}
		plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk="+provider+"example-desk:1,verifier="+provider+"example-verifier:2,worker="+provider+"example-worker:3\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
		got, err := f.ListOpenIssues(repo)
		if err != nil {
			t.Fatalf("%s %v", name, err)
		}
		// The verifier and worker records authored by the bound dispatcher are excluded even while
		// open; a look-alike title by anyone else stays ordinary work.
		if len(got) != 3 || got[0].Title != "ordinary work" || got[1].Number != 3 || got[2].Number != 5 {
			t.Fatalf("%s intake included run record: %+v", name, got)
		}
	}
}
