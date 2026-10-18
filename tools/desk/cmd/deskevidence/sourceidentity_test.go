package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #3232
func TestEvidenceSourceIdentity(t *testing.T) {
	for _, lane := range []string{"Evidence", "outcome"} {
		t.Run(lane, func(t *testing.T) {
			for _, merged := range []bool{true, false} {
				t.Run(map[bool]string{true: "merged source", false: "unpublished foreign source"}[merged], func(t *testing.T) {
					f, _ := setupFake(t)
					useRealPublishIdentityGate(t)
					root := gitRoot(t, "Another App", issueLoopBotEmail)
					git := func(args ...string) string {
						t.Helper()
						c := exec.Command("git", args...)
						c.Dir = root
						b, e := c.CombinedOutput()
						if e != nil {
							t.Fatalf("git %v: %v %s", args, e, b)
						}
						return strings.TrimSpace(string(b))
					}
					head := git("rev-parse", "HEAD")
					old := git("rev-parse", "refs/remotes/origin/main")
					git("update-ref", "refs/remotes/origin/evidence", old)
					if merged {
						git("update-ref", "refs/remotes/origin/main", head)
					}
					verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
						r := stubAdmittedReceipt(target)
						r.Binding.Source = head
						return r, nil
					}
					f.setFile(evidenceBrief, "# Brief\n\n## Evidence\n")
					args := []string{"example-org/tracker", "evidence", "--evidence-file", evidenceBrief, "--root", root}
					if lane == "outcome" {
						line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
						record := writeRepoFile(t, "record.json", line+"\n")
						args = []string{"example-org/tracker", "evidence", "--outcome-record", record, "--root", root}
					}
					code := run(args)
					if merged {
						if code != deskkit.ExitOK || f.putCalls != 1 {
							t.Fatalf("merged immutable source refused: rc=%d writes=%d", code, f.putCalls)
						}
						if len(f.writes) != 1 || f.writes[0].Branch != "evidence" {
							t.Fatalf("conditional destination binding lost: %+v", f.writes)
						}
					} else if code != deskkit.ExitRefused || f.putCalls != 0 {
						t.Fatalf("new foreign source accepted: rc=%d writes=%d", code, f.putCalls)
					}
					if git("rev-parse", "HEAD") != head {
						t.Fatal("immutable source moved")
					}
					if git("rev-parse", "refs/remotes/origin/evidence") != old {
						t.Fatal("source commits were published")
					}
				})
			}
		})
	}
}

func TestEvidenceIdentityBinding(t *testing.T) {
	setupFake(t)
	var got deskkit.PublishIdentityInput
	publishIdentityGateFn = func(in deskkit.PublishIdentityInput) error { got = in; return nil }
	r := deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Source: strings.Repeat("a", 40)}}
	if err := publishEvidenceIdentityGate("fixture", r); err != nil {
		t.Fatal(err)
	}
	if got.Source != r.Binding.Source || got.Base != "main" || got.Role != "verifier" || got.RemoteTip != "" {
		t.Fatalf("source boundary changed: %+v", got)
	}
	if err := publishEvidenceIdentityGate("fixture", deskkit.VerifierReceipt{}); err == nil {
		t.Fatal("missing admission source accepted")
	}
}
