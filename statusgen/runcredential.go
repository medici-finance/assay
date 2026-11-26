package main

// runcredential.go — the verifier's own run-scoped, read-only forge credential
// for an admitted verifyrun (#6746).
//
// An admitted run reads the forge (an acceptance approval, through
// --acceptance-forge) only with a credential it mints itself for the verifier
// role. It never uses one from the environment (GH_TOKEN, GITHUB_TOKEN) or the
// forge CLI's stored login, in the run's own process or in a statusgen a Verify
// row starts. Every way the credential can be missing is a could-not-check:
// no minter, a role the window may not act as, a grant wider than read, a mint
// error or a credential file that fails the checks below. None of them is a
// pass.
//
// Who mints: this process, by running `desktoken verifier --repo <owner/name>
// --read-only-out <path>`. The role is fixed. No flag or variable selects
// another, and desktoken refuses a role its window's loop does not act as.
// statusgen never passes --cross-role.
// Scope: the forge narrows it to issues, metadata and pull_requests at read,
// on the attested repository only. desktoken refuses a wider grant, and this
// file checks the granted map again before the credential is used.
// Lifetime: minted at most once per run, on the first read that needs it. It
// is never cached and lives in a private directory made for this run. When
// the run ends it is revoked at the forge and the directory removed.
// GitHub's own expiry (an hour) bounds a revocation that fails.
// What a row sees: only a row whose command passes --acceptance-forge gets
// runCredentialEnv, set to the credential file's PATH. The value is never in
// any row's environment. Every other row gets nothing new. The value is never
// printed, logged or written to a witness.
// That row scoping is hygiene, not containment: rowReadsForge matches the
// row's own text, and the file sits in a directory the invoking user owns, so
// a later row run as that user can read the value once it is minted. The
// bound is the credential itself: read-only, one repository, revoked at run
// end.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// runCredentialEnv names the variable that carries the run credential's file
// path to a Verify row that reads the forge. It holds a path, never the value.
const runCredentialEnv = "ASSAY_VERIFYRUN_READ_CREDENTIAL"

// runCredentialRole is the one role an admitted run mints for.
const runCredentialRole = "verifier"

// runCredentialMintTimeout bounds the desktoken call.
const runCredentialMintTimeout = 2 * time.Minute

// runReadCredentialMint mints the credential to out for repo and returns
// what the minter printed on stdout. It is a package variable so the test
// binary can make sure that no test ever reaches a real minter. TestMain
// replaces it, and a test that wants the real exec path restores
// execRunCredentialMint itself, with a fake desktoken on PATH.
var runReadCredentialMint = execRunCredentialMint

// currentRunCredential is the admitted run's credential while the run is in
// progress, for this process's own forge reads (rulingForgeClient). Only
// runVerifyrun sets it. Nothing reads it from the environment.
var currentRunCredential atomic.Pointer[runReadCredential]

// runReadCredential is one admitted run's credential. It is minted lazily,
// at most once.
type runReadCredential struct {
	repo   string // owner/name from the attestation; "" when unresolvable
	why    string // why repo is unresolvable
	once   sync.Once
	dir    string
	path   string
	secret string
	err    error
}

// attestationRepoRe pulls owner/name out of a Verification-Attestation line.
var attestationRepoRe = regexp.MustCompile(`^Verification-Attestation: ([A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+)#[0-9]+ `)

// newRunReadCredential prepares, but does not mint, the credential for the
// run that attestation admits.
func newRunReadCredential(attestation string) *runReadCredential {
	m := attestationRepoRe.FindStringSubmatch(attestation)
	if m == nil {
		return &runReadCredential{why: "the attestation names no owner/name repository to narrow it to"}
	}
	return &runReadCredential{repo: m[1]}
}

// activate makes the checked host run's credential available to the forge
// reader. Its caller owns this scope and must release it before returning.
func (c *runReadCredential) activate(stderr io.Writer) func() {
	previous := currentRunCredential.Swap(c)
	state := verifyrunAdmission.Swap(verifyrunAdmitted)
	return func() {
		currentRunCredential.Store(previous)
		verifyrunAdmission.Store(state)
		c.close(stderr)
	}
}

