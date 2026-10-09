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
// ITS MIGRATIONS ADD NO OPERATION TO Forge. The kinds statusgen's first migrations consume map
// onto operations the interface already enumerated with both backends: `issues` is
// deskkit.Forge.ListOpenIssues, `trust` is deskkit.Forge.IssueTrustEvents, and `comments` is
// deskkit.Forge.ListCommentsTyped on the issue or change target its address flag names. The
// reads statusgen's remaining sites needed and the frozen surface lacked were added SEPARATELY,
// under the freeze rule, by forge-neutral brief 33: ops 55-58 (ListIssues, IssueStateEvents,
// ListChangeCommits, RepoDefaultBranch) and their result fields, with this verb's kinds as their
// consuming call sites — `issue-list` (op 55), `issue-states` (op 56), `change-commits` (op 57),
// `default-branch` (op 58), plus `issue`, `changes`, `change` and `change-files`, which carry the
// new result fields of existing operations. Each new kind is waiting on statusgen's migration
// (forge-neutral brief 18). forge-neutral brief 35 consumes `issue` and the new `comments` fields
// in the ruling resolver, the new `comments` fields and change target in the transcribe lanes'
// sign-off resolver, and `issue` with `trust` in the verdict-issue read. Every kind is TRANSPORT:
// it carries the forge's answer, absent values stay absent, and no kind maps an author type or an
// edit time to an accept-or-refuse outcome — that mapping belongs to the consumer.
//
// THREE ADDRESSING SHAPES. Per-REPO kinds (`issues`, `issue-list`, `changes`, `default-branch`)
// take --repo. Per-ISSUE kinds (`trust`, `issue`, `issue-states`) take --issue owner/name#N, and
// per-CHANGE kinds (`change`, `change-commits`, `change-files`) take --change owner/name#N.
// `comments` takes EXACTLY ONE of --issue and --change: the caller names the target, the verb
// never guesses it from the number and never retries under the other target, because on a forge
// with separate issue and change number sequences (GitLab) the same number names two different
// threads. `trust` and `comments --issue` are what statusgen's --scan-issues trust gate and
// un-block lane consume, one issue at a time, in place of the `gh api graphql` / `gh api
// --paginate` shell-outs that 401'd under a replaced HOME. Each kind accepts exactly its own
// address flag and refuses the others, so a caller can never ask a per-issue kind for a whole
// repo or the other way round; --state and --label are accepted only by the kinds that define them.
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
// trust gate and un-block lane; the forge-neutral brief 33 kinds: statusgen's remaining sites,
// per forge-neutral briefs 18 and 35).
var readKinds = map[string]string{
	"issues":         "open issues per repo (deskkit.Forge.ListOpenIssues)",
	"trust":          "an issue's trust-gate content events, one bounded page (deskkit.Forge.IssueTrustEvents)",
	"comments":       "an issue's or a change's whole comment thread, oldest first (deskkit.Forge.ListCommentsTyped, on the target the address flag names)",
	"issue-list":     "issues of a stated state, optionally one label, with Incomplete on overflow (deskkit.Forge.ListIssues)",
	"issue":          "one issue: state, body, author with type, closer (deskkit.Forge.GetIssueTyped, issue kind)",
	"issue-states":   "an issue's close/reopen events and closing changes (deskkit.Forge.IssueStateEvents)",
	"changes":        "changes of a stated state with author, base, fork facts (deskkit.Forge.ListChanges)",
	"change":         "one change, with its merge-commit SHA (deskkit.Forge.GetPullRequest)",
	"change-commits": "a change's own commit SHAs, with Complete (deskkit.Forge.ListChangeCommits)",
	"change-files":   "a change's file list with patches, reconciled against its file count (deskkit.Forge.ListChangedFiles + GetPullRequest)",
	"default-branch": "a repository's default branch (deskkit.Forge.RepoDefaultBranch)",
}

// repoKinds are addressed by --repo; issueKinds by --issue; changeKinds by --change. `comments`
// is in BOTH issueKinds and changeKinds and must be given exactly one of the two.
var (
	repoKinds   = map[string]bool{"issues": true, "issue-list": true, "changes": true, "default-branch": true}
	issueKinds  = map[string]bool{"trust": true, "comments": true, "issue": true, "issue-states": true}
	changeKinds = map[string]bool{"comments": true, "change": true, "change-commits": true, "change-files": true}
)

// stateWords is the CLOSED --state vocabulary per kind that takes one. A kind absent from this
// map refuses --state; a word absent from its kind's set is refused (exit 5), never defaulted.
var stateWords = map[string][]string{
	"issue-list": {deskkit.IssueStateOpen, deskkit.IssueStateClosed, deskkit.IssueStateAll},
	"changes":    {"merged", "closed", "all"},
}

