// deskread — the read verb statusgen reaches a forge through (the statusgen-off-the-forge-CLI
// brief, slice 1).
//
// WHY THIS EXISTS. statusgen is a separate Go module that does not import deskkit, and it
// carried its own forge-CLI shell-outs, so none of the guarantees the seam gives the desk verbs
// — the enumerated operation set, the refusal instead of a fallback, the per-forge backend —
// reached it. This verb is the seam's read half packaged as a process: statusgen runs it and
// parses JSON, rather than linking a package or shelling a forge CLI. The module boundary stays;
// the forge dependency leaves.
//
// IT ADDS NO OPERATION TO Forge. Every read kind maps onto an operation the interface already
// enumerates with both backends implemented. The freeze rule (an added op needs a consuming call
// site in the same change) is satisfied by CONSUMING the surface, not widening it: `issues` is
// deskkit.Forge.ListOpenIssues, `trust` is deskkit.Forge.IssueTrustEvents, `comments` is
// deskkit.Forge.ListCommentsTyped on an ISSUE, and nothing else.
//
// TWO ADDRESSING SHAPES. `issues` is a per-REPO read (--repo). `trust` and `comments` are
// per-ISSUE reads (--issue owner/name#N): they are what statusgen's --scan-issues trust gate and
// un-block lane consume, one issue at a time, in place of the `gh api graphql` / `gh api
// --paginate` shell-outs that 401'd under a replaced HOME. Each kind accepts exactly its own
// address flag and refuses the other, so a caller can never ask a per-issue kind for a whole
// repo or the other way round.
//
// WHY A NEW VERB RATHER THAN A deskboard SUBCOMMAND. deskboard's subcommands all COMPOSE a desk
// view (prs, queue, health, next-up, throughput, stalled …) under the roster's scope and
// staleness semantics; a raw read is the only one whose output would not be a board, and the
// caller would have to opt out of every board-shaped default to use it. deskboard's permit
// register row was also deliberately narrowed by the read-verbs brief and spent by the
// non-board-reads brief — a new consumer inside it re-widens a row the stream just closed. And
// the JSON contract below is pinned by a consumer across a module boundary and a pinned release,
// so it must not move whenever a board schema does.
//
// ONE INVOCATION SERVES A REPO SET. --repo is repeatable and the reads run concurrently under a
// bounded worker count. This is the performance half of the verb and the reason it is not one
// call per repo in a loop: the shape it replaces was N serial process starts, N token
// resolutions and N sequential round trips.
//
// PARTIAL IS A RESULT, NOT AN ERROR. A repo that could not be read appears in "partial" with its
// reason and does NOT appear in "repos"; the exit code stays 0. A caller that treats "some repos
// unreadable" as "nothing found anywhere" is exactly the could-not-check-as-a-clean failure this
// shape exists to prevent — the envelope makes the distinction unmissable. Only when NO repo
// could be read is the run unverifiable (exit 6).
//
// Exit codes (deskkit contract): 0 ok (including a partial read) · 3 disabled · 5 refused (bad
// flags or an unknown kind) · 6 unverifiable (no repo — or, for a per-issue kind, no issue —
// could be read).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// envelopeSchema is the version of the JSON contract below. A consumer checks it and REFUSES an
// unrecognised value rather than best-effort parsing — the same fail-closed direction statusgen's
// own brief parser already takes on an unknown brief schema. Bump it only for a change a pinned
// consumer could not read correctly; adding an omitempty field is not such a change.
const envelopeSchema = 1

// readKinds is the CLOSED set of reads this verb serves. It is a set of KINDS, never an address:
// there is no --query, --endpoint or --path flag, and adding one would reopen exactly the
// passthrough the forge-surface control forbids. Slice 1 landed `issues`; each further kind is
// added with the call site that consumes it (`trust` and `comments`: statusgen --scan-issues'
// trust gate and un-block lane).
var readKinds = map[string]string{
	"issues":   "open issues per repo (deskkit.Forge.ListOpenIssues)",
	"trust":    "an issue's trust-gate content events, one bounded page (deskkit.Forge.IssueTrustEvents)",
	"comments": "an issue's whole comment thread, oldest first (deskkit.Forge.ListCommentsTyped, issue kind)",
}

