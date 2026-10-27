package main

// desktoken.go — the desktoken CLI: a thin, path-printing adapter over the role-credential
// provider (internal/credential).
//
// The mint-or-reuse logic, the install-id caches, the JWT, the token-cache custody checks and
// the GitLab rotation all live in the provider, which the in-process lookup
// (deskkit.RoleTokenForOwner) runs too. This file parses the command line, builds the request
// from the SAME deskkit snapshot builder the in-process caller uses, prints the PATH the
// provider returns (never the value), and writes the invocation's one audit row exactly as it
// always has. Nothing here re-implements a check the provider makes.

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/credential"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// validRoles is the fixed set of desk roles, owned by the provider (credential.KnownRoles):
// a role's custody is parameterised by its name, so the CLI and the in-process lookup admit
// exactly the same set. cell-issues and release-runner are selectable only by name, never a
// loop's default; auditor is the read-only hardening identity.
var validRoles = credential.KnownRoles()

// auditCtx accumulates fields for ONE audit line per invocation.
// finalize is deferred so exactly one line is written.
type auditCtx struct {
	verb       string
	role       string
	appName    string
	installID  string
	appID      string
	detail     string
	argsDigest string
	// suppress marks an invocation that performed NO ACT — the cache-reuse path, which
	// contacts nothing, mints nothing and changes nothing. Its row was the ledger's single
	// largest contributor (145,639 reuse rows against 2,583 real mints on one measured
	// host, #1035) and recorded a no-op. It is honoured on the SUCCESS path only: a reuse
	// that fails still writes its row, because a refusal is an act the ledger must carry.
	//
	// Nothing downstream reads a desktoken row: no budget, no breaker, no idempotency
	// decision and no gate consults one — desktoken does not call AllowWrite at all — so
	// this removes forensic detail about invocations that did nothing, and no control's
	// input. Per-invocation visibility, reuses included, is what brief 23's opt-in local
	// perf record is for.
	suppress bool
	// recorded is set when the provider handed this invocation its record (a mint, a
	// rotation, a custody read, or a refusal past the guard). A provider failure with no
	// record is the provider's own guard refusing, which wrote its own row — so finalize
	// skips (skip), exactly as a refusal by run()'s guard writes no desktoken row.
	recorded bool
	skip     bool
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "desktoken",
		Verb:       a.verb,
		ArgsDigest: a.argsDigest,
		Repo:       "", // token minting is repo-independent
		Result:     result,
		Detail:     detail,
	})
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	// A help screen is not an invocation of the verb, so it appends NO row. The ledger this
	// would land in is append-only, never rotated, and counted per tool for the write budget
	// and the circuit breaker (deskkit/audit.go, ratelimit.go) — see helprequest.go.
	if deskkit.IsHelpRequest(err) || a.skip {
		return
	}
	if err == nil {
		if a.suppress {
			return // a cache reuse performed no act — see auditCtx.suppress
		}
		a.log(deskkit.ResultOK, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	case deskkit.ExitUnverifiable:
		result = deskkit.ResultUnverifiable
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// adapters wires the provider to this invocation: deskkit's controls (the same ones the
// in-process lookup gets), the transport, and an Audit adapter that FOLDS the provider's
// record into this invocation's one row instead of writing a second one. The row itself is
// still written by finalize, so its tool, verb, digest and result are unchanged.
func (a *auditCtx) adapters(client *http.Client) credential.Adapters {
	return deskkit.CredentialAdapters(func(r credential.Record) {
		a.recorded = true
		if r.Verb != "" {
			a.verb = r.Verb
		}
		if r.Err == nil {
			a.detail = r.Detail
		}
	}, client)
}

// providerFailed returns the provider's failure as the desk-tools error the CLI has always
// returned, marking the row skipped when the failure carried no record (the provider's guard
// refused and recorded itself).
func (a *auditCtx) providerFailed(err error) error {
	if !a.recorded {
		a.skip = true
	}
	return deskkit.CredentialError(err)
}

// httpClient is a test hook — production code uses http.DefaultClient.
var httpClient = http.DefaultClient

// --- helpers --------------------------------------------------------------------

// roleEnvPrefix converts a role name to its env-var prefix:
// "issue-loop" → "ISSUE_LOOP", "reviewer" → "REVIEWER".
func roleEnvPrefix(role string) string {
	return strings.ToUpper(strings.ReplaceAll(role, "-", "_"))
}

// appNameFor resolves the App-NAME a role mints as — the role→App indirection
// that lets N roles mint with M<N Apps without a symlink or a copied
// key. It is the stem for the PEM file (`<app-name>.pem`), the App ID key and the install
// ID key. Absent a binding it is `<role>-app`, byte-identical to the pre-binding layout.
func appNameFor(role string) string {
	return deskkit.AppBinding(role)
}

// effectiveBindings renders the role→App bindings for every valid role, sorted,
// as a single `role=app-name` space-separated string for the --version echo
// (Verify rows 2 and 3 read this). It resolves each role through appNameFor, so
// it reports exactly the App each role would mint as.
func effectiveBindings() string {
	roles := roleNames() // sorted
	parts := make([]string, 0, len(roles))
	for _, r := range roles {
		parts = append(parts, r+"="+appNameFor(r))
	}
	return strings.Join(parts, " ")
}

// roleNames returns the valid roles as a sorted slice for error messages.
func roleNames() []string {
	out := make([]string, 0, len(validRoles))
	for r := range validRoles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// home expands a leading "~/" to the user's home directory. No-op if not ~/.
func home(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return p // will fail downstream with a clear path error
		}
		return filepath.Join(h, p[2:])
	}
	return p
}

// installationInfo is the relevant fields from GET /app/installations (the provider's type).
type installationInfo = credential.InstallationInfo

// tokenResult is the relevant fields of the access_tokens response (the provider's type).
type tokenResult = credential.TokenResult

// parseOwner extracts the owner from a repo slug (owner/name) or uses the raw
// string as the owner if there's no slash.
func parseOwner(repo string) string {
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[:i]
	}
	return repo
}

