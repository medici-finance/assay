//go:build parity

package main

import "testing"

// TestParityMutateHonoured is the positive half of the negative control: row 5 depends on
// a `-tags parity` build ACTUALLY dropping the named [plan] env line, so a regression that made
// the injector inert everywhere would quietly turn row 5 green-for-the-wrong-reason.
//
// This file names CELLCTL_PARITY_MUTATE and therefore carries `//go:build parity`, which is what
// row 14 requires of every file in the package that does.
func TestParityMutateHonoured(t *testing.T) {
	t.Setenv("CELLCTL_PARITY_MUTATE", "KUBECONFIG")
	if !parityDropsPlanEnv("KUBECONFIG=/dev/null") {
		t.Error("a -tags parity build must drop the named plan env line (row 5's negative control)")
	}
	if parityDropsPlanEnv("HOME=/x") {
		t.Error("only the NAMED key may be dropped")
	}
	t.Setenv("CELLCTL_PARITY_MUTATE", "")
	if parityDropsPlanEnv("KUBECONFIG=/dev/null") {
		t.Error("an unset injector must drop nothing, even in a parity build")
	}
}
