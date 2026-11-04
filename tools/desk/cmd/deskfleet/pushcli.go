package main

// pushcli.go — the `push-token` command tree (forge-gitlab/18): two modes, each with its own
// credential-file flag and no flag that takes a credential VALUE. The only environment sources
// are GITLAB_API_BASE (a URL) and, for rotate, the desk loop's session marker (the advisory
// refusal); neither is a credential.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

const pushTokenLong = `push-token rotates the board-push credential STATUSGEN_PUSH_TOKEN (a write_repository-only
project access token at Maintainer, held in a masked, protected CI/CD variable) in ONE process,
and checks it read-only. The new value is never printed, logged, or put in argv, the
environment or a file (docs/adopting-assay-gitlab.md §2c "Rotating it").

  deskfleet push-token rotate --project <path-or-id> --rotator-token-file <file> --token-name <name>
                              [--days N] [--min-days N] [--create] [--dry-run]
  deskfleet push-token check  --project <path-or-id> --reader-token-file <file> --token-name <name>
                              [--min-days N]

Run 'deskfleet push-token <mode> --help' for a mode's options.

Exit: 0 ok · 1 a step failed · 2 usage · 5 refused · 6 could-not-check.`

const pushRotateLong = `rotate replaces STATUSGEN_PUSH_TOKEN: it reads the project-level variable and the one active
project access token named --token-name (refusing, before any write, unless the variable is
masked, protected and scoped *, the token is scoped exactly [write_repository] at level 40, and
the default branch is protected), rotates that token (or, with --create and none active,
creates one), writes the new value into the variable with masked, protected and scope * sent
explicitly, reads the variable back, and proves the new token by three probes: by-id (under
the rotator credential), self (under the new token) and push (a receive-pack advertisement on
the API host, under the new token). A read-back that disagrees revokes the new token at once
and confirms the revoke ("contained: revoked <id>", exit 1); a revoke that cannot be confirmed
prints "containment-failed: revoke <id> by hand" (exit 6).

  deskfleet push-token rotate --project <path-or-id> --rotator-token-file <file> --token-name <name>
                              [--days N] [--min-days N] [--create] [--dry-run]

The rotator credential (Maintainer, api scope) is read from --rotator-token-file only, which
must pass the owner-only custody check. A real run requires GITLAB_API_BASE ending in /api/v4.
rotate refuses while the desk loop's session marker DESK_LOOP is set (advisory only).
--dry-run enumerates every step and makes ZERO network calls.

Exit: 0 rotated and proven · 1 a step failed · 2 usage · 5 refused · 6 could-not-check.`

const pushCheckLong = `check is the read-only monitor between rotations: it finds the one active project access token
named --token-name and runs the by-id probe (active, not revoked, scopes exactly
[write_repository], at least --min-days left). It sends no POST, PUT or DELETE.

  deskfleet push-token check  --project <path-or-id> --reader-token-file <file> --token-name <name>
                              [--min-days N]

The reader credential (read-only API scope at Maintainer is enough) is read from
--reader-token-file only, which must pass the owner-only custody check.

Exit: 0 healthy · 1 below threshold, revoked, inactive, wrongly scoped, none or several · 2 usage
· 5 refused · 6 could-not-check.`

func pushRotateBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "project", Kind: cli.String, Flag: "project", Usage: "GitLab project (path or id) whose variable to rotate"},
		{Key: "rotator-token-file", Kind: cli.String, Flag: "rotator-token-file",
			Usage: "FILE holding the rotator credential (Maintainer, api; owner-only; never a flag value)"},
		{Key: "token-name", Kind: cli.String, Flag: "token-name", Usage: "name of the project access token to rotate"},
		{Key: "days", Kind: cli.Int, Flag: "days", Default: pushRotateDefaultDays, Usage: "new token lifetime in days (1-90)"},
		{Key: "min-days", Kind: cli.Int, Flag: "min-days", Default: pushDefaultMinDays,
			Usage: "minimum days the proof demands remain (below --days)"},
		{Key: "create", Kind: cli.Bool, Flag: "create",
			Usage: "create the token and/or the variable when absent (after an expiry or a containment)"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Usage: "enumerate every step; make zero network calls"},
		apiBaseBinding(),
		sessionMarkerBinding(),
	}
}

func pushCheckBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "project", Kind: cli.String, Flag: "project", Usage: "GitLab project (path or id)"},
		{Key: "reader-token-file", Kind: cli.String, Flag: "reader-token-file",
			Usage: "FILE holding a read-only reader credential (owner-only; never a flag value)"},
		{Key: "token-name", Kind: cli.String, Flag: "token-name", Usage: "name of the project access token to check"},
		{Key: "min-days", Kind: cli.Int, Flag: "min-days", Default: pushDefaultMinDays,
			Usage: "alarm below this many days left"},
		apiBaseBinding(),
	}
}

// sessionMarkerBinding is the desk loop's session marker: environment-only, read for the
// advisory refusal alone. It is not a credential.
func sessionMarkerBinding() cli.Binding {
	return cli.Binding{Key: "session-marker", Kind: cli.String, Env: []string{pushSessionMarker},
		Usage: "the desk loop's session marker (environment only; rotate refuses while it is set)"}
}

func newPushTokenCLI(e *env) *cobra.Command {
	push := &cobra.Command{
		Use:   "push-token rotate|check [flags]",
		Short: "rotate (or check) the board-push credential STATUSGEN_PUSH_TOKEN in one process",
		Long:  pushTokenLong,
		Args:  noPositional,
	}
	push.RunE = func(cmd *cobra.Command, _ []string) error {
		cmd.SetOut(cmd.ErrOrStderr())
		_ = cmd.Help()
		return exitErr(exitUsage)
	}
	push.AddCommand(
		pushModeCLI(e, "rotate", "rotate the token and install it into the variable, then prove it", pushRotateLong,
			pushRotateBindings()),
		pushModeCLI(e, "check", "read-only: the active token's record against --min-days", pushCheckLong,
			pushCheckBindings()),
	)
	return push
}

func pushModeCLI(e *env, mode, short, long string, bindings []cli.Binding) *cobra.Command {
	cmd := &cobra.Command{
		Use:   mode + " --project <path-or-id> --token-name <name> [flags]",
		Short: short,
		Long:  long,
		Args:  noPositional,
	}
	set := cli.Declare(cmd, bindings...)
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		v, err := set.Resolve(cli.Inputs{LookupEnv: e.lookupEnv})
		if err != nil {
			fmt.Fprintf(e.stderr, "deskfleet push-token %s: %v\n", mode, err)
			return exitErr(exitUsage)
		}
		if onResolve != nil {
			onResolve(v)
		}
		o := pushOpts{mode: mode, project: v.String("project"), tokenName: v.String("token-name"),
			minDays: v.Int("min-days"), apiBase: v.String("gitlab-api-base")}
		if mode == "rotate" {
			o.credFlag, o.credFile = "--rotator-token-file", v.String("rotator-token-file")
			o.days, o.create, o.dryRun = v.Int("days"), v.Bool("create"), v.Bool("dry-run")
			o.sessionTag = v.String("session-marker")
		} else {
			o.credFlag, o.credFile = "--reader-token-file", v.String("reader-token-file")
		}
		return codeErr(pushTokenFn(o, e))
	}
	return cmd
}
