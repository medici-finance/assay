package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskprStderr is the seam for warnIfConflicting's advisory output. Production writes to
// os.Stderr; tests redirect it to a buffer to assert on the warning text without
// capturing the real process stream.
var deskprStderr io.Writer = os.Stderr

// pollAttempts / pollSleep pace warnIfConflicting's wait for GitHub's asynchronous
// `mergeable` computation to settle out of UNKNOWN (#1264). GitHub returns UNKNOWN for a
// just-created/updated PR until a background job computes the test-merge, so a single read
// can miss a real CONFLICTING as a transient UNKNOWN. They are package vars so tests can
// shrink the sleep to a no-op and set a deterministic attempt count.
var (
	pollAttempts = 6
	pollSleep    = func() { time.Sleep(700 * time.Millisecond) }
)

// getwd is the seam for the tool's working directory (the worktree it runs in).
// Production uses os.Getwd; tests point it at a scratch worktree fixture without
// os.Chdir (which would race parallel processes).
var getwd = os.Getwd

const maxBodyBytes = 16 * 1024 // body cap

var (
	baseRe = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)
	pullRe = regexp.MustCompile(`/pull/(\d+)`)
)

// gitFacts is the positively-verified state preflight establishes before any write.
type gitFacts struct {
	dir           string
	branch        string
	defaultBranch string
	defaultRef    string // fully-qualified remote-tracking ref, e.g. "refs/remotes/origin/main" (unambiguous by construction, #840)
	repo          string // owner/name
	head          string // HEAD sha
}

// ghPR is the slice of a PR deskpr needs from `gh pr list --json`.
type ghPR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
}

// auditCtx accumulates the fields for the ONE audit line every invocation emits
// (audit every path). finalize is deferred so exactly one line is written no
// matter which branch returns.
type auditCtx struct {
	verb          string
	repo          string
	pr            *int
	head          string
	detail        string
	successResult string // ResultOK unless a noop set it to ResultNoop
}

