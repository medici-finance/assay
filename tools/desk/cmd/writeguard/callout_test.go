package main

// Tests for the adopter DANGEROUS-COMMAND callout (callout.go).
//
// Every callout here is a FIXTURE written by the test into its own temp dir. None of
// them is anybody's real callout, and none of them carries a real organisation's
// command patterns: what is under test is the guard's half of the contract — that a
// configured callout is consulted, that its "block" is honoured, that every way it can
// fail to answer refuses the write, and that none of it can weaken a compiled
// indicator.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeCallout writes an executable shell fixture and returns its absolute path.
func writeCallout(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	// t.TempDir is 0700, which already satisfies the directory rule; asserting it
	// keeps the fixture honest if that ever changes.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod callout dir: %v", err)
	}
	p := filepath.Join(dir, "callout")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("writing the callout fixture: %v", err)
	}
	return p
}

// calloutCfg is a worker session (NOT shared-homed, so no exemption) with a callout
// configured, and a cwd inside the shared checkout so commands are in scope.
func calloutCfg(callout string) Config {
	return Config{SharedRoot: shared, ProjectDir: shared + "-wt", Cwd: shared, Callout: callout}
}

// synthetic is a command no compiled indicator matches. Every positive-control and
// fail-closed row below uses it, which is what makes those rows about the CALLOUT: if
// a compiled indicator matched it, the block would prove nothing about the hook.
//
// It is deliberately a made-up tool name. The point of the callout is that the shared
// guard does not know any particular organisation's build vocabulary, and a test that
// hard-coded one would reintroduce exactly what the hook removes.
const synthetic = "frobnicate --build"

func TestSyntheticCommandLacksCompiledMatch(t *testing.T) {
	// The premise of the rows below. If this ever fails, those rows have silently
	// become tests of a compiled indicator instead of the callout.
	cfg := Config{SharedRoot: shared, ProjectDir: shared + "-wt", Cwd: shared}
	if v := cfg.CheckBash(synthetic); v.Block {
		t.Fatalf("the synthetic command is matched by a compiled indicator (%q) — "+
			"the callout rows below would prove nothing", v.Reason)
	}
}

// --- Verify row 4: positive control ------------------------

// TestCalloutBlocksWhatItFlags — a fixture callout that flags a synthetic build
// command makes that command BLOCKED, and the refusal carries the callout's own
// reason so an operator can tell adopter policy from a guard rule.
func TestCalloutBlocksWhatItFlags(t *testing.T) {
	// The fixture reads the JSON request on stdin and flags the synthetic verb.
	c := writeCallout(t, `
req=$(cat)
case "$req" in
  *frobnicate*) echo "block fixture policy: frobnicate writes into the tree it runs in" ;;
  *)            echo allow ;;
esac
`)
	cfg := calloutCfg(c)

	v := cfg.CheckBash(synthetic)
	if !v.Block {
		t.Fatal("the callout flagged the command and it was NOT blocked — the hook is not wired in")
	}
	if !strings.Contains(v.Reason, "fixture policy") {
		t.Fatalf("the refusal does not carry the callout's own reason:\n%s", v.Reason)
	}
	if !strings.Contains(v.Reason, "adopter policy") {
		t.Fatalf("the refusal does not distinguish adopter policy from a guard rule:\n%s", v.Reason)
	}

	// The same callout says allow for anything else, and that must not block.
	if v := cfg.CheckBash("echo hello"); v.Block {
		t.Fatalf("an allowed command was blocked: %s", v.Reason)
	}
}

// TestCalloutSkippedOutsideGuardedScope — a command that neither names the shared
// checkout nor runs inside it never reaches the callout at all. The callout here
// blocks EVERYTHING, so a block would mean the guard is asking an adopter about
// commands that have nothing to do with the protected tree.
func TestCalloutSkippedOutsideGuardedScope(t *testing.T) {
	cfg := Config{
		SharedRoot: shared,
		ProjectDir: worktree,
		Cwd:        worktree,
		Callout:    writeCallout(t, "echo block everything"),
	}
	if v := cfg.CheckBash("echo hi > " + worktree + "/out.txt"); v.Block {
		t.Fatalf("an out-of-scope command reached the callout: %s", v.Reason)
	}
}

