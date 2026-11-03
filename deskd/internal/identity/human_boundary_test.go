package identity

import (
	"os"
	"os/exec"
	"testing"
)

// regression: #3669
func TestDisabledPolicyRefusesTerminalAuthority(t *testing.T) {
	if os.Getenv("DESKD_TEST_POLICY_DISABLED") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDisabledPolicyRefusesTerminalAuthority$")
		cmd.Env = append(os.Environ(), "DESKD_TEST_POLICY_DISABLED=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("disabled terminal gate: %v\n%s", err, out)
		}
		return
	}
	g := &TerminalGate{Cell: func(string) (TerminalCell, bool) { panic("disabled policy consulted cell authority") }}
	for _, login := range []string{"", "operator", "other-user"} {
		if g.answererOn(login, "assay") {
			t.Fatalf("disabled terminal gate authorized %q", login)
		}
	}
}
