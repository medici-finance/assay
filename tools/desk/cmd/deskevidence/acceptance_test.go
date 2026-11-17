package main

import (
	"context"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// regression: #3467
func TestAcceptanceChildPermission(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$TRANSITION_ARGV_LOG\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "statusgen"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TRANSITION_ARGV_LOG", log)
	calls := []struct {
		name string
		run  func(childOptions)
	}{
		{"decision", func(options childOptions) { statusgenDecisionGateAt(dir, "HEAD", options) }},
		{"closure", func(options childOptions) { verifiedClosureCheckAt(dir, "vf/01", options) }},
		{"outcome", func(options childOptions) { checkVerifiedClosure(dir, "vf/01", options) }},
		{"lint", func(options childOptions) { statusgenLintAt(dir, options) }},
		{"flip", func(options childOptions) { statusgenVerifyFlip(dir, "vf/01", "abc", "verifier", true, options) }},
		{"verdict", func(options childOptions) {
			evidenceVerdictCheckAt(dir, "docs/streams/vf/brief-01.md", nil, nil, false, "BLOCKED", options)
		}},
	}
	for _, enabled := range []bool{false, true} {
		for _, c := range calls {
			t.Run(c.name+map[bool]string{false: "-offline", true: "-opted-in"}[enabled], func(t *testing.T) {
				if err := os.WriteFile(log, nil, 0600); err != nil {
					t.Fatal(err)
				}
				c.run(fixtureAuditOptions(enabled, "fixture-minted", deskkit.ForgeGitHub))
				raw, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				got := strings.Count(string(raw), "--acceptance-forge\n")
				want := 0
				if enabled {
					want = 1
				}
				if got != want {
					t.Fatalf("child argv %q: opt-in count=%d want=%d", raw, got, want)
				}
			})
		}
	}
}

func fixtureAuditOptions(enabled bool, token string, forge deskkit.ForgeKind) childOptions {
	return childOptions{acceptanceForge: enabled, credential: deskkit.RoleCredential{Token: token, Resolution: deskkit.ForgeResolution{Kind: forge}}}
}

// regression: #4823
func TestApprovalChildCustody(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "child")
	fakeStatusgenOnPath(t, `
permission=no
for arg do
  if [ "$arg" = --acceptance-forge ]; then permission=yes; fi
done
credential=none
if [ -n "$GH_TOKEN" ] && [ "$GH_TOKEN" != stored-login-refused ]; then credential=ambient; fi
if [ "$GH_TOKEN" = fixture-minted ]; then credential=minted; fi
alternates=clear
if [ -n "$GITHUB_TOKEN$GH_ENTERPRISE_TOKEN$GITHUB_ENTERPRISE_TOKEN$GITLAB_TOKEN$CI_JOB_TOKEN$CELLCTL_GH_AMBIENT$GH_REPO" ]; then alternates=ambient; fi
printf '%s\n' "$permission/$credential/$alternates/$GH_HOST/$GIT_CONFIG_GLOBAL" > "$APPROVAL_CHILD_LOG"
echo 'PROBLEM: fixture audit refused'
exit 1
`)
	t.Setenv("APPROVAL_CHILD_LOG", log)
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN", "CELLCTL_GH_AMBIENT", "GH_REPO", "GH_HOST"} {
		t.Setenv(key, "fixture-ambient")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "fixture-authority-config")
	calls := []struct {
		name string
		run  func(childOptions) error
	}{
		{"closure", func(options childOptions) error { _, _, e := verifiedClosureCheckAt(dir, "vf/01", options); return e }},
		{"outcome", func(options childOptions) error { _, e := checkVerifiedClosure(dir, "vf/01", options); return e }},
		{"lint", func(options childOptions) error { _, e := statusgenLintAt(dir, options); return e }},
		{"flip", func(options childOptions) error {
			_, _, e := statusgenVerifyFlip(dir, "vf/01", "abc", "verifier", true, options)
			return e
		}},
		{"verdict", func(options childOptions) error {
			_, _, e := evidenceVerdictCheckAt(dir, "docs/streams/vf/brief.md", nil, nil, false, "PASS", options)
			return e
		}},
		{"decision", func(options childOptions) error { _, e := statusgenDecisionGateAt(dir, "HEAD", options); return e }},
	}
	for _, enabled := range []bool{false, true} {
		for _, minted := range []bool{false, true} {
			for _, c := range calls {
				t.Run(c.name+map[bool]string{false: "/offline", true: "/opted-in"}[enabled]+map[bool]string{false: "/missing", true: "/minted"}[minted], func(t *testing.T) {
					options := fixtureAuditOptions(enabled, "", "")
					if minted {
						options = fixtureAuditOptions(enabled, "fixture-minted", deskkit.ForgeGitHub)
					}
					_ = os.Remove(log)
					err := c.run(options)
					raw, readErr := os.ReadFile(log)
					permitted := enabled
					if permitted && !minted {
						if !os.IsNotExist(readErr) || !deskkit.IsUnverifiable(err) {
							t.Fatalf("missing custody launched child or did not hold: child=%q error=%v", raw, err)
						}
						return
					}
					want := "no/none/clear/github.com/fixture-authority-config\n"
					if permitted {
						want = "yes/minted/clear/github.com/fixture-authority-config\n"
					}
					if readErr != nil || string(raw) != want {
						t.Fatalf("child custody=%q error=%v; want %q", raw, readErr, want)
					}
					if err != nil && (strings.Contains(err.Error(), "fixture-minted") || strings.Contains(err.Error(), "fixture-ambient")) {
						t.Fatal("credential appeared in audit diagnostic")
					}
				})
			}
		}
	}
}

