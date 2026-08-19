package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fabricated (nonexistent) layout — resolvePath degrades to Clean for paths
// that don't exist, so these are deterministic on any machine.
const (
	shared   = "/Users/test/work/example/tracker"
	worktree = "/Users/test/work/example/tracker-issue-99"
	nativeWT = shared + "/.claude/worktrees/worker-a"
	// nestedWT mimics the #742 repro: a worktree nested directly under the
	// shared root but OUTSIDE the recognized .claude/worktrees carve-out
	// (e.g. an ad-hoc worktree, or any layout the guard doesn't special-case).
	// cwdInShared is true here even though the session isn't writing to the
	// shared checkout at all — the fix must key on the actual write target,
	// not on this cwd quirk.
	nestedWT = shared + "/jane"
)

func workerCfg() Config {
	return Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree}
}

func TestExempt(t *testing.T) {
	coordinator := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared, SharedOK: true}
	if !coordinator.Exempt() {
		t.Fatal("shared-homed session with WRITEGUARD_SHARED_OK must be exempt")
	}
	// #1035: shared-homed WITHOUT the opt-in token is no longer exempt.
	unclaimed := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}
	if unclaimed.Exempt() {
		t.Fatal("shared-homed session without WRITEGUARD_SHARED_OK must NOT be exempt")
	}
	// Exemption is keyed on the session HOME, not on where it cd'd to:
	// a worktree-homed session that cd'd into the shared checkout stays guarded.
	wanderer := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: shared, SharedOK: true}
	if wanderer.Exempt() {
		t.Fatal("worktree-homed session must not become exempt by cd-ing into the shared checkout")
	}
	if workerCfg().Exempt() {
		t.Fatal("worktree-homed session must not be exempt")
	}
	native := Config{SharedRoot: shared, ProjectDir: nativeWT, Cwd: nativeWT, SharedOK: true}
	if native.Exempt() {
		t.Fatal("native-worktree-homed session must not be exempt, token or not")
	}
}

// TestIssue1007ExemptionKeysOnCwd reproduces #1007: subagents dispatched
// with isolation:worktree (and EnterWorktree sessions) inherit
// CLAUDE_PROJECT_DIR = the shared checkout while operating from a linked
// worktree — under the old ProjectDir==SharedRoot rule they were silently
// EXEMPT, making the backstop inert for exactly the sessions it
// exists to guard. Exemption must key on where the call actually operates.
func TestIssue1007ExemptionKeysOnCwd(t *testing.T) {
	// SharedOK is set on every not-exempt case: even WITH the #1035 opt-in
	// token (which dispatched sessions may inherit from a coordinator
	// shell's environment), these populations must stay guarded — the
	// token never substitutes for genuinely operating from the shared
	// checkout.
	notExempt := map[string]Config{
		// Harness-worktree subagent: project dir = shared, cwd = .claude/worktrees/<x>.
		"harness worktree subagent": {SharedRoot: shared, ProjectDir: shared, Cwd: nativeWT, SharedOK: true},
		"harness worktree subdir":   {SharedRoot: shared, ProjectDir: shared, Cwd: nativeWT + "/tools", SharedOK: true},
		// EnterWorktree session: project dir = shared, cwd = sibling worktree.
		"sibling worktree session": {SharedRoot: shared, ProjectDir: shared, Cwd: worktree, SharedOK: true},
		// Ad-hoc worktree nested inside the shared root, detected via its
		// actual git toplevel (CwdTop).
		"nested ad-hoc worktree": {SharedRoot: shared, ProjectDir: shared, Cwd: nestedWT, CwdTop: nestedWT, SharedOK: true},
	}
	for name, cfg := range notExempt {
		t.Run("not-exempt/"+name, func(t *testing.T) {
			if cfg.Exempt() {
				t.Fatalf("config %+v must NOT be exempt", cfg)
			}
		})
	}

	// Exempt cases all carry the #1035 opt-in token — since #1035 the
	// shared-homed exemption exists only when it is explicitly claimed.
	exempt := map[string]Config{
		// The coordinator's sanctioned shared-checkout flow: cwd genuinely
		// inside the shared checkout.
		"coordinator at root":      {SharedRoot: shared, ProjectDir: shared, Cwd: shared, SharedOK: true},
		"coordinator in subdir":    {SharedRoot: shared, ProjectDir: shared, Cwd: shared + "/docs/streams", SharedOK: true},
		"coordinator with cwd top": {SharedRoot: shared, ProjectDir: shared, Cwd: shared, CwdTop: shared, SharedOK: true},
		// Malformed payload without cwd: fall back to the project-dir answer
		// rather than blocking the coordinator outright.
		"empty cwd fallback": {SharedRoot: shared, ProjectDir: shared, Cwd: "", SharedOK: true},
	}
	for name, cfg := range exempt {
		t.Run("exempt/"+name, func(t *testing.T) {
			if !cfg.Exempt() {
				t.Fatalf("config %+v must stay exempt", cfg)
			}
		})
	}
}

// TestIssue1007HarnessWorktreeGuarded proves the newly-guarded population is
// actually guarded end to end: a harness-worktree subagent writing to the
// shared checkout blocks, while its own-worktree writes pass — and the block
// message points at ITS worktree as home, not the shared checkout.
func TestIssue1007HarnessWorktreeGuarded(t *testing.T) {
	cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: nativeWT}
	if v := cfg.CheckFilePath("Write", shared+"/STATUS.md"); !v.Block {
		t.Fatal("harness-worktree subagent writing to the shared checkout must block")
	}
	if v := cfg.CheckFilePath("Write", nativeWT+"/notes.md"); v.Block {
		t.Fatalf("harness-worktree subagent writing inside its own worktree must pass: %s", v.Reason)
	}
	v := cfg.CheckBash("echo hi > " + shared + "/STATUS.md")
	if !v.Block {
		t.Fatal("harness-worktree subagent bash-writing to the shared checkout must block")
	}
	if !strings.Contains(v.Reason, "("+nativeWT+")") {
		t.Fatalf("block reason must name the worktree as home, not the shared checkout: %s", v.Reason)
	}
	if v := cfg.CheckBash("echo hi > " + nativeWT + "/notes.md"); v.Block {
		t.Fatalf("harness-worktree subagent bash-writing inside its own worktree must pass: %s", v.Reason)
	}
}

func TestCheckFilePath(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		block bool
	}{
		{"shared root file", shared + "/STATUS.md", true},
		{"shared docs/streams", shared + "/docs/streams/findings/x.md", true},
		{"shared tools file (isolation vector)", shared + "/tools/statusgen/alarms.go", true},
		{"own worktree", worktree + "/tools/statusgen/alarms.go", false},
		{"native worktree under shared/.claude/worktrees", nativeWT + "/tools/x.go", false},
		{"sibling repo", "/Users/test/work/example/agents/src/x.ts", false},
		{"scratchpad", "/private/tmp/claude-501/whatever/scratch.md", false},
		{"prefix-sibling dir does not false-positive", shared + "-v4/notes.md", false},
	}
	cfg := workerCfg()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cfg.CheckFilePath("Write", tc.path)
			if v.Block != tc.block {
				t.Fatalf("path %q: got block=%v want %v (reason: %s)", tc.path, v.Block, tc.block, v.Reason)
			}
		})
	}
}

func TestCheckFilePathRelative(t *testing.T) {
	// Relative path resolves against the payload cwd.
	cfg := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: shared}
	if v := cfg.CheckFilePath("Edit", "STATUS.md"); !v.Block {
		t.Fatal("relative path with cwd inside the shared checkout must block")
	}
	cfg.Cwd = worktree
	if v := cfg.CheckFilePath("Edit", "STATUS.md"); v.Block {
		t.Fatal("relative path with cwd inside own worktree must pass")
	}
}

func TestCheckFilePathExemptSessionNeverBlocks(t *testing.T) {
	cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared, SharedOK: true}
	if !cfg.Exempt() {
		t.Fatal("expected exempt")
	}
	// main() returns before checks when exempt; assert the invariant here too.
	if v := cfg.CheckFilePath("Write", shared+"/STATUS.md"); v.Block && cfg.Exempt() {
		t.Log("CheckFilePath blocks in isolation, but Exempt() short-circuits it in run()")
	}
}

func TestCheckBashBlocked(t *testing.T) {
	cases := []struct{ name, cmd string }{
		{"cd into shared (confirmed isolation vector)", "cd " + shared + " && go run ./tools/statusgen --root ."},
		{"pushd into shared", "pushd " + shared + "; make"},
		{"redirect into shared", "echo hi > " + shared + "/STATUS.md"},
		{"append into shared", "cat x >> " + shared + "/docs/notes.md"},
		{"cp into shared", "cp /tmp/x.go " + shared + "/tools/statusgen/x.go"},
		{"mv into shared", "mv x " + shared + "/y"},
		{"rm in shared", "rm -rf " + shared + "/frontend/dist"},
		{"tee into shared", "make | tee " + shared + "/build.log"},
		{"sed -i in shared", "sed -i '' 's/a/b/' " + shared + "/README.md"},
		{"git -C shared commit", "git -C " + shared + " commit -m x"},
		{"git -C shared checkout (branch-switch crime)", "git -C " + shared + " checkout -b feature/x"},
		{"git -C shared restore", "git -C " + shared + " restore ."},
		{"statusgen against shared root", "go run ./tools/statusgen --root " + shared},
		{"relative write while cwd in shared", "touch STATUS.md"},
		{"cd into shared even with read-only follow-up", "cd " + shared + " && ls -la"},
		{"cd into shared subdirectory", "cd " + shared + "/docs/streams && ls"},
		{"multiple cd, one into shared", "cd " + worktree + " && cd " + shared + " && ls"},
		// Relative traversal from a native worktree cwd.
		{"relative cd traversal from native worktree", "cd ../../.. && go run ./tools/statusgen --root ."},
		{"relative redirect traversal from native worktree", "echo hi > ../../../STATUS.md"},
		{"relative git commit traversal from native worktree", "git -C ../../.. commit -am x"},
		// Relative traversal from an external (outside-shared) worktree cwd.
		{"relative cd traversal from external worktree", "cd ../tracker && go run ./tools/statusgen --root ."},
		{"relative redirect traversal from external worktree", "echo hi > ../tracker/STATUS.md"},
		// Dotted traversal that launders the root through .claude/worktrees.
		{"dotted traversal redirect", "echo hi > " + shared + "/.claude/worktrees/../../STATUS.md"},
		{"dotted traversal cd into shared", "cd " + shared + "/.claude/worktrees/../.. && go run ./tools/statusgen --root ."},
		// Absolute paths must still be blocked when Cwd is empty (no relative resolution needed).
		{"absolute path, empty cwd — redirect", "echo hi > " + shared + "/STATUS.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := workerCfg()
			if tc.name == "relative write while cwd in shared" {
				cfg.Cwd = shared
			}
			// Native-worktree traversal tests run from a native-worktree cwd.
			if strings.Contains(tc.name, "from native worktree") || strings.Contains(tc.name, "dotted traversal") {
				cfg.Cwd = nativeWT
			}
			if strings.Contains(tc.name, "empty cwd") {
				cfg.Cwd = ""
			}
			if v := cfg.CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q: expected block", tc.cmd)
			}
		})
	}
}

func TestCheckBashAllowed(t *testing.T) {
	cases := []struct{ name, cmd string }{
		{"worktree add from shared (sanctioned isolation recipe)", "git -C " + shared + " worktree add /tmp/x -b feat/x origin/main"},
		{"git fetch shared", "git -C " + shared + " fetch origin main"},
		{"git log shared", "git -C " + shared + " log --oneline -5"},
		{"git status shared", "git -C " + shared + " status --porcelain"},
		{"read a shared file", "cat " + shared + "/README.md"},
		{"grep shared", "grep -r foo " + shared + "/docs"},
		{"write into native worktree", "echo hi > " + nativeWT + "/notes.md"},
		{"cd into native worktree", "cd " + nativeWT + " && go test ./..."},
		{"cd into native worktree, mention shared in read", "cd " + nativeWT + "/tools && git -C " + shared + " fetch origin main"},
		{"cd into native worktree, git log shared", "cd " + nativeWT + " && git -C " + shared + " log --oneline -5"},
		{"cd into native worktree, commit inside it", "cd " + nativeWT + " && git add -A && git commit -m fix"},
		{"pushd into native worktree, then read shared", "pushd " + nativeWT + " && ls " + shared + "/docs"},
		{"writes in own worktree", "cd " + worktree + " && go run ./tools/statusgen --root . && git add -A && git commit -m x"},
		{"sibling repo write", "cp x /Users/test/work/example/agents/y"},
		{"unrelated command", "go test ./... && ls -la"},
		// Relative writes that stay inside the worktree.
		{"relative write inside own worktree", "echo hi > ./notes.md"},
		{"relative cd inside own worktree", "cd ./subdir && go test ./..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
			}
		})
	}
}

