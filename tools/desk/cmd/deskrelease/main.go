// Command deskrelease cuts a release TAG for this repository's two release
// namespaces — `desk-tools/vX.Y.Z` and `statusgen/vX.Y.Z` — as the desk App, via the
// GitHub git-data API. It exists to replace a `git push` allow-list grant, and the
// reason it can is structural:
//
//	Bash(/opt/desk-tools/bin/deskrelease cut *) has the same trailing-wildcard
//	shape as Bash(git push origin desk-tools/v*), but the LAST layer the wildcard
//	reaches is THIS binary's argv parser, which refuses everything it does not
//	recognise. A glob has no end anchor; a Go main() does.
//
// Two precisions on that sentence, because a PR arguing that absolute-sounding claims
// about matchers are how this class of defect happened must not make one:
//
//   - the wildcard does NOT reach only this parser. The SHELL sees the command text
//     first, and this binary can do nothing about that layer. What it does supply is
//     the END anchor that the layer beneath the glob was missing: whatever argv
//     survives the shell arrives at a parser that refuses anything unrecognised.
//   - the rule must name the ABSOLUTE path. /opt/desk-tools/bin is NOT on PATH — every
//     desk tool is invoked as /opt/desk-tools/bin/<tool> … — so `Bash(deskrelease cut *)`
//     could never match the command text and would be an INERT rule, which is exactly
//     the dead-rule defect described below for `git -C … push` against
//     `^git push origin …`. Putting the bare name on PATH to make it match is worse: the
//     grant would then authorise whatever `deskrelease` resolves to, including a
//     `tools/desk/dist/deskrelease` built from any branch. That is the same argument
//     deskTokenPath already makes one level down (github.go).
//
// Background. The push grant was found to be unbounded: Claude Code's Bash
// wildcard "matches any sequence of characters INCLUDING spaces", so one `*` spans
// arguments. `git push origin desk-tools/v0.1.1 +HEAD:refs/heads/main` matches the
// allow rule (prefix matches; the `*` absorbs the rest) and matches no force-deny
// (there is no literal `--force`/`-f`). Sandbox testing against a throwaway bare repo
// confirmed four working escapes — branch move via `^{}:refs/heads/main`, force move
// via a bare ` +main`, real force via bundled `-uf`, and non-default-branch deletion
// via ` :refs/heads/<branch>`. Nothing downstream catches them: branch protection is
// unavailable on this plan (HTTP 403 "Upgrade to GitHub Pro"), a pre-commit hook is
// pre-COMMIT so a refspec write to main never reaches it, and deskpushguard neither
// inspects destination refs nor fails closed. The allow-list was the only layer.
//
// Deny-rule patches were evaluated and are NOT what this tool replicates: the first
// three-line set blocked only a minority of the dangerous shapes, was quote-defeatable (`<tag> "+main"`
// evades `* +*` because the character before `+` becomes `"`), and a rule ending `:*`
// is parsed as a legacy PREFIX rule, so the obvious `Bash(git push *:*)` is inert and
// never fires. String-matching defending string-matching is a defeated layer. The
// systemic form: allow rules that end in unanchored wildcards.
//
// The push grant also never delivered the capability it was requested for:
// `cut-release/SKILL.md` invokes `git -C <toolkit> push origin desk-tools/v<X.Y.Z>`,
// and the rules compile anchored at `^git push origin …`, which a `git -C …` command
// does not match. deskrelease has no such variance — one fixed argv shape, no `-C`,
// no cwd dependence, no local checkout at all.
//
// What it will do, and nothing else:
//
//   - `cut <tag>` — create refs/tags/<tag> at the CURRENT remote origin/main HEAD.
//
// There is no delete verb, no force verb, no override/waiver verb, no --repo, no
// --sha, and no way to move an existing ref. The repo comes from the ROSTER
// (ASSAY_RELEASE_REPO, defaulting to the shipped public home) — read from the
// config-home file only, never from a flag, an argument or the environment, and still
// screened by IsAllowedRepo — so a fork can name its own release home while no caller
// of this binary can retarget it. The tag's commit is re-read from the remote at act
// time and is never caller-supplied, so there is no argument through which a caller
// can select what gets released.
//
// Guarantees it holds:
//
//   - the kill switch runs before anything that ACTS — before the cut argv parse, the
//     identity mint, and any network call. Precisely: the non-acting requests (`version`,
//     `--version`, `-v`, `help`, `-h`, `--help`, and `cut -h` / `cut --help` with no other
//     word) are answered by the command tree ahead of Guard() and print without consulting
//     it; every path that could touch the remote runs after it;
//   - the tag is validated against ^(desk-tools|statusgen)/v\d+\.\d+\.\d+$ AND
//     screened for refspec metacharacters, each with its own refusal reason;
//   - ANY unrecognised flag or extra operand is REFUSED, never ignored — a silently
//     ignored operand is precisely the hole this replaces;
//   - the tag argument comes from argv only; no stdin, no environment variable;
//   - an EXISTING tag is never moved or re-pointed — consumers pin by tag+sha256, so a
//     moved tag is the exact substitution the pin prevents. A wrong release is fixed
//     FORWARD with the next patch version. An existing tag ALREADY at the current
//     origin/main HEAD is a NOOP (exit 0, nothing written): the requested end state is
//     the actual state, so a transient error on the create cannot burn a version number.
//     An existing tag at any other commit — or a HEAD that cannot be resolved — is the
//     hard refusal;
//   - the tag it creates is LIGHTWEIGHT (POST /git/refs makes a ref, not a tag object),
//     so there is no tag message for anyone to compose. releaseguard reads
//     `Release-Override:` trailers only via `cat-file tag` and only when the tag is
//     annotated, so a caller of this tool cannot author a waiver by ANY route — the
//     override channel is closed rather than relocated. See the README section for what
//     that costs and what a releaser does when a waiver is genuinely warranted;
//   - it acts as the desk App (the desk App), so the audit trail names an
//     identity rather than whatever local credential the calling session holds;
//   - exactly one audit line per invocation, including every refusal, so a
//     refusal loop trips the outward-write rate limit;
//   - it fails CLOSED on anything it cannot positively verify.
//
// It deliberately leaves release-desk.yml, release-statusgen.yml and releaseguard
// untouched: creating the tag ref still fires `on: push: tags:`, and releaseguard's
// unwaivable `on-main` / `tag-format` checks still gate the build.
//
// Exit codes: 0 ok/noop · 2 usage · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
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

