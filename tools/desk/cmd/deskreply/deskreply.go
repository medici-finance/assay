package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// getwd is the seam for the tool's working directory (the worktree it runs in).
// Production uses os.Getwd; tests point it at a scratch worktree fixture without
// os.Chdir (which would race parallel processes).
var getwd = os.Getwd

const maxBodyBytes = 16 * 1024 // body cap (16 KiB)

// gitFacts is the positively-verified worktree state preflight establishes before any
// write. deskreply reads it with READ-ONLY git; it never pushes.
type gitFacts struct {
	dir    string
	branch string // current branch (never "HEAD"/detached)
	repo   string // owner/name parsed from origin
}

// ghPRView is the slice of `gh pr view --json` deskreply needs to verify the PR is OPEN
// and that the worktree's branch is the PR's head branch.
type ghPRView struct {
	State       string `json:"state"`
	URL         string `json:"url"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
}

// auditCtx accumulates the fields for the ONE audit line every invocation emits
// ("audit every path"). finalize is deferred so exactly one line is written no matter
// which branch returns.
type auditCtx struct {
	repo          string
	pr            *int
	head          string // PR head oid (the idempotency "headSHA")
	bodyDigest    string
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
		Tool:       "deskreply",
		Verb:       "reply",
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		PR:         a.pr,
		HeadSHA:    headp,
		BodyDigest: a.bodyDigest,
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

// cmdReply implements the single verb `deskreply <owner/repo> <pr> --body-file F`.
// Flow: parse args → read+scan body → verify worktree is my own PR's checkout
// → verify the PR is OPEN and its head branch is my branch → idempotency
// → rate limit → `gh pr comment` under ambient identity → audit.
func cmdReply(args []string) (err error) {
	ac := &auditCtx{}
	defer func() { ac.finalize(err) }()

	// The documented signature places the two positionals FIRST, then flags:
	//   deskreply <owner/repo> <pr> --body-file F
	// Reject anything that does not start with the two positionals (a leading "-" is a
	// misplaced flag or a typo) so we never mis-read a flag as a repo.
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return deskkit.Refused("refused: usage: deskreply <owner/repo> <pr> --body-file F")
	}
	repo := args[0]
	prArg := args[1]

	fs := flag.NewFlagSet("deskreply", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress flag's own output; we craft messages
	bodyFile := fs.String("body-file", "", "path to a file containing the reply body (required)")
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, body digest, reason, identity)")
	if perr := fs.Parse(args[2:]); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: unexpected extra arguments after <owner/repo> <pr>")
	}
	if strings.TrimSpace(*bodyFile) == "" {
		return deskkit.Refused("refused: --body-file is required (no stdin/inline body)")
	}
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}

	pr, prErr := strconv.Atoi(prArg)
	if prErr != nil || pr <= 0 {
		return deskkit.Refused("refused: PR must be a positive integer, got " + prArg)
	}
	ac.pr = &pr

	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused("refused: " + repo + " is not in the desk-tools repo set")
	}
	ac.repo = repo

	// Body: file only, 16 KiB cap, secret scan. The scan's verdict routes through
	// deskkit.HandleScanRefusal, so a refusal advertises the audited override
	// (#585) and an override taken here writes its row BEFORE the post.
	body, berr := readBody(*bodyFile)
	if berr != nil {
		return berr
	}
	if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
		Tool: "deskreply", Verb: "reply", Repo: repo, PR: &pr, Reason: *scanOverride,
		Surface: deskkit.SurfaceBody, Content: body,
	}, deskkit.BodyCheck(body)); serr != nil {
		return serr
	}
	ac.bodyDigest = deskkit.Sha256Hex(body)

	// This must be the worker's OWN PR. Establish the worktree facts, then
	// confirm the positional repo is this worktree's origin — a worker replies on the PR
	// that lives in its own origin; replying elsewhere is the desk's job via deskpost.
	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	facts, perr := preflight(dir)
	if perr != nil {
		return perr
	}
	if facts.repo != repo {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is not this worktree's origin (%s) — deskreply only replies on your own PR",
			repo, facts.repo))
	}

	// Mint/rewrite the worker App installation token, scoped to THIS repo (facts.repo,
	// already verified above to equal the caller-given repo), so every subsequent gh
	// invocation (view PR state and post comment) authenticates as the worker App
	// installation that actually has access to it (#562: omitting --repo let desktoken
	// fall back to its example-org default, minting a token for the wrong installation
	// whenever deskreply ran against a medici-finance/* repo).
	if merr := mintWorkerToken(facts.repo); merr != nil {
		return deskkit.Unverifiable("cannot mint worker token for deskreply", merr)
	}

	// Public-repo gate: refuse to write to a public repo
	// without a qualifying +1 from an authorized human.
	owner, name := splitOwnerRepo(repo)
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name, pr); gerr != nil {
		return gerr
	}

	// Verify the PR is OPEN and that its head branch is the branch checked out here. Any
	// API/parse failure is unverifiable (exit 6), never a silent assume-open.
	view, verr := viewPR(dir, repo, pr)
	if verr != nil {
		return deskkit.Unverifiable("cannot read PR state — refuse rather than guess", verr)
	}
	if view.URL != "" {
		ac.detail = view.URL
	}
	if !strings.EqualFold(view.State, "OPEN") {
		return deskkit.Refused(fmt.Sprintf("refused: PR #%d is %s, not OPEN — deskreply only replies on open PRs", pr, view.State))
	}
	if view.HeadRefName != facts.branch {
		return deskkit.Refused(fmt.Sprintf(
			"refused: this worktree is on %q but PR #%d's head branch is %q — deskreply only replies on YOUR OWN PR",
			facts.branch, pr, view.HeadRefName))
	}
	ac.head = view.HeadRefOid

	// idempotency, key (repo, pr, headSHA, bodyDigest): this exact reply body already
	// posted at this PR head → noop, no `gh pr comment`. LoadEntries fails CLOSED on a
	// corrupt audit file (exit 6) BEFORE the dedup decision, so corruption can never
	// masquerade as "already done" and skip a needed post.
	entries, lerr := deskkit.LoadEntries()
	if lerr != nil {
		return lerr // already an Unverifiable *DeskError (exit 6)
	}
	if alreadyReplied(entries, repo, pr, ac.head, ac.bodyDigest) {
		ac.successResult = deskkit.ResultNoop
		ac.detail = fmt.Sprintf("identical reply already posted at head %s%s", shortSHA(ac.head), urlSuffix(view.URL))
		fmt.Printf("noop: identical reply already posted on PR #%d at %s\n", pr, shortSHA(ac.head))
		return nil
	}

	// Outward-write rate limit — counts ALL deskreply attempts in the last
	// rolling hour, checked immediately before the post.
	if werr := deskkit.AllowWrite("deskreply", repo, pr); werr != nil {
		return werr
	}

	// The post. argv is constructed literally: `gh pr comment <pr> -R <repo> --body-file
	// <path>` — the ONLY mutating gh verb deskreply can ever emit. No review/ready/create.
	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return deskkit.Unverifiable("cannot stage reply body", terr)
	}
	defer cleanup()

	out, cErr := gh(dir, "pr", "comment", strconv.Itoa(pr), "-R", repo, "--body-file", bodyPath)
	if cErr != nil {
		return deskkit.Unverifiable("gh pr comment failed", cErr)
	}
	if url := lastURL(out); url != "" {
		ac.detail = "commented " + url
		fmt.Println(url)
	} else {
		ac.detail = "commented on PR #" + strconv.Itoa(pr)
		fmt.Printf("commented on PR #%d\n", pr)
	}
	return nil
}

