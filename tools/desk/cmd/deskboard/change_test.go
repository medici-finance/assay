package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestChangeCommandUsesRequestedSubject(t *testing.T) {
	installFakeGH(t)
	const repo = "example-org/tracker"
	head := strings.Repeat("a", 40)
	subjects, histories, rollups := 0, 0, 0
	forgeHooks.getPR = func(got string, n int) (*deskkit.PullRequest, error) {
		t.Helper()
		if got != repo || n != 7 {
			t.Fatalf("wrong change %s#%d", got, n)
		}
		subjects++
		return &deskkit.PullRequest{Number: 7, State: "open", HeadSHA: head}, nil
	}
	forgeHooks.reviews = func(got string, n int) ([]deskkit.Review, error) {
		if got != repo || n != 7 {
			t.Fatalf("wrong reviews %s#%d", got, n)
		}
		histories++
		return []deskkit.Review{{ID: 81, Body: "Retain this finding.", CommitID: head}}, nil
	}
	forgeHooks.checks = func(got, sha string) (*deskkit.ChecksAtHead, error) {
		if got != repo || sha != head {
			t.Fatalf("wrong checks %s@%s", got, sha)
		}
		rollups++
		return &deskkit.ChecksAtHead{}, nil
	}
	var out, errout bytes.Buffer
	code := run([]string{"change", repo, "7", head, "--json"}, &out, &errout)
	var result deskkit.ChangeInspection
	if code != 0 || json.Unmarshal(out.Bytes(), &result) != nil || result.Head != head || len(result.Reviews) != 1 || result.Reviews[0].Body != "Retain this finding." || result.CheckRead != "empty" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	if subjects != 1 || histories != 1 || rollups != 1 {
		t.Fatalf("subject/history/rollup reads: %d/%d/%d", subjects, histories, rollups)
	}
	out.Reset()
	code = run([]string{"change", "outside/repo", "7", "--json"}, &out, &errout)
	if code != deskkit.ExitRefused || subjects != 1 {
		t.Fatalf("unadmitted subject reached the forge: %d, %d", code, subjects)
	}
}

// The inventory exercises each verb with an ordinary successful subject.
func inspectionSubjectFixture(t *testing.T) {
	t.Helper()
	forgeHooks.getPR = func(_ string, number int) (*deskkit.PullRequest, error) {
		return &deskkit.PullRequest{Number: number, State: "open", HeadSHA: strings.Repeat("a", 40)}, nil
	}
}
