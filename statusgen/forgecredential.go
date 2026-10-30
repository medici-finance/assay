package main

// forgecredential.go — the one place statusgen decides whether a forge read
// may fall back to a STORED login: the forge CLI's `auth token`, the login
// it keeps in its config directory (hosts.yml) or the system keyring, and a
// cell's CELLCTL_GH_AMBIENT copy of that login.
//
// Inside an admitted verifyrun that fallback is closed, and so is it wherever
// verifier admission would be required (a verify DESK_LOOP, or the verifier
// attestation file in the working repository), verifyrun or not. A Verify row runs with
// the forge token variables removed (admittedRowEnv), so a row that reached a
// forge read would otherwise authenticate as whoever is logged in on the
// runner. One function decides (storedLoginRefusal); every credential site
// receives its answer: the ruling-read client (rulingForgeClient), the
// dead-claim decay's token (githubDecayToken) and every forge CLI launch, each
// of which is wrapped in ghGuard. A refused read with no token variable set is
// a could-not-check on the site's existing error path, never an anonymous or
// ambient-authenticated read. Where none of storedLoginRefusal's signals holds,
// nothing changes.

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// verifyrunAdmission is this process's own record of where it stands in a
// verifyrun. runVerifyrun sets it; nothing reads it from the environment or a
// file, so no Verify row (a separate process) can change it.
var verifyrunAdmission atomic.Int32

const (
	verifyrunOutside    int32 = iota // not inside runVerifyrun
	verifyrunPending                 // inside, admission not yet established
	verifyrunAdmitted                // admission established and admitted
	verifyrunUnadmitted              // admission established: not required
)

// admissionState is the established state for an admission's attestation:
// admitted when there is one, else not required.
func admissionState(attestation string) int32 {
	if attestation != "" {
		return verifyrunAdmitted
	}
	return verifyrunUnadmitted
}

// storedLoginRefusal is the one decision: "" when this process may fall back
// to a stored forge login, else why it may not. It refuses when any signal of
// an admitted verifyrun holds:
//
//   - this process is inside verifyrun and admission is not yet established
//     (fail closed) or is established as admitted;
//   - the environment carries the grep pins admittedRowEnv appends to every
//     admitted row's environment, so this process is (or descends from) an
//     admitted Verify row;
//   - verifier admission would be required here: DESK_LOOP names a verify
//     loop, or the working directory's repository holds the verifier
//     attestation file (verifierAdmissionMarked, the predicate admission
//     itself uses, read the way admission reads it: a git that cannot be
//     started finds no file, there as here).
//
// It is computed on every call, never cached.
func storedLoginRefusal() string {
	switch verifyrunAdmission.Load() {
	case verifyrunPending:
		return "inside a verifyrun whose admission is not yet established"
	case verifyrunAdmitted:
		return "inside an admitted verifyrun"
	}
	if admittedRowEnvMarked() {
		return "inside an admitted Verify row"
	}
	if verifierAdmissionMarked(".") {
		return "where verifier admission is required"
	}
	return ""
}

// admittedRowEnvMarked reports whether this process's environment carries the
// git config pins admittedRowEnv appends to an admitted row's environment.
// The pins are read from admittedRowEnv itself, so the signature cannot drift
// from what admission writes.
func admittedRowEnvMarked() bool {
	pins := 0
	for _, kv := range admittedRowEnv(nil) {
		key, want, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(key, "GIT_CONFIG_KEY_") && !strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		if got, set := os.LookupEnv(key); !set || got != want {
			return false
		}
		pins++
	}
	return pins > 0
}

// forgeTokenSet reports whether any forge token variable the forge CLI reads
// for github.com (GH_TOKEN, GITHUB_TOKEN) or an enterprise host
// (GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN) is set, and returns the env
// additions that close the stored-login fallback for whichever family is
// unset: a placeholder the forge refuses, so the CLI cannot fall back to its
// own login for those hosts.
func forgeTokenSet() (bool, []string) {
	set := func(names ...string) bool {
		for _, n := range names {
			if strings.TrimSpace(os.Getenv(n)) != "" {
				return true
			}
		}
		return false
	}
	dotcom, enterprise := set("GH_TOKEN", "GITHUB_TOKEN"), set("GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN")
	var add []string
	if !dotcom {
		add = append(add, "GH_TOKEN="+storedLoginPlaceholder)
	}
	if !enterprise {
		add = append(add, "GH_ENTERPRISE_TOKEN="+storedLoginPlaceholder)
	}
	return dotcom || enterprise, add
}

// storedLoginPlaceholder stands in for an unset token family on a guarded
// forge CLI launch. It is not a credential: the forge answers it with an
// authentication failure, which the site reports as could-not-check.
const storedLoginPlaceholder = "stored-login-refused"

// ghGuard is how a forge CLI launch receives the decision. Every
// exec.Command("gh", …) in statusgen is passed through it before it starts.
// When storedLoginRefusal refuses: with no forge token variable set the
// command is never started (its Err is set, so Output/Run return the refusal
// on the site's existing error path); otherwise it runs with
// CELLCTL_GH_AMBIENT removed and any unset token family replaced by a
// placeholder, so the CLI authenticates only with the variable it was given.
// When it does not refuse, the command is returned unchanged.
func ghGuard(cmd *exec.Cmd) *exec.Cmd {
	why := storedLoginRefusal()
	if why == "" {
		return cmd
	}
	hasToken, add := forgeTokenSet()
	if !hasToken {
		cmd.Err = storedLoginError(why)
		return cmd
	}
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	var kept []string
	for _, kv := range env {
		if key, _, _ := strings.Cut(kv, "="); strings.EqualFold(key, "CELLCTL_GH_AMBIENT") {
			continue
		}
		kept = append(kept, kv)
	}
	cmd.Env = append(kept, add...)
	return cmd
}

// storedLoginError is the could-not-check every refused site reports.
func storedLoginError(why string) error {
	return fmt.Errorf("could-not-check: no forge token variable is set and the stored forge login is not used %s", why)
}

// refusedDoer is the transport of a ruling-read client built under a refusal
// with no token: it makes no request and answers every one with the refusal,
// which the resolver reports as unresolvable (could-not-check).
type refusedDoer struct{ why string }

func (d refusedDoer) Do(*http.Request) (*http.Response, error) {
	return nil, storedLoginError(d.why)
}

// verifierAdmissionMarked reports the two signals that make verifier admission
// required at root: a verify loop (DESK_LOOP naming verify) and the
// attestation file deskdispatch writes into a verifier worktree's git
// directory. A root that is not a repository, or a git that cannot be started,
// finds no file.
func verifierAdmissionMarked(root string) bool {
	if strings.Contains(strings.ToLower(os.Getenv("DESK_LOOP")), "verify") {
		return true
	}
	meta, err := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "assay-verifier-attestation.json").Output()
	if err != nil {
		return false
	}
	_, err = os.Stat(strings.TrimSpace(string(meta)))
	return err == nil
}
