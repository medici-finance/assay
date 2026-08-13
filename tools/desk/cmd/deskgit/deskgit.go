package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// getwd is the seam for the tool's working directory (the repo it runs in). Production
// uses os.Getwd; tests point it at a scratch checkout without os.Chdir (which would race
// parallel processes).
var getwd = os.Getwd

// prNumRe / branchRe bound the values that flow into a constructed refspec. Digits-only
// for --pr; a single ref-ish token for --branch with no leading dash (arg-injection),
// no `+` (force), no `:` (second refspec), no `..`/`~`/`^`/`?`/`*`/whitespace.
var (
	prNumRe  = regexp.MustCompile(`^[0-9]+$`)
	branchRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
)

// The fetch invocation always carries these hardening flags (issue #1555 security review):
//   - --upload-pack=git-upload-pack pins the remote-helper program on the CLI, which
//     overrides remote.origin.uploadpack from .git/config OR from injected env config —
//     the code-execution vector (proven: CLI flag beats both);
//   - --refmap= clears the configured refmap so ONLY the explicit positional refspec
//     below applies, so a malicious remote.origin.fetch cannot redirect writes to local
//     branches (proven: config refspec is otherwise applied);
//   - --no-recurse-submodules (ADDED IN THE assay PORT, not in #1556) pins
//     submodule recursion OFF. `fetch.recurseSubmodules`/`submodule.<n>.fetchRecurseSubmodules`
//     in .git/config can turn one gated fetch into fetches of arbitrary .gitmodules URLs,
//     each carrying its own transport and its own config — i.e. it escapes the effective-URL
//     repo gate that the other pins exist to enforce. Same class as the refmap pin (config
//     redirecting a fetch), so it is pinned on the CLI the same way.
var fetchHardening = []string{"--refmap=", "--upload-pack=git-upload-pack", "--no-recurse-submodules"}

// trackingRefspec confines writes to remote-tracking refs only.
const trackingRefspec = "+refs/heads/*:refs/remotes/origin/*"