// helpLong is the root help's prose (desktools-v2/46: the command tree generates the rest).
const helpLong = `deskrelease — cut a release tag as the desk App, via the GitHub git-data API

usage:
  deskrelease cut <tag> [--dry-run]
  deskrelease version

<tag> must match ^(assay|desk-tools|statusgen)/v[0-9]+\.[0-9]+\.[0-9]+$ — e.g. desk-tools/v0.1.3.
The assay namespace is the UMBRELLA line (e.g. assay/v0.9.0); the other two
are per-artifact lines. This list is the set this tool CUTS, not the set this repo releases:
daily-harvest/vX.Y.Z tags exist and are cut by their own release path, not by deskrelease.
The tag is created at the CURRENT remote origin/main HEAD, re-read at act time; there is
no --sha and no --repo. An existing tag is REFUSED, never moved (fix forward with the
next patch version). Any extra operand or unknown flag is REFUSED, never ignored.

There is no delete, force, move, or override verb, and no git push is ever executed.

exit codes: 0 ok/noop · 2 usage · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable`

// cutHelp is `deskrelease cut --help`.
const cutHelp = `deskrelease cut — create refs/tags/<tag> at the current remote origin/main HEAD, as the desk App.

usage:
  deskrelease cut <tag> [--dry-run]

<tag> is (desk-tools|statusgen|assay)/vMAJOR.MINOR.PATCH — argv only, never stdin or the
environment. --dry-run validates and resolves everything but writes nothing. Any other word
— an extra operand, an unknown flag, a help word after the tag — is REFUSED (exit 5) and
audited, never ignored.

exit codes: 0 ok/noop · 2 usage · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable`

func main() {
	os.Exit(deskkit.PerfRun("deskrelease", nil, os.Args[1:], func(a []string) int {
		return execute(a, os.Stdout, os.Stderr, startup)
	}))
}

// startup is the process startup a cut runs before anything else; help and version never
// reach it. false means the verb is not activated (exit 6).
func startup(errw io.Writer) bool {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(errw)
	return deskkit.CheckVerbActivation(errw)
}

// run is one invocation on the package streams, without the process startup — the shape the
// package's tests drive.
func run(argv []string) int {
	return execute(argv, stdout, stderr, nil)
}

