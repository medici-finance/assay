package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// collect.go — the READ half. Every gh invocation in this file is a GET; the write
// argvs live in post.go and are reachable only from --post. TestReportOnlyNeverMutates
// enumerates what this file constructs and fails on any mutating verb.

// comment is the subset of a GitHub comment the classifier consumes.
type comment struct {
	AuthorLogin string
	AuthorID    int64
	AuthorType  string
	Body        string
	CreatedAt   time.Time
}

// item is one collected decision-queue entry.
type item struct {
	Repo        string
	Number      int
	Title       string
	URL         string
	CreatedAt   time.Time
	Labels      []string
	Body        string
	AuthorLogin string
	AuthorID    int64
	AuthorType  string

	// CommentsRead is the three-state hinge for this item: false means the thread did
	// not come back, which classifyItem treats as could-not-read rather than as "no
	// blessing-authority comment exists". The difference between those two readings is the whole
	// gap between a report and a guess.
	CommentsRead bool
	CommentsErr  string
	Comments     []comment
}

// repoRead is one repo's read outcome. It records the states separately on purpose:
//
//   - Err != ""            → the repo did not answer. Its items are UNKNOWN.
//   - LabelSetRead == false → the label inventory did not answer, so a zero row-count
//     under a label cannot be distinguished from a missing label.
//   - MissingLabels        → the label does not EXIST in this repo. Zero rows under it
//     means nobody can have applied it yet — not that the queue is empty. `human-only`
//     is in exactly this state in both tracked repos today (measured 2026-08-13): the
//     label is introduced by this brief and applied during a later sweep.
type repoRead struct {
	Repo          string
	Err           string
	LabelSetRead  bool
	MissingLabels []string
	Items         int
}

// collection is everything one run read, plus how well it read it.
type collection struct {
	Items []*item
	Repos []repoRead
	// Scope is the repo list the run was asked to read, in order, so an empty digest can
	// name what it looked at. "I read these five repos and found nothing" is a report;
	// "nothing" is not.
	Scope []string
}

// Partial reports whether any repo failed to answer — the flag that turns a --post into
// an exit 6 while still posting a digest that names the gap in its own body.
func (c *collection) Partial() bool {
	for _, r := range c.Repos {
		if r.Err != "" || !r.LabelSetRead {
			return true
		}
	}
	return false
}

// digestLabels is the input label set: the standing decision queue plus the infra asks
// only the human can perform.
var digestLabels = []string{needsDecisionLabel, humanOnlyLabel}

// collect reads every repo in scope. A repo that fails is RECORDED and the run
// continues: one unreachable repo must not suppress the other four repos' items, and it
// must not be silently absent either. Both halves of that are the point.
func collect(scope []string) *collection {
	c := &collection{Scope: append([]string(nil), scope...)}
	seen := map[string]bool{}
	for _, repo := range scope {
		rr := repoRead{Repo: repo}
		labels, lerr := repoLabels(repo)
		if lerr != nil {
			rr.LabelSetRead = false
			rr.Err = "label inventory unreadable: " + deskkit.StripControl(lerr.Error())
			c.Repos = append(c.Repos, rr)
			continue
		}
		rr.LabelSetRead = true
		var failed []string
		for _, lbl := range digestLabels {
			if !labels[strings.ToLower(lbl)] {
				rr.MissingLabels = append(rr.MissingLabels, lbl)
				continue
			}
			its, err := listIssues(repo, lbl)
			if err != nil {
				failed = append(failed, lbl+": "+deskkit.StripControl(err.Error()))
				continue
			}
			for _, it := range its {
				key := fmt.Sprintf("%s#%d", it.Repo, it.Number)
				if seen[key] {
					continue // an item carrying BOTH labels is one item, not two rows
				}
				seen[key] = true
				loadComments(it)
				c.Items = append(c.Items, it)
				rr.Items++
			}
		}
		if len(failed) > 0 {
			rr.Err = "issue list failed for " + strings.Join(failed, "; ")
		}
		c.Repos = append(c.Repos, rr)
	}
	sort.SliceStable(c.Items, func(i, j int) bool {
		if !c.Items[i].CreatedAt.Equal(c.Items[j].CreatedAt) {
			return c.Items[i].CreatedAt.Before(c.Items[j].CreatedAt) // oldest first: the queue's shame is its age
		}
		if c.Items[i].Repo != c.Items[j].Repo {
			return c.Items[i].Repo < c.Items[j].Repo
		}
		return c.Items[i].Number < c.Items[j].Number
	})
	return c
}

