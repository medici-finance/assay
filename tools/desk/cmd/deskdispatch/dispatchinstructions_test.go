package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellstop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rulesFixture is a cell directory with its session-writable places, and a rules directory
// outside it holding one dispatch-instruction file per kit.
type rulesFixture struct {
	cellDir, rulesDir string
	files             map[string]string // kit -> absolute path
	digests           map[string]string // kit -> sha256
}

func newRulesFixture(t *testing.T, kits ...string) *rulesFixture {
	t.Helper()
	f := &rulesFixture{cellDir: t.TempDir(), rulesDir: t.TempDir(), files: map[string]string{}, digests: map[string]string{}}
	for _, d := range []string{"worktrees/worker-desk", "memory/worker-desk"} {
		if err := os.MkdirAll(filepath.Join(f.cellDir, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, kit := range kits {
		f.files[kit] = f.write(t, filepath.Join(f.rulesDir, kit+"-rules.md"), "# "+kit+" role rules\n\n- Example rule for the "+kit+" kit: cite the brief row you ran.\n")
		f.digests[kit] = digestOf(t, f.files[kit])
	}
	return f
}

func (f *rulesFixture) write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func digestOf(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// value composes the launch value for the given kits, in order.
func (f *rulesFixture) value(kits ...string) string {
	var e []deskkit.DispatchInstruction
	for _, k := range kits {
		e = append(e, deskkit.DispatchInstruction{Kit: k, Path: f.files[k], SHA256: f.digests[k]})
	}
	return deskkit.FormatDispatchInstructions(e)
}

// set exports the value and the cell marker, as a cellctl window carries them.
func (f *rulesFixture) set(t *testing.T, value string) {
	t.Helper()
	t.Setenv(deskkit.EnvDispatchInstructions, value)
	t.Setenv(cellstop.EnvCellDir, f.cellDir)
}

// rulesBlock is the block a prompt must end with when kit took its entry.
func rulesBlock(t *testing.T, kit, path, digest string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return "## Role dispatch instructions (" + kit + ") — source " + path + " sha256:" + digest + "\n\n" +
		dispatchRulesFirstLine + "\n\n" + string(body) + "\n" // the emitter's own final newline
}

// runRules runs one dispatch against the shared stub and root and returns the emitted prompt.
func runRules(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	pf := filepath.Join(t.TempDir(), "prompt.md")
	rc := run(append(append([]string{}, args...), "--root", root, "--repo", allowedRepo, "--tier", "strong", "--prompt-file", pf))
	body, err := os.ReadFile(emittedPathFor(pf))
	if err != nil {
		return "", rc
	}
	return string(body), rc
}

// regression: F-role-dispatch-instructions
func TestDispatchRulesInPrompt(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		entry string // the entry the run must take
	}{
		{"fresh worker", []string{"item-30", "--kit", "worker"}, "worker"},
		{"shepherding worker", []string{"item-30", "--kit", "worker", "--pr", "42"}, "worker"},
		{"shepherding worker-objective", []string{"item-30", "--kit", "worker-objective", "--pr", "42"}, "worker-objective"},
		{"kit in another case", []string{"item-30", "--kit", "Worker"}, "worker"},
		{"kit with surrounding space", []string{"item-30", "--kit", " worker "}, "worker"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixEntropy(t)
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home-dc30")
			f := newRulesFixture(t, "worker", "worker-objective")
			f.set(t, f.value("worker", "worker-objective"))

			prompt, rc := runRules(t, root, tc.args...)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0", rc)
			}
			want := rulesBlock(t, tc.entry, f.files[tc.entry], f.digests[tc.entry])
			if !strings.HasSuffix(prompt, want) {
				t.Fatalf("the prompt does not end with the %s block:\n%s\n--- prompt tail ---\n%s", tc.entry, want, tail(prompt, 800))
			}
			kitAt := strings.LastIndex(prompt, "# Standing clauses — ")
			if kitAt < 0 || kitAt > strings.LastIndex(prompt, want) {
				t.Error("the block does not follow the class kit")
			}
			other := "worker-objective"
			if tc.entry == other {
				other = "worker"
			}
			if strings.Contains(prompt, f.files[other]) {
				t.Errorf("the prompt quotes the %s entry too", other)
			}

			recs := recordLines(t, home)
			if len(recs) != 1 {
				t.Fatalf("record lines = %d, want 1: %s", len(recs), dump(recs))
			}
			if got := sv(recs[0].DispatchInstructionsSHA256); got != `"`+f.digests[tc.entry]+`"` {
				t.Errorf("record dispatch_instructions_sha256 = %s, want %s", got, f.digests[tc.entry])
			}
			raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", deskkit.DispatchRecordsFile))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), f.rulesDir) {
				t.Errorf("the record carries the file's path: %s", raw)
			}
		})
	}

	// A kit the value names no entry for, and an unset value, build one and the same prompt.
	t.Run("review takes nothing", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		s.replies = append(s.replies, reply{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"})
		args := []string{"assay--pr-42", "--kit", "review", "--pr", "42", "--dry-run"}
		t.Setenv(deskkit.EnvDispatchInstructions, "")
		unset, rc := runRules(t, root, args...)
		if rc != deskkit.ExitOK {
			t.Fatalf("unset rc = %d", rc)
		}
		f := newRulesFixture(t, "worker", "worker-objective")
		f.set(t, f.value("worker", "worker-objective"))
		set, rc := runRules(t, root, args...)
		if rc != deskkit.ExitOK {
			t.Fatalf("set rc = %d", rc)
		}
		if set != unset {
			t.Errorf("--kit review prompt differs when the value names worker kits only")
		}
	})

	// The worker prompt with the value set is the unset prompt plus the block, and nothing else.
	t.Run("worker block only addition", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		s.replies = happyReplies("/private/tmp/worker-home-dc30")
		args := []string{"item-30", "--kit", "worker", "--dry-run"}
		t.Setenv(deskkit.EnvDispatchInstructions, "")
		unset, rc := runRules(t, root, args...)
		if rc != deskkit.ExitOK {
			t.Fatalf("unset rc = %d", rc)
		}
		f := newRulesFixture(t, "worker")
		f.set(t, f.value("worker"))
		set, rc := runRules(t, root, args...)
		if rc != deskkit.ExitOK {
			t.Fatalf("set rc = %d", rc)
		}
		if !strings.HasPrefix(set, unset) {
			t.Fatal("the set prompt does not start with the unset prompt")
		}
		if got, want := set[len(unset):], "---\n\n"+rulesBlock(t, "worker", f.files["worker"], f.digests["worker"]); got != want {
			t.Errorf("the addition is\n%q\nwant\n%q", got, want)
		}
	})
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// regression: F-role-dispatch-instructions
func TestDispatchRulesRefusals(t *testing.T) {
	type setup func(t *testing.T, f *rulesFixture, s *stub) (value, file string)
	worker := func(t *testing.T, f *rulesFixture, _ *stub) (string, string) {
		return f.value("worker"), f.files["worker"]
	}
	cases := []struct {
		name  string
		setup setup
		cell  func(f *rulesFixture) (string, bool) // the ASSAY_CELL_DIR to export, and whether to export it
	}{
		{name: "missing file", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			v, p := worker(t, f, s)
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			return v, p
		}},
		{name: "unreadable file", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			v, p := worker(t, f, s)
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
			return v, p
		}},
		{name: "edited after launch", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			v, p := worker(t, f, s)
			f.write(t, p, "# worker role rules\n\n- An edit made after the window launched.\n")
			return v, p
		}},
		{name: "malformed value", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			p := f.files["worker"]
			return "worker=" + p + "@sha256:" + strings.ToUpper(f.digests["worker"]), p
		}},
		{name: "unknown kit", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			p := f.files["worker"]
			return "worker=" + p + "@sha256:" + f.digests["worker"] + ",example-kit=" + p + "@sha256:" + f.digests["worker"], p
		}},
		{name: "relative path", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			return "worker=rules/worker-rules.md@sha256:" + f.digests["worker"], "rules/worker-rules.md"
		}},
		{name: "in own git worktree", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			own := t.TempDir()
			p := f.write(t, filepath.Join(own, "worker-rules.md"), "# rules in the session's own tree\n")
			s.replies = append(s.replies, reply{match: "rev-parse --show-toplevel", stdout: own})
			return deskkit.FormatDispatchInstructions([]deskkit.DispatchInstruction{{Kit: "worker", Path: p, SHA256: digestOf(t, p)}}), p
		}},
		{name: "under memory", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			p := f.write(t, filepath.Join(f.cellDir, "memory", "worker-desk", "rules.md"), "# rules in memory\n")
			return deskkit.FormatDispatchInstructions([]deskkit.DispatchInstruction{{Kit: "worker", Path: p, SHA256: digestOf(t, p)}}), p
		}},
		{name: "memory through an outside symlink", setup: func(t *testing.T, f *rulesFixture, s *stub) (string, string) {
			real := f.write(t, filepath.Join(f.cellDir, "memory", "worker-desk", "rules.md"), "# rules in memory\n")
			link := filepath.Join(t.TempDir(), "looks-outside")
			if err := os.Symlink(filepath.Join(f.cellDir, "memory"), link); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(link, "worker-desk", "rules.md")
			return deskkit.FormatDispatchInstructions([]deskkit.DispatchInstruction{{Kit: "worker", Path: p, SHA256: digestOf(t, real)}}), p
		}},
		{name: "cell dir unset", setup: worker, cell: func(*rulesFixture) (string, bool) { return "", false }},
		{name: "cell dir nonexistent", setup: worker, cell: func(f *rulesFixture) (string, bool) {
			return filepath.Join(f.cellDir, "no-such-cell"), true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home-dc30")
			f := newRulesFixture(t, "worker")
			value, file := tc.setup(t, f, s)
			f.set(t, value)
			if tc.cell != nil {
				dir, ok := tc.cell(f)
				if ok {
					t.Setenv(cellstop.EnvCellDir, dir)
				} else {
					t.Setenv(cellstop.EnvCellDir, "")
					os.Unsetenv(cellstop.EnvCellDir)
				}
			}
			var rc int
			stderr := captureStderr(t, func() {
				_, rc = runRules(t, root, "item-30", "--kit", "worker")
			})
			if rc == deskkit.ExitOK {
				t.Fatalf("rc = 0, want a refusal; stderr:\n%s", stderr)
			}
			if s.ran("dispatch-claim.sh") || s.ran("deskclaim-ref") {
				t.Error("a claim was attempted")
			}
			if s.ran("deskwt add") {
				t.Error("a worktree was cut")
			}
			if recs := recordLines(t, home); len(recs) != 0 {
				t.Errorf("record lines written: %s", dump(recs))
			}
			for _, want := range []string{"kit worker", file} {
				if !strings.Contains(stderr, want) {
					t.Errorf("message does not name %q:\n%s", want, stderr)
				}
			}
		})
	}

	// cellctl and deskdispatch reach the custody check through the one deskkit function.
	src, err := os.ReadFile(filepath.Join("..", "cellctl", "rolecontext.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "func sessionWritablePlace") {
		t.Error("cellctl's rolecontext.go still defines its own sessionWritablePlace")
	}
}

