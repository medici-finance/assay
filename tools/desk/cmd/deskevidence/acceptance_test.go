package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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
	old, oldToken := acceptanceForge, ghToken
	ghToken = "fixture-minted"
	t.Cleanup(func() { acceptanceForge, ghToken = old, oldToken })
	calls := []struct {
		name string
		run  func()
	}{
		{"closure", func() { verifiedClosureCheckAt(dir, "vf/01") }},
		{"outcome", func() { checkVerifiedClosure(dir, "vf/01") }},
		{"lint", func() { statusgenLintAt(dir) }},
		{"flip", func() { statusgenVerifyFlip(dir, "vf/01", "abc", "verifier", true) }},
	}
	for _, enabled := range []bool{false, true} {
		for _, c := range calls {
			t.Run(c.name+map[bool]string{false: "-offline", true: "-opted-in"}[enabled], func(t *testing.T) {
				acceptanceForge = enabled
				if err := os.WriteFile(log, nil, 0600); err != nil {
					t.Fatal(err)
				}
				c.run()
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
	oldFlag, oldToken := acceptanceForge, ghToken
	t.Cleanup(func() { acceptanceForge, ghToken = oldFlag, oldToken })
	calls := []struct {
		name string
		run  func() error
	}{
		{"closure", func() error { _, _, e := verifiedClosureCheckAt(dir, "vf/01"); return e }},
		{"outcome", func() error { _, e := checkVerifiedClosure(dir, "vf/01"); return e }},
		{"lint", func() error { _, e := statusgenLintAt(dir); return e }},
		{"flip", func() error { _, _, e := statusgenVerifyFlip(dir, "vf/01", "abc", "verifier", true); return e }},
		{"verdict", func() error {
			_, _, e := evidenceVerdictCheckAt(dir, "docs/streams/vf/brief.md", nil, nil, false, "PASS")
			return e
		}},
		{"decision", func() error { _, e := statusgenDecisionGateAt(dir, "HEAD"); return e }},
	}
	for _, enabled := range []bool{false, true} {
		for _, minted := range []bool{false, true} {
			for _, c := range calls {
				t.Run(c.name+map[bool]string{false: "/offline", true: "/opted-in"}[enabled]+map[bool]string{false: "/missing", true: "/minted"}[minted], func(t *testing.T) {
					acceptanceForge, ghToken = enabled, ""
					if minted {
						ghToken = "fixture-minted"
					}
					_ = os.Remove(log)
					err := c.run()
					raw, readErr := os.ReadFile(log)
					permitted := enabled && c.name != "decision"
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
	oldFlag, oldToken := acceptanceForge, ghToken
	acceptanceForge, ghToken = false, ""
	t.Cleanup(func() { acceptanceForge, ghToken = oldFlag, oldToken })
	_, _ = statusgenDecisionGateAt(dir, "HEAD")
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("credential-free child consulted the stored forge login")
	}
}
