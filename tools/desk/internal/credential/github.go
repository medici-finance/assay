package credential

// github.go — the GitHub App installation-token arm: mint-or-reuse against the single
// on-disk cache, with the install-id caches in front of resolution (installid.go).
//
// The steps, and every refusal message, are the ones cmd/desktoken ran before the provider
// existed; they were moved here, not rewritten, so the CLI and the in-process caller cannot
// drift apart — there is one implementation and two thin adapters over it.

import (
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
	"sort"
	"strings"
	"time"
)

const (
	// GitHubHost is the one provider host this arm mints for. A request naming any other
	// host is refused: the App JWT and the installation token it buys are github.com
	// credentials, and there is no parameter by which a caller points the exchange elsewhere.
	GitHubHost = "github.com"

	// GitHubAPIBase is the single home of the GitHub REST/GraphQL host literal — every GitHub
	// call this provider makes goes to it, and deskkit.GitHubAPIBase aliases it, so the literal
	// is constructed in exactly one place. It is a constant, not a request field: the provider
	// is not a generic URL/token-request service.
	GitHubAPIBase = "https://api.github.com"

	// ScopeInstallation is the one grant scope this arm mints: the App installation's own
	// repository set. A repository-narrowed token is a different credential with a different
	// cache key, and is refused rather than silently widened to the installation.
	ScopeInstallation = "installation"

	// ReuseWindow is the threshold for reusing a cached installation token. GitHub App
	// installation tokens live ~60 min; a cache is reused while it is younger than 50. It is
	// also the CONSERVATIVE expiry reported for a legacy cache — one written before the mint
	// recorded the forge's own expires_at — so such a cache is never credited with a lifetime
	// nobody observed. deskkit's per-process memo bound (45m) stays strictly under it.
	ReuseWindow = 50 * time.Minute

	// TokenLifetimeMinutes is the nominal lifetime of a minted installation token, used only
	// by the --ttl report.
	TokenLifetimeMinutes = 60
)

// GitHubRequest is the explicit, already-resolved snapshot a GitHub credential operation
// runs against. Nothing in it is read from the environment by the provider: the caller
// (deskkit's snapshot builder) resolved every field from configuration before the call, so
// the CLI and the in-process lookup present the same inputs and get the same answer.
type GitHubRequest struct {
	// Host is the provider host; only GitHubHost is admitted.
	Host string
	// Role is the desk role the credential is for (KnownRole).
	Role string
	// AppName is the App the role mints as (the role→App binding) and AppID its numeric id.
	AppName string
	AppID   string
	// Owner is the account whose installation the token is for. It may be empty only when
	// InstallID is given.
	Owner string
	// InstallID is an explicit installation id (<PREFIX>_INSTALL_ID); empty resolves it.
	InstallID string
	// PEMPath is the App private key; PEMErr is a DEFERRED not-found error, raised only at
	// the point the key is read so precondition order is unchanged.
	PEMPath string
	PEMErr  error
	// TokenPath overrides the cache path (<PREFIX>_TOKEN); empty uses the default
	// <write-dir>/<role>-token-<install>.
	TokenPath string
	// Paths is the App-credential search path (install-id caches, probe, default cache dir).
	Paths Paths
	// Scope is the grant scope: "" or ScopeInstallation.
	Scope string
	// Fresh drops the cache and its sidecars and bypasses the install-id caches.
	Fresh bool
	// PathOnly resolves the cache path and returns it without reading or minting (--ttl).
	PathOnly bool
	// BeforeRename is a test seam run between the synced temp write and the publishing
	// rename of a newly minted token.
	BeforeRename func()
}

// TokenResult is the relevant fields of the access_tokens response.
type TokenResult struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	// Permissions is what GitHub actually GRANTED this installation token — the only place
	// the grant is observable. It is recorded beside the cache (.perms) for `deskroster
	// preflight` to check against the role's duties (#571).
	Permissions map[string]string `json:"permissions"`
}