// cutFn is the cut handler the command tree calls with the full argv (`cut` first); the
// contract tests replace it to observe what reaches it.
var cutFn = cmdCut

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what version, --version and -v print.
func versionLine() string {
	s, b := deskkit.Version()
	return fmt.Sprintf("deskrelease sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

// isHelpWord reports whether a is a help request.
func isHelpWord(a string) bool { return a == "-h" || a == "--help" }

// execute runs one invocation against a fresh command tree. pre is the process startup; it
// runs only on a cut, after the command line parses, and never for help or version.
//
// The tree declares NO setting: no flag beyond help and version, no environment variable, no
// configuration key. `cut` takes its argv raw (DisableFlagParsing) so that parseCutArgs stays
// its ONE parser — the argv strictness this tool exists for (an unknown flag or extra operand
// is refused with 5 and audited, never ignored) is not handed to a general-purpose parser.
func execute(argv []string, out, errw io.Writer, pre func(io.Writer) bool) int {
	prevOut, prevErr := stdout, stderr
	stdout, stderr = out, errw
	defer func() { stdout, stderr = prevOut, prevErr }()

	build := func() *cobra.Command { return newTree(out, errw, pre) }
	return cli.Run(build, argv, cli.Options{
		IO:              cli.IO{Out: out, Err: errw},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A word the tree does not know (an unknown verb or root flag) is a usage error (2),
		// as before; cut's own refusals stay 5 through parseCutArgs.
		UsageExit: 2,
	})
}

// newTree builds deskrelease's command tree on the given streams; pre is as for execute.
func newTree(out, errw io.Writer, pre func(io.Writer) bool) *cobra.Command {
	root := cli.NewRoot("deskrelease", "cut a release tag as the desk App, via the GitHub git-data API")
	root.Long = helpLong
	// -v printed the version before the migration; it still does.
	root.Flags().BoolP("version", "v", false, "print the version and exit")
	// A bare deskrelease prints its usage to stderr and exits 2, as before.
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		cmd.SetOut(errw)
		_ = cmd.Help()
		return exitErr(2)
	}
	cut := &cobra.Command{
		Use:   "cut <tag> [--dry-run]",
		Short: "create refs/tags/<tag> at the current remote origin/main HEAD",
		Long:  cutHelp,
		// parseCutArgs is cut's one parser; see execute's doc comment.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, rest []string) error {
			// A help word ALONE is a request for help: nothing is named, so nothing can
			// act. After a tag it is an operand like any other and parseCutArgs refuses it.
			if len(rest) == 1 && isHelpWord(rest[0]) {
				return cmd.Help()
			}
			if pre != nil && !pre(errw) {
				return exitErr(deskkit.ExitUnverifiable)
			}
			// Resolve the release home before anything reads it. Unset leaves the shipped
			// default in place, so this is a no-op for an adopter who configures nothing.
			applyConfiguredReleaseRepo()
			deskkit.WarnIfUnpinned(errw)
			// Kill switch FIRST among the acts — before parsing the cut argv or touching
			// the network. A disabled suite exits 3 after Guard audits result=disabled.
			if err := deskkit.Guard(); err != nil {
				fmt.Fprintln(errw, "deskrelease: "+err.Error())
				return exitErr(deskkit.ExitCodeOf(err))
			}
			return codeErr(cutFn(append([]string{"cut"}, rest...)))
		},
	}
	root.AddCommand(cut)
	// `deskrelease version` printed the build stamp before the migration; it still does.
	root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			fmt.Fprintln(out, versionLine())
			return nil
		}})
	return root
}

// cmdCut runs the one mutating verb through the outward-write flow.
//
// The exit-2 / exit-5 split is deliberate and is the whole argv-strictness contract:
//
//   - NOTHING was specified (`deskrelease cut`) → usage, exit 2, no audit line. No act
//     was attempted, so there is nothing to record and no budget to spend.
//   - SOMETHING was specified that this tool will not honour — an extra operand, an
//     unknown flag, a malformed tag — → REFUSED, exit 5, audited. This is the shape a
//     glob would have absorbed silently, so it is recorded loudly.
func cmdCut(argv []string) int {
	rest := argv[1:]
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "usage: deskrelease cut <tag> [--dry-run]")
		fmt.Fprintln(stderr, "  <tag> is (desk-tools|statusgen)/vMAJOR.MINOR.PATCH — argv only, never stdin/env")
		return 2
	}
	return runOutward(argv, func(entries []deskkit.Entry) writeResult {
		return planCut(rest)
	})
}
