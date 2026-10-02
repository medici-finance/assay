package deskkit

// preflight.go — the desk's OPERATING-ENVELOPE check, run once at boot, before
// any work is claimed.
//
// Roughly one in five of the verifier desk's open issues is not a finding about
// the work; it is the desk discovering its own envelope MID-PASS:
//
//   - #794 — the App pem lives in a directory the minter never looks in, so a
//     fresh shell cannot mint once the ~1h token cache lapses.
//   - #571 — the App's installation is missing the scope the role's duty needs
//     (PR comment), discovered at comment time.
//   - #823 — no permitted outward write transport at LANDING time: the pass is
//     complete, correct and unlandable.
//   - #638 — weeks of commits under an email whose numeric prefix is the App id
//     instead of the bot USER id, so nothing is account-linked.
//   - #679 — a sibling checkout a queued brief's rows need is simply not there.
//
// Each of those cost a whole pass and produced an issue ABOUT THE DESK. A
// boot-time preflight converts every one of them into a single loud line before
// anything is claimed.
//
// Three properties are the whole point and each is pinned by a test:
//
//  1. THREE STATE, ZERO VALUE RED. Every check answers checked-clean,
//     checked-failed, or could-not-check. The zero value of CheckState is
//     could-not-check, so a check that returns without looking can never read
//     green by omission.
//  2. A PREFLIGHT FAILURE IS COULD-NOT-RUN FOR THE WHOLE PASS. The desk prints
//     ONE line and stops. It does not claim work, burn a pass, or file an issue
//     about its own envelope — the envelope issues above already exist.
//  3. A PROBE REJECTION IS A STOP (AGENTS.md, "Scope rejections"). There is no
//     retry-under-another-identity path in this file, and
//     TestPreflightProbeRejectionIsNotRetried pins that the probe is invoked at
//     most once.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// CheckState is one check's three-state answer.
//
// The zero value is deliberately CouldNotCheck, not CheckedClean: a check that
// returns its result struct without ever looking must read RED. "Green by
// omission" is how a preflight becomes a rubber stamp.
type CheckState int

const (
	// CouldNotCheck — the check could not look. NOT a pass. This is the zero value.
	CouldNotCheck CheckState = iota
	// CheckedClean — the check looked and the envelope is intact.
	CheckedClean
	// CheckedFailed — the check looked and the envelope is broken.
	CheckedFailed
	// CheckedNotApplicable — the check does not apply to THIS envelope, on a
	// forge/config axis the check itself is authoritative about — e.g. the GitHub
	// App installation-grant read of app-scopes-vs-duties on a GitLab-forge repo,
	// where a PAT records no per-mint permission sidecar to read (#671). It is
	// NOT a pass in the checked-clean sense (nothing was verified) and NOT a
	// could-not-check (the check DID look — it looked and found the check
	// inapplicable), so it is a distinct fourth state: it does not redden the
	// envelope, and it is surfaced explicitly rather than folded into the
	// checked-clean count, so "green because inapplicable" can never be read as
	// "green because verified". A future author must not reach for this to paper
	// over an unreadable grant on the forge the check DOES cover — that is
	// could-not-check, and rounding it up here would be the exact green-by-omission
	// failure the three-state contract exists to stop.
	CheckedNotApplicable
)

// String renders the state in the fixed vocabulary the desk reports in
// ("checked-clean" / "checked-failed" / "could-not-check" / "not-applicable").
// An out-of-range value renders as could-not-check rather than as an unknown
// token, keeping the fail-closed reading of a corrupted value.
func (s CheckState) String() string {
	switch s {
	case CheckedClean:
		return "checked-clean"
	case CheckedFailed:
		return "checked-failed"
	case CheckedNotApplicable:
		return "not-applicable"
	default:
		return "could-not-check"
	}
}

// Green reports whether this state was VERIFIED clean. ONLY CheckedClean is. A
// not-applicable check is NOT green: nothing was verified, so it must never be
// counted as a checked-clean pass. Use Passing to ask the different question
// "does this state permit the pass to proceed".
func (s CheckState) Green() bool { return s == CheckedClean }

// Passing reports whether this state permits the pass to proceed. A verified
// CheckedClean does, and so does CheckedNotApplicable — a check that does not
// apply to this envelope is not a broken envelope. CouldNotCheck and
// CheckedFailed do NOT: an unread envelope and a broken one both block. Passing
// is deliberately WIDER than Green so a not-applicable check clears the boot
// without ever being miscounted as verified.
func (s CheckState) Passing() bool { return s == CheckedClean || s == CheckedNotApplicable }

// Check names of the six envelope checks. They are exported constants because
// the summary line, the tests and the consumers all refer to the same names —
// a check renamed in one place and not another is a check that silently stops
// being reported on.
const (
	CheckColdMint       = "token-mint-cold"
	CheckAppScopes      = "app-scopes-vs-duties"
	CheckWriteTransport = "write-transport"
	CheckCommitIdentity = "commit-identity"
	CheckSiblings       = "sibling-checkouts"
	CheckAmbientID      = "ambient-identity"
)

// Check is one envelope check's result.
//
// Remediation is a NAMED fix, not a diagnosis: the reader must be able to act on
// it without opening an issue. It is required whenever State is not
// CheckedClean; NewCheck refuses to build a non-green result without one, so the
// "it's broken, good luck" result shape does not exist.
type Check struct {
	Name        string
	State       CheckState
	Detail      string
	Remediation string
	// Refs are the issue numbers this check exists because of, rendered into the
	// summary so the reader can see the failure already has a home and does NOT
	// need a new issue filed about it.
	Refs string
	// Notice is an informational message: something the reader should see that
	// does NOT itself block the pass. It usually rides a green check, but it can
	// ride a red one too — the ambient-identity check keeps its human-login
	// warning on every transport outcome, so a transport failure never hides it.
	// A sibling checkout an
	// UNCLAIMED cross-repo brief declares is absent at boot is a notice, not a
	// failure (#661) — the brief is not being claimed now, so the pass proceeds;
	// the notice records that claiming it later needs that checkout.
	Notice string
}

// clean builds a green result. A green check carries no remediation by construction.
func clean(name, detail, refs string) Check {
	return Check{Name: name, State: CheckedClean, Detail: detail, Refs: refs}
}

// failed builds a checked-failed result: the check LOOKED and the envelope is broken.
func failed(name, detail, remediation, refs string) Check {
	return Check{Name: name, State: CheckedFailed, Detail: detail, Remediation: remediation, Refs: refs}
}

// unchecked builds a could-not-check result: the check could not look. It is
// NOT a pass, and it carries the remediation that would let it look next time.
func unchecked(name, detail, remediation, refs string) Check {
	return Check{Name: name, State: CouldNotCheck, Detail: detail, Remediation: remediation, Refs: refs}
}

// notApplicable builds a not-applicable result: the check LOOKED and found it
// does not apply to this envelope (a forge/config axis the check is
// authoritative about). It does not redden the envelope, but it is not a
// checked-clean pass either — nothing was verified — so it carries the
// remediation naming what a human should confirm out of band, and it is
// surfaced on its own in the summary rather than folded into the checked-clean
// count.
func notApplicable(name, detail, remediation, refs string) Check {
	return Check{Name: name, State: CheckedNotApplicable, Detail: detail, Remediation: remediation, Refs: refs}
}

// PreflightReport is the whole envelope answer for one role.
type PreflightReport struct {
	Role   string
	Checks []Check
}

// Green reports whether the envelope permits the pass to proceed: every check is
// PASSING (checked-clean, or not-applicable to this envelope). An empty report is
// NOT green: a preflight that ran no checks proved nothing. A not-applicable
// check does not block the boot, but it is not counted as verified — see
// SummaryLine, which reports the checked-clean tally separately and surfaces each
// not-applicable check on its own.
func (r PreflightReport) Green() bool {
	if len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.State.Passing() {
			return false
		}
	}
	return true
}

// Blocking returns the checks that block the pass — could-not-check and
// checked-failed — in check order. A not-applicable check is NOT blocking (it is
// surfaced by NotApplicable instead), so it never appears here.
func (r PreflightReport) Blocking() []Check {
	var out []Check
	for _, c := range r.Checks {
		if !c.State.Passing() {
			out = append(out, c)
		}
	}
	return out
}

// NotApplicable returns the checks that looked and found themselves inapplicable
// to this envelope, in check order. They do not block the pass, but they are
// surfaced explicitly (SummaryLine renders them) so a green boot that rests on an
// inapplicable check can never be mistaken for one where the check verified clean.
func (r PreflightReport) NotApplicable() []Check {
	var out []Check
	for _, c := range r.Checks {
		if c.State == CheckedNotApplicable {
			out = append(out, c)
		}
	}
	return out
}

// Notices returns the informational notices attached to checks (usually green
// ones, see Check.Notice), in check order, each prefixed with the check name. A
// notice is something the
// reader should see that does NOT block the pass — an unclaimed brief's absent
// cross-repo sibling, say (#661) — so it is reported even on a GREEN preflight.
func (r PreflightReport) Notices() []string {
	var out []string
	for _, c := range r.Checks {
		if c.Notice != "" {
			out = append(out, c.Name+": "+c.Notice)
		}
	}
	return out
}

// SummaryLine is the ONE line a red preflight prints. It is one line by
// construction — every embedded newline and carriage return is collapsed to a
// space — because the contract is "report one line and stop", and a probe's
// multi-line stderr pasted into a boot message is how one line becomes forty.
func (r PreflightReport) SummaryLine() string {
	green := 0
	for _, c := range r.Checks {
		if c.State.Green() {
			green++
		}
	}
	verdict := "RED"
	if r.Green() {
		verdict = "GREEN"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "preflight role=%s %s %d/%d checked-clean", r.Role, verdict, green, len(r.Checks))
	for _, c := range r.Blocking() {
		fmt.Fprintf(&b, " · %s=%s: %s → fix: %s", c.Name, c.State, c.Detail, c.Remediation)
		if c.Refs != "" {
			fmt.Fprintf(&b, " [%s]", c.Refs)
		}
	}
	for _, c := range r.NotApplicable() {
		fmt.Fprintf(&b, " · %s=%s: %s", c.Name, c.State, c.Detail)
		if c.Remediation != "" {
			fmt.Fprintf(&b, " → confirm: %s", c.Remediation)
		}
		if c.Refs != "" {
			fmt.Fprintf(&b, " [%s]", c.Refs)
		}
	}
	for _, n := range r.Notices() {
		fmt.Fprintf(&b, " · NOTICE %s", n)
	}
	return oneLine(b.String())
}

