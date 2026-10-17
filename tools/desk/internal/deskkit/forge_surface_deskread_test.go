package deskkit

import (
	"reflect"
	"sort"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/forgeban"
)

// forge_surface_deskread_test.go — the statusgen-off-`gh` work's forge-surface regression.
//
// `deskread` (tools/desk/cmd/deskread) is a NEW BINARY on the seam, reached by RUNNING a
// process — never a new method on the Forge interface. Every read its statusgen consumer needs
// was already enumerated with both backends before this work landed (ListOpenIssues and its
// siblings), so nothing here should ever need to grow the interface. This test pins that two
// independent ways:
//
//  1. The interface's method SET, by name. Adding, removing or renaming a method changes this
//     list — the freeze rule (spec §6) says an added op needs a consuming call site in the SAME
//     change, which is a legitimate reason for this list to change on some OTHER piece of work;
//     it is never legitimate for deskread's own migrations to be that reason, because deskread's
//     whole argument for existing is that it consumes the surface rather than widening it.
//  2. The shell-exec ban's ceiling (tools/desk/internal/forgeban), which only ever drops when a
//     forge-CLI call site is migrated and locked in. deskread and its statusgen consumer do not
//     touch tools/desk/internal/forgeban/allowlist.go at all (statusgen is a separate module
//     with no gh-shellout permit rows of its own — the register counts desk-tools call sites),
//     so a change to the ceiling from THIS work is itself the failure this row exists to catch.
//
// The ceiling asserted here is the value at this work's OWN base — not any number an authoring
// document may have cited when it was written — because the ceiling has moved since then from
// unrelated desk-tools migrations (deskflip/deskreply, per allowlist.go's own header), which is
// expected drift this test is not about. What it guards is narrower and still exact: that
// NOTHING in this diff moves it.
func TestForgeSurfaceUnchangedByDeskread(t *testing.T) {
	typ := reflect.TypeOf((*Forge)(nil)).Elem()
	got := make([]string, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		got[i] = typ.Method(i).Name
	}
	sort.Strings(got)

	want := []string{
		"ApplyLabels", "ApproveGate", "ChangeDiff", "ChecksAtHead", "CloseIssue", "CloseIssueTyped",
		"CompareRefs", "CreateDraftChange", "DeleteRef", "EditChange", "EditComment",
		"FileIssue", "GetCommit", "GetIssue", "GetIssueTyped", "GetPullRequest",
		"IssueContentEvents", "IssueReactions", "IssueTrustEvents", "ListChangedFiles",
		"ListChanges", "ListComments", "ListCommitChanges", "ListFileCommits",
		"ListCommentsTyped", "ListIssueLabelEvents", "ListLabelEvents", "ListLabels", "ListOpenChanges",
		"ListOpenIssues", "ListRecentCommits", "ListWorkflowFiles", "MarkReadyForReview",
		"MatchingRefs",
		"OpenChangeForBranch", "OpenMergeHold", "PRTrustEvents", "PostComment",
		"PostCommentTyped", "PostReview", "PushTransportHint", "ReadFile", "ReadMergeHold",
		"RefExists", "ReopenIssue", "RepoHardeningRead", "RepoVisibility",
		"RequiredStatusChecks", "ReviewQueueSnapshot", "ReviewsAtHead", "RunStatus", "RunWorkflow", "SearchIssues", "SearchOpenChanges",
		"SetMergeHold", "WriteFile",
		// Ops 55-58, added under the freeze rule by forge-neutral brief 33 with deskread kinds as consumers.
		"ListIssues", "IssueStateEvents", "ListChangeCommits", "RepoDefaultBranch",
		// Ops 62-63, added under the freeze rule by forge-neutral brief 17 with deskrun log and retry as consumers.
		"RunLog", "RetryRun",
	}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Forge interface method set changed (%d methods, want %d).\n"+
			"got:  %v\nwant: %v\n"+
			"If this diff added a method to support the deskread verb, that is the exact widening "+
			"the ground rule forbids ('add no operation to Forge'). If some OTHER change legitimately "+
			"grew the surface, this test's `want` list needs updating in THAT change, not here.",
			len(got), len(want), got, want)
	}

	// forgeban's ceiling at this work's own base (freshness-checked against this branch before
	// any statusgen/deskread work landed). statusgen carries no allowlist rows of its own to
	// migrate off, so this diff must never move it.
	//
	// Re-based 5→6 by the credfence balancing-loops work: its preflight ambient-identity
	// check legitimately ADDED one forge-CLI permit (internal/deskkit/preflight.go's
	// ambientLoginProbe reads `gh api user` to learn the ambient identity — the D2 identity
	// layer that cannot route through the token-minting interface; #1528). That is a real,
	// reviewed widening of the permit list, not the silent deskread drift this row guards
	// against, so the base moves with it — exactly the "re-base in whatever change legitimately
	// moved it" this test's own message names.
	//
	// Re-based 6→7 by the deskapps manifest-flow installer (#1260): its driver-ruled permit
	// cmd/deskapps/identity.go::runGH::gh reads the signed-in `gh auth` login before any App
	// token can exist. 7 = the #1528 ambientLoginProbe row plus the #1260 runGH row on top of
	// the five pre-existing permits; the same two rows forgeban's allowedInvocationCeiling cites.
	//
	// Re-based 7→6 by desktools-v2/03 (#2025, under the read-path ruling on #1911): deskmerge's
	// reads moved onto a minted App token — minted for the repository's installation and valid for
	// all of it — and its cmd/deskmerge/exec.go::runGH::gh permit was REMOVED with the
	// reach-around — a narrowing of the permit list, re-based in the change that made it, as this
	// test's own message asks.
	const baseCeiling = 6
	if c := forgeban.Ceiling(); c != baseCeiling {
		t.Fatalf("forgeban.Ceiling() = %d, want %d — this diff must not move the shell-exec ban's "+
			"ceiling (statusgen is a separate module with no allowlist rows to migrate here; any "+
			"change to this ceiling came from outside this work's own scope and this constant needs "+
			"re-basing in whatever change legitimately moved it, not silently accepted here)",
			c, baseCeiling)
	}
}
