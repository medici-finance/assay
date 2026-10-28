package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: class verification-handoff
func TestHandoffScanner(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-805.md": closedPlaceholderFile("alpha", 805, "bug"),
	})
	archive := filepath.Join(root, "docs/streams/alpha/done")
	if err := os.MkdirAll(archive, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "issue-806.md"), []byte(closedPlaceholderFile("alpha", 806, "bug")), 0600); err != nil {
		t.Fatal(err)
	}
	issues := []ghIssue{
		{Number: 801, Labels: lbl("bug", "raised-by:verifier")},
		{Number: 802, Labels: lbl("bug", "to:worker")},
		{Number: 803, Labels: lbl("bug", "to:worker", "to:intake")},
		{Number: 804, Labels: lbl("bug", "to:")},
		{Number: 805, Labels: lbl("bug", "to:worker")},
		{Number: 806, Labels: lbl("bug", "to:worker")},
		{Number: 807, Labels: lbl("bug", "needs-decision")},
	}
	plans, closes, notices, skipped := planScan(root, streams, fixtureLister(map[string][]ghIssue{scanHomeRepo(): issues}, ""), blessAll, labelHoldClearAll)
	if len(plans) != 1 || plans[0].Issue != 801 || len(closes) != 1 || closes[0].Issue != 805 || closes[0].Action != "sweep" || len(skipped) != 0 {
		t.Errorf("addressed scanner policy: plans=%+v closes=%+v skipped=%+v", plans, closes, skipped)
	}
	if len(notices) != 5 {
		t.Fatalf("missing addressed/unknown holds: %v", notices)
	}
	t.Run("routing does not override retirement", func(t *testing.T) {
		for _, address := range [][]string{{"to:worker"}, {"to:"}, {"to:worker", "to:intake"}} {
			for _, status := range []string{"todo", "done"} {
				for _, excluded := range []string{"", "question", "verify-gate", "live-verify", "needs-decision", "review-request", "needs-human"} {
					t.Run(strings.Join(address, "+")+"/"+status+"/"+excluded, func(t *testing.T) {
						body := strings.Replace(closedPlaceholderFile("alpha", 808, "bug"), "status: done", "status: "+status, 1)
						streams, root := scanFixtureRepo(t, map[string]string{"issue-808.md": body})
						labels := append([]string{"bug", excluded}, address...)
						plans, closes, _, skipped := planScan(root, streams, fixtureLister(map[string][]ghIssue{scanHomeRepo(): {{Number: 808, Labels: lbl(labels...)}}}, ""), blessAll, labelHoldClearAll)
						want := ""
						if status == "done" {
							want = "sweep"
						} else if excluded != "" && excluded != "question" {
							want = "retire-label"
						}
						if len(plans) != 0 || len(skipped) != 0 {
							t.Fatalf("addressed retirement created work or skipped: %v %v", plans, skipped)
						}
						if want == "" {
							if len(closes) != 0 {
								t.Fatalf("ordinary addressed work changed: %v", closes)
							}
						} else if len(closes) != 1 || closes[0].Action != want || closes[0].Dest != archivedPath(filepath.Join(root, "docs/streams/alpha/issue-808.md")) {
							t.Fatalf("addressed retirement: want %s with archive destination; got %+v", want, closes)
						}
					})
				}
			}
		}
	})
	_, closes, _, skipped = planScan(root, streams, fixtureLister(nil, scanHomeRepo()), blessAll, labelHoldClearAll)
	if len(skipped) != 1 || len(closes) != 0 {
		t.Fatalf("unreadable observation became empty: %v %v", skipped, closes)
	}
}