// oneLine collapses any newline/CR/tab run into a single space so a captured
// stderr can never turn the summary into a multi-line boot dump.
func oneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// Err maps a non-green report to the canonical COULD-NOT-RUN refusal.
//
// Both non-green states map to ExitUnverifiable (6) on purpose. From the pass's
// point of view there is no difference between "the envelope is broken" and "we
// could not tell whether the envelope is broken": in both cases the pass did not
// run and must not be recorded as having run. Collapsing checked-failed to a
// different, softer code is how a red envelope becomes a retried one.
func (r PreflightReport) Err() error {
	if r.Green() {
		return nil
	}
	return Unverifiable(r.SummaryLine(), nil)
}

// --- probes ----------------------------------------------------------------

// ProbeVerdict is what a READ-ONLY write-transport probe learned.
//
// ProbeRejected is a STOP (AGENTS.md, "Scope rejections"): the caller reports it
// and halts. It never re-runs the probe under a different identity, and this
// package supplies no way to.
type ProbeVerdict int

const (
	// ProbeInconclusive — the probe could not reach a verdict (no git, no remote,
	// transport error). could-not-check, never a pass.
	ProbeInconclusive ProbeVerdict = iota
	// ProbePermitted — the transport authenticated and the landing ref would be accepted.
	ProbePermitted
	// ProbeRejected — the transport is denied. A STOP.
	ProbeRejected
)

// Landing is the role's landing path: where this pass's output would be written.
type Landing struct {
	// Dir is the working tree the probe runs in ("" → the process's cwd).
	Dir string
	// Remote is the git remote name the landing targets.
	Remote string
	// Branch is the landing branch. Empty means "this worktree's current branch",
	// resolved at probe time; a DETACHED worktree therefore probes inconclusive
	// rather than guessing a branch.
	Branch string
}

// PreflightProbes are the injectable edges of the six checks. A nil field is
// filled with the real, environment-backed probe — tests supply their own so the
// suite is hermetic (no network, no credentials, no git remote).
type PreflightProbes struct {
	// ColdMint mints the role's GitHub App token in a FRESH process with a
	// scrubbed environment and returns the TOKEN FILE PATH (never the token
	// value). repo is the owner/name slug whose INSTALLATION the token is minted
	// against; empty means the minter applies its own default. It is consulted
	// ONLY on the GitHub custody path — a GitLab-forge repo takes GitLabColdCustody.
	ColdMint func(role, repo string) (tokenPath string, err error)
	// ResolveForgeKind reports which forge serves the role's repo, so the
	// cold-mint and app-scopes checks pick the right custody path (#655). It
	// answers from the repo's configured forge (ASSAY_REPO_FORGES) then the origin
	// remote's host, and returns "" (unresolved) when neither answers — unresolved
	// is treated as GitHub, the historical default, so a GitHub adopter with no
	// forge config is unaffected. It is never a caller-supplied forge selector:
	// the default wraps the ONE resolver (forgeresolve.go's resolveForgeKind).
	ResolveForgeKind func(repo string) ForgeKind
	// GitLabColdCustody verifies — READ-ONLY, without rotating — that a fresh
	// `desktoken --forge gitlab <role>` would have what it needs: the role's 0600
	// gitlab-<role>.token on the App-credential search path, non-empty, and
	// GITLAB_API_BASE set. It returns the custody file PATH (never the token
	// value). It NEVER rotates: rotation invalidates the live PAT, so a boot probe
	// that ran it would silently kill a working credential on every boot (#655).
	GitLabColdCustody func(role string) (tokenPath string, err error)
	// GrantedScopes returns the permission map GitHub granted the installation
	// the token was minted for. tokenPath is what ColdMint returned; an empty
	// tokenPath means the mint produced nothing to read scopes from.
	GrantedScopes func(role, tokenPath string) (map[string]string, error)
	// WriteTransport performs ONE read-only probe of the landing path.
	// Called at most once per preflight — see the ProbeRejected doc.
	WriteTransport func(l Landing) (ProbeVerdict, string, error)
	// CommitEmail returns the commit author email this worktree would commit under.
	CommitEmail func(dir string) (string, error)
	// AppIDFor returns the role's GitHub App id — the value that must NOT appear
	// as the commit email's numeric prefix (#638).
	AppIDFor func(role string) (string, error)
	// QueuedSiblings returns the sibling checkouts the QUEUED briefs declare.
	QueuedSiblings func(root string) ([]SiblingReq, error)
	// SiblingRoots returns the multi-repo roots the desk is CONFIGURED to trust
	// (DESK_ROOTS / the topology map). A declared `../<repo>` sibling resolves to
	// that repo's configured checkout path rather than a flat `../<repo>` next to
	// the desk root (#661) — the flat layout is only the fallback. nil → the real
	// ConfiguredRoots.
	SiblingRoots func() ([]RootConfig, error)
	// DirExists reports whether a directory is present and readable.
	DirExists func(path string) (bool, error)
	// AmbientLogin returns the login the AMBIENT `gh` credential would post/push
	// as — the identity a tool fall-through would silently use. It is the value
	// `gh api user` reports, NOT a minted App token: the check exists to catch an
	// ambient credential that is a bot slug, and to warn on a human one. It
	// returns "" with no error, OR an error wrapping ErrNoAmbientIdentity, when
	// there is no usable ambient human identity (`gh` not logged in; a credential
	// that answers 401/403 on /user, i.e. an App/integration token) — nothing to
	// fall through to as a person, safe. Any other error means it could not look
	// (gh absent, exec failure, timeout, a server or rate-limit answer) —
	// could-not-check.
	AmbientLogin func() (login string, err error)
	// CredHelperMatchesApp reports whether every credential source git consults
	// for the landing remote's push URL — the ordered helper chain across all
	// config scopes, plus an embedded URL credential or an Authorization
	// extraHeader, which git uses ahead of any helper — is the SAME minted App
	// token the pass lands under (appTokenPath). When it is, the write-transport
	// probe (check 3) and a real push from this envelope present the same
	// credential; when any other source could answer first, the two can disagree
	// and the check is red. detail names what was found. An error is
	// could-not-check.
	CredHelperMatchesApp func(l Landing, appTokenPath string) (matches bool, detail string, err error)
}

// SiblingReq is one sibling checkout a queued brief declares it needs, with the
// brief that declared it so a missing checkout names its claimant.
type SiblingReq struct {
	Brief string
	// Rel is the declared path, relative to the repo root (e.g. "../tracker").
	Rel string
}

// PreflightRequest is a fully-specified preflight. Preflight(role) is the
// convenience wrapper for the common case.
type PreflightRequest struct {
	Role string
	Root string
	// Repo is the owner/name slug the role operates on. Empty means "derive it
	// from the landing remote" — an App token is minted against an INSTALLATION,
	// and a probe that lets the minter fall back to its built-in default owner
	// tests an installation the pass will never use.
	Repo    string
	Landing Landing
	// ClaimedBrief, when non-empty, puts the sibling-checkout check in CLAIM mode:
	// an absent sibling declared by THIS brief is a hard failure, while an absent
	// sibling declared by any OTHER (unclaimed) queued brief degrades to a notice
	// (#661). Empty is BOOT mode — nothing is claimed yet, so every absent sibling
	// is a notice and none blocks boot. The value is matched as a substring of the
	// brief's stream-relative path, so a caller may name the brief by number
	// ("43"), file name ("brief-43-x.md"), or full path.
	ClaimedBrief string
	Probes       PreflightProbes
}

// Preflight runs the six envelope checks for a role against the current
// working tree and environment, in the order a pass would hit them.
//
// It is the ONE entry point desks call at boot:
//
//	rep := deskkit.Preflight("verifier")
//	if err := rep.Err(); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(deskkit.ExitCodeOf(err)) }
//
// Nothing is claimed, nothing is filed, and the process stops.
func Preflight(role string) PreflightReport {
	return PreflightRequest{Role: role, Root: "."}.Run()
}

// Run executes the six checks. It never returns an error: every failure mode is
// a Check with a state and a remediation, because an error return is the one
// shape a caller can drop on the floor and still proceed.
func (req PreflightRequest) Run() PreflightReport {
	role := strings.ToLower(strings.TrimSpace(req.Role))
	root := req.Root
	if root == "" {
		root = "."
	}
	l := req.Landing
	if l.Remote == "" {
		l.Remote = "origin"
	}
	if l.Dir == "" {
		l.Dir = root
	}
	p := req.Probes.withDefaults()

	rep := PreflightReport{Role: role}
	if role == "" {
		rep.Checks = []Check{unchecked(CheckColdMint, "no role given",
			"call preflight with the desk role this session acts as (reviewer/verifier/worker/desk/issue-loop/intake-loop)", "")}
		return rep
	}

	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		repo = deriveRepoSlug(l.Dir, l.Remote)
	}

	// Which forge serves this repo decides the credential custody path the
	// cold-mint and app-scopes checks take. It is RESOLVED (from ASSAY_REPO_FORGES
	// then the remote host), never a caller's choice; unresolved reads as GitHub,
	// the historical default, so a GitHub adopter with no forge config is
	// unaffected (#655).
	forge := p.ResolveForgeKind(repo)

	tokenPath, mint := checkColdMint(p, role, repo, forge)
	rep.Checks = append(rep.Checks,
		mint,
		checkAppScopes(p, role, tokenPath, forge),
		checkWriteTransport(p, l),
		checkCommitIdentity(p, role, l.Dir),
		checkSiblings(p, root, req.ClaimedBrief),
		checkAmbientIdentity(p, l, tokenPath, forge),
	)
	return rep
}

