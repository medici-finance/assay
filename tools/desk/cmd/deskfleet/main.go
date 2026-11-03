// Command deskfleet provisions an Assay desk fleet on GitLab, and creates the fleet's queue
// and provenance labels on either forge, with no bash, curl or jq — so a Windows adopter runs
// it from native PowerShell. It is the Go port of tools/create-fleet-gitlab.sh, which stays
// as the reference implementation.
//
//	deskfleet provision --group <path> --prefix <name> [--project <path>] --owner-token-file <file> [--out-dir <dir>] [--pat-expiry-days N] [--avatars-dir <dir> | --no-avatars] [--dry-run]
//	deskfleet provision --avatars-only --avatars-dir <dir> --prefix <name> [--out-dir <dir>] [--dry-run]
//	deskfleet labels    --forge gitlab --project <path> --token-file <file> [--dry-run]
//	deskfleet labels    --forge github --repo <owner/name> --token-file <file> [--dry-run]
//
// `provision` creates the seven per-role service accounts (tables.go carries the script's
// role table unchanged), their group memberships and — for an account created by THIS run —
// one personal access token each, written to gitlab-<role>.token in the config home: the name
// `desktoken --forge gitlab <role>` reads, so no link or copy step follows. With --project it
// then configures the project's protected `main`, MR approvals, protected release tags, the
// two merge checks, and the fleet labels. With --avatars-dir each new account sets its own
// avatar (PUT /user/avatar, as that account's own PAT); without it the avatar step is skipped
// and named — there is no default remote icon fetch. `--avatars-only` sets the avatars of
// accounts that already exist, from their token files. `labels` creates only the labels, on
// either forge, from the SAME table.
//
// CUSTODY (the recorded fleet-token custody ruling). Each token file is created owner-only,
// then read back through the deskkit owner-only custody evaluation before its path is reported. A definite failure
// stops the run and names the credential for revocation; an inconclusive read-back WARNS,
// names the file, and continues (option 2). A run that fails partway through the account/token
// loop STOPS and REPORTS every token it minted — by role, account, token name and id, never
// the value — plus every account it created with no token and every write whose outcome it
// could not check (as could-not-check, never "nothing to revoke"); it writes that report
// beside the token files, exits non-zero, and revokes nothing (the partial-run ruling: report).
//
// Every mode that reaches the network prints its target before its first request.
//
// OFFLINE CONTRACT. --dry-run enumerates every action and makes ZERO network calls. A real
// run refuses before any network contact unless GITLAB_API_BASE is set (an environment-only
// binding the verb resolves when it runs; there is no default host) — and on success it prints the export
// line the operator adds to the shell that runs the desk verbs; it never writes roster.env.
//
// A BOOTSTRAP VERB. Like deskinstall, it runs before a fleet — or a roster — exists, so it
// reads no roster and consults no kill switch; it acts only on the forge a human points it at
// with a credential that human supplies as a FILE (never argv, never the environment).
//
// PARSING. cli.go builds the Cobra command tree (through the desk adapter, internal/cli); help
// and --version answer before any environment, file or network read.
//
// Exit: 0 ok · 1 a step failed (the output names each) · 2 usage · 5 refused (a precondition,
// or a token file that is definitely not owner-only).
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// env is every seam the tests replace. Production values come from defaultEnv.
type env struct {
	stdout, stderr   io.Writer
	http             *http.Client
	lookupEnv        func(string) (string, bool)
	now              func() time.Time
	createRestricted createRestrictedFunc
	classifyCustody  func(path string) deskkit.CustodyVerdict
	githubAPIBase    string
}

func defaultEnv() *env {
	return &env{
		stdout:           os.Stdout,
		stderr:           os.Stderr,
		http:             &http.Client{Timeout: 60 * time.Second},
		lookupEnv:        os.LookupEnv,
		now:              time.Now,
		createRestricted: createRestricted,
		classifyCustody:  deskkit.ClassifyCustodyOwnerOnly,
		githubAPIBase:    deskkit.GitHubAPIBase,
	}
}

const (
	exitOK      = 0
	exitFailed  = 1
	exitUsage   = 2
	exitRefused = deskkit.ExitRefused
)

func main() {
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	os.Exit(deskkit.PerfRunMeasured("deskfleet", []string{"provision", "labels"}, os.Args[1:], func(a []string, measurement *perfrec.Session) (int, perfrec.Context) {
		sink := func(c perfrec.Context) {
			selected = c
			if measurement != nil {
				measurement.SelectContext(c)
			}
		}
		return run(a, defaultEnv(), sink), selected
	}))
}

// credentialLinks says how readCredentialFile treats a symbolic link at the path it reads.
// Under either policy every check judges, and the read opens, the file actually read — never
// the link: on Windows deskkit.VerifyCustodyOwnerOnly reads the ACL from the path it is given,
// so passing a link path there would judge the link rather than the credential.
type credentialLinks int

const (
	// operatorNamedFile: a path the operator named on the command line (--owner-token-file,
	// labels --token-file). A link there is the operator's own layout, so it is followed, and
	// the checks run on its fully resolved target.
	operatorNamedFile credentialLinks = iota
	// custodyTokenFile: a desk token-custody file (<out-dir>/gitlab-<role>.token). It is read
	// under the custody-link rule main's other readers of the same file apply
	// (deskkit.LstatCustody with CustodySameDirLink, as `desktoken --forge gitlab` and the
	// preflight do): the documented same-directory link is followed, any other link is
	// refused, so a file planted elsewhere is never presented as the role's credential.
	custodyTokenFile
)

// readCredentialFile reads a credential file under the READ-side custody standard
// (deskkit.VerifyCustodyOwnerOnly — any failure refuses), with links handled per links. The
// value is returned for use as a request header only; nothing here formats it.
func readCredentialFile(flagName, path string, links credentialLinks) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required for a real run (a FILE holding the credential — never "+
			"a flag value or an environment variable)", flagName)
	}
	var (
		target string
		fi     os.FileInfo
		err    error
	)
	switch links {
	case custodyTokenFile:
		target, fi, err = deskkit.LstatCustody(path, deskkit.CustodySameDirLink)
	default:
		if target, err = filepath.EvalSymlinks(path); err == nil {
			fi, err = os.Lstat(target)
		}
	}
	if err != nil {
		var linkErr *deskkit.CustodyLinkError
		if errors.As(err, &linkErr) {
			return "", fmt.Errorf("%s: %v", flagName, err)
		}
		return "", fmt.Errorf("%s %s: %v", flagName, path, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s %s is not a regular file", flagName, target)
	}
	if err := deskkit.VerifyCustodyOwnerOnly(target, fi); err != nil {
		return "", fmt.Errorf("%s: %v", flagName, err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", flagName, target, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("%s %s is empty", flagName, path)
	}
	return tok, nil
}

// gitlabAPIBase checks the GITLAB_API_BASE value the running verb resolved (raw: the declared
// environment-only binding). There is deliberately no default: GitLab is commonly
// self-hosted, and a guessed host would receive a live credential.
func gitlabAPIBase(raw string) (string, error) {
	base := strings.TrimSpace(raw)
	if base == "" {
		return "", fmt.Errorf("GITLAB_API_BASE is not set — refusing before any network contact. Set it to " +
			"your deployment's REST v4 base (self-hosted: https://gitlab.example.com/api/v4; gitlab.com " +
			"SaaS: https://gitlab.com/api/v4). There is no default host")
	}
	if err := validateAPIBase("GITLAB_API_BASE", base); err != nil {
		return "", err
	}
	return strings.TrimRight(base, "/"), nil
}
