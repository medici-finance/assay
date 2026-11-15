package main

import (
	"os"
	"strings"
	"testing"
)

// regression: #2026
// TestCloseBracketReadSetForge: the close check removes a row's read-set
// token only as verifyrun writes it (review S21). A token-shaped span moved
// ahead of the tree, carrying a principal or a tree, after the source, or
// with any content verifyrun does not write, stays in the cell and refuses
// the close, so a token cannot hide a forged identity, tree or principal
// from the compare while the whole-cell readers still see it.
func TestCloseBracketReadSetForge(t *testing.T) {
	fx := newContainerFixture(t)
	isolateGitIdentity(t)
	t.Setenv("DESK_LOOP", "")
	tick := pinBracketClock(t)
	putFile(t, fx.check, validCheck)
	if res, stderr := captureVerifyrun(t, fx.openArgs()); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d stderr %q", res.code, stderr)
	}
	tick()
	honest := getFile(t, fx.receipt)
	hostTree := receiptTree(t, honest)
	if err := os.Remove(fx.witness); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	tok := rowReadSetToken()
	cell := "human:fixture @ " + hostTree + " " + tok + " (git-config)"
	if cells := witnessRunnerCells(witness); len(cells) != 1 || cells[0] != cell {
		t.Fatalf("fixture: the container's Runner cell is %q, want %q", cells, cell)
	}
	for name, forged := range map[string]string{
		"token ahead of tree": "human:fixture (reads could-not-check: @ 0123456789ab) @ " + hostTree + " (git-config)",
		"token names a tree":  "human:fixture @ " + hostTree + " (reads could-not-check: @ 0123456789ab) (git-config)",
		"token principal":     "human:fixture @ " + hostTree + " (reads could-not-check: on-behalf-of human:mallory) (git-config)",
		"token after source":  "human:fixture @ " + hostTree + " (git-config) " + tok,
		"free-text reason":    "human:fixture @ " + hostTree + " (reads could-not-check: anything) (git-config)",
		"short digest":        "human:fixture @ " + hostTree + " (reads sha256:abc) (git-config)",
		"two tokens":          "human:fixture @ " + hostTree + " " + tok + " " + tok + " (git-config)",
	} {
		text := strings.Replace(witness, "| "+cell+" |", "| "+forged+" |", 1)
		if text == witness {
			t.Fatalf("%s: fixture edit did not change the witness", name)
		}
		putFile(t, fx.witness, text)
		res, stderr := captureVerifyrun(t, fx.closeArgs())
		if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "is not the cell the host derived") {
			t.Fatalf("%s: exit %d stderr %q, want the row-runner refusal", name, res.code, stderr)
		}
		if got := getFile(t, fx.witness); got != text {
			t.Fatalf("%s: a refused close changed the witness:\n%s", name, got)
		}
	}

	// Control: the token as verifyrun wrote it closes.
	putFile(t, fx.witness, witness)
	if res, stderr := captureVerifyrun(t, fx.closeArgs()); res.code != verifyrunExitPass {
		t.Fatalf("honest close: exit %d stderr %q", res.code, stderr)
	}
}

// regression: #2026
// TestFlipRunnerReadSetStrip: the flip's runner agreement drops a row's
// read-set token only as verifyrun writes it (review S21); any other
// token-shaped span stays in the runner it reports.
func TestFlipRunnerReadSetStrip(t *testing.T) {
	tok := rowReadSetToken()
	sha := "(reads sha256:" + strings.Repeat("ab", 32) + ")"
	for name, c := range map[string]struct{ cell, want string }{
		"writer token":      {"v @ abc " + tok + " (git-config)", "v @ abc (git-config)"},
		"other host's goos": {"v @ abc " + readSetOffLinuxToken("plan9") + " (git-config)", "v @ abc (git-config)"},
		"container token":   {"v @ abc (reads could-not-check: no read-set recorder in this build)", "v @ abc"},
		"digest token":      {"v @ abc " + sha + " (git-config)", "v @ abc (git-config)"},
		"goos with space":   {"v @ abc (reads could-not-check: recorder is Linux-only (a b))", "v @ abc (reads could-not-check: recorder is Linux-only (a b))"},
		"no token":          {"v @ abc (git-config)", "v @ abc (git-config)"},
		"token ahead of @":  {"v (reads could-not-check: @ x) @ abc", "v (reads could-not-check: @ x) @ abc"},
		"token principal":   {"v @ abc (reads could-not-check: on-behalf-of human:m)", "v @ abc (reads could-not-check: on-behalf-of human:m)"},
		"token after other": {"v @ abc (git-config) " + tok, "v @ abc (git-config) " + tok},
	} {
		table := "| # | Result | Date | Runner |\n|---|---|---|---|\n| 1 | PASS | 2026-11-08 | " + c.cell + " |"
		rows, _ := flipDateRunnerRows(strings.Split(table, "\n"))
		if len(rows) != 1 || rows[0][1] != c.want {
			t.Errorf("%s: runner %q, want %q", name, rows, c.want)
		}
	}
	if got := stripWriterReadSetTokens("ran " + tok + " fine (reads could-not-check: other)"); got != "ran fine (reads could-not-check: other)" {
		t.Errorf("row-text strip: %q", got)
	}
}
