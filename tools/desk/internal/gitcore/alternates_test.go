package gitcore

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// --- Shared-object (alternates) clones --------------------------------------
//
// `git clone --shared` and `git clone --reference` copy no objects: the clone's
// objects/info/alternates names the source repository's object directory and every
// object that was not written locally is READ FROM THERE. gitcore hands go-git a chroot
// rooted at the repository's own .git, so without filesystem.Options.AlternatesFS those
// borrowed objects are invisible — and go-git's tree walk does not report that as an
// error. object.TreeWalker.Next turns a failed subtree read into io.EOF, so the HEAD
// tree walk TRUNCATES: Worktree.Status then reports the index entries whose HEAD-side
// entries vanished as staged ADDITIONS, and deskpr's staging preflight refuses a clean
// checkout with "staged-but-uncommitted changes — commit them first".
//
// These tests run the matrix against BOTH gitcore and the git binary in the same
// fixture, because parity with git is the actual contract: a clean supported clone
// passes, a real staged change still refuses, and objects that genuinely cannot be read
// produce an explicit could-not-check rather than either answer. No network, no
// credentials, no live infrastructure.

// sharedClone builds a source repository holding NESTED directories, clones it with
// --shared, and makes one local commit in the clone. The local commit is what makes the
// fixture sharp: the new commit and its ROOT tree are written locally, while every
// unchanged nested tree and blob still exists only in the source repository's object
// store. That is precisely the shape that truncates the walk rather than failing it.
func sharedClone(t *testing.T) (src *gittest.Fixture, clone string) {
	t.Helper()
	src = gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(src.Dir, "nested", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src.Dir, "nested", "deep", "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Git("add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Git("commit", "-q", "-m", "nested tree"); err != nil {
		t.Fatal(err)
	}

	clone = filepath.Join(t.TempDir(), "clone")
	gitIn(t, "", "clone", "--shared", "--no-checkout", "-q", src.Dir, clone)
	gitIn(t, clone, "checkout", "-q", "main")
	gitIn(t, clone, "config", "user.name", "test")
	gitIn(t, clone, "config", "user.email", "test@example.invalid")

	alt, err := os.ReadFile(filepath.Join(clone, ".git", "objects", "info", "alternates"))
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: this git did not write objects/info/alternates for --shared: %v", err)
	}
	if strings.TrimSpace(string(alt)) == "" {
		t.Fatal("COULD-NOT-CHECK: objects/info/alternates is empty, so this fixture is not a shared-object clone")
	}

	if err := os.WriteFile(filepath.Join(clone, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, clone, "add", "local.txt")
	gitIn(t, clone, "commit", "-q", "-m", "a local commit, whose unchanged subtrees stay in the alternate")

	// The fixture's own precondition: the nested subtree must NOT be in the clone's own
	// object store. Proved by breaking the borrow and watching git itself lose the
	// object — then restoring it.
	nested := strings.TrimSpace(mustGitOut(t, clone, "rev-parse", "HEAD:nested"))
	withAlternatesHidden(t, clone, func() {
		if err := gitErrIn(clone, "cat-file", "-e", nested+"^{tree}"); err == nil {
			t.Fatalf("COULD-NOT-CHECK: nested tree %s is in the clone's OWN object store, so this fixture does not exercise alternates at all", nested)
		}
	})
	return src, clone
}

// withAlternatesHidden runs fn with the clone's objects/info/alternates moved aside, so
// nothing — git or gitcore — can reach the borrowed object store, then restores it.
func withAlternatesHidden(t *testing.T, clone string, fn func()) {
	t.Helper()
	p := filepath.Join(clone, ".git", "objects", "info", "alternates")
	hidden := p + ".hidden"
	if err := os.Rename(p, hidden); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(hidden, p); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := gitOut(dir, args...); err != nil {
		t.Fatalf("git %s in %q: %v: %s", strings.Join(args, " "), dir, err, out)
	}
}

func mustGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitOut(dir, args...)
	if err != nil {
		t.Fatalf("git %s in %q: %v: %s", strings.Join(args, " "), dir, err, out)
	}
	return out
}

