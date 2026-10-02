package deskkit

// publishidentity.go — the PUBLISH-TIME half of the wrong-worktree-identity class
// (the forge-qualified-identity stream; issue #1490).
//
// THE CLASS. A worktree can carry the wrong App identity in its `user.*` config —
// inherited from a shared checkout's stale value, set by a manual `git worktree add`, or
// left behind by an editor's git. Commits authored there read as a DIFFERENT role's bot
// than the session acting. Provisioning fixes (deskwt role-init, deskdispatch's
// worktree-create) stamp the right identity on NEW worktrees, which stops new cases; they
// cannot stop a case that arrives through any other route. This check is the second layer:
// the PUSH path itself refuses to publish a commit whose identity is not the session
// role's, so the class cannot recur however the worktree was made.
//
// WHY AT PUBLISH TIME, AND WHY BOTH LAYERS. The two layers trip on different signals at
// different moments: provisioning acts when a worktree is created (on the config it writes),
// this acts when a commit is about to leave the machine (on the commits themselves). A
// worktree provisioned correctly and later re-pointed at a stale identity passes the first
// and is caught by the second; a worktree never provisioned by the desk at all is caught
// only by the second. Neither is redundant with the other — they guard the same fault at
// two independent trust boundaries.
//
// WHAT IT CHECKS. For every commit the push would publish — `refs/remotes/origin/<base>..HEAD`,
// narrowed to exclude a verified remote tip (see WHICH COMMITS below) — the AUTHOR and the COMMITTER must both resolve to the session role's bound identity, by
// the SAME forge-aware rules preflight's commit-identity check applies to a worktree's config
// (forgeidentity.go): a GitHub identity's exact bot-USER-id noreply address, a GitLab
// service-account noreply SHAPE, or an explicitly trusted GitLab session address. A merge
// commit created by the forge itself (GitHub's "Merge pull request" / "Update branch",
// authored or committed as `noreply@github.com`) is EXEMPT — it is not the session's commit
// and never carries the role identity, so requiring it to would false-refuse a legitimately
// updated branch. There is deliberately no override flag: the remedy is to fix the worktree
// and re-author, not to wave the commit through.
//
// WHICH COMMITS — THE RANGE (issue #1967). The check judges the commits THIS push ADDS to the
// remote, not every commit on the branch. Walking the whole `origin/<base>..HEAD` re-judged
// commits the remote PR head already held — an earlier commit by a different trusted App on a
// mixed-author PR — and refused a push that published none of them, so the only way to update
// such a PR was a raw push around the gate. A caller may therefore offer a REMOTE TIP: the
// commit the remote branch this push updates already holds. The range then also excludes
// everything reachable from that tip — `HEAD ^refs/remotes/origin/<base> ^<tip>` — which is a
// SUBSET of the wide walk, never a superset: narrowing can only drop commits the remote
// already has, it can never add or reorder anything.
//
// The narrowing FAILS CLOSED to the wide walk, never open. The offered tip is used only when
// all three hold: (1) it is spelled as a full hex object name or a fully-qualified
// `refs/remotes/origin/…` remote-tracking ref — never `HEAD`, a local branch, an abbreviated
// sha or a revision expression, any of which could name the commit being pushed and empty the
// range; (2) it resolves to a commit in this repository — a first push (no remote branch yet)
// or an unfetched head has nothing to anchor on; (3) it is an ancestor of HEAD — a remote
// branch that was force-moved, or a tip this branch diverged from, cannot be fast-forwarded
// by deskpr's plain (never forced) push, so nothing about it is trusted to shrink the range.
// Any failure falls back to the whole `origin/<base>..HEAD`, and the refusal (if any) says the
// tip was offered and why it was not used. A STALE tracking ref that lags the remote is safe
// by construction: it is an ancestor of the real remote head, so the range it yields is a
// superset of what the push adds.

import (
	"fmt"
	"strings"
)

// CheckPublishIdentity is the check name this gate reports under, alongside the other
// local gates (the --check enumeration in deskpr, the preflight check table).
const CheckPublishIdentity = "publish-identity"

// PublishCommit is one commit the push would publish, reduced to the fields the identity
// check reads. Parents lets the check tell a merge commit (>=2 parents) from an ordinary
// one, so a forge-authored merge can be exempted without exempting an ordinary commit that
// merely happens to carry a forge address.
type PublishCommit struct {
	SHA            string
	Subject        string
	Parents        []string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
}