// placeholderOwner is the account the shipped topology carries as a placeholder.
// It is NOT a usable default owner: when --repo is absent the account must be
// resolved from the configured roster, and if that resolves nothing the mint
// fails closed NAMING this placeholder rather than silently resolving an
// installation for it. The old default (owner := placeholderOwner whenever --repo
// was absent) surfaced in a configured deployment as a bare rc=6 that read like a
// bad credential, when the real cause was that the owner had never been resolved.
const placeholderOwner = "example-org"

// configuredOwners returns the sorted, de-duplicated set of account owners across
// the configured allowed-repo set (ASSAY_ALLOWED_REPOS) — the same
// write-authorisation roster the other desk verbs resolve the org from. A
// deployment that names all its repos under one account yields one owner; the
// unconfigured/placeholder tree yields none.
func configuredOwners() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range deskkit.AllowedRepos() {
		o := parseOwner(r)
		if o == "" || seen[o] {
			continue
		}
		seen[o] = true
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}

// resolveMintOwner determines the account whose App installation the token is
// minted against.
//
//   - --repo given: its owner half, verbatim (unchanged behaviour).
//   - --repo absent: the owner is resolved from the SAME configured source the
//     other desk verbs read — the allowed-repo roster (ASSAY_ALLOWED_REPOS).
//     Exactly one configured owner is used. Zero or several fail CLOSED with a
//     message that names what to do (exit 6), never a silent mint against the
//     shipped placeholder that reads back as an auth failure.
func resolveMintOwner(repo string) (string, error) {
	if strings.TrimSpace(repo) != "" {
		return parseOwner(repo), nil
	}
	owners := configuredOwners()
	switch len(owners) {
	case 1:
		return owners[0], nil
	case 0:
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"COULD-NOT-CHECK: no --repo was given and the allowed-repo roster (%s) names no account, so "+
				"the owner cannot be resolved. The mint would otherwise fall back to the shipped placeholder "+
				"%q, which is not a real installation — a mint against it fails as if the credential were bad "+
				"when the real cause is that no owner is configured. Pass --repo <owner>/<name>, or set %s in %s.",
			deskkit.EnvAllowedRepos, placeholderOwner, deskkit.EnvAllowedRepos, deskkit.ConfigHomePath()), nil)
	default:
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"COULD-NOT-CHECK: no --repo was given and the allowed-repo roster names more than one account "+
				"(%s), so the owner is ambiguous. An installation token is per account and this process will "+
				"not guess which one — pass --repo <owner>/<name> to name the account.",
			strings.Join(owners, ", ")), nil)
	}
}

// --- provider primitives the coverage verb reuses ---------------------------------

// parsePrivateKey tries PKCS1 then PKCS8 decoding of a PEM-encoded RSA key.
func parsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	return credential.ParsePrivateKey(pemData)
}

// buildJWT constructs an RS256-signed App JWT valid for 9 minutes.
func buildJWT(appID string, now time.Time, key *rsa.PrivateKey) (string, error) {
	return credential.BuildJWT(appID, now, key)
}

