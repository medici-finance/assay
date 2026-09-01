package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// apiBaseURL is the GitHub API/GraphQL base. It is overridable ONLY from in-package
// tests (a fake httptest server); there is deliberately NO env var or flag override — a
// production override could redirect the App-token mint or a review-post to an attacker.
// This is the same test-hook shape deskkit uses for its desk-tools directory.
// The host literal is sourced from deskkit.GitHubAPIBase (the forge module) so it is never
// constructed in a cmd package (the forge-abstraction seam); the var is kept so in-package tests can
// still point it at a fake server.
var apiBaseURL = deskkit.GitHubAPIBase

// reviewerBotLogin() is the reviewer App's bot identity — the unforgeable distinct actor.
// deskpost posts every review/comment/flip AS this App by minting the
// App installation token from the App private key; it NEVER falls back to the caller's
// gh token. A PR author therefore cannot forge an App approval, and only a
// holder of the App PEM can post as this login.
// The slug is adopter configuration, not a compiled-in constant:
// the `reviewer=<slug>:<id>` entry of ASSAY_TRUSTED_BOT_SLUGS binds it.
//
// isReviewerBot is a MATCHER, and that is the fix for an earlier review blocker.
// The previous shape returned the bare login and every gate compared
// `r.User.Login == reviewerBotLogin()`, on the stated premise that an unbound role
// yields "" and "" matches no author. It does match one: GitHub renders a deleted
// review author as `"user": null`, decoding to `Login: ""`. With the reviewer role
// unbound, a review with no author satisfied BOTH flip gates — the correctness lane
// returned found/APPROVED-at-head for a prose body, and the security lane returned pass
// for a body carrying the marker. Refusing on the unbound role is what closes it; the
// empty-login guard closes the actor half independently.
func isReviewerBot(login string) bool {
	want, ok := deskkit.RoleAppLogin("reviewer")
	if !ok || login == "" {
		return false
	}
	return login == want
}

// reviewerBotDisplay is for MESSAGE TEXT only — never a comparison.
func reviewerBotDisplay() string { return deskkit.RoleAppLoginOrEmpty("reviewer") }

// Token config env names follow the neutral per-role scheme (<ROLE>_*), so the tool is not coupled
// to one org. The App ID has NO source default — it must come from per-deployment config
// (the config home's apps.env exports REVIEWER_APP_ID); mint fails loudly if it is unset.
const (
	envAppID     = "REVIEWER_APP_ID"
	envInstallID = "REVIEWER_INSTALL_ID"
	envPEM       = "REVIEWER_PEM"
)