// TestIssue147_GoTestVetOnStatusgenPackage pins #147: `go test`/`go vet`
// naming the statusgen package by path (e.g. "./tools/statusgen") must
// never block, on any session shape, because neither subcommand executes
// statusgen's main() — the only thing that actually writes STATUS.md.
// Only `go run` does that, and isInvocation already encodes exactly this
// distinction (a tool name only counts as an INVOCATION when the command's
// leading verb is "go run"; every other go subcommand — test, vet, build,
// install, doc — is a mention, not a write). That is what makes #147's
// literal repro pass today without a change to the decision logic.
//
// This test exists so a future edit that widens isInvocation's go-run gate
// back to "any go subcommand" (e.g. while adding support for some other
// tool) cannot silently reopen the exact false-block #147 reported — it
// would fail here first, against the precise command shapes and session
// populations #147 named (verify-desk running directly from the shared
// checkout, and a dispatched verifier agent that inherits the parent's
// shared ProjectDir while its own cwd is a private worktree).
func TestIssue147_GoTestVetOnStatusgenPackage(t *testing.T) {
	passCases := []struct{ name, cmd string }{
		{"go test package path", "go test ./tools/statusgen"},
		{"go vet package path", "go vet ./tools/statusgen"},
		{"go test with -run flag", "go test -run TestFoo ./tools/statusgen"},
		{"go test recursive wildcard", "go test ./tools/statusgen/..."},
		{"go vet recursive wildcard", "go vet ./tools/statusgen/..."},
		{"go test with -v and -count", "go test -v -count=1 ./tools/statusgen"},
		{"go build (compiles, never runs main)", "go build ./tools/statusgen"},
	}

	t.Run("worker (worktree-homed)", func(t *testing.T) {
		for _, tc := range passCases {
			t.Run(tc.name, func(t *testing.T) {
				if v := workerCfg().CheckBash(tc.cmd); v.Block {
					t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
				}
			})
		}
	})

	// verify-desk itself: genuinely homed AND cwd'd in the shared checkout
	// (no cd at all) — the first population #147 names. This is what makes
	// CheckBash even inspect the command (mentions/cwdInShared both true);
	// it must still pass because the command writes nothing.
	t.Run("shared-homed (verify-desk repro)", func(t *testing.T) {
		cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}
		for _, tc := range passCases {
			t.Run(tc.name, func(t *testing.T) {
				if v := cfg.CheckBash(tc.cmd); v.Block {
					t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
				}
			})
		}
	})

	// A dispatched verifier agent: ProjectDir inherited as the shared root
	// (#1007's population), but Cwd/CwdTop genuinely resolve to its own
	// /private/tmp worktree — the second population #147 names.
	t.Run("dispatched worker inheriting shared ProjectDir", func(t *testing.T) {
		dispatchWT := "/private/tmp/claude-501/verify-desk-worker/scratchpad-wt"
		cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: dispatchWT, CwdTop: dispatchWT}
		for _, tc := range passCases {
			t.Run(tc.name, func(t *testing.T) {
				if v := cfg.CheckBash(tc.cmd); v.Block {
					t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
				}
			})
		}
	})

	// Genuine violations must stay caught: #147's fix must not widen into
	// `go run` (which DOES execute statusgen's main()) losing its guard.
	t.Run("genuine violations still blocked", func(t *testing.T) {
		blockCases := []struct{ name, cmd string }{
			{"go run against shared root, no --root (cwd default)", "go run ./tools/statusgen"},
			{"go run --root pointed at shared", "go run ./tools/statusgen --root " + shared},
			{"bare statusgen invocation", "statusgen"},
		}
		cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}
		for _, tc := range blockCases {
			t.Run(tc.name, func(t *testing.T) {
				if v := cfg.CheckBash(tc.cmd); !v.Block {
					t.Fatalf("command %q: expected block, got pass", tc.cmd)
				}
			})
		}
	})
}

func TestResolvePathSymlinks(t *testing.T) {
	// Real dirs: a symlink to the "shared" root must not evade the guard.
	base := t.TempDir()
	realShared := filepath.Join(base, "shared")
	if err := os.MkdirAll(realShared, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(realShared, link); err != nil {
		t.Fatal(err)
	}
	cfg := Config{SharedRoot: realShared, ProjectDir: filepath.Join(base, "wt"), Cwd: filepath.Join(base, "wt")}
	// Target through the symlink, file itself does not exist yet (Write).
	if v := cfg.CheckFilePath("Write", filepath.Join(link, "new-file.md")); !v.Block {
		t.Fatal("symlinked path into the shared root must block")
	}
}

// TestIssue742FalsePositives reproduces the two false-positive classes from
// example-org/tracker#742: a home-relative rm and a
// pure-API gh command, both run from a session whose cwd sits nested
// inside the shared checkout outside the .claude/worktrees carve-out
// (nestedWT) — the exact repro shape from the issue. Also covers the
// same commands from the sanctioned external/native worktree homes, and
// $HOME-prefixed paths, and a bare no-op-looking `cd ~/.claude`.
func TestIssue742FalsePositives(t *testing.T) {
	cwds := map[string]string{
		"nested-outside-worktrees (issue repro)": nestedWT,
		"external sibling worktree":              worktree,
		"native .claude/worktrees":               nativeWT,
	}
	cmds := []string{
		"rm ~/.config/assay/claims/x.claim",
		"rm $HOME/.config/assay/claims/x.claim",
		"gh issue create --title x --body y",
		"gh pr create --title 'fix' --body 'body text'",
	}
	for cwdName, cwd := range cwds {
		for _, cmd := range cmds {
			t.Run(cwdName+"/"+cmd, func(t *testing.T) {
				cfg := Config{SharedRoot: shared, ProjectDir: cwd, Cwd: cwd}
				if v := cfg.CheckBash(cmd); v.Block {
					t.Fatalf("command %q from cwd %s: unexpected block: %s", cmd, cwd, v.Reason)
				}
			})
		}
	}
}

// TestIssue742TruePositivesPreserved demonstrates the isolation backstop still
// blocks a genuine shared-checkout write, including from the same
// nested-but-uncarved-out cwd shape used above to prove the fix didn't
// just widen the exemption blindly.
func TestIssue742TruePositivesPreserved(t *testing.T) {
	cwds := map[string]string{
		"nested-outside-worktrees (issue repro)": nestedWT,
		"external sibling worktree":              worktree,
	}
	for cwdName, cwd := range cwds {
		t.Run(cwdName+"/rm shared file", func(t *testing.T) {
			cfg := Config{SharedRoot: shared, ProjectDir: cwd, Cwd: cwd}
			cmd := "rm " + shared + "/somefile"
			if v := cfg.CheckBash(cmd); !v.Block {
				t.Fatalf("command %q from cwd %s: expected block", cmd, cwd)
			}
		})
		t.Run(cwdName+"/append into shared file", func(t *testing.T) {
			cfg := Config{SharedRoot: shared, ProjectDir: cwd, Cwd: cwd}
			cmd := "echo hi >> " + shared + "/file"
			if v := cfg.CheckBash(cmd); !v.Block {
				t.Fatalf("command %q from cwd %s: expected block", cmd, cwd)
			}
		})
	}
	// Bare relative write from a cwd nested in the shared checkout must
	// still block — the argument-aware fix must not treat "no ~ present"
	// as "not a shared write".
	t.Run("relative rm from nested cwd still blocks", func(t *testing.T) {
		cfg := Config{SharedRoot: shared, ProjectDir: nestedWT, Cwd: nestedWT}
		if v := cfg.CheckBash("rm dist/output.bin"); !v.Block {
			t.Fatal("relative rm with cwd nested in the shared checkout must still block")
		}
	})
}

// TestCompoundCommandBypassBlocked verifies that FindAllStringIndex
// (not FindStringIndex) is used for target-aware indicators, closing the
// bypass where a safe first occurrence passes the check and subsequent
// dangerous occurrences go uninspected (#742 review finding).
func TestCompoundCommandBypassBlocked(t *testing.T) {
	cfg := workerCfg()
	blockCases := []string{
		// Safe first redirect, dangerous second — must still block.
		"echo ok > /tmp/x && echo pwn >> " + shared + "/STATUS.md",
		// Safe first mutation, dangerous second.
		"mkdir /tmp/x && rm -rf " + shared + "/docs",
		// Safe touch, dangerous mv.
		"touch /tmp/y && mv src " + shared + "/dst",
		// Safe first redirect to worktree, dangerous second to shared.
		"echo ok > " + worktree + "/notes.md && echo pwn >> " + shared + "/STATUS.md",
		// Two different target-aware indicators (tee + redirect) — both must be evaluated.
		"tee /dev/null && echo pwn > " + shared + "/STATUS.md",
		// Redirect safe, then tee into shared.
		"echo ok > /dev/null && echo pwn | tee " + shared + "/build.log",
	}
	for _, cmd := range blockCases {
		t.Run("block:"+cmd[:minLen(cmd, 40)], func(t *testing.T) {
			if v := cfg.CheckBash(cmd); !v.Block {
				t.Fatalf("command %q: expected block", cmd)
			}
		})
	}

	// Compound commands where ALL writes are safe should still pass.
	allowCases := []string{
		// Both redirects safe.
		"echo ok > /tmp/x && echo ok2 >> /tmp/y",
		// Safe mkdir and rm outside shared.
		"mkdir /tmp/x && rm -rf /tmp/x",
		// Redirect into worktree is fine.
		"echo ok > " + worktree + "/notes.md && rm " + worktree + "/old.md",
		// tee + redirect, both to safe locations.
		"tee /tmp/log && echo ok > /tmp/status",
	}
	for _, cmd := range allowCases {
		t.Run("allow:"+cmd[:minLen(cmd, 40)], func(t *testing.T) {
			if v := cfg.CheckBash(cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", cmd, v.Reason)
			}
		})
	}
}

// TestIssue751SpacelessRedirectBlocked reproduces #751: the redirection
// regex's prefix used to require whitespace/digit immediately before the
// operator, so a word char directly preceding `>`/`>>` (no space) evaded
// detection entirely and the write target was never checked.
func TestIssue751SpacelessRedirectBlocked(t *testing.T) {
	cfg := workerCfg()
	blockCases := []string{
		"echo c>>" + shared + "/STATUS.md",
		"echo c>" + shared + "/STATUS.md",
	}
	for _, cmd := range blockCases {
		t.Run("block:"+cmd, func(t *testing.T) {
			if v := cfg.CheckBash(cmd); !v.Block {
				t.Fatalf("command %q: expected block (space-less redirect into shared checkout)", cmd)
			}
		})
	}
	// Relative space-less redirect with cwd inside the shared checkout must
	// also block (mirrors the existing "relative write while cwd in shared"
	// case, but with no space before the operator).
	t.Run("block: relative space-less redirect, cwd in shared", func(t *testing.T) {
		relCfg := cfg
		relCfg.Cwd = shared
		if v := relCfg.CheckBash("echo c>>STATUS.md"); !v.Block {
			t.Fatal("expected block: relative space-less redirect with cwd inside the shared checkout")
		}
	})
}

// TestIssue751SpacelessRedirectAllowed proves the broadened prefix does not
// introduce false positives: legitimate space-less redirects to the caller's
// own worktree, to ~/.claude/**, and to any other path outside the shared
// checkout must still pass, exactly as their spaced equivalents do.
func TestIssue751SpacelessRedirectAllowed(t *testing.T) {
	cfg := workerCfg()
	allowCases := []string{
		"echo c>>" + worktree + "/notes.md",
		"echo c>" + worktree + "/notes.md",
		"echo c>>~/.config/assay/claims/x.claim",
		"echo c>/tmp/scratch.txt",
	}
	for _, cmd := range allowCases {
		t.Run("allow:"+cmd, func(t *testing.T) {
			if v := cfg.CheckBash(cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", cmd, v.Reason)
			}
		})
	}
}

// TestIssue1006TargetAwareIndicators reproduces #1006: the formerly-bare
// indicators (statusgen, mutating git subcommand, bazel build) denied on mere
// MENTION of a shared-checkout path even when the write landed in the
// worker's own worktree — blocking the sanctioned fanout boot shape
// (`git -C <shared> fetch … && go run ./tools/statusgen --root .`). They
// must now key on the resolved WRITE target: -C/--git-dir/--work-tree for
// git, --root for statusgen, cwd otherwise.
func TestIssue1006TargetAwareIndicators(t *testing.T) {
	allowed := []struct{ name, cmd string }{
		{"fanout boot shape (issue repro)", "git -C " + shared + " fetch origin && go run ./tools/statusgen --root ."},
		{"shared read + own-worktree git write", "git -C " + shared + " fetch origin && git add -A && git commit -m x"},
		{"shared read + bazel build in own worktree", "git -C " + shared + " log --oneline -3 && bazel build"},
		{"shared read + statusgen --root=own", "git -C " + shared + " fetch origin && go run ./tools/statusgen --root=."},
		{"shared file read + statusgen", "cat " + shared + "/README.md && go run ./tools/statusgen --root ."},
		{"statusgen rooted at own worktree, shared mentioned", "cat " + shared + "/STATUS.md && go run ./tools/statusgen --root " + worktree},
		{"git -C own worktree commit, shared mentioned", "git -C " + shared + " log -1 && git -C " + worktree + " commit -m x"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
			}
		})
	}

	blocked := []struct {
		name, cmd string
		cwd       string // "" = workerCfg default (worktree)
	}{
		{"statusgen rooted at shared", "go run ./tools/statusgen --root " + shared, ""},
		{"statusgen --root=shared", "go run ./tools/statusgen --root=" + shared, ""},
		{"compound: safe fetch then statusgen at shared", "git -C " + shared + " fetch origin && go run ./tools/statusgen --root " + shared, ""},
		{"compound: own commit then shared commit", "git -C " + worktree + " commit -m ok && git -C " + shared + " commit -m x", ""},
		{"git --git-dir=shared commit", "git --git-dir=" + shared + "/.git commit -m x", ""},
		{"git --work-tree shared checkout", "git --work-tree " + shared + " checkout -- .", ""},
		{"statusgen defaulting to cwd inside shared", "go run ./tools/statusgen", shared},
		{"bazel build with cwd inside shared", "bazel build", shared},
		{"git add relative with cwd inside shared", "git add -A", shared},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			cfg := workerCfg()
			if tc.cwd != "" {
				cfg.Cwd = tc.cwd
			}
			if v := cfg.CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q (cwd %s): expected block", tc.cmd, cfg.Cwd)
			}
		})
	}

	// Unknown cwd + cwd-relative indicator = fail safe: the write target
	// cannot be established, so a shared-mentioning command must block, not
	// pass silently.
	t.Run("block/fail-safe on unknown cwd", func(t *testing.T) {
		cfg := workerCfg()
		cfg.Cwd = ""
		cmd := "cat " + shared + "/README.md && go run ./tools/statusgen"
		if v := cfg.CheckBash(cmd); !v.Block {
			t.Fatalf("command %q with unknown cwd: expected fail-safe block", cmd)
		}
	})
}

