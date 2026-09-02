package deskkit

// roletoken.go — the two identity facts a desk verb needs before it touches the forge:
// WHICH App role this session acts as, and the installation TOKEN that role authenticates
// with.
//
// WHY THEY LIVE TOGETHER, AND HERE. Both were previously per-command: the loop-to-role
// table was private to the boot verb, and each write verb carried its own copy of "shell
// out to the token minter, read the file it names". A read verb that needed the same
// token therefore had no way to ask for one, and the read path fell through to whatever
// account the ambient `gh` keyring happened to hold. Under a config home that is not the
// operator's own that account cannot authenticate, so every private repo came back 401 —
// the board went blind to exactly the queue it exists to surface. Worse than the 401: an
// unusable keyring account has been observed to make GraphQL reads return a bogus
// rate-limit error, and some GraphQL reads return an empty list rather than an error at
// all. A false-empty read is an absence that looks like an answer, which is the one shape
// this codebase treats as unacceptable.
//
// FAIL CLOSED, AND NEVER PRINT THE TOKEN. Every failure below is a typed refusal that
// names the role and the token PATH. The token VALUE is never placed in an error, a log
// line, or an audit record — only the path is.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// loopTokenRoles maps a desk LOOP name (what a session presents in $DESK_LOOP, and what a
// human arms a STOP.<name> flag on) to the APP role whose installation token that session
// mints and acts under.
//
// The two vocabularies are genuinely different — a loop is a WINDOW, a role is an App
// IDENTITY — and keeping them separate is why a session names its role once rather than
// spelling both halves at every call site and eventually spelling them apart.
var loopTokenRoles = map[string]string{
	"the-desk":       "desk",
	"worker-desk":    "worker",
	"pr-review-desk": "reviewer",
	"verify-desk":    "verifier",
	"intake-desk":    "issue-loop",
}

// LoopTokenRoles returns a COPY of the loop-to-App-role table, so a caller that ranges
// over it cannot mutate the shared one.
func LoopTokenRoles() map[string]string {
	out := make(map[string]string, len(loopTokenRoles))
	for k, v := range loopTokenRoles {
		out[k] = v
	}
	return out
}

// TokenRoleForLoop returns the App role a loop acts under. ok=false means the loop has no
// App identity — which is a refusal at every call site, never a default role.
func TokenRoleForLoop(loop string) (role string, ok bool) {
	r, ok := loopTokenRoles[loop]
	return r, ok
}

// LoopsWithTokenRole lists the loop names that have an App role, sorted, for a message
// that has to tell the reader what to export.
func LoopsWithTokenRole() []string {
	out := make([]string, 0, len(loopTokenRoles))
	for loop := range loopTokenRoles {
		if IsKnownLoopName(loop) {
			out = append(out, loop)
		}
	}
	sort.Strings(out)
	return out
}

// RequireLoopIdentity is the check every OUTWARD verb makes before it writes anything:
// this session must present a loop identity in $DESK_LOOP.
//
// WHAT IT PROTECTS. The kill switch's per-loop halt is `STOP.<loop>`, matched against
// $DESK_LOOP. With the variable unset there is nothing to match, so a STOP.<name> flag a
// human is holding never fires and the stop silently fails — the session keeps writing
// while the operator believes it has been halted. The boot verb has checked this since it
// was written; an outward verb run OUTSIDE a booted window did not, which is the gap.
//
// It cannot EXPORT the variable — a child process has no reach into its caller's shell —
// so the refusal carries the exact line to run. An UNRECOGNISED name is could-not-check
// (exit 6), never "some other loop": a name nothing recognises tells you nothing about
// whether a stop is held, and reading it as an answer is how a silent failure gets
// reported as clean. That is the same three-state rule the kill switch itself applies.
func RequireLoopIdentity(verb string) error {
	raw := strings.TrimSpace(os.Getenv(loopEnv))
	if raw == "" {
		return Refused(fmt.Sprintf(
			"%s: $%s is unset, so a STOP.<loop> flag a human is holding would never match this session "+
				"and the stop would silently fail. Run `export %s=<loop>` in THIS shell — one of: %s — "+
				"then re-run %s. (%s is a child process; it cannot export into your shell.)",
			verb, loopEnv, loopEnv, strings.Join(KnownLoopNames(), ", "), verb, verb))
	}
	if _, known := LoopFlagNames(raw); !known {
		return Unverifiable(fmt.Sprintf(
			"%s: $%s=%q is not a loop name the kill switch recognises, so whether a stop flag is held for "+
				"this session CANNOT be established — that is could-not-check, never 'no stop held'. "+
				"Known loop names: %s.",
			verb, loopEnv, raw, strings.Join(KnownLoopNames(), ", ")), nil)
	}
	return nil
}