// --- Verify row 4, second half: the compiled indicators with the hook UNSET ----

// TestCompiledIndicatorsBlockSansCallout — with no callout configured, every
// compiled indicator still blocks its own case. This is the byte-identical-when-unset
// property: adding the hook must not have moved any compiled behaviour.
//
// It is ALSO Verify row 6's instrument. Scratch-removing any one indicatorSpec from
// guard.go turns its row here RED, which is what proves the compiled defaults are
// genuinely exercised rather than shadowed by the callout.
func TestCompiledIndicatorsBlockSansCallout(t *testing.T) {
	cfg := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree} // Callout unset
	cases := map[string]string{
		"output redirection":       "echo pwn > " + shared + "/STATUS.md",
		"tee":                      "echo pwn | tee " + shared + "/STATUS.md",
		"file mutation command":    "rm -rf " + shared + "/docs",
		"sed -i":                   "sed -i 's/a/b/' " + shared + "/STATUS.md",
		"mutating git subcommand":  "git -C " + shared + " commit -am x",
		"statusgen (writes STATUS": "statusgen --root " + shared,
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			v := cfg.CheckBash(cmd)
			if !v.Block {
				t.Fatalf("compiled indicator %q no longer blocks %q — with no callout configured "+
					"the compiled set is the WHOLE guard", name, cmd)
			}
			if !strings.Contains(v.Reason, name) {
				t.Fatalf("blocked, but not by %q:\n%s", name, v.Reason)
			}
		})
	}
}

// TestUnsetCalloutNeverBlocksAlone — the unset hook adds nothing. A command
// no compiled indicator matches passes exactly as it did before the hook existed.
func TestUnsetCalloutNeverBlocksAlone(t *testing.T) {
	cfg := Config{SharedRoot: shared, ProjectDir: shared + "-wt", Cwd: shared}
	for _, cmd := range []string{synthetic, "echo hello", "gh pr list"} {
		if v := cfg.CheckBash(cmd); v.Block {
			t.Fatalf("unset callout blocked %q: %s", cmd, v.Reason)
		}
	}
}

// --- Verify row 5: every failure mode BLOCKS ------------------------

