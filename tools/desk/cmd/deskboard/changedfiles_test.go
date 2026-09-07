package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// The risk-class gate is only as good as the changed-file list it reads. These are the
// three ways that input used to lie:
//
//  1. `gh pr view --json files` is files(first: 100) with NO pagination and no signal
//     that it truncated. On #1618 (changedFiles=652) it returned an ALPHABETICAL
//     window ending inside docs/, so every k8s/… and services/… path was invisible.
//  2. A short read was indistinguishable from a complete one — no reconciliation
//     against GitHub's own changed_files, the cross-check evalCI already does for CI.
//  3. A renamed file was reported under its NEW name only, so `git mv` OUT of a
//     security directory silently de-risk-classed the change.
// ---------------------------------------------------------------------------

const cfRepo = "example-org/tracker"

// actionsWith runs `actions` over one approved-at-head draft PR with the given file
// fixture and PR metadata, and returns its row.
func actionsWith(t *testing.T, prFilesJSON, prMetaJSON string) actionRow {
	t.Helper()
	head := "deadbeefcafe"
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", cfRepo)
	// The PR carries an Issue: link trailer: real App-authored PRs always carry one (deskpr
	// makes it mandatory), and a trailer keeps the #587 trailer-absent-App risk term silent so
	// these cases isolate the PATH classification they are about. Issue: (not Brief:) so the
	// owning-brief risk term stays silent too — there is no brief to resolve here.
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"t","body":"Issue: #123","isDraft":true,"author":{"login":"app/assay-worker-app"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":`+jsonStr("looks good")+`,"submitted_at":"2026-07-10T00:00:00Z"}]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", prFilesJSON)
	t.Setenv("DESKBOARD_GH_PRMETA_JSON", prMetaJSON)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, r := range rep.Rows {
		if r.Number == 42 {
			return r
		}
	}
	t.Fatal("PR #42 missing from the actions report")
	return actionRow{}
}

// TestTruncatedDiffIsRiskClassed — GitHub says 652 files, the read produced 1. The
// classification cannot be trusted, so the PR degrades to risk-classed.
func TestTruncatedDiffIsRiskClassed(t *testing.T) {
	row := actionsWith(t, `[{"filename":"README.md"}]`, `{"changed_files":652}`)
	if !row.RiskClassed {
		t.Fatal("a TRUNCATED changed-file read must be risk-classed — the trigger we did not see is the point")
	}
	if row.Action != actSecReview {
		t.Fatalf("action = %s, want %s", row.Action, actSecReview)
	}
}

// TestCompleteCleanDiffIsNotRiskClassed — the control: a complete, clean read still
// classifies clean. The fail-closed rule must not swallow every PR.
func TestCompleteCleanDiffIsNotRiskClassed(t *testing.T) {
	row := actionsWith(t, `[{"filename":"README.md"}]`, `{"changed_files":1}`)
	if row.RiskClassed {
		t.Fatal("a complete, clean diff must not be risk-classed")
	}
}

// TestTrailerAbsentAppRowSaysWhy is the #587 deskboard proof: an approved-at-head, CI-green,
// clean-PATH draft PR authored by a role App but carrying NO link trailer is risk-classed and
// surfaces SECURITY-REVIEW-REQUIRED whose note NAMES the reason. Before #587 the same row read
// as a clean FLIP candidate — the exact hole this closes. The only difference from the clean
// control above is the missing trailer, so this isolates the trailer-absent-App term.
func TestTrailerAbsentAppRowSaysWhy(t *testing.T) {
	head := "deadbeefcafe"
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", cfRepo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"t","body":"a change with no link trailer at all","isDraft":true,"author":{"login":"app/assay-worker-app"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":`+jsonStr("looks good")+`,"submitted_at":"2026-07-10T00:00:00Z"}]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`) // a clean, non-risky path
	t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	var row actionRow
	for _, r := range rep.Rows {
		if r.Number == 42 {
			row = r
		}
	}
	if !row.RiskClassed {
		t.Fatal("a trailer-less App-authored PR must be risk-classed (#587)")
	}
	if row.Action != actSecReview {
		t.Fatalf("action = %s, want %s (a risk-classed PR with no Security-Review pass)", row.Action, actSecReview)
	}
	if !strings.Contains(row.Note, "trailer absent on App-authored PR") {
		t.Fatalf("the SECURITY-REVIEW-REQUIRED row must SAY WHY; note = %q", row.Note)
	}
}

// TestRenameOutOfSecurityPathIsRiskClassed — `git mv secrets/auth/token.go
// config/authz/token.go`. GitHub reports only
// the NEW path, which matches nothing; previous_filename is what makes it visible.
func TestRenameOutOfSecurityPathIsRiskClassed(t *testing.T) {
	row := actionsWith(t,
		`[{"filename":"config/authz/token.go","previous_filename":"secrets/auth/token.go","status":"renamed"}]`,
		`{"changed_files":1}`)
	if !row.RiskClassed {
		t.Fatal("a rename OUT of a security path must stay risk-classed — one git mv would otherwise waive the gate")
	}
}

// TestRenameIntoSecurityPathIsRiskClassed — the mirror: the NEW path is the trigger.
func TestRenameIntoSecurityPathIsRiskClassed(t *testing.T) {
	row := actionsWith(t,
		`[{"filename":"secrets/new.key","previous_filename":"docs/New.md","status":"renamed"}]`,
		`{"changed_files":1}`)
	if !row.RiskClassed {
		t.Fatal("a rename INTO a security path must be risk-classed")
	}
}

// TestFetchChangedFilesPaginatesAndReconciles — the fetcher itself: no changed_files
// (an API that does not report one) disables the cross-check rather than faking it.
func TestFetchChangedFilesPaginatesAndReconciles(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON",
		`[{"filename":"a.go"},{"filename":"b/c.go","previous_filename":"b/old.go","status":"renamed"}]`)

	t.Run("no changed_files reported -> complete", func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{}`)
		files, complete, err := fetchChangedFiles(cfRepo, 42)
		if err != nil {
			t.Fatal(err)
		}
		if !complete {
			t.Fatal("no changed_files in the payload must disable the cross-check, not fail it")
		}
		for _, want := range []string{"a.go", "b/c.go", "b/old.go"} {
			if !files[want] {
				t.Fatalf("changed-file set %v is missing %q", files, want)
			}
		}
	})

	t.Run("short read -> incomplete", func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":9}`)
		_, complete, err := fetchChangedFiles(cfRepo, 42)
		if err != nil {
			t.Fatal(err)
		}
		if complete {
			t.Fatal("2 entries read against a reported 9 must report incomplete")
		}
	})
}