// TestQuotedSubshellCdAndEnvChdirBlocked covers the PR #1017 review blocker:
// three shapes whose writes land in the shared checkout were blocked pre-#1006
// (incidentally, by the bare git/statusgen indicators firing on mention) and
// fell open once indicators became target-aware — the cd lived inside a quoted
// subshell string that cdIntoDir's prefix class didn't match, and env -C is a
// cd equivalent the guard didn't model. Both are now handled explicitly.
func TestQuotedSubshellCdAndEnvChdirBlocked(t *testing.T) {
	blocked := []struct{ name, cmd string }{
		// The reviewer's three regression shapes, verbatim.
		{"sh -c single-quoted cd into shared", "sh -c 'cd " + shared + " && git commit -m x'"},
		{"bash -c double-quoted cd into shared", `bash -c "cd ` + shared + ` && statusgen"`},
		{"env -C shared git commit", "env -C " + shared + " git commit -m x"},
		// env chdir flag variants.
		{"env --chdir shared statusgen", "env --chdir " + shared + " statusgen"},
		{"env --chdir=shared git commit", "env --chdir=" + shared + " git commit -m x"},
		{"env with assignment before -C", "env FOO=1 -C " + shared + " git commit -m x"},
		// Quoted-subshell variants.
		{"bash -c double-quoted pushd into shared", `bash -c "pushd ` + shared + `; make"`},
		{"sh -c cd into shared subdir", "sh -c 'cd " + shared + "/docs && touch x'"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q: expected block", tc.cmd)
			}
		})
	}

	allowed := []struct{ name, cmd string }{
		// env scanning stops at the wrapped command: make -C shared builds
		// there but is not a chdir (passed before this PR too — unchanged).
		{"env make -C shared (not a chdir)", "env make -C " + shared + " docs"},
		{"make -C shared (no env)", "make -C " + shared + " docs"},
		// cd/env -C into the session's OWN worktree stays fine.
		{"sh -c cd into own worktree", "sh -c 'cd " + worktree + " && git commit -m x'"},
		{"env -C own worktree git commit", "env -C " + worktree + " git commit -m x"},
		// env with assignments only, wrapped command reads shared.
		{"env assignment + shared read", "env FOO=1 git -C " + shared + " log -1"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
			}
		})
	}
}

// TestIssue1026PathPrefixedCommands reproduces #1026: the env chdir check
// added for the PR #1017 review matched only a bare `env` — its prefix class
// `[\s;&|('"]` has no "/", so a path-prefixed `/usr/bin/env -C <shared>`
// evaded it. Same mechanism, same file: the file-mutation indicator's prefix
// class `[\s;&|(]` also lacked "/", so `/bin/rm -rf <shared>/dist` passed.
// Both classes now include "/" (and quotes), matching commands by BASENAME;
// the word-char before the "env" in dotenv/myenv keeps those from tripping.
// The remaining indicators (git, tee, sed, statusgen, bazel) use \b, which
// already treats "/" as a boundary — covered by the block cases below.
func TestIssue1026PathPrefixedCommands(t *testing.T) {
	blocked := []struct{ name, cmd string }{
		// The issue's evasion shape, verbatim mechanism.
		{"path-prefixed env -C shared git commit", "/usr/bin/env -C " + shared + " git commit -m x"},
		// env chdir flag variants, mirroring the #1017 cases.
		{"path-prefixed env --chdir shared statusgen", "/bin/env --chdir " + shared + " statusgen"},
		{"path-prefixed env --chdir=shared git commit", "/usr/bin/env --chdir=" + shared + " git commit -m x"},
		{"path-prefixed env with assignment before -C", "/usr/bin/env FOO=1 -C " + shared + " git commit -m x"},
		{"quoted path-prefixed env -C shared", `"/usr/bin/env" -C ` + shared + ` git commit -m x`},
		{"path-prefixed env after semicolon", "true; /usr/bin/env -C " + shared + " git commit -m x"},
		// Same mechanism in the file-mutation indicator's prefix class.
		{"path-prefixed rm of shared path", "/bin/rm -rf " + shared + "/dist"},
		{"path-prefixed cp into shared", "/bin/cp x.txt " + shared + "/README.md"},
		{"path-prefixed install into shared", "/usr/bin/install -m 644 x " + shared + "/x"},
		// \b-based indicators already match path-prefixed forms — pin that.
		{"path-prefixed git -C shared commit", "/usr/bin/git -C " + shared + " commit -m x"},
		{"path-prefixed tee into shared", "echo x | /usr/bin/tee " + shared + "/STATUS.md"},
		{"path-prefixed sed -i on shared file", "/usr/bin/sed -i '' 's/a/b/' " + shared + "/README.md"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q: expected block", tc.cmd)
			}
		})
	}

	allowed := []struct{ name, cmd string }{
		// dotenv/myenv-style names must not trip the basename env match:
		// the char before their "env" is a word char, outside the class.
		{"dotenv -C shared is not env", "dotenv -C " + shared + " reader"},
		{"path-prefixed dotenv -C shared is not env", "./scripts/dotenv -C " + shared + " reader"},
		{"myenv --chdir shared is not env", "myenv --chdir " + shared + " reader"},
		// A shared path whose component is literally "env" scans past the
		// match without finding -C/--chdir — read stays a read.
		{"shared path with env component", "ls " + shared + "/env"},
		// Path-prefixed env into the session's OWN worktree stays fine.
		{"path-prefixed env -C own worktree", "/usr/bin/env -C " + worktree + " git commit -m x"},
		// Path-prefixed env with assignments only, wrapped command reads shared.
		{"path-prefixed env assignment + shared read", "/usr/bin/env FOO=1 git -C " + shared + " log -1"},
		// Path-prefixed mutation whose own target is outside shared.
		{"shared read + path-prefixed rm of own dist", "cat " + shared + "/README.md && /bin/rm -rf ./dist"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q: unexpected block: %s", tc.cmd, v.Reason)
			}
		})
	}
}

// TestIssue1035SharedHomedOptIn covers #1035 (the owner's "1+2" decision, option 2):
// the shared-homed exemption is OPT-IN via WRITEGUARD_SHARED_OK. A session
// homed in the shared checkout WITHOUT the token gets blocked on shared-
// checkout writes — with a message naming the token and the isolation
// alternative — while the SAME session with the token keeps the pre-#1035
// exempt behavior. The token never widens anything else: worktree-homed
// sessions stay guarded (with the generic isolation message, not the opt-in one),
// and out-of-shared writes from a shared-homed session stay allowed either way.
func TestIssue1035SharedHomedOptIn(t *testing.T) {
	sharedHomedCfg := func(ok bool) Config {
		return Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared, SharedOK: ok}
	}

	// The blocked-path table drives both tool families through the full
	// hook decision (Exempt gate + check), mirroring run()'s order.
	decide := func(cfg Config, tool, target string) Verdict {
		if cfg.Exempt() {
			return Verdict{}
		}
		if tool == "Bash" {
			return cfg.CheckBash(target)
		}
		return cfg.CheckFilePath(tool, target)
	}

	cases := []struct {
		name      string
		tool      string // "Bash" or a file tool
		target    string // file path or bash command
		withToken bool
		block     bool
	}{
		// Without the token: shared-homed writes to the shared checkout block.
		{"no token: Write shared file", "Write", shared + "/STATUS.md", false, true},
		{"no token: Edit shared stream README", "Edit", shared + "/docs/streams/example-stream/README.md", false, true},
		{"no token: bash redirect into shared", "Bash", "echo hi > " + shared + "/STATUS.md", false, true},
		{"no token: bash relative write, cwd in shared", "Bash", "touch STATUS.md", false, true},
		{"no token: bash git commit in shared", "Bash", "git add -A && git commit -m x", false, true},
		// With the token: identical calls keep the pre-#1035 exempt behavior.
		{"token: Write shared file", "Write", shared + "/STATUS.md", true, false},
		{"token: Edit shared stream README", "Edit", shared + "/docs/streams/example-stream/README.md", true, false},
		{"token: bash redirect into shared", "Bash", "echo hi > " + shared + "/STATUS.md", true, false},
		{"token: bash relative write, cwd in shared", "Bash", "touch STATUS.md", true, false},
		{"token: bash git commit in shared", "Bash", "git add -A && git commit -m x", true, false},
		// Writes OUTSIDE the shared checkout from a shared-homed session are
		// fine with or without the token (the guard only protects shared).
		{"no token: Write to /tmp", "Write", "/private/tmp/scratch/notes.md", false, false},
		{"no token: Write to sibling repo", "Write", "/Users/test/work/example/agents/x.ts", false, false},
		{"no token: bash write to native worktree", "Bash", "echo hi > " + nativeWT + "/notes.md", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := decide(sharedHomedCfg(tc.withToken), tc.tool, tc.target)
			if v.Block != tc.block {
				t.Fatalf("%s %q (token=%v): got block=%v want %v (reason: %s)",
					tc.tool, tc.target, tc.withToken, v.Block, tc.block, v.Reason)
			}
		})
	}

	// The opt-in block message must name the token and the isolation
	// alternative — that message IS the discovery mechanism for both the
	// coordinator (export the token) and a mis-homed worker (make a worktree).
	t.Run("block message names token and worktree alternative", func(t *testing.T) {
		for tool, target := range map[string]string{
			"Write": shared + "/STATUS.md",
			"Bash":  "echo hi > " + shared + "/STATUS.md",
		} {
			v := decide(sharedHomedCfg(false), tool, target)
			if !v.Block {
				t.Fatalf("%s %q: expected block", tool, target)
			}
			for _, want := range []string{
				"WRITEGUARD_SHARED_OK=1",
				"git worktree add",
				"#1035",
				shared, // names the shared checkout it is protecting
			} {
				if !strings.Contains(v.Reason, want) {
					t.Fatalf("%s block reason must contain %q, got:\n%s", tool, want, v.Reason)
				}
			}
		}
	})

	// Worktree-homed sessions keep the generic isolation message — the opt-in
	// message (with its "export the token" suggestion) must never be shown
	// to a session that should isolate-or-stay-away regardless of any token.
	t.Run("worktree-homed block keeps generic message", func(t *testing.T) {
		v := workerCfg().CheckFilePath("Write", shared+"/STATUS.md")
		if !v.Block {
			t.Fatal("expected block")
		}
		if strings.Contains(v.Reason, "OPT-IN") {
			t.Fatalf("worktree-homed session must get the generic isolation message, got:\n%s", v.Reason)
		}
		if !strings.Contains(v.Reason, worktree) {
			t.Fatalf("generic message must name the session's own worktree as home, got:\n%s", v.Reason)
		}
	})

	// A worktree-homed session that inherited the token from a coordinator
	// shell stays fully guarded — same verdicts as without it.
	t.Run("token does not rescue worktree-homed sessions", func(t *testing.T) {
		cfg := workerCfg()
		cfg.SharedOK = true
		if cfg.Exempt() {
			t.Fatal("worktree-homed session with the token must not be exempt")
		}
		if v := cfg.CheckFilePath("Write", shared+"/STATUS.md"); !v.Block {
			t.Fatal("worktree-homed session with the token must still block shared writes")
		}
		if v := cfg.CheckBash("echo hi > " + shared + "/STATUS.md"); !v.Block {
			t.Fatal("worktree-homed session with the token must still block shared bash writes")
		}
	})
}