// auditCtx accumulates the fields for the ONE audit line every invocation emits.
type auditCtx struct {
	verb          string
	repo          string
	detail        string
	successResult string
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "deskgit",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}

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
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// cmdFetch refreshes refs from origin. Three mutually exclusive modes, each building a
// FIXED git argv (literal verbs + hardening flags + a refspec derived from a validated
// value) so no caller flag, no `--upload-pack`/`--exec`, and no arbitrary refspec can
// reach git:
//
//	(bare)        git fetch --refmap= --upload-pack=git-upload-pack [--prune] origin +refs/heads/*:refs/remotes/origin/*
//	--pr <N>      … origin refs/pull/<N>/head:refs/heads/pr<N>      (N digits-only)
//	--branch <B>  … origin refs/heads/<B>:refs/heads/<B>           (B ref-ish; not main/master
//	                                                                in ANY case)
//
// BOTH ref-writing modes additionally refuse a destination that differs only by CASE from
// an existing local branch — see localRefTarget/branchCollision.
//
// The tool does NOT sandbox a fully attacker-controlled repo `.git/config` (git has
// other exec knobs such as core.sshCommand/core.fsmonitor); it closes the proven
// upload-pack, env, and fetch-refspec vectors and honestly claims no more.
func cmdFetch(args []string) (err error) {
	ac := &auditCtx{verb: "fetch"}
	defer func() { ac.finalize(err) }()

	// Named transport-exec refusal FIRST — before the FlagSet, so the specific reason
	// wins over the generic "flag provided but not defined" / "extra operand", and so
	// the guard is independent of the flag table. See transportexec.go for why the
	// source PR's incidental refusal was not enough. (Port strengthening.)
	if terr := checkTransportExec(args); terr != nil {
		return terr
	}

	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	prune := fs.Bool("prune", false, "also drop remote-tracking refs deleted on origin")
	prNum := fs.String("pr", "", "fetch pull/<N>/head into local branch pr<N> (digits only)")
	branch := fs.String("branch", "", "fetch origin's <B> into local branch <B> (not main/master in any case)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags — " + perr.Error())
	}
	if fs.NArg() != 0 {
		// Refusing operands is what stops a raw refspec (`+refs/heads/x:refs/heads/main`,
		// the local-ref-move vector) from being passed through.
		return deskkit.Refused(fmt.Sprintf(
			"refused: fetch takes no positional operands (got %q); use --pr/--branch for a specific ref",
			strings.Join(fs.Args(), " ")))
	}

	// Build the mode-specific tail (git verbs/refspec) from validated values only.
	var tail []string
	var detail string
	modes := 0
	if *prNum != "" {
		modes++
		if !prNumRe.MatchString(*prNum) {
			return deskkit.Refused("refused: --pr must be digits only, got " + *prNum)
		}
		tail = []string{"origin", "refs/pull/" + *prNum + "/head:refs/heads/pr" + *prNum}
		detail = "fetched pull/" + *prNum + "/head -> pr" + *prNum
	}
	if *branch != "" {
		modes++
		b := *branch
		if !branchRe.MatchString(b) || isProtectedBranch(b) {
			return deskkit.Refused("refused: --branch must be a ref-ish name and not main/master, got " + b)
		}
		if why := branchRejectReason(b); why != "" {
			return deskkit.Refused("refused: --branch " + why + ", got " + b)
		}
		tail = []string{"origin", "refs/heads/" + b + ":refs/heads/" + b}
		detail = "fetched origin " + b + " -> " + b
	}
	if *prune {
		modes++
	}
	if modes > 1 {
		return deskkit.Refused("refused: --prune, --pr, and --branch are mutually exclusive")
	}
	if *prNum == "" && *branch == "" {
		// bare (optionally --prune): confine writes to remote-tracking refs.
		tail = []string{"origin", trackingRefspec}
		detail = "fetched origin"
		if *prune {
			tail = append([]string{"--prune", "origin"}, trackingRefspec)
			detail = "fetched origin --prune"
		}
	}

	dir, werr := getwd()
	if werr != nil {
		return deskkit.Unverifiable("cannot determine working directory", werr)
	}
	if _, terr := runGit(dir, "rev-parse", "--is-inside-work-tree"); terr != nil {
		return deskkit.Unverifiable("not inside a git worktree", terr)
	}

	// Case-collision check needs the worktree, so it runs here rather than with the
	// per-mode validation above. It is a REFUSAL (exit 5), not a fetch failure. It is keyed
	// on the LOCAL REF each mode writes, not on the flag value, so it covers every
	// ref-writing mode rather than just --branch (correctness review).
	if flagName, target := localRefTarget(*branch, *prNum); target != "" {
		if why := branchCollision(dir, target); why != "" {
			// #226(2): this refusal is deliberately ordered BEFORE the origin parse, so its
			// audit line had no slug and emitted an EMPTY `repo` field — a ledger consumer
			// filtering or grouping by repo could not see these refusals under the
			// repository they were aimed at. The refusal is ALREADY decided at this point;
			// the lookup below only LABELS the audit line and can neither admit nor block
			// anything. It is best-effort: on any error `repo` stays empty, exactly as
			// before. The ordering the refusal depends on is untouched — no `git fetch`
			// runs on a colliding destination either way.
			ac.repo = bestEffortOriginRepo(dir)
			return deskkit.Refused("refused: " + flagName + " " + why + ", got " + target)
		}
	}

	// Gate on the EFFECTIVE origin URL — `git ls-remote --get-url` expands
	// url.<base>.insteadOf and exits WITHOUT contacting the remote, so an insteadOf
	// rewrite cannot present an allowed identity while fetching elsewhere.
	originURL, oerr := runGit(dir, "ls-remote", "--get-url", "origin")
	if oerr != nil {
		return deskkit.Unverifiable("cannot resolve effective origin URL", oerr)
	}
	repo, rerr := parseRepo(originURL)
	if rerr != nil {
		// REFUSED (exit 5), not unverifiable (exit 6): the tool positively determined the
		// URL is unacceptable — this is the same class of decision as the IsAllowedRepo
		// miss just below, which already exits 5. Exit 6 is for "could not run ls-remote".
		// Filing smuggling attempts (padded paths, path-borne '@', remote-helper forms) in
		// the same audit bucket as network faults is the very confusion transportexec.go
		// argues against for the named guard (correctness review).
		return deskkit.Refused("refused: cannot parse origin repo from " +
			redactURL(originURL) + ": " + rerr.Error())
	}
	ac.repo = repo
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused("refused: origin " + repo + " is not in the desk-tools repo set")
	}

	fetchArgs := append([]string{"fetch"}, fetchHardening...)
	fetchArgs = append(fetchArgs, tail...)

	out, ferr := runGit(dir, fetchArgs...)
	if ferr != nil {
		return deskkit.Unverifiable("git fetch failed", ferr)
	}
	// Record the EFFECTIVE origin URL on the success path (security review). Without it
	// the audit line for a legitimate refresh, a foreign-host fetch and the residual-2
	// insteadOf-to-local-path smuggle are byte-identical in every security-relevant field
	// (repo/result/detail all equal; only argsDigest varies, and it digests the TOOL's
	// args, which are the same in all three). While residual 2 is open this is the one
	// compensating control available: it does not gate anything, but it makes the smuggle
	// DETECTABLE after the fact instead of invisible.
	ac.detail = detail + " [origin " + redactURL(originURL) + "]"
	if out != "" {
		fmt.Fprintln(os.Stderr, out)
	}
	return nil
}