func (a *auditCtx) log(result, detail string) {
	var headp *string
	if a.head != "" {
		h := a.head
		headp = &h
	}
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "deskpr",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		PR:         a.pr,
		HeadSHA:    headp,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	if err == nil {
		result := a.successResult
		if result == "" {
			result = deskkit.ResultOK
		}
		a.log(result, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		result = deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// cmdCreate implements `deskpr create`. Flow: verify
// preconditions → secret-scan → idempotency → push (plain, never --force) →
// `gh pr create --draft` → print URL.
func cmdCreate(args []string) (err error) {
	ac := &auditCtx{verb: "create"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress flag's own output; we craft messages
	title := fs.String("title", "", "PR title (required)")
	bodyFile := fs.String("body-file", "", "path to a file containing the PR body")
	bodyMin := fs.String("body-min", "", "one-line PR body (alternative to --body-file)")
	base := fs.String("base", "main", "base branch")
	root := fs.String("root", ".", "repo root the Brief: trailer resolves against (docs/streams under it)")
	asApp := fs.Bool("as-app", true, "authenticate as this session's App role via desktoken (worker by default; the verifier App under DESK_LOOP=verify-desk, etc.); --as-app=false for example-org fallback")
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, surface digest, reason, identity)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: create takes no positional arguments")
	}
	// Validate the override BEFORE anything else runs, so a malformed one refuses in
	// milliseconds rather than after a token mint and a remote read.
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}
	if strings.TrimSpace(*title) == "" {
		return deskkit.Refused("refused: --title is required")
	}
	if (*bodyFile == "") == (*bodyMin == "") {
		return deskkit.Refused("refused: provide exactly one of --body-file or --body-min")
	}
	if !baseRe.MatchString(*base) {
		return deskkit.Refused("refused: --base must be a plain branch name")
	}
	body, berr := readBody(*bodyFile, *bodyMin)
	if berr != nil {
		return berr
	}
	if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
		Tool: "deskpr", Verb: "create", Reason: *scanOverride,
		Surface: "PR body", Content: body,
	}, deskkit.ScanSurface("PR body", body)); serr != nil {
		return serr
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}

	// example-stream/02: the PR→brief link is a data edge, not a convention. Refuse a
	// body without exactly one trailer — BEFORE any network call (getwd/preflight are
	// local; token mint and PR listing come after) — and resolve the brief under --root.
	trailerIssue, terr := requireTrailer(body, *root, dir)
	if terr != nil {
		return terr
	}
	facts, perr := preflight(dir, *base)
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	// seatbelt: scan title, branch, and the diff-vs-default before any push.
	if scanErr := scanWrite(facts, *title, "create", *scanOverride); scanErr != nil {
		return scanErr
	}

	// #203: the PUBLIC-REPO SELF-CONTAINMENT scan. It runs HERE rather than beside the
	// secret scan above because it needs the target repo, which only preflight establishes
	// — the secret scan's question ("is there a credential in this text") is repo-
	// independent, this one is not. It is a no-op on a known-private repo and on an
	// unconfigured roster (deskkit.SelfContainApplies), so the create path on a private
	// repo is byte-for-byte what it was.
	//
	// The verdict routes through HandleScanRefusal like every other scan on this path, so
	// the refusal advertises the SAME audited override rather than introducing a second
	// bypass a worker would have to learn — and an override taken here writes its row
	// before the push.
	scOpts := deskkit.SelfContainOpts{Repo: facts.repo, NumberHint: trailerIssue}
	for _, sc := range []struct {
		surface string
		content []byte
	}{
		{"PR body", body},
		{"PR title", []byte(*title)},
	} {
		if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: "create", Repo: facts.repo, Reason: *scanOverride,
			Surface: sc.surface, Content: sc.content,
		}, deskkit.SelfContainCheck(sc.surface, sc.content, scOpts)); serr != nil {
			return serr
		}
	}

	// --as-app defaults to true: mint/reuse the worker App installation
	// token so every subsequent gh invocation (list, create) authenticates as
	// the worker App instead of the ambient example-org identity. Pass
	// --as-app=false during the transition for the example-org fallback.
	requireWorkerAuth = *asApp
	if *asApp {
		if merr := mintWorkerToken(facts.repo); merr != nil {
			return deskkit.Unverifiable("cannot mint worker token for --as-app", merr)
		}
	}

	// idempotency (#140/#148 duplicate-PR class): an open PR already on this head
	// branch → print its URL, noop, exit 0. Checked BEFORE any push or create.
	prs, lerr := listOpenPRs(facts.dir, facts.repo, facts.branch)
	if lerr != nil {
		return deskkit.Unverifiable("cannot list existing PRs for the branch", lerr)
	}
	if pr := matchHead(prs, facts.branch); pr != nil {
		ac.pr = &pr.Number
		ac.successResult = deskkit.ResultNoop
		ac.detail = "open PR already exists " + pr.URL
		fmt.Printf("noop: open PR already exists for %s: %s\n", facts.branch, pr.URL)
		return nil
	}

	// Outward-write rate limit — checked immediately before the push.
	//
	// REPO-WIDE, at the per-PR cap, because a create's own audit line records the number of
	// the PR it is about to make (`ac.pr = &n`, below) — a new number every time. Scoping
	// the gate on a PR number therefore reads a bucket this call site can never fill: an
	// earlier fix aimed it at the repo's UNNUMBERED bucket, which left creates on the
	// 100/hr per-repo tier while looking like a 10/hr cap, and inverted the meter so that
	// only FAILED creates (which record no number) accumulated (#439, third
	// review). The repo-wide scope counts every line this tool writes on this repo, so the
	// bucket the gate reads is a superset of wherever the write lands — see
	// deskkit.AllowWriteRepoWide.
	//
	// Held at the per-PR cap, not the per-repo one: this is the verb behind the
	// PR-flood risk and it was hard-capped at 10/hr before the tiers existed.
	if werr := deskkit.AllowWriteRepoWide("deskpr", facts.repo); werr != nil {
		return werr
	}

	// Public-repo gate: refuse to write to a public repo
	// without a qualifying +1 from an authorized human.
	// A create has no PR number yet, so the gate is asked about the trailer's
	// tracking issue instead: `trailerIssue` is the `Issue: #<N>` number, or 0
	// for a `Brief:` trailer (a brief resolves to a file, not a reactions
	// surface). On a non-blessed public repo this gives the `Issue:` path the
	// per-issue-+1 admission — a +1 from the blessing authority on that issue
	// admits the create — while a `Brief:` create still fails closed (issue 0,
	// no reactions surface) with exit 6 (#1707). This does not touch the
	// blessed-repo path: a repo carrying a standing per-repo authorization
	// (deskkit publicbless.go: a human-maintained sentinel file naming exact
	// repos) passes the gate regardless of the number, with a stderr NOTICE,
	// and create proceeds. The change never relaxes the gate — it only routes
	// the issue number the create already required to the surface that checks it.
	owner, name := splitOwnerRepo(facts.repo)
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name, trailerIssue); gerr != nil {
		return gerr
	}

	// Plain push. argv is constructed literally: no --force / --force-with-lease can
	// ever be emitted, and no caller flag is forwarded to git.
	if _, pushErr := git(facts.dir, "push", "-u", "origin", facts.branch); pushErr != nil {
		return deskkit.Unverifiable("git push failed", pushErr)
	}

	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return deskkit.Unverifiable("cannot stage PR body", terr)
	}
	defer cleanup()

	// --draft is hardcoded and unconditional. There is no flag to omit it.
	out, cErr := gh(facts.dir, "pr", "create", "-R", facts.repo, "--draft",
		"--head", facts.branch, "--base", *base,
		"--title", *title, "--body-file", bodyPath)
	if cErr != nil {
		return deskkit.Unverifiable("gh pr create failed", cErr)
	}
	url := lastURL(out)
	detail := "created " + url
	if n := prNumberFromURL(url); n > 0 {
		ac.pr = &n
		// Post-create mergeable check (#770): a PR GitHub reports CONFLICTING gets zero
		// pull_request runs at its head — indistinguishable, on the audit line or any
		// board, from "checks still pending" until something names the mergeable state
		// specifically. Advisory only: this can never turn a create that already
		// succeeded into a reported failure.
		detail += warnIfConflicting(facts.dir, facts.repo, n)
	}
	ac.detail = detail
	fmt.Println(url)
	return nil
}

