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
// CONCURRENCY. Rotate-on-mint is destructive by construction: the endpoint invalidates the
// presented token as it issues the successor. Two rotations that overlap therefore race for
// one credential — the loser presents a token the winner already killed, gets 401
// invalid_token, and the custody file can be left holding a value that is already dead, with
// no live successor anywhere. Self-rotation cannot recover from that: reaching the endpoint
// at all requires a live token. Only a group owner re-issuing the PAT does.
//
// The original design accepted that race, on the reading that roles are single-window and
// parallel ACTORS should hold per-actor service accounts. That reading does not cover the
// case that actually bites: ONE window issuing several tool calls in parallel, each of which
// mints. Two independent layers close it, each failing for a different reason in a different
// component:
//
//   - SERIALISATION (this file). A per-role advisory lock on gitlab-<role>.token.lock spans
//     read-current → rotate → write-verify, so overlapping mints for one role run in
//     sequence instead of racing. Each then reads the live token its predecessor persisted
//     and the custody file always holds a credential that works. A lock that cannot be taken
//     is a REFUSAL before the second rotate is in flight — never an unserialised rotation.
//   - NOT MINTING WHERE NOTHING IS SPENT (callers). A read-only or dry-run verb asks for the
//     custody PATH with --no-rotate and performs no rotation at all, so a parallel sweep of
//     reads no longer spends N rotations on work that writes nothing. That removes most of
//     the contention the lock then has to serialise.
//
// The lock is a separate file, never the custody file itself: writeVerifyGitLabToken lands the
// rotated value by rename, so a lock taken on the custody inode would be orphaned by the very
// write it is meant to guard. The lock file is created once and never renamed, so its inode is
// stable for the life of the custody directory.
//
// CUSTODY LAYOUT. The custody path may be a SYMLINK to the file the provisioning step actually
// wrote — that is the documented layout (docs/adopting-assay-gitlab.md §2 links
// gitlab-<role>.token at the provisioned <prefix>-<role>-bot.token), not an exotic
// deployment. A rotation therefore writes THROUGH the link: it renames onto the link's
// resolved target and leaves the link itself in place. Renaming onto the LINK PATH instead
// would replace the link with a regular file, which has two consequences that only show up
// later:
//
//   - the provisioned file the link pointed at keeps the PRE-ROTATION value forever, so any
//     step that re-creates the documented link (an idempotent re-run of the provisioning
//     step, a re-issue script) silently re-points custody at a token the rotation already
//     invalidated — the next API read then presents a dead credential and 401s, with nothing
//     in the rotate path itself having failed;
//   - the layout the operator provisioned is gone, so the next `ln -s` is not a no-op.
//
// Writing through the link keeps ONE file holding the live credential, whichever name is used
// to reach it. The read-back verification then reads back through the CUSTODY PATH rather than
// the target, so a rotation that broke the link fails at mint time instead of at the next read.
//
// SELF-CHECK. The rotation endpoint's 200 says the forge ISSUED the successor; it does not say
// the forge will ACCEPT it on the very next request. In the field (#1142) a verb's first API
// read right after its own rotation returned 401 while the on-disk token answered 200 to a
// direct probe seconds later — the shape of server-side propagation lag after self-rotation,
// which the mint path used to assume away. So after the rotated value is persisted, and before
// the path is printed, the command performs ONE live, read-only GET with the new token
// (gitlabSelfCheckPath). A 200 is the only result that returns the path as good. Anything else
// exits non-zero naming the endpoint, the status the NEW token got, and whether the forge still
// accepted the PREVIOUS token — one further read-only GET, so the operator can tell lag (old
// still accepted) from a lockout (neither accepted) without guessing. There is no retry loop
// and no sleep: a re-run is one rotation, costs the caller nothing it did not already spend on
// the failed first read, and keeps the mint path free of a timing heuristic tuned to one
// instance.

