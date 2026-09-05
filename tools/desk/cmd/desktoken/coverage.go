package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// coverage.go — `desktoken coverage <role>`: enumerate the repositories a role's
// App installations can see.
//
// This verb answers ONE question the coordinator kept hand-writing a JWT probe
// for: does role X's App see repo Y? Every API call it
// makes is one `desktoken` already makes to mint a token — list installations,
// mint a per-installation token, list that installation's repositories — with the
// answer PRINTED instead of discarded.
//
// It is READ-ONLY on the forge and on disk. The tokens it mints to read each
// installation's repository list are held in memory and dropped; NOTHING is
// written to the token cache or a .perms sidecar. A cache written under an
// enumeration would shadow the next real mint's permission view — the exact
// masking `--fresh` exists to undo — so this verb never writes one. The JWT and
// the installation tokens are never printed, logged, or audited: the audit line
// records role, installation count, repository count, filter and result only.

// coverageInstallation is the subset of GET /app/installations this verb renders.
type coverageInstallation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
	RepositorySelection string `json:"repository_selection"`
}

// coverageRepo is one repository from GET /installation/repositories.
type coverageRepo struct {
	FullName string `json:"full_name"`
}

// installationReposPage is one page of GET /installation/repositories.
type installationReposPage struct {
	Repositories []coverageRepo `json:"repositories"`
}

// coveragePerPage is the page size requested from /installation/repositories.
// It is a package var — never a compiled-in const — so a test can force
// pagination across two pages without a 100-repository fixture. Production keeps
// the GitHub maximum.
var coveragePerPage = 100

// coverageResult is one installation's rendered coverage, after enumeration.
type coverageResult struct {
	installation coverageInstallation
	repos        []string // full_name each, sorted
}

// jsonInstallation is the --json shape for one installation.
type jsonInstallation struct {
	ID        int64    `json:"id"`
	Account   string   `json:"account"`
	Type      string   `json:"type"`
	Selection string   `json:"selection"`
	Repos     []string `json:"repos"`
}

// jsonCoverage is the top-level --json object.
type jsonCoverage struct {
	Installations []jsonInstallation `json:"installations"`
}

// accountTypeShort maps GitHub's account.type ("Organization"/"User") to the
// rendered short form. An unrecognised value renders verbatim rather than being
// coerced.
func accountTypeShort(t string) string {
	switch t {
	case "Organization":
		return "Org"
	case "User":
		return "User"
	default:
		return t
	}
}

// cmdCoverage implements `desktoken coverage <role> [--repo <slug>] [--json]`.
func cmdCoverage(args []string) (err error) {
	ac := &auditCtx{verb: "coverage", argsDigest: deskkit.ArgsDigest(os.Args[1:])}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("desktoken coverage", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	repo := fs.String("repo", "", "repo slug (owner/name): print only the installation that sees it; exit 0 if one does, 5 if none")
	forge := fs.String("forge", "", "forge backend: empty/github (default); gitlab is refused (a PAT has no installation to enumerate)")
	asJSON := fs.Bool("json", false, "emit the enumeration as one JSON object")

	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("bad flags: " + perr.Error())
	}
	if len(positionals) != 1 {
		return deskkit.Refused("exactly one <role> argument required, one of: " + strings.Join(roleNames(), ", "))
	}
	role := positionals[0]
	if !validRoles[role] {
		return deskkit.Refused("unknown role " + role + "; valid: " + strings.Join(roleNames(), ", "))
	}
	ac.role = role

	// --forge gitlab is refused BEFORE any network contact: a GitLab PAT has no
	// App installation to enumerate, so the verb is GitHub-only by construction.
	switch strings.ToLower(strings.TrimSpace(*forge)) {
	case "", "github":
		// GitHub App enumeration below.
	case "gitlab":
		return deskkit.Refused("coverage is GitHub-only: a GitLab PAT has no App installation to enumerate")
	default:
		return deskkit.Refused("unknown --forge " + *forge + "; valid: github (default)")
	}

	prefix := roleEnvPrefix(role)

	// Sign an App JWT (the same primitive the mint path uses). This reads the App
	// PEM and App ID; it writes nothing.
	jwt, jerr := buildAppJWTForRole(role, prefix)
	if jerr != nil {
		return jerr
	}

	// List every installation of this App.
	installs, ierr := listInstallations(jwt)
	if ierr != nil {
		return deskkit.Unverifiable("list App installations", ierr)
	}

	// Enumerate each installation's repositories, minting a per-installation token
	// into MEMORY only. Stable order: by account.login.
	sort.Slice(installs, func(i, j int) bool {
		if strings.EqualFold(installs[i].Account.Login, installs[j].Account.Login) {
			return installs[i].ID < installs[j].ID
		}
		return strings.ToLower(installs[i].Account.Login) < strings.ToLower(installs[j].Account.Login)
	})

	results := make([]coverageResult, 0, len(installs))
	totalRepos := 0
	for _, inst := range installs {
		instID := fmt.Sprintf("%d", inst.ID)
		tok, xerr := exchangeJWT(jwt, instID)
		if xerr != nil {
			return deskkit.Unverifiable("mint token for installation "+instID+" (account "+inst.Account.Login+")", xerr)
		}
		repos, rerr := listInstallationRepos(tok.Token)
		if rerr != nil {
			// A page read that failed: could-not-check is NOT "not covered". Refuse
			// with exit 6 naming the installation; never present a short list as complete.
			return deskkit.Unverifiable(fmt.Sprintf(
				"list repositories for installation %s (account %s) — a page read failed; the enumeration is incomplete and no partial list is printed",
				instID, inst.Account.Login), rerr)
		}
		sort.Strings(repos)
		results = append(results, coverageResult{installation: inst, repos: repos})
		totalRepos += len(repos)
	}

	// --repo filter: print only the installation that sees the slug; exit 0 if one
	// does, 5 if none. The match is on full_name, so a same-named repo under a
	// different owner does not match.
	if *repo != "" {
		slug := strings.TrimSpace(*repo)
		var hit *coverageResult
		for i := range results {
			for _, full := range results[i].repos {
				if strings.EqualFold(full, slug) {
					hit = &results[i]
					break
				}
			}
			if hit != nil {
				break
			}
		}
		if hit == nil {
			ac.detail = fmt.Sprintf("role=%s installations=%d repos=%d filter=%s result=not-covered",
				role, len(results), totalRepos, slug)
			return deskkit.Refused(fmt.Sprintf("no installation of the %s App sees %s (searched %d installation(s))",
				role, slug, len(results)))
		}
		ac.detail = fmt.Sprintf("role=%s installations=%d repos=%d filter=%s result=covered-by=%d",
			role, len(results), totalRepos, slug, hit.installation.ID)
		if *asJSON {
			renderJSON([]coverageResult{*hit})
		} else {
			renderText([]coverageResult{*hit})
		}
		return nil
	}

	ac.detail = fmt.Sprintf("role=%s installations=%d repos=%d filter=none result=listed",
		role, len(results), totalRepos)
	if *asJSON {
		renderJSON(results)
	} else {
		renderText(results)
	}
	return nil
}