// warnIfConflicting reads a just-created PR's mergeable status via `gh pr view` and
// prints a loud WARNING to deskprStderr when GitHub reports CONFLICTING, returning a
// non-empty suffix for the audit detail line in that case (empty otherwise, including on
// a read/parse failure — see below).
//
// Why this matters (#770): GitHub skips `pull_request` Actions runs on a PR whose merge
// state is CONFLICTING, so a conflicted PR sits at zero check-runs indefinitely. That
// zero reads identically to "checks haven't started yet" on the pr-review-desk and any
// board, so a conflicted PR silently stalls — exactly what happened to #749/#750, which
// were correct code stuck only on a rebase no one was told to do.
//
// GitHub computes `mergeable` asynchronously, so a freshly created/updated PR reports
// UNKNOWN until a background test-merge settles (#1264). A single read taken too early
// would miss a real CONFLICTING as a transient UNKNOWN, so this polls briefly
// (pollAttempts reads, pollSleep between) for the field to leave UNKNOWN before deciding.
//
// Advisory only: a transient `gh pr view` failure, an unparseable payload, or a value
// that never settles out of UNKNOWN must never turn an already-successful create/update
// into a reported failure, so every non-CONFLICTING path here is a stderr note (or
// silence), not a returned error — the PR already exists by the time this runs.
func warnIfConflicting(dir, repo string, prNum int) string {
	var v struct {
		Mergeable        string `json:"mergeable"`
		MergeStateStatus string `json:"mergeStateStatus"`
	}
	for attempt := 0; ; attempt++ {
		out, err := gh(dir, "pr", "view", strconv.Itoa(prNum), "-R", repo,
			"--json", "mergeable,mergeStateStatus")
		if err != nil {
			fmt.Fprintf(deskprStderr, "deskpr: WARNING could not read mergeable status for %s#%d — %v\n", repo, prNum, err)
			return ""
		}
		v = struct {
			Mergeable        string `json:"mergeable"`
			MergeStateStatus string `json:"mergeStateStatus"`
		}{}
		if uerr := json.Unmarshal([]byte(out), &v); uerr != nil {
			fmt.Fprintf(deskprStderr, "deskpr: WARNING unparseable mergeable status for %s#%d — %v\n", repo, prNum, uerr)
			return ""
		}
		// Settled (MERGEABLE / CONFLICTING) or out of attempts: stop polling.
		if v.Mergeable != "UNKNOWN" || attempt >= pollAttempts-1 {
			break
		}
		pollSleep()
	}
	if v.Mergeable == "UNKNOWN" {
		fmt.Fprintf(deskprStderr, "deskpr: WARNING mergeable status for %s#%d did not settle out of UNKNOWN after "+
			"%d polls — GitHub had not finished computing it; re-check the PR's merge state before relying on CI (#1264)\n",
			repo, prNum, pollAttempts)
		return ""
	}
	if v.Mergeable != "CONFLICTING" {
		return ""
	}
	fmt.Fprintf(deskprStderr, "deskpr: WARNING %s#%d is CONFLICTING (mergeStateStatus=%s) — GitHub will not run "+
		"pull_request checks at this head; merge/rebase the base branch into this PR before expecting CI to fire "+
		"(#770)\n", repo, prNum, v.MergeStateStatus)
	return fmt.Sprintf(" — CONFLICTING (mergeStateStatus=%s), CI will not run until resolved", v.MergeStateStatus)
}

