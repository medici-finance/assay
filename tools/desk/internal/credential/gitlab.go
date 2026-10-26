package credential

// gitlab.go — GitLab token custody: rotate-on-mint + expiry backstop, and the read-only
// custody lookup.
//
// GitLab personal access tokens are long-lived, so this arm closes the custody gap by a
// DIFFERENT mechanism than GitHub's short-lived mints:
//
//   - Rotate-on-mint. A rotation calls POST /personal_access_tokens/self/rotate, which returns
//     a NEW token and atomically invalidates the caller's current one — at most ONE credential
//     per role is ever valid, and a captured token dies at the next rotation.
//   - Expiry backstop. The group token-lifetime policy sets the new token's expiry, so an idle
//     fleet leaves no live credential once it elapses — a second layer that fails for a
//     different reason (time) in a different component (the GitLab server).
//
// ROTATION IS EXPLICIT. A rotation happens only when the request says Rotate; a read-only
// lookup (Rotate false) verifies custody and returns the path and value without any network
// contact and never rotates, so a read-only verb cannot spend the role's credential.
//
// CONCURRENCY. Rotation is destructive by construction, so two overlapping rotations for one
// role race for one credential and can leave custody holding a dead value. A per-role advisory
// lock on gitlab-<role>.token.lock spans read-current → rotate → write-verify; a lock that
// cannot be taken is a REFUSAL before the second rotate is in flight. The lock is a separate
// file because the write lands by rename, which would orphan a lock held on the custody inode.
//
// CUSTODY LAYOUT. The custody path may be a same-directory SYMLINK to the provisioned file
// (the documented layout). A rotation writes THROUGH the link — it renames onto the resolved
// target and leaves the link in place — and the read-back goes through the custody path, so a
// rotation that broke the link fails at mint time rather than at the next read. A link whose
// target is ANOTHER ROLE's custody file is refused: following it would hand one role's PAT out
// as another's.
//
// SELF-CHECK. After the rotated value is persisted the arm performs ONE live, read-only GET
// with the new token (gitlabSelfCheckPath); only a 200 returns the path as good. Anything else
// names the status the new token got and whether the forge still accepted the previous one —
// lag versus lockout — with no retry loop and no sleep (#1142).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitLabRequest is the explicit snapshot a GitLab custody operation runs against.
type GitLabRequest struct {
	// Role is the desk role (KnownRole).
	Role string
	// Paths is the App-credential search path the custody file is found on.
	Paths Paths
	// APIBase is the deployment's REST v4 base. Required for a rotation; there is NO default,
	// because rotation transmits the role's live PAT.
	APIBase string
	// Rotate asks for a rotation. False is the read-only custody lookup.
	Rotate bool
	// LockWait bounds how long a rotation waits for a peer's rotation to finish.
	LockWait time.Duration
}

// GitLabRotateResult is the subset of the rotation response this arm consumes. The token
// value is written to custody and NEVER printed; expires_at is reported (a date, not a secret).
type GitLabRotateResult struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Active    bool   `json:"active"`
}

// GitLabSelfCheckPath is the endpoint the post-rotation self-check reads: the current-user
// read, the cheapest authenticated GET GitLab has, needing no project coordinate.
const GitLabSelfCheckPath = "/user"

// GitLabTokenFileName is the per-role custody file: gitlab-<role>.token.
func GitLabTokenFileName(role string) string { return "gitlab-" + role + ".token" }

// GitLabRotateLockPath is the per-role serialisation file beside the custody path.
func GitLabRotateLockPath(tokenPath string) string { return tokenPath + ".lock" }

