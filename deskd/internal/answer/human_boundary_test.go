package answer

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/humanid"
)

// regression: #3669
func TestDisabledPolicyRefusesDirectSignedAndLivePaths(t *testing.T) {
	if os.Getenv("DESKD_TEST_POLICY_DISABLED") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDisabledPolicyRefusesDirectSignedAndLivePaths$")
		cmd.Env = append(os.Environ(), "DESKD_TEST_POLICY_DISABLED=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("disabled direct path: %v\n%s", err, out)
		}
		return
	}
	s := NewService(&fakeWriter{}, nil, nil, nil)
	s.SetAnswerAs("operator")
	if err := s.requireLive(&Session{live: true, Actor: "operator", ExpiresAt: time.Now().Add(time.Hour)}); err != humanid.ErrDisabled {
		t.Fatalf("direct live path: %v", err)
	}
	act := &SignedAct{verified: true, login: "operator", expires: time.Now().Add(time.Hour)}
	if err := s.requireAct(act, contract.Action("answer"), Ref{Repo: "example-org/repo", Number: 1}); err != humanid.ErrDisabled {
		t.Fatalf("direct signed path: %v", err)
	}
	w := NewPerRequestHumanWriter("http://invalid.example")
	if _, err := w.token(withSignedAct(context.Background(), act)); err != humanid.ErrDisabled {
		t.Fatalf("signed writer bypass: %v", err)
	}
}
