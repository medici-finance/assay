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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// apiBaseURL is the GitHub API base. It is overridable ONLY from in-package
// tests (a fake httptest server); there is deliberately NO env var or flag override.
var apiBaseURL = "https://api.github.com"

// httpClient is a test hook. It is deliberately NOT http.DefaultClient: DefaultClient has
// no Timeout, and every request this tool makes runs INSIDE the audit flock (writeflow.go).
// A hung connection would therefore pin the SHARED ~/.config/assay/audit.lock and
// starve deskpost and deskrelease along with it, for as long as the TCP stack took to give
// up. The 60s acquire deadline bounds the WAITERS; this bounds the HOLDER, which is the
// half that actually causes the starvation (#406).
//
// 30s is per-request, not per-invocation: an invocation makes up to ~6 calls (lookup, two
// mints, fetch, PUT, one 401 re-mint). It is generous for a few-KB Contents-API call and
// still an order of magnitude inside the 60s a waiter will tolerate.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// verifierBotLogin() is the verifier App's bot identity — the unforgeable
// identity for Evidence commits.
// The slug is adopter configuration, not a compiled-in constant:
// the `verifier=<slug>:<id>` entry of ASSAY_TRUSTED_BOT_SLUGS binds it.
//
// isVerifierBot is a MATCHER, and it is half of the fix for a review blocker.
// The previous shape returned the bare login on the stated premise that an
// unbound role yields "" and "" matches no author. It matched one: with the verifier
// role unbound, `checkAttribution("")` took the `author == verifierBotLogin()` arm and
// returned `detail="author=" err=nil` — an Evidence commit carrying NO author identity
// reported as PROVEN attribution, with the could-not-check third state unreachable.
func isVerifierBot(author string) bool {
	want, ok := deskkit.RoleAppLogin("verifier")
	if !ok || author == "" {
		return false
	}
	return author == want
}

// verifierBotDisplay is for MESSAGE TEXT only — never a comparison.
func verifierBotDisplay() string { return deskkit.RoleAppLoginOrEmpty("verifier") }

// installCache memoises the resolved installation ID per owner/name for the life of the
// process. mintVerifierToken is called two or three times per invocation (fetch, commit,
// and a 401 re-mint); without this the discovery GET would repeat each time.
var (
	installCacheMu sync.Mutex
	installCache   = map[string]string{}
)

// resolveInstallID returns the verifier App's installation ID for owner/name.
//
// It ASKS GitHub rather than carrying a constant. `GET /repos/{owner}/{repo}/installation`
// is authenticated with the App's own JWT and returns the installation of *that App* on
// that repo, so the answer cannot belong to a different App by construction.
//
// This replaces a hardcoded `installForOwner` that returned 100000001 / 100000002 under a
// doc comment claiming they were the verifier App's. They are the REVIEWER App's — proven
// against the API (#228):
//
//	$ gh api /orgs/medici-finance/installations
//	<reviewer App slug>   id=<installation id>
//	<verifier App slug>   id=<installation id>
//
// and the same two numbers are still hardcoded verbatim in cmd/deskpost/github.go, the
// reviewer App's tool. The failure was closed (a verifier JWT against a reviewer
// installation is rejected → exit 6) but it was also opaque, and a *correct* constant
// would only re-seat the class of bug — the next App install, org migration or new repo
// owner puts it straight back. Discovery has no constant to go stale.
//
// VERIFIER_INSTALL_ID still overrides, as the explicit escape hatch for an environment
// that cannot reach the discovery endpoint. There is deliberately no silent fallback: if
// discovery fails and no override is set, the tool is Unverifiable (exit 6), never
// "guess an ID and see".
func resolveInstallID(jwt, owner, name string) (string, error) {
	if v := os.Getenv("VERIFIER_INSTALL_ID"); v != "" {
		return v, nil
	}
	key := owner + "/" + name
	installCacheMu.Lock()
	cached, ok := installCache[key]
	installCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	url := fmt.Sprintf("%s/repos/%s/%s/installation", apiBaseURL, owner, name)
	req, rerr := http.NewRequest(http.MethodGet, url, nil)
	if rerr != nil {
		return "", deskkit.Unverifiable("cannot build installation-lookup request", rerr)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, rerr := httpClient.Do(req)
	if rerr != nil {
		return "", deskkit.Unverifiable("GET repos/"+key+"/installation failed", rerr)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", deskkit.Unverifiable(
			"the verifier App is not installed on "+key+
				" (GET /repos/"+key+"/installation → 404) — install it, or set VERIFIER_INSTALL_ID", nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", deskkit.Unverifiable(
			fmt.Sprintf("GET repos/%s/installation returned HTTP %d", key, resp.StatusCode), nil)
	}
	var out struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", deskkit.Unverifiable("cannot parse installation-lookup response for "+key, err)
	}
	id := out.ID.String()
	if id == "" || id == "0" {
		return "", deskkit.Unverifiable("empty installation id in lookup response for "+key, nil)
	}

	installCacheMu.Lock()
	installCache[key] = id
	installCacheMu.Unlock()
	return id, nil
}