// TestIssue1190FalsePositives reproduces #1190: the guard blocked commands
// that write nothing at all, because it matched indicator keywords in LITERAL
// TEXT and resolved relative targets in the wrong cwd frame. Every case here
// was observed blocking a real desk session.
//
// The cwd is the SHARED checkout for most cases on purpose: a dispatched
// agent's Bash cwd resets to the hook payload cwd between calls, which for a
// worker booted from the shared checkout is the shared checkout — so
// cwdInShared is true and the command reaches the indicators (#1190 ev. 4).
func TestIssue1190FalsePositives(t *testing.T) {
	// A worker that owns `worktree` but whose Bash payload cwd is `shared`.
	resetCwd := func() Config {
		cfg := workerCfg()
		cfg.Cwd = shared
		return cfg
	}

	allowed := []struct {
		name, cmd string
		cwd       string // "" = payload cwd reset to shared
	}{
		// (1) A read-only grep whose PATH contains the board generator's name.
		{"grep of a statusgen source file in own worktree", "grep -rn 'foo' " + worktree + "/tools/statusgen/registers.go", ""},
		{"grep for the word statusgen in own worktree", "grep -rn statusgen " + worktree + "/tools", ""},
		{"cat a statusgen source file", "cat " + worktree + "/tools/statusgen/registers.go", ""},
		// A quoted grep alternation that happens to contain command names
		// (observed blocking as a "file mutation command" — the `|mkdir|`).
		{"grep alternation containing command names", "grep -iE 'arrow|mkdir|sentinel' " + worktree + "/notes.txt", ""},
		// (2) `>` inside a quoted awk program is a comparison, not a redirect.
		{"awk comparison in a pipeline", "git -C " + shared + " log --oneline -5 | awk 'NF>1 {print $1}'", ""},
		{"awk comparison, double quotes", `ls -la | awk "NF>1 {print $1}"`, ""},
		{"jq comparison in quotes", `gh pr list --json number | jq '.[] | select(.number>1)'`, ""},
		// (3) Markdown blockquotes / ASCII arrows in an issue or PR body.
		{"gh issue body with a blockquote", "gh issue create --title x --body 'symptom:\n> BLOCKED by the guard\n'", ""},
		{"gh issue body with an ASCII arrow", "gh issue create --title x --body 'payload cwd->shared checkout'", ""},
		{"gh pr comment with an arrow", `gh pr comment 1 --body "state: todo->done"`, ""},
		{"gh issue body with a leading blockquote", "gh issue create --title x --body '> quoted line from the log'", ""},
		{"gh issue body via heredoc", "gh issue create --title x --body-file - <<'EOF'\n> quoted line\nstatusgen writes STATUS.md\nEOF", ""},
		// (4) A leading cd into the worker's own worktree sets the frame for
		// relative targets — the write lands in the worktree, not in shared.
		{"cd worktree then statusgen --root . --lint", "cd " + worktree + " && go run ./tools/statusgen --root . --lint", ""},
		{"cd worktree then statusgen --root .", "cd " + worktree + " && go run ./tools/statusgen --root .", ""},
		{"cd worktree then git add/commit", "cd " + worktree + " && git add -A && git commit -m x", ""},
		{"cd worktree then relative redirect", "cd " + worktree + " && echo hi > notes.md", ""},
		{"cd worktree then bazel build", "cd " + worktree + " && bazel build", ""},
		{"cd worktree subdir then touch", "cd " + worktree + "/tools && touch x.go", ""},
		{"cd worktree then tee a relative file", "cd " + worktree + " && go test ./... 2>&1 | tee out.txt", ""},
		// (5) Read-only board modes write nothing, even rooted at shared.
		{"statusgen --lint at shared root", "go run ./tools/statusgen --root " + shared + " --lint", ""},
		{"statusgen --check at shared root", "go run ./tools/statusgen --root " + shared + " --check", ""},
		{"statusgen --dry-run at shared root", "go run ./tools/statusgen --root " + shared + " --dry-run", ""},
		{"statusgen --lint with cwd in shared", "go run ./tools/statusgen --lint", shared},
		// (6) Out-of-repo claim-file writes (#1190 ev. 2) — including content
		// that itself contains a redirect-looking character.
		{"claim file write with an arrow in the content", "printf 'state: blocked->retry\\n' > ~/.config/assay/claims/x.claim", ""},
		{"claim file write with a blockquote in the content", "printf 'note:\n> blocked\n' > $HOME/.config/assay/claims/x.claim", ""},
		{"mkdir the claim dir", "mkdir -p ~/.config/assay/claims", ""},
		// (7) A cd whose destination is a shell variable: the guard cannot
		// know where it goes, and <shared>/$SCRATCH is an invented path. A cd
		// it cannot model never advances the frame either, so the writes that
		// follow are still judged against the payload cwd (see the paired
		// "cd variable then …" block cases).
		{"cd into a variable scratch dir", "S=/tmp/scratch && cd $S && go test ./...", ""},
		{"cd into a braced variable", "cd ${SCRATCH} && ls", ""},
		{"cd into a command substitution", "cd $(mktemp -d) && ls", ""},
		// A variable target the COMMAND ITSELF binds to a literal is not
		// unknown — the guard resolves the binding and judges the real
		// destination. This is the scratch-redirect shape captured live from a
		// coordinator session (#1259 field validation).
		{"redirect into an assigned scratch path", `S=/tmp/scratch; go doc ./tools > "$S"/doc.txt`, ""},
		{"redirect into an assigned scratch path, && form", `SP=/tmp/scratch && go doc ./tools > "$SP/doc.txt"`, ""},
		{"copy into an assigned scratch path", "SCRATCH=/tmp/scratch; cp notes.md $SCRATCH/notes.md", ""},
		{"redirect into an assigned braced path", "D=/tmp && echo hi > ${D}/out.txt", ""},
		// An unresolvable head is judged against the frame, exactly as before
		// #1190 — from the worker's OWN frame that is not the shared checkout,
		// so it passes. (The paired shared-frame cases block; see
		// TestIssue1190TruePositivesPreserved.)
		{"unresolvable head from the worker's own frame", "cd " + worktree + " && echo hi > $SCRATCH/out.txt", ""},
		{"unresolvable head with cwd in the worktree", "cp notes.md $SCRATCH/notes.md", worktree},
		// (9) A copy whose SOURCE is in the shared checkout is a read: only
		// the destination is written.
		{"cp a shared file out to tmp", "cp " + shared + "/README.md /tmp/readme.md", ""},
		{"cp a shared file into own worktree", "cp " + shared + "/docs/x.md " + worktree + "/docs/x.md", ""},
		{"rsync shared docs out to tmp", "rsync -a " + shared + "/docs/ /tmp/docs/", ""},
		{"symlink pointing at a shared path", "ln -s " + shared + "/tools/desk /tmp/desk", ""},
		// (8) Running a tool that LIVES in the shared checkout is a read.
		{"run desktoken from the shared checkout", shared + "/tools/desk/desktoken worker", ""},
		{"go run a shared-checkout tool with output to own worktree", "go run " + shared + "/tools/desk/cmd/deskroster > " + worktree + "/roster.txt", ""},

		// (10) Git subcommands that are not the mutating subcommand the
		// alternation names (#1259 review). `\b` fires between `merge` and the
		// `-` of `merge-base`, so a pure read was refused — with the untrue
		// reason "mutating git subcommand targeting the shared checkout".
		{"git merge-base --is-ancestor", "git merge-base --is-ancestor HEAD origin/main", ""},
		{"git merge-base two commits", "git merge-base HEAD origin/main", ""},
		{"git merge-tree", "git merge-tree HEAD origin/main", ""},
		{"git checkout-index --help style prefix", "git checkout-index -a -n", ""},
		// (11) Read-only MODES of subcommands that otherwise mutate.
		{"git stash list", "git stash list", ""},
		{"git stash show", "git stash show -p", ""},
		{"git clean dry run, short cluster", "git clean -nd", ""},
		{"git clean dry run, long flag", "git clean --dry-run -d", ""},
		{"git apply --check", "git apply --check /tmp/p.diff", ""},
		{"git checkout --help", "git checkout --help", ""},
		{"git commit -h", "git commit -h", ""},
		// (12) A newline is a shell segment boundary. It bounded segmentBefore
		// but not segmentAfter, so a mutation verb on line 1 swallowed every
		// bare word on every following line as one of its targets, and a bare
		// word resolved against a shared cwd landed inside the shared checkout
		// (`SIM-OK` became <shared>/SIM-OK). The `;` form was always allowed.
		{"rm on line 1, echo on line 2", "rm -rf /private/tmp/zz\necho SIM-OK", ""},
		{"rm on line 1, two more lines", "rm -rf /private/tmp/zz\necho SIM-OK\necho DONE", ""},
		{"mkdir then a bare word on the next line", "mkdir -p /tmp/zz\nls", ""},
		{"tee to tmp then a bare word on the next line", "echo hi | tee /tmp/out\necho SIM-OK", ""},
		// (13) Shapes captured live from a coordinator session (#1259 field
		// validation): the tool name inside quoted prose, and a bare grep
		// pattern that ends a pipeline.
		{"tool name inside a quoted URL", "gh api 'repos/x/y/contents/tools/statusgen/registers.go'", ""},
		{"tool name inside a quoted title", "gh issue create --title 'statusgen drops a register' --body-file /tmp/b.md", ""},
		{"tool name as a bare grep pattern ending a pipeline", "gh pr list --json title | grep statusgen", ""},
		{"multi-line quoted body with mkdir and > prose", "gh issue create --title x --body 'repro:\n  mkdir -p /tmp/x\n  echo y > /tmp/z\n'", ""},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			cfg := resetCwd()
			if tc.cwd != "" {
				cfg.Cwd = tc.cwd
			}
			if v := cfg.CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q (cwd %s): unexpected block: %s", tc.cmd, cfg.Cwd, v.Reason)
			}
		})
	}
}

// TestIssue1190TruePositivesPreserved is the other half of every #1190 fix:
// the genuine write that each false-positive shape resembles must still
// block. Narrowing the guard must never make a real shared-checkout write
// pass — the isolation backstop is unchanged.
func TestIssue1190TruePositivesPreserved(t *testing.T) {
	resetCwd := func() Config {
		cfg := workerCfg()
		cfg.Cwd = shared
		return cfg
	}

	blocked := []struct {
		name, cmd string
		cwd       string
	}{
		// (1) A real statusgen invocation writing the shared board.
		{"statusgen at shared root", "go run ./tools/statusgen --root " + shared, ""},
		{"bare statusgen with cwd in shared", "statusgen", shared},
		// A wrapper still leaves the tool in command position.
		{"nohup statusgen with cwd in shared", "nohup statusgen &", shared},
		{"env assignment then statusgen, cwd in shared", "GOWORK=off statusgen", shared},
		{"sudo bazel build with cwd in shared", "sudo bazel build", shared},
		{"time go run statusgen, cwd in shared", "time go run ./tools/statusgen", shared},
		{"statusgen after cd out, still rooted at shared", "cd " + worktree + " && go run ./tools/statusgen --root " + shared, ""},
		{"statusgen --lint then a real run at shared", "go run ./tools/statusgen --root " + worktree + " --lint && go run ./tools/statusgen --root " + shared, ""},
		{"rm of a statusgen source file in shared", "rm " + shared + "/tools/statusgen/registers.go", ""},
		// (2) A real redirect at the end of a pipeline whose program quotes ">".
		{"awk pipeline redirected into shared", "ls -la | awk 'NF>1 {print $1}' > " + shared + "/out.txt", ""},
		{"awk pipeline appended into shared", "ls | awk 'NF>1' >> " + shared + "/STATUS.md", ""},
		// (3) A gh read whose OUTPUT is redirected into the shared checkout.
		{"gh issue view redirected into shared", "gh issue view 1 > " + shared + "/notes.md", ""},
		{"gh body arrow plus a real shared redirect", "gh issue create --body 'a -> b' && echo done > " + shared + "/log.txt", ""},
		// (4) The cd frame only ever moves OUT of shared: absolute shared
		// targets, a cd back into shared, and a bare relative write with the
		// payload cwd inside shared all still block.
		{"cd worktree then rm an absolute shared path", "cd " + worktree + " && rm -rf " + shared + "/frontend/dist", ""},
		{"cd worktree then cd shared", "cd " + worktree + " && cd " + shared + " && git add -A", ""},
		{"relative git add with cwd in shared, no cd", "git add -A && git commit -m x", ""},
		{"relative touch with cwd in shared, no cd", "touch STATUS.md", ""},
		{"cd out then back with a relative write", "cd " + worktree + " && cd " + shared + " && echo hi > STATUS.md", ""},
		// A cd quoted inside prose is not a cd: it must not move the frame a
		// following relative write is judged in.
		{"quoted cd in an issue body then a relative write", "gh issue create --body 'cd /tmp' && echo x > STATUS.md", ""},
		{"echoed cd then a relative rm", "echo 'cd /tmp' && rm dist/output.bin", ""},
		{"heredoc body containing a cd then a relative write", "cat <<'EOF' > /tmp/notes\ncd /tmp\nEOF\ntouch STATUS.md", ""},
		// (5) Heredocs and quoted spans that ARE commands stay visible.
		{"heredoc script fed to bash", "bash <<'EOF'\nrm -rf " + shared + "/dist\nEOF", ""},
		{"sh -c quoted rm of a shared path", "sh -c 'rm " + shared + "/STATUS.md'", ""},
		{"bash -c quoted redirect into shared", `bash -c "echo pwn > ` + shared + `/STATUS.md"`, ""},
		{"sh -lc quoted rm of a shared path", "sh -lc 'rm " + shared + "/STATUS.md'", ""},
		// (6) Out-of-repo write plus an in-repo one in the same command.
		{"claim file write plus a shared write", "printf 'x -> y\\n' > ~/.claude/x.claim && echo pwn > " + shared + "/STATUS.md", ""},
		// (7) An unresolvable cd destination does NOT unlock the writes that
		// follow: they are still judged against the payload cwd frame.
		{"cd variable then relative write, cwd in shared", "cd $S && echo hi > STATUS.md", ""},
		{"cd variable then absolute shared write", "cd $S && rm -rf " + shared + "/docs", ""},
		{"cd variable then git add, cwd in shared", "cd ${SCRATCH} && git add -A", ""},
		// A literal shared prefix is evidence even with an expansion after it.
		{"redirect into shared with a variable subpath", "echo hi > " + shared + "/$SUB/out.txt", ""},
		{"rm a shared path with a variable leaf", "rm -rf " + shared + "/${DIR}", ""},
		// (9) The copy DESTINATION still blocks, including -t form; and mv
		// mutates its source, so moving a file OUT of shared still blocks.
		{"cp into shared", "cp /tmp/x.go " + shared + "/tools/x.go", ""},
		{"cp -t shared", "cp -t " + shared + "/tools /tmp/x.go", ""},
		{"rsync into shared", "rsync -a /tmp/docs/ " + shared + "/docs/", ""},
		{"mv a shared file out to tmp", "mv " + shared + "/README.md /tmp/readme.md", ""},
		{"ln -s into shared", "ln -s /tmp/x " + shared + "/tools/x", ""},
		// (8) Writing INTO the shared checkout's tooling, not running it.
		{"cp over a shared-checkout tool", "cp ./desktoken " + shared + "/tools/desk/desktoken", ""},

		// (10) The genuine writes each newly-allowed git read resembles.
		{"git clean -fd with cwd in shared", "git clean -fd", ""},
		{"git clean -fdx with cwd in shared", "git clean -fdx", ""},
		{"git stash with cwd in shared", "git stash", ""},
		{"git stash push with cwd in shared", "git stash push -m x", ""},
		{"git stash pop with cwd in shared", "git stash pop", ""},
		{"git apply without --check", "git apply /tmp/p.diff", ""},
		{"git checkout -b with cwd in shared", "git checkout -b feature/x", ""},
		{"git merge with cwd in shared", "git merge origin/main", ""},
		{"git commit -m with cwd in shared", "git commit -m x", ""},
		{"git reset --hard with cwd in shared", "git reset --hard HEAD", ""},
		{"git restore with cwd in shared", "git restore .", ""},
		// (12) The newline is a segment boundary, not a licence: a mutation on
		// a LATER line is still inspected, and a backslash-continued line is
		// one command whose target is on the next line.
		{"safe rm on line 1, shared rm on line 2", "rm -rf /tmp/zz\nrm -rf " + shared + "/dist", ""},
		{"echo on line 1, relative touch on line 2, cwd in shared", "echo hi\ntouch STATUS.md", ""},
		{"line-continued rm of a shared path", "rm -rf \\\n  " + shared + "/dist", ""},
		{"line-continued rm, cwd in shared, relative target", "rm -rf \\\n  dist", ""},
		// (14) #1259 review blocking 1 — a target whose head expansion the
		// COMMAND ITSELF binds is not unknown. Each of these was blocked
		// before this PR and admitted at its head.
		{"assigned var redirect into shared", "D=" + shared + "; echo pwn > $D/STATUS.md", ""},
		{"assigned var braced redirect into shared", "D=" + shared + "; echo pwn > ${D}/STATUS.md", ""},
		{"assigned var rm in shared", "D=" + shared + "; rm -rf $D/dist", ""},
		{"assigned var git -C commit in shared", "D=" + shared + "; git -C $D commit -am pwn", ""},
		{"assigned var git -C add in shared", "D=" + shared + "; git -C $D add -A", ""},
		{"assigned var tee into shared", "D=" + shared + "; echo pwn | tee $D/STATUS.md", ""},
		{"assigned var sed -i in shared", "D=" + shared + "; sed -i '' s/a/b/ $D/README.md", ""},
		{"assigned var quoted value", `D="` + shared + `"; echo pwn > $D/STATUS.md`, ""},
		{"assigned var used by a cd, then a relative write", "D=" + shared + "; cd $D; git add -A", ""},
		{"pwd substitution redirect", "echo pwn > `pwd`/STATUS.md", ""},
		{"pwd command substitution redirect", "echo pwn > $(pwd)/STATUS.md", ""},
		{"PWD variable redirect", "echo pwn > $PWD/STATUS.md", ""},
		// An unresolvable head is judged against the frame, as before #1190:
		// from a frame inside the shared checkout it blocks.
		{"unresolvable head with the frame in shared", `go doc ./tools > "$S"/doc.txt`, ""},
		{"unresolvable head copy with the frame in shared", "cp notes.md $SCRATCH/notes.md", ""},
		{"unresolvable head rm with the frame in shared", "rm -rf $SCRATCH/dist", ""},

		// (15) #1259 red team — the effective-cwd frame may only advance
		// where shell semantics guarantee the shell's cwd really moved. Each
		// row below was observed destroying tracked or untracked files in a
		// fixture when admitted.
		{"cd to a stale worktree with ; then git clean", "cd /path/does-not-exist; git clean -fdx", ""},
		{"cd inside a subshell then git clean", "(cd " + worktree + ") ; git clean -fdx", ""},
		// The `&&` twin of the row above (#1259 re-review, B3). Only the `;`
		// form was pinned, so `depth[at] == 0` — the test that stops a SUBSHELL
		// cd from advancing the frame — could be bypassed with the suite green.
		// In a real shell the subshell cd does not move the parent, so the
		// relative write that follows lands in the shared checkout.
		{"cd inside a subshell with && then a relative rm", "(cd " + worktree + ") && rm -rf docs", ""},
		{"cd inside a subshell with && then git clean", "(cd " + worktree + ") && git clean -fdx", ""},
		{"cd inside a brace group with && then a relative write", "{ cd " + worktree + "; } && echo pwn > STATUS.md", ""},
		{"cd inside sh -c then git restore/clean", "sh -c 'cd " + worktree + "' ; git restore . ; git clean -fdx", ""},
		{"pushd/popd then git checkout", "pushd " + worktree + " >/dev/null; popd >/dev/null; git checkout -- . ; git clean -fdx", ""},
		{"cd then cd $OLDPWD then git reset", "cd " + worktree + " && cd $OLDPWD && git reset --hard HEAD && git clean -fdx", ""},
		{"cd inside an untaken branch then a relative write", "if false; then cd " + worktree + "; fi; echo pwn > STATUS.md", ""},
		{"cd then cd a default-expansion then rm", "cd " + worktree + " && cd ${X:-" + shared + "} && rm -rf docs", ""},
		{"assignment, cd out, cd back by variable, rm", "S=" + shared + "; cd " + worktree + "; cd $S; rm -rf docs", ""},
		{"cd out with ; then a relative write", "cd " + worktree + "; echo pwn > STATUS.md", ""},
		{"cd out on its own line then a relative write", "cd " + worktree + "\necho pwn > STATUS.md", ""},
		// (16) #1259 red team — literal masking must not hide a command.
		{"eval single-quoted git clean", "eval 'git clean -fdx'", ""},
		{"eval double-quoted redirect into shared", `eval "echo pwn > ` + shared + `/STATUS.md"`, ""},
		{"heredoc word mentioned in a comment, rm below", "# see <<EOF for the format\nrm -rf " + shared + "/docs", ""},
		{"heredoc word mentioned in a quoted string, rm below", "echo \"uses <<EOF here\"\nrm -rf " + shared + "/docs", ""},
		{"quoted command name rm", `"rm" -rf docs/streams`, ""},
		{"quoted command name tee", `echo x | "tee" STATUS.md`, ""},
		// (17) #1259 red team — a wrapped or quoted invocation is still an
		// invocation of the board generator.
		{"sh -c statusgen", "sh -c 'statusgen'", ""},
		{"bash -lc statusgen --root .", "bash -lc 'statusgen --root .'", ""},
		{"bash -c bazel build", "bash -c 'bazel build'", ""},
		{"timeout statusgen", "timeout 60 statusgen", ""},
		{"caffeinate statusgen", "caffeinate statusgen", ""},
		{"arch statusgen", "arch -arm64 statusgen", ""},
		{"xargs statusgen", "xargs statusgen", ""},
		{"quoted statusgen", `"statusgen"`, ""},
		// (18) #1259 red team — a read-only flag on a LATER line must not
		// disarm an earlier real invocation (the same newline asymmetry, from
		// the other side).
		{"statusgen then statusgen --lint on the next line", "statusgen\nstatusgen --lint", ""},
		{"statusgen then a --lint comment", "statusgen --root .\n# next time use --lint", ""},
		{"statusgen then an echoed --check", "statusgen\necho --check", ""},
		// (19) #1259 red team — a copy-like flag that writes its SOURCE.
		{"rsync --remove-source-files out of shared", "rsync -a --remove-source-files " + shared + "/docs/ /tmp/dest/", ""},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			cfg := resetCwd()
			if tc.cwd != "" {
				cfg.Cwd = tc.cwd
			}
			if v := cfg.CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q (cwd %s): expected block", tc.cmd, cfg.Cwd)
			}
		})
	}
}

