package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #7308
func TestPlainCommandApproval(t *testing.T) {
	const spanRule = "changed commands require one unambiguous code span and identical surrounding context"
	const otherCells = "plain-shell replacement must retain every other original row cell"
	const exactApproval = "human approval does not name the exact transition payload"
	for _, tc := range []struct{ mode, refusal string }{
		{"approved", ""},
		{"legacy", spanRule},
		{"v1-link", "plain-shell replacement requires an explicit v2 PlainCommand approval"},
		{"Command", exactApproval},
		{"missing", "human approval omits a changed original command: row 2"},
		{"wrong-predecessor", exactApproval},
		{"expect", exactApproval},
		{"class", spanRule},
		{"shell", spanRule},
		{"pwsh", spanRule},
		{"cmd", spanRule},
		{"prose-led", spanRule},
		{"span-to-plain", spanRule},
		{"obligation", otherCells},
		{"environment", otherCells},
		{"extra-column", otherCells},
		{"delivery-environment", otherCells},
		{"restored-environment", otherCells},
		{"span", spanRule},
		{"gate", spanRule},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			mode := tc.mode
			original, next := "sh check.sh", "out=$(mktemp) && trap 'rm -f \"$out\"' 0 && sh check.sh --no-pull > \"$out\""
			class, shell := "check +flow", "sh"
			switch mode {
			case "pwsh", "cmd":
				shell = mode
			case "prose-led":
				original, next = ": prior description", ": amended description"
			case "span-to-plain":
				original = "`sh ./check.sh`"
			case "gate":
				class = "gate:human"
			}
			f := contextFixture(t, []bool{true}, func(stage, body string) string {
				switch stage {
				case "original":
					body = strings.Replace(body, "| Expect |", "| Expect | Class | Shell | Env | Note |", 1)
					return strings.Replace(body, "| 2 | `sh check.sh` | exit 1 |", "| 2 | "+original+" | exit 1 | "+class+" | "+shell+" | repo | contract A |", 1)
				case "delivery":
					if mode == "delivery-environment" || mode == "restored-environment" {
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
					case "restored-environment":
						return strings.Replace(body, "| host |", "| repo |", 1)
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
			if tc.refusal != "" {
				if !strings.Contains(why, tc.refusal) {
					t.Fatalf("got refusal %q, want %q", why, tc.refusal)
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
		{": description only", false},
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
		rows := briefVerifyRows("| # | Class | Command | Expect |\n|---|---|---|---|\n| 1 | check | " + c.cell + " | exit 0 |")
		if len(rows) != 1 {
			t.Fatalf("parsed %d rows for %q, want one", len(rows), c.cell)
		}
		if got := plainTransitionCommand(rows[0]); got != c.ok {
			t.Errorf("%q eligibility = %t, want %t", c.cell, got, c.ok)
		}
	}
}