// withDefaults fills every nil probe with the real environment-backed one.
func (p PreflightProbes) withDefaults() PreflightProbes {
	if p.ColdMint == nil {
		p.ColdMint = coldMintProbe
	}
	if p.ResolveForgeKind == nil {
		p.ResolveForgeKind = ForgeKindForRepo
	}
	if p.GitLabColdCustody == nil {
		p.GitLabColdCustody = gitlabColdCustodyProbe
	}
	if p.GrantedScopes == nil {
		p.GrantedScopes = grantedScopesProbe
	}
	if p.WriteTransport == nil {
		p.WriteTransport = writeTransportProbe
	}
	if p.CommitEmail == nil {
		p.CommitEmail = commitEmailProbe
	}
	if p.AppIDFor == nil {
		p.AppIDFor = AppID
	}
	if p.QueuedSiblings == nil {
		p.QueuedSiblings = QueuedSiblings
	}
	if p.SiblingRoots == nil {
		p.SiblingRoots = ConfiguredRoots
	}
	if p.DirExists == nil {
		p.DirExists = dirExistsProbe
	}
	if p.AmbientLogin == nil {
		p.AmbientLogin = ambientLoginProbe
	}
	if p.CredHelperMatchesApp == nil {
		p.CredHelperMatchesApp = credHelperMatchesAppProbe
	}
	return p
}

// --- check 1: token mint, cold ---------------------------------------------

// checkColdMint proves the role can obtain a credential from a FRESH process
// with no inherited token, pem override or App-id override — the exact state a
// long session lands in ~1h after boot when the cached token lapses (#794), and
// the state a worker starts in when the tool is not even on PATH (#567).
//
// The credential's SHAPE is forge-specific, so the check is forge-aware (#655): a
// GitLab-forge repo has no App PEM/apps.env and must NOT be told to provision
// one — it takes the GitLab PAT custody path. An unresolved or GitHub forge takes
// the GitHub App mint path, byte-for-byte the pre-#655 behaviour.
func checkColdMint(p PreflightProbes, role, repo string, forge ForgeKind) (string, Check) {
	if forge == ForgeGitLab {
		return checkGitLabColdCustody(p, role)
	}
	return checkGitHubColdMint(p, role, repo)
}

// checkGitHubColdMint is the #794/#567 GitHub-App cold-mint check: a token mints
// from a fresh, scrubbed process reading <role>-app.pem and apps.env off the
// App-credential search path.
func checkGitHubColdMint(p PreflightProbes, role, repo string) (string, Check) {
	tokenPath, err := p.ColdMint(role, repo)
	if err != nil {
		st := CheckedFailed
		if IsUnverifiable(err) && strings.Contains(err.Error(), "could not run") {
			st = CouldNotCheck
		}
		c := Check{
			Name:   CheckColdMint,
			State:  st,
			Detail: oneLine(err.Error()),
			Remediation: "put the desk-tools bin dir on PATH and point " + EnvConfigHome +
				" at the directory holding <role>-app.pem and apps.env, then re-run preflight",
			Refs: "#794 #567",
		}
		return "", c
	}
	if strings.TrimSpace(tokenPath) == "" {
		return "", unchecked(CheckColdMint, "the mint reported success but named no token file",
			"re-run `desktoken "+role+"` by hand and read its stdout — it must print the token cache PATH", "#794")
	}
	return tokenPath, clean(CheckColdMint, "cold mint ok ("+tokenPath+")", "#794 #567")
}

// checkGitLabColdCustody is the GitLab arm of the cold-mint check (#655). GitLab
// custody is a PROVISIONED PAT, not a minted App token: the role's rotate-on-mint
// credential lives 0600 in gitlab-<role>.token on the App-credential search path,
// and `desktoken --forge gitlab <role>` rotates it against GITLAB_API_BASE. This
// check proves that rotate's preconditions READ-ONLY — it never rotates, because
// rotation invalidates the live PAT and a boot probe must not silently kill a
// working credential (the issue's explicit constraint). Its remediation names the
// GitLab custody path, never App PEMs/apps.env: a GitLab adopter has none, and
// pointing one at PEM files was the exact #655 symptom.
func checkGitLabColdCustody(p PreflightProbes, role string) (string, Check) {
	const refs = "#655 #794"
	tokenPath, err := p.GitLabColdCustody(role)
	if err != nil {
		st := CheckedFailed
		if strings.Contains(err.Error(), "could not stat") || strings.Contains(err.Error(), "could not read") {
			st = CouldNotCheck
		}
		return "", Check{
			Name:   CheckColdMint,
			State:  st,
			Detail: oneLine(err.Error()),
			Remediation: "provision the role's GitLab PAT 0600 as gitlab-" + role +
				".token on the App-credential search path (set " + EnvConfigHome + " to that directory), and " +
				"set GITLAB_API_BASE to your REST v4 base (self-hosted: https://gitlab.example.com/api/v4; " +
				"gitlab.com SaaS: https://gitlab.com/api/v4) — the GitLab custody path needs no GitHub App private key",
			Refs: refs,
		}
	}
	if strings.TrimSpace(tokenPath) == "" {
		return "", unchecked(CheckColdMint, "the GitLab custody check reported success but named no token file",
			"run `desktoken --forge gitlab "+role+"` by hand and read its stdout — it must print the token file PATH", refs)
	}
	return tokenPath, clean(CheckColdMint, "gitlab cold custody ok, rotate-on-mint ready ("+tokenPath+")", refs)
}

// coldMintProbe is the real cold mint: `desktoken <role>` in a fresh process
// with a SCRUBBED environment.
//
// "Cold" is the load-bearing word. Inheriting this process's environment would
// carry <ROLE>_TOKEN / <ROLE>_PEM / <ROLE>_APP_ID / GH_TOKEN forward and the
// probe would pass on an ambient credential that a fresh shell will not have —
// which is precisely the failure #794 describes: everything works until the warm
// cache lapses. Only the home-defining variables, PATH, the config-home knob,
// the proxy/TLS variables the network call needs, and TMPDIR survive.
//
// The probe MINTS (it does not use --ttl): --ttl fails on a cold machine that
// has no cache yet, which is the normal state at boot, so it would report red
// for a perfectly healthy envelope. Minting writes only the 0600 token cache —
// it is the same call the desk makes at boot anyway, and no repo is touched.
func coldMintProbe(role, repo string) (string, error) {
	bin, err := exec.LookPath("desktoken")
	if err != nil {
		return "", Refused("desktoken is not on PATH: " + err.Error())
	}
	args := []string{role}
	if strings.TrimSpace(repo) != "" {
		args = append(args, "--repo", repo)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = scrubbedEnv()
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			msg := oneLine(string(ee.Stderr))
			if msg == "" {
				msg = "desktoken " + role + " exited " + strconv.Itoa(ee.ExitCode())
			}
			return "", Unverifiable("cold mint refused: "+msg, nil)
		}
		// Could not even run the binary (timeout, exec failure): could-not-check.
		return "", Unverifiable("could not run desktoken "+role+": "+oneLine(err.Error()), nil)
	}
	return strings.TrimSpace(lastLine(string(out))), nil
}

// ForgeKindForRepo is the shared forge-kind probe: it wraps resolveForgeKind — the
// ONE resolver in this tree (forgeresolve.go, #659) — so every caller reads the
// forge from the SAME place ForgeFor does (ASSAY_REPO_FORGES, then the origin
// remote host) rather than growing a second, driftable answer. An unresolvable
// forge returns "" (not an error): the caller treats "" as GitHub, the historical
// default, so a GitHub adopter that never configured ASSAY_REPO_FORGES is
// unaffected (#655).
//
// It is the default for the preflight cold-mint / app-scopes checks AND the seam
// deskboot's token-mint step reads to decide the mint's custody path (#676), so the
// two halves of a boot — the preflight probe and the mint it precedes — resolve the
// forge identically. It reads NO custody credential: it answers only WHICH forge,
// never touching a token file, so a probe can never rotate or mint as a side effect.
func ForgeKindForRepo(repo string) ForgeKind {
	res, err := resolveForgeKind(parseForgeSlug(repo))
	if err != nil {
		return ""
	}
	return res.Kind
}

// parseForgeSlug turns an "owner/name" slug into a ForgeRepo for resolveForgeKind.
// An empty or malformed slug yields a zero ForgeRepo, whose "/" key simply misses
// the roster map so resolution falls through to the remote-host step — never a
// panic and never an invented owner.
func parseForgeSlug(repo string) ForgeRepo {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok {
		return ForgeRepo{}
	}
	return ForgeRepo{Owner: owner, Name: name}
}

// gitlabColdCustodyProbe verifies — READ-ONLY, without rotating — that a fresh
// `desktoken --forge gitlab <role>` would have what it needs (#655). It mirrors
// that command's own preconditions (cmd/desktoken/gitlab.go): the role's PAT at
// gitlab-<role>.token on the App-credential search path, a 0600 regular file, a
// non-empty value, and GITLAB_API_BASE set (the rotate refuses before any network
// contact without it). It returns the custody file PATH — never the token value.
//
// It deliberately does NOT rotate. The GitLab mint is rotate-on-mint: it
// invalidates the current PAT and issues a new one. Running that as a boot probe
// would silently invalidate a live credential on every preflight, so this proves
// the rotate's preconditions without performing it — the read-only analogue of
// the GitHub cold mint, which writes only its 0600 cache and touches no repo.
func gitlabColdCustodyProbe(role string) (string, error) {
	name := gitlabTokenFileName(role)
	path, searched, found := FindConfigFile(name)
	if !found {
		return "", fmt.Errorf("gitlab token file not found: no %s on the App-credential search path (searched: %s)",
			name, strings.Join(searched, ", "))
	}
	target, fi, serr := LstatCustody(path, CustodySameDirLink)
	if serr != nil {
		if _, isLink := serr.(*CustodyLinkError); isLink {
			return "", serr
		}
		return "", fmt.Errorf("could not stat gitlab token file at %s: %v", path, serr)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("gitlab custody at %s is not a regular file (mode %s); custody requires a 0600 regular file", target, fi.Mode())
	}
	// The owner-only check and the read both take the target LstatCustody judged, so a
	// followed link's resolved file is what is checked and read on every platform.
	if err := VerifyCustodyOwnerOnly(target, fi); err != nil {
		return "", err
	}
	raw, rerr := os.ReadFile(target)
	if rerr != nil {
		return "", fmt.Errorf("could not read gitlab token file at %s: %v", target, rerr)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return "", fmt.Errorf("the gitlab token file at %s is empty", path)
	}
	if strings.TrimSpace(os.Getenv("GITLAB_API_BASE")) == "" {
		return "", fmt.Errorf("GITLAB_API_BASE is not set — a fresh `desktoken --forge gitlab %s` rotate refuses "+
			"before contacting any host without it", role)
	}
	return path, nil
}

