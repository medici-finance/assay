// Command deskfleet provisions an Assay desk fleet on GitLab, and creates the fleet's queue
// and provenance labels on either forge, with no bash, curl or jq — so a Windows adopter runs
// it from native PowerShell. It is the Go port of tools/create-fleet-gitlab.sh, which stays
// as the reference implementation.
//
//	deskfleet provision --group <path> --prefix <name> [--project <path>] --owner-token-file <file> [--out-dir <dir>] [--pat-expiry-days N] [--dry-run]
//	deskfleet labels    --forge gitlab --project <path> --token-file <file> [--dry-run]
//	deskfleet labels    --forge github --repo <owner/name> --token-file <file> [--dry-run]
//
// `provision` creates the seven per-role service accounts (tables.go carries the script's
// role table unchanged), their group memberships and — for an account created by THIS run —
// one personal access token each, written to gitlab-<role>.token in the config home: the name
// `desktoken --forge gitlab <role>` reads, so no link or copy step follows. With --project it
// then configures the project's protected `main`, MR approvals, protected release tags, the
// two merge checks, and the fleet labels. `labels` creates only the labels, on either forge,
// from the SAME table.
//
// CUSTODY (the recorded fleet-token custody ruling). Each token file is created owner-only,
// then read back through the deskkit owner-only custody evaluation before its path is reported. A definite failure
// stops the run and names the credential for revocation; an inconclusive read-back WARNS,
// names the file, and continues (option 2). A run that fails partway through the account/token
// loop STOPS and REPORTS every token it minted — by role, account, token name and id, never
// the value — writes that report beside the token files, exits non-zero, and revokes nothing
// (the partial-run ruling: report).
//
// OFFLINE CONTRACT. --dry-run enumerates every action and makes ZERO network calls. A real
// run refuses before any network contact unless GITLAB_API_BASE is set (read from the
// environment at call time; there is no default host) — and on success it prints the export
// line the operator adds to the shell that runs the desk verbs; it never writes roster.env.
//
// A BOOTSTRAP VERB. Like deskinstall, it runs before a fleet — or a roster — exists, so it
// reads no roster and consults no kill switch; it acts only on the forge a human points it at
// with a credential that human supplies as a FILE (never argv, never the environment).
//
// Exit: 0 ok · 1 a step failed (the output names each) · 2 usage · 5 refused (a precondition,
// or a token file that is definitely not owner-only).
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskfleet — Go-native GitLab fleet provisioning and forge-neutral fleet labels.

USAGE:
  deskfleet provision --group <path-or-id> --prefix <name> --owner-token-file <file>
                      [--project <path-or-id>] [--out-dir <dir>] [--pat-expiry-days N] [--dry-run]
  deskfleet labels --forge gitlab --project <path-or-id> --token-file <file> [--dry-run]
  deskfleet labels --forge github --repo <owner/name>    --token-file <file> [--dry-run]
  deskfleet --version

provision  creates the seven role service accounts, their group memberships and (for an
           account created by this run) one PAT each, written owner-only to
           gitlab-<role>.token in --out-dir (default: the config home, where
           desktoken --forge gitlab <role> reads it). With --project it also protects main,
           sets MR approvals, protects release tags, sets the pipeline and discussion merge
           checks, and creates the fleet labels. Requires GITLAB_API_BASE (your REST v4 base,
           e.g. https://gitlab.example.com/api/v4) — there is no default host.
labels     creates the nine fleet labels (review-request, six raised-by:<role>, and the
           authorization-needed / approval-needed pair) on one GitHub repo or GitLab project.
           An existing label is a no-op.

Credentials are read from FILES only (--owner-token-file / --token-file), which must pass
the owner-only custody check; they are never accepted as a flag value or env variable, and
never printed — only paths are.

--dry-run  enumerates every action and makes ZERO network calls.

Custody (the recorded ruling): a minted token whose owner-only read-back is INCONCLUSIVE is
kept with a WARNING naming the file; one that is definitely not owner-only stops the run.
A run that fails partway through minting stops, reports every token it minted (never the
value) for manual revocation, and revokes nothing.

Exit: 0 ok · 1 a step failed · 2 usage · 5 refused.`

// env is every seam the tests replace. Production values come from defaultEnv.
type env struct {
	stdout, stderr   io.Writer
	http             *http.Client
	getenv           func(string) string
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
		getenv:           os.Getenv,
		now:              time.Now,
		createRestricted: createRestricted,
		classifyCustody:  deskkit.ClassifyCustodyOwnerOnly,
		githubAPIBase:    "https://api.github.com",
	}
}

const (
	exitOK      = 0
	exitFailed  = 1
	exitUsage   = 2
	exitRefused = deskkit.ExitRefused
)

func main() {
	os.Exit(run(os.Args[1:], defaultEnv()))
}

func run(args []string, e *env) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(e.stdout, "deskfleet sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return exitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(e.stderr, usage)
		if len(args) == 0 {
			return exitUsage
		}
		return exitOK
	}
	switch args[0] {
	case "provision":
		return cmdProvision(args[1:], e)
	case "labels":
		return cmdLabels(args[1:], e)
	}
	fmt.Fprintf(e.stderr, "deskfleet: unknown subcommand %q\n\n%s\n", args[0], usage)
	return exitUsage
}

// readCredentialFile reads a human-supplied credential file under the READ-side custody
// standard (deskkit.VerifyCustodyOwnerOnly — any failure refuses). The value is returned for
// use as a request header only; nothing here formats it.
func readCredentialFile(flagName, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required for a real run (a FILE holding the credential — never "+
			"a flag value or an environment variable)", flagName)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", flagName, path, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s %s is not a regular file", flagName, path)
	}
	if err := deskkit.VerifyCustodyOwnerOnly(path, fi); err != nil {
		return "", fmt.Errorf("%s: %v", flagName, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", flagName, path, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("%s %s is empty", flagName, path)
	}
	return tok, nil
}

// gitlabAPIBase reads GITLAB_API_BASE at call time. There is deliberately no default: GitLab
// is commonly self-hosted, and a guessed host would receive a live credential.
func gitlabAPIBase(e *env) (string, error) {
	base := strings.TrimSpace(e.getenv("GITLAB_API_BASE"))
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