// PublishIdentityInput is the shared check's argument. Commits is a test seam: nil selects
// the default `git log` reader (defaultPublishCommits); a test supplies its own slice so the
// identity logic is exercised without building a real repository for every case.
type PublishIdentityInput struct {
	// Dir is the worktree whose HEAD is about to be published.
	Dir string
	// Base is the plain base branch name; the published range is
	// refs/remotes/origin/<Base>..HEAD. The remote-tracking ref is spelled in FULL so a
	// stray local branch literally named `origin/<base>` cannot shadow it (the same
	// ambiguity preflight and the worker lineage self-check guard against).
	Base string
	// RemoteTip, when non-empty, names the commit the REMOTE branch this push updates already
	// holds (#1967): a full hex object name the forge reported for the PR head, or the
	// fully-qualified remote-tracking ref `refs/remotes/origin/<branch>`. The range then also
	// excludes everything reachable from it, so commits already on the remote are not
	// re-judged. It is used only when it passes the fail-closed rules in the file header;
	// otherwise — and when it is empty — the whole refs/remotes/origin/<Base>..HEAD is judged.
	// Every caller states it explicitly (TestPubIdentityCallersTip).
	RemoteTip string
	// Role is the session role the commits must be attributed to (worker, verifier, …).
	Role string
	// Commits, when non-nil, replaces the default git reader (test seam). It receives the
	// resolved range and must honour it.
	Commits func(dir string, rng PublishRange) ([]PublishCommit, error)
}

// PublishRange is the resolved set of commits the gate judges: everything reachable from
// HEAD and not from Base, nor — when Tip is set — from Tip.
type PublishRange struct {
	// Base is the fully-qualified remote-tracking base ref, refs/remotes/origin/<base>.
	Base string
	// Tip is the resolved full object name of the remote tip excluded from the range, or ""
	// when the range is the whole Base..HEAD.
	Tip string
	// Offered is the RemoteTip the caller offered, verbatim ("" when none).
	Offered string
	// Widened says why an offered tip was NOT used, so a refusal can name it ("" when no tip
	// was offered or the offered tip was used).
	Widened string
}

// Narrowed reports whether the range excludes a remote tip beyond the base.
func (r PublishRange) Narrowed() bool { return r.Tip != "" }

// revArgs is the `git log` revision set for the range.
func (r PublishRange) revArgs() []string {
	args := []string{"HEAD", "^" + r.Base}
	if r.Tip != "" {
		args = append(args, "^"+r.Tip)
	}
	return args
}

// String renders the range for a message.
func (r PublishRange) String() string {
	if r.Tip == "" {
		return r.Base + "..HEAD"
	}
	return r.Base + "..HEAD excluding the remote tip " + shortPublishSHA(r.Tip)
}

// remoteTipRefPrefix is the only ref namespace an offered tip may be spelled in: the remote
// this push goes to. A local branch (refs/heads/…) or another remote's ref is not evidence
// of what origin holds.
const remoteTipRefPrefix = "refs/remotes/origin/"

// tipSpellingProblem returns why an offered tip is not an acceptable SPELLING, or "" when it
// is: a full 40- or 64-hex object name, or refs/remotes/origin/<name> built only from ref-safe
// characters with no revision syntax (`..`, `^`, `~`, `@{`, `:`) that could make it name
// something other than the ref itself.
func tipSpellingProblem(tip string) string {
	if isFullHexObjectName(tip) {
		return ""
	}
	if !strings.HasPrefix(tip, remoteTipRefPrefix) || len(tip) == len(remoteTipRefPrefix) {
		return "it is neither a full object name nor a fully-qualified " + remoteTipRefPrefix + "<branch> ref"
	}
	if strings.Contains(tip, "..") || strings.HasSuffix(tip, "/") || strings.HasSuffix(tip, ".lock") {
		return "it is not a well-formed ref name"
	}
	for _, r := range tip {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '/' || r == '-' || r == '_' || r == '.'
		if !ok {
			return "it carries a character outside a plain ref name (revision syntax is not accepted)"
		}
	}
	return ""
}

// isFullHexObjectName reports whether s is a full SHA-1 (40) or SHA-256 (64) hex object name.
func isFullHexObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// resolvePublishRange applies the fail-closed narrowing rules (file header) to an offered
// remote tip. It never errors: every reason the tip cannot be used widens the range to the
// whole Base..HEAD and is recorded in Widened. A base ref that does not resolve is left for
// the commit reader to report as could-not-check, exactly as before the narrowing existed.
func resolvePublishRange(dir, base, tip string) PublishRange {
	rng := PublishRange{Base: "refs/remotes/origin/" + base, Offered: tip}
	tip = strings.TrimSpace(tip)
	if tip == "" {
		return rng
	}
	if why := tipSpellingProblem(tip); why != "" {
		rng.Widened = "the offered remote tip " + fmt.Sprintf("%q", tip) + " was not used: " + why
		return rng
	}
	out, err := gitOut(dir, "rev-parse", "--verify", "--quiet", tip+"^{commit}")
	sha := strings.TrimSpace(out)
	if err != nil || !isFullHexObjectName(sha) {
		rng.Widened = "the offered remote tip " + fmt.Sprintf("%q", tip) + " was not used: it does not resolve to a " +
			"commit here (a first push, or a remote head not fetched yet — `git fetch origin` and retry)"
		return rng
	}
	if _, aerr := gitOut(dir, "merge-base", "--is-ancestor", sha, "HEAD"); aerr != nil {
		rng.Widened = "the offered remote tip " + fmt.Sprintf("%q", tip) + " was not used: it is not an ancestor of HEAD " +
			"(the remote branch was force-moved, or this branch diverged from it)"
		return rng
	}
	rng.Tip = strings.ToLower(sha)
	return rng
}