// deriveRepoSlug reads the landing remote's URL as the RAW configured value (go-git's
// RemoteURL, matching `git config --get remote.<name>.url` — no `insteadOf` expansion) and
// extracts owner/name through the single shared parser (ParseRemoteRepo). That parser
// handles every remote shape a desk checkout uses, including an ssh HOST ALIAS and the
// rewritten/hybrid forms an `insteadOf` config bakes onto it (issue 1470) — the alias form
// is the one a naive parser drops, and dropping it silently sends the mint at the minter's
// default owner instead of the one the pass will land on. It returns "" when it cannot
// resolve — an empty slug lets the minter apply its own default, and the cold-mint check
// reports whatever that produces rather than this function inventing a repo.
func deriveRepoSlug(dir, remote string) string {
	repo, err := gitcore.Open(orDot(dir))
	if err != nil {
		return ""
	}
	out, err := repo.RemoteURL(remote)
	if err != nil {
		return ""
	}
	slug, err := RemoteRepoSlug(out)
	if err != nil {
		return ""
	}
	return slug
}

// RepoSlugForDir resolves owner/name from a checkout's `origin` remote, the exported entry to
// deriveRepoSlug's ssh-alias-aware parse. It returns "" when the dir is not a repo, has no origin, or
// the URL does not parse — a could-not-resolve the caller reports as itself, never a guessed repo.
func RepoSlugForDir(dir string) string { return deriveRepoSlug(dir, "origin") }

// scrubbedEnv is the ALLOWLIST a cold probe runs under. An allowlist, not a
// denylist: a new credential env var added elsewhere must not silently start
// warming this probe.
//
// The home-defining variables are load-bearing, not incidental. The child mint
// resolves the roster and the App-credential home through os.UserHomeDir()
// (rosterconfig.go's configHomeFile, appconfig.go's expandHome), and
// os.UserHomeDir() reads a DIFFERENT variable per platform: HOME on unix/plan9,
// %USERPROFILE% on Windows. An allowlist that carried only HOME therefore left
// the Windows child with no home at all — os.UserHomeDir() failed with
// "%userprofile% is not defined", the roster read as absent, and the cold mint
// refused on an envelope that was actually intact (#642). Keeping every
// platform's home variable — plus HOMEDRIVE/HOMEPATH, the pair git-for-Windows
// composes a home from — lets the child reconstruct the SAME home the parent
// resolved, on any OS, without dragging a credential across. On unix the Windows
// names are simply unset and skipped, so this is not a widening of what a unix
// child inherits.
func scrubbedEnv() []string {
	keep := []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH",
		"PATH", EnvConfigHome, "TMPDIR",
		"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR"}
	var env []string
	for _, k := range keep {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// --- check 2: App scopes vs the role's rostered duties ----------------------

// Duty is one thing the role is rostered to DO, bound to the GitHub App
// permission that lets it. The duty list is the mechanism this repo ships; the
// role→App binding is the policy the roster supplies (ASSAY_TRUSTED_BOT_SLUGS).
type Duty struct {
	Permission string
	Level      string
	Why        string
}

// requiredDuties are the three duties every desk role's App must be able to
// perform. They are exactly the three #571 names: a verifier that can open an
// issue but goes silent on PR threads has a scope gap that only shows up at
// comment time, three quarters of the way through a pass.
var requiredDuties = []Duty{
	{Permission: "pull_requests", Level: "write", Why: "PR comment / review (#571)"},
	{Permission: "issues", Level: "write", Why: "file and comment on issues"},
	{Permission: "contents", Level: "write", Why: "land commits (Evidence, status)"},
}

// checkAppScopes compares the installation's GRANTED permissions against the
// role's duties. A missing scope is checked-failed; an unreadable grant is
// could-not-check — reading a permission LISTING as a grant would be the exact
// mistake AGENTS.md forbids in the other direction, so an ABSENT listing is
// certainly not one.
//
// ROLE→APP BINDING. This check is role-aware THROUGH the binding
// with no code of its own: tokenPath is what the role's cold mint produced, and that mint
// resolves the App the role is BOUND to (<ROLE>_APP; appconfig.go AppBinding). So the grant
// read here is already the BOUND App's grant. requiredDuties is one fixed set for every
// role, so when several roles are bound to one App they mint one grant that this check reads
// per role — a shared grant covering the duties passes every bound role. Proven by
// TestMultiRoleSharedGrantPassesEveryBoundRole.
func checkAppScopes(p PreflightProbes, role, tokenPath string, forge ForgeKind) Check {
	const refs = "#571"
	// The grant this check reads is a GitHub App INSTALLATION grant, recorded in a
	// .perms sidecar the GitHub mint writes. GitLab has no such object: a PAT's
	// scopes are set by the group owner at provisioning and are not observable
	// offline from any mint response (there is no per-mint permission sidecar to
	// read). So on a GitLab-forge repo this GitHub-installation-grant check does
	// not APPLY — it is not that the grant could not be read, it is that there is
	// no such grant on this forge. Reporting could-not-check reddened the envelope
	// and a correctly provisioned GitLab fleet could not boot (#671); reporting
	// checked-clean would be a false pass (nothing was verified). It is therefore
	// CheckedNotApplicable: it does not block the boot, it is surfaced on its own
	// (never folded into the checked-clean count), and it carries a GitLab-native
	// human-confirm remediation with NO App/PEM/`--fresh` text — that GitHub
	// remediation is the #655 wrong-forge trap one check further along. The
	// credential ENVELOPE control on GitLab is the cold-mint check's GitLab arm
	// (PAT custody, read-only); the scopes a PAT actually carries are set out of
	// band at the group's Access Tokens page and confirmed there by a human, not
	// observable to this offline check.
	if forge == ForgeGitLab {
		return notApplicable(CheckAppScopes,
			"gitlab credential: this check reads a GitHub App installation grant, which a GitLab PAT does not "+
				"have — a PAT's scopes are set by the group owner at provisioning and are not observable offline "+
				"from a mint grant (GitLab records no per-mint permission sidecar), so the GitHub grant check does "+
				"not apply to a GitLab PAT",
			"confirm at the GitLab group's Access Tokens page (Settings → Access Tokens) that the "+role+" PAT "+
				"carries the scopes its duties need (api / write_repository); this is an out-of-band human check, "+
				"not something the offline preflight can verify",
			"#655, #671")
	}
	if tokenPath == "" {
		return unchecked(CheckAppScopes, "no token was minted, so no grant could be read",
			"fix the "+CheckColdMint+" check first — scopes are read from the mint's recorded grant", refs)
	}
	// After any installation permission change the cached token still carries the OLD grant
	// for the rest of the ~50-min reuse window, and the .perms sidecar this check reads is
	// only rewritten on a FRESH mint — so every remediation below says `--fresh`, not a plain
	// re-mint (which would be a no-op that re-reads the same stale grant).
	granted, err := p.GrantedScopes(role, tokenPath)
	if err != nil {
		return unchecked(CheckAppScopes, oneLine(err.Error())+grantSidecarAge(tokenPath),
			"re-mint FRESH with `desktoken "+role+" --fresh` so the grant sidecar is (re)written next to the "+
				"token cache; after any installation permission change the cached token still carries the old "+
				"grant — re-mint fresh", refs)
	}
	if len(granted) == 0 {
		return unchecked(CheckAppScopes, "the recorded grant is empty"+grantSidecarAge(tokenPath),
			"re-mint FRESH with `desktoken "+role+" --fresh`; if the grant is still empty the installation grants "+
				"nothing and needs its permissions set (after any installation permission change the cached token "+
				"still carries the old grant — re-mint fresh)", refs)
	}
	var missing []string
	for _, d := range requiredDuties {
		if !levelSatisfies(granted[d.Permission], d.Level) {
			missing = append(missing, fmt.Sprintf("%s=%s (want %s, for %s)",
				d.Permission, orNone(granted[d.Permission]), d.Level, d.Why))
		}
	}
	if len(missing) > 0 {
		return failed(CheckAppScopes, "granted scopes do not cover the role's duties: "+strings.Join(missing, "; ")+grantSidecarAge(tokenPath),
			"add the missing permission to the "+role+" App's INSTALLATION (both installs) and accept the "+
				"permission-update prompt, THEN re-mint FRESH with `desktoken "+role+" --fresh` — after any "+
				"installation permission change the cached token still carries the old grant, so a plain re-mint is "+
				"a no-op; this check compares the granted scopes against the role's rostered duty set", refs)
	}
	return clean(CheckAppScopes, fmt.Sprintf("all %d rostered duties covered", len(requiredDuties))+grantSidecarAge(tokenPath), refs)
}

// grantSidecarAge returns a short " (grant recorded <age> ago)" suffix for the app-scopes
// detail line, computed from the .perms sidecar's mtime, or "" if it cannot be read. The age
// is the diagnostic that tells a stale-grant remediation from a fresh one: a grant recorded
// long ago against a since-changed installation is exactly the #571 trap `--fresh` closes.
func grantSidecarAge(tokenPath string) string {
	if tokenPath == "" {
		return ""
	}
	fi, err := os.Stat(tokenPath + ".perms")
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" (grant sidecar recorded %s ago)", roundAge(time.Since(fi.ModTime())))
}

// roundAge renders a duration to whole minutes (or seconds under a minute) for a human
// detail line — the sidecar age only needs coarse resolution to flag a stale grant.
func roundAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// levelSatisfies reports whether a granted permission level covers a required
// one. GitHub's App levels are ordered read < write < admin.
func levelSatisfies(granted, want string) bool {
	rank := map[string]int{"": 0, "read": 1, "write": 2, "admin": 3}
	g, ok := rank[strings.ToLower(strings.TrimSpace(granted))]
	if !ok {
		return false
	}
	w, ok := rank[strings.ToLower(strings.TrimSpace(want))]
	if !ok {
		return false
	}
	return g >= w
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not granted)"
	}
	return s
}

