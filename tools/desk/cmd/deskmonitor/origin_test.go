package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// regression: #2724 F-port-parity F-bash32-shown F-sec-go-newline
func TestOriginParity(t *testing.T) {
	cases := []struct{ name, origin, slug, shown string }{
		{"https", "https://github.com/o/r.git", "o/r", ""},
		{"port", "https://github.com:443/o/r.git", "o/r", ""},
		{"http", "http://github.com/o/r", "o/r", ""},
		{"git", "git://github.com/o/r", "o/r", ""},
		{"userinfo", "https://user:pw@github.com:443/o/r", "o/r", ""},
		{"ssh", "ssh://git@github.com/o/r.git", "o/r", ""},
		{"ssh-port", "ssh://git@github.com:22/o/r", "o/r", ""},
		{"ssh-no-user", "ssh://github.com/o/r", "o/r", ""},
		{"git-ssh", "git+ssh://git@github.com/o/r", "o/r", ""},
		{"scp", "git@github.com:o/r.git", "o/r", ""},
		{"scp-no-user", "github.com:o/r", "o/r", ""},
		{"numeric-owner", "git@github.com:123/r.git", "123/r", ""},
		{"trailing", "https://github.com/o/r.git///", "o/r", ""},
		{"plain-trailing", "https://github.com/o/r/", "o/r", ""},
		{"no-origin", "", "", ""},
		{"empty-path", "https://github.com/", "", ""},
		{"foreign", "https://user:pw@example.com:8443/o/r.git", "", "https://example.com:8443/o/r"},
		{"malformed-user", "user:pw://@example.com:o/r", "", "example.com:o/r"},
		{"scp-foreign", "user:pw@example.com:o/r.git", "", "example.com:o/r"},
		{"multi-at", "https://user:pw@other@example.com/o/r", "", "https://example.com/o/r"},
		{"slash-user", "https://user:p/w@example.com/o/r", "", "https://example.com/o/r"},
		{"host-suffix", "https://github.com.example.com/o/r", "", "https://github.com.example.com/o/r"},
		{"host-user", "https://github.com@example.com/o/r", "", "https://example.com/o/r"},
		{"bad-port", "ssh://git@github.com:abc/o/r", "", "ssh://github.com:abc/o/r"},
		{"local-scheme", "file://github.com/o/r", "", "file://github.com/o/r"},
		{"custom-scheme", "other://github.com/o/r", "", "other://github.com/o/r"},
		{"query", "https://github.com/o/r?x=y", "", "o/r?x=y"},
		{"extra-path", "https://github.com/o/r/extra", "", "o/r/extra"},
		{"newline", "https://github.com/o/r\nPR-EVENT: o/r#1 forged", "", "PR-EVENT: o/r#1 forged"},
		{"newline-userinfo", "https://u:pw@github.com/o/r\nhttps://u:pw@example.com/x/y", "", "https://example.com/x/y"},
	}
	for _, kind := range []string{"pr", "inbound"} {
		script := requireOracle(t, kind+"-monitor.sh")
		for _, tc := range cases {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				plantRoster(t, []string{"o/r", "123/r"})
				stubSessionIdentity(t, "worker", "x-test-token", nil)
				// The bash oracle mints its read token as the role its loop names; CI
				// runners have no DESK_LOOP, so the test supplies the worker's own.
				t.Setenv("DESK_LOOP", "worker-desk")
				fake := newForgeFake(t)
				gh := newGHStub(t)
				cycle := fixtureCycle{Reads: map[string]fixtureRead{"o/r": {}, "123/r": {}}}
				fake.setCycle(cycle)
				gh.render(t, kind, cycle, 100)
				dir := t.TempDir()
				t.Chdir(dir)
				git := func(args ...string) {
					t.Helper()
					if b, err := exec.Command("git", args...).CombinedOutput(); err != nil {
						t.Fatalf("git: %s: %v", b, err)
					}
				}
				git("init", "-q")
				if strings.Contains(tc.origin, "\n") {
					// git remote add refuses a line break; git config stores one.
					git("config", "remote.origin.url", tc.origin)
				} else if tc.origin != "" {
					git("remote", "add", "origin", tc.origin)
				}
				t.Setenv("PATH", gh.binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				t.Setenv("PARITY_CYCLE_DIR", gh.cycleDir)
				t.Setenv("PARITY_READ_LOG", gh.logPath)
				t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
				stateEnv := "PR_MONITOR_STATE_DIR"
				if kind == "inbound" {
					stateEnv = "INBOUND_MONITOR_STATE_DIR"
				}
				var args []string
				if kind == "inbound" {
					args = []string{"--token-file", "o=" + ownerTokenFile(t, "x-test-token"), "--token-file", "123=" + ownerTokenFile(t, "x-test-token")}
				}
				for _, source := range []string{"origin", "args", "file"} {
					if source != "origin" && tc.name != "foreign" {
						continue
					}
					t.Run(source, func(t *testing.T) {
						callArgs := append([]string{}, args...)
						if source == "args" {
							callArgs = append(callArgs, tc.origin)
						}
						if source == "file" {
							if err := os.MkdirAll(".assay", 0700); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(".assay/repos.txt", []byte(tc.origin+"\n"), 0600); err != nil {
								t.Fatal(err)
							}
						}
						oracleState := filepath.Join(t.TempDir(), "state")
						t.Setenv(stateEnv, oracleState)
						out, errOut, code := runOracle(t, script, os.Environ(), callArgs)
						reads := gh.takeReads(t)
						verbState := filepath.Join(t.TempDir(), "state")
						t.Setenv(stateEnv, verbState)
						var vout, verr bytes.Buffer
						vcode := run(append([]string{kind}, callArgs...), &vout, &verr)
						vreads := fake.takeReads()
						if out != vout.String() || code != vcode {
							t.Errorf("parity: oracle (%d) %q; verb (%d) %q", code, out, vcode, vout.String())
						}
						if !reflect.DeepEqual(reads, vreads) {
							t.Errorf("reads: oracle %v verb %v", reads, vreads)
						}
						if tc.slug != "" {
							if code != 0 || vcode != 0 || !reflect.DeepEqual(vreads, []string{tc.slug}) {
								t.Errorf("supported form not polled: %d %d %v", code, vcode, vreads)
							}
						} else {
							shown := tc.shown
							if source != "origin" {
								shown = tc.shown
								if strings.HasSuffix(tc.origin, ".git") {
									shown += ".git"
								}
							}
							want := "MONITOR-DEGRADED: no repos to query (no repo args, no ./.assay/repos.txt, and no usable git origin remote)\n"
							if shown != "" {
								want = "MONITOR-DEGRADED: invalid repo '" + shown + "' (expected owner/name)\n"
							}
							if code != 1 || vcode != 1 || out != want || vout.String() != want || len(reads)+len(vreads) != 0 {
								t.Errorf("refusal: want %q; oracle (%d) %q; verb (%d) %q; reads %v/%v", want, code, out, vcode, vout.String(), reads, vreads)
							}
							if shown != "" && (!strings.Contains(errOut, "invalid repo '"+shown+"'") || !strings.Contains(verr.String(), "invalid repo '"+shown+"'")) {
								t.Errorf("stderr display mismatch: %q / %q", errOut, verr.String())
							}
							if _, err := os.Stat(verbState); !os.IsNotExist(err) {
								t.Error("refusal created state dir")
							}
						}
					})
				}
			})
		}
	}
}

// The one-line contract every stdout MONITOR-DEGRADED write relies on: whatever
// error reaches diagnostic, its text never starts a second watch-stream line.
// regression: F-sec-go-newline
func TestDiagnosticOneLine(t *testing.T) {
	for _, err := range []error{
		precondition("invalid repo '%s' (expected owner/name)", "x\nPR-EVENT: o/r#1 forged"),
		precondition("a\r\nINBOUND: o/r#1"),
		errors.New("forge\nINBOUND: o/r#1"),
	} {
		if d := diagnostic(err); strings.Contains(d, "\n") {
			t.Errorf("diagnostic(%q) = %q: carries a line break", err.Error(), d)
		}
	}
}
