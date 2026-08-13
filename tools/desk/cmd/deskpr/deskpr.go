package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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
	defaultRef    string // remote-tracking ref, e.g. "origin/main"
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
	asApp := fs.Bool("as-app", true, "authenticate as the worker App via desktoken worker (default on); --as-app=false for example-org fallback")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: create takes no positional arguments")
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
	if serr := deskkit.ScanSurface("PR body", body); serr != nil {
		return serr
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	facts, perr := preflight(dir)
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	// seatbelt: scan title, branch, and the diff-vs-default before any push.
	if scanErr := scanWrite(facts, *title); scanErr != nil {
		return scanErr
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
	// Deskpr creates a PR, which has no issue number yet — the gate call uses 0
	// as a sentinel for "no associated issue/PR" and must refuse with exit 6.
	// The rule: when there is no associated issue/PR (a repo-level action
	// with no reactions surface), PublicRepoGate MUST REFUSE with exit 6.
	// This means deskpr cannot operate on a public repo until a human creates
	// the issue and adds the +1 first.
	owner, name := splitOwnerRepo(facts.repo)
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name, 0); gerr != nil {
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
	if n := prNumberFromURL(url); n > 0 {
		ac.pr = &n
	}
	ac.detail = "created " + url
	fmt.Println(url)
	return nil
}

// cmdUpdate implements `deskpr update`: a follow-up push of the current branch to its
// EXISTING open draft PR (the fix→re-review hot path). It refuses if no open PR exists
// for the branch, or if that PR is not a draft. No PR is ever created here.
func cmdUpdate(args []string) (err error) {
	ac := &auditCtx{verb: "update"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	asApp := fs.Bool("as-app", true, "authenticate as the worker App via desktoken worker (default on); --as-app=false for example-org fallback")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: update takes no arguments")
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	facts, perr := preflight(dir)
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	if scanErr := scanWrite(facts, ""); scanErr != nil {
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
	if !pr.IsDraft {
		return deskkit.Refused(fmt.Sprintf(
			"refused: PR #%d is not a draft — deskpr update only pushes to draft PRs", pr.Number))
	}
	ac.pr = &pr.Number

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
	ac.detail = "pushed to " + pr.URL
	fmt.Println(pr.URL)
	return nil
}

// preflight verifies the preconditions IN-TOOL and returns the positively-verified
// facts. Ordering matters: the default-branch refusal fires on branch name alone
// (exit 5) BEFORE origin/HEAD is consulted, so a missing origin/HEAD can never mask a
// push to main; a detached HEAD or unreadable origin/HEAD is unverifiable (exit 6).
func preflight(dir string) (*gitFacts, error) {
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
	defOut, derr := git(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if derr != nil {
		return nil, deskkit.Unverifiable(
			"cannot read origin/HEAD (default branch unverifiable) — run `git remote set-head origin --auto`", derr)
	}
	defaultBranch := strings.TrimPrefix(strings.TrimSpace(defOut), "origin/")
	if defaultBranch == "" {
		return nil, deskkit.Unverifiable("origin/HEAD resolved empty", nil)
	}
	if branch == defaultBranch {
		return nil, deskkit.Refused("refused: on the default branch (" + branch + ")")
	}
	defaultRef := "origin/" + defaultBranch

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

	cnt, cerr := git(dir, "rev-list", "--count", defaultRef+"..HEAD")
	if cerr != nil {
		return nil, deskkit.Unverifiable("cannot count commits ahead of "+defaultRef, cerr)
	}
	if cnt == "0" {
		return nil, deskkit.Refused("refused: branch has no commits ahead of " + defaultRef)
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
func scanWrite(f *gitFacts, title string) error {
	if title != "" {
		if err := deskkit.ScanSurface("PR title", []byte(title)); err != nil {
			return err
		}
	}
	if err := deskkit.ScanSurface("branch name", []byte(f.branch)); err != nil {
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
	// line (context, `+` added, `-` removed) still goes through BodyCheck in full, so
	// detection strength on author-written content is unchanged. Gated here at the
	// diff-scanning callsite, not inside deskkit.BodyCheck — BodyCheck is generic (also
	// used verbatim on PR bodies/comments/reviews) and must not grow diff-format
	// awareness.
	if err := deskkit.ScanSurface("branch diff vs "+f.defaultRef,
		[]byte(stripDiffMetaLines(diff))); err != nil {
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
