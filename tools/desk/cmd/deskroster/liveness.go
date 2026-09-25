package main

// liveness.go — `deskroster liveness --repo OWNER/NAME`: a read-only NOTICE surface that
// asks the repo's own forge what it currently says about every trusted login the roster
// configures, and prints a NOTICE when something changed (deleted, renamed, reclaimed,
// suspended). It never touches TrustedAuthor/TrustedHumanAuthor/Blessed's pass/fail verdict
// — see internal/deskkit/trustliveness.go for the classifier and internal/deskkit/trust.go's
// cross-reference doc-comment.
//
// The --repo flag exists ONLY to resolve a token through the same forgeFor seam
// `deskroster list` already runs its PR reads through (forge.go): GET /users/{login} (or,
// on GitLab, GET /users?username=) is host-level and does not depend on which repo minted
// the credential, so this never mints a second kind of token or widens forgeFor's scope.
//
// FORGE DISPATCH (assay#1667). forgeFor resolves a *deskkit.Forge value; this file type-
// switches on its CONCRETE type to pick the matching AccountFetcher/identity-enumerator
// pair — never constructs a new GitHubForge/GitLabForge literal itself (that stays confined
// to deskkit.ForgeFor: TestForgeSingleConstructionSite), only reads the already-minted
// Token/BaseURL/Client fields off the value forgeFor returned. A forge this file does not
// recognise (neither GitHub nor GitLab) is a could-not-check NOTICE, never a refusal or
// silence — the roster itself may be perfectly configured; only that repo's forge has no
// liveness implementation yet.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func cmdLiveness(args []string) error {
	fs := flag.NewFlagSet("liveness", flag.ContinueOnError)
	repo := fs.String("repo", "", "OWNER/NAME — resolves a minted token to read GitHub with (host-level read, any configured repo works)")
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fs.SetOutput(os.Stdout)
			fmt.Fprintln(os.Stdout, "deskroster liveness --repo OWNER/NAME\n\n"+
				"Read-only: asks the repo's own forge what it CURRENTLY says about every trusted\n"+
				"login the roster configures — on GitHub: Humans, Bless, Bots; on GitLab: the\n"+
				"forge-qualified bot/service-account identities (Config.BotIdents) — and prints a\n"+
				"NOTICE line for each one that is not exactly what the roster expects (deleted,\n"+
				"renamed, reclaimed, suspended, or unpinned). It never gates, never auto-revokes, and\n"+
				"never changes TrustedAuthor/TrustedHumanAuthor's verdict — see the roster liveness\n"+
				"section of tools/desk/README.md. GitHub and GitLab are supported; any other forge\n"+
				"prints a could-not-check NOTICE naming the gap.")
			fs.PrintDefaults()
			return nil
		}
	}
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("liveness: " + err.Error())
	}
	if fs.NArg() > 0 {
		return deskkit.Refused(fmt.Sprintf("refused: `liveness` takes no positional arguments (got %q)", fs.Arg(0)))
	}
	if strings.TrimSpace(*repo) == "" {
		return deskkit.Refused("liveness requires --repo OWNER/NAME to resolve a minted token to read GitHub with")
	}

	cfg := deskkit.EffectiveConfig()
	if !cfg.Configured() {
		return deskkit.Refused("liveness: the roster is unconfigured — nothing was checked. " +
			"This is a refusal, not a clean 0-finding run: configure ASSAY_BLESS_LOGIN/ASSAY_TRUSTED_LOGINS " +
			"(or the equivalent config-home file) before running the liveness check.")
	}

	f, _, ferr := forgeFor(*repo)
	if ferr != nil {
		return deskkit.Unverifiable("liveness: cannot resolve a Forge for "+*repo, ferr)
	}

	var (
		fetcher    deskkit.AccountFetcher
		identities []deskkit.RosterIdentity
	)
	switch forge := f.(type) {
	case *deskkit.GitHubForge:
		fetcher = &deskkit.HTTPAccountFetcher{Token: forge.Token, BaseURL: forge.BaseURL, Client: forge.Client}
		identities = deskkit.RosterIdentities(cfg)
	case *deskkit.GitLabForge:
		fetcher = &deskkit.HTTPGitLabAccountFetcher{Token: forge.Token, BaseURL: forge.BaseURL, Client: forge.Client}
		identities = deskkit.GitLabRosterIdentities(cfg)
	default:
		fmt.Println("NOTICE: could-not-check — " + *repo + " is served by a forge account-liveness has no " +
			"implementation for (neither GitHub nor GitLab)")
		fmt.Fprintln(os.Stderr, "liveness: 0 identities checked (unrecognised forge), 1 notice")
		return nil
	}

	findings := deskkit.CheckRosterLiveness(fetcher, identities)
	notices := deskkit.RenderLivenessNotices(findings)
	for _, line := range notices {
		fmt.Println(line)
	}
	fmt.Fprintf(os.Stderr, "liveness: %d identities, %d notices\n", len(identities), len(notices))
	return nil
}
