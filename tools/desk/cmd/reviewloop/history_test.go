package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func planFixture(t *testing.T, count int) []string {
	t.Helper()
	var actions, prs []string
	for i := 0; i < count; i++ {
		action := "NEEDS-REVIEW"
		if i%2 == 1 {
			action = "FLIP"
		}
		actions = append(actions, fmt.Sprintf(`{"repo":"example-org/example-repo","number":%d,"action":%q}`, 42+i, action))
		prs = append(prs, fmt.Sprintf(`{"repo":"example-org/example-repo","number":%d,"headSHA":"aaaaaaaaaaaabbbbbbbbbbbb"}`, 42+i))
	}
	a := fmt.Sprintf(`{"asOf":%q,"scope":{"repos":["example-org/example-repo"],"count":1},"prPopulation":{"complete":true},"rows":[%s]}`, fixedNow, strings.Join(actions, ","))
	return []string{"--actions", writeTemp(t, "a.json", a), "--prs", writeTemp(t, "p.json", `{"prs":[`+strings.Join(prs, ",")+`]}`), "--now", fixedNow}
}

// regression: #2793
func TestPlanHistoryIndependent(t *testing.T) {
	// These records explain why a completed-write predicate is not a dispatch
	// predicate. Exercise its exact semantics as a positive control, then assert
	// the board's current operation survives coalescing. The binary dependency
	// test separately proves that cmdPlan cannot consult global history.
	for _, result := range []string{"ok", "noop", "dryrun", "refused", "unverifiable"} {
		for _, verb := range []string{"review", "review:correctness:approve", "review:security:approve", "review:correctness:request-changes", "ready"} {
			t.Run(result+"/"+verb, func(t *testing.T) {
				for _, action := range []string{"NEEDS-REVIEW", "RE-REVIEW", "FLIP"} {
					r := headed(t, action, 42, "aaaaaaaaaaaabbbbbbbbbbbb")
					entry := deskkit.Entry{Tool: "deskpost", Repo: r.Repo, PR: &r.Number, HeadSHA: &r.Head, Verb: verb, Result: result, BodyDigest: "different-review-body"}
					entries := []deskkit.Entry{entry}
					completed := result == "ok" || result == "noop"
					if got := deskkit.AlreadyDoneIn(entries, r.Repo, r.Number, r.Head, verb); got != completed {
						t.Fatalf("producer control: exact completed-write match = %v, want %v", got, completed)
					}
					if got := deskkit.AlreadyDoneIn(entries, r.Repo, r.Number, r.Head, r.Rule.Verb); got != (completed && verb == r.Rule.Verb) {
						t.Fatalf("producer control: operation key unexpectedly matches %q", verb)
					}
					got := Dispatched(Coalesce([]*Board{healthyBoard(t, r)}))
					if len(got) != 1 || got[0].Key.Verb != r.Rule.Verb {
						t.Fatalf("current %s operation hidden by historical %s %s", action, verb, result)
					}
				}
			})
		}
	}
	for _, count := range []int{0, 1, 33} {
		args := planFixture(t, count)
		var normal, dry strings.Builder
		if err := cmdPlan(args, &normal); err != nil {
			t.Fatal(err)
		}
		if err := cmdPlan(append(args, "--dry-run"), &dry); err != nil {
			t.Fatal(err)
		}
		if normal.String() != dry.String() {
			t.Fatalf("%d keys: dry-run changed plan", count)
		}
		if got := strings.Count(normal.String(), "  emit"); got != count {
			t.Fatalf("%d keys: emitted %d", count, got)
		}
	}
}
