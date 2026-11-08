// Command deskgitlab provisions the Assay fleet on a GitLab top-level group: the seven
// per-role service accounts, their group memberships and first PATs, each account's own
// avatar, and — with --project — the fleet project's protected `main`, MR-approval settings,
// protected release tags, merge checks and labels.
//
// It is the Go port of tools/create-fleet-gitlab.sh (desktools-v2/30), which is now a
// transparent adapter that execs `deskgitlab create`. The behaviour, the output lines and
// the exit codes are the script's; the changelog fragment changelog/cli-gitlab-create.md
// records where they deliberately differ.
//
// Exit codes: 0 every step succeeded · 1 a step failed (listed in the summary) or a fatal
// API refusal · 2 usage (an unknown flag, a bad value, a missing requirement), before any
// effect.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const rootLong = `deskgitlab provisions the Assay fleet on a GitLab top-level group.

Verbs:
  create   idempotently provision the seven per-role service accounts and, with
           --project, the fleet project's settings (tools/create-fleet-gitlab.sh
           runs this verb).

Run 'deskgitlab create --help' for its options.

Exit codes: 0 ok · 1 a step failed · 2 usage, before any effect.`

const createLong = `Idempotently provisions the Assay fleet's seven per-role GitLab service
accounts (spec.md §2), their group memberships and first PATs, then — when
--project is given — the fleet project's protected main branch, MR-approval
settings, protected release tags, merge checks and labels (spec.md §3).

Required: --group and --prefix (or, with --avatars-only, --prefix and --out-dir).

Project-level steps are skipped with a NOTICE, never silently, when --project is
omitted. --tier ultimate also scripts the two Ultimate refinements (a custom
reviewer role that cannot push, and an external-status-check verdict lane); each
is recorded as could-not-check on an instance that 403s it, never a silent
downgrade. free and premium add nothing beyond the core lane.

Auth: a group-owner personal access token, supplied ONLY via the GITLAB_TOKEN
environment variable (never a flag, never argv) — required unless --dry-run or
--avatars-only. This command never stores it.

Avatars are set by each account ON ITSELF (PUT /user/avatar with that role's
own PAT): a group Owner cannot set a service account's avatar. By default the
public role icons are fetched from
https://assay.guide/assets/app-icon-<role>.png; a missing or unfetchable icon is
a NOTICE, never a failure.