// cmdUpdate implements `deskpr update`: a follow-up push of the current branch to its
// EXISTING open PR — draft OR ready-flipped (the fix→re-review hot path, and #788's
// keep-an-approved-PR-current path). It refuses if no open PR exists for the branch;
// that same refusal covers closed and merged PRs, because the listing is --state open.
// The draft/ready distinction is intentionally NOT a gate: the author-owns-branch (head
// match), bodycheck, rate-limit and public-repo guards apply identically either way. No
// PR is ever created here.
func cmdUpdate(args []string) (err error) {
	ac := &auditCtx{verb: "update"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	asApp := fs.Bool("as-app", true, "authenticate as this session's App role via desktoken (worker by default; the verifier App under DESK_LOOP=verify-desk, etc.); --as-app=false for example-org fallback")
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, surface digest, reason, identity)")
	root := fs.String("root", ".", "repo root the Brief: trailer resolves against (docs/streams under it)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: update takes no arguments")
	}
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	// update has no --base flag: it refreshes an EXISTING PR's body, so the ahead-count
	// stays pinned to the repo default (origin/HEAD) exactly as before — an empty base
	// selects that default inside preflight.
	facts, perr := preflight(dir, "")
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	if scanErr := scanWrite(facts, "", "update", *scanOverride); scanErr != nil {
		return scanErr
	}

	requireWorkerAuth = *asApp
	if *asApp {
		if merr := mintWorkerToken(facts.repo); merr != nil {
			return deskkit.Unverifiable("cannot mint worker token for --as-app", merr)
		}
	}

	prs, lerr := listOpenPRs(facts.dir, facts.repo, facts.branch)
	if lerr != nil {
		return deskkit.Unverifiable("cannot list PRs for the branch", lerr)
	}
	pr := matchHead(prs, facts.branch)
	if pr == nil {
		return deskkit.Refused("refused: no open PR for " + facts.branch + " — run `deskpr create` first")
	}
	// #788: a ready-flipped (non-draft) OPEN PR is accepted here too. The draft-only
	// refusal that used to sit here was an artifact of update being written for the
	// draft-PR workflow first; it left approved PRs that go stale against a moving main
	// with no sanctioned push path. Every other guard (head-owns-branch above, bodycheck
	// in scanWrite, AllowWrite rate limit, and the public-repo gate below) is unchanged,
	// so lifting the draft distinction does not widen what update may push — only which
	// open PR states it will push to.
	ac.pr = &pr.Number

	// example-stream/02: an update pushes to a PR whose BODY lives on GitHub, so the
	// trailer check reads it from the PR. A PR whose body lacks the link line refuses
	// here (exit 5, message names the line to add) — the worker edits the body, then
	// re-runs update. This is the migration-window behavior for pre-trailer PRs.
	var prView struct {
		Body string `json:"body"`
	}
	bOut, berr := gh(facts.dir, "pr", "view", strconv.Itoa(pr.Number), "-R", facts.repo, "--json", "body")
	if berr != nil {
		return deskkit.Unverifiable("cannot read PR body for trailer check", berr)
	}
	if uerr := json.Unmarshal([]byte(bOut), &prView); uerr != nil {
		return deskkit.Unverifiable("cannot parse PR body for trailer check", uerr)
	}
	// update ignores the trailer's issue number: the gate below is asked about the
	// PR being updated (pr.Number), which is the reactions surface for an update.
	if _, terr := requireTrailer([]byte(prView.Body), *root, dir); terr != nil {
		return terr
	}

	// idempotency: this exact head already pushed to this PR → noop.
	if deskkit.AlreadyDone(facts.repo, pr.Number, facts.head, "update") {
		ac.successResult = deskkit.ResultNoop
		ac.detail = "head already pushed to " + pr.URL
		fmt.Printf("noop: %s already pushed to %s\n", shortSHA(facts.head), pr.URL)
		return nil
	}

	if werr := deskkit.AllowWrite("deskpr", facts.repo, pr.Number); werr != nil {
		return werr
	}

	// Public-repo gate: refuse to update a PR on a public repo
	// without a qualifying +1 from an authorized human on the associated issue.
	owner, name := splitOwnerRepo(facts.repo)
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name, pr.Number); gerr != nil {
		return gerr
	}

	if _, pushErr := git(facts.dir, "push", "-u", "origin", facts.branch); pushErr != nil {
		return deskkit.Unverifiable("git push failed", pushErr)
	}
	// Post-update mergeable check (#1264): the push moved the head, so GitHub recomputes
	// mergeability. A push that lands the PR in CONFLICTING gets zero pull_request runs at
	// the new head — the same silent stall the create path guards against — so warn loudly
	// here too. Advisory only: this can never turn an already-completed push into a failure.
	detail := "pushed to " + pr.URL
	detail += warnIfConflicting(facts.dir, facts.repo, pr.Number)
	ac.detail = detail
	fmt.Println(pr.URL)
	return nil
}

