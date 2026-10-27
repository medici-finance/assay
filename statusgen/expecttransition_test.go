package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func expectTransitionFixture(t *testing.T, command, atAmendment bool) transitionFixture {
	t.Helper()
	f := contextFixture(t, nil, func(stage, body string) string {
		if stage == "original" {
			body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `true` | exit 1 |", 1)
		}
		if stage == "delivery" && !atAmendment || stage == "amendment" && atAmendment {
			body = strings.Replace(body, "| 1 | `true` | exit 1 |", "| 1 | `true` | exit 0 |", 1)
		}
		if stage == "amendment" && !command {
			body = strings.Replace(body, "`sh check.sh --no-pull`", "`sh check.sh`", 1)
		}
		return body
	})
	rows := "1"
	if command {
		rows = "1,2"
	}
	setExpectApproval(t, &f, rows)
	return f
}

func setExpectApproval(t *testing.T, f *transitionFixture, rows string) {
	t.Helper()
	i := strings.LastIndex(f.payload, "rows=")
	f.payload = f.payload[:i] + "rows=" + rows
	login, ok := HumanLogin("alex")
	if !ok {
		t.Fatal("fixture human missing")
	}
	f.forge.commentBody = rlCommentJSONWith(login, "User", rlRepo, rlIssue, f.payload, rlPosted, rlPosted)
}

// regression: #4505. The actual shared plan must retain original assertions by
// default and apply only the precise original-row columns approved by the human.
func TestExpectOnlyTransition(t *testing.T) {
	for _, name := range []string{"delivery-expect", "amendment-expect", "with-command", "command-only-approval", "missing-command", "supplemental-id", "duplicate-id", "reordered-id", "offline"} {
		t.Run(name, func(t *testing.T) {
			command := name != "delivery-expect" && name != "amendment-expect"
			f := expectTransitionFixture(t, command, name == "amendment-expect")
			switch name {
			case "command-only-approval":
				setExpectApproval(t, &f, "2")
			case "missing-command":
				setExpectApproval(t, &f, "1")
			case "supplemental-id":
				setExpectApproval(t, &f, "1,2,3")
			case "duplicate-id":
				setExpectApproval(t, &f, "1,1,2")
			case "reordered-id":
				setExpectApproval(t, &f, "2,1")
			case "offline":
				transitionForge = false
			}
			verify, evidence, err := briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			plan, why := protectedAcceptanceFor(verify, evidence, f.descriptor, f.path)
			want := name == "delivery-expect" || name == "amendment-expect" || name == "with-command" || name == "command-only-approval"
			if (why == "") != want {
				t.Fatalf("accepted=%v want=%v: %s", why == "", want, why)
			}
			if !want {
				return
			}
			rows := briefVerifyRows(plan.Verify)
			expect := "exit 0"
			if name == "command-only-approval" {
				expect = "exit 1"
			}
			if rows[0].Command != "true" || rows[0].Expect != expect {
				t.Fatalf("row1: %+v", rows[0])
			}
			if name == "command-only-approval" && plan.Revisions["1"] != "" {
				t.Fatal("unapproved Expect received amendment credit")
			}
			if name != "command-only-approval" && plan.Revisions["1"] != f.amendment {
				t.Fatal("approved Expect has no immutable amendment source")
			}
			// A coordinate-only protected record is deliberately not an approval.
			_, why = protectedAcceptanceFor(verify, evidence, strings.Split(f.descriptor, "\n")[0], f.path)
			if why == "" { // The planner retains original rows; the actual runner refuses drift.
				original, _ := protectedAcceptanceFor(verify, evidence, strings.Split(f.descriptor, "\n")[0], f.path)
				if originalDefinitionDrift(original.Verify, verify) == "" {
					t.Fatal("protected record accepted original Expect drift")
				}
			}
		})
	}
}

