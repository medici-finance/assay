package regression

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// cdIntoMktempRE matches a cd (or pushd) whose operand is a mktemp command substitution:
// `cd "$(mktemp -d …)"`, unquoted or with backticks. On bash 3.2 `cd ""` succeeds and stays put,
// so when mktemp fails the cd lands in the caller's working directory, and a scratch path
// resolved from there (`T="$(cd "$(mktemp -d)" && pwd -P)"`) names that directory.
var cdIntoMktempRE = regexp.MustCompile("\\b(cd|pushd)(\\s+-[LPe@]+)*\\s+[\"']?(\\$\\(|`)\\s*mktemp\\b")

// trapRE and recursiveRmRE find what turns that into data loss: an EXIT trap, set inline or
// through a cleanup function, that removes a directory tree.
var (
	trapRE        = regexp.MustCompile(`(^|[;&|\s])trap\s`)
	recursiveRmRE = regexp.MustCompile(`\brm\s+(-[a-zA-Z]*[rR][a-zA-Z]*|--recursive)\b`)
)

// scratchTrapFaults returns file:line for every line in src, outside comments, that resolves a
// path through `cd "$(mktemp …)"` when a later line sets a trap and the script removes a tree.
func scratchTrapFaults(name string, src []byte) []string {
	lines := strings.Split(string(src), "\n")
	code := make([]bool, len(lines))
	for i, line := range lines {
		code[i] = !strings.HasPrefix(strings.TrimSpace(line), "#")
	}
	var faults []string
	for i, line := range lines {
		if !code[i] || !cdIntoMktempRE.MatchString(line) {
			continue
		}
		trap, rm := false, false
		for j := i; j < len(lines); j++ {
			if code[j] {
				trap = trap || trapRE.MatchString(lines[j])
				rm = rm || recursiveRmRE.MatchString(lines[j])
			}
		}
		if trap && rm {
			faults = append(faults, fmt.Sprintf("%s:%d: resolves a scratch path through cd \"$(mktemp …)\" ahead of a removing trap; "+
				"assign mktemp's output, check it is a non-empty directory, then cd (#6161)", name, i+1))
		}
	}
	return faults
}

// shellLike reports whether a file is shell the guard reads: a shell or workflow extension, or
// an extensionless file with a shell shebang. Markdown is prose and is not read.
func shellLike(path string, head []byte) bool {
	switch filepath.Ext(path) {
	case ".sh", ".bash", ".bats", ".zsh", ".ksh", ".yml", ".yaml":
		return true
	case "":
		first, _, _ := bytes.Cut(head, []byte("\n"))
		return bytes.HasPrefix(first, []byte("#!")) && bytes.Contains(first, []byte("sh"))
	}
	return false
}

