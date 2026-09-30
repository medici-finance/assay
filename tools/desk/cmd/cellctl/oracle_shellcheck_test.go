package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestOracleShellcheckClean keeps the bash oracle shellcheck-clean. The port is proved against
// that oracle, and the oracle's own brief pins it with a `shellcheck` Verify row, but no job ran
// shellcheck over it, so word-splitting (SC2086) and subshell (SC2030/SC2031) findings landed on
// main unobserved and sat there until a post-merge verify pass tripped over them. This runs the
// same whole-file check in `go test`, so on any machine with shellcheck installed a new finding
// at ANY site in the oracle is a red test, not only at the sites fixed so far.
//
// Where shellcheck is not installed the test SKIPS and says so: that is could-not-check, never a
// pass. CI does not install shellcheck yet (#1875), so in CI this test skips, and a non-verbose
// `go test` prints that skip as a plain `ok` — the guard binds only where shellcheck exists. The positive control below proves the shellcheck it found still flags a planted SC2086,
// so a broken or stubbed shellcheck fails here instead of reporting the oracle clean.
func TestOracleShellcheckClean(t *testing.T) {
	const oracle = "../../../cellctl/testdata/cellctl-shell-oracle.sh"
	if _, err := os.Stat(oracle); err != nil {
		t.Skipf("could-not-check: oracle not readable from this checkout (%v)", err)
	}
	sc, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("could-not-check: shellcheck is not on PATH, so the oracle was not linted")
	}

	// Positive control: one planted unquoted list expansion, the exact shape this guard exists for.
	plant := filepath.Join(t.TempDir(), "plant.sh")
	body := "#!/usr/bin/env bash\nVALUES=\"a b\"\nvalue_in(){ :; }\nvalue_in x $VALUES\n"
	if err := os.WriteFile(plant, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(sc, "-f", "gcc", plant).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "SC2086") {
		t.Fatalf("positive control: %s did not flag a planted SC2086 (err=%v)\n%s", sc, err, out)
	}

	out, err = exec.Command(sc, "-f", "gcc", oracle).CombinedOutput()
	if err != nil {
		t.Errorf("shellcheck %s: %v\n%s\nQuote the expansion, or, where the word-splitting is "+
			"intended, add a `# shellcheck disable=SCnnnn  # reason` directive on the line above.",
			oracle, err, out)
	}
}