import (
	"encoding/json"
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

// gitlabRotateLockPath is the per-role serialisation file that guards one custody file's
// read→rotate→write-verify sequence: the custody path plus a .lock suffix, so it sits beside
// the credential it guards and is per-ROLE by construction (two roles never contend).
//
// It is deliberately NOT the custody file. writeVerifyGitLabToken lands the new token by
// rename, which puts a different inode at the custody path; a lock held on the old inode is
// released into a file nothing will ever open again, and the next caller locks the NEW inode
// without contention — a lock that silently stops locking at exactly the moment it matters.
// A dedicated file is created once and never replaced, so every caller contends on one inode.
func gitlabRotateLockPath(tokenPath string) string { return tokenPath + ".lock" }

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

// gitlabRotateLock is an acquired per-role rotation lock. release() is idempotent-safe for a
// single deferred call and never masks the command's own error.
type gitlabRotateLock struct{ f *os.File }

func (l *gitlabRotateLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = deskkit.UnlockFile(l.f)
	_ = l.f.Close()
}

// acquireGitLabRotateLock takes the per-role rotation lock, waiting up to gitlabRotateLockWait
// for a peer to finish.
//
// It FAILS CLOSED in every non-acquisition case. There is no "proceed anyway" branch: an
// unserialised rotation is precisely the operation that can leave the role with no live
// credential, and a refusal costs a retry whereas a lost race costs a group owner's
// intervention. That asymmetry is why the lock cannot be advisory-in-practice.
//
// Both refusals name the recovery path, because the operator reading them may be reading them
// AFTER the lockout rather than before it.
func acquireGitLabRotateLock(role, tokenPath string) (*gitlabRotateLock, error) {
	lockPath := gitlabRotateLockPath(tokenPath)
	f, oerr := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if oerr != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"cannot open the per-role gitlab rotation lock at %s (%v) — REFUSING to rotate unserialised. "+
				"Two overlapping rotations for role %s invalidate each other's token and can leave %s holding "+
				"a dead value with no live successor. Make the directory writable by this user and re-run.",
			lockPath, oerr, role, tokenPath), nil)
	}
	deadline := time.Now().Add(gitlabRotateLockWait)
	for {
		lerr := deskkit.TryLockExclusive(f)
		if lerr == nil {
			return &gitlabRotateLock{f: f}, nil
		}
		if !errors.Is(lerr, deskkit.ErrLockBusy) {
			_ = f.Close()
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"cannot acquire the per-role gitlab rotation lock at %s (%v) — REFUSING to rotate unserialised "+
					"for role %s. A rotation that is not serialised can invalidate a peer's token and leave %s "+
					"holding a dead value.", lockPath, lerr, role, tokenPath), nil)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			// Note what a leftover lock FILE does and does not mean: the advisory lock is
			// released by the OS when the holding process exits, so a file left on disk by a
			// crashed mint holds nothing. Waiting this long means a peer rotation is genuinely
			// still in flight — deleting the file would not help and would only remove the
			// serialisation from under the peer.
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"another gitlab rotation for role %s has held %s for >%s — REFUSING to start a second, "+
					"overlapping rotation: the two would invalidate each other's token and could leave %s "+
					"holding a dead value with no live successor. Re-run once the other mint finishes, and "+
					"issue mints for one role SEQUENTIALLY rather than in parallel. (The lock is released by "+
					"the OS when its holder exits, so a leftover lock file is never itself the cause — do not "+
					"delete it.) If the role's PAT is ALREADY dead (desk verbs failing 401 invalid_token), "+
					"self-rotation cannot recover it: a group owner must re-issue the role's PAT "+
					"(Group > Settings > Access Tokens) and write it 0600 to %s.",
				role, lockPath, gitlabRotateLockWait, tokenPath, tokenPath), nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

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

// gitlabSelfCheckPath is the endpoint the post-rotation self-check reads: the current-user
// read, the cheapest authenticated GET GitLab has and the one the pilot used to prove a rotated
// token live (and a captured one dead). It is on every tier, it needs no project coordinate
// (a role is minted before any --repo is known), and its response is a small account record —
// never a token.
const gitlabSelfCheckPath = "/user"