// TestIssue1190SentinelClaim covers #1190 evidence 1: WRITEGUARD_SHARED_OK is
// read from the HOOK process env, and the Bash tool starts a fresh shell per
// call, so a session can never set it — the sanctioned exemption was
// unreachable through the normal tool surface. The claim now also lives in a
// sentinel file, which survives the per-call shell but expires and can be
// scoped to specific checkouts so it cannot silently become a permanent
// machine-wide exemption (the #1035 hole).
func TestIssue1190SentinelClaim(t *testing.T) {
	t.Run("env token still claims", func(t *testing.T) {
		t.Setenv("WRITEGUARD_SHARED_OK", "1")
		t.Setenv("WRITEGUARD_SHARED_OK_FILE", filepath.Join(t.TempDir(), "absent"))
		if !sharedOKClaimed(shared) {
			t.Fatal("env token must still claim the exemption")
		}
	})

	t.Run("no token and no sentinel does not claim", func(t *testing.T) {
		t.Setenv("WRITEGUARD_SHARED_OK", "")
		t.Setenv("WRITEGUARD_SHARED_OK_FILE", filepath.Join(t.TempDir(), "absent"))
		if sharedOKClaimed(shared) {
			t.Fatal("no claim must mean no exemption")
		}
	})

	write := func(t *testing.T, body string, age time.Duration) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "writeguard-shared-ok")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if age > 0 {
			mt := time.Now().Add(-age)
			if err := os.Chtimes(p, mt, mt); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}

	cases := []struct {
		name  string
		body  string
		age   time.Duration
		claim bool
	}{
		{"empty sentinel claims any checkout", "", 0, true},
		{"comment-only sentinel claims any checkout", "# claimed by the coordinator\n", 0, true},
		{"sentinel scoped to this checkout claims", shared + "\n", 0, true},
		{"sentinel scoped elsewhere does not claim", "/Users/test/work/example/other-repo\n", 0, false},
		{"sentinel listing several checkouts claims", "/Users/test/other\n" + shared + "\n", 0, true},
		{"expired sentinel does not claim", "", 24 * time.Hour, false},
		{"expired scoped sentinel does not claim", shared + "\n", 24 * time.Hour, false},
		{"sentinel just inside the TTL claims", "", time.Hour, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WRITEGUARD_SHARED_OK", "")
			t.Setenv("WRITEGUARD_SHARED_OK_FILE", write(t, tc.body, tc.age))
			if got := sharedOKClaimed(shared); got != tc.claim {
				t.Fatalf("sharedOKClaimed = %v, want %v (body %q, age %s)", got, tc.claim, tc.body, tc.age)
			}
		})
	}

	// The TTL is configurable, and a claim outside it is ignored.
	t.Run("TTL is configurable", func(t *testing.T) {
		t.Setenv("WRITEGUARD_SHARED_OK", "")
		t.Setenv("WRITEGUARD_SHARED_OK_FILE", write(t, "", 2*time.Hour))
		t.Setenv("WRITEGUARD_SHARED_OK_TTL", "1h")
		if sharedOKClaimed(shared) {
			t.Fatal("a sentinel older than the configured TTL must not claim")
		}
		t.Setenv("WRITEGUARD_SHARED_OK_TTL", "72h")
		if !sharedOKClaimed(shared) {
			t.Fatal("a sentinel inside the configured TTL must claim")
		}
	})

	// A claim NEVER exempts a session that is not shared-homed: the file is
	// as claimable by a dispatched worker as the env var was, so the
	// load-bearing half of the isolation backstop must not depend on it.
	t.Run("claim never rescues a worktree-homed session", func(t *testing.T) {
		cfg := workerCfg()
		cfg.SharedOK = true
		if cfg.Exempt() {
			t.Fatal("worktree-homed session with a claim must not be exempt")
		}
		if v := cfg.CheckFilePath("Write", shared+"/STATUS.md"); !v.Block {
			t.Fatal("worktree-homed session with a claim must still block shared writes")
		}
	})

	// The block message must teach the reachable mechanism, not only the
	// unreachable env var — that message is the discovery path (#1190) — and
	// it must name it as a HUMAN act, since a session can no longer perform it
	// (#1259 review, blocking 2).
	t.Run("block message names the sentinel file as a human act", func(t *testing.T) {
		cfg := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}
		v := cfg.CheckFilePath("Write", shared+"/STATUS.md")
		if !v.Block {
			t.Fatal("expected block")
		}
		for _, want := range []string{
			"writeguard-shared-ok", "WRITEGUARD_SHARED_OK=1", "git worktree add",
			"human terminal", "only a HUMAN can claim it",
		} {
			if !strings.Contains(v.Reason, want) {
				t.Fatalf("block reason must contain %q, got:\n%s", want, v.Reason)
			}
		}
	})
}

// TestIssue1259ClaimSentinelIsHumanOnly covers the PR #1259 review's second
// blocking finding: the sentinel file made the sanctioned exemption reachable,
// but reachable BY THE POPULATION IT MUST EXCLUDE. Observed at that head, every
// persona, every shape: `mkdir -p ~/.config/assay && touch
// ~/.config/assay/writeguard-shared-ok` ADMITTED, the bare touch ADMITTED, the
// $HOME form ADMITTED, and a Write tool call straight at the path ADMITTED —
// one Bash call from any dispatched worker and every shared-homed session on
// the machine was exempt for 12h, with the block message supplying the recipe.
//
// The guard now refuses every tool-surface write to that path, from every
// session, BEFORE the exemption gate. A human at a terminal is outside the
// hook and unaffected, so the claim stays reachable exactly where it was meant
// to be made and nowhere else.
func TestIssue1259ClaimSentinelIsHumanOnly(t *testing.T) {
	const sentinel = "/Users/test/.config/assay/writeguard-shared-ok"
	withSentinel := func(cfg Config) Config {
		cfg.SentinelPath = sentinel
		return cfg
	}
	personas := map[string]Config{
		// The #1035 population: shared-homed by construction, no claim.
		"shared-homed, unclaimed": withSentinel(Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}),
		// A shared-homed session that ALREADY holds a claim must not be able
		// to renew it either — the check runs before Exempt.
		"shared-homed, claimed": withSentinel(Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared, SharedOK: true}),
		// The isolation-backstop populations.
		"worktree-homed, cwd reset to shared": withSentinel(Config{SharedRoot: shared, ProjectDir: worktree, Cwd: shared}),
		"worktree-homed, cwd its worktree":    withSentinel(Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree}),
		"harness worktree subagent":           withSentinel(Config{SharedRoot: shared, ProjectDir: shared, Cwd: nativeWT}),
	}
	blockedBash := []string{
		"mkdir -p /Users/test/.config/assay && touch " + sentinel,
		"touch " + sentinel,
		"printf '%s\\n' " + shared + " > " + sentinel,
		"echo " + shared + " >> " + sentinel,
		"cp /tmp/claim " + sentinel,
		"mv /tmp/claim " + sentinel,
		"tee " + sentinel + " < /tmp/claim",
		"rm " + sentinel,
		"C=" + sentinel + "; touch $C",
		"sh -c 'touch " + sentinel + "'",
	}
	for name, cfg := range personas {
		for _, cmd := range blockedBash {
			t.Run("block/"+name+"/"+cmd[:minLen(cmd, 34)], func(t *testing.T) {
				v := cfg.CheckClaimSentinel("Bash", cmd)
				if !v.Block {
					t.Fatalf("command %q from %s must not be able to issue the claim", cmd, name)
				}
				if !strings.Contains(v.Reason, "HUMAN-ONLY") {
					t.Fatalf("block reason must say the claim is human-only, got:\n%s", v.Reason)
				}
			})
		}
		for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
			t.Run("block/"+name+"/"+tool, func(t *testing.T) {
				if v := cfg.CheckClaimSentinel(tool, sentinel); !v.Block {
					t.Fatalf("%s at the sentinel path from %s must be refused", tool, name)
				}
			})
		}
	}

	// The gate is exactly as narrow as the claim: it must not block the
	// neighbouring operations, nor anything else on the machine.
	cfg := personas["worktree-homed, cwd its worktree"]
	allowed := []struct{ tool, target string }{
		{"Bash", "mkdir -p /Users/test/.config/assay"},
		{"Bash", "cat " + sentinel},
		{"Bash", "ls -la /Users/test/.config/assay"},
		{"Bash", "stat " + sentinel},
		{"Bash", "touch /Users/test/.config/assay/other-file"},
		{"Bash", "touch " + worktree + "/notes.md"},
		{"Write", worktree + "/notes.md"},
		{"Write", "/Users/test/.config/assay/other-file"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.tool+"/"+tc.target[:minLen(tc.target, 40)], func(t *testing.T) {
			if v := cfg.CheckClaimSentinel(tc.tool, tc.target); v.Block {
				t.Fatalf("%s %q must pass the sentinel gate: %s", tc.tool, tc.target, v.Reason)
			}
		})
	}

	// With no sentinel path configured the gate is inert (main.go passes ""
	// when the home directory cannot be resolved).
	t.Run("inert without a configured sentinel path", func(t *testing.T) {
		bare := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree}
		if v := bare.CheckClaimSentinel("Bash", "touch "+sentinel); v.Block {
			t.Fatal("the sentinel gate must be inert when no sentinel path is configured")
		}
	})
}

