package credential

// readonly.go — the ScopeReadOnly arm of the GitHub provider: one run-scoped, read-only
// installation credential, narrowed AT THE FORGE to one repository and to read permissions,
// written once to a caller-named path and never cached.
//
// It exists for one caller: an admitted verifier run (statusgen verifyrun) that must read an
// acceptance approval from the forge without taking a credential from its environment or
// from the gh CLI's stored login. The run names an output path inside its own private temp
// directory, reads the credential from there for the rows that need it, and revokes it and
// removes the directory when the run ends.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// mintedExpiryNoSidecar parses the forge's expires_at for a credential that has no cache and
// so no .expires sidecar; an unparseable value is reported at the conservative reuse bound.
func mintedExpiryNoSidecar(expiresAt string, now time.Time) time.Time {
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAt)); err == nil {
		return t
	}
	return now.Add(ReuseWindow)
}

// ReadOnlyPermissions is the whole grant a ScopeReadOnly credential asks the forge for: what
// reading a pull request's reviews, an issue's comments and a repository's metadata needs,
// all at "read". The forge's GRANTED map is checked against ReadOnlyGrantOK before anything is
// written; asking for read is not proof of receiving read.
var ReadOnlyPermissions = map[string]string{
	"issues":        "read",
	"metadata":      "read",
	"pull_requests": "read",
}

// ReadOnlyGrantOK reports whether a granted permission map is non-empty and holds nothing but
// "read". An empty map is NOT read-only: it is an unobserved grant, and an unobserved grant is
// could-not-check, never assumed narrow.
func ReadOnlyGrantOK(perms map[string]string) bool {
	if len(perms) == 0 {
		return false
	}
	for _, v := range perms {
		if v != "read" {
			return false
		}
	}
	return true
}

// admitReadOnly is the ScopeReadOnly half of the lower custody boundary.
func admitReadOnly(req GitHubRequest) *Error {
	if strings.TrimSpace(req.Owner) == "" {
		return refused("credential provider: a " + ScopeReadOnly + " credential is narrowed to one repository, so it needs the repository's owner")
	}
	if !cacheNameOK(req.Repository) {
		return refused(fmt.Sprintf("credential provider: repository %q is not a repository name — a %s credential is narrowed to exactly one", req.Repository, ScopeReadOnly))
	}
	if req.TokenPath == "" || !filepath.IsAbs(req.TokenPath) {
		return refused("credential provider: a " + ScopeReadOnly + " credential is written only to an absolute caller-named path (--read-only-out)")
	}
	if req.Fresh || req.PathOnly {
		return refused("credential provider: a " + ScopeReadOnly + " credential is never cached, so --fresh and --ttl do not apply to it")
	}
	return nil
}

// githubReadOnly mints and writes the ScopeReadOnly credential. installID is already resolved;
// jwt is the App JWT signed during resolution, or "" when the id came from a cache.
func githubReadOnly(req GitHubRequest, ad Adapters, installID, jwt string) (Credential, string, bool, error) {
	out := req.TokenPath
	// The directory must already exist and be private to this user: the caller made it for
	// this run. A credential is never written into a directory another principal can list.
	dfi, derr := os.Lstat(filepath.Dir(out))
	if derr != nil {
		return Credential{}, "", false, unverifiable("read-only credential directory: "+filepath.Dir(out), derr)
	}
	if !dfi.IsDir() || dfi.Mode().Perm()&0o077 != 0 {
		return Credential{}, "", false, unverifiable(fmt.Sprintf(
			"read-only credential directory %s must be a private (0700) directory, has %v", filepath.Dir(out), dfi.Mode()), nil)
	}
	for _, p := range []string{out, PermsPath(out)} {
		if _, err := os.Lstat(p); err == nil || !os.IsNotExist(err) {
			return Credential{}, "", false, refused("credential provider: " + p + " already exists — a " + ScopeReadOnly +
				" credential is written once to a fresh path and never over another file")
		}
	}

	if jwt == "" {
		var kerr *Error
		jwt, kerr = signAppJWT(req, ad.Now())
		if kerr != nil {
			return Credential{}, "", false, kerr
		}
	}
	payload, _ := json.Marshal(struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{[]string{req.Repository}, ReadOnlyPermissions})
	result, xerr := exchangeJWTBody(ad.HTTP, jwt, installID, payload)
	if xerr != nil {
		return Credential{}, "", false, unverifiable("exchange JWT for a "+ScopeReadOnly+" installation token", xerr)
	}
	if !ReadOnlyGrantOK(result.Permissions) {
		// The forge answered with a grant wider than read (or did not say what it granted).
		// The credential is revoked at once and never written: a wider grant is not this
		// credential, whatever was asked for.
		revokeErr := RevokeInstallationToken(ad.HTTP, result.Token)
		msg := fmt.Sprintf("the forge granted %s, not read-only — refused and not written", grantString(result.Permissions))
		if revokeErr != nil {
			msg += "; revoking it failed (" + revokeErr.Error() + "), it expires on its own at " + result.ExpiresAt
		}
		return Credential{}, "", false, unverifiable(msg, nil)
	}
	if err := writeExclusive(out, result.Token); err != nil {
		_ = RevokeInstallationToken(ad.HTTP, result.Token)
		return Credential{}, "", false, unverifiable("write read-only credential to "+out, err)
	}
	if err := writeExclusive(PermsPath(out), grantString(result.Permissions)); err != nil {
		_ = os.Remove(out)
		_ = RevokeInstallationToken(ad.HTTP, result.Token)
		return Credential{}, "", false, unverifiable("write read-only credential grant to "+PermsPath(out), err)
	}
	expires := mintedExpiryNoSidecar(result.ExpiresAt, ad.Now())
	detail := fmt.Sprintf("minted run-scoped %s %s credential [app=%s install %s repo %s/%s grant %s] (expires %s)",
		ScopeReadOnly, req.Role, req.AppName, installID, req.Owner, req.Repository, grantString(result.Permissions), result.ExpiresAt)
	return Credential{
		Secret:    Secret{v: result.Token},
		Path:      out,
		ExpiresAt: expires,
		Outcome:   OutcomeMinted,
		InstallID: installID,
	}, detail, false, nil
}

// grantString renders a permission map in the same sorted JSON shape as the .perms sidecar.
func grantString(perms map[string]string) string {
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
	return b.String()
}

// writeExclusive creates path 0600 with O_CREATE|O_EXCL, which fails on ANY existing entry at
// path, a symlink (dangling or not) included: it never replaces, truncates or writes through.
func writeExclusive(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}

// RevokeInstallationToken ends an installation credential before its natural expiry
// (DELETE /installation/token, authenticated by the credential itself). 204 is success.
func RevokeInstallationToken(c *http.Client, secret string) error {
	if c == nil {
		return errors.New("no HTTP transport wired")
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("nothing to revoke")
	}
	status, _, err := githubDo(c, "DELETE", GitHubAPIBase+"/installation/token", secret, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("revoke HTTP %d", status)
	}
	return nil
}