// GitLab verifies the role's GitLab custody and either returns it unchanged (read-only) or
// rotates it in place, serialized per role, and self-checks the successor.
//
// RECORDS. One per operation, under the verb "custody-read" or "rotate" — the verb records
// what was actually DONE, so a read-only lookup never appears as a rotation in the trail.
// semantic: S-role-credential
func GitLab(req GitLabRequest, ad Adapters) (Credential, error) {
	verb := "rotate"
	if !req.Rotate {
		verb = "custody-read"
	}
	if e := ad.check(req.Rotate); e != nil {
		ad.audit(verb, e, e.Error())
		return Credential{}, e
	}
	if gerr := ad.Guard(); gerr != nil {
		return Credential{}, gerr
	}
	cred, detail, err := gitlabOperation(req, ad)
	if ferr := ad.finish(verb, err, detail, false); ferr != nil {
		return Credential{}, ferr
	}
	return cred, nil
}

func gitlabOperation(req GitLabRequest, ad Adapters) (Credential, string, error) {
	role := req.Role
	if !KnownRole(role) {
		return Credential{}, "", refuseRole(role)
	}

	// A GitLab release-runner credential is a PIPELINE TRIGGER TOKEN, not a PAT: it has no
	// self-rotate endpoint. Rotation is an operator act in the project's CI/CD settings; the
	// read-only lookup still verifies and returns custody.
	if role == "release-runner" && req.Rotate {
		return Credential{}, "", refused("the GitLab release-runner credential is a pipeline trigger token, which has no " +
			"self-rotate endpoint — rotate it in the project's CI/CD settings, re-provision " +
			GitLabTokenFileName(role) + " (0600), and use --no-rotate to verify custody")
	}

	name := GitLabTokenFileName(role)
	path, found := req.Paths.find(name)
	if !found {
		return Credential{}, "", unverifiable(fmt.Sprintf(
			"gitlab token file not found: no %s on the App-credential search path. Searched: %s. "+
				"Provision the role's initial PAT (0600) into one of those directories via a group owner "+
				"(the walkthrough records which: %s), or set %s to that directory.",
			name, strings.Join(req.Paths.Dirs, ", "), req.Paths.Doc, req.Paths.Knob), nil)
	}

	// Lstat, not Stat: the documented same-directory link is followed; any other link at the
	// custody path is refused rather than checked and read through.
	target, fi, serr := ad.Custody.Lstat(path, true)
	if serr != nil {
		if ad.Custody.IsLinkRefusal(serr) {
			return Credential{}, "", unverifiable(serr.Error(), nil)
		}
		return Credential{}, "", unverifiable("cannot stat gitlab token file at "+path, serr)
	}
	if e := refuseForeignCustody(role, path, target); e != nil {
		return Credential{}, "", e
	}
	// Non-file custody refuses — custody is a 0600 regular file.
	if !fi.Mode().IsRegular() {
		return Credential{}, "", unverifiable(fmt.Sprintf(
			"gitlab custody at %s is not a regular file (mode %s); token custody requires a 0600 regular "+
				"file — re-provision the role's PAT there via a group owner", target, fi.Mode()), nil)
	}
	// Owner-only custody, behind the OS boundary (#667).
	if err := ad.Custody.OwnerOnly(target, fi); err != nil {
		return Credential{}, "", unverifiable(err.Error(), nil)
	}

	// READ-ONLY LOOKUP. Custody is verified; read the value and stop. No lock is taken and no
	// network is contacted: a concurrent rotation lands its value by rename, so a reader sees
	// the whole old token or the whole new one, never a fragment.
	if !req.Rotate {
		cur, cerr := readGitLabCustody(target)
		if cerr != nil {
			return Credential{}, "", cerr
		}
		return Credential{Secret: Secret{v: cur}, Path: path, Outcome: OutcomeCustodyRead},
			fmt.Sprintf("read gitlab %s custody path (--no-rotate: no rotation performed)", role), nil
	}

	// SERIALISE THE WHOLE DESTRUCTIVE SEQUENCE: read only once the lock is held, so the value
	// presented to the endpoint is the one the previous holder persisted.
	lock, lerr := acquireGitLabRotateLock(ad.Lock, req.LockWait, role, path)
	if lerr != nil {
		return Credential{}, "", lerr
	}
	defer lock.release()

	current, cerr := readGitLabCustody(target)
	if cerr != nil {
		return Credential{}, "", cerr
	}

	// Resolve the network target BEFORE first contact: an unset base refuses here — the PAT is
	// never transmitted to a guessed host.
	base := strings.TrimSpace(req.APIBase)
	if base == "" {
		return Credential{}, "", unverifiable(
			"GITLAB_API_BASE is not set — refusing to transmit the role's PAT to a default target. Set it to "+
				"your deployment's REST v4 base (self-hosted: https://gitlab.example.com/api/v4; gitlab.com SaaS: "+
				"https://gitlab.com/api/v4) and re-run.", nil)
	}
	// Announce the target before contact: the base is a URL, never a token.
	fmt.Fprintf(ad.stderr(), "gitlab: rotating role %s token via %s\n", role, base)

	result, xerr := RotateGitLabToken(ad.HTTP, base, current)
	if xerr != nil {
		return Credential{}, "", unverifiable("rotate gitlab token for role "+role, xerr)
	}

	if werr := writeVerifyGitLabToken(path, result.Token); werr != nil {
		// LOCKOUT: rotation already invalidated the old token and the new one could not be
		// persisted. Name the recovery path — never the token value.
		return Credential{}, "", unverifiable(fmt.Sprintf(
			"LOCKOUT: rotation succeeded but persisting the new token to %s failed (%v). The previous token "+
				"is now invalid and the new one could not be saved. Recover by re-issuing the role's PAT via a "+
				"group owner (Group > Settings > Access Tokens) and writing it 0600 to %s. The new token value "+
				"is NOT printed.", path, werr, path), nil)
	}

	// SELF-CHECK. One read-only GET, no retry, no sleep; only a 200 returns the path as good.
	status, scerr := GitLabSelfCheck(ad.HTTP, base, result.Token)
	if scerr != nil {
		return Credential{}, "", unverifiable(fmt.Sprintf(
			"post-rotation self-check for role %s could not be completed: the rotated token is persisted "+
				"at %s but its acceptance by the forge is unverified, so that path is NOT returned as good. "+
				"Re-run the mint once the forge is reachable.", role, path), scerr)
	}
	if status != 200 {
		return Credential{}, "", gitlabSelfCheckFailure(ad.HTTP, role, base, path, current, status)
	}

	exp := result.ExpiresAt
	cred := Credential{Secret: Secret{v: result.Token}, Path: path, Outcome: OutcomeRotated}
	if t, perr := time.Parse("2006-01-02", exp); perr == nil {
		cred.ExpiresAt = t
	} else if t, perr := time.Parse(time.RFC3339, exp); perr == nil {
		cred.ExpiresAt = t
	}
	if exp == "" {
		exp = "per group policy"
	}
	return cred, fmt.Sprintf("rotated gitlab %s token in place (expires %s; self-check GET %s %d)",
		role, exp, GitLabSelfCheckPath, status), nil
}

