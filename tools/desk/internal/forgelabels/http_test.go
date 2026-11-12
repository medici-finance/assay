package forgelabels

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewHTTPTransportCarriesTheCredentialAsAHeaderOnly(t *testing.T) {
	var gotAuth, gotPrivate, gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPrivate, gotURL = r.Header.Get("Authorization"), r.Header.Get("PRIVATE-TOKEN"), r.URL.String()
		w.WriteHeader(200)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	for _, forge := range []string{"github", "gitlab"} {
		tr, err := NewHTTPTransport(forge, srv.URL+"/", srv.Client(), "s3cret-example")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Do("GET", "/labels?page=1", nil); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(gotURL, "s3cret-example") {
			t.Errorf("%s: the credential reached the URL: %s", forge, gotURL)
		}
		switch forge {
		case "github":
			if gotAuth != "Bearer s3cret-example" || gotPrivate != "" {
				t.Errorf("github headers: Authorization=%q PRIVATE-TOKEN=%q", gotAuth, gotPrivate)
			}
		case "gitlab":
			if gotPrivate != "s3cret-example" || gotAuth != "" {
				t.Errorf("gitlab headers: Authorization=%q PRIVATE-TOKEN=%q", gotAuth, gotPrivate)
			}
		}
	}
}

// A redirect is returned as the answer, never followed: the follow would carry PRIVATE-TOKEN
// to whatever host the Location names.
func TestNewHTTPTransportRefusesRedirects(t *testing.T) {
	followed := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	defer srv.Close()
	tr, err := NewHTTPTransport("gitlab", srv.URL, srv.Client(), "s3cret-example")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tr.Do("GET", "/x", nil)
	if err != nil || resp.Status != http.StatusFound || followed {
		t.Errorf("redirect: status=%d err=%v followed=%v; want the 302 returned, not followed", resp.Status, err, followed)
	}
}

func TestValidateAPIBase(t *testing.T) {
	for _, ok := range []string{"https://api.github.com", "http://127.0.0.1:8080", "http://localhost/api/v4"} {
		if err := ValidateAPIBase("b", ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://gitlab.example.com/api/v4", "ftp://x", "not a url", ""} {
		if err := ValidateAPIBase("b", bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if _, err := NewHTTPTransport("github", "http://api.example.com", nil, "t"); err == nil {
		t.Error("NewHTTPTransport accepted a cleartext non-loopback base")
	}
}

func TestReadCredentialFile(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "good")
	if err := os.WriteFile(good, []byte("  tok-example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if tok, err := ReadCredentialFile("F", good); err != nil || tok != "tok-example" {
		t.Errorf("ReadCredentialFile(good) = %q, %v", tok, err)
	}
	empty := filepath.Join(d, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"unset": "", "missing": filepath.Join(d, "none"), "directory": d, "empty": empty} {
		if _, err := ReadCredentialFile("F", p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if runtime.GOOS != "windows" {
		wide := filepath.Join(d, "wide")
		if err := os.WriteFile(wide, []byte("tok"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCredentialFile("F", wide); err == nil {
			t.Error("a group/world-readable credential file was accepted")
		}
	}
}