// bestEffortOriginRepo resolves the effective origin slug for AUDIT LABELLING ONLY
// (#226 item 2). It is never a gate: every caller has already decided its
// outcome, and an error here yields "" — the same empty `repo` field the line carried
// before. `ls-remote --get-url` expands url.<base>.insteadOf locally and contacts no
// remote, so calling it on a refusal path adds no network side effect.
func bestEffortOriginRepo(dir string) string {
	originURL, err := runGit(dir, "ls-remote", "--get-url", "origin")
	if err != nil {
		return ""
	}
	repo, err := parseRepo(originURL)
	if err != nil {
		return ""
	}
	return repo
}

// redactURL strips any userinfo from a remote URL before it reaches an audit line or a
// refusal message. An https remote can legitimately carry `user:token@`, and the audit
// ledger is a durable artifact — recording the effective URL (security review) must not
// turn the audit ledger into a credential store. Only the userinfo is removed; host and path, the parts
// that make the line useful for detecting a smuggled destination, are preserved verbatim.
//
// Deliberately string-level and total: it must never fail or panic on the hostile URLs it
// exists to log, so it does not go through net/url (which rejects many shapes git accepts).
func redactURL(raw string) string {
	u := strings.TrimSpace(raw)
	if i := strings.Index(u, "://"); i >= 0 {
		scheme, rest := u[:i+3], u[i+3:]
		if at := strings.LastIndexByte(rest[:authorityEnd(rest)], '@'); at >= 0 {
			return scheme + "<redacted>@" + rest[at+1:]
		}
		return u
	}
	// scp-like [user@]host:path — a user component here is a username, not a secret, but
	// redact it for consistency so no shape leaks a token through a different branch.
	if colon := strings.IndexByte(u, ':'); colon >= 0 {
		if slash := strings.IndexByte(u, '/'); slash < 0 || colon < slash {
			if at := strings.LastIndexByte(u[:colon], '@'); at >= 0 {
				return "<redacted>@" + u[at+1:]
			}
		}
	}
	return u
}