Exit status: 0 when every step succeeded; 1 when any step failed (a failed
settings step does not abort the ones after it — every step runs and the
failures are listed in the summary); 2 for a usage error, before any effect.`

func main() {
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	os.Exit(deskkit.PerfRunMeasured("deskgitlab", []string{"create"}, os.Args[1:], func(a []string, measurement *perfrec.Session) (int, perfrec.Context) {
		sink := func(c perfrec.Context) {
			selected = c
			if measurement != nil {
				measurement.SelectContext(c)
			}
		}
		code := execute(a, os.Stdout, os.Stderr, sink)
		return code, selected
	}))
}

// lookupEnv is the ONE environment read the settings make (GITLAB_TOKEN, the only declared
// environment source). A variable so the contract tests can count it: help and version read
// nothing.
var lookupEnv = os.LookupEnv

// provisionFn is the typed handler the command tree calls; the contract tests replace it to
// observe the options that reach it.
var provisionFn = provision

// onResolve, when set, sees every invocation's resolved settings (the contract tests' window on
// which source won).
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
	s, b := deskkit.Version()
	return fmt.Sprintf("deskgitlab sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

// bindings are every setting `create` declares. Each is a flag with the script's spelling and
// default, except the owner credential, which is environment-only and Secret: it can never be
// a flag, so it can never reach argv.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "group", Kind: cli.String, Flag: "group",
			Usage: "top-level GitLab group (`path-or-id`) that owns the fleet's service accounts"},
		{Key: "prefix", Kind: cli.String, Flag: "prefix",
			Usage: "`name` prefix for generated usernames, e.g. myorg -> myorg-reviewer-bot"},
		{Key: "project", Kind: cli.String, Flag: "project",
			Usage: "fleet project (`path-or-id`) to configure; omit to provision accounts only"},
		{Key: "gitlab-url", Kind: cli.String, Flag: "gitlab-url", Default: defaultGitLabURL,
			Usage: "instance base `url` (self-managed EE: your instance's base URL)"},
		{Key: "pat-expiry-days", Kind: cli.String, Flag: "pat-expiry-days", Default: strconv.Itoa(defaultPATDays),
			Usage: "PAT lifetime in whole `days` (spec.md §5's RECOMMENDED backstop)"},
		{Key: "out-dir", Kind: cli.String, Flag: "out-dir",
			Usage: "`dir` the minted token files are written to, 0600 each; refused if other users can write to it (default: a fresh temp dir)"},
		{Key: "avatars-dir", Kind: cli.String, Flag: "avatars-dir",
			Usage: "upload `dir`/<role>.png as each new account's own avatar (default: the public role icons)"},
		{Key: "no-avatars", Kind: cli.Bool, Flag: "no-avatars",
			Usage: "skip the avatar step entirely (prints a NOTICE)"},
		{Key: "avatars-only", Kind: cli.Bool, Flag: "avatars-only",
			Usage: "re-upload avatars for existing accounts from the token files under --out-dir; nothing else"},
		{Key: "tier", Kind: cli.String, Flag: "tier",
			Usage: "free | premium | ultimate (`t`); ultimate also scripts the two Ultimate refinements"},
		{Key: "status-check-url", Kind: cli.String, Flag: "status-check-url",
			Usage: "verdict lane endpoint `url` to register the external status check against (--tier ultimate)"},
		{Key: "status-check-name", Kind: cli.String, Flag: "status-check-name", Default: defaultStatusCheckName,
			Usage: "`name` of the external status check the verdict lane posts to"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run",
			Usage: "enumerate every action; make zero network calls"},
		{Key: "gitlab-token", Kind: cli.String, Env: []string{"GITLAB_TOKEN"}, Secret: true,
			Usage: "group-owner PAT (environment only)"},
	}
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer, sinks ...func(perfrec.Context)) int {
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	build := func() *cobra.Command {
		root := cli.NewRoot("deskgitlab", "Assay GitLab fleet provisioning")
		root.Long = rootLong
		create := &cobra.Command{
			Use:   "create --group <group> --prefix <prefix> [flags]",
			Short: "idempotently provision the fleet's service accounts and project settings",
			Long:  createLong,
			Args:  cobra.NoArgs,
		}
		set := cli.Declare(create, bindings()...)
		create.RunE = func(cmd *cobra.Command, _ []string) error {
			return codeErr(runCreate(cmd, set, stdout, stderr))
		}
		root.AddCommand(create)
		return root
	}
	return cli.Run(build, args, cli.Options{
		Performance:     sink,
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		UsageExit:       cli.ExitUsage,
	})
}

// runCreate resolves and checks the settings, then hands them to the provisioner: every
// refusal happens here, before any file, directory or network effect.
func runCreate(cmd *cobra.Command, set *cli.Set, stdout, stderr io.Writer) int {
	v, err := set.Resolve(cli.Inputs{LookupEnv: lookupEnv})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return cli.ExitUsage
	}
	if onResolve != nil {
		onResolve(v)
	}
	o := optionsFrom(v)
	if err := validate(o); err != nil {
		fmt.Fprintln(stderr, err.Error())
		var r *refusal
		if errors.As(err, &r) && r.usage {
			fmt.Fprint(stderr, cmd.UsageString())
		}
		return cli.ExitUsage
	}
	return provisionFn(o, stdout, stderr)
}

// options are one run's settings, typed.
type options struct {
	group, project, prefix string
	gitlabURL              string
	patExpiryDays          string // as given; validate checks it is a whole number of days
	outDir, avatarsDir     string
	noAvatars, avatarsOnly bool
	dryRun                 bool
	tier                   string
	statusCheckURL         string
	statusCheckName        string
	token                  string // NEVER printed
}

func optionsFrom(v *cli.Values) options {
	return options{
		group: v.String("group"), project: v.String("project"), prefix: v.String("prefix"),
		gitlabURL: v.String("gitlab-url"), patExpiryDays: v.String("pat-expiry-days"),
		outDir: v.String("out-dir"), avatarsDir: v.String("avatars-dir"),
		noAvatars: v.Bool("no-avatars"), avatarsOnly: v.Bool("avatars-only"), dryRun: v.Bool("dry-run"),
		tier: v.String("tier"), statusCheckURL: v.String("status-check-url"),
		statusCheckName: v.String("status-check-name"), token: v.String("gitlab-token"),
	}
}

// refusal is a usage error: exit 2, before any effect. usage asks for the usage text after
// the message, as the script printed it.
type refusal struct {
	msg   string
	usage bool
}

func (r *refusal) Error() string { return r.msg }
func (r *refusal) ExitCode() int { return cli.ExitUsage }

// validate is the domain check: every combination the provisioner refuses, in the script's
// order and with its messages, plus three it adds (listed in changelog/cli-gitlab-create.md).
// It reads nothing and touches nothing, so a refused run has had no effect.
func validate(o options) error {
	if o.avatarsOnly {
		if o.noAvatars {
			return &refusal{msg: "error: --avatars-only and --no-avatars are contradictory"}
		}
		if o.prefix == "" || o.outDir == "" {
			return &refusal{msg: "error: --avatars-only requires --prefix and --out-dir (the token files it re-uses)", usage: true}
		}
	} else if o.group == "" || o.prefix == "" {
		return &refusal{msg: "error: --group and --prefix are required", usage: true}
	}
	if !validTier(o.tier) {
		return &refusal{msg: "error: --tier must be one of: free, premium, ultimate (got '" + o.tier + "')", usage: true}
	}
	// The script let the LAST of --avatars-dir / --no-avatars win; a flag parser keeps no
	// order, so the pair is refused rather than resolved by a guess.
	if o.noAvatars && o.avatarsDir != "" {
		return &refusal{msg: "error: --avatars-dir and --no-avatars are contradictory"}
	}
	if n, err := strconv.Atoi(o.patExpiryDays); err != nil || n < 1 || n > 365 || strings.HasPrefix(o.patExpiryDays, "+") {
		return &refusal{msg: "error: --pat-expiry-days must be a whole number of days, 1-365 (got '" + o.patExpiryDays + "')", usage: true}
	}
	if err := validateGitLabURL(o.gitlabURL); err != nil {
		return &refusal{msg: err.Error()}
	}
	// --avatars-only authenticates as each ROLE (its own token file), so it needs no owner
	// PAT; every other live mode does. An empty value counts as unset, as `-z` did.
	if !o.dryRun && !o.avatarsOnly && o.token == "" {
		return &refusal{msg: "error: GITLAB_TOKEN must be set (a group-owner PAT) unless --dry-run or --avatars-only"}
	}
	return nil
}

// validTier reports whether t is a --tier value (empty: not given).
func validTier(t string) bool {
	switch t {
	case "", "free", "premium", "ultimate":
		return true
	}
	return false
}
