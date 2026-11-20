package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #7308
func TestPlainCommandApproval(t *testing.T) {
	const original = "sh check.sh"
	const next = "out=$(mktemp) && trap 'rm -f \"$out\"' 0 && sh check.sh --no-pull > \"$out\""
	for _, mode := range []string{"approved", "legacy", "v1-link", "Command", "missing", "wrong-predecessor",
		"expect", "class", "shell", "obligation", "environment", "extra-column", "delivery-environment", "span", "gate"} {
		t.Run(mode, func(t *testing.T) {
			f := contextFixture(t, []bool{true}, func(stage, body string) string {
				switch stage {
				case "original":
					body = strings.Replace(body, "| Expect |", "| Expect | Class | Shell | Env | Note |", 1)
					class := "check +flow"
					if mode == "gate" {
						class = "gate:human"
					}
					return strings.Replace(body, "| 2 | \x60sh check.sh\x60 | exit 1 |", "| 2 | "+original+" | exit 1 | "+class+" | sh | repo | contract A |", 1)
				case "delivery":
					if mode == "delivery-environment" {
						return strings.Replace(body, "| repo |", "| host |", 1)
					}
				case "amendment":
					body = strings.Replace(body, "| "+original+" |", "| "+next+" |", 1)
					switch mode {
					case "expect":
						return strings.Replace(body, "| exit 1 |", "| exit 0 |", 1)
					case "class":
						return strings.Replace(body, "| check +flow |", "| gate:model +flow |", 1)
					case "shell":
						return strings.Replace(body, "| sh |", "| pwsh |", 1)
					case "obligation":
						return strings.Replace(body, "| check +flow |", "| check |", 1)
					case "environment":
						return strings.Replace(body, "| repo |", "| host |", 1)
					case "extra-column":
						return strings.Replace(body, "| contract A |", "| contract B |", 1)
					case "span":
						return strings.Replace(body, "| "+next+" |", "| Run \x60"+next+"\x60 here |", 1)
					}
				}
				return body
			})
			descriptor := strings.Replace(f.descriptor, "Acceptance-Transition:", "Acceptance-Transition: link=1", 1)
			payload := strings.Replace(f.payload, "transition v1", "transition v2", 1)
			payload = strings.Replace(payload, "amendment=", "link=1\npredecessor="+f.delivery+"\namendment=", 1)
			payload = strings.Replace(payload, "rows=2", "rows=2:PlainCommand", 1)
			switch mode {
			case "legacy":
				descriptor, payload = f.descriptor, f.payload
			case "v1-link":
				payload = f.payload
			case "Command":
				payload = strings.Replace(payload, "2:PlainCommand", "2:Command", 1)
			case "missing":
				payload = strings.Replace(payload, "2:PlainCommand", "", 1)
			case "wrong-predecessor":
				payload = strings.Replace(payload, "predecessor="+f.delivery, "predecessor="+f.base, 1)
			}
			login, _ := HumanLogin("alex")
			f.forge.commentBody = rlCommentJSONWith(login, "User", rlRepo, rlIssue, payload, rlPosted, rlPosted)
			verify, evidence, err := briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			plan, why := protectedAcceptanceFor(verify, evidence, descriptor, f.path)
			if mode != "approved" {
				if why == "" {
					t.Fatal("accepted", mode)
				}
				return
			}
			if why != "" {
				t.Fatal(why)
			}
			row := briefVerifyRows(plan.Verify)[1]
			if row.Command != next || row.Expect != "exit 1" || plan.Revisions["2"] != f.amendment || plan.Changes != "2=Command" {
				t.Fatalf("unexpected protected plan: row=%+v revisions=%v changes=%q", row, plan.Revisions, plan.Changes)
			}
			vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
			vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
			record := filepath.Join(t.TempDir(), "acceptance.txt")
			if err := os.WriteFile(record, []byte(descriptor), 0600); err != nil {
				t.Fatal(err)
			}
			result, diagnostics := captureVerifyrun(t, []string{"--root", f.root, "--brief", f.path, "--acceptance-record", record, "--acceptance-forge"})
			if result.code != 0 {
				t.Fatalf("verifyrun exited %d: %s", result.code, diagnostics)
			}
			_, evidence, err = briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			witnesses := parseEvidenceRows(evidence)["2"]
			if len(witnesses) != 1 || witnessCommandOf(witnesses[0].Text) != next || !strings.Contains(witnesses[0].Text, "pass exit=1") {
				t.Fatalf("unexpected witness for the approved command: %+v", witnesses)
			}
		})
	}
}

func TestPlainTransitionCommandShape(t *testing.T) {
	for _, c := range []struct {
		cell string
		ok   bool
	}{
		{"go test ./...", true},
		{"out=$(mktemp) && cat < input > \"$out\"", true},
		{"test -n \"$name\" \\| grep '^[0-9]$'", true},
		{"Run \x60sh check.sh\x60 on the named host", false},
		{"sh check.sh # on the named host", false},
		{"sh check.sh <!-- context -->", false},
		{"sh check.sh <b>context</b>", false},
		{"sh check.sh &nbsp;", false},
		{"sh check.sh [note](elsewhere)", false},
		{"sh check.sh \u200b", false},
		{"sh check.sh \u202e", false},
		{"", false},
	} {
		row := verifyRow{Class: classCheck, Classed: true, Shell: "sh", Command: verifyCommand(c.cell), CommandCell: c.cell}
		if got := plainTransitionCommand(row); got != c.ok {
			t.Errorf("%q eligibility = %t, want %t", c.cell, got, c.ok)
		}
	}
}