// resolveReviewerPEM finds the reviewer key on the SAME App-credential search path
// apps.env is read from (deskkit.FindConfigFile / confighome.go) rather than at a
// hardcoded ~/.config/assay — the two drifting apart is #794, and desktoken alone
// honouring the search path leaves this tool minting from the old fixed directory.
//
// It returns the path it would use plus every directory searched, so a not-found refusal
// can name all of them instead of one.
func resolveReviewerPEM() (path string, searched []string) {
	p, dirs, _ := deskkit.FindConfigFile("reviewer-app.pem")
	return p, dirs
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// home expands a leading ~/ to the user's home directory.
func home(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// installForOwner maps a repo owner to its App installation ID (an App installed on more
// than one account resolves only the repos of ITS installation). The installation ids are
// per-deployment machine identities and MUST NOT live in source (a public-repo security review), so they
// are resolved from config exactly like the App ID and PEM path: env <ROLE>_INSTALL_ID (a
// single-installation override, what REVIEWER_INSTALL_ID has always been) or the per-owner
// <ROLE>_INSTALL_ID_<OWNER> line in the config home's apps.env. It FAILS CLOSED — a token
// cannot target an installation this process cannot name.
func installForOwner(owner string) (string, error) {
	return deskkit.InstallID("reviewer", owner)
}

// mintInstallationToken performs the JWT→installation-token exchange: read the App PEM
// (PKCS1 or PKCS8 RSA), sign a
// short-lived RS256 App JWT, POST it to the installation access_tokens endpoint, return
// the installation token. The ONE intentional deviation: the token is
// returned in memory and NEVER written to a file — deskpost mints fresh per invocation,
// no caching. The token is never logged, printed, or placed in an audit record.
//
// A missing/unreadable PEM is Unverifiable (exit 6) and names the manual path.
func mintInstallationToken(owner string) (string, error) {
	// Resolve the reviewer App ID: env REVIEWER_APP_ID, else the config home's apps.env — never a
	// source default (a fresh invocation works with no shell sourcing; the ID stays out of source).
	appID, err := deskkit.AppID("reviewer")
	if err != nil {
		return "", deskkit.Unverifiable(err.Error()+
			" — the App token cannot be minted without it", nil)
	}
	installID, ierr := installForOwner(owner)
	if ierr != nil {
		return "", deskkit.Unverifiable(ierr.Error()+
			" — the App token cannot be minted without it", nil)
	}
	var searched []string
	pemPath := strings.TrimSpace(os.Getenv(envPEM))
	if pemPath == "" {
		pemPath, searched = resolveReviewerPEM()
	}
	pemPath = home(pemPath)

	keyPEM, err := os.ReadFile(pemPath)
	if err != nil {
		where := pemPath
		if len(searched) > 0 {
			where = fmt.Sprintf("%s (searched: %s)", pemPath, strings.Join(searched, ", "))
		}
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"cannot read reviewer App key at %s — restore it there (see the pr-review-desk skill), "+
				"or put the directory that has it at the head of the App-credential search path "+
				"(%s=<dir>), or name the file outright (%s=<file>); "+
				"the App token cannot be minted without it (#794)",
			where, deskkit.EnvConfigHome, envPEM), err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", deskkit.Unverifiable("no PEM block in reviewer App key at "+pemPath, nil)
	}
	var key *rsa.PrivateKey
	if k, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		key = k
	} else if k8, e2 := x509.ParsePKCS8PrivateKey(block.Bytes); e2 == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return "", deskkit.Unverifiable("reviewer App PKCS8 key is not RSA", nil)
		}
		key = rk
	} else {
		return "", deskkit.Unverifiable("cannot parse reviewer App RSA private key (tried PKCS1 and PKCS8)", err)
	}

	now := time.Now()
	header := b64url([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`,
		now.Add(-60*time.Second).Unix(), now.Add(9*time.Minute).Unix(), appID)
	signingInput := header + "." + b64url([]byte(claims))
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", deskkit.Unverifiable("cannot sign reviewer App JWT", err)
	}
	jwt := signingInput + "." + b64url(sig)

	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", apiBaseURL, installID)
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", deskkit.Unverifiable("POST access_tokens failed", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", deskkit.Unverifiable(fmt.Sprintf("access_tokens HTTP %d", resp.StatusCode), nil)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", deskkit.Unverifiable("cannot parse access_tokens response", err)
	}
	if out.Token == "" {
		return "", deskkit.Unverifiable("empty token in access_tokens response", nil)
	}
	return out.Token, nil
}

// ghClient is the App-authenticated GitHub client for one (owner, repo). The token lives
// only in memory. A single 401 mid-operation triggers exactly ONE re-mint+retry — the
// scoped exemption (a token re-mint re-verifies nothing about the world).
type ghClient struct {
	owner, repo string
	token       string
	http        *http.Client
}

func newGHClient(owner, repo string) (*ghClient, error) {
	tok, err := mintInstallationToken(owner)
	if err != nil {
		return nil, err
	}
	return &ghClient{owner: owner, repo: repo, token: tok, http: http.DefaultClient}, nil
}

// apiError is a non-2xx REST/GraphQL response. Callers map it to Unverifiable (exit 6):
// an API error mid-check means the precondition could not be positively verified.
//
// A 403 additionally carries the App permission that gates the endpoint (scope) and the
// installation the token was minted for (install) — see appScopeFor.
type apiError struct {
	status  int
	method  string
	path    string
	scope   string // App permission gating this path, "" when unknown (403 only)
	install string // installation the token was minted for (403 only)
}

func (e *apiError) Error() string {
	base := fmt.Sprintf("GitHub API %s %s returned HTTP %d", e.method, e.path, e.status)
	if e.status != http.StatusForbidden {
		return base
	}
	// #348 / #252: a raw "returned HTTP 403" reads exactly like a network
	// blip, so `deskpost ready` 403s get mistaken for transient failures and re-run
	// through a raw `gh pr ready` that has none of this tool's preconditions. A 403 from an App
	// installation token is an ANSWER, not a failure: the installation does not hold the
	// permission, and it will not hold it on the next attempt either.
	msg := base + " — 403 from an App installation token is a PERMISSION answer, not a" +
		" transient failure: retrying changes nothing until the grant does"
	if e.scope != "" {
		msg += fmt.Sprintf(", and this endpoint is gated by the reviewer App permission %q", e.scope)
	}
	if e.install != "" {
		msg += fmt.Sprintf(" (installation %s; the grant is per-App but the TOKEN embeds the"+
			" permission set at mint time, so a token minted before the grant still 403s)", e.install)
	}
	return msg + ". Grant it and re-mint (docs/github-apps-setup.md), or record it as deliberately" +
		" withheld — but do NOT route around this refusal with a raw `gh pr ready` / `gh pr review`," +
		" which drops every precondition this call exists to verify."
}

// appScopeFor names the GitHub App permission that gates a REST path, for the 403
// diagnosis above. It is a DIAGNOSTIC, never a gate: an unrecognised path yields "" and
// the error degrades to the generic 403 text. Deliberately keyed on the path SHAPE the
// client itself builds (github.go is the only place these paths are constructed), so a
// new call site that forgets to add an entry loses a hint, never correctness.
func appScopeFor(method, path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	switch {
	case strings.HasSuffix(path, "/status"):
		// The LEGACY combined-status endpoint. Both this and check-runs must be readable —
		// ready.go aborts on either, which is why "fall back to check-runs when status
		// 403s" could never have worked (#348).
		return "statuses: read"
	case strings.HasSuffix(path, "/check-runs"):
		return "checks: read"
	case strings.Contains(path, "/pulls/"):
		if method == http.MethodGet {
			return "pull_requests: read"
		}
		return "pull_requests: write"
	case strings.Contains(path, "/issues/"):
		if method == http.MethodGet {
			return "issues: read"
		}
		return "issues: write"
	default:
		// /graphql and /app/installations/... are not gated by a single named permission.
		return ""
	}
}

// doJSON performs a REST call, decoding a 2xx JSON body into out (if non-nil). On a 401
// it re-mints the installation token ONCE and retries. Any other non-2xx is an *apiError.
func (c *ghClient) doJSON(method, path string, in, out any) error {
	return c.doJSONRetry(method, path, in, out, true)
}

func (c *ghClient) doJSONRetry(method, path string, in, out any, allowRemint bool) error {
	var bodyReader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return deskkit.Unverifiable("cannot marshal request body", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBaseURL+path, bodyReader)
	if err != nil {
		return deskkit.Unverifiable("cannot build request", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("%s %s failed", method, path), err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized && allowRemint {
		tok, merr := mintInstallationToken(c.owner)
		if merr != nil {
			return merr
		}
		c.token = tok
		return c.doJSONRetry(method, path, in, out, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := &apiError{status: resp.StatusCode, method: method, path: path}
		if resp.StatusCode == http.StatusForbidden {
			e.scope = appScopeFor(method, path)
			e.install, _ = installForOwner(c.owner)
		}
		return e
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return deskkit.Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), err)
		}
	}
	return nil
}

// --- REST payload shapes (only the fields we consume) ---

type prInfo struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Draft  bool   `json:"draft"`
	NodeID string `json:"node_id"`
	// ChangedFiles is GitHub's OWN count of the files this PR touches. It is the
	// reconciliation partner for listFiles, exactly as TotalCount is for the two CI
	// rollups: without it, a files walk that stops early believes it saw
	// everything, and a risk path hidden past the short read waives the gate.
	ChangedFiles int `json:"changed_files"`
	User         struct {
		Login string `json:"login"` // REST rendering: App authors are "<slug>[bot]"
		ID    int64  `json:"id"`    // permanent numeric id (recycled-login defense)
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type reviewInfo struct {
	// ID is GitHub's review id — surfaced in dedup messages (#518 direction 2) so a
	// suppressed caller can tell WHICH review suppressed it without another API call.
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	State       string `json:"state"` // APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED | PENDING
	CommitID    string `json:"commit_id"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

type combinedStatus struct {
	State      string `json:"state"`
	TotalCount int    `json:"total_count"`
	Statuses   []struct {
		State   string `json:"state"` // success | pending | failure | error
		Context string `json:"context"`
	} `json:"statuses"`
}

type checkRunsResp struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`     // queued | in_progress | completed
		Conclusion string `json:"conclusion"` // success | failure | neutral | ...
	} `json:"check_runs"`
}

// prFile is ONE entry of GET /pulls/{n}/files. PreviousFilename is set only when
// Status is "renamed" — and decoding it is load-bearing, not cosmetic: without it a
// `git mv secrets/auth/token.go config/authz/token.go`
// plus arbitrary edits reports ONLY the new path, and the risk-path gate never sees
// the security path the change actually came from. One `git mv`, plausibly deniable as
// a refactor, would de-risk-class the most security-relevant edit shape there is.
// Triggers are matched against BOTH paths (see prFilePaths).
type prFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"` // added | modified | removed | renamed | ...
}

// prFilePaths flattens file entries to every path the change touched — the new path AND
// the pre-rename path. Blank entries are preserved as blank so the classifier's
// fail-closed check on malformed listings still sees them.
func prFilePaths(files []prFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Filename)
		if f.PreviousFilename != "" && f.PreviousFilename != f.Filename {
			out = append(out, f.PreviousFilename)
		}
	}
	return out
}