var usage = `deskread — read-only forge reads on the seam, as JSON

usage:
  deskread issues         --repo   <owner/repo>   [--repo   <owner/repo> …]
  deskread issue-list     --repo   <owner/repo> … [--state open|closed|all] [--label <name>]
  deskread changes        --repo   <owner/repo> …  --state merged|closed|all
  deskread default-branch --repo   <owner/repo> …
  deskread trust          --issue  <owner/repo#N> [--issue  <owner/repo#N> …]
  deskread issue          --issue  <owner/repo#N> …
  deskread issue-states   --issue  <owner/repo#N> …
  deskread comments       --issue  <owner/repo#N> …   (exactly one of --issue / --change)
  deskread comments       --change <owner/repo#N> …
  deskread change         --change <owner/repo#N> …
  deskread change-commits --change <owner/repo#N> …
  deskread change-files   --change <owner/repo#N> …
  deskread --version

flags:
  --repo <owner/repo>      repeatable (per-repo kinds); ONE invocation serves the whole set, read concurrently
  --issue <owner/repo#N>   repeatable (trust, issue, issue-states, comments); one entry per issue
  --change <owner/repo#N>  repeatable (change, change-commits, change-files, comments); one entry per change
  --state <word>           issue-list (open|closed|all, default open) and changes (merged|closed|all, required);
                           any other word, or --state on another kind, is refused
  --label <name>           issue-list only; ONE label (a comma is refused — the forge reads it as a list)
  --max-parallel <N>       bounded concurrency for the set (default 6)
  --ci-workflow-token      read as the CI job's own workflow token instead of an App role (see identity)

output: a versioned JSON envelope on stdout

  {"schema": 1, "kind": "issues",
   "repos":   [{"repo": "o/n", "issues": [{"number": …, "state": "open", …}]}],
   "partial": [{"repo": "o/n", "reason": "…"}]}

  per-repo kinds answer under "repos": issue-list adds "incomplete" and "pageCap" beside
  "issues" (each issue also carries "closedAt" once closed); changes answers "changes" with
  "incomplete" and "pageCap"; default-branch answers "defaultBranch".

  {"schema": 1, "kind": "trust",
   "items":   [{"repo": "o/n", "number": 7, "trust": {"bodyEditedAt": "…", "complete": true,
               "events": [{"authorLogin": …, "authorId": …, "createdAt": …, "editedAt": …}]}}],
   "partial": [{"repo": "o/n", "number": 7, "reason": "…"}]}

  {"schema": 1, "kind": "comments",
   "items":   [{"repo": "o/n", "number": 7, "comments": [{"authorLogin": …, "authorId": …,
               "createdAt": …, "body": …, "databaseId": …, "authorType": …, "url": …,
               "updatedAt": …}]}],
   "partial": [{"repo": "o/n", "number": 7, "reason": "…"}]}

  per-issue and per-change kinds answer under "items", one key per kind: "issue",
  "issueStates", "change", "changeCommits", "changeFiles".

A repo (or issue, or change) that could not be read lands in "partial" with its reason and is
ABSENT from "repos" ("items"); the exit code is still 0. That is deliberate: a caller must be
able to tell "no open issues" / "no comments" from "could not look". Exit 6 only when NOTHING in
the set could be read. A trust read's "complete": false means the thread overflowed the single
bounded page — the caller fails that issue closed; it is an answer, not a partial. The same holds
for every "complete": false and "incomplete": true the other kinds carry.

Absent is never a value: an empty "authorType", "updatedAt", "closedBy", "mergeCommitSha",
"crossRepo" or "headRepo" is could-not-check for the consumer, never "User", "unedited", "no
closer", "no merge commit" or "same repository". A file with "patchAbsent": true has a patch
the forge did not serve, never an empty one. No kind maps any of these to an outcome.

identity: two transports, named in the envelope's "identity" object and on one stderr line.
  app-custody (the default): reads authenticate as this session's minted App role via the deskkit
  resolver. There is no ambient-credential fallback: a session with no resolvable role is
  refused, never silently degraded onto whatever the local forge CLI happens to hold.
  ci-workflow-token (opt-in): --ci-workflow-token alone selects it. It is CI-only (refused
  outside a CI job and on pull_request_target), read-only (every write method is refused),
  same-repository (only the job's own GITHUB_REPOSITORY is read; another --repo lands in
  "partial" and a --issue/--change in another repository is refused) and limited to the kinds
  issues, trust and comments. The token is read from DESKREAD_CI_WORKFLOW_TOKEN only and must be
  an installation token (ghs_); without the flag that variable is ignored. A refusal reads
  "deskread: refused [ci-transport:<layer>]: <reason>" and exits 5. The identity object's
  repository and runId are copied from the environment and are NOT verified: CI detection is a
  guard against misuse, not proof of a CI job.
`