// grantedScopesProbe reads the permission grant desktoken records next to the
// token cache at mint time (`<tokenPath>.perms`).
//
// The grant is only ever visible in the access-token RESPONSE, so the minter is
// the only component that can observe it. Recording it there and reading it here
// keeps the JWT-signing logic in one place instead of duplicating it into this
// package to ask GitHub the same question a second time.
func grantedScopesProbe(_ string, tokenPath string) (map[string]string, error) {
	path := tokenPath + ".perms"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no recorded grant at %s: %w", path, err)
	}
	return parsePermsJSON(string(raw))
}

// parsePermsJSON reads the flat {"key":"value"} permission object desktoken
// writes. It is deliberately a tiny hand parser rather than encoding/json into
// map[string]any so a nested or unexpected shape is an ERROR (could-not-check)
// instead of a silently empty map that would read as "nothing granted".
func parsePermsJSON(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("recorded grant is not a JSON object")
	}
	out := map[string]string{}
	body := strings.TrimSpace(s[1 : len(s)-1])
	if body == "" {
		return out, nil
	}
	for _, pair := range strings.Split(body, ",") {
		k, v, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("recorded grant has a malformed entry %q", strings.TrimSpace(pair))
		}
		k = strings.Trim(strings.TrimSpace(k), `"`)
		v = strings.Trim(strings.TrimSpace(v), `"`)
		if k == "" {
			return nil, fmt.Errorf("recorded grant has an empty permission name")
		}
		out[k] = v
	}
	return out, nil
}

// --- check 3: write transport ----------------------------------------------

// checkWriteTransport probes the role's LANDING path before the pass starts,
// rather than discovering at landing time that no outward write transport is
// permitted (#823 — a pass verified two briefs to a clean PASS and could land
// neither).
//
// The probe is READ-ONLY by construction (see writeTransportProbe) and a
// rejection is a STOP: this function reports it and returns. There is no branch
// that tries a second credential — AGENTS.md, "A scope rejection is a STOP —
// never re-push the change under a different identity."
func checkWriteTransport(p PreflightProbes, l Landing) Check {
	const refs = "#823"
	verdict, detail, err := p.WriteTransport(l)
	if err != nil {
		return unchecked(CheckWriteTransport, oneLine(err.Error()),
			"run the landing probe by hand (`git -C "+orDot(l.Dir)+" push --dry-run "+l.Remote+" HEAD`) and read the transport error", refs)
	}
	switch verdict {
	case ProbePermitted:
		return clean(CheckWriteTransport, "landing transport permitted for "+l.Remote+" "+orCurrent(l.Branch)+
			" ("+oneLine(detail)+")", refs)
	case ProbeRejected:
		return failed(CheckWriteTransport, "landing transport REJECTED for "+l.Remote+" "+orCurrent(l.Branch)+
			": "+oneLine(detail),
			"STOP — do not retry under another identity. Obtain the write permission for THIS identity "+
				"(or run in a permission mode that admits the desk's landing verbs) and re-run preflight", refs)
	default:
		return unchecked(CheckWriteTransport, "probe inconclusive: "+oneLine(detail),
			"ensure a git remote named "+l.Remote+" exists and this worktree is on a branch (a detached HEAD has no landing ref)", refs)
	}
}

// writeTransportProbe is the real, NON-MUTATING write-transport probe:
// `git push --dry-run`.
//
// --dry-run does everything except send the update: it authenticates, contacts
// the remote's receive-pack and evaluates the ref update, then stops. That is
// exactly the read-only "would this land?" question, and it is NOT the
// permission LISTING that AGENTS.md warns is neither grant nor bar — a listing
// says what the docs claim, this says what the transport does.
//
// It runs ONCE. Every failure path below returns; none re-invokes git with a
// different credential.
func writeTransportProbe(l Landing) (ProbeVerdict, string, error) {
	dir := orDot(l.Dir)
	if _, err := exec.LookPath("git"); err != nil {
		return ProbeInconclusive, "git is not on PATH", nil
	}
	probeRepo, err := gitcore.Open(dir)
	if err != nil {
		return ProbeInconclusive, "cannot open " + dir + " as a git repository", nil
	}
	branch := strings.TrimSpace(l.Branch)
	if branch == "" {
		out, err := probeRepo.SymbolicRefShortHEAD()
		if err != nil {
			return ProbeInconclusive, "detached HEAD: no landing branch to probe", nil
		}
		branch = strings.TrimSpace(out)
	}
	if _, err := probeRepo.RemoteURL(l.Remote); err != nil {
		return ProbeInconclusive, "no remote named " + l.Remote, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "push", "--dry-run", "--porcelain",
		l.Remote, "HEAD:refs/heads/"+branch)
	// GIT_TERMINAL_PROMPT=0: a probe must never block on an interactive
	// credential prompt — an unauthenticated transport has to FAIL, loudly.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	text := oneLine(string(out))
	if err == nil {
		return ProbePermitted, text, nil
	}
	if rejectionRe.MatchString(text) {
		return ProbeRejected, text, nil
	}
	return ProbeInconclusive, text, nil
}

// rejectionRe matches the transport answers that mean DENIED, as opposed to
// "could not reach a verdict". Both the git/GitHub wordings and the harness
// permission-classifier wording (#823) are here: from the desk's seat a
// classifier denial and a 403 are the same fact — this identity has no permitted
// write transport to its landing path.
var rejectionRe = regexp.MustCompile(`(?i)(permission denied|denied to |403|401|authentication failed|not authorized|resource not accessible|protected branch|pre-receive hook declined|refusing to allow|blocked by|permission to .* denied)`)

// --- check 4: commit identity ----------------------------------------------

// noreplyRe splits an App noreply commit email into its numeric prefix and its
// login half: "<bot-user-id>+<app-slug>[bot]@users.noreply.github.com".
var noreplyRe = regexp.MustCompile(`^(\d+)\+(.+)@users\.noreply\.github\.com$`)

// checkCommitIdentity proves this worktree would commit under an email whose
// numeric prefix is the role's BOT USER id — not its App id (#638).
//
// The difference is invisible locally and total remotely: with the bot user id,
// `gh api …/commits/<sha>` answers author.login=<slug>[bot], type=Bot; with the
// App id it answers author.login=null, type=null and the commit is attributed to
// nobody. #638 found weeks of Evidence commits in the second state, and the SKILL
// that prescribed it was itself wrong — which is why this is a CHECK and not a
// doc fix.
func checkCommitIdentity(p PreflightProbes, role, dir string) Check {
	const refs = "#638"
	ident, bound := EffectiveConfig().RoleBotIdentity(role)
	if !bound {
		return unchecked(CheckCommitIdentity, "the roster binds no App to role "+role,
			"add "+role+"=<forge>:<slug-or-login>[:<id>] to "+EnvTrustedBotSlugs+" in "+ConfigHomePath(), refs)
	}

	email, err := p.CommitEmail(dir)
	if err != nil {
		return unchecked(CheckCommitIdentity, oneLine(err.Error()), commitIdentityRemedy(ident, dir), refs)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return failed(CheckCommitIdentity, "no commit author email is configured", commitIdentityRemedy(ident, dir), refs)
	}

	// The expected commit address is derived from the ENTRY's forge; neither forge
	// falls back to the other's shape. GitHub keeps the #638 exact-match logic; GitLab
	// matches the service-account noreply SHAPE (its group id / suffix are not derivable
	// from the roster — forgeidentity.go).
	if ident.Forge == ForgeGitLab {
		return checkGitLabCommitIdentity(ident, email, dir, refs)
	}
	return checkGitHubCommitIdentity(p, ident, role, email, dir, refs)
}