// perIssueKinds are the kinds addressed by --issue rather than --repo.
var perIssueKinds = map[string]bool{"trust": true, "comments": true}

var usage = `deskread — read-only forge reads on the seam, as JSON

usage:
  deskread issues   --repo  <owner/repo>   [--repo  <owner/repo> …]
  deskread trust    --issue <owner/repo#N> [--issue <owner/repo#N> …]
  deskread comments --issue <owner/repo#N> [--issue <owner/repo#N> …]
  deskread --version

flags:
  --repo <owner/repo>     repeatable (issues only); ONE invocation serves the whole set, read concurrently
  --issue <owner/repo#N>  repeatable (trust, comments only); same set semantics, one entry per issue
  --max-parallel <N>      bounded concurrency for the set (default 6)

output: a versioned JSON envelope on stdout

  {"schema": 1, "kind": "issues",
   "repos":   [{"repo": "o/n", "issues": [{"number": …, "state": "open", …}]}],
   "partial": [{"repo": "o/n", "reason": "…"}]}

  {"schema": 1, "kind": "trust",
   "items":   [{"repo": "o/n", "number": 7, "trust": {"bodyEditedAt": "…", "complete": true,
               "events": [{"authorLogin": …, "authorId": …, "createdAt": …, "editedAt": …}]}}],
   "partial": [{"repo": "o/n", "number": 7, "reason": "…"}]}

  {"schema": 1, "kind": "comments",
   "items":   [{"repo": "o/n", "number": 7, "comments": [{"authorLogin": …, "authorId": …,
               "createdAt": …, "body": …}]}],
   "partial": [{"repo": "o/n", "number": 7, "reason": "…"}]}

A repo (or issue) that could not be read lands in "partial" with its reason and is ABSENT from
"repos" ("items"); the exit code is still 0. That is deliberate: a caller must be able to tell
"no open issues" / "no comments" from "could not look". Exit 6 only when NOTHING in the set could
be read. A trust read's "complete": false means the thread overflowed the single bounded page —
the caller fails that issue closed; it is an answer, not a partial.

identity: reads authenticate as this session's minted App role via the deskkit resolver. There
is no ambient-credential fallback — a session with no resolvable role is refused, never silently
degraded onto whatever the local forge CLI happens to hold.
`

