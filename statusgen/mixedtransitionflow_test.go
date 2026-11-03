package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #4299
// The real host-transition, open/run/close and closure entrypoints assemble one
// protected run. Only admission transport, approval lookup and the container's
// interface inventory are fixture seams: no model, live forge or Docker runs.
// Container witness rows and the author-added disposition are producer output,
// not a hand-authored substitute for either lane.
func TestMixedTransitionProductionFlow(t *testing.T) {
	pinClock(t)
	// The base fixture plants an unrelated original-command edit to test
	// re-derivation. This flow selects current container rows, so retain that
	// original row unchanged rather than imply an unapproved edit is accepted.
	f := contextFixture(t, nil, func(stage, body string) string {
		if stage == "delivery" {
			return strings.Replace(body, "| 1 | `false` | exit 9 |", "| 1 | `true` | exit 0 |", 1)
		}
		return body
	})
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	fx := newContainerFixture(t)
	fx.home, fx.brief = f.root, f.path
	t.Setenv("DESK_LOOP", "")
	putFile(t, fx.check, checkJSON(1, fixtureRun, f.amendment, vfBriefRel, "m"))
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0600); err != nil {
		t.Fatal(err)
	}
	// The protected host execution must start on its immutable clean source.
	// A prior uncommitted witness cannot be hidden by the Evidence-only token.
	originalBrief := getFile(t, f.path)
	putFile(t, f.path, strings.Replace(originalBrief, "\n## Review", "\nAn earlier witness is not committed.\n\n## Review", 1))
	resetReadMemo()
	if res, out := captureVerifyrunOut(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge", "--rows", "3"}); res.code == 0 || !strings.Contains(out, "clean immutable source") {
		t.Fatalf("unclean transition execution did not refuse: %d %s", res.code, out)
	}
	putFile(t, f.path, originalBrief)
	resetReadMemo()
	res, out := captureVerifyrunOut(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge", "--rows", "3"})
	if res.code != verifyrunExitPass {
		t.Fatalf("host supplemental: %d %s", res.code, out)
	}
	rows := parseEvidenceRows(evidenceOf(t, f.path))["3"]
	if len(rows) != 1 || !strings.Contains(rows[0].Text, "author-added;") || witnessTreeOf(rows[0].Text) != f.amendment {
		t.Fatalf("host supplemental classification/source: %+v", rows)
	}
	clock := nowFunc()
	saved := nowFunc
	nowFunc = func() time.Time { return clock }
	defer func() { nowFunc = saved }()
	if res, diag := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}); res.code != 0 {
		t.Fatalf("open:%d %s", res.code, diag)
	}
	if res, out := captureVerifyrunOut(t, append(fx.runArgs(), "--rows", "1,2")); res.code != 0 {
		t.Fatalf("container:%d %s", res.code, out)
	}
	clock = clock.Add(time.Minute)
	closeArgs := []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
	// The producer's short token is usable only with the exact host-bound cell.
	// A forged source is rejected before a bracket is written.
	honestWitness := getFile(t, fx.witness)
	forgedWitness := strings.ReplaceAll(honestWitness, " @ "+f.amendment[:12], " @ "+f.delivery[:12])
	if forgedWitness == honestWitness {
		t.Fatal("container did not emit the short source token")
	}
	putFile(t, fx.witness, forgedWitness)
	if res, diag := captureVerifyrun(t, closeArgs); res.code == 0 || !strings.Contains(diag, "Runner") {
		t.Fatalf("forged source close: %d %s", res.code, diag)
	}
	if got := getFile(t, fx.witness); got != forgedWitness {
		t.Fatal("refused close modified the forged witness")
	}
	putFile(t, fx.witness, honestWitness)
	sourcePath := filepath.Join(f.root, "check.sh")
	honestSource := getFile(t, sourcePath)
	putFile(t, sourcePath, honestSource+"# changed source\n")
	if res, diag := captureVerifyrun(t, closeArgs); res.code == 0 {
		t.Fatalf("changed source closed:%s", diag)
	}
	putFile(t, sourcePath, honestSource)
	if res, diag := captureVerifyrun(t, []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}); res.code != 0 {
		t.Fatalf("close:%d %s", res.code, diag)
	}
	vfAppend(t, f.root, "\n\n"+getFile(t, fx.witness)+"\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	if res, diag := captureVerifyrun(t, []string{"--check", "--brief", f.path, "--acceptance-forge"}); res.code != 0 {
		t.Fatalf("mixed closure:%d %s", res.code, diag)
	}
	honestBrief := getFile(t, f.path)
	putFile(t, f.path, strings.Replace(honestBrief, "author-added; execution exit=0; no acceptance credit", "pass exit=0", 1))
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	if res, diag := captureVerifyrun(t, []string{"--check", "--brief", f.path, "--acceptance-forge"}); res.code == 0 {
		t.Fatalf("supplemental credit forgery closed:%s", diag)
	}
}