// exchangeJWT POSTs the signed JWT to GitHub's installation access_tokens endpoint through
// the provider's one implementation of that call.
func exchangeJWT(jwt, installID string, clients ...*http.Client) (*tokenResult, error) {
	return credential.ExchangeJWT(authHTTP(clients), jwt, installID)
}

// --- TTL reporting --------------------------------------------------------------

// printTTL reports the remaining life and age of the cached token and returns an error if
// no cache file exists. The remaining life is measured to the cache's ACTUAL expiry — the
// forge's own expires_at, recorded at mint (`expiry=recorded`) — or, for a cache minted
// before expiry was recorded, to the conservative reuse bound mtime + 50m (`expiry=bound`),
// never to a lifetime nobody observed.
func printTTL(tokenPath string) error {
	fi, err := os.Stat(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return deskkit.Unverifiable("no cached token at "+tokenPath, nil)
		}
		return deskkit.Unverifiable("cannot stat cached token", err)
	}
	expires, recorded, eerr := credential.CacheExpiry(tokenPath)
	if eerr != nil {
		return deskkit.Unverifiable("cannot read cached token expiry", eerr)
	}
	now := time.Now()
	age := now.Sub(fi.ModTime())
	remaining := int(expires.Sub(now).Minutes())
	if remaining < 0 {
		remaining = 0
	}
	source := "bound"
	if recorded {
		source = "recorded"
	}
	fmt.Printf("TTL=%dm age=%dm path=%s expiry=%s\n", remaining, int(age.Minutes()), tokenPath, source)
	return nil
}

// --- flag parsing helper --------------------------------------------------------

// parseInterspersed parses fs allowing flags before OR after positional args.
// Copied from deskwt since it's unexported.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	return positionals, nil
}

// --- key-path resolution (#794) --------------------------------------------------

// resolvePEMPath finds the App private key for an App-NAME. It is deskkit's resolver — the
// same one the provider's request snapshot uses — so the coverage verb and the mint can never
// look for a key in different places. See deskkit.ResolveAppPEMPath for the order and the
// deferred not-found error.
func resolvePEMPath(appName, prefix string) (string, error) {
	return deskkit.ResolveAppPEMPath(appName, prefix)
}

// permsPath is the sidecar recording what GitHub GRANTED the installation the token was
// minted for (#571).
func permsPath(tokenPath string) string { return credential.PermsPath(tokenPath) }

// tokenPathNotice is the one-line NOTICE that accompanies every token path this tool prints.
//
// The tool's whole output contract is "a PATH, never the value", and until this line existed
// nothing in the output said so. A caller that used stdout as the credential — the obvious
// reading of a tool called desktoken — got `401 Bad credentials` from its next forge call,
// three processes downstream, with nothing naming the cause. The notice puts the cause next
// to the effect.
//
// It goes to STDERR, deliberately: stdout is a machine surface that callers pipe into a
// variable, and a second line on it would break every one of them. There is no --print-token
// flag and this notice is not a step towards one: printing the value on stdout is the exact
// disclosure this tool's 0600 cache file exists to avoid, and `cat "$(desktoken …)"` already
// serves the caller without putting the credential in this process's own output.
const tokenPathNotice = `desktoken: NOTICE — stdout above is the token FILE PATH, not the token itself. Using it ` +
	`directly as a credential is what produces a bare "401 Bad credentials" from the next forge call. ` +
	"Read the value with: cat \"$(desktoken <role> --repo <owner/name>)\""

// printTokenPath writes the token FILE PATH on stdout — never the token — and the notice
// above on stderr. Both print sites go through it so the two can never drift apart.
func printTokenPath(path string) {
	fmt.Println(path)
	fmt.Fprintln(os.Stderr, tokenPathNotice)
}

// beforeTokenCacheRename is a test seam: when set, the provider's cache write calls it after
// the new token is fully written and synced to the temp file and before the rename that
// publishes it, so a test can observe the cache path at the one instant a torn write would be
// visible.
var beforeTokenCacheRename func()

// writeTokenCacheAtomic publishes token at path (0600) so a concurrent reader sees either the
// previous contents or the new token, never an empty or partial file. It is the provider's
// write (credential.WriteTokenCache) with this package's test seam.
func writeTokenCacheAtomic(path, token string) error {
	return credential.WriteTokenCache(path, token, beforeTokenCacheRename)
}

// privateKeyModeOK reports whether a private-key file's mode keeps the key away from OTHER
// principals (the provider's rule: no other-read, no group/other write; group-read admitted
// for a Kubernetes Secret volume read through fsGroup).
func privateKeyModeOK(mode os.FileMode) bool { return credential.PrivateKeyModeOK(mode) }