// checkGitHubCommitIdentity is the #638 commit-identity logic: the email must carry the
// bot USER id, not the App id (which lands the commit account-UNLINKED), and must name
// the role's own bound App.
func checkGitHubCommitIdentity(p PreflightProbes, ident BotIdentity, role, email, dir, refs string) Check {
	spec := ident.CommitEmailSpec()
	if !spec.Derivable {
		return unchecked(CheckCommitIdentity, "the roster pins no BOT USER id for "+ident.Slug,
			"pin it: "+EnvTrustedBotSlugs+" entry "+role+"=github:"+ident.Slug+":<bot-user-id> (the bot USER id, from `gh api /users/"+ident.Slug+"[bot]`)", refs)
	}
	want := spec.Exact
	if strings.EqualFold(email, want) {
		return clean(CheckCommitIdentity, "commit email carries the bot USER id ("+want+")", refs)
	}
	m := noreplyRe.FindStringSubmatch(email)
	if m == nil {
		return failed(CheckCommitIdentity, "commit email "+email+" is not the App noreply form",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	prefix, login := m[1], m[2]
	if appID, aerr := p.AppIDFor(role); aerr == nil && strings.TrimSpace(appID) == prefix {
		return failed(CheckCommitIdentity,
			"commit email prefix "+prefix+" is the APP id, not the bot USER id — commits land account-UNLINKED (author.login=null)",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	if !strings.EqualFold(login, ident.Slug+"[bot]") {
		return failed(CheckCommitIdentity, "commit email names "+login+", but role "+role+" is bound to "+ident.Slug+"[bot]",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	return failed(CheckCommitIdentity, "commit email prefix "+prefix+" is neither the bot USER id ("+
		strconv.FormatInt(ident.ID, 10)+") nor a recognised id",
		"git -C "+orDot(dir)+" config user.email "+want, refs)
}

// checkGitLabCommitIdentity validates a GitLab worktree's commit author. On GitLab the
// desk runs TWO distinct identities (#643): the SESSION / implementer identity (a real
// GitLab user, e.g. `qa-bot`) authors the commits under an ordinary user address, while
// the role SERVICE ACCOUNT — the analog of the GitHub role App — is used only for minted
// API writes. So this check accepts EITHER:
//
//   - a commit email that is an EXPLICITLY TRUSTED session address
//     (ASSAY_GITLAB_SESSION_EMAILS — the two-identity path), or
//   - the role service-account noreply SHAPE (the commit-as-SA path). The group id and
//     per-account suffix are not in the roster, so the shape is the tightest available
//     check for that form.
//
// A GitHub noreply address for a GitLab entry is a hard failure (the cross-forge case),
// never a fall-through that a skipped check would let pass; and an email that is neither
// a trusted session address nor the service-account shape still FAILS. The session
// allowlist is the ONLY widening here, it is EXACT-MATCH from the trusted roster, and it
// is never consulted on a GitHub identity (the #638 bot-USER-id guarantee is untouched).
func checkGitLabCommitIdentity(ident BotIdentity, email, dir, refs string) Check {
	if GitLabSessionEmailAllowed(email) {
		return clean(CheckCommitIdentity, "commit email "+email+" is an explicitly trusted GitLab session / "+
			"implementer address ("+EnvGitLabSessionEmails+"); the role service account ("+ident.Slug+
			") is the API-write identity, not the commit author", refs)
	}
	if ident.CommitEmailSpec().Accepts(email) {
		return clean(CheckCommitIdentity, "commit email is the GitLab service-account noreply form ("+email+
			"); the group id and per-account suffix are not derivable from the roster, so the shape is the "+
			"tightest available check", refs)
	}
	if noreplyRe.MatchString(email) {
		return failed(CheckCommitIdentity,
			"commit email "+email+" is the GitHub noreply form, but role's identity is a GitLab service "+
				"account — a GitHub-shaped address for a GitLab entry lands the commit under no GitLab identity",
			commitIdentityRemedy(ident, dir), refs)
	}
	return failed(CheckCommitIdentity,
		"commit email "+email+" is neither an explicitly trusted GitLab session / implementer address "+
			"("+EnvGitLabSessionEmails+") nor the role service-account noreply form "+
			"(service_account_group_<group-id>_<suffix>@noreply.<host>)",
		commitIdentityRemedy(ident, dir), refs)
}

// commitIdentityRemedy renders the forge-appropriate fix for a missing or wrong commit
// author email. GitHub can name the exact address to set; GitLab cannot (the roster does
// not carry the group id / suffix), so it points at the provisioned service-account form.
func commitIdentityRemedy(ident BotIdentity, dir string) string {
	spec := ident.CommitEmailSpec()
	if spec.Forge == ForgeGitHub && spec.Derivable {
		return "git -C " + orDot(dir) + " config user.email " + spec.Exact
	}
	if spec.Forge == ForgeGitLab {
		return "commit as the session / implementer identity — set this worktree's user.email to a GitLab " +
			"user address listed in " + EnvGitLabSessionEmails + " — OR, to commit AS the service account, " +
			"set it to the " + ident.Slug + " GitLab service-account noreply address " +
			"(service_account_group_<group-id>_<suffix>@noreply.<host>) provisioned for it"
	}
	return "pin the bot USER id in " + EnvTrustedBotSlugs + " for " + ident.Slug +
		", then set this worktree's user.email to the resulting noreply address"
}

// commitEmailProbe reads the effective commit author email: GIT_AUTHOR_EMAIL
// wins (it is what a `git -c`-free commit in this process would actually use),
// then the worktree's git config.
func commitEmailProbe(dir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GIT_AUTHOR_EMAIL")); v != "" {
		return v, nil
	}
	out, err := gitOut(orDot(dir), "config", "--get", "user.email")
	if err != nil {
		return "", fmt.Errorf("no commit author email: git config user.email is unset in %s", orDot(dir))
	}
	return strings.TrimSpace(out), nil
}

// --- check 5: sibling checkouts --------------------------------------------

// checkSiblings proves the sibling checkouts the QUEUED briefs declare can be
// found, before a pass claims a brief whose rows cannot run (#679 — two rows
// deferred mid-verify for want of a co-located sibling checkout).
//
// Two properties #661 added, after a flat-layout assumption bricked a whole
// cell's boot:
//
//   - RESOLUTION IS NOT FLAT. A declared `../<repo>` is resolved through the
//     CONFIGURED roots (DESK_ROOTS / the topology map) first, so a desk whose
//     checkouts do not sit in a `../<repo>` layout — a pod at
//     /workspace/<org>/<repo>, say — still locates the sibling. The flat
//     `<root>/../<repo>` join is only the fallback when no configured root's
//     repo name matches.
//   - PRESENCE IS SCOPED TO THE CLAIM. At boot (claimedBrief == "") nothing is
//     claimed, so an absent sibling is a NOTICE, not a failure — an unclaimed
//     brief's missing cross-repo checkout must not block the boot of every loop
//     in the cell. Only when a brief is being CLAIMED does its own absent
//     sibling become a hard failure: that brief's rows genuinely cannot run.
func checkSiblings(p PreflightProbes, root, claimedBrief string) Check {
	const refs = "#679 #661"
	reqs, err := p.QueuedSiblings(root)
	if err != nil {
		return unchecked(CheckSiblings, oneLine(err.Error()),
			"run preflight from a repo root that has docs/streams/ (or pass --root), so the queue can be read", refs)
	}
	if len(reqs) == 0 {
		return clean(CheckSiblings, "no queued brief declares an out-of-repo checkout", refs)
	}
	// Resolve declared ../<repo> siblings through the configured roots so a
	// non-flat checkout layout still finds them (#661). A roots-config error is
	// not this check's to raise: fall back to the flat per-sibling join.
	roots, rootsErr := p.SiblingRoots()
	if rootsErr != nil {
		roots = nil
	}
	var missing, notices []string
	for _, r := range reqs {
		abs := resolveSiblingPath(root, r.Rel, roots)
		ok, derr := p.DirExists(abs)
		if derr != nil {
			return unchecked(CheckSiblings, "cannot stat "+abs+" (declared by "+r.Brief+"): "+oneLine(derr.Error()),
				"make "+abs+" readable, or correct the out-of-repo declaration in "+r.Brief, refs)
		}
		if ok {
			continue
		}
		entry := r.Rel + " (declared by " + r.Brief + ")"
		if claimedBrief != "" && briefMatchesClaim(r.Brief, claimedBrief) {
			missing = append(missing, entry)
		} else {
			notices = append(notices, entry)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return failed(CheckSiblings, "sibling checkout(s) the CLAIMED brief declares are absent: "+strings.Join(missing, "; "),
			"clone or `git worktree add` the missing checkout(s) — resolve their location via "+RootsEnv+
				" if your layout is not ../<repo> — before claiming that brief; a row that cannot run must not be claimed", refs)
	}
	present := len(reqs) - len(notices)
	c := clean(CheckSiblings, fmt.Sprintf("%d declared sibling checkout(s) resolved", len(reqs)), refs)
	if len(notices) > 0 {
		sort.Strings(notices)
		c.Detail = fmt.Sprintf("%d of %d declared sibling checkout(s) present", present, len(reqs))
		c.Notice = "unclaimed brief(s) declare an out-of-repo checkout not present here: " + strings.Join(notices, "; ") +
			" — not a boot failure; needed only when that brief is claimed (resolve its location via " + RootsEnv + ")"
	}
	return c
}

// repoName returns the name segment of an owner/name repo slug.
func repoName(slug string) string {
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		return slug[i+1:]
	}
	return slug
}

// resolveSiblingPath turns a brief's declared ../<repo> sibling into the
// directory the check should stat. It prefers the CONFIGURED checkout for the
// repo whose name matches <repo> (DESK_ROOTS / the topology map), so a desk
// whose checkouts do not sit in a flat ../<repo> layout still resolves the
// sibling (#661). Only when no configured root's repo name matches does it fall
// back to the historical flat join of <root>/../<repo>.
func resolveSiblingPath(root, rel string, roots []RootConfig) string {
	name := strings.TrimPrefix(rel, "../")
	// The declared head is ../<repo>; guard against anything trailing it.
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name = name[:i]
	}
	for _, r := range roots {
		if repoName(r.Repo) == name {
			if abs, err := filepath.Abs(r.Path); err == nil {
				return abs
			}
			return r.Path
		}
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(root, rel)
}

// briefMatchesClaim reports whether the queued brief that declared a sibling is
// the one the pass is CLAIMING. The claim id is matched as a substring of the
// brief's stream-relative path, so a caller may name the brief by its number
// ("43"), its file name ("brief-43-x.md"), or its full path.
func briefMatchesClaim(briefPath, claim string) bool {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return false
	}
	return strings.Contains(briefPath, claim)
}

// siblingRe extracts a sibling checkout root from an out-of-repo declaration:
// "../tracker/docs/x.md" → "../tracker". Only the ../<name> head is taken — the check is
// "is the checkout there", not "is that one file there".
var siblingRe = regexp.MustCompile(`\.\./([A-Za-z0-9][A-Za-z0-9._-]*)`)

// awaitingRe matches a stream-README brief row whose Status is in the Awaiting
// queue. Rows outside the queue are NOT scanned: a sibling declared by a brief
// nobody is about to claim is not an envelope failure, and treating it as one
// would make preflight red for work that is not queued.
var awaitingRe = regexp.MustCompile(`(?i)\|\s*(implemented|verified)\s*\|`)

// QueuedSiblings reads the sibling checkouts declared by the briefs currently in
// the Awaiting queue under <root>/docs/streams/.
//
// It reads ONLY the brief schema's `out-of-repo files:` declaration — not the
// whole body. A brief that mentions ../<repo> inside an Evidence table or a
// Verify command has not DECLARED a dependency, and counting those would red the
// preflight on prose.
func QueuedSiblings(root string) ([]SiblingReq, error) {
	streams := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streams)
	if err != nil {
		return nil, fmt.Errorf("cannot read the brief queue at %s: %w", streams, err)
	}
	seen := map[string]bool{}
	var out []SiblingReq
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		readme, rerr := os.ReadFile(filepath.Join(streams, e.Name(), "README.md"))
		if rerr != nil {
			continue // a stream dir without a README carries no queue
		}
		queued := map[string]bool{}
		for _, line := range strings.Split(string(readme), "\n") {
			if !awaitingRe.MatchString(line) {
				continue
			}
			cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
			if len(cells) == 0 {
				continue
			}
			if num := strings.TrimSpace(cells[0]); num != "" {
				queued[num] = true
			}
		}
		for num := range queued {
			matches, _ := filepath.Glob(filepath.Join(streams, e.Name(), "brief-"+num+"-*.md"))
			for _, m := range matches {
				raw, berr := os.ReadFile(m)
				if berr != nil {
					continue
				}
				rel := m
				if r, rerr := filepath.Rel(root, m); rerr == nil && r != "" {
					rel = r
				}
				for _, sib := range declaredSiblings(string(raw)) {
					key := rel + "\x00" + sib
					if seen[key] {
						continue
					}
					seen[key] = true
					out = append(out, SiblingReq{Brief: rel, Rel: sib})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Brief != out[j].Brief {
			return out[i].Brief < out[j].Brief
		}
		return out[i].Rel < out[j].Rel
	})
	return out, nil
}

// declaredSiblings pulls the ../<name> roots out of a brief's `out-of-repo
// files:` declaration. "none" (the overwhelmingly common value) yields nothing.
func declaredSiblings(brief string) []string {
	var out []string
	seen := map[string]bool{}
	lines := strings.Split(brief, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "out-of-repo files:") {
			continue
		}
		block := []string{line}
		// A list-form declaration continues onto following "- " bullet lines.
		for _, next := range lines[i+1:] {
			t := strings.TrimSpace(next)
			if !strings.HasPrefix(t, "- ") {
				break
			}
			block = append(block, next)
		}
		joined := strings.Join(block, "\n")
		if strings.Contains(strings.ToLower(joined), "none") && !siblingRe.MatchString(joined) {
			continue
		}
		for _, m := range siblingRe.FindAllStringSubmatch(joined, -1) {
			sib := "../" + m[1]
			if seen[sib] {
				continue
			}
			seen[sib] = true
			out = append(out, sib)
		}
	}
	sort.Strings(out)
	return out
}

func dirExistsProbe(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.IsDir(), nil
}

// --- check 6: ambient identity ---------------------------------------------

// checkAmbientIdentity proves the AMBIENT credential a tool fall-through would
// reach is safe. It is the two layers ABOVE the per-tool refusal the write-verbs
// migration delivered: the tools no longer fall through to an ambient credential,
// but a wrong ambient login and a mis-pointed credential helper are still latent
// in the envelope, and all three symptoms can fire together — an issue filed
// under an ambient login while the tool stamps its raised-by role, a push refused
// on the credential helper though the App token minted, and a claims read failing
// on the same helper. This check closes the two halves the per-tool refusal
// cannot see from inside one tool:
//
//   - IDENTITY. Desk writes go out under minted App tokens; no desk needs an
//     ambient human login, and the only state with no identity-fall-through risk
//     is the one with NO usable ambient human identity. So:
//     no usable ambient human identity (`gh` not logged in with no stored
//     credential `gh auth token` can still read, a credential that answers 401
//     on /user, or an App/integration token's 403) passes this half;
//     "not logged in" with a stored credential still readable (an OS-keyring
//     login behind an empty config dir) is could-not-check — a wrapper that
//     resolves `gh auth token` hands it back, and whose it is cannot be told
//     without using it;
//     a HUMAN login — the blessing login or any other — is a non-blocking
//     WARNING (a Notice on the result) naming the login, because a fall-through
//     would act as that human, the blessing login included, which is the case
//     where a stray write lands under the maintainer's name;
//     a bot/App slug is checked-failed — a fall-through under a bot identity
//     misattributes and dodges the App-token mint;
//     a probe that could not look (gh absent, exec failure, timeout, any other
//     answer) is could-not-check.
//     Before #1798 the safe state was unreachable: the real probe reported
//     "not logged in" and the App-token 403 as errors, the check read both as
//     could-not-check, and the only green ambient state was a logged-in
//     blessing human.
//   - TRANSPORT. Every credential source git consults for the landing remote's
//     push URL — each helper in the ordered chain git builds across all config
//     scopes, and any embedded URL credential or Authorization extraHeader it
//     uses ahead of them — must be the SAME minted App token the pass lands
//     under, so the write-transport probe (check 3) and a real push from this
//     envelope present the same credential. A competing source that git would
//     consult first is what the probe-green/push-red split is made of, and it
//     reads red here rather than hiding behind the last-configured helper. This
//     half is independent of the identity half and runs whatever that half found.
//
// It is forge-aware. The ambient-login half reads a GitHub `gh` identity, which a
// GitLab-forge repo does not use, so on GitLab the check is not-applicable (it
// does not redden a correctly provisioned GitLab envelope) — the same shape the
// app-scopes check takes for the GitHub-only installation grant.
func checkAmbientIdentity(p PreflightProbes, l Landing, tokenPath string, forge ForgeKind) Check {
	const refs = "#1527 #1798"
	if forge == ForgeGitLab {
		return notApplicable(CheckAmbientID,
			"gitlab credential: the ambient-identity check reads a GitHub `gh` login and matches the origin "+
				"credential helper against a minted GitHub App token, neither of which a GitLab desk uses — the "+
				"GitLab credential envelope is covered by the cold-mint custody check (PAT custody, read-only)",
			"confirm out of band that the interactive git credential for this host is the operator's own, not a "+
				"service account, and that the origin credential helper reads the provisioned GitLab PAT",
			refs)
	}

	cfg := EffectiveConfig()

	// Identity half. identity is the clean-detail phrase; notice is the
	// non-blocking warning a human ambient login carries on EVERY outcome below,
	// so a transport failure never hides it.
	var identity, notice string
	login, err := p.AmbientLogin()
	switch {
	case errors.Is(err, ErrNoAmbientIdentity):
		identity = "no usable ambient human identity (" + oneLine(strings.TrimPrefix(
			strings.TrimPrefix(err.Error(), ErrNoAmbientIdentity.Error()), ": ")) + ")"
	case errors.Is(err, ErrStoredAmbientCredential):
		return unchecked(CheckAmbientID, oneLine(err.Error()),
			ambientClearRemedy+"; then re-run preflight", refs)
	case err != nil:
		return unchecked(CheckAmbientID, "could not read the ambient gh identity: "+oneLine(err.Error()),
			"run `gh api user` by hand in this same shell and read its answer — a not-logged-in gh and an "+
				"HTTP 401/403 already count as no ambient human identity, so what blocked the read is something "+
				"else (gh missing from PATH, a timeout, a server or rate-limit error); fix that and re-run preflight", refs)
	default:
		got := strings.ToLower(strings.TrimSpace(login))
		switch {
		case got == "":
			identity = "no ambient gh identity is set (nothing to fall through to)"
		case isAmbientBot(cfg, got):
			botRemedy := ambientClearRemedy + "; if the bot credential is the interactive gh login rather than " +
				"a token variable, log that bot account out of gh"
			botDetail := "the ambient gh login is " + got + ", a bot/App slug — a tool fall-through would post " +
				"as a bot and dodge the App-token mint"
			return failed(CheckAmbientID, botDetail, botRemedy, refs)
		default:
			identity = "ambient gh login is a human (" + got + ", warned)"
			whose := "that human"
			if bless := strings.ToLower(strings.TrimSpace(cfg.Bless.Login)); bless != "" && got == bless {
				whose = "that human — the blessing login, so a stray write would land under the maintainer's name"
			}
			notice = "WARNING: the ambient gh login is " + got + ", a human — a tool that falls through to the " +
				"ambient credential would act as " + whose + ". Desk writes use minted App tokens and need no " +
				"ambient login: " + ambientClearRemedy
		}
	}

	// Transport half: every credential source git consults for the landing remote's
	// push URL must be the minted App token, so the probe and a real push present
	// the same credential.
	c := checkAmbientTransport(p, l, tokenPath, identity, refs)
	c.Notice = notice
	return c
}

// ambientClearRemedy is the ONE remediation for an ambient credential a desk shell
// should not carry. It leads with clearing the credential, and it never
// recommends logging a human in.
//
// An empty GH_CONFIG_DIR alone is NOT enough on a machine whose gh login lives in
// the OS keyring: `gh api user` reports "not logged in" from an empty config
// directory, but `gh auth token` still returns the keyring login — and cellctl's
// per-verb shim resolves `gh auth token` when GH_TOKEN and GH_ENTERPRISE_TOKEN
// are unset and its `gh` wrapper hands the result to every `gh` child as
// GH_TOKEN (#1145, #1631), so the human credential the operator just cleared
// comes back for exactly the `gh` a fall-through would run. Setting GH_TOKEN to
// the role's minted App token closes both paths: it outranks every stored gh
// credential, the shim and its wrapper leave an explicit GH_TOKEN alone, and
// /user answers 403 for it, which this check reads as no ambient human identity.
const ambientClearRemedy = "clear the ambient gh credential for desk shells — unset GITHUB_TOKEN/GH_ENTERPRISE_TOKEN " +
	"and export GH_TOKEN as the role's minted App token (the cache file `desktoken <role>` prints), which outranks " +
	"every stored gh login; an empty GH_CONFIG_DIR alone is not enough, because `gh auth token` still returns an " +
	"OS-keyring login from an empty config dir, and a verb shim whose gh wrapper hands that to gh as GH_TOKEN brings the human back. " +
	"Never log a human in, or switch the operator's own gh account, to clear this"

// checkAmbientTransport is the transport half of checkAmbientIdentity: every
// credential source git consults for the landing remote's push URL must be the
// minted App token. identity is the identity half's clean phrase, carried into
// the checked-clean detail.
func checkAmbientTransport(p PreflightProbes, l Landing, tokenPath, identity, refs string) Check {
	if strings.TrimSpace(tokenPath) == "" {
		return unchecked(CheckAmbientID,
			"no App token was minted, so the credential helper cannot be matched against it",
			"fix the "+CheckColdMint+" check first — the credential-helper match reads the minted token path", refs)
	}
	ok, detail, herr := p.CredHelperMatchesApp(l, tokenPath)
	if herr != nil {
		return unchecked(CheckAmbientID, "credential-helper resolution: "+oneLine(herr.Error()),
			"list the helper chain by hand (`git -C "+orDot(l.Dir)+" config --show-origin --get-regexp "+
				"'^credential\\.'` against every URL `git -C "+orDot(l.Dir)+" remote get-url --push --all "+l.Remote+
				"` prints) and confirm every applicable helper is the App token helper and no netrc entry answers "+
				"for the host", refs)
	}
	if !ok {
		return failed(CheckAmbientID,
			"the credential helper chain (and any credential git presents ahead of it) for the "+l.Remote+
				" push URL is not solely the minted App token ("+oneLine(detail)+
				") — the write-transport probe and a real push could disagree, the probe-green/push-red split",
			"reset the helper chain for the "+l.Remote+" URL with an empty credential.helper entry, then add only the "+
				"desk's App-token helper after it; remove any embedded URL credential, Authorization extraHeader or netrc "+
				"entry for it, and check every push URL the remote lists", refs)
	}
	return clean(CheckAmbientID, identity+" and every credential source git consults for the "+l.Remote+
		" push URL is the minted App token", refs)
}

// isAmbientBot reports whether an ambient login renders as a bot/App account or
// is a configured trusted-bot slug. Either is disqualifying: a fall-through under
// a bot identity is exactly what the credential fence exists to stop. It reads
// the SHAPE (looksLikeBot — the same predicate the blessing gate uses) and the
// roster's own bot set, so a slug named without the `[bot]` suffix is caught too.
func isAmbientBot(cfg Config, login string) bool {
	if looksLikeBot(login) {
		return true
	}
	slug := strings.TrimSuffix(login, "[bot]")
	if _, ok := cfg.Bots[slug]; ok {
		return true
	}
	if _, ok := cfg.BotIdents[slug]; ok {
		return true
	}
	return false
}

// ErrNoAmbientIdentity is what ambientLoginProbe wraps when `gh` answered, and
// the answer is that there is NO usable ambient human identity: `gh` is not
// logged in, or the ambient credential answers 401/403 on /user (an
// App/integration token is not a user). It is the SAFE state for the identity
// half — nothing to fall through to as a person — and must never be read as
// could-not-check: doing so is exactly what made the safe state unreachable
// (#1798).
var ErrNoAmbientIdentity = errors.New("no usable ambient human identity")

// ErrStoredAmbientCredential is what ambientLoginProbe wraps when `gh api user`
// answered "not logged in" but `gh auth token` still returns a stored credential.
// That is the shape an empty GH_CONFIG_DIR gives on a machine whose gh login
// lives in the OS keyring: the API call finds no config, the keyring credential
// is one local call away, and a wrapper that resolves `gh auth token` into
// GH_TOKEN (cellctl's per-verb shim does) hands it straight back to the next
// `gh`. It is NOT the no-identity state, and the check cannot tell whose
// credential it is without using it, so it reads as could-not-check. The
// credential itself is never kept or echoed.
var ErrStoredAmbientCredential = errors.New("a stored gh credential is still reachable")

// ambientProbeTimeout bounds the `gh api user` call. A var only so the timeout
// path is testable without a 15-second test.
var ambientProbeTimeout = 15 * time.Second

// ghExitAuthRequired is gh's documented exit status for "authentication required"
// (not logged in, no token).
const ghExitAuthRequired = 4

// ambientLoginProbe reads the login the ambient `gh` credential would act as.
//
// It runs `gh api user` under the AMBIENT environment on purpose — the opposite
// of the cold mint's scrubbed env — because the whole question is "what identity
// would a tool fall-through silently use". Its outcomes:
//
//   - a login on stdout → that login;
//   - `gh` answered, and the answer is "no usable human identity" (see
//     classifyNoAmbientIdentity) → an error wrapping ErrNoAmbientIdentity —
//     except that a "not logged in" answer is only taken once `gh auth token`
//     confirms no stored credential is readable either (storedAmbientCredential);
//     one that is → an error wrapping ErrStoredAmbientCredential;
//   - gh absent, gh could not be started, the call timed out, or any other
//     answer → a plain error (could-not-check).
func ambientLoginProbe() (string, error) {
	bin, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh is not on PATH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ambientProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "api", "user", "-q", ".login")
	// A child gh leaves behind (a pager, a helper) can hold stdout open past the
	// kill; WaitDelay bounds that wait so the timeout is the real bound.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("gh api user did not answer within %s", ambientProbeTimeout)
		}
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			stderr := oneLine(string(ee.Stderr))
			if reason, ok := classifyNoAmbientIdentity(ee.ExitCode(), stderr); ok {
				if reason == ghNotLoggedInReason {
					// The stored-credential look is launched HERE, from the one
					// function the forge-CLI ban's register already names for the
					// ambient `gh` launch, not from a helper that would be a second
					// exec site; storedAmbientCredential only judges the result.
					tctx, tcancel := context.WithTimeout(context.Background(), ambientProbeTimeout)
					defer tcancel()
					tcmd := exec.CommandContext(tctx, bin, "auth", "token")
					tcmd.WaitDelay = time.Second
					tout, terr := tcmd.Output()
					if err := storedAmbientCredential(tout, terr, tctx.Err()); err != nil {
						return "", err
					}
				}
				return "", fmt.Errorf("%w: %s", ErrNoAmbientIdentity, reason)
			}
			return "", fmt.Errorf("gh api user failed: %s", stderr)
		}
		return "", fmt.Errorf("could not run gh api user: %v", oneLine(err.Error()))
	}
	return strings.TrimSpace(string(out)), nil
}

