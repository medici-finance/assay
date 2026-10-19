package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

func TestQualificationProbe(t *testing.T) {
	q := reviewtest.Read(t)
	switch q.Case {
	case "needs-review", "approved", "partial", "access-failure":
	default:
		t.Fatalf("unknown board fixture %q", q.Case)
	}
	reviewtest.Home(t, q, classDegradeRoster)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	var prs []prBase
	head := q.Head
	if head == "" {
		head = strings.Repeat("a", 40)
	}
	for i := 0; i < q.Size; i++ {
		p := greenRollupPR(i+1, head)
		p.MergeStateStatus = "CLEAN"
		p.Author.Login = "shared-agent"
		prs = append(prs, p)
	}
	b, err := json.Marshal(prs)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", string(b))
	if q.Case == "access-failure" {
		t.Setenv("DESKBOARD_GH_FAIL_REPO", "example-org/tracker")
	}
	stubForgeHooks(t, forgeHookSet{reviews: func(_ string, n int) ([]deskkit.Review, error) {
		if q.Case == "partial" && n == 1 {
			return nil, errors.New("fixture reviews unreadable")
		}
		if q.Case == "approved" {
			return []deskkit.Review{{Author: deskkit.Account{Login: "gl-reviewer", ID: 41987965}, State: "APPROVED", CommitID: head, Body: "Verdict: approve\nSecurity-Review: pass", SubmittedAt: "2026-09-14T16:00:00Z"}}, nil
		}
		return nil, nil
	}})
	part, err := sweepActionsRepo("example-org/tracker", nil, nil, nil, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	r := reviewtest.Response{Exit: deskkit.ExitCodeOf(err), Depth: map[string]int{}, Facts: map[string]string{"truncated": fmt.Sprint(part.truncated)}}
	degraded := 0
	if err == nil {
		heads, _, headErr := fetchOpenPRs("example-org/tracker")
		if headErr != nil {
			t.Fatal(headErr)
		}
		byNumber := map[int]string{}
		var headRows []map[string]any
		for _, p := range heads {
			byNumber[p.Number] = p.HeadRefOid
			headRows = append(headRows, map[string]any{"repo": "example-org/tracker", "number": p.Number, "headSHA": p.HeadRefOid})
		}
		for _, row := range part.rows {
			if strings.Contains(row.Note, "DEGRADED:") {
				degraded++
			}
			r.Rows = append(r.Rows, reviewtest.Row{ID: fmt.Sprintf("%s#%d", row.Repo, row.Number), Head: byNumber[row.Number], Action: row.Action})
			r.Depth[row.Action]++
		}
		// Preserve production scores and row order across the two-command join.
		r.Actions, err = json.Marshal(map[string]any{"asOf": "2026-10-17T12:00:00Z", "scope": map[string]any{"repos": []string{"example-org/tracker"}, "count": 1}, "prPopulation": map[string]any{"complete": !part.truncated, "cap": 100}, "rows": part.rows})
		if err != nil {
			t.Fatal(err)
		}
		r.PRs, err = json.Marshal(map[string]any{"prs": headRows})
		if err != nil {
			t.Fatal(err)
		}
	}
	r.Facts["headSource"] = "second production open-PR read; actions omits head"
	r.Facts["degradedRows"] = fmt.Sprint(degraded)
	reviewtest.Write(t, r)
}
