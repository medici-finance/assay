package main

// walk.go — `deskinbox walk [--item K] [repo ...]`, ported from the oracle's `--walk`
// case (assay-inbox.sh:1336-1354) and render_walk() (assay-inbox.sh:569-588): print ONE
// item in the five-part decision format and exit. Deliberately non-interactive — it never
// prompts and never blocks on a tty; the turn-taking belongs to the `ask-decision` skill,
// which shells this with an incrementing --item.

import (
	"fmt"
	"io"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// renderWalk prints the five-part block for one rendered item, in the oracle's exact
// section order and prefixing.
func renderWalk(w io.Writer, r rendered) {
	fmt.Fprintln(w, r.Header)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Context")
	for _, c := range r.Context {
		fmt.Fprintln(w, "  - "+c)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options")
	for _, o := range r.Options {
		line := "  " + o.Letter + ". " + o.Text
		if o.Recommended {
			line += "   [recommended]"
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Reply shape")
	fmt.Fprintln(w, "  "+r.Reply)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Verification")
	fmt.Fprintln(w, "  "+r.Verification)
}

// runWalk resolves the repo queue, builds the requested 1-based item's detail + rendering,
// and prints it. It returns the process exit code (deskkit's shared taxonomy — see
// summary.go) and writes to stdout/stderr exactly as the oracle's walk case does: the
// block, a blank line, then the terminating summary.
func runWalk(stdout, stderr io.Writer, repos []string, walkItem int, now time.Time) int {
	if err := validateRepos(repos); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	items, failures := fetchQueue(repos)
	for _, f := range failures {
		fmt.Fprintf(stderr, "deskinbox: QUERY FAILED for %s: %v\n", f.Repo, f.Err)
	}

	if len(items) == 0 {
		fmt.Fprintln(stdout, summaryText(0, len(repos), failures))
		if len(failures) > 0 {
			return deskkit.ExitUnverifiable
		}
		return deskkit.ExitOK
	}
	if walkItem > len(items) {
		fmt.Fprintf(stderr, "deskinbox: --item %d is out of range (the queue holds %d item(s))\n", walkItem, len(items))
		return deskkit.ExitRefused
	}

	k := walkItem - 1
	it := items[k]
	fr := deskkit.ForgeRepo{}
	if owner, name, ok := splitRepo(it.Repo); ok {
		fr = deskkit.ForgeRepo{Owner: owner, Name: name}
	}
	d := fetchDetail(fr, it.Number)
	r := buildRendered(it, d, k, len(items), now)

	renderWalk(stdout, r)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, summaryText(len(items), len(repos), failures))
	if len(failures) > 0 {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

func splitRepo(repo string) (owner, name string, ok bool) {
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			return repo[:i], repo[i+1:], true
		}
	}
	return "", "", false
}
