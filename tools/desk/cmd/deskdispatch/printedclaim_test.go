package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// printedCmdArgv runs one backticked command of the NOT LAUNCHED line through sh, with the claim
// tool and desktoken stubbed on PATH, and returns the argv the claim tool receives.
func printedCmdArgv(t *testing.T, line, verb, tok string) []string {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("could-not-check: no sh on PATH")
	}
	var cmdline string
	for _, m := range regexp.MustCompile("`([^`]*)`").FindAllStringSubmatch(line, -1) {
		if strings.HasPrefix(m[1], goClaimBinary+" "+verb+" ") {
			cmdline = m[1]
		}
	}
	if cmdline == "" {
		t.Fatalf("NOT LAUNCHED line carries no %s command:\n%s", verb, line)
	}
	bin := t.TempDir()
	for name, body := range map[string]string{
		"desktoken":   `printf '%s' "$TOK"`,
		goClaimBinary: `printf '%s\0' "$@"`,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!"+sh+"\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(sh, "-c", cmdline)
	cmd.Env = append(os.Environ(), "TOK="+tok, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("sh could not run %q: %v", cmdline, err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
}

// The progress command printed for a branchless verifier claim reaches the claim tool as
// whole words: the key, then each value flag followed by its own value, "-" for the branch.
// deskclaim-ref's TestPrintedProgressRuns runs this argv against the claim it acquired.
// regression: #6896
func TestPrintedClaimArgv(t *testing.T) {
	for _, loop := range []string{"", "verify-desk", "worker-desk"} {
		t.Run("loop="+loop, func(t *testing.T) {
			t.Setenv("DESK_LOOP", loop)
			o := dispatchOpts{item: "stream/05", kit: "verifier"}
			plan := dispatchPlan{claimKey: "at--stream--05", claimReleaseHint: goClaimBinary}
			line := captureStderr(t, func() { o.notLaunched(plan, "medici-finance/assay", "/tmp/home") })
			tok := filepath.Join(t.TempDir(), "tok")
			want := []string{"progress", "at--stream--05", "--repo", "medici-finance/assay", "--branch", "-", "--token-file", tok}
			if got := printedCmdArgv(t, line, "progress", tok); !slices.Equal(got, want) {
				t.Errorf("printed progress parses to\n%q\nwant\n%q", got, want)
			}
			want = []string{"release", "at--stream--05", "--repo", "medici-finance/assay", "--token-file", tok}
			if got := printedCmdArgv(t, line, "release", tok); !slices.Equal(got, want) {
				t.Errorf("printed release parses to\n%q\nwant\n%q", got, want)
			}
		})
	}
}