// TestCalloutFailureModesAllBlock — fail-closed for a write GUARD means BLOCK. Each
// row is a distinct way the question can go unanswered; none may pass.
func TestCalloutFailureModesAllBlock(t *testing.T) {
	longRun := writeCallout(t, "sleep 30\necho allow\n")

	missing := filepath.Join(t.TempDir(), "does-not-exist")

	notExec := filepath.Join(t.TempDir(), "not-exec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\necho allow\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	worldWritable := writeCallout(t, "echo allow\n")
	if err := os.Chmod(worldWritable, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	writableDir := t.TempDir()
	if err := os.Chmod(writableDir, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	inWritableDir := filepath.Join(writableDir, "callout")
	if err := os.WriteFile(inWritableDir, []byte("#!/bin/sh\necho allow\n"), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	cases := map[string]string{
		"non-zero exit":        writeCallout(t, "exit 7\n"),
		"non-zero after allow": writeCallout(t, "echo allow\nexit 3\n"),
		"garbage output":       writeCallout(t, "echo '<<< not a verdict >>>'\n"),
		"silence":              writeCallout(t, "exit 0\n"),
		"missing file":         missing,
		"not executable":       notExec,
		"world-writable file":  worldWritable,
		"world-writable dir":   inWritableDir,
		"relative path":        "relative/callout",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := calloutCfg(path)
			v := cfg.CheckBash(synthetic)
			if !v.Block {
				t.Fatalf("failure mode %q did NOT block — a guard that allows on "+
					"'I could not ask' is answering a question nobody put to it", name)
			}
			if !strings.Contains(v.Reason, "did not answer") {
				t.Fatalf("failure mode %q blocked, but not as a fail-closed refusal:\n%s", name, v.Reason)
			}
			if !strings.Contains(v.Reason, "Nothing about this command was judged") {
				t.Fatalf("failure mode %q does not tell the operator the check did not run:\n%s", name, v.Reason)
			}
		})
	}

	t.Run("timeout", func(t *testing.T) {
		// A short deadline: exercising the real 5s default would cost the suite five
		// seconds for no extra coverage. The branch under test is the same one.
		cfg := calloutCfg(longRun)
		cfg.CalloutTimeout = 150 * time.Millisecond
		v := cfg.CheckBash(synthetic)
		if !v.Block {
			t.Fatal("a callout that never answers did not block")
		}
		if !strings.Contains(v.Reason, "did not answer within") {
			t.Fatalf("the timeout refusal does not name the timeout:\n%s", v.Reason)
		}
	})
}

// --- the only-widens property ------------------------

// TestCalloutNeverClearsCompiledIndicator is the ONLY-WIDENS proof. A callout that
// says "allow" to absolutely everything is configured, and every compiled indicator's
// case is re-run: each must still block. If the callout were consulted first — or its
// verdict allowed to override — these would pass, which is the exact fail-open shape a
// green suite could otherwise hide.
func TestCalloutNeverClearsCompiledIndicator(t *testing.T) {
	permissive := writeCallout(t, "cat > /dev/null\necho allow\n")
	cfg := Config{SharedRoot: shared, ProjectDir: worktree, Cwd: worktree, Callout: permissive}

	for name, cmd := range map[string]string{
		"output redirection":      "echo pwn > " + shared + "/STATUS.md",
		"tee":                     "echo pwn | tee " + shared + "/STATUS.md",
		"file mutation command":   "rm -rf " + shared + "/docs",
		"sed -i":                  "sed -i 's/a/b/' " + shared + "/STATUS.md",
		"mutating git subcommand": "git -C " + shared + " commit -am x",
	} {
		t.Run(name, func(t *testing.T) {
			v := cfg.CheckBash(cmd)
			if !v.Block {
				t.Fatalf("a permissive callout CLEARED the compiled indicator %q. A callout may "+
					"only ADD blocks; it is consulted after the compiled set precisely so no "+
					"answer of its can reach one", name)
			}
			if strings.Contains(v.Reason, "callout") {
				t.Fatalf("the compiled indicator's refusal was replaced by a callout verdict:\n%s", v.Reason)
			}
		})
	}
}

// TestCalloutReceivesTheRequestEnvelope pins the wire contract an adopter writes
// against: one JSON object on stdin, carrying the verbatim command, the cwd and the
// shared root. A silent change to any field name breaks every deployed callout, so it
// is pinned here rather than left to prose.
func TestCalloutReceivesTheRequestEnvelope(t *testing.T) {
	out := filepath.Join(t.TempDir(), "seen.json")
	c := writeCallout(t, "cat > '"+out+"'\necho allow\n")
	cfg := calloutCfg(c)

	cmd := "frobnicate --build && echo 'quoted \"text\" stays verbatim'"
	if v := cfg.CheckBash(cmd); v.Block {
		t.Fatalf("unexpected block: %s", v.Reason)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the callout received no stdin: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"version":1`,
		`"tool":"Bash"`,
		`"command":`,
		`"cwd":`,
		`"shared_root":`,
		"quoted \\\"text\\\" stays verbatim", // the command survives JSON-escaped, not shell-mangled
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("the request envelope is missing %s:\n%s", want, got)
		}
	}
}

// TestCalloutVerdictParsing pins the tiny answer vocabulary: exactly two words are
// recognised, case-insensitively, and anything else is a failure rather than a
// decision. A guard that maps an unfamiliar answer onto "allow" is a guard that
// allows what it did not understand.
func TestCalloutVerdictParsing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prints string
		block  bool
	}{
		{"allow", "echo allow", false},
		{"ALLOW uppercase", "echo ALLOW", false},
		{"allow with trailing space", "echo 'allow   '", false},
		{"block", "echo block", true},
		{"BLOCK with reason", "echo 'BLOCK because reasons'", true},
		{"allowed but chatty", "echo 'allow, probably'", true}, // not the word "allow"
		{"deny is not vocabulary", "echo deny", true},
		{"yes is not vocabulary", "echo yes", true},
		{"empty line", "echo ''", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := calloutCfg(writeCallout(t, "cat > /dev/null\n"+tc.prints+"\n"))
			v := cfg.CheckBash(synthetic)
			if v.Block != tc.block {
				t.Fatalf("verdict %q: Block=%v, want %v (reason: %s)", tc.name, v.Block, tc.block, v.Reason)
			}
		})
	}
}