// remoteHelperRe matches a git remote-helper transport form `<helper>::<address>`
// (e.g. `ext::sh -c …`). The helper program is executed by git for that transport, so
// this form is refused outright (issue #1555 re-review) — the pinned --upload-pack
// does not help when the execution comes from the transport itself. `scheme://…` (a
// single `:` then `//`) and scp-like `user@host:path` (a single `:`) do not match.
var remoteHelperRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+.-]*::`)

// schemeRe anchors `scheme://` to the START of the URL, per RFC 3986 scheme syntax (a
// letter followed by letters/digits/`+`/`-`/`.`). Anchoring is what keeps a `://` buried in
// an scp-like path from routing that URL into the host-bearing branch — see parseRepo.
var schemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// parseRepo extracts owner/name from an https, ssh, or scp-style git remote URL and, for
// a URL that names a host (scheme:// or scp-like), REQUIRES the path to be exactly
// `owner/repo` — so a padded path such as `github.com/<attacker>/<pad>/example-org/…`
// cannot present an allowed slug in its trailing components (issue #1555 re-review).
//
// Getting the HOST-BEARING test right is what the security review was about: the
// exact-path rule only bites on URLs this function actually routes to exactStructRepoPath,
// and two shapes used to escape that routing — a `scheme://` URL whose PATH contains '@',
// and a scp-like URL with no `user@` component. Both are closed above; both are covered by
// TestParseRepo_AuthorityParsing. The lesson is that the bypass was in the ROUTING, not in
// the exact-path check itself.
//
// Two residuals, documented and NOT fixed — read these before assuming the gate binds
// identity:
//
//  1. The host is NOT bound to `github.com`, because the desk machine's origins use ssh
//     host ALIASES (`git@github-example:owner/repo`), so a hard host requirement would refuse
//     the real remote. A URL with an exact allowed `owner/repo` on an UNEXPECTED host still
//     passes the gate and performs a READ-ONLY fetch (the upload-pack pin + env scrub
//     ensure no code executes). Binding host via an explicit allowlist is a follow-up.
//  2. BARE LOCAL PATHS are not subject to the exact-path rule at all — a real local repo
//     lives at an arbitrary absolute path, so lenientRepoPath takes the last two
//     components. This branch IS reachable in production: an `insteadOf` rewrite pointing
//     at any local directory whose last two components spell an allowed slug passes the
//     gate, and its content then lands on the tracking refs the desk loops consume. Closing
//     it needs a different identity model for local paths (an allowlist of local roots),
//     not a parser tweak — hence a residual rather than a fix here.
func parseRepo(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if remoteHelperRe.MatchString(u) {
		return "", fmt.Errorf("remote-helper transport form is not allowed: %q", raw)
	}
	u = strings.TrimSuffix(u, ".git")
	// Scheme detection is ANCHORED (security review hygiene). `strings.Index(u, "://")`
	// matched `://` ANYWHERE, so an scp-like URL carrying `://` inside its path was routed
	// into this host-bearing branch and its trailing components accepted as an exact
	// `owner/repo` — skipping the exact-path rule for exactly the shape it was written for.
	// It was fail-closed only by luck (real git splits on the first `://` too and then
	// rejects the protocol name), and it becomes load-bearing the moment a host allowlist
	// lands. Anchoring routes those URLs to the scp-like branch below, which is STRICTER.
	if loc := schemeRe.FindStringIndex(u); loc != nil {
		rest := u[loc[1]:]
		// The authority ends at the first '/', '?' or '#', so userinfo can only occur
		// BEFORE it. Scanning the whole remainder for '@' (the pre-fix behaviour) let a
		// PATH containing '@' swallow the real host as userinfo: in
		// `https://evil.example.com/pad@github.com/owner/repo` the '@' is in the path,
		// yet the scan consumed `evil.example.com/pad@` and then applied the exact-path
		// check to a mere SUFFIX of the path — so a foreign host presented an allowed
		// slug. Bounding the scan to the authority closes it (security review).
		ae := authorityEnd(rest)
		if at := strings.LastIndexByte(rest[:ae], '@'); at >= 0 {
			rest = rest[at+1:]
			ae = authorityEnd(rest)
		}
		// The PATH begins only where the authority ended AT A '/'. If the authority ended
		// at '?' or '#' there is no path at all — the pre-fix code searched the whole
		// remainder for '/', so a query or fragment that happened to spell `/owner/repo`
		// was reported as the path and the gate passed it. authorityEnd already knew where
		// the authority stopped; path extraction now agrees with it (security review).
		if ae >= len(rest) || rest[ae] != '/' {
			return "", fmt.Errorf("no path in url %q", raw)
		}
		path := rest[ae+1:]
		if j := strings.IndexAny(path, "?#"); j >= 0 {
			path = path[:j]
		}
		return exactRepoPath(path)
	}
	// scp-like: [user@]host:owner/repo — git recognises this form whenever a ':' appears
	// before the first '/', and the `user@` part is OPTIONAL. Gating on the presence of
	// '@' (the pre-fix behaviour) meant a userless `host:owner/repo` fell through to the
	// lenient bare-path branch below and got last-two-components treatment, so a padded
	// `evil.example.com:pad/owner/repo` presented an allowed slug — while the very same
	// URL WITH a user component was correctly refused (security review).
	if colon := strings.IndexByte(u, ':'); colon >= 0 {
		if slash := strings.IndexByte(u, '/'); slash < 0 || colon < slash {
			return exactRepoPath(u[colon+1:])
		}
	}
	// Bare local path (no host). Lenient last-two by necessity: a real local repo lives at
	// an arbitrary absolute path, so no exact-path rule can apply. See the Residual note in
	// parseRepo's doc comment — this branch is REACHABLE IN PRODUCTION via an insteadOf
	// rewrite, and its identity check is weak by nature.
	return lenientRepoPath(u)
}

