// Command deskdecision is the decision-issue gate: for a `gate: human` brief it files ONE
// self-contained decision issue, unless the brief already has one.
//
// It is the pure-Go port of the repo-local helper `tools/decision-issue.sh` `ensure` verb,
// shipped with desk-tools so a repo that does not carry that script still has a gate.
// deskdispatch resolves it the way it resolves the claim tool: this binary on PATH first, then
// the repo's script, then a refusal naming both. Both speak one protocol — the same argument
// contract, exit codes, issue title and body, and dedupe markers — so a brief filed by either
// is found by the other (deskkit/decisionensure.go holds it).
//
// USAGE:
//
//	deskdecision ensure <brief-file> [--repo <owner/name>] [--at creation|start|spec]
//	                    [--held] [--dry-run] [--as <role>]
//	deskdecision --version
//
// Every forge read and write goes through the deskkit forge client under the named role's
// own credential (GitHub or GitLab, resolved from the roster and the repo's remote) — never an
// ambient CLI login and never a token from the environment.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskdecision"

const rootLong = `deskdecision — the decision-issue gate.

  deskdecision ensure <brief-file> [--repo <owner/name>] [--at creation|start|spec]
                      [--held] [--dry-run] [--as <role>]
  deskdecision --version

ensure files ONE self-contained decision issue for a 'gate: human' brief, unless
the brief already has one: its frontmatter 'decision-issue: <N>' (read, never
written), or an issue in any state whose body carries the marker
'<!-- decision-gate: <id> -->' or the legacy '<!-- needs-decision: <id> -->'.
It is the port of the repo-local helper tools/decision-issue.sh, and both agree
byte for byte on what they file, so either finds the other's issue.

Exit codes:
  0  ok              — not-gated, not-needed, deferred, exists, created or dry-run
  4  rate-limited    — the write budget refused the filing; nothing was filed
  5  refused         — the brief or the command line is malformed
  6  could-not-check — a lookup or the filing failed; nothing is created on a
                       lookup that could not be completed`

const ensureLong = `Files the decision issue for a 'gate: human' brief, once.

Order: the brief's own checks (gate, board row, decision-trigger against --at,
spec ripeness) run first and read no credential; then every existing-record
lookup; then the body is composed and checked for self-containment (no links or
URLs); then — unless --dry-run — the issue is filed and labelled needs-decision
when that label exists on the repo (filed unlabelled with a notice when it does
not).

--repo defaults to the working directory's origin remote. --at names the calling
stage (default start); a brief whose decision-trigger is a later stage is
deferred. --held files for a brief whose board row already reads done (the
decision-gate hold refused a change to it). --as names the role whose credential
performs the reads and the filing (default: the dispatcher role).`

// ensureOptions are the typed values the command tree hands the gate.
type ensureOptions struct {
	brief, repo, at, as string
	held, dryRun        bool
}

// ensureFn is the typed handler the command tree calls; the contract tests replace it.
var ensureFn = runEnsure

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// forgeFor resolves the forge for a repo under a role — a seam so the tests run offline.
var forgeFor = func(repo deskkit.ForgeRepo, role string) (deskkit.DecisionEnsureForge, error) {
	f, _, err := deskkit.ResolveForge(repo, role)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// allowCreate charges the outward-write budget for one filing. A filing creates a target
// whose number is not known in advance, so the repo-wide scope is the bucket it lands in
// (the same reasoning as deskfile new).
var allowCreate = func(repo string) error { return deskkit.AllowWriteRepoWide(toolName, repo) }

type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("%s sourceSHA=%s builtAt=%s releaseTag=%s", toolName, sha, built, deskkit.ReleaseTagOrDev())
}

func main() {
	os.Exit(deskkit.PerfRun("deskdecision", []string{"ensure"}, os.Args[1:], func(a []string) int {
		return execute(a, os.Stdout, os.Stderr, startup)
	}))
}

// startup is the process startup an ensure runs first; help and version never reach it.
// false means the verb is not activated (exit 6).
func startup(errw io.Writer) bool {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(errw)
	return deskkit.CheckVerbActivation(errw)
}

// bindings are every setting ensure declares: flags and defaults only, no environment
// variable and no configuration file.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "repo", Kind: cli.String, Flag: "repo", Default: "", Usage: "target `owner/name` (default: the working directory's origin remote)"},
		{Key: "at", Kind: cli.String, Flag: "at", Default: "start", Usage: "calling stage: creation, start or spec"},
		{Key: "held", Kind: cli.Bool, Flag: "held", Default: false, Usage: "file even though the board row already reads done"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Default: false, Usage: "print what would be filed; write nothing"},
		{Key: "as", Kind: cli.String, Flag: "as", Default: deskkit.DispatcherRole, Usage: "`role` whose credential reads and files"},
	}
}

