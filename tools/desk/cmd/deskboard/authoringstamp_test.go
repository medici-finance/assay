package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const listRun = "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"

// The `prs` stamp-sweep listing (#6855): an attested change missing its authored-* pair is
// listed with the record's own fix line; a stamped change and a change with no line are not.
func TestPRsListsAuthoringStampGap(t *testing.T) {
	installFakeGH(t)
	stubForgeList(t, func(repo string) (*deskkit.OpenChanges, error) {
		line := deskkit.WorkerAttestationLinePrefix + " " + repo + "#41 run=" + listRun + "\n"
		ch := func(n int, body string, labels ...string) deskkit.OpenChange {
			return deskkit.OpenChange{Number: n, Title: "a change", Body: body, State: "OPEN", Draft: true,
				Author: deskkit.Account{Login: "ada"}, CreatedAt: "2026-01-01T00:00:00Z", HeadSHA: "abc123",
				MergeStateStatus: "CLEAN", Labels: labels,
				Rollup: []deskkit.RollupNode{{Typename: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"}}}
		}
		return &deskkit.OpenChanges{Cap: prListLimit, Changes: []deskkit.OpenChange{
			ch(7, line, deskkit.DispatchedModelPrefix+"m", deskkit.DispatchedTierPrefix+"strong"),
			ch(8, line, deskkit.AuthoredModelPrefix+"m", deskkit.AuthoredTierPrefix+"strong"),
			ch(9, "no line here\n"),
		}}, nil
	})
	reads := 0
	prev := readWorkerRecordFn
	readWorkerRecordFn = func(_ deskkit.Forge, fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
		reads++
		return deskkit.WorkerReceipt{Issue: n, Binding: deskkit.WorkerBinding{Run: listRun, Repo: fr.Slug(),
			Kit: "worker", Model: "m", Tier: "strong"}}, nil
	}
	t.Cleanup(func() { readWorkerRecordFn = prev })

	rep, err := cmdPRs(Header{AsOf: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("prs: %v", err)
	}
	view := rep.value.(prsReport)
	if len(view.PRs) == 0 {
		t.Fatal("no rows")
	}
	repos := map[string]bool{}
	for _, r := range view.PRs {
		repos[r.Repo] = true
		switch r.Number {
		case 7:
			want := fmt.Sprintf("deskdispatch --stamp-only --repo %s --pr 7 --attestation 41 --model m --tier strong --kit worker", r.Repo)
			if r.AuthoringStamp != "missing" || r.StampOnly != want {
				t.Errorf("%s#7: authoringStamp=%q stampOnly=%q, want missing / %q", r.Repo, r.AuthoringStamp, r.StampOnly, want)
			}
		default:
			if r.AuthoringStamp != "" || r.StampOnly != "" {
				t.Errorf("%s#%d must not be listed: %q %q", r.Repo, r.Number, r.AuthoringStamp, r.StampOnly)
			}
		}
	}
	if reads != len(repos) {
		t.Errorf("records read = %d, want one per nominated change (%d)", reads, len(repos))
	}
	var buf bytes.Buffer
	rep.render(&buf)
	if !strings.Contains(buf.String(), "    stamp-only: deskdispatch --stamp-only") {
		t.Errorf("the table must print the fix line under the row:\n%s", buf.String())
	}
}

// A record that does not validate lists the change with no fix line: a STOP, never a guess.
func TestPRsStampGapRecordRefused(t *testing.T) {
	installFakeGH(t)
	stubForgeList(t, func(repo string) (*deskkit.OpenChanges, error) {
		return &deskkit.OpenChanges{Cap: prListLimit, Changes: []deskkit.OpenChange{{
			Number: 7, Title: "a change", State: "OPEN", Draft: true, Author: deskkit.Account{Login: "ada"},
			Body:      deskkit.WorkerAttestationLinePrefix + " " + repo + "#41 run=" + listRun + "\n",
			CreatedAt: "2026-01-01T00:00:00Z", HeadSHA: "abc123", MergeStateStatus: "CLEAN",
			Rollup: []deskkit.RollupNode{{Typename: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"}},
		}}}, nil
	})
	prev := readWorkerRecordFn
	readWorkerRecordFn = func(deskkit.Forge, deskkit.ForgeRepo, int) (deskkit.WorkerReceipt, error) {
		return deskkit.WorkerReceipt{}, deskkit.Refused("record was edited after creation")
	}
	t.Cleanup(func() { readWorkerRecordFn = prev })

	rep, err := cmdPRs(Header{AsOf: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("a bad record must not take the board down: %v", err)
	}
	for _, r := range rep.value.(prsReport).PRs {
		if !strings.HasPrefix(r.AuthoringStamp, "refused: ") || r.StampOnly != "" {
			t.Errorf("%s#%d: authoringStamp=%q stampOnly=%q, want refused and no fix line", r.Repo, r.Number, r.AuthoringStamp, r.StampOnly)
		}
	}
}