// issueInfo is the /issues/{n} rendering. GitHub numbers issues and pull requests from
// ONE per-repo sequence and serves BOTH from the issues endpoint, so this shape is what
// resolves a bare number to an object kind: the `pull_request` sub-object is present iff
// the number is a pull request (the documented discriminator) and absent for a plain
// issue. Only the fields the comment path consumes are decoded — the author identity the
// trust gate needs, and that discriminator.
type issueInfo struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
}

// getPR fetches the pull request (head SHA, state, draft, node id).
func (c *ghClient) getPR(pr int) (*prInfo, error) {
	var p prInfo
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", c.owner, c.repo, pr)
	if err := c.doJSON(http.MethodGet, path, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// getIssue fetches the ISSUE rendering of a number. It answers both "does this number
// exist at all" and "is it an issue or a pull request" (see issueInfo.PullRequest) — the
// two questions a 404 from getPR leaves open.
func (c *ghClient) getIssue(n int) (*issueInfo, error) {
	var i issueInfo
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", c.owner, c.repo, n)
	if err := c.doJSON(http.MethodGet, path, nil, &i); err != nil {
		return nil, err
	}
	return &i, nil
}

// isNotFound reports whether err is a 404 from the REST layer. A 404 is the ONLY status
// that licenses the kind re-resolution below: every other failure (403, 5xx, transport)
// says nothing about what the number IS, and must propagate unchanged.
//
// It UNWRAPS: deskkit.DeskError implements Unwrap, so errors.As would also find an
// *apiError nested inside an Unverifiable. No path does that today — doJSONRetry returns
// *apiError bare and reserves Unverifiable for marshal/build/transport/parse failures,
// which carry no status — but anything that starts wrapping an apiError in a DeskError
// silently WIDENS what counts as "404", and with it what licenses the second lookup.
func isNotFound(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.status == http.StatusNotFound
}

// getPRHead is the light re-read used for the head-pin (review) and the TOCTOU re-read
// (ready) — it fetches only the current head SHA.
func (c *ghClient) getPRHead(pr int) (string, error) {
	p, err := c.getPR(pr)
	if err != nil {
		return "", err
	}
	return p.Head.SHA, nil
}

// listReviews returns all reviews on the PR (paginated).
func (c *ghClient) listReviews(pr int) ([]reviewInfo, error) {
	var all []reviewInfo
	for page := 1; ; page++ {
		var chunk []reviewInfo
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=100&page=%d", c.owner, c.repo, pr, page)
		if err := c.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		all = append(all, chunk...)
		if len(chunk) < 100 {
			break
		}
	}
	return all, nil
}

// timelineEvent is one entry of the issue/PR timeline. Only `labeled` events matter to the
// model-capability floor, and only their label name plus the login that APPLIED it: that
// applier is what separates a dispatcher attestation from a self-applied stamp.
type timelineEvent struct {
	Event string `json:"event"`
	Label struct {
		Name string `json:"name"`
	} `json:"label"`
	Actor struct {
		Login string `json:"login"`
	} `json:"actor"`
}

// listLabelEvents returns the PR's `labeled` timeline events — the label name AND the login
// that applied it — which the applier-aware stamp reader (AttestedModelStampOf) needs to
// tell a dispatcher attestation from a self-applied one. It walks every page. An empty
// result is a PR with no labels, which the floor reads as UNATTESTED; a read error
// propagates and the caller refuses could-not-check rather than proceeding blind.
func (c *ghClient) listLabelEvents(pr int) ([]deskkit.LabelEvent, error) {
	var out []deskkit.LabelEvent
	for page := 1; ; page++ {
		var chunk []timelineEvent
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/timeline?per_page=100&page=%d", c.owner, c.repo, pr, page)
		if err := c.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, e := range chunk {
			if e.Event != "labeled" {
				continue
			}
			out = append(out, deskkit.LabelEvent{Name: e.Label.Name, AppliedBy: e.Actor.Login})
		}
		if len(chunk) < 100 {
			break
		}
	}
	return out, nil
}

// maxFilePages bounds the files walk (100/page). Exceeding it leaves the fetched entry
// count BELOW the PR's changed_files, which the caller reconciles and fails closed on —
// the same degrade-closed shape maxCIPages has for the CI rollups.
const maxFilePages = 40 // 4000 entries; GitHub's own /files cap is 3000

// listFiles returns the PR's changed file ENTRIES (paginated). The caller must reconcile
// len(result) against prInfo.ChangedFiles before trusting it as a complete diff — a walk
// that stops on a short page cannot tell "that was the last page" from "the API stopped
// giving me pages", and the second reading is how a risk path stays invisible.
func (c *ghClient) listFiles(pr int) ([]prFile, error) {
	var all []prFile
	for page := 1; page <= maxFilePages; page++ {
		var chunk []prFile
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", c.owner, c.repo, pr, page)
		if err := c.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		all = append(all, chunk...)
		if len(chunk) < 100 {
			break
		}
	}
	return all, nil
}

// CI rollup paging. GitHub's default page size is THIRTY: a request that omits
// per_page silently returns only the first 30 statuses / check runs of a longer rollup,
// with the true size in total_count. The original single-shot reads therefore judged a
// head green on a partial view — a fail-OPEN (a failing 31st check run flipped the PR).
// Both rollups are now walked to exhaustion, and evalCI cross-checks what we hold against
// the total_count GitHub reported, so a rollup we cannot read IN FULL degrades CLOSED.
const (
	ciPerPage  = 100 // GitHub's maximum
	maxCIPages = 25  // 2500 items; exceeding it leaves len < TotalCount → ciPending (closed)
)

// combinedStatusAt / checkRunsAt fetch the two CI rollups at a commit, ACROSS ALL PAGES.
// TotalCount is carried through from the first page — the count GitHub asserts exists at
// the head, which evalCI reconciles against the items actually fetched.
func (c *ghClient) combinedStatusAt(sha string) (*combinedStatus, error) {
	var agg combinedStatus
	for page := 1; page <= maxCIPages; page++ {
		var cs combinedStatus
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/status?per_page=%d&page=%d",
			c.owner, c.repo, sha, ciPerPage, page)
		if err := c.doJSON(http.MethodGet, path, nil, &cs); err != nil {
			return nil, err
		}
		if page == 1 {
			agg.State = cs.State
			agg.TotalCount = cs.TotalCount
		}
		agg.Statuses = append(agg.Statuses, cs.Statuses...)
		if len(cs.Statuses) < ciPerPage {
			break // short page → last page
		}
	}
	return &agg, nil
}

func (c *ghClient) checkRunsAt(sha string) (*checkRunsResp, error) {
	var agg checkRunsResp
	for page := 1; page <= maxCIPages; page++ {
		var cr checkRunsResp
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=%d&page=%d",
			c.owner, c.repo, sha, ciPerPage, page)
		if err := c.doJSON(http.MethodGet, path, nil, &cr); err != nil {
			return nil, err
		}
		if page == 1 {
			agg.TotalCount = cr.TotalCount
		}
		agg.CheckRuns = append(agg.CheckRuns, cr.CheckRuns...)
		if len(cr.CheckRuns) < ciPerPage {
			break
		}
	}
	return &agg, nil
}