// preflight verifies the preconditions IN-TOOL and returns the positively-verified
// facts. Ordering matters: the default-branch refusal fires on branch name alone
// (exit 5) BEFORE origin/HEAD is consulted, so a missing origin/HEAD can never mask a
// push to main; a detached HEAD or unreadable origin/HEAD is unverifiable (exit 6).
func preflight(dir, base string) (*gitFacts, error) {
	if out, err := git(dir, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
		return nil, deskkit.Unverifiable("not inside a git worktree", err)
	}
	branch, err := git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve current branch", err)
	}
	if branch == "HEAD" || branch == "" {
		return nil, deskkit.Unverifiable("detached HEAD — check out a feature branch first", nil)
	}
	// Refuse the default branch names outright, even if origin/HEAD is unreadable.
	if isDefaultName(branch) {
		return nil, deskkit.Refused("refused: on the default branch (" + branch + ") — deskpr only pushes feature branches")
	}
	// Resolve origin/HEAD to its FULLY-QUALIFIED target (no --short). A stray local branch
	// literally named `origin/main` (the `deskwt --branch origin/main` gotcha) makes the
	// short name `origin/main` ambiguous: `symbolic-ref --short` then disambiguates its
	// output to `remotes/origin/main`, which `TrimPrefix(…, "origin/")` cannot strip, so the
	// old `"origin/" + defaultBranch` produced the unresolvable `origin/remotes/origin/main`
	// and every rev-list/diff below aborted exit 128 (#840). The un-shortened target
	// `refs/remotes/origin/main` is unambiguous by construction, so derive the branch name
	// AND the base ref from it and use that fully-qualified ref everywhere downstream.
	defOut, derr := git(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if derr != nil {
		return nil, deskkit.Unverifiable(
			"cannot read origin/HEAD (default branch unverifiable) — run `git remote set-head origin --auto`", derr)
	}
	defaultRef := strings.TrimSpace(defOut)
	defaultBranch := strings.TrimPrefix(defaultRef, "refs/remotes/origin/")
	if defaultBranch == "" || defaultBranch == defaultRef {
		return nil, deskkit.Unverifiable("origin/HEAD resolved to an unexpected target: "+defaultRef, nil)
	}
	if branch == defaultBranch {
		return nil, deskkit.Refused("refused: on the default branch (" + branch + ")")
	}

	originURL, oerr := git(dir, "config", "--get", "remote.origin.url")
	if oerr != nil {
		return nil, deskkit.Unverifiable("cannot read remote.origin.url", oerr)
	}
	repo, rerr := parseRepo(originURL)
	if rerr != nil {
		return nil, deskkit.Unverifiable("cannot parse origin repo from "+originURL, rerr)
	}
	if !deskkit.IsAllowedRepo(repo) {
		return nil, deskkit.Refused("refused: origin " + repo + " is not in the desk-tools repo set")
	}

	// No staged-but-uncommitted changes: `git diff --cached --quiet` exits 1 when the
	// index has content not yet committed.
	if _, serr := git(dir, "diff", "--cached", "--quiet"); serr != nil {
		if code, ok := exitCode(serr); ok && code == 1 {
			return nil, deskkit.Refused("refused: staged-but-uncommitted changes — commit them first")
		}
		return nil, deskkit.Unverifiable("cannot check staged changes", serr)
	}

	// Count "commits ahead" against the base the PR will ACTUALLY open against — the
	// caller's --base (`gh pr create --base <base>`), resolved to its fully-qualified
	// remote-tracking ref. Counting against origin/HEAD instead (the repo default) is a
	// bug for any --base other than the default branch: a branch legitimately ahead of
	// its intended base — e.g. a stacked PR whose base is another feature branch — but
	// NOT ahead of the default branch was false-refused as "no commits ahead", forcing
	// the caller onto the ambient-identity `gh pr create` fallback that mis-attributes
	// the PR author (#55). An empty base (the `update` verb, which has no --base and must
	// stay pinned to origin/HEAD as before) keeps the repo default.
	baseRef := defaultRef
	if base != "" {
		baseRef = "refs/remotes/origin/" + base
	}
	// The base must have a resolvable remote-tracking ref. Without this, a missing/
	// unfetched base ref aborts `git rev-list` at exit 128 and reads as unverifiable — but
	// we surface it with a precise, actionable message rather than a bare count failure.
	if _, verr := git(dir, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}"); verr != nil {
		return nil, deskkit.Unverifiable(
			"base ref "+baseRef+" does not resolve — fetch the base branch (`git fetch origin`) first", verr)
	}
	cnt, cerr := git(dir, "rev-list", "--count", baseRef+"..HEAD")
	if cerr != nil {
		return nil, deskkit.Unverifiable("cannot count commits ahead of "+baseRef, cerr)
	}
	if cnt == "0" {
		return nil, deskkit.Refused("refused: branch has no commits ahead of " + baseRef)
	}

	head, herr := git(dir, "rev-parse", "HEAD")
	if herr != nil {
		return nil, deskkit.Unverifiable("cannot resolve HEAD sha", herr)
	}
	return &gitFacts{
		dir: dir, branch: branch, defaultBranch: defaultBranch,
		defaultRef: defaultRef, repo: repo, head: head,
	}, nil
}