// SessionTokenRole resolves the App role THIS session acts as, from the loop identity it
// presents. It fails closed at each step and never guesses a role: acting under the wrong
// App identity is the failure it exists to prevent, and a guessed one would be
// indistinguishable from a correct one in the audit trail.
func SessionTokenRole(verb string) (role, loop string, err error) {
	if err := RequireLoopIdentity(verb); err != nil {
		return "", "", err
	}
	names, _ := LoopFlagNames(strings.TrimSpace(os.Getenv(loopEnv)))
	loop = names[0]
	role, ok := TokenRoleForLoop(loop)
	if !ok {
		return "", loop, Unverifiable(fmt.Sprintf(
			"%s: loop %q has no App role, so which identity this session should act under cannot be "+
				"established. Loops that carry an App role: %s.",
			verb, loop, strings.Join(LoopsWithTokenRole(), ", ")), nil)
	}
	return role, loop, nil
}

// tokenMinter is the seam the token lookup runs through. Production shells out to the
// minter binary; a test replaces it so no real App credential is needed. It returns the
// PATH of the cached token file — the minter prints a path and never the token value, and
// this side keeps that property.
var tokenMinter = func(role, owner string) (path string, stderr string, err error) {
	cmd := exec.Command("desktoken", role, "--repo", owner)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	return strings.TrimSpace(out.String()), strings.TrimSpace(errb.String()), err
}

// OwnerOf returns the account half of an "owner/name" slug. A bare owner is returned
// unchanged, so a caller that only knows the account (an owner-scoped search) can ask for
// its token without inventing a repository name.
func OwnerOf(repo string) string {
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[:i]
	}
	return repo
}

// RoleTokenForOwner returns the role's App installation token for one ACCOUNT, plus the
// path of the file it was read from.
//
// PER-OWNER, NOT PER-REPO. A GitHub App is installed per account, and an installation
// token resolves only the repositories of ITS installation. A token minted for one owner
// used against another owner's repo does not 401 — it fails with "could not resolve to a
// repository", which reads like a missing repo rather than a wrong identity. So the owner
// is the unit, and every caller keys its cache on it.
func RoleTokenForOwner(role, owner string) (token, path string, err error) {
	role = strings.TrimSpace(role)
	owner = strings.TrimSpace(owner)
	if role == "" {
		return "", "", Unverifiable("no App role named for the token lookup — a token cannot be minted "+
			"for an identity this process cannot name", nil)
	}
	if owner == "" {
		return "", "", Unverifiable(fmt.Sprintf(
			"no account named for the %s App token lookup — an installation token is per account, so "+
				"there is nothing to mint against", role), nil)
	}
	path, stderr, err := tokenMinter(role, owner)
	if err != nil {
		detail := firstLine(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return "", path, Unverifiable(fmt.Sprintf(
			"cannot mint the %s App installation token for %s (%s)", role, owner, detail), err)
	}
	if path == "" {
		return "", "", Unverifiable(fmt.Sprintf(
			"the token minter returned no path for the %s App installation token on %s", role, owner), nil)
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", path, Unverifiable(fmt.Sprintf(
			"cannot read the %s App installation token at %s", role, path), rerr)
	}
	token = strings.TrimSpace(string(b))
	if token == "" {
		return "", path, Unverifiable(fmt.Sprintf(
			"the %s App installation token at %s is empty", role, path), nil)
	}
	return token, path, nil
}

// RoleTokenForRepo is RoleTokenForOwner keyed by a repository slug, for the callers that
// hold one.
func RoleTokenForRepo(role, repo string) (token, path string, err error) {
	return RoleTokenForOwner(role, OwnerOf(repo))
}