// gitlabSelfCheck performs ONE read-only GET of gitlabSelfCheckPath authenticated with token
// and returns the HTTP status the forge answered. It never retries, never sleeps and never
// reads the body: the caller decides on the status alone. A transport failure is returned as
// an error (the check could not be made — not a rejection, not an acceptance).
//
// The token is never placed in an error string — an error from this function is printed.
func gitlabSelfCheck(base, token string) (int, error) {
	url := strings.TrimRight(base, "/") + gitlabSelfCheckPath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("create self-check request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %w", gitlabSelfCheckPath, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// gitlabSelfCheckFailure builds the non-zero verdict for a self-check the NEW token did not
// pass. It probes the PREVIOUS token once, read-only, on the same endpoint so the line names
// which token the forge accepted: previous accepted + new rejected is the propagation-lag
// shape (#1142) and a re-run is the remedy; neither accepted is a lockout only a group owner
// can recover. It never restores the previous token to custody — the rotation endpoint has
// reported it invalidated, and a custody that holds a token the forge is about to reject is
// worse than one that holds the successor the forge has not yet caught up with.
//
// Neither token value reaches the message.
func gitlabSelfCheckFailure(role, base, path, previous string, newStatus int) error {
	prev := ""
	prevStatus, perr := gitlabSelfCheck(base, previous)
	switch {
	case perr != nil:
		prev = "could not be checked (" + perr.Error() + ")"
	case prevStatus == 200:
		prev = "still ACCEPTED (HTTP 200)"
	default:
		prev = fmt.Sprintf("rejected (HTTP %d)", prevStatus)
	}
	return deskkit.Unverifiable(fmt.Sprintf(
		"post-rotation self-check FAILED for role %s: GET %s with the NEW token answered HTTP %d; "+
			"the PREVIOUS token was %s. The rotation endpoint reported success and the new token is "+
			"persisted at %s, but the forge did not accept it on a live read, so that path is NOT "+
			"returned as good. If the previous token was still accepted this is the forge propagating "+
			"the rotation — re-run the mint once and it rotates from the persisted value. If NEITHER "+
			"token is accepted the role is locked out: a group owner must re-issue the role's PAT "+
			"(Group > Settings > Access Tokens) and write it 0600 to %s.",
		role, gitlabSelfCheckPath, newStatus, prev, path, path), nil)
}

// gitlabCustodyWriteTarget resolves WHICH file a rotation's bytes must land in so that the
// custody PATH keeps the shape the deployment provisioned. For an ordinary regular-file
// custody the answer is the path itself. For a SYMLINK custody — the documented layout, see
// the file header — it is the link's fully resolved target, so the rename swaps the file the
// link points AT and the link survives the rotation.
//
// linked reports whether path was a symlink, so the caller can assert afterwards that it still
// is one. A link that cannot be resolved is an ERROR, never a silent fall-back to writing at
// the link path: resolving is how the rotation learns where the live credential actually
// lives, and guessing would strand the rotated value somewhere nothing reads.
func gitlabCustodyWriteTarget(path string) (target string, linked bool, err error) {
	fi, lerr := os.Lstat(path)
	if lerr != nil {
		// No entry to inspect. cmdGitLabRotate has already stat'd and refused on a missing or
		// non-regular custody, so this is only reachable for a path that vanished between the
		// two calls; write where we were told and let the rename surface the failure.
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

// writeVerifyGitLabToken persists token 0600 through path and reads it back to confirm the
// bytes landed. A write that reports success but does not durably persist the new token is a
// lockout waiting to happen, because rotation has already invalidated the old one; the
// read-back turns that into an observed failure at mint time instead.
//
// The write is temp-file-then-rename in the TARGET's own directory, so the credential swap is
// atomic: a crash or an error mid-write never leaves a torn file that holds neither the old nor
// a whole new token — the custody path either still resolves to the old bytes or to the whole
// new token, never a fragment. When the directory is not writable, creating the temp file fails
// here, and that surfaces as the lockout the caller reports (rather than a silently torn
// custody file).
//
// The target is resolved rather than assumed to be path: on a symlinked custody layout the
// rename must land on the link's target so the link survives (file header, CUSTODY LAYOUT).
//
// It is also DURABLE before it returns: the temp file is fsync'd before the rename and the
// containing directory afterwards. The caller prints the custody path the moment this returns
// and the verb that invoked it immediately re-reads that path to obtain the credential it will
// present, so "written" has to mean written — not merely queued behind a rename the operating
// system has not committed. The directory fsync is best-effort: not every platform lets a
// directory handle be synced, and a platform that refuses it is not a reason to fail a rotation
// whose bytes are already on disk.
func writeVerifyGitLabToken(path, token string) error {
	target, linked, terr := gitlabCustodyWriteTarget(path)
	if terr != nil {
		return terr
	}
	dir := filepath.Dir(target)
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
	syncDir(dir)

	// Read back through the CUSTODY PATH, not the target: on a linked layout that proves both
	// that the bytes landed AND that the path the verbs read still resolves to them.
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read-back %s: %w", path, err)
	}
	if string(got) != token {
		return fmt.Errorf("read-back mismatch: persisted token does not match the rotated value")
	}
	// A custody link must still be a link. If it is not, the rotation has just converted the
	// layout the deployment provisioned into a regular file, and the file the link pointed at
	// is now stranded holding the invalidated token — the condition that makes a later re-link
	// hand a dead credential to the next read.
	//
	// This is a BACKSTOP under the target resolution above, not a second implementation of it:
	// with the resolver correct nothing reaches it, which is why it carries no mutation entry
	// (a mutant that disables it survives by construction). It is kept because the cost of a
	// silently converted layout is a lockout an operator only discovers at the next read.
	if linked {
		fi, lerr := os.Lstat(path)
		if lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("custody symlink at %s did not survive the rotation (it is now a regular file); "+
				"the rotated token must be written THROUGH the link, never over it", path)
		}
	}
	return nil
}

// syncDir best-effort fsyncs a directory so a rename into it is durable. Failures are ignored:
// syncing a directory handle is not supported everywhere (notably Windows), and a platform that
// refuses it must not turn a completed rotation into a reported lockout.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// readGitLabCustody reads the role's current PAT from an already-mode-verified custody file.
// It is shared by the rotating and the --no-rotate paths so both refuse identically on an
// unreadable or empty custody file — a read-only verb that accepted an empty credential would
// report a usable path for a role that has none.
//
// The token VALUE never reaches an error string: an error from this function is printed.
func readGitLabCustody(path string) (string, error) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", deskkit.Unverifiable("cannot read gitlab token file at "+path, rerr)
	}
	current := strings.TrimSpace(string(raw))
	if current == "" {
		return "", deskkit.Unverifiable(
			"gitlab token file at "+path+" is empty — re-provision the role's PAT (0600) via a group owner", nil)
	}
	return current, nil
}