func main() {
	// This tool reads the roster to resolve its acting role, so ciEligible=false: config-home
	// file only, never the environment, in CI as well as locally — the same declaration every
	// other acting read verb makes.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		s, b := deskkit.Version()
		fmt.Fprintf(stdout, "deskread sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stderr, usage)
		return deskkit.ExitRefused
	}

	kind := args[0]
	if _, ok := readKinds[kind]; !ok {
		fmt.Fprintf(stderr, "deskread: refused: unknown read kind %q — the kind set is closed (%s)\n",
			kind, strings.Join(sortedKinds(), ", "))
		return deskkit.ExitRefused
	}

	repos, issues, maxPar, ferr := parseFlags(args[1:])
	if ferr != nil {
		fmt.Fprintf(stderr, "deskread: refused: %v\n", ferr)
		return deskkit.ExitRefused
	}
	if perIssueKinds[kind] {
		return runPerIssue(kind, repos, issues, maxPar, stdout, stderr)
	}
	if len(issues) > 0 {
		fmt.Fprintf(stderr, "deskread: refused: --issue does not address the %q kind — it reads whole repos (--repo)\n", kind)
		return deskkit.ExitRefused
	}
	if len(repos) == 0 {
		fmt.Fprintln(stderr, "deskread: refused: no --repo given — an empty set is never reported as an empty result")
		return deskkit.ExitRefused
	}

	env := readSet(kind, repos, maxPar)
	enc, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "deskread: could-not-check: rendering the envelope failed: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	fmt.Fprintln(stdout, string(enc))

	// EVERY repo unreadable is the one case that is not a partial result: there is nothing to
	// be partial ABOUT, and a consumer handed an all-empty envelope with exit 0 would have no
	// signal left. The reasons are still in the envelope, so the caller can report them.
	if len(env.Repos) == 0 {
		fmt.Fprintf(stderr, "deskread: could-not-check: no repo in the set of %d could be read\n", len(repos))
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

func sortedKinds() []string {
	out := make([]string, 0, len(readKinds))
	for k := range readKinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// issueTarget is one --issue address: a repo coordinate plus an issue number.
type issueTarget struct {
	Repo   string
	Number int
}

// parseFlags reads the repeatable --repo and --issue sets and the concurrency bound. It is
// hand-rolled rather than flag.FlagSet because both address flags repeat, and it REFUSES an
// unknown flag rather than ignoring it: a typo'd flag that silently does nothing is how a caller
// ends up reading a narrower set than it asked for and never finds out. Which address flag a
// kind accepts is checked by the caller, against the kind.
func parseFlags(args []string) (repos []string, issues []issueTarget, maxParallel int, err error) {
	maxParallel = 6
	seen := map[string]bool{}
	seenIssue := map[issueTarget]bool{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return nil, nil, 0, fmt.Errorf("--repo needs a value")
			}
			i++
			r := strings.TrimSpace(args[i])
			if !validRepoSlug(r) {
				return nil, nil, 0, fmt.Errorf("bad --repo %q — expected owner/name", r)
			}
			if !seen[r] {
				seen[r] = true
				repos = append(repos, r)
			}
		case "--issue":
			if i+1 >= len(args) {
				return nil, nil, 0, fmt.Errorf("--issue needs a value")
			}
			i++
			tgt, terr := parseIssueTarget(args[i])
			if terr != nil {
				return nil, nil, 0, terr
			}
			if !seenIssue[tgt] {
				seenIssue[tgt] = true
				issues = append(issues, tgt)
			}
		case "--max-parallel":
			if i+1 >= len(args) {
				return nil, nil, 0, fmt.Errorf("--max-parallel needs a value")
			}
			i++
			n := 0
			if _, serr := fmt.Sscanf(args[i], "%d", &n); serr != nil || n < 1 {
				return nil, nil, 0, fmt.Errorf("bad --max-parallel %q — expected a positive integer", args[i])
			}
			maxParallel = n
		default:
			return nil, nil, 0, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return repos, issues, maxParallel, nil
}

func validRepoSlug(r string) bool {
	owner, name, ok := strings.Cut(r, "/")
	return ok && owner != "" && name != "" && strings.Count(r, "/") == 1
}

// parseIssueTarget reads `owner/name#N`. N must be a positive decimal integer and the repo a
// well-formed slug; anything else is refused, never half-parsed into a different address.
func parseIssueTarget(v string) (issueTarget, error) {
	v = strings.TrimSpace(v)
	repo, num, ok := strings.Cut(v, "#")
	if !ok || !validRepoSlug(repo) {
		return issueTarget{}, fmt.Errorf("bad --issue %q — expected owner/name#N", v)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 || strconv.Itoa(n) != num {
		return issueTarget{}, fmt.Errorf("bad --issue %q — the issue number must be a positive integer", v)
	}
	return issueTarget{Repo: repo, Number: n}, nil
}

// --- the JSON contract -------------------------------------------------------------------

// Envelope is what a consumer parses. Schema is checked FIRST by the consumer; everything else
// is data. Repos and Partial are disjoint by construction.
type Envelope struct {
	Schema  int           `json:"schema"`
	Kind    string        `json:"kind"`
	Repos   []RepoResult  `json:"repos"`
	Partial []PartialRepo `json:"partial"`
}

// RepoResult is one repo that WAS read. An empty Issues slice is a real answer — "this repo has
// no open issues" — which is precisely the answer a PartialRepo entry does NOT make.
type RepoResult struct {
	Repo   string      `json:"repo"`
	Issues []IssueJSON `json:"issues"`
}

// PartialRepo is one repo that was NOT read, and why. It never carries a data field: there is no
// shape in which an unread repo also has a (possibly empty) result.
type PartialRepo struct {
	Repo   string `json:"repo"`
	Reason string `json:"reason"`
}

// IssueJSON is deskkit.IssueSummary rendered for the wire. It carries the backend-independent fields
// the interface already serves and adds nothing of its own.
type IssueJSON struct {
	Number      int      `json:"number"`
	Title       string   `json:"title,omitempty"`
	State       string   `json:"state"`
	AuthorLogin string   `json:"authorLogin,omitempty"`
	AuthorID    int64    `json:"authorId,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	URL         string   `json:"url,omitempty"`
}

// readSet performs one read kind across the whole repo set concurrently, bounded by maxParallel.
// Results are re-sorted into the caller's repo ORDER afterwards so the envelope is deterministic
// — a JSON contract whose element order depends on which goroutine finished first is a contract
// no test can pin.
func readSet(kind string, repos []string, maxParallel int) Envelope {
	type slot struct {
		res  *RepoResult
		part *PartialRepo
	}
	slots := make([]slot, len(repos))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, r := range repos {
		wg.Add(1)
		go func(i int, repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			issues, err := readIssues(repo)
			if err != nil {
				slots[i].part = &PartialRepo{Repo: repo, Reason: err.Error()}
				return
			}
			slots[i].res = &RepoResult{Repo: repo, Issues: issues}
		}(i, r)
	}
	wg.Wait()

	env := Envelope{Schema: envelopeSchema, Kind: kind, Repos: []RepoResult{}, Partial: []PartialRepo{}}
	for _, s := range slots {
		switch {
		case s.res != nil:
			env.Repos = append(env.Repos, *s.res)
		case s.part != nil:
			env.Partial = append(env.Partial, *s.part)
		}
	}
	return env
}

// readIssues is the `issues` kind: ONE enumerated operation, no client-side filtering that could
// turn a read failure into an empty answer.
func readIssues(repo string) ([]IssueJSON, error) {
	f, fr, err := forgeFor(repo)
	if err != nil {
		return nil, err
	}
	sums, lerr := f.ListOpenIssues(fr)
	if lerr != nil {
		return nil, lerr
	}
	out := make([]IssueJSON, 0, len(sums))
	for _, s := range sums {
		out = append(out, IssueJSON{
			Number:      s.Number,
			Title:       s.Title,
			State:       "open", // ListOpenIssues serves open issues by definition.
			AuthorLogin: s.Author.Login,
			AuthorID:    s.Author.ID,
			Labels:      s.Labels,
			CreatedAt:   s.CreatedAt,
			URL:         s.URL,
		})
	}
	return out, nil
}

// --- the per-issue kinds (trust, comments) --------------------------------------------------

// ItemEnvelope is the per-issue kinds' contract. It is a SEPARATE shape from Envelope so the
// `issues` kind's pinned output does not move: a per-issue read answers under "items", keyed by
// repo AND number, and its "partial" entries carry the number too. Items and Partial are
// disjoint by construction, exactly as Repos and Partial are.
type ItemEnvelope struct {
	Schema  int           `json:"schema"`
	Kind    string        `json:"kind"`
	Items   []ItemResult  `json:"items"`
	Partial []PartialItem `json:"partial"`
}

// ItemResult is one issue that WAS read. Exactly one of Trust / Comments is set, by kind. An
// empty Comments list is a real answer — "nobody has commented" — which is why it is a pointer:
// it renders as [] when read, and is absent only on the other kind.
type ItemResult struct {
	Repo     string         `json:"repo"`
	Number   int            `json:"number"`
	Trust    *TrustJSON     `json:"trust,omitempty"`
	Comments *[]CommentJSON `json:"comments,omitempty"`
}

// PartialItem is one issue that was NOT read, and why. It never carries a data field.
type PartialItem struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Reason string `json:"reason"`
}

// TrustJSON is deskkit.TrustPayload rendered for the wire. Times are RFC3339 with sub-second
// precision kept (the blessing rule voids a blessing on a same-instant tie, so rounding here
// could turn a void into a pass); a zero time renders as "".
type TrustJSON struct {
	BodyEditedAt string      `json:"bodyEditedAt,omitempty"`
	Complete     bool        `json:"complete"`
	Events       []EventJSON `json:"events"`
}

// EventJSON is one deskkit.ContentEvent. AuthorLogin is the rendered form the trust set expects
// (an App re-suffixed "<slug>[bot]"); an empty login is a deleted account and stays empty.
type EventJSON struct {
	AuthorLogin string `json:"authorLogin,omitempty"`
	AuthorID    int64  `json:"authorId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	EditedAt    string `json:"editedAt,omitempty"`
}

// CommentJSON is one deskkit.Comment in the fields an issue-thread consumer reads: who, when,
// and the body (the un-block lane checks it for the desk-automation marker).
type CommentJSON struct {
	AuthorLogin string `json:"authorLogin,omitempty"`
	AuthorID    int64  `json:"authorId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	Body        string `json:"body"`
}

func runPerIssue(kind string, repos []string, issues []issueTarget, maxPar int, stdout, stderr io.Writer) int {
	if len(repos) > 0 {
		fmt.Fprintf(stderr, "deskread: refused: --repo does not address the %q kind — it reads one issue at a time (--issue owner/name#N)\n", kind)
		return deskkit.ExitRefused
	}
	if len(issues) == 0 {
		fmt.Fprintf(stderr, "deskread: refused: no --issue given — an empty set is never reported as an empty result\n")
		return deskkit.ExitRefused
	}
	env := readItems(kind, issues, maxPar)
	enc, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "deskread: could-not-check: rendering the envelope failed: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	fmt.Fprintln(stdout, string(enc))
	if len(env.Items) == 0 {
		fmt.Fprintf(stderr, "deskread: could-not-check: no issue in the set of %d could be read\n", len(issues))
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// readItems performs one per-issue kind across the set concurrently, bounded by maxParallel, and
// re-sorts into the caller's order — the same determinism readSet keeps.
func readItems(kind string, issues []issueTarget, maxParallel int) ItemEnvelope {
	type slot struct {
		res  *ItemResult
		part *PartialItem
	}
	slots := make([]slot, len(issues))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, tgt := range issues {
		wg.Add(1)
		go func(i int, tgt issueTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := readItem(kind, tgt)
			if err != nil {
				slots[i].part = &PartialItem{Repo: tgt.Repo, Number: tgt.Number, Reason: err.Error()}
				return
			}
			slots[i].res = res
		}(i, tgt)
	}
	wg.Wait()

	env := ItemEnvelope{Schema: envelopeSchema, Kind: kind, Items: []ItemResult{}, Partial: []PartialItem{}}
	for _, s := range slots {
		switch {
		case s.res != nil:
			env.Items = append(env.Items, *s.res)
		case s.part != nil:
			env.Partial = append(env.Partial, *s.part)
		}
	}
	return env
}

// readItem is ONE enumerated operation per kind. No client-side filtering: a read failure is an
// error (→ partial), never an empty answer.
func readItem(kind string, tgt issueTarget) (*ItemResult, error) {
	f, fr, err := forgeFor(tgt.Repo)
	if err != nil {
		return nil, err
	}
	res := &ItemResult{Repo: tgt.Repo, Number: tgt.Number}
	switch kind {
	case "trust":
		tp, terr := f.IssueTrustEvents(fr, tgt.Number)
		if terr != nil {
			return nil, terr
		}
		if tp == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no trust payload for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		tj := &TrustJSON{BodyEditedAt: wireTime(tp.BodyEdited), Complete: tp.Complete, Events: []EventJSON{}}
		for _, e := range tp.Events {
			tj.Events = append(tj.Events, EventJSON{
				AuthorLogin: e.Author, AuthorID: e.AuthorID,
				CreatedAt: wireTime(e.CreatedAt), EditedAt: wireTime(e.EditedAt),
			})
		}
		res.Trust = tj
	case "comments":
		cs, cerr := f.ListCommentsTyped(fr, tgt.Number, deskkit.TargetIssue)
		if cerr != nil {
			return nil, cerr
		}
		out := make([]CommentJSON, 0, len(cs))
		for _, c := range cs {
			out = append(out, CommentJSON{
				AuthorLogin: c.Author.Login, AuthorID: c.Author.ID,
				CreatedAt: c.CreatedAt, Body: c.Body,
			})
		}
		res.Comments = &out
	default:
		return nil, deskkit.Refused("unknown per-issue kind " + kind)
	}
	return res, nil
}

func wireTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
