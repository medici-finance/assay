// Command repohardenguard checks a repository's live hardening settings against
// docs/repo-hardening-checklist.md and reports THREE states per row:
//
//	checked-ok       the value was read and matches the checklist
//	checked-wrong    the value was read and does not match
//	could-not-check  the value could not be established at this permission level
//
// plus `not available` for a setting the repo's plan or visibility does not
// offer, which the checklist records with its reason.
//
// The third state is the reason this program exists. A past incident
// records a repository that sat public for three days because nobody was
// watching its settings; the failure that keeps that class of incident alive is
// an instrument that reads an admin-gated `null` as an answer. GitHub returns
// `security_and_analysis: null` and omits `bypass_actors` entirely for a
// non-admin token, and those responses are identical whether the setting is on
// or off. So: an admin-gated field that does not resolve is could-not-check,
// never ok and never absent, and a could-not-check is never a pass.
//
// GET-ONLY. This program applies nothing. Every setting in the checklist is a
// repository-admin act and belongs to the human.
//
// It deliberately does NOT gate on the desk write-authorisation set — the
// allow-listed-repos membership check in deskkit. That set authorises WRITES,
// and medici-finance/assay is deliberately outside it; making a read-only
// checker depend on it would create pressure to widen a write-authorisation
// set for a read. This program reads NO roster: its scope is the checklist's
// Repo column and the repo's own live settings, and nothing else. (It is
// listed in echocoverage_test's exemptFromRoster for exactly that reason.)
//
// Exit codes (deskkit convention):
//
//	0  every row checked-ok or not available
//	5  at least one checked-wrong — the world disagrees with the checklist
//	6  at least one could-not-check, or the guard could not run at all
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const defaultChecklist = "docs/repo-hardening-checklist.md"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errW io.Writer) int {
	fs := flag.NewFlagSet("repohardenguard", flag.ContinueOnError)
	fs.SetOutput(errW)
	repo := fs.String("repo", "", "repository to check, owner/name (required)")
	path := fs.String("checklist", defaultChecklist, "path to the hardening checklist")
	jsonOut := fs.Bool("json", false, "emit results as JSON")
	fs.Usage = func() {
		fmt.Fprintf(errW, "usage: repohardenguard --repo <owner/name> [--checklist %s] [--json]\n", defaultChecklist)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitUnverifiable
	}
	if strings.TrimSpace(*repo) == "" {
		fmt.Fprintln(errW, "repohardenguard: --repo is required — a run with no repo checks nothing and must not exit 0")
		fs.Usage()
		return deskkit.ExitUnverifiable
	}

	cl, err := ParseChecklistFile(*path)
	if err != nil {
		fmt.Fprintf(errW, "repohardenguard: %v\n", err)
		return deskkit.ExitUnverifiable
	}

	// The repo asked about must be one the checklist declares. Anything else is
	// a refusal, not an empty green run: a repo outside the declared set has no
	// rows by construction, and "no rows" must never render as "nothing wrong".
	if !cl.Covers(*repo) {
		fmt.Fprintf(errW, "repohardenguard: %q is not a repo the checklist %s declares — declared repos are: %s\n",
			*repo, *path, strings.Join(cl.Repos, ", "))
		return deskkit.ExitUnverifiable
	}

	mine, census := cl.RowsFor(*repo)
	if len(mine) == 0 {
		fmt.Fprintf(errW, "repohardenguard: the checklist %s declares %q but has no rows for it — a declared repo with nothing to check must not exit 0\n", *path, *repo)
		return deskkit.ExitUnverifiable
	}

	c := Checker{Get: ghGet}

	// Preflight. Two jobs, both load-bearing:
	//   1. It proves the token can read this repo at all. Every "404 means the
	//      thing is absent" call below rests on that; without it a token with no
	//      access would report the whole checklist as absent-and-wrong.
	//   2. It names the acting identity, because evidence for an admin-gated row
	//      is worthless unless it says who ran it.
	if _, perr := c.Get("repos/" + *repo); perr != nil {
		fmt.Fprintf(errW, "repohardenguard: cannot read %s (%v) — nothing below could be established, so no row is reported\n", *repo, perr)
		return deskkit.ExitUnverifiable
	}

	results := make([]Result, 0, len(mine))
	for _, r := range mine {
		results = append(results, c.CheckRow(r))
	}

	code := exitCode(results)
	scope := scopeLine(*repo, len(cl.Rows), census)
	if *jsonOut {
		emitJSON(out, *repo, identity(c), *path, scope, results, code)
	} else {
		emitText(out, *repo, identity(c), *path, scope, results, code)
	}
	return code
}

