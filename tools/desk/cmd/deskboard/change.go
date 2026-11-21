package main

import (
	"fmt"
	"io"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

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
	return &Report{value: struct {
		Header
		*deskkit.ChangeInspection
	}{h, r}, render: func(w io.Writer) {
		fmt.Fprintf(w, "%s#%d head=%s %s draft=%t mergeable=%s merge-state=%s\n", repo, number, r.Head, r.State, r.Draft, r.Mergeable, r.MergeState)
		fmt.Fprintf(w, "reviews (%s; inline discussions %s) %s\n", r.ReviewRead, r.InlineDiscussions, deskkit.StripControl(r.URL))
		for _, v := range r.Reviews {
			fmt.Fprintf(w, "  %s %s %s %s", v.Binding, v.Lane, deskkit.StripControl(v.Author), deskkit.StripControl(v.State))
			if v.ID != 0 {
				fmt.Fprintf(w, " id=%d", v.ID)
			}
			fmt.Fprintf(w, " %s\n", deskkit.StripControl(v.URL))
			if v.Body != "" {
				fmt.Fprintf(w, "    %q\n", v.Body)
			}
		}
		if r.ReviewError != "" {
			fmt.Fprintln(w, deskkit.StripControl(r.ReviewError))
		}
		fmt.Fprintf(w, "checks (%s)\n", r.CheckRead)
		if r.CheckError != "" {
			fmt.Fprintln(w, deskkit.StripControl(r.CheckError))
		}
		for _, c := range r.Checks {
			fmt.Fprintf(w, "  %s %s job=%s run=%s %s\n", c.Result, deskkit.StripControl(c.Name), c.JobID, c.RunID, deskkit.StripControl(c.URL))
		}
	}}, nil
}
