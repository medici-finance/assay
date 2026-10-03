package main

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Real Git and a local TLS server prove which identity reaches the transport.
// All credentials here are placeholders; no external endpoint is contacted.
func TestRoleFetchCredentialIsolation(t *testing.T) {
	for _, lane := range []string{"netrc", "header", "header-global", "header-env", "header-include", "cookie"} {
		t.Run(lane, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			bare := mustGit(t, work, "remote", "get-url", "origin")
			mustGit(t, bare, "update-server-info")
			var mu sync.Mutex
			requests, bad := 0, false
			files := http.FileServer(http.Dir(bare))
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, _ := r.BasicAuth()
				mu.Lock()
				requests++
				if (r.Header.Get("Authorization") != "" && (user != "x-access-token" || pass != fixtureTokenValue)) || r.Header.Get("Cookie") != "" {
					bad = true
				}
				mu.Unlock()
				if user != "x-access-token" || pass != fixtureTokenValue {
					w.Header().Set("WWW-Authenticate", `Basic realm="fixture"`)
					w.WriteHeader(401)
					return
				}
				files.ServeHTTP(w, r)
			}))
			defer srv.Close()
			cert := filepath.Join(t.TempDir(), "ca.pem")
			if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			mustGit(t, work, "config", "--global", "http.sslCAInfo", cert)
			mustGit(t, work, "remote", "set-url", "origin", srv.URL)
			u, _ := url.Parse(srv.URL)
			switch lane {
			case "netrc":
				writeFile(t, filepath.Join(os.Getenv("HOME"), ".netrc"), "machine "+u.Hostname()+" login ambient password placeholder\n")
			case "header":
				mustGit(t, work, "config", "http."+srv.URL+".extraHeader", "Authorization: Basic YW1iaWVudDpwdWJsaWM=")
			case "header-global":
				mustGit(t, work, "config", "--global", "http."+srv.URL+".extraHeader", "Authorization: Basic YW1iaWVudDpwdWJsaWM=")
			case "header-env":
				t.Setenv("GIT_CONFIG_COUNT", "1")
				t.Setenv("GIT_CONFIG_KEY_0", "http."+srv.URL+".extraHeader")
				t.Setenv("GIT_CONFIG_VALUE_0", "Authorization: Basic YW1iaWVudDpwdWJsaWM=")
			case "header-include":
				inc := filepath.Join(t.TempDir(), "included.gitconfig")
				mustGit(t, work, "config", "--file", inc, "http."+srv.URL+".extraHeader", "Authorization: Basic YW1iaWVudDpwdWJsaWM=")
				mustGit(t, work, "config", "include.path", inc)
			case "cookie":
				cookie := filepath.Join(t.TempDir(), "cookies.txt")
				writeFile(t, cookie, "# Netscape HTTP Cookie File\n"+u.Hostname()+"\tFALSE\t/\tTRUE\t2147483647\tsession\tplaceholder\n")
				mustGit(t, work, "config", "http."+srv.URL+".cookieFile", cookie)
			}
			path, err := roleCredentialPath("verifier", "example-org/tracker", srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = fetchRoleHTTPS(work, "verifier", srv.URL, "x-access-token", path)
			mu.Lock()
			gotRequests, gotBad := requests, bad
			mu.Unlock()
			if err != nil {
				t.Errorf("fetch: %v", err)
			}
			if gotRequests == 0 || gotBad {
				t.Fatalf("requests=%d non-role identity observed=%v", gotRequests, gotBad)
			}
		})
	}
}