// renderText prints one block per installation, in the order given, each followed
// by its repositories one indented line each.
func renderText(results []coverageResult) {
	for _, r := range results {
		fmt.Printf("installation %d account=%s type=%s selection=%s repos=%d\n",
			r.installation.ID,
			r.installation.Account.Login,
			accountTypeShort(r.installation.Account.Type),
			r.installation.RepositorySelection,
			len(r.repos),
		)
		for _, full := range r.repos {
			fmt.Printf("  %s\n", full)
		}
	}
}

// renderJSON emits the enumeration as one JSON object.
func renderJSON(results []coverageResult) {
	out := jsonCoverage{Installations: make([]jsonInstallation, 0, len(results))}
	for _, r := range results {
		repos := r.repos
		if repos == nil {
			repos = []string{}
		}
		out.Installations = append(out.Installations, jsonInstallation{
			ID:        r.installation.ID,
			Account:   r.installation.Account.Login,
			Type:      accountTypeShort(r.installation.Account.Type),
			Selection: r.installation.RepositorySelection,
			Repos:     repos,
		})
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

// listInstallations GETs /app/installations with the App JWT and returns every
// installation. Read-only.
func listInstallations(jwt string) ([]coverageInstallation, error) {
	url := deskkit.GitHubAPIBase + "/app/installations?per_page=100"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET installations: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read installations response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("installations HTTP %d: %s", resp.StatusCode, string(body))
	}

	var installs []coverageInstallation
	if err := json.Unmarshal(body, &installs); err != nil {
		return nil, fmt.Errorf("parse installations response: %w", err)
	}
	return installs, nil
}

// listInstallationRepos pages GET /installation/repositories authenticated with
// an installation token, following pages until one returns fewer than
// coveragePerPage. A page that does not return HTTP 200 is an ERROR — never a
// shorter list read as complete.
func listInstallationRepos(token string) ([]string, error) {
	var out []string
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/installation/repositories?per_page=%d&page=%d",
			deskkit.GitHubAPIBase, coveragePerPage, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET repositories page %d: %w", page, err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return nil, fmt.Errorf("read repositories page %d: %w", page, rerr)
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("repositories page %d HTTP %d: %s", page, resp.StatusCode, string(body))
		}
		var pg installationReposPage
		if err := json.Unmarshal(body, &pg); err != nil {
			return nil, fmt.Errorf("parse repositories page %d: %w", page, err)
		}
		for _, r := range pg.Repositories {
			out = append(out, r.FullName)
		}
		if len(pg.Repositories) < coveragePerPage {
			break
		}
	}
	return out, nil
}

// buildAppJWTForRole resolves the role's App PEM and App ID off the
// App-credential search path and returns a signed App JWT. It reuses the mint
// path's primitives (resolvePEMPath, checkPrivateKeyMode, parsePrivateKey,
// buildJWT) and writes NOTHING.
func buildAppJWTForRole(role, prefix string) (string, error) {
	pemPath, pemErr := resolvePEMPath(role, prefix)

	appID, aerr := deskkit.AppID(role)
	if aerr != nil {
		return "", deskkit.Unverifiable(aerr.Error(), nil)
	}

	if pemErr != nil {
		return "", pemErr
	}
	fi, perr := os.Stat(pemPath)
	if perr != nil {
		if os.IsNotExist(perr) {
			return "", deskkit.Unverifiable("private key not found at "+pemPath, perr)
		}
		return "", deskkit.Unverifiable("cannot stat private key at "+pemPath, perr)
	}
	if merr := checkPrivateKeyMode(pemPath, fi.Mode()); merr != nil {
		return "", merr
	}
	keyPEM, rerr := os.ReadFile(pemPath)
	if rerr != nil {
		return "", deskkit.Unverifiable("cannot read private key at "+pemPath, rerr)
	}
	key, kerr := parsePrivateKey(keyPEM)
	if kerr != nil {
		return "", deskkit.Unverifiable("parse private key from "+pemPath, kerr)
	}
	jwt, jerr := buildJWT(appID, time.Now(), key)
	if jerr != nil {
		return "", deskkit.Unverifiable("sign JWT", jerr)
	}
	return jwt, nil
}
