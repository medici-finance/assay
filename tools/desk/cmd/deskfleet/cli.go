package main

// cli.go — deskfleet's command tree (desktools-v2/23). Cobra, through the desk adapter
// (internal/cli), parses every verb, validates its arguments and prints help and the version;
// the handlers receive typed options and keep every domain check, custody decision and exit
// code they had. Nothing binds from a config file. The one environment source is
// GITLAB_API_BASE, declared on the two verbs that reach GitLab and read only when a verb runs:
// help and version read nothing.

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const rootLong = `deskfleet — Go-native GitLab fleet provisioning and forge-neutral fleet labels.

Verbs:
  provision  creates the seven role service accounts, their group memberships and (for an
             account created by this run) one PAT each, written owner-only to
             gitlab-<role>.token in --out-dir; with --project it also configures the
             project. --avatars-only sets the avatars of existing accounts.
  labels     creates the nine fleet labels on one GitHub repo or GitLab project.

Run 'deskfleet <verb> --help' for a verb's options.

Credentials are read from FILES only (--owner-token-file / --token-file), which must pass
the owner-only custody check; they are never accepted as a flag value or env variable, and
never printed — only paths are.

--dry-run  enumerates every action and makes ZERO network calls.

Exit: 0 ok · 1 a step failed · 2 usage · 5 refused.`

const provisionLong = `provision creates the seven role service accounts, their group memberships and (for an
account created by this run) one PAT each, written owner-only to gitlab-<role>.token in
--out-dir (default: the config home, where desktoken --forge gitlab <role> reads it). With
--project it also protects main, sets MR approvals, protects release tags, sets the pipeline
and discussion merge checks, creates the fleet labels, and mints the board-push credential:
a write_repository-only project access token at Maintainer, stored as the masked, protected
CI/CD variable STATUSGEN_PUSH_TOKEN (never a role-fleet PAT; where the forge cannot mint
one, a recorded failure naming the manual step). With --avatars-dir, each new
account sets <dir>/<role>.png as its own avatar, signed in as itself; without it (or with
--no-avatars) the avatar step is SKIPPED and named (no icon is fetched from the web).

  deskfleet provision --group <path-or-id> --prefix <name> --owner-token-file <file>
                      [--project <path-or-id>] [--out-dir <dir>] [--pat-expiry-days N]
                      [--avatars-dir <dir> | --no-avatars] [--dry-run]
  deskfleet provision --avatars-only --avatars-dir <dir> --prefix <name> [--out-dir <dir>] [--dry-run]

--avatars-only sets the avatars of existing accounts from their gitlab-<role>.token files
and touches nothing else.

A real run requires GITLAB_API_BASE (your REST v4 base, e.g. https://gitlab.example.com/api/v4)
— there is no default host. --dry-run enumerates every action and makes ZERO network calls.

The owner credential is read from --owner-token-file only, which must pass the owner-only
custody check. A link at that path is followed, and the custody check judges the file it
resolves to. A gitlab-<role>.token read back from --out-dir may only be a link to a file in
its own directory, the rule desktoken applies to that file.

Custody (the recorded ruling): a minted token whose owner-only read-back is INCONCLUSIVE is
kept with a WARNING naming the file; one that is definitely not owner-only stops the run.
A run that fails partway through minting stops, reports every token it minted (never the
value) for manual revocation, names every account it created with no token and every request
whose outcome it could not check, and revokes nothing.

Exit: 0 ok · 1 a step failed · 2 usage · 5 refused.`

const labelsLong = `labels creates the nine fleet labels (review-request, six raised-by:<role>, and the
authorization-needed / approval-needed pair) on one GitHub repo or GitLab project. An
existing label is a no-op.

  deskfleet labels --forge gitlab --project <path-or-id> --token-file <file> [--dry-run]
  deskfleet labels --forge github --repo <owner/name>    --token-file <file> [--dry-run]

The credential is read from --token-file only, which must pass the owner-only custody
check; a link at that path is followed. --forge gitlab also requires GITLAB_API_BASE for a
real run. --dry-run enumerates the labels and makes ZERO network calls.

Exit: 0 ok · 1 a step failed · 2 usage · 5 refused.`

