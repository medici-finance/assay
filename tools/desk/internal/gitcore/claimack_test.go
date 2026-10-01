package gitcore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
)

func ackPkt(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }

// Real Git supplies advertisements and healthy writes; only the fault responses
// are substituted. No fixture fault performs the requested mutation.
func ackServer(t *testing.T, repo string, response *string, posts *int) string {
	t.Helper()
	path, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: filepath.Join(strings.TrimSpace(string(path)), "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + filepath.Dir(repo), "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
	if b, err := exec.Command("git", "-C", repo, "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		t.Fatalf("config: %v %s", err, b)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			*posts++
			if response != nil {
				io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
				io.WriteString(w, *response)
				return
			}
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL + "/" + filepath.Base(repo)
}

func TestAckHTTPMatrix(t *testing.T) {
	ref := plumbing.ReferenceName("refs/dispatch/example--pr-7--security")
	okLine := ackPkt("ok " + ref.String() + "\n")
	unpack := ackPkt("unpack ok\n")
	reports := map[string]string{
		"empty": "", "unpack-only": unpack + "0000",
		"wrong-ref":      unpack + ackPkt("ok refs/dispatch/other\n") + "0000",
		"duplicate":      unpack + okLine + okLine + "0000",
		"wrong-ref-ng":   unpack + ackPkt("ng refs/dispatch/other failed\n") + "0000",
		"unpack-failure": ackPkt("unpack missing objects\n") + okLine + "0000",
		"truncated":      unpack + okLine, "malformed": ackPkt("invalid\n") + "0000",
		"ng-ok":     unpack + ackPkt("ng "+ref.String()+" ok\n") + "0000",
		"ng-failed": unpack + ackPkt("ng "+ref.String()+" failed\n") + "0000",
	}
	for name, report := range reports {
		for _, op := range []string{"create", "update", "delete"} {
			t.Run(name+"/"+op, func(t *testing.T) {
				repo := bareServer(t)
				objs, newOID, err := MintClaimTag("example", "fixture", time.Unix(1, 0))
				if err != nil {
					t.Fatal(err)
				}
				old := plumbing.ZeroHash
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if op != "create" {
					v, err := PushRefUpdate(ctx, RefUpdate{URL: repo, Ref: ref, New: newOID, Objects: objs})
					if err != nil || v != RefUpdateApplied {
						t.Fatalf("seed: %v %v", v, err)
					}
					old = newOID
				}
				wire := ""
				if report != "" {
					wire = ackPkt("\x01"+report) + "0000"
				}
				posts := 0
				url := ackServer(t, repo, &wire, &posts)
				isNG := strings.HasPrefix(name, "ng-")
				if op == "delete" {
					_, err := DeleteRef(ctx, url, nil, ref)
					_, rejected := err.(*RefRejectedError)
					if err == nil || rejected != isNG {
						t.Fatalf("delete err=%v rejected=%v", err, rejected)
					}
				} else {
					v, err := PushRefUpdateVerdict(ctx, RefUpdate{URL: url, Ref: ref, Old: old, New: newOID, Objects: objs})
					if isNG {
						if err != nil || v.Result != RefUpdateRejected {
							t.Fatalf("expected rejection: %v %v", v, err)
						}
					} else if err == nil {
						t.Fatalf("accepted invalid report: %v", v)
					}
				}
				if posts != 1 {
					t.Fatalf("POSTs=%d, want exactly one", posts)
				}
				b, err := exec.Command("git", "-C", repo, "show-ref", "--verify", "--hash", ref.String()).Output()
				if op == "create" {
					if err == nil {
						t.Fatal("fault fixture unexpectedly created ref")
					}
				} else if err != nil || strings.TrimSpace(string(b)) != old.String() {
					t.Fatalf("fault fixture changed old ref: %s %v", b, err)
				}
			})
		}
	}
}

func TestAckHTTPPositive(t *testing.T) {
	repo := bareServer(t)
	posts := 0
	url := ackServer(t, repo, nil, &posts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ref := plumbing.ReferenceName("refs/dispatch/example--issue-7")
	objs, oid, err := MintClaimTag("example", "fixture", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	u := RefUpdate{URL: url, Ref: ref, New: oid, Objects: objs}
	if v, err := PushRefUpdate(ctx, u); err != nil || v != RefUpdateApplied {
		t.Fatalf("create: %v %v", v, err)
	}
	if v, err := PushRefUpdate(ctx, u); err != nil || v != RefUpdateRejected {
		t.Fatalf("CAS: %v %v", v, err)
	}
	u.Old = oid
	u.Objects, u.New, err = MintClaimTag("example", "advanced", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if v, err := PushRefUpdate(ctx, u); err != nil || v != RefUpdateApplied {
		t.Fatalf("update: %v %v", v, err)
	}
	if v, err := DeleteRef(ctx, url, nil, ref); err != nil || v != DeleteDone {
		t.Fatalf("delete: %v %v", v, err)
	}
	if v, err := DeleteRef(ctx, url, nil, ref); err != nil || v != DeleteAbsent {
		t.Fatalf("absent: %v %v", v, err)
	}
	if posts != 4 {
		t.Fatalf("POSTs=%d want4", posts)
	}
}

func TestAckWireBounds(t *testing.T) {
	ref := plumbing.ReferenceName("refs/dispatch/example")
	inner := ackPkt("unpack ok\n") + ackPkt("ok "+ref.String()+"\n") + "0000"
	for _, band := range []capability.Capability{"", capability.Sideband, capability.Sideband64k} {
		req := packp.NewReferenceUpdateRequest()
		req.Commands = []*packp.Command{{Name: ref}}
		if band != "" {
			req.Capabilities.Set(band)
		}
		wire := inner
		if band != "" {
			wire = ackPkt("\x01"+inner) + "0000"
		}
		body := &ackBody{}
		body.data.WriteString(wire)
		if marker, _, err := wireAck(body, req); err != nil || marker != "ok" {
			t.Fatalf("healthy %s: %s %v", band, marker, err)
		}
		for name, data := range map[string]string{"missing-flush": wire[:len(wire)-4], "trailing": wire + ackPkt("extra"), "oversize": strings.Repeat("x", maxWireAck+1)} {
			body := &ackBody{ReadCloser: io.NopCloser(strings.NewReader(data))}
			io.Copy(io.Discard, body)
			if _, _, err := wireAck(body, req); err == nil {
				t.Errorf("%s/%s accepted", band, name)
			}
		}
		if band != "" {
			body := &ackBody{}
			body.data.WriteString(ackPkt("\x02"+inner) + "0000")
			if _, _, err := wireAck(body, req); err == nil {
				t.Errorf("%s accepted progress as ack", band)
			}
		}
	}
	// A second inner record after the first inner flush must not be ignored.
	reqTail := packp.NewReferenceUpdateRequest()
	reqTail.Commands = []*packp.Command{{Name: ref}}
	tail := &ackBody{}
	tail.data.WriteString(inner + ackPkt("extra") + "0000")
	if _, _, err := wireAck(tail, reqTail); err == nil {
		t.Fatal("accepted trailing inner record")
	}
	// The retained channel-1 limit is independent of the outer wire limit.
	req := packp.NewReferenceUpdateRequest()
	req.Commands = []*packp.Command{{Name: ref}}
	req.Capabilities.Set(capability.Sideband64k)
	huge := ackPkt("unpack ok\n") + ackPkt("ng r "+strings.Repeat("x", 65510)+"\n") + "0000"
	req.Commands[0].Name = "r"
	body := &ackBody{}
	body.data.WriteString(ackPkt("\x01"+huge[:40000]) + ackPkt("\x01"+huge[40000:]) + "0000")
	if _, _, err := wireAck(body, req); err == nil {
		t.Fatal("accepted oversized valid channel1 report")
	}

}

func TestAckNoReportCap(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			return
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
		io.WriteString(w, ackPkt("# service=git-receive-pack\n")+"0000"+ackPkt(strings.Repeat("1", 40)+" refs/dispatch/example\x00delete-refs\n")+"0000")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	objs, oid, err := MintClaimTag("example", "fixture", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	ref := plumbing.ReferenceName("refs/dispatch/example")
	if _, err := PushRefUpdate(ctx, RefUpdate{URL: server.URL, Ref: ref, New: oid, Objects: objs}); err == nil {
		t.Fatal("create accepted missing report-status")
	}
	if _, err := DeleteRef(ctx, server.URL, nil, ref); err == nil {
		t.Fatal("delete accepted missing report-status")
	}
	if posts != 0 {
		t.Fatalf("POSTs=%d", posts)
	}
}

func TestAckIDBounds(t *testing.T) {
	for _, s := range []string{"a\nb", "a b", strings.Repeat("a", 129)} {
		if boundedID(s) != "invalid" {
			t.Fatalf("accepted %q", s)
		}
	}
	if boundedID("AB12:12ef-34") != "AB12:12ef-34" {
		t.Fatal("lost valid ID")
	}
	b := &ackBody{ReadCloser: io.NopCloser(bytes.NewReader(make([]byte, maxWireAck+1)))}
	io.Copy(io.Discard, b)
	if b.data.Len() != maxWireAck || !b.overflow {
		t.Fatal("capture not bounded")
	}
}

func TestAckReceiptIsolation(t *testing.T) {
	// Two concurrent operations get different response correlation IDs. The
	// observer is private to the operation, independent of trace consumers.
	type answer struct {
		receipt RefReceipt
		err     error
	}
	results := make(chan answer, 2)
	for _, key := range []string{"first", "second"} {
		key := key
		go func() {
			var receipt RefReceipt
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
					io.WriteString(w, ackPkt("# service=git-receive-pack\n")+"0000"+ackPkt(strings.Repeat("0", 40)+" capabilities^{}\x00report-status\n")+"0000")
					return
				}
				io.Copy(io.Discard, r.Body)
				w.Header().Set("X-GitHub-Request-Id", key)
				w.Header().Set("Authorization", "Bearer never-copy-this")
				w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
				io.WriteString(w, ackPkt("unpack ok\n")+ackPkt("ng refs/dispatch/"+key+" failed\n")+"0000")
			}))
			defer server.Close()
			objs, oid, err := MintClaimTag(key, "secret-payload-never-copy", time.Unix(1, 0))
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err = PushRefUpdate(ctx, RefUpdate{URL: server.URL, Ref: plumbing.ReferenceName("refs/dispatch/" + key), New: oid, Objects: objs, Trace: func(r RefReceipt) { receipt = r }})
			}
			results <- answer{receipt, err}
		}()
	}
	seen := map[string]bool{}
	for range 2 {
		result := <-results
		r := result.receipt
		if result.err != nil || r.Ref != "refs/dispatch/"+r.RequestID || r.Verdict != "rejected" || r.HTTPStatus != 200 || r.PackObjects != 2 || r.PackBytes == 0 || r.PackSHA256 == "" {
			t.Fatalf("receipt=%+v err=%v", r, result.err)
		}
		if seen[r.RequestID] {
			t.Fatal("receipt crossed operations")
		}
		seen[r.RequestID] = true
		if strings.Contains(fmt.Sprint(r), "never-copy") {
			t.Fatal("payload/header in receipt")
		}
	}
}
