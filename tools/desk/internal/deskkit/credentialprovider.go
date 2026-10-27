package deskkit

// credentialprovider.go — the deskkit side of the role-credential provider
// (internal/credential): the configuration SNAPSHOT a request is built from, the ADAPTERS
// that carry this package's controls into the provider, the error mapping back to desk-tools
// exit classes, and the in-process GitHub minter RoleTokenForOwner runs.
//
// WHY THE PROVIDER DOES NOT IMPORT THIS PACKAGE. deskkit owns the in-process caller, so the
// provider importing deskkit would be a cycle. Instead every control the desktoken CLI
// applied — the kill-switch guard, the audit ledger, the custody-file link and owner-only
// checks, the advisory lock — is handed to the provider here, as the SAME deskkit function
// the CLI called. The CLI builds its request and its adapters from these helpers too, so the
// two callers cannot present different inputs or apply different checks.
//
// WHAT CHANGED FOR A DESK VERB. Before the provider, RoleTokenForOwner forked the
// `desktoken` binary found first on PATH and scraped its stderr for the refusal. It now runs
// the provider as a function call: no process, no PATH lookup, the same on-disk cache and the
// same audit row. A verb on a host with no desktoken on PATH mints exactly as one with it.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
)

// AppProvisioningDoc names the walkthrough that records where a deployment provisions its
// role pems, apps.env and GitLab custody files. It is a DOC REFERENCE, not the directory:
// the tool ships the search-path mechanism (EnvConfigHome), the deployment supplies the
// value, and a refusal points at the doc that records it (#794).
const AppProvisioningDoc = "docs/github-apps-setup.md"

// CredentialPaths is the App-credential search path snapshot every provider request carries:
// the directories ConfigHomeDirs searches, the knob that prepends one, and the doc above.
func CredentialPaths() credential.Paths {
	return credential.Paths{Dirs: ConfigHomeDirs(), Knob: EnvConfigHome, Doc: AppProvisioningDoc}
}

// ResolveAppPEMPath finds the App private key for an App-NAME (the role→App binding's stem;
// `<role>-app` when the role is unbound).
//
// Order: an explicit <PREFIX>_PEM override wins verbatim; otherwise the first existing
// <app-name>.pem across the App-credential search path.
//
// It returns BOTH a path and a DEFERRED not-found error, raised by the provider only at the
// point the key is READ, so the order in which preconditions are reported is unchanged (a
// missing App ID still reports as a missing App ID). The refusal names EVERY directory
// searched, the knob that adds one, and the walkthrough — the #794 symptom was a bare
// "private key not found at <one path>" that named none of the three.
func ResolveAppPEMPath(appName, prefix string) (string, error) {
	if override := strings.TrimSpace(os.Getenv(prefix + "_PEM")); override != "" {
		return expandHome(override), nil
	}
	name := appName + ".pem"
	path, searched, found := FindConfigFile(name)
	if found {
		return path, nil
	}
	return path, Unverifiable(fmt.Sprintf(
		"private key not found: no %s on the App-credential search path. Searched: %s. If this "+
			"deployment provisions role pems and apps.env elsewhere (its walkthrough records where: %s), "+
			"set %s to that directory, or set %s_PEM to the key path. A fresh shell cannot mint any App "+
			"token until this is closed (#794).",
		name, strings.Join(searched, ", "), AppProvisioningDoc,
		EnvConfigHome, prefix), nil)
}

// GitHubCredentialRequest builds the configuration snapshot for one GitHub App credential
// request for role: the bound App, its key path (with any deferred not-found), its App ID,
// the explicit <PREFIX>_INSTALL_ID and <PREFIX>_TOKEN overrides, and the search path. The
// caller sets Owner (and Fresh / PathOnly).
//
// The provider reads none of this itself — it is resolved here, once, so the CLI and the
// in-process lookup present byte-identical requests for the same environment.
func GitHubCredentialRequest(role string) (credential.GitHubRequest, error) {
	if !credential.KnownRole(role) {
		return credential.GitHubRequest{}, Refused("unknown role " + role + "; valid: " +
			strings.Join(credential.RoleNames(), ", "))
	}
	appName := AppBinding(role)
	prefix := AppEnvPrefix(appName)
	pemPath, pemErr := ResolveAppPEMPath(appName, prefix)
	appID, err := AppIDForApp(appName)
	if err != nil {
		return credential.GitHubRequest{}, Unverifiable(err.Error(), nil)
	}
	req := credential.GitHubRequest{
		Host:      credential.GitHubHost,
		Role:      role,
		AppName:   appName,
		AppID:     appID,
		InstallID: strings.TrimSpace(os.Getenv(prefix + "_INSTALL_ID")),
		PEMPath:   pemPath,
		PEMErr:    pemErr,
		Paths:     CredentialPaths(),
		Scope:     credential.ScopeInstallation,
	}
	if v := os.Getenv(prefix + "_TOKEN"); v != "" {
		req.TokenPath = expandHome(v)
	}
	return req, nil
}

