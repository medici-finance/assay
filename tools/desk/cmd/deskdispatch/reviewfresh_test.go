package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Exercise repeated dispatch through real local git allocation and a fake forge.
// A previous review's tracked and untracked evidence must survive byte-for-byte.
func TestReviewFreshAllocation(t *testing.T) {
	for _, lane := range []string{"assay--pr-77", "assay--pr-77--security"} {
		t.Run(lane, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			gh := installGHStamp(t)
			ddGit(t, "", "init", "-b", "main", root)
			ddGit(t, root, "config", "user.name", "Example")
			ddGit(t, root, "config", "user.email", "example@example.invalid")
			ddGit(t, root, "config", "commit.gpgsign", "false")
			if err := os.WriteFile(filepath.Join(root, "evidence.txt"), []byte("seed\n"), 0600); err != nil {
				t.Fatal(err)
			}
			ddGit(t, root, "add", "evidence.txt")
			ddGit(t, root, "commit", "-m", "seed")
			ddGit(t, root, "remote", "add", "origin", "https://github.com/medici-finance/assay.git")
			ddGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
			refs := ddGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads")
			var homes, claims []string
			held := false
			prior := execCommand
			execCommand = func(name string, args ...string) *exec.Cmd {
				if strings.HasSuffix(name, filepath.FromSlash(claimScriptRel)) && args[0] == "acquire" {
					claims = append(claims, args[1])
					if held {
						return exec.Command("sh", "-c", "echo 'LIVE foreign holder' >&2; exit 5")
					}
					held = true
				}
				if strings.HasSuffix(name, filepath.FromSlash(claimScriptRel)) && args[0] == "show" {
					return exec.Command("sh", "-c", "echo 'owner=other state=dispatched age=1m branch=-'")
				}
				if name == "git" {
					return exec.Command(name, args...)
				}
				if name == "deskwt" && args[0] == "add" {
					if args[len(args)-2] != "--role" || args[len(args)-1] != "reviewer" {
						t.Fatalf("wrong worktree role: %v", args)
					}
					s.calls = append(s.calls, append([]string{name}, args...))
					target := filepath.Join(root, "homes", args[1])
					homes = append(homes, target)
					if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
						t.Fatal(err)
					}
					argv := []string{"-C", root, "worktree", "add"}
					for i := 2; i < len(args); i++ {
						switch args[i] {
						case "--detach":
							argv = append(argv, "--detach")
						case "--branch":
							i++
							argv = append(argv, "-b", args[i])
						case "--base", "--role":
							i++
						}
					}
					argv = append(argv, target, "refs/remotes/origin/main")
					c := exec.Command("sh", append([]string{"-c", "git \"$@\" >&2 && printf '%s\\n' \"$REVIEW_TARGET\"", "fixture"}, argv...)...)
					c.Env = append(os.Environ(), "REVIEW_TARGET="+target)
					return c
				}
				return prior(name, args...)
			}
			t.Cleanup(func() { execCommand = prior })
			dispatch := func() int {
				return run([]string{lane, "--root", root, "--repo", allowedRepo, "--kit", "review", "--pr", "77", "--model", "example-model-1", "--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "prompt.md")})
			}
			if rc := dispatch(); rc != 0 {
				t.Fatalf("first dispatch=%d", rc)
			}
			first := homes[0]
			for file, data := range map[string]string{"evidence.txt": "retained tracked proof\n", ".review-proof": "retained untracked proof\n"} {
				if err := os.WriteFile(filepath.Join(first, file), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ddGit(t, first, "add", "evidence.txt")
			before := ddGit(t, first, "status", "--porcelain")
			index := ddGit(t, first, "ls-files", "--stage")
			head := ddGit(t, first, "rev-parse", "HEAD")
			held = false // the prior review completed and released its original claim
			if rc := dispatch(); rc != 0 {
				t.Fatalf("second dispatch=%d; retained reviewer home blocks re-review", rc)
			}
			if len(homes) != 2 || homes[0] == homes[1] {
				t.Fatalf("homes reused: %v", homes)
			}
			if ddGit(t, first, "ls-files", "--stage") != index {
				t.Fatal("retained index entries changed")
			}
			if got := ddGit(t, first, "status", "--porcelain"); got != before {
				t.Fatal("retained index/worktree changed")
			}
			if ddGit(t, first, "rev-parse", "HEAD") != head {
				t.Fatal("retained HEAD moved")
			}
			for file, want := range map[string]string{"evidence.txt": "retained tracked proof\n", ".review-proof": "retained untracked proof\n"} {
				b, e := os.ReadFile(filepath.Join(first, file))
				if e != nil || string(b) != want {
					t.Fatalf("lost %s: %v", file, e)
				}
			}
			if ddGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads") != refs {
				t.Fatal("review allocation changed branch refs")
			}
			for _, home := range homes {
				if out, e := exec.Command("git", "-C", home, "symbolic-ref", "-q", "HEAD").CombinedOutput(); e == nil {
					t.Fatalf("review worktree has branch: %s", out)
				}
			}
			for _, key := range claims {
				if key != lane {
					t.Fatalf("claim changed from %s to %s", lane, key)
				}
			}
			if !gh.applied(deskkit.DispatchedTierPrefix + "strong") {
				t.Fatal("normal stamp step skipped")
			}
			for _, m := range gh.minted {
				if m.role != "reviewer" {
					t.Fatalf("stamp role=%s", m.role)
				}
			}
			writes := len(gh.requests)
			if rc := dispatch(); rc != deskkit.ExitRefused {
				t.Fatalf("live claim rc=%d", rc)
			}
			if len(homes) != 2 || len(gh.requests) != writes {
				t.Fatal("live claim reached allocation or stamp")
			}
		})
	}
}

func TestReviewRejectsBranch(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	rc, _ := runCapturingStderr(t, []string{"assay--pr-77", "--root", root, "--repo", allowedRepo, "--kit", "review", "--branch", "feat/review", "--dry-run", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitRefused {
		t.Fatalf("explicit review branch rc=%d, want refused", rc)
	}
}

func TestReviewAllocationFailure(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	gh := installGHStamp(t)
	s.replies = []reply{{match: "remote get-url origin", stdout: "https://github.com/medici-finance/assay.git"}, {match: "deskwt add", stderr: "refused: target already exists (never clobbered)", code: deskkit.ExitRefused}}
	p := filepath.Join(t.TempDir(), "prompt.md")
	rc, _ := runCapturingStderr(t, []string{"assay--pr-77", "--root", root, "--repo", allowedRepo, "--kit", "review", "--pr", "77", "--model", "example-model-1", "--tier", "strong", "--prompt-file", p})
	if rc != deskkit.ExitRefused || !s.ran("dispatch-claim.sh release assay--pr-77") {
		t.Fatalf("allocation failure did not release original claim: rc=%d calls=%v", rc, s.calls)
	}
	if len(s.deskwtCalls()) != 1 || len(gh.requests) != 0 {
		t.Fatal("allocation failure retried or reached forge stamp")
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("failed allocation emitted prompt")
	}
}