// TestIssue1259CdScanMasksLiterals pins #1259 re-review B1: cdIntoDir was the
// one rule still reading the RAW command while every other rule in this PR had
// moved to masked text. A relative `cd` quoted inside prose or a heredoc body
// was therefore read as a real cd, resolved against the shared cwd, and blocked
// commands that write nothing — quoting a shell recipe in an issue or PR body
// is a daily desk shape, and the reviewer hit the first row live, twice.
//
// The pairing is the point: masking must be maskLiterals (which keeps `sh -c` /
// `eval` arguments and shell-fed heredoc bodies VISIBLE), never maskQuoted, so
// every genuine cd into the shared checkout still blocks.
func TestIssue1259CdScanMasksLiterals(t *testing.T) {
	// A worker that owns `worktree` but whose Bash payload cwd is `shared` —
	// the population for whom a relative cd resolves INTO the shared checkout.
	resetCwd := func() Config {
		cfg := workerCfg()
		cfg.Cwd = shared
		return cfg
	}

	allowed := []struct{ name, cmd string }{
		{"grep pattern containing cd and pushd", `grep -n "cd /\|(cd \|pushd" f.go`},
		{"grep pattern containing a relative cd", `grep -n 'cd ..' f.go`},
		{"issue body quoting a cd recipe", `gh issue create --title t --body "run: cd docs && ls"`},
		{"pr comment heredoc quoting a cd recipe", "gh pr comment 1 --body-file - <<EOF\nrepro:\n  cd docs && ls\nEOF"},
		{"echoed cd piped to pbcopy", `echo "cd docs" | pbcopy`},
		{"issue body quoting an absolute cd into shared", `gh issue create --title t --body "run: cd ` + shared + ` && ls"`},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := resetCwd().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q writes nothing and must pass: %s", tc.cmd, v.Reason)
			}
		})
	}

	// The genuine cd each allowed shape resembles. maskLiterals keeps shell
	// arguments and shell-fed heredocs visible, so these are untouched.
	blocked := []struct{ name, cmd string }{
		{"cd into shared then a relative rm", "cd " + shared + " && rm -rf docs"},
		{"bare relative cd with cwd in shared", "cd docs"},
		{"sh -c cd into shared", "sh -c 'cd " + shared + " && rm -rf docs'"},
		{"sh -c relative cd with cwd in shared", "sh -c 'cd docs && rm -rf x'"},
		{"heredoc script fed to bash containing a cd", "bash <<'EOF'\ncd " + shared + "\nEOF"},
		{"env -C into shared", "env -C " + shared + " rm -rf docs"},
		{"assigned var then cd by variable", "D=" + shared + "; cd $D"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := resetCwd().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q cds into the shared checkout and must block", tc.cmd)
			}
		})
	}

	// From the worker's OWN cwd nothing else in the command lands in the shared
	// checkout, so cdIntoDir is the only rule that can block — which is what
	// makes these rows the ones that fail if the masking is tightened from
	// maskLiterals to maskQuoted (shell arguments and shell-fed heredoc bodies
	// hidden). They are the reason the fix is maskLiterals and not maskQuoted.
	ownCwd := []struct{ name, cmd string }{
		{"sh -c cd into shared then ls", "sh -c 'cd " + shared + " && ls'"},
		{"bash -c cd into a shared subdir", `bash -c "cd ` + shared + `/docs && ls"`},
		{"heredoc fed to sh containing a cd into shared", "sh <<'EOF'\ncd " + shared + "\nls\nEOF"},
	}
	for _, tc := range ownCwd {
		t.Run("block-own-cwd/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q cds a child shell into the shared checkout and must block", tc.cmd)
			}
		})
	}
}

// TestIssue1259QuotedSubstitutionIsExecutedCode pins the carve-out that keeps a
// `$( … )` span visible inside double quotes.
//
// The masking exists to stop the guard reading PROSE as a command. A command
// substitution inside double quotes is not prose: the shell runs it, before the
// surrounding command, with its writes fully effective. Once the cd scan moved
// to maskLiterals, nothing else covered this class — `echo "$(cd <shared> &&
// rm -rf docs)"` genuinely deletes the directory and was admitted.
//
// Both directions matter, so both are pinned here: revealing the substitution
// must not re-block any of the prose shapes the same delta fixed.
func TestIssue1259QuotedSubstitutionIsExecutedCode(t *testing.T) {
	// Persona: an ordinary dispatched worker sitting in its OWN worktree — the
	// population the isolation backstop constrains. Nothing exotic is needed to reach this.
	blocked := []struct{ name, cmd string }{
		{"substituted cd into shared then a relative rm", `echo "$(cd ` + shared + ` && rm -rf docs)"`},
		{"substituted cd into shared then git clean", `echo "$(cd ` + shared + ` && git clean -fdx)"`},
		{"substituted absolute rm into shared", `echo "$(rm -rf ` + shared + `/docs)"`},
		{"substituted git -C reset on shared", `echo "$(git -C ` + shared + ` reset --hard HEAD)"`},
		{"substituted redirect over a shared file", `echo "$(echo pwn > ` + shared + `/STATUS.md)"`},
		{"substitution as a printf argument", `printf '%s' "$(rm -rf ` + shared + `/docs)"`},
		// The substitution's own quoting must not end the span early: a `)`
		// inside it is a literal, and counting it would blank the tail — which
		// is exactly where a payload would be hidden.
		{"quoted close-paren inside the substitution", `echo "$(echo ')' ; rm -rf ` + shared + `/docs)"`},
		{"quoted open-paren inside the substitution", `echo "$(echo \"(\" ; rm -rf ` + shared + `/docs)"`},
		{"nested substitution", `echo "$(echo $(rm -rf ` + shared + `/docs))"`},
		// The payload sits AFTER an inner substitution closes, so only correct
		// paren NESTING keeps the span open long enough to see it. Without the
		// depth counter the inner `)` ends the span and the tail is re-masked,
		// hiding the write — the mutation harness found this row missing.
		{"payload after a nested substitution closes", `echo "$(echo $(date) ; rm -rf ` + shared + `/docs)"`},
		{"git clean after a nested substitution closes", `echo "$(echo $(date) && git -C ` + shared + ` clean -fdx)"`},
		{"second substitution after a harmless first", `echo "$(date)" "$(rm -rf ` + shared + `/docs)"`},
		{"unbalanced substitution stays visible", `echo "$(rm -rf ` + shared + `/docs"`},
		// Neighbours that were already correct; they anchor the shape so a
		// future masking change cannot quietly take them with it.
		{"unquoted substitution", `echo $(cd ` + shared + ` && rm -rf docs)`},
		{"substitution in an assignment", `X="$(cd ` + shared + ` && rm -rf docs)"`},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q executes a write into the shared checkout and must block", tc.cmd)
			}
		})
	}

	// The paired true-negatives. Revealing `$( … )` must stay NARROW: it may
	// not re-block the prose shapes this PR fixed, and it may not turn every
	// substitution into a block regardless of where it writes.
	allowed := []struct{ name, cmd string }{
		{"harmless substitution", `echo "$(date)"`},
		{"substituted write outside the checkout", `echo "$(cd /private/tmp/scratch && rm -rf junk)"`},
		{"substituted READ of the shared checkout", `echo "$(git -C ` + shared + ` status --short)"`},
		{"substituted read in an issue body", `gh issue create --title t --body "output: $(cat notes.txt)"`},
		// \$ inside double quotes is a literal dollar — prose, not a
		// substitution — so the escape must be honoured.
		{"escaped dollar is prose", `echo "\$(cd ` + shared + ` && rm -rf docs)"`},
		// The B1 rows: single quotes perform no substitution, and a
		// double-quoted body with no `$(` is untouched by this carve-out.
		{"issue body quoting a cd recipe", `gh issue create --title t --body "run: cd docs && ls"`},
		{"grep pattern containing a relative cd", `grep -n 'cd ..' f.go`},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q writes nothing in the shared checkout and must pass: %s", tc.cmd, v.Reason)
			}
		})
	}

	// cdIntoDir reads its targets from the RAW command at offsets found in the
	// masked one, so masking must stay byte-for-byte length-preserving. The
	// carve-out only skips blanking, but that invariant is load-bearing enough
	// to assert rather than assume.
	for _, tc := range append(append([]struct{ name, cmd string }{}, blocked...), allowed...) {
		t.Run("length/"+tc.name, func(t *testing.T) {
			if got, want := len(maskLiterals(tc.cmd)), len(tc.cmd); got != want {
				t.Fatalf("maskLiterals(%q) changed length: got %d, want %d", tc.cmd, got, want)
			}
		})
	}
}

// TestIssue1259UnquotedHeredocBodyIsExpanded is the heredoc half of the same
// rule, and the delta review's blocking finding.
//
// maskHeredocBodies blanks a heredoc body whenever the INTRODUCING COMMAND is
// not a shell. That axis is right for prose and for a quoted `cd`, but it is
// the wrong axis for command substitution: with an UNQUOTED heredoc word
// (<<EOF) the SHELL expands the body before the introducing command ever runs,
// so `cat <<EOF … $(rm -rf <shared>/docs) … EOF` genuinely deletes the
// directory — `cat` never sees it. `gh pr comment --body-file - <<EOF` is a
// daily desk idiom, which is what makes this ordinary rather than exotic.
//
// A QUOTED word (<<'EOF' / <<"EOF") suppresses expansion entirely, so its body
// really is literal text — blanking stays correct there, and the prose shapes
// this PR fixed stay fixed. Both directions are pinned so a future masking
// change cannot silently reopen either one.
func TestIssue1259UnquotedHeredocBodyIsExpanded(t *testing.T) {
	// Persona: an ordinary dispatched worker in its OWN worktree.
	blocked := []struct{ name, cmd string }{
		{"unquoted heredoc substitutes an rm into shared", "cat <<EOF\n$(rm -rf " + shared + "/docs)\nEOF"},
		{"unquoted heredoc substitutes a redirect over a shared file", "cat <<EOF\n$(echo pwn > " + shared + "/STATUS.md)\nEOF"},
		{"unquoted heredoc substitutes git clean on shared", "cat <<EOF\n$(git -C " + shared + " clean -fdx)\nEOF"},
		{"gh comment body-file heredoc, unquoted word", "gh pr comment 1 --body-file - <<EOF\n$(rm -rf " + shared + "/docs)\nEOF"},
		{"tab-stripped <<- heredoc, unquoted word", "cat <<-EOF\n\t$(rm -rf " + shared + "/docs)\n\tEOF"},
		{"substituted cd into shared then a relative rm", "cat <<EOF\n$(cd " + shared + " && rm -rf docs)\nEOF"},
		{"substitution spanning two body lines", "cat <<EOF\n$(cd " + shared + " &&\nrm -rf docs)\nEOF"},
		{"substituted tee onto a shared file", "cat <<EOF\n$(echo x | tee " + shared + "/STATUS.md)\nEOF"},
		// The span walk must survive the same evasions as the double-quote one:
		// a literal `)` inside the substitution's own quotes, a payload sitting
		// after an inner substitution closes, and an unbalanced `$(`.
		{"quoted close-paren inside the substitution", "cat <<EOF\n$(echo ')' ; rm -rf " + shared + "/docs)\nEOF"},
		{"payload after a nested substitution closes", "cat <<EOF\n$(echo $(date) ; rm -rf " + shared + "/docs)\nEOF"},
		{"git clean after a nested substitution closes", "cat <<EOF\n$(echo $(date) && git -C " + shared + " clean -fdx)\nEOF"},
		{"nested substitution", "cat <<EOF\n$(echo $(rm -rf " + shared + "/docs))\nEOF"},
		{"second substitution after a harmless first", "cat <<EOF\nfirst $(date) then $(rm -rf " + shared + "/docs)\nEOF"},
		{"unbalanced substitution stays visible", "cat <<EOF\n$(rm -rf " + shared + "/docs\nEOF"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q executes a write into the shared checkout and must block", tc.cmd)
			}
		})
	}

	// The paired true-negatives. A quoted delimiter performs no expansion, and
	// an unquoted body carrying prose or a read-only substitution is still the
	// prose the masking exists for — none of it may start blocking.
	allowed := []struct{ name, cmd string }{
		{"quoted word, same rm text is prose", "cat <<'EOF'\n$(rm -rf " + shared + "/docs)\nEOF"},
		{"double-quoted word, same rm text is prose", "cat <<\"EOF\"\n$(rm -rf " + shared + "/docs)\nEOF"},
		{"quoted word, nested substitution is prose", "cat <<'EOF'\n$(echo $(rm -rf " + shared + "/docs))\nEOF"},
		{"quoted word, a cd recipe is prose", "cat <<'EOF'\ncd " + shared + " && rm -rf docs\nEOF"},
		{"gh comment body-file heredoc, quoted word", "gh pr comment 1 --body-file - <<'EOF'\nrun: cd docs && ls\nEOF"},
		{"unquoted word carrying plain prose", "cat <<EOF\nnote: then cd " + shared + " and rm -rf docs by hand\nEOF"},
		{"unquoted word, harmless substitution", "cat <<EOF\nreport for $(date)\nEOF"},
		{"unquoted word, read-only substitution on shared", "cat <<EOF\nhead is $(git -C " + shared + " rev-parse HEAD)\nEOF"},
		// \$ is an escape inside an unquoted heredoc too: the body carries a
		// literal `$(`, which the shell does not run.
		{"escaped dollar in an unquoted body is prose", "cat <<EOF\nliteral \\$(rm -rf " + shared + "/docs) in prose\nEOF"},
		// The neighbours that were already correct, anchored here so a future
		// change to this loop cannot take them with it.
		{"heredoc word merely mentioned in a comment", "# see <<EOF for the format\necho ok"},
		{"shell-fed heredoc with a quoted word still blocks nothing harmless", "bash <<'EOF'\ngit -C " + shared + " status --short\nEOF"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q writes nothing in the shared checkout and must pass: %s", tc.cmd, v.Reason)
			}
		})
	}

	// Same length invariant as the double-quote carve-out: cdIntoDir reads
	// targets from the RAW command at offsets found in the masked one.
	for _, tc := range append(append([]struct{ name, cmd string }{}, blocked...), allowed...) {
		t.Run("length/"+tc.name, func(t *testing.T) {
			if got, want := len(maskLiterals(tc.cmd)), len(tc.cmd); got != want {
				t.Fatalf("maskLiterals(%q) changed length: got %d, want %d", tc.cmd, got, want)
			}
		})
	}
}

