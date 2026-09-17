// identity.go — "gh api user" and the owned-orgs lookup (brief 02 facts: "Identity: gh api
// user (login, email, avatar_url) and gh api user/memberships/orgs ... at start; never a
// token of its own"). Both are read-only calls to the CLI's own `gh auth` login — deskapps
// mints nothing of its own until a conversion succeeds.
//
// Neither is ever called for --dry-run: a dry run's whole job is to report the planned URL
// and App rows without touching anything live, network included.
package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

// ghOwnedOrgs returns the orgs this login administers (role=admin), the set Screen 1 offers
// as org-owned targets.
func ghOwnedOrgs() ([]string, error) {
	out, err := runGH("api", "user/memberships/orgs", "--jq", `.[] | select(.role=="admin") | .organization.login`)
	if err != nil {
		return nil, fmt.Errorf("gh api user/memberships/orgs: %w", err)
	}
	var orgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			orgs = append(orgs, line)
		}
	}
	return orgs, nil
}
