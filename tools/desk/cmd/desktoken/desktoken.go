package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// validRoles is the fixed set of desk roles. A role's config is parameterised
// by the role name: ~/.config/assay/<role>-app.pem, <ROLE>_APP_ID, etc.
var validRoles = map[string]bool{
	"reviewer":    true,
	"verifier":    true,
	"worker":      true,
	"desk":        true,
	"issue-loop":  true,
	"intake-loop": true,
}

const (
	// cacheMaxAge is the threshold for reusing a cached installation token.
	// GitHub App installation tokens live ~60 min; we reuse if < 50 min old.
	cacheMaxAge = 50 * time.Minute
	// tokenExpiry is the lifetime of the installation token we mint; the JWT
	// that authenticates the mint is short-lived (9 min), but the token itself
	// from GitHub lives ~60 min.
	tokenExpiryMinutes = 60
)

// auditCtx accumulates fields for ONE audit line per invocation.
// finalize is deferred so exactly one line is written.
type auditCtx struct {
	verb       string
	role       string
	installID  string
	appID      string
	detail     string
	argsDigest string
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "desktoken",
		Verb:       a.verb,
		ArgsDigest: a.argsDigest,
		Repo:       "", // token minting is repo-independent
		Result:     result,
		Detail:     detail,
	})
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	if err == nil {
		a.log(deskkit.ResultOK, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	case deskkit.ExitUnverifiable:
		result = deskkit.ResultUnverifiable
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// httpClient is a test hook — production code uses http.DefaultClient.
var httpClient = http.DefaultClient

// --- helpers --------------------------------------------------------------------

// roleEnvPrefix converts a role name to its env-var prefix:
// "issue-loop" → "ISSUE_LOOP", "reviewer" → "REVIEWER".
func roleEnvPrefix(role string) string {
	return strings.ToUpper(strings.ReplaceAll(role, "-", "_"))
}

// roleNames returns the valid roles as a sorted slice for error messages.
func roleNames() []string {
	out := make([]string, 0, len(validRoles))
	for r := range validRoles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// home expands a leading "~/" to the user's home directory. No-op if not ~/.
func home(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return p // will fail downstream with a clear path error
		}
		return filepath.Join(h, p[2:])
	}
	return p
}

// b64url returns the base64-url-encoded, no-padding form of b.
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// envOr returns the env var value for key, or def if unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// installationInfo is the relevant fields from GET /app/installations.
type installationInfo struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
}

// resolveInstallID queries the GitHub App installations endpoint with the
// signed JWT and returns the installation ID whose account.login matches
// owner. Returns an error if no match is found — fails closed on any
// ambiguity.
func resolveInstallID(jwt, owner string) (string, error) {
	url := deskkit.GitHubAPIBase + "/app/installations"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET installations: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read installations response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("installations HTTP %d: %s", resp.StatusCode, string(body))
	}

	var installs []installationInfo
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

// parseOwner extracts the owner from a repo slug (owner/name) or uses the raw
// string as the owner if there's no slash.
func parseOwner(repo string) string {
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[:i]
	}
	return repo
}

// --- key parsing ----------------------------------------------------------------

// parsePrivateKey tries PKCS1 then PKCS8 decoding of a PEM-encoded RSA key.
func parsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
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

// --- JWT signing ----------------------------------------------------------------

// buildJWT constructs an RS256-signed App JWT valid for 9 minutes, with iat
// skewed 60 s into the past for clock drift.
func buildJWT(appID string, now time.Time, key *rsa.PrivateKey) (string, error) {
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

// --- GitHub API exchange --------------------------------------------------------

// tokenResult is the relevant fields from the GitHub API response.
type tokenResult struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	// Permissions is what GitHub actually GRANTED this installation token. It is
	// the only place the grant is observable, and #571 was a scope gap discovered
	// three quarters of the way through a pass — so it is recorded (writePerms)
	// for `deskroster preflight` to check against the role's duties at boot.
	Permissions map[string]string `json:"permissions"`
}