// scanWrite runs the shared secret scan over the title (when set), the branch name, and
// the diff-vs-default before any push (a best-effort seatbelt; the committed-code
// residual is out of scope here).
//
// Each surface is scanned through deskkit.ScanSurface with its own NAME, so a refusal
// says which of the three fired (#328): all three used to report as "body",
// and a refusal that misidentifies its own surface sends the operator to rewrite text
// that was never the problem.
//
// override (#585): every surface here routes its verdict through
// deskkit.HandleScanRefusal, so a refusal on ANY of the three advertises the audited
// bypass and, when one is supplied, records exactly WHICH surface was waved through. The
// branch diff is the surface that matters most in practice — it is the one carrying
// go.sum blocks, lockfile digests and pre-existing content the branch cannot edit away.
func scanWrite(f *gitFacts, title, verb, override string) error {
	scanWith := func(surface string, scanner func(string, []byte) error, content []byte) error {
		return deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: verb, Repo: f.repo, Reason: override,
			Surface: surface, Content: content,
		}, scanner(surface, content))
	}
	if title != "" {
		if err := scanWith("PR title", deskkit.ScanSurface, []byte(title)); err != nil {
			return err
		}
	}
	if err := scanWith("branch name", deskkit.ScanSurface, []byte(f.branch)); err != nil {
		return err
	}
	diff, err := git(f.dir, "diff", f.defaultRef+"...HEAD")
	if err != nil {
		return deskkit.Unverifiable("cannot compute diff vs "+f.defaultRef+" for the secret scan", err)
	}
	// #1052 (second vector): git-diff header lines (`diff --git a/<path> b/<path>`,
	// `index …`, `--- a/<path>`, `+++ b/<path>`, `rename from/to …`, `similarity index …`)
	// are tool-generated, not author content — but a repo path routinely runs ≥32 chars
	// of deskkit's [A-Za-z0-9+/=] base64ish charset (e.g. `a/tools/desk/internal/deskkit/
	// config.go`), so BodyCheck's high-entropy-run check refused diffs touching those
	// files. Strip ONLY the strictly-matched meta lines before scanning; every content
	// line (context, `+` added, `-` removed) still goes through the secret arms in full,
	// so detection strength on author-written content is unchanged. Gated here at the
	// diff-scanning callsite, not inside deskkit.BodyCheck — BodyCheck is generic (also
	// used verbatim on PR bodies/comments/reviews) and must not grow diff-format
	// awareness.
	//
	// The diff surface is scanned in TWO passes because its two checks need DIFFERENT
	// line directions (see deskkit.ScanSurfaceSecrets):
	//
	//   - the SECRET arms read the whole stripped diff — added, removed and context
	//     lines alike. A credential on a removed line is still in the repository's
	//     history, and a banner or Secret manifest already on origin arrives on exactly
	//     this surface; that breadth is long-standing and deliberate.
	//   - the impersonated-human-ruling guard reads the ADDED lines only. A deletion
	//     cannot introduce a forged ruling — only added text can claim a human's voice —
	//     and scanning removed lines false-positived on retirement branches whose whole
	//     point was to DELETE old attribution lines ("Ruling: … — <name>" and kin).
	//     Because that guard is non-overridable by design, the false positive was a
	//     hard stop with no audited way through. Narrowing it to the added direction
	//     removes that class while catching an added forged attribution exactly as
	//     before.
	if err := scanWith("branch diff vs "+f.defaultRef, deskkit.ScanSurfaceSecrets,
		[]byte(stripDiffMetaLines(diff))); err != nil {
		return err
	}
	if err := scanWith("added lines of branch diff vs "+f.defaultRef, deskkit.ScanSurfaceRulingClaim,
		[]byte(addedDiffLines(diff))); err != nil {
		return err
	}
	return nil
}

// reHunkHeader matches the deterministic RANGE part of a unified-diff hunk header —
// `@@ -27,5 +27,5 @@` — and nothing after it. Group 1 is the part that is kept; whatever
// follows on the line is git's funcname (see stripDiffMetaLines). Written to be strict:
// if a line does not match, it is kept WHOLE and scanned, so a malformed or unfamiliar
// header can only ever cause MORE scanning, never less.
var reHunkHeader = regexp.MustCompile(`^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)`)

