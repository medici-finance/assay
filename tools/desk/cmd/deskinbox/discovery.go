package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"io"
)

// discoveryJSON is a narrow adapter for the shell inbox's discovery stage. It
// preserves that caller's wire shape; detail, screen and flow stay with the caller.
// A failed repo returns partial data AND exit 2, just as the shell did. The typed
// listing refuses page overflow instead of silently losing the oldest issues.
func discoveryJSON(out, stderr io.Writer, repos []string) int {
	if err := validateRepos(repos); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitRefused
	}
	rows, failures := fetchQueue(repos)
	type label struct {
		Name string `json:"name"`
	}
	type row struct {
		Repo      string  `json:"repo"`
		Number    int     `json:"number"`
		Title     string  `json:"title"`
		URL       string  `json:"url"`
		CreatedAt string  `json:"createdAt"`
		Labels    []label `json:"labels"`
	}
	result := make([]row, 0, len(rows))
	for _, it := range rows {
		r := row{Repo: it.Repo, Number: it.Number, Title: it.Title, URL: it.URL, CreatedAt: it.CreatedAt, Labels: []label{}}
		for _, l := range it.Labels {
			r.Labels = append(r.Labels, label{Name: l})
		}
		result = append(result, r)
	}
	for _, f := range failures {
		fmt.Fprintf(stderr, "deskinbox: QUERY FAILED for %s: %v\n", f.Repo, f.Err)
	}
	fmt.Fprintf(stderr, "deskinbox: issue-discovery-v1 population=human escalation labels; freshness=this invocation; requested repos=%d; failed repos=%d; successful enumerations=complete\n", len(repos), len(failures))
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(failures) != 0 {
		return 2
	}
	return 0
}
