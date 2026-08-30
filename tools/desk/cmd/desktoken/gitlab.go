package main

// gitlab.go — GitLab token custody: rotate-on-mint + expiry backstop.
//
// GitLab personal access tokens are long-lived, so naive handling would be a custody
// downgrade from the GitHub path's short-lived minted tokens — which the security-parity
// ruling forbids. This path closes the gap by a DIFFERENT mechanism than GitHub's:
//
//   - Rotate-on-mint. Every mint calls POST /personal_access_tokens/self/rotate, which
//     returns a NEW token and atomically invalidates the caller's current one. At most ONE
//     credential per role is ever valid, and a captured token dies at the next mint — parity
//     with GitHub's short-lived tokens by the single-valid-credential property rather than by
//     TTL shape.
//   - Expiry backstop. Rotation sets the new token's expiry per the GROUP token-lifetime
//     policy (7 days RECOMMENDED; set on the group, not here). An idle fleet that never mints
//     again leaves no live credential once the backstop elapses. That backstop fails for a
//     different reason (time) in a different component (the GitLab server) than rotation does,
//     so it is a genuine second layer under the rotation control, not a duplicate of it.
//
// File custody is unchanged from the GitHub path: the role's token lives 0600 in
// gitlab-<role>.token on the App-credential search path, and this command prints the PATH
// only — never the token value, never to env or argv.
//
// Roles are single-window by convention; a second concurrent invocation for the same role
// rotates the first's token out from under it BY DESIGN (the first's token is invalidated the
// moment the second rotates). Parallel actors get per-actor service accounts, never a shared
// token. This is stated in the command help rather than defended with a lock.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabAPIBase returns the explicitly-configured GitLab REST v4 base URL and whether it was
// set. GitLab Enterprise is self-hosted, so there is no universal host the way GitHub has
// api.github.com; the deployment supplies its own via GITLAB_API_BASE (e.g.
// https://gitlab.example.com/api/v4; gitlab.com's SaaS base is https://gitlab.com/api/v4).
//
// There is deliberately NO default. Rotation transmits the role's live PAT in a PRIVATE-TOKEN
// header, so silently falling back to gitlab.com would send a self-hosted deployment's
// credential to a public SaaS endpoint the moment it forgot to configure its host. The base is
// therefore REQUIRED and a bare invocation refuses rather than probing a default target
// (no-default-probe convention).
//
// Read at call time (not a package-level var) so a test — or a shell — that sets the env var
// after process start still takes effect.
func gitlabAPIBase() (string, bool) {
	v := strings.TrimSpace(os.Getenv("GITLAB_API_BASE"))
	return v, v != ""
}

// gitlabTokenFileName is the per-role custody file: gitlab-<role>.token, resolved across the
// same App-credential search path the GitHub path uses.
func gitlabTokenFileName(role string) string { return "gitlab-" + role + ".token" }

// gitlabRotateResult is the subset of the rotation response this command consumes. The token
// value is written to the custody file and NEVER printed; expires_at is reported (a date, not
// a secret) so the audit line records the backstop the group policy applied.
type gitlabRotateResult struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Active    bool   `json:"active"`
}

// rotateGitLabToken POSTs to the self-rotation endpoint authenticated with the CURRENT token
// and returns the new one. The endpoint atomically invalidates the current token, so the
// caller must persist result.Token before discarding the current value.
//
// No expires_at is sent: the new token's lifetime is set by the group token-lifetime policy
// (the expiry backstop). Enforcing a policy here would duplicate group configuration and is
// deliberately out of scope (see the brief).
//
// The current token is never placed in an error string — an error must be safe to print.
func rotateGitLabToken(base, current string) (*gitlabRotateResult, error) {
	url := strings.TrimRight(base, "/") + "/personal_access_tokens/self/rotate"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create rotate request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", current)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST rotate: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rotate response: %w", err)
	}
	// GitLab returns 200 for a successful self-rotation; accept 201 defensively.
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		// The error body is a GitLab error message (e.g. {"message":"401 Unauthorized"}),
		// never a token — safe to surface for diagnosis.
		return nil, fmt.Errorf("rotate HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result gitlabRotateResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse rotate response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("rotation response carried no token")
	}
	return &result, nil
}

// writeVerifyGitLabToken persists token to path 0600 and reads it back to confirm the bytes
// landed. A write that reports success but does not durably persist the new token is a
// lockout waiting to happen, because rotation has already invalidated the old one; the
// read-back turns that into an observed failure at mint time instead.
//
// The write is temp-file-then-rename in the same directory, so the credential swap is atomic:
// a crash or an error mid-write never leaves a torn file that holds neither the old nor a
// whole new token — path either still holds the old bytes or holds the whole new token, never
// a fragment. When the directory is not writable, creating the temp file fails here, and that
// surfaces as the lockout the caller reports (rather than a silently torn custody file).
func writeVerifyGitLabToken(path, token string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gitlab-token-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup of the temp if we bail before the rename lands.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write([]byte(token)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	// os.CreateTemp already makes the file 0600; assert it before it becomes the credential.
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp over %s: %w", path, err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read-back %s: %w", path, err)
	}
	if string(got) != token {
		return fmt.Errorf("read-back mismatch: persisted token does not match the rotated value")
	}
	return nil
}

