package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

// regression: #5704
func TestBoundSubjectHold(t *testing.T) {
	boundEnv(t)
	_, err := verifyBinding(prRef{Repo: allowedRepo, N: 42}, "demo:b-07@2")
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("bound filing bypassed selected capability hold: %v", err)
	}
	_, err = verifyBinding(prRef{Repo: allowedRepo, N: 42}, "demo:m-07@2")
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("milestone entered bound brief lookup: %v", err)
	}
}