// TestDispatchRulesRepoWorktrees pins the listing the custody re-check adds for the target
// repository: the main work tree and every linked one, read from the common git directory, so a
// dispatch-instruction file inside any of them is refused.
func TestDispatchRulesRepoWorktrees(t *testing.T) {
	gitRun := func(dir string, args ...string) {
		t.Helper()
		if r := runCmd(dir, "git", args...); r.err != nil {
			t.Fatalf("git %v: %v: %s", args, r.err, r.stderr)
		}
	}
	main := t.TempDir()
	gitRun(main, "init", "-q")
	gitRun(main, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "base")
	linked := filepath.Join(t.TempDir(), "linked")
	gitRun(main, "worktree", "add", "-q", "--detach", linked)
	got, err := repoWorktrees(linked)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, d := range []string{main, linked} {
		real, err := filepath.EvalSymlinks(d)
		if err != nil {
			t.Fatal(err)
		}
		want[real] = true
	}
	seen := map[string]bool{}
	for _, d := range got {
		real, err := filepath.EvalSymlinks(d)
		if err != nil {
			t.Fatalf("listed %s, which does not resolve: %v", d, err)
		}
		seen[real] = true
	}
	for d := range want {
		if !seen[d] {
			t.Errorf("work tree %s missing from %v", d, got)
		}
	}
	bare := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(bare))
	if outside, err := repoWorktrees(bare); err != nil || len(outside) != 0 {
		t.Errorf("a directory outside any repository listed %v, err %v; want none", outside, err)
	}
}
