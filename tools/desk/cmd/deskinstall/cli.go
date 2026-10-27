package main

// cli.go — deskinstall's command tree (desktools-v2/35). Cobra, through the desk adapter
// (internal/cli), parses the flags and prints help and the version; dispatch (main.go)
// receives typed options and keeps every mode rule, refusal and exit code the hand-rolled
// parser had. Nothing binds from a config file or the environment: deskinstall declares
// flags and their defaults only. Hash and attestation verification stay in Install.

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// usage is the root help's long text; Cobra appends the flag list from the bindings.
const usage = `deskinstall — Go-native, version-pinned, sha256- and attestation-verified toolchain installer.

  deskinstall --manifest <paired-versions.yaml> --dest <dir> [--platform <os-arch>]
  deskinstall --harness cursor --forge <github|gitlab> --repo <path> [--bundle <dir>] [--check]
  deskinstall --harness cursor --forge <github|gitlab> --repo <path> --release vX.Y.Z [--releases <dir>] [--fetch] [--release-home <owner/repo>] [--dry-run|--check]
  deskinstall --harness cursor --bundle <dir> --release vX.Y.Z --pack <archive-path>
  deskinstall --version
  deskinstall -h | --help

MODE 1 — acquire, verify, place (the pinned toolchain binaries). It resolves the
pinned tag + per-platform sha256 from the manifest (a pinned repo tag, never a
floating ref), downloads the release assets for the detected platform, VERIFIES
their sha256 and REFUSES on any mismatch, then VERIFIES each asset's build
attestation (<asset>.sigstore.json from the same release: SLSA provenance signed
by this repo's release workflow, run by the push of the pinned tag or by
workflow_dispatch from main, logged in the transparency log) and REFUSES on any
failure, then installs the verified binaries into --dest.
Any refusal places nothing. The attestation check is mandatory — no flag skips it.

Windows assets (from the release build matrix):
  statusgen-windows-amd64.exe      / statusgen-windows-arm64.exe
  desk-tools-windows-amd64.tar.gz  / desk-tools-windows-arm64.tar.gz

A successful run prints the .assay/ledger.jsonl path (component-model.md §5):
this mode's own effects are inside the boundary and record no line there, but
the path is where a later outside-effect component would, and it is what
` + "`deskdisable`" + ` reads when reversing one.

MODE 2 — harness placement (Cursor's install mechanism IS file placement: no
marketplace, no per-harness plugin manifest). Places the packaging roster's
skills into --repo/.cursor/skills, plugins/assay/references/*.md as a SIBLING
references/ tree so the skills' ../../references/*.md includes resolve, the
generated .cursor/rules/assay.mdc if the bundle carries one, and writes the
shared AGENTS.md bindings fragment between stable assay:bindings delimiters
(forge-substituted: gh on github, glab/--forge gitlab on gitlab). Idempotent —
a second identical run changes no byte.

Managed ownership is recorded in .cursor/assay-managed.json. Unrelated content is
preserved; modified managed files and unmanaged collisions refuse before writes.
AGENTS.md content outside the bounded Assay block is preserved. --check is read-only.

--harness is mutually exclusive with --manifest/--dest: supplying both is a
refusal naming both modes, never a silent precedence.

exit (mode 1, and mode 2 without --check): 0 installed/placed · 5 refused (usage errors too)
exit (mode 2 with --check): 0 clean · 1 drift · 2 could-not-check`

// options are the typed values the command tree hands to dispatch.
type options struct {
	manifest, dest, platform             string
	harness, forge, repo, bundle         string
	release, releases, releaseHome, pack string
	check, dryRun, fetch                 bool
}

// dispatchFn is the typed handler the command tree calls; the contract tests replace it to
// observe the options that reach it.
var dispatchFn = dispatch

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

// versionLine is what every version form prints.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskinstall sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskinstall declares: the flag spellings it always had, as plain
// flags with no environment or config source.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "manifest", Kind: cli.String, Flag: "manifest",
			Usage: "mode 1: path to the pin manifest (" + manifestName + "); required"},
		{Key: "dest", Kind: cli.String, Flag: "dest",
			Usage: "mode 1: PATH-resolvable directory the verified binaries are placed in; required"},
		{Key: "platform", Kind: cli.String, Flag: "platform",
			Usage: "mode 1: override auto-detection (e.g. windows-amd64); default: this host"},
		{Key: "harness", Kind: cli.String, Flag: "harness",
			Usage: `mode 2: harness to place for; only "cursor" is supported`},
		{Key: "forge", Kind: cli.String, Flag: "forge",
			Usage: "mode 2: github | gitlab, the desk-transport vocabulary the AGENTS.md bindings name; required"},
		{Key: "repo", Kind: cli.String, Flag: "repo",
			Usage: "mode 2: the adopter repo root the tree is placed into; required"},
		{Key: "bundle", Kind: cli.String, Flag: "bundle",
			Usage: `mode 2: the plugins/assay-shaped source tree to place from (default "plugins/assay" under the current directory)`},
		{Key: "check", Kind: cli.Bool, Flag: "check",
			Usage: "mode 2: diff what a run would place against --repo; writes nothing (0 clean, 1 drift, 2 could-not-check)"},
		{Key: "release", Kind: cli.String, Flag: "release",
			Usage: "mode 2: acquire the exact umbrella Cursor bundle vX.Y.Z and verify its published SHA-256, version and file manifest (integrity, not provenance)"},
		{Key: "releases", Kind: cli.String, Flag: "releases",
			Usage: "mode 2: local dir holding <version>.checksums.txt and <version>.cursor-bundle.tar.gz"},
		{Key: "fetch", Kind: cli.Bool, Flag: "fetch",
			Usage: "mode 2: permit release-asset downloads; URLs print first"},
		{Key: "release-home", Kind: cli.String, Flag: "release-home",
			Usage: "mode 2: override the release home (default medici-finance/assay)"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run",
			Usage: "mode 2: preview exact writes and retirement of unchanged managed files"},
		{Key: "pack", Kind: cli.String, Flag: "pack",
			Usage: "mode 2: generate a release archive from --bundle at the matching --release"},
	}
}