// TestScratchTrapNeverResolvesCwd is the class guard for an EXIT trap that deletes the caller's
// working directory: no shell file in the tree may resolve its scratch path through
// `cd "$(mktemp …)"` ahead of a trap that removes it. Planted controls keep the matcher honest.
// regression: #6161
func TestScratchTrapNeverResolvesCwd(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tools", "cellctl", "tests", "down.test.sh")); err != nil {
		t.Fatalf("repository root %s does not hold the cellctl suites: %v", root, err)
	}
	scanned, suites := 0, 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !shellLike(path, src) {
			return nil
		}
		scanned++
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "tools/cellctl/tests/") && strings.HasSuffix(rel, ".test.sh") {
			suites++
		}
		if faults := scratchTrapFaults(rel, src); len(faults) > 0 {
			t.Errorf("%s", strings.Join(faults, "\n"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if suites < 18 {
		t.Errorf("scanned %d cellctl suites (%d shell files in all), want at least 18", suites, scanned)
	}
	planted := "#!/usr/bin/env bash\n" +
		"T=\"$(cd \"$(mktemp -d \"${TMPDIR:-/tmp}/x.XXXXXX\")\" && pwd -P)\"\n" +
		"tmp=`cd $(mktemp -d) && pwd`\n" +
		"# T=\"$(cd \"$(mktemp -d)\" && pwd -P)\" in a comment is not a site\n" +
		"made=\"$(mktemp -d)\" || exit 1\n" +
		"[ -n \"$made\" ] && [ -d \"$made\" ] || exit 1\n" +
		"S=\"$(cd \"$made\" && pwd -P)\" || exit 1\n" +
		"trap 'rm -rf \"$T\"' EXIT\n" +
		"U=\"$(cd \"$(mktemp -d)\" && pwd -P)\"\n"
	got := strings.Join(scratchTrapFaults("plant.sh", []byte(planted)), "\n")
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, "plant.sh:2:") || !strings.Contains(got, "plant.sh:3:") {
		t.Errorf("planted controls: want exactly lines 2 and 3 flagged (line 9 follows the trap), got:\n%s", got)
	}
	viaFunction := "tmp=\"$(cd \"$(mktemp -d)\" && pwd -P)\"\ncleanup() { rm -rf \"$tmp\"; }\ntrap cleanup EXIT\n"
	if got := scratchTrapFaults("fn.sh", []byte(viaFunction)); len(got) != 1 {
		t.Errorf("planted control: a cleanup-function trap must flag line 1, got %q", got)
	}
	noTrap := "T=\"$(cd \"$(mktemp -d)\" && pwd -P)\"\nrm -rf \"$T/sub\"\n"
	if got := scratchTrapFaults("notrap.sh", []byte(noTrap)); len(got) != 0 {
		t.Errorf("planted control: no trap, no fault; got %q", got)
	}
}

// hazardBashes returns every distinct bash this host has among the system one and the one on
// PATH, keyed by resolved path, with its major version.
func hazardBashes(t *testing.T) map[string]string {
	t.Helper()
	candidates := []string{"/bin/bash", "/opt/homebrew/bin/bash", "/usr/local/bin/bash"}
	if found, err := exec.LookPath("bash"); err == nil {
		candidates = append(candidates, found)
	}
	out := map[string]string{}
	for _, c := range candidates {
		real, err := filepath.EvalSymlinks(c)
		if err != nil {
			continue
		}
		if _, seen := out[real]; seen {
			continue
		}
		probe := exec.Command(real, "-c", `printf %s "${BASH_VERSINFO[0]}"`)
		probe.Env = FixtureEnv()
		v, err := probe.Output()
		if err != nil {
			continue
		}
		out[real] = string(v)
	}
	if len(out) == 0 {
		t.Skip("prerequisite: bash is required for the shell hazard fixture")
	}
	return out
}

// runFromVictim runs script under bash with mktemp shimmed to fail, from victim (which holds a
// sentinel), and reports whether the sentinel survived, the output and the exit error.
func runFromVictim(t *testing.T, bash, script, victim string) (kept bool, out []byte, err error) {
	t.Helper()
	shim := t.TempDir()
	if err := os.WriteFile(filepath.Join(shim, "mktemp"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(victim, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), shellBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, script)
	cmd.Dir = victim
	cmd.Env = FixtureEnv("PATH="+shim+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin",
		"TMPDIR="+t.TempDir(), "HOME="+t.TempDir(), "KUBECONFIG=/dev/null", "LC_ALL=C")
	out, err = cmd.CombinedOutput()
	_, statErr := os.Stat(sentinel)
	return statErr == nil, out, err
}

// TestScratchTrapFailClosed proves the hazard itself: a fixed suite run with mktemp failing, from
// a directory holding a sentinel, exits non-zero and leaves the sentinel. On bash 3.2 a planted
// copy of the old line, run only against a disposable directory under this test's own
// t.TempDir(), deletes its sentinel, so the shim provokes the real failure.
// regression: #6161
func TestScratchTrapFailClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	suite := filepath.Join(root, "tools", "cellctl", "tests", "down.test.sh")
	if _, err := os.Stat(suite); err != nil {
		t.Fatal(err)
	}
	unfixed := filepath.Join(t.TempDir(), "unfixed.sh")
	if err := os.WriteFile(unfixed, []byte("#!/usr/bin/env bash\nset -euo pipefail\n"+
		"T=\"$(cd \"$(mktemp -d \"${TMPDIR:-/tmp}/unfixed.XXXXXX\")\" && pwd -P)\"\n"+
		"trap 'rm -rf \"$T\"' EXIT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controlled := false
	for bash, major := range hazardBashes(t) {
		t.Run("bash"+major, func(t *testing.T) {
			victim := filepath.Join(t.TempDir(), "cwd")
			if err := os.Mkdir(victim, 0o700); err != nil {
				t.Fatal(err)
			}
			kept, out, err := runFromVictim(t, bash, suite, victim)
			if err == nil {
				t.Errorf("%s: down.test.sh exited 0 with mktemp failing\n%s", bash, out)
			}
			if !kept {
				t.Errorf("%s: down.test.sh deleted its working directory's sentinel with mktemp failing\n%s", bash, out)
			}
			if major != "3" {
				return
			}
			disposable := filepath.Join(t.TempDir(), "disposable")
			if err := os.Mkdir(disposable, 0o700); err != nil {
				t.Fatal(err)
			}
			if kept, out, _ := runFromVictim(t, bash, unfixed, disposable); kept {
				t.Errorf("%s: control: the unfixed line left its sentinel, so the shim did not provoke the hazard\n%s", bash, out)
			}
			controlled = true
		})
	}
	if !controlled {
		t.Log("no bash 3.2 on this host: the fixed suite is proven, the unfixed control is not run")
	}
}
