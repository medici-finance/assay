package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// post.go — the ONE write this tool performs: create-or-update its own weekly digest
// issue. Every other write in the neighbourhood belongs to another tool and is named in
// the digest's own boundary section rather than performed here.
//
// THE WEEK IS THE IDENTITY. `Decision digest <ISO week>` is matched EXACTLY: same week →
// the existing issue is edited in place, so a Monday digest re-run on Thursday does not
// mint a second one; new week → no match → a new issue. There is no fuzzy fallback,
// because a near-match ("Decision digest 2026-W32") is a DIFFERENT week's report and
// editing it would overwrite a record the human may still be working through.

// existing is one candidate weekly issue found by title.
type existing struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// findWeekly locates this week's digest issue by EXACT title.
//
// Ambiguity is refused, never resolved: two open issues with the identical title mean
// somebody (or some other run) created a duplicate, and picking one would silently
// abandon the other's content. The refusal names both.
func findWeekly(repo, title string) (*existing, error) {
	out, err := runGH("issue", "list", "--repo", repo, "--state", "all", "--limit", "50",
		"--search", `"`+title+`" in:title`, "--json", "number,title,state")
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-check: the search for this week's digest issue in "+
			deskkit.StripControl(repo)+" failed — deskdigest will not create a second digest on an "+
			"unanswered question", err)
	}
	var cands []existing
	if jerr := json.Unmarshal([]byte(strings.TrimSpace(out)), &cands); jerr != nil {
		return nil, deskkit.Unverifiable("could-not-check: the digest-issue search did not parse", jerr)
	}
	var hits []existing
	for _, c := range cands {
		if c.Title == title {
			hits = append(hits, c)
		}
	}
	switch len(hits) {
	case 0:
		return nil, nil
	case 1:
		return &hits[0], nil
	default:
		var ns []string
		for _, h := range hits {
			ns = append(ns, "#"+strconv.Itoa(h.Number))
		}
		return nil, deskkit.Refused("refused: " + strconv.Itoa(len(hits)) + " issues in " +
			deskkit.StripControl(repo) + " already carry the exact title " + strconv.Quote(title) +
			" (" + strings.Join(ns, ", ") + "). deskdigest will not pick one: editing the wrong duplicate " +
			"silently abandons whatever the human wrote on the other.")
	}
}

// postDigest creates or updates this week's issue. Returns the issue number and whether
// it was newly created.
func postDigest(ac *auditCtx, d *digest, repo, body string, stderr io.Writer) (int, bool, error) {
	// The body is scanned before it can reach the remote. The digest quotes issue titles
	// and body lines authored elsewhere, so its content is public-origin text even though
	// this tool assembled it.
	if err := bodyCheck([]byte(body)); err != nil {
		return 0, false, err
	}

	title := d.title()
	prev, err := findWeekly(repo, title)
	if err != nil {
		return 0, false, err
	}

	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return 0, false, deskkit.Unverifiable("cannot stage the digest body", terr)
	}
	defer cleanup()

	if prev == nil {
		// A create's number is not known in advance, so AllowWriteRepoWide is the scope
		// its writes land in — the same reasoning as deskpr create and deskfile new.
		if werr := allowWriteRepoWide(repo); werr != nil {
			return 0, false, werr
		}
		ac.wrote = true
		out, cerr := runGH("issue", "create", "--repo", repo, "--title", title, "--body-file", bodyPath)
		if cerr != nil {
			return 0, false, deskkit.Unverifiable("gh issue create failed for the weekly digest", cerr)
		}
		n := issueNumberFromURL(deskkit.StripControl(strings.TrimSpace(out)))
		if n > 0 {
			ac.target = &n
		}
		fmt.Fprintf(stderr, "deskdigest: created %s#%d %s\n", repo, n, title)
		fmt.Fprintf(stderr, "deskdigest: to supersede last week's digest, a human or the desk runs:\n  %s\n",
			supersedeHint(repo, n))
		return n, true, nil
	}

	// A CLOSED digest for the current week is not reopened. Reopening is a decision about
	// whether the week is still live, and this tool decides nothing; it also is not a
	// close lane, so it is not deskclose's either. Say so and stop.
	if !strings.EqualFold(prev.State, "OPEN") {
		return 0, false, deskkit.Refused(fmt.Sprintf(
			"refused: this week's digest %s#%d is %s. deskdigest does not reopen an issue — whether the "+
				"week is still live is a judgement, and this tool renders reports rather than making them.",
			deskkit.StripControl(repo), prev.Number, strings.ToLower(deskkit.StripControl(prev.State))))
	}

	if werr := allowWrite(repo, prev.Number); werr != nil {
		return 0, false, werr
	}
	n := prev.Number
	ac.target = &n
	ac.wrote = true
	if _, eerr := runGH("issue", "edit", strconv.Itoa(prev.Number), "--repo", repo, "--body-file", bodyPath); eerr != nil {
		return 0, false, deskkit.Unverifiable("gh issue edit failed for the weekly digest", eerr)
	}
	fmt.Fprintf(stderr, "deskdigest: updated %s#%d %s in place (same week)\n", repo, prev.Number, title)
	return prev.Number, false, nil
}

// writeTempBody stages the body in a 0600 temp file. gh reads it via --body-file so the
// content never rides an argv, where a process listing would expose it.
func writeTempBody(body string) (string, func(), error) {
	f, err := os.CreateTemp("", "deskdigest-body-*.md")
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, werr := f.WriteString(body); werr != nil {
		f.Close()
		cleanup()
		return "", func() {}, werr
	}
	if cerr := f.Close(); cerr != nil {
		cleanup()
		return "", func() {}, cerr
	}
	return name, cleanup, nil
}

// issueNumberFromURL extracts the trailing issue number from a gh-printed URL.
func issueNumberFromURL(u string) int {
	i := strings.LastIndex(u, "/")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(u[i+1:]))
	if err != nil {
		return 0
	}
	return n
}
