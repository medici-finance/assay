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
// enumerates with both backends implemented and golden-pinned. The freeze rule (an added op needs
// a consuming call site in the same change) is satisfied by CONSUMING the surface, not widening
// it: `issues` is deskkit.Forge.ListOpenIssues and nothing else.
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
// flags or an unknown kind) · 6 unverifiable (no repo could be read).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// envelopeSchema is the version of the JSON contract below. A consumer checks it and REFUSES an
// unrecognised value rather than best-effort parsing — the same fail-closed direction statusgen's
// own brief parser already takes on an unknown brief schema. Bump it only for a change a pinned
// consumer could not read correctly; adding an omitempty field is not such a change.
const envelopeSchema = 1

// readKinds is the CLOSED set of reads this verb serves. It is a set of KINDS, never an address:
// there is no --query, --endpoint or --path flag, and adding one would reopen exactly the
// passthrough the forge-surface control forbids. Slice 1 lands `issues`; each further kind is
// added with the call site that consumes it.
var readKinds = map[string]string{
	"issues": "open issues per repo (deskkit.Forge.ListOpenIssues)",
}

var usage = `deskread — read-only forge reads on the seam, as JSON

usage:
  deskread issues --repo <owner/repo> [--repo <owner/repo> …]
  deskread --version

flags:
  --repo <owner/repo>  repeatable; ONE invocation serves the whole set, read concurrently
  --max-parallel <N>   bounded concurrency for the set (default 6)

output: a versioned JSON envelope on stdout

  {"schema": 1, "kind": "issues",
   "repos":   [{"repo": "o/n", "issues": [{"number": …, "state": "open", …}]}],
   "partial": [{"repo": "o/n", "reason": "…"}]}

A repo that could not be read lands in "partial" with its reason and is ABSENT from "repos";
the exit code is still 0. That is deliberate: a caller must be able to tell "no open issues"
from "could not look". Exit 6 only when NO repo in the set could be read.

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

	repos, maxPar, ferr := parseFlags(args[1:])
	if ferr != nil {
		fmt.Fprintf(stderr, "deskread: refused: %v\n", ferr)
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

// parseFlags reads the repeatable --repo set and the concurrency bound. It is hand-rolled rather
// than flag.FlagSet because --repo repeats, and it REFUSES an unknown flag rather than ignoring
// it: a typo'd flag that silently does nothing is how a caller ends up reading a narrower set
// than it asked for and never finds out.
func parseFlags(args []string) (repos []string, maxParallel int, err error) {
	maxParallel = 6
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return nil, 0, fmt.Errorf("--repo needs a value")
			}
			i++
			r := strings.TrimSpace(args[i])
			if _, _, ok := strings.Cut(r, "/"); !ok || strings.Count(r, "/") != 1 {
				return nil, 0, fmt.Errorf("bad --repo %q — expected owner/name", r)
			}
			if !seen[r] {
				seen[r] = true
				repos = append(repos, r)
			}
		case "--max-parallel":
			if i+1 >= len(args) {
				return nil, 0, fmt.Errorf("--max-parallel needs a value")
			}
			i++
			n := 0
			if _, serr := fmt.Sscanf(args[i], "%d", &n); serr != nil || n < 1 {
				return nil, 0, fmt.Errorf("bad --max-parallel %q — expected a positive integer", args[i])
			}
			maxParallel = n
		default:
			return nil, 0, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return repos, maxParallel, nil
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