// unknownArg names the first command-line word deskinstall does not declare, as the
// hand-rolled parser's "unknown argument" refusal did; "" when every word is declared.
func unknownArg(cmd *cobra.Command, argv []string) string {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			return ""
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		if !strings.HasPrefix(a, "--") {
			if a == "-h" {
				continue
			}
			return a
		}
		name, _, hasValue := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		f := cmd.Flags().Lookup(name)
		if f == nil {
			return a
		}
		if !hasValue && f.NoOptDefVal == "" {
			i++ // the flag's value, whatever it looks like
		}
	}
	return ""
}

// newInstallCLI builds one fresh command tree for one invocation. argv is the command
// line, for naming an unknown word exactly as it was written.
func newInstallCLI(argv []string, stdout, stderr io.Writer) *cobra.Command {
	root := cli.NewRoot("deskinstall", "version-pinned, sha256- and attestation-verified toolchain installer")
	root.Long = usage
	root.Args = cobra.ArbitraryArgs
	// --version is the root's own flag, not Cobra's built-in one: Cobra answers its version
	// flag before anything else, so `deskinstall --version --manifest x` would print the
	// version. The legacy parser answered it only as the sole word.
	root.Flags().Bool("version", false, "print the version and exit (only as the sole argument)")
	set := cli.Declare(root, bindings()...)
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		// Cobra resolves the word help to its own help command, which has none of the root's
		// flags: `--check help` reaches here with c that command. Help answers only as the
		// sole word, so the refused word is "help", never the declared flag.
		if c != c.Root() && c.Name() == "help" {
			return fmt.Errorf("unknown argument %q", "help")
		}
		if a := unknownArg(c, argv); a != "" {
			return fmt.Errorf("unknown argument %q", a)
		}
		return err
	})
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() >= 0 {
			fmt.Fprintf(stderr, "deskinstall: unknown argument %q\nRun 'deskinstall --help' for usage.\n", "--")
			return exitErr(deskkit.ExitRefused)
		}
		if len(args) > 0 {
			switch {
			case len(args) == 1 && args[0] == "version" && cmd.Flags().NFlag() == 0:
				_, err := fmt.Fprintln(stdout, versionLine())
				return err
			}
			fmt.Fprintf(stderr, "deskinstall: unknown argument %q\nRun 'deskinstall --help' for usage.\n", args[0])
			return exitErr(deskkit.ExitRefused)
		}
		if v, _ := cmd.Flags().GetBool("version"); v {
			if cmd.Flags().NFlag() != 1 {
				fmt.Fprintf(stderr, "deskinstall: unknown argument %q\nRun 'deskinstall --help' for usage.\n", "--version")
				return exitErr(deskkit.ExitRefused)
			}
			_, err := fmt.Fprintln(stdout, versionLine())
			return err
		}
		if cmd.Flags().NFlag() == 0 {
			// No arguments is a refusal, as it always was: the help screen on stderr, exit 5.
			cmd.SetOut(stderr)
			_ = cmd.Help()
			return exitErr(deskkit.ExitRefused)
		}
		v, err := set.Resolve(cli.Inputs{LookupEnv: noEnv})
		if err != nil {
			fmt.Fprintf(stderr, "deskinstall: %v\n", err)
			return exitErr(deskkit.ExitRefused)
		}
		if onResolve != nil {
			onResolve(v)
		}
		return codeErr(dispatchFn(optionsFrom(v), stdout, stderr))
	}
	return root
}

// noEnv is the environment deskinstall reads: none. No binding names a variable, so the
// adapter never asks; this makes the absence explicit.
func noEnv(string) (string, bool) { return "", false }

// run is one invocation against a fresh command tree. sinks, when given, receive the
// performance context the parser selected (main's measurement).
func run(args []string, stdout, stderr io.Writer, sinks ...func(perfrec.Context)) int {
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	// The one legacy single-dash spelling: -version as the sole argument.
	if len(args) == 1 && args[0] == "-version" {
		args = []string{"--version"}
	}
	return cli.Run(func() *cobra.Command { return newInstallCLI(args, stdout, stderr) }, args, cli.Options{
		Performance: sink,
		IO:          cli.IO{Out: stdout, Err: stderr},
		UsageExit:   deskkit.ExitRefused,
	})
}

func optionsFrom(v *cli.Values) options {
	return options{
		manifest: v.String("manifest"), dest: v.String("dest"), platform: v.String("platform"),
		harness: v.String("harness"), forge: v.String("forge"), repo: v.String("repo"), bundle: v.String("bundle"),
		release: v.String("release"), releases: v.String("releases"), releaseHome: v.String("release-home"),
		pack: v.String("pack"), check: v.Bool("check"), dryRun: v.Bool("dry-run"), fetch: v.Bool("fetch"),
	}
}
