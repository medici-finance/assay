package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2794
func TestWriteReceiptObject(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "EVIDENCE.md"), []byte("old row\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "EVIDENCE.md")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "Evidence fixture")
	baseCommit := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "EVIDENCE.md"), []byte("row\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "EVIDENCE.md")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "Landed Evidence")
	commit, blob := git("rev-parse", "HEAD"), git("rev-parse", "HEAD:EVIDENCE.md")
	for _, backend := range []string{"github", "gitlab"} {
		for _, scenario := range []string{"create", "update", "noop"} {
			t.Run(backend+"/"+scenario, func(t *testing.T) {
				var f Forge
				var expected string
				content := "old row\n"
				if scenario == "noop" {
					content = "row\n"
				}
				if backend == "github" {
					s := newGoldenServer(t)
					if scenario == "create" {
						s.contentsGetStatus = 404
					} else {
						s.contentsGet = ghServedFile("EVIDENCE.md", content)
						expected = blobID(content, false)
					}
					s.contentsPut = map[string]any{"content": map[string]any{"sha": blob}, "commit": map[string]any{"sha": commit, "author": map[string]any{"name": "assay-verifier-app[bot]"}}}
					f = s.forge()
				} else {
					s := newGLServer(t)
					s.project = map[string]any{"default_branch": "main"}
					if scenario != "create" {
						s.repoFile = map[string]map[string]any{"EVIDENCE.md": {"content": glB64(content), "last_commit_id": baseCommit, "blob_id": blob}}
						expected = baseCommit
						if scenario == "noop" {
							expected = commit
							s.repoFile["EVIDENCE.md"]["last_commit_id"] = commit
						}
					}
					s.createCommitResp, s.updateCommitResp = map[string]any{"id": commit}, map[string]any{"id": commit}
					f = s.forge()
				}
				res, err := f.WriteFile(forgeTestRepo, WriteFileInput{File: "EVIDENCE.md", Branch: "evidence/row", Content: []byte("row\n"), Message: "Evidence fixture", ExpectedSHA: expected, StartBranch: "main"})
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "noop" {
					if res.Changed || res.CommitSHA != "" || string(res.ContentID) != expected {
						t.Fatalf("noop fabricated a commit or lost precondition: %+v", res)
					}
					return
				}
				if kind := git("cat-file", "-t", string(res.CommitSHA)); kind != "commit" {
					t.Fatalf("landing receipt names a %s object, want commit", kind)
				}
				if got := git("show", string(res.CommitSHA)+":EVIDENCE.md"); got != "row" {
					t.Fatalf("receipt does not locate landed Evidence: %q", got)
				}
				git("merge-base", "--is-ancestor", baseCommit, string(res.CommitSHA))
				if got := git("show", string(res.CommitSHA)+"^:EVIDENCE.md"); got != "old row" {
					t.Fatalf("receipt is not the introduction commit: %s", got)
				}
				wantContent := blob
				if backend == "gitlab" {
					wantContent = commit
				}
				if string(res.ContentID) != wantContent {
					t.Fatalf("lost file precondition: %+v", res)
				}
			})
		}
	}
}