// provisionFn and labelsFn are the typed handlers the command tree calls; the contract tests
// replace them to observe the options that reach each.
var (
	provisionFn = runProvision
	labelsFn    = runLabels
)

// onResolve, when set, sees every invocation's resolved settings (the contract tests' window
// on which source won).
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == 0 {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version prints.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskfleet sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// apiBaseBinding is GITLAB_API_BASE: environment-only, read when a verb runs. It is a base
// URL, not a credential; it has no flag, as it never had one.
func apiBaseBinding() cli.Binding {
	return cli.Binding{Key: "gitlab-api-base", Kind: cli.String, Env: []string{"GITLAB_API_BASE"},
		Usage: "GitLab REST v4 base (environment only)"}
}

// provisionBindings are every setting `provision` declares: the Go-flag spellings, defaults
// and usage text it had, plus the environment-only API base.
func provisionBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "group", Kind: cli.String, Flag: "group",
			Usage: "top-level GitLab group (path or id) that owns the fleet"},
		{Key: "prefix", Kind: cli.String, Flag: "prefix",
			Usage: "username prefix: <prefix>-<role>-bot"},
		{Key: "project", Kind: cli.String, Flag: "project",
			Usage: "fleet project (path or id) whose settings and labels to configure"},
		{Key: "owner-token-file", Kind: cli.String, Flag: "owner-token-file",
			Usage: "FILE holding a group-owner PAT (owner-only; never a flag value)"},
		{Key: "out-dir", Kind: cli.String, Flag: "out-dir",
			Usage: "where gitlab-<role>.token files are written (default: the config home)"},
		{Key: "pat-expiry-days", Kind: cli.Int, Flag: "pat-expiry-days", Default: 7,
			Usage: "PAT lifetime in days (1-365)"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run",
			Usage: "enumerate every action; make zero network calls"},
		{Key: "avatars-dir", Kind: cli.String, Flag: "avatars-dir",
			Usage: "upload <dir>/<role>.png as each account's own avatar (PUT /user/avatar, as that role's own PAT); omitted = the avatar step is skipped"},
		{Key: "avatars-only", Kind: cli.Bool, Flag: "avatars-only",
			Usage: "set the avatars of accounts that already exist, as each role, from the gitlab-<role>.token files under --out-dir; creates, mints and configures nothing"},
		{Key: "no-avatars", Kind: cli.Bool, Flag: "no-avatars",
			Usage: "skip the avatar step (prints a NOTICE); the script's flag of the same name"},
		apiBaseBinding(),
	}
}

// labelsBindings are every setting `labels` declares.
func labelsBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "forge", Kind: cli.String, Flag: "forge", Usage: "github | gitlab"},
		{Key: "repo", Kind: cli.String, Flag: "repo", Usage: "GitHub repository owner/name (--forge github)"},
		{Key: "project", Kind: cli.String, Flag: "project", Usage: "GitLab project path or id (--forge gitlab)"},
		{Key: "token-file", Kind: cli.String, Flag: "token-file", Usage: "FILE holding a credential allowed to create labels"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Usage: "enumerate the labels; make zero network calls"},
		apiBaseBinding(),
	}
}

// noPositional refuses any word after a verb's flags, as the Go flag parsers did.
func noPositional(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q for %q", args[0], cmd.CommandPath())
	}
	return nil
}

