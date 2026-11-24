package deskkit

import (
	"fmt"
	"regexp"
	"strings"
)

// verifierhead.go — the pre-merge verifier target (#7623).
//
// A verifier run normally executes reviewed, MERGED code only: its source is
// the fetched origin/main commit. A pre-merge run executes an UNMERGED change
// head with the verifier's credential. That is unreviewed-by-merge code, so
// this file is the whole admission rule for it, and every rule fails closed:
//
//   - the repository opts in on the owner-held roster (EnvVerifierPremergeRepos);
//     unset, every repository refuses;
//   - the change is open and its CURRENT head is exactly the named head;
//   - the reviewer App's own decisive verdict AT THAT HEAD is APPROVED, no
//     reviewer-App CHANGES_REQUESTED stands at that head, and no other login's
//     review counts;
//   - the reviewer App's security lane has posted `Security-Review: pass` AT
//     THAT HEAD, and no security fail of its stands after that pass;
//   - the change's head branch lives in the base repository (never a fork),
//     and its author is a trusted public author on the roster (a role App or
//     an accountable human, id-pinned);
//   - the change author is none of the verifier, reviewer or dispatching
//     desk Apps, so the implementer is never the witness;
//   - the head is read again after the reviews, at admission and at every use
//     point (--check-verifier), so a moved head voids the admission;
//   - any forge read failure is could-not-check, never admitted;
//   - the run executes in a credential-less, contained runner. None exists
//     yet, so PremergeRunnerProblem refuses every pre-merge target and the
//     whole feature is INERT: a host-lane row runs as the desk's OS user and
//     can reach every same-user credential (stored forge logins, App key
//     files, the credential helper, the ssh home, same-user sockets), not
//     only the run's own read-only one (docs/verify-row-strength.md,
//     "What an admitted row can reach").

// VerifierHeadTarget names the unmerged change and the exact head commit a
// pre-merge verifier run is admitted at.
type VerifierHeadTarget struct {
	Change int
	Head   string
}

// IsZero reports whether no target was named (a merged-main run).
func (t VerifierHeadTarget) IsZero() bool { return t.Change == 0 && t.Head == "" }

var targetHeadRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Problem checks the target's shape: both halves, a positive change number
// and a full 40-hex lowercase commit (never an abbreviation or a ref name).
func (t VerifierHeadTarget) Problem() string {
	switch {
	case t.Change <= 0:
		return "a pre-merge target needs a positive change number (--target-change)"
	case !targetHeadRe.MatchString(t.Head):
		return "a pre-merge target needs the full 40-hex head commit (--target-head)"
	}
	return ""
}

// parsePremergeRepos parses EnvVerifierPremergeRepos. A malformed value lands
// in bad and refuses the whole roster; the returned set is then empty.
func parsePremergeRepos(raw string, bad func(format string, a ...any)) map[string]bool {
	out := map[string]bool{}
	invalid := false
	for _, entry := range splitList(raw) {
		key := strings.ToLower(strings.TrimSpace(entry))
		owner, name, ok := strings.Cut(key, "/")
		if !ok || owner == "" || name == "" || strings.Contains(name, "/") || strings.ContainsAny(key, "*?[] ") {
			invalid = true
			bad("%s: entry %q is not a full owner/name slug — this key decides which repository may run "+
				"unmerged code with the verifier's credential, so a basename or pattern is refused", EnvVerifierPremergeRepos, entry)
			continue
		}
		if out[key] {
			invalid = true
			bad("%s: repository %q is listed more than once", EnvVerifierPremergeRepos, entry)
			continue
		}
		out[key] = true
	}
	if invalid {
		return map[string]bool{}
	}
	return out
}

// VerifierPremergeOptedIn answers the repository's opt-in. An unconfigured or
// refused roster is could-not-check; a repository not listed, or not on the
// allowed repository set, is refused.
func VerifierPremergeOptedIn(repo string) error {
	c := EffectiveConfig()
	if !c.Configured() {
		return Unverifiable("pre-merge verifier admission could not read a configured roster", nil)
	}
	if !c.PremergeRepos[strings.ToLower(repo)] || !IsAllowedRepo(repo) {
		return Refused(fmt.Sprintf("pre-merge verifier runs are off for %s: the repository is not opted in on %s (default off)", repo, EnvVerifierPremergeRepos))
	}
	return nil
}

