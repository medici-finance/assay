package main

// labels.go — the forge label set on a house cell's repos: `cellctl new --kind house` ensures
// it, `cellctl check` reports it.
//
// The set is internal/forgelabels' one canonical table, built from the roles the cell's own
// roster binds (deskkit.Config.BoundRoles, the projection `deskfile --to` / `--raised-by`
// resolve against), so every `to:<role>` a desk can stamp is one the install created. The
// targets are the CELL_ROOTS repos (the cell's scope).
//
// LABEL CREATION IS AN OPERATOR ACT under the operator's credential: a FILE the operator
// names (`new --labels-token-file`, cell.env CELL_LABELS_TOKEN_FILE for `check`). cellctl
// never mints one and never reads a desk role's App token. A step that cannot finish — no
// credential given, a credential the forge denies, any failed create — is reported NOT
// ENSURED with the exact `deskfleet labels …` command that finishes it. It never reads as done.

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/forgelabels"
)

// labelsHTTP is the client every label request goes through; tests point it at a stub.
var labelsHTTP = &http.Client{Timeout: 30 * time.Second}

// labelsTokenKey is the cell.env key naming the operator credential `check` reads labels with.
const labelsTokenKey = "CELL_LABELS_TOKEN_FILE"

// boundRolesUnder returns the roles the roster under home binds, or nil when that roster is
// not configured or does not parse — a set derived from a broken roster would be a guess.
func boundRolesUnder(home string) []string {
	restore := withHome(home)
	defer restore()
	cfg := deskkit.LoadConfig(deskkit.ClassWrite)
	if !cfg.Configured() || len(cfg.Problems) > 0 {
		return nil
	}
	return cfg.BoundRoles()
}

// labelRepos is the CELL_ROOTS keys: the repos (github) or project paths (gitlab) in scope.
func labelRepos(roots string) []string {
	var out []string
	for _, e := range rootEntries(roots) {
		if e[0] != "" {
			out = append(out, e[0])
		}
	}
	return out
}

func labelsBackend(forge, base, target, token string) (forgelabels.Backend, error) {
	t, err := forgelabels.NewHTTPTransport(forge, base, labelsHTTP, token)
	if err != nil {
		return nil, err
	}
	if forge == "gitlab" {
		return forgelabels.GitLab(t, target), nil
	}
	return forgelabels.GitHub(t, target), nil
}

// ensureCellLabels is `new --kind house`'s labels step. It prints one line per repo and
// returns false when the operator handed it a credential and any repo's set is still not
// ensured — `new` then exits 1. With no credential the step is DEFERRED, not done: every repo
// prints NOT ENSURED with its finish command and `new` keeps its exit 0, because scaffolding
// without the operator's label-write credential is a legitimate install order (the roster
// often comes after the cell), and `cellctl check` keeps reporting the set until it exists.
func ensureCellLabels(forge, base string, repos, roles []string, tokenFile string) bool {
	given := strings.TrimSpace(tokenFile) != ""
	finish := func(repo string) string { return forgelabels.Command(forge, repo, roles, base, false) }
	notEnsured := func(why string) bool {
		for _, r := range repos {
			fmt.Printf("[new] labels: NOT ENSURED on %s — %s; finish with: %s\n", r, why, finish(r))
		}
		return !given
	}
	if len(roles) == 0 {
		return notEnsured("the cell's roster binds no role yet, so the to:<role> / raised-by:<role> rows cannot be derived (write the roster first)")
	}
	if !given {
		return notEnsured("no --labels-token-file given (label creation is an operator act, under an operator credential that can write labels)")
	}
	tok, err := forgelabels.ReadCredentialFile("--labels-token-file", tokenFile)
	if err != nil {
		return notEnsured(err.Error())
	}
	set := forgelabels.Canonical(roles)
	// Name where the credential goes before the first request, as `deskfleet labels` does.
	fmt.Printf("[new] labels: target %s API %s, repos %s, roles %s\n", forge, base, strings.Join(repos, ","), strings.Join(roles, ","))
	ok := true
	for _, r := range repos {
		b, err := labelsBackend(forge, base, r, tok)
		if err != nil {
			fmt.Printf("[new] labels: NOT ENSURED on %s — %v; finish with: %s\n", r, err, finish(r))
			ok = false
			continue
		}
		rs := forgelabels.Ensure(b, set)
		n, denied := forgelabels.Failed(rs)
		if n == 0 {
			created := 0
			for _, x := range rs {
				if !x.Exists {
					created++
				}
			}
			fmt.Printf("[new] labels: ensured %d on %s (%d created, %d already present)\n", len(set), r, created, len(set)-created)
			continue
		}
		ok = false
		var names []string
		for _, x := range rs {
			if x.Err != nil {
				names = append(names, x.Label.Name)
			}
		}
		why := fmt.Sprintf("%d of %d not created (%s)", n, len(set), strings.Join(names, ", "))
		if denied {
			why += " — the credential cannot write labels on this target"
		}
		fmt.Printf("[new] labels: NOT ENSURED on %s — %s; finish with: %s\n", r, why, finish(r))
	}
	return ok
}

// labelsRows is `check`'s labels report for a house cell: per repo, ok when every label of
// the roster-derived set is present (an inherited GitLab group label counts), MISS naming
// each missing label, could-not-check when the set cannot be derived or the labels cannot be
// read. A could-not-check row is never an ok row.
func (c *Cell) labelsRows(k *checker) {
	repos := labelRepos(c.Env.Get("CELL_ROOTS"))
	if len(repos) == 0 {
		return
	}
	roles := boundRolesUnder(c.Home)
	if len(roles) == 0 {
		k.cnc("labels: the cell's roster binds no role, so the to:<role> / raised-by:<role> set cannot be derived (fix the roster row above)")
		return
	}
	base := c.ForgeAPIBase
	byHand := func(repo string) string { return forgelabels.Command(c.Forge, repo, roles, base, true) }
	tokenFile := c.Env.Get(labelsTokenKey)
	if strings.TrimSpace(tokenFile) == "" {
		for _, r := range repos {
			k.cnc("labels on %s: %s unset in cell.env (a FILE holding an operator credential that can read labels); by hand: %s",
				r, labelsTokenKey, byHand(r))
		}
		return
	}
	tok, err := forgelabels.ReadCredentialFile(labelsTokenKey, tokenFile)
	if err != nil {
		for _, r := range repos {
			k.cnc("labels on %s: %v", r, err)
		}
		return
	}
	set := forgelabels.Canonical(roles)
	// Name where the credential goes before the first request, as `deskfleet labels` does.
	fmt.Printf("[check] labels: reading from %s API %s, repos %s\n", c.Forge, base, strings.Join(repos, ","))
	for _, r := range repos {
		b, err := labelsBackend(c.Forge, base, r, tok)
		var missing []string
		if err == nil {
			missing, err = forgelabels.Check(b, set)
		}
		switch {
		case err != nil:
			k.cnc("labels on %s: could not be read (%v)", r, err)
		case len(missing) > 0:
			k.chk(false, "labels on %s: %d of %d missing: %s — finish with: %s", r, len(missing), len(set),
				strings.Join(missing, ", "), forgelabels.Command(c.Forge, r, roles, base, false))
		default:
			k.chk(true, "labels on %s: all %d present (roles %s)", r, len(set), strings.Join(roles, ","))
		}
	}
}
