package main

// gitlab.go — the desktoken CLI's GitLab arm: forge selection and the path-printing adapter
// over the provider's GitLab custody operation (internal/credential, gitlab.go).
//
// Rotate-on-mint, the expiry backstop, the per-role rotation lock, the write-through custody
// layout, the post-rotation self-check and every refusal message live in the provider, which
// documents them; this file decides WHEN the GitLab arm is taken, resolves its API base from
// the environment, and prints the custody PATH the provider returns — never the value.
//
// ROTATION IS EXPLICIT. `desktoken <role> --forge gitlab` rotates; `--no-rotate` is the
// read-only custody lookup, which makes no network contact and never rotates. The provider
// enforces both, so a caller that reaches it any other way gets the same answer.

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabAPIBase returns the explicitly-configured GitLab REST v4 base URL and whether it was
// set. GitLab Enterprise is self-hosted, so there is no universal host the way GitHub has
// api.github.com; the deployment supplies its own via GITLAB_API_BASE (e.g.
// https://gitlab.example.com/api/v4; gitlab.com's SaaS base is https://gitlab.com/api/v4).
//
// There is deliberately NO default. Rotation transmits the role's live PAT in a PRIVATE-TOKEN
// header, so silently falling back to gitlab.com would send a self-hosted deployment's
// credential to a public SaaS endpoint the moment it forgot to configure its host. An unset
// base reaches the provider empty, and the provider refuses before first contact.
//
// Read at call time (not a package-level var) so a test — or a shell — that sets the env var
// after process start still takes effect.
func gitlabAPIBase() (string, bool) {
	v := strings.TrimSpace(os.Getenv("GITLAB_API_BASE"))
	return v, v != ""
}

// gitlabRotateLockPath is the per-role serialisation file that guards one custody file's
// read→rotate→write-verify sequence (the provider's credential.GitLabRotateLockPath). It is
// deliberately NOT the custody file: the write lands by rename, which would orphan a lock held
// on the old inode.
func gitlabRotateLockPath(tokenPath string) string { return credential.GitLabRotateLockPath(tokenPath) }

// gitlabRotateLockWait bounds how long a mint waits for another mint's rotation to finish. A
// rotation is one HTTP round trip plus a small local write, so a wait beyond this is a hung
// or very slow rotation rather than ordinary queueing. It is the same order as the outward
// write lock the posting verbs hold (60s), for the same reason: long enough that a normal
// parallel sweep queues through it without an operator ever seeing a refusal, short enough
// that a genuinely stuck peer surfaces as an error instead of an indefinite hang.
//
// A var, not a const, so the contention tests can assert the REFUSAL branch without spending
// a minute of wall clock proving it. Nothing in production writes it, and no flag or
// environment variable exposes it — a deployment cannot lengthen the window into an
// indefinite hang or shorten it into spurious refusals.
var gitlabRotateLockWait = 60 * time.Second

// gitlabRepoResolved reports whether --repo AFFIRMATIVELY resolves to the GitLab forge, so a
// bare `desktoken <role> --repo <gitlab-slug>` with NO explicit --forge takes the GitLab PAT
// custody path instead of attempting a GitHub App mint. A PAT-backed GitLab bot provisions no
// GitHub App credential, so the App mint dies with `no App ID for App "<role>-app"` (exit 6) —
// #772 named that symptom, #798 closes it by routing the resolved forge.
//
// It resolves ONLY on a DEFINITE GitLab answer. A could-not-check resolution — no
// ASSAY_REPO_FORGES entry names the repo AND the origin remote maps to no known forge — is NOT
// read as "it is GitLab": it returns false and the caller falls through to the GitHub mint, so
// every repo whose forge cannot be POSITIVELY resolved keeps its exact pre-#798 behaviour. This
// is the same three-state fall-through requireGitHubForge makes on the deskpost side (only an
// affirmative non-GitHub resolution diverts), never a guess. It reads the roster/remote, never
// a token file, so it costs no custody.
func gitlabRepoResolved(repo string) bool {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return false
	}
	owner, name, found := strings.Cut(repo, "/")
	if !found || owner == "" || name == "" {
		// A bare owner is not a repo coordinate ForgeKindFor can resolve — leave forge
		// selection to the GitHub mint's own owner resolution (resolveMintOwner).
		return false
	}
	res, err := deskkit.ForgeKindFor(deskkit.ForgeRepo{Owner: owner, Name: name})
	if err != nil {
		return false // could-not-check → fall through to the GitHub mint, never assume GitLab.
	}
	return res.Kind == deskkit.ForgeGitLab
}