// PublishIdentityMatchesRole refuses (exit 5) when any commit the push would publish is not
// authored AND committed by the session role's bound identity. It is the ONE shared gate the
// pushing verbs (deskpr create/update) and the Evidence-landing verb (deskevidence) call
// before any network write.
//
// The verdict is three-state, per the codebase's instrument rule:
//   - an unbound role, or a commit carrying a foreign identity, is REFUSED (exit 5);
//   - a GitHub role whose bot USER id the roster does not pin cannot be checked, so it is
//     UNVERIFIABLE (exit 6) — could-not-check, never rounded up to a pass;
//   - an empty range (nothing to publish) and every commit matching are clean (nil).
func PublishIdentityMatchesRole(in PublishIdentityInput) error {
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if role == "" {
		return Unverifiable("publish-identity: no session role to check commits against — $DESK_LOOP resolves to no App role", nil)
	}
	ident, bound := EffectiveConfig().RoleBotIdentity(role)
	if !bound {
		return Refused(fmt.Sprintf(
			"publish-identity: the roster binds no identity to role %q, so the commits about to be published "+
				"cannot be attributed to it — add %s=<forge>:<slug-or-login>[:<id>] to %s in %s",
			role, role, EnvTrustedBotSlugs, ConfigHomePath()))
	}

	// A GitHub identity with no pinned bot USER id yields no address to compare against, so
	// the check cannot be made — could-not-check, exactly as preflight's commit-identity
	// check reports it. GitLab identities validate by SHAPE, so they are never blocked here.
	spec := ident.CommitEmailSpec()
	if ident.Forge == ForgeGitHub && !spec.Derivable {
		return Unverifiable(fmt.Sprintf(
			"publish-identity: the roster pins no bot USER id for %s, so a published commit's author cannot be "+
				"verified against it — pin it: %s entry %s=github:%s:<bot-user-id>",
			ident.Slug, EnvTrustedBotSlugs, role, ident.Slug), nil)
	}

	read := in.Commits
	if read == nil {
		read = defaultPublishCommits
	}
	rng := resolvePublishRange(in.Dir, in.Base, in.RemoteTip)
	commits, err := read(in.Dir, rng)
	if err != nil {
		return Unverifiable(fmt.Sprintf(
			"publish-identity: cannot enumerate the commits %s would publish (%s) — "+
				"could-not-check, never clean", rng.String(), oneLine(err.Error())), err)
	}

	for _, c := range commits {
		// A forge-authored merge (GitHub "Merge pull request" / "Update branch") is not the
		// session's commit and never carries the role identity — exempt it, but ONLY when it
		// really is a merge (>=2 parents), so an ordinary commit carrying a forge address is
		// still checked.
		if len(c.Parents) >= 2 && (IsForgeMergeIdentity(c.AuthorEmail) || IsForgeMergeIdentity(c.CommitterEmail)) {
			continue
		}
		if !emailMatchesRole(ident, c.AuthorEmail) {
			return publishIdentityRefusal(ident, role, c, rng, "authored", c.AuthorName, c.AuthorEmail)
		}
		if !emailMatchesRole(ident, c.CommitterEmail) {
			return publishIdentityRefusal(ident, role, c, rng, "committed", c.CommitterName, c.CommitterEmail)
		}
	}
	return nil
}

// emailMatchesRole reports whether a commit email resolves to this role's identity, by the
// same forge-aware rules preflight's commit-identity check applies. On GitHub the address
// must be the exact bot-USER-id noreply form. On GitLab it may be the service-account
// noreply SHAPE, or an explicitly trusted session/implementer address (the two-identity
// path, #643) — and the session allowlist is consulted ONLY for a GitLab identity, never a
// GitHub one, so the #638 bot-USER-id guarantee is untouched.
func emailMatchesRole(ident BotIdentity, email string) bool {
	if ident.Forge == ForgeGitLab && GitLabSessionEmailAllowed(email) {
		return true
	}
	return ident.CommitEmailSpec().Accepts(email)
}

