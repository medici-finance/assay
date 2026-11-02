package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #5359
func TestForgeReadDelay(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, tc := range []struct {
		name   string
		status int
		h      http.Header
		want   time.Duration
	}{
		{"primary", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1010"}}, 11 * time.Second},
		{"secondary", 429, http.Header{"Retry-After": {"4"}}, 4 * time.Second},
		{"secondary403", 403, http.Header{"Retry-After": {"4"}}, 4 * time.Second},
		{"http date", 429, http.Header{"Retry-After": {now.Add(4 * time.Second).UTC().Format(http.TimeFormat)}}, 5 * time.Second},
		{"past reset", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"999"}}, time.Second},
		{"permission", 403, http.Header{}, 0},
		{"bad credential", 401, http.Header{"Retry-After": {"4"}}, 0},
		{"success", 200, http.Header{"Retry-After": {"4"}}, 0},
		{"invalid reset", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"no"}}, 0},
		{"unexhausted", 403, http.Header{"X-Ratelimit-Remaining": {"1"}, "X-Ratelimit-Reset": {"1010"}}, 0},
		{"invalid retry", 429, http.Header{"Retry-After": {"-2"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := forgeReadDelay(tc.status, tc.h, now); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestForgeReadHTTP(t *testing.T) {
	for _, mode := range []string{"reset then success", "repeat denial", "permission", "wait exceeds budget", "cancelled", "post"} {
		t.Run(mode, func(t *testing.T) {
			calls, waits := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer same-credential" {
					t.Error("credential changed")
				}
				if calls == 1 || mode == "repeat denial" {
					if mode != "permission" {
						w.Header().Set("X-RateLimit-Remaining", "0")
						w.Header().Set("X-RateLimit-Reset", "1010")
					}
					w.WriteHeader(403)
					fmt.Fprint(w, `{"message":"denied"}`)
					return
				}
				fmt.Fprint(w, `{"result":"observed"}`)
			}))
			defer srv.Close()
			budget := &forgeReadWait{left: time.Minute}
			if mode == "wait exceeds budget" {
				budget.left = time.Second
			}
			d := &forgeReadDoer{inner: srv.Client(), budget: budget, now: func() time.Time { return time.Unix(1000, 0) }, wait: func(ctx context.Context, d time.Duration) error {
				waits++
				if d != 11*time.Second {
					t.Errorf("wait %v", d)
				}
				if mode == "cancelled" {
					return context.Canceled
				}
				return nil
			}}
			method := http.MethodGet
			if mode == "post" {
				method = http.MethodPost
			}
			req, _ := http.NewRequest(method, srv.URL+"/same-read", nil)
			req.Header.Set("Authorization", "Bearer same-credential")
			resp, err := d.Do(req)
			if mode == "cancelled" {
				if !errors.Is(err, context.Canceled) || calls != 1 {
					t.Fatalf("err=%v calls=%d", err, calls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			wantCalls, wantWait, wantStatus := 1, 0, 403
			if mode == "reset then success" || mode == "repeat denial" {
				wantCalls, wantWait = 2, 1
			}
			if mode == "reset then success" {
				wantStatus = 200
				if !strings.Contains(string(body), "observed") {
					t.Fatal(string(body))
				}
			}
			if calls != wantCalls || waits != wantWait || resp.StatusCode != wantStatus {
				t.Fatalf("calls=%d waits=%d status=%d", calls, waits, resp.StatusCode)
			}
		})
	}
}

func TestForgeReadBudgetSharedAcrossRequests(t *testing.T) {
	b := &forgeReadWait{left: 20 * time.Second}
	if !b.reserve(11*time.Second) || b.reserve(11*time.Second) || b.reserve(0) || b.reserve(-time.Second) {
		t.Fatal("unbounded/shared wait budget")
	}
}

func TestForgeReadCLI(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("fixture shell is POSIX-only")
	}
	for _, mode := range []string{"reset then success", "repeat denial", "permission", "quota unknown", "not CI", "oversized reset", "credential changed", "graphql reset", "quota missing", "probe failed", "wait budget"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			count := filepath.Join(dir, "count")
			script := `#!/bin/sh
[ "$GH_TOKEN" = test-secret-value ] || { echo "credential changed" >&2; exit 9; }
if [ "$2" = rate_limit ]; then
 case "$MODE" in
  'probe failed') exit 1;;
  'quota missing') echo '{"resources":{"core":{"reset":1010}}}';;
  'graphql reset') echo '{"resources":{"core":{"remaining":100,"reset":9999999999},"graphql":{"remaining":0,"reset":1010}}}';;
  'quota unknown') echo '{"resources":{"core":{"remaining":1,"reset":1010}}}';;
  'oversized reset') echo '{"resources":{"core":{"remaining":0,"reset":9999999999}}}';;
  *) echo '{"resources":{"core":{"remaining":0,"reset":1010}}}';;
 esac
 exit 0
fi
n=0
[ ! -f "$COUNT" ] || n=$(cat "$COUNT")
n=$((n+1));echo "$n" > "$COUNT"
if [ "$MODE" = permission ]; then echo 'gh: Resource not accessible by integration (HTTP 403)' >&2;exit 1;fi
if [ "$n" = 1 ] || [ "$MODE" = 'repeat denial' ];then echo "gh: API rate limit exceeded (HTTP 403) credential=$GH_TOKEN" >&2;exit 1;fi
echo '[{"observed":true}]'
`
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("COUNT", count)
			t.Setenv("MODE", mode)
			t.Setenv("GH_TOKEN", "test-secret-value")
			t.Setenv("GH_HOST", "")
			t.Setenv("GITHUB_ACTIONS", "true")
			if mode == "not CI" {
				t.Setenv("GITHUB_ACTIONS", "false")
			}
			waits := 0
			made := 0
			resource := "core"
			if mode == "graphql reset" {
				resource = "graphql"
			}
			budget := &forgeReadWait{left: time.Minute}
			if mode == "wait budget" {
				budget.left = time.Second
			}
			out, err := forgeCLIReadWith(resource, func() *exec.Cmd {
				made++
				cmd := ghGuard(exec.Command("gh", "api", "repos/example/project/pulls/1/files"))
				if mode == "credential changed" && made > 1 {
					cmd.Env = append(os.Environ(), "GH_TOKEN=second-secret")
				}
				return cmd
			}, budget, func() time.Time { return time.Unix(1000, 0) }, func(context.Context, time.Duration) error { waits++; return nil })
			data, _ := os.ReadFile(count)
			calls := strings.TrimSpace(string(data))
			if mode == "reset then success" || mode == "credential changed" || mode == "graphql reset" {
				if err != nil || waits != 1 || calls != "2" || !strings.Contains(string(out), "observed") {
					t.Fatalf("out=%s err=%v waits=%d calls=%s", out, err, waits, calls)
				}
				return
			}
			wantCalls, wantWait := "1", 0
			if mode == "repeat denial" {
				wantCalls, wantWait = "2", 1
			}
			if err == nil || calls != wantCalls || waits != wantWait {
				t.Fatalf("err=%v waits=%d calls=%s", err, waits, calls)
			}
			if strings.Contains(err.Error(), "test-secret-value") {
				t.Fatal("credential in diagnostic")
			}
			if !strings.Contains(err.Error(), "HTTP 403") {
				t.Fatal("actual provider denial discarded:", err)
			}
		})
	}
}
