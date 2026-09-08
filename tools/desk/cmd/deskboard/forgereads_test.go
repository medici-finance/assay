package main

// forgereads_test.go — unit coverage for the peripheral reads that moved from the `gh` CLI
// onto the typed Forge seam (the forge-seam migration). Each asserts the deskboard-side mapping
// of one migrated call site: the transport changed, the read's meaning did not.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestFetchRecentCommits_MapsShas(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{recentCommits: commitsOf("s1", "s2", "s3")})
	shas, empty, err := fetchRecentCommits("example-org/tracker")
	if err != nil || empty {
		t.Fatalf("fetchRecentCommits err=%v empty=%v, want nil/false", err, empty)
	}
	if len(shas) != 3 || shas[0] != "s1" || shas[2] != "s3" {
		t.Fatalf("shas = %v, want [s1 s2 s3]", shas)
	}
}

func TestFetchRecentCommits_EmptyRepoIsKnown(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		recentCommits: func(string, int) ([]deskkit.RepoCommit, error) {
			return nil, &deskkit.ForgeAPIError{Status: 409, Method: "GET", Path: "/commits"}
		},
	})
	shas, empty, err := fetchRecentCommits("example-org/proposals")
	if err != nil {
		t.Fatalf("an empty-repo signal must not be a read error: %v", err)
	}
	if !empty || len(shas) != 0 {
		t.Fatalf("empty=%v shas=%v, want empty=true, no shas", empty, shas)
	}
}

func TestFetchHeadCommit_MapsDateAndCommitterLogin(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		getCommit: func(string, string) (*deskkit.RepoCommit, error) {
			return &deskkit.RepoCommit{
				SHA: "abc", CommittedDate: "2026-09-01T10:00:00Z",
				CommitterLogin: "pusher", AuthorLogin: "author",
			}, nil
		},
	})
	when, by, err := fetchHeadCommit("example-org/tracker", "abc")
	if err != nil {
		t.Fatalf("fetchHeadCommit: %v", err)
	}
	if when.IsZero() {
		t.Fatal("committed date was not parsed")
	}
	if by != "pusher" {
		t.Fatalf("by = %q, want pusher (the committer login wins)", by)
	}
}

func TestFetchHeadCommit_FallsBackToAuthorLogin(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		getCommit: func(string, string) (*deskkit.RepoCommit, error) {
			return &deskkit.RepoCommit{SHA: "abc", CommittedDate: "2026-09-01T10:00:00Z", AuthorLogin: "author"}, nil
		},
	})
	_, by, err := fetchHeadCommit("example-org/tracker", "abc")
	if err != nil {
		t.Fatalf("fetchHeadCommit: %v", err)
	}
	if by != "author" {
		t.Fatalf("by = %q, want author (no committer login → author fallback)", by)
	}
}

func TestFetchHeadCommit_EmptyAttributionIsUnknown(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		getCommit: func(string, string) (*deskkit.RepoCommit, error) {
			return &deskkit.RepoCommit{SHA: "abc", CommittedDate: "2026-09-01T10:00:00Z"}, nil
		},
	})
	_, by, err := fetchHeadCommit("example-org/tracker", "abc")
	if err != nil {
		t.Fatalf("fetchHeadCommit: %v", err)
	}
	if by != "" {
		t.Fatalf("by = %q, want empty — unresolved attribution is UNKNOWN, never a guessed login", by)
	}
}

func TestSearchOpenPRs_MapsResultRows(t *testing.T) {
	installFakeForge(t)
	t.Setenv("DESKBOARD_GH_SEARCH_JSON",
		`[{"number":9,"title":"open pr","createdAt":"2026-01-01T00:00:00Z","repository":{"nameWithOwner":"example-org/other"}}]`)
	rows, err := searchOpenPRs("example-org")
	if err != nil {
		t.Fatalf("searchOpenPRs: %v", err)
	}
	if len(rows) != 1 || rows[0].Number != 9 || rows[0].Repository.NameWithOwner != "example-org/other" {
		t.Fatalf("rows = %+v, want one row #9 in example-org/other", rows)
	}
}

func TestListWorkflowFiles_NotFoundIsKnownAnswer(t *testing.T) {
	installFakeForge(t) // no DESKBOARD_GH_WORKFLOWS_DIR_JSON → the fake answers 404
	_, err := listWorkflowFiles("example-org/tracker", "abc")
	if err == nil {
		t.Fatal("a repo with no .github/workflows must return a not-found error, not an empty list")
	}
	if !isNotFound(err) {
		t.Fatalf("err = %v, want an isNotFound (404) the zero-CI probe reads as a checked zero", err)
	}
}

func TestFetchCombinedStatusTotal_ReadsChecksAtHeadTotal(t *testing.T) {
	installFakeForge(t)
	t.Setenv("DESKBOARD_GH_COMBINED_STATUS_JSON", `{"total_count":3,"statuses":[]}`)
	total, err := fetchCombinedStatusTotal("example-org/tracker", "abc")
	if err != nil {
		t.Fatalf("fetchCombinedStatusTotal: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (ChecksAtHead.StatusTotalCount)", total)
	}
}

func TestChangedFilesBetween_MapsCompareFiles(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			return &deskkit.RefComparison{Status: "behind", Files: []deskkit.ChangedFile{{Filename: "a.go"}, {Filename: "b.go"}}}, nil
		},
	})
	set, err := changedFilesBetween("example-org/tracker", "main", "abc")
	if err != nil {
		t.Fatalf("changedFilesBetween: %v", err)
	}
	if !set["a.go"] || !set["b.go"] || len(set) != 2 {
		t.Fatalf("set = %v, want {a.go, b.go}", set)
	}
}

func TestFetchPRState_MergedFlagWithoutTimestamp(t *testing.T) {
	installFakeForge(t)
	t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"closed","merged":true}`)
	state, merged, detail := fetchPRState("example-org/tracker", 1580)
	if state != prStateMerged || merged == nil || !*merged {
		t.Fatalf("state=%q merged=%v — merged:true with no merged_at is still MERGED (#400 N1); detail=%s", state, merged, detail)
	}
}