func main() {
	// The custody path reads the roster to resolve its acting role, so its class is
	// ciEligible=false: config-home file only, never the environment, in CI as well as locally.
	// Only the explicit --ci-workflow-token opt-in changes that (toolClassFor).
	deskkit.SetToolClass(toolClassFor(os.Args[1:]))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// ciTransportKinds is the CLOSED set of read kinds the CI workflow-token transport serves. It is
// separate from readKinds on purpose: a kind added to readKinds is NOT served under the CI
// transport until a reviewed diff adds it here (forge-neutral brief 18 owns each addition its
// CI-lane sites need). TestCITransportKindsAreReads pins that every entry is also a readKinds kind.
var ciTransportKinds = map[string]bool{"issues": true, "trust": true, "comments": true}

// toolClassFor is the one place deskread picks its roster class. The CI class (ClassForTool(true),
// which is ClassCI only inside CI) is chosen ONLY when the arguments parse cleanly and carry the
// --ci-workflow-token opt-in as a flag of its own, never as the value of another flag. Everything
// else, including a malformed command line, is today's ClassForTool(false): config-home file only.
// Under the CI transport deskread resolves no acting role, so this does not loosen the rule that
// acting tools stay file-only.
func toolClassFor(args []string) deskkit.ToolClass {
	if len(args) >= 2 {
		if o, err := parseFlags(args[1:]); err == nil && o.ci {
			return deskkit.ClassForTool(true)
		}
	}
	return deskkit.ClassForTool(false)
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

	o, ferr := parseFlags(args[1:])
	if ferr != nil {
		fmt.Fprintf(stderr, "deskread: refused: %v\n", ferr)
		return deskkit.ExitRefused
	}
	// Every refusal below happens BEFORE any forge is resolved, so a refused run makes zero
	// forge calls.
	ce := readCIEnv()
	if o.ci {
		tr, layer, reason := ciGate(kind, ce)
		if tr == nil {
			return ciRefuse(stderr, layer, reason)
		}
		o.ciT = tr
	} else if ce.token != "" {
		// The opt-in is the flag alone: a token that is merely present is never used.
		fmt.Fprintf(stderr, "deskread: %s is set but ignored: --ci-workflow-token was not given, so the App custody path runs\n", ciTokenEnv)
	}
	if rerr := checkAddressing(kind, &o); rerr != nil {
		fmt.Fprintf(stderr, "deskread: refused: %v\n", rerr)
		return deskkit.ExitRefused
	}
	if o.ciT != nil {
		if layer, reason := ciGateTargets(kind, o); layer != "" {
			return ciRefuse(stderr, layer, reason)
		}
	}
	resetCustodyRole()
	if !repoKinds[kind] {
		return runPerItem(kind, o, stdout, stderr)
	}

	env := readSet(kind, o)
	env.Identity = identityFor(o)
	writeIdentityLine(stderr, env.Identity)
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
		fmt.Fprintf(stderr, "deskread: could-not-check: no repo in the set of %d could be read\n", len(o.repos))
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// checkAddressing refuses every flag the kind does not define, and an empty address set. For the
// per-item kinds it settles the ONE target the run reads (o.target, o.items): `comments` with both
// --issue and --change, or with neither, is refused — the caller names the target.
func checkAddressing(kind string, o *readOpts) error {
	if words, takes := stateWords[kind]; takes {
		if o.state == "" && kind == "issue-list" {
			o.state = deskkit.IssueStateOpen
		}
		if o.state == "" {
			return fmt.Errorf("the %q kind needs --state (%s) — the state set is stated, never defaulted", kind, strings.Join(words, "|"))
		}
		if !contains(words, o.state) {
			return fmt.Errorf("unknown --state %q for the %q kind (want %s)", o.state, kind, strings.Join(words, "|"))
		}
	} else if o.stateSet {
		return fmt.Errorf("--state does not apply to the %q kind", kind)
	}
	if o.labelSet && kind != "issue-list" {
		return fmt.Errorf("--label does not apply to the %q kind", kind)
	}
	if o.labelSet {
		if strings.TrimSpace(o.label) == "" {
			return fmt.Errorf("--label needs a non-empty value")
		}
		if strings.Contains(o.label, ",") {
			return fmt.Errorf("--label %q contains a comma, which the forge reads as a list of labels — one label only", o.label)
		}
	}

	if repoKinds[kind] {
		if len(o.issues) > 0 || len(o.changes) > 0 {
			return fmt.Errorf("--issue/--change do not address the %q kind — it reads whole repos (--repo)", kind)
		}
		if len(o.repos) == 0 {
			return fmt.Errorf("no --repo given — an empty set is never reported as an empty result")
		}
		return nil
	}
	if len(o.repos) > 0 {
		return fmt.Errorf("--repo does not address the %q kind — it reads one item at a time (%s)", kind, addrFlags(kind))
	}
	switch {
	case len(o.issues) > 0 && len(o.changes) > 0:
		return fmt.Errorf("the %q kind takes exactly one of --issue and --change, never both — the caller names the target", kind)
	case len(o.issues) > 0:
		if !issueKinds[kind] {
			return fmt.Errorf("--issue does not address the %q kind — it reads changes (--change owner/name#N)", kind)
		}
		o.target, o.items = deskkit.TargetIssue, o.issues
	case len(o.changes) > 0:
		if !changeKinds[kind] {
			return fmt.Errorf("--change does not address the %q kind — it reads issues (--issue owner/name#N)", kind)
		}
		o.target, o.items = deskkit.TargetChange, o.changes
	default:
		return fmt.Errorf("no %s given — an empty set is never reported as an empty result", addrFlags(kind))
	}
	return nil
}

func addrFlags(kind string) string {
	switch {
	case issueKinds[kind] && changeKinds[kind]:
		return "--issue or --change owner/name#N"
	case changeKinds[kind]:
		return "--change owner/name#N"
	}
	return "--issue owner/name#N"
}

func contains(set []string, w string) bool {
	for _, s := range set {
		if s == w {
			return true
		}
	}
	return false
}

func sortedKinds() []string {
	out := make([]string, 0, len(readKinds))
	for k := range readKinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// issueTarget is one --issue or --change address: a repo coordinate plus a number.
type issueTarget struct {
	Repo   string
	Number int
}

// readOpts is everything parseFlags read. target/items are settled by checkAddressing.
type readOpts struct {
	repos       []string
	issues      []issueTarget
	changes     []issueTarget
	state       string
	stateSet    bool
	label       string
	labelSet    bool
	maxParallel int

	target deskkit.TargetKind
	items  []issueTarget

	// ci is the --ci-workflow-token opt-in as parsed; ciT is the transport the gate settled (nil
	// on the custody path). The token lives only inside ciT and is never rendered.
	ci  bool
	ciT *ciTransport
}

// parseFlags reads the repeatable --repo, --issue and --change sets, the --state and --label
// filters and the concurrency bound. It is hand-rolled rather than flag.FlagSet because the
// address flags repeat, and it REFUSES an unknown flag rather than ignoring it: a typo'd flag that
// silently does nothing is how a caller ends up reading a narrower set than it asked for and never
// finds out. Which flags a kind accepts is checked afterwards, against the kind (checkAddressing).
func parseFlags(args []string) (readOpts, error) {
	o := readOpts{maxParallel: 6}
	seen := map[string]bool{}
	seenIssue := map[issueTarget]bool{}
	seenChange := map[issueTarget]bool{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--repo needs a value")
			}
			i++
			r := strings.TrimSpace(args[i])
			if !validRepoSlug(r) {
				return o, fmt.Errorf("bad --repo %q — expected owner/name", r)
			}
			if !seen[r] {
				seen[r] = true
				o.repos = append(o.repos, r)
			}
		case "--issue", "--change":
			flag := args[i]
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s needs a value", flag)
			}
			i++
			tgt, terr := parseItemTarget(flag, args[i])
			if terr != nil {
				return o, terr
			}
			if flag == "--issue" {
				if !seenIssue[tgt] {
					seenIssue[tgt] = true
					o.issues = append(o.issues, tgt)
				}
			} else if !seenChange[tgt] {
				seenChange[tgt] = true
				o.changes = append(o.changes, tgt)
			}
		case "--state":
			if i+1 >= len(args) || o.stateSet {
				return o, fmt.Errorf("--state needs exactly one value")
			}
			i++
			o.state, o.stateSet = strings.TrimSpace(args[i]), true
		case "--label":
			if i+1 >= len(args) || o.labelSet {
				return o, fmt.Errorf("--label needs exactly one value")
			}
			i++
			o.label, o.labelSet = args[i], true
		case ciWorkflowTokenFlag:
			o.ci = true
		case "--max-parallel":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--max-parallel needs a value")
			}
			i++
			n := 0
			if _, serr := fmt.Sscanf(args[i], "%d", &n); serr != nil || n < 1 {
				return o, fmt.Errorf("bad --max-parallel %q — expected a positive integer", args[i])
			}
			o.maxParallel = n
		default:
			return o, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return o, nil
}

// validRepoSlug is deskkit.ValidRepoSlug, the same check the CI-token constructor re-runs, so the
// verb's argument check and the constructor's cannot drift apart.
func validRepoSlug(r string) bool { return deskkit.ValidRepoSlug(r) }

// parseItemTarget reads `owner/name#N` for --issue or --change. N must be a positive decimal
// integer and the repo a well-formed slug; anything else is refused, never half-parsed into a
// different address.
func parseItemTarget(flag, v string) (issueTarget, error) {
	v = strings.TrimSpace(v)
	repo, num, ok := strings.Cut(v, "#")
	if !ok || !validRepoSlug(repo) {
		return issueTarget{}, fmt.Errorf("bad %s %q — expected owner/name#N", flag, v)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 || strconv.Itoa(n) != num {
		return issueTarget{}, fmt.Errorf("bad %s %q — the number must be a positive integer", flag, v)
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
	// Identity says which transport authenticated the reads (omitempty and additive: schema
	// stays 1, and a consumer that declares only the fields it reads never sees it). It never
	// carries a credential.
	Identity *IdentityJSON `json:"identity,omitempty"`
}

// RepoResult is one repo that WAS read. An empty Issues slice is a real answer — "this repo has
// no open issues" — which is precisely the answer a PartialRepo entry does NOT make. Exactly one
// payload is set per kind: Issues (issues, issue-list), Changes (changes), DefaultBranch
// (default-branch). Incomplete and PageCap are set by the bounded list kinds (issue-list,
// changes) and absent on the others.
type RepoResult struct {
	Repo          string       `json:"repo"`
	Issues        []IssueJSON  `json:"issues"`
	Incomplete    *bool        `json:"incomplete,omitempty"`
	PageCap       int          `json:"pageCap,omitempty"`
	Changes       []ChangeJSON `json:"changes,omitempty"`
	DefaultBranch string       `json:"defaultBranch,omitempty"`
}

// MarshalJSON keeps the `issues` kind's pinned output byte-identical — "issues" is ALWAYS present
// on it, as [] when empty — while a kind that carries no issue list omits the key rather than
// rendering a misleading `"issues": null`. Issues is nil exactly when the kind has no issue list;
// the issue-listing kinds always set a non-nil slice.
func (r RepoResult) MarshalJSON() ([]byte, error) {
	type wire struct {
		Repo          string        `json:"repo"`
		Issues        *[]IssueJSON  `json:"issues,omitempty"`
		Incomplete    *bool         `json:"incomplete,omitempty"`
		PageCap       int           `json:"pageCap,omitempty"`
		Changes       *[]ChangeJSON `json:"changes,omitempty"`
		DefaultBranch string        `json:"defaultBranch,omitempty"`
	}
	w := wire{Repo: r.Repo, Incomplete: r.Incomplete, PageCap: r.PageCap, DefaultBranch: r.DefaultBranch}
	if r.Issues != nil {
		w.Issues = &r.Issues
	}
	if r.Changes != nil {
		w.Changes = &r.Changes
	}
	return json.Marshal(w)
}

// PartialRepo is one repo that was NOT read, and why. It never carries a data field: there is no
// shape in which an unread repo also has a (possibly empty) result.
type PartialRepo struct {
	Repo   string `json:"repo"`
	Reason string `json:"reason"`
}

// IssueJSON is deskkit.IssueSummary rendered for the wire. It carries the backend-independent fields
// the interface already serves and adds nothing of its own. ClosedAt and AuthorType are set by
// `issue-list` only (op 55), and only where the forge reported them.
type IssueJSON struct {
	Number      int      `json:"number"`
	Title       string   `json:"title,omitempty"`
	State       string   `json:"state"`
	AuthorLogin string   `json:"authorLogin,omitempty"`
	AuthorID    int64    `json:"authorId,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	URL         string   `json:"url,omitempty"`
	ClosedAt    string   `json:"closedAt,omitempty"`
	AuthorType  string   `json:"authorType,omitempty"`
}

// AccountJSON is one deskkit.Account. An empty Type is could-not-check, never "User"; a zero ID
// is an account the forge did not report.
type AccountJSON struct {
	Login string `json:"login,omitempty"`
	ID    int64  `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
}

func accountJSON(a deskkit.Account) *AccountJSON {
	if a.Login == "" && a.ID == 0 && a.Type == "" {
		return nil
	}
	return &AccountJSON{Login: a.Login, ID: a.ID, Type: a.Type}
}

// ChangeJSON is one deskkit.ChangeRef (the `changes` kind). Every forge-neutral brief 33 field is
// omitempty and empty where the forge did not establish it.
type ChangeJSON struct {
	Number    int          `json:"number"`
	State     string       `json:"state"`
	HeadSHA   string       `json:"headSha,omitempty"`
	HeadRef   string       `json:"headRef,omitempty"`
	Title     string       `json:"title,omitempty"`
	Body      string       `json:"body,omitempty"`
	MergedAt  string       `json:"mergedAt,omitempty"`
	Author    *AccountJSON `json:"author,omitempty"`
	BaseRef   string       `json:"baseRef,omitempty"`
	CrossRepo string       `json:"crossRepo,omitempty"`
	HeadRepo  string       `json:"headRepo,omitempty"`
}

// readSet performs one per-repo kind across the whole repo set concurrently, bounded by
// maxParallel. Results are re-sorted into the caller's repo ORDER afterwards so the envelope is
// deterministic — a JSON contract whose element order depends on which goroutine finished first
// is a contract no test can pin.
func readSet(kind string, o readOpts) Envelope {
	type slot struct {
		res  *RepoResult
		part *PartialRepo
	}
	slots := make([]slot, len(o.repos))
	sem := make(chan struct{}, o.maxParallel)
	var wg sync.WaitGroup
	for i, r := range o.repos {
		wg.Add(1)
		go func(i int, repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := readRepo(kind, repo, o)
			if err != nil {
				slots[i].part = &PartialRepo{Repo: repo, Reason: err.Error()}
				return
			}
			slots[i].res = res
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

// readRepo is ONE enumerated operation per per-repo kind, no client-side filtering that could
// turn a read failure into an empty answer.
func readRepo(kind, repo string, o readOpts) (*RepoResult, error) {
	f, fr, err := forgeForRun(repo, o)
	if err != nil {
		return nil, err
	}
	res := &RepoResult{Repo: repo}
	switch kind {
	case "issues":
		sums, lerr := f.ListOpenIssues(fr)
		if lerr != nil {
			return nil, lerr
		}
		res.Issues = make([]IssueJSON, 0, len(sums))
		for _, s := range sums {
			res.Issues = append(res.Issues, IssueJSON{
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
	case "issue-list":
		l, lerr := f.ListIssues(fr, deskkit.IssueListQuery{State: o.state, Label: o.label})
		if lerr != nil {
			return nil, lerr
		}
		if l == nil {
			return nil, deskkit.Unverifiable("the forge returned no issue list for "+repo, nil)
		}
		res.Issues = make([]IssueJSON, 0, len(l.Issues))
		for _, s := range l.Issues {
			res.Issues = append(res.Issues, IssueJSON{
				Number: s.Number, Title: s.Title, State: s.State,
				AuthorLogin: s.Author.Login, AuthorID: s.Author.ID, AuthorType: s.Author.Type,
				Labels: s.Labels, CreatedAt: s.CreatedAt, ClosedAt: s.ClosedAt, URL: s.URL,
			})
		}
		inc := l.Incomplete
		res.Incomplete, res.PageCap = &inc, l.PageCap
	case "changes":
		cl, lerr := f.ListChanges(fr, changeStates(o.state))
		if lerr != nil {
			return nil, lerr
		}
		if cl == nil {
			return nil, deskkit.Unverifiable("the forge returned no change list for "+repo, nil)
		}
		res.Changes = make([]ChangeJSON, 0, len(cl.Changes))
		for _, c := range cl.Changes {
			res.Changes = append(res.Changes, ChangeJSON{
				Number: c.Number, State: c.State, HeadSHA: c.HeadSHA, HeadRef: c.HeadRef,
				Title: c.Title, Body: c.Body, MergedAt: c.MergedAt, Author: accountJSON(c.Author),
				BaseRef: c.BaseRef, CrossRepo: c.CrossRepo, HeadRepo: c.HeadRepo,
			})
		}
		inc := cl.Incomplete
		res.Incomplete, res.PageCap = &inc, cl.PageCap
	case "default-branch":
		b, berr := f.RepoDefaultBranch(fr)
		if berr != nil {
			return nil, berr
		}
		if b == "" {
			// The seam already refuses an empty branch; this is the verb's own floor, so an
			// empty answer can never render as a repo with no default branch.
			return nil, deskkit.Unverifiable("the forge reported no default branch for "+repo, nil)
		}
		res.DefaultBranch = b
	default:
		return nil, deskkit.Refused("unknown per-repo kind " + kind)
	}
	return res, nil
}

// changeStates maps the `changes` kind's closed --state vocabulary onto the seam's state set.
// `closed` is closed-UNMERGED (the seam keeps MERGED distinct from CLOSED); `all` is all three.
func changeStates(word string) deskkit.ChangeStates {
	switch word {
	case "merged":
		return deskkit.ChangeStates{Merged: true}
	case "closed":
		return deskkit.ChangeStates{Closed: true}
	case "all":
		return deskkit.ChangeStates{Open: true, Merged: true, Closed: true}
	}
	return deskkit.ChangeStates{} // unreachable: checkAddressing refused the word; the seam refuses an empty set
}

// --- the per-item kinds ----------------------------------------------------------------------

// ItemEnvelope is the per-item kinds' contract. It is a SEPARATE shape from Envelope so the
// `issues` kind's pinned output does not move: a per-item read answers under "items", keyed by
// repo AND number, and its "partial" entries carry the number too. Items and Partial are
// disjoint by construction, exactly as Repos and Partial are.
type ItemEnvelope struct {
	Schema  int           `json:"schema"`
	Kind    string        `json:"kind"`
	Items   []ItemResult  `json:"items"`
	Partial []PartialItem `json:"partial"`
	// Identity is the same additive record Envelope carries.
	Identity *IdentityJSON `json:"identity,omitempty"`
}

// ItemResult is one issue or change that WAS read. Exactly one payload is set, by kind. An
// empty Comments list is a real answer — "nobody has commented" — which is why it is a pointer:
// it renders as [] when read, and is absent only on the other kinds.
type ItemResult struct {
	Repo          string             `json:"repo"`
	Number        int                `json:"number"`
	Trust         *TrustJSON         `json:"trust,omitempty"`
	Comments      *[]CommentJSON     `json:"comments,omitempty"`
	Issue         *IssueDetailJSON   `json:"issue,omitempty"`
	IssueStates   *IssueStatesJSON   `json:"issueStates,omitempty"`
	Change        *ChangeDetailJSON  `json:"change,omitempty"`
	ChangeCommits *ChangeCommitsJSON `json:"changeCommits,omitempty"`
	ChangeFiles   *ChangeFilesJSON   `json:"changeFiles,omitempty"`
}

// PartialItem is one issue or change that was NOT read, and why. It never carries a data field.
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

// CommentJSON is one deskkit.Comment in the fields a thread consumer reads: who, when, and the
// body (the un-block lane checks it for the desk-automation marker). DatabaseID, AuthorType, URL
// and UpdatedAt (forge-neutral brief 33) are the fields the sign-off and ruling resolvers select
// and pin on; each is omitempty, so a comment that carries none of them renders byte-identical to
// the output before they existed. An empty AuthorType or UpdatedAt is could-not-check, never
// "User" or "unedited".
type CommentJSON struct {
	AuthorLogin string `json:"authorLogin,omitempty"`
	AuthorID    int64  `json:"authorId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	Body        string `json:"body"`
	DatabaseID  int64  `json:"databaseId,omitempty"`
	AuthorType  string `json:"authorType,omitempty"`
	URL         string `json:"url,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// IssueDetailJSON is one deskkit.Issue read through GetIssueTyped(issue kind). ClosedBy is set
// only while the issue is closed and the forge named the closer; absent is could-not-check.
type IssueDetailJSON struct {
	State    string       `json:"state"`
	Title    string       `json:"title,omitempty"`
	Body     string       `json:"body,omitempty"`
	URL      string       `json:"url,omitempty"`
	Labels   []string     `json:"labels,omitempty"`
	Author   *AccountJSON `json:"author,omitempty"`
	ClosedBy *AccountJSON `json:"closedBy,omitempty"`
}

// IssueStatesJSON is deskkit.IssueStateHistory. Complete=false: a connection still paginated or
// a closing change's merged state was unreadable, so every "merged" below is could-not-check.
type IssueStatesJSON struct {
	Complete       bool                `json:"complete"`
	Events         []StateEventJSON    `json:"events"`
	ClosingChanges []ClosingChangeJSON `json:"closingChanges"`
}

// StateEventJSON is one close or reopen.
type StateEventJSON struct {
	Kind      string       `json:"kind"`
	Actor     *AccountJSON `json:"actor,omitempty"`
	CreatedAt string       `json:"createdAt,omitempty"`
}

// ClosingChangeJSON is one change the forge records as closing the issue. Repo is the change's
// own owner/name, empty where unresolvable.
type ClosingChangeJSON struct {
	Repo   string       `json:"repo,omitempty"`
	Number int          `json:"number"`
	Merged bool         `json:"merged"`
	Author *AccountJSON `json:"author,omitempty"`
}

// ChangeDetailJSON is one deskkit.PullRequest. MergeCommitSHA is set only for a merged change.
type ChangeDetailJSON struct {
	State          string       `json:"state"`
	Draft          bool         `json:"draft,omitempty"`
	Merged         bool         `json:"merged,omitempty"`
	MergedAt       string       `json:"mergedAt,omitempty"`
	MergeCommitSHA string       `json:"mergeCommitSha,omitempty"`
	HeadSHA        string       `json:"headSha,omitempty"`
	HeadRef        string       `json:"headRef,omitempty"`
	BaseRef        string       `json:"baseRef,omitempty"`
	CrossRepo      string       `json:"crossRepo,omitempty"`
	ChangedFiles   int          `json:"changedFiles"`
	Title          string       `json:"title,omitempty"`
	Body           string       `json:"body,omitempty"`
	URL            string       `json:"url,omitempty"`
	Author         *AccountJSON `json:"author,omitempty"`
}

// ChangeCommitsJSON is deskkit.ChangeCommits. Complete=false: a SHA missing from the list is
// could-not-check.
type ChangeCommitsJSON struct {
	Complete bool     `json:"complete"`
	SHAs     []string `json:"shas"`
}

// ChangeFilesJSON is ListChangedFiles reconciled against the change's OWN file count, as the
// seam's ListChangedFiles contract asks of its caller: Complete is true only when the listed
// count equals ChangedFiles, so a list cut short by a page ceiling never reads as the whole.
type ChangeFilesJSON struct {
	Complete     bool       `json:"complete"`
	ChangedFiles int        `json:"changedFiles"`
	Files        []FileJSON `json:"files"`
}

// FileJSON is one deskkit.ChangedFile. PatchAbsent is always rendered: true means the forge
// served no patch (binary, oversized, collapsed), never an empty one.
type FileJSON struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previousFilename,omitempty"`
	Status           string `json:"status,omitempty"`
	Patch            string `json:"patch,omitempty"`
	PatchAbsent      bool   `json:"patchAbsent"`
}

func runPerItem(kind string, o readOpts, stdout, stderr io.Writer) int {
	env := readItems(kind, o)
	env.Identity = identityFor(o)
	writeIdentityLine(stderr, env.Identity)
	enc, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "deskread: could-not-check: rendering the envelope failed: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	fmt.Fprintln(stdout, string(enc))
	if len(env.Items) == 0 {
		fmt.Fprintf(stderr, "deskread: could-not-check: no item in the set of %d could be read\n", len(o.items))
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// readItems performs one per-item kind across the set concurrently, bounded by maxParallel, and
// re-sorts into the caller's order — the same determinism readSet keeps.
func readItems(kind string, o readOpts) ItemEnvelope {
	type slot struct {
		res  *ItemResult
		part *PartialItem
	}
	slots := make([]slot, len(o.items))
	sem := make(chan struct{}, o.maxParallel)
	var wg sync.WaitGroup
	for i, tgt := range o.items {
		wg.Add(1)
		go func(i int, tgt issueTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := readItem(kind, o.target, tgt, o)
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

// readItem is ONE enumerated operation per kind (change-files: the file list plus the change's
// own count it is reconciled against). No client-side filtering: a read failure is an error (→
// partial), never an empty answer — and never a retry under another target.
func readItem(kind string, target deskkit.TargetKind, tgt issueTarget, o readOpts) (*ItemResult, error) {
	f, fr, err := forgeForRun(tgt.Repo, o)
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
		// The target is the one the caller's address flag named (checkAddressing). It is passed
		// through as given: no inference from the number, no second read under the other target.
		cs, cerr := f.ListCommentsTyped(fr, tgt.Number, target)
		if cerr != nil {
			return nil, cerr
		}
		out := make([]CommentJSON, 0, len(cs))
		for _, c := range cs {
			out = append(out, CommentJSON{
				AuthorLogin: c.Author.Login, AuthorID: c.Author.ID,
				CreatedAt: c.CreatedAt, Body: c.Body,
				DatabaseID: c.DatabaseID, AuthorType: c.Author.Type, URL: c.URL, UpdatedAt: c.UpdatedAt,
			})
		}
		res.Comments = &out
	case "issue":
		iss, ierr := f.GetIssueTyped(fr, tgt.Number, deskkit.TargetIssue)
		if ierr != nil {
			return nil, ierr
		}
		if iss == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no issue for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		if iss.IsPullRequest {
			// A change answered under the issue kind is a kind mismatch, never an item.
			return nil, deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s#%d is a change, not an issue", tgt.Repo, tgt.Number), nil)
		}
		res.Issue = &IssueDetailJSON{
			State: iss.State, Title: iss.Title, Body: iss.Body, URL: iss.URL, Labels: iss.Labels,
			Author: accountJSON(iss.Author), ClosedBy: accountJSON(iss.ClosedBy),
		}
	case "issue-states":
		h, herr := f.IssueStateEvents(fr, tgt.Number)
		if herr != nil {
			return nil, herr
		}
		if h == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no state history for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		sj := &IssueStatesJSON{Complete: h.Complete, Events: []StateEventJSON{}, ClosingChanges: []ClosingChangeJSON{}}
		for _, e := range h.Events {
			sj.Events = append(sj.Events, StateEventJSON{Kind: e.Kind, Actor: accountJSON(e.Actor), CreatedAt: e.CreatedAt})
		}
		for _, c := range h.ClosingChanges {
			sj.ClosingChanges = append(sj.ClosingChanges, ClosingChangeJSON{
				Repo: c.Repo, Number: c.Number, Merged: c.Merged, Author: accountJSON(c.Author),
			})
		}
		res.IssueStates = sj
	case "change":
		pr, perr := f.GetPullRequest(fr, tgt.Number)
		if perr != nil {
			return nil, perr
		}
		if pr == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no change for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		res.Change = &ChangeDetailJSON{
			State: pr.State, Draft: pr.Draft, Merged: pr.Merged, MergedAt: pr.MergedAt,
			MergeCommitSHA: pr.MergeCommitSHA, HeadSHA: pr.HeadSHA, HeadRef: pr.HeadRef,
			BaseRef: pr.BaseRef, CrossRepo: pr.CrossRepo, ChangedFiles: pr.ChangedFiles,
			Title: pr.Title, Body: pr.Body, URL: pr.URL, Author: accountJSON(pr.Author),
		}
	case "change-commits":
		cc, cerr := f.ListChangeCommits(fr, tgt.Number)
		if cerr != nil {
			return nil, cerr
		}
		if cc == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no commit list for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		shas := cc.SHAs
		if shas == nil {
			shas = []string{}
		}
		res.ChangeCommits = &ChangeCommitsJSON{Complete: cc.Complete, SHAs: shas}
	case "change-files":
		pr, perr := f.GetPullRequest(fr, tgt.Number)
		if perr != nil {
			return nil, perr
		}
		if pr == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("the forge returned no change for %s#%d", tgt.Repo, tgt.Number), nil)
		}
		files, ferr := f.ListChangedFiles(fr, tgt.Number)
		if ferr != nil {
			return nil, ferr
		}
		fj := &ChangeFilesJSON{
			Complete:     len(files) == pr.ChangedFiles,
			ChangedFiles: pr.ChangedFiles,
			Files:        make([]FileJSON, 0, len(files)),
		}
		for _, cf := range files {
			fj.Files = append(fj.Files, FileJSON{
				Filename: cf.Filename, PreviousFilename: cf.PreviousFilename, Status: cf.Status,
				Patch: cf.Patch, PatchAbsent: cf.PatchAbsent,
			})
		}
		res.ChangeFiles = fj
	default:
		return nil, deskkit.Refused("unknown per-item kind " + kind)
	}
	return res, nil
}

func wireTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