// ensure mints the credential on first use and returns its file path and
// value. The error names the reason. It never contains the value.
func (c *runReadCredential) ensure() (string, string, error) {
	c.once.Do(func() {
		c.path, c.secret, c.err = c.mint()
	})
	return c.path, c.secret, c.err
}

func (c *runReadCredential) mint() (string, string, error) {
	if c.repo == "" {
		return "", "", errors.New(c.why)
	}
	dir, err := os.MkdirTemp("", "verifyrun-read-")
	if err != nil {
		return "", "", fmt.Errorf("cannot make the run's private credential directory: %v", err)
	}
	c.dir = dir
	out := filepath.Join(dir, "read-credential")
	printed, err := runReadCredentialMint(c.repo, out)
	if err != nil {
		return "", "", fmt.Errorf("the %s mint failed: %v", runCredentialRole, err)
	}
	if strings.TrimSpace(printed) != out {
		revokeUnusedRunCredential(out)
		return "", "", fmt.Errorf("the %s mint printed a path other than the one asked for", runCredentialRole)
	}
	secret, err := readRunCredentialFile(out)
	if err != nil {
		revokeUnusedRunCredential(out)
		return "", "", err
	}
	return out, secret, nil
}

// revokeUnusedRunCredential revokes, best effort, a credential the minter
// wrote to out that this run then refused (a wrong printed path, or a file or
// grant that failed its checks). It is never used for a read: the value is
// read only to end it at the forge, so a refused credential does not live on
// until its own expiry. A missing, non-regular or garbled file revokes
// nothing. The run's directory is removed at run end as usual.
func revokeUnusedRunCredential(out string) {
	fi, err := os.Lstat(out)
	if err != nil || !fi.Mode().IsRegular() {
		return
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return
	}
	secret := strings.TrimSpace(string(raw))
	if secret == "" || strings.ContainsAny(secret, " \t\r\n") {
		return
	}
	_ = revokeRunCredential(secret)
}

// close revokes a minted credential and removes the run's directory. A
// failed revocation is reported without the value. The credential then
// expires on its own.
func (c *runReadCredential) close(w io.Writer) {
	if c == nil {
		return
	}
	if c.secret != "" {
		if err := revokeRunCredential(c.secret); err != nil && w != nil {
			_, _ = fmt.Fprintf(w, "statusgen verifyrun: the run's read-only credential could not be revoked (%v); it expires on its own within the hour\n", err)
		}
		c.secret = ""
	}
	if c.dir != "" {
		_ = os.RemoveAll(c.dir)
	}
}

// execRunCredentialMint runs the verifier's minter. Its environment is this
// process's own minus the forge credential variables. The minter does not
// need them, and this keeps the mint from depending on one.
//
// The executable is the literal name "desktoken", resolved on PATH by
// exec.CommandContext itself, so the launch inventory (internal/gitlaunch)
// can see from the source that this construction never names Git.
func execRunCredentialMint(repo, out string) (string, error) {
	if _, err := exec.LookPath("desktoken"); err != nil {
		return "", errors.New("no desktoken on PATH to mint it with")
	}
	ctx, cancel := context.WithTimeout(context.Background(), runCredentialMintTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "desktoken", runCredentialRole, "--repo", repo, "--read-only-out", out)
	cmd.Env = runCredentialMintEnv(os.Environ())
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if len(detail) > 300 {
			detail = detail[:300] + "..."
		}
		if detail != "" {
			return "", fmt.Errorf("%v: %s", err, detail)
		}
		return "", err
	}
	return string(stdout), nil
}

// runCredentialMintEnv is environ without the forge credential variables.
func runCredentialMintEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		if key, _, _ := strings.Cut(kv, "="); !admittedRowForgeToken(key) && !strings.EqualFold(key, runCredentialEnv) {
			env = append(env, kv)
		}
	}
	return env
}