// refuseForeignCustody refuses a custody link whose resolved target is ANOTHER role's GitLab
// custody file. The same-directory link layout points gitlab-<role>.token at the provisioned
// file; pointing it at gitlab-<other>.token instead would read, and on rotation spend, a
// different role's PAT under this role's name.
func refuseForeignCustody(role, path, target string) *Error {
	if target == path {
		return nil
	}
	base := filepath.Base(target)
	if strings.HasPrefix(base, "gitlab-") && strings.HasSuffix(base, ".token") && base != GitLabTokenFileName(role) {
		return unverifiable(fmt.Sprintf(
			"gitlab custody for role %s at %s links to %s, another role's custody file — refusing to read "+
				"or rotate one role's PAT under another role's name. Point the link at this role's own "+
				"provisioned token file.", role, path, base), nil)
	}
	return nil
}

// --- the rotation lock ---------------------------------------------------------------------

type gitlabRotateLock struct {
	f  *os.File
	lk Locker
}

func (l *gitlabRotateLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = l.lk.Unlock(l.f)
	_ = l.f.Close()
}

// acquireGitLabRotateLock takes the per-role rotation lock, waiting up to wait for a peer to
// finish. It FAILS CLOSED in every non-acquisition case: an unserialised rotation is precisely
// the operation that can leave the role with no live credential.
func acquireGitLabRotateLock(lk Locker, wait time.Duration, role, tokenPath string) (*gitlabRotateLock, *Error) {
	lockPath := GitLabRotateLockPath(tokenPath)
	f, oerr := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if oerr != nil {
		return nil, unverifiable(fmt.Sprintf(
			"cannot open the per-role gitlab rotation lock at %s (%v) — REFUSING to rotate unserialised. "+
				"Two overlapping rotations for role %s invalidate each other's token and can leave %s holding "+
				"a dead value with no live successor. Make the directory writable by this user and re-run.",
			lockPath, oerr, role, tokenPath), nil)
	}
	deadline := time.Now().Add(wait)
	for {
		lerr := lk.TryLock(f)
		if lerr == nil {
			return &gitlabRotateLock{f: f, lk: lk}, nil
		}
		if !lk.IsBusy(lerr) {
			_ = f.Close()
			return nil, unverifiable(fmt.Sprintf(
				"cannot acquire the per-role gitlab rotation lock at %s (%v) — REFUSING to rotate unserialised "+
					"for role %s. A rotation that is not serialised can invalidate a peer's token and leave %s "+
					"holding a dead value.", lockPath, lerr, role, tokenPath), nil)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			// The advisory lock is released by the OS when its holder exits, so a leftover
			// lock FILE holds nothing; waiting this long means a peer is genuinely in flight.
			return nil, unverifiable(fmt.Sprintf(
				"another gitlab rotation for role %s has held %s for >%s — REFUSING to start a second, "+
					"overlapping rotation: the two would invalidate each other's token and could leave %s "+
					"holding a dead value with no live successor. Re-run once the other mint finishes, and "+
					"issue mints for one role SEQUENTIALLY rather than in parallel. (The lock is released by "+
					"the OS when its holder exits, so a leftover lock file is never itself the cause — do not "+
					"delete it.) If the role's PAT is ALREADY dead (desk verbs failing 401 invalid_token), "+
					"self-rotation cannot recover it: a group owner must re-issue the role's PAT "+
					"(Group > Settings > Access Tokens) and write it 0600 to %s.",
				role, lockPath, wait, tokenPath, tokenPath), nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// --- GitLab API ----------------------------------------------------------------------------

// noRedirect copies c and refuses every redirect (SEC-1): both GitLab calls send
// PRIVATE-TOKEN, a custom header net/http forwards across hosts on a followed redirect.
func noRedirect(c *http.Client) *http.Client {
	cp := http.Client{}
	if c != nil {
		cp = *c
	}
	cp.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &cp
}

// RotateGitLabToken POSTs to the self-rotation endpoint authenticated with the CURRENT token
// and returns the new one. The current token is never placed in an error string.
func RotateGitLabToken(c *http.Client, base, current string) (*GitLabRotateResult, error) {
	url := strings.TrimRight(base, "/") + "/personal_access_tokens/self/rotate"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create rotate request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", current)
	req.Header.Set("Accept", "application/json")

	resp, err := noRedirect(c).Do(req)
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
		return nil, fmt.Errorf("rotate HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result GitLabRotateResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse rotate response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("rotation response carried no token")
	}
	return &result, nil
}

// GitLabSelfCheck performs ONE read-only GET of GitLabSelfCheckPath with token and returns
// the status. It never retries, sleeps or reads the body; a transport failure is an error.
func GitLabSelfCheck(c *http.Client, base, token string) (int, error) {
	url := strings.TrimRight(base, "/") + GitLabSelfCheckPath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("create self-check request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Accept", "application/json")

	resp, err := noRedirect(c).Do(req)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %w", GitLabSelfCheckPath, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// gitlabSelfCheckFailure builds the verdict for a self-check the NEW token did not pass,
// probing the PREVIOUS token once, read-only, so the line names which token the forge
// accepted. It never restores the previous token to custody. Neither value reaches the text.
func gitlabSelfCheckFailure(c *http.Client, role, base, path, previous string, newStatus int) *Error {
	prev := ""
	prevStatus, perr := GitLabSelfCheck(c, base, previous)
	switch {
	case perr != nil:
		prev = "could not be checked (" + perr.Error() + ")"
	case prevStatus == 200:
		prev = "still ACCEPTED (HTTP 200)"
	default:
		prev = fmt.Sprintf("rejected (HTTP %d)", prevStatus)
	}
	return unverifiable(fmt.Sprintf(
		"post-rotation self-check FAILED for role %s: GET %s with the NEW token answered HTTP %d; "+
			"the PREVIOUS token was %s. The rotation endpoint reported success and the new token is "+
			"persisted at %s, but the forge did not accept it on a live read, so that path is NOT "+
			"returned as good. If the previous token was still accepted this is the forge propagating "+
			"the rotation — re-run the mint once and it rotates from the persisted value. If NEITHER "+
			"token is accepted the role is locked out: a group owner must re-issue the role's PAT "+
			"(Group > Settings > Access Tokens) and write it 0600 to %s.",
		role, GitLabSelfCheckPath, newStatus, prev, path, path), nil)
}

// --- custody write ---------------------------------------------------------------------------

// GitLabCustodyWriteTarget resolves WHICH file a rotation's bytes must land in so the custody
// PATH keeps its provisioned shape: the path itself, or a symlink's fully resolved target.
// A link that cannot be resolved is an ERROR, never a silent write at the link path.
func GitLabCustodyWriteTarget(path string) (target string, linked bool, err error) {
	fi, lerr := os.Lstat(path)
	if lerr != nil {
		return path, false, nil
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return path, false, nil
	}
	resolved, rerr := filepath.EvalSymlinks(path)
	if rerr != nil {
		return "", true, fmt.Errorf("resolve custody symlink %s: %w", path, rerr)
	}
	return resolved, true, nil
}

// writeVerifyGitLabToken persists token 0600 THROUGH path (temp file, fsync, rename onto the
// resolved target, directory fsync) and reads it back through the custody path. A link must
// still be a link afterwards (a backstop under the target resolution; no mutation entry).
func writeVerifyGitLabToken(path, token string) error {
	target, linked, terr := GitLabCustodyWriteTarget(path)
	if terr != nil {
		return terr
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".gitlab-token-*.tmp")
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
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("rename temp over %s: %w", target, err)
	}
	SyncDir(dir)

	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read-back %s: %w", path, err)
	}
	if string(got) != token {
		return fmt.Errorf("read-back mismatch: persisted token does not match the rotated value")
	}
	if linked {
		fi, lerr := os.Lstat(path)
		if lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("custody symlink at %s did not survive the rotation (it is now a regular file); "+
				"the rotated token must be written THROUGH the link, never over it", path)
		}
	}
	return nil
}

// readGitLabCustody reads the role's current PAT from an already-mode-verified custody file,
// refusing an unreadable or empty one. The value never reaches an error string.
func readGitLabCustody(path string) (string, *Error) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", unverifiable("cannot read gitlab token file at "+path, rerr)
	}
	current := strings.TrimSpace(string(raw))
	if current == "" {
		return "", unverifiable(
			"gitlab token file at "+path+" is empty — re-provision the role's PAT (0600) via a group owner", nil)
	}
	return current, nil
}