// authorityEnd returns the index at which the authority component of `host/path?q#f`
// ends — the first '/', '?' or '#', or len(s) when none is present.
func authorityEnd(s string) int {
	end := len(s)
	for _, c := range []byte{'/', '?', '#'} {
		if i := strings.IndexByte(s, c); i >= 0 && i < end {
			end = i
		}
	}
	return end
}

// isProtectedBranch reports whether b names the desk's integration branches, compared
// CASE-INSENSITIVELY (security review).
//
// The comparison used to be case-sensitive (`b == "main"`), which was a false guarantee on
// the desk machine: `--branch` writes into `.git/refs/heads/`, and darwin/APFS is
// case-INSENSITIVE by default, so `refs/heads/Main` and `refs/heads/main` are the SAME ref.
// `--branch Main` therefore passed the guard and replaced local `main`. The no-force
// property did not save it — the constructed refspec carries no leading `+`, but git
// believed it was CREATING a ref that did not exist, so no fast-forward check ran at all
// and an unrelated foreign commit landed on the branch the desk loops merge from, at
// exit 0 with an audit line reading `ok`. Verified on darwin with git 2.55.0: `Main`,
// `mAiN` and `MASTER` each rewrote the canonical ref; `main` was correctly refused.
//
// EqualFold is ASCII-adequate here because branchRe already confines b to ASCII.
func isProtectedBranch(b string) bool {
	return strings.EqualFold(b, "main") || strings.EqualFold(b, "master")
}

// localRefTarget returns the flag name and the LOCAL branch each ref-writing mode will
// write, or ("", "") for modes that touch only remote-tracking refs (bare / --prune).
//
// The collision check keys on this rather than on the raw flag value, because the defect it
// guards is a property of the DESTINATION REF, not of the caller's spelling: --pr N writes
// `pr<N>`, which is not the string the caller typed. Gating on `*branch != ""` covered only
// one of the two ref-writing modes while branchCollision's doc claimed the general
// principle — an overstatement of exactly the kind this PR was twice marked down for
// (correctness review, reproduced: a differently-cased local `pr<N>` was replaced by an
// unrelated commit at exit 0, non-fast-forward, same shape as the --branch blocker).
func localRefTarget(branch, prNum string) (flag, target string) {
	switch {
	case branch != "":
		return "--branch", branch
	case prNum != "":
		return "--pr", "pr" + prNum
	default:
		return "", ""
	}
}

