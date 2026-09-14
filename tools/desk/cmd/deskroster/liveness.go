package main

// liveness.go — `deskroster liveness --repo OWNER/NAME`: a read-only NOTICE surface that
// asks GitHub what it currently says about every trusted login the roster configures, and
// prints a NOTICE when something changed (deleted, renamed, reclaimed). It never touches
// TrustedAuthor/TrustedHumanAuthor/Blessed's pass/fail verdict — see
// internal/deskkit/trustliveness.go for the classifier and internal/deskkit/trust.go's
// cross-reference doc-comment.
//
// The --repo flag exists ONLY to resolve a token through the same forgeFor seam
// `deskroster list` already runs its PR reads through (forge.go): GET /users/{login} is
// host-level and does not depend on which repo minted the credential, so this never mints a
// second kind of token or widens forgeFor's scope.

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
				"Read-only: asks GitHub what it CURRENTLY says about every trusted login the roster\n"+
				"configures (Humans, Bless, Bots), and prints a NOTICE line for each one that is not\n"+
				"exactly what the roster expects (deleted, renamed, reclaimed, or unpinned). It never\n"+
				"gates, never auto-revokes, and never changes TrustedAuthor/TrustedHumanAuthor's\n"+
				"verdict — see the roster liveness section of tools/desk/README.md. GitHub-only in\n"+
				"this version; GitLab account-liveness is untracked follow-up (medici-finance/assay#933).")
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
	gf, ok := f.(*deskkit.GitHubForge)
	if !ok {
		fmt.Println("NOTICE: could-not-check — " + *repo + " is not GitHub-backed: account-liveness is " +
			"GitHub-only in this version; GitLab account-liveness is untracked follow-up (medici-finance/assay#933)")
		fmt.Fprintln(os.Stderr, "liveness: 0 identities checked (non-GitHub forge), 1 notice")
		return nil
	}
	fetcher := &deskkit.HTTPAccountFetcher{Token: gf.Token, BaseURL: gf.BaseURL, Client: gf.Client}

	identities := deskkit.RosterIdentities(cfg)
	findings := deskkit.CheckRosterLiveness(fetcher, identities)
	notices := deskkit.RenderLivenessNotices(findings)
	for _, line := range notices {
		fmt.Println(line)
	}
	fmt.Fprintf(os.Stderr, "liveness: %d identities, %d notices\n", len(identities), len(notices))
	return nil
}