// premergeIdentities resolves the logins admission compares against. An
// unbound role is could-not-check: no comparison runs against an empty login.
func premergeIdentities() (reviewer string, excluded []string, err error) {
	reviewer, ok := RoleAppLogin("reviewer")
	if !ok {
		return "", nil, Unverifiable("pre-merge verifier admission needs the reviewer role bound on the roster", nil)
	}
	for _, role := range []string{"verifier", DispatcherRole} {
		login, ok := RoleAppLogin(role)
		if !ok {
			return "", nil, Unverifiable("pre-merge verifier admission needs the "+role+" role bound on the roster", nil)
		}
		excluded = append(excluded, login)
	}
	return reviewer, append(excluded, reviewer), nil
}

// premergeHead reads the change and requires it open at exactly t.Head.
func premergeHead(f Forge, repo ForgeRepo, t VerifierHeadTarget) (*PullRequest, error) {
	pr, err := f.GetPullRequest(repo, t.Change)
	if err != nil || pr == nil {
		return nil, Unverifiable(fmt.Sprintf("pre-merge verifier admission could not read change %d", t.Change), err)
	}
	if pr.Number != 0 && pr.Number != t.Change {
		return nil, Unverifiable(fmt.Sprintf("the forge answered change %d for change %d", pr.Number, t.Change), nil)
	}
	if pr.State != "open" {
		return nil, Refused(fmt.Sprintf("change %d is %q, not open: a pre-merge run is admitted only at an open change's head", t.Change, pr.State))
	}
	if pr.HeadSHA != t.Head {
		return nil, Refused(fmt.Sprintf("change %d's head moved: it is %s, not the admitted %s; the admission is void", t.Change, shortSHA(pr.HeadSHA), shortSHA(t.Head)))
	}
	return pr, nil
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	if s == "" {
		return "(none)"
	}
	return s
}

// premergeRunnerContained answers whether this run executes in a runner that
// holds no credential but the verifier's run-scoped read-only one. No such
// runner exists, so it is false. It is a package variable only so this
// package's tests can exercise the guards behind it; nothing outside the
// package can change it, and no production code sets it.
var premergeRunnerContained = func() bool { return false }

// PremergeRunnerProblem refuses a pre-merge target unless the run is
// contained. Until a credential-less runner lands, it refuses every one.
func PremergeRunnerProblem() error {
	if premergeRunnerContained() {
		return nil
	}
	return Refused("pre-merge verifier runs are inert: no credential-less, contained runner exists, and a host-lane " +
		"row runs as the desk's OS user with every same-user credential in reach (stored forge logins, App key files, " +
		"the credential helper, the ssh home), so unmerged code is never run there")
}

// AdmitVerifierHead is the pre-merge admission rule. It runs at dispatch,
// before the run record exists, and again at every use point. A nil error is
// the only admission.
func AdmitVerifierHead(f Forge, repo string, t VerifierHeadTarget) error {
	if why := t.Problem(); why != "" {
		return Refused(why)
	}
	if err := VerifierPremergeOptedIn(repo); err != nil {
		return err
	}
	reviewer, excluded, err := premergeIdentities()
	if err != nil {
		return err
	}
	if f == nil {
		return Unverifiable("pre-merge verifier admission has no forge to read", nil)
	}
	fr, err := verifierRepo(repo)
	if err != nil {
		return err
	}
	pr, err := premergeHead(f, fr, t)
	if err != nil {
		return err
	}
	author := pr.Author.Login
	if strings.TrimSpace(author) == "" {
		return Unverifiable(fmt.Sprintf("change %d's author could not be read", t.Change), nil)
	}
	for _, login := range excluded {
		if SameActor(author, login) {
			return Refused(fmt.Sprintf("change %d was authored by %s: the implementer is never the verifier's witness or approver", t.Change, author))
		}
	}
	if err := premergeOriginProblem(pr, t.Change); err != nil {
		return err
	}
	reviews, err := f.ReviewsAtHead(fr, t.Change)
	if err != nil {
		return Unverifiable(fmt.Sprintf("pre-merge verifier admission could not read change %d's reviews", t.Change), err)
	}
	if why := premergeApprovalProblem(reviewer, reviews, t.Head); why != "" {
		return Refused(fmt.Sprintf("change %d at %s: %s", t.Change, shortSHA(t.Head), why))
	}
	if why := premergeSecurityProblem(reviewer, reviews, t.Head); why != "" {
		return Refused(fmt.Sprintf("change %d at %s: %s", t.Change, shortSHA(t.Head), why))
	}
	// Read the head again AFTER the reviews: an approval read against a head
	// that moved while the reviews were fetched is not an approval of t.Head.
	if _, err := premergeHead(f, fr, t); err != nil {
		return err
	}
	// Last, so every other guard is still read and reported: the runner must be
	// contained (#7628 review S1). Today this refuses every target.
	return PremergeRunnerProblem()
}