// regression: #4505. Execute the production route and retain its own generated
// witnesses; a supplied PASS or hand-written acceptance-boolean is never used.
func TestExpectTransitionExecution(t *testing.T) {
	f := expectTransitionFixture(t, false, false)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0600); err != nil {
		t.Fatal(err)
	}
	r, diag := captureVerifyrun(t, []string{"--root", f.root, "--brief", f.path, "--acceptance-record", descriptor, "--acceptance-forge"})
	if r.code != 0 {
		t.Fatalf("execution %d: %s", r.code, diag)
	}
	_, ev, err := briefSections(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ev, "Acceptance-Changes: 1=Expect") {
		t.Fatalf("missing changed-column receipt: %s", ev)
	}
	rows := parseEvidenceRows(ev)
	if len(rows["1"]) != 1 || witnessCommandOf(rows["1"][0].Text) != "true" || witnessStateOf(rows["1"][0].Text) != statePass {
		t.Fatalf("row1: %+v", rows["1"])
	}
	if !strings.Contains(rows["3"][0].Text, "author-added;") {
		t.Fatal("supplemental row gained credit")
	}
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	code, out, diag := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01", "--acceptance-forge"})
	if code != 0 {
		t.Fatalf("closure %d: %s %s", code, out, diag)
	}
	streams, _, err := loadStreams(f.root)
	if err != nil {
		t.Fatal(err)
	}
	coverage := evaluateCoverage(f.root, streams, coverageOptions{Revision: f.amendment})
	if !coverage["vf/01"].Released {
		t.Fatalf("approved Expect coverage: %+v", coverage["vf/01"])
	}
}

// regression: #4505. An approval of an Expect cannot hide another changed
// column, and author-added rows stay outside the transition's authority.
func TestExpectTransitionRefusals(t *testing.T) {
	for _, name := range []string{"hidden-command", "supplemental-expect", "changed-class"} {
		t.Run(name, func(t *testing.T) {
			f := contextFixture(t, nil, func(stage, body string) string {
				if stage == "original" {
					body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `true` | exit 1 |", 1)
				}
				if stage == "delivery" {
					body = strings.Replace(body, "| 1 | `true` | exit 1 |", "| 1 | `true` | exit 0 |", 1)
				}
				if stage == "amendment" {
					switch name {
					case "hidden-command":
						body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `false` | exit 0 |", 1)
					case "supplemental-expect":
						body = strings.Replace(body, "| 3 | `printf supplemental` | exit 0 |", "| 3 | `printf supplemental` | exit 1 |", 1)
					case "changed-class":
						body = strings.Replace(body, "| # | Command | Expect |", "| # | Command | Expect | Class |", 1)
						body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `true` | exit 0 | check:ci |", 1)
					}
				}
				return body
			})
			setExpectApproval(t, &f, "1,2")
			verify, evidence, err := briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			if _, why := protectedAcceptanceFor(verify, evidence, f.descriptor, f.path); why == "" {
				t.Fatal("unsupported column change accepted")
			}
		})
	}
}

// regression: #4505. A command-only ruling cannot silently acquire approval of
// an earlier delivered Expect; the runner still tests the original assertion.
func TestExpectScopeExecution(t *testing.T) {
	for _, name := range []string{"command-only", "protected-record"} {
		t.Run(name, func(t *testing.T) {
			f := expectTransitionFixture(t, true, false)
			setExpectApproval(t, &f, "2")
			vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
			vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
			text := f.descriptor
			if name == "protected-record" {
				text = strings.Split(text, "\n")[0]
			}
			descriptor := filepath.Join(t.TempDir(), "transition.txt")
			if err := os.WriteFile(descriptor, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			r, diag := captureVerifyrun(t, []string{"--root", f.root, "--brief", f.path, "--acceptance-record", descriptor, "--acceptance-forge"})
			_, ev, err := briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			if name == "protected-record" {
				if r.code != verifyrunExitCouldNot || len(parseEvidenceRows(ev)) != 0 {
					t.Fatalf("protected record ran original drift: %d %s %s", r.code, diag, ev)
				}
				return
			}
			rows := parseEvidenceRows(ev)
			if r.code != verifyrunExitFail || len(rows["1"]) != 1 || witnessStateOf(rows["1"][0].Text) != stateFail {
				t.Fatalf("original assertion was not owed: %d %s %s", r.code, diag, ev)
			}
			if !strings.Contains(ev, "Acceptance-Changes: 2=Command") || strings.Contains(ev, "1=Expect") {
				t.Fatal("command-only ruling was upgraded to Expect approval")
			}
		})
	}
}