// repoLabels reads a repo's label inventory. It exists for ONE reason: `gh issue list
// --label X` on a repo where X does not exist returns an empty array and exit 0 —
// measured 2026-08-13 against the tracked repos — so "no items carry this label" and
// "this label has never been created" are the same bytes. Without this probe the digest
// would report an empty human-only queue every week for a label nobody has applied yet,
// which is precisely the failure the empty-week invariant forbids.
func repoLabels(repo string) (map[string]bool, error) {
	// Validate the slug the same way listIssues and loadComments do, so a malformed
	// --repos entry surfaces as the config error it is rather than as a `gh label list`
	// exit-1 that reads like a repo outage — a could-not-read the operator would go chase
	// the wrong way.
	if _, _, ok := splitRepo(repo); !ok {
		return nil, fmt.Errorf("malformed repo slug %s", deskkit.StripControl(repo))
	}
	out, err := runGH("label", "list", "--repo", repo, "--limit", "300", "--json", "name")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Name string `json:"name"`
	}
	if jerr := json.Unmarshal([]byte(out), &raw); jerr != nil {
		return nil, fmt.Errorf("label list did not parse: %w", jerr)
	}
	set := map[string]bool{}
	for _, l := range raw {
		set[strings.ToLower(strings.TrimSpace(l.Name))] = true
	}
	return set, nil
}

// ghIssue is the REST shape for an issue.
//
// The REST endpoint is used in preference to `gh issue list --json author` because the
// body-override gate needs the author's NUMERIC id: `gh issue list` reports a GraphQL
// node id, and deskkit's trust check is the id-pinned form for the same reason
// deskclose's is — a login is a claim about a name, and a recycled or squatted one must
// not be able to hand itself an override.
type ghIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	User struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"user"`
	// PullRequest is present iff this "issue" is really a pull request. The REST issues
	// endpoint returns both; a PR in the decision digest would be a category error.
	PullRequest *struct{} `json:"pull_request"`
}

// listIssues reads the OPEN issues carrying one label. --paginate is deliberate: a
// truncated read that silently dropped the tail would be the same defect as a skipped
// week, one level down.
func listIssues(repo, label string) ([]*item, error) {
	owner, name, ok := splitRepo(repo)
	if !ok {
		return nil, fmt.Errorf("malformed repo slug %s", deskkit.StripControl(repo))
	}
	out, err := runGH("api", "-H", "Accept: application/vnd.github+json", "--paginate",
		fmt.Sprintf("repos/%s/%s/issues?state=open&per_page=100&labels=%s", owner, name, url.QueryEscape(label)))
	if err != nil {
		return nil, err
	}
	var raw []ghIssue
	if jerr := json.Unmarshal([]byte(concatJSONArrays(out)), &raw); jerr != nil {
		return nil, fmt.Errorf("issue list did not parse: %w", jerr)
	}
	items := make([]*item, 0, len(raw))
	for _, r := range raw {
		if r.PullRequest != nil {
			continue
		}
		it := &item{
			Repo:        repo,
			Number:      r.Number,
			Title:       deskkit.StripControl(r.Title),
			URL:         deskkit.StripControl(r.HTMLURL),
			Body:        r.Body,
			AuthorLogin: r.User.Login,
			AuthorID:    r.User.ID,
			AuthorType:  r.User.Type,
		}
		if ts, perr := time.Parse(time.RFC3339, r.CreatedAt); perr == nil {
			it.CreatedAt = ts.UTC()
		}
		for _, l := range r.Labels {
			it.Labels = append(it.Labels, l.Name)
		}
		items = append(items, it)
	}
	return items, nil
}

// ghComment is the REST shape for an issue comment. The REST endpoint is used rather
// than `gh issue view --json comments` because the classifier needs the author's numeric
// ID: the blessing-authority check is the strict id-pinned form, and a login on its own
// is a claim about a name.
type ghComment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	User      struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"user"`
}