// gitlabHTTPClient is the client the GitLab arm hands the provider: the client authHTTP would
// use (a measured client a caller passes, else the httpClient hook). The provider copies it
// and refuses every redirect before either GitLab call (review finding SEC-1: PRIVATE-TOKEN is
// a custom header net/http forwards across hosts on a followed redirect), so the measurement
// survives and the refusal is added where the token is sent.
func gitlabHTTPClient(clients []*http.Client) *http.Client { return authHTTP(clients) }

// rotateGitLabToken POSTs to the self-rotation endpoint with the CURRENT token through the
// provider's one implementation of that call (redirects refused).
func rotateGitLabToken(base, current string, clients ...*http.Client) (*credential.GitLabRotateResult, error) {
	return credential.RotateGitLabToken(gitlabHTTPClient(clients), base, current)
}

// gitlabSelfCheckPath is the endpoint the post-rotation self-check reads.
const gitlabSelfCheckPath = credential.GitLabSelfCheckPath

// gitlabSelfCheck performs ONE read-only GET of gitlabSelfCheckPath with token through the
// provider (redirects refused) and returns the status the forge answered.
func gitlabSelfCheck(base, token string, clients ...*http.Client) (int, error) {
	return credential.GitLabSelfCheck(gitlabHTTPClient(clients), base, token)
}

// gitlabCustodyWriteTarget resolves where a rotation writes: path itself, or a symlinked
// custody path's resolved target, so the rename lands on the file the link points AT.
func gitlabCustodyWriteTarget(path string) (target string, linked bool, err error) {
	return credential.GitLabCustodyWriteTarget(path)
}

// cmdGitLabRotate implements `desktoken --forge gitlab <role>`: the provider verifies custody,
// and — with rotate — rotates in place under the per-role lock, write-verifies and self-checks
// the successor; this prints the custody PATH.
//
// With rotate=false (`--no-rotate`) the provider performs the SAME custody checks, makes no
// network contact and leaves the credential untouched. That is the shape a read-only or dry-run
// verb asks for: it needs to know WHERE the role's credential is, not to spend a rotation it
// will not use.
//
// Refusal behaviors mirror the GitHub key path: a missing custody file, a non-regular-file
// custody, a link to another role's custody, and a wrong file mode each refuse (exit 6) with a
// named remedy, and no token-shaped value ever reaches stdout, stderr, env, or argv.
func cmdGitLabRotate(role string, ac *auditCtx, rotate bool, clients ...*http.Client) error {
	// The audit verb records what was actually DONE. A no-rotate lookup logged as "rotate"
	// would put rotations in the trail that never happened.
	ac.verb = "rotate"
	if !rotate {
		ac.verb = "custody-read"
	}
	ac.role = role

	base, _ := gitlabAPIBase()
	cred, err := credential.GitLab(credential.GitLabRequest{
		Role:     role,
		Paths:    deskkit.CredentialPaths(),
		APIBase:  base,
		Rotate:   rotate,
		LockWait: gitlabRotateLockWait,
	}, ac.adapters(gitlabHTTPClient(clients)))
	if err != nil {
		return ac.providerFailed(err)
	}
	// Success: print the PATH only.
	fmt.Println(cred.Path)
	return nil
}
