package main

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which EVERY child process this verb starts
// flows. Production binds it to exec.Command; tests wrap it to record the real
// constructed argv. Nothing else in this package builds a command, so there is exactly one
// place argv is assembled — and every value in it is a literal verb or an already-validated
// value, never a shell string.
//
// SINCE THE FORGE MIGRATION THE ONLY THING THAT REACHES IT IS `git`. Every forge read and
// write this verb makes now goes through the resolved deskkit.Forge (an HTTP client bound
// to an explicitly minted App token), so there is no forge CLI to launch and no ambient CLI
// identity to fall back to. The seam stays because the repo resolution still reads
// `git remote get-url origin` — a local read that carries no identity at all.
var execCommand = exec.Command

// mintTokenFn is the seam the App-token lookup runs through, so a test can exercise the
// verb without a real App credential. Production binds it to the shared deskkit resolver,
// which shells out to the token minter and reads the file it names.
var mintTokenFn = deskkit.RoleTokenForRepo

// forgeAPIBase is a TEST-ONLY override of the API base the resolved GitHub backend is
// pointed at. It is EMPTY in production, which means "the backend's own default" — so this
// verb binds no forge host literal of its own anywhere, and there is deliberately no flag or
// environment variable that sets it (a production override could redirect a ready-flip, and
// the queue labels that go with it, at an attacker).
var forgeAPIBase string

// init installs this verb's existing, tested App-token lookup as the GitHub custody step
// deskkit.ForgeFor calls, instead of ForgeFor's default (which resolves the same
// RoleTokenForRepo path itself).
//
// WHY A HOOK RATHER THAN THE DEFAULT PATH. Two reasons, and only the second is about tests.
// First, the app-token CONDITION has to refuse BEFORE the first forge call, with a message
// naming the role and the token path — so this verb has to perform the lookup itself in any
// case, and letting ForgeFor repeat it would mint twice per run. Second, `mintTokenFn` is
// the seam the identity tests drive; routing custody through it keeps those tests exercising
// the same lookup the production path uses rather than a parallel one.
//
// The base URL is read HERE, at call time, so a per-test override still reaches the Forge
// this produces. (See forgeresolve.go's header for the resolver contract this plugs into.)
func init() {
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
		tok, _, merr := mintTokenFn(role, repo.Slug())
		if merr != nil {
			return "", "", merr
		}
		return tok, forgeAPIBase, nil
	})
}

type runResult struct {
	stdout string
	stderr string
	err    error
}

func runCmd(dir, name string, args ...string) runResult {
	cmd := execCommand(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return runResult{
		stdout: strings.TrimSpace(out.String()),
		stderr: strings.TrimSpace(errb.String()),
		err:    err,
	}
}

// firstLine reduces output to one line for a step report; an empty result renders as
// "(no output)" so a report can never read as though a tool said something it did not.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no output)"
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// repoSlugFromURL reduces an origin URL to owner/name for the SSH and HTTPS spellings.
// Anything else returns "" so the caller refuses instead of acting on a guess.
func repoSlugFromURL(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if at := strings.Index(s, "@"); at >= 0 {
			s = s[at+1:]
		}
	} else if i := strings.Index(s, ":"); i >= 0 && strings.Contains(s[:i], "@") {
		s = s[i+1:]
		parts := strings.Split(s, "/")
		if len(parts) < 2 {
			return ""
		}
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}