// loadComments fetches an item's thread. A failure sets CommentsRead=false and leaves
// the reason attached — it never returns an empty thread, because an empty thread and an
// unread thread classify differently and must stay distinguishable all the way to the
// rendered row.
func loadComments(it *item) {
	owner, name, ok := splitRepo(it.Repo)
	if !ok {
		it.CommentsRead = false
		it.CommentsErr = "malformed repo slug " + deskkit.StripControl(it.Repo)
		return
	}
	out, err := runGH("api", "-H", "Accept: application/vnd.github+json", "--paginate",
		fmt.Sprintf("repos/%s/%s/issues/%d/comments?per_page=100", owner, name, it.Number))
	if err != nil {
		it.CommentsRead = false
		it.CommentsErr = deskkit.StripControl(firstLine(err.Error()))
		return
	}
	var raw []ghComment
	if jerr := json.Unmarshal([]byte(concatJSONArrays(out)), &raw); jerr != nil {
		it.CommentsRead = false
		it.CommentsErr = "comment thread did not parse"
		return
	}
	it.CommentsRead = true
	for _, r := range raw {
		c := comment{AuthorLogin: r.User.Login, AuthorID: r.User.ID, AuthorType: r.User.Type, Body: r.Body}
		if ts, perr := time.Parse(time.RFC3339, r.CreatedAt); perr == nil {
			c.CreatedAt = ts.UTC()
		}
		it.Comments = append(it.Comments, c)
	}
}

// concatJSONArrays joins the `][` seam `gh api --paginate` leaves between pages so the
// result parses as one array.
func concatJSONArrays(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "[]"
	}
	return strings.NewReplacer("][", ",", "]\n[", ",").Replace(s)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func splitRepo(repo string) (owner, name string, ok bool) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ---------------------------------------------------------------------------
// R-3 decisions already taken — READ, never written
// ---------------------------------------------------------------------------

// r3MarkerRe matches the machine-readable record a desk leaves when it takes a decision
// under R-3. deskdigest READS this marker and never writes one: taking an R-3 decision
// is the desk's act, and RECORDING it in the weekly digest — with the veto deadline — is
// this tool's. R-3 makes the digest the veto surface, so the surface must show every
// decision it can find and must not be able to manufacture one.
var r3MarkerRe = regexp.MustCompile(`(?is)<!--\s*desk-r3-decision v1\s*-->\s*(.*)`)

// r3DecisionRe pulls the decision sentence out of the marker comment.
var r3DecisionRe = regexp.MustCompile(`(?im)^[ \t>*_-]*decision:[ \t]*(.+)$`)

// vetoWindowDays is R-3's stated window: "recorded in the weekly decision digest with a
// 7-day veto window. If the human does not veto within 7 days, the desk's recorded decision
// stands."
const vetoWindowDays = 7

// r3Decision is one desk decision awaiting its veto deadline.
type r3Decision struct {
	Item     *item
	Decision string
	Taken    time.Time
	VetoEnds time.Time
}

// r3Decisions scans the collected items for desk R-3 decision markers. Only a TRUSTED
// author's marker counts — an untrusted comment claiming the desk decided something
// would otherwise print in the digest as though the desk had.
func r3Decisions(c *collection) []r3Decision {
	var out []r3Decision
	for _, it := range c.Items {
		if !it.CommentsRead {
			continue // already reported as could-not-read; not silently absent
		}
		for i := range it.Comments {
			cm := &it.Comments[i]
			if !deskkit.TrustedAuthorID(cm.AuthorLogin, cm.AuthorID) {
				continue
			}
			m := r3MarkerRe.FindStringSubmatch(cm.Body)
			if m == nil {
				continue
			}
			dec := "(marker present, no `decision:` line)"
			if d := r3DecisionRe.FindStringSubmatch(m[1]); d != nil {
				dec = oneLine(d[1])
			}
			out = append(out, r3Decision{
				Item:     it,
				Decision: dec,
				Taken:    cm.CreatedAt,
				VetoEnds: cm.CreatedAt.AddDate(0, 0, vetoWindowDays),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].VetoEnds.Before(out[j].VetoEnds) })
	return out
}