// InstallationInfo is the relevant fields of GET /app/installations.
type InstallationInfo struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
}

// ErrInstallationGone is wrapped into a 404 from the access-tokens exchange: the
// installation id names an installation GitHub no longer has.
var ErrInstallationGone = errors.New("installation not found")

// GitHub obtains the role's GitHub App installation token: the on-disk cache when it is
// still within the reuse window, else a fresh mint that replaces it.
//
// CONTROLS, IN ORDER. Required adapters present (else refused); the live guard (DISABLED,
// STOP, heartbeat — its own row on refusal); the request's host, role, App, installation and
// scope admissible; the cache refused when it is a symlink or not exactly 0600; the private
// key refused when missing or readable by others.
//
// RECORDS. A reuse writes none. A mint writes one. A refusal past the guard writes one. A
// path-only lookup writes none (its caller reports the TTL and owns that row).
// semantic: S-role-credential
func GitHub(req GitHubRequest, ad Adapters) (Credential, error) {
	const verb = "mint"
	if e := ad.check(false); e != nil {
		ad.audit(verb, e, e.Error())
		return Credential{}, e
	}
	if gerr := ad.Guard(); gerr != nil {
		return Credential{}, gerr // the guard recorded its own refusal
	}
	cred, detail, suppress, err := githubOperation(req, ad)
	if err == nil && req.PathOnly {
		return cred, nil
	}
	if ferr := ad.finish(verb, err, detail, suppress); ferr != nil {
		return Credential{}, ferr
	}
	return cred, nil
}

// admitGitHub is the lower custody boundary: every refusal here holds with the CLI bypassed.
func admitGitHub(req GitHubRequest) *Error {
	if req.Host != GitHubHost {
		return refused(fmt.Sprintf("credential provider: host %q is not %s — the GitHub App arm mints only for "+
			"%s and never sends an App JWT or installation token to another host", req.Host, GitHubHost, GitHubHost))
	}
	if !KnownRole(req.Role) {
		return refuseRole(req.Role)
	}
	if !cacheNameOK(req.AppName) {
		return refused(fmt.Sprintf("credential provider: App name %q for role %s is not a valid App binding", req.AppName, req.Role))
	}
	if strings.TrimSpace(req.AppID) == "" {
		return refused(fmt.Sprintf("credential provider: no App ID for App %q", req.AppName))
	}
	if req.Scope != "" && req.Scope != ScopeInstallation {
		return refused(fmt.Sprintf("credential provider: scope %q is not admitted — this arm mints only the App "+
			"installation's own grant (%q); a narrowed or widened token is a different credential", req.Scope, ScopeInstallation))
	}
	if req.InstallID != "" && !allDigits(req.InstallID) {
		return refused(fmt.Sprintf("credential provider: installation id %q is not an installation id (digits only)", req.InstallID))
	}
	if req.InstallID == "" && strings.TrimSpace(req.Owner) == "" {
		return refused(fmt.Sprintf("credential provider: no account named for the %s App token — an installation "+
			"token is per account, so there is nothing to mint against", req.Role))
	}
	return nil
}