// alreadyReplied is the pure predicate over entries already loaded (and verified) by
// deskkit.LoadEntries. It reports whether an identical reply body was already posted on
// this PR at this head — matching the full (repo, pr, headSHA, bodyDigest) key with a
// terminal result of ok/noop. A prior REFUSAL never counts (a refusal followed by a state
// change is a legitimate retry).
func alreadyReplied(entries []deskkit.Entry, repo string, pr int, head, digest string) bool {
	for i := range entries {
		e := &entries[i]
		if e.Repo == repo &&
			e.Verb == "reply" &&
			e.PR != nil && *e.PR == pr &&
			e.HeadSHA != nil && *e.HeadSHA == head &&
			e.BodyDigest == digest &&
			(e.Result == deskkit.ResultOK || e.Result == deskkit.ResultNoop) {
			return true
		}
	}
	return false
}

// preflight verifies the worktree is a git checkout on a real branch and resolves its
// origin repo (owner/name). A detached HEAD or an unreadable origin is unverifiable
// (exit 6); an origin outside the deskkit set is refused (exit 5).
func preflight(dir string) (*gitFacts, error) {
	if out, err := git(dir, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
		return nil, deskkit.Unverifiable("not inside a git worktree", err)
	}
	branch, err := git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve current branch", err)
	}
	if branch == "HEAD" || branch == "" {
		return nil, deskkit.Unverifiable("detached HEAD — check out your PR's feature branch first", nil)
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
	return &gitFacts{dir: dir, branch: branch, repo: repo}, nil
}

// viewPR reads the PR's state / head branch / head oid / url via the ambient gh identity.
func viewPR(dir, repo string, pr int) (*ghPRView, error) {
	out, err := gh(dir, "pr", "view", strconv.Itoa(pr), "-R", repo,
		"--json", "state,url,headRefName,headRefOid")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, fmt.Errorf("gh pr view returned no data")
	}
	var v ghPRView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, fmt.Errorf("cannot parse gh pr view JSON: %w", err)
	}
	if v.State == "" || v.HeadRefName == "" {
		return nil, fmt.Errorf("gh pr view JSON missing state/headRefName")
	}
	return &v, nil
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

func readBody(bodyFile string) ([]byte, error) {
	b, err := os.ReadFile(bodyFile)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read --body-file", err)
	}
	if len(b) > maxBodyBytes {
		return nil, deskkit.Refused(fmt.Sprintf("refused: body exceeds %d bytes (%d)", maxBodyBytes, len(b)))
	}
	return b, nil
}

func writeTempBody(body []byte) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "deskreply-body-*.md")
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
	return ""
}

func urlSuffix(url string) string {
	if url == "" {
		return ""
	}
	return " (" + url + ")"
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// splitOwnerRepo splits "owner/name" into its components.
func splitOwnerRepo(repo string) (owner, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