// classifyNoAmbientIdentity reads a FAILED `gh api user` (exit status + stderr)
// and reports whether the failure is gh's answer "there is no usable human
// identity here", with the reason. It is deliberately narrow: only the three
// answers that name the credential's shape count, and everything else — a
// server error, a network failure, a RATE-LIMIT 403 (a human token can be
// rate-limited) — is not an answer about identity and stays could-not-check.
func classifyNoAmbientIdentity(exitCode int, stderr string) (string, bool) {
	low := strings.ToLower(stderr)
	switch {
	case strings.Contains(low, "rate limit"):
		return "", false
	case strings.Contains(low, "(http 401)"):
		return "the ambient credential authenticates as no one: HTTP 401 on /user", true
	case strings.Contains(low, "(http 403)"):
		return "the ambient credential is not a user: HTTP 403 on /user, an App/integration token", true
	case exitCode == ghExitAuthRequired || strings.Contains(low, "gh auth login"):
		return ghNotLoggedInReason, true
	}
	return "", false
}

// ghNotLoggedInReason is classifyNoAmbientIdentity's reason for the "not logged
// in" answer — the one answer that is only true of the API call's config view,
// not of every credential gh can still read locally.
const ghNotLoggedInReason = "gh is not logged in"

// storedAmbientCredential judges the `gh auth token` call (local, no network)
// ambientLoginProbe makes after `gh api user` said "not logged in": out and
// runErr are that call's result, ctxErr its timeout context's error. It returns
// nil only when the call shows no stored credential: a non-zero exit, or an
// empty answer. A non-empty answer is an error wrapping
// ErrStoredAmbientCredential; a call that could not run or did not answer in
// time is a plain error (could-not-check). The answer is only tested for being
// empty — it is never returned, logged, or kept.
func storedAmbientCredential(out []byte, runErr, ctxErr error) error {
	err := runErr
	if ctxErr != nil {
		return fmt.Errorf("gh reports not logged in, and gh auth token did not answer within %s, so a stored "+
			"credential could not be ruled out", ambientProbeTimeout)
	}
	if err != nil {
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			return nil
		}
		return fmt.Errorf("gh reports not logged in, and gh auth token could not run: %v", oneLine(err.Error()))
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil
	}
	return fmt.Errorf("%w: gh api user reports not logged in, but gh auth token still returns a stored "+
		"credential (an OS-keyring login read past an empty config dir, say) that a wrapper resolving gh auth "+
		"token into GH_TOKEN hands back to the next gh", ErrStoredAmbientCredential)
}