// exchangeJWT POSTs the signed JWT to GitHub's installation access_tokens
// endpoint and returns the result. The token value is the response's `token`
// field; it is written to the cache file and never printed.
func exchangeJWT(jwt, installID string) (*tokenResult, error) {
	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", deskkit.GitHubAPIBase, installID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST access_tokens: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("access_tokens HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result tokenResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("empty token in response")
	}
	return &result, nil
}

// --- TTL reporting --------------------------------------------------------------

// printTTL reports the age of the cached token and returns an error if no
// cache file exists.
func printTTL(tokenPath string) error {
	fi, err := os.Stat(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return deskkit.Unverifiable("no cached token at "+tokenPath, nil)
		}
		return deskkit.Unverifiable("cannot stat cached token", err)
	}
	age := time.Since(fi.ModTime())
	remaining := tokenExpiryMinutes - int(age.Minutes())
	if remaining < 0 {
		remaining = 0
	}
	fmt.Printf("TTL=%dm age=%dm path=%s\n", remaining, int(age.Minutes()), tokenPath)
	return nil
}

// --- flag parsing helper --------------------------------------------------------

// parseInterspersed parses fs allowing flags before OR after positional args.
// Copied from deskwt since it's unexported.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	return positionals, nil
}

// --- key-path resolution (#794) --------------------------------------------------

// provisioningDoc names the walkthrough that records where THIS deployment
// provisions its role pems and apps.env.
//
// It is a DOC REFERENCE, not the directory itself, and that is the settled
// ruling — settled by the tooling, not by argument. #794 asks for the house
// directory to be named; `leaksweep run --tree` refuses it: the literal path is
// a registered house-local token and this file ships in the publication tree, so
// a build carrying it fails the tree sweep. Both constraints are satisfiable at
// once: the tool ships the search-path MECHANISM (deskkit.EnvConfigHome), the
// deployment supplies the VALUE, and the refusal points at the withheld doc that
// records it. The reader still closes the gap in one move; the public tree stays
// clean.
const provisioningDoc = "docs/github-apps-setup.md"

// resolvePEMPath finds the role's App private key.
//
// Order: an explicit <ROLE>_PEM override wins verbatim; otherwise the first
// existing <role>-app.pem across the App-credential search path
// (ASSAY_CONFIG_HOME, then the shipped ~/.config/assay).
//
// It returns BOTH a path and a deferred not-found error. The error is deliberately
// NOT raised here: it is raised at the point the key is actually READ, so the
// order in which desktoken reports its preconditions is unchanged (a missing App
// ID still reports as a missing App ID, not as a missing key). The path returned
// alongside a not-found error is the head-of-search-path candidate, so every
// downstream message still names a concrete file.
//
// The refusal names EVERY directory searched, the knob that adds one, and the
// walkthrough that records where this deployment provisions keys. The #794
// symptom was a bare "private key not found at <one path>" that named none of
// the three, so a fresh-shell mint failure read as a broken tool.
func resolvePEMPath(role, prefix string) (string, error) {
	if override := strings.TrimSpace(os.Getenv(prefix + "_PEM")); override != "" {
		return home(override), nil
	}
	name := role + "-app.pem"
	path, searched, found := deskkit.FindConfigFile(name)
	if found {
		return path, nil
	}
	return path, deskkit.Unverifiable(fmt.Sprintf(
		"private key not found: no %s on the App-credential search path. Searched: %s. If this "+
			"deployment provisions role pems and apps.env elsewhere (its walkthrough records where: %s), "+
			"set %s to that directory, or set %s_PEM to the key path. A fresh shell cannot mint any App "+
			"token until this is closed (#794).",
		name, strings.Join(searched, ", "), provisioningDoc,
		deskkit.EnvConfigHome, prefix), nil)
}

// permsPath is the sidecar recording what GitHub GRANTED the installation the
// token was minted for. The grant is visible ONLY in the access-token response,
// so the minter is the only component that can observe it; recording it here is
// what lets `deskroster preflight` check the App's scopes against the role's
// duties (#571) without a second JWT-signing implementation.
func permsPath(tokenPath string) string { return tokenPath + ".perms" }