// scopeLine accounts for EVERY row in the checklist, not just the ones this run
// evaluated. Silent row-loss is the failure it exists to make visible: before
// this, a row the run did not pick up simply never appeared anywhere in the
// output, so the count looked right while the coverage had shrunk. The rows for
// this repo plus the rows for each other repo must add up to the file's total,
// and this line shows the reader that sum.
func scopeLine(repo string, total int, census map[string]int) string {
	others := make([]string, 0, len(census))
	for r, n := range census {
		if r != repo {
			others = append(others, fmt.Sprintf("%d for %s", n, r))
		}
	}
	sort.Strings(others)
	s := fmt.Sprintf("rows: %d in the checklist — %d for %s, evaluated below", total, census[repo], repo)
	if len(others) > 0 {
		s += fmt.Sprintf("; %s (a separate run)", strings.Join(others, ", "))
	}
	return s
}

// identity reports the login the reads were made as. Best effort: an App
// installation token cannot read /user, and that is reported as unknown rather
// than guessed — a wrong identity on an admin-gated row is worse than none.
func identity(c Checker) string {
	body, err := c.Get("user")
	if err != nil {
		return "unknown (the token cannot read /user)"
	}
	var u struct {
		Login string `json:"login"`
	}
	if json.Unmarshal(body, &u) != nil || u.Login == "" {
		return "unknown"
	}
	return u.Login
}

func exitCode(results []Result) int {
	wrong, unknown := 0, 0
	for _, r := range results {
		switch r.State {
		case StateWrong:
			wrong++
		case StateUnknown:
			unknown++
		}
	}
	switch {
	case wrong > 0:
		return deskkit.ExitRefused
	case unknown > 0:
		return deskkit.ExitUnverifiable
	default:
		return deskkit.ExitOK
	}
}

func emitText(w io.Writer, repo, who, path, scope string, results []Result, code int) {
	fmt.Fprintf(w, "repohardenguard %s\n", repo)
	fmt.Fprintf(w, "checklist: %s\n", path)
	fmt.Fprintf(w, "identity: %s\n", who)
	// Every verdict below was measured against an endpoint the parser proved is
	// scoped to this repo, so the name in the header is the repo that was read.
	fmt.Fprintf(w, "%s\n", scope)
	fmt.Fprintln(w)

	width := 0
	for _, r := range results {
		if n := len(r.Row.ID); n > width {
			width = n
		}
	}
	for _, r := range results {
		fmt.Fprintf(w, "%-15s %-*s %s\n", r.State, width, r.Row.ID, r.Detail)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "summary: %s (%d rows)\n", summary(results), len(results))
	switch code {
	case deskkit.ExitRefused:
		fmt.Fprintln(w, "RED: at least one setting does not match the checklist. Apply it with that row's Set command, or change the checklist if the requirement was wrong.")
	case deskkit.ExitUnverifiable:
		fmt.Fprintln(w, "RED: at least one value could not be established. This is NOT a pass. Admin-gated rows must be re-run by a repository admin, and the evidence must name that identity.")
	default:
		fmt.Fprintln(w, "GREEN: every checkable row matches, and every other row is a recorded divergence.")
	}
}

// summary lists only the non-zero categories, so an all-green run's output does
// not contain the string "could-not-check" at all — a reader (or a Verify row)
// greps the output for that token to decide whether anything went unread.
func summary(results []Result) string {
	counts := map[State]int{}
	for _, r := range results {
		counts[r.State]++
	}
	order := []State{StateOK, StateWrong, StateUnknown, StateNotAvailable}
	var parts []string
	for _, s := range order {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	if len(parts) == 0 {
		return "no rows"
	}
	return strings.Join(parts, ", ")
}

type jsonRow struct {
	ID       string `json:"id"`
	Repo     string `json:"repo"`
	Setting  string `json:"setting"`
	Gated    string `json:"gated"`
	Field    string `json:"field"`
	Required string `json:"required"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
}

func emitJSON(w io.Writer, repo, who, path, scope string, results []Result, code int) {
	rep := struct {
		Repo      string         `json:"repo"`
		Identity  string         `json:"identity"`
		Checklist string         `json:"checklist"`
		Scope     string         `json:"scope"`
		Exit      int            `json:"exit"`
		Counts    map[string]int `json:"counts"`
		Rows      []jsonRow      `json:"rows"`
	}{Repo: repo, Identity: who, Checklist: path, Scope: scope, Exit: code, Counts: map[string]int{}}

	for _, r := range results {
		rep.Counts[string(r.State)]++
		rep.Rows = append(rep.Rows, jsonRow{
			ID: r.Row.ID, Repo: r.Row.Repo, Setting: r.Row.Setting, Gated: r.Row.Gated,
			Field: r.Row.Field, Required: r.Row.Required, State: string(r.State), Detail: r.Detail,
		})
	}
	sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].ID < rep.Rows[j].ID })

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}