// postReview submits a head-pinned review AS THE APP. event is APPROVE or
// REQUEST_CHANGES; commit_id pins the verdict to the reviewed head.
func (c *ghClient) postReview(pr int, head, event, body string) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", c.owner, c.repo, pr)
	in := map[string]any{"commit_id": head, "event": event, "body": body}
	return c.doJSON(http.MethodPost, path, in, nil)
}

// postComment posts a plain issue comment AS THE APP.
func (c *ghClient) postComment(pr int, body string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", c.owner, c.repo, pr)
	in := map[string]any{"body": body}
	return c.doJSON(http.MethodPost, path, in, nil)
}

// fetchPRTrustPayload runs the trust-gate GraphQL QUERY (deskkit.PRTrustQuery) and
// returns the raw response body for deskkit.ParsePRTrustPayload. A read, not a
// mutation — GraphQL rides POST but changes nothing.
func (c *ghClient) fetchPRTrustPayload(pr int) ([]byte, error) {
	in := map[string]any{
		"query": deskkit.PRTrustQuery,
		"variables": map[string]any{
			"owner": c.owner, "name": c.repo, "number": pr,
		},
	}
	var raw json.RawMessage
	if err := c.doJSON(http.MethodPost, "/graphql", in, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// fetchIssueTrustPayload is the ISSUE form of the above: same GraphQL read, deskkit's
// IssueTrustQuery (issue body edit time + comment events — an issue has no reviews or
// review threads, which is the whole difference between the two queries).
func (c *ghClient) fetchIssueTrustPayload(n int) ([]byte, error) {
	in := map[string]any{
		"query": deskkit.IssueTrustQuery,
		"variables": map[string]any{
			"owner": c.owner, "name": c.repo, "number": n,
		},
	}
	var raw json.RawMessage
	if err := c.doJSON(http.MethodPost, "/graphql", in, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// prTrustGate enforces the desk trust gate (deskkit/trust.go) on a mutating verb's
// target PR: a PR authored outside the compiled-in trusted set (login AND numeric id
// checked — REST has both) with no CURRENT blessing is REFUSED (exit 5,
// audited) — deskpost must never post a verdict, comment, or ready-flip on unvetted
// third-party work. Blessing is bless-then-edit aware (deskkit.Blessed);
// an unverifiable trust read is exit 6, never a guess.
func prTrustGate(c *ghClient, pr int, authorLogin string, authorID int64) error {
	return trustGate(c, kindPR, pr, authorLogin, authorID)
}

// trustGate is the ONE implementation for BOTH object kinds — `comment` can land on either
// since #296, and an issue in a PUBLIC repo is exactly the surface the gate was
// built for: arbitrary external users can open one, and the desk must not comment on unvetted
// third-party text until the blessing authority has blessed it. The two kinds differ only in which GraphQL
// query/parser pair reads the thread; keeping one body means a future tightening (say, the
// hash-pinned blessing in deskkit's TODO) cannot land on PRs and miss issues, which is
// precisely how a gate acquires a seam.
//
// The selector is the TYPED targetKind, not a string: a stringly-typed selector is itself a
// seam — a mismatched literal ("Issue" for "issue") would silently send an issue through the
// PR query. It fails closed today (the PR query returns no blessing, so the gate refuses),
// but "fails closed by luck" is not a property worth keeping when the compiler can remove
// the possibility.
func trustGate(c *ghClient, k targetKind, n int, authorLogin string, authorID int64) error {
	if deskkit.TrustedAuthorID(authorLogin, authorID) {
		return nil
	}
	kind := k.String()
	fetch, parse := c.fetchPRTrustPayload, deskkit.ParsePRTrustPayload
	if k == kindIssue {
		fetch, parse = c.fetchIssueTrustPayload, deskkit.ParseIssueTrustPayload
	}
	raw, err := fetch(n)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("cannot read trust events for %s #%d (trust gate)", kind, n), err)
	}
	bodyEdited, events, complete, perr := parse(raw)
	if perr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("cannot parse trust events for %s #%d (trust gate)", kind, n), perr)
	}
	if !complete || !deskkit.Blessed(bodyEdited, events) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s #%d author %q is not a trusted desk identity and carries no current blessing "+
				"(trust gate) — a comment from the configured blessing authority on the %s admits it; edits after a blessing re-quarantine",
			kind, n, authorLogin, kind))
	}
	return nil
}