// TestIssue1259MaskingFailsClosed pins the #1259 security review.
//
// scanWriteIndicators — the write detector itself — runs its regexes over
// maskLiterals(cmd). On main the indicator scan saw the RAW command, so this PR
// is what put a masker in front of it, and every masking misjudgement is now an
// ADMIT. The masker judged quoting on two rules the shell does not use: it
// opened a span on a backslash-ESCAPED quote, and it had no idea what a `#`
// comment was. One odd quote anywhere therefore blanked the rest of the call —
// `# don't do this by hand` followed by `rm -rf <shared>/docs` was admitted,
// and the rm executed.
//
// The rule is not "parse the shell perfectly", it is "never blank text you did
// not understand". Escapes and comments are honoured, and an UNTERMINATED span
// is restored verbatim so the indicators judge it.
func TestIssue1259MaskingFailsClosed(t *testing.T) {
	blocked := []struct{ name, cmd string }{
		{"apostrophe in a comment, rm below", "# don't do this by hand\nrm -rf " + shared + "/docs"},
		{"apostrophe in a comment, redirect below", "# won't work\necho x > " + shared + "/STATUS.md"},
		{"escaped single quote then rm", `echo it\'s ; rm -rf ` + shared + `/docs`},
		{"escaped double quote then rm", `echo \" ; rm -rf ` + shared + `/docs`},
		{"escaped quote after a balanced span", `printf '%s' \' && rm -rf ` + shared + `/docs`},
		{"unterminated double quote then tee", `echo "unterminated ; tee ` + shared + `/STATUS.md`},
		{"unterminated single quote then git clean", `echo 'unterminated ; git -C ` + shared + ` clean -fdx`},
		{"comment on the same line as a later write", "echo ok # don't\nrm -rf " + shared + "/docs"},
		// An escaped quote INSIDE a double-quoted span must not close it early
		// either — that desynchronises every later span the same way.
		{"escaped quote inside a double-quoted span", `echo "a\"b" ; rm -rf ` + shared + `/docs`},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q writes into the shared checkout and must block", tc.cmd)
			}
		})
	}

	// The claim gate shares the scan, so the same hole self-served the
	// human-only exemption. Pin it there too.
	const sentinel = "/Users/test/.config/assay/writeguard-shared-ok"
	cfg := workerCfg()
	cfg.SentinelPath = sentinel
	for _, tc := range []struct{ name, cmd string }{
		{"comment apostrophe then touch", "# don't\ntouch " + sentinel},
		{"escaped quote then touch", `echo it\'s ; touch ` + sentinel},
		{"unterminated quote then touch", `echo "oops ; touch ` + sentinel},
	} {
		t.Run("sentinel/"+tc.name, func(t *testing.T) {
			if v := cfg.CheckClaimSentinel("Bash", tc.cmd); !v.Block {
				t.Fatalf("command %q issues the human-only claim and must block", tc.cmd)
			}
		})
	}

	// Failing closed must not cost the prose wins. A comment executes nothing,
	// so its text is still masked — it just cannot corrupt the quote state.
	allowed := []struct{ name, cmd string }{
		{"comment with an apostrophe and nothing else", "# don't do this by hand\necho ok"},
		{"comment naming the board generator", "# statusgen writes STATUS.md\necho ok"},
		{"hash mid-word is not a comment", "echo a#b"},
		{"hash inside a quoted issue body", `gh issue create --title t --body "see #123 for context"`},
		{"url fragment inside quotes", `curl 'http://x/#frag'`},
		{"grep pattern containing a relative cd", `grep -n 'cd ..' f.go`},
		{"issue body quoting a cd recipe", `gh issue create --title t --body "run: cd docs && ls"`},
		{"awk program is not a redirect", `awk 'NF>1 {print $1}' f.txt`},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q writes nothing in the shared checkout and must pass: %s", tc.cmd, v.Reason)
			}
		})
	}

	for _, tc := range append(append([]struct{ name, cmd string }{}, blocked...), allowed...) {
		t.Run("length/"+tc.name, func(t *testing.T) {
			if got, want := len(maskLiterals(tc.cmd)), len(tc.cmd); got != want {
				t.Fatalf("maskLiterals(%q) changed length: got %d, want %d", tc.cmd, got, want)
			}
		})
	}
}

// TestIssue1259OverlappingHeredocsCannotAlias pins the #1259 security review.
//
// Every heredocRe match computes bodyStart from the first newline after its own
// introducer, so two heredocs on ONE LINE alias the same body offsets. The body
// masking used to write bytes per match, so when the second delimiter was
// QUOTED its blanking pass overwrote the `$( … )` bytes the first (UNQUOTED)
// pass had deliberately preserved: `cat <<EOF <<'Z'` / `$(rm -rf <shared>/docs)`
// / `EOF` executed under both bash and zsh and was admitted.
//
// Blank and keep are now computed for all heredocs before any byte is written,
// and keep wins — so neither delimiter order can un-preserve executed code.
func TestIssue1259OverlappingHeredocsCannotAlias(t *testing.T) {
	blocked := []struct{ name, cmd string }{
		{"unquoted then single-quoted word", "cat <<EOF <<'Z'\n$(rm -rf " + shared + "/docs)\nEOF\nprose\nZ"},
		{"unquoted then double-quoted word", "cat <<EOF <<\"Z\"\n$(rm -rf " + shared + "/docs)\nEOF\nprose\nZ"},
		{"distinct words, second quoted", "cat <<A <<'B'\n$(rm -rf " + shared + "/docs)\nA\nprose\nB"},
		{"second terminator never appears", "cat <<EOF <<'Z'\n$(rm -rf " + shared + "/docs)\nEOF\nprose"},
		// The reverse order is the case the old "never un-blank" invariant got
		// wrong from the other side: the quoted pass ran first and blanked the
		// bytes the unquoted pass then wanted to preserve.
		{"quoted word first, unquoted second", "cat <<'Z' <<EOF\n$(rm -rf " + shared + "/docs)\nZ\nprose\nEOF"},
		{"overlapping bodies carrying a backtick payload", "cat <<EOF <<'Z'\n`git -C " + shared + " clean -fdx`\nEOF\nprose\nZ"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q expands its first body and must block", tc.cmd)
			}
		})
	}

	const sentinel = "/Users/test/.config/assay/writeguard-shared-ok"
	cfg := workerCfg()
	cfg.SentinelPath = sentinel
	t.Run("sentinel/aliased heredoc bodies", func(t *testing.T) {
		cmd := "cat <<EOF <<'Z'\n$(touch " + sentinel + ")\nEOF\np\nZ"
		if v := cfg.CheckClaimSentinel("Bash", cmd); !v.Block {
			t.Fatalf("command %q issues the human-only claim and must block", cmd)
		}
	})

	// Two QUOTED delimiters expand nothing, so both bodies stay prose.
	t.Run("allow/both words quoted", func(t *testing.T) {
		cmd := "cat <<'A' <<'B'\n$(rm -rf " + shared + "/docs)\nA\nprose\nB"
		if v := workerCfg().CheckBash(cmd); v.Block {
			t.Fatalf("command %q expands nothing and must pass: %s", cmd, v.Reason)
		}
	})
}

// TestIssue1259BacktickSubstitutionIsExecutedCode pins the #1259 security
// review and the delta review's blocking finding — the same rule's third
// instance.
//
// An unquoted heredoc word makes the shell expand the body, and expansion means
// ALL expansion: “ ` … ` “ runs exactly like `$( … )`. Trusting `$( … )` to
// be the only executed form left a hole THIS PR OPENED — main has no heredoc
// masking, so main saw these bodies plainly and refused them. Reachability is
// higher than for `$( … )`, not lower: the backtick is the markdown code-span
// character, and `gh pr comment --body-file - <<EOF` is a daily desk idiom.
//
// The same applies inside double quotes, where this PR's other carve-out had
// the same gap.
func TestIssue1259BacktickSubstitutionIsExecutedCode(t *testing.T) {
	blocked := []struct{ name, cmd string }{
		{"backtick git clean in an unquoted body", "cat <<EOF\n`git -C " + shared + " clean -fdx`\nEOF"},
		{"backtick git reset in an unquoted body", "cat <<EOF\n`git -C " + shared + " reset --hard HEAD`\nEOF"},
		{"backtick redirect in an unquoted body", "cat <<EOF\n`echo pwn > " + shared + "/STATUS.md`\nEOF"},
		{"backtick tee in an unquoted body", "cat <<EOF\n`echo x | tee " + shared + "/STATUS.md`\nEOF"},
		{"backtick rm in an unquoted body", "cat <<EOF\n`rm -rf " + shared + "/docs`\nEOF"},
		{"gh comment body-file heredoc, backtick payload", "gh pr comment 1 --body-file - <<EOF\n`git -C " + shared + " reset --hard HEAD`\nEOF"},
		{"backtick inside double quotes", "echo \"`git -C " + shared + " clean -fdx`\""},
		{"backtick inside double quotes, rm payload", "echo \"`rm -rf " + shared + "/docs`\""},
		// Unterminated backtick leaves the tail visible — fail closed.
		{"unterminated backtick in an unquoted body", "cat <<EOF\n`rm -rf " + shared + "/docs\nEOF"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q executes a write into the shared checkout and must block", tc.cmd)
			}
		})
	}

	const sentinel = "/Users/test/.config/assay/writeguard-shared-ok"
	cfg := workerCfg()
	cfg.SentinelPath = sentinel
	t.Run("sentinel/backtick touch in an unquoted body", func(t *testing.T) {
		cmd := "cat <<EOF\n`touch " + sentinel + "`\nEOF"
		if v := cfg.CheckClaimSentinel("Bash", cmd); !v.Block {
			t.Fatalf("command %q issues the human-only claim and must block", cmd)
		}
	})

	// A quoted delimiter and single quotes both suppress backtick expansion, so
	// the identical text stays prose — the markdown code-span case desk agents
	// write all day.
	allowed := []struct{ name, cmd string }{
		{"quoted word, backtick payload is prose", "cat <<'EOF'\n`git -C " + shared + " clean -fdx`\nEOF"},
		{"markdown code span in a single-quoted body", "gh pr comment 1 --body 'see `rm -rf docs` in the recipe'"},
		{"quoted-word heredoc quoting a recipe", "gh pr comment 1 --body-file - <<'EOF'\nrun `git -C " + shared + " clean -fdx` by hand\nEOF"},
		{"backtick read-only substitution in an unquoted body", "cat <<EOF\nhead is `git -C " + shared + " rev-parse HEAD`\nEOF"},
		{"escaped backtick is prose", "cat <<EOF\nliteral \\`rm -rf " + shared + "/docs\\` here\nEOF"},
		{"backtick pwd outside the checkout", "echo `pwd`"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if v := workerCfg().CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q writes nothing in the shared checkout and must pass: %s", tc.cmd, v.Reason)
			}
		})
	}

	for _, tc := range append(append([]struct{ name, cmd string }{}, blocked...), allowed...) {
		t.Run("length/"+tc.name, func(t *testing.T) {
			if got, want := len(maskLiterals(tc.cmd)), len(tc.cmd); got != want {
				t.Fatalf("maskLiterals(%q) changed length: got %d, want %d", tc.cmd, got, want)
			}
		})
	}
}

// TestIssue1259ExpansionResolution pins the two resolution sources that are not
// visible in the command text: the hook's own environment (which the Bash
// tool's shell inherits) and the shell-dynamic variables that must NEVER be
// read from it.
func TestIssue1259ExpansionResolution(t *testing.T) {
	cfg := workerCfg()
	cfg.Cwd = shared

	t.Run("an exported env var resolves", func(t *testing.T) {
		t.Setenv("MY_SCRATCH", "/tmp/scratch")
		if v := cfg.CheckBash("echo hi > $MY_SCRATCH/out.txt"); v.Block {
			t.Fatalf("an env-resolvable destination outside the checkout must pass: %s", v.Reason)
		}
		t.Setenv("MY_SCRATCH", shared+"/docs")
		if v := cfg.CheckBash("echo hi > $MY_SCRATCH/out.txt"); !v.Block {
			t.Fatal("an env-resolvable destination INSIDE the checkout must block")
		}
	})

	t.Run("shell-dynamic vars are never read from the hook env", func(t *testing.T) {
		t.Setenv("OLDPWD", "/tmp/somewhere-else")
		// The hook's OLDPWD says nothing about the tool shell's. The frame
		// must fail safe to the payload cwd, so the reset still blocks.
		if v := cfg.CheckBash("cd " + worktree + " && cd $OLDPWD && git reset --hard HEAD"); !v.Block {
			t.Fatal("a cd to a shell-dynamic variable must fail safe, not resolve from the hook env")
		}
	})
}

func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}

// gitFixture builds a throwaway repo with a linked worktree, so run() — which
// derives the shared root by shelling out to `git rev-parse --git-common-dir` —
// can be driven end to end with a real hook payload. Global/system git config
// is neutralised so a developer's core.hooksPath or commit template cannot make
// the fixture behave differently from CI.
func gitFixture(t *testing.T) (sharedRoot, linkedWT string) {
	t.Helper()
	base := t.TempDir()
	sharedRoot = filepath.Join(base, "shared")
	linkedWT = filepath.Join(base, "wt")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main", sharedRoot)
	git("-C", sharedRoot, "commit", "-q", "--allow-empty", "-m", "root")
	git("-C", sharedRoot, "worktree", "add", "-q", "-b", "work", linkedWT)
	return sharedRoot, linkedWT
}

