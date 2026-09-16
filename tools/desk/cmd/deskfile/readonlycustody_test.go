package main

// readonlycustody_test.go — `check` is a DRY RUN, so it must not spend a credential rotation.
//
// deskfile mints the session-role token before the forge resolver runs, and on the GitLab
// custody path that mint is a destructive self-rotation: the endpoint invalidates the token
// the caller presents as it issues the successor. `check` files nothing, yet it drove one
// rotation per invocation — so a window running several checks in parallel raced its own
// rotations, and the loser's 401 could leave the role's custody file holding a dead value that
// only a group owner can replace.
//
// These tests pin both halves of the fix: the dry-run verb ASKS for read-only custody, and
// that request actually reaches the minter as --no-rotate.

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCheckAsksForReadOnlyCustody — the dry-run verb requests a NON-rotating credential
// lookup, and the writing verbs do not. Asserted on the resolver's own argument rather than on
// a downstream side effect, because that argument IS the decision: only the caller knows
// whether it is about to write, and `check` reaches the forge by exactly the same path `new`
// does.
func TestCheckAsksForReadOnlyCustody(t *testing.T) {
	t.Run("check is read-only", func(t *testing.T) {
		withEnv(t)
		rec := curForge
		rc, out := runCapture([]string{"check", "-R", allowedRepo, "--title", "a unique substantive title here"})
		if rc != 0 {
			t.Fatalf("check rc = %d, want 0:\n%s", rc, out)
		}
		if !rec.readOnlyCustody {
			t.Fatal("check asked for a ROTATING credential mint — a dry run that files nothing must not " +
				"spend a rotation; on GitLab custody that rotation can revoke the role's live PAT")
		}
	})

	t.Run("new is not read-only", func(t *testing.T) {
		withEnv(t)
		rec := curForge
		body := bodyFileWith(t, "a body for the filing")
		rc, out := runCapture([]string{"new", "-R", allowedRepo,
			"--title", "another unique substantive title", "--body-file", body})
		if rc != 0 {
			t.Fatalf("new rc = %d, want 0:\n%s", rc, out)
		}
		if rec.readOnlyCustody {
			t.Fatal("new asked for READ-ONLY custody — a verb that writes must take the ordinary " +
				"(rotating) mint, or the fix would have quietly retired rotate-on-mint for filings")
		}
	})
}

// TestMintSessionTokenPassesNoRotateWhenReadOnly pins the plumbing under the boolean: the
// read-only request has to arrive at the minter as the flag that actually suppresses the
// rotation. A test that only checked the boolean would still pass if the flag were dropped on
// the way to the child process, which is precisely where the behaviour lives.
func TestMintSessionTokenPassesNoRotateWhenReadOnly(t *testing.T) {
	cases := []struct {
		name      string
		readOnly  bool
		wantFlag  bool
		rationale string
	}{
		{"read-only lookup", true, true, "a dry run must suppress the rotation"},
		{"writing mint", false, false, "a writing verb keeps rotate-on-mint"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			var got []string
			oldExec := execCommand
			execCommand = func(name string, args ...string) *exec.Cmd {
				if name == "desktoken" {
					got = append([]string{name}, args...)
				}
				// Route to a command that fails fast: mintSessionToken's error path is not
				// under test here, only the argv it builds.
				return oldExec("false")
			}
			t.Cleanup(func() { execCommand = oldExec })

			_ = mintSessionToken(allowedRepo, tc.readOnly)

			if got == nil {
				t.Fatal("mintSessionToken did not invoke desktoken")
			}
			line := strings.Join(got, " ")
			has := false
			for _, a := range got {
				if a == "--no-rotate" {
					has = true
				}
			}
			if has != tc.wantFlag {
				t.Fatalf("desktoken argv = %q; --no-rotate present = %v, want %v (%s)",
					line, has, tc.wantFlag, tc.rationale)
			}
			// --repo stays mandatory on both paths: an App installed on more than one account
			// mints for the wrong installation without it.
			if !strings.Contains(line, "--repo "+allowedRepo) {
				t.Fatalf("desktoken argv = %q, want --repo %s", line, allowedRepo)
			}
		})
	}
}