// checkPrivateKeyMode is the fail-closed wrapper over privateKeyModeOK: it returns the exit-6
// refusal naming the rule and the observed mode, or nil.
func checkPrivateKeyMode(pemPath string, mode os.FileMode) error {
	if e := credential.CheckPrivateKeyMode(pemPath, mode); e != nil {
		return deskkit.CredentialError(e)
	}
	return nil
}

// --- main entry point -----------------------------------------------------------

// run is the CLI entry point. It returns an exit code.
func run(args []string) int { return runSelected(args, nil) }

func runSelected(args []string, sink func(perfrec.Context)) int { return runMeasured(args, sink, nil) }

func runMeasured(args []string, sink func(perfrec.Context), measurement *perfrec.Session) int {
	// The global diagnostic switch, taken BEFORE any flag parsing so `--trace` works
	// alongside this verb's bare-role-name positional grammar (which would otherwise refuse
	// it as an undefined flag) and is invisible to the FlagSets below.
	args = deskkit.TakeTraceFlag(args)

	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("desktoken sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		// Echo the EFFECTIVE role→App bindings so a two-App
		// deployment can see, without minting, which App each role resolves to.
		// One `bindings=` line, `role=app-name` per role, sorted. Absent binding
		// prints the default `<role>-app`. This is a pure read (env + apps.env).
		fmt.Printf("bindings=%s\n", effectiveBindings())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// TIER ONE of the help retrofit (deskkit/helprequest.go). A SUBCOMMAND help request —
	// `desktoken <sub> --help` — is a request for a help screen, not an invocation of the verb, and
	// until this it was recorded as `refused: bad flags: flag: help requested`: exit 5 plus one
	// row appended to a ledger the write budget counts and nothing rotates (1,043 such rows
	// measured on one operating desk host over 32 days). It returns HERE, before Guard, and
	// writes nothing. HelpOnly matches only the unambiguous single-token shape, so a `--help`
	// that is another flag's VALUE cannot be mistaken for one; every wider spelling falls
	// through to cmdToken / cmdCoverage's own parse, which hands flag.ErrHelp to
	// deskkit.IsHelpRequest (tier two): it prints usage, exits 0, and the finalizer writes no row.
	if deskkit.HelpOnly(args) {
		fmt.Fprintln(os.Stderr, usage)
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its
	// own result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	// `coverage` is a distinct read-only verb (list the repos a role's App
	// installations can see). No role is named "coverage", so the dispatch is
	// unambiguous.
	if args[0] == "coverage" {
		cerr := cmdCoverageMeasured(args[1:], measurement, sink)
		if deskkit.IsHelpRequest(cerr) {
			fmt.Fprintln(os.Stderr, usage)
			return deskkit.ExitOK
		}
		deskkit.ReportError(os.Stderr, cerr)
		return deskkit.ExitCodeOf(cerr)
	}

	err := cmdTokenMeasured(args, measurement, sink)
	if deskkit.IsHelpRequest(err) {
		// TIER TWO terminus: print the help screen the operator asked for, exit 0.
		fmt.Fprintln(os.Stderr, usage)
		return deskkit.ExitOK
	}
	// The shared exit path. With DESK_TRACE off this is byte-identical to the
	// fmt.Fprintln(os.Stderr, err.Error()) it replaces.
	deskkit.ReportError(os.Stderr, err)
	return deskkit.ExitCodeOf(err)
}

// cmdToken implements the mint-or-reuse logic.
func cmdToken(args []string, sinks ...func(perfrec.Context)) error {
	return cmdTokenMeasured(args, nil, sinks...)
}

func cmdTokenMeasured(args []string, measurement *perfrec.Session, sinks ...func(perfrec.Context)) (err error) {
	ac := &auditCtx{verb: "mint", argsDigest: deskkit.ArgsDigest(os.Args[1:])}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("desktoken", flag.ContinueOnError)
	var context perfrec.Context
	selected := cli.GoPerformance(fs, func(c perfrec.Context) {
		context = c
		if measurement != nil {
			measurement.SelectContext(c)
		}
		for _, sink := range sinks {
			if sink != nil {
				sink(c)
			}
		}
	})
	defer selected()
	fs.SetOutput(new(strings.Builder))
	repo := fs.String("repo", "", "repo slug (owner/name) for install auto-pick")
	forge := fs.String("forge", "", "forge backend: empty/github (default) mints a GitHub App installation token; gitlab rotates the role's PAT in place (rotate-on-mint custody)")
	ttl := fs.Bool("ttl", false, "print remaining TTL of cached token (does not mint)")
	fresh := fs.Bool("fresh", false, "delete any cached token and its .perms/.owner sidecars before minting, and bypass the install-id caches — forces a fresh mint after a GitHub-App permission change (the cached token otherwise carries the old grant for up to the ~50-min reuse window)")
	noRotate := fs.Bool("no-rotate", false, "read-only credential lookup: verify the role's existing custody and print its PATH without performing a destructive rotation. GitLab: skips the self-rotation (and its network contact) entirely — the shape a read-only or dry-run verb asks for. GitHub: no effect, the App mint is cached and non-destructive already, so the flag is accepted for callers that do not know the forge before they ask")

	positionals, perr := parseInterspersed(fs, args)
	selected()
	authClient := measuredAuth(context, measurement)
	if perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches the parser as flag.ErrHelp. A help
		// screen is not a refusal and writes no audit row (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
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

	// Forge dispatch. gitlab takes an entirely different custody path — rotate-on-mint
	// against an existing PAT file — and needs no App PEM or App ID, so it dispatches BEFORE
	// any GitHub-credential resolution.
	//
	//   - An EXPLICIT --forge is authoritative: `gitlab` takes the PAT path, `github` forces
	//     the App mint even against a GitLab-resolved repo (an operator override).
	//   - With NO --forge (the common shell + the shape deskfile/pr-review-desk invoke), the
	//     forge is RESOLVED from --repo: a repo that AFFIRMATIVELY resolves to GitLab takes the
	//     PAT path, so `desktoken reviewer --repo <gitlab-slug>` no longer dies in the GitHub
	//     App mint with `no App ID for App "reviewer-app"` — the credential a PAT-backed GitLab
	//     bot never provisions (#772 named it, #798 closes it). A GitHub or
	//     could-not-check resolution falls through to the App mint, so every repo whose forge
	//     cannot be POSITIVELY resolved keeps its exact pre-#798 behaviour — the same
	//     three-state fall-through requireGitHubForge makes on the deskpost side.
	//
	// --no-rotate selects the READ-ONLY shape of whichever path is taken. It is passed as
	// `rotate=!noRotate` rather than branching here, so the custody checks a read-only lookup
	// makes stay byte-identical to the rotating one — a read verb that accepted custody the
	// write verb would refuse is a difference an operator would only discover at the moment
	// it mattered. On the GitHub path it is a no-op: mint-or-reuse is already non-destructive.
	switch strings.ToLower(strings.TrimSpace(*forge)) {
	case "":
		if gitlabRepoResolved(*repo) {
			return cmdGitLabRotate(role, ac, !*noRotate, authClient)
		}
		// fall through to the GitHub App-token mint path.
	case "github":
		// fall through to the GitHub App-token mint path.
	case "gitlab":
		return cmdGitLabRotate(role, ac, !*noRotate, authClient)
	default:
		return deskkit.Refused("unknown --forge " + *forge + "; valid: github (default), gitlab")
	}

	// GitHub App installation token: build the request from the SAME snapshot builder the
	// in-process lookup uses (deskkit.GitHubCredentialRequest — the role→App binding, the key
	// path with its deferred not-found, the App ID, the <PREFIX>_INSTALL_ID and <PREFIX>_TOKEN
	// overrides, the search path), then run the provider. Every cache, custody, key and
	// exchange step — and every refusal message — is the provider's (internal/credential).
	req, rerr := deskkit.GitHubCredentialRequest(role)
	if rerr != nil {
		return rerr
	}
	ac.appName = req.AppName
	ac.appID = req.AppID

	// The account this mint is for. With --repo absent it is resolved from the configured
	// roster (resolveMintOwner) rather than defaulting to the shipped placeholder; an
	// unresolvable owner fails closed — unless an explicit <PREFIX>_INSTALL_ID already answers
	// WHICH installation, in which case the account only feeds the owner sidecar and its
	// absence costs one installations call next time, never a wrong answer.
	owner, oerr := resolveMintOwner(*repo)
	if oerr != nil {
		if req.InstallID == "" {
			return oerr
		}
		owner = ""
	}
	req.Owner = owner
	req.Fresh = *fresh
	req.PathOnly = *ttl
	req.BeforeRename = beforeTokenCacheRename

	cred, gerr := credential.GitHub(req, ac.adapters(authHTTP([]*http.Client{authClient})))
	if gerr != nil {
		return ac.providerFailed(gerr)
	}
	ac.installID = cred.InstallID

	// --ttl: report cached token age and expiry and exit (no mint).
	if *ttl {
		return printTTL(cred.Path)
	}

	// Output only the token file path — never the token value.
	printTokenPath(cred.Path)
	if cred.Outcome == credential.OutcomeReused {
		ac.suppress = true // no network call, no mint, nothing changed — no audit row
	}
	return nil
}
