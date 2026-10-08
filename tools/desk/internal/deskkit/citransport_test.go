package deskkit

// citransport_test.go — forge-neutral brief 34: the read-only decorator and the CI-token constructor,
// proven at the deskkit level against a recording forge and an httptest server.

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// ciTestToken is an installation-shaped test token. It is not a credential.
const ciTestToken = "gh" + "s_test_tok"

// countingServer answers every request with an empty JSON list and counts them.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// ciGitHubRoster makes the given repos resolve to GitHub without reading git.
func ciGitHubRoster(t *testing.T, repos ...string) {
	t.Helper()
	r := goldenRoster()
	var parts []string
	for _, repo := range repos {
		parts = append(parts, repo+"=github")
	}
	r[EnvRepoForges] = strings.Join(parts, ",")
	withRoster(t, r)
}

// TestReadOnlyForgeRefusesEveryWriteMethod walks the Forge interface by reflection over the
// decorator wrapping a recording inner forge and checks every method against the obClass table:
// a read reaches the inner forge, anything else returns the decorator's own Refused error and the
// inner forge never sees it. It also asserts the decorator type embeds nothing, so a method added
// to Forge cannot be delegated implicitly.
func TestReadOnlyForgeRefusesEveryWriteMethod(t *testing.T) {
	dt := reflect.TypeOf(readOnlyForge{})
	for i := 0; i < dt.NumField(); i++ {
		if dt.Field(i).Anonymous {
			t.Fatalf("readOnlyForge embeds %s: an embedded field delegates every method the file does not write out, so the decorator would not be default-deny", dt.Field(i).Type)
		}
	}

	rec := &ciRecordingForge{}
	ro := ReadOnly(rec)
	if _, ok := ro.(*readOnlyForge); !ok {
		t.Fatalf("ReadOnly returned %T, want *readOnlyForge", ro)
	}
	rv := reflect.ValueOf(ro)
	it := reflect.TypeOf((*Forge)(nil)).Elem()
	if it.NumMethod() != len(obClass) {
		t.Fatalf("Forge has %d methods but obClass classifies %d — classify the new method", it.NumMethod(), len(obClass))
	}
	reads, writes := 0, 0
	for i := 0; i < it.NumMethod(); i++ {
		m := it.Method(i)
		class, ok := obClass[m.Name]
		if !ok {
			t.Errorf("Forge.%s has no obClass classification", m.Name)
			continue
		}
		if m.Type.IsVariadic() {
			t.Fatalf("Forge.%s is variadic; this walk passes zero values positionally", m.Name)
		}
		args := make([]reflect.Value, m.Type.NumIn())
		for j := range args {
			args[j] = reflect.Zero(m.Type.In(j))
		}
		before := len(rec.called())
		out := rv.MethodByName(m.Name).Call(args)
		reached := len(rec.called()) > before && rec.called()[len(rec.called())-1] == m.Name

		if class == "read" {
			reads++
			if !reached {
				t.Errorf("read method %s did not reach the inner forge", m.Name)
			}
			continue
		}
		writes++
		if reached {
			t.Errorf("write method %s (class %q) reached the inner forge", m.Name, class)
		}
		if len(out) == 0 {
			t.Errorf("write method %s has no result to carry a refusal", m.Name)
			continue
		}
		err, _ := out[len(out)-1].Interface().(error)
		if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), m.Name) || !strings.Contains(err.Error(), "the CI workflow-token transport is read-only") {
			t.Errorf("write method %s: want a Refused error naming the method and the read-only phrase, got %v", m.Name, err)
		}
	}
	if reads == 0 || writes == 0 {
		t.Fatalf("the walk saw %d reads and %d writes — it would be vacuous", reads, writes)
	}
}

// TestCITransportRefusesWriteMethods drives the write methods the brief names, with VALID
// arguments, over a forge built by ReadOnlyForgeForCIToken with its API base on an httptest
// server: each must refuse with the method name and the read-only phrase, and the server must see
// zero requests. Zero-value arguments would not do: the backend refuses several of them itself
// without a request, so a decorator that delegated would pass.
func TestCITransportRefusesWriteMethods(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "a"}
	ciGitHubRoster(t, repo.Slug())
	srv, hits := countingServer(t)
	defer SetCITokenAPIBaseForTest(srv.URL)()

	f, _, err := ReadOnlyForgeForCIToken(repo, repo.Slug(), ciTestToken)
	if err != nil {
		t.Fatalf("ReadOnlyForgeForCIToken: %v", err)
	}
	cases := map[string]func() error{
		"PostComment": func() error { _, e := f.PostComment(repo, 7, "a comment body"); return e },
		"FileIssue":   func() error { _, e := f.FileIssue(repo, IssueInput{Title: "a title", Body: "a body"}); return e },
		"ApplyLabels": func() error {
			_, e := f.ApplyLabels(repo, 7, LabelChange{Target: TargetIssue, Add: []LabelSpec{{Name: "bug", Color: "ff0000"}}})
			return e
		},
		"CloseIssue": func() error { return f.CloseIssue(repo, 7, "completed") },
		"DeleteRef":  func() error { return f.DeleteRef(repo, "heads/feature-x") },
		"EditComment": func() error {
			return f.EditComment(repo, "IC_kwDOAAAAAc4AAAAB", "an edited body")
		},
	}
	for name, call := range cases {
		err := call()
		if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "the CI workflow-token transport is read-only") {
			t.Errorf("%s: want a Refused error naming the method and the read-only phrase, got %v", name, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the server saw %d requests from refused writes, want 0", n)
	}
	// Positive control: a read on the same forge reaches the server, so the zero above is not a
	// server that never answers.
	if _, err := f.ListOpenIssues(repo); err != nil {
		t.Fatalf("ListOpenIssues on the read-only forge: %v", err)
	}
	if hits.Load() == 0 {
		t.Fatal("a read reached no request: the zero count above proves nothing")
	}
}