// stripDiffMetaLines drops git-GENERATED text from a unified diff, leaving every line of
// author content intact for BodyCheck. Two kinds of generated text:
//
//  1. HEADER LINES, via a two-state scanner: header-mode (entered at `^diff --git `)
//     drops every line until the first hunk header (`^@@ `); after that the scanner exits
//     header mode and keeps everything (#1052 second vector).
//  2. HUNK-HEADER FUNCNAMES, the tail of a `@@ … @@ <funcname>` line (#328).
//
// (2) is what unblocked `.assay-versions` pin bumps, and the reason is worth stating
// because it is not a content judgement. The funcname is a copy of the nearest preceding
// line that git's funcname heuristic matched, TRUNCATED to 80 bytes. `.assay-versions`
// line 16 is `statusgen statusgen/v0.6.0 <64-hex>`; it starts with a letter, so git adopts
// it as the funcname for every hunk below it and emits the sha256 cut to 53 hex. 53 is
// neither 40 nor 64, so deskkit's git-SHA exemption misses it and it scans as a secret —
// and since line 16 and the per-platform pin block are ~15 lines apart, at the default -U3
// no diff shape can merge them into one headerless hunk. Every pin bump was refused, on
// the most routine edit made to the file the pinning scheme depends on.
//
// The generic statement, which is why this is fixed here and not by special-casing a
// filename: a funcname is diff METADATA that git derives by truncating a line of the
// file. Truncation is exactly what defeats a length-anchored exemption, and it can happen
// to any file whose funcname line carries a long token — not just this one. Nothing is
// lost by not scanning it: it is an UNCHANGED line of an already-committed file (not
// something this branch introduces, which is what a pre-push scan is for), and the
// truncated copy could not be scanned reliably anyway.
//
// The RANGE (`@@ -27,5 +27,5 @@`) is kept: it is pure line arithmetic, carries no file
// content, and keeping it preserves the diff's shape for anything reading the scanned text.
//
// Taken together this ensures:
//   - All git-generated header lines are stripped (index, ---, +++, rename, similarity,
//     mode changes, binary-file markers -- the exhaustive enumeration of git's header
//     grammar would be fragile).
//   - A hunk content line `++ <secret>` (rendered as `+++ <secret>` in the unified
//     diff) is NOT mistaken for a `+++ b/...` header -- inside a hunk the scanner is
//     out of header mode, so every line is kept and scanned by BodyCheck. The same holds
//     for a content line beginning `@@`: in a real diff it carries a ` `/`+`/`-` marker,
//     so it never matches reHunkHeader's `^@@ ` anchor.
//   - A diff quoted in documentation (no `^diff --git ` lines) is never stripped -- the
//     scanner never enters header mode, and the funcname strip is gated on that same
//     signal, so a prose-quoted hunk header is scanned as the prose it is.
func stripDiffMetaLines(diff string) string {
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if isGitOutput {
			if m := reHunkHeader.FindStringSubmatch(ln); m != nil {
				ln = m[1]
			} else if len(ln) > 0 && (ln[0] == '+' || ln[0] == '-' || ln[0] == ' ') {
				// Strip the unified-diff line marker (+/-/space): it is diff SYNTAX glued to
				// the content, not part of it. Leaving it glued caused false refusals when the
				// marker joined otherwise-clean content (an added line
				// `+/tools/approvalguard/approvalguard` — the leading `+` made isPathLike's
				// "no +/=" rule reject an otherwise word-shaped 34-char path). Stripping the
				// one marker CANNOT weaken detection — the real content is what we want to
				// scan, and `+ghp_…` / `+<base64-secret>` still match once the `+` is gone —
				// while removing the whole "diff marker glued to content" false-positive class.
				ln = ln[1:]
			}
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// addedDiffLines extracts the ADDED content lines of a unified diff — the lines this
// branch INTRODUCES — with their leading `+` markers stripped, one per line. It is the
// input to the impersonated-human-ruling guard's pass over the diff surface (see
// scanWrite): only added text can claim a human's voice, so removed and context lines
// are excluded by construction rather than by asking the guard to understand diffs.
//
// It mirrors stripDiffMetaLines' two-state scanner so the two views cannot disagree
// about what is a header:
//
//   - header mode (entered at `^diff --git `, exited at the first `^@@ `) drops
//     git-generated header lines, which is what keeps a `+++ b/<path>` file header from
//     ever being read as an added content line;
//   - inside a hunk, a line is kept exactly when it begins `+` (an added line), with
//     that one marker removed — so line-start positioning and per-line citation
//     handling inside the guard see the line as it exists in the file. A content line
//     that itself begins `+` (rendered `++…` in the diff) keeps its remaining
//     characters, which can only widen what is scanned, never narrow it;
//   - hunk headers (`@@ … @@`) are pure line arithmetic and carry no author voice, so
//     they are dropped;
//   - text that never presents a `diff --git ` line is not git-diff output at all; it
//     is returned WHOLE, so a malformed or unfamiliar input can only ever cause MORE
//     scanning, never less — the same fail-open-toward-scanning posture as
//     reHunkHeader's strictness.
func addedDiffLines(diff string) string {
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if !isGitOutput {
			kept = append(kept, ln)
			continue
		}
		if strings.HasPrefix(ln, "+") {
			kept = append(kept, ln[1:])
		}
	}
	return strings.Join(kept, "\n")
}

func listOpenPRs(dir, repo, branch string) ([]ghPR, error) {
	out, err := gh(dir, "pr", "list", "-R", repo, "--head", branch, "--state", "open",
		"--json", "number,url,isDraft,headRefName")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var prs []ghPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("cannot parse gh pr list JSON: %w", err)
	}
	return prs, nil
}

func matchHead(prs []ghPR, branch string) *ghPR {
	for i := range prs {
		if prs[i].HeadRefName == branch {
			return &prs[i]
		}
	}
	return nil
}

// parseRepo extracts owner/name from an https, ssh, or scp-style git remote URL.
func parseRepo(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	u = strings.TrimSuffix(u, ".git")
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return normRepoPath(rest[slash+1:])
		}
		return "", fmt.Errorf("no path in url %q", raw)
	}
	if at := strings.Index(u, "@"); at >= 0 && strings.Contains(u, ":") {
		// scp-like: [user@]host:owner/repo
		colon := strings.Index(u, ":")
		return normRepoPath(u[colon+1:])
	}
	return normRepoPath(u)
}

func normRepoPath(p string) (string, error) {
	p = strings.Trim(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("cannot parse owner/repo from %q", p)
	}
	owner, repo := parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", fmt.Errorf("empty owner/repo in %q", p)
	}
	return owner + "/" + repo, nil
}