// cmdGitLabRotate implements `desktoken --forge gitlab <role>`: read the current token file,
// rotate via the API, write-verify the new value 0600 in place, and print the path.
//
// Refusal behaviors mirror the GitHub key path: a missing custody file, a non-regular-file
// custody, and a wrong file mode each refuse (exit 6) with a named remedy, and no token-shaped
// value ever reaches stdout, stderr, env, or argv.
func cmdGitLabRotate(role string, ac *auditCtx) error {
	ac.verb = "rotate"
	ac.role = role

	name := gitlabTokenFileName(role)

	// Locate the existing custody file across the App-credential search path.
	path, searched, found := deskkit.FindConfigFile(name)
	if !found {
		return deskkit.Unverifiable(fmt.Sprintf(
			"gitlab token file not found: no %s on the App-credential search path. Searched: %s. "+
				"Provision the role's initial PAT (0600) into one of those directories via a group owner "+
				"(the walkthrough records which: %s), or set %s to that directory.",
			name, strings.Join(searched, ", "), provisioningDoc, deskkit.EnvConfigHome), nil)
	}

	fi, serr := os.Stat(path)
	if serr != nil {
		return deskkit.Unverifiable("cannot stat gitlab token file at "+path, serr)
	}
	// Non-file custody (a directory, a symlink target that is not a regular file, a socket)
	// refuses — custody is a 0600 regular file, mirroring the GitHub key contract.
	if !fi.Mode().IsRegular() {
		return deskkit.Unverifiable(fmt.Sprintf(
			"gitlab custody at %s is not a regular file (mode %s); token custody requires a 0600 regular "+
				"file — re-provision the role's PAT there via a group owner", path, fi.Mode()), nil)
	}
	if fi.Mode().Perm() != 0o600 {
		return deskkit.Unverifiable(fmt.Sprintf(
			"gitlab token file at %s has permissions %o; must be 0600 — run: chmod 600 %s",
			path, fi.Mode().Perm(), path), nil)
	}

	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read gitlab token file at "+path, rerr)
	}
	current := strings.TrimSpace(string(raw))
	if current == "" {
		return deskkit.Unverifiable(
			"gitlab token file at "+path+" is empty — re-provision the role's PAT (0600) via a group owner", nil)
	}

	// Resolve the network target BEFORE first contact. With no default (see gitlabAPIBase), an
	// unset GITLAB_API_BASE refuses here — the role's live PAT is never transmitted to a guessed
	// host — rather than silently POSTing the credential to gitlab.com's SaaS endpoint.
	base, ok := gitlabAPIBase()
	if !ok {
		return deskkit.Unverifiable(
			"GITLAB_API_BASE is not set — refusing to transmit the role's PAT to a default target. Set it to "+
				"your deployment's REST v4 base (self-hosted: https://gitlab.example.com/api/v4; gitlab.com SaaS: "+
				"https://gitlab.com/api/v4) and re-run.", nil)
	}
	// Announce the target before contact (no-default-probe convention): the operator sees exactly
	// which host is about to receive the credential. The base is a URL, never a token.
	fmt.Fprintf(os.Stderr, "gitlab: rotating role %s token via %s\n", role, base)

	// Rotate. The endpoint invalidates `current` atomically and returns the new token; from
	// here until write-verify succeeds, the old token is dead and the new one lives only in
	// memory, so the new value MUST be persisted before returning.
	result, xerr := rotateGitLabToken(base, current)
	if xerr != nil {
		return deskkit.Unverifiable("rotate gitlab token for role "+role, xerr)
	}

	if werr := writeVerifyGitLabToken(path, result.Token); werr != nil {
		// LOCKOUT: rotation already invalidated the old token and the new one could not be
		// persisted. Print the recovery path — never the token value.
		return deskkit.Unverifiable(fmt.Sprintf(
			"LOCKOUT: rotation succeeded but persisting the new token to %s failed (%v). The previous token "+
				"is now invalid and the new one could not be saved. Recover by re-issuing the role's PAT via a "+
				"group owner (Group > Settings > Access Tokens) and writing it 0600 to %s. The new token value "+
				"is NOT printed.", path, werr, path), nil)
	}

	// Success: print the PATH only.
	fmt.Println(path)
	exp := result.ExpiresAt
	if exp == "" {
		exp = "per group policy"
	}
	ac.detail = fmt.Sprintf("rotated gitlab %s token in place (expires %s)", role, exp)
	return nil
}