func githubOperation(req GitHubRequest, ad Adapters) (cred Credential, detail string, suppress bool, err error) {
	if e := admitGitHub(req); e != nil {
		return Credential{}, "", false, e
	}
	role, appName, owner := req.Role, req.AppName, req.Owner

	installID := req.InstallID
	// installIDFromCache records that the id came from one of the two caches. It scopes the
	// 404 invalidation route: only a CACHED id is removed when the exchange rejects it.
	var installIDFromCache bool
	var prebuiltJWT string

	if installID == "" {
		// CACHE BEFORE RESOLUTION (#1036). Both probes are local, both are positive identity
		// matches on (App, account), and both fall through to resolution on any doubt.
		// --fresh skips both: it means "the App or its permissions may have changed".
		if !req.Fresh {
			if id, ok := probeCachedInstallID(req.Paths, ad.Now(), role, appName, owner); ok {
				installID, installIDFromCache = id, true
			} else if id, ok := readInstallIDCache(req.Paths, ad.Now(), appName, owner); ok {
				installID, installIDFromCache = id, true
			}
		}
	}

	if installID == "" {
		// Must read PEM, sign JWT before we know the install ID.
		jwt, kerr := signAppJWT(req, ad.Now())
		if kerr != nil {
			return Credential{}, "", false, kerr
		}
		prebuiltJWT = jwt
		resolvedID, rerr := resolveInstallID(ad.HTTP, jwt, owner)
		if rerr != nil {
			return Credential{}, "", false, unverifiable("resolve installation for owner "+owner, rerr)
		}
		installID = resolvedID
		// Record the answer so the NEXT cold token cache costs no installations call.
		writeInstallIDCache(req.Paths, appName, owner, installID)
	}

	// Per-install token cache path, written to the head of the App-credential search path
	// (#794: key, cache, apps.env and provisioning dir are one directory).
	tokenPath := req.TokenPath
	if tokenPath == "" {
		p, ok := req.Paths.writePath(fmt.Sprintf("%s-token-%s", role, installID))
		if !ok {
			return Credential{}, "", false, refused("credential provider: no App-credential directory to hold the " + role + " token cache")
		}
		tokenPath = p
	}

	if req.PathOnly {
		return Credential{Path: tokenPath, Outcome: OutcomePathOnly, InstallID: installID}, "", false, nil
	}

	// --fresh: drop the cached token and EVERY sidecar before the reuse check, so a mint that
	// follows a GitHub-App permission change cannot be short-circuited by the reuse window,
	// and so .perms is rewritten with the NEW grant (#571). Idempotent.
	if req.Fresh {
		if rmErr := os.Remove(tokenPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return Credential{}, "", false, unverifiable("cannot remove cached token for --fresh: "+tokenPath, rmErr)
		}
		if rmErr := os.Remove(PermsPath(tokenPath)); rmErr != nil && !os.IsNotExist(rmErr) {
			return Credential{}, "", false, unverifiable("cannot remove token .perms sidecar for --fresh: "+PermsPath(tokenPath), rmErr)
		}
		if rmErr := os.Remove(OwnerSidecarPath(tokenPath)); rmErr != nil && !os.IsNotExist(rmErr) {
			return Credential{}, "", false, unverifiable("cannot remove token .owner sidecar for --fresh: "+OwnerSidecarPath(tokenPath), rmErr)
		}
		if rmErr := os.Remove(ExpirySidecarPath(tokenPath)); rmErr != nil && !os.IsNotExist(rmErr) {
			return Credential{}, "", false, unverifiable("cannot remove token .expires sidecar for --fresh: "+ExpirySidecarPath(tokenPath), rmErr)
		}
	}

	// Reuse the cached token while it is inside the reuse window. Lstat, not Stat: the cache is
	// this provider's own regular file, so a symlink here is refused — never handed out as the
	// role's token, and never written through by the mint below.
	if _, fi, serr := ad.Custody.Lstat(tokenPath, false); serr == nil {
		if fi.Mode().Perm() != 0o600 {
			return Credential{}, "", false, unverifiable(
				fmt.Sprintf("token cache at %s has permissions %o; must be 0600", tokenPath, fi.Mode().Perm()), nil)
		}
		now := ad.Now()
		age := now.Sub(fi.ModTime())
		expires, recorded := cachedExpiry(tokenPath, fi.ModTime())
		if age < ReuseWindow && (!recorded || now.Before(expires)) {
			raw, rerr := os.ReadFile(tokenPath)
			if rerr != nil {
				return Credential{}, "", false, unverifiable("cannot read token cache: "+tokenPath, rerr)
			}
			if tok := strings.TrimSpace(string(raw)); tok != "" {
				// ADOPT this cache file for the probe if it is not adopted already. Writing a
				// separate file does not touch the token's mtime, so adopting never extends
				// its life — it only lets the NEXT call recognise it without resolving.
				if owner != "" && !SidecarMatches(tokenPath, appName, owner) {
					writeOwnerSidecar(tokenPath, appName, owner)
				}
				detail = fmt.Sprintf("reused cached %s token [app=%s install %s] (%dm old)", role, appName, installID, int(age.Minutes()))
				return Credential{
					Secret:    Secret{v: tok},
					Path:      tokenPath,
					ExpiresAt: expires,
					Outcome:   OutcomeReused,
					InstallID: installID,
				}, detail, true, nil // no network call, no mint, nothing changed — no record
			}
			// An EMPTY cache is not a credential: fall through and replace it.
		}
	} else if ad.Custody.IsLinkRefusal(serr) {
		return Credential{}, "", false, unverifiable(serr.Error(), nil)
	} else if !os.IsNotExist(serr) {
		return Credential{}, "", false, unverifiable("cannot stat token cache: "+tokenPath, serr)
	}

	// --- Mint new token ---
	jwt := prebuiltJWT
	if jwt == "" {
		var kerr *Error
		jwt, kerr = signAppJWT(req, ad.Now())
		if kerr != nil {
			return Credential{}, "", false, kerr
		}
	}

	result, xerr := ExchangeJWT(ad.HTTP, jwt, installID)
	if xerr != nil {
		// A 404 on an id that came from a CACHE is the one failure the caches can cause. Drop
		// the entry and say so. Only a cached id is invalidated — an explicit install id is the
		// operator's own statement and is never second-guessed here.
		if installIDFromCache && errors.Is(xerr, ErrInstallationGone) && owner != "" {
			invalidateInstallIDCache(req.Paths, appName, owner)
			return Credential{}, "", false, unverifiable(fmt.Sprintf(
				"installation %s (cached for app=%s owner=%s) is no longer valid: %v. The cached "+
					"installation id has been removed; re-run and it will be resolved again.",
				installID, appName, owner, xerr), xerr)
		}
		return Credential{}, "", false, unverifiable("exchange JWT for installation token", xerr)
	}

	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return Credential{}, "", false, unverifiable("mkdir token dir", err)
	}
	if err := os.Chmod(filepath.Dir(tokenPath), 0o700); err != nil {
		return Credential{}, "", false, unverifiable("chmod token dir", err)
	}
	if err := WriteTokenCache(tokenPath, result.Token, req.BeforeRename); err != nil {
		return Credential{}, "", false, unverifiable("write token to "+tokenPath, err)
	}
	writePerms(tokenPath, result.Permissions)
	if owner != "" {
		writeOwnerSidecar(tokenPath, appName, owner)
	}
	expires := mintedExpiry(tokenPath, result.ExpiresAt, ad.Now())

	detail = fmt.Sprintf("minted new %s token [app=%s install %s] (expires %s)", role, appName, installID, result.ExpiresAt)
	return Credential{
		Secret:    Secret{v: result.Token},
		Path:      tokenPath,
		ExpiresAt: expires,
		Outcome:   OutcomeMinted,
		InstallID: installID,
	}, detail, false, nil
}