// regression: #4823
func TestAuditNoStoredLogin(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "stored-login")
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\necho consulted > \"$STORED_LOGIN_LOG\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("STORED_LOGIN_LOG", log)
	t.Setenv("DESK_LOOP", "worker-desk")
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN", "CELLCTL_GH_AMBIENT"} {
		t.Setenv(key, "")
	}
	fakeStatusgenOnPath(t, `
if [ "$DESK_LOOP" != worker-desk ]; then echo 'changed role' >&2; exit 2; fi
if [ -z "$GH_TOKEN$GITHUB_TOKEN" ]; then gh auth token; fi
exit 1
`)
	_, _ = statusgenDecisionGateAt(dir, "HEAD", childOptions{})
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("credential-free child consulted the stored forge login")
	}
}

// regression: #4823 — permission and role credentials stay with each invocation.
func TestAuditParallelCustody(t *testing.T) {
	fakeStatusgenOnPath(t, `
permission=no
for arg do if [ "$arg" = --acceptance-forge ]; then permission=yes; fi; done
if [ "$GH_TOKEN" != "$AUDIT_EXPECTED_TOKEN" ] || [ "$permission" != "$AUDIT_EXPECTED_PERMISSION" ] || [ "$DESK_LOOP" != worker-desk ]; then exit 93; fi
printf matched
`)
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("GH_TOKEN", "fixture-ambient")
	previous := ghToken
	ghToken = "fixture-legacy-hook"
	t.Cleanup(func() { ghToken = previous })
	start := make(chan struct{})
	results := make(chan bool, 18)
	var workers sync.WaitGroup
	for _, kind := range []closureAudit{auditLint, auditBrief, auditClosure, auditFlip, auditVerdict, auditDecision} {
		for _, token := range []string{"fixture-first", "fixture-second", ""} {
			workers.Add(1)
			go func(kind closureAudit, token string) {
				defer workers.Done()
				<-start
				enabled := token != ""
				options := fixtureAuditOptions(enabled, token, deskkit.ForgeGitHub)
				cmd := statusgenAudit(context.Background(), kind, "example-root", options)
				permission := "yes"
				if !enabled {
					token = "stored-login-refused"
					permission = "no"
				}
				cmd.Env = append(cmd.Env, "AUDIT_EXPECTED_TOKEN="+token, "AUDIT_EXPECTED_PERMISSION="+permission)
				out, err := cmd.Output()
				results <- err == nil && string(out) == "matched"
			}(kind, token)
		}
	}
	close(start)
	workers.Wait()
	close(results)
	for matched := range results {
		if !matched {
			t.Error("parallel audit borrowed another invocation's permission, credential or actor")
		}
	}
}