// readRunCredentialFile reads and checks a run credential written by the
// minter. The file must be a regular file, not a link. On POSIX it must be
// 0600. It must be non-empty. Its .perms sidecar must be a non-empty grant
// holding nothing but "read". Any failure is an error.
func readRunCredentialFile(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("the run credential path is not absolute")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("the run credential file is unreadable: %v", err)
	}
	if !fi.Mode().IsRegular() {
		return "", errors.New("the run credential file is not a regular file")
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("the run credential file has mode %04o, not 0600", fi.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("the run credential file is unreadable: %v", err)
	}
	secret := strings.TrimSpace(string(raw))
	if secret == "" || strings.ContainsAny(secret, " \t\r\n") {
		return "", errors.New("the run credential file holds no single credential")
	}
	grant, err := os.ReadFile(path + ".perms")
	if err != nil {
		return "", errors.New("the run credential has no recorded grant, so its scope is unknown")
	}
	var perms map[string]string
	if err := json.Unmarshal(grant, &perms); err != nil || len(perms) == 0 {
		return "", errors.New("the run credential's recorded grant is unreadable, so its scope is unknown")
	}
	for k, v := range perms {
		if v != "read" {
			return "", fmt.Errorf("the run credential's grant is wider than read-only (%s=%s)", k, v)
		}
	}
	return secret, nil
}

// revokeRunCredential ends the credential at the forge (DELETE
// /installation/token, authenticated by the credential itself).
func revokeRunCredential(secret string) error {
	req, err := http.NewRequest(http.MethodDelete, githubAPIBase+"/installation/token", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return errors.New("the forge was unreachable")
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// runCredentialDoer is the transport of a forge client an admitted context
// built with no usable run credential. It makes no request and answers every
// request with a could-not-check, which the resolver reports as unresolvable.
type runCredentialDoer struct{ why string }

func (d runCredentialDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("could-not-check: " + d.why)
}

// admittedForgeClient is the forge client for an admitted context, or nil
// outside one. Inside one it carries only the verifier's run-scoped
// read-only credential. Neither a forge credential variable nor the stored
// login is consulted. Two contexts are admitted:
//
//   - this process is an admitted verifyrun: the run's own credential, minted
//     on first use;
//   - this process's environment is an admitted Verify row's
//     (admittedRowEnvMarked): the credential file the run handed this row
//     through runCredentialEnv. A row the run gave no credential has none.
func admittedForgeClient() *ghClient {
	secret, why, admitted := admittedReadCredential()
	switch {
	case !admitted:
		return nil
	case why != "":
		return &ghClient{doer: runCredentialDoer{why}, base: githubAPIBase}
	}
	return newGHClient(secret)
}

// admittedReadCredential is the one source of a forge credential in an
// admitted context, for both forge read paths: the REST client
// (admittedForgeClient) and every forge CLI launch (ghGuard). admitted is
// false outside an admitted context, and the caller then keeps its existing
// behaviour. Inside one, it returns the run credential's value, or why there
// is none.
func admittedReadCredential() (secret, why string, admitted bool) {
	const only = "the forge is read only with the verifier's run-scoped read-only credential, and "
	switch {
	case verifyrunAdmission.Load() == verifyrunAdmitted:
		c := currentRunCredential.Load()
		if c == nil {
			return "", "inside an admitted verifyrun " + only + "this run has none", true
		}
		_, secret, err := c.ensure()
		if err != nil {
			return "", "inside an admitted verifyrun " + only + "it is unavailable: " + err.Error(), true
		}
		return secret, "", true
	case admittedRowEnvMarked():
		path := os.Getenv(runCredentialEnv)
		if path == "" {
			return "", "inside an admitted Verify row " + only + "the run handed this row none", true
		}
		secret, err := readRunCredentialFile(path)
		if err != nil {
			return "", "inside an admitted Verify row " + only + "it is unavailable: " + err.Error(), true
		}
		return secret, "", true
	}
	return "", "", false
}

// rowReadsForge reports whether a Verify row's command asks for the forge
// read that needs the run credential.
func rowReadsForge(command string) bool {
	return strings.Contains(command, "--acceptance-forge")
}
