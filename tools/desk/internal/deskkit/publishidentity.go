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
// WHAT IT CHECKS. For every commit the push would publish — `refs/remotes/origin/<base>..HEAD`
// — the AUTHOR and the COMMITTER must both resolve to the session role's bound identity, by
// the SAME forge-aware rules preflight's commit-identity check applies to a worktree's config
// (forgeidentity.go): a GitHub identity's exact bot-USER-id noreply address, a GitLab
// service-account noreply SHAPE, or an explicitly trusted GitLab session address. A merge
// commit created by the forge itself (GitHub's "Merge pull request" / "Update branch",
// authored or committed as `noreply@github.com`) is EXEMPT — it is not the session's commit
// and never carries the role identity, so requiring it to would false-refuse a legitimately
// updated branch. There is deliberately no override flag: the remedy is to fix the worktree
// and re-author, not to wave the commit through.

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
	// Role is the session role the commits must be attributed to (worker, verifier, …).
	Role string
	// Commits, when non-nil, replaces the default git reader (test seam).
	Commits func(dir, base string) ([]PublishCommit, error)
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
	commits, err := read(in.Dir, in.Base)
	if err != nil {
		return Unverifiable(fmt.Sprintf(
			"publish-identity: cannot enumerate the commits refs/remotes/origin/%s..HEAD would publish (%s) — "+
				"could-not-check, never clean", in.Base, oneLine(err.Error())), err)
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
			return publishIdentityRefusal(ident, role, c, "authored", c.AuthorName, c.AuthorEmail)
		}
		if !emailMatchesRole(ident, c.CommitterEmail) {
			return publishIdentityRefusal(ident, role, c, "committed", c.CommitterName, c.CommitterEmail)
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
func publishIdentityRefusal(ident BotIdentity, role string, c PublishCommit, verb, foundName, foundEmail string) error {
	return Refused(fmt.Sprintf(
		"publish-identity: commit %s (%q) is %s by %s, but role %q publishes as %s — a commit carrying "+
			"another identity would be published under the wrong actor. Fix this worktree's identity and "+
			"re-author: `git commit --amend --reset-author` after correcting user.name/user.email, or "+
			"re-create the worktree with `deskwt role-init --role %s`. There is no override.",
		shortPublishSHA(c.SHA), c.Subject, verb, foundIdentity(foundName, foundEmail), role, expectedIdentity(ident), role))
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

// defaultPublishCommits reads the commits refs/remotes/origin/<base>..HEAD would publish,
// newest first, via `git log`. Fields are separated by US (\x1f) and commits by NUL (the
// -z record separator), so a subject can carry any character short of those two control
// bytes without breaking the parse. A base ref that does not resolve is returned as an
// error (git exits non-zero), which the caller reports as could-not-check.
func defaultPublishCommits(dir, base string) ([]PublishCommit, error) {
	const fieldSep = "\x1f"
	rangeSpec := "refs/remotes/origin/" + base + "..HEAD"
	format := strings.Join([]string{"%H", "%P", "%an", "%ae", "%cn", "%ce", "%s"}, fieldSep)
	out, err := gitOut(dir, "log", "--no-color", "-z", "--format="+format, rangeSpec)
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