// premergeApprovalProblem requires the reviewer App's standing decisive verdict
// at exactly head to be APPROVED, and no reviewer-App CHANGES_REQUESTED at that
// head at all: one App identity may carry several review lanes, so a later
// approval never launders an earlier request for changes at the same head.
// Reviews by any other login — a human, another App — are not read.
func premergeApprovalProblem(reviewer string, reviews []Review, head string) string {
	var app []AppReview
	for _, r := range reviews {
		if !SameActor(r.Author.Login, reviewer) {
			continue
		}
		if r.State == "CHANGES_REQUESTED" && r.CommitID == head {
			return "the reviewer App requested changes at this head"
		}
		app = append(app, AppReview{AuthorLogin: r.Author.Login, State: r.State, CommitID: r.CommitID, SubmittedAt: r.SubmittedAt})
	}
	switch ReduceAppVerdict(reviewer, app, head) {
	case AppVerdictApproved:
		return ""
	case AppVerdictStale:
		return "the reviewer App's latest verdict is not at this head"
	case AppVerdictNone:
		return "the reviewer App has not approved this head"
	default:
		return "the reviewer App's verdict at this head is not APPROVED"
	}
}

// premergeOriginProblem refuses a change whose head is not in the base
// repository, or whose author is not a trusted public author. A pre-merge run
// executes on the desk host as the desk's OS user, so the code must come from
// the repository itself and from an identity the roster already trusts. An
// unanswered head repository is could-not-check, never "same".
func premergeOriginProblem(pr *PullRequest, change int) error {
	switch pr.CrossRepo {
	case CrossRepoSame:
	case CrossRepoFork:
		return Refused(fmt.Sprintf("change %d's head is in a fork: a pre-merge run is admitted only for a branch in the base repository", change))
	default:
		return Unverifiable(fmt.Sprintf("change %d's head repository could not be read", change), nil)
	}
	login := pr.Author.Login
	if !TrustedPublicAuthor(login) || !TrustedAuthorID(login, pr.Author.ID) {
		return Refused(fmt.Sprintf("change %d's author %s is not a trusted role App or accountable human on the roster", change, login))
	}
	return nil
}

// premergeSecurityProblem requires the reviewer App's security lane to pass
// at exactly head. Reviews are reduced in submission order: a
// `Security-Review: fail` stands whatever commit it names until a later pass
// at head, and a fail AT head refuses even when a later pass exists (the same
// no-laundering rule as a request for changes). A pass at another commit
// grants nothing.
func premergeSecurityProblem(reviewer string, reviews []Review, head string) string {
	pass := false
	for _, r := range reviews {
		if !SameActor(r.Author.Login, reviewer) {
			continue
		}
		switch {
		case HasSecurityReviewFail(r.Body):
			if r.CommitID == head {
				return "the reviewer App's security lane failed this head"
			}
			pass = false
		case HasSecurityReviewPass(r.Body) && r.CommitID == head:
			pass = true
		}
	}
	if !pass {
		return "the reviewer App's security lane has not passed this head (`Security-Review: pass` at exactly this head)"
	}
	return ""
}

// ResolveVerifierHeadForge resolves the forge a dispatching desk reads the
// change and its reviews through at pre-merge admission. Recovery
// (RecoverVerifierAttestation) reads under the same role.
func ResolveVerifierHeadForge(repo string) (Forge, error) {
	fr, err := verifierRepo(repo)
	if err != nil {
		return nil, err
	}
	f, _, err := ResolveForge(fr, DispatcherRole)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// bindingHeadTarget returns the pre-merge target a binding records; zero for
// a merged-main binding.
func bindingHeadTarget(b VerifierBinding) VerifierHeadTarget {
	return VerifierHeadTarget{Change: b.TargetChange, Head: b.TargetHead}
}