// mintVerifierToken performs the JWT -> installation-token exchange for the verifier App.
// The token is returned in memory and NEVER written to a cache file or logged.
// Inherits the same JWT flow as deskpost's mintInstallationToken and desktoken's cmdToken.
//
// It takes the repo (owner AND name), not just the owner, because the installation is
// resolved per-repo from the App's own JWT — see resolveInstallID.
func mintVerifierToken(owner, name string) (string, error) {
	// Resolve the verifier App ID: env VERIFIER_APP_ID, else the config home's apps.env.
	appID, err := deskkit.AppID("verifier")
	if err != nil {
		return "", deskkit.Unverifiable(err.Error()+" — the verifier App token cannot be minted without it", nil)
	}
	// The key comes off the SAME App-credential search path apps.env was read from
	// (deskkit.FindConfigFile / confighome.go). Hardcoding ~/.config/assay here while a
	// deployment's provisioning wrote App material to a different directory is #794 — and
	// desktoken alone honouring the search path is only two thirds of the fix: this tool
	// mints its own verifier token and would keep failing in a fresh shell.
	var searched []string
	pemPath := strings.TrimSpace(os.Getenv("VERIFIER_PEM"))
	if pemPath == "" {
		pemPath, searched, _ = deskkit.FindConfigFile("verifier-app.pem")
	}
	pemPath = expandHome(pemPath)

	keyPEM, err := os.ReadFile(pemPath)
	if err != nil {
		where := pemPath
		if len(searched) > 0 {
			where = fmt.Sprintf("%s (searched: %s)", pemPath, strings.Join(searched, ", "))
		}
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"cannot read verifier App key at %s — restore it there, or put the directory that has it "+
				"at the head of the App-credential search path (%s=<dir>), or name the file outright "+
				"(VERIFIER_PEM=<file>), then retry; the verifier App token cannot be minted without it (#794)",
			where, deskkit.EnvConfigHome), err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", deskkit.Unverifiable("no PEM block in verifier App key at "+pemPath, nil)
	}
	var key *rsa.PrivateKey
	if k, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		key = k
	} else if k8, e2 := x509.ParsePKCS8PrivateKey(block.Bytes); e2 == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return "", deskkit.Unverifiable("verifier App PKCS8 key is not RSA", nil)
		}
		key = rk
	} else {
		return "", deskkit.Unverifiable("cannot parse verifier App RSA private key (tried PKCS1 and PKCS8)", err)
	}

	now := time.Now()
	header := b64([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`,
		now.Add(-60*time.Second).Unix(), now.Add(9*time.Minute).Unix(), appID)
	signingInput := header + "." + b64([]byte(claims))
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", deskkit.Unverifiable("cannot sign verifier App JWT", err)
	}
	jwt := signingInput + "." + b64(sig)

	installID, ierr := resolveInstallID(jwt, owner, name)
	if ierr != nil {
		return "", ierr
	}

	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", apiBaseURL, installID)
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := httpClient.Do(req)
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

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// --- GitHub Contents API ---

// fileInfo is the relevant fields from the GitHub Contents API response.
type fileInfo struct {
	SHA     string `json:"sha"`
	Content string `json:"content"` // base64-encoded
}

// fetchRemoteFile reads a file from the GitHub Contents API.
// Returns (sha, content, error). Content is the decoded bytes.
func fetchRemoteFile(owner, name, path, ref string) (sha string, content []byte, err error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		apiBaseURL, owner, name, path, ref)

	// Use the verifier App token so we read as the App (though for a public
	// repo the unauthenticated read would work — consistency is key).
	tok, terr := mintVerifierToken(owner, name)
	if terr != nil {
		return "", nil, terr
	}

	req, rerr := http.NewRequest(http.MethodGet, url, nil)
	if rerr != nil {
		return "", nil, deskkit.Unverifiable("cannot build GET request for "+path, rerr)
	}
	req.Header.Set("Authorization", "token "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, rerr := httpClient.Do(req)
	if rerr != nil {
		return "", nil, deskkit.Unverifiable("GET "+path+" failed", rerr)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", nil, notFoundError(fmt.Sprintf("file %q not found on branch %q", path, ref))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		// One re-mint + retry on 401.
		tok2, merr := mintVerifierToken(owner, name)
		if merr != nil {
			return "", nil, merr
		}
		req2, _ := http.NewRequest(http.MethodGet, url, nil)
		req2.Header.Set("Authorization", "token "+tok2)
		req2.Header.Set("Accept", "application/vnd.github+json")
		req2.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp2, rerr2 := httpClient.Do(req2)
		if rerr2 != nil {
			return "", nil, deskkit.Unverifiable("GET "+path+" failed on retry", rerr2)
		}
		defer resp2.Body.Close()
		resp = resp2
		body, _ = io.ReadAll(resp.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, deskkit.Unverifiable(
			fmt.Sprintf("GitHub API GET %s returned HTTP %d", path, resp.StatusCode), nil)
	}

	var fi fileInfo
	if err := json.Unmarshal(body, &fi); err != nil {
		return "", nil, deskkit.Unverifiable("cannot parse contents response for "+path, err)
	}
	if fi.SHA == "" {
		return "", nil, deskkit.Unverifiable("empty SHA in contents response for "+path, nil)
	}

	decoded, derr := base64.StdEncoding.DecodeString(fi.Content)
	if derr != nil {
		return "", nil, deskkit.Unverifiable("cannot decode base64 content for "+path, derr)
	}

	return fi.SHA, decoded, nil
}

// commitFile creates or updates a file via the GitHub Contents API as the verifier App.
// Returns the new blob/tree SHA and the git author name GitHub recorded on the commit
// ("" when the response carried none — see checkAttribution).
func commitFile(owner, name, path, branch, currentSHA string, content []byte) (newSHA, author string, err error) {
	tok, terr := mintVerifierToken(owner, name)
	if terr != nil {
		return "", "", terr
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	message := "Evidence: verification row for " + path

	in := map[string]any{
		"message": message,
		"content": encoded,
		"branch":  branch,
	}
	if currentSHA != "" {
		in["sha"] = currentSHA
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", apiBaseURL, owner, name, path)

	return commitFileWithTok(tok, url, in, owner, name)
}

func commitFileWithTok(tok, url string, in map[string]any, owner, name string) (newSHA, author string, err error) {
	body, merr := json.Marshal(in)
	if merr != nil {
		return "", "", deskkit.Unverifiable("cannot marshal commit request", merr)
	}

	req, rerr := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if rerr != nil {
		return "", "", deskkit.Unverifiable("cannot build PUT request", rerr)
	}
	req.Header.Set("Authorization", "token "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, rerr := httpClient.Do(req)
	if rerr != nil {
		return "", "", deskkit.Unverifiable("PUT contents failed", rerr)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		// One re-mint + retry.
		tok2, merr := mintVerifierToken(owner, name)
		if merr != nil {
			return "", "", merr
		}
		req2, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req2.Header.Set("Authorization", "token "+tok2)
		req2.Header.Set("Accept", "application/vnd.github+json")
		req2.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req2.Header.Set("Content-Type", "application/json")
		resp2, rerr2 := httpClient.Do(req2)
		if rerr2 != nil {
			return "", "", deskkit.Unverifiable("PUT contents failed on retry", rerr2)
		}
		defer resp2.Body.Close()
		respBody, _ = io.ReadAll(resp2.Body)
		resp = resp2
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", deskkit.Unverifiable(
			fmt.Sprintf("GitHub API PUT contents returned HTTP %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	// The Contents-API response carries the commit GitHub actually created. `commit.author`
	// is the GIT identity on it, which for an App-token commit is the App's bot — the
	// property Evidence commits are supposed to have.
	var result struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
		Commit struct {
			SHA    string `json:"sha"`
			Author struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", deskkit.Unverifiable("cannot parse commit response", err)
	}
	return result.Content.SHA, result.Commit.Author.Name, nil
}

// checkAttribution reads the git author GitHub recorded on the commit just created and
// decides whether the Evidence carries the unforgeable verifier identity.
//
// Three states, deliberately — not two:
//
//   - the verifier role is UNBOUND → REFUSED. Unverifiable (exit 6). Ordered first.
//   - isVerifierBot(author)          → proven. nil, ok=true.
//   - author is some OTHER non-empty login → PROVEN WRONG. Unverifiable (exit 6),
//     naming what landed. The commit is already on the remote (a Contents-API PUT is not
//     a local commit), so this is a report rather than a prevention — which is exactly why
//     it must be loud: a silent exit 0 is how an Evidence row signed by the wrong identity
//     gets treated as verify-desk output. The report is ONE-SHOT: once the misattributed
//     commit has landed, a re-run sees remote content == local content and returns noop
//     (exit 0) from the idempotency check above, without ever reaching this function. So
//     the flag fires exactly once, and the audit line it writes is the durable record —
//     which is why auditCtx.finalize carries the path/repo/branch/SHA into it rather than
//     logging the bare error (#406).
//   - author == "" → COULD NOT CHECK. Warned on stderr and recorded in the audit detail;
//     exit stays 0. Refusing here would fail the whole verify-desk loop on a response-shape
//     surprise with no evidence of anything wrong, and "could not check" is already this
//     repo's third state rather than a synonym for either verdict.
//
// The expected value is grounded in real commits, not in the constant's say-so: every
// deskevidence commit on example-org/tracker carries
// `commit.author.name = "<verifier-app-slug>[bot]"` and
// `commit.author.email = "<bot-user-id>+<verifier-app-slug>[bot]@users.noreply.github.com"`
// (`gh api search/commits`). Note the COMMITTER varies on those commits
// (worker App, or a human on a rebase), which is why only the author is gated.
// A FOURTH state was added at review: the verifier role UNBOUND. It is ordered FIRST
// because it is the only state in which this function does not know what the right
// answer looks like, and every arm below it would otherwise answer with false
// confidence — an unbound role used to make the empty author match the "proven" arm.
// It refuses (Unverifiable) rather than reporting could-not-check, because an unbound
// role is a CONFIGURATION defect the operator can fix, not a response-shape surprise.
func checkAttribution(author string) (detail string, err error) {
	want, bound := deskkit.RoleAppLogin("verifier")
	switch {
	case !bound:
		return "attribution=REFUSED (verifier role unbound)", deskkit.Unverifiable(
			"cannot check the Evidence commit's attribution: "+deskkit.RequireRole("verifier").Error(), nil)
	case isVerifierBot(author):
		return "author=" + want, nil
	case author == "":
		fmt.Fprintf(stderr, "deskevidence: WARNING: the commit response carried no author name — "+
			"could NOT verify the Evidence commit is attributed to %s\n", want)
		return "attribution=could-not-check", nil
	default:
		return "attribution=WRONG (" + author + ")", deskkit.Unverifiable(
			"the commit landed attributed to "+author+", not "+want+
				" — the Evidence row does NOT carry the verifier identity; check VERIFIER_APP_ID / VERIFIER_PEM / VERIFIER_INSTALL_ID", nil)
	}
}

// notFoundError wraps a 404 for callers to check.
func notFoundError(msg string) *deskkit.DeskError {
	return deskkit.Unverifiable(msg, nil)
}