// newFleetCLI builds one fresh command tree for one invocation.
func newFleetCLI(e *env) *cobra.Command {
	root := cli.NewRoot("deskfleet", "Go-native GitLab fleet provisioning and forge-neutral fleet labels")
	root.Long = rootLong
	root.Args = cobra.NoArgs
	// --version is the root's own flag, not Cobra's built-in one: Cobra answers its version
	// flag before argument validation, so `deskfleet --version extra` would print the version
	// and exit 0. Here the root's NoArgs refuses any word beside it (exit 2).
	root.Flags().Bool("version", false, "print the version and exit")
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if v, _ := cmd.Flags().GetBool("version"); v {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionLine())
			return err
		}
		// No verb is a usage error, as it always was: the help screen on stderr, exit 2.
		cmd.SetOut(cmd.ErrOrStderr())
		_ = cmd.Help()
		return exitErr(exitUsage)
	}

	provision := &cobra.Command{
		Use:   "provision --group <path-or-id> --prefix <name> --owner-token-file <file> [flags]",
		Short: "create the fleet's service accounts, tokens and (with --project) project settings",
		Long:  provisionLong,
		Args:  noPositional,
	}
	pset := cli.Declare(provision, provisionBindings()...)
	provision.RunE = func(_ *cobra.Command, _ []string) error {
		v, err := pset.Resolve(cli.Inputs{LookupEnv: e.lookupEnv})
		if err != nil {
			fmt.Fprintf(e.stderr, "deskfleet provision: %v\n", err)
			return exitErr(exitUsage)
		}
		if onResolve != nil {
			onResolve(v)
		}
		o := provisionOptsFrom(v)
		if err := validateProvision(&o); err != nil {
			fmt.Fprintf(e.stderr, "deskfleet provision: %v\n", err)
			return exitErr(exitUsage)
		}
		return codeErr(provisionFn(o, e))
	}

	labels := &cobra.Command{
		Use:   "labels --forge github|gitlab (--repo <owner/name> | --project <path-or-id>) --token-file <file> [flags]",
		Short: "create the nine fleet labels on one GitHub repo or GitLab project",
		Long:  labelsLong,
		Args:  noPositional,
	}
	lset := cli.Declare(labels, labelsBindings()...)
	labels.RunE = func(_ *cobra.Command, _ []string) error {
		v, err := lset.Resolve(cli.Inputs{LookupEnv: e.lookupEnv})
		if err != nil {
			fmt.Fprintf(e.stderr, "deskfleet labels: %v\n", err)
			return exitErr(exitUsage)
		}
		if onResolve != nil {
			onResolve(v)
		}
		o := labelsOptsFrom(v)
		if err := validateLabels(&o); err != nil {
			fmt.Fprintf(e.stderr, "deskfleet labels: %v\n", err)
			return exitErr(exitUsage)
		}
		return codeErr(labelsFn(o, e))
	}

	root.AddCommand(provision, labels)
	return root
}

// run is one invocation against a fresh command tree. sinks, when given, receive the
// performance context the parser selected (main's measurement).
func run(args []string, e *env, sinks ...func(perfrec.Context)) int {
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	return cli.Run(func() *cobra.Command { return newFleetCLI(e) }, args, cli.Options{
		Performance:  sink,
		IO:           cli.IO{Out: e.stdout, Err: e.stderr},
		UsageExit:    exitUsage,
		GoFlagCompat: true,
	})
}

func provisionOptsFrom(v *cli.Values) provisionOpts {
	return provisionOpts{
		group: v.String("group"), prefix: v.String("prefix"), project: v.String("project"),
		ownerTokenFile: v.String("owner-token-file"), outDir: v.String("out-dir"),
		patExpiryDays: v.Int("pat-expiry-days"), dryRun: v.Bool("dry-run"),
		avatarsDir: v.String("avatars-dir"), avatarsOnly: v.Bool("avatars-only"), noAvatars: v.Bool("no-avatars"),
		apiBase: v.String("gitlab-api-base"),
	}
}

func labelsOptsFrom(v *cli.Values) labelsOpts {
	return labelsOpts{
		forge: v.String("forge"), repo: v.String("repo"), project: v.String("project"),
		tokenFile: v.String("token-file"), dryRun: v.Bool("dry-run"),
		apiBase: v.String("gitlab-api-base"),
	}
}
