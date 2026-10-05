package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const resumeSHA = "1111111111111111111111111111111111111111"

func TestResumeSource(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		for _, kit := range []string{"worker", "worker-objective"} {
			for _, dry := range []bool{false, true} {
				t.Run(forge+"/"+kit+"/dry="+itoa(map[bool]int{true: 1}[dry]), func(t *testing.T) {
					s := &stub{}
					home, root := s.install(t)
					readResumeChange = liveResumeChange
					plantScripts(t, root)
					repo := allowedRepo
					if forge == "gitlab" {
						installGLStamp(t, home)
						repo = glProject
					}
					reads := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						reads++
						if r.Method != "GET" {
							t.Errorf("unexpected write: %s", r.Method)
						}
						var body any
						if forge == "github" {
							body = map[string]any{"number": 42, "state": "open", "head": map[string]any{"ref": "fix/resume-evidence", "sha": resumeSHA, "repo": map[string]any{"full_name": repo}}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": repo}}}
						} else {
							body = map[string]any{"changes_count": "1", "iid": 42, "state": "opened", "source_branch": "fix/resume-evidence", "sha": resumeSHA, "source_project_id": 1, "target_project_id": 1}
						}
						json.NewEncoder(w).Encode(body)
					}))
					defer server.Close()
					if forge == "github" {
						deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) { return ghStampToken, server.URL, nil })
						t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })
					} else {
						t.Setenv("GITLAB_API_BASE", server.URL)
					}
					s.replies = []reply{
						{match: "remote get-url origin", stdout: "https://" + forge + ".com/" + repo + ".git"},
						{match: "rev-parse --verify --quiet refs/remotes/origin/fix/resume-evidence", stdout: resumeSHA},
						{match: "deskwt add", stdout: filepath.Join(t.TempDir(), "home")},
					}
					args := []string{"item-1", "--root", root, "--repo", repo, "--pr", "42", "--kit", kit}
					if kit == "worker-objective" {
						args = append(args, "--branch", "fix/resume-evidence")
					}
					if dry {
						args = append(args, "--dry-run")
					} else {
						args = append(args, "--prompt-file", filepath.Join(t.TempDir(), "prompt.md"))
					}
					out, errOut, rc := ddRunCapture(t, args)
					if rc != 0 {
						t.Fatalf("dispatch failed: %d %s", rc, errOut)
					}
					if reads == 0 {
						t.Fatal("resume never read the forge source branch")
					}
					if dry {
						if !strings.Contains(out, "branch=fix/resume-evidence") || s.ran("fetch") || s.ran("acquire") {
							t.Fatalf("dry plan: %s calls=%v", out, s.calls)
						}
					} else {
						argv := deskwtAddArgv(s)
						if !strings.Contains(argv, "--branch fix/resume-evidence") || !strings.Contains(argv, "--base "+resumeSHA) {
							t.Fatalf("wrong resume allocation: %s", argv)
						}
					}
				})
			}
		}
	}
}

func TestResumeRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, branch, head, state, cross, explicit, remote, want string
		fetchFail, readFail                                      bool
	}{
		{name: "missing-source", branch: "", want: "source branch is missing"},
		{name: "bad-source", branch: "bad..branch", want: "source branch is missing"},
		{name: "missing-head", branch: "fix/source", head: "missing", want: "head is missing"},
		{name: "closed", branch: "fix/source", state: "closed", want: "open change"},
		{name: "fork", branch: "fix/source", cross: "fork", want: "target repository"},
		{name: "unknown-repo", branch: "fix/source", cross: "unknown", want: "target repository"},
		{name: "explicit-mismatch", branch: "fix/source", explicit: "feat/other", want: "differs"},
		{name: "fetch-failed", branch: "fix/source", fetchFail: true, want: "cannot refresh"},
		{name: "missing-remote", branch: "fix/source", remote: "missing", want: "does not resolve"},
		{name: "stale-remote", branch: "fix/source", remote: strings.Repeat("2", 40), want: "does not resolve"},
		{name: "read-failed", branch: "fix/source", readFail: true, want: "cannot read resume source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			head, state, cross, remote := resumeSHA, "open", deskkit.CrossRepoSame, resumeSHA
			if tc.head != "" {
				head = ""
			}
			if tc.state != "" {
				state = tc.state
			}
			if tc.cross != "" {
				cross = tc.cross
			}
			if tc.remote != "" {
				remote = tc.remote
			}
			readResumeChange = func(dispatchOpts, string) (deskkit.PullRequest, error) {
				if tc.readFail {
					return deskkit.PullRequest{}, fmt.Errorf("fixture read failed")
				}
				return deskkit.PullRequest{State: state, HeadRef: tc.branch, HeadSHA: head, CrossRepo: cross}, nil
			}
			fetchCode := 0
			if tc.fetchFail {
				fetchCode = 1
			}
			s.replies = []reply{{match: "remote get-url origin", stdout: "https://github.com/" + allowedRepo + ".git"}, {match: "fetch --quiet", code: fetchCode}, {match: "rev-parse --verify --quiet", stdout: remote}}
			prompt := filepath.Join(t.TempDir(), "prompt.md")
			args := []string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "42", "--prompt-file", prompt}
			if tc.explicit != "" {
				args = append(args, "--branch", tc.explicit)
			}
			_, errOut, rc := ddRunCapture(t, args)
			if rc == 0 || !strings.Contains(errOut, tc.want) {
				t.Fatalf("wanted %q refusal, got %d %s", tc.want, rc, errOut)
			}
			if s.ran("acquire") || s.ran("deskwt") || s.ran("deskroster") {
				t.Fatalf("refusal created durable state: %v", s.calls)
			}
			if _, err := os.Stat(prompt); !os.IsNotExist(err) {
				t.Fatal("refusal emitted prompt")
			}
		})
	}
}

func TestResumeAllocationBoundary(t *testing.T) {
	for _, kit := range []string{"worker", "worker-objective"} {
		for _, source := range []*resumeSource{nil, {branch: "fix/source", head: resumeSHA}, {branch: "fix/other", head: resumeSHA, verified: true}, {branch: "fix/source", head: "", verified: true}} {
			s := &stub{}
			s.install(t)
			result := createDispatchWorktree(dispatchOpts{kit: kit, pr: 42}, dispatchPlan{branch: "fix/source", resume: source})
			if result.err == nil || s.ran("deskwt") {
				t.Fatalf("unchecked resume reached allocation: source=%+v", source)
			}
		}
	}
}

// Every allocator is already inventoried by the shared allocation class guard.
// This second-site plant proves a new resume path cannot bypass that boundary.
func TestResumeClassPlant(t *testing.T) {
	src := []byte("package main; func secondResume(o dispatchOpts) { runCmd(o.root, \"deskwt\", \"add\", \"second-resume\", \"--branch\", \"feat/item\", \"--base\", mainlineRef) }")
	if bad := allocationViolations(t, "second_resume.go", src); len(bad) == 0 {
		t.Fatal("planted unchecked resume was admitted")
	}
}