// TestIssue1259RunWiresTheClaimGate pins the human-only claim gate AT ITS CALL
// SITE (#1259 re-review, B2). TestIssue1259ClaimSentinelIsHumanOnly proves
// CheckClaimSentinel itself thoroughly, but nothing in the package drove run(),
// so deleting the three lines that invoke it — the headline fix of this delta —
// left the suite green while the guard admitted `touch <sentinel>` again.
//
// Two properties are only observable through run() with a real payload:
//   - the gate is WIRED at all; and
//   - it runs BEFORE Exempt(), so a session that already holds a claim cannot
//     renew its own indefinitely. The exempt sub-case asserts the exemption is
//     genuinely in force first, so it cannot pass for the wrong reason.
func TestIssue1259RunWiresTheClaimGate(t *testing.T) {
	sharedRoot, wt := gitFixture(t)
	sentinelDir := filepath.Join(t.TempDir(), "assay")
	sentinel := filepath.Join(sentinelDir, "writeguard-shared-ok")
	// The sentinel lives OUTSIDE the checkout: nothing but the claim gate can
	// account for a block on it, so a deleted call site cannot be masked by the
	// ordinary write-indicator scan.
	t.Setenv("WRITEGUARD_SHARED_OK_FILE", sentinel)
	t.Setenv("WRITEGUARD_SHARED_OK", "")

	payload := func(cwd, tool string, in map[string]string) *bytes.Reader {
		t.Helper()
		b, err := json.Marshal(map[string]any{"cwd": cwd, "tool_name": tool, "tool_input": in})
		if err != nil {
			t.Fatal(err)
		}
		return bytes.NewReader(b)
	}
	verdict := func(t *testing.T, cwd, tool string, in map[string]string) Verdict {
		t.Helper()
		v, err := run(payload(cwd, tool, in))
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return v
	}
	bash := func(cmd string) map[string]string { return map[string]string{"command": cmd} }

	t.Run("a worker session cannot issue the claim", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", wt)
		v := verdict(t, wt, "Bash", bash("touch "+sentinel))
		if !v.Block {
			t.Fatal("run() must refuse a tool-surface write to the claim sentinel — the gate is not wired")
		}
		if !strings.Contains(v.Reason, "HUMAN-ONLY") {
			t.Fatalf("block must come from the claim gate, got:\n%s", v.Reason)
		}
	})

	t.Run("a Write tool call at the sentinel is refused", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", wt)
		v := verdict(t, wt, "Write", map[string]string{"file_path": sentinel})
		if !v.Block || !strings.Contains(v.Reason, "HUMAN-ONLY") {
			t.Fatalf("run() must refuse a Write at the sentinel, got block=%v:\n%s", v.Block, v.Reason)
		}
	})

	t.Run("an exempt session cannot renew its own claim", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", sharedRoot)
		t.Setenv("WRITEGUARD_SHARED_OK", "1")
		// The exemption must actually be in force, or the assertion below
		// would pass even with the gate ordered after Exempt().
		if v := verdict(t, sharedRoot, "Bash", bash("touch "+filepath.Join(sharedRoot, "STATUS.md"))); v.Block {
			t.Fatalf("fixture invalid: this session must be exempt, got:\n%s", v.Reason)
		}
		v := verdict(t, sharedRoot, "Bash", bash("touch "+sentinel))
		if !v.Block || !strings.Contains(v.Reason, "HUMAN-ONLY") {
			t.Fatalf("the claim gate must run BEFORE Exempt(): an exempt session must not renew its own claim (block=%v)\n%s", v.Block, v.Reason)
		}
	})

	t.Run("the gate is as narrow through run() as it is in isolation", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", wt)
		for _, in := range []struct {
			tool string
			args map[string]string
		}{
			{"Bash", bash("cat " + sentinel)},
			{"Bash", bash("mkdir -p " + sentinelDir)},
			{"Bash", bash("touch " + filepath.Join(sentinelDir, "other-file"))},
			{"Write", map[string]string{"file_path": filepath.Join(sentinelDir, "other-file")}},
			{"Bash", bash("touch " + filepath.Join(wt, "notes.md"))},
		} {
			if v := verdict(t, wt, in.tool, in.args); v.Block {
				t.Fatalf("%s %v must pass: %s", in.tool, in.args, v.Reason)
			}
		}
	})

	// Anchor: the isolation backstop itself is still reachable through run(), so a
	// mutation that guts the call chain wholesale cannot hide behind the rows
	// above (all of which are blocks the sentinel gate owns).
	t.Run("the isolation backstop is still wired", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", wt)
		if v := verdict(t, wt, "Bash", bash("rm -rf "+filepath.Join(sharedRoot, "docs"))); !v.Block {
			t.Fatal("run() must still block a worker's write to the shared checkout")
		}
		if v := verdict(t, wt, "Write", map[string]string{"file_path": filepath.Join(sharedRoot, "STATUS.md")}); !v.Block {
			t.Fatal("run() must still block a worker's Write into the shared checkout")
		}
	})
}

func TestPathTokens(t *testing.T) {
	tokens := pathTokens("echo hi > " + shared + "/STATUS.md")
	found := false
	for _, tok := range tokens {
		if tok == shared+"/STATUS.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected absolute path token, got %v", tokens)
	}

	// Relative paths.
	tokens = pathTokens("echo hi > ../../../STATUS.md")
	found = false
	for _, tok := range tokens {
		if tok == "../../../STATUS.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected relative path token, got %v", tokens)
	}

	// Flags must be skipped.
	tokens = pathTokens("rm -rf " + shared + "/dist")
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "-") {
			t.Fatalf("flag %q must not appear in path tokens: %v", tok, tokens)
		}
	}

	// Embedded paths in key=value options.
	tokens = pathTokens("git --git-dir=" + shared + "/.git commit -m x")
	found = false
	for _, tok := range tokens {
		if tok == shared+"/.git" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected embedded path from --git-dir=, got %v", tokens)
	}
}

// TestEncodedSharedRootSubstringInScratchpadIsAllowed pins the operator's
// 2026-08-16 ruling (C): the isolation backstop must EXEMPT a git mutation whose real
// target is a genuine non-shared-checkout location, even when the command
// string incidentally contains the shared-checkout path as a SUBSTRING.
//
// The recurring real-world shape is a session scratchpad whose directory name
// is the shared checkout's absolute path with the separators re-encoded as
// dashes — e.g. shared root  /Users/x/y/tracker  becomes a component
// /private/tmp/claude-501/-Users-x-y-tracker/<session>/scratchpad. A `git
// clone <url> <that path>/foo`, a `git -C <that path> commit`, or a redirect
// into it targets /private/tmp/..., which is NOT under the shared root — so it
// must be ALLOWED. A crude `strings.Contains(cmd, sharedRoot)` heuristic (or
// one matching the dash-encoded form) would wrongly refuse it, which cost
// multiple worker cycles and pushed operators toward path-trick workarounds.
//
// The guard already decides by canonicalized path CONTAINMENT (underDir), not
// substring, so these all pass today; this test is the regression fence that
// keeps it that way. (The residual "too complex to verify it stays inside the
// worktree / must target its own worktree" refusal that operators still hit
// comes from the Claude Code HARNESS worktree-isolation feature for
// isolation:worktree agents — NOT this hook, which is the only PreToolUse hook
// this repo wires.)
func TestEncodedSharedRootSubstringInScratchpadIsAllowed(t *testing.T) {
	// dashEncoded is the shared root with every separator turned into a dash —
	// the literal substring a naive matcher would key on. Building it from
	// `shared` keeps the two in lockstep if the fixture ever changes.
	dashEncoded := strings.ReplaceAll(strings.TrimPrefix(shared, "/"), "/", "-")
	// A scratchpad path that CONTAINS that substring but lives entirely under
	// /private/tmp — never inside the shared checkout.
	scratch := "/private/tmp/claude-501/-" + dashEncoded + "/170f3fae/scratchpad"
	dest := scratch + "/foo"

	if strings.Contains(scratch, "/"+strings.TrimPrefix(shared, "/")) {
		t.Fatalf("test bug: scratch must not contain the slash-form shared root: %s", scratch)
	}
	if !strings.Contains(scratch, dashEncoded) {
		t.Fatalf("test bug: scratch must contain the dash-encoded shared root substring: %s", scratch)
	}

	// The false-positive is independent of where the session is homed, so pin
	// it for both a worktree-homed worker and a (would-be) shared-homed session
	// whose payload cwd is the shared root — the population most exposed to a
	// substring heuristic.
	homings := map[string]Config{
		"worktree-homed worker":             {SharedRoot: shared, ProjectDir: worktree, Cwd: worktree},
		"shared-homed session (cwd=shared)": {SharedRoot: shared, ProjectDir: shared, Cwd: shared},
	}
	allowed := []struct{ name, cmd string }{
		{"clone into scratchpad", "git clone https://github.com/example/tracker " + dest},
		{"git -C scratchpad commit", "git -C " + dest + " commit -am wip"},
		{"git -C scratchpad add", "git -C " + scratch + " add ."},
		{"redirect into scratchpad", "echo hi > " + dest + "/notes.md"},
		{"mkdir scratchpad", "mkdir -p " + dest},
		{"rm inside scratchpad", "rm -rf " + dest},
	}
	for hn, cfg := range homings {
		for _, tc := range allowed {
			t.Run(hn+"/"+tc.name, func(t *testing.T) {
				if v := cfg.CheckBash(tc.cmd); v.Block {
					t.Fatalf("substring-only match must NOT block a real non-shared target\ncmd:  %s\nreason: %s", tc.cmd, v.Reason)
				}
			})
		}
	}

	// The exemption must NOT weaken the real block: a git mutation whose
	// resolved target genuinely IS the shared checkout (or a subdir) still
	// blocks, from a worktree-homed worker.
	worker := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree}
	blocked := []struct{ name, cmd string }{
		{"git -C shared-root commit", "git -C " + shared + " commit -am x"},
		{"git -C shared subdir add", "git -C " + shared + "/docs add ."},
		{"redirect into shared root", "echo pwn > " + shared + "/STATUS.md"},
		{"clone INTO the shared checkout", "git clone https://github.com/example/tracker " + shared + "/vendor/x && git -C " + shared + "/vendor/x reset --hard"},
	}
	for _, tc := range blocked {
		t.Run("still-blocked/"+tc.name, func(t *testing.T) {
			if v := worker.CheckBash(tc.cmd); !v.Block {
				t.Fatalf("a real shared-checkout mutation must still block: %s", tc.cmd)
			}
		})
	}
}

// TestIssue1193FindDeleteExec closes the substitution gap #1193 names: the
// 2026-08-16 incident's worker answered a false-positive `rm` block by
// substituting `find … -delete`, which matched no indicator at all. find is
// now a target-aware file-mutation indicator that fires ONLY for the mutating
// primaries (-delete, the -exec/-ok family) with a resolved ROOT inside the
// shared checkout. Both directions matter: a bare read-only find must never
// fire, and the absolute out-of-checkout forms — the sanctioned re-issue the
// block message teaches — must be admitted.
func TestIssue1193FindDeleteExec(t *testing.T) {
	// A worker that owns `worktree` but whose Bash payload cwd is `shared`
	// (the #1190 population — cwdInShared puts every command in scope).
	resetCwd := func() Config {
		cfg := workerCfg()
		cfg.Cwd = shared
		return cfg
	}

	allowed := []struct {
		name, cmd string
		cwd       string // "" = payload cwd reset to shared
	}{
		// The incident shape, correctly re-issued with an absolute target.
		{"find -delete on an absolute tmp path", "find /tmp/x -delete", ""},
		{"find -delete with filters on a tmp path", "find /tmp/x -name '*.tmp' -delete", ""},
		{"find -exec rm + form on a tmp path", "find /tmp/x -exec rm {} +", ""},
		{"find -exec rm escaped-semicolon form", `find /tmp/x -name '*.tmp' -exec rm {} \;`, ""},
		{"find -execdir on a tmp path", `find /tmp/x -execdir chmod 600 {} \;`, ""},
		{"find -L pre-path option before a tmp root", "find -L /tmp/x -delete", ""},
		{"find -delete under the worker's own worktree", "find " + worktree + "/dist -delete", ""},
		// Read-only finds never fire, whatever the cwd frame.
		{"read-only find on a relative path from shared cwd", "find docs -name '*.md'", ""},
		{"read-only bare find from shared cwd", "find . -type f", ""},
		{"read-only find piped from shared cwd", "find tools -name '*.go' | wc -l", ""},
		{"read-only find -print0 into xargs grep", "find docs -name '*.md' -print0 | xargs -0 grep -l foo", ""},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			cfg := resetCwd()
			if tc.cwd != "" {
				cfg.Cwd = tc.cwd
			}
			if v := cfg.CheckBash(tc.cmd); v.Block {
				t.Fatalf("command %q (cwd %s): unexpected block: %s", tc.cmd, cfg.Cwd, v.Reason)
			}
		})
	}

	blocked := []struct {
		name, cmd string
		cwd       string
	}{
		// The substitute the incident worker reached for, aimed at the
		// checkout: a relative root from a shared cwd resolves inside it.
		{"relative find -delete from shared cwd", "find docs -delete", ""},
		{"relative find -delete with filters", "find docs -name '*.md' -delete", ""},
		{"absolute find -delete on the shared checkout", "find " + shared + "/docs -delete", ""},
		{"find -exec rm over a shared root", "find " + shared + "/tools -name '*.go' -exec rm {} +", ""},
		{"find -execdir over a relative root from shared cwd", `find docs -execdir rm {} \;`, ""},
		{"rootless find -delete defaults to the shared cwd", "find -delete", ""},
		{"find -exec cp with a literal shared destination", `find /tmp/x -exec cp {} ` + shared + `/tools/x.go \;`, ""},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.name, func(t *testing.T) {
			cfg := resetCwd()
			if tc.cwd != "" {
				cfg.Cwd = tc.cwd
			}
			if v := cfg.CheckBash(tc.cmd); !v.Block {
				t.Fatalf("command %q (cwd %s): expected block", tc.cmd, cfg.Cwd)
			}
		})
	}
}

// TestIssue1193BlockMessageTeachesReissue: every block message must carry the
// sanctioned exit (re-issue the SAME command with absolute targets or one
// `cd <abs-dir> && …` chain) and the explicit no-substitution line, for both
// the worktree-homed and the shared-homed (#1035) message branches.
func TestIssue1193BlockMessageTeachesReissue(t *testing.T) {
	assertGuidance := func(t *testing.T, reason string) {
		t.Helper()
		for _, want := range []string{
			"re-issue the same command with absolute target paths",
			"cd <abs-dir> && ",
			"substitution is an escalation-worthy policy violation",
		} {
			if !strings.Contains(reason, want) {
				t.Fatalf("block reason missing %q:\n%s", want, reason)
			}
		}
	}

	worktreeHomed := workerCfg()
	v := worktreeHomed.CheckBash("rm -rf " + shared + "/docs")
	if !v.Block {
		t.Fatal("expected block for rm of a shared path")
	}
	assertGuidance(t, v.Reason)

	sharedHomed := Config{SharedRoot: shared, ProjectDir: shared, Cwd: shared}
	v = sharedHomed.CheckBash("touch STATUS.md")
	if !v.Block {
		t.Fatal("expected block for relative touch from a shared-homed session")
	}
	assertGuidance(t, v.Reason)
}