// signAppJWT reads and checks the App private key and signs the App JWT. The key is READ
// here, so a deferred not-found from the snapshot is raised here (#794).
func signAppJWT(req GitHubRequest, now time.Time) (string, *Error) {
	if req.PEMErr != nil {
		var pe *Error
		if errors.As(req.PEMErr, &pe) {
			return "", pe
		}
		return "", unverifiable(req.PEMErr.Error(), nil)
	}
	pemPath := req.PEMPath
	fi, perr := os.Stat(pemPath)
	if perr != nil {
		if os.IsNotExist(perr) {
			return "", unverifiable("private key not found at "+pemPath, perr)
		}
		return "", unverifiable("cannot stat private key at "+pemPath, perr)
	}
	if merr := CheckPrivateKeyMode(pemPath, fi.Mode()); merr != nil {
		return "", merr
	}
	keyPEM, rerr := os.ReadFile(pemPath)
	if rerr != nil {
		return "", unverifiable("cannot read private key at "+pemPath, rerr)
	}
	key, kerr := ParsePrivateKey(keyPEM)
	if kerr != nil {
		return "", unverifiable("parse private key from "+pemPath, kerr)
	}
	jwt, jerr := BuildJWT(req.AppID, now, key)
	if jerr != nil {
		return "", unverifiable("sign JWT", jerr)
	}
	return jwt, nil
}

