package deskkit

import (
	"strings"
	"testing"
)

// TestCheckOwnPR pins the shared own-PR checkout guard (#1901): a checkout is the PR's own
// when its branch IS the PR's head branch, or its HEAD commit IS the PR's head commit —
// exactly, never a descendant — and the PR is OPEN.
//
// FAIL-FIRST: ownpr-mutations.json — the CONTROL mutant drops the head-commit rule (the
// pre-#1901 branch-name-only guard) and the two accept-by-head rows go red; the other
// mutants loosen the equality and the refuse rows go red.
func TestCheckOwnPR(t *testing.T) {
	const (
		prHead  = "1111111111111111111111111111111111111111"
		other   = "2222222222222222222222222222222222222222"
		prRef   = "feature/pr-head"
		neutral = "neutral-rework"
	)
	open := func(ref, oid string) OwnPRRemote {
		return OwnPRRemote{Number: 7, State: "OPEN", HeadRef: ref, HeadOid: oid}
	}
	cases := []struct {
		name   string
		local  OwnPRLocal
		remote OwnPRRemote
		want   OwnPRMatch // "" = refused
	}{
		{"branch equal, same head", OwnPRLocal{prRef, prHead}, open(prRef, prHead), OwnPRByBranch},
		{"branch equal, head moved on (unpushed commit)", OwnPRLocal{prRef, other}, open(prRef, prHead), OwnPRByBranch},
		{"different branch, HEAD == headRefOid", OwnPRLocal{neutral, prHead}, open(prRef, prHead), OwnPRByHead},
		{"different branch, HEAD == headRefOid (upper-case hex)", OwnPRLocal{neutral, strings.ToUpper(prHead)}, open(prRef, prHead), OwnPRByHead},
		{"detached HEAD at headRefOid", OwnPRLocal{"", prHead}, open(prRef, prHead), OwnPRByHead},
		{"different branch, HEAD != headRefOid", OwnPRLocal{neutral, other}, open(prRef, prHead), ""},
		{"detached HEAD elsewhere", OwnPRLocal{"", other}, open(prRef, prHead), ""},
		{"different branch, HEAD is an abbreviation of headRefOid", OwnPRLocal{neutral, prHead[:12]}, open(prRef, prHead), ""},
		{"different branch, forge reported no head oid", OwnPRLocal{neutral, prHead}, open(prRef, ""), ""},
		{"different branch, no local head", OwnPRLocal{neutral, ""}, open(prRef, ""), ""},
		{"detached HEAD does not match an empty head ref", OwnPRLocal{"", other}, open("", prHead), ""},
		{"merged PR, branch equal", OwnPRLocal{prRef, prHead}, OwnPRRemote{7, "MERGED", prRef, prHead}, ""},
		{"merged PR, HEAD == headRefOid", OwnPRLocal{neutral, prHead}, OwnPRRemote{7, "MERGED", prRef, prHead}, ""},
		{"closed PR, HEAD == headRefOid", OwnPRLocal{neutral, prHead}, OwnPRRemote{7, "closed", prRef, prHead}, ""},
		{"closed PR, detached at headRefOid", OwnPRLocal{"", prHead}, OwnPRRemote{7, "CLOSED", prRef, prHead}, ""},
		{"open in lower case", OwnPRLocal{neutral, prHead}, OwnPRRemote{7, "open", prRef, prHead}, OwnPRByHead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckOwnPR("deskreply", tc.local, tc.remote)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("CheckOwnPR admitted the checkout (by %s); want a refusal", got)
				}
				if ExitCodeOf(err) != ExitRefused {
					t.Fatalf("refusal exit = %d, want %d (Refused): %v", ExitCodeOf(err), ExitRefused, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckOwnPR refused: %v; want admitted by %s", err, tc.want)
			}
			if got != tc.want {
				t.Fatalf("admitted by %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCheckOwnPRRefusalNamesBothSides: the refusal is the operator's only diagnosis, so it
// names the worktree's branch and commit, the PR's head branch and commit, the tool, and
// the push-first remedy for the descendant case.
func TestCheckOwnPRRefusalNamesBothSides(t *testing.T) {
	_, err := CheckOwnPR("deskpr edit",
		OwnPRLocal{Branch: "neutral-rework", Head: "2222222222222222222222222222222222222222"},
		OwnPRRemote{Number: 42, State: "OPEN", HeadRef: "feature/pr-head", HeadOid: "1111111111111111111111111111111111111111"})
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, want := range []string{`"neutral-rework"`, "222222222222", "PR #42", `"feature/pr-head"`, "111111111111", "deskpr edit", "push them"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not carry %q: %s", want, err)
		}
	}
	_, err = CheckOwnPR("deskreply", OwnPRLocal{Head: "2222222222222222222222222222222222222222"},
		OwnPRRemote{Number: 42, State: "OPEN", HeadRef: "feature/pr-head", HeadOid: "1111111111111111111111111111111111111111"})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("a detached-HEAD refusal should say so: %v", err)
	}
}
