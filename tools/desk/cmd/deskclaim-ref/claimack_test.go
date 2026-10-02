package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"io"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaimTrace(t *testing.T) {
	repo := refusingServer(t)
	installHook(t, repo)
	b, e := exec.Command("git", "--exec-path").Output()
	if e != nil {
		t.Fatal(e)
	}
	backend := &cgi.Handler{Path: filepath.Join(strings.TrimSpace(string(b)), "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + filepath.Dir(repo), "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
	if b, e := exec.Command("git", "-C", repo, "config", "http.receivepack", "true").CombinedOutput(); e != nil {
		t.Fatalf("config: %v %s", e, b)
	}
	posts := 0
	var packHash string
	var objectCount uint32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", "ABCD:1234:5678:9ABC:12345678")
		if r.Method == "POST" {
			posts++
			data, e := io.ReadAll(r.Body)
			if e != nil {
				t.Error(e)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
			i := bytes.Index(data, []byte("PACK"))
			if i >= 0 {
				packHash = fmt.Sprintf("%x", sha256.Sum256(data[i:]))
				objectCount = binary.BigEndian.Uint32(data[i+8 : i+12])
			}
		}
		backend.ServeHTTP(w, r)
	}))
	defer srv.Close()
	old := errOut
	var diagnostics bytes.Buffer
	errOut = &diagnostics
	t.Cleanup(func() { errOut = old })
	deskkit.SetTrace(true)
	t.Cleanup(deskkit.ResetTrace)
	g := &gogitStore{url: srv.URL + "/" + filepath.Base(repo), host: "loopback"}
	id := "example--pr-7--security"
	got := g.CreateIfAbsent(id, claimMessage(id, "fixture", "claimed", "-", ""))
	if got != deskkit.ClaimWriteRejected || posts != 1 || objectCount != 2 || !strings.Contains(g.TransportCause(), hookSays) {
		t.Fatalf("fixture failed: result=%v posts=%d objects=%d cause=%s", got, posts, objectCount, g.TransportCause())
	}
	t.Logf("one rejected POST; objects=%d pack_sha256=%s cause=%s diagnostics=%q", objectCount, packHash, g.TransportCause(), diagnostics.String())
	for _, required := range []string{"pack_sha256=" + packHash, "pack_objects=2", "verdict=rejected", "http_status=200", "new=", "ABCD:1234:5678:9ABC:12345678"} {
		if !strings.Contains(diagnostics.String(), required) {
			t.Errorf("DESK_TRACE missing request evidence %q", required)
		}
	}
}

func TestClaimEmptyAck(t *testing.T) {
	repo := refusingServer(t)
	b, e := exec.Command("git", "--exec-path").Output()
	if e != nil {
		t.Fatal(e)
	}
	backend := &cgi.Handler{Path: filepath.Join(strings.TrimSpace(string(b)), "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + filepath.Dir(repo), "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
	if b, e := exec.Command("git", "-C", repo, "config", "http.receivepack", "true").CombinedOutput(); e != nil {
		t.Fatalf("config: %v %s", e, b)
	}
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
			w.WriteHeader(http.StatusOK)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	defer srv.Close()
	g := &gogitStore{url: srv.URL + "/" + filepath.Base(repo), host: "loopback"}
	id := "example--pr-7--security"
	got := g.CreateIfAbsent(id, claimMessage(id, "fixture", "claimed", "-", ""))
	_, referr := exec.Command("git", "-C", repo, "show-ref", "--verify", "refs/dispatch/"+id).CombinedOutput()
	if posts != 1 || referr == nil {
		t.Fatal("invalid empty-response fixture")
	}
	if got != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("empty HTTP acknowledgment accepted: outcome=%v (Applied=%v), posts=%d, actual ref absent", got, deskkit.ClaimWriteApplied, posts)
	}
}

func TestClaimAckMatrix(t *testing.T) {
	id := "example--pr-7--security"
	ref := "refs/dispatch/" + id
	pkt := func(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }
	reports := map[string]string{
		"empty":             "",
		"unpack-only":       pkt("unpack ok\n") + "0000",
		"wrong-ref":         pkt("unpack ok\n") + pkt("ok refs/dispatch/unrelated\n") + "0000",
		"duplicate":         pkt("unpack ok\n") + pkt("ok "+ref+"\n") + pkt("ok "+ref+"\n") + "0000",
		"wrong-ref-refusal": pkt("unpack ok\n") + pkt("ng refs/dispatch/unrelated failed\n") + "0000",
		"unpack-failed":     pkt("unpack missing objects\n") + "0000",
		"truncated":         pkt("unpack ok\n") + pkt("ok "+ref+"\n"),
		"malformed":         pkt("invalid unpack record\n") + "0000",
	}
	for label, report := range reports {
		t.Run(label, func(t *testing.T) {
			repo := refusingServer(t)
			b, e := exec.Command("git", "--exec-path").Output()
			if e != nil {
				t.Fatal(e)
			}
			backend := &cgi.Handler{Path: filepath.Join(strings.TrimSpace(string(b)), "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + filepath.Dir(repo), "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
			if b, e := exec.Command("git", "-C", repo, "config", "http.receivepack", "true").CombinedOutput(); e != nil {
				t.Fatalf("config: %v %s", e, b)
			}
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					posts++
					io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
					w.WriteHeader(http.StatusOK)
					if report != "" {
						io.WriteString(w, pkt("\x01"+report)+"0000")
					}
					return
				}
				backend.ServeHTTP(w, r)
			}))
			defer srv.Close()
			g := &gogitStore{url: srv.URL + "/" + filepath.Base(repo), host: "loopback"}
			got := g.CreateIfAbsent(id, claimMessage(id, "fixture", "claimed", "-", ""))
			if posts != 1 {
				t.Fatal("invalid fixture")
			}
			if got != deskkit.ClaimWriteUnverifiable {
				t.Errorf("untrustworthy acknowledgment outcome=%v wantUnverifiable", got)
			}
		})
	}
}

func TestClaimTraceScrub(t *testing.T) {
	old := errOut
	var b bytes.Buffer
	errOut = &b
	t.Cleanup(func() { errOut = old; deskkit.ResetTrace() })
	canary := "ghp_" + strings.Repeat("x", 36)
	receipt := gitcore.RefReceipt{Ref: "refs/dispatch/" + canary + strings.Repeat("r", 10000), RequestID: canary, Old: strings.Repeat("0", 40), New: strings.Repeat("1", 40)}
	deskkit.SetTrace(false)
	claimTrace(receipt)
	if b.Len() != 0 {
		t.Fatal("trace emitted while disabled")
	}
	deskkit.SetTrace(true)
	claimTrace(receipt)
	if strings.Contains(b.String(), canary) || !strings.Contains(b.String(), "<redacted>") || b.Len() > 1500 {
		t.Fatalf("unsafe trace: size=%d", b.Len())
	}
}