func ensureCommand(stdout, stderr io.Writer, pre func(io.Writer) bool) *cobra.Command {
	cmd := &cobra.Command{Use: "ensure <brief-file>", Short: "File the decision issue for a gate: human brief, once", Long: ensureLong}
	set := cli.Declare(cmd, bindings()...)
	cmd.Args = func(_ *cobra.Command, args []string) error {
		switch {
		case len(args) == 0:
			return fmt.Errorf("ensure needs a brief file")
		case len(args) > 1:
			return fmt.Errorf("unknown argument %q", args[1])
		}
		return nil
	}
	cmd.RunE = func(_ *cobra.Command, args []string) error {
		v, err := set.Resolve(cli.Inputs{})
		if err != nil {
			fmt.Fprintf(stderr, "%s: refused: %v\n", toolName, err)
			return exitErr(deskkit.ExitRefused)
		}
		if onResolve != nil {
			onResolve(v)
		}
		if pre != nil && !pre(stderr) {
			return exitErr(deskkit.ExitUnverifiable)
		}
		return codeErr(ensureFn(ensureOptions{
			brief: args[0], repo: v.String("repo"), at: v.String("at"), as: v.String("as"),
			held: v.Bool("held"), dryRun: v.Bool("dry-run"),
		}, stdout, stderr))
	}
	return cmd
}

func newTree(stdout, stderr io.Writer, pre func(io.Writer) bool) *cobra.Command {
	root := cli.NewRoot(toolName, "the decision-issue gate")
	root.Long = rootLong
	root.Args = func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q", args[0])
		}
		return nil
	}
	root.RunE = func(*cobra.Command, []string) error {
		fmt.Fprintf(stderr, "%s: refused: choose a verb: ensure\n", toolName)
		return exitErr(deskkit.ExitRefused)
	}
	root.AddCommand(ensureCommand(stdout, stderr, pre))
	return root
}

// execute runs one invocation against a fresh command tree. pre is the process startup; it
// runs after the command line parses and never for help or version.
func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) bool) int {
	build := func() *cobra.Command { return newTree(stdout, stderr, pre) }
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		UsageExit:       deskkit.ExitRefused,
	})
}

// runEnsure is the gate itself. It honours the kill switch before any work and records one
// audit line per run, billed by what the run did at the forge.
func runEnsure(o ensureOptions, stdout, stderr io.Writer) int {
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)
	res := deskkit.EnsureDecisionIssue(deskkit.DecisionEnsureRequest{
		BriefFile: o.brief,
		Repo:      o.repo,
		OriginRepo: func() (string, error) {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return deskkit.OriginRepoSlug(wd)
		},
		At:     o.at,
		Held:   o.held,
		DryRun: o.dryRun,
		Forge: func(repo deskkit.ForgeRepo) (deskkit.DecisionEnsureForge, error) {
			return forgeFor(repo, o.as)
		},
		BeforeCreate: allowCreate,
		Out:          stdout,
		Err:          stderr,
	})
	detail := res.State + " " + o.brief
	if res.Number > 0 {
		detail += " #" + strconv.Itoa(res.Number)
	}
	if err := deskkit.Log(deskkit.Entry{Tool: toolName, Verb: "ensure", Repo: res.Repo, Result: auditResult(res), Detail: detail}); err != nil {
		fmt.Fprintf(stderr, "%s: cannot audit the ensure (%s): %v\n", toolName, res.State, err)
		return deskkit.ExitUnverifiable
	}
	return res.Code
}

// auditResult bills a run by what it did at the forge: only a filing (or a filing that may
// have reached the forge) charges the write budget.
func auditResult(r deskkit.DecisionEnsureResult) string {
	switch r.State {
	case "created":
		return deskkit.ResultOK
	case "exists":
		return deskkit.ResultNoop
	case "dry-run":
		return deskkit.ResultDryRun
	case "refused":
		return deskkit.ResultRefused
	case "rate-limited":
		return deskkit.ResultRateLimited
	case "unverifiable":
		if r.Attempted {
			return deskkit.ResultUnverifiable
		}
		return deskkit.ResultUnwritten
	default: // not-gated, not-needed, deferred
		return deskkit.ResultLocal
	}
}