// requireTrailer enforces the example-stream/02 link grammar on a PR body: exactly one
// `Brief: <stream>/<NN>` that resolves to a brief file under --root, or `Issue: #<N>` for
// issue-only work. Absence, duplicates, both-kinds and non-resolving briefs are all
// constraint refusals (exit 5). There is deliberately no bypass flag — a worker-typeable
// bypass makes the edge asserted again.
//
// The one exempt body is the machine-derived issue-loop scan carrier, recognised by the
// deskkit.ScanBodyMarker that `deskscanbody emit` writes at its head. That body is
// regenerated from the branch diff on every push and reconciles a whole-scope scan
// spanning many issues, so it structurally cannot carry one per-issue trailer: no
// `Issue: #N` can be both correct and stable across a re-push. The trailer gate exists to
// force HUMAN-authored PRs to name their work item, which does not apply to this one
// machine-owned body — so it is exempt, and only it (the marker is emitter-written, not a
// worker-typeable bypass flag). Every human-authored body still faces the full gate below.
//
// On success it also returns the trailer's issue number: the parsed `#<N>` for an
// `Issue:` trailer, or 0 for a `Brief:` trailer (a brief resolves to a file, not an
// issue, so it has no reactions surface) — and 0 for the exempt scan carrier likewise.
// The create path feeds this to the public-repo gate so a non-blessed public repo gains
// the per-issue-+1 path (a +1 on the named tracking issue admits the create) instead of
// the structural no-number hard-fail (#1707).
func requireTrailer(body []byte, root, dir string) (int, error) {
	// Head-anchored, not a whole-body substring: the emitter writes ScanBodyMarker as
	// the FIRST line (deskkit.ScanPRBody), so the exemption matches it only at the body
	// head. A body that merely quotes the marker somewhere in its prose is NOT exempt —
	// this keeps the carve-out keyed to genuinely emitter-produced carrier bodies.
	if strings.HasPrefix(strings.TrimLeft(string(body), " \t\r\n"), deskkit.ScanBodyMarker) {
		return 0, nil
	}
	trs, err := deskkit.ParseTrailers(body)
	if err != nil {
		return 0, deskkit.Refused("refused: " + err.Error())
	}
	if len(trs) == 0 {
		return 0, deskkit.Refused("refused: PR body carries no trailer — add exactly one line " +
			"`Brief: <stream>/<NN>` naming the brief this PR delivers (e.g. `Brief: example-stream/02`), " +
			"or `Issue: #<N>` for issue-only work")
	}
	if trs[0].Kind == deskkit.TrailerIssue {
		// Value is the bare digits (trailer.go guarantees `[0-9]+`); Atoi cannot fail,
		// but treat any parse anomaly as "no number" (0) rather than admitting a
		// negative that would read as a sentinel to the gate.
		n, perr := strconv.Atoi(trs[0].Value)
		if perr != nil || n <= 0 {
			return 0, nil
		}
		return n, nil
	}
	stream, nn, ok := splitBriefTrailer(trs[0].Value)
	if !ok {
		return 0, deskkit.Refused(fmt.Sprintf("refused: trailer %q does not name a brief as <stream>/<NN> or <stream>:<NN>", trs[0].Value))
	}
	// A relative --root resolves against the WORK DIR (the getwd seam), never the
	// process cwd — tests call cmdCreate directly with a bound getwd, and a glob
	// against the real process cwd would silently miss the fixture.
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md"))
	if len(matches) == 0 {
		return 0, deskkit.Refused(fmt.Sprintf("refused: `Brief: %s` does not resolve to a brief under --root: no %s found",
			trs[0].Value, filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md")))
	}
	return 0, nil
}

// splitBriefTrailer reduces the accepted trailer value forms to (stream, NN):
// <stream>/<NN> (brief-v1), <stream>:<NN>, <repo>:<stream>:<NN>, and the full
// <cell>:<repo>:<stream>:<NN>. For the colon forms the LAST two parts are stream and NN;
// the repo/cell prefixes resolve against graph-repos.yaml elsewhere (example-stream/01)
// and are not needed for the file resolution here. NN must be numeric.
func splitBriefTrailer(v string) (stream, nn string, ok bool) {
	var parts []string
	if strings.Contains(v, ":") {
		parts = strings.Split(v, ":")
		if len(parts) < 2 {
			return "", "", false
		}
		stream, nn = parts[len(parts)-2], parts[len(parts)-1]
	} else {
		parts = strings.Split(v, "/")
		if len(parts) != 2 {
			return "", "", false
		}
		stream, nn = parts[0], parts[1]
	}
	if stream == "" || nn == "" {
		return "", "", false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return "", "", false
		}
	}
	return stream, nn, true
}

func readBody(bodyFile, bodyMin string) ([]byte, error) {
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot read --body-file", err)
		}
		if len(b) > maxBodyBytes {
			return nil, deskkit.Refused(fmt.Sprintf("refused: body exceeds %d bytes (%d)", maxBodyBytes, len(b)))
		}
		return b, nil
	}
	b := []byte(bodyMin)
	if len(b) > maxBodyBytes {
		return nil, deskkit.Refused(fmt.Sprintf("refused: --body-min exceeds %d bytes", maxBodyBytes))
	}
	return b, nil
}

func writeTempBody(body []byte) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "deskpr-body-*.md")
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	if _, err := f.Write(body); err != nil {
		f.Close()
		os.Remove(name)
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", func() {}, err
	}
	return name, func() { os.Remove(name) }, nil
}

func lastURL(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
			return l
		}
	}
	return strings.TrimSpace(out)
}

func prNumberFromURL(url string) int {
	m := pullRe.FindStringSubmatch(url)
	if len(m) != 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

func isDefaultName(branch string) bool { return branch == "main" || branch == "master" }

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func exitCode(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), true
	}
	return 0, false
}

// splitOwnerRepo splits "owner/name" into its components.
func splitOwnerRepo(repo string) (owner, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