// CredentialAdapters wires this package's controls into the provider: the live Guard, the
// custody link and owner-only checks, the advisory lock, the wall clock and stderr. audit
// receives the operation's one record; client is the transport every forge call uses.
func CredentialAdapters(audit func(credential.Record), client *http.Client) credential.Adapters {
	return credential.Adapters{
		Guard: Guard,
		Audit: audit,
		Custody: credential.Custody{
			Lstat: func(path string, sameDirLink bool) (string, os.FileInfo, error) {
				policy := CustodyNoLinks
				if sameDirLink {
					policy = CustodySameDirLink
				}
				return LstatCustody(path, policy)
			},
			IsLinkRefusal: func(err error) bool {
				var le *CustodyLinkError
				return errors.As(err, &le)
			},
			OwnerOnly: VerifyCustodyOwnerOnly,
		},
		Lock: credential.Locker{
			TryLock: TryLockExclusive,
			Unlock:  UnlockFile,
			IsBusy:  func(err error) bool { return errors.Is(err, ErrLockBusy) },
		},
		HTTP:   client,
		Now:    time.Now,
		Stderr: os.Stderr,
	}
}

// CredentialError maps a provider failure to the desk-tools error it has always been: a
// refused request is exit 5, an unverifiable state exit 6, with the provider's message
// verbatim. Anything that is already a desk-tools error (the guard's own refusal) passes
// through unchanged.
func CredentialError(err error) error {
	if err == nil {
		return nil
	}
	var ce *credential.Error
	if !errors.As(err, &ce) {
		return err
	}
	if ce.Class == credential.ClassRefused {
		if ce.Err != nil {
			return RefusedWithCause(ce.Msg, ce.Err)
		}
		return Refused(ce.Msg)
	}
	return Unverifiable(ce.Msg, ce.Err)
}

// CredentialResult is the audit result a provider record's error maps to.
func CredentialResult(err error) string {
	if err == nil {
		return ResultOK
	}
	switch ExitCodeOf(CredentialError(err)) {
	case ExitDisabled:
		return ResultDisabled
	case ExitRefused:
		return ResultRefused
	default:
		return ResultUnverifiable
	}
}

// credentialHTTP is the transport the in-process GitHub minter sends through. A package var
// so a test can point it at a fake forge; production uses the default client.
var credentialHTTP = http.DefaultClient

// SetCredentialHTTP installs c as the in-process minter's transport and returns a function
// that restores the previous one. It is a TEST seam in the SetRoleTokenMinter mould: the
// desktoken CLI's parity test drives the CLI and the in-process lookup against ONE fake
// forge, and the in-process transport lives here. Production installs nothing; a nil c
// leaves the transport unchanged.
func SetCredentialHTTP(c *http.Client) (restore func()) {
	prev := credentialHTTP
	if c != nil {
		credentialHTTP = c
	}
	return func() { credentialHTTP = prev }
}

// providerMint is the in-process GitHub App mint behind tokenMinter: the provider, run as a
// function call. It writes the SAME audit row the desktoken CLI writes for a mint (tool
// desktoken, the CLI's argument digest), none for a cache reuse, and returns the cache PATH —
// the contract tokenMinter has always had. stderr is always empty: the refusal travels as a
// typed error, not as text to scrape.
//
// ORDER, AS THE CLI. The guard runs first (the CLI's own top-of-run Guard), so a refusal to
// build the request — an unknown role, no App ID — is recorded only while desk tools are
// live, and records one row exactly as the CLI's refusal does. The provider then runs the
// guard again for the operation itself.
func providerMint(role, owner string) (path, stderr string, err error) {
	if gerr := Guard(); gerr != nil {
		return "", "", gerr // the guard recorded its own refusal
	}
	digest := ArgsDigest([]string{role, "--repo", owner})
	audit := func(r credential.Record) {
		detail := r.Detail
		if r.Err != nil {
			detail = r.Err.Error()
		}
		_ = Log(Entry{Tool: "desktoken", Verb: r.Verb, ArgsDigest: digest, Result: CredentialResult(r.Err), Detail: detail})
	}
	req, rerr := GitHubCredentialRequest(role)
	if rerr != nil {
		audit(credential.Record{Verb: "mint", Err: rerr})
		return "", "", rerr
	}
	req.Owner = OwnerOf(owner)
	cred, gerr := credential.GitHub(req, CredentialAdapters(audit, credentialHTTP))
	if gerr != nil {
		return "", "", CredentialError(gerr)
	}
	return cred.Path, "", nil
}