// cmdGitLabRotate implements `desktoken --forge gitlab <role>`: read the current token file,
// rotate via the API, write-verify the new value 0600 in place, self-check it with one live
// read, and print the path.
//
// With rotate=false (`--no-rotate`) it performs the SAME custody checks and prints the same
// path, but makes no network contact and leaves the credential untouched. That is the shape a
// read-only or dry-run verb asks for: it needs to know WHERE the role's credential is, not to
// spend a rotation it will not use. Skipping the rotation does not weaken the rotate-on-mint
// property — at most one credential per role is still ever valid, and it still dies at the
// next rotation — it only stops verbs that write nothing from driving that rotation.
//
// Refusal behaviors mirror the GitHub key path: a missing custody file, a non-regular-file
// custody, and a wrong file mode each refuse (exit 6) with a named remedy, and no token-shaped
// value ever reaches stdout, stderr, env, or argv.
func cmdGitLabRotate(role string, ac *auditCtx, rotate bool) error {
	// The audit verb records what was actually DONE. A no-rotate lookup logged as "rotate"
	// would put rotations in the trail that never happened, which is exactly the kind of
	// audit line an incident reconstruction cannot trust.
	ac.verb = "rotate"
	if !rotate {
		ac.verb = "custody-read"
	}
	ac.role = role

	// A GitLab release-runner credential is a PIPELINE TRIGGER TOKEN, not a PAT: it has no
	// self-rotate endpoint, and presenting it to the PAT rotation endpoint would only earn a
	// 401 that reads like a broken custody file. Rotation is an operator act in the project's
	// CI/CD settings; the read-only lookup (--no-rotate) still verifies and prints custody.
	if role == "release-runner" && rotate {
		return deskkit.Refused("the GitLab release-runner credential is a pipeline trigger token, which has no " +
			"self-rotate endpoint — rotate it in the project's CI/CD settings, re-provision " +
			gitlabTokenFileName(role) + " (0600), and use --no-rotate to verify custody")
	}

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

	// Lstat, not Stat: the documented same-directory link (CUSTODY LAYOUT above) is followed;
	// any other link at the custody path is refused rather than checked and read through.
	fi, serr := deskkit.LstatCustody(path, deskkit.CustodySameDirLink)
	if serr != nil {
		if _, isLink := serr.(*deskkit.CustodyLinkError); isLink {
			return deskkit.Unverifiable(serr.Error(), nil)
		}
		return deskkit.Unverifiable("cannot stat gitlab token file at "+path, serr)
	}
	// Non-file custody (a directory, a symlink target that is not a regular file, a socket)
	// refuses — custody is a 0600 regular file, mirroring the GitHub key contract.
	if !fi.Mode().IsRegular() {
		return deskkit.Unverifiable(fmt.Sprintf(
			"gitlab custody at %s is not a regular file (mode %s); token custody requires a 0600 regular "+
				"file — re-provision the role's PAT there via a group owner", path, fi.Mode()), nil)
	}
	// Owner-only custody, behind the OS boundary (#667): a POSIX 0600 test on unix, the
	// owner-only NTFS ACL evaluation on Windows, where os.FileMode's permission bits are
	// synthetic (a normal file reads 0666) and a 0600 test would reject a correctly
	// ACL-locked PAT — the same check the cold-mint probe and ForgeFor custody read use.
	if err := deskkit.VerifyCustodyOwnerOnly(path, fi); err != nil {
		return deskkit.Unverifiable(err.Error(), nil)
	}

	// READ-ONLY LOOKUP (--no-rotate). Custody has been verified above; report the path and
	// stop. No lock is taken: this reads and writes nothing that a concurrent rotation could
	// tear, because the rotation lands its new value by rename, so a reader sees either the
	// whole old token or the whole new one and never a fragment. Taking the rotation lock
	// here would queue reads behind a rotation's network round trip, reintroducing exactly
	// the contention this path exists to remove.
	if !rotate {
		// The custody VALUE is read only to assert the credential is actually there — an
		// empty file would otherwise hand back a path for a role that has no token. It is
		// discarded immediately: like the rotating path, only the PATH is printed.
		if _, cerr := readGitLabCustody(path); cerr != nil {
			return cerr
		}
		fmt.Println(path)
		ac.detail = fmt.Sprintf("read gitlab %s custody path (--no-rotate: no rotation performed)", role)
		return nil
	}

	// SERIALISE THE WHOLE DESTRUCTIVE SEQUENCE. The lock must be held across read→rotate→
	// write-verify, not merely across the rotate call: a token read BEFORE a peer's rotation
	// and presented AFTER it is already dead, which is the losing half of the original race.
	// Reading only once the lock is held guarantees the value presented to the endpoint is
	// the one the previous holder persisted.
	lock, lerr := acquireGitLabRotateLock(role, path)
	if lerr != nil {
		return lerr
	}
	defer lock.release()

	current, cerr := readGitLabCustody(path)
	if cerr != nil {
		return cerr
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

	// SELF-CHECK. The new token is persisted; now prove the forge ACCEPTS it before the path
	// is handed to a caller that will present it on its very next request. One read-only GET,
	// no retry, no sleep (file header, SELF-CHECK). A status other than 200 — or no status at
	// all — never returns the path as good.
	status, serr := gitlabSelfCheck(base, result.Token)
	if serr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"post-rotation self-check for role %s could not be completed: the rotated token is persisted "+
				"at %s but its acceptance by the forge is unverified, so that path is NOT returned as good. "+
				"Re-run the mint once the forge is reachable.", role, path), serr)
	}
	if status != 200 {
		return gitlabSelfCheckFailure(role, base, path, current, status)
	}

	// Success: print the PATH only.
	fmt.Println(path)
	exp := result.ExpiresAt
	if exp == "" {
		exp = "per group policy"
	}
	ac.detail = fmt.Sprintf("rotated gitlab %s token in place (expires %s; self-check GET %s %d)",
		role, exp, gitlabSelfCheckPath, status)
	return nil
}