// TestCITransportPinsAPIBase pins that the CI backend's API host cannot be redirected by the
// environment: with the test seam empty the base is the real API host (read off the built value, no
// request sent); with the seam on server A and GITHUB_API_URL naming server B, the read goes to A.
func TestCITransportPinsAPIBase(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "a"}
	ciGitHubRoster(t, repo.Slug())

	f, _, err := ReadOnlyForgeForCIToken(repo, repo.Slug(), ciTestToken)
	if err != nil {
		t.Fatalf("ReadOnlyForgeForCIToken: %v", err)
	}
	gh := unwrapGitHub(t, f)
	if gh.BaseURL != GitHubAPIBase {
		t.Fatalf("with no seam the CI backend's base is %q, want %q", gh.BaseURL, GitHubAPIBase)
	}
	if gh.Token != ciTestToken {
		t.Fatalf("the CI backend does not carry the handed token")
	}

	srvA, hitsA := countingServer(t)
	srvB, hitsB := countingServer(t)
	t.Setenv("GITHUB_API_URL", srvB.URL)
	t.Setenv("GITHUB_SERVER_URL", srvB.URL)
	defer SetCITokenAPIBaseForTest(srvA.URL)()
	f2, _, err := ReadOnlyForgeForCIToken(repo, repo.Slug(), ciTestToken)
	if err != nil {
		t.Fatalf("ReadOnlyForgeForCIToken: %v", err)
	}
	if _, err := f2.ListOpenIssues(repo); err != nil {
		t.Fatalf("ListOpenIssues: %v", err)
	}
	if hitsA.Load() == 0 {
		t.Fatal("server A (the seam) recorded no request: the precondition failed, so B's zero proves nothing")
	}
	if n := hitsB.Load(); n != 0 {
		t.Fatalf("GITHUB_API_URL's server recorded %d requests: the environment redirected the token", n)
	}
}

// unwrapGitHub digs the *GitHubForge out of the CI constructor's ReadOnly(OutboundChecked(...)).
func unwrapGitHub(t *testing.T, f Forge) *GitHubForge {
	t.Helper()
	ro, ok := f.(*readOnlyForge)
	if !ok {
		t.Fatalf("CI constructor returned %T, want the read-only decorator outermost", f)
	}
	ob, ok := ro.inner.(*outboundForge)
	if !ok {
		t.Fatalf("read-only decorator wraps %T, want the outbound-checked decorator", ro.inner)
	}
	gh, ok := ob.Forge.(*GitHubForge)
	if !ok {
		t.Fatalf("outbound decorator wraps %T, want *GitHubForge", ob.Forge)
	}
	return gh
}

// TestCITransportConstructorRefusals covers the constructor's own checks with deskread bypassed:
// a non-installation token, a repository other than the job's, a non-GitHub forge. Each is Refused
// and the server records zero requests.
func TestCITransportConstructorRefusals(t *testing.T) {
	a := ForgeRepo{Owner: "o", Name: "a"}
	b := ForgeRepo{Owner: "o", Name: "b"}
	ciGitHubRoster(t, a.Slug(), b.Slug())
	srv, hits := countingServer(t)
	defer SetCITokenAPIBaseForTest(srv.URL)()

	for _, tok := range []string{"gh" + "p_x", "gi" + "thub_pat_x", "gh" + "o_x", "ghu_x", "plain", ""} {
		if _, _, err := ReadOnlyForgeForCIToken(a, a.Slug(), tok); err == nil || !IsRefused(err) || !strings.Contains(err.Error(), "not an app installation token") {
			t.Errorf("token %q: want Refused (not an app installation token), got %v", tok, err)
		}
	}
	if _, _, err := ReadOnlyForgeForCIToken(b, a.Slug(), ciTestToken); err == nil || !IsRefused(err) || !strings.Contains(err.Error(), "own repository") {
		t.Errorf("another repository: want Refused naming the binding, got %v", err)
	}
	if _, _, err := ReadOnlyForgeForCIToken(a, "", ciTestToken); err == nil || !IsRefused(err) {
		t.Errorf("an empty job repository: want Refused, got %v", err)
	}
	if _, _, err := ReadOnlyForgeForCIToken(a, "O/A", ciTestToken); err != nil {
		t.Errorf("the job repository compares case-insensitively: %v", err)
	}
	r := goldenRoster()
	r[EnvRepoForges] = "g/lab=gitlab"
	withRoster(t, r)
	gl := ForgeRepo{Owner: "g", Name: "lab"}
	if _, _, err := ReadOnlyForgeForCIToken(gl, gl.Slug(), ciTestToken); err == nil || !IsRefused(err) || !strings.Contains(err.Error(), "GitHub only") {
		t.Errorf("a GitLab repo: want Refused (GitHub only), got %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("refused constructions sent %d requests, want 0", n)
	}
}