// writePerms records the granted permission map next to the token cache, 0600
// (it names an App's capability surface — not a secret, but not world-readable
// either). A failure to record is NOT fatal to the mint: the token is the
// deliverable, and preflight reads a missing sidecar as could-not-check rather
// than as a pass.
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
	_ = os.WriteFile(permsPath(tokenPath), []byte(b.String()), 0o600)
}

// privateKeyModeOK reports whether a private-key file's mode keeps the key away
// from OTHER principals. The rule is expressed as bits, not an exact mode:
//
//   - reject if readable by others (mode & 0o004), or writable by group or
//     others (mode & 0o022);
//   - allow otherwise.
//
// So owner-only 0600/0400 AND group-read 0440/0640 pass; 0644, 0660, 0666,
// 0604 and 0620 fail. Group-read is admitted deliberately: a Kubernetes
// Secret volume is materialised by the kubelet root-owned, and a pod running as
// a non-root user reads it through securityContext.fsGroup — group-read — so
// the key in that pod is necessarily 0440 (root:<fsGroup>). An exact-0600 rule
// is unsatisfiable there (0600/0400 would be owner-root-only and unreadable by
// the pod), and the mint would fail closed on every tick. Group-read does not
// widen exposure beyond the pod's own group; other-read and any group/other
// write are still refused. This checker is for the App PRIVATE KEY only; the
// token cache and the GitLab PAT custody file are written by this tool at 0600
// and keep their exact-mode checks.
func privateKeyModeOK(mode os.FileMode) bool {
	perm := mode.Perm()
	return perm&0o004 == 0 && perm&0o022 == 0
}

// checkPrivateKeyMode is the fail-closed wrapper over privateKeyModeOK: it
// returns the exit-6 refusal naming the rule and the observed mode, or nil.
func checkPrivateKeyMode(pemPath string, mode os.FileMode) error {
	if privateKeyModeOK(mode) {
		return nil
	}
	return deskkit.Unverifiable(
		fmt.Sprintf("private key at %s has permissions %04o; must not be readable by others or writable by group/others "+
			"(0600, 0400, 0440 and 0640 are accepted) — run: chmod 600 %s", pemPath, mode.Perm(), pemPath), nil)
}

// --- main entry point -----------------------------------------------------------