// publishIdentityRefusal renders the exit-5 refusal, naming the commit, the identity found,
// the identity expected, and the remedy. There is no override flag — the fix is to correct
// the worktree and re-author the commit, not to wave it through.
func publishIdentityRefusal(ident BotIdentity, role string, c PublishCommit, rng PublishRange, verb, foundName, foundEmail string) error {
	msg := fmt.Sprintf(
		"publish-identity: commit %s (%q) is %s by %s, but role %q publishes as %s — a commit carrying "+
			"another identity would be published under the wrong actor. Fix this worktree's identity and "+
			"re-author: `git commit --amend --reset-author` after correcting user.name/user.email, or "+
			"re-create the worktree with `deskwt role-init --role %s`. There is no override. Range judged: %s.",
		shortPublishSHA(c.SHA), c.Subject, verb, foundIdentity(foundName, foundEmail), role, expectedIdentity(ident), role,
		rng.String())
	if rng.Widened != "" {
		// Name why the narrower range was not used: a commit the remote branch already holds is
		// only re-judged here because its tip could not be trusted, and the remedy differs.
		msg += " Note: " + rng.Widened + ", so the whole range was judged."
	}
	return Refused(msg)
}

// foundIdentity renders the offending "<name> <email>" pair, tolerating an empty half so a
// commit missing one still names the other rather than printing an empty string.
func foundIdentity(name, email string) string {
	name, email = strings.TrimSpace(name), strings.TrimSpace(email)
	switch {
	case name == "" && email == "":
		return "an empty identity"
	case name == "":
		return "<" + email + ">"
	case email == "":
		return name
	default:
		return name + " <" + email + ">"
	}
}

// expectedIdentity renders what the role's commits must carry, forge-appropriately: a
// GitHub identity names its exact bot-USER-id address, a GitLab one names the
// service-account noreply shape (and the trusted-session alternative), since neither the
// group id nor the suffix is derivable from the roster.
func expectedIdentity(ident BotIdentity) string {
	spec := ident.CommitEmailSpec()
	if spec.Forge == ForgeGitLab {
		return ident.PrimaryLogin() + " (the service-account noreply address " +
			"service_account_group_<group-id>_<suffix>@noreply.<host>, or an address listed in " +
			EnvGitLabSessionEmails + ")"
	}
	if spec.Derivable {
		return ident.PrimaryLogin() + " (" + spec.Exact + ")"
	}
	return ident.PrimaryLogin()
}

// IsForgeMergeIdentity reports whether email is a forge's own merge-commit identity — the
// address a forge stamps on a merge it creates on the operator's behalf, which is not the
// session's commit and never carries a role identity. GitHub uses `noreply@github.com` for
// both "Merge pull request" (as committer) and "Update branch" (as author and committer);
// GitLab attributes a UI merge to the acting user, so it has no distinct system address to
// list here.
func IsForgeMergeIdentity(email string) bool {
	return strings.EqualFold(strings.TrimSpace(email), "noreply@github.com")
}

// defaultPublishCommits reads the commits the resolved range would publish (refs/remotes/
// origin/<base>..HEAD, minus the remote tip when one was accepted), newest first, via `git log`. Fields are separated by US (\x1f) and commits by NUL (the
// -z record separator), so a subject can carry any character short of those two control
// bytes without breaking the parse. A base ref that does not resolve is returned as an
// error (git exits non-zero), which the caller reports as could-not-check.
func defaultPublishCommits(dir string, rng PublishRange) ([]PublishCommit, error) {
	const fieldSep = "\x1f"
	format := strings.Join([]string{"%H", "%P", "%an", "%ae", "%cn", "%ce", "%s"}, fieldSep)
	args := append([]string{"log", "--no-color", "-z", "--format=" + format}, rng.revArgs()...)
	// `--` ends the revision list, so no revision can ever be read as a path.
	args = append(args, "--")
	out, err := gitOut(dir, args...)
	if err != nil {
		return nil, err
	}
	var commits []PublishCommit
	for _, rec := range strings.Split(out, "\x00") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		f := strings.Split(rec, fieldSep)
		if len(f) < 7 {
			return nil, fmt.Errorf("unparseable git log record (%d fields): %q", len(f), rec)
		}
		var parents []string
		if p := strings.TrimSpace(f[1]); p != "" {
			parents = strings.Fields(p)
		}
		commits = append(commits, PublishCommit{
			SHA:            strings.TrimSpace(f[0]),
			Parents:        parents,
			AuthorName:     f[2],
			AuthorEmail:    f[3],
			CommitterName:  f[4],
			CommitterEmail: f[5],
			// The subject is the last field; it never contains the field separator, and -z
			// makes NUL (not newline) the record boundary, so it is taken whole.
			Subject: f[6],
		})
	}
	return commits, nil
}

// shortPublishSHA renders a commit sha at a readable width without assuming a length.
func shortPublishSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