// credHelperMatchesAppProbe reports whether EVERY credential source git consults for the
// landing remote's PUSH URL (the URL `git push` — and the write-transport probe's
// `push --dry-run` — authenticate against) is the minted App token. The judgement is
// credTransportMatchesApp (credhelperchain.go): the ordered helper chain across every config
// scope, not the single last value --get-urlmatch returns. It is READ-ONLY: it never runs a
// helper and never contacts the remote — a probe that authenticated would be the mutating
// side effect this check exists to keep out of a boot.
func credHelperMatchesAppProbe(l Landing, appTokenPath string) (bool, string, error) {
	dir := orDot(l.Dir)
	if _, err := exec.LookPath("git"); err != nil {
		return false, "", fmt.Errorf("git is not on PATH")
	}
	remote := l.Remote
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	// --all: `git push` pushes to EVERY push URL the remote has (every pushurl, or every url
	// when there is no pushurl), so every one of them is judged, and one failing fails the
	// check. Judging only the first would read green while a later URL authenticates as
	// something else.
	out, err := gitOut(dir, "remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return false, "", fmt.Errorf("no remote named %s in %s", remote, dir)
	}
	var urls []string
	for _, line := range strings.Split(out, "\n") {
		if u := strings.TrimSpace(line); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return false, "", fmt.Errorf("the %s remote in %s lists no push URL", remote, dir)
	}
	var cantCheck error
	details := make([]string, 0, len(urls))
	for i, u := range urls {
		ok, detail, cerr := credTransportMatchesApp(dir, u, appTokenPath)
		prefix := ""
		if len(urls) > 1 {
			prefix = fmt.Sprintf("push URL %d of %d: ", i+1, len(urls))
		}
		switch {
		case cerr != nil:
			if cantCheck == nil {
				cantCheck = fmt.Errorf("%s%w", prefix, cerr)
			}
		case !ok:
			// A red verdict outranks a could-not-check on another URL: it is a finding.
			return false, prefix + detail, nil
		default:
			details = append(details, detail)
		}
	}
	if cantCheck != nil {
		return false, "", cantCheck
	}
	if len(urls) == 1 {
		return true, details[0], nil
	}
	return true, fmt.Sprintf("all %d push URLs pass: %s", len(urls), strings.Join(details, "; ")), nil
}

// --- small shared helpers ---------------------------------------------------

func gitOut(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	return string(out), err
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func orDot(s string) string {
	if strings.TrimSpace(s) == "" {
		return "."
	}
	return s
}

func orCurrent(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(current branch)"
	}
	return s
}