// run is the CLI entry point. It returns an exit code.
func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("desktoken sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its
	// own result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	// `coverage` is a distinct read-only verb (list the repos a role's App
	// installations can see). No role is named "coverage", so the dispatch is
	// unambiguous.
	if args[0] == "coverage" {
		cerr := cmdCoverage(args[1:])
		if cerr != nil {
			fmt.Fprintln(os.Stderr, cerr.Error())
		}
		return deskkit.ExitCodeOf(cerr)
	}

	err := cmdToken(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}

// cmdToken implements the mint-or-reuse logic.
func cmdToken(args []string) (err error) {
	ac := &auditCtx{verb: "mint", argsDigest: deskkit.ArgsDigest(os.Args[1:])}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("desktoken", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	repo := fs.String("repo", "", "repo slug (owner/name) for install auto-pick")
	forge := fs.String("forge", "", "forge backend: empty/github (default) mints a GitHub App installation token; gitlab rotates the role's PAT in place (rotate-on-mint custody)")
	ttl := fs.Bool("ttl", false, "print remaining TTL of cached token (does not mint)")
	fresh := fs.Bool("fresh", false, "delete any cached token and its .perms sidecar before minting — forces a fresh mint after a GitHub-App permission change (the cached token otherwise carries the old grant for up to the ~50-min reuse window)")

	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("bad flags: " + perr.Error())
	}
	if len(positionals) != 1 {
		return deskkit.Refused("exactly one <role> argument required, one of: " + strings.Join(roleNames(), ", "))
	}
	role := positionals[0]
	if !validRoles[role] {
		return deskkit.Refused("unknown role " + role + "; valid: " + strings.Join(roleNames(), ", "))
	}
	ac.role = role

	// Forge dispatch. The default (empty/github) mints a GitHub App installation token
	// below. gitlab takes an entirely different custody path — rotate-on-mint against an
	// existing PAT file — and needs no App PEM or App ID, so it dispatches BEFORE any
	// GitHub-credential resolution.
	switch strings.ToLower(strings.TrimSpace(*forge)) {
	case "", "github":
		// fall through to the GitHub App-token mint path.
	case "gitlab":
		return cmdGitLabRotate(role, ac)
	default:
		return deskkit.Refused("unknown --forge " + *forge + "; valid: github (default), gitlab")
	}

	prefix := roleEnvPrefix(role)

	// Resolve PEM path across the App-credential search path (#794). A not-found
	// error is DEFERRED to the point the key is read (see resolvePEMPath), so the
	// existing precondition-reporting order is unchanged.
	pemPath, pemErr := resolvePEMPath(role, prefix)

	// Resolve App ID: env <ROLE>_APP_ID, else ~/.config/assay/apps.env (no source default —
	// a fresh invocation works with no shell sourcing).
	appID, err := deskkit.AppID(role)
	if err != nil {
		return deskkit.Unverifiable(err.Error(), nil)
	}
	ac.appID = appID

	// Resolve install ID: env override takes priority; otherwise resolve at
	// runtime by signing a JWT and querying GET /app/installations, matching
	// account.login against the repo owner. When --repo is absent we default
	// the owner to "example-org".
	// This is attribution (which App name appears), not authorization (which
	// session is permitted to act as the role) — the caller holds the key and
	// controls the env, so every key is readable by any session of the same OS
	// user. The tool provides audit trail, not access control.
	var installID string
	var prebuiltJWT string

	if override := os.Getenv(prefix + "_INSTALL_ID"); override != "" {
		installID = override
	} else {
		// Resolve install ID at runtime: build JWT, query GitHub
		// /app/installations, match account.login against repo owner.
		// Default owner to example-org when --repo is absent.
		owner := "example-org"
		if *repo != "" {
			owner = parseOwner(*repo)
		}

		// Must read PEM, sign JWT before we know the install ID.
		// The key is READ here, so a deferred not-found from resolvePEMPath is
		// raised here — naming every directory searched (#794).
		if pemErr != nil {
			return pemErr
		}
		fi, perr := os.Stat(pemPath)
		if perr != nil {
			if os.IsNotExist(perr) {
				return deskkit.Unverifiable("private key not found at "+pemPath, perr)
			}
			return deskkit.Unverifiable("cannot stat private key at "+pemPath, perr)
		}
		if merr := checkPrivateKeyMode(pemPath, fi.Mode()); merr != nil {
			return merr
		}

		keyPEM, rerr := os.ReadFile(pemPath)
		if rerr != nil {
			return deskkit.Unverifiable("cannot read private key at "+pemPath, rerr)
		}
		key, kerr := parsePrivateKey(keyPEM)
		if kerr != nil {
			return deskkit.Unverifiable("parse private key from "+pemPath, kerr)
		}

		now := time.Now()
		jwt, jerr := buildJWT(appID, now, key)
		if jerr != nil {
			return deskkit.Unverifiable("sign JWT", jerr)
		}
		prebuiltJWT = jwt

		resolvedID, rerr := resolveInstallID(jwt, owner)
		if rerr != nil {
			return deskkit.Unverifiable("resolve installation for owner "+owner, rerr)
		}
		installID = resolvedID
	}
	ac.installID = installID

	// Per-install token cache path: always suffixed with the install ID so
	// different Apps' installations on the same account do not share a file.
	// The reviewer App's example-org install (100000002) also gets the suffix, and
	// each App manages its own cache independently via the same
	// shared conventions.
	// The cache is WRITTEN to the head of the App-credential search path, so a
	// deployment that points ASSAY_CONFIG_HOME at its provisioning directory
	// reads its key and writes its cache in the SAME place. #794's closing line:
	// "the key-lookup dir, the cache dir, the apps.env dir and the provisioning
	// dir must be the same one."
	defaultToken := deskkit.ConfigHomeWritePath(fmt.Sprintf("%s-token-%s", role, installID))
	tokenPath := home(envOr(prefix+"_TOKEN", defaultToken))

	// --ttl: report cached token age and exit (no mint).
	if *ttl {
		return printTTL(tokenPath)
	}

	// --fresh: drop any cached token AND its .perms sidecar before the reuse check, so a mint
	// that follows a GitHub-App permission change cannot be short-circuited by the up-to-50-min
	// cache-reuse window below, and so the grant sidecar (writePerms) is rewritten with the NEW
	// scope. Without this, "re-mint" after a permission change is a NO-OP for the rest of the
	// reuse window: the cached token is returned and its stale .perms is what `deskroster
	// preflight` reads for the app-scopes check (#571). Idempotent — a missing file is fine.
	if *fresh {
		if rmErr := os.Remove(tokenPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return deskkit.Unverifiable("cannot remove cached token for --fresh: "+tokenPath, rmErr)
		}
		if rmErr := os.Remove(permsPath(tokenPath)); rmErr != nil && !os.IsNotExist(rmErr) {
			return deskkit.Unverifiable("cannot remove token .perms sidecar for --fresh: "+permsPath(tokenPath), rmErr)
		}
	}

	// Reuse cached token if < 50 min old.
	if fi, serr := os.Stat(tokenPath); serr == nil {
		if fi.Mode().Perm() != 0o600 {
			return deskkit.Unverifiable(
				fmt.Sprintf("token cache at %s has permissions %o; must be 0600", tokenPath, fi.Mode().Perm()), nil)
		}
		age := time.Since(fi.ModTime())
		if age < cacheMaxAge {
			// Output only the token file path — never the token value.
			fmt.Println(tokenPath)
			ac.detail = fmt.Sprintf("reused cached %s token [install %s] (%dm old)", role, installID, int(age.Minutes()))
			return nil
		}
	} else if !os.IsNotExist(serr) {
		return deskkit.Unverifiable("cannot stat token cache: "+tokenPath, serr)
	}

	// --- Mint new token ---
	var jwt string
	if prebuiltJWT != "" {
		jwt = prebuiltJWT
	} else {
		// Read the App private key. Check file permissions — see checkPrivateKeyMode.
		// The key is READ here, so a deferred not-found from resolvePEMPath is
		// raised here — naming every directory searched (#794).
		if pemErr != nil {
			return pemErr
		}
		fi, perr := os.Stat(pemPath)
		if perr != nil {
			if os.IsNotExist(perr) {
				return deskkit.Unverifiable("private key not found at "+pemPath, perr)
			}
			return deskkit.Unverifiable("cannot stat private key at "+pemPath, perr)
		}
		if merr := checkPrivateKeyMode(pemPath, fi.Mode()); merr != nil {
			return merr
		}

		keyPEM, rerr := os.ReadFile(pemPath)
		if rerr != nil {
			return deskkit.Unverifiable("cannot read private key at "+pemPath, rerr)
		}

		key, kerr := parsePrivateKey(keyPEM)
		if kerr != nil {
			return deskkit.Unverifiable("parse private key from "+pemPath, kerr)
		}

		// Build and sign the App JWT.
		now := time.Now()
		var jerr error
		jwt, jerr = buildJWT(appID, now, key)
		if jerr != nil {
			return deskkit.Unverifiable("sign JWT", jerr)
		}
	}

	// Exchange for an installation access token.
	result, xerr := exchangeJWT(jwt, installID)
	if xerr != nil {
		return deskkit.Unverifiable("exchange JWT for installation token", xerr)
	}

	// Write the token to the cache file (0600).
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return deskkit.Unverifiable("mkdir token dir", err)
	}
	if err := os.Chmod(filepath.Dir(tokenPath), 0o700); err != nil {
		return deskkit.Unverifiable("chmod token dir", err)
	}
	if err := os.WriteFile(tokenPath, []byte(result.Token), 0o600); err != nil {
		return deskkit.Unverifiable("write token to "+tokenPath, err)
	}
	if err := os.Chmod(tokenPath, 0o600); err != nil {
		return deskkit.Unverifiable("chmod token cache", err)
	}
	writePerms(tokenPath, result.Permissions)

	// Output only the token file path — never the token value.
	fmt.Println(tokenPath)
	ac.detail = fmt.Sprintf("minted new %s token [install %s] (expires %s)", role, installID, result.ExpiresAt)
	return nil
}
