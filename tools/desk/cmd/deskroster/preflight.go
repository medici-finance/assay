package main

// preflight.go — the `deskroster preflight` verb.
//
// It lives on deskroster because the roster is already the desk's "who is
// operating, as what" surface: a session declares its role here, and the
// envelope check is the question "can that role actually operate?" asked before
// the session declares anything.
//
// The verb is a pure READ. It claims nothing, files nothing, and writes nothing
// to the roster — which is the point. A desk whose envelope is broken must NOT
// register work it cannot do.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const preflightUsage = `deskroster preflight — desk operating-envelope check, run BEFORE any work is claimed.

USAGE:
  deskroster preflight --role <role> [--root <dir>] [--repo <owner/name>]
                       [--remote <name>] [--branch <name>] [--verbose]

Runs five checks, each answering checked-clean / checked-failed / could-not-check
with a NAMED remediation:

  token-mint-cold        a token mints from a FRESH process with a scrubbed env   (#794 #567)
  app-scopes-vs-duties   the installation's grant covers the role's duties        (#571)
  write-transport        a READ-ONLY probe of the role's landing path             (#823)
  commit-identity        the commit email carries the BOT USER id, not the App id (#638)
  sibling-checkouts      the checkouts the QUEUED briefs declare are present      (#679)

A non-green result is COULD-NOT-RUN for the whole pass: one summary line, exit 6.
The desk stops — it does not claim work, burn a pass, or file an issue about its
own envelope (those issues already exist; the refs above are them).

A probe REJECTION is a STOP. It is never retried under another identity
(AGENTS.md, "Scope rejections") and this verb offers no way to.

Exit: 0 all checked-clean · 3 disabled · 5 refused (bad flags) · 6 preflight red.`

// cmdPreflight implements `deskroster preflight`.
//
// `--help` exits 0. That is a contract, not a nicety: an unknown subcommand
// exits 5, so "does this build of deskroster HAVE preflight?" is answerable by a
// consumer (a skill's boot section, CI) with one call and no output parsing.
func cmdPreflight(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(os.Stdout, preflightUsage)
		return nil
	}

	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	role := fs.String("role", "", "desk role this session acts as (reviewer|verifier|worker|desk|issue-loop|intake-loop)")
	root := fs.String("root", ".", "repo root whose queued briefs declare the sibling checkouts")
	remote := fs.String("remote", "origin", "git remote the role's landing path targets")
	branch := fs.String("branch", "", "landing branch (default: this worktree's current branch)")
	repo := fs.String("repo", "", "owner/name whose INSTALLATION the token is minted against (default: derived from the landing remote)")
	verbose := fs.Bool("verbose", false, "print every check, not just the summary line")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}
	if strings.TrimSpace(*role) == "" {
		return deskkit.Refused("preflight requires --role: the envelope is checked FOR a role, and " +
			"guessing which one is how a desk passes a preflight it never ran")
	}

	rep := deskkit.PreflightRequest{
		Role:    *role,
		Root:    *root,
		Repo:    *repo,
		Landing: deskkit.Landing{Dir: *root, Remote: *remote, Branch: *branch},
	}.Run()

	if *verbose {
		for _, c := range rep.Checks {
			fmt.Fprintf(os.Stdout, "%-22s %-16s %s\n", c.Name, c.State, c.Detail)
			if c.Remediation != "" {
				fmt.Fprintf(os.Stdout, "%-22s %-16s fix: %s%s\n", "", "", c.Remediation, refSuffix(c.Refs))
			}
		}
	}

	// ONE line, on stdout when green and via the error (stderr) when not.
	if err := rep.Err(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, rep.SummaryLine())
	return nil
}

func refSuffix(refs string) string {
	if refs == "" {
		return ""
	}
	return " [" + refs + "]"
}
