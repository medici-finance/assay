package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// The board owns presentation of the collected provider facts. The shared
// inspection never needs to link the board's review reducer into the forge hub.
type changeCheck struct {
	deskkit.InspectedCheck
	Result string `json:"result"`
}

func changeCheckResults(facts []deskkit.InspectedCheck) []changeCheck {
	if facts == nil {
		return nil
	}
	rows := make([]changeCheck, 0, len(facts))
	for _, c := range facts {
		pass, pending, fail, unknown := reviewflow.CountChecks([]reviewflow.CheckFact{{Status: c.Status, Conclusion: c.Conclusion, State: c.State}})
		result := "skipped-or-neutral"
		switch {
		case fail > 0:
			result = "failed"
		case pass > 0:
			result = "passed"
		case pending > 0:
			result = "pending"
		case unknown > 0:
			result = "unknown"
		}
		rows = append(rows, changeCheck{InspectedCheck: c, Result: result})
	}
	return rows
}

func cmdChange(h Header, repo string, number int, expected []string) (*Report, error) {
	if len(expected) > 1 || (len(expected) == 1 && !isFullCommitSHA(expected[0])) {
		return nil, deskkit.Refused("change: expected-head must be a full lowercase commit SHA")
	}
	f, fr, err := forgeFor(repo)
	if err != nil {
		return nil, err
	}
	var sha string
	if len(expected) == 1 {
		sha = expected[0]
	}
	r, err := deskkit.InspectChange(f, fr, number, sha)
	if err != nil {
		return nil, err
	}
	checks := changeCheckResults(r.Checks)
	return &Report{value: struct {
		Header
		*deskkit.ChangeInspection
		Checks []changeCheck `json:"checks"`
	}{h, r, checks}, renderTo: func(w io.Writer) error {
		// Compose the table in memory, where a write cannot fail, then write it once
		// and hand that write's error back so the caller can exit unverifiable.
		var b strings.Builder
		fmt.Fprintf(&b, "%s#%d head=%s %s draft=%t mergeable=%s merge-state=%s\n", repo, number, r.Head, r.State, r.Draft, r.Mergeable, r.MergeState)
		fmt.Fprintf(&b, "reviews (%s; inline discussions %s) %s\n", r.ReviewRead, r.InlineDiscussions, deskkit.StripControl(r.URL))
		for _, v := range r.Reviews {
			fmt.Fprintf(&b, "  %s %s %s %s", v.Binding, v.Lane, deskkit.StripControl(v.Author), deskkit.StripControl(v.State))
			if v.ID != 0 {
				fmt.Fprintf(&b, " id=%d", v.ID)
			}
			fmt.Fprintf(&b, " %s\n", deskkit.StripControl(v.URL))
			if v.Body != "" {
				fmt.Fprintf(&b, "    %q\n", v.Body)
			}
		}
		if r.ReviewError != "" {
			fmt.Fprintln(&b, deskkit.StripControl(r.ReviewError))
		}
		fmt.Fprintf(&b, "checks (%s)\n", r.CheckRead)
		if r.CheckError != "" {
			fmt.Fprintln(&b, deskkit.StripControl(r.CheckError))
		}
		for _, c := range checks {
			fmt.Fprintf(&b, "  %s %s job=%s run=%s %s\n", c.Result, deskkit.StripControl(c.Name), c.JobID, c.RunID, deskkit.StripControl(c.URL))
		}
		_, err := io.WriteString(w, b.String())
		return err
	}}, nil
}
