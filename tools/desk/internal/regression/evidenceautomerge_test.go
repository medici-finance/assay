package regression

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The existing live build-test job runs this package on every push and PR.
// These suites use API stubs and scratch fixtures; none contacts GitHub.
func TestEvidenceAutomergeRefusal(t *testing.T) {
	shellFloor(t, "tools/evidence-automerge/automerge-refusal_test.sh")
}

func TestEvidenceAutomergeStep(t *testing.T) {
	shellFloor(t, "tools/evidence-automerge/automerge-step_test.sh")
}

func TestEvidenceAutomergePoll(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX workflow fixtures run in the Linux build-test job")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), shellBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "tools/evidence-automerge/automerge-poll_test.py"))
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "TMPDIR="+t.TempDir(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("offline Evidence poll suite: %v (deadline: %v)\n%s", err, ctx.Err(), out)
	}
	t.Log(string(out))
}

// Keep the workflow's read-only Python adapter coupled to the desk's canonical
// marker and declaration readers. No runtime/network service is added.
func TestEvidenceCompletionCanonicalReaders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX workflow adapter runs in the Linux floor")
	}
	bodies := []string{"Security-Review: pass", "Security-Review: fail", "Verdict: approve", "Verdict: request-changes", "Undeclared-desk-decision: pending", "ordinary note"}
	var cases []map[string]any
	for _, body := range bodies {
		for _, form := range []string{body, "**" + body + "**", "`" + body + "`", "> " + body, "* " + body, "```text\n" + body + "\n```", "~~~\n" + body + "\n~~~", "| " + body + " |", strings.ReplaceAll(body, "pass", "pa*ss"), strings.ToUpper(body), strings.ReplaceAll(body, "i", "\u0131"), strings.ReplaceAll(body, "i", "\u0130"), strings.ReplaceAll(body, "s", "\u017f")} {
			cases = append(cases, map[string]any{"body": form, "pass": deskkit.HasSecurityReviewPass(form), "fail": deskkit.HasSecurityReviewFail(form), "correctness": deskkit.CorrectnessNoteState(form), "hold": len(deskkit.UndeclaredDeskDecisionLines(form)) > 0})
		}
	}
	// Non-newline separators remain content in the canonical readers.
	for _, separator := range []string{"\v", "\f", "\r", "\u0085", "\u2028", "\u2029"} {
		for _, body := range bodies {
			form := "ordinary prefix" + separator + body
			cases = append(cases, map[string]any{"body": form, "pass": deskkit.HasSecurityReviewPass(form), "fail": deskkit.HasSecurityReviewFail(form), "correctness": deskkit.CorrectnessNoteState(form), "hold": len(deskkit.UndeclaredDeskDecisionLines(form)) > 0})
		}
	}
	block := deskkit.RenderDecidedBlock([]deskkit.DecidedItem{{Decision: "chosen", Alternative: "other", Cost: "small"}})
	var declarations []map[string]any
	for _, body := range []string{"", block, strings.ReplaceAll(block, "   cost: small", ""), "## Desk-decided\nmissing marker", "```text\n" + block + "\n```", block + "```text\n2. decision: missing fields\n```\n", "\u001c" + block, strings.ReplaceAll(block, "alternative", "alternat\u0131ve"), strings.ReplaceAll(block, "cost", "co\u017ft"), "## Desk-decided\n```text\n<!-- desk-r3-decision v1 -->\n1. decision: chosen\n alternative: other\n cost: small\n```\n"} {
		for _, labels := range [][]string{nil, {"desk-decided"}, {"DESK-DECIDED"}, {"de\u017fk-decided"}, {"des\u212a-decided"}} {
			declarations = append(declarations, map[string]any{"body": body, "labels": labels, "clear": deskkit.DeskDecidedRefusal(1, labels, body, nil, "assay-reviewer-app[bot]", "head") == ""})
		}
	}
	head := strings.Repeat("a", 40)
	var timelines []map[string]any
	for _, securityHold := range []bool{false, true} {
		for _, clear := range []bool{false, true} {
			hold := "```text\nSecurity-Review: pass\n```\nUndeclared-desk-decision: pending"
			note, state := "Security-Review: pass", "COMMENTED"
			if securityHold {
				hold = "Security-Review: pass\nUndeclared-desk-decision: pending"
				note = "```text\nSecurity-Review: pass\n```"
			}
			if clear {
				if securityHold {
					note = "Security-Review: pass"
				} else {
					note, state = "Verdict: approve", "APPROVED"
				}
			}
			reviews := []deskkit.DeskDecidedReview{
				{Login: "assay-reviewer-app[bot]", State: "COMMENTED", CommitID: head, Body: "Security-Review: pass"},
				{Login: "assay-reviewer-app[bot]", State: "APPROVED", CommitID: head, Body: "Verdict: approve"},
				{Login: "assay-reviewer-app[bot]", State: "COMMENTED", CommitID: head, Body: hold},
				{Login: "assay-reviewer-app[bot]", State: state, CommitID: head, Body: note},
			}
			timelines = append(timelines, map[string]any{"reviews": reviews, "head": head, "clear": deskkit.DeskDecidedRefusal(1, nil, "", reviews, "assay-reviewer-app[bot]", head) == ""})
		}
	}
	raw, err := json.Marshal(map[string]any{"markers": cases, "declarations": declarations, "timelines": timelines})
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	program := `import json,sys
sys.path.insert(0,sys.argv[1])
import merge_clean as m
for case in json.load(sys.stdin)["markers"]:
 body=case["body"]
 values=m.marker_values(body,r"Verdict:[ \t]*(approve|request-changes)",True)
 actual={"pass":bool(m.marker_values(body,r"Security-Review:[ \t]*pass",True)),"fail":bool(m.marker_values(body,r"Security-Review:[ \t]*fail")),"hold":bool(m.marker_values(body,r"Undeclared-desk-decision:[ \t]*(.+)")),"correctness":"CHANGES_REQUESTED" if m.marker_values(body,r"Verdict:[ \t]*request-changes") else "APPROVED" if any(value.lower()=="approve" for value in values) else ""}
 for key,value in actual.items():
  assert value==case[key],(body,key,value,case[key])
`
	// The same stdin is used in two calls so the second assertion cannot be
	// skipped by an iterator or the marker pass consuming the declaration data.
	declarationProgram := `import json,sys
sys.path.insert(0,sys.argv[1])
import merge_clean as m
for case in json.load(sys.stdin)["declarations"]:
 assert m.declaration_clear(case["body"],case["labels"] or [])==case["clear"],case
`
	timelineProgram := `import json,sys
sys.path.insert(0,sys.argv[1])
import merge_clean as m
for case in json.load(sys.stdin)["timelines"]:
 reviews=[{"id":i+1,"user":{"login":r["Login"],"type":"Bot"},"state":r["State"],"commit_id":r["CommitID"],"body":r["Body"],"submitted_at":"2026-10-12T00:00:00Z"} for i,r in enumerate(case["reviews"])]
 assert m.reviewed(reviews,case["head"])==case["clear"],case
`
	for _, script := range []string{program, declarationProgram, timelineProgram} {
		ctx, cancel := context.WithTimeout(context.Background(), shellBudget)
		cmd := exec.CommandContext(ctx, "python3", "-B", "-c", script, filepath.Join(root, "tools/evidence-automerge"))
		cmd.Stdin = bytes.NewReader(raw)
		cmd.Env = FixtureEnv("KUBECONFIG=/dev/null")
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("canonical reader parity: %v\n%s", err, out)
		}
	}
}
