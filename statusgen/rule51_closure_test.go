package main

// rule51_closure_test.go — the closure gate's side of the rule-51 lane
// (#4823): what `brief --check-verified` (the check a deskevidence landing
// runs) credits for a repo-wide lint row whose offline run is red.
//
// This file uses only identifiers that predate the lane, and builds the
// stamp from its documented digest input itself, so it also runs against the
// code before the change: there TestRule51ClosureCreditsStampedForgeClean
// fails (the stamped witness is a plain failure), and the refusal tests pass.

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const (
	r51Tree    = "abcdef123456"
	r51Date    = "2026-11-19"
	r51OutHash = "0123456789ab"
	r51Pin     = "v1.0.99"
)

// r51Stamp renders a stamp the way the lane documents it: the digest is
// sha256 over newline-joined fields (rule51-v1, row, normalized command,
// offline state, offline exit, offline output hash, tree, date, forge state,
// forge rc, forge output hash, pin, credential class, credential digest).
func r51Stamp(id, command, forgeState, forgeRC, pin, credClass, credCell, tree, date string) string {
	fields := []string{"rule51-v1", id, normalizeCommandText(command), "fail", "1", r51OutHash, tree, date,
		forgeState, forgeRC, "aaaaaaaaaaaa", pin, credClass, credCell}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\n")))
	return "rule51=" + forgeState + " forge-rc=" + forgeRC + " forge-out=sha256:aaaaaaaaaaaa pin=" + pin +
		" cred=" + credClass + " cred-digest=" + credCell + " stamp=sha256:" + hex.EncodeToString(sum[:])
}

func r51Witness(id, command, stamp, tree, date string) string {
	result := "fail exit=1"
	if stamp != "" {
		result += " " + stamp
	}
	return "| " + id + " | `" + command + "` | " + result + " | sha256:" + r51OutHash + " | " + date +
		" | assay-verifier-app[bot] @ " + tree + " |"
}

func r51Verify(command string) string {
	return "| # | Command | Expect |\n|---|---|---|\n| 1 | `" + command + "` | exit 0, `LINT: PASS` |\n"
}

func r51Close(t *testing.T, verify, evidence string) error {
	t.Helper()
	_, err := checkSelectedClosure(t.TempDir(), "x/01", "verified", "2026-11-19 assay-verifier-app[bot]",
		briefArtifacts{Verify: verify, Evidence: evidence})
	return err
}

const r51Cred = "sha256:0f0f0f0f0f0f"

// TestRule51ClosureCreditsStampedForgeClean: a lint row red offline whose
// witness carries a well-formed rule-51 stamp with a clean forge run lands.
func TestRule51ClosureCreditsStampedForgeClean(t *testing.T) {
	for _, cmd := range []string{"statusgen --root . --lint", "statusgen --lint", "statusgen --lint --root .", "cd statusgen && go run . --root .. --lint"} {
		stamp := r51Stamp("1", cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date)
		evidence := witnessHeader + "\n" + r51Witness("1", cmd, stamp, r51Tree, r51Date) + "\n"
		if err := r51Close(t, r51Verify(cmd), evidence); err != nil {
			t.Errorf("%s: a stamped forge-clean rule-51 witness was refused: %v", cmd, err)
		}
	}
}

// TestRule51ClosureRefusals: every other shape stays exactly as strict.
func TestRule51ClosureRefusals(t *testing.T) {
	const cmd = "statusgen --root . --lint"
	good := r51Stamp("1", cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date)
	diffCmd := "statusgen --root . --lint --diff-base refs/remotes/origin/main"
	cases := []struct {
		name, verify, evidence string
	}{
		{"forge red", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "fail", "1", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"forge could-not-run", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "could-not-run", "1", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"forge pass label with non-zero rc", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "1", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"unstamped offline red", r51Verify(cmd), r51Witness("1", cmd, "", r51Tree, r51Date)},
		{"hand citation in prose", r51Verify(cmd), r51Witness("1", cmd, "", r51Tree, r51Date) + "\n\nRule 51: `statusgen --root . --lint --acceptance-forge` exited 0 with `LINT: PASS` at " + r51Tree + ", sha256:aaaaaaaaaaaa."},
		{"hand-edited stamp (digest does not bind)", r51Verify(cmd), r51Witness("1", cmd, good[:len(good)-4]+"0000", r51Tree, r51Date)},
		{"stamp copied to another tree", r51Verify(cmd), r51Witness("1", cmd, good, "fedcba654321", r51Date)},
		{"stamp copied to another date", r51Verify(cmd), r51Witness("1", cmd, good, r51Tree, "2026-11-20")},
		{"dev pin", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "0", "dev", "verifier-run-read", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"no credential", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "0", r51Pin, "verifier-run-read", "-", r51Tree, r51Date), r51Tree, r51Date)},
		{"another credential class", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "0", r51Pin, "ambient-login", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"dirty tree", r51Verify(cmd), r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, r51Tree+"+dirty", r51Date), r51Tree+"+dirty", r51Date)},
		{"diff-base row", r51Verify(diffCmd), r51Witness("1", diffCmd, r51Stamp("1", diffCmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, r51Tree, r51Date), r51Tree, r51Date)},
		{"Expect asks for a problem count", "| # | Command | Expect |\n|---|---|---|\n| 1 | `" + cmd + "` | exit 1 |\n", r51Witness("1", cmd, good, r51Tree, r51Date)},
	}
	for _, c := range cases {
		err := r51Close(t, c.verify, witnessHeader+"\n"+c.evidence+"\n")
		if err == nil {
			t.Errorf("%s: credited, want refused", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "row 1: fail") {
			t.Errorf("%s: refused for another reason: %v", c.name, err)
		}
	}
}