func gitErrIn(dir string, args ...string) error {
	_, err := gitOut(dir, args...)
	return err
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_AUTHOR_DATE=2001-02-03T04:05:06Z", "GIT_COMMITTER_DATE=2001-02-03T04:05:06Z",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestHasStagedChangesSharedCloneIsClean is the regression proper: a shared-object clone
// that git itself calls clean must not be refused as having staged changes.
func TestHasStagedChangesSharedCloneIsClean(t *testing.T) {
	_, clone := sharedClone(t)
	if err := gitErrIn(clone, "diff", "--cached", "--quiet"); err != nil {
		t.Fatalf("COULD-NOT-CHECK: git itself reports staged changes in the fixture clone: %v", err)
	}
	repo, err := Open(clone)
	if err != nil {
		t.Fatalf("open shared clone: %v", err)
	}
	staged, err := repo.HasStagedChanges()
	if err != nil {
		t.Fatalf("HasStagedChanges on a clean shared clone = _, %v, want false, nil", err)
	}
	if staged {
		t.Fatal("HasStagedChanges reported staged changes in a shared-object clone git calls clean")
	}
}

// TestHasStagedChangesSharedCloneStillRefusesRealStaging is the other half of the
// contract, and the one that keeps this fix from being a loosened check: recognising a
// borrowed object store must not make a genuinely staged change read as clean.
func TestHasStagedChangesSharedCloneStillRefusesRealStaging(t *testing.T) {
	t.Run("staged edit of a file that lives in the alternate", func(t *testing.T) {
		_, clone := sharedClone(t)
		if err := os.WriteFile(filepath.Join(clone, "seed.txt"), []byte("edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, clone, "add", "seed.txt")
		assertStagedAgreesWithGit(t, clone, true)
	})

	t.Run("staged deletion of a file that lives in the alternate", func(t *testing.T) {
		// The direction a truncated HEAD-tree walk can HIDE rather than invent: if the
		// deleted path is dropped from the HEAD side too, it vanishes from both sides of
		// the diff and a dirty index reads as CLEAN.
		_, clone := sharedClone(t)
		gitIn(t, clone, "rm", "-q", "nested/deep/a.txt")
		assertStagedAgreesWithGit(t, clone, true)
	})

	t.Run("untracked file is not a staged change", func(t *testing.T) {
		_, clone := sharedClone(t)
		if err := os.WriteFile(filepath.Join(clone, "never-added.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStagedAgreesWithGit(t, clone, false)
	})
}

// assertStagedAgreesWithGit checks HasStagedChanges against `git diff --cached --quiet`
// in the same checkout, and that both say what the case intends. Disagreement with git
// is the failure that matters; want is there so a fixture that silently stops setting up
// the case it names cannot pass.
func assertStagedAgreesWithGit(t *testing.T, dir string, want bool) {
	t.Helper()
	gitDirty := gitErrIn(dir, "diff", "--cached", "--quiet") != nil
	if gitDirty != want {
		t.Fatalf("COULD-NOT-CHECK: git diff --cached --quiet reports dirty=%v, but this case is meant to be dirty=%v", gitDirty, want)
	}
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := repo.HasStagedChanges()
	if err != nil {
		t.Fatalf("HasStagedChanges = _, %v, want %v, nil", err, want)
	}
	if staged != gitDirty {
		t.Fatalf("HasStagedChanges = %v but git diff --cached --quiet says dirty=%v", staged, gitDirty)
	}
}

// TestHasStagedChangesUnreadableAlternateIsUnverifiable pins the honest-failure half:
// when the borrowed object store cannot be read, the answer is an explicit
// could-not-check — never "clean", and never the misread "staged additions" that
// go-git's truncating tree walk would otherwise manufacture.
func TestHasStagedChangesUnreadableAlternateIsUnverifiable(t *testing.T) {
	cases := []struct {
		name     string
		break_   func(t *testing.T, src *gittest.Fixture, clone string)
		wantKind error
	}{
		{
			name: "alternate object directory no longer exists",
			break_: func(t *testing.T, src *gittest.Fixture, clone string) {
				if err := os.RemoveAll(filepath.Join(src.Dir, ".git", "objects")); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: ErrUnsupportedAlternates,
		},
		{
			name: "alternates entry is relative, which go-git cannot resolve",
			break_: func(t *testing.T, src *gittest.Fixture, clone string) {
				p := filepath.Join(clone, ".git", "objects", "info", "alternates")
				if err := os.WriteFile(p, []byte("../../elsewhere/objects\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: ErrUnsupportedAlternates,
		},
		{
			name: "alternates file is gone entirely",
			break_: func(t *testing.T, src *gittest.Fixture, clone string) {
				if err := os.Remove(filepath.Join(clone, ".git", "objects", "info", "alternates")); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: ErrObjectStoreIncomplete,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, clone := sharedClone(t)
			tc.break_(t, src, clone)

			// Parity: git cannot read those objects either, so could-not-check is the
			// honest answer rather than a gitcore-only shortfall.
			if err := gitErrIn(clone, "cat-file", "-e", "HEAD:nested^{tree}"); err == nil {
				t.Skip("COULD-NOT-CHECK: git still reads the borrowed objects here, so the fixture did not break the store")
			}

			repo, err := Open(clone)
			if err != nil {
				t.Fatalf("Open must still open a repository whose object store is incomplete: %v", err)
			}
			staged, err := repo.HasStagedChanges()
			if err == nil {
				t.Fatalf("HasStagedChanges returned %v with no error on an unreadable object store — a confident answer computed from half a repository", staged)
			}
			if !errors.Is(err, ErrObjectStoreIncomplete) {
				t.Fatalf("error %v does not wrap ErrObjectStoreIncomplete, so a caller cannot tell could-not-check from a real refusal", err)
			}
			if !errors.Is(err, tc.wantKind) {
				t.Fatalf("error %v does not wrap %v", err, tc.wantKind)
			}
			if staged {
				t.Fatal("could-not-check must not also claim staged changes")
			}
		})
	}
}

// TestHasStagedChangesAfterRepackNeedsNoAlternate pins the repair path: once the
// borrowed objects are materialised locally the checkout stands on its own, and the
// answer stays clean with no alternate store at all.
func TestHasStagedChangesAfterRepackNeedsNoAlternate(t *testing.T) {
	src, clone := sharedClone(t)
	gitIn(t, clone, "repack", "-a", "-q")
	if err := os.Remove(filepath.Join(clone, ".git", "objects", "info", "alternates")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(src.Dir, ".git", "objects")); err != nil {
		t.Fatal(err)
	}
	if err := gitErrIn(clone, "diff", "--cached", "--quiet"); err != nil {
		t.Fatalf("COULD-NOT-CHECK: git reports staged changes after repack: %v", err)
	}
	repo, err := Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := repo.HasStagedChanges()
	if err != nil || staged {
		t.Fatalf("HasStagedChanges (repacked, no alternate) = %v, %v, want false, nil", staged, err)
	}
}

// TestReadAlternatesBoundary pins the filesystem boundary the alternates resolution is
// given: the nearest common ancestor of the directories the repository itself declares,
// never one of those directories (go-git chroots at its PARENT and joins "objects"), and
// never the filesystem root.
func TestReadAlternatesBoundary(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + filepath.Join("srv", "pool")
	cases := []struct {
		name  string
		paths []string
		want  string
		fail  bool
	}{
		{
			name:  "single alternate roots at its parent, never at itself",
			paths: []string{filepath.Join(root, "a.git", "objects")},
			want:  filepath.Join(root, "a.git"),
		},
		{
			name:  "two alternates root at their nearest common ancestor",
			paths: []string{filepath.Join(root, "a.git", "objects"), filepath.Join(root, "b.git", "objects")},
			want:  root,
		},
		{
			name:  "the filesystem root is refused rather than handing over everything",
			paths: []string{sep + filepath.Join("a.git", "objects"), sep + filepath.Join("b.git", "objects")},
			fail:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := alternatesRoot(tc.paths)
			if tc.fail {
				if err == nil {
					t.Fatalf("alternatesRoot(%v) = %q, want a refusal", tc.paths, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("alternatesRoot(%v): %v", tc.paths, err)
			}
			if got != tc.want {
				t.Fatalf("alternatesRoot(%v) = %q, want %q", tc.paths, got, tc.want)
			}
			for _, p := range tc.paths {
				rel, rerr := filepath.Rel(got, p)
				if rerr != nil || strings.HasPrefix(rel, "..") || rel == "." {
					t.Fatalf("root %q does not strictly contain alternate %q (rel=%q, err=%v)", got, p, rel, rerr)
				}
			}
		})
	}
}
