package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigureApprovals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		degrade bool
	}{
		{"gitlab_ce_404_degrades", 404, true}, {"gitlab_401_still_fails", 401, false},
		{"gitlab_403_still_fails", 403, false}, {"gitlab_500_still_fails", 500, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				if r.Method != "POST" || r.URL.Path != "/projects/7/approvals" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, `{"message":"synthetic error"}`)
			}))
			defer srv.Close()
			out, errs := &bytes.Buffer{}, &bytes.Buffer{}
			p := &provisioner{e: &env{stdout: out, stderr: errs}, gl: newGitLabClient(srv.URL, srv.Client(), "synthetic-minted")}
			p.configureApprovals("/projects/7")
			if count != 1 {
				t.Fatalf("tier/error path made %d requests", count)
			}
			notice := strings.Contains(out.String(), "failed-at-tier") && strings.Contains(out.String(), "do not count approvals as a server-enforced gate on this tier")
			if tc.degrade {
				if len(p.failures) != 0 || !notice {
					t.Fatalf("expected warned tier gap, failures=%v stdout=%s stderr=%s", p.failures, out, errs)
				}
			} else {
				if len(p.failures) != 1 || !strings.Contains(errs.String(), "approval settings write failed") || strings.Contains(out.String(), "NOTICE") {
					t.Fatalf("must fail closed: failures=%v stdout=%s stderr=%s", p.failures, out, errs)
				}
			}
		})
	}
}
