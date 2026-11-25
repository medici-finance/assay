package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

func TestCoverageReplayKeepsOriginalPinAndWitness(t *testing.T) {
	f := newGitV2Fixture(t)
	f.AfterMerge = map[string]string{
		evaluation.WitnessPinFile: strings.Replace(gv2Pins, "statusgen/v1.0.59", "statusgen/v1.0.60", 1),
	}
	f.build()
	bin := buildStatusgenBinary(t)
	cmd := exec.Command(bin, "--root", f.Root, "--coverage", "--brief", gv2BriefID, "--coverage-replay-at", f.Merge, "--json")
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("coverage replay: %v\n%s", err, &diagnostic)
	}
	var report coverageReplayReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode coverage replay: %v\n%s", err, output)
	}
	head := gv2Git(t, f.Root, "rev-parse", "HEAD")
	if !report.Advisory || report.Head.Revision != head || report.Replay.Revision != f.Merge {
		t.Fatalf("unexpected subjects: %+v", report)
	}
	for _, row := range []string{"1", "2"} {
		current := claimFor(t, report.Head.Coverage, row)
		if current.Released || !strings.Contains(current.Reason, evaluation.WitnessPinFile) {
			t.Errorf("row %s should hold at HEAD on the new pin: %+v", row, current)
		}
		old := claimFor(t, report.Replay.Coverage, row)
		if !old.Released || old.Result != covPass || old.Revision != f.Base {
			t.Errorf("row %s did not retain its original run revision and PASS: %+v", row, old)
		}
	}
	if dirty := gv2Git(t, f.Root, "status", "--porcelain"); dirty != "" || f.coverage().Released {
		t.Fatalf("replay changed the tree or the normal coverage guard: dirty=%q", dirty)
	}
}

func TestCoverageReplayKeepsHolds(t *testing.T) {
	f := newGitV2Fixture(t).build()
	replay := func(commit string) coverageReplaySide {
		t.Helper()
		var out, diagnostic bytes.Buffer
		if code := runCoverageReplay(f.Root, gv2BriefID, commit, true, &out, &diagnostic); code != coverageExitOK {
			t.Fatalf("replay exited %d: %s", code, &diagnostic)
		}
		var report coverageReplayReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		return report.Replay
	}
	if c := claimFor(t, replay(f.Base).Coverage, "1"); evaluation.WitnessGap(c.Reason) != evaluation.GapRecordMissing {
		t.Fatalf("a pre-record replay must report the missing record: %+v", c)
	}
	rows := append([]gv2Row(nil), f.Rows...)
	rows[0].Expect = "exit 0; prints A"
	gv2Write(t, f.Root, gv2BriefRel(), f.briefWithRows(rows))
	changed := replay(f.Merge).Coverage
	if c := claimFor(t, changed, "1"); c.Released || !strings.Contains(strings.ToLower(c.Reason), "expect") {
		t.Fatalf("the old record cannot satisfy a changed criterion: %+v", c)
	}
	if c := claimFor(t, changed, "2"); !c.Released {
		t.Fatalf("a changed criterion must not invalidate the other row: %+v", c)
	}
	bf := mustParseBriefFile(t, f.BriefPath)
	failure := covWitnessRow("2", f.Rows[1].Command, stateFail, f.Base)
	gv2Write(t, f.Root, gv2BriefRel(), gv2Brief(f.Inputs, f.Rows, strings.TrimSpace(bf.Evidence)+"\n"+failure))
	if c := claimFor(t, replay(f.Merge).Coverage, "2"); c.Result != covFail || c.Released {
		t.Fatalf("the later failure must hold the row: %+v", c)
	}
}

func TestCoverageReplayRefusesUnresolvedSubjects(t *testing.T) {
	f := newGitV2Fixture(t).build()
	gv2Git(t, f.Root, "checkout", "--detach", f.Base)
	for _, revision := range []string{f.Base[:12], f.Merge} {
		var out, diagnostic bytes.Buffer
		if code := runCoverageReplay(f.Root, gv2BriefID, revision, true, &out, &diagnostic); code != coverageExitCouldNot || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Errorf("replay at %s: exit %d, stdout=%q, stderr=%q", revision, code, &out, &diagnostic)
		}
	}
}