// --- key parsing / JWT -------------------------------------------------------------------

// ParsePrivateKey tries PKCS1 then PKCS8 decoding of a PEM-encoded RSA key.
func ParsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		return key, nil
	}
	return nil, fmt.Errorf("parse RSA private key (tried PKCS1 and PKCS8)")
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// BuildJWT constructs an RS256-signed App JWT valid for 9 minutes, with iat skewed 60 s into
// the past for clock drift.
func BuildJWT(appID string, now time.Time, key *rsa.PrivateKey) (string, error) {
	header := b64url([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`,
		now.Add(-60*time.Second).Unix(),
		now.Add(9*time.Minute).Unix(),
		appID,
	)
	payload := b64url([]byte(claims))
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}

// PrivateKeyModeOK reports whether a private-key file's mode keeps the key away from OTHER
// principals: reject if readable by others (0o004) or writable by group or others (0o022).
// Group-read is admitted deliberately — a Kubernetes Secret volume read through fsGroup is
// necessarily 0440 — while other-read and any group/other write are refused. This is for the
// App PRIVATE KEY only; the token cache and GitLab custody keep their exact-mode checks.
func PrivateKeyModeOK(mode os.FileMode) bool {
	perm := mode.Perm()
	return perm&0o004 == 0 && perm&0o022 == 0
}

// CheckPrivateKeyMode is the fail-closed wrapper over PrivateKeyModeOK.
func CheckPrivateKeyMode(pemPath string, mode os.FileMode) *Error {
	if PrivateKeyModeOK(mode) {
		return nil
	}
	return unverifiable(
		fmt.Sprintf("private key at %s has permissions %04o; must not be readable by others or writable by group/others "+
			"(0600, 0400, 0440 and 0640 are accepted) — run: chmod 600 %s", pemPath, mode.Perm(), pemPath), nil)
}

// --- GitHub API ---------------------------------------------------------------------------

func githubGET(c *http.Client, method, url, jwt string) (int, []byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// resolveInstallID returns the installation id whose account.login matches owner, failing
// closed on no match.
func resolveInstallID(c *http.Client, jwt, owner string) (string, error) {
	status, body, err := githubGET(c, "GET", GitHubAPIBase+"/app/installations", jwt)
	if err != nil {
		if status == 0 {
			return "", fmt.Errorf("GET installations: %w", err)
		}
		return "", fmt.Errorf("read installations response: %w", err)
	}
	if status != 200 {
		return "", fmt.Errorf("installations HTTP %d: %s", status, string(body))
	}
	var installs []InstallationInfo
	if err := json.Unmarshal(body, &installs); err != nil {
		return "", fmt.Errorf("parse installations response: %w", err)
	}
	for _, inst := range installs {
		if strings.EqualFold(inst.Account.Login, owner) {
			return fmt.Sprintf("%d", inst.ID), nil
		}
	}
	return "", fmt.Errorf("no installation found for owner %q", owner)
}

// ExchangeJWT POSTs the signed JWT to the installation access_tokens endpoint. The token
// value is the response's `token` field; it is cached and never printed.
func ExchangeJWT(c *http.Client, jwt, installID string) (*TokenResult, error) {
	if c == nil {
		return nil, errors.New("no HTTP transport wired")
	}
	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", GitHubAPIBase, installID)
	status, body, err := githubGET(c, "POST", url, jwt)
	if err != nil {
		if status == 0 {
			return nil, fmt.Errorf("POST access_tokens: %w", err)
		}
		return nil, fmt.Errorf("read response: %w", err)
	}
	if status == 404 {
		return nil, fmt.Errorf("%w: access_tokens HTTP %d: %s", ErrInstallationGone, status, string(body))
	}
	if status != 201 {
		return nil, fmt.Errorf("access_tokens HTTP %d: %s", status, string(body))
	}
	var result TokenResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("empty token in response")
	}
	return &result, nil
}

// --- the cache and its sidecars ------------------------------------------------------------

// PermsPath is the sidecar recording what GitHub GRANTED the installation the token was
// minted for (#571).
func PermsPath(tokenPath string) string { return tokenPath + ".perms" }

// ExpirySidecarPath records the forge's own expires_at for the cached token, so a reuse
// returns the ACTUAL expiry rather than an assumed one. A cache without it (a legacy cache,
// or a mint whose response carried no parseable expiry) is reported at the conservative
// reuse bound, mtime + ReuseWindow.
func ExpirySidecarPath(tokenPath string) string { return tokenPath + ".expires" }

// cachedExpiry returns the cache's expiry and whether it was RECORDED (true) or is the
// conservative legacy bound (false). A recorded expiry that is already past forces a mint.
func cachedExpiry(tokenPath string, mtime time.Time) (time.Time, bool) {
	b, err := os.ReadFile(ExpirySidecarPath(tokenPath))
	if err == nil {
		if t, perr := time.Parse(time.RFC3339, strings.TrimSpace(string(b))); perr == nil {
			return t, true
		}
	}
	return mtime.Add(ReuseWindow), false
}

// CacheExpiry reports the expiry of the token cache at tokenPath: the recorded mint expiry
// when the `.expires` sidecar holds one, else the conservative legacy bound mtime +
// ReuseWindow. recorded says which. An unreadable cache is an error, never a guessed expiry.
func CacheExpiry(tokenPath string) (expires time.Time, recorded bool, err error) {
	fi, err := os.Lstat(tokenPath)
	if err != nil {
		return time.Time{}, false, err
	}
	expires, recorded = cachedExpiry(tokenPath, fi.ModTime())
	return expires, recorded, nil
}

// mintedExpiry records and returns the minted token's expiry. An unparseable or missing
// expires_at is never replaced by an invented lifetime: no sidecar is written and the
// conservative bound from now is returned, exactly what a later reuse of this cache reports.
func mintedExpiry(tokenPath, expiresAt string, now time.Time) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAt))
	if err != nil {
		_ = os.Remove(ExpirySidecarPath(tokenPath))
		return now.Add(ReuseWindow)
	}
	_ = os.WriteFile(ExpirySidecarPath(tokenPath), []byte(t.UTC().Format(time.RFC3339)+"\n"), 0o600)
	return t
}

// WriteTokenCache publishes token at path (0600) so a concurrent reader sees either the
// previous contents or the new token, never an empty or partial file: temp file in the SAME
// directory, synced, renamed over the path. A symlink at path is replaced, not written
// through. beforeRename, when non-nil, runs between the synced temp write and the rename.
func WriteTokenCache(path, token string, beforeRename func()) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write([]byte(token)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if beforeRename != nil {
		beforeRename()
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp over %s: %w", path, err)
	}
	SyncDir(dir)
	return nil
}

// writePerms records the granted permission map next to the token cache, 0600. Not fatal:
// preflight reads a missing sidecar as could-not-check rather than as a pass.
func writePerms(tokenPath string, perms map[string]string) {
	if len(perms) == 0 {
		return
	}
	keys := make([]string, 0, len(perms))
	for k := range perms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%q:%q", k, perms[k])
	}
	b.WriteString("}")
	_ = os.WriteFile(PermsPath(tokenPath), []byte(b.String()), 0o600)
}

// SyncDir best-effort fsyncs a directory so a rename into it is durable.
func SyncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