// markReadyForReview flips a draft PR to ready via the GraphQL mutation (the only GitHub
// API for this transition; `gh pr ready` uses the same). This is deskpost's SOLE
// mutating verb beyond review/comment — there is no un-ready, merge, close, or edit.
func (c *ghClient) markReadyForReview(nodeID string) error {
	q := `mutation($id:ID!){markPullRequestReadyForReview(input:{pullRequestId:$id}){pullRequest{isDraft}}}`
	in := map[string]any{"query": q, "variables": map[string]any{"id": nodeID}}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return deskkit.Unverifiable("markPullRequestReadyForReview GraphQL error: "+out.Errors[0].Message, nil)
	}
	return nil
}

// RepoVisibility implements deskkit.RepoInfoFetcher for the App-authenticated client.
func (c *ghClient) RepoVisibility(owner, repo string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	var info struct {
		Visibility string `json:"visibility"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &info); err != nil {
		return "", err
	}
	if info.Visibility == "" {
		return "", fmt.Errorf("repo %s/%s has no .visibility field in API response", owner, repo)
	}
	return info.Visibility, nil
}

// IssueReactions implements deskkit.RepoInfoFetcher for the App-authenticated client.
func (c *ghClient) IssueReactions(owner, repo string, issueNumber int) ([]deskkit.Reaction, error) {
	// SINGLE PAGE, by decision — same reasoning as deskkit.HTTPRepoInfoFetcher's
	// IssueReactions, where it is written out in full. Fails closed past 100 reactions
	// on one issue; keep the two implementations in step.
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions?per_page=100", owner, repo, issueNumber)
	// The reactions API requires the squirrel-girl preview accept header.
	// Our doJSON method sets the standard accept header, so we need to use a raw request.
	url := apiBaseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot build reactions request", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.squirrel-girl-preview+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("GET %s failed", path), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		// Try once with reminted token
		tok, merr := mintInstallationToken(c.owner)
		if merr != nil {
			return nil, merr
		}
		c.token = tok
		req, _ = http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "token "+c.token)
		req.Header.Set("Accept", "application/vnd.github.squirrel-girl-preview+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp2, err2 := c.http.Do(req)
		if err2 != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("GET %s failed on retry", path), err2)
		}
		defer resp2.Body.Close()
		resp = resp2
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{status: resp.StatusCode, method: http.MethodGet, path: path}
	}
	body, _ := io.ReadAll(resp.Body)
	var reactions []deskkit.Reaction
	if err := json.Unmarshal(body, &reactions); err != nil {
		return nil, deskkit.Unverifiable("cannot parse reactions response", err)
	}
	return reactions, nil
}
