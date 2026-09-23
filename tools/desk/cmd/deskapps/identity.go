// identity.go — "gh api user" (brief 02 facts: "Identity: gh api user (login, email,
// avatar_url) ... at start; never a token of its own"). A read-only call to the CLI's own
// `gh auth` login — deskapps mints nothing of its own until a conversion succeeds.
//
// Never called for --dry-run: a dry run's whole job is to report the planned URL and App
// rows without touching anything live, network included.
//
// ghOwnedOrgs (the org-membership lookup design.md §2 describes Screen 1 offering) was
// dropped 2026-09-18 (Desk-decided, PR review finding on assay#1260): it had zero call
// sites — Screen 1 never rendered the owned-orgs list this function fetched — so it was
// dead code carrying a live `gh` shell-out for nothing. Removing it does not touch
// ghIdentity/runGH's own gh-auth precondition, which is a separate, still-open finding.
package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// runGH is a test hook: production shells out to the real `gh` CLI; tests substitute a
// fake so no test here ever contacts github.com.
var runGH = func(args ...string) ([]byte, error) {
	return exec.Command("gh", args...).Output()
}

// ghUser is the subset of `gh api user` this brief reads.
type ghUser struct {
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// ghIdentity resolves the signed-in `gh auth` login driving this CLI invocation — the
// identity every screen names, and the one the callback's owner check compares a
// personal-owned conversion against (design.md §8: "Browser signed in as someone else").
func ghIdentity() (ghUser, error) {
	out, err := runGH("api", "user")
	if err != nil {
		return ghUser{}, fmt.Errorf("gh api user: %w", err)
	}
	var u ghUser
	if err := json.Unmarshal(out, &u); err != nil {
		return ghUser{}, fmt.Errorf("parse gh api user: %w", err)
	}
	return u, nil
}