// branchCollision reports why target must be refused because it differs only by CASE from a
// ref that already exists locally — i.e. the fetch would silently rewrite a ref the caller
// did not name. Returns "" when there is no collision.
//
// isProtectedBranch alone fixes the two magic names, but the underlying defect is a
// filesystem ref-NAMESPACE collision, not those names: on a case-insensitive filesystem
// ANY existing local branch can be rewritten by naming a different spelling of it. This is
// therefore applied to EVERY mode that writes a local ref (see localRefTarget), so the
// guarantee stated here is the guarantee the code delivers. An EXACT-spelling match is not
// a collision — that is the ordinary update of a branch the caller did in fact name, and it
// stays subject to git's normal fast-forward rules.
func branchCollision(dir, target string) string {
	out, err := runGit(dir, "for-each-ref", "--format=%(refname:strip=2)", "refs/heads/")
	if err != nil {
		// Enumeration failed: fail CLOSED for this extra check rather than assume no
		// collision — the caller still gets a distinct, actionable reason.
		return "cannot enumerate local branches to check for a case-collision"
	}
	for _, n := range strings.Fields(out) {
		if n != target && strings.EqualFold(n, target) {
			return "differs only by case from existing local branch " + n +
				" (a case-insensitive filesystem would silently rewrite it)"
		}
	}
	return ""
}

// branchRejectReason enforces the git check-ref-format rules that branchRe's character
// class cannot express — it constrains CHARACTERS, not SEQUENCES. Returns "" when ok.
//
// Without these, a value such as `a..b` passed branchRe, reached git inside a constructed
// refspec, and was refused only by git's own validation — surfacing as exit 6
// (unverifiable, "git fetch failed") rather than the tool's own exit 5 (refused). deskgit
// validates before it delegates everywhere else; this makes --branch consistent, and keeps
// caller mistakes distinguishable from tool/network failures in the audit line
// (security review). `@{` and other exotica are already excluded by branchRe.
// Two further rejections come from the security review (#226) —
// non-blocking hygiene, both PRE-EXISTING. `branchRe` admits a `refs/`-prefixed value and
// the literal `HEAD`, and neither was rejected, so both reached the constructed refspec:
//
//   - `refs/heads/x` created the NESTED ref `refs/heads/refs/heads/x`. It does not touch
//     the real branch (verified byte-identical before/after) and name resolution still
//     returns the true branch — not an escape, but a confusing artifact.
//   - `HEAD` created `refs/heads/HEAD`, which makes git emit an "ambiguous refname"
//     warning on every subsequent resolution. `HEAD` itself still resolves to the checked-
//     out commit.
//
// Rejecting both removes the ambiguity-warning class entirely. `HEAD` and `refs/` are
// matched case-INSENSITIVELY: on a case-insensitive filesystem (macOS, where the desk
// runs) `refs/heads/head` and `refs/heads/HEAD` are the same file, so admitting `head`
// would leave the same ambiguity reachable through a case variant.
func branchRejectReason(b string) string {
	switch {
	case strings.EqualFold(b, "HEAD"):
		return "must not be the literal 'HEAD' (creates refs/heads/HEAD — an ambiguous refname)"
	case len(b) >= 5 && strings.EqualFold(b[:5], "refs/"):
		return "must not start with 'refs/' (creates a nested refs/heads/refs/… ref)"
	case strings.Contains(b, ".."):
		return "must not contain '..'"
	case strings.Contains(b, "//"):
		return "must not contain '//'"
	case strings.HasSuffix(b, "/"):
		return "must not end in '/'"
	case strings.HasSuffix(b, "."):
		return "must not end in '.'"
	case strings.HasSuffix(b, ".lock"):
		return "must not end in '.lock'"
	}
	return ""
}

// exactRepoPath requires the path (for a host-bearing URL) to be EXACTLY owner/repo.
func exactRepoPath(p string) (string, error) {
	p = strings.Trim(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("expected exactly owner/repo, got %q", p)
	}
	return parts[0] + "/" + parts[1], nil
}

// lenientRepoPath takes the last two path components (bare local path only).
func lenientRepoPath(p string) (string, error) {
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
