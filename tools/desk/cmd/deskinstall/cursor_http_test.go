package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
)

// cursorHTTPTransport redirects only the selected fixture asset to a local
// server while retaining the production client's HTTP reader and size checks.
type cursorHTTPTransport struct {
	target *url.URL
	next   *http.Transport
}

func (r cursorHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "github.com" || !strings.HasSuffix(req.URL.Path, "/"+cursorbundle.AssetName) {
		return nil, fmt.Errorf("unexpected fixture request: %s", req.URL)
	}
	clone := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = r.target.Scheme
	u.Host = r.target.Host
	clone.URL = &u
	return r.next.RoundTrip(clone)
}

// regression: #2701, F-bundle-fetch-size
func TestCursorLargeHTTPBundle(t *testing.T) {
	payload := make([]byte, 2<<20)
	_, _ = rand.New(rand.NewSource(2701)).Read(payload)
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	mustWrite(t, filepath.Join(bundle, ".claude-plugin", "plugin.json"), `{"version":"1.2.3"}`)
	mustWrite(t, filepath.Join(bundle, "skills", "adopt", "support.bin"), string(payload))
	var packed bytes.Buffer
	if err := cursorbundle.Pack(bundle, "v1.2.3", &packed); err != nil {
		t.Fatal(err)
	}
	raw := packed.Bytes()
	repo, releases := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(releases, "v1.2.3.checksums.txt"), strings.Repeat("a", 64)+"  statusgen-linux-amd64\n"+fmt.Sprintf("%x  %s\n", sha256.Sum256(raw), cursorbundle.AssetName))
	args := []string{"--harness", "cursor", "--release", "v1.2.3", "--releases", releases, "--repo", repo, "--forge", "github", "--fetch"}
	if len(raw) <= 1<<20 || len(raw) > cursorbundle.MaxArchiveBytes {
		t.Fatalf("fixture compressed size %d", len(raw))
	}
	t.Logf("valid packed archive: %d bytes", len(raw))
	served := raw
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		body := served
		mu.Unlock()
		w.(http.Flusher).Flush() // unknown Content-Length exercises streaming bound
		_, _ = w.Write(body)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	old := http.DefaultTransport
	http.DefaultTransport = cursorHTTPTransport{target, transport}
	t.Cleanup(func() { http.DefaultTransport = old })
	before := treeSnapshot(t, repo)
	for _, mode := range []string{"--dry-run", "", "--check"} {
		next := append([]string{}, args...)
		if mode != "" {
			next = append(next, mode)
		}
		var out bytes.Buffer
		if code := run(next, &out, &out); code != 0 {
			t.Fatalf("%s exit %d: %s", mode, code, &out)
		}
		if mode == "--dry-run" && !(reflect.DeepEqual(before, treeSnapshot(t, repo))) {
			t.Fatal("preview changed adopter")
		}
		if mode == "--dry-run" {
			if _, err := os.Stat(filepath.Join(repo, ".cursor")); !os.IsNotExist(err) {
				t.Fatalf("preview wrote adopter: %v", err)
			}
		}
	}
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 3 {
		t.Fatalf("wanted real fetch in preview/apply/check, got %d", gotCalls)
	}
	got, err := os.ReadFile(filepath.Join(repo, ".cursor", "skills", "adopt", "support.bin"))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("downloaded support bytes differ: %v", err)
	}
	mu.Lock()
	before = treeSnapshot(t, repo)
	served = make([]byte, cursorbundle.MaxArchiveBytes+1)
	mu.Unlock()
	for _, mode := range []string{"--dry-run", "", "--check"} {
		next := append([]string{}, args...)
		if mode != "" {
			next = append(next, mode)
		}
		var out bytes.Buffer
		if code := run(next, &out, &out); code == 0 || !strings.Contains(out.String(), "exceeds") {
			t.Fatalf("over-limit %s exit %d: %s", mode, code, &out)
		}
		if !(reflect.DeepEqual(before, treeSnapshot(t, repo))) {
			t.Fatal("oversized download changed adopter")
		}
		got, err = os.ReadFile(filepath.Join(repo, ".cursor", "skills", "adopt", "support.bin"))
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("refusal changed adopter: %v", err)
		}
	}
}
